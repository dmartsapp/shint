"""nmap, udp and ping. Ping cases run one at a time: ICMP replies reach every ping on the machine."""
import json
import subprocess

import env
from env import PORTS
from runner import add, add_func, spawn

# ---------------- G: nmap
def nm(cid, *a, **kw):
    add("G." + cid, ["nmap", "127.0.0.1"] + list(a), group="G nmap", max=kw.pop("max", 20), timeout=kw.pop("timeout", 60), **kw)
lo, hi = min(PORTS["http"], PORTS["tcp_closed"]), max(PORTS["http"], PORTS["tcp_closed"])
for cid, a, rc in [
    ("single-port-open", ["--from", "{tcp_echo}", "--to", "{tcp_echo}"], 0), ("single-closed", ["--from", "{tcp_closed}", "--to", "{tcp_closed}"], 0),
    ("port-1", ["--from", "1", "--to", "1"], 0), ("port-65535", ["--from", "65535", "--to", "65535"], 0),
    ("from-0", ["--from", "0", "--to", "5"], 2), ("to-65536", ["--from", "65530", "--to", "65536"], 2),
    ("from-gt-to", ["--from", "10", "--to", "5"], 2), ("from-abc", ["--from", "abc"], 2), ("from-neg", ["--from=-1"], 2),
    ("to-abc", ["--to", "abc"], 2), ("json-range", ["--from", "{tcp_echo}", "--to", "{tcp_echo}", "--json"], 0),
    ("count-2", ["--from", "{tcp_echo}", "--to", "{tcp_echo}", "--count", "2"], 0), ("timeout-1", ["--from", "{tcp_echo}", "--to", "{tcp_echo}", "--timeout", "1"], 0),
    ("delay-0", ["--from", "{tcp_echo}", "--to", "{tcp_echo}", "--delay", "0"], 0),
]:
    nm(cid, *a, rc=rc)
_open_v4 = [PORTS[k] for k in ("http", "https", "https_bad", "tcp_echo", "tcp_close", "tcp_silent") if k in PORTS]


def found_all_open(r):
    found = {s["port"] for s in json.loads(r["out"])["stats"] if s["success"]}
    missed = sorted(p for p in _open_v4 if lo <= p <= hi and p not in found)
    return "open ports missed: %s" % missed if missed else None


add("G.finds-every-open-port", ["nmap", "127.0.0.1", "--from", str(lo), "--to", str(hi), "--json"], group="G nmap", max=30, rc=0, check=found_all_open)
add("G.full-range-localhost-json", ["nmap", "127.0.0.1", "--from", "1", "--to", "65535", "--json", "--timeout", "1"], group="G nmap", rc=0, max=120, timeout=150,
    check=lambda r: None if len(json.loads(r["out"])["stats"]) == 65535 else "JSON lists %d ports, want 65535" % len(json.loads(r["out"])["stats"]))
add("G.ipv6", ["nmap", "::1", "--from", "{tcp_echo6}", "--to", "{tcp_echo6}"], group="G nmap", rc=0, contains=["port open"])
add("G.dns-fail", ["nmap", "no-such-host.invalid"], group="G nmap", rc=1)
add("G.filtered-target", ["nmap", "192.0.2.1", "--from", "80", "--to", "82", "--timeout", "1"], group="G nmap", rc=0, max=10)
add("G.hostname-multi", ["nmap", "localhost", "--from", "{tcp_echo}", "--to", "{tcp_echo}", "--json"], group="G nmap", rc=None)

# ---------------- H: udp
def ud(cid, *a, **kw):
    port = kw.pop("port", "{udp_echo}")
    add("H." + cid, ["udp", kw.pop("host", "127.0.0.1"), port] + list(a), group="H udp", max=kw.pop("max", 15), timeout=kw.pop("timeout", 40), **kw)
