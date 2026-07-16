package validate

import (
	"fmt"
	"io"
	"mime/multipart"
	"reflect"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// FormFile is one file part from a multipart/form-data stream, still
// un-consumed at the moment ParseFormBody's onFile callback is called --
// reading from Reader() consumes the underlying multipart stream in real
// time (Multipart Form Streaming feature, AD-022 in STATE.md). Not safe to
// retain past onFile's own call, same "synchronous use only" precedent as
// execution.Context.Body()'s own doc comment.
type FormFile struct {
	part *multipart.Part
}

// FieldName returns the form field name (Content-Disposition's "name").
func (f *FormFile) FieldName() string {
	return f.part.FormName()
}

// Filename returns the uploaded file's own name (Content-Disposition's
// "filename").
func (f *FormFile) Filename() string {
	return f.part.FileName()
}

// ContentType returns the part's own Content-Type header, or "" if absent.
func (f *FormFile) ContentType() string {
	return f.part.Header.Get("Content-Type")
}

// Reader returns the live part -- *multipart.Part already implements
// io.Reader directly, reading it consumes the multipart stream.
func (f *FormFile) Reader() io.Reader {
	return f.part
}

// ParseFormBody is the real implementation behind gonest.ParseRestFormBody[T]
// (T is a pointer type at the call site, e.g. ParseFormBody[*CreatePostForm]).
// m must be the *schema.Schema built via NewSchema[T] for that same T
// (AD-019) -- resolveSchema panics if it describes a different type. ctx
// must come from a request whose Content-Type is multipart/form-data AND
// whose app was built with AppOptions.EnableFormStreaming -- otherwise this
// panics with a plain string identifying the missing setup (same category
// of failure as resolveSchema's own mismatch panic: a coding/config error,
// not a request-validation failure, so it is NOT returned via the (T,
// error) channel below).
//
// Walks the raw multipart stream exactly ONCE (mime/multipart.NewReader,
// NextPart() until io.EOF), since form fields and file parts can be
// interleaved in any order the client actually sent them (Multipart Form
// Streaming design.md's Architecture Overview):
//  1. A part with a non-empty FileName() is a FILE -- onFile is invoked
//     synchronously, THE MOMENT that part is reached, while its bytes are
//     still un-consumed (true streaming: a caller can pipe file.Reader()
//     straight to S3/etc from inside onFile, before the next part is even
//     read). If onFile returns a non-nil error, the walk stops immediately
//     and that error becomes a violation (field = the file's own form
//     field name).
//  2. A part with an empty FileName() is a regular form FIELD, validated
//     against m's `form:"..."` tag (a NEW tag family, distinct from
//     json/param/query -- see design.md's Tech Decisions) via the SAME
//     coerceParamString/validateValue/Custom(fn) machinery param/query
//     already use, since a raw multipart field value is a string exactly
//     like a raw param/query value is.
//  3. After the walk completes (io.EOF), any Required field NEVER seen in
//     the stream at all is collected as a violation (same convention
//     validateStruct/ParseParams/ParseQuery already use for absent
//     Required fields).
//  4. If any violations were collected: return exception.NewBadRequestException(violations)
//     as the error.
//  5. Otherwise: populate a fresh *structType field-by-field via the
//     shared populate core (tag="form"), using the SAME raw/coerced value
//     already produced during the walk as the presence map's value.
func ParseFormBody[T any](ctx *execution.Context, m *schema.Schema, onFile func(*FormFile) error) (T, error) {
	var zero T
	structType := reflect.TypeOf(zero).Elem()
	resolveSchema(m, structType)

	stream, boundary, ok := ctx.FormStream()
	if !ok {
		panic("gonest: form stream unavailable -- enable AppOptions.EnableFormStreaming when building the app, and ensure the request's Content-Type is multipart/form-data with a boundary")
	}

	props := m.OwnProperties()
	byKey := make(map[string]*schema.PropertyBuilder, len(props))
	for _, p := range props {
		key, visible := tagKeyVisible(p.Field(), "form")
		if !visible {
			continue
		}
		byKey[key] = p
	}

	mr := multipart.NewReader(stream, boundary)

	var violations []violation
	presence := map[string]any{}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return zero, exception.NewBadRequestException([]violation{
				{Field: "", Message: fmt.Sprintf("invalid multipart body: %v", err)},
			})
		}

		if part.FileName() != "" {
			file := &FormFile{part: part}
			if err := onFile(file); err != nil {
				return zero, exception.NewBadRequestException([]violation{
					{Field: part.FormName(), Message: err.Error()},
				})
			}
			continue
		}

		key := part.FormName()
		p, known := byKey[key]
		if !known {
			// A field the schema never declared -- ignored, same
			// graceful-leniency precedent as an unrecognized JSON key
			// (validateStruct only ever walks m.OwnProperties(), never the
			// raw JSON object's own keys). NextPart()'s next call discards
			// whatever of THIS part goes unread, standard
			// mime/multipart.Reader behavior.
			continue
		}

		raw, err := io.ReadAll(part)
		if err != nil {
			violations = append(violations, violation{Field: key, Message: fmt.Sprintf("failed to read field: %v", err)})
			continue
		}
		rawStr := string(raw)

		if _, isCustom := p.CustomFunc(); isCustom {
			violations = append(violations, validateValue(rawStr, p, key)...)
			presence[key] = rawStr
			continue
		}

		coerced, err := coerceParamString(rawStr, p.KindValue())
		if err != nil {
			violations = append(violations, violation{Field: key, Message: err.Error()})
			continue
		}

		violations = append(violations, validateValue(coerced, p, key)...)
		presence[key] = coerced
	}

	for key, p := range byKey {
		if _, present := presence[key]; !present && p.IsRequired() {
			violations = append(violations, violation{Field: key, Message: "required"})
		}
	}

	if len(violations) > 0 {
		return zero, exception.NewBadRequestException(violations)
	}

	out := reflect.New(structType)
	if err := populate(out.Elem(), presence, m, "form"); err != nil {
		// Should be unreachable in practice, same rationale as
		// ParseParams'/ParseJsonBody's own equivalent case: the validate
		// pass above already proved every present field's shape matches
		// what T expects.
		return zero, exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to populate form: %v", err)},
		})
	}

	return out.Interface().(T), nil
}

// MustFormBody is the real implementation behind gonest.MustParseRestFormBody[T]
// -- ParseFormBody, panicking on any returned error (AD-021's panicking
// twin, for call sites that don't want to handle the error themselves).
func MustFormBody[T any](ctx *execution.Context, m *schema.Schema, onFile func(*FormFile) error) T {
	v, err := ParseFormBody[T](ctx, m, onFile)
	if err != nil {
		panic(err)
	}
	return v
}
