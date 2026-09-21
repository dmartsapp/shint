package handlers

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

func TestTCPListenHandlerIPv6BindAcceptsAndEchoes(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			TCPListenHandler("::1", port, true, 1, 5, &jsonOutput)
		})
	}()

	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("tcp", net.JoinHostPort("::1", strconv.Itoa(port)))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to dial IPv6 listener: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte("hello6")); err != nil {
		t.Fatalf("failed to write to listener: %v", err)
	}
	reply := make([]byte, 6)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(reply)
	if err != nil {
		t.Fatalf("failed to read echo reply: %v", err)
	}
	if string(reply[:n]) != "hello6" {
		t.Errorf("echo reply = %q, want %q", reply[:n], "hello6")
	}
	_ = conn.Close()

	select {
	case out := <-done:
		if !strings.Contains(out, "connection accepted") {
			t.Errorf("expected connection-accepted log line, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TCPListenHandler did not return after accepting its one IPv6 connection")
	}
}

func TestTCPListenHandlerAcceptsAndEchoes(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			TCPListenHandler("127.0.0.1", port, true, 1, 5, &jsonOutput)
		})
	}()

	// Give the listener a moment to bind before dialing.
	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to dial listener: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("failed to write to listener: %v", err)
	}

	reply := make([]byte, 5)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(reply)
	if err != nil {
		t.Fatalf("failed to read echo reply: %v", err)
	}
	if string(reply[:n]) != "hello" {
		t.Errorf("echo reply = %q, want %q", reply[:n], "hello")
	}
	_ = conn.Close()

	select {
	case out := <-done:
		if !strings.Contains(out, "connection accepted") {
			t.Errorf("expected connection-accepted log line, got:\n%s", out)
		}
		if !strings.Contains(out, "data received") && !strings.Contains(out, "bytes=5") {
			t.Errorf("expected data-received log line with 5 bytes, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TCPListenHandler did not return after accepting its one connection")
	}
}

func TestTCPListenHandlerJSONEvent(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := true

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			TCPListenHandler("127.0.0.1", port, false, 1, 5, &jsonOutput)
		})
	}()

	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to dial listener: %v", err)
	}
	if _, err := conn.Write([]byte("json-test")); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	_ = conn.Close()

	select {
	case out := <-done:
		lines := strings.Split(strings.TrimSpace(out), "\n")
		found := false
		for _, line := range lines {
			var event lib.ListenEvent
			if err := json.Unmarshal([]byte(line), &event); err == nil {
				if event.Protocol == "tcp" && event.Preview == "json-test" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("expected a JSON listen event with preview %q, got:\n%s", "json-test", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TCPListenHandler did not return after accepting its one connection")
	}
}

func TestTCPListenHandlerJSONEventEchoMeasurements(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := true

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			TCPListenHandler("127.0.0.1", port, true, 1, 5, &jsonOutput)
		})
	}()

	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to dial listener: %v", err)
	}
	if _, err := conn.Write([]byte("measure-me")); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	reply := make([]byte, len("measure-me"))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(reply); err != nil {
		t.Fatalf("failed to read echo reply: %v", err)
	}
	_ = conn.Close()

	select {
	case out := <-done:
		lines := strings.Split(strings.TrimSpace(out), "\n")
		found := false
		for _, line := range lines {
			var event lib.ListenEvent
			if err := json.Unmarshal([]byte(line), &event); err == nil && event.Protocol == "tcp" && event.Preview == "measure-me" {
				found = true
				if event.BytesRead != len("measure-me") {
					t.Errorf("BytesRead = %d, want %d", event.BytesRead, len("measure-me"))
				}
				if event.BytesSent != len("measure-me") {
					t.Errorf("BytesSent = %d, want %d (echo was enabled)", event.BytesSent, len("measure-me"))
				}
				if event.ProcessingTimeUs <= 0 {
					t.Errorf("ProcessingTimeUs = %d, want > 0", event.ProcessingTimeUs)
				}
			}
		}
		if !found {
			t.Errorf("expected a JSON listen event with preview %q, got:\n%s", "measure-me", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TCPListenHandler did not return after accepting its one connection")
	}
}

func TestTCPListenHandlerZeroCountRunsUntilInterrupted(t *testing.T) {
	requireInterrupt(t)
	// count=0 means "unlimited": it must not return on its own, and must
	// shut down cleanly on SIGINT (the Ctrl+C path).
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			TCPListenHandler("127.0.0.1", port, false, 0, 1, &jsonOutput)
		})
	}()

	select {
	case out := <-done:
		t.Fatalf("TCPListenHandler with count=0 returned before being interrupted, output:\n%s", out)
	case <-time.After(300 * time.Millisecond):
		// Expected: still running.
	}

	sendInterrupt(t)

	select {
	case out := <-done:
		if !strings.Contains(out, "done") {
			t.Errorf("expected a done summary line after interrupt, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TCPListenHandler did not shut down within 5s of SIGINT")
	}
}

