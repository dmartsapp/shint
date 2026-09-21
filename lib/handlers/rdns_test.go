package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// withLookup swaps the reverse resolver for the duration of a test.
func withLookup(t *testing.T, fn func(ctx context.Context, address string) ([]string, error)) {
	t.Helper()
	old := lookupAddr
	lookupAddr = fn
	t.Cleanup(func() { lookupAddr = old })
}

func TestReverseName(t *testing.T) {
	for in, want := range map[string]string{
		"192.0.2.1":       "1.2.0.192.in-addr.arpa.",
		"8.8.8.8":         "8.8.8.8.in-addr.arpa.",
		"127.0.0.1":       "1.0.0.127.in-addr.arpa.",
		"0.0.0.0":         "0.0.0.0.in-addr.arpa.",
		"::ffff:10.1.2.3": "3.2.1.10.in-addr.arpa.", // looked up as the IPv4 address it wraps
		// the example in RFC 3596 section 2.5
		"4321:0:1:2:3:4:567:89ab": "b.a.9.8.7.6.5.0.4.0.0.0.3.0.0.0.2.0.0.0.1.0.0.0.0.0.0.0.1.2.3.4.ip6.arpa.",
		"::1":                     "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa.",
	} {
		if got := ReverseName(netip.MustParseAddr(in)); got != want {
			t.Errorf("ReverseName(%s)\n got %s\nwant %s", in, got, want)
		}
	}
}

func rdns(t *testing.T, json bool, iterations int, timeout int, host string) (string, bool) {
	t.Helper()
	throttle := false
	var ok bool
	out := captureStdout(t, func() { ok = RDNSHandler(context.Background(), &json, iterations, 0, &throttle, timeout, host) })
	return out, ok
}

func TestRDNSHandlerReportsTheNames(t *testing.T) {
	withLookup(t, func(_ context.Context, address string) ([]string, error) {
		if address != "192.0.2.10" {
			t.Errorf("looked up %q", address)
		}
		return []string{"host-a.example.", "host-b.example."}, nil
	})
	out, ok := rdns(t, false, 1, 2, "192.0.2.10")
	if !ok {
		t.Errorf("reported failure:\n%s", out)
	}
	for _, want := range []string{
		"[rdns] OK dns resolved host=192.0.2.10 addresses=1 ips=[192.0.2.10]",
		"[rdns] OK reverse lookup address=192.0.2.10 query=10.2.0.192.in-addr.arpa. attempt=1/1 names=[host-a.example.,host-b.example.]",
		"[rdns] OK done lookups=1 resolved=1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestRDNSHandlerAnAddressWithoutANameIsAFailedCheck(t *testing.T) {
	notFound := &net.DNSError{Err: "no such host", Name: "10.2.0.192.in-addr.arpa.", IsNotFound: true}
	for name, fn := range map[string]func(context.Context, string) ([]string, error){
		"NXDOMAIN":     func(context.Context, string) ([]string, error) { return nil, notFound },
		"empty answer": func(context.Context, string) ([]string, error) { return nil, nil },
	} {
		withLookup(t, fn)
		out, ok := rdns(t, false, 1, 2, "192.0.2.10")
		if ok || !strings.Contains(out, "[rdns] ERROR reverse lookup failed address=192.0.2.10") || !strings.Contains(out, "[rdns] OK done lookups=1 resolved=0") {
			t.Errorf("%s: ok=%v\n%s", name, ok, out)
		}
	}
}

// Some names plus an error (the resolver dropped a malformed record) still has
// names to report.
func TestRDNSHandlerPartialAnswerCounts(t *testing.T) {
	withLookup(t, func(context.Context, string) ([]string, error) {
		return []string{"good.example."}, errors.New("some records were malformed")
	})
	if out, ok := rdns(t, false, 1, 2, "192.0.2.10"); !ok || !strings.Contains(out, "names=[good.example.]") {
		t.Errorf("ok=%v\n%s", ok, out)
	}
}

func TestRDNSHandlerCountAndTimeoutPerLookup(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	withLookup(t, func(ctx context.Context, address string) ([]string, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 2 { // the second lookup hangs until its own timeout
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []string{"ok.example."}, nil
	})
	start := time.Now()
	out, ok := rdns(t, false, 3, 1, "192.0.2.10")
	if calls != 3 {
		t.Errorf("%d lookups, want 3 (--count 3)", calls)
	}
	if ok {
		t.Error("one timed-out lookup must fail the run")
	}
	if took := time.Since(start); took < 900*time.Millisecond || took > 3*time.Second {
		t.Errorf("--timeout 1 bounded one lookup, yet the run took %v", took)
	}
	if strings.Count(out, "reverse lookup failed") != 1 || !strings.Contains(out, "attempt=3/3") || !strings.Contains(out, "resolved=2") {
		t.Errorf("the run must go on after a failed lookup:\n%s", out)
	}
}

func TestRDNSHandlerJSON(t *testing.T) {
	withLookup(t, func(_ context.Context, address string) ([]string, error) {
		if address == "::1" {
			return []string{"localhost."}, nil
		}
		return nil, &net.DNSError{Err: "no such host", IsNotFound: true}
	})
	out, ok := rdns(t, true, 1, 2, "::1")
	if !ok {
		t.Errorf("reported failure:\n%s", out)
	}
	var doc struct {
		ModuleName string          `json:"module_name"`
		DNS        lib.DNSLookup   `json:"dns_lookup"`
		Stats      []lib.RDNSStats `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	if doc.ModuleName != "rdns" || !doc.DNS.Success || len(doc.Stats) != 1 {
		t.Fatalf("document: %+v", doc)
	}
	s := doc.Stats[0]
	if !s.Success || s.Address != "::1" || len(s.Names) != 1 || s.Names[0] != "localhost." || !strings.HasSuffix(s.Query, ".ip6.arpa.") || s.Error != "" {
		t.Errorf("stat = %+v", s)
	}

	out, ok = rdns(t, true, 1, 2, "192.0.2.10")
	if err := json.Unmarshal([]byte(out), &doc); err != nil || ok {
		t.Fatalf("a failed lookup under --json is one document (ok=%v, err=%v):\n%s", ok, err, out)
	}
	if s := doc.Stats[0]; s.Success || s.Error == "" || s.Names == nil || len(s.Names) != 0 {
		t.Errorf("failed stat = %+v (names must be [], not null)", s)
	}
}

func TestRDNSHandlerForwardLookupFailure(t *testing.T) {
	withLookup(t, func(context.Context, string) ([]string, error) {
		t.Error("no reverse lookup after a failed name lookup")
		return nil, nil
	})
	out, ok := rdns(t, false, 1, 2, "no-such-host.invalid")
	if ok || !strings.Contains(out, "[rdns] ERROR dns resolution failed") {
		t.Errorf("ok=%v\n%s", ok, out)
	}
}
