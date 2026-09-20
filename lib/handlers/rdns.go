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

const rdnsModule = "rdns"

// lookupAddr is the reverse lookup itself. It is a variable so tests can
// substitute a resolver, the way probePort is for nmap; in the program it is
// the system resolver's PTR query.
var lookupAddr = func(ctx context.Context, address string) ([]string, error) {
	var resolver net.Resolver
	return resolver.LookupAddr(ctx, address)
}

// ReverseName is the name a PTR query for ip is made under: the address
// reversed, under in-addr.arpa (IPv4) or ip6.arpa (IPv6, one label per hex
// digit) - RFC 1035 section 3.5 and RFC 3596 section 2.5. An IPv4-mapped IPv6
// address is looked up as the IPv4 address it wraps, as the resolver does.
func ReverseName(ip netip.Addr) string {
	ip = ip.Unmap()
	if ip.Is4() {
		b := ip.As4()
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa.", b[3], b[2], b[1], b[0])
	}
	b := ip.As16()
	var sb strings.Builder
	for i := len(b) - 1; i >= 0; i-- {
		fmt.Fprintf(&sb, "%x.%x.", b[i]&0x0f, b[i]>>4)
	}
	sb.WriteString("ip6.arpa.")
	return sb.String()
}

// RDNSHandler looks up the host names each address of host maps back to
// (reverse DNS, PTR records), iterations times. host may be an IP address, which
// is looked up as is, or a name: like every command, a name is resolved first
// and each address it resolves to is checked. --timeout bounds each lookup.
//
// It reports true only if every address had at least one name. An address with
// no PTR record is a failed check, not an empty success: many addresses have
// none, and knowing that is the point of asking.
func RDNSHandler(jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, host string) (ok bool) {
	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     rdnsModule,
		Host:     host,
		FromPort: 53,
		ToPort:   53,
		Protocol: "dns",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Throttle: *throttle,
	}
	output.ModuleName = rdnsModule
	istart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	addresses, err := lib.ResolveName(ctx, host)
	cancel()
	if err != nil {
		if *jsonoutput {
			output.DNSLookup = lib.DNSLookup{Hostname: host, Success: false, Error: err.Error(), TimeTaken: time.Since(istart).Microseconds()}
			output.Error = err.Error()
			output.Stats = make([]lib.RDNSStats, 0)
			output.StartTime = istart.UnixMicro()
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		} else {
			fmt.Println(lib.LogWithTimestamp(rdnsModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error(), "time", time.Since(istart)), true))
		}
		return false
	}

	if *jsonoutput {
		output.DNSLookup = lib.DNSLookup{Hostname: host, Success: true, ResolvedAddresses: addresses, TimeTaken: time.Since(istart).Microseconds()}
		output.Stats = make([]lib.RDNSStats, 0)
		output.StartTime = istart.UnixMicro()
	} else {
		fmt.Println(lib.LogWithTimestamp(rdnsModule, "dns resolved "+lib.Fields("host", host, "addresses", len(addresses), "ips", "["+strings.Join(addresses, ",")+"]", "time", time.Since(istart)), false))
	}

	var resolved, failures int
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		for _, address := range addresses {
			time.Sleep(attemptDelay(delay, *throttle))
			query := address
			if ip, perr := netip.ParseAddr(address); perr == nil {
				query = ReverseName(ip)
			}
			begin := time.Now()
			lctx, lcancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			names, lerr := lookupAddr(lctx, address)
			lcancel()
			taken := time.Since(begin)
			if len(names) == 0 && lerr == nil {
				lerr = fmt.Errorf("no PTR record for %s", address)
			}
			// A partial answer - some names, plus an error about the ones the
			// resolver discarded as malformed - still counts: the names are good.
			success := len(names) > 0
			if success {
				resolved++
			} else {
				failures++
			}

			if *jsonoutput {
				stat := lib.RDNSStats{Address: address, Query: query, Names: names, Success: success, SentTime: begin.UnixMicro(), RecvTime: time.Now().UnixMicro(), TimeTaken: taken.Microseconds()}
				if stat.Names == nil {
					stat.Names = []string{}
				}
				if !success {
					stat.Error = lerr.Error()
				}
				output.Stats = append(output.Stats.([]lib.RDNSStats), stat)
				continue
			}
			where := lib.Fields("address", address, "query", query, "attempt", fmt.Sprintf("%d/%d", attempt, iterations))
			if success {
				fmt.Println(lib.LogWithTimestamp(rdnsModule, "reverse lookup "+where+" "+lib.Fields("names", "["+strings.Join(names, ",")+"]", "time", taken), false))
			} else {
				fmt.Println(lib.LogWithTimestamp(rdnsModule, "reverse lookup failed "+where+" "+lib.Fields("time", taken, "error", lerr.Error()), true))
			}
		}
	}

	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
	} else {
		fmt.Println(lib.LogWithTimestamp(rdnsModule, "done "+lib.Fields("lookups", iterations*len(addresses), "resolved", resolved, "total_time", time.Since(istart)), false))
	}
	return failures == 0
}
