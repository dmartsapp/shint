package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
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

// withProbe swaps the single-port check for the duration of a test.
func withProbe(t *testing.T, fake func(ctx context.Context, host string, port, timeout int) (bool, error)) {
	t.Helper()
	orig := probePort
	probePort = fake
	t.Cleanup(func() { probePort = orig })
}

func nmapStatsFromJSON(t *testing.T, out string) ([]lib.NmapStats, lib.JSONOutput) {
	t.Helper()
	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.NmapStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	return stats, result
}

// TestNmapHandlerCoversWholeRangeEvenWhenItOutlastsPerPortTimeout is the
// regression test for scans being cut short: the per-port timeout (the
// `timeout` argument) is what each port gets, not a budget for the range, so
// a scan that takes several times longer than it - many unresponsive ports,
// each running out its own dial - must still visit every port and find the
// open ones at the far end.
func TestNmapHandlerCoversWholeRangeEvenWhenItOutlastsPerPortTimeout(t *testing.T) {
	const perDial = 400 * time.Millisecond
	withProbe(t, func(ctx context.Context, host string, port, timeout int) (bool, error) {
		time.Sleep(perDial) // an unresponsive port, answering just inside its timeout
		if port == 1000 || port == 1100 {
			return true, nil
		}
		return false, errors.New("closed")
	})

	// 1..1100 is 3 waves of the 500-dial cap: ~1.2s, past the 1s per-port timeout.
	jsonOutput := true
	out := captureStdout(t, func() {
		NmapHandler(context.Background(), "127.0.0.1", 1, 1100, 1, 1, false, &jsonOutput)
	})

	stats, result := nmapStatsFromJSON(t, out)
	if len(stats) != 1100 {
		t.Fatalf("scanned %d of 1100 ports", len(stats))
	}
	open := map[int]bool{}
	for _, s := range stats {
		if s.Success {
			open[s.Port] = true
		}
	}
	if !open[1000] || !open[1100] || len(open) != 2 {
		t.Errorf("open ports = %v, want exactly 1000 and 1100 (the last port of the range included)", open)
	}
	if result.Error != "" {
		t.Errorf("a complete scan reported an error: %q", result.Error)
	}
}

func TestNmapHandlerTextSummaryCountsFinishedPorts(t *testing.T) {
	withProbe(t, func(ctx context.Context, host string, port, timeout int) (bool, error) {
		if port == 7 {
			return true, nil
		}
		return false, errors.New("closed")
	})
	jsonOutput := false
	out := captureStdout(t, func() {
		NmapHandler(context.Background(), "127.0.0.1", 1, 20, 1, 1, false, &jsonOutput)
	})
	if !strings.Contains(out, "[nmap] OK scan complete ports_scanned=20 open=1") {
		t.Errorf("expected a complete-scan summary for 20 ports with 1 open, got:\n%s", out)
	}
}

// TestNmapHandlerReportsInterruptedScan: a cancelled scan says how far it
// got, and dials aborted mid-flight are not recorded as closed ports.
func TestNmapHandlerReportsInterruptedScan(t *testing.T) {
	withProbe(t, func(ctx context.Context, host string, port, timeout int) (bool, error) {
		if port <= 100 {
			return false, errors.New("closed") // answers at once
		}
		<-ctx.Done() // never answers until the scan is cancelled
		return false, ctx.Err()
	})

	for _, tc := range []struct {
		name string
		json bool
	}{{"text", false}, {"json", true}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			jsonOutput := tc.json
			out := captureStdout(t, func() {
				NmapHandler(ctx, "127.0.0.1", 1, 2000, 1, 30, false, &jsonOutput)
			})

			if tc.json {
				stats, result := nmapStatsFromJSON(t, out)
				if !strings.Contains(result.Error, "scan interrupted") || !strings.Contains(result.Error, "of 2000 ports") {
					t.Errorf("expected an interrupted-scan error naming the 2000-port plan, got %q", result.Error)
				}
				for _, s := range stats {
					if s.Port > 100 {
						t.Errorf("port %d was aborted mid-dial but recorded as a result (success=%v)", s.Port, s.Success)
					}
				}
				return
			}
			if !strings.Contains(out, "[nmap] ERROR scan interrupted") || !strings.Contains(out, "ports_total=2000") {
				t.Errorf("expected an interrupted-scan error line with ports_total=2000, got:\n%s", out)
			}
			if strings.Contains(out, "scan complete") {
				t.Errorf("an interrupted scan must not also claim to be complete:\n%s", out)
			}
		})
	}
}

