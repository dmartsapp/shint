package handlers

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dmartsapp/shint/lib"
)

// How the repeating checks (telnet, web, udp, ntp, rdns, wol) end when the user
// presses Ctrl+C. main gives each handler a context that is cancelled by
// SIGINT/SIGTERM and by nothing else - never a deadline, so --timeout still
// bounds one operation and never the run. On cancellation a handler
//
//   - stops launching attempts, and cuts short the --delay pause it may be in;
//   - abandons attempts still in flight: one that never finished is not one that
//     failed, so it is not counted (as nmap does with the ports it never got to);
//   - then finishes as it always does - the statistics for what did complete, the
//     "done" line, or the complete JSON document - with an "interrupted" line (or,
//     in JSON, the run-level error) saying how far it got, and reports false, so
//     the exit status is 1: the run was cut short, like an nmap scan.
//
// A second Ctrl+C is not caught (see interruptContext in main.go), so it ends
// the process at once if something is stuck.

// pause waits d, or until ctx is cancelled, and reports whether the whole pause
// elapsed.
func pause(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// watchCancel makes a read or write on conn that is in progress return promptly
// once ctx is cancelled, by expiring the connection's deadline; the caller then
// checks ctx.Err() to tell that apart from a real timeout. Call the returned
// function when done with conn.
func watchCancel(ctx context.Context, conn net.Conn) (stop func()) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetDeadline(time.Now())
		case <-done:
		}
	}()
	return func() { close(done) }
}

// interruptedNote is the run-level error a --json document carries when the run
// was cut short.
func interruptedNote(completed, planned int) string {
	return fmt.Sprintf("interrupted: %d of %d attempts completed", completed, planned)
}

// interruptedLine is the ERROR log line that says how far an interrupted run got.
func interruptedLine(module string, completed, planned int, started time.Time) string {
	return lib.LogWithTimestamp(module, "interrupted "+lib.Fields("attempts_completed", completed, "attempts_planned", planned, "time", time.Since(started)), true)
}
