#!/usr/bin/env python3
"""Tests for release.py, end to end, in throw-away repositories: a bare "origin", a clone
of it, the real readme-reconcile.py and release-check.sh, main's real README, and a fake gh.

    python3 .github/scripts/test_release.py
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
TOP = os.path.dirname(os.path.dirname(HERE))
SCRIPT = os.path.join(HERE, "release.py")

FAKE_GH = r'''#!/usr/bin/env python3
import json, os, sys
path = os.environ["FAKE_GH_STATE"]
state = json.load(open(path))
a = sys.argv[1:]
with open(path + ".log", "a") as log:
    log.write(" ".join(a) + "\n")
def out(x):
    print(json.dumps(x)); sys.exit(0)
if a[:2] == ["auth", "status"]: sys.exit(0)
if a[:1] == ["api"]:
    if "-X" in a: sys.exit(0)
    out(state.get("milestones", []))
if a[:2] == ["issue", "list"]: out(state.get("issues", []))
if a[:2] == ["run", "list"]:
    if "--workflow" in a:
        out([{"databaseId": 7, "status": "completed", "conclusion": state.get("check", "success"), "url": "https://example/check"}])
    out([{"databaseId": i, "workflowName": w, "status": "completed", "conclusion": "success", "url": "u"}
         for i, w in enumerate(["Lint", "Vulnerability Check", "Binary Build & Release", "Docker Hub Release", "GHCR Release"])])
if a[:2] == ["release", "view"]:
    out({"assets": [{"name": n} for n in ("shint.linux.amd64", "shint.linux.amd64.sha256")], "url": "https://example/release"})
sys.exit(1)
'''

ENV = {
    "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
    "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
    "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null",
    "RELEASE_TODAY": "2026-11-15", "RELEASE_CHECK_CMD": "true", "RELEASE_LIVE_CMD": "true",
    "RELEASE_DOCS_CMD": "true", "RELEASE_POLL": "0", "RELEASE_VERIFY_BINARY": "0", "RELEASE_YES": "1",
}


class Fixture(unittest.TestCase):
    """A bare origin with main (and a v4.2.2 tag), cloned into work/."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="test-release-")
        self.origin = os.path.join(self.tmp, "origin.git")
        self.work = os.path.join(self.tmp, "work")
        self.gh = os.path.join(self.tmp, "gh")
        self.state = os.path.join(self.tmp, "gh-state.json")
        with open(self.gh, "w") as f:
            f.write(FAKE_GH)
        os.chmod(self.gh, 0o755)
        self.set_gh()
        self.env = dict(os.environ, **ENV, GH=self.gh, FAKE_GH_STATE=self.state)

        subprocess.run(["git", "init", "-q", "--bare", "-b", "main", self.origin], check=True)
        seed = os.path.join(self.tmp, "seed")
        os.makedirs(os.path.join(seed, ".github", "scripts"))
        self.put(seed, "main.go", 'package main\n\nvar (\n\tVersion string = "4.2.2"\n)\n')
        self.put(seed, "CHANGELOG.md", "# Changelog\n\nThe intro.\n\n## v4.2.2 - 2026-09-26\n\n- **An old fix.** Text.\n")
        shutil.copy(os.path.join(TOP, "readme.md"), os.path.join(seed, "readme.md"))
        for s in ("readme-reconcile.py", "release-check.sh"):
            shutil.copy(os.path.join(HERE, s), os.path.join(seed, ".github", "scripts", s))
        self.git(seed, "init", "-q", "-b", "main")
        self.git(seed, "add", "-A")
        self.git(seed, "commit", "-q", "-m", "main")
        self.git(seed, "tag", "-a", "v4.2.2", "-m", "v4.2.2")
        self.git(seed, "remote", "add", "origin", self.origin)
        self.git(seed, "push", "-q", "origin", "main", "--tags")
        self.seed = seed
        self.git(self.tmp, "clone", "-q", self.origin, self.work)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    # --- helpers
    def set_gh(self, **state):
        with open(self.state, "w") as f:
            json.dump(state, f)

    def put(self, where, name, text):
        with open(os.path.join(where, name), "w") as f:
            f.write(text)

    def read(self, name, where=None):
        with open(os.path.join(where or self.work, name)) as f:
            return f.read()

    def git(self, where, *args):
        r = subprocess.run(["git"] + list(args), cwd=where, capture_output=True, text=True, env=dict(os.environ, **ENV))
        if r.returncode != 0:
            raise AssertionError("git %s: %s" % (" ".join(args), r.stderr))
        return r.stdout.strip()

    def release(self, *args, **env):
        r = subprocess.run([sys.executable, SCRIPT] + list(args), cwd=self.work, capture_output=True, text=True,
                           env=dict(self.env, **env), start_new_session=True)   # no terminal: /dev/tty fails
        return r.returncode, r.stdout + r.stderr

    def origin_ref(self, ref):
        r = subprocess.run(["git", "rev-parse", "-q", "--verify", ref], cwd=self.origin, capture_output=True, text=True)
        return r.stdout.strip() or None

    def ready(self, summary="`tls` checks certificates", includes=""):
        """release/v4.3.0 started, with a Summary and a changelog entry, committed."""
        rc, out = self.release("start", "4.3.0")
        self.assertEqual(rc, 0, out)
        page = self.read("releases/v4.3.0.md").replace("Summary:\n", "Summary: %s\n" % summary)
        if includes:
            page = page.replace("Includes:\n", "Includes: %s\n" % includes)
        self.put(self.work, "releases/v4.3.0.md", page)
        cl = self.read("CHANGELOG.md").replace("## v4.3.0 - unreleased\n\n", "## v4.3.0 - unreleased\n\n- **`tls` is new.** It checks certificates.\n\n")
        self.put(self.work, "CHANGELOG.md", cl)
        self.git(self.work, "commit", "-q", "-am", "feat: tls")
        return self.git(self.work, "rev-parse", "HEAD")


