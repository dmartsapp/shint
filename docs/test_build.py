#!/usr/bin/env python3
"""Tests for docs/build.py: the Markdown renderer's rules and the published-site URL check.

    python3 docs/test_build.py

Standard library only, like build.py itself.
"""
import json
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build  # noqa: E402


def rendered_site():
    pages = build.collect_pages()
    version = build.read_version()
    return {"%s.html" % p["slug"]: build.render_page(p, pages, version) for p in pages}


class Tokens(unittest.TestCase):
    """fill_tokens: the current release in a page, from main.go's Version."""

    def test_the_three_tokens(self):
        self.assertEqual(build.fill_tokens("{{version}} {{minor}} {{major}}", "4.3.0"), "4.3.0 4.3 4")

    def test_a_backslash_keeps_one_literal(self):
        self.assertEqual(build.fill_tokens(r"\{{version}} is {{version}}", "4.3.0"), "{{version}} is 4.3.0")

    def test_other_braces_are_left_alone(self):
        for text in ("${{ github.ref_name }}", "{{ version }}", "{{foo}}", "{version}"):
            with self.subTest(text=text):
                self.assertEqual(build.fill_tokens(text, "4.3.0"), text)

    def test_every_page_is_filled_in_except_its_escaped_tokens(self):
        for src in sorted(build.SRC.glob("*.md")):
            raw = src.read_text()
            out = build.fill_tokens(raw, "9.8.7")
            for name in ("version", "minor", "major"):
                with self.subTest(page=src.name, token=name):
                    self.assertEqual(out.count("{{%s}}" % name), raw.count("\\{{%s}}" % name))


class Performance(unittest.TestCase):
    """performance_data / performance_html: the Performance page, from .profiling/."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="test-perf-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def run_(self, where, tag, mid, wall, label=""):
        d = self.tmp / where
        d.mkdir(parents=True)
        (d / "summary.json").write_text(json.dumps({
            "tag": tag, "kind": "release", "run_at": "2026-09-27T00:00:00+00:00",
            "machine": {"id": mid, "label": label, "cpu": {"model": "cpu", "logical_cores": 8}, "load_1m_start": 2.5},
            "binary": {"host_size_bytes": 1000},
            "scenarios": {"startup": {"available": True, "what": "start",
                                      "wall_ms": {"median": wall, "min": wall, "max": wall},
                                      "cpu_ms": {"median": 1, "min": 1, "max": 1},
                                      "max_rss_mib": {"median": 2, "min": 2, "max": 2}},
                          "dns": {"available": False, "what": "dns"}}}))

    def test_committed_runs_in_version_order_and_never_dev(self):
        self.run_("v4.10.0/t", "v4.10.0", "m1", 3)
        self.run_("v4.9.0/t", "v4.9.0", "m1", 2)
        self.run_("dev/v4.9.0-1-gabc/t", "v4.9.0-1-gabc", "m1", 9)
        d = build.performance_data(self.tmp)
        self.assertEqual([r["tag"] for r in d["runs"]], ["v4.9.0", "v4.10.0"])
        self.assertEqual(d["runs"][0]["s"]["startup"]["w"], [2, 2, 2])
        self.assertEqual(d["runs"][0]["load"], 2.5)
        self.assertNotIn("dns", d["runs"][0]["s"])                      # not available in that build
        self.assertEqual([s[0] for s in d["scenarios"]], ["startup", "dns"])
        self.assertEqual(set(d["machines"]), {"m1"})

    def test_the_data_cannot_end_its_script_tag(self):
        self.run_("v4.0.0/t", "v4.0.0", "m1", 1, label="</script><b>x")
        out = build.performance_html(build.performance_data(self.tmp))
        data = out.split('id="perf-data">', 1)[1].split("</script>", 1)[0]
        self.assertIn("<\\/script>", data)
        self.assertEqual(json.loads(data)["machines"]["m1"]["label"], "</script><b>x")
        self.assertIn("<noscript>", out)

    def test_no_runs(self):
        self.assertIn("No profiling runs yet", build.performance_html(build.performance_data(self.tmp)))

    def test_the_site_has_the_page(self):
        page = rendered_site()["tech-performance.html"]
        self.assertIn('id="perf-data"', page)
        self.assertIn('src="assets/performance.js"', page)


class SiteUrlCheck(unittest.TestCase):
    """check_site_urls: absolute URLs of the published site must name files that exist."""

    @classmethod
    def setUpClass(cls):
        cls.rendered = rendered_site()

    def check(self, text):
        return build.check_site_urls({"t.md": text}, self.rendered)

    def test_repository_is_currently_clean(self):
        self.assertEqual(build.check_site_urls(build.site_url_sources(), self.rendered), [])

    def test_valid_urls_pass(self):
        base = build.SITE_URL
        for url in (base, base + "/", base + "/docs/", base + "/docs/index.html",
                    base + "/docs/install.html", base + "/docs/install.html#docker",
                    base + "/docs/assets/style.css", base + "/docs/src/index.md"):
            with self.subTest(url=url):
                self.assertEqual(self.check("see " + url), [])

    def test_the_readme_bug_is_caught(self):
        """The README once linked to /install.html instead of /docs/install.html and nothing noticed."""
        pages = ("install", "usage", "cookbook", "troubleshooting", "tech-architecture")
        text = "\n".join("[x](%s/%s.html)" % (build.SITE_URL, p) for p in pages)
        problems = self.check(text)
        self.assertEqual(len(problems), len(pages))
        joined = "\n".join(problems)
        for page in pages:
            self.assertIn("did you mean %s/docs/%s.html?" % (build.SITE_URL, page), joined)

    def test_missing_page_without_a_suggestion(self):
        problems = self.check(build.SITE_URL + "/docs/no-such-page.html")
        self.assertEqual(len(problems), 1)
        self.assertNotIn("did you mean", problems[0])

    def test_missing_anchor_is_caught(self):
        problems = self.check(build.SITE_URL + "/docs/install.html#no-such-section")
        self.assertEqual(len(problems), 1)
        self.assertIn("#no-such-section", problems[0])

    def test_trailing_punctuation_is_not_part_of_the_url(self):
        self.assertEqual(self.check("Docs: %s/docs/install.html." % build.SITE_URL), [])
        self.assertEqual(self.check("(%s/docs/install.html)" % build.SITE_URL), [])

    def test_other_hosts_are_ignored(self):
        self.assertEqual(self.check("https://example.com/shint/install.html https://github.com/dmartsapp/shint"), [])


class Markdown(unittest.TestCase):
    def render(self, md):
        return build.render_blocks(md.splitlines(), [], set())

    def test_md_links_become_html(self):
        self.assertIn('href="install.html#docker"', self.render("[x](install.md#docker)"))

    def test_pipes_inside_code_do_not_split_table_cells(self):
        html = self.render("| a | b |\n|---|---|\n| `x|y` | z |")
        self.assertIn("<code>x|y</code>", html)

    def test_log_lines_are_highlighted(self):
        html = build.render_code("Sun Sep 20 01:26:48 MDT 2026: [telnet] OK connect ok host=1.2.3.4", "text")
        self.assertIn('tok-ok">OK<', html)
        self.assertIn('tok-mod">[telnet]<', html)


if __name__ == "__main__":
    unittest.main(verbosity=2)
