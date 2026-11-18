//go:build windows

package ip

import (
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GetIpNetTable2 has no wrapper in golang.org/x/sys/windows; it is called
// from iphlpapi.dll, loaded from the system directory only.
var (
	iphlpapi       = windows.NewLazySystemDLL("iphlpapi.dll")
	getIPNetTable2 = iphlpapi.NewProc("GetIpNetTable2")
)

// platformNeighbours reads Windows's neighbour table, IPv4 and IPv6, with
// GetIpNetTable2, which any user may call.
func platformNeighbours() ([]sysNeighbour, error) {
	if err := getIPNetTable2.Find(); err != nil {
		return nil, fmt.Errorf("could not read the neighbour table: %w", err)
	}
	var table unsafe.Pointer
	if r, _, _ := getIPNetTable2.Call(uintptr(windows.AF_UNSPEC), uintptr(unsafe.Pointer(&table))); r != 0 {
		return nil, fmt.Errorf("could not read the neighbour table: %w", windows.Errno(r))
	}
	defer windows.FreeMibTable(table)
	rows := *(*uint32)(table)
	b := unsafe.Slice((*byte)(table), winNeighbourHeader+int(rows)*winNeighbourRow)
	names := map[uint32]string{}
	if list, err := net.Interfaces(); err == nil {
		for _, ifc := range list {
			names[uint32(ifc.Index)] = ifc.Name
		}
	}
	return parseWinNeighbourTable(b, names)
}
