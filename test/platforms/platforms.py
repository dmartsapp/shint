#!/usr/bin/env python3
"""Platform tests: one script per release binary, and the results file each one fills.

    python3 test/platforms/platforms.py generate          # write the scripts, and blank results files
    python3 test/platforms/platforms.py check             # scripts current, results files consistent
    python3 test/platforms/platforms.py summary [--verify]

Every platform the Makefile builds has a script here, made from template.sh (POSIX sh)
or template.ps1 (Windows PowerShell) with the platform's settings on top: what its
binary is called and what each platform-dependent command should do there. A tester
runs the script with the release's binary; it writes results/<os>-<arch>.txt: the
release and the binary's SHA-256, the system, and how many checks passed. `summary`
puts them side by side, and with --verify checks each binary's SHA-256 against the
one published with that release, so a result is known to be from that release's binary.
"""

import os
import re
import sys
import urllib.error
import urllib.request

sys.dont_write_bytecode = True

HERE = os.path.dirname(os.path.abspath(__file__))
TOP = os.path.dirname(os.path.dirname(HERE))
REPO = "dmartsapp/shint"

# What the platform-dependent commands do on each platform (and why):
#   ss:    yes = lists sockets (Linux /proc, Windows iphlpapi); no = "not supported yet", exit 1;
#          absent = not in the binary at all (AIX: main_ss.go is built !aix), exit 2
#   route: ip route and ip arp - yes on Linux, macOS, the BSDs and Windows; no elsewhere (exit 1)
#   ping:  yes = 127.0.0.1 must answer; sysctl = Linux kernels, where a normal user may ping
#          only if net.ipv4.ping_group_range allows it, so a permission error is a skip;
#          no = no unprivileged ICMP socket (#112), so ping must fail saying so (#111) -
#          seen on FreeBSD 14.4 and NetBSD 11.1, expected (not yet seen) on OpenBSD, Solaris,
#          illumos and AIX, whose own ping is setuid root or needs a privilege
PLATFORMS = [
    # os, arch, ss, route, ping
    ("aix", "ppc64", "absent", "no", "no"),
    ("android", "arm64", "yes", "yes", "sysctl"),
    ("darwin", "amd64", "no", "yes", "yes"),
    ("darwin", "arm64", "no", "yes", "yes"),
    ("freebsd", "amd64", "no", "yes", "no"),
    ("freebsd", "arm64", "no", "yes", "no"),
    ("illumos", "amd64", "no", "no", "no"),
    ("linux", "amd64", "yes", "yes", "sysctl"),
    ("linux", "arm", "yes", "yes", "sysctl"),
    ("linux", "arm64", "yes", "yes", "sysctl"),
    ("linux", "ppc64le", "yes", "yes", "sysctl"),
    ("netbsd", "amd64", "no", "yes", "no"),
    ("netbsd", "arm64", "no", "yes", "no"),
    ("openbsd", "amd64", "no", "yes", "no"),
    ("openbsd", "arm64", "no", "yes", "no"),
    ("solaris", "amd64", "no", "no", "no"),
    ("windows", "amd64", "yes", "yes", "yes"),
    ("windows", "arm64", "yes", "yes", "yes"),
]

FIELDS = ["platform", "release", "commit", "built", "go", "binary", "binary_sha256", "tested_at",
          "os", "privileges", "internet", "passed", "failed", "skipped", "result"]


def read(path, **kw):
    with open(path, encoding="utf-8", **kw) as f:
        return f.read()


class Platform:
    def __init__(self, goos, arch, ss, route, ping):
        self.goos, self.arch, self.ss, self.route, self.ping = goos, arch, ss, route, ping
        self.name = "%s/%s" % (goos, arch)
        self.slug = "%s-%s" % (goos, arch)
        self.windows = goos == "windows"
        self.asset = "shint.%s.%s%s" % (goos, arch, ".exe" if self.windows else "")
        self.script = self.slug + (".ps1" if self.windows else ".sh")
        self.results = os.path.join("results", self.slug + ".txt")


def platforms():
    return [Platform(*p) for p in PLATFORMS]


def makefile_platforms(top=TOP):
    """The os/arch pairs the Makefile builds (its GOOS=... GOARCH=... lines)."""
    text = read(os.path.join(top, "Makefile"))
    return sorted(set("%s/%s" % m for m in re.findall(r"GOOS=(\w+) GOARCH=(\w+)[^\n]*? go build", text)))


