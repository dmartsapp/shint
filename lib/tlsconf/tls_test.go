package tlsconf

import (
	"os"
	"testing"

	"github.com/dmartsapp/shint/v4/lib/internal/testutil"
)

func TestBuildTLSConfigDefaults(t *testing.T) {
	cfg, err := Build("", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=false by default")
	}
	if cfg.RootCAs != nil {
		t.Error("expected no custom RootCAs by default")
	}
	if len(cfg.Certificates) != 0 {
		t.Error("expected no client certificates by default")
	}
}

func TestBuildTLSConfigInsecure(t *testing.T) {
	cfg, err := Build("", "", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
}

func TestBuildTLSConfigWithCACert(t *testing.T) {
	ca := testutil.NewCA(t)
	cfg, err := Build(ca.CAFile, "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("expected RootCAs to be set")
	}
	// Full end-to-end verification that a server certificate signed by this
	// CA is actually trusted is covered by TestWebHandlerMutualTLS.
}

func TestBuildTLSConfigCACertMissingFile(t *testing.T) {
	_, err := Build("/nonexistent/path/ca.pem", "", "", false)
	if err == nil {
		t.Fatal("expected error for missing CA file, got nil")
	}
}

func TestBuildTLSConfigCACertInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	badFile := dir + "/bad-ca.pem"
	if err := os.WriteFile(badFile, []byte("not a valid pem"), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	_, err := Build(badFile, "", "", false)
	if err == nil {
		t.Fatal("expected error for invalid PEM content, got nil")
	}
}

func TestBuildTLSConfigClientCertAndKey(t *testing.T) {
	ca := testutil.NewCA(t)
	cfg, err := Build("", ca.ClientCertFile, ca.ClientKeyFile, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected exactly one client certificate, got %d", len(cfg.Certificates))
	}
}

func TestBuildTLSConfigCertWithoutKey(t *testing.T) {
	ca := testutil.NewCA(t)
	_, err := Build("", ca.ClientCertFile, "", false)
	if err == nil {
		t.Fatal("expected error when --cert is given without --key")
	}
}

func TestBuildTLSConfigKeyWithoutCert(t *testing.T) {
	ca := testutil.NewCA(t)
	_, err := Build("", "", ca.ClientKeyFile, false)
	if err == nil {
		t.Fatal("expected error when --key is given without --cert")
	}
}

func TestBuildTLSConfigBadCertKeyPair(t *testing.T) {
	ca := testutil.NewCA(t)
	// Mismatched cert/key pair (server cert with client key) must fail to load.
	_, err := Build("", ca.ServerCertFile, ca.ClientKeyFile, false)
	if err == nil {
		t.Fatal("expected error for mismatched cert/key pair, got nil")
	}
}
