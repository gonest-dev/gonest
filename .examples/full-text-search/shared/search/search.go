package search

import (
	"reflect"
	"strings"
	"time"

	"gonest.dev/gonest"
)

// #region OperatorMatchString

type OperatorMatchString string

func (o OperatorMatchString) String() string { return string(o) }

const (
	OperatorMatchStringEQ    OperatorMatchString = "eq"
	OperatorMatchStringLIKE  OperatorMatchString = "like"
	OperatorMatchStringNEQ   OperatorMatchString = "neq"
	OperatorMatchStringNLIKE OperatorMatchString = "nlike"
)

// #endregion
// #region OperatorMatchNumber

type OperatorMatchNumber string

func (o OperatorMatchNumber) String() string { return string(o) }

const (
	OperatorMatchNumberEQ   OperatorMatchNumber = "eq"
	OperatorMatchNumberGT   OperatorMatchNumber = "gt"
	OperatorMatchNumberGTE  OperatorMatchNumber = "gte"
	OperatorMatchNumberLT   OperatorMatchNumber = "lt"
	OperatorMatchNumberLTE  OperatorMatchNumber = "lte"
	OperatorMatchNumberNEQ  OperatorMatchNumber = "neq"
	OperatorMatchNumberNGT  OperatorMatchNumber = "ngt"
	OperatorMatchNumberNGTE OperatorMatchNumber = "ngte"
	OperatorMatchNumberNLT  OperatorMatchNumber = "nlt"
	OperatorMatchNumberNLTE OperatorMatchNumber = "nlte"
)

// #endregion
// #region OperatorMatchBoolean

type OperatorMatchBoolean string

func (o OperatorMatchBoolean) String() string { return string(o) }

const (
	OperatorMatchBooleanEQ  OperatorMatchBoolean = "eq"
	OperatorMatchBooleanNEQ OperatorMatchBoolean = "neq"
)

// #endregion
// #region OperatorMatchDate

type OperatorMatchDate string

func (o OperatorMatchDate) String() string { return string(o) }

const (
	OperatorMatchDateEQ   OperatorMatchDate = "eq"
	OperatorMatchDateGT   OperatorMatchDate = "gt"
	OperatorMatchDateGTE  OperatorMatchDate = "gte"
	OperatorMatchDateLT   OperatorMatchDate = "lt"
	OperatorMatchDateLTE  OperatorMatchDate = "lte"
	OperatorMatchDateNEQ  OperatorMatchDate = "neq"
	OperatorMatchDateNGT  OperatorMatchDate = "ngt"
	OperatorMatchDateNGTE OperatorMatchDate = "ngte"
	OperatorMatchDateNLT  OperatorMatchDate = "nlt"
	OperatorMatchDateNLTE OperatorMatchDate = "nlte"
)

// #endregion
// #region OperatorRange

type OperatorRange string

func (o OperatorRange) String() string { return string(o) }

const (
	OperatorRangeIN  OperatorRange = "in"
	OperatorRangeNIN OperatorRange = "nin"
)

// #endregion
// #region Matcher

// Matcher is satisfied by *MatchString[T]/*MatchNumber[T]/*MatchBool/
// *MatchDate -- every Match* type's own Match method already has this exact
// (value *T) bool shape (see each type's own Match doc comment), so
// MatchField below can dispatch through it generically instead of every
// entity's own Where predicate hand-rolling the "IsDirty, then Match" pair
// once per field.
type Matcher[T comparable] interface{ Match(value *T) bool }

