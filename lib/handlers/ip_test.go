package handlers

import (
	"encoding/json"
	"errors"
	"net"
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

func TestIPListsEveryInterfaceAndAddress(t *testing.T) {
	fakeInterfaces(t, sampleInterfaces, nil)
	jsonOut := false
	var ok bool
	out := captureStdout(t, func() { ok = IPHandler(&jsonOut, "") })
	if !ok {
		t.Errorf("IPHandler = false; output:\n%s", out)
	}
	for _, want := range []string{
		"[ip] OK interface name=lo0 index=1 state=up flags=[loopback,multicast]",
		"mtu=16384 addresses=2",
		"name=en0 index=15 state=up flags=[broadcast,multicast] mtu=1500 mac=aa:bb:cc:dd:ee:ff addresses=4",
		"name=utun3 index=20 state=down flags=[pointtopoint,multicast] mtu=1380 addresses=0",
		"address interface=lo0 address=127.0.0.1 prefix=127.0.0.1/8 family=ipv4 kind=loopback",
		"address interface=lo0 address=::1 prefix=::1/128 family=ipv6 kind=loopback",
		"address interface=en0 address=192.168.1.20 prefix=192.168.1.20/24 family=ipv4 kind=private",
		"address=fe80::1c2d:3e4f:5a6b:7c8d prefix=fe80::1c2d:3e4f:5a6b:7c8d/64 family=ipv6 kind=link-local",
		"address=2001:db8::20 prefix=2001:db8::20/64 family=ipv6 kind=documentation",
		"address=203.0.113.9 prefix=203.0.113.9/32 family=ipv4 kind=documentation",
		"[ip] OK done interfaces=3 addresses=6",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
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
	if strings.Contains(out, "family=ipv6") || !strings.Contains(out, "family=ipv4") || !strings.Contains(out, "done interfaces=3 addresses=3") {
		t.Errorf("-4 should list IPv4 addresses only (3):\n%s", out)
	}
	useFamily(t, false, true)
	out = captureStdout(t, func() { IPHandler(&jsonOut, "") })
	if strings.Contains(out, "family=ipv4") || !strings.Contains(out, "done interfaces=3 addresses=3") {
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
	if en0.Name != "en0" || en0.MAC != "aa:bb:cc:dd:ee:ff" || en0.State != "up" || len(en0.Addresses) != 4 || en0.Addresses[0].PrefixLength != 24 {
		t.Errorf("en0 = %+v", en0)
	}
	if doc.Stats[2].Addresses == nil || len(doc.Stats[2].Addresses) != 0 {
		t.Errorf("an interface with no addresses must have an empty list, not null: %+v", doc.Stats[2])
	}
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
// interface, and every address it reports is a valid prefix with a known kind.
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
		for _, a := range st.Addresses {
			if a.Kind == "" || a.PrefixLength <= 0 || !strings.Contains(a.Prefix, "/") {
				t.Errorf("interface %s: odd address %+v", st.Name, a)
			}
		}
	}
}
