#!/usr/bin/env python3
"""Post the outcome of a release tag's workflows to Slack, as one clean message.

    python3 .github/scripts/notify-slack.py

Run by .github/workflows/notify-slack.yaml, which the tag push starts alongside
the five release workflows. This waits until each of them has finished (or the
wait runs out), then posts the result: the tag, whether everything passed, one
line per workflow with its duration and a link to its run, what failed and where,
and buttons for the release and the runs.

When everything passed, the release is announced, for the people who use shint
rather than the ones who build it: in a second Slack post, and as a discussion in
the repository's Announcements category (titled "vX.Y.Z released", once per
version). Both hold the release's one-line summary (releases/vX.Y.Z.md), the
headline of each changelog entry for the version (CHANGELOG.md), how to get it,
and links to the release notes, the changelog and the documentation. A failed or
unfinished release is never announced.

Settings come from the environment:

  GH_TOKEN            token for the gh CLI (actions: read, contents: read)
  REPO, TAG, SHA      owner/name, the tag (v4.0.6) and the commit it points at
  RUN_ID              this workflow's own run, ignored while waiting
  ACTOR               who pushed the tag
  SERVER_URL          https://github.com
  SLACK_WEBHOOK_URL   the incoming webhook; without it nothing is posted to Slack
                      (a fork, or the secret not set) and the run still succeeds
  ANNOUNCE_DISCUSSIONS=1  also announce a release that passed in Discussions
                      (GH_TOKEN then needs discussions: write)
  WAIT_SECONDS        how long to wait for the workflows (default 3000)
  POLL_SECONDS        seconds between looks (default 20)
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

# The workflows a release tag starts, in the order they are shown:
# (the workflow's `name:`, the label shown in Slack). A test keeps this list in
# step with the workflow files, so adding a release workflow without listing it
# here fails `make workflows`.
WORKFLOWS = [
    ("Binary Build & Release", "Binary build and release"),
    ("Vulnerability Check", "Vulnerability check"),
    ("Lint", "Linting"),
    ("Docker Hub Release", "Docker Hub image"),
    ("GHCR Release", "GHCR image"),
]

GREEN, RED, AMBER = "#2EB67D", "#E01E5A", "#ECB22E"
OK_CONCLUSIONS = ("success",)
NOT_RUN_CONCLUSIONS = ("skipped", "neutral")


def parse_time(text):
    try:
        return datetime.strptime(text, "%Y-%m-%dT%H:%M:%SZ")
    except (TypeError, ValueError):
        return None


def duration(run):
    start, end = parse_time(run.get("startedAt")), parse_time(run.get("updatedAt"))
    return int((end - start).total_seconds()) if start and end and end >= start else None


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


def fetch_runs(gh, repo, sha, tag, own_run):
    """The newest run of each workflow that this tag push started."""
    runs = gh(["run", "list", "--repo", repo, "--commit", sha, "--limit", "100", "--json",
               "databaseId,workflowName,status,conclusion,url,startedAt,updatedAt,event,headBranch"])
    latest = {}
    for run in runs or []:
        if run.get("event") != "push" or run.get("headBranch") != tag or str(run["databaseId"]) == str(own_run):
            continue
        name = run["workflowName"]
        if name not in latest or run["databaseId"] > latest[name]["databaseId"]:
            latest[name] = run
    return latest


def wait_for_runs(fetch, names, wait_seconds, poll_seconds, sleep=time.sleep, clock=time.monotonic, log=print):
    """Poll until every named workflow has completed. Returns (runs, timed_out)."""
    started = clock()
    runs = {}
    while True:
        try:
            runs = fetch()
        except RuntimeError as err:  # a hiccup in the API: look again rather than give up
            log("could not list the runs (%s); will retry" % err)
        pending = [n for n in names if n not in runs or runs[n].get("status") != "completed"]
        if not pending:
            return runs, False
        if clock() - started >= wait_seconds:
            return runs, True
        log("waiting for: " + ", ".join("%s (%s)" % (n, runs[n]["status"] if n in runs else "not started") for n in pending))
        sleep(poll_seconds)


def failed_steps(gh, repo, run, limit=3):
    """'job > step' for what failed in a run, at most limit of them."""
    try:
        jobs = gh(["run", "view", str(run["databaseId"]), "--repo", repo, "--json", "jobs"])["jobs"]
    except (RuntimeError, KeyError, TypeError):
        return []
    found = []
    for job in jobs:
        if job.get("conclusion") != "failure":
            continue
        steps = [s["name"] for s in job.get("steps", []) if s.get("conclusion") == "failure"]
        found.append("%s › %s" % (job["name"], steps[0]) if steps else job["name"])
    return found[:limit] + (["+%d more" % (len(found) - limit)] if len(found) > limit else [])


def classify(runs, timed_out):
    """('passed' | 'failed' | 'running', number passed)."""
    conclusions = [runs[n]["conclusion"] if n in runs and runs[n].get("status") == "completed" else None
                   for n, _ in WORKFLOWS]
    passed = sum(1 for c in conclusions if c in OK_CONCLUSIONS)
    if any(c is not None and c not in OK_CONCLUSIONS and c not in NOT_RUN_CONCLUSIONS for c in conclusions):
        return "failed", passed
    if any(c is None for c in conclusions) or timed_out:
        return "running", passed
    return ("passed" if passed == len(WORKFLOWS) else "failed"), passed


def line_for(label, run, extra, failures):
    if run is None:
        return ":grey_question:  *%s*  ·  did not start" % label
    if run.get("status") != "completed":
        icon, state = ":hourglass_flowing_sand:", "still running"
    elif run["conclusion"] in OK_CONCLUSIONS:
        icon, state = ":white_check_mark:", None
    elif run["conclusion"] in NOT_RUN_CONCLUSIONS:
        icon, state = ":fast_forward:", run["conclusion"]
    else:
        icon, state = ":x:", run["conclusion"].replace("_", " ")
    parts = ["%s  *<%s|%s>*" % (icon, run["url"], label)]
    for part in (state, extra, human(duration(run))):
        if part:
            parts.append(part)
    text = "  ·  ".join(parts)
    for failure in failures:
        text += "\n        `%s`" % failure
    return text


def build_payload(tag, repo, sha, subject, actor, server, runs, timed_out, assets, release_url, failures):
    """The Slack message (a legacy attachment holding Block Kit blocks, which is what
    gives it the coloured bar). Pure: no network, no environment."""
    state, passed = classify(runs, timed_out)
    total = len(WORKFLOWS)
    starts = [parse_time(r.get("startedAt")) for r in runs.values() if r.get("startedAt")]
    ends = [parse_time(r.get("updatedAt")) for r in runs.values() if r.get("status") == "completed"]
    elapsed = human((max(ends) - min(starts)).total_seconds()) if starts and ends and max(ends) >= min(starts) else None

    if state == "passed":
        color, headline = GREEN, ":rocket:  *shint %s is released*" % tag
        summary = "All %d checks passed" % total
    elif state == "failed":
        color, headline = RED, ":rotating_light:  *shint %s - the release pipeline failed*" % tag
        summary = "%d of %d checks passed" % (passed, total)
    else:
        color, headline = AMBER, ":hourglass_flowing_sand:  *shint %s - the pipeline is still running*" % tag
        summary = "%d of %d checks passed so far" % (passed, total)
    if elapsed:
        summary += "  ·  finished in %s" % elapsed if state != "running" else "  ·  after %s" % elapsed

    lines = []
    for name, label in WORKFLOWS:
        run = runs.get(name)
        extra = None
        if name == "Binary Build & Release" and assets and run and run.get("conclusion") == "success":
            extra = "%d files attached" % assets
        lines.append(line_for(label, run, extra, failures.get(name, [])))

    short = sha[:7]
    context = ["Tag *%s*" % tag, "Commit <%s/%s/commit/%s|`%s`>" % (server, repo, sha, short)]
    if subject:
        context.append(subject if len(subject) <= 80 else subject[:77] + "...")
    if actor:
        context.append("pushed by %s" % actor)

    buttons = []
    if release_url and state == "passed":
        buttons.append({"type": "button", "text": {"type": "plain_text", "text": "Release notes"}, "url": release_url, "style": "primary"})
    buttons.append({"type": "button", "text": {"type": "plain_text", "text": "Workflow runs"},
                    "url": "%s/%s/actions?query=branch%%3A%s" % (server, repo, tag)})

    blocks = [
        {"type": "section", "text": {"type": "mrkdwn", "text": headline + "\n" + summary}},
        {"type": "divider"},
        {"type": "section", "text": {"type": "mrkdwn", "text": "\n".join(lines)}},
        {"type": "context", "elements": [{"type": "mrkdwn", "text": "  ·  ".join(context)}]},
        {"type": "actions", "elements": buttons},
    ]
    fallback = {"passed": "shint %s released: all %d checks passed" % (tag, total),
                "failed": "shint %s: release pipeline failed (%d of %d checks passed)" % (tag, passed, total),
                "running": "shint %s: pipeline still running (%d of %d checks passed)" % (tag, passed, total)}[state]
    return {"text": fallback, "attachments": [{"color": color, "blocks": blocks}]}


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


def write_step_summary(payload, tag):
    path = os.environ.get("GITHUB_STEP_SUMMARY")
    if not path:
        return
    blocks = payload["attachments"][0]["blocks"]
    with open(path, "a", encoding="utf-8") as out:
        out.write("### Slack notification for %s\n\n%s\n" % (tag, blocks[0]["text"]["text"].replace("*", "**")))


def main(env=None, gh=gh_json, sleep=time.sleep, clock=time.monotonic):
    env = os.environ if env is None else env
    repo, tag, sha = env.get("REPO", ""), env.get("TAG", ""), env.get("SHA", "")
    if not (repo and tag and sha):
        print("REPO, TAG and SHA are required", file=sys.stderr)
        return 2
    url = env.get("SLACK_WEBHOOK_URL", "")
    dry = env.get("DRY_RUN") == "1"
    discussions = env.get("ANNOUNCE_DISCUSSIONS") == "1"
    if not url and not dry:
        print("SLACK_WEBHOOK_URL is not set (a fork, or the secret is missing): nothing to post to Slack.")
        if not discussions:
            return 0

    names = [n for n, _ in WORKFLOWS]
    runs, timed_out = wait_for_runs(lambda: fetch_runs(gh, repo, sha, tag, env.get("RUN_ID", "")), names,
                                    int(env.get("WAIT_SECONDS", "3000")), int(env.get("POLL_SECONDS", "20")),
                                    sleep=sleep, clock=clock)
    failures = {}
    for name in names:
        run = runs.get(name)
        if run and run.get("status") == "completed" and run["conclusion"] not in OK_CONCLUSIONS + NOT_RUN_CONCLUSIONS:
            failures[name] = failed_steps(gh, repo, run)

    assets = release_url = subject = None
    try:
        release = gh(["release", "view", tag, "--repo", repo, "--json", "assets,url"])
        assets, release_url = len(release.get("assets", [])), release.get("url")
    except (RuntimeError, AttributeError):
        pass  # no release (yet): the message simply has no release button
    try:
        subject = gh(["api", "repos/%s/commits/%s" % (repo, sha), "--jq", "{message: .commit.message}"])["message"].splitlines()[0]
    except (RuntimeError, IndexError, KeyError, TypeError):
        pass

    payload = build_payload(tag, repo, sha, subject, env.get("ACTOR", ""), env.get("SERVER_URL", "https://github.com"),
                            runs, timed_out, assets, release_url, failures)
    print(payload["text"])
    write_step_summary(payload, tag)

    # The announcement: only for a release whose every check passed.
    announcement = discussion = None
    if classify(runs, timed_out)[0] == "passed":
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
