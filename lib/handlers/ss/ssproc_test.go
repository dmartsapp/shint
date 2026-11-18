package ss

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib"
)

func TestProcAddrPort(t *testing.T) {
	le, be := binary.LittleEndian, binary.BigEndian
	for _, c := range []struct {
		in     string
		family string
		order  binary.ByteOrder
		want   string
	}{
		{"0100007F:0016", "ipv4", le, "127.0.0.1:22"},
		{"7F000001:0016", "ipv4", be, "127.0.0.1:22"},
		{"00000000:0000", "ipv4", le, "0.0.0.0:0"},
		{"0501A8C0:C350", "ipv4", le, "192.168.1.5:50000"},
		{"00000000000000000000000001000000:1F90", "ipv6", le, "[::1]:8080"},
		{"00000000000000000000000000000001:1F90", "ipv6", be, "[::1]:8080"},
		{"0000000000000000FFFF00000100007F:0050", "ipv6", le, "[::ffff:127.0.0.1]:80"},
		{"000080FE00000000FF005452123456FE:0222", "ipv6", le, "[fe80::5254:ff:fe56:3412]:546"},
	} {
		got, err := procAddrPort(c.in, c.family, c.order)
		if err != nil || got.String() != c.want {
			t.Errorf("procAddrPort(%q, %s): %v %v, want %s", c.in, c.family, got, err, c.want)
		}
	}
	for _, in := range []string{"0100007F", "0100007F:16", "100007F:0016", "0100007G:0016", "0100007F:001G", "0100007F:0016:1", ""} {
		if got, err := procAddrPort(in, "ipv4", le); err == nil {
			t.Errorf("procAddrPort(%q) = %v, want an error", in, got)
		}
	}
	if _, err := procAddrPort("0100007F:0016", "ipv6", le); err == nil {
		t.Error("an IPv4 address in an IPv6 table: want an error")
	}
}

// procTCP is a /proc/net/tcp as a little-endian kernel writes it: a listener
// on 0.0.0.0:22, a connection with bytes queued, a TIME-WAIT socket (inode 0)
// and the listener again, as a table read in pieces can repeat a line.
const procTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0016 00000000:0000 0A 00000000:00000003 00:00000000 00000000     0        0 1001 1 0000000000000000 100 0 0 10 0
   1: 0501A8C0:C350 22D8B85D:01BB 01 00000010:00000000 02:000000F2 00000000  1000        0 1002 2 0000000000000000 20 4 30 10 -1
   2: 0501A8C0:C351 22D8B85D:01BB 06 00000000:00000000 03:00000F11 00000000     0        0 0 3 0000000000000000
   0: 00000000:0016 00000000:0000 0A 00000000:00000003 00:00000000 00000000     0        0 1001 1 0000000000000000 100 0 0 10 0
`

const procUDP = `   sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
  100: 00000000:14E9 00000000:0000 07 00000000:00000000 00:00000000 00000000   107        0 1003 2 0000000000000000 0
  101: 0501A8C0:D431 08080808:0035 01 00000000:00000000 00:00000000 00000000  1000        0 1004 2 0000000000000000 0
