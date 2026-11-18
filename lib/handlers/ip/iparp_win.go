package ip

import (
	"encoding/binary"
	"fmt"
)

// This file decodes the neighbour table Windows returns from GetIpNetTable2.
// It has no build tag, so its tests run everywhere on bytes laid out by hand;
// iparp_windows.go asks Windows for the table.
//
// MIB_IPNET_TABLE2 is a ULONG row count, padded to 8 bytes, then the rows.
// MIB_IPNET_ROW2 (88 bytes): Address (SOCKADDR_INET, 28), InterfaceIndex (4),
// InterfaceLuid (8), PhysicalAddress (32), PhysicalAddressLength (4), State
// (4), Flags (1: bit 0 IsRouter, bit 1 IsUnreachable, then padding to 4), and
// ReachabilityTime (4). Little-endian throughout.
const (
	winNeighbourHeader = 8
	winNeighbourRow    = 88
)

// windowsNeighbourStates are the NL_NEIGHBOR_STATE values.
var windowsNeighbourStates = map[uint32]string{
	0: "UNREACHABLE", 1: "INCOMPLETE", 2: "PROBE", 3: "DELAY", 4: "STALE", 5: "REACHABLE", 6: "PERMANENT",
}

// parseWinNeighbourTable decodes the table. Multicast and broadcast entries,
// which Windows lists as permanent neighbours, are left out. A row count the
// bytes cannot hold is an error rather than a read past the end.
func parseWinNeighbourTable(b []byte, names map[uint32]string) ([]sysNeighbour, error) {
	if len(b) < winNeighbourHeader {
		return nil, fmt.Errorf("the neighbour table is %d bytes, shorter than its header", len(b))
	}
	n := uint64(binary.LittleEndian.Uint32(b))
	if n*winNeighbourRow > uint64(len(b)-winNeighbourHeader) {
		return nil, fmt.Errorf("the neighbour table says it has %d rows, but holds %d bytes", n, len(b)-winNeighbourHeader)
	}
	var out []sysNeighbour
	for i := 0; i < int(n); i++ {
		r := b[winNeighbourHeader+i*winNeighbourRow : winNeighbourHeader+(i+1)*winNeighbourRow]
		addr, _, ok := winSockaddr(binary.LittleEndian.Uint16(r[0:2]), [24]byte(r[4:28]))
		if !ok {
			continue
		}
		macLen := min(int(binary.LittleEndian.Uint32(r[72:76])), 32)
		mac := r[40 : 40+macLen]
		if addr.IsMulticast() || addr == limitedBroadcast || isBroadcastMAC(mac) {
			continue
		}
		iface := names[binary.LittleEndian.Uint32(r[28:32])]
		if addr.Is6() && addr.IsLinkLocalUnicast() && iface != "" {
			addr = addr.WithZone(iface)
		}
		state := windowsNeighbourStates[binary.LittleEndian.Uint32(r[76:80])]
		out = append(out, sysNeighbour{Addr: addr, MAC: macText(mac), Iface: iface, State: state, Router: r[80]&1 != 0})
	}
	return out, nil
}
