// Command gencert writes the two throwaway TLS certificates the black-box test
// battery (../README.md) serves from its HTTPS test servers:
//
//	good.pem / good.key  valid for localhost, 127.0.0.1 and ::1
//	bad.pem  / bad.key   valid only for other.example, so a client that connects
//	                     to 127.0.0.1 must refuse it
//
// Both are self-signed, expire after a day, and are written to the directory
// given as the only argument. Nothing here is ever a real credential.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func write(dir, name, kind string, der []byte) error {
	data := pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der})
	return os.WriteFile(filepath.Join(dir, name), data, 0o600)
}

func issue(dir, name, cn string, dnsNames []string, ips []net.IP) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true, // self-signed, and used as its own trust anchor via --cacert
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	if err := write(dir, name+".pem", "CERTIFICATE", der); err != nil {
		return err
	}
	return write(dir, name+".key", "EC PRIVATE KEY", keyDER)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gencert <output directory>")
		os.Exit(2)
	}
	dir := os.Args[1]
	loopback := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	if err := issue(dir, "good", "localhost", []string{"localhost"}, loopback); err != nil {
		fmt.Fprintln(os.Stderr, "gencert:", err)
		os.Exit(1)
	}
	if err := issue(dir, "bad", "other.example", []string{"other.example"}, nil); err != nil {
		fmt.Fprintln(os.Stderr, "gencert:", err)
		os.Exit(1)
	}
}
