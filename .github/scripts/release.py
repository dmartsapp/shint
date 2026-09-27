#!/usr/bin/env python3
"""Start and publish a shint release, each with one command.

    make release-start VERSION=4.4.0    python3 .github/scripts/release.py start 4.4.0
    make release                        python3 .github/scripts/release.py
    make release DRY_RUN=1              python3 .github/scripts/release.py --dry-run

`start` (on a clean checkout): points main's README roadmap row for the release at the
branch's working page (adding an "In progress" row if it has none) and pushes main; then
creates release/vX.Y.Z from it with "## vX.Y.Z - unreleased" in CHANGELOG.md and the working
page branch_readme.md, commits and pushes. The working page is where the release's scope,
bugs, changes and to-dos are kept; its "Summary:" line is what main's README roadmap will
say about the release.

Without a subcommand, on release/vX.Y.Z, on release day:

  prepare - local, nothing pushed
    1. preflight: a clean tree, the tag free, the changelog section has entries,
       branch_readme.md has a Summary and nothing unchecked under "To do"; the
       milestone's open issues are listed
    2. origin/main merged in if it moved (a conflict stops here)
    3. stamped: branch_readme.md moved to releases/vX.Y.Z.md (the record on main); the
       version from the branch name and today's date from the clock - never typed - into
       main.go, the changelog heading and the page's title; main's README roadmap row from
       the Summary (readme-reconcile.py --summary), linking to the record
    4. make profile TAG=vX.Y.Z on the stamped tree: the release's runtime stats and profiles,
       and the machine they were measured on, in .profiling/vX.Y.Z/<time>/ (test/profile); then
       the site rebuilt, its Performance page drawing .profiling/ with this release in it
    5. the release commit, its message generated from the changelog, the profile in it
    6. make attest (make check, signed, on this exact commit), make test-live, and
       release-check; a failure undoes the release commit and stops
  "Publish vX.Y.Z? [y/N]"
  publish
    7. main fast-forwarded to the release commit and pushed; then the wait for Check on
       that commit - a red Check stops here, and nothing is tagged
    8. the annotated tag (its message from the changelog), pushed: the release pipeline
    9. the release workflows watched, the assets counted, this machine's binary downloaded
       and its checksum, attestation and --version checked, the milestone closed

Every step recognises work already done, so after a failure (or a "no" at the prompt) the
same command continues where it stopped. --dry-run runs the prepare stage in a throw-away
clone with pushing disabled, and leaves the clone for you to look at.

Environment, all optional: NO_ATTEST=1 (make check instead of make attest: Check on main
then runs the whole suite), SKIP_LIVE=1 (no make test-live), RELEASE_YES=1 (answer the
prompt yes). For the tests only: RELEASE_TODAY, RELEASE_CHECK_CMD, RELEASE_LIVE_CMD,
RELEASE_DOCS_CMD, RELEASE_PROFILE_CMD ({tag} is replaced), RELEASE_POLL, RELEASE_VERIFY_BINARY=0, GH.

Standard library only, like docs/build.py.
"""
import datetime
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import time

# Importing readme-reconcile.py (for its Roadmap table) must not leave a __pycache__ in the
# working tree: a Python without a cache prefix (Linux, Homebrew) writes one next to the
# source, and an untracked file makes the tree "dirty" for the next step.
sys.dont_write_bytecode = True

REPO = os.environ.get("GH_REPO", "dmartsapp/shint")
TAG_RE = re.compile(r"^v\d+\.\d+\.\d+$")
VERSION_RE = re.compile(r"^\d+\.\d+\.\d+$")
MARKER = "Full changelog for v%s"          # release-check.sh and the GitHub release look for it


class Stop(Exception):
    """A reason to stop, written for the person running the release."""


# --- running things -------------------------------------------------------------------

def run(*args, check=True, quiet=True, env=None):
    """Run a command; return its stdout. quiet=False streams the output instead."""
    full_env = dict(os.environ, **(env or {}))
    if quiet:
        r = subprocess.run(args, capture_output=True, text=True, env=full_env)
    else:
        r = subprocess.run(args, text=True, env=full_env)
    if check and r.returncode != 0:
        detail = ((r.stderr or "") + (r.stdout or "")).strip() if quiet else ""
        raise Stop("`%s` failed (exit %d)%s" % (" ".join(args), r.returncode, (": " + detail[:400]) if detail else ""))
    return (r.stdout or "").strip() if quiet else ""


