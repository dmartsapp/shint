#!/usr/bin/env python3
"""Tests for readme-reconcile.py: what it changes, what it leaves alone, that it is
idempotent, and that it fails closed on a README it does not understand.

    python3 .github/scripts/test_readme_reconcile.py
"""
import contextlib
import datetime
import importlib.util
import io
import json
import os
import subprocess
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("readme_reconcile", os.path.join(HERE, "readme-reconcile.py"))
rr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rr)

D = datetime.date

# A README the way main's looked before v4.2.0 was released.
OLD = """# shint

**that SHIt Network Tool** - the network checks you run every day, in one small program.

[![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Commands

| Command | What it answers |
|---|---|
| `shint telnet <host> <port>` | Can I open a TCP connection? |
| `shint ping <host>` | Is this host reachable? |
| `shint listen tcp\\|udp\\|http <port>` | Run a local test server. |

## Roadmap

Intro text that must not change.

| Release | Sprint | What is coming |
|---|---|---|
| **v4.0.4** | Released Sep 20 | Fixes for ping |
| **v4.1.0** | Oct 5 - Oct 18 | web --timing, wol |
| **v4.2.0** | Oct 19 - Nov 1 | ip, dns |
| **v4.3.0** | Nov 2 - Nov 15 | tls |
| **v4.5.x** | After Dec 13 | Patch releases only |
| **v5.0.0** | Not scheduled | The next major release |
| **Later** | | Package managers |

Text after the table.

## License

[MIT](LICENSE)
"""

CHANGELOG = """# Changelog

## v4.2.1 - 2026-10-02

Two fixes found after v4.2.0.

- **A fix.** Details.

## v4.2.0 - 2026-09-21

New commands ip and dns, and fixes.

- **A bullet.** Text.

## v4.0.5 - 2026-09-20

- **`Ctrl+C` on a repeating check now shows the summary.** More.
"""

HELP = """Usage:
  shint [command]

Available Commands:
  cidr        Work out a subnet: network, mask, range and size
  completion  Generate the autocompletion script
  dns         Look up DNS records (A | AAAA) like dig
  help        Help about any command
  listen      Start a local listener
  ping        Send ICMP ECHO_REQUEST to a host
  telnet      Connect to a host on a specific port

Flags:
  -h, --help   help for shint
"""

TAGS = {"v4.0.3": D(2026, 9, 20), "v4.0.4": D(2026, 9, 20), "v4.2.0": D(2026, 9, 21)}


def put(path, text):
    with open(path, "w") as f:
        f.write(text)


def get(path):
    with open(path) as f:
        return f.read()


def changelog():
    with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False) as f:
        f.write(CHANGELOG)
    try:
        return rr.read_changelog(f.name)
    finally:
        os.unlink(f.name)


def run(text=OLD, tags=TAGS, milestones=None, commands=None, only_tag=None):
    return rr.reconcile(text, tags, changelog(), milestones, commands, only_tag)


def row(text, version):
    for line in text.split("\n"):
        if line.startswith("| **%s**" % version):
            return line
    return None