ud("data", "--data", "hello", rc=0, contains=["probe open"]); ud("data-json", "--data", "hello", "--json", rc=0)
ud("data-empty-uses-filler", "--data", "", rc=0); ud("data-unicode", "--data", "héllo ☃", rc=0); ud("data-multiline", "--data", "a\nb\nc", rc=0)
ud("payload-0", "--payload", "0"); ud("payload-1", "--payload", "1", rc=0); ud("payload-1400", "--payload", "1400", rc=0)
ud("payload-8000", "--payload", "8000"); ud("payload-65507", "--payload", "65507"); ud("payload-65508", "--payload", "65508", rc=2)
ud("payload-neg", "--payload", "-1", rc=2); ud("payload-huge-1e9", "--payload", "1000000000", rc=2)
ud("payload-int64max", "--payload", "9223372036854775807", rc=2); ud("payload-abc", "--payload", "abc", rc=2)
ud("closed-port", port="{udp_closed}", rc=1); ud("silent-port", port="{udp_silent}", rc=0, contains=["open|filtered"], max=10, **{}) if False else None
add("H.silent-port", ["udp", "127.0.0.1", "{udp_silent}", "--timeout", "2", "--delay", "0"], group="H udp", rc=0, contains=["open|filtered"], max=10)
add("H.silent-port-json", ["udp", "127.0.0.1", "{udp_silent}", "--timeout", "2", "--delay", "0", "--json"], group="H udp", rc=0, max=10)
add("H.big-reply", ["udp", "127.0.0.1", "{udp_big}", "--delay", "0", "--data", "x"], group="H udp", rc=0)
add("H.ipv6-echo", ["udp", "::1", "{udp_echo6}", "--delay", "0", "--data", "x"], group="H udp", rc=0)
add("H.count-50", ["udp", "127.0.0.1", "{udp_echo}", "--delay", "0", "--count", "50", "--data", "x"], group="H udp", rc=0, serial=True,
    check=lambda r: None if r["out"].count("probe open") == 50 else "expected 50 open probes, got %d" % r["out"].count("probe open"))
add("H.dns-fail", ["udp", "no-such-host.invalid", "53", "--delay", "0"], group="H udp", rc=1)
add("H.port-0", ["udp", "127.0.0.1", "0"], group="H udp", rc=2)
add("H.port-65536", ["udp", "127.0.0.1", "65536"], group="H udp", rc=2)

# ---------------- I: ping
def pg(cid, *a, **kw):
    host = kw.pop("host", "127.0.0.1")
    add("I." + cid, ["ping", host] + list(a), group="I ping", serial=True, needs=("icmp",), max=kw.pop("max", 20), timeout=kw.pop("timeout", 45), **kw)
pg("basic", "--count", "2", "--delay", "0", rc=0); pg("json", "--count", "2", "--delay", "0", "--json", rc=0)
for cid, p, rc in [("payload-0", "0", 0), ("payload-1448", "1448", 0), ("payload-1449", "1449", 2), ("payload-neg", "-1", 2), ("payload-abc", "abc", 2), ("payload-huge", "99999999999", 2)]:
    pg(cid, "--count", "1", "--delay", "0", "--payload", p, rc=rc)
pg("timeout-0", "--timeout", "0", rc=2); pg("count-0", "--count", "0", rc=2)
pg("ipv6", "--count", "1", "--delay", "0", host="::1", rc=0); pg("localhost-dual", "--count", "1", "--delay", "0", host="localhost", rc=None)
pg("unreachable-1s", "--count", "1", "--delay", "0", "--timeout", "1", host="192.0.2.1", rc=1, max=12)
pg("unreachable-json", "--count", "1", "--delay", "0", "--timeout", "1", "--json", host="192.0.2.1", rc=1, max=12)
pg("dns-fail", "--count", "1", host="no-such-host.invalid", rc=1); pg("dns-fail-json", "--count", "1", "--json", host="no-such-host.invalid", rc=1)
pg("multicast", "--count", "1", "--delay", "0", "--timeout", "1", host="224.0.0.1", rc=None, max=12)
pg("broadcast", "--count", "1", "--delay", "0", "--timeout", "1", host="255.255.255.255", rc=None, max=12)
pg("unspecified", "--count", "1", "--delay", "0", "--timeout", "1", host="0.0.0.0", rc=None, max=12)
pg("empty-host", "--count", "1", host="", rc=None); pg("count-30-fast", "--count", "30", "--delay", "0", "--timeout", "1", rc=0)
pg("throttle-off-delay-0", "--count", "3", "--delay", "0", "--timeout", "1", rc=0)


# A reply that belongs to some other ping must never make an unreachable address look reachable.
# The neighbour pings loopback (always answered) while the victim pings TEST-NET-1 (never answered).
def ping_crosstalk():
    """Run the scenario up to six times: on an affected build it fools the victim about nine
    times in ten, so a build that survives all six is not affected (and one that is fooled once is)."""
    for attempt in range(6):
        neighbour = subprocess.Popen([env.BIN, "ping", "127.0.0.1", "--count", "3", "--delay", "600", "--timeout", "2"],
                                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL)
        rc, out, err, dur = spawn(["ping", "192.0.2.1", "--count", "1", "--delay", "0", "--timeout", "2"], timeout=30)
        neighbour.wait()
        if rc == 0:
            return ["ping to an unreachable address exited 0 (run %d of 6): it took another ping's reply as its own: %s"
                    % (attempt + 1, out.strip().splitlines()[-3][-90:])]
    return None


add_func("I.no-false-success-from-another-ping", ping_crosstalk, group="I ping", needs=("icmp",))
