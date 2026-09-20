package handlers

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// rawServer is a bare TCP (or TLS) HTTP responder that knows exactly how
// many bytes each request occupied on the wire and answers with a fixed raw
// response, so web's byte counts can be checked against ground truth
// rather than against another shint component.
type rawServer struct {
	url      *url.URL
	mu       sync.Mutex
	received []int
}

func startRawServer(t *testing.T, l net.Listener, scheme, response string) *rawServer {
	t.Helper()
	u, err := url.Parse(scheme + "://" + l.Addr().String() + "/")
	if err != nil {
		t.Fatalf("bad server url: %v", err)
	}
	rs := &rawServer{url: u}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				br := bufio.NewReader(c)
				total, contentLength := 0, 0
				for {
					line, err := br.ReadString('\n')
					total += len(line)
					if err != nil {
						return
					}
					if line == "\r\n" {
						break
					}
					if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(k, "content-length") {
						contentLength, _ = strconv.Atoi(strings.TrimSpace(v))
					}
				}
				n, _ := io.CopyN(io.Discard, br, int64(contentLength))
				total += int(n)
				rs.mu.Lock()
				rs.received = append(rs.received, total)
				rs.mu.Unlock()
				_, _ = c.Write([]byte(response))
			}(c)
		}
	}()
	t.Cleanup(func() { _ = l.Close() })
	return rs
}

func (rs *rawServer) requestBytes(t *testing.T) []int {
	t.Helper()
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]int(nil), rs.received...)
}

const rawResponse = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello"

