package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMain lets the tests below run the real CLI without a build step: when
// SHINT_TEST_RUN_MAIN is set, this test binary behaves as `shint` itself, so
// a test can start it as a subprocess and check the actual exit status and
// output streams - the things a shell script sees.
func TestMain(m *testing.M) {
	if os.Getenv("SHINT_TEST_RUN_MAIN") == "1" {
		main() // exits the process itself
		return
	}
	os.Exit(m.Run())
}

// runShint runs the CLI with args and returns its exit status, stdout, stderr.
func runShint(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), "SHINT_TEST_RUN_MAIN=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("could not run shint %v: %v", args, err)
	}
	return code, out.String(), errb.String()
}

func tcpListener(t *testing.T) (port int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

func closedTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func udpSocket(t *testing.T, echo bool) (port int) {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := c.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if echo {
				_, _ = c.WriteToUDP(buf[:n], from)
			}
		}
	}()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// ntpSocket is a minimal SNTP server on loopback: it answers every 48-byte
// request with a well-formed reply stamped with the real time.
func ntpSocket(t *testing.T) (port int) {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	stamp := func() uint64 {
		now := time.Now()
		return uint64(now.Unix()+2208988800)<<32 | (uint64(now.Nanosecond())<<32)/1e9
	}
	go func() {
		buf := make([]byte, 512)
		for {
			n, from, err := c.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < 48 {
				continue
			}
			reply := make([]byte, 48)
			reply[0], reply[1] = 4<<3|4, 2 // version 4, server mode, stratum 2
			copy(reply[24:32], buf[40:48]) // originate = the client's transmit
			for i, v := range [2]uint64{stamp(), stamp()} {
				for b := 0; b < 8; b++ {
					reply[32+i*8+b] = byte(v >> (56 - 8*b))
				}
			}
			_, _ = c.WriteToUDP(reply, from)
		}
	}()
	return c.LocalAddr().(*net.UDPAddr).Port
}

func closedUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	_ = c.Close()
	return port
}

