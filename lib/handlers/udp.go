package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
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

// MaxUDPPayload is the most one UDP datagram carries: 65535 minus the 8-byte UDP
// header and the 20-byte IPv4 header. A bigger payload cannot be sent at all.
const MaxUDPPayload = 65507

// ValidateUDPPayload rejects a --payload (the size of the generated filler) or a
// --data that no datagram can carry, before anything is sent: a negative size used
// to crash the handler with a panic, and a huge one asked it for memory it could
// not have. --payload is ignored when --data is given, so only one of them is
// checked.
func ValidateUDPPayload(size int, data string) error {
	if data != "" {
		if len(data) > MaxUDPPayload {
			return fmt.Errorf("--data is %d bytes, but one UDP datagram carries at most %d", len(data), MaxUDPPayload)
		}
		return nil
	}
	if size < 0 || size > MaxUDPPayload {
		return fmt.Errorf("--payload for udp must be between 0 and %d bytes (the most one UDP datagram carries), got %d", MaxUDPPayload, size)
	}
	return nil
}

// ParseHexPayload decodes the value of udp's --hex flag into the bytes to send:
// pairs of hex digits, in either case, with spaces and colons ignored wherever they
// fall - so "00010203ff", "00 01 02 03 FF" and "00:01:02:03:ff" are the same five
// bytes. It is the unambiguous way to send a binary payload: --data is text as typed,
// and a backslash escape such as \x00 reaches shint as four ordinary characters.
//
// It refuses what it cannot read rather than guess: no digits at all (a datagram of
// zero bytes is --payload 0), an odd number of digits, a character that is not a hex
// digit (a "0x" prefix included), and more bytes than one datagram carries.
func ParseHexPayload(s string) ([]byte, error) {
	digits := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r == ' ' || r == ':':
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			digits = append(digits, byte(r))
		default:
			return nil, fmt.Errorf("--hex: %q is not a hex digit (write the bytes as 00010203ff, or 00 01 02 03 ff, or 00:01:02:03:ff)", r)
		}
	}
	if len(digits) == 0 {
		return nil, fmt.Errorf("--hex needs at least one byte, as two hex digits (a datagram with no data is --payload 0)")
	}
	if len(digits)%2 != 0 {
		return nil, fmt.Errorf("--hex has %d hex digits, an odd number: every byte is two digits", len(digits))
	}
	if len(digits)/2 > MaxUDPPayload {
		return nil, fmt.Errorf("--hex is %d bytes, but one UDP datagram carries at most %d", len(digits)/2, MaxUDPPayload)
	}
	out := make([]byte, len(digits)/2)
	if _, err := hex.Decode(out, digits); err != nil {
		return nil, fmt.Errorf("--hex: %v", err) // unreachable: every character was checked above
	}
	return out, nil
}

// UDPHandler sends the probe iterations times to every address host resolves
// to. It reports false if the lookup failed or any probe found the port closed
// (the OS surfaced an ICMP port-unreachable) or hit an error. An "open|filtered"
// probe - no reply, no ICMP error - is inconclusive rather than a failure: many
// UDP services simply do not answer input they do not understand. ctx is
// cancelled by Ctrl+C and nothing else - never a deadline; see interrupt.go for
// how a cancelled run ends. data is the payload exactly as it is sent - text from
// --data, or the bytes --hex decoded (a Go string holds any bytes); when it is empty
// the payload is payloadSize bytes of filler.
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
		printAuthoritative(udpModule, findAuthoritative(ctx, host, authoritativeTimeout(timeout)))
	} else {
		output.DNSLookup = lib.DNSLookup{Hostname: host, Success: true, ResolvedAddresses: ipaddresses, TimeTaken: time.Since(istart).Microseconds(), Authoritative: findAuthoritative(ctx, host, authoritativeTimeout(timeout))}
		output.Stats = make([]lib.UDPStats, 0)
		output.StartTime = istart.UnixMicro()
	}

	var WG sync.WaitGroup
	var openCount, failures, completed int
	slots := newAttemptSlots()
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
			if !slots.acquire(ctx) {
				break attempts
			}
			WG.Add(1)
			go func(ip string, attempt int) {
				defer WG.Done()
				defer slots.release()
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
						stat.Error = lib.ExplainError(err)
					}
					output.Stats = append(output.Stats.([]lib.UDPStats), stat)
				}
				statsMutex.Unlock()

				if !*jsonoutput {
					if err != nil {
						fmt.Println(lib.LogWithTimestamp(udpModule, "probe error "+lib.Fields("host", ip, "port", port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", timeTaken, "error", lib.ExplainError(err)), true))
					} else {
						// A closed port makes the run exit 1, so it is logged as the failed check it is; open and
						// open|filtered are not failures.
						fmt.Println(lib.LogWithTimestamp(udpModule, "probe "+state+" "+lib.Fields("host", ip, "port", port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "sent", len(payload), "received", len(received), "time", timeTaken), state == "closed"))
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
