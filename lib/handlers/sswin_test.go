package handlers

import (
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"
)

// winTable builds a table the way Windows lays it out: a little-endian row
// count, then the rows.
func winTable(rows ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(rows)))
	for _, r := range rows {
		b = append(b, r...)
	}
	return b
}

func dw(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

// wport is a port as Windows stores it: network byte order in the low two
// bytes of a DWORD.
func wport(p uint16) []byte { return []byte{byte(p >> 8), byte(p), 0, 0} }

func wv4(s string) []byte { a := netip.MustParseAddr(s).As4(); return a[:] }
func wv6(s string) []byte { a := netip.MustParseAddr(s).As16(); return a[:] }

func cat(parts ...[]byte) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}
	return b
}

func TestParseWinTables(t *testing.T) {
	tcp4 := winTable(
		cat(dw(2), wv4("0.0.0.0"), wport(445), wv4("0.0.0.0"), wport(28771), dw(4)), // a listener, garbage remote port
		cat(dw(5), wv4("10.0.0.5"), wport(50000), wv4("93.184.216.34"), wport(443), dw(1234)),
		cat(dw(11), wv4("10.0.0.5"), wport(50001), wv4("93.184.216.34"), wport(443), dw(0)),
		cat(dw(99), wv4("10.0.0.5"), wport(50002), wv4("10.0.0.6"), wport(80), dw(1234)),
	)
	rows, err := parseWinTable(tcp4, "tcp", "ipv4")
	if err != nil || len(rows) != 4 {
		t.Fatalf("tcp4: %v %+v", err, rows)
	}
	l, c, tw, odd := rows[0], rows[1], rows[2], rows[3]
	if l.stat.State != "LISTEN" || l.stat.LocalPort != 445 || l.stat.RemoteAddress != "" || l.stat.RemotePort != 0 || l.pid != 4 {
		t.Errorf("listener %+v", l)
	}
	if c.stat.State != "ESTABLISHED" || c.stat.LocalAddress != "10.0.0.5" || c.stat.LocalPort != 50000 || c.stat.RemoteAddress != "93.184.216.34" || c.stat.RemotePort != 443 || c.pid != 1234 {
		t.Errorf("connection %+v", c)
	}
	if tw.stat.State != "TIME-WAIT" || tw.pid != 0 || odd.stat.State != "UNKNOWN-99" {
		t.Errorf("time-wait %+v, unknown state %+v", tw, odd)
	}
	if c.stat.RecvQ != nil || c.stat.UID != nil {
		t.Errorf("Windows reports no queues or user: %+v", c.stat)
	}

	tcp6 := winTable(
		cat(wv6("fe80::1"), dw(12), wport(5985), wv6("fe80::2"), dw(12), wport(51000), dw(5), dw(888)),
		cat(wv6("::"), dw(0), wport(135), wv6("::"), dw(0), wport(0), dw(2), dw(900)),
	)
	rows, err = parseWinTable(tcp6, "tcp", "ipv6")
	if err != nil || len(rows) != 2 {
		t.Fatalf("tcp6: %v %+v", err, rows)
	}
	if s := rows[0].stat; s.LocalAddress != "fe80::1%12" || s.LocalPort != 5985 || s.RemoteAddress != "fe80::2%12" || s.RemotePort != 51000 || s.State != "ESTABLISHED" || rows[0].pid != 888 {
		t.Errorf("link-local connection %+v", rows[0])
	}
	if s := rows[1].stat; s.LocalAddress != "::" || s.State != "LISTEN" || s.RemoteAddress != "" || rows[1].pid != 900 {
		t.Errorf("v6 listener %+v", rows[1])
	}

	udp4 := winTable(cat(wv4("0.0.0.0"), wport(5353), dw(2000)))
	rows, err = parseWinTable(udp4, "udp", "ipv4")
	if err != nil || len(rows) != 1 || rows[0].stat.LocalPort != 5353 || rows[0].stat.State != "" || rows[0].pid != 2000 {
		t.Errorf("udp4: %v %+v", err, rows)
	}
	udp6 := winTable(cat(wv6("2001:db8::7"), dw(3), wport(546), dw(2001)))
	rows, err = parseWinTable(udp6, "udp", "ipv6")
	if err != nil || len(rows) != 1 || rows[0].stat.LocalAddress != "2001:db8::7" || rows[0].stat.LocalPort != 546 || rows[0].pid != 2001 {
		t.Errorf("udp6 (a scope on a global address is not a zone): %v %+v", err, rows)
	}

	empty, err := parseWinTable(winTable(), "tcp", "ipv4")
	if err != nil || len(empty) != 0 {
		t.Errorf("empty table: %v %+v", err, empty)
	}
}

func TestParseWinTableRefuses(t *testing.T) {
	row := cat(dw(5), wv4("10.0.0.5"), wport(1), wv4("10.0.0.6"), wport(2), dw(3))
	for name, c := range map[string]struct {
		b    []byte
		says string
	}{
		"no header":     {[]byte{1, 0}, "shorter than its header"},
		"short row":     {winTable(row)[:20], "says it has 1 rows, but holds 16 bytes"},
		"too many rows": {cat(dw(1<<30), row), "says it has 1073741824 rows"},
		"max rows":      {cat(dw(0xffffffff), row), "rows, but holds 24 bytes"},
	} {
		if _, err := parseWinTable(c.b, "tcp", "ipv4"); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v, want %q", name, err, c.says)
		}
	}
}

func TestWinSockets(t *testing.T) {
	rows, _ := parseWinTable(winTable(
		cat(dw(2), wv4("0.0.0.0"), wport(80), wv4("0.0.0.0"), wport(0), dw(4)),
		cat(dw(11), wv4("10.0.0.5"), wport(50001), wv4("10.0.0.6"), wport(80), dw(0)),
		cat(dw(5), wv4("10.0.0.5"), wport(50002), wv4("10.0.0.6"), wport(80), dw(4242)),
	), "tcp", "ipv4")
	stats := winSockets(rows, map[uint32]string{4: "System", 0: "System Idle Process"})
	if len(stats[0].Processes) != 1 || stats[0].Processes[0].PID != 4 || stats[0].Processes[0].Name != "System" || stats[0].Processes[0].FD != nil {
		t.Errorf("listener %+v", stats[0])
	}
	if stats[1].Processes != nil || stats[1].OwnerHidden {
		t.Errorf("PID 0 is no process: %+v", stats[1])
	}
	if len(stats[2].Processes) != 1 || stats[2].Processes[0].PID != 4242 || stats[2].Processes[0].Name != "" {
		t.Errorf("a process with no name in the snapshot %+v", stats[2])
	}
}
