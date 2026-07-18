package app

import "gonest.dev/gonest/internal/logger"

// Options is Nest-parity bootstrap config for NewApp/MustNewApp (BufferLogs,
// LogLevels), matching INSIGHT.md's literal call sites
// (gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...}) --
// exported publicly as gonest.AppOptions, a plain alias of this type). It is
// captured on *App; LogLevels is now REAL (see NewApp's own call to
// logger.Configure) -- BufferLogs remains inert (no buffering mechanism
// exists yet, only immediate output).
//
// Lives here (not internal/adapter/fiber) even though FiberApp.Init needs it
// too: internal/adapter/fiber imports this package (for the HttpAdapter/
// Options types its own Init/RegisterRoute signatures require), and this
// package must therefore NEVER import internal/adapter/fiber back -- see
// RegisterTestAdapter (test_app.go) for how MustNewTestApp still gets a real
// *fiber.FiberApp without that import.
type Options struct {
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

	// DisableLoaded, when true, skips printing the 3 structured log lines
	// that detail how many Modules, Controllers, and Routes were loaded
	// during bootstrap. The final "Listening on: ..." line still prints.
	DisableLoaded bool

	// EnableFormStreaming, when true, configures the underlying HTTP
	// adapter to stream the request body instead of buffering it whole
	// before any Handler runs -- required for ParseRestFormBody/
	// MustParseRestFormBody (internal/validate) to genuinely forward a
	// multipart file's bytes to storage as they arrive, never buffering
	// the whole file locally first (Multipart Form Streaming feature,
	// AD-022 in STATE.md). This is an APP-WIDE setting: the underlying
	// fiber.Config is immutable once fiber.New() returns, so it must be
	// known at adapter construction time (see HttpAdapter.Init's own doc
	// comment) and applies to every route the app serves, not just
	// form-upload ones -- though every OTHER existing call site
	// (ParseRestJsonBody/ParseRestParams/ParseRestQuery) is unaffected
	// either way, since Context.Body() still auto-drains the stream into
	// a buffer on first touch regardless of this setting.
	EnableFormStreaming bool

	// GraphqlPath overrides the fixed "/graphql" endpoint Query/Mutation
	// dispatch through (graphql-support feature, Milestone 17) -- empty
	// string (the zero value) keeps the "/graphql" default. The real-protocol
	// Subscription transports (graphql-transport-ws over WebSocket,
	// graphql-sse's Distinct and Single connection modes, Milestone 18)
	// share this SAME path -- overriding it moves all of them consistently.
	GraphqlPath string
}

// LogLevel is internal/logger.Level's own alias -- kept under this name
// (rather than requiring every Options caller to import internal/logger
// directly) since Options/NewApp/MustNewApp already lived under this name
// before the real Logger existed.
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

// OnListen is the "bind succeeded" callback shape passed to App.MustListen.
// Declared as a named type -- rather than used inline as func() everywhere
// -- so it reads as a distinct, documented concept at call sites, matching
// INSIGHT.md's exact two call shapes: gonest.OnListen(func(){...}) and a
// literal nil. It is nil-safe by construction: a nil OnListen is just a nil
// func value, so callers can guard with a plain `if f != nil { f() }`
// without any special-casing.
type OnListen func()