// MatchField checks whether acc (one Where field, e.g.
// gonest.Accessor[MatchString[string]]) was set at all -- if not, that field
// imposes no constraint and MatchField reports true. Otherwise it delegates
// to the dirty Match value's own Match method against value (the entity's
// own field, addressed so a nullable column can pass its existing pointer
// straight through -- see MatchString/MatchNumber/MatchBool/MatchDate's own
// Match doc comments for the nil-handling each implements).
//
// PM mirrors the same "T value, *T implements the interface" pattern
// gonest's own root NewApp[T, PT] uses (gonest.go) -- M is the plain Match*
// struct Accessor[M] actually stores (a value type, so JSON
// marshal/unmarshal and dirty-tracking work the normal Accessor way); PM is
// inferred as *M and is what actually satisfies Matcher[T], since every
// Match method has a pointer receiver.
func MatchField[T comparable, M any, PM interface {
	*M
	Matcher[T]
}](acc gonest.Accessor[M], value *T) bool {
	if !acc.IsDirty() {
		return true
	}
	m := acc.Get()
	return PM(&m).Match(value)
}

// #endregion
// #region MatchString

// MatchString holds one string field's requested match operators as
// gonest.Accessor[string] instead of *string -- Accessor.IsDirty() is
// this whole package's one reusable "was this operator actually set" check
// (same dirty-tracking CreateBodyDTO/UpdateBodyDTO-style write DTOs already
// use), rather than a nil-pointer check re-implemented per operator family.
// At most one operator is expected to be dirty per query, keyed the same
// way as OperatorMatchString's own const names (eq/like/neq/nlike).
type MatchString[T ~string] struct {
	Eq    gonest.Accessor[T] `json:"eq"`
	Like  gonest.Accessor[T] `json:"like"`
	Neq   gonest.Accessor[T] `json:"neq"`
	Nlike gonest.Accessor[T] `json:"nlike"`
}

var _ Matcher[string] = (*MatchString[string])(nil)

// Match dispatches each dirty operator on m through its own
// OperatorMatchString* const, ANDing every dirty operator together -- any
// entity's Where predicate calls this instead of re-implementing the
// eq/like/neq/nlike comparisons itself. value is a *T (not T) so ONE method
// covers both a plain string field (pass &localVar) and a nullable *string
// column (pass the pointer field directly) -- a nil value is treated as T's
// zero value (""), same semantics the old, now-removed MatchNullable had.
func (m *MatchString[T]) Match(value *T) bool {
	var v T
	if value != nil {
		v = *value
	}
	ops := map[OperatorMatchString]gonest.Accessor[T]{
		OperatorMatchStringEQ:    m.Eq,
		OperatorMatchStringNEQ:   m.Neq,
		OperatorMatchStringLIKE:  m.Like,
		OperatorMatchStringNLIKE: m.Nlike,
	}
	for op, target := range ops {
		if !target.IsDirty() {
			continue
		}
		switch op {
		case OperatorMatchStringEQ:
			if v != target.Get() {
				return false
			}
		case OperatorMatchStringNEQ:
			if v == target.Get() {
				return false
			}
		case OperatorMatchStringLIKE:
			if !strings.Contains(string(v), string(target.Get())) {
				return false
			}
		case OperatorMatchStringNLIKE:
			if strings.Contains(string(v), string(target.Get())) {
				return false
			}
		}
	}
	return true
}

var MatchStringSchema = gonest.NewSchema(func(t *MatchString[string], s *gonest.Schema) {
	s.Title("search.MatchString")
	s.Property(&t.Eq).String()
	s.Property(&t.Like).String()
	s.Property(&t.Neq).String()
	s.Property(&t.Nlike).String()
})

// #endregion
// #region MatchNumber

// MatchNumber holds one numeric field's requested match operators as
// gonest.Accessor[T] -- see MatchString's own doc comment for why Accessor
// (not *T) is this package's uniform "was this operator set" mechanism.
// Keyed the same way as OperatorMatchNumber's own const names.
type MatchNumber[T ~int | ~int32 | ~int64 | ~float32 | ~float64] struct {
	Eq   gonest.Accessor[T] `json:"eq"`
	Gt   gonest.Accessor[T] `json:"gt"`
	Gte  gonest.Accessor[T] `json:"gte"`
	Lt   gonest.Accessor[T] `json:"lt"`
	Lte  gonest.Accessor[T] `json:"lte"`
	Neq  gonest.Accessor[T] `json:"neq"`
	Ngt  gonest.Accessor[T] `json:"ngt"`
	Ngte gonest.Accessor[T] `json:"ngte"`
	Nlt  gonest.Accessor[T] `json:"nlt"`
	Nlte gonest.Accessor[T] `json:"nlte"`
}

