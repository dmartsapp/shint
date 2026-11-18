---
title: Limitations
lead: What shint cannot do, where it has not been tested, and what is still broken. Said plainly, so nothing surprises you on a machine that matters.
description: shint's limitations - the platforms it is built for but not tested on, the commands that differ by platform, what it deliberately does not do, and the bugs still open - and how you can help test it.
section: Reference
order: 2
nav: Limitations
---

## Where shint is tested, and where it is not

shint is built for 18 platforms. Every release ships all of them. But only two are tested regularly, and that is the biggest limitation there is.

| Platform | Binaries | How it is tested |
|---|---|---|
| macOS on Apple silicon | `shint.darwin.arm64` | **Every change.** The whole test suite runs on the maintainer's Mac: the unit and command-line tests, a battery of more than 500 black-box cases, lint and the vulnerability check. A live smoke test against real hosts runs before each release. |
| Linux on amd64 | `shint.linux.amd64` | **Every merge.** The same test suite runs on GitHub's Ubuntu runners after each merge to `main`. `ss` was also checked against a real Linux host when it was written. |
| Linux on arm64, 32-bit arm and ppc64le; macOS on Intel | `shint.linux.arm64`, `shint.linux.arm`, `shint.linux.ppc64le`, `shint.darwin.amd64` | **Built, never run by the project.** The code is the same as on their tested siblings, so problems are unlikely, but nobody checks. |
| Windows | `shint.windows.amd64.exe`, `shint.windows.arm64.exe` | **Built, never run by the project.** The code written for Windows (`ss`'s socket tables, where `dns` finds the hosts file) is compiled, and unit-tested on recorded data where it can be, but it has never run on Windows. Neither has stopping a run with `Ctrl+C`. |
| FreeBSD, OpenBSD, NetBSD, Solaris, illumos, Android, AIX | the rest | **Built, never run by the project.** |

So on everything below the first two rows, **you are the testing**. If shint misbehaves there, an issue with what you ran and what it printed is the only way anyone finds out.

### Testers wanted

Windows, the BSDs, Solaris, illumos and AIX need people who use them. You do not need to know Go. Download the binary for your platform from the [latest release](https://github.com/dmartsapp/shint/releases/latest) and run:

```bash
shint --version
shint ip
shint ping 127.0.0.1 --count 3
shint telnet example.com 443
shint web https://example.com
shint tls example.com
shint dns example.com MX
shint nmap 127.0.0.1 -p 22,80,443
shint ntp pool.ntp.org
shint ss -l
shint ping example.com --count 20    # press Ctrl+C partway through
```

Then [open an issue](https://github.com/dmartsapp/shint/issues/new) with your platform and its version (`uname -a`, or `winver` on Windows) and the output: everything, or just the commands that surprised you. `shint ss` says it is not supported on most of these systems; that is expected (see below). If you have Go and Python, `make check` runs the whole test suite, which tells us far more.

## Commands that differ by platform

- **`ss` works on Linux and Windows only.** On macOS, the BSDs, Solaris and illumos it says it is not supported yet and exits `1`; the BSDs are planned ([#105](https://github.com/dmartsapp/shint/issues/105)). **The AIX binary has no `ss` at all.** AIX has no way to list its sockets, with the process holding each, without root. Its own tools read kernel memory as root, and shint never asks for root. So the command is left out of that binary rather than shipped as one that can never work. See [ss](ss.md#platforms).
- **`ip route` and `ip arp` work on Linux, macOS, the BSDs and Windows.** On Solaris, illumos and AIX they say they are not supported yet and exit `1`; `ip` itself works everywhere. The Windows tables, like `ss`'s, have not been read on a real Windows machine by the project.
- **`--interface` is not on `ping` yet**: its ICMP library cannot send from an address so far ([#69](https://github.com/dmartsapp/shint/issues/69)). The other commands that send take it.
- **`ping` on Linux may need a setting.** It uses unprivileged ICMP, which Linux allows only to the groups in `net.ipv4.ping_group_range`. Most desktop distributions allow everyone; a hardened server may not. See [troubleshooting](troubleshooting.md). macOS, the BSDs and Windows need nothing.
- **`listen --bind ::` accepting IPv4 too** has been verified on macOS and Linux only.
- **Some systems get no binary at all**, because Go, which shint is written in, does not build for them: HP-UX, Solaris on SPARC, and AIX on anything but IBM POWER.

## What shint does not do

These are choices, not gaps waiting to be filled. Each one keeps shint simple, safe to run anywhere, and free of root.

- **It is for networks you run.** A sweep of public addresses warns before it starts and is paced by `--delay` (1 s per probe by default), so a public `/24` takes minutes, not seconds. Set `--delay` for your own networks. See [Responsible use](security.md#responsible-use).
- **It never needs root.** So there is nothing that needs raw packets: no SYN or UDP port scans, no OS or version detection, no packet capture, and no traceroute ([#67](https://github.com/dmartsapp/shint/issues/67) explores whether one can be done without privileges).
- **`nmap` is a TCP connect scan.** Each open port's `service=` is the service usually found on that port number. It is a guess: nothing is read from the port.
- **TCP and UDP cannot tell "nothing there" from "a firewall dropped it".** A port that does not answer, and a UDP probe with no reply (`open|filtered`), mean exactly that and no more.
- **`web` never uses a proxy.** `HTTP_PROXY` and friends are ignored, on purpose: it measures the path from this machine to the server, not to a proxy.
- **It changes nothing.** No command adds an address, closes a socket, stops a process or alters a setting.
- **No banner grabbing, no iSCSI checks.** Both were planned and dropped; the reasons are on the [design page](tech-design.md#what-shint-does-not-do-and-why).

## Bugs still open

At the time of writing. The [issue tracker](https://github.com/dmartsapp/shint/issues?q=is%3Aissue+is%3Aopen+label%3Abug) always has the current list.

- **A name on several `/etc/hosts` lines may resolve to its first address only** ([#65](https://github.com/dmartsapp/shint/issues/65)). This affects every command that looks up a name.
- **`ping --timeout` does not limit the name lookup** ([#14](https://github.com/dmartsapp/shint/issues/14)). The ping library resolves with its own fixed 5-second limit.
- **`ping` stopped with `Ctrl+C` ends without its statistics** ([#25](https://github.com/dmartsapp/shint/issues/25)).

## Quirks kept on purpose

A few things in the JSON output are odd, and stay that way so that scripts written against them keep working: `timeout_ms` holds seconds, and `ping`'s `from_port`/`to_port` hold `7` and mean nothing. They are listed in [Output](output.md#inputparams).
