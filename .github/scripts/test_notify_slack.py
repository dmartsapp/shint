#!/usr/bin/env python3
"""Tests for notify-slack.py: the result of every part, the message, the failure issue,
the posting, that its lists match release.yaml, and that the webhook URL can never end up
in the repository. No network beyond a local server.

    python3 .github/scripts/test_notify_slack.py
"""
import contextlib
import glob
import importlib.util
import io
import json
import os
import re
import subprocess
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
spec = importlib.util.spec_from_file_location("notify_slack", os.path.join(HERE, "notify-slack.py"))
ns = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ns)

TAG, REPO, SHA = "v4.5.0", "dmartsapp/shint", "6607f211d296a90bb0a40ff92d278ef8aaf8e8ae"
SERVER = "https://github.com"
RELEASE_URL = "https://github.com/dmartsapp/shint/releases/tag/v4.5.0"


def needs(**results):
    """NEEDS as the workflow sends it: every part passed, unless a result is given
    (keyword names use _ for -)."""
    out = {}
    for part, _, _ in ns.PARTS:
        out[part] = {"result": results.get(part.replace("-", "_"), "success"), "outputs": {}}
    out["verify-tag"]["outputs"] = {"version": "4.5.0", "minor": "4.5", "major": "4"}
    if out["github-release"]["result"] == "success":
        out["github-release"]["outputs"] = {"url": RELEASE_URL, "assets": "73"}
    out["vulncheck"]["outputs"] = {"vulnerabilities": "0"}
    out["docker-hub"]["outputs"] = {"digest": "sha256:" + "ab" * 32, "image": "docker.io/farhansabbir/shint"}
    return out


def job(name, conclusion="success", failed_step=None, start="2026-11-30T10:00:00Z", end="2026-11-30T10:01:30Z", jid=1):
    steps = [{"name": "Set up job", "conclusion": "success"}]
    if failed_step:
        steps.append({"name": failed_step, "conclusion": "failure"})
    return {"name": name, "conclusion": conclusion, "steps": steps, "started_at": start, "completed_at": end,
            "html_url": "%s/%s/actions/runs/9/job/%d" % (SERVER, REPO, jid)}


JOBS = [job("Verify the tag (vX.Y.Z, on main)", end="2026-11-30T10:00:05Z", jid=1),
        job("golangci-lint", start="2026-11-30T10:00:10Z", end="2026-11-30T10:00:50Z", jid=2),
        job("Build linux/amd64", start="2026-11-30T10:01:00Z", end="2026-11-30T10:02:00Z", jid=3),
        job("Build linux/arm64", start="2026-11-30T10:01:00Z", end="2026-11-30T10:02:30Z", jid=4),
        job("Docker Hub image", start="2026-11-30T10:01:00Z", end="2026-11-30T10:08:00Z", jid=5)]


def payload(n, jobs=JOBS, release_url=RELEASE_URL, issue=None):
    return ns.build_payload(TAG, REPO, SHA, "release: things (v4.5.0)", "farhansabbir", SERVER, "9", n, jobs, release_url, issue)


def body_of(p):
    return p["attachments"][0]["blocks"][2]["text"]["text"]


