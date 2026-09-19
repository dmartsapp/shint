package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

// HTTPListenHandler starts a minimal HTTP server for testing both plain TCP
// reachability and basic HTTP behavior against one endpoint: any method on
// "/" gets a small JSON status dict back, and every other path is a 404
// (also a JSON status dict, for the same reason the whole thing exists -
// nothing to parse differently depending on the outcome). Accepts up to
// maxRequests requests (0 = unlimited, run until Ctrl+C).
func HTTPListenHandler(bind string, port int, maxRequests int, idleTimeout int, jsonoutput *bool) {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "listen failed "+lib.Fields("address", addr, "error", err.Error()), true))
		os.Exit(1)
	}

	var requestCount int64
	budgetReached := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		body := map[string]string{"status": "ok"}
		if r.URL.Path != "/" {
			status = http.StatusNotFound
			body = map[string]string{"status": "not found"}
		}

		js, _ := json.Marshal(body)
		w.Header().Set("Content-Type", "application/json")
		// Tell net/http to close this connection right after the response
		// instead of leaving it idle for possible reuse: a deterministic,
		// immediate close (rather than waiting for Shutdown's idle-conn
		// sweep) shrinks the window between "response handed to net/http"
		// and "connection actually gone" that the code below has to wait out.
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

		if *jsonoutput {
			event := lib.HTTPListenEvent{
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: status,
				RemoteAddr: r.RemoteAddr,
				UnixTimeUs: time.Now().UnixMicro(),
			}
			eventJS, _ := json.Marshal(event)
			fmt.Println(string(eventJS))
		} else {
			fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "request "+lib.Fields("method", r.Method, "path", r.URL.Path, "status", status, "remote", r.RemoteAddr), false))
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

	server := &http.Server{Handler: mux}
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
	go func() { serverErr <- server.Serve(listener) }()

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
		// writing its response and its connection has closed, or the grace
		// period below elapses first.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = server.Shutdown(shutdownCtx)
		cancel()
		<-serverErr
	}

	// See httpExitGrace: belt-and-suspenders margin against the process
	// exiting a moment before the OS has actually sent the last response.
	time.Sleep(httpExitGrace)

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "done "+lib.Fields("requests", atomic.LoadInt64(&requestCount), "total_time", time.Since(istart)), false))
	}
}
