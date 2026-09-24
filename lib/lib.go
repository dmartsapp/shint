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

const (
	// MaxTimeoutSeconds and MaxDelayMilliseconds are one day. --timeout and --delay
	// become a time.Duration of nanoseconds, which overflows above about 292 years:
	// a huge value silently turned into a wait that has already run out. A day is
	// far beyond any check anyone waits for, and nothing near overflow.
	MaxTimeoutSeconds    = 24 * 60 * 60
	MaxDelayMilliseconds = 24 * 60 * 60 * 1000
)

// RequireTimeout validates --timeout for a command that waits on the network: 1 to
// MaxTimeoutSeconds.
func RequireTimeout(value int) error {
	if value < 1 || value > MaxTimeoutSeconds {
		return fmt.Errorf("--timeout must be between 1 and %d seconds (a day), got %d", MaxTimeoutSeconds, value)
	}
	return nil
}

// RequireIdleTimeout validates --timeout for a listener, where it is an idle
// timeout and 0 means none: 0 to MaxTimeoutSeconds.
func RequireIdleTimeout(value int) error {
	if value < 0 || value > MaxTimeoutSeconds {
		return fmt.Errorf("--timeout must be between 0 (no timeout) and %d seconds (a day), got %d", MaxTimeoutSeconds, value)
	}
	return nil
}

// RequireDelay validates --delay: 0 to MaxDelayMilliseconds.
func RequireDelay(value int) error {
	if value < 0 || value > MaxDelayMilliseconds {
		return fmt.Errorf("--delay must be between 0 and %d milliseconds (a day), got %d", MaxDelayMilliseconds, value)
	}
	return nil
}

// RequireNonNegative validates a flag whose 0 is meaningful ("no limit"), so only
// a negative value is a mistake.
func RequireNonNegative(name string, value int) error {
	if value < 0 {
		return fmt.Errorf("--%s must be 0 or a positive integer, got %d", name, value)
	}
	return nil
}

// InFlightLimit is how many network operations a command may have running at once:
// at most max, and never more than half of what the process's descriptor limit
// allows. The other half is for standard streams, resolver sockets and the rest of
// the runtime; without that margin a run that starts every attempt at once (--count
// 3000 --delay 0, or a scan of a filtered host) runs out of file descriptors and
// the surplus attempts fail with "too many open files" - blamed on the target.
func InFlightLimit(max int) int {
	limit, ok := descriptorLimit()
	return inFlightLimit(max, limit, ok)
}

// inFlightLimit is InFlightLimit for a given descriptor limit (ok false: unknown).
func inFlightLimit(max int, limit uint64, ok bool) int {
	if !ok {
		return max
	}
	half := limit / 2
	if half < 4 {
		half = 4 // some progress is better than none, however low the limit
	}
	if half > uint64(max) {
		return max
	}
	return int(half)
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
