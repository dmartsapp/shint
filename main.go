package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/dmartsapp/shint/lib"
	"github.com/dmartsapp/shint/lib/handlers"
	"github.com/spf13/cobra"
)

var (
	// Version is overridden at build time via -ldflags "-X main.Version=...".
	Version string = "4.0.1"
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
	cacertFile          string
	certFile            string
	keyFile             string
	insecureSkipVerify  bool
	udpData             string
	listenBind          string
	listenEcho          bool
	listenMaxCount      int
)

var rootCmd = &cobra.Command{
	Use:     filepath.Base(os.Args[0]),
	Short:   "SHINT - that SHIt Network Tool",
	Long:    `A simple network utility tool that provides telnet, ping, nmap, udp, web client, and listener functionalities.`,
	Version: Version,
}

var telnetCmd = &cobra.Command{
	Use:   "telnet [host] [port]",
	Short: "Connect to a host on a specific port",
	Long:  `This command allows you to test connectivity to a host on a specific port using TCP.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host := args[0]
		port, err := lib.ValidatePort(args[1])
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := lib.RequirePositive("count", iterations); err != nil {
			fmt.Println(err)
			return
		}
		if err := lib.RequirePositive("timeout", timeout); err != nil {
			fmt.Println(err)
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
		if err := lib.RequirePositive("count", iterations); err != nil {
			fmt.Println(err)
			return
		}
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
		if err := lib.RequirePositive("count", iterations); err != nil {
			fmt.Println(err)
			return
		}
		if err := lib.RequirePositive("timeout", timeout); err != nil {
			fmt.Println(err)
			return
		}

		URL, err := url.Parse(args[0])
		if err != nil {
			fmt.Println("Invalid URL")
			return
		}
		if URL.Scheme == "" {
			URL, err = url.Parse("https://" + args[0])
			if err != nil {
				fmt.Println("Invalid URL")
				return
			}
		}

		tlsConfig, err := handlers.BuildTLSConfig(cacertFile, certFile, keyFile, insecureSkipVerify)
		if err != nil {
			fmt.Println(err)
			return
		}

		handlers.WebHandler(&jsonoutput, iterations, delay, &throttle, timeout, URL, httpmethod, httpdata, httpheaders, includeresponsebody, tlsConfig)
	},
}

var nmapCmd = &cobra.Command{
	Use:   "nmap [host]",
	Short: "Scan for open TCP ports on a host",
	Long:  `This command scans for open TCP ports on a host within a given range.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := lib.RequirePositive("count", iterations); err != nil {
			fmt.Println(err)
			return
		}
		if err := lib.RequirePositive("timeout", timeout); err != nil {
			fmt.Println(err)
			return
		}
		if fromport < 1 || fromport > 65535 || endport < 1 || endport > 65535 {
			fmt.Println("--from and --to must both be between 1 and 65535")
			return
		}
		if fromport > endport {
			fmt.Println("--from must be less than or equal to --to")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		handlers.NmapHandler(ctx, args[0], fromport, endport, iterations, timeout, throttle, &jsonoutput)
	},
}

var udpCmd = &cobra.Command{
	Use:   "udp [host] [port]",
	Short: "Send a UDP probe to a host on a specific port",
	Long:  `This command sends a UDP datagram to a host on a specific port and reports whether a reply, an ICMP port-unreachable, or nothing at all came back within the timeout.`,
	Args:  cobra.ExactArgs(2),
	Example: rootCmd.Name() + " udp 8.8.8.8 53 --data \"\\x00\\x00\"",
	Run: func(cmd *cobra.Command, args []string) {
		host := args[0]
		port, err := lib.ValidatePort(args[1])
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := lib.RequirePositive("count", iterations); err != nil {
			fmt.Println(err)
			return
		}
		if err := lib.RequirePositive("timeout", timeout); err != nil {
			fmt.Println(err)
			return
		}

		handlers.UDPHandler(&jsonoutput, iterations, delay, &throttle, timeout, payload_size, udpData, port, host)
	},
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Start a local TCP, UDP, or HTTP listener for testing",
	Long:  `The listen command starts a simple TCP, UDP, or HTTP listener on this machine so that telnet, udp, web, and nmap can be exercised end-to-end without needing an external server. Use --count to bound how many connections/packets/requests are accepted (0 = run until Ctrl+C).`,
}

