package handlers

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/lib"
)

// fakeNTP is an SNTP server whose clock and behaviour a test controls. It
// answers each request with a well-formed reply, then applies mangle (if any)
// so a test can break exactly one thing.
type fakeNTP struct {
	offset  time.Duration // its clock minus the real one
	stratum byte
	leap    byte
	refid   [4]byte
	hold    time.Duration // time between receiving the request and sending the reply
	silent  bool
	mangle  func(reply, request []byte)
}

func (f *fakeNTP) start(t *testing.T, network, ip string) (port int) {
	t.Helper()
	conn, err := net.ListenUDP(network, &net.UDPAddr{IP: net.ParseIP(ip)})
	if err != nil {
		t.Skipf("cannot listen on %s: %v", ip, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if f.silent || n < ntpPacketSize {
				continue
			}
			request := append([]byte(nil), buf[:n]...)
			received := toNTP(time.Now().Add(f.offset))
			time.Sleep(f.hold)
			reply := make([]byte, ntpPacketSize)
			reply[0] = f.leap<<6 | 4<<3 | 4 // version 4, mode 4 (server)
			reply[1] = f.stratum
			copy(reply[12:16], f.refid[:])
			copy(reply[24:32], request[40:48]) // originate = the client's transmit
			binary.BigEndian.PutUint64(reply[32:], uint64(received))
			binary.BigEndian.PutUint64(reply[40:], uint64(toNTP(time.Now().Add(f.offset))))
			if f.mangle != nil {
				f.mangle(reply, request)
			}
			_, _ = conn.WriteToUDP(reply, addr)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func newFake() *fakeNTP { return &fakeNTP{stratum: 2, refid: [4]byte{192, 0, 2, 1}} }

func TestNTPTimestampConversion(t *testing.T) {
	if got := toNTP(time.Unix(0, 0)); got != ntpTimestamp(2208988800)<<32 {
		t.Errorf("the Unix epoch is %#x in NTP time, want 2208988800 seconds", uint64(got))
	}
	for name, tc := range map[string]struct {
		ts   ntpTimestamp
		want time.Time
	}{
		"the Unix epoch":               {ntpTimestamp(2208988800) << 32, time.Unix(0, 0)},
		"top bit set: 1968 (2^31 s)":   {ntpTimestamp(1<<31) << 32, time.Date(1968, 1, 20, 3, 14, 8, 0, time.UTC)},
		"seconds 0 is 2036, not 1900":  {0, time.Date(2036, 2, 7, 6, 28, 16, 0, time.UTC)},
		"half a second past the epoch": {ntpTimestamp(2208988800)<<32 | 1<<31, time.Unix(0, 500_000_000)},
	} {
		if got := fromNTP(tc.ts); !got.Equal(tc.want) {
			t.Errorf("%s: fromNTP = %v, want %v", name, got.UTC(), tc.want.UTC())
		}
	}
	for _, at := range []time.Time{time.Now(), time.Date(2026, 9, 20, 1, 2, 3, 123456789, time.UTC), time.Date(2040, 6, 1, 0, 0, 0, 999999999, time.UTC), time.Date(1975, 1, 1, 0, 0, 0, 1, time.UTC)} {
		diff := fromNTP(toNTP(at)).Sub(at)
		if diff < -time.Nanosecond || diff > time.Nanosecond {
			t.Errorf("round trip of %v is off by %v", at, diff)
		}
	}
}

func TestBuildNTPRequest(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 500, time.UTC)
	b := buildNTPRequest(now)
	if len(b) != 48 || b[0] != 0x23 {
		t.Fatalf("length %d, first byte %#x; want 48 and 0x23 (leap 0, version 4, client mode)", len(b), b[0])
	}
	if ntpTimestamp(binary.BigEndian.Uint64(b[40:])) != toNTP(now) {
		t.Error("the transmit timestamp is not the send time")
	}
	for i, v := range b {
		if (i < 40 && i != 0 && v != 0) || (i >= 48) {
			t.Errorf("byte %d is %#x, want 0", i, v)
		}
	}
}

func TestParseNTPReplyRefusesWhatIsNotAnAnswer(t *testing.T) {
	sent := toNTP(time.Now())
	good := func() []byte {
		b := make([]byte, 48)
		b[0], b[1] = 4<<3|4, 2
		binary.BigEndian.PutUint64(b[24:], uint64(sent))
		binary.BigEndian.PutUint64(b[32:], uint64(toNTP(time.Now())))
		binary.BigEndian.PutUint64(b[40:], uint64(toNTP(time.Now())))
		return b
	}
	if _, err := parseNTPReply(good(), sent); err != nil {
		t.Fatalf("a good reply was refused: %v", err)
	}
	for name, tc := range map[string]struct {
		edit func(b []byte) []byte
		want string
	}{
		"too short":            {func(b []byte) []byte { return b[:47] }, "short reply"},
		"client mode":          {func(b []byte) []byte { b[0] = 4<<3 | 3; return b }, "not a server reply"},
		"broadcast mode":       {func(b []byte) []byte { b[0] = 4<<3 | 5; return b }, "not a server reply"},
		"version 0":            {func(b []byte) []byte { b[0] = 0<<3 | 4; return b }, "unsupported NTP version"},
		"version 5":            {func(b []byte) []byte { b[0] = 5<<3 | 4; return b }, "unsupported NTP version"},
		"someone else's reply": {func(b []byte) []byte { b[31] ^= 1; return b }, "does not match the request"},
		"kiss-o'-death RATE":   {func(b []byte) []byte { b[1] = 0; copy(b[12:16], "RATE"); return b }, "RATE (asked to slow down)"},
		"kiss-o'-death DENY":   {func(b []byte) []byte { b[1] = 0; copy(b[12:16], "DENY"); return b }, "access denied"},
		"unsynchronized leap":  {func(b []byte) []byte { b[0] |= 3 << 6; return b }, "unsynchronized"},
		"stratum 16":           {func(b []byte) []byte { b[1] = 16; return b }, "stratum is 16"},
		"no transmit time":     {func(b []byte) []byte { copy(b[40:48], make([]byte, 8)); return b }, "no transmit time"},
	} {
		if _, err := parseNTPReply(tc.edit(good()), sent); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want one containing %q", name, err, tc.want)
		}
	}
}

func TestReferenceIDs(t *testing.T) {
	if got := referenceID(1, []byte{'G', 'P', 'S', 0}); got != "GPS" {
		t.Errorf("stratum 1 reference = %q, want GPS", got)
	}
	if got := referenceID(2, []byte{192, 0, 2, 7}); got != "192.0.2.7" {
		t.Errorf("stratum 2 reference = %q, want 192.0.2.7", got)
	}
}

func TestQueryNTPMeasuresTheOffset(t *testing.T) {
	for name, offset := range map[string]time.Duration{"server ahead by 2s": 2 * time.Second, "server behind by 1.5s": -1500 * time.Millisecond, "in step": 0} {
		f := newFake()
		f.offset = offset
		port := f.start(t, "udp4", "127.0.0.1")
		ex, err := queryNTP("127.0.0.1", port, 2)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if d := ex.offset - offset; d < -50*time.Millisecond || d > 50*time.Millisecond {
			t.Errorf("%s: offset %v, want about %v", name, ex.offset, offset)
		}
		if ex.roundTrip < -time.Millisecond || ex.roundTrip > 100*time.Millisecond {
			t.Errorf("%s: round trip %v on loopback", name, ex.roundTrip)
		}
		if ex.reply.stratum != 2 || ex.reply.referenceID != "192.0.2.1" || ex.reply.version != 4 {
			t.Errorf("%s: reply = %+v", name, ex.reply)
		}
	}
}

// The time the server spends between receiving the request (T2) and replying
// (T3) is not network delay and must not be counted as such - that is what the
// (T3 - T2) term is for. Nor may it disturb the offset.
func TestQueryNTPSubtractsTheServersProcessingTime(t *testing.T) {
	f := newFake()
	f.offset = 3 * time.Second
	f.hold = 250 * time.Millisecond
	port := f.start(t, "udp4", "127.0.0.1")
	ex, err := queryNTP("127.0.0.1", port, 3)
	if err != nil {
		t.Fatal(err)
	}
	if ex.roundTrip > 100*time.Millisecond {
		t.Errorf("round trip %v includes the server's 250ms of processing", ex.roundTrip)
	}
	if d := ex.offset - 3*time.Second; d < -50*time.Millisecond || d > 50*time.Millisecond {
		t.Errorf("offset %v, want about 3s", ex.offset)
	}
}

func run(t *testing.T, json bool, port int, maxOffset time.Duration, timeout int, host string) (string, bool) {
	t.Helper()
	throttle := false
	var ok bool
	out := captureStdout(t, func() { ok = NTPHandler(&json, 1, 0, &throttle, timeout, port, maxOffset, host) })
	return out, ok
}

func TestNTPHandlerTextOutput(t *testing.T) {
	f := newFake()
	f.offset = 2 * time.Second
	port := f.start(t, "udp4", "127.0.0.1")
	out, ok := run(t, false, port, 0, 2, "127.0.0.1")
	if !ok {
		t.Errorf("reported failure:\n%s", out)
	}
	for _, want := range []string{"[ntp] OK dns resolved host=127.0.0.1", "[ntp] OK response server=127.0.0.1 address=127.0.0.1 attempt=1/1 stratum=2 offset=+1.9", "leap=none version=4 reference=192.0.2.1", "[ntp] OK done queries=1 answered=1"} {
		if !strings.Contains(out, want) && !strings.Contains(out, strings.Replace(want, "offset=+1.9", "offset=+2.0", 1)) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestNTPHandlerJSONIsOneCleanDocument(t *testing.T) {
	f := newFake()
	f.offset = -time.Second
	port := f.start(t, "udp4", "127.0.0.1")
	out, ok := run(t, true, port, 0, 2, "127.0.0.1")
	if !ok {
		t.Errorf("reported failure:\n%s", out)
	}
	var doc struct {
		ModuleName string         `json:"module_name"`
		DNS        lib.DNSLookup  `json:"dns_lookup"`
		Stats      []lib.NTPStats `json:"stats"`
		Error      string         `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	if doc.ModuleName != "ntp" || !doc.DNS.Success || len(doc.Stats) != 1 {
		t.Fatalf("unexpected document: %+v", doc)
	}
	s := doc.Stats[0]
	if !s.Success || s.Stratum != 2 || s.Version != 4 || s.LeapIndicator != "none" || s.ReferenceID != "192.0.2.1" || s.Error != "" {
		t.Errorf("stat = %+v", s)
	}
	if s.OffsetUs > -900_000 || s.OffsetUs < -1_100_000 {
		t.Errorf("offset_µs = %d, want about -1000000", s.OffsetUs)
	}
	if s.ServerTimeUs == 0 || s.SentTime == 0 || s.RecvTime < s.SentTime {
		t.Errorf("times: %+v", s)
	}
}

func TestNTPHandlerReportsUntrustworthyAndMissingReplies(t *testing.T) {
	kod := newFake()
	kod.stratum = 0
	kod.refid = [4]byte{'R', 'A', 'T', 'E'}
	kodPort := kod.start(t, "udp4", "127.0.0.1")
	if out, ok := run(t, false, kodPort, 0, 2, "127.0.0.1"); ok || !strings.Contains(out, "[ntp] ERROR query failed") || !strings.Contains(out, "kiss-o'-death") {
		t.Errorf("a kiss-o'-death must fail the check (ok=%v):\n%s", ok, out)
	}

	spoofed := newFake()
	spoofed.mangle = func(reply, _ []byte) { reply[25] ^= 0xff }
	spoofedPort := spoofed.start(t, "udp4", "127.0.0.1")
	if out, ok := run(t, false, spoofedPort, 0, 2, "127.0.0.1"); ok || !strings.Contains(out, "does not match the request") {
		t.Errorf("a reply to a different request must fail (ok=%v):\n%s", ok, out)
	}

	silent := newFake()
	silent.silent = true
	silentPort := silent.start(t, "udp4", "127.0.0.1")
	start := time.Now()
	out, ok := run(t, false, silentPort, 0, 1, "127.0.0.1")
	if ok || !strings.Contains(out, "[ntp] ERROR query failed") || !strings.Contains(out, "timeout") {
		t.Errorf("no reply must fail with a timeout (ok=%v):\n%s", ok, out)
	}
	if took := time.Since(start); took < 900*time.Millisecond || took > 3*time.Second {
		t.Errorf("--timeout 1 took %v", took)
	}
}

func TestNTPHandlerMaxOffset(t *testing.T) {
	f := newFake()
	f.offset = 2 * time.Second
	port := f.start(t, "udp4", "127.0.0.1")
	if out, ok := run(t, false, port, 500*time.Millisecond, 2, "127.0.0.1"); ok || !strings.Contains(out, "is larger than --max-offset 500ms") {
		t.Errorf("an offset beyond --max-offset must fail (ok=%v):\n%s", ok, out)
	}
	if out, ok := run(t, false, port, 5*time.Second, 2, "127.0.0.1"); !ok {
		t.Errorf("an offset within --max-offset must pass:\n%s", out)
	}
	behind := newFake() // a clock that is behind counts too
	behind.offset = -2 * time.Second
	behindPort := behind.start(t, "udp4", "127.0.0.1")
	if out, ok := run(t, false, behindPort, 500*time.Millisecond, 2, "127.0.0.1"); ok {
		t.Errorf("a negative offset beyond --max-offset must fail:\n%s", out)
	}
}

func TestNTPHandlerDNSFailure(t *testing.T) {
	out, ok := run(t, false, 123, 0, 2, "no-such-host.invalid")
	if ok || !strings.Contains(out, "[ntp] ERROR dns resolution failed") {
		t.Errorf("ok=%v:\n%s", ok, out)
	}
	out, ok = run(t, true, 123, 0, 2, "no-such-host.invalid")
	var doc struct {
		Error string        `json:"error"`
		DNS   lib.DNSLookup `json:"dns_lookup"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil || ok || doc.Error == "" || doc.DNS.Success {
		t.Errorf("a failed lookup under --json must be one JSON document with the error (ok=%v, err=%v):\n%s", ok, err, out)
	}
}

func TestNTPHandlerIPv6(t *testing.T) {
	f := newFake()
	port := f.start(t, "udp6", "::1")
	if out, ok := run(t, false, port, 0, 2, "::1"); !ok || !strings.Contains(out, "address=::1") {
		t.Errorf("ok=%v:\n%s", ok, out)
	}
}
