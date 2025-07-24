package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

	// --- Execute WebHandler and capture its output ---
	
	// Redirect stdout to a buffer to capture the output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

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

	// Run the handler
	WebHandler(&jsonOutput, iterations, delay, &throttle, timeout, serverURL, method, data, headers, includeBody)

	// Restore stdout and read captured output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	
	// --- Validate the output ---
	outputStr := buf.String()
	if !strings.Contains(outputStr, `"status_code":200`) {
		t.Errorf("Expected status code 200 in output, got:\n%s", outputStr)
	}

	if !strings.Contains(outputStr, `"method":"POST"`) {
		t.Errorf("Expected method POST in JSON output, got:\n%s", outputStr)
	}

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

	log.Println("WebHandler unit test passed.")
}