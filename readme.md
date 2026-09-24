# shint v4.2.0 - released 2026-09-21

> **This page describes the `release/v4.2.0` branch only**: what the release delivered, the bugs it fixed, and what changed on the branch. It is a record of that release, added after the tag and not part of it. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Release | [v4.2.0](https://github.com/dmartsapp/shint/releases/tag/v4.2.0), tag on [`0b7a671`](https://github.com/dmartsapp/shint/commit/0b7a671), 2026-09-21 |
| Kind | Minor - two sprints' worth, folded into one release: `release/v4.1.0`'s work was merged into this branch and **v4.1.0 was never tagged on its own** |
| Milestones | [v4.1.0](https://github.com/dmartsapp/shint/milestone/1) and [v4.2.0](https://github.com/dmartsapp/shint/milestone/3), both closed |
| Everything that differs from the previous release | [v4.0.6...v4.2.0](https://github.com/dmartsapp/shint/compare/v4.0.6...v4.2.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.2.0` and `v4.1.0` sections) and `docs/src/` |

## New commands and features

**Shipped in v4.1.0** (superseded before its own tag; the work is real, the release number is not):

| Target | Delivered in |
|---|---|
| `ntp <server>` - check this machine's clock against an NTP server | [`9fe6294`](https://github.com/dmartsapp/shint/commit/9fe6294) (Issue #19) |
| `wol <mac>` - send a Wake-on-LAN magic packet | [`9fe6294`](https://github.com/dmartsapp/shint/commit/9fe6294) (Issue #18) |
| `rdns <ip-or-host>` - reverse DNS lookup | [`fca06fc`](https://github.com/dmartsapp/shint/commit/fca06fc) (Issue #16) |
| `cidr <prefix>...` - offline subnet calculator | [`9fe6294`](https://github.com/dmartsapp/shint/commit/9fe6294) (Issue #17) |
| `-4`/`--ipv4` and `-6`/`--ipv6` - check one address family only | [`dd7b2e5`](https://github.com/dmartsapp/shint/commit/dd7b2e5) (Issue #23) |
| `web --timing` - DNS/connect/TLS/wait/download breakdown, per redirect hop | [`04b1cb2`](https://github.com/dmartsapp/shint/commit/04b1cb2) (Issue #20) |
| `Ctrl+C` shows the summary on `ntp`, `rdns` and `wol` too | [`4f0596a`](https://github.com/dmartsapp/shint/commit/4f0596a) (Issue #24) |
| Module path `github.com/dmartsapp/shint/v4`, so `go install .../v4@latest` works | [`e7c9900`](https://github.com/dmartsapp/shint/commit/e7c9900) (Issue #15) |
| SHA-256 checksum and signed build attestation for every release binary | [`c33657f`](https://github.com/dmartsapp/shint/commit/c33657f) |

**New in v4.2.0 itself:**

| Target | Delivered in |
|---|---|
| `ip` - this machine's network interfaces and addresses | [`c1ce7c7`](https://github.com/dmartsapp/shint/commit/c1ce7c7) (Issue #42) |
| `dns <name> [type] [@server]` - DNS lookups, like `dig`, in shint's format | [`47dd568`](https://github.com/dmartsapp/shint/commit/47dd568) (Issue #43) |
| Every command that resolves a name shows who runs its DNS (`dns authoritative ...`) | [`1c14005`](https://github.com/dmartsapp/shint/commit/1c14005) (Issue #44) |
| `udp --hex` sends exact bytes, for binary probes | [`b228df5`](https://github.com/dmartsapp/shint/commit/b228df5) (Issue #46) |

## Bugs fixed

Mostly what the black-box test battery (added in v4.0.6) found, still open at the start of this branch:

| Bug | Fix |
|---|---|
| `udp --payload -1` (or a huge value) crashed with a stack trace; `--timeout`/`--delay` silently overflowed | [`2d16da1`](https://github.com/dmartsapp/shint/commit/2d16da1) (Issues #29, #30) |
| `listen`/`completion` printed the help page and exited `0` for an unknown subcommand; `listen --count -1` was accepted as "unlimited" | [`2d16da1`](https://github.com/dmartsapp/shint/commit/2d16da1) (Issue #33) |
| `udp` logged a closed probe as `OK`; `ping` logged a send error as `OK` - both while exiting `1` | [`2d16da1`](https://github.com/dmartsapp/shint/commit/2d16da1) (Issue #32) |
| `--count N --delay 0` opened a socket per attempt, so a low file-descriptor limit failed attempts that had nothing wrong | [`81833ed`](https://github.com/dmartsapp/shint/commit/81833ed) (Issue #31) |
| `web` held the whole response body in memory (662 MB peak for a 300 MB download) | [`ed7c7dd`](https://github.com/dmartsapp/shint/commit/ed7c7dd) (Issue #28) |
| `listen http` never logged or counted a request it could not serve (garbage, a stalled body) | [`e885952`](https://github.com/dmartsapp/shint/commit/e885952) (Issues #34, #35) |
| `web` mishandled a URL without a scheme, and `listen tcp` logged one line per 4 KB read instead of per burst | [`f8decf6`](https://github.com/dmartsapp/shint/commit/f8decf6) (Issues #38, #39) |
| The listener/`udp` reply preview could garble the terminal on unsafe bytes | [`130c73a`](https://github.com/dmartsapp/shint/commit/130c73a) (Issue #22) |
| `cidr` error messages leaked `net/netip`'s internal wording instead of naming the cause | [`73c520f`](https://github.com/dmartsapp/shint/commit/73c520f) |
| The tests did not compile on Windows (`syscall.Kill`); `make vet` did not check other operating systems | [`5c032c8`](https://github.com/dmartsapp/shint/commit/5c032c8) (Issue #13) |

Documented rather than changed: `web` never uses a proxy, and `--timeout` has no effect on `listen udp` (Issues #36, #40) - both in [`f8decf6`](https://github.com/dmartsapp/shint/commit/f8decf6).

## Other changes on the branch

- **`listen tcp` was removed, then restored.** [`82543df`](https://github.com/dmartsapp/shint/commit/82543df) removed it as a breaking-change exploration; [`12072f4`](https://github.com/dmartsapp/shint/commit/12072f4) reverted that. It stays in the 4.x line; its removal is parked for v5.0.0 as Issue #48, documented on `main`'s README.
- **Release integrity and process**, from the v4.1.0 side of the branch: the `Check` workflow running `make check` after every merge to `main` ([`f656b08`](https://github.com/dmartsapp/shint/commit/f656b08)), the release created with `softprops/action-gh-release@v3` ([`aa53ff3`](https://github.com/dmartsapp/shint/commit/aa53ff3), Issue #12), a release branch's README scoped to that branch and `make release-check` as the release-day preflight ([`954b278`](https://github.com/dmartsapp/shint/commit/954b278)), and CI-failure issues carrying the real run URL ([`f656b08`](https://github.com/dmartsapp/shint/commit/f656b08)).
- **The tool's expansion is now "Simple Host INspection Toolkit"** ([`f656b08`](https://github.com/dmartsapp/shint/commit/f656b08)) - the name, binary, repository, image and every URL are unchanged.
- Docs for every new command, the module-path change, and web timing ([`060fd20`](https://github.com/dmartsapp/shint/commit/060fd20)).
- The version bump, the dated changelogs (both v4.1.0's and v4.2.0's) and the docs version examples are in the release commit [`e28b859`](https://github.com/dmartsapp/shint/commit/e28b859).

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
