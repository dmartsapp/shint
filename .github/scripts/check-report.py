#!/usr/bin/env python3
"""Build, sign and verify the report of a local `make check` run.

    check-report.py build  --log LOG --out REPORT        the report, from the output of make check
    check-report.py sign   --report REPORT --key KEY --out NOTE   report + SSH signature = the git note
    check-report.py verify --note NOTE --allowed-signers FILE --tree TREE [--max-age-days N]

The point: CI on main runs a short subset instead of the whole suite when the whole suite
has already passed on the maintainer's machine, and that has to be something CI can check
and a forger cannot produce. So the machine that ran the suite writes down what ran (the
report), signs those exact bytes with an SSH key (`ssh-keygen -Y sign`), and stores both as a
git note on the commit (refs/notes/checks). CI verifies the signature against a list of
allowed public keys, and that the report is for the tree that was pushed, complete, passing
and recent. Anything else is "no report", and CI then runs the whole suite: a missing or bad
report costs time, never coverage.

The note is the report's bytes followed by the armored SSH signature, so verifying is
"everything before the signature block is what was signed" - no canonicalisation to get wrong.

What a signature proves: that the holder of the key stands behind this report for this
tree. It does not prove the tests were run honestly; `make attest` is what runs them, and
CI still reruns the vulnerability check itself.

Standard library only, plus ssh-keygen (OpenSSH 8.0 or newer, on every runner and Mac).
Exit status: 0 done / valid, 1 not valid (the reason is printed), 2 usage or input error.
"""
import argparse
import datetime
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

NAMESPACE = "shint-check"
SCHEMA = 1
BEGIN_SIG = "-----BEGIN SSH SIGNATURE-----"

# What `make check` must have run for a report to count. Each is the text after "==> " in
# its output. A report that lacks one is refused, so a shortened run cannot be passed off.
REQUIRED_STAGES = ["gofmt", "go vet", "go test -race", "black-box battery", "golangci-lint",
                   "govulncheck", "documentation", "workflows"]
DONE_LINE = "==> all checks passed"


class Bad(Exception):
    """The input is not what it should be; the message says why."""


def now():
    return datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0)


def git(*args):
    r = subprocess.run(["git"] + list(args), capture_output=True, text=True)
    if r.returncode != 0:
        raise Bad("git %s failed: %s" % (" ".join(args), r.stderr.strip()[:200]))
    return r.stdout.strip()


def tool_version(cmd):
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=20)
    except (OSError, subprocess.TimeoutExpired):
        return None
    text = (r.stdout + r.stderr).strip().splitlines()
    return text[0].strip()[:100] if text else None


# --- build ----------------------------------------------------------------------------

def parse_log(text):
    """The stages that ran, in order, with what can be said about each, from make check's output."""
    if DONE_LINE not in text:
        raise Bad("the log does not end in '%s': make check did not pass" % DONE_LINE)
    stages, current = [], None
    for line in text.splitlines():
        m = re.match(r"^==> (.+?)\s*$", line)
        if m and m.group(1) != DONE_LINE[4:]:
            current = {"name": m.group(1), "ok": True}
            stages.append(current)
            continue
        if current is None:
            continue
        if current["name"] == "black-box battery":
            b = re.match(r"^battery: (\d+) cases: (\d+) passed, (\d+) known issues, (\d+) skipped, (\d+) FAILED", line)
            if b:
                current["detail"] = "%s cases: %s passed, %s known issues, %s skipped, %s failed" % b.groups()
                if b.group(5) != "0":
                    current["ok"] = False
        elif current["name"] == "go test -race":
            if re.match(r"^(FAIL|---\s*FAIL)", line):
                current["ok"] = False
            elif line.startswith("ok  "):
                current["detail"] = "%d packages ok" % (int(current.get("detail", "0 ").split()[0]) + 1)
        elif current["name"] == "govulncheck" and "No vulnerabilities found" in line:
            current["detail"] = "No vulnerabilities found"
    return stages


def build_report(log_text, commit=None, tree=None, when=None):
    stages = parse_log(log_text)
    seen = [s["name"] for s in stages]
    missing = [n for n in REQUIRED_STAGES if n not in seen]
    if missing:
        raise Bad("the log lacks stage(s): %s" % ", ".join(missing))
    failed = [s["name"] for s in stages if not s["ok"]]
    if failed:
        raise Bad("stage(s) failed: %s" % ", ".join(failed))
    uname = os.uname()
    return {
        "schema": SCHEMA,
        "commit": commit or git("rev-parse", "HEAD"),
        "tree": tree or git("rev-parse", "HEAD^{tree}"),
        "created": (when or now()).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "host": {"os": uname.sysname.lower(), "arch": uname.machine},
        "tools": {k: v for k, v in {
            "go": tool_version(["go", "version"]),
            "golangci-lint": tool_version(["golangci-lint", "--version"]),
            "govulncheck": tool_version(["govulncheck", "-version"]),
            "actionlint": tool_version(["actionlint", "-version"]),
            "python": "python " + sys.version.split()[0],
        }.items() if v},
        "result": "pass",
        "stages": stages,
        "log_sha256": hashlib.sha256(log_text.encode()).hexdigest(),
    }


