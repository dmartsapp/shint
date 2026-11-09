# shint v4.3.0 - work in progress

Summary: `tls`: a server's certificate - chain, trust, names, expiry, protocol and cipher; SBOMs and security reports with every release
Includes:

> The working page for `release/v4.3.0`: what the release is meant to deliver, the bugs it fixes, what changed, and what is left to do - kept up to date as the branch moves. **`Summary:`** is the one plain-language line main's README roadmap will show for this release; **`Includes:`** names releases folded into this one (like `v4.4.0`), if any. `make release` reads both, refuses to publish while anything under **To do** is unchecked, and moves this file to `releases/v4.3.0.md` in the release commit, where it stays on `main` as the release's record. Main's roadmap row links here in the meantime.

| | |
|---|---|
| Milestone | [v4.3.0](https://github.com/dmartsapp/shint/milestone/4) - due Nov 15 |
| Built on | `main`, after the v4.2.1 tag; `main` merged in after the v4.2.2 tag ([`88e4904`](https://github.com/dmartsapp/shint/commit/88e4904)) after the v4.2.3 tag ([`31f831f`](https://github.com/dmartsapp/shint/commit/31f831f)), and since for `main`'s docs and tooling ([`0377625`](https://github.com/dmartsapp/shint/commit/0377625): the profiling tooling, the Performance page and two machines' history); after the v4.2.4 tag - the Go 1.27.3 security patch, and the security and support policy - in the merge after it; and the SBOMs, the image checks and #75's fix, all made on `main`, merged in [`9b84699`](https://github.com/dmartsapp/shint/commit/9b84699) |
| Everything that differs from main | [main...release/v4.3.0](https://github.com/dmartsapp/shint/compare/main...release/v4.3.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.3.0 - unreleased` section) and `docs/src/` |

## To do

- [x] every target and bug below is done, or moved to a later milestone
- [x] the `tls` target has an issue of its own: [#95](https://github.com/dmartsapp/shint/issues/95) (`nmap`, moved to v4.4.0, gets its own there)
- [x] the changelog has an entry for every change a user would notice
- [x] `Summary:` above says what this release is, in plain language
- [x] a clean rehearsal: `make release DRY_RUN=1` (2026-11-09: every check, the live smoke test and release-check passed; nothing pushed)

## Targets

| Target | Issue | State |
|---|---|---|
| `tls` - certificate chain and trust, names, expiry, protocol and cipher; `--warn-days N` (a `WARN` line) and `--fail-days N` (a failed check), `--servername`, `--cacert` | [#95](https://github.com/dmartsapp/shint/issues/95) | Done - [`eab0903`](https://github.com/dmartsapp/shint/commit/eab0903). See [tls](https://dmartsapp.github.io/shint/docs/tls.html) |
| `web -k`: the "TLS verification disabled" warning logs at `ERROR` while exit status is 0 | [#37](https://github.com/dmartsapp/shint/issues/37) | Done - [`eab0903`](https://github.com/dmartsapp/shint/commit/eab0903): a new line level, `WARN`, advisory; `ERROR` always means exit 1 now |
| `web`: `Response Received` instead of `OK` on a response line, and the status's standard name (`status_text`, from the IANA registry) next to its number - text and `--json`, also on `--timing` hops and `listen http` | [#90](https://github.com/dmartsapp/shint/issues/90) | Done - [`4f208a7`](https://github.com/dmartsapp/shint/commit/4f208a7) |
| SBOM + a preserved vulnerability report, for both binaries and images | [#71](https://github.com/dmartsapp/shint/issues/71) | Done on `main` ([`5d9c1fb`](https://github.com/dmartsapp/shint/commit/5d9c1fb), [`2abc3f6`](https://github.com/dmartsapp/shint/commit/2abc3f6), [`e39bfbb`](https://github.com/dmartsapp/shint/commit/e39bfbb)), merged in. The binaries' SBOMs come from `.github/scripts/sbom.py`, the images' from Syft; Grype and Dockle reports, the `govulncheck` report, Docker's own SBOM and provenance attestations, Hadolint on the Dockerfile. v4.0.0 to v4.2.4 have their binaries' SBOMs, and v4.2.4 its images', already (made afterwards). **v4.3.0's tag is the first real run** of Syft, Grype, Dockle and the image attestations - watch the image jobs |

## Bugs

| Issue | State |
|---|---|
| [#75](https://github.com/dmartsapp/shint/issues/75) Release workflows: v4.2.2's cache-collision fix did not work - `golang/govulncheck-action` still restores its cache over the modules `golangci-lint` downloaded (about 1590 `tar: ... Cannot open: File exists` lines per workflow on the v4.2.2 tag) | Done on `main` ([`61fe1cf`](https://github.com/dmartsapp/shint/commit/61fe1cf)), merged in: `Set up Go` restores the cache once, the action's restore is off. The changelog corrects v4.2.2's claim. **Confirm on v4.3.0's tag logs**: no `Cannot open: File exists` lines in the three gate jobs |
| [#84](https://github.com/dmartsapp/shint/issues/84) `listen http`: a client that stopped sending mid-body still got net/http's `200 OK`, while the listener logged "incomplete, nothing answered" - since v4.2.0; found when it made Check on `main` fail now and then ([#83](https://github.com/dmartsapp/shint/issues/83)) | Done - [`7943de4`](https://github.com/dmartsapp/shint/commit/7943de4): the handler aborts, so nothing is written; a new test reproduces it every time |
| [#91](https://github.com/dmartsapp/shint/issues/91) Go 1.27.1 standard-library vulnerabilities: `govulncheck` found 10 that shint's code reaches (GO-2026-6809: compromised intermediate certificates still trusted, and nine more), in every release up to v4.2.3 | Released in [v4.2.4](https://github.com/dmartsapp/shint/releases/tag/v4.2.4) (2026-11-08), a security patch outside the cycle - first made here ([`516e339`](https://github.com/dmartsapp/shint/commit/516e339)), carried to `release/v4.2.4`, and back with `main`. The advisory: [Security](https://dmartsapp.github.io/shint/docs/security.html#2026-11-08-go-1-27-1-standard-library) |

## After the release

- Confirm [#75](https://github.com/dmartsapp/shint/issues/75) in the tag's logs: no `Cannot open: File exists` lines in the gate jobs of Binary Build & Release, Docker Hub Release and GHCR Release.
- Check the first real run of Syft, Grype, Dockle and the image attestations ([#71](https://github.com/dmartsapp/shint/issues/71)): the image jobs green, and the release's 89 assets (`make release` counts them).
- Close #95, #37, #90, #84, #71 and #75 as resolved in v4.3.0, then the milestone.
- On `main`: `tls` in the README's command tables (they can only list a released command).

## Moved

| Target | To | Why |
|---|---|---|
| `nmap` upgrades: `--ports 22,80,443` lists, CIDR/multi-host sweeps, service names | v4.4.0 | Agreed with the user on 2026-11-09: both headline features did not fit the time left before Nov 15; `tls` stays, `nmap` moves. |
| [#65](https://github.com/dmartsapp/shint/issues/65) `rdns` (and every command via `lib.ResolveName`): a name on several hosts-file lines may resolve to its first address only | v4.4.0 | Still needs a reproduction from the machine that showed it (moved here from v4.2.2 already). Nothing changed for it in v4.3.0. |

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
