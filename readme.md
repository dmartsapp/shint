# shint v4.2.2 - work in progress

> **This page describes the `release/v4.2.2` branch only**: what the release is meant to deliver, the bugs it fixes, and what has changed on the branch. It is a working page. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Milestone | [v4.2.2](https://github.com/dmartsapp/shint/milestone/10) - no due date: an ad hoc patch, not on the fixed two-week minor-release cadence |
| Built on | `main`, after the v4.2.1 tag |
| Everything that differs from v4.2.1 | [v4.2.1...release/v4.2.2](https://github.com/dmartsapp/shint/compare/v4.2.1...release/v4.2.2) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.2.2 - unreleased` section) and `docs/src/` |

## Bugs

| Issue | State |
|---|---|
| The other three release workflows (`build.yaml`, `docker-hub.yaml`, `ghcr.yaml`) had the same Go-module-cache collision v4.2.1 fixed only in the standalone Vulnerability Check workflow | **Done** - [`b49bde5`](https://github.com/dmartsapp/shint/commit/b49bde5) |
| [#64](https://github.com/dmartsapp/shint/issues/64) `dns`: a forward query never checks `/etc/hosts`, so `shint dns localhost` NXDOMAINs | **Done** - external contribution by [@littfed](https://github.com/littfed), [PR #66](https://github.com/dmartsapp/shint/pull/66) ([`f16d63f`](https://github.com/dmartsapp/shint/commit/f16d63f), merged as [`62b199c`](https://github.com/dmartsapp/shint/commit/62b199c)). Reviewed and verified before merging (full diff read, all new + existing tests run including `-race`, the 447-case battery, `golangci-lint`/`govulncheck`, and a manual check of the real binary against the original bug); docs/changelog were not part of the PR and still need adding |
| [#65](https://github.com/dmartsapp/shint/issues/65) `rdns` (and every command via `lib.ResolveName`): a name on multiple `/etc/hosts` lines may only resolve its first address | Not started - needs a real repro from the reporting machine first (see the issue) before a fix is targeted |

A smaller gap noticed while investigating #64 but not yet filed as its own issue: `dns <name> PTR` for a non-IP name sends a literal, near-always-empty wire query instead of being rejected the way a non-PTR type is rejected for a literal IP address. Fold into #64's fix, or file separately - not yet decided.

## Other changes on the branch

None currently. `--verbose` was built here first, then moved to `release/v4.3.0` ([`e12584e`](https://github.com/dmartsapp/shint/commit/e12584e)) once it was noticed to be a new capability, not a fix - the project's own semver policy ("Minor: a new capability, existing behaviour intact") puts it in a minor release, not this patch. [`d85073c`](https://github.com/dmartsapp/shint/commit/d85073c) is reverted here ([`1046f8c`](https://github.com/dmartsapp/shint/commit/1046f8c)), not dropped from history.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