`

func TestParseProcNet(t *testing.T) {
	got, err := parseProcNet(strings.NewReader(procTCP), "tcp", "ipv4", binary.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("%d sockets, want 3 (the repeated line read once): %+v", len(got), got)
	}
	l, c, tw := got[0], got[1], got[2]
	if l.stat.State != "LISTEN" || l.stat.LocalAddress != "0.0.0.0" || l.stat.LocalPort != 22 || l.stat.RemoteAddress != "" || l.stat.RemotePort != 0 ||
		*l.stat.RecvQ != 3 || *l.stat.SendQ != 0 || l.inode != 1001 || l.uid != 0 {
		t.Errorf("listener %+v (recv %d, send %d)", l, *l.stat.RecvQ, *l.stat.SendQ)
	}
	if c.stat.State != "ESTABLISHED" || c.stat.LocalAddress != "192.168.1.5" || c.stat.LocalPort != 50000 || c.stat.RemoteAddress != "93.184.216.34" ||
		c.stat.RemotePort != 443 || *c.stat.SendQ != 16 || c.uid != 1000 || c.inode != 1002 {
		t.Errorf("connection %+v", c)
	}
	if tw.stat.State != "TIME-WAIT" || tw.inode != 0 || tw.stat.LocalPort != 50001 {
		t.Errorf("time-wait %+v", tw)
	}

	udp, err := parseProcNet(strings.NewReader(procUDP), "udp", "ipv4", binary.LittleEndian)
	if err != nil || len(udp) != 2 {
		t.Fatalf("udp: %v %+v", err, udp)
	}
	if udp[0].stat.State != "UNCONN" || udp[0].stat.LocalPort != 5353 || udp[0].stat.RemoteAddress != "" ||
		udp[1].stat.State != "ESTABLISHED" || udp[1].stat.RemoteAddress != "8.8.8.8" || udp[1].stat.RemotePort != 53 {
		t.Errorf("udp %+v", udp)
	}

	header := strings.SplitN(procTCP, "\n", 2)[0] + "\n"
	for name, table := range map[string]string{
		"no header":          strings.SplitN(procTCP, "\n", 2)[1],
		"short line":         header + "   0: 00000000:0016 00000000:0000 0A\n",
		"bad state":          header + "   0: 00000000:0016 00000000:0000 XY 00000000:00000000 00:00000000 00000000 0 0 1 1\n",
		"long state":         header + "   0: 00000000:0016 00000000:0000 00A 00000000:00000000 00:00000000 00000000 0 0 1 1\n",
		"bad queues":         header + "   0: 00000000:0016 00000000:0000 0A 0000000000000000 00:00000000 00000000 0 0 1 1\n",
		"bad uid":            header + "   0: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000 -1 0 1 1\n",
		"bad inode":          header + "   0: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 x 1\n",
		"bad address":        header + "   0: 0000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 1 1\n",
		"too long line":      header + strings.Repeat("0", 5000) + "\n",
		"ipv6 in ipv4 table": header + "   0: 00000000000000000000000001000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 1 1\n",
	} {
		if _, err := parseProcNet(strings.NewReader(table), "tcp", "ipv4", binary.LittleEndian); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// fakeProc makes a /proc with the tables above and processes holding their
// sockets: nginx 100 and 101 share the listener (inode 1001), 100 also holds
// the connection (1002) and an unrelated file; "evil" (300) holds the
// connected UDP socket under a name with an escape sequence in it. Process
// 200's descriptors cannot be read when denied is set.
func fakeProc(t *testing.T, denied bool) string {
	t.Helper()
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := func(pid, fd int, target string) {
		t.Helper()
		dir := filepath.Join(root, strconv.Itoa(pid), "fd")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, strconv.Itoa(fd))); err != nil {
			t.Fatal(err)
		}
	}
	write("net/tcp", procTCP)
	write("net/udp", procUDP)
	write("100/comm", "nginx\n")
	link(100, 6, "socket:[1001]")
	link(100, 9, "socket:[1002]")
	link(100, 3, "/var/log/nginx/access.log")
	link(100, 4, "pipe:[1001]")
	write("101/comm", "nginx\n")
	link(101, 6, "socket:[1001]")
	write("300/comm", "evil\x1b[2J\n")
	link(300, 5, "socket:[1004]")
	write("self/comm", "shint\n") // not a PID: skipped
	link(200, 3, "socket:[1003]")
	if denied {
		dir := filepath.Join(root, "200", "fd")
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	}
	return root
}

func withUserNames(t *testing.T) {
	t.Helper()
	orig := procUserName
	procUserName = func(uid uint32) string {
		if uid == 107 {
			return ""
		}
		return "user" + strconv.Itoa(int(uid))
	}
	t.Cleanup(func() { procUserName = orig })
}

func byLocalPort(stats []lib.SSStats) map[string]lib.SSStats {
	m := map[string]lib.SSStats{}
	for _, s := range stats {
		m[s.Protocol+"/"+strconv.Itoa(s.LocalPort)] = s
	}
	return m
}

func TestReadProcSockets(t *testing.T) {
	withUserNames(t)
	if os.Geteuid() == 0 {
		t.Skip("run as root, a directory with no permissions can still be read")
	}
	root := fakeProc(t, true)
	stats, err := readProcSockets(root, binary.LittleEndian, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 5 {
		t.Fatalf("%d sockets, want 5 (no IPv6 tables, which is not an error): %+v", len(stats), stats)
	}
	m := byLocalPort(stats)

	l := m["tcp/22"]
	if len(l.Processes) != 2 || l.Processes[0].PID != 100 || *l.Processes[0].FD != 6 || l.Processes[1].PID != 101 || l.Processes[0].Name != "nginx" || l.OwnerHidden ||
		*l.UID != 0 || l.User != "user0" {
		t.Errorf("shared listener %+v", l)
	}
	c := m["tcp/50000"]
	if len(c.Processes) != 1 || c.Processes[0].PID != 100 || *c.Processes[0].FD != 9 || c.OwnerHidden || c.User != "user1000" {
		t.Errorf("connection %+v", c)
	}
	tw := m["tcp/50001"]
	if tw.Processes != nil || tw.OwnerHidden || tw.UID != nil || tw.User != "" {
		t.Errorf("TIME-WAIT: no process, no user, not hidden: %+v", tw)
	}
	// 1003 is held by process 200, whose descriptors could not be read.
	u := m["udp/5353"]
	if u.Processes != nil || !u.OwnerHidden || *u.UID != 107 || u.User != "" {
		t.Errorf("hidden owner %+v", u)
	}
	e := m["udp/54321"]
	if len(e.Processes) != 1 || e.Processes[0].Name != "evil\x1b[2J" || *e.Processes[0].FD != 5 {
		t.Errorf("udp connection %+v", e)
	}

	// As root nothing is hidden: a socket whose process is not found has
	// none (it exited meanwhile).
	stats, err = readProcSockets(root, binary.LittleEndian, 0)
	if err != nil {
		t.Fatal(err)
	}
	if u := byLocalPort(stats)["udp/5353"]; u.OwnerHidden {
		t.Errorf("as root: %+v", u)
	}
}

// A socket of this user's own whose process is not found is not hidden when
// every process could be read: the process has exited meanwhile.
func TestReadProcSocketsOwnSocketGone(t *testing.T) {
	withUserNames(t)
	root := fakeProc(t, false)
	if err := os.Remove(filepath.Join(root, "300", "fd", "5")); err != nil {
		t.Fatal(err)
	}
	stats, err := readProcSockets(root, binary.LittleEndian, 1000)
	if err != nil {
		t.Fatal(err)
	}
	m := byLocalPort(stats)
	if e := m["udp/54321"]; e.Processes != nil || e.OwnerHidden {
		t.Errorf("own socket, process gone: %+v", e)
	}
	if u := m["udp/5353"]; len(u.Processes) != 1 || u.Processes[0].PID != 200 || u.Processes[0].Name != "" {
		t.Errorf("readable process 200 (no comm): %+v", u)
	}
	// Another user's socket whose process is not found is hidden, even when
	// every listed process could be read: /proc may not list theirs.
	if err := os.Remove(filepath.Join(root, "200", "fd", "3")); err != nil {
		t.Fatal(err)
	}
	stats, _ = readProcSockets(root, binary.LittleEndian, 1000)
	if u := byLocalPort(stats)["udp/5353"]; !u.OwnerHidden {
		t.Errorf("another user's socket, no process found: %+v", u)
	}
}

func TestReadProcSocketsErrors(t *testing.T) {
	withUserNames(t)
	root := fakeProc(t, false)
	if err := os.Remove(filepath.Join(root, "net", "udp")); err != nil {
		t.Fatal(err)
	}
	if _, err := readProcSockets(root, binary.LittleEndian, 1000); err == nil || !strings.Contains(err.Error(), "/proc/net/udp: it does not exist (is /proc mounted?)") {
		t.Errorf("missing udp table: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "net", "udp"), []byte(procUDP+"garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readProcSockets(root, binary.LittleEndian, 1000); err == nil || !strings.Contains(err.Error(), "could not read /proc/net/udp: line 4:") {
		t.Errorf("bad line: %v", err)
	}
}
