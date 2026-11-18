//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package ip

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// llinfo is the routing-socket sysctl for link-layer entries: NET_RT_FLAGS
// with RTF_LLINFO, asked once per address family.
var llinfo = route.RIBType(unix.NET_RT_FLAGS)

// platformNeighbours reads the neighbour table of macOS and the BSDs the way
// their own arp(8) and ndp(8) do: a routing-socket sysctl per family for the
// link-layer entries (NET_RT_FLAGS with RTF_LLINFO), whose gateway is the
// hardware address learned for the destination. A plain routing table dump
// (NET_RT_DUMP) is not enough: FreeBSD has kept these entries out of it since
// its link-layer tables were split from the routing table (only macOS still
// lists them there), so FreeBSD looked as if it had no neighbours at all. The
// system keeps no state words here, so State is empty.
func platformNeighbours() ([]sysNeighbour, error) {
	var msgs []route.Message
	for _, af := range []int{syscall.AF_INET, syscall.AF_INET6} {
		rib, err := route.FetchRIB(af, llinfo, unix.RTF_LLINFO)
		if err != nil {
			if af == syscall.AF_INET6 && (errors.Is(err, syscall.EAFNOSUPPORT) || errors.Is(err, syscall.ENOENT)) {
				continue // a kernel built without IPv6
			}
			return nil, fmt.Errorf("could not read the neighbour table: %w", err)
		}
		m, err := route.ParseRIB(llinfo, rib)
		if err != nil {
			return nil, fmt.Errorf("could not read the neighbour table: %w", err)
		}
		msgs = append(msgs, m...)
	}
	ifaces := map[int]bsdIface{}
	if list, err := net.Interfaces(); err == nil {
		for _, i := range list {
			ifaces[i.Index] = bsdIface{i.Name, i.Flags&net.FlagLoopback != 0}
		}
	}
	local := map[netip.Addr]bool{}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				if ip, ok := netip.AddrFromSlice(n.IP); ok {
					local[ip.Unmap().WithZone("")] = true
				}
			}
		}
	}
	return bsdNeighbours(msgs, ifaces, local), nil
}

type bsdIface struct {
	name     string
	loopback bool
}

// bsdNeighbours turns the link-layer entries into neighbours. It leaves out
// what is not a neighbour: an entry on a loopback interface, a multicast
// destination, the broadcast MAC, and this machine's own addresses (FreeBSD
// lists each as a permanent entry).
func bsdNeighbours(msgs []route.Message, ifaces map[int]bsdIface, local map[netip.Addr]bool) []sysNeighbour {
	var out []sysNeighbour
	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok || rm.Flags&unix.RTF_LLINFO == 0 || len(rm.Addrs) <= unix.RTAX_GATEWAY || ifaces[rm.Index].loopback {
			continue
		}
		dst, ok := routeAddr(rm.Addrs[unix.RTAX_DST])
		link, isLink := rm.Addrs[unix.RTAX_GATEWAY].(*route.LinkAddr)
		if !ok || !isLink || dst.IsMulticast() || isBroadcastMAC(link.Addr) || local[dst.WithZone("")] {
			continue
		}
		out = append(out, sysNeighbour{Addr: dst, MAC: macText(link.Addr), Iface: ifaces[rm.Index].name})
	}
	return out
}
