"""The dns command against a loopback DNS server that answers, refuses, lies, truncates and goes silent."""
import json

from runner import add

S = "@127.0.0.1:{dns}"
FAST = ["--delay", "0", "--timeout", "2"]


def dns(cid, args, fast=True, **kw):
    kw.setdefault("needs", ("dns",))
    add("N." + cid, ["dns"] + args + (FAST if fast else []), group="N dns", max=kw.pop("max", 15), **kw)


def stats(r):
    return json.loads(r["out"])["stats"]


# ---- answers
dns("a", ["ok.example", "A", S], rc=0, contains=["192.0.2.1", "type=A ttl=300", "rcode=noerror"])
dns("both-families-by-default", ["ok.example", S], rc=0, contains=["type=A ", "type=AAAA ", "2001:db8::1", "queries=2 answered=2"])
dns("mx", ["ok.example", "MX", S], rc=0, contains=['data="10 mail.ok.example."'])
dns("ns-soa-caa", ["ok.example", "NS", S], rc=0, contains=["data=ns.example."])
dns("soa", ["ok.example", "SOA", S], rc=0, contains=["ns.example. hostmaster.example. 1 7200 3600 1209600 300"])
dns("caa", ["ok.example", "CAA", S], rc=0, contains=['0 issue \\"letsencrypt.org\\"'])
dns("txt", ["ok.example", "TXT", S], rc=0, contains=['data="v=spf1 -all"'])
dns("lowercase-type", ["ok.example", "mx", S], rc=0)
dns("server-first", [S, "ok.example", "A"], rc=0, contains=["192.0.2.1"])
dns("reverse-of-an-address", ["192.0.2.1", S], rc=0, contains=["1.2.0.192.in-addr.arpa.", "type=PTR", "data=host.example."])
dns("reverse-ipv6", ["2001:db8::1", S], rc=0, contains=["ip6.arpa.", "type=PTR"])
dns("json", ["ok.example", "A", S, "--json"], rc=0,
    check=lambda r: None if stats(r)[0]["success"] and stats(r)[0]["answers"][0]["data"] == "192.0.2.1" and "dns_lookup" not in json.loads(r["out"]) else "unexpected JSON shape")
dns("only-ipv4-with-minus-4", ["ok.example", S, "-4"], rc=0, contains=["type=A "], absent=["type=AAAA"])
dns("count", ["ok.example", "A", S, "--count", "3"], rc=0, contains=["attempt=1/3", "attempt=3/3", "queries=3 answered=3"])
dns("over-tcp", ["ok.example", "A", S, "--tcp"], rc=0, contains=["transport=tcp"], absent=["retried="])
dns("no-recurse", ["ok.example", "A", S, "--no-recurse"], rc=0)
dns("truncated-over-udp-is-retried-over-tcp", ["big.example", "A", S], rc=0, contains=["transport=tcp", "retried=tcp", "192.0.2.7"])
dns("hostile-txt-is-escaped", ["txt.example", "TXT", S], rc=0, contains=["\\x1b[31mred", "line1\\nline2\\x00", "\\xff\\xfe"],
    check=lambda r: "a TXT record reached the terminal unescaped" if any(c in r["out"] for c in "\x1b\x00") else None)

# ---- answers that are not answers: each a failed check with its own reason
dns("nxdomain", ["nx.example", "A", S], rc=1, contains=["ERROR query failed", "rcode=nxdomain", "no such domain (NXDOMAIN)", "type=SOA"])
dns("nxdomain-json", ["nx.example", "A", S, "--json"], rc=1,
    check=lambda r: None if stats(r)[0]["rcode"] == "NXDOMAIN" and not stats(r)[0]["success"] and stats(r)[0]["authority"] else "the JSON does not describe the NXDOMAIN")
dns("nodata", ["nodata.example", "A", S], rc=1, contains=["no A records", "exists but has none of this type"])
dns("servfail", ["servfail.example", "A", S], rc=1, contains=["SERVFAIL"])
dns("refused", ["refused.example", "A", S], rc=1, contains=["REFUSED"])
dns("garbage-reply", ["garbage.example", "A", S, "--tcp"], rc=1, contains=["malformed reply"])
dns("garbage-over-udp-times-out", ["garbage.example", "A", S, "--timeout", "1"], rc=1, contains=["ignored 1 reply(ies)"], max=10)
dns("wrong-id-is-not-an-answer", ["wrongid.example", "A", S, "--timeout", "1"], rc=1, absent=["203.0.113.66"], contains=["ignored"], max=10)
dns("silent-server-times-out", ["silent.example", "A", S, "--timeout", "1"], rc=1, contains=["no DNS server answered", "i/o timeout"], max=10)
dns("nothing-listening", ["ok.example", "A", "@127.0.0.1:{udp_closed}", "--timeout", "1"], rc=1, contains=["connection refused"], max=10)
dns("server-name-that-does-not-resolve", ["ok.example", "A", "@no-such-host.invalid"], rc=1, contains=["dns resolution failed"], needs=())

# ---- using it wrongly: exit 2, nothing on stdout
dns("no-arguments", [], rc=2, needs=())
dns("unsupported-type", ["ok.example", "BOGUS", S], rc=2, contains=["unsupported record type"])
dns("two-servers", ["ok.example", S, S], rc=2, contains=["only one @server"])
dns("address-with-a-type", ["192.0.2.1", "MX", S], rc=2, contains=["PTR"])
dns("both-families", ["ok.example", S, "-4", "-6"], rc=2)
dns("label-too-long", ["a" * 64 + ".example", S], rc=2, contains=["longer than 63"])
dns("unicode-name", ["bücher.de", S], rc=2, contains=["xn--"])
dns("space-in-name", ["a b.example", S], rc=2)
dns("empty-label", ["a..example", S], rc=2)
dns("server-port-0", ["ok.example", "@127.0.0.1:0"], rc=2, contains=["invalid server"], needs=())
dns("server-port-70000", ["ok.example", "@127.0.0.1:70000"], rc=2, needs=())
dns("count-0", ["ok.example", S, "--count", "0"], rc=2)
dns("timeout-0", ["ok.example", S, "--timeout", "0", "--delay", "0"], rc=2, fast=False)
