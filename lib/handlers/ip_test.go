package handlers

import (
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib"
)

func ipnet(cidr string) net.Addr {
	ip, n, _ := net.ParseCIDR(cidr)
	n.IP = ip // the way Interface.Addrs reports it: the host address with the network's mask
	return n
}

// fakeInterfaces stands in for the operating system's table.
func fakeInterfaces(t *testing.T, list []ifaceInfo, err error) {
	t.Helper()
	old := systemInterfaces
	systemInterfaces = func() ([]ifaceInfo, error) { return list, err }
	t.Cleanup(func() { systemInterfaces = old })
}

func useFamily(t *testing.T, v4, v6 bool) {
	t.Helper()
	if err := lib.SetIPFamily(v4, v6); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.SetIPFamily(false, false) })
}

var sampleInterfaces = []ifaceInfo{
	{Name: "lo0", Index: 1, MTU: 16384, Flags: net.FlagUp | net.FlagLoopback | net.FlagMulticast, Addrs: []net.Addr{ipnet("127.0.0.1/8"), ipnet("::1/128")}},
	{Name: "en0", Index: 15, MTU: 1500, MAC: "aa:bb:cc:dd:ee:ff", Flags: net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
		Addrs: []net.Addr{ipnet("192.168.1.20/24"), ipnet("fe80::1c2d:3e4f:5a6b:7c8d/64"), ipnet("2001:db8::20/64"), ipnet("203.0.113.9/32")}},
	{Name: "utun3", Index: 20, MTU: 1380, Flags: net.FlagPointToPoint | net.FlagMulticast},
}

func TestIPShowsEverythingAboutAnInterfaceOnOneLine(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	jsonOut := false
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "") })
	if !ok {
		t.Errorf("IPHandler = false; output:\n%s", out)
	}
	for _, want := range []string{
		"[ip] OK interface name=lo0 state=up ipv4=[127.0.0.1/8] ipv6=[::1/128] mtu=16384 flags=[loopback,multicast]\n",
		"[ip] OK interface name=en0 state=up ipv4=[192.168.1.20/24,203.0.113.9/32] ipv6=[fe80::1c2d:3e4f:5a6b:7c8d/64,2001:db8::20/64] mac=aa:bb:cc:dd:ee:ff mtu=1500 flags=[broadcast,multicast]\n",
		"[ip] OK interface name=utun3 state=down mtu=1380 flags=[pointtopoint,multicast]\n", // no address: no ipv4 or ipv6 field
		"[ip] OK done interfaces=3 addresses=6",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	// one line per interface, and nothing else but the summary: no separate address lines
	if lines := strings.Count(out, "\n"); lines != 4 {
		t.Errorf("want 4 lines (3 interfaces and the summary), got %d:\n%s", lines, out)
	}
	if strings.Contains(out, "[ip] OK address ") || strings.Contains(out, "index=") || strings.Contains(out, "kind=") {
		t.Errorf("the old layout is still there:\n%s", out)
	}
	if strings.Contains(out, "ERROR") {
		t.Errorf("a clean listing printed an ERROR line:\n%s", out)
	}
	if strings.Index(out, "name=lo0") > strings.Index(out, "name=en0") || strings.Index(out, "name=en0") > strings.Index(out, "name=utun3") {
		t.Errorf("interfaces are not in the order the system gave them:\n%s", out)
	}
}

func TestIPOneInterfaceByNameIgnoringCase(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	jsonOut := false
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "EN0") })
	if !ok || !strings.Contains(out, "name=en0") || strings.Contains(out, "name=lo0") || !strings.Contains(out, "done interfaces=1 addresses=4") {
		t.Errorf("ok=%v; want only en0:\n%s", ok, out)
	}
}

func TestIPUnknownInterfaceIsAFailedCheckNamingTheOnesThatExist(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	jsonOut := false
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "wlan9") })
	if ok {
		t.Error("IPHandler = true for an interface that does not exist")
	}
	if !strings.Contains(out, "[ip] ERROR failed error=") || !strings.Contains(out, "no such interface wlan9 (available: lo0, en0, utun3)") {
		t.Errorf("output:\n%s", out)
	}
}

func TestIPFamilyChoiceFiltersAddresses(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	useFamily(t, true, false)
	jsonOut := false
	out := captureStdout(t, func() { IPHandler(&jsonOut, "") })
	if strings.Contains(out, "ipv6=") || !strings.Contains(out, "ipv4=[192.168.1.20/24,203.0.113.9/32]") || !strings.Contains(out, "done interfaces=3 addresses=3") {
		t.Errorf("-4 should list IPv4 addresses only (3):\n%s", out)
	}
	useFamily(t, false, true)
	out = captureStdout(t, func() { IPHandler(&jsonOut, "") })
	if strings.Contains(out, "ipv4=") || !strings.Contains(out, "ipv6=[::1/128]") || !strings.Contains(out, "done interfaces=3 addresses=3") {
		t.Errorf("-6 should list IPv6 addresses only (3):\n%s", out)
	}
}

