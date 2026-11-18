package web

import (
	"net/http"
	"net/url"
	"strings"
)

// A request's credentials are not written into shint's output (#99): JSON in
// particular ends up in CI logs, build artifacts and shared reports. The
// values of headers that carry a credential, and the password of a URL, show
// as [redacted]; the header names stay, and so does an Authorization's scheme
// ("Bearer [redacted]"), which says how the request authenticated without
// saying with what. --show-credentials writes them as they are, for debugging.
// Response headers are never touched: showing what the server sent is the
// point of the command.

const redacted = "[redacted]"

// credentialWords are the parts of a header's name that say it holds a
// credential, besides the standard Authorization, Proxy-Authorization and
// Cookie: X-API-Key, X-Auth-Token, X-Session-Id, X-Client-Secret...
var credentialWords = []string{"token", "secret", "password", "passwd", "apikey", "api-key", "api_key", "session", "auth"}

// credentialHeader is whether a header carries a credential.
func credentialHeader(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "authorization", "proxy-authorization", "cookie":
		return true
	}
	for _, w := range credentialWords {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

// redactValue is what a credential header's value shows: [redacted], after the
// scheme of an Authorization or Proxy-Authorization.
func redactValue(name, value string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization":
		if scheme, _, ok := strings.Cut(strings.TrimSpace(value), " "); ok && scheme != "" {
			return scheme + " " + redacted
		}
	}
	return redacted
}

// shownHeader is h as the output shows it.
func shownHeader(h http.Header, show bool) http.Header {
	if show || h == nil {
		return h
	}
	out := make(http.Header, len(h))
	for name, values := range h {
		if !credentialHeader(name) {
			out[name] = values
			continue
		}
		r := make([]string, len(values))
		for i, v := range values {
			r[i] = redactValue(name, v)
		}
		out[name] = r
	}
	return out
}

// shownHeaderLines is -H's "Name: value" forms as the output shows them.
func shownHeaderLines(lines []string, show bool) []string {
	if show {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		name, value, ok := strings.Cut(l, ":")
		if ok && credentialHeader(name) {
			out[i] = strings.TrimSpace(name) + ": " + redactValue(name, value)
		} else {
			out[i] = l
		}
	}
	return out
}

// shownURL is a URL as the output shows it: with its password, if it has one,
// as xxxxx (net/url's own redaction).
func shownURL(u *url.URL, show bool) string {
	if show {
		return u.String()
	}
	return u.Redacted()
}

// shownLocation is a redirect's target as the output shows it.
func shownLocation(location string, show bool) string {
	u, err := url.Parse(location)
	if show || err != nil {
		return location
	}
	return u.Redacted()
}