var _ Matcher[int] = (*MatchNumber[int])(nil)

// MatchNumberIntSchema is MatchNumber[int]'s own schema -- kind="integer",
// reused (via SchemaMap) for every whole-number instantiation (int/int32/
// int64 and their pointer forms).
var MatchNumberIntSchema = gonest.NewSchema(func(t *MatchNumber[int], s *gonest.Schema) {
	s.Title("search.MatchNumberInt")
	s.Property(&t.Eq).Integer()
	s.Property(&t.Gt).Integer()
	s.Property(&t.Gte).Integer()
	s.Property(&t.Lt).Integer()
	s.Property(&t.Lte).Integer()
	s.Property(&t.Neq).Integer()
	s.Property(&t.Ngt).Integer()
	s.Property(&t.Ngte).Integer()
	s.Property(&t.Nlt).Integer()
	s.Property(&t.Nlte).Integer()
})

// MatchNumberFloatSchema is MatchNumber[float64]'s own schema -- kind=
// "number" (Double(), NOT Integer()), reused (via SchemaMap) for every
// floating-point instantiation (float32/float64 and their pointer forms).
// Kept as its own schema rather than reusing MatchNumberIntSchema because
// validate.go enforces kind="integer" by rejecting any non-whole JSON
// number ("expected integer, got non-integer number") -- reusing the int
// schema here would silently break fractional filter values like 3.5.
var MatchNumberFloatSchema = gonest.NewSchema(func(t *MatchNumber[float64], s *gonest.Schema) {
	s.Title("search.MatchNumberFloat")
	s.Property(&t.Eq).Double()
	s.Property(&t.Gt).Double()
	s.Property(&t.Gte).Double()
	s.Property(&t.Lt).Double()
	s.Property(&t.Lte).Double()
	s.Property(&t.Neq).Double()
	s.Property(&t.Ngt).Double()
	s.Property(&t.Ngte).Double()
	s.Property(&t.Nlt).Double()
	s.Property(&t.Nlte).Double()
})

// Match dispatches each dirty operator on m through its own
// OperatorMatchNumber* const, ANDing every dirty operator together. value is
// a *T (not T) so ONE method covers both a plain numeric field (pass
// &localVar) and a nullable *T column (pass the pointer field directly) --
// a nil value is treated as T's zero value (0).
func (m *MatchNumber[T]) Match(value *T) bool {
	var v T
	if value != nil {
		v = *value
	}
	ops := map[OperatorMatchNumber]gonest.Accessor[T]{
		OperatorMatchNumberEQ:   m.Eq,
		OperatorMatchNumberNEQ:  m.Neq,
		OperatorMatchNumberGT:   m.Gt,
		OperatorMatchNumberNGT:  m.Ngt,
		OperatorMatchNumberGTE:  m.Gte,
		OperatorMatchNumberNGTE: m.Ngte,
		OperatorMatchNumberLT:   m.Lt,
		OperatorMatchNumberNLT:  m.Nlt,
		OperatorMatchNumberLTE:  m.Lte,
		OperatorMatchNumberNLTE: m.Nlte,
	}
	for op, target := range ops {
		if !target.IsDirty() {
			continue
		}
		switch op {
		case OperatorMatchNumberEQ:
			if v != target.Get() {
				return false
			}
		case OperatorMatchNumberNEQ:
			if v == target.Get() {
				return false
			}
		case OperatorMatchNumberGT:
			if !(v > target.Get()) {
				return false
			}
		case OperatorMatchNumberNGT:
			if v > target.Get() {
				return false
			}
		case OperatorMatchNumberGTE:
			if !(v >= target.Get()) {
				return false
			}
		case OperatorMatchNumberNGTE:
			if v >= target.Get() {
				return false
			}
		case OperatorMatchNumberLT:
			if !(v < target.Get()) {
				return false
			}
		case OperatorMatchNumberNLT:
			if v < target.Get() {
				return false
			}
		case OperatorMatchNumberLTE:
			if !(v <= target.Get()) {
				return false
			}
		case OperatorMatchNumberNLTE:
			if v <= target.Get() {
				return false
			}
		}
	}
	return true
}

