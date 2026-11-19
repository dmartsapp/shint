#!/usr/bin/env python3
"""Tests for test/platforms/platforms.py: the scripts it makes, and how it reads results files."""

import os
import shutil
import subprocess
import sys
import tempfile
import unittest
import urllib.error

sys.dont_write_bytecode = True
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import platforms as P  # noqa: E402

SHA = "9190a2377e65b60ee0b099f426145e4b02b7d7128f7ecfad3b5007da00d26ea2"


def platform(slug):
    return next(p for p in P.platforms() if p.slug == slug)


def filled(platform_name="linux/amd64", release="v4.3.0", passed=2, failed=1, skipped=1,
           result="fail", lines=None, sha=SHA, drop=None):
    fields = {"platform": platform_name, "release": release, "commit": "b245cc4", "built": "2026-11-17T19:49:21+0000",
              "go": "go1.27.4", "binary": "shint.linux.amd64", "binary_sha256": sha,
              "tested_at": "2026-11-18T21:27:40Z", "os": "Linux 6.12, Debian GNU/Linux 13 (trixie)",
              "privileges": "normal user", "internet": "yes", "passed": str(passed), "failed": str(failed),
              "skipped": str(skipped), "result": result}
    if drop:
        del fields[drop]
    if lines is None:
        lines = ["PASS  version: --version names linux/amd64 (v4.3.0)",
                 "PASS  help: lists the commands",
                 "FAIL  ss -l: finds the TCP listener and its process",
                 "      exit=2 (wanted 0), wanted a line matching: pid=1( |$)",
                 "      Error: unknown command \"ss\" for \"shint\"",
                 "SKIP  ntp: pool.ntp.org answers",
                 "      no NTP answer - UDP port 123 may be blocked here (exit=1)"]
    head = ["# shint platform test: %s" % platform_name, "# Written by test/platforms/linux-amd64.sh"]
    return "\n".join(head + ["%s: %s" % (k, v) for k, v in fields.items()] + [""] + lines) + "\n"


class Platforms(unittest.TestCase):
    def test_every_makefile_platform_has_a_test(self):
        self.assertEqual(P.makefile_platforms(), sorted(p.name for p in P.platforms()))

    def test_names(self):
        w = platform("windows-arm64")
        self.assertEqual((w.asset, w.script, w.results), ("shint.windows.arm64.exe", "windows-arm64.ps1",
                                                          os.path.join("results", "windows-arm64.txt")))
        a = platform("linux-arm")
        self.assertEqual((a.name, a.asset, a.script), ("linux/arm", "shint.linux.arm", "linux-arm.sh"))

    def test_the_expectations_follow_the_build_constraints(self):
        by = {p.slug: p for p in P.platforms()}
        self.assertEqual(by["aix-ppc64"].ss, "absent")            # main_ss.go: //go:build !aix
        self.assertEqual({p.ss for p in P.platforms() if p.goos in ("linux", "android", "windows")}, {"yes"})
        self.assertEqual({p.route for p in P.platforms() if p.goos in ("aix", "solaris", "illumos")}, {"no"})
        self.assertEqual({p.ping for p in P.platforms() if p.goos in ("linux", "android")}, {"sysctl"})
        self.assertEqual(by["freebsd-amd64"].ping, "no")             # seen on FreeBSD 14.4 (#112)
        self.assertEqual({p.ping for p in P.platforms() if p.goos in ("darwin", "windows")}, {"yes"})


