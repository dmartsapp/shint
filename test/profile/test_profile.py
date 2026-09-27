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

P.QUIET_LIMIT = 0          # the tests never wait for a quiet machine

FAKE = """#!/bin/sh
# a stand-in binary: no dns command, like a release before v4.2.0
case "$1" in
  dns) echo 'Error: unknown command "dns" for "shint"' >&2; exit 1 ;;
esac
exit 0
"""


def summary(tag, machine_id, wall, rss, size, kind="release", available=True, at="2026-09-27T00:00:00+00:00"):
    s = {"schema": 1, "tag": tag, "kind": kind, "run_at": at,
         "machine": {"id": machine_id, "power": "ac", "model": "Model-" + machine_id, "cpu": {"logical_cores": 8}, "load_1m_start": 1.5},
         "binary": {"host_size_bytes": size}, "scenarios": {}}
    for name, *_ in P.SCENARIOS:
        s["scenarios"][name] = {"available": available, "wall_ms": {"median": wall, "min": wall * .99, "max": wall * 1.01},
                                "cpu_ms": {"median": wall, "min": wall * .99, "max": wall * 1.01},
                                "max_rss_mib": {"median": rss, "min": rss, "max": rss}}
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

    def test_a_failure_is_seen_in_the_output_too(self):
        self.assertTrue(P.measure(["/bin/sh", "-c", "echo 'x: [web] ERROR response'"])["failed"])     # exit 0, as before v4.0.3
        self.assertTrue(P.measure(["/bin/sh", "-c", "echo '{\"success\": false}'"])["failed"])
        self.assertFalse(P.measure(["/bin/sh", "-c", "echo 'x: [web] OK response'"])["failed"])

    def test_measure_reports_the_exit_code_and_the_process_resources(self):
        r = P.measure(["/bin/sh", "-c", "exit 3"])
        self.assertTrue(r["failed"])
        self.assertEqual(r["exit"], 3)
        self.assertGreater(r["wall_ms"], 0)
        self.assertGreater(r["max_rss_mib"], 0)

    def test_a_command_the_version_lacks_is_not_available(self):
        self.assertFalse(P.available(self.bin, ["dns", "x"]))
        self.assertTrue(P.available(self.bin, ["telnet", "x"]))
        self.assertTrue(P.available(self.bin, ["--version"]))

    def test_interleaved_runs_measure_every_binary_the_same_number_of_times(self):
        other = os.path.join(self.tmp, "other")
        with open(other, "w") as f:
            f.write("#!/bin/sh\nexit 0\n")                  # this one has dns
        os.chmod(other, 0o755)
        got = P.run_interleaved({"old": self.bin, "new": other}, {"dns": 1}, only=["startup", "dns"], log=False)
        self.assertEqual(set(got), {"old", "new"})
        self.assertEqual(got["old"]["startup"]["wall_ms"]["n"], got["new"]["startup"]["wall_ms"]["n"])
        self.assertFalse(got["old"]["dns"]["available"])
        self.assertTrue(got["new"]["dns"]["available"])

    def test_ab_of_two_binaries(self):
        out = P.ab(self.bin, self.bin, only=["startup", "dns"], repeat_scale=0.1)
        self.assertIn("shint(A) (ab", out)
        self.assertIn("shint(B) (ab", out)
        self.assertRegex(out, re.compile(r"^startup +\d", re.M))
        self.assertRegex(out, re.compile(r"^dns +n/a in shint\(A\), shint\(B\)$", re.M))
        with self.assertRaises(SystemExit):
            P.ab("not-a-ref", self.bin)

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
        self.old_out, P.OUT = P.OUT, os.path.join(self.tmp, ".profiling")

    def tearDown(self):
        P.OUT = self.old_out
        shutil.rmtree(self.tmp, ignore_errors=True)

    def run_(self, s, stamp=None, dev=False):
        self.n = getattr(self, "n", 0) + 1
        d = os.path.join(P.OUT, "dev" if dev else "", s["tag"], stamp or "2026-09-27T0000%02dZ" % self.n)
        os.makedirs(d)
        with open(os.path.join(d, "summary.json"), "w") as f:
            json.dump(s, f)

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
        self.run_(summary("v4.2.3", "m1", 1.0, 1.0, 1, at="2026-09-01T00:00:00+00:00"), "2026-09-01T000000Z")
        self.run_(summary("v4.2.3", "m1", 2.0, 1.0, 1, at="2026-09-02T00:00:00+00:00"), "2026-09-02T000000Z")
        self.assertTrue(P.resolve("v4.2.3").endswith("2026-09-02T000000Z/summary.json"))
        with self.assertRaises(SystemExit):
            P.resolve("v9.9.9")

    def test_two_tags_are_compared_on_the_same_machine(self):
        self.run_(summary("v4.0.0", "m1", 10.0, 1.0, 1, at="2026-09-01T00:00:00+00:00"), "2026-09-01T000000Z")
        self.run_(summary("v4.0.0", "m2", 99.0, 1.0, 1, at="2026-09-02T00:00:00+00:00"), "2026-09-02T000000Z")
        self.run_(summary("v4.1.0", "m1", 20.0, 1.0, 1))
        self.assertNotIn("WARNING", P.compare("v4.0.0", "v4.1.0", "m1"))
        self.assertIn("+100.0%", P.compare("v4.0.0", "v4.1.0", "m1"))
        self.assertIn("load (1 min) at the start: 1.5, 1.5", P.compare("v4.0.0", "v4.1.0", "m1"))
        # neither build ran on this machine and none is named: each tag's latest run, and a warning
        self.assertIn("WARNING: different machines", P.compare("v4.0.0", "v4.1.0"))

    def test_history_of_one_machine(self):
        self.run_(summary("v4.0.0", "m1", 10.0, 1.0, 1000, kind="backfill"))
        self.run_(summary("v4.10.0", "m1", 20.0, 1.0, 1100))
        self.run_(summary("v4.9.0", "m1", 15.0, 1.0, 1050))
        self.run_(summary("v4.9.0", "m2", 5.0, 1.0, 1050))
        out = P.history()                                   # this machine has no runs: the busiest one
        self.assertIn("machine m1: Model-m1", out)
        self.assertRegex(out, r"scenario +v4\.0\.0\* +v4\.9\.0 +v4\.10\.0")   # version order, * = backfill
        self.assertRegex(out, re.compile(r"^startup +10\.0 +15\.0 +20\.0 +\S+ +\+100\.0%$", re.M))
        self.assertRegex(out, re.compile(r"^binary MiB .*\+10\.0%$", re.M))
        self.assertRegex(out, re.compile(r"^load 1m +1\.50 +1\.50 +1\.50", re.M))
        self.assertIn("v4.10.0", P.history(last=1))
        self.assertNotIn("v4.9.0", P.history(last=1))
        m2 = P.history("m2")
        self.assertIn("machine m2", m2)
        self.assertNotIn("v4.0.0", m2)
        with self.assertRaises(SystemExit):
            P.history("m")                                  # ambiguous: m1 and m2

    def test_a_failed_scenario_is_marked_not_charted_as_a_result(self):
        s = summary("v4.0.0", "m1", 10.0, 1.0, 1)
        s["scenarios"]["web"]["ok"] = False
        self.run_(s)
        self.run_(summary("v4.1.0", "m1", 10.0, 1.0, 1))
        self.assertRegex(P.history(), re.compile(r"^web +10\.0! +10\.0", re.M))
        self.assertRegex(P.compare("v4.0.0", "v4.1.0", "m1"), re.compile(r"^web .*FAILED in v4\.0\.0$", re.M))


    def test_dev_runs_only_when_asked(self):
        self.run_(summary("v4.0.0", "m1", 10.0, 1.0, 1))
        self.run_(summary("v4.0.0-3-gabc", "m1", 12.0, 1.0, 1, kind="dev"), dev=True)
        self.assertNotIn("gabc", P.history())
        self.assertIn("v4.0.0-3-gabc", P.history(dev=True))

    def test_machines_table(self):
        self.run_(summary("v4.0.0", "m1", 10.0, 1.0, 1))
        self.run_(summary("v4.1.0", "m1", 10.0, 1.0, 1))
        self.run_(summary("v4.1.0", "m2", 10.0, 1.0, 1))
        out = P.machines_table()
        self.assertIn("m1  2 runs, 2 builds (v4.0.0 .. v4.1.0)", out)
        self.assertIn("m2  1 run, 1 build (v4.1.0 .. v4.1.0)", out)
        self.assertIn("Model-m2, 8 cores", out)

    def test_a_busy_machine_is_noted(self):
        self.assertIn("busy", P.busy_note({"load_1m_start": 9.0, "cpu": {"logical_cores": 10}}))
        self.assertEqual(P.busy_note({"load_1m_start": 3.0, "cpu": {"logical_cores": 10}}), "")

    def test_spark_and_noise(self):
        self.assertEqual(P.spark([1, None, 3]), "\u2581 \u2588")
        self.assertEqual(P.spark([None]), "")
        a = {"w": {"median": 10, "min": 8, "max": 12}}
        self.assertTrue(P.noisy(a, {"w": {"median": 11, "min": 11, "max": 11}}, "w"))
        self.assertFalse(P.noisy(a, {"w": {"median": 20, "min": 20, "max": 20}}, "w"))


if __name__ == "__main__":
    unittest.main(verbosity=2)
