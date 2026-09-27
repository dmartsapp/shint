#!/usr/bin/env python3
"""Runtime stats and profiles of a shint build, kept per release as history in .profiling/.

    make profile                       .profiling/dev/<git describe>/<time>/  (yours; git ignores it)
    make profile TAG=v4.3.0            .profiling/v4.3.0/<time>/  (make release runs this, and commits it)
    make profile-compare A=v4.2.3 B=v4.3.0
    python3 test/profile/profile.py --backfill v4.0.0 v4.2.3 ...   released binaries, measured here

A run builds this tree's binary (release flags, this machine's platform) and records, in
<run>/summary.json:

  * the machine it ran on - OS, CPU, cores, memory, model, power source, load, and an id made
    from the hardware (a hash, so the public history does not name the machine). Timings are
    only comparable between runs on the same machine; --compare says when they are not.
  * scenarios: shint run as a user runs it, against the battery's loopback servers, each
    several times after one unmeasured warm-up - wall time, CPU time (user + system) and peak
    memory of the process, from the operating system's accounting of that one process
    (wait4). The same scenarios run against any version's binary, which is what --backfill does.
  * the size of the binary for every release platform (built here with the release flags)
  * the Go tests' time per package and the slowest tests (go test -json, no race detector),
    and the black-box battery's time per group and its slowest cases
  * CPU and memory profiles (pprof) of the lib and lib/handlers tests, and - when the tree
    has the `profiling` build tag's hook - of the binary itself in each scenario, each with a
    plain-text "top" summary next to it, readable without any tools

shint measures what shint did. What it records is the time and resources of its own runs,
not the operating system's performance counters, which count differently.

--quick records only the machine, the scenarios and the host binary's size. After a run the
script waits (up to 90 s) for the machine's TIME_WAIT sockets to drain, so a check started
straight after it does not run out of local ports (issue #78). Before measuring, it waits (up to
90 s) for the 1-minute load to fall to 30% of the cores, so every run starts from a comparably
quiet machine; the load it started at is recorded, and a run that started busy says so.

Standard library only, like the battery.
"""
import argparse
import datetime
import glob
import hashlib
import json
import os
import platform
import re
import shutil
import socket
import statistics
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request

sys.dont_write_bytecode = True
HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
BATTERY = os.path.join(REPO, "test", "battery")
OUT = os.path.join(REPO, ".profiling")
SCHEMA = 1
REPO_SLUG = os.environ.get("GH_REPO", "dmartsapp/shint")

# name, arguments ({placeholders} are the battery servers' ports), repetitions, what it shows.
SCENARIOS = [
    ("startup", ["--version"], 20, "process start to exit: the floor under every command"),
    ("help", ["--help"], 10, "cobra building and printing the command tree"),
    ("telnet", ["telnet", "127.0.0.1", "{tcp_echo}", "--count", "100", "--delay", "0"], 5, "100 TCP connects, in parallel"),
    ("telnet-json", ["telnet", "127.0.0.1", "{tcp_echo}", "--count", "100", "--delay", "0", "--json"], 5, "the same, with the JSON document"),
    ("web", ["web", "http://127.0.0.1:{http}/ok", "--count", "50", "--delay", "0"], 5, "50 small HTTP requests"),
    ("web-300mb", ["web", "http://127.0.0.1:{http}/big", "--count", "1", "--delay", "0", "--timeout", "60"], 3, "one 300 MB download: memory must not grow with the body"),
    ("udp", ["udp", "127.0.0.1", "{udp_echo}", "--count", "100", "--delay", "0"], 5, "100 UDP probes to an echo server"),
    ("nmap", ["nmap", "127.0.0.1", "--from", "20000", "--to", "21999", "--timeout", "1"], 3, "2000 closed loopback ports"),
    ("ping", ["ping", "127.0.0.1", "--count", "10", "--delay", "0", "--timeout", "2"], 3, "10 ICMP echoes (needs unprivileged ICMP)"),
    ("dns", ["dns", "ok.example", "A", "@127.0.0.1:{dns}", "--count", "20", "--delay", "0", "--timeout", "2"], 5, "20 DNS queries to a local server"),
    ("cidr", ["cidr", "10.0.0.0/8", "2001:db8::/32"], 10, "pure computation"),
    ("ip", ["ip"], 10, "reading the interface table"),
    ("listen-http", None, 3, "shint as the server: 300 requests from 8 clients"),
]
LISTEN_REQUESTS = 300
LISTEN_CLIENTS = 8
PROFILED_PACKAGES = ["./lib/handlers", "./lib"]


# --- small helpers ----------------------------------------------------------------------

def out_of(*args, cwd=REPO, env=None, timeout=600):
    """A command's stdout, or "" if it failed or is missing."""
    try:
        r = subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=timeout,
                           env=dict(os.environ, **(env or {})))
    except (OSError, subprocess.TimeoutExpired):
        return ""
    return r.stdout.strip() if r.returncode == 0 else ""


def read(path, default=""):
    try:
        with open(path) as f:
            return f.read()
    except OSError:
        return default


def say(text):
    print(text, flush=True)


def stats(values):
    """median, mean, min, max and standard deviation - rounded, for the summary."""
    values = [v for v in values if v is not None]
    if not values:
        return None
    return {
        "median": round(statistics.median(values), 3),
        "mean": round(statistics.mean(values), 3),
        "min": round(min(values), 3),
        "max": round(max(values), 3),
        "stdev": round(statistics.stdev(values), 3) if len(values) > 1 else 0.0,
        "n": len(values),
    }


def utc_stamp(now=None):
    """The run folder's name: a UTC time without colons, which some file systems refuse."""
    return (now or datetime.datetime.now(datetime.timezone.utc)).strftime("%Y-%m-%dT%H%M%SZ")


# --- the machine --------------------------------------------------------------------------

def sysctl(name):
    return out_of("sysctl", "-n", name, cwd=None)


