package handlers

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/lib"
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
			conn.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(portStr)
	return p, func() { listener.Close() }
}

func TestTelnetHandlerSuccessJSON(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		TelnetHandler(&jsonOutput, 1, 0, &throttle, 3, 4, port, ctx, "127.0.0.1")
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
	listener.Close()

	jsonOutput, throttle := false, false
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		TelnetHandler(&jsonOutput, 1, 0, &throttle, 2, 4, port, ctx, "127.0.0.1")
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		TelnetHandler(&jsonOutput, 1, 0, &throttle, 2, 4, 80, ctx, "this-host-should-not-exist.invalid")
	})

	if !strings.Contains(out, "[telnet] ERROR dns resolution failed") {
		t.Errorf("expected a dns-resolution-failed error line, got:\n%s", out)
	}
}

func TestTelnetHandlerMultipleIterationsNoRace(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		TelnetHandler(&jsonOutput, 10, 0, &throttle, 3, 4, port, ctx, "127.0.0.1")
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
