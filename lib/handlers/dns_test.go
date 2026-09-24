package handlers

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/dmartsapp/shint/v4/lib"
)

// ---- a fake DNS server: UDP and TCP on one loopback port

type dnsRequest struct {
	msg  dnsmessage.Message
	tcp  bool
	from net.Addr
}

type fakeDNS struct {
	addr string // 127.0.0.1:port
	mu   sync.Mutex
	seen []dnsRequest
}

func (f *fakeDNS) requests() []dnsRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]dnsRequest(nil), f.seen...)
}

// startFakeDNS answers with whatever packets handle returns for each request (none
// means: say nothing). The same handler serves UDP and TCP.
func startFakeDNS(t *testing.T, handle func(dnsRequest) [][]byte) *fakeDNS {
	t.Helper()
	var udp net.PacketConn
	var tcp net.Listener
	for i := 0; i < 20; i++ {
		u, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		l, err := net.Listen("tcp", u.LocalAddr().String())
		if err == nil {
			udp, tcp = u, l
			break
		}
		_ = u.Close()
	}
	if udp == nil {
		t.Skip("could not get the same UDP and TCP port on loopback")
	}
	f := &fakeDNS{addr: udp.LocalAddr().String()}
	t.Cleanup(func() { _ = udp.Close(); _ = tcp.Close() })
	record := func(r dnsRequest) {
		f.mu.Lock()
		f.seen = append(f.seen, r)
		f.mu.Unlock()
	}
	go func() {
		buf := make([]byte, 65535)
		for {
			n, from, err := udp.ReadFrom(buf)
			if err != nil {
				return
			}
			var m dnsmessage.Message
			if m.Unpack(buf[:n]) != nil {
				continue
			}
			go func(r dnsRequest) {
				record(r)
				for _, p := range handle(r) {
					_, _ = udp.WriteTo(p, r.from)
				}
			}(dnsRequest{msg: m, from: from})
		}
	}()
	go func() {
		for {
			c, err := tcp.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				var size [2]byte
				if _, err := readFull(c, size[:]); err != nil {
					return
				}
				body := make([]byte, binary.BigEndian.Uint16(size[:]))
				if _, err := readFull(c, body); err != nil {
					return
				}
				var m dnsmessage.Message
				if m.Unpack(body) != nil {
					return
				}
				r := dnsRequest{msg: m, tcp: true, from: c.RemoteAddr()}
				record(r)
				for _, p := range handle(r) {
					frame := make([]byte, 2, 2+len(p))
					binary.BigEndian.PutUint16(frame, uint16(len(p)))
					_, _ = c.Write(append(frame, p...))
				}
			}(c)
		}
	}()
	return f
}

