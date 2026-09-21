package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

const ipModule = "ip"

// ifaceInfo is what "ip" needs to know about one interface, separated from the
// net package's types so tests can describe interfaces without owning any.
type ifaceInfo struct {
	Name     string
	Index    int
	MTU      int
	MAC      string
	Flags    net.Flags
	Addrs    []net.Addr
	AddrsErr error
}

// systemInterfaces is where the list comes from. It is a variable so tests can
// substitute interfaces, the way probePort is for nmap; in the program it is the
// operating system's interface table (net.Interfaces), which needs no privileges.
var systemInterfaces = func() ([]ifaceInfo, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]ifaceInfo, 0, len(list))
	for i := range list {
		info := ifaceInfo{Name: list[i].Name, Index: list[i].Index, MTU: list[i].MTU, MAC: list[i].HardwareAddr.String(), Flags: list[i].Flags}
		info.Addrs, info.AddrsErr = list[i].Addrs()
		out = append(out, info)
	}
	return out, nil
}

// flagNames are the flags of an interface as words, in the order the net
// package prints them, without the "up" that State already says.
func flagNames(f net.Flags) []string {
	names := make([]string, 0, 6)
	for _, part := range strings.Split(f.String(), "|") {
		if part != "" && part != "up" && part != "0" {
			names = append(names, part)
		}
	}
	return names
}

// interfaceStats turns one interface into its report. Only addresses the
// -4/-6 family choice allows are listed. An address the net package returns in
// a form that is not an IP network is skipped: there is nothing to say about it.
func interfaceStats(info ifaceInfo) lib.IPStats {
	stat := lib.IPStats{Name: info.Name, Index: info.Index, State: "down", Flags: flagNames(info.Flags), MTU: info.MTU, MAC: info.MAC, Addresses: []lib.IPAddress{}}
	if info.Flags&net.FlagUp != 0 {
		stat.State = "up"
	}
	if info.AddrsErr != nil {
		stat.Error = info.AddrsErr.Error()
	}
	for _, a := range info.Addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || !lib.FamilyAllows(ipnet.IP) {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipnet.IP)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		ones, bits := ipnet.Mask.Size()
		if bits == 0 { // a mask the net package could not express as a prefix
			continue
		}
		if addr.Is4() && bits == 128 { // an IPv4 address carried with a 16-byte mask
			ones -= 96
		}
		family := "ipv6"
		if addr.Is4() {
			family = "ipv4"
		}
		stat.Addresses = append(stat.Addresses, lib.IPAddress{
			Address:      addr.String(),
			Prefix:       fmt.Sprintf("%s/%d", addr, ones),
			PrefixLength: ones,
			Family:       family,
			Kind:         addressKind(addr),
		})
	}
	return stat
}

// IPHandler lists this machine's network interfaces - or, given a name, that
// one - with their state, MTU, hardware address and every address. It reads the
// operating system's interface table and sends nothing, so it needs no
// privileges and no network.
//
// It reports true when it listed at least one interface. An interface name that
// does not exist is a failed check (with the names that do), not a usage error:
// the name is data, and what is available depends on the machine.
func IPHandler(jsonoutput *bool, name string) (ok bool) {
	start := time.Now()
	all, err := systemInterfaces()
	var chosen []ifaceInfo
	for _, info := range all {
		if name == "" || strings.EqualFold(info.Name, name) {
			chosen = append(chosen, info)
		}
	}
	failure := ""
	switch {
	case err != nil:
		failure = "could not read the interface table: " + err.Error()
	case name != "" && len(chosen) == 0:
		names := make([]string, 0, len(all))
		for _, info := range all {
			names = append(names, info.Name)
		}
		failure = fmt.Sprintf("no such interface %s (available: %s)", name, strings.Join(names, ", "))
	case len(chosen) == 0:
		failure = "this machine reports no network interfaces"
	}

	stats := make([]lib.IPStats, 0, len(chosen))
	addresses, unreadable := 0, 0
	for _, info := range chosen {
		st := interfaceStats(info)
		addresses += len(st.Addresses)
		if st.Error != "" {
			unreadable++
		}
		stats = append(stats, st)
	}
	if failure == "" && unreadable > 0 {
		failure = fmt.Sprintf("the addresses of %d interface(s) could not be read", unreadable)
	}

	if *jsonoutput {
		end := time.Now()
		output := lib.LocalJSONOutput{
			InputParams:    lib.InputParams{Mode: ipModule, Data: name},
			ModuleName:     ipModule,
			Stats:          stats,
			Error:          failure,
			StartTime:      start.UnixMicro(),
			EndTime:        end.UnixMicro(),
			TotalTimeTaken: end.Sub(start).Microseconds(),
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
		return failure == ""
	}

	if failure != "" && len(stats) == 0 {
		fmt.Println(lib.LogWithTimestamp(ipModule, "failed "+lib.Fields("error", failure), true))
		return false
	}
	for _, st := range stats {
		fields := []any{"name", st.Name, "index", st.Index, "state", st.State, "flags", "[" + strings.Join(st.Flags, ",") + "]", "mtu", st.MTU}
		if st.MAC != "" {
			fields = append(fields, "mac", st.MAC)
		}
		fields = append(fields, "addresses", len(st.Addresses))
		if st.Error != "" {
			fields = append(fields, "error", st.Error)
		}
		fmt.Println(lib.LogWithTimestamp(ipModule, "interface "+lib.Fields(fields...), st.Error != ""))
		for _, a := range st.Addresses {
			fmt.Println(lib.LogWithTimestamp(ipModule, "address "+lib.Fields("interface", st.Name, "address", a.Address, "prefix", a.Prefix, "family", a.Family, "kind", a.Kind), false))
		}
	}
	fmt.Println(lib.LogWithTimestamp(ipModule, "done "+lib.Fields("interfaces", len(stats), "addresses", addresses, "total_time", time.Since(start)), false))
	return failure == ""
}
