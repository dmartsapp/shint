package cmd

import (
	"github.com/farhansabbir/telnet/cmd/nmap"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(nmapCmd)
	nmapCmd.Flags().Int("from", 1, "Start port for TCP scan")
	nmapCmd.Flags().Int("to", 80, "End port for TCP scan")
}

var nmapCmd = &cobra.Command{
	Use:   "nmap [host]",
	Short: "Scan for open TCP ports on a host",
	Long:  `This command scans for open TCP ports on a host within a given range.`,
	Run: func(cmd *cobra.Command, args []string) {
		nmap.Execute(cmd, args)
	},
}
