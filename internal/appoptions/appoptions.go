// Package appoptions holds AppOptions itself, split out of internal/app
// (where it lived per AD-004 in STATE.md, "1 package per DI-graph type"
// doesn't apply to it -- it's pure config, no graph relationships) into its
// own leaf package for a NEW reason: internal/adapter/fiber's FiberApp.Init
// now needs to read opts.EnableFormStreaming to configure fiber.Config
// BEFORE fiber.New() is called (Multipart Form Streaming feature, AD-022 in
// STATE.md), but internal/app already imports internal/adapter/fiber
// (internal/app/testapp.go's MustNewTestApp constructs a concrete
// fiber.FiberApp directly) -- AppOptions living in internal/app would make
// internal/adapter/fiber importing internal/app (for the AppOptions type
// HttpAdapter.Init's signature requires) an import cycle. This package has
// zero dependency on either internal/app or internal/adapter/fiber, so both
// can import it freely. internal/app re-exports everything here via plain
// type aliases (same "alias, not a new type" pattern LogLevel/logger.Level
// already used before this split), so no existing call site inside
// internal/app itself needed to change.
package appoptions

import "gonest.dev/gonest/internal/logger"

// AppOptions is Nest-parity bootstrap config for NewApp/MustNewApp
// (BufferLogs, LogLevels), matching INSIGHT.md's literal call sites
// (gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})).
// It is captured on *App; LogLevels is now REAL (see NewApp's own call to
// logger.Configure) -- BufferLogs remains inert (no buffering mechanism
// exists yet, only immediate output).
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
// (rather than requiring every AppOptions caller to import internal/logger
// directly) since AppOptions/NewApp/MustNewApp already lived under this name
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
