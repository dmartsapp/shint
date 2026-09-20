package handlers

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/netip"
	"strings"
	"time"

	"github.com/dmartsapp/shint/lib"
)

const cidrModule = "cidr"

// Subnet is one prefix given to "cidr": what the user typed, and what it
// parsed to (a host address keeps its host bits until the network is worked out).
type Subnet struct {
	Input  string
	Prefix netip.Prefix
}

// ParseSubnets parses every argument before anything is printed, so a typo in
// the third prefix is a usage error rather than two results and then a
// failure. A bare address is a host route: /32 for IPv4, /128 for IPv6.
func ParseSubnets(inputs []string) ([]Subnet, error) {
	subnets := make([]Subnet, 0, len(inputs))
	for _, input := range inputs {
		text := strings.TrimSpace(input)
		if text == "" {
			return nil, fmt.Errorf("empty prefix")
		}
		var prefix netip.Prefix
		if strings.Contains(text, "/") {
			p, err := netip.ParsePrefix(text)
			if err != nil {
				return nil, fmt.Errorf("invalid prefix %q: %v", input, err)
			}
			prefix = p
		} else {
			a, err := netip.ParseAddr(text)
			if err != nil {
				return nil, fmt.Errorf("invalid address or prefix %q", input)
			}
			prefix = netip.PrefixFrom(a, a.BitLen())
		}
		if prefix.Addr().Zone() != "" {
			return nil, fmt.Errorf("invalid prefix %q: zone identifiers are not supported", input)
		}
		if !prefix.IsValid() {
			return nil, fmt.Errorf("invalid prefix %q", input)
		}
		subnets = append(subnets, Subnet{Input: input, Prefix: prefix})
	}
	return subnets, nil
}

// kindRanges are the special ranges "cidr" names, checked after the ones
// net/netip knows about. A prefix is classified by its network address alone,
// so a prefix that spans several kinds (0.0.0.0/0) reports the first one.
var kindRanges = []struct {
	prefix netip.Prefix
	kind   string
}{
	{netip.MustParsePrefix("192.0.2.0/24"), "documentation"},
	{netip.MustParsePrefix("198.51.100.0/24"), "documentation"},
	{netip.MustParsePrefix("203.0.113.0/24"), "documentation"},
	{netip.MustParsePrefix("2001:db8::/32"), "documentation"},
	{netip.MustParsePrefix("100.64.0.0/10"), "shared"},
	{netip.MustParsePrefix("240.0.0.0/4"), "reserved"},
}

// addressKind names what an address is for: unspecified, loopback, link-local,
// multicast, private (RFC 1918 and IPv6 unique-local), documentation, shared
// (carrier-grade NAT), broadcast, reserved, or global - none of those, which
// is not a promise that it is routable on the internet.
func addressKind(a netip.Addr) string {
	switch {
	case a.IsUnspecified():
		return "unspecified"
	case a.IsLoopback():
		return "loopback"
	case a.IsLinkLocalUnicast():
		return "link-local"
	case a.IsMulticast():
		return "multicast"
	case a.IsPrivate():
		return "private"
	case a == netip.AddrFrom4([4]byte{255, 255, 255, 255}):
		return "broadcast"
	}
	for _, r := range kindRanges {
		if r.prefix.Contains(a) {
			return r.kind
		}
	}
	if a.IsGlobalUnicast() {
		return "global"
	}
	return "reserved"
}

// addrFromBits builds an address of the given size (32 or 128 bits) whose
// leading ones bits are set - the netmask - or, with invert, whose trailing
// bits are set (the wildcard).
func addrFromBits(size, ones int, invert bool) netip.Addr {
	b := make([]byte, size/8)
	for i := 0; i < size; i++ {
		set := i < ones
		if invert {
			set = !set
		}
		if set {
			b[i/8] |= 0x80 >> (i % 8)
		}
	}
	a, _ := netip.AddrFromSlice(b)
	return a
}

// subnetStats works out everything "cidr" reports about one prefix.
func subnetStats(s Subnet) lib.CIDRStats {
	network := s.Prefix.Masked()
	first := network.Addr()
	size := first.BitLen()
	ones := s.Prefix.Bits()

	// The last address is the network address with every host bit set.
	raw := first.AsSlice()
	for i := ones; i < size; i++ {
		raw[i/8] |= 0x80 >> (i % 8)
	}
	last, _ := netip.AddrFromSlice(raw)

	total := new(big.Int).Lsh(big.NewInt(1), uint(size-ones))
	stat := lib.CIDRStats{
		Input:        s.Input,
		Network:      network.String(),
		PrefixLength: ones,
		Netmask:      addrFromBits(size, ones, false).String(),
		FirstAddress: first.String(),
		LastAddress:  last.String(),
		Addresses:    total.String(),
		Kind:         addressKind(first),
	}
	if first.Is4() {
		stat.Family = "ipv4"
		stat.Wildcard = addrFromBits(size, ones, true).String()
		switch ones {
		case 32: // a single host
			stat.UsableHosts, stat.FirstHost, stat.LastHost = "1", first.String(), first.String()
		case 31: // a point-to-point link: both addresses are hosts (RFC 3021)
			stat.UsableHosts, stat.FirstHost, stat.LastHost = "2", first.String(), last.String()
		default: // the first address is the network, the last the broadcast
			stat.UsableHosts = new(big.Int).Sub(total, big.NewInt(2)).String()
			stat.FirstHost, stat.LastHost = first.Next().String(), last.Prev().String()
			stat.Broadcast = last.String()
		}
	} else {
		stat.Family = "ipv6" // no broadcast: every address in the prefix can be assigned
		stat.UsableHosts, stat.FirstHost, stat.LastHost = total.String(), first.String(), last.String()
	}
	return stat
}

// CIDRHandler prints the subnet arithmetic for each prefix: network, mask,
// range, size and kind. It is pure computation - no network, no name lookup -
// so it cannot fail once the prefixes have parsed (see ParseSubnets) and always
// reports true.
func CIDRHandler(jsonoutput *bool, subnets []Subnet) (ok bool) {
	start := time.Now()
	stats := make([]lib.CIDRStats, 0, len(subnets))
	inputs := make([]string, 0, len(subnets))
	for _, s := range subnets {
		stats = append(stats, subnetStats(s))
		inputs = append(inputs, s.Input)
	}

	if *jsonoutput {
		end := time.Now()
		output := lib.LocalJSONOutput{
			InputParams:    lib.InputParams{Mode: cidrModule, Data: strings.Join(inputs, ",")},
			ModuleName:     cidrModule,
			Stats:          stats,
			StartTime:      start.UnixMicro(),
			EndTime:        end.UnixMicro(),
			TotalTimeTaken: end.Sub(start).Microseconds(),
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
		return true
	}

	for _, st := range stats {
		fields := []any{"input", st.Input, "network", st.Network, "family", st.Family, "netmask", st.Netmask}
		if st.Wildcard != "" {
			fields = append(fields, "wildcard", st.Wildcard)
		}
		fields = append(fields, "first", st.FirstAddress, "last", st.LastAddress)
		if st.Broadcast != "" {
			fields = append(fields, "broadcast", st.Broadcast)
		}
		fields = append(fields, "addresses", st.Addresses, "hosts", st.UsableHosts, "first_host", st.FirstHost, "last_host", st.LastHost, "kind", st.Kind)
		fmt.Println(lib.LogWithTimestamp(cidrModule, "subnet "+lib.Fields(fields...), false))
	}
	fmt.Println(lib.LogWithTimestamp(cidrModule, "done "+lib.Fields("prefixes", len(stats), "total_time", time.Since(start)), false))
	return true
}
