package cmd

import (
	"github.com/farhansabbir/telnet/cmd/telnet"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(telnetCmd)
}

var telnetCmd = &cobra.Command{
	Use:   "telnet [host] [port]",
	Short: "Connect to a host on a specific port",
	Long:  `This command allows you to test connectivity to a host on a specific port using TCP.`,
	Run: func(cmd *cobra.Command, args []string) {
		telnet.Execute(cmd, args)
	},
}