// #endregion
// #region MatchBool

// MatchBool holds one boolean field's requested match operators as
// gonest.Accessor[bool] -- see MatchString's own doc comment for why.
// Keyed the same way as OperatorMatchBoolean's own const names (eq/neq
// only).
type MatchBool struct {
	Eq  gonest.Accessor[bool] `json:"eq"`
	Neq gonest.Accessor[bool] `json:"neq"`
}

var _ Matcher[bool] = (*MatchBool)(nil)

var MatchBoolSchema = gonest.NewSchema(func(t *MatchBool, s *gonest.Schema) {
	s.Title("search.MatchBool")
	s.Property(&t.Eq).Boolean()
	s.Property(&t.Neq).Boolean()
})

// Match dispatches each dirty operator on m through its own
// OperatorMatchBoolean* const. value is a *bool (not bool) for the same
// single-method-covers-nullable-too reason as MatchString/MatchNumber's own
// Match -- a nil value is treated as false.
func (m *MatchBool) Match(value *bool) bool {
	var v bool
	if value != nil {
		v = *value
	}
	ops := map[OperatorMatchBoolean]gonest.Accessor[bool]{
		OperatorMatchBooleanEQ:  m.Eq,
		OperatorMatchBooleanNEQ: m.Neq,
	}
	for op, target := range ops {
		if !target.IsDirty() {
			continue
		}
		switch op {
		case OperatorMatchBooleanEQ:
			if v != target.Get() {
				return false
			}
		case OperatorMatchBooleanNEQ:
			if v == target.Get() {
				return false
			}
		}
	}
	return true
}

// #endregion
// #region MatchDate

// MatchDate holds one date/time field's requested match operators as
// gonest.Accessor[time.Time] -- see MatchString's own doc comment for why.
// Keyed the same way as OperatorMatchDate's own const names.
type MatchDate struct {
	Eq   gonest.Accessor[time.Time] `json:"eq"`
	Gt   gonest.Accessor[time.Time] `json:"gt"`
	Gte  gonest.Accessor[time.Time] `json:"gte"`
	Lt   gonest.Accessor[time.Time] `json:"lt"`
	Lte  gonest.Accessor[time.Time] `json:"lte"`
	Neq  gonest.Accessor[time.Time] `json:"neq"`
	Ngt  gonest.Accessor[time.Time] `json:"ngt"`
	Ngte gonest.Accessor[time.Time] `json:"ngte"`
	Nlt  gonest.Accessor[time.Time] `json:"nlt"`
	Nlte gonest.Accessor[time.Time] `json:"nlte"`
}

var _ Matcher[time.Time] = (*MatchDate)(nil)

var MatchDateSchema = gonest.NewSchema(func(t *MatchDate, s *gonest.Schema) {
	s.Title("search.MatchDate")
	s.Property(&t.Eq).DateTime()
	s.Property(&t.Gt).DateTime()
	s.Property(&t.Gte).DateTime()
	s.Property(&t.Lt).DateTime()
	s.Property(&t.Lte).DateTime()
	s.Property(&t.Neq).DateTime()
	s.Property(&t.Ngt).DateTime()
	s.Property(&t.Ngte).DateTime()
	s.Property(&t.Nlt).DateTime()
	s.Property(&t.Nlte).DateTime()
})