class Render(unittest.TestCase):
    def test_a_unix_script_has_its_settings_and_no_markers(self):
        text = P.render(platform("openbsd-arm64"))
        self.assertTrue(text.startswith("#!/bin/sh\n# shint platform test for openbsd/arm64"))
        for line in ("PLATFORM=openbsd/arm64\n", "ASSET=shint.openbsd.arm64\n", "EXPECT_SS=no ", "EXPECT_ROUTE=yes "):
            self.assertIn(line, text)
        self.assertIn("#   sh openbsd-arm64.sh ./shint.openbsd.arm64\n", text)
        self.assertNotIn("@@", text)

    def test_a_windows_script_is_powershell(self):
        text = P.render(platform("windows-amd64"))
        self.assertIn("$Platform = 'windows/amd64'\n", text)
        self.assertIn("$Asset = 'shint.windows.amd64.exe'\n", text)
        self.assertIn("powershell -ExecutionPolicy Bypass -File .\\windows-amd64.ps1 .\\shint.windows.amd64.exe", text)
        self.assertNotIn("@@", text)

    def test_the_settings_come_before_the_code(self):
        text = P.render(platform("linux-amd64"))
        self.assertLess(text.index("PLATFORM=linux/amd64"), text.index("set -u"))
        text = P.render(platform("windows-amd64"))
        self.assertLess(text.index("$Platform = "), text.index("$ErrorActionPreference"))

    def test_no_parenthesis_in_a_parameter_expansion(self):
        # OpenBSD's ksh cannot parse ${v%% (*} (found on OpenBSD 8.0): dash and bash can,
        # so sh -n here would not catch it.
        import re
        with open(os.path.join(P.HERE, "template.sh")) as f:
            for n, line in enumerate(f, 1):
                if not line.lstrip().startswith("#"):
                    self.assertIsNone(re.search(r"\$\{[^}]*[()]", line), "template.sh:%d: %s" % (n, line.strip()))

    @unittest.skipUnless(shutil.which("sh"), "no sh")
    def test_every_unix_script_parses(self):
        for p in P.platforms():
            if p.windows:
                continue
            r = subprocess.run(["sh", "-n", os.path.join(P.HERE, p.script)], capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, "%s: %s" % (p.script, r.stderr))

    def test_blank_results_are_valid_and_untested(self):
        p = platform("solaris-amd64")
        text = P.blank_results(p)
        self.assertEqual(P.validate_results(p, text), [])
        fields, checks = P.parse_results(text)
        self.assertEqual((fields["platform"], fields["result"], fields["passed"], checks), ("solaris/amd64", "untested", "", []))


class Results(unittest.TestCase):
    def setUp(self):
        self.p = platform("linux-amd64")

    def test_a_filled_file_is_valid(self):
        self.assertEqual(P.validate_results(self.p, filled()), [])
        self.assertEqual(P.validate_results(self.p, filled(failed=0, result="pass",
                                                           lines=["PASS  a", "PASS  b", "SKIP  c"])), [])

    def test_a_byte_order_mark_and_crlf_are_read(self):
        self.assertEqual(P.validate_results(self.p, "﻿" + filled().replace("\n", "\r\n")), [])

    def test_counts_must_match_the_lines(self):
        errors = P.validate_results(self.p, filled(passed=5))
        self.assertEqual(errors, ["passed says 5 but there are 2 PASS lines"])

    def test_result_must_follow_the_failures(self):
        self.assertIn("result is pass with 1 failed", P.validate_results(self.p, filled(result="pass")))
        self.assertIn("result is fail with 0 failed",
                      P.validate_results(self.p, filled(failed=0, lines=["PASS  a", "PASS  b", "SKIP  c"])))

    def test_the_platform_must_be_the_files(self):
        self.assertIn("platform is 'linux/arm64', not linux/amd64", P.validate_results(self.p, filled("linux/arm64")))

    def test_a_missing_field(self):
        self.assertEqual(P.validate_results(self.p, filled(drop="binary_sha256")), ["missing binary_sha256"])

    def test_bad_values(self):
        errors = P.validate_results(self.p, filled(sha="abc", release=""))
        self.assertIn("binary_sha256 is not a SHA-256", errors)
        self.assertIn("no release", errors)
        self.assertEqual(P.validate_results(self.p, filled(sha="unknown")), [])

    def test_untested_with_results_is_wrong(self):
        text = P.blank_results(self.p).replace("release:", "release: v4.4.0")
        self.assertEqual(P.validate_results(self.p, text), ["result is untested but it has results (release)"])


