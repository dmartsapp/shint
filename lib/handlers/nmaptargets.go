package handlers

import (
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"strings"

	"github.com/dmartsapp/shint/v4/lib"
)

// maxSweepAddresses is the most addresses the prefixes of one nmap run may
// cover: a /16 of IPv4, a /112 of IPv6. A sweep wider than that runs for hours
// and, with --json, holds a result for every address and port in memory.
const maxSweepAddresses = 1 << 16

// NmapTarget is one argument to nmap: a host name or an address, which is
// looked up, or a prefix (192.168.1.0/24), whose addresses are swept.
type NmapTarget struct {
	Input  string
	Prefix netip.Prefix // valid for a prefix only, with its host bits cleared
}

// IsPrefix is whether the target is a prefix to sweep.
func (t NmapTarget) IsPrefix() bool { return t.Prefix.IsValid() }

// ParseNmapTargets checks every argument before anything is scanned, so a typo
// in the third target is a usage error rather than two scans and a failure.
// -4/-6 must already be set: an address or prefix of the other family is
// refused here.
func ParseNmapTargets(inputs []string) ([]NmapTarget, error) {
	targets := make([]NmapTarget, 0, len(inputs))
	var swept int
	for _, input := range inputs {
		if strings.TrimSpace(input) == "" {
			return nil, fmt.Errorf("empty target: give a host name, an address or a prefix like 192.168.1.0/24")
		}
		if strings.Contains(input, "/") {
			prefix, err := netip.ParsePrefix(input)
			if err != nil {
				return nil, fmt.Errorf("invalid prefix %q: %s", input, reason(err))
			}
			if prefix.Addr().Zone() != "" {
				return nil, fmt.Errorf("invalid prefix %q: zone identifiers are not supported", input)
			}
			if !lib.FamilyAllows(net.IP(prefix.Addr().AsSlice())) {
				return nil, fmt.Errorf("%s is an %s prefix, but %s was requested", input, familyOf(prefix.Addr()), familyFlag())
			}
			n := sweepSize(prefix)
			if n > maxSweepAddresses {
				return nil, fmt.Errorf("%s has %s addresses to scan; one run sweeps at most %d (a /16 of IPv4): split it into smaller prefixes", input, sweepSizeText(prefix), maxSweepAddresses)
			}
			if swept += int(n); swept > maxSweepAddresses {
				return nil, fmt.Errorf("the prefixes given have %d addresses to scan; one run sweeps at most %d (a /16 of IPv4)", swept, maxSweepAddresses)
			}
			targets = append(targets, NmapTarget{Input: input, Prefix: prefix.Masked()})
			continue
		}
		if _, err := netip.ParseAddr(input); err == nil {
			if err := lib.HostFamilyConflict(input); err != nil {
				return nil, err
			}
		} else if !hostNameShape(input) {
			return nil, fmt.Errorf("invalid target %q: not a host name, an address or a prefix like 192.168.1.0/24", input)
		}
		targets = append(targets, NmapTarget{Input: input})
	}
	return targets, nil
}

// hostNameShape is whether s could be a host name: letters, digits, hyphens,
// underscores and dots, at most 253 characters, not starting with a hyphen or
// a dot. Whether it exists is the lookup's business; this keeps spaces,
// control characters and shell leftovers out of it.
func hostNameShape(s string) bool {
	if len(s) > 253 || s[0] == '-' || s[0] == '.' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !hostNameByte(s[i]) {
			return false
		}
	}
	return true
}

func hostNameByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return c == '-' || c == '.' || c == '_'
}

// localRanges are the ranges a sweep covers at full speed: private (RFC 1918
// and unique-local fc00::/7), shared address space (100.64.0.0/10, carrier
// NAT), loopback and link-local. Every other range counts as public -
// documentation and reserved ones too - so a sweep there gets the warning and
// the pace (#106).
var localRanges = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("::1/128"),
}

// isPublicSweep is whether a prefix sweeps addresses outside localRanges. A
// prefix of a single address (a /32, a /128) is a host, not a sweep.
func isPublicSweep(p netip.Prefix) bool {
	if sweepSize(p) <= 1 {
		return false
	}
	for _, r := range localRanges {
		if r.Bits() <= p.Bits() && r.Contains(p.Addr()) {
			return false
		}
	}
	return true
}

// sweepSize is how many addresses a prefix sweeps, or maxSweepAddresses+1 for
// one wider than any sweep may be. An IPv4 prefix of /30 or wider leaves out
// its network and broadcast addresses, which no host has; a /31 and a /32 have
// neither (RFC 3021). IPv6 has no broadcast address, so every address counts.
func sweepSize(p netip.Prefix) uint64 {
	hostBits := p.Addr().BitLen() - p.Bits()
	if hostBits > 17 {
		return maxSweepAddresses + 1
	}
	n := uint64(1) << hostBits
	if p.Addr().Is4() && hostBits >= 2 {
		n -= 2
	}
	return n
}

// sweepSizeText is a prefix's address count written out, however large (an
// IPv6 /64 holds 2^64).
func sweepSizeText(p netip.Prefix) string {
	hostBits := p.Addr().BitLen() - p.Bits()
	n := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))
	if p.Addr().Is4() && hostBits >= 2 {
		n.Sub(n, big.NewInt(2))
	}
	return n.String()
}

// sweepAddresses lists the addresses a prefix sweeps, in order.
func sweepAddresses(p netip.Prefix) []string {
	p = p.Masked()
	n := sweepSize(p)
	out := make([]string, 0, n)
	a := p.Addr()
	if p.Addr().Is4() && p.Addr().BitLen()-p.Bits() >= 2 {
		a = a.Next() // past the network address
	}
	for i := uint64(0); i < n; i++ {
		out = append(out, a.String())
		a = a.Next()
	}
	return out
}

func familyOf(a netip.Addr) string {
	if a.Is4() {
		return "IPv4"
	}
	return "IPv6"
}

// familyFlag is the -4/-6 flag in force (callers check that one is).
func familyFlag() string {
	if lib.NetworkType == "ip4" {
		return "-4/--ipv4"
	}
	return "-6/--ipv6"
}