// Match dispatches each dirty operator on m through its own
// OperatorMatchDate* const. value is a *time.Time (not time.Time) for the
// same single-method reason as MatchString/MatchNumber/MatchBool's own
// Match -- but unlike those, a nil value does NOT fall back to a zero
// time.Time (year 1) for comparison, since a zero date is never a plausible
// real filter value: it only satisfies m when every operator is clean (not
// dirty), i.e. a nullable column left nil (e.g. DeletedAt = "never
// deleted") only matches a Where that didn't ask for any date operator on
// it at all.
func (m *MatchDate) Match(value *time.Time) bool {
	if value == nil {
		return !m.Eq.IsDirty() && !m.Neq.IsDirty() && !m.Gt.IsDirty() && !m.Ngt.IsDirty() &&
			!m.Gte.IsDirty() && !m.Ngte.IsDirty() && !m.Lt.IsDirty() && !m.Nlt.IsDirty() &&
			!m.Lte.IsDirty() && !m.Nlte.IsDirty()
	}
	v := *value
	ops := map[OperatorMatchDate]gonest.Accessor[time.Time]{
		OperatorMatchDateEQ:   m.Eq,
		OperatorMatchDateNEQ:  m.Neq,
		OperatorMatchDateGT:   m.Gt,
		OperatorMatchDateNGT:  m.Ngt,
		OperatorMatchDateGTE:  m.Gte,
		OperatorMatchDateNGTE: m.Ngte,
		OperatorMatchDateLT:   m.Lt,
		OperatorMatchDateNLT:  m.Nlt,
		OperatorMatchDateLTE:  m.Lte,
		OperatorMatchDateNLTE: m.Nlte,
	}
	for op, target := range ops {
		if !target.IsDirty() {
			continue
		}
		switch op {
		case OperatorMatchDateEQ:
			if !v.Equal(target.Get()) {
				return false
			}
		case OperatorMatchDateNEQ:
			if v.Equal(target.Get()) {
				return false
			}
		case OperatorMatchDateGT:
			if !v.After(target.Get()) {
				return false
			}
		case OperatorMatchDateNGT:
			if v.After(target.Get()) {
				return false
			}
		case OperatorMatchDateGTE:
			if v.Before(target.Get()) {
				return false
			}
		case OperatorMatchDateNGTE:
			if !v.Before(target.Get()) {
				return false
			}
		case OperatorMatchDateLT:
			if !v.Before(target.Get()) {
				return false
			}
		case OperatorMatchDateNLT:
			if v.Before(target.Get()) {
				return false
			}
		case OperatorMatchDateLTE:
			if v.After(target.Get()) {
				return false
			}
		case OperatorMatchDateNLTE:
			if !v.After(target.Get()) {
				return false
			}
		}
	}
	return true
}

// #endregion
// #region Fields

// Fields selects/removes top-level output fields. T is a phantom type
// parameter -- it never appears in Fields' own field layout -- used for two
// things: (1) making Fields[Person] and Fields[OtherEntity] distinct Go
// types, since gonest.NewSchema registers one *Schema per reflect.Type and
// two entities need two DIFFERENT enum validations on the exact same
// Select/Remove []string shape; (2) letting FieldsSchemaFor[T] derive that
// enum straight from T's own `json:"..."` tags via FieldNames[T], instead of
// hand-listing an entity's field names a second time.
type Fields[T any] struct {
	Select []string `json:"select"`
	Remove []string `json:"remove"`
}

