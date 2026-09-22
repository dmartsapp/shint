package handlers

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/dmartsapp/shint/v4/lib"
)

const dnsModule = "dns"

// dnsDefaultPort is where a DNS server listens unless @server says otherwise.
const dnsDefaultPort = "53"

// ednsPayload is the UDP payload size announced in the EDNS0 option: 1232 bytes
// is what the DNS community settled on (DNS Flag Day 2020) as the most that
// crosses the internet without IP fragmentation.
const ednsPayload = 1232

// typeCAA has no constant in dnsmessage; it is decoded here (RFC 8659).
const typeCAA = dnsmessage.Type(257)

// dnsTypeNames are the record types "dns" can ask for.
var dnsTypeNames = []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT", "SOA", "SRV", "PTR", "CAA"}

var dnsTypeByName = map[string]dnsmessage.Type{
	"A": dnsmessage.TypeA, "AAAA": dnsmessage.TypeAAAA, "CNAME": dnsmessage.TypeCNAME, "MX": dnsmessage.TypeMX,
	"NS": dnsmessage.TypeNS, "TXT": dnsmessage.TypeTXT, "SOA": dnsmessage.TypeSOA, "SRV": dnsmessage.TypeSRV,
	"PTR": dnsmessage.TypePTR, "CAA": typeCAA,
}

// typeName is a record type in dig's spelling: A, MX, ... and TYPE65 for one
// shint does not know by name.
func typeName(t dnsmessage.Type) string {
	for name, known := range dnsTypeByName {
		if known == t {
			return name
		}
	}
	return "TYPE" + strconv.Itoa(int(t))
}

// DNSQuery is what the command line asked: the name (already fully qualified),
// the record types, and where to ask.
type DNSQuery struct {
	Input   string // the name or address exactly as typed
	Name    string // the name asked for, fully qualified: "example.com."
	Types   []dnsmessage.Type
	Server  string // host or address after @; empty: the system's DNS servers
	Port    string // the server's port
	Reverse bool   // Name is the reverse (PTR) name of an IP address
}

// DNSOptions are the flags of "dns" that change how a question is asked.
type DNSOptions struct {
	ForceTCP  bool // --tcp: never use UDP
	NoRecurse bool // --no-recurse: clear the "recursion desired" bit
}

// ParseDNSArgs reads "name [type] [@server]" (the type and the @server may come
// in either order after the name). An IP address as the name is a reverse
// lookup, like dig -x. Without a type a name is asked for its A and AAAA records
// (only one of them under -4 or -6), the way every other command looks at both
// families. Everything is checked before anything is sent.
func ParseDNSArgs(args []string) (DNSQuery, error) {
	var name, typ, server string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "@"):
			if server != "" {
				return DNSQuery{}, errors.New("only one @server can be given")
			}
			if server = a[1:]; server == "" {
				return DNSQuery{}, errors.New("@ needs a server: an address or a name, as in @1.1.1.1")
			}
		case name == "":
			name = a
		case typ == "":
			typ = strings.ToUpper(a)
		default:
			return DNSQuery{}, errors.New("too many arguments: expected a name, then optionally a type and an @server")
		}
	}
	q := DNSQuery{Input: name, Port: dnsDefaultPort}
	if name == "" {
		return q, errors.New("a name or an IP address is required")
	}

	var want dnsmessage.Type
	if typ != "" {
		t, ok := dnsTypeByName[typ]
		if !ok {
			return q, fmt.Errorf("unsupported record type %q (supported: %s)", typ, strings.Join(dnsTypeNames, ", "))
		}
		want = t
	}

	if addr, err := netip.ParseAddr(name); err == nil {
		if addr.Zone() != "" {
			return q, fmt.Errorf("invalid address %q: zone identifiers are not supported", name)
		}
		if want != 0 && want != dnsmessage.TypePTR {
			return q, fmt.Errorf("%s is an IP address, which can only be asked for its PTR record (its reverse name), not %s", name, typ)
		}
		q.Name, q.Types, q.Reverse = ReverseName(addr), []dnsmessage.Type{dnsmessage.TypePTR}, true
	} else {
		fq, err := normalizeDNSName(name)
		if err != nil {
			return q, err
		}
		q.Name = fq
		switch {
		case want != 0:
			q.Types = []dnsmessage.Type{want}
		case lib.NetworkType == "ip4":
			q.Types = []dnsmessage.Type{dnsmessage.TypeA}
		case lib.NetworkType == "ip6":
			q.Types = []dnsmessage.Type{dnsmessage.TypeAAAA}
		default:
			q.Types = []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA}
		}
	}

	if server != "" {
		host, port, err := parseDNSServer(server)
		if err != nil {
			return q, err
		}
		if err := lib.HostFamilyConflict(host); err != nil {
			return q, err
		}
		q.Server, q.Port = host, port
	}
	return q, nil
}

