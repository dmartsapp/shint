package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dmartsapp/go-ping/netutils"
	"github.com/dmartsapp/telnet/lib"
)

func HandleICMP(host string, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, payload_size int) {

	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     "icmp",
		Host:     host,
		FromPort: int(7),
		ToPort:   int(7),
		Protocol: "icmp",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Payload:  payload_size,
		Throttle: *throttle,
	}
	output.ModuleName = "icmp"
	start := time.Now()
	pinger, err := netutils.NewPinger(host)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(err.Error(), true))
		os.Exit(1)
	}

	wg := sync.WaitGroup{}

	if !*jsonoutput {
		wg.Add(1)
		go func(pinger *netutils.Pinger, wg *sync.WaitGroup) {
			defer wg.Done()
			for log := range pinger.StreamLog() {
				fmt.Println(lib.LogWithTimestamp(log, false))
			}
		}(pinger, &wg)
	}

	pinger.
		SetPingCount(iterations).
		SetParallelPing(true).
		SetPayloadSizeInBytes(payload_size).
		SetPingDelayInMS(delay).
		SetRandomizedPingDelay(*throttle)
	err = pinger.PingAll()
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(err.Error(), true))
		os.Exit(1)
	}

	wg.Wait()
	pinger.MeasureStats()
	// pinger.MeasureStats()
	if !*jsonoutput {
		fmt.Println("========================================= Ping stats ============================================")
		fmt.Printf("Packets sent: %d, Packets received: %d, Packets lost: %d, Ping success: %d%% \n", pinger.Count*len(pinger.Destination), (pinger.Count*len(pinger.Destination) - pinger.Stats.Loss), pinger.Stats.Loss, ((pinger.Count*len(pinger.Destination) - pinger.Stats.Loss) * 100 / (pinger.Count * len(pinger.Destination))))
		fmt.Printf("Total time: %v, Resolve time: %v\n", pinger.Stats.TotalTime, pinger.Stats.ResolveTime)
		fmt.Printf("Min time: %dms, Max time: %dms, Avg time: %.3fms, Std dev: %.3f, Total time: %v\n", pinger.Stats.Min, pinger.Stats.Max, pinger.Stats.Avg, pinger.Stats.StdDev, pinger.Stats.TotalTime)
	} else {
		output.DNSLookup = lib.DNSLookup{
			Hostname:          host,
			Success:           true,
			ResolvedAddresses: lib.ConvertIPToStringSlice(pinger.Destination),
			TimeTaken:         pinger.Stats.ResolveTime.Microseconds(),
		}
		output.StartTime = start.UnixMicro()
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		output.Stats = make([]lib.ICMPStats, 0)
		for _, pckts := range pinger.Stats.Packets {
			stat := lib.ICMPStats{}
			stat.Address = pckts.Destination.String()
			stat.Success = !pckts.ErrorEncountered
			stat.Sequence = pckts.Sequence
			stat.SentTime = pckts.SentDateTimeUNIX
			stat.RecvTime = pckts.ReceiveDateTimeUNIX
			stat.TimeTaken = stat.RecvTime - stat.SentTime
			output.Stats = append(output.Stats.([]lib.ICMPStats), stat)
		}

		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))

	}
}
