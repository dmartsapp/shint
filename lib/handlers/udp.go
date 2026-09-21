package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/shint/lib"
)

const udpModule = "udp"

// probeUDP sends a single UDP datagram to ip:port and classifies the result
// the way a basic UDP port scanner does, since UDP has no handshake to
// confirm a listener is present:
//   - "open": a reply datagram was received before the timeout.
//   - "closed": the OS surfaced an ICMP port-unreachable (read fails with a
//     "connection refused" style error) for the connected socket.
//   - "open|filtered": nothing came back before the timeout. This is the
//     common case for UDP services that don't reply to unexpected input, so
//     it cannot be told apart from a firewall silently dropping the packet.
func probeUDP(ctx context.Context, ip string, port int, timeout int, payload []byte) (state string, received []byte, err error) {
	dialer := net.Dialer{Timeout: time.Duration(timeout) * time.Second}
	conn, dialErr := dialer.DialContext(ctx, "udp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if dialErr != nil {
		return "error", nil, dialErr
	}
	defer func() { _ = conn.Close() }()
	defer watchCancel(ctx, conn)() // Ctrl+C ends a wait for a reply at once

	if _, werr := conn.Write(payload); werr != nil {
		return "error", nil, werr
	}

	if ctx.Err() != nil {
		return "error", nil, ctx.Err()
	}
	buf := make([]byte, 2048)
	if derr := conn.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Second)); derr != nil {
		return "error", nil, derr
	}
	n, rerr := conn.Read(buf)
	if rerr == nil {
		return "open", buf[:n], nil
	}
	if ctx.Err() != nil { // cut off by Ctrl+C, not a timeout: say nothing about the port
		return "error", nil, ctx.Err()
	}
	if netErr, ok := rerr.(net.Error); ok && netErr.Timeout() {
		return "open|filtered", nil, nil
	}
	if strings.Contains(rerr.Error(), "refused") {
		return "closed", nil, nil
	}
	return "error", nil, rerr
}

// UDPHandler sends the probe iterations times to every address host resolves
// to. It reports false if the lookup failed or any probe found the port closed
// (the OS surfaced an ICMP port-unreachable) or hit an error. An "open|filtered"
// probe - no reply, no ICMP error - is inconclusive rather than a failure: many
// UDP services simply do not answer input they do not understand. ctx is
// cancelled by Ctrl+C and nothing else - never a deadline; see interrupt.go for
// how a cancelled run ends.
func UDPHandler(ctx context.Context, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, payloadSize int, data string, port int, host string) (ok bool) {
	var statsMutex sync.Mutex
	output := lib.JSONOutput{}
	payload := []byte(data)
	if len(payload) == 0 {
		payload = []byte(strings.Repeat("d", payloadSize))
	}
	output.InputParams = lib.InputParams{
		Mode:     udpModule,
		Host:     host,
		FromPort: port,
		ToPort:   port,
		Protocol: "udp",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Payload:  len(payload),
		Throttle: *throttle,
	}
	output.ModuleName = udpModule
	istart := time.Now()
	dnsCtx, cancelDNS := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancelDNS()
	ipaddresses, err := lib.ResolveName(dnsCtx, host)
	if err != nil {
		if *jsonoutput {
			output.DNSLookup = lib.DNSLookup{Hostname: host, Success: false, Error: err.Error(), TimeTaken: time.Since(istart).Microseconds()}
		} else {
			fmt.Println(lib.LogWithTimestamp(udpModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error(), "time", time.Since(istart)), true))
		}
		if *jsonoutput {
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		}
		return false
	}

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(udpModule, "dns resolved "+lib.Fields("host", host, "addresses", len(ipaddresses), "ips", "["+strings.Join(ipaddresses, ",")+"]", "time", time.Since(istart)), false))
	} else {
		output.DNSLookup = lib.DNSLookup{Hostname: host, Success: true, ResolvedAddresses: ipaddresses, TimeTaken: time.Since(istart).Microseconds()}
		output.Stats = make([]lib.UDPStats, 0)
		output.StartTime = istart.UnixMicro()
	}

	var WG sync.WaitGroup
	var openCount, failures, completed int
attempts:
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		for _, ip := range ipaddresses {
			if ctx.Err() != nil {
				break attempts
			}
			if *throttle {
				randDelay, err := rand.Int(rand.Reader, big.NewInt(10000))
				if err != nil {
					fmt.Println(err)
				} else {
					delay = int(randDelay.Int64())
				}
			}
			if !pause(ctx, time.Millisecond*time.Duration(delay)) {
				break attempts
			}
			WG.Add(1)
			go func(ip string, attempt int) {
				defer WG.Done()
				start := time.Now()
				state, received, err := probeUDP(ctx, ip, port, timeout, payload)
				timeTaken := time.Since(start)
				if err != nil && ctx.Err() != nil {
					return // cut off by Ctrl+C: never finished, so it neither passed nor failed
				}

				statsMutex.Lock()
				completed++
				if state == "open" {
					openCount++
				}
				if state == "closed" || state == "error" {
					failures++
				}
				if *jsonoutput {
					stat := lib.UDPStats{
						Address:       ip,
						Port:          port,
						State:         state,
						Success:       state != "error",
						BytesSent:     len(payload),
						BytesReceived: len(received),
						SentTime:      start.UnixMicro(),
						RecvTime:      time.Now().UnixMicro(),
						TimeTaken:     timeTaken.Microseconds(),
					}
					if len(received) > 0 {
						stat.ResponsePreview = previewBytes(received)
					}
					if err != nil {
						stat.Error = err.Error()
					}
					output.Stats = append(output.Stats.([]lib.UDPStats), stat)
				}
				statsMutex.Unlock()

				if !*jsonoutput {
					if err != nil {
						fmt.Println(lib.LogWithTimestamp(udpModule, "probe error "+lib.Fields("host", ip, "port", port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", timeTaken, "error", err.Error()), true))
					} else {
						fmt.Println(lib.LogWithTimestamp(udpModule, "probe "+state+" "+lib.Fields("host", ip, "port", port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "sent", len(payload), "received", len(received), "time", timeTaken), false))
					}
				}
			}(ip, attempt)
		}
	}
	WG.Wait()
	planned := iterations * len(ipaddresses)
	interrupted := completed < planned // only Ctrl+C keeps an attempt from being made

	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		if interrupted {
			output.Error = interruptedNote(completed, planned)
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
	} else {
		sent := planned
		if interrupted {
			fmt.Println(interruptedLine(udpModule, completed, planned, istart))
			sent = completed
		}
		fmt.Println(lib.LogWithTimestamp(udpModule, "done "+lib.Fields("probes_sent", sent, "open", openCount, "total_time", time.Since(istart)), false))
	}
	return failures == 0 && !interrupted
}
