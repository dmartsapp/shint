package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	payload    bool
	iterations int
	udp        *bool
)

func init() {
	fmt.Println("Initializing...")
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

}
