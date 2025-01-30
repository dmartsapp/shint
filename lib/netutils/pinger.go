package netutils

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type ICMPPacket struct {
	Destination         net.IPAddr `json:"destination"`
	PayloadSize         int        `json:"payload_size"`
	Sequence            int        `json:"sequence_number"`
	SentDateTimeUNIX    int64      `json:"sent_datetime_unix_ms"`
	ReceiveDateTimeUNIX int64      `json:"receive_datetime_unix_ms"`
	ErrorEncountered    bool       `json:"is_error_encountered"`
	ErrorStr            string     `json:"error_string"`
}
type Stats struct {
	Packets         []ICMPPacket  `json:"icmp_packets"`
	Loss            int           `json:"loss"`
	Min             int           `json:"min_ms"`
	Max             int           `json:"max_ms"`
	Avg             float64       `json:"avg_ms"`
	StdDev          float64       `json:"stddev"`
	ResolveTime     time.Duration `json:"resolve_time_ms"`
	ResolveTimedOut bool          `json:"is_resolve_timed_out"`
	TotalTime       time.Duration `json:"total_time_taken_ms"`
}

type Pinger struct {
	DestinationStr     string   `json:"destination"`
	Destination        []net.IP `json:"destination_ip_addresses"`
	TTL                int      `json:"ttl"`
	ResolveTimeout     int      `json:"resolve_timeout_ms"`
	Payload            string   `json:"payload_data"`
	Count              int      `json:"ping_count"`
	Stats              *Stats   `json:"stats"`
	IsSequential       bool     `json:"is_sequential_ping"`
	PingDelay          int      `json:"ping_delay_ms"`
	RandomizePingDelay bool     `json:"is_ping_delay_random"`
	MTU                int      `json:"mtu"`
}

var (
	_pinger_wg      = sync.WaitGroup{}
	_pinger_channel = make(chan ICMPPacket, 1)
	_stream_channel = make((chan string))
	// _pinger_mutux = sync.Mutex{}
)

const (
	_DEFAULT_TTL                = 1000
	_DEFAULT_PING_COUNT         = 4
	_DEFAULT_PAYLOAD_SIZE       = 4
	_DEFAULT_MTU                = 1500
	_DEFAULT_MAX_PAYLOAD_SIZE   = _DEFAULT_MTU - 16 - 20 - 16 // MTU - 16 bytes_of_src_datetime - 20 bytes_of_icmp_header - 16 bytes_of_checksum
	_DEFAULT_NETWORK            = "ip4"
	_DEFAULT_RESOLVE_TIMEOUT_MS = 5000
	_DEFAULT_PING_DELAY_MS      = 1000
	_DEFAULT_MAX_DELAY          = 10000
	_DEFAULT_LISTEN_ADDRESS     = "0.0.0.0"
)

func NewPinger(destination string) *Pinger {
	pinger := Pinger{
		DestinationStr: destination,
		TTL:            _DEFAULT_TTL,
		Destination:    []net.IP{},
		Payload:        strings.Repeat("d", _DEFAULT_PAYLOAD_SIZE),
		Count:          _DEFAULT_PING_COUNT,
		Stats:          &Stats{},
		IsSequential:   true,
		ResolveTimeout: _DEFAULT_RESOLVE_TIMEOUT_MS,
		PingDelay:      _DEFAULT_PING_DELAY_MS,
		MTU:            _DEFAULT_MTU,
	}

	return &pinger
}

func (pinger *Pinger) MeasureStats() *Stats {
	// fmt.Println("Calculate the stats, now that pingers have stopped sending packets")
	if pinger.Stats.ResolveTimedOut {
		return pinger.Stats
	}
	timetaken := make([]int, 0)
	sum := 0
	for _, packet := range pinger.Stats.Packets {
		if packet.ErrorEncountered {
			continue
		}
		sum += int(packet.ReceiveDateTimeUNIX - packet.SentDateTimeUNIX)
		timetaken = append(timetaken, int(packet.ReceiveDateTimeUNIX-packet.SentDateTimeUNIX))
	}
	pinger.Stats.Avg = float64(sum) / float64(pinger.Count)
	if len(timetaken) > 0 {
		pinger.Stats.Max = slices.Max(timetaken)
		pinger.Stats.Min = slices.Min(timetaken)
	}
	return pinger.Stats
}

func (pinger *Pinger) Stream() <-chan string {
	return _stream_channel
}