def machine():
    """Everything about this machine that can change a timing, and an id that stays the same.

    The id is a hash of the hardware's own identifier (IOPlatformUUID on macOS,
    /etc/machine-id on Linux), so runs can be grouped by machine in a public history without
    publishing the identifier or the host name. SHINT_PROFILE_MACHINE adds a readable label."""
    system = platform.system()
    m = {
        "label": os.environ.get("SHINT_PROFILE_MACHINE", ""),
        "os": system,
        "os_version": platform.release(),
        "kernel": platform.version(),
        "arch": platform.machine(),
        "model": "",
        "cpu": {"model": platform.processor(), "logical_cores": os.cpu_count()},
        "memory_bytes": None,
        "power": "unknown",
        "load_1m_start": round(os.getloadavg()[0], 2) if hasattr(os, "getloadavg") else None,
        "go": out_of("go", "env", "GOVERSION"),
        "python": platform.python_version(),
    }
    raw_id = ""
    if system == "Darwin":
        m["os_version"] = "macOS " + out_of("sw_vers", "-productVersion", cwd=None)
        m["model"] = sysctl("hw.model")
        m["cpu"]["model"] = sysctl("machdep.cpu.brand_string") or m["cpu"]["model"]
        for key, name in (("physical_cores", "hw.physicalcpu"), ("performance_cores", "hw.perflevel0.physicalcpu"),
                          ("efficiency_cores", "hw.perflevel1.physicalcpu")):
            v = sysctl(name)
            if v.isdigit():
                m["cpu"][key] = int(v)
        mem = sysctl("hw.memsize")
        m["memory_bytes"] = int(mem) if mem.isdigit() else None
        batt = out_of("pmset", "-g", "batt", cwd=None)
        m["power"] = "ac" if "AC Power" in batt else "battery" if "Battery Power" in batt else "unknown"
        found = re.search(r'"IOPlatformUUID" = "([^"]+)"', out_of("ioreg", "-rd1", "-c", "IOPlatformExpertDevice", cwd=None))
        raw_id = found.group(1) if found else ""
    elif system == "Linux":
        pretty = re.search(r'^PRETTY_NAME="?([^"\n]*)', read("/etc/os-release"), re.M)
        if pretty:
            m["os_version"] = pretty.group(1)
        m["model"] = " ".join(x for x in (read("/sys/devices/virtual/dmi/id/sys_vendor").strip(),
                                          read("/sys/devices/virtual/dmi/id/product_name").strip()) if x)
        cpuinfo = read("/proc/cpuinfo")
        name = re.search(r"^model name\s*:\s*(.+)$", cpuinfo, re.M)
        if name:
            m["cpu"]["model"] = name.group(1).strip()
        cores = set(re.findall(r"^physical id\s*:\s*(\d+)\s*$.*?^core id\s*:\s*(\d+)\s*$", cpuinfo, re.M | re.S))
        if cores:
            m["cpu"]["physical_cores"] = len(cores)
        mem = re.search(r"^MemTotal:\s*(\d+) kB", read("/proc/meminfo"), re.M)
        m["memory_bytes"] = int(mem.group(1)) * 1024 if mem else None
        online = [read(os.path.join(d, "online")).strip() for d in _glob_dirs("/sys/class/power_supply", ("AC", "ADP", "ACAD"))]
        m["power"] = "ac" if "1" in online else "battery" if online else "unknown"
        raw_id = read("/etc/machine-id").strip()
    if not raw_id:
        raw_id = "%s|%s|%s|%s" % (platform.node(), m["model"], m["cpu"]["model"], m["memory_bytes"])
    m["id"] = hashlib.sha256(("shint-profile:" + raw_id).encode()).hexdigest()[:12]
    return m


QUIET_LIMIT = float(os.environ.get("SHINT_PROFILE_QUIET_WAIT", "90"))


def wait_for_quiet(cores, limit=None, share=0.3):
    """Wait (up to `limit` seconds) until the 1-minute load is at most `share` of the cores, so
    every run starts from a comparably quiet machine: the load average also carries whatever ran in
    the minute before - a build, the previous scenario, another program. Returns the seconds waited."""
    if not hasattr(os, "getloadavg"):
        return 0.0
    limit = QUIET_LIMIT if limit is None else limit
    t0, target = time.time(), max(1.0, (cores or 1) * share)
    while os.getloadavg()[0] > target and time.time() - t0 < limit:
        time.sleep(3)
    return round(time.time() - t0, 1)


def busy_note(m):
    """A note when the machine was already busy - its 1-minute load above half its cores -
    because other work competing for the processor makes every timing slower and noisier."""
    load, cores = m.get("load_1m_start"), (m.get("cpu") or {}).get("logical_cores") or 1
    if load is not None and load > cores / 2:
        return "the machine was busy at the start (1-minute load %.1f on %d cores): the timings are slower and noisier than on a quiet machine" % (load, cores)
    return ""


def _glob_dirs(base, prefixes):
    try:
        return [os.path.join(base, d) for d in os.listdir(base) if d.startswith(prefixes)]
    except OSError:
        return []


# --- building -----------------------------------------------------------------------------

def platforms(makefile=None):
    """(GOOS, GOARCH, GOARM) of every release target, as the Makefile's recipes build them."""
    seen = []
    for line in read(makefile or os.path.join(REPO, "Makefile")).splitlines():
        m = re.search(r"GOOS=(\w+) GOARCH=(\w+)(?: GOARM=(\d+))? go build -o bin/", line)
        if m and (m.group(1), m.group(2), m.group(3) or "") not in seen:
            seen.append((m.group(1), m.group(2), m.group(3) or ""))
    return seen


def go_build(dest, goos=None, goarch=None, goarm="", tags=""):
    """This tree's binary with the release flags (trimmed, stripped, no cgo); True if it built."""
    env = {"CGO_ENABLED": "0"}
    if goos:
        env.update(GOOS=goos, GOARCH=goarch)
    if goarm:
        env["GOARM"] = goarm
    args = ["go", "build", "-buildvcs=true", "-trimpath", "-ldflags", "-s -w", "-o", dest]
    if tags:
        args[2:2] = ["-tags", tags]
    r = subprocess.run(args + ["."], cwd=REPO, capture_output=True, text=True, env=dict(os.environ, **env))
    if r.returncode != 0:
        say("  build failed (%s): %s" % (goos and "%s/%s" % (goos, goarch) or "host", r.stderr.strip()[:300]))
    return r.returncode == 0


