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

func TestUDPListenHandlerIPv6BindReceivesAndEchoes(t *testing.T) {
	port := freeUDPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			UDPListenHandler("::1", port, true, 1, 5, &jsonOutput)
		})
	}()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("udp", net.JoinHostPort("::1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("failed to dial IPv6 UDP listener: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("hello6")); err != nil {
		t.Fatalf("failed to write: %v", err)
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

	select {
	case out := <-done:
		if !strings.Contains(out, "packet received") {
			t.Errorf("expected packet-received log line, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UDPListenHandler did not return after accepting its one IPv6 packet")
	}
}

func TestUDPListenHandlerReceivesAndEchoes(t *testing.T) {
	port := freeUDPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			UDPListenHandler("127.0.0.1", port, true, 1, 5, &jsonOutput)
		})
	}()
	time.Sleep(100 * time.Millisecond) // let the listener bind

	conn, err := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("failed to dial UDP listener: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("failed to write: %v", err)
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

	select {
	case out := <-done:
		if !strings.Contains(out, "packet received") {
			t.Errorf("expected packet-received log line, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UDPListenHandler did not return after accepting its one packet")
	}
}

func TestUDPListenHandlerJSONEvent(t *testing.T) {
	port := freeUDPPort(t)
	jsonOutput := true

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			UDPListenHandler("127.0.0.1", port, false, 1, 5, &jsonOutput)
		})
	}()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("failed to dial UDP listener: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("json-udp")); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	select {
	case out := <-done:
		var event lib.ListenEvent
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &event); err != nil {
			t.Fatalf("failed to unmarshal listen event: %v\noutput:\n%s", err, out)
		}
		if event.Protocol != "udp" || event.Preview != "json-udp" {
			t.Errorf("unexpected event: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UDPListenHandler did not return after accepting its one packet")
	}
}

func TestUDPListenHandlerZeroCountRunsUntilInterrupted(t *testing.T) {
	requireInterrupt(t)
	port := freeUDPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			UDPListenHandler("127.0.0.1", port, false, 0, 1, &jsonOutput)
		})
	}()

	select {
	case out := <-done:
		t.Fatalf("UDPListenHandler with count=0 returned before being interrupted, output:\n%s", out)
	case <-time.After(300 * time.Millisecond):
	}

	sendInterrupt(t)

	select {
	case out := <-done:
		if !strings.Contains(out, "done") {
			t.Errorf("expected a done summary line after interrupt, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UDPListenHandler did not shut down within 5s of SIGINT")
	}
}
