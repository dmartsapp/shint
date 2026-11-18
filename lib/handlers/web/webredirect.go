package web

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// A request's credentials go no further than the origin they were given for
// (#98). A client certificate (--cert) is offered to any server that asks for
// one, and the headers given with -H travel with every request: net/http drops
// Authorization and Cookie when a redirect changes host, but forwards an
// X-API-Key as it is. So a request that carries either does not follow a
// redirect to another origin - the redirect is the response, and the run says
// why. A request without credentials follows redirects as before, up to 10.

// redirectStop is where a redirect would have led, when it was not followed.
// The client's CheckRedirect fills it in; the attempt reports it.
type redirectStop struct {
	location string
}

type redirectStopKey struct{}

// credentialsOf names what a request carries that must stay with its origin,
// or "" when it carries nothing of the kind.
func credentialsOf(tlsConfig *tls.Config, headers []string) string {
	cert := tlsConfig != nil && len(tlsConfig.Certificates) > 0
	switch {
	case cert && len(headers) > 0:
		return "a client certificate and headers given with -H"
	case cert:
		return "a client certificate"
	case len(headers) > 0:
		return "headers given with -H"
	}
	return ""
}

// notFollowedReason says why a redirect was not followed.
func notFollowedReason(credentials string) string {
	return "another origin, and the request carries " + credentials
}

// redirectPolicy is the client's CheckRedirect: net/http's own default (stop
// after 10), a request's credentials kept at its origin, and --timing told of
// every hop that is followed.
func redirectPolicy(credentials string) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if credentials != "" && !sameOrigin(via[0].URL, req.URL) {
			if stop, ok := req.Context().Value(redirectStopKey{}).(*redirectStop); ok {
				stop.location = req.URL.String()
			}
			return http.ErrUseLastResponse // the redirect is the response
		}
		if rec, ok := req.Context().Value(timingKey{}).(*timingRecorder); ok {
			rec.redirected(req)
		}
		return nil
	}
}

// sameOrigin is whether to may be sent what was given for from: the same
// scheme, host and port - or the same host moving from plain HTTP to HTTPS on
// the default ports, where the credentials stay with that host, encrypted.
func sameOrigin(from, to *url.URL) bool {
	if !strings.EqualFold(strings.TrimSuffix(from.Hostname(), "."), strings.TrimSuffix(to.Hostname(), ".")) {
		return false
	}
	fs, ts := strings.ToLower(from.Scheme), strings.ToLower(to.Scheme)
	fp, tp := effectivePort(fs, from.Port()), effectivePort(ts, to.Port())
	if fs == ts {
		return fp == tp
	}
	return fs == "http" && ts == "https" && fp == "80" && tp == "443"
}

// effectivePort is a URL's port, or its scheme's default.
func effectivePort(scheme, port string) string {
	if port != "" {
		return port
	}
	switch scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}
