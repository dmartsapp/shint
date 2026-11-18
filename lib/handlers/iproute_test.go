package handlers

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// procRoute is a /proc/net/route as a little-endian kernel writes it: a default
// route through 192.168.1.254, the network itself, a route that is not up, and
// an unreachable one.
const procRoute = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	FE01A8C0	0003	0	0	100	00000000	0	0	0
eth0	0001A8C0	00000000	0001	0	0	100	00FFFFFF	0	0	0
eth1	0000000A	00000000	0000	0	0	0	000000FF	0	0	0
*	0000A8C0	00000000	0201	0	0	0	0000FFFF	0	0	0
`

// procIPv6Route holds what ip -6 route shows - a default route through a
// link-local router and an on-link /64 - and what it does not: a local address,
// a cached clone, multicast, and the kernel's null route.
const procIPv6Route = `00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000000000000000001 00000400 00000001 00000000 00000003 eth0
20010db8000000010000000000000000 40 00000000000000000000000000000000 00 00000000000000000000000000000000 00000100 00000001 00000000 00000001 eth0
20010db8000000010000000000000005 80 00000000000000000000000000000000 00 00000000000000000000000000000000 00000000 00000002 00000000 80200001 eth0
20010db8000000020000000000000007 80 00000000000000000000000000000000 00 fe800000000000000000000000000001 00000000 00000000 00000000 01000003 eth0
ff000000000000000000000000000000 08 00000000000000000000000000000000 00 00000000000000000000000000000000 00000100 00000004 00000000 00000001 eth0
00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 ffffffff 00000001 00000000 00200200 lo
`

func TestParseProcRoute(t *testing.T) {
	rs, err := parseProcRoute(strings.NewReader(procRoute), binary.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 3 {
		t.Fatalf("%d routes, want 3 (one is not up): %+v", len(rs), rs)
	}
	if d := rs[0]; d.Prefix.String() != "0.0.0.0/0" || d.Gateway.String() != "192.168.1.254" || d.Iface != "eth0" || d.Metric != 100 || !d.HasMetric {
		t.Errorf("default %+v", d)
	}
	if n := rs[1]; n.Prefix.String() != "192.168.1.0/24" || n.Gateway.IsValid() || n.Reject {
		t.Errorf("network %+v", n)
	}
	if u := rs[2]; u.Prefix.String() != "192.168.0.0/16" || !u.Reject || u.Iface != "" {
		t.Errorf("unreachable %+v", u)
	}
	// A big-endian kernel writes the same table with the words the other way round.
	be := "Iface Destination Gateway Flags RefCnt Use Metric Mask\neth0 00000000 C0A801FE 0003 0 0 100 00000000\n"
	if rs, err := parseProcRoute(strings.NewReader(be), binary.BigEndian); err != nil || rs[0].Gateway.String() != "192.168.1.254" {
		t.Errorf("big-endian: %v %+v", err, rs)
	}
	for name, table := range map[string]string{
		"no header":   "eth0 00000000 FE01A8C0 0003 0 0 100 00000000\n",
		"short line":  "Iface\neth0 00000000\n",
		"bad address": "Iface\neth0 0000000 FE01A8C0 0003 0 0 100 00000000\n",
		"bad mask":    "Iface\neth0 00000000 FE01A8C0 0003 0 0 100 FF00FF00\n",
		"bad metric":  "Iface\neth0 00000000 FE01A8C0 0003 0 0 x 00000000\n",
		"bad flags":   "Iface\neth0 00000000 FE01A8C0 zz 0 0 100 00000000\n",
	} {
		if _, err := parseProcRoute(strings.NewReader(table), binary.LittleEndian); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestParseProcIPv6Route(t *testing.T) {
	rs, err := parseProcIPv6Route(strings.NewReader(procIPv6Route))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("%d routes, want 2: %+v", len(rs), rs)
	}
	if d := rs[0]; d.Prefix.String() != "::/0" || d.Gateway.String() != "fe80::1%eth0" || d.Metric != 1024 {
		t.Errorf("default %+v", d)
	}
	if n := rs[1]; n.Prefix.String() != "2001:db8:0:1::/64" || n.Gateway.IsValid() || n.Metric != 256 {
		t.Errorf("network %+v", n)
	}
	for name, line := range map[string]string{
		"nine fields":   "00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 00000400 00000001 00000000 00000003\n",
		"bad address":   "0000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 00000400 00000001 00000000 00000003 eth0\n",
		"prefix of 129": "00000000000000000000000000000000 81 00000000000000000000000000000000 00 00000000000000000000000000000000 00000400 00000001 00000000 00000003 eth0\n",
	} {
		if _, err := parseProcIPv6Route(strings.NewReader(line)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestWinSockaddr(t *testing.T) {
	var v4, v6 [24]byte
	copy(v4[:], []byte{192, 168, 1, 254})
	a, _, ok := winSockaddr(2, v4)
	if !ok || a.String() != "192.168.1.254" {
		t.Errorf("AF_INET: %v %v", a, ok)
	}
	ll := netip.MustParseAddr("fe80::1").As16()
	copy(v6[4:20], ll[:])
	binary.LittleEndian.PutUint32(v6[20:24], 12)
	a, scope, ok := winSockaddr(23, v6)
	if !ok || a.String() != "fe80::1" || scope != 12 {
		t.Errorf("AF_INET6: %v %d %v", a, scope, ok)
	}
	if _, _, ok := winSockaddr(17, v4); ok {
		t.Error("another family: want not ok")
	}
}

func withRoutes(t *testing.T, routes []sysRoute, err error) {
	t.Helper()
	orig := readRoutes
	readRoutes = func() ([]sysRoute, error) { return routes, err }
	t.Cleanup(func() { readRoutes = orig })
}

func withKernelRoute(t *testing.T, iface, source string, err error) {
	t.Helper()
	orig := kernelRoute
	kernelRoute = func(dst netip.Addr) (string, netip.Addr, error) {
		if err != nil {
			return "", netip.Addr{}, err
		}
		return iface, netip.MustParseAddr(source), nil
	}
	t.Cleanup(func() { kernelRoute = orig })
}

func rt(prefix, gateway, iface string, metric int, scoped bool) sysRoute {
	r := sysRoute{Prefix: netip.MustParsePrefix(prefix), Iface: iface, Metric: metric, HasMetric: metric >= 0, Scoped: scoped}
	if gateway != "" {
		r.Gateway = netip.MustParseAddr(gateway)
	}
	return r
}

func routeTable() []sysRoute {
	return []sysRoute{
		rt("192.168.1.0/24", "", "en0", 100, false),
		rt("fe80::/64", "", "en0", -1, false),
		rt("0.0.0.0/0", "", "utun4", -1, true),
		rt("::/0", "fe80::1%en0", "en0", -1, false),
		rt("10.8.0.0/16", "192.168.1.1", "en0", 50, false),
		rt("0.0.0.0/0", "192.168.1.254", "en0", 100, false),
		{Prefix: netip.MustParsePrefix("10.66.0.0/16"), Reject: true},
	}
}

func TestIPRouteList(t *testing.T) {
	withRoutes(t, routeTable(), nil)
	asText := false
	out := captureStdout(t, func() {
		if !IPRouteHandler(&asText, "") {
			t.Error("want ok")
		}
	})
	want := []string{
		"route destination=default family=ipv4 gateway=192.168.1.254 interface=en0 metric=100\n",
		"route destination=default family=ipv4 interface=utun4 scoped=true\n",
		"route destination=10.8.0.0/16 family=ipv4 gateway=192.168.1.1 interface=en0 metric=50\n",
		"route destination=10.66.0.0/16 family=ipv4 unreachable=true\n",
		"route destination=192.168.1.0/24 family=ipv4 interface=en0 metric=100\n",
		"route destination=default family=ipv6 gateway=fe80::1%en0 interface=en0\n",
		"route destination=fe80::/64 family=ipv6 interface=en0\n",
		"done routes=7 ipv4=5 ipv6=2 total_time=",
	}
	last := -1
	for _, w := range want {
		i := strings.Index(out, w)
		if i < 0 || i < last {
			t.Errorf("want %q, in this order, in:\n%s", w, out)
		}
		last = i
	}

	asJSON := true
	useFamily(t, false, true)
	out = captureStdout(t, func() { IPRouteHandler(&asJSON, "") })
	for _, w := range []string{`"module_name": "ip-route"`, `"destination": "::/0"`, `"default": true`, `"gateway": "fe80::1%en0"`} {
		if !strings.Contains(out, w) {
			t.Errorf("-6 --json: want %s in:\n%s", w, out)
		}
	}
	if strings.Contains(out, `"ipv4"`) || strings.Contains(out, `"metric"`) {
		t.Errorf("-6: no IPv4 route, and no metric where none was given:\n%s", out)
	}
}

func TestIPRouteLookup(t *testing.T) {
	withRoutes(t, routeTable(), nil)
	asText := false
	// The kernel picks en0: the default through 192.168.1.254 matches, not
	// utun4's scoped default.
	withKernelRoute(t, "en0", "192.168.1.20", nil)
	out := captureStdout(t, func() { IPRouteHandler(&asText, "8.8.8.8") })
	if !strings.Contains(out, "route destination=8.8.8.8 route=0.0.0.0/0 gateway=192.168.1.254 interface=en0 source=192.168.1.20\n") {
		t.Errorf("default:\n%s", out)
	}
	// The longest prefix wins.
	out = captureStdout(t, func() { IPRouteHandler(&asText, "10.8.3.4") })
	if !strings.Contains(out, "route=10.8.0.0/16 gateway=192.168.1.1 interface=en0") {
		t.Errorf("longest prefix:\n%s", out)
	}
	// On a directly reached network there is no gateway.
	out = captureStdout(t, func() { IPRouteHandler(&asText, "192.168.1.77") })
	if !strings.Contains(out, "route destination=192.168.1.77 route=192.168.1.0/24 interface=en0 source=192.168.1.20\n") || strings.Contains(out, "gateway=") {
		t.Errorf("on-link:\n%s", out)
	}
	// The kernel picked utun4: its scoped default is the route.
	withKernelRoute(t, "utun4", "100.96.70.128", nil)
	out = captureStdout(t, func() { IPRouteHandler(&asText, "1.1.1.1") })
	if !strings.Contains(out, "route=0.0.0.0/0 interface=utun4 source=100.96.70.128") {
		t.Errorf("scoped:\n%s", out)
	}

	// The system sends from an interface no listed route covers (Linux's
	// loopback routes live in the local table): no route, and no gateway taken
	// from another interface's route.
	withKernelRoute(t, "lo", "127.0.0.1", nil)
	out = captureStdout(t, func() { IPRouteHandler(&asText, "127.0.0.1") })
	if !strings.Contains(out, "route destination=127.0.0.1 interface=lo source=127.0.0.1\n") || strings.Contains(out, "gateway=") || strings.Contains(out, "route=") {
		t.Errorf("local:\n%s", out)
	}

	withKernelRoute(t, "", "", errors.New("dial udp 10.66.1.1:9: connect: network is unreachable"))
	asJSON := true
	out = captureStdout(t, func() {
		if IPRouteHandler(&asJSON, "10.66.1.1") {
			t.Error("no route: want a failure")
		}
	})
	if !strings.Contains(out, `"error": "no route to 10.66.1.1: network is unreachable"`) || !strings.Contains(out, `"stats": []`) {
		t.Errorf("unreachable:\n%s", out)
	}
}

func TestIPRouteNotReadable(t *testing.T) {
	withRoutes(t, nil, errors.New("ip route is not supported on plan9 yet"))
	asText := false
	out := captureStdout(t, func() {
		if IPRouteHandler(&asText, "") {
			t.Error("want a failure")
		}
	})
	if !strings.Contains(out, `[ip-route] ERROR failed error="ip route is not supported on plan9 yet"`) {
		t.Errorf("%s", out)
	}
	withRoutes(t, nil, nil)
	out = captureStdout(t, func() { IPRouteHandler(&asText, "") })
	if !strings.Contains(out, "this machine reports no routes") {
		t.Errorf("empty table:\n%s", out)
	}
}

func TestMatchRoutePrefersUnscopedThenMetric(t *testing.T) {
	rs := []sysRoute{rt("0.0.0.0/0", "10.0.0.2", "en0", 200, false), rt("0.0.0.0/0", "10.0.0.3", "en0", 100, true), rt("0.0.0.0/0", "10.0.0.1", "en0", 100, false)}
	r, ok := matchRoute(rs, netip.MustParseAddr("8.8.8.8"), "en0")
	if !ok || r.Gateway.String() != "10.0.0.1" {
		t.Errorf("got %+v", r)
	}
	if _, ok := matchRoute(nil, netip.MustParseAddr("8.8.8.8"), ""); ok {
		t.Error("no routes: no match")
	}
}