// The three exit statuses, as documented on the exitOK/exitFailure/exitUsage
// constants: 0 every check passed, 1 at least one check failed, 2 misuse.
func TestExitStatus(t *testing.T) {
	open := strconv.Itoa(tcpListener(t))
	closed := strconv.Itoa(closedTCPPort(t))
	const dead = "this-host-should-not-exist.invalid"

	web200 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer web200.Close()
	web404 := httptest.NewServer(http.NotFoundHandler())
	defer web404.Close()

	udpEcho := strconv.Itoa(udpSocket(t, true))
	udpSilent := strconv.Itoa(udpSocket(t, false))
	udpClosed := strconv.Itoa(closedUDPPort(t))
	ntpUp := strconv.Itoa(ntpSocket(t))

	fast := []string{"--delay", "0", "--timeout", "2"}
	cases := []struct {
		name string
		args []string
		want int
	}{
		// telnet
		{"telnet open port", []string{"telnet", "127.0.0.1", open}, 0},
		{"telnet connection refused", []string{"telnet", "127.0.0.1", closed}, 1},
		{"telnet dns failure", []string{"telnet", dead, "80"}, 1},
		{"telnet port out of range", []string{"telnet", "127.0.0.1", "99999"}, 2},
		{"telnet missing argument", []string{"telnet", "127.0.0.1"}, 2},
		{"telnet count zero", []string{"telnet", "127.0.0.1", open, "--count", "0"}, 2},

		// web: a response is a response, whatever its status; no response is a failure
		{"web 200", []string{"web", web200.URL}, 0},
		{"web 404 is still a response", []string{"web", web404.URL}, 0},
		{"web connection refused", []string{"web", "http://127.0.0.1:" + closed + "/"}, 1},
		{"web unknown flag", []string{"web", web200.URL, "--nope"}, 2},

		// nmap
		{"nmap finds an open port", []string{"nmap", "127.0.0.1", "--from", open, "--to", open}, 0},
		{"nmap finding nothing is a completed scan", []string{"nmap", "127.0.0.1", "--from", closed, "--to", closed}, 0},
		{"nmap dns failure", []string{"nmap", dead}, 1},
		{"nmap from greater than to", []string{"nmap", "127.0.0.1", "--from", "90", "--to", "10"}, 2},

		// udp: only evidence the port is closed (or an error) is a failure
		{"udp reply", []string{"udp", "127.0.0.1", udpEcho, "--data", "x"}, 0},
		{"udp no reply is inconclusive", []string{"udp", "127.0.0.1", udpSilent}, 0},
		{"udp closed port", []string{"udp", "127.0.0.1", udpClosed}, 1},
		{"udp dns failure", []string{"udp", dead, "53"}, 1},

		// ntp: no usable time reply is a failure; --max-offset makes the offset a check
		{"ntp answers", []string{"ntp", "127.0.0.1", "--port", ntpUp}, 0},
		{"ntp offset within --max-offset", []string{"ntp", "127.0.0.1", "--port", ntpUp, "--max-offset", "5000"}, 0},
		{"ntp no reply", []string{"ntp", "127.0.0.1", "--port", udpSilent, "--timeout", "1"}, 1},
		{"ntp dns failure", []string{"ntp", dead}, 1},
		{"ntp port out of range", []string{"ntp", "127.0.0.1", "--port", "99999"}, 2},
		{"ntp negative --max-offset", []string{"ntp", "127.0.0.1", "--max-offset", "-1"}, 2},
		{"ntp missing argument", []string{"ntp"}, 2},

		// wol: sent is success (there is no reply to wait for); bad input is a usage error
		{"wol sent", []string{"wol", "aa:bb:cc:dd:ee:ff", "--broadcast", "127.0.0.1", "--port", udpSilent}, 0},
		{"wol bad MAC", []string{"wol", "not-a-mac"}, 2},
		{"wol IPv6 broadcast", []string{"wol", "aa:bb:cc:dd:ee:ff", "--broadcast", "::1"}, 2},
		{"wol port zero", []string{"wol", "aa:bb:cc:dd:ee:ff", "--port", "0"}, 2},
		{"wol count zero", []string{"wol", "aa:bb:cc:dd:ee:ff", "--count", "0"}, 2},

		// cidr: pure computation; only bad input fails, and that is a usage error
		{"cidr IPv4", []string{"cidr", "192.168.1.10/24"}, 0},
		{"cidr several, both families", []string{"cidr", "10.0.0.0/8", "2001:db8::/32", "8.8.8.8"}, 0},
		{"cidr prefix out of range", []string{"cidr", "10.0.0.0/33"}, 2},
		{"cidr one bad among good", []string{"cidr", "10.0.0.0/8", "nope"}, 2},
		{"cidr missing argument", []string{"cidr"}, 2},

		// ping (no ICMP privileges needed for these)
		{"ping dns failure", []string{"ping", dead}, 1},
		{"ping timeout zero", []string{"ping", "127.0.0.1", "--timeout", "0"}, 2},
		{"ping payload too large", []string{"ping", "127.0.0.1", "--payload", "5000"}, 2},
		{"ping payload negative", []string{"ping", "127.0.0.1", "--payload", "-1"}, 2},

		// listen
		{"listen bad port", []string{"listen", "tcp", "99999"}, 2},
		{"listen port already in use", []string{"listen", "tcp", open, "--bind", "127.0.0.1"}, 1},

		// generic
		{"unknown command", []string{"bogus"}, 2},
		{"help", []string{"--help"}, 0},
		{"version", []string{"--version"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.args[0] != "listen" && tc.args[0] != "ping" && !strings.HasPrefix(tc.args[0], "-") && tc.args[0] != "bogus" {
				args = append(append([]string{}, tc.args...), fast...)
			}
			code, stdout, stderr := runShint(t, args...)
			if code != tc.want {
				t.Errorf("shint %v: exit status %d, want %d\nstdout:\n%s\nstderr:\n%s", args, code, tc.want, stdout, stderr)
			}
		})
	}
}

