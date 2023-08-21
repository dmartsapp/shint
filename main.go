package main

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

var (
	payload    bool
	iterations int
	udp        *bool
	timeout    int
)

func init() {
	flag.IntVar(&iterations, "iterations", 1, "Number of times to check")
	flag.IntVar(&timeout, "timeout", 5, "Timeout in seconds to connect")
	flag.BoolVar(&payload, "download", false, "Check if payload can be downloaded")
	udp = flag.Bool("udp", false, "Use UDP instead of tcp to connect to endpoint")
}

func main() {

	flag.Parse()
	if len(flag.Args()) < 1 {
		flag.PrintDefaults()
		os.Exit(0)
	}
	if len(flag.Args()) == 1 {
		url, err := url.Parse(flag.Args()[0])
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		port, _ := strconv.Atoi(url.Port())
		if port == 0 {
			switch url.Scheme {
			case "http":
				port = 80
			case "https":
				port = 443
			default:
				port = 443
			}
		}
		fmt.Println(port)
		fmt.Println(url)
	} else {
		if !*udp {
			addr := flag.Args()[0] + ":" + flag.Args()[1]

			start := time.Now()
			_, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
			end := time.Now()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Printf("%v", strconv.Itoa(int(end.Sub(start).Milliseconds())))
		}
	}

}
