// Package logger implements gonest's own startup/diagnostic logging --
// replacing the underlying HTTP adapter's own default console output
// (e.g. Fiber's ASCII-art banner) with a single, adapter-agnostic log
// format, matching Nest's own startup log convention:
//
//	2026-07-15T10:18:07.123Z [INFO] Gonest started on: http://127.0.0.1:3000
//
// A package-level active Logger is used throughout -- internal/emitter and
// internal/scheduler (both of which recover a listener/job's own panic in
// isolation, with no logger reference threaded through their own
// constructors) call Error(...) directly rather than needing an injected
// dependency. active defaults to consoleLogger (this file's own
// implementation) and is swappable via SetActive -- see AppOptions.Logger
// (internal/app/options.go) for the public bootstrap-time hook.
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
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

// Logger is gonest's structured logging contract -- 5 severities matching
// Nest's LoggerService shape, each accepting an optional structured meta
// map (0 or 1 -- 2+ is a caller bug, same variadic-at-most-one convention
// AppOptions/NewApp already use). Implement this to replace gonest's own
// console output (AppOptions.Logger) or to obtain the active instance
// anywhere via GetLogger/GetLoggerFor.
type Logger interface {
	Error(message string, meta ...map[string]any)
	Warn(message string, meta ...map[string]any)
	Info(message string, meta ...map[string]any)
	Debug(message string, meta ...map[string]any)
	Verbose(message string, meta ...map[string]any)
}

// consoleLogger is the built-in Logger implementation -- same
// timestamp+tag+stdout format gonest has always used, now reachable
// through the Logger interface instead of being the only option.
type consoleLogger struct{}

func (consoleLogger) Error(message string, meta ...map[string]any) {
	write(LevelError, "", message, meta)
}
func (consoleLogger) Warn(message string, meta ...map[string]any) {
	write(LevelWarn, "", message, meta)
}
func (consoleLogger) Info(message string, meta ...map[string]any) { write(LevelLog, "", message, meta) }
func (consoleLogger) Debug(message string, meta ...map[string]any) {
	write(LevelDebug, "", message, meta)
}
func (consoleLogger) Verbose(message string, meta ...map[string]any) {
	write(LevelVerbose, "", message, meta)
}

// contextLogger wraps parent, prefixing every line with "[name] " before
// delegating -- shared by GetLogger(name) (caller-chosen string) and
// GetLoggerFor[T]() (name derived from T via reflect). Works uniformly
// regardless of parent's concrete implementation (consoleLogger or a
// caller-supplied one via AppOptions.Logger), since it only touches the
// message string, never parent's internals.
type contextLogger struct {
	name   string
	parent Logger
}

func (c *contextLogger) Error(message string, meta ...map[string]any) {
	c.parent.Error("["+c.name+"] "+message, meta...)
}
func (c *contextLogger) Warn(message string, meta ...map[string]any) {
	c.parent.Warn("["+c.name+"] "+message, meta...)
}
func (c *contextLogger) Info(message string, meta ...map[string]any) {
	c.parent.Info("["+c.name+"] "+message, meta...)
}
func (c *contextLogger) Debug(message string, meta ...map[string]any) {
	c.parent.Debug("["+c.name+"] "+message, meta...)
}
func (c *contextLogger) Verbose(message string, meta ...map[string]any) {
	c.parent.Verbose("["+c.name+"] "+message, meta...)
}

var (
	mu      sync.Mutex
	out     io.Writer = os.Stdout
	allowed           = defaultAllowed()
	active  Logger    = consoleLogger{}
)

func defaultAllowed() map[Level]bool {
	return map[Level]bool{LevelError: true, LevelWarn: true, LevelLog: true}
}

// Configure restricts which levels actually print -- called once per
// bootstrap by internal/app's NewApp/MustNewApp/MustNewTestApp, from
// AppOptions.LogLevels. An empty/nil levels resets to the default set
// (Error/Warn/Log -- Nest's own default, Debug/Verbose opt-in only). Only
// affects consoleLogger's own output -- a caller-supplied Logger (via
// SetActive/AppOptions.Logger) is responsible for its own level filtering.
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

// SetActive replaces the Logger every GetLogger/GetLoggerFor call and every
// package-level Error/Warn/Info/Debug/Verbose call delegates to. A nil l
// resets to the built-in consoleLogger -- called unconditionally by
// NewApp/MustNewApp/MustNewTestApp at the start of every bootstrap (via
// AppOptions.Logger, nil unless the caller set it), so a custom Logger from
// an earlier bootstrap in the same process never leaks into the next one.
func SetActive(l Logger) {
	mu.Lock()
	defer mu.Unlock()
	if l == nil {
		active = consoleLogger{}
		return
	}
	active = l
}

// Active returns the currently active Logger (consoleLogger, the default,
// unless SetActive was called with a non-nil value).
func Active() Logger {
	mu.Lock()
	defer mu.Unlock()
	return active
}

// GetLogger returns the active Logger, optionally wrapped to prefix every
// line with a caller-chosen context name (e.g. "cron:invoice-sync" -- a
// name that isn't a Go type). Panics if given more than one context name
// (same at-most-one variadic contract as NewApp(opts ...Options)).
func GetLogger(optionalNamedContext ...string) Logger {
	if len(optionalNamedContext) > 1 {
		panic("gonest: GetLogger accepts at most one named context")
	}
	if len(optionalNamedContext) == 0 {
		return Active()
	}
	return &contextLogger{name: optionalNamedContext[0], parent: Active()}
}

// GetLoggerFor returns the active Logger wrapped to prefix every line with
// T's own type name, derived via reflect (T's pointee name for a pointer
// type, so GetLoggerFor[*Service]() prefixes "[Service]", not "[]").
func GetLoggerFor[T any]() Logger {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return &contextLogger{name: t.Name(), parent: Active()}
}

// SetOutput redirects where consoleLogger's own log lines are written.
// Test-only hook (tests substitute a bytes.Buffer to assert on printed
// output instead of touching the package-level os.Stdout); production code
// never needs to call this. Has no effect on a caller-supplied Logger set
// via SetActive -- that implementation owns its own output entirely.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	out = w
}

func write(level Level, context, message string, meta []map[string]any) {
	mu.Lock()
	e, w := allowed[level], out
	mu.Unlock()
	if !e {
		return
	}
	line := fmt.Sprintf("%s [%s]", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), level.tag())
	if context != "" {
		line += fmt.Sprintf(" [%s]", context)
	}
	line += " " + message
	if len(meta) > 0 && meta[0] != nil {
		if b, err := json.Marshal(meta[0]); err == nil {
			line += " " + string(b)
		}
	}
	fmt.Fprintln(w, line)
}

// Error logs message at LevelError via the active Logger.
func Error(message string, meta ...map[string]any) { Active().Error(message, meta...) }

// Warn logs message at LevelWarn via the active Logger.
func Warn(message string, meta ...map[string]any) { Active().Warn(message, meta...) }

// Info logs message at LevelLog (printed tag "INFO") via the active Logger.
func Info(message string, meta ...map[string]any) { Active().Info(message, meta...) }

// Debug logs message at LevelDebug via the active Logger.
func Debug(message string, meta ...map[string]any) { Active().Debug(message, meta...) }

// Verbose logs message at LevelVerbose via the active Logger.
func Verbose(message string, meta ...map[string]any) { Active().Verbose(message, meta...) }
