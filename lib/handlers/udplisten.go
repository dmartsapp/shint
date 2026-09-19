package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dmartsapp/shint/lib"
)

const listenUDPModule = "listen-udp"

// UDPListenHandler starts a UDP listener so the udp probe command (or any
// other UDP client) can be tested against a local endpoint. It accepts up to
// maxPackets datagrams (0 = unlimited, run until Ctrl+C), logging each one
// and optionally echoing it back to the sender.
func UDPListenHandler(bind string, port int, echo bool, maxPackets int, idleTimeout int, jsonoutput *bool) {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(listenUDPModule, "resolve failed "+lib.Fields("address", addr, "error", err.Error()), true))
		os.Exit(1)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(listenUDPModule, "listen failed "+lib.Fields("address", addr, "error", err.Error()), true))
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	if !*jsonoutput {
		limit := "unlimited"
		if maxPackets > 0 {
			limit = strconv.Itoa(maxPackets)
		}
		fmt.Println(lib.LogWithTimestamp(listenUDPModule, "listening "+lib.Fields("address", addr, "max_packets", limit, "echo", echo), false))
	}

	buf := make([]byte, 65535)
	var packetCount, totalBytes int64
	istart := time.Now()
	for maxPackets <= 0 || packetCount < int64(maxPackets) {
		if idleTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(time.Duration(idleTimeout) * time.Second))
		}
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
			default:
				if !*jsonoutput {
					fmt.Println(lib.LogWithTimestamp(listenUDPModule, "read failed "+lib.Fields("error", err.Error()), true))
				}
			}
			break
		}
		packetCount++
		totalBytes += int64(n)
		preview := previewBytes(buf[:n])
		if *jsonoutput {
			event := lib.ListenEvent{Protocol: "udp", RemoteAddr: remote.String(), LocalAddr: conn.LocalAddr().String(), BytesRead: n, Preview: preview, UnixTimeUs: time.Now().UnixMicro()}
			js, _ := json.Marshal(event)
			fmt.Println(string(js))
		} else {
			fmt.Println(lib.LogWithTimestamp(listenUDPModule, "packet received "+lib.Fields("remote", remote.String(), "bytes", n, "preview", preview), false))
		}
		if echo {
			if _, werr := conn.WriteToUDP(buf[:n], remote); werr != nil && !*jsonoutput {
				fmt.Println(lib.LogWithTimestamp(listenUDPModule, "echo failed "+lib.Fields("remote", remote.String(), "error", werr.Error()), true))
			}
		}
	}

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(listenUDPModule, "done "+lib.Fields("packets", packetCount, "bytes_received", totalBytes, "total_time", time.Since(istart)), false))
	}
}
