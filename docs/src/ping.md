---
title: ping
lead: Send ICMP echo requests to see whether a host is reachable and how long it takes to answer, over IPv4 and IPv6.
description: shint ping sends ICMP echo requests to a host over IPv4 and IPv6 and reports latency, loss and standard deviation.
section: Commands
order: 2
nav: ping
---

## What it does

`shint ping` sends ICMP echo requests (the same as the classic `ping`) and reports the round-trip time of each reply. A name that resolves to both IPv4 and IPv6 addresses is pinged over **both** in the same run, and each reply is labelled `(ipv4)` or `(ipv6)`.

## Syntax

```bash
shint ping <host> [--count N] [--delay MS] [--payload BYTES] [--throttle] [--json]
```

The [shared flags](usage.md#flags-shared-by-every-command) apply. `--timeout` is how long each echo request waits for its reply before it counts as lost (default 5 seconds). It does not cover the name lookup, which keeps the ping library's own fixed 5-second limit. `--payload` must be between 0 and 1448 bytes.

## Examples

### Ping the local machine

```bash
shint ping 127.0.0.1 --count 2 --delay 500
```

```text
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=20.375µs
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK received reply for request #1 from 127.0.0.1 (ipv4) in 0ms bytes=4
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK received reply for request #2 from 127.0.0.1 (ipv4) in 0ms bytes=4

========================================= icmp STATISTICS =========================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 0s, average: 0s, maximum: 0s
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK done packets_lost=0 stddev_ms=0.000 resolve_time=20.375µs total_time=501.641667ms
```
### A dual-stack host

Every iteration pings every address:

```bash
shint ping google.com --count 2 --delay 500
```

```text
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.45.142] time=3.208084ms
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK dns authoritative zone=google.com nameservers=[ns1.google.com,ns2.google.com,ns3.google.com,ns4.google.com] time=41.446ms
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK received reply for request #1 from 142.251.45.142 (ipv4) in 27ms bytes=4
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK received reply for request #1 from 2607:f8b0:400a:803::200e (ipv6) in 32ms bytes=4
Sun Sep 20 15:37:49 MDT 2026: [icmp] OK received reply for request #2 from 2607:f8b0:400a:803::200e (ipv6) in 28ms bytes=4
Sun Sep 20 15:37:49 MDT 2026: [icmp] OK received reply for request #2 from 142.251.45.142 (ipv4) in 29ms bytes=4

========================================= icmp STATISTICS =========================================
Requests sent: 4, Response received: 4, Success: 100%
Latency: minimum: 27ms, average: 29ms, maximum: 32ms
Sun Sep 20 15:37:49 MDT 2026: [icmp] OK done packets_lost=0 stddev_ms=1.871 resolve_time=3.208084ms total_time=562.66075ms
```
### JSON

```bash
shint ping 127.0.0.1 --json
```

```json
{
  "input_params": { ... },
  "module_name": "icmp",
  "dns_lookup": { ... },
  "stats": [
    {
      "address": "127.0.0.1",
      "success": true,
      "sequence": 1,
      "payload_size_bytes": 4,
      "recv_unixtime_ms": 1789940269446,
      "sent_unixtime_ms": 1789940269446,
      "time_taken_ms": 0
    }
  ],
  "end_time_unixtime_µs": 1789940269446986,
  "start_time_unixtime_µs": 1789940269446342,
  "total_time_taken_µs": 644,
  "error": ""
}
```
Ping times in JSON are in **milliseconds**, unlike the other commands, which use microseconds.

### A host that does not answer

A request that gets no reply counts as lost once `--timeout` has passed - here 2 seconds, so the run takes about 2 seconds:

```bash
shint ping 192.0.2.1 --count 1 --delay 0 --timeout 2
```

```text
Sun Sep 20 10:27:13 MDT 2026: [icmp] OK dns resolved host=192.0.2.1 addresses=1 ips=[192.0.2.1] time=92.917µs
Sun Sep 20 10:27:15 MDT 2026: [icmp] ERROR no reply for request #1 from 192.0.2.1: read udp 0.0.0.0:0: raw-read udp 0.0.0.0:0: i/o timeout

========================================= icmp STATISTICS =========================================
Requests sent: 1, Response received: 0
Latency: minimum: 0, average: 0, maximum: 0
Sun Sep 20 10:27:15 MDT 2026: [icmp] OK done packets_lost=1 stddev_ms=0.000 resolve_time=92.917µs total_time=2.002622083s
```

The lost request is logged as an `ERROR` line, like any other failed check, and the exit status is `1`. (`192.0.2.1` is a reserved documentation address that never answers.)

## Reading the results

- **`received reply ... in 29ms bytes=4`** - one echo request was answered; the time is the round trip, and `bytes` is the size of the echo payload (see below).
- **`bytes=N`** is the **payload size** of the echo: what `--payload` sets (default 4). It does not count the 8-byte ICMP header or the IP header, so it reads like Windows `ping` (`bytes=32`); Unix `ping` prints payload + 8, so its familiar `64 bytes` is `--payload 56` here. shint shows the size it sent: the ping library does not report the length of the reply it read, but RFC 792 requires an echo reply to return the request's data unchanged, so the two are the same. A lost request has no reply, so it shows no size.
- **Statistics block** - requests sent, replies received, and minimum, average and maximum latency.
- **`packets_lost`, `stddev_ms`** - on the final line: how many requests went unanswered and how much the latency varied (jitter).

The exit status is `0` only if every echo request was answered, and `1` if any went unanswered or the name could not be resolved.

## Permissions

ICMP normally needs privileges, but shint uses the unprivileged ICMP sockets that modern systems offer:

- **macOS, the BSDs and Windows:** works for a normal user.
- **Linux:** depends on the `net.ipv4.ping_group_range` setting. If `ping` reports a permission error, run it as root or widen the range with `sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"`.
- **Docker:** the same rules apply as on the host.

## Good to know

- The size of each request's payload is `--payload` (default 4 bytes). It can be at most 1448 bytes, so an echo request never has to fragment; a larger or negative value is a usage error (exit status `2`) rather than being quietly changed.
- Some networks block ICMP entirely. A failed ping does not always mean a host is down - try [`telnet`](telnet.md) against a port you know should be open.
