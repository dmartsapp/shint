package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/go-ping/netutils"
	"github.com/dmartsapp/shint/lib"
)

const icmpModule = "icmp"

func HandleICMP(host string, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, payload_size int) {

	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     icmpModule,
		Host:     host,
		FromPort: 7,
		ToPort:   7,
		Protocol: "icmp",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Payload:  payload_size,
		Throttle: *throttle,
	}
	output.ModuleName = icmpModule
	start := time.Now()
	pinger, err := netutils.NewPinger(host)
	if err != nil {
		fmt.Println(lib.LogWithTimestamp(icmpModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error()), true))
		os.Exit(1)
	}

	wg := sync.WaitGroup{}

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(icmpModule, "dns resolved "+lib.Fields("host", host, "addresses", len(pinger.Destination), "ips", "["+strings.Join(lib.ConvertIPToStringSlice(pinger.Destination), ",")+"]", "time", pinger.Stats.ResolveTime), false))
		wg.Add(1)
		go func(pinger *netutils.Pinger, wg *sync.WaitGroup) {
			defer wg.Done()
			for log := range pinger.StreamLog() {
				fmt.Println(lib.LogWithTimestamp(icmpModule, log, false))
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
		fmt.Println(lib.LogWithTimestamp(icmpModule, "ping failed "+lib.Fields("host", host, "error", err.Error()), true))
		os.Exit(1)
	}

	wg.Wait()
	pinger.MeasureStats()
	totalPings := pinger.Count * len(pinger.Destination)
	if !*jsonoutput {
		durations := make([]time.Duration, 0, len(pinger.Stats.Packets))
		for _, pckt := range pinger.Stats.Packets {
			if !pckt.ErrorEncountered {
				durations = append(durations, time.Duration(pckt.ReceiveDateTimeUNIX-pckt.SentDateTimeUNIX)*time.Millisecond)
			}
		}
		fmt.Println(lib.LogStats(icmpModule, durations, totalPings))
		fmt.Println(lib.LogWithTimestamp(icmpModule, "done "+lib.Fields("packets_lost", pinger.Stats.Loss, "stddev_ms", fmt.Sprintf("%.3f", pinger.Stats.StdDev), "resolve_time", pinger.Stats.ResolveTime, "total_time", pinger.Stats.TotalTime), false))
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
		output.Stats = make([]lib.ICMPStats, 0, len(pinger.Stats.Packets))
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
