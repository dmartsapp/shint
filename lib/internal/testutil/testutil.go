package testutil

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dmartsapp/shint/v4/lib"
)

// CaptureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it, so handler functions that print directly to
// stdout (rather than returning a value) can be asserted against.
//
// The pipe is drained by a goroutine running concurrently with fn, not
// afterwards: os.Pipe has a limited kernel buffer (64KB on macOS/Linux), and
// a handler that logs more than that before returning - nmap's JSON output
// over a few hundred ports does - would otherwise block forever on the
// write with nothing yet reading the other end.
func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	os.Stdout = old
	_ = w.Close()
	return <-outCh
}

// freePort asks the OS for an ephemeral TCP port and immediately frees it,
// for tests that need a port number known in advance to be unused.
func FreeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free TCP port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func FreeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("failed to find a free UDP port: %v", err)
	}
	defer func() { _ = c.Close() }()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// CA bundles a self-signed CA plus a server and client certificate
// signed by it, written to PEM files under a temp directory, for exercising
// tlsconf.Build and mTLS end-to-end without hitting the network.
type CA struct {
	CAFile         string
	ServerCertFile string
	ServerKeyFile  string
	ClientCertFile string
	ClientKeyFile  string
	ServerTLSCert  tls.Certificate
	CAPool         *x509.CertPool
}

func NewCA(t *testing.T) *CA {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "shint-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	caFile := filepath.Join(dir, "ca.pem")
	WritePEM(t, caFile, "CERTIFICATE", caDER)

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	srvCertFile, srvKeyFile, srvTLSCert := IssueCert(t, dir, "server", caCert, caKey, x509.ExtKeyUsageServerAuth, []net.IP{net.ParseIP("127.0.0.1")}, []string{"localhost"})
	cliCertFile, cliKeyFile, _ := IssueCert(t, dir, "client", caCert, caKey, x509.ExtKeyUsageClientAuth, nil, nil)

	return &CA{
		CAFile:         caFile,
		ServerCertFile: srvCertFile,
		ServerKeyFile:  srvKeyFile,
		ClientCertFile: cliCertFile,
		ClientKeyFile:  cliKeyFile,
		ServerTLSCert:  srvTLSCert,
		CAPool:         pool,
	}
}

func IssueCert(t *testing.T, dir, name string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, usage x509.ExtKeyUsage, ips []net.IP, dnsNames []string) (certFile, keyFile string, tlsCert tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate %s key: %v", name, err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "shint-test-" + name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  ips,
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create %s cert: %v", name, err)
	}
	certFile = filepath.Join(dir, name+".pem")
	WritePEM(t, certFile, "CERTIFICATE", der)

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal %s key: %v", name, err)
	}
	keyFile = filepath.Join(dir, name+"-key.pem")
	WritePEM(t, keyFile, "EC PRIVATE KEY", keyBytes)

	tlsCert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("load %s key pair: %v", name, err)
	}
	return certFile, keyFile, tlsCert
}

func WritePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("encode PEM for %s: %v", path, err)
	}
}

// WaitForListener polls the port with a raw TCP dial (never sending any
// bytes) until it accepts a connection or the timeout elapses, since
// listen.HTTP's listener binds asynchronously on its own goroutine.
// Deliberately not an HTTP request: that would itself count against
// maxRequests and throw off tests asserting an exact request count.
func WaitForListener(t *testing.T, port int) {
	t.Helper()
	WaitForListenerOn(t, "127.0.0.1", port)
}

func WaitForListenerOn(t *testing.T, host string, port int) {
	t.Helper()
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("listener on port %d did not become ready in time", port)
}

func MustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

func UseFamily(t *testing.T, v4, v6 bool) {
	t.Helper()
	if err := lib.SetIPFamily(v4, v6); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.SetIPFamily(false, false) })
}

func EchoListener(t *testing.T) (port int, closeFn func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(portStr)
	return p, func() { _ = listener.Close() }
}

func EchoListenerIPv6(t *testing.T) (port int, closeFn func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Fatalf("failed to start IPv6 listener: %v", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(portStr)
	return p, func() { _ = listener.Close() }
}

// CancelAfter returns a context that is cancelled (as Ctrl+C would) after d.
func CancelAfter(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(d, cancel)
	t.Cleanup(func() { timer.Stop(); cancel() })
	return ctx
}

// InterruptedCounts pulls the numbers out of the "interrupted" line.
func InterruptedCounts(t *testing.T, out string) (completed, planned int) {
	t.Helper()
	m := interruptedRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no \"interrupted\" line in the output:\n%s", out)
	}
	completed, _ = strconv.Atoi(m[1])
	planned, _ = strconv.Atoi(m[2])
	return completed, planned
}

func RunWithin(t *testing.T, limit time.Duration, fn func()) string {
	t.Helper()
	start := time.Now()
	out := CaptureStdout(t, fn)
	if took := time.Since(start); took > limit {
		t.Errorf("the run took %v after Ctrl+C, want it to wind down within %v", took, limit)
	}
	return out
}

var interruptedRe = regexp.MustCompile(`ERROR interrupted attempts_completed=(\d+) attempts_planned=(\d+)`)
