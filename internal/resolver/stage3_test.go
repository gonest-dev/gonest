package resolver

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
)

// --- fixtures -------------------------------------------------------------

type stage3AService struct{ Value string }
type stage3BService struct{ Value string }
type stage3DependentService struct {
	Dep *stage3AService
}

// --- independence / parallelism -------------------------------------------

// TestResolve_TwoIndependentProviders_RunConcurrently proves Stage 3 does
// not serialize independent providers: both Constructors must be inside
// their critical section (blocked on a shared start gate) at the same time,
// which is only possible if their goroutines are running concurrently, not
// sequentially.
func TestResolve_TwoIndependentProviders_RunConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)

	release := make(chan struct{})

	a := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *stage3AService {
			wg.Done()
			<-release
			return &stage3AService{Value: "a"}
		})
	})
	b := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *stage3BService {
			wg.Done()
			<-release
			return &stage3BService{Value: "b"}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(a, b)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	a.Declare()
	b.Declare()

	done := make(chan error, 1)
	go func() {
		done <- Resolve(context.Background(), []*module.Module{m})
	}()

	// Both Constructors must reach wg.Done() (i.e. start running) before
	// either can finish -- proof of true concurrency, not serialization.
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// success: both started concurrently
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for both independent providers' Constructors to start concurrently -- looks serialized")
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

// --- dependency ordering ---------------------------------------------------

// TestResolve_DependentProvider_WaitsForDependencyDone proves a provider
// with a dependency does not invoke its own Constructor until the
// dependency's Constructor has completed.
func TestResolve_DependentProvider_WaitsForDependencyDone(t *testing.T) {
	var depFinished atomic.Bool
	var dependentSawDepFinished atomic.Bool

	a := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *stage3AService {
			time.Sleep(50 * time.Millisecond)
			depFinished.Store(true)
			return &stage3AService{Value: "a"}
		})
	})

	var dependent *provider.Provider
	dependent = provider.New(func(p *provider.Provider) {
		dep := inject.Must[*stage3AService](dependent)
		p.Constructor(func() *stage3DependentService {
			dependentSawDepFinished.Store(depFinished.Load())
			return &stage3DependentService{Dep: dep}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(a, dependent)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	a.Declare()
	dependent.Declare()

	if err := Resolve(context.Background(), []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !dependentSawDepFinished.Load() {
		t.Fatalf("dependent's Constructor ran before dependency's Constructor finished")
	}
}

// --- ctx timeout -------------------------------------------------------

// TestResolve_ConstructorReceivesConfiguredContext proves a
// func(context.Context) T Constructor receives the same ctx (with its
// configured deadline) passed to Resolve, not a fresh background context.
func TestResolve_ConstructorReceivesConfiguredContext(t *testing.T) {
	var gotDeadline time.Time
	var hadDeadline bool

	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func(ctx context.Context) *stage3AService {
			gotDeadline, hadDeadline = ctx.Deadline()
			return &stage3AService{}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	p.Declare()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Resolve(ctx, []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !hadDeadline {
		t.Fatalf("Constructor's ctx had no deadline, want the bootstrap timeout propagated")
	}
	if gotDeadline.IsZero() {
		t.Fatalf("Constructor's ctx deadline is zero")
	}
}

// --- error cancels siblings -------------------------------------------------------

// TestResolve_ConstructorError_CancelsSiblingGoroutines proves a Constructor
// returning an error cancels the shared context, and a sibling goroutine
// still mid-flight observes cancellation instead of running to completion.
func TestResolve_ConstructorError_CancelsSiblingGoroutines(t *testing.T) {
	siblingStarted := make(chan struct{})
	var siblingCompleted atomic.Bool
	var siblingSawCancel atomic.Bool

	failing := provider.New(func(p *provider.Provider) {
		p.Constructor(func() (*stage3AService, error) {
			<-siblingStarted
			// give the sibling a moment to be blocked on ctx.Done()
			time.Sleep(20 * time.Millisecond)
			return nil, errors.New("boom")
		})
	})

	sibling := provider.New(func(p *provider.Provider) {
		p.Constructor(func(ctx context.Context) *stage3BService {
			close(siblingStarted)
			select {
			case <-ctx.Done():
				siblingSawCancel.Store(true)
				return &stage3BService{}
			case <-time.After(2 * time.Second):
				siblingCompleted.Store(true)
				return &stage3BService{}
			}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(failing, sibling)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	failing.Declare()
	sibling.Declare()

	err := Resolve(context.Background(), []*module.Module{m})
	if err == nil {
		t.Fatalf("Resolve() error = nil, want error from failing Constructor")
	}
	if !siblingSawCancel.Load() {
		t.Fatalf("sibling goroutine did not observe ctx cancellation")
	}
	if siblingCompleted.Load() {
		t.Fatalf("sibling goroutine ran to completion instead of being cancelled")
	}
}

// --- panic recovery -------------------------------------------------------

// TestResolve_ConstructorPanic_IsRecoveredAsError proves a panicking
// Constructor is recovered and surfaced as an error from Resolve, without
// crashing the test process.
func TestResolve_ConstructorPanic_IsRecoveredAsError(t *testing.T) {
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *stage3AService {
			panic("constructor exploded")
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	p.Declare()

	err := Resolve(context.Background(), []*module.Module{m})
	if err == nil {
		t.Fatalf("Resolve() error = nil, want recovered panic surfaced as error")
	}
}

// --- copy-in-place -------------------------------------------------------

// TestResolve_CopyInPlace_PlaceholderReflectsRealData proves the placeholder
// MustInject returned earlier is mutated in place once Resolve completes.
func TestResolve_CopyInPlace_PlaceholderReflectsRealData(t *testing.T) {
	a := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *stage3AService {
			return &stage3AService{Value: "real-a"}
		})
	})

	var placeholder *stage3AService
	var dependent *provider.Provider
	dependent = provider.New(func(p *provider.Provider) {
		placeholder = inject.Must[*stage3AService](dependent)
		p.Constructor(func() *stage3DependentService {
			return &stage3DependentService{}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(a, dependent)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	a.Declare()
	dependent.Declare()

	if err := Resolve(context.Background(), []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if placeholder == nil {
		t.Fatalf("placeholder is nil")
	}
	if placeholder.Value != "real-a" {
		t.Fatalf("placeholder.Value = %q after Resolve(), want %q (copy-in-place did not happen)", placeholder.Value, "real-a")
	}
}

// TestResolve_DoesNotCrossContaminateWithUnrelatedPendingEdges proves
// Resolve scopes graph-building to the module tree it was given: pending
// edges recorded by a completely unrelated, previously-resolved module tree
// (e.g. from an earlier NewApp call, or MustInject calls in another
// package-level var elsewhere in the process) must not leak into this
// Resolve call's cycle detection or dependency graph. Without scoping,
// internal/inject's process-global pending-edge bookkeeping would make two
// independent NewApp calls in the same process interfere with each other.
func TestResolve_DoesNotCrossContaminateWithUnrelatedPendingEdges(t *testing.T) {
	type unrelatedX struct{}
	type unrelatedY struct{}

	var unrelatedA, unrelatedB *provider.Provider
	unrelatedA = provider.New(func(p *provider.Provider) {
		inject.Must[*unrelatedY](unrelatedA)
		p.Constructor(func() *unrelatedX { return &unrelatedX{} })
	})
	unrelatedB = provider.New(func(p *provider.Provider) {
		inject.Must[*unrelatedX](unrelatedB)
		p.Constructor(func() *unrelatedY { return &unrelatedY{} })
	})
	unrelatedModule := module.New(func(m *module.Module) {
		m.Providers(unrelatedA, unrelatedB)
	})
	if _, err := unrelatedModule.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	unrelatedA.Declare()
	unrelatedB.Declare()
	// Deliberately NOT resolved -- these pending edges (A <-> B, a genuine
	// cycle) sit in internal/inject's global bookkeeping, unrelated to the
	// module tree resolved below.

	type contaminationCheckService struct{}
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *contaminationCheckService {
			return &contaminationCheckService{}
		})
	})
	m := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	p.Declare()

	err := Resolve(context.Background(), []*module.Module{m})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil -- unrelated module's pending edges (a real cycle) must not leak into this call's graph/cycle detection", err)
	}
}

// --- PendingAllEdge slot-write (T6) ----------------------------------------

// stage3Pingable/stage3FakePinger are placeholder target/implementation
// types for the PendingAllEdge slot-write tests below -- same
// "content does not matter, only identity/type" role stage3AService/
// stage3BService play for the PendingEdge (single-valor) tests above.
type stage3Pingable interface {
	Ping() string
}

type stage3FakePinger struct {
	name string
}

func (f *stage3FakePinger) Ping() string { return f.name }

// TestInvokeAndCopy_PendingAllEdge_WritesMatchedNodeIntoItsSlot proves a
// node listed as Matches[i] in a recorded PendingAllEdge has its resolved
// value written into edge.Slice.Index(i) once invokeAndCopy(node) runs, and
// that a node NOT listed in any edge's Matches (invokeAndCopy called for it
// too) leaves every OTHER slot in that same Slice untouched.
func TestInvokeAndCopy_PendingAllEdge_WritesMatchedNodeIntoItsSlot(t *testing.T) {
	inject.Reset()
	defer inject.Reset()

	matchA := provider.New(func(p *provider.Provider) {
		p.Constructor(func() stage3Pingable { return &stage3FakePinger{name: "a"} })
	})
	matchB := provider.New(func(p *provider.Provider) {
		p.Constructor(func() stage3Pingable { return &stage3FakePinger{name: "b"} })
	})
	unrelated := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *stage3AService { return &stage3AService{Value: "unrelated"} })
	})
	// matchA/matchB/unrelated must already be Declare()'d (ResolvedType()
	// populated) before owner's own Declare() runs findAllRefs against them
	// -- see internal/inject's findAllRefs/mustAllProvider, which reads
	// ResolvedType() directly without Declare()'ing candidates itself.
	matchA.Declare()
	matchB.Declare()
	unrelated.Declare()

	var owner *provider.Provider
	owner = provider.New(func(p *provider.Provider) {
		inject.MustAll[stage3Pingable](owner)
		p.Constructor(func() *stage3BService { return &stage3BService{Value: "owner"} })
	})

	m := module.New(func(m *module.Module) {
		m.Providers(owner, matchA, matchB, unrelated)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	owner.Declare()

	edges := inject.PendingAllEdges()
	if len(edges) != 1 {
		t.Fatalf("PendingAllEdges() len = %d, want 1", len(edges))
	}
	edge := edges[0]
	if len(edge.Matches) != 2 {
		t.Fatalf("edge.Matches len = %d, want 2", len(edge.Matches))
	}

	idxA, idxB := -1, -1
	for i, m := range edge.Matches {
		switch m {
		case module.ProviderRef(matchA):
			idxA = i
		case module.ProviderRef(matchB):
			idxB = i
		}
	}
	if idxA == -1 || idxB == -1 {
		t.Fatalf("edge.Matches did not contain both matchA and matchB: %+v", edge.Matches)
	}

	if err := invokeAndCopy(context.Background(), matchA, nil); err != nil {
		t.Fatalf("invokeAndCopy(matchA) error = %v", err)
	}

	gotA, ok := edge.Slice.Index(idxA).Interface().(stage3Pingable)
	if !ok || gotA == nil || gotA.Ping() != "a" {
		t.Fatalf("edge.Slice.Index(idxA) = %#v, want resolved matchA value (Ping() == %q)", edge.Slice.Index(idxA).Interface(), "a")
	}
	if !edge.Slice.Index(idxB).IsNil() {
		t.Fatalf("edge.Slice.Index(idxB) was written by invokeAndCopy(matchA), want untouched (nil)")
	}

	// unrelated is not listed in edge.Matches at all -- running
	// invokeAndCopy for it must leave every slot in edge.Slice exactly as
	// it was (idxA still "a", idxB still nil), proving the m == node guard
	// in invokeAndCopy's PendingAllEdge loop actually filters.
	if err := invokeAndCopy(context.Background(), unrelated, nil); err != nil {
		t.Fatalf("invokeAndCopy(unrelated) error = %v", err)
	}

	gotA2, ok := edge.Slice.Index(idxA).Interface().(stage3Pingable)
	if !ok || gotA2 == nil || gotA2.Ping() != "a" {
		t.Fatalf("edge.Slice.Index(idxA) changed after invokeAndCopy(unrelated), got %#v", edge.Slice.Index(idxA).Interface())
	}
	if !edge.Slice.Index(idxB).IsNil() {
		t.Fatalf("edge.Slice.Index(idxB) was written by invokeAndCopy(unrelated), want untouched (nil)")
	}
}

// TestResolve_ResolvesProvidersWithNoPendingEdgesToo proves Stage 3 resolves
// EVERY registered provider, not only ones reachable via a MustInject
// chain -- a provider nobody depends on (never a pending-edge target) must
// still have its Constructor invoked if it's registered in an assembled
// module.
func TestResolve_ResolvesProvidersWithNoPendingEdgesToo(t *testing.T) {
	var invoked atomic.Bool
	standalone := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *stage3BService {
			invoked.Store(true)
			return &stage3BService{Value: "standalone"}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(standalone)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	standalone.Declare()

	if err := Resolve(context.Background(), []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !invoked.Load() {
		t.Fatalf("standalone provider's Constructor was never invoked -- Stage 3 must resolve ALL registered providers, not just pending-edge targets")
	}
}
