package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

const telnetModule = "telnet"

// TelnetHandler checks TCP connectivity to host:port, once per resolved
// address per iteration. timeout (seconds) is what each individual
// connection attempt - and the DNS lookup - gets to complete; it is not a
// budget for the whole run. That matters because --count and --delay
// stretch a run well past --timeout (8 attempts one second apart take 8
// seconds), and an attempt that started after a run-wide deadline had
// already passed used to fail instantly with a bogus "i/o timeout".
//
// It reports true only if the DNS lookup and every attempt succeeded, which
// the caller turns into the process exit status. ctx is cancelled by Ctrl+C
// and nothing else - never a deadline; see interrupt.go for how a cancelled
// run ends.
func TelnetHandler(ctx context.Context, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, payload_size int, port int, host string) (ok bool) {
	var statsMutex sync.Mutex
	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     telnetModule,
		Host:     host,
		FromPort: port,
		ToPort:   port,
		Protocol: "tcp",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Payload:  payload_size,
		Throttle: *throttle,
	}
	output.ModuleName = telnetModule
	istart := time.Now() // capture initial time
	dnsCtx, cancelDNS := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	ipaddresses, err := lib.ResolveName(dnsCtx, host)
	cancelDNS()
	var stats = make([]time.Duration, 0)
	var completed int // attempts that finished, whatever the outcome; guarded by statsMutex
	ok = err == nil
	if err != nil {
		if *jsonoutput {
			output.DNSLookup = lib.DNSLookup{
				Hostname:  host,
				Success:   false,
				Error:     err.Error(),
				TimeTaken: time.Since(istart).Microseconds(),
			}
		} else {
			fmt.Println(lib.LogWithTimestamp(telnetModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error(), "time", time.Since(istart)), true))
			fmt.Println(lib.LogStats(telnetModule, stats, iterations))
		}
	} else {
		if !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(telnetModule, "dns resolved "+lib.Fields("host", host, "addresses", len(ipaddresses), "ips", "["+strings.Join(ipaddresses, ",")+"]", "time", time.Since(istart)), false))
		} else {
			output.DNSLookup = lib.DNSLookup{
				Hostname:          host,
				Success:           true,
				ResolvedAddresses: ipaddresses,
				TimeTaken:         time.Since(istart).Microseconds(),
			}
		}
		var WG sync.WaitGroup
		if *jsonoutput {
			output.Stats = make([]lib.TelnetStats, 0)
			output.StartTime = istart.UnixMicro()
		}
	attempts:
		for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
			attempt := i + 1
			for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
				if ctx.Err() != nil {
					break attempts
				}
				// Attempts are launched one at a time, --delay apart (a random
				// delay with --throttle), each in its own goroutine so a slow
				// attempt does not hold up the next. The delay comes before
				// every attempt, including the first.
				if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 1000 ms
					in, err := rand.Int(rand.Reader, big.NewInt(10000))
					if err != nil {
						fmt.Println(err)
					} else {
						delay = int(in.Int64())
					}
				}
				if !pause(ctx, time.Millisecond*time.Duration(delay)) {
					break attempts
				}
				WG.Add(1)
				go func(ip string, attempt int) {
					defer WG.Done()
					start := time.Now()
					_, err := lib.IsPortUp(ctx, ip, port, timeout)
					timeTaken := time.Since(start)
					if err != nil && ctx.Err() != nil {
						return // cut off by Ctrl+C: never finished, so it neither passed nor failed
					}
					statsMutex.Lock()
					completed++
					statsMutex.Unlock()
					if err != nil {
						if *jsonoutput {
							stat := lib.TelnetStats{
								Address:   ip,
								Success:   false,
								SentTime:  start.UnixMicro(),
								TimeTaken: timeTaken.Microseconds(),
								Error:     lib.ExplainError(err),
							}
							statsMutex.Lock()
							output.Stats = append(output.Stats.([]lib.TelnetStats), stat)
							statsMutex.Unlock()
						} else {
							fmt.Println(lib.LogWithTimestamp(telnetModule, "connect failed "+lib.Fields("host", ip, "port", port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", timeTaken, "error", lib.ExplainError(err)), true))
						}
					} else {
						statsMutex.Lock()
						stats = append(stats, timeTaken)
						if *jsonoutput {
							stat := lib.TelnetStats{
								Address:   ip,
								Success:   true,
								SentTime:  start.UnixMicro(),
								RecvTime:  time.Now().UnixMicro(),
								TimeTaken: timeTaken.Microseconds(),
							}
							output.Stats = append(output.Stats.([]lib.TelnetStats), stat)
						}
						statsMutex.Unlock()
						if !*jsonoutput {
							fmt.Println(lib.LogWithTimestamp(telnetModule, "connect ok "+lib.Fields("host", ip, "port", port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", timeTaken), false))
						}
					}
				}(ip, attempt)
			}
		}
		WG.Wait()
		planned := iterations * len(ipaddresses)
		interrupted := completed < planned // only Ctrl+C keeps an attempt from being made
		// stats only ever holds successful attempts, so anything short of
		// iterations x addresses means at least one attempt failed - or, when the
		// run was interrupted, never ran.
		ok = len(stats) == planned
		if interrupted {
			output.Error = interruptedNote(completed, planned)
		}
		if !*jsonoutput {
			statsMutex.Lock()
			if interrupted {
				fmt.Println(interruptedLine(telnetModule, completed, planned, istart))
				fmt.Println(lib.LogStats(telnetModule, stats, completed))
			} else {
				fmt.Println(lib.LogStats(telnetModule, stats, planned))
			}
			statsMutex.Unlock()
		}
	}

	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
	} else {
		fmt.Println(lib.LogWithTimestamp(telnetModule, "done "+lib.Fields("total_time", time.Since(istart)), false))
	}
	return ok
}
