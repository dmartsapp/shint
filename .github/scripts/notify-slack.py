#!/usr/bin/env python3
"""The outcome of a release, from the Release workflow's own jobs: to Slack, to a
failure issue, and - for a release that was published - as an announcement.

    python3 .github/scripts/notify-slack.py

Run by two jobs at the end of .github/workflows/release.yaml, which `need` every
other job, so nothing is waited for: each job's result (success, failure,
cancelled, skipped) and outputs arrive in NEEDS (the workflow's toJSON(needs)),
and the run's job list (GitHub's API) adds durations and the step each failed
job failed in.

- announce: posts one message to Slack - the tag, whether the release was
  published, one line per part with its result, what failed and where (and why
  a part was skipped: the part it needs did not pass), the failure issue if one
  was opened - and, when the release was published, announces it, for the people
  who use shint rather than the ones who build it: a second Slack post, and a
  discussion in the repository's Announcements category ("vX.Y.Z released",
  once per version). Both hold the release's one-line summary (releases/vX.Y.Z.md),
  the headline of each changelog entry for the version, how to get it, and links
  to the release notes, the changelog and the documentation.
- report-failure (ISSUE_BODY set): writes the failure issue's body - every part
  with its result, each failed job and step, the run's link - and posts nothing.

Settings come from the environment:

  NEEDS               toJSON(needs) of the calling job: {job: {result, outputs}}
  GH_TOKEN            token for the gh CLI (actions: read, contents: read)
  REPO, TAG, SHA      owner/name, the tag (v4.5.0) and the commit it points at
  RUN_ID              the Release workflow's run (its jobs, their steps)
  ACTOR               who pushed the tag
  SERVER_URL          https://github.com
  SLACK_WEBHOOK_URL   the incoming webhook; without it nothing is posted to Slack
                      (a fork, or the secret not set) and the run still succeeds
  ANNOUNCE_DISCUSSIONS=1  also announce a published release in Discussions
                      (GH_TOKEN then needs discussions: write)
  ISSUE_BODY          write the failure issue's body to this file, and only that
  DRY_RUN=1           print the messages instead of posting them
  CHANGELOG           the changelog to announce from (default CHANGELOG.md)
  RELEASES_DIR        where releases/vX.Y.Z.md lives (default releases)

The webhook URL is a credential: it is never printed, logged or put in an error.
Standard library only, like the other scripts here.
"""
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime

# The Release workflow's jobs, in the order they are shown: (job id, the start of
# the job's `name:` - a matrix job's names share it -, the label shown). Tests keep
# this list, and NEEDS_OF, in step with release.yaml.
PARTS = [
    ("verify-tag", "Verify the tag", "Release tag"),
    ("lint", "golangci-lint", "golangci-lint"),
    ("vulncheck", "govulncheck", "govulncheck"),
    ("hadolint", "hadolint", "Dockerfile (hadolint)"),
    ("binaries", "Build ", "Binaries"),
    ("github-release", "Publish the GitHub release", "GitHub release"),
    ("docker-hub", "Docker Hub image", "Docker Hub image"),
    ("ghcr", "GHCR image", "GHCR image"),
    ("image-reports", "Attach the images", "Images' SBOMs and reports on the release"),
    ("readme", "Propose main's README", "README proposal for main"),
]
# What a release is: these all passed means it was published, whole. The README
# proposal comes after and publishes nothing, so its failure is reported, not a
# reason to withhold the announcement.
PUBLISHING = [p[0] for p in PARTS if p[0] != "readme"]
# Each part's `needs:` in release.yaml - to say why a part was skipped.
NEEDS_OF = {
    "verify-tag": [],
    "lint": ["verify-tag"],
    "vulncheck": ["verify-tag"],
    "hadolint": ["verify-tag"],
    "binaries": ["verify-tag", "lint", "vulncheck", "hadolint"],
    "github-release": ["verify-tag", "vulncheck", "binaries"],
    "docker-hub": ["verify-tag", "lint", "vulncheck", "hadolint"],
    "ghcr": ["verify-tag", "lint", "vulncheck", "hadolint"],
    "image-reports": ["github-release", "docker-hub", "ghcr"],
    "readme": ["github-release"],
}
LABEL = {p[0]: p[2] for p in PARTS}

