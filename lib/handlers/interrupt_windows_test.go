//go:build windows

package handlers

import "testing"

// A Windows process cannot send itself SIGINT (syscall.Kill does not exist
// there), so the tests that stop a listener with Ctrl+C are skipped. They still
// compile, which is what lets `go vet` and `go test` build the package.
func requireInterrupt(t *testing.T) {
	t.Helper()
	t.Skip("a test process cannot deliver SIGINT to itself on Windows")
}

func sendInterrupt(t *testing.T) {
	t.Helper()
	t.Fatal("sendInterrupt is not available on Windows; requireInterrupt should have skipped the test")
}