// normalizeDNSName checks a host name and returns it fully qualified (with the
// trailing dot). Labels are 1-63 characters and the whole name at most 253;
// letters, digits, hyphens, underscores (SRV names start with them) and the
// wildcard star are accepted. Non-ASCII names are refused rather than sent as
// raw bytes that no server would match: internationalized names go in their
// xn-- form.
func normalizeDNSName(s string) (string, error) {
	if s == "." {
		return ".", nil
	}
	name := strings.TrimSuffix(s, ".")
	if name == "" || strings.HasPrefix(name, ".") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid name %q: empty label", s)
	}
	if len(name) > 253 {
		return "", fmt.Errorf("invalid name: %d characters is longer than the 253 a DNS name can have", len(name))
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) > 63 {
			return "", fmt.Errorf("invalid name: the label %q is longer than 63 characters", label[:20]+"...")
		}
		for _, r := range label {
			switch {
			case r > 127:
				return "", fmt.Errorf("invalid name %q: internationalized names must be written in their xn-- (punycode) form", s)
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '*':
			default:
				return "", fmt.Errorf("invalid name %q: %q is not allowed in a DNS name", s, string(r))
			}
		}
	}
	return name + ".", nil
}

// parseDNSServer splits the part after @ into a host (an address or a name) and
// a port: 1.1.1.1, 1.1.1.1:5353, [::1], [::1]:5353, ::1 and dns.google:5353 are
// all fine.
func parseDNSServer(s string) (host, port string, err error) {
	port = dnsDefaultPort
	if addr, perr := netip.ParseAddr(s); perr == nil { // an address on its own, including a bare IPv6 one
		return addr.String(), port, nil
	}
	host = s
	if h, p, serr := net.SplitHostPort(s); serr == nil {
		host, port = h, p
		if _, verr := lib.ValidatePort(p); verr != nil {
			return "", "", fmt.Errorf("invalid server %q: %s", s, verr.Error())
		}
	} else if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		host = s[1 : len(s)-1]
	} else if strings.Contains(s, ":") {
		return "", "", fmt.Errorf("invalid server %q: write an IPv6 address with a port as [address]:port", s)
	}
	if host == "" || strings.ContainsAny(host, " \t/") {
		return "", "", fmt.Errorf("invalid server %q", s)
	}
	return host, port, nil
}

// systemDNSServers is where the servers to ask come from when there is no
// @server: the ones the system is configured with. It is a variable so tests can
// choose them. In the program it asks Go's own resolver which servers it would
// use, by giving it a Dial function that only writes the addresses down and then
// fails: that works the same on Linux, macOS and Windows, wherever the
// configuration actually lives, and nothing is sent.
var systemDNSServers = func(ctx context.Context) ([]string, error) {
	var mu sync.Mutex
	var found []string
	seen := map[string]bool{}
	probe := &net.Resolver{PreferGo: true, Dial: func(_ context.Context, _, address string) (net.Conn, error) {
		mu.Lock()
		defer mu.Unlock()
		if !seen[address] {
			seen[address] = true
			found = append(found, address)
		}
		return nil, errors.New("shint: reading the resolver configuration")
	}}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, _ = probe.LookupHost(probeCtx, "shint-resolver-probe.invalid.")
	mu.Lock()
	defer mu.Unlock()
	if len(found) == 0 {
		return nil, errors.New("no DNS servers are configured on this machine")
	}
	return append([]string(nil), found...), nil
}

