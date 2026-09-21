package handlers

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
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
	return conn.LocalAddr().(*net.UDPAddr).Port, func() { _ = conn.Close() }
}

// startEchoUDPServerIPv6 is startEchoUDPServer's IPv6 loopback counterpart.
func startEchoUDPServerIPv6(t *testing.T) (port int, closeFn func()) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("::1")})
	if err != nil {
		t.Fatalf("failed to start IPv6 UDP echo server: %v", err)
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
	return conn.LocalAddr().(*net.UDPAddr).Port, func() { _ = conn.Close() }
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
	return conn.LocalAddr().(*net.UDPAddr).Port, func() { _ = conn.Close() }
}

func TestProbeUDPOpen(t *testing.T) {
	port, closeFn := startEchoUDPServer(t)
	defer closeFn()

	state, received, err := probeUDP(context.Background(), "127.0.0.1", port, 2, []byte("ping"))
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

func TestProbeUDPOpenIPv6(t *testing.T) {
	port, closeFn := startEchoUDPServerIPv6(t)
	defer closeFn()

	state, received, err := probeUDP(context.Background(), "::1", port, 2, []byte("ping6"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != "open" {
		t.Errorf("state = %q, want %q", state, "open")
	}
	if string(received) != "ping6" {
		t.Errorf("received = %q, want %q", received, "ping6")
	}
}

func TestProbeUDPOpenFiltered(t *testing.T) {
	port, closeFn := startSilentUDPServer(t)
	defer closeFn()

	state, received, err := probeUDP(context.Background(), "127.0.0.1", port, 1, []byte("ping"))
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
	_ = conn.Close()

	state, _, err := probeUDP(context.Background(), "127.0.0.1", port, 2, []byte("ping"))
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
		UDPHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 2, 4, "hello", port, "127.0.0.1")
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
		UDPHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 1, 4, "", 53, "this-host-should-not-exist.invalid")
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
		UDPHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 2, 6, "", port, "127.0.0.1")
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
		UDPHandler(context.Background(), &jsonOutput, 3, 0, &throttle, 2, 4, "x", port, "127.0.0.1")
	})

	count := strings.Count(out, "[udp] OK probe open")
	if count != 3 {
		t.Errorf("expected 3 successful probe log lines, got %d in:\n%s", count, out)
	}
	if !strings.Contains(out, "probes_sent=3 open=3") {
		t.Errorf("expected done summary with probes_sent=3 open=3, got:\n%s", out)
	}
}

// A closed port makes the run exit 1, so its line is an ERROR line - it used to be
// logged as OK while the exit status said 1 - and open and open|filtered, which do
// not fail the run, are OK lines.
func TestUDPHandlerLogLevelFollowsWhetherTheProbeFailed(t *testing.T) {
	jsonOutput, throttle := false, false
	run := func(port int) (bool, string) {
		var ok bool
		out := captureStdout(t, func() {
			ok = UDPHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 1, 4, "x", port, "127.0.0.1")
		})
		return ok, out
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	closedPort := conn.LocalAddr().(*net.UDPAddr).Port
	_ = conn.Close()
	ok, out := run(closedPort)
	if ok || !strings.Contains(out, "[udp] ERROR probe closed ") || strings.Contains(out, "OK probe closed") {
		t.Errorf("closed: ok=%v, want an ERROR line:\n%s", ok, out)
	}

	open, closeOpen := startEchoUDPServer(t)
	defer closeOpen()
	if ok, out := run(open); !ok || !strings.Contains(out, "[udp] OK probe open ") || strings.Contains(out, "] ERROR ") {
		t.Errorf("open: ok=%v:\n%s", ok, out)
	}

	silent, closeSilent := startSilentUDPServer(t)
	defer closeSilent()
	if ok, out := run(silent); !ok || !strings.Contains(out, "[udp] OK probe open|filtered ") || strings.Contains(out, "] ERROR ") {
		t.Errorf("open|filtered is inconclusive, not a failure: ok=%v:\n%s", ok, out)
	}
}

// A negative --payload used to crash the handler (strings.Repeat panics on a
// negative count) and a huge one asked it for memory it could not have.
func TestValidateUDPPayload(t *testing.T) {
	for _, size := range []int{0, 1, 4, 1472, 8000, MaxUDPPayload} {
		if err := ValidateUDPPayload(size, ""); err != nil {
			t.Errorf("--payload %d: %v", size, err)
		}
	}
	for _, size := range []int{-1, MaxUDPPayload + 1, 1000000000, 9223372036854775807, -9223372036854775808} {
		err := ValidateUDPPayload(size, "")
		if err == nil || !strings.Contains(err.Error(), "--payload for udp must be between 0 and 65507 bytes") {
			t.Errorf("--payload %d: error = %v", size, err)
		}
	}
	// --payload is ignored when --data is given, so a bad size does not matter then
	if err := ValidateUDPPayload(-1, "hello"); err != nil {
		t.Errorf("--data given, --payload ignored: %v", err)
	}
	if err := ValidateUDPPayload(4, strings.Repeat("x", MaxUDPPayload)); err != nil {
		t.Errorf("--data of the largest size: %v", err)
	}
	if err := ValidateUDPPayload(4, strings.Repeat("x", MaxUDPPayload+1)); err == nil || !strings.Contains(err.Error(), "--data is 65508 bytes") {
		t.Errorf("--data one byte too large: %v", err)
	}
}

