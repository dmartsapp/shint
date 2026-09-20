package handlers

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dmartsapp/shint/lib"
)

const ntpModule = "ntp"

const (
	// NTPDefaultPort is the registered NTP port.
	NTPDefaultPort = 123
	ntpPacketSize  = 48
	// ntpEpochOffset is the seconds between the NTP epoch (1900-01-01) and the
	// Unix epoch (1970-01-01).
	ntpEpochOffset = 2208988800
)

// ntpTimestamp is NTP's 64-bit time: seconds since 1900 in the top 32 bits, a
// binary fraction of a second in the bottom 32 (resolution about 0.23 ns).
type ntpTimestamp uint64

// toNTP converts t. NTP seconds wrap in 2036; like every SNTP client, this
// reads a value whose top bit is clear as 2036-2104 (see fromNTP).
func toNTP(t time.Time) ntpTimestamp {
	secs := uint64(t.Unix()+ntpEpochOffset) & 0xffffffff
	frac := (uint64(t.Nanosecond()) << 32) / 1e9
	return ntpTimestamp(secs<<32 | frac)
}

// fromNTP converts ts to a time. RFC 4330: if the top bit of the seconds is
// set the time is in 1968-2036, otherwise in 2036-2104.
func fromNTP(ts ntpTimestamp) time.Time {
	secs := int64(ts >> 32)
	if ts>>63 == 0 {
		secs += 1 << 32
	}
	nanos := (int64(ts&0xffffffff) * 1e9) >> 32
	return time.Unix(secs-ntpEpochOffset, nanos)
}

// ntpReply is what a server's answer says, decoded and checked.
type ntpReply struct {
	leap        int
	version     int
	stratum     int
	referenceID string
	receive     time.Time // T2: the server received our request
	transmit    time.Time // T3: the server sent its reply
}

var leapNames = [...]string{"none", "insert", "delete", "unsynchronized"}

// buildNTPRequest is a 48-byte client (mode 3) request, version 4, whose
// transmit timestamp is t1. The server copies it back as the originate
// timestamp, which is how the reply is matched to this request.
func buildNTPRequest(t1 time.Time) []byte {
	b := make([]byte, ntpPacketSize)
	b[0] = 0<<6 | 4<<3 | 3 // leap 0, version 4, mode 3 (client)
	binary.BigEndian.PutUint64(b[40:], uint64(toNTP(t1)))
	return b
}

// parseNTPReply decodes and validates a server reply to a request sent with
// transmit timestamp sent. It refuses anything that is not a usable answer:
// the wrong size or mode, a reply that echoes a different originate timestamp
// (not an answer to this request), a "kiss-o'-death" (stratum 0), a server
// that says its own clock is not synchronized, or one with no transmit time.
func parseNTPReply(b []byte, sent ntpTimestamp) (ntpReply, error) {
	if len(b) < ntpPacketSize {
		return ntpReply{}, fmt.Errorf("short reply: %d bytes, an NTP packet is %d", len(b), ntpPacketSize)
	}
	r := ntpReply{
		leap:    int(b[0] >> 6),
		version: int(b[0] >> 3 & 7),
		stratum: int(b[1]),
	}
	mode := int(b[0] & 7)
	if mode != 4 {
		return r, fmt.Errorf("not a server reply: mode %d, want 4", mode)
	}
	if r.version < 1 || r.version > 4 {
		return r, fmt.Errorf("unsupported NTP version %d", r.version)
	}
	if ntpTimestamp(binary.BigEndian.Uint64(b[24:])) != sent {
		return r, errors.New("reply does not match the request (originate timestamp differs)")
	}
	r.referenceID = referenceID(r.stratum, b[12:16])
	if r.stratum == 0 {
		return r, fmt.Errorf("kiss-o'-death from the server: %s (%s)", r.referenceID, kissMeaning(r.referenceID))
	}
	if r.leap == 3 {
		return r, errors.New("the server reports its clock as unsynchronized (leap indicator 3)")
	}
	if r.stratum > 15 {
		return r, fmt.Errorf("the server's stratum is %d: it is not synchronized to a time source", r.stratum)
	}
	rx, tx := ntpTimestamp(binary.BigEndian.Uint64(b[32:])), ntpTimestamp(binary.BigEndian.Uint64(b[40:]))
	if tx == 0 {
		return r, errors.New("the reply carries no transmit time")
	}
	r.receive, r.transmit = fromNTP(rx), fromNTP(tx)
	return r, nil
}