func readFull(c net.Conn, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, err := c.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func nm(s string) dnsmessage.Name { return dnsmessage.MustNewName(s) }

func rrHeader(name string, typ dnsmessage.Type, ttl uint32) dnsmessage.ResourceHeader {
	return dnsmessage.ResourceHeader{Name: nm(name), Type: typ, Class: dnsmessage.ClassINET, TTL: ttl}
}

func aRR(name, ip string, ttl uint32) dnsmessage.Resource {
	var a [4]byte
	copy(a[:], net.ParseIP(ip).To4())
	return dnsmessage.Resource{Header: rrHeader(name, dnsmessage.TypeA, ttl), Body: &dnsmessage.AResource{A: a}}
}

func aaaaRR(name, ip string, ttl uint32) dnsmessage.Resource {
	var a [16]byte
	copy(a[:], net.ParseIP(ip).To16())
	return dnsmessage.Resource{Header: rrHeader(name, dnsmessage.TypeAAAA, ttl), Body: &dnsmessage.AAAAResource{AAAA: a}}
}

func soaRR(name string, ttl uint32) dnsmessage.Resource {
	return dnsmessage.Resource{Header: rrHeader(name, dnsmessage.TypeSOA, ttl), Body: &dnsmessage.SOAResource{
		NS: nm("ns1.example.com."), MBox: nm("hostmaster.example.com."), Serial: 2026092101, Refresh: 7200, Retry: 3600, Expire: 1209600, MinTTL: 300}}
}

// reply builds the answer to req: the ID and question echoed, the sections given.
// mods adjust the message before it is packed.
func reply(req dnsRequest, rcode dnsmessage.RCode, answers, authority []dnsmessage.Resource, mods ...func(*dnsmessage.Message)) []byte {
	m := dnsmessage.Message{
		Header:      dnsmessage.Header{ID: req.msg.ID, Response: true, RecursionDesired: req.msg.RecursionDesired, RecursionAvailable: true, RCode: rcode},
		Questions:   req.msg.Questions,
		Answers:     answers,
		Authorities: authority,
	}
	for _, mod := range mods {
		mod(&m)
	}
	p, err := m.Pack()
	if err != nil {
		panic(err)
	}
	return p
}

// ---- running the handler

func queryFor(t *testing.T, f *fakeDNS, args ...string) DNSQuery {
	t.Helper()
	q, err := ParseDNSArgs(append(args, "@"+f.addr))
	if err != nil {
		t.Fatalf("ParseDNSArgs(%v): %v", args, err)
	}
	return q
}

func runDNS(t *testing.T, q DNSQuery, opt DNSOptions, timeout, iterations int, asJSON bool) (bool, string) {
	t.Helper()
	throttle := false
	var ok bool
	out := captureStdout(t, func() {
		ok = DNSHandler(context.Background(), &asJSON, iterations, 0, &throttle, timeout, q, opt)
	})
	return ok, out
}

func mustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

// ---- answers and record formats

func TestDNSRecordFormats(t *testing.T) {
	txt := &dnsmessage.TXTResource{TXT: []string{"v=spf1 include:_spf.example.com", " ~all"}}
	answers := map[dnsmessage.Type][]dnsmessage.Resource{
		dnsmessage.TypeA:     {aRR("example.com.", "192.0.2.10", 300), aRR("example.com.", "192.0.2.11", 300)},
		dnsmessage.TypeAAAA:  {aaaaRR("example.com.", "2001:db8::10", 60)},
		dnsmessage.TypeCNAME: {{Header: rrHeader("www.example.com.", dnsmessage.TypeCNAME, 120), Body: &dnsmessage.CNAMEResource{CNAME: nm("example.com.")}}},
		dnsmessage.TypeMX: {{Header: rrHeader("example.com.", dnsmessage.TypeMX, 300), Body: &dnsmessage.MXResource{Pref: 10, MX: nm("mail.example.com.")}},
			{Header: rrHeader("example.com.", dnsmessage.TypeMX, 300), Body: &dnsmessage.MXResource{Pref: 20, MX: nm("mail2.example.com.")}}},
		dnsmessage.TypeNS:  {{Header: rrHeader("example.com.", dnsmessage.TypeNS, 86400), Body: &dnsmessage.NSResource{NS: nm("ns1.example.com.")}}},
		dnsmessage.TypeTXT: {{Header: rrHeader("example.com.", dnsmessage.TypeTXT, 3600), Body: txt}},
		dnsmessage.TypeSOA: {soaRR("example.com.", 3600)},
		dnsmessage.TypeSRV: {{Header: rrHeader("_sip._tcp.example.com.", dnsmessage.TypeSRV, 300), Body: &dnsmessage.SRVResource{Priority: 10, Weight: 60, Port: 5060, Target: nm("sip.example.com.")}}},
		typeCAA:            {{Header: rrHeader("example.com.", typeCAA, 300), Body: &dnsmessage.UnknownResource{Type: typeCAA, Data: append([]byte{0, 5}, []byte("issuephrase-not-used")...)}}},
	}
	// a proper CAA: flags 0, tag "issue", value "letsencrypt.org"
	answers[typeCAA][0].Body = &dnsmessage.UnknownResource{Type: typeCAA, Data: append([]byte{0, 5}, []byte("issueletsencrypt.org")...)}

	cases := []struct {
		args []string
		typ  dnsmessage.Type
		want []string
	}{
		{[]string{"example.com", "A"}, dnsmessage.TypeA, []string{"answers=2", "answer name=example.com. type=A ttl=300 data=192.0.2.10", "data=192.0.2.11"}},
		{[]string{"example.com", "AAAA"}, dnsmessage.TypeAAAA, []string{"type=AAAA ttl=60 data=2001:db8::10"}},
		{[]string{"www.example.com", "CNAME"}, dnsmessage.TypeCNAME, []string{"answer name=www.example.com. type=CNAME ttl=120 data=example.com."}},
		{[]string{"example.com", "MX"}, dnsmessage.TypeMX, []string{`type=MX ttl=300 data="10 mail.example.com."`, `data="20 mail2.example.com."`}},
		{[]string{"example.com", "NS"}, dnsmessage.TypeNS, []string{"type=NS ttl=86400 data=ns1.example.com."}},
		{[]string{"example.com", "TXT"}, dnsmessage.TypeTXT, []string{`type=TXT ttl=3600 data="v=spf1 include:_spf.example.com ~all"`}},
		{[]string{"example.com", "SOA"}, dnsmessage.TypeSOA, []string{`type=SOA ttl=3600 data="ns1.example.com. hostmaster.example.com. 2026092101 7200 3600 1209600 300"`}},
		{[]string{"_sip._tcp.example.com", "SRV"}, dnsmessage.TypeSRV, []string{`type=SRV ttl=300 data="10 60 5060 sip.example.com."`}},
		{[]string{"example.com", "CAA"}, typeCAA, []string{`type=CAA ttl=300 data="0 issue \"letsencrypt.org\""`}},
	}
	for _, c := range cases {
		t.Run(c.args[1], func(t *testing.T) {
			f := startFakeDNS(t, func(r dnsRequest) [][]byte {
				return [][]byte{reply(r, dnsmessage.RCodeSuccess, answers[r.msg.Questions[0].Type], nil)}
			})
			q := queryFor(t, f, c.args...)
			ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
			if !ok {
				t.Errorf("DNSHandler = false:\n%s", out)
			}
			mustContain(t, out, append([]string{"[dns] OK query name=" + q.Name + " type=" + c.args[1] + " nameserver=" + f.addr + " transport=udp attempt=1/1 rcode=noerror flags=[qr,rd,ra]", "[dns] OK done queries=1 answered=1"}, c.want...)...)
			if strings.Contains(out, "] ERROR ") || strings.Contains(out, "ERROR") {
				t.Errorf("a successful answer printed the text ERROR (an ERROR line means a failed check; the response code is lower case for that reason):\n%s", out)
			}
		})
	}
}

func TestDNSReverseLookupOfAnAddress(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{{Header: rrHeader("8.8.8.8.in-addr.arpa.", dnsmessage.TypePTR, 21599), Body: &dnsmessage.PTRResource{PTR: nm("dns.google.")}}}, nil)}
	})
	q := queryFor(t, f, "8.8.8.8")
	if !q.Reverse || q.Name != "8.8.8.8.in-addr.arpa." || len(q.Types) != 1 || q.Types[0] != dnsmessage.TypePTR {
		t.Fatalf("query = %+v", q)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
	mustContain(t, out, "type=PTR", "answer name=8.8.8.8.in-addr.arpa. type=PTR ttl=21599 data=dns.google.")
	if !ok {
		t.Errorf("DNSHandler = false:\n%s", out)
	}
	if got := f.requests()[0].msg.Questions[0].Name.String(); got != "8.8.8.8.in-addr.arpa." {
		t.Errorf("asked for %q", got)
	}
}

