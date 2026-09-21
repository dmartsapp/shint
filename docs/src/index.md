---
title: shint
lead: One small tool for the network checks you run every day - connect, ping, request, scan, probe, look up, check a clock and listen - with the same clear output on every platform.
description: shint is a single-binary network diagnostics tool - telnet-style port checks, ping, HTTP requests with timing, port scans, UDP probes, reverse DNS, NTP clock checks, Wake-on-LAN, a subnet calculator and local test listeners - with readable output, JSON, and proper exit codes.
section: Get started
order: 1
nav: Overview
---

:::html
<div class="hero-actions">
  <a class="btn primary" href="install.html">Install shint</a>
  <a class="btn" href="usage.html">Read the docs</a>
  <a class="btn" href="https://github.com/dmartsapp/shint/releases/latest">Download the latest release</a>
</div>
:::

## What is shint?

shint - *Simple Host INspection Toolkit* - puts the small utilities you keep reaching for into **one program**: a `telnet`-style port check, `ping`, an HTTP client, a port scanner, a UDP probe, a reverse DNS lookup, a clock check against a time server, Wake-on-LAN, a subnet calculator, and local test servers to try them against. Nothing to install alongside it, nothing to configure, and every command reports what it found in the same readable format.

:::html
<div class="facts">
  <div class="fact"><b>10</b><span>commands in one tool</span></div>
  <div class="fact"><b>14</b><span>OS / CPU combinations</span></div>
  <div class="fact"><b>1 file</b><span>static binary, ~10 MB</span></div>
  <div class="fact"><b>IPv4 + IPv6</b><span>checked in one run</span></div>
</div>
:::

## Why people use it

- **Answers, not noise.** Every line says what was checked, whether it worked, and how long it took: `[telnet] OK connect ok host=142.251.46.78 port=443 attempt=1/1 time=30.89725ms`.
- **The same everywhere.** One download per platform (Linux, macOS, Windows, the BSDs, Solaris, Android) or one Docker image. The commands, flags and output are identical on all of them.
- **Made for scripts.** Add `--json` for machine-readable output, and rely on standard exit codes: `0` everything passed, `1` something failed, `2` you used it wrongly.
- **Practise safely.** `shint listen` starts a TCP, UDP or HTTP server on your own machine, so you can test firewalls, load balancers and other tools without touching anything real.
- **Honest numbers.** Timeouts apply to the thing they are named for, and byte counts are measured on the wire - headers included - so the client's and the server's figures match.
- **Private by design.** No telemetry, no accounts, no configuration files. shint only talks to the addresses you give it.

## See it work

Is the port open - over IPv4 *and* IPv6?

```bash
shint telnet google.com 443
```

```text
Sun Sep 20 01:50:14 MDT 2026: [telnet] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.46.78] time=47.672458ms
Sun Sep 20 01:50:15 MDT 2026: [telnet] OK connect ok host=2607:f8b0:400a:803::200e port=443 attempt=1/1 time=30.388833ms
Sun Sep 20 01:50:16 MDT 2026: [telnet] OK connect ok host=142.251.46.78 port=443 attempt=1/1 time=30.467459ms

======================================= telnet STATISTICS =======================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 30.388833ms, average: 30.428146ms, maximum: 30.467459ms
Sun Sep 20 01:50:16 MDT 2026: [telnet] OK done total_time=2.081599125s
```
Start a test web server on your machine, and call it:

```bash
shint listen http 8080          # in one terminal
shint web http://127.0.0.1:8080/   # in another
```

```text
Sun Sep 20 01:50:18 MDT 2026: [web] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=114.5µs
Sun Sep 20 01:50:19 MDT 2026: [web] OK response url=http://127.0.0.1:8080/ status=200 bytes_sent=97 bytes_received=160 speed=50.92KB/s attempt=1/1 time=3.0685ms
```
Which ports are open on a machine?

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
## The commands

:::html
<div class="cards">
  <a class="card" href="telnet.html"><strong><code>telnet</code></strong><span>Can I open a TCP connection to this host and port?</span></a>
  <a class="card" href="ping.html"><strong><code>ping</code></strong><span>Is this host reachable, and how fast does it answer?</span></a>
  <a class="card" href="web.html"><strong><code>web</code></strong><span>Make an HTTP or HTTPS request and see what came back, including TLS and mutual TLS.</span></a>
  <a class="card" href="nmap.html"><strong><code>nmap</code></strong><span>Which TCP ports are open on this host?</span></a>
  <a class="card" href="udp.html"><strong><code>udp</code></strong><span>Send a UDP probe and classify the result: open, closed, or open|filtered.</span></a>
  <a class="card" href="ntp.html"><strong><code>ntp</code></strong><span>How far is this machine's clock from a time server?</span></a>
  <a class="card" href="wol.html"><strong><code>wol</code></strong><span>Wake a machine on your network with a Wake-on-LAN packet.</span></a>
  <a class="card" href="rdns.html"><strong><code>rdns</code></strong><span>Which names does this IP address map back to? Reverse DNS.</span></a>
  <a class="card" href="cidr.html"><strong><code>cidr</code></strong><span>Network, mask, range and size of a subnet, offline.</span></a>
  <a class="card" href="ip.html"><strong><code>ip</code></strong><span>What does this machine have? Its interfaces and addresses.</span></a>
  <a class="card" href="listen.html"><strong><code>listen</code></strong><span>Run a local TCP, UDP or HTTP server to test against.</span></a>
</div>
:::

## Instead of juggling tools

| You would normally use | With shint |
|---|---|
| `telnet host port`, `nc -zv host port` | `shint telnet host port` |
| `ping host` | `shint ping host` |
| `curl -v https://host/` | `shint web https://host/` |
| `curl -w '%{time_connect} %{time_starttransfer}' https://host/` | `shint web https://host/ --timing` |
| `nmap -p 20-9000 host` | `shint nmap host --from 20 --to 9000` |
| `nc -u host port` | `shint udp host port` |
| `nc -l 9000`, `python3 -m http.server` | `shint listen tcp 9000`, `shint listen http 8080` |
| `sntp pool.ntp.org`, `ntpdate -q pool.ntp.org` | `shint ntp pool.ntp.org` |
| `wakeonlan aa:bb:cc:dd:ee:ff`, `etherwake` | `shint wol aa:bb:cc:dd:ee:ff` |
| `dig -x 8.8.8.8`, `host 8.8.8.8` | `shint rdns 8.8.8.8` |
| `ipcalc 192.168.1.0/24`, `sipcalc` | `shint cidr 192.168.1.0/24` |
| `ip addr`, `ifconfig`, `ipconfig` | `shint ip` |

shint covers the everyday checks; it does not replace the full feature sets of `curl` or `nmap`.

## Where to next

- **New here?** [Install shint](install.md) takes a minute, then [Using shint](usage.md) explains the flags every command shares.
- **Have a specific job?** The [cookbook](cookbook.md) has ready-made recipes, and [troubleshooting](troubleshooting.md) covers the common surprises.
- **Curious how it works?** The [Technical](tech-architecture.md) section documents the architecture, every source file, the CI/CD pipeline and how releases are cut.