// referenceID names what the server is synchronized to: for stratum 0 and 1 a
// four-character code (a kiss code, or a clock such as GPS), for a higher
// stratum the upstream server's IPv4 address - or, when that server is IPv6, a
// hash of it that only looks like an address.
func referenceID(stratum int, b []byte) string {
	if stratum <= 1 {
		return strings.TrimRight(string(b), "\x00 ")
	}
	return net.IP(b).String()
}

func kissMeaning(code string) string {
	switch code {
	case "RATE":
		return "asked to slow down"
	case "DENY", "RSTR":
		return "access denied"
	default:
		return "see RFC 5905 section 7.4"
	}
}

// ntpExchange is the outcome of one query: the times that matter and the two
// derived measurements.
type ntpExchange struct {
	reply     ntpReply
	sent      time.Time // T1
	received  time.Time // T4
	offset    time.Duration
	roundTrip time.Duration
}

// queryNTP asks one server for the time and works out this machine's offset
// from it, the standard way: with T1 (request sent), T2 (server received),
// T3 (server replied) and T4 (reply received),
//
//	offset     = ((T2 - T1) + (T3 - T4)) / 2   server time minus local time
//	round trip = (T4 - T1) - (T3 - T2)         network delay, not server time
//
// which is right even when the network delay is not symmetric, as long as it
// is about the same in both directions.
func queryNTP(address string, port int, timeoutSeconds int) (ntpExchange, error) {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(address, strconv.Itoa(port)), time.Duration(timeoutSeconds)*time.Second)
	if err != nil {
		return ntpExchange{}, err
	}
	defer func() { _ = conn.Close() }()

	t1 := time.Now()
	request := buildNTPRequest(t1)
	if err := conn.SetDeadline(time.Now().Add(time.Duration(timeoutSeconds) * time.Second)); err != nil {
		return ntpExchange{}, err
	}
	if _, err := conn.Write(request); err != nil {
		return ntpExchange{}, err
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	t4 := time.Now()
	if err != nil {
		return ntpExchange{sent: t1}, err
	}
	reply, err := parseNTPReply(buf[:n], ntpTimestamp(binary.BigEndian.Uint64(request[40:])))
	if err != nil {
		return ntpExchange{sent: t1, received: t4}, err
	}
	return ntpExchange{
		reply:     reply,
		sent:      t1,
		received:  t4,
		offset:    (reply.receive.Sub(t1) + reply.transmit.Sub(t4)) / 2,
		roundTrip: t4.Sub(t1) - reply.transmit.Sub(reply.receive),
	}, nil
}

// formatOffset renders a signed duration with an explicit sign: "+1.5ms".
func formatOffset(d time.Duration) string {
	if d >= 0 {
		return "+" + d.String()
	}
	return d.String()
}