func TestDNSWithoutATypeAsksForBothFamiliesUnlessOneIsChosen(t *testing.T) {
	handle := func(r dnsRequest) [][]byte {
		if r.msg.Questions[0].Type == dnsmessage.TypeAAAA {
			return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aaaaRR("example.com.", "2001:db8::1", 60)}, nil)}
		}
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 60)}, nil)}
	}
	asked := func(f *fakeDNS) []string {
		var got []string
		for _, r := range f.requests() {
			got = append(got, typeName(r.msg.Questions[0].Type))
		}
		return got
	}
	f := startFakeDNS(t, handle)
	ok, out := runDNS(t, queryFor(t, f, "example.com"), DNSOptions{}, 2, 1, false)
	if !ok || len(asked(f)) != 2 || !strings.Contains(out, "done queries=2 answered=2") {
		t.Errorf("ok=%v asked=%v:\n%s", ok, asked(f), out)
	}
	useFamily(t, true, false)
	g := startFakeDNS(t, handle)
	runDNS(t, queryFor(t, g, "example.com"), DNSOptions{}, 2, 1, false)
	if got := asked(g); len(got) != 1 || got[0] != "A" {
		t.Errorf("-4 asked %v, want just A", got)
	}
	// -6 cannot run against this IPv4-only fake server (an IPv4 @server is refused under -6), so only what it would ask is checked
	useFamily(t, false, true)
	q, err := ParseDNSArgs([]string{"example.com"})
	if err != nil || len(q.Types) != 1 || q.Types[0] != dnsmessage.TypeAAAA {
		t.Errorf("-6: types %v err %v, want just AAAA", q.Types, err)
	}
}

// ---- negative and failing answers

func TestDNSNegativeAnswersAreFailedChecksWithTheirReason(t *testing.T) {
	cases := []struct {
		name   string
		rcode  dnsmessage.RCode
		answer []dnsmessage.Resource
		auth   []dnsmessage.Resource
		want   []string
	}{
		{"nxdomain", dnsmessage.RCodeNameError, nil, []dnsmessage.Resource{soaRR("example.com.", 300)},
			[]string{"rcode=nxdomain", `error="no such domain (NXDOMAIN): nope.example.com. does not exist"`, "authority name=example.com. type=SOA ttl=300"}},
		{"no data", dnsmessage.RCodeSuccess, nil, []dnsmessage.Resource{soaRR("example.com.", 300)},
			[]string{"rcode=noerror", `error="no A records: nope.example.com. exists but has none of this type"`, "authority=1"}},
		{"only a cname", dnsmessage.RCodeSuccess, []dnsmessage.Resource{{Header: rrHeader("nope.example.com.", dnsmessage.TypeCNAME, 60), Body: &dnsmessage.CNAMEResource{CNAME: nm("elsewhere.example.net.")}}}, nil,
			[]string{"alias (CNAME) whose target has none", "answer name=nope.example.com. type=CNAME"}},
		{"servfail", dnsmessage.RCodeServerFailure, nil, nil, []string{"rcode=servfail", "the server failed to answer (SERVFAIL)"}},
		{"refused", dnsmessage.RCodeRefused, nil, nil, []string{"rcode=refused", "the server refused to answer (REFUSED)"}},
		{"notimp", dnsmessage.RCodeNotImplemented, nil, nil, []string{"the server answered with NOTIMP"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := startFakeDNS(t, func(r dnsRequest) [][]byte { return [][]byte{reply(r, c.rcode, c.answer, c.auth)} })
			ok, out := runDNS(t, queryFor(t, f, "nope.example.com", "A"), DNSOptions{}, 2, 1, false)
			if ok {
				t.Errorf("DNSHandler = true for a negative answer:\n%s", out)
			}
			mustContain(t, out, append([]string{"[dns] ERROR query failed name=nope.example.com. type=A nameserver=" + f.addr, "[dns] OK done queries=1 answered=0"}, c.want...)...)
		})
	}
}

// ---- transports and EDNS

func TestDNSTruncatedAnswerIsAskedAgainOverTCP(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		if !r.tcp {
			return [][]byte{reply(r, dnsmessage.RCodeSuccess, nil, nil, func(m *dnsmessage.Message) { m.Truncated = true })}
		}
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("big.example.com.", "192.0.2.7", 30)}, nil)}
	})
	ok, out := runDNS(t, queryFor(t, f, "big.example.com", "A"), DNSOptions{}, 2, 1, false)
	if !ok {
		t.Errorf("DNSHandler = false:\n%s", out)
	}
	mustContain(t, out, "transport=tcp", "retried=tcp", "data=192.0.2.7")
	reqs := f.requests()
	if len(reqs) != 2 || reqs[0].tcp || !reqs[1].tcp {
		t.Errorf("wanted one UDP then one TCP request, got %+v", reqs)
	}
}

