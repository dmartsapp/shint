package main

import (
	"flag"
	"fmt"
	"net"
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
	flag.IntVar(&iterations, "n", 1, "Number of times to check")
	flag.IntVar(&timeout, "t", 5, "Timeout in seconds to connect")
	flag.BoolVar(&payload, "payload", false, "Check if payload can be downloaded")
	udp = flag.Bool("udp", false, "Use UDP instead of tcp to connect to endpoint")
}

func main() {
	flag.Parse()
	if len(flag.Args()) < 2 {
		flag.PrintDefaults()
		os.Exit(0)
	}
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