// serverList turns the query's server choice into addresses to ask, in order:
// the @server (resolved first if it is a name), or the system's servers, keeping
// only the family -4/-6 allows. resolved is what a name resolved to, for the log.
func serverList(ctx context.Context, q DNSQuery, timeout int) (servers []string, resolved []string, err error) {
	if q.Server != "" {
		if addr, perr := netip.ParseAddr(q.Server); perr == nil {
			return []string{net.JoinHostPort(addr.String(), q.Port)}, nil, nil
		}
		rctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		ips, rerr := lib.ResolveName(rctx, q.Server)
		if rerr != nil {
			return nil, nil, rerr
		}
		for _, ip := range ips {
			servers = append(servers, net.JoinHostPort(ip, q.Port))
		}
		return servers, ips, nil
	}
	all, serr := systemDNSServers(ctx)
	if serr != nil {
		return nil, nil, serr
	}
	for _, s := range all {
		host, _, herr := net.SplitHostPort(s)
		if herr != nil {
			continue
		}
		if ip := net.ParseIP(host); ip != nil && !lib.FamilyAllows(ip) {
			continue
		}
		servers = append(servers, s)
	}
	if len(servers) == 0 {
		return nil, nil, fmt.Errorf("none of this machine's DNS servers (%s) is an %s address", strings.Join(all, ", "), lib.FamilyName())
	}
	return servers, nil, nil
}

// buildDNSQuery makes the question. The ID is random so a reply that was not
// made for this question can be told from the one that was.
func buildDNSQuery(name string, qtype dnsmessage.Type, recurse, edns bool) (dnsmessage.Message, error) {
	dnsName, err := dnsmessage.NewName(name)
	if err != nil {
		return dnsmessage.Message{}, fmt.Errorf("invalid name %q: %w", name, err)
	}
	var id [2]byte
	if _, err := rand.Read(id[:]); err != nil {
		return dnsmessage.Message{}, err
	}
	msg := dnsmessage.Message{
		Header:    dnsmessage.Header{ID: binary.BigEndian.Uint16(id[:]), RecursionDesired: recurse},
		Questions: []dnsmessage.Question{{Name: dnsName, Type: qtype, Class: dnsmessage.ClassINET}},
	}
	if edns {
		opt := dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName(".")}, Body: &dnsmessage.OPTResource{}}
		if err := opt.Header.SetEDNS0(ednsPayload, dnsmessage.RCodeSuccess, false); err != nil {
			return dnsmessage.Message{}, err
		}
		msg.Additionals = []dnsmessage.Resource{opt}
	}
	return msg, nil
}

// answersQuestion says why reply is not the answer to req, or nil if it is: it
// must carry the same ID, be a response, and repeat the question (a server that
// cannot make sense of a query may leave the question out of an error reply).
func answersQuestion(req, reply dnsmessage.Message) error {
	switch {
	case reply.ID != req.ID:
		return errors.New("wrong ID")
	case !reply.Response:
		return errors.New("not a response")
	case len(reply.Questions) == 0:
		if reply.RCode == dnsmessage.RCodeSuccess {
			return errors.New("no question repeated")
		}
		return nil
	}
	q, want := reply.Questions[0], req.Questions[0]
	if q.Type != want.Type || q.Class != want.Class || !strings.EqualFold(q.Name.String(), want.Name.String()) {
		return errors.New("a different question")
	}
	return nil
}

// udpExchange sends the query and waits for its answer. The socket is connected,
// so the kernel already drops datagrams from anywhere but the server; anything
// that still arrives and is not the answer to this question (a wrong ID, a
// different question, garbage) is ignored and the wait goes on - a forged or
// stray packet must not end the query - and counted, so a timeout can say so.
func udpExchange(ctx context.Context, server string, req dnsmessage.Message, packed []byte, timeout time.Duration) (dnsmessage.Message, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, lib.DialNetwork("udp"), server)
	if err != nil {
		return dnsmessage.Message{}, err
	}
	defer func() { _ = conn.Close() }()
	stop := watchCancel(ctx, conn)
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(packed); err != nil {
		return dnsmessage.Message{}, err
	}
	buf := make([]byte, 65535)
	ignored := 0
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if ignored > 0 {
				return dnsmessage.Message{}, fmt.Errorf("%s (ignored %d reply(ies) that did not answer this question)", shortErr(err), ignored)
			}
			return dnsmessage.Message{}, err
		}
		var reply dnsmessage.Message
		if reply.Unpack(buf[:n]) != nil || answersQuestion(req, reply) != nil {
			ignored++
			continue
		}
		return reply, nil
	}
}