class Start(Fixture):
    def test_it_creates_and_pushes_the_branch_the_heading_and_the_page(self):
        rc, out = self.release("start", "4.3.0")
        self.assertEqual(rc, 0, out)
        self.assertEqual(self.git(self.work, "rev-parse", "--abbrev-ref", "HEAD"), "release/v4.3.0")
        self.assertIsNotNone(self.origin_ref("refs/heads/release/v4.3.0"))
        self.assertIn("## v4.3.0 - unreleased\n\n## v4.2.2 - 2026-09-26", self.read("CHANGELOG.md"))
        page = self.read("releases/v4.3.0.md")
        self.assertTrue(page.startswith("# shint v4.3.0 - work in progress"), page[:60])
        self.assertRegex(page, r"(?m)^Summary:$")
        self.assertIn("after the v4.2.2 tag", page)
        self.assertEqual(self.git(self.work, "log", "-1", "--format=%s"), "release: start v4.3.0")

    def test_it_refuses_an_existing_branch_and_a_bad_version(self):
        self.assertEqual(self.release("start", "4.3.0")[0], 0)
        self.git(self.work, "switch", "-q", "main")
        rc, out = self.release("start", "4.3.0")
        self.assertEqual(rc, 1)
        self.assertIn("already exists", out)
        rc, out = self.release("start", "v4.3")
        self.assertEqual(rc, 1)
        self.assertIn("VERSION must look like", out)


class Refusals(Fixture):
    def test_a_branch_that_is_not_a_release_branch(self):
        rc, out = self.release()
        self.assertEqual(rc, 1)
        self.assertIn("run this on a release branch", out)

    def test_no_summary(self):
        self.ready(summary="")
        self.put(self.work, "releases/v4.3.0.md", re.sub(r"(?m)^Summary:.*$", "Summary:", self.read("releases/v4.3.0.md")))
        self.git(self.work, "commit", "-q", "-am", "x")
        rc, out = self.release()
        self.assertEqual(rc, 1)
        self.assertIn("no 'Summary:' line", out)

    def test_an_empty_changelog_section(self):
        self.release("start", "4.3.0")
        self.put(self.work, "releases/v4.3.0.md", self.read("releases/v4.3.0.md").replace("Summary:\n", "Summary: s\n"))
        self.git(self.work, "commit", "-q", "-am", "x")
        rc, out = self.release()
        self.assertEqual(rc, 1)
        self.assertIn("has no entries yet", out)

    def test_a_typed_date(self):
        self.ready()
        self.put(self.work, "CHANGELOG.md", self.read("CHANGELOG.md").replace("## v4.3.0 - unreleased", "## v4.3.0 - 2026-MM-DD"))
        self.git(self.work, "commit", "-q", "-am", "x")
        rc, out = self.release()
        self.assertEqual(rc, 1)
        self.assertIn("never typed", out)

    def test_an_uncommitted_change(self):
        self.ready()
        self.put(self.work, "main.go", "changed\n")
        rc, out = self.release()
        self.assertEqual(rc, 1)
        self.assertIn("uncommitted changes", out)


