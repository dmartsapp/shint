package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dmartsapp/shint/lib"
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
	method string
	path   string
	status int
	remote string
	// processing is how long the handler took, from entering it to the
	// response being flushed.
	processing time.Duration
}

func (e *httpExchange) record(method, path string, status int, remote string, processing time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.info = httpExchangeInfo{served: true, method: method, path: path, status: status, remote: remote, processing: processing}
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

	// reportExchange runs when a connection closes, i.e. once net/http has
	// finished with it and the byte counts are final: reading any request
	// body it discards after the handler returns, writing the response's
	// last bytes. Reporting from the handler instead would undercount both.
	reportExchange := func(ex *httpExchange, received, sent int64) {
		e := ex.snapshot()
		if !e.served {
			return // e.g. a bare TCP probe (telnet/nmap) that never sent a request
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
			}
			eventJS, _ := json.Marshal(event)
			fmt.Println(string(eventJS))
		} else {
			fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "request "+lib.Fields("method", e.method, "path", e.path, "status", e.status, "remote", e.remote, "bytes_received", received, "bytes_sent", sent, "time_taken", e.processing), false))
		}
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
		_, _ = io.Copy(io.Discard, r.Body)

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

		// atomic.AddInt64 hands each concurrent caller a distinct,
		// monotonically increasing value, so exactly one goroutine ever
		// observes n == maxRequests - safe to close(budgetReached)
		// unconditionally there without a sync.Once or a double-close.
		n := atomic.AddInt64(&requestCount, 1)
		if maxRequests > 0 && n == int64(maxRequests) {
			close(budgetReached)
		}
	})

	server := &http.Server{
		Handler: mux,
		// Each accepted connection gets its own exchange record, and its
		// close hook (see reportExchange) - set here, before net/http reads
		// a single byte from the connection.
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			ex := &httpExchange{}
			if cc, ok := c.(*countingConn); ok {
				cc.onClose = func(cc *countingConn) { reportExchange(ex, cc.read.Load(), cc.written.Load()) }
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
