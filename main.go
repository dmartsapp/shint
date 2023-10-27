package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	iterations int
	udp        *bool
	timeout    int
	web        *bool
)

const (
	SuccessNoError   uint8 = 0
	NoSuchHostError  uint8 = 2
	TimeoutError     uint8 = 3
	UnreachableError uint8 = 5
	HttpGetError     uint8 = 4
	UnknownError     uint8 = 1
)

func init() {
	flag.IntVar(&iterations, "count", 1, "Number of times to check")
	flag.IntVar(&timeout, "timeout", 5, "Timeout in seconds to connect")
	udp = flag.Bool("udp", false, "Flag option (Doesn't expect any value after option). Use UDP instead of tcp to connect to endpoint")
	web = flag.Bool("web", false, "Flag option (Doesn't expect any value after option). Use web request.")

	flag.Usage = func() {
		fmt.Println("Usage: " + os.Args[0] + " [options] <fqdn|IP> port")
		fmt.Println("options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example (fqdn): " + os.Args[0] + " google.com 443")
		fmt.Println("Example (IP): " + os.Args[0] + " 10.10.10.10 443")
		fmt.Println("Example (fqdn with -web flag to send 'https' request to path '/pages/index.html' as 'web' client): " + os.Args[0] + " -web https://google.com/pages/index.html")
		os.Exit(int(SuccessNoError))
	}
}

func optionExists(flagname string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == flagname {
			found = true
		}
	})

	return found
}

func resolveName(ipaddress string) *net.IPAddr {
	ip, err := net.ResolveIPAddr("", ipaddress)
	if err != nil {
		fmt.Println(err.Error())
		if strings.Contains(err.Error(), "no such host") {
			os.Exit(int(NoSuchHostError))
		}
		os.Exit(int(UnknownError))

	}
	return ip
}

func main() {

	flag.Parse()
	if !*web {
		if len(flag.Args()) != 2 {
			flag.Usage()
		}
	}

	if !*udp {
		if !*web {
			// this is regular TCP telnet
			port := flag.Args()[1]
			start := time.Now()
			ip := resolveName(flag.Args()[0]).String()
			end := time.Now()
			fmt.Println("Successfully resolved '" + ip + "' to '" + ip + "' in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")

			for i := 0; i < iterations; i++ {
				timetaken := dialNow("tcp", ip+":"+port, timeout)
				if timetaken >= 0 {
					fmt.Println("Successfully reached '" + ip + ":" + port + "' in: " + strconv.Itoa(timetaken) + "ms.")
				} else {
					fmt.Println("Unable to reach '" + ip + ":" + port + "'")
				}
			}
		}

		if *web {
			// this is web request; Check for other flags
			url := flag.Args()[0]
			fmt.Println("Trying to access URL: " + url)
			if matches, _ := regexp.MatchString(`(?:https?://)`, url); matches {

				httpClient := &http.Client{Timeout: time.Second * time.Duration(timeout)}

				if strings.Contains(url, "https") {
					httpsTransport := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					}
					httpClient = &http.Client{Transport: httpsTransport, Timeout: time.Second * time.Duration(timeout)}
				}

				ret := int(SuccessNoError)
				for i := 0; i < iterations; i++ {
					start := time.Now()
					resp, err := httpClient.Get(url)
					end := time.Now()
					if err != nil {
						if strings.Contains(err.Error(), "refused") {
							fmt.Println(url + " is down. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Microseconds())) + "µs")
							ret = (int(UnreachableError))
						}
						if strings.Contains(err.Error(), "Client.Timeout") {
							fmt.Println(url + " is down within elasped timeout. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							ret = (int(TimeoutError))
						}
						if strings.Contains(err.Error(), "reset by peer") {
							fmt.Println(url + ": unable to connect within elasped timeout (Possible protocol mismatch, e.g. http vs https). Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							ret = (int(TimeoutError))
						}
						//fmt.Println(err.Error())
						os.Exit(ret)
					}

					payload, _ := io.ReadAll(resp.Body)

					fmt.Printf("Read: %v bytes.", len(string(payload)))
					defer resp.Body.Close()
					fmt.Print(" HTTP Response code: " + resp.Status + ". ")
					fmt.Println(" Response received in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")
					ret = int(resp.StatusCode)
				}
				os.Exit(int(ret))
			} else {
				fmt.Println(url + " is a not valid http/https url")
			}

		}
	} else {
		// this is for UDP request
		fmt.Println("Placeholder for regular UDP")
	}
}

// func dialNow(protocol string, addressport string, timeout int, wg *sync.WaitGroup) int {
func dialNow(protocol string, addressport string, timeout int) int {
	start := time.Now()
	connect, err := net.DialTimeout(protocol, addressport, time.Duration(timeout)*time.Second)
	end := time.Now()
	if err != nil {

		if strings.Contains(err.Error(), "timeout") {
			return -1

		}
		if strings.Contains(err.Error(), "refused") {
			fmt.Println(addressport + " combination is down. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Microseconds())) + "µs")
			return -1
		}
		// wg.Done()
		fmt.Println(err.Error())
		return -1
	}
	connect.Close()

	// wg.Done()
	return int((end.Sub(start)).Milliseconds())
}
