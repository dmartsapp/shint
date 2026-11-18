package handlers

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

const ipArpModule = "ip-arp"

// sysNeighbour is one neighbour-table entry as a platform reader found it: an
// address on a local network, the hardware address learned for it ("" while
// unresolved, or when resolving failed), the interface, and what the system
// says about it.
type sysNeighbour struct {
	Addr   netip.Addr // a link-local IPv6 address carries its interface as zone
	MAC    string
	Iface  string
	State  string // REACHABLE, STALE, FAILED, ...; "" where the system does not say
	Router bool   // an IPv6 neighbour that announced itself as a router
}

// readNeighbours is where the neighbour table comes from: iparp_linux.go
// (netlink), iparp_bsd.go (the routing table's link-layer entries),
// iparp_windows.go, or iparp_other.go, which says the system is not supported.
// A variable so tests can substitute a table.
var readNeighbours = platformNeighbours

// IPArpHandler lists this machine's neighbour table - ARP for IPv4, NDP for
// IPv6 - or, given an interface, the neighbours on it (#109). Addresses are
// shown as they are: nothing is looked up, so it is instant and works when
// DNS does not. The neighbour that is a default route's gateway is marked. It
// reads the system's tables and sends nothing; it reports true when it listed
// at least one neighbour.
func IPArpHandler(jsonoutput *bool, iface string) (ok bool) {
	start := time.Now()
	all, err := readNeighbours()
	failure := ""
	if err == nil && iface != "" {
		if names, nerr := InterfaceNames(); nerr == nil && !slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, iface) }) {
			failure = noSuchInterface(iface, names)
		}
	}
	gateways := defaultGateways()

	stats := make([]lib.NeighbourStats, 0, len(all))
	var chosen []sysNeighbour
	for _, n := range all {
		if !familyAllowed(n.Addr) || (iface != "" && !strings.EqualFold(n.Iface, iface)) {
			continue
		}
		chosen = append(chosen, n)
	}
	slices.SortStableFunc(chosen, func(a, b sysNeighbour) int {
		return cmp.Or(
			cmp.Compare(boolRank(!a.Addr.Is4()), boolRank(!b.Addr.Is4())),
			cmp.Compare(a.Iface, b.Iface),
			a.Addr.Compare(b.Addr),
		)
	})
	for _, n := range chosen {
		stats = append(stats, lib.NeighbourStats{Family: familyName(n.Addr), Address: n.Addr.String(), MAC: n.MAC, Interface: n.Iface,
			State: n.State, Router: n.Router, Gateway: gateways[n.Addr]})
	}

	switch {
	case err != nil:
		failure = err.Error()
	case failure != "":
	case len(stats) == 0 && iface != "":
		failure = "no neighbours known on " + escapeBytes([]byte(iface))
	case len(stats) == 0:
		failure = "this machine knows no neighbours"
	}

	if *jsonoutput {
		end := time.Now()
		output := lib.LocalJSONOutput{
			InputParams: lib.InputParams{Mode: ipArpModule, Data: iface},
			ModuleName:  ipArpModule,
			Stats:       stats,
			Error:       failure,
			StartTime:   start.UnixMicro(), EndTime: end.UnixMicro(), TotalTimeTaken: end.Sub(start).Microseconds(),
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
		return failure == ""
	}

	if failure != "" {
		fmt.Println(lib.LogWithTimestamp(ipArpModule, "failed "+lib.Fields("error", failure), true))
		return false
	}
	v4 := 0
	for _, st := range stats {
		if st.Family == "ipv4" {
			v4++
		}
		fields := []any{"address", st.Address, "family", st.Family}
		if st.MAC != "" {
			fields = append(fields, "mac", st.MAC)
		}
		if st.Interface != "" {
			fields = append(fields, "interface", st.Interface)
		}
		if st.State != "" {
			fields = append(fields, "state", st.State)
		}
		if st.Router {
			fields = append(fields, "router", true)
		}
		if st.Gateway {
			fields = append(fields, "gateway", true)
		}
		fmt.Println(lib.LogWithTimestamp(ipArpModule, "neighbour "+lib.Fields(fields...), false))
	}
	fmt.Println(lib.LogWithTimestamp(ipArpModule, "done "+lib.Fields("neighbours", len(stats), "ipv4", v4, "ipv6", len(stats)-v4, "total_time", time.Since(start)), false))
	return true
}

// defaultGateways are the next hops of this machine's default routes, so the
// neighbour that is one can be marked. A routing table that cannot be read
// marks nothing: the neighbour table stands on its own.
func defaultGateways() map[netip.Addr]bool {
	out := map[netip.Addr]bool{}
	routes, err := readRoutes()
	if err != nil {
		return out
	}
	for _, r := range routes {
		if r.Prefix.Bits() == 0 && r.Gateway.IsValid() {
			out[r.Gateway] = true
		}
	}
	return out
}

// macText writes a hardware address the usual way (aa:bb:cc:dd:ee:ff), or ""
// for none: empty, or all zeros (an entry still resolving, or one that
// failed).
func macText(b []byte) string {
	zero := true
	for _, x := range b {
		if x != 0 {
			zero = false
		}
	}
	if len(b) == 0 || zero {
		return ""
	}
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02x", x)
	}
	return strings.Join(parts, ":")
}

// isBroadcastMAC is ff:ff:ff:ff:ff:ff, which a table may hold for a
// network's broadcast address - not a neighbour.
func isBroadcastMAC(b []byte) bool {
	if len(b) != 6 {
		return false
	}
	for _, x := range b {
		if x != 0xff {
			return false
		}
	}
	return true
}
