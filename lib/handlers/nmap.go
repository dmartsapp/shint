package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/shint/lib"
)

const nmapModule = "nmap"

// maxConcurrentPortScans bounds the number of simultaneous dial attempts so
// scanning a wide port range (e.g. 1-65535) cannot exhaust local file
// descriptors or overwhelm the target.
const maxConcurrentPortScans = 500

func NmapHandler(ctx context.Context, host string, fromport, endport, iterations, timeout int, throttle bool, jsonoutput *bool) {
	output := lib.JSONOutput{}
	istart := time.Now()
	if *jsonoutput {
		output.InputParams = lib.InputParams{
			Mode:     nmapModule,
			Host:     host,
			FromPort: fromport,
			ToPort:   endport,
			Protocol: "tcp",
			Timeout:  timeout,
			Count:    iterations,
			Delay:    0,
			Payload:  0,
			Throttle: throttle,
		}
		output.ModuleName = nmapModule
		output.StartTime = istart.UnixMicro()
		output.Stats = make([]lib.NmapStats, 0)
	}

	ipaddresses, err := lib.ResolveName(ctx, host)
	if err != nil {
		if *jsonoutput {
			output.Error = err.Error()
		} else {
			fmt.Println(lib.LogWithTimestamp(nmapModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error(), "time", time.Since(istart)), true))
		}
	} else {
		if !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(nmapModule, "dns resolved "+lib.Fields("host", host, "addresses", len(ipaddresses), "ips", "["+strings.Join(ipaddresses, ",")+"]", "time", time.Since(istart)), false))
		} else {
			output.DNSLookup = lib.DNSLookup{
				Hostname:          host,
				Success:           true,
				ResolvedAddresses: ipaddresses,
				TimeTaken:         time.Since(istart).Microseconds(),
			}
		}
		var WG sync.WaitGroup
		var statsMutex sync.Mutex
		semaphore := make(chan struct{}, maxConcurrentPortScans)
		var openCount int
		totalScans := 0
		for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
			for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
				for port := fromport; port <= endport; port++ { // we need to loop over all ports individually
					if throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 10000 ms
						randDelay, err := rand.Int(rand.Reader, big.NewInt(10000))
						if err != nil {
							fmt.Println(err)
							return // added return to exit if error occurs
						}
						time.Sleep(time.Millisecond * time.Duration(randDelay.Int64()))
					}
					totalScans++
					WG.Add(1)
					semaphore <- struct{}{}
					go func(ip string, port int) {
						defer WG.Done()
						defer func() { <-semaphore }()
						_, err := lib.IsPortUp(ip, port, timeout)
						if err != nil {
							if *jsonoutput {
								statsMutex.Lock()
								output.Stats = append(output.Stats.([]lib.NmapStats), lib.NmapStats{Address: ip, Port: port, Success: false})
								statsMutex.Unlock()
							}
						} else {
							statsMutex.Lock()
							openCount++
							if *jsonoutput {
								output.Stats = append(output.Stats.([]lib.NmapStats), lib.NmapStats{Address: ip, Port: port, Success: true})
							}
							statsMutex.Unlock()
							if !*jsonoutput {
								fmt.Println(lib.LogWithTimestamp(nmapModule, "port open "+lib.Fields("host", ip, "port", port), false))
							}
						}
					}(ip, port)
				}
			}
		}
		WG.Wait()
		if !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(nmapModule, "scan complete "+lib.Fields("ports_scanned", totalScans, "open", openCount, "time", time.Since(istart)), false))
		}
	}

	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		JS, jsonErr := json.MarshalIndent(output, "", "  ")
		if jsonErr != nil {
			fmt.Println(lib.LogWithTimestamp(nmapModule, jsonErr.Error(), true))
			os.Exit(1)
		}
		fmt.Println(string(JS))
	} else {
		fmt.Println(lib.LogWithTimestamp(nmapModule, "done "+lib.Fields("total_time", time.Since(istart)), false))
	}
}
