---
title: Design notes
lead: The non-obvious decisions in shint, why they were made, and the bugs that shaped them.
description: Design notes for shint - timeouts, wire-level byte counting, the port scanner, UDP classification, dual-stack behaviour and the minimal HTTP listener.
section: Technical
order: 4
nav: Design notes
---

These notes explain the parts of shint that look surprising until you know the history. Each ends with what pins the behaviour in place.

## Timeouts bound one operation, never the whole run

**The rule.** `--timeout` is what *one* thing gets: one connection attempt, one request, one port, one probe, one DNS lookup. It is never a deadline for the whole command.

**Why.** `--count` and `--delay` stretch a run far past `--timeout` (eight attempts one second apart take eight seconds), and a scan of thousands of ports takes many times the per-port timeout. Two commands once handed `--timeout` to the *whole run* as a context deadline as well:

- `telnet --count 8` reported attempts 5-8 as `i/o timeout` after ~100 microseconds - even against a listener that was up - because the run-wide deadline had already passed when they started.
- `nmap --from 20 --to 9000` against a host that drops packets silently stopped after about 5 seconds, having tried a fraction of the range, and printed nothing to say so.

Both were false results presented as real ones, which is the worst kind of bug for a diagnostic tool.

**How it is enforced.** `main.go` gives no command a deadline. The commands that repeat a check - `nmap`, `telnet`, `web`, `udp` - get `interruptContext()`, cancelled only by `Ctrl+C` (or `SIGTERM`); it has no deadline for `--timeout` to leak into. Each timeout is derived *inside* the handler, per operation: a DNS lookup gets its own `context.WithTimeout`, and so does each connection attempt.

**What pins it.** `TestTelnetHandlerRunLongerThanTimeoutStillSucceeds`, `TestNmapHandlerCoversWholeRangeEvenWhenItOutlastsPerPortTimeout`, `TestInterruptContextHasNoDeadline`.

## Bytes are counted on the wire

**The rule.** `bytes_sent` and `bytes_received` (in `web`) and `bytes_received` / `bytes_sent` (in `listen http`) are the raw bytes that crossed the connection - request or status line, headers and body - and both sides count them the same way, so they agree.

**Why.** An earlier version estimated the response-header size from the parsed headers and never measured what was sent, while the listener counted only body bytes. The two could never match, and an estimate cannot see what the HTTP client adds itself (`Host`, `Accept-Encoding`, `Content-Length`) or chunked-encoding framing.

**How it works.**

- `countingConn` wraps a `net.Conn` and tallies bytes read and written.
- On the client, `newCountingTransport` wraps every connection. HTTPS connections are dialed and handshaken inside `DialTLSContext` so the wrapper sits *above* TLS and counts HTTP bytes, matching what a plain-HTTP listener sees.
- A connection can be reused (keep-alive) or several can be used (redirects), so `wireMeter` snapshots each connection's counters when the request first gets it (`httptrace.GotConn`) and reports only the difference, summed over every connection touched.
- On the server, `countingListener` wraps accepted connections, and the request is *reported when its connection closes* - the only moment the counts are final, since `net/http` finishes reading a body and writing the last chunk after the handler returns.

**Details that matter.**

- The listener answers `Connection: close`, so each connection carries exactly one request and the connection's counts *are* the request's counts.
- The listener **reads and discards the request body before answering.** Go's HTTP client abandons an upload still in progress the moment a complete `Connection: close` response arrives, so answering first would cut a large POST short (a 200 KB body stopped at ~168 KB) and both sides' counts with it. The body is never inspected.
- It never parses or acts on headers - no conditional requests, no `ETag`. A validator is only useful if `If-None-Match` were honoured, and the listener deliberately does not.
- `net/http` half-closes with `CloseWrite` when it leaves a body unread; the wrapper passes that through so the behaviour is not silently lost.

**What pins it.** `TestWebAndHTTPListenAgreeOnBytes` (0 B to 2 MB), `TestWebHandlerBytesIncludeHeaders` (plain and TLS, against a raw server that knows the truth), `TestWebHandlerReusedConnectionCountsPerRequest`, `TestWebHandlerBytesCoverEveryRedirectHop`, `TestHTTPListenHandlerCountsRawBytes`, `TestHTTPListenHandlerIgnoresRequestHeaders`.

## The port scanner

**Concurrency.** One goroutine per port, but a buffered channel used as a semaphore holds at most `maxConcurrentPortScans` (500) in flight, so `--from 1 --to 65535` cannot exhaust file descriptors or flood the target.

**Time is dominated by silence.** A port that refuses answers in milliseconds; a port whose packets are dropped costs the whole `--timeout`. Worst-case time is roughly `ports / 500 x timeout`, and results arrive in bursts because a whole batch times out together. That is why a scan prints `scan started` with the total, then a `progress` line every three seconds including `in_flight` - so a burst-wise scan does not look hung.

**Honest counts.** `ports_scanned` counts probes that *finished*. A dial aborted by cancellation is neither open nor closed, so it is not recorded as either; a scan that did not reach its plan says `scan interrupted ports_scanned=X ports_total=Y` and exits `1` instead of looking complete.

**Testability.** The single-port check is the variable `probePort`, and the progress period is `progressInterval`; tests substitute a slow or scripted probe and a short interval rather than depending on real slow networks.

## UDP classification

UDP has no handshake, so `probeUDP` uses a *connected* UDP socket and reads the result the way a UDP scanner does: a reply means `open`; an ICMP port-unreachable surfaces as a "connection refused" read error and means `closed`; a read timeout means `open|filtered`, which cannot be told apart from a firewall dropping the datagram. The exit status treats only `closed` and errors as failures, because `open|filtered` is genuinely inconclusive.

## Dual-stack by default

`lib.NetworkType` is `"ip"`, so every lookup returns both A and AAAA records, and `telnet`, `ping`, `nmap` and `udp` test *each address separately* (`web` resolves the name for its report, and the HTTP client then picks an address itself). That is the useful behaviour - IPv6 problems are exactly the ones you would otherwise miss - and it has a consequence worth knowing: on a network without IPv6, the IPv6 attempt fails, so the exit status is `1` even though the service works over IPv4. Test a specific address, or read per-address results from `--json`, when that matters.

## The HTTP listener is deliberately minimal

`listen http` answers `{"status":"ok"}` on `/` and 404 elsewhere, for any method, and does nothing else. It makes no attempt to interpret requests, so it is a *fixed point* to measure against: the same request always gets the same response, byte for byte. On shutdown it flushes the response and waits (`Server.Shutdown` plus a short grace period) so the process cannot exit a beat before the operating system has sent the last response - a real race that once produced `curl: (52) Empty reply from server` on the final request.

## Output stability and known quirks

JSON keys are a contract (see [Command-line design](tech-cli.md#the-output-contract)). Three historical quirks are kept for compatibility rather than fixed, and are documented so nobody is surprised: `timeout_ms` holds seconds; `ping` reports milliseconds where the rest report microseconds; and `ping`'s `from_port`/`to_port` hold `7`, the echo port, which means nothing for ICMP.

`ping` applies `--timeout` to each echo reply (`configurePinger` calls go-ping's `SetReplyTimeoutInMS`), but not to the name lookup: go-ping's `NewPinger` resolves inside its constructor with a fixed 5-second limit, so nothing is left to configure by the time a pinger exists. A `--payload` the library would silently shrink is rejected up front (`ValidatePingPayload`), with the limit read from the library itself (`MaxPingPayload`) so it cannot drift.
