// Package lib holds the pieces shared by more than one command handler: name
// resolution, the single-port TCP dial, argument validators and latency
// statistics here, and in output.go the JSON output types and the text log
// format that keep every command's output consistent.
//
// It knows nothing about cobra or the process: handlers call into it, never
// the other way around. See docs/src/tech-architecture.md.
package lib

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"time"
)

const (
	// DATETIMEFORMAT is the timestamp layout at the start of every text log
	// line: Go's time.UnixDate, e.g. "Sun Sep 20 01:26:48 MDT 2026".
	DATETIMEFORMAT string = time.UnixDate
	// NetworkType controls DNS resolution family for telnet/nmap/udp/web's
	// diagnostic lookups (ping's own resolution lives in the go-ping
	// dependency and is already dual-stack): "ip" resolves both A and AAAA
	// records, "ip4"/"ip6" restrict to one. Dual-stack by default so a host
	// with both records gets checked over both protocols, the same way
	// ping already does.
	NetworkType string = "ip"
	// Protocol is the network IsPortUp dials.
	Protocol string = "tcp"
)

// ResolveName resolves name to its IP addresses as strings - IPv4 and IPv6
// alike (see NetworkType). ctx bounds the lookup; callers derive it from
// --timeout, and never from anything wider than the lookup itself.
func ResolveName(ctx context.Context, name string) ([]string, error) {
	var resolver net.Resolver
	ipaddresses, err := resolver.LookupIP(ctx, NetworkType, name)
	var addresses = make([]string, 0)
	for _, address := range ipaddresses {
		addresses = append(addresses, address.String())
	}
	return addresses, err
}

// ResolveNameToIPs is ResolveName returning parsed net.IP values, for callers
// that need the addresses themselves rather than their text.
func ResolveNameToIPs(ctx context.Context, name string) ([]net.IP, error) {
	var resolver net.Resolver
	return resolver.LookupIP(ctx, NetworkType, name)
}

// GetMinAvgMax returns the smallest, mean and largest of stats. stats must not
// be empty; callers (LogStats) check before calling.
func GetMinAvgMax(stats []time.Duration) (time.Duration, time.Duration, time.Duration) {
	max := slices.Max(stats)
	min := slices.Min(stats)
	var avg int
	for _, stat := range stats {
		avg += int(stat.Nanoseconds())
	}
	return min, time.Duration(avg / len(stats)), max
}

// SortTimeDurationSlice sorts *stats ascending, in place. Nothing in the
// commands uses it today; it is exercised by the tests only.
func SortTimeDurationSlice(stats *[]time.Duration) {
	sort.SliceStable(*stats, func(i, j int) bool {
		return ((*stats)[i] <= (*stats)[j])
	})
}

// IsPortUp attempts a TCP connection to host:port, bounded by whichever of
// timeout (seconds) or ctx's own deadline/cancellation comes first. Using
// DialContext (rather than a bare Dial with only dialer.Timeout) means a
// caller scanning many ports - nmap's port range in particular - can cancel
// or expire ctx once and have every in-flight and not-yet-started dial stop
// promptly, instead of each one running out its full per-dial timeout.
func IsPortUp(ctx context.Context, host string, port int, timeout int) (bool, error) {
	var dialer = net.Dialer{Timeout: time.Duration(timeout) * time.Second}
	conn, err := dialer.DialContext(ctx, Protocol, net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false, err
	}
	defer func() { _ = conn.Close() }()
	return true, nil
}

// ValidatePort parses raw as a TCP/UDP port number and ensures it falls within the valid range.
func ValidatePort(raw string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid port number %q", raw)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}
	return port, nil
}

// RequirePositive validates that a flag's integer value is at least 1.
func RequirePositive(name string, value int) error {
	if value < 1 {
		return fmt.Errorf("--%s must be a positive integer, got %d", name, value)
	}
	return nil
}

// ConvertIPToStringSlice returns the text form of each address; nil for an
// empty input.
func ConvertIPToStringSlice(ips []net.IP) []string {
	var result []string
	for _, ip := range ips {
		result = append(result, ip.String())
	}
	return result
}
