package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCredentialHeader(t *testing.T) {
	for _, name := range []string{"Authorization", "proxy-authorization", "Cookie", "X-API-Key", "X-Api_Key", "X-Auth-Token", "X-Session-Id", "X-Client-Secret", "X-Password", "Private-Token"} {
		if !credentialHeader(name) {
			t.Errorf("credentialHeader(%q) = false", name)
		}
	}
	for _, name := range []string{"Accept", "Content-Type", "User-Agent", "Host", "X-Request-Id", "Cache-Control"} {
		if credentialHeader(name) {
			t.Errorf("credentialHeader(%q) = true", name)
		}
	}
}

func TestRedactValue(t *testing.T) {
	for _, c := range [][3]string{
		{"Authorization", "Bearer s3cret", "Bearer [redacted]"},
		{"Authorization", "Basic dXNlcjpwYXNz", "Basic [redacted]"},
		{"Authorization", "s3cret", "[redacted]"},
		{"Proxy-Authorization", " Basic abc ", "Basic [redacted]"},
		{"Cookie", "session=abc; theme=dark", "[redacted]"},
		{"X-API-Key", "k3y with spaces", "[redacted]"},
	} {
		if got := redactValue(c[0], c[1]); got != c[2] {
			t.Errorf("redactValue(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestShownHeadersLeaveTheRequestAlone(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer s3cret"}, "Accept": {"text/plain"}}
	shown := shownHeader(h, false)
	if shown.Get("Authorization") != "Bearer [redacted]" || shown.Get("Accept") != "text/plain" {
		t.Errorf("shown = %v", shown)
	}
	if h.Get("Authorization") != "Bearer s3cret" {
		t.Fatalf("the request's own header changed: %v", h)
	}
	if got := shownHeader(h, true); got.Get("Authorization") != "Bearer s3cret" {
		t.Errorf("--show-credentials: %v", got)
	}
	lines := shownHeaderLines([]string{"X-API-Key: k3y", "Accept: text/plain", "malformed"}, false)
	if strings.Join(lines, "|") != "X-API-Key: [redacted]|Accept: text/plain|malformed" {
		t.Errorf("lines = %q", lines)
	}
}

func TestCredentialsAreNotInTheOutput(t *testing.T) {
	var gotAuth, gotKey atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		gotKey.Store(r.Header.Get("X-API-Key"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL + "/ok")
	u.User = url.UserPassword("alice", "hunter2")
	headers := []string{"Authorization: Bearer s3cret", "X-API-Key: k3y", "Accept: text/plain"}
	run := func(jsonOut, show bool) string {
		throttle := false
		return captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOut, 1, 0, &throttle, 5, u, "GET", "", headers, false, nil, false, show)
		})
	}
	for _, jsonOut := range []bool{true, false} {
		out := run(jsonOut, false)
		for _, secret := range []string{"s3cret", "k3y", "hunter2"} {
			if strings.Contains(out, secret) {
				t.Errorf("json=%v: %q is in the output:\n%s", jsonOut, secret, out)
			}
		}
		if !strings.Contains(out, "alice:xxxxx@") {
			t.Errorf("json=%v: the URL's password is not shown as xxxxx:\n%s", jsonOut, out)
		}
		if jsonOut && (!strings.Contains(out, "Bearer [redacted]") || !strings.Contains(out, "text/plain")) {
			t.Errorf("the JSON lost what it should keep:\n%s", out)
		}
	}
	if gotAuth.Load() != "Bearer s3cret" || gotKey.Load() != "k3y" {
		t.Fatalf("the server did not get the real headers: %v %v", gotAuth.Load(), gotKey.Load())
	}
	out := run(true, true)
	for _, secret := range []string{"Bearer s3cret", "k3y", "alice:hunter2@"} {
		if !strings.Contains(out, secret) {
			t.Errorf("--show-credentials: %q is not in the output:\n%s", secret, out)
		}
	}
}
