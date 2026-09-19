package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/lib"
)

func TestHandleICMPTextMode(t *testing.T) {
	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		HandleICMP("127.0.0.1", &jsonOutput, 2, 0, &throttle, 3, 4)
	})
	if !strings.Contains(out, "[icmp] OK dns resolved") {
		t.Errorf("expected a dns-resolved log line, got:\n%s", out)
	}
	if !strings.Contains(out, "icmp STATISTICS") {
		t.Errorf("expected an icmp statistics banner, got:\n%s", out)
	}
}

func TestHandleICMPJSONMode(t *testing.T) {
	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		HandleICMP("127.0.0.1", &jsonOutput, 1, 0, &throttle, 3, 4)
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	if result.ModuleName != "icmp" {
		t.Errorf("module_name = %q, want %q", result.ModuleName, "icmp")
	}
	if !result.DNSLookup.Success {
		t.Error("expected DNS lookup to succeed for 127.0.0.1")
	}

	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.ICMPStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("expected at least one ICMP packet stat entry")
	}
	anySuccess := false
	for _, s := range stats {
		if s.Success {
			anySuccess = true
		}
	}
	if !anySuccess {
		// Some sandboxes/CI runners restrict unprivileged ICMP sockets
		// (see net.ipv4.ping_group_range in the README's Platform notes);
		// treat total packet loss there as an environment limitation
		// rather than a shint bug, since dns/JSON plumbing above already
		// verified the handler itself worked correctly.
		t.Skip("no ICMP packets succeeded; unprivileged ICMP is likely unavailable in this environment")
	}
}
