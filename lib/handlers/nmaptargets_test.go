package handlers

import (
	"slices"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib"
)

func TestParseNmapTargets(t *testing.T) {
	targets, err := ParseNmapTargets([]string{"web1.example", "127.0.0.1", "::1", "192.168.1.0/24", "192.168.1.77/24", "fd00::/120", "host_with_underscore", "fqdn.example."})
	if err != nil {
		t.Fatal(err)
	}
	prefixes := map[string]string{"192.168.1.0/24": "192.168.1.0/24", "192.168.1.77/24": "192.168.1.0/24", "fd00::/120": "fd00::/120"}
	for _, tg := range targets {
		want, isPrefix := prefixes[tg.Input]
		if tg.IsPrefix() != isPrefix || (isPrefix && tg.Prefix.String() != want) {
			t.Errorf("%s: prefix %v %v, want %v %q", tg.Input, tg.IsPrefix(), tg.Prefix, isPrefix, want)
		}
	}
	// The widest a sweep may be, and two that add up to just that.
	for _, inputs := range [][]string{{"10.1.0.0/16"}, {"fd00::/112"}, {"10.0.0.0/17", "10.0.128.0/17"}} {
		if _, err := ParseNmapTargets(inputs); err != nil {
			t.Errorf("%v: %v", inputs, err)
		}
	}
}

func TestParseNmapTargetsRefuses(t *testing.T) {
	for _, c := range []struct {
		inputs []string
		says   string
	}{
		{[]string{""}, "empty target"},
		{[]string{"  "}, "empty target"},
		{[]string{"10.0.0.0/8"}, "10.0.0.0/8 has 16777214 addresses to scan; one run sweeps at most 65536"},
		{[]string{"fd00::/64"}, "fd00::/64 has 18446744073709551616 addresses"},
		{[]string{"::/0"}, "has 340282366920938463463374607431768211456 addresses"},
		{[]string{"10.0.0.0/16", "10.1.0.0/16"}, "the prefixes given have 131068 addresses to scan"},
		{[]string{"10.0.0.0/33"}, `invalid prefix "10.0.0.0/33"`},
		{[]string{"300.1.1.1/24"}, `invalid prefix "300.1.1.1/24"`},
		{[]string{"web1/24"}, `invalid prefix "web1/24"`},
		{[]string{"a b"}, `invalid target "a b"`},
		{[]string{"-x"}, `invalid target "-x"`},
		{[]string{".example"}, `invalid target ".example"`},
		{[]string{"host;id"}, `invalid target "host;id"`},
		{[]string{"host\x1b[2J"}, "invalid target"},
		{[]string{"h\u00e9te"}, "invalid target"},
		{[]string{strings.Repeat("a", 254)}, "invalid target"},
		{[]string{"127.0.0.1", "ok.example", "bad target"}, `invalid target "bad target"`},
	} {
		_, err := ParseNmapTargets(c.inputs)
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("ParseNmapTargets(%.60q): %v; want an error saying %q", c.inputs, err, c.says)
		}
	}
}

func TestParseNmapTargetsFamily(t *testing.T) {
	defer func() { _ = lib.SetIPFamily(false, false) }()
	_ = lib.SetIPFamily(true, false)
	for input, says := range map[string]string{"fd00::/120": "fd00::/120 is an IPv6 prefix, but -4/--ipv4 was requested", "::1": "-4/--ipv4"} {
		if _, err := ParseNmapTargets([]string{input}); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("-4 %s: %v; want %q", input, err, says)
		}
	}
	if _, err := ParseNmapTargets([]string{"192.168.1.0/30", "web1.example"}); err != nil {
		t.Errorf("-4 with an IPv4 prefix and a name: %v", err)
	}
	_ = lib.SetIPFamily(false, true)
	if _, err := ParseNmapTargets([]string{"192.168.1.0/30"}); err == nil || !strings.Contains(err.Error(), "is an IPv4 prefix, but -6/--ipv6 was requested") {
		t.Errorf("-6 with an IPv4 prefix: %v", err)
	}
}

func TestSweepAddresses(t *testing.T) {
	for _, c := range []struct {
		prefix      string
		count       int
		first, last string
	}{
		{"192.168.1.0/24", 254, "192.168.1.1", "192.168.1.254"},
		{"192.168.1.0/30", 2, "192.168.1.1", "192.168.1.2"},
		{"192.168.1.0/31", 2, "192.168.1.0", "192.168.1.1"}, // RFC 3021: no network or broadcast address
		{"192.168.1.9/32", 1, "192.168.1.9", "192.168.1.9"},
		{"10.0.0.0/16", 65534, "10.0.0.1", "10.0.255.254"},
		{"fd00::/126", 4, "fd00::", "fd00::3"},
		{"fd00::1/128", 1, "fd00::1", "fd00::1"},
		{"fd00::/112", 65536, "fd00::", "fd00::ffff"},
	} {
		targets, err := ParseNmapTargets([]string{c.prefix})
		if err != nil {
			t.Fatal(err)
		}
		got := sweepAddresses(targets[0].Prefix)
		if len(got) != c.count || got[0] != c.first || got[len(got)-1] != c.last || int(sweepSize(targets[0].Prefix)) != c.count {
			t.Errorf("%s: %d addresses %s..%s (size %d), want %d %s..%s", c.prefix, len(got), got[0], got[len(got)-1], sweepSize(targets[0].Prefix), c.count, c.first, c.last)
		}
		if len(slices.Compact(slices.Clone(got))) != len(got) {
			t.Errorf("%s: an address is listed twice", c.prefix)
		}
	}
}

func TestServiceName(t *testing.T) {
	for port, want := range map[int]string{22: "ssh", 53: "dns", 80: "http", 443: "https", 3389: "rdp", 5432: "postgresql", 8500: "consul-http", 2222: "", 3000: "", 65535: ""} {
		if got := serviceName(port); got != want {
			t.Errorf("serviceName(%d) = %q, want %q", port, got, want)
		}
	}
	for port, name := range tcpServices {
		if port < 1 || port > 65535 || name == "" || strings.ToLower(name) != name || strings.ContainsAny(name, " _=\"") {
			t.Errorf("tcpServices[%d] = %q: want a port 1-65535 and a lower-case name of letters, digits and hyphens", port, name)
		}
	}
}
