---
title: ip
lead: What does this machine have? Every network interface and address, with its state, MTU, hardware address and what each address is for - offline, no privileges.
description: shint ip lists this machine's network interfaces with their state, flags, MTU, hardware address and every IPv4 and IPv6 address, offline and without privileges.
section: Commands
order: 10
nav: ip
---

## What it does

When a check behaves strangely the first question is often about *this* machine: which addresses does it have, on which interface, is that interface up, what is its MTU? `shint ip` answers that in shint's usual format, on Linux, macOS and Windows alike, where the tool that does it otherwise differs on each (`ip addr`, `ifconfig`, `ipconfig`).

It reads the operating system's interface table and **sends nothing**: no network is needed and no privileges - it works as a normal user, offline, instantly.

## Syntax

```bash
shint ip [interface] [-4 | -6] [--json]
```

With no argument every interface is listed; with an interface name (`en0`, `eth0`, `lo`; the case does not matter) just that one. `-4` and `-6` list only the addresses of that family. The other [shared flags](usage.md#flags-shared-by-every-command) (`--count`, `--timeout`, `--delay`, ...) mean nothing for a command that never waits, and are ignored.

## What it reports

One `interface` line per interface, followed by one `address` line for each of its addresses:

| Field | Meaning |
|---|---|
| `name`, `index` | The interface's name and the operating system's number for it. |
| `state` | `up` or `down`: the interface's administrative state (what `ip link set up/down` changes). It is not the same as having a cable or a signal. |
| `flags` | What the system says about it, as words: `broadcast`, `loopback`, `pointtopoint`, `multicast`, `running` (there is a carrier and the driver is active). `up` is not repeated here. |
| `mtu` | The largest packet the interface carries, in bytes. |
| `mac` | The hardware address; absent for interfaces that have none (loopback, tunnels). |
| `addresses` | How many addresses are listed for it (after `-4`/`-6`). |
| `address` | The address on its own, and `prefix`: the same address with its configured prefix length, the way you would write it in a configuration (`192.168.1.20/24`). |
| `family` | `ipv4` or `ipv6`. |
| `kind` | What the address is for: `loopback`, `private` (RFC 1918 and IPv6 unique-local), `link-local`, `multicast`, `shared` (carrier-grade NAT), `documentation`, `global` and the rest - the same words `shint cidr` uses. `global` means none of the special ranges; it is not a promise that the address is reachable from the internet. |

## Examples

### Everything

```bash
shint ip
```

```text
Mon Sep 21 11:41:23 MDT 2026: [ip] OK interface name=lo0 index=1 state=up flags=[loopback,multicast,running] mtu=16384 addresses=3
Mon Sep 21 11:41:23 MDT 2026: [ip] OK address interface=lo0 address=127.0.0.1 prefix=127.0.0.1/8 family=ipv4 kind=loopback
Mon Sep 21 11:41:23 MDT 2026: [ip] OK address interface=lo0 address=::1 prefix=::1/128 family=ipv6 kind=loopback
Mon Sep 21 11:41:23 MDT 2026: [ip] OK address interface=lo0 address=fe80::1 prefix=fe80::1/64 family=ipv6 kind=link-local
Mon Sep 21 11:41:23 MDT 2026: [ip] OK interface name=en0 index=15 state=up flags=[broadcast,multicast,running] mtu=1500 mac=aa:bb:cc:dd:ee:ff addresses=3
Mon Sep 21 11:41:23 MDT 2026: [ip] OK address interface=en0 address=192.168.1.20 prefix=192.168.1.20/24 family=ipv4 kind=private
Mon Sep 21 11:41:23 MDT 2026: [ip] OK address interface=en0 address=fe80::1c2d:3e4f:5a6b:7c8d prefix=fe80::1c2d:3e4f:5a6b:7c8d/64 family=ipv6 kind=link-local
Mon Sep 21 11:41:23 MDT 2026: [ip] OK address interface=en0 address=2600:1700:abcd:1234::20 prefix=2600:1700:abcd:1234::20/64 family=ipv6 kind=global
Mon Sep 21 11:41:23 MDT 2026: [ip] OK interface name=utun3 index=20 state=down flags=[pointtopoint,multicast] mtu=1380 addresses=0
Mon Sep 21 11:41:23 MDT 2026: [ip] OK done interfaces=3 addresses=6 total_time=805.792µs
```

Read it top to bottom: the loopback interface with its three addresses; `en0`, the real network card, with a private IPv4 address and two IPv6 ones; and a tunnel that is `down` and has no address at all. An interface with no addresses is still listed - knowing it exists and is down is useful.

### One interface, one family

```bash
shint ip en0 -4
```

```text
Mon Sep 21 11:42:10 MDT 2026: [ip] OK interface name=en0 index=15 state=up flags=[broadcast,multicast,running] mtu=1500 mac=aa:bb:cc:dd:ee:ff addresses=1
Mon Sep 21 11:42:10 MDT 2026: [ip] OK address interface=en0 address=192.168.1.20 prefix=192.168.1.20/24 family=ipv4 kind=private
Mon Sep 21 11:42:10 MDT 2026: [ip] OK done interfaces=1 addresses=1 total_time=612.375µs
```

`-4` filters the *addresses*, not the interfaces: an interface that only has IPv6 addresses is still listed, with `addresses=0`.

### An interface that does not exist

```bash
shint ip wlan9
```

```text
Mon Sep 21 11:42:31 MDT 2026: [ip] ERROR failed error="no such interface wlan9 (available: lo0, en0, utun3)"
```

The exit status is `1`, and the message names the interfaces that do exist, which is usually what you wanted to know. The name is data, and what is available depends on the machine, so this is a failed check rather than a [usage error](#exit-status).

### JSON

```bash
shint ip en0 --json
```

```json
{
  "input_params": { ... },
  "module_name": "ip",
  "stats": [
    {
      "name": "en0",
      "index": 15,
      "state": "up",
      "flags": ["broadcast", "multicast", "running"],
      "mtu": 1500,
      "mac": "aa:bb:cc:dd:ee:ff",
      "addresses": [
        {
          "address": "192.168.1.20",
          "prefix": "192.168.1.20/24",
          "prefix_length": 24,
          "family": "ipv4",
          "kind": "private"
        }
      ]
    }
  ],
  "end_time_unixtime_µs": ...,
  "start_time_unixtime_µs": ...,
  "total_time_taken_µs": ...,
  "error": ""
}
```

Like [`cidr`](cidr.md), the document has no `dns_lookup`: nothing is looked up. `input_params.data` holds the interface name you asked for (empty for all). `addresses` is always a list, empty rather than `null` for an interface with none; `mac` is left out when there is none. When the run fails, `error` says why and `stats` holds whatever could be listed.

## Exit status

`0` when at least one interface was listed. `1` when the named interface does not exist, the interface table could not be read, or the addresses of an interface could not be read (that interface is reported with an `ERROR` line and the others are still listed). `2` for using it wrongly: `-4` and `-6` together, or more than one interface name.

## Good to know

- **Addresses are shown as configured**: `192.168.1.20/24` is the address *with* its prefix length, not the network. To work out the network, mask and range of a prefix, use [`cidr`](cidr.md).
- **IPv6 link-local addresses** (`fe80::...`) are shown without the `%en0` zone: the interface name is on the line above.
- **Names differ by platform** (`eth0` or `enp3s0` on Linux, `en0` on macOS, `Ethernet` on Windows) and this lists whatever the system calls them. The match is case-insensitive.
- **It says nothing about routes.** Which interface a packet to a given destination would leave by, and the default gateway, are the routing table - planned as `ip route`.
- **It does not change anything.** There is no way to add an address or bring an interface up or down with shint, and there never will be: it needs privileges and shint never does.
