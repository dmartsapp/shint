package lib

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
)

// withFamily sets the address family for a test and puts it back afterwards.
func withFamily(t *testing.T, ipv4Only, ipv6Only bool) {
	t.Helper()
	old := NetworkType
	t.Cleanup(func() { NetworkType = old })
	if err := SetIPFamily(ipv4Only, ipv6Only); err != nil {
		t.Fatal(err)
	}
}

func TestSetIPFamily(t *testing.T) {
	old := NetworkType
	defer func() { NetworkType = old }()
	for _, tc := range []struct {
		v4, v6 bool
		want   string
	}{{false, false, "ip"}, {true, false, "ip4"}, {false, true, "ip6"}} {
		if err := SetIPFamily(tc.v4, tc.v6); err != nil || NetworkType != tc.want {
			t.Errorf("SetIPFamily(%v, %v): NetworkType = %q, err = %v; want %q", tc.v4, tc.v6, NetworkType, err, tc.want)
		}
	}
	if err := SetIPFamily(true, true); err == nil {
		t.Error("-4 together with -6 must be an error")
	}
}

func TestFamilyAllowsAndDialNetwork(t *testing.T) {
	v4, v6, mapped := net.ParseIP("192.0.2.1"), net.ParseIP("2001:db8::1"), net.ParseIP("::ffff:192.0.2.1")
	for _, tc := range []struct {
		v4Only, v6Only   bool
		allows4, allows6 bool
		tcp, udp         string
	}{
		{false, false, true, true, "tcp", "udp"},
		{true, false, true, false, "tcp4", "udp4"},
		{false, true, false, true, "tcp6", "udp6"},
	} {
		withFamily(t, tc.v4Only, tc.v6Only)
		if FamilyAllows(v4) != tc.allows4 || FamilyAllows(v6) != tc.allows6 {
			t.Errorf("%s: FamilyAllows(v4) = %v, (v6) = %v", NetworkType, FamilyAllows(v4), FamilyAllows(v6))
		}
		if FamilyAllows(mapped) != tc.allows4 {
			t.Errorf("%s: an IPv4-mapped address counts as IPv4", NetworkType)
		}
		if DialNetwork("tcp") != tc.tcp || DialNetwork("udp") != tc.udp {
			t.Errorf("%s: DialNetwork = %q, %q; want %q, %q", NetworkType, DialNetwork("tcp"), DialNetwork("udp"), tc.tcp, tc.udp)
		}
		if got := DialNetwork("unix"); got != "unix" {
			t.Errorf("DialNetwork(unix) = %q: only tcp and udp are rewritten", got)
		}
	}
}

func TestHostFamilyConflict(t *testing.T) {
	for _, tc := range []struct {
		v4Only, v6Only bool
		host           string
		conflict       bool
	}{
		{true, false, "::1", true},
		{true, false, "2001:db8::1", true},
		{true, false, "192.0.2.1", false},
		{true, false, "example.com", false}, // a name is resolved for the family, never a conflict
		{false, true, "192.0.2.1", true},
		{false, true, "::ffff:192.0.2.1", true}, // an IPv4-mapped address is IPv4
		{false, true, "::1", false},
		{false, true, "example.com", false},
		{false, false, "::1", false},
		{false, false, "192.0.2.1", false},
	} {
		withFamily(t, tc.v4Only, tc.v6Only)
		err := HostFamilyConflict(tc.host)
		if (err != nil) != tc.conflict {
			t.Errorf("%s with %s: err = %v, want conflict=%v", tc.host, NetworkType, err, tc.conflict)
		}
		if err != nil && !strings.Contains(err.Error(), tc.host) {
			t.Errorf("the message should name the host: %v", err)
		}
	}
}

