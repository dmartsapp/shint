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

	"github.com/dmartsapp/shint/v4/lib"
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

// NmapScan is what one nmap run covers: its targets, as ParseNmapTargets
// checked them, and its ports, sorted and each once.
type NmapScan struct {
	Targets []NmapTarget
	Ports   []int
	// PortList is whether the ports were given as a list (--ports) rather than
	// a --from/--to range; the JSON record of the run then has the list.
	PortList bool
}

// scanAddress is one address to scan and the target it came from.
type scanAddress struct {
	target, ip string
}

// NmapHandler reports true if every target was looked up and the scan ran to
// the end. Finding no open ports is a result, not a failure; a failed DNS
// lookup or a scan cut short (Ctrl+C) is a failure. A target whose lookup
// fails is reported and the others are still scanned. An address that more
// than one target leads to is scanned once, for the first of them.
func NmapHandler(ctx context.Context, scan NmapScan, iterations, timeout int, throttle bool, jsonoutput *bool) (ok bool) {
	output := lib.NmapJSONOutput{}
	istart := time.Now()
	ports := scan.Ports
	single := len(scan.Targets) == 1
	if *jsonoutput {
		output.InputParams = lib.InputParams{
			Mode:     nmapModule,
			FromPort: ports[0],
			ToPort:   ports[len(ports)-1],
			Protocol: "tcp",
			Timeout:  timeout,
			Count:    iterations,
			Delay:    0,
			Payload:  0,
			Throttle: throttle,
		}
		if single {
			output.InputParams.Host = scan.Targets[0].Input
		} else {
			for _, t := range scan.Targets {
				output.InputParams.Targets = append(output.InputParams.Targets, t.Input)
			}
		}
		if scan.PortList {
			output.InputParams.Ports = lib.PortsText(ports)
		}
		output.ModuleName = nmapModule
		output.StartTime = istart.UnixMicro()
		output.Stats = make([]lib.NmapStats, 0)
	}

	ok = true
	var problems []string // the JSON error: failed lookups, an interrupted scan
	var addresses []scanAddress
	seen := make(map[string]bool)
	add := func(target string, ips []string) {
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				addresses = append(addresses, scanAddress{target: target, ip: ip})
			}
		}
	}
	for _, t := range scan.Targets {
		if t.IsPrefix() {
			ips := sweepAddresses(t.Prefix)
			if !*jsonoutput {
				fields := []any{"target", t.Input}
				if network := t.Prefix.String(); network != t.Input {
					fields = append(fields, "network", network)
				}
				fmt.Println(lib.LogWithTimestamp(nmapModule, "sweep "+lib.Fields(append(fields, "addresses", len(ips))...), false))
			}
			add(t.Input, ips)
			continue
		}
		// Each lookup gets --timeout to itself; the scan that follows does
		// not (--timeout is what each port gets, not a budget for the range).
		start := time.Now()
		ipaddresses, err := func() ([]string, error) {
			dnsCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()
			return lib.ResolveName(dnsCtx, t.Input)
		}()
		if err != nil {
			ok = false
			if *jsonoutput {
				failed := lib.DNSLookup{Hostname: t.Input, ResolvedAddresses: ipaddresses, Error: err.Error(), TimeTaken: time.Since(start).Microseconds()}
				if single {
					problems = append(problems, err.Error())
					output.DNSLookup = &failed
				} else {
					problems = append(problems, "dns resolution failed for "+t.Input+": "+err.Error())
					output.DNSLookups = append(output.DNSLookups, failed)
				}
			} else {
				fmt.Println(lib.LogWithTimestamp(nmapModule, "dns resolution failed "+lib.Fields("host", t.Input, "error", err.Error(), "time", time.Since(start)), true))
			}
			continue
		}
		if !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(nmapModule, "dns resolved "+lib.Fields("host", t.Input, "addresses", len(ipaddresses), "ips", "["+strings.Join(ipaddresses, ",")+"]", "time", time.Since(start)), false))
			printAuthoritative(nmapModule, findAuthoritative(ctx, t.Input, authoritativeTimeout(timeout)))
		} else {
			lookup := lib.DNSLookup{
				Hostname:          t.Input,
				Success:           true,
				ResolvedAddresses: ipaddresses,
				TimeTaken:         time.Since(start).Microseconds(),
				Authoritative:     findAuthoritative(ctx, t.Input, authoritativeTimeout(timeout)),
			}
			if single {
				output.DNSLookup = &lookup
			} else {
				output.DNSLookups = append(output.DNSLookups, lookup)
			}
		}
		add(t.Input, ipaddresses)
	}

	// Nothing to scan because every lookup failed: those failures are the
	// result. (A lookup that succeeds always has an address.)
	if len(addresses) > 0 || ok {
		var WG sync.WaitGroup
		var statsMutex sync.Mutex
		inFlight := lib.InFlightLimit(maxConcurrentPortScans)
		semaphore := make(chan struct{}, inFlight)
		// scanned counts ports whose probe actually finished (open or
		// closed); a dial aborted by cancellation is neither, and is not
		// counted. plannedScans is what the whole run would cover.
		var openCount, scanned int
		openHosts := make(map[string]bool)
		plannedScans := iterations * len(addresses) * len(ports)

		// Text mode only: --json output must stay one clean document.
		stopProgress := func() {}
		if !*jsonoutput {
			fields := []any{"ports_total", plannedScans}
			if len(addresses) > 1 {
				fields = append(fields, "hosts", len(addresses))
			}
			fmt.Println(lib.LogWithTimestamp(nmapModule, "scan started "+lib.Fields(append(fields, "timeout", fmt.Sprintf("%ds", timeout), "max_in_flight", inFlight)...), false))
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
		for i := 0; i < iterations; i++ { // the whole scan, --count times
			for _, addr := range addresses { // every address of every target
				for _, port := range ports {
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
							return false // added return to exit if error occurs
						}
						select {
						case <-time.After(time.Millisecond * time.Duration(randDelay.Int64())):
						case <-ctx.Done():
							break scanLoop
						}
					}
					WG.Add(1)
					semaphore <- struct{}{}
					go func(addr scanAddress, port int) {
						defer WG.Done()
						defer func() { <-semaphore }()
						_, err := probePort(ctx, addr.ip, port, timeout)
						if err != nil && ctx.Err() != nil {
							// Aborted mid-dial by the scan being cancelled: this
							// port was not found closed, it was never finished.
							return
						}
						statsMutex.Lock()
						scanned++
						if err == nil {
							openCount++
							openHosts[addr.ip] = true
						}
						if *jsonoutput {
							output.Stats = append(output.Stats.([]lib.NmapStats), lib.NmapStats{Target: addr.target, Address: addr.ip, Port: port, Service: serviceName(port), Success: err == nil})
						}
						statsMutex.Unlock()
						if err == nil && !*jsonoutput {
							fields := []any{"host", addr.ip, "port", port}
							if name := serviceName(port); name != "" {
								fields = append(fields, "service", name)
							}
							fmt.Println(lib.LogWithTimestamp(nmapModule, "port open "+lib.Fields(fields...), false))
						}
					}(addr, port)
				}
			}
		}
		WG.Wait()
		stopProgress() // before the summary, so a late tick can't print after it
		// A scan can only fall short of its plan by being cancelled (Ctrl+C
		// or the caller's context). Say so, rather than presenting a partial
		// range as a finished scan.
		summary := []any{"ports_scanned", scanned}
		if scanned < plannedScans {
			summary = append(summary, "ports_total", plannedScans)
		}
		summary = append(summary, "open", openCount)
		if len(addresses) > 1 {
			summary = append(summary, "hosts_with_open_ports", len(openHosts))
		}
		summary = append(summary, "time", time.Since(istart))
		if scanned < plannedScans {
			ok = false
			if *jsonoutput {
				problems = append(problems, fmt.Sprintf("scan interrupted: %d of %d ports scanned", scanned, plannedScans))
			} else {
				fmt.Println(lib.LogWithTimestamp(nmapModule, "scan interrupted "+lib.Fields(summary...), true))
			}
		} else if !*jsonoutput {
			fmt.Println(lib.LogWithTimestamp(nmapModule, "scan complete "+lib.Fields(summary...), false))
		}
	}

	if *jsonoutput {
		output.Error = strings.Join(problems, "; ")
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
	return ok
}
