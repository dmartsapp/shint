package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	iterations int
	delay      int
	throttle   *bool
	timeout    int
	web        *bool
	nmap       *bool
	fromport   int
	endport    int
)

const (
	SuccessNoError uint8  = 0
	HTTP_CLIENT    string = "dmarts.app-http-v0.1"
)

func init() {
	flag.IntVar(&iterations, "count", 1, "Number of times to check connectivity")
	flag.IntVar(&timeout, "timeout", 5, "Timeout in seconds to connect")
	flag.IntVar(&delay, "delay", 0, "Seconds delay between each iteration given in count")
	web = flag.Bool("web", false, "Use web request as a web client.")
	throttle = flag.Bool("throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request.")
	nmap = flag.Bool("nmap", false, "Flag option to run tcp port scan. This flag ignores all other parameters except -from and -to, if mentioned.")
	flag.IntVar(&fromport, "from", 1, "Start port to begin TCP scan from.")
	flag.IntVar(&endport, "to", 80, "End port to run TCP scan to.")

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

}
