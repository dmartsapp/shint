//go:build unix

package pace

import (
	"context"
	"testing"
	"time"
)

// A surplus attempt waits its turn; Ctrl+C while it waits ends the run instead of
// blocking on a slot forever.
func TestWaitingForASlotIsCutShortByCtrlC(t *testing.T) {
	slots := Slots(make(chan struct{}, 1))
	if !slots.Acquire(context.Background()) {
		t.Fatal("the first slot is free")
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	begin := time.Now()
	if slots.Acquire(ctx) {
		t.Error("acquired a slot that was not free")
	}
	if time.Since(begin) > time.Second {
		t.Errorf("Ctrl+C did not release the waiting attempt: %v", time.Since(begin))
	}
	slots.Release()
	if !slots.Acquire(context.Background()) {
		t.Error("a released slot can be taken again")
	}
}
