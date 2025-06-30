package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"crypto/rand"
	"context"

	"github.com/farhansabbir/telnet/lib"
)

const (
	HTTP_CLIENT_USER_AGENT string = "dmarts.app-http-v0.1"
)

func WebHandler(jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, URL *url.URL) {
	output := lib.JSONOutput{}
	istart := time.Now()
	var stats = make([]time.Duration, 0)
	var MUTEX sync.RWMutex

	if !*jsonoutput {
		ipaddresses, err := lib.ResolveName(context.Background(), URL.Hostname())
		if err != nil {
			fmt.Printf("%s ", lib.LogWithTimestamp(err.Error(), true))
		} else {
			fmt.Println(lib.LogWithTimestamp("DNS lookup successful for "+URL.Hostname()+"' to "+strconv.Itoa(len(ipaddresses))+" addresses '["+strings.Join(ipaddresses[:], ", ")+"]' in "+time.Since(istart).String(), false))
		}
	}

	if *jsonoutput {
		output.InputParams = lib.InputParams{
			Mode:     "web",
			Host:     URL.Host,
			Protocol: "tcp",
			Timeout:  timeout,
			Count:    iterations,
			Delay:    delay,
			Payload:  0,
			Throttle: *throttle,
		}
		output.ModuleName = "web"
		output.InputParams.Host = URL.Host
		output.DNSLookup = lib.DNSLookup{
			Hostname: URL.Hostname(),
		}
		output.InputParams.FromPort, _ = strconv.Atoi(URL.Port())
		output.InputParams.ToPort = output.InputParams.FromPort
		output.StartTime = istart.UnixMicro()
		output.Stats = make([]lib.WebStats, 0)
	}

	var WG sync.WaitGroup
	for i := 0; i < iterations; i++ {
		if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 1000 ms
			i, err := rand.Int(rand.Reader, big.NewInt(10000))
			if err != nil {
				if !*jsonoutput {
					fmt.Println(err)
					return // added return to exit if error occurs
				} else {
					output.Error = err.Error()
					return
				}
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

			request, err := http.NewRequest("GET", URL.String(), nil) // only setup for get requests
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
			header := response.Header
			time_taken := time.Since(start) //capture the time taken
			
			MUTEX.Lock()
			stats = append(stats, time_taken)
			MUTEX.Unlock()

			if *jsonoutput {
				stat := lib.WebStats{}
				stat.URL = URL.String()
				stat.Success = true
				stat.StatusCode = response.StatusCode
				stat.BytesDownloaded = len(string(body)) + len(header)
				stat.SentTime = start.UnixMicro()
				stat.RecvTime = time.Now().UnixMicro()
				stat.TimeTaken = stat.RecvTime - stat.SentTime
				output.Stats = append(output.Stats.([]lib.WebStats), stat)
			} else {
				fmt.Println(lib.LogWithTimestamp("Response: "+response.Status+", bytes downloaded: "+strconv.Itoa(len(string(body)))+", speed: "+strconv.FormatFloat((float64(len(string(body)))/float64(time_taken.Seconds())/1024), 'G', -1, 64)+"KB/s, time taken: "+time_taken.String(), false))
			}
		}(URL)
	}
	WG.Wait()
	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		output.Error = ""
		JS, jsonErr := json.MarshalIndent(output, "", "  ")
		if jsonErr != nil {
			fmt.Println(lib.LogWithTimestamp(jsonErr.Error(), true))
			os.Exit(1)
		}
		fmt.Println(string(JS))
	} else {
		MUTEX.RLock()
		fmt.Println(lib.LogStats("web", stats, iterations))
		MUTEX.RUnlock()
		fmt.Println("Total time taken: " + time.Since(istart).String())
	}
}
