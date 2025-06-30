package ping

import (
	"github.com/farhansabbir/telnet/lib/handlers"
	"github.com/spf13/cobra"
)

func Execute(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Help()
		return
	}

	host := args[0]

	timeout, _ := cmd.Flags().GetInt("timeout")
	iterations, _ := cmd.Flags().GetInt("count")
	delay, _ := cmd.Flags().GetInt("delay")
	throttle, _ := cmd.Flags().GetBool("throttle")
	jsonoutput, _ := cmd.Flags().GetBool("json")
	payloadSize, _ := cmd.Flags().GetInt("payload")

	handlers.HandleICMP(host, &jsonoutput, iterations, delay, &throttle, timeout, payloadSize)
}
