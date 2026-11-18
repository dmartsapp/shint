package udp

import (
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestUDPHandlerInterruptAbandonsAProbeWaitingForAReply(t *testing.T) {
	port, closeFn := startSilentUDPServer(t)
	defer closeFn()
	jsonOutput, throttle := false, false
	var ok bool
	ctx := testutil.CancelAfter(t, 300*time.Millisecond)
	out := testutil.RunWithin(t, 2*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 5, 0, &throttle, 30, 4, "", port, "127.0.0.1")
	})
	if ok {
		t.Error("an interrupted run must not report success")
	}
	completed, planned := testutil.InterruptedCounts(t, out)
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
	ctx := testutil.CancelAfter(t, 400*time.Millisecond)
	out := testutil.RunWithin(t, 3*time.Second, func() {
		Run(ctx, &jsonOutput, 200, 40, &throttle, 2, 4, "hello", port, "127.0.0.1")
	})
	completed, planned := testutil.InterruptedCounts(t, out)
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
	ctx := testutil.CancelAfter(t, 200*time.Millisecond)
	start := time.Now()
	_, _, err := probeUDP(ctx, "127.0.0.1", port, 30, []byte("x"))
	if err == nil || ctx.Err() == nil {
		t.Errorf("err = %v, ctx.Err() = %v: a cut-off probe must say so, not report open|filtered", err, ctx.Err())
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("the probe waited %v after cancellation", took)
	}
}
