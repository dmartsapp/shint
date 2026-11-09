"""tls: the certificate check, against the battery's HTTPS servers - {https} serves good.pem (self-signed,
for localhost, 127.0.0.1 and ::1, a day long) and {https_bad} bad.pem (for other.example only); see gencert."""
import json

from runner import add


def tl(cid, *a, **kw):
    # the quick defaults first: a case's own --timeout comes after them and wins
    add("O." + cid, ["tls", "--delay", "0", "--timeout", "5"] + list(a), group="O tls", max=kw.pop("max", 15),
        needs=kw.pop("needs", ("tls",)), **kw)


def json_check(want_success, *keys):
    def check(r):
        try:
            s = json.loads(r["out"])["stats"][0]
        except (ValueError, KeyError, IndexError) as e:
            return "not a tls JSON document: %s" % e
        if s.get("success") is not want_success:
            return "success is %r, want %r" % (s.get("success"), want_success)
        missing = [k for k in keys if not s.get(k)]
        return "empty or missing: " + ", ".join(missing) if missing else None
    return check


def listed_once(r):
    out = r["out"]
    if out.count("] OK handshake ") != 3:
        return "expected 3 handshake lines, got %d" % out.count("] OK handshake ")
    if out.count("certificate depth=0") != 1:
        return "the certificate was listed %d times, not once" % out.count("certificate depth=0")
    return None


# ---------------- O: tls - trusted, untrusted, expiry
tl("trusted-cacert", "127.0.0.1", "{https}", "--cacert", "{good_pem}", rc=0,
   contains=["] OK handshake server=127.0.0.1", "] OK certificate depth=0", "] OK verified ", "] WARN expires soon", "] OK done checks=1 passed=1"],
   absent=["ERROR"])
tl("trusted-by-name", "localhost", "{https}", "-4", "--cacert", "{good_pem}", rc=0, contains=["server=localhost"])
tl("untrusted", "127.0.0.1", "{https}", rc=1, contains=["] ERROR verification failed", "signed by an unknown authority", "--cacert"])
tl("warn-days-0", "127.0.0.1", "{https}", "--cacert", "{good_pem}", "--warn-days", "0", rc=0, absent=["WARN", "ERROR"])
tl("warn-days-is-not-a-failure", "127.0.0.1", "{https}", "--cacert", "{good_pem}", "--warn-days", "365", rc=0, contains=["] WARN expires soon"])
tl("fail-days", "127.0.0.1", "{https}", "--cacert", "{good_pem}", "--fail-days", "2", rc=1,
   contains=["] ERROR expires too soon", "within --fail-days 2"])

# ---------------- names
tl("name-mismatch", "127.0.0.1", "{https_bad}", "--cacert", "{bad_pem}", rc=1, contains=["] ERROR verification failed"])
tl("servername-the-certificate-is-for", "127.0.0.1", "{https_bad}", "--cacert", "{bad_pem}", "--servername", "other.example", rc=0,
   contains=["server=other.example address=127.0.0.1"])
tl("servername-wrong", "127.0.0.1", "{https}", "--cacert", "{good_pem}", "--servername", "other.example", rc=1, contains=["not other.example"])

# ---------------- no TLS there
tl("plain-text-port", "127.0.0.1", "{http}", "--timeout", "1", rc=1, contains=["] ERROR handshake failed", "did not answer with TLS within --timeout 1s"])
tl("closed-port", "127.0.0.1", "{tcp_closed}", rc=1, contains=["] ERROR handshake failed"])
tl("silent-port", "127.0.0.1", "{tcp_silent}", "--timeout", "1", rc=1, max=10, contains=["] ERROR handshake failed"])
tl("dns-fail", "no-such-host.invalid", rc=1, contains=["dns resolution failed"], needs=())

# ---------------- output
tl("json", "127.0.0.1", "{https}", "--cacert", "{good_pem}", "--json", rc=0,
   check=json_check(True, "tls_version", "cipher_suite", "verified", "verified_chain", "certificates", "warnings"))
tl("json-untrusted", "127.0.0.1", "{https}", "--json", rc=1, check=json_check(False, "verify_error", "certificates"))
tl("count-3-lists-the-certificate-once", "127.0.0.1", "{https}", "--cacert", "{good_pem}", "--count", "3", rc=0, check=listed_once)

# ---------------- usage
tl("port-0", "127.0.0.1", "0", rc=2, needs=())
tl("port-99999", "127.0.0.1", "99999", rc=2, needs=())
tl("negative-warn-days", "127.0.0.1", "{https}", "--warn-days", "-1", rc=2)
tl("negative-fail-days", "127.0.0.1", "{https}", "--fail-days", "-1", rc=2)
tl("cacert-missing-file", "127.0.0.1", "{https}", "--cacert", "does-not-exist.pem", rc=2)
tl("cacert-not-pem", "127.0.0.1", "{https}", "--cacert", "{not_pem}", rc=2)
tl("no-host", rc=2, needs=())
tl("three-arguments", "127.0.0.1", "443", "extra", rc=2, needs=())
