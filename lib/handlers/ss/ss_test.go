package ss

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib"
	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func u32(v uint32) *uint32 { return &v }
func fdp(v int) *int       { return &v }

// ssTable is a machine's sockets as a platform reader returns them, out of
// order on purpose.
func ssTable() []lib.SSStats {
	return []lib.SSStats{
		{Protocol: "udp", Family: "ipv4", State: "UNCONN", LocalAddress: "0.0.0.0", LocalPort: 5353, RecvQ: u32(0), SendQ: u32(0), UID: u32(107), OwnerHidden: true},
		{Protocol: "tcp", Family: "ipv4", State: "ESTABLISHED", LocalAddress: "192.168.1.5", LocalPort: 50000, RemoteAddress: "93.184.216.34", RemotePort: 443,
			RecvQ: u32(0), SendQ: u32(16), UID: u32(1000), User: "fsabbir", Processes: []lib.SSProcess{{PID: 4321, Name: "curl", FD: fdp(5)}}},
		{Protocol: "tcp", Family: "ipv6", State: "LISTEN", LocalAddress: "::", LocalPort: 80, RecvQ: u32(3), SendQ: u32(0), UID: u32(0), User: "root",
			Processes: []lib.SSProcess{{PID: 100, Name: "nginx", FD: fdp(6)}, {PID: 101, Name: "nginx", FD: fdp(6)}}},
		{Protocol: "tcp", Family: "ipv4", State: "LISTEN", LocalAddress: "0.0.0.0", LocalPort: 22, RecvQ: u32(0), SendQ: u32(0), UID: u32(0), User: "root", OwnerHidden: true},
		{Protocol: "tcp", Family: "ipv4", State: "TIME-WAIT", LocalAddress: "192.168.1.5", LocalPort: 50001, RemoteAddress: "93.184.216.34", RemotePort: 443, RecvQ: u32(0), SendQ: u32(0)},
		{Protocol: "udp", Family: "ipv4", State: "ESTABLISHED", LocalAddress: "192.168.1.5", LocalPort: 54321, RemoteAddress: "8.8.8.8", RemotePort: 53,
			RecvQ: u32(0), SendQ: u32(0), UID: u32(1000), User: "fsabbir", Processes: []lib.SSProcess{{PID: 300, Name: "evil\x1b[2J name", FD: fdp(5)}}},
	}
}

func withSockets(t *testing.T, stats []lib.SSStats, err error) {
	t.Helper()
	orig := readSockets
	readSockets = func() ([]lib.SSStats, error) { return stats, err }
	t.Cleanup(func() { readSockets = orig })
}

func ssRun(t *testing.T, q Query, asJSON bool) (string, bool) {
	t.Helper()
	var ok bool
	out := testutil.CaptureStdout(t, func() { ok = Run(&asJSON, q) })
	return out, ok
}

func ssJSON(t *testing.T, out string) lib.LocalJSONOutput {
	t.Helper()
	var doc lib.LocalJSONOutput
	var stats []lib.SSStats
	doc.Stats = &stats
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	doc.Stats = stats
	return doc
}

func socketLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if _, rest, ok := strings.Cut(l, "[ss] OK socket "); ok {
			lines = append(lines, rest)
		}
	}
	return lines
}

