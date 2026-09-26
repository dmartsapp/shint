# shint v4.3.0 - work in progress

> **This page describes the `release/v4.3.0` branch only**: what the release is meant to deliver, the bugs it fixes, and what has changed on the branch. It is a working page. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Milestone | [v4.3.0](https://github.com/dmartsapp/shint/milestone/4) - due Nov 15 |
| Built on | `main`, after the v4.2.1 tag; `main` merged in after the v4.2.2 tag ([`88e4904`](https://github.com/dmartsapp/shint/commit/88e4904)) |
| Everything that differs from main | [main...release/v4.3.0](https://github.com/dmartsapp/shint/compare/main...release/v4.3.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.3.0 - unreleased` section) and `docs/src/` |

## Milestone targets

| Target | Issue | State |
|---|---|---|
| `tls`/`cert` - certificate chain, expiry, SANs, protocol/cipher, `--warn-days N` | - | Not started |
| `nmap` upgrades: `--ports 22,80,443` lists, CIDR/multi-host sweeps, service names | - | Not started |
| `telnet` banner grabbing, `--send`/`--expect` | [#45](https://github.com/dmartsapp/shint/issues/45) | Not started |
| `web -k`: the "TLS verification disabled" warning logs at `ERROR` while exit status is 0 | [#37](https://github.com/dmartsapp/shint/issues/37) | Not started - fits naturally with the `tls` work |
| SBOM (`syft`) + a preserved vulnerability report (`govulncheck`'s existing scan, plus `grype` for the container images specifically), for both binaries and images | [#71](https://github.com/dmartsapp/shint/issues/71) | Not started - touches `build.yaml`, `docker-hub.yaml`, `ghcr.yaml`, `vulncheck.yaml` together |

## Carried over from v4.2.2

Before v4.2.2 shipped, its changes were cherry-picked here so this branch was built and tested with them. v4.2.2 is released now (2026-09-26) and `main` is merged in ([`88e4904`](https://github.com/dmartsapp/shint/commit/88e4904)), so these are `main`'s own changes; the table stays as a record of the picks. The CI row did not actually fix the collision - see [#75](https://github.com/dmartsapp/shint/issues/75) below.

| Change | On `release/v4.2.2` | Here |
|---|---|---|
| CI: the Go-module-cache collision fixed in `build.yaml`, `docker-hub.yaml` and `ghcr.yaml` | [`b49bde5`](https://github.com/dmartsapp/shint/commit/b49bde5) | [`f20a3c4`](https://github.com/dmartsapp/shint/commit/f20a3c4) |
| [#64](https://github.com/dmartsapp/shint/issues/64) `dns` checks the hosts file and `localhost` before the network - by [@littfed](https://github.com/littfed), [PR #66](https://github.com/dmartsapp/shint/pull/66) | [`f16d63f`](https://github.com/dmartsapp/shint/commit/f16d63f) | [`fa6a733`](https://github.com/dmartsapp/shint/commit/fa6a733) |
| #64 docs and changelog - by [@littfed](https://github.com/littfed), [PR #73](https://github.com/dmartsapp/shint/pull/73) | [`a750d94`](https://github.com/dmartsapp/shint/commit/a750d94) | [`434d004`](https://github.com/dmartsapp/shint/commit/434d004) |
| #64 follow-up: `-4`/`-6` no longer filter hosts-file answers; other record types still need a server | [`39c990e`](https://github.com/dmartsapp/shint/commit/39c990e) | [`dcb8568`](https://github.com/dmartsapp/shint/commit/dcb8568) |
| #64 docs: the hosts file answers A and AAAA only; how to ask DNS itself | [`a6716ef`](https://github.com/dmartsapp/shint/commit/a6716ef) | [`e05d4b3`](https://github.com/dmartsapp/shint/commit/e05d4b3) |

## Other changes on the branch

- **New flag: `--verbose`** - extra diagnostic lines about a command's internal steps (name resolution so far), to stderr, off by default. **Done** - [`e12584e`](https://github.com/dmartsapp/shint/commit/e12584e). Built first on `release/v4.2.2`, moved here once it was noticed to be a new capability, not a fix - the project's own semver policy puts it in a minor release. See [Verbose](https://dmartsapp.github.io/shint/docs/usage.html#verbose).

## Bugs

| Issue | State |
|---|---|
| [#75](https://github.com/dmartsapp/shint/issues/75) Release workflows: v4.2.2's cache-collision fix did not work - `golang/govulncheck-action` still restores its cache over the modules `golangci-lint` downloaded (about 1590 `tar: ... Cannot open: File exists` lines per workflow on the v4.2.2 tag) | Not started - the fix is in the issue; confirmed only on the next tag's logs. The v4.3.0 changelog should also correct v4.2.2's claim |
| [#65](https://github.com/dmartsapp/shint/issues/65) `rdns` (and every command via `lib.ResolveName`): a name on several hosts-file lines may resolve to its first address only | Not started - moved from v4.2.2; still needs a repro from the machine that showed it |

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
