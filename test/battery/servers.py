"""Loopback test servers for the battery: HTTP(S) with one path per misbehaviour,
TCP echo/close/silent, UDP echo/silent/big-reply, and a closed TCP and UDP port.
Every server binds port 0 and records the port it got in env.PORTS."""
import gzip
import os
import socket
import ssl
import struct
import threading
import time

from env import PORTS

def rst(conn):
    try:
        conn.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack("ii", 1, 0))
    except OSError:
        pass
    conn.close()

def read_head(conn, limit=4 << 20):
    conn.settimeout(10)
    data = b""
    while b"\r\n\r\n" not in data:
        chunk = conn.recv(65536)
        if not chunk:
            return None, b""
        data += chunk
        if len(data) > limit:
            break
    head, _, rest = data.partition(b"\r\n\r\n")
    return head, rest

def read_body(conn, head, rest):
    clen = 0
    te = b""
    for line in head.split(b"\r\n")[1:]:
        k, _, v = line.partition(b":")
        if k.strip().lower() == b"content-length":
            try: clen = int(v.strip())
            except ValueError: pass
        if k.strip().lower() == b"transfer-encoding":
            te = v.strip().lower()
    body = rest
    if te == b"chunked":
        while not body.endswith(b"0\r\n\r\n"):
            c = conn.recv(65536)
            if not c: break
            body += c
        return body
    while len(body) < clen:
        c = conn.recv(65536)
        if not c: break
        body += c
    return body

def resp(status, body=b"", headers=None, close=False):
    h = {"Content-Length": str(len(body))}
    if headers: h.update(headers)
    if close: h["Connection"] = "close"
    return (f"HTTP/1.1 {status}\r\n" + "".join(f"{k}: {v}\r\n" for k, v in h.items()) + "\r\n").encode() + body

