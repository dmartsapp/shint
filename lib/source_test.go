package lib

import (
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
)

// loopbackName is the name of this machine's loopback interface (lo, lo0,
// "Loopback Pseudo-Interface 1"), which every machine has.
func loopbackName(t *testing.T) string {
	t.Helper()
	list, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, ifc := range list {
		if ifc.Flags&net.FlagLoopback != 0 {
			return ifc.Name
		}
	}
	t.Skip("no loopback interface")
	return ""
}

func TestSetSourceInterface(t *testing.T) {
	defer func() { _ = SetSourceInterface("") }()
	lo := loopbackName(t)
	for _, value := range []string{lo, strings.ToUpper(lo), "127.0.0.1"} {
		if err := SetSourceInterface(value); err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		name, addrs := SourceInterface()
		if !slicesHas(addrs, netip.MustParseAddr("127.0.0.1")) || (value != "127.0.0.1" && name != lo) || SourceLabel() != name {
			t.Errorf("%q: %s %v", value, name, addrs)
		}
	}

	for value, says := range map[string]string{
		"no-such-iface0": "no such interface no-such-iface0 (available: ",
		"192.0.2.99":     "this machine has no address 192.0.2.99",
		"bad\x1b[2J":     `no such interface bad\x1b[2J (available:`,
	} {
		err := SetSourceInterface(value)
		if err == nil || !strings.Contains(err.Error(), says) || strings.Contains(err.Error(), "\x1b") {
			t.Errorf("%q: %v, want %q", value, err, says)
		}
		if name, addrs := SourceInterface(); name != "" || addrs != nil {
			t.Errorf("%q: a failed --interface must leave none set: %s %v", value, name, addrs)
		}
	}
	if err := SetSourceInterface("no-such-iface0"); !strings.Contains(err.Error(), lo) {
		t.Errorf("the list should name %s: %v", lo, err)
	}

	orig := SystemInterfaces
	SystemInterfaces = func() ([]net.Interface, error) { return nil, errors.New("no table") }
	defer func() { SystemInterfaces = orig }()
	if err := SetSourceInterface(lo); err == nil || !strings.Contains(err.Error(), "could not read the interface table") {
		t.Errorf("unreadable table: %v", err)
	}
}

// On Solaris and illumos an interface's addresses may sit on its logical
// interfaces (net0:1 held the global IPv6 address on Solaris 11.4):
// --interface net0 takes those too, and net0:1 on its own still works.
func TestSourceInterfaceTakesLogicalInterfaces(t *testing.T) {
	defer func() { _ = SetSourceInterface("") }()
	origList, origAddrs := SystemInterfaces, addrsOf
	defer func() { SystemInterfaces, addrsOf = origList, origAddrs }()
	table := map[string][]string{
		"lo0":    {"127.0.0.1/8"},
		"net0":   {"192.168.1.127/24", "fe80::11:32ff:fe24:e691/10"},
		"net0:1": {"2001:db8:1::127/64"},
		"net0:a": {"192.0.2.1/24"},    // not a logical interface's name
		"net01":  {"192.0.2.2/24"},    // another interface
		"net1:1": {"198.51.100.1/24"}, // another interface's logical one
	}
	SystemInterfaces = func() ([]net.Interface, error) {
		var out []net.Interface
		for i, n := range []string{"lo0", "net0", "net0:1", "net0:a", "net01", "net1:1"} {
			out = append(out, net.Interface{Index: i + 1, Name: n, Flags: net.FlagUp})
		}
		return out, nil
	}
	addrsOf = func(ifc net.Interface) ([]net.Addr, error) {
		var out []net.Addr
		for _, c := range table[ifc.Name] {
			ip, n, _ := net.ParseCIDR(c)
			n.IP = ip
			out = append(out, n)
		}
		return out, nil
	}
	for value, want := range map[string]string{
		"net0":   "192.168.1.127 fe80::11:32ff:fe24:e691%net0 2001:db8:1::127",
		"NET0":   "192.168.1.127 fe80::11:32ff:fe24:e691%net0 2001:db8:1::127",
		"net0:1": "2001:db8:1::127",
		"net01":  "192.0.2.2",
	} {
		if err := SetSourceInterface(value); err != nil {
			t.Fatalf("%s: %v", value, err)
		}
		_, addrs := SourceInterface()
		var got []string
		for _, a := range addrs {
			got = append(got, a.String())
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%s: %v, want %s", value, got, want)
		}
	}
}

func slicesHas(addrs []netip.Addr, want netip.Addr) bool {
	for _, a := range addrs {
		if a == want {
			return true
		}
	}
	return false
}

// The source matches the destination's family, link-local for link-local.
func TestSourceFor(t *testing.T) {
	defer func() { sourceName, sourceAddrs = "", nil }()
	sourceName = "en0"
	sourceAddrs = []netip.Addr{netip.MustParseAddr("fe80::1%en0"), netip.MustParseAddr("2001:db8::20"), netip.MustParseAddr("192.168.1.20")}
	for _, c := range []struct {
		dst  string
		want int
		src  string
	}{
		{"8.8.8.8", 0, "192.168.1.20"},
		{"2001:4860::8888", 0, "2001:db8::20"},
		{"fe80::99%en0", 0, "fe80::1%en0"},
		{"", 0, "192.168.1.20"}, // a name: IPv4 when the interface has it
		{"", 6, "2001:db8::20"},
		{"", 4, "192.168.1.20"},
	} {
		var dst netip.Addr
		if c.dst != "" {
			dst = netip.MustParseAddr(c.dst)
		}
		got, err := sourceFor(dst, c.want)
		if err != nil || got.String() != c.src {
			t.Errorf("sourceFor(%q, %d) = %v %v, want %s", c.dst, c.want, got, err, c.src)
		}
	}
	sourceAddrs = []netip.Addr{netip.MustParseAddr("192.168.1.20")}
	if _, err := sourceFor(netip.MustParseAddr("2001:4860::8888"), 0); err == nil || err.Error() != "--interface en0 has no IPv6 address to send from" {
		t.Errorf("no IPv6: %v", err)
	}
	if got, _ := sourceFor(netip.Addr{}, 0); got.String() != "192.168.1.20" {
		t.Errorf("a name, IPv4 only: %v", got)
	}
}

func TestBindSource(t *testing.T) {
	defer func() { sourceName, sourceAddrs = "", nil }()
	var d net.Dialer
	if err := BindSource(&d, "tcp", "127.0.0.1:80"); err != nil || d.LocalAddr != nil {
		t.Errorf("without --interface nothing is bound: %v %v", d.LocalAddr, err)
	}
	sourceName = "lo"
	sourceAddrs = []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}
	if err := BindSource(&d, "tcp", "127.0.0.1:80"); err != nil || d.LocalAddr.String() != "127.0.0.1:0" || d.LocalAddr.Network() != "tcp" {
		t.Errorf("tcp: %v %v", d.LocalAddr, err)
	}
	if err := BindSource(&d, "udp6", "example.com:53"); err != nil || d.LocalAddr.String() != "[::1]:0" || d.LocalAddr.Network() != "udp" {
		t.Errorf("udp6, a name: %v %v", d.LocalAddr, err)
	}
}