func (pinger *Pinger) Ping() error {
	start := time.Now()
	// resolve the name first to populate pinger object properties
	if err := pinger.resolveName(pinger.DestinationStr); err != nil {
		pinger.Stats.TotalTime = time.Since(start)
		return err
	}
	_pinger_wg.Add(1)
	// start monitoring the pinger channel for incoming data from completed pings
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		for packet := range _pinger_channel {
			pinger.Stats.Packets = append(pinger.Stats.Packets, packet)
			if len(pinger.Stats.Packets) == pinger.Count {
				close(_pinger_channel)
				close(_stream_channel)
			}
		}
	}(&_pinger_wg)

	if pinger.IsSequential {
		for seq := range pinger.Count {
			for _, ip := range pinger.Destination {
				pinger.sendicmp(ip, seq)
				if pinger.RandomizePingDelay {
					pinger.PingDelay = rand.Intn(_DEFAULT_MAX_DELAY)
				}
				time.Sleep(time.Millisecond * time.Duration(pinger.PingDelay))
			}
		}
		// close(_pinger_channel)
	} else {

		for seq := range pinger.Count {
			for _, ip := range pinger.Destination {
				_pinger_wg.Add(1)
				go func(wg *sync.WaitGroup) {
					defer wg.Done()
					pinger.sendicmp(ip, seq)
				}(&_pinger_wg)

				if pinger.RandomizePingDelay {
					pinger.PingDelay = rand.Intn(_DEFAULT_MAX_DELAY)
				}
				time.Sleep(time.Millisecond * time.Duration(pinger.PingDelay))
			}
		}
		_pinger_wg.Wait()
		// close(_pinger_channel)
	}
	pinger.Stats.TotalTime = time.Since(start)
	return nil
}

func (pinger *Pinger) SetParallelPing(parallel bool) *Pinger {
	// explicitly sets the ping to run in parallel
	pinger.IsSequential = !parallel
	return pinger
}

func (pinger *Pinger) SetPayloadSizeInBytes(payload_size int) *Pinger {
	// explicitly sets the size of the ping requests within boundary of _DEFAULT_MAX_PAYLOAD_SIZE
	// returns nil
	pinger.Payload = strings.Repeat("d", payload_size%_DEFAULT_MAX_PAYLOAD_SIZE)
	return pinger
}

func (pinger *Pinger) SetPingCount(count int) *Pinger {
	// explicitly set ping count. Checks if set below 0, then converts to absolute
	// default is usually 4 as defined in _DEFAULT_PING_COUNT
	// returns nil
	if count < 0 {
		count *= -1
	}
	pinger.Count = count
	return pinger
}

func (pinger *Pinger) SetResolveTimeout(timeout int) *Pinger {
	// explicitly set ping delay. Checks for timeout less than 0ms
	// default is usually 5000ms as defined in _DEFAULT_RESOLVE_TIMEOUT_MS, hence sets if timeout <0
	if timeout < 0 {
		pinger.ResolveTimeout = _DEFAULT_RESOLVE_TIMEOUT_MS
	} else {
		pinger.ResolveTimeout = timeout
	}
	return pinger
}

func (pinger *Pinger) SetPingDelayInMS(delay int) *Pinger {
	// explicitly set ping delay. Checks for delay
	// default is usually 1000ms as defined in _DEFAULT_PING_DELAY_MS, hence sets if delay <=0
	if delay < 0 {
		pinger.PingDelay = _DEFAULT_PING_DELAY_MS
	} else {
		pinger.PingDelay = delay
	}
	return pinger
}

func (pinger *Pinger) SetTTL(ttl int) *Pinger {
	pinger.TTL = ttl
	return pinger
}

func (pinger *Pinger) SetRandomizedPingDelay(random bool) {
	// explicitly set if ping delay should be randomized
	// returns nil
	pinger.RandomizePingDelay = random
}

func (p *Pinger) String() string {
	// returns json representation of the pinger object
	if str, err := json.Marshal(p); err != nil {
		log.Fatal(err)
		return err.Error()
	} else {
		return string(str)
	}
}

func (stats *Stats) String() string {
	// returns json representation of the pinger object
	if str, err := json.Marshal(stats); err != nil {
		log.Fatal(err)
		return err.Error()
	} else {
		return string(str)
	}
}

func (pinger *Pinger) resolveName(destination string) error {
	// method resolves the name against a timeout defined in ResolveTimeout
	// also populates basic properties like
	// - resolved addresses and
	// - time taken to resolve
	// - if timed out to resolve, marks resolvedtimedout to true
	// - returns error if error encountered while resolve execution
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*time.Duration(pinger.ResolveTimeout))
	defer cancel()
	start := time.Now()
	addr, err := net.DefaultResolver.LookupIP(ctx, _DEFAULT_NETWORK, destination)
	if err != nil {
		pinger.Stats.ResolveTime = time.Duration(time.Since(start).Milliseconds())
		pinger.Stats.ResolveTimedOut = true
		_stream_channel <- "Unable to resolve for " + destination + " with " + strconv.Itoa(len(pinger.Payload)) + " bytes of data"
		return err
	}
	pinger.Destination = addr
	pinger.Stats.ResolveTime = time.Duration(time.Since(start).Milliseconds())
	pinger.Stats.ResolveTimedOut = false

	return nil
}