class ResultTests(unittest.TestCase):
    def test_classify(self):
        self.assertEqual(ns.classify(needs()), ("published", len(ns.PARTS)))
        self.assertEqual(ns.classify(needs(readme="failure"))[0], "published")      # after the release: reported, not a reason
        self.assertEqual(ns.classify(needs(lint="failure"))[0], "failed")
        self.assertEqual(ns.classify(needs(ghcr="cancelled"))[0], "failed")
        self.assertEqual(ns.classify(needs(verify_tag="failure"))[0], "refused")
        self.assertEqual(ns.classify({})[0], "refused")                              # nothing known: nothing claimed

    def test_parse_needs(self):
        self.assertEqual(ns.parse_needs('{"lint": {"result": "failure", "outputs": {}}}'), {"lint": {"result": "failure", "outputs": {}}})
        self.assertEqual(ns.parse_needs("not json"), {})
        self.assertEqual(ns.parse_needs(""), {})

    def test_a_skipped_part_says_which_part_it_needed(self):
        n = needs(binaries="failure", github_release="skipped", image_reports="skipped", readme="skipped")
        self.assertEqual(ns.why_skipped("github-release", n), "Binaries did not pass (failure)")
        self.assertEqual(ns.why_skipped("readme", n), "GitHub release was skipped")
        # a skipped job's start and end are the same moment: no "0s" next to it (seen in the rehearsal)
        jobs = [job("Attach the images' SBOMs and reports", "skipped", start="2026-11-30T10:05:00Z", end="2026-11-30T10:05:00Z")]
        line = [l for l in ns.part_lines(n, jobs, slack=False) if l.startswith("| [Images'")][0]
        self.assertNotIn("0s", line)

    def test_failed_steps_name_each_failed_leg(self):
        jobs = [job("Build linux/arm64", "failure", "Build the binary"), job("Build linux/amd64"),
                job("Build aix/ppc64", "failure")]
        self.assertEqual(ns.failed_steps("binaries", jobs), ["Build linux/arm64 › Build the binary", "Build aix/ppc64"])
        many = [job("Build x/%d" % i, "failure") for i in range(6)]
        self.assertEqual(ns.failed_steps("binaries", many)[-1], "+2 more")

    def test_durations(self):
        self.assertEqual(ns.human(372), "6m 12s")
        self.assertEqual(ns.human(45), "45s")
        self.assertIsNone(ns.human(None))
        self.assertEqual(ns.part_duration("binaries", JOBS), 90)                     # first leg's start to last leg's end
        self.assertIsNone(ns.part_duration("ghcr", JOBS))

    def test_published(self):
        self.assertEqual(ns.published(needs()), ["GitHub release", "Docker Hub image", "GHCR image"])
        self.assertEqual(ns.published(needs(binaries="failure", github_release="skipped")), ["Docker Hub image", "GHCR image"])


class MessageTests(unittest.TestCase):
    def test_published(self):
        p = payload(needs())
        att = p["attachments"][0]
        self.assertEqual(att["color"], ns.GREEN)
        head = att["blocks"][0]["text"]["text"]
        self.assertIn("shint v4.5.0 is released", head)
        self.assertIn("All %d parts passed" % len(ns.PARTS), head)
        self.assertIn("finished in 8m 00s", head)
        body = body_of(p)
        order = [body.index(label) for _, _, label in ns.PARTS]
        self.assertEqual(order, sorted(order), "parts are listed in the workflow's order")
        self.assertEqual(body.count(":white_check_mark:"), len(ns.PARTS))
        self.assertIn("version 4.5.0", body)
        self.assertIn("73 files", body)
        self.assertIn("`" + "ab" * 6 + "`", body)                                   # the image digest, short
        self.assertIn("1m 30s", body)                                                # the binaries' span
        self.assertIn("<%s/%s/actions/runs/9/job/2|golangci-lint>" % (SERVER, REPO), body)   # a single job links to itself
        buttons = att["blocks"][-1]["elements"]
        self.assertEqual([b["text"]["text"] for b in buttons], ["Release notes", "The run"])
        self.assertTrue(buttons[1]["url"].endswith("/actions/runs/9"))
        self.assertIn("/commit/" + SHA, json.dumps(p))

    def test_a_failure_names_what_failed_what_was_skipped_and_what_went_out(self):
        n = needs(binaries="failure", github_release="skipped", image_reports="skipped", readme="skipped")
        jobs = JOBS + [job("Build aix/ppc64", "failure", "Build the binary", jid=8)]
        p = payload(n, jobs=jobs, release_url=None, issue="120")
        att = p["attachments"][0]
        self.assertEqual(att["color"], ns.RED)
        head = att["blocks"][0]["text"]["text"]
        self.assertIn("the release failed", head)
        self.assertIn("published: Docker Hub image, GHCR image", head)
        body = body_of(p)
        self.assertIn(":x:  *Binaries*", body)
        self.assertIn("`Build aix/ppc64 › Build the binary`", body)
        self.assertIn("skipped: Binaries did not pass (failure)", body)
        names = [b["text"]["text"] for b in att["blocks"][-1]["elements"]]
        self.assertEqual(names, ["The run", "Failure issue #120"])                  # no release, so no release button

    def test_published_with_a_failed_readme_proposal(self):
        p = payload(needs(readme="failure"))
        self.assertEqual(p["attachments"][0]["color"], ns.AMBER)
        self.assertIn("is released", p["attachments"][0]["blocks"][0]["text"]["text"])
        self.assertIn("README proposal did not go through", p["attachments"][0]["blocks"][0]["text"]["text"])

    def test_a_refused_tag(self):
        # GitHub reports every job after a failed one as skipped
        n = {part: {"result": "skipped", "outputs": {}} for part, _, _ in ns.PARTS}
        n["verify-tag"]["result"] = "failure"
        p = payload(n, jobs=[], release_url=None)
        self.assertIn("the tag was refused", p["attachments"][0]["blocks"][0]["text"]["text"])
        self.assertIn(":fast_forward:  *golangci-lint*  ·  skipped: Release tag did not pass (failure)", body_of(p))
        self.assertEqual(ns.classify({})[0], "refused")   # and with nothing known, nothing is claimed

    def test_slack_limits(self):
        many = [job("Build x/%d %s" % (i, "y" * 80), "failure", "Build the binary " * 5) for i in range(30)]
        p = payload(needs(binaries="failure", github_release="skipped"), jobs=many, release_url=None)
        blocks = p["attachments"][0]["blocks"]
        self.assertLessEqual(len(blocks), 50)
        for b in blocks:
            if b["type"] == "section":
                self.assertLessEqual(len(b["text"]["text"]), 3000)
        self.assertEqual(json.loads(json.dumps(p)), p)

    def test_long_subject_is_cut(self):
        p = ns.build_payload(TAG, REPO, SHA, "x" * 500, "a", SERVER, "9", needs(), JOBS, None, None)
        self.assertLess(len(p["attachments"][0]["blocks"][3]["elements"][0]["text"]), 300)