def report_bytes(report):
    """The one way the report is written, so the signed bytes are what anyone re-reads."""
    return (json.dumps(report, indent=2, sort_keys=True) + "\n").encode()


# --- sign -----------------------------------------------------------------------------

def sign(report, key):
    """The note: the report's bytes, then the armored signature over exactly those bytes."""
    if shutil.which("ssh-keygen") is None:
        raise Bad("ssh-keygen is not installed")
    with tempfile.TemporaryDirectory() as d:
        path = os.path.join(d, "report.json")
        with open(path, "wb") as f:
            f.write(report)
        r = subprocess.run(["ssh-keygen", "-Y", "sign", "-f", key, "-n", NAMESPACE, path], capture_output=True, text=True)
        if r.returncode != 0:
            raise Bad("ssh-keygen could not sign (is the key right, and unlocked or in ssh-agent?): %s" % r.stderr.strip()[:200])
        with open(path + ".sig", "rb") as f:
            return report + f.read()


# --- verify ---------------------------------------------------------------------------

def split_note(note):
    i = note.find(BEGIN_SIG.encode())
    if i <= 0:
        raise Bad("the note has no signature")
    return note[:i], note[i:]


def verify(note, allowed_signers, tree, max_age_days=14, at=None):
    """Return a one-line description of the valid report, or raise Bad with the reason."""
    if shutil.which("ssh-keygen") is None:
        raise Bad("ssh-keygen is not installed")
    report_raw, sig = split_note(note)
    with tempfile.TemporaryDirectory() as d:
        sigfile = os.path.join(d, "report.sig")
        with open(sigfile, "wb") as f:
            f.write(sig)
        found = subprocess.run(["ssh-keygen", "-Y", "find-principals", "-f", allowed_signers, "-s", sigfile], capture_output=True, text=True)
        principals = [p for p in found.stdout.split() if p]
        if found.returncode != 0 or not principals:
            raise Bad("the signing key is not in the allowed signers")
        principal = principals[0]
        r = subprocess.run(["ssh-keygen", "-Y", "verify", "-f", allowed_signers, "-I", principal, "-n", NAMESPACE, "-s", sigfile],
                           input=report_raw, capture_output=True)
        if r.returncode != 0:
            raise Bad("the signature does not match the report")
    try:
        report = json.loads(report_raw)
    except ValueError:
        raise Bad("the signed report is not JSON")
    if report.get("schema") != SCHEMA:
        raise Bad("the report's schema is %r, expected %d" % (report.get("schema"), SCHEMA))
    if report.get("result") != "pass":
        raise Bad("the report's result is %r, not pass" % report.get("result"))
    if report.get("tree") != tree:
        raise Bad("the report is for tree %s, not %s" % (str(report.get("tree"))[:12], tree[:12]))
    names = {s.get("name"): s for s in report.get("stages", [])}
    missing = [n for n in REQUIRED_STAGES if n not in names]
    if missing:
        raise Bad("the report lacks stage(s): %s" % ", ".join(missing))
    bad = [n for n in REQUIRED_STAGES if not names[n].get("ok")]
    if bad:
        raise Bad("stage(s) not ok: %s" % ", ".join(bad))
    try:
        created = datetime.datetime.strptime(report["created"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc)
    except (KeyError, ValueError):
        raise Bad("the report has no valid creation time")
    clock = at or now()
    if created > clock + datetime.timedelta(hours=1):
        raise Bad("the report is dated in the future (%s)" % report["created"])
    if clock - created > datetime.timedelta(days=max_age_days):
        raise Bad("the report is older than %d days (%s)" % (max_age_days, report["created"]))
    host = report.get("host", {})
    return "signed by %s on %s/%s at %s, tree %s" % (principal, host.get("os", "?"), host.get("arch", "?"), report["created"], tree[:12])


# --- command line ---------------------------------------------------------------------

def read(path, mode="r"):
    try:
        with open(path, mode) as f:
            return f.read()
    except OSError as e:
        raise Bad("cannot read %s: %s" % (path, e.strerror))


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    b = sub.add_parser("build"); b.add_argument("--log", required=True); b.add_argument("--out", required=True)
    b.add_argument("--commit"); b.add_argument("--tree")
    s = sub.add_parser("sign"); s.add_argument("--report", required=True); s.add_argument("--key", required=True); s.add_argument("--out", required=True)
    v = sub.add_parser("verify"); v.add_argument("--note", required=True); v.add_argument("--allowed-signers", required=True)
    v.add_argument("--tree", required=True); v.add_argument("--max-age-days", type=int, default=14)
    a = ap.parse_args(argv)
    try:
        if a.cmd == "build":
            with open(a.out, "wb") as f:
                f.write(report_bytes(build_report(read(a.log), a.commit, a.tree)))
            print("check-report: report written to %s" % a.out)
        elif a.cmd == "sign":
            with open(a.out, "wb") as f:
                f.write(sign(read(a.report, "rb"), a.key))
            print("check-report: signed note written to %s" % a.out)
        else:
            print("check-report: valid: " + verify(read(a.note, "rb"), a.allowed_signers, a.tree, a.max_age_days))
    except Bad as e:
        print("check-report: %s" % e, file=sys.stderr)
        return 1 if a.cmd == "verify" else 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