func webStatsFromJSON(t *testing.T, out string) []lib.WebStats {
	t.Helper()
	var parsed struct {
		Stats []lib.WebStats `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to unmarshal web JSON: %v\noutput:\n%s", err, out)
	}
	return parsed.Stats
}

// TestWebHandlerBytesIncludeHeaders is the core of the measurement: what web
// reports as sent/received is everything on the wire (request line, headers
// and body out; status line, headers and body in), not just the bodies.
func TestWebHandlerBytesIncludeHeaders(t *testing.T) {
	const body = "abc"
	headers := []string{"X-Test: 1"}

	check := func(t *testing.T, rs *rawServer, tlsConfig *tls.Config) {
		jsonOutput, throttle := true, false
		out := captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, rs.url, "POST", body, headers, false, tlsConfig)
		})
		stats := webStatsFromJSON(t, out)
		if len(stats) != 1 {
			t.Fatalf("expected 1 stat, got %d\n%s", len(stats), out)
		}
		gotRequests := rs.requestBytes(t)
		if len(gotRequests) != 1 {
			t.Fatalf("server saw %d requests, want 1", len(gotRequests))
		}
		if stats[0].BytesSent != int64(gotRequests[0]) {
			t.Errorf("bytes_sent = %d, server actually read %d", stats[0].BytesSent, gotRequests[0])
		}
		if stats[0].BytesSent <= int64(len(body)) {
			t.Errorf("bytes_sent = %d does not include request headers (body alone is %d)", stats[0].BytesSent, len(body))
		}
		if stats[0].BytesReceived != int64(len(rawResponse)) {
			t.Errorf("bytes_received = %d, server actually sent %d", stats[0].BytesReceived, len(rawResponse))
		}
	}

	t.Run("http", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		check(t, startRawServer(t, l, "http", rawResponse), nil)
	})

	// Over TLS the count is taken on the decrypted side, so it must still
	// match the plaintext HTTP bytes the server saw, not the (larger)
	// encrypted stream.
	t.Run("https counts plaintext HTTP bytes", func(t *testing.T) {
		ca := generateTestCA(t)
		l, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{ca.ServerTLSCert}})
		if err != nil {
			t.Fatal(err)
		}
		tlsConfig, err := BuildTLSConfig(ca.CAFile, "", "", false)
		if err != nil {
			t.Fatal(err)
		}
		check(t, startRawServer(t, l, "https", rawResponse), tlsConfig)
	})
}

// TestWebHandlerLogLine covers the text-mode response line: one OK (the log
// level), no "response ok" / "200 OK" repeats, and the same sent/received
// figures the JSON carries.
func TestWebHandlerLogLine(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rs := startRawServer(t, l, "http", rawResponse)

	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, rs.url, "GET", "", nil, false, nil)
	})

	var line string
	for _, candidate := range strings.Split(out, "\n") {
		if strings.Contains(candidate, "[web]") && strings.Contains(candidate, "response") && strings.Contains(candidate, "status=") {
			line = candidate
		}
	}
	if line == "" {
		t.Fatalf("no response line in output:\n%s", out)
	}
	if n := strings.Count(line, "OK"); n != 1 {
		t.Errorf("response line has %d \"OK\"s, want just the log level:\n%s", n, line)
	}
	if !strings.Contains(line, "status=200 ") {
		t.Errorf("expected numeric status=200, got:\n%s", line)
	}
	m := regexp.MustCompile(`bytes_sent=(\d+) bytes_received=(\d+)`).FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("response line lacks bytes_sent=/bytes_received=:\n%s", line)
	}
	if sent, _ := strconv.Atoi(m[1]); sent != rs.requestBytes(t)[0] {
		t.Errorf("bytes_sent=%d, server actually read %d", sent, rs.requestBytes(t)[0])
	}
	if received, _ := strconv.Atoi(m[2]); received != len(rawResponse) {
		t.Errorf("bytes_received=%d, server actually sent %d", received, len(rawResponse))
	}
}

// TestWebHandlerReusedConnectionCountsPerRequest guards the per-request
// snapshotting: when a keep-alive connection serves two requests, the second
// must report its own bytes, not the connection's running total.
func TestWebHandlerReusedConnectionCountsPerRequest(t *testing.T) {
	var conns int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	server.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			atomic.AddInt64(&conns, 1)
		}
	}
	server.Start()
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)

	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, 2, 150, &throttle, 5, serverURL, "GET", "", nil, false, nil)
	})
	stats := webStatsFromJSON(t, out)
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}
	if got := atomic.LoadInt64(&conns); got != 1 {
		t.Fatalf("server saw %d connections, want 1 (test needs the connection to be reused)", got)
	}
	if stats[0].BytesSent == 0 || stats[0].BytesReceived == 0 {
		t.Fatalf("zero byte counts: %+v", stats[0])
	}
	if stats[0].BytesSent != stats[1].BytesSent || stats[0].BytesReceived != stats[1].BytesReceived {
		t.Errorf("identical requests on a reused connection reported different bytes: %d/%d vs %d/%d",
			stats[0].BytesSent, stats[0].BytesReceived, stats[1].BytesSent, stats[1].BytesReceived)
	}
}

// splitListenEvent separates the listener's single JSON line from web's
// indented JSON document when both were printed to the same captured stdout.
func splitListenEvent(t *testing.T, out string) (lib.HTTPListenEvent, string) {
	t.Helper()
	var event lib.HTTPListenEvent
	var web []string
	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, `{"method"`) {
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Fatalf("bad listener event %q: %v", line, err)
			}
			found = true
			continue
		}
		web = append(web, line)
	}
	if !found {
		t.Fatalf("no listener event in output:\n%s", out)
	}
	return event, strings.Join(web, "\n")
}

// TestWebAndHTTPListenAgreeOnBytes is the property the whole change is for:
// run web against listen http and the two sides report the same numbers -
// web's bytes_sent is the listener's bytes_received and vice versa - for
// requests with no body, small bodies, and bodies far past the 256KB net/http
// is willing to discard on its own.
func TestWebAndHTTPListenAgreeOnBytes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		body   int
	}{
		{"no body", "GET", 0},
		{"small body", "POST", 5},
		{"200KB body", "POST", 200_000},
		{"700KB body", "POST", 700_000},
		{"2MB body", "PUT", 2_000_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := freeTCPPort(t)
			serverURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port) + "/")
			listenJSON, webJSON, throttle := true, true, false
			body := strings.Repeat("x", tc.body)

			out := captureStdout(t, func() {
				done := make(chan struct{})
				go func() {
					HTTPListenHandler("127.0.0.1", port, 1, 10, &listenJSON)
					close(done)
				}()
				waitForListenerReady(t, port)
				WebHandler(context.Background(), &webJSON, 1, 0, &throttle, 10, serverURL, tc.method, body, []string{"X-Test: 1"}, false, nil)
				<-done
			})

			event, webOut := splitListenEvent(t, out)
			stats := webStatsFromJSON(t, webOut)
			if len(stats) != 1 {
				t.Fatalf("expected 1 web stat, got %d\n%s", len(stats), webOut)
			}
			if stats[0].BytesSent != event.BytesReceived {
				t.Errorf("web sent %d bytes but listen http received %d", stats[0].BytesSent, event.BytesReceived)
			}
			if stats[0].BytesReceived != event.BytesSent {
				t.Errorf("web received %d bytes but listen http sent %d", stats[0].BytesReceived, event.BytesSent)
			}
			// Headers are in the totals: a bodyless GET still has bytes.
			if event.BytesReceived <= int64(tc.body) || event.BytesSent <= int64(len(`{"status":"ok"}`)) {
				t.Errorf("counts look body-only: received=%d (body %d), sent=%d", event.BytesReceived, tc.body, event.BytesSent)
			}
		})
	}
}

// TestHTTPListenHandlerCountsRawBytes checks the listener's own figures
// against bytes written and read by a raw client, with no shint web client
// involved.
func TestHTTPListenHandlerCountsRawBytes(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := true

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("127.0.0.1", port, 1, 5, &jsonOutput)
		})
	}()
	waitForListenerReady(t, port)

	request := "POST /x HTTP/1.1\r\nHost: example.test\r\nContent-Length: 5\r\nX-Anything: at-all\r\n\r\nhello"
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	response, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	_ = conn.Close()

	select {
	case out := <-done:
		var event lib.HTTPListenEvent
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v\noutput:\n%s", err, out)
		}
		if event.BytesReceived != int64(len(request)) {
			t.Errorf("bytes_received = %d, client wrote %d", event.BytesReceived, len(request))
		}
		if event.BytesSent != int64(len(response)) {
			t.Errorf("bytes_sent = %d, client read %d", event.BytesSent, len(response))
		}
		if event.Method != "POST" || event.Path != "/x" || event.StatusCode != http.StatusNotFound {
			t.Errorf("unexpected event: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after its one request")
	}
}

// TestHTTPListenHandlerIgnoresRequestHeaders pins down that request headers
// are never acted on: conditional, range and content-negotiation headers
// change nothing, and the response carries no validator (ETag,
// Last-Modified) that could only be honored by inspecting them.
func TestHTTPListenHandlerIgnoresRequestHeaders(t *testing.T) {
	port := freeTCPPort(t)
	jsonOutput := false

	done := make(chan string, 1)
	go func() {
		done <- captureStdout(t, func() {
			HTTPListenHandler("127.0.0.1", port, 2, 5, &jsonOutput)
		})
	}()
	waitForListenerReady(t, port)

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	do := func(headers map[string]string) (*http.Response, string) {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}

	plain, plainBody := do(nil)
	loaded, loadedBody := do(map[string]string{
		"If-None-Match":     `"anything"`,
		"If-Modified-Since": "Mon, 01 Jan 2001 00:00:00 GMT",
		"Range":             "bytes=0-1",
		"Authorization":     "Bearer nope",
	})
	for name, resp := range map[string]*http.Response{"plain": plain, "loaded": loaded} {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", name, resp.StatusCode)
		}
		for _, h := range []string{"ETag", "Last-Modified", "Content-Range"} {
			if v := resp.Header.Get(h); v != "" {
				t.Errorf("%s: unexpected %s header %q", name, h, v)
			}
		}
	}
	if plainBody != loadedBody || plainBody != `{"status":"ok"}` {
		t.Errorf("bodies differ or unexpected: %q vs %q", plainBody, loadedBody)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("HTTPListenHandler did not stop after 2 requests")
	}
}

func TestCountingConnCountsBothDirectionsAndClosesOnce(t *testing.T) {
	client, server := net.Pipe()
	var hooks int32
	var finalRead, finalWritten int64
	cc := &countingConn{Conn: server}
	cc.onClose = func(c *countingConn) {
		atomic.AddInt32(&hooks, 1)
		finalRead, finalWritten = c.read.Load(), c.written.Load()
	}

	go func() {
		_, _ = client.Write([]byte("12345"))
		buf := make([]byte, 3)
		_, _ = io.ReadFull(client, buf)
	}()

	buf := make([]byte, 5)
	if _, err := io.ReadFull(cc, buf); err != nil {
		t.Fatal(err)
	}
	if _, err := cc.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	_ = cc.Close()
	_ = cc.Close()
	_ = client.Close()

	if finalRead != 5 || finalWritten != 3 {
		t.Errorf("counted read=%d written=%d, want 5 and 3", finalRead, finalWritten)
	}
	if got := atomic.LoadInt32(&hooks); got != 1 {
		t.Errorf("onClose ran %d times, want 1", got)
	}
}

// recordingListener wraps accepted connections in countingConns and keeps
// them, so a test can total what a server really read and wrote.
type recordingListener struct {
	net.Listener
	mu    sync.Mutex
	conns []*countingConn
}

func (l *recordingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	cc := &countingConn{Conn: c}
	l.mu.Lock()
	l.conns = append(l.conns, cc)
	l.mu.Unlock()
	return cc, nil
}

func (l *recordingListener) totals() (read, written int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.conns {
		read += c.read.Load()
		written += c.written.Load()
	}
	return read, written
}

// TestWebHandlerBytesCoverEveryRedirectHop: the client follows redirects, so
// the reported bytes must be the whole exchange - the redirect and the
// final request, on one connection or several - not just the last hop.
func TestWebHandlerBytesCoverEveryRedirectHop(t *testing.T) {
	newServer := func(h http.HandlerFunc) (*httptest.Server, *recordingListener) {
		s := httptest.NewUnstartedServer(h)
		rl := &recordingListener{Listener: s.Listener}
		s.Listener = rl
		s.Start()
		t.Cleanup(s.Close)
		return s, rl
	}

	final, finalListener := newServer(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello")) })

	for _, tc := range []struct {
		name string
		// where the first server's redirect points: its own /final (same
		// keep-alive connection) or the other server (a second connection)
		crossHost bool
	}{{"same connection", false}, {"different connection", true}} {
		t.Run(tc.name, func(t *testing.T) {
			var first *httptest.Server
			first, firstListener := newServer(func(w http.ResponseWriter, r *http.Request) {
				if tc.crossHost {
					http.Redirect(w, r, final.URL+"/final", http.StatusFound)
				} else if r.URL.Path == "/" {
					http.Redirect(w, r, first.URL+"/final", http.StatusFound)
				} else {
					_, _ = w.Write([]byte("hello"))
				}
			})
			serverURL, _ := url.Parse(first.URL + "/")

			jsonOutput, throttle := true, false
			out := captureStdout(t, func() {
				WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, serverURL, "GET", "", nil, false, nil)
			})
			stats := webStatsFromJSON(t, out)
			if len(stats) != 1 || stats[0].StatusCode != http.StatusOK {
				t.Fatalf("expected one 200 stat after following the redirect, got %+v", stats)
			}

			// Server-side counters settle a beat after the client is done.
			want := func() (int64, int64) {
				r1, w1 := firstListener.totals()
				r2, w2 := int64(0), int64(0)
				if tc.crossHost {
					r2, w2 = finalListener.totals()
				}
				return r1 + r2, w1 + w2
			}
			deadline := time.Now().Add(2 * time.Second)
			for {
				serverRead, serverWritten := want()
				if stats[0].BytesSent == serverRead && stats[0].BytesReceived == serverWritten {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("web reported sent=%d received=%d, servers read %d and wrote %d across the whole exchange",
						stats[0].BytesSent, stats[0].BytesReceived, serverRead, serverWritten)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