// Usage errors belong on stderr and must not touch stdout, so a script piping
// stdout (or --json into jq) never mixes them with results.
func TestUsageErrorsGoToStderrOnly(t *testing.T) {
	for _, args := range [][]string{
		{"telnet", "127.0.0.1", "99999"},
		{"nmap", "127.0.0.1", "--from", "90", "--to", "10"},
		{"telnet", "127.0.0.1"},
		{"cidr", "10.0.0.0/33"},
		{"cidr"},
		{"wol", "not-a-mac"},
		{"ntp", "127.0.0.1", "--port", "0"},
		{"bogus"},
	} {
		code, stdout, stderr := runShint(t, args...)
		if code != 2 {
			t.Errorf("shint %v: exit status %d, want 2", args, code)
		}
		if stdout != "" {
			t.Errorf("shint %v: usage error leaked onto stdout:\n%s", args, stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Errorf("shint %v: no message on stderr", args)
		}
		if n := strings.Count(stderr, "unknown command"); n > 1 {
			t.Errorf("shint %v: error printed %d times, want once:\n%s", args, n, stderr)
		}
	}
}

// A failed check under --json is still exactly one JSON document on stdout
// (exit status 1), with the failure recorded in stats - never a stray text
// line in front of it.
func TestFailuresUnderJSONAreStillJSON(t *testing.T) {
	closed := strconv.Itoa(closedTCPPort(t))
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"web connection refused", []string{"web", "--json", "http://127.0.0.1:" + closed + "/", "--timeout", "2", "--delay", "0"}},
		{"telnet connection refused", []string{"telnet", "--json", "127.0.0.1", closed, "--timeout", "2", "--delay", "0"}},
		{"ping dns failure", []string{"ping", "--json", "this-host-should-not-exist.invalid"}},
		{"udp closed port", []string{"udp", "--json", "127.0.0.1", strconv.Itoa(closedUDPPort(t)), "--timeout", "2", "--delay", "0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runShint(t, tc.args...)
			if code != 1 {
				t.Errorf("exit status %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if !json.Valid([]byte(stdout)) {
				t.Fatalf("stdout is not a single JSON document:\n%s", stdout)
			}
			if stderr != "" {
				t.Errorf("unexpected stderr:\n%s", stderr)
			}
		})
	}

	t.Run("web failure is recorded in stats", func(t *testing.T) {
		_, stdout, _ := runShint(t, "web", "--json", "http://127.0.0.1:"+closed+"/", "--timeout", "2", "--delay", "0")
		var doc struct {
			Stats []struct {
				Success bool     `json:"success"`
				Errors  []string `json:"errors"`
			} `json:"stats"`
		}
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Stats) != 1 || doc.Stats[0].Success || len(doc.Stats[0].Errors) == 0 {
			t.Errorf("expected one failed stat with an error, got %+v", doc.Stats)
		}
	})
}

// A listener that finishes its work (its --count reached) exits 0.
func TestListenExitsZeroWhenDone(t *testing.T) {
	port := closedTCPPort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "listen", "tcp", strconv.Itoa(port), "--bind", "127.0.0.1", "--count", "2", "--timeout", "5")
	cmd.Env = append(os.Environ(), "SHINT_TEST_RUN_MAIN=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	dial := func() bool {
		c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}
	for i := 0; i < 2; { // two connections use up --count 2
		if dial() {
			i++
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("listener that reached its --count should exit 0, got: %v", err)
	}
}

// TestScanContextHasNoDeadline pins the fix for nmap scans being cut short:
// the scan's context may be cancelled (Ctrl+C) but must never carry a
// deadline, in particular not one derived from --timeout, which is only the
// per-port connect timeout.
func TestInterruptContextHasNoDeadline(t *testing.T) {
	ctx, stop := interruptContext()
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		t.Fatalf("interrupt context has a deadline (%v); --timeout must bound each operation, not the whole run", deadline)
	}
	if ctx.Err() != nil {
		t.Fatalf("interrupt context starts out cancelled: %v", ctx.Err())
	}
}

