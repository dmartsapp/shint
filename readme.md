# shint v4.0.4 - released 2026-09-20

> **This page describes the `release/v4.0.4` branch only**: what the release delivered, the bugs it fixed, and what changed on the branch. It is a record of that release, added after the tag and not part of it. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Release | [v4.0.4](https://github.com/dmartsapp/shint/releases/tag/v4.0.4), tag on [`d959348`](https://github.com/dmartsapp/shint/commit/d959348), 2026-09-20 |
| Kind | Patch on v4.0.3 |
| Milestone | None - this release predates the milestone and issue tracking; the work is recorded in the changelog and the commits below |
| Everything that differs from the previous release | [v4.0.3...v4.0.4](https://github.com/dmartsapp/shint/compare/v4.0.3...v4.0.4) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.0.4` section) and `docs/src/` |

## Targets

| Target | Delivered in |
|---|---|
| `ping --timeout` that works, `ping` input validation, and the payload size on every reply | [`5e07215`](https://github.com/dmartsapp/shint/commit/5e07215), [`ece6d18`](https://github.com/dmartsapp/shint/commit/ece6d18) |
| A release pipeline that starts only from a `vX.Y.Z` tag on `main` | [`e1153ef`](https://github.com/dmartsapp/shint/commit/e1153ef) |
| The Makefile as the one place to build and test from, with the workflow trigger rule enforced by `make check` | [`933b41b`](https://github.com/dmartsapp/shint/commit/933b41b) |
| The release process written down: `release/vX.Y.Z` branches, a two-week cadence, one release in flight | [`e9dc06d`](https://github.com/dmartsapp/shint/commit/e9dc06d) |

## Bugs fixed

| Bug | Fix |
|---|---|
| `ping` ignored `--timeout`: every echo request waited a fixed second | [`5e07215`](https://github.com/dmartsapp/shint/commit/5e07215) |
| `ping` silently shrank an oversized `--payload` and zeroed a negative one; it is now a usage error (exit 2) | [`5e07215`](https://github.com/dmartsapp/shint/commit/5e07215) |
| A lost `ping` request was logged as `OK` while the run exited 1; it is now `ERROR` | [`5e07215`](https://github.com/dmartsapp/shint/commit/5e07215) |
| `ping --json` reported `payload_size_bytes` as 0 in every document | [`ece6d18`](https://github.com/dmartsapp/shint/commit/ece6d18) |
| `udp --help` suggested `--data "\x00\x00"`, which is sent as text, not as bytes | [`715a5a0`](https://github.com/dmartsapp/shint/commit/715a5a0) |
| The failure issue that CI files showed raw `${{ ... }}` text, because GitHub expands those only in workflow files | [`513bd5a`](https://github.com/dmartsapp/shint/commit/513bd5a) |
| Five broken documentation links on the README front page (they lacked `/docs/`) | [`3a4fa2f`](https://github.com/dmartsapp/shint/commit/3a4fa2f) |

Not code, so not on this branch: [#11](https://github.com/dmartsapp/shint/issues/11) (the GHCR package was private although the documentation listed it as an install source - found while releasing this version, fixed by a settings change).

## Other changes on the branch

- **The README gained a Roadmap** (one release every two weeks) and `python3 docs/build.py --check` now also verifies, offline, that every published-site URL and `#anchor` in the README, the changelog and the docs sources exists ([`3a4fa2f`](https://github.com/dmartsapp/shint/commit/3a4fa2f), [`6b3cf19`](https://github.com/dmartsapp/shint/commit/6b3cf19)).
- The `shint` binary that `make run` builds is git-ignored.
- The live smoke test (`make test-live`) also checks that `ping` shows the payload size.
- The version bump, the dated changelog and the docs version examples are in the release commit [`d959348`](https://github.com/dmartsapp/shint/commit/d959348).

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