// FieldNames returns every `json:"name"` tag value declared on T (a
// struct), flattened the same way encoding/json itself promotes anonymous
// embedded fields (e.g. entity.Person's `*Indexable`/`*Creatable`/etc --
// FieldNames recurses into each embedded struct/pointer-to-struct field
// instead of treating it as one opaque field) -- skips untagged fields and
// `json:"-"`. This is read-only reflection over an EXISTING struct's shape
// (safe and cheap, same category of thing schemaFor-style lookups already
// do); it does not attempt to synthesize a new struct type, which is the
// wall SchemaMap's own doc comment already ran into for a full
// reflection-derived Where.
func FieldNames[T any]() []string {
	return collectFieldNames(reflect.TypeFor[T]())
}

func collectFieldNames(t reflect.Type) []string {
	names := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				names = append(names, collectFieldNames(ft)...)
				continue
			}
		}
		tag, ok := f.Tag.Lookup("json")
		if !ok || tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// FieldsSchemaFor builds Fields[T]'s own schema, constraining every
// Select/Remove entry to one of FieldNames[T]() via the real Enum(...)
// (gonest Milestone 21) -- an invalid name (typo, or a field T doesn't
// have) fails validation instead of silently no-op-ing at Search time, and
// shows up in generated OpenAPI docs as a real "enum" array. fn runs AFTER
// Select/Remove are registered, against the SAME (t, s) -- the caller's one
// hook into this schema's own structuring flow (Title, Description, or any
// further Property call), same shape as SortFieldSchemaFor/ResultSchemaFor
// below, deliberately NOT a bare `title string` param (a caller wanting more
// than a title -- e.g. a Description too -- would otherwise have no way to
// reach into a schema this function fully owns and never exposes).
func FieldsSchemaFor[T any](fn func(t *Fields[T], s *gonest.Schema)) *gonest.Schema {
	itemsFn := func(m *gonest.ArraySchema) { m.String().Enum(FieldNames[T]()...) }
	return gonest.NewSchema(func(t *Fields[T], s *gonest.Schema) {
		s.Property(&t.Select).Array().Items(itemsFn)
		s.Property(&t.Remove).Array().Items(itemsFn)
		fn(t, s)
	})
}

// #endregion
// #region Query

// Query is the entity-agnostic shell of a Search request -- mirrors the
// gist's `Partial<SearchText & SearchBlock<B>> & {fields, sort}` (Where is
// deliberately NOT here: it's the one piece that varies per entity's own
// fields/operators, hand-written per entity as e.g. person.Where, and
// composed alongside Query via embedding -- see QuerySchemaFor's own doc
// comment). T is Fields[T]'s own phantom type parameter, threaded through so
// Query[Person, _] and Query[OtherEntity, _] stay distinct types (same
// reasoning as Fields[T]'s own doc comment). W is the entity's own
// hand-written Where type (e.g. person.QueryDTOWhere) -- unlike T, W can't
// be entity-generic itself (each entity's Where has its own field set/
// operators), but THREADING it through Query[T, W] as a second type
// parameter still lets an entity's ENTIRE query DTO collapse to one type
// alias (`type QueryDTO = search.Query[entity.Person, QueryDTOWhere]`)
// instead of a wrapper struct that embeds Query[T] and re-declares its own
// Where field on top.
//
// Text/Offset/Limit live directly on Query (not wrapped in their own
// Text/Pagination sub-structs) specifically so Go source never has to write
// a doubled-up `q.Text.Text`/`q.Pagination.Offset` -- a struct whose type
// name and lone field name collide (`type Text struct{ Text ... }`) forces
// that ugliness the moment it's embedded, since Go's shallowest-match
// promotion rule resolves `q.Text` to the EMBEDDED STRUCT itself before ever
// reaching its inner field of the same name.
type Query[TEntity any, TWhere any] struct {
	Text   gonest.Accessor[*string] `json:"text"`
	Offset gonest.Accessor[*int64]  `json:"offset"`
	Limit  gonest.Accessor[*int64]  `json:"limit"`
	Where  *TWhere                  `json:"where"`
	Fields *Fields[TEntity]         `json:"fields"`
	Sort   []SortField[TEntity]     `json:"sort"`
}

