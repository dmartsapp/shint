//go:build !aix

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dmartsapp/shint/v4/lib"
	"github.com/dmartsapp/shint/v4/lib/handlers/ss"
)

// ss is left out of the AIX binary: AIX offers no way to list its sockets
// with the process holding each without root (its own netstat -A and rmsock,
// and lsof, read kernel memory as root), and shint never requires root. On
// other systems it is not written yet, and the command says so (#105).

// ss: -t, -u, -l, --ports (a port list read by lib.ParsePorts), --pid
var (
	ssTCP       bool
	ssUDP       bool
	ssListening bool
	ssPorts     string
	ssPID       int
)

var ssCmd = &cobra.Command{
	Use:   "ss",
	Short: "List this machine's TCP and UDP sockets and the process holding each",
	Long: `This command lists this machine's own TCP and UDP sockets, IPv4 and IPv6, one line (or one JSON entry) per socket with everything known about it: the protocol, the state (LISTEN, ESTABLISHED, TIME-WAIT, ...; UNCONN for a UDP socket with no peer), the local and remote address and port, the bytes queued, the user it belongs to, and the process holding it - its name, PID and file descriptor number.

It reads the operating system's socket tables and sends nothing. It needs no privileges: without them, a socket of another user's process is still listed, but not which process holds it - the line says owner=hidden, and a WARN line counts them. Run it as root to see every process. It works on Linux and Windows; on other systems it says it is not supported yet (exit status 1), and the AIX binary has no ss command at all: AIX has no way to read its sockets and their processes without root.

-t and -u list only TCP or only UDP sockets, -l only listening ones, --ports only those with one of these ports at either end, --pid only those a process holds. When nothing matches, that is a failed check (exit status 1), so "ss -l --ports 5432" tells a script whether anything listens on port 5432. Only --json, -4 and -6 of the shared flags apply.`,
	Args: cobra.NoArgs,
	Example: rootCmd.Name() + ` ss -l` + "\n" +
		rootCmd.Name() + ` ss -t --ports 22,443` + "\n" +
		rootCmd.Name() + ` ss -l --ports 5432 --json`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := lib.SetIPFamily(ipv4Only, ipv6Only); err != nil {
			usage(err.Error())
			return
		}
		query := ss.Query{TCP: ssTCP, UDP: ssUDP, Listening: ssListening}
		if cmd.Flags().Changed("ports") {
			ports, err := lib.ParsePorts("ports", ssPorts)
			if err != nil {
				usage(err.Error())
				return
			}
			query.Ports, query.PortsText = ports, lib.PortsText(ports)
		}
		if cmd.Flags().Changed("pid") {
			if ssPID < 1 {
				usage(fmt.Sprintf("--pid %d is not a process ID: give a number of 1 or more", ssPID))
				return
			}
			query.PID = ssPID
		}
		finish(ss.Run(&jsonoutput, query))
	},
}

func init() {
	ssCmd.Flags().BoolVarP(&ssTCP, "tcp", "t", false, "List only TCP sockets")
	ssCmd.Flags().BoolVarP(&ssUDP, "udp", "u", false, "List only UDP sockets")
	ssCmd.Flags().BoolVarP(&ssListening, "listening", "l", false, "List only listening sockets: TCP in LISTEN, UDP with no peer")
	ssCmd.Flags().StringVar(&ssPorts, "ports", "", "List only sockets with one of these ports at either end: 22,80,443 and ranges like 8000-8100")
	ssCmd.Flags().IntVar(&ssPID, "pid", 0, "List only the sockets this process holds")
	platformCommands = append(platformCommands, ssCmd)
}
