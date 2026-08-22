package exception_test

import (
	"net/http"
	"testing"

	"gonest.dev/gonest/internal/exception"
)

// builtinCase describes one built-in exception constructor under test: the
// constructor itself (returning an Exception so all five can share one table),
// and the fixed status/name it must always produce regardless of details.
type builtinCase struct {
	name        string
	construct   func(details any) exception.Exception
	wantStatus  int
	wantName    string
	wantMessage string
}

func builtinCases() []builtinCase {
	return []builtinCase{
		{
			name:        "NotFoundException",
			construct:   func(details any) exception.Exception { return exception.NewNotFoundException("", details) },
			wantStatus:  http.StatusNotFound,
			wantName:    "NotFoundException",
			wantMessage: "Resource not found",
		},
		{
			name:        "BadRequestException",
			construct:   func(details any) exception.Exception { return exception.NewBadRequestException("", details) },
			wantStatus:  http.StatusBadRequest,
			wantName:    "BadRequestException",
			wantMessage: "Bad request",
		},
		{
			name:        "ConflictException",
			construct:   func(details any) exception.Exception { return exception.NewConflictException("", details) },
			wantStatus:  http.StatusConflict,
			wantName:    "ConflictException",
			wantMessage: "Conflict",
		},
		{
			name:        "UnauthorizedException",
			construct:   func(details any) exception.Exception { return exception.NewUnauthorizedException("", details) },
			wantStatus:  http.StatusUnauthorized,
			wantName:    "UnauthorizedException",
			wantMessage: "Unauthorized",
		},
		{
			name:        "ForbiddenException",
			construct:   func(details any) exception.Exception { return exception.NewForbiddenException("", details) },
			wantStatus:  http.StatusForbidden,
			wantName:    "ForbiddenException",
			wantMessage: "Forbidden",
		},
	}
}

// TestBuiltinException_ConstructorFixedFields verifies each built-in
// constructor sets the correct fixed status code and name, defaults the
// message to the built-in default string, and passes details through
// unchanged -- including when details is nil (mirrors INSIGHT.md's
// NewUnauthorizedException(nil)).
func TestBuiltinException_ConstructorFixedFields(t *testing.T) {
	details := map[string]any{"userId": "abc123"}

	for _, tc := range builtinCases() {
		t.Run(tc.name, func(t *testing.T) {
			exc := tc.construct(details)

			if got := exc.Status(); got != tc.wantStatus {
				t.Errorf("Status() = %d, want %d", got, tc.wantStatus)
			}
			if got := exc.Name(); got != tc.wantName {
				t.Errorf("Name() = %q, want %q", got, tc.wantName)
			}
			if got := exc.Message(); got != tc.wantMessage {
				t.Errorf("Message() = %q, want %q", got, tc.wantMessage)
			}
			if got := exc.Details(); got == nil {
				t.Errorf("Details() = nil, want %v", details)
			}
		})
	}
}

// TestBuiltinException_ConstructorAcceptsNilDetails verifies New*Exception(nil)
// works for all five built-ins, per INSIGHT.md's NewUnauthorizedException(nil)
// usage.
func TestBuiltinException_ConstructorAcceptsNilDetails(t *testing.T) {
	for _, tc := range builtinCases() {
		t.Run(tc.name, func(t *testing.T) {
			exc := tc.construct(nil)

			if got := exc.Status(); got != tc.wantStatus {
				t.Errorf("Status() = %d, want %d", got, tc.wantStatus)
			}
			if got := exc.Name(); got != tc.wantName {
				t.Errorf("Name() = %q, want %q", got, tc.wantName)
			}
			if got := exc.Details(); got != nil {
				t.Errorf("Details() = %v, want nil", got)
			}
		})
	}
}

