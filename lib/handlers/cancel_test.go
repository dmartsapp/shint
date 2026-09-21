package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// cancelAfter returns a context that is cancelled (as Ctrl+C would) after d.
func cancelAfter(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(d, cancel)
	t.Cleanup(func() { timer.Stop(); cancel() })
	return ctx
}

var interruptedRe = regexp.MustCompile(`ERROR interrupted attempts_completed=(\d+) attempts_planned=(\d+)`)

// interruptedCounts pulls the numbers out of the "interrupted" line.
func interruptedCounts(t *testing.T, out string) (completed, planned int) {
	t.Helper()
	m := interruptedRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no \"interrupted\" line in the output:\n%s", out)
	}
	completed, _ = strconv.Atoi(m[1])
	planned, _ = strconv.Atoi(m[2])
	return completed, planned
}

func runWithin(t *testing.T, limit time.Duration, fn func()) string {
	t.Helper()
	start := time.Now()
	out := captureStdout(t, fn)
	if took := time.Since(start); took > limit {
		t.Errorf("the run took %v after Ctrl+C, want it to wind down within %v", took, limit)
	}
	return out
}

// ---- telnet

func TestTelnetHandlerInterruptedReportsHowFarItGot(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()
	jsonOutput, throttle := false, false
	var ok bool
	ctx := cancelAfter(t, 400*time.Millisecond)
	out := runWithin(t, 3*time.Second, func() {
		ok = TelnetHandler(ctx, &jsonOutput, 200, 40, &throttle, 2, 4, port, "127.0.0.1")
	})
	if ok {
		t.Error("an interrupted run was cut short: it must not report success")
	}
	completed, planned := interruptedCounts(t, out)
	if planned != 200 || completed < 3 || completed >= 200 {
		t.Errorf("completed %d of %d, want a few of 200", completed, planned)
	}
	if !strings.Contains(out, "Requests sent: "+strconv.Itoa(completed)+", Response received: "+strconv.Itoa(completed)) {
		t.Errorf("the statistics must describe the %d attempts that ran:\n%s", completed, out)
	}
	if got := strings.Count(out, "connect ok"); got != completed {
		t.Errorf("%d \"connect ok\" lines for %d completed attempts", got, completed)
	}
	if strings.Contains(out, "connect failed") {
		t.Errorf("an attempt cut off by Ctrl+C is not a failure:\n%s", out)
	}
	if !strings.Contains(out, "[telnet] OK done") {
		t.Errorf("an interrupted run still ends with its done line:\n%s", out)
	}
}

// The --delay before an attempt is cut short too: a long delay must not make
// Ctrl+C wait it out.
func TestTelnetHandlerInterruptCutsTheDelayShort(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()
	jsonOutput, throttle := false, false
	ctx := cancelAfter(t, 200*time.Millisecond)
	out := runWithin(t, 1500*time.Millisecond, func() {
		TelnetHandler(ctx, &jsonOutput, 5, 30000, &throttle, 2, 4, port, "127.0.0.1")
	})
	completed, planned := interruptedCounts(t, out)
	if completed != 0 || planned != 5 {
		t.Errorf("completed %d of %d, want 0 of 5 (still in the first 30 s delay)", completed, planned)
	}
	if !strings.Contains(out, "Requests sent: 0") {
		t.Errorf("no attempt ran:\n%s", out)
	}
}

func TestTelnetHandlerInterruptedJSONIsOneCompleteDocument(t *testing.T) {
	port, closeFn := startEchoListener(t)
	defer closeFn()
	jsonOutput, throttle := true, false
	ctx := cancelAfter(t, 350*time.Millisecond)
	var ok bool
	out := runWithin(t, 3*time.Second, func() {
		ok = TelnetHandler(ctx, &jsonOutput, 200, 40, &throttle, 2, 4, port, "127.0.0.1")
	})
	var doc struct {
		Error string `json:"error"`
		Stats []struct {
			Success bool `json:"success"`
		} `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`^interrupted: (\d+) of 200 attempts completed$`).FindStringSubmatch(doc.Error)
	if ok || m == nil {
		t.Fatalf("ok=%v, error=%q", ok, doc.Error)
	}
	if n, _ := strconv.Atoi(m[1]); n != len(doc.Stats) || n == 0 {
		t.Errorf("error says %s attempts completed, stats has %d", m[1], len(doc.Stats))
	}
}

// ---- web

func TestWebHandlerInterruptedReportsHowFarItGot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	jsonOutput, throttle := false, false
	var ok bool
	ctx := cancelAfter(t, 400*time.Millisecond)
	out := runWithin(t, 3*time.Second, func() {
		ok = WebHandler(ctx, &jsonOutput, 200, 40, &throttle, 5, u, "GET", "", nil, false, nil)
	})
	if ok {
		t.Error("an interrupted run must not report success")
	}
	completed, planned := interruptedCounts(t, out)
	if planned != 200 || completed < 3 || completed >= 200 {
		t.Errorf("completed %d of %d, want a few of 200", completed, planned)
	}
	if !strings.Contains(out, "Requests sent: "+strconv.Itoa(completed)+", Response received: "+strconv.Itoa(completed)) || !strings.Contains(out, "[web] OK done") {
		t.Errorf("the statistics and the done line must follow:\n%s", out)
	}
	if strings.Contains(out, "request failed") {
		t.Errorf("an attempt cut off by Ctrl+C is not a failure:\n%s", out)
	}
}

// A request still waiting for the server when Ctrl+C arrives is abandoned - not
// counted, not reported as a failure - and does not hold the run up.
func TestWebHandlerInterruptAbandonsARequestInFlight(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	u, _ := url.Parse(srv.URL)
	jsonOutput, throttle := false, false
	ctx := cancelAfter(t, 300*time.Millisecond)
	out := runWithin(t, 2*time.Second, func() {
		WebHandler(ctx, &jsonOutput, 3, 0, &throttle, 30, u, "GET", "", nil, false, nil)
	})
	completed, planned := interruptedCounts(t, out)
	if completed != 0 || planned != 3 {
		t.Errorf("completed %d of %d, want 0 of 3", completed, planned)
	}
	if strings.Contains(out, "request failed") || strings.Contains(out, "response url=") {
		t.Errorf("the abandoned request must leave no result:\n%s", out)
	}
}

func TestWebHandlerInterruptedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	jsonOutput, throttle := true, false
	ctx := cancelAfter(t, 350*time.Millisecond)
	var ok bool
	out := runWithin(t, 3*time.Second, func() {
		ok = WebHandler(ctx, &jsonOutput, 200, 40, &throttle, 5, u, "GET", "", nil, false, nil)
	})
	var doc struct {
		Error string        `json:"error"`
		Stats []interface{} `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`^interrupted: (\d+) of 200 attempts completed$`).FindStringSubmatch(doc.Error)
	if ok || m == nil {
		t.Fatalf("ok=%v, error=%q", ok, doc.Error)
	}
	if n, _ := strconv.Atoi(m[1]); n != len(doc.Stats) || n == 0 {
		t.Errorf("error says %s attempts completed, stats has %d", m[1], len(doc.Stats))
	}
}

