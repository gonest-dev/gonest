package logger

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// withCapturedOutput redirects package output to a fresh buffer for the
// duration of fn, restoring the previous output and level configuration
// afterward -- this package's state is process-global, so tests must not
// leak configuration into one another.
func withCapturedOutput(t *testing.T, fn func(buf *bytes.Buffer)) {
	t.Helper()
	buf := &bytes.Buffer{}
	SetOutput(buf)
	t.Cleanup(func() {
		SetOutput(io.Writer(os.Stdout))
		Configure(nil)
		SetActive(nil)
	})
	fn(buf)
}

type spyLogger struct {
	lines []string
}

func (s *spyLogger) Error(message string, meta ...map[string]any) { s.record("ERROR", message, meta) }
func (s *spyLogger) Warn(message string, meta ...map[string]any)  { s.record("WARN", message, meta) }
func (s *spyLogger) Info(message string, meta ...map[string]any)  { s.record("INFO", message, meta) }
func (s *spyLogger) Debug(message string, meta ...map[string]any) { s.record("DEBUG", message, meta) }
func (s *spyLogger) Verbose(message string, meta ...map[string]any) {
	s.record("VERBOSE", message, meta)
}
func (s *spyLogger) record(tag, message string, meta []map[string]any) {
	line := tag + " " + message
	if len(meta) > 0 && meta[0] != nil {
		line += " has-meta"
	}
	s.lines = append(s.lines, line)
}

func TestSetActive_CustomLogger_ReplacesConsoleOutput_PackageFuncsDelegate(t *testing.T) {
	spy := &spyLogger{}
	SetActive(spy)
	t.Cleanup(func() { SetActive(nil) })

	Info("hello")
	Error("boom", map[string]any{"code": 1})

	if len(spy.lines) != 2 {
		t.Fatalf("spy.lines = %v, want 2 entries", spy.lines)
	}
	if spy.lines[0] != "INFO hello" {
		t.Fatalf("spy.lines[0] = %q, want %q", spy.lines[0], "INFO hello")
	}
	if spy.lines[1] != "ERROR boom has-meta" {
		t.Fatalf("spy.lines[1] = %q, want %q", spy.lines[1], "ERROR boom has-meta")
	}
}

func TestSetActive_Nil_ResetsToConsoleLogger(t *testing.T) {
	SetActive(&spyLogger{})
	SetActive(nil)

	if _, ok := Active().(consoleLogger); !ok {
		t.Fatalf("Active() = %T, want consoleLogger after SetActive(nil)", Active())
	}
}

func TestGetLogger_NoArgs_ReturnsActiveDirectly(t *testing.T) {
	t.Cleanup(func() { SetActive(nil) })
	spy := &spyLogger{}
	SetActive(spy)

	if GetLogger() != Logger(spy) {
		t.Fatalf("GetLogger() did not return the active spy logger directly")
	}
}

func TestGetLogger_WithName_PrefixesEveryLine(t *testing.T) {
	t.Cleanup(func() { SetActive(nil) })
	spy := &spyLogger{}
	SetActive(spy)

	GetLogger("cron:invoice-sync").Info("tick")

	if len(spy.lines) != 1 || spy.lines[0] != "INFO [cron:invoice-sync] tick" {
		t.Fatalf("spy.lines = %v, want [\"INFO [cron:invoice-sync] tick\"]", spy.lines)
	}
}

func TestGetLogger_MoreThanOneName_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("GetLogger with 2 names did not panic")
		}
	}()
	GetLogger("a", "b")
}

type getLoggerForFixture struct{}

func TestGetLoggerFor_PointerType_PrefixesWithPointeeName(t *testing.T) {
	t.Cleanup(func() { SetActive(nil) })
	spy := &spyLogger{}
	SetActive(spy)

	GetLoggerFor[*getLoggerForFixture]().Warn("careful")

	if len(spy.lines) != 1 || spy.lines[0] != "WARN [getLoggerForFixture] careful" {
		t.Fatalf("spy.lines = %v, want [\"WARN [getLoggerForFixture] careful\"]", spy.lines)
	}
}

func TestInfo_PrintsISO8601TimestampAndINFOTag(t *testing.T) {
	withCapturedOutput(t, func(buf *bytes.Buffer) {
		Info("Gonest started on: http://127.0.0.1:3000")

		line := buf.String()
		if !strings.Contains(line, "[INFO]") {
			t.Fatalf("output = %q, want it to contain [INFO]", line)
		}
		if !strings.Contains(line, "Gonest started on: http://127.0.0.1:3000") {
			t.Fatalf("output = %q, missing the message", line)
		}
		// ISO 8601 with milliseconds and Z suffix: YYYY-MM-DDTHH:mm:ss.sssZ
		if !strings.Contains(line, "T") || !strings.Contains(line, "Z [INFO]") {
			t.Fatalf("output = %q, want ISO8601 timestamp immediately before [INFO]", line)
		}
	})
}

func TestError_PrintsERRORTag(t *testing.T) {
	withCapturedOutput(t, func(buf *bytes.Buffer) {
		Error("boom")
		if !strings.Contains(buf.String(), "[ERROR] boom") {
			t.Fatalf("output = %q, want it to contain [ERROR] boom", buf.String())
		}
	})
}

func TestConfigure_DefaultLevels_ErrorWarnLogAllowed_DebugVerboseSuppressed(t *testing.T) {
	withCapturedOutput(t, func(buf *bytes.Buffer) {
		Configure(nil) // reset to default

		Error("e")
		Warn("w")
		Info("i")
		Debug("d")
		Verbose("v")

		out := buf.String()
		for _, want := range []string{"[ERROR] e", "[WARN] w", "[INFO] i"} {
			if !strings.Contains(out, want) {
				t.Fatalf("output missing %q, got: %q", want, out)
			}
		}
		for _, notWant := range []string{"[DEBUG]", "[VERBOSE]"} {
			if strings.Contains(out, notWant) {
				t.Fatalf("output contains %q, want Debug/Verbose suppressed by default: %q", notWant, out)
			}
		}
	})
}

func TestConfigure_RestrictsToExplicitLevelsOnly(t *testing.T) {
	withCapturedOutput(t, func(buf *bytes.Buffer) {
		Configure([]Level{LevelError})

		Error("e")
		Warn("w")
		Info("i")

		out := buf.String()
		if !strings.Contains(out, "[ERROR] e") {
			t.Fatalf("output missing [ERROR] e, got: %q", out)
		}
		if strings.Contains(out, "[WARN]") || strings.Contains(out, "[INFO]") {
			t.Fatalf("output = %q, want Warn/Info suppressed when Configure([]Level{LevelError}) was called", out)
		}
	})
}
