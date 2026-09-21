---
title: telnet
lead: Check whether a TCP connection to a host and port can be opened - the modern, scriptable version of typing `telnet host port` just to see if it connects.
description: shint telnet tests TCP connectivity to a host and port over IPv4 and IPv6, with timing, repeat counts and JSON output.
section: Commands
order: 1
nav: telnet
---

## What it does

`shint telnet` opens a TCP connection to the host and port you give it, reports whether it succeeded and how long it took, and closes it again. Nothing is sent; it only proves the port is reachable. If the name resolves to several addresses - typically one IPv4 and one IPv6 - **each address is tested separately**.

Use it to answer "can I reach this service from here?" before blaming the application.

## Syntax

```bash
shint telnet <host> <port> [--count N] [--delay MS] [--timeout S] [--throttle] [--json]
```

`<host>` is a name or an IP address; `<port>` is 1-65535. The [shared flags](usage.md#flags-shared-by-every-command) apply; `--payload` is not used.

## Examples

### Check a port

Start something to connect to (in another terminal, `shint listen tcp 9000`), then:

```bash
shint telnet 127.0.0.1 9000
```

```text
Sun Sep 20 01:50:09 MDT 2026: [telnet] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=165.625µs
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK connect ok host=127.0.0.1 port=9000 attempt=1/1 time=4.606042ms

======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 4.606042ms, average: 4.606042ms, maximum: 4.606042ms
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK done total_time=1.006831458s
```
### Check a public service (IPv4 and IPv6)

`google.com` resolves to two addresses, so two connections are made and both appear in the results:

```bash
shint telnet google.com 443
```

```text
Sun Sep 20 01:50:14 MDT 2026: [telnet] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.46.78] time=47.672458ms
Sun Sep 20 01:50:14 MDT 2026: [telnet] OK dns authoritative zone=google.com nameservers=[ns1.google.com,ns2.google.com,ns3.google.com,ns4.google.com] time=41.446ms
Sun Sep 20 01:50:15 MDT 2026: [telnet] OK connect ok host=2607:f8b0:400a:803::200e port=443 attempt=1/1 time=30.388833ms
Sun Sep 20 01:50:16 MDT 2026: [telnet] OK connect ok host=142.251.46.78 port=443 attempt=1/1 time=30.467459ms

======================================= telnet STATISTICS =======================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 30.388833ms, average: 30.428146ms, maximum: 30.467459ms
Sun Sep 20 01:50:16 MDT 2026: [telnet] OK done total_time=2.081599125s
```
### Repeat the check

```bash
shint telnet 127.0.0.1 9000 --count 2 --delay 300
```

```text
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=148.709µs
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK connect ok host=127.0.0.1 port=9000 attempt=1/2 time=1.276833ms
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK connect ok host=127.0.0.1 port=9000 attempt=2/2 time=1.144375ms

======================================= telnet STATISTICS =======================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 1.144375ms, average: 1.210604ms, maximum: 1.276833ms
Sun Sep 20 01:50:10 MDT 2026: [telnet] OK done total_time=604.396834ms
```
### A port that is closed

```bash
shint telnet 127.0.0.1 9999 --timeout 2
```

```text
Sun Sep 20 01:50:11 MDT 2026: [telnet] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=91.708µs
Sun Sep 20 01:50:12 MDT 2026: [telnet] ERROR connect failed host=127.0.0.1 port=9999 attempt=1/1 time=1.015625ms error="dial tcp 127.0.0.1:9999: connect: connection refused"

======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 0
Latency: minimum: 0, average: 0, maximum: 0
Sun Sep 20 01:50:12 MDT 2026: [telnet] OK done total_time=1.00302275s
```
The exit status is `1` whenever a connection fails, so this works in scripts:

```bash
shint telnet 127.0.0.1 9999 --timeout 2 --delay 0 || echo "not reachable"
```

## Reading the results

| Message | What it means |
|---|---|
| `connect ok` | The connection was established. `time` is how long that took. |
| `connection refused` | The host answered, but nothing is listening on that port. |
| `i/o timeout` | Nothing answered at all within `--timeout` - typically a firewall silently dropping the packets, or a host that is down. |
| `no route to host` / `network is unreachable` | There is no path to that address from this machine (common for an IPv6 address on a network without IPv6). Add `-4` to check IPv4 only. |
| `address family not supported by protocol` | This machine cannot use IPv6 at all (its kernel has it disabled), so the IPv6 address of a dual-stack name cannot be tried. Add `-4`. |
| `dns resolution failed` | The name could not be resolved, so no connection was tried. |

:::tip Tell a firewall from a stopped service
"Connection refused" comes back in milliseconds because the target actively said no. A timeout means silence. If a port that should be open times out, suspect a firewall on the path; if it is refused, the host is up but the service is not.
:::

## JSON

```bash
shint telnet 127.0.0.1 9000 --json
```

```json
{
  "input_params": { ... },
  "module_name": "telnet",
  "dns_lookup": { ... },
  "stats": [
    {
      "address": "127.0.0.1",
      "success": false,
      "recv_unixtime_µs": 0,
      "sent_unixtime_µs": 1789890611755345,
      "time_taken_µs": 709,
      "error": "dial tcp 127.0.0.1:9000: connect: connection refused"
    }
  ],
  "end_time_unixtime_µs": 1789890611756071,
  "start_time_unixtime_µs": 1789890610754105,
  "total_time_taken_µs": 1001966,
  "error": ""
}
```
Each attempt is one entry in `stats`; failed attempts have `"success": false` and an `error`. Times are in microseconds. See [Output formats](output.md) for every field.

## Good to know

- **`Ctrl+C` shows the summary.** A run with a large `--count` stops, prints how far it got, the statistics and the `done` line, and exits `1` (cut short) - see [Stopping early](usage.md#stopping-early).
- `--timeout` applies to each connection attempt (and to the DNS lookup), not to the whole run.
- With `--count N`, N attempts are made against **every** address, one after another, each preceded by `--delay`.
- The default `--delay 1000` means a single check takes about a second; use `--delay 0` for an immediate answer.
