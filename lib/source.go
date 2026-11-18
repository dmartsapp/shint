package lib

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
	"unicode"
)

// --interface (#69): the checks' traffic leaves from a given interface, or a
// given address of this machine. It binds the source address - the portable,
// unprivileged way (curl --interface does the same) - and never binds a device
// by name, which needs privileges on some systems. Set once at startup with
// SetSourceInterface, like NetworkType with SetIPFamily.

var (
	sourceName  string       // what --interface named: an interface, or an address
	sourceAddrs []netip.Addr // the addresses to send from; nil without --interface
)

// SystemInterfaces is where the interfaces come from: the operating system's
// table. A variable so tests can describe interfaces of their own.
var SystemInterfaces = net.Interfaces

// NoSuchInterface is what shint says about an interface name that does not
// exist: the name, and the ones that do. On Windows and Solaris the names are
// long and easy to get wrong ("Ethernet 2", "Wi-Fi"), so the list is the help.
// The name is what the user typed, so it is printed escaped.
func NoSuchInterface(name string, available []string) string {
	return fmt.Sprintf("no such interface %s (available: %s)", printable(name), strings.Join(available, ", "))
}

// printable writes s with anything that is not printable escaped, so a name
// typed with control characters cannot act on the terminal.
func printable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, "\\x%02x", r)
		}
	}
	return b.String()
}

// SetSourceInterface applies --interface: value is an interface name (any
// case) or an address this machine has. An empty value turns it off. The error
// says plainly what is wrong: no such interface (and which there are), an
// address this machine does not have, or an interface with no address to send
// from.
func SetSourceInterface(value string) error {
	sourceName, sourceAddrs = "", nil
	if value == "" {
		return nil
	}
	list, err := SystemInterfaces()
	if err != nil {
		return fmt.Errorf("could not read the interface table: %w", err)
	}
	if a, err := netip.ParseAddr(value); err == nil {
		a = a.Unmap()
		for _, ifc := range list {
			for _, have := range interfaceAddrs(ifc) {
				if have.WithZone("") == a.WithZone("") {
					sourceName, sourceAddrs = value, []netip.Addr{have}
					return nil
				}
			}
		}
		return fmt.Errorf("this machine has no address %s (shint ip lists its addresses)", a)
	}
	names := make([]string, 0, len(list))
	for _, ifc := range list {
		names = append(names, ifc.Name)
		if strings.EqualFold(ifc.Name, value) {
			addrs := interfaceAddrs(ifc)
			if len(addrs) == 0 {
				return fmt.Errorf("interface %s has no address to send from (is it up?)", ifc.Name)
			}
			sourceName, sourceAddrs = ifc.Name, addrs
			return nil
		}
	}
	return fmt.Errorf("%s", NoSuchInterface(value, names))
}

// interfaceAddrs are an interface's IP addresses, a link-local IPv6 one with
// the interface as its zone.
func interfaceAddrs(ifc net.Interface) []netip.Addr {
	raw, err := ifc.Addrs()
	if err != nil {
		return nil
	}
	var out []netip.Addr
	for _, x := range raw {
		ipnet, ok := x.(*net.IPNet)
		if !ok {
			continue
		}
		a, ok := netip.AddrFromSlice(ipnet.IP)
		if !ok {
			continue
		}
		a = a.Unmap()
		if a.Is6() && a.IsLinkLocalUnicast() {
			a = a.WithZone(ifc.Name)
		}
		out = append(out, a)
	}
	return out
}

// SourceLabel is what --interface named, for input_params ("" without it).
func SourceLabel() string { return sourceName }

// SourceInterface is what --interface named ("" without it), and the
// addresses traffic leaves from.
func SourceInterface() (string, []netip.Addr) {
	return sourceName, sourceAddrs
}

// sourceFor picks the address to send from for a destination: one of its
// family, link-local for a link-local destination and otherwise not.
// wantFamily is 4 or 6 when the destination is a name the dialer resolves
// itself (the dialer then keeps to the family of the address picked here), 0
// for "IPv4 if the interface has an IPv4 address".
func sourceFor(dst netip.Addr, wantFamily int) (netip.Addr, error) {
	is4 := wantFamily == 4 || (wantFamily == 0 && dst.IsValid() && dst.Is4())
	if !dst.IsValid() && wantFamily == 0 {
		is4 = false
		for _, a := range sourceAddrs {
			if a.Is4() {
				is4 = true
			}
		}
	}
	linkLocal := dst.IsValid() && dst.Is6() && dst.IsLinkLocalUnicast()
	var fallback netip.Addr
	for _, a := range sourceAddrs {
		if a.Is4() != is4 {
			continue
		}
		if is4 || a.IsLinkLocalUnicast() == linkLocal {
			return a, nil
		}
		if !fallback.IsValid() {
			fallback = a
		}
	}
	if fallback.IsValid() {
		return fallback, nil
	}
	family := "IPv6"
	if is4 {
		family = "IPv4"
	}
	return netip.Addr{}, fmt.Errorf("--interface %s has no %s address to send from", sourceName, family)
}

// BindSource sets a dialer's source address for --interface, given the
// network ("tcp", "udp6", ...) and the address it will dial ("host:port").
// Without --interface it does nothing.
func BindSource(d *net.Dialer, network, address string) error {
	if sourceAddrs == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	dst, _ := netip.ParseAddr(host)
	want := 0
	switch {
	case strings.HasSuffix(network, "4"):
		want = 4
	case strings.HasSuffix(network, "6"):
		want = 6
	}
	src, err := sourceFor(dst.Unmap(), want)
	if err != nil {
		return err
	}
	if strings.HasPrefix(network, "udp") {
		d.LocalAddr = net.UDPAddrFromAddrPort(netip.AddrPortFrom(src, 0))
	} else {
		d.LocalAddr = net.TCPAddrFromAddrPort(netip.AddrPortFrom(src, 0))
	}
	return nil
}
