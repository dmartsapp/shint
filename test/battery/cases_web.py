"""The web command: URL forms, methods, headers, bodies, hostile servers, TLS."""
import json

from runner import add

H = "http://127.0.0.1:{http}"
def web(cid, url, *extra, **kw):
    a = ["web", url, "--delay", "0", "--timeout", kw.pop("timeout_flag", "5")] + list(extra)
    add("F." + cid, a, group="F web", max=kw.pop("max", 15), **kw)

# ---------------- F: web - URL forms
for cid, url, rc in [
    ("ok", H + "/ok", 0), ("no-scheme", "127.0.0.1:{http}/ok", None), ("scheme-relative", "//127.0.0.1:{http}/ok", None),
    ("ipv6", "http://[::1]:{http6}/ok", 0), ("userinfo", "http://user:pass@127.0.0.1:{http}/echo", 0),
    ("port-0", "http://127.0.0.1:0/", 1), ("port-99999", "http://127.0.0.1:99999/", None),
    ("scheme-upper", "HTTP://127.0.0.1:{http}/ok", 0), ("ftp", "ftp://127.0.0.1/", 2), ("file", "file:///etc/passwd", 2),
    ("empty-host", "http://", 2), ("space-in-path", H + "/a b", None), ("unicode-path", H + "/%C3%A9", None),
    ("unicode-path-raw", H + "/é", None), ("long-path", H + "/" + "a" * 20000, None),
    ("query-fragment", H + "/ok?x=1&y=2#frag", 0), ("no-path", H, None), ("trailing-dot-host", "http://127.0.0.1.:{http}/ok", None),
    ("localhost", "http://localhost:{http}/ok", None), ("not-a-url", "not a url", 2), ("empty", "", 2),
    ("just-scheme", "http:", 2), ("colon-only", ":", 2), ("percent-bad", H + "/%zz", None),
    ("host-percent", "http://127.0.0.1%25en0:{http}/", None),
]:
    web("url-" + cid, url, rc=rc, note=repr(url[:60]), **({"timeout_flag": "1"} if cid in ("no-scheme", "scheme-relative") else {}))

# a URL without a scheme is fetched over https:// - also when it is host:port (once parsed as the scheme "host")
web("url-no-scheme-to-a-tls-server", "127.0.0.1:{https}/ok", "--cacert", "{good_pem}", rc=0, needs=("tls",))
web("url-no-scheme-host-and-port", "localhost:{https}/ok", "--cacert", "{good_pem}", rc=0, needs=("tls",))
web("url-scheme-typo", "http:127.0.0.1", rc=2, contains=["http:// or https://"])

# ---------------- methods / headers / bodies
for cid, extra in [
    ("get", ["-X", "GET"]), ("post", ["-X", "POST", "-P", "abc"]), ("put", ["-X", "PUT", "-P", "abc"]), ("delete", ["-X", "DELETE"]),
    ("patch", ["-X", "PATCH", "-P", "{}"]), ("head", ["-X", "HEAD"]), ("options", ["-X", "OPTIONS"]), ("trace", ["-X", "TRACE"]),
    ("connect", ["-X", "CONNECT"]), ("lowercase-get", ["-X", "get"]), ("invalid-method-space", ["-X", "NOT VALID"]),
    ("empty-method", ["-X", ""]), ("get-with-body", ["-X", "GET", "-P", "body-on-get"]), ("body-empty", ["-X", "POST", "-P", ""]),
    ("body-unicode", ["-X", "POST", "-P", "héllo wörld ☃"]), ("body-100k", ["-X", "POST", "-P", "x" * 100000]),
]:
    web("method-" + cid, H + "/echo", *extra, note=" ".join(extra)[:50])
for cid, extra in [
    ("simple", ["-H", "X-A: 1"]), ("no-colon", ["-H", "novalue"]), ("empty-name", ["-H", ":x"]), ("empty-value", ["-H", "X-A:"]),
    ("duplicate", ["-H", "X-A: 1", "-H", "X-A: 2"]), ("host-override", ["-H", "Host: virtual.example"]),
    ("ua-override", ["-H", "User-Agent: custom/1"]), ("ua-empty", ["-H", "User-Agent:"]),
    ("content-length-lie", ["-X", "POST", "-P", "abc", "-H", "Content-Length: 99"]),
    ("te-chunked", ["-X", "POST", "-P", "abc", "-H", "Transfer-Encoding: chunked"]),
    ("crlf-injection", ["-H", "X-A: a\r\nInjected: yes"]), ("lf-injection", ["-H", "X-A: a\nInjected: yes"]),
    ("long-value", ["-H", "X-A: " + "v" * 100000]), ("non-ascii", ["-H", "X-A: héllo"]),
    ("space-in-name", ["-H", "X A: 1"]), ("accept-identity", ["-H", "Accept-Encoding: identity"]),
    ("conn-close", ["-H", "Connection: close"]), ("many-headers", sum([["-H", "X-H%d: v" % i] for i in range(200)], [])),
]:
    path = "/host" if cid == "host-override" else "/echo"
    web("header-" + cid, H + path, *extra, note=" ".join(extra)[:50])

# semantic checks on what shint actually sends
def sent(r):
    return r["out"]
