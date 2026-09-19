package handlers

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/lib"
)

// startEchoUDPServer listens on a UDP port and echoes back whatever it
// receives, so probeUDP/UDPHandler can be tested against an "open" service.
func startEchoUDPServer(t *testing.T) (port int, closeFn func()) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("failed to start UDP echo server: %v", err)
	}
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = conn.WriteToUDP(buf[:n], addr)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port, func() { conn.Close() }
}

// startSilentUDPServer listens on a UDP port but never replies, so probes
// against it should time out into the "open|filtered" state.
func startSilentUDPServer(t *testing.T) (port int, closeFn func()) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("failed to start silent UDP server: %v", err)
	}
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, _, err := conn.ReadFromUDP(buf); err != nil {
				return
			}
			// Deliberately never respond.
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port, func() { conn.Close() }
}

func TestProbeUDPOpen(t *testing.T) {
	port, closeFn := startEchoUDPServer(t)
	defer closeFn()

	state, received, err := probeUDP("127.0.0.1", port, 2, []byte("ping"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != "open" {
		t.Errorf("state = %q, want %q", state, "open")
	}
	if string(received) != "ping" {
		t.Errorf("received = %q, want %q", received, "ping")
	}
}

func TestProbeUDPOpenFiltered(t *testing.T) {
	port, closeFn := startSilentUDPServer(t)
	defer closeFn()

	state, received, err := probeUDP("127.0.0.1", port, 1, []byte("ping"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != "open|filtered" {
		t.Errorf("state = %q, want %q", state, "open|filtered")
	}
	if len(received) != 0 {
		t.Errorf("expected no data received, got %q", received)
	}
}

func TestProbeUDPClosed(t *testing.T) {
	// Bind then immediately close a UDP port so nothing is listening; on
	// most platforms this triggers a "connection refused" ICMP-unreachable
	// surfaced by the OS on the next read from a connected UDP socket.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("failed to find a free UDP port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	state, _, err := probeUDP("127.0.0.1", port, 2, []byte("ping"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != "closed" {
		t.Errorf("state = %q, want %q (this can be platform-dependent for ICMP unreachable delivery)", state, "closed")
	}
}

func TestUDPHandlerJSONOpen(t *testing.T) {
	port, closeFn := startEchoUDPServer(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		UDPHandler(&jsonOutput, 1, 0, &throttle, 2, 4, "hello", port, "127.0.0.1")
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.UDPStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected one stat entry, got %d", len(stats))
	}
	if stats[0].State != "open" {
		t.Errorf("state = %q, want %q", stats[0].State, "open")
	}
	if stats[0].BytesSent != len("hello") {
		t.Errorf("bytes_sent = %d, want %d", stats[0].BytesSent, len("hello"))
	}
}

func TestUDPHandlerDNSFailure(t *testing.T) {
	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		UDPHandler(&jsonOutput, 1, 0, &throttle, 1, 4, "", 53, "this-host-should-not-exist.invalid")
	})
	if !strings.Contains(out, "[udp] ERROR dns resolution failed") {
		t.Errorf("expected a dns-resolution-failed error line, got:\n%s", out)
	}
}

func TestUDPHandlerUsesGeneratedPayloadWhenDataEmpty(t *testing.T) {
	port, closeFn := startEchoUDPServer(t)
	defer closeFn()

	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		UDPHandler(&jsonOutput, 1, 0, &throttle, 2, 6, "", port, "127.0.0.1")
	})

	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput:\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.UDPStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if len(stats) != 1 || stats[0].BytesSent != 6 {
		t.Fatalf("expected 6-byte generated payload to be sent, got %+v", stats)
	}
}

func TestUDPHandlerTextModeMultipleAttempts(t *testing.T) {
	port, closeFn := startEchoUDPServer(t)
	defer closeFn()

	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		UDPHandler(&jsonOutput, 3, 0, &throttle, 2, 4, "x", port, "127.0.0.1")
	})

	count := strings.Count(out, "[udp] OK probe open")
	if count != 3 {
		t.Errorf("expected 3 successful probe log lines, got %d in:\n%s", count, out)
	}
	if !strings.Contains(out, "probes_sent=3 open=3") {
		t.Errorf("expected done summary with probes_sent=3 open=3, got:\n%s", out)
	}
}