def render(p, here=HERE):
    if p.windows:
        run = "powershell -ExecutionPolicy Bypass -File .\\%s .\\%s" % (p.script, p.asset)
        settings = ["$Platform = '%s'" % p.name,
                    "$Slug = '%s'" % p.slug,
                    "$Asset = '%s'" % p.asset,
                    "$ExpectSS = '%s'      # ss: yes = works; no = says \"not supported yet\"; absent = not in this binary" % p.ss,
                    "$ExpectRoute = '%s'   # ip route and ip arp: yes = work; no = say \"not supported yet\"" % p.route,
                    "$ExpectPing = '%s'    # ping 127.0.0.1: yes = must answer; no = must say it cannot open an ICMP socket" % p.ping]
        template = "template.ps1"
    else:
        run = "sh %s ./%s" % (p.script, p.asset)
        settings = ["PLATFORM=%s" % p.name,
                    "SLUG=%s" % p.slug,
                    "ASSET=%s" % p.asset,
                    "EXPECT_SS=%s      # ss: yes = works; no = says \"not supported yet\"; absent = not in this binary" % p.ss,
                    "EXPECT_ROUTE=%s   # ip route and ip arp: yes = work; no = say \"not supported yet\"" % p.route,
                    "EXPECT_PING=%s    # ping 127.0.0.1: yes = must answer; sysctl = a permission error is a skip; no = must say it cannot open an ICMP socket" % p.ping]
        template = "template.sh"
    header = ["# shint platform test for %s - made by test/platforms/platforms.py from %s:" % (p.name, template),
              "# edit the template, not this file, then run `make platform-tests`.",
              "#",
              "# Run it on a %s machine with that release's binary, %s:" % (p.name, "in PowerShell" if p.windows else "in a shell"),
              "#   " + run,
              "# It writes %s next to itself. Send that file back:" % p.results.replace(os.sep, "/"),
              "# https://github.com/dmartsapp/shint/issues/new"]
    text = read(os.path.join(here, template))
    text = text.replace("# @@HEADER@@\n", "\n".join(header) + "\n", 1)
    text = text.replace("# @@SETTINGS@@\n", "\n".join(settings) + "\n", 1)
    return text


def blank_results(p):
    lines = ["# shint platform test: %s" % p.name,
             "# Blank until test/platforms/%s has been run on %s; the script rewrites this file." % (p.script, p.name),
             "platform: %s" % p.name]
    lines += ["%s:" % k for k in FIELDS[1:-1]]
    lines += ["result: untested"]
    return "\n".join(lines) + "\n"


def generate(here=HERE, log=print):
    for p in platforms():
        path = os.path.join(here, p.script)
        text = render(p, here)
        old = read(path, newline="") if os.path.exists(path) else None
        if old != text:
            with open(path, "w", newline="\n") as f:
                f.write(text)
            os.chmod(path, 0o755)
            log("wrote %s" % os.path.relpath(path, TOP))
        results = os.path.join(here, p.results)
        if not os.path.exists(results):   # never overwrite a tester's results
            os.makedirs(os.path.dirname(results), exist_ok=True)
            with open(results, "w", newline="\n") as f:
                f.write(blank_results(p))
            log("wrote %s" % os.path.relpath(results, TOP))


def parse_results(text):
    """A results file as (fields, check lines): the "key: value" header, then PASS/FAIL/SKIP lines."""
    if text.startswith("\ufeff"):
        text = text[1:]
    fields, checks = {}, []
    in_header = True
    for line in text.splitlines():
        if line.startswith("#"):
            continue
        if in_header:
            if not line.strip():
                in_header = False
                continue
            key, sep, value = line.partition(":")
            if sep:
                fields[key.strip()] = value.strip()
            continue
        m = re.match(r"^(PASS|FAIL|SKIP)  (.+)$", line)
        if m:
            checks.append((m.group(1), m.group(2)))
    return fields, checks


def validate_results(p, text):
    """What is wrong with a results file for platform p (a list of messages, empty when fine)."""
    fields, checks = parse_results(text)
    errors = []
    missing = [k for k in FIELDS if k not in fields]
    extra = [k for k in fields if k not in FIELDS]
    if missing:
        errors.append("missing %s" % ", ".join(missing))
    if extra:
        errors.append("unknown %s" % ", ".join(extra))
    if missing:
        return errors
    if fields["platform"] != p.name:
        errors.append("platform is %r, not %s" % (fields["platform"], p.name))
    result = fields["result"]
    if result == "untested":
        filled = [k for k in FIELDS if k not in ("platform", "result") and fields[k]]
        if filled or checks:
            errors.append("result is untested but it has results (%s)" % ", ".join(filled or ["check lines"]))
        return errors
    if result not in ("pass", "fail"):
        return errors + ["result is %r: pass, fail or untested" % result]
    if not fields["release"]:
        errors.append("no release")
    if not re.match(r"^([0-9a-f]{64}|unknown)$", fields["binary_sha256"]):
        errors.append("binary_sha256 is not a SHA-256")
    if not re.match(r"^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$", fields["tested_at"]):
        errors.append("tested_at is not a UTC time like 2026-11-20T10:00:00Z")
    counts = {}
    for key, status in (("passed", "PASS"), ("failed", "FAIL"), ("skipped", "SKIP")):
        if not fields[key].isdigit():
            errors.append("%s is not a number" % key)
            continue
        counts[key] = int(fields[key])
        lines = sum(1 for s, _ in checks if s == status)
        if counts[key] != lines:
            errors.append("%s says %d but there are %d %s lines" % (key, counts[key], lines, status))
    if "failed" in counts and (result == "pass") != (counts["failed"] == 0):
        errors.append("result is %s with %d failed" % (result, counts["failed"]))
    return errors


