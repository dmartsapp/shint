"""Command-line handling and the telnet command: usage errors, flag parsing, host and port forms."""
import json
import re
import sys

from runner import add

E = "{tcp_echo}"

# ---------------- A: CLI basics
for g, (cid, args, rc, cont) in enumerate([
    ("version", ["--version"], 0, ["{version}"]), ("version-short", ["-v"], 0, ["{version}"]),
    ("help", ["--help"], 0, ["Usage"]), ("help-short", ["-h"], 0, ["Usage"]),
    ("no-args", [], 0, ["Usage"]), ("unknown-cmd", ["bogus"], 2, []),
    ("help-telnet", ["help", "telnet"], 0, ["telnet"]), ("telnet-help", ["telnet", "--help"], 0, ["Usage"]),
    ("help-listen-tcp", ["listen", "tcp", "--help"], 0, ["Usage"]),
    ("help-listen-http", ["listen", "http", "--help"], 0, ["Usage"]), ("help-listen-udp", ["listen", "udp", "--help"], 0, ["--echo"]),
    ("help-listen-lists-tcp-udp-http", ["listen", "--help"], 0, ["  tcp ", "  udp ", "  http "]),
    ("completion-bash", ["completion", "bash"], 0, []), ("completion-zsh", ["completion", "zsh"], 0, []),
    ("completion-fish", ["completion", "fish"], 0, []), ("completion-powershell", ["completion", "powershell"], 0, []),
    ("completion-none", ["completion"], None, []), ("completion-bogus", ["completion", "bogus"], 2, []),
    ("listen-bogus", ["listen", "bogus"], 2, []),
    ("unknown-flag", ["telnet", "127.0.0.1", E, "--nope"], 2, []),
    ("flag-before-cmd", ["--count", "2", "--delay", "0", "telnet", "127.0.0.1", E], 0, []),
    ("flag-after-args", ["telnet", "127.0.0.1", E, "--delay", "0"], 0, []),
    ("double-dash", ["telnet", "--delay", "0", "--", "127.0.0.1", E], 0, []),
    ("extra-arg", ["telnet", "127.0.0.1", E, "extra"], 2, []),
    ("missing-port", ["telnet", "127.0.0.1"], 2, []), ("missing-all", ["telnet"], 2, []),
    ("web-no-arg", ["web"], 2, []), ("nmap-no-arg", ["nmap"], 2, []), ("udp-one-arg", ["udp", "127.0.0.1"], 2, []),
    ("single-dash-long", ["telnet", "127.0.0.1", E, "-count", "2"], 2, []),
    ("case-sensitive-flag", ["telnet", "127.0.0.1", E, "--Count", "2"], 2, []),
]):
    add("A." + cid, args, rc=rc, contains=cont, group="A cli", max=15)

# ---------------- B: numeric / boolean flag parsing (telnet against an open port)
def tel(*extra): return ["telnet", "127.0.0.1", E, "--delay", "0"] + list(extra)
for cid, extra, rc in [
    ("count-0", ["--count", "0"], 2), ("count-neg", ["--count", "-1"], 2), ("count-1", ["--count", "1"], 0),
    ("count-3", ["--count", "3"], 0), ("count-abc", ["--count", "abc"], 2), ("count-float", ["--count", "1.5"], 2),
    ("count-empty", ["--count", ""], 2), ("count-huge", ["--count", "99999999999999999999"], 2),
    ("count-equals", ["--count=2"], 0), ("count-repeated", ["--count", "2", "--count", "3"], 0),
    ("timeout-0", ["--timeout", "0"], 2), ("timeout-neg", ["--timeout", "-1"], 2), ("timeout-1", ["--timeout", "1"], 0),
    ("timeout-abc", ["--timeout", "abc"], 2), ("timeout-int64max", ["--timeout", "9223372036854775807"], 2),
    ("timeout-2^31", ["--timeout", "2147483648"], 2), ("timeout-3600", ["--timeout", "3600"], 0),
    ("timeout-a-day", ["--timeout", "86400"], 0), ("timeout-a-day-and-a-second", ["--timeout", "86401"], 2),
    ("timeout-10^10", ["--timeout", "10000000000"], 2),
    ("delay-neg", ["--delay", "-1"], 2), ("delay-abc", ["--delay", "abc"], 2), ("delay-float", ["--delay", "1.5"], 2),
    ("delay-int64max-count1", ["--delay", "9223372036854775807"], 2),
    ("delay-a-day-and-a-millisecond", ["--delay", "86400001"], 2),
    ("json-false", ["--json=false"], 0), ("json-true", ["--json=true"], 0), ("json-maybe", ["--json=maybe"], 2),
    ("payload-neg-ignored", ["--payload", "-1"], None), ("payload-huge-ignored", ["--payload", "99999999999"], None),
    ("throttle-count1", ["--throttle"], 0),
]:
    a = ["telnet", "127.0.0.1", E] + ([] if cid.startswith("delay") else ["--delay", "0"]) + extra
    add("B." + cid, a, rc=rc, group="B flags", max=20, timeout=30)