class Tree(unittest.TestCase):
    """check() and summary() over a copy of test/platforms."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.here = os.path.join(self.tmp, "platforms")
        os.mkdir(self.here)
        for name in ("template.sh", "template.ps1"):
            shutil.copy(os.path.join(P.HERE, name), self.here)
        P.generate(self.here, log=lambda *_: None)

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def test_the_repository_is_current(self):
        self.assertEqual(P.check(), [])

    def test_a_fresh_tree_is_clean(self):
        self.assertEqual(P.check(self.here), [])

    def test_a_hand_edited_script_is_caught(self):
        with open(os.path.join(self.here, "freebsd-amd64.sh"), "a") as f:
            f.write("echo edited\n")
        self.assertEqual(P.check(self.here), ["freebsd-amd64.sh is not what its template makes (make platform-tests)"])

    def test_stray_and_missing_files_are_caught(self):
        os.remove(os.path.join(self.here, "results", "aix-ppc64.txt"))
        open(os.path.join(self.here, "plan9-amd64.sh"), "w").close()
        open(os.path.join(self.here, "results", "plan9-amd64.txt"), "w").close()
        self.assertEqual(P.check(self.here), [
            "results/aix-ppc64.txt is missing (make platform-tests)",
            "plan9-amd64.sh is not the script of any platform",
            "results/plan9-amd64.txt is not the results file of any platform"])

    def test_generate_never_overwrites_results(self):
        path = os.path.join(self.here, "results", "linux-amd64.txt")
        with open(path, "w") as f:
            f.write(filled())
        P.generate(self.here, log=lambda *_: None)
        with open(path) as f:
            self.assertEqual(f.read(), filled())
        self.assertEqual(P.check(self.here), [])

    def test_summary_and_verify(self):
        with open(os.path.join(self.here, "results", "linux-amd64.txt"), "w") as f:
            f.write(filled())
        with open(os.path.join(self.here, "results", "darwin-arm64.txt"), "w") as f:
            f.write(filled("darwin/arm64", release="v4.4.0", sha="0" * 64))
        with open(os.path.join(self.here, "results", "freebsd-amd64.txt"), "w") as f:
            f.write(filled("freebsd/amd64", release="v9.9.9"))
        published = {("v4.3.0", "shint.linux.amd64"): SHA, ("v4.4.0", "shint.darwin.arm64"): "f" * 64}

        def fetch(release, asset):
            if (release, asset) not in published:
                raise urllib.error.HTTPError("url", 404, "Not Found", None, None)
            return published[(release, asset)]

        lines = []
        code = P.summary(self.here, verify=True, fetch=fetch, out=lines.append)
        self.assertEqual(code, 1)   # a mismatch
        text = "\n".join(lines)
        self.assertRegex(text, r"linux/amd64 +v4\.3\.0 +fail +2/1/1 +2026-11-18 +verified")
        self.assertRegex(text, r"darwin/arm64 +v4\.4\.0 +fail +2/1/1 +2026-11-18 +MISMATCH: not v4\.4\.0's published binary")
        self.assertRegex(text, r"freebsd/amd64 .*v9\.9\.9 publishes no shint\.freebsd\.amd64 \(not released yet\?\)")
        self.assertRegex(text, r"aix/ppc64 +- +untested +- +- +-")
        self.assertTrue(text.endswith("3 of 18 platforms tested, 0 passed"))

    def test_a_dev_build_is_not_verified(self):
        p = platform("linux-amd64")
        self.assertEqual(P.provenance(p, {"release": "4.4.0", "binary_sha256": SHA}, None), "not a release build")
        self.assertEqual(P.provenance(p, {"release": "v4.4.0", "binary_sha256": "unknown"}, None), "no SHA-256")


if __name__ == "__main__":
    unittest.main()
