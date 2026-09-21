// Command shint - "Simple Host INspection Toolkit" - bundles the everyday network
// checks (telnet-style port checks, ping, an HTTP client, a port scanner, a
// UDP probe, a clock check against an NTP server, Wake-on-LAN, reverse DNS,
// a subnet calculator, an interface lister, DNS lookups and local test
// listeners) into one static binary.
//
// This file is only the command line: it declares the cobra commands and
// their flags, validates arguments, and turns each handler's result into the
// process exit status. All the networking lives in lib/handlers, and the
// pieces they share in lib. See docs/src/tech-architecture.md.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
	"github.com/dmartsapp/shint/v4/lib/handlers"
	"github.com/spf13/cobra"
)

var (
	// Version is what `shint --version` prints. The default is the release
	// this source describes (bump it in the release commit); the Makefile,
	// the release workflow (tag/commit/time) and the Dockerfile each override
	// it at link time with -ldflags "-X main.Version=...", so every kind of
	// build reports something traceable. See docs/src/tech-release.md.
	Version string = "4.0.6"
)

// Flag values, bound in init(). cobra parses the command line into these
// package-level variables before a command's Run function is called.
var (
	// Shared flags: persistent on the root command, so every command has them
	// with the same name, default and meaning.
	iterations   int
	delay        int
	throttle     bool
	timeout      int
	payload_size int
	jsonoutput   bool
	ipv4Only     bool
	ipv6Only     bool

	// nmap
	fromport int
	endport  int

	// web
	httpmethod          string
	httpdata            string
	httpheaders         []string
	includeresponsebody bool
	cacertFile          string
	certFile            string
	keyFile             string
	insecureSkipVerify  bool
	webTiming           bool

	// udp
	udpData string
	udpHex  string

	// dns
	dnsTCP       bool
	dnsNoRecurse bool

	// wol
	wolBroadcast string
	wolPort      int

	// ntp
	ntpPort      int
	ntpMaxOffset int

	// listen. listenMaxCount shadows the root --count so that a listener
	// defaults to "run until Ctrl+C" rather than "one and done".
	listenBind     string
	listenEcho     bool
	listenMaxCount int
)

// Exit status. shint checks several things per run (every resolved address,
// every --count iteration), so it follows the convention of fping, its
// closest relative, rather than curl's "one request, one code":
//
//	0  every check passed
//	1  at least one check failed: connection refused or timed out, DNS
//	   failure, no HTTP response, a UDP port reported closed, lost pings,
//	   a run cut short by Ctrl+C (a scan, or a repeating check stopped before
//	   its --count was done), no usable time reply (or a clock offset beyond
//	   --max-offset), an address with no reverse (PTR) name, or a
//	   Wake-on-LAN packet that could not be sent, a DNS question that got no
//	   records of the type asked (NXDOMAIN, no data, SERVFAIL, REFUSED, no
//	   answer), an interface that does not exist (ip)
//	2  the command was used wrongly (bad argument, flag or value); nothing ran
//
// Results - including "ERROR" lines about failed checks - go to stdout; usage
// errors go to stderr, so `shint ... --json | jq` only ever sees JSON.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// exitCode is what main exits with once the chosen command has run.
var exitCode = exitOK

// applyFamily applies -4/-6 for a command that takes a host, and reports
// whether the run may go on. Asking for both, or naming an address of the other
// family (an IPv6 address with -4), is a usage error: nothing could be checked.
func applyFamily(host string) bool {
	if err := lib.SetIPFamily(ipv4Only, ipv6Only); err != nil {
		usage(err.Error())
		return false
	}
	if err := lib.HostFamilyConflict(host); err != nil {
		usage(err.Error())
		return false
	}
	return true
}