// tcpExchange is the same over TCP, which needs no forgery check (a stream from
// the server cannot be joined by a stranger's packets), so a reply that is wrong
// is an error, not something to wait past.
func tcpExchange(ctx context.Context, server string, req dnsmessage.Message, packed []byte, timeout time.Duration) (dnsmessage.Message, error) {
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dctx, lib.DialNetwork("tcp"), server)
	if err != nil {
		return dnsmessage.Message{}, err
	}
	defer func() { _ = conn.Close() }()
	stop := watchCancel(ctx, conn)
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	frame := make([]byte, 2, 2+len(packed))
	binary.BigEndian.PutUint16(frame, uint16(len(packed)))
	if _, err := conn.Write(append(frame, packed...)); err != nil {
		return dnsmessage.Message{}, err
	}
	var size [2]byte
	if _, err := io.ReadFull(conn, size[:]); err != nil {
		return dnsmessage.Message{}, err
	}
	body := make([]byte, binary.BigEndian.Uint16(size[:]))
	if _, err := io.ReadFull(conn, body); err != nil {
		return dnsmessage.Message{}, err
	}
	var reply dnsmessage.Message
	if err := reply.Unpack(body); err != nil {
		return dnsmessage.Message{}, fmt.Errorf("malformed reply: %w", err)
	}
	if err := answersQuestion(req, reply); err != nil {
		return dnsmessage.Message{}, fmt.Errorf("the reply does not answer the question: %w", err)
	}
	return reply, nil
}

// shortErr is the cause of a network error without the "read udp
// 192.168.1.5:54966->192.0.2.55:53:" wrapper Go puts in front of it: the line it
// appears on already names the server. "i/o timeout", "connection refused".
func shortErr(err error) string {
	var op *net.OpError
	if errors.As(err, &op) && op.Err != nil {
		msg := op.Err.Error()
		for _, prefix := range []string{"read: ", "write: ", "connect: ", "dial: "} {
			msg = strings.TrimPrefix(msg, prefix)
		}
		return msg
	}
	return err.Error()
}

// dnsAnswer is what one question got.
type dnsAnswer struct {
	Msg        dnsmessage.Message
	Server     string
	Transport  string
	RetriedTCP bool
	Skipped    []string
	Took       time.Duration
	Err        error // set when no server answered
}

// exchangeOne asks one server. Over UDP unless --tcp; a truncated UDP answer is
// asked again over TCP. A server that rejects the EDNS option (FORMERR, NOTIMP,
// BADVERS: some old or odd servers do) is asked again without it.
func exchangeOne(ctx context.Context, server, name string, qtype dnsmessage.Type, opt DNSOptions, timeout time.Duration) (msg dnsmessage.Message, transport string, retried bool, err error) {
	edns := true
	for {
		req, berr := buildDNSQuery(name, qtype, !opt.NoRecurse, edns)
		if berr != nil {
			return msg, "", false, berr
		}
		packed, perr := req.Pack()
		if perr != nil {
			return msg, "", false, perr
		}
		transport, retried = "udp", false
		if opt.ForceTCP {
			transport = "tcp"
			msg, err = tcpExchange(ctx, server, req, packed, timeout)
		} else {
			msg, err = udpExchange(ctx, server, req, packed, timeout)
			if err == nil && msg.Truncated {
				transport, retried = "tcp", true
				msg, err = tcpExchange(ctx, server, req, packed, timeout)
			}
		}
		if err != nil {
			return msg, transport, retried, err
		}
		if edns && rejectsEDNS(msg) {
			edns = false
			continue
		}
		return msg, transport, retried, nil
	}
}

// rejectsEDNS reports whether an answer says "I do not understand EDNS".
func rejectsEDNS(msg dnsmessage.Message) bool {
	if msg.RCode == dnsmessage.RCodeFormatError || msg.RCode == dnsmessage.RCodeNotImplemented {
		return true
	}
	for _, r := range msg.Additionals {
		if r.Header.Type == dnsmessage.TypeOPT && r.Header.ExtendedRCode(msg.RCode) == 16 { // BADVERS
			return true
		}
	}
	return false
}

