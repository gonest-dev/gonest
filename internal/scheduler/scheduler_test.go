package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	ran := false
	s := New(func(s *Scheduler) {
		ran = true
	})

	if s == nil {
		t.Fatal("expected New to return a non-nil *Scheduler")
	}
	if ran {
		t.Fatal("expected New(fn) to defer fn, not run it synchronously")
	}
}

func TestDeclare_ExecutesFn(t *testing.T) {
	ran := false
	s := New(func(s *Scheduler) {
		ran = true
	})

	s.Declare()

	if !ran {
		t.Fatal("expected Declare to run the deferred fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	s := New(func(s *Scheduler) {
		count++
	})

	s.Declare()
	s.Declare()

	if count != 1 {
		t.Fatalf("expected fn to run exactly once across repeated Declare calls, ran %d times", count)
	}
}

func TestTimeout_RunsExactlyOnceAfterDuration(t *testing.T) {
	runs := make(chan struct{}, 10)

	s := New(func(s *Scheduler) {
		s.Timeout("warmup", 20*time.Millisecond, func(ctx context.Context) {
			runs <- struct{}{}
		})
	})
	s.Declare()

	select {
	case <-runs:
	case <-time.After(time.Second):
		t.Fatal("Timeout job did not run within 1s")
	}

	select {
	case <-runs:
		t.Fatal("Timeout job ran more than once")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestInterval_RunsRepeatedly(t *testing.T) {
	runs := make(chan struct{}, 10)

	s := New(func(s *Scheduler) {
		s.Interval("ping", 10*time.Millisecond, func(ctx context.Context) {
			select {
			case runs <- struct{}{}:
			default:
			}
		})
	})
	s.Declare()

	for i := 0; i < 3; i++ {
		select {
		case <-runs:
		case <-time.After(time.Second):
			t.Fatalf("Interval job did not run a %dth time within 1s", i+1)
		}
	}
}

func TestCron_ValidExpression_DoesNotPanic(t *testing.T) {
	s := New(func(s *Scheduler) {
		s.Cron("cleanup", "0 0 * * *", func(ctx context.Context) {})
	})
	s.Declare()
}

func TestCron_InvalidExpression_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected Cron to panic on an invalid expression")
		}
	}()

	s := New(func(s *Scheduler) {
		s.Cron("bad", "not a cron expr", func(ctx context.Context) {})
	})
	s.Declare()
}

func TestRunIsolated_PanicNeverPropagates(t *testing.T) {
	ranAfterPanic := make(chan struct{})

	s := New(func(s *Scheduler) {
		s.Timeout("boom", 10*time.Millisecond, func(ctx context.Context) {
			panic("boom")
		})
		s.Timeout("after", 30*time.Millisecond, func(ctx context.Context) {
			close(ranAfterPanic)
		})
	})
	s.Declare() // must not panic here despite the first job's own panic

	select {
	case <-ranAfterPanic:
	case <-time.After(time.Second):
		t.Fatal("expected a later scheduled job to still run after an earlier one panicked")
	}
}
