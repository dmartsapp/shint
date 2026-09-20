#!/usr/bin/env python3
"""Tests for docs/build.py: the Markdown renderer's rules and the published-site URL check.

    python3 docs/test_build.py

Standard library only, like build.py itself.
"""
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build  # noqa: E402


def rendered_site():
    pages = build.collect_pages()
    version = build.read_version()
    return {"%s.html" % p["slug"]: build.render_page(p, pages, version) for p in pages}


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
