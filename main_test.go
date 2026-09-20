package main

import "testing"

// TestScanContextHasNoDeadline pins the fix for nmap scans being cut short:
// the scan's context may be cancelled (Ctrl+C) but must never carry a
// deadline, in particular not one derived from --timeout, which is only the
// per-port connect timeout.
func TestScanContextHasNoDeadline(t *testing.T) {
	ctx, stop := scanContext()
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		t.Fatalf("scan context has a deadline (%v); --timeout must bound each port, not the whole scan", deadline)
	}
	if ctx.Err() != nil {
		t.Fatalf("scan context starts out cancelled: %v", ctx.Err())
	}
}