func TestDNSForceTCPNeverUsesUDP(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	ok, out := runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{ForceTCP: true}, 2, 1, false)
	if !ok {
		t.Errorf("DNSHandler = false:\n%s", out)
	}
	mustContain(t, out, "transport=tcp")
	if strings.Contains(out, "retried=") {
		t.Errorf("--tcp is not a retry:\n%s", out)
	}
	for _, r := range f.requests() {
		if !r.tcp {
			t.Error("a UDP query was sent although --tcp was given")
		}
	}
}

func TestDNSAnnouncesEDNSAndTheRecursionBit(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{}, 2, 1, false)
	runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{NoRecurse: true}, 2, 1, false)
	reqs := f.requests()
	if len(reqs) != 2 {
		t.Fatalf("got %d requests", len(reqs))
	}
	if !reqs[0].msg.RecursionDesired || reqs[1].msg.RecursionDesired {
		t.Errorf("recursion desired: default %v (want true), --no-recurse %v (want false)", reqs[0].msg.RecursionDesired, reqs[1].msg.RecursionDesired)
	}
	opt := reqs[0].msg.Additionals
	if len(opt) != 1 || opt[0].Header.Type != dnsmessage.TypeOPT || int(opt[0].Header.Class) != ednsPayload {
		t.Errorf("the query does not carry an EDNS option announcing %d bytes: %+v", ednsPayload, opt)
	}
}

func TestDNSAsksAgainWithoutEDNSWhenAServerRejectsIt(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		if len(r.msg.Additionals) > 0 { // an old server: EDNS is a format error
			return [][]byte{reply(r, dnsmessage.RCodeFormatError, nil, nil)}
		}
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	ok, out := runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{}, 2, 1, false)
	if !ok || !strings.Contains(out, "rcode=noerror") {
		t.Errorf("ok=%v:\n%s", ok, out)
	}
	if reqs := f.requests(); len(reqs) != 2 || len(reqs[1].msg.Additionals) != 0 {
		t.Errorf("wanted a second query without the EDNS option, got %+v", reqs)
	}
}

// ---- replies that are not the answer

func TestDNSIgnoresRepliesThatAreNotTheAnswer(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		wrongID := reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "203.0.113.66", 30)}, nil, func(m *dnsmessage.Message) { m.ID ^= 0xffff })
		otherQuestion := reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("evil.example.", "203.0.113.67", 30)}, nil,
			func(m *dnsmessage.Message) {
				m.Questions = []dnsmessage.Question{{Name: nm("evil.example."), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}
			})
		notAResponse := reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "203.0.113.68", 30)}, nil, func(m *dnsmessage.Message) { m.Response = false })
		return [][]byte{wrongID, otherQuestion, notAResponse, []byte("this is not a dns message"),
			reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	ok, out := runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{}, 2, 1, false)
	if !ok || !strings.Contains(out, "data=192.0.2.1") || strings.Contains(out, "203.0.113.") {
		t.Errorf("ok=%v: a reply that was not the answer was believed:\n%s", ok, out)
	}
}

func TestDNSIgnoresAForgedReplyFromAnotherSource(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		// A stranger on another port sends a well-formed reply with the right ID
		// straight to the client's socket before the real answer comes.
		spoofer, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err == nil {
			defer func() { _ = spoofer.Close() }()
			_, _ = spoofer.WriteTo(reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "203.0.113.66", 30)}, nil), r.from)
			time.Sleep(100 * time.Millisecond)
		}
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	ok, out := runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{}, 2, 1, false)
	if !ok || !strings.Contains(out, "data=192.0.2.1") || strings.Contains(out, "203.0.113.66") {
		t.Errorf("ok=%v: a forged reply from another source was believed:\n%s", ok, out)
	}
}

func TestDNSSilenceIsATimeoutAndSaysWhatWasIgnored(t *testing.T) {
	silent := startFakeDNS(t, func(dnsRequest) [][]byte { return nil })
	begin := time.Now()
	ok, out := runDNS(t, queryFor(t, silent, "example.com", "A"), DNSOptions{}, 1, 1, false)
	if ok || time.Since(begin) > 4*time.Second {
		t.Errorf("ok=%v after %v:\n%s", ok, time.Since(begin), out)
	}
	mustContain(t, out, "[dns] ERROR query failed name=example.com. type=A attempt=1/1", "no DNS server answered: "+silent.addr, "i/o timeout")

	noise := startFakeDNS(t, func(r dnsRequest) [][]byte {
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, nil, nil, func(m *dnsmessage.Message) { m.ID ^= 1 })}
	})
	ok, out = runDNS(t, queryFor(t, noise, "example.com", "A"), DNSOptions{}, 1, 1, false)
	if ok {
		t.Errorf("ok=true although only wrong replies came:\n%s", out)
	}
	mustContain(t, out, "ignored 1 reply(ies) that did not answer this question")
}