class IssueTests(unittest.TestCase):
    def test_the_issue_lists_every_part_and_the_failed_step(self):
        n = needs(binaries="failure", github_release="skipped", image_reports="skipped", readme="skipped")
        jobs = JOBS + [job("Build aix/ppc64", "failure", "Build the binary", jid=8)]
        body = ns.issue_body(TAG, SHA, SERVER, REPO, "9", n, jobs)
        self.assertTrue(body.startswith("### CI failure: Release v4.5.0\n"))
        self.assertIn("%s/%s/actions/runs/9" % (SERVER, REPO), body)
        self.assertIn("**Published:** Docker Hub image, GHCR image.", body)
        self.assertIn("| Binaries | ❌ failure · 2m 30s - failed in `Build aix/ppc64 › Build the binary` |", body)
        self.assertIn("| GitHub release | ⏭️ skipped: Binaries did not pass (failure) |", body)
        self.assertEqual(body.count("\n| "), len(ns.PARTS) + 1)                      # the header and a row per part

    def test_main_writes_the_issue_body_and_posts_nothing(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path = os.path.join(tmp.name, "issue.md")
        n = needs(lint="failure", binaries="skipped", github_release="skipped", docker_hub="skipped", ghcr="skipped",
                  image_reports="skipped", readme="skipped")
        env = {"REPO": REPO, "TAG": TAG, "SHA": SHA, "RUN_ID": "9", "NEEDS": json.dumps(n), "ISSUE_BODY": path,
               "SLACK_WEBHOOK_URL": "http://127.0.0.1:9/services/TSECRET/BSECRET/XSECRET"}
        calls = []

        def gh(args):
            calls.append(args)
            return {"jobs": [job("golangci-lint", "failure", "golangci-lint", jid=2)]}
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(ns.main(env, gh=gh), 0)
        with open(path, encoding="utf-8") as f:
            body = f.read()
        self.assertIn("failed in `golangci-lint › golangci-lint`", body)
        self.assertIn("**Published:** nothing.", body)
        self.assertEqual(calls, [["api", "repos/%s/actions/runs/9/jobs?per_page=100" % REPO]])


class FakeSlack(BaseHTTPRequestHandler):
    replies = []   # (status, body) served in turn; the last one repeats
    seen = []

    def do_POST(self):
        FakeSlack.seen.append(json.loads(self.rfile.read(int(self.headers["Content-Length"]))))
        status, body = FakeSlack.replies[min(len(FakeSlack.seen) - 1, len(FakeSlack.replies) - 1)]
        self.send_response(status)
        self.end_headers()
        self.wfile.write(body.encode())

    def log_message(self, *args):
        pass


class PostTests(unittest.TestCase):
    def serve(self, replies):
        FakeSlack.replies, FakeSlack.seen = replies, []
        server = HTTPServer(("127.0.0.1", 0), FakeSlack)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        # a URL that looks like a real webhook, so a leak into the output is visible
        return "http://127.0.0.1:%d/services/TSECRET/BSECRET/XSECRETXSECRET" % server.server_address[1]

    def call(self, fn):
        out = io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(out):
            result = fn()
        return result, out.getvalue()

    def test_ok(self):
        url = self.serve([(200, "ok")])
        result, out = self.call(lambda: ns.post(url, {"text": "hi"}, sleep=lambda s: None))
        self.assertTrue(result)
        self.assertEqual(FakeSlack.seen, [{"text": "hi"}])
        self.assertNotIn("SECRET", out)

    def test_a_message_slack_refuses_is_not_retried(self):
        url = self.serve([(400, "invalid_payload")])
        result, out = self.call(lambda: ns.post(url, {"text": "hi"}, sleep=lambda s: None))
        self.assertFalse(result)
        self.assertEqual(len(FakeSlack.seen), 1)
        self.assertIn("400", out)
        self.assertNotIn("SECRET", out)

    def test_a_server_error_is_retried(self):
        url = self.serve([(500, "oops"), (500, "oops"), (200, "ok")])
        result, out = self.call(lambda: ns.post(url, {"text": "hi"}, sleep=lambda s: None))
        self.assertTrue(result)
        self.assertEqual(len(FakeSlack.seen), 3)

    def test_unreachable(self):
        url = "http://127.0.0.1:9/services/TSECRET/BSECRET/XSECRET"
        result, out = self.call(lambda: ns.post(url, {"text": "hi"}, attempts=2, sleep=lambda s: None))
        self.assertFalse(result)
        self.assertNotIn("SECRET", out)

    def test_main_without_a_webhook_posts_nothing_and_succeeds(self):
        calls = []
        result, out = self.call(lambda: ns.main({"REPO": REPO, "TAG": TAG, "SHA": SHA}, gh=lambda a: calls.append(a)))
        self.assertEqual(result, 0)
        self.assertEqual(calls, [])
        self.assertIn("not set", out)

    def gh_for(self, created, jobs=JOBS):
        def gh(args):
            if args[0] == "api" and "/actions/runs/" in args[1]:
                return {"jobs": jobs}
            if args[0] == "api" and "/commits/" in args[1]:
                return {"message": "release: a thing (v4.5.0)\n\nbody"}
            if args[:2] == ["api", "graphql"]:
                query = next(a for a in args if a.startswith("query="))
                if "createDiscussion" in query:
                    created.append(next(a for a in args if a.startswith("body=")))
                    return {"data": {"createDiscussion": {"discussion": {"url": "https://github.com/dmartsapp/shint/discussions/99"}}}}
                return {"data": {"repository": {"id": "R_1", "discussionCategories": {"nodes": [{"id": "C_1", "name": "Announcements"}]},
                                                "discussions": {"nodes": []}}}}
            raise RuntimeError("not needed")
        return gh

    def announce_env(self, n, url=None):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        d = tmp.name
        with open(os.path.join(d, "CHANGELOG.md"), "w") as f:
            f.write(CHANGELOG.replace("## v4.0.6 ", "## v4.5.0 "))
        os.mkdir(os.path.join(d, "releases"))
        with open(os.path.join(d, "releases", TAG + ".md"), "w") as f:
            f.write("# shint v4.5.0 - released\n\nSummary: Two wrong answers fixed\n")
        env = {"REPO": REPO, "TAG": TAG, "SHA": SHA, "RUN_ID": "9", "ACTOR": "farhansabbir", "NEEDS": json.dumps(n),
               "ANNOUNCE_DISCUSSIONS": "1", "CHANGELOG": os.path.join(d, "CHANGELOG.md"), "RELEASES_DIR": os.path.join(d, "releases")}
        if url:
            env["SLACK_WEBHOOK_URL"] = url
        return env

    def test_main_end_to_end_announces_a_published_release(self):
        url = self.serve([(200, "ok")])
        created = []
        result, out = self.call(lambda: ns.main(self.announce_env(needs(), url), gh=self.gh_for(created), sleep=lambda s: None))
        self.assertEqual(result, 0, out)
        self.assertEqual(len(FakeSlack.seen), 2)                     # the result, then the announcement
        self.assertIn("released", FakeSlack.seen[0]["text"])
        self.assertIn("release: a thing (v4.5.0)", json.dumps(FakeSlack.seen[0]))
        self.assertIn(RELEASE_URL, json.dumps(FakeSlack.seen[0]))   # the release's URL came from github-release's output
        self.assertEqual(FakeSlack.seen[1]["text"], "shint v4.5.0 is out: Two wrong answers fixed")
        self.assertEqual(len(created), 1)
        self.assertIn("Announced in Discussions", out)
        self.assertNotIn("SECRET", out)

    def test_main_never_announces_a_failed_release(self):
        url = self.serve([(200, "ok")])
        created = []
        n = needs(ghcr="failure", image_reports="failure")
        n["report-failure"] = {"result": "success", "outputs": {"issue": "121"}}
        result, out = self.call(lambda: ns.main(self.announce_env(n, url), gh=self.gh_for(created), sleep=lambda s: None))
        self.assertEqual(result, 0, out)
        self.assertEqual(len(FakeSlack.seen), 1)
        self.assertIn("failed", FakeSlack.seen[0]["text"])
        self.assertIn("issues/121", json.dumps(FakeSlack.seen[0]))  # the failure issue, from report-failure's output
        self.assertEqual(created, [])

    def test_main_announces_in_discussions_without_a_webhook(self):
        created = []
        result, out = self.call(lambda: ns.main(self.announce_env(needs()), gh=self.gh_for(created), sleep=lambda s: None))
        self.assertEqual(result, 0, out)
        self.assertEqual(len(created), 1)
        self.assertIn("nothing to post to Slack", out)

    def test_main_reports_delivery_failure(self):
        url = self.serve([(404, "no_service")])
        env = {"REPO": REPO, "TAG": TAG, "SHA": SHA, "SLACK_WEBHOOK_URL": url, "NEEDS": json.dumps(needs())}

        def gh(args):
            raise RuntimeError("no jobs, no commit")   # both are optional in the message
        result, _ = self.call(lambda: ns.main(env, gh=gh, sleep=lambda s: None))
        self.assertEqual(result, 1)


CHANGELOG = """# Changelog

## v4.0.7 - unreleased

- **Not this one.**

## v4.0.6 - 2026-09-21

- **`web` no longer reports a cut-short response as a success ([#40](https://github.com/dmartsapp/shint/issues/40)).** The details
  carry on here.
- **`ping` no longer reports a dead host as reachable.** More text & <things>.
- Built with Go 1.27.4 (was 1.27.3). Then more.
- Release process: **the battery runs on every release.**

## v4.0.5 - 2026-09-20

- **Older.**
"""


class AnnouncementTests(unittest.TestCase):
    def test_changelog_entries_are_the_versions_items(self):
        entries = ns.changelog_entries(CHANGELOG, "v4.0.6")
        self.assertEqual(len(entries), 4)
        self.assertTrue(entries[0].endswith("The details carry on here."))   # a wrapped entry is joined
        self.assertEqual(ns.changelog_entries(CHANGELOG, "v4.0.9"), [])
        self.assertEqual(ns.changelog_entries(CHANGELOG, "v4.0.5"), ["**Older.**"])

    def test_headline(self):
        e = ns.changelog_entries(CHANGELOG, "v4.0.6")
        self.assertEqual(ns.headline(e[0]), "**`web` no longer reports a cut-short response as a success ([#40](https://github.com/dmartsapp/shint/issues/40)).**")
        self.assertEqual(ns.headline(e[2]), "Built with Go 1.27.4 (was 1.27.3).")
        self.assertEqual(ns.headline(e[3]), "Release process: **the battery runs on every release.**")

    def test_mrkdwn(self):
        self.assertEqual(ns.mrkdwn("**a [b](https://x.example/?q=1&r=2) `c` <d> & e**"),
                         "*a <https://x.example/?q=1&amp;r=2|b> `c` &lt;d&gt; &amp; e*")

    def test_announcement(self):
        a = ns.build_announcement("v4.0.6", "Two wrong answers fixed", ns.changelog_entries(CHANGELOG, "v4.0.6"),
                                  "https://github.com/dmartsapp/shint/releases/tag/v4.0.6")
        t = json.dumps(a)
        self.assertIn("shint v4.0.6 is out", t)
        self.assertIn("Two wrong answers fixed", t)
        self.assertIn("<https://github.com/dmartsapp/shint/issues/40|#40>", t)
        self.assertIn("docker pull farhansabbir/shint:4.0.6", t)
        self.assertIn("go install github.com/dmartsapp/shint/v4@v4.0.6", t)
        self.assertNotIn("battery runs", t)            # release-process entries are for builders, not users
        self.assertNotIn("Not this one", t)
        self.assertEqual([b["text"]["text"] for b in a["attachments"][0]["blocks"][3]["elements"]], ["Release notes", "Changelog", "Documentation"])
        self.assertIsNone(ns.build_announcement("v4.0.9", None, [], None))

    def test_announcement_counts_what_it_leaves_out(self):
        entries = ["**Item %d %s.**" % (i, "x" * 400) for i in range(12)]
        a = ns.build_announcement("v4.0.6", None, entries, None)
        text = a["attachments"][0]["blocks"][1]["text"]["text"]
        self.assertIn("...and 4 more in the release notes", text)
        self.assertLess(len(text), 3000)               # Slack's limit for a section
        self.assertNotIn("Release notes", json.dumps(a["attachments"][0]["blocks"][3]))   # no release URL, no button

    def test_discussion(self):
        body = ns.build_discussion("v4.0.6", "Two wrong answers fixed", ns.changelog_entries(CHANGELOG, "v4.0.6"), None)
        self.assertTrue(body.startswith("**shint v4.0.6 is out.** Two wrong answers fixed"))
        self.assertIn("- **`ping` no longer reports a dead host as reachable.**", body)
        self.assertIn("(https://github.com/dmartsapp/shint/releases/tag/v4.0.6)", body)
        self.assertNotIn("battery runs", body)
        self.assertEqual(ns.discussion_title("v4.0.6"), "v4.0.6 released")

    def fake_discussions(self, existing=(), categories=("Announcements", "General")):
        calls = []

        def gh(args):
            calls.append(args)
            query = next(a for a in args if a.startswith("query="))
            if "createDiscussion" in query:
                return {"data": {"createDiscussion": {"discussion": {"url": "https://github.com/dmartsapp/shint/discussions/99"}}}}
            return {"data": {"repository": {"id": "R_1",
                    "discussionCategories": {"nodes": [{"id": "C_" + c, "name": c} for c in categories]},
                    "discussions": {"nodes": [{"title": t, "url": "https://github.com/dmartsapp/shint/discussions/1"} for t in existing]}}}}
        return gh, calls

    def test_a_discussion_is_made_once(self):
        gh, calls = self.fake_discussions()
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(ns.announce_in_discussions(gh, REPO, "v4.0.6", "@/etc/passwd body"), "https://github.com/dmartsapp/shint/discussions/99")
        create = calls[1]
        self.assertIn("body=@/etc/passwd body", create)              # a raw field: never read as a file
        self.assertEqual(create[create.index("body=@/etc/passwd body") - 1], "-f")
        self.assertIn("category=C_Announcements", create)
        self.assertIn("title=v4.0.6 released", create)

        gh, calls = self.fake_discussions(existing=("v4.0.6 released",))
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(ns.announce_in_discussions(gh, REPO, "v4.0.6", "body"), "https://github.com/dmartsapp/shint/discussions/1")
        self.assertEqual(len(calls), 1)                               # found it: nothing created

        gh, calls = self.fake_discussions(categories=("General",))
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertIsNone(ns.announce_in_discussions(gh, REPO, "v4.0.6", "body"))


class RepositoryTests(unittest.TestCase):
    """PARTS and NEEDS_OF against .github/workflows/release.yaml itself."""

    def release_yaml(self):
        with open(os.path.join(ROOT, ".github", "workflows", "release.yaml"), encoding="utf-8") as f:
            return f.read()

    def jobs(self):
        """{job id: (its name:, its needs)} from release.yaml - read by indentation, no YAML library."""
        source = self.release_yaml()
        block = source[source.index("\njobs:\n"):]
        found = {}
        current = None
        for line in block.splitlines():
            m = re.match(r"^  ([a-z0-9-]+):\s*$", line)
            if m:
                current = m.group(1)
                found[current] = ["", []]
                continue
            if current is None:
                continue
            m = re.match(r"^    name:\s*(.+)$", line)
            if m:
                found[current][0] = m.group(1).strip()
            m = re.match(r"^    needs:\s*(.+)$", line)
            if m:
                value = m.group(1).strip()
                found[current][1] = [x.strip() for x in value.strip("[]").split(",")] if value.startswith("[") else [value]
        return found

    def test_the_parts_are_the_workflows_jobs(self):
        jobs = self.jobs()
        self.assertEqual({p[0] for p in ns.PARTS}, set(jobs) - {"report-failure", "announce"})
        for part, prefix, _ in ns.PARTS:
            self.assertTrue(jobs[part][0].startswith(prefix), "%s's name %r does not start with %r" % (part, jobs[part][0], prefix))
            others = [j for j, (name, _) in jobs.items() if j != part and name.startswith(prefix)]
            self.assertEqual(others, [], "%r would match another job's name too" % prefix)

    def test_needs_of_is_the_workflows_needs(self):
        jobs = self.jobs()
        for part, _, _ in ns.PARTS:
            self.assertEqual(sorted(ns.NEEDS_OF[part]), sorted(jobs[part][1]), part)

    def test_the_last_jobs_need_every_part(self):
        jobs = self.jobs()
        parts = {p[0] for p in ns.PARTS}
        self.assertEqual(set(jobs["report-failure"][1]), parts)
        self.assertEqual(set(jobs["announce"][1]), parts | {"report-failure"})
        source = self.release_yaml()
        self.assertIn("NEEDS: ${{ toJSON(needs) }}", source)

    def test_the_webhook_is_a_secret_and_event_text_never_reaches_a_shell(self):
        source = self.release_yaml()
        self.assertIn("SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}", source)
        for line in source.splitlines():
            if "${{ github.event" in line:
                self.assertRegex(line.strip(), r"^[A-Z_]+: \$\{\{ github\.event", "event text only through env: " + line.strip())
        on_block = source[source.index("\non:"):source.index("\njobs:")]
        self.assertNotIn("workflow_run", on_block)      # the trigger rule allows only tag pushes

    def test_no_slack_webhook_url_is_committed(self):
        needle = "hooks.slack" + ".com/services/"
        listing = subprocess.run(["git", "ls-files"], cwd=ROOT, capture_output=True, text=True)
        if listing.returncode != 0:
            self.skipTest("not a git checkout")
        leaks = []
        for rel in listing.stdout.splitlines():
            path = os.path.join(ROOT, rel)
            try:
                with open(path, encoding="utf-8", errors="ignore") as f:
                    if needle in f.read():
                        leaks.append(rel)
            except OSError:
                pass
        self.assertEqual(leaks, [], "a Slack webhook URL is a credential: keep it in the SLACK_WEBHOOK_URL secret")


if __name__ == "__main__":
    unittest.main()
