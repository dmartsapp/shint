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
	// Timing is present only with --timing: where the request's time went,
	// one entry per hop when a redirect was followed.
	Timing *WebTiming `json:"timing,omitempty"`
}

// WebTiming is the --timing breakdown of one request.
type WebTiming struct {
	Hops []WebHopTiming `json:"hops"`
}

// WebHopTiming is where the time went for one hop of a request (the request
// itself, then each redirect it followed), in microseconds:
//
//	dns       the name lookup for this hop (0: an IP address, or a reused connection)
//	connect   opening the TCP connection (0 on a reused connection)
//	tls       the TLS handshake (0 for http://, or a reused connection)
//	wait      the request fully sent -> the first byte of the response: the
//	          server's time to respond plus one network round trip
//	download  the first byte of the response -> its body fully read
//	total     the hop from start to finish
//
// StatusCode is 0 for a hop that got no response (the request failed there).
type WebHopTiming struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Reused     bool   `json:"reused_connection"`
	DNSUs      int64  `json:"dns_µs"`
	ConnectUs  int64  `json:"connect_µs"`
	TLSUs      int64  `json:"tls_µs"`
	WaitUs     int64  `json:"wait_µs"`
	DownloadUs int64  `json:"download_µs"`
	TotalUs    int64  `json:"total_µs"`
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

// CIDRStats is one subnet as "cidr" reports it. Addresses and UsableHosts are
// decimal strings, not numbers: an IPv6 prefix can hold more addresses than a
// 64-bit integer (a /64 has 2^64, a /0 has 2^128) and JSON consumers would
// silently round them. Wildcard is set for IPv4 only, and Broadcast only for
// an IPv4 prefix that has one (a /31 and a /32 do not - RFC 3021).
type CIDRStats struct {
	Input        string `json:"input"`
	Network      string `json:"network"`
	Family       string `json:"family"`
	PrefixLength int    `json:"prefix_length"`
	Netmask      string `json:"netmask"`
	Wildcard     string `json:"wildcard,omitempty"`
	FirstAddress string `json:"first_address"`
	LastAddress  string `json:"last_address"`
	Broadcast    string `json:"broadcast,omitempty"`
	Addresses    string `json:"addresses"`
	UsableHosts  string `json:"usable_hosts"`
	FirstHost    string `json:"first_host"`
	LastHost     string `json:"last_host"`
	Kind         string `json:"kind"`
}

