package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/go-ping/v2/netutils"
	"github.com/dmartsapp/shint/v4/lib"
	"github.com/dmartsapp/shint/v4/lib/handlers/dns"
)

const icmpModule = "icmp"

// restrictFamily applies -4/-6 to the addresses go-ping resolved. go-ping's own
// SetNetwork cannot do it: NewPinger has already resolved the name, both
// families, by the time a Pinger exists, and nothing resolves again. It fails,
// as a lookup failure does, when nothing of the requested family is left.
func restrictFamily(pinger *netutils.Pinger, host string) error {
	if lib.NetworkType == "ip" {
		return nil
	}
	pinger.SetNetwork(lib.NetworkType)
	kept := make([]net.IP, 0, len(pinger.Destination))
	for _, ip := range pinger.Destination {
		if lib.FamilyAllows(ip) {
			kept = append(kept, ip)
		}
	}
	if len(kept) == 0 {
		return fmt.Errorf("%s has no %s address", host, lib.FamilyName())
	}
	pinger.Destination = kept
	return nil
}

// MaxPingPayload is the largest echo payload, in bytes, the ping library will
// send in one unfragmented packet. It is read from the library itself rather
// than copied, so it cannot drift: SetPayloadSizeInBytes clamps to its limit.
func MaxPingPayload() int {
	return len(new(netutils.Pinger).SetPayloadSizeInBytes(math.MaxInt32).Payload)
}

// ValidatePayload rejects a --payload the ping library would silently
// shrink (or, for a negative value, silently zero). Called before anything is
// sent so misuse is a usage error, not a quietly different ping.
func ValidatePayload(size int) error {
	if max := MaxPingPayload(); size < 0 || size > max {
		return fmt.Errorf("--payload for ping must be between 0 and %d bytes (larger echo requests would fragment), got %d", max, size)
	}
	return nil
}

// isLostPing reports whether a line streamed by the ping library describes an
// echo request that got no reply. The library emits three kinds of line, each
// from a single place, so the prefix is a reliable signal: "received reply for
// request #N ..." (the only good one), "no reply for request #N ..." (nothing
// came back in time) and "error sending request #N to ..." (the system refused
// to send it: an unspecified or unreachable destination, say). The last two are
// failed checks, logged at ERROR level like one in every other command; they used
// to be logged as OK while the exit status said 1.
func isLostPing(line string) bool {
	return strings.HasPrefix(line, "no reply") || strings.HasPrefix(line, "error sending request")
}

// withPayloadSize adds the echo payload size to a reply line, the way ping
// tools show the size of each reply ("64 bytes from ..."): the line ends with
// bytes=N, which is the payload (what --payload sets), not counting the 8 byte
// ICMP header or the IP header - as Windows ping's bytes=32 does; Unix ping
// prints payload + 8. Only replies get it; a lost request has no reply to size.
// The library records the payload size it sent but not the length of the reply
// it read; RFC 792 requires an echo reply to return the request's data
// unchanged, so the two match.
func withPayloadSize(line string, payload int) string {
	if strings.HasPrefix(line, "received reply") {
		return line + " " + lib.Fields("bytes", payload)
	}
	return line
}

// permissionsPage is where the ping page explains which systems let a normal
// user open an ICMP socket.
const permissionsPage = "https://dmartsapp.github.io/shint/docs/ping.html#permissions"

// socketErrors is each distinct reason a request could not be sent at all
// because the ICMP socket would not open ("socket: protocol not supported" on
// FreeBSD, "socket: permission denied" on Linux outside
// net.ipv4.ping_group_range). The library records it on the request but logs
// only send errors and timeouts, so without this a run that sent nothing ended
// with no ERROR line (#111).
func socketErrors(packets []netutils.ICMPPacket) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range packets {
		if p.ErrorEncountered && strings.Contains(p.ErrorStr, "socket:") && !seen[p.ErrorStr] {
			seen[p.ErrorStr] = true
			out = append(out, p.ErrorStr)
		}
	}
	return out
}

// icmpStat is one request's JSON entry. A request without a reply has no
// receive time, so its time_taken_ms is 0, not the negative send time (#111),
// and its error says why.
func icmpStat(p netutils.ICMPPacket) lib.ICMPStats {
	stat := lib.ICMPStats{
		Address:     p.Destination.String(),
		Success:     !p.ErrorEncountered,
		Sequence:    p.Sequence,
		PayloadSize: p.PayloadSize,
		SentTime:    p.SentDateTimeUNIX,
		RecvTime:    p.ReceiveDateTimeUNIX,
	}
	if p.ErrorEncountered {
		stat.Error = p.ErrorStr
	} else {
		stat.TimeTaken = p.ReceiveDateTimeUNIX - p.SentDateTimeUNIX
	}
	return stat
}

// configurePinger applies the shared flags to a pinger. timeout (seconds) is
// how long each echo request waits for its reply before it counts as lost.
// The name lookup is not covered: netutils.NewPinger resolves inside its
// constructor with the library's own fixed 5 second limit, so there is
// nothing left to configure by the time a pinger exists.
func configurePinger(pinger *netutils.Pinger, iterations, delay int, throttle bool, timeout, payloadSize int) {
	pinger.
		SetPingCount(iterations).
		SetParallelPing(true).
		SetPayloadSizeInBytes(payloadSize).
		SetPingDelayInMS(delay).
		SetRandomizedPingDelay(throttle).
		SetReplyTimeoutInMS(timeout * 1000)
}

