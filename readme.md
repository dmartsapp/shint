# shint v4.1.0 - work in progress

> **This page describes the `release/v4.1.0` branch only**: what the release is meant to deliver, the bugs it fixes, and what has changed on the branch. It is a working page. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Milestone | [v4.1.0](https://github.com/dmartsapp/shint/milestone/1) - due Oct 18 |
| Sprint | Oct 5 - Oct 18 (work started early, on Sep 20) |
| Everything that differs from `main` | [main...release/v4.1.0](https://github.com/dmartsapp/shint/compare/main...release/v4.1.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.1.0 - unreleased` section) and `docs/src/` |

## Milestone targets

| Target | Issue | State |
|---|---|---|
| Verified downloads: a SHA-256 checksum file for every binary, and a signed build attestation | [#21](https://github.com/dmartsapp/shint/issues/21) | Done on the branch. The attestation step can only run on a real release tag, so its first run is the release itself |
| `web --timing` - DNS, connect, TLS, first byte, download, per redirect hop | [#20](https://github.com/dmartsapp/shint/issues/20) | Done: code, tests, documentation |
| `ntp` - check the clock against a time server | [#19](https://github.com/dmartsapp/shint/issues/19) | Done: code, tests, documentation |
| `wol` - Wake-on-LAN magic packet | [#18](https://github.com/dmartsapp/shint/issues/18) | Done: code, tests, documentation |
| `cidr` - subnet calculator, offline | [#17](https://github.com/dmartsapp/shint/issues/17) | Done: code, tests, documentation |
| `rdns` - reverse DNS lookup | [#16](https://github.com/dmartsapp/shint/issues/16) | Done: code, tests, documentation |
| Module path `github.com/dmartsapp/shint/v4`, so `go install` works | [#15](https://github.com/dmartsapp/shint/issues/15) | Done on the branch; an outside module imports it at the branch commit. `go install ...@v4.1.0` can only be checked once the tag exists |
| A `Check` workflow: `make check` after every merge to `main` | - | Done on the branch; first run when the branch is merged |
| CI failure issues that carry the real run URL | - | Done on the branch |
| README changes for `main`: new expansion, badge row, support line | - | Done, but queued for `main` as a README-only commit on the branch `readme/main-4.1.0` - not carried by this branch |

## Bugs

| Issue | State |
|---|---|
| [#13](https://github.com/dmartsapp/shint/issues/13) The tests do not compile on Windows | Fixed on the branch |
| [#12](https://github.com/dmartsapp/shint/issues/12) The release workflow uses an outdated release action | Fixed on the branch; first run on the release tag |
| [#22](https://github.com/dmartsapp/shint/issues/22) Listeners print received bytes raw (binary garbles the terminal, escape sequences are interpreted) | Fixed on the branch |

Not code, so not on this branch: [#11](https://github.com/dmartsapp/shint/issues/11) (the GHCR package is private - a settings change).

## Other changes on the branch

- The tool's expansion is now "Simple Host INspection Toolkit" (`shint --help`, the documentation home page).
- `make vet` also vets for Windows, FreeBSD and Solaris, so a test that only builds on Unix fails in `make check`.
- `make workflows` covers the new scripts (checksums, failure-issue text, release preflight) and runs `actionlint` with no exceptions; `make release-check` is the release-day preflight.
- The live smoke test (`make test-live`) covers `web --timing`, `rdns`, `ntp`, `wol` and `cidr`.
- Documentation: a page for each new command, the `web` timing section, Install ("Verify your download", `go install`), CI/CD workflows, Testing, Releases and tagging.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