# ---------------- C: host forms
HOSTS = ["", " ", "localhost", "127.0.0.1", "127.1", "0", "0.0.0.0", "::", "[::1]", "::ffff:127.0.0.1", "2130706433",
         "0x7f000001", "0177.0.0.1", "localhost.", "LOCALHOST", "a" * 300, ("a" * 64) + ".com", "bücher.de",
         "xn--bcher-kva.de", "host_name", "fe80::1%lo0", "1.2.3", "1.2.3.4.5", "256.1.1.1", "999999999999",
         "http://localhost", "localhost:80", "user@localhost", "nonexistent.invalid", ".", "..", "a..b"]
for i, h in enumerate(HOSTS):
    add("C.host-%02d" % i, ["telnet", h, E, "--delay", "0", "--timeout", "3"], group="C hosts", max=15, note="host=%r" % h[:40])
add("C.host-dash-flaglike", ["telnet", "-bad", E], rc=2, group="C hosts")
add("C.host-dashdash-flaglike", ["telnet", "--bad", E], rc=2, group="C hosts")
add("C.host-ipv6-loopback", ["telnet", "::1", "{tcp_echo6}", "--delay", "0"], rc=0, group="C hosts")
add("C.host-ipv6-json", ["telnet", "::1", "{tcp_echo6}", "--delay", "0", "--json"], rc=0, group="C hosts")

# ---------------- D: ports
for i, pt in enumerate(["0", "1", "65535", "65536", "-1", "+80", "080", " 80", "80.0", "0x50", "http", "", "1e3", "99999999999999999999", "٨٠"]):
    add("D.port-%02d" % i, ["telnet", "127.0.0.1", pt, "--delay", "0", "--timeout", "2"], group="D ports", max=10, note="port=%r" % pt)

# ---------------- E: telnet behaviours
add("E.refused", ["telnet", "127.0.0.1", "{tcp_closed}", "--delay", "0"], rc=1, group="E telnet")
add("E.refused-json", ["telnet", "127.0.0.1", "{tcp_closed}", "--delay", "0", "--json"], rc=1, group="E telnet")
add("E.silent-server-connects", ["telnet", "127.0.0.1", "{tcp_silent}", "--delay", "0"], rc=0, group="E telnet")
add("E.closing-server-connects", ["telnet", "127.0.0.1", "{tcp_close}", "--delay", "0"], rc=0, group="E telnet")
add("E.blackhole-timeout", ["telnet", "192.0.2.1", "80", "--delay", "0", "--timeout", "2"], rc=1, group="E telnet", max=8)
add("E.blackhole-json", ["telnet", "192.0.2.1", "80", "--delay", "0", "--timeout", "2", "--json"], rc=1, group="E telnet", max=8)
add("E.dns-fail", ["telnet", "no-such-host.invalid", "80", "--delay", "0"], rc=1, group="E telnet")
add("E.dns-fail-json", ["telnet", "no-such-host.invalid", "80", "--delay", "0", "--json"], rc=1, group="E telnet")
add("E.count-100-fast", ["telnet", "127.0.0.1", E, "--count", "100", "--delay", "0"], rc=0, group="E telnet", serial=True,
    check=lambda r: None if r["out"].count("connect ok") == 100 else "expected 100 'connect ok' lines, got %d" % r["out"].count("connect ok"))
