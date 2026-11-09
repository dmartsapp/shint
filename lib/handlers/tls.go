package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildTLSConfig assembles the *tls.Config used by the web command from
// optional CLI-supplied material:
//   - cacertPath: a PEM file of one or more CA certificates appended to the
//     system trust pool, so a self-signed or internal CA can be trusted for
//     verifying the server's certificate.
//   - certPath/keyPath: a PEM client certificate/key pair presented to the
//     server for mutual TLS (mTLS) authentication. Both must be given together.
//   - insecure: skips server certificate verification entirely (like curl -k).
//     Only intended for quick diagnostics against hosts with self-signed
//     certificates when the CA is not available.
func BuildTLSConfig(cacertPath, certPath, keyPath string, insecure bool) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: insecure}

	if cacertPath != "" {
		pool, err := LoadCAPool(cacertPath)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = pool
	}

	if (certPath == "") != (keyPath == "") {
		return nil, fmt.Errorf("both --cert and --key must be provided together for mutual TLS")
	}
	if certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("unable to load client certificate/key pair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}

// LoadCAPool is what --cacert trusts, for web and tls: the system's trust pool with
// the certificates of a PEM file added, so a self-signed or internal CA can be
// trusted alongside the public ones.
func LoadCAPool(cacertPath string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(cacertPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read CA certificate file %q: %w", cacertPath, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no valid PEM certificates found in CA file %q", cacertPath)
	}
	return pool, nil
}