def http_conn(conn, addr):
    try:
        while True:
            head, rest = read_head(conn)
            if head is None: return
            line = head.split(b"\r\n")[0].decode("latin1")
            parts = line.split(" ")
            method, path = parts[0], (parts[1] if len(parts) > 1 else "/")
            body = read_body(conn, head, rest)
            p = path.split("?")[0]
            if method == "HEAD" and p != "/head":  # headers only, as HTTP requires
                conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 42\r\n\r\n")
                continue
            if p == "/ok":
                conn.sendall(resp("200 OK", b"ok"))
            elif p == "/echo":
                conn.sendall(resp("200 OK", head + b"\r\n\r\n" + body, {"Content-Type": "text/plain"}))
            elif p == "/host":
                h = [l for l in head.split(b"\r\n")[1:] if l.lower().startswith(b"host:")]
                conn.sendall(resp("200 OK", (h[0] if h else b"no host header")))
            elif p == "/empty":
                conn.sendall(resp("200 OK", b""))
            elif p == "/trunc":      # promises 1000 bytes, sends 100, closes cleanly
                conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n" + b"x" * 100); conn.close(); return
            elif p == "/reset":      # sends part of the body, then a TCP RST
                conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n" + b"x" * 100); time.sleep(0.2); rst(conn); return
            elif p == "/slowbody":   # headers, 10 bytes, then stalls
                conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n" + b"x" * 10); time.sleep(60); return
            elif p == "/slowhdr":
                time.sleep(60); return
            elif p.startswith("/delay/"):
                time.sleep(float(p.split("/")[2])); conn.sendall(resp("200 OK", b"late"))
            elif p == "/noresp":
                conn.close(); return
            elif p == "/loop":
                conn.sendall(resp("302 Found", b"", {"Location": "/loop"}))
            elif p.startswith("/chain/"):
                n = int(p.split("/")[2])
                if n == 0: conn.sendall(resp("200 OK", b"end"))
                else: conn.sendall(resp("302 Found", b"", {"Location": f"/chain/{n-1}"}))
            elif p == "/rel":
                conn.sendall(resp("302 Found", b"", {"Location": "../ok"}))
            elif p == "/307":
                conn.sendall(resp("307 Temporary Redirect", b"", {"Location": "/echo"}))
            elif p == "/301post":
                conn.sendall(resp("301 Moved Permanently", b"", {"Location": "/echo"}))
            elif p == "/noloc":
                conn.sendall(resp("302 Found", b""))
            elif p == "/big":        # 300 MB
                n = 300 * 1024 * 1024
                conn.sendall(f"HTTP/1.1 200 OK\r\nContent-Length: {n}\r\n\r\n".encode())
                chunk = b"\0" * 65536; sent = 0
                while sent < n:
                    conn.sendall(chunk); sent += len(chunk)
            elif p == "/chunked":
                conn.sendall(b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n6\r\n world\r\n0\r\n\r\n")
            elif p == "/gzip":
                z = gzip.compress(b"hello " * 2000)
                conn.sendall(resp("200 OK", z, {"Content-Encoding": "gzip"}))
            elif p == "/204":
                conn.sendall(b"HTTP/1.1 204 No Content\r\n\r\n")
            elif p == "/304":
                conn.sendall(b"HTTP/1.1 304 Not Modified\r\n\r\n")
            elif p == "/head":
                conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n")
            elif p == "/bighdr":
                conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nX-Big: " + b"a" * (2 << 20) + b"\r\n\r\nok")
            elif p == "/weird":
                conn.sendall(b"HTTP/1.1 999 Weird\r\nContent-Length: 2\r\n\r\nok")
            elif p == "/garbage":
                conn.sendall(b"this is not http\r\n\r\n"); conn.close(); return
            elif p == "/http10":
                conn.sendall(b"HTTP/1.0 200 OK\r\n\r\nold school"); conn.close(); return
            elif p == "/binary":
                conn.sendall(resp("200 OK", bytes(range(256)) * 4, {"Content-Type": "application/octet-stream"}))
            elif p == "/json":
                conn.sendall(resp("200 OK", b'{"a": 1, "b": [1, 2, 3], "c": {"d": null}}', {"Content-Type": "application/json"}))
            elif p == "/badjson":
                conn.sendall(resp("200 OK", b'{"a": 1,', {"Content-Type": "application/json"}))
            elif p == "/bigjson":
                conn.sendall(resp("200 OK", b'{"data": "' + b"x" * (5 << 20) + b'"}', {"Content-Type": "application/json"}))
            elif p.startswith("/status/"):
                code = int(p.split("/")[2]); conn.sendall(resp(f"{code} Status", b"s"))
            else:
                conn.sendall(resp("404 Not Found", b"nope"))
    except Exception:
        try: conn.close()
        except Exception: pass

def serve(name, handler, family=socket.AF_INET, host="127.0.0.1", ctx=None, kind=socket.SOCK_STREAM):
    s = socket.socket(family, kind)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    s.bind((host, 0))
    port = s.getsockname()[1]
    PORTS[name] = port
    if kind == socket.SOCK_DGRAM:
        threading.Thread(target=handler, args=(s,), daemon=True).start()
        return
    s.listen(512)
    def loop():
        while True:
            try: conn, addr = s.accept()
            except OSError: return
            def run(c=conn, a=addr):
                try:
                    if ctx is not None:
                        c = ctx.wrap_socket(c, server_side=True)
                    handler(c, a)
                except Exception:
                    try: c.close()
                    except Exception: pass
            threading.Thread(target=run, daemon=True).start()
    threading.Thread(target=loop, daemon=True).start()

def tcp_echo(c, a):
    c.settimeout(30)
    try:
        while True:
            d = c.recv(65536)
            if not d: break
            c.sendall(d)
    finally:
        c.close()
def tcp_close(c, a): c.close()
def tcp_silent(c, a): time.sleep(120); c.close()
def udp_echo(s):
    while True:
        d, a = s.recvfrom(65536); s.sendto(d, a)
def udp_silent(s):
    while True: s.recvfrom(65536)
def udp_big(s):
    while True:
        d, a = s.recvfrom(65536); s.sendto(b"B" * 4000, a)

# ---------------------------------------------------------------- a DNS server
# UDP and TCP on one port. What a name does is in its first label: ok.example
# answers properly, nx.example is NXDOMAIN, big.example is truncated over UDP and
# complete over TCP, garbage.example answers with junk, silent.example does not
# answer, wrongid.example answers with the wrong ID, and so on (see dns_reply).

def _dns_name(name):
    out = b""
    for label in name.rstrip(".").split("."):
        if label:
            out += bytes([len(label)]) + label.encode()
    return out + b"\0"


def _dns_rr(name, rtype, ttl, rdata):
    return _dns_name(name) + struct.pack(">HHIH", rtype, 1, ttl, len(rdata)) + rdata


def _dns_question(data):
    """(id, name, type, question bytes) of a query, or None."""
    if len(data) < 12:
        return None
    ident, _flags, qd = struct.unpack(">HHH", data[:6])
    if qd != 1:
        return None
    i, labels = 12, []
    while i < len(data) and data[i]:
        n = data[i]
        labels.append(data[i + 1:i + 1 + n].decode("latin1"))
        i += 1 + n
    end = i + 1 + 4
    qtype = struct.unpack(">H", data[i + 1:i + 3])[0]
    return ident, ".".join(labels).lower(), qtype, data[12:end]


def _dns_msg(ident, question, rcode=0, answers=(), authority=(), truncated=False):
    flags = 0x8000 | 0x0100 | 0x0080 | rcode | (0x0200 if truncated else 0)
    head = struct.pack(">HHHHHH", ident, flags, 1, len(answers), len(authority), 0)
    return head + question + b"".join(answers) + b"".join(authority)


def dns_reply(data, tcp):
    """The bytes to send back for a query, or None to stay silent."""
    q = _dns_question(data)
    if q is None:
        return None
    ident, name, qtype, question = q
    soa = _dns_rr("example", 6, 300, _dns_name("ns.example") + _dns_name("hostmaster.example") + struct.pack(">IIIII", 1, 7200, 3600, 1209600, 300))
    if name == "silent.example":
        return None
    if name == "garbage.example":
        return b"this is not a dns reply"
    if name == "wrongid.example":
        return _dns_msg(ident ^ 0xFFFF, question, 0, [_dns_rr(name, 1, 30, bytes([203, 0, 113, 66]))])
    if name == "servfail.example":
        return _dns_msg(ident, question, 2)
    if name == "refused.example":
        return _dns_msg(ident, question, 5)
    if name == "nodata.example":
        return _dns_msg(ident, question, 0, [], [soa])
    if name == "big.example":
        if not tcp:
            return _dns_msg(ident, question, 0, [], [], truncated=True)
        return _dns_msg(ident, question, 0, [_dns_rr(name, 1, 30, bytes([192, 0, 2, 7]))])
    if name.endswith("in-addr.arpa") or name.endswith("ip6.arpa"):
        if qtype == 12:
            return _dns_msg(ident, question, 0, [_dns_rr(name, 12, 300, _dns_name("host.example"))])
        return _dns_msg(ident, question, 0, [], [soa])
    if name == "txt.example" and qtype == 16:
        strings = [b"\x1b[31mred\x1b[0m", b"line1\nline2\x00", b"\xff\xfe"]
        rdata = b"".join(bytes([len(x)]) + x for x in strings)
        return _dns_msg(ident, question, 0, [_dns_rr(name, 16, 60, rdata)])
    if name in ("ok.example", "txt.example"):
        answers = {1: _dns_rr(name, 1, 300, bytes([192, 0, 2, 1])),
                   28: _dns_rr(name, 28, 300, bytes.fromhex("20010db8000000000000000000000001")),
                   15: _dns_rr(name, 15, 300, struct.pack(">H", 10) + _dns_name("mail.ok.example")),
                   2: _dns_rr(name, 2, 300, _dns_name("ns.example")),
                   16: _dns_rr(name, 16, 300, b"\x0bv=spf1 -all"),
                   6: _dns_rr(name, 6, 300, _dns_name("ns.example") + _dns_name("hostmaster.example") + struct.pack(">IIIII", 1, 7200, 3600, 1209600, 300)),
                   257: _dns_rr(name, 257, 300, b"\x00\x05issueletsencrypt.org")}
        if qtype in answers:
            return _dns_msg(ident, question, 0, [answers[qtype]])
        return _dns_msg(ident, question, 0, [], [soa])
    return _dns_msg(ident, question, 3, [], [soa])


def start_dns():
    """UDP and TCP on the same port; records it as PORTS['dns']. Returns True if it started."""
    for _ in range(20):
        u = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        u.bind(("127.0.0.1", 0))
        port = u.getsockname()[1]
        t = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        t.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            t.bind(("127.0.0.1", port))
        except OSError:
            u.close()
            t.close()
            continue
        t.listen(64)

        def udp_loop():
            while True:
                try:
                    data, addr = u.recvfrom(4096)
                except OSError:
                    return
                out = dns_reply(data, False)
                if out is not None:
                    u.sendto(out, addr)

        def tcp_conn(c):
            try:
                c.settimeout(10)
                size = c.recv(2)
                if len(size) < 2:
                    return
                body = b""
                want = struct.unpack(">H", size)[0]
                while len(body) < want:
                    chunk = c.recv(want - len(body))
                    if not chunk:
                        return
                    body += chunk
                out = dns_reply(body, True)
                if out is not None:
                    c.sendall(struct.pack(">H", len(out)) + out)
            except OSError:
                pass
            finally:
                c.close()

        def tcp_loop():
            while True:
                try:
                    c, _ = t.accept()
                except OSError:
                    return
                threading.Thread(target=tcp_conn, args=(c,), daemon=True).start()

        threading.Thread(target=udp_loop, daemon=True).start()
        threading.Thread(target=tcp_loop, daemon=True).start()
        PORTS["dns"] = port
        return True
    return False


def tlsctx(cert, key):
    c = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    c.load_cert_chain(cert, key)
    return c


def ipv6_loopback_available():
    try:
        t = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
        t.bind(("::1", 0))
        t.close()
        return True
    except OSError:
        return False


def start(certdir):
    """Start every server; fills env.PORTS. certdir holds good.pem/good.key and
    bad.pem/bad.key (see gencert) - without them the HTTPS servers are skipped."""
    serve("http", http_conn)
    have_v6 = ipv6_loopback_available()
    if have_v6:
        serve("http6", http_conn, socket.AF_INET6, "::1")
        serve("tcp_echo6", tcp_echo, socket.AF_INET6, "::1")
        serve("udp_echo6", udp_echo, socket.AF_INET6, "::1", kind=socket.SOCK_DGRAM)
    if certdir and os.path.exists(os.path.join(certdir, "good.pem")):
        serve("https", http_conn, ctx=tlsctx(f"{certdir}/good.pem", f"{certdir}/good.key"))
        serve("https_bad", http_conn, ctx=tlsctx(f"{certdir}/bad.pem", f"{certdir}/bad.key"))
    serve("tcp_echo", tcp_echo)
    serve("tcp_close", tcp_close)
    serve("tcp_silent", tcp_silent)
    serve("udp_echo", udp_echo, kind=socket.SOCK_DGRAM)
    serve("udp_silent", udp_silent, kind=socket.SOCK_DGRAM)
    serve("udp_big", udp_big, kind=socket.SOCK_DGRAM)
    # a closed UDP and TCP port: bind, note the number, release
    for name, kind in (("udp_closed", socket.SOCK_DGRAM), ("tcp_closed", socket.SOCK_STREAM)):
        t = socket.socket(socket.AF_INET, kind)
        t.bind(("127.0.0.1", 0))
        PORTS[name] = t.getsockname()[1]
        t.close()
    have_dns = start_dns()
    return {"ipv6": have_v6, "tls": "https" in PORTS, "dns": have_dns}