web("sem-echo-request", H + "/echo", "-X", "POST", "-P", "abc", "-H", "X-A: 1", "--json", "-W", rc=0,
    check=lambda r: [m for m in ["POST /echo", "X-A: 1", "abc", "dmarts.app-http-v0.1", "Content-Length: 3"] if m not in r["out"]] and ["what was sent lacks: %s" % [m for m in ["POST /echo", "X-A: 1", "abc", "dmarts.app-http-v0.1", "Content-Length: 3"] if m not in r["out"]]] or None)
web("sem-host-header-is-honoured", H + "/host", "-H", "Host: virtual.example", "--json", "-W", rc=0,
    check=lambda r: None if "virtual.example" in r["out"] else "-H 'Host: virtual.example' was ignored: the server saw the URL's host instead")

# ---------------- server behaviours
for cid, path, rc, extra in [
    ("empty-body", "/empty", 0, []), ("trunc-cleanly", "/trunc", 1, []), ("reset-mid-body", "/reset", 1, []),
    ("slow-body", "/slowbody", 1, ["--timeout", "2"]), ("slow-headers", "/slowhdr", 1, ["--timeout", "2"]),
    ("no-response", "/noresp", 1, []), ("redirect-loop", "/loop", 1, []), ("chain-5", "/chain/5", 0, []),
    ("chain-9", "/chain/9", 0, []), ("chain-10", "/chain/10", 1, []), ("chain-11", "/chain/11", 1, []),
    ("relative-redirect", "/rel", None, []), ("307", "/307", 0, ["-X", "POST", "-P", "keepme"]), ("301-post", "/301post", 0, ["-X", "POST", "-P", "dropme"]),
    ("redirect-no-location", "/noloc", None, []), ("chunked", "/chunked", 0, []), ("gzip", "/gzip", 0, []), ("204", "/204", 0, []),
    ("304", "/304", 0, []), ("head-with-length", "/head", 0, ["-X", "HEAD"]), ("giant-header", "/bighdr", 0, []),
    ("status-999", "/weird", 0, []), ("garbage-response", "/garbage", 1, []), ("http-1.0", "/http10", 0, []), ("binary-body", "/binary", 0, []),
    ("404", "/status/404", 0, []), ("500", "/status/500", 0, []), ("401", "/status/401", 0, []), ("delay-1s", "/delay/1", 0, []),
    ("delay-3s-timeout-2", "/delay/3", 1, ["--timeout", "2"]),
]:
    web("srv-" + cid, H + path, *extra, rc=rc, timeout=60, max=20)
    web("srv-" + cid + "-json", H + path, "--json", *extra, timeout=60, max=20,
        **({"check": lambda r: "the JSON reports success for a response that was cut short" if any(s["success"] for s in json.loads(r["out"])["stats"]) else None}
           if cid in ("trunc-cleanly", "reset-mid-body", "slow-body") else {}))
web("srv-json-body-W", H + "/json", "--json", "-W", rc=0)
web("srv-badjson-body-W", H + "/badjson", "--json", "-W", rc=0)
web("srv-bigjson-body-W", H + "/bigjson", "--json", "-W", rc=0, max=30)
web("srv-binary-body-W", H + "/binary", "--json", "-W", rc=0)
web("srv-empty-W", H + "/empty", "--json", "-W", rc=0)
web("srv-gzip-bytes", H + "/gzip", "--json", rc=0)
web("count-100", H + "/ok", "--count", "100", rc=0, serial=True, check=lambda r: None if r["out"].count("OK response") == 100 else "expected 100 response lines, got %d" % r["out"].count("OK response"))
web("count-100-json", H + "/ok", "--count", "100", "--json", rc=0, serial=True, check=lambda r: None if sum(s["success"] for s in json.loads(r["out"])["stats"]) == 100 else "expected 100 successful requests in the JSON")

# ---------------- TLS
HS = "https://127.0.0.1:{https}/ok"
for cid, extra, rc in [
    ("selfsigned-untrusted", [], 1), ("selfsigned-trusted-cacert", ["--cacert", "{good_pem}"], 0), ("selfsigned-insecure", ["-k"], 0),
    ("mismatch-name-cacert", ["--cacert", "{bad_pem}"], 1), ("cacert-missing-file", ["--cacert", "does-not-exist.pem"], 2),
    ("cacert-not-pem", ["--cacert", "{not_pem}"], 2), ("cert-without-key", ["--cert", "{good_pem}"], 2), ("key-without-cert", ["--key", "{good_key}"], 2),
    ("cert-and-key-missing", ["--cert", "nope.pem", "--key", "nope.key"], 2), ("insecure-and-cacert", ["-k", "--cacert", "{good_pem}"], 0),
    ("insecure-json", ["-k", "--json"], 0),
]:
    web("tls-" + cid, HS, *extra, rc=rc)
web("tls-mismatch-server-insecure", "https://127.0.0.1:{https_bad}/ok", "-k", rc=0)
web("tls-mismatch-server-cacert", "https://127.0.0.1:{https_bad}/ok", "--cacert", "{bad_pem}", rc=1)
web("tls-plain-http-to-https-port", "http://127.0.0.1:{https}/ok", rc=None)
web("tls-https-to-plain-port", "https://127.0.0.1:{http}/ok", rc=1)

# ---------------- proxies and environment
web("env-proxy-ignored-or-used", H + "/ok", rc=None, envvars={"HTTP_PROXY": "http://127.0.0.1:1", "http_proxy": "http://127.0.0.1:1"},
    note="does web honour HTTP_PROXY? (a dead proxy makes the request fail if it does)")