// QuerySchemaFor registers Query[T, W]'s own 6 properties onto s -- called
// from inside an entity's own gonest.NewSchema(func(t *EntityQueryDTO, s
// *gonest.Schema) {...}) callback with q pointing at that same t (e.g.
// `type QueryDTO = search.Query[entity.Person, QueryDTOWhere]` makes t and
// q identical). whereSchema is W's own already-built *gonest.Schema (hand-
// written per entity, same as it always was -- Where has no meaning at this
// generic layer, so QuerySchemaFor only wires the REFERENCE in, it doesn't
// know how to build it).
func QuerySchemaFor[T any, W any](s *gonest.Schema, q *Query[T, W], whereSchema *gonest.Schema) {
	s.Property(&q.Text).String()
	s.Property(&q.Offset).Integer().Min(0)
	s.Property(&q.Limit).Integer().Min(1)
	s.Property(&q.Where).Object(func(om *gonest.ObjectSchema) { om.Schema(whereSchema) })
	s.Property(&q.Fields).Object(func(om *gonest.ObjectSchema) {
		om.Schema(FieldsSchemaFor[T](func(t *Fields[T], s *gonest.Schema) { s.Title("search.Fields") }))
	})
	s.Property(&q.Sort).Array().Items(func(m *gonest.ArraySchema) {
		m.Object(SortFieldSchemaFor[T](func(t *SortField[T], s *gonest.Schema) { s.Title("search.SortField") }))
	})
}

// #endregion
// #region SortField

// SortDirection is the sort value domain (-1/1).
type SortDirection int

const (
	SortDirectionASC  SortDirection = 1
	SortDirectionDESC SortDirection = -1
)

// SortField is one entry of a query's requested sort order -- an
// array-of-objects stand-in for a mapped-type sort, since Go's
// schema.Property can't describe a map keyed by arbitrary field names. T is
// the same phantom type parameter Fields[T] uses, for the same 2 reasons:
// a distinct Go type per entity (so 2 entities can each register their own
// schema for the identical Field/Order shape) and letting
// SortFieldSchemaFor[T] constrain Field to one of FieldNames[T]() via the
// real Enum(...), same as FieldsSchemaFor already does for Select/Remove --
// a typo'd sort field silently sorting nothing (the old default: branch in
// sortLess) is the exact same class of bug Enum was built to catch.
type SortField[T any] struct {
	Field string        `json:"field"`
	Order SortDirection `json:"order"`
}

// SortFieldSchemaFor builds SortField[T]'s own schema -- fn runs AFTER
// Field/Order are registered, against the SAME (t, s), the caller's one
// hook into this schema's own structuring flow (Title, Description, or any
// further Property call) -- see FieldsSchemaFor's own doc comment for why
// this is a callback rather than a bare `title string` param.
func SortFieldSchemaFor[T any](fn func(t *SortField[T], s *gonest.Schema)) *gonest.Schema {
	return gonest.NewSchema(func(t *SortField[T], s *gonest.Schema) {
		s.Property(&t.Field).String().Min(1).Required().Enum(FieldNames[T]()...)
		s.Property(&t.Order).Integer().Required()
		fn(t, s)
	})
}

// #endregion
// #region Result

// Result is the response shape for any Search call.
type Result[I any] struct {
	Items  []I   `json:"items"`
	Total  int64 `json:"total"`
	Offset int64 `json:"offset"`
	Limit  int64 `json:"limit"`
}

func NewResult[I any]() *Result[I] {
	return &Result[I]{Items: []I{}}
}

