"""The listen command (byte counts, malformed input, timeouts), signals, and web's memory use.

These drive shint from a small script instead of a single command line, because the
listener is the thing under test and the script is its client."""
import os
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time

import env
from runner import add, add_func, fmt, spawn


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


def start_listener(args):
    """Start `shint listen ...`; its output goes to files (a pipe nobody drains would
    stall a listener that logs one line per read). Returns (process, read_output)."""
    fo, fe = tempfile.TemporaryFile("w+"), tempfile.TemporaryFile("w+")
    p = subprocess.Popen([env.BIN] + args, stdout=fo, stderr=fe, stdin=subprocess.DEVNULL, text=True)
    time.sleep(0.5)

    def read():
        fo.seek(0)
        fe.seek(0)
        return fo.read(), fe.read()
    return p, read


def finish(p, read, wait=5):
    """Wait for the listener to end on its own; interrupt it if it does not.
    Returns (how, output, stderr) where how is its exit status, 'interrupted' or 'hung'."""
    try:
        p.wait(timeout=wait)
        how = p.returncode
    except subprocess.TimeoutExpired:
        p.send_signal(signal.SIGINT)
        try:
            p.wait(timeout=5)
            how = "interrupted"
        except subprocess.TimeoutExpired:
            p.kill()
            p.wait()
            how = "hung"
    out, err = read()
    return how, out, err


def listen_case(cid, proto, client, expect_in, expect_absent=(), extra=(), count=1,
                must_end=True, note="", max_lines=None):
    """Start `listen <proto>` with --count, run client(port), and check the listener's log
    for expect_in (substrings) and that it ended by itself (unless must_end is False)."""
    def run():
        port = free_port()
        p, read = start_listener(["listen", proto, str(port), "--count", str(count)] + list(extra))
        problems = []
        try:
            client(port)
        except Exception as e:
            problems.append("test client failed: %s" % e)
        how, out, err = finish(p, read)
        if "panic" in out + err or "goroutine " in err:
            problems.append("panic in the listener")
        if must_end and how != 0:
            problems.append("listener did not end by itself after --count %d (%s)" % (count, how))
        for s in expect_in:
            if s not in out:
                problems.append("log lacks %r" % s)
        for s in expect_absent:
            if s in out:
                problems.append("log has %r" % s)
        if max_lines is not None and len(out.splitlines()) > max_lines:
            problems.append("the listener logged %d lines, want at most %d" % (len(out.splitlines()), max_lines))
        return problems
    add_func(cid, run, group="J listen", note=note)


def tcp_client(payload, read_back=False, linger_rst=False, wait=0.0, before=0.0):
    def go(port):
        s = socket.create_connection(("127.0.0.1", port))
        s.settimeout(20)
        time.sleep(before)
        if read_back:
            t = threading.Thread(target=lambda: (s.sendall(payload), s.shutdown(socket.SHUT_WR)), daemon=True)
            t.start()
            got = 0
            while True:
                d = s.recv(65536)
                if not d:
                    break
                got += len(d)
            if got != len(payload):
                raise AssertionError("echo returned %d bytes, sent %d" % (got, len(payload)))
        else:
            if payload:
                s.sendall(payload)
            time.sleep(wait)
        if linger_rst:  # close with a reset, after the server has had time to read what was sent
            s.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, b"\x01\x00\x00\x00\x00\x00\x00\x00")
        s.close()
    return go


def raw_http(payload, wait_reply=True):
    def go(port):
        s = socket.create_connection(("127.0.0.1", port))
        s.settimeout(3)
        if callable(payload):
            payload(s)
        else:
            s.sendall(payload)
        if wait_reply:
            try:
                while s.recv(65536):
                    pass
            except OSError:
                pass
        s.close()
    return go


# ---------------- J: listen tcp
listen_case("J.tcp-20MB-inbound-exact-count", "tcp", tcp_client(b"z" * 20_000_000),
            ["bytes_received=20000000", "connections=1"])
listen_case("J.tcp-20MB-inbound-is-a-handful-of-lines", "tcp", tcp_client(b"z" * 20_000_000),
            ["bytes_received=20000000", "reads="], max_lines=80)
