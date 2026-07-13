package gonest

import "github.com/gonest-dev/gonest/internal/app"

// LogLevel identifies one of Nest's 5 standard log severities. See
// internal/app.LogLevel's doc comment for the full contract (iota-based
// const block plus a debug-friendly String()).
type LogLevel = app.LogLevel

const (
	// LogLevelError is the most severe level -- unrecoverable failures.
	LogLevelError = app.LogLevelError
	// LogLevelWarn signals a recoverable but noteworthy condition.
	LogLevelWarn = app.LogLevelWarn
	// LogLevelLog is Nest's default, general-purpose informational level.
	LogLevelLog = app.LogLevelLog
	// LogLevelDebug carries diagnostic detail useful during development.
	LogLevelDebug = app.LogLevelDebug
	// LogLevelVerbose is the most granular, chattiest level.
	LogLevelVerbose = app.LogLevelVerbose
)

// OnListen is the "bind succeeded" callback shape passed to App.MustListen.
// See internal/app.OnListen's doc comment for its nil-safety contract.
type OnListen = app.OnListen
