package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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

const listenTCPModule = "listen-tcp"

// previewBytes renders a short, single-line, human-safe preview of received
// bytes for logging: trailing newlines trimmed and long payloads truncated.
func previewBytes(b []byte) string {
	const maxPreview = 120
	s := strings.TrimRight(string(b), "\r\n")
	if len(s) > maxPreview {
		return s[:maxPreview] + "..."
	}
	return s
}

// TCPListenHandler starts a plain TCP listener so other shint commands
// (telnet, nmap) or third-party tools can be exercised against a local
// endpoint when no real server is available. It accepts up to maxConnections
// connections (0 = unlimited, run until Ctrl+C), logging each byte chunk
// received and optionally echoing it back to the client.
func TCPListenHandler(bind string, port int, echo bool, maxConnections int, idleTimeout int, jsonoutput *bool) {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "listen failed "+lib.Fields("address", addr, "error", err.Error()), true))
		os.Exit(1)
	}
	defer func() { _ = listener.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	if !*jsonoutput {
		limit := "unlimited"
		if maxConnections > 0 {
			limit = strconv.Itoa(maxConnections)
		}
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "listening "+lib.Fields("address", addr, "max_connections", limit, "echo", echo), false))
	}

	var wg sync.WaitGroup
	var connCount, totalBytesReceived, totalBytesSent int64
	istart := time.Now()
	for maxConnections <= 0 || connCount < int64(maxConnections) {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
			default:
				if !*jsonoutput {
					fmt.Println(lib.LogWithTimestamp(listenTCPModule, "accept failed "+lib.Fields("error", err.Error()), true))
				}
			}
			break
		}
		connCount++
		wg.Add(1)
		go func(conn net.Conn) {
			defer wg.Done()
			received, sent := handleTCPConnection(conn, echo, idleTimeout, jsonoutput)
			atomic.AddInt64(&totalBytesReceived, int64(received))
			atomic.AddInt64(&totalBytesSent, int64(sent))
		}(conn)
	}
	wg.Wait()

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "done "+lib.Fields("connections", connCount, "bytes_received", totalBytesReceived, "bytes_sent", totalBytesSent, "total_time", time.Since(istart)), false))
	}
}

// handleTCPConnection services one connection and returns the total bytes
// read from and written to it. Each read (and its optional echo write) is
// treated as one "request" for measurement purposes: the time between the
// read returning and the echo write completing is reported as that
// request's processing time.
func handleTCPConnection(conn net.Conn, echo bool, idleTimeout int, jsonoutput *bool) (received int, sent int) {
	defer func() { _ = conn.Close() }()
	remote := conn.RemoteAddr().String()
	local := conn.LocalAddr().String()
	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "connection accepted "+lib.Fields("remote", remote, "local", local), false))
	}
	buf := make([]byte, 4096)
	for {
		if idleTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(time.Duration(idleTimeout) * time.Second))
		}
		reqStart := time.Now()
		n, err := conn.Read(buf)
		if n > 0 {
			received += n
			preview := previewBytes(buf[:n])
			written := 0
			if echo {
				if wn, werr := conn.Write(buf[:n]); werr != nil {
					if !*jsonoutput {
						fmt.Println(lib.LogWithTimestamp(listenTCPModule, "echo failed "+lib.Fields("remote", remote, "error", werr.Error()), true))
					}
				} else {
					written = wn
					sent += wn
				}
			}
			processingTime := time.Since(reqStart)
			if *jsonoutput {
				event := lib.ListenEvent{
					Protocol:         "tcp",
					RemoteAddr:       remote,
					LocalAddr:        local,
					BytesRead:        n,
					BytesSent:        written,
					ProcessingTimeUs: processingTime.Microseconds(),
					Preview:          preview,
					UnixTimeUs:       time.Now().UnixMicro(),
				}
				js, _ := json.Marshal(event)
				fmt.Println(string(js))
			} else {
				fmt.Println(lib.LogWithTimestamp(listenTCPModule, "data received "+lib.Fields("remote", remote, "bytes_received", n, "bytes_sent", written, "time_taken", processingTime, "preview", preview), false))
			}
		}
		if err != nil {
			break
		}
	}
	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "connection closed "+lib.Fields("remote", remote, "bytes_received", received, "bytes_sent", sent), false))
	}
	return received, sent
}