// queryServers asks the servers in order and stops at the first that answers at
// all - any response code counts: NXDOMAIN is an answer. The servers that did
// not answer (no reply in time, connection refused) are listed with why. This is
// what a resolver does, and what dig does.
func queryServers(ctx context.Context, servers []string, name string, qtype dnsmessage.Type, opt DNSOptions, timeout time.Duration) dnsAnswer {
	var skipped []string
	for _, server := range servers {
		if ctx.Err() != nil {
			return dnsAnswer{Err: ctx.Err()}
		}
		began := time.Now()
		msg, transport, retried, err := exchangeOne(ctx, server, name, qtype, opt, timeout)
		if err == nil {
			return dnsAnswer{Msg: msg, Server: server, Transport: transport, RetriedTCP: retried, Skipped: skipped, Took: time.Since(began)}
		}
		if ctx.Err() != nil {
			return dnsAnswer{Err: ctx.Err()}
		}
		skipped = append(skipped, fmt.Sprintf("%s (%s)", server, shortErr(err)))
	}
	return dnsAnswer{Err: fmt.Errorf("no DNS server answered: %s", strings.Join(skipped, ", ")), Skipped: skipped}
}

// recordOf writes a resource record the way dig does.
func recordOf(r dnsmessage.Resource) lib.DNSRecord {
	rec := lib.DNSRecord{Name: r.Header.Name.String(), Type: typeName(r.Header.Type), TTL: r.Header.TTL}
	switch b := r.Body.(type) {
	case *dnsmessage.AResource:
		rec.Data = net.IP(b.A[:]).String()
	case *dnsmessage.AAAAResource:
		rec.Data = net.IP(b.AAAA[:]).String()
	case *dnsmessage.CNAMEResource:
		rec.Data = b.CNAME.String()
	case *dnsmessage.NSResource:
		rec.Data = b.NS.String()
	case *dnsmessage.PTRResource:
		rec.Data = b.PTR.String()
	case *dnsmessage.MXResource:
		rec.Data = fmt.Sprintf("%d %s", b.Pref, b.MX)
	case *dnsmessage.SRVResource:
		rec.Data = fmt.Sprintf("%d %d %d %s", b.Priority, b.Weight, b.Port, b.Target)
	case *dnsmessage.SOAResource:
		rec.Data = fmt.Sprintf("%s %s %d %d %d %d %d", b.NS, b.MBox, b.Serial, b.Refresh, b.Retry, b.Expire, b.MinTTL)
	case *dnsmessage.TXTResource:
		rec.Strings = append([]string{}, b.TXT...)
		rec.Data = strings.Join(b.TXT, "")
	case *dnsmessage.UnknownResource:
		if r.Header.Type == typeCAA {
			rec.Data = caaText(b.Data)
		} else {
			rec.Data = fmt.Sprintf("\\# %d %s", len(b.Data), hex.EncodeToString(b.Data))
		}
	default:
		rec.Data = "?"
	}
	return rec
}

// caaText writes a CAA record as flags, tag and value: 0 issue "letsencrypt.org".
func caaText(data []byte) string {
	if len(data) < 2 || 2+int(data[1]) > len(data) {
		return fmt.Sprintf("\\# %d %s", len(data), hex.EncodeToString(data))
	}
	tagLen := int(data[1])
	return fmt.Sprintf("%d %s %s", data[0], string(data[2:2+tagLen]), strconv.Quote(string(data[2+tagLen:])))
}

// records converts a section, leaving out the EDNS pseudo-record.
func records(section []dnsmessage.Resource) []lib.DNSRecord {
	out := make([]lib.DNSRecord, 0, len(section))
	for _, r := range section {
		if r.Header.Type != dnsmessage.TypeOPT {
			out = append(out, recordOf(r))
		}
	}
	return out
}

var rcodeNames = map[dnsmessage.RCode]string{
	dnsmessage.RCodeSuccess: "NOERROR", dnsmessage.RCodeFormatError: "FORMERR", dnsmessage.RCodeServerFailure: "SERVFAIL",
	dnsmessage.RCodeNameError: "NXDOMAIN", dnsmessage.RCodeNotImplemented: "NOTIMP", dnsmessage.RCodeRefused: "REFUSED",
}

