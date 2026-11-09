package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// tlsServer serves TLS on a loopback port with cert, completing every handshake,
// until the test ends; it returns the port.
func tlsServer(t *testing.T, cert tls.Certificate) int {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = c.(*tls.Conn).Handshake()
				_ = c.Close()
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// plainServer answers every connection with a line of plain text, as an HTTP
// server on a non-TLS port would.
func plainServer(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = c.Write([]byte("HTTP/1.0 400 Bad Request\r\n\r\n"))
			_ = c.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// selfSigned is a self-signed certificate for 127.0.0.1, valid from notBefore to
// notAfter, and a pool that trusts it.
func selfSigned(t *testing.T, notBefore, notAfter time.Time) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "shint-test-self"},
		NotBefore: notBefore, NotAfter: notAfter, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: parsed}, pool
}

// runTLS runs the handler once against 127.0.0.1:port, as text and as JSON.
func runTLS(t *testing.T, opts TLSCheckOptions, count int) (text string, ok bool, doc lib.JSONOutput, stats []lib.TLSStats) {
	t.Helper()
	off := false
	text = captureStdout(t, func() {
		f := false
		ok = TLSCheckHandler(context.Background(), &f, count, 0, &off, 3, "127.0.0.1", opts)
	})
	out := captureStdout(t, func() {
		j := true
		TLSCheckHandler(context.Background(), &j, count, 0, &off, 3, "127.0.0.1", opts)
	})
	var raw struct {
		lib.JSONOutput
		Stats []lib.TLSStats `json:"stats"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("JSON: %v\n%s", err, out)
	}
	return text, ok, raw.JSONOutput, raw.Stats
}

func TestTLSATrustedChainExpiringSoonWarnsAndPasses(t *testing.T) {
	ca := generateTestCA(t)
	port := tlsServer(t, ca.ServerTLSCert)
	text, ok, doc, stats := runTLS(t, TLSCheckOptions{Port: port, RootCAs: ca.CAPool, CustomCA: true, WarnDays: 30}, 2)
	if !ok {
		t.Fatalf("a trusted chain failed:\n%s", text)
	}
	for _, want := range []string{"] OK handshake server=127.0.0.1 address=127.0.0.1 port=", `version="TLS 1.3"`,
		"] OK certificate depth=0 subject=CN=shint-test-server issuer=CN=shint-test-ca", "names=[localhost,127.0.0.1]", `key="ECDSA P-256"`,
		`] OK verified server=127.0.0.1`, `chain="CN=shint-test-server > CN=shint-test-ca" trust="system and --cacert"`,
		"] WARN expires soon server=127.0.0.1", "within --warn-days 30", "] OK done checks=2 passed=2"} {
		if !strings.Contains(text, want) {
			t.Errorf("the output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "ERROR") {
		t.Errorf("a passing check printed ERROR:\n%s", text)
	}
	// The certificate is listed once, not once per attempt.
	if n := strings.Count(text, "certificate depth=0"); n != 1 {
		t.Errorf("certificate depth=0 printed %d times, want 1", n)
	}
	if doc.ModuleName != "tls" || len(stats) != 2 {
		t.Fatalf("JSON: module %q, %d stats", doc.ModuleName, len(stats))
	}
	s := stats[0]
	if !s.Success || !s.Verified || s.TLSVersion != "TLS 1.3" || s.CipherSuite == "" || s.Trust != "system and --cacert" ||
		len(s.VerifiedChain) != 2 || len(s.Certificates) != 1 || s.Certificates[0].PublicKey != "ECDSA P-256" ||
		s.ExpiresInDays <= 0 || s.ExpiresInDays > 1 || len(s.Warnings) == 0 || s.Error != "" {
		t.Errorf("JSON stat: %+v", s)
	}
	if c := s.Certificates[0]; c.Subject != "CN=shint-test-server" || len(c.IPAddresses) != 1 || c.DNSNames[0] != "localhost" || len(c.SHA256) != 64 {
		t.Errorf("JSON certificate: %+v", c)
	}
}

func TestTLSFailDaysTurnsExpiryIntoAFailure(t *testing.T) {
	ca := generateTestCA(t)
	port := tlsServer(t, ca.ServerTLSCert)
	text, ok, _, stats := runTLS(t, TLSCheckOptions{Port: port, RootCAs: ca.CAPool, CustomCA: true, FailDays: 1}, 1)
	if ok || !strings.Contains(text, "] ERROR expires too soon") || !strings.Contains(text, "within --fail-days 1") {
		t.Errorf("--fail-days 1 against a certificate that expires within the hour: ok=%v\n%s", ok, text)
	}
	if stats[0].Success || !stats[0].Verified || !strings.Contains(stats[0].Error, "--fail-days 1") {
		t.Errorf("JSON stat: %+v", stats[0])
	}
	// --warn-days 0: no warning at all.
	if strings.Contains(text, "WARN") {
		t.Errorf("--warn-days 0 still warned:\n%s", text)
	}
}

func TestTLSUntrustedNameMismatchExpiredAndPlainTextFail(t *testing.T) {
	ca := generateTestCA(t)
	port := tlsServer(t, ca.ServerTLSCert)
	expired, expiredPool := selfSigned(t, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	expiredPort := tlsServer(t, expired)
	for _, c := range []struct {
		name string
		opts TLSCheckOptions
		want string
	}{
		{"untrusted", TLSCheckOptions{Port: port, RootCAs: x509.NewCertPool()}, "signed by an unknown authority"},
		{"name mismatch", TLSCheckOptions{Port: port, RootCAs: ca.CAPool, ServerName: "other.example"}, "certificate is valid for localhost, not other.example"},
		{"expired", TLSCheckOptions{Port: expiredPort, RootCAs: expiredPool}, "expired on"},
		{"plain text", TLSCheckOptions{Port: plainServer(t), RootCAs: ca.CAPool}, "did not answer with TLS"},
	} {
		t.Run(c.name, func(t *testing.T) {
			text, ok, _, stats := runTLS(t, c.opts, 1)
			if ok {
				t.Fatalf("passed:\n%s", text)
			}
			if !strings.Contains(text, "] ERROR ") || !strings.Contains(text, c.want) || !strings.Contains(text, "passed=0") {
				t.Errorf("the output lacks an ERROR with %q:\n%s", c.want, text)
			}
			if stats[0].Success || !strings.Contains(stats[0].Error, c.want) {
				t.Errorf("JSON stat: %+v", stats[0])
			}
		})
	}
}

func TestTLSServerNameIsSentAndVerified(t *testing.T) {
	ca := generateTestCA(t)
	port := tlsServer(t, ca.ServerTLSCert)
	text, ok, _, stats := runTLS(t, TLSCheckOptions{Port: port, RootCAs: ca.CAPool, ServerName: "localhost"}, 1)
	if !ok || stats[0].ServerName != "localhost" || !strings.Contains(text, "server=localhost address=127.0.0.1") {
		t.Errorf("--servername localhost against a certificate for localhost: ok=%v %+v\n%s", ok, stats[0], text)
	}
}

func TestFormatDays(t *testing.T) {
	for d, want := range map[time.Duration]string{
		72 * time.Hour: "3d", 47 * time.Hour: "47h", 90 * time.Minute: "1h", -72 * time.Hour: "-3d", -5 * time.Hour: "-5h",
	} {
		if got := formatDays(d); got != want {
			t.Errorf("formatDays(%v) = %q, want %q", d, got, want)
		}
	}
}