def ok(*args):
    return subprocess.run(args, capture_output=True, text=True).returncode == 0


def git(*args, **kw):
    return run("git", *args, **kw)


def gh(*args, **kw):
    return run(os.environ.get("GH", "gh"), *args, **kw)


def gh_json(*args, default=None):
    try:
        return json.loads(gh(*args) or "null")
    except (Stop, ValueError):
        return default


def shell(cmd):
    """A make target or a test hook; its output goes straight to the terminal."""
    print("  $ " + cmd, flush=True)
    return subprocess.run(cmd, shell=True).returncode == 0


def step(text):
    print("\n==> " + text, flush=True)


def say(text):
    print("  " + text, flush=True)


def poll_seconds():
    return float(os.environ.get("RELEASE_POLL", "10"))


# --- what the repository says ------------------------------------------------------------

def read(path):
    with open(path) as f:
        return f.read()


def write(path, text):
    with open(path, "w") as f:
        f.write(text)


def branch_version():
    branch = git("rev-parse", "--abbrev-ref", "HEAD")
    m = re.match(r"^release/v(\d+\.\d+\.\d+)$", branch)
    if not m:
        raise Stop("run this on a release branch (release/vX.Y.Z); this is '%s'" % branch)
    return m.group(1)


def changelog_section(version, text=None):
    """(the heading's date or 'unreleased', the section's body) - or (None, None)."""
    text = read("CHANGELOG.md") if text is None else text
    m = re.search(r"^## v%s - (.+)$" % re.escape(version), text, re.M)
    if not m:
        return None, None
    rest = text[m.end():]
    nxt = re.search(r"^## ", rest, re.M)
    return m.group(1).strip(), (rest[:nxt.start()] if nxt else rest).strip("\n")


def bullets(body):
    """The first bold phrase of every entry, or its first 90 characters."""
    out = []
    for line in body.splitlines():
        if line.startswith("- "):
            m = re.search(r"\*\*(.+?)\*\*", line)
            out.append((m.group(1) if m else line[2:])[:90])
    return out


def working_page(version):
    """(path, summary, includes, unchecked to-dos). The page is branch_readme.md while the
    release is in flight; the release commit moves it to releases/vX.Y.Z.md (which is also
    where a branch started before branch_readme.md keeps it)."""
    path = "branch_readme.md"
    if not os.path.exists(path):
        path = "releases/v%s.md" % version
    if not os.path.exists(path):
        raise Stop("branch_readme.md is missing: it is the branch's working page, and its Summary line is main's "
                   "README roadmap row (make release-start creates it)")
    text = read(path)
    m = re.search(r"^Summary:[ \t]*(.*)$", text, re.M)
    summary = m.group(1).strip() if m else ""
    if not summary:
        raise Stop("%s has no 'Summary:' line - one plain-language sentence for main's README "
                   "roadmap, like: Summary: `tls` checks certificates, and `nmap` takes port lists" % path)
    m = re.search(r"^Includes:[ \t]*(.*)$", text, re.M)
    includes = [t for t in re.split(r"[,\s]+", m.group(1).strip()) if t] if m else []
    for t in includes:
        if not TAG_RE.match(t) or t == "v" + version:
            raise Stop("%s: 'Includes:' lists other releases folded into this one, like v4.4.0; got %r" % (path, t))
    todo = re.search(r"^## To do[ \t]*$(.*?)(?=^## |\Z)", text, re.M | re.S)
    unchecked = re.findall(r"^\s*[-*] \[ \] (.+)$", todo.group(1), re.M) if todo else []
    return path, summary, includes, unchecked


def is_release_commit(version, ref="HEAD"):
    return MARKER % version in git("log", "-1", "--format=%B", ref)


def is_ancestor(a, b):
    return ok("git", "merge-base", "--is-ancestor", a, b)


def remote_tag(tag):
    return git("ls-remote", "--tags", "origin", "refs/tags/" + tag) != ""


def local_tag(tag):
    return ok("git", "rev-parse", "-q", "--verify", "refs/tags/" + tag)


