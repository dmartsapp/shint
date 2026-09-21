package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestWebHandler tests the WebHandler function.
func TestWebHandler(t *testing.T) {
	// Mock server that asserts request properties
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test Headers
		if r.Header.Get("X-Test-Header") != "TestValue" {
			t.Errorf("Expected header 'X-Test-Header' to be 'TestValue', got '%s'", r.Header.Get("X-Test-Header"))
		}

		// Test Method
		if r.Method != http.MethodPost {
			t.Errorf("Expected method 'POST', got '%s'", r.Method)
		}

		// Test Body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}
		if string(body) != `{"key":"value"}` {
			t.Errorf("Expected body '{\"key\":\"value\"}', got '%s'", string(body))
		}

		// Send response
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Setup WebHandler parameters
	jsonOutput := true
	includeBody := true
	iterations := 1
	delay := 0
	throttle := false
	timeout := 5
	serverURL, _ := url.Parse(server.URL)
	method := "POST"
	data := `{"key":"value"}`
	headers := []string{"X-Test-Header: TestValue"}

	// Run the handler, capturing its stdout output
	outputStr := captureStdout(t, func() {
		WebHandler(context.Background(), &jsonOutput, iterations, delay, &throttle, timeout, serverURL, method, data, headers, includeBody, nil)
	})

	// --- Validate the output ---
	// Unmarshal to inspect JSON details
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(outputStr), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON output: %v", err)
	}

	statsList, ok := result["stats"].([]interface{})
	if !ok || len(statsList) == 0 {
		t.Fatal("No stats found in JSON output")
	}
	firstStat, ok := statsList[0].(map[string]interface{})
	if !ok {
		t.Fatal("Could not parse first stat entry")
	}

	if statusCode, ok := firstStat["status_code"].(float64); !ok || statusCode != 200 {
		t.Errorf("Expected status_code 200 in output, got '%v'", firstStat["status_code"])
	}

	request, ok := firstStat["request"].(map[string]interface{})
	if !ok {
		t.Fatal("No request object in stats")
	}
	if request["method"] != "POST" {
		t.Errorf("Expected request method POST, got '%v'", request["method"])
	}
	if request["body"] != data {
		t.Errorf("Expected request body to echo the sent payload %q, got '%v'", data, request["body"])
	}

	response, ok := firstStat["response"].(map[string]interface{})
	if !ok {
		t.Fatal("No response object in stats")
	}
	responseBody, ok := response["body"].(map[string]interface{})
	if !ok {
		t.Fatal("No response body in stats response")
	}
	if responseBody["status"] != "ok" {
		t.Errorf("Expected response body status 'ok', got '%v'", responseBody["status"])
	}

	// bytes_received / bytes_sent cover everything on the wire, headers
	// included, not just the bodies - and bandwidth_kbs is derived from
	// bytes_received, not from a bug that summed the number of header keys
	// instead of bytes.
	bodyBytes := len(`{"status":"ok"}`)
	bytesReceived, ok := firstStat["bytes_received"].(float64)
	if !ok || int(bytesReceived) <= bodyBytes {
		t.Errorf("Expected bytes_received > body-only size (%d), got %v", bodyBytes, firstStat["bytes_received"])
	}
	bytesSent, ok := firstStat["bytes_sent"].(float64)
	if !ok || int(bytesSent) <= len(data) {
		t.Errorf("Expected bytes_sent > request body size (%d), got %v", len(data), firstStat["bytes_sent"])
	}
	bandwidthKBs, ok := firstStat["bandwidth_kbs"].(float64)
	if !ok || bandwidthKBs <= 0 {
		t.Errorf("Expected a positive bandwidth_kbs, got %v", firstStat["bandwidth_kbs"])
	}

	log.Println("WebHandler unit test passed.")
}

// TestWebHandlerMutualTLS exercises the --cacert/--cert/--key path end to
// end: a server that requires and verifies a client certificate, hit
// through BuildTLSConfig's assembled tls.Config.
func TestWebHandlerMutualTLS(t *testing.T) {
	ca := generateTestCA(t)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.ServerTLSCert},
		ClientCAs:    ca.CAPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	server.StartTLS()
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)

	t.Run("succeeds with matching client cert and CA", func(t *testing.T) {
		tlsConfig, err := BuildTLSConfig(ca.CAFile, ca.ClientCertFile, ca.ClientKeyFile, false)
		if err != nil {
			t.Fatalf("BuildTLSConfig error: %v", err)
		}
		jsonOutput, throttle := true, false
		out := captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, serverURL, "GET", "", nil, false, tlsConfig)
		})
		if !strings.Contains(out, `"status_code": 200`) {
			t.Errorf("expected a successful 200 response, got:\n%s", out)
		}
	})

	t.Run("fails without a client cert", func(t *testing.T) {
		tlsConfig, err := BuildTLSConfig(ca.CAFile, "", "", false)
		if err != nil {
			t.Fatalf("BuildTLSConfig error: %v", err)
		}
		jsonOutput, throttle := false, false
		out := captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, serverURL, "GET", "", nil, false, tlsConfig)
		})
		if !strings.Contains(out, "ERROR") {
			t.Errorf("expected a TLS handshake error without a client cert, got:\n%s", out)
		}
	})

	t.Run("fails without trusting the CA", func(t *testing.T) {
		tlsConfig, err := BuildTLSConfig("", ca.ClientCertFile, ca.ClientKeyFile, false)
		if err != nil {
			t.Fatalf("BuildTLSConfig error: %v", err)
		}
		jsonOutput, throttle := false, false
		out := captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, serverURL, "GET", "", nil, false, tlsConfig)
		})
		if !strings.Contains(out, "ERROR") {
			t.Errorf("expected a certificate trust error without --cacert, got:\n%s", out)
		}
	})

	t.Run("insecure flag skips server verification", func(t *testing.T) {
		tlsConfig, err := BuildTLSConfig("", ca.ClientCertFile, ca.ClientKeyFile, true)
		if err != nil {
			t.Fatalf("BuildTLSConfig error: %v", err)
		}
		jsonOutput, throttle := true, false
		out := captureStdout(t, func() {
			WebHandler(context.Background(), &jsonOutput, 1, 0, &throttle, 5, serverURL, "GET", "", nil, false, tlsConfig)
		})
		if !strings.Contains(out, `"status_code": 200`) {
			t.Errorf("expected a successful 200 response with --insecure, got:\n%s", out)
		}
	})
}