func TestSSText(t *testing.T) {
	withSockets(t, ssTable(), nil)
	out, ok := ssRun(t, Query{}, false)
	want := []string{
		"proto=tcp state=LISTEN local=0.0.0.0:22 owner=hidden user=root",
		"proto=tcp state=LISTEN local=[::]:80 recv_q=3 processes=[nginx(pid=100,fd=6),nginx(pid=101,fd=6)] user=root",
		"proto=tcp state=ESTABLISHED local=192.168.1.5:50000 remote=93.184.216.34:443 send_q=16 process=curl pid=4321 fd=5 user=fsabbir",
		"proto=tcp state=TIME-WAIT local=192.168.1.5:50001 remote=93.184.216.34:443",
		"proto=udp state=UNCONN local=0.0.0.0:5353 owner=hidden uid=107",
		// escaped, then quoted for its space - as dns does with TXT data
		`proto=udp state=ESTABLISHED local=192.168.1.5:54321 remote=8.8.8.8:53 process="evil\\x1b[2J name" pid=300 fd=5 user=fsabbir`,
	}
	got := socketLines(out)
	if !ok || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ok %v, lines:\n%s\nwant:\n%s", ok, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if strings.Contains(out, "\x1b") {
		t.Error("a raw escape character reached the terminal")
	}
	if !strings.Contains(out, "[ss] OK done sockets=6 tcp=4 udp=2 listening=3 owners_hidden=2 total_time=") ||
		!strings.Contains(out, "[ss] WARN owners hidden sockets=2 warning=") {
		t.Errorf("summary:\n%s", out)
	}
}

func TestSSFilters(t *testing.T) {
	withSockets(t, ssTable(), nil)
	defer func() { _ = lib.SetIPFamily(false, false) }()
	ports := func(spec string) Query {
		p, err := lib.ParsePorts("ports", spec)
		if err != nil {
			t.Fatal(err)
		}
		return Query{Ports: p, PortsText: lib.PortsText(p)}
	}
	for _, c := range []struct {
		name   string
		q      Query
		family string
		locals []string
	}{
		{"tcp", Query{TCP: true}, "", []string{"0.0.0.0:22", "[::]:80", "192.168.1.5:50000", "192.168.1.5:50001"}},
		{"udp", Query{UDP: true}, "", []string{"0.0.0.0:5353", "192.168.1.5:54321"}},
		{"both", Query{TCP: true, UDP: true}, "", []string{"0.0.0.0:22", "[::]:80", "192.168.1.5:50000", "192.168.1.5:50001", "0.0.0.0:5353", "192.168.1.5:54321"}},
		{"listening", Query{Listening: true}, "", []string{"0.0.0.0:22", "[::]:80", "0.0.0.0:5353"}},
		{"listening udp", Query{Listening: true, UDP: true}, "", []string{"0.0.0.0:5353"}},
		{"remote port", ports("443"), "", []string{"192.168.1.5:50000", "192.168.1.5:50001"}},
		{"port list", ports("22,53,80"), "", []string{"0.0.0.0:22", "[::]:80", "192.168.1.5:54321"}},
		{"pid", Query{PID: 101}, "", []string{"[::]:80"}},
		{"-6", Query{}, "6", []string{"[::]:80"}},
		{"-4 listening tcp", Query{Listening: true, TCP: true}, "4", []string{"0.0.0.0:22"}},
	} {
		_ = lib.SetIPFamily(c.family == "4", c.family == "6")
		out, ok := ssRun(t, c.q, false)
		var got []string
		for _, l := range socketLines(out) {
			local := strings.Fields(l)[2]
			if !strings.HasPrefix(local, "local=") {
				local = strings.Fields(l)[1] // a socket with no state
			}
			got = append(got, strings.TrimPrefix(local, "local="))
		}
		if !ok || strings.Join(got, " ") != strings.Join(c.locals, " ") {
			t.Errorf("%s: ok %v, %v, want %v", c.name, ok, got, c.locals)
		}
	}
}

func TestSSNoMatch(t *testing.T) {
	withSockets(t, ssTable(), nil)
	defer func() { _ = lib.SetIPFamily(false, false) }()
	_ = lib.SetIPFamily(true, false)
	p, _ := lib.ParsePorts("ports", "5432")
	out, ok := ssRun(t, Query{TCP: true, Listening: true, Ports: p, PortsText: "5432"}, false)
	if ok || !strings.Contains(out, `[ss] ERROR failed error="no socket matches: tcp, IPv4, listening, port 5432"`) || strings.Contains(out, "socket proto") {
		t.Errorf("ok %v:\n%s", ok, out)
	}
	_ = lib.SetIPFamily(false, false)

	// A process whose sockets cannot be seen: say that some were hidden.
	out, ok = ssRun(t, Query{PID: 999}, true)
	doc := ssJSON(t, out)
	if ok || doc.Error != "no socket matches: held by process 999 (the processes of 2 sockets could not be seen; run as root to include them)" || len(doc.Stats.([]lib.SSStats)) != 0 {
		t.Errorf("ok %v, error %q", ok, doc.Error)
	}

	withSockets(t, nil, nil)
	if out, ok := ssRun(t, Query{}, false); ok || !strings.Contains(out, "this machine reports no TCP or UDP sockets") {
		t.Errorf("no sockets at all: ok %v:\n%s", ok, out)
	}
}

