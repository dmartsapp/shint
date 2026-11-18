//go:build linux

package ip

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
)

// platformNeighbours asks the kernel for its neighbour table over netlink
// (an RTM_GETNEIGH dump, IPv4 and IPv6 together), which any user may read.
func platformNeighbours() ([]sysNeighbour, error) {
	rib, err := syscall.NetlinkRIB(syscall.RTM_GETNEIGH, syscall.AF_UNSPEC)
	if err != nil {
		return nil, fmt.Errorf("could not read the neighbour table: %w", err)
	}
	msgs, err := syscall.ParseNetlinkMessage(rib)
	if err != nil {
		return nil, fmt.Errorf("could not read the neighbour table: %w", err)
	}
	names := map[int]string{}
	if list, err := net.Interfaces(); err == nil {
		for _, ifc := range list {
			names[ifc.Index] = ifc.Name
		}
	}
	var out []sysNeighbour
	for _, m := range msgs {
		if m.Header.Type != syscall.RTM_NEWNEIGH {
			continue
		}
		n, ok, err := parseNdMsg(m.Data, binary.NativeEndian, names)
		if err != nil {
			return nil, fmt.Errorf("could not read the neighbour table: %w", err)
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}