def platform_sizes(work):
    sizes = {}
    for goos, goarch, goarm in platforms():
        dest = os.path.join(work, "shint-%s-%s%s" % (goos, goarch, goarm))
        if go_build(dest, goos, goarch, goarm):
            sizes["%s/%s%s" % (goos, goarch, ("v" + goarm) if goarm else "")] = os.path.getsize(dest)
    return sizes


def has_profiling_hook():
    """True when the tree has the `profiling` build tag's hook (SHINT_CPUPROFILE, SHINT_MEMPROFILE)."""
    for name in os.listdir(REPO):
        if name.endswith(".go") and not name.endswith("_test.go") and "//go:build profiling" in read(os.path.join(REPO, name)):
            return True
    return False


# --- scenarios ----------------------------------------------------------------------------

def start_servers():
    """The battery's loopback servers (HTTP, TCP and UDP echo, DNS); returns their ports."""
    sys.path.insert(0, BATTERY)
    import env as battery_env  # noqa: E402
    import servers  # noqa: E402
    servers.start(None)
    return dict(battery_env.PORTS)


def available(binary, args):
    """Whether this binary has the scenario's command (--backfill runs versions without some)."""
    if not args or args[0].startswith("-"):
        return True
    try:
        r = subprocess.run([binary, args[0], "--help"], capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        return False
    return r.returncode == 0 and "unknown command" not in (r.stdout + r.stderr)


def maxrss_bytes(ru):
    return ru.ru_maxrss if sys.platform == "darwin" else ru.ru_maxrss * 1024   # Linux reports KiB


def measure(argv, env=None, timeout=180, during=None):
    """Run once; the process's own wall time, CPU time and peak memory (wait4), and exit code.
    during, if given, runs in this thread while the process does (the listen scenario's clients)."""
    t0 = time.perf_counter()
    p = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                         env=dict(os.environ, **(env or {})))
    killer = threading.Timer(timeout, p.kill)
    killer.start()
    try:
        if during:
            during()
        _, status, ru = os.wait4(p.pid, 0)
    finally:
        killer.cancel()
    wall = time.perf_counter() - t0
    p.returncode = os.waitstatus_to_exitcode(status)
    return {"wall_ms": wall * 1000, "user_ms": ru.ru_utime * 1000, "sys_ms": ru.ru_stime * 1000,
            "cpu_ms": (ru.ru_utime + ru.ru_stime) * 1000, "max_rss_mib": maxrss_bytes(ru) / 1048576,
            "exit": p.returncode}


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def listen_clients(port, requests=LISTEN_REQUESTS, clients=LISTEN_CLIENTS):
    """Wait for the listener, then send it `requests` requests from `clients` threads."""
    deadline = time.time() + 10
    while time.time() < deadline:
        try:
            socket.create_connection(("127.0.0.1", port), timeout=1).close()
            break
        except OSError:
            time.sleep(0.02)

    def one():
        try:
            c = socket.create_connection(("127.0.0.1", port), timeout=10)
            c.sendall(b"GET / HTTP/1.1\r\nHost: profile\r\nConnection: close\r\n\r\n")
            while c.recv(65536):
                pass
            c.close()
        except OSError:
            pass

    def worker(n):
        for _ in range(n):
            one()

    share = [requests // clients + (1 if i < requests % clients else 0) for i in range(clients)]
    threads = [threading.Thread(target=worker, args=(n,)) for n in share]
    for t in threads:
        t.start()
    for t in threads:
        t.join()


def scenario_argv(binary, args, ports):
    """The argv of one run, and what to do while it runs; for the listen scenario, a fresh port."""
    if args is None:
        port = free_port()
        return [binary, "listen", "http", str(port), "--count", str(LISTEN_REQUESTS)], (lambda: listen_clients(port))
    return [binary] + [a.format(**ports) for a in args], None


def scenario_entry(what, shown, runs):
    exits = [r["exit"] for r in runs]
    return {"what": what, "args": shown, "available": True, "repeat": len(runs), "ok": all(e == 0 for e in exits),
            "exit_codes": exits, "wall_ms": stats([r["wall_ms"] for r in runs]), "cpu_ms": stats([r["cpu_ms"] for r in runs]),
            "user_ms": stats([r["user_ms"] for r in runs]), "sys_ms": stats([r["sys_ms"] for r in runs]),
            "max_rss_mib": stats([r["max_rss_mib"] for r in runs])}


def run_interleaved(binaries, ports, repeat_scale=1.0, only=None, log=True):
    """Every scenario for one or more binaries: {label: {scenario: entry}}. With several, each
    round runs every binary once, in an order that rotates from round to round, so whatever
    drifts while the rounds run - the machine warming up, other work coming and going - falls on
    every binary alike. (Measured one after another, it does not: a backfill once showed v4.2.3
    30-45% slower than v4.0.6 on short scenarios, which measured side by side are within 3%.)"""
    labels = list(binaries)
    results = {l: {} for l in labels}
    for name, args, repeat, what in SCENARIOS:
        if only and name not in only:
            continue
        shown = ["listen", "http", "<port>", "--count", str(LISTEN_REQUESTS)] if args is None else args
        have = [l for l in labels if available(binaries[l], args or ["listen"])]
        for l in labels:
            if l not in have:
                results[l][name] = {"what": what, "args": shown, "available": False}
        if not have:
            if log:
                say("  %-12s n/a (no such command)" % name)
            continue
        n = max(1, int(round(repeat * repeat_scale)))
        runs = {l: [] for l in have}
        for i in range(n + 1):                                  # round 0 is a warm-up, not kept
            k = i % len(have)
            for l in have[k:] + have[:k]:
                argv, during = scenario_argv(binaries[l], args, ports)
                r = measure(argv, during=during, timeout=60 if during else 180)
                if i:
                    runs[l].append(r)
        for l in have:
            results[l][name] = scenario_entry(what, shown, runs[l])
        if log:
            e = results[have[0]][name]
            if len(labels) == 1:
                say("  %-12s wall %8.1f ms   cpu %8.1f ms   peak %6.1f MiB%s" % (
                    name, e["wall_ms"]["median"], e["cpu_ms"]["median"], e["max_rss_mib"]["median"],
                    "" if e["ok"] else "   (exit %s)" % sorted(set(e["exit_codes"]))))
            else:
                say("  %-12s wall ms: %s" % (name, "  ".join("%s %s" % (l, fmt(results[l][name]["wall_ms"]["median"])) for l in have)))
    return results


def run_scenarios(binary, ports, repeat_scale=1.0, only=None, log=True):
    return run_interleaved({"this": binary}, ports, repeat_scale, only, log)["this"]


# --- tests and profiles -------------------------------------------------------------------

def go_tests():
    """Each package's and the slowest tests' time, from one go test -json run (no race detector)."""
    t0 = time.time()
    r = subprocess.run(["go", "test", "-count=1", "-json", "./..."], cwd=REPO, capture_output=True, text=True)
    packages, tests = {}, []
    for line in r.stdout.splitlines():
        try:
            ev = json.loads(line)
        except ValueError:
            continue
        if ev.get("Action") not in ("pass", "fail", "skip") or "Elapsed" not in ev:
            continue
        if ev.get("Test"):
            if "/" not in ev["Test"]:
                tests.append({"test": ev["Test"], "package": ev["Package"], "seconds": ev["Elapsed"], "result": ev["Action"]})
        else:
            packages[ev["Package"]] = {"seconds": ev["Elapsed"], "result": ev["Action"]}
    tests.sort(key=lambda t: -t["seconds"])
    return {"ok": r.returncode == 0, "wall_seconds": round(time.time() - t0, 2), "packages": packages, "slowest": tests[:15]}


def pprof_top(binary, profile, dest, extra=()):
    text = out_of("go", "tool", "pprof", "-top", "-nodecount=30", *extra, binary, profile)
    if text:
        with open(dest, "w") as f:
            f.write(text + "\n")


def go_test_profiles(run_dir, work):
    """CPU and memory profiles of the packages whose tests run shint's code in-process."""
    made = []
    for pkg in PROFILED_PACKAGES:
        name = pkg.strip("./").replace("/", "-")
        testbin = os.path.join(work, name + ".test")
        cpu = os.path.join(run_dir, "test-cpu-%s.pprof" % name)
        mem = os.path.join(run_dir, "test-mem-%s.pprof" % name)
        r = subprocess.run(["go", "test", "-count=1", "-o", testbin, "-cpuprofile", cpu, "-memprofile", mem, pkg],
                           cwd=REPO, capture_output=True, text=True)
        if r.returncode != 0:
            say("  go test %s with profiles failed: %s" % (pkg, (r.stdout + r.stderr).strip()[-300:]))
        for prof, extra in ((cpu, ()), (mem, ("-sample_index=alloc_space",))):
            if os.path.exists(prof):
                pprof_top(testbin, prof, prof.replace(".pprof", ".top.txt"), extra)
                made.append(os.path.basename(prof))
    return made


COMMAND_PROFILE_RUNS = 10


def command_profiles(run_dir, work, ports):
    """With the `profiling` hook: the binary's own CPU and memory profile in each scenario.
    A run of a few milliseconds gives the CPU profiler (100 samples a second) almost nothing,
    so each scenario runs COMMAND_PROFILE_RUNS times and the profiles are merged."""
    binary = os.path.join(work, "shint-profiling")
    if not go_build(binary, tags="profiling"):
        return []
    made = []
    for name, args, _, _ in SCENARIOS:
        if name in ("startup", "help", "cidr", "ip") or not available(binary, args or ["listen"]):
            continue
        parts = {"cpu": [], "mem": []}
        for i in range(COMMAND_PROFILE_RUNS):
            cpu = os.path.join(work, "%s-cpu-%d.pprof" % (name, i))
            mem = os.path.join(work, "%s-mem-%d.pprof" % (name, i))
            argv, during = scenario_argv(binary, args, ports)
            measure(argv, env={"SHINT_CPUPROFILE": cpu, "SHINT_MEMPROFILE": mem}, during=during,
                    timeout=60 if during else 180)
            for kind, path in (("cpu", cpu), ("mem", mem)):
                if os.path.exists(path) and os.path.getsize(path):
                    parts[kind].append(path)
        for kind, extra in (("cpu", ()), ("mem", ("-sample_index=alloc_space",))):
            if not parts[kind]:
                continue
            merged = os.path.join(run_dir, "cmd-%s-%s.pprof" % (kind, name))
            with open(merged, "wb") as f:
                r = subprocess.run(["go", "tool", "pprof", "-proto", binary] + parts[kind], cwd=REPO, stdout=f,
                                   stderr=subprocess.DEVNULL)
            if r.returncode != 0 or not os.path.getsize(merged):
                os.remove(merged)
                continue
            pprof_top(binary, merged, merged.replace(".pprof", ".top.txt"), extra)
            made.append(os.path.basename(merged))
    return made


def battery(binary, work):
    """The black-box battery's time per group and its slowest cases (its verdicts are make check's job)."""
    report = os.path.join(work, "battery.json")
    t0 = time.time()
    r = subprocess.run([sys.executable, os.path.join(BATTERY, "run.py"), "--bin", binary, "--report", report],
                       cwd=REPO, capture_output=True, text=True)
    wall = round(time.time() - t0, 2)
    try:
        results = json.load(open(report))
    except (OSError, ValueError):
        return {"ok": False, "wall_seconds": wall, "error": (r.stdout + r.stderr).strip()[-300:]}
    groups = {}
    for c in results:
        g = groups.setdefault(c.get("group", "?"), {"cases": 0, "seconds": 0.0})
        g["cases"] += 1
        g["seconds"] = round(g["seconds"] + float(c.get("dur") or 0), 2)
    slowest = sorted(results, key=lambda c: -float(c.get("dur") or 0))[:10]
    return {"ok": r.returncode == 0, "wall_seconds": wall, "cases": len(results), "groups": groups,
            "slowest": [{"id": c.get("id"), "group": c.get("group"), "seconds": round(float(c.get("dur") or 0), 2)} for c in slowest]}


def time_wait_count():
    """Sockets in TIME_WAIT: netstat on macOS, ss (whose first column is the state) or netstat on Linux."""
    if sys.platform == "darwin":
        text = out_of("netstat", "-an", "-p", "tcp", cwd=None)
    else:
        text = out_of("ss", "-tan", cwd=None) or out_of("netstat", "-tan", cwd=None)
    return sum(1 for l in text.splitlines() if "TIME_WAIT" in l or "TIME-WAIT" in l)


def settle(limit=1000, wait=90):
    """Wait for the connections this run closed to leave TIME_WAIT (issue #78)."""
    t0 = time.time()
    n = time_wait_count()
    while n > limit and time.time() - t0 < wait:
        time.sleep(3)
        n = time_wait_count()
    return {"time_wait_after": n, "waited_seconds": round(time.time() - t0, 1)}


# --- a run --------------------------------------------------------------------------------

def git_source():
    return {
        "commit": out_of("git", "rev-parse", "HEAD"),
        "describe": out_of("git", "describe", "--tags", "--always", "--dirty"),
        "branch": out_of("git", "rev-parse", "--abbrev-ref", "HEAD"),
        "dirty": bool(out_of("git", "status", "--porcelain", "--untracked-files=no")),
        "version": (re.search(r'Version\s+string\s*=\s*"([^"]*)"', read(os.path.join(REPO, "main.go"))) or [None, None])[1],
    }


def run_dir_for(tag, kind, now=None):
    base = os.path.join(OUT, "dev") if kind == "dev" else OUT
    d = os.path.join(base, tag, utc_stamp(now))
    os.makedirs(d, exist_ok=True)
    return d


def write_summary(run_dir, summary):
    with open(os.path.join(run_dir, "summary.json"), "w") as f:
        json.dump(summary, f, indent=1, sort_keys=False)
        f.write("\n")


def profile_tree(tag, quick):
    t0 = time.time()
    kind = "release" if tag else "dev"
    source = git_source()
    tag = tag or source["describe"] or "unknown"
    run_dir = run_dir_for(tag, kind)
    say("profile: %s -> %s" % (tag, os.path.relpath(run_dir, REPO)))
    mach = machine()
    say("  waiting for a quiet machine (1-minute load at most 30%% of %s cores, up to 90 s) ..." % mach["cpu"]["logical_cores"])
    mach["quiet_wait_seconds"] = wait_for_quiet(mach["cpu"]["logical_cores"])
    mach["load_1m_start"] = round(os.getloadavg()[0], 2) if hasattr(os, "getloadavg") else None
    summary = {"schema": SCHEMA, "tag": tag, "kind": kind, "run_at": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds"),
               "source": source, "machine": mach, "notes": []}
    say("  machine %s: %s, %s, %s cores, %s GiB, power %s" % (
        summary["machine"]["id"], summary["machine"]["os_version"], summary["machine"]["cpu"]["model"],
        summary["machine"]["cpu"]["logical_cores"], round((summary["machine"]["memory_bytes"] or 0) / 2**30, 1), summary["machine"]["power"]))
    if busy_note(summary["machine"]):
        summary["notes"].append(busy_note(summary["machine"]))
        say("  NOTE: " + busy_note(summary["machine"]))
    work = tempfile.mkdtemp(prefix="shint-profile-")
    try:
        host = os.path.join(work, "shint")
        if not go_build(host):
            raise SystemExit("profile: this tree does not build")
        summary["binary"] = {"host_size_bytes": os.path.getsize(host), "platform_sizes": {}}
        ports = start_servers()
        say("scenarios (median of each):")
        summary["scenarios"] = run_scenarios(host, ports)
        profiles = []
        if not quick:
            say("binary sizes, every release platform ...")
            summary["binary"]["platform_sizes"] = platform_sizes(work)
            say("go test -json ...")
            summary["go_tests"] = go_tests()
            say("go test profiles (%s) ..." % ", ".join(PROFILED_PACKAGES))
            profiles += go_test_profiles(run_dir, work)
            if has_profiling_hook():
                say("the binary's own profiles, per scenario ...")
                profiles += command_profiles(run_dir, work, ports)
            else:
                summary["notes"].append("no `profiling` build tag hook in this tree: no per-scenario profiles of the binary")
            say("black-box battery ...")
            summary["battery"] = battery(host, work)
        summary["profiles"] = sorted(profiles)
    finally:
        shutil.rmtree(work, ignore_errors=True)
    summary["machine"]["load_1m_end"] = round(os.getloadavg()[0], 2) if hasattr(os, "getloadavg") else None
    say("waiting for TIME_WAIT sockets to drain ...")
    summary["settle"] = settle()
    summary["duration_seconds"] = round(time.time() - t0, 1)
    write_summary(run_dir, summary)
    say("profile: done in %.0f s - %s" % (summary["duration_seconds"], os.path.relpath(run_dir, REPO)))
    return run_dir


def host_asset():
    goos = {"Darwin": "darwin", "Linux": "linux"}.get(platform.system())
    goarch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(platform.machine().lower())
    if not goos or not goarch:
        raise SystemExit("no release binary for this platform (%s/%s)" % (platform.system(), platform.machine()))
    return "shint.%s.%s" % (goos, goarch)


def github(url):
    req = urllib.request.Request(url, headers={"User-Agent": "shint-profile", "Accept": "application/vnd.github+json"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return r.read()


def release_binary(tag, work):
    """A release's binary for this machine, from its public download URL (no gh needed)."""
    asset = host_asset()
    dest = os.path.join(work, tag, asset)
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    with open(dest, "wb") as f:
        f.write(github("https://github.com/%s/releases/download/%s/%s" % (REPO_SLUG, tag, asset)))
    os.chmod(dest, 0o755)
    return dest


def backfill(tags):
    """Released binaries for this machine's platform, downloaded and measured with the same
    scenarios - all of them interleaved, round by round (see run_interleaved)."""
    asset = host_asset()
    work = tempfile.mkdtemp(prefix="shint-backfill-")
    try:
        bins, infos = {}, {}
        for tag in tags:
            try:
                bins[tag] = release_binary(tag, work)
                infos[tag] = json.loads(github("https://api.github.com/repos/%s/releases/tags/%s" % (REPO_SLUG, tag)))
            except (OSError, ValueError) as e:
                say("backfill %s: no %s (%s)" % (tag, asset, e))
                bins.pop(tag, None)
        if not bins:
            raise SystemExit("backfill: nothing to measure")
        mach = machine()
        say("backfill: %d releases (%s), interleaved; machine %s" % (len(bins), " ".join(bins), mach["id"]))
        mach["quiet_wait_seconds"] = wait_for_quiet(mach["cpu"]["logical_cores"])
        mach["load_1m_start"] = round(os.getloadavg()[0], 2)
        note = busy_note(mach)
        if note:
            say("  NOTE: " + note)
        ports = start_servers()
        t0 = time.time()
        at = datetime.datetime.now(datetime.timezone.utc)
        results = run_interleaved(bins, ports)
        mach["load_1m_end"] = round(os.getloadavg()[0], 2)
        settled = settle()
        for tag, binary in bins.items():
            info = infos.get(tag, {})
            sizes = {}
            for a in info.get("assets", []):
                m = re.match(r"^shint\.([a-z0-9]+)\.([a-z0-9]+)(?:\.exe)?$", a["name"])
                if m:
                    sizes["%s/%s" % (m.group(1), m.group(2))] = a["size"]
            summary = {"schema": SCHEMA, "tag": tag, "kind": "backfill", "run_at": at.isoformat(timespec="seconds"),
                       "source": {"release": info.get("html_url", ""), "published_at": info.get("published_at", ""),
                                  "asset": asset, "sha256": hashlib.sha256(open(binary, "rb").read()).hexdigest(),
                                  "version_output": out_of(binary, "--version", cwd=None)},
                       "machine": dict(mach), "binary": {"host_size_bytes": os.path.getsize(binary), "platform_sizes": sizes},
                       "scenarios": results[tag], "settle": settled, "duration_seconds": round(time.time() - t0, 1),
                       "notes": ["measured after the fact from the released binary: scenarios and sizes only - no tests, battery or profiles",
                                 "measured interleaved with %s, round by round, so drift during the run falls on all of them alike"
                                 % ", ".join(x for x in bins if x != tag) if len(bins) > 1 else "measured on its own"] + ([note] if note else [])}
            write_summary(run_dir_for(tag, "backfill", at), summary)
            say("  wrote .profiling/%s/%s" % (tag, utc_stamp(at)))
    finally:
        shutil.rmtree(work, ignore_errors=True)


def ab(ref_a, ref_b, only=None, repeat_scale=2.0):
    """Two builds head to head, now, on this machine, interleaved: the fairest comparison there
    is. A ref is a release tag (its published binary), HEAD (this tree, built), or a binary's path."""
    work = tempfile.mkdtemp(prefix="shint-ab-")
    try:
        def binary(ref):
            if os.path.isfile(ref):
                return ref, os.path.basename(ref)
            if ref in ("HEAD", ".", "tree"):
                dest = os.path.join(work, "tree", "shint")
                os.makedirs(os.path.dirname(dest))
                if not go_build(dest):
                    raise SystemExit("ab: this tree does not build")
                return dest, "HEAD(%s)" % (out_of("git", "describe", "--tags", "--always", "--dirty") or "tree")
            if re.match(r"^v\d+\.\d+\.\d+$", ref):
                return release_binary(ref, work), ref
            raise SystemExit("ab: %r is not a release tag, HEAD or a binary" % ref)
        (pa, la), (pb, lb) = binary(ref_a), binary(ref_b)
        if la == lb:
            la, lb = la + "(A)", lb + "(B)"
        mach = machine()
        say("ab: %s vs %s, interleaved, on machine %s" % (la, lb, mach["id"]))
        mach["quiet_wait_seconds"] = wait_for_quiet(mach["cpu"]["logical_cores"])
        mach["load_1m_start"] = round(os.getloadavg()[0], 2)
        res = run_interleaved({la: pa, lb: pb}, start_servers(), repeat_scale, only)
        now = datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds")
        a, b = ({"tag": l, "kind": "ab", "run_at": now, "machine": mach, "binary": {"host_size_bytes": os.path.getsize(path)},
                 "scenarios": res[l]} for l, path in ((la, pa), (lb, pb)))
        settle()
        return compare_summaries(a, b)
    finally:
        shutil.rmtree(work, ignore_errors=True)


# --- reading the history ------------------------------------------------------------------

METRICS = {"wall": ("wall_ms", "ms", "wall time"), "cpu": ("cpu_ms", "ms", "CPU time"), "rss": ("max_rss_mib", "MiB", "peak memory")}
SPARKS = "▁▂▃▄▅▆▇█"


def version_key(tag):
    m = re.match(r"^v(\d+)\.(\d+)\.(\d+)", tag or "")
    return tuple(int(x) for x in m.groups()) if m else (10 ** 9,)


def load_runs(dev=False):
    """Every run's summary, with its folder in "_path": the committed ones (releases and
    backfills), and with dev=True your own too - in version order, then time order."""
    patterns = [os.path.join(OUT, "v*", "*", "summary.json")]
    if dev:
        patterns.append(os.path.join(OUT, "dev", "*", "*", "summary.json"))
    runs = []
    for pattern in patterns:
        for f in glob.glob(pattern):
            try:
                with open(f) as fh:
                    s = json.load(fh)
            except (OSError, ValueError):
                continue
            s["_path"] = os.path.dirname(f)
            runs.append(s)
    runs.sort(key=lambda s: (version_key(s.get("tag")), s.get("run_at", "")))
    return runs


def machine_of(run):
    return run.get("machine", {}).get("id", "?")


def latest_per_build(runs, mid):
    """Machine mid's runs, one per build (its latest), in version order."""
    out = {}
    for r in runs:
        if machine_of(r) == mid:
            out.pop(r["tag"], None)
            out[r["tag"]] = r
    return sorted(out.values(), key=lambda r: version_key(r["tag"]))


def pick_machine(runs, wanted=None):
    """The machine asked for; else this one, if it has runs; else the one with the most runs."""
    ids = [machine_of(r) for r in runs]
    if wanted:
        match = sorted({i for i in ids if i.startswith(wanted)})
        if len(match) != 1:
            raise SystemExit("no single machine matches %r (machines: %s)" % (wanted, ", ".join(sorted(set(ids)))))
        return match[0]
    here = machine()["id"]
    if here in ids:
        return here
    return max(sorted(set(ids)), key=ids.count)


def describe_machine(run):
    m = run.get("machine", {})
    cpu = m.get("cpu", {})
    cores = "%s cores" % cpu.get("logical_cores", "?")
    if cpu.get("performance_cores"):
        cores += " (%s+%s)" % (cpu["performance_cores"], cpu.get("efficiency_cores", 0))
    parts = [m.get("label"), m.get("model"), cpu.get("model"), cores,
             "%.0f GiB" % (m["memory_bytes"] / 2 ** 30) if m.get("memory_bytes") else "", m.get("os_version") or m.get("os")]
    return ", ".join(x for x in parts if x)


def fmt(v):
    if v is None:
        return "n/a"
    return "%.0f" % v if abs(v) >= 100 else "%.1f" % v if abs(v) >= 10 else "%.2f" % v


def spark(values):
    known = [v for v in values if v is not None]
    if not known:
        return ""
    lo, hi = min(known), max(known)
    return "".join(" " if v is None else SPARKS[0 if hi == lo else int((v - lo) / (hi - lo) * (len(SPARKS) - 1))] for v in values)


def change(a, b):
    if a in (None, 0) or b is None:
        return ""
    return "%+.1f%%" % ((b - a) / a * 100)


def noisy(sa, sb, key):
    """True when the change between two runs is smaller than either run's own spread (max - min)."""
    try:
        a, b = sa[key], sb[key]
        d = abs(b["median"] - a["median"]) / a["median"]
        return d <= max((a["max"] - a["min"]) / a["median"], (b["max"] - b["min"]) / b["median"])
    except (KeyError, TypeError, ZeroDivisionError):
        return False


def history(wanted=None, metric="wall", last=None, dev=False):
    """Every scenario across one machine's builds, with a trend and the first-to-last change."""
    runs = load_runs(dev)
    if not runs:
        return "no profiling runs in %s" % os.path.relpath(OUT, REPO)
    mid = pick_machine(runs, wanted)
    builds = latest_per_build(runs, mid)
    if last:
        builds = builds[-last:]
    key, unit, label = METRICS[metric]
    names = []
    for r in builds:
        names += [n for n in r.get("scenarios", {}) if n not in names]
    width = max(8, max(len(r["tag"]) for r in builds) + 1)
    lines = ["machine %s: %s" % (mid, describe_machine(builds[-1])),
             "%s (%s, median) over %d build%s%s" % (label, unit, len(builds), "" if len(builds) == 1 else "s",
                                                   "; * = measured later from the published binary" if any(r.get("kind") == "backfill" for r in builds) else ""),
             ""]
    lines.append("%-12s" % "scenario" + "".join("%*s" % (width, r["tag"] + ("*" if r.get("kind") == "backfill" else "")) for r in builds)
                 + "   trend" + " " * max(0, len(builds) - 5) + "  first->last")

    def row(name, values, first_last_noisy=False):
        known = [v for v in values if v is not None]
        delta = change(known[0], known[-1]) if len(known) > 1 else ""
        lines.append("%-12s" % name + "".join("%*s" % (width, fmt(v)) for v in values)
                     + "   %-*s  %s%s" % (max(5, len(values)), spark(values), delta, " ~" if first_last_noisy and delta else ""))

    for name in names:
        values, stats_ = [], []
        for r in builds:
            sc = r.get("scenarios", {}).get(name)
            ok = sc and sc.get("available")
            values.append(sc[key]["median"] if ok else None)
            stats_.append(sc if ok else None)
        known = [s for s in stats_ if s]
        row(name, values, len(known) > 1 and noisy(known[0], known[-1], key))
    lines.append("")
    row("load 1m", [r.get("machine", {}).get("load_1m_start") for r in builds])
    row("binary MiB", [r.get("binary", {}).get("host_size_bytes", 0) / 2 ** 20 or None for r in builds])
    for k, label_ in (("go_tests", "go test s"), ("battery", "battery s")):
        vals = [(r.get(k) or {}).get("wall_seconds") for r in builds]
        if any(v is not None for v in vals):
            row(label_, vals)
    lines.append("")
    lines.append("~ : the first-to-last change is smaller than the runs' own spread (noise). load 1m: the machine's load when the")
    lines.append("run started - a busy machine is slower for every scenario. Other machines: make profile-machines")
    return "\n".join(lines)


def machines_table(dev=False):
    runs = load_runs(dev)
    if not runs:
        return "no profiling runs in %s" % os.path.relpath(OUT, REPO)
    here = machine()["id"]
    lines = []
    for mid in sorted({machine_of(r) for r in runs}, key=lambda i: (i != here, i)):
        mine = [r for r in runs if machine_of(r) == mid]
        builds = latest_per_build(runs, mid)
        lines.append("%s%s  %d run%s, %d build%s (%s .. %s), last %s" % (
            mid, "  (this machine)" if mid == here else "", len(mine), "" if len(mine) == 1 else "s",
            len(builds), "" if len(builds) == 1 else "s", builds[0]["tag"], builds[-1]["tag"], mine[-1].get("run_at", "")[:10]))
        lines.append("    " + describe_machine(mine[-1]))
    return "\n".join(lines)


def resolve(ref, mid=None):
    """A run folder, a summary.json, or a tag: its latest run - on machine mid, if given."""
    if os.path.isfile(ref):
        return ref
    if os.path.isfile(os.path.join(ref, "summary.json")):
        return os.path.join(ref, "summary.json")
    runs = [r for r in load_runs(dev=True) if r.get("tag") == ref and (mid is None or machine_of(r) == mid)]
    if runs:
        return os.path.join(runs[-1]["_path"], "summary.json")
    raise SystemExit("compare: no profiling run found for %r%s" % (ref, " on machine %s" % mid if mid else ""))


def compare(ref_a, ref_b, wanted=None):
    """Two runs side by side. Two tags are compared on the same machine: the one asked for, or
    this one when both builds have run on it; failing that, each tag's latest run."""
    mid = None
    if wanted:
        mid = pick_machine(load_runs(dev=True), wanted)
    else:
        here = machine()["id"]
        try:
            resolve(ref_a, here), resolve(ref_b, here)
            mid = here
        except SystemExit:
            mid = None
    with open(resolve(ref_a, mid)) as fa, open(resolve(ref_b, mid)) as fb:
        return compare_summaries(json.load(fa), json.load(fb))


def compare_summaries(a, b):
    lines = ["%s (%s, %s)  ->  %s (%s, %s)" % (a["tag"], a["kind"], a["run_at"], b["tag"], b["kind"], b["run_at"])]
    ma, mb = a.get("machine", {}), b.get("machine", {})
    if ma.get("id") != mb.get("id"):
        lines.append("WARNING: different machines (%s, %s) - the timings are not comparable, only the sizes are" % (ma.get("id"), mb.get("id")))
    else:
        lines.append("machine %s: %s" % (ma.get("id"), describe_machine(b)))
        if ma.get("power") != mb.get("power"):
            lines.append("note: power source differs (%s, %s)" % (ma.get("power"), mb.get("power")))
        lines.append("load (1 min) at the start: %s, %s - a busy machine is slower for every scenario" % (ma.get("load_1m_start"), mb.get("load_1m_start")))
    lines.append("")
    lines.append("%-12s %12s %12s %9s   %10s %10s %9s   %10s %9s %9s" % ("scenario", "wall ms A", "B", "change", "cpu ms A", "B", "change", "peak MiB A", "B", "change"))
    names = list(a.get("scenarios", {})) + [n for n in b.get("scenarios", {}) if n not in a.get("scenarios", {})]
    for name in names:
        sa, sb = a.get("scenarios", {}).get(name), b.get("scenarios", {}).get(name)
        if not sa or not sb or not sa.get("available") or not sb.get("available"):
            lines.append("%-12s %s" % (name, "n/a in " + ", ".join(x["tag"] for x, s in ((a, sa), (b, sb)) if not s or not s.get("available"))))
            continue
        cols = []
        for key in ("wall_ms", "cpu_ms", "max_rss_mib"):
            va, vb = sa[key]["median"], sb[key]["median"]
            cols.append((va, vb, change(va, vb) + (" ~" if noisy(sa, sb, key) else "")))
        lines.append("%-12s %12.1f %12.1f %9s   %10.1f %10.1f %9s   %10.1f %9.1f %9s" % ((name,) + tuple(x for c in cols for x in c)))
    ba, bb = a.get("binary", {}), b.get("binary", {})
    lines.append("")
    lines.append("binary (this platform): %s -> %s bytes %s" % (ba.get("host_size_bytes"), bb.get("host_size_bytes"),
                                                             change(ba.get("host_size_bytes"), bb.get("host_size_bytes"))))
    for key, label in (("go_tests", "go test"), ("battery", "battery")):
        if a.get(key) and b.get(key):
            lines.append("%s: %s s -> %s s %s" % (label, a[key]["wall_seconds"], b[key]["wall_seconds"],
                                                  change(a[key]["wall_seconds"], b[key]["wall_seconds"])))
    lines.append("")
    lines.append("~ : smaller than the runs' own spread (noise)")
    return "\n".join(lines)


def main(argv):
    ap = argparse.ArgumentParser(description="Runtime stats and profiles of a shint build (see the docstring).")
    ap.add_argument("--tag", help="a release tag vX.Y.Z: the run goes to .profiling/<tag>/ (default: .profiling/dev/<git describe>/)")
    ap.add_argument("--quick", action="store_true", help="the machine, the scenarios and the binary size only")
    ap.add_argument("--backfill", nargs="+", metavar="TAG", help="measure these releases' published binaries instead of this tree")
    ap.add_argument("--compare", nargs=2, metavar=("A", "B"), help="compare two runs (a tag, a run folder or a summary.json)")
    ap.add_argument("--history", action="store_true", help="every scenario across one machine's builds, with a trend")
    ap.add_argument("--machines", action="store_true", help="every machine that has profiling runs")
    ap.add_argument("--machine", help="the machine (its id, or the start of it) for --history and --compare; default: this one")
    ap.add_argument("--metric", choices=sorted(METRICS), default="wall", help="for --history: wall, cpu or rss (default wall)")
    ap.add_argument("--last", type=int, help="for --history: only the last N builds")
    ap.add_argument("--dev", action="store_true", help="for --history and --machines: include your own runs in .profiling/dev/")
    ap.add_argument("--ab", nargs=2, metavar=("A", "B"), help="two builds head to head, now, interleaved: a release tag, HEAD, or a binary's path")
    ap.add_argument("--only", help="for --ab: only these scenarios (comma-separated)")
    a = ap.parse_args(argv)
    if a.compare:
        print(compare(a.compare[0], a.compare[1], a.machine))
        return 0
    if a.ab:
        print(ab(a.ab[0], a.ab[1], a.only.split(",") if a.only else None))
        return 0
    if a.history:
        print(history(a.machine, a.metric, a.last, a.dev))
        return 0
    if a.machines:
        print(machines_table(a.dev))
        return 0
    if a.backfill:
        backfill(a.backfill)
        return 0
    if a.tag and not re.match(r"^v\d+\.\d+\.\d+$", a.tag):
        ap.error("--tag must be vX.Y.Z")
    profile_tree(a.tag, a.quick)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
