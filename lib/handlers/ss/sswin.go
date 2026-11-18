package ss

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/dmartsapp/shint/v4/lib"
)

// This file decodes the socket tables Windows returns from
// GetExtendedTcpTable and GetExtendedUdpTable with the OWNER_PID classes. It
// has no build tag, so its tests run everywhere on recorded bytes;
// ss_windows.go is what asks Windows for them.
//
// Each table is a DWORD row count followed by the rows. Every field is a
// little-endian DWORD except an IPv6 address (16 bytes); addresses are in
// network byte order, and a port is in network byte order in the low two
// bytes of its DWORD.

// winRowSize is the size of one row: MIB_TCPROW_OWNER_PID,
// MIB_TCP6ROW_OWNER_PID, MIB_UDPROW_OWNER_PID, MIB_UDP6ROW_OWNER_PID.
var winRowSize = map[string]int{"tcp/ipv4": 24, "tcp/ipv6": 56, "udp/ipv4": 12, "udp/ipv6": 28}

// windowsTCPStates are the MIB_TCP_STATE values.
var windowsTCPStates = map[uint32]string{
	1: "CLOSE", 2: "LISTEN", 3: "SYN-SENT", 4: "SYN-RECV", 5: "ESTABLISHED", 6: "FIN-WAIT-1",
	7: "FIN-WAIT-2", 8: "CLOSE-WAIT", 9: "CLOSING", 10: "LAST-ACK", 11: "TIME-WAIT", 12: "DELETE-TCB",
}

// winSocket is a table row and the PID that owns it (0: none, as in
// TIME-WAIT).
type winSocket struct {
	stat lib.SSStats
	pid  uint32
}

// parseWinTable decodes one table. A row count the bytes cannot hold is an
// error rather than a read past the end.
func parseWinTable(b []byte, protocol, family string) ([]winSocket, error) {
	size := winRowSize[protocol+"/"+family]
	if len(b) < 4 {
		return nil, fmt.Errorf("the %s %s table is %d bytes, shorter than its header", family, protocol, len(b))
	}
	n := uint64(binary.LittleEndian.Uint32(b))
	if n*uint64(size) > uint64(len(b)-4) {
		return nil, fmt.Errorf("the %s %s table says it has %d rows, but holds %d bytes", family, protocol, n, len(b)-4)
	}
	out := make([]winSocket, 0, n)
	for i := 0; i < int(n); i++ {
		r := b[4+i*size : 4+(i+1)*size]
		out = append(out, winRow(r, protocol, family))
	}
	return out, nil
}

func winRow(r []byte, protocol, family string) winSocket {
	dword := func(at int) uint32 { return binary.LittleEndian.Uint32(r[at:]) }
	port := func(at int) uint16 { return uint16(r[at])<<8 | uint16(r[at+1]) }
	addr4 := func(at int) netip.Addr { return netip.AddrFrom4([4]byte(r[at : at+4])) }
	addr6 := func(at, scopeAt int) netip.Addr {
		a := netip.AddrFrom16([16]byte(r[at : at+16]))
		if scope := dword(scopeAt); scope != 0 && (a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast()) {
			a = a.WithZone(strconv.FormatUint(uint64(scope), 10))
		}
		return a
	}

	var local, remote netip.AddrPort
	var state, pid uint32
	switch protocol + "/" + family {
	case "tcp/ipv4": // state, local addr, local port, remote addr, remote port, pid
		state, pid = dword(0), dword(20)
		local, remote = netip.AddrPortFrom(addr4(4), port(8)), netip.AddrPortFrom(addr4(12), port(16))
	case "tcp/ipv6": // local addr[16], scope, port, remote addr[16], scope, port, state, pid
		local, remote = netip.AddrPortFrom(addr6(0, 16), port(20)), netip.AddrPortFrom(addr6(24, 40), port(44))
		state, pid = dword(48), dword(52)
	case "udp/ipv4": // local addr, local port, pid
		local, pid = netip.AddrPortFrom(addr4(0), port(4)), dword(8)
	case "udp/ipv6": // local addr[16], scope, port, pid
		local, pid = netip.AddrPortFrom(addr6(0, 16), port(20)), dword(24)
	}

	s := lib.SSStats{Protocol: protocol, Family: family, LocalAddress: local.Addr().String(), LocalPort: int(local.Port())}
	if protocol == "tcp" {
		s.State = windowsTCPStates[state]
		if s.State == "" {
			s.State = fmt.Sprintf("UNKNOWN-%d", state)
		}
		setRemote(&s, remote)
	}
	return winSocket{stat: s, pid: pid}
}

// winSockets puts process names to the rows: PID 0 is no process (the
// System Idle Process, which Windows names for sockets in TIME-WAIT).
func winSockets(rows []winSocket, names map[uint32]string) []lib.SSStats {
	out := make([]lib.SSStats, 0, len(rows))
	for _, r := range rows {
		if r.pid != 0 {
			r.stat.Processes = []lib.SSProcess{{PID: int(r.pid), Name: names[r.pid]}}
		}
		out = append(out, r.stat)
	}
	return out
}