def open_issues(milestones):
    found = []
    for title in milestones:
        for i in gh_json("issue", "list", "--repo", REPO, "--milestone", title, "--state", "open",
                         "--json", "number,title", default=[]) or []:
            found.append("#%s %s (%s)" % (i["number"], i["title"], title))
    return found


# --- start --------------------------------------------------------------------------------

PAGE = """# shint {tag} - work in progress

Summary:
Includes:

> The working page for `release/{tag}`: what the release is meant to deliver, the bugs it fixes, what changed, and what is left to do - kept up to date as the branch moves. **`Summary:`** is the one plain-language line main's README roadmap will show for this release; **`Includes:`** names releases folded into this one (like `v4.4.0`), if any. `make release` reads both, refuses to publish while anything under **To do** is unchecked, and moves this file to `releases/{tag}.md` in the release commit, where it stays on `main` as the release's record. Main's roadmap row links here in the meantime.

| | |
|---|---|
| Milestone | {milestone} |
| Built on | `main`, after the {previous} tag |
| Everything that differs from main | [main...release/{tag}](https://github.com/{repo}/compare/main...release/{tag}) |

## To do

- [ ] every target and bug below is done, or moved to a later milestone
- [ ] the changelog has an entry for every change a user would notice
- [ ] `Summary:` above says what this release is, in plain language

## Targets

| Target | Issue | State |
|---|---|---|

## Bugs

| Issue | State |
|---|---|

## Other changes
"""


BRANCH_PAGE = "branch_readme.md"


def reconcile_module():
    """readme-reconcile.py, imported: its Roadmap table reader."""
    import importlib.util
    spec = importlib.util.spec_from_file_location("readme_reconcile", ".github/scripts/readme-reconcile.py")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def link_row(tag, target, new_text=None):
    """Point main's roadmap row for tag at target: [**vX.Y.Z**](target). With new_text, a
    release that has no row gets one ('In progress'), in version order. Returns True if
    readme.md changed."""
    rr = reconcile_module()
    text = read("readme.md")
    lines = text.split("\n")
    table = rr.Table(lines, "## Roadmap")
    version = tuple(int(x) for x in tag[1:].split("."))
    cell = "[**%s**](%s)" % (tag, target)
    row = next((r for r in table.rows if rr.version_of(r[0]) == version), None)
    if row is None:
        if new_text is None:
            return False
        key = lambda v: rr.LATER if v is None else v
        pos = next((i for i, r in enumerate(table.rows) if key(rr.version_of(r[0])) > version), len(table.rows))
        table.rows.insert(pos, [cell, "In progress", new_text])
    elif row[0] == cell:
        return False
    else:
        row[0] = cell
    write("readme.md", "\n".join(table.render(lines)))
    return True


def start(version):
    if not VERSION_RE.match(version):
        raise Stop("VERSION must look like 4.4.0, got %r" % version)
    tag, branch = "v" + version, "release/v" + version
    dirty = git("status", "--porcelain")
    if dirty:
        raise Stop("the working tree has uncommitted changes:\n%s" % "\n".join("    " + l for l in dirty.splitlines()[:10]))
    step("start %s" % branch)
    git("fetch", "-q", "origin", "--tags")
    if ok("git", "rev-parse", "-q", "--verify", "refs/heads/" + branch) or git("ls-remote", "--heads", "origin", branch):
        raise Stop("%s already exists" % branch)
    if local_tag(tag) or remote_tag(tag):
        raise Stop("the tag %s already exists" % tag)

    # main's README: the release's roadmap row links to the branch's working page
    git("switch", "-q", "--detach", "origin/main")
    page_url = "https://github.com/%s/blob/%s/%s" % (REPO, branch, BRANCH_PAGE)
    if link_row(tag, page_url, new_text="In progress - see its working page."):
        git("add", "readme.md")
        git("commit", "-q", "-m", "docs: README - %s's roadmap row links to its working page" % tag)
        git("push", "-q", "origin", "HEAD:refs/heads/main")
        if ok("git", "merge-base", "--is-ancestor", "refs/heads/main", "HEAD"):
            git("update-ref", "refs/heads/main", git("rev-parse", "HEAD"))
        say("main's README: the %s row links to %s" % (tag, page_url))
    git("switch", "-q", "-c", branch)

    text = read("CHANGELOG.md")
    if changelog_section(version, text)[0] is None:
        m = re.search(r"^## v", text, re.M)
        at = m.start() if m else len(text)
        write("CHANGELOG.md", text[:at] + "## %s - unreleased\n\n" % tag + text[at:])
    tags = sorted((t for t in git("tag", "--list", "v*").split() if TAG_RE.match(t)),
                  key=lambda t: tuple(int(x) for x in t[1:].split(".")))
    previous = tags[-1] if tags else "last"
    ms = next((m for m in gh_json("api", "repos/%s/milestones?state=open&per_page=100" % REPO, default=[]) or []
               if m.get("title") == tag), None)
    if ms:
        due = (" - due %s" % ms["due_on"][:10]) if ms.get("due_on") else " - no due date"
        milestone = "[%s](https://github.com/%s/milestone/%s)%s" % (tag, REPO, ms["number"], due)
    else:
        milestone = "%s (no milestone found - create it: gh api repos/%s/milestones -f title=%s)" % (tag, REPO, tag)
    if not os.path.exists(BRANCH_PAGE):
        write(BRANCH_PAGE, PAGE.format(tag=tag, milestone=milestone, previous=previous, repo=REPO))
    git("add", "CHANGELOG.md", BRANCH_PAGE)
    git("commit", "-q", "-m", "release: start %s" % tag)
    git("push", "-q", "-u", "origin", branch)
    say("created and pushed %s: keep %s (Summary, To do, targets, bugs) up to date as the sprint goes," % (branch, BRANCH_PAGE))
    say("and the '## %s - unreleased' section of CHANGELOG.md; on release day, run: make release" % tag)


