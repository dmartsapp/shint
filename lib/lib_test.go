package lib

import (
	"context"
	"net"
	"strconv"
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
