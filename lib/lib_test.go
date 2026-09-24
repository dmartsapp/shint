package lib

import (
	"context"
	"math"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidatePort(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{"valid low", "1", 1, false},
		{"valid high", "65535", 65535, false},
		{"valid typical", "8080", 8080, false},
		{"zero", "0", 0, true},
		{"negative", "-1", 0, true},
		{"too large", "65536", 0, true},
		{"not a number", "abc", 0, true},
		{"empty", "", 0, true},
		{"float", "80.5", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ValidatePort(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ValidatePort(%q) = %d, nil; want error", c.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePort(%q) unexpected error: %v", c.raw, err)
			}
			if got != c.want {
				t.Errorf("ValidatePort(%q) = %d, want %d", c.raw, got, c.want)
			}
		})
	}
}

func TestRequirePositive(t *testing.T) {
	if err := RequirePositive("count", 1); err != nil {
		t.Errorf("RequirePositive(1) unexpected error: %v", err)
	}
	if err := RequirePositive("count", 100); err != nil {
		t.Errorf("RequirePositive(100) unexpected error: %v", err)
	}
	if err := RequirePositive("count", 0); err == nil {
		t.Error("RequirePositive(0) expected error, got nil")
	}
	if err := RequirePositive("count", -5); err == nil {
		t.Error("RequirePositive(-5) expected error, got nil")
	}
}

func TestGetMinAvgMax(t *testing.T) {
	stats := []time.Duration{
		10 * time.Millisecond,
		30 * time.Millisecond,
		20 * time.Millisecond,
	}
	min, avg, max := GetMinAvgMax(stats)
	if min != 10*time.Millisecond {
		t.Errorf("min = %v, want 10ms", min)
	}
	if max != 30*time.Millisecond {
		t.Errorf("max = %v, want 30ms", max)
	}
	if avg != 20*time.Millisecond {
		t.Errorf("avg = %v, want 20ms", avg)
	}
}

func TestGetMinAvgMaxSingleValue(t *testing.T) {
	stats := []time.Duration{5 * time.Millisecond}
	min, avg, max := GetMinAvgMax(stats)
	if min != max || min != avg || min != 5*time.Millisecond {
		t.Errorf("single-value stats mismatch: min=%v avg=%v max=%v", min, avg, max)
	}
}

func TestSortTimeDurationSlice(t *testing.T) {
	stats := []time.Duration{30 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond}
	SortTimeDurationSlice(&stats)
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	for i := range stats {
		if stats[i] != want[i] {
			t.Errorf("SortTimeDurationSlice()[%d] = %v, want %v", i, stats[i], want[i])
		}
	}
}

