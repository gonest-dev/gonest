// Package scheduler implements the declarative Scheduler API: Cron/
// Interval/Timeout jobs, each execution isolated (its own recover, never
// crashes the process nor blocks any other scheduled execution).
// Equivalent to @nestjs/schedule. See ROADMAP.md's Milestone 10.
package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"time"

	cronlib "github.com/robfig/cron/v3"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolver"
)

// Scheduler represents a single unit of job registration: its builder fn
// (deferred until Declare runs, same New*-deferred pattern Provider/
// Controller/Listener already use) is expected to call Cron/Interval/
// Timeout one or more times, each spawning its own background goroutine.
type Scheduler struct {
	fn func(*Scheduler)

	owner    *module.Module
	declared bool
}

// New creates a Scheduler that defers fn until Declare runs it -- same
// deferred-builder pattern as Provider/Controller/Listener (AD-015's
// 3-phase bootstrap), since fn is expected to call MustInject, which needs
// a known module scope to resolve against.
func New(fn func(*Scheduler)) *Scheduler {
	return &Scheduler{fn: fn}
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

// runIsolated invokes fn(context.Background()) with its own recover --
// a panic never propagates past this call, never crashes the process, and
// never prevents a LATER scheduled execution of the same (or any other)
// job. No logger exists yet in this framework (see AppOptions' own doc
// comment on BufferLogs/LogLevels being inert config today), so a
// recovered panic is silently swallowed rather than crashing the process
// -- a documented limitation, not a bug, until a real Logger feature
// exists (same stance as internal/emitter.Emitter.Emit's own recover).
func runIsolated(fn func(ctx context.Context)) {
	defer func() { _ = recover() }()
	fn(context.Background())
}

// Cron schedules fn to run repeatedly following expr, a standard 5-field
// cron expression (minute hour day-of-month month day-of-week, e.g.
// "0 0 * * *"), indefinitely until the process exits. Panics at
// registration time if expr fails to parse. name identifies this job for
// debugging (not otherwise used yet -- no Logger exists to attribute
// output to it). Returns s so calls can chain (mirrors Controller.Tags/
// BearerAuth's own chaining precedent).
func (s *Scheduler) Cron(name string, expr string, fn func(ctx context.Context)) *Scheduler {
	schedule, err := cronlib.ParseStandard(expr)
	if err != nil {
		panic(fmt.Sprintf("gonest: invalid Cron expression %q for job %q: %v", expr, name, err))
	}

	go func() {
		for {
			next := schedule.Next(time.Now())
			d := time.Until(next)
			if d < 0 {
				d = 0
			}
			time.Sleep(d)
			runIsolated(fn)
		}
	}()

	return s
}

// Interval schedules fn to run every d, indefinitely until the process
// exits.
func (s *Scheduler) Interval(name string, d time.Duration, fn func(ctx context.Context)) *Scheduler {
	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for range ticker.C {
			runIsolated(fn)
		}
	}()

	return s
}

// Timeout schedules fn to run EXACTLY once, after d.
func (s *Scheduler) Timeout(name string, d time.Duration, fn func(ctx context.Context)) *Scheduler {
	go func() {
		time.Sleep(d)
		runIsolated(fn)
	}()

	return s
}
