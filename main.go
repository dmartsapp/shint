package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/farhansabbir/go-ping/netutils"
	"github.com/farhansabbir/telnet/lib"
)

var (
	iterations   int = 1
	delay        int = 1000
	throttle     *bool
	timeout      int = 5
	payload_size int = 4
	web          *bool
	nmap         *bool
	ping         *bool
	version      *bool
	jsonoutput   *bool
	fromport     int = 1
	endport      int = 80
	MUTEX        sync.RWMutex
	Version      string = "0.1BETA"
)

const (
	SuccessNoError         uint8  = 0
	HTTP_CLIENT_USER_AGENT string = "dmarts.app-http-v0.1"
)

func init() {

	flag.IntVar(&iterations, "count", iterations, "Number of times to check connectivity")
	// flag.IntVar(&iterations, "c", iterations, "Number of times to check connectivity")
	flag.IntVar(&timeout, "timeout", timeout, "Timeout in seconds to connect")
	// flag.IntVar(&timeout, "t", timeout, "Timeout in seconds to connect")
	flag.IntVar(&delay, "delay", delay, "Seconds delay between each iteration given in count")
	flag.IntVar(&payload_size, "payload", payload_size, "Ping payload size in bytes")
	web = flag.Bool("web", false, "Use web request as a web client.")
	ping = flag.Bool("ping", false, "Use ICMP echo to test basic reachability")
	throttle = flag.Bool("throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request.")
	nmap = flag.Bool("nmap", false, "Flag option to run tcp port scan. This flag ignores all other parameters except -from and -to, if mentioned.")
	flag.IntVar(&fromport, "from", fromport, "Start port to begin TCP scan from. (applicable with -nmap option only)")
	flag.IntVar(&endport, "to", endport, "End port to run TCP scan to. (applicable with -nmap option only)")
	version = flag.Bool("version", false, "Show version of this tool")
	jsonoutput = flag.Bool("json", false, "Flag option to output only in JSON format")

	flag.Usage = func() {
		fmt.Println("Version: " + Version)
		fmt.Println("Usage: " + os.Args[0] + " [options] <fqdn|IP> port")
		fmt.Println("options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example (fqdn): " + os.Args[0] + " google.com 443")
		fmt.Println("Example (IP): " + os.Args[0] + " 10.10.10.10 443")
		// fmt.Println("Example (ping with timeout of 1s and count of 10 for every IP addresses resolved): " + os.Args[0] + " -ping -count 10 -timeout 1 google.com")
		// fmt.Println("Example (fqdn with -web flag to send 'https' request to path '/pages/index.html' as client with user-agent set as '" + HTTP_CLIENT_USER_AGENT + "'): " + os.Args[0] + " -web https://google.com/pages/index.html")
		os.Exit(int(SuccessNoError))
	}
}

type WebRequest struct {
	url   string
	stats map[string][]int
}

func NewRequest(url string) *WebRequest {
	return &WebRequest{
		url:   url,
		stats: make(map[string][]int),
	}
}

