---
title: udp
lead: Send a UDP datagram and classify what comes back - open, closed, or open|filtered - the way a UDP port scan does.
description: shint udp sends a UDP probe to a host and port over IPv4 and IPv6 and reports whether it was answered, refused, or unanswered.
section: Commands
order: 5
nav: udp
---

## What it does

UDP has no handshake, so there is no direct way to ask "is anything listening?". `shint udp` sends one datagram to the host and port and watches for the answer, then reports the best conclusion that evidence allows:

| State | What happened | What it means |
|---|---|---|
| `open` | A reply datagram came back before the timeout. | A service is listening and answered. |
| `closed` | The operating system reported an ICMP "port unreachable". | Nothing is listening on that port. |
| `open|filtered` | Nothing came back before the timeout. | Cannot be told apart: either a service that simply does not reply to input it does not understand, or a firewall silently dropping the packet. |

If the name resolves to several addresses, each one is probed.

## Syntax

```bash
shint udp <host> <port> [--data TEXT | --payload BYTES] [--count N] [--timeout S] [--json]
```

| Flag | Meaning |
|---|---|
| `-D`, `--data TEXT` | The payload to send. Sent exactly as typed - it is text, and backslash escapes such as `\x00` are **not** interpreted. At most 65507 bytes. |
| `--payload N` | If `--data` is not given, send N bytes of filler (default 4), from 0 to 65507 - the most one UDP datagram carries (65535 minus the 8-byte UDP and 20-byte IPv4 headers). A negative or larger value is a usage error (exit `2`); it used to crash. |

The other [shared flags](usage.md#flags-shared-by-every-command) apply; `--timeout` is how long to wait for a reply.

## Examples

### A service that answers

Start an echo listener in another terminal with `shint listen udp 9001 --echo`, then:

```bash
shint udp 127.0.0.1 9001 --data hello
```

```text
Sun Sep 20 01:50:30 MDT 2026: [udp] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=132.541µs
Sun Sep 20 01:50:31 MDT 2026: [udp] OK probe open host=127.0.0.1 port=9001 attempt=1/1 sent=5 received=5 time=962.833µs
Sun Sep 20 01:50:31 MDT 2026: [udp] OK done probes_sent=1 open=1 total_time=1.002634541s
```
`sent=5 received=5`: five bytes went out and the listener echoed them back.

### A closed port

```bash
shint udp 127.0.0.1 9998 --timeout 2
```

```text
Sun Sep 20 01:50:33 MDT 2026: [udp] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=114.625µs
Sun Sep 20 01:50:34 MDT 2026: [udp] ERROR probe closed host=127.0.0.1 port=9998 attempt=1/1 sent=4 received=0 time=1.312875ms
Sun Sep 20 01:50:34 MDT 2026: [udp] OK done probes_sent=1 open=0 total_time=1.002529458s
```
### No answer

Public DNS servers ignore a datagram that is not a valid query, so the probe is unanswered for the whole timeout:

```bash
shint udp 8.8.8.8 53 --timeout 2
```

```text
Sun Sep 20 01:50:34 MDT 2026: [udp] OK dns resolved host=8.8.8.8 addresses=1 ips=[8.8.8.8] time=122µs
Sun Sep 20 01:50:37 MDT 2026: [udp] OK probe open|filtered host=8.8.8.8 port=53 attempt=1/1 sent=4 received=0 time=2.003359834s
Sun Sep 20 01:50:37 MDT 2026: [udp] OK done probes_sent=1 open=0 total_time=3.005550834s
```
`open|filtered` here does not mean the port is closed - only that no reply was seen. To learn more, send a payload the service understands (a real query, for a DNS server).

### JSON

```bash
shint udp 127.0.0.1 9001 --payload 16 --json
```

```json
{
  "input_params": { ... },
  "module_name": "udp",
  "dns_lookup": { ... },
  "stats": [
    {
      "address": "127.0.0.1",
      "port": 9001,
      "state": "open",
      "success": true,
      "bytes_sent": 16,
      "bytes_received": 16,
      "response_preview": "dddddddddddddddd",
      "sent_unixtime_µs": 1789890632670582,
      "recv_unixtime_µs": 1789890632671846,
      "time_taken_µs": 1263
    }
  ],
  "end_time_unixtime_µs": 1789890632671871,
  "start_time_unixtime_µs": 1789890631669283,
  "total_time_taken_µs": 1002588,
  "error": ""
}
```
## Exit status

`0` when the probe was answered or is inconclusive (`open` or `open|filtered`), `1` when the port was reported `closed`, the probe hit an error, or the name could not be resolved.

## Good to know

- **`Ctrl+C` shows the summary.** A run with a large `--count` stops, prints how far it got, the statistics and the `done` line, and exits `1` (cut short) - see [Stopping early](usage.md#stopping-early).
- On networks that drop ICMP, a genuinely closed UDP port looks like `open|filtered`.
- `--payload` is filler; use `--data` when the service expects something specific.
- To scan many UDP ports, run `udp` in a shell loop - `nmap` is TCP only.
