package lib

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	// DATETIMEFORMAT string = "Mon, 02 Jan 2006 15:04:05 MST"
	DATETIMEFORMAT string = time.UnixDate
	NetworkType    string = "ip4" // other networks are ip which includes both v4 and v6, and ip6 which is only v6
	Protocol       string = "tcp"
)

func ResolveName(ctx context.Context, name string) ([]string, error) {
	var resolver net.Resolver
	ipaddresses, err := resolver.LookupIP(ctx, NetworkType, name)
	var addresses = make([]string, 0)
	for _, address := range ipaddresses {
		addresses = append(addresses, address.String())
	}
	return addresses, err
}

func ResolveNameToIPs(ctx context.Context, name string) ([]net.IP, error) {
	var resolver net.Resolver
	return resolver.LookupIP(ctx, NetworkType, name)
}

func LogWithTimestamp(log string, iserror bool) string {
	if !iserror {
		return time.Now().Format(DATETIMEFORMAT) + ": " + log
	}
	return time.Now().Format(DATETIMEFORMAT) + ": Error! " + log

}

func GetMinAvgMax(stats []time.Duration) (time.Duration, time.Duration, time.Duration) {
	max := slices.Max(stats)
	min := slices.Min(stats)
	var avg int
	for _, stat := range stats {
		avg += int(stat.Nanoseconds())
	}
	return min, time.Duration(avg / len(stats)), max
}

func SortTimeDurationSlice(stats *[]time.Duration) {
	sort.SliceStable(*stats, func(i, j int) bool {
		return ((*stats)[i] <= (*stats)[j])
	})
}

func LogStats(modulename string, stats []time.Duration, iterations int) string {
	if len(stats) > 0 {
		min, avg, max := GetMinAvgMax(stats)
		return "\n" + strings.Repeat("=", (45-len(modulename))) + " " + modulename + " STATISTICS " + strings.Repeat("=", (45-len(modulename))) + "\nRequests sent: " + strconv.Itoa(iterations) + ", Response received: " + strconv.Itoa(len(stats)) + ", Success: " + strconv.Itoa(len(stats)*100/iterations) + "%\nLatency: minimum: " + min.String() + ", average: " + avg.String() + ", maximum: " + max.String()
	} else {
		return "\n" + strings.Repeat("=", (45-len(modulename))) + " " + modulename + " STATISTICS " + strings.Repeat("=", (45-len(modulename))) + "\nRequests sent: " + strconv.Itoa(iterations) + ", Response received: " + strconv.Itoa(len(stats)) + "\nLatency: minimum: 0, average: 0, maximum: 0"
	}

}

func IsPortUp(host string, port int, timeout int) (bool, error) {
	var dialer = net.Dialer{Timeout: time.Duration(timeout * int(time.Second))}
	conn, err := dialer.Dial(Protocol, host+":"+strconv.Itoa(port))
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return true, nil
}

var ListenAddr = "0.0.0.0"

// Mostly based on https://github.com/golang/net/blob/master/icmp/ping_test.go
// All ye beware, there be dragons below...

func Ping(dst *net.IPAddr, options ...map[string]int) (*net.IPAddr, time.Duration, error) {
	icmp_payload := "devn" // 4 bytes per char
	var seq int
	var icmpconn *icmp.PacketConn
	var err error
	for _, option := range options {
		seq = option["seq"]
	}
	// Start listening for icmp replies
	if runtime.GOOS == "windows" {
		if icmpconn, err = icmp.ListenPacket("ip4:icmp", ListenAddr); err != nil {
			return nil, 0, err
		}
		defer icmpconn.Close()
	} else {
		if icmpconn, err = icmp.ListenPacket("udp4", ListenAddr); err != nil {
			return nil, 0, err
		}
		defer icmpconn.Close()
	}
	// Make a new ICMP message
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  seq,                  //<< uint(seq), // TODO
			Data: []byte(icmp_payload), // 4 bytes per char
		},
	}
	msg_bytes, err := msg.Marshal(nil)
	if err != nil {
		return dst, 0, err
	}
	// Send it
	start := time.Now()
	if runtime.GOOS == "windows" {
		_, err := icmpconn.WriteTo(msg_bytes, dst)
		if err != nil {
			fmt.Println(err)
			return dst, 0, err
		}
	} else {
		_, err = icmpconn.WriteTo(msg_bytes, &net.UDPAddr{IP: net.ParseIP(dst.IP.String())})
		if err != nil {
			fmt.Println(err)
			return dst, 0, err
		}
	}

	// Wait for a reply
	reply := make([]byte, 1500)
	err = icmpconn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		return dst, 0, err
	}
	n, peer, err := icmpconn.ReadFrom(reply)
	if err != nil {
		return dst, 0, err
	}
	duration := time.Since(start)

	rm, err := icmp.ParseMessage(1, reply[:n])
	if err != nil {
		return dst, 0, err
	}
	switch rm.Type {
	case ipv4.ICMPTypeEchoReply:
		body, _ := rm.Body.Marshal(ipv4.ICMPTypeEchoReply.Protocol())
		if string(body[4:]) == icmp_payload {
			return dst, duration, nil
		} else {
			return dst, duration, fmt.Errorf("request and response payloads do not match")
		}

	default:
		return dst, 0, fmt.Errorf("%v %+v", peer, rm.Type)
	}
}
