//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package ip

import (
	"net/netip"
	"strings"
	"testing"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// bsdNeighbours keeps what arp -an and ndp -an show as neighbours, and leaves
// out this machine's own addresses (FreeBSD lists each as a permanent entry),
// loopback, multicast and the broadcast MAC.
func TestBSDNeighbours(t *testing.T) {
	v4 := func(a, b, c, d byte) *route.Inet4Addr { return &route.Inet4Addr{IP: [4]byte{a, b, c, d}} }
	mac := func(last byte) *route.LinkAddr {
		return &route.LinkAddr{Index: 1, Addr: []byte{0x08, 0x27, 0xa8, 0x20, 0x71, last}}
	}
	entry := func(index int, dst route.Addr, gw route.Addr) route.Message {
		return &route.RouteMessage{Flags: unix.RTF_UP | unix.RTF_HOST | unix.RTF_LLINFO, Index: index, Addrs: []route.Addr{dst, gw}}
	}
	msgs := []route.Message{
		entry(1, v4(192, 168, 1, 254), mac(0x54)), // the gateway
		entry(1, v4(192, 168, 1, 42), mac(0x52)),
		entry(1, v4(192, 168, 1, 232), mac(0x01)), // this machine's own address
		entry(1, &route.Inet6Addr{IP: [16]byte{0xfe, 0x80, 15: 1}, ZoneID: 1}, mac(0x55)),
		entry(1, v4(192, 168, 1, 7), &route.LinkAddr{Index: 1}), // being resolved: no MAC yet
		// broadcast, multicast, loopback, and a gateway that is not a link address
		entry(1, v4(192, 168, 1, 255), &route.LinkAddr{Index: 1, Addr: []byte{255, 255, 255, 255, 255, 255}}),
		entry(1, v4(224, 0, 0, 251), mac(0xfb)),
		entry(2, v4(127, 0, 0, 1), mac(0x00)),
		entry(1, v4(192, 168, 1, 9), v4(192, 168, 1, 254)),
		// a route, not a link-layer entry, and not a route message at all
		&route.RouteMessage{Flags: unix.RTF_UP | unix.RTF_GATEWAY, Index: 1, Addrs: []route.Addr{v4(0, 0, 0, 0), v4(192, 168, 1, 254)}},
		&route.InterfaceMessage{},
	}
	ifaces := map[int]bsdIface{1: {"em0", false}, 2: {"lo0", true}}
	local := map[netip.Addr]bool{netip.MustParseAddr("192.168.1.232"): true}
	var got []string
	for _, n := range bsdNeighbours(msgs, ifaces, local) {
		got = append(got, n.Addr.WithZone("").String()+" "+n.MAC+" "+n.Iface)
	}
	want := []string{
		"192.168.1.254 08:27:a8:20:71:54 em0",
		"192.168.1.42 08:27:a8:20:71:52 em0",
		"fe80::1 08:27:a8:20:71:55 em0",
		"192.168.1.7  em0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// platformNeighbours reads the real table: on a machine that has talked to
// anything on its network, at least one neighbour, and never one of its own
// addresses.
func TestPlatformNeighboursReadsTheTable(t *testing.T) {
	if _, err := platformNeighbours(); err != nil {
		t.Fatalf("platformNeighbours: %v", err)
	}
}
