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

// formBodySource is the Parseable implementation behind
// ctx.Body().Form(onFile) (unified-parse-api feature). Holds onFile
// alongside ctx (context.md's Decision: "the onFile callback is an argument
// of Form(), not of Parse[T]/MustParse[T]").
//
// ctx must come from a request whose Content-Type is multipart/form-data AND
// whose app was built with AppOptions.EnableFormStreaming -- otherwise
// ParseInto panics with a plain string identifying the missing setup (same
// category of failure as resolveSchema's own mismatch panic: a
// coding/config error, not a request-validation failure, so it is NOT
// returned via the error return below -- spec.md's P.5).
type formBodySource struct {
	req    *execution.Request
	onFile func(*execution.FormFile) error
}

// NewFormBodySource builds a Parseable for ctx's multipart/form-data body.
// onFile is invoked for each file part; nil means file parts are silently
// skipped (spec.md's Edge Cases). Exported so internal/app can wire one into
// a Context's BodySource per-request.
func NewFormBodySource(req *execution.Request, onFile func(*execution.FormFile) error) execution.Parseable {
	return &formBodySource{req: req, onFile: onFile}
}

// ParseInto walks s.req's raw multipart stream exactly ONCE
// (mime/multipart.NewReader, NextPart() until io.EOF), since form fields and
// file parts can be interleaved in any order the client actually sent them
// (Multipart Form Streaming design.md's Architecture Overview), populating
// dst (a *T) via T's `form:"name"` struct tags, validating against schema
// (a *schema.Schema) first:
//  1. resolveSchema confirms schema describes T, panicking immediately,
//     BEFORE reading any part at all, otherwise.
//  2. A part with a non-empty FileName() is a FILE -- s.onFile is invoked
//     synchronously, THE MOMENT that part is reached, while its bytes are
//     still un-consumed (true streaming: a caller can pipe file.Reader()
//     straight to S3/etc from inside onFile, before the next part is even
//     read), UNLESS s.onFile is nil, in which case the file part is
//     silently skipped. If onFile returns a non-nil error, the walk stops
//     immediately and that error becomes a violation (field = the file's
//     own form field name).
//  3. A part with an empty FileName() is a regular form FIELD, validated
//     against schema's `form:"..."` tag (a NEW tag family, distinct from
//     json/param/query/header -- see design.md's Tech Decisions) via the
//     SAME coerceParamString/validateValue/Custom(fn) machinery param/query
//     already use, since a raw multipart field value is a string exactly
//     like a raw param/query value is.
//  4. After the walk completes (io.EOF), any Required field NEVER seen in
//     the stream at all is collected as a violation (same convention
//     validateStruct/paramsSource/querySource already use for absent
//     Required fields).
//  5. If any violations were collected: return exception.NewBadRequestException(violations)
//     as the error.
//  6. Otherwise: populate dst field-by-field via the shared populate core
//     (tag="form"), using the SAME raw/coerced value already produced
//     during the walk as the presence map's value.
func (s *formBodySource) ParseInto(dst any, schemaArg any) error {
	m := schemaArg.(*schema.Schema)
	dstVal := reflect.ValueOf(dst).Elem()
	resolveSchema(m, dstVal.Type())

	stream, boundary, ok := s.req.FormStream()
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
			return exception.NewBadRequestException([]violation{
				{Field: "", Message: fmt.Sprintf("invalid multipart body: %v", err)},
			})
		}

		if part.FileName() != "" {
			if s.onFile != nil {
				file := execution.NewFormFile(part)
				if err := s.onFile(file); err != nil {
					return exception.NewBadRequestException([]violation{
						{Field: part.FormName(), Message: err.Error()},
					})
				}
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
		return exception.NewBadRequestException(violations)
	}

	if err := populate(dstVal, presence, m, "form"); err != nil {
		// Should be unreachable in practice, same rationale as
		// paramsSource's/jsonBodySource's own equivalent case: the validate
		// pass above already proved every present field's shape matches
		// what T expects.
		return exception.NewBadRequestException([]violation{
			{Field: "", Message: fmt.Sprintf("failed to populate form: %v", err)},
		})
	}

	return nil
}
