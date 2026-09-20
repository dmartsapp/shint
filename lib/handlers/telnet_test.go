package handlers

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

func startEchoListener(t *testing.T) (port int, closeFn func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(portStr)
	return p, func() { _ = listener.Close() }
}

func startEchoListenerIPv6(t *testing.T) (port int, closeFn func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Fatalf("failed to start IPv6 listener: %v", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(portStr)
	return p, func() { _ = listener.Close() }
}

func TestTelnetHandlerIPv6Loopback(t *testing.T) {
	port, closeFn := startEchoListenerIPv6(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		TelnetHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 3, 4, port, "::1")
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.TelnetStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != 1 || !stats[0].Success {
		t.Fatalf("expected one successful IPv6 stat entry, got %+v", stats)
	}
}

func TestTelnetHandlerSuccessJSON(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		TelnetHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 3, 4, port, "127.0.0.1")
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	if !result.DNSLookup.Success {
		t.Fatal("expected DNS lookup to succeed for 127.0.0.1")
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.TelnetStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != 1 || !stats[0].Success {
		t.Fatalf("expected one successful stat entry, got %+v", stats)
	}
}

func TestTelnetHandlerFailureText(t *testing.T) {
	// Grab a free port and immediately release it so nothing is listening.
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	_ = listener.Close()

	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		TelnetHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 2, 4, port, "127.0.0.1")
	})

	if !strings.Contains(out, "[telnet] ERROR connect failed") {
		t.Errorf("expected a connect-failed error line, got:\n%s", out)
	}
	if !strings.Contains(out, "Response received: 0") {
		t.Errorf("expected zero successful responses in the stats banner, got:\n%s", out)
	}
}

func TestTelnetHandlerDNSFailure(t *testing.T) {
	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		TelnetHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 2, 4, 80, "this-host-should-not-exist.invalid")
	})

	if !strings.Contains(out, "[telnet] ERROR dns resolution failed") {
		t.Errorf("expected a dns-resolution-failed error line, got:\n%s", out)
	}
}

func TestTelnetHandlerMultipleIterationsNoRace(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		TelnetHandler(context.Background(), &jsonOutput, 10, 0, &throttle, 3, 4, port, "127.0.0.1")
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.TelnetStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != 10 {
		t.Fatalf("expected 10 stat entries from 10 iterations, got %d", len(stats))
	}
}

// TestTelnetHandlerRunLongerThanTimeoutStillSucceeds is the regression test
// for false "i/o timeout" failures on later attempts: --timeout is what each
// connection attempt gets, so a run whose --count x --delay adds up to several
// times --timeout must still report every attempt against a healthy listener
// as a success. Four attempts 400ms apart take ~1.6s against a 1s timeout;
// when the timeout was also a deadline for the whole run, the attempts after
// the first second failed instantly.
func TestTelnetHandlerRunLongerThanTimeoutStillSucceeds(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	start := time.Now()
	out := captureStdout(t, func() {
		TelnetHandler(context.Background(), &jsonOutput, 4, 400, &throttle, 1, 4, port, "127.0.0.1")
	})
	if elapsed := time.Since(start); elapsed < 1200*time.Millisecond {
		t.Fatalf("run took only %v; the test needs to outlast the 1s --timeout to mean anything", elapsed)
	}

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.TelnetStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != 4 {
		t.Fatalf("expected 4 attempts, got %d", len(stats))
	}
	for i, s := range stats {
		if !s.Success {
			t.Errorf("attempt %d against a listener that is up failed: %q", i+1, s.Error)
		}
	}
}
