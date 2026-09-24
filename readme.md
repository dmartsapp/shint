# shint v4.2.1 - released 2026-09-23

> **This page describes the `release/v4.2.1` branch only**: what the release delivered, the bugs it fixed, and what changed on the branch. It is a record of that release, added after the tag and not part of it. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Release | [v4.2.1](https://github.com/dmartsapp/shint/releases/tag/v4.2.1), tag on [`87e0f78`](https://github.com/dmartsapp/shint/commit/87e0f78), 2026-09-23 |
| Kind | Patch on v4.2.0 - bugs found and reported during v4.2.0 validation, plus release-process work built during that same window |
| Milestone | [v4.2.1](https://github.com/dmartsapp/shint/milestone/9), closed |
| Everything that differs from the previous release | [v4.2.0...v4.2.1](https://github.com/dmartsapp/shint/compare/v4.2.0...v4.2.1) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.2.1` section) and `docs/src/` |

## Bugs fixed

Found while the maintainer validated v4.2.0 by hand, working through it gradually:

| Bug | Fix |
|---|---|
| `shint ip` printed an `interface` line and then one `address` line per address, each saying the address twice; its JSON nested a five-field object per address | [`c7d7259`](https://github.com/dmartsapp/shint/commit/c7d7259) (Issue #52) - now one line and one flat object per interface |
| `shint dns` called the field who answered `server=`, a generic word for a command whose whole job is talking to nameservers | [`2281092`](https://github.com/dmartsapp/shint/commit/2281092) (Issue #53) - renamed to `nameserver=`/`nameserver` in text and JSON |
| The Vulnerability Check workflow had a redundant `Set up Go` step colliding with `golang/govulncheck-action@v1`'s own internal one, restoring the Go module cache twice into the same paths (`tar: ... Cannot open: File exists`, every release) | [`3aba604`](https://github.com/dmartsapp/shint/commit/3aba604) - the scan itself always ran for real; this was wasted time and alarming log noise, not a false pass. Confirmed fixed against the real v4.2.1 tag run's log. |

## New build platforms

Requested directly ("add AIX and Solaris SPARC support"), and extended after checking what else was missing:

| Platform | Delivered in |
|---|---|
| `aix/ppc64` (IBM POWER - Go's only AIX port) | [`9b1848e`](https://github.com/dmartsapp/shint/commit/9b1848e) |
| `linux/arm` (`GOARM=6`), `linux/ppc64le`, `illumos/amd64` | [`29fc863`](https://github.com/dmartsapp/shint/commit/29fc863) |

**Not added, because it does not exist**: Solaris SPARC. Go's Solaris port has only ever targeted `amd64` - confirmed with `go tool dist list`, not assumed. A release now has 36 assets (18 binaries, 18 `.sha256` files) instead of 28. Vetting `linux/arm` for the first time - the only 32-bit-`int` target shint ships - caught a real, pre-existing test portability bug (two test files used `math.MaxInt64`-scale sentinel values that do not compile as `int` on a 32-bit platform), fixed in the same commit.

## Release process, built during the v4.2.0 validation window

Not v4.2.1-specific bug fixes, but real infrastructure that landed on this branch (and, for the README rewrite, directly on `main`) while the maintainer worked through v4.2.0:

- **Signed local check reports**, replacing an earlier commit-status receipt: `make attest` runs `make check`, signs a report of it with an SSH key, and attaches it to the commit as a git note (`refs/notes/checks`). The `Check` workflow on `main` verifies it and runs a ~1-minute subset instead of the whole ~3.5-minute suite when the note is valid. [`38e93a5`](https://github.com/dmartsapp/shint/commit/38e93a5), [`7482e6f`](https://github.com/dmartsapp/shint/commit/7482e6f). See [the signed local check report](https://dmartsapp.github.io/shint/docs/tech-ci.html#the-signed-local-check-report).
- **`main`'s README is reconciled automatically after a release**: `.github/scripts/readme-reconcile.py` reads the README and brings it in line with the release tags, the changelog, the GitHub milestones and the binary's `--help`, warning rather than guessing about what it cannot know. A tag-triggered workflow runs it and opens a pull request (or, since this repository does not let Actions open pull requests, an issue that links a branch). [`8381cce`](https://github.com/dmartsapp/shint/commit/8381cce), [`bf503ce`](https://github.com/dmartsapp/shint/commit/bf503ce), [`a26efbc`](https://github.com/dmartsapp/shint/commit/a26efbc). First real run, on this release's own tag, worked correctly (Issue #63). See [README after a release](https://dmartsapp.github.io/shint/docs/tech-release.html#readme-after-a-release).
- **`main`'s README was rewritten**: the "Why shint?" narrative - explicitly not replacing `ping`/`telnet`/`curl`/`nmap`/`dig`/`nc`, what shint took from each, and two side-by-side comparisons with classic tools. [`fdc76ac`](https://github.com/dmartsapp/shint/commit/fdc76ac), pushed directly to `main` per the maintainer's explicit instruction (a README-on-main change, not branch-scoped work).
- The stale GitHub Discussions welcome post was replaced and eight release announcements were posted, directly via the GitHub API - not a code change, so no commit.

## Other changes on the branch

- Docs updated for every new platform (`docs/src/install.md`, `tech-ci.md`, `tech-release.md`, `tech-source.md`) and the version-example bump to `4.2.1` throughout.
- The version bump, the dated changelog and the docs version examples are in the release commit [`87e0f78`](https://github.com/dmartsapp/shint/commit/87e0f78) itself.
- Issues #52, #53 and the stale CI-failure issue #62 (already fixed by earlier commits on `main`, closed as stale rather than re-fixed) were closed after this release shipped.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