class Roadmap(unittest.TestCase):
    def test_a_planned_row_becomes_released_on_the_tags_date(self):
        new, changes, _ = run()
        self.assertEqual(row(new, "v4.2.0"), "| **v4.2.0** | Released Sep 21 | ip, dns |")
        self.assertTrue(any("v4.2.0" in c and "Released Sep 21" in c and "Oct 19 - Nov 1" in c for c in changes), changes)

    def test_the_plan_text_is_kept_and_the_changelog_summary_offered_for_review(self):
        _, _, warnings = run()
        self.assertTrue(any("v4.2.0" in w and "New commands ip and dns, and fixes." in w for w in warnings), warnings)

    def test_a_tag_without_a_row_gets_one_in_version_order_with_the_changelog_summary(self):
        tags = dict(TAGS, **{"v4.2.1": D(2026, 10, 2)})
        new, changes, _ = run(tags=tags)
        self.assertEqual(row(new, "v4.2.1"), "| **v4.2.1** | Released Oct 2 | Two fixes found after v4.2.0. |")
        lines = new.split("\n")
        self.assertLess(lines.index(row(new, "v4.2.0")), lines.index(row(new, "v4.2.1")))
        self.assertLess(lines.index(row(new, "v4.2.1")), lines.index(row(new, "v4.3.0")))
        self.assertTrue(any("Row added for **v4.2.1**" in c for c in changes))

    def test_a_patch_row_goes_before_the_patch_plan_row(self):
        tags = dict(TAGS, **{"v4.5.1": D(2026, 12, 20)})
        new, _, _ = run(tags=tags)
        lines = new.split("\n")
        self.assertLess(lines.index(row(new, "v4.5.1")), lines.index(row(new, "v4.5.x")))
        self.assertLess(lines.index(row(new, "v4.3.0")), lines.index(row(new, "v4.5.1")))

    def test_the_first_bullet_is_the_summary_when_a_section_has_no_opening_sentence(self):
        text = OLD.replace("| **v4.0.4** | Released Sep 20 | Fixes for ping |", "| **v4.0.4** | Released Sep 20 | Fixes for ping |")
        tags = dict(TAGS, **{"v4.0.5": D(2026, 9, 20)})
        new, _, _ = run(text=text, tags=tags)
        self.assertEqual(row(new, "v4.0.5"), "| **v4.0.5** | Released Sep 20 | `Ctrl+C` on a repeating check now shows the summary. |")

    def test_a_tag_with_no_changelog_section_still_gets_a_row_and_a_warning(self):
        tags = dict(TAGS, **{"v4.9.9": D(2027, 1, 1)})
        new, _, warnings = run(tags=tags)
        self.assertIn("See the [changelog](CHANGELOG.md).", row(new, "v4.9.9"))
        self.assertTrue(any("v4.9.9 has no section in CHANGELOG.md" in w for w in warnings))

    def test_a_wrong_date_is_corrected_to_the_tags(self):
        new, changes, _ = run(tags={"v4.0.4": D(2026, 9, 19)})
        self.assertEqual(row(new, "v4.0.4"), "| **v4.0.4** | Released Sep 19 | Fixes for ping |")
        self.assertTrue(any("date was corrected (Released Sep 20 -> Released Sep 19)" in c for c in changes))

    def test_tags_older_than_the_table_are_ignored(self):
        new, _, _ = run(tags={"v3.1.0": D(2025, 1, 1), "v4.0.3": D(2026, 9, 20)})
        self.assertNotIn("v3.1.0", new)
        self.assertNotIn("**v4.0.3**", new)

    def test_only_tag_limits_the_work_to_that_tag(self):
        tags = dict(TAGS, **{"v4.2.1": D(2026, 10, 2)})
        new, _, _ = run(tags=tags, only_tag="v4.2.1")
        self.assertIsNotNone(row(new, "v4.2.1"))
        self.assertIn("Oct 19 - Nov 1", row(new, "v4.2.0"))   # v4.2.0 was not asked for

    def test_planned_rows_follow_their_milestones_and_nothing_else_does(self):
        ms = {"v4.3.0": D(2026, 11, 22), "v4.0.4": D(2026, 1, 1)}
        new, changes, _ = run(milestones=ms)
        self.assertEqual(row(new, "v4.3.0"), "| **v4.3.0** | Nov 9 - Nov 22 | tls |")
        self.assertIn("Released Sep 20", row(new, "v4.0.4"))       # a released row keeps its date
        self.assertIn("After Dec 13", row(new, "v4.5.x"))          # a patch plan row has no window
        self.assertIn("Not scheduled", row(new, "v5.0.0"))
        self.assertTrue(any("v4.3.0" in c and "milestone" in c for c in changes))

    def test_warnings_for_what_cannot_be_known(self):
        tags = {"v4.0.4": D(2026, 9, 20), "v4.2.0": D(2026, 9, 21)}
        text = OLD.replace("| **v4.3.0** | Nov 2 - Nov 15 | tls |", "| **v4.3.0** | Released Nov 15 | tls |")
        _, _, warnings = run(text=text, tags=tags, milestones={"v4.2.0": D(2026, 11, 1)})
        joined = "\n".join(warnings)
        self.assertIn("**v4.3.0** is marked Released but there is no tag", joined)
        self.assertIn("**v4.1.0** is still planned although v4.2.0 has been released", joined)
        self.assertIn("v4.2.0 was released 28 days ahead of its sprint window (Oct 19 - Nov 1)", joined)

    def test_no_tag_warnings_when_no_tags_are_known(self):
        _, _, warnings = run(tags={})
        self.assertEqual(warnings, [])

    def test_a_pipe_in_a_summary_does_not_break_the_row(self):
        cl = {"v4.2.1": {"date": "2026-10-02", "summary": "a | b"}}
        new, _, _ = rr.reconcile(OLD, dict(TAGS, **{"v4.2.1": D(2026, 10, 2)}), cl)
        self.assertEqual(row(new, "v4.2.1"), "| **v4.2.1** | Released Oct 2 | a \\| b |")

    def test_a_changelog_date_that_differs_from_the_tag_is_reported_and_the_tag_wins(self):
        cl = {"v4.2.0": {"date": "2026-09-22", "summary": "x"}}
        new, _, warnings = rr.reconcile(OLD, TAGS, cl)
        self.assertIn("Released Sep 21", row(new, "v4.2.0"))
        self.assertTrue(any("changelog is dated 2026-09-22 but the tag is dated 2026-09-21" in w for w in warnings))