func rcodeName(c dnsmessage.RCode) string {
	if n, ok := rcodeNames[c]; ok {
		return n
	}
	return "RCODE" + strconv.Itoa(int(c))
}

func headerFlags(h dnsmessage.Header) []string {
	flags := []string{}
	for _, f := range []struct {
		on   bool
		name string
	}{{h.Response, "qr"}, {h.Authoritative, "aa"}, {h.Truncated, "tc"}, {h.RecursionDesired, "rd"}, {h.RecursionAvailable, "ra"}, {h.AuthenticData, "ad"}, {h.CheckingDisabled, "cd"}} {
		if f.on {
			flags = append(flags, f.name)
		}
	}
	return flags
}

// verdict decides whether an answer is a success and, if not, says why in words
// that name what is wrong rather than repeating the code.
func verdict(a dnsAnswer, qtype dnsmessage.Type, name string) (ok bool, reason string) {
	switch a.Msg.RCode {
	case dnsmessage.RCodeSuccess:
	case dnsmessage.RCodeNameError:
		return false, "no such domain (NXDOMAIN): " + name + " does not exist"
	case dnsmessage.RCodeServerFailure:
		return false, "the server failed to answer (SERVFAIL)"
	case dnsmessage.RCodeRefused:
		return false, "the server refused to answer (REFUSED)"
	default:
		return false, "the server answered with " + rcodeName(a.Msg.RCode)
	}
	cname := false
	for _, r := range a.Msg.Answers {
		if r.Header.Type == qtype {
			return true, ""
		}
		if r.Header.Type == dnsmessage.TypeCNAME {
			cname = true
		}
	}
	if cname {
		return false, "no " + typeName(qtype) + " records: " + name + " is an alias (CNAME) whose target has none"
	}
	return false, "no " + typeName(qtype) + " records: " + name + " exists but has none of this type"
}

