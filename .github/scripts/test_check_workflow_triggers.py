"""Tests for check-workflow-triggers.py:  python3 .github/scripts/test_check_workflow_triggers.py"""
import importlib.util
import os
import unittest

here = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("triggers", os.path.join(here, "check-workflow-triggers.py"))
triggers = importlib.util.module_from_spec(spec)
spec.loader.exec_module(triggers)


def problems(on_block):
    """Problems for a workflow whose `on:` block is on_block (a str)."""
    return triggers.problems_in(triggers.parse_on_block("name: x\n" + on_block + "\njobs:\n  a:\n    runs-on: x\n"))


RELEASE = """on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'
"""


class Accepted(unittest.TestCase):
    def test_release_tag_trigger(self):
        self.assertEqual(problems(RELEASE), [])

    def test_reusable_workflow_only(self):
        self.assertEqual(problems("on:\n  workflow_call:\n"), [])

    def test_push_to_main_ignoring_dot_github(self):
        block = "on:\n  push:\n    branches:\n      - main\n    paths-ignore:\n      - '.github/**'\n"
        self.assertEqual(problems(block), [])

    def test_flow_lists_and_comments(self):
        block = "on:  # what starts it\n  push:\n    branches: [main]\n    paths-ignore: ['.github/**']  # workflows only\n"
        self.assertEqual(problems(block), [])


class Refused(unittest.TestCase):
    def assert_refused(self, block, fragment):
        found = problems(block)
        self.assertTrue(any(fragment in f for f in found), "%r not in %r" % (fragment, found))

    def test_the_old_loose_tag_pattern(self):
        self.assert_refused("on:\n  push:\n    tags:\n      - 'v*.*.*'\n", "must be exactly")

    def test_a_second_tag_pattern(self):
        self.assert_refused(RELEASE + "      - 'release/**'\n", "must be exactly")

    def test_pull_request(self):
        self.assert_refused("on:\n  pull_request:\n    branches: [main]\n", "`pull_request` is not allowed")

    def test_schedule_and_manual_runs(self):
        self.assert_refused("on:\n  schedule:\n    - cron: '0 0 * * 0'\n", "`schedule` is not allowed")
        self.assert_refused("on:\n  workflow_dispatch:\n", "`workflow_dispatch` is not allowed")

    def test_release_branches(self):
        self.assert_refused("on:\n  push:\n    branches:\n      - 'release/**'\n      - main\n    paths-ignore: ['.github/**']\n", "must be exactly [main]")

    def test_main_without_ignoring_dot_github(self):
        self.assert_refused("on:\n  push:\n    branches: [main]\n", "paths-ignore")

    def test_main_ignoring_something_else(self):
        self.assert_refused("on:\n  push:\n    branches: [main]\n    paths-ignore: ['docs/**']\n", "paths-ignore")

    def test_bare_push_runs_everywhere(self):
        self.assert_refused("on:\n  push:\n", "bare `push:`")

    def test_tags_mixed_with_branches(self):
        self.assert_refused(RELEASE + "    branches: [main]\n", "must not be combined")

    def test_reusable_workflow_with_a_second_trigger(self):
        self.assert_refused("on:\n  workflow_call:\n  push:\n    tags:\n      - 'v[0-9]+.[0-9]+.[0-9]+'\n", "no other trigger")

    def test_unsupported_push_keys(self):
        self.assert_refused(RELEASE.replace("tags:", "tags-ignore:"), "unsupported keys")


class Unreadable(unittest.TestCase):
    """A shape the reader does not understand is an error, never a pass."""

    def test_inline_on(self):
        with self.assertRaises(ValueError):
            triggers.parse_on_block("on: push\njobs: {}\n")
        with self.assertRaises(ValueError):
            triggers.parse_on_block("on: [push, pull_request]\njobs: {}\n")

    def test_no_on_block(self):
        with self.assertRaises(ValueError):
            triggers.parse_on_block("name: x\njobs: {}\n")


class RealWorkflows(unittest.TestCase):
    def test_every_workflow_in_the_repository_follows_the_rule(self):
        root = os.path.normpath(os.path.join(here, "..", "workflows"))
        names = sorted(n for n in os.listdir(root) if n.endswith((".yaml", ".yml")))
        self.assertGreaterEqual(len(names), 6)  # five release workflows + the tag guard
        for name in names:
            self.assertEqual(triggers.check_file(os.path.join(root, name)), [], name)


if __name__ == "__main__":
    unittest.main()
