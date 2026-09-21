# shint v4.2.0 - work in progress

> **This page describes the `release/v4.2.0` branch only**: what the release is meant to deliver, the bugs it fixes, and what has changed on the branch. It is a working page. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Milestone | [v4.2.0](https://github.com/dmartsapp/shint/milestone/3) - due Nov 1 |
| Sprint | Oct 19 - Nov 1 (work started early, on Sep 21) |
| Built on | `release/v4.1.0`, which is not on `main` yet: 4.2.0 uses its code (the `/v4` module path, `-4`/`-6`, `rdns`). When 4.1.0 is released this branch is rebased onto `main` |
| Everything that differs from 4.1.0 | [release/v4.1.0...release/v4.2.0](https://github.com/dmartsapp/shint/compare/release/v4.1.0...release/v4.2.0) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.2.0 - unreleased` section) and `docs/src/` |

## Milestone targets

| Target | Issue | State |
|---|---|---|
| `ip` - this machine's interfaces and addresses | [#42](https://github.com/dmartsapp/shint/issues/42) | Planned first: new code only, no conflict with the open 4.1.0 fixes |
| `dns` - dig-style lookups: A, AAAA, MX, TXT, NS, CNAME, SRV, SOA, PTR, `@server`, TTLs, timing | [#43](https://github.com/dmartsapp/shint/issues/43) | Planned second: new code only |
| Authoritative name servers shown whenever shint resolves a name | [#44](https://github.com/dmartsapp/shint/issues/44) | Planned, after the open 4.1.0 fixes (it touches every handler's resolve step) |
| `telnet` banner grabbing, `--send` / `--expect` | [#45](https://github.com/dmartsapp/shint/issues/45) | Planned, after 4.1.0's #31 (the telnet attempt loop) |
| `udp --hex` - send a binary payload | [#46](https://github.com/dmartsapp/shint/issues/46) | Planned, after 4.1.0's #29 and #32 (the udp handler) |

## Bugs

Found by the black-box battery and planned for this release (all `fix-future-release`):

| Issue | State |
|---|---|
| [#35](https://github.com/dmartsapp/shint/issues/35) `listen http`: malformed or timed-out requests are neither logged nor counted | Planned, after 4.1.0's #34 (same code) |
| [#36](https://github.com/dmartsapp/shint/issues/36) `web` ignores `HTTP_PROXY` / `HTTPS_PROXY`, and the documentation does not say so | Planned (documentation) |
| [#38](https://github.com/dmartsapp/shint/issues/38) `web`: a URL without a scheme (`host:port/path`) fails with only "Invalid URL" | Planned |
| [#39](https://github.com/dmartsapp/shint/issues/39) `listen tcp`: one log line per 4 KB read makes a large transfer unreadable | Planned |
| [#40](https://github.com/dmartsapp/shint/issues/40) `listen udp`: `--timeout` is accepted and does nothing | Planned (documentation) |

## Other changes on the branch

None yet.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
