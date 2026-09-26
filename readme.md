# shint v4.3.0 - work in progress

> **This page describes the `release/v4.3.0` branch only**: what the release is meant to deliver, the bugs it fixes, and what has changed on the branch. It is a working page. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Milestone | [v4.3.0](https://github.com/dmartsapp/shint/milestone/4) - due Nov 15 |
| Built on | `main`, after the v4.2.1 tag, with v4.2.2's fixes cherry-picked ([below](#carried-over-from-v422)) |
| Everything that differs from main | [main...release/v4.3.0](https://github.com/dmartsapp/shint/compare/main...release/v4.3.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.3.0 - unreleased` section; `v4.2.2 - unreleased` below it until v4.2.2 ships) and `docs/src/` |

## Milestone targets

| Target | Issue | State |
|---|---|---|
| `tls`/`cert` - certificate chain, expiry, SANs, protocol/cipher, `--warn-days N` | - | Not started |
| `nmap` upgrades: `--ports 22,80,443` lists, CIDR/multi-host sweeps, service names | - | Not started |
| `telnet` banner grabbing, `--send`/`--expect` | [#45](https://github.com/dmartsapp/shint/issues/45) | Not started |
| `web -k`: the "TLS verification disabled" warning logs at `ERROR` while exit status is 0 | [#37](https://github.com/dmartsapp/shint/issues/37) | Not started - fits naturally with the `tls` work |
| SBOM (`syft`) + a preserved vulnerability report (`govulncheck`'s existing scan, plus `grype` for the container images specifically), for both binaries and images | [#71](https://github.com/dmartsapp/shint/issues/71) | Not started - touches `build.yaml`, `docker-hub.yaml`, `ghcr.yaml`, `vulncheck.yaml` together |

## Carried over from v4.2.2

v4.2.2 is not released yet. Its changes are cherry-picked here so this branch is built and tested with them; once v4.2.2 ships, `main` is brought into this branch and they become `main`'s own commits.

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

None found on this branch yet.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