// TestBuiltinException_SatisfiesException verifies all five built-in types
// satisfy the Exception interface structurally, via HttpException embedding
// -- mirrors T1's own assertion pattern.
func TestBuiltinException_SatisfiesException(t *testing.T) {
	var (
		_ exception.Exception = (*exception.NotFoundException)(nil)
		_ exception.Exception = (*exception.BadRequestException)(nil)
		_ exception.Exception = (*exception.ConflictException)(nil)
		_ exception.Exception = (*exception.UnauthorizedException)(nil)
		_ exception.Exception = (*exception.ForbiddenException)(nil)
	)
}

// TestBuiltinException_PanicRecoverRoundTrip verifies each built-in exception
// can be panicked with and recovered via a type assertion back to its
// pointer type, mirroring INSIGHT.md's own test pattern:
// exc, ok := recover().(*gonest.NotFoundException).
func TestBuiltinException_PanicRecoverRoundTrip(t *testing.T) {
	t.Run("NotFoundException", func(t *testing.T) {
		defer func() {
			r := recover()
			exc, ok := r.(*exception.NotFoundException)
			if !ok {
				t.Fatalf("recover() type assertion to *NotFoundException failed, got %T", r)
			}
			if exc.Status() != http.StatusNotFound {
				t.Errorf("Status() = %d, want %d", exc.Status(), http.StatusNotFound)
			}
			if exc.Name() != "NotFoundException" {
				t.Errorf("Name() = %q, want %q", exc.Name(), "NotFoundException")
			}
		}()
		panic(exception.NewNotFoundException("", map[string]any{"userId": "abc123"}))
	})

	t.Run("BadRequestException", func(t *testing.T) {
		defer func() {
			r := recover()
			exc, ok := r.(*exception.BadRequestException)
			if !ok {
				t.Fatalf("recover() type assertion to *BadRequestException failed, got %T", r)
			}
			if exc.Status() != http.StatusBadRequest {
				t.Errorf("Status() = %d, want %d", exc.Status(), http.StatusBadRequest)
			}
			if exc.Name() != "BadRequestException" {
				t.Errorf("Name() = %q, want %q", exc.Name(), "BadRequestException")
			}
		}()
		panic(exception.NewBadRequestException("", nil))
	})

	t.Run("ConflictException", func(t *testing.T) {
		defer func() {
			r := recover()
			exc, ok := r.(*exception.ConflictException)
			if !ok {
				t.Fatalf("recover() type assertion to *ConflictException failed, got %T", r)
			}
			if exc.Status() != http.StatusConflict {
				t.Errorf("Status() = %d, want %d", exc.Status(), http.StatusConflict)
			}
			if exc.Name() != "ConflictException" {
				t.Errorf("Name() = %q, want %q", exc.Name(), "ConflictException")
			}
		}()
		panic(exception.NewConflictException("", nil))
	})

	t.Run("UnauthorizedException", func(t *testing.T) {
		defer func() {
			r := recover()
			exc, ok := r.(*exception.UnauthorizedException)
			if !ok {
				t.Fatalf("recover() type assertion to *UnauthorizedException failed, got %T", r)
			}
			if exc.Status() != http.StatusUnauthorized {
				t.Errorf("Status() = %d, want %d", exc.Status(), http.StatusUnauthorized)
			}
			if exc.Name() != "UnauthorizedException" {
				t.Errorf("Name() = %q, want %q", exc.Name(), "UnauthorizedException")
			}
		}()
		panic(exception.NewUnauthorizedException("", nil))
	})

	t.Run("ForbiddenException", func(t *testing.T) {
		defer func() {
			r := recover()
			exc, ok := r.(*exception.ForbiddenException)
			if !ok {
				t.Fatalf("recover() type assertion to *ForbiddenException failed, got %T", r)
			}
			if exc.Status() != http.StatusForbidden {
				t.Errorf("Status() = %d, want %d", exc.Status(), http.StatusForbidden)
			}
			if exc.Name() != "ForbiddenException" {
				t.Errorf("Name() = %q, want %q", exc.Name(), "ForbiddenException")
			}
		}()
		panic(exception.NewForbiddenException("", map[string]any{"reason": "no access"}))
	})
}
