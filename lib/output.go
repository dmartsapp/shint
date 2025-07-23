package lib

import (
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

func LogWithTimestamp(log string, iserror bool) string {
	if !iserror {
		return time.Now().Format(DATETIMEFORMAT) + ": " + log
	}
	return time.Now().Format(DATETIMEFORMAT) + ": Error! " + log

}

func LogStats(modulename string, stats []time.Duration, iterations int) string {
	if len(stats) > 0 {
		min, avg, max := GetMinAvgMax(stats)
		return "\n" + strings.Repeat("=", (45-len(modulename))) + " " + modulename + " STATISTICS " + strings.Repeat("=", (45-len(modulename))) + "\nRequests sent: " + strconv.Itoa(iterations) + ", Response received: " + strconv.Itoa(len(stats)) + ", Success: " + strconv.Itoa(len(stats)*100/iterations) + "%\nLatency: minimum: " + min.String() + ", average: " + avg.String() + ", maximum: " + max.String()
	} else {
		return "\n" + strings.Repeat("=", (45-len(modulename))) + " " + modulename + " STATISTICS " + strings.Repeat("=", (45-len(modulename))) + "\nRequests sent: " + strconv.Itoa(iterations) + ", Response received: " + strconv.Itoa(len(stats)) + "\nLatency: minimum: 0, average: 0, maximum: 0"
	}

}
