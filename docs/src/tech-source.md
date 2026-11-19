---
title: Source reference
lead: Every file in the repository - what it is for, what it contains, and how it connects to the rest.
description: File-by-file reference for the shint repository - Go sources, tests, build files, workflows and documentation.
section: Technical
order: 2
nav: Source reference
---

## Root

| File | Purpose |
|---|---|
| `main.go` | The command line. Defines the cobra commands (`telnet`, `ping`, `web`, `nmap`, `udp`, `ntp`, `wol`, `rdns`, `dns`, `cidr`, `ip`, `listen tcp|udp|http`), every flag, argument validation, the exit-status scheme, and `scanContext`. Holds the `Version` constant; `releaseVersion` reads the release out of whatever was stamped into it, for `userAgent`. |
| `main_ss.go` | The `ss` command and its flags, built for every platform but AIX (`//go:build !aix`). Its `init` adds it to `platformCommands`. See [No `ss` on AIX](tech-design.md#no-ss-on-aix). |
| `main_test.go` | End-to-end tests of the CLI. `TestMain` makes the test binary behave as `shint` when `SHINT_TEST_RUN_MAIN=1`, so tests run it as a subprocess and assert real exit statuses and stdout/stderr. |
| `go.mod`, `go.sum` | The module (`github.com/dmartsapp/shint/v4`), the Go version, and pinned dependencies. The `go` line ({{go}}) is the one place the Go release is chosen: see [Moving to a new Go release](tech-ci.md#moving-to-a-new-go-release). |
| `toolchain_test.go` | Fails when the `Dockerfile`'s builder image is not the Go `go.mod` names, so the container images and the release binaries are built with the same Go. |
| `Makefile` | The one place to build and test from: cross-compilation targets for every release platform, the version-string logic for local builds, and the checks (`make check`, `make test-live`, `make test-full`). |
| `Dockerfile` | Two-stage image build: Go build stage, distroless final stage. |
| `.dockerignore`, `.gitignore` | What stays out of the Docker build context and out of git (`bin/`, `*.json`, `.DS_Store`). |
| `test/live/basic_module_test.sh` | Live-internet smoke test: builds the binary and runs one check per command against real hosts. Run it with `make test-live`. |
| `test/platforms/` | Platform tests: `template.sh` and `template.ps1`, the script of each release binary that `platforms.py` makes from them (`make platform-tests`; never edited by hand), and `results/`, the latest results from each platform (`make platform-summary`). `test_platforms.py` tests `platforms.py`; `make check` checks the scripts are current and the results add up. See [Platform tests](tech-testing.md#platform-tests). |
| `CHANGELOG.md` | The release history; also rendered as the [Changelog](changelog.md) page. |
| `readme.md` | The short introduction that GitHub shows on the front page. On `main` it is the project README (all milestones and releases); a release branch's copy is `main`'s (release branches up to v4.2.2 kept a working page of their own here) - see [Releases and tagging](tech-release.md#branches-and-cadence). |
| `LICENSE` | MIT. |
| `index.html`, `.nojekyll` | Make GitHub Pages serve the site under `docs/` as plain static files (see [This documentation](tech-docs.md)). |

### main.go in detail

| Symbol | What it is |
|---|---|
| `Version` | The version string. `"{{version}}"` for a source build; overridden with `-ldflags "-X main.Version=..."` by the Makefile, the release workflow and the Dockerfile (see [Releases and tagging](tech-release.md#how-the-version-string-gets-into-the-binary)). |
| `versionLine()` | What `--version` and `-v` print: `Version`, then the Go toolchain and the platform the binary was built for, in brackets, as `go version` writes them (`runtime.Version()`, `GOOS/GOARCH`). |
| flag variables (`iterations`, `timeout`, ...) | Package-level variables bound to flags in `init()`. Shared flags live on the root command as *persistent* flags. |
| `exitOK`, `exitFailure`, `exitUsage`, `exitCode` | The exit-status scheme (0 / 1 / 2) and the value `main` exits with. |
| `usage(msg)` | Prints a usage error to stderr and records status 2. |
| `finish(ok)` | Records status 1 when a handler reports failure. |
| `checkRunFlags(usesDelay)`, `checkListenFlags()` | The shared flag ranges, checked once for every command: `--count` at least 1, `--timeout` 1 to a day, `--delay` 0 to a day (`lib.RequireTimeout`, `RequireDelay`, ...); for `listen`, `--count` at least 0 and `--timeout` 0 to a day. |
| `unknownSubcommand` | The Run of `listen` and `completion`, which only group other commands: an argument they do not know is a usage error (exit 2), not the help page with exit 0. |
| `rootCmd`, `telnetCmd`, `pingCmd`, `webCmd`, `nmapCmd`, `udpCmd`, `ntpCmd`, `wolCmd`, `rdnsCmd`, `dnsCmd`, `cidrCmd`, `ipCmd`, `listenCmd` (+ `listenTCPCmd`, `listenUDPCmd`, `listenHTTPCmd`) | The commands. Each `Run` validates, calls one handler, and passes its result to `finish`. |
| `interruptContext()` | The context `nmap`, `telnet`, `web`, `udp`, `ntp`, `rdns` and `wol` run under: cancelled by `Ctrl+C`/`SIGTERM`, deliberately without any deadline, and un-registered after the first signal so a second `Ctrl+C` ends the process. |
| `init()` | Registers all flags. Note `listenCmd` re-declares `--count` (default `0`) and `webCmd` re-declares `--payload` (`-P`, the body), shadowing the root flags. |
| `main()` | Adds the commands - and `platformCommands`, the ones only some platforms build - runs cobra, exits with `exitUsage` if cobra returned an error, otherwise with `exitCode`. |

## lib - shared code

| File | Contents |
|---|---|
| `lib/lib.go` | Constants `DATETIMEFORMAT` (`time.UnixDate`), `NetworkType` (`"ip"`: A and AAAA) and `Protocol` (`"tcp"`). `ResolveName` and `ResolveNameToIPs` (DNS), `IsPortUp` (one TCP dial bounded by a per-attempt timeout *and* a context), `ValidatePort`, `RequirePositive`, `GetMinAvgMax` (latency statistics), `ConvertIPToStringSlice`, and `SortTimeDurationSlice` (currently exercised only by tests). |
| `lib/descriptors_unix.go`, `descriptors_other.go` | `descriptorLimit`: the process's soft `RLIMIT_NOFILE` on Unix (Go raises it to the hard limit at start), unknown elsewhere. `lib.InFlightLimit` turns it into a bound on simultaneous attempts: half the limit, at most the cap, at least 4. |
| `lib/family.go` | The address family a run uses: `NetworkType` (`"ip"`, `"ip4"`, `"ip6"`), `SetIPFamily` (the `-4`/`-6` flags), `FamilyAllows`, `DialNetwork` (`tcp` to `tcp4`/`tcp6` for the HTTP client), `HostFamilyConflict` (an address of the other family), and `FamilyHint`/`ExplainError` (the hint on an IPv6 failure that is really this machine's lack of IPv6). |
| `lib/output.go` | The JSON contract: `JSONOutput`, `InputParams`, `DNSLookup`, and the per-command stats types `TelnetStats`, `WebStats`, `NmapStats`, `ICMPStats`, `UDPStats`, `NTPStats`, `WOLStats`, `RDNSStats`, `CIDRStats`, `IPStats` and `IPAddress`, `SSStats` and `SSProcess`, `DNSStats` and `DNSRecord`, the `--timing` types `WebTiming` and `WebHopTiming`, `LocalJSONOutput` (the document of `wol`, `cidr`, `ip` and `ss`, which have no `dns_lookup`), `NmapJSONOutput` (`nmap`'s, whose `dns_lookup` is there only for one host name or address, with `dns_lookups` for several targets), plus the listener events `ListenEvent` and `HTTPListenEvent`. Also the text log: `LogWithTimestamp` (the `time: [module] OK|ERROR message` prefix), `Fields` (the `key=value` suffix, quoting values with spaces) and `LogStats` (the statistics banner). |
| `lib/source.go` | `--interface` (#69): `SetSourceInterface` resolves an interface name (any case) or an address of this machine to the addresses to send from - or says what is wrong, with `NoSuchInterface` listing the names that exist; `BindSource` sets a `net.Dialer`'s source address for each dial, of the destination's family (link-local for link-local). Every dial of `telnet`, `nmap` (`IsPortUp`), `web`, `tls`, `udp`, `ntp`, `dns`, `rdns` and `wol` goes through it. |
| `lib/ports.go` | `ParsePorts` (`nmap --ports` and `ss --ports`: numbers and ranges, sorted, each once), `PortRange` (`--from`/`--to`) and `PortsText` (a sorted list written back with runs as ranges). |
| `lib/family_test.go` | The family logic and the hint, built from the exact error a host with IPv6 disabled reported. |
| `lib/ports_test.go` | What `ParsePorts` accepts and refuses (empty items, signs, words, out-of-range and backwards ranges), and `PortsText` round trips. |
| `lib/lib_test.go`, `lib/output_test.go` | Unit tests for the above: validators, statistics maths, DNS (loopback, IPv6 literal, dual-stack, invalid host), `IsPortUp` (open, closed, IPv6, context cancellation), log formatting. |

## lib/handlers - one package per command

Each command is a package of its own under `lib/handlers/`, holding its code, its per-platform files (`_linux.go`, `_windows.go`, ... where the systems differ) and its tests. `main.go` calls each package's entry point - `telnet.Run`, `ip.Routes`, `listen.HTTP` - and nothing else. A package depends on `lib`, on the shared packages in `lib/handlers/internal/` below, and - for the authoritative-server lookup every resolving command prints - on `dns`; no command imports another otherwise.

| Package | Files | What it holds, and what its tests cover |
|---|---|---|
| `telnet` | `telnet.go` | `Run`: resolve, then one connection attempt per iteration per address; true only if all succeed. Tests: success and failure output, IPv6 loopback, DNS failure, no data race, a run longer than `--timeout`, and `Ctrl+C` (`telnet_cancel_test.go`). |
| `ping` | `icmp.go` | `Run` wraps go-ping's `Pinger` (parallel pings, streamed lines) and converts its statistics; false on any lost ping. `restrictFamily` applies `-4`/`-6`; `ValidatePayload`. Tests: text and JSON, the payload size on replies, `--timeout` reaching the pinger, which lines count as lost. |
| `web` | `web.go`, `webredact.go`, `webredirect.go`, `webtiming.go` | `Run`: builds the request, sends it through the byte-counting transport (`internal/wire`), reports status, timing, speed and bytes; failed attempts in JSON `stats`. `UserAgent` (set by `main` to `shint/<release> (+URL)`), `ParseURL`; credential redaction, the redirect policy (credentials never leave the origin), and the `--timing` recorder (`timingRecorder`, a hop per connection request). Tests: round trips and mutual TLS, redaction, redirects, timing against a fake clock and real servers, the body cap, the TLS notes in JSON, the byte-measurement guarantees against `listen http` and raw servers (`wirebytes_test.go`), and `Ctrl+C`. |
| `tlscheck` | `tlscheck.go` | `Run` and `Options`: a handshake per address with the TLS versions shint offers (`tlsMinVersion`..`tlsMaxVersion`, shown as `offered=`), the chain verified against the name (`verifyChain`), expiry judged on the verified chain, errors in plain words (`explainTLSError`). Tests: trusted, untrusted, expired and wrong-name chains, `--warn-days`/`--fail-days`, servers limited to TLS 1.2 or to TLS 1.0-1.1. |
| `nmap` | `nmap.go`, `nmaptargets.go`, `nmapservices.go` | `Run`, given a `Scan` (targets and ports): looks up each host name, sweeps each prefix, scans each address once, writes an `NmapJSONOutput`; a sweep of public addresses gets `publicWarning` and probes spaced by `Scan.Delay` (#106). `ParseTargets` checks every target first (a host name's shape, families, a prefix's size, 65,536 addresses at most); `isPublicSweep`, `sweepAddresses`; `tcpServices` behind `service=`. Tests: open ports on IPv4 and IPv6, the concurrency cap and the descriptor limit, whole-range coverage, interruption, progress lines, port lists, several targets, sweeps and their pace. |
| `udp` | `udp.go` | `probeUDP` (open / closed / open\|filtered / error) and `Run`; `ValidatePayload`, `ParseHexPayload`. Tests: each state, generated payloads, multiple attempts, `Ctrl+C`. |
| `ntp` | `ntp.go` | `Run` and `queryNTP`: one SNTP v4 query per address; NTP's 64-bit timestamps with the 2036 wrap; the 48-byte request and the replies it refuses. Tests: a controllable fake SNTP server (offsets, processing time, malformed replies, timeouts, IPv6), `Ctrl+C`. |
| `wol` | `wol.go` | `Run`, `MagicPacket` (6 x 0xFF and the MAC 16 times), `ParseMAC`, `ParseBroadcast`. Tests: the packet received by a real UDP socket, `Ctrl+C`. |
| `rdns` | `rdns.go` | `Run`; `lookupAddr`, the resolver call, a variable so tests substitute it (Go's resolver, bound to `--interface` when given). Tests: a substituted resolver, `Ctrl+C`. |
| `dns` | `dns.go`, `authoritative.go` | `Run`, `ParseArgs` and `Options`: the DNS client (`golang.org/x/net/dns/dnsmessage`; UDP with EDNS, TCP on truncation or `--tcp`; the hosts file first); `ReverseName` (the `in-addr.arpa.`/`ip6.arpa.` name). `FindAuthoritative`, `PrintAuthoritative` and `AuthoritativeTimeout`: the zone and name servers every resolving command shows, asked in parallel through the same client. Tests: a fake DNS server on loopback (every record type, negative answers, truncation and retry, EDNS fallback, wrong IDs), the authoritative lookup's cases. |
| `cidr` | `cidr.go` | `Run` and `ParseSubnets`: subnet arithmetic with `net/netip` and `math/big`. Tests: values worked out by hand and checked against `net.ParseCIDR`. |
| `ip` | `ip.go`; `iproute.go`, `iproute_proc.go`, `iproute_win.go`, `iproute_{linux,bsd,bsd_darwin,bsd_other,windows,other}.go`; `iparp.go`, `iparp_netlink.go`, `iparp_win.go`, `iparp_{linux,bsd,windows,other}.go` | `Interfaces` (the interface table behind `systemInterfaces`), `InterfaceNames` and `noSuchInterface`. `Routes`: the routing table from `readRoutes`, or the way to an address (`kernelRoute`, a UDP connect; `matchRoute` on the system's interface only). `Neighbours`: the neighbour table from `readNeighbours`, the default gateways marked. The parsers (`/proc`, netlink messages, Windows rows) have no build tag and are tested everywhere; `platformRoutes`/`platformNeighbours` per system. Tests: fake interface tables, `/proc` in both byte orders, macOS's flag combinations, lookups, netlink and Windows rows built byte by byte, unknown interfaces. |
| `ss` | `ss.go`, `ssproc.go`, `sswin.go`, `ss_{linux,windows,other}.go` | `Run` and `Query`: the sockets from `readSockets`, the filters, the sort order, the text line, the `owners hidden` warning. Linux: `/proc/net/{tcp,tcp6,udp,udp6}` and `/proc/<pid>/fd`; Windows: `GetExtendedTcpTable`/`GetExtendedUdpTable` rows and a process snapshot. Tests: text and JSON, every filter, a fake `/proc`, Windows tables built byte by byte. |
| `listen` | `tcplisten.go`, `udplisten.go`, `httplisten.go`, `listennotice.go` | `TCP`, `UDP` and `HTTP`: the listeners; `httpExchange` records the request each connection served, reported on close so byte counts are final; `exposureNotice` (the `reachable from other machines` warning). Tests: accept and echo, IPv6, JSON events and their measurements, runs until interrupted. |

### Shared packages

| Package | Files | What it holds |
|---|---|---|
| `lib/handlers/internal/pace` | `pacing.go`, `interrupt.go` | How a repeating check runs and ends: `Slots` (at most 500 attempts in flight, fewer under a low descriptor limit), `Delay` (`--delay`/`--throttle`), `Pause` (a delay `Ctrl+C` cuts short), `WatchCancel` (a cancellation unblocks a read), `InterruptedLine`/`InterruptedNote`. Tested. |
| `lib/handlers/internal/term` | `escape.go` | Text that is safe to print and short to read: `Escape` (untrusted bytes - a packet, a DNS record, a process name - escaped), `Preview` (trimmed and cut, for listeners and `udp`), and the plain-words error helpers `Reason`, `ShortError`, `UnwrapPath`. Tested. |
| `lib/handlers/internal/wire` | `wireconn.go` | Byte counting on the wire, shared by `web` and `listen http`: `Conn` (`BytesRead`, `BytesWritten`, `FirstWritten`, `SetOnClose`), `Listener`, `NewTransport` (an HTTP/1.1 transport whose plain and TLS connections are counted) and `Meter` (a request's share of a connection). |
| `lib/tlsconf` | `tls.go` | `Build`: `--cacert`, `--cert`/`--key` and `--insecure` as a `tls.Config` (TLS 1.2 at least); `LoadCAPool`. Used by `web` and `tls`. Tests: every branch. |
| `lib/internal/testutil` | `testutil.go`, `interrupt_{unix,windows}.go` | What the tests share: `CaptureStdout` (stdout through a pipe drained concurrently), `FreeTCPPort`, `FreeUDPPort`, a throw-away certificate authority (`NewCA`, `IssueCert`, `WritePEM`), echo listeners, `WaitForListener`, `UseFamily`, `MustContain`, `CancelAfter`/`InterruptedCounts`/`RunWithin` for the `Ctrl+C` tests, and `SendInterrupt`/`RequireInterrupt` (SIGINT to the test process; skipped on Windows). Used only by `_test.go` files. |

## Build and packaging

| File | Purpose |
|---|---|
| `Makefile` | One target per platform (`linux-amd64`, `darwin-arm64`, ...), the groups `linux`/`darwin`/`windows`/..., `all` (the desktop triad), `all-platforms` (every release target), `run`, `clean`. Builds with `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.Version=..."`. It also holds every check: `check`, `test`, `test-live`, `test-full`, `fmt-check`, `vet`, `lint`, `vuln`, `docs-check`, `workflows`, `tools` and `help` (see [Testing](tech-testing.md#running-the-tests)). |
| `Dockerfile` | Stage 1 `golang:{{go}}-alpine` (the Go `go.mod` names - `toolchain_test.go` keeps the two the same) compiles the binary for `TARGETOS`/`TARGETARCH`; stage 2 `gcr.io/distroless/static-debian12:nonroot` carries only the binary and CA certificates. `ENTRYPOINT ["/shint"]`, `CMD ["--help"]`. |
| `.dockerignore` | Keeps `.git`, `.github`, `bin`, markdown files and scripts out of the build context. |

## Continuous integration

| File | Purpose |
|---|---|
| `.github/scripts/verify-release-tag.sh` | The release-tag guard (`release.yaml`'s first job): the tag must be `vX.Y.Z` (format), and its commit must be on `main` (the GitHub compare API). |
| `.github/scripts/test-verify-release-tag.sh` | Offline tests for the guard, with a fake `gh`: `bash .github/scripts/test-verify-release-tag.sh`. |
| `.github/scripts/check-workflow-triggers.py` | Fails if any workflow can be started by anything other than a `vX.Y.Z` tag (or a push to `main` that ignores `.github/**`). See [the trigger rule](tech-ci.md#the-workflow-trigger-rule). |
| `.github/scripts/test_check_workflow_triggers.py` | Tests for that check, including a run over the real workflow files. |
| `.github/workflows/release.yaml` | Everything a `vX.Y.Z` tag starts, as one workflow's jobs: the tag guard (`verify-tag`), the checks once (`lint`, `vulncheck`, `hadolint`), the 18 binaries and the GitHub release, the Docker Hub and GHCR images and their reports on the release, the README proposal (`readme`), and last the failure issue and the Slack/Discussions announcement - each job handing on what the next needs. See [The release workflow](tech-ci.md#the-release-workflow). |
| `.github/scripts/notify-slack.py` | The outcome of a release, from every job's result (`toJSON(needs)`) and the run's jobs: the Slack message and, for a published release, the announcement (Slack, Discussions) - or, with `ISSUE_BODY`, the failure issue's body. Standard library only; tested by `test_notify_slack.py`, which also checks its list of parts against `release.yaml`. |
| `.github/workflows/check.yaml` | After a merge to `main` (never a branch, PR or tag; ignores `.github/**`): `make check-quick` when a valid signed local check report is on the commit, the whole `make check` when it is not; opens an issue on failure. |
| `.github/scripts/changelog-section.sh` | Prints one release's section of `CHANGELOG.md`: the release page's "What's new" (with `test-changelog-section.sh`). |
| `.github/scripts/attest.sh` | `make attest`: runs `make check`, builds and signs the report, attaches it to the commit as a git note (`refs/notes/checks`) and pushes the note. |
| `.github/scripts/check-report.py` | Builds the report from `make check`'s output, signs it with `ssh-keygen -Y sign`, and verifies a note (signature, tree, every stage, age). Standard library only. |
| `.github/scripts/test_check_report.py` | Its tests, with throw-away SSH keys: a valid report, and each way one is refused. |
| `.github/scripts/verify-local-check.sh` | The Check workflow's side: fetches the note and the allowed signers (from the commit before the push) and asks `check-report.py`. Writes `fast=true/false`; never fails. |
| `.github/scripts/test-attest.sh` | Tests for `attest.sh`, the verifier and the hook, in throw-away repositories. |
| `.githooks/pre-push` | Installed by `make hooks`: pushing `main` or a release tag with no valid signed report runs `make attest` first. |
| `.github/allowed_signers` | The public keys whose signed reports CI accepts (namespace `shint-check`). |
| `.github/scripts/readme-reconcile.py` | Brings a README in line with the release tags, the changelog, the milestones and the binary's `--help`, puts back what it must always have (expansion, badges with the donate button, support line), warns about what it cannot know; idempotent, fails closed. |
| `.github/scripts/test_readme_reconcile.py` | Its tests, on README fixtures (35 of them). |
| `.github/scripts/readme-release.sh` | `make readme-reconcile`: branches `readme/main-<tag>` off `main`, runs the script, commits, and with `PUSH=1` pushes and opens the pull request. |
| `.github/scripts/test-readme-release.sh` | Tests for it, in throw-away git repositories with a fake `gh`. |
| `.github/scripts/release-check.sh` | `make release-check`, run by `make release` after the release commit: on top of `main`, `readme.md` changing only `main`'s Roadmap rows (with this release's row Released), `releases/vX.Y.Z.md` present, the version, dated changelog and release-commit message agreeing, no attribution trailers, the tag free. |
| `.github/scripts/test-release-check.sh` | Tests for it, in throw-away git repositories. |
| `.github/scripts/release.py` | `make release-start` and `make release`: starts a release branch with its changelog heading and working page, and on release day stamps, checks, commits and - after one confirmation - pushes `main`, waits for Check, tags and verifies the release; resumable ([Making a release](tech-release.md#making-a-release)). |
| `.github/scripts/test_release.py` | Tests for it, end to end: a bare origin, a clone, the real reconcile and release-check scripts, and a fake `gh`. |
| `branch_readme.md` | On a release branch only: its working page - targets, bugs, changes, the **To do** list, and the `Summary:` that becomes the release's roadmap row. `make release` refuses to publish while a to-do is unchecked and moves it to `releases/vX.Y.Z.md`. |
| `releases/vX.Y.Z.md` | A release's record on `main`: its working page, as it was when it shipped. See [`releases/`](https://github.com/dmartsapp/shint/tree/main/releases). |
| `.github/scripts/write-checksums.sh` | Writes `<binary>.sha256` next to each release binary and verifies it. |
| `.github/scripts/test-write-checksums.sh` | Offline tests for it: format, an independent hash, the platform's own verifier, tampering. |
| `.github/scripts/write-ci-failure-issue.sh` | Writes the body of the issue a failed Check files, with the real run URL (a failed release's issue comes from `notify-slack.py`). |
| `.github/scripts/test-write-ci-failure-issue.sh` | Offline tests for it. |

Each workflow is explained in [CI/CD workflows](tech-ci.md).

## Documentation

| File | Purpose |
|---|---|
| `docs/src/*.md` | The page sources (Markdown with a small front-matter header). |
| `docs/build.py` | The site generator: renders the sources and `CHANGELOG.md` into HTML, checks internal links and the published-site URLs in the README, changelog and sources, and can verify the committed pages are current. |
| `docs/test_build.py` | Tests for the generator (`python3 docs/test_build.py`): the published-site URL check and a few renderer rules. |
| `docs/*.html` | The generated pages that GitHub Pages serves. **Do not edit these by hand.** |
| `docs/assets/style.css`, `site.js`, `favicon.svg` | The site's design, its small progressive-enhancement script, and its icon. |

See [This documentation](tech-docs.md) for how to change and rebuild the site.
