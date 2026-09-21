---
title: ntp
lead: Check this machine's clock against an NTP time server - the offset, the round trip, and whether the answer can be trusted.
description: shint ntp asks an NTP server for the time over UDP and reports how far this machine's clock is from it, for IPv4 and IPv6, with an optional pass/fail threshold.
section: Commands
order: 6
nav: ntp
---

## What it does

Certificates, logs, Kerberos, TOTP codes and distributed databases all break quietly when a machine's clock drifts. `shint ntp` asks a time server what time it is, works out how far this machine's clock is from the answer, and reports it - so you can tell "my clock is 3 seconds off" from "the network is slow" in one command.

It sends a single **SNTP** (the simple form of NTP, [RFC 4330](https://www.rfc-editor.org/rfc/rfc4330)) request over UDP port 123 to each address the server name resolves to, IPv4 and IPv6. It only **reads** the time: it never changes your clock, and needs no special privileges.

## Syntax

```bash
shint ntp <server> [--port N] [--max-offset MS] [--count N] [--timeout S] [--json]
```

| Flag | Default | Meaning |
|---|---|---|
| `--port N` | `123` | The server's UDP port. |
| `--max-offset MS` | `0` (off) | Turn the offset into a check: a server whose offset is larger than this many **milliseconds**, in either direction, fails the query and the command exits `1`. |

The other [shared flags](usage.md#flags-shared-by-every-command) apply; `--timeout` is how long each server gets to answer.

## What it reports

| Field | Meaning |
|---|---|
| `offset` | The server's time **minus this machine's time**. Positive means this clock is *behind*; negative means it is *ahead*. |
| `round_trip` | The network delay of the exchange, with the server's own processing time taken out. |
| `stratum` | How far the server is from a reference clock: `1` is attached to one (a GPS receiver, an atomic clock), `2` gets its time from a stratum 1, and so on. |
| `leap` | The server's leap-second warning: `none`, `insert` (the last minute of the day has 61 seconds), `delete` (59) or `unsynchronized`. |
| `version` | The NTP version the server answered with. |
| `reference` | What the server is synchronized to: a code such as `GPS` for stratum 1, otherwise the IPv4 address of its upstream server (or, when that one is IPv6, a hash that only looks like an address). |

How the offset is worked out is the standard NTP calculation: with T1 (request sent), T2 (server received it), T3 (server replied) and T4 (reply received),

```text
offset     = ((T2 - T1) + (T3 - T4)) / 2
round trip = (T4 - T1) - (T3 - T2)
```

which is right even though the two clocks disagree, and correct for the time the server spends between receiving and replying. Its accuracy is limited by how symmetric the network path is: an error of up to half the `round_trip` is possible, so on a fast network the offset is good to about a millisecond, on a slow or lopsided one less so.

## Examples

### A server with several addresses

```bash
shint ntp time.cloudflare.com
```

```text
Sun Sep 20 17:30:39 MDT 2026: [ntp] OK dns resolved host=time.cloudflare.com addresses=4 ips=[2606:4700:f1::1,2606:4700:f1::123,162.159.200.1,162.159.200.123] time=65.099125ms
Sun Sep 20 17:30:40 MDT 2026: [ntp] OK response server=time.cloudflare.com address=2606:4700:f1::1 attempt=1/1 stratum=3 offset=+342.371µs round_trip=17.080016ms leap=none version=4 reference=10.153.8.152 time=18.36625ms
Sun Sep 20 17:30:41 MDT 2026: [ntp] OK response server=time.cloudflare.com address=2606:4700:f1::123 attempt=1/1 stratum=3 offset=+264.983µs round_trip=14.698355ms leap=none version=4 reference=10.153.8.152 time=16.118625ms
Sun Sep 20 17:30:42 MDT 2026: [ntp] OK response server=time.cloudflare.com address=162.159.200.1 attempt=1/1 stratum=3 offset=+76.992µs round_trip=15.669518ms leap=none version=4 reference=10.153.8.152 time=16.583291ms
Sun Sep 20 17:30:43 MDT 2026: [ntp] OK response server=time.cloudflare.com address=162.159.200.123 attempt=1/1 stratum=3 offset=-81.488µs round_trip=15.396832ms leap=none version=4 reference=10.153.8.152 time=16.096666ms
Sun Sep 20 17:30:43 MDT 2026: [ntp] OK done queries=4 answered=4 total_time=4.137023958s
```

All four addresses answered, and this machine's clock is within a third of a millisecond of every one. Each address is asked in turn, `--delay` apart (a second by default), which is why four addresses took four seconds; `--delay 0` asks back to back.

### A threshold: is the clock good enough?

```bash
shint ntp pool.ntp.org --max-offset 1
```

```text
Sun Sep 20 17:30:43 MDT 2026: [ntp] OK dns resolved host=pool.ntp.org addresses=4 ips=[185.214.143.237,72.14.182.49,108.61.215.221,72.14.186.59] time=40.837583ms
Sun Sep 20 17:30:44 MDT 2026: [ntp] ERROR query failed server=pool.ntp.org address=185.214.143.237 attempt=1/1 time=75.919958ms error="offset -8.77073ms is larger than --max-offset 1ms"
Sun Sep 20 17:30:45 MDT 2026: [ntp] ERROR query failed server=pool.ntp.org address=72.14.182.49 attempt=1/1 time=67.974458ms error="offset +3.503899ms is larger than --max-offset 1ms"
Sun Sep 20 17:30:46 MDT 2026: [ntp] ERROR query failed server=pool.ntp.org address=108.61.215.221 attempt=1/1 time=72.541542ms error="offset +8.316053ms is larger than --max-offset 1ms"
Sun Sep 20 17:30:47 MDT 2026: [ntp] ERROR query failed server=pool.ntp.org address=72.14.186.59 attempt=1/1 time=66.197417ms error="offset +3.618105ms is larger than --max-offset 1ms"
Sun Sep 20 17:30:47 MDT 2026: [ntp] OK done queries=4 answered=0 total_time=4.32928825s
```

With a limit of 1 ms, every answer was further out than that - the public pool's servers differ from each other by several milliseconds - so each query is an `ERROR` and the exit status is `1`. In a script you would allow whatever suits your use; `--max-offset 500` (half a second) is plenty for lining up logs from several machines.

### No server there

```bash
shint ntp 127.0.0.1 --timeout 2
```

```text
Sun Sep 20 17:30:47 MDT 2026: [ntp] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=31.458µs
Sun Sep 20 17:30:48 MDT 2026: [ntp] ERROR query failed server=127.0.0.1 address=127.0.0.1 attempt=1/1 time=821.334µs error="read udp 127.0.0.1:55468->127.0.0.1:123: read: connection refused"
Sun Sep 20 17:30:48 MDT 2026: [ntp] OK done queries=1 answered=0 total_time=1.002439458s
```

Nothing listens on UDP 123 on this machine, so the operating system reported "connection refused". A server that is simply unreachable, or a firewall that drops UDP 123, would run out the `--timeout` instead.

### JSON

```bash
shint ntp time.cloudflare.com --json
```

```json
{
  "input_params": { ... },
  "module_name": "ntp",
  "dns_lookup": { ... },
  "stats": [
    {
      "address": "2606:4700:f1::1",
      "success": true,
      "stratum": 3,
      "version": 4,
      "leap_indicator": "none",
      "reference_id": "10.153.8.152",
      "offset_µs": 187,
      "round_trip_µs": 13307,
      "server_unixtime_µs": 1789947406566820,
      "sent_unixtime_µs": 1789947406558329,
      "recv_unixtime_µs": 1789947406573287,
      "time_taken_µs": 15268
    },
    ...
  ],
  "end_time_unixtime_µs": ...,
  "start_time_unixtime_µs": ...,
  "total_time_taken_µs": ...,
  "error": ""
}
```

`stats` has one entry per address and iteration. `offset_µs` is signed, and `round_trip_µs` is the network delay; `server_unixtime_µs` is the time the server said it was. A failed query has `success: false` and the reason in `error`.

## Exit status

`0` when the name resolved and every query got a trustworthy answer (and, with `--max-offset`, none was out by more than the limit). `1` when the name could not be resolved, a server did not answer within `--timeout`, an answer could not be trusted, or an offset exceeded `--max-offset`. `2` for a bad port, a negative `--max-offset`, or a missing server.

## Good to know

- **`Ctrl+C` shows how far it got.** With a large `--count`, the run stops, prints an `interrupted` line and its `done` line, and exits `1` (cut short); an attempt still in flight is dropped, not counted as a failure - see [Stopping early](usage.md#stopping-early).
- **An answer that cannot be trusted is a failed query, not a result.** That covers: a reply that does not echo this request's timestamp (someone else's packet, or a spoof); a "kiss-o'-death" (stratum 0 with a code such as `RATE` - the server asks you to slow down - or `DENY`); a server that reports itself unsynchronized (leap indicator `3`, or stratum 16); and a reply with no transmit time.
- **Be gentle with public servers.** Many limit how often one address may ask; a `RATE` reply means you asked too often. The default `--delay` of a second already keeps `--count` polite; do not run it in a tight loop.
- **Firewalls** often block outbound UDP 123. If you get timeouts everywhere, that is the first thing to check.
- **This reads the clock, it does not set it.** Use your operating system's time service (`chronyd`, `systemd-timesyncd`, `w32time`, `timed`) for that.
- SNTP asks each server once. It does not do NTP's filtering across many samples, so treat a single reading as a good estimate, and use `--count` to see how much it varies.
