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
shint udp <host> <port> [--data TEXT | --hex BYTES | --payload N] [--count N] [--timeout S] [--json]
```

| Flag | Meaning |
|---|---|
| `-D`, `--data TEXT` | The payload to send. Sent exactly as typed - it is text, and backslash escapes such as `\x00` are **not** interpreted. At most 65507 bytes. |
| `--hex BYTES` | The payload to send as **exact bytes**, written as hex digits: `00010203ff`, `00 01 02 03 ff` and `00:01:02:03:FF` are the same five bytes (either case; spaces and colons are ignored). This is how to send a binary probe - a DNS or STUN request, say - which `--data` cannot: a `\x00` typed there reaches shint as four ordinary characters. At least one byte and at most 65507; an odd number of digits, a character that is not a hex digit (a `0x` prefix included) or an empty value is a usage error (exit `2`). Cannot be combined with `--data`. |
| `--payload N` | If neither `--data` nor `--hex` is given, send N bytes of filler (default 4), from 0 to 65507 - the most one UDP datagram carries (65535 minus the 8-byte UDP and 20-byte IPv4 headers). A negative or larger value is a usage error (exit `2`); it used to crash. |

`--payload` is ignored when `--data` or `--hex` is given. The log line and the JSON show how many bytes went out (`sent=`, `bytes_sent`). The other [shared flags](usage.md#flags-shared-by-every-command) apply; `--timeout` is how long to wait for a reply.

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
`open|filtered` here does not mean the port is closed - only that no reply was seen. To learn more, send a payload the service understands (a real query, for a DNS server) - see the next example.

### A binary payload

`--hex` sends exact bytes. This is a real DNS query for `example.com` (an A record, header `1234 0100 0001 0000 0000 0000`, then the name as length-prefixed labels, then type and class):

```bash
shint udp 8.8.8.8 53 --hex "1234 0100 0001 0000 0000 0000 07 6578616d706c65 03 636f6d 00 0001 0001" --timeout 3
```

```text
Mon Sep 21 14:47:05 MDT 2026: [udp] OK dns resolved host=8.8.8.8 addresses=1 ips=[8.8.8.8] time=34µs
Mon Sep 21 14:47:06 MDT 2026: [udp] OK probe open host=8.8.8.8 port=53 attempt=1/1 sent=29 received=61 time=38.178875ms
Mon Sep 21 14:47:06 MDT 2026: [udp] OK done probes_sent=1 open=1 total_time=1.040096042s
```

29 bytes went out and the server answered with 61: this time it is `open`, where the four filler bytes above got no answer. With `--json` the reply is in `response_preview`, escaped so a binary reply cannot garble the terminal - `\x81\x80` are the header flags of a successful answer, and the name and the two addresses are in there. (Bytes that happen to be valid text, such as `example`, are shown as text; everything else as `\xNN`.) To read a DNS answer properly use [`shint dns`](dns.md); this is for protocols shint has no command for.

The same against a local listener, to see exactly what arrives (`shint listen udp 9001 --echo`):

```bash
shint udp 127.0.0.1 9001 --hex "00 01 02 ff"
```

```text
Mon Sep 21 14:46:59 MDT 2026: [udp] OK probe open host=127.0.0.1 port=9001 attempt=1/1 sent=4 received=4 time=971.625µs
```

The listener reports `bytes=4 preview=\x00\x01\x02\xff`.

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
