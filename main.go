package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/farhansabbir/telnet/lib/handlers"
	"github.com/spf13/cobra"
)

var (
	Version string
)

var (
	iterations   int
	delay        int
	throttle     bool
	timeout      int
	payload_size int
	jsonoutput   bool
	fromport     int
	endport      int
)

var rootCmd = &cobra.Command{
	Use:     "telnet",
	Short:   "A simple network utility tool",
	Long:    `A simple network utility tool that provides telnet, ping, nmap, and web client functionalities.`,
	Version: Version,
}

var telnetCmd = &cobra.Command{
	Use:   "telnet [host] [port]",
	Short: "Connect to a host on a specific port",
	Long:  `This command allows you to test connectivity to a host on a specific port using TCP.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host := args[0]
		port, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Invalid port number")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		handlers.TelnetHandler(&jsonoutput, iterations, delay, &throttle, timeout, payload_size, port, ctx, host)
	},
}

var pingCmd = &cobra.Command{
	Use:   "ping [host]",
	Short: "Send ICMP ECHO_REQUEST to a host",
	Long:  `This command sends ICMP ECHO_REQUEST packets to a host to test reachability.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handlers.HandleICMP(args[0], &jsonoutput, iterations, delay, &throttle, timeout, payload_size)
	},
}

var webCmd = &cobra.Command{
	Use:   "web [url]",
	Short: "Make an HTTP GET request to a URL",
	Long:  `This command makes an HTTP GET request to a URL and displays the response.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		URL, err := url.Parse(args[0])
		if err != nil {
			fmt.Println("Invalid URL")
			return
		}

		handlers.WebHandler(&jsonoutput, iterations, delay, &throttle, timeout, URL)
	},
}

var nmapCmd = &cobra.Command{
	Use:   "nmap [host]",
	Short: "Scan for open TCP ports on a host",
	Long:  `This command scans for open TCP ports on a host within a given range.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		handlers.NmapHandler(ctx, args[0], fromport, endport, iterations, timeout, throttle, &jsonoutput)
	},
}

func init() {
	rootCmd.PersistentFlags().IntVar(&iterations, "count", 1, "Number of times to check connectivity")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 5, "Timeout in seconds to connect")
	rootCmd.PersistentFlags().IntVar(&delay, "delay", 1000, "Seconds delay between each iteration given in count")
	rootCmd.PersistentFlags().IntVar(&payload_size, "payload", 4, "Ping payload size in bytes")
	rootCmd.PersistentFlags().BoolVar(&throttle, "throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request.")
	rootCmd.PersistentFlags().BoolVar(&jsonoutput, "json", false, "Flag option to output only in JSON format")
	nmapCmd.Flags().IntVar(&fromport, "from", 1, "Start port for TCP scan")
	nmapCmd.Flags().IntVar(&endport, "to", 80, "End port for TCP scan")
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
}

func main() {
	rootCmd.AddCommand(telnetCmd, pingCmd, webCmd, nmapCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}