# --- prepare ------------------------------------------------------------------------------

def today():
    fixed = os.environ.get("RELEASE_TODAY")
    if fixed:
        return datetime.date.fromisoformat(fixed).isoformat()
    return datetime.date.today().isoformat()


def stamp(path, pattern, replacement, what):
    text = read(path)
    new, n = re.subn(pattern, replacement, text, count=1, flags=re.M)
    if n != 1:
        raise Stop("could not stamp %s in %s" % (what, path))
    write(path, new)


def tags_with_dates(tag, day):
    """The lines readme-reconcile.py --tags-file reads: every tag's date, plus this one's."""
    out = git("for-each-ref", "--format=%(refname:short) %(taggerdate:short) %(creatordate:short)", "refs/tags")
    lines = []
    for line in out.splitlines():
        parts = line.split()
        if parts and TAG_RE.match(parts[0]) and parts[0] != tag:
            dates = [p for p in parts[1:] if re.match(r"\d{4}-\d{2}-\d{2}$", p)]
            if dates:
                lines.append("%s %s" % (parts[0], dates[0]))
    lines.append("%s %s" % (tag, day))
    return "\n".join(lines) + "\n"


def prepare(version, summary, includes, page, day):
    tag = "v" + version
    step("main")
    if is_ancestor("origin/main", "HEAD"):
        say("the branch is on top of origin/main")
    else:
        r = subprocess.run(["git", "merge", "--no-edit", "origin/main"], capture_output=True, text=True)
        if r.returncode != 0:
            conflicts = git("diff", "--name-only", "--diff-filter=U", check=False)
            subprocess.run(["git", "merge", "--abort"], capture_output=True)
            raise Stop("origin/main has moved and does not merge cleanly (%s). Merge it by hand - git merge "
                       "origin/main, resolve, commit - then run make release again."
                       % (", ".join(conflicts.split()) or r.stderr.strip()[:200]))
        say("origin/main had moved: merged it in (%s)" % git("rev-parse", "--short", "HEAD"))
    base = git("rev-parse", "HEAD")

    step("stamp %s, dated %s" % (tag, day))
    stamp("main.go", r'^(\s*Version\s+string\s*=\s*")[^"]*(")', r"\g<1>%s\g<2>" % version, "the Version")
    say("main.go: Version = \"%s\"" % version)
    stamp("CHANGELOG.md", r"^## %s - unreleased$" % re.escape(tag), "## %s - %s" % (tag, day), "the '%s - unreleased' heading" % tag)
    say("CHANGELOG.md: ## %s - %s" % (tag, day))
    record = "releases/%s.md" % tag
    if page != record:
        os.makedirs("releases", exist_ok=True)
        git("mv", page, record)
        say("%s -> %s (the release's record on main)" % (page, record))
        page = record
    stamp(page, r"^# .*$", "# shint %s - released %s" % (tag, day), "the title")
    say("%s: released %s" % (page, day))

    work = tempfile.mkdtemp(prefix="shint-release-")
    try:
        tags_file = os.path.join(work, "tags.txt")
        write(tags_file, tags_with_dates(tag, day))
        args = ["python3", ".github/scripts/readme-reconcile.py", "--tags-file", tags_file,
                "--tag", tag, "--summary", summary, "--report", os.path.join(work, "report.md")]
        if includes:
            args += ["--folded", ",".join(includes)]
        milestones = gh_json("api", "repos/%s/milestones?state=all&per_page=100" % REPO)
        if milestones is not None:
            write(os.path.join(work, "milestones.json"), json.dumps(milestones))
            args += ["--milestones", os.path.join(work, "milestones.json")]
        report = run(*args)
    finally:
        shutil.rmtree(work, ignore_errors=True)
    for line in report.splitlines():
        if line.startswith("- "):
            say("readme.md " + line)
    if link_row(tag, "releases/%s.md" % tag):
        say("readme.md: the %s row links to releases/%s.md" % (tag, tag))

    profile(tag, base)                         # before the site: its Performance page draws .profiling/

    docs = os.environ.get("RELEASE_DOCS_CMD", "python3 docs/build.py")
    if not shell(docs):
        raise Stop("rebuilding the site failed (%s)" % docs)

    step("the release commit")
    _, body = changelog_section(version)
    git("add", "-A")
    message = "release: %s (%s)\n\n%s\n%s\n%s\n%s\n\n%s\n" % (
        summary, tag, summary,
        ("\nIncludes %s, folded into this release.\n" % ", ".join(includes)) if includes else "",
        MARKER % version, "=" * len(MARKER % version), body)
    commit(message)
    say("%s %s" % (git("rev-parse", "--short", "HEAD"), git("log", "-1", "--format=%s")))
    return base


