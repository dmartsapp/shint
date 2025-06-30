package web

import (
	"fmt"
	"net/url"

	"github.com/farhansabbir/telnet/lib/handlers"
	"github.com/spf13/cobra"
)

func Execute(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Help()
		return
	}

	URL, err := url.Parse(args[0])
	if err != nil {
		fmt.Println("Invalid URL")
		return
	}

	timeout, _ := cmd.Flags().GetInt("timeout")
	iterations, _ := cmd.Flags().GetInt("count")
	delay, _ := cmd.Flags().GetInt("delay")
	throttle, _ := cmd.Flags().GetBool("throttle")
	jsonoutput, _ := cmd.Flags().GetBool("json")

	handlers.WebHandler(&jsonoutput, iterations, delay, &throttle, timeout, URL)
}