func TestSSReadError(t *testing.T) {
	withSockets(t, nil, errors.New("ss is not supported on plan9 yet: it reads the socket tables of Linux and Windows"))
	out, ok := ssRun(t, Query{}, false)
	if ok || !strings.Contains(out, `[ss] ERROR failed error="ss is not supported on plan9 yet`) {
		t.Errorf("text: ok %v:\n%s", ok, out)
	}
	out, ok = ssRun(t, Query{Listening: true}, true)
	doc := ssJSON(t, out)
	if ok || !strings.HasPrefix(doc.Error, "ss is not supported on plan9") || doc.Stats == nil || len(doc.Stats.([]lib.SSStats)) != 0 || !strings.Contains(out, `"stats": []`) {
		t.Errorf("json: ok %v:\n%s", ok, out)
	}
}

func TestSSJSON(t *testing.T) {
	withSockets(t, ssTable(), nil)
	p, _ := lib.ParsePorts("ports", "80,22")
	out, ok := ssRun(t, Query{TCP: true, Listening: true, Ports: p, PortsText: lib.PortsText(p)}, true)
	doc := ssJSON(t, out)
	in := doc.InputParams
	if !ok || doc.ModuleName != "ss" || in.Mode != "ss" || in.Protocol != "tcp" || in.Ports != "22,80" || !in.Listening || in.PID != 0 || doc.Error != "" {
		t.Errorf("ok %v, input_params %+v, error %q", ok, in, doc.Error)
	}
	stats := doc.Stats.([]lib.SSStats)
	if len(stats) != 2 || stats[0].LocalPort != 22 || !stats[0].OwnerHidden || stats[0].Processes != nil || *stats[0].UID != 0 ||
		stats[1].LocalPort != 80 || len(stats[1].Processes) != 2 || *stats[1].Processes[1].FD != 6 || *stats[1].RecvQ != 3 {
		t.Errorf("stats %+v", stats)
	}
	// uid 0 and a zero queue are values, not absent ones; a peer-less socket
	// has no remote fields at all.
	for _, want := range []string{`"uid": 0`, `"send_q": 0`, `"owner_hidden": true`, `"fd": 6`} {
		if !strings.Contains(out, want) {
			t.Errorf("want %s in:\n%s", want, out)
		}
	}
	for _, absent := range []string{`"remote_address"`, `"remote_port"`, `"dns_lookup"`, `"targets"`} {
		if strings.Contains(out, absent) {
			t.Errorf("%s should be absent:\n%s", absent, out)
		}
	}

	// A Windows socket: no queues, user or descriptor.
	withSockets(t, []lib.SSStats{{Protocol: "udp", Family: "ipv4", LocalAddress: "0.0.0.0", LocalPort: 5353, Processes: []lib.SSProcess{{PID: 2000, Name: "svchost.exe"}}}}, nil)
	out, ok = ssRun(t, Query{}, true)
	for _, absent := range []string{`"state"`, `"recv_q"`, `"uid"`, `"user"`, `"fd"`, `"owner_hidden"`} {
		if !ok || strings.Contains(out, absent) {
			t.Errorf("ok %v, %s should be absent:\n%s", ok, absent, out)
		}
	}
	out, _ = ssRun(t, Query{Listening: true}, false)
	if lines := socketLines(out); len(lines) != 1 || lines[0] != "proto=udp local=0.0.0.0:5353 process=svchost.exe pid=2000" {
		t.Errorf("windows udp text: %q", lines)
	}
}