def profile(tag, base):
    """make profile TAG=vX.Y.Z on the stamped tree, so the release commit carries its runtime stats.
    The profile measures the tree it is about to be committed from: a profile cannot be part of
    the commit it measured, so it runs just before. A failure undoes the stamping."""
    step("profile %s" % tag)
    where = os.path.join(".profiling", tag)
    existed = os.path.isdir(where)
    cmd = os.environ.get("RELEASE_PROFILE_CMD", "make profile TAG={tag}").replace("{tag}", tag)

    def undo(why):
        git("reset", "-q", "--hard", base)
        if not existed:
            shutil.rmtree(where, ignore_errors=True)
        raise Stop("%s. The stamping was undone (the branch is at %s, as before); fix it and run make release "
                   "again." % (why, base[:10]))

    if not shell(cmd):
        undo("`%s` failed" % cmd)
    runs = sorted(d for d in (os.listdir(where) if os.path.isdir(where) else []) if os.path.isfile(os.path.join(where, d, "summary.json")))
    if not runs:
        undo("`%s` wrote no run under %s/" % (cmd, where))
    say("%s/%s: this release's runtime stats, going into the release commit" % (where, runs[-1]))


def commit(message):
    r = subprocess.run(["git", "commit", "-q", "-F", "-"], input=message, text=True, capture_output=True)
    if r.returncode != 0:
        raise Stop("git commit failed: " + (r.stderr or r.stdout).strip()[:300])


def verify_locally(version, base, dry_run):
    """The checks, on the release commit. A failure takes the release commit back off."""
    def undo(why):
        if base:
            git("reset", "-q", "--hard", base)
            raise Stop("%s. The release commit was taken back off the branch (it is now at %s, as before "
                       "the stamping); fix it on the branch, commit, and run make release again." % (why, base[:10]))
        raise Stop(why + ". Fix it on the branch, commit, and run make release again.")

    step("checks on the release commit")
    if "RELEASE_CHECK_CMD" in os.environ:
        check = os.environ["RELEASE_CHECK_CMD"]
    else:
        check = "make check" if dry_run or os.environ.get("NO_ATTEST") == "1" else "make attest"
    if not shell(check):
        undo("`%s` failed" % check)
    if os.environ.get("SKIP_LIVE") != "1":
        live = os.environ.get("RELEASE_LIVE_CMD", "make test-live")
        if not shell(live):
            undo("`%s` failed" % live)
    env = {"NO_FETCH": "1"} if dry_run else {}
    r = subprocess.run(["bash", ".github/scripts/release-check.sh"], env=dict(os.environ, **env))
    if r.returncode != 0:
        undo("release-check failed")