func main() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				Version = setting.Value[:9]
			}
			if setting.Key == "vcs.time" {
				Version += " " + setting.Value
			}
		}
	}
	flag.Parse() // read the flags passed for processing

	// if (!*web) && (!*nmap) && (!*version) && (!*ping) { // ping, nmap and web needs single param like -nmap 10.10.18.121 or "-web https://google.com" respectively, while telnet needs two parameters like 10.10.18.121 22 for IP and Port respectively
	if (!*web) && (!*nmap) && (!*version) && (!*ping) { // nmap and web needs single param like -nmap 10.10.18.121 or "-web https://google.com" respectively, while telnet needs two parameters like 10.10.18.121 22 for IP and Port respectively
		if len(flag.Args()) != 2 { // telnet only needs 2 params, so show usage and exit for additional parameters
			flag.Usage()
			os.Exit(int(SuccessNoError))
		}
	}
	if *version {
		fmt.Println("Version: " + Version)
		os.Exit(0)
	}
	// setting up timeout context to ensure we exit after defined timeout
	CTXTIMEOUT, CANCEL := context.WithTimeout(context.Background(), time.Duration(time.Second*time.Duration(timeout)))
	defer CANCEL()

	if *nmap {
		istart := time.Now()                                         // capture initial time
		ipaddresses, err := lib.ResolveName(CTXTIMEOUT, flag.Arg(0)) // resolve DNS
		var stats = make([]time.Duration, 0)
		if err != nil {
			fmt.Printf("%s ", lib.LogWithTimestamp(err.Error(), true))
			fmt.Println(lib.LogStats("telnet", stats, iterations))
		} else { // this is where no error occured in DNS lookup and we can proceed with regular nmap now
			fmt.Println(lib.LogWithTimestamp("DNS lookup successful for "+flag.Arg(0)+"' to "+strconv.Itoa(len(ipaddresses))+" addresses '["+strings.Join(ipaddresses[:], ", ")+"]' in "+time.Since(istart).String(), false))
			var WG sync.WaitGroup
			// var MUTEX sync.RWMutex
			for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
				for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
					for port := fromport; port <= endport; port++ { // we need to loop over all ports individually
						if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 10000 ms
							i, err := rand.Int(rand.Reader, big.NewInt(10000))
							if err != nil {
								fmt.Println(err)
								return // added return to exit if error occurs
							}
							time.Sleep(time.Millisecond * time.Duration(i.Int64()))
						}
						WG.Add(1)
						go func(ip string, port int) {
							defer WG.Done()
							_, err := lib.IsPortUp(ip, port, timeout) // check if given port from this iteration is up or not
							if err != nil {

							} else {
								fmt.Println(lib.LogWithTimestamp(ip+" has port "+strconv.Itoa(port)+" open", false))
							}
						}(ip, port)
					}
				}
			}
			WG.Wait()
		}
		fmt.Println("Total time taken: " + time.Since(istart).String())
	} else if *web {
		if len(flag.Args()) <= 0 {
			fmt.Println(lib.LogWithTimestamp("Missing URL", true))
			os.Exit(1)
		}
		istart := time.Now()
		URL, err := url.Parse(flag.Arg(0))
		if err != nil {
			fmt.Println(lib.LogWithTimestamp(err.Error(), true))
			os.Exit(1)
		}
		var WG sync.WaitGroup
		for i := 0; i < iterations; i++ {
			if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 1000 ms
				i, err := rand.Int(rand.Reader, big.NewInt(10000))
				if err != nil {
					fmt.Println(err)
					return // added return to exit if error occurs
				}
				time.Sleep(time.Millisecond * time.Duration(i.Int64()))
			}
			WG.Add(1)
			go func(URL *url.URL) {
				defer WG.Done()
				client := &http.Client{
					Timeout: time.Duration(time.Duration(timeout) * time.Second),
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12},
					},
				} // setup http transport not to validate the SSL certificate

				request, err := http.NewRequest("GET", flag.Arg(0), nil) // only setup for get requests
				if err != nil {
					if strings.Contains(err.Error(), "tls") {
						fmt.Println(lib.LogWithTimestamp(err.Error(), true))
						return
					} else {
						return
					}

				}
				request.Header.Set("user-agent", HTTP_CLIENT_USER_AGENT) // set the header for the user-agent
				start := time.Now()                                      // capture initial time
				response, err := client.Do(request)
				if err != nil {
					fmt.Println(lib.LogWithTimestamp(err.Error(), true))
					return
				}
				defer response.Body.Close()
				body, _ := io.ReadAll(response.Body) // read the entire body, this should consume most of the time
				time_taken := time.Since(start)      //capture the time taken
				stats := make(map[string]int, 0)
				stats["time_taken"] = int(time_taken)
				// fmt.Println(float64(len(string(body))) / float64(time_taken.Seconds()))
				fmt.Println(lib.LogWithTimestamp("Response: "+response.Status+", bytes downloaded: "+strconv.Itoa(len(string(body)))+", speed: "+strconv.FormatFloat((float64(len(string(body)))/float64(time_taken.Seconds())/1024), 'G', -1, 64)+"KB/s, time taken: "+time_taken.String(), false))

			}(URL)
		}
		WG.Wait()
		fmt.Println("Total time taken: " + time.Since(istart).String())
	} else if *ping {
		if len(flag.Args()) <= 0 {
			fmt.Println(lib.LogWithTimestamp("Missing URL/address to ping", true))
			os.Exit(1)
		}
		output := lib.JSONOutput{}
		output.InputParams = lib.InputParams{
			Mode:     "icmp",
			Host:     flag.Arg(0),
			FromPort: int(7),
			ToPort:   int(7),
			Protocol: "icmp",
			Timeout:  timeout,
			Count:    iterations,
			Delay:    delay,
			Payload:  payload_size,
			Throttle: *throttle,
		}
		output.ModuleName = "icmp"
		start := time.Now()
		pinger, err := netutils.NewPinger(flag.Arg(0))
		if err != nil {
			fmt.Println(lib.LogWithTimestamp(err.Error(), true))
			os.Exit(1)
		}

		wg := sync.WaitGroup{}

		if !*jsonoutput {
			wg.Add(1)
			go func(pinger *netutils.Pinger, wg *sync.WaitGroup) {
				defer wg.Done()
				for log := range pinger.StreamLog() {
					fmt.Println(lib.LogWithTimestamp(log, false))
				}
			}(pinger, &wg)
		}

		pinger.
			SetPingCount(iterations).
			SetParallelPing(true).
			SetPayloadSizeInBytes(payload_size).
			SetPingDelayInMS(delay).
			SetRandomizedPingDelay(*throttle)
		err = pinger.PingAll()
		if err != nil {
			fmt.Println(lib.LogWithTimestamp(err.Error(), true))
			os.Exit(1)
		}

		wg.Wait()
		pinger.MeasureStats()
		// pinger.MeasureStats()
		if !*jsonoutput {
			fmt.Println("========================================= Ping stats ============================================")
			fmt.Printf("Packets sent: %d, Packets received: %d, Packets lost: %d, Ping success: %d%% \n", pinger.Count*len(pinger.Destination), (pinger.Count*len(pinger.Destination) - pinger.Stats.Loss), pinger.Stats.Loss, ((pinger.Count*len(pinger.Destination) - pinger.Stats.Loss) * 100 / (pinger.Count * len(pinger.Destination))))
			fmt.Printf("Total time: %v, Resolve time: %v\n", pinger.Stats.TotalTime, pinger.Stats.ResolveTime)
			fmt.Printf("Min time: %dms, Max time: %dms, Avg time: %.3fms, Std dev: %.3f, Total time: %v\n", pinger.Stats.Min, pinger.Stats.Max, pinger.Stats.Avg, pinger.Stats.StdDev, pinger.Stats.TotalTime)
		} else {
			output.DNSLookup = lib.DNSLookup{
				Hostname:          flag.Arg(0),
				Success:           true,
				ResolvedAddresses: lib.ConvertIPToStringSlice(pinger.Destination),
				TimeTaken:         pinger.Stats.ResolveTime.Microseconds(),
			}
			output.StartTime = start.UnixMicro()
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			output.Stats = make([]lib.ICMPStats, 0)
			for _, pckts := range pinger.Stats.Packets {
				stat := lib.ICMPStats{}
				stat.Address = pckts.Destination.String()
				stat.Success = !pckts.ErrorEncountered
				stat.Sequence = pckts.Sequence
				stat.SentTime = pckts.SentDateTimeUNIX
				stat.RecvTime = pckts.ReceiveDateTimeUNIX
				stat.TimeTaken = stat.RecvTime - stat.SentTime
				output.Stats = append(output.Stats.([]lib.ICMPStats), stat)
			}

			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))

		}

	} else { // this should be ideally telnet if not web or nmap or ping
		port, err := strconv.ParseUint(flag.Arg(1), 10, 64)
		if err != nil {
			fmt.Println(lib.LogWithTimestamp("Invalid port '"+flag.Arg(1)+"'", true))
			flag.Usage()
			os.Exit(1)
		}
		output := lib.JSONOutput{}
		output.InputParams = lib.InputParams{
			Mode:     "telnet",
			Host:     flag.Arg(0),
			FromPort: int(port),
			ToPort:   int(port),
			Protocol: "tcp",
			Timeout:  timeout,
			Count:    iterations,
			Delay:    delay,
			Payload:  payload_size,
			Throttle: *throttle,
		}
		output.ModuleName = "telnet"
		istart := time.Now()                                         // capture initial time
		ipaddresses, err := lib.ResolveName(CTXTIMEOUT, flag.Arg(0)) // resolve DNS
		var stats = make([]time.Duration, 0)
		if err != nil {
			if *jsonoutput {
				output.DNSLookup = lib.DNSLookup{
					Hostname:          flag.Arg(0),
					Success:           false,
					ResolvedAddresses: nil,
					TimeTaken:         time.Since(istart).Microseconds(),
				}
			} else {
				fmt.Printf("%s ", lib.LogWithTimestamp(err.Error(), true))
				fmt.Println(lib.LogStats("telnet", stats, iterations))
			}
		} else {
			if !*jsonoutput {
				fmt.Println(lib.LogWithTimestamp("DNS lookup successful for "+flag.Arg(0)+"' to "+strconv.Itoa(len(ipaddresses))+" addresses '["+strings.Join(ipaddresses[:], ", ")+"]' in "+time.Since(istart).String(), false))
			} else {
				output.DNSLookup = lib.DNSLookup{
					Hostname:          flag.Arg(0),
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
				for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
					if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 1000 ms
						// time.Sleep(time.Millisecond * time.Duration(rand.Intn(10000)))
						in, err := rand.Int(rand.Reader, big.NewInt(10000))
						if err != nil {
							fmt.Println(err)
						}
						delay = int(in.Int64())
					}
					time.Sleep(time.Millisecond * time.Duration(delay))
					WG.Add(1)
					go func(ip string) {
						defer WG.Done()
						start := time.Now()                            // capture initial time
						_, err := lib.IsPortUp(ip, int(port), timeout) // check if given port from this iteration is up or not
						if err != nil {
							if *jsonoutput {
								stat := lib.TelnetStats{}
								stat.Address = ip
								stat.Success = false
								stat.TimeTaken = time.Since(start).Microseconds()
								output.Stats = append(output.Stats.([]lib.TelnetStats), stat)
							} else {
								fmt.Println(lib.LogWithTimestamp(err.Error()+" Time taken: "+time.Since(start).String(), true))
							}
						} else {
							MUTEX.Lock()
							time_taken := time.Since(start) //capture the time taken
							stats = append(stats, time_taken)
							defer MUTEX.Unlock()
							if *jsonoutput {
								stat := lib.TelnetStats{}
								stat.Address = ip
								stat.Success = true
								stat.SentTime = start.UnixMicro()
								stat.RecvTime = time.Now().UnixMicro()
								stat.TimeTaken = time.Since(start).Microseconds()
								output.Stats = append(output.Stats.([]lib.TelnetStats), stat)
							} else {
								fmt.Println(lib.LogWithTimestamp("Successfully connected to "+ip+" on port "+strconv.Itoa(int(port))+" after "+time_taken.String(), false))
							}

						}
					}(ip)
				}
			}
			WG.Wait()
			if !*jsonoutput {
				MUTEX.RLock()
				fmt.Println(lib.LogStats("telnet", stats, (iterations * len(ipaddresses))))
				MUTEX.RUnlock()
			}
		}

		if *jsonoutput {
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		} else {
			fmt.Println("Total time taken: " + time.Since(istart).String())
		}
	}
}
