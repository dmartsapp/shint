package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// fakeClock lets a test move time by exactly the amounts it chooses.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newFakeRecorder(u string) (*timingRecorder, *fakeClock, *httptrace.ClientTrace) {
	clock := &fakeClock{t: time.Unix(1_000_000, 0)}
	rec := newTimingRecorder(u)
	rec.now = clock.now
	return rec, clock, rec.trace()
}

// The breakdown is arithmetic on the moments the trace reports; with a clock
// the test controls, every figure is exact.
func TestTimingRecorderBreakdown(t *testing.T) {
	rec, clock, tr := newFakeRecorder("https://example.test/")
	tr.GetConn("example.test:443")
	clock.advance(1 * time.Millisecond)
	tr.DNSStart(httptrace.DNSStartInfo{})
	clock.advance(4 * time.Millisecond)
	tr.DNSDone(httptrace.DNSDoneInfo{})
	tr.ConnectStart("tcp", "192.0.2.1:443")
	clock.advance(10 * time.Millisecond)
	tr.ConnectDone("tcp", "192.0.2.1:443", nil)
	tr.TLSHandshakeStart()
	clock.advance(20 * time.Millisecond)
	tr.TLSHandshakeDone(tls.ConnectionState{}, nil)
	tr.GotConn(httptrace.GotConnInfo{Reused: false})
	tr.WroteRequest(httptrace.WroteRequestInfo{})
	clock.advance(30 * time.Millisecond)
	tr.GotFirstResponseByte()
	clock.advance(5 * time.Millisecond)
	rec.finish(200)

	hops := rec.hopTimings()
	if len(hops) != 1 {
		t.Fatalf("%d hops, want 1", len(hops))
	}
	want := lib.WebHopTiming{URL: "https://example.test/", StatusCode: 200, Reused: false,
		DNSUs: 4000, ConnectUs: 10000, TLSUs: 20000, WaitUs: 30000, DownloadUs: 5000, TotalUs: 70000}
	if hops[0] != want {
		t.Errorf("\n got %+v\nwant %+v", hops[0], want)
	}
}

func TestTimingRecorderReusedConnectionHasNoSetupTime(t *testing.T) {
	rec, clock, tr := newFakeRecorder("http://example.test/")
	tr.GetConn("example.test:80")
	tr.GotConn(httptrace.GotConnInfo{Reused: true, WasIdle: true})
	tr.WroteRequest(httptrace.WroteRequestInfo{})
	clock.advance(12 * time.Millisecond)
	tr.GotFirstResponseByte()
	rec.finish(204)
	h := rec.hopTimings()[0]
	if !h.Reused || h.DNSUs != 0 || h.ConnectUs != 0 || h.TLSUs != 0 || h.WaitUs != 12000 || h.StatusCode != 204 {
		t.Errorf("hop = %+v", h)
	}
}

func TestTimingRecorderFollowsRedirects(t *testing.T) {
	rec, clock, tr := newFakeRecorder("http://a.test/")
	tr.GetConn("a.test:80")
	tr.WroteRequest(httptrace.WroteRequestInfo{})
	clock.advance(10 * time.Millisecond)
	tr.GotFirstResponseByte()
	clock.advance(2 * time.Millisecond)
	// the client's CheckRedirect runs; the redirect body is drained; the next hop begins
	next, _ := url.Parse("https://b.test/landing")
	rec.redirected(&http.Request{URL: next, Response: &http.Response{StatusCode: 301}})
	clock.advance(1 * time.Millisecond)
	tr.GetConn("b.test:443")
	tr.WroteRequest(httptrace.WroteRequestInfo{})
	clock.advance(20 * time.Millisecond)
	tr.GotFirstResponseByte()
	clock.advance(3 * time.Millisecond)
	rec.finish(200)

	hops := rec.hopTimings()
	if len(hops) != 2 {
		t.Fatalf("%d hops, want 2", len(hops))
	}
	if hops[0].URL != "http://a.test/" || hops[0].StatusCode != 301 || hops[0].WaitUs != 10000 || hops[0].TotalUs != 13000 {
		t.Errorf("hop 1 = %+v", hops[0])
	}
	if hops[1].URL != "https://b.test/landing" || hops[1].StatusCode != 200 || hops[1].WaitUs != 20000 || hops[1].DownloadUs != 3000 || hops[1].TotalUs != 23000 {
		t.Errorf("hop 2 = %+v", hops[1])
	}
}

