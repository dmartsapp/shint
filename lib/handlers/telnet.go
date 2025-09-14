package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/tnt/lib"
)

func TelnetHandler(jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, payload_size int, port int, CTXTIMEOUT context.Context, host string) {
	var MUTEX sync.RWMutex
	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     "telnet",
		Host:     host,
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
	istart := time.Now()                                  // capture initial time
	ipaddresses, err := lib.ResolveName(CTXTIMEOUT, host) // resolve DNS
	var stats = make([]time.Duration, 0)
	if err != nil {
		if *jsonoutput {
			output.DNSLookup = lib.DNSLookup{
				Hostname:          host,
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
			fmt.Println(lib.LogWithTimestamp("DNS lookup successful for "+host+"' to "+strconv.Itoa(len(ipaddresses))+" addresses '["+strings.Join(ipaddresses[:], ", ")+"]' in "+time.Since(istart).String(), false))
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
