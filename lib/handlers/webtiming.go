package handlers

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// timingKey carries a request's timingRecorder in its context, so the client's
// CheckRedirect (which is shared by every request) can find the right one.
type timingKey struct{}

// timingHop is the moments one hop of a request passed through, as the
// net/http trace reports them. A zero time means "did not happen" (no DNS on
// an IP address, no connect on a reused connection, no TLS over http://).
type timingHop struct {
	url    string
	status int
	reused bool

	start, end                time.Time
	dnsStart, dnsDone         time.Time
	connectStart, connectDone time.Time
	connectOK                 time.Time // set once a connect attempt succeeds
	tlsStart, tlsDone         time.Time
	wrote, firstByte          time.Time
}

// timingRecorder collects the trace of one request (one attempt): a hop per
// connection request, so a followed redirect adds a hop. The hooks run on
// several goroutines - the one that called Do, and the one that dials - so
// everything is under a mutex.
type timingRecorder struct {
	mu   sync.Mutex
	now  func() time.Time
	hops []*timingHop
	urls []string // the URL of each hop: the request's, then each redirect target
}

func newTimingRecorder(url string) *timingRecorder {
	return &timingRecorder{now: time.Now, urls: []string{url}}
}

// current runs f on the newest hop, if there is one yet.
func (r *timingRecorder) current(f func(h *timingHop)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.hops); n > 0 {
		f(r.hops[n-1])
	}
}

// trace is the hooks that feed the recorder.
func (r *timingRecorder) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		// A new hop starts each time the client asks for a connection; the
		// previous hop, if it was redirected, ends at that moment (its body
		// has been read and closed by then).
		GetConn: func(string) {
			r.mu.Lock()
			defer r.mu.Unlock()
			now := r.now()
			if n := len(r.hops); n > 0 && r.hops[n-1].end.IsZero() {
				r.hops[n-1].end = now
			}
			hop := &timingHop{start: now}
			if i := len(r.hops); i < len(r.urls) {
				hop.url = r.urls[i]
			}
			r.hops = append(r.hops, hop)
		},
		DNSStart: func(httptrace.DNSStartInfo) {
			r.current(func(h *timingHop) {
				if h.dnsStart.IsZero() {
					h.dnsStart = r.now()
				}
			})
		},
		DNSDone: func(httptrace.DNSDoneInfo) { r.current(func(h *timingHop) { h.dnsDone = r.now() }) },
		// A dual-stack name can be tried on several addresses: connect time is
		// from the first attempt to the one that succeeded (or the last to fail).
		ConnectStart: func(_, _ string) {
			r.current(func(h *timingHop) {
				if h.connectStart.IsZero() {
					h.connectStart = r.now()
				}
			})
		},
		ConnectDone: func(_, _ string, err error) {
			r.current(func(h *timingHop) {
				if h.connectOK.IsZero() {
					h.connectDone = r.now()
					if err == nil {
						h.connectOK = h.connectDone
					}
				}
			})
		},
		TLSHandshakeStart: func() { r.current(func(h *timingHop) { h.tlsStart = r.now() }) },
		TLSHandshakeDone:  func(tls.ConnectionState, error) { r.current(func(h *timingHop) { h.tlsDone = r.now() }) },
		GotConn:           func(info httptrace.GotConnInfo) { r.current(func(h *timingHop) { h.reused = info.Reused }) },
		WroteRequest:      func(httptrace.WroteRequestInfo) { r.current(func(h *timingHop) { h.wrote = r.now() }) },
		GotFirstResponseByte: func() {
			r.current(func(h *timingHop) {
				if h.firstByte.IsZero() {
					h.firstByte = r.now()
				}
			})
		},
	}
}

// redirected is called by the client's CheckRedirect with the request that
// follows a redirect: it names the next hop and records how the last one ended.
func (r *timingRecorder) redirected(next *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, next.URL.String())
	if n := len(r.hops); n > 0 && next.Response != nil {
		r.hops[n-1].status = next.Response.StatusCode
	}
}

// finish ends the last hop with the response it got: call it as soon as the
// body has been read.
func (r *timingRecorder) finish(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.hops); n > 0 {
		r.hops[n-1].end = r.now()
		r.hops[n-1].status = status
	}
}

// abort ends the last hop where the request failed (status stays 0).
func (r *timingRecorder) abort() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.hops); n > 0 && r.hops[n-1].end.IsZero() {
		r.hops[n-1].end = r.now()
	}
}

// span is end - start, or zero if either moment did not happen.
func span(start, end time.Time) time.Duration {
	if start.IsZero() || end.IsZero() {
		return 0
	}
	return end.Sub(start)
}

// hopTimings turns what was recorded into the reported breakdown.
func (r *timingRecorder) hopTimings() []lib.WebHopTiming {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]lib.WebHopTiming, 0, len(r.hops))
	for _, h := range r.hops {
		end := h.end
		if end.IsZero() {
			end = r.now()
		}
		out = append(out, lib.WebHopTiming{
			URL:        h.url,
			StatusCode: h.status,
			Reused:     h.reused,
			DNSUs:      span(h.dnsStart, h.dnsDone).Microseconds(),
			ConnectUs:  span(h.connectStart, h.connectDone).Microseconds(),
			TLSUs:      span(h.tlsStart, h.tlsDone).Microseconds(),
			WaitUs:     span(h.wrote, h.firstByte).Microseconds(),
			DownloadUs: span(h.firstByte, end).Microseconds(),
			TotalUs:    end.Sub(h.start).Microseconds(),
		})
	}
	return out
}

// checkRedirect is the client's redirect policy when --timing is on: net/http's
// own default (stop after 10) plus telling the request's recorder about the
// hop. Every request carries its own recorder in its context.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if rec, ok := req.Context().Value(timingKey{}).(*timingRecorder); ok {
		rec.redirected(req)
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// timingOrNil is the JSON form of a recorder, or nothing without --timing.
func timingOrNil(rec *timingRecorder) *lib.WebTiming {
	if rec == nil {
		return nil
	}
	return &lib.WebTiming{Hops: rec.hopTimings()}
}

// timingLines is the text form: one "timing" log line per hop, none without
// --timing. Durations are printed like every other "time=" field.
func timingLines(rec *timingRecorder, attempt, iterations int) []string {
	if rec == nil {
		return nil
	}
	hops := rec.hopTimings()
	lines := make([]string, 0, len(hops))
	us := func(v int64) time.Duration { return time.Duration(v) * time.Microsecond }
	for i, h := range hops {
		connection := "new"
		if h.Reused {
			connection = "reused"
		}
		fields := []any{"url", h.URL, "hop", fmt.Sprintf("%d/%d", i+1, len(hops))}
		if h.StatusCode != 0 {
			fields = append(fields, "status", h.StatusCode)
		}
		fields = append(fields, "connection", connection, "dns", us(h.DNSUs), "connect", us(h.ConnectUs), "tls", us(h.TLSUs),
			"wait", us(h.WaitUs), "download", us(h.DownloadUs), "total", us(h.TotalUs), "attempt", fmt.Sprintf("%d/%d", attempt, iterations))
		lines = append(lines, lib.LogWithTimestamp(webModule, "timing "+lib.Fields(fields...), false))
	}
	return lines
}