func TestTimingRecorderKeepsTheTimeSpentBeforeAFailure(t *testing.T) {
	rec, clock, tr := newFakeRecorder("http://down.test/")
	tr.GetConn("down.test:80")
	tr.DNSStart(httptrace.DNSStartInfo{})
	clock.advance(3 * time.Millisecond)
	tr.DNSDone(httptrace.DNSDoneInfo{})
	tr.ConnectStart("tcp", "192.0.2.9:80")
	clock.advance(7 * time.Millisecond)
	tr.ConnectDone("tcp", "192.0.2.9:80", fmt.Errorf("connection refused"))
	rec.abort()
	h := rec.hopTimings()[0]
	if h.StatusCode != 0 || h.DNSUs != 3000 || h.ConnectUs != 7000 || h.WaitUs != 0 || h.DownloadUs != 0 || h.TotalUs != 10000 {
		t.Errorf("hop = %+v", h)
	}
}

// A dual-stack name can be tried on more than one address: connect time runs
// from the first attempt until the one that succeeds.
func TestTimingRecorderConnectSpansFailedAddresses(t *testing.T) {
	rec, clock, tr := newFakeRecorder("http://dual.test/")
	tr.GetConn("dual.test:80")
	tr.ConnectStart("tcp", "[2001:db8::1]:80")
	clock.advance(50 * time.Millisecond)
	tr.ConnectDone("tcp", "[2001:db8::1]:80", fmt.Errorf("network is unreachable"))
	tr.ConnectStart("tcp", "192.0.2.1:80")
	clock.advance(8 * time.Millisecond)
	tr.ConnectDone("tcp", "192.0.2.1:80", nil)
	tr.ConnectDone("tcp", "192.0.2.2:80", fmt.Errorf("late loser")) // must not move a success
	rec.abort()
	if got := rec.hopTimings()[0].ConnectUs; got != 58000 {
		t.Errorf("connect = %dµs, want 58000", got)
	}
}

// ---- against real servers

