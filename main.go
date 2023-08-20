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
)

func init() {
	flag.IntVar(&iterations, "n", 1, "Number of times to check")
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
		tcp, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		start := time.Now()
		_, err = net.DialTCP("tcp", nil, tcp)
		end := time.Now()
		fmt.Printf("lo %v", strconv.Itoa(int(end.Sub(start).Milliseconds())))
	}

}
