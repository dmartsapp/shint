package handlers

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// maxInFlightAttempts is the most attempts telnet, web and udp run at once (nmap
// has its own bound, maxConcurrentPortScans, of the same size): each one holds a
// socket, and --count 3000 --delay 0 would otherwise open three thousand.
const maxInFlightAttempts = 500

// attemptSlots bounds how many attempts run at the same time. The launcher takes a
// slot before it starts an attempt and the attempt gives it back when it ends, so
// with --delay 0 the surplus simply waits its turn; lib.InFlightLimit lowers the
// bound where the process may open few files.
type attemptSlots chan struct{}

func newAttemptSlots() attemptSlots {
	return make(attemptSlots, lib.InFlightLimit(maxInFlightAttempts))
}

// acquire waits for a free slot, and reports false if Ctrl+C came first.
func (s attemptSlots) acquire(ctx context.Context) bool {
	select {
	case s <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s attemptSlots) release() { <-s }

// attemptDelay is how long to pause before an attempt: --delay milliseconds,
// or with --throttle a random 0-10 seconds instead, to imitate uneven traffic.
// Like every command, it is applied before each attempt, including the first
// (see "Flags shared by every command" in docs/src/usage.md).
func attemptDelay(delayMs int, throttle bool) time.Duration {
	if throttle {
		n, err := rand.Int(rand.Reader, big.NewInt(10000))
		if err == nil {
			return time.Duration(n.Int64()) * time.Millisecond
		}
	}
	return time.Duration(delayMs) * time.Millisecond
}
