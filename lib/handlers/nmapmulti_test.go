package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib"
)

// openOn is a probe that finds open exactly the address:port pairs given, and
// every other port refused, without touching the network.
func openOn(t *testing.T, open ...string) {
	t.Helper()
	set := make(map[string]bool)
	for _, o := range open {
		set[o] = true
	}
	withProbe(t, func(ctx context.Context, host string, port, timeout int) (bool, error) {
		if set[host+":"+itoa(port)] {
			return true, nil
		}
		return false, errors.New("connection refused")
	})
}

func nmapRun(t *testing.T, scan NmapScan, json bool) (string, bool) {
	t.Helper()
	var ok bool
	out := captureStdout(t, func() {
		ok = NmapHandler(context.Background(), scan, 1, 1, false, &json)
	})
	return out, ok
}

func targetsOf(t *testing.T, inputs ...string) []NmapTarget {
	t.Helper()
	targets, err := ParseNmapTargets(inputs)
	if err != nil {
		t.Fatal(err)
	}
	return targets
}

func TestNmapPortListAndServiceNames(t *testing.T) {
	openOn(t, "127.0.0.1:22", "127.0.0.1:2222")
	ports, _ := lib.ParsePorts("ports", "2222,22,80")
	scan := NmapScan{Targets: targetsOf(t, "127.0.0.1"), Ports: ports, PortList: true}

	out, ok := nmapRun(t, scan, false)
	if !ok || !strings.Contains(out, "port open host=127.0.0.1 port=22 service=ssh\n") || !strings.Contains(out, "port open host=127.0.0.1 port=2222\n") ||
		strings.Contains(out, "port=80") || !strings.Contains(out, "scan started ports_total=3 timeout=") || !strings.Contains(out, "scan complete ports_scanned=3 open=2 time=") {
		t.Errorf("text (ok %v):\n%s", ok, out)
	}
	if strings.Contains(out, "hosts=") || strings.Contains(out, "hosts_with_open_ports") {
		t.Errorf("one address: no host counts expected:\n%s", out)
	}

	out, ok = nmapRun(t, scan, true)
	stats, doc := nmapStatsFromJSON(t, out)
	p := doc.InputParams
	if !ok || p.Ports != "22,80,2222" || p.FromPort != 22 || p.ToPort != 2222 || p.Host != "127.0.0.1" || p.Targets != nil || doc.Error != "" {
		t.Errorf("input_params %+v, error %q", p, doc.Error)
	}
	byPort := map[int]lib.NmapStats{}
	for _, s := range stats {
		byPort[s.Port] = s
	}
	if len(stats) != 3 || byPort[22].Service != "ssh" || byPort[80].Service != "http" || byPort[2222].Service != "" ||
		!byPort[22].Success || byPort[80].Success || byPort[22].Target != "127.0.0.1" {
		t.Errorf("stats %+v", stats)
	}
	if strings.Contains(out, `"targets"`) || strings.Contains(out, `"dns_lookups"`) || doc.DNSLookup == nil || doc.DNSLookup.Hostname != "127.0.0.1" {
		t.Errorf("one target: dns_lookup, and no targets or dns_lookups, expected:\n%s", out)
	}
}

func TestNmapRangeHasNoPortList(t *testing.T) {
	openOn(t)
	out, _ := nmapRun(t, rangeScan("127.0.0.1", 20, 22), true)
	_, doc := nmapStatsFromJSON(t, out)
	if doc.InputParams.Ports != "" || doc.InputParams.FromPort != 20 || doc.InputParams.ToPort != 22 || strings.Contains(out, `"ports"`) {
		t.Errorf("a --from/--to range: input_params %+v", doc.InputParams)
	}
}

// Several targets: a prefix, an address it already covers, and another address.
func TestNmapSeveralTargets(t *testing.T) {
	openOn(t, "192.0.2.2:22", "198.51.100.7:443")
	scan := NmapScan{Targets: targetsOf(t, "192.0.2.5/30", "192.0.2.5", "198.51.100.7"), Ports: []int{22, 443}, PortList: true}

	out, ok := nmapRun(t, scan, false)
	for _, want := range []string{
		"sweep target=192.0.2.5/30 network=192.0.2.4/30 addresses=2\n",
		"dns resolved host=192.0.2.5 addresses=1 ips=[192.0.2.5]",
		"dns resolved host=198.51.100.7 addresses=1",
		"scan started ports_total=6 hosts=3 timeout=",
		"port open host=198.51.100.7 port=443 service=https\n",
		"scan complete ports_scanned=6 open=1 hosts_with_open_ports=1 time=",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text: want %q in:\n%s", want, out)
		}
	}
	if !ok || strings.Contains(out, "port=22") {
		t.Errorf("192.0.2.2 is not in 192.0.2.4/30 (ok %v):\n%s", ok, out)
	}

	out, ok = nmapRun(t, scan, true)
	stats, doc := nmapStatsFromJSON(t, out)
	p := doc.InputParams
	if !ok || p.Host != "" || strings.Join(p.Targets, " ") != "192.0.2.5/30 192.0.2.5 198.51.100.7" || doc.DNSLookup != nil {
		t.Errorf("input_params %+v, dns_lookup %+v", p, doc.DNSLookup)
	}
	if len(doc.DNSLookups) != 2 || doc.DNSLookups[0].Hostname != "192.0.2.5" || !doc.DNSLookups[1].Success {
		t.Errorf("dns_lookups %+v", doc.DNSLookups)
	}
	// 192.0.2.5 is scanned once, for the prefix that led to it first.
	targets := map[string]string{}
	for _, s := range stats {
		if prev, seen := targets[s.Address+itoa(s.Port)]; seen {
			t.Errorf("%s:%d scanned twice (%s, %s)", s.Address, s.Port, prev, s.Target)
		}
		targets[s.Address+itoa(s.Port)] = s.Target
	}
	if len(stats) != 6 || targets["192.0.2.5"+"22"] != "192.0.2.5/30" || targets["198.51.100.7"+"443"] != "198.51.100.7" {
		t.Errorf("stats %+v", stats)
	}
}

