package web

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
	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func withHeadersOnly(t *testing.T) {
	t.Helper()
	HeadersOnly = true
	t.Cleanup(func() { HeadersOnly = false })
}

func headersServer(t *testing.T) *url.URL {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Zeta", "last")
		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Set("Content-Type", `text/plain; charset="utf-8"`)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("the body is never shown"))
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL + "/x")
	return u
}

// --headers-only: the status line and a line per header value, sorted, and
// nothing else - no lookup lines, no statistics, no body.
func TestHeadersOnlyText(t *testing.T) {
	withHeadersOnly(t)
	u := headersServer(t)
	throttle, asText := false, false
	var ok bool
	out := testutil.CaptureStdout(t, func() {
		ok = Run(context.Background(), &asText, 1, 0, &throttle, 5, u, "GET", "", nil, false, nil, false, false, 0)
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{
		"[web] Response Received url=" + u.String() + " status=202 status_text=Accepted proto=HTTP/1.1",
		"[web] OK header name=Content-Length value=23",
		`[web] OK header name=Content-Type value="text/plain; charset=\"utf-8\""`,
		"[web] OK header name=Date value=",
		"[web] OK header name=Set-Cookie value=a=1",
		"[web] OK header name=Set-Cookie value=b=2",
		"[web] OK header name=X-Zeta value=last",
	}
	if !ok || len(lines) != len(want) {
		t.Fatalf("ok %v, %d lines, want %d:\n%s", ok, len(lines), len(want), out)
	}
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Errorf("line %d: %q, want it to contain %q", i, lines[i], w)
		}
	}
	for _, absent := range []string{"dns resolved", "STATISTICS", "done", "never shown", "bytes_received"} {
		if strings.Contains(out, absent) {
			t.Errorf("%q should not be shown:\n%s", absent, out)
		}
	}
}

func TestHeadersOnlyJSON(t *testing.T) {
	withHeadersOnly(t)
	u := headersServer(t)
	throttle, asJSON := false, true
	out := testutil.CaptureStdout(t, func() {
		Run(context.Background(), &asJSON, 1, 0, &throttle, 5, u, "GET", "", nil, false, nil, false, false, 0)
	})
	var doc struct {
		InputParams lib.InputParams  `json:"input_params"`
		Stats       []map[string]any `json:"stats"`
		Error       string           `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !doc.InputParams.HeadersOnly || len(doc.Stats) != 1 {
		t.Fatalf("input_params %+v, stats %v", doc.InputParams, doc.Stats)
	}
	s := doc.Stats[0]
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	if strings.Join(sortedStrings(keys), ",") != "headers,status_code,status_text,success,url" {
		t.Errorf("a stat holds %v, want url, success, status and headers only", keys)
	}
	h := s["headers"].(map[string]any)
	if cookies := h["Set-Cookie"].([]any); len(cookies) != 2 || h["X-Zeta"].([]any)[0] != "last" || s["status_code"].(float64) != 202 {
		t.Errorf("headers %v", h)
	}

	// A request that gets no response: its error, no headers.
	closed, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(testutil.FreeTCPPort(t)) + "/")
	out = testutil.CaptureStdout(t, func() {
		if Run(context.Background(), &asJSON, 1, 0, &throttle, 2, closed, "GET", "", nil, false, nil, false, false, 0) {
			t.Error("no server: want a failure")
		}
	})
	doc.Stats = nil
	if err := json.Unmarshal([]byte(out), &doc); err != nil || len(doc.Stats) != 1 || doc.Stats[0]["error"] == nil || doc.Stats[0]["headers"] != nil || doc.Stats[0]["success"] != false {
		t.Errorf("failure: %v %v", err, doc.Stats)
	}
}

func sortedStrings(s []string) []string {
	out := append([]string(nil), s...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
