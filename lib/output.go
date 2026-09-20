package lib

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DNSLookup is the outcome of resolving the target's host name; every JSON
// document carries one.
type DNSLookup struct {
	Hostname          string   `json:"hostname"`
	ResolvedAddresses []string `json:"resolved_addresses"`
	Error             string   `json:"error"`
	Success           bool     `json:"success"`
	TimeTaken         int64    `json:"time_taken_µs"`
}

// InputParams records what a command ran with, so a saved JSON document says
// how it was produced. Compatibility notes: Timeout carries the --timeout
// value in seconds despite the key name (timeout_ms); FromPort/ToPort hold 7
// for ping and mean nothing there; Sequential is always false.
type InputParams struct {
	Mode       string   `json:"module_name"`
	Sequential bool     `json:"sequential"`
	Throttle   bool     `json:"throttle"`
	Host       string   `json:"host"`
	FromPort   int      `json:"from_port"`
	ToPort     int      `json:"to_port"`
	Protocol   string   `json:"protocol"`
	Timeout    int      `json:"timeout_ms"`
	Count      int      `json:"count"`
	Delay      int      `json:"delay_ms"`
	Payload    int      `json:"payload_bytes"`
	Method     string   `json:"method"`
	Data       string   `json:"data"`
	Headers    []string `json:"headers"`
}

// TelnetStats is one connection attempt, in microseconds; Error is set only
// when the attempt failed.
type TelnetStats struct {
	Address   string `json:"address"`
	Success   bool   `json:"success"`
	RecvTime  int64  `json:"recv_unixtime_µs"`
	SentTime  int64  `json:"sent_unixtime_µs"`
	TimeTaken int64  `json:"time_taken_µs"`
	Error     string `json:"error,omitempty"`
}

// WebStats is one HTTP attempt. A failed attempt (no response at all) is still
// recorded, with Success false and the reason in Errors.
type WebStats struct {
	URL       string         `json:"url"`
	Errors    []string       `json:"errors"`
	Request   map[string]any `json:"request"`
	Response  map[string]any `json:"response"`
	Success   bool           `json:"success"`
	RecvTime  int64          `json:"recv_unixtime_µs"`
	SentTime  int64          `json:"sent_unixtime_µs"`
	TimeTaken int64          `json:"time_taken_µs"`
	// BytesSent/BytesReceived are everything that crossed the connection for
	// this request - request line + headers + body out, status line +
	// headers + body in - measured the same way "listen http" measures its
	// own bytes_received/bytes_sent, so the two sides can be compared directly.
	BytesSent     int64   `json:"bytes_sent"`
	BytesReceived int64   `json:"bytes_received"`
	StatusCode    int     `json:"status_code"`   // added field to store the HTTP status code
	BandwidthKBs  float64 `json:"bandwidth_kbs"` // BytesReceived / TimeTaken, in KB/s
}

// NmapStats is one scanned port. Every port in the range is listed, open or
// not; Success means the connection was accepted.
type NmapStats struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Success bool   `json:"success"`
}

// ICMPStats is one echo request. Unlike the other stats its times are in
// milliseconds (the ping library's resolution).
type ICMPStats struct {
	Address     string `json:"address"`
	Success     bool   `json:"success"`
	Sequence    int    `json:"sequence"` // added Sequence field to store the sequence number of the ICMP packet
	PayloadSize int    `json:"payload_size_bytes"`
	RecvTime    int64  `json:"recv_unixtime_ms"`
	SentTime    int64  `json:"sent_unixtime_ms"`
	TimeTaken   int64  `json:"time_taken_ms"`
}

// UDPStats holds the outcome of a single UDP probe attempt. Because UDP is
// connectionless, State reflects the best-effort classification nmap-style
// UDP scans use: "open" (a reply was received), "closed" (an ICMP
// port-unreachable was surfaced by the OS), or "open|filtered" (no reply
// arrived within the timeout, so it cannot be distinguished from a silently
// dropped packet).
type UDPStats struct {
	Address         string `json:"address"`
	Port            int    `json:"port"`
	State           string `json:"state"`
	Success         bool   `json:"success"`
	BytesSent       int    `json:"bytes_sent"`
	BytesReceived   int    `json:"bytes_received"`
	ResponsePreview string `json:"response_preview,omitempty"`
	SentTime        int64  `json:"sent_unixtime_µs"`
	RecvTime        int64  `json:"recv_unixtime_µs"`
	TimeTaken       int64  `json:"time_taken_µs"`
	Error           string `json:"error,omitempty"`
}

