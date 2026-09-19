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

	"github.com/dmartsapp/shint/lib"
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
	defer listener.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	if !*jsonoutput {
		limit := "unlimited"
		if maxConnections > 0 {
			limit = strconv.Itoa(maxConnections)
		}
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "listening "+lib.Fields("address", addr, "max_connections", limit, "echo", echo), false))
	}

	var wg sync.WaitGroup
	var connCount, totalBytes int64
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
			n := handleTCPConnection(conn, echo, idleTimeout, jsonoutput)
			atomic.AddInt64(&totalBytes, int64(n))
		}(conn)
	}
	wg.Wait()

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "done "+lib.Fields("connections", connCount, "bytes_received", totalBytes, "total_time", time.Since(istart)), false))
	}
}

func handleTCPConnection(conn net.Conn, echo bool, idleTimeout int, jsonoutput *bool) int {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	local := conn.LocalAddr().String()
	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "connection accepted "+lib.Fields("remote", remote, "local", local), false))
	}
	buf := make([]byte, 4096)
	total := 0
	for {
		if idleTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(time.Duration(idleTimeout) * time.Second))
		}
		n, err := conn.Read(buf)
		if n > 0 {
			total += n
			preview := previewBytes(buf[:n])
			if *jsonoutput {
				event := lib.ListenEvent{Protocol: "tcp", RemoteAddr: remote, LocalAddr: local, BytesRead: n, Preview: preview, UnixTimeUs: time.Now().UnixMicro()}
				js, _ := json.Marshal(event)
				fmt.Println(string(js))
			} else {
				fmt.Println(lib.LogWithTimestamp(listenTCPModule, "data received "+lib.Fields("remote", remote, "bytes", n, "preview", preview), false))
			}
			if echo {
				if _, werr := conn.Write(buf[:n]); werr != nil && !*jsonoutput {
					fmt.Println(lib.LogWithTimestamp(listenTCPModule, "echo failed "+lib.Fields("remote", remote, "error", werr.Error()), true))
				}
			}
		}
		if err != nil {
			break
		}
	}
	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenTCPModule, "connection closed "+lib.Fields("remote", remote, "bytes_total", total), false))
	}
	return total
}
