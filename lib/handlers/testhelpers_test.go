package handlers

import (
	"bytes"
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
	"testing"
	"time"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it, so handler functions that print directly to
// stdout (rather than returning a value) can be asserted against.
//
// The pipe is drained by a goroutine running concurrently with fn, not
// afterwards: os.Pipe has a limited kernel buffer (64KB on macOS/Linux), and
// a handler that logs more than that before returning - nmap's JSON output
// over a few hundred ports does - would otherwise block forever on the
// write with nothing yet reading the other end.
func captureStdout(t *testing.T, fn func()) string {
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
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free TCP port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("failed to find a free UDP port: %v", err)
	}
	defer func() { _ = c.Close() }()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// testCA bundles a self-signed CA plus a server and client certificate
// signed by it, written to PEM files under a temp directory, for exercising
// BuildTLSConfig and mTLS end-to-end without hitting the network.
type testCA struct {
	CAFile         string
	ServerCertFile string
	ServerKeyFile  string
	ClientCertFile string
	ClientKeyFile  string
	ServerTLSCert  tls.Certificate
	CAPool         *x509.CertPool
}

func generateTestCA(t *testing.T) *testCA {
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
	writePEM(t, caFile, "CERTIFICATE", caDER)

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	srvCertFile, srvKeyFile, srvTLSCert := issueCert(t, dir, "server", caCert, caKey, x509.ExtKeyUsageServerAuth, []net.IP{net.ParseIP("127.0.0.1")}, []string{"localhost"})
	cliCertFile, cliKeyFile, _ := issueCert(t, dir, "client", caCert, caKey, x509.ExtKeyUsageClientAuth, nil, nil)

	return &testCA{
		CAFile:         caFile,
		ServerCertFile: srvCertFile,
		ServerKeyFile:  srvKeyFile,
		ClientCertFile: cliCertFile,
		ClientKeyFile:  cliKeyFile,
		ServerTLSCert:  srvTLSCert,
		CAPool:         pool,
	}
}

func issueCert(t *testing.T, dir, name string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, usage x509.ExtKeyUsage, ips []net.IP, dnsNames []string) (certFile, keyFile string, tlsCert tls.Certificate) {
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
	writePEM(t, certFile, "CERTIFICATE", der)

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal %s key: %v", name, err)
	}
	keyFile = filepath.Join(dir, name+"-key.pem")
	writePEM(t, keyFile, "EC PRIVATE KEY", keyBytes)

	tlsCert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("load %s key pair: %v", name, err)
	}
	return certFile, keyFile, tlsCert
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
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
