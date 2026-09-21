---
title: Using shint
lead: Every command works the same way - a target, some flags, and readable output. Learn the shared parts once.
description: Command syntax, the flags shared by every shint command, text and JSON output, and exit status.
section: Get started
order: 3
nav: Using shint
---

## The shape of a command

```plain
shint <command> <target> [flags]
```

The target comes first (a host and port, a URL, a port to listen on); flags change how the check runs. Flags can go before or after the target.

```text
A simple network utility tool that provides telnet, ping, nmap, udp, web client, ntp, wol, rdns, cidr and listener functionalities.

Usage:
  shint [command]

Available Commands:
  cidr        Work out a subnet: network, mask, range and size
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  listen      Start a local TCP, UDP, or HTTP listener for testing
  nmap        Scan for open TCP ports on a host
  ntp         Check this machine's clock against an NTP time server
  ping        Send ICMP ECHO_REQUEST to a host
  rdns        Look up the names an IP address maps back to (reverse DNS)
  telnet      Connect to a host on a specific port
  udp         Send a UDP probe to a host on a specific port
  web         Make an HTTP request to a URL
  wol         Send a Wake-on-LAN magic packet to wake a machine on your network

Flags:
      --count int     Number of times to check connectivity (listen commands: max connections/packets to accept, 0 = unlimited) (default 1)
      --delay int     Milliseconds delay between each iteration given in count (default 1000)
  -h, --help          help for shint
  -4, --ipv4          Resolve and check IPv4 addresses only (a host with both kinds is normally checked over both)
  -6, --ipv6          Resolve and check IPv6 addresses only
      --json          Flag option to output only in JSON format
      --payload int   Ping/UDP payload size in bytes (filler content, ignored if --data is set on udp) (default 4)
      --throttle      Flag option to throttle between every iteration of count to simulate non-uniform request.
      --timeout int   Timeout in seconds to connect (listen commands: idle read timeout, 0 = no timeout) (default 5)
  -v, --version       version for shint

Use "shint [command] --help" for more information about a command.
```
`shint <command> --help` shows the details for one command, and `shint --version` prints the version.

## Flags shared by every command

These are defined once, so they mean the same thing everywhere they apply:

