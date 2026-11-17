package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSameOrigin(t *testing.T) {
	for _, c := range []struct {
		from, to string
		same     bool
	}{
		{"http://a.example/x", "http://a.example/y", true},
		{"http://a.example/x", "http://A.EXAMPLE:80/y", true},
		{"https://a.example/x", "https://a.example:443/y", true},
		{"http://a.example/x", "https://a.example/y", true}, // the same host, moving to TLS on the default ports
		{"http://a.example.:80/x", "http://a.example/y", true},
		{"http://[::1]:8080/x", "http://[::1]:8080/y", true},
		{"https://a.example/x", "http://a.example/y", false}, // leaving TLS
		{"http://a.example/x", "http://b.example/y", false},
		{"http://a.example/x", "http://a.example:8080/y", false},
		{"http://a.example:8080/x", "https://a.example:8443/y", false},
		{"http://a.example/x", "http://sub.a.example/y", false},
		{"http://127.0.0.1:8080/x", "http://localhost:8080/y", false},
	} {
		from, _ := url.Parse(c.from)
		to, _ := url.Parse(c.to)
		if got := sameOrigin(from, to); got != c.same {
			t.Errorf("sameOrigin(%s, %s) = %v, want %v", c.from, c.to, got, c.same)
		}
	}
}

func TestCredentialsOf(t *testing.T) {
	cert := &tls.Config{Certificates: []tls.Certificate{{}}}
	for _, c := range []struct {
		cfg     *tls.Config
		headers []string
		want    string
	}{
		{nil, nil, ""},
		{&tls.Config{}, nil, ""},
		{cert, nil, "a client certificate"},
		{nil, []string{"X-API-Key: k"}, "headers given with -H"},
		{cert, []string{"X-API-Key: k"}, "a client certificate and headers given with -H"},
	} {
		if got := credentialsOf(c.cfg, c.headers); got != c.want {
			t.Errorf("credentialsOf(%v, %v) = %q, want %q", c.cfg != nil && len(c.cfg.Certificates) > 0, c.headers, got, c.want)
		}
	}
}

// redirectingServers starts "elsewhere", which counts the requests it gets
// and remembers the X-API-Key header, and "origin", which redirects /away to
// elsewhere (another port: another origin) and /near to its own /landed.
func redirectingServers(t *testing.T, tlsServers *testCA) (origin, elsewhere *httptest.Server, hits *int32, key *atomic.Value) {
	t.Helper()
	hits, key = new(int32), new(atomic.Value)
	key.Store("")
	elsewhereHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		key.Store(r.Header.Get("X-API-Key"))
		_, _ = w.Write([]byte("elsewhere"))
	})
	if tlsServers != nil {
		elsewhere = httptest.NewUnstartedServer(elsewhereHandler)
		// Asks every client for a certificate, the way a host after one would.
		elsewhere.TLS = &tls.Config{Certificates: []tls.Certificate{tlsServers.ServerTLSCert}, ClientAuth: tls.RequestClientCert}
		elsewhere.StartTLS()
	} else {
		elsewhere = httptest.NewServer(elsewhereHandler)
	}
	t.Cleanup(elsewhere.Close)
	originHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, elsewhere.URL+"/stolen", http.StatusFound)
		case "/near":
			http.Redirect(w, r, "/landed", http.StatusFound)
		default:
			_, _ = w.Write([]byte("landed with key=" + r.Header.Get("X-API-Key")))
		}
	})
	if tlsServers != nil {
		origin = httptest.NewUnstartedServer(originHandler)
		origin.TLS = &tls.Config{Certificates: []tls.Certificate{tlsServers.ServerTLSCert}, ClientCAs: tlsServers.CAPool, ClientAuth: tls.RequireAndVerifyClientCert}
		origin.StartTLS()
	} else {
		origin = httptest.NewServer(originHandler)
	}
	t.Cleanup(origin.Close)
	return origin, elsewhere, hits, key
}