func TestConvertIPToStringSlice(t *testing.T) {
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("10.0.0.1")}
	got := ConvertIPToStringSlice(ips)
	want := []string{"127.0.0.1", "10.0.0.1"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestConvertIPToStringSliceEmpty(t *testing.T) {
	got := ConvertIPToStringSlice(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestResolveNameLoopback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := ResolveName(ctx, "localhost")
	if err != nil {
		t.Fatalf("ResolveName(localhost) unexpected error: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("ResolveName(localhost) returned no addresses")
	}
}

func TestResolveNameResolvesIPv6Literal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := ResolveName(ctx, "::1")
	if err != nil {
		t.Fatalf("ResolveName(::1) unexpected error: %v", err)
	}
	if len(addrs) != 1 || addrs[0] != "::1" {
		t.Errorf("ResolveName(::1) = %v, want [::1]", addrs)
	}
}

func TestResolveNameDualStackNetworkType(t *testing.T) {
	// NetworkType controls telnet/nmap/udp/web's DNS resolution family;
	// must be "ip" (both A and AAAA) rather than the old IPv4-only
	// default, or a dual-stack host would only ever be checked over v4.
	if NetworkType != "ip" {
		t.Errorf("NetworkType = %q, want \"ip\" for dual-stack resolution", NetworkType)
	}
}

func TestResolveNameInvalidHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := ResolveName(ctx, "this-host-should-not-exist.invalid")
	if err == nil {
		t.Fatal("ResolveName(invalid host) expected error, got nil")
	}
}

func TestResolveNameToIPs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := ResolveNameToIPs(ctx, "localhost")
	if err != nil {
		t.Fatalf("ResolveNameToIPs(localhost) unexpected error: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("ResolveNameToIPs(localhost) returned no addresses")
	}
}

func TestResolveNameLogsVerboseStartAndReturn(t *testing.T) {
	buf := withVerbose(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := ResolveName(ctx, "localhost"); err != nil {
		t.Fatalf("ResolveName(localhost) unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "[resolve] VERBOSE starting host=localhost") {
		t.Errorf("ResolveName verbose output = %q, missing the starting line", got)
	}
	if !strings.Contains(got, "[resolve] VERBOSE returned host=localhost") {
		t.Errorf("ResolveName verbose output = %q, missing the returned line", got)
	}
}

func TestResolveNameLogsVerboseFailure(t *testing.T) {
	buf := withVerbose(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := ResolveName(ctx, "this-host-should-not-exist.invalid"); err == nil {
		t.Fatal("ResolveName(invalid host) expected error, got nil")
	}
	if got := buf.String(); !strings.Contains(got, "[resolve] VERBOSE failed host=this-host-should-not-exist.invalid") {
		t.Errorf("ResolveName verbose output = %q, missing the failed line", got)
	}
}

func TestResolveNameSilentWithoutVerbose(t *testing.T) {
	buf := withVerbose(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := ResolveName(ctx, "localhost"); err != nil {
		t.Fatalf("ResolveName(localhost) unexpected error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("ResolveName wrote verbose output %q while Verbose is false, want nothing", got)
	}
}

func TestResolveNameToIPsLogsVerbose(t *testing.T) {
	buf := withVerbose(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := ResolveNameToIPs(ctx, "localhost"); err != nil {
		t.Fatalf("ResolveNameToIPs(localhost) unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "[resolve] VERBOSE starting host=localhost") || !strings.Contains(got, "[resolve] VERBOSE returned host=localhost") {
		t.Errorf("ResolveNameToIPs verbose output = %q, missing the expected lines", got)
	}
}

func TestIsPortUpOpenPortIPv6(t *testing.T) {
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	portNum, _ := strconv.Atoi(port)

	up, err := IsPortUp(context.Background(), "::1", portNum, 2)
	if err != nil {
		t.Fatalf("IsPortUp unexpected error: %v", err)
	}
	if !up {
		t.Error("IsPortUp() = false, want true for an open IPv6 port")
	}
}

func TestIsPortUpOpenPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	portNum, _ := strconv.Atoi(port)

	up, err := IsPortUp(context.Background(), "127.0.0.1", portNum, 2)
	if err != nil {
		t.Fatalf("IsPortUp unexpected error: %v", err)
	}
	if !up {
		t.Error("IsPortUp() = false, want true for an open port")
	}
}

func TestIsPortUpClosedPort(t *testing.T) {
	// Find a free port, then close the listener so it's guaranteed closed.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	portNum, _ := strconv.Atoi(port)
	_ = listener.Close()

	up, err := IsPortUp(context.Background(), "127.0.0.1", portNum, 2)
	if err == nil {
		t.Error("IsPortUp() expected error for a closed port, got nil")
	}
	if up {
		t.Error("IsPortUp() = true, want false for a closed port")
	}
}

func TestIsPortUpRespectsContextCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	portNum, _ := strconv.Atoi(port)
	_ = listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled before dialing

	start := time.Now()
	_, err = IsPortUp(ctx, "127.0.0.1", portNum, 30) // long per-dial timeout
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error when dialing with an already-canceled context")
	}
	if elapsed > 2*time.Second {
		t.Errorf("IsPortUp took %v with a pre-canceled context; expected it to fail almost immediately instead of waiting out the 30s dial timeout", elapsed)
	}
}

func TestRequireRanges(t *testing.T) {
	const day, dayMs = 86400, 86400000
	// As large/small as int can safely hold on every platform shint builds for - 32-bit
	// architectures (linux/arm) included, where a literal like 9223372036854775807 does not
	// even compile. Plenty large enough to still be "clearly invalid" for what is tested here.
	const hugePositive, hugeNegative = math.MaxInt32, math.MinInt32
	if MaxTimeoutSeconds != day || MaxDelayMilliseconds != dayMs {
		t.Fatalf("the limits are a day: %d s, %d ms", MaxTimeoutSeconds, MaxDelayMilliseconds)
	}
	ok := func(err error) bool { return err == nil }
	cases := []struct {
		name string
		fn   func(int) error
		good []int
		bad  []int
	}{
		{"RequireTimeout", RequireTimeout, []int{1, 5, 3600, day}, []int{0, -1, day + 1, hugePositive, hugeNegative}},
		{"RequireIdleTimeout", RequireIdleTimeout, []int{0, 1, day}, []int{-1, day + 1, hugePositive}},
		{"RequireDelay", RequireDelay, []int{0, 1, 1000, dayMs}, []int{-1, dayMs + 1, hugePositive, hugeNegative}},
		{"RequireNonNegative", func(v int) error { return RequireNonNegative("count", v) }, []int{0, 1, hugePositive}, []int{-1, hugeNegative}},
	}
	for _, c := range cases {
		for _, v := range c.good {
			if err := c.fn(v); !ok(err) {
				t.Errorf("%s(%d) = %v, want nil", c.name, v, err)
			}
		}
		for _, v := range c.bad {
			if err := c.fn(v); err == nil {
				t.Errorf("%s(%d) = nil, want an error", c.name, v)
			}
		}
	}
	// what the user reads
	if got := RequireTimeout(hugePositive).Error(); got != "--timeout must be between 1 and 86400 seconds (a day), got 2147483647" {
		t.Errorf("message: %q", got)
	}
	if got := RequireDelay(-1).Error(); got != "--delay must be between 0 and 86400000 milliseconds (a day), got -1" {
		t.Errorf("message: %q", got)
	}
	// nothing accepted may overflow the duration it becomes
	if d := time.Duration(MaxTimeoutSeconds) * time.Second; d <= 0 {
		t.Errorf("the largest timeout overflows: %v", d)
	}
	if d := time.Duration(MaxDelayMilliseconds) * time.Millisecond; d <= 0 {
		t.Errorf("the largest delay overflows: %v", d)
	}
}

func TestInFlightLimit(t *testing.T) {
	cases := []struct {
		name  string
		max   int
		limit uint64
		known bool
		want  int
	}{
		{"unknown limit: the cap", 500, 0, false, 500},
		{"a generous limit: the cap", 500, 1048576, true, 500},
		{"unlimited", 500, 1<<63 - 1, true, 500},
		{"1024 (the usual Linux default)", 500, 1024, true, 500},
		{"512: exactly half is the cap", 500, 1000, true, 500},
		{"256 (an old default, or ulimit -n 256): half", 500, 256, true, 128},
		{"64", 500, 64, true, 32},
		{"8: half of it is 4", 500, 8, true, 4},
		{"below the floor: still makes progress", 500, 3, true, 4},
		{"zero: still makes progress", 500, 0, true, 4},
		{"a lower cap than the limit allows", 100, 4096, true, 100},
	}
	for _, c := range cases {
		if got := inFlightLimit(c.max, c.limit, c.known); got != c.want {
			t.Errorf("%s: inFlightLimit(%d, %d, %v) = %d, want %d", c.name, c.max, c.limit, c.known, got, c.want)
		}
	}
	// the real process: a positive bound no larger than the cap
	if got := InFlightLimit(500); got < 4 || got > 500 {
		t.Errorf("InFlightLimit(500) = %d", got)
	}
}