GREEN, RED, AMBER = "#2EB67D", "#E01E5A", "#ECB22E"
ICON = {"success": ":white_check_mark:", "failure": ":x:", "cancelled": ":no_entry_sign:", "skipped": ":fast_forward:"}
MARK = {"success": "✅", "failure": "❌", "cancelled": "🚫", "skipped": "⏭️"}


def parse_time(text):
    try:
        return datetime.strptime(text, "%Y-%m-%dT%H:%M:%SZ")
    except (TypeError, ValueError):
        return None


def human(seconds):
    if seconds is None:
        return None
    minutes, secs = divmod(int(seconds), 60)
    return "%dm %02ds" % (minutes, secs) if minutes else "%ds" % secs


def gh_json(args):
    """Run the gh CLI and parse its JSON output."""
    done = subprocess.run(["gh"] + list(args), capture_output=True, text=True)
    if done.returncode != 0:
        raise RuntimeError("gh %s failed: %s" % (" ".join(args[:2]), done.stderr.strip()[:200]))
    return json.loads(done.stdout or "null")


def parse_needs(text):
    """NEEDS (toJSON(needs)) as {job: {"result": ..., "outputs": {...}}}; a job it does not
    name is missing, which reads as "did not run"."""
    try:
        data = json.loads(text or "{}")
    except ValueError:
        return {}
    out = {}
    for job, v in (data or {}).items():
        if isinstance(v, dict):
            out[job] = {"result": v.get("result") or "unknown", "outputs": v.get("outputs") or {}}
    return out


def fetch_jobs(gh, repo, run_id):
    """The run's jobs (name, conclusion, steps, times, URL), or [] when the API says nothing."""
    if not run_id:
        return []
    try:
        return gh(["api", "repos/%s/actions/runs/%s/jobs?per_page=100" % (repo, run_id)]).get("jobs") or []
    except (RuntimeError, AttributeError, TypeError):
        return []


def jobs_of(part, jobs):
    """The run's jobs that are this part - one, or a matrix job's legs."""
    prefix = next(p[1] for p in PARTS if p[0] == part)
    return [j for j in jobs if (j.get("name") or "").startswith(prefix)]


def failed_steps(part, jobs, limit=4):
    """'job › step' for each failed job of a part (a matrix's legs each), at most limit."""
    found = []
    for job in jobs_of(part, jobs):
        if job.get("conclusion") not in ("failure", "timed_out"):
            continue
        steps = [s["name"] for s in job.get("steps") or [] if s.get("conclusion") == "failure"]
        found.append("%s › %s" % (job["name"], steps[0]) if steps else job["name"])
    return found[:limit] + (["+%d more" % (len(found) - limit)] if len(found) > limit else [])


def part_duration(part, jobs):
    starts = [parse_time(j.get("started_at")) for j in jobs_of(part, jobs)]
    ends = [parse_time(j.get("completed_at")) for j in jobs_of(part, jobs)]
    starts, ends = [t for t in starts if t], [t for t in ends if t]
    if not starts or not ends or max(ends) < min(starts):
        return None
    return (max(ends) - min(starts)).total_seconds()


def result_of(part, needs):
    return needs.get(part, {}).get("result", "not run")


def why_skipped(part, needs):
    """For a skipped part: which part it needs did not pass."""
    for dep in NEEDS_OF.get(part, []):
        r = result_of(dep, needs)
        if r != "success":
            return "%s %s" % (LABEL[dep], "was skipped" if r == "skipped" else "did not pass (%s)" % r)
    return None


