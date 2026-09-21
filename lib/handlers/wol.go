package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

const wolModule = "wol"

const (
	// WOLDefaultBroadcast is the limited broadcast address: every host on the
	// local network segment, reached through the default route's interface.
	WOLDefaultBroadcast = "255.255.255.255"
	// WOLDefaultPort is the "discard" port, the conventional target for magic packets.
	WOLDefaultPort = 9
	// magicPacketSize is 6 bytes of 0xFF followed by the MAC address 16 times.
	magicPacketSize = 6 + 16*6
)

// ParseMAC accepts a MAC address as six hex bytes separated by ":" or "-"
// (aa:bb:cc:dd:ee:ff), in Cisco dotted groups (aabb.ccdd.eeff), or as 12
// bare hex digits (aabbccddeeff). Anything that is not exactly six bytes is
// refused - net.ParseMAC on its own also accepts 8- and 20-byte addresses.
func ParseMAC(s string) (net.HardwareAddr, error) {
	text := strings.TrimSpace(s)
	if len(text) == 12 && !strings.ContainsAny(text, ":-.") {
		var parts []string
		for i := 0; i < 12; i += 2 {
			parts = append(parts, text[i:i+2])
		}
		text = strings.Join(parts, ":")
	}
	mac, err := net.ParseMAC(text)
	if err != nil || len(mac) != 6 {
		return nil, fmt.Errorf("invalid MAC address %q: use six hex bytes, such as aa:bb:cc:dd:ee:ff", s)
	}
	return mac, nil
}

// ParseBroadcast checks the --broadcast value: an IPv4 address. Wake-on-LAN
// here is IPv4 UDP broadcast; IPv6 has no broadcast.
func ParseBroadcast(s string) (netip.Addr, error) {
	a, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil || !a.Is4() || a.IsUnspecified() || a.IsMulticast() {
		return netip.Addr{}, fmt.Errorf("--broadcast must be an IPv4 address such as 192.168.1.255, got %q", s)
	}
	return a, nil
}

// MagicPacket builds the Wake-on-LAN "magic packet": six 0xFF bytes, then the
// target's MAC address sixteen times - 102 bytes.
func MagicPacket(mac net.HardwareAddr) []byte {
	packet := make([]byte, 0, magicPacketSize)
	for i := 0; i < 6; i++ {
		packet = append(packet, 0xff)
	}
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}
	return packet
}

// sendUDP sends one datagram to dest. Go enables SO_BROADCAST on UDP sockets,
// so a broadcast address needs no special privilege.
func sendUDP(dest netip.AddrPort, payload []byte, timeoutSeconds int) (int, error) {
	conn, err := net.DialUDP("udp4", nil, net.UDPAddrFromAddrPort(dest))
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetWriteDeadline(time.Now().Add(time.Duration(timeoutSeconds) * time.Second)); err != nil {
		return 0, err
	}
	return conn.Write(payload)
}

// WOLHandler sends the magic packet for mac to broadcast:port, iterations
// times. It reports true only if every packet was handed to the network.
//
// That is all it can know: Wake-on-LAN has no reply, so "sent" is not "woke".
// The machine has to be set up to listen for the packet (its firmware and
// network card), be on the same network segment as this one, and the
// broadcast has to reach it. ctx is cancelled by Ctrl+C and nothing else -
// never a deadline; see interrupt.go for how a cancelled run ends.
func WOLHandler(ctx context.Context, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, mac net.HardwareAddr, broadcast netip.Addr, port int) (ok bool) {
	start := time.Now()
	dest := netip.AddrPortFrom(broadcast, uint16(port))
	packet := MagicPacket(mac)
	output := lib.LocalJSONOutput{
		InputParams: lib.InputParams{
			Mode:     wolModule,
			Host:     broadcast.String(),
			FromPort: port,
			ToPort:   port,
			Protocol: "udp",
			Timeout:  timeout,
			Count:    iterations,
			Delay:    delay,
			Payload:  len(packet),
			Throttle: *throttle,
			Data:     mac.String(),
		},
		ModuleName: wolModule,
		StartTime:  start.UnixMicro(),
	}
	stats := make([]lib.WOLStats, 0, iterations)

	sent, attempted := 0, 0
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		if ctx.Err() != nil || !pause(ctx, attemptDelay(delay, *throttle)) {
			break
		}
		attempted++
		begin := time.Now()
		n, err := sendUDP(dest, packet, timeout)
		taken := time.Since(begin)
		if err == nil && n != len(packet) {
			err = fmt.Errorf("short write: %d of %d bytes", n, len(packet))
		}
		stat := lib.WOLStats{MAC: mac.String(), Address: broadcast.String(), Port: port, Success: err == nil, BytesSent: n, SentTime: begin.UnixMicro(), TimeTaken: taken.Microseconds()}
		if err != nil {
			stat.Error = err.Error()
		} else {
			sent++
		}
		stats = append(stats, stat)
		if !*jsonoutput {
			if err != nil {
				fmt.Println(lib.LogWithTimestamp(wolModule, "send failed "+lib.Fields("mac", mac.String(), "to", dest.String(), "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", taken, "error", err.Error()), true))
			} else {
				fmt.Println(lib.LogWithTimestamp(wolModule, "magic packet sent "+lib.Fields("mac", mac.String(), "to", dest.String(), "bytes", n, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", taken), false))
			}
		}
	}

	interrupted := attempted < iterations // only Ctrl+C keeps a packet from being sent
	if *jsonoutput {
		output.Stats = stats
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		if interrupted {
			output.Error = interruptedNote(attempted, iterations)
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
	} else {
		if interrupted {
			fmt.Println(interruptedLine(wolModule, attempted, iterations, start))
		}
		fmt.Println(lib.LogWithTimestamp(wolModule, "done "+lib.Fields("packets_sent", sent, "total_time", time.Since(start)), false))
	}
	return sent == iterations
}