func TestParseHexPayload(t *testing.T) {
	five := []byte{0x00, 0x01, 0x02, 0x03, 0xff}
	good := map[string][]byte{
		"00010203ff":            five,
		"00010203FF":            five,
		"00 01 02 03 ff":        five,
		"00:01:02:03:FF":        five,
		"  00:01 02:03 ff  ":    five,
		"0001 0203 ff":          five, // separators fall wherever they like
		"0 0 0 1 0 2 0 3 f f":   five, // ... even inside a byte: they are ignored, not parsed
		"aB":                    {0xab},
		"00":                    {0x00},
		"deadbeef":              {0xde, 0xad, 0xbe, 0xef},
		"123401000001000000000": nil, // odd - filled in below
	}
	delete(good, "123401000001000000000")
	for in, want := range good {
		got, err := ParseHexPayload(in)
		if err != nil || string(got) != string(want) {
			t.Errorf("ParseHexPayload(%q) = %v, %v; want %v", in, got, err, want)
		}
	}

	bad := map[string]string{
		"":         "at least one byte",
		"   ":      "at least one byte",
		":: ::":    "at least one byte",
		"0":        "1 hex digits, an odd number",
		"abc":      "3 hex digits, an odd number",
		"00 0":     "3 hex digits, an odd number",
		"0x00":     `'x' is not a hex digit`,
		"zz":       `'z' is not a hex digit`,
		"00-01":    `'-' is not a hex digit`,
		"00,01":    `',' is not a hex digit`,
		"0g":       `'g' is not a hex digit`,
		"é0":       `'é' is not a hex digit`,
		"00\n01":   `'\n' is not a hex digit`,
		"\\x00":    `'\\' is not a hex digit`,
		"00\x0001": `'\x00' is not a hex digit`,
	}
	for in, want := range bad {
		got, err := ParseHexPayload(in)
		if err == nil || !strings.Contains(err.Error(), want) || got != nil {
			t.Errorf("ParseHexPayload(%q) = %v, %v; want an error containing %q", in, got, err, want)
		}
		if err != nil && !strings.HasPrefix(err.Error(), "--hex") {
			t.Errorf("ParseHexPayload(%q): the error should name the flag: %v", in, err)
		}
	}

	// the largest datagram is fine, one byte more is not
	if got, err := ParseHexPayload(strings.Repeat("ab", MaxUDPPayload)); err != nil || len(got) != MaxUDPPayload {
		t.Errorf("the largest payload: %d bytes, %v", len(got), err)
	}
	if _, err := ParseHexPayload(strings.Repeat("ab", MaxUDPPayload+1)); err == nil || !strings.Contains(err.Error(), "--hex is 65508 bytes") {
		t.Errorf("one byte too many: %v", err)
	}
}

// The bytes --hex decodes reach the wire exactly - a zero byte and bytes that are not
// valid text included - and the reply is previewed escaped.
func TestUDPHandlerSendsBinaryPayloadExactly(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 2048)
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		got <- append([]byte(nil), buf[:n]...)
		_, _ = conn.WriteToUDP(buf[:n], from)
	}()
	port := conn.LocalAddr().(*net.UDPAddr).Port

	want, err := ParseHexPayload("00 01 02 ff fe 80 0a 41")
	if err != nil {
		t.Fatal(err)
	}
	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		UDPHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 2, 4, string(want), port, "127.0.0.1")
	})
	select {
	case b := <-got:
		if string(b) != string(want) {
			t.Errorf("the server received % x, want % x", b, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the server received nothing")
	}
	var result lib.JSONOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	statsJSON, _ := json.Marshal(result.Stats)
	var stats []lib.UDPStats
	if err := json.Unmarshal(statsJSON, &stats); err != nil || len(stats) != 1 {
		t.Fatalf("stats: %v %s", err, statsJSON)
	}
	if stats[0].BytesSent != len(want) || stats[0].BytesReceived != len(want) {
		t.Errorf("bytes sent/received = %d/%d, want %d each", stats[0].BytesSent, stats[0].BytesReceived, len(want))
	}
	if stats[0].ResponsePreview != `\x00\x01\x02\xff\xfe\x80\nA` {
		t.Errorf("response_preview = %q", stats[0].ResponsePreview)
	}
}
