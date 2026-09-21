"""Declares, runs and judges the battery's cases.

A case is one shint invocation (add) or one small script that drives shint
(add_func). Every invocation is judged against invariants that hold for any
command - no panic, exit status 0/1/2, --json prints one valid JSON document,
usage errors go to stderr, an ERROR line goes with exit 1 and never with exit 0,
nothing hangs - plus whatever the case itself expects.

A case that reveals a known bug carries known="#<issue>". It must keep failing
until the bug is fixed: when it starts passing the run fails and says to remove
the marker (and close the issue), so the list of known problems cannot rot.
"""
import json
import os
import re
import signal
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor

import env

CASES = []

PANIC = re.compile(r"panic:|goroutine \d+ \[|runtime error|fatal error|SIGSEGV|unexpected signal")


def add(cid, args, rc=None, contains=(), absent=(), max=25, timeout=45, envvars=None, note="",
        wrap=None, check=None, group="", known=None, known_on=None, needs=(), serial=False):
    """One shint invocation. rc is the expected exit status (None: any of 0/1/2);
    max is the seconds it may take before it counts as slow; timeout kills it."""
    CASES.append(dict(id=cid, args=args, rc=rc, contains=contains, absent=absent, max=max,
                      timeout=timeout, envvars=envvars, note=note, wrap=wrap, check=check,
                      group=group, known=known, known_on=known_on, needs=needs,
                      serial=serial, func=None))


def add_func(cid, fn, group="", known=None, known_on=None, needs=(), note=""):
    """A scripted case: fn() returns None or a list of problems. Always run alone."""
    CASES.append(dict(id=cid, args=[], func=fn, group=group, known=known, known_on=known_on,
                      needs=needs, serial=True, note=note))


def fmt(args):
    def sub(x):
        def one(m):
            name = m.group(1)
            if name in env.PORTS:
                return str(env.PORTS[name])
            if name in env.VARS:
                return str(env.VARS[name])
            return m.group(0)
        return re.sub(r"\{(\w+)\}", one, x)
    return [sub(x) if isinstance(x, str) else str(x) for x in args]


def spawn(args, timeout=45, wrap=None, envvars=None):
    """Run shint with (already substituted) args. Returns (rc, out, err, seconds);
    rc is "TIMEOUT" when it had to be killed. The proxy variables are cleared."""
    cmd = [env.BIN] + list(args)
    if wrap:  # e.g. "ulimit -n 256;"
        cmd = ["/bin/sh", "-c", wrap + ' exec "$@"', "sh"] + cmd
    e = dict(os.environ)
    for k in ("HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"):
        e.pop(k, None)
    if envvars:
        e.update(envvars)
    t0 = time.time()
    p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, stdin=subprocess.DEVNULL,
                         env=e, start_new_session=True)
    try:
        out, err = p.communicate(timeout=timeout)
        rc = p.returncode
    except subprocess.TimeoutExpired:
        os.killpg(p.pid, signal.SIGKILL)
        out, err = p.communicate()
        rc = "TIMEOUT"
    return rc, out.decode("utf-8", "replace"), err.decode("utf-8", "replace"), time.time() - t0


def invariants(args, rc, out, err, dur, timeout, limit):
    flags = []
    if PANIC.search(out) or PANIC.search(err):
        flags.append("panic or crash text in the output")
    if rc == "TIMEOUT":
        flags.append("hung: killed after %ss" % timeout)
    elif rc not in (0, 1, 2):
        flags.append("unexpected exit status %r" % rc)
    is_json = "--json" in args and not (args and args[0] == "listen")
    if rc in (0, 1) and is_json:
        try:
            json.loads(out)
        except ValueError as e:
            flags.append("--json output is not one valid JSON document: %s" % str(e)[:80])
    if rc == 2:
        if out.strip():
            flags.append("usage error (exit 2) printed on stdout")
        if not err.strip():
            flags.append("usage error (exit 2) with nothing on stderr")
    if rc == 0 and not is_json and "ERROR" in out:
        flags.append("exit 0 although an ERROR line was printed")
    if rc == 1 and not is_json and "ERROR" not in out and "interrupted" not in out:
        flags.append("exit 1 without an ERROR line saying why")
    if rc in (0, 1) and not is_json and err.strip():
        flags.append("unexpected output on stderr from a completed run")
    if isinstance(rc, int) and dur > limit:
        flags.append("slow: %.1fs (limit %ss)" % (dur, limit))
    return flags


def run_case(c):
    r = dict(id=c["id"], group=c["group"], note=c.get("note", ""), known=c["known"], known_on=c["known_on"], flags=[], skipped=None, args=[], rc_actual=None, dur=0.0, out="", err="")
    missing = [n for n in c["needs"] if not env.FEATURES.get(n)]
    if missing:
        r["skipped"] = "needs " + ", ".join(missing)
        return r
    t0 = time.time()
    if c["func"]:
        try:
            extra = c["func"]()
            r["flags"] = [extra] if isinstance(extra, str) else list(extra or [])
        except Exception as e:  # a broken script is a failed case, not a crashed run
            r["flags"] = ["case script raised %s: %s" % (type(e).__name__, e)]
        r["dur"] = round(time.time() - t0, 2)
        return r
    args = fmt(c["args"])
    r["args"] = args
    try:
        rc, out, err, dur = spawn(args, c["timeout"], c["wrap"], c["envvars"])
    except OSError as e:
        r["flags"] = ["could not start %s: %s" % (env.BIN, e)]
        return r
    r.update(rc_actual=rc, dur=round(dur, 2), out=out, err=err)
    flags = invariants(args, rc, out, err, dur, c["timeout"], c["max"])
    if c["rc"] is not None and rc != c["rc"]:
        flags.append("expected exit %r, got %r" % (c["rc"], rc))
    for s in c["contains"]:
        s = fmt([s])[0]
        if s not in out and s not in err:
            flags.append("missing expected text %r" % s)
    for s in c["absent"]:
        if s in out or s in err:
            flags.append("unexpected text %r" % s)
    if c["check"]:
        try:
            extra = c["check"](r)
            if extra:
                flags += [extra] if isinstance(extra, str) else list(extra)
        except Exception as e:
            flags.append("check raised %s: %s" % (type(e).__name__, e))
    r["flags"] = flags
    if not flags:  # keep the full output only of a case that needs looking at
        r["out"], r["err"] = out[:2000], err[:1000]
    return r


def run_all(cases, jobs):
    """Parallel cases first, then the serial ones (ping and scripted cases, which
    must not share the machine's ICMP socket or timing with anything else)."""
    par = [c for c in cases if not c["serial"]]
    ser = [c for c in cases if c["serial"]]
    with ThreadPoolExecutor(max_workers=jobs) as ex:
        results = list(ex.map(run_case, par))
    results += [run_case(c) for c in ser]
    return results


def applies(r):
    """Does this result's known-issue marker apply on this platform?"""
    if not r["known"]:
        return False
    if not r["known_on"]:
        return True
    return any(sys.platform.startswith(p) for p in r["known_on"])


def verdict(r):
    if r["skipped"]:
        return "skip"
    if applies(r):
        return "xfail" if r["flags"] else "xpass"
    return "fail" if r["flags"] else "pass"
