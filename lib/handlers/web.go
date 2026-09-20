package handlers

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/shint/lib"
)

const (
	HTTP_CLIENT_USER_AGENT string = "dmarts.app-http-v0.1"
	webModule              string = "web"
)

func WebHandler(jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, URL *url.URL, method string, data string, headers []string, includeresponsebody bool, tlsConfig *tls.Config) {
	output := lib.JSONOutput{}
	istart := time.Now()
	var stats = make([]time.Duration, 0)
	var statsMutex sync.Mutex

	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	if !*jsonoutput {
		if tlsConfig.InsecureSkipVerify {
			fmt.Println(lib.LogWithTimestamp(webModule, "tls verification disabled "+lib.Fields("warning", "certificate checks are skipped, response may not be trustworthy"), true))
		}
		if len(tlsConfig.Certificates) > 0 {
			fmt.Println(lib.LogWithTimestamp(webModule, "using client certificate for mutual TLS", false))
		}
		if tlsConfig.RootCAs != nil {
			fmt.Println(lib.LogWithTimestamp(webModule, "using custom CA bundle to verify server certificate", false))
		}

		ipaddresses, err := lib.ResolveName(context.Background(), URL.Hostname())
		if err != nil {
			fmt.Println(lib.LogWithTimestamp(webModule, "dns resolution failed "+lib.Fields("host", URL.Hostname(), "error", err.Error()), true))
		} else {
			fmt.Println(lib.LogWithTimestamp(webModule, "dns resolved "+lib.Fields("host", URL.Hostname(), "addresses", len(ipaddresses), "ips", "["+strings.Join(ipaddresses, ",")+"]", "time", time.Since(istart)), false))
		}
	}

	if *jsonoutput {
		output.InputParams = lib.InputParams{
			Mode:     webModule,
			Host:     URL.Host,
			Protocol: "tcp",
			Timeout:  timeout,
			Count:    iterations,
			Delay:    delay,
			Payload:  len(data),
			Throttle: *throttle,
			Method:   method,
			Data:     data,
			Headers:  headers,
		}
		output.ModuleName = webModule
		resolvedIPs, err := lib.ResolveNameToIPs(context.Background(), URL.Hostname())
		if err != nil {
			output.DNSLookup = lib.DNSLookup{Hostname: URL.Hostname()}
			output.Error = err.Error()
			output.DNSLookup.Error = err.Error()
			output.DNSLookup.Success = false
			output.DNSLookup.TimeTaken = time.Since(istart).Microseconds()
		} else {
			output.DNSLookup = lib.DNSLookup{Hostname: URL.Hostname()}
			output.DNSLookup.Success = true
			output.DNSLookup.ResolvedAddresses = lib.ConvertIPToStringSlice(resolvedIPs)
			output.DNSLookup.TimeTaken = time.Since(istart).Microseconds()
		}

		if port, err := parsePort(URL.Port()); err == nil && port != 0 {
			output.InputParams.FromPort = port
		} else if URL.Scheme == "https" {
			output.InputParams.FromPort = 443
		} else {
			output.InputParams.FromPort = 80
		}
		output.InputParams.ToPort = output.InputParams.FromPort
		output.StartTime = istart.UnixMicro()
		output.Stats = make([]lib.WebStats, 0)
	}

	client := &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: newCountingTransport(tlsConfig),
	}

	var WG sync.WaitGroup
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 1000 ms
			randDelay, err := rand.Int(rand.Reader, big.NewInt(10000))
			if err != nil {
				if !*jsonoutput {
					fmt.Println(err)
				}
				return
			}
			time.Sleep(time.Millisecond * time.Duration(randDelay.Int64()))
		} else if delay > 0 {
			time.Sleep(time.Millisecond * time.Duration(delay))
		}
		WG.Add(1)
		go func(URL *url.URL, attempt int) {
			defer WG.Done()
			errors := make([]string, 0)

			request, err := http.NewRequest(method, URL.String(), strings.NewReader(data))
			if err != nil {
				fmt.Println(lib.LogWithTimestamp(webModule, "request build failed "+lib.Fields("url", URL.String(), "error", err.Error()), true))
				return
			}
			request.Header.Set("user-agent", HTTP_CLIENT_USER_AGENT)
			for _, h := range headers {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					request.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				} else {
					errors = append(errors, "invalid header format: "+fmt.Sprint(h))
				}
			}

			// Bytes are counted on the connection itself, so they cover
			// everything that actually travelled (request line, headers and
			// body out; status line, headers and body in), the same way
			// listen http counts them - and every hop, if a redirect is followed.
			var meter wireMeter
			request = request.WithContext(httptrace.WithClientTrace(request.Context(), meter.clientTrace()))

			start := time.Now()
			response, err := client.Do(request)
			if err != nil {
				fmt.Println(lib.LogWithTimestamp(webModule, "request failed "+lib.Fields("url", URL.String(), "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", time.Since(start), "error", err.Error()), true))
				return
			}
			defer func() { _ = response.Body.Close() }()
			body, _ := io.ReadAll(response.Body)
			header := response.Header
			timeTaken := time.Since(start)
			bytesSent, bytesReceived := meter.totals()
			bandwidthKBs := float64(bytesReceived) / timeTaken.Seconds() / 1024

			statsMutex.Lock()
			stats = append(stats, timeTaken)
			if *jsonoutput {
				stat := lib.WebStats{}
				stat.URL = URL.String()
				if includeresponsebody {
					var jsondata interface{}
					if jsonErr := json.Unmarshal(body, &jsondata); jsonErr != nil {
						errors = append(errors, "JSON parse error: "+jsonErr.Error())
						stat.Response = map[string]any{"body": string(body), "header": header}
					} else {
						stat.Response = map[string]any{"body": jsondata, "header": header}
					}
				} else {
					stat.Response = map[string]any{"header": header}
				}
				stat.Request = map[string]any{"method": method, "body": data, "headers": request.Header}
				stat.Success = true
				stat.StatusCode = response.StatusCode
				stat.BytesSent = bytesSent
				stat.BytesReceived = bytesReceived
				stat.BandwidthKBs = bandwidthKBs
				stat.SentTime = start.UnixMicro()
				stat.RecvTime = time.Now().UnixMicro()
				stat.TimeTaken = timeTaken.Microseconds()
				stat.Errors = errors
				output.Stats = append(output.Stats.([]lib.WebStats), stat)
			}
			statsMutex.Unlock()

			if !*jsonoutput {
				fmt.Println(lib.LogWithTimestamp(webModule, "response "+lib.Fields("url", URL.String(), "status", response.StatusCode, "bytes_sent", bytesSent, "bytes_received", bytesReceived, "speed", fmt.Sprintf("%.2fKB/s", bandwidthKBs), "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", timeTaken), false))
			}
		}(URL, attempt)
	}
	WG.Wait()
	if *jsonoutput {
		output.InputParams.Headers = headers
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		output.Error = ""
		JS, jsonErr := json.MarshalIndent(output, "", "  ")
		if jsonErr != nil {
			fmt.Println(lib.LogWithTimestamp(webModule, jsonErr.Error(), true))
			os.Exit(1)
		}
		fmt.Println(string(JS))
	} else {
		statsMutex.Lock()
		fmt.Println(lib.LogStats(webModule, stats, iterations))
		statsMutex.Unlock()
		fmt.Println(lib.LogWithTimestamp(webModule, "done "+lib.Fields("total_time", time.Since(istart)), false))
	}
}

func parsePort(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return lib.ValidatePort(raw)
}
