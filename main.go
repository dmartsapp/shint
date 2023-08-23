package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
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
		os.Exit(0)
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
		os.Exit(1)
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
		if regex.MatchString(flag.Args()[0]) {
			start := time.Now()
			ip = resolveName(flag.Args()[0]).String()
			end := time.Now()
			fmt.Println("Resolved '" + flag.Args()[0] + "' to '" + ip + "' in " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")

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

			url := scheme + "://" + flag.Args()[0] + ":" + flag.Args()[1] + getpath
			fmt.Println(url)
			if *download {
				fmt.Println("Placeholder for web request download")
				// this is for downloading entire payload; No summary
			}

		} else {
			// this is regular TCP telnet
			fmt.Println("Placeholder for regular TCP")
		}
	} else {
		// this is for UDP request
		fmt.Println("Placeholder for regular UDP")
	}

	// if len(flag.Args()) == 1 {
	// 	url, err := url.Parse(flag.Args()[0])
	// 	if err != nil {
	// 		fmt.Println(err.Error())
	// 		os.Exit(1)
	// 	}

	// 	port, _ := strconv.Atoi(url.Port())
	// 	if port == 0 {
	// 		switch url.Scheme {
	// 		case "http":
	// 			port = 80
	// 		case "https":
	// 			port = 443
	// 		default:
	// 			port = 443
	// 		}
	// 	}

	// 	resp, _ := http.Get(url.String())

	// 	fmt.Println(resp.StatusCode)

	// 	/*
	// 		placeholder for http or https check with or without payload download
	// 	*/
	// } else {
	// 	if !*udp {
	// 		addr := flag.Args()[0] + ":" + flag.Args()[1]
	// 		start := time.Now()
	// 		tcpaddr, err := net.ResolveTCPAddr("tcp", addr)
	// 		end := time.Now()
	// 		if err != nil {
	// 			fmt.Println(err.Error())
	// 			os.Exit(1)
	// 		}
	// 		fmt.Println("Resolved IP:Port of '" + addr + "' to " + tcpaddr.String() + " in " + strconv.Itoa(int(end.Sub(start).Milliseconds())) + "ms")
	// 		// var wg sync.WaitGroup
	// 		for i := 0; i < iterations; i++ {
	// 			// resp := dialNow("tcp", addr, timeout, &wg)
	// 			resp := dialNow("tcp", addr, timeout)
	// 			fmt.Println("TCP port checked successfully for '" + addr + "' in: " + strconv.Itoa(resp) + "ms")
	// 		}
	// 		// wg.Wait()

	// 	} else {
	// 		/*
	// 			Placeholder for UDP connection
	// 		*/
	// 		fmt.Println("UDP")
	// 	}
	// }

}

// func dialNow(protocol string, addressport string, timeout int, wg *sync.WaitGroup) int {
func dialNow(protocol string, addressport string, timeout int) int {
	start := time.Now()
	connect, err := net.DialTimeout(protocol, addressport, time.Duration(timeout)*time.Second)
	end := time.Now()
	if err != nil {
		fmt.Println(err)
		// wg.Done()
		os.Exit(1)
	}
	connect.Close()
	// wg.Done()
	return int((end.Sub(start)).Milliseconds())
}
