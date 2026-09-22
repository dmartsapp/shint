#!/usr/bin/env python3
"""Tests for check-report.py: the report is built from what make check printed, signed with a
real SSH key (a throw-away one, made here), and refused when anything about it is off.

    python3 .github/scripts/test_check_report.py
"""
import datetime
import importlib.util
import io
import json
import os
import shutil
import subprocess
import tempfile
import unittest
import contextlib

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("check_report", os.path.join(HERE, "check-report.py"))
cr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cr)

TREE = "a" * 40
OTHER_TREE = "b" * 40

LOG = """==> gofmt
==> go vet
    go vet (GOOS=windows)
    go vet (GOOS=freebsd)
==> go test -race
ok  \tgithub.com/x/shint/v4\t47.9s
ok  \tgithub.com/x/shint/v4/lib\t1.0s
?   \tgithub.com/x/shint/v4/test/battery/gencert\t[no test files]
==> black-box battery
battery: 445 cases against /tmp/x/shint

known issues still failing (3 cases, #37 x3):

battery: 445 cases: 442 passed, 3 known issues, 0 skipped, 0 FAILED
==> golangci-lint
0 issues.
==> govulncheck
No vulnerabilities found
==> documentation
==> workflows
==> all checks passed
"""


def need_ssh_keygen(test):
    return unittest.skipUnless(shutil.which("ssh-keygen"), "ssh-keygen is not installed")(test)


class Keys:
    """Throw-away ed25519 keys and an allowed-signers file naming some of them."""

    def __init__(self, tmp):
        self.tmp = tmp

    def make(self, name):
        path = os.path.join(self.tmp, name)
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", path], check=True)
        return path

    def allow(self, entries, filename="allowed_signers"):
        """entries: [(principal, key path)] -> the allowed_signers file the way git and ssh read it"""
        path = os.path.join(self.tmp, filename)
        with open(path, "w") as f:
            for principal, key in entries:
                with open(key + ".pub") as p:
                    f.write('%s namespaces="%s" %s\n' % (principal, cr.NAMESPACE, p.read().strip()))
        return path