// Run pings every address host resolves to and reports true only if no
// echo request was lost. timeout is how long each echo request waits for its
// reply (the name lookup keeps the ping library's own 5 second limit).
func Run(host string, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, payload_size int) (ok bool) {

	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     icmpModule,
		Host:     host,
		FromPort: 7,
		ToPort:   7,
		Protocol: "icmp",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Payload:  payload_size,
		Throttle: *throttle,
	}
	output.ModuleName = icmpModule
	start := time.Now()
	pinger, err := netutils.NewPinger(host)
	if err == nil {
		err = restrictFamily(pinger, host)
	}
	if err != nil {
		if *jsonoutput {
			// Still one JSON document, so `--json | jq` keeps working.
			output.DNSLookup = lib.DNSLookup{Hostname: host, Success: false, Error: err.Error(), TimeTaken: time.Since(start).Microseconds()}
			output.Error = err.Error()
			output.Stats = make([]lib.ICMPStats, 0)
			output.StartTime = start.UnixMicro()
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		} else {
			fmt.Println(lib.LogWithTimestamp(icmpModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error()), true))
		}
		return false
	}

	configurePinger(pinger, iterations, delay, *throttle, timeout, payload_size)

	wg := sync.WaitGroup{}

	var authoritative *lib.Authoritative // for the JSON document; text mode prints its line below
	if *jsonoutput {
		authoritative = dns.FindAuthoritative(context.Background(), host, dns.AuthoritativeTimeout(timeout))
	}

	// In text mode go-ping's own log lines are streamed as replies arrive; in
	// JSON mode everything is reported once at the end from Stats.Packets.
	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(icmpModule, "dns resolved "+lib.Fields("host", host, "addresses", len(pinger.Destination), "ips", "["+strings.Join(lib.ConvertIPToStringSlice(pinger.Destination), ",")+"]", "time", pinger.Stats.ResolveTime), false))
		dns.PrintAuthoritative(icmpModule, dns.FindAuthoritative(context.Background(), host, dns.AuthoritativeTimeout(timeout)))
		wg.Add(1)
		go func(pinger *netutils.Pinger, payload int, wg *sync.WaitGroup) {
			defer wg.Done()
			for log := range pinger.StreamLog() {
				fmt.Println(lib.LogWithTimestamp(icmpModule, withPayloadSize(log, payload), isLostPing(log)))
			}
		}(pinger, len(pinger.Payload), &wg)
	}

	err = pinger.PingAll()
	if err != nil {
		if *jsonoutput {
			output.DNSLookup = lib.DNSLookup{
				Hostname:          host,
				Success:           true,
				ResolvedAddresses: lib.ConvertIPToStringSlice(pinger.Destination),
				TimeTaken:         pinger.Stats.ResolveTime.Microseconds(),
				Authoritative:     authoritative,
			}
			output.Error = err.Error()
			output.Stats = make([]lib.ICMPStats, 0)
			output.StartTime = start.UnixMicro()
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		} else {
			fmt.Println(lib.LogWithTimestamp(icmpModule, "ping failed "+lib.Fields("host", host, "error", err.Error()), true))
		}
		return false
	}

	wg.Wait()
	pinger.MeasureStats()
	totalPings := pinger.Count * len(pinger.Destination)
	sockErrs := socketErrors(pinger.Stats.Packets)
	if !*jsonoutput {
		for _, e := range sockErrs {
			fmt.Println(lib.LogWithTimestamp(icmpModule, "ping failed "+lib.Fields("host", host, "error", e, "note", "could not open an ICMP socket: see "+permissionsPage), true))
		}
		durations := make([]time.Duration, 0, len(pinger.Stats.Packets))
		for _, pckt := range pinger.Stats.Packets {
			if !pckt.ErrorEncountered {
				durations = append(durations, time.Duration(pckt.ReceiveDateTimeUNIX-pckt.SentDateTimeUNIX)*time.Millisecond)
			}
		}
		fmt.Println(lib.LogStats(icmpModule, durations, totalPings))
		fmt.Println(lib.LogWithTimestamp(icmpModule, "done "+lib.Fields("packets_lost", pinger.Stats.Loss, "stddev_ms", fmt.Sprintf("%.3f", pinger.Stats.StdDev), "resolve_time", pinger.Stats.ResolveTime, "total_time", pinger.Stats.TotalTime), false))
	} else {
		output.DNSLookup = lib.DNSLookup{
			Hostname:          host,
			Success:           true,
			ResolvedAddresses: lib.ConvertIPToStringSlice(pinger.Destination),
			TimeTaken:         pinger.Stats.ResolveTime.Microseconds(),
			Authoritative:     authoritative,
		}
		output.StartTime = start.UnixMicro()
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		output.Stats = make([]lib.ICMPStats, 0, len(pinger.Stats.Packets))
		for _, pckts := range pinger.Stats.Packets {
			output.Stats = append(output.Stats.([]lib.ICMPStats), icmpStat(pckts))
		}
		if len(sockErrs) > 0 {
			output.Error = strings.Join(sockErrs, "; ")
		}

		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))

	}
	return pinger.Stats.Loss == 0
}