add("E.count-100-json", ["telnet", "127.0.0.1", E, "--count", "100", "--delay", "0", "--json"], rc=0, group="E telnet", serial=True,
    check=lambda r: None if sum(s["success"] for s in json.loads(r["out"])["stats"]) == 100 else "expected 100 successful attempts in the JSON")

# listen takes --count too, and -1 is not a valid value for it (0 means "until Ctrl+C")
add("B.listen-count-negative", ["listen", "tcp", "{tcp_closed}", "--count", "-1"], rc=2, group="B flags", max=8, timeout=4)
add("B.listen-http-count-negative", ["listen", "http", "{tcp_closed}", "--count", "-1"], rc=2, group="B flags", max=8, timeout=4)
add("B.listen-udp-count-negative", ["listen", "udp", "{tcp_closed}", "--count", "-1"], rc=2, group="B flags", max=8, timeout=4)

# the dns authoritative line is for host names that have a zone: never an address or a single label
add("E.no-authoritative-line-for-an-address", ["telnet", "127.0.0.1", E, "--delay", "0"], rc=0, absent=["dns authoritative"], group="E telnet")
add("E.no-authoritative-line-for-localhost", ["telnet", "localhost", E, "--delay", "0"], rc=None, absent=["dns authoritative"], group="E telnet")
add("E.no-authoritative-in-json-for-an-address", ["telnet", "127.0.0.1", E, "--delay", "0", "--json"], rc=0, group="E telnet",
    check=lambda r: "an IP address has no zone, but dns_lookup has an authoritative field" if "authoritative" in json.loads(r["out"])["dns_lookup"] else None)

# ---------------- M: ip (reads the interface table; needs nothing from the network)
add("M.ip", ["ip"], rc=0, contains=["[ip] OK interface name=", "[ip] OK done interfaces="], group="M ip", max=10)
add("M.ip-json", ["ip", "--json"], rc=0, group="M ip", max=10,
    check=lambda r: None if json.loads(r["out"])["stats"] else "the JSON lists no interfaces")
add("M.ip-one-line-per-interface", ["ip"], rc=0, group="M ip", max=10, absent=["[ip] OK address ", "kind=", "index="],
    check=lambda r: None if all(l.split("] ", 1)[1].startswith(("OK interface name=", "OK done interfaces=")) for l in r["out"].splitlines() if "[ip] " in l) else "a line that is neither an interface nor the summary")
add("M.ip-json-is-flat", ["ip", "--json"], rc=0, group="M ip", max=10,
    check=lambda r: None if all(set(s) <= {"name", "state", "ipv4", "ipv6", "mac", "mtu", "flags", "error"} and all(isinstance(a, str) for a in s["ipv4"] + s["ipv6"]) for s in json.loads(r["out"])["stats"]) else "an interface entry has a key or a nested value that it should not")
add("M.ip-4-lists-no-ipv6", ["ip", "-4"], rc=0, absent=["ipv6="], group="M ip", max=10)
add("M.ip-6-lists-no-ipv4", ["ip", "-6"], rc=0, absent=["ipv4="], group="M ip", max=10)
add("M.ip-unknown-interface", ["ip", "no-such-interface0"], rc=1, contains=["no such interface no-such-interface0"], group="M ip", max=10)
add("M.ip-unknown-interface-json", ["ip", "no-such-interface0", "--json"], rc=1, group="M ip", max=10,
    check=lambda r: None if "no such interface" in json.loads(r["out"])["error"] else "the JSON does not say why")
