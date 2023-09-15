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
	download   *bool
	iterations int
	udp        *bool
	timeout    int
	httpOnly   *bool
	web        *bool
	path       string
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
	flag.StringVar(&path, "path", "/", "Path to send web request to. Requires 'web' flag set first.")
	download = flag.Bool("download", false, "Flag option (Doesn't expect any value after option). Download the contents of web request and print to STDOUT. Requires 'web' flag.")
	httpOnly = flag.Bool("http", false, "Flag option (Doesn't expect any value after option). Use http instead of default https for web requests. Requires 'web' flag.")
	web = flag.Bool("web", false, "Flag option (Doesn't expect any value after option). Use web request on top of regular telnet. 'http' and 'download' flags and 'path' option only works if this flag is used.")

	flag.Usage = func() {
		fmt.Println("Usage: " + os.Args[0] + " [options] <fqdn|IP> port")
		fmt.Println("options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example (fqdn): " + os.Args[0] + " google.com 443")
		fmt.Println("Example (IP): " + os.Args[0] + " 10.10.10.10 443")
		fmt.Println("Example (fqdn with -web and -http flags to send 'http' request to path '/pages/index.html' as 'web' client): " + os.Args[0] + " -web -http -path '/pages/index.html' 10.10.10.10 443")
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
	if len(flag.Args()) != 2 {
		flag.Usage()
	}

	// var ip *net.IPAddr

	// to := time.AfterFunc(time.Duration(0*int(time.Second)), func() {
	// 	IP, err := net.ResolveIPAddr("", flag.Args()[0])
	// 	if err != nil {
	// 		fmt.Println(err.Error())
	// 		if strings.Contains(err.Error(), "no such host") {
	// 			os.Exit(int(NoSuchHostError))
	// 		}
	// 		os.Exit(int(UnknownError))

	// 	}
	// 	ip = IP
	// })
	// fmt.Println(time.Now())
	// time.Sleep(time.Duration(timeout * int(time.Second)))
	// to.Stop()
	// fmt.Println(ip)

	regex, _ := regexp.Compile("[a-z|A-Z]")

	if !*udp {
		ip := flag.Args()[0]
		port := flag.Args()[1]
		if regex.MatchString(flag.Args()[0]) {
			start := time.Now()
			ip = resolveName(flag.Args()[0]).String()
			end := time.Now()
			fmt.Println("Successfully resolved '" + flag.Args()[0] + "' to '" + ip + "' in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")

		}

		if *web {
			// this is web request; Check for other flags
			scheme := "https"
			getpath := "/"

			if *httpOnly {
				scheme = "http"
			}
			if optionExists("path") {
				getpath = path
			}
			test := dialNow("tcp", ip+":"+port, 2)
			if test == -1 {
				fmt.Println("Could not establish tcp connection to the IP port")
				os.Exit(1)
			}
			url := scheme + "://" + ip + ":" + port + getpath
			// fmt.Println("Trying to access URL: " + url)
			if *download {
				httpClient := &http.Client{Timeout: time.Second * time.Duration(timeout)}
				if !*httpOnly {
					httpsTransport := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					}
					httpClient = &http.Client{Transport: httpsTransport, Timeout: time.Second * time.Duration(timeout)}
				}
				ret := int(SuccessNoError)
				fmt.Println(`Trying to access url: ` + url)
				for i := 0; i < iterations; i++ {
					start := time.Now()
					resp, err := httpClient.Get(url)
					end := time.Now()
					if err != nil {
						if strings.Contains(err.Error(), "refused") {
							fmt.Println(url + " is down. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Microseconds())) + "µs")
							os.Exit(int(UnreachableError))
						}
						if strings.Contains(err.Error(), "Client.Timeout") {
							fmt.Println(url + " is down within elasped timeout. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							os.Exit(int(TimeoutError))
						}
						if strings.Contains(err.Error(), "reset by peer") {
							fmt.Println(url + ": unable to connect within elasped timeout (Possible protocol mismatch, e.g. http vs https). Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							os.Exit(int(TimeoutError))
						}
						fmt.Println(err.Error())
						os.Exit(int(HttpGetError))
					}

					payload, _ := io.ReadAll(resp.Body)
					fmt.Println("\nHeaders")
					for key, value := range resp.Header {
						fmt.Println(key + ":" + strings.Join(value, ""))
					}
					fmt.Println("\n\n")
					fmt.Println(string(payload))

					fmt.Printf("\nRead: %v bytes.\n", len(string(payload)))
					defer resp.Body.Close()
					fmt.Print("HTTP Response code: " + resp.Status + ". ")
					fmt.Println("Response received in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")
					ret = int(resp.StatusCode)
				}
				os.Exit(int(ret))
			} else {
				httpClient := &http.Client{Timeout: time.Second * time.Duration(timeout)}
				if !*httpOnly {
					httpsTransport := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					}
					httpClient = &http.Client{Transport: httpsTransport, Timeout: time.Second * time.Duration(timeout)}
				}
				ret := int(SuccessNoError)
				fmt.Println(`Trying to access url: ` + url)
				for i := 0; i < iterations; i++ {
					start := time.Now()
					resp, err := httpClient.Get(url)
					end := time.Now()
					if err != nil {
						if strings.Contains(err.Error(), "refused") {
							fmt.Println(url + " is down. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Microseconds())) + "µs")
							os.Exit(int(UnreachableError))
						}
						if strings.Contains(err.Error(), "Client.Timeout") {
							fmt.Println(url + " is down within elasped timeout. Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							os.Exit(int(TimeoutError))
						}
						if strings.Contains(err.Error(), "reset by peer") {
							fmt.Println(url + ": unable to connect within elasped timeout (Possible protocol mismatch, e.g. http vs https). Elapsed time: " + strconv.Itoa(int(end.Sub(start).Seconds())) + "s")
							os.Exit(int(TimeoutError))
						}
						fmt.Println(err.Error())
						os.Exit(int(HttpGetError))
					}

					payload, _ := io.ReadAll(resp.Body)
					// fmt.Println("\nHeaders")
					// for key, value := range resp.Header {
					// 	fmt.Println(key + ":" + strings.Join(value, ""))
					// }
					// fmt.Println("\nBody")
					// fmt.Println(string(payload))

					fmt.Printf("\nRead: %v bytes.\n", len(string(payload)))
					defer resp.Body.Close()
					fmt.Print("HTTP Response code: " + resp.Status + ". ")
					fmt.Println("Response received in: " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")
					ret = int(resp.StatusCode)
				}
				os.Exit(int(ret))

			}

		} else {

			// this is regular TCP telnet
			for i := 0; i < iterations; i++ {
				timetaken := dialNow("tcp", ip+":"+port, timeout)
				if timetaken >= 0 {
					fmt.Println("Successfully reached '" + ip + ":" + port + "' in: " + strconv.Itoa(timetaken) + "ms.")
				} else {
					fmt.Println("Unable to reach '" + ip + ":" + port + "'")
				}

			}
			os.Exit(int(SuccessNoError))
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
