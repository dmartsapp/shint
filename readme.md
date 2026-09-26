# shint v4.2.2 - work in progress

> **This page describes the `release/v4.2.2` branch only**: what the release is meant to deliver, the bugs it fixes, and what has changed on the branch. It is a working page. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Milestone | [v4.2.2](https://github.com/dmartsapp/shint/milestone/10) - no due date: an ad hoc patch, not on the fixed two-week minor-release cadence |
| Built on | `main`, after the v4.2.1 tag |
| Everything that differs from v4.2.1 | [v4.2.1...release/v4.2.2](https://github.com/dmartsapp/shint/compare/v4.2.1...release/v4.2.2) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.2.2 - unreleased` section) and `docs/src/` |
| State | **Ready for release**: every change below is done and `make test-full` passes on the branch. Next is the release commit (version, dated changelog, `main`'s README) |

## Bugs

| Issue | State |
|---|---|
| The other three release workflows (`build.yaml`, `docker-hub.yaml`, `ghcr.yaml`) had the same Go-module-cache collision v4.2.1 fixed only in the standalone Vulnerability Check workflow | **Done** - [`b49bde5`](https://github.com/dmartsapp/shint/commit/b49bde5) |
| [#64](https://github.com/dmartsapp/shint/issues/64) `dns`: a forward query never checks `/etc/hosts`, so `shint dns localhost` NXDOMAINs | **Done** - external contribution by [@littfed](https://github.com/littfed), [PR #66](https://github.com/dmartsapp/shint/pull/66) ([`f16d63f`](https://github.com/dmartsapp/shint/commit/f16d63f), merged as [`62b199c`](https://github.com/dmartsapp/shint/commit/62b199c)). Reviewed and verified before merging (full diff read, all new + existing tests run including `-race`, the 447-case battery, `golangci-lint`/`govulncheck`, and a manual check of the real binary against the original bug). Docs and changelog also by [@littfed](https://github.com/littfed), [PR #73](https://github.com/dmartsapp/shint/pull/73) ([`a750d94`](https://github.com/dmartsapp/shint/commit/a750d94), merged as [`4e5e268`](https://github.com/dmartsapp/shint/commit/4e5e268)), checked against the real binary's output before merging. Two edge cases found while doing that, fixed on the branch before release: `dns -4 localhost AAAA` answered "no AAAA records", and a name in the hosts file asked for another type with no usable server failed with an empty error instead of "no server to ask" ([`39c990e`](https://github.com/dmartsapp/shint/commit/39c990e), with tests); the docs now say the hosts file answers A and AAAA only ([`a6716ef`](https://github.com/dmartsapp/shint/commit/a6716ef)) |
| [#65](https://github.com/dmartsapp/shint/issues/65) `rdns` (and every command via `lib.ResolveName`): a name on multiple `/etc/hosts` lines may only resolve its first address | Not started - needs a real repro from the reporting machine first (see the issue) before a fix is targeted; does not block this release |

A smaller gap noticed while investigating #64 but not yet filed as its own issue: `dns <name> PTR` for a non-IP name sends a literal, near-always-empty wire query instead of being rejected the way a non-PTR type is rejected for a literal IP address. Fold into #64's fix, or file separately - not yet decided.

Every change on this branch is also cherry-picked onto [`release/v4.3.0`](https://github.com/dmartsapp/shint/tree/release/v4.3.0) ([`f20a3c4`](https://github.com/dmartsapp/shint/commit/f20a3c4), [`fa6a733`](https://github.com/dmartsapp/shint/commit/fa6a733), [`434d004`](https://github.com/dmartsapp/shint/commit/434d004), [`dcb8568`](https://github.com/dmartsapp/shint/commit/dcb8568), [`e05d4b3`](https://github.com/dmartsapp/shint/commit/e05d4b3)), so the next minor release is built and tested with them before this one ships.

## Other changes on the branch

None currently. `--verbose` was built here first, then moved to `release/v4.3.0` ([`e12584e`](https://github.com/dmartsapp/shint/commit/e12584e)) once it was noticed to be a new capability, not a fix - the project's own semver policy ("Minor: a new capability, existing behaviour intact") puts it in a minor release, not this patch. [`d85073c`](https://github.com/dmartsapp/shint/commit/d85073c) is reverted here ([`1046f8c`](https://github.com/dmartsapp/shint/commit/1046f8c)), not dropped from history.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