// checkRunFlags validates the shared flags a checking command uses and reports
// whether the run may go on: --count at least 1, --timeout 1 to a day of seconds
// and, for a command that pauses between attempts, --delay 0 to a day of
// milliseconds. Beyond a day a value is not a wait but an overflow of the duration
// it becomes, and used to turn into a check that failed at once.
func checkRunFlags(usesDelay bool) bool {
	checks := []error{lib.RequirePositive("count", iterations), lib.RequireTimeout(timeout)}
	if usesDelay {
		checks = append(checks, lib.RequireDelay(delay))
	}
	for _, err := range checks {
		if err != nil {
			usage(err.Error())
			return false
		}
	}
	return true
}

// usage reports a bad argument or flag value on stderr and marks the run as a
// usage error. Callers return right after it; nothing is checked.
func usage(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	exitCode = exitUsage
}

// finish records a handler's outcome: handlers report true when every check
// they ran passed.
func finish(ok bool) {
	if !ok && exitCode == exitOK {
		exitCode = exitFailure
	}
}

var rootCmd = &cobra.Command{
	Use:     filepath.Base(os.Args[0]),
	Short:   "shint - Simple Host INspection Toolkit",
	Long:    `A simple network utility tool that provides telnet, ping, nmap, udp, web client, ntp, wol, rdns, dns, cidr, ip and listener functionalities.`,
	Version: Version,
}

var telnetCmd = &cobra.Command{
	Use:   "telnet [host] [port]",
	Short: "Connect to a host on a specific port",
	Long:  `This command allows you to test connectivity to a host on a specific port using TCP.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host := args[0]
		if !applyFamily(host) {
			return
		}
		port, err := lib.ValidatePort(args[1])
		if err != nil {
			usage(err.Error())
			return
		}
		if !checkRunFlags(true) {
			return
		}

		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.TelnetHandler(ctx, &jsonoutput, iterations, delay, &throttle, timeout, payload_size, port, host))
	},
}

var pingCmd = &cobra.Command{
	Use:   "ping [host]",
	Short: "Send ICMP ECHO_REQUEST to a host",
	Long:  `This command sends ICMP ECHO_REQUEST packets to a host to test reachability.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !applyFamily(args[0]) {
			return
		}
		if !checkRunFlags(true) {
			return
		}
		if err := handlers.ValidatePingPayload(payload_size); err != nil {
			usage(err.Error())
			return
		}
		finish(handlers.HandleICMP(args[0], &jsonoutput, iterations, delay, &throttle, timeout, payload_size))
	},
}

var webCmd = &cobra.Command{
	Use:   "web [url]",
	Short: "Make an HTTP request to a URL",
	Long: `This command makes an HTTP request to a URL and displays the response. Follows redirects (up to 10, and bytes are counted across every hop) but does not fetch embedded resources.

A URL without a scheme (example.com, 127.0.0.1:8080/status) is fetched over https://; write http:// for a plain-HTTP server. Only http and https URLs are fetched.

It connects straight to the address the name resolves to and never uses a proxy: HTTP_PROXY, HTTPS_PROXY and NO_PROXY are ignored.`,
	Args: cobra.ExactArgs(1),
	Example: rootCmd.Name() + " web --json -H \"authorization:Bearer <token>\" -H \"content-type:application/json\" http://google.com --count 1" + "\n" +
		rootCmd.Name() + " web https://example.com --timing",
	Run: func(cmd *cobra.Command, args []string) {
		if !checkRunFlags(true) {
			return
		}

		URL, err := handlers.ParseWebURL(args[0])
		if err != nil {
			usage(err.Error())
			return
		}

		if !applyFamily(URL.Hostname()) {
			return
		}

		tlsConfig, err := handlers.BuildTLSConfig(cacertFile, certFile, keyFile, insecureSkipVerify)
		if err != nil {
			usage(err.Error())
			return
		}

		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.WebHandler(ctx, &jsonoutput, iterations, delay, &throttle, timeout, URL, httpmethod, httpdata, httpheaders, includeresponsebody, tlsConfig, webTiming))
	},
}

