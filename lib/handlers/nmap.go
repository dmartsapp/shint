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

// progressInterval is how often a running text-mode scan reports how far it
// has got. A scan against a host that drops packets finishes in bursts (a
// whole batch of dials waiting out its timeout together), so without this
// it can sit silent for as long as --timeout and look hung.
var progressInterval = 3 * time.Second

// probePort is the single-port TCP check, a variable so tests can swap in a
// slow or scripted one.
var probePort = lib.IsPortUp

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

	// The DNS lookup gets --timeout to itself; the scan that follows does
	// not (--timeout is what each port gets, not a budget for the range).
	ipaddresses, err := func() ([]string, error) {
		dnsCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		return lib.ResolveName(dnsCtx, host)
	}()
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
		// scanned counts ports whose probe actually finished (open or
		// closed); a dial aborted by cancellation is neither, and is not
		// counted. plannedScans is what the whole run would cover.
		var openCount, scanned int
		plannedScans := iterations * len(ipaddresses) * (endport - fromport + 1)

		// Text mode only: --json output must stay one clean document.
		stopProgress := func() {}
		if !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(nmapModule, "scan started "+lib.Fields("ports_total", plannedScans, "timeout", fmt.Sprintf("%ds", timeout), "max_in_flight", maxConcurrentPortScans), false))
			done := make(chan struct{})
			var progressWG sync.WaitGroup
			progressWG.Add(1)
			go func() {
				defer progressWG.Done()
				ticker := time.NewTicker(progressInterval)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						statsMutex.Lock()
						finished, open := scanned, openCount
						statsMutex.Unlock()
						fmt.Println(lib.LogWithTimestamp(nmapModule, "progress "+lib.Fields("ports_scanned", fmt.Sprintf("%d/%d", finished, plannedScans), "percent", fmt.Sprintf("%.1f%%", float64(finished)*100/float64(plannedScans)), "open", open, "in_flight", len(semaphore), "elapsed", time.Since(istart).Round(time.Millisecond)), false))
					case <-done:
						return
					}
				}
			}()
			var once sync.Once
			stopProgress = func() {
				once.Do(func() {
					close(done)
					progressWG.Wait()
				})
			}
		}
		defer stopProgress() // covers the early return inside the loop
	scanLoop:
		for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
			for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
				for port := fromport; port <= endport; port++ { // we need to loop over all ports individually
					// Stop launching new scans once ctx is done, so a wide
					// range (e.g. 1-65535) doesn't keep spawning dials well
					// past the caller's overall deadline just because each
					// individual dial has its own separate timeout.
					if ctx.Err() != nil {
						break scanLoop
					}
					if throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 10000 ms
						randDelay, err := rand.Int(rand.Reader, big.NewInt(10000))
						if err != nil {
							fmt.Println(err)
							return // added return to exit if error occurs
						}
						select {
						case <-time.After(time.Millisecond * time.Duration(randDelay.Int64())):
						case <-ctx.Done():
							break scanLoop
						}
					}
					WG.Add(1)
					semaphore <- struct{}{}
					go func(ip string, port int) {
						defer WG.Done()
						defer func() { <-semaphore }()
						_, err := probePort(ctx, ip, port, timeout)
						if err != nil && ctx.Err() != nil {
							// Aborted mid-dial by the scan being cancelled: this
							// port was not found closed, it was never finished.
							return
						}
						statsMutex.Lock()
						scanned++
						if err == nil {
							openCount++
						}
						if *jsonoutput {
							output.Stats = append(output.Stats.([]lib.NmapStats), lib.NmapStats{Address: ip, Port: port, Success: err == nil})
						}
						statsMutex.Unlock()
						if err == nil && !*jsonoutput {
							fmt.Println(lib.LogWithTimestamp(nmapModule, "port open "+lib.Fields("host", ip, "port", port), false))
						}
					}(ip, port)
				}
			}
		}
		WG.Wait()
		stopProgress() // before the summary, so a late tick can't print after it
		// A scan can only fall short of its plan by being cancelled (Ctrl+C
		// or the caller's context). Say so, rather than presenting a partial
		// range as a finished scan.
		if scanned < plannedScans {
			if *jsonoutput {
				output.Error = fmt.Sprintf("scan interrupted: %d of %d ports scanned", scanned, plannedScans)
			} else {
				fmt.Println(lib.LogWithTimestamp(nmapModule, "scan interrupted "+lib.Fields("ports_scanned", scanned, "ports_total", plannedScans, "open", openCount, "time", time.Since(istart)), true))
			}
		} else if !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(nmapModule, "scan complete "+lib.Fields("ports_scanned", scanned, "open", openCount, "time", time.Since(istart)), false))
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
