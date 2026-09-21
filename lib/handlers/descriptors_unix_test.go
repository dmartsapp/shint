//go:build unix

package handlers

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// lowerDescriptorLimit makes this process's soft descriptor limit 128 for the rest
// of the test, the way a container or a script that ran "ulimit -n 128" would. (A
// constant, because Rlimit.Cur is a uint64 on most systems and an int64 on some.)
func lowerDescriptorLimit(t *testing.T) {
	t.Helper()
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old); err != nil {
		t.Skipf("cannot read the descriptor limit: %v", err)
	}
	lowered := old
	lowered.Cur = 128
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lowered); err != nil {
		t.Skipf("cannot lower the descriptor limit: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old) })
}

// nmap's own bound (500) is lowered the same way, and the line that announces the
// scan says what it is.
func TestNmapInFlightFollowsTheDescriptorLimit(t *testing.T) {
	lowerDescriptorLimit(t)
	var running, peak int32
	var mu sync.Mutex
	withProbe(t, func(ctx context.Context, host string, port, timeout int) (bool, error) {
		n := atomic.AddInt32(&running, 1)
		mu.Lock()
		if n > peak {
			peak = n
		}
		mu.Unlock()
		time.Sleep(15 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return false, nil
	})
	jsonOutput := false
	out := captureStdout(t, func() {
		NmapHandler(context.Background(), "127.0.0.1", 20000, 20000+399, 1, 5, false, &jsonOutput)
	})
	want := lib.InFlightLimit(maxConcurrentPortScans)
	if want != 64 {
		t.Fatalf("with a limit of 128 descriptors the bound should be 64, got %d", want)
	}
	if peak > int32(want) || peak < 2 {
		t.Errorf("%d probes ran at once, want at most %d", peak, want)
	}
	if !strings.Contains(out, "max_in_flight="+strconv.Itoa(want)) {
		t.Errorf("the scan line should say max_in_flight=%d:\n%.600s", want, out)
	}
}

// A surplus attempt waits its turn; Ctrl+C while it waits ends the run instead of
// blocking on a slot forever.
func TestWaitingForASlotIsCutShortByCtrlC(t *testing.T) {
	slots := attemptSlots(make(chan struct{}, 1))
	if !slots.acquire(context.Background()) {
		t.Fatal("the first slot is free")
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	begin := time.Now()
	if slots.acquire(ctx) {
		t.Error("acquired a slot that was not free")
	}
	if time.Since(begin) > time.Second {
		t.Errorf("Ctrl+C did not release the waiting attempt: %v", time.Since(begin))
	}
	slots.release()
	if !slots.acquire(context.Background()) {
		t.Error("a released slot can be taken again")
	}
}
