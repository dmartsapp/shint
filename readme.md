# shint

**Simple Host INspection Toolkit** - the network checks you run every day, in one small program.

[![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Lint](https://github.com/dmartsapp/shint/actions/workflows/lint.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/lint.yaml)
[![Vulnerability check](https://github.com/dmartsapp/shint/actions/workflows/vulncheck.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/vulncheck.yaml)
[![Build](https://github.com/dmartsapp/shint/actions/workflows/build.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/build.yaml)
[![Docker Hub](https://github.com/dmartsapp/shint/actions/workflows/docker-hub.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/docker-hub.yaml)
[![GHCR](https://github.com/dmartsapp/shint/actions/workflows/ghcr.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/ghcr.yaml)
[![Check](https://github.com/dmartsapp/shint/actions/workflows/check.yaml/badge.svg?branch=main)](https://github.com/dmartsapp/shint/actions/workflows/check.yaml)
[![Discussions](https://img.shields.io/github/discussions/dmartsapp/shint)](https://github.com/dmartsapp/shint/discussions)
[![Donate](https://img.shields.io/badge/Donate-PayPal-00457C?logo=paypal&logoColor=white)](https://www.paypal.com/paypalme/farhanssiddique)

**[Download](https://github.com/dmartsapp/shint/releases/latest)** &nbsp;|&nbsp; **[Documentation](https://dmartsapp.github.io/shint/)** &nbsp;|&nbsp; [Changelog](CHANGELOG.md) &nbsp;|&nbsp; [Discussions](https://github.com/dmartsapp/shint/discussions)

## Why shint?

shint puts the checks you reach for every day - is it up, can I connect, does it answer, what's open, what does DNS say - into **one file** that works the same on Linux, macOS, Windows and more.

**We are not trying to replace `ping`, `telnet`, `curl`, `nmap`, `dig`, `nc` or the rest.** They have earned their place through decades of real, trusted use, and each one still does its own job better than almost anything else could. shint doesn't compete with them - it learns from them, and brings what makes each one great together, under one roof: **simplicity, ease of use, for everyone.**

### The problem

A real troubleshooting session reaches for several of them, in sequence, every time:

```text
ping           - is it up?
  │
telnet / nc    - can I connect?
  │
curl           - does it answer?
  │
nmap           - what else is open?
  │
dig, ntpdate, ip addr, ...   - and whichever of a dozen other tools this particular question needs
  │
a script to glue the answers together, because no two of them agree on how to talk to one
```

Nine tools, nine syntaxes, nine output formats - and the moment you want to feed an answer into a script or a CI pipeline, you're writing a parser for each one. shint's job is to be the one command you already know how to use, for all of it, before you reach for the others.

### What we brought in, from each

| From | What we kept | In shint |
|---|---|---|
| **ping** | ICMP reachability and latency, payload size, round-trip statistics | ✅ `shint ping` |
| **telnet** / **nc** | "Can I open a TCP connection to this host and port?" | ✅ `shint telnet` |
| **curl** | HTTP and HTTPS requests, headers and body, TLS and mutual TLS, where the time went | ✅ `shint web` |
| **nmap** | TCP port scanning across a range | ✅ `shint nmap` |
| **nc -u** | Raw UDP probing, exact byte payloads | ✅ `shint udp` |
| **dig** | DNS lookups - A, AAAA, MX, TXT, NS and more, against any server | ✅ `shint dns` |
| **dig -x** | Reverse DNS (PTR) lookups | ✅ `shint rdns` |
| **ipcalc** / **sipcalc** | Subnet math: network, mask, range, size | ✅ `shint cidr` |
| **ifconfig** / **ip addr** | This machine's interfaces and addresses | ✅ `shint ip` |
| **ntpdate** / **sntp** | Clock offset against a time server | ✅ `shint ntp` |
| **wakeonlan** | Wake-on-LAN magic packets | ✅ `shint wol` |
| **nc -l** / `python -m http.server` | A local test server to check against | ✅ `shint listen` |

### And underneath all of it

Every one of those got this whether the original tool had it or not:

- **One log line, every command:** `<time>: [module] OK|ERROR <message> key=value ...` - grep-able the same way, everywhere
- **`--json` everywhere** - built in for scripts and pipelines, not bolted onto one command
- **The same exit codes everywhere** - `0` passed, `1` failed, `2` you used it wrong
- **One binary, no accounts, no telemetry, no config file** - the same on Linux, macOS and Windows

That consistency is the actual thing shint adds. None of the tools above had a reason to agree with each other before, and it's what lets shint sit comfortably next to the automation and CI pipelines those older tools were never quite built for.

## See it work

**We're not asking you to give up the tools you already trust - just to show you one that also speaks the language of scripts, CI and the automation growing up around them.**

### Is it up, can I connect?

```bash
# the old way - two tools, two syntaxes, two output shapes
ping -c 2 example.com
telnet example.com 443
```

```text
$ shint ping example.com --count 2
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK dns resolved host=example.com addresses=1 ips=[93.184.216.34] time=20.375µs
Sun Sep 20 15:37:48 MDT 2026: [icmp] OK received reply for request #1 from 93.184.216.34 (ipv4) in 27ms bytes=4
Sun Sep 20 15:37:49 MDT 2026: [icmp] OK received reply for request #2 from 93.184.216.34 (ipv4) in 28ms bytes=4

========================================= icmp STATISTICS =========================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 27ms, average: 27.5ms, maximum: 28ms
Sun Sep 20 15:37:49 MDT 2026: [icmp] OK done packets_lost=0 stddev_ms=0.500 resolve_time=20.375µs total_time=1.021s

$ shint telnet example.com 443
Sun Sep 20 01:50:14 MDT 2026: [telnet] OK dns resolved host=example.com addresses=1 ips=[93.184.216.34] time=25.109ms
Sun Sep 20 01:50:15 MDT 2026: [telnet] OK connect ok host=93.184.216.34 port=443 attempt=1/1 time=30.467ms
```

Same shape, both commands - and both answer with `--json` instead, the moment a script is what's actually asking.

### A target to test against

```bash
# the old way
nc -l 9002
```

```text
$ shint listen tcp 9002 --echo --count 1
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK listening address=0.0.0.0:9002 max_connections=1 echo=true
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK connection accepted remote=127.0.0.1:59324 local=127.0.0.1:9002
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK data received remote=127.0.0.1:59324 bytes_received=12 bytes_sent=12 time_taken=28.708µs preview="hello shint"
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK done connections=1 bytes_received=12 bytes_sent=12 total_time=410.53875ms
```

`nc -l` still works exactly as it always has, and nothing here says otherwise. `shint listen` is there for when you also want a byte count, a timestamp and `--json` on the way out - built for the next tool in the pipeline, not just the person reading the terminal.

## Commands

| Command | What it answers |
|---|---|
| `shint telnet <host> <port>` | Can I open a TCP connection to this host and port? |
| `shint ping <host>` | Is this host reachable, and how fast does it answer? |
| `shint web <url>` | What does this URL return, and where did the time go? (HTTP and HTTPS, headers, bodies, TLS, mutual TLS, `--timing`) |
| `shint nmap <host> --from 1 --to 1024` | Which TCP ports are open? |
| `shint udp <host> <port>` | Does this UDP service answer? (`--hex` sends exact bytes) |
| `shint dns <name> [type] [@server]` | What does DNS say about this name? A, AAAA, MX, TXT, NS and more, like `dig`. |
| `shint rdns <ip-or-host>` | Which names does this address map back to? (reverse DNS) |
| `shint ntp <server>` | How far is this machine's clock from a time server? |
| `shint wol <mac>` | Wake a machine on your network (Wake-on-LAN). |
| `shint cidr <prefix>` | Network, mask, range and size of a subnet, offline. |
| `shint ip [interface]` | Which network interfaces and addresses does this machine have? |
| `shint listen tcp\|udp\|http <port>` | Run a local test server to check against. |

## Install

Download the file for your platform from the [latest release](https://github.com/dmartsapp/shint/releases/latest), make it executable, and run it:

```
chmod +x shint.linux.amd64
./shint.linux.amd64 --version
```

Or use Docker:

```
docker run --rm farhansabbir/shint:latest telnet example.com 443
```

You can also build it with Go: `go install github.com/dmartsapp/shint/v4@latest`. Every release binary comes with a SHA-256 checksum file and a signed build attestation, so you can [verify your download](https://dmartsapp.github.io/shint/docs/install.html#verify-your-download).

Step-by-step instructions for every platform, building from source and shell completion are in the **[installation guide](https://dmartsapp.github.io/shint/docs/install.html)**.

## Learn more

- **[Using shint](https://dmartsapp.github.io/shint/docs/usage.html)** - the flags every command shares, output formats and exit codes
- **[Cookbook](https://dmartsapp.github.io/shint/docs/cookbook.html)** - ready-made recipes
- **[Troubleshooting](https://dmartsapp.github.io/shint/docs/troubleshooting.html)** - the common surprises, explained
- **[Technical documentation](https://dmartsapp.github.io/shint/docs/tech-architecture.html)** - architecture, every source file, CI/CD, releases and testing
- **[Discussions](https://github.com/dmartsapp/shint/discussions)** - ask a question, share an idea, see what shipped

## Roadmap

shint ships **one release every two weeks, one at a time**. Here is what has shipped and what is planned. Dates for what is planned are targets, and the plan may change as we learn what is most useful. v4.1.0 was never released on its own: everything planned for it shipped in v4.2.0, which came out ahead of its window to clear the backlog of fixes and features.

| Release | Sprint | What it brings |
|---|---|---|
| **v4.0.4** | Released Sep 20 | Fixes: `ping --timeout` now works and `ping` shows the payload size on every reply, clearer `udp` help, documentation fixes. Releases now build only from a version tag on `main` |
| **v4.0.5** | Released Sep 20 | `Ctrl+C` on a repeating `telnet`, `web` or `udp` run shows the summary and how far it got |
| **v4.0.6** | Released Sep 21 | Two wrong answers fixed: `web` no longer reports a cut-short response as a success, and `ping` no longer reports a dead host as reachable. A black-box test battery, and every release's result posted to Slack |
| **v4.1.0** | Shipped in v4.2.0 | `web --timing` (where the time went: DNS, connect, TLS, first byte, download), `wol` (wake a machine on your network), `cidr` (subnet calculator), `ntp` (check your clock against a time server), `rdns` (reverse DNS lookups), `-4`/`-6` to check one address family. Verified downloads: SHA-256 checksums and signed build attestations for every binary. `go install` support |
| **v4.2.0** | Released Sep 21 | `ip` (your interfaces and addresses), `dns` (lookups like `dig`), authoritative name servers shown whenever shint resolves a name, `udp --hex` for binary payloads, and fixes for what the test battery found: no crashes or overflows on bad flag values, `web` no longer holds a whole download in memory, `--count` with `--delay 0` no longer runs out of file descriptors, failures logged as errors, and `listen http` logs the requests it cannot serve |
| **v4.3.0** | Nov 2 - Nov 15 | `tls` (certificate chain and expiry checks), `nmap` upgrades: port lists, subnet sweeps, service names, banner grabbing in `telnet` |
| **v4.4.0** | Nov 16 - Nov 29 | `ip route` (routing table and default gateway), richer `ping`: sub-millisecond timings, and "unreachable" replies told apart from timeouts |
| **v4.5.0** | Nov 30 - Dec 13 | `speed` (measure throughput between two of your machines) |
| **v4.5.x** | After Dec 13 | Patch releases only: fixes, no new features. A lot has changed since v4.0, and this is the time to check it |
| **v5.0.0** | Not scheduled | The next major release, for the changes that would break what works today - see [What will break in v5.0.0](#what-will-break-in-v500) |
| **Later** | | Package managers such as Homebrew |

The plan runs to v4.5.0. After it there are no more minor releases - only patches - until v5.0.0.

Everything on the list keeps shint's ground rules: one small file, nothing to configure, and **never any need for administrator or root privileges**. Have an idea, or want something moved up? [Open an issue](https://github.com/dmartsapp/shint/issues).

### What will break in v5.0.0

A change that would break scripts on purpose - taking a command or a flag away, changing what one means - does not go into a v4.x release. It waits for the next major release, v5.0.0, which has no date yet. What is planned for it so far:

- **`shint listen tcp` is removed.** `shint listen` keeps `listen http` and `listen udp`. `listen http` also accepts a plain TCP connection, so it is the target for a `telnet` or `nmap` port check (a connection that sends nothing is accepted and not logged). `--echo` becomes a `listen udp` flag only; today `listen http` accepts it and ignores it. If you script `shint listen tcp <port>`, switch to `shint listen http <port>` - or to `nc -l <port>` if you want a listener that shows raw bytes. **Until v5.0.0, `listen tcp` keeps working.**

## Privacy

shint collects nothing. The only network traffic is what a check itself needs - DNS queries, TCP and UDP connections, ICMP packets and the HTTP requests you ask for - sent to the addresses you name.

## License

[MIT](LICENSE) &copy; Farhan Sabbir Siddique

If shint saves you time, you can [support its development](https://www.paypal.com/paypalme/farhanssiddique).