class Invariants(unittest.TestCase):
    def test_the_expansion_badges_donate_button_and_support_line_come_back(self):
        new, changes, _ = run()
        self.assertIn("**Simple Host INspection Toolkit**", new)
        self.assertNotIn("SHIt", new)
        for key, _ in rr.BADGES:
            self.assertIn(key, new, key)
        self.assertIn("[![Donate]", new)
        self.assertTrue(new.rstrip().endswith("[support its development](%s)." % rr.SUPPORT_URL))
        self.assertTrue(any("Donate" in c for c in changes) and any("support" in c for c in changes))

    def test_badges_are_added_under_the_existing_ones_in_order(self):
        new, _, _ = run()
        badges = [l for l in new.split("\n") if l.startswith("[![")]
        self.assertEqual(len(badges), len(rr.BADGES))
        self.assertEqual([b for b in badges], [md for _, md in rr.BADGES])

    def test_present_elements_are_left_alone(self):
        once, _, _ = run()
        twice, changes, warnings = rr.reconcile(once, TAGS, changelog())
        self.assertEqual(once, twice)
        self.assertEqual(changes, [])

    def test_a_readme_with_no_badges_gets_a_badge_row_under_the_tagline(self):
        bare = OLD.replace("[![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest)\n", "") \
                  .replace("[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)\n", "")
        new, _, _ = run(text=bare)
        lines = new.split("\n")
        tag = next(i for i, l in enumerate(lines) if l.startswith("**Simple"))
        self.assertEqual(lines[tag + 1], "")
        self.assertTrue(lines[tag + 2].startswith("[![Latest release]"))

    def test_a_tagline_it_does_not_know_is_left_alone_with_a_warning(self):
        text = OLD.replace("**that SHIt Network Tool**", "**Something else entirely**")
        new, _, warnings = run(text=text)
        self.assertIn("**Something else entirely**", new)
        self.assertTrue(any("tagline" in w for w in warnings))

    def test_an_existing_support_line_is_not_duplicated_wherever_it_is(self):
        text = OLD.replace("## License", "Please [support it](%s) if you like.\n\n## License" % rr.SUPPORT_URL)
        new, changes, _ = run(text=text)
        self.assertEqual(new.count(rr.SUPPORT_URL), 2)   # the donate badge + the sentence already there
        self.assertFalse(any("support (donate) line" in c for c in changes))


