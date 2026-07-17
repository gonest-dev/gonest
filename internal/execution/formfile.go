package execution

import (
	"io"
	"mime/multipart"
)

// FormFile is one file part from a multipart/form-data stream, still
// un-consumed at the moment the Form() Parseable's onFile callback is
// called -- reading from Reader() consumes the underlying multipart stream
// in real time (Multipart Form Streaming feature, AD-022 in STATE.md). Not
// safe to retain past onFile's own call, same "synchronous use only"
// precedent as Context.RawBody()'s own doc comment.
//
// Lives in internal/execution (not internal/validate, where it lived before
// the unified-parse-api feature): BodySource.Form's onFile signature
// (func(*FormFile) error) is declared here, in execution, so it must name a
// type execution itself owns -- internal/validate imports execution (not
// the reverse), so validate's formBodySource can freely use this same type.
type FormFile struct {
	part *multipart.Part
}

// NewFormFile wraps part as a FormFile. Exported so internal/validate (which
// walks the raw multipart stream) can construct one per file part
// encountered.
func NewFormFile(part *multipart.Part) *FormFile {
	return &FormFile{part: part}
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
