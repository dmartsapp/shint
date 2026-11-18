package pace

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestPauseStopsWhenTheContextIsCancelled(t *testing.T) {
	if !Pause(context.Background(), 20*time.Millisecond) {
		t.Error("an undisturbed pause reports that it elapsed")
	}
	if !Pause(context.Background(), 0) {
		t.Error("a zero pause is a no-op")
	}
	ctx := testutil.CancelAfter(t, 100*time.Millisecond)
	start := time.Now()
	if Pause(ctx, 30*time.Second) {
		t.Error("a cancelled pause must report that it did not elapse")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("the pause outlasted the cancellation")
	}
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if Pause(done, 0) {
		t.Error("with a context already cancelled, even a zero pause says stop")
	}
}

func TestWatchCancelUnblocksAPendingRead(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close(); _ = client.Close() }()
	ctx := testutil.CancelAfter(t, 100*time.Millisecond)
	stop := WatchCancel(ctx, client)
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