var nmapCmd = &cobra.Command{
	Use:   "nmap [host]",
	Short: "Scan for open TCP ports on a host",
	Long:  `This command scans for open TCP ports on a host within a given range. --timeout is how long each individual port is given to answer; the scan itself runs until the whole range has been covered (Ctrl+C stops it early and reports how far it got).`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !applyFamily(args[0]) {
			return
		}
		if !checkRunFlags(false) {
			return
		}
		if fromport < 1 || fromport > 65535 || endport < 1 || endport > 65535 {
			usage("--from and --to must both be between 1 and 65535")
			return
		}
		if fromport > endport {
			usage("--from must be less than or equal to --to")
			return
		}

		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.NmapHandler(ctx, args[0], fromport, endport, iterations, timeout, throttle, &jsonoutput))
	},
}

// interruptContext is the context a run happens under: cancelled by Ctrl+C or
// SIGTERM, and deliberately nothing else. It must not carry a deadline derived
// from --timeout - that flag bounds one operation (a port, a connection, a
// request), and bounding the whole run by it silently cut every run that
// outlasted it short after --timeout seconds: nmap stopped partway through its
// range, telnet reported false failures.
//
// On the first Ctrl+C the run winds down and reports how far it got (see
// lib/handlers/interrupt.go). The handler is then removed, so a second Ctrl+C
// ends the process at once if something is stuck.
func interruptContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx, stop
}

var udpCmd = &cobra.Command{
	Use:   "udp [host] [port]",
	Short: "Send a UDP probe to a host on a specific port",
	Long: `This command sends a UDP datagram to a host on a specific port and reports whether a reply, an ICMP port-unreachable, or nothing at all came back within the timeout.

The payload is one of three things: --data sends a message, as text exactly as typed; --hex sends exact bytes, written as hex digits (00010203ff, or 00 01 02 03 ff, or 00:01:02:03:ff), which is how to send a binary probe such as a DNS or STUN request; --payload sends that many bytes of filler. --hex and --data cannot be combined, and --payload is ignored when either is given. Backslash escapes such as \x00 are not interpreted in --data (the shell passes them through as ordinary characters): use --hex for bytes.`,
	Args: cobra.ExactArgs(2),
	Example: rootCmd.Name() + ` udp 127.0.0.1 9001 --data "hello"` + "\n" +
		rootCmd.Name() + ` udp 127.0.0.1 9001 --hex "00 01 02 ff"` + "\n" +
		rootCmd.Name() + ` udp 8.8.8.8 53 --payload 16 --timeout 3`,
	Run: func(cmd *cobra.Command, args []string) {
		host := args[0]
		if !applyFamily(host) {
			return
		}
		port, err := lib.ValidatePort(args[1])
		if err != nil {
			usage(err.Error())
			return
		}
		if !checkRunFlags(true) {
			return
		}
		data := udpData
		if cmd.Flags().Changed("hex") {
			if cmd.Flags().Changed("data") {
				usage("--hex and --data cannot be used together: give the payload one way")
				return
			}
			bytes, err := handlers.ParseHexPayload(udpHex)
			if err != nil {
				usage(err.Error())
				return
			}
			data = string(bytes)
		}
		if err := handlers.ValidateUDPPayload(payload_size, data); err != nil {
			usage(err.Error())
			return
		}

		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.UDPHandler(ctx, &jsonoutput, iterations, delay, &throttle, timeout, payload_size, data, port, host))
	},
}

var rdnsCmd = &cobra.Command{
	Use:   "rdns [host]",
	Short: "Look up the names an IP address maps back to (reverse DNS)",
	Long: `This command does a reverse DNS lookup: it asks which host names an IP address maps back to (its PTR records, found under in-addr.arpa for IPv4 and ip6.arpa for IPv6). Give it an IP address, or a host name - a name is resolved first and every address it resolves to is looked up.

An address that has no PTR record is reported as a failed check (exit status 1); many addresses have none. --timeout is how long each lookup gets. Uses the system's DNS servers, like the other commands.`,
	Args: cobra.ExactArgs(1),
	Example: rootCmd.Name() + ` rdns 8.8.8.8` + "\n" +
		rootCmd.Name() + ` rdns example.com --json`,
	Run: func(cmd *cobra.Command, args []string) {
		if !applyFamily(args[0]) {
			return
		}
		if !checkRunFlags(true) {
			return
		}
		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.RDNSHandler(ctx, &jsonoutput, iterations, delay, &throttle, timeout, args[0]))
	},
}