| Flag | Default | Meaning |
|---|---|---|
| `--count N` | `1` | How many times to repeat the check. For `listen`, the number of connections, packets or requests to accept before exiting (default `0`: keep going until Ctrl+C). |
| `--timeout S` | `5` | Seconds to wait - see [what it limits](#what-timeout-limits) below. For `listen`, the idle time before a quiet connection is closed (`0`: never). |
| `--delay MS` | `1000` | Milliseconds to pause before each attempt. Use `--delay 0` for back-to-back checks. |
| `--throttle` | off | Wait a random 0-10 seconds between attempts instead of a fixed `--delay`, to imitate uneven traffic. |
| `--payload N` | `4` | Filler payload size in bytes for `ping` and `udp`. (On `web`, `-P` is the request *body* instead.) |
| `--json` | off | Print one machine-readable JSON document instead of log lines. |
| `-4`, `--ipv4` | off | Resolve and check **IPv4 addresses only**. See [IPv4 only, or IPv6 only](#ipv4-only-or-ipv6-only). |
| `-6`, `--ipv6` | off | Resolve and check **IPv6 addresses only**. |

:::note Every attempt waits first
`--delay` is applied before each attempt, including the first, which is why a default `telnet` takes about a second. Add `--delay 0` when you want an immediate answer. (`cidr` does no attempts, so it has no delay.)
:::

### IPv4 only, or IPv6 only

A name with both IPv4 and IPv6 addresses is checked over **both**, and each address is reported on its own - that is how an IPv6 problem gets noticed. When you only care about one family, say so:

```bash
shint telnet google.com 443
```

```text
Sun Sep 20 22:33:54 MDT 2026: [telnet] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.46.78] time=46.0835ms
Sun Sep 20 22:33:55 MDT 2026: [telnet] OK connect ok host=2607:f8b0:400a:803::200e port=443 attempt=1/1 time=30.424709ms
Sun Sep 20 22:33:56 MDT 2026: [telnet] OK connect ok host=142.251.46.78 port=443 attempt=1/1 time=28.368791ms

======================================= telnet STATISTICS =======================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 28.368791ms, average: 29.39675ms, maximum: 30.424709ms
Sun Sep 20 22:33:56 MDT 2026: [telnet] OK done total_time=2.077337417s
```

```bash
shint telnet google.com 443 -4
```

```text
Sun Sep 20 22:33:56 MDT 2026: [telnet] OK dns resolved host=google.com addresses=1 ips=[142.251.46.78] time=5.221708ms
Sun Sep 20 22:33:57 MDT 2026: [telnet] OK connect ok host=142.251.46.78 port=443 attempt=1/1 time=30.681166ms

======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 30.681166ms, average: 30.681166ms, maximum: 30.681166ms
Sun Sep 20 22:33:57 MDT 2026: [telnet] OK done total_time=1.037681542s
```

`-4` resolves only the IPv4 address (`addresses=1`) and checks only that; `-6` does the same for IPv6. It is the right tool on a machine that has no IPv6 at all, where the IPv6 check would otherwise fail and turn the exit status into `1` even though the service is fine over IPv4 (see [Troubleshooting](troubleshooting.md#ping-and-telnet-report-an-ipv6-failure-but-the-host-works)).

The flags work on every command that resolves a name - `telnet`, `ping`, `nmap`, `udp`, `web`, `ntp` and `rdns`. For `web` they also bind the connection itself, so the HTTP client cannot pick the other family. A few rules keep them honest:

- `-4` and `-6` together is a usage error (exit `2`); so is `-4` with an IPv6 address, or `-6` with an IPv4 address, since nothing could be checked:

```text
::1 is an IPv6 address, but -4/--ipv4 was requested
```

- If a name has no address of the requested family, that is reported like a failed lookup (exit `1`).
- `wol` is IPv4 only (IPv6 has no broadcast), so `-6` is refused there. `cidr` and `listen` do not resolve names and ignore the flags (`listen` takes `--bind`).

### What --timeout limits

`--timeout` always bounds one operation, never the whole run - so `--count 20 --delay 2000` is free to take 40 seconds.

| Command | `--timeout` is how long... |
|---|---|
| `telnet` | each connection attempt, and the DNS lookup, may take |
| `web` | each request may take, from connecting to the last byte of the response |
| `nmap` | each *port* may take to answer, and the DNS lookup |
| `udp` | each probe waits for a reply, and the DNS lookup |
| `ping` | each echo request waits for its reply before it counts as lost (the name lookup keeps the ping library's own fixed 5-second limit) |
| `ntp` | each server gets to answer, and the DNS lookup |
| `rdns` | each reverse lookup gets, and the DNS lookup of a host name |
| `wol` | each send may take (it never waits for a reply - there is none) |
| `listen` | a connection may sit idle before it is closed |
| `cidr` | *(not used: it never waits)* |

## Reading the output

Every command prints one line per event in the same shape:

```plain
<time>: [<module>] OK|ERROR <message> key=value key=value ...
```

```text
Sun Sep 20 01:50:09 MDT 2026: [telnet] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=165.625µs
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK connect ok host=127.0.0.1 port=9000 attempt=1/1 time=4.606042ms

======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 4.606042ms, average: 4.606042ms, maximum: 4.606042ms
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK done total_time=1.006831458s
```
- The **module** in brackets says which command spoke (`telnet`, `icmp`, `web`, `nmap`, `udp`, `ntp`, `wol`, `rdns`, `cidr`, `listen-tcp`, `listen-udp`, `listen-http`).
- **OK or ERROR** is the level of that one line.
- The `key=value` pairs are easy to `grep` and `awk`; values with spaces are quoted.
- Commands that repeat a check finish with a **statistics** block: requests sent, responses received, and minimum, average and maximum latency.

The full description, including every JSON field, is on the [Output formats](output.md) page.

## JSON output

`--json` replaces the log lines with a single JSON document (the listen commands print one JSON line per event instead). Pipe it into `jq`, a script, or a monitoring agent:

```bash
shint web --json https://example.com | jq '.stats[0].status_code'
```

Failed checks are still valid JSON: they appear in `stats` with `"success": false` and the reason, so `--json` output never mixes text lines into the document.

## Exit status

shint checks several things per run - every address a name resolves to, every `--count` iteration - so its exit status follows the convention of `fping`, which does the same:

| Status | Meaning |
|---|---|
| `0` | Every check passed. |
| `1` | At least one check failed: connection refused or timed out, DNS failure, no HTTP response, a UDP port reported closed, a lost ping, a scan cut short, a run stopped by `Ctrl+C` before its `--count` was done, no usable time reply, an address with no reverse name, or a Wake-on-LAN packet that could not be sent. |
| `2` | The command was used wrongly - a bad argument, flag or value. Nothing ran. |

That makes shint usable in shell conditions and CI:

```bash
if shint telnet db.internal 5432 --timeout 2; then
  echo "database reachable"
else
  echo "database NOT reachable"
fi
```

What counts as a failure, per command:

| Command | Exit `1` when |
|---|---|
| `telnet` | the lookup or any connection attempt failed |
| `ping` | the lookup failed or any echo request went unanswered |
| `web` | any attempt got no HTTP response. **A 404 or 500 is still a response** and exits `0` - read the status from the output or `--json` |
| `nmap` | the lookup failed or the scan was cut short. Finding no open ports is a completed scan and exits `0` |
| `udp` | the lookup failed, the port was reported `closed`, or the probe errored. `open|filtered` (no reply) is inconclusive, not a failure |
| `ntp` | the lookup failed, a server did not answer, an answer could not be trusted (an unsynchronized server, a mismatched reply, a "kiss-o'-death"), or - with `--max-offset` - the clock is further out than that |
| `rdns` | the name lookup failed, or an address has no PTR record (or its lookup failed or timed out) |
| `wol` | a magic packet could not be sent. A packet that was sent is a success: there is no reply to check |
| `cidr` | never. Bad input is exit `2` and nothing is printed |
| `listen` | the port could not be bound |

Results, including `ERROR` lines about failed checks, go to **stdout**; usage errors go to **stderr**. So `shint ... --json | jq` only ever sees JSON.

## Stopping early

Press `Ctrl+C`. A run that repeats a check - `telnet`, `web`, `udp`, `ntp`, `rdns` and `wol` with `--count`, and `nmap` over its port range - stops and **tells you how far it got**, instead of leaving you with a bare `^C`:

```bash
shint web http://127.0.0.1:18091/ --count 100
```

```text
Sun Sep 20 23:14:31 MDT 2026: [web] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=48.791µs
Sun Sep 20 23:14:32 MDT 2026: [web] OK response url=http://127.0.0.1:18091/ status=200 bytes_sent=98 bytes_received=160 speed=58.33KB/s attempt=1/100 time=2.678917ms
Sun Sep 20 23:14:33 MDT 2026: [web] OK response url=http://127.0.0.1:18091/ status=200 bytes_sent=98 bytes_received=160 speed=83.95KB/s attempt=2/100 time=1.861292ms
Sun Sep 20 23:14:34 MDT 2026: [web] OK response url=http://127.0.0.1:18091/ status=200 bytes_sent=98 bytes_received=160 speed=91.62KB/s attempt=3/100 time=1.705375ms
Sun Sep 20 23:14:35 MDT 2026: [web] OK response url=http://127.0.0.1:18091/ status=200 bytes_sent=98 bytes_received=160 speed=92.28KB/s attempt=4/100 time=1.69325ms
Sun Sep 20 23:14:36 MDT 2026: [web] OK response url=http://127.0.0.1:18091/ status=200 bytes_sent=98 bytes_received=160 speed=138.13KB/s attempt=5/100 time=1.131208ms
Sun Sep 20 23:14:36 MDT 2026: [web] ERROR interrupted attempts_completed=5 attempts_planned=100 time=5.4974175s

========================================== web STATISTICS ==========================================
Requests sent: 5, Response received: 5, Success: 100%
Latency: minimum: 1.131208ms, average: 1.814008ms, maximum: 2.678917ms
Sun Sep 20 23:14:36 MDT 2026: [web] OK done total_time=5.497643708s
```

The run ended after five of the hundred requests. What you get on `Ctrl+C`:

- An `ERROR interrupted` line with `attempts_completed` and `attempts_planned`.
- The usual **statistics** for the attempts that did complete (`ntp`, `rdns`, `wol` and `udp` have no statistics block: their `done` line counts what completed), and the `done` line. With `--json` you get the one complete document, whose run-level `error` reads `interrupted: 5 of 100 attempts completed`.
- **An attempt that was still in flight is dropped, not counted as a failure**: it never finished, so it neither passed nor failed. The `--delay` pause you may have been in is cut short too.
- **Exit status `1`**: the run was cut short, exactly as for an interrupted `nmap` scan. (`0` means every requested check passed.)
- **A second `Ctrl+C` ends the process at once**, for the case where something is stuck.

The listen commands stop and print their summary, as before. `ping` still ends right away (the ping library it uses cannot be cancelled part-way; this is tracked and planned with its other upgrades).

## Next

Pick a command: [telnet](telnet.md), [ping](ping.md), [web](web.md), [nmap](nmap.md), [udp](udp.md) or [listen](listen.md) - or browse the [cookbook](cookbook.md).