func TestNmapOneFailedLookupOfSeveral(t *testing.T) {
	openOn(t, "192.0.2.1:22")
	scan := NmapScan{Targets: targetsOf(t, "this-host-should-not-exist.invalid", "192.0.2.1"), Ports: []int{22}}

	out, ok := nmapRun(t, scan, false)
	if ok || !strings.Contains(out, "ERROR dns resolution failed host=this-host-should-not-exist.invalid") || !strings.Contains(out, "port open host=192.0.2.1 port=22 service=ssh") ||
		!strings.Contains(out, "scan complete ports_scanned=1 open=1 time=") {
		t.Errorf("want the other target scanned and the run failed (ok %v):\n%s", ok, out)
	}

	out, ok = nmapRun(t, scan, true)
	stats, doc := nmapStatsFromJSON(t, out)
	if ok || len(stats) != 1 || !strings.HasPrefix(doc.Error, "dns resolution failed for this-host-should-not-exist.invalid: ") ||
		len(doc.DNSLookups) != 2 || doc.DNSLookups[0].Success || doc.DNSLookups[0].Error == "" || !doc.DNSLookups[1].Success {
		t.Errorf("ok %v, error %q, dns_lookups %+v, stats %+v", ok, doc.Error, doc.DNSLookups, stats)
	}
}

func TestNmapEveryLookupFails(t *testing.T) {
	openOn(t)
	scan := NmapScan{Targets: targetsOf(t, "a.invalid", "b.invalid"), Ports: []int{22}}
	out, ok := nmapRun(t, scan, false)
	if ok || strings.Count(out, "dns resolution failed") != 2 || strings.Contains(out, "scan started") {
		t.Errorf("want two failed lookups and no scan (ok %v):\n%s", ok, out)
	}
}

func TestNmapSweepOfAPrefix(t *testing.T) {
	openOn(t, "10.9.8.7:22", "10.9.8.200:22")
	scan := NmapScan{Targets: targetsOf(t, "10.9.8.0/24"), Ports: []int{22}}
	out, ok := nmapRun(t, scan, false)
	if !ok || !strings.Contains(out, "sweep target=10.9.8.0/24 addresses=254\n") || strings.Contains(out, "network=") ||
		strings.Contains(out, "dns resolved") || !strings.Contains(out, "scan complete ports_scanned=254 open=2 hosts_with_open_ports=2 time=") {
		t.Errorf("ok %v:\n%s", ok, out)
	}
	out, _ = nmapRun(t, scan, true)
	stats, doc := nmapStatsFromJSON(t, out)
	if strings.Contains(out, `"dns_lookup`) {
		t.Errorf("a prefix has no lookup:\n%.400s", out)
	}
	if len(stats) != 254 || doc.InputParams.Host != "10.9.8.0/24" || doc.DNSLookup != nil || doc.DNSLookups != nil {
		t.Errorf("%d stats, input_params %+v, dns_lookup %+v", len(stats), doc.InputParams, doc.DNSLookup)
	}
}

// One target whose lookup fails: dns_lookup says which name and why, and the
// run-level error is the lookup's.
func TestNmapSingleFailedLookupJSON(t *testing.T) {
	openOn(t)
	out, ok := nmapRun(t, NmapScan{Targets: targetsOf(t, "this-host-should-not-exist.invalid"), Ports: []int{22}}, true)
	stats, doc := nmapStatsFromJSON(t, out)
	l := doc.DNSLookup
	if ok || len(stats) != 0 || l == nil || l.Hostname != "this-host-should-not-exist.invalid" || l.Success || l.Error == "" || doc.Error != l.Error {
		t.Errorf("ok %v, error %q, dns_lookup %+v:\n%s", ok, doc.Error, l, out)
	}
}
