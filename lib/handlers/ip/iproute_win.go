package ip

import (
	"encoding/binary"
	"net/netip"
)

// winSockaddr is the address in a Windows SOCKADDR_INET, given its family and
// the 24 bytes after the family and port: for AF_INET (2) the address is the
// first 4 bytes; for AF_INET6 (23) the flow information comes first, then the
// 16-byte address, then the scope ID. No build tag, so it is tested everywhere;
// iproute_windows.go and the neighbour table use it.
func winSockaddr(family uint16, data [24]byte) (addr netip.Addr, scope uint32, ok bool) {
	switch family {
	case 2:
		return netip.AddrFrom4([4]byte(data[0:4])), 0, true
	case 23:
		return netip.AddrFrom16([16]byte(data[4:20])), binary.LittleEndian.Uint32(data[20:24]), true
	}
	return netip.Addr{}, 0, false
}
