package handlers

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

const ipRouteModule = "ip-route"

// sysRoute is one routing-table entry as a platform reader found it, before it
// is filtered and written out.
type sysRoute struct {
	Prefix    netip.Prefix // masked
	Gateway   netip.Addr   // the next hop; not valid for a directly reached network
	Iface     string
	Metric    int
	HasMetric bool
	Scoped    bool // macOS: only for traffic bound to Iface (RTF_IFSCOPE)
	Reject    bool // drops what it matches
}

// limitedBroadcast (255.255.255.255) has a route on every interface on macOS
// and Windows; it says nothing about where traffic goes, and is left out.
var limitedBroadcast = netip.AddrFrom4([4]byte{255, 255, 255, 255})

// readRoutes is where the routing table comes from: iproute_linux.go,
// iproute_bsd.go (macOS and the BSDs), iproute_windows.go, or
// iproute_other.go, which says the system is not supported. A variable so
// tests can substitute a table.
var readRoutes = platformRoutes

// kernelRoute is the interface and source address the system would use to
// reach an address, found by connecting a UDP socket (which sends nothing). A
// variable so tests do not depend on the machine's network.
var kernelRoute = func(dst netip.Addr) (iface string, source netip.Addr, err error) {
	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.AddrPortFrom(dst, 9)))
	if err != nil {
		return "", netip.Addr{}, err
	}
	defer func() { _ = conn.Close() }()
	local := conn.LocalAddr().(*net.UDPAddr).AddrPort().Addr().Unmap()
	return interfaceOf(local), local, nil
}

// interfaceOf is the name of the interface that has address a, or "".
func interfaceOf(a netip.Addr) string {
	list, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifc := range list {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, x := range addrs {
			if ipnet, ok := x.(*net.IPNet); ok {
				if b, ok := netip.AddrFromSlice(ipnet.IP); ok && b.Unmap() == a.WithZone("") {
					return ifc.Name
				}
			}
		}
	}
	return ""
}

// routeStat writes a route out.
func routeStat(r sysRoute) lib.RouteStats {
	st := lib.RouteStats{Family: familyName(r.Prefix.Addr()), Destination: r.Prefix.String(), Default: r.Prefix.Bits() == 0,
		Interface: r.Iface, Scoped: r.Scoped, Unreachable: r.Reject}
	if r.Gateway.IsValid() && !r.Gateway.IsUnspecified() {
		st.Gateway = r.Gateway.String()
	}
	if r.HasMetric {
		m := r.Metric
		st.Metric = &m
	}
	return st
}

func familyName(a netip.Addr) string {
	if a.Is4() {
		return "ipv4"
	}
	return "ipv6"
}

// sortRoutes puts IPv4 before IPv6, default routes first (unscoped before
// scoped), then by destination and prefix length, then by metric.
func sortRoutes(rs []sysRoute) {
	slices.SortStableFunc(rs, func(a, b sysRoute) int {
		return cmp.Or(
			cmp.Compare(boolRank(!a.Prefix.Addr().Is4()), boolRank(!b.Prefix.Addr().Is4())),
			cmp.Compare(boolRank(a.Prefix.Bits() != 0), boolRank(b.Prefix.Bits() != 0)),
			cmp.Compare(boolRank(a.Scoped), boolRank(b.Scoped)),
			a.Prefix.Addr().Compare(b.Prefix.Addr()),
			cmp.Compare(a.Prefix.Bits(), b.Prefix.Bits()),
			cmp.Compare(a.Metric, b.Metric),
			cmp.Compare(a.Iface, b.Iface),
		)
	})
}

// matchRoute is the route that carries traffic to dst: the longest prefix that
// contains it, unscoped before scoped and the lower metric on a tie, among the
// routes on iface - the interface the system itself picked - or among all
// routes when that is not known. A route on another interface is never taken:
// when the system sends from an interface no listed route covers (Linux keeps
// local and loopback routes in a table of their own; policy routing uses
// others), there is no route to show, rather than a wrong one.
func matchRoute(rs []sysRoute, dst netip.Addr, iface string) (sysRoute, bool) {
	var best sysRoute
	found := false
	for _, r := range rs {
		if r.Reject || !r.Prefix.Contains(dst.WithZone("")) || (iface != "" && r.Iface != iface) {
			continue
		}
		if !found || r.Prefix.Bits() > best.Prefix.Bits() ||
			(r.Prefix.Bits() == best.Prefix.Bits() && (boolRank(r.Scoped) < boolRank(best.Scoped) || (r.Scoped == best.Scoped && r.Metric < best.Metric))) {
			best, found = r, true
		}
	}
	return best, found
}