func TestDNSMalformedReplyOverTCPIsReportedNotWaitedOn(t *testing.T) {
	f := startFakeDNS(t, func(dnsRequest) [][]byte { return [][]byte{[]byte("garbage")} })
	ok, out := runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{ForceTCP: true}, 2, 1, false)
	if ok {
		t.Errorf("ok=true:\n%s", out)
	}
	mustContain(t, out, "malformed reply")
}

// ---- which server

func closedUDPAddr(t *testing.T) string {
	t.Helper()
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := c.LocalAddr().String()
	_ = c.Close()
	return addr
}

func fakeSystemServers(t *testing.T, servers ...string) {
	t.Helper()
	old := systemDNSServers
	systemDNSServers = func(context.Context) ([]string, error) { return servers, nil }
	t.Cleanup(func() { systemDNSServers = old })
}

func TestDNSSkipsServersThatDoNotAnswer(t *testing.T) {
	dead := closedUDPAddr(t)
	live := startFakeDNS(t, func(r dnsRequest) [][]byte {
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	fakeSystemServers(t, dead, live.addr)
	q, err := ParseDNSArgs([]string{"example.com", "A"})
	if err != nil {
		t.Fatal(err)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
	if !ok {
		t.Errorf("DNSHandler = false:\n%s", out)
	}
	mustContain(t, out, "nameserver="+live.addr, "skipped="+`"`+dead+" (", "connection refused")
	if strings.Contains(out, "ERROR") {
		t.Errorf("a query that a later server answered printed the text ERROR:\n%s", out)
	}

	fakeSystemServers(t, dead, closedUDPAddr(t))
	ok, out = runDNS(t, q, DNSOptions{}, 2, 1, false)
	if ok {
		t.Errorf("ok=true although no server answered:\n%s", out)
	}
	mustContain(t, out, "ERROR query failed", "no DNS server answered: "+dead)
}

func TestDNSSystemServersFollowTheFamilyChoice(t *testing.T) {
	fakeSystemServers(t, "[2001:db8::53]:53", "192.0.2.53:53")
	useFamily(t, true, false)
	servers, _, err := serverList(context.Background(), DNSQuery{Port: "53"}, 1)
	if err != nil || len(servers) != 1 || servers[0] != "192.0.2.53:53" {
		t.Errorf("-4: servers=%v err=%v", servers, err)
	}
	useFamily(t, false, true)
	servers, _, err = serverList(context.Background(), DNSQuery{Port: "53"}, 1)
	if err != nil || len(servers) != 1 || servers[0] != "[2001:db8::53]:53" {
		t.Errorf("-6: servers=%v err=%v", servers, err)
	}
	fakeSystemServers(t, "192.0.2.53:53")
	if _, _, err = serverList(context.Background(), DNSQuery{Port: "53"}, 1); err == nil || !strings.Contains(err.Error(), "none of this machine's DNS servers") {
		t.Errorf("-6 with only an IPv4 server: err=%v", err)
	}
}

func TestDNSServerGivenByNameIsResolvedFirst(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	_, port, _ := net.SplitHostPort(f.addr)
	q, err := ParseDNSArgs([]string{"example.com", "A", "@localhost:" + port})
	if err != nil {
		t.Fatal(err)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
	if !ok {
		t.Errorf("DNSHandler = false:\n%s", out)
	}
	mustContain(t, out, "[dns] OK server resolved host=localhost")

	bad, _ := ParseDNSArgs([]string{"example.com", "A", "@no-such-host.invalid"})
	ok, out = runDNS(t, bad, DNSOptions{}, 2, 1, false)
	if ok {
		t.Error("ok=true for a server name that does not resolve")
	}
	mustContain(t, out, "[dns] ERROR dns resolution failed host=no-such-host.invalid")
}

// ---- output

func TestDNSEscapesWhatARecordContains(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		hostile := &dnsmessage.TXTResource{TXT: []string{"\x1b[31mred\x1b[0m", "line1\nline2\x00", "\xff\xfe"}}
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{{Header: rrHeader("example.com.", dnsmessage.TypeTXT, 60), Body: hostile}}, nil)}
	})
	_, out := runDNS(t, queryFor(t, f, "example.com", "TXT"), DNSOptions{}, 2, 1, false)
	if strings.ContainsAny(out, "\x1b\x00") || strings.Contains(out, "\xff") {
		t.Errorf("a TXT record reached the terminal unescaped: %q", out)
	}
	mustContain(t, out, `\x1b[31mred`, `line1\nline2\x00`, `\xff\xfe`)
	if lines := strings.Split(strings.TrimRight(out, "\n"), "\n"); len(lines) != 3 { // query, answer, done
		t.Errorf("a line break inside a record forged extra log lines (%d lines):\n%s", len(lines), out)
	}
}