// redirectRun runs web once with --json and returns its one stat.
func redirectRun(t *testing.T, target string, headers []string, tlsConfig *tls.Config, timing bool) map[string]any {
	t.Helper()
	u, _ := url.Parse(target)
	jsonOutput, throttle := true, false
	out := captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, u, "GET", "", headers, false, tlsConfig, timing)
	})
	var o struct {
		Stats []map[string]any `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil || len(o.Stats) != 1 {
		t.Fatalf("want one stat (%v):\n%s", err, out)
	}
	return o.Stats[0]
}

func TestARedirectToAnotherOriginKeepsTheHeaders(t *testing.T) {
	origin, elsewhere, hits, key := redirectingServers(t, nil)
	stat := redirectRun(t, origin.URL+"/away", []string{"X-API-Key: s3cret"}, nil, false)
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Fatalf("the other origin got %d request(s) (X-API-Key %q): the redirect was followed", n, key.Load())
	}
	if stat["status_code"] != float64(302) || stat["success"] != true {
		t.Errorf("want the 302 as a successful response, got status %v success %v", stat["status_code"], stat["success"])
	}
	nf, _ := stat["redirect_not_followed"].(map[string]any)
	if nf == nil || nf["location"] != elsewhere.URL+"/stolen" || !strings.Contains(nf["reason"].(string), "headers given with -H") {
		t.Errorf("redirect_not_followed = %v", stat["redirect_not_followed"])
	}

	// Text mode: a WARN line, then the 302 as the response.
	u, _ := url.Parse(origin.URL + "/away")
	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, u, "GET", "", []string{"X-API-Key: s3cret"}, false, nil, false)
	})
	if !strings.Contains(out, "] WARN redirect not followed ") || !strings.Contains(out, "location="+elsewhere.URL+"/stolen") || !strings.Contains(out, "status=302") {
		t.Errorf("text output:\n%s", out)
	}
}

func TestWithoutCredentialsARedirectIsFollowed(t *testing.T) {
	origin, _, hits, _ := redirectingServers(t, nil)
	stat := redirectRun(t, origin.URL+"/away", nil, nil, false)
	if n := atomic.LoadInt32(hits); n != 1 || stat["status_code"] != float64(200) {
		t.Fatalf("want the redirect followed (1 request, 200), got %d request(s), status %v", n, stat["status_code"])
	}
	if _, ok := stat["redirect_not_followed"]; ok {
		t.Errorf("redirect_not_followed is set on a followed redirect: %v", stat["redirect_not_followed"])
	}
}

func TestASameOriginRedirectIsFollowedWithTheHeaders(t *testing.T) {
	origin, _, hits, _ := redirectingServers(t, nil)
	u, _ := url.Parse(origin.URL + "/near")
	jsonOutput, throttle := false, false
	out := captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, u, "GET", "", []string{"X-API-Key: s3cret"}, false, nil, false)
	})
	if strings.Contains(out, "redirect not followed") || !strings.Contains(out, "status=200") || atomic.LoadInt32(hits) != 0 {
		t.Errorf("a same-origin redirect with -H:\n%s", out)
	}
}

func TestARedirectToAnotherOriginKeepsTheClientCertificate(t *testing.T) {
	ca := generateTestCA(t)
	tlsConfig, err := BuildTLSConfig(ca.CAFile, ca.ClientCertFile, ca.ClientKeyFile, false)
	if err != nil {
		t.Fatal(err)
	}
	origin, _, hits, _ := redirectingServers(t, ca)
	stat := redirectRun(t, origin.URL+"/away", nil, tlsConfig, false)
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Fatalf("the other origin, which asks for a client certificate, got %d request(s)", n)
	}
	nf, _ := stat["redirect_not_followed"].(map[string]any)
	if stat["status_code"] != float64(302) || nf == nil || !strings.Contains(nf["reason"].(string), "a client certificate") {
		t.Errorf("status %v, redirect_not_followed %v", stat["status_code"], stat["redirect_not_followed"])
	}
	// The same origin still gets it: the mutual-TLS origin answers /near's target.
	stat = redirectRun(t, origin.URL+"/near", nil, tlsConfig, false)
	if stat["status_code"] != float64(200) {
		t.Errorf("a same-origin redirect with --cert: status %v", stat["status_code"])
	}
}

func TestARedirectNotFollowedIsOneHopWithTiming(t *testing.T) {
	origin, _, hits, _ := redirectingServers(t, nil)
	stat := redirectRun(t, origin.URL+"/away", []string{"X-API-Key: s3cret"}, nil, true)
	timing, _ := stat["timing"].(map[string]any)
	hops, _ := timing["hops"].([]any)
	if atomic.LoadInt32(hits) != 0 || len(hops) != 1 {
		t.Errorf("want one hop and no request elsewhere, got %d hop(s), %d request(s): %v", len(hops), atomic.LoadInt32(hits), stat["timing"])
	}
}