def classify(needs):
    """('refused' | 'published' | 'failed', parts that passed)."""
    passed = sum(1 for p, _, _ in PARTS if result_of(p, needs) == "success")
    if result_of("verify-tag", needs) != "success":
        return "refused", passed
    if all(result_of(p, needs) == "success" for p in PUBLISHING):
        return "published", passed
    return "failed", passed


def published(needs):
    """What went out: the GitHub release and each image that was pushed."""
    return [LABEL[p] for p in ("github-release", "docker-hub", "ghcr") if result_of(p, needs) == "success"]


def extra_for(part, outputs):
    """What a part handed on, worth a word in its line."""
    if part == "verify-tag" and outputs.get("version"):
        return "version %s" % outputs["version"]
    if part == "vulncheck" and outputs.get("vulnerabilities") not in (None, "", "unknown"):
        return "%s vulnerabilities" % outputs["vulnerabilities"]
    if part == "github-release" and outputs.get("assets"):
        return "%s files" % outputs["assets"]
    if part in ("docker-hub", "ghcr") and outputs.get("digest"):
        return "`%s`" % outputs["digest"].replace("sha256:", "")[:12]
    return None


def part_lines(needs, jobs, slack=True):
    """One line per part (Slack mrkdwn, or a Markdown table row): its result, what it
    handed on, how long it took, and - when it did not pass - why: the failed job and
    step, or the part it needed that did not pass."""
    lines = []
    for part, _, label in PARTS:
        result = result_of(part, needs)
        bits = []
        if result == "skipped":
            why = why_skipped(part, needs)
            bits.append("skipped: " + why if why else "skipped")
        elif result != "success":
            bits.append(result)
        extra = extra_for(part, needs.get(part, {}).get("outputs", {}))
        if extra and result == "success":
            bits.append(extra)
        took = human(part_duration(part, jobs)) if result != "skipped" else None
        if took:
            bits.append(took)
        fails = failed_steps(part, jobs)
        own = jobs_of(part, jobs)
        url = own[0].get("html_url") if len(own) == 1 else None
        if slack:
            name = "*<%s|%s>*" % (url, label) if url else "*%s*" % label
            text = "  ·  ".join(["%s  %s" % (ICON.get(result, ":grey_question:"), name)] + bits)
            text += "".join("\n        `%s`" % f for f in fails)
        else:
            name = "[%s](%s)" % (label, url) if url else label
            cell = "%s %s" % (MARK.get(result, "❔"), " · ".join(bits or [result]))
            if fails:
                cell += " - failed in " + ", ".join("`%s`" % f for f in fails)
            text = "| %s | %s |" % (name, cell.replace("|", "/"))
        lines.append(text)
    return lines


