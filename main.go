package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/farhansabbir/telnet/lib"
)

var (
	iterations int = 1
	delay      int
	throttle   *bool
	timeout    int = 5
	web        *bool
	nmap       *bool
	fromport   int = 1
	endport    int = 80
)

const (
	SuccessNoError uint8  = 0
	HTTP_CLIENT    string = "dmarts.app-http-v0.1"
)

func init() {
	flag.IntVar(&iterations, "count", iterations, "Number of times to check connectivity")
	flag.IntVar(&timeout, "timeout", timeout, "Timeout in seconds to connect")
	flag.IntVar(&delay, "delay", delay, "Seconds delay between each iteration given in count")
	web = flag.Bool("web", false, "Use web request as a web client.")
	throttle = flag.Bool("throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request. This is useful for networks/systems with AV or IDS")
	nmap = flag.Bool("nmap", false, "Flag option to run tcp port scan. This flag ignores all other parameters except -from and -to, if mentioned.")
	flag.IntVar(&fromport, "from", fromport, "Start port to begin TCP scan from.")
	flag.IntVar(&endport, "to", endport, "End port to run TCP scan to.")

	flag.Usage = func() {
		fmt.Println("Usage: " + os.Args[0] + " [options] <fqdn|IP> port")
		fmt.Println("options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example (fqdn): " + os.Args[0] + " google.com 443")
		fmt.Println("Example (IP): " + os.Args[0] + " 10.10.10.10 443")
		fmt.Println("Example (fqdn with -web flag to send 'https' request to path '/pages/index.html' as client with user-agent set as '" + HTTP_CLIENT + "'): " + os.Args[0] + " -web https://google.com/pages/index.html")
		os.Exit(int(SuccessNoError))
	}
}

func main() {
	flag.Parse()         // read the flags passed for processing
	if !*web || !*nmap { // nmap and web needs single param like -nmap 10.10.18.121 or "-web https://google.com" respectively, while telnet needs two parameters like 10.10.18.121 22 for IP and Port respectively
		if len(flag.Args()) != 2 { // telnet only needs 2 params, so show usage and exit for additional parameters
			flag.Usage()
			os.Exit(int(SuccessNoError))
		}
	}
	// setting up timeout context to ensure we exit after defined timeout
	CTXTIMEOUT, CANCEL := context.WithTimeout(context.Background(), time.Duration(time.Second*time.Duration(timeout)))
	defer CANCEL()

	if *nmap {

	} else if *web {

	} else { // this should be ideally telnet if not web or nmap
		port, err := strconv.ParseUint(flag.Arg(1), 10, 64)
		if err != nil {
			fmt.Println(lib.LogWithTimestamp(err.Error(), true))
			os.Exit(1)
		}
		start := time.Now()
		ipaddresses, err := lib.ResolveName(CTXTIMEOUT, flag.Arg(0))
		if err != nil {
			fmt.Printf("%s ", lib.LogWithTimestamp(err.Error(), true))
		} else {
			fmt.Println(lib.LogWithTimestamp("DNS lookup successful for "+flag.Arg(0)+"' to "+strconv.Itoa(len(ipaddresses))+" addresses '["+strings.Join(ipaddresses[:], ", ")+"]' in "+time.Since(start).String(), false))
			var stats = make([]time.Duration, 0)
			for i := 0; i < iterations; i++ { // loop over the ip addresses for the iterations required
				for _, ip := range ipaddresses { //  we need to loop over all ip addresses returned, even for once
					var dialer net.Dialer
					start = time.Now()
					conn, err := dialer.DialContext(CTXTIMEOUT, lib.Protocol, ip+":"+strconv.Itoa(int(port)))
					if err != nil {
						if strings.Contains(err.Error(), "i/o timeout") {
							fmt.Println(lib.LogWithTimestamp("Timeout connecting to "+ip+" on port "+strconv.Itoa(int(port))+" after "+time.Since(start).String(), true))
						} else {
							fmt.Println(lib.LogWithTimestamp(err.Error(), true))
						}
						stats = append(stats, time.Since(start))
						continue
					} else {
						stats = append(stats, time.Since(start))
						fmt.Println(lib.LogWithTimestamp("Successfully connected to "+ip+" on port "+strconv.Itoa(int(port))+" after "+time.Since(start).String(), false))
					}
					conn.Close()
				}
			}
			fmt.Println(lib.LogStats(stats, iterations))
		}

	}

}
