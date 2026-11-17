# shint v4.4.0 - work in progress

Summary: this machine's tables (`ip route`, `ss`), a richer `ping`, `nmap` port lists and sweeps, and `web` and `listen` hardened after an outside security review
Includes:

> The working page for `release/v4.4.0`: what the release is meant to deliver, the bugs it fixes, what changed, and what is left to do - kept up to date as the branch moves. **`Summary:`** is the one plain-language line main's README roadmap will show for this release; **`Includes:`** names releases folded into this one (like `v4.4.0`), if any. `make release` reads both, refuses to publish while anything under **To do** is unchecked, and moves this file to `releases/v4.4.0.md` in the release commit, where it stays on `main` as the release's record. Main's roadmap row links here in the meantime.

| | |
|---|---|
| Milestone | [v4.4.0](https://github.com/dmartsapp/shint/milestone/2) - due 2026-11-29 |
| Built on | `main`, after the v4.3.0 tag |
| Everything that differs from main | [main...release/v4.4.0](https://github.com/dmartsapp/shint/compare/main...release/v4.4.0) |

## To do

- [ ] every target and bug below is done, or moved to a later milestone
- [ ] the `ip route` target has an issue of its own
- [ ] `ping`'s targets ([#14](https://github.com/dmartsapp/shint/issues/14), [#25](https://github.com/dmartsapp/shint/issues/25)): the go-ping release they need is out, and `go.mod` uses it
- [ ] the changelog has an entry for every change a user would notice
- [ ] `Summary:` above says what this release is, in plain language
- [ ] a clean rehearsal: `make release DRY_RUN=1`

## Targets

| Target | Issue | State |
|---|---|---|
| `ip route`: this machine's routing table and default gateway | - (to file) | Not started |
| `ss`: this machine's sockets, each with the process that owns it - one record per socket | [#68](https://github.com/dmartsapp/shint/issues/68) | Not started |
| `ping`: sub-millisecond timings, unreachable replies told apart from timeouts, statistics on Ctrl+C - needs a new go-ping release | [#25](https://github.com/dmartsapp/shint/issues/25) | Not started |
| `--interface`: send a check's traffic through a given interface or source address | [#69](https://github.com/dmartsapp/shint/issues/69) | Not started |
| `nmap` upgrades: port lists (`--ports 22,80,443`), subnet sweeps, service names - moved from v4.3.0 | [#94](https://github.com/dmartsapp/shint/issues/94) | Not started |
| `web --json`: say when certificate checks are skipped, and when a client certificate or a custom CA is used (security review) | [#100](https://github.com/dmartsapp/shint/issues/100) | Not started |
| `listen http`: default read timeouts, and a start-up line when it is reachable from the network (security review) | [#102](https://github.com/dmartsapp/shint/issues/102) | Not started |

## Bugs

| Issue | State |
|---|---|
| [#98](https://github.com/dmartsapp/shint/issues/98) `web`: a redirect to another host gets the client certificate and the custom headers (security review) | Done - [`b5c04b5`](https://github.com/dmartsapp/shint/commit/b5c04b5): the redirect is the response, with a `WARN` line and `redirect_not_followed` in the JSON |
| [#99](https://github.com/dmartsapp/shint/issues/99) `web --json`: credential headers' values (`Authorization`, `Cookie`...) are written as given (security review) | Not started |
| [#101](https://github.com/dmartsapp/shint/issues/101) `web --json -W`: the response body is kept whole in memory, however large (security review) | Not started |
| [#65](https://github.com/dmartsapp/shint/issues/65) `rdns` (and every command through `lib.ResolveName`): a name on several hosts-file lines may resolve to its first address only - moved from v4.3.0, needs a reproduction | Not started |
| [#14](https://github.com/dmartsapp/shint/issues/14) `ping --timeout` does not cover the DNS lookup (go-ping resolves with a fixed 5 s limit) - needs a new go-ping release | Not started |

## Other changes

| Change | Where | State |
|---|---|---|
| CI: actions pinned to commits, base images to digests, and a check that refuses unpinned ones (security review) - [#103](https://github.com/dmartsapp/shint/issues/103) | `main` (CI only, no release needed) | Not started |