func TestDNSJSONIsOneDocument(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		if r.msg.Questions[0].Type == dnsmessage.TypeMX {
			return [][]byte{reply(r, dnsmessage.RCodeNameError, nil, []dnsmessage.Resource{soaRR("example.com.", 300)})}
		}
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30), aRR("example.com.", "192.0.2.2", 30)}, nil)}
	})
	var doc struct {
		ModuleName string          `json:"module_name"`
		DNSLookup  json.RawMessage `json:"dns_lookup"`
		Error      string          `json:"error"`
		Stats      []lib.DNSStats  `json:"stats"`
	}
	ok, out := runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{}, 2, 1, true)
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if !ok || doc.ModuleName != "dns" || doc.DNSLookup != nil || doc.Error != "" || len(doc.Stats) != 1 {
		t.Fatalf("ok=%v module=%q dns_lookup=%s error=%q stats=%d", ok, doc.ModuleName, doc.DNSLookup, doc.Error, len(doc.Stats))
	}
	s := doc.Stats[0]
	if !s.Success || s.Nameserver != f.addr || s.Transport != "udp" || s.RCode != "NOERROR" || len(s.Answers) != 2 || s.Answers[0].Data != "192.0.2.1" || s.Answers[0].TTL != 30 ||
		s.Authority == nil || len(s.Flags) == 0 || s.Flags[0] != "qr" {
		t.Errorf("stat = %+v", s)
	}

	ok, out = runDNS(t, queryFor(t, f, "nope.example.com", "MX"), DNSOptions{}, 2, 1, true)
	doc.Stats = nil
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	s = doc.Stats[0]
	if ok || s.Success || s.RCode != "NXDOMAIN" || !strings.Contains(s.Error, "no such domain") || len(s.Authority) != 1 || s.Authority[0].Type != "SOA" || s.Answers == nil {
		t.Errorf("failed stat = %+v (ok=%v)", s, ok)
	}
}

func TestDNSCountRepeatsTheQuestion(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		return [][]byte{reply(r, dnsmessage.RCodeSuccess, []dnsmessage.Resource{aRR("example.com.", "192.0.2.1", 30)}, nil)}
	})
	ok, out := runDNS(t, queryFor(t, f, "example.com", "A"), DNSOptions{}, 2, 3, false)
	mustContain(t, out, "attempt=1/3", "attempt=2/3", "attempt=3/3", "done queries=3 answered=3")
	if !ok || len(f.requests()) != 3 {
		t.Errorf("ok=%v requests=%d", ok, len(f.requests()))
	}
}

func TestDNSCtrlCStopsAndReportsHowFarItGot(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		time.Sleep(3 * time.Second) // slower than the test is willing to wait
		return nil
	})
	q := queryFor(t, f, "example.com", "A")
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)
	throttle, asJSON := false, false
	var ok bool
	begin := time.Now()
	out := captureStdout(t, func() { ok = DNSHandler(ctx, &asJSON, 5, 0, &throttle, 30, q, DNSOptions{}) })
	if ok || time.Since(begin) > 2*time.Second {
		t.Errorf("ok=%v after %v: Ctrl+C did not stop the run promptly:\n%s", ok, time.Since(begin), out)
	}
	mustContain(t, out, "[dns] ERROR interrupted attempts_completed=0 attempts_planned=5", "[dns] OK done queries=0")
	if strings.Contains(out, "query failed") {
		t.Errorf("a query cut off by Ctrl+C is not a failed check:\n%s", out)
	}
}

// ---- arguments

func TestParseDNSArgs(t *testing.T) {
	good := []struct {
		args   []string
		name   string
		types  []string
		server string
		port   string
	}{
		{[]string{"example.com"}, "example.com.", []string{"A", "AAAA"}, "", "53"},
		{[]string{"Example.COM."}, "Example.COM.", []string{"A", "AAAA"}, "", "53"},
		{[]string{"example.com", "mx"}, "example.com.", []string{"MX"}, "", "53"},
		{[]string{"example.com", "@1.1.1.1"}, "example.com.", []string{"A", "AAAA"}, "1.1.1.1", "53"},
		{[]string{"@1.1.1.1", "example.com", "TXT"}, "example.com.", []string{"TXT"}, "1.1.1.1", "53"},
		{[]string{"example.com", "NS", "@dns.google"}, "example.com.", []string{"NS"}, "dns.google", "53"},
		{[]string{"example.com", "@dns.google:5353"}, "example.com.", []string{"A", "AAAA"}, "dns.google", "5353"},
		{[]string{"example.com", "@[2606:4700:4700::1111]:5353"}, "example.com.", []string{"A", "AAAA"}, "2606:4700:4700::1111", "5353"},
		{[]string{"example.com", "@2606:4700:4700::1111"}, "example.com.", []string{"A", "AAAA"}, "2606:4700:4700::1111", "53"},
		{[]string{"example.com", "@[::1]"}, "example.com.", []string{"A", "AAAA"}, "::1", "53"},
		{[]string{"_sip._tcp.example.com", "SRV"}, "_sip._tcp.example.com.", []string{"SRV"}, "", "53"},
		{[]string{"*.example.com", "A"}, "*.example.com.", []string{"A"}, "", "53"},
		{[]string{"."}, ".", []string{"A", "AAAA"}, "", "53"},
		{[]string{"8.8.8.8"}, "8.8.8.8.in-addr.arpa.", []string{"PTR"}, "", "53"},
		{[]string{"8.8.8.8", "ptr"}, "8.8.8.8.in-addr.arpa.", []string{"PTR"}, "", "53"},
		{[]string{"::ffff:8.8.8.8"}, "8.8.8.8.in-addr.arpa.", []string{"PTR"}, "", "53"},
		{[]string{"2001:db8::1"}, "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.", []string{"PTR"}, "", "53"},
	}
	for _, c := range good {
		q, err := ParseDNSArgs(c.args)
		if err != nil {
			t.Errorf("ParseDNSArgs(%v): %v", c.args, err)
			continue
		}
		var types []string
		for _, ty := range q.Types {
			types = append(types, typeName(ty))
		}
		if q.Name != c.name || strings.Join(types, ",") != strings.Join(c.types, ",") || q.Server != c.server || q.Port != c.port {
			t.Errorf("ParseDNSArgs(%v) = name %q types %v server %q port %q, want %q %v %q %q", c.args, q.Name, types, q.Server, q.Port, c.name, c.types, c.server, c.port)
		}
	}
	bad := []struct {
		args []string
		want string
	}{
		{nil, "a name or an IP address is required"},
		{[]string{"@1.1.1.1"}, "a name or an IP address is required"},
		{[]string{"example.com", "FOO"}, `unsupported record type "FOO" (supported: A, AAAA, CNAME, MX, NS, TXT, SOA, SRV, PTR, CAA)`},
		{[]string{"example.com", "A", "MX"}, "too many arguments"},
		{[]string{"example.com", "@1.1.1.1", "@8.8.8.8"}, "only one @server"},
		{[]string{"example.com", "@"}, "@ needs a server"},
		{[]string{"8.8.8.8", "A"}, "which can only be asked for its PTR record"},
		{[]string{"fe80::1%en0"}, "zone identifiers are not supported"},
		{[]string{"bücher.de"}, "xn--"},
		{[]string{"exa mple.com"}, "is not allowed in a DNS name"},
		{[]string{"a..b"}, "empty label"},
		{[]string{".example.com"}, "empty label"},
		{[]string{strings.Repeat("a", 64) + ".com"}, "longer than 63"},
		{[]string{strings.Repeat("a.", 130) + "com"}, "longer than the 253"},
		{[]string{"example.com", "@1.1.1.1:0"}, "invalid server"},
		{[]string{"example.com", "@1.1.1.1:99999"}, "invalid server"},
		{[]string{"example.com", "@fe80::1:53:zz"}, "invalid server"},
		{[]string{"example.com", "@a/b"}, "invalid server"},
	}
	for _, c := range bad {
		_, err := ParseDNSArgs(c.args)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("ParseDNSArgs(%v) error = %v, want one containing %q", c.args, err, c.want)
		}
	}
}

