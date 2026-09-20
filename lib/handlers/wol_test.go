package handlers

import (
	"bytes"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

func TestParseMAC(t *testing.T) {
	want := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	for _, in := range []string{"aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff", "aabb.ccdd.eeff", "aabbccddeeff", "  aa:bb:cc:dd:ee:ff  "} {
		got, err := ParseMAC(in)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("ParseMAC(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "aa:bb:cc:dd:ee", "aa:bb:cc:dd:ee:ff:00", "gg:bb:cc:dd:ee:ff", "aabbccddeef", "aabbccddeeff00",
		"00:00:5e:00:53:01:02:03", "not a mac", "aa:bb:cc:dd:ee:ff:", "192.168.1.1"} {
		if got, err := ParseMAC(bad); err == nil {
			t.Errorf("ParseMAC(%q) accepted it: %v", bad, got)
		}
	}
}

func TestParseBroadcast(t *testing.T) {
	for _, ok := range []string{"255.255.255.255", "192.168.1.255", "127.0.0.1", " 10.0.0.255 "} {
		if _, err := ParseBroadcast(ok); err != nil {
			t.Errorf("ParseBroadcast(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"", "::1", "ff02::1", "0.0.0.0", "224.0.0.1", "192.168.1", "example.com", "300.1.1.1", "::ffff:1.2.3.4"} {
		if _, err := ParseBroadcast(bad); err == nil {
			t.Errorf("ParseBroadcast(%q) accepted it", bad)
		}
	}
}

// The layout is fixed by the Wake-on-LAN specification (AMD Magic Packet
// Technology white paper): six bytes of 0xFF, then the MAC sixteen times.
func TestMagicPacketLayout(t *testing.T) {
	mac := net.HardwareAddr{0x01, 0x23, 0x45, 0x67, 0x89, 0xab}
	p := MagicPacket(mac)
	if len(p) != 102 {
		t.Fatalf("length %d, want 102", len(p))
	}
	if !bytes.Equal(p[:6], bytes.Repeat([]byte{0xff}, 6)) {
		t.Errorf("does not start with six 0xFF bytes: % x", p[:6])
	}
	for i := 0; i < 16; i++ {
		if got := p[6+i*6 : 12+i*6]; !bytes.Equal(got, mac) {
			t.Errorf("repetition %d is % x, want % x", i+1, got, mac)
		}
	}
}

// receiveUDP listens on 127.0.0.1 and collects every datagram until closed.
func receiveUDP(t *testing.T) (port int, packets func() [][]byte) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	got := make(chan []byte, 16)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			got <- append([]byte(nil), buf[:n]...)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port, func() [][]byte {
		var all [][]byte
		for {
			select {
			case p := <-got:
				all = append(all, p)
			case <-time.After(300 * time.Millisecond):
				return all
			}
		}
	}
}

func TestWOLHandlerSendsTheMagicPacket(t *testing.T) {
	port, packets := receiveUDP(t)
	mac, _ := ParseMAC("aa:bb:cc:dd:ee:ff")
	jsonOutput, throttle := false, false
	var ok bool
	out := captureStdout(t, func() {
		ok = WOLHandler(&jsonOutput, 3, 0, &throttle, 2, mac, netip.MustParseAddr("127.0.0.1"), port)
	})
	if !ok {
		t.Errorf("WOLHandler reported failure:\n%s", out)
	}
	got := packets()
	if len(got) != 3 {
		t.Fatalf("the listener received %d packets, want 3", len(got))
	}
	for i, p := range got {
		if !bytes.Equal(p, MagicPacket(mac)) {
			t.Errorf("packet %d is not the magic packet for %s: % x", i+1, mac, p)
		}
	}
	for _, want := range []string{"[wol] OK magic packet sent mac=aa:bb:cc:dd:ee:ff to=127.0.0.1:", "bytes=102 attempt=1/3", "attempt=3/3", "[wol] OK done packets_sent=3"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestWOLHandlerJSONIsOneCleanDocument(t *testing.T) {
	port, packets := receiveUDP(t)
	mac, _ := ParseMAC("aabbccddeeff")
	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		WOLHandler(&jsonOutput, 2, 0, &throttle, 2, mac, netip.MustParseAddr("127.0.0.1"), port)
	})
	packets()
	var doc struct {
		ModuleName string          `json:"module_name"`
		Stats      []lib.WOLStats  `json:"stats"`
		DNS        map[string]any  `json:"dns_lookup"`
		Input      lib.InputParams `json:"input_params"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, out)
	}
	if doc.ModuleName != "wol" || len(doc.Stats) != 2 || doc.DNS != nil {
		t.Errorf("unexpected document: %+v", doc)
	}
	for _, s := range doc.Stats {
		if !s.Success || s.BytesSent != 102 || s.MAC != "aa:bb:cc:dd:ee:ff" || s.Port != port || s.Error != "" {
			t.Errorf("bad stat: %+v", s)
		}
	}
	if doc.Input.Mode != "wol" || doc.Input.Payload != 102 || doc.Input.Protocol != "udp" || doc.Input.Count != 2 {
		t.Errorf("input_params = %+v", doc.Input)
	}
}

// A send that fails is an ERROR line and a false result, and the run goes on.
// Port 0 is not a destination anyone can send to.
func TestWOLHandlerReportsAFailedSend(t *testing.T) {
	mac, _ := ParseMAC("aa:bb:cc:dd:ee:ff")
	jsonOutput, throttle := false, false
	var ok bool
	out := captureStdout(t, func() {
		ok = WOLHandler(&jsonOutput, 2, 0, &throttle, 2, mac, netip.MustParseAddr("127.0.0.1"), 0)
	})
	if strings.Contains(out, "magic packet sent") {
		t.Skip("this platform accepts a send to port 0")
	}
	if ok {
		t.Error("a failed send must report false")
	}
	for _, want := range []string{"[wol] ERROR send failed", "attempt=1/2", "attempt=2/2", "[wol] OK done packets_sent=0"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
