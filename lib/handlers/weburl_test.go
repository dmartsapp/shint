package handlers

import (
	"strings"
	"testing"
)

func TestParseWebURL(t *testing.T) {
	good := []struct{ in, want string }{
		{"example.com", "https://example.com"},
		{"example.com/path?x=1", "https://example.com/path?x=1"},
		{"127.0.0.1:8080/ok", "https://127.0.0.1:8080/ok"},
		{"localhost:8080", "https://localhost:8080"}, // once parsed as the scheme "localhost"
		{"[::1]:8080/x", "https://[::1]:8080/x"},
		{"//127.0.0.1/x", "https://127.0.0.1/x"},
		{"  example.com  ", "https://example.com"},
		{"http://example.com", "http://example.com"},
		{"HTTP://EXAMPLE.com/A", "http://EXAMPLE.com/A"},
		{"https://user:pw@example.com:8443/p?q=1#f", "https://user:pw@example.com:8443/p?q=1#f"},
		{"http://[2606:4700:4700::1111]/", "http://[2606:4700:4700::1111]/"},
	}
	for _, c := range good {
		u, err := ParseWebURL(c.in)
		if err != nil {
			t.Errorf("ParseWebURL(%q): %v", c.in, err)
			continue
		}
		if u.String() != c.want {
			t.Errorf("ParseWebURL(%q) = %s, want %s", c.in, u, c.want)
		}
	}
	bad := []struct{ in, want string }{
		{"", "an empty URL"},
		{"   ", "an empty URL"},
		{"http:example.com", "write the scheme as http:// or https://"},
		{"https:/example.com", "write the scheme as http:// or https://"},
		{"HTTP:", "write the scheme as http:// or https://"},
		{"ftp://example.com/", `unsupported scheme "ftp"`},
		{"file:///etc/passwd", `unsupported scheme "file"`},
		{"gopher://h/", "web fetches http:// and https:// URLs"},
		{"http://", "it has no host"},
		{"https://:8080/", "it has no host"},
		{"not a url", "invalid URL"},
		{"http://exa mple.com", "invalid URL"},
		{"http://[::1", "invalid URL"},
		{"https://%zz.example.com", "invalid URL"},
	}
	for _, c := range bad {
		u, err := ParseWebURL(c.in)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("ParseWebURL(%q) = %v, %v; want an error containing %q", c.in, u, err, c.want)
		}
	}
}

// web measures the direct path from this machine: it never goes through a
// proxy, whatever HTTP_PROXY and friends say. The documentation says so (issue
// #36), and this keeps it true: a transport with a Proxy function would honour
// the environment.
func TestWebTransportNeverUsesAProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	if tr := newCountingTransport(nil); tr.Proxy != nil {
		t.Error("the web transport has a Proxy function: HTTP_PROXY would be honoured")
	}
}
