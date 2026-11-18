//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package handlers

import (
	"fmt"
	"net"
	"syscall"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// platformNeighbours reads the neighbour table of macOS and the BSDs: the
// routing table's link-layer entries (RTF_LLINFO), whose gateway is the
// hardware address learned for the destination - the entries ip route leaves
// out. The system keeps no state words here, so State is empty.
func platformNeighbours() ([]sysNeighbour, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, fmt.Errorf("could not read the neighbour table: %w", err)
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, fmt.Errorf("could not read the neighbour table: %w", err)
	}
	type ifc struct {
		name     string
		loopback bool
	}
	ifaces := map[int]ifc{}
	if list, err := net.Interfaces(); err == nil {
		for _, i := range list {
			ifaces[i.Index] = ifc{i.Name, i.Flags&net.FlagLoopback != 0}
		}
	}
	var out []sysNeighbour
	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok || rm.Flags&unix.RTF_LLINFO == 0 || len(rm.Addrs) <= unix.RTAX_GATEWAY || ifaces[rm.Index].loopback {
			continue
		}
		dst, ok := routeAddr(rm.Addrs[unix.RTAX_DST])
		link, isLink := rm.Addrs[unix.RTAX_GATEWAY].(*route.LinkAddr)
		if !ok || !isLink || dst.IsMulticast() || isBroadcastMAC(link.Addr) {
			continue
		}
		out = append(out, sysNeighbour{Addr: dst, MAC: macText(link.Addr), Iface: ifaces[rm.Index].name})
	}
	return out, nil
}
