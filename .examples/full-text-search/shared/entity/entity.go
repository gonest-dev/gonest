package entity

import (
	"time"

	"github.com/google/uuid"
	"gonest.dev/gonest"
)

// #region Indexable

type Indexable struct {
	ID string `json:"id"`
}

func NewIndexable() Indexable {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return Indexable{ID: id.String()}
}

// #endregion
// #region Creatable

type Creatable struct {
	CreatedAt time.Time `json:"created_at"`
}

func NewCreatable() Creatable {
	return Creatable{CreatedAt: time.Now()}
}

// #endregion
// #region Updatable

type Updatable struct {
	UpdatedAt time.Time `json:"updated_at"`
}

func NewUpdatable() Updatable {
	return Updatable{UpdatedAt: time.Now()}
}

// #endregion
// #region Deletable

type Deletable struct {
	DeletedAt *time.Time `json:"deleted_at"`
}

func NewDeletable() Deletable {
	return Deletable{DeletedAt: nil}
}

// #endregion
// #region Person

type PersonProps struct {
	Name     gonest.Accessor[string]  `json:"name"`
	Age      gonest.Accessor[int]     `json:"age"`
	IsActive gonest.Accessor[bool]    `json:"is_active"`
	Picture  gonest.Accessor[*string] `json:"picture"`
}

func NewPersonProps() PersonProps {
	return PersonProps{
		Name:     gonest.NewAccessor("noname"),
		Age:      gonest.NewAccessor(0),
		IsActive: gonest.NewAccessor(true),
		Picture:  gonest.NewAccessor[*string](nil),
	}
}

// Person embeds its 5 mixins BY VALUE (not pointer) -- a pointer embed's
// promoted fields (e.g. `t.ID` meaning `t.Indexable.ID`) are unaddressable
// through a nil pointer, and gonest.NewSchema's `var zero T` always starts
// with every embedded pointer nil, so `s.Property(&t.ID)` would nil-panic
// the instant PersonSchema (below) tried to register it -- empirically
// confirmed before choosing value embeds over pointer embeds here. Value
// embeds have no such nil state, so PersonSchema can describe Person's real
// (flat, encoding/json-promoted) field set directly via Property(), with no
// nested Object(ref) workaround needed.
type Person struct {
	Indexable
	Creatable
	Updatable
	Deletable
	PersonProps
}

func NewPerson(props ...PersonProps) *Person {
	i := &Person{
		Indexable:   NewIndexable(),
		Creatable:   NewCreatable(),
		Updatable:   NewUpdatable(),
		Deletable:   NewDeletable(),
		PersonProps: NewPersonProps(),
	}
	if len(props) > 0 {
		p := props[0]
		p.Name.Sync(&i.PersonProps.Name)
		p.Age.Sync(&i.PersonProps.Age)
		p.IsActive.Sync(&i.PersonProps.IsActive)
		p.Picture.Sync(&i.PersonProps.Picture)
	}
	return i
}

// PersonSchema describes Person's real (flat) JSON response shape --
// id/created_at/updated_at/deleted_at/name/age/is_active/picture all at the
// top level, exactly matching what res.Json(person) actually serializes
// (encoding/json promotes every value-embedded mixin's own fields the same
// way). Used as the documented response body for every person.Controller
// route that returns a Person.
var PersonSchema = gonest.NewSchema(func(t *Person, s *gonest.Schema) {
	s.Title("PersonEntity")
	s.Property(&t.ID).String().Required()
	s.Property(&t.CreatedAt).DateTime().Required()
	s.Property(&t.UpdatedAt).DateTime().Required()
	s.Property(&t.DeletedAt).DateTime().Nullable()
	s.Property(&t.Name).String().Required()
	s.Property(&t.Age).Integer().Required()
	s.Property(&t.IsActive).Boolean().Required()
	s.Property(&t.Picture).String().Nullable()
})

// #endregion