listen_case("J.tcp-empty-connection", "tcp", tcp_client(b""), ["connection closed", "bytes_received=0"])
listen_case("J.tcp-client-reset-after-data", "tcp", tcp_client(b"abc", linger_rst=True, wait=0.3), ["bytes_received=3"])
listen_case("J.tcp-binary-payload-preview-escaped", "tcp", tcp_client(bytes(range(256))), ["bytes_received=256", "\\x00"])
listen_case("J.tcp-idle-timeout-closes-connection", "tcp", tcp_client(b"", wait=3), ["connection closed"], extra=["--timeout", "1"])
listen_case("J.tcp-timeout-0-waits-forever", "tcp", tcp_client(b"late", before=2), ["bytes_received=4"], extra=["--timeout", "0"])
listen_case("J.tcp-echo-5MB-round-trip", "tcp", tcp_client(b"e" * 5_000_000, read_back=True),
            ["bytes_received=5000000", "bytes_sent=5000000"], extra=["--echo"])


def many_clients(port):
    ss = [socket.create_connection(("127.0.0.1", port)) for _ in range(150)]
    for i, s in enumerate(ss):
        s.sendall(b"n%d" % i)
    for s in ss:
        s.close()


listen_case("J.tcp-150-concurrent-connections", "tcp", many_clients, ["connections=150", "bytes_received=490"], count=150)


# ---------------- J: listen udp
def udp_client(sizes):
    def go(port):
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        for n in sizes:
            s.sendto(b"u" * n, ("127.0.0.1", port))
            time.sleep(0.05)
    return go


listen_case("J.udp-sizes-0-1-1472-8000", "udp", udp_client([0, 1, 1472, 8000]), ["packets=4", "bytes_received=9473"], count=4)
listen_case("J.udp-echo", "udp", udp_client([1472]), ["packets=1", "bytes_received=1472"], extra=["--echo"])

# ---------------- J: listen http
listen_case("J.http-100KB-header", "http", raw_http(b"GET / HTTP/1.1\r\nHost: x\r\nX-Big: " + b"a" * 100_000 + b"\r\n\r\n"), ["status=200", "bytes_received=100036"])
listen_case("J.http-1MB-url", "http", raw_http(b"GET /" + b"a" * 1_000_000 + b" HTTP/1.1\r\nHost: x\r\n\r\n"), ["status=404", "bytes_received=1000027"])
listen_case("J.http-post-20MB", "http", raw_http(lambda s: (s.sendall(b"POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: 20000000\r\n\r\n"), s.sendall(b"b" * 20_000_000))),
            ["method=POST", "bytes_received=20000056"])
listen_case("J.http-chunked-upload", "http", raw_http(b"POST /c HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n"), ["method=POST", "bytes_received=72"])
listen_case("J.http-unusual-method", "http", raw_http(b"FROB /x HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"), ["method=FROB"])
listen_case("J.http-head", "http", raw_http(b"HEAD /h HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"), ["method=HEAD", "status=404"])
listen_case("J.http-expect-100-continue", "http", raw_http(b"POST /e HTTP/1.1\r\nHost: x\r\nExpect: 100-continue\r\nContent-Length: 4\r\nConnection: close\r\n\r\nabcd"), ["method=POST"])
listen_case("J.http-json-output", "http", raw_http(b"GET /j HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"), ['"path":"/j"', '"status_code":404'], extra=["--json"])
# a request that is not HTTP at all: answered 400 by net/http and, today, neither logged nor counted
listen_case("J.http-malformed-request-is-logged-and-counted", "http", raw_http(b"GARBAGE\r\n\r\n"), ["done requests=1"], note="a malformed request never reaches the handler")
listen_case("J.http-client-stalls-mid-body-is-not-logged-as-ok", "http",
            raw_http(b"POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: 1000\r\n\r\nshort", wait_reply=True), [],
            expect_absent=["bytes_sent=0 time_taken"], extra=["--timeout", "2"],
            note="the client never got a response, yet the log says the request was answered")


# ---------------- K: signals and closed pipes (Unix only)
def signal_case(cid, args, sig, after, expect_rc, expect_text):
    def run():
        p = subprocess.Popen([env.BIN] + fmt([str(free_port()) if a == "{freeport}" else a for a in args]), stdout=subprocess.PIPE, stderr=subprocess.PIPE, stdin=subprocess.DEVNULL, text=True)
        time.sleep(after)
        p.send_signal(sig)
        try:
            out, err = p.communicate(timeout=10)
        except subprocess.TimeoutExpired:
            p.kill()
            p.communicate()
            return ["did not stop within 10s of the signal"]
        problems = []
        if p.returncode != expect_rc:
            problems.append("exit status %r after the signal, want %r" % (p.returncode, expect_rc))
        for s in expect_text:
            if s not in out:
                problems.append("output lacks %r" % s)
        return problems
    add_func(cid, run, group="K signals", needs=("unix",))


