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

	"github.com/dmartsapp/shint/lib"
)

const telnetModule = "telnet"

// TelnetHandler checks TCP connectivity to host:port, once per resolved
// address per iteration. timeout (seconds) is what each individual
// connection attempt - and the DNS lookup - gets to complete; it is not a
// budget for the whole run. That matters because --count and --delay
// stretch a run well past --timeout (8 attempts one second apart take 8
// seconds), and an attempt that started after a run-wide deadline had
// already passed used to fail instantly with a bogus "i/o timeout".
func TelnetHandler(jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, payload_size int, port int, host string) {
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
	dnsCtx, cancelDNS := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	ipaddresses, err := lib.ResolveName(dnsCtx, host)
	cancelDNS()
	var stats = make([]time.Duration, 0)
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
		for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
			attempt := i + 1
			for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
				if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 1000 ms
					in, err := rand.Int(rand.Reader, big.NewInt(10000))
					if err != nil {
						fmt.Println(err)
					} else {
						delay = int(in.Int64())
					}
				}
				time.Sleep(time.Millisecond * time.Duration(delay))
				WG.Add(1)
				go func(ip string, attempt int) {
					defer WG.Done()
					start := time.Now()
					_, err := lib.IsPortUp(context.Background(), ip, port, timeout)
					timeTaken := time.Since(start)
					if err != nil {
						if *jsonoutput {
							stat := lib.TelnetStats{
								Address:   ip,
								Success:   false,
								SentTime:  start.UnixMicro(),
								TimeTaken: timeTaken.Microseconds(),
								Error:     err.Error(),
							}
							statsMutex.Lock()
							output.Stats = append(output.Stats.([]lib.TelnetStats), stat)
							statsMutex.Unlock()
						} else {
							fmt.Println(lib.LogWithTimestamp(telnetModule, "connect failed "+lib.Fields("host", ip, "port", port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", timeTaken, "error", err.Error()), true))
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
		if !*jsonoutput {
			statsMutex.Lock()
			fmt.Println(lib.LogStats(telnetModule, stats, (iterations * len(ipaddresses))))
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
}
