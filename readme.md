# shint

**that SHIt Network Tool** - the network checks you run every day, in one small program.

[![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**[Download](https://github.com/dmartsapp/shint/releases/latest)** &nbsp;|&nbsp; **[Documentation](https://dmartsapp.github.io/shint/)** &nbsp;|&nbsp; [Changelog](CHANGELOG.md)

## What is it?

shint puts the small utilities you keep reaching for - checking whether a port is open, pinging a host, making a web request, scanning for open ports, probing a UDP service, and running a quick test server - into **one file** that works the same on Linux, macOS, Windows and more. Every command explains what it found in the same clear, readable format.

## Why use it?

- **One tool instead of five.** `telnet`, `ping`, `curl`, `nmap` and `nc` habits, without hunting for each one.
- **Answers, not noise.** Every line says what was checked, whether it worked, and how long it took.
- **Works everywhere.** A single download per platform, or a Docker image. Nothing else to install.
- **Made for scripts.** Add `--json` for machine-readable output. Standard exit codes: `0` everything passed, `1` something failed, `2` you used it wrongly.
- **Practise safely.** `shint listen` starts a test server on your own machine, so you can try things without touching anything real.
- **Private.** No telemetry, no accounts, no configuration files.

## See it work

Can I reach this service - over IPv4 and IPv6?

```
$ shint telnet google.com 443
Sun Sep 20 01:50:14 MDT 2026: [telnet] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.46.78] time=47.672458ms
Sun Sep 20 01:50:15 MDT 2026: [telnet] OK connect ok host=2607:f8b0:400a:803::200e port=443 attempt=1/1 time=30.388833ms
Sun Sep 20 01:50:16 MDT 2026: [telnet] OK connect ok host=142.251.46.78 port=443 attempt=1/1 time=30.467459ms
```

Start a test web server, then call it:

```
$ shint listen http 8080
Sun Sep 20 01:50:18 MDT 2026: [listen-http] OK listening address=0.0.0.0:8080 max_requests=6
Sun Sep 20 01:50:19 MDT 2026: [listen-http] OK request method=GET path=/ status=200 remote=127.0.0.1:59313 bytes_received=97 bytes_sent=160 time_taken=504.375µs

$ shint web http://127.0.0.1:8080/
Sun Sep 20 01:50:19 MDT 2026: [web] OK response url=http://127.0.0.1:8080/ status=200 bytes_sent=97 bytes_received=160 speed=50.92KB/s attempt=1/1 time=3.0685ms
```

## Commands

| Command | What it answers |
|---|---|
| `shint telnet <host> <port>` | Can I open a TCP connection to this host and port? |
| `shint ping <host>` | Is this host reachable, and how fast does it answer? |
| `shint web <url>` | What does this URL return? (HTTP and HTTPS, headers, bodies, TLS, mutual TLS) |
| `shint nmap <host> --from 1 --to 1024` | Which TCP ports are open? |
| `shint udp <host> <port>` | Does this UDP service answer? |
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

Step-by-step instructions for every platform, building from source and shell completion are in the **[installation guide](https://dmartsapp.github.io/shint/docs/install.html)**.

## Learn more

- **[Using shint](https://dmartsapp.github.io/shint/docs/usage.html)** - the flags every command shares, output formats and exit codes
- **[Cookbook](https://dmartsapp.github.io/shint/docs/cookbook.html)** - ready-made recipes
- **[Troubleshooting](https://dmartsapp.github.io/shint/docs/troubleshooting.html)** - the common surprises, explained
- **[Technical documentation](https://dmartsapp.github.io/shint/docs/tech-architecture.html)** - architecture, every source file, CI/CD, releases and testing

## Roadmap

shint ships **one release every two weeks, one at a time**. Here is what is planned. Dates are targets, and the plan may change as we learn what is most useful.

| Release | Sprint | What is coming |
|---|---|---|
| **v4.0.4** | Released Sep 20 | Fixes: `ping --timeout` now works and `ping` shows the payload size on every reply, clearer `udp` help, documentation fixes. Releases now build only from a version tag on `main` |
| **v4.1.0** | Oct 5 - Oct 18 | `web --timing` (where the time went: DNS, connect, TLS, first byte, download), `wol` (wake a machine on your network), `cidr` (subnet calculator), `ntp` (check your clock against a time server). Verified downloads: SHA-256 checksums and signed build attestations for every binary |
| **v4.2.0** | Oct 19 - Nov 1 | `ip` (your interfaces and addresses), `dns` (lookups like `dig`), authoritative name servers shown whenever shint resolves a name, banner grabbing in `telnet`, `udp --hex` for binary payloads |
| **v4.3.0** | Nov 2 - Nov 15 | `tls` (certificate chain and expiry checks), `nmap` upgrades: port lists, subnet sweeps, service names |
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
