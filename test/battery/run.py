#!/usr/bin/env python3
"""Run the black-box battery against a shint binary: python3 test/battery/run.py --bin ./shint
See README.md. Exit status 0 when every case passed or is a known issue that still fails."""
import argparse
import collections
import importlib
import json
import os
import re
import resource
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import env  # noqa: E402
import known_issues  # noqa: E402
import runner  # noqa: E402
import servers  # noqa: E402

MODULES = ["cases_cli", "cases_web", "cases_net", "cases_listen"]


def version_of(repo):
    m = re.search(r'Version\s*=\s*"([^"]+)"', open(os.path.join(repo, "main.go")).read())
    return m.group(1) if m else ""


def icmp_works():
    rc, _, _, _ = runner.spawn(["ping", "127.0.0.1", "--count", "1", "--timeout", "2"], timeout=15)
    return rc == 0


def show(r, full):
    print("  %s  (%s, %.1fs)" % (r["id"], r["group"], r["dur"]))
    if r["args"]:
        print("      $ shint " + " ".join(r["args"])[:200])
    for f in r["flags"]:
        print("      ! " + f)
    if full:
        lines = (r["out"] + r["err"]).splitlines()
        start = next((i for i, l in enumerate(lines) if runner.PANIC.search(l)), max(0, len(lines) - 8))
        for line in lines[start:start + 8]:  # from the panic itself, else the end of the output
            print("      | " + line[:170])


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--bin", required=True, help="the shint binary to test")
    ap.add_argument("--group", action="append", help="only groups starting with this (repeatable)")
    ap.add_argument("--only", help="only cases whose id contains this text")
    ap.add_argument("--jobs", type=int, default=8)
    ap.add_argument("--list", action="store_true", help="list the cases and exit")
    ap.add_argument("--verbose", action="store_true", help="also show every known issue and its output")
    ap.add_argument("--report", help="write every result as JSON to this file")
    a = ap.parse_args()

    # every test server lives in this one process: give it all the descriptors it may have
    soft, hard = resource.getrlimit(resource.RLIMIT_NOFILE)
    resource.setrlimit(resource.RLIMIT_NOFILE, (min(hard, 65536) if hard != resource.RLIM_INFINITY else 65536, hard))
    env.BIN = os.path.abspath(a.bin)
    repo = os.path.abspath(os.path.join(HERE, "..", ".."))
    certdir = tempfile.mkdtemp(prefix="shint-battery-")
    try:
        gen = subprocess.run(["go", "run", "./test/battery/gencert", certdir], cwd=repo, capture_output=True, text=True)
        if gen.returncode != 0:
            print("note: could not generate test certificates (%s); TLS cases are skipped" % gen.stderr.strip()[:120])
        feats = servers.start(certdir if gen.returncode == 0 else None)
        env.VARS.update(good_pem=certdir + "/good.pem", good_key=certdir + "/good.key",
                        bad_pem=certdir + "/bad.pem", bad_key=certdir + "/bad.key",
                        not_pem=os.path.join(HERE, "run.py"), version=version_of(repo))
        env.FEATURES.update(feats)
        env.FEATURES["unix"] = os.name == "posix"
        env.FEATURES["icmp"] = False if a.list else icmp_works()
        for m in MODULES:
            importlib.import_module(m)

        ids = {c["id"] for c in runner.CASES}
        stale = sorted(set(known_issues.KNOWN) - ids)
        if stale:
            print("known_issues.py names cases that do not exist: %s" % ", ".join(stale))
            return 2
        for c in runner.CASES:
            if c["id"] in known_issues.KNOWN:
                c["known"], c["known_on"] = known_issues.KNOWN[c["id"]]
        cases = [c for c in runner.CASES
                 if (not a.group or any(c["group"].startswith(g) for g in a.group))
                 and (not a.only or a.only in c["id"])]
        if a.list:
            for c in cases:
                print("%-45s %-10s %s" % (c["id"], c["group"], c["known"] or ""))
            return 0
        print("battery: %d cases against %s" % (len(cases), env.BIN), flush=True)
        results = runner.run_all(cases, a.jobs)
    finally:
        shutil.rmtree(certdir, ignore_errors=True)

    by = collections.defaultdict(list)
    for r in results:
        by[runner.verdict(r)].append(r)
    if a.report:
        json.dump(results, open(a.report, "w"), indent=1, default=str)

    if by["fail"]:
        print("\nFAILED - not a known issue:")
        for r in by["fail"]:
            show(r, True)
    if by["xpass"]:
        print("\nA KNOWN ISSUE NO LONGER FAILS - good. Delete its line from test/battery/known_issues.py (the issue is closed when the fix is released):")
        for r in by["xpass"]:
            print("  %s  (%s)" % (r["id"], r["known"]))
    if by["xfail"]:
        issues = collections.Counter(r["known"] for r in by["xfail"])
        print("\nknown issues still failing (%d cases, %s):" % (len(by["xfail"]), ", ".join("%s x%d" % kv for kv in sorted(issues.items()))))
        if a.verbose:
            for r in by["xfail"]:
                show(r, True)
    if by["skip"]:
        why = collections.Counter(r["skipped"] for r in by["skip"])
        print("\nskipped %d cases: %s" % (len(by["skip"]), "; ".join("%s (%d)" % kv for kv in why.items())))
    bad = len(by["fail"]) + len(by["xpass"])
    print("\nbattery: %d cases: %d passed, %d known issues, %d skipped, %d FAILED" % (
        len(results), len(by["pass"]), len(by["xfail"]), len(by["skip"]), bad))
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
