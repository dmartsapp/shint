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

**How it is enforced.** `main.go` gives no command a deadline. The commands that repeat a check - `nmap`, `telnet`, `web`, `udp`, `ntp`, `rdns`, `wol` - get `interruptContext()`, cancelled only by `Ctrl+C` (or `SIGTERM`); it has no deadline for `--timeout` to leak into. Each timeout is derived *inside* the handler, per operation: a DNS lookup gets its own `context.WithTimeout`, and so does each connection attempt.

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

**What it is given.** Since v4.4.0 ([#94](https://github.com/dmartsapp/shint/issues/94)), any number of targets - host names, addresses and prefixes - and the ports as a range or a list. Every target is checked before anything is scanned (`ParseNmapTargets`), so a typo in the third is a usage error, not two scans and a failure. A prefix is capped at 65,536 addresses across the run: past that a sweep takes hours, and `--json` would hold every result in memory. Each address is scanned once, whichever target led to it first, and a failed lookup does not stop the other targets - it makes the run exit `1`. The scan stays a TCP connect scan: raw SYN packets would need root (or `CAP_NET_RAW`), and a tool that answers "can this machine connect there" should connect the way its applications do.

**Concurrency.** One goroutine per port, but a buffered channel used as a semaphore holds at most `maxConcurrentPortScans` (500) in flight, so `--from 1 --to 65535` cannot exhaust file descriptors or flood the target.

**Time is dominated by silence.** A port that refuses answers in milliseconds; a port whose packets are dropped costs the whole `--timeout`. Worst-case time is roughly `ports / 500 x timeout`, and results arrive in bursts because a whole batch times out together. That is why a scan prints `scan started` with the total, then a `progress` line every three seconds including `in_flight` - so a burst-wise scan does not look hung.

**Honest counts.** `ports_scanned` counts probes that *finished*. A dial aborted by cancellation is neither open nor closed, so it is not recorded as either; a scan that did not reach its plan says `scan interrupted ports_scanned=X ports_total=Y` and exits `1` instead of looking complete.

**Testability.** The single-port check is the variable `probePort`, and the progress period is `progressInterval`; tests substitute a slow or scripted probe and a short interval rather than depending on real slow networks.

## UDP classification

UDP has no handshake, so `probeUDP` uses a *connected* UDP socket and reads the result the way a UDP scanner does: a reply means `open`; an ICMP port-unreachable surfaces as a "connection refused" read error and means `closed`; a read timeout means `open|filtered`, which cannot be told apart from a firewall dropping the datagram. The exit status treats only `closed` and errors as failures, because `open|filtered` is genuinely inconclusive.

## Dual-stack by default

`lib.NetworkType` is `"ip"` by default (`"ip4"` or `"ip6"` with `-4`/`-6`, set once at startup by `lib.SetIPFamily`), so every lookup returns both A and AAAA records, and `telnet`, `ping`, `nmap`, `udp`, `ntp` and `rdns` test *each address separately* (`web` resolves the name for its report, and the HTTP client then picks an address itself). That is the useful behaviour - IPv6 problems are exactly the ones you would otherwise miss - and it has a consequence worth knowing: on a network without IPv6, the IPv6 attempt fails, so the exit status is `1` even though the service works over IPv4. That is what `-4` and `-6` are for: they restrict resolution (and, for `web`, the HTTP client's own dialing) to one family, and contradicting them - both together, or an address of the other family - is a usage error. A failed IPv6 connect that is really this machine's lack of IPv6 (`address family not supported`, `network is unreachable`) gets a hint pointing at `-4`.

## The HTTP listener is deliberately minimal

`listen http` answers `{"status":"ok"}` on `/` and 404 elsewhere, for any method, and does nothing else. It makes no attempt to interpret requests, so it is a *fixed point* to measure against: the same request always gets the same response, byte for byte. On shutdown it flushes the response and waits (`Server.Shutdown` plus a short grace period) so the process cannot exit a beat before the operating system has sent the last response - a real race that once produced `curl: (52) Empty reply from server` on the final request.

## Checking, not reconnaissance

shint is for checking infrastructure you are responsible for. Any network tool can be pointed elsewhere. Skilled attackers have far better tools already, so shint is no help to them. The realistic risk is a well-meaning user sweeping a network that is not theirs. The rule for features, decided on 2026-11-18 ([#106](https://github.com/dmartsapp/shint/issues/106)):

- **Never anything whose main value is covert reconnaissance.** No stealth or evasion (raw packets, spoofing, decoys, fragmentation), no fingerprinting (banners, versions, operating systems), no vulnerability probing, no password guessing. Banner grabbing was dropped for related reasons (below).
- **Make the risky case deliberate, without new flags.** A sweep of public addresses prints a warning and is paced by `--delay`, the shared flag that already exists, at its default of 1 s per probe. Its owner sets their own pace, and `--delay 0` removes it. Hosts and private ranges keep full speed, so everyday checks cost nothing extra. A `--public` flag was considered and turned down: every extra parameter is something users must learn.
- **Be identifiable.** `web`'s User-Agent is `shint/<release> (+URL)`, not an anonymous string.
- **No policing.** No telemetry, phoning home, kill switch, target blocklists or accounts. shint is MIT-licensed open source, so such checks would be removed in minutes and stop no one. They would only break the promise that shint collects nothing.

## What shint does not do, and why

Some features were planned and then dropped. Each is recorded here with its reasons, so the question does not have to be argued again from scratch - and so it can be reopened on the same terms if the reasons stop holding.

### No banner grabbing in telnet

Planned for v4.3.0 as [#45](https://github.com/dmartsapp/shint/issues/45) and dropped on 2026-09-26, before any of it was built. `telnet --banner` would have shown what a server sends first (SSH, SMTP and FTP greet the client), and `--send TEXT`/`--expect TEXT` would have turned a line written after connecting, and the reply to it, into a pass/fail check.

The gap it would have filled is real: a connection that opens does not prove the service behind it works, because a load balancer, a proxy or a Kubernetes Service can accept connections with nothing healthy behind them. But the services where that matters most already have a command that checks them end to end, in their own protocol:

- **HTTP**: `web` - the status, the bytes and the timing of a real request.
- **DNS and NTP**: `dns` and `ntp`.
- **TLS**: a TLS server sends nothing until the client starts the handshake, so there is no banner to read. Checking a TLS service means doing the handshake, which is a command of its own (`tls`, planned for v4.3.0).

What is left is SSH, SMTP, FTP, POP3, IMAP and a few databases. `nc host port` already shows their greeting, and a real health check of one of them belongs to that service's own tooling.

`--send`/`--expect` is where the cost was. One flag becomes a small protocol scripter: SMTP wants `\r\n` line endings, a binary protocol would need a `--hex` like `udp`'s, and an exchange with more than one step (`EHLO`, then `QUIT`) does not fit one send and one expect. Each of those is the next feature request, and together they are a lot to maintain for a *simple* inspection toolkit.

So `telnet` stays what its name promises: can a connection be opened, from here, to that address and port, and how long did it take. Naming the service behind an open port is `nmap`'s job. Its service names (v4.4.0, [#94](https://github.com/dmartsapp/shint/issues/94)) come from the port number; if they ever need evidence rather than convention, reading a banner there is where to reconsider it.

### No iSCSI checks

Proposed for v4.5.0 as [#82](https://github.com/dmartsapp/shint/issues/82), alongside the Fibre Channel checks (#81), and dropped on 2026-09-26, before any of it was built. The idea was to check iSCSI storage from the client side: ask a portal which targets it offers, and ask the disks behind a target whether they are ready.

Every one of those checks is an iSCSI **login**, even the discovery that only lists targets. A login is not a probe but an event on the storage system. Storage arrays log it, their access lists and CHAP secrets decide whether it is allowed, and it creates a session. Getting the session's identity wrong is worse than a failed check. A login that reuses the initiator name and session ID of a session that is already open is, by the protocol's rules, a *reinstatement*: the target closes the existing session, so a health check could cut a production server off from its disks. That is more involvement with production storage than a simple inspection toolkit should have, however carefully it is written.

What remains without a login is either already here or not iSCSI:

- **Is the portal reachable?** `shint telnet <portal> 3260` - a connection, nothing sent.
- **Is this host's own session up?** The host's iSCSI tooling (`iscsiadm -m session`) reports it. The kernel also publishes each session's state in files any user can read, so shint could show it without a login; that part is not planned either, and is the one piece to reconsider alongside the Fibre Channel checks (#81), which read the same kind of state.

### No `ss` on AIX

Decided on 2026-11-18, when `ss` ([#68](https://github.com/dmartsapp/shint/issues/68)) was built for Linux and Windows. AIX has no documented interface that lists TCP and UDP sockets, with the process holding each, to a user without root. Its own tools read kernel memory as root: `netstat -A` gives each connection's control block address, `rmsock` turns that into a process, and `lsof` does the same. Its `/proc/<pid>/fd` lists a process's descriptors, but cannot open a socket entry, and does not say that an entry is one. Nothing on AIX plays the part of Linux's `/proc/net`. IBM's documentation for `/proc` and the network library routines was checked for exactly this, and has nothing.

shint never asks for root, and going through kernel memory or running other programs would break its rules anyway. So no version of `ss` could work on AIX. **The AIX binary is built without the command.** This is the first command that differs between binaries.

How it is done: a command that only some platforms build lives in a file of its own whose build tag leaves it out (`main_ss.go`, `//go:build !aix`). That file's `init` adds the command to `platformCommands`, which `main` registers with the others. With nothing calling it, the linker drops the handler code from that binary too.

The rule for next time: **leave a command out of a binary only when no version of shint could do it on that system.** When it is merely not written yet - `ss` on macOS and the BSDs ([#105](https://github.com/dmartsapp/shint/issues/105)) - the command stays in and says plainly that it is not supported there yet (exit `1`). A user then learns what is missing, and a script can tell "not yet" from "not on this system" (an unknown command, exit `2`).

## Output stability and known quirks

JSON keys are a contract (see [Command-line design](tech-cli.md#the-output-contract)). Three historical quirks are kept for compatibility rather than fixed, and are documented so nobody is surprised: `timeout_ms` holds seconds; `ping` reports milliseconds where the rest report microseconds; and `ping`'s `from_port`/`to_port` hold `7`, the echo port, which means nothing for ICMP.

`ping` applies `--timeout` to each echo reply (`configurePinger` calls go-ping's `SetReplyTimeoutInMS`), but not to the name lookup: go-ping's `NewPinger` resolves inside its constructor with a fixed 5-second limit, so nothing is left to configure by the time a pinger exists. A `--payload` the library would silently shrink is rejected up front (`ValidatePingPayload`), with the limit read from the library itself (`MaxPingPayload`) so it cannot drift.
