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
	"sync/atomic"
	"time"

	"github.com/dmartsapp/shint/lib"
)

const (
	// HTTP_CLIENT_USER_AGENT is the default User-Agent; -H "User-Agent: ..."
	// overrides it.
	HTTP_CLIENT_USER_AGENT string = "dmarts.app-http-v0.1"
	webModule              string = "web"
)

// WebHandler makes the request iterations times and reports true only if every
// one of them got an HTTP response. The status code is data, not a verdict
// (as with curl without -f): a 404 or 500 is a response, and is reported as
// such; a refused connection, a timeout, a TLS failure or DNS failure is not.
// ctx is cancelled by Ctrl+C and nothing else - never a deadline; see
// interrupt.go for how a cancelled run ends.
func WebHandler(ctx context.Context, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, URL *url.URL, method string, data string, headers []string, includeresponsebody bool, tlsConfig *tls.Config) (ok bool) {
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

		ipaddresses, err := lib.ResolveName(ctx, URL.Hostname())
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
		resolvedIPs, err := lib.ResolveNameToIPs(ctx, URL.Hostname())
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

	// One client for the whole run. Timeout bounds each request end to end
	// (connect, TLS, response); the transport counts bytes on the wire (see
	// wireconn.go). Redirects are followed (net/http's default, up to 10).
	client := &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: newCountingTransport(tlsConfig),
	}

	var WG sync.WaitGroup
	var failed int32    // attempts that got no HTTP response
	var completed int32 // attempts that finished, whatever the outcome
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		if ctx.Err() != nil {
			break
		}
		if *throttle { // check if throttle is enable, then slow things down a bit of random milisecond wait between 0 1000 ms
			randDelay, err := rand.Int(rand.Reader, big.NewInt(10000))
			if err != nil {
				if !*jsonoutput {
					fmt.Println(err)
				}
				return false
			}
			if !pause(ctx, time.Millisecond*time.Duration(randDelay.Int64())) {
				break
			}
		} else if !pause(ctx, time.Millisecond*time.Duration(delay)) {
			break
		}
		WG.Add(1)
		go func(URL *url.URL, attempt int) {
			defer WG.Done()
			errors := make([]string, 0)
			var meter wireMeter

			// fail records an attempt that got no response: as an ERROR log
			// line in text mode, or as a stats entry with success=false in
			// --json mode - never a stray text line among the JSON.
			fail := func(stage string, request *http.Request, start time.Time, err error) {
				atomic.AddInt32(&failed, 1)
				atomic.AddInt32(&completed, 1)
				elapsed := time.Since(start)
				if !*jsonoutput {
					fmt.Println(lib.LogWithTimestamp(webModule, stage+" "+lib.Fields("url", URL.String(), "attempt", fmt.Sprintf("%d/%d", attempt, iterations), "time", elapsed, "error", err.Error()), true))
					return
				}
				reqHeaders := http.Header{}
				if request != nil {
					reqHeaders = request.Header
				}
				sent, received := meter.totals()
				stat := lib.WebStats{
					URL:           URL.String(),
					Errors:        append(append([]string{}, errors...), err.Error()),
					Request:       map[string]any{"method": method, "body": data, "headers": reqHeaders},
					Response:      map[string]any{},
					Success:       false,
					SentTime:      start.UnixMicro(),
					RecvTime:      time.Now().UnixMicro(),
					TimeTaken:     elapsed.Microseconds(),
					BytesSent:     sent,
					BytesReceived: received,
				}
				statsMutex.Lock()
				output.Stats = append(output.Stats.([]lib.WebStats), stat)
				statsMutex.Unlock()
			}

			request, err := http.NewRequestWithContext(ctx, method, URL.String(), strings.NewReader(data))
			if err != nil {
				fail("request build failed", nil, time.Now(), err)
				return
			}
			request.Header.Set("user-agent", HTTP_CLIENT_USER_AGENT)
			// -H values are "Name: value"; a malformed one is skipped and
			// reported in the JSON "errors" field.
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
			request = request.WithContext(httptrace.WithClientTrace(request.Context(), meter.clientTrace()))

			start := time.Now()
			response, err := client.Do(request)
			if err != nil {
				if ctx.Err() != nil {
					return // cut off by Ctrl+C: never finished, so it neither passed nor failed
				}
				fail("request failed", request, start, err)
				return
			}
			defer func() { _ = response.Body.Close() }()
			body, readErr := io.ReadAll(response.Body)
			if readErr != nil {
				if ctx.Err() != nil {
					return // cut off by Ctrl+C while the body was arriving
				}
				// A body that ends early (the server closed the connection short of its
				// Content-Length, reset it, or stalled until --timeout) is a response that
				// did not arrive: a failed attempt, not a success with fewer bytes.
				fail("response incomplete", request, start, readErr)
				return
			}
			atomic.AddInt32(&completed, 1)
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
	done := int(atomic.LoadInt32(&completed))
	interrupted := done < iterations // only Ctrl+C keeps an attempt from being made
	if *jsonoutput {
		output.InputParams.Headers = headers
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		output.Error = ""
		if interrupted {
			output.Error = interruptedNote(done, iterations)
		}
		JS, jsonErr := json.MarshalIndent(output, "", "  ")
		if jsonErr != nil {
			fmt.Println(lib.LogWithTimestamp(webModule, jsonErr.Error(), true))
			os.Exit(1)
		}
		fmt.Println(string(JS))
	} else {
		statsMutex.Lock()
		if interrupted {
			fmt.Println(interruptedLine(webModule, done, iterations, istart))
			fmt.Println(lib.LogStats(webModule, stats, done))
		} else {
			fmt.Println(lib.LogStats(webModule, stats, iterations))
		}
		statsMutex.Unlock()
		fmt.Println(lib.LogWithTimestamp(webModule, "done "+lib.Fields("total_time", time.Since(istart)), false))
	}
	return atomic.LoadInt32(&failed) == 0 && !interrupted
}

func parsePort(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return lib.ValidatePort(raw)
}