def build_payload(tag, repo, sha, subject, actor, server, run_id, needs, jobs, release_url, issue):
    """The Slack message (a legacy attachment holding Block Kit blocks, which is what
    gives it the coloured bar). Pure: no network, no environment."""
    state, passed = classify(needs)
    total = len(PARTS)
    readme_ok = result_of("readme", needs) == "success"
    starts = [parse_time(j.get("started_at")) for j in jobs if j.get("started_at")]
    ends = [parse_time(j.get("completed_at")) for j in jobs if j.get("completed_at")]
    elapsed = human((max(ends) - min(starts)).total_seconds()) if starts and ends and max(ends) >= min(starts) else None

    if state == "published" and readme_ok:
        color, headline = GREEN, ":rocket:  *shint %s is released*" % tag
        summary = "All %d parts passed" % total
    elif state == "published":
        color, headline = AMBER, ":rocket:  *shint %s is released* - the README proposal did not go through" % tag
        summary = "%d of %d parts passed" % (passed, total)
    elif state == "refused":
        color, headline = RED, ":no_entry:  *shint %s - the tag was refused; nothing was built or published*" % tag
        summary = "A release tag must be vX.Y.Z on a commit on main"
    else:
        color, headline = RED, ":rotating_light:  *shint %s - the release failed*" % tag
        out = published(needs)
        summary = "%d of %d parts passed  ·  published: %s" % (passed, total, ", ".join(out) if out else "nothing")
    if elapsed:
        summary += "  ·  finished in %s" % elapsed

    short = sha[:7]
    context = ["Tag *%s*" % tag, "Commit <%s/%s/commit/%s|`%s`>" % (server, repo, sha, short)]
    if subject:
        context.append(subject if len(subject) <= 80 else subject[:77] + "...")
    if actor:
        context.append("pushed by %s" % actor)

    buttons = []
    if release_url and state == "published":
        buttons.append({"type": "button", "text": {"type": "plain_text", "text": "Release notes"}, "url": release_url, "style": "primary"})
    run_url = "%s/%s/actions/runs/%s" % (server, repo, run_id) if run_id else "%s/%s/actions?query=branch%%3A%s" % (server, repo, tag)
    buttons.append({"type": "button", "text": {"type": "plain_text", "text": "The run"}, "url": run_url})
    if issue:
        buttons.append({"type": "button", "text": {"type": "plain_text", "text": "Failure issue #%s" % issue},
                        "url": "%s/%s/issues/%s" % (server, repo, issue)})

    blocks = [
        {"type": "section", "text": {"type": "mrkdwn", "text": headline + "\n" + summary}},
        {"type": "divider"},
        {"type": "section", "text": {"type": "mrkdwn", "text": "\n".join(part_lines(needs, jobs))[:2900]}},
        {"type": "context", "elements": [{"type": "mrkdwn", "text": "  ·  ".join(context)}]},
        {"type": "actions", "elements": buttons},
    ]
    fallback = {"published": "shint %s released: %d of %d parts passed" % (tag, passed, total),
                "refused": "shint %s: the tag was refused, nothing was built" % tag,
                "failed": "shint %s: the release failed (%d of %d parts passed)" % (tag, passed, total)}[state]
    return {"text": fallback, "attachments": [{"color": color, "blocks": blocks}]}


def issue_body(tag, sha, server, repo, run_id, needs, jobs):
    """The failure issue: every part with its result, each failed job and step, the run."""
    state, passed = classify(needs)
    run_url = "%s/%s/actions/runs/%s" % (server, repo, run_id)
    out = published(needs)
    went_out = ("**Published:** %s." % ", ".join(out) if out else "**Published:** nothing.") + (
        " The release is whole: what failed came after it." if state == "published"
        else " A part that did not pass stopped the parts that need it; the parts beside it ran.")
    lines = ["### CI failure: Release %s" % tag, "",
             "The Release workflow failed on `%s` (commit `%s`): %d of %d parts passed." % (tag, sha, passed, len(PARTS)), "",
             "**Run:** %s" % run_url, "", went_out, "",
             "| Part | Result |", "|---|---|"]
    lines += part_lines(needs, jobs, slack=False)
    lines += ["", "Open the run, find the step that failed and read its log. Once it is fixed, "
              "**Re-run failed jobs** on the run starts the failed jobs again, and the ones that were "
              "skipped because of them."]
    return "\n".join(lines) + "\n"


# --- the announcement ------------------------------------------------------------------

DOCS_URL = "https://dmartsapp.github.io/shint/"
CHANGELOG_URL = "https://dmartsapp.github.io/shint/docs/changelog.html"
DOCKER_IMAGE = "farhansabbir/shint"   # the Docker Hub image the release workflow pushes
MAX_HIGHLIGHTS = 8                    # changelog headlines shown; the rest are counted
MAX_HEADLINE = 240                    # characters per headline, before it is cut