// ---- udp

func TestUDPHandlerInterruptAbandonsAProbeWaitingForAReply(t *testing.T) {
	port, closeFn := startSilentUDPServer(t)
	defer closeFn()
	jsonOutput, throttle := false, false
	var ok bool
	ctx := cancelAfter(t, 300*time.Millisecond)
	out := runWithin(t, 2*time.Second, func() {
		ok = UDPHandler(ctx, &jsonOutput, 5, 0, &throttle, 30, 4, "", port, "127.0.0.1")
	})
	if ok {
		t.Error("an interrupted run must not report success")
	}
	completed, planned := interruptedCounts(t, out)
	if completed != 0 || planned != 5 {
		t.Errorf("completed %d of %d, want 0 of 5", completed, planned)
	}
	if strings.Contains(out, "probe open|filtered") || strings.Contains(out, "probe error") {
		t.Errorf("a probe cut off by Ctrl+C says nothing about the port:\n%s", out)
	}
	if !strings.Contains(out, "probes_sent=0") {
		t.Errorf("the done line counts the probes that finished:\n%s", out)
	}
}

func TestUDPHandlerInterruptedReportsHowFarItGot(t *testing.T) {
	port, closeFn := startEchoUDPServer(t)
	defer closeFn()
	jsonOutput, throttle := false, false
	ctx := cancelAfter(t, 400*time.Millisecond)
	out := runWithin(t, 3*time.Second, func() {
		UDPHandler(ctx, &jsonOutput, 200, 40, &throttle, 2, 4, "hello", port, "127.0.0.1")
	})
	completed, planned := interruptedCounts(t, out)
	if planned != 200 || completed < 3 || completed >= 200 {
		t.Errorf("completed %d of %d, want a few of 200", completed, planned)
	}
	if got := strings.Count(out, "probe open"); got != completed {
		t.Errorf("%d \"probe open\" lines for %d completed probes", got, completed)
	}
}

func TestProbeUDPStopsWhenTheContextIsCancelled(t *testing.T) {
	port, closeFn := startSilentUDPServer(t)
	defer closeFn()
	ctx := cancelAfter(t, 200*time.Millisecond)
	start := time.Now()
	_, _, err := probeUDP(ctx, "127.0.0.1", port, 30, []byte("x"))
	if err == nil || ctx.Err() == nil {
		t.Errorf("err = %v, ctx.Err() = %v: a cut-off probe must say so, not report open|filtered", err, ctx.Err())
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("the probe waited %v after cancellation", took)
	}
}

// ---- the helpers

func TestPauseStopsWhenTheContextIsCancelled(t *testing.T) {
	if !pause(context.Background(), 20*time.Millisecond) {
		t.Error("an undisturbed pause reports that it elapsed")
	}
	if !pause(context.Background(), 0) {
		t.Error("a zero pause is a no-op")
	}
	ctx := cancelAfter(t, 100*time.Millisecond)
	start := time.Now()
	if pause(ctx, 30*time.Second) {
		t.Error("a cancelled pause must report that it did not elapse")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("the pause outlasted the cancellation")
	}
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if pause(done, 0) {
		t.Error("with a context already cancelled, even a zero pause says stop")
	}
}

func TestWatchCancelUnblocksAPendingRead(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close(); _ = client.Close() }()
	ctx := cancelAfter(t, 100*time.Millisecond)
	stop := watchCancel(ctx, client)
	defer stop()
	start := time.Now()
	buf := make([]byte, 1)
	if _, err := client.Read(buf); err == nil {
		t.Fatal("the read should have failed once the deadline expired")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("the read outlasted the cancellation")
	}
}