func TestResolveNameFollowsTheFamily(t *testing.T) {
	ctx := context.Background()
	withFamily(t, true, false)
	if got, err := ResolveName(ctx, "127.0.0.1"); err != nil || len(got) != 1 || got[0] != "127.0.0.1" {
		t.Errorf("ip4, 127.0.0.1: %v, %v", got, err)
	}
	if got, err := ResolveName(ctx, "::1"); err == nil {
		t.Errorf("ip4 must not resolve an IPv6 literal, got %v", got)
	}
	if err := SetIPFamily(false, true); err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveName(ctx, "::1"); err != nil || len(got) != 1 || got[0] != "::1" {
		t.Errorf("ip6, ::1: %v, %v", got, err)
	}
	if got, err := ResolveName(ctx, "127.0.0.1"); err == nil {
		t.Errorf("ip6 must not resolve an IPv4 literal, got %v", got)
	}
}

// The exact error a host with IPv6 disabled produced for `shint telnet google.com 443`.
func ipv6Error(errno syscall.Errno) error {
	return &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("2607:f8b0:400a:80d::200e"), Port: 443}, Err: os.NewSyscallError("socket", errno)}
}

func TestFamilyHint(t *testing.T) {
	err := ipv6Error(syscall.EAFNOSUPPORT)
	// Linux words the errno "...by protocol", macOS "...by protocol family": the
	// shared part is what the report showed.
	if got := err.Error(); !strings.HasPrefix(got, "dial tcp [2607:f8b0:400a:80d::200e]:443: socket: address family not supported by protocol") {
		t.Fatalf("the constructed error is not the reported one: %s", got)
	}
	if h := FamilyHint(err); !strings.Contains(h, "cannot use IPv6") || !strings.Contains(h, "-4") {
		t.Errorf("hint for EAFNOSUPPORT = %q", h)
	}
	if h := FamilyHint(ipv6Error(syscall.ENETUNREACH)); !strings.Contains(h, "no route to IPv6") || !strings.Contains(h, "-4") {
		t.Errorf("hint for ENETUNREACH = %q", h)
	}
	// through the wrapper the HTTP client adds
	wrapped := &url.Error{Op: "Get", URL: "https://google.com", Err: err}
	if FamilyHint(wrapped) == "" {
		t.Error("the hint must survive url.Error wrapping (web)")
	}
	for name, e := range map[string]error{
		"an IPv4 address":         &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 443}, Err: os.NewSyscallError("connect", syscall.ENETUNREACH)},
		"a refusal over IPv6":     ipv6Error(syscall.ECONNREFUSED),
		"a timeout over IPv6":     ipv6Error(syscall.ETIMEDOUT),
		"no address in the error": &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("socket", syscall.EAFNOSUPPORT)},
		"a plain error":           errors.New("something else"),
		"nil":                     nil,
	} {
		if h := FamilyHint(e); h != "" {
			t.Errorf("%s: unexpected hint %q", name, h)
		}
	}
}

func TestExplainError(t *testing.T) {
	err := ipv6Error(syscall.EAFNOSUPPORT)
	got := ExplainError(err)
	if !strings.HasPrefix(got, err.Error()+" (") || !strings.HasSuffix(got, "use -4 to check IPv4 only)") {
		t.Errorf("ExplainError = %q", got)
	}
	plain := errors.New("connection refused")
	if ExplainError(plain) != "connection refused" {
		t.Errorf("an error without a hint must be unchanged, got %q", ExplainError(plain))
	}
}

func TestSchemeHint(t *testing.T) {
	plain := errors.New(`Get "https://127.0.0.1:8080/": http: server gave HTTP response to HTTPS client`)
	if got := SchemeHint(plain); got != "the server speaks plain HTTP: use http:// in the URL" {
		t.Errorf("SchemeHint = %q", got)
	}
	for _, err := range []error{nil, errors.New("connection refused"), errors.New("tls: failed to verify certificate")} {
		if got := SchemeHint(err); got != "" {
			t.Errorf("SchemeHint(%v) = %q, want none", err, got)
		}
	}
	if got := ExplainError(plain); !strings.HasSuffix(got, "HTTPS client (the server speaks plain HTTP: use http:// in the URL)") {
		t.Errorf("ExplainError = %q", got)
	}
}
