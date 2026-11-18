//go:build !windows

package testutil

import (
	"os"
	"syscall"
	"testing"
)

// RequireInterrupt is called first by the tests that stop a listener with
// Ctrl+C. Nothing to check on Unix.
func RequireInterrupt(t *testing.T) {
	t.Helper()
}

// SendInterrupt delivers SIGINT to the test process itself, which is how the
// listeners' "run until interrupted" mode is stopped in real use.
func SendInterrupt(t *testing.T) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("failed to signal SIGINT: %v", err)
	}
}
