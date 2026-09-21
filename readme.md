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
| `ip` - this machine's interfaces and addresses | [#42](https://github.com/dmartsapp/shint/issues/42) | **Done**: code, tests, documentation page, battery cases |
| `dns` - dig-style lookups: A, AAAA, MX, TXT, NS, CNAME, SRV, SOA, PTR, `@server`, TTLs, timing | [#43](https://github.com/dmartsapp/shint/issues/43) | **Done**: code, tests against a fake DNS server, documentation page, cookbook recipe, battery cases (a Python DNS server) |
| Authoritative name servers shown whenever shint resolves a name | [#44](https://github.com/dmartsapp/shint/issues/44) | **Done**: every command that resolves a name prints `dns authoritative`; `dns_lookup.authoritative` with `--json` |
| `telnet` banner grabbing, `--send` / `--expect` | [#45](https://github.com/dmartsapp/shint/issues/45) | Waiting for 4.1.0's #31 (it changes the telnet attempt loop this touches) |
| `udp --hex` - send a binary payload | [#46](https://github.com/dmartsapp/shint/issues/46) | Waiting for 4.1.0's #29 and #32 (they change the udp handler this touches) |

## Bugs

Found by the black-box battery and planned for this release (all `fix-future-release`):

| Issue | State |
|---|---|
| [#35](https://github.com/dmartsapp/shint/issues/35) `listen http`: malformed or timed-out requests are neither logged nor counted | Waiting for 4.1.0's #34 (same code) |
| [#36](https://github.com/dmartsapp/shint/issues/36) `web` ignores `HTTP_PROXY` / `HTTPS_PROXY`, and the documentation does not say so | **Done**: documented in `web --help`, the web page and Troubleshooting, and pinned by a test |
| [#38](https://github.com/dmartsapp/shint/issues/38) `web`: a URL without a scheme (`host:port/path`) fails with only "Invalid URL" | **Done**: fetched over `https://`, clear usage errors, a hint for a plain-HTTP server |
| [#39](https://github.com/dmartsapp/shint/issues/39) `listen tcp`: one log line per 4 KB read makes a large transfer unreadable | **Done**: reads that arrive together are one line; a 20 MB upload is 24 lines, not 4,887 |
| [#40](https://github.com/dmartsapp/shint/issues/40) `listen udp`: `--timeout` is accepted and does nothing | **Done**: documented in the flag help and the listen page |

## Other changes on the branch

- **`escapeBytes`**: the escaping that keeps untrusted bytes from garbling the terminal (from the listener previews) is now one shared helper, used by the listeners, `udp` and `dns`.
- **`golang.org/x/net`** (for `dnsmessage`) is now a direct dependency; it was already in the module through go-ping.
- **The battery** has grown from 356 to 415 cases (`ip`, `dns` against a Python DNS server, the URL cases, the listener log size, the authoritative line), and its list of known issues is down to the 16 still open.
- **The live smoke test** checks `ip`, `dns` (forward and reverse) and the authoritative line, and its IPv4-only telnet check no longer depends on how many addresses google.com has.

## Still to do on this branch

The three items that touch code the open 4.1.0 fixes are changing - `telnet`'s attempt loop (#31), the `udp` handler (#29, #32) and the HTTP listener (#34) - wait until those are settled, to avoid rebasing the same lines twice.

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
