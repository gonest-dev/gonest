package value_test

import (
	"encoding/json"
	"testing"

	"gonest.dev/gonest/internal/value"
)

// --- Accessor[T] core behaviour ---

func TestNew_WithoutArgs_NotDirty(t *testing.T) {
	v := value.New[string]()
	if v.IsDirty() {
		t.Fatal("New() without args: want dirty=false, got true")
	}
	var zero string
	if v.Get() != zero {
		t.Fatalf("New() without args: want zero value, got %v", v.Get())
	}
}

func TestNew_WithArg_DirtyAndValueSet(t *testing.T) {
	v := value.New("hello")
	if !v.IsDirty() {
		t.Fatal("New(val): want dirty=true, got false")
	}
	if v.Get() != "hello" {
		t.Fatalf("New(val): want %q, got %q", "hello", v.Get())
	}
}

func TestSet_MarksDirtyAndStoresValue(t *testing.T) {
	var v value.Accessor[int]
	v.Set(42)
	if !v.IsDirty() {
		t.Fatal("Set(): want dirty=true, got false")
	}
	if v.Get() != 42 {
		t.Fatalf("Set(): want 42, got %d", v.Get())
	}
}

func TestGetAny_ReturnsValueAsAny(t *testing.T) {
	v := value.New(99)
	if v.GetAny() != 99 {
		t.Fatalf("GetAny(): want 99, got %v", v.GetAny())
	}
}

func TestOnDirty_CalledOnlyWhenDirty(t *testing.T) {
	called := false
	var clean value.Accessor[string]
	clean.OnDirty(func(s string) { called = true })
	if called {
		t.Fatal("OnDirty on clean value: callback must not be called")
	}

	dirty := value.New("x")
	dirty.OnDirty(func(s string) { called = true })
	if !called {
		t.Fatal("OnDirty on dirty value: callback must be called")
	}
}

func TestApply_WritesOnlyWhenDirty(t *testing.T) {
	target := "original"

	var clean value.Accessor[string]
	clean.Apply(&target)
	if target != "original" {
		t.Fatalf("Apply on clean value: target must not change, got %q", target)
	}

	dirty := value.New("updated")
	dirty.Apply(&target)
	if target != "updated" {
		t.Fatalf("Apply on dirty value: want %q, got %q", "updated", target)
	}
}

func TestSync_WritesOnlyWhenDirty(t *testing.T) {
	target := value.New("original")

	var clean value.Accessor[string]
	clean.Sync(&target)
	if target.Get() != "original" {
		t.Fatalf("Sync on clean value: target must not change, got %q", target.Get())
	}

	dirty := value.New("updated")
	dirty.Sync(&target)
	if target.Get() != "updated" {
		t.Fatalf("Sync on dirty value: want %q, got %q", "updated", target.Get())
	}
	if !target.IsDirty() {
		t.Fatal("Sync on dirty value: target must be marked dirty")
	}
}

func TestSync_MarksDestDirtyEvenIfDestWasClean(t *testing.T) {
	var dest value.Accessor[int]
	src := value.New(5)
	src.Sync(&dest)
	if !dest.IsDirty() {
		t.Fatal("Sync: dest must become dirty after sync from dirty src")
	}
	if dest.Get() != 5 {
		t.Fatalf("Sync: want 5, got %d", dest.Get())
	}
}

// --- JSON integration ---

func TestMarshalJSON_EmitsInnerValueDirectly(t *testing.T) {
	v := value.New("world")
	b, err := json.Marshal(&v)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}
	if string(b) != `"world"` {
		t.Fatalf("MarshalJSON: want %q, got %q", `"world"`, string(b))
	}
}

func TestUnmarshalJSON_SetsDirtyAndValue(t *testing.T) {
	var v value.Accessor[int]
	if err := json.Unmarshal([]byte(`7`), &v); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if !v.IsDirty() {
		t.Fatal("UnmarshalJSON: want dirty=true, got false")
	}
	if v.Get() != 7 {
		t.Fatalf("UnmarshalJSON: want 7, got %d", v.Get())
	}
}

func TestUnmarshalJSON_NullSetsDirty(t *testing.T) {
	var v value.Accessor[string]
	if err := json.Unmarshal([]byte(`null`), &v); err != nil {
		t.Fatalf("UnmarshalJSON null error: %v", err)
	}
	if !v.IsDirty() {
		t.Fatal("UnmarshalJSON null: want dirty=true (field was explicitly present), got false")
	}
}

func TestUnmarshalJSON_OmittedFieldStaysClean(t *testing.T) {
	type patch struct {
		Name  value.Accessor[string] `json:"name"`
		Email value.Accessor[string] `json:"email"`
	}
	var p patch
	if err := json.Unmarshal([]byte(`{"name":"alice"}`), &p); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if !p.Name.IsDirty() {
		t.Fatal("Name: want dirty=true")
	}
	if p.Email.IsDirty() {
		t.Fatal("Email (omitted): want dirty=false, got true")
	}
}

// --- ToMap ---

type patchUser struct {
	Name  value.Accessor[string] `json:"name"`
	Age   value.Accessor[int]    `json:"age,omitempty"`
	Email value.Accessor[string] `json:"-"`
	NoTag value.Accessor[bool]
	Fixed string `json:"fixed_data"`
}

func TestToDirtyMap_IncludesOnlyDirtyFields(t *testing.T) {
	u := patchUser{}
	u.Name.Set("leandro")
	u.Email.Set("leandro@example.com")
	u.NoTag.Set(true)

	m := value.ToDirtyMap(u)

	if m["name"] != "leandro" {
		t.Fatalf("name: want %q, got %v", "leandro", m["name"])
	}
	if m["Email"] != "leandro@example.com" {
		t.Fatalf("Email (tag=-): want fallback to field name, got %v", m["Email"])
	}
	if m["NoTag"] != true {
		t.Fatalf("NoTag: want true, got %v", m["NoTag"])
	}
	if len(m) != 3 {
		t.Fatalf("ToMap: want 3 entries, got %d: %v", len(m), m)
	}
}

func TestToMap_PointerToStruct(t *testing.T) {
	u := &patchUser{}
	u.Age.Set(30)
	m := value.ToDirtyMap(u)
	if m["age"] != 30 {
		t.Fatalf("age: want 30, got %v", m["age"])
	}
}

func TestToMap_NonStructReturnsEmpty(t *testing.T) {
	m := value.ToDirtyMap(123)
	if len(m) != 0 {
		t.Fatalf("ToMap(int): want empty map, got %v", m)
	}
}
