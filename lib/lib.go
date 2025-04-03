package lib

import (
	"context"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// DATETIMEFORMAT string = "Mon, 02 Jan 2006 15:04:05 MST"
	DATETIMEFORMAT string = time.UnixDate
	NetworkType    string = "ip4" // other networks are ip which includes both v4 and v6, and ip6 which is only v6
	Protocol       string = "tcp"
)

func ResolveName(ctx context.Context, name string) ([]string, error) {
	var resolver net.Resolver
	ipaddresses, err := resolver.LookupIP(ctx, NetworkType, name)
	var addresses = make([]string, 0)
	for _, address := range ipaddresses {
		addresses = append(addresses, address.String())
	}
	return addresses, err
}

func ResolveNameToIPs(ctx context.Context, name string) ([]net.IP, error) {
	var resolver net.Resolver
	return resolver.LookupIP(ctx, NetworkType, name)
}

func LogWithTimestamp(log string, iserror bool) string {
	if !iserror {
		return time.Now().Format(DATETIMEFORMAT) + ": " + log
	}
	return time.Now().Format(DATETIMEFORMAT) + ": Error! " + log

}

func GetMinAvgMax(stats []time.Duration) (time.Duration, time.Duration, time.Duration) {
	max := slices.Max(stats)
	min := slices.Min(stats)
	var avg int
	for _, stat := range stats {
		avg += int(stat.Nanoseconds())
	}
	return min, time.Duration(avg / len(stats)), max
}

func SortTimeDurationSlice(stats *[]time.Duration) {
	sort.SliceStable(*stats, func(i, j int) bool {
		return ((*stats)[i] <= (*stats)[j])
	})
}

func LogStats(modulename string, stats []time.Duration, iterations int) string {
	if len(stats) > 0 {
		min, avg, max := GetMinAvgMax(stats)
		return "\n" + strings.Repeat("=", (45-len(modulename))) + " " + modulename + " STATISTICS " + strings.Repeat("=", (45-len(modulename))) + "\nRequests sent: " + strconv.Itoa(iterations) + ", Response received: " + strconv.Itoa(len(stats)) + ", Success: " + strconv.Itoa(len(stats)*100/iterations) + "%\nLatency: minimum: " + min.String() + ", average: " + avg.String() + ", maximum: " + max.String()
	} else {
		return "\n" + strings.Repeat("=", (45-len(modulename))) + " " + modulename + " STATISTICS " + strings.Repeat("=", (45-len(modulename))) + "\nRequests sent: " + strconv.Itoa(iterations) + ", Response received: " + strconv.Itoa(len(stats)) + "\nLatency: minimum: 0, average: 0, maximum: 0"
	}

}

func IsPortUp(host string, port int, timeout int) (bool, error) {
	var dialer = net.Dialer{Timeout: time.Duration(timeout * int(time.Second))}
	conn, err := dialer.Dial(Protocol, host+":"+strconv.Itoa(port))
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return true, nil
}

type DNSLookup struct {
	Hostname          string   `json:"hostname"`
	ResolvedAddresses []string `json:"resolved_addresses"`
	Error             string   `json:"error"`
	Success           bool     `json:"success"`
	TimeTaken         int64    `json:"time_taken_µs"`
}

type InputParams struct {
	Mode       string `json:"module_name"`
	Sequential bool   `json:"sequential"`
	Throttle   bool   `json:"throttle"`
	Host       string `json:"host"`
	FromPort   int    `json:"from_port"`
	ToPort     int    `json:"to_port"`
	Protocol   string `json:"protocol"`
	Timeout    int    `json:"timeout_ms"`
	Count      int    `json:"count"`
	Delay      int    `json:"delay_ms"`
	Payload    int    `json:"payload_bytes"`
}

type TelnetStats struct {
	Address   string `json:"address"`
	Success   bool   `json:"success"`
	SentTime  int64  `json:"sent_unixtime_µs"`
	RecvTime  int64  `json:"recv_unixtime_µs"`
	TimeTaken int64  `json:"time_taken_µs"`
}

type WebStats struct {
	URL             string `json:"url"`
	Success         bool   `json:"success"`
	SentTime        int64  `json:"sent_unixtime_µs"`
	RecvTime        int64  `json:"recv_unixtime_µs"`
	TimeTaken       int64  `json:"time_taken_µs"`
	BytesDownloaded int    `json:"bytes_downloaded"` // added field to store the number of bytes downloaded
	StatusCode      int    `json:"status_code"`      // added field to store the HTTP status code
}

type ICMPStats struct {
	Address     string `json:"address"`
	Success     bool   `json:"success"`
	Sequence    int    `json:"sequence"` // added Sequence field to store the sequence number of the ICMP packet
	PayloadSize int    `json:"payload_size_bytes"`
	SentTime    int64  `json:"sent_unixtime_ms"`
	RecvTime    int64  `json:"recv_unixtime_ms"`
	TimeTaken   int64  `json:"time_taken_ms"`
}
type JSONOutput struct {
	InputParams    InputParams `json:"input_params"`
	ModuleName     string      `json:"module_name"`
	DNSLookup      DNSLookup   `json:"dns_lookup"`
	Stats          any         `json:"stats"`
	StartTime      int64       `json:"start_time_unixtime_µs"`
	EndTime        int64       `json:"end_time_unixtime_µs"`
	TotalTimeTaken int64       `json:"total_time_taken_µs"`
	Error          string      `json:"error"`
}

func ConvertIPToStringSlice(ips []net.IP) []string {
	var result []string
	for _, ip := range ips {
		result = append(result, ip.String())
	}
	return result
}
