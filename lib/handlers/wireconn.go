package handlers

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync"
	"sync/atomic"

	"github.com/dmartsapp/shint/v4/lib"
)

// countingConn wraps a net.Conn and tallies every byte that crosses it in
// each direction. Both "web" (client side) and "listen http" (server side)
// measure through it, which is what makes their figures directly comparable:
// each side counts everything the HTTP layer put on / took off the
// connection - request or status line, headers and body alike, not just the
// body - so what web reports as sent is what listen http reports as
// received, and vice versa. Over TLS the count is taken on the decrypted
// side of the connection (the HTTP bytes, without TLS record overhead).
type countingConn struct {
	net.Conn
	read    atomic.Int64
	written atomic.Int64

	// onClose, if set before the connection is first used, runs exactly
	// once when the connection is closed - the point at which read/written
	// are final. Used by the listen side to report a request's byte totals
	// only after net/http has finished with the connection.
	onClose   func(*countingConn)
	closeOnce sync.Once
	closeErr  error

	// head is the first few bytes written to the connection: enough to read the
	// status line off a response net/http wrote by itself for a request it could
	// not parse, which never passes through a handler.
	headMu sync.Mutex
	head   []byte
}

// headBytes is how much of the start of a response countingConn remembers.
const headBytes = 16

func (c *countingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.read.Add(int64(n))
	return n, err
}

func (c *countingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.written.Add(int64(n))
	if n > 0 {
		c.headMu.Lock()
		if room := headBytes - len(c.head); room > 0 {
			c.head = append(c.head, p[:min(n, room)]...)
		}
		c.headMu.Unlock()
	}
	return n, err
}

// firstWritten is the start of what was written to the connection (at most
// headBytes bytes).
func (c *countingConn) firstWritten() []byte {
	c.headMu.Lock()
	defer c.headMu.Unlock()
	return append([]byte(nil), c.head...)
}

func (c *countingConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.Conn.Close()
		if c.onClose != nil {
			c.onClose(c)
		}
	})
	return c.closeErr
}

// CloseWrite passes a half-close through to the wrapped connection. net/http
// looks for this method on the conn when it must close after leaving part of
// a request body unread (it half-closes and waits briefly so the client
// isn't handed a TCP reset instead of the response); without it the wrapper
// would silently drop that behavior.
func (c *countingConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return nil
}

// countingListener wraps every accepted connection in a countingConn.
type countingListener struct{ net.Listener }

func (l countingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: conn}, nil
}

// newCountingTransport builds the web command's HTTP/1.1 transport with
// every connection wrapped in a countingConn. HTTPS connections are dialed
// and handshaken here (rather than by http.Transport) so the counter sits
// above TLS and counts HTTP bytes, matching what an http:// listener sees.
// Supplying a custom TLS config already keeps http.Transport on HTTP/1.1, so
// framing stays the plain byte stream both sides count.
func newCountingTransport(tlsConfig *tls.Config) *http.Transport {
	var dialer net.Dialer
	return &http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, lib.DialNetwork(network), addr)
			if err != nil {
				return nil, err
			}
			return &countingConn{Conn: conn}, nil
		},
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			raw, err := dialer.DialContext(ctx, lib.DialNetwork(network), addr)
			if err != nil {
				return nil, err
			}
			cfg := tlsConfig.Clone()
			if cfg.ServerName == "" {
				if host, _, splitErr := net.SplitHostPort(addr); splitErr == nil {
					cfg.ServerName = host
				} else {
					cfg.ServerName = addr
				}
			}
			conn := tls.Client(raw, cfg)
			// net/http reports the handshake to httptrace only when it does
			// the handshake itself; this dialer does it, so it reports it too
			// (that is how web --timing sees the TLS time).
			trace := httptrace.ContextClientTrace(ctx)
			if trace != nil && trace.TLSHandshakeStart != nil {
				trace.TLSHandshakeStart()
			}
			err = conn.HandshakeContext(ctx)
			if trace != nil && trace.TLSHandshakeDone != nil {
				trace.TLSHandshakeDone(conn.ConnectionState(), err)
			}
			if err != nil {
				_ = raw.Close()
				return nil, err
			}
			return &countingConn{Conn: conn}, nil
		},
	}
}

// wireMeter measures the bytes one client.Do call put on the wire, across
// every connection it touched. That is more than one connection when the
// client follows a redirect to another host, and a connection may be a reused
// keep-alive one that already carried earlier requests, so each connection is
// snapshotted the first time this call gets it and only the difference
// counts. Use one wireMeter per request, wired in through clientTrace.
type wireMeter struct{ spans []wireSpan }

type wireSpan struct {
	conn                      *countingConn
	readBefore, writtenBefore int64
}

func (m *wireMeter) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		// Runs on the goroutine that called client.Do, once per hop.
		GotConn: func(info httptrace.GotConnInfo) {
			cc, ok := info.Conn.(*countingConn)
			if !ok {
				return
			}
			for _, sp := range m.spans {
				if sp.conn == cc {
					return
				}
			}
			m.spans = append(m.spans, wireSpan{conn: cc, readBefore: cc.read.Load(), writtenBefore: cc.written.Load()})
		},
	}
}

// totals returns the bytes written to and read from the wire since the
// meter's connections were first handed to the request.
func (m *wireMeter) totals() (sent, received int64) {
	for _, sp := range m.spans {
		sent += sp.conn.written.Load() - sp.writtenBefore
		received += sp.conn.read.Load() - sp.readBefore
	}
	return sent, received
}
