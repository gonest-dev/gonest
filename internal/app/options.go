package app

// AppOptions is Nest-parity bootstrap config for NewApp/MustNewApp
// (BufferLogs, LogLevels), matching INSIGHT.md's literal call sites
// (gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})).
// It is captured on *App and otherwise inert per design.md's "App
// (extended)" component -- no Logger exists yet to consume it, so this type
// only proves the shape compiles and stores cleanly; nothing reads its
// fields in this task.
//
// It lives in internal/app rather than its own internal/appoptions package
// per AD-004 (.specs/project/STATE.md): AD-004's "1 package per DI-graph
// type" rule applies to graph participants (Provider/Module/Controller/
// Route) that get resolved/walked/collided against each other during
// bootstrap. AppOptions has zero relationships to that graph -- it is pure
// config for App itself, the same category as HttpMethod living inside
// internal/route rather than its own package (see internal/route/method.go).
// A dedicated package would be over-engineering for a handful of small,
// non-graph types with no cross-package reuse need.
type AppOptions struct {
	// BufferLogs, when true, defers emitting any buffered log output until
	// a Logger is attached later (Nest's own BufferLogs semantics). Stored
	// only -- no Logger exists yet in this codebase to act on it.
	BufferLogs bool
	// LogLevels restricts which LogLevel values a future Logger should
	// emit. Stored only -- see BufferLogs' comment above.
	LogLevels []LogLevel
}

// LogLevel identifies one of Nest's 5 standard log severities. Modeled
// after HttpMethod in internal/route/method.go: an iota-based const block
// plus a switch-based String() for debug-friendly output.
type LogLevel int

const (
	// LogLevelError is the most severe level -- unrecoverable failures.
	LogLevelError LogLevel = iota
	// LogLevelWarn signals a recoverable but noteworthy condition.
	LogLevelWarn
	// LogLevelLog is Nest's default, general-purpose informational level.
	LogLevelLog
	// LogLevelDebug carries diagnostic detail useful during development.
	LogLevelDebug
	// LogLevelVerbose is the most granular, chattiest level.
	LogLevelVerbose
)

// String implements fmt.Stringer for debug-friendly output.
func (l LogLevel) String() string {
	switch l {
	case LogLevelError:
		return "Error"
	case LogLevelWarn:
		return "Warn"
	case LogLevelLog:
		return "Log"
	case LogLevelDebug:
		return "Debug"
	case LogLevelVerbose:
		return "Verbose"
	default:
		return "Unknown"
	}
}

// OnListen is the "bind succeeded" callback shape passed to a future
// App.MustListen (see design.md's "OnListen" component). Declared as a
// named type -- rather than used inline as func() everywhere -- so it reads
// as a distinct, documented concept at call sites, matching INSIGHT.md's
// exact two call shapes: gonest.OnListen(func(){...}) and a literal nil.
// It is nil-safe by construction: a nil OnListen is just a nil func value,
// so callers can guard with a plain `if f != nil { f() }` without any
// special-casing.
type OnListen func()
