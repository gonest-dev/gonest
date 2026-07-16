// Package scheduler implements the declarative Scheduler API: Cron/
// Interval/Timeout jobs, each execution isolated (its own recover, never
// crashes the process nor blocks any other scheduled execution), each
// individually stoppable by name. Equivalent to @nestjs/schedule. See
// ROADMAP.md's Milestone 10.
package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	cronlib "github.com/robfig/cron/v3"

	"gonest.dev/gonest/internal/logger"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// jobHandle backs one named Cron/Interval/Timeout registration -- Stop
// closes stop exactly once (via sync.Once, so calling Stop twice on the
// same job is a safe no-op), which every job's own goroutine selects on
// to exit its loop (Cron/Interval) or cancel its pending fire (Timeout).
type jobHandle struct {
	stop chan struct{}
	once sync.Once
}

func newJobHandle() *jobHandle {
	return &jobHandle{stop: make(chan struct{})}
}

func (h *jobHandle) Stop() {
	h.once.Do(func() { close(h.stop) })
}

// Scheduler represents a single unit of job registration: its builder fn
// (deferred until Declare runs, same New*-deferred pattern Provider/
// Controller already use) is expected to call Cron/Interval/Timeout one or
// more times, each spawning its own background goroutine, individually
// stoppable via Stop(name).
type Scheduler struct {
	fn func(*Scheduler)

	owner    *module.Module
	declared bool

	mu   sync.Mutex
	jobs map[string]*jobHandle
}

// New creates a Scheduler that defers fn until Declare runs it -- same
// deferred-builder pattern as Provider/Controller/Listener (AD-015's
// 3-phase bootstrap), since fn is expected to call MustInject, which needs
// a known module scope to resolve against.
func New(fn func(*Scheduler)) *Scheduler {
	return &Scheduler{fn: fn, jobs: make(map[string]*jobHandle)}
}

// Declare runs this scheduler's deferred fn exactly once -- idempotent,
// same contract as Provider.Declare/Controller.Declare/Listener.Declare.
// Cron/Interval/Timeout calls made inside fn spawn their background
// goroutines immediately, as fn runs.
func (s *Scheduler) Declare() {
	if s.declared {
		return
	}
	s.declared = true
	if s.fn != nil {
		s.fn(s)
	}
}

// IsScheduler is the marker method that satisfies module.SchedulerRef, so
// *Scheduler can be passed to (*module.Module).Schedulers without module
// needing to import this package.
func (s *Scheduler) IsScheduler() {}

// SetOwnerModule associates this scheduler with the module that owns it.
// Called by module assembly (Stage 1) once ownership is known.
func (s *Scheduler) SetOwnerModule(m *module.Module) {
	s.owner = m
}

// OwnerModule implements module.Owner. Returns nil until SetOwnerModule
// has been called.
func (s *Scheduler) OwnerModule() *module.Module {
	return s.owner
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// scoped to a SINGLE module: this Scheduler's own OwnerModule (same
// single-module encapsulation Controller/Listener's own ResolveDirect
// use -- a Scheduler has exactly one owning module, registered directly
// via Module.Schedulers).
func (s *Scheduler) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect([]*module.Module{s.owner}, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// same single-module scope as ResolveDirect.
func (s *Scheduler) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll([]*module.Module{s.owner}, t)
}

// register creates and stores a jobHandle for name, overwriting any
// previous handle registered under the same name (re-registering a name
// implicitly replaces the old job -- the old goroutine keeps running
// unaware of this, matching "last registration under this name is the one
// Stop(name) controls" -- callers should not reuse a name for 2 genuinely
// different jobs on the same Scheduler).
func (s *Scheduler) register(name string) *jobHandle {
	h := newJobHandle()
	s.mu.Lock()
	s.jobs[name] = h
	s.mu.Unlock()
	return h
}

// Stop cancels the named job: a Cron/Interval job's next scheduled fire
// (and every one after it) never happens; a Timeout job not yet fired
// never fires at all. A currently-RUNNING execution (already past its own
// select) is not interrupted -- Stop only prevents FUTURE fires. No-op if
// name was never registered, or was already stopped.
func (s *Scheduler) Stop(name string) {
	s.mu.Lock()
	h, ok := s.jobs[name]
	s.mu.Unlock()
	if ok {
		h.Stop()
	}
}

// runIsolated invokes fn(context.Background()) with its own recover -- a
// panic never propagates past this call, never crashes the process, and
// never prevents a LATER scheduled execution of the same (or any other)
// job. The recovered value is logged via internal/logger.Error (Nest's own
// equivalent behavior: a job failing surfaces in the log, not silently).
func runIsolated(name string, fn func(ctx context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Sprintf("scheduled job %q panicked: %v", name, r))
		}
	}()
	fn(context.Background())
}

// Cron schedules fn to run repeatedly following expr, a standard 5-field
// cron expression (minute hour day-of-month month day-of-week, e.g.
// "0 0 * * *"), until the process exits or Stop(name) is called. Panics at
// registration time if expr fails to parse. Returns s so calls can chain
// (mirrors Controller.Tags/BearerAuth's own chaining precedent).
func (s *Scheduler) Cron(name string, expr string, fn func(ctx context.Context)) *Scheduler {
	schedule, err := cronlib.ParseStandard(expr)
	if err != nil {
		panic(fmt.Sprintf("gonest: invalid Cron expression %q for job %q: %v", expr, name, err))
	}

	h := s.register(name)
	go func() {
		for {
			d := time.Until(schedule.Next(time.Now()))
			if d < 0 {
				d = 0
			}
			timer := time.NewTimer(d)
			select {
			case <-timer.C:
				runIsolated(name, fn)
			case <-h.stop:
				timer.Stop()
				return
			}
		}
	}()

	return s
}

// Interval schedules fn to run every d, until the process exits or
// Stop(name) is called.
func (s *Scheduler) Interval(name string, d time.Duration, fn func(ctx context.Context)) *Scheduler {
	h := s.register(name)
	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runIsolated(name, fn)
			case <-h.stop:
				return
			}
		}
	}()

	return s
}

// Timeout schedules fn to run EXACTLY once, after d, unless Stop(name) is
// called before it fires.
func (s *Scheduler) Timeout(name string, d time.Duration, fn func(ctx context.Context)) *Scheduler {
	h := s.register(name)
	go func() {
		timer := time.NewTimer(d)
		select {
		case <-timer.C:
			runIsolated(name, fn)
		case <-h.stop:
			timer.Stop()
			return
		}
	}()

	return s
}