def check(here=HERE, top=TOP):
    """Every problem with the platform tests: a missing or stale script, a stray file, a results
    file that does not add up. Empty when all is well."""
    problems = []
    ours = sorted(p.name for p in platforms())
    built = makefile_platforms(top)
    for name in sorted(set(built) - set(ours)):
        problems.append("the Makefile builds %s, which has no platform test (add it to PLATFORMS)" % name)
    for name in sorted(set(ours) - set(built)):
        problems.append("%s has a platform test but the Makefile does not build it" % name)
    expected_scripts = set()
    expected_results = set()
    for p in platforms():
        expected_scripts.add(p.script)
        expected_results.add(os.path.basename(p.results))
        path = os.path.join(here, p.script)
        if not os.path.exists(path):
            problems.append("%s is missing (make platform-tests)" % p.script)
        elif read(path, newline="") != render(p, here):
            problems.append("%s is not what its template makes (make platform-tests)" % p.script)
        results = os.path.join(here, p.results)
        if not os.path.exists(results):
            problems.append("%s is missing (make platform-tests)" % p.results)
        else:
            for e in validate_results(p, read(results)):
                problems.append("%s: %s" % (p.results, e))
    for name in sorted(os.listdir(here)):
        if re.match(r"^[a-z0-9]+-[a-z0-9]+\.(sh|ps1)$", name) and name not in expected_scripts:
            problems.append("%s is not the script of any platform" % name)
    results_dir = os.path.join(here, "results")
    if os.path.isdir(results_dir):
        for name in sorted(os.listdir(results_dir)):
            if name.endswith(".txt") and name not in expected_results:
                problems.append("results/%s is not the results file of any platform" % name)
    return problems


def fetch_published_sha256(release, asset):
    url = "https://github.com/%s/releases/download/%s/%s.sha256" % (REPO, release, asset)
    with urllib.request.urlopen(url, timeout=20) as r:
        text = r.read().decode("utf-8", "replace")
    m = re.search(r"\b([0-9a-fA-F]{64})\b", text)
    return m.group(1).lower() if m else None


def provenance(p, fields, fetch):
    """Whether the tested binary is the one published with its release."""
    release, sha = fields.get("release", ""), fields.get("binary_sha256", "")
    if not re.match(r"^v\d+\.\d+\.\d+$", release):
        return "not a release build"
    if not re.match(r"^[0-9a-f]{64}$", sha):
        return "no SHA-256"
    try:
        published = fetch(release, p.asset)
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return "%s publishes no %s (not released yet?)" % (release, p.asset)
        return "could not fetch the published SHA-256 (%s)" % e
    except Exception as e:   # no network, say
        return "could not fetch the published SHA-256 (%s)" % e
    if published == sha:
        return "verified"
    return "MISMATCH: not %s's published binary" % release


def summary(here=HERE, verify=False, fetch=fetch_published_sha256, out=print):
    rows = []
    mismatches = 0
    for p in platforms():
        path = os.path.join(here, p.results)
        fields = {}
        if os.path.exists(path):
            fields, _ = parse_results(read(path))
        result = fields.get("result") or "untested"
        row = [p.name, fields.get("release") or "-", result]
        if result == "untested":
            row += ["-", "-"]
        else:
            row += ["%s/%s/%s" % (fields.get("passed"), fields.get("failed"), fields.get("skipped")),
                    (fields.get("tested_at") or "-")[:10]]
        if verify:
            if result == "untested":
                row.append("-")
            else:
                v = provenance(p, fields, fetch)
                mismatches += v.startswith("MISMATCH")
                row.append(v)
        rows.append(row)
    head = ["platform", "release", "result", "pass/fail/skip", "tested"] + (["binary"] if verify else [])
    widths = [max(len(str(r[i])) for r in rows + [head]) for i in range(len(head))]
    for r in [head] + rows:
        out("  ".join(str(c).ljust(w) for c, w in zip(r, widths)).rstrip())
    tested = sum(1 for r in rows if r[2] != "untested")
    out("")
    out("%d of %d platforms tested, %d passed" % (tested, len(rows), sum(1 for r in rows if r[2] == "pass")))
    return 1 if mismatches else 0


def main(argv):
    if len(argv) < 2 or argv[1] not in ("generate", "check", "summary"):
        print(__doc__.strip().split("\n\n")[0], file=sys.stderr)
        return 2
    if argv[1] == "generate":
        generate()
        return 0
    if argv[1] == "check":
        problems = check()
        for problem in problems:
            print("platform tests: " + problem, file=sys.stderr)
        if not problems:
            print("platform tests: %d scripts current, results files consistent" % len(PLATFORMS))
        return 1 if problems else 0
    return summary(verify="--verify" in argv[2:])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