add("M.ip-both-families", ["ip", "-4", "-6"], rc=2, group="M ip", max=10)
add("M.ip-two-arguments", ["ip", "a", "b"], rc=2, group="M ip", max=10)
add("M.ip-ignores-shared-flags", ["ip", "--count", "5", "--timeout", "1", "--delay", "3000"], rc=0, group="M ip", max=6)

# ---------------- P: ss (reads this machine's socket tables; Linux and Windows only)
SS_KEYS = {"protocol", "family", "state", "local_address", "local_port", "remote_address", "remote_port",
           "recv_q", "send_q", "uid", "user", "processes", "owner_hidden"}


def ss(cid, args, **kw):
    add("P." + cid, ["ss"] + list(args), group="P ss", max=kw.pop("max", 10), **kw)


for cid, args in [("argument", ["extra"]), ("both-families", ["-4", "-6"]), ("pid-zero", ["--pid", "0"]),
                  ("pid-negative", ["--pid", "-5"]), ("pid-not-a-number", ["--pid", "abc"]), ("pid-huge", ["--pid", "99999999999999999999"]),
                  ("ports-zero", ["--ports", "0"]), ("ports-empty", ["--ports", ""]), ("ports-bad-range", ["--ports", "1-x"]),
                  ("ports-backwards", ["--ports", "80-22"]), ("ports-injection", ["--ports", "22;id"]), ("ports-too-big", ["--ports", "65536"])]:
    ss(cid, args, rc=2)
ss("help", ["--help"], rc=0, contains=["--listening", "--ports", "--pid", "--tcp", "--udp"])

if sys.platform.startswith("linux"):
    ss("lists", [], rc=0, contains=["[ss] OK socket proto=", "[ss] OK done sockets="])
    ss("finds-the-battery-listener", ["-l", "-t", "--ports", "{tcp_echo}"], rc=0,
       contains=["state=LISTEN local=127.0.0.1:{tcp_echo} ", " pid="], absent=["owner=hidden", "proto=udp"])
    ss("finds-the-battery-udp-server", ["-l", "-u", "--ports", "{udp_echo}"], rc=0, contains=["proto=udp state=UNCONN local=127.0.0.1:{udp_echo} ", " pid="])
    ss("nothing-on-a-closed-port", ["-l", "--ports", "{tcp_closed}"], rc=1, contains=["no socket matches: listening, port {tcp_closed}"])
    ss("json", ["--json"], rc=0,
       check=lambda r: None if all(set(s) <= SS_KEYS for s in json.loads(r["out"])["stats"]) and json.loads(r["out"])["stats"] else "no sockets, or a socket entry with a key it should not have")
    ss("json-listener-has-its-process", ["-l", "-t", "--ports", "{tcp_echo}", "--json"], rc=0,
       check=lambda r: None if [s for s in json.loads(r["out"])["stats"] if s["local_port"] == int(r["args"][-2]) and s.get("processes") and "fd" in s["processes"][0]] else "the battery's listener is not listed with its process and descriptor")
    ss("ipv4-only", ["-4"], rc=0, absent=["local=["])
    ss("ipv6-only", ["-6"], absent=["local=0.", "local=1", "local=2", "local=3", "local=4", "local=5", "local=6", "local=7", "local=8", "local=9"])
    ss("pid-that-holds-nothing", ["--pid", "999999999"], rc=1, contains=["no socket matches: held by process 999999999"])
    ss("ignores-shared-flags", ["-l", "--count", "5", "--timeout", "1", "--delay", "3000"], rc=0)
else:
    ss("not-supported", [], rc=1, contains=["[ss] ERROR failed error=\"ss is not supported on "])
    ss("not-supported-json", ["--json"], rc=1,
       check=lambda r: None if "is not supported on" in json.loads(r["out"])["error"] and json.loads(r["out"])["stats"] == [] else "the JSON does not say why, or lists sockets")
