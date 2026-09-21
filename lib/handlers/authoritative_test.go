package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/dmartsapp/shint/v4/lib"
)

func nsRR(owner, host string) dnsmessage.Resource {
	return dnsmessage.Resource{Header: rrHeader(owner, dnsmessage.TypeNS, 3600), Body: &dnsmessage.NSResource{NS: nm(host)}}
}

// zonesServer answers NS questions from a table of zone apexes; any other name is
// NOERROR with no answer (as a name below an apex is) unless it is in nxdomain.
// delay slows the answer for the names listed in it.
type zonesServer struct {
	apexes   map[string][]string // owner -> name servers
	alias    map[string]string   // name -> the name it is a CNAME of
	nxdomain map[string]bool
	delay    map[string]time.Duration
}

func (z zonesServer) start(t *testing.T) *fakeDNS {
	t.Helper()
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		q := r.msg.Questions[0]
		name := strings.TrimSuffix(strings.ToLower(q.Name.String()), ".")
		if d := z.delay[name]; d > 0 {
			time.Sleep(d)
		}
		if q.Type != dnsmessage.TypeNS {
			return [][]byte{reply(r, dnsmessage.RCodeSuccess, nil, nil)}
		}
		if z.nxdomain[name] {
			return [][]byte{reply(r, dnsmessage.RCodeNameError, nil, nil)}
		}
		// A name that is an alias is answered the way a recursive resolver does it: the
		// CNAME, then the records of the name it points to - owned by that name.
		if target, ok := z.alias[name]; ok {
			answers := []dnsmessage.Resource{{Header: rrHeader(name+".", dnsmessage.TypeCNAME, 60), Body: &dnsmessage.CNAMEResource{CNAME: nm(target + ".")}}}
			for _, h := range z.apexes[target] {
				answers = append(answers, nsRR(target+".", h))
			}
			return [][]byte{reply(r, dnsmessage.RCodeSuccess, answers, nil)}
		}
		var answers []dnsmessage.Resource
		for _, h := range z.apexes[name] {
			answers = append(answers, nsRR(name+".", h))
		}
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, answers, nil)}
	})
	fakeSystemServers(t, f.addr)
	return f
}

func TestFindAuthoritativeName(t *testing.T) {
	z := zonesServer{apexes: map[string][]string{
		"example.com":   {"B.iana-servers.net.", "a.iana-servers.net.", "a.iana-servers.net."}, // unsorted, duplicated, capitals
		"com":           {"a.gtld-servers.net."},
		"c.example.com": {"ns1.delegated.example.net."},
		"alias.example": {"ns.alias.example."},
		"example":       {"a.nic.example."},
		"github.com":    {"dns1.nsone.net.", "ns-1.awsdns.org."},
	}, alias: map[string]string{"www.github.com": "github.com"}}
	z.start(t)
	find := func(host string) *lib.Authoritative {
		return findAuthoritative(context.Background(), host, 2*time.Second)
	}

	if a := find("www.example.com"); a == nil || a.Zone != "example.com" || strings.Join(a.Nameservers, ",") != "a.iana-servers.net,b.iana-servers.net" || a.Error != "" || a.TimeTaken <= 0 {
		t.Errorf("www.example.com: %+v", a)
	}
	if a := find("example.com."); a.Zone != "example.com" {
		t.Errorf("an apex: %+v", a)
	}
	if a := find("WWW.Example.COM"); a.Zone != "example.com" {
		t.Errorf("capitals: %+v", a)
	}
	// a subzone delegated elsewhere is closer than the zone above it
	if a := find("x.y.c.example.com"); a.Zone != "c.example.com" || a.Nameservers[0] != "ns1.delegated.example.net" {
		t.Errorf("subzone: %+v", a)
	}
	// only the top-level domain has name servers: it is the zone that would answer for the name
	if a := find("nothing.example"); a.Zone != "example" {
		t.Errorf("tld: %+v", a)
	}
	// The bug this guards: the resolver answers an NS question about an alias with the
	// alias and the NS records of its target. Those records are not the alias's own.
	if a := find("www.github.com"); a.Zone != "github.com" || strings.Join(a.Nameservers, ",") != "dns1.nsone.net,ns-1.awsdns.org" {
		t.Errorf("an alias must not be reported as its own zone: %+v", a)
	}
}

