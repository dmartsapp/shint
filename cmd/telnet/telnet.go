package telnet

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/farhansabbir/telnet/lib/handlers"
	"github.com/spf13/cobra"
)

func Execute(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		cmd.Help()
		return
	}

	host := args[0]
	port, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Println("Invalid port number")
		return
	}

	timeout, _ := cmd.Flags().GetInt("timeout")
	iterations, _ := cmd.Flags().GetInt("count")
	delay, _ := cmd.Flags().GetInt("delay")
	throttle, _ := cmd.Flags().GetBool("throttle")
	jsonoutput, _ := cmd.Flags().GetBool("json")
	payloadSize, _ := cmd.Flags().GetInt("payload")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	handlers.TelnetHandler(&jsonoutput, iterations, delay, &throttle, timeout, payloadSize, port, ctx, host)
}
