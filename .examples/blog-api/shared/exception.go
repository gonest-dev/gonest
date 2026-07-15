package shared

import (
	"net/http"

	"github.com/gonest-dev/gonest"
)

// DuplicateEmailException is a dev-defined exception (INSIGHT.md's own
// `type FooExampleError struct { gonest.HttpException }` pattern) --
// thrown when creating a user whose email already exists (SQLite's own
// UNIQUE constraint violation, translated into a structured HTTP error).
type DuplicateEmailException struct {
	gonest.HttpException
}

func NewDuplicateEmailException(email string) *DuplicateEmailException {
	return &DuplicateEmailException{
		HttpException: gonest.NewHttpException().
			SetStatus(http.StatusConflict).
			SetName("DuplicateEmailException").
			SetMessage("email already in use").
			SetDetails(map[string]string{"email": email}),
	}
}

// DomainFilter catches DuplicateEmailException globally (registered via
// Module.Filters on the root AppModule) -- ctx.Json(err) alone produces
// the full {"name","message","details"} body via HttpException's own
// MarshalJSON (promoted through embedding), no manual field mapping
// needed.
var DomainFilter = gonest.NewFilter(func(filter *gonest.Filter) {
	filter.Catch(&DuplicateEmailException{}, func(ctx *gonest.RestContext, err *DuplicateEmailException) {
		ctx.Status(err.Status()).Json(err)
	})
})
