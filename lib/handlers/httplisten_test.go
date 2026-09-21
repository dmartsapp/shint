package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
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
		// Both figures are wire bytes - headers included - so each is
		// strictly more than the body alone (exact figures are checked
		// against a raw client in TestHTTPListenHandlerCountsRawBytes).
		if event.BytesReceived <= int64(len(reqBody)) {
			t.Errorf("BytesReceived = %d, want more than the %d-byte body (headers count too)", event.BytesReceived, len(reqBody))
		}
		if event.BytesSent <= int64(len(respBody)) {
			t.Errorf("BytesSent = %d, want more than the %d-byte body (headers count too)", event.BytesSent, len(respBody))
		}
		if event.ProcessingTimeUs <= 0 {
			t.Errorf("ProcessingTimeUs = %d, want > 0", event.ProcessingTimeUs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after its one request")
	}
}

func TestHTTPListenHandlerZeroCountRunsUntilInterrupted(t *testing.T) {
	requireInterrupt(t)
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

	sendInterrupt(t)

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

// runHTTPListener starts the listener for maxRequests requests, lets client talk to
// it over raw connections, and returns everything it printed once it stops.
func runHTTPListener(t *testing.T, maxRequests, idleTimeout int, asJSON bool, client func(port int)) string {
	t.Helper()
	port := freeTCPPort(t)
	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() { HTTPListenHandler("127.0.0.1", port, maxRequests, idleTimeout, &asJSON) })
	}()
	waitForListenerReady(t, port)
	client(port)
	select {
	case out := <-done:
		return out
	case <-time.After(15 * time.Second):
		t.Fatal("HTTPListenHandler did not stop by itself: a request it was sent was not counted")
		return ""
	}
}

// rawExchange sends payload on a fresh connection and returns what came back until
// the connection ends.
func rawExchange(t *testing.T, port int, payload string) string {
	t.Helper()
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	_, _ = conn.Write([]byte(payload))
	var got strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			return got.String()
		}
	}
}

// A request net/http cannot parse never reached the handler, so it was neither logged
// nor counted (issue #35): a listener told to take one request waited forever for a
// "real" one, and a client sending garbage - the very case someone is diagnosing -
// left no trace.
func TestHTTPListenerLogsAndCountsARequestItCannotParse(t *testing.T) {
	var reply string
	out := runHTTPListener(t, 1, 5, false, func(port int) { reply = rawExchange(t, port, "GARBAGE\r\n\r\n") })
	if !strings.HasPrefix(reply, "HTTP/1.1 400") {
		t.Errorf("the client should be answered 400, got %q", reply)
	}
	mustContain(t, out, "[listen-http] ERROR request rejected status=400 remote=127.0.0.1:", "bytes_received=11 ", `error="the request could not be read: answered 400 Bad Request"`, "[listen-http] OK done requests=1 ")
	if strings.Contains(out, "OK request") {
		t.Errorf("a request that could not be read is not an OK request:\n%s", out)
	}
}

// A connection that sends nothing - what telnet and nmap do - is not a request.
func TestHTTPListenerBareConnectionsAreNotRequests(t *testing.T) {
	out := runHTTPListener(t, 1, 5, false, func(port int) {
		for i := 0; i < 3; i++ {
			c, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
			if err != nil {
				t.Fatal(err)
			}
			_ = c.Close()
		}
		time.Sleep(100 * time.Millisecond)
		rawExchange(t, port, "GET / HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")
	})
	if strings.Contains(out, "rejected") || strings.Contains(out, "incomplete") || strings.Count(out, "request method=") != 1 {
		t.Errorf("only the one real request should be reported:\n%s", out)
	}
	mustContain(t, out, "done requests=1 ")
}

// A body that never completes was logged as "OK request ... status=404 bytes_sent=0"
// although the client got nothing (issue #34).
func TestHTTPListenerReportsARequestWhoseBodyNeverCompletes(t *testing.T) {
	var reply string
	out := runHTTPListener(t, 1, 1, false, func(port int) {
		reply = rawExchange(t, port, "POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: 1000\r\n\r\nshort")
	})
	if reply != "" {
		t.Errorf("nothing is owed to a client that stopped sending, got %q", reply)
	}
	mustContain(t, out, "[listen-http] ERROR request incomplete method=POST path=/up remote=127.0.0.1:", "bytes_received=57 bytes_sent=0 ", "the client did not finish sending the request", "done requests=1 ")
	if strings.Contains(out, "OK request") || strings.Contains(out, "status=404") {
		t.Errorf("the request must not be logged as answered:\n%s", out)
	}
}

// Framing that cannot be parsed - discovered by the handler, while it reads the body -
// is answered 400 and reported like the requests net/http rejects itself.
func TestHTTPListenerAnswersA400ForABodyItCannotParse(t *testing.T) {
	var reply string
	out := runHTTPListener(t, 1, 5, false, func(port int) {
		reply = rawExchange(t, port, "POST /c HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\nzz\r\nhello\r\n0\r\n\r\n")
	})
	if !strings.HasPrefix(reply, "HTTP/1.1 400") {
		t.Errorf("the client should be answered 400, got %q", reply)
	}
	mustContain(t, out, "[listen-http] ERROR request rejected method=POST path=/c status=400 ", "the request body could not be read: invalid byte in chunk length", "done requests=1 ")
}

func TestHTTPListenerJSONEventsForRequestsThatWereNotServed(t *testing.T) {
	out := runHTTPListener(t, 2, 1, true, func(port int) {
		rawExchange(t, port, "GARBAGE\r\n\r\n")
		rawExchange(t, port, "POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: 1000\r\n\r\nshort")
	})
	var events []lib.HTTPListenEvent
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		var ev lib.HTTPListenEvent
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			t.Fatalf("not a JSON event: %q", l)
		}
		events = append(events, ev)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d:\n%s", len(events), out)
	}
	if events[0].StatusCode != 400 || events[0].Error == "" || events[0].BytesSent == 0 {
		t.Errorf("rejected event: %+v", events[0])
	}
	if events[1].StatusCode != 0 || events[1].Method != "POST" || events[1].Path != "/up" || !strings.Contains(events[1].Error, "did not finish") || events[1].BytesSent != 0 {
		t.Errorf("incomplete event: %+v", events[1])
	}
}

func TestStatusFromHeadAndIsClientGone(t *testing.T) {
	for head, want := range map[string]int{"HTTP/1.1 400 Bad Request": 400, "HTTP/1.0 431 Req": 431, "HTTP/1.1 200": 200, "": 0, "HTTP/1.1 4": 0, "GET / HTTP/1.1": 0, "HTTP/1.1 xyz Bad": 0} {
		if got := statusFromHead([]byte(head)); got != want {
			t.Errorf("statusFromHead(%q) = %d, want %d", head, got, want)
		}
	}
	gone := []error{io.ErrUnexpectedEOF, io.EOF, net.ErrClosed, fmt.Errorf("read tcp: %w", syscall.ECONNRESET), &net.OpError{Op: "read", Err: os.ErrDeadlineExceeded}}
	for _, err := range gone {
		if !isClientGone(err) {
			t.Errorf("isClientGone(%v) = false, want true", err)
		}
	}
	for _, err := range []error{errors.New("invalid byte in chunk length"), errors.New("malformed chunked encoding"), errors.New("http: request body too large")} {
		if isClientGone(err) {
			t.Errorf("isClientGone(%v) = true: framing that cannot be parsed is answered 400", err)
		}
	}
}
