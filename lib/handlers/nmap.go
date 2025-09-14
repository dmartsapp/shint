package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/tnt/lib"
)

func NmapHandler(ctx context.Context, host string, fromport, endport, iterations, timeout int, throttle bool, jsonoutput *bool) {
	output := lib.JSONOutput{}
	istart := time.Now()
	if *jsonoutput {
		output.InputParams = lib.InputParams{
			Mode:     "nmap",
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
		output.ModuleName = "nmap"
		output.StartTime = istart.UnixMicro()
		output.Stats = make([]lib.NmapStats, 0)
	}

	ipaddresses, err := lib.ResolveName(ctx, host) // resolve DNS
	if err != nil {
		if *jsonoutput {
			output.Error = err.Error()
		} else {
			fmt.Printf("%s ", lib.LogWithTimestamp(err.Error(), true))
		}
	} else { // this is where no error occured in DNS lookup and we can proceed with regular nmap now
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
		var MUTEX sync.RWMutex
		for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
			for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
				for port := fromport; port <= endport; port++ { // we need to loop over all ports individually
					if throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 10000 ms
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
							if *jsonoutput {
								MUTEX.Lock()
								output.Stats = append(output.Stats.([]lib.NmapStats), lib.NmapStats{Address: ip, Port: port, Success: false})
								MUTEX.Unlock()
							}
						} else {
							if *jsonoutput {
								MUTEX.Lock()
								output.Stats = append(output.Stats.([]lib.NmapStats), lib.NmapStats{Address: ip, Port: port, Success: true})
								MUTEX.Unlock()
							} else {
								fmt.Println(lib.LogWithTimestamp(ip+" has port "+strconv.Itoa(port)+" open", false))
							}
						}
					}(ip, port)
				}
			}
		}
		WG.Wait()
	}

	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		JS, jsonErr := json.MarshalIndent(output, "", "  ")
		if jsonErr != nil {
			fmt.Println(lib.LogWithTimestamp(jsonErr.Error(), true))
			os.Exit(1)
		}
		fmt.Println(string(JS))
	} else {
		fmt.Println("Total time taken: " + time.Since(istart).String())
	}
}