var dnsCmd = &cobra.Command{
	Use:   "dns [name] [type] [@server]",
	Short: "Look up DNS records (A, AAAA, MX, TXT, NS, ...) like dig",
	Long: `This command asks a DNS server a question and shows the answer: the exchange (which server answered, over UDP or TCP, the response code, the header flags, the counts and the time) and one line per record with its TTL. The types are A, AAAA, CNAME, MX, NS, TXT, SOA, SRV, PTR and CAA; without a type a name is asked for its A and AAAA records (only one of them with -4 or -6). An IP address is looked up in reverse (its PTR record), like dig -x.

Without @server the servers this machine is configured with are asked, in order, and the first that answers is used (a server that does not answer is skipped, and the line says so). @server is an address or a name, with an optional port (@1.1.1.1, @dns.google, @[::1]:5353), and can be an authoritative server: the "aa" flag and the SOA in the authority section show who is speaking. --no-recurse asks a server only for what it knows itself.

A question that gets no record of the type asked - the name does not exist (NXDOMAIN), it exists but has none of that type, the server fails (SERVFAIL) or refuses, or nothing answers - is a failed check (exit status 1), each with its reason. An answer counts only if it is for this question: replies with a different ID or question are ignored. --timeout is how long each server gets; --count repeats the question. Unprivileged: an ordinary UDP query, retried over TCP if the answer is truncated.`,
	Args: cobra.RangeArgs(1, 3),
	Example: rootCmd.Name() + ` dns example.com` + "\n" +
		rootCmd.Name() + ` dns example.com MX @1.1.1.1` + "\n" +
		rootCmd.Name() + ` dns 8.8.8.8` + "\n" +
		rootCmd.Name() + ` dns example.com SOA @a.iana-servers.net --no-recurse`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := lib.SetIPFamily(ipv4Only, ipv6Only); err != nil {
			usage(err.Error())
			return
		}
		query, err := handlers.ParseDNSArgs(args)
		if err != nil {
			usage(err.Error())
			return
		}
		if !checkRunFlags(true) {
			return
		}
		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.DNSHandler(ctx, &jsonoutput, iterations, delay, &throttle, timeout, query, handlers.DNSOptions{ForceTCP: dnsTCP, NoRecurse: dnsNoRecurse}))
	},
}

var cidrCmd = &cobra.Command{
	Use:   "cidr [prefix]...",
	Short: "Work out a subnet: network, mask, range and size",
	Long: `This command does subnet arithmetic on one or more IPv4 or IPv6 prefixes: the network address, netmask, wildcard mask, first and last address, broadcast address (IPv4), how many addresses and usable hosts it holds, and what kind of range it is (private, loopback, link-local, ...). A bare address is treated as a single host (/32 or /128).

It works entirely offline: nothing is looked up and nothing is sent. Only --json applies; the other shared flags are ignored.`,
	Args: cobra.MinimumNArgs(1),
	Example: rootCmd.Name() + ` cidr 192.168.1.10/24` + "\n" +
		rootCmd.Name() + ` cidr 10.0.0.0/8 2001:db8::/32 --json`,
	Run: func(cmd *cobra.Command, args []string) {
		subnets, err := handlers.ParseSubnets(args)
		if err != nil {
			usage(err.Error())
			return
		}
		finish(handlers.CIDRHandler(&jsonoutput, subnets))
	},
}

