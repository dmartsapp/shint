#!/usr/bin/env python3
"""Tests for profile.py: the parts that do not need a shint build.

    python3 test/profile/test_profile.py
"""
import json
import os
import re
import shutil
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import profile as P  # noqa: E402

FAKE = """#!/bin/sh
# a stand-in binary: no dns command, like a release before v4.2.0
case "$1" in
  dns) echo 'Error: unknown command "dns" for "shint"' >&2; exit 1 ;;
esac
exit 0
"""


def summary(tag, machine_id, wall, rss, size, kind="release", available=True):
    s = {"schema": 1, "tag": tag, "kind": kind, "run_at": "2026-09-27T00:00:00+00:00",
         "machine": {"id": machine_id, "power": "ac"}, "binary": {"host_size_bytes": size},
         "scenarios": {}}
    for name, *_ in P.SCENARIOS:
        s["scenarios"][name] = {"available": available, "wall_ms": {"median": wall}, "cpu_ms": {"median": wall},
                                "max_rss_mib": {"median": rss}}
    return s


class Helpers(unittest.TestCase):
    def test_stats(self):
        s = P.stats([3, 1, 2, None])
        self.assertEqual((s["median"], s["min"], s["max"], s["n"]), (2, 1, 3, 3))
        self.assertEqual(P.stats([5])["stdev"], 0.0)
        self.assertIsNone(P.stats([]))

    def test_the_run_folder_name_has_no_colons(self):
        self.assertRegex(P.utc_stamp(), r"^\d{4}-\d{2}-\d{2}T\d{6}Z$")

    def test_every_release_platform_is_read_from_the_makefile(self):
        got = P.platforms()
        self.assertEqual(len(got), 18, got)
        self.assertIn(("linux", "arm", "6"), got)
        self.assertIn(("darwin", "arm64", ""), got)
        self.assertIn(("aix", "ppc64", ""), got)

    def test_the_machine_is_recorded_with_an_id_but_not_its_name(self):
        m = P.machine()
        self.assertRegex(m["id"], r"^[0-9a-f]{12}$")
        self.assertTrue(m["os"] and m["arch"] and m["cpu"]["logical_cores"])
        self.assertNotIn(os.uname().nodename, json.dumps(m))


class Scenarios(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="test-profile-")
        self.bin = os.path.join(self.tmp, "shint")
        with open(self.bin, "w") as f:
            f.write(FAKE)
        os.chmod(self.bin, 0o755)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_measure_reports_the_exit_code_and_the_process_resources(self):
        r = P.measure(["/bin/sh", "-c", "exit 3"])
        self.assertEqual(r["exit"], 3)
        self.assertGreater(r["wall_ms"], 0)
        self.assertGreater(r["max_rss_mib"], 0)

    def test_a_command_the_version_lacks_is_not_available(self):
        self.assertFalse(P.available(self.bin, ["dns", "x"]))
        self.assertTrue(P.available(self.bin, ["telnet", "x"]))
        self.assertTrue(P.available(self.bin, ["--version"]))

    def test_run_scenarios(self):
        got = P.run_scenarios(self.bin, {}, only=["startup", "dns"], log=False)
        self.assertEqual(set(got), {"startup", "dns"})
        self.assertFalse(got["dns"]["available"])
        s = got["startup"]
        self.assertTrue(s["available"] and s["ok"])
        self.assertEqual(s["repeat"], 20)
        self.assertEqual(len(s["exit_codes"]), 20)          # the warm-up is not kept
        self.assertEqual(s["wall_ms"]["n"], 20)


class Compare(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="test-profile-")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def write(self, name, s):
        d = os.path.join(self.tmp, name)
        os.makedirs(d)
        with open(os.path.join(d, "summary.json"), "w") as f:
            json.dump(s, f)
        return d

    def test_changes_are_shown_in_percent(self):
        a = self.write("a", summary("v4.2.3", "m1", 10.0, 20.0, 1000))
        b = self.write("b", summary("v4.3.0", "m1", 15.0, 10.0, 900))
        out = P.compare(a, b)
        self.assertIn("v4.2.3 (release", out)
        self.assertIn("+50.0%", out)
        self.assertIn("-50.0%", out)
        self.assertIn("1000 -> 900 bytes -10.0%", out)
        self.assertNotIn("WARNING", out)

    def test_different_machines_are_called_out(self):
        a = self.write("a", summary("v4.2.3", "m1", 10.0, 20.0, 1000))
        b = self.write("b", summary("v4.3.0", "m2", 10.0, 20.0, 1000))
        self.assertIn("WARNING: different machines", P.compare(a, b))

    def test_a_scenario_missing_from_one_side_is_na(self):
        a = self.write("a", summary("v4.0.0", "m1", 10.0, 20.0, 1000, available=False))
        b = self.write("b", summary("v4.3.0", "m1", 10.0, 20.0, 1000))
        self.assertRegex(P.compare(a, b), re.compile(r"^dns +n/a in v4\.0\.0$", re.M))

    def test_a_tag_resolves_to_its_latest_run(self):
        old = P.OUT
        P.OUT = self.tmp
        try:
            for stamp, wall in (("2026-09-01T000000Z", 1.0), ("2026-09-02T000000Z", 2.0)):
                d = os.path.join(self.tmp, "v4.2.3", stamp)
                os.makedirs(d)
                with open(os.path.join(d, "summary.json"), "w") as f:
                    json.dump(summary("v4.2.3", "m1", wall, 1.0, 1), f)
            self.assertTrue(P.resolve("v4.2.3").endswith("2026-09-02T000000Z/summary.json"))
            with self.assertRaises(SystemExit):
                P.resolve("v9.9.9")
        finally:
            P.OUT = old


if __name__ == "__main__":
    unittest.main(verbosity=2)
