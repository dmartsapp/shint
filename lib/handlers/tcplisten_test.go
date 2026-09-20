package handlers

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/lib"
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