def changelog_entries(text, tag):
    """The entries ("- ..." items, one string each) of the changelog section for tag
    (## v4.4.0 - 2026-11-29), or [] when the changelog has no such section."""
    version = tag.lstrip("v")
    heading = re.compile(r"^## v%s( |$)" % re.escape(version))
    entries, inside = [], False
    for line in text.splitlines():
        if line.startswith("## "):
            if inside:
                break
            inside = bool(heading.match(line))
            continue
        if not inside:
            continue
        if line.startswith("- "):
            entries.append(line[2:].strip())
        elif entries and line.strip() and line.startswith("  "):
            entries[-1] += " " + line.strip()   # a wrapped entry
    return entries


def headline(entry):
    """What an entry says first: its leading bold sentence (with what comes before
    it, as in "Release process: **...**"), else its first sentence."""
    m = re.match(r"^([^*]{0,40}?)\*\*(.+?)\*\*", entry)
    if m:
        return (m.group(1) + "**" + m.group(2) + "**").strip()
    first = re.split(r"(?<=[.!?])\s", entry, maxsplit=1)[0]
    return first


def mrkdwn(md):
    """Markdown as Slack mrkdwn: &, < and > escaped, **bold** as *bold*, and
    [text](url) as <url|text>. `code` is the same in both."""
    text = md.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    text = re.sub(r"\[([^\]]+)\]\(([^)\s]+)\)", lambda m: "<%s|%s>" % (m.group(2), m.group(1)), text)
    return re.sub(r"\*\*(.+?)\*\*", r"*\1*", text)


def cut(text, limit):
    if len(text) <= limit:
        return text
    return text[:limit - 1].rsplit(" ", 1)[0] + "…"


def release_summary(text):
    """The Summary: line of a releases/vX.Y.Z.md, or None."""
    for line in text.splitlines():
        if line.startswith("Summary:"):
            return line[len("Summary:"):].strip() or None
    return None


def build_announcement(tag, summary, entries, release_url):
    """The release announcement, posted after the result when everything passed, or
    None when the changelog has nothing for this version. Pure: no network."""
    items = [e for e in entries if not e.lower().startswith("release process")]
    if not items:
        return None
    shown = items[:MAX_HIGHLIGHTS]
    bullets = ["•  " + cut(mrkdwn(headline(e)), MAX_HEADLINE) for e in shown]
    if len(items) > len(shown):
        bullets.append("_...and %d more in the release notes._" % (len(items) - len(shown)))

    intro = ":tada:  *shint %s is out*" % tag
    if summary:
        intro += "\n" + mrkdwn(summary)
    version = tag.lstrip("v")
    get = ["Download from the <%s|release>" % release_url if release_url else "Download from GitHub",
           "`docker pull %s:%s`" % (DOCKER_IMAGE, version),
           "`go install github.com/dmartsapp/shint/v4@%s`" % tag]

    buttons = []
    if release_url:
        buttons.append({"type": "button", "text": {"type": "plain_text", "text": "Release notes"}, "url": release_url, "style": "primary"})
    buttons.append({"type": "button", "text": {"type": "plain_text", "text": "Changelog"}, "url": CHANGELOG_URL})
    buttons.append({"type": "button", "text": {"type": "plain_text", "text": "Documentation"}, "url": DOCS_URL})

    blocks = [
        {"type": "section", "text": {"type": "mrkdwn", "text": intro}},
        {"type": "section", "text": {"type": "mrkdwn", "text": "*What's new*\n" + "\n".join(bullets)}},
        {"type": "context", "elements": [{"type": "mrkdwn", "text": "  ·  ".join(get)}]},
        {"type": "actions", "elements": buttons},
    ]
    fallback = "shint %s is out" % tag + (": " + summary if summary else "")
    return {"text": fallback, "attachments": [{"color": GREEN, "blocks": blocks}]}


def discussion_title(tag):
    return "%s released" % tag


