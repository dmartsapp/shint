package handlers

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
)

// ndMsg builds a netlink neighbour message's data the way the kernel lays it
// out: struct ndmsg, then each attribute padded to four bytes.
func ndMsg(family byte, ifindex int32, state uint16, flags byte, addr, mac []byte) []byte {
	b := []byte{family, 0, 0, 0}
	b = binary.LittleEndian.AppendUint32(b, uint32(ifindex))
	b = binary.LittleEndian.AppendUint16(b, state)
	b = append(b, flags, 0)
	attr := func(typ uint16, val []byte) {
		if val == nil {
			return
		}
		b = binary.LittleEndian.AppendUint16(b, uint16(4+len(val)))
		b = binary.LittleEndian.AppendUint16(b, typ)
		b = append(b, val...)
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
	}
	attr(ndaDst, addr)
	attr(ndaLLAddr, mac)
	return b
}

func TestParseNdMsg(t *testing.T) {
	names := map[int]string{2: "eth0"}
	mac := []byte{0x60, 0xd8, 0xa4, 0x35, 0x8b, 0x04}
	ll := netip.MustParseAddr("fe80::62d8:a4ff:fe35:8b04").As16()
	for _, c := range []struct {
		name string
		data []byte
		ok   bool
		want sysNeighbour
	}{
		{"reachable", ndMsg(2, 2, 0x02, 0, []byte{192, 168, 1, 254}, mac), true,
			sysNeighbour{Addr: netip.MustParseAddr("192.168.1.254"), MAC: "60:d8:a4:35:8b:04", Iface: "eth0", State: "REACHABLE"}},
		{"failed, no MAC", ndMsg(2, 2, 0x20, 0, []byte{192, 168, 1, 77}, nil), true,
			sysNeighbour{Addr: netip.MustParseAddr("192.168.1.77"), Iface: "eth0", State: "FAILED"}},
		{"IPv6 router", ndMsg(10, 2, 0x04, 0x80, ll[:], mac), true,
			sysNeighbour{Addr: netip.MustParseAddr("fe80::62d8:a4ff:fe35:8b04%eth0"), MAC: "60:d8:a4:35:8b:04", Iface: "eth0", State: "STALE", Router: true}},
		{"NOARP", ndMsg(2, 2, 0x40, 0, []byte{127, 0, 0, 1}, mac), false, sysNeighbour{}},
		{"multicast", ndMsg(2, 2, 0x80, 0, []byte{224, 0, 0, 251}, mac), false, sysNeighbour{}},
		{"broadcast MAC", ndMsg(2, 2, 0x80, 0, []byte{192, 168, 1, 255}, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}), false, sysNeighbour{}},
		{"bridge family", ndMsg(7, 2, 0x02, 0, nil, mac), false, sysNeighbour{}},
	} {
		got, ok, err := parseNdMsg(c.data, binary.LittleEndian, names)
		if err != nil || ok != c.ok || (ok && got != c.want) {
			t.Errorf("%s: %+v %v %v, want %+v %v", c.name, got, ok, err, c.want, c.ok)
		}
	}
	bad := ndMsg(2, 2, 0x02, 0, []byte{192, 168, 1, 1}, nil)
	binary.LittleEndian.PutUint16(bad[12:14], 200) // an attribute longer than the message
	if _, _, err := parseNdMsg(bad, binary.LittleEndian, names); err == nil {
		t.Error("an attribute past the end: want an error")
	}
	if _, _, err := parseNdMsg([]byte{2, 0, 0}, binary.LittleEndian, names); err == nil {
		t.Error("a short header: want an error")
	}
}

// buildWinNeighbourRow builds one MIB_IPNET_ROW2.
func buildWinNeighbourRow(family uint16, addr []byte, ifindex uint32, mac []byte, state uint32, router bool) []byte {
	r := make([]byte, winNeighbourRow)
	binary.LittleEndian.PutUint16(r[0:2], family)
	if family == 2 {
		copy(r[4:8], addr)
	} else {
		copy(r[8:24], addr)
	}
	binary.LittleEndian.PutUint32(r[28:32], ifindex)
	copy(r[40:72], mac)
	binary.LittleEndian.PutUint32(r[72:76], uint32(len(mac)))
	binary.LittleEndian.PutUint32(r[76:80], state)
	if router {
		r[80] = 1
	}
	return r
}

func TestParseWinNeighbourTable(t *testing.T) {
	mac := []byte{0x60, 0xd8, 0xa4, 0x35, 0x8b, 0x04}
	ll := netip.MustParseAddr("fe80::1").As16()
	rows := [][]byte{
		buildWinNeighbourRow(2, []byte{192, 168, 1, 254}, 7, mac, 5, false),
		buildWinNeighbourRow(2, []byte{192, 168, 1, 9}, 7, []byte{0, 0, 0, 0, 0, 0}, 0, false),
		buildWinNeighbourRow(23, ll[:], 7, mac, 4, true),
		buildWinNeighbourRow(2, []byte{224, 0, 0, 22}, 7, []byte{1, 0, 0x5e, 0, 0, 0x16}, 6, false),
		buildWinNeighbourRow(2, []byte{255, 255, 255, 255}, 7, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 6, false),
	}
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(rows)))
	b = append(b, 0, 0, 0, 0)
	for _, r := range rows {
		b = append(b, r...)
	}
	got, err := parseWinNeighbourTable(b, map[uint32]string{7: "Ethernet 2"})
	if err != nil || len(got) != 3 {
		t.Fatalf("%v %+v", err, got)
	}
	if got[0] != (sysNeighbour{Addr: netip.MustParseAddr("192.168.1.254"), MAC: "60:d8:a4:35:8b:04", Iface: "Ethernet 2", State: "REACHABLE"}) {
		t.Errorf("reachable %+v", got[0])
	}
	if got[1].MAC != "" || got[1].State != "UNREACHABLE" {
		t.Errorf("unresolved %+v", got[1])
	}
	if got[2].Addr.String() != "fe80::1%Ethernet 2" || !got[2].Router || got[2].State != "STALE" {
		t.Errorf("router %+v", got[2])
	}
	if _, err := parseWinNeighbourTable(b[:100], nil); err == nil || !strings.Contains(err.Error(), "says it has 5 rows") {
		t.Errorf("truncated: %v", err)
	}
	if _, err := parseWinNeighbourTable([]byte{1}, nil); err == nil {
		t.Error("no header: want an error")
	}
}