class Release(Fixture):
    def test_end_to_end(self):
        self.ready()
        before_main = self.origin_ref("refs/heads/main")
        rc, out = self.release()
        self.assertEqual(rc, 0, out)
        head = self.git(self.work, "rev-parse", "HEAD")
        self.assertEqual(self.origin_ref("refs/heads/main"), head)
        self.assertEqual(self.origin_ref("refs/tags/v4.3.0^{commit}"), head)
        self.assertNotEqual(before_main, head)
        # stamped from the branch name and the clock
        self.assertIn('Version string = "4.3.0"', self.read("main.go"))
        self.assertIn("## v4.3.0 - 2026-11-15", self.read("CHANGELOG.md"))
        self.assertTrue(self.read("releases/v4.3.0.md").startswith("# shint v4.3.0 - released 2026-11-15"))
        # main's README: only the roadmap row, from the Summary
        self.assertIn("| **v4.3.0** | Released Nov 15 | `tls` checks certificates |", self.read("readme.md"))
        diff = self.git(self.work, "diff", "-U0", before_main, "HEAD", "--", "readme.md")
        changed = [l for l in diff.splitlines() if l[:1] in "+-" and not l.startswith(("+++", "---"))]
        self.assertTrue(changed and all(re.match(r"^[-+]\| \*\*v\d+\.\d+\.(\d+|x)\*\* \|", l) for l in changed), changed)
        # the messages
        msg = self.git(self.work, "log", "-1", "--format=%B")
        self.assertTrue(msg.startswith("release: `tls` checks certificates (v4.3.0)"), msg[:80])
        self.assertIn("Full changelog for v4.3.0", msg)
        self.assertIn("**`tls` is new.**", msg)
        self.assertNotRegex(msg, r"(?i)co-authored-by|generated with")
        tagmsg = self.git(self.work, "tag", "-l", "--format=%(contents)", "v4.3.0")
        self.assertTrue(tagmsg.startswith("v4.3.0: `tls` checks certificates"), tagmsg[:80])
        self.assertIn("**`tls` is new.**", tagmsg)
        self.assertIn("v4.3.0 is released", out)

    def test_main_that_moved_is_merged_in(self):
        self.ready()
        self.put(self.seed, "other.txt", "main moved\n")
        self.git(self.seed, "add", "other.txt")
        self.git(self.seed, "commit", "-q", "-m", "docs: something on main")
        self.git(self.seed, "push", "-q", "origin", "main")
        moved = self.git(self.seed, "rev-parse", "HEAD")
        rc, out = self.release()
        self.assertEqual(rc, 0, out)
        self.assertIn("merged it in", out)
        main = self.origin_ref("refs/heads/main")
        self.assertEqual(subprocess.run(["git", "merge-base", "--is-ancestor", moved, main], cwd=self.origin).returncode, 0)

    def test_a_conflict_with_main_stops_and_changes_nothing(self):
        start = self.ready()
        self.put(self.seed, "main.go", 'package main\n\nvar (\n\tVersion string = "4.2.9"\n)\n')
        self.git(self.seed, "commit", "-q", "-am", "main: a clashing change")
        self.git(self.seed, "push", "-q", "origin", "main")
        self.put(self.work, "main.go", 'package main\n\n// a comment\nvar (\n\tVersion string = "4.2.3"\n)\n')
        self.git(self.work, "commit", "-q", "-am", "branch: a clashing change")
        head = self.git(self.work, "rev-parse", "HEAD")
        before = self.origin_ref("refs/heads/main")
        rc, out = self.release()
        self.assertEqual(rc, 1)
        self.assertIn("does not merge cleanly", out)
        self.assertEqual(self.git(self.work, "rev-parse", "HEAD"), head)
        self.assertEqual(self.git(self.work, "status", "--porcelain"), "")
        self.assertEqual(self.origin_ref("refs/heads/main"), before)
        self.assertNotEqual(start, None)

    def test_failing_checks_take_the_release_commit_back_off(self):
        start = self.ready()
        rc, out = self.release(RELEASE_CHECK_CMD="false")
        self.assertEqual(rc, 1)
        self.assertIn("taken back off", out)
        self.assertEqual(self.git(self.work, "rev-parse", "HEAD"), start)
        self.assertIn("## v4.3.0 - unreleased", self.read("CHANGELOG.md"))
        self.assertIsNone(self.origin_ref("refs/tags/v4.3.0"))
        rc, out = self.release()               # fixed: the same command goes all the way
        self.assertEqual(rc, 0, out)

    def test_a_red_check_on_main_stops_before_the_tag_and_a_rerun_carries_on(self):
        self.ready()
        self.set_gh(check="failure")
        rc, out = self.release()
        self.assertEqual(rc, 1)
        self.assertIn("Nothing is tagged", out)
        head = self.git(self.work, "rev-parse", "HEAD")
        self.assertEqual(self.origin_ref("refs/heads/main"), head)
        self.assertIsNone(self.origin_ref("refs/tags/v4.3.0"))
        self.set_gh(check="success")
        rc, out = self.release()
        self.assertEqual(rc, 0, out)
        self.assertIn("carrying on", out)
        self.assertEqual(self.origin_ref("refs/heads/main"), head)       # no second release commit
        self.assertEqual(self.origin_ref("refs/tags/v4.3.0^{commit}"), head)

    def test_without_a_terminal_or_a_yes_nothing_is_pushed(self):
        self.ready()
        before = self.origin_ref("refs/heads/main")
        env = dict(self.env)
        env.pop("RELEASE_YES")
        r = subprocess.run([sys.executable, SCRIPT], cwd=self.work, capture_output=True, text=True, env=env,
                           start_new_session=True)
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIn("no terminal", r.stderr)
        self.assertEqual(self.origin_ref("refs/heads/main"), before)
        self.assertIsNone(self.origin_ref("refs/tags/v4.3.0"))

    def test_a_published_release_only_gets_checked_again(self):
        self.ready()
        self.assertEqual(self.release()[0], 0)
        head = self.origin_ref("refs/heads/main")
        rc, out = self.release()
        self.assertEqual(rc, 0, out)
        self.assertIn("already on origin", out)
        self.assertEqual(self.origin_ref("refs/heads/main"), head)

    def test_includes_marks_folded_releases_shipped_in_this_one(self):
        self.ready(includes="v4.4.0")
        rc, out = self.release()
        self.assertEqual(rc, 0, out)
        self.assertRegex(self.read("readme.md"), r"(?m)^\| \*\*v4\.4\.0\*\* \| Shipped in v4\.3\.0 \|")
        self.assertIn("Includes v4.4.0", self.git(self.work, "log", "-1", "--format=%B"))

    def test_a_dry_run_pushes_nothing_and_leaves_the_branch_alone(self):
        head = self.ready()
        before = self.origin_ref("refs/heads/main")
        rc, out = self.release("--dry-run", RELEASE_YES="")
        self.assertEqual(rc, 0, out)
        self.assertIn("rehearsal done - nothing was pushed", out)
        self.assertEqual(self.git(self.work, "rev-parse", "HEAD"), head)
        self.assertEqual(self.origin_ref("refs/heads/main"), before)
        self.assertIsNone(self.origin_ref("refs/tags/v4.3.0"))
        where = re.search(r"release commit is in (\S+)", out).group(1)
        self.assertIn("Full changelog for v4.3.0", self.git(where, "log", "-1", "--format=%B"))
        shutil.rmtree(where, ignore_errors=True)


if __name__ == "__main__":
    unittest.main(verbosity=2)