// WOLStats is one magic packet sent by "wol". Success means the operating
// system accepted the packet for sending; it says nothing about whether the
// machine woke up - Wake-on-LAN has no reply.
type WOLStats struct {
	MAC       string `json:"mac"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Success   bool   `json:"success"`
	BytesSent int    `json:"bytes_sent"`
	SentTime  int64  `json:"sent_unixtime_µs"`
	TimeTaken int64  `json:"time_taken_µs"`
	Error     string `json:"error,omitempty"`
}

// NTPStats is one time query sent by "ntp". Offsets are signed and in
// microseconds: OffsetUs is server time minus local time, so a positive value
// means this machine's clock is behind. RoundTripUs is the network delay of the
// exchange, not the server's processing time. Error is set only for a failed
// query, and then only Address, SentTime and TimeTaken carry values.
type NTPStats struct {
	Address       string `json:"address"`
	Success       bool   `json:"success"`
	Stratum       int    `json:"stratum"`
	Version       int    `json:"version"`
	LeapIndicator string `json:"leap_indicator"`
	ReferenceID   string `json:"reference_id"`
	OffsetUs      int64  `json:"offset_µs"`
	RoundTripUs   int64  `json:"round_trip_µs"`
	ServerTimeUs  int64  `json:"server_unixtime_µs"`
	SentTime      int64  `json:"sent_unixtime_µs"`
	RecvTime      int64  `json:"recv_unixtime_µs"`
	TimeTaken     int64  `json:"time_taken_µs"`
	Error         string `json:"error,omitempty"`
}

// RDNSStats is one reverse lookup by "rdns". Query is the in-addr.arpa or
// ip6.arpa name that was asked for; Names are the host names the address maps
// back to (a PTR record set, so there may be several). Error is set only for a
// failed lookup - "no such host" means the address has no PTR record.
type RDNSStats struct {
	Address   string   `json:"address"`
	Query     string   `json:"query"`
	Names     []string `json:"names"`
	Success   bool     `json:"success"`
	SentTime  int64    `json:"sent_unixtime_µs"`
	RecvTime  int64    `json:"recv_unixtime_µs"`
	TimeTaken int64    `json:"time_taken_µs"`
	Error     string   `json:"error,omitempty"`
}

// IPStats is one network interface listed by "ip". State is "up" or "down"
// (the interface's own administrative flag, not whether it has a carrier);
// Flags are the operating system's flag names (broadcast, multicast, loopback,
// point-to-point, running); MAC is empty for interfaces that have none
// (loopback, tunnels). Error is set only when the addresses of an interface
// could not be read.
type IPStats struct {
	Name      string      `json:"name"`
	Index     int         `json:"index"`
	State     string      `json:"state"`
	Flags     []string    `json:"flags"`
	MTU       int         `json:"mtu"`
	MAC       string      `json:"mac,omitempty"`
	Addresses []IPAddress `json:"addresses"`
	Error     string      `json:"error,omitempty"`
}

// IPAddress is one address on an interface: Prefix is the address with its
// prefix length as configured ("192.168.1.20/24"), Kind what the address is
// for (loopback, private, link-local, global, ...; see "cidr").
type IPAddress struct {
	Address      string `json:"address"`
	Prefix       string `json:"prefix"`
	PrefixLength int    `json:"prefix_length"`
	Family       string `json:"family"`
	Kind         string `json:"kind"`
}

// DNSStats is one question asked by "dns": Name and Type are what was asked,
// Server and Transport who answered and how (Transport is "tcp" when the answer
// came over TCP, which is when it was truncated over UDP or --tcp was given),
// RCode the response code (NOERROR, NXDOMAIN, SERVFAIL, ...) and Flags the
// header flags that were set (qr, aa, tc, rd, ra, ad, cd). Success means the
// server answered with at least one record of the type asked. Answers and
// Authority are the two sections that matter (the latter carries the SOA of a
// negative answer); Additional counts the third without listing it. Skipped
// names the servers tried first that did not answer. Error is set only when the
// question failed - including when no server answered, in which case Server is
// empty.
type DNSStats struct {
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	Server         string      `json:"server,omitempty"`
	Transport      string      `json:"transport,omitempty"`
	RCode          string      `json:"rcode,omitempty"`
	Flags          []string    `json:"flags"`
	Success        bool        `json:"success"`
	Answers        []DNSRecord `json:"answers"`
	Authority      []DNSRecord `json:"authority"`
	Additional     int         `json:"additional_count"`
	RetriedOverTCP bool        `json:"retried_over_tcp,omitempty"`
	Skipped        []string    `json:"skipped_servers,omitempty"`
	SentTime       int64       `json:"sent_unixtime_µs"`
	RecvTime       int64       `json:"recv_unixtime_µs"`
	TimeTaken      int64       `json:"time_taken_µs"`
	Error          string      `json:"error,omitempty"`
}

// DNSRecord is one resource record: Data is its value written the way dig
// writes it ("10 mail.example.com." for an MX, "0 issue \"letsencrypt.org\""
// for a CAA); a TXT record's Data is its character strings joined, and Strings
// keeps them apart.
type DNSRecord struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     uint32   `json:"ttl"`
	Data    string   `json:"data"`
	Strings []string `json:"strings,omitempty"`
}

// LocalJSONOutput is the JSON document of a command that never looks up a
// name ("cidr", "wol"): JSONOutput without dns_lookup, which would only ever
// read as a failed lookup. Everything else is the same shape.
type LocalJSONOutput struct {
	InputParams    InputParams `json:"input_params"`
	ModuleName     string      `json:"module_name"`
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
