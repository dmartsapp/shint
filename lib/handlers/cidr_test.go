package handlers

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/lib"
)

func mustSubnet(t *testing.T, input string) lib.CIDRStats {
	t.Helper()
	subnets, err := ParseSubnets([]string{input})
	if err != nil {
		t.Fatalf("ParseSubnets(%q): %v", input, err)
	}
	return subnetStats(subnets[0])
}

// The expected values are worked out by hand from the definitions (RFC 4632
// for IPv4, RFC 3021 for /31, RFC 4291 for IPv6), not from the code.
func TestSubnetStats(t *testing.T) {
	tests := []struct {
		input string
		want  lib.CIDRStats
	}{
		{"192.168.1.10/24", lib.CIDRStats{
			Network: "192.168.1.0/24", Family: "ipv4", PrefixLength: 24, Netmask: "255.255.255.0", Wildcard: "0.0.0.255",
			FirstAddress: "192.168.1.0", LastAddress: "192.168.1.255", Broadcast: "192.168.1.255",
			Addresses: "256", UsableHosts: "254", FirstHost: "192.168.1.1", LastHost: "192.168.1.254", Kind: "private"}},
		{"10.0.0.0/8", lib.CIDRStats{
			Network: "10.0.0.0/8", Family: "ipv4", PrefixLength: 8, Netmask: "255.0.0.0", Wildcard: "0.255.255.255",
			FirstAddress: "10.0.0.0", LastAddress: "10.255.255.255", Broadcast: "10.255.255.255",
			Addresses: "16777216", UsableHosts: "16777214", FirstHost: "10.0.0.1", LastHost: "10.255.255.254", Kind: "private"}},
		{"172.16.5.130/26", lib.CIDRStats{ // 130 falls in the third /26 of .5: .128 - .191
			Network: "172.16.5.128/26", Family: "ipv4", PrefixLength: 26, Netmask: "255.255.255.192", Wildcard: "0.0.0.63",
			FirstAddress: "172.16.5.128", LastAddress: "172.16.5.191", Broadcast: "172.16.5.191",
			Addresses: "64", UsableHosts: "62", FirstHost: "172.16.5.129", LastHost: "172.16.5.190", Kind: "private"}},
		{"203.0.113.6/30", lib.CIDRStats{
			Network: "203.0.113.4/30", Family: "ipv4", PrefixLength: 30, Netmask: "255.255.255.252", Wildcard: "0.0.0.3",
			FirstAddress: "203.0.113.4", LastAddress: "203.0.113.7", Broadcast: "203.0.113.7",
			Addresses: "4", UsableHosts: "2", FirstHost: "203.0.113.5", LastHost: "203.0.113.6", Kind: "documentation"}},
		{"192.0.2.0/31", lib.CIDRStats{ // RFC 3021: both addresses are hosts, there is no broadcast
			Network: "192.0.2.0/31", Family: "ipv4", PrefixLength: 31, Netmask: "255.255.255.254", Wildcard: "0.0.0.1",
			FirstAddress: "192.0.2.0", LastAddress: "192.0.2.1",
			Addresses: "2", UsableHosts: "2", FirstHost: "192.0.2.0", LastHost: "192.0.2.1", Kind: "documentation"}},
		{"8.8.8.8", lib.CIDRStats{ // a bare address is a /32
			Network: "8.8.8.8/32", Family: "ipv4", PrefixLength: 32, Netmask: "255.255.255.255", Wildcard: "0.0.0.0",
			FirstAddress: "8.8.8.8", LastAddress: "8.8.8.8",
			Addresses: "1", UsableHosts: "1", FirstHost: "8.8.8.8", LastHost: "8.8.8.8", Kind: "global"}},
		{"0.0.0.0/0", lib.CIDRStats{
			Network: "0.0.0.0/0", Family: "ipv4", PrefixLength: 0, Netmask: "0.0.0.0", Wildcard: "255.255.255.255",
			FirstAddress: "0.0.0.0", LastAddress: "255.255.255.255", Broadcast: "255.255.255.255",
			Addresses: "4294967296", UsableHosts: "4294967294", FirstHost: "0.0.0.1", LastHost: "255.255.255.254", Kind: "unspecified"}},
		{"2001:db8::/32", lib.CIDRStats{ // 2^96 addresses; there is no wildcard or broadcast
			Network: "2001:db8::/32", Family: "ipv6", PrefixLength: 32, Netmask: "ffff:ffff::",
			FirstAddress: "2001:db8::", LastAddress: "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff",
			Addresses: "79228162514264337593543950336", UsableHosts: "79228162514264337593543950336",
			FirstHost: "2001:db8::", LastHost: "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff", Kind: "documentation"}},
		{"fe80::1/64", lib.CIDRStats{
			Network: "fe80::/64", Family: "ipv6", PrefixLength: 64, Netmask: "ffff:ffff:ffff:ffff::",
			FirstAddress: "fe80::", LastAddress: "fe80::ffff:ffff:ffff:ffff",
			Addresses: "18446744073709551616", UsableHosts: "18446744073709551616",
			FirstHost: "fe80::", LastHost: "fe80::ffff:ffff:ffff:ffff", Kind: "link-local"}},
		{"::1", lib.CIDRStats{
			Network: "::1/128", Family: "ipv6", PrefixLength: 128, Netmask: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			FirstAddress: "::1", LastAddress: "::1", Addresses: "1", UsableHosts: "1", FirstHost: "::1", LastHost: "::1", Kind: "loopback"}},
		{"::/0", lib.CIDRStats{
			Network: "::/0", Family: "ipv6", PrefixLength: 0, Netmask: "::",
			FirstAddress: "::", LastAddress: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			Addresses: "340282366920938463463374607431768211456", UsableHosts: "340282366920938463463374607431768211456",
			FirstHost: "::", LastHost: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", Kind: "unspecified"}},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			tc.want.Input = tc.input
			if got := mustSubnet(t, tc.input); got != tc.want {
				t.Errorf("\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestAddressKinds(t *testing.T) {
	for input, want := range map[string]string{
		"127.0.0.1": "loopback", "10.1.2.3": "private", "172.31.0.1": "private", "192.168.0.1": "private",
		"169.254.1.1": "link-local", "224.0.0.1": "multicast", "100.64.0.1": "shared", "240.0.0.1": "reserved",
		"255.255.255.255": "broadcast", "1.1.1.1": "global", "fc00::1": "private", "fd12:3456::1": "private",
		"ff02::1": "multicast", "2606:4700::1111": "global", "0.0.0.0": "unspecified",
	} {
		if got := mustSubnet(t, input).Kind; got != want {
			t.Errorf("kind of %s = %q, want %q", input, got, want)
		}
	}
}

// The standard library's ParseCIDR is an independent check of the network
// address and mask for every prefix length of both families.
func TestSubnetStatsAgreeWithNetParseCIDR(t *testing.T) {
	for _, addr := range []string{"203.0.113.77", "2001:db8:abcd:1234:5678:9abc:def0:1234"} {
		size := 32
		if strings.Contains(addr, ":") {
			size = 128
		}
		for ones := 0; ones <= size; ones++ {
			input := addr + "/" + itoa(ones)
			_, want, err := net.ParseCIDR(input)
			if err != nil {
				t.Fatal(err)
			}
			got := mustSubnet(t, input)
			if got.Network != want.String() {
				t.Errorf("%s: network %s, net.ParseCIDR says %s", input, got.Network, want)
			}
			if wantMask := net.IP(want.Mask).String(); size == 32 && got.Netmask != wantMask {
				t.Errorf("%s: netmask %s, net.ParseCIDR says %s", input, got.Netmask, wantMask)
			}
		}
	}
}

func itoa(n int) string {
	digits := "0123456789"
	if n < 10 {
		return digits[n : n+1]
	}
	return itoa(n/10) + digits[n%10:n%10+1]
}

func TestParseSubnetsRejectsBadInput(t *testing.T) {
	for _, bad := range []string{"", "  ", "abc", "300.1.1.1/24", "10.0.0.0/33", "10.0.0.0/-1", "10.0.0.0/24/8", "10.0.0/24", "10.0.0.0/", "/24", "::1/129", "fe80::1%eth0/64", "1.2.3.4/x"} {
		if _, err := ParseSubnets([]string{bad}); err == nil {
			t.Errorf("ParseSubnets(%q) accepted it", bad)
		}
	}
	// one bad prefix among good ones rejects the whole run, before anything is printed
	if _, err := ParseSubnets([]string{"10.0.0.0/8", "nope", "192.168.0.0/16"}); err == nil {
		t.Error("a bad prefix among good ones was accepted")
	}
}

func TestCIDRHandlerText(t *testing.T) {
	subnets, err := ParseSubnets([]string{"192.168.1.10/24", "2001:db8::/32"})
	if err != nil {
		t.Fatal(err)
	}
	jsonOutput := false
	var ok bool
	out := captureStdout(t, func() { ok = CIDRHandler(&jsonOutput, subnets) })
	if !ok {
		t.Error("CIDRHandler reported failure")
	}
	for _, want := range []string{
		"[cidr] OK subnet input=192.168.1.10/24 network=192.168.1.0/24 family=ipv4 netmask=255.255.255.0 wildcard=0.0.0.255 first=192.168.1.0 last=192.168.1.255 broadcast=192.168.1.255 addresses=256 hosts=254 first_host=192.168.1.1 last_host=192.168.1.254 kind=private",
		"[cidr] OK subnet input=2001:db8::/32 network=2001:db8::/32 family=ipv6 netmask=ffff:ffff:: first=2001:db8:: last=2001:db8:ffff:ffff:ffff:ffff:ffff:ffff addresses=79228162514264337593543950336",
		"[cidr] OK done prefixes=2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "wildcard=") && strings.Count(out, "wildcard=") != 1 {
		t.Errorf("IPv6 must not print a wildcard:\n%s", out)
	}
}

func TestCIDRHandlerJSONIsOneCleanDocument(t *testing.T) {
	subnets, _ := ParseSubnets([]string{"10.0.0.0/8", "::1"})
	jsonOutput := true
	out := captureStdout(t, func() { CIDRHandler(&jsonOutput, subnets) })
	var doc struct {
		ModuleName string          `json:"module_name"`
		Stats      []lib.CIDRStats `json:"stats"`
		Error      string          `json:"error"`
		Extra      map[string]any  `json:"dns_lookup"`
		Input      map[string]any  `json:"input_params"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, out)
	}
	if doc.ModuleName != "cidr" || len(doc.Stats) != 2 || doc.Stats[0].Network != "10.0.0.0/8" || doc.Stats[1].Network != "::1/128" {
		t.Errorf("unexpected document: %+v", doc)
	}
	if doc.Extra != nil {
		t.Error("a command with no name lookup must not carry dns_lookup")
	}
	if doc.Input["module_name"] != "cidr" || doc.Input["data"] != "10.0.0.0/8,::1" {
		t.Errorf("input_params = %v", doc.Input)
	}
	// big counts stay exact: they are strings, not numbers
	if !strings.Contains(out, `"addresses": "16777216"`) {
		t.Errorf("addresses should be a decimal string:\n%s", out)
	}
}