func (pinger *Pinger) sendicmp(destination net.IP, seq int) {
	time.Sleep(time.Millisecond * time.Duration(pinger.PingDelay))
	icmppacket := ICMPPacket{
		Destination: net.IPAddr{
			IP: destination,
		},
		Sequence:         seq,
		PayloadSize:      len(pinger.Payload),
		SentDateTimeUNIX: time.Now().UnixMilli(),
	}
	var icmpconn *icmp.PacketConn
	var err error

	// Start listening for icmp replies
	if runtime.GOOS == "windows" {
		if icmpconn, err = icmp.ListenPacket("ip4:icmp", _DEFAULT_LISTEN_ADDRESS); err != nil {
			icmppacket.ErrorEncountered = true
			pinger.Stats.Loss += 1
			icmppacket.ErrorStr = err.Error()
			_pinger_channel <- icmppacket
			return
		}
		defer icmpconn.Close()
	} else {
		if icmpconn, err = icmp.ListenPacket("udp4", _DEFAULT_LISTEN_ADDRESS); err != nil {
			icmppacket.ErrorEncountered = true
			pinger.Stats.Loss += 1
			icmppacket.ErrorStr = err.Error()
			_pinger_channel <- icmppacket
			return
		}
		defer icmpconn.Close()
	}
	// Make a new ICMP message
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   seq & 0xffff,
			Seq:  seq,                    //<< uint(seq), // TODO
			Data: []byte(pinger.Payload), // 4 bytes per char
		},
	}
	msg_bytes, err := msg.Marshal(nil)
	if err != nil {
		icmppacket.ErrorEncountered = true
		pinger.Stats.Loss += 1
		icmppacket.ErrorStr = err.Error()
		_pinger_channel <- icmppacket
		return
	}
	// _stream_channel <- "Sending request #" + strconv.Itoa(seq) + " to " + destination.String() + " with " + strconv.Itoa(len(pinger.Payload)) + " bytes of data"
	if runtime.GOOS == "windows" {
		_, err := icmpconn.WriteTo(msg_bytes, &net.IPAddr{IP: destination})
		if err != nil {
			icmppacket.ErrorEncountered = true
			pinger.Stats.Loss += 1
			icmppacket.ErrorStr = err.Error()
			_stream_channel <- "Error encountered for request #" + strconv.Itoa(seq) + " to " + destination.String() + " with " + strconv.Itoa(len(pinger.Payload)) + " bytes of data"
			_pinger_channel <- icmppacket
			return
		}
	} else {
		_, err = icmpconn.WriteTo(msg_bytes, &net.UDPAddr{IP: destination})
		icmppacket.SentDateTimeUNIX = time.Now().UnixMilli()
		if err != nil {
			icmppacket.ErrorEncountered = true
			pinger.Stats.Loss += 1
			icmppacket.ErrorStr = err.Error()
			_stream_channel <- "Error encountered for request #" + strconv.Itoa(seq) + " to " + destination.String() + " with " + strconv.Itoa(len(pinger.Payload)) + " bytes of data"
			_pinger_channel <- icmppacket
			return
		}
	}

	for {
		// Wait for a reply
		reply := make([]byte, _DEFAULT_MTU)
		err = icmpconn.SetReadDeadline(time.Now().Add(time.Duration(pinger.TTL) * time.Millisecond))
		if err != nil {
			icmppacket.ErrorEncountered = true
			pinger.Stats.Loss += 1
			icmppacket.ErrorStr = err.Error()
			_stream_channel <- "Error encountered for request #" + strconv.Itoa(seq) + " to " + destination.String() + " with " + strconv.Itoa(len(pinger.Payload)) + " bytes of data"
			_pinger_channel <- icmppacket
			return
		}
		n, _, err := icmpconn.ReadFrom(reply)
		if err != nil {
			icmppacket.ErrorEncountered = true
			pinger.Stats.Loss += 1
			icmppacket.ErrorStr = err.Error()
			_stream_channel <- "Error encountered for request #" + strconv.Itoa(seq) + " to " + destination.String() + " with " + strconv.Itoa(len(pinger.Payload)) + " bytes of data"
			_pinger_channel <- icmppacket
			return
		}

		rm, err := icmp.ParseMessage(1, reply[:n])
		if err != nil {
			icmppacket.ErrorEncountered = true
			pinger.Stats.Loss += 1
			icmppacket.ErrorStr = err.Error()
			_stream_channel <- "Error encountered for request #" + strconv.Itoa(seq) + " to " + destination.String() + " with " + strconv.Itoa(len(pinger.Payload)) + " bytes of data"
			_pinger_channel <- icmppacket
			return
		}
		switch rm.Type {
		case ipv4.ICMPTypeEchoReply:
			body, _ := rm.Body.Marshal(ipv4.ICMPTypeEchoReply.Protocol())

			if int(body[3]) == seq {
				icmppacket.ReceiveDateTimeUNIX = time.Now().UnixMilli()
				_stream_channel <- time.Now().Local().Format("12/12/2014 18:23:21") + ": Received response for request #" + strconv.Itoa(seq) + " from " + destination.String() + " with " + strconv.Itoa(icmppacket.PayloadSize) + " bytes of data"
				_pinger_channel <- icmppacket
				return
			} else { // sequence mismatch, look for another packet to match
				continue
			}

			// default:
			// 	return dst, 0, fmt.Errorf("%v %+v", peer, rm.Type)
		}
	}
}
