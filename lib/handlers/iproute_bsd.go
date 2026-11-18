//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package handlers

import (
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// platformRoutes reads the routing table of macOS and the BSDs through the
// routing socket's dump (sysctl NET_RT_DUMP), which any user may read.
func platformRoutes() ([]sysRoute, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, fmt.Errorf("could not read the routing table: %w", err)
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, fmt.Errorf("could not read the routing table: %w", err)
	}
	names := map[int]string{}
	if list, err := net.Interfaces(); err == nil {
		for _, ifc := range list {
			names[ifc.Index] = ifc.Name
		}
	}
	var out []sysRoute
	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok {
			continue
		}
		if r, ok := bsdRoute(rm.Flags, rm.Addrs, names[rm.Index]); ok {
			out = append(out, r)
		}
	}
	return out, nil
}

// bsdRoute turns one route message into a route, or leaves it out: routes
// that are not up, ARP/NDP cache entries (RTF_LLINFO, whose "gateway" is a
// hardware address), host routes cloned from a network route (RTF_WASCLONED on
// macOS), multicast destinations and the limited broadcast address - none of
// them a route anyone set up.
func bsdRoute(flags int, addrs []route.Addr, iface string) (sysRoute, bool) {
	if flags&unix.RTF_UP == 0 || flags&unix.RTF_LLINFO != 0 || flags&rtfWasCloned != 0 || len(addrs) <= unix.RTAX_DST {
		return sysRoute{}, false
	}
	dst, ok := routeAddr(addrs[unix.RTAX_DST])
	if !ok || dst.IsMulticast() || dst == limitedBroadcast {
		return sysRoute{}, false
	}
	bits := dst.BitLen()
	if flags&unix.RTF_HOST == 0 {
		bits = 0
		if len(addrs) > unix.RTAX_NETMASK {
			if mask, ok := maskBits(addrs[unix.RTAX_NETMASK], dst.BitLen()); ok {
				bits = mask
			}
		}
	}
	prefix, err := dst.WithZone("").Prefix(bits)
	if err != nil {
		return sysRoute{}, false
	}
	r := sysRoute{Prefix: prefix, Iface: iface, Scoped: flags&rtfIfScope != 0, Reject: flags&(unix.RTF_REJECT|unix.RTF_BLACKHOLE) != 0}
	if flags&unix.RTF_GATEWAY != 0 && len(addrs) > unix.RTAX_GATEWAY {
		if gw, ok := routeAddr(addrs[unix.RTAX_GATEWAY]); ok {
			r.Gateway = gw
		}
	}
	return r, true
}

// routeAddr is an IPv4 or IPv6 address from a route message, with the
// interface zone of a link-local IPv6 address (fe80::1%en0).
func routeAddr(a route.Addr) (netip.Addr, bool) {
	switch v := a.(type) {
	case *route.Inet4Addr:
		return netip.AddrFrom4(v.IP), true
	case *route.Inet6Addr:
		ip := netip.AddrFrom16(v.IP)
		if v.ZoneID != 0 && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
			if ifc, err := net.InterfaceByIndex(v.ZoneID); err == nil {
				ip = ip.WithZone(ifc.Name)
			}
		}
		return ip, true
	}
	return netip.Addr{}, false
}

// maskBits is the prefix length of a netmask from a route message. The
// kernel leaves out a mask's trailing zero bytes, so a short one is padded; a
// mask that is not contiguous is not a prefix.
func maskBits(a route.Addr, bitLen int) (int, bool) {
	var b []byte
	switch v := a.(type) {
	case *route.Inet4Addr:
		b = v.IP[:]
	case *route.Inet6Addr:
		b = v.IP[:]
	default:
		return 0, false
	}
	ones, size := net.IPMask(b[:bitLen/8]).Size()
	if size == 0 {
		return 0, false
	}
	return ones, true
}
