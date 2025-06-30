package cmd

import (
	"github.com/farhansabbir/telnet/cmd/ping"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(pingCmd)
}

var pingCmd = &cobra.Command{
	Use:   "ping [host]",
	Short: "Send ICMP ECHO_REQUEST to a host",
	Long:  `This command sends ICMP ECHO_REQUEST packets to a host to test reachability.`,
	Run: func(cmd *cobra.Command, args []string) {
		ping.Execute(cmd, args)
	},
}
