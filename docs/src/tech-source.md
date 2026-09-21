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
| `main.go` | The command line. Defines the cobra commands (`telnet`, `ping`, `web`, `nmap`, `udp`, `ntp`, `wol`, `rdns`, `dns`, `cidr`, `ip`, `listen tcp|udp|http`), every flag, argument validation, the exit-status scheme, and `scanContext`. Holds the `Version` constant. |
| `main_test.go` | End-to-end tests of the CLI. `TestMain` makes the test binary behave as `shint` when `SHINT_TEST_RUN_MAIN=1`, so tests run it as a subprocess and assert real exit statuses and stdout/stderr. |
| `go.mod`, `go.sum` | The module (`github.com/dmartsapp/shint/v4`), the Go version, and pinned dependencies. |
| `Makefile` | The one place to build and test from: cross-compilation targets for every release platform, the version-string logic for local builds, and the checks (`make check`, `make test-live`, `make test-full`). |
| `Dockerfile` | Two-stage image build: Go build stage, distroless final stage. |
| `.dockerignore`, `.gitignore` | What stays out of the Docker build context and out of git (`bin/`, `*.json`, `.DS_Store`). |
| `basic_module_test.sh` | Live-internet smoke test: builds the binary and runs one check per command against real hosts. Run it with `make test-live`. |
| `CHANGELOG.md` | The release history; also rendered as the [Changelog](changelog.md) page. |
| `readme.md` | The short introduction that GitHub shows on the front page. On `main` it is the project README (all milestones and releases); on a release branch it is a working page for that branch alone and is never merged into `main` (see [Releases and tagging](tech-release.md#branches-and-cadence)). |
| `LICENSE` | MIT. |
| `index.html`, `.nojekyll` | Make GitHub Pages serve the site under `docs/` as plain static files (see [This documentation](tech-docs.md)). |

### main.go in detail

| Symbol | What it is |
|---|---|
| `Version` | The version string. `"4.2.0"` for a source build; overridden with `-ldflags "-X main.Version=..."` by the Makefile, the release workflow and the Dockerfile (see [Releases and tagging](tech-release.md#how-the-version-string-gets-into-the-binary)). |
| flag variables (`iterations`, `timeout`, ...) | Package-level variables bound to flags in `init()`. Shared flags live on the root command as *persistent* flags. |
| `exitOK`, `exitFailure`, `exitUsage`, `exitCode` | The exit-status scheme (0 / 1 / 2) and the value `main` exits with. |
| `usage(msg)` | Prints a usage error to stderr and records status 2. |
| `finish(ok)` | Records status 1 when a handler reports failure. |
| `checkRunFlags(usesDelay)`, `checkListenFlags()` | The shared flag ranges, checked once for every command: `--count` at least 1, `--timeout` 1 to a day, `--delay` 0 to a day (`lib.RequireTimeout`, `RequireDelay`, ...); for `listen`, `--count` at least 0 and `--timeout` 0 to a day. |
| `unknownSubcommand` | The Run of `listen` and `completion`, which only group other commands: an argument they do not know is a usage error (exit 2), not the help page with exit 0. |
| `rootCmd`, `telnetCmd`, `pingCmd`, `webCmd`, `nmapCmd`, `udpCmd`, `ntpCmd`, `wolCmd`, `rdnsCmd`, `dnsCmd`, `cidrCmd`, `ipCmd`, `listenCmd` (+ `listenTCPCmd`, `listenUDPCmd`, `listenHTTPCmd`) | The commands. Each `Run` validates, calls one handler, and passes its result to `finish`. |
| `interruptContext()` | The context `nmap`, `telnet`, `web`, `udp`, `ntp`, `rdns` and `wol` run under: cancelled by `Ctrl+C`/`SIGTERM`, deliberately without any deadline, and un-registered after the first signal so a second `Ctrl+C` ends the process. |
| `init()` | Registers all flags. Note `listenCmd` re-declares `--count` (default `0`) and `webCmd` re-declares `--payload` (`-P`, the body), shadowing the root flags. |
| `main()` | Adds the commands, runs cobra, exits with `exitUsage` if cobra returned an error, otherwise with `exitCode`. |

## lib - shared code

| File | Contents |
|---|---|
| `lib/lib.go` | Constants `DATETIMEFORMAT` (`time.UnixDate`), `NetworkType` (`"ip"`: A and AAAA) and `Protocol` (`"tcp"`). `ResolveName` and `ResolveNameToIPs` (DNS), `IsPortUp` (one TCP dial bounded by a per-attempt timeout *and* a context), `ValidatePort`, `RequirePositive`, `GetMinAvgMax` (latency statistics), `ConvertIPToStringSlice`, and `SortTimeDurationSlice` (currently exercised only by tests). |
| `lib/descriptors_unix.go`, `descriptors_other.go` | `descriptorLimit`: the process's soft `RLIMIT_NOFILE` on Unix (Go raises it to the hard limit at start), unknown elsewhere. `lib.InFlightLimit` turns it into a bound on simultaneous attempts: half the limit, at most the cap, at least 4. |
| `lib/family.go` | The address family a run uses: `NetworkType` (`"ip"`, `"ip4"`, `"ip6"`), `SetIPFamily` (the `-4`/`-6` flags), `FamilyAllows`, `DialNetwork` (`tcp` to `tcp4`/`tcp6` for the HTTP client), `HostFamilyConflict` (an address of the other family), and `FamilyHint`/`ExplainError` (the hint on an IPv6 failure that is really this machine's lack of IPv6). |
| `lib/output.go` | The JSON contract: `JSONOutput`, `InputParams`, `DNSLookup`, and the per-command stats types `TelnetStats`, `WebStats`, `NmapStats`, `ICMPStats`, `UDPStats`, `NTPStats`, `WOLStats`, `RDNSStats`, `CIDRStats`, `IPStats` and `IPAddress`, `DNSStats` and `DNSRecord`, the `--timing` types `WebTiming` and `WebHopTiming`, `LocalJSONOutput` (the document of `wol` and `cidr`, which have no `dns_lookup`), plus the listener events `ListenEvent` and `HTTPListenEvent`. Also the text log: `LogWithTimestamp` (the `time: [module] OK|ERROR message` prefix), `Fields` (the `key=value` suffix, quoting values with spaces) and `LogStats` (the statistics banner). |
| `lib/family_test.go` | The family logic and the hint, built from the exact error a host with IPv6 disabled reported. |
| `lib/lib_test.go`, `lib/output_test.go` | Unit tests for the above: validators, statistics maths, DNS (loopback, IPv6 literal, dual-stack, invalid host), `IsPortUp` (open, closed, IPv6, context cancellation), log formatting. |

## lib/handlers - one file per command

| File | Contents |
|---|---|
| `telnet.go` | `TelnetHandler`: resolve, then one connection attempt per iteration per address; reports `true` only if all succeed. |
| `icmp.go` | `HandleICMP`: wraps go-ping's `Pinger` (parallel pings, streamed log lines), converts its statistics to shint's output; reports `false` on any lost ping. `restrictFamily` applies `-4`/`-6` to the addresses go-ping resolved (its own `SetNetwork` cannot: the constructor has already resolved both families). |
| `web.go` | `WebHandler`: builds the request (method, body, headers), sends it through a counting transport, and reports status, timing, speed and bytes. Records failed attempts in JSON `stats`. `HTTP_CLIENT_USER_AGENT` is the default `User-Agent`. |
| `webtiming.go` | The `--timing` recorder: `timingRecorder` collects an `httptrace` per request (a hop per connection request, so a redirect adds one), `timingHop` holds the moments, and `hopTimings`, `timingLines` and `timingOrNil` turn them into the JSON and text forms. `checkRedirect` is net/http's default redirect policy plus telling the recorder about each hop. |
| `wireconn.go` | Wire-level byte counting shared by `web` and `listen http`: `countingConn` (a `net.Conn` wrapper), `countingListener`, `newCountingTransport` (HTTP/1.1 transport whose plain and TLS connections are wrapped) and `wireMeter` (per-request accounting across connections and redirect hops). |
| `tls.go` | `BuildTLSConfig`: turns `--cacert`, `--cert`/`--key` and `--insecure` into a `tls.Config` (minimum TLS 1.2). |
| `nmap.go` | `NmapHandler`: the port scanner. `maxConcurrentPortScans` (500), `progressInterval` (3 s) and `probePort` (the single-port check, a variable so tests can substitute it). |
| `udp.go` | `probeUDP` (one probe, classified as open / closed / open\|filtered / error) and `UDPHandler`. |
| `ntp.go` | `NTPHandler` and `queryNTP`: one SNTP v4 query per resolved address. `toNTP`/`fromNTP` convert NTP's 64-bit timestamps (with the 2036 wrap), `buildNTPRequest` and `parseNTPReply` build the 48-byte request and refuse a reply that is not an answer (wrong mode, mismatched originate timestamp, kiss-o'-death, unsynchronized, stratum 16, no transmit time). |
| `wol.go` | `WOLHandler`, `MagicPacket` (6 x 0xFF + the MAC 16 times), `ParseMAC` (four spellings, exactly six bytes) and `ParseBroadcast` (IPv4 only). |
| `rdns.go` | `RDNSHandler` and `ReverseName` (the `in-addr.arpa.` / `ip6.arpa.` name). `lookupAddr` is the resolver call, a variable so tests can substitute it. |
| `authoritative.go` | `findAuthoritative` (the `dns authoritative` line and `dns_lookup.authoritative`): the name and each parent are asked for their NS records in parallel through the `dns` engine, a record counts only if the candidate itself owns it (a resolver answers an NS question about an alias with the target's records), and the answer is decided as soon as every closer candidate has answered. `printAuthoritative` writes the line; `queryNS` is the test hook. |
| `dns.go` | `DNSHandler` and `ParseDNSArgs`: the DNS client. It builds the question with `golang.org/x/net/dns/dnsmessage` (the module's one direct dependency besides cobra and go-ping), asks over UDP with an EDNS option and over TCP when truncated, accepts only a reply with the right ID, question and source, and writes each record the way `dig` does. `systemDNSServers` (a hook) finds the system's servers by giving Go's resolver a `Dial` that only records the address. |
| `escape.go` | `escapeBytes`: the one place untrusted bytes (a packet, a DNS record) are made safe to print; `previewBytes` adds trimming and truncation on top. |
| `ip.go` | `IPHandler`: the interface table (`net.Interfaces`, behind the `systemInterfaces` hook so tests can describe interfaces), the per-interface report and the `-4`/`-6` filter. Uses `addressKind` from `cidr.go`. |
| `cidr.go` | `ParseSubnets`, `subnetStats` and `CIDRHandler`: subnet arithmetic with `net/netip` and `math/big` (IPv6 counts overflow 64 bits). |
| `pacing.go` (`attemptSlots`) | `attemptDelay`: the `--delay` / `--throttle` pause before an attempt, shared by `ntp`, `wol` and `rdns`. |
| `interrupt.go` | How a repeating check ends on `Ctrl+C`: `pause` (a `--delay` that a cancellation cuts short), `watchCancel` (expires a connection's deadline so a blocked read returns), and `interruptedLine` / `interruptedNote` (the `ERROR interrupted ...` line and the JSON run-level error). |
| `tcplisten.go` | `TCPListenHandler` and `handleTCPConnection`; also `previewBytes`, the short single-line preview used by all listeners and `udp`: printable text as is, everything else escaped (`\xNN`, `\n`, ...). |
| `udplisten.go` | `UDPListenHandler`. |
| `httplisten.go` | `HTTPListenHandler`: the minimal HTTP server. `httpExchange` records the one request each connection served, reported when the connection closes so byte counts are final. |

### Tests in lib/handlers

| File | What it covers |
|---|---|
| `testhelpers_test.go` | Shared helpers: `captureStdout` (redirects stdout to a pipe drained concurrently), `freeTCPPort`, `freeUDPPort`, and a throw-away certificate authority (`generateTestCA`, `issueCert`, `writePEM`) for TLS tests. |
| `interrupt_unix_test.go`, `interrupt_windows_test.go` | `sendInterrupt` / `requireInterrupt`: how the "runs until interrupted" tests stop a listener (SIGINT to the test process; skipped on Windows, where a process cannot signal itself). |
| `telnet_test.go` | Success and failure output, IPv6 loopback, DNS failure, concurrency (no data race), and the regression test that a run longer than `--timeout` still succeeds. |
| `icmp_test.go` | Text and JSON mode, the payload size shown on replies, and the pieces that need no network: `--timeout` reaching the pinger, payload validation, and which log lines count as a lost request (the live tests skip themselves where unprivileged ICMP is unavailable). |
| `web_test.go` | A full request/response round trip and mutual-TLS scenarios (with the client certificate, without it, without trusting the CA, and with `--insecure`). |
| `wirebytes_test.go` | The byte-measurement guarantees: `web` and `listen http` agree for bodies from 0 B to 2 MB, `web`'s counts match a raw server over HTTP and HTTPS, keep-alive reuse, redirect hops, header-insensitivity of the listener. |
| `tls_test.go` | Every branch of `BuildTLSConfig`. |
| `nmap_test.go` | Finding open ports (IPv4 and IPv6), the concurrency cap, whole-range coverage, interruption reporting, progress lines, `--json` cleanliness. Uses the `probePort` hook. |
| `udp_test.go` | `probeUDP` for each state, generated payloads, multiple attempts. |
| `cancel_test.go` | `Ctrl+C` on `telnet`, `web`, `udp`, `ntp`, `rdns` and `wol`: how far the run got, the statistics for what completed, an attempt in flight dropped rather than counted as a failure, a long `--delay` cut short, a complete JSON document, and the `pause` / `watchCancel` helpers. |
| `tcplisten_test.go`, `udplisten_test.go`, `httplisten_test.go` | Accept and echo, IPv6 binding, JSON events and their measurements, and "runs until interrupted" with `--count 0`. |
| `authoritative_test.go` | The lookup against a fake DNS server: the closest zone, a delegated subzone, only the TLD, an alias that must not become its own zone, names with no zone, a slow parent that must not delay the answer, unreachable servers and Ctrl+C. |
| `dns_test.go` | A fake DNS server on loopback (UDP and TCP on one port): every record type's format, negative answers with their reasons, truncation and the TCP retry, `--tcp`, the EDNS option and its fallback, replies with a wrong ID or question, a forged reply from another source, timeouts, skipped servers, escaping of hostile TXT data, Ctrl+C, and argument parsing. |
| `ip_test.go` | Fake interface tables: every field, one interface by name, `-4`/`-6`, JSON shape (and the empty list, not `null`), an unknown name, unreadable addresses, and the real table once. |
| `cidr_test.go`, `wol_test.go`, `ntp_test.go`, `rdns_test.go` | Subnet values worked out by hand and checked against `net.ParseCIDR`; the magic packet received by a real UDP socket; `ntp` against a controllable fake SNTP server (offsets, server processing time, malformed replies, timeouts, IPv6); `rdns` with a substituted resolver. |
| `webtiming_test.go` | The `--timing` recorder against a fake clock (exact figures), and against real servers: slow first byte, HTTPS handshake, a redirect, a reused connection, a refused connection, the redirect limit. |
| `preview_test.go` | `previewBytes`: printable text unchanged, everything else escaped, cut on a character boundary, always one safe line. |

## Build and packaging

| File | Purpose |
|---|---|
| `Makefile` | One target per platform (`linux-amd64`, `darwin-arm64`, ...), the groups `linux`/`darwin`/`windows`/..., `all` (the desktop triad), `all-platforms` (every release target), `run`, `clean`. Builds with `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.Version=..."`. It also holds every check: `check`, `test`, `test-live`, `test-full`, `fmt-check`, `vet`, `lint`, `vuln`, `docs-check`, `workflows`, `tools` and `help` (see [Testing](tech-testing.md#running-the-tests)). |
| `Dockerfile` | Stage 1 `golang:1.27.1-alpine` compiles the binary for `TARGETOS`/`TARGETARCH`; stage 2 `gcr.io/distroless/static-debian12:nonroot` carries only the binary and CA certificates. `ENTRYPOINT ["/shint"]`, `CMD ["--help"]`. |
| `.dockerignore` | Keeps `.git`, `.github`, `bin`, markdown files and scripts out of the build context. |

## Continuous integration

| File | Purpose |
|---|---|
| `.github/workflows/verify-release-tag.yaml` | The tag guard, a reusable workflow every other workflow calls first: the tag must be `vX.Y.Z` and its commit must be on `main`. No trigger of its own. |
| `.github/scripts/verify-release-tag.sh` | The guard's checks (tag format, then the GitHub compare API). |
| `.github/scripts/test-verify-release-tag.sh` | Offline tests for the guard, with a fake `gh`: `bash .github/scripts/test-verify-release-tag.sh`. |
| `.github/scripts/check-workflow-triggers.py` | Fails if any workflow can be started by anything other than a `vX.Y.Z` tag (or a push to `main` that ignores `.github/**`). See [the trigger rule](tech-ci.md#the-workflow-trigger-rule). |
| `.github/scripts/test_check_workflow_triggers.py` | Tests for that check, including a run over the real workflow files. |
| `.github/workflows/lint.yaml` | Guard, then `golangci-lint`; opens an issue on failure. |
| `.github/workflows/vulncheck.yaml` | Guard, then `govulncheck` with a step summary; opens an issue on failure. |
| `.github/workflows/build.yaml` | Guard, gate, then 14 cross-builds (each attested and given a `.sha256` file), then the GitHub Release with the binaries and checksum files attached. |
| `.github/workflows/docker-hub.yaml` | Guard, gate, then a multi-arch image pushed to Docker Hub. |
| `.github/workflows/ghcr.yaml` | Guard, gate, then the same image pushed to GitHub Container Registry. |
| `.github/workflows/check.yaml` | After a merge to `main` (never a branch, PR or tag; ignores `.github/**`): `make check-quick` when the local check receipt is there, the whole `make check` when it is not; opens an issue on failure. |
| `.github/scripts/changelog-section.sh` | Prints one release's section of `CHANGELOG.md`: the release page's "What's new" (with `test-changelog-section.sh`). |
| `.github/scripts/post-check-status.sh` | `make attest`: records that `make check` passed locally, as the commit status `local/make-check` on the pushed commit. |
| `.github/scripts/verify-local-check.sh` | The Check workflow's side: is there a valid receipt (success, from whoever pushed, for this tree)? Writes `fast=true/false`; never fails. |
| `.github/scripts/test-local-check.sh` | Tests for both, in throw-away repositories with a fake `gh`. |
| `.github/workflows/readme-reconcile.yaml` | After a release tag, once the GitHub Release exists: proposes `main`'s README for it (a branch and a pull request, or an issue that links the branch). Never commits to `main`. |
| `.github/scripts/readme-reconcile.py` | Brings a README in line with the release tags, the changelog, the milestones and the binary's `--help`, puts back what it must always have (expansion, badges with the donate button, support line), warns about what it cannot know; idempotent, fails closed. |
| `.github/scripts/test_readme_reconcile.py` | Its tests, on README fixtures (35 of them). |
| `.github/scripts/readme-release.sh` | `make readme-reconcile`: branches `readme/main-<tag>` off `main`, runs the script, commits, and with `PUSH=1` pushes and opens the pull request. |
| `.github/scripts/test-readme-release.sh` | Tests for it, in throw-away git repositories with a fake `gh`. |
| `.github/scripts/release-check.sh` | `make release-check`: the release-day preflight on a release branch (on top of `main`, `readme.md` equal to `main`'s, version, changelog, release commit, no attribution trailers, tag free). |
| `.github/scripts/test-release-check.sh` | Tests for it, in throw-away git repositories. |
| `.github/scripts/write-checksums.sh` | Writes `<binary>.sha256` next to each release binary and verifies it. |
| `.github/scripts/test-write-checksums.sh` | Offline tests for it: format, an independent hash, the platform's own verifier, tampering. |
| `.github/scripts/write-ci-failure-issue.sh` | Writes the body of the issue a failed check files, with the real run URL. |
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