# --- publish ------------------------------------------------------------------------------

def ask(tag):
    if os.environ.get("RELEASE_YES") == "1":
        return True
    try:
        with open("/dev/tty") as tty:
            print("\nPublish %s? This pushes main and then the tag, which starts the release pipeline "
                  "and cannot be taken back. [y/N] " % tag, end="", flush=True)
            return tty.readline().strip().lower() in ("y", "yes")
    except OSError:
        raise Stop("no terminal to ask 'Publish %s?' on - run make release in a terminal (or RELEASE_YES=1)" % tag)


def summary_card(version, summary, includes, issues):
    tag = "v" + version
    day, body = changelog_section(version)
    step("ready to publish %s" % tag)
    say("commit    %s %s" % (git("rev-parse", "--short", "HEAD"), git("log", "-1", "--format=%s")))
    say("date      %s" % day)
    say("summary   %s" % summary)
    if includes:
        say("includes  %s (their roadmap rows say 'Shipped in %s')" % (", ".join(includes), tag))
    row = next((l for l in read("readme.md").splitlines()
                if re.match(r"^\| \[?\*\*%s\*\*(\]\([^)]*\))? \|" % re.escape(tag), l)), "(none)")
    say("README    %s" % row)
    say("changes   %s" % git("diff", "--shortstat", "origin/main", "HEAD"))
    for b in bullets(body):
        say("  - " + b)
    for i in issues:
        say("OPEN ISSUE on the milestone: " + i)


