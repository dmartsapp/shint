---
title: nmap
lead: Find which TCP ports are open on a host, across a range you choose, with live progress so long scans never look stuck.
description: shint nmap scans a range of TCP ports on a host over IPv4 and IPv6, reports open ports, and shows progress on long scans.
section: Commands
order: 4
nav: nmap
---

## What it does

`shint nmap` tries to open a TCP connection to every port in a range and reports the ones that answer. It is a small, single-host scanner for everyday questions ("what is listening on this machine?"), not a replacement for the full [nmap](https://nmap.org). Up to 500 ports are tried at the same time, so even wide ranges are practical.

If the host name resolves to several addresses, every address is scanned.

## Syntax

```bash
shint nmap <host> [--from PORT] [--to PORT] [--timeout S] [--json]
```

| Flag | Default | Meaning |
|---|---|---|
| `--from` | `1` | First port to scan. |
| `--to` | `80` | Last port to scan (inclusive). |
| `--timeout` | `5` | Seconds **each port** is given to answer. It is not a limit on the whole scan. |
| `--count` | `1` | Scan the whole range this many times. |
| `--throttle` | off | Wait a random 0-10 seconds before each port (slow and quiet). |
| `--json` | off | One JSON document with every port's result. |

`--from` and `--to` must be between 1 and 65535, with `--from` not greater than `--to`.

## Examples

### A small range

```bash
shint nmap 127.0.0.1 --from 2220 --to 2225
```

```text
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=75.709µs
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK scan started ports_total=6 timeout=5s max_in_flight=500
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK port open host=127.0.0.1 port=2222
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK scan complete ports_scanned=6 open=1 time=705.125µs
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK done total_time=723.167µs
```
Only **open** ports are printed in text mode. The `scan started` line tells you how much is about to happen, and `scan complete` confirms every port was tried.

### A wider range

```bash
shint nmap 127.0.0.1 --from 2000 --to 3000
```

```text
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=53.167µs
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK scan started ports_total=1001 timeout=5s max_in_flight=500
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK port open host=127.0.0.1 port=2500
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK port open host=127.0.0.1 port=2222
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK scan complete ports_scanned=1001 open=2 time=15.927792ms
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK done total_time=15.9805ms
```
Ports are reported as they answer, so they can appear out of order. Use `--json` for a complete, per-port list.

### A host that does not answer

When packets are silently dropped (a firewall in the way), every port has to run out its timeout. The scan prints its size up front and a progress line every three seconds, so you can see it is working:

```bash
shint nmap 192.0.2.1 --from 1 --to 2000 --timeout 2
```

```text
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK dns resolved host=192.0.2.1 addresses=1 ips=[192.0.2.1] time=54.291µs
Sun Sep 20 01:50:37 MDT 2026: [nmap] OK scan started ports_total=2000 timeout=2s max_in_flight=500
Sun Sep 20 01:50:40 MDT 2026: [nmap] OK progress ports_scanned=550/2000 percent=27.5% open=0 in_flight=500 elapsed=3.002s
Sun Sep 20 01:50:43 MDT 2026: [nmap] OK progress ports_scanned=1065/2000 percent=53.2% open=0 in_flight=500 elapsed=6.001s
Sun Sep 20 01:50:45 MDT 2026: [nmap] OK scan complete ports_scanned=2000 open=0 time=8.0430845s
Sun Sep 20 01:50:45 MDT 2026: [nmap] OK done total_time=8.04328325s
```
`in_flight` is how many ports are currently waiting for an answer. A steady `in_flight=500` with a slow count means "waiting on unresponsive ports", not "stuck". Scans that finish before the first three-second tick print no progress lines.

### Stopping early

Press `Ctrl+C` and the scan stops and says how far it got, instead of pretending to be complete:

```text
Sun Sep 20 01:53:06 MDT 2026: [nmap] OK dns resolved host=192.0.2.1 addresses=1 ips=[192.0.2.1] time=82.417µs
Sun Sep 20 01:53:06 MDT 2026: [nmap] OK scan started ports_total=3000 timeout=5s max_in_flight=500
Sun Sep 20 01:53:09 MDT 2026: [nmap] OK progress ports_scanned=0/3000 percent=0.0% open=0 in_flight=500 elapsed=3.002s
Sun Sep 20 01:53:10 MDT 2026: [nmap] ERROR scan interrupted ports_scanned=0 ports_total=3000 open=0 time=4.004428042s
Sun Sep 20 01:53:10 MDT 2026: [nmap] OK done total_time=4.004508667s
```
An interrupted scan exits with status `1`.

### JSON

```bash
shint nmap 127.0.0.1 --from 2220 --to 2224 --json
```

```json
{
  "input_params": { ... },
  "module_name": "nmap",
  "dns_lookup": { ... },
  "stats": [
    {
      "address": "127.0.0.1",
      "port": 2221,
      "success": false
    },
    {
      "address": "127.0.0.1",
      "port": 2224,
      "success": false
    },
    {
      "address": "127.0.0.1",
      "port": 2222,
      "success": true
    },
    {
      "address": "127.0.0.1",
      "port": 2223,
      "success": false
    },
    {
      "address": "127.0.0.1",
      "port": 2220,
      "success": false
    }
  ],
  "end_time_unixtime_µs": 1789890637883939,
  "start_time_unixtime_µs": 1789890637883635,
  "total_time_taken_µs": 304,
  "error": ""
}
```
Every port scanned appears in `stats`, open or not.

## How long will it take?

`--timeout` is what each port gets, so total time depends on **how many ports do not answer**:

| Port behaviour | Cost |
|---|---|
| Open, or actively refused (`connection refused`) | Milliseconds - the host answers at once |
| Silently dropped by a firewall | The full `--timeout`, for every such port |

With up to 500 ports in flight (`max_in_flight` on the `scan started` line says how many; it is lower where the process may open few files - half its descriptor limit), a worst case (a host that drops everything) takes about **ports ÷ 500 × timeout** seconds:

| Range | `--timeout 5` (default) | `--timeout 1` |
|---|---|---|
| 1-1024 | about 15 s | about 3 s |
| 20-9000 | about 90 s | about 18 s |
| 1-65535 | about 11 min | about 2 min |

On a fast, reliable network, `--timeout 1` is usually plenty and makes silent hosts much quicker to scan.

## Reading the results

- **A port listed as open** accepted the connection.
- **A port not listed** was refused, or nothing answered in time. TCP scanning cannot tell a stopped service from a firewall that drops packets - a filtered port and a closed one look the same here.
- The exit status is `0` for a completed scan (whether or not anything was open), and `1` if the lookup failed or the scan was cut short.

## Good to know

- Only TCP is scanned. For UDP, use [`udp`](udp.md), one port at a time.
- Scanning hosts you do not own or manage may be against the rules of the network you are on. Scan your own systems, or ones you have permission to test.
