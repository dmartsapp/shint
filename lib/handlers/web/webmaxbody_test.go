package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestReadBody(t *testing.T) {
	for _, c := range []struct {
		body      string
		max       int64
		kept      string
		truncated bool
	}{
		{"hello", 0, "hello", false},
		{"hello", 10, "hello", false},
		{"hello", 5, "hello", false}, // exactly the limit: nothing left out
		{"hello", 3, "hel", true},
		{"", 3, "", false},
	} {
		got, truncated, err := readBody(strings.NewReader(c.body), c.max)
		if err != nil || string(got) != c.kept || truncated != c.truncated {
			t.Errorf("readBody(%q, %d) = %q, %v, %v; want %q, %v", c.body, c.max, got, truncated, err, c.kept, c.truncated)
		}
	}
}

func TestABodyLongerThanMaxBodyIsCut(t *testing.T) {
	big := `{"data":"` + strings.Repeat("x", 3<<20) + `"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(big))
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	run := func(maxBody int64) map[string]any {
		jsonOut, throttle := true, false
		out := testutil.CaptureStdout(t, func() {
			Run(context.Background(), &jsonOut, 1, 0, &throttle, 5, u, "GET", "", nil, true, nil, false, false, maxBody)
		})
		var o struct {
			Stats []map[string]any `json:"stats"`
		}
		if err := json.Unmarshal([]byte(out), &o); err != nil || len(o.Stats) != 1 {
			t.Fatalf("want one stat (%v)", err)
		}
		return o.Stats[0]
	}
	stat := run(1 << 20)
	body, _ := stat["response"].(map[string]any)["body"].(string)
	if stat["body_truncated"] != true || len(body) != 1<<20 {
		t.Errorf("want the first 1 MiB as text and body_truncated, got %d bytes, body_truncated=%v", len(body), stat["body_truncated"])
	}
	if received, _ := stat["bytes_received"].(float64); received < float64(len(big)) {
		t.Errorf("bytes_received %v does not cover the whole %d-byte body", received, len(big))
	}
	if errs, _ := stat["errors"].([]any); len(errs) != 0 {
		t.Errorf("a cut body is not an error: %v", errs)
	}
	stat = run(0) // no limit: the whole document, parsed
	if _, ok := stat["body_truncated"]; ok {
		t.Errorf("body_truncated set with no limit")
	}
	if doc, ok := stat["response"].(map[string]any)["body"].(map[string]any); !ok || len(doc["data"].(string)) != 3<<20 {
		t.Errorf("with no limit, the body is not the whole parsed document")
	}
}
