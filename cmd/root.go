package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "telnet",
	Short: "A simple network utility tool",
	Long:  `A simple network utility tool that provides telnet, ping, nmap, and web client functionalities.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default action when no subcommand is provided
		fmt.Println("Welcome to the network utility tool. Use 'telnet --help' for more information.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Int("count", 1, "Number of times to check connectivity")
	rootCmd.PersistentFlags().Int("timeout", 5, "Timeout in seconds to connect")
	rootCmd.PersistentFlags().Int("delay", 1000, "Seconds delay between each iteration given in count")
	rootCmd.PersistentFlags().Int("payload", 4, "Ping payload size in bytes")
	rootCmd.PersistentFlags().Bool("throttle", false, "Flag option to throttle between every iteration of count to simulate non-uniform request.")
	rootCmd.PersistentFlags().Bool("json", false, "Flag option to output only in JSON format")
}