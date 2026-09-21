---
title: wol
lead: Send a Wake-on-LAN magic packet to switch on a machine on your network - no privileges needed.
description: shint wol broadcasts a Wake-on-LAN magic packet as a UDP datagram to wake a sleeping or switched-off machine on the local network segment.
section: Commands
order: 7
nav: wol
---

## What it does

Wake-on-LAN lets a machine that is asleep or switched off be powered on by a network message. `shint wol` builds that message - the **magic packet**: six bytes of `0xFF` followed by the target's MAC address sixteen times, 102 bytes in all - and broadcasts it as a UDP datagram to the local network.

It is an ordinary UDP send. It needs **no special privileges** and no raw sockets.

:::note What "sent" means
Wake-on-LAN has no reply. `shint wol` can tell you the packet was handed to the network; it cannot tell you the machine woke up. Whether it does depends on the machine, not on shint.
:::

## Before you use it

- **The machine must be set up for it.** Wake-on-LAN is switched on in the machine's firmware (BIOS/UEFI, often "Wake on LAN" or "Power on by PCIe") and in its network card's settings in the operating system. Wi-Fi rarely supports it; a wired connection almost always does.
- **You need its MAC address**, the hardware address of its network card (`ip link` on Linux, `ifconfig` on macOS, `ipconfig /all` on Windows).
- **It must be on the same network segment** as the machine running shint. Broadcasts do not cross routers, so this does not work across the internet, and usually not across a VPN.

## Syntax

```bash
shint wol <mac> [--broadcast ADDR] [--port N] [--count N] [--delay MS] [--json]
```

| Flag | Default | Meaning |
|---|---|---|
| `--broadcast ADDR` | `255.255.255.255` | The IPv4 broadcast address to send to. The default reaches every host on the network of the interface your default route uses. To target one subnet - or when the machine has several network interfaces - give that subnet's broadcast address, such as `192.168.1.255` (`shint cidr` can work it out). |
| `--port N` | `9` | The UDP port. Machines listen for the magic packet regardless of port; `9` (discard) and `7` (echo) are the conventional ones. |

The other [shared flags](usage.md#flags-shared-by-every-command) apply: `--count` sends the packet more than once (`--delay` apart), which can help on a busy network, and `--timeout` bounds each send.

The MAC address may be written `aa:bb:cc:dd:ee:ff`, `aa-bb-cc-dd-ee-ff`, in Cisco's dotted groups `aabb.ccdd.eeff`, or as twelve bare hex digits `aabbccddeeff`, in either case.

## Examples

### Wake a machine

```bash
shint wol aa:bb:cc:dd:ee:ff
```

The packet goes to `255.255.255.255:9`. For one particular subnet:

```bash
shint wol aa:bb:cc:dd:ee:ff --broadcast 192.168.1.255
```

### See exactly what is sent

To try it without waking anything, aim it at a listener on this machine. In one terminal:

```bash
shint listen udp 9098 --count 1
```

In another:

```bash
shint wol aa:bb:cc:dd:ee:ff --broadcast 127.0.0.1 --port 9098
```

```text
Sun Sep 20 17:36:27 MDT 2026: [wol] OK magic packet sent mac=aa:bb:cc:dd:ee:ff to=127.0.0.1:9098 bytes=102 attempt=1/1 time=1.210209ms
Sun Sep 20 17:36:27 MDT 2026: [wol] OK done packets_sent=1 total_time=1.003254875s
```

and the listener reports the packet it received:

```text
Sun Sep 20 17:36:25 MDT 2026: [listen-udp] OK listening address=0.0.0.0:9098 max_packets=1 echo=false
Sun Sep 20 17:36:27 MDT 2026: [listen-udp] OK packet received remote=127.0.0.1:62695 bytes=102 preview=\xff\xff\xff\xff\xff\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff\xaa\xbb\xcc\xdd\xee\xff
Sun Sep 20 17:36:27 MDT 2026: [listen-udp] OK done packets=1 bytes_received=102 total_time=2.016482125s
```

`bytes=102` is the whole magic packet. The preview shows it byte by byte: six `\xff`, then `\xaa\xbb\xcc\xdd\xee\xff` again and again - the MAC address sixteen times. (Listeners show anything that is not plain text as `\xNN` escapes, so a binary packet cannot disturb your terminal.)

### JSON

```bash
shint wol aa:bb:cc:dd:ee:ff --broadcast 127.0.0.1 --port 9097 --json
```

```json
{
  "input_params": { ... },
  "module_name": "wol",
  "stats": [
    {
      "mac": "aa:bb:cc:dd:ee:ff",
      "address": "127.0.0.1",
      "port": 9097,
      "success": true,
      "bytes_sent": 102,
      "sent_unixtime_µs": 1789947265536073,
      "time_taken_µs": 365
    }
  ],
  "end_time_unixtime_µs": 1789947265536440,
  "start_time_unixtime_µs": 1789947265536068,
  "total_time_taken_µs": 372,
  "error": ""
}
```

Like [`cidr`](cidr.md), the document has no `dns_lookup`: nothing is looked up. In `input_params`, `data` holds the MAC address, `payload_bytes` is `102`, and (as for every command) `timeout_ms` carries the `--timeout` value in seconds. In a stat, `success` means the operating system accepted the packet for sending.

### A mistake

```bash
shint wol not-a-mac
```

```text
invalid MAC address "not-a-mac": use six hex bytes, such as aa:bb:cc:dd:ee:ff
```

Standard error, exit status `2`. `--broadcast ::1` is refused the same way: Wake-on-LAN here is IPv4 broadcast, and IPv6 has no broadcast.

## Exit status

`0` when every packet was handed to the network, `1` when a send failed (for example, "network is unreachable" on a machine with no network), `2` for a malformed MAC address, broadcast address or port.

## Good to know

- **`Ctrl+C` shows how far it got.** With a large `--count`, the run stops, prints an `interrupted` line and its `done` line, and exits `1` (cut short); an attempt still in flight is dropped, not counted as a failure - see [Stopping early](usage.md#stopping-early).
- **It did not wake?** Check the machine's firmware and network-card settings first; then try `--broadcast` with the subnet's own broadcast address; then check the machine is on the same segment (wired, same VLAN).
- **A machine that is fully powered off** (not just asleep) often needs Wake-on-LAN enabled for the "S5" state, a separate setting on many boards.
- A **directed broadcast** to another subnet (`--broadcast 10.1.2.255` while you are on `10.1.1.0/24`) is only forwarded if the routers in between are set up to forward it, which they usually are not.
