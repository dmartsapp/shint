---
title: ip
lead: What does this machine have, and where does its traffic go? Its interfaces and addresses, its routing table, and its neighbour table - offline, no privileges.
description: shint ip lists this machine's network interfaces and addresses; shint ip route its routing table, or which way traffic to an address goes; shint ip arp its neighbour table (ARP and NDP) - offline and without privileges.
section: Commands
order: 12
nav: ip
---

## What it does

When a check behaves strangely the first question is often about *this* machine. `shint ip` answers it in shint's usual format, on Linux, macOS and Windows alike, where the tools that do it otherwise differ on each:

| Command | Question | Instead of |
|---|---|---|
| `shint ip [interface]` | Which interfaces and addresses does this machine have? Is the interface up, what is its MTU? | `ip addr`, `ifconfig`, `ipconfig` |
| [`shint ip route [address]`](#routes-ip-route) | Where does this machine's traffic go: its routing table and default gateway - or which way traffic to one address goes | `ip route`, `netstat -rn`, `route print` |
| [`shint ip arp [interface]`](#neighbours-ip-arp) | Which hardware addresses has it learned for the IPs on its networks (ARP and NDP)? | `arp -an`, `ip neigh`, `Get-NetNeighbor` |

All three read the operating system's tables and **send nothing**: no network is needed and no privileges - they work as a normal user, offline, instantly. Nothing is looked up either: addresses are shown as they are.

## Syntax

```bash
shint ip [interface] [-4 | -6] [--json]
```

With no argument every interface is listed; with an interface name (`en0`, `eth0`, `lo`; the case does not matter) just that one. `-4` and `-6` list only the addresses of that family. The other [shared flags](usage.md#flags-shared-by-every-command) (`--count`, `--timeout`, `--delay`, ...) mean nothing for a command that never waits, and are ignored.

## What it reports

One `interface` line per interface - everything about it in one place:

| Field | Meaning |
|---|---|
| `name` | The interface's name. |
| `state` | `up` or `down`: the interface's administrative state (what `ip link set up/down` changes). It is not the same as having a cable or a signal. |
| `ipv4`, `ipv6` | Its addresses of that family, each with its configured prefix length, the way you would write it in a configuration (`192.168.1.20/24`). Left out when the interface has none (or when `-4`/`-6` leaves that family out). |
| `mac` | The hardware address; absent for interfaces that have none (loopback, tunnels). |
| `mtu` | The largest packet the interface carries, in bytes. |
| `flags` | What the system says about it, as words: `broadcast`, `loopback`, `pointtopoint`, `multicast`, `running` (there is a carrier and the driver is active). `up` is not repeated here. |

Then a `done` line: how many interfaces and how many addresses were listed.

## Examples

### Everything

```bash
shint ip
```

```text
Mon Sep 21 11:41:23 MDT 2026: [ip] OK interface name=lo0 state=up ipv4=[127.0.0.1/8] ipv6=[::1/128,fe80::1/64] mtu=16384 flags=[loopback,multicast,running]
Mon Sep 21 11:41:23 MDT 2026: [ip] OK interface name=en0 state=up ipv4=[192.168.1.20/24] ipv6=[fe80::1c2d:3e4f:5a6b:7c8d/64,2600:1700:abcd:1234::20/64] mac=aa:bb:cc:dd:ee:ff mtu=1500 flags=[broadcast,multicast,running]
Mon Sep 21 11:41:23 MDT 2026: [ip] OK interface name=utun3 state=down mtu=1380 flags=[pointtopoint,multicast]
Mon Sep 21 11:41:23 MDT 2026: [ip] OK done interfaces=3 addresses=6 total_time=805.792µs
```

Read it line by line: the loopback interface with its three addresses; `en0`, the real network card, with a private IPv4 address and two IPv6 ones; and a tunnel that is `down` and has no address at all. An interface with no addresses is still listed, without `ipv4` or `ipv6`.

### One interface, one family

```bash
shint ip en0 -4
```

```text
Mon Sep 21 11:42:10 MDT 2026: [ip] OK interface name=en0 state=up ipv4=[192.168.1.20/24] mac=aa:bb:cc:dd:ee:ff mtu=1500 flags=[broadcast,multicast,running]
Mon Sep 21 11:42:10 MDT 2026: [ip] OK done interfaces=1 addresses=1 total_time=612.375µs
```

`-4` filters the *addresses*, not the interfaces: an interface that only has IPv6 addresses is still listed, without an `ipv4` field.

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
      "state": "up",
      "ipv4": ["192.168.1.20/24"],
      "ipv6": ["fe80::1c2d:3e4f:5a6b:7c8d/64", "2600:1700:abcd:1234::20/64"],
      "mac": "aa:bb:cc:dd:ee:ff",
      "mtu": 1500,
      "flags": ["broadcast", "multicast", "running"]
    }
  ],
  "end_time_unixtime_µs": ...,
  "start_time_unixtime_µs": ...,
  "total_time_taken_µs": ...,
  "error": ""
}
```

One object per interface, and nothing nested in it: the addresses are plain strings, so `jq -r '.stats[] | select(.state=="up") | .ipv4[]'` is all it takes to list the IPv4 addresses of the interfaces that are up. Like [`cidr`](cidr.md), the document has no `dns_lookup`: nothing is looked up. `input_params.data` holds the interface name you asked for (empty for all). `ipv4` and `ipv6` are always lists, empty rather than `null` for an interface with none; `mac` is left out when there is none. When the run fails, `error` says why and `stats` holds whatever could be listed.

## Exit status

- **`ip`**: `0` when at least one interface was listed. `1` when the named interface does not exist, the interface table could not be read, or the addresses of an interface could not be read (that interface is reported with an `ERROR` line and the others are still listed). `2` for using it wrongly: `-4` and `-6` together, or more than one interface name.
- **`ip route`**: `0` when it listed routes, or found the way to the address. `1` when there is no route to the address, the table could not be read, or the system is not supported yet. `2` for a name or anything else that is not an IP address, an address of the family `-4`/`-6` excludes, or more than one argument.
- **`ip arp`**: `0` when at least one neighbour was listed. `1` when the named interface does not exist, no neighbour is known (on it), the table could not be read, or the system is not supported yet. `2` for `-4` and `-6` together, or more than one interface name.

## Good to know

- **Addresses are shown as configured**: `192.168.1.20/24` is the address *with* its prefix length, not the network. To work out the network, mask and range of a prefix, use [`cidr`](cidr.md).
- **IPv6 link-local addresses** (`fe80::...`) are shown without the `%en0` zone: the interface name is on the same line.
- **What kind of address is it?** `ip` lists what is configured and does not classify it. For private, link-local, global and the rest, ask [`cidr`](cidr.md): `shint cidr 192.168.1.20`.
- **Names differ by platform** (`eth0` or `enp3s0` on Linux, `en0` on macOS, `Ethernet` on Windows) and this lists whatever the system calls them. The match is case-insensitive.
- **Routes and neighbours** are the subcommands below: [`ip route`](#routes-ip-route) and [`ip arp`](#neighbours-ip-arp). An interface literally named `route`, `arp` or `neigh` cannot be given to `shint ip`: the subcommand wins.
- **It does not change anything.** There is no way to add an address or bring an interface up or down with shint, and there never will be: it needs privileges and shint never does.

## Routes: ip route

```bash
shint ip route [address] [-4 | -6] [--json]
```

With no argument, `ip route` lists the routing table, one `route` line per route; the default routes come first:

```text
Wed Nov 18 09:19:41 CST 2026: [ip-route] OK route destination=default family=ipv4 gateway=192.168.1.254 interface=en0
Wed Nov 18 09:19:41 CST 2026: [ip-route] OK route destination=default family=ipv4 interface=utun4 scoped=true
Wed Nov 18 09:19:41 CST 2026: [ip-route] OK route destination=127.0.0.0/8 family=ipv4 interface=lo0
Wed Nov 18 09:19:41 CST 2026: [ip-route] OK route destination=192.168.1.0/24 family=ipv4 interface=en0
Wed Nov 18 09:19:41 CST 2026: [ip-route] OK route destination=default family=ipv6 gateway=fe80::62d8:a4ff:fe35:8b04%en0 interface=en0
Wed Nov 18 09:19:41 CST 2026: [ip-route] OK done routes=5 ipv4=4 ipv6=1 total_time=1.013458ms
```

| Field | Meaning |
|---|---|
| `destination` | The prefix the route covers; `default` for `0.0.0.0/0` or `::/0`. |
| `family` | `ipv4` or `ipv6`. |
| `gateway` | The next hop. Absent for a network reached directly on the interface. |
| `interface` | Where the traffic leaves. |
| `metric` | The route's metric, where the system keeps one (Linux, Windows). |
| `scoped` | macOS: the route is used only by traffic bound to its interface - a VPN's own default route, say. The unscoped default is the one everything else takes. |
| `unreachable` | The route drops what it matches (a reject or blackhole route). |

What is listed is the table of routes, as `ip route` shows it: macOS's ARP cache entries and cloned host routes, multicast and broadcast routes, and Linux's local-address table are left out (the neighbours are [`ip arp`](#neighbours-ip-arp)).

### Which way traffic goes

Give an address, and `ip route` says which way traffic to it would go:

```text
$ shint ip route 8.8.8.8
Wed Nov 18 09:19:41 CST 2026: [ip-route] OK route destination=8.8.8.8 route=0.0.0.0/0 gateway=192.168.1.254 interface=en0 source=192.168.1.204
```

`route` is the routing-table entry that matches it - the longest prefix - and `gateway` its next hop. `interface` and `source` are what the system itself picks, found by connecting a UDP socket, which sends nothing - so they are right even where policy routing is in play. Only a route on that interface is shown: when no listed route covers it (Linux keeps loopback and local addresses in a table of its own), the line has the interface and source and no route, rather than another interface's gateway. An address with no way there at all is a failed check: `no route to 10.66.1.1: network is unreachable`.

It takes an **address, never a name** - nothing is looked up; `shint dns` finds a name's addresses. A name is a usage error (exit `2`), and so is an IPv6 address with `-4` (or the other way round).

## Neighbours: ip arp

```bash
shint ip arp [interface] [-4 | -6] [--json]     # ip neigh is the same command
```

The neighbour table: the hardware (MAC) address this machine has learned for each IP address on its local networks - ARP for IPv4, NDP for IPv6. One `neighbour` line each:

```text
Wed Nov 18 09:22:38 CST 2026: [ip-arp] OK neighbour address=192.168.1.42 family=ipv4 mac=08:27:a8:20:71:52 interface=en0 state=STALE
Wed Nov 18 09:22:38 CST 2026: [ip-arp] OK neighbour address=192.168.1.77 family=ipv4 interface=en0 state=FAILED
Wed Nov 18 09:22:38 CST 2026: [ip-arp] OK neighbour address=192.168.1.254 family=ipv4 mac=60:d8:a4:35:8b:04 interface=en0 state=REACHABLE gateway=true
Wed Nov 18 09:22:38 CST 2026: [ip-arp] OK done neighbours=3 ipv4=3 ipv6=0 total_time=1.127936ms
```

| Field | Meaning |
|---|---|
| `address` | The neighbour's IP address, as it is: never looked up, so the list is instant and works when DNS does not (`shint rdns` finds a name for one). |
| `mac` | The hardware address learned for it. **Absent** while it is being resolved and when resolving failed - often the answer to "why can't I reach that host on my own network?". |
| `interface` | The network it is on. |
| `state` | Where the system reports it (Linux, Windows; not macOS or the BSDs): `REACHABLE`, `STALE`, `DELAY`, `PROBE`, `INCOMPLETE`, `FAILED`, `PERMANENT` (Windows says `UNREACHABLE` where Linux says `FAILED`). |
| `router` | An IPv6 neighbour that announced itself as a router. |
| `gateway` | The neighbour is the next hop of a default route - the first thing to check when nothing outside the local network answers. |

Give an interface name to list only its neighbours. A name that does not exist is a failed check that names the ones that do - the same answer `shint ip <name>` and `--interface` give:

```text
Wed Nov 18 09:22:38 CST 2026: [ip-arp] ERROR failed error="no such interface wlan9 (available: lo0, en0, utun3)"
```

The machine's own addresses, multicast and broadcast entries are not neighbours, and are left out.

## Platforms

| | `ip` | `ip route` | `ip arp` |
|---|---|---|---|
| Linux (and Android, where apps may read the tables) | yes | yes (`/proc`) | yes (netlink) |
| macOS, FreeBSD, NetBSD, OpenBSD | yes | yes (the routing socket) | yes (the routing socket) |
| Windows | yes | yes | yes |
| Solaris, illumos, AIX | yes | not supported yet (exit `1`) | not supported yet (exit `1`) |

The Windows tables are read through `GetIpForwardTable2` and `GetIpNetTable2`, which any user may call.
