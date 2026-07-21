// Package person implements the Person domain: CRUD + a generic search
// endpoint (QUERY /person) built on top of shared/search's Match/Fields/
// SortField/Result types.
package person

import (
	"reflect"
	"time"

	"gonest.dev/gonest"

	"full-text-search/shared/entity"
	"full-text-search/shared/search"
)

// ParamsDTO is the :person_id route param, shared by GET/PUT/DELETE.
type ParamsDTO struct {
	PersonID string `param:"person_id"`
}

var ParamsDTOSchema = gonest.NewSchema(func(t *ParamsDTO, s *gonest.Schema) {
	s.Title("person.ParamsDTO")
	s.Property(&t.PersonID).String().Min(1).Required()
})

// BodyCreateDTO mirrors entity.PersonProps field-for-field (same Accessor
// fields, same dirty-tracking) as its own DEFINED type -- not a type alias --
// because gonest.NewSchema registers one *Schema per reflect.Type, and
// UpdateBodyDTO below needs a DIFFERENT (all-optional) rule set for the
// exact same field shape; two NewSchema calls for the same aliased type
// would panic ("already registered"). Convertible to entity.PersonProps via
// a plain Go conversion (see service.go's Create).
type BodyCreateDTO entity.PersonProps

var BodyCreateDTOSchema = gonest.NewSchema(func(t *BodyCreateDTO, s *gonest.Schema) {
	s.Title("person.CreateBodyDTO")
	s.Property(&t.Name).String().Min(1).Required()
	s.Property(&t.Age).Integer().Min(0).Required()
	s.Property(&t.IsActive).Boolean().Required()
	s.Property(&t.Picture).String().Nullable()
})

// BodyUpdateDTO is PATCH-style despite riding on PUT: only fields present in
// the JSON payload (Accessor.IsDirty()) are applied to the stored entity. Its
// own defined type for the same reason as CreateBodyDTO's doc comment.
type BodyUpdateDTO entity.PersonProps

var BodyUpdateDTOSchema = gonest.NewSchema(func(t *BodyUpdateDTO, s *gonest.Schema) {
	s.Title("person.UpdateBodyDTO")
	s.Property(&t.Name).String().Min(1)
	s.Property(&t.Age).Integer().Min(0)
	s.Property(&t.IsActive).Boolean()
	s.Property(&t.Picture).String().Nullable()
})

// objectSchemaFor is a thin wrapper over search.SchemaMap[reflect.TypeFor[V]()] --
// every Where field below looks its nested Match*Schema up by the Go type
// it filters (string/int/bool/time.Time) instead of hardcoding
// search.MatchStringSchema/MatchNumberIntSchema/etc directly, so adding a
// new primitive to search.SchemaMap is the only change needed to support it
// here too.
func objectSchemaFor[T any]() func(*gonest.ObjectSchema) {
	return func(om *gonest.ObjectSchema) { om.Schema(search.SchemaMap[reflect.TypeFor[T]()]) }
}

// QueryDTOWhere holds every Person field's requested filter -- Accessor[Match*]
// instead of a plain pointer so presence is checked the same dirty-tracking
// way CreateBodyDTO/UpdateBodyDTO already use, even though nothing here is
// ever written back to an entity.
type QueryDTOWhere struct {
	ID        gonest.Accessor[search.MatchString[string]] `json:"id"`
	Name      gonest.Accessor[search.MatchString[string]] `json:"name"`
	Age       gonest.Accessor[search.MatchNumber[int]]    `json:"age"`
	IsActive  gonest.Accessor[search.MatchBool]           `json:"is_active"`
	Picture   gonest.Accessor[search.MatchString[string]] `json:"picture"`
	CreatedAt gonest.Accessor[search.MatchDate]           `json:"created_at"`
	UpdatedAt gonest.Accessor[search.MatchDate]           `json:"updated_at"`
	DeletedAt gonest.Accessor[search.MatchDate]           `json:"deleted_at"`
}

// QueryDTO is the QUERY /person body -- mirrors the gist's
// `Partial<SearchText & SearchBlock<B>> & {where, fields, sort}` shape. A
// plain type alias of search.Query[entity.Person, QueryDTOWhere] -- Where
// now lives directly on Query itself (search.Query's own 2nd type
// parameter), so there's no wrapper struct left to declare here at all.
type QueryDTO = search.Query[entity.Person, QueryDTOWhere]

var QueryDTOSchema = gonest.NewSchema(func(t *QueryDTO, s *gonest.Schema) {
	s.Title("person.QueryDTO")
	// queryDTOWhereSchema is QueryDTOWhere's own schema, built once so
	// QueryDTOSchema (below) can pass it straight into search.QuerySchemaFor.
	search.QuerySchemaFor(s, t, gonest.NewSchema(func(t *QueryDTOWhere, s *gonest.Schema) {
		s.Title("person.QueryDTOWhere")
		s.Property(&t.ID).Object(objectSchemaFor[string]())
		s.Property(&t.Name).Object(objectSchemaFor[string]())
		s.Property(&t.Age).Object(objectSchemaFor[int]())
		s.Property(&t.IsActive).Object(objectSchemaFor[bool]())
		s.Property(&t.Picture).Object(objectSchemaFor[*string]())
		s.Property(&t.CreatedAt).Object(objectSchemaFor[time.Time]())
		s.Property(&t.UpdatedAt).Object(objectSchemaFor[time.Time]())
		s.Property(&t.DeletedAt).Object(objectSchemaFor[*time.Time]())
	}))
})
