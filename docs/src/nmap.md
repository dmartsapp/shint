---
title: nmap
lead: Find which TCP ports are open on a host, several hosts or a whole subnet, across a range or a list of ports, with live progress so long scans never look stuck.
description: shint nmap scans TCP ports on hosts and subnets over IPv4 and IPv6 - ranges or port lists - names the usual service on each open port, and shows progress on long scans.
section: Commands
order: 5
nav: nmap
---

## What it does

`shint nmap` tries to open a TCP connection to every port you ask about and reports the ones that answer, with the service usually found on each. It is a small scanner for everyday questions ("what is listening on this machine?", "which hosts on this subnet answer SSH?"), not a replacement for the full [nmap](https://nmap.org). Up to 500 ports are tried at the same time, so even wide ranges are practical.

It makes plain TCP connections, the way any program does: no raw packets, so it needs no privileges, and what it finds is what an application on this machine could reach.

Give it one target or several: host names, addresses and prefixes (`192.168.1.0/24`), in any mix. A host name that resolves to several addresses has every address scanned; a prefix has each of its addresses scanned. An address that more than one target leads to is scanned once.

## Syntax

```bash
shint nmap <target>... [--from PORT --to PORT | --ports LIST] [--timeout S] [--json]
```

A target is a host name, an IPv4 or IPv6 address, or a prefix to sweep.

| Flag | Default | Meaning |
|---|---|---|
| `--from` | `1` | First port to scan. |
| `--to` | `80` | Last port to scan (inclusive). |
| `--ports`, `-p` | | The ports to scan as a list, instead of `--from`/`--to`: `22,80,443`, ranges like `8000-8100`, or both (`22,80,8000-8100`). Each port is scanned once, in order, however often the list names it. |
| `--timeout` | `5` | Seconds **each port** is given to answer. It is not a limit on the whole scan. |
| `--count` | `1` | Scan the whole range this many times. |
| `--throttle` | off | Wait a random 0-10 seconds before each port (slow and quiet). |
| `--json` | off | One JSON document with every port's result. |

`--from` and `--to` must be between 1 and 65535, with `--from` not greater than `--to`. `--ports` takes the same ports, as decimal numbers only; it cannot be combined with `--from` or `--to`.

### Prefixes

A prefix is swept: every address in it is scanned. An IPv4 prefix leaves out its network and broadcast addresses, which no host has, so `192.168.1.0/24` is its 254 host addresses, `.1` to `.254` (a `/31` and a `/32` have neither, and every address is scanned). An IPv6 prefix has no broadcast address, and every address in it is scanned.

A prefix with host bits set is read as its network: `192.168.1.77/24` sweeps `192.168.1.0/24`, and the `sweep` line says so with `network=`.

One run sweeps at most **65,536 addresses** - an IPv4 `/16`, or an IPv6 `/112` - across all its prefixes. A wider sweep is a usage error that says how many addresses it would have been; split it into smaller prefixes, or narrow it to the hosts you mean. An IPv6 `/64` holds 2^64 addresses, which no scanner can sweep by trying each one.

`-4` and `-6` apply to prefixes as they do to addresses: an IPv6 prefix with `-4` is a usage error.

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

### A list of ports

```bash
shint nmap 127.0.0.1 --ports 22,80,443,2222,5432,8080-8090
```

```text
Tue Nov 17 22:03:53 CST 2026: [nmap] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=61.333µs
Tue Nov 17 22:03:53 CST 2026: [nmap] OK scan started ports_total=16 timeout=5s max_in_flight=500
Tue Nov 17 22:03:53 CST 2026: [nmap] OK port open host=127.0.0.1 port=5432 service=postgresql
Tue Nov 17 22:03:53 CST 2026: [nmap] OK port open host=127.0.0.1 port=2222
Tue Nov 17 22:03:53 CST 2026: [nmap] OK port open host=127.0.0.1 port=8080 service=http-alt
Tue Nov 17 22:03:53 CST 2026: [nmap] OK scan complete ports_scanned=16 open=3 time=1.730333ms
Tue Nov 17 22:03:53 CST 2026: [nmap] OK done total_time=1.745375ms
```
Each open port shows the service usually found on it (`service=`), when shint has a name for that port: 2222 has none. See [Service names](#service-names).

### Several hosts and a subnet

```bash
shint nmap 127.0.0.0/29 ::1 -p 22,8080 --timeout 1
```

```text
Tue Nov 17 22:03:42 CST 2026: [nmap] OK sweep target=127.0.0.0/29 addresses=6
Tue Nov 17 22:03:42 CST 2026: [nmap] OK dns resolved host=::1 addresses=1 ips=[::1] time=42.833µs
Tue Nov 17 22:03:42 CST 2026: [nmap] OK scan started ports_total=14 hosts=7 timeout=1s max_in_flight=500
Tue Nov 17 22:03:42 CST 2026: [nmap] OK port open host=::1 port=8080 service=http-alt
Tue Nov 17 22:03:42 CST 2026: [nmap] OK port open host=127.0.0.1 port=8080 service=http-alt
Tue Nov 17 22:03:43 CST 2026: [nmap] OK scan complete ports_scanned=14 open=2 hosts_with_open_ports=2 time=1.002719875s
Tue Nov 17 22:03:43 CST 2026: [nmap] OK done total_time=1.003067875s
```
A prefix gets a `sweep` line saying how many addresses it has; a host name or an address is looked up as usual. With more than one address, `scan started` adds `hosts=` (how many addresses are scanned) and `scan complete` adds `hosts_with_open_ports=`. The addresses of `127.0.0.0/29` other than `127.0.0.1` do not exist on this machine, so their ports ran out the one-second timeout.

If a host name cannot be looked up, its `dns resolution failed` line says so and the other targets are still scanned; the run then exits `1`.

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
Every port scanned appears in `stats`, open or not. Each entry also has `target`, the argument its address came from, and `service` for a port shint has a name for:

```bash
shint nmap 127.0.0.1 -p 2222,8080 --json
```

```json
{
  "input_params": {
    ...
    "host": "127.0.0.1",
    "from_port": 2222,
    "to_port": 8080,
    ...
    "ports": "2222,8080"
  },
  "module_name": "nmap",
  "dns_lookup": { ... },
  "stats": [
    {
      "target": "127.0.0.1",
      "address": "127.0.0.1",
      "port": 2222,
      "success": true
    },
    {
      "target": "127.0.0.1",
      "address": "127.0.0.1",
      "port": 8080,
      "service": "http-alt",
      "success": true
    }
  ],
  ...
}
```
With `--ports`, `input_params.ports` is the list, sorted and with runs written as ranges, and `from_port`/`to_port` are its lowest and highest port. A run given more than one target lists them in `input_params.targets` (and `host` is empty) and has `dns_lookups`, one lookup per host name or address, in place of `dns_lookup`; a prefix is swept without a lookup. See [Output formats](output.md#inputparams).

## Service names

The `service` an open port shows is the service usually found on that port number - `ssh` on 22, `https` on 443, `postgresql` on 5432, `consul-http` on 8500 - from a table built into shint. It is a convention, not a check: shint reads nothing from the port, so a web server moved to port 22 still shows `service=ssh`. A port the table has no name for shows none.

The names are the ones people know the services by (`dns` for 53, `rdp` for 3389, rather than IANA's `domain` and `ms-wbt-server`), and the table also knows common unregistered ports: Redis, Docker, Kubernetes, Consul, Prometheus, Elasticsearch, MongoDB and others. A port that several unrelated services commonly use - 3000, 5000, 9000 - is left out rather than guessed at.

## How long will it take?

`--timeout` is what each port gets, so total time depends on **how many ports do not answer**:

| Port behaviour | Cost |
|---|---|
| Open, or actively refused (`connection refused`) | Milliseconds - the host answers at once |
| Silently dropped by a firewall | The full `--timeout`, for every such port |

With up to 500 ports in flight (`max_in_flight` on the `scan started` line says how many; it is lower where the process may open few files - half its descriptor limit), a worst case (a host that drops everything) takes about **ports ÷ 500 × timeout** seconds, where ports is `ports_total` on the `scan started` line - every port of every address:

| Range | `--timeout 5` (default) | `--timeout 1` |
|---|---|---|
| 1-1024 | about 15 s | about 3 s |
| 20-9000 | about 90 s | about 18 s |
| 1-65535 | about 11 min | about 2 min |
| A `/24` (254 addresses), one port | about 5 s | about 1 s |
| A `/24`, 1-1024 | about 43 min | about 9 min |

On a fast, reliable network, `--timeout 1` is usually plenty and makes silent hosts much quicker to scan. Addresses with nothing on them are the silent ones in a sweep, so sweep for a few ports (`-p 22,80,443`) rather than a wide range.

## Reading the results

- **A port listed as open** accepted the connection.
- **A port not listed** was refused, or nothing answered in time. TCP scanning cannot tell a stopped service from a firewall that drops packets - a filtered port and a closed one look the same here.
- The exit status is `0` for a completed scan (whether or not anything was open), and `1` if a lookup failed or the scan was cut short.

## Good to know

- Only TCP is scanned. For UDP, use [`udp`](udp.md), one port at a time.
- The service names are a guess from the port number; to check what a web or TLS port really serves, use [`web`](web.md) or [`tls`](tls.md).
- Scanning hosts you do not own or manage may be against the rules of the network you are on. Scan your own systems, or ones you have permission to test.