func withNeighbours(t *testing.T, ns []sysNeighbour, err error) {
	t.Helper()
	orig := readNeighbours
	readNeighbours = func() ([]sysNeighbour, error) { return ns, err }
	t.Cleanup(func() { readNeighbours = orig })
}

func neighbourTable() []sysNeighbour {
	return []sysNeighbour{
		{Addr: netip.MustParseAddr("fe80::1%en0"), MAC: "60:d8:a4:35:8b:04", Iface: "en0", State: "STALE", Router: true},
		{Addr: netip.MustParseAddr("192.168.1.254"), MAC: "60:d8:a4:35:8b:04", Iface: "en0", State: "REACHABLE"},
		{Addr: netip.MustParseAddr("192.168.1.77"), Iface: "en0", State: "FAILED"},
		{Addr: netip.MustParseAddr("10.0.0.5"), MAC: "02:42:ac:11:00:02", Iface: "docker0"},
	}
}

func TestIPArp(t *testing.T) {
	withNeighbours(t, neighbourTable(), nil)
	withRoutes(t, []sysRoute{rt("0.0.0.0/0", "192.168.1.254", "en0", 0, false), rt("::/0", "fe80::1%en0", "en0", 0, false)}, nil)
	fakeInterfaces(t, []ifaceInfo{{Name: "lo0"}, {Name: "en0"}, {Name: "docker0"}}, nil)
	asText := false
	out := captureStdout(t, func() {
		if !IPArpHandler(&asText, "") {
			t.Error("want ok")
		}
	})
	want := []string{
		"neighbour address=10.0.0.5 family=ipv4 mac=02:42:ac:11:00:02 interface=docker0\n",
		"neighbour address=192.168.1.77 family=ipv4 interface=en0 state=FAILED\n",
		"neighbour address=192.168.1.254 family=ipv4 mac=60:d8:a4:35:8b:04 interface=en0 state=REACHABLE gateway=true\n",
		"neighbour address=fe80::1%en0 family=ipv6 mac=60:d8:a4:35:8b:04 interface=en0 state=STALE router=true gateway=true\n",
		"done neighbours=4 ipv4=3 ipv6=1 total_time=",
	}
	last := -1
	for _, w := range want {
		i := strings.Index(out, w)
		if i < 0 || i < last {
			t.Errorf("want %q, in this order, in:\n%s", w, out)
		}
		last = i
	}

	// One interface, one family, JSON.
	asJSON := true
	useFamily(t, true, false)
	out = captureStdout(t, func() { IPArpHandler(&asJSON, "EN0") })
	for _, w := range []string{`"module_name": "ip-arp"`, `"address": "192.168.1.254"`, `"gateway": true`, `"state": "FAILED"`} {
		if !strings.Contains(out, w) {
			t.Errorf("want %s in:\n%s", w, out)
		}
	}
	if strings.Contains(out, "docker0") || strings.Contains(out, "fe80::") || strings.Contains(out, `"mac": ""`) {
		t.Errorf("en0, IPv4 only, and no empty MAC:\n%s", out)
	}
}

func TestIPArpFailures(t *testing.T) {
	withNeighbours(t, neighbourTable(), nil)
	withRoutes(t, nil, errors.New("no routing table here")) // marks nothing, fails nothing
	fakeInterfaces(t, []ifaceInfo{{Name: "lo0"}, {Name: "Ethernet 2"}, {Name: "Wi-Fi"}}, nil)
	asText := false
	out := captureStdout(t, func() {
		if IPArpHandler(&asText, "Ethernet9") {
			t.Error("an unknown interface: want a failure")
		}
	})
	if !strings.Contains(out, `[ip-arp] ERROR failed error="no such interface Ethernet9 (available: lo0, Ethernet 2, Wi-Fi)"`) {
		t.Errorf("unknown interface:\n%s", out)
	}
	out = captureStdout(t, func() { IPArpHandler(&asText, "Wi-Fi") })
	if !strings.Contains(out, `error="no neighbours known on Wi-Fi"`) {
		t.Errorf("an interface with no neighbours:\n%s", out)
	}
	withNeighbours(t, nil, errors.New("ip arp is not supported on plan9 yet"))
	out = captureStdout(t, func() { IPArpHandler(&asText, "") })
	if !strings.Contains(out, `error="ip arp is not supported on plan9 yet"`) {
		t.Errorf("unsupported:\n%s", out)
	}
}

func TestMACText(t *testing.T) {
	for in, want := range map[string]string{"\x60\xd8\xa4\x35\x8b\x04": "60:d8:a4:35:8b:04", "": "", "\x00\x00\x00\x00\x00\x00": ""} {
		if got := macText([]byte(in)); got != want {
			t.Errorf("macText(% x) = %q, want %q", in, got, want)
		}
	}
	if got := macText(net.HardwareAddr{0, 0x1b, 0x21, 0x3a, 0x4f, 0x5e, 0x60, 0x71}); got != "00:1b:21:3a:4f:5e:60:71" {
		t.Errorf("an EUI-64: %q", got)
	}
}