// DNSHandler asks the question(s) in q, iterations times, and shows the answers:
// for each question one line about the exchange (server, transport, response
// code, header flags, counts, time) and one per record in the answer and
// authority sections. --timeout is how long each server gets; a server that
// does not answer is skipped for the next. It reports true only if every
// question was answered with at least one record of the type asked: NXDOMAIN, an
// empty answer, SERVFAIL, REFUSED, a malformed reply or no reply at all are all
// failed checks, each with its reason.
// ctx is cancelled by Ctrl+C and nothing else - never a deadline; see
// interrupt.go for how a cancelled run ends.
func DNSHandler(ctx context.Context, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, q DNSQuery, opt DNSOptions) (ok bool) {
	types := make([]string, 0, len(q.Types))
	for _, t := range q.Types {
		types = append(types, typeName(t))
	}
	port, _ := strconv.Atoi(q.Port)
	output := lib.LocalJSONOutput{ModuleName: dnsModule, Stats: make([]lib.DNSStats, 0)}
	output.InputParams = lib.InputParams{
		Mode:     dnsModule,
		Host:     q.Name,
		FromPort: port,
		ToPort:   port,
		Protocol: "dns",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Throttle: *throttle,
		Data:     strings.Join(types, ","),
	}
	istart := time.Now()
	output.StartTime = istart.UnixMicro()

	finishEarly := func(text string, err error) bool {
		if *jsonoutput {
			output.Error = err.Error()
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		} else {
			fmt.Println(lib.LogWithTimestamp(dnsModule, text+" "+lib.Fields("error", err.Error(), "time", time.Since(istart)), true))
		}
		return false
	}

	servers, resolved, err := serverList(ctx, q, timeout)
	if err != nil {
		if q.Server != "" {
			return finishEarly("dns resolution failed "+lib.Fields("host", q.Server), err)
		}
		return finishEarly("no server to ask", err)
	}
	if !*jsonoutput && len(resolved) > 0 {
		fmt.Println(lib.LogWithTimestamp(dnsModule, "server resolved "+lib.Fields("host", q.Server, "addresses", len(resolved), "ips", "["+strings.Join(resolved, ",")+"]", "time", time.Since(istart)), false))
	}

	var answered, completed int
attempts:
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		for _, qtype := range q.Types {
			if ctx.Err() != nil || !pause(ctx, attemptDelay(delay, *throttle)) {
				break attempts
			}
			begin := time.Now()
			a := queryServers(ctx, servers, q.Name, qtype, opt, time.Duration(timeout)*time.Second)
			if a.Err != nil && ctx.Err() != nil {
				break attempts // cut off by Ctrl+C: never finished, so it neither answered nor failed
			}
			completed++

			stat := lib.DNSStats{Name: q.Name, Type: typeName(qtype), Flags: []string{}, Answers: []lib.DNSRecord{}, Authority: []lib.DNSRecord{},
				Skipped: a.Skipped, SentTime: begin.UnixMicro(), RecvTime: time.Now().UnixMicro(), TimeTaken: time.Since(begin).Microseconds()}
			var success bool
			var reason string
			if a.Err != nil {
				reason = a.Err.Error()
			} else {
				stat.Nameserver, stat.Transport, stat.RetriedOverTCP = a.Server, a.Transport, a.RetriedTCP
				stat.RCode, stat.Flags = rcodeName(a.Msg.RCode), headerFlags(a.Msg.Header)
				stat.Answers, stat.Authority = records(a.Msg.Answers), records(a.Msg.Authorities)
				stat.Additional = len(records(a.Msg.Additionals))
				stat.TimeTaken = a.Took.Microseconds()
				success, reason = verdict(a, qtype, q.Name)
			}
			stat.Success = success
			if success {
				answered++
			} else {
				stat.Error = reason
			}

			if *jsonoutput {
				output.Stats = append(output.Stats.([]lib.DNSStats), stat)
				continue
			}
			printDNSStat(stat, attempt, iterations)
		}
	}

	planned := iterations * len(q.Types)
	interrupted := completed < planned // only Ctrl+C keeps an attempt from being made
	if *jsonoutput {
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		if interrupted {
			output.Error = interruptedNote(completed, planned)
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
	} else {
		queries := planned
		if interrupted {
			fmt.Println(interruptedLine(dnsModule, completed, planned, istart))
			queries = completed
		}
		fmt.Println(lib.LogWithTimestamp(dnsModule, "done "+lib.Fields("queries", queries, "answered", answered, "total_time", time.Since(istart)), false))
	}
	return answered == planned && !interrupted
}

// printDNSStat writes one question's lines: the exchange, then its records.
func printDNSStat(s lib.DNSStats, attempt, iterations int) {
	where := []any{"name", s.Name, "type", s.Type}
	if s.Nameserver != "" {
		where = append(where, "nameserver", s.Nameserver, "transport", s.Transport)
	}
	where = append(where, "attempt", fmt.Sprintf("%d/%d", attempt, iterations))
	if s.RCode != "" {
		// Lower case in the log: NOERROR would contain "ERROR", and an ERROR line means a failed check.
		where = append(where, "rcode", strings.ToLower(s.RCode), "flags", "["+strings.Join(s.Flags, ",")+"]", "answers", len(s.Answers), "authority", len(s.Authority), "additional", s.Additional)
	}
	if s.RetriedOverTCP {
		where = append(where, "retried", "tcp")
	}
	if len(s.Skipped) > 0 && s.Nameserver != "" {
		where = append(where, "skipped", strings.Join(s.Skipped, ", "))
	}
	where = append(where, "time", time.Duration(s.TimeTaken)*time.Microsecond)
	if s.Success {
		fmt.Println(lib.LogWithTimestamp(dnsModule, "query "+lib.Fields(where...), false))
	} else {
		fmt.Println(lib.LogWithTimestamp(dnsModule, "query failed "+lib.Fields(append(where, "error", s.Error)...), true))
	}
	for _, section := range []struct {
		label   string
		records []lib.DNSRecord
	}{{"answer", s.Answers}, {"authority", s.Authority}} {
		for _, r := range section.records {
			fmt.Println(lib.LogWithTimestamp(dnsModule, section.label+" "+lib.Fields("name", r.Name, "type", r.Type, "ttl", r.TTL, "data", escapeBytes([]byte(r.Data))), false))
		}
	}
}
