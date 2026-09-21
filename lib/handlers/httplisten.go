package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

const listenHTTPModule = "listen-http"

// httpExitGrace is a small pause after the server has (logically) stopped
// serving, before HTTPListenHandler returns and the process can exit. Go's
// runtime does not wait for other goroutines when main() returns: net/http
// flushes a handler's buffered response and closes the connection on its
// own internal per-connection goroutine, and finishing that is not
// synchronized with Server.Shutdown returning tightly enough to rule out
// the process exiting a beat too early and dropping the last response
// before the kernel has actually put its bytes on the wire. This grace
// period is the cheap, well-established way to close that window.
const httpExitGrace = 250 * time.Millisecond

// exchangeKey is the context key under which each connection's httpExchange
// travels from Server.ConnContext to the handler.
type exchangeKey struct{}

// httpExchange is the one request a connection served, as far as the
// listener cares: the request line's method and path, the status it
// answered with, and who asked. The connection's close hook reads it to
// report the request together with the connection's final byte counts.
// Request headers and body are deliberately absent - the listener never
// looks at them, it only counts their bytes.
type httpExchange struct {
	mu   sync.Mutex
	info httpExchangeInfo
}

type httpExchangeInfo struct {
	served bool
	// outcome is set for a request that was seen but not served: "incomplete"
	// (the client never finished it and nothing was answered) or "rejected" (it
	// was answered 400 because it could not be read). err says why.
	outcome string
	err     string
	method  string
	path    string
	status  int
	remote  string
	// processing is how long the handler took, from entering it to the
	// response being flushed.
	processing time.Duration
}

func (e *httpExchange) record(method, path string, status int, remote string, processing time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.info = httpExchangeInfo{served: true, method: method, path: path, status: status, remote: remote, processing: processing}
}

// fail records that the request this connection carried was not served. The peer
// is kept from the moment the connection was accepted.
func (e *httpExchange) fail(outcome, method, path string, status int, err string, processing time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.info.outcome, e.info.method, e.info.path, e.info.status, e.info.err, e.info.processing = outcome, method, path, status, err, processing
}

func (e *httpExchange) snapshot() httpExchangeInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.info
}