var ipCmd = &cobra.Command{
	Use:   "ip [interface]",
	Short: "List this machine's network interfaces and addresses",
	Long: `This command lists the network interfaces of this machine and what is on them: for each interface its state (up or down), flags, MTU and hardware (MAC) address, and every address with its prefix length and what kind of address it is (loopback, private, link-local, global, ...). Give an interface name to see just that one.

It reads the operating system's interface table and sends nothing, so it needs no network and no privileges. -4 and -6 list only the addresses of that family. An interface name that does not exist is a failed check (exit status 1) and names the ones that do. Only --json, -4 and -6 apply; the other shared flags are ignored.`,
	Args: cobra.MaximumNArgs(1),
	Example: rootCmd.Name() + ` ip` + "\n" +
		rootCmd.Name() + ` ip en0 --json` + "\n" +
		rootCmd.Name() + ` ip -4`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := lib.SetIPFamily(ipv4Only, ipv6Only); err != nil {
			usage(err.Error())
			return
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		finish(handlers.IPHandler(&jsonoutput, name))
	},
}

var wolCmd = &cobra.Command{
	Use:   "wol [mac]",
	Short: "Send a Wake-on-LAN magic packet to wake a machine on your network",
	Long: `This command broadcasts a Wake-on-LAN "magic packet" - six 0xFF bytes followed by the machine's MAC address sixteen times, 102 bytes in all - as a UDP datagram to the local network, which asks a sleeping or switched-off machine to power on.

The machine must have Wake-on-LAN enabled in its firmware and network card, and must be on the same network segment as this one. The packet has no reply, so a successful send only means it was handed to the network: shint cannot tell whether the machine woke. Needs no special privileges.`,
	Args: cobra.ExactArgs(1),
	Example: rootCmd.Name() + ` wol aa:bb:cc:dd:ee:ff` + "\n" +
		rootCmd.Name() + ` wol aa:bb:cc:dd:ee:ff --broadcast 192.168.1.255`,
	Run: func(cmd *cobra.Command, args []string) {
		if ipv6Only {
			usage("wol is IPv4 only (a magic packet is a broadcast, and IPv6 has none); -6/--ipv6 does not apply")
			return
		}
		mac, err := handlers.ParseMAC(args[0])
		if err != nil {
			usage(err.Error())
			return
		}
		broadcast, err := handlers.ParseBroadcast(wolBroadcast)
		if err != nil {
			usage(err.Error())
			return
		}
		if wolPort < 1 || wolPort > 65535 {
			usage("--port must be between 1 and 65535")
			return
		}
		if !checkRunFlags(true) {
			return
		}
		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.WOLHandler(ctx, &jsonoutput, iterations, delay, &throttle, timeout, mac, broadcast, wolPort))
	},
}

var ntpCmd = &cobra.Command{
	Use:   "ntp [server]",
	Short: "Check this machine's clock against an NTP time server",
	Long: `This command asks an NTP server for the time (a single SNTP query over UDP port 123) and reports how far this machine's clock is from it: the offset (server time minus local time, so a positive offset means this clock is behind), the network round-trip time, and the server's stratum and reference. If the name resolves to several addresses, each one is asked. --timeout is how long each server gets to answer.

A reply that cannot be trusted - it answers a different request, the server says its own clock is unsynchronized, or it sends a "kiss-o'-death" - counts as a failed query. --max-offset turns the offset into a check: the command exits 1 if the clock is further out than that.`,
	Args: cobra.ExactArgs(1),
	Example: rootCmd.Name() + ` ntp pool.ntp.org` + "\n" +
		rootCmd.Name() + ` ntp time.cloudflare.com --max-offset 500`,
	Run: func(cmd *cobra.Command, args []string) {
		if !applyFamily(args[0]) {
			return
		}
		if ntpPort < 1 || ntpPort > 65535 {
			usage("--port must be between 1 and 65535")
			return
		}
		if ntpMaxOffset < 0 {
			usage("--max-offset must be 0 (only report) or a number of milliseconds")
			return
		}
		if !checkRunFlags(true) {
			return
		}
		ctx, stop := interruptContext()
		defer stop()

		finish(handlers.NTPHandler(ctx, &jsonoutput, iterations, delay, &throttle, timeout, ntpPort, time.Duration(ntpMaxOffset)*time.Millisecond, args[0]))
	},
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Run:   unknownSubcommand,
	Short: "Start a local HTTP or UDP listener for testing",
	Long:  `The listen command starts a simple HTTP or UDP listener on this machine so that web, udp, telnet, and nmap can be exercised end-to-end without needing an external server. Use --count to bound how many requests/packets are accepted (0 = run until Ctrl+C).`,
}