func TestParseDNSArgsRefusesAServerOfTheOtherFamily(t *testing.T) {
	useFamily(t, true, false)
	if _, err := ParseDNSArgs([]string{"example.com", "@2606:4700:4700::1111"}); err == nil || !strings.Contains(err.Error(), "-4") {
		t.Errorf("-4 with an IPv6 server: err=%v", err)
	}
	if _, err := ParseDNSArgs([]string{"example.com", "@dns.google"}); err != nil {
		t.Errorf("-4 with a server name is fine (it is resolved for IPv4): %v", err)
	}
}

func TestRecordOfHandlesWhatItDoesNotKnow(t *testing.T) {
	odd := dnsmessage.Resource{Header: rrHeader("example.com.", dnsmessage.Type(65), 5), Body: &dnsmessage.UnknownResource{Type: 65, Data: []byte{1, 2, 0xab}}}
	if got := recordOf(odd); got.Type != "TYPE65" || got.Data != `\# 3 0102ab` {
		t.Errorf("unknown type: %+v", got)
	}
	short := dnsmessage.Resource{Header: rrHeader("example.com.", typeCAA, 5), Body: &dnsmessage.UnknownResource{Type: typeCAA, Data: []byte{0, 9, 'x'}}}
	if got := recordOf(short); got.Type != "CAA" || !strings.HasPrefix(got.Data, `\# 3 `) {
		t.Errorf("a CAA record that does not parse must not panic or lie: %+v", got)
	}
}

// The real system: whatever machine this runs on either has DNS servers or says so.
func TestSystemDNSServersFromTheRealResolver(t *testing.T) {
	servers, err := systemDNSServers(context.Background())
	if err != nil {
		t.Skipf("this environment has no DNS servers configured: %v", err)
	}
	for _, s := range servers {
		if _, _, err := net.SplitHostPort(s); err != nil {
			t.Errorf("server %q is not host:port", s)
		}
	}
}

// ---- hosts file and localhost resolution (#64)