// HTTPListenHandler starts a minimal HTTP server for testing both plain TCP
// reachability and basic HTTP behavior against one endpoint: any method on
// "/" gets a small JSON status dict back, and every other path is a 404
// (also a JSON status dict, for the same reason the whole thing exists -
// nothing to parse differently depending on the outcome). Accepts up to
// maxRequests requests (0 = unlimited, run until Ctrl+C).
//
// Only the request line (method and path) is used, for routing and logging.
// Request headers are never parsed, inspected, or acted on (no conditional
// requests, no content negotiation), and the request body is never looked at
// or used (no echo) - it is read and thrown away, see the handler. What the
// listener does with all of it is count: each request is reported with the
// raw bytes that crossed its connection in both directions - headers and
// body included - measured the same way "web" measures its own
// bytes_sent/bytes_received, so the two sides' figures line up.
func HTTPListenHandler(bind string, port int, maxRequests int, idleTimeout int, jsonoutput *bool) {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "listen failed "+lib.Fields("address", addr, "error", err.Error()), true))
		os.Exit(1)
	}

	var requestCount, totalReceived, totalSent int64
	budgetReached := make(chan struct{})

	// countRequest counts one request toward --count. atomic.AddInt64 hands each
	// concurrent caller a distinct, monotonically increasing value, so exactly one
	// goroutine ever observes n == maxRequests - safe to close(budgetReached)
	// unconditionally there without a sync.Once or a double-close.
	countRequest := func() {
		n := atomic.AddInt64(&requestCount, 1)
		if maxRequests > 0 && n == int64(maxRequests) {
			close(budgetReached)
		}
	}

	// reportExchange runs when a connection closes, i.e. once net/http has
	// finished with it and the byte counts are final: reading any request
	// body it discards after the handler returns, writing the response's
	// last bytes. Reporting from the handler instead would undercount both.
	//
	// Three things can have happened on the connection. A request was served: it
	// is reported (the handler has already counted it). Nothing at all arrived - a
	// bare TCP probe from telnet or nmap - and it is not a request, so nothing is
	// said. Or bytes arrived and no request was served: the client never finished
	// it (incomplete: nothing was answered), or net/http could not read it and
	// answered 400 itself, or the handler could not read its body (rejected). Those
	// were requests somebody sent, and they used to vanish: not logged, not counted,
	// so a --count 1 listener waited forever for a "real" request. They are logged
	// as ERROR lines, with what is known, and counted.
	reportExchange := func(ex *httpExchange, received, sent int64, head []byte) {
		e := ex.snapshot()
		if !e.served {
			if received == 0 && e.outcome == "" {
				return // e.g. a bare TCP probe (telnet/nmap) that never sent a request
			}
			if e.outcome == "" { // it never reached the handler
				if status := statusFromHead(head); status > 0 {
					e.outcome, e.status = "rejected", status
					e.err = "the request could not be read: answered " + strconv.Itoa(status) + " " + http.StatusText(status)
				} else {
					e.outcome = "incomplete"
					e.err = "the connection ended before a complete request arrived, and nothing was answered"
				}
			}
			countRequest()
		}
		atomic.AddInt64(&totalReceived, received)
		atomic.AddInt64(&totalSent, sent)

		if *jsonoutput {
			event := lib.HTTPListenEvent{
				Method:           e.method,
				Path:             e.path,
				StatusCode:       e.status,
				RemoteAddr:       e.remote,
				BytesReceived:    received,
				BytesSent:        sent,
				ProcessingTimeUs: e.processing.Microseconds(),
				UnixTimeUs:       time.Now().UnixMicro(),
				Error:            e.err,
			}
			eventJS, _ := json.Marshal(event)
			fmt.Println(string(eventJS))
			return
		}
		if e.served {
			fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "request "+lib.Fields("method", e.method, "path", e.path, "status", e.status, "remote", e.remote, "bytes_received", received, "bytes_sent", sent, "time_taken", e.processing), false))
			return
		}
		fields := []any{}
		if e.method != "" {
			fields = append(fields, "method", e.method, "path", e.path)
		}
		if e.status > 0 {
			fields = append(fields, "status", e.status)
		}
		fields = append(fields, "remote", e.remote, "bytes_received", received, "bytes_sent", sent, "error", e.err)
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "request "+e.outcome+" "+lib.Fields(fields...), true))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqStart := time.Now()

		// Read the request body and throw it away unseen, before answering.
		// Nothing is parsed or acted on; this is only about the byte count
		// and the client: net/http would otherwise discard at most 256KB of
		// an unread body on its own, and more importantly Go's HTTP client
		// abandons a still-running upload the moment a complete
		// "Connection: close" response arrives - so answering first would cut
		// a large body short and both sides' counts with it.
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			// The body could not be read to its end, so the request was not served: it
			// was logged as "OK request ... status=404" although the client got nothing
			// (issue #34). A client that stalled or went away is owed no answer;
			// framing that cannot be parsed is answered 400. Either way it is reported
			// (see reportExchange) and counted.
			ex, _ := r.Context().Value(exchangeKey{}).(*httpExchange)
			since := time.Since(reqStart)
			if isClientGone(err) {
				if ex != nil {
					ex.fail("incomplete", r.Method, r.URL.Path, 0, "the client did not finish sending the request: "+shortErr(err), since)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":"bad request"}`))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			if ex != nil {
				ex.fail("rejected", r.Method, r.URL.Path, http.StatusBadRequest, "the request body could not be read: "+err.Error(), since)
			}
			return
		}

		status := http.StatusOK
		body := map[string]string{"status": "ok"}
		if r.URL.Path != "/" {
			status = http.StatusNotFound
			body = map[string]string{"status": "not found"}
		}

		// Drain and count the request body: nothing in the response depends
		// on it, but doing so both measures bytes received and lets net/http
		// reuse the connection's read side cleanly instead of abandoning an
		// unread body.

		js, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		// Tell net/http to close this connection right after the response
		// instead of leaving it idle for possible reuse: a deterministic,
		// immediate close (rather than waiting for Shutdown's idle-conn
		// sweep) shrinks the window between "response handed to net/http"
		// and "connection actually gone" that the code below has to wait out.
		// It also makes every connection carry exactly one request, so the
		// connection's byte counts are that request's byte counts.
		w.Header().Set("Connection", "close")
		w.WriteHeader(status)
		_, _ = w.Write(js)
		// Push the response out to the connection now rather than leaving
		// it to whatever's left of net/http's internal buffering: the
		// caller (below) may decide this was the last request and start
		// shutting the server down as soon as this handler returns, so the
		// bytes should already be past our own userspace buffer by then.
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		processingTime := time.Since(reqStart)

		if ex, ok := r.Context().Value(exchangeKey{}).(*httpExchange); ok {
			ex.record(r.Method, r.URL.Path, status, r.RemoteAddr, processingTime)
		}

		countRequest()
	})

	server := &http.Server{
		Handler: mux,
		// Each accepted connection gets its own exchange record, and its
		// close hook (see reportExchange) - set here, before net/http reads
		// a single byte from the connection.
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			ex := &httpExchange{}
			ex.info.remote = c.RemoteAddr().String()
			if cc, ok := c.(*countingConn); ok {
				cc.onClose = func(cc *countingConn) { reportExchange(ex, cc.read.Load(), cc.written.Load(), cc.firstWritten()) }
			}
			return context.WithValue(ctx, exchangeKey{}, ex)
		},
	}
	if idleTimeout > 0 {
		d := time.Duration(idleTimeout) * time.Second
		server.ReadTimeout = d
		server.WriteTimeout = d
		server.IdleTimeout = d
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !*jsonoutput {
		limit := "unlimited"
		if maxRequests > 0 {
			limit = strconv.Itoa(maxRequests)
		}
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "listening "+lib.Fields("address", addr, "max_requests", limit), false))
	}

	istart := time.Now()
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(countingListener{listener}) }()

	var triggeredShutdown bool
	select {
	case <-ctx.Done():
	case <-budgetReached:
		triggeredShutdown = true
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed && !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "server error "+lib.Fields("error", err.Error()), true))
		}
	}

	if triggeredShutdown || ctx.Err() != nil {
		// Server.Shutdown blocks until every in-flight handler - including
		// the one that just closed budgetReached above - has finished
		// writing its response and its connection has closed (which is also
		// when reportExchange prints that request), or the grace period
		// below elapses first.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = server.Shutdown(shutdownCtx)
		cancel()
		<-serverErr
	}

	// See httpExitGrace: belt-and-suspenders margin against the process
	// exiting a moment before the OS has actually sent the last response.
	time.Sleep(httpExitGrace)

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "done "+lib.Fields("requests", atomic.LoadInt64(&requestCount), "bytes_received", atomic.LoadInt64(&totalReceived), "bytes_sent", atomic.LoadInt64(&totalSent), "total_time", time.Since(istart)), false))
	}
}

// statusFromHead reads the status code off the start of an HTTP response
// ("HTTP/1.1 400 Bad Request"), or 0 if it does not begin like one.
func statusFromHead(head []byte) int {
	s := string(head)
	if len(s) < 12 || !strings.HasPrefix(s, "HTTP/") || s[8] != ' ' {
		return 0
	}
	n, err := strconv.Atoi(s[9:12])
	if err != nil {
		return 0
	}
	return n
}

// isClientGone reports whether reading a request body failed because the client
// stalled past the timeout or went away - as opposed to sending something that
// cannot be parsed - so that no answer is owed.
func isClientGone(err error) bool {
	var netErr net.Error
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF), errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		return true
	case errors.As(err, &netErr) && netErr.Timeout():
		return true
	}
	return false
}
