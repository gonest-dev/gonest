package app

import "github.com/gonest-dev/gonest/internal/logger"

// AppOptions is Nest-parity bootstrap config for NewApp/MustNewApp
// (BufferLogs, LogLevels), matching INSIGHT.md's literal call sites
// (gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})).
// It is captured on *App; LogLevels is now REAL (see NewApp's own call to
// logger.Configure) -- BufferLogs remains inert (no buffering mechanism
// exists yet, only immediate output).
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
	// only -- no buffering mechanism exists yet to act on it.
	BufferLogs bool
	// LogLevels restricts which LogLevel values the real internal/logger
	// package emits -- see NewApp's own call to logger.Configure(opts.LogLevels).
	LogLevels []LogLevel
	// DisableBanner, when true, skips MustListen's own "Gonest" ASCII
	// banner + version line -- the 3 structured [INFO] log lines still
	// print regardless (those carry actual operational information: bind
	// address, loaded module/controller/route counts, PID).
	DisableBanner bool
}

// LogLevel is internal/logger.Level's own alias -- kept under this name
// (rather than requiring every AppOptions caller to import internal/logger
// directly) since AppOptions/NewApp/MustNewApp already lived in this
// package before the real Logger existed.
type LogLevel = logger.Level

const (
	// LogLevelError is the most severe level -- unrecoverable failures.
	LogLevelError = logger.LevelError
	// LogLevelWarn signals a recoverable but noteworthy condition.
	LogLevelWarn = logger.LevelWarn
	// LogLevelLog is Nest's default, general-purpose informational level.
	LogLevelLog = logger.LevelLog
	// LogLevelDebug carries diagnostic detail useful during development.
	LogLevelDebug = logger.LevelDebug
	// LogLevelVerbose is the most granular, chattiest level.
	LogLevelVerbose = logger.LevelVerbose
)

// OnListen is the "bind succeeded" callback shape passed to a future
// App.MustListen (see design.md's "OnListen" component). Declared as a
// named type -- rather than used inline as func() everywhere -- so it reads
// as a distinct, documented concept at call sites, matching INSIGHT.md's
// exact two call shapes: gonest.OnListen(func(){...}) and a literal nil.
// It is nil-safe by construction: a nil OnListen is just a nil func value,
// so callers can guard with a plain `if f != nil { f() }` without any
// special-casing.
type OnListen func()
