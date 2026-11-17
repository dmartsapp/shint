package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestTheJSONSaysHowCertificatesAreTreated: what web's text mode notes in a
// line each - -k, --cert, --cacert - is in the JSON too (#100).
func TestTheJSONSaysHowCertificatesAreTreated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	type out struct {
		InputParams struct {
			TLS *struct {
				VerifyCertificates bool `json:"verify_certificates"`
				ClientCertificate  bool `json:"client_certificate"`
				CustomCA           bool `json:"custom_ca"`
			} `json:"tls"`
		} `json:"input_params"`
		Warnings []string `json:"warnings"`
	}
	run := func(cfg *tls.Config) (out, string) {
		jsonOut, throttle := true, false
		raw := captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOut, 1, 0, &throttle, 5, u, "GET", "", nil, false, cfg, false, false, 0)
		})
		var o out
		if err := json.Unmarshal([]byte(raw), &o); err != nil || o.InputParams.TLS == nil {
			t.Fatalf("no input_params.tls (%v):\n%s", err, raw)
		}
		return o, raw
	}

	o, raw := run(nil)
	if tl := o.InputParams.TLS; !tl.VerifyCertificates || tl.ClientCertificate || tl.CustomCA || len(o.Warnings) != 0 || strings.Contains(raw, `"warnings"`) {
		t.Errorf("default: %+v, warnings %v", *tl, o.Warnings)
	}

	insecure, _ := BuildTLSConfig("", "", "", true)
	o, _ = run(insecure)
	if o.InputParams.TLS.VerifyCertificates || len(o.Warnings) != 1 || !strings.Contains(o.Warnings[0], "tls verification disabled") {
		t.Errorf("-k: %+v, warnings %v", *o.InputParams.TLS, o.Warnings)
	}

	ca := generateTestCA(t)
	withCert, err := BuildTLSConfig(ca.CAFile, ca.ClientCertFile, ca.ClientKeyFile, false)
	if err != nil {
		t.Fatal(err)
	}
	o, _ = run(withCert)
	if tl := o.InputParams.TLS; !tl.VerifyCertificates || !tl.ClientCertificate || !tl.CustomCA || len(o.Warnings) != 0 {
		t.Errorf("--cert --cacert: %+v, warnings %v", *tl, o.Warnings)
	}
}
