package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dmartsapp/go-ping/v2/netutils"
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

// TestConfigurePingerAppliesTimeout is the regression test for --timeout being
// ignored by ping: the value must reach the library as the per-reply wait
// (whole seconds -> milliseconds). It used to stay at the library's fixed
// one second whatever the flag said.
func TestConfigurePingerAppliesTimeout(t *testing.T) {
	for _, seconds := range []int{1, 3, 10} {
		pinger := new(netutils.Pinger)
		configurePinger(pinger, 2, 250, false, seconds, 16)
		if pinger.ReplyTimeoutMS != seconds*1000 {
			t.Errorf("--timeout %d gave ReplyTimeoutMS %d, want %d", seconds, pinger.ReplyTimeoutMS, seconds*1000)
		}
		if pinger.Count != 2 || pinger.PingDelay != 250 || len(pinger.Payload) != 16 {
			t.Errorf("other flags were not applied: count=%d delay=%d payload=%d", pinger.Count, pinger.PingDelay, len(pinger.Payload))
		}
	}
}

func TestValidatePingPayload(t *testing.T) {
	max := MaxPingPayload()
	if max <= 0 || max > 65507 {
		t.Fatalf("MaxPingPayload() = %d, want a sane positive size", max)
	}
	for _, ok := range []int{0, 4, max} {
		if err := ValidatePingPayload(ok); err != nil {
			t.Errorf("payload %d should be valid: %v", ok, err)
		}
	}
	for _, bad := range []int{-1, max + 1, 65535} {
		if err := ValidatePingPayload(bad); err == nil {
			t.Errorf("payload %d should be rejected", bad)
		}
	}
	// The limit is the library's own: a value it would silently clamp is rejected.
	if got := len(new(netutils.Pinger).SetPayloadSizeInBytes(max + 100).Payload); got != max {
		t.Errorf("library clamps to %d but MaxPingPayload() says %d", got, max)
	}
}

func TestIsLostPing(t *testing.T) {
	for line, want := range map[string]bool{
		"received reply for request #1 from 127.0.0.1 (ipv4) in 0ms":        false,
		"received reply for request #2 from 2607:f8b0::200e (ipv6) in 31ms": false,
		"no reply for request #1 from 192.0.2.1: read udp: i/o timeout":     true,
		"no reply for request #3 from 2001:db8::1: read udp: i/o timeout":   true,
	} {
		if got := isLostPing(line); got != want {
			t.Errorf("isLostPing(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestWithPayloadSize(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"received reply for request #1 from 127.0.0.1 (ipv4) in 0ms", "received reply for request #1 from 127.0.0.1 (ipv4) in 0ms bytes=56"},
		{"received reply for request #2 from 2607:f8b0::200e (ipv6) in 31ms", "received reply for request #2 from 2607:f8b0::200e (ipv6) in 31ms bytes=56"},
		// a lost request has no reply to size, and other lines are left alone
		{"no reply for request #1 from 192.0.2.1: read udp: i/o timeout", "no reply for request #1 from 192.0.2.1: read udp: i/o timeout"},
		{"error sending request #1 to 192.0.2.1: boom", "error sending request #1 to 192.0.2.1: boom"},
	} {
		if got := withPayloadSize(tc.line, 56); got != tc.want {
			t.Errorf("withPayloadSize(%q, 56) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// TestHandleICMPShowsPayloadSize: every reply line carries the payload size
// (bytes=N, like the size on a real ping reply) and the JSON stats carry it as
// payload_size_bytes - a field that used to be 0 in every document.
func TestHandleICMPShowsPayloadSize(t *testing.T) {
	const payload = 32
	throttle := false

	jsonOutput := false
	text := captureStdout(t, func() {
		HandleICMP("127.0.0.1", &jsonOutput, 2, 0, &throttle, 3, payload)
	})
	replies := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "received reply") {
			replies++
			if !strings.HasSuffix(line, " bytes=32") {
				t.Errorf("reply line does not end with the payload size:\n%s", line)
			}
		}
	}
	if replies == 0 {
		t.Skip("no ICMP replies; unprivileged ICMP is likely unavailable in this environment")
	}

	jsonOutput = true
	out := captureStdout(t, func() {
		HandleICMP("127.0.0.1", &jsonOutput, 2, 0, &throttle, 3, payload)
	})
	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.ICMPStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats) == 0 {
		t.Fatal("no stats")
	}
	for i, s := range stats {
		if s.PayloadSize != payload {
			t.Errorf("stats[%d].payload_size_bytes = %d, want %d", i, s.PayloadSize, payload)
		}
	}
}
