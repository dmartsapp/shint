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
| `main.go` | The command line. Defines the cobra commands (`telnet`, `ping`, `web`, `nmap`, `udp`, `listen tcp|udp|http`), every flag, argument validation, the exit-status scheme, and `scanContext`. Holds the `Version` constant. |
| `main_test.go` | End-to-end tests of the CLI. `TestMain` makes the test binary behave as `shint` when `SHINT_TEST_RUN_MAIN=1`, so tests run it as a subprocess and assert real exit statuses and stdout/stderr. |
| `go.mod`, `go.sum` | The module (`github.com/dmartsapp/shint`), the Go version, and pinned dependencies. |
| `Makefile` | The one place to build and test from: cross-compilation targets for every release platform, the version-string logic for local builds, and the checks (`make check`, `make test-live`, `make test-full`). |
| `Dockerfile` | Two-stage image build: Go build stage, distroless final stage. |
| `.dockerignore`, `.gitignore` | What stays out of the Docker build context and out of git (`bin/`, `*.json`, `.DS_Store`). |
| `basic_module_test.sh` | Live-internet smoke test: builds the binary and runs one check per command against real hosts. Run it with `make test-live`. |
| `CHANGELOG.md` | The release history; also rendered as the [Changelog](changelog.md) page. |
| `readme.md` | The short introduction that GitHub shows on the front page. |
| `LICENSE` | MIT. |
| `index.html`, `.nojekyll` | Make GitHub Pages serve the site under `docs/` as plain static files (see [This documentation](tech-docs.md)). |

### main.go in detail