func webJSON(t *testing.T, count, delay int, target string, tlsConfig *tls.Config, timing bool) []lib.WebStats {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, count, delay, &throttle, 5, u, "GET", "", nil, false, tlsConfig, timing)
	})
	var doc struct {
		Stats []lib.WebStats `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	return doc.Stats
}

func TestWebTimingSlowServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond) // the server thinks first ...
		_, _ = w.Write([]byte("first"))
		w.(http.Flusher).Flush()
		time.Sleep(120 * time.Millisecond) // ... then the body trickles in
		_, _ = w.Write([]byte("second"))
	}))
	defer srv.Close()

	stats := webJSON(t, 1, 0, srv.URL, nil, true)
	if len(stats) != 1 || stats[0].Timing == nil || len(stats[0].Timing.Hops) != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	h := stats[0].Timing.Hops[0]
	if h.StatusCode != 200 || h.Reused || h.URL != srv.URL {
		t.Errorf("hop = %+v", h)
	}
	if h.WaitUs < 140_000 || h.WaitUs > 600_000 {
		t.Errorf("wait = %dµs, want about 150ms", h.WaitUs)
	}
	if h.DownloadUs < 100_000 || h.DownloadUs > 600_000 {
		t.Errorf("download = %dµs, want about 120ms", h.DownloadUs)
	}
	if h.DNSUs != 0 || h.TLSUs != 0 {
		t.Errorf("an IP address over http needs no DNS or TLS: %+v", h)
	}
	if h.TotalUs < h.WaitUs+h.DownloadUs {
		t.Errorf("total %d is less than wait + download %d", h.TotalUs, h.WaitUs+h.DownloadUs)
	}
	// the parts account for the whole, give or take the trace's own bookkeeping
	if sum := h.DNSUs + h.ConnectUs + h.TLSUs + h.WaitUs + h.DownloadUs; h.TotalUs-sum > 50_000 || sum > h.TotalUs {
		t.Errorf("phases sum to %dµs of %dµs total: %+v", sum, h.TotalUs, h)
	}
}

func TestWebTimingHTTPSShowsTheHandshake(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	cfg := srv.Client().Transport.(*http.Transport).TLSClientConfig
	stats := webJSON(t, 1, 0, srv.URL, cfg, true)
	h := stats[0].Timing.Hops[0]
	if h.TLSUs <= 0 || h.StatusCode != 200 || h.Reused {
		t.Errorf("hop = %+v, want a TLS handshake time", h)
	}
}

func TestWebTimingFollowsARedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("landed")) }))
	defer final.Close()
	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/landing", http.StatusFound)
	}))
	defer start.Close()

	hops := webJSON(t, 1, 0, start.URL, nil, true)[0].Timing.Hops
	if len(hops) != 2 {
		t.Fatalf("%d hops, want 2: %+v", len(hops), hops)
	}
	if hops[0].URL != start.URL || hops[0].StatusCode != 302 || hops[1].URL != final.URL+"/landing" || hops[1].StatusCode != 200 {
		t.Errorf("hops = %+v", hops)
	}
	for i, h := range hops {
		if h.Reused || h.TotalUs <= 0 {
			t.Errorf("hop %d = %+v (two different servers: two new connections)", i+1, h)
		}
	}
}

func TestWebTimingSecondRequestReusesTheConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	stats := webJSON(t, 2, 300, srv.URL, nil, true) // the first finishes long before the second starts
	if len(stats) != 2 {
		t.Fatalf("%d stats, want 2", len(stats))
	}
	reused := 0
	for _, s := range stats {
		h := s.Timing.Hops[0]
		if h.Reused {
			reused++
			if h.DNSUs != 0 || h.ConnectUs != 0 || h.TLSUs != 0 {
				t.Errorf("a reused connection has no setup time: %+v", h)
			}
		}
	}
	if reused != 1 {
		t.Errorf("%d requests reused the connection, want exactly the second", reused)
	}
}

func TestWebTimingKeepsTheTimeSpentBeforeAFailure(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	target := closed.URL
	closed.Close() // now nothing listens there
	stats := webJSON(t, 1, 0, target, nil, true)
	if len(stats) != 1 || stats[0].Success {
		t.Fatalf("stats = %+v", stats)
	}
	if stats[0].Timing == nil || len(stats[0].Timing.Hops) != 1 || stats[0].Timing.Hops[0].StatusCode != 0 || stats[0].Timing.Hops[0].TotalUs <= 0 {
		t.Errorf("a failed request still reports its timing: %+v", stats[0].Timing)
	}
}

// --timing installs its own redirect policy; it must be net/http's default
// (stop after 10) and nothing else, so the same server sees the same number of
// requests with and without the flag.
func TestWebTimingKeepsTheDefaultRedirectLimit(t *testing.T) {
	var srv *httptest.Server
	var hits atomic.Int32
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()

	without := webJSON(t, 1, 0, srv.URL, nil, false)
	hitsWithout := hits.Swap(0)
	with := webJSON(t, 1, 0, srv.URL, nil, true)

	for name, s := range map[string]lib.WebStats{"without --timing": without[0], "with --timing": with[0]} {
		if s.Success || !strings.Contains(strings.Join(s.Errors, " "), "stopped after 10 redirects") {
			t.Errorf("%s: %+v", name, s)
		}
	}
	if got := hits.Load(); got != hitsWithout || got != 10 {
		t.Errorf("the server saw %d requests with --timing and %d without; net/http stops after 10", got, hitsWithout)
	}
	if n := len(with[0].Timing.Hops); n != 10 {
		t.Errorf("%d hops recorded for 10 requests", n)
	}
}

func TestWebTimingTextOutputAndOffByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	run := func(timing bool) string {
		jsonOutput, throttle := false, false
		return captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, u, "GET", "", nil, false, nil, timing)
		})
	}
	with := run(true)
	want := "[web] OK timing url=" + srv.URL + " hop=1/1 status=200 connection=new dns="
	if !strings.Contains(with, want) {
		t.Errorf("output lacks %q:\n%s", want, with)
	}
	for _, field := range []string{" connect=", " tls=", " wait=", " download=", " total=", " attempt=1/1"} {
		if !strings.Contains(with, field) {
			t.Errorf("timing line lacks %q:\n%s", field, with)
		}
	}
	if without := run(false); strings.Contains(without, "timing") {
		t.Errorf("no --timing, yet the output mentions it:\n%s", without)
	}
	if raw := webJSONRaw(t, srv.URL, false); strings.Contains(raw, `"timing"`) {
		t.Errorf("no --timing, yet the JSON has a timing key:\n%s", raw)
	}
}

func webJSONRaw(t *testing.T, target string, timing bool) string {
	t.Helper()
	u, _ := url.Parse(target)
	jsonOutput, throttle := true, false
	return captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, u, "GET", "", nil, false, nil, timing)
	})
}

// -4 and -6 must bind the HTTP client's own dialer too: web hands the address
// choice to net/http, which would otherwise wander into the other family.
func TestWebDialsOnlyTheRequestedFamily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close() // an IPv4 listener
	old := lib.NetworkType
	defer func() { lib.NetworkType = old }()

	lib.NetworkType = "ip4"
	if s := webJSON(t, 1, 0, srv.URL, nil, false); len(s) != 1 || !s[0].Success {
		t.Errorf("-4 against an IPv4 server should work: %+v", s)
	}
	lib.NetworkType = "ip6" // the server is on 127.0.0.1: an IPv6-only dialer cannot reach it
	if s := webJSON(t, 1, 0, srv.URL, nil, false); len(s) != 1 || s[0].Success {
		t.Errorf("-6 must not reach an IPv4 address: %+v", s)
	}
	lib.NetworkType = "ip"
	if s := webJSON(t, 1, 0, srv.URL, nil, false); len(s) != 1 || !s[0].Success {
		t.Errorf("no restriction should work: %+v", s)
	}
}
