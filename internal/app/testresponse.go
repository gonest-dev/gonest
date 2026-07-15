package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gonest-dev/gonest/internal/route"
)

// TestResponse wraps the *http.Response MustRequest dispatched, exposing
// test-assertion helpers (AssertStatus/AssertJsonPath) instead of forcing
// every caller to decode/compare manually.
type TestResponse struct {
	resp *http.Response
	body []byte
}

// AssertStatus fails t (via t.Helper()+t.Fatalf) if the actual response
// status differs from want.
func (r *TestResponse) AssertStatus(t *testing.T, want int) {
	t.Helper()
	if r.resp.StatusCode != want {
		t.Fatalf("status = %d, want %d (body: %s)", r.resp.StatusCode, want, string(r.body))
	}
}

// AssertJsonPath decodes the response body as JSON, extracts the value at
// path (dot-notation, e.g. "id", "address.zip"), and fails t if it doesn't
// deep-equal want. want's own JSON-decoded shape is used for comparison
// (e.g. an int64(42) want is compared against the decoded float64 the same
// way encoding/json itself would decode a bare number into `any`) -- both
// sides are round-tripped through json.Marshal/Unmarshal into `any` so a
// caller passing a concrete Go type (int64(42)) compares equal to the
// body's own decoded numeric representation.
func (r *TestResponse) AssertJsonPath(t *testing.T, path string, want any) {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(r.body, &decoded); err != nil {
		t.Fatalf("failed to decode response body as JSON: %v (body: %s)", err, string(r.body))
	}

	got, ok := lookupJSONPath(decoded, strings.Split(path, "."))
	if !ok {
		t.Fatalf("JSON path %q not found in response body: %s", path, string(r.body))
	}

	wantNormalized, err := normalizeJSONValue(want)
	if err != nil {
		t.Fatalf("failed to normalize want value %v: %v", want, err)
	}

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(wantNormalized)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("JSON path %q = %s, want %s", path, gotJSON, wantJSON)
	}
}

// normalizeJSONValue round-trips want through json.Marshal/Unmarshal into
// `any`, so a caller-supplied concrete Go value (e.g. int64(42)) compares
// against the body's own decoded representation (json.Unmarshal always
// produces float64 for bare JSON numbers) on equal footing.
func normalizeJSONValue(want any) (any, error) {
	b, err := json.Marshal(want)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// lookupJSONPath walks segments (a dot-notation path already split on ".")
// into decoded (the result of json.Unmarshal into `any`), returning the
// value found and whether every segment resolved to an existing map key.
func lookupJSONPath(decoded any, segments []string) (any, bool) {
	cur := decoded
	for _, seg := range segments {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[seg]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// MustRequest dispatches an in-memory HTTP request (method, path, body)
// against the app MustNewTestApp built -- no real network port is bound,
// same mechanism every existing test in this codebase already uses via the
// concrete Fiber adapter's own Test method (see internal/app.HttpAdapter's
// new Test method, satisfied by every adapter implementation). body, if
// non-nil, is JSON-encoded as the request body (same encoding
// ctx.Json/MustJsonBody already use elsewhere). Panics on a transport-level
// failure -- never on a non-2xx status, which is for the caller's own
// AssertStatus to check.
func (a *TestApp) MustRequest(method route.HttpMethod, path string, body any) *TestResponse {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			panic(fmt.Sprintf("gonest: MustRequest failed to JSON-encode body: %v", err))
		}
		reader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method.String(), path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.adapter.Test(req)
	if err != nil {
		panic(fmt.Sprintf("gonest: MustRequest transport error: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Sprintf("gonest: MustRequest failed to read response body: %v", err))
	}

	return &TestResponse{resp: resp, body: respBody}
}
