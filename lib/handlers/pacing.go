package handlers

import (
	"crypto/rand"
	"math/big"
	"time"
)

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