// ResultSchemaFor builds Result[I]'s own schema for OpenAPI response
// documentation -- itemRef is the ALREADY-BUILT *gonest.Schema for I itself
// (e.g. entity.PersonSchema), reused as-is via Array().Object(ref) rather
// than re-walked. This is documentation-only: Result is never a
// gonest.MustParse/Parse target (Search always CONSTRUCTS one, never
// receives it as a request body), so unlike Fields[T]/SortField[T]/Query[T]
// there is no runtime validation angle here -- purely so a route's own
// r.Response(200, func(r *gonest.RouteResponse) { r.Schema(...) }) can point
// at something real instead of leaving the response body undocumented. fn
// runs AFTER Items/Total/Offset/Limit are registered, against the SAME
// (t, s) -- see FieldsSchemaFor's own doc comment for why this is a
// callback rather than a bare `title string` param.
func ResultSchemaFor[I any](itemRef *gonest.Schema, fn func(t *Result[I], s *gonest.Schema)) *gonest.Schema {
	return gonest.NewSchema(func(t *Result[I], s *gonest.Schema) {
		s.Property(&t.Items).Array().Object(itemRef).Required()
		s.Property(&t.Total).Integer().Required()
		s.Property(&t.Offset).Integer().Required()
		s.Property(&t.Limit).Integer().Required()
		fn(t, s)
	})
}

// #endregion
// #region SchemaMap

// SchemaMap centralizes the primitive-Go-type -> Match*Schema lookup so a
// per-entity Where's own schema (e.g. person.whereSchema) never hardcodes
// search.MatchStringSchema/MatchNumberIntSchema/etc directly -- it looks the
// right one up by the field's own Go type instead. Where itself still has
// to be a hand-written struct (Go generics can't synthesize a new struct
// TYPE from another type's fields at compile time, and gonest.NewSchema's
// Property(&t.Field) needs a real, statically-declared field address --
// see this package's own doc notes), but every entity's Where at least
// shares ONE registration point per primitive instead of repeating it.
//
// MatchNumberIntSchema declares kind="integer" (validate.go rejects a
// non-whole JSON number against it: "expected integer, got non-integer
// number") -- safe to reuse verbatim for int/int32/int64 (Integer()/Int32()
// both set kind="integer", only the OpenAPI format differs).
// MatchNumberFloatSchema declares kind="number" -- reused for float32/
// float64 (Float()/Double() both set kind="number", only the OpenAPI format
// differs), matching every type in the Number constraint. The two are never
// swapped for each other: doing so would either wrongly reject fractional
// values (int schema on a float field) or wrongly accept them (float schema
// on an int field).
//
// A nullable column (*string/*int/.../*time.Time) filters through the exact
// same JSON shape as its non-nullable counterpart -- only the RUNTIME
// comparison differs (MatchNullable treats a nil value as the type's zero
// value instead of Match's direct compare), so each pointer entry below
// points at the SAME *gonest.Schema as its non-pointer counterpart, rather
// than a distinct type.
var SchemaMap = map[reflect.Type]*gonest.Schema{
	reflect.TypeFor[string]():     MatchStringSchema,
	reflect.TypeFor[*string]():    MatchStringSchema,
	reflect.TypeFor[bool]():       MatchBoolSchema,
	reflect.TypeFor[*bool]():      MatchBoolSchema,
	reflect.TypeFor[int]():        MatchNumberIntSchema,
	reflect.TypeFor[*int]():       MatchNumberIntSchema,
	reflect.TypeFor[int32]():      MatchNumberIntSchema,
	reflect.TypeFor[*int32]():     MatchNumberIntSchema,
	reflect.TypeFor[int64]():      MatchNumberIntSchema,
	reflect.TypeFor[*int64]():     MatchNumberIntSchema,
	reflect.TypeFor[float32]():    MatchNumberFloatSchema,
	reflect.TypeFor[*float32]():   MatchNumberFloatSchema,
	reflect.TypeFor[float64]():    MatchNumberFloatSchema,
	reflect.TypeFor[*float64]():   MatchNumberFloatSchema,
	reflect.TypeFor[time.Time]():  MatchDateSchema,
	reflect.TypeFor[*time.Time](): MatchDateSchema,
}

// #endregion