var listenTCPCmd = &cobra.Command{
	Use:   "tcp [port]",
	Short: "Start a TCP listener on the given port",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		port, err := lib.ValidatePort(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		handlers.TCPListenHandler(listenBind, port, listenEcho, listenMaxCount, timeout, &jsonoutput)
	},
}

var listenUDPCmd = &cobra.Command{
	Use:   "udp [port]",
	Short: "Start a UDP listener on the given port",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		port, err := lib.ValidatePort(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		handlers.UDPListenHandler(listenBind, port, listenEcho, listenMaxCount, timeout, &jsonoutput)
	},
}

var listenHTTPCmd = &cobra.Command{
	Use:   "http [port]",
	Short: "Start a minimal JSON HTTP listener on the given port",
	Long:  `Starts a basic HTTP server useful for both plain TCP and HTTP reachability checks: any method on "/" returns a small {"status":"ok"} JSON body, and every other path returns 404 with {"status":"not found"}.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		port, err := lib.ValidatePort(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		handlers.HTTPListenHandler(listenBind, port, listenMaxCount, timeout, &jsonoutput)
	},
}

func init() {
	rootCmd.PersistentFlags().IntVar(&iterations, "count", 1, "Number of times to check connectivity (listen commands: max connections/packets to accept, 0 = unlimited)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 5, "Timeout in seconds to connect (listen commands: idle read timeout, 0 = no timeout)")
	rootCmd.PersistentFlags().IntVar(&delay, "delay", 1000, "Milliseconds delay between each iteration given in count")
	rootCmd.PersistentFlags().IntVar(&payload_size, "payload", 4, "Ping/UDP payload size in bytes (filler content, ignored if --data is set on udp)")
	rootCmd.PersistentFlags().BoolVar(&throttle, "throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request.")
	rootCmd.PersistentFlags().BoolVar(&jsonoutput, "json", false, "Flag option to output only in JSON format")

	webCmd.Flags().StringVarP(&httpmethod, "method", "X", "GET", "HTTP method to use (GET, POST, PUT, DELETE)")
	webCmd.Flags().StringVarP(&httpdata, "payload", "P", "", "HTTP payload data to send")
	webCmd.Flags().StringArrayVarP(&httpheaders, "header", "H", []string{}, "HTTP headers to send (can be specified multiple times)")
	webCmd.Flags().BoolVarP(&includeresponsebody, "withbody", "W", false, "Include the response body in the JSON output")
	webCmd.Flags().StringVar(&cacertFile, "cacert", "", "Path to a PEM CA certificate bundle to trust in addition to the system pool (for self-signed/internal CAs)")
	webCmd.Flags().StringVar(&certFile, "cert", "", "Path to a PEM client certificate for mutual TLS (requires --key)")
	webCmd.Flags().StringVar(&keyFile, "key", "", "Path to the PEM private key matching --cert (requires --cert)")
	webCmd.Flags().BoolVarP(&insecureSkipVerify, "insecure", "k", false, "Skip TLS certificate verification (INSECURE, for diagnostics only)")

	nmapCmd.Flags().IntVar(&fromport, "from", 1, "Start port for TCP scan")
	nmapCmd.Flags().IntVar(&endport, "to", 80, "End port for TCP scan")

	udpCmd.Flags().StringVarP(&udpData, "data", "D", "", "Explicit payload data to send instead of the generated --payload filler")

	listenCmd.PersistentFlags().StringVar(&listenBind, "bind", "0.0.0.0", "Local address to bind the listener to")
	listenCmd.PersistentFlags().BoolVar(&listenEcho, "echo", false, "Echo received data back to the sender")
	// Shadows the root --count flag (default 1) for every listen subcommand:
	// a listener's whole point is usually to stay up until the user is done
	// with it, so "keep listening until Ctrl+C" is the sensible default here,
	// unlike the "one check and done" default that fits telnet/ping/web/nmap/udp.
	listenCmd.PersistentFlags().IntVar(&listenMaxCount, "count", 0, "Max connections/packets/requests to accept, 0 = unlimited (run until Ctrl+C)")
	listenCmd.AddCommand(listenTCPCmd, listenUDPCmd, listenHTTPCmd)

	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
	rootCmd.Version = Version
}

func main() {
	rootCmd.AddCommand(telnetCmd, pingCmd, webCmd, nmapCmd, udpCmd, listenCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
