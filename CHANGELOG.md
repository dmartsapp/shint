# Changelog

Notable changes to shint, newest first. Versions follow [semantic versioning](https://semver.org); a change that scripts could notice is called out in bold. Full documentation: https://dmartsapp.github.io/shint/

Releases before v3.0.0 predate this file; see the [GitHub releases](https://github.com/dmartsapp/shint/releases) and tags.

## v4.0.3 - 2026-09-20

- **Exit status now follows convention.** A failed check used to exit `0`, so `if shint telnet host 22; then ...` could never take the "down" branch. Now, following `fping` (which also checks several targets per run): **`0`** every check passed, **`1`** at least one check failed - connection refused or timed out, DNS failure, no HTTP response (a 404 or 500 is still a response), a UDP port reported closed, a lost ping, or a scan cut short - and **`2`** the command was used wrongly (bad argument, flag or value; nothing ran). An `nmap` scan that finds no open ports is a completed scan and exits `0`; an inconclusive UDP `open|filtered` is not a failure. Usage errors are printed to stderr (previously stdout, and cobra's error was printed twice), so results on stdout stay clean. Scripts that relied on the old always-`0` behaviour need updating.
- `--json` stays JSON when a check fails: `web` records a failed attempt in `stats` (`success: false`, `errors`, and the bytes that did travel) instead of printing a text line in front of the document, and `ping` prints a JSON document, not a text line, when the lookup fails.
- `web`'s help text no longer claims it does not follow redirects; it does (up to 10), and counts bytes across every hop.
- Fixed `telnet` reporting false failures on longer runs: `--timeout` was also being used as a deadline for the *whole* command, so once `--count` x `--delay` outlasted it (8 attempts one second apart against the default 5s), every later attempt failed instantly with `i/o timeout` even though the port was open. `--timeout` is now what each attempt (and the DNS lookup) gets, like every other command. `nmap` had the same flaw and is fixed the same way, below; no other command carried it.
- Fixed `nmap` not scanning the whole `--from`..`--to` range: `--timeout` was also being used as a deadline for the *entire* scan, so any scan that took longer than `--timeout` seconds (a wide range against a host that drops packets, where every filtered port runs out its own timeout) silently stopped partway - e.g. `--from 20 --to 9000` against such a host attempted about 2,000 of 8,981 ports and never reached port 5000. `--timeout` is now only the per-port connect timeout (as it is for every other command), and the scan runs to the end of the range. Trade-off: scans of hosts with many unresponsive ports take as long as they actually need - see the [nmap page](https://dmartsapp.github.io/shint/docs/nmap.html) for how to estimate and speed that up.
- `nmap` now reports what it actually did: `ports_scanned` counts ports whose probe finished (it used to count ports started), a dial aborted by cancellation is no longer recorded as a closed port, and a scan stopped by Ctrl+C says `scan interrupted ports_scanned=X ports_total=Y` (an `error` field in `--json`) rather than looking finished. `--throttle`'s random per-port delay can now be interrupted with Ctrl+C.
- `nmap` text output now shows how far a scan has got: a `scan started` line (`ports_total`, `timeout`, `max_in_flight`) and a `progress` line every 3 seconds (`ports_scanned=X/Y percent open in_flight elapsed`), so a long scan of unresponsive ports no longer looks hung. Quick scans print no progress lines and `--json` output is unchanged.
- **Documentation.** A full documentation site on GitHub Pages (https://dmartsapp.github.io/shint/): installation for every platform, every command with real, captured output, the text and JSON output formats, a cookbook and troubleshooting guide, and a Technical section covering the architecture, every source file, the CI/CD workflows, the release and tagging philosophy and the testing approach. The README is now a short introduction, this changelog moved out of it into its own file, and code comments were added across the sources.
- `basic_module_test.sh` (the live-internet smoke test) copes with real exit statuses: under `set -e` a failing command would otherwise have ended the script instead of being counted as one failed test.

## v4.0.2 - 2026-09-19

- `web` and `listen http` now measure bytes the same way: everything on the wire, headers included, counted at the connection instead of estimated from the parsed message. v4.0.1's `web` figure estimated response-header size and never measured what was sent, while `listen http` reported body bytes only, so the two never agreed. `web` now reports `bytes_sent` and `bytes_received` (`--json` and text), summed over every hop if a redirect is followed - `bytes_received` replaces `bytes_downloaded`, and `bandwidth_kbs` is derived from it - and `listen http`'s `bytes_received` / `bytes_sent` are the full request and response, so `web`'s `bytes_sent` equals the listener's `bytes_received` and vice versa. The listener also totals both in its exit summary.
- `web --json`: `input_params.payload_bytes` is now the request body size; it used to add the number of `-H` flags to it.
- `web` text output: the response line no longer repeats the status three times over (`response ok ... status="200 OK"` is now `response ... status=200`).
- `listen http` reads the request body before answering (it already discarded it in v4.0.1, but only after the response went out): Go's HTTP client abandons an upload that is still in progress once a complete `Connection: close` response comes back, so a large POST/PUT (past roughly the socket buffer - about 170KB on loopback) could be cut off partway.
- `listen http` responses carry no validators (`ETag`/`Last-Modified`) and request headers are never acted on - see [HTTP listener](https://dmartsapp.github.io/shint/docs/listen.html#http).

## v4.0.1 - 2026-09-19

- `listen tcp` now measures each read (+ optional echo write) as one handled "request": text-mode `data received`/`connection closed`/`done` lines and JSON-mode `ListenEvent`s report `bytes_sent` alongside the existing `bytes_received`, plus `processing_time_µs` (`time_taken` in text mode) for how long that read/echo cycle took.
- `listen http` now reports `bytes_received` (the request body's length, now drained rather than ignored), `bytes_sent` (the response body's length), and `processing_time_µs`/`time_taken` for every request, in both text and `--json` mode.
- Fixed `web`'s bandwidth figure (`speed` in text mode, now also exposed as `bandwidth_kbs` in `--json` output): it previously added `len(header)` - the number of header *keys*, not their byte size - onto the downloaded-bytes count, understating the real transfer size and the KB/s derived from it. Both now use an actual header byte-size estimate.

## v4.0.0 - 2026-09-19

- **Full dual-stack IPv6 support.** `telnet`, `nmap`, `udp`, `web`, and `listen` all resolve and operate over both IPv4 and IPv6 now (a dual-stack hostname is checked/scanned/pinged over both in the same run) - `NetworkType` changed from `"ip4"` to `"ip"`. `listen tcp`/`listen udp`/`listen http` accept `--bind ::` for IPv6 (or dual-stack, platform-dependent - see [Platform notes](https://dmartsapp.github.io/shint/docs/install.html#platform-notes)). `ping` already got this in the previous release via the [go-ping](https://github.com/dmartsapp/go-ping) v2.0.0 upgrade (which also fixed a data race, a payload-size clamp bug, a sequence-matching bug, and a JSON-encoding bug on the ICMP side); every other command catches up here.
- Fixed `listen tcp`/`listen udp`/`listen http` exiting after exactly one connection/packet/request by default: `listen`'s own `--count` now defaults to `0` (unlimited, until Ctrl+C) instead of inheriting the root `--count`'s default of `1`.
- Fixed `listen http` occasionally dropping its own response (`curl: (52) Empty reply from server`) right as it hit its request budget - a real race between the process exiting and net/http's internal per-connection goroutine still flushing that same response. Hardened with an explicit flush, a deterministic `Connection: close`, and a proper `Server.Shutdown` wait before returning.
- CI split from one combined workflow into five independent ones (lint, vulnerability check, binary build & release, Docker Hub, GHCR) so each has its own live status instead of one pass/fail covering everything - see the table at the top.

## v3.1.0 - 2026-09-19

- Added `listen http`: a minimal JSON status endpoint (any method on `/` returns `{"status":"ok"}`, every other path 404s with `{"status":"not found"}`), useful for both plain TCP and HTTP-level reachability checks against the same process.
- Docker images now publish to Docker Hub (`farhansabbir/shint`) in addition to GHCR.

## v3.0.0 - 2026-09-19

- Added `udp` (UDP probe with `open`/`closed`/`open|filtered` classification).
- Added `listen tcp` / `listen udp` local listeners for testing without a real remote server.
- Added `--cacert`/`--cert`/`--key`/`--insecure` to `web` for authenticated TLS checks (custom CA trust and mutual TLS) against HTTPS services.
- Fixed data races on the JSON stats slices in `telnet` and `web` (concurrent goroutines appending to a shared slice without synchronization).
- Fixed a nil-pointer risk in `web` when given an unparseable URL.
- Fixed `web --json --withbody` serializing the already-consumed request body reader instead of the payload actually sent.
- Added input validation (port ranges, positive `--count`/`--timeout`, `--from <= --to` on `nmap`) instead of panicking or hanging on bad input.
- Bounded `nmap`'s concurrency so a wide port range can't exhaust file descriptors, and fixed it ignoring context cancellation during the scan loop so a slow/wide scan now actually stops at the caller's `--timeout` instead of running every dial out individually.
- Unified text-mode logging across every command into one greppable format (see [Logging format](https://dmartsapp.github.io/shint/docs/output.html)).
- Upgraded to Go 1.26.3 and the latest available cobra/x-net/x-sys releases; removed stray dead config (`go.env`, `.gitmodules`) left over from an earlier private-submodule setup.
- Expanded the release build matrix from 6 to 14 OS/architecture targets (added FreeBSD, OpenBSD, NetBSD, Solaris, Android).
- Added a multi-arch Docker image published to GHCR on every tagged release (see [Docker image](https://dmartsapp.github.io/shint/docs/install.html#docker)).
- Added an exhaustive, hermetic test suite (60 tests) covering both packages, including a full mTLS round trip against a locally-generated CA.