def build_discussion(tag, summary, entries, release_url):
    """The Discussions announcement, in Markdown: every highlight (there is room), or
    None when the changelog has nothing for this version. Pure: no network."""
    items = [e for e in entries if not e.lower().startswith("release process")]
    if not items:
        return None
    release_url = release_url or "https://github.com/dmartsapp/shint/releases/tag/%s" % tag
    lines = ["**shint %s is out.**%s" % (tag, " " + summary if summary else ""), "", "## What's new", ""]
    lines += ["- " + headline(e) for e in items]
    lines += ["", "Every change, with the details, is in the [release notes](%s) and the [changelog](%s)." % (release_url, CHANGELOG_URL),
              "", "## Get it", "",
              "- [Download from the release](%s): a binary for each platform, with checksums, signed attestations and SBOMs" % release_url,
              "- `docker pull %s:%s`" % (DOCKER_IMAGE, tag.lstrip("v")),
              "- `go install github.com/dmartsapp/shint/v4@%s`" % tag,
              "", "[Documentation](%s)" % DOCS_URL]
    return "\n".join(lines) + "\n"


DISCUSSION_LOOKUP = """query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    id
    discussionCategories(first: 25) { nodes { id name } }
    discussions(first: 50, orderBy: {field: CREATED_AT, direction: DESC}) { nodes { title url } }
  }
}"""

DISCUSSION_CREATE = """mutation($repo: ID!, $category: ID!, $title: String!, $body: String!) {
  createDiscussion(input: {repositoryId: $repo, categoryId: $category, title: $title, body: $body}) {
    discussion { url }
  }
}"""


def announce_in_discussions(gh, repo, tag, body, category="Announcements"):
    """Create the release's discussion, unless one with its title exists already (a
    re-run). Returns its URL, or None if it could not be made. The body goes to gh
    as a raw field (-f), so nothing in it is read as a file name."""
    owner, name = repo.split("/", 1)
    try:
        found = gh(["api", "graphql", "-f", "query=" + DISCUSSION_LOOKUP, "-f", "owner=" + owner, "-f", "name=" + name])["data"]["repository"]
    except (RuntimeError, KeyError, TypeError) as err:
        print("could not look up the repository's discussions (%s)" % err)
        return None
    title = discussion_title(tag)
    for d in found["discussions"]["nodes"]:
        if d["title"] == title:
            print("Discussions already has %r: %s" % (title, d["url"]))
            return d["url"]
    ids = [c["id"] for c in found["discussionCategories"]["nodes"] if c["name"] == category]
    if not ids:
        print("the repository has no %r discussion category" % category)
        return None
    try:
        made = gh(["api", "graphql", "-f", "query=" + DISCUSSION_CREATE, "-f", "repo=" + found["id"],
                   "-f", "category=" + ids[0], "-f", "title=" + title, "-f", "body=" + body])
        return made["data"]["createDiscussion"]["discussion"]["url"]
    except (RuntimeError, KeyError, TypeError) as err:
        print("could not create the discussion (%s)" % err)
        return None


def read_file(path):
    try:
        with open(path, encoding="utf-8") as f:
            return f.read()
    except OSError:
        return ""


def post(url, payload, attempts=3, sleep=time.sleep):
    """POST to the webhook. Returns True on Slack's 'ok'. Never prints the URL."""
    body = json.dumps(payload).encode("utf-8")
    for attempt in range(1, attempts + 1):
        try:
            request = urllib.request.Request(url, data=body, headers={"Content-Type": "application/json"})
            with urllib.request.urlopen(request, timeout=20) as response:
                answer = response.read().decode("utf-8", "replace").strip()
            if answer == "ok":
                return True
            print("Slack answered %r" % answer[:80])
            return False  # a message Slack refuses will be refused again
        except urllib.error.HTTPError as err:
            print("Slack answered HTTP %d" % err.code)
            if 400 <= err.code < 500 and err.code != 429:
                return False
        except (urllib.error.URLError, OSError) as err:
            print("could not reach Slack (%s)" % type(err).__name__)
        if attempt < attempts:
            sleep(3 * attempt)
    return False


