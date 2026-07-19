package validate

import (
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// --- T10 fixtures ---------------------------------------------------------

// EnvAllFieldsFixture exercises 4 env-tagged fields of mixed kinds,
// none Required/Default -- proving ParseEnvInto populates an instance
// matching every currently-set env var exactly.
type EnvAllFieldsFixture struct {
	Host   string `env:"ENV_T10_HOST"`
	Port   int64  `env:"ENV_T10_PORT"`
	Debug  bool   `env:"ENV_T10_DEBUG"`
	Region string `env:"ENV_T10_REGION"`
	NoTag  string // deliberately no `env:"..."` tag at all
}

var envAllFieldsSchema = func() *schema.Schema {
	f := &EnvAllFieldsFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Host).String()
	m.Property(&f.Port).Integer()
	m.Property(&f.Debug).Boolean()
	m.Property(&f.Region).String()
	return m
}()

// EnvTwoRequiredFixture declares 2 Required fields with NO Default -- used
// to prove collect-all records both violations when neither var is set.
type EnvTwoRequiredFixture struct {
	Host string `env:"ENV_T10_REQ_HOST"`
	Port int64  `env:"ENV_T10_REQ_PORT"`
}

var envTwoRequiredSchema = func() *schema.Schema {
	f := &EnvTwoRequiredFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Host).String().Required()
	m.Property(&f.Port).Integer().Required()
	return m
}()

// EnvDefaultFixture declares one field with a Default -- used to prove the
// default is used when the var is absent, and the real value is used when
// present (even if the real value differs from the default).
type EnvDefaultFixture struct {
	Host string `env:"ENV_T10_DEFAULT_HOST"`
}

var envDefaultSchema = func() *schema.Schema {
	f := &EnvDefaultFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Host).String().Default("127.0.0.1")
	return m
}()

// EnvEmptyButSetFixture is used to prove an empty-but-set env var is
// treated as PRESENT (goes through normal coercion), not absent.
type EnvEmptyButSetFixture struct {
	Host string `env:"ENV_T10_EMPTY_HOST"`
}

var envEmptyButSetSchema = func() *schema.Schema {
	f := &EnvEmptyButSetFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Host).String().Default("should-not-be-used")
	return m
}()

// --- unit tests -------------------------------------------------------

func TestParseEnvInto_AllFieldsSet_PopulatesCorrectly(t *testing.T) {
	t.Setenv("ENV_T10_HOST", "db.internal")
	t.Setenv("ENV_T10_PORT", "5432")
	t.Setenv("ENV_T10_DEBUG", "true")
	t.Setenv("ENV_T10_REGION", "us-east-1")

	var dst EnvAllFieldsFixture
	if err := ParseEnvInto(&dst, envAllFieldsSchema); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if dst.Host != "db.internal" {
		t.Fatalf("expected Host %q, got %q", "db.internal", dst.Host)
	}
	if dst.Port != 5432 {
		t.Fatalf("expected Port 5432, got %d", dst.Port)
	}
	if dst.Debug != true {
		t.Fatalf("expected Debug true, got %v", dst.Debug)
	}
	if dst.Region != "us-east-1" {
		t.Fatalf("expected Region %q, got %q", "us-east-1", dst.Region)
	}
}

func TestParseEnvInto_IntegerCoercionFails_RecordsViolation(t *testing.T) {
	t.Setenv("ENV_T10_HOST", "db.internal")
	t.Setenv("ENV_T10_PORT", "not-a-number")
	t.Setenv("ENV_T10_DEBUG", "true")
	t.Setenv("ENV_T10_REGION", "us-east-1")

	var dst EnvAllFieldsFixture
	err := ParseEnvInto(&dst, envAllFieldsSchema)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	exc := expectBadRequest(t, err)
	vs := violationsOf(t, exc)
	if !hasFieldViolation(vs, "ENV_T10_PORT") {
		t.Fatalf("expected a violation for field %q, got %+v", "ENV_T10_PORT", vs)
	}
}

func TestParseEnvInto_TwoRequiredMissing_CollectsBothViolations(t *testing.T) {
	// Neither ENV_T10_REQ_HOST nor ENV_T10_REQ_PORT is set.
	var dst EnvTwoRequiredFixture
	err := ParseEnvInto(&dst, envTwoRequiredSchema)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	exc := expectBadRequest(t, err)
	vs := violationsOf(t, exc)
	if len(vs) != 2 {
		t.Fatalf("expected exactly 2 violations (collect-all), got %d: %+v", len(vs), vs)
	}
	if !hasFieldViolation(vs, "ENV_T10_REQ_HOST") {
		t.Fatalf("expected a violation for field %q, got %+v", "ENV_T10_REQ_HOST", vs)
	}
	if !hasFieldViolation(vs, "ENV_T10_REQ_PORT") {
		t.Fatalf("expected a violation for field %q, got %+v", "ENV_T10_REQ_PORT", vs)
	}
}

func TestParseEnvInto_DefaultUsedWhenAbsent_RealValueUsedWhenPresent(t *testing.T) {
	// Absent: resolves to the Default.
	var dstAbsent EnvDefaultFixture
	if err := ParseEnvInto(&dstAbsent, envDefaultSchema); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dstAbsent.Host != "127.0.0.1" {
		t.Fatalf("expected Default value %q, got %q", "127.0.0.1", dstAbsent.Host)
	}

	// Present: resolves to the real env var value, NOT the Default.
	t.Setenv("ENV_T10_DEFAULT_HOST", "10.0.0.5")
	var dstPresent EnvDefaultFixture
	if err := ParseEnvInto(&dstPresent, envDefaultSchema); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dstPresent.Host != "10.0.0.5" {
		t.Fatalf("expected real env value %q, got %q", "10.0.0.5", dstPresent.Host)
	}
}

func TestParseEnvInto_EmptyButSetEnvVar_TreatedAsPresent(t *testing.T) {
	t.Setenv("ENV_T10_EMPTY_HOST", "")

	var dst EnvEmptyButSetFixture
	if err := ParseEnvInto(&dst, envEmptyButSetSchema); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Present-but-empty must NOT trigger Default -- it goes through the
	// normal coercion path and resolves to the empty string.
	if dst.Host != "" {
		t.Fatalf("expected empty string (not the Default), got %q", dst.Host)
	}
}

func TestParseEnvInto_FieldWithoutEnvTag_Ignored(t *testing.T) {
	t.Setenv("ENV_T10_HOST", "db.internal")
	t.Setenv("ENV_T10_PORT", "5432")
	t.Setenv("ENV_T10_DEBUG", "true")
	t.Setenv("ENV_T10_REGION", "us-east-1")

	var dst EnvAllFieldsFixture
	if err := ParseEnvInto(&dst, envAllFieldsSchema); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// NoTag was never registered as a property at all (no m.Property call
	// for it), so it stays at its Go zero value regardless of env vars.
	if dst.NoTag != "" {
		t.Fatalf("expected NoTag to stay zero-value, got %q", dst.NoTag)
	}
}
