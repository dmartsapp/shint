"""Tests for check-pins.py."""
import importlib.util
import os
import sys
import unittest

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("check_pins", os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-pins.py"))
check_pins = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check_pins)

SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64


class Workflows(unittest.TestCase):
    def problems(self, text):
        return check_pins.check_workflow("w.yaml", text)

    def test_pinned_and_local_pass(self):
        self.assertEqual(self.problems(
            "steps:\n"
            "  - uses: actions/checkout@%s # v5.1.0\n"
            "    with: {fetch-depth: 0}\n"
            "  - uses: github/codeql-action/init@%s\n"
            "jobs:\n  verify:\n    uses: ./.github/workflows/verify-release-tag.yaml\n"
            "  - uses: docker://alpine@%s\n"
            "  - uses: 'actions/setup-go@%s'\n" % (SHA, SHA, DIGEST, SHA)), [])

    def test_tags_branches_and_short_shas_fail(self):
        for ref in ["actions/checkout@v5", "actions/checkout@main", "actions/checkout@v5.1.0",
                    "actions/checkout@" + "a" * 7, "actions/checkout@" + "A" * 40, "docker://alpine:3.20", "actions/checkout"]:
            self.assertEqual(len(self.problems("      - uses: %s\n" % ref)), 1, ref)

    def test_the_line_is_named(self):
        p = self.problems("on: push\n\n  - uses: actions/checkout@v5\n")
        self.assertIn("w.yaml:3:", p[0])


class Dockerfiles(unittest.TestCase):
    def problems(self, text):
        return check_pins.check_dockerfile("Dockerfile", text)

    def test_digests_stages_and_scratch_pass(self):
        self.assertEqual(self.problems(
            "FROM golang:1.27.4-alpine@%s AS builder\n"
            "FROM --platform=$BUILDPLATFORM gcr.io/distroless/static-debian12:nonroot@%s\n"
            "FROM builder AS test\n"
            "from scratch\n" % (DIGEST, DIGEST)), [])

    def test_tags_fail(self):
        for image in ["golang:1.27.4-alpine", "gcr.io/distroless/static-debian12:nonroot", "alpine", "alpine@sha256:abc"]:
            self.assertEqual(len(self.problems("FROM %s AS x\n" % image)), 1, image)


class TheRepository(unittest.TestCase):
    def test_this_repository_is_pinned(self):
        root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
        cwd = os.getcwd()
        os.chdir(root)
        try:
            self.assertEqual(check_pins.main([]), 0)
        finally:
            os.chdir(cwd)


if __name__ == "__main__":
    unittest.main()