@need_ssh_keygen
class SignAndVerify(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.keys = Keys(self.tmp)
        self.key = self.keys.make("maintainer")
        self.allowed = self.keys.allow([("farhansabbir", self.key)])
        self.when = cr.now()

    def report(self, **changes):
        r = cr.build_report(LOG, commit="c" * 40, tree=TREE, when=self.when)
        r.update(changes)
        return r

    def note(self, report=None, key=None):
        return cr.sign(cr.report_bytes(report or self.report()), key or self.key)

    def bad(self, note, needle, tree=TREE, **kw):
        with self.assertRaises(cr.Bad) as cm:
            cr.verify(note, kw.pop("allowed", self.allowed), tree, **kw)
        self.assertIn(needle, str(cm.exception))

    def test_a_signed_report_for_this_tree_is_valid(self):
        msg = cr.verify(self.note(), self.allowed, TREE)
        self.assertIn("signed by farhansabbir", msg)
        self.assertIn(TREE[:12], msg)

    def test_the_note_is_the_report_then_the_signature(self):
        note = self.note()
        report_raw, sig = cr.split_note(note)
        self.assertEqual(json.loads(report_raw)["tree"], TREE)
        self.assertTrue(sig.startswith(b"-----BEGIN SSH SIGNATURE-----"))
        self.assertEqual(report_raw, cr.report_bytes(self.report()))

    def test_changing_a_byte_of_the_report_breaks_the_signature(self):
        note = self.note()
        self.bad(note.replace(b'"result": "pass"', b'"result": "pasz"'), "signature does not match")
        self.bad(note.replace(TREE.encode(), OTHER_TREE.encode(), 1), "signature does not match", tree=OTHER_TREE)

    def test_a_key_that_is_not_allowed_is_refused(self):
        stranger = self.keys.make("stranger")
        self.bad(self.note(key=stranger), "not in the allowed signers")

    def test_a_second_allowed_key_verifies_and_is_named(self):
        second = self.keys.make("second")
        allowed = self.keys.allow([("farhansabbir", self.key), ("ci-maintainer", second)], "two")
        self.assertIn("signed by ci-maintainer", cr.verify(self.note(key=second), allowed, TREE))

    def test_a_signature_made_for_another_purpose_is_refused(self):
        # the same key, signing the same bytes under another namespace, must not count
        path = os.path.join(self.tmp, "r.json")
        with open(path, "wb") as f:
            f.write(cr.report_bytes(self.report()))
        subprocess.run(["ssh-keygen", "-Y", "sign", "-f", self.key, "-n", "git", path], check=True, capture_output=True)
        with open(path, "rb") as f, open(path + ".sig", "rb") as s:
            note = f.read() + s.read()
        self.bad(note, "signature does not match")   # verify is asked for the shint-check namespace only

    def test_a_report_for_another_tree_is_refused(self):
        self.bad(self.note(), "the report is for tree", tree=OTHER_TREE)

    def test_a_report_that_is_too_old_or_from_the_future_is_refused(self):
        old = self.report(created=(self.when - datetime.timedelta(days=15)).strftime("%Y-%m-%dT%H:%M:%SZ"))
        self.bad(self.note(old), "older than 14 days")
        self.assertIn("signed by", cr.verify(self.note(old), self.allowed, TREE, max_age_days=30))
        future = self.report(created=(self.when + datetime.timedelta(days=2)).strftime("%Y-%m-%dT%H:%M:%SZ"))
        self.bad(self.note(future), "in the future")
        self.bad(self.note(self.report(created="yesterday")), "no valid creation time")

    def test_an_incomplete_or_failing_report_is_refused_even_if_signed(self):
        short = self.report(stages=[s for s in self.report()["stages"] if s["name"] != "black-box battery"])
        self.bad(self.note(short), "lacks stage(s): black-box battery")
        stages = self.report()["stages"]
        stages[3] = dict(stages[3], ok=False)
        self.bad(self.note(self.report(stages=stages)), "not ok: black-box battery")
        self.bad(self.note(self.report(result="fail")), "not pass")
        self.bad(self.note(self.report(schema=2)), "schema is 2")

    def test_a_note_without_a_signature_or_with_garbage_is_refused(self):
        self.bad(cr.report_bytes(self.report()), "no signature")
        self.bad(b"not json\n" + self.note().split(b"-----BEGIN")[0][:0] + b"-----BEGIN SSH SIGNATURE-----\nAAAA\n-----END SSH SIGNATURE-----\n", "not in the allowed signers")

    def test_signing_with_a_key_that_does_not_exist_says_so(self):
        with self.assertRaises(cr.Bad) as cm:
            cr.sign(cr.report_bytes(self.report()), os.path.join(self.tmp, "nope"))
        self.assertIn("could not sign", str(cm.exception))

    def test_the_command_line_round_trips(self):
        log = os.path.join(self.tmp, "check.log")
        with open(log, "w") as f:
            f.write(LOG)
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertEqual(cr.main(["build", "--log", log, "--out", os.path.join(self.tmp, "r.json"), "--commit", "c" * 40, "--tree", TREE]), 0)
            self.assertEqual(cr.main(["sign", "--report", os.path.join(self.tmp, "r.json"), "--key", self.key, "--out", os.path.join(self.tmp, "n.txt")]), 0)
            self.assertEqual(cr.main(["verify", "--note", os.path.join(self.tmp, "n.txt"), "--allowed-signers", self.allowed, "--tree", TREE]), 0)
        self.assertIn("check-report: valid: signed by farhansabbir", out.getvalue())
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            self.assertEqual(cr.main(["verify", "--note", os.path.join(self.tmp, "n.txt"), "--allowed-signers", self.allowed, "--tree", OTHER_TREE]), 1)
            self.assertEqual(cr.main(["verify", "--note", os.path.join(self.tmp, "missing"), "--allowed-signers", self.allowed, "--tree", TREE]), 1)
        self.assertIn("the report is for tree", err.getvalue())


class Build(unittest.TestCase):
    def test_the_report_says_what_ran(self):
        r = cr.build_report(LOG, commit="c" * 40, tree=TREE)
        self.assertEqual([s["name"] for s in r["stages"]], cr.REQUIRED_STAGES)
        by = {s["name"]: s for s in r["stages"]}
        self.assertEqual(by["black-box battery"]["detail"], "445 cases: 442 passed, 3 known issues, 0 skipped, 0 failed")
        self.assertEqual(by["go test -race"]["detail"], "2 packages ok")
        self.assertEqual(by["govulncheck"]["detail"], "No vulnerabilities found")
        self.assertEqual((r["schema"], r["result"], r["tree"], r["commit"]), (1, "pass", TREE, "c" * 40))
        self.assertEqual(len(r["log_sha256"]), 64)
        self.assertIn("os", r["host"])

    def test_a_run_that_did_not_finish_is_not_a_report(self):
        with self.assertRaises(cr.Bad) as cm:
            cr.build_report(LOG.replace("==> all checks passed\n", ""))
        self.assertIn("did not pass", str(cm.exception))

    def test_a_missing_stage_is_refused(self):
        with self.assertRaises(cr.Bad) as cm:
            cr.build_report(LOG.replace("==> documentation\n", ""))
        self.assertIn("lacks stage(s): documentation", str(cm.exception))

    def test_a_failed_battery_or_test_is_refused(self):
        with self.assertRaises(cr.Bad) as cm:
            cr.build_report(LOG.replace("0 FAILED", "2 FAILED"))
        self.assertIn("failed: black-box battery", str(cm.exception))
        with self.assertRaises(cr.Bad) as cm:
            cr.build_report(LOG.replace("ok  \tgithub.com/x/shint/v4\t47.9s", "FAIL\tgithub.com/x/shint/v4\t47.9s"))
        self.assertIn("failed: go test -race", str(cm.exception))

    def test_the_bytes_are_stable(self):
        a = cr.report_bytes(cr.build_report(LOG, commit="c" * 40, tree=TREE, when=datetime.datetime(2026, 1, 1, tzinfo=datetime.timezone.utc)))
        b = cr.report_bytes(json.loads(a))
        self.assertEqual(a, b)


if __name__ == "__main__":
    unittest.main(verbosity=1)
