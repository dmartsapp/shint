package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
)

// bodyServer answers every connection with headers that promise wantBytes of
// body and then misbehaves as end says: "close" sends part of it and closes
// cleanly, "reset" sends part of it and resets the connection, "stall" sends
// part of it and goes quiet.
func bodyServer(t *testing.T, end string) *url.URL {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				buf := make([]byte, 4096)
				_, _ = conn.Read(buf)
				_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n" + strings.Repeat("x", 100)))
				switch end {
				case "reset":
					time.Sleep(100 * time.Millisecond)
					_ = conn.(*net.TCPConn).SetLinger(0)
					_ = conn.Close()
				case "stall":
					time.Sleep(10 * time.Second)
					_ = conn.Close()
				default:
					_ = conn.Close()
				}
			}(conn)
		}
	}()
	u, _ := url.Parse("http://" + listener.Addr().String() + "/")
	return u
}

// A response body cut short used to be reported as a success with fewer bytes:
// the read error was dropped. It is a failed attempt, in text and in JSON.
func TestWebIncompleteBodyIsAFailure(t *testing.T) {
	for _, end := range []string{"close", "reset", "stall"} {
		t.Run(end, func(t *testing.T) {
			target := bodyServer(t, end)
			timeout := 5
			if end == "stall" {
				timeout = 1
			}
			throttle := false

			text := false
			var ok bool
			out := captureStdout(t, func() {
				ok = WebHandler(context.Background(), &text, 1, 0, &throttle, timeout, target, "GET", "", nil, false, nil)
			})
			if ok {
				t.Errorf("WebHandler reported success for a body that ended early; output:\n%s", out)
			}
			if !strings.Contains(out, "ERROR response incomplete") {
				t.Errorf("no 'ERROR response incomplete' line; output:\n%s", out)
			}
			if strings.Contains(out, "OK response") {
				t.Errorf("an OK response line was printed for a body that ended early:\n%s", out)
			}

			asJSON := true
			out = captureStdout(t, func() {
				ok = WebHandler(context.Background(), &asJSON, 1, 0, &throttle, timeout, target, "GET", "", nil, false, nil)
			})
			if ok {
				t.Errorf("--json: WebHandler reported success for a body that ended early")
			}
			var doc struct {
				Stats []struct {
					Success bool     `json:"success"`
					Errors  []string `json:"errors"`
				} `json:"stats"`
			}
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("--json output is not valid JSON: %v\n%s", err, out)
			}
			if len(doc.Stats) != 1 || doc.Stats[0].Success || len(doc.Stats[0].Errors) == 0 {
				t.Errorf("--json: want one stat with success=false and an error, got %+v", doc.Stats)
			}
		})
	}
}