func TestIPJSONIsOneDocumentWithTheUsualSkeleton(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	jsonOut := true
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "") })
	if !ok {
		t.Error("IPHandler = false")
	}
	var doc struct {
		ModuleName string          `json:"module_name"`
		DNSLookup  json.RawMessage `json:"dns_lookup"`
		Error      string          `json:"error"`
		Stats      []lib.IPStats   `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if doc.ModuleName != "ip" || doc.DNSLookup != nil || doc.Error != "" || len(doc.Stats) != 3 {
		t.Errorf("module=%q dns_lookup=%s error=%q stats=%d", doc.ModuleName, doc.DNSLookup, doc.Error, len(doc.Stats))
	}
	en0 := doc.Stats[1]
	if en0.Name != "en0" || en0.MAC != "aa:bb:cc:dd:ee:ff" || en0.State != "up" || en0.MTU != 1500 ||
		strings.Join(en0.IPv4, " ") != "192.168.1.20/24 203.0.113.9/32" || strings.Join(en0.IPv6, " ") != "fe80::1c2d:3e4f:5a6b:7c8d/64 2001:db8::20/64" {
		t.Errorf("en0 = %+v", en0)
	}
	if doc.Stats[2].IPv4 == nil || doc.Stats[2].IPv6 == nil || len(doc.Stats[2].IPv4)+len(doc.Stats[2].IPv6) != 0 {
		t.Errorf("an interface with no addresses must have empty lists, not null: %+v", doc.Stats[2])
	}
}

// Everything about an interface is in its own entry: the JSON has one object per interface,
// and an interface's addresses are plain strings in it, with nothing nested below them.
func TestIPJSONEntryIsFlat(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	jsonOut := true
	out := captureStdout(t, func() { IPHandler(&jsonOut, "en0") })
	var doc struct {
		Stats []map[string]any `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil || len(doc.Stats) != 1 {
		t.Fatalf("%v\n%s", err, out)
	}
	keys := make([]string, 0, len(doc.Stats[0]))
	for k, v := range doc.Stats[0] {
		keys = append(keys, k)
		if list, ok := v.([]any); ok {
			for _, item := range list {
				if _, isString := item.(string); !isString {
					t.Errorf("%s holds a %T, want strings only", k, item)
				}
			}
		}
	}
	if want := "flags ipv4 ipv6 mac mtu name state"; strings.Join(sortedStrings(keys), " ") != want {
		t.Errorf("keys = %v, want %s", sortedStrings(keys), want)
	}
}

func sortedStrings(in []string) []string {
	out := append([]string{}, in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestIPJSONFailureIsStillOneDocument(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	jsonOut := true
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "nope") })
	var doc struct {
		Error string        `json:"error"`
		Stats []lib.IPStats `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if ok || !strings.Contains(doc.Error, "no such interface nope") || doc.Stats == nil || len(doc.Stats) != 0 {
		t.Errorf("ok=%v error=%q stats=%v", ok, doc.Error, doc.Stats)
	}
}

func TestIPAnInterfaceWhoseAddressesCannotBeReadFailsTheRun(t *testing.T) {
	list := append([]ifaceInfo{}, sampleInterfaces...)
	list[1].AddrsErr = errors.New("permission denied")
	list[1].Addrs = nil
	fakeInterfaces(t, list, nil)
	jsonOut := false
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "") })
	if ok {
		t.Error("IPHandler = true although one interface's addresses could not be read")
	}
	if !strings.Contains(out, "[ip] ERROR interface name=en0") || !strings.Contains(out, "error=") {
		t.Errorf("the unreadable interface is not reported as an ERROR:\n%s", out)
	}
	if !strings.Contains(out, "name=lo0") || !strings.Contains(out, "done interfaces=3") {
		t.Errorf("the other interfaces should still be listed:\n%s", out)
	}
}

func TestIPTableErrorAndEmptyTable(t *testing.T) {
	jsonOut := false
	fakeInterfaces(t, nil, errors.New("boom"))
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "") })
	if ok || !strings.Contains(out, "ERROR failed error=") || !strings.Contains(out, "could not read the interface table: boom") {
		t.Errorf("ok=%v:\n%s", ok, out)
	}
	fakeInterfaces(t, nil, nil)
	out = captureStdout(t, func() { ok = IPHandler(&jsonOut, "") })
	if ok || !strings.Contains(out, "reports no network interfaces") {
		t.Errorf("ok=%v:\n%s", ok, out)
	}
}

// The real interface table, once: whatever machine this runs on has at least one
// interface, and every address it reports is a valid prefix of the family it is listed under.
func TestIPRealInterfaces(t *testing.T) {
	jsonOut := true
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "") })
	var doc struct {
		Stats []lib.IPStats `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if len(doc.Stats) == 0 {
		t.Skip("this environment reports no network interfaces")
	}
	if !ok {
		t.Errorf("IPHandler = false on the real interface table:\n%s", out)
	}
	for _, st := range doc.Stats {
		for _, a := range st.IPv4 {
			if p, err := netip.ParsePrefix(a); err != nil || !p.Addr().Is4() {
				t.Errorf("interface %s: %q is not an IPv4 prefix (%v)", st.Name, a, err)
			}
		}
		for _, a := range st.IPv6 {
			if p, err := netip.ParsePrefix(a); err != nil || !p.Addr().Is6() {
				t.Errorf("interface %s: %q is not an IPv6 prefix (%v)", st.Name, a, err)
			}
		}
	}
}
