//go:build windows

package ss

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/dmartsapp/shint/v4/lib"
)

// iphlpapi is loaded from the system directory only, never from the current
// directory or PATH.
var (
	iphlpapi            = windows.NewLazySystemDLL("iphlpapi.dll")
	getExtendedTCPTable = iphlpapi.NewProc("GetExtendedTcpTable")
	getExtendedUDPTable = iphlpapi.NewProc("GetExtendedUdpTable")
)

// The table classes that include the owning process: TCP_TABLE_OWNER_PID_ALL
// (every state, listeners included) and UDP_TABLE_OWNER_PID.
const (
	tcpTableOwnerPIDAll = 5
	udpTableOwnerPID    = 1
)

// platformSockets reads Windows's TCP and UDP tables, IPv4 and IPv6. They
// carry each socket's owning PID, which any user may read, so no owner is
// hidden; process names come from a snapshot of the process list.
func platformSockets() ([]lib.SSStats, error) {
	var rows []winSocket
	for _, t := range []struct {
		proc             *windows.LazyProc
		class            uintptr
		af               uintptr
		protocol, family string
	}{
		{getExtendedTCPTable, tcpTableOwnerPIDAll, windows.AF_INET, "tcp", "ipv4"},
		{getExtendedTCPTable, tcpTableOwnerPIDAll, windows.AF_INET6, "tcp", "ipv6"},
		{getExtendedUDPTable, udpTableOwnerPID, windows.AF_INET, "udp", "ipv4"},
		{getExtendedUDPTable, udpTableOwnerPID, windows.AF_INET6, "udp", "ipv6"},
	} {
		b, err := extendedTable(t.proc, t.af, t.class)
		if err != nil {
			return nil, fmt.Errorf("could not read the %s %s socket table: %w", t.family, t.protocol, err)
		}
		got, err := parseWinTable(b, t.protocol, t.family)
		if err != nil {
			return nil, err
		}
		rows = append(rows, got...)
	}
	return winSockets(rows, processNames()), nil
}

// extendedTable calls GetExtendedTcpTable or GetExtendedUdpTable, which say
// how big a buffer they need; the table can grow between the asking and the
// reading, so it asks again a few times.
func extendedTable(proc *windows.LazyProc, af, class uintptr) ([]byte, error) {
	if err := proc.Find(); err != nil {
		return nil, err
	}
	var size uint32
	var buf []byte
	for range 8 {
		var p uintptr
		if size > 0 {
			buf = make([]byte, size)
			p = uintptr(unsafe.Pointer(&buf[0]))
		}
		r, _, _ := proc.Call(p, uintptr(unsafe.Pointer(&size)), 0, af, class, 0)
		switch {
		case r == 0 && buf != nil:
			return buf, nil
		case r == 0 || windows.Errno(r) == windows.ERROR_INSUFFICIENT_BUFFER:
			continue
		default:
			return nil, windows.Errno(r)
		}
	}
	return nil, errors.New("the table kept growing while it was read")
}

// processNames maps PIDs to program names from a snapshot of the process
// list, which any user may take. A process that cannot be named is left out,
// and its sockets show no name.
func processNames() map[uint32]string {
	names := map[uint32]string{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return names
	}
	defer func() { _ = windows.CloseHandle(snap) }()
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		names[e.ProcessID] = windows.UTF16ToString(e.ExeFile[:])
	}
	return names
}