// runTCPListener starts a listener that takes one connection, lets client talk
// to it, and returns what the listener logged once the connection is over.
func runTCPListener(t *testing.T, echo bool, client func(net.Conn)) string {
	t.Helper()
	port := freeTCPPort(t)
	jsonOutput := false
	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() { TCPListenHandler("127.0.0.1", port, echo, 1, 5, &jsonOutput) })
	}()
	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		if conn, err = net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to dial listener: %v", err)
	}
	client(conn)
	_ = conn.Close()
	select {
	case out := <-done:
		return out
	case <-time.After(10 * time.Second):
		t.Fatal("TCPListenHandler did not return after its one connection")
		return ""
	}
}

func dataLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "data received") {
			lines = append(lines, l)
		}
	}
	return lines
}

// bytesIn adds up the bytes_received of the data lines.
func bytesIn(t *testing.T, lines []string) int {
	t.Helper()
	total := 0
	for _, l := range lines {
		i := strings.Index(l, " bytes_received=")
		if i < 0 {
			t.Fatalf("no bytes_received in %q", l)
		}
		field := strings.Fields(l[i+1:])[0]
		n, err := strconv.Atoi(strings.TrimPrefix(field, "bytes_received="))
		if err != nil {
			t.Fatal(err)
		}
		total += n
	}
	return total
}

// A transfer arrives in as many reads as the network hands over; one log line
// per read buried the result (issue #39). It is logged as a few lines, the
// totals still exact.
func TestTCPListenerLogsABulkTransferAsAFewLines(t *testing.T) {
	const size = 5_000_000
	out := runTCPListener(t, false, func(c net.Conn) {
		if _, err := c.Write(make([]byte, size)); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	lines := dataLines(out)
	if len(lines) == 0 || len(lines) > 30 {
		t.Errorf("a %d byte transfer logged %d data lines, want a handful:\n%.2000s", size, len(lines), out)
	}
	if got := bytesIn(t, lines); got != size {
		t.Errorf("the data lines add up to %d bytes, want %d", got, size)
	}
	if !strings.Contains(out, "reads=") {
		t.Errorf("a line that covers several reads should say how many (reads=N):\n%.1000s", out)
	}
	mustContain(t, out, "connection closed remote=", "bytes_received=5000000 bytes_sent=0", "done connections=1 bytes_received=5000000")
	if strings.LastIndex(out, "data received") > strings.Index(out, "connection closed") {
		t.Errorf("a data line came after the connection closed:\n%.1500s", out)
	}
}

// Data that arrives at different moments is still one line each, and a single
// read is logged exactly as it always was: no reads= field.
func TestTCPListenerKeepsSeparateMomentsSeparate(t *testing.T) {
	out := runTCPListener(t, false, func(c net.Conn) {
		_, _ = c.Write([]byte("first"))
		time.Sleep(3 * burstGap)
		_, _ = c.Write([]byte("second!"))
	})
	lines := dataLines(out)
	if len(lines) != 2 || !strings.Contains(lines[0], "bytes_received=5 ") || !strings.Contains(lines[0], "preview=first") ||
		!strings.Contains(lines[1], "bytes_received=7 ") || !strings.Contains(lines[1], "preview=second!") {
		t.Errorf("want two lines, 5 bytes then 7:\n%s", out)
	}
	if strings.Contains(out, "reads=") {
		t.Errorf("a single read must not say reads=:\n%s", out)
	}
}

// Writes that follow each other closely are one line - the first read's preview,
// the sum of the bytes - unless the network delivers them a moment apart.
func TestTCPListenerMergesWritesThatFollowEachOther(t *testing.T) {
	out := runTCPListener(t, true, func(c net.Conn) {
		for i := 0; i < 5; i++ {
			_, _ = c.Write([]byte("chunk"))
		}
		buf := make([]byte, 64)
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		got := 0
		for got < 25 {
			n, err := c.Read(buf)
			if err != nil {
				break
			}
			got += n
		}
	})
	lines := dataLines(out)
	if len(lines) == 0 || len(lines) > 2 || bytesIn(t, lines) != 25 {
		t.Errorf("five quick writes of 5 bytes should be one line (at most two): %d lines, %d bytes\n%s", len(lines), bytesIn(t, lines), out)
	}
	if !strings.Contains(lines[0], "preview=chunk") || !strings.Contains(out, "bytes_received=25 bytes_sent=25") {
		t.Errorf("preview and echo totals:\n%s", out)
	}
}

// --json is unchanged: an event per read, for whoever consumes them.
func TestTCPListenerJSONStaysPerRead(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := true
	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() { TCPListenHandler("127.0.0.1", port, false, 1, 5, &jsonOutput) })
	}()
	var conn net.Conn
	var err error
	for i := 0; i < 50; i++ {
		if conn, err = net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Write(make([]byte, 100_000))
	_ = conn.Close()
	out := <-done
	events := 0
	total := 0
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		var ev lib.ListenEvent
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			t.Fatalf("not a JSON event: %q", l)
		}
		events++
		total += ev.BytesRead
	}
	if events < 100_000/4096 || total != 100_000 {
		t.Errorf("%d events for %d bytes: --json must still report every read", events, total)
	}
}
