package accessor_test

import (
	"testing"
	"time"

	"gonest.dev/gonest/internal/accessor"
)

type PersonProps struct {
	Name      accessor.Accessor[string]    `json:"name"`
	Age       accessor.Accessor[int]       `json:"age"`
	BirthDate accessor.Accessor[time.Time] `json:"birth_date"`
}

type PersonEntity struct {
	PersonProps
	Status string
}

type RawPerson struct {
	Name string `json:"name"`
	Age  *int   `json:"age"`
}

func TestSyncAccessorFields_AccessorToAccessor(t *testing.T) {
	dstProps := PersonProps{
		Name: accessor.New("old name"),
		Age:  accessor.New(20),
	}
	dst := &PersonEntity{PersonProps: dstProps}

	srcProps := PersonProps{
		Name: accessor.New("new name"),
	}

	accessor.SyncAccessorFields(dst, &srcProps)

	if dst.Name.Get() != "new name" {
		t.Errorf("expected Name to be 'new name', got %q", dst.Name.Get())
	}
	if !dst.Name.IsDirty() {
		t.Errorf("expected Name to be dirty in dst")
	}
	if dst.Age.Get() != 20 {
		t.Errorf("expected Age to remain 20, got %d", dst.Age.Get())
	}
}

func TestSyncAccessorFields_AccessorToRawAndPointer(t *testing.T) {
	dst := &RawPerson{
		Name: "old name",
	}

	srcProps := PersonProps{
		Name: accessor.New("new name"),
		Age:  accessor.New(30),
	}

	accessor.SyncAccessorFields(dst, &srcProps)

	if dst.Name != "new name" {
		t.Errorf("expected Name to be 'new name', got %q", dst.Name)
	}
	if dst.Age == nil || *dst.Age != 30 {
		t.Errorf("expected Age pointer to be 30, got %v", dst.Age)
	}
}

func TestSyncAccessorFields_NilOrNonStructInputs(t *testing.T) {
	// Should not panic on nil or primitive inputs
	accessor.SyncAccessorFields(nil, nil)
	var x int = 10
	accessor.SyncAccessorFields(&x, &x)
}