func TestDNSLocalhostResolvesBothFamiliesWithoutNetwork(t *testing.T) {
	q, err := ParseDNSArgs([]string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
	if !ok {
		t.Errorf("DNSHandler(localhost) failed:\n%s", out)
	}
	mustContain(t, out,
		"query name=localhost. type=A nameserver=hosts transport=file attempt=1/1 rcode=noerror flags=[qr,aa] answers=1",
		"answer name=localhost. type=A ttl=0 data=127.0.0.1",
		"query name=localhost. type=AAAA nameserver=hosts transport=file attempt=1/1 rcode=noerror flags=[qr,aa] answers=1",
		"answer name=localhost. type=AAAA ttl=0 data=::1",
		"done queries=2 answered=2",
	)
}

func TestDNSLocalhostFollowsIPv4AndIPv6FamilyFlags(t *testing.T) {
	// IPv4 only (-4)
	useFamily(t, true, false)
	q4, err := ParseDNSArgs([]string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	ok4, out4 := runDNS(t, q4, DNSOptions{}, 2, 1, false)
	if !ok4 {
		t.Errorf("DNSHandler(localhost, -4) failed:\n%s", out4)
	}
	mustContain(t, out4,
		"query name=localhost. type=A nameserver=hosts transport=file",
		"answer name=localhost. type=A ttl=0 data=127.0.0.1",
		"done queries=1 answered=1",
	)
	if strings.Contains(out4, "type=AAAA") {
		t.Errorf("-4 output contains AAAA query:\n%s", out4)
	}

	// IPv6 only (-6)
	useFamily(t, false, true)
	q6, err := ParseDNSArgs([]string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	ok6, out6 := runDNS(t, q6, DNSOptions{}, 2, 1, false)
	if !ok6 {
		t.Errorf("DNSHandler(localhost, -6) failed:\n%s", out6)
	}
	mustContain(t, out6,
		"query name=localhost. type=AAAA nameserver=hosts transport=file",
		"answer name=localhost. type=AAAA ttl=0 data=::1",
		"done queries=1 answered=1",
	)
	if strings.Contains(out6, "type=A ") {
		t.Errorf("-6 output contains A query:\n%s", out6)
	}
}

func TestDNSCustomHostsFileResolution(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts")
	content := []byte(strings.Join([]string{
		"# comment line",
		"192.0.2.55   dev.local  dev-alias.local # inline comment",
		"2001:db8::55 dev.local",
		"",
	}, "\n"))
	if err := os.WriteFile(hostsPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	old := hostsFile
	hostsFile = func() string { return hostsPath }
	t.Cleanup(func() { hostsFile = old })

	// Forward lookup dev.local (dual-stack)
	q, err := ParseDNSArgs([]string{"dev.local"})
	if err != nil {
		t.Fatal(err)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
	if !ok {
		t.Errorf("DNSHandler(dev.local) failed:\n%s", out)
	}
	mustContain(t, out,
		"query name=dev.local. type=A nameserver=hosts transport=file",
		"answer name=dev.local. type=A ttl=0 data=192.0.2.55",
		"query name=dev.local. type=AAAA nameserver=hosts transport=file",
		"answer name=dev.local. type=AAAA ttl=0 data=2001:db8::55",
		"done queries=2 answered=2",
	)

	// Forward lookup alias dev-alias.local
	qAlias, err := ParseDNSArgs([]string{"dev-alias.local", "A"})
	if err != nil {
		t.Fatal(err)
	}
	okAlias, outAlias := runDNS(t, qAlias, DNSOptions{}, 2, 1, false)
	if !okAlias {
		t.Errorf("DNSHandler(dev-alias.local) failed:\n%s", outAlias)
	}
	mustContain(t, outAlias,
		"query name=dev-alias.local. type=A nameserver=hosts transport=file",
		"answer name=dev-alias.local. type=A ttl=0 data=192.0.2.55",
	)
}

func TestDNSExplicitServerNeverUsesHostsFallback(t *testing.T) {
	f := startFakeDNS(t, func(r dnsRequest) [][]byte {
		// Server intentionally returns NXDOMAIN for localhost
		return [][]byte{reply(r, dnsmessage.RCodeNameError, nil, nil)}
	})
	q, err := ParseDNSArgs([]string{"localhost", "A", "@" + f.addr})
	if err != nil {
		t.Fatal(err)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
	if ok {
		t.Errorf("expected failed query with explicit @server returning NXDOMAIN, got ok=true")
	}
	mustContain(t, out,
		"nameserver="+f.addr,
		"transport=udp",
		"rcode=nxdomain",
		"no such domain (NXDOMAIN): localhost. does not exist",
	)
	if strings.Contains(out, "nameserver=hosts") {
		t.Errorf("query with explicit @server used hosts fallback:\n%s", out)
	}
	reqs := f.requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 wire request to fake server, got %d", len(reqs))
	}
}

func TestDNSLocalhostSucceedsEvenWhenNoSystemDNSServers(t *testing.T) {
	oldSys := systemDNSServers
	systemDNSServers = func(context.Context) ([]string, error) {
		return nil, errors.New("no DNS servers are configured on this machine")
	}
	t.Cleanup(func() { systemDNSServers = oldSys })

	q, err := ParseDNSArgs([]string{"localhost", "A"})
	if err != nil {
		t.Fatal(err)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, false)
	if !ok {
		t.Errorf("DNSHandler(localhost) failed when no system DNS servers configured:\n%s", out)
	}
	mustContain(t, out,
		"query name=localhost. type=A nameserver=hosts transport=file",
		"answer name=localhost. type=A ttl=0 data=127.0.0.1",
		"done queries=1 answered=1",
	)
}

func TestDNSLocalhostJSONOutput(t *testing.T) {
	q, err := ParseDNSArgs([]string{"localhost", "A"})
	if err != nil {
		t.Fatal(err)
	}
	ok, out := runDNS(t, q, DNSOptions{}, 2, 1, true)
	if !ok {
		t.Fatalf("DNSHandler(localhost, JSON) failed:\n%s", out)
	}
	var doc lib.LocalJSONOutput
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out)
	}
	stats := doc.Stats.([]any)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat entry, got %d", len(stats))
	}
	m := stats[0].(map[string]any)
	if m["nameserver"] != "hosts" || m["transport"] != "file" || m["rcode"] != "NOERROR" || m["success"] != true {
		t.Errorf("unexpected stat JSON: %+v", m)
	}
	answers := m["answers"].([]any)
	if len(answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(answers))
	}
	ans := answers[0].(map[string]any)
	if ans["name"] != "localhost." || ans["type"] != "A" || ans["data"] != "127.0.0.1" {
		t.Errorf("unexpected answer JSON: %+v", ans)
	}
}
