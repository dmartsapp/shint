package ip

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

// This file decodes Linux's netlink neighbour messages (RTM_NEWNEIGH, the
// answer to an RTM_GETNEIGH dump). It has no build tag, so its tests run
// everywhere; iparp_linux.go asks the kernel for them.

// The neighbour states (include/uapi/linux/neighbour.h, NUD_*).
var linuxNeighbourStates = map[uint16]string{
	0x01: "INCOMPLETE", 0x02: "REACHABLE", 0x04: "STALE", 0x08: "DELAY",
	0x10: "PROBE", 0x20: "FAILED", 0x40: "NOARP", 0x80: "PERMANENT",
}

const (
	nudNoARP  = 0x40
	ntfRouter = 0x80 // ndm_flags: the neighbour is a router
	ndaDst    = 1    // attribute: the neighbour's address
	ndaLLAddr = 2    // attribute: its hardware address
)

// parseNdMsg decodes one neighbour message's data: a struct ndmsg (family,
// three bytes of padding, interface index, state, flags, type: 12 bytes) and
// then its attributes, each "length, type, value" padded to four bytes. names
// maps interface indexes to names. ok is false for an entry that is not an
// IPv4 or IPv6 neighbour, or one marked NOARP (loopback, multicast: no
// neighbour at all).
func parseNdMsg(b []byte, order binary.ByteOrder, names map[int]string) (sysNeighbour, bool, error) {
	if len(b) < 12 {
		return sysNeighbour{}, false, errors.New("a neighbour message shorter than its header")
	}
	family := b[0]
	ifindex := int(int32(order.Uint32(b[4:8])))
	state := order.Uint16(b[8:10])
	flags := b[10]
	var addr netip.Addr
	var mac []byte
	for rest := b[12:]; len(rest) >= 4; {
		n := int(order.Uint16(rest[0:2]))
		if n < 4 || n > len(rest) {
			return sysNeighbour{}, false, errors.New("a neighbour attribute runs past its message")
		}
		typ, val := order.Uint16(rest[2:4]), rest[4:n]
		switch typ {
		case ndaDst:
			if a, ok := netip.AddrFromSlice(val); ok {
				addr = a
			}
		case ndaLLAddr:
			mac = val
		}
		rest = rest[min((n+3)&^3, len(rest)):]
	}
	if (family != 2 && family != 10) || !addr.IsValid() || state&nudNoARP != 0 || addr.IsMulticast() || isBroadcastMAC(mac) {
		return sysNeighbour{}, false, nil
	}
	n := sysNeighbour{Addr: addr, MAC: macText(mac), Iface: names[ifindex], State: linuxNeighbourStates[state], Router: flags&ntfRouter != 0}
	if addr.IsLinkLocalUnicast() && addr.Is6() && n.Iface != "" {
		n.Addr = addr.WithZone(n.Iface)
	}
	return n, true, nil
}
