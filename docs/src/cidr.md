---
title: cidr
lead: Subnet arithmetic without a calculator - network, mask, range, size and kind, for IPv4 and IPv6, entirely offline.
description: shint cidr works out the network address, netmask, wildcard, first and last address, broadcast, host counts and kind of one or more IPv4 or IPv6 prefixes, without touching the network.
section: Commands
order: 9
nav: cidr
---

## What it does

`shint cidr` takes one or more prefixes in CIDR notation (`192.168.1.10/24`, `2001:db8::/32`) and works out everything you usually reach for a subnet calculator for. It is **pure computation**: nothing is looked up and nothing is sent, so it works anywhere, instantly, and needs no network.

A bare address (`8.8.8.8`, `::1`) is treated as a single host: `/32` for IPv4, `/128` for IPv6. A prefix does not have to be written with its host bits cleared: `192.168.1.10/24` is the `192.168.1.0/24` network.

## Syntax

```bash
shint cidr <prefix> [<prefix> ...] [--json]
```

Only `--json` applies. The other [shared flags](usage.md#flags-shared-by-every-command) (`--count`, `--timeout`, `--delay`, ...) mean nothing for a command that never waits, and are ignored.

All the prefixes are checked before anything is printed, so one typo in the third is a [usage error](#exit-status), not two results followed by a failure.

## What it reports

| Field | Meaning |
|---|---|
| `input` | The prefix as you typed it. |
| `network` | The network, with the host bits cleared. |
| `family` | `ipv4` or `ipv6`. |
| `netmask` | The mask as an address (`255.255.255.0`, `ffff:ffff::`). |
| `wildcard` | The inverse of the netmask - what routers and ACLs sometimes want. **IPv4 only.** |
| `first`, `last` (JSON: `first_address`, `last_address`) | The first and last address of the range. |
| `broadcast` | The broadcast address. **IPv4 only, and only for a range that has one** (not `/31` or `/32`). |
| `addresses` | How many addresses the range holds. |
| `hosts` (JSON: `usable_hosts`) | How many of them can be given to a machine (see below). |
| `first_host`, `last_host` | The first and last of those. |
| `kind` | What sort of range the *network address* falls in: `private`, `loopback`, `link-local`, `multicast`, `documentation`, `shared` (carrier-grade NAT, `100.64.0.0/10`), `reserved`, `broadcast`, `unspecified`, or `global` - none of those. |

How the host count works:

| Prefix | `hosts` | Why |
|---|---|---|
| IPv4 `/0` to `/30` | `addresses - 2` | The first address is the network, the last the broadcast. |
| IPv4 `/31` | `2` | Both addresses are hosts on a point-to-point link ([RFC 3021](https://www.rfc-editor.org/rfc/rfc3021)). |
| IPv4 `/32` | `1` | A single host. |
| IPv6, any | `addresses` | IPv6 has no broadcast address; every address in the prefix can be assigned. |

## Examples

### A typical subnet

```bash
shint cidr 192.168.1.10/24
```

```text
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK subnet input=192.168.1.10/24 network=192.168.1.0/24 family=ipv4 netmask=255.255.255.0 wildcard=0.0.0.255 first=192.168.1.0 last=192.168.1.255 broadcast=192.168.1.255 addresses=256 hosts=254 first_host=192.168.1.1 last_host=192.168.1.254 kind=private
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK done prefixes=1 total_time=750.5µs
```

The address you gave (`.10`) is somewhere in the network; `network=192.168.1.0/24` is the network itself, and `hosts=254` because two of the 256 addresses are the network and the broadcast.

### Several prefixes, both families

```bash
shint cidr 10.0.0.0/8 2001:db8::/32
```

```text
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK subnet input=10.0.0.0/8 network=10.0.0.0/8 family=ipv4 netmask=255.0.0.0 wildcard=0.255.255.255 first=10.0.0.0 last=10.255.255.255 broadcast=10.255.255.255 addresses=16777216 hosts=16777214 first_host=10.0.0.1 last_host=10.255.255.254 kind=private
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK subnet input=2001:db8::/32 network=2001:db8::/32 family=ipv6 netmask=ffff:ffff:: first=2001:db8:: last=2001:db8:ffff:ffff:ffff:ffff:ffff:ffff addresses=79228162514264337593543950336 hosts=79228162514264337593543950336 first_host=2001:db8:: last_host=2001:db8:ffff:ffff:ffff:ffff:ffff:ffff kind=documentation
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK done prefixes=2 total_time=205.375µs
```

An IPv6 prefix is enormous: a `/32` holds 79,228,162,514,264,337,593,543,950,336 addresses (2^96). The count is exact - it is computed with arbitrary-precision integers, not floating point.

### Small prefixes

```bash
shint cidr 192.0.2.0/31 8.8.8.8
```

```text
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK subnet input=192.0.2.0/31 network=192.0.2.0/31 family=ipv4 netmask=255.255.255.254 wildcard=0.0.0.1 first=192.0.2.0 last=192.0.2.1 addresses=2 hosts=2 first_host=192.0.2.0 last_host=192.0.2.1 kind=documentation
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK subnet input=8.8.8.8 network=8.8.8.8/32 family=ipv4 netmask=255.255.255.255 wildcard=0.0.0.0 first=8.8.8.8 last=8.8.8.8 addresses=1 hosts=1 first_host=8.8.8.8 last_host=8.8.8.8 kind=global
Sun Sep 20 17:30:33 MDT 2026: [cidr] OK done prefixes=2 total_time=175.75µs
```

A `/31` has no broadcast address and both of its addresses are hosts; a bare address is a `/32`.

### JSON

```bash
shint cidr 172.16.5.130/26 10.0.0.0/8 --json
```

```json
{
  "input_params": { ... },
  "module_name": "cidr",
  "stats": [
    {
      "input": "172.16.5.130/26",
      "network": "172.16.5.128/26",
      "family": "ipv4",
      "prefix_length": 26,
      "netmask": "255.255.255.192",
      "wildcard": "0.0.0.63",
      "first_address": "172.16.5.128",
      "last_address": "172.16.5.191",
      "broadcast": "172.16.5.191",
      "addresses": "64",
      "usable_hosts": "62",
      "first_host": "172.16.5.129",
      "last_host": "172.16.5.190",
      "kind": "private"
    }
  ],
  "end_time_unixtime_µs": 1789947033226617,
  "start_time_unixtime_µs": 1789947033226581,
  "total_time_taken_µs": 36,
  "error": ""
}
```

The document has no `dns_lookup`: nothing is looked up, and an empty lookup would only read as a failed one. `stats` has one entry per prefix, in the order given (one is shown). `addresses` and `usable_hosts` are **strings**, because an IPv6 prefix can hold more addresses than a 64-bit integer, and JSON tools would silently round them.

### Bad input

```bash
shint cidr 10.0.0.0/33
```

```text
invalid prefix "10.0.0.0/33": prefix length out of range
```

This goes to standard error and the exit status is `2`.

## Exit status

`0` when every prefix was understood and worked out, `2` when any of them is not a valid address or prefix (nothing is printed to standard output in that case). There is no `1`: a subnet calculation cannot fail once the input has parsed.

## Good to know

- `kind` describes the network address only. A prefix that spans several kinds (`0.0.0.0/0` covers everything) reports the kind of its first address.
- `global` means "none of the special kinds above" - it is not a promise that the range is routable on the internet.
- An IPv6 address with a zone (`fe80::1%eth0`) is refused: a zone names an interface on one machine, not a range of addresses.
- Prefixes are read strictly: `10.0.0/24` (three octets) or `10.0.0.0/24/8` are errors, not guesses.
