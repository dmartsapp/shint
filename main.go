package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dmartsapp/tnt/lib/handlers"
	"github.com/spf13/cobra"
)

var (
	Version string = "0.0.1"
)

var (
	iterations          int
	delay               int
	throttle            bool
	timeout             int
	payload_size        int
	jsonoutput          bool
	fromport            int
	endport             int
	httpmethod          string
	httpdata            string
	httpheaders         []string
	includeresponsebody bool
)

var rootCmd = &cobra.Command{
	Use:     filepath.Base(os.Args[0]),
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
	Use:     "web [url]",
	Short:   "Make an HTTP request to a URL",
	Long:    `This command makes an HTTP request to a URL and displays the response. Does not follow redirects or embedded resources.`,
	Args:    cobra.ExactArgs(1),
	Example: rootCmd.Name() + " web --json -H \"authorization:Bearer <token>\" -H \"content-type:application/json\" http://google.com --count 1",
	Run: func(cmd *cobra.Command, args []string) {
		URL, err := url.Parse(args[0])
		if err != nil {
			fmt.Println("Invalid URL")
			return
		}

		handlers.WebHandler(&jsonoutput, iterations, delay, &throttle, timeout, URL, httpmethod, httpdata, httpheaders, includeresponsebody)
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
	webCmd.Flags().StringVarP(&httpmethod, "method", "X", "GET", "HTTP method to use (GET, POST, PUT, DELETE)")
	webCmd.Flags().StringVarP(&httpdata, "payload", "P", "", "HTTP payload data to send")
	webCmd.Flags().StringArrayVarP(&httpheaders, "header", "H", []string{}, "HTTP headers to send (can be specified multiple times)")
	webCmd.Flags().BoolVarP(&includeresponsebody, "withbody", "W", false, "Include the response body in the JSON output")
	nmapCmd.Flags().IntVar(&fromport, "from", 1, "Start port for TCP scan")
	nmapCmd.Flags().IntVar(&endport, "to", 80, "End port for TCP scan")
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
	rootCmd.Version = Version
}

func main() {
	rootCmd.AddCommand(telnetCmd, pingCmd, webCmd, nmapCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
