package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
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
	SuccessNoError  uint8 = 0
	NoSuchHostError uint8 = 2
	TimeoutError    uint8 = 3
	HttpGetError    uint8 = 4
	UnknownError    uint8 = 1
)

func init() {
	flag.IntVar(&iterations, "iterations", 1, "Number of times to check")
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

	regex, _ := regexp.Compile("[a-z|A-Z]")

	if !*udp {
		ip := flag.Args()[0]
		port := flag.Args()[1]
		if regex.MatchString(flag.Args()[0]) {
			start := time.Now()
			ip = resolveName(flag.Args()[0]).String()
			end := time.Now()
			fmt.Println("Successfully resolved '" + flag.Args()[0] + "' to '" + ip + "' in " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")

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

			url := scheme + "://" + ip + ":" + port + getpath
			fmt.Println(url)
			if *download {
				fmt.Println("Placeholder for web request download")
				// this is for downloading entire payload; No summary

				return
			} else {
				fmt.Println("Placeholder for summary of web request")
				httpClient := &http.Client{}
				if !*httpOnly {
					httpsTransport := &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					}
					httpClient = &http.Client{Transport: httpsTransport}
				}
				start := time.Now()
				resp, err := httpClient.Get(url)

				if err != nil {
					fmt.Println(err.Error())
					os.Exit(int(HttpGetError))
				}

				payload, _ := ioutil.ReadAll(resp.Body)

				end := time.Now()
				fmt.Println(resp.Status)
				fmt.Printf("%v bytes\n", len(payload))

				fmt.Println((end.Sub(start)).Milliseconds())
				resp.Body.Close()
			}

		} else {

			// this is regular TCP telnet
			for i := 0; i < iterations; i++ {
				timetaken := dialNow("tcp", ip+":"+port, timeout)
				fmt.Println("Successfully reached '" + ip + ":" + port + "' in " + strconv.Itoa(timetaken) + "ms.")
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
			fmt.Println("Unreachable port. Timeout after " + strconv.Itoa(timeout) + " seconds")
			os.Exit(int(TimeoutError))
		}
		// wg.Done()
		fmt.Println(err.Error())
		os.Exit(int(UnknownError))
	}
	connect.Close()

	// wg.Done()
	return int((end.Sub(start)).Milliseconds())
}