class Commands(unittest.TestCase):
    def test_a_command_the_table_lacks_gets_a_row_before_listen(self):
        cmds = rr.read_commands(self.help_file())
        new, changes, _ = run(commands=cmds)
        lines = new.split("\n")
        i = lines.index("| `shint cidr` | Work out a subnet: network, mask, range and size |")
        self.assertLess(i, next(k for k, l in enumerate(lines) if l.startswith("| `shint listen")))
        self.assertTrue(any("`shint cidr`" in c for c in changes))

    def test_existing_commands_and_help_and_completion_are_skipped_and_a_pipe_is_escaped(self):
        new, _, _ = run(commands=rr.read_commands(self.help_file()))
        self.assertEqual(new.count("`shint telnet"), 1)
        self.assertEqual(new.count("`shint ping"), 1)
        self.assertNotIn("`shint help`", new)
        self.assertNotIn("`shint completion`", new)
        self.assertIn("| `shint dns` | Look up DNS records (A \\| AAAA) like dig |", new)

    def test_it_is_idempotent_for_the_commands_table(self):
        cmds = rr.read_commands(self.help_file())
        once, _, _ = run(commands=cmds)
        twice, changes, _ = rr.reconcile(once, TAGS, changelog(), None, cmds)
        self.assertEqual(once, twice)
        self.assertEqual(changes, [])

    def help_file(self):
        f = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
        f.write(HELP)
        f.close()
        self.addCleanup(os.unlink, f.name)
        return f.name


class Everything(unittest.TestCase):
    def test_a_full_run_is_idempotent_and_touches_nothing_it_should_not(self):
        once, _, _ = run(commands=[("cidr", "Work out a subnet")], milestones={"v4.3.0": D(2026, 11, 15)})
        twice, changes, _ = rr.reconcile(once, TAGS, changelog(), {"v4.3.0": D(2026, 11, 15)}, [("cidr", "Work out a subnet")])
        self.assertEqual(once, twice)
        self.assertEqual(changes, [])
        for untouched in ("Intro text that must not change.", "Text after the table.", "## License", "[MIT](LICENSE)", "| Release | Sprint | What is coming |"):
            self.assertIn(untouched, once)
        # the rows it was not asked about are byte for byte what they were
        for v in ("v4.0.4", "v4.1.0", "v4.3.0", "v4.5.x", "v5.0.0"):
            self.assertEqual(row(once, v), row(OLD, v), v)
        self.assertIn("| **Later** | | Package managers |", once)   # an empty cell stays empty

    def test_a_readme_without_a_roadmap_is_refused(self):
        with self.assertRaises(rr.Unreadable):
            rr.reconcile(OLD.replace("## Roadmap", "## Plans"), TAGS, {})

    def test_a_roadmap_with_the_wrong_columns_is_refused(self):
        with self.assertRaises(rr.Unreadable):
            rr.reconcile(OLD.replace("| Release | Sprint | What is coming |\n|---|---|---|", "| Release | What is coming |\n|---|---|"), TAGS, {})

    def test_a_roadmap_with_no_release_rows_is_refused(self):
        bare = "\n".join(l for l in OLD.split("\n") if not l.startswith("| **"))
        with self.assertRaises(rr.Unreadable):
            rr.reconcile(bare, TAGS, {})

    def test_the_help_text_must_have_a_command_list(self):
        with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as f:
            f.write("nothing useful\n")
        self.addCleanup(os.unlink, f.name)
        with self.assertRaises(rr.Unreadable):
            rr.read_commands(f.name)