| Symbol | What it is |
|---|---|
| `Version` | The version string. `"4.0.4"` for a source build; overridden with `-ldflags "-X main.Version=..."` by the Makefile, the release workflow and the Dockerfile (see [Releases and tagging](tech-release.md#how-the-version-string-gets-into-the-binary)). |
| flag variables (`iterations`, `timeout`, ...) | Package-level variables bound to flags in `init()`. Shared flags live on the root command as *persistent* flags. |
| `exitOK`, `exitFailure`, `exitUsage`, `exitCode` | The exit-status scheme (0 / 1 / 2) and the value `main` exits with. |
| `usage(msg)` | Prints a usage error to stderr and records status 2. |
| `finish(ok)` | Records status 1 when a handler reports failure. |
| `rootCmd`, `telnetCmd`, `pingCmd`, `webCmd`, `nmapCmd`, `udpCmd`, `listenCmd` (+ `listenTCPCmd`, `listenUDPCmd`, `listenHTTPCmd`) | The commands. Each `Run` validates, calls one handler, and passes its result to `finish`. |
| `interruptContext()` | The context `nmap`, `telnet`, `web` and `udp` run under: cancelled by `Ctrl+C`/`SIGTERM`, deliberately without any deadline, and un-registered after the first signal so a second `Ctrl+C` ends the process. |
| `init()` | Registers all flags. Note `listenCmd` re-declares `--count` (default `0`) and `webCmd` re-declares `--payload` (`-P`, the body), shadowing the root flags. |
| `main()` | Adds the commands, runs cobra, exits with `exitUsage` if cobra returned an error, otherwise with `exitCode`. |

## lib - shared code

| File | Contents |
|---|---|
| `lib/lib.go` | Constants `DATETIMEFORMAT` (`time.UnixDate`), `NetworkType` (`"ip"`: A and AAAA) and `Protocol` (`"tcp"`). `ResolveName` and `ResolveNameToIPs` (DNS), `IsPortUp` (one TCP dial bounded by a per-attempt timeout *and* a context), `ValidatePort`, `RequirePositive`, `GetMinAvgMax` (latency statistics), `ConvertIPToStringSlice`, and `SortTimeDurationSlice` (currently exercised only by tests). |
| `lib/output.go` | The JSON contract: `JSONOutput`, `InputParams`, `DNSLookup`, and the per-command stats types `TelnetStats`, `WebStats`, `NmapStats`, `ICMPStats`, `UDPStats`, plus the listener events `ListenEvent` and `HTTPListenEvent`. Also the text log: `LogWithTimestamp` (the `time: [module] OK|ERROR message` prefix), `Fields` (the `key=value` suffix, quoting values with spaces) and `LogStats` (the statistics banner). |
| `lib/lib_test.go`, `lib/output_test.go` | Unit tests for the above: validators, statistics maths, DNS (loopback, IPv6 literal, dual-stack, invalid host), `IsPortUp` (open, closed, IPv6, context cancellation), log formatting. |

## lib/handlers - one file per command

| File | Contents |
|---|---|
| `telnet.go` | `TelnetHandler`: resolve, then one connection attempt per iteration per address; reports `true` only if all succeed. |
| `icmp.go` | `HandleICMP`: wraps go-ping's `Pinger` (parallel pings, streamed log lines), converts its statistics to shint's output; reports `false` on any lost ping. |
| `web.go` | `WebHandler`: builds the request (method, body, headers), sends it through a counting transport, and reports status, timing, speed and bytes. Records failed attempts in JSON `stats`. `HTTP_CLIENT_USER_AGENT` is the default `User-Agent`. |
| `wireconn.go` | Wire-level byte counting shared by `web` and `listen http`: `countingConn` (a `net.Conn` wrapper), `countingListener`, `newCountingTransport` (HTTP/1.1 transport whose plain and TLS connections are wrapped) and `wireMeter` (per-request accounting across connections and redirect hops). |
| `tls.go` | `BuildTLSConfig`: turns `--cacert`, `--cert`/`--key` and `--insecure` into a `tls.Config` (minimum TLS 1.2). |
| `nmap.go` | `NmapHandler`: the port scanner. `maxConcurrentPortScans` (500), `progressInterval` (3 s) and `probePort` (the single-port check, a variable so tests can substitute it). |
| `udp.go` | `probeUDP` (one probe, classified as open / closed / open\|filtered / error) and `UDPHandler`. |
| `interrupt.go` | How a repeating check ends on `Ctrl+C`: `pause` (a `--delay` that a cancellation cuts short), `watchCancel` (expires a connection's deadline so a blocked read returns), and `interruptedLine` / `interruptedNote` (the `ERROR interrupted ...` line and the JSON run-level error). |
| `tcplisten.go` | `TCPListenHandler` and `handleTCPConnection`; also `previewBytes`, the short single-line preview used by all listeners and `udp`. |
| `udplisten.go` | `UDPListenHandler`. |
| `httplisten.go` | `HTTPListenHandler`: the minimal HTTP server. `httpExchange` records the one request each connection served, reported when the connection closes so byte counts are final. |

### Tests in lib/handlers

| File | What it covers |
|---|---|
| `testhelpers_test.go` | Shared helpers: `captureStdout` (redirects stdout to a pipe drained concurrently), `freeTCPPort`, `freeUDPPort`, and a throw-away certificate authority (`generateTestCA`, `issueCert`, `writePEM`) for TLS tests. |
| `telnet_test.go` | Success and failure output, IPv6 loopback, DNS failure, concurrency (no data race), and the regression test that a run longer than `--timeout` still succeeds. |
| `icmp_test.go` | Text and JSON mode, the payload size shown on replies, and the pieces that need no network: `--timeout` reaching the pinger, payload validation, and which log lines count as a lost request (the live tests skip themselves where unprivileged ICMP is unavailable). |
| `web_test.go` | A full request/response round trip and mutual-TLS scenarios (with the client certificate, without it, without trusting the CA, and with `--insecure`). |
| `wirebytes_test.go` | The byte-measurement guarantees: `web` and `listen http` agree for bodies from 0 B to 2 MB, `web`'s counts match a raw server over HTTP and HTTPS, keep-alive reuse, redirect hops, header-insensitivity of the listener. |
| `tls_test.go` | Every branch of `BuildTLSConfig`. |
| `nmap_test.go` | Finding open ports (IPv4 and IPv6), the concurrency cap, whole-range coverage, interruption reporting, progress lines, `--json` cleanliness. Uses the `probePort` hook. |
| `udp_test.go` | `probeUDP` for each state, generated payloads, multiple attempts. |
| `cancel_test.go` | `Ctrl+C` on `telnet`, `web` and `udp`: how far the run got, the statistics for what completed, an attempt in flight dropped rather than counted as a failure, a long `--delay` cut short, a complete JSON document, and the `pause` / `watchCancel` helpers. |
| `tcplisten_test.go`, `udplisten_test.go`, `httplisten_test.go` | Accept and echo, IPv6 binding, JSON events and their measurements, and "runs until interrupted" with `--count 0`. |

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
| `.github/workflows/build.yaml` | Guard, gate, then 14 cross-builds, then the GitHub Release with the binaries attached. |
| `.github/workflows/docker-hub.yaml` | Guard, gate, then a multi-arch image pushed to Docker Hub. |
| `.github/workflows/ghcr.yaml` | Guard, gate, then the same image pushed to GitHub Container Registry. |
| `.github/ISSUE_TEMPLATE/ci_failure.md` | The body of the issue a failed lint or vulnerability run files. |

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
