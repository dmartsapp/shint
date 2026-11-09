package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

const tlsModule = "tls"

// TLSDefaultPort is HTTPS's port, what `shint tls host` checks without a port.
const TLSDefaultPort = 443

// TLSCheckOptions are what tls takes beyond the shared flags.
type TLSCheckOptions struct {
	Port       int
	ServerName string         // sent as SNI and verified against; "" means the host itself
	RootCAs    *x509.CertPool // what is trusted; nil means the system's pool
	CustomCA   bool           // RootCAs holds a --cacert file as well as the system's pool
	WarnDays   int            // a certificate that expires within this many days gets a WARN line (0: none)
	FailDays   int            // ... and within this many fails the check (0: only an expired or untrusted one fails)
}

// handshakeError is an error after the connection was made: in the TLS
// handshake itself, not in reaching the server.
type handshakeError struct {
	err     error
	timeout time.Duration
}

func (e handshakeError) Error() string { return e.err.Error() }
func (e handshakeError) Unwrap() error { return e.err }

// tlsResult is what one handshake found.
type tlsResult struct {
	state       tls.ConnectionState
	verified    [][]*x509.Certificate
	verifyErr   error
	handshakeIn time.Duration
}

// tlsHandshake connects to address:port and completes a TLS handshake, offering
// serverName as SNI (Go sends none for an address). It skips Go's own
// verification so the certificates can be reported whatever their state; they
// are verified afterwards, by verifyChain, against the name the user asked for.
func tlsHandshake(ctx context.Context, address string, port int, serverName string, timeoutSeconds int) (tlsResult, error) {
	timeout := time.Duration(timeoutSeconds) * time.Second
	dialer := net.Dialer{Timeout: timeout}
	begin := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		return tlsResult{}, err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return tlsResult{}, err
	}
	// #nosec G402 -- verification is done by verifyChain, against the name asked for:
	// skipping it here is what lets an untrusted or expired certificate be shown.
	tc := tls.Client(conn, &tls.Config{ServerName: serverName, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if err := tc.HandshakeContext(ctx); err != nil {
		return tlsResult{}, handshakeError{err: err, timeout: timeout}
	}
	return tlsResult{state: tc.ConnectionState(), handshakeIn: time.Since(begin)}, nil
}

// verifyChain is what a browser decides about the certificates a server sent:
// does the first lead, through the others, to a trusted root, and is it valid
// for name (a host name, or an address checked against its IP addresses) now?
func verifyChain(certs []*x509.Certificate, name string, roots *x509.CertPool, now time.Time) ([][]*x509.Certificate, error) {
	if len(certs) == 0 {
		return nil, errors.New("the server sent no certificate")
	}
	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	return certs[0].Verify(x509.VerifyOptions{DNSName: name, Roots: roots, Intermediates: intermediates, CurrentTime: now})
}

// explainTLSError puts a handshake or verification error in plain words, with
// what to do about it where there is something to do. name is the server name
// sent.
func explainTLSError(err error, name string, certs []*x509.Certificate, now time.Time) string {
	var unknown x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	var hostname x509.HostnameError
	var recordHeader tls.RecordHeaderError
	var inHandshake handshakeError
	var netErr net.Error
	switch {
	case errors.As(err, &inHandshake) && errors.As(err, &netErr) && netErr.Timeout():
		return fmt.Sprintf("the server accepted the connection and did not answer with TLS within --timeout %s - is it a plain-text port, or one that waits for the client to speak another protocol first?", inHandshake.timeout)
	case errors.As(err, &unknown):
		issuer := "its issuer"
		if len(certs) > 0 {
			issuer = strconv.Quote(certs[len(certs)-1].Issuer.String())
		}
		return fmt.Sprintf("signed by an unknown authority: %s is not a trusted root, and the server sent no certificate that leads to one (a private CA is trusted with --cacert)", issuer)
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		for _, c := range certs {
			if now.After(c.NotAfter) {
				return fmt.Sprintf("the certificate %q expired on %s, %s ago", c.Subject.String(), c.NotAfter.UTC().Format(time.DateOnly), formatDays(now.Sub(c.NotAfter)))
			}
			if now.Before(c.NotBefore) {
				return fmt.Sprintf("the certificate %q is not valid until %s", c.Subject.String(), c.NotBefore.UTC().Format(time.RFC3339))
			}
		}
	case errors.As(err, &hostname):
		return strings.TrimPrefix(hostname.Error(), "x509: ")
	case errors.As(err, &recordHeader):
		return "the server did not answer with TLS - is it a plain-text port? (" + err.Error() + ")"
	case strings.Contains(err.Error(), "remote error: tls: handshake failure") || strings.Contains(err.Error(), "remote error: tls: unrecognized name"):
		return fmt.Sprintf("the server refused the handshake - it may have no certificate for %q (the name sent; --servername sends another), or share no cipher suite with shint (%s)", name, err.Error())
	case strings.Contains(err.Error(), "unsupported protocol version") || strings.Contains(err.Error(), "protocol version not supported"):
		return "the server and shint share no TLS version: shint speaks TLS 1.2 and 1.3, so the server may speak only TLS 1.1 or older (" + err.Error() + ")"
	}
	return strings.TrimPrefix(strings.TrimPrefix(lib.ExplainError(err), "x509: "), "tls: ")
}

// daysUntil is how many days remain until t: fractional, negative once past.
func daysUntil(t, now time.Time) float64 {
	return math.Round(t.Sub(now).Hours()/24*100) / 100
}

// formatDays renders a duration the way a certificate's lifetime is read: in
// days, or in hours under two days. Negative durations keep their sign.
func formatDays(d time.Duration) string {
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	if d < 48*time.Hour {
		return fmt.Sprintf("%s%dh", sign, int(d.Hours()))
	}
	return fmt.Sprintf("%s%dd", sign, int(d.Hours()/24))
}

// publicKeyName describes a certificate's public key: "ECDSA P-256", "RSA 2048".
func publicKeyName(c *x509.Certificate) string {
	switch k := c.PublicKey.(type) {
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", k.N.BitLen())
	case *ecdsa.PublicKey:
		return "ECDSA " + k.Curve.Params().Name
	case ed25519.PublicKey:
		return "Ed25519"
	}
	return c.PublicKeyAlgorithm.String()
}

// describeCertificate is c as tls reports it.
func describeCertificate(c *x509.Certificate, now time.Time) lib.TLSCertificate {
	ips := make([]string, 0, len(c.IPAddresses))
	for _, ip := range c.IPAddresses {
		ips = append(ips, ip.String())
	}
	sum := sha256.Sum256(c.Raw)
	names := c.DNSNames
	if names == nil {
		names = []string{}
	}
	return lib.TLSCertificate{
		Subject:            c.Subject.String(),
		Issuer:             c.Issuer.String(),
		DNSNames:           names,
		IPAddresses:        ips,
		NotBefore:          c.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:           c.NotAfter.UTC().Format(time.RFC3339),
		ExpiresInDays:      daysUntil(c.NotAfter, now),
		SerialNumber:       hex.EncodeToString(c.SerialNumber.Bytes()),
		PublicKey:          publicKeyName(c),
		SignatureAlgorithm: c.SignatureAlgorithm.String(),
		IsCA:               c.IsCA,
		SHA256:             hex.EncodeToString(sum[:]),
	}
}

// chainNames is a verified chain as a line can show it: "CN=a > CN=b > CN=root".
func chainNames(chain []*x509.Certificate) []string {
	out := make([]string, 0, len(chain))
	for _, c := range chain {
		out = append(out, c.Subject.String())
	}
	return out
}

// TLSCheckHandler connects to every address host resolves to, iterations times,
// completes a TLS handshake and reports what the server presented: the TLS
// version and cipher suite, every certificate (subject, issuer, names, validity,
// key), whether the chain leads to a trusted root and is valid for the name
// asked for, and how long until the first of them expires.
//
// It reports true only if the lookup and every handshake succeeded, every chain
// was verified, and - when opts.FailDays is above zero - no certificate expires
// within that many days. A certificate that expires within opts.WarnDays gets a
// WARN line and does not fail the check. ctx is cancelled by Ctrl+C and nothing
// else; see interrupt.go for how a cancelled run ends.
func TLSCheckHandler(ctx context.Context, jsonoutput *bool, iterations int, delay int, throttle *bool, timeout int, host string, opts TLSCheckOptions) (ok bool) {
	serverName := opts.ServerName
	if serverName == "" {
		serverName = strings.TrimSuffix(host, ".")
	}
	trust := "system"
	if opts.CustomCA {
		trust = "system and --cacert"
	}
	output := lib.JSONOutput{}
	output.InputParams = lib.InputParams{
		Mode:     tlsModule,
		Host:     host,
		FromPort: opts.Port,
		ToPort:   opts.Port,
		Protocol: "tcp",
		Timeout:  timeout,
		Count:    iterations,
		Delay:    delay,
		Throttle: *throttle,
	}
	output.ModuleName = tlsModule
	stats := make([]lib.TLSStats, 0)
	istart := time.Now()
	dnsCtx, cancelDNS := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancelDNS()
	ipaddresses, err := lib.ResolveName(dnsCtx, host)
	if err != nil {
		if *jsonoutput {
			output.DNSLookup = lib.DNSLookup{Hostname: host, Success: false, Error: err.Error(), TimeTaken: time.Since(istart).Microseconds()}
			output.Error = err.Error()
			output.Stats = stats
			output.StartTime = istart.UnixMicro()
			output.EndTime = time.Now().UnixMicro()
			output.TotalTimeTaken = output.EndTime - output.StartTime
			JS, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(JS))
		} else {
			fmt.Println(lib.LogWithTimestamp(tlsModule, "dns resolution failed "+lib.Fields("host", host, "error", err.Error(), "time", time.Since(istart)), true))
		}
		return false
	}
	if !*jsonoutput {
		fmt.Println(lib.LogWithTimestamp(tlsModule, "dns resolved "+lib.Fields("host", host, "addresses", len(ipaddresses), "ips", "["+strings.Join(ipaddresses, ",")+"]", "time", time.Since(istart)), false))
		printAuthoritative(tlsModule, findAuthoritative(ctx, host, authoritativeTimeout(timeout)))
	} else {
		output.DNSLookup = lib.DNSLookup{Hostname: host, Success: true, ResolvedAddresses: ipaddresses, TimeTaken: time.Since(istart).Microseconds(), Authoritative: findAuthoritative(ctx, host, authoritativeTimeout(timeout))}
		output.StartTime = istart.UnixMicro()
	}

	var verifiedCount, failures, completed int
	shown := map[string]bool{} // certificates already listed, by fingerprint: once per run is enough
attempts:
	for i := 0; i < iterations; i++ {
		attempt := i + 1
		for _, ip := range ipaddresses {
			if ctx.Err() != nil || !pause(ctx, attemptDelay(delay, *throttle)) {
				break attempts
			}
			begin := time.Now()
			res, herr := tlsHandshake(ctx, ip, opts.Port, serverName, timeout)
			taken := time.Since(begin)
			if herr != nil && ctx.Err() != nil {
				break attempts // cut off by Ctrl+C: never finished, so it neither passed nor failed
			}
			now := time.Now()
			certs := res.state.PeerCertificates
			stat := lib.TLSStats{Address: ip, Port: opts.Port, ServerName: serverName, SentTime: begin.UnixMicro(), TimeTaken: taken.Microseconds(),
				Certificates: make([]lib.TLSCertificate, 0, len(certs)), Warnings: make([]string, 0)}
			where := lib.Fields("server", serverName, "address", ip, "port", opts.Port, "attempt", fmt.Sprintf("%d/%d", attempt, iterations))
			var failure string
			if herr != nil {
				failure = explainTLSError(herr, serverName, nil, now)
				stat.Error = failure
				if !*jsonoutput {
					fmt.Println(lib.LogWithTimestamp(tlsModule, "handshake failed "+where+" "+lib.Fields("time", taken, "error", failure), true))
				}
			} else {
				stat.RecvTime = now.UnixMicro()
				stat.TLSVersion = tls.VersionName(res.state.Version)
				stat.CipherSuite = tls.CipherSuiteName(res.state.CipherSuite)
				res.verified, res.verifyErr = verifyChain(certs, serverName, opts.RootCAs, now)
				for _, c := range certs {
					stat.Certificates = append(stat.Certificates, describeCertificate(c, now))
				}
				// Expiry is judged on the chain that verified - the one a client uses,
				// up to its trust anchor - not on every certificate sent: servers
				// often add cross-signed ones that clients never need, expired or not.
				// A chain that did not verify fails anyway; its certificates as sent
				// are what there is to judge.
				judged := certs
				if res.verifyErr == nil {
					stat.Verified = true
					stat.Trust = trust
					stat.VerifiedChain = chainNames(res.verified[0])
					judged = res.verified[0]
				} else {
					stat.VerifyError = explainTLSError(res.verifyErr, serverName, certs, now)
					failure = "verification failed: " + stat.VerifyError
				}
				soonest := math.Inf(1)
				for _, c := range judged {
					days := daysUntil(c.NotAfter, now)
					soonest = math.Min(soonest, days)
					if opts.WarnDays > 0 && days >= 0 && days < float64(opts.WarnDays) {
						stat.Warnings = append(stat.Warnings, fmt.Sprintf("%s expires in %s, on %s - within --warn-days %d", c.Subject.String(), formatDays(c.NotAfter.Sub(now)), c.NotAfter.UTC().Format(time.DateOnly), opts.WarnDays))
					}
				}
				if len(judged) > 0 {
					stat.ExpiresInDays = soonest
				}
				if failure == "" && opts.FailDays > 0 && len(judged) > 0 && soonest < float64(opts.FailDays) {
					failure = fmt.Sprintf("a certificate expires in %s, within --fail-days %d", formatDays(time.Duration(soonest*24*float64(time.Hour))), opts.FailDays)
				}
				if failure != "" {
					stat.Error = failure
				}

				if !*jsonoutput {
					fmt.Println(lib.LogWithTimestamp(tlsModule, "handshake "+where+" "+lib.Fields("version", stat.TLSVersion, "cipher", stat.CipherSuite, "certificates", len(certs), "time", taken), false))
					for depth, c := range certs {
						d := stat.Certificates[depth]
						if shown[d.SHA256] {
							continue
						}
						shown[d.SHA256] = true
						fields := []any{"depth", depth, "subject", d.Subject, "issuer", d.Issuer, "not_after", c.NotAfter.UTC().Format(time.DateOnly), "expires_in", formatDays(c.NotAfter.Sub(now))}
						if len(d.DNSNames)+len(d.IPAddresses) > 0 {
							fields = append(fields, "names", "["+strings.Join(append(append([]string{}, d.DNSNames...), d.IPAddresses...), ",")+"]")
						}
						fields = append(fields, "key", d.PublicKey, "signature", d.SignatureAlgorithm)
						fmt.Println(lib.LogWithTimestamp(tlsModule, "certificate "+lib.Fields(fields...), false))
					}
					if stat.Verified {
						fmt.Println(lib.LogWithTimestamp(tlsModule, "verified "+where+" "+lib.Fields("chain", strings.Join(stat.VerifiedChain, " > "), "trust", trust), false))
					} else {
						fmt.Println(lib.LogWithTimestamp(tlsModule, "verification failed "+where+" "+lib.Fields("error", stat.VerifyError), true))
					}
					for _, w := range stat.Warnings {
						fmt.Println(lib.LogWithLevel(tlsModule, lib.LevelWarn, "expires soon "+where+" "+lib.Fields("warning", w)))
					}
					if stat.Verified && failure != "" {
						fmt.Println(lib.LogWithTimestamp(tlsModule, "expires too soon "+where+" "+lib.Fields("error", failure), true))
					}
				}
			}
			stat.Success = failure == ""
			completed++
			if stat.Success {
				verifiedCount++
			} else {
				failures++
			}
			stats = append(stats, stat)
		}
	}

	planned := iterations * len(ipaddresses)
	interrupted := completed < planned // only Ctrl+C keeps an attempt from being made
	if *jsonoutput {
		output.Stats = stats
		output.EndTime = time.Now().UnixMicro()
		output.TotalTimeTaken = output.EndTime - output.StartTime
		if interrupted {
			output.Error = interruptedNote(completed, planned)
		}
		JS, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(JS))
	} else {
		checks := planned
		if interrupted {
			fmt.Println(interruptedLine(tlsModule, completed, planned, istart))
			checks = completed
		}
		fmt.Println(lib.LogWithTimestamp(tlsModule, "done "+lib.Fields("checks", checks, "passed", verifiedCount, "total_time", time.Since(istart)), false))
	}
	return failures == 0 && !interrupted
}
