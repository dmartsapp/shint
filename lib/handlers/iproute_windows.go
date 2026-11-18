//go:build windows

package handlers

import (
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// platformRoutes reads Windows's routing table, IPv4 and IPv6, with
// GetIpForwardTable2, which any user may call. Multicast routes and the
// limited broadcast address are left out.
func platformRoutes() ([]sysRoute, error) {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_UNSPEC, &table); err != nil {
		return nil, fmt.Errorf("could not read the routing table: %w", err)
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	names := map[uint32]string{}
	if list, err := net.Interfaces(); err == nil {
		for _, ifc := range list {
			names[uint32(ifc.Index)] = ifc.Name
		}
	}
	var out []sysRoute
	for _, row := range table.Rows() {
		dst, _, ok := winSockaddr(row.DestinationPrefix.Prefix.Family, *(*[24]byte)(unsafe.Pointer(&row.DestinationPrefix.Prefix.Data)))
		if !ok || dst.IsMulticast() || dst == limitedBroadcast {
			continue
		}
		prefix, err := dst.Prefix(int(row.DestinationPrefix.PrefixLength))
		if err != nil {
			continue
		}
		r := sysRoute{Prefix: prefix, Iface: names[row.InterfaceIndex], Metric: int(row.Metric), HasMetric: true}
		if gw, scope, ok := winSockaddr(row.NextHop.Family, *(*[24]byte)(unsafe.Pointer(&row.NextHop.Data))); ok && !gw.IsUnspecified() {
			if scope != 0 && gw.IsLinkLocalUnicast() {
				gw = gw.WithZone(names[scope])
			}
			r.Gateway = gw
		}
		out = append(out, r)
	}
	return out, nil
}
