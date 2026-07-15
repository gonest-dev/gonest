// Package logger implements gonest's own startup/diagnostic logging --
// replacing the underlying HTTP adapter's own default console output
// (e.g. Fiber's ASCII-art banner) with a single, adapter-agnostic log
// format, matching Nest's own startup log convention:
//
//	2026-07-15T10:18:07.123Z [INFO] Gonest started on: http://127.0.0.1:3000
//
// A package-level default logger is used throughout -- internal/emitter and
// internal/scheduler (both of which recover a listener/job's own panic in
// isolation, with no logger reference threaded through their own
// constructors) call Error(...) directly rather than needing an injected
// dependency.
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level identifies one of Nest's 5 standard log severities. AppOptions.
// LogLevels (internal/app) is a direct alias of this type -- see
// options.go's own doc comment for why the type lives here rather than in
// internal/app.
type Level int

const (
	// LevelError is the most severe level -- unrecoverable failures.
	LevelError Level = iota
	// LevelWarn signals a recoverable but noteworthy condition.
	LevelWarn
	// LevelLog is Nest's default, general-purpose informational level --
	// printed with tag "INFO" (see tag() below), not "LOG".
	LevelLog
	// LevelDebug carries diagnostic detail useful during development.
	LevelDebug
	// LevelVerbose is the most granular, chattiest level.
	LevelVerbose
)

// String implements fmt.Stringer for debug-friendly output (distinct from
// tag(), which is what actually gets printed in a log line).
func (l Level) String() string {
	switch l {
	case LevelError:
		return "Error"
	case LevelWarn:
		return "Warn"
	case LevelLog:
		return "Log"
	case LevelDebug:
		return "Debug"
	case LevelVerbose:
		return "Verbose"
	default:
		return "Unknown"
	}
}

// tag is the short, all-caps label printed in every log line. LevelLog
// prints as "INFO" (matching Nest's own general-purpose informational
// convention), not "LOG" -- deliberately distinct from String() above,
// which stays as originally shipped for any existing debug-formatting
// caller.
func (l Level) tag() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelWarn:
		return "WARN"
	case LevelLog:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	case LevelVerbose:
		return "VERBOSE"
	default:
		return "LOG"
	}
}

var (
	mu      sync.Mutex
	out     io.Writer = os.Stdout
	allowed           = defaultAllowed()
)

func defaultAllowed() map[Level]bool {
	return map[Level]bool{LevelError: true, LevelWarn: true, LevelLog: true}
}

// Configure restricts which levels actually print -- called once per
// bootstrap by internal/app's NewApp/MustNewApp/MustNewTestApp, from
// AppOptions.LogLevels. An empty/nil levels resets to the default set
// (Error/Warn/Log -- Nest's own default, Debug/Verbose opt-in only).
func Configure(levels []Level) {
	mu.Lock()
	defer mu.Unlock()
	if len(levels) == 0 {
		allowed = defaultAllowed()
		return
	}
	allowed = make(map[Level]bool, len(levels))
	for _, l := range levels {
		allowed[l] = true
	}
}

// SetOutput redirects where log lines are written. Test-only hook (tests
// substitute a bytes.Buffer to assert on printed output instead of
// touching the package-level os.Stdout); production code never needs to
// call this.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	out = w
}

func write(level Level, message string) {
	mu.Lock()
	e, w := allowed[level], out
	mu.Unlock()
	if !e {
		return
	}
	fmt.Fprintf(w, "%s [%s] %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), level.tag(), message)
}

// Error logs message at LevelError.
func Error(message string) { write(LevelError, message) }

// Warn logs message at LevelWarn.
func Warn(message string) { write(LevelWarn, message) }

// Info logs message at LevelLog (printed tag "INFO").
func Info(message string) { write(LevelLog, message) }

// Debug logs message at LevelDebug.
func Debug(message string) { write(LevelDebug, message) }

// Verbose logs message at LevelVerbose.
func Verbose(message string) { write(LevelVerbose, message) }