class CommandLine(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp()
        self.readme = os.path.join(self.dir, "readme.md")
        self.changelog = os.path.join(self.dir, "CHANGELOG.md")
        self.tags = os.path.join(self.dir, "tags.txt")
        put(self.readme, OLD)
        put(self.changelog, CHANGELOG)
        put(self.tags, "# tag date\nv4.0.4 2026-09-20\nv4.2.0 2026-09-21\nnot-a-tag 2026-01-01\n")

    def main(self, *args):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = rr.main(["--readme", self.readme, "--changelog", self.changelog, "--tags-file", self.tags] + list(args))
        return rc, out.getvalue(), err.getvalue()

    def test_it_writes_the_readme_and_a_report(self):
        report = os.path.join(self.dir, "report.md")
        rc, out, _ = self.main("--report", report)
        self.assertEqual(rc, 0)
        self.assertIn("Released Sep 21", get(self.readme))
        self.assertEqual(get(report), out)
        self.assertIn("## Changes", out)
        self.assertIn("## Needs a human look", out)

    def test_check_changes_nothing_and_says_whether_there_is_work(self):
        rc, _, _ = self.main("--check")
        self.assertEqual(rc, 1)
        self.assertEqual(get(self.readme), OLD)
        self.assertEqual(self.main()[0], 0)
        rc, out, _ = self.main("--check")
        self.assertEqual(rc, 0)
        self.assertNotIn("## Changes", out)

    def test_a_second_run_reports_nothing_to_do_for_the_changes(self):
        self.main()
        rc, out, _ = self.main()
        self.assertEqual(rc, 0)
        self.assertNotIn("## Changes", out)

    def test_bad_input_exits_2_and_writes_nothing(self):
        put(self.readme, "# no roadmap here\n")
        rc, _, err = self.main()
        self.assertEqual(rc, 2)
        self.assertIn("Roadmap", err)
        self.assertEqual(get(self.readme), "# no roadmap here\n")
        self.assertEqual(self.main("--tag", "4.2.0")[0], 2)
        self.assertEqual(rr.main(["--readme", os.path.join(self.dir, "missing.md"), "--tags-file", self.tags]), 2)

    def test_an_unknown_tag_is_refused(self):
        rc, _, err = self.main("--tag", "v9.9.9")
        self.assertEqual(rc, 2)
        self.assertIn("v9.9.9", err)

    def test_milestones_file_is_read(self):
        ms = os.path.join(self.dir, "ms.json")
        put(ms, json.dumps([{"title": "v4.3.0", "due_on": "2026-11-15T00:00:00Z", "state": "open"}, {"title": "no due date", "due_on": None}]))
        self.assertEqual(self.main("--milestones", ms)[0], 0)
        self.assertIn("| **v4.3.0** | Nov 2 - Nov 15 | tls |", get(self.readme))
        put(ms, "not json")
        self.assertEqual(self.main("--milestones", ms)[0], 2)


class GitTags(unittest.TestCase):
    """The date is the tag's date in the tagger's own timezone, not UTC."""

    def test_the_tag_date_is_the_taggers_local_date(self):
        env = dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@example.com", GIT_COMMITTER_NAME="t",
                   GIT_COMMITTER_EMAIL="t@example.com", GIT_CONFIG_GLOBAL="/dev/null", GIT_CONFIG_SYSTEM="/dev/null")
        with tempfile.TemporaryDirectory() as d:
            def git(*a, **kw):
                return subprocess.run(["git"] + list(a), cwd=d, env=dict(env, **kw), check=True, capture_output=True, text=True)
            git("init", "-q", "-b", "main")
            put(os.path.join(d, "f"), "x")
            git("add", "-A")
            git("commit", "-q", "-m", "c")
            # 23:19 on Sep 20 in UTC-6 is already Sep 21 in UTC
            git("tag", "-a", "v4.0.5", "-m", "v4.0.5", GIT_COMMITTER_DATE="2026-09-20T23:19:12-0600")
            git("tag", "v4.0.6")   # lightweight: the commit's date
            git("tag", "not-a-release")
            cwd = os.getcwd()
            os.chdir(d)
            try:
                tags = rr.read_git_tags()
            finally:
                os.chdir(cwd)
        self.assertEqual(tags["v4.0.5"], D(2026, 9, 20))
        self.assertIn("v4.0.6", tags)
        self.assertNotIn("not-a-release", tags)


if __name__ == "__main__":
    unittest.main(verbosity=1)
