package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestWebHandlerInterruptedReportsHowFarItGot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	jsonOutput, throttle := false, false
	var ok bool
	ctx := testutil.CancelAfter(t, 400*time.Millisecond)
	out := testutil.RunWithin(t, 3*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 200, 40, &throttle, 5, u, "GET", "", nil, false, nil, false, false, 0)
	})
	if ok {
		t.Error("an interrupted run must not report success")
	}
	completed, planned := testutil.InterruptedCounts(t, out)
	if planned != 200 || completed < 3 || completed >= 200 {
		t.Errorf("completed %d of %d, want a few of 200", completed, planned)
	}
	if !strings.Contains(out, "Requests sent: "+strconv.Itoa(completed)+", Response received: "+strconv.Itoa(completed)) || !strings.Contains(out, "[web] OK done") {
		t.Errorf("the statistics and the done line must follow:\n%s", out)
	}
	if strings.Contains(out, "request failed") {
		t.Errorf("an attempt cut off by Ctrl+C is not a failure:\n%s", out)
	}
}

// A request still waiting for the server when Ctrl+C arrives is abandoned - not
// counted, not reported as a failure - and does not hold the run up.
func TestWebHandlerInterruptAbandonsARequestInFlight(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	u, _ := url.Parse(srv.URL)
	jsonOutput, throttle := false, false
	ctx := testutil.CancelAfter(t, 300*time.Millisecond)
	out := testutil.RunWithin(t, 2*time.Second, func() {
		Run(ctx, &jsonOutput, 3, 0, &throttle, 30, u, "GET", "", nil, false, nil, false, false, 0)
	})
	completed, planned := testutil.InterruptedCounts(t, out)
	if completed != 0 || planned != 3 {
		t.Errorf("completed %d of %d, want 0 of 3", completed, planned)
	}
	if strings.Contains(out, "request failed") || strings.Contains(out, "response url=") {
		t.Errorf("the abandoned request must leave no result:\n%s", out)
	}
}

func TestWebHandlerInterruptedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	jsonOutput, throttle := true, false
	ctx := testutil.CancelAfter(t, 350*time.Millisecond)
	var ok bool
	out := testutil.RunWithin(t, 3*time.Second, func() {
		ok = Run(ctx, &jsonOutput, 200, 40, &throttle, 5, u, "GET", "", nil, false, nil, false, false, 0)
	})
	var doc struct {
		Error string        `json:"error"`
		Stats []interface{} `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`^interrupted: (\d+) of 200 attempts completed$`).FindStringSubmatch(doc.Error)
	if ok || m == nil {
		t.Fatalf("ok=%v, error=%q", ok, doc.Error)
	}
	if n, _ := strconv.Atoi(m[1]); n != len(doc.Stats) || n == 0 {
		t.Errorf("error says %s attempts completed, stats has %d", m[1], len(doc.Stats))
	}
}
