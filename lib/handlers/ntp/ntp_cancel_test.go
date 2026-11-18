package ntp

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestNTPHandlerInterruptedReportsHowFarItGot(t *testing.T) {
	f := newFake()
	port := f.start(t, "udp4", "127.0.0.1")
	jsonOutput, throttle := false, false
	var ok bool
	ctx := testutil.CancelAfter(t, 400*time.Millisecond)
	out := testutil.RunWithin(t, 3*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 200, 40, &throttle, 2, port, 0, "127.0.0.1")
	})
	if ok {
		t.Error("an interrupted run must not report success")
	}
	completed, planned := testutil.InterruptedCounts(t, out)
	if planned != 200 || completed < 3 || completed >= 200 {
		t.Errorf("completed %d of %d, want a few of 200", completed, planned)
	}
	if got := strings.Count(out, "[ntp] OK response"); got != completed {
		t.Errorf("%d response lines for %d completed queries", got, completed)
	}
	if !strings.Contains(out, "[ntp] OK done queries="+strconv.Itoa(completed)+" answered="+strconv.Itoa(completed)) {
		t.Errorf("the done line counts what completed:\n%s", out)
	}
}

func TestNTPHandlerInterruptAbandonsAQueryWaitingForAReply(t *testing.T) {
	f := newFake()
	f.silent = true
	port := f.start(t, "udp4", "127.0.0.1")
	jsonOutput, throttle := false, false
	ctx := testutil.CancelAfter(t, 300*time.Millisecond)
	out := testutil.RunWithin(t, 2*time.Second, func() {
		Run(ctx, &jsonOutput, 5, 0, &throttle, 30, port, 0, "127.0.0.1")
	})
	completed, planned := testutil.InterruptedCounts(t, out)
	if completed != 0 || planned != 5 {
		t.Errorf("completed %d of %d, want 0 of 5", completed, planned)
	}
	if strings.Contains(out, "query failed") {
		t.Errorf("a query cut off by Ctrl+C is not a failed query:\n%s", out)
	}
}

func TestQueryNTPStopsWhenTheContextIsCancelled(t *testing.T) {
	f := newFake()
	f.silent = true
	port := f.start(t, "udp4", "127.0.0.1")
	ctx := testutil.CancelAfter(t, 200*time.Millisecond)
	start := time.Now()
	_, err := queryNTP(ctx, "127.0.0.1", port, 30)
	if err == nil || ctx.Err() == nil || time.Since(start) > 2*time.Second {
		t.Errorf("err = %v, ctx.Err() = %v after %v", err, ctx.Err(), time.Since(start))
	}
}