// IPRouteHandler lists this machine's routing table - or, given an address,
// says which way traffic to it would go (#108). It reads the system's tables
// and sends nothing. It reports true when it listed at least one route, or
// found the way to the address.
func IPRouteHandler(jsonoutput *bool, target string) (ok bool) {
	start := time.Now()
	all, err := readRoutes()
	routes := make([]sysRoute, 0, len(all))
	for _, r := range all {
		if familyAllowed(r.Prefix.Addr()) {
			routes = append(routes, r)
		}
	}
	sortRoutes(routes)

	failure := ""
	var stats any
	var lookup *lib.RouteLookup
	switch {
	case err != nil:
		failure = err.Error()
		stats = make([]lib.RouteStats, 0)
	case target != "":
		lookup, failure = lookupRoute(routes, target)
		if lookup != nil {
			stats = []lib.RouteLookup{*lookup}
		} else {
			stats = make([]lib.RouteLookup, 0)
		}
	default:
		list := make([]lib.RouteStats, 0, len(routes))
		for _, r := range routes {
			list = append(list, routeStat(r))
		}
		stats = list
		if len(list) == 0 {
			failure = "this machine reports no routes"
		}
	}

	if *jsonoutput {
		end := time.Now()
		output := lib.LocalJSONOutput{
			InputParams: lib.InputParams{Mode: ipRouteModule, Data: target},
			ModuleName:  ipRouteModule,
			Stats:       stats,
			Error:       failure,
			StartTime:   start.UnixMicro(), EndTime: end.UnixMicro(), TotalTimeTaken: end.Sub(start).Microseconds(),
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
		return failure == ""
	}

	if failure != "" {
		fmt.Println(lib.LogWithTimestamp(ipRouteModule, "failed "+lib.Fields("error", failure), true))
		return false
	}
	if lookup != nil {
		fields := []any{"destination", lookup.Destination}
		for _, kv := range [][2]string{{"route", lookup.Route}, {"gateway", lookup.Gateway}, {"interface", lookup.Interface}, {"source", lookup.Source}} {
			if kv[1] != "" {
				fields = append(fields, kv[0], kv[1])
			}
		}
		fmt.Println(lib.LogWithTimestamp(ipRouteModule, "route "+lib.Fields(fields...), false))
		fmt.Println(lib.LogWithTimestamp(ipRouteModule, "done "+lib.Fields("total_time", time.Since(start)), false))
		return true
	}
	v4 := 0
	for _, r := range routes {
		st := routeStat(r)
		if st.Family == "ipv4" {
			v4++
		}
		dest := st.Destination
		if st.Default {
			dest = "default"
		}
		fields := []any{"destination", dest, "family", st.Family}
		if st.Gateway != "" {
			fields = append(fields, "gateway", st.Gateway)
		}
		if st.Interface != "" {
			fields = append(fields, "interface", st.Interface)
		}
		if st.Metric != nil {
			fields = append(fields, "metric", *st.Metric)
		}
		if st.Scoped {
			fields = append(fields, "scoped", true)
		}
		if st.Unreachable {
			fields = append(fields, "unreachable", true)
		}
		fmt.Println(lib.LogWithTimestamp(ipRouteModule, "route "+lib.Fields(fields...), false))
	}
	fmt.Println(lib.LogWithTimestamp(ipRouteModule, "done "+lib.Fields("routes", len(routes), "ipv4", v4, "ipv6", len(routes)-v4, "total_time", time.Since(start)), false))
	return true
}

// familyAllowed is whether -4/-6 lets an address through.
func familyAllowed(a netip.Addr) bool {
	switch lib.NetworkType {
	case "ip4":
		return a.Is4()
	case "ip6":
		return a.Is6()
	}
	return true
}

// lookupRoute finds the way to target: the system's own choice of interface
// and source, and the table's route for it. failure is set when there is no
// way there.
func lookupRoute(routes []sysRoute, target string) (*lib.RouteLookup, string) {
	dst, err := netip.ParseAddr(target)
	if err != nil {
		return nil, fmt.Sprintf("%q is not an IP address: give an address, like 8.8.8.8 or 2001:4860:4860::8888 (shint dns can look up a name)", target)
	}
	dst = dst.Unmap()
	iface, source, err := kernelRoute(dst)
	if err != nil {
		return nil, fmt.Sprintf("no route to %s: %s", dst, unwrapNet(err))
	}
	out := &lib.RouteLookup{Destination: dst.String(), Interface: iface}
	if source.IsValid() {
		out.Source = source.String()
	}
	if r, ok := matchRoute(routes, dst, iface); ok {
		out.Route = r.Prefix.String()
		if r.Gateway.IsValid() && !r.Gateway.IsUnspecified() {
			out.Gateway = r.Gateway.String()
		}
		if out.Interface == "" {
			out.Interface = r.Iface
		}
	}
	return out, ""
}

// unwrapNet is a network error's own words, without Go's "dial udp ...:"
// prefix.
func unwrapNet(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}