// NTPHandler asks every address host resolves to for the time, iterations
// times, and reports how far this machine's clock is from each answer. It
// reports true only if the lookup and every query succeeded, and - when
// maxOffset is above zero - no offset was larger than maxOffset either.
//
// A reply that arrives but cannot be trusted (wrong originate timestamp, a
// kiss-o'-death, an unsynchronized server) is a failed query, not a result.
func NTPHandler(jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, port int, maxOffset time.Duration, host string) (ok bool) {
	var statsMutex sync.Mutex
	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     ntpModule,
		Host:     host,
		FromPort: port,
		ToPort:   port,
		Protocol: "udp",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Payload:  ntpPacketSize,
		Throttle: *throttle,
	}
	output.ModuleName = ntpModule
	istart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	ipaddresses, err := lib.ResolveName(ctx, host)
	if err != nil {
		if *jsonoutput {
			output.DNSLookup = lib.DNSLookup{Hostname: host, Success: false, Error: err.Error(), TimeTaken: time.Since(istart).Microseconds()}
			output.Error = err.Error()
			output.Stats = make([]lib.NTPStats, 0)
			output.StartTime = istart.UnixMicro()
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		} else {
			fmt.Println(lib.LogWithTimestamp(ntpModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error(), "time", time.Since(istart)), true))
		}
		return false
	}

	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(ntpModule, "dns resolved "+lib.Fields("host", host, "addresses", len(ipaddresses), "ips", "["+strings.Join(ipaddresses, ",")+"]", "time", time.Since(istart)), false))
	} else {
		output.DNSLookup = lib.DNSLookup{Hostname: host, Success: true, ResolvedAddresses: ipaddresses, TimeTaken: time.Since(istart).Microseconds()}
		output.Stats = make([]lib.NTPStats, 0)
		output.StartTime = istart.UnixMicro()
	}

	var answered, failures int
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		for _, ip := range ipaddresses {
			time.Sleep(attemptDelay(delay, *throttle))
			begin := time.Now()
			ex, qerr := queryNTP(ip, port, timeout)
			taken := time.Since(begin)
			if qerr == nil && maxOffset > 0 && absDuration(ex.offset) > maxOffset {
				qerr = fmt.Errorf("offset %s is larger than --max-offset %s", formatOffset(ex.offset), maxOffset)
			}

			statsMutex.Lock()
			if qerr != nil {
				failures++
			} else {
				answered++
			}
			if *jsonoutput {
				stat := lib.NTPStats{Address: ip, Success: qerr == nil, SentTime: begin.UnixMicro(), TimeTaken: taken.Microseconds()}
				if !ex.received.IsZero() {
					stat.RecvTime = ex.received.UnixMicro()
				}
				if ex.reply.stratum != 0 || ex.reply.version != 0 {
					stat.Stratum, stat.Version, stat.ReferenceID = ex.reply.stratum, ex.reply.version, ex.reply.referenceID
					stat.LeapIndicator = leapNames[ex.reply.leap&3]
				}
				if !ex.reply.transmit.IsZero() {
					stat.OffsetUs, stat.RoundTripUs, stat.ServerTimeUs = ex.offset.Microseconds(), ex.roundTrip.Microseconds(), ex.reply.transmit.UnixMicro()
				}
				if qerr != nil {
					stat.Error = qerr.Error()
				}
				output.Stats = append(output.Stats.([]lib.NTPStats), stat)
			}
			statsMutex.Unlock()

			if !*jsonoutput {
				where := lib.Fields("server", host, "address", ip, "attempt", fmt.Sprintf("%d/%d", attempt, iterations))
				if qerr != nil {
					fmt.Println(lib.LogWithTimestamp(ntpModule, "query failed "+where+" "+lib.Fields("time", taken, "error", qerr.Error()), true))
				} else {
					fmt.Println(lib.LogWithTimestamp(ntpModule, "response "+where+" "+lib.Fields("stratum", ex.reply.stratum, "offset", formatOffset(ex.offset), "round_trip", ex.roundTrip, "leap", leapNames[ex.reply.leap&3], "version", ex.reply.version, "reference", ex.reply.referenceID, "time", taken), false))
				}
			}
		}
	}

	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
	} else {
		fmt.Println(lib.LogWithTimestamp(ntpModule, "done "+lib.Fields("queries", iterations*len(ipaddresses), "answered", answered, "total_time", time.Since(istart)), false))
	}
	return failures == 0
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
