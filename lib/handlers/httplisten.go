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

// HTTPListenHandler starts a minimal HTTP server for testing both plain TCP
// reachability and basic HTTP behavior against one endpoint: any method on
// "/" gets a small JSON status dict back, and every other path is a 404
// (also a JSON status dict, for the same reason the whole thing exists -
// nothing to parse differently depending on the outcome). Accepts up to
// maxRequests requests (0 = unlimited, run until Ctrl+C).
func HTTPListenHandler(bind string, port int, maxRequests int, idleTimeout int, jsonoutput *bool) {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))

	var requestCount int64
	done := make(chan struct{})

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
		w.WriteHeader(status)
		_, _ = w.Write(js)

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
		// observes n == maxRequests - safe to close(done) unconditionally
		// there without a sync.Once or risking a double-close.
		n := atomic.AddInt64(&requestCount, 1)
		if maxRequests > 0 && n == int64(maxRequests) {
			close(done)
		}
	})

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "listen failed "+lib.Fields("address", addr, "error", err.Error()), true))
		os.Exit(1)
	}

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

	select {
	case <-ctx.Done():
	case <-done:
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed && !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "server error "+lib.Fields("error", err.Error()), true))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenHTTPModule, "done "+lib.Fields("requests", atomic.LoadInt64(&requestCount), "total_time", time.Since(istart)), false))
	}
}
