package handlers

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/dmartsapp/shint/v4/lib"
)

// nsRecord is one NS record with the name that owns it: the owner is what says
// which zone the record is the delegation of. (net.Resolver.LookupNS returns only
// the host names, and a resolver answers a query for an alias with the alias and
// the NS records of the name it points to - attributing those to the alias
// names the wrong zone.)
type nsRecord struct{ Owner, Host string }

// queryNS asks for the NS records at name and returns those owned by exactly that
// name. A name that is not a zone apex - it does not exist, or exists without name
// servers of its own - is (nil, nil), not an error: that is the ordinary answer
// for most names in a walk up the labels. It is a variable so tests can supply
// answers without a DNS server.
var queryNS = func(ctx context.Context, servers []string, name string, timeout time.Duration) ([]nsRecord, error) {
	answer := queryServers(ctx, servers, name+".", dnsmessage.TypeNS, DNSOptions{}, timeout)
	if answer.Err != nil {
		return nil, answer.Err
	}
	switch answer.Msg.RCode {
	case dnsmessage.RCodeSuccess:
	case dnsmessage.RCodeNameError:
		return nil, nil
	default:
		return nil, fmt.Errorf("the server answered with %s", rcodeName(answer.Msg.RCode))
	}
	var out []nsRecord
	for _, r := range answer.Msg.Answers {
		ns, ok := r.Body.(*dnsmessage.NSResource)
		if !ok || !strings.EqualFold(strings.TrimSuffix(r.Header.Name.String(), "."), name) {
			continue
		}
		out = append(out, nsRecord{Owner: name, Host: ns.NS.String()})
	}
	return out, nil
}

// maxZoneCandidates bounds the walk up a name's labels.
const maxZoneCandidates = 8

// zoneCandidates lists host and every parent of it down to the top-level domain,
// closest first: www.example.co.uk gives www.example.co.uk, example.co.uk, co.uk
// and uk. It gives none for a name that has no zone of its own to look for: an IP
// address, a single label (localhost, a bare machine name) or *.localhost.
func zoneCandidates(host string) []string {
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if name == "" || net.ParseIP(name) != nil || !strings.Contains(name, ".") || strings.HasSuffix(name, ".localhost") {
		return nil
	}
	labels := strings.Split(name, ".")
	var out []string
	for i := 0; i < len(labels) && len(out) < maxZoneCandidates; i++ {
		out = append(out, strings.Join(labels[i:], "."))
	}
	return out
}

// findAuthoritative works out which zone host belongs to and its name servers.
// NS records exist only at a zone's apex (example.com has them, www.example.com
// does not), so the name and each of its parents are asked at once and the closest
// one that has name servers of its own is the zone. Only a candidate whose answer
// is owned by that very name counts, which keeps an alias from borrowing the
// zone of the name it points to.
//
// The answer is decided as soon as every candidate closer than the first one
// found has answered: the ones above it cannot change it, and are dropped, so a
// slow lookup of the top-level domain does not hold up the result.
//
// It returns nil when host has no zone to look for (see zoneCandidates). timeout
// bounds the whole lookup and ctx (Ctrl+C) cuts it short. What it reports is the
// delegation as the system's resolver serves it - not proof of which server
// answered a query and not an authoritative (AA) answer: that is what "shint dns
// @server" is for.
func findAuthoritative(ctx context.Context, host string, timeout time.Duration) *lib.Authoritative {
	candidates := zoneCandidates(host)
	if len(candidates) == 0 {
		return nil
	}
	start := time.Now()
	found := &lib.Authoritative{Nameservers: []string{}}
	defer func() { found.TimeTaken = time.Since(start).Microseconds() }()

	lctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	servers, _, err := serverList(lctx, DNSQuery{Port: dnsDefaultPort}, int(timeout/time.Second)+1)
	if err != nil {
		found.Error = "the name servers could not be looked up: " + err.Error()
		return found
	}

	type result struct {
		index   int
		records []nsRecord
		err     error
	}
	results := make(chan result, len(candidates))
	for i, c := range candidates {
		go func(i int, name string) {
			records, err := queryNS(lctx, servers, name, timeout)
			results <- result{i, records, err}
		}(i, c)
	}

	answered := make([]*result, len(candidates))
	var firstErr error
	for range candidates {
		r := <-results
		answered[r.index] = &r
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		for i := range answered {
			if answered[i] == nil {
				break // a closer candidate has not answered yet: too early to say
			}
			if len(answered[i].records) > 0 {
				hosts := make([]string, 0, len(answered[i].records))
				seen := map[string]bool{}
				for _, rec := range answered[i].records {
					h := strings.ToLower(strings.TrimSuffix(rec.Host, "."))
					if h != "" && !seen[h] {
						seen[h] = true
						hosts = append(hosts, h)
					}
				}
				sort.Strings(hosts)
				found.Zone, found.Nameservers = candidates[i], hosts
				return found
			}
		}
	}
	found.Error = "no name servers found for " + candidates[0] + " or any of its parent zones"
	if firstErr != nil {
		found.Error = "the name servers could not be looked up: " + shortErr(firstErr)
	}
	return found
}

// authoritativeTimeout bounds the extra name-server lookup a command makes after
// it resolves a host name: --timeout, but never more than three seconds - it is
// context, and must not hold up the check it decorates.
func authoritativeTimeout(timeoutSeconds int) time.Duration {
	d := time.Duration(timeoutSeconds) * time.Second
	if d > 3*time.Second {
		d = 3 * time.Second
	}
	return d
}

// printAuthoritative is the text-mode line that follows "dns resolved": the zone
// the host belongs to and its authoritative name servers. Nothing is printed when
// there is nothing to say - the host has no zone of its own (an IP address, a
// single-label name) or none was found: this is information a command offers,
// not a check that can fail.
func printAuthoritative(module string, a *lib.Authoritative) {
	if a == nil || a.Zone == "" {
		return
	}
	fmt.Println(lib.LogWithTimestamp(module, "dns authoritative "+lib.Fields("zone", a.Zone, "nameservers", "["+strings.Join(a.Nameservers, ",")+"]", "time", time.Duration(a.TimeTaken)*time.Microsecond), false))
}