func TestFindAuthoritativeSkipsNamesThatHaveNoZone(t *testing.T) {
	f := zonesServer{apexes: map[string][]string{"example.com": {"a.example."}}}.start(t)
	for _, host := range []string{"192.0.2.1", "2001:db8::1", "localhost", "myhost", "", "foo.localhost"} {
		if a := findAuthoritative(context.Background(), host, time.Second); a != nil {
			t.Errorf("findAuthoritative(%q) = %+v, want nil", host, a)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("no lookup should be made for these, got %d", n)
	}
}

func TestFindAuthoritativeWhenThereIsNothingToFind(t *testing.T) {
	zonesServer{nxdomain: map[string]bool{"nothing.example": true, "example": true}}.start(t)
	a := findAuthoritative(context.Background(), "nothing.example", time.Second)
	if a == nil || a.Zone != "" || a.Nameservers == nil || len(a.Nameservers) != 0 || a.Error != "no name servers found for nothing.example or any of its parent zones" {
		t.Errorf("%+v", a)
	}
}

// Only the candidates closer than the first one found can change the answer, so a
// slow lookup of a parent - typically the top-level domain - must not delay it.
func TestFindAuthoritativeDoesNotWaitForParentsOnceTheAnswerIsDecided(t *testing.T) {
	zonesServer{
		apexes: map[string][]string{"example.com": {"ns.example.com."}, "com": {"a.gtld.example."}},
		delay:  map[string]time.Duration{"com": 3 * time.Second},
	}.start(t)
	begin := time.Now()
	a := findAuthoritative(context.Background(), "www.example.com", 5*time.Second)
	if a.Zone != "example.com" || time.Since(begin) > time.Second {
		t.Errorf("zone %q after %v: the slow parent held up the answer", a.Zone, time.Since(begin))
	}
	// ...but a closer candidate that has not answered yet must be waited for: the
	// name itself is the apex here, and it is the slow one.
	zonesServer{
		apexes: map[string][]string{"slow.example.com": {"ns.slow.example.com."}, "example.com": {"ns.example.com."}},
		delay:  map[string]time.Duration{"slow.example.com": 400 * time.Millisecond},
	}.start(t)
	if a := findAuthoritative(context.Background(), "slow.example.com", 5*time.Second); a.Zone != "slow.example.com" {
		t.Errorf("the closer candidate was not waited for: zone %q", a.Zone)
	}
}

func TestFindAuthoritativeSaysWhenTheServersCouldNotBeAsked(t *testing.T) {
	fakeSystemServers(t, closedUDPAddr(t))
	a := findAuthoritative(context.Background(), "www.example.com", time.Second)
	if a == nil || a.Zone != "" || !strings.Contains(a.Error, "the name servers could not be looked up") || !strings.Contains(a.Error, "connection refused") {
		t.Errorf("%+v", a)
	}
	printed := captureStdout(t, func() { printAuthoritative("telnet", a) })
	if printed != "" {
		t.Errorf("nothing is printed when no zone was found: %q", printed)
	}
}

func TestFindAuthoritativeIsCutShortByCtrlC(t *testing.T) {
	zonesServer{delay: map[string]time.Duration{"www.example.com": 5 * time.Second, "example.com": 5 * time.Second, "com": 5 * time.Second}}.start(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(150*time.Millisecond, cancel)
	begin := time.Now()
	findAuthoritative(ctx, "www.example.com", 30*time.Second)
	if time.Since(begin) > 2*time.Second {
		t.Errorf("Ctrl+C did not cut the lookup short: %v", time.Since(begin))
	}
}

func TestPrintAuthoritativeAndItsTimeout(t *testing.T) {
	out := captureStdout(t, func() {
		printAuthoritative("web", &lib.Authoritative{Zone: "example.com", Nameservers: []string{"a.iana-servers.net", "b.iana-servers.net"}, TimeTaken: 31200})
	})
	if !strings.Contains(out, "[web] OK dns authoritative zone=example.com nameservers=[a.iana-servers.net,b.iana-servers.net] time=31.2ms") {
		t.Errorf("line: %q", out)
	}
	if got := captureStdout(t, func() { printAuthoritative("web", nil) }); got != "" {
		t.Errorf("nil: %q", got)
	}
	for in, want := range map[int]time.Duration{1: time.Second, 3: 3 * time.Second, 5: 3 * time.Second, 60: 3 * time.Second} {
		if got := authoritativeTimeout(in); got != want {
			t.Errorf("authoritativeTimeout(%d) = %v, want %v", in, got, want)
		}
	}
}
