package route

import "testing"

func TestHttpGet_String(t *testing.T) {
	got := HttpGet.String()
	want := "GET"
	if got != want {
		t.Fatalf("HttpGet.String() = %q, want %q", got, want)
	}
}

func TestHttpPost_String(t *testing.T) {
	got := HttpPost.String()
	want := "POST"
	if got != want {
		t.Fatalf("HttpPost.String() = %q, want %q", got, want)
	}
}

func TestHttpPut_String(t *testing.T) {
	got := HttpPut.String()
	want := "PUT"
	if got != want {
		t.Fatalf("HttpPut.String() = %q, want %q", got, want)
	}
}

func TestHttpDelete_String(t *testing.T) {
	got := HttpDelete.String()
	want := "DELETE"
	if got != want {
		t.Fatalf("HttpDelete.String() = %q, want %q", got, want)
	}
}

func TestHttpQuery_String(t *testing.T) {
	got := HttpQuery.String()
	want := "QUERY"
	if got != want {
		t.Fatalf("HttpQuery.String() = %q, want %q", got, want)
	}
}

func TestHttpMethod_String_Unknown(t *testing.T) {
	unknown := HttpMethod(999)
	got := unknown.String()
	want := "Unknown"
	if got != want {
		t.Fatalf("HttpMethod(999).String() = %q, want %q", got, want)
	}
}