// removedListeners are the listen subcommands that no longer exist, with what to use
// instead, so a script that still calls one is told rather than only "unknown command".
var removedListeners = map[string]string{
	"tcp": `"listen tcp" was removed in v4.2.0: use "shint listen http <port>" (a TCP connection to it works too) or "shint listen udp <port>"`,
}

// unknownSubcommand is the Run of a command that only groups others (listen,
// completion): cobra shows the help for an argument it does not know, and exits 0,
// as if nothing was wrong. Using a command wrongly is exit status 2 with the
// reason on stderr; asking for the group itself just shows its help.
func unknownSubcommand(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		msg := fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())
		if hint, ok := removedListeners[args[0]]; ok && cmd.Name() == "listen" {
			msg += ": " + hint
		}
		usage(msg)
		return
	}
	_ = cmd.Help()
}

// checkListenFlags validates what the listen commands share: --count 0 (until
// Ctrl+C) or more, and --timeout as an idle timeout of 0 (none) to a day.
func checkListenFlags() bool {
	for _, err := range []error{lib.RequireNonNegative("count", listenMaxCount), lib.RequireIdleTimeout(timeout)} {
		if err != nil {
			usage(err.Error())
			return false
		}
	}
	return true
}

var listenUDPCmd = &cobra.Command{
	Use:   "udp [port]",
	Short: "Start a UDP listener on the given port",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		port, err := lib.ValidatePort(args[0])
		if err != nil {
			usage(err.Error())
			return
		}
		if !checkListenFlags() {
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
			usage(err.Error())
			return
		}
		if !checkListenFlags() {
			return
		}
		handlers.HTTPListenHandler(listenBind, port, listenMaxCount, timeout, &jsonoutput)
	},
}

