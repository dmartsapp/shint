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
- [ ] `ss` has run on a real Windows machine (`shint.exe ss`, `ss -l`, `ss --json`): its Windows code is unit-tested on recorded tables, but the system calls have only been compiled, never run
- [ ] `ping`'s targets ([#14](https://github.com/dmartsapp/shint/issues/14), [#25](https://github.com/dmartsapp/shint/issues/25)): the go-ping release they need is out, and `go.mod` uses it
- [ ] the changelog has an entry for every change a user would notice
- [ ] `Summary:` above says what this release is, in plain language
- [ ] a clean rehearsal: `make release DRY_RUN=1`

## Targets

| Target | Issue | State |
|---|---|---|
| `ip route`: this machine's routing table and default gateway | - (to file) | Not started |
| `ss`: this machine's sockets, each with the process that owns it - one record per socket | [#68](https://github.com/dmartsapp/shint/issues/68) | Done on Linux and Windows - [`65da11b`](https://github.com/dmartsapp/shint/commit/65da11b): `-t`/`-u`/`-l`/`--ports`/`--pid`, `owner=hidden` without privileges, exit 1 when nothing matches; JSON `processes`, `owner_hidden`. Other systems say "not supported yet"; the BSDs are [#105](https://github.com/dmartsapp/shint/issues/105) (v4.5.0). The AIX binary is built without `ss` (no unprivileged interface exists there). Windows still needs a real run (see To do) |
| `ping`: sub-millisecond timings, unreachable replies told apart from timeouts, statistics on Ctrl+C - needs a new go-ping release | [#25](https://github.com/dmartsapp/shint/issues/25) | Not started |
| `--interface`: send a check's traffic through a given interface or source address | [#69](https://github.com/dmartsapp/shint/issues/69) | Not started |
| Responsible use: a sweep of public addresses warns and is paced by `--delay`; `web`'s User-Agent names shint and its release; the rule on the Security and design pages | [#106](https://github.com/dmartsapp/shint/issues/106) | Done - [`d492b69`](https://github.com/dmartsapp/shint/commit/d492b69): `WARN public addresses` with probes and estimate, probes spaced by `--delay` (1 s default, `0` for none); JSON `warnings`, `delay_ms`; User-Agent `shint/<release> (+URL)`; Security "Responsible use", design "Checking, not reconnaissance" |
| `nmap` upgrades: port lists (`--ports 22,80,443`), subnet sweeps, service names - moved from v4.3.0 | [#94](https://github.com/dmartsapp/shint/issues/94) | Done - [`3554ffc`](https://github.com/dmartsapp/shint/commit/3554ffc): `--ports`/`-p`, several targets and prefixes (up to 65,536 addresses), `service=`; JSON `target`, `service`, `ports`, `targets`, `dns_lookups` |
| `web --json`: say when certificate checks are skipped, and when a client certificate or a custom CA is used (security review) | [#100](https://github.com/dmartsapp/shint/issues/100) | Done - [`379da60`](https://github.com/dmartsapp/shint/commit/379da60): `input_params.tls`, top-level `warnings` |
| `listen http`: default read timeouts, and a start-up line when it is reachable from the network (security review) | [#102](https://github.com/dmartsapp/shint/issues/102) | Done - [`cf919a5`](https://github.com/dmartsapp/shint/commit/cf919a5): the start-up warning; the timeouts were already there (`--timeout`, 5 s by default) - the issue's premise was wrong |

## Bugs

| Issue | State |
|---|---|
| [#98](https://github.com/dmartsapp/shint/issues/98) `web`: a redirect to another host gets the client certificate and the custom headers (security review) | Done - [`b5c04b5`](https://github.com/dmartsapp/shint/commit/b5c04b5): the redirect is the response, with a `WARN` line and `redirect_not_followed` in the JSON |
| [#99](https://github.com/dmartsapp/shint/issues/99) `web --json`: credential headers' values (`Authorization`, `Cookie`...) are written as given (security review) | Done - [`3664001`](https://github.com/dmartsapp/shint/commit/3664001): `[redacted]` by default, a URL's password as `xxxxx`; `--show-credentials` |
| [#101](https://github.com/dmartsapp/shint/issues/101) `web --json -W`: the response body is kept whole in memory, however large (security review) | Done - [`5e66789`](https://github.com/dmartsapp/shint/commit/5e66789): `--max-body` (10M by default), `body_truncated` |
| [#65](https://github.com/dmartsapp/shint/issues/65) `rdns` (and every command through `lib.ResolveName`): a name on several hosts-file lines may resolve to its first address only - moved from v4.3.0, needs a reproduction | Not started |
| [#14](https://github.com/dmartsapp/shint/issues/14) `ping --timeout` does not cover the DNS lookup (go-ping resolves with a fixed 5 s limit) - needs a new go-ping release | Not started |

## Other changes

| Change | Where | State |
|---|---|---|
| CI: actions pinned to commits, base images to digests, and a check that refuses unpinned ones (security review) - [#103](https://github.com/dmartsapp/shint/issues/103) | `main` (CI only, no release needed) | Done on `main` - [`db0fe0b`](https://github.com/dmartsapp/shint/commit/db0fe0b): 56 `uses:` pinned to commits, both base images to digests, and `check-pins.py` in `make workflows`; merged in by `make release` |

## After the release

- `main`'s README: link the Limitations page (`https://dmartsapp.github.io/shint/docs/limitations.html`) from its Limitations section, and say there that `ss` covers Linux and Windows only and is not in the AIX binary, and that a sweep of public addresses warns and is paced by `--delay` (#106). Add `ss` to the Commands table if the reconcile has not.
- Record the README demo again with an `ss -l` line (`vhs docs/assets/demo.tape`), now that `ss` is released.
