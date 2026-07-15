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
	})
	fn(buf)
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
