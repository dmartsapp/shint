package cmd

import (
	"github.com/farhansabbir/telnet/cmd/web"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(webCmd)
}

var webCmd = &cobra.Command{
	Use:   "web [url]",
	Short: "Make an HTTP GET request to a URL",
	Long:  `This command makes an HTTP GET request to a URL and displays the response.`,
	Run: func(cmd *cobra.Command, args []string) {
		web.Execute(cmd, args)
	},
}
