# shint v4.3.0 - work in progress

Summary: `tls` (certificate chain and expiry checks), `nmap` upgrades: port lists, subnet sweeps, service names
Includes:

> The working page for `release/v4.3.0`: what the release is meant to deliver, the bugs it fixes, what changed, and what is left to do - kept up to date as the branch moves. **`Summary:`** is the one plain-language line main's README roadmap will show for this release; **`Includes:`** names releases folded into this one (like `v4.4.0`), if any. `make release` reads both, refuses to publish while anything under **To do** is unchecked, and moves this file to `releases/v4.3.0.md` in the release commit, where it stays on `main` as the release's record. Main's roadmap row links here in the meantime.

| | |
|---|---|
| Milestone | [v4.3.0](https://github.com/dmartsapp/shint/milestone/4) - due Nov 15 |
| Built on | `main`, after the v4.2.1 tag; `main` merged in after the v4.2.2 tag ([`88e4904`](https://github.com/dmartsapp/shint/commit/88e4904)) after the v4.2.3 tag ([`31f831f`](https://github.com/dmartsapp/shint/commit/31f831f)), and since for `main`'s docs and tooling - latest [`eb41dbd`](https://github.com/dmartsapp/shint/commit/eb41dbd): the profiling tooling and the Performance page |
| Everything that differs from main | [main...release/v4.3.0](https://github.com/dmartsapp/shint/compare/main...release/v4.3.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.3.0 - unreleased` section) and `docs/src/` |

## To do

- [ ] every target and bug below is done, or moved to a later milestone
- [ ] the `tls` and `nmap` targets have issues of their own (they have none yet)
- [ ] the changelog has an entry for every change a user would notice
- [ ] `Summary:` above says what this release is, in plain language
- [ ] a clean rehearsal: `make release DRY_RUN=1`

## Targets

| Target | Issue | State |
|---|---|---|
| `tls`/`cert` - certificate chain, expiry, SANs, protocol/cipher, `--warn-days N` | - | Not started |
| `nmap` upgrades: `--ports 22,80,443` lists, CIDR/multi-host sweeps, service names | - | Not started |
| `web -k`: the "TLS verification disabled" warning logs at `ERROR` while exit status is 0 | [#37](https://github.com/dmartsapp/shint/issues/37) | Not started - fits naturally with the `tls` work |
| SBOM (`syft`) + a preserved vulnerability report (`govulncheck`'s existing scan, plus `grype` for the container images specifically), for both binaries and images | [#71](https://github.com/dmartsapp/shint/issues/71) | Not started - touches `build.yaml`, `docker-hub.yaml`, `ghcr.yaml`, `vulncheck.yaml` together |

## Bugs

| Issue | State |
|---|---|
| [#75](https://github.com/dmartsapp/shint/issues/75) Release workflows: v4.2.2's cache-collision fix did not work - `golang/govulncheck-action` still restores its cache over the modules `golangci-lint` downloaded (about 1590 `tar: ... Cannot open: File exists` lines per workflow on the v4.2.2 tag) | Not started - the fix is in the issue; confirmed only on the next tag's logs. The v4.3.0 changelog should also correct v4.2.2's claim |
| [#65](https://github.com/dmartsapp/shint/issues/65) `rdns` (and every command via `lib.ResolveName`): a name on several hosts-file lines may resolve to its first address only | Not started - moved from v4.2.2; still needs a repro from the machine that showed it |
| [#84](https://github.com/dmartsapp/shint/issues/84) `listen http`: a client that stopped sending mid-body still got net/http's `200 OK`, while the listener logged "incomplete, nothing answered" - since v4.2.0; found when it made Check on `main` fail now and then ([#83](https://github.com/dmartsapp/shint/issues/83)) | Done - [`7943de4`](https://github.com/dmartsapp/shint/commit/7943de4): the handler aborts, so nothing is written; a new test reproduces it every time |

## Dropped

| Target | Issue | Why |
|---|---|---|
| `telnet` banner grabbing, `--send`/`--expect` | [#45](https://github.com/dmartsapp/shint/issues/45), closed as not planned | Dropped 2026-09-26, before any of it was built. The services where "the port opens, but does the service work?" matters most are already checked end to end in their own protocol (`web`, `dns`, `ntp`, and `tls` for TLS, which has no banner to read); `nc host port` shows the greeting of the rest; and `--send`/`--expect` would grow into a protocol scripter (line endings, binary payloads, multi-step exchanges). Naming the service on an open port stays with `nmap`. The full reasoning: [No banner grabbing in telnet](https://dmartsapp.github.io/shint/docs/tech-design.html#no-banner-grabbing-in-telnet). |

## Other changes

- **New flag: `--verbose`** - extra diagnostic lines about a command's internal steps (name resolution so far), to stderr, off by default. **Done** - [`e12584e`](https://github.com/dmartsapp/shint/commit/e12584e). Built first on `release/v4.2.2`, moved here once it was noticed to be a new capability, not a fix - the project's own semver policy puts it in a minor release. See [Verbose](https://dmartsapp.github.io/shint/docs/usage.html#verbose).
- **`main` merged in after the v4.2.3 tag**: `shint --version` shows the Go version and platform ([#77](https://github.com/dmartsapp/shint/issues/77)), the fd-pressure tests no longer fail on the machine's own connection limits ([#78](https://github.com/dmartsapp/shint/issues/78)), and the new release process ([#76](https://github.com/dmartsapp/shint/issues/76)) - which is why this page is `branch_readme.md` now, with a To do list, and `readme.md` on this branch is `main`'s README. Before, this page was the branch's own `readme.md`, put back to `main`'s at release time. How a release is made now: [Making a release](https://dmartsapp.github.io/shint/docs/tech-release.html#making-a-release).
- **The `profiling` build tag's hook** (`profiling.go`, and `stopProfiling` before every exit in `main.go`): with `-tags profiling`, `SHINT_CPUPROFILE`/`SHINT_MEMPROFILE` get CPU and memory profiles of the run, so `make profile` records the binary's own profile per scenario. Release binaries are built without it. The profiling tooling itself (`make profile`, `.profiling/`, and `make release` committing a profile with every release) is on `main`.

### Carried over from v4.2.2

Before v4.2.2 shipped, its changes were cherry-picked here so this branch was built and tested with them. v4.2.2 is released now (2026-09-26) and `main` is merged in ([`88e4904`](https://github.com/dmartsapp/shint/commit/88e4904)), so these are `main`'s own changes; the table stays as a record of the picks. The CI row did not actually fix the collision - see [#75](https://github.com/dmartsapp/shint/issues/75) below.

| Change | On `release/v4.2.2` | Here |
|---|---|---|
| CI: the Go-module-cache collision fixed in `build.yaml`, `docker-hub.yaml` and `ghcr.yaml` | [`b49bde5`](https://github.com/dmartsapp/shint/commit/b49bde5) | [`f20a3c4`](https://github.com/dmartsapp/shint/commit/f20a3c4) |
| [#64](https://github.com/dmartsapp/shint/issues/64) `dns` checks the hosts file and `localhost` before the network - by [@littfed](https://github.com/littfed), [PR #66](https://github.com/dmartsapp/shint/pull/66) | [`f16d63f`](https://github.com/dmartsapp/shint/commit/f16d63f) | [`fa6a733`](https://github.com/dmartsapp/shint/commit/fa6a733) |
| #64 docs and changelog - by [@littfed](https://github.com/littfed), [PR #73](https://github.com/dmartsapp/shint/pull/73) | [`a750d94`](https://github.com/dmartsapp/shint/commit/a750d94) | [`434d004`](https://github.com/dmartsapp/shint/commit/434d004) |
| #64 follow-up: `-4`/`-6` no longer filter hosts-file answers; other record types still need a server | [`39c990e`](https://github.com/dmartsapp/shint/commit/39c990e) | [`dcb8568`](https://github.com/dmartsapp/shint/commit/dcb8568) |
| #64 docs: the hosts file answers A and AAAA only; how to ask DNS itself | [`a6716ef`](https://github.com/dmartsapp/shint/commit/a6716ef) | [`e05d4b3`](https://github.com/dmartsapp/shint/commit/e05d4b3) |