// ListenEvent describes a single inbound connection/packet observed by the
// "listen tcp"/"listen udp" commands, emitted as one JSON line per event
// when --json is set. BytesSent and ProcessingTimeUs are only populated by
// "listen tcp" (its read+optional-echo cycle is treated as one "request"
// handled); "listen udp" leaves them at zero, so omitempty keeps its event
// shape unchanged.
type ListenEvent struct {
	Protocol         string `json:"protocol"`
	RemoteAddr       string `json:"remote_address"`
	LocalAddr        string `json:"local_address"`
	BytesRead        int    `json:"bytes_read"`
	BytesSent        int    `json:"bytes_sent,omitempty"`
	ProcessingTimeUs int64  `json:"processing_time_µs,omitempty"`
	Preview          string `json:"preview,omitempty"`
	UnixTimeUs       int64  `json:"unixtime_µs"`
	Error            string `json:"error,omitempty"`
}

// HTTPListenEvent describes a single request observed by "listen http",
// emitted as one JSON line per request when --json is set. BytesReceived and
// BytesSent are the raw bytes that crossed the connection (request line +
// headers + body in, status line + headers + body out), counted without
// parsing them, and match what "web" reports as its own bytes_sent and
// bytes_received for the same exchange.
type HTTPListenEvent struct {
	Method           string `json:"method"`
	Path             string `json:"path"`
	StatusCode       int    `json:"status_code"`
	RemoteAddr       string `json:"remote_address"`
	BytesReceived    int64  `json:"bytes_received"`
	BytesSent        int64  `json:"bytes_sent"`
	ProcessingTimeUs int64  `json:"processing_time_µs"`
	UnixTimeUs       int64  `json:"unixtime_µs"`
}

// JSONOutput is the document every command prints with --json: a shared
// skeleton (inputs, DNS result, timing, run-level error) around Stats, which
// holds a slice of the command's own stats type. Even a run whose checks all
// failed is one valid document. See docs/src/output.md.
type JSONOutput struct {
	InputParams    InputParams `json:"input_params"`
	ModuleName     string      `json:"module_name"`
	DNSLookup      DNSLookup   `json:"dns_lookup"`
	Stats          any         `json:"stats"`
	EndTime        int64       `json:"end_time_unixtime_µs"`
	StartTime      int64       `json:"start_time_unixtime_µs"`
	TotalTimeTaken int64       `json:"total_time_taken_µs"`
	Error          string      `json:"error"`
}

// LogWithTimestamp formats a single human-readable log line uniformly across
// every module: "<timestamp>: [<module>] <OK|ERROR> <message>". Keeping the
// prefix identical for telnet/ping/web/nmap/udp/listen output makes the
// stream easy to grep/parse regardless of which command produced it.
func LogWithTimestamp(module string, message string, iserror bool) string {
	status := "OK"
	if iserror {
		status = "ERROR"
	}
	return fmt.Sprintf("%s: [%s] %s %s", time.Now().Format(DATETIMEFORMAT), module, status, message)
}

// Fields renders a uniform, greppable "key=value" suffix used by every
// module's per-attempt log line, e.g. Fields("host", ip, "port", 443, "time", d).
// Values are formatted with %v except strings containing whitespace, which are quoted.
func Fields(kv ...any) string {
	if len(kv)%2 != 0 {
		panic("lib.Fields: odd number of arguments")
	}
	parts := make([]string, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		val := kv[i+1]
		str := fmt.Sprint(val)
		if s, ok := val.(string); ok && strings.ContainsAny(s, " \t") {
			str = strconv.Quote(s)
		}
		parts = append(parts, key+"="+str)
	}
	return strings.Join(parts, " ")
}

// LogStats renders the uniform "=== <module> STATISTICS ===" summary banner
// shared by telnet/web/nmap/icmp/udp text-mode output.
func LogStats(modulename string, stats []time.Duration, iterations int) string {
	pad := 45 - len(modulename)
	if pad < 1 {
		pad = 1
	}
	header := "\n" + strings.Repeat("=", pad) + " " + modulename + " STATISTICS " + strings.Repeat("=", pad) + "\n"
	if iterations <= 0 {
		return header + "Requests sent: 0, Response received: " + strconv.Itoa(len(stats))
	}
	if len(stats) > 0 {
		min, avg, max := GetMinAvgMax(stats)
		return header + "Requests sent: " + strconv.Itoa(iterations) + ", Response received: " + strconv.Itoa(len(stats)) + ", Success: " + strconv.Itoa(len(stats)*100/iterations) + "%\nLatency: minimum: " + min.String() + ", average: " + avg.String() + ", maximum: " + max.String()
	}
	return header + "Requests sent: " + strconv.Itoa(iterations) + ", Response received: 0\nLatency: minimum: 0, average: 0, maximum: 0"
}
