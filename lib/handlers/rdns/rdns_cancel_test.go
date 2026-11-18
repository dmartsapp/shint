package rdns

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestRDNSHandlerInterrupted(t *testing.T) {
	calls := 0
	withLookup(t, func(ctx context.Context, address string) ([]string, error) {
		calls++
		if calls == 4 { // the fourth lookup hangs until Ctrl+C
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []string{"host.example."}, nil
	})
	jsonOutput, throttle := false, false
	var ok bool
	ctx := testutil.CancelAfter(t, 300*time.Millisecond)
	out := testutil.RunWithin(t, 3*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 50, 20, &throttle, 30, "192.0.2.10")
	})
	if ok {
		t.Error("an interrupted run must not report success")
	}
	completed, planned := testutil.InterruptedCounts(t, out)
	if planned != 50 || completed != 3 {
		t.Errorf("completed %d of %d, want the 3 lookups that finished before the hung one", completed, planned)
	}
	if strings.Contains(out, "reverse lookup failed") {
		t.Errorf("the lookup cut off by Ctrl+C is not a failed lookup:\n%s", out)
	}
	if !strings.Contains(out, "[rdns] OK done lookups=3 resolved=3") {
		t.Errorf("the done line counts what completed:\n%s", out)
	}
}
