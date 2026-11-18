//go:build darwin

package handlers

import (
	"testing"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// bsdRoute keeps the routes netstat -rn shows as routes, and leaves out the
// ARP/NDP entries, cloned host routes, multicast and broadcast.
func TestBSDRoute(t *testing.T) {
	v4 := func(a, b, c, d byte) *route.Inet4Addr { return &route.Inet4Addr{IP: [4]byte{a, b, c, d}} }
	link := &route.LinkAddr{Index: 14}
	mac := &route.LinkAddr{Index: 14, Addr: []byte{0x60, 0xd8, 0xa4, 0x35, 0x8b, 0x04}}
	addrs := func(dst, gw, mask route.Addr) []route.Addr { return []route.Addr{dst, gw, mask} }
	up, gw, host := unix.RTF_UP, unix.RTF_GATEWAY, unix.RTF_HOST
	for _, c := range []struct {
		name  string
		flags int
		addrs []route.Addr
		want  string // the destination, gateway and scope, or "" for left out
	}{
		{"default", up | gw | unix.RTF_STATIC, addrs(v4(0, 0, 0, 0), v4(192, 168, 1, 254), v4(0, 0, 0, 0)), "0.0.0.0/0 192.168.1.254 false"},
		{"scoped default", up | unix.RTF_STATIC | unix.RTF_IFSCOPE, addrs(v4(0, 0, 0, 0), link, v4(0, 0, 0, 0)), "0.0.0.0/0 invalid IP true"},
		{"network", up | unix.RTF_CLONING, addrs(v4(192, 168, 1, 0), link, v4(255, 255, 255, 0)), "192.168.1.0/24 invalid IP false"},
		{"short mask", up, addrs(v4(10, 0, 0, 0), link, &route.Inet4Addr{IP: [4]byte{255}}), "10.0.0.0/8 invalid IP false"},
		{"host", up | host, addrs(v4(127, 0, 0, 1), v4(127, 0, 0, 1), nil), "127.0.0.1/32 invalid IP false"},
		{"ARP entry", up | host | unix.RTF_LLINFO, addrs(v4(192, 168, 1, 42), mac, nil), ""},
		{"cloned", up | host | unix.RTF_WASCLONED, addrs(v4(8, 8, 8, 8), link, nil), ""},
		{"multicast", up, addrs(v4(224, 0, 0, 0), link, v4(240, 0, 0, 0)), ""},
		{"broadcast", up | host, addrs(v4(255, 255, 255, 255), link, nil), ""},
		{"down", gw, addrs(v4(0, 0, 0, 0), v4(192, 168, 1, 254), v4(0, 0, 0, 0)), ""},
	} {
		r, ok := bsdRoute(c.flags, c.addrs, "en0")
		got := ""
		if ok {
			got = r.Prefix.String() + " " + r.Gateway.String() + " " + map[bool]string{true: "true", false: "false"}[r.Scoped]
		}
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
