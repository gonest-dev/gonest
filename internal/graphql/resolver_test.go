package graphql

import (
	"testing"

	"gonest.dev/gonest/internal/module"
)

var _ module.ResolverRef = (*Resolver)(nil)
var _ module.Owner = (*Resolver)(nil)

func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	executed := false

	r := New(func(r *Resolver) {
		executed = true
	})

	if executed {
		t.Fatalf("New(fn) executed fn synchronously, want deferred until bootstrap runs it")
	}
	if r == nil {
		t.Fatalf("New(fn) returned nil *Resolver")
	}
}

func TestResolver_SatisfiesModuleOwner(t *testing.T) {
	var owner module.Owner = New(func(r *Resolver) {})
	if owner == nil {
		t.Fatalf("*Resolver does not satisfy module.Owner")
	}
}

func TestOwnerModule_PopulatedAfterSetOwnerModule(t *testing.T) {
	r := New(func(r *Resolver) {})

	m := module.New(func(m *module.Module) {})
	r.SetOwnerModule(m)

	if got := r.OwnerModule(); got != m {
		t.Fatalf("OwnerModule() = %v, want %v", got, m)
	}
}

func TestOwnerModule_NilBeforeAssociation(t *testing.T) {
	r := New(func(r *Resolver) {})

	if got := r.OwnerModule(); got != nil {
		t.Fatalf("OwnerModule() = %v, want nil before any module associates this resolver", got)
	}
}

func TestDeclare_ExecutesFn(t *testing.T) {
	executed := false
	r := New(func(r *Resolver) {
		executed = true
	})

	r.Declare()

	if !executed {
		t.Fatalf("Declare() did not execute fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	r := New(func(r *Resolver) {
		count++
	})

	r.Declare()
	r.Declare()
	r.Declare()

	if count != 1 {
		t.Fatalf("fn executed %d times across 3 Declare() calls, want exactly 1", count)
	}
}

func TestResolver_QueryMutationSubscription_AccumulateAndRoundTrip(t *testing.T) {
	r := New(func(r *Resolver) {
		r.Query("hello", func(q *Query) {
			q.Handler(func(ctx *Context) any { return "world" })
		})
		r.Mutation("createUser", func(m *Mutation) {
			m.Handler(func(ctx *Context) any { return nil })
		})
		r.Subscription("onUserCreated", func(s *Subscription) {
			s.Handler(func(ctx *Context, emit func(any)) {})
		})
	})
	r.Declare()

	queries := r.OwnQueries()
	if len(queries) != 1 || queries[0].Name() != "hello" {
		t.Fatalf("OwnQueries() = %+v, want 1 entry named 'hello'", queries)
	}
	if got := queries[0].HandlerFunc()(nil); got != "world" {
		t.Fatalf("query handler returned %v, want %q", got, "world")
	}

	mutations := r.OwnMutations()
	if len(mutations) != 1 || mutations[0].Name() != "createUser" {
		t.Fatalf("OwnMutations() = %+v, want 1 entry named 'createUser'", mutations)
	}

	subs := r.OwnSubscriptions()
	if len(subs) != 1 || subs[0].Name() != "onUserCreated" {
		t.Fatalf("OwnSubscriptions() = %+v, want 1 entry named 'onUserCreated'", subs)
	}
	if subs[0].HandlerFunc() == nil {
		t.Fatal("subscription handler was not stored")
	}
}
