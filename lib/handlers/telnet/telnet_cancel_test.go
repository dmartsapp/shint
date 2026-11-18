package telnet

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestTelnetHandlerInterruptedReportsHowFarItGot(t *testing.T) {
	port, closeFn := testutil.EchoListener(t)
	defer closeFn()
	jsonOutput, throttle := false, false
	var ok bool
	ctx := testutil.CancelAfter(t, 400*time.Millisecond)
	out := testutil.RunWithin(t, 3*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 200, 40, &throttle, 2, 4, port, "127.0.0.1")
	})
	if ok {
		t.Error("an interrupted run was cut short: it must not report success")
	}
	completed, planned := testutil.InterruptedCounts(t, out)
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
	port, closeFn := testutil.EchoListener(t)
	defer closeFn()
	jsonOutput, throttle := false, false
	ctx := testutil.CancelAfter(t, 200*time.Millisecond)
	out := testutil.RunWithin(t, 1500*time.Millisecond, func() {
		Run(ctx, &jsonOutput, 5, 30000, &throttle, 2, 4, port, "127.0.0.1")
	})
	completed, planned := testutil.InterruptedCounts(t, out)
	if completed != 0 || planned != 5 {
		t.Errorf("completed %d of %d, want 0 of 5 (still in the first 30 s delay)", completed, planned)
	}
	if !strings.Contains(out, "Requests sent: 0") {
		t.Errorf("no attempt ran:\n%s", out)
	}
}

func TestTelnetHandlerInterruptedJSONIsOneCompleteDocument(t *testing.T) {
	port, closeFn := testutil.EchoListener(t)
	defer closeFn()
	jsonOutput, throttle := true, false
	ctx := testutil.CancelAfter(t, 350*time.Millisecond)
	var ok bool
	out := testutil.RunWithin(t, 3*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 200, 40, &throttle, 2, 4, port, "127.0.0.1")
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
