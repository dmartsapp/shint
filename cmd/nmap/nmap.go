package nmap

import (
	"context"
	"time"

	"github.com/farhansabbir/telnet/lib/handlers"
	"github.com/spf13/cobra"
)

func Execute(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Help()
		return
	}

	host := args[0]

	fromport, _ := cmd.Flags().GetInt("from")
	endport, _ := cmd.Flags().GetInt("to")
	timeout, _ := cmd.Flags().GetInt("timeout")
	iterations, _ := cmd.Flags().GetInt("count")
	throttle, _ := cmd.Flags().GetBool("throttle")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	handlers.NmapHandler(ctx, host, fromport, endport, iterations, timeout, throttle)
}
