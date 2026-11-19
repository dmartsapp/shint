# shint

### Respect the classics. Rethink the experience.

**Simple Host INspection Toolkit** - the network checks you run every day, in one small program.

[![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Check](https://github.com/dmartsapp/shint/actions/workflows/check.yaml/badge.svg?branch=main)](https://github.com/dmartsapp/shint/actions/workflows/check.yaml)
[![Release](https://github.com/dmartsapp/shint/actions/workflows/release.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/release.yaml)
[![Donate](https://img.shields.io/badge/Donate-PayPal-00457C?logo=paypal&logoColor=white)](https://www.paypal.com/paypalme/farhanssiddique)

**[Documentation](https://dmartsapp.github.io/shint/)** &nbsp;|&nbsp; **[Latest release](https://github.com/dmartsapp/shint/releases/latest)** &nbsp;|&nbsp; [Changelog](CHANGELOG.md) &nbsp;|&nbsp; [Discussions](https://github.com/dmartsapp/shint/discussions)

![shint checking a connection, an HTTP request with timing, DNS, a TLS certificate as JSON, and open ports](docs/assets/demo.gif)

`ping`, `telnet`, `curl`, `openssl`, `nmap`, `dig`: each is great at its job, and each has its own syntax and its own output. shint does the everyday checks of all of them in **one static binary**, with:

- **one output format**: `<time>: [command] OK|ERROR <what happened> key=value ...`, the same for every command;
- **`--json` on every command**, for scripts and CI;
- **the same exit codes everywhere**: `0` passed, `1` failed, `2` used wrongly;
- **no root, no config file, no telemetry**: one file, built for Linux, macOS, Windows, the BSDs, Solaris, illumos, AIX and Android.

The classics stay where they are, doing what they do best. shint is the one command you already know, for the checks you run most.

## Install

```bash
# Linux (amd64)
curl -Lo shint https://github.com/dmartsapp/shint/releases/latest/download/shint.linux.amd64 && chmod +x shint

# macOS (Apple silicon)
curl -Lo shint https://github.com/dmartsapp/shint/releases/latest/download/shint.darwin.arm64 && chmod +x shint

# Docker
docker run --rm farhansabbir/shint telnet example.com 443

# Go
go install github.com/dmartsapp/shint/v4@latest
```

Windows and the other platforms, checksums and signed attestations: see the [installation guide](https://dmartsapp.github.io/shint/docs/install.html).

## Commands

| Command | Instead of | What it answers |
|---|---|---|
| `shint telnet <host> <port>` | `telnet`, `nc -zv` | Can I connect to this port? |
| `shint ping <host>` | `ping` | Is it up, and how fast does it answer? |
| `shint web <url>` | `curl -v` | What does this URL return, and where did the time go? |
| `shint tls <host> [port]` | `openssl s_client` | Is the certificate trusted, valid for the name, and when does it expire? |
| `shint nmap <host\|prefix> -p 22,80,443` | `nmap` | Which TCP ports are open, on one host or a whole subnet? |
| `shint udp <host> <port>` | `nc -u` | Does this UDP service answer? |
| `shint dns <name> [type] [@server]` | `dig` | What does DNS say about this name? |
| `shint rdns <address>` | `dig -x` | Which names does this address map back to? |
| `shint ntp <server>` | `ntpdate -q` | How far off is this machine's clock? |
| `shint wol <mac>` | `wakeonlan` | Wake a machine on my network. |
| `shint cidr <prefix>` | `ipcalc` | What are this subnet's network, mask, range and size? |
| `shint ip [interface]` | `ip addr`, `ipconfig` | What interfaces and addresses does this machine have? |
| `shint ip route [address]` | `ip route`, `netstat -rn` | Which way does traffic to this address go, and through which gateway? |
| `shint ip arp` | `ip neigh`, `arp -a` | Which hardware addresses has this machine learned on its networks? |
| `shint ss -l` | `ss`, `netstat -anp` | What is listening on this machine, and which process holds each socket? |
| `shint listen tcp\|udp\|http <port>` | `nc -l` | Give me a local server to test against. |

Every command takes `--json`, `-4`/`-6` and `--help`. In a script:

```bash
shint telnet db.internal 5432 > /dev/null && echo "database reachable"
shint tls example.com --json | jq '.stats[0].expires_in_days'
```

More in the documentation: [using shint](https://dmartsapp.github.io/shint/docs/usage.html), the [cookbook](https://dmartsapp.github.io/shint/docs/cookbook.html), [troubleshooting](https://dmartsapp.github.io/shint/docs/troubleshooting.html), [output and JSON](https://dmartsapp.github.io/shint/docs/output.html), and [how it is built and tested](https://dmartsapp.github.io/shint/docs/tech-architecture.html).

## Limitations

- **Use it on networks you run, or have permission to test.** Its scans are ordinary connections that the target's firewall and services log, and it deliberately has nothing for scanning unnoticed. A sweep of public addresses warns before it starts and is paced by `--delay` (a probe a second by default).
- **It never needs root**, so nothing that needs raw packets: no SYN scans, no packet capture, no traceroute.
- **Some commands differ by platform.** `ss` lists sockets on Linux and Windows only (the BSDs, Solaris and illumos are planned; the AIX binary has no `ss`). `ip route` and `ip arp` work on Linux, macOS, the BSDs and Windows. `ping` does not work on FreeBSD, NetBSD, OpenBSD or Solaris, which have no ICMP socket a normal user may open ([#112](https://github.com/dmartsapp/shint/issues/112)).
- **Not every build has been run.** Every release ships 18 binaries; 7 have been run by the project:

| Platform | amd64 | arm64 | Other |
|---|:-:|:-:|:-:|
| macOS | ⬜ | ✅ every change | - |
| Linux | ✅ every change | ⬜ | ⬜ arm &nbsp; ⬜ ppc64le |
| Windows | ✅ v4.4.0 | ⬜ | - |
| FreeBSD | ✅ v4.4.0 | ⬜ | - |
| NetBSD | ✅ v4.4.0 | ⬜ | - |
| OpenBSD | ✅ v4.4.0 | ⬜ | - |
| Solaris | ✅ v4.4.0 | - | - |
| illumos | ⬜ | - | - |
| Android | - | ⬜ | - |
| AIX | - | - | ⬜ ppc64 |

✅ **every change**: the whole test suite runs on it. ✅ **v4.4.0**: its [platform test](test/platforms/) passed for v4.4.0. ⬜ built for every release, not run by the project yet: if you use one, a run of its platform test is the most useful thing you can send ([Testers wanted](https://dmartsapp.github.io/shint/docs/limitations.html#testers-wanted)). A dash: there is no such build.

Everything else - what was tested where, what differs, the bugs still open - is on the [Limitations page](https://dmartsapp.github.io/shint/docs/limitations.html).

## Roadmap

A release every two weeks. Earlier releases are in the [changelog](CHANGELOG.md).

| Release | Sprint | What it brings |
|---|---|---|
| [**v4.3.0**](releases/v4.3.0.md) | Released Nov 17 | `tls`: a server's certificate - chain, trust, names, expiry, protocol and cipher; SBOMs and security reports with every release |
| [**v4.4.0**](releases/v4.4.0.md) | Released Nov 19 | this machine's sockets, routes and neighbours (`ss`, `ip route`, `ip arp`), `nmap` port lists and subnet sweeps, `--interface` to check from a chosen interface, `web --headers-only`, `tls` showing the TLS versions it offered, sweeps of public addresses that warn and are paced by `--delay`, and `web` and `listen` hardened after an outside security review |
| [**v4.5.0**](https://github.com/dmartsapp/shint/blob/release/v4.5.0/branch_readme.md) | Nov 30 - Dec 13 | Performance and storage: `speed` (throughput between two of your machines), `disk` (I/O on a folder or a disk), Fibre Channel path state |
| **v4.6.0** | Dec 14 - Dec 27 | `inspect` (one command that checks a URL or host layer by layer and says where it breaks), path diagnostics and DNS propagation, `ping` fixes, and go-net - the networking library under shint |
| **v5.0.0** | Not scheduled | Only changes that break scripts: `--timeout` and `--delay` in seconds, `listen tcp` removed (use `listen http`) |
| **Later** | | Homebrew and Scoop packages |

Everything keeps the ground rules: one small file, nothing to configure, never root. Ideas are welcome as [issues](https://github.com/dmartsapp/shint/issues).

## Security and privacy

shint sends only what a check needs, to the addresses you name, and collects nothing. Every release binary comes with a checksum, a signed build attestation and SBOMs; every Docker image with SBOMs and vulnerability reports. Report a vulnerability privately [through GitHub](https://github.com/dmartsapp/shint/security/advisories/new); the policy is in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) &copy; Farhan Sabbir Siddique

If shint saves you time, you can [support its development](https://www.paypal.com/paypalme/farhanssiddique).
