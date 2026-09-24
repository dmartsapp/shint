package lib

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// withVerbose points verboseOut at a buffer and sets Verbose for the
// duration of the test, restoring both after - the same swap-and-restore
// shape family_test.go uses for NetworkType.
func withVerbose(t *testing.T, v bool) *bytes.Buffer {
	t.Helper()
	oldOut, oldV := verboseOut, Verbose
	buf := &bytes.Buffer{}
	verboseOut, Verbose = buf, v
	t.Cleanup(func() { verboseOut, Verbose = oldOut, oldV })
	return buf
}

func TestLogWithTimestampOK(t *testing.T) {
	line := LogWithTimestamp("telnet", "connect ok host=1.2.3.4", false)
	if !strings.Contains(line, "[telnet]") {
		t.Errorf("expected module tag [telnet] in %q", line)
	}
	if !strings.Contains(line, "OK") {
		t.Errorf("expected OK status in %q", line)
	}
	if strings.Contains(line, "ERROR") {
		t.Errorf("did not expect ERROR status in %q", line)
	}
	if !strings.Contains(line, "connect ok host=1.2.3.4") {
		t.Errorf("expected message body preserved in %q", line)
	}
}

func TestLogWithTimestampError(t *testing.T) {
	line := LogWithTimestamp("web", "request failed", true)
	if !strings.Contains(line, "[web]") {
		t.Errorf("expected module tag [web] in %q", line)
	}
	if !strings.Contains(line, "ERROR") {
		t.Errorf("expected ERROR status in %q", line)
	}
}

func TestFields(t *testing.T) {
	got := Fields("host", "1.2.3.4", "port", 443, "time", 5*time.Millisecond)
	want := "host=1.2.3.4 port=443 time=5ms"
	if got != want {
		t.Errorf("Fields() = %q, want %q", got, want)
	}
}

func TestFieldsQuotesValuesWithSpaces(t *testing.T) {
	got := Fields("status", "200 OK")
	want := `status="200 OK"`
	if got != want {
		t.Errorf("Fields() = %q, want %q", got, want)
	}
}

func TestFieldsPanicsOnOddArgs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for odd number of arguments, got none")
		}
	}()
	Fields("host", "1.2.3.4", "port") //nolint:staticcheck // SA5012: deliberately odd arity, this test exercises the panic on it
}

func TestLogStatsWithSuccesses(t *testing.T) {
	stats := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	out := LogStats("telnet", stats, 2)
	if !strings.Contains(out, "telnet STATISTICS") {
		t.Errorf("expected banner with module name, got %q", out)
	}
	if !strings.Contains(out, "Requests sent: 2, Response received: 2, Success: 100%") {
		t.Errorf("expected 100%% success line, got %q", out)
	}
}

func TestLogStatsWithNoResponses(t *testing.T) {
	out := LogStats("nmap", []time.Duration{}, 5)
	if !strings.Contains(out, "Response received: 0") {
		t.Errorf("expected zero responses noted, got %q", out)
	}
}

func TestLogStatsWithZeroIterationsDoesNotPanic(t *testing.T) {
	// Guards against a division-by-zero panic if a caller ever passes 0.
	out := LogStats("udp", []time.Duration{}, 0)
	if !strings.Contains(out, "Requests sent: 0") {
		t.Errorf("expected zero requests noted, got %q", out)
	}
}

func TestLogStatsPartialSuccess(t *testing.T) {
	stats := []time.Duration{10 * time.Millisecond}
	out := LogStats("nmap", stats, 4)
	if !strings.Contains(out, "Success: 25%") {
		t.Errorf("expected 25%% success line, got %q", out)
	}
}

func TestLogVerboseWritesWhenEnabled(t *testing.T) {
	buf := withVerbose(t, true)
	LogVerbose("resolve", "starting host=example.com")
	got := buf.String()
	if !strings.Contains(got, "[resolve] VERBOSE starting host=example.com") {
		t.Errorf("LogVerbose output = %q, missing the expected tag/message", got)
	}
}

func TestLogVerboseSilentByDefault(t *testing.T) {
	buf := withVerbose(t, false)
	LogVerbose("resolve", "starting host=example.com")
	if got := buf.String(); got != "" {
		t.Errorf("LogVerbose wrote %q while Verbose is false, want nothing", got)
	}
}

func TestSetVerboseAppliesTheFlag(t *testing.T) {
	old := Verbose
	t.Cleanup(func() { Verbose = old })
	SetVerbose(true)
	if !Verbose {
		t.Error("SetVerbose(true) left Verbose false")
	}
	SetVerbose(false)
	if Verbose {
		t.Error("SetVerbose(false) left Verbose true")
	}
}