def write_step_summary(needs, jobs, tag):
    path = os.environ.get("GITHUB_STEP_SUMMARY")
    if not path:
        return
    with open(path, "a", encoding="utf-8") as out:
        out.write("### Release %s\n\n| Part | Result |\n|---|---|\n%s\n" % (tag, "\n".join(part_lines(needs, jobs, slack=False))))


def main(env=None, gh=gh_json, sleep=time.sleep):
    env = os.environ if env is None else env
    repo, tag, sha = env.get("REPO", ""), env.get("TAG", ""), env.get("SHA", "")
    if not (repo and tag and sha):
        print("REPO, TAG and SHA are required", file=sys.stderr)
        return 2
    needs = parse_needs(env.get("NEEDS", ""))
    run_id = env.get("RUN_ID", "")
    server = env.get("SERVER_URL", "https://github.com")

    if env.get("ISSUE_BODY"):
        body = issue_body(tag, sha, server, repo, run_id, needs, fetch_jobs(gh, repo, run_id))
        with open(env["ISSUE_BODY"], "w", encoding="utf-8") as f:
            f.write(body)
        print(body)
        return 0

    url = env.get("SLACK_WEBHOOK_URL", "")
    dry = env.get("DRY_RUN") == "1"
    discussions = env.get("ANNOUNCE_DISCUSSIONS") == "1"
    if not url and not dry:
        print("SLACK_WEBHOOK_URL is not set (a fork, or the secret is missing): nothing to post to Slack.")
        if not discussions:
            return 0

    jobs = fetch_jobs(gh, repo, run_id)
    state, _ = classify(needs)
    release_url = needs.get("github-release", {}).get("outputs", {}).get("url") or None
    issue = needs.get("report-failure", {}).get("outputs", {}).get("issue") or None
    subject = None
    try:
        subject = gh(["api", "repos/%s/commits/%s" % (repo, sha), "--jq", "{message: .commit.message}"])["message"].splitlines()[0]
    except (RuntimeError, IndexError, KeyError, TypeError):
        pass

    payload = build_payload(tag, repo, sha, subject, env.get("ACTOR", ""), server, run_id, needs, jobs, release_url, issue)
    print(payload["text"])
    write_step_summary(needs, jobs, tag)

    # The announcement: only for a release that was published, whole.
    announcement = discussion = None
    if state == "published":
        entries = changelog_entries(read_file(env.get("CHANGELOG", "CHANGELOG.md")), tag)
        summary = release_summary(read_file(os.path.join(env.get("RELEASES_DIR", "releases"), tag + ".md")))
        announcement = build_announcement(tag, summary, entries, release_url)
        discussion = build_discussion(tag, summary, entries, release_url)
        if announcement is None:
            print("The changelog has no entries for %s: no announcement." % tag)

    if dry:
        print(json.dumps(payload, indent=2))
        if announcement:
            print(json.dumps(announcement, indent=2))
        if discussion and discussions:
            print("--- the discussion %r ---\n%s" % (discussion_title(tag), discussion))
        return 0

    status = 0
    if url:
        if post(url, payload, sleep=sleep):
            print("Posted to Slack.")
            if announcement is not None:
                if post(url, announcement, sleep=sleep):
                    print("Posted the announcement to Slack.")
                else:
                    print("The announcement was not delivered to Slack.", file=sys.stderr)
                    status = 1
        else:
            print("The message was not delivered to Slack.", file=sys.stderr)
            status = 1
    if discussions and discussion is not None:
        made = announce_in_discussions(gh, repo, tag, discussion)
        if made:
            print("Announced in Discussions: %s" % made)
        else:
            print("The release was not announced in Discussions.", file=sys.stderr)
            status = 1
    return status


if __name__ == "__main__":
    sys.exit(main())
