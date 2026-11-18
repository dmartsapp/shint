package wol

import (
	"encoding/json"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestWOLHandlerInterruptCutsTheDelayShort(t *testing.T) {
	port, packets := receiveUDP(t)
	mac, _ := ParseMAC("aa:bb:cc:dd:ee:ff")
	jsonOutput, throttle := false, false
	var ok bool
	ctx := testutil.CancelAfter(t, 250*time.Millisecond)
	out := testutil.RunWithin(t, 1500*time.Millisecond, func() {
		ok = Run(ctx, &jsonOutput, 5, 30000, &throttle, 2, mac, netip.MustParseAddr("127.0.0.1"), port)
	})
	if ok || len(packets()) != 0 {
		t.Errorf("ok=%v; nothing should have been sent during the first 30 s delay", ok)
	}
	completed, planned := testutil.InterruptedCounts(t, out)
	if completed != 0 || planned != 5 {
		t.Errorf("completed %d of %d, want 0 of 5", completed, planned)
	}
	if !strings.Contains(out, "[wol] OK done packets_sent=0") {
		t.Errorf("the done line still follows:\n%s", out)
	}
}

func TestWOLHandlerInterruptedJSON(t *testing.T) {
	port, packets := receiveUDP(t)
	mac, _ := ParseMAC("aa:bb:cc:dd:ee:ff")
	jsonOutput, throttle := true, false
	ctx := testutil.CancelAfter(t, 400*time.Millisecond)
	var ok bool
	out := testutil.RunWithin(t, 3*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 200, 40, &throttle, 2, mac, netip.MustParseAddr("127.0.0.1"), port)
	})
	got := len(packets())
	var doc struct {
		Error string        `json:"error"`
		Stats []interface{} `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`^interrupted: (\d+) of 200 attempts completed$`).FindStringSubmatch(doc.Error)
	if ok || m == nil {
		t.Fatalf("ok=%v, error=%q", ok, doc.Error)
	}
	if n, _ := strconv.Atoi(m[1]); n != len(doc.Stats) || n != got || n == 0 {
		t.Errorf("error says %s, stats has %d, the receiver got %d", m[1], len(doc.Stats), got)
	}
}
