---
title: Cookbook
lead: Ready-made recipes for the jobs shint is most often used for.
description: Practical shint recipes - check reachability, monitor in scripts, compare client and server byte counts, test firewalls and TLS.
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
shint listen tcp 9000
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