def wait_for(what, fetch, done, timeout):
    """Poll fetch() until done(result) or the timeout; returns the last result."""
    deadline = time.time() + timeout
    result = fetch()
    while not done(result):
        if time.time() > deadline:
            raise Stop("gave up waiting for %s after %d minutes - look at GitHub Actions, then run make release again"
                       % (what, timeout // 60))
        time.sleep(poll_seconds())
        result = fetch()
    return result


def check_on_main(sha):
    fetch = lambda: gh_json("run", "list", "--repo", REPO, "--workflow", "check.yaml", "--commit", sha,
                            "--json", "databaseId,status,conclusion,url", "--limit", "1", default=[]) or []
    runs = wait_for("the Check run on main", fetch, lambda r: bool(r), 300)
    say("Check started: %s" % runs[0].get("url", ""))
    runs = wait_for("Check to finish", fetch, lambda r: r and r[0].get("status") == "completed", 3600)
    return runs[0]


def release_runs(tag):
    fetch = lambda: gh_json("run", "list", "--repo", REPO, "--branch", tag, "--limit", "20",
                            "--json", "databaseId,workflowName,status,conclusion,url", default=[]) or []
    seen = {"n": -1, "stable": 0}

    def settled(runs):
        # every run has finished, and no new one has appeared for two polls in a row
        if not runs or any(r.get("status") != "completed" for r in runs) or len(runs) < 5:
            seen["stable"] = 0
        elif len(runs) == seen["n"]:
            seen["stable"] += 1
        seen["n"] = len(runs)
        return seen["stable"] >= 2

    return wait_for("the release workflows", fetch, settled, 3600)


def verify_binary(tag, assets):
    osname = platform.system().lower()
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    name = "shint.%s.%s" % (osname, arch)
    if name not in assets or name + ".sha256" not in assets:
        say("no %s asset for this machine; skipped the binary check" % name)
        return True
    work = tempfile.mkdtemp(prefix="shint-verify-")
    try:
        gh("release", "download", tag, "--repo", REPO, "--dir", work, "--pattern", name, "--pattern", name + ".sha256")
        binary = os.path.join(work, name)
        want = read(binary + ".sha256").split()[0]
        with open(binary, "rb") as f:
            got = hashlib.sha256(f.read()).hexdigest()
        if got != want:
            say("FAIL %s: sha256 %s, published %s" % (name, got, want))
            return False
        say("%s: sha256 matches" % name)
        if not ok(os.environ.get("GH", "gh"), "attestation", "verify", binary, "--repo", REPO):
            say("FAIL %s: gh attestation verify" % name)
            return False
        say("%s: attestation verified" % name)
        os.chmod(binary, 0o755)
        version = run(binary, "--version", check=False)
        if not version.startswith(tag + "/"):
            say("FAIL %s --version says %r" % (name, version))
            return False
        say("%s --version: %s" % (name, version))
        return True
    finally:
        shutil.rmtree(work, ignore_errors=True)


def previous_asset_count(tag):
    tags = sorted((t for t in git("tag", "--list", "v*").split() if TAG_RE.match(t) and t != tag),
                  key=lambda t: tuple(int(x) for x in t[1:].split(".")))
    if not tags:
        return None
    rel = gh_json("release", "view", tags[-1], "--repo", REPO, "--json", "assets")
    return len(rel["assets"]) if rel else None


def close_milestones(titles):
    ms = gh_json("api", "repos/%s/milestones?state=open&per_page=100" % REPO, default=[]) or []
    for title in titles:
        m = next((x for x in ms if x.get("title") == title), None)
        if not m:
            continue
        if m.get("open_issues", 0):
            say("milestone %s left open: %d open issue(s)" % (title, m["open_issues"]))
            continue
        gh("api", "-X", "PATCH", "repos/%s/milestones/%s" % (REPO, m["number"]), "-f", "state=closed")
        say("milestone %s closed" % title)


def publish(version, summary, includes):
    tag, branch = "v" + version, "release/v" + version
    sha = git("rev-parse", "HEAD")
    _, body = changelog_section(version)

    step("main")
    git("fetch", "-q", "origin")
    if is_ancestor(sha, "origin/main"):
        say("origin/main already has the release commit")
    else:
        if not is_ancestor("origin/main", sha):
            raise Stop("origin/main has commits the release commit does not. Take the release commit off "
                       "(git reset --hard HEAD~1 - it was never pushed to main) and run make release again: "
                       "it merges main in and stamps again.")
        git("push", "-q", "origin", "%s:refs/heads/%s" % (sha, branch))
        git("push", "-q", "origin", "%s:refs/heads/main" % sha)
        say("pushed main at %s" % sha[:10])
    if ok("git", "merge-base", "--is-ancestor", "refs/heads/main", sha):
        git("update-ref", "refs/heads/main", sha)   # the local main follows (a fast-forward)

    if not remote_tag(tag):
        step("Check on main")
        r = check_on_main(sha)
        if r.get("conclusion") != "success":
            raise Stop("Check on main ended '%s' for the release commit (%s). Nothing is tagged. If it was a "
                       "flake, re-run it (gh run rerun %s) and run make release again - it waits for Check again and "
                       "tags. If the release commit itself is broken, fix it forward on main; the tag then goes on "
                       "the fix, by hand (docs/src/tech-release.md)." % (r.get("conclusion"), r.get("url", ""), r.get("databaseId", "")))
        say("Check passed")

        step("the tag")
        if not local_tag(tag):
            r = subprocess.run(["git", "tag", "-a", tag, sha, "-F", "-"], input="%s: %s\n\n%s\n" % (tag, summary, body),
                               text=True, capture_output=True)
            if r.returncode != 0:
                raise Stop("git tag failed: " + r.stderr.strip()[:200])
        git("push", "-q", "origin", "refs/tags/" + tag)
        say("pushed %s - the release pipeline is running" % tag)
    else:
        say("the tag %s is already on origin" % tag)

    step("the release pipeline")
    runs = release_runs(tag)
    failed = [r for r in runs if r.get("conclusion") not in ("success", "skipped")]
    for r in runs:
        say("%-26s %s" % (r.get("workflowName", "?"), r.get("conclusion")))
    rel = gh_json("release", "view", tag, "--repo", REPO, "--json", "assets,url") or {"assets": []}
    assets = [a["name"] for a in rel.get("assets", [])]
    want = previous_asset_count(tag)
    say("release %s: %d assets%s" % (rel.get("url", tag), len(assets),
                                    "" if want is None else " (the previous release had %d)" % want))
    good = not failed and assets and (want is None or len(assets) == want)
    if os.environ.get("RELEASE_VERIFY_BINARY", "1") != "0" and assets:
        good = verify_binary(tag, assets) and good
    close_milestones([tag] + includes)
    if not good:
        raise Stop("%s is tagged, but something above failed - look at it; nothing here can be undone, fix forward" % tag)
    step("%s is released" % tag)
    say("main's README already shows it; releases/%s.md is on main as its record." % tag)


# --- the whole thing ----------------------------------------------------------------------

def rehearsal_clone():
    """A throw-away clone at this branch, with origin's refs and pushing disabled."""
    top = git("rev-parse", "--show-toplevel")
    head = git("rev-parse", "HEAD")
    branch = git("rev-parse", "--abbrev-ref", "HEAD")
    git("fetch", "-q", "origin", "--tags")
    where = tempfile.mkdtemp(prefix="shint-release-rehearsal-")
    git("clone", "-q", "--no-checkout", top, where)
    os.chdir(where)
    git("fetch", "-q", top, "+refs/remotes/origin/*:refs/remotes/origin/*", "+refs/tags/*:refs/tags/*")
    git("remote", "set-url", "--push", "origin", "/dev/null/pushing-is-disabled-in-a-rehearsal")
    git("checkout", "-q", "-B", branch, head)
    return where


def release(dry_run):
    if dry_run:
        where = rehearsal_clone()
        step("rehearsal in %s - nothing will be pushed" % where)
    version = branch_version()
    tag = "v" + version
    dirty = git("status", "--porcelain")
    if dirty:
        raise Stop("the working tree has uncommitted changes:\n%s" % "\n".join("    " + l for l in dirty.splitlines()[:10]))
    if not dry_run:
        git("fetch", "-q", "origin", "--tags")
        run(os.environ.get("GH", "gh"), "auth", "status")
    page, summary, includes, unchecked = working_page(version)

    if remote_tag(tag):                       # published: only the pipeline and the checks are left
        publish(version, summary, includes)
        return
    if local_tag(tag) and not is_release_commit(version):
        raise Stop("a local tag %s exists but the branch tip is not its release commit - look at it (git show %s)" % (tag, tag))

    issues = [] if dry_run else open_issues([tag] + includes)
    if is_release_commit(version):
        say("the branch tip is already the release commit (%s); carrying on from there" % git("rev-parse", "--short", "HEAD"))
        if not dry_run and not is_ancestor("HEAD", "origin/main"):
            verify_locally(version, None, dry_run)
    else:
        day, body = changelog_section(version)
        if day is None:
            raise Stop("CHANGELOG.md has no '## %s' section" % tag)
        if day != "unreleased":
            raise Stop("CHANGELOG.md's '## %s' heading says '%s', not 'unreleased' - the date is stamped by "
                       "make release, never typed" % (tag, day))
        if not bullets(body):
            raise Stop("CHANGELOG.md's '## %s - unreleased' section has no entries yet" % tag)
        if unchecked and not dry_run:
            raise Stop("%d to-do item(s) in %s are still open:\n%s\nCheck them off (- [x]) when they are done, or move "
                       "them to an issue, commit, and run make release again" % (len(unchecked), page, "\n".join("    - [ ] " + u for u in unchecked)))
        if local_tag(tag) or remote_tag(tag):
            raise Stop("the tag %s already exists" % tag)
        base = prepare(version, summary, includes, page, today())
        verify_locally(version, base, dry_run)

    summary_card(version, summary, includes, issues)
    if dry_run:
        for u in unchecked:
            say("OPEN TO-DO (a real make release stops on it): - [ ] " + u)
        step("rehearsal done - nothing was pushed")
        say("the release commit is in %s (rm -rf it when you are done looking)" % os.getcwd())
        return
    if not ask(tag):
        sha = git("rev-parse", "--short", "HEAD")
        say("not published; nothing was pushed. The release commit %s stays on your branch: run make release "
            "again to publish it, or take it off with: git reset --hard %s~1" % (sha, sha))
        return
    publish(version, summary, includes)


def main(argv):
    try:
        if argv[:1] == ["start"]:
            if len(argv) != 2:
                raise Stop("usage: release.py start X.Y.Z  (make release-start VERSION=X.Y.Z)")
            start(argv[1])
        elif argv in ([], ["--dry-run"]):
            release(dry_run=bool(argv))
        else:
            raise Stop("usage: release.py [--dry-run] | release.py start X.Y.Z")
    except Stop as e:
        print("\nrelease: STOPPED - %s" % e, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
