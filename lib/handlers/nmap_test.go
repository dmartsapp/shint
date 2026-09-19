package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/lib"
)

func TestNmapHandlerFindsOpenPortIPv6(t *testing.T) {
	port, closeFn := startEchoListenerIPv6(t)
	defer closeFn()

	jsonOutput := true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		NmapHandler(ctx, "::1", port, port, 1, 2, false, &jsonOutput)
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.NmapStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != 1 || !stats[0].Success || stats[0].Address != "::1" {
		t.Fatalf("expected one successful IPv6 stat entry for ::1, got %+v", stats)
	}
}

func TestNmapHandlerFindsOpenPortInRange(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()

	// Scan a small range around the open port to keep the test fast while
	// still exercising the "some ports open, some closed" path.
	from := port - 1
	to := port + 1

	jsonOutput := true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		NmapHandler(ctx, "127.0.0.1", from, to, 1, 2, false, &jsonOutput)
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.NmapStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != (to - from + 1) {
		t.Fatalf("expected %d ports scanned, got %d", to-from+1, len(stats))
	}

	foundOpen := false
	for _, s := range stats {
		if s.Port == port {
			if !s.Success {
				t.Errorf("expected port %d to be reported open", port)
			}
			foundOpen = true
		}
	}
	if !foundOpen {
		t.Fatalf("expected port %d to appear in scan results", port)
	}
}

func TestNmapHandlerTextModeLogsOnlyOpenPorts(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()

	jsonOutput := false
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		NmapHandler(ctx, "127.0.0.1", port, port, 1, 2, false, &jsonOutput)
	})

	if !strings.Contains(out, "[nmap] OK port open") {
		t.Errorf("expected a port-open log line, got:\n%s", out)
	}
	if !strings.Contains(out, "scan complete") {
		t.Errorf("expected a scan-complete summary line, got:\n%s", out)
	}
}

func TestNmapHandlerDNSFailure(t *testing.T) {
	jsonOutput := false
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		NmapHandler(ctx, "this-host-should-not-exist.invalid", 1, 10, 1, 1, false, &jsonOutput)
	})

	if !strings.Contains(out, "[nmap] ERROR dns resolution failed") {
		t.Errorf("expected a dns-resolution-failed error line, got:\n%s", out)
	}
}

func TestNmapHandlerWideRangeCompletesUnderConcurrencyCap(t *testing.T) {
	// Regression test for the unbounded-goroutine nmap bug: scanning a
	// range larger than maxConcurrentPortScans must still finish promptly
	// (the semaphore must actually release permits) rather than stall.
	// Every port in the range is closed, so on most platforms the OS
	// refuses each dial almost instantly; this runs on the test's own
	// goroutine (bounded by `go test -timeout`) rather than a background
	// goroutine with a hand-rolled timeout, so nothing can be left running
	// after the test returns and race with a later test's use of stdout.
	from := 20000
	to := from + maxConcurrentPortScans*2 // > one full semaphore batch

	jsonOutput := true
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	out := captureStdout(t, func() {
		NmapHandler(ctx, "127.0.0.1", from, to, 1, 1, false, &jsonOutput)
	})

	if !strings.Contains(out, `"module_name": "nmap"`) {
		t.Errorf("expected valid nmap JSON output, got:\n%s", out)
	}
}

func TestNmapHandlerStopsScanningWhenContextExpires(t *testing.T) {
	// Verifies the fix for NmapHandler ignoring ctx during the scan loop: a
	// scan across many ports with a deliberately long per-dial timeout must
	// still return shortly after ctx's own deadline, instead of running
	// every remaining dial out to its full per-port timeout regardless.
	from := 30000
	to := from + maxConcurrentPortScans*4 // several times the semaphore size

	jsonOutput := true
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	captureStdout(t, func() {
		// A 10s per-dial timeout would make an unbounded scan of this
		// range take a very long time if ctx cancellation were ignored.
		NmapHandler(ctx, "127.0.0.1", from, to, 1, 10, false, &jsonOutput)
	})
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("NmapHandler took %v after a 300ms context deadline with a 10s per-dial timeout; expected it to stop scanning shortly after the deadline instead of running out every dial", elapsed)
	}
}
