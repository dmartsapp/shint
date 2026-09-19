package lib

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type DNSLookup struct {
	Hostname          string   `json:"hostname"`
	ResolvedAddresses []string `json:"resolved_addresses"`
	Error             string   `json:"error"`
	Success           bool     `json:"success"`
	TimeTaken         int64    `json:"time_taken_µs"`
}

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

type TelnetStats struct {
	Address   string `json:"address"`
	Success   bool   `json:"success"`
	RecvTime  int64  `json:"recv_unixtime_µs"`
	SentTime  int64  `json:"sent_unixtime_µs"`
	TimeTaken int64  `json:"time_taken_µs"`
	Error     string `json:"error,omitempty"`
}

type WebStats struct {
	URL             string         `json:"url"`
	Errors          []string       `json:"errors"`
	Request         map[string]any `json:"request"`
	Response        map[string]any `json:"response"`
	Success         bool           `json:"success"`
	RecvTime        int64          `json:"recv_unixtime_µs"`
	SentTime        int64          `json:"sent_unixtime_µs"`
	TimeTaken       int64          `json:"time_taken_µs"`
	BytesDownloaded int            `json:"bytes_downloaded"` // added field to store the number of bytes downloaded
	StatusCode      int            `json:"status_code"`      // added field to store the HTTP status code
}

type NmapStats struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Success bool   `json:"success"`
}

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
// when --json is set.
type ListenEvent struct {
	Protocol   string `json:"protocol"`
	RemoteAddr string `json:"remote_address"`
	LocalAddr  string `json:"local_address"`
	BytesRead  int    `json:"bytes_read"`
	Preview    string `json:"preview,omitempty"`
	UnixTimeUs int64  `json:"unixtime_µs"`
	Error      string `json:"error,omitempty"`
}

// HTTPListenEvent describes a single request observed by "listen http",
// emitted as one JSON line per request when --json is set.
type HTTPListenEvent struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	RemoteAddr string `json:"remote_address"`
	UnixTimeUs int64  `json:"unixtime_µs"`
}

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
