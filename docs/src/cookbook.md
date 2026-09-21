---
title: Cookbook
lead: Ready-made recipes for the jobs shint is most often used for.
description: Practical shint recipes - check reachability, monitor in scripts, find where a request is slow, check a clock, look up an address, wake a machine, compare client and server byte counts, test firewalls and TLS.
section: Guides
order: 1
nav: Cookbook
---

## Is this service reachable?

```bash
shint telnet db.internal 5432 --timeout 2
```

A refusal or a timeout exits `1`, so it drops straight into a script:

```bash
if shint telnet db.internal 5432 --timeout 2 --delay 0 > /dev/null; then
  echo "up"
else
  echo "down"
fi
```

Both the IPv4 and the IPv6 address are tried, and *every* one must connect for the exit status to be `0`. If your network has no IPv6 route, the IPv6 attempt fails and so does the exit status - test the IPv4 address directly (`shint telnet 192.0.2.10 5432`), or read the per-address `success` fields from `--json`.

## Watch reachability over time

Twenty checks, two seconds apart:

```bash
shint telnet example.com 443 --count 20 --delay 2000
```

The statistics block at the end shows how many succeeded and the minimum, average and maximum connect time.

## Health-check an HTTP endpoint in a script

`web` exits `0` for any HTTP response, so check the status explicitly:

```bash
status=$(shint web --json https://example.com/health | jq '.stats[0].status_code')
[ "$status" = "200" ] || { echo "unhealthy: $status"; exit 1; }
```

If the request gets no response at all (refused, timeout, TLS failure), `web` itself exits `1` and the JSON has `"success": false` with the reason.

## Compare what the client sent with what the server received

Both sides count the same bytes, headers included, so a mismatch points at something in between - a proxy that rewrites headers, say.

```bash
shint listen http 8080 --count 1      # terminal 1
shint web http://127.0.0.1:8080/      # terminal 2
```

`web` reports `bytes_sent=97 bytes_received=160`; the listener reports `bytes_received=97 bytes_sent=160`. Point `web` at a real service instead and compare its numbers with that service's own access logs.

## Test a firewall rule

On the machine that should accept traffic:

```bash
shint listen http 9000
```

From another machine:

```bash
shint telnet server.example.com 9000 --timeout 2
```

`connect ok` means the rule allows it. A timeout means something is dropping the packets; a refusal means the packets got through but nothing was listening (start the listener and retry).

## Find what is open on a machine

```bash
shint nmap 192.168.1.10 --from 1 --to 1024 --timeout 1
```

Lower `--timeout` makes scans of hosts with many filtered ports much quicker. See [nmap](nmap.md#how-long-will-it-take) for the timing rules.

## Check a private HTTPS service

With an internal certificate authority:

```bash
shint web --cacert internal-ca.pem https://intranet.example.com/
```

With mutual TLS:

```bash
shint web --cacert ca.pem --cert client.pem --key client-key.pem https://api.example.com/
```

For a quick look at a service with a self-signed certificate, `-k` skips verification - but treat the answer as untrusted.

## Probe a UDP service

```bash
shint udp 192.168.1.1 53 --data "hello" --timeout 3
```

`open` means it answered; `closed` means the host said nothing is listening; `open|filtered` means silence. See [udp](udp.md) for how to read that.

## Check only IPv4 (or only IPv6)

A name with both kinds of address is checked over both. On a machine with no IPv6 that fails the IPv6 half, so if IPv4 is what you mean:

```bash
shint telnet db.internal 5432 -4 --timeout 2
```

`-6` is the reverse. Both work on `telnet`, `ping`, `nmap`, `udp`, `web`, `ntp` and `rdns` ([details](usage.md#ipv4-only-or-ipv6-only)).

## Find where a request is slow

`--timing` splits each request into DNS, connect, TLS, wait for the first byte, and download, per redirect hop:

```bash
shint web https://example.com --timing
```

A large `dns` points at the resolver, `connect` at the network path or a firewall, `tls` at the handshake (or a distant server), `wait` at the server itself, and `download` at the size of the response or the bandwidth. The [web page](web.md#timing-where-the-time-went) shows real output for each case, and `--json` gives the same numbers in microseconds for a script to alert on.

## Is this machine's clock right?

```bash
shint ntp pool.ntp.org --max-offset 500
```

Exit `0` when every server that answered says your clock is within half a second, `1` if one is further out or nothing answered. The offset is signed - positive when your clock is behind - and `--json` gives it as `offset_µs`. See [ntp](ntp.md).

```bash
if ! shint ntp time.cloudflare.com --max-offset 500 --delay 0 > /dev/null; then
  echo "clock is off (or the time server cannot be reached)"
fi
```

## Whose address is this?

```bash
shint rdns 203.0.113.9
```

Lists the names the address maps back to, or exits `1` with `no such host` when it has no PTR record. Given a host name instead, it looks up every address the name has - a quick way to see whether a service's addresses all carry a sensible reverse name. See [rdns](rdns.md).

## What does DNS say - and did my change go live?

```bash
shint dns example.com MX @1.1.1.1
shint dns example.com NS
shint dns example.com SOA @a.iana-servers.net --no-recurse
```

The first asks a specific resolver for the mail servers. The second lists the zone's name servers. The third asks one of those name servers **directly**: `flags=[qr,aa]` means an authoritative answer, and the serial number in the SOA tells you whether the version you published is the one being served. If a resolver still shows the old value, the TTL on its line is how many seconds it may keep doing so. A name that does not exist exits `1` with `no such domain (NXDOMAIN)` and the zone's SOA. See [dns](dns.md).

## Wake a machine on your network

```bash
shint wol aa:bb:cc:dd:ee:ff --broadcast 192.168.1.255
```

Sends the magic packet to the subnet. Success means the packet was sent; the machine has to be set up for Wake-on-LAN and be on the same network segment ([wol](wol.md) explains both). To find the subnet's broadcast address, `shint cidr 192.168.1.0/24` prints it.

## Work out a subnet

```bash
shint cidr 10.20.30.40/22
```

Prints the network (`10.20.28.0/22`), netmask, wildcard, first and last address, broadcast, and how many hosts fit - for several prefixes at once, IPv4 and IPv6, with no network involved. See [cidr](cidr.md).

## Use shint from Docker

```bash
docker run --rm farhansabbir/shint:latest telnet example.com 443
docker run --rm farhansabbir/shint:latest web --json https://example.com/health
```

Inside a container, "localhost" is the container itself. To test something on the host, use the host's address (`host.docker.internal` on Docker Desktop) or `--network host` on Linux.

## Imitate uneven traffic

`--throttle` waits a random 0-10 seconds between attempts, which is useful for exercising monitoring or rate limits:

```bash
shint web https://example.com/ --count 10 --throttle
```

## Keep machine-readable results

```bash
shint web --json https://example.com/ > result.json
shint telnet example.com 443 --json | jq '.stats[] | {address, success, micros: .["time_taken_µs"]}'
```

Failed checks are valid JSON too, so the same pipeline works on failure.
