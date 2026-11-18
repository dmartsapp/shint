package listen

import (
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestReachableFromNetwork(t *testing.T) {
	for bind, want := range map[string]bool{
		"0.0.0.0": true, "::": true, "[::]": true, "": true, "192.168.1.5": true, "fe80::1": true, "myhost.example": true,
		"127.0.0.1": false, "127.0.0.2": false, "::1": false, "[::1]": false, "localhost": false, "LOCALHOST": false,
	} {
		if got := reachableFromNetwork(bind); got != want {
			t.Errorf("reachableFromNetwork(%q) = %v, want %v", bind, got, want)
		}
	}
}

func TestExposureNotice(t *testing.T) {
	out := testutil.CaptureStdout(t, func() { exposureNotice(listenHTTPModule, "0.0.0.0", "0.0.0.0:8080") })
	if !strings.Contains(out, "] WARN reachable from other machines address=0.0.0.0:8080 ") || !strings.Contains(out, "--bind 127.0.0.1") {
		t.Errorf("notice: %q", out)
	}
	if out := testutil.CaptureStdout(t, func() { exposureNotice(listenHTTPModule, "127.0.0.1", "127.0.0.1:8080") }); out != "" {
		t.Errorf("a loopback listener printed a notice: %q", out)
	}
}