// TestUDPHelpDoesNotPromiseEscapes: the udp help once showed --data "\x00\x00",
// which reads as two zero bytes but is eight ordinary characters - the shell
// passes the backslashes through and shint sends the text as typed (a
// listener received `bytes=8 preview=\x00\x00`). The help must not suggest
// escapes work, and must say they are not interpreted.
func TestUDPHelpDoesNotPromiseEscapes(t *testing.T) {
	code, stdout, stderr := runShint(t, "udp", "--help")
	if code != 0 {
		t.Fatalf("udp --help exited %d: %s", code, stderr)
	}
	if strings.Contains(stdout, `\x00\x00`) {
		t.Errorf("udp --help still shows the misleading \\x00\\x00 example:\n%s", stdout)
	}
	if !strings.Contains(stdout, "not interpreted") {
		t.Errorf("udp --help should say that backslash escapes are not interpreted:\n%s", stdout)
	}
	if !strings.Contains(stdout, `--data "hello"`) {
		t.Errorf("udp --help should show a working --data example:\n%s", stdout)
	}
}

// startShint starts the CLI as a subprocess whose stdout can be watched while it
// runs, which runShint (it waits for the process to end) cannot do.
type liveShint struct {
	cmd *exec.Cmd
	mu  sync.Mutex
	buf bytes.Buffer
}

func startShint(t *testing.T, args ...string) *liveShint {
	t.Helper()
	l := &liveShint{cmd: exec.Command(os.Args[0], args...)}
	l.cmd.Env = append(os.Environ(), "SHINT_TEST_RUN_MAIN=1")
	pipe, err := l.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := pipe.Read(chunk)
			l.mu.Lock()
			l.buf.Write(chunk[:n])
			l.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { _ = l.cmd.Process.Kill() })
	return l
}

func (l *liveShint) output() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// waitFor blocks until the output contains want at least n times.
func (l *liveShint) waitFor(t *testing.T, want string, n int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for strings.Count(l.output(), want) < n {
		if time.Now().After(deadline) {
			t.Fatalf("waited for %d x %q; output so far:\n%s", n, want, l.output())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Ctrl+C on a repeating check ends the run the way it ends when it finishes: the
// statistics, the done line, and a line saying how far it got - and exit status
// 1, because the run was cut short. Not a bare "^C" and nothing else.
func TestCtrlCShowsTheSummary(t *testing.T) {
	web200 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer web200.Close()
	open := strconv.Itoa(tcpListener(t))
	udpEcho := strconv.Itoa(udpSocket(t, true))

	for _, tc := range []struct {
		name, module, progress string
		summary                []string // what ends the output besides the "interrupted" line
		args                   []string
	}{
		{"web", "web", "] OK response url=", []string{"web STATISTICS", "Requests sent: "}, []string{"web", web200.URL, "--count", "500", "--delay", "40"}},
		{"telnet", "telnet", "] OK connect ok", []string{"telnet STATISTICS", "Requests sent: "}, []string{"telnet", "127.0.0.1", open, "--count", "500", "--delay", "40"}},
		// udp has no statistics block; its summary is the done line
		{"udp", "udp", "] OK probe open", []string{"probes_sent="}, []string{"udp", "127.0.0.1", udpEcho, "--data", "x", "--count", "500", "--delay", "40"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := startShint(t, tc.args...)
			l.waitFor(t, tc.progress, 3)
			if err := l.cmd.Process.Signal(os.Interrupt); err != nil {
				t.Skipf("this platform cannot deliver an interrupt to a process: %v", err)
			}
			done := make(chan error, 1)
			go func() { done <- l.cmd.Wait() }()
			var err error
			select {
			case err = <-done:
			case <-time.After(15 * time.Second):
				t.Fatalf("shint did not end after Ctrl+C; output:\n%s", l.output())
			}
			code := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			out := l.output()
			if code != 1 {
				t.Errorf("exit status %d after Ctrl+C, want 1 (the run was cut short)\n%s", code, out)
			}
			wants := append([]string{"[" + tc.module + "] ERROR interrupted attempts_completed=", "attempts_planned=500", "[" + tc.module + "] OK done"}, tc.summary...)
			for _, want := range wants {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks %q:\n%s", want, out)
				}
			}
		})
	}
}