signal_case("K.sigterm-ends-with-summary", ["telnet", "127.0.0.1", "{tcp_echo}", "--count", "20", "--delay", "500"], signal.SIGTERM, 1.8, 1, ["interrupted", "done"])
signal_case("K.sigint-ends-with-summary", ["telnet", "127.0.0.1", "{tcp_echo}", "--count", "20", "--delay", "500"], signal.SIGINT, 1.8, 1, ["interrupted", "done"])
signal_case("K.listen-sigterm-ends-with-summary", ["listen", "tcp", "{freeport}"], signal.SIGTERM, 1.0, 0, ["done connections=0"])


def closed_pipe():
    p = subprocess.Popen([env.BIN] + fmt(["telnet", "127.0.0.1", "{tcp_echo}", "--count", "6", "--delay", "300"]),
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE, stdin=subprocess.DEVNULL)
    p.stdout.readline()
    p.stdout.close()  # the reader goes away, like `| head -1`
    try:
        _, err = p.communicate(timeout=10)
    except subprocess.TimeoutExpired:
        p.kill()
        return ["kept running after its output pipe closed"]
    text = err.decode("utf-8", "replace")
    problems = []
    if "panic" in text or "goroutine " in text:
        problems.append("panic when stdout closed: " + text[:120])
    if p.returncode not in (-signal.SIGPIPE, 141, 1):
        problems.append("exit status %r when stdout closed" % p.returncode)
    return problems


add_func("K.closed-stdout-ends-quietly", closed_pipe, group="K signals", needs=("unix",))


# ---------------- L: resource use
def big_body_memory():
    """web must not hold a 300 MB body in memory just to count it."""
    args = fmt(["web", "http://127.0.0.1:{http}/big", "--timeout", "60"])
    p = subprocess.Popen([env.BIN] + args, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL)
    _, _, usage = os.wait4(p.pid, 0)
    rss = usage.ru_maxrss * (1 if sys.platform == "darwin" else 1024)  # bytes on macOS, KB elsewhere
    limit = 150 * 1024 * 1024
    if rss > limit:
        return ["peak memory %d MB downloading a 300 MB body (limit %d MB)" % (rss >> 20, limit >> 20)]
    return None



add_func("L.web-300MB-body-memory", big_body_memory, group="L resources", needs=("unix",))


# ---------------- F (web): a URL without a scheme is https://, and a plain-HTTP server says so
def no_scheme_plain_server():
    port = free_port()
    proc, read = start_listener(["listen", "http", str(port)])
    try:
        rc, out, err, dur = spawn(["web", "127.0.0.1:%d" % port, "--delay", "0", "--timeout", "3"], timeout=20)
    finally:
        proc.send_signal(signal.SIGINT)
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
    problems = []
    if rc != 1:
        problems.append("exit status %r, want 1 (a failed check)" % rc)
    if "use http:// in the URL" not in out:
        problems.append("the failure does not say the server speaks plain HTTP: %s" % out.strip()[-160:])
    return problems


add_func("F.url-no-scheme-plain-http-server-is-explained", no_scheme_plain_server, group="F web")


# ---------------- E (telnet): many attempts at once with few file descriptors
def fd_pressure():
    """--count 1500 --delay 0 under ulimit -n 96 must take turns, not run out of descriptors.
    The target is shint's own listener: the Python servers cannot accept thousands a second."""
    port = free_port()
    proc, _read = start_listener(["listen", "tcp", str(port)])
    try:
        rc, out, err, dur = spawn(["telnet", "127.0.0.1", str(port), "--count", "1500", "--delay", "0", "--timeout", "10"],
                                  timeout=90, wrap="ulimit -n 96;")
    finally:
        proc.send_signal(signal.SIGINT)
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
    problems = []
    ok, failed = out.count("connect ok"), out.count("connect failed")
    if rc != 0 or failed or ok != 1500:
        problems.append("exit %r, %d of 1500 attempts connected, %d failed (%s)" % (rc, ok, failed, out[out.find("connect failed"):][:120].strip() if failed else ""))
    return problems


add_func("E.fd-pressure-many-attempts-at-once", fd_pressure, group="E telnet", needs=("unix",))
