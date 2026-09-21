---
title: Testing
lead: How shint is tested, what the tests are built on, and which past bugs each regression test guards.
description: Testing strategy for shint - hermetic tests, helpers, test hooks, end-to-end exit-status tests, the smoke test, and how to run everything.
section: Technical
order: 7
nav: Testing
---

## Approach

- **Hermetic.** Unit and integration tests never leave the machine: each test starts its own listener on `127.0.0.1` or `::1` (port `0`, so the OS picks a free one) and points the code at it. No test needs the internet, DNS, or root - except where noted for ICMP.
- **Real sockets, not mocks.** The tests dial real listeners and run real TLS handshakes with a throw-away certificate authority. A mock would not catch a wrong timeout or a byte-count that is only wrong on the wire.
- **The race detector is on.** Handlers are concurrent; run `go test -race`.
- **Every bug gets a test that fails without the fix.** Each regression test below was checked against the broken behaviour before the fix was kept.

There are 111 top-level Go tests: 79 in `lib/handlers`, 25 in `lib` and 7 end-to-end tests in `main_test.go` (one of which, `TestExitStatus`, runs 27 scenarios). Outside `go test`, `make check` also runs a shell test for the [release-tag guard](tech-ci.md#the-release-tag-guard), Python tests for the [workflow trigger rule](tech-ci.md#the-workflow-trigger-rule) (18) and for the documentation generator (10).

## Running the tests

**The Makefile is the one place to build and test from.** Every check has a `make` target, and CI and the release checklist use those targets rather than their own copies of the commands:

| Command | What it runs | Needs |
|---|---|---|
| `make check` | Everything below except the live test, in this order: tool check, `gofmt`, `go vet`, `go test -race ./...`, the black-box battery, `golangci-lint`, `govulncheck`, the documentation tests and check, the workflow checks | the tools below; no network for the tests (`govulncheck` reads the vulnerability database) |
| `make test-live` | The [live smoke test](#the-live-smoke-test) | the internet and unprivileged ICMP |
| `make test-full` | `make check`, then `make test-live` - run this before a release | both |
| `make test` | The Go tests with the race detector, then the [black-box battery](#the-black-box-battery) | Go, python3 |
| `make test-go`, `make test-battery` | Each half of `make test` on its own | as above |
| `make fmt-check`, `vet`, `lint`, `vuln`, `docs-check`, `workflows` | One step of `make check` on its own | as above |

`make check` takes about a minute and a half (the Go tests about 15 s cold, the battery about a minute). The first thing it does is check that the tools are installed and prints the install command for any that are not: `golangci-lint` (it must be **v2.13.2**, the version CI pins - a different version is refused), `govulncheck`, `actionlint`, and `python3`. A tool that is not on `PATH` can be named: `make check GOLANGCI_LINT=/path/to/golangci-lint`.

For finer control, call `go test` directly:

```bash
go test -race -count=3 ./...                   # shake out flakiness
go test -run TestNmap -v ./lib/handlers/       # one family, verbosely
```

`go test -race` on the root package takes about 15 seconds because its tests start the CLI as subprocesses.

## Building blocks

| Helper | Where | What it gives you |
|---|---|---|
| `captureStdout(t, fn)` | `testhelpers_test.go` | Runs `fn` with `os.Stdout` redirected and returns what it printed. The pipe is drained by a goroutine *while* `fn` runs, so a handler that prints more than the pipe buffer cannot deadlock. |
| `freeTCPPort`, `freeUDPPort` | `testhelpers_test.go` | An unused port number, for tests that need to know it in advance. |
| `generateTestCA`, `issueCert` | `testhelpers_test.go` | A self-signed CA plus server and client certificates written to a temp directory, for mutual-TLS tests. |
| `startEchoListener` (+ IPv6 form) | `telnet_test.go` | A TCP listener that accepts and closes. |
| `rawServer` | `wirebytes_test.go` | A bare TCP/TLS responder that knows exactly how many bytes each request occupied, to check `web`'s counts against ground truth. |
| `recordingListener` | `wirebytes_test.go` | Wraps accepted connections in counters so a test can total what a real server read and wrote. |
| `runShint`, `TestMain` | `main_test.go` | Runs the *real CLI* as a subprocess and returns its exit status, stdout and stderr. |

### Hooks for testing

Two package-level variables exist only so tests can control time and slowness deterministically:

- `probePort` (`nmap.go`) - the single-port check. Tests swap in a slow or scripted probe instead of depending on a genuinely unresponsive host.
- `progressInterval` (`nmap.go`) - how often `nmap` prints progress. Tests shorten it to milliseconds.

### The CLI as a subprocess

`TestMain` checks `SHINT_TEST_RUN_MAIN=1`. When it is set, the test binary calls `main()` and exits - so `os.Args[0]` *is* shint. `runShint` re-executes the test binary with that variable and a real argument list, and captures the true process exit status and both output streams. There is no build step and no dependence on a `shint` binary on `PATH`. This is how the exit-status rules are tested: what a shell script sees is exactly what is asserted.

## What guards what

| Test | The bug it prevents |
|---|---|
| `TestTelnetHandlerRunLongerThanTimeoutStillSucceeds` | `--timeout` used as a run-wide deadline: later attempts failed instantly with a bogus `i/o timeout`. |
| `TestNmapHandlerCoversWholeRangeEvenWhenItOutlastsPerPortTimeout`, `TestScanContextHasNoDeadline` | The same flaw in `nmap`: scans stopping partway through the range. |
| `TestTelnetHandlerInterrupted...`, `TestWebHandlerInterrupted...`, `TestUDPHandlerInterrupt...`, `TestPause...`, `TestWatchCancel...` | `Ctrl+C` ending a repeating check with nothing to show for it; a request in flight counted as a failure; a long `--delay` making `Ctrl+C` wait; a probe cut off mid-wait reported as `open|filtered`. |
| `TestCtrlCShowsTheSummary` | The same, end to end: a real `SIGINT` to the real CLI - the `interrupted` line, the statistics, the `done` line, exit status `1`. |
| `TestNmapHandlerReportsInterruptedScan` | A cut-short scan presented as complete; aborted dials recorded as "closed". |
| `TestNmapHandlerThrottleDelayIsInterruptible` | `Ctrl+C` having to wait out a random 10-second throttle delay. |
| `TestNmapHandlerReportsProgress`, `...FastScanPrintsNoProgress`, `...JSONHasNoProgressLines` | Progress output: present for slow scans, absent for quick ones, never in JSON. |
| `TestWebAndHTTPListenAgreeOnBytes` | Client and server disagreeing about bytes (0 B to 2 MB, including bodies over `net/http`'s discard limit). |
| `TestWebHandlerBytesIncludeHeaders`, `...CoverEveryRedirectHop`, `...ReusedConnectionCountsPerRequest` | Byte counts that were body-only, last-hop-only, or cumulative across a reused connection. |
| `TestHTTPListenHandlerIgnoresRequestHeaders` | The listener acting on conditional headers or emitting validators. |
| `TestExitStatus` (27 scenarios) | Failed checks or bad usage exiting `0`; the 404-is-a-response, completed-scan and `open|filtered` rules. |
| `TestUsageErrorsGoToStderrOnly` | Usage errors on stdout, or printed twice. |
| `TestFailuresUnderJSONAreStillJSON` | A stray text line in front of the JSON when a check fails. |
| `TestListenExitsZeroWhenDone` | A finished listener exiting non-zero. |
| `TestConfigurePingerAppliesTimeout` | `ping --timeout` being recorded but never applied: every echo request waited a fixed second. |
| `TestValidatePingPayload`, the `ping` cases of `TestExitStatus` | A `--payload` outside 0-1448 silently clamped, and `--timeout 0` accepted, instead of a usage error (exit `2`). |
| `TestIsLostPing` | A lost echo request logged at `OK` level while the run exited `1`. |
| `TestWithPayloadSize`, `TestHandleICMPShowsPayloadSize` | Reply lines without the payload size; `payload_size_bytes` in the JSON being `0`. |
| `TestUDPHelpDoesNotPromiseEscapes` | `udp --help` showing `\x00` escapes, which are never interpreted. |

## The black-box battery

`go test` calls the code; the battery runs **the shipped binary** the way a user does and attacks it from the outside. It is 356 cases in `test/battery`, started by `make test-battery` (part of `make test` and `make check`, about a minute) and needing only `python3` and `go`. Everything stays on the machine: the cases talk to servers the battery starts on loopback, and each server misbehaves in one particular way - an HTTP server with a path for a truncated body, a reset mid-body, a stall, a redirect loop, a 2 MB header, a 300 MB download, chunked and gzip and HTTP/1.0 replies, garbage instead of HTTP; TLS servers with a good and a wrong certificate; TCP servers that echo, close at once or say nothing; UDP servers that echo, stay silent or answer with more than a packet's worth; a closed TCP and UDP port.

| Group | What it throws at the tool |
|---|---|
| A, B | the command line: unknown commands and flags, flags before and after arguments, every numeric flag at zero, negative, huge and not-a-number, boolean forms |
| C, D | hosts and ports written every odd way: `127.1`, `0x7f000001`, `::ffff:127.0.0.1`, 300-character names, Unicode, trailing dots, ports 0, 65536, `+80`, `0x50`, Arabic digits |
| E | `telnet`: refused, silent and closing servers, a black-hole address, DNS failure, 100 fast attempts, file-descriptor pressure |
| F | `web`: URL forms, every method, header edge cases including CR/LF injection and 200 headers, hostile servers, TLS and mutual-TLS options, proxy variables |
| G, H, I | `nmap` (single ports, ranges, the whole port range, filtered targets), `udp` (payload sizes, closed and silent ports, big replies), `ping` (payload limits, unreachable, multicast, and a reply that belongs to another ping) |
| J | `listen`: 20 MB uploads, resets, binary data, idle timeouts, 150 concurrent connections, malformed and half-finished HTTP requests |
| K, L | signals and closed pipes (SIGINT, SIGTERM, `| head -1`), and memory use on a 300 MB response |

Each invocation is judged against rules that hold for every command - no panic, exit status 0, 1 or 2, `--json` prints one valid document, a usage error goes to stderr, exit 1 comes with an `ERROR` line and exit 0 without one, nothing hangs - plus what the case itself expects. Cases that open hundreds of connections run one at a time, after the parallel ones.

**Known issues.** The battery was written by hunting for bugs, so some cases fail because of bugs that are open on the issue tracker. Each is listed in `test/battery/known_issues.py` with its issue number and **must keep failing**: the run is green while the bug exists, and turns red when the case starts to pass, telling you to delete the line. Anything failing that is not listed fails the run. So a fix and its line in the list change in the same commit, and a new bug found by the battery is added to the list with its issue. Details for adding a case are in `test/battery/README.md`.

## The live smoke test

`make test-live` (which runs `basic_module_test.sh`) builds the binary and runs one check per command against real hosts (`google.com`, `httpbin.org`, `8.8.8.8`), asserting on exit status *and* an expected string in the output - including that `ping` shows its payload size. It needs the internet and ICMP, so it is not part of `make check` or `go test`; `make test-full` includes it, and it should pass before a release. It counts failures and continues rather than stopping at the first.

## The release-tag guard

Workflows run only for a release tag, so a mistake in one would first show itself on release day. Two things stand in for that. `bash .github/scripts/test-verify-release-tag.sh` runs the guard script against a fake `gh` (no network) and checks: a name that is not `vX.Y.Z` is refused **without** calling GitHub; `identical` and `behind` pass; `ahead` and `diverged` are refused; an API error refuses; a missing input is an error. And the script was run once against the real repository - v4.0.3's commit passes, a release-branch tip is refused as `ahead` - with the same `gh api` call the workflow makes. The workflow files themselves are checked with [actionlint](https://github.com/rhysd/actionlint), and by the [trigger-rule check](tech-ci.md#the-workflow-trigger-rule), which fails if any workflow could be started by anything but a release tag on `main` (or, for a future workflow, a push to `main` that ignores `.github/**`). All three run as `make workflows`, part of `make check`.

## Lint and vulnerability checks

`golangci-lint` (v2.13.2, default linters) and `govulncheck` gate every release in CI ([CI/CD workflows](tech-ci.md)). `make lint` and `make vuln` run them locally, as part of `make check`; CI will not tell you about a failure until the tag is already pushed.

## Writing a good test here

- Start a real listener, and use port `0`.
- Keep durations short but assert on *behaviour*, not timing, where you can; where timing is the point (a run outlasting a timeout), make the margin generous.
- Never call `t.Fatal` from inside a `captureStdout` callback - it would leave stdout redirected.
- Add a test that fails without your fix, and confirm it does.