// TestNmapHandlerThrottleDelayIsInterruptible: --throttle waits up to 10s
// before each port; cancelling the scan must not have to sit that out.
func TestNmapHandlerThrottleDelayIsInterruptible(t *testing.T) {
	withProbe(t, func(ctx context.Context, host string, port, timeout int) (bool, error) {
		return false, errors.New("closed")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	jsonOutput := false

	start := time.Now()
	captureStdout(t, func() {
		NmapHandler(ctx, "127.0.0.1", 1, 50, 1, 1, true, &jsonOutput)
	})
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("throttled scan took %v to stop after its context was cancelled at 200ms", elapsed)
	}
}

// slowClosedProbe is an unresponsive port: it answers "closed", but only
// after a delay, so batches of dials finish in bursts the way they do against
// a host that drops packets.
func slowClosedProbe(delay time.Duration) func(context.Context, string, int, int) (bool, error) {
	return func(ctx context.Context, host string, port, timeout int) (bool, error) {
		time.Sleep(delay)
		return false, errors.New("closed")
	}
}

func setProgressInterval(t *testing.T, d time.Duration) {
	t.Helper()
	orig := progressInterval
	progressInterval = d
	t.Cleanup(func() { progressInterval = orig })
}

// TestNmapHandlerReportsProgress: a slow text-mode scan says how big it is up
// front and then how far along it is, so it never looks hung.
func TestNmapHandlerReportsProgress(t *testing.T) {
	withProbe(t, slowClosedProbe(300*time.Millisecond))
	setProgressInterval(t, 100*time.Millisecond)

	// 1..1100 is 3 waves of the 500-dial cap: ~900ms, several ticks.
	jsonOutput := false
	out := captureStdout(t, func() {
		NmapHandler(context.Background(), "127.0.0.1", 1, 1100, 1, 5, false, &jsonOutput)
	})

	if !strings.Contains(out, "[nmap] OK scan started ports_total=1100 timeout=5s max_in_flight=500") {
		t.Errorf("expected a scan-started line naming the scope, got:\n%s", out)
	}

	re := regexp.MustCompile(`\[nmap\] OK progress ports_scanned=(\d+)/1100 percent=[\d.]+% open=0 in_flight=(\d+) elapsed=\S+`)
	matches := re.FindAllStringSubmatch(out, -1)
	if len(matches) < 3 {
		t.Fatalf("expected several progress lines over a ~900ms scan ticking every 100ms, got %d:\n%s", len(matches), out)
	}
	last, sawInFlight := -1, false
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		if n < last {
			t.Errorf("progress went backwards: %d after %d", n, last)
		}
		last = n
		if inFlight, _ := strconv.Atoi(m[2]); inFlight > 0 {
			sawInFlight = true
		}
	}
	if !sawInFlight {
		t.Errorf("no progress line reported probes in flight, though the scan was waiting on them:\n%s", out)
	}

	// Progress precedes the summary, and never trails after it.
	summary := strings.Index(out, "scan complete ports_scanned=1100")
	if summary < 0 {
		t.Fatalf("missing scan-complete summary:\n%s", out)
	}
	if idx := strings.LastIndex(out, "] OK progress"); idx > summary {
		t.Errorf("a progress line was printed after the scan-complete summary:\n%s", out)
	}
}

// TestNmapHandlerFastScanPrintsNoProgress: a scan that finishes before the
// first tick stays quiet apart from its start line and summary.
func TestNmapHandlerFastScanPrintsNoProgress(t *testing.T) {
	withProbe(t, func(ctx context.Context, host string, port, timeout int) (bool, error) {
		return false, errors.New("closed")
	})
	jsonOutput := false
	out := captureStdout(t, func() {
		NmapHandler(context.Background(), "127.0.0.1", 1, 50, 1, 1, false, &jsonOutput)
	})
	if strings.Contains(out, "progress") {
		t.Errorf("a scan that finished well inside the progress interval printed progress:\n%s", out)
	}
	if !strings.Contains(out, "scan started ports_total=50") || !strings.Contains(out, "scan complete ports_scanned=50") {
		t.Errorf("expected start and complete lines, got:\n%s", out)
	}
}

// TestNmapHandlerJSONHasNoProgressLines: --json output is one document a
// program parses; progress text must never leak into it.
func TestNmapHandlerJSONHasNoProgressLines(t *testing.T) {
	withProbe(t, slowClosedProbe(300*time.Millisecond))
	setProgressInterval(t, 50*time.Millisecond)

	jsonOutput := true
	out := captureStdout(t, func() {
		NmapHandler(context.Background(), "127.0.0.1", 1, 600, 1, 5, false, &jsonOutput)
	})
	stats, _ := nmapStatsFromJSON(t, out) // fails the test if anything but the JSON document was printed
	if len(stats) != 600 {
		t.Errorf("scanned %d of 600 ports", len(stats))
	}
	if strings.Contains(out, "progress") || strings.Contains(out, "scan started") {
		t.Errorf("text-mode progress leaked into --json output:\n%s", out)
	}
}
