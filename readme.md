# shint v4.2.2 - released 2026-09-26

> **This page describes the `release/v4.2.2` branch only**: what the release delivered, the bugs it fixed, and what changed on the branch. It is a record of that release, added after the tag and not part of it. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Release | [v4.2.2](https://github.com/dmartsapp/shint/releases/tag/v4.2.2), tag on [`d12d643`](https://github.com/dmartsapp/shint/commit/d12d643), 2026-09-26 |
| Kind | Patch on v4.2.1 - a `dns` bug found while validating v4.2.0/v4.2.1, fixed by the project's first outside contribution, and a CI fix that turned out not to work (below) |
| Milestone | [v4.2.2](https://github.com/dmartsapp/shint/milestone/10), closed |
| Everything that differs from the previous release | [v4.2.1...v4.2.2](https://github.com/dmartsapp/shint/compare/v4.2.1...v4.2.2) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.2.2` section) and `docs/src/` |

## Bugs fixed

| Bug | Fix |
|---|---|
| [#64](https://github.com/dmartsapp/shint/issues/64) `shint dns` never checked the hosts file for a forward query, so `shint dns localhost` failed with `NXDOMAIN` while every other shint command resolved it | [`f16d63f`](https://github.com/dmartsapp/shint/commit/f16d63f) by [@littfed](https://github.com/littfed) ([PR #66](https://github.com/dmartsapp/shint/pull/66), merged as [`62b199c`](https://github.com/dmartsapp/shint/commit/62b199c)): `A`/`AAAA` questions without an `@server` are answered from the hosts file (and a built-in `localhost` fallback) first, as `nameserver=hosts transport=file`. Docs and changelog, also by @littfed: [`a750d94`](https://github.com/dmartsapp/shint/commit/a750d94) ([PR #73](https://github.com/dmartsapp/shint/pull/73), merged as [`4e5e268`](https://github.com/dmartsapp/shint/commit/4e5e268)). Two edge cases found while checking those docs against the binary, fixed before the release: [`39c990e`](https://github.com/dmartsapp/shint/commit/39c990e) (`dns -4 localhost AAAA` said "no AAAA records"; a hosts-file name asked for another type with no usable server failed with an empty error) and [`a6716ef`](https://github.com/dmartsapp/shint/commit/a6716ef) (the docs now say the hosts file answers `A` and `AAAA` only). Verified on the released binary. |

## Not fixed after all

| Problem | What happened |
|---|---|
| The Go-module-cache collision (`tar: ... Cannot open: File exists`) in the Lint & vulnerability gate of `build.yaml`, `docker-hub.yaml` and `ghcr.yaml` | [`b49bde5`](https://github.com/dmartsapp/shint/commit/b49bde5) set `cache: false` on the job's own `Set up Go`. **It did not work**: the v4.2.2 tag runs still log about 1590 of these lines each. The collision is between `golang/govulncheck-action`'s cache restore and the modules `golangci-lint` has already downloaded (read-only files), not between two restores - so the wrong cache was switched off. Nothing failed and the scan ran for real, but the v4.2.2 changelog calls it fixed. Tracked in [#75](https://github.com/dmartsapp/shint/issues/75) for v4.3.0, with the fix (restore the cache once, in `Set up Go`; `cache: false` on the action). |

## Not in this release

- [#65](https://github.com/dmartsapp/shint/issues/65) (`rdns`/`lib.ResolveName`: a name on several hosts-file lines may resolve to its first address only) - never reproduced; moved to v4.3.0, waiting on output from the machine that showed it.
- `--verbose` - built on this branch first ([`d85073c`](https://github.com/dmartsapp/shint/commit/d85073c)), reverted here ([`1046f8c`](https://github.com/dmartsapp/shint/commit/1046f8c)) once it was recognised as a new capability, which the project's semver policy puts in a minor release; it is on `release/v4.3.0` ([`e12584e`](https://github.com/dmartsapp/shint/commit/e12584e)).
- `dns <name> PTR` for a name that is not an address sends a near-always-empty query instead of being refused - noticed while fixing #64, not filed.

## Other changes on the branch

- `main` had moved after the branch was cut (a docs-only commit), so it was merged in before the release: [`2298c39`](https://github.com/dmartsapp/shint/commit/2298c39).
- The release commit [`d12d643`](https://github.com/dmartsapp/shint/commit/d12d643) has the version bump, the changelog, the docs' version examples and `main`'s README. It went out with a placeholder changelog heading, `## v4.2.2 - 2026-MM-DD`, which the docs site showed; fixed on `main` in [`5da3666`](https://github.com/dmartsapp/shint/commit/5da3666). The tag keeps the placeholder (published tags are never moved); the GitHub release notes were not affected.
- `main`'s README got its v4.2.2 row after the tag: [`82a7fcc`](https://github.com/dmartsapp/shint/commit/82a7fcc) (from README Reconcile, issue [#74](https://github.com/dmartsapp/shint/issues/74)).
- Every change on this branch was also cherry-picked onto `release/v4.3.0`, so the next minor release was built and tested with them.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
