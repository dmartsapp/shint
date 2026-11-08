package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib"
)

// statusServer answers every request with the given status - and, for HTTP/1.1,
// a reason phrase of its own, which shint must ignore in favour of the
// standard name.
func statusServer(t *testing.T, status int) *url.URL {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return u
}

// runWeb is a one-request run's text output.
func runWeb(t *testing.T, u *url.URL, timing bool) string {
	t.Helper()
	asJSON, throttle := false, false
	return captureStdout(t, func() {
		WebHandler(context.Background(), &asJSON, 1, 0, &throttle, 5, u, "GET", "", nil, false, nil, timing)
	})
}

// oneStat is the single attempt of a one-request --json run.
func oneStat(t *testing.T, u *url.URL, timing bool) lib.WebStats {
	t.Helper()
	stats := webJSON(t, 1, 0, u.String(), nil, timing)
	if len(stats) != 1 {
		t.Fatalf("%d stats, want 1", len(stats))
	}
	return stats[0]
}

// A response line is levelled "Response Received" whatever the status, and
// names the status by its standard name next to the number (issue #90).
func TestWebResponseLineNamesTheStatus(t *testing.T) {
	for _, tc := range []struct {
		status int
		text   string // as the line prints it: quoted when it has a space
	}{
		{200, "OK"},
		{403, "Forbidden"},
		{404, `"Not Found"`},
		{413, `"Content Too Large"`},  // Go's own name is "Request Entity Too Large"
		{520, `"Server Error (5xx)"`}, // unregistered (Cloudflare's): named by its class
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			u := statusServer(t, tc.status)
			out := runWeb(t, u, false)
			want := "[web] Response Received url=" + u.String() + " status=" + strconv.Itoa(tc.status) + " status_text=" + tc.text + " bytes_sent="
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
			if strings.Contains(out, "OK response") {
				t.Errorf("the old OK level is still printed:\n%s", out)
			}
		})
	}
}

// --json carries the same: result is the level the text line shows, and
// status_text the standard name - "ERROR" and "" when no response arrived.
func TestWebJSONCarriesTheResultAndTheStatusText(t *testing.T) {
	got := oneStat(t, statusServer(t, http.StatusForbidden), false)
	if got.Result != "Response Received" || got.StatusCode != 403 || got.StatusText != "Forbidden" || !got.Success {
		t.Errorf("result=%q status_code=%d status_text=%q success=%v, want Response Received/403/Forbidden/true",
			got.Result, got.StatusCode, got.StatusText, got.Success)
	}

	closed, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(freeTCPPort(t)) + "/")
	got = oneStat(t, closed, false)
	if got.Result != "ERROR" || got.StatusCode != 0 || got.StatusText != "" || got.Success {
		t.Errorf("no response: result=%q status_code=%d status_text=%q success=%v, want ERROR/0/\"\"/false",
			got.Result, got.StatusCode, got.StatusText, got.Success)
	}
	if out := runWeb(t, closed, false); !strings.Contains(out, "[web] ERROR request failed url=") || strings.Contains(out, "Response Received") {
		t.Errorf("a request with no response should stay an ERROR line:\n%s", out)
	}
}

// Every hop of a followed redirect names its own status, in text and JSON.
func TestWebTimingHopsNameTheirStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/final", http.StatusMovedPermanently)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL + "/")

	mustContain(t, runWeb(t, u, true),
		"hop=1/2 status=301 status_text=\"Moved Permanently\" connection=",
		"hop=2/2 status=200 status_text=OK connection=")

	hops := oneStat(t, u, true).Timing.Hops
	if len(hops) != 2 || hops[0].StatusText != "Moved Permanently" || hops[1].StatusText != "OK" {
		t.Errorf("hops = %+v, want status_text Moved Permanently then OK", hops)
	}
}

// listen http names the status it answered, too; its level stays OK - it is
// the server, and it answered.
func TestHTTPListenerNamesTheStatusItAnswered(t *testing.T) {
	get := func(port int) func(string) {
		return func(path string) {
			resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + path)
			if err != nil {
				t.Errorf("GET %s: %v", path, err)
				return
			}
			_ = resp.Body.Close()
		}
	}
	out := runHTTPListener(t, 2, 5, false, func(port int) { get(port)("/"); get(port)("/missing") })
	mustContain(t, out,
		"[listen-http] OK request method=GET path=/ status=200 status_text=OK remote=",
		"[listen-http] OK request method=GET path=/missing status=404 status_text=\"Not Found\" remote=")

	out = runHTTPListener(t, 1, 5, true, func(port int) { get(port)("/missing") })
	var event lib.HTTPListenEvent
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, `"status_code"`) {
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				t.Fatalf("json.Unmarshal: %v\n%s", err, line)
			}
		}
	}
	if event.StatusCode != 404 || event.StatusText != "Not Found" {
		t.Errorf("event status_code=%d status_text=%q, want 404 Not Found:\n%s", event.StatusCode, event.StatusText, out)
	}
}