func init() {
	rootCmd.PersistentFlags().IntVar(&iterations, "count", 1, "Number of times to check connectivity (listen commands: max requests/packets to accept, 0 = unlimited)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 5, "Timeout in seconds to connect (listen http: idle timeout on a connection, 0 = no timeout; no effect on listen udp)")
	rootCmd.PersistentFlags().IntVar(&delay, "delay", 1000, "Milliseconds delay between each iteration given in count")
	rootCmd.PersistentFlags().IntVar(&payload_size, "payload", 4, "Ping/UDP payload size in bytes (filler content, ignored if --data or --hex is set on udp)")
	rootCmd.PersistentFlags().BoolVar(&throttle, "throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request.")
	rootCmd.PersistentFlags().BoolVar(&jsonoutput, "json", false, "Flag option to output only in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&ipv4Only, "ipv4", "4", false, "Resolve and check IPv4 addresses only (a host with both kinds is normally checked over both)")
	rootCmd.PersistentFlags().BoolVarP(&ipv6Only, "ipv6", "6", false, "Resolve and check IPv6 addresses only")

	webCmd.Flags().StringVarP(&httpmethod, "method", "X", "GET", "HTTP method to use (GET, POST, PUT, DELETE)")
	webCmd.Flags().StringVarP(&httpdata, "payload", "P", "", "HTTP payload data to send")
	webCmd.Flags().StringArrayVarP(&httpheaders, "header", "H", []string{}, "HTTP headers to send (can be specified multiple times)")
	webCmd.Flags().BoolVarP(&includeresponsebody, "withbody", "W", false, "Include the response body in the JSON output")
	webCmd.Flags().StringVar(&cacertFile, "cacert", "", "Path to a PEM CA certificate bundle to trust in addition to the system pool (for self-signed/internal CAs)")
	webCmd.Flags().StringVar(&certFile, "cert", "", "Path to a PEM client certificate for mutual TLS (requires --key)")
	webCmd.Flags().StringVar(&keyFile, "key", "", "Path to the PEM private key matching --cert (requires --cert)")
	webCmd.Flags().BoolVarP(&insecureSkipVerify, "insecure", "k", false, "Skip TLS certificate verification (INSECURE, for diagnostics only)")
	webCmd.Flags().BoolVar(&webTiming, "timing", false, "Show where each request's time went: DNS, connect, TLS, wait for the first byte, download (one line per redirect hop; a timing object per stat with --json)")

	nmapCmd.Flags().IntVar(&fromport, "from", 1, "Start port for TCP scan")
	nmapCmd.Flags().IntVar(&endport, "to", 80, "End port for TCP scan")

	udpCmd.Flags().StringVarP(&udpData, "data", "D", "", "Explicit payload data to send instead of the generated --payload filler")
	udpCmd.Flags().StringVar(&udpHex, "hex", "", "Exact payload bytes as hex digits, e.g. 00010203ff or \"00 01 02 03 ff\" (spaces and colons are ignored; not with --data)")

	dnsCmd.Flags().BoolVar(&dnsTCP, "tcp", false, "Ask over TCP only (by default UDP, with a TCP retry when the answer is truncated)")
	dnsCmd.Flags().BoolVar(&dnsNoRecurse, "no-recurse", false, "Clear the \"recursion desired\" bit: ask the server only for what it knows itself (use with an authoritative @server)")

	wolCmd.Flags().StringVar(&wolBroadcast, "broadcast", handlers.WOLDefaultBroadcast, "IPv4 broadcast address to send the magic packet to (for one subnet, e.g. 192.168.1.255)")
	wolCmd.Flags().IntVar(&wolPort, "port", handlers.WOLDefaultPort, "UDP port to send the magic packet to")

	ntpCmd.Flags().IntVar(&ntpPort, "port", handlers.NTPDefaultPort, "UDP port of the NTP server")
	ntpCmd.Flags().IntVar(&ntpMaxOffset, "max-offset", 0, "Exit 1 if the clock offset is larger than this many milliseconds (0 = only report it)")

	listenCmd.PersistentFlags().StringVar(&listenBind, "bind", "0.0.0.0", "Local address to bind the listener to")
	listenUDPCmd.Flags().BoolVar(&listenEcho, "echo", false, "Echo received datagrams back to the sender")
	// Shadows the root --count flag (default 1) for every listen subcommand:
	// a listener's whole point is usually to stay up until the user is done
	// with it, so "keep listening until Ctrl+C" is the sensible default here,
	// unlike the "one check and done" default that fits telnet/ping/web/nmap/udp.
	listenCmd.PersistentFlags().IntVar(&listenMaxCount, "count", 0, "Max requests (http) or packets (udp) to accept, 0 = unlimited (run until Ctrl+C)")
	listenCmd.AddCommand(listenUDPCmd, listenHTTPCmd)

	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
	rootCmd.Version = Version
}

// main registers the commands (here rather than in init(), so the command
// variables above are fully initialised first), runs cobra, and exits with the
// status the chosen command recorded - see the exit-status notes above.
func main() {
	rootCmd.AddCommand(telnetCmd, pingCmd, webCmd, nmapCmd, udpCmd, ntpCmd, wolCmd, rdnsCmd, dnsCmd, cidrCmd, ipCmd, listenCmd)
	// cobra adds its completion command only when it executes. Adding it here - after
	// the commands above, since cobra skips it while the root has none - lets it treat
	// an argument it does not know as a mistake, like listen.
	rootCmd.InitDefaultCompletionCmd()
	for _, c := range rootCmd.Commands() {
		if c.Name() == "completion" {
			c.Run = unknownSubcommand
		}
	}
	// cobra has already printed the error (and usage help) to stderr. Every
	// error Execute returns is a usage error - the Run functions never return
	// one; they report through usage() and finish() instead.
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitUsage)
	}
	os.Exit(exitCode)
}
