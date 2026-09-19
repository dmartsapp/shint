package handlers

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dmartsapp/shint/lib"
)

func TestHTTPListenHandlerIPv6Bind(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("::1", port, 1, 5, &jsonOutput)
		})
	}()

	waitForListenerReadyOn(t, "::1", port)

	resp, err := http.Get("http://[::1]:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after its one IPv6 request")
	}
}

func TestHTTPListenHandlerRootAllMethods(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("127.0.0.1", port, 3, 5, &jsonOutput)
		})
	}()

	base := "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	waitForListenerReady(t, port)

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		req, err := http.NewRequest(method, base, nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s / failed: %v", method, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s / status = %d, want 200", method, resp.StatusCode)
		}
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		_ = resp.Body.Close()
		if body["status"] != "ok" {
			t.Errorf("%s / body = %v, want status=ok", method, body)
		}
	}

	select {
	case out := <-done:
		if !strings.Contains(out, "method=GET") || !strings.Contains(out, "method=POST") || !strings.Contains(out, "method=DELETE") {
			t.Errorf("expected log lines for all three methods, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after 3 requests")
	}
}

func TestHTTPListenHandlerUnknownPathIs404(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("127.0.0.1", port, 1, 5, &jsonOutput)
		})
	}()

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForListenerReady(t, port)

	resp, err := http.Get(base + "/anything/else")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "not found" {
		t.Errorf("body = %v, want status=\"not found\"", body)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after its one request")
	}
}

func TestHTTPListenHandlerJSONEvents(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := true

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("127.0.0.1", port, 1, 5, &jsonOutput)
		})
	}()

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForListenerReady(t, port)

	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case out := <-done:
		var event lib.HTTPListenEvent
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v\noutput:\n%s", err, out)
		}
		if event.Method != http.MethodGet || event.Path != "/" || event.StatusCode != http.StatusOK {
			t.Errorf("unexpected event: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after its one request")
	}
}

func TestHTTPListenHandlerJSONEventMeasurements(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := true

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("127.0.0.1", port, 1, 5, &jsonOutput)
		})
	}()

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForListenerReady(t, port)

	reqBody := "hello-http-listener"
	resp, err := http.Post(base+"/", "text/plain", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	select {
	case out := <-done:
		var event lib.HTTPListenEvent
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v\noutput:\n%s", err, out)
		}
		if event.BytesReceived != int64(len(reqBody)) {
			t.Errorf("BytesReceived = %d, want %d", event.BytesReceived, len(reqBody))
		}
		if event.BytesSent != int64(len(respBody)) {
			t.Errorf("BytesSent = %d, want %d", event.BytesSent, len(respBody))
		}
		if event.ProcessingTimeUs <= 0 {
			t.Errorf("ProcessingTimeUs = %d, want > 0", event.ProcessingTimeUs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after its one request")
	}
}

func TestHTTPListenHandlerZeroCountRunsUntilInterrupted(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("127.0.0.1", port, 0, 5, &jsonOutput)
		})
	}()

	waitForListenerReady(t, port)

	select {
	case out := <-done:
		t.Fatalf("HTTPListenHandler with count=0 returned before being interrupted, output:\n%s", out)
	case <-time.After(300 * time.Millisecond):
	}

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("failed to signal SIGINT: %v", err)
	}

	select {
	case out := <-done:
		if !strings.Contains(out, "done") {
			t.Errorf("expected a done summary line after interrupt, got:\n%s", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not shut down within 5s of SIGINT")
	}
}

// waitForListenerReady polls the port with a raw TCP dial (never sending any
// bytes) until it accepts a connection or the timeout elapses, since
// HTTPListenHandler's listener binds asynchronously on its own goroutine.
// Deliberately not an HTTP request: that would itself count against
// maxRequests and throw off tests asserting an exact request count.
func waitForListenerReady(t *testing.T, port int) {
	t.Helper()
	waitForListenerReadyOn(t, "127.0.0.1", port)
}

func waitForListenerReadyOn(t *testing.T, host string, port int) {
	t.Helper()
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("listener on port %d did not become ready in time", port)
}
