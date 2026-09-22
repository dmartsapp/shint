---
title: This documentation
lead: How the documentation site is built, hosted and maintained - and how to add or change a page.
description: How the shint documentation site is structured, built with docs/build.py, and published with GitHub Pages.
section: Technical
order: 8
nav: This documentation
---

## Structure

The site is plain static HTML served by GitHub Pages. There is no framework and no external service: no JavaScript build, no CDN, no web fonts.

```plain
docs/
├── build.py           the generator (Python 3, standard library only)
├── src/*.md           one Markdown source per page
├── assets/            style.css, site.js, favicon.svg
└── *.html             generated pages - committed, never edited by hand
CHANGELOG.md           at the repo root; rendered as the Changelog page
index.html, .nojekyll  at the repo root; see Hosting
```

The generator turns each `docs/src/<name>.md` into `docs/<name>.html`, wraps it in the shared layout (top bar, sidebar, "On this page" list, previous/next links), and renders `CHANGELOG.md` as the [Changelog](changelog.md) page - so the changelog has exactly one source of truth.

## Changing a page

```bash
$EDITOR docs/src/telnet.md
python3 docs/build.py           # regenerate the HTML
python3 docs/build.py --check   # verify pages are current and every link and anchor resolves
git add docs && git commit
```

A documentation change needs no release branch, no version bump and no tag - it is not a release. Push it like any other change to `main` ([Merging a branch to main](tech-ci.md#merging-a-branch-to-main-a-practical-checklist)):

```bash
git fetch origin && git merge --ff-only origin/main   # only if main moved since you started
make attest                                            # optional: signs a report so Check runs the quick path
git push origin main
```

That triggers `Check` - the whole `make check`, or `make check-quick` if `make attest` ran first - and nothing else: no binaries, no release, no tag.

`--check` exits non-zero if a generated page is stale, an internal link or `#anchor` is broken, **or a published-site URL** (`https://dmartsapp.github.io/shint/...`) in `readme.md`, `CHANGELOG.md` or a page source does not name a page that exists (and an anchor that exists on it). The last rule runs offline: the site is the repository root served as-is, so `.../shint/docs/install.html` must be `docs/install.html` in the repo. It was added after the README linked to `.../shint/install.html` (missing `/docs/`) and every link in that section returned 404 for a release without anything noticing. Use it as a pre-commit or release check.

The generator has its own tests, also standard-library only: `python3 docs/test_build.py`. They cover the URL check (including a regression test for exactly that README bug) and a few renderer rules.

## Page format

Each source starts with a small header, then Markdown:

```plain
---
title: telnet
lead: One-sentence summary shown under the title.
description: Text for search engines and link previews.
section: Commands
order: 1
nav: telnet
---

## First section
```

| Header key | Meaning |
|---|---|
| `title` | The page heading and browser title. |
| `lead` | The larger introductory sentence under the title. |
| `description` | The `<meta name="description">`. |
| `section` | Sidebar group: `Get started`, `Commands`, `Guides`, `Reference` or `Technical`. |
| `order` | Position within the group. |
| `nav` | The sidebar label, if different from `title`. |

The page title is drawn from the header, so the body starts at `##`. The Markdown is a deliberately small dialect:

| Write | To get |
|---|---|
| `## Heading`, `### Sub` | Section headings with anchors; they feed "On this page" |
| `- item`, `1. item` | Lists (nest by indenting) |
| `| a | b |` with a `|---|---|` row | A table |
| text between single backticks | Inline code |
| `**bold**`, `*italic*`, `[text](other.md#anchor)` | Bold, italic, and links (`.md` links become `.html`) |
| a fenced block with `bash`, `text`, `json`, `yaml`, `go`, `dockerfile` or `makefile` | A code block with a label, copy button and light syntax colouring; `text` colours log lines (`OK` green, `ERROR` red) |
| `:::note Title` ... `:::` (also `tip`, `warning`) | A callout box |
| `:::html` ... `:::` | Raw HTML, for the home-page cards and the diagrams |

### Adding a page

1. Create `docs/src/<name>.md` with the header above.
2. Run `python3 docs/build.py`. The page appears in the sidebar automatically, ordered by `section` and `order`.
3. Link to it from wherever it belongs, using `[text](<name>.md)`.

## Writing guidelines

- **Real output only.** Examples show output from the real binary, copied verbatim - never typed by hand. If behaviour changes, regenerate the example.
- **Plain language first.** The user-facing pages explain what and why before how; technical detail lives in the Technical section.
- **Every claim checkable.** State versions, defaults and limits exactly as the code has them, and link to the page that goes deeper.
- **The README stays short.** It introduces shint and links here; detail belongs on this site.

## Hosting

The repository's Pages setting builds from the `main` branch, folder `/`. Two small files at the repository root make that work for a site that lives under `docs/`:

- **`.nojekyll`** tells GitHub Pages to serve files exactly as they are instead of running them through Jekyll (which would try to process the Markdown sources and skip anything starting with `_`).
- **`index.html`** at the root redirects the site's front door to `docs/`, so `https://dmartsapp.github.io/shint/` lands on the home page.

If the Pages source is ever switched to the `/docs` folder, the pages keep working unchanged and the root stub simply becomes unused.

Publishing is automatic: a push to `main` runs GitHub's own `pages-build-deployment` workflow, and the change is live within a minute or two. Nothing in this repository's workflows builds the site, which is why the generated HTML is committed.
