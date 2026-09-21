//go:build unix

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// runShintWithDescriptors runs the real CLI in a process that may open only
// `limit` files - what a container, a CI runner or a script that ran ulimit -n
// gives it - against servers in this test process, which are not held to that limit.
func runShintWithDescriptors(t *testing.T, limit int, args ...string) (code int, stdout string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	script := fmt.Sprintf(`ulimit -n %d && exec "$0" "$@"`, limit)
	cmd := exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", script, os.Args[0]}, args...)...)
	cmd.Env = append(os.Environ(), "SHINT_TEST_RUN_MAIN=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Skipf("cannot run the CLI under ulimit -n %d: %v", limit, err)
	}
	return code, out.String()
}

// acceptingTCPListener is a TCP server that accepts every connection and closes it.
// (tcpListener only listens: a connection nobody accepts stays in the kernel's backlog,
// which holds about 128, and then a burst of hundreds of connects loses a few.)
func acceptingTCPListener(t *testing.T) (port int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return l.Addr().(*net.TCPAddr).Port
}

// "--count N --delay 0" started every attempt at once, so a process that could open
// fewer than N files failed the surplus with "too many open files" and blamed the
// target (issue #31): 3000 attempts under ulimit -n 256 lost 2379 of them. Now the
// attempts wait their turn, and 700 attempts under 256 descriptors all succeed.
func TestManyAttemptsAtOnceWithFewDescriptors(t *testing.T) {
	const attempts, limit = 700, 256
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		_, _ = w.Write([]byte("ok"))
	}))
	defer web.Close()

	cases := []struct {
		name string
		args []string
		ok   string
	}{
		{"telnet", []string{"telnet", "127.0.0.1", strconv.Itoa(acceptingTCPListener(t))}, "connect ok"},
		{"web", []string{"web", web.URL}, "OK response"},
		{"udp", []string{"udp", "127.0.0.1", strconv.Itoa(udpSocket(t, true)), "--data", "x"}, "probe open"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append(c.args, "--count", strconv.Itoa(attempts), "--delay", "0", "--timeout", "20")
			code, out := runShintWithDescriptors(t, limit, args...)
			if got := strings.Count(out, c.ok); code != 0 || got != attempts || strings.Contains(out, "too many open files") {
				t.Errorf("exit %d, %d of %d attempts succeeded, 'too many open files' seen: %v", code, got, attempts, strings.Contains(out, "too many open files"))
			}
		})
	}
}
