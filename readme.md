# shint v4.1.0 - work in progress

> **This page describes the `release/v4.1.0` branch only**: what the release is meant to deliver, the bugs it fixes, and what has changed on the branch. It is a working page. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Milestone | [v4.1.0](https://github.com/dmartsapp/shint/milestone/1) |
| Sprint | Oct 5 - Oct 18 (work started early, on Sep 20) |
| Everything that differs from `main` | [main...release/v4.1.0](https://github.com/dmartsapp/shint/compare/main...release/v4.1.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.1.0 - unreleased` section) and `docs/src/` |

## Milestone targets

| Target | State |
|---|---|
| Verified downloads: a SHA-256 checksum file for every binary, and a signed build attestation | Done on the branch. The attestation step can only run on a real release tag, so its first run is the release itself |
| A `Check` workflow: `make check` after every merge to `main` | Done on the branch |
| CI failure issues that carry the real run URL | Done on the branch |
| `cidr` - subnet calculator (IPv4 and IPv6, offline) | Code and tests done; documentation pending |
| `wol` - send a Wake-on-LAN magic packet | Code written; tests and documentation pending |
| `ntp` - check this machine's clock against a time server | Code written; tests and documentation pending |
| `web --timing` - where the time went: DNS, connect, TLS, first byte, download | Not started |
| README changes for `main`: new expansion, badge row, support line | Done, but queued for `main` as a README-only commit on the branch `readme/main-4.1.0` - not carried by this branch |

## Bugs

| Issue | State |
|---|---|
| [#13](https://github.com/dmartsapp/shint/issues/13) The tests do not compile on Windows | Fixed on the branch |
| [#12](https://github.com/dmartsapp/shint/issues/12) The release workflow uses an outdated release action | Fixed on the branch; first run on the release tag |

Also on the milestone board but not part of this branch's code: [#11](https://github.com/dmartsapp/shint/issues/11) (the GHCR package is private - a settings change).

## Other changes on the branch

- The tool's expansion is now "Simple Host INspection Toolkit" (`shint --help`, the documentation home page).
- `make vet` also vets for Windows, FreeBSD and Solaris, so a test that only builds on Unix fails in `make check`.
- `make workflows` covers the new scripts (checksums, failure-issue text) and runs `actionlint` with no exceptions.
- Documentation for all of the above: Install ("Verify your download"), CI/CD workflows, Testing, Releases and tagging.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
