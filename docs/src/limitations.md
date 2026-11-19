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
| Windows 10 on amd64 | `shint.windows.amd64.exe` | **Run twice**, on Windows 10 (22H2) before v4.4.0, as a normal user and as an administrator: all 34 checks of its [platform test](#testers-wanted) passed both times - the first run of `ss`, `ip route` and `ip arp` on Windows. Stopping a run with `Ctrl+C` has not been tried. |
| FreeBSD on amd64 | `shint.freebsd.amd64` | **Run once**, on FreeBSD 14.4 before v4.4.0, with its [platform test](#testers-wanted): `ip arp` saw no neighbours (fixed before the release) and `ping` cannot work there ([#112](https://github.com/dmartsapp/shint/issues/112)); the other 30 checks passed. |
| NetBSD on amd64 | `shint.netbsd.amd64` | **Run once**, on NetBSD 11.1 before v4.4.0: all 32 checks of its platform test passed, `ip route` and `ip arp` (IPv4 and IPv6 neighbours) matched `netstat -rn`, `arp -an` and `ndp -an`, and `ping` fails as on FreeBSD ([#112](https://github.com/dmartsapp/shint/issues/112)). |
| OpenBSD on amd64 | `shint.openbsd.amd64` | **Run once**, on OpenBSD 8.0 before v4.4.0: the platform test itself had to be fixed first (OpenBSD's shell could not parse it), then all 32 checks passed; `ip arp` matched `arp -an`, `ip route` matched `netstat -rn` once the subnet's broadcast entry was left out (fixed), and `ping` fails as on FreeBSD ([#112](https://github.com/dmartsapp/shint/issues/112)). |
| Solaris 11.4 on amd64 | `shint.solaris.amd64` | **Run once**, on Solaris 11.4 before v4.4.0: the platform test had to use the POSIX tools in `/usr/xpg4/bin` (Solaris's own `grep` has no `-E`), then all 31 checks passed; `--interface net0` now includes the addresses on `net0:1` and its other logical interfaces (fixed), `ip route` and `ip arp` say they are not supported yet, and `ping` fails as on the BSDs ([#112](https://github.com/dmartsapp/shint/issues/112)). |
| FreeBSD, NetBSD, OpenBSD and Windows on arm64, illumos, Android, AIX | the rest | **Built, never run by the project.** |

So on everything below the first two rows, **you are the testing**. If shint misbehaves there, an issue with what you ran and what it printed is the only way anyone finds out.

### Testers wanted

Windows, the BSDs, Solaris, illumos, Android and AIX need people who use them. You do not need to know Go: every release binary has a test script of its own in [`test/platforms/`](https://github.com/dmartsapp/shint/tree/main/test/platforms) - a shell script, or PowerShell for Windows. It runs about 30 checks in a minute: it starts shint's own listeners on `127.0.0.1` and checks them, reads the machine's interfaces, routes and sockets, and, when example.com answers, tries a few checks over the internet. It changes nothing and needs no root. It knows what your platform should do, so `ss` saying "not supported yet" on FreeBSD counts as a pass.

Download your platform's binary and its script, from the same release. On FreeBSD, for example (`fetch` or `wget` work too):

```bash
curl -LO https://github.com/dmartsapp/shint/releases/download/v{{version}}/shint.freebsd.amd64
curl -LO https://raw.githubusercontent.com/dmartsapp/shint/v{{version}}/test/platforms/freebsd-amd64.sh
chmod +x shint.freebsd.amd64
sh freebsd-amd64.sh ./shint.freebsd.amd64
```

On Windows, in PowerShell:

```powershell
Invoke-WebRequest -OutFile shint.windows.amd64.exe https://github.com/dmartsapp/shint/releases/download/v{{version}}/shint.windows.amd64.exe
Invoke-WebRequest -OutFile windows-amd64.ps1 https://raw.githubusercontent.com/dmartsapp/shint/v{{version}}/test/platforms/windows-amd64.ps1
powershell -ExecutionPolicy Bypass -File .\windows-amd64.ps1 .\shint.windows.amd64.exe
```

The script writes `results/freebsd-amd64.txt` (`results\windows-amd64.txt`): which release you tested and the binary's SHA-256, the system and its version, how many checks passed, and the error lines of any that failed - never the machine's name or its addresses. [Open an issue](https://github.com/dmartsapp/shint/issues/new) and attach it: a check that fails on your system is exactly what we need to know. Results that come in are kept in [`test/platforms/results/`](https://github.com/dmartsapp/shint/tree/main/test/platforms/results), one file per platform, and each one is checked against the SHA-256 published with its release.

If you have Go and Python, `make check` runs the whole test suite, which tells us far more.

## Commands that differ by platform

- **`ss` works on Linux and Windows only.** On macOS, the BSDs, Solaris and illumos it says it is not supported yet and exits `1`; the BSDs ([#105](https://github.com/dmartsapp/shint/issues/105)), Solaris 11 ([#113](https://github.com/dmartsapp/shint/issues/113)) and illumos ([#114](https://github.com/dmartsapp/shint/issues/114)) are planned. **The AIX binary has no `ss` at all.** AIX has no way to list its sockets, with the process holding each, without root. Its own tools read kernel memory as root, and shint never asks for root. So the command is left out of that binary rather than shipped as one that can never work. See [ss](ss.md#platforms).
- **`ip route` and `ip arp` work on Linux, macOS, the BSDs and Windows.** On Solaris, illumos and AIX they say they are not supported yet and exit `1`; `ip` itself works everywhere. On Windows they, and `ss`, first ran for v4.4.0 (Windows 10, as a normal user).
- **`--interface` is not on `ping` yet**: its ICMP library cannot send from an address so far (it comes with go-net, [#70](https://github.com/dmartsapp/shint/issues/70)). The other commands that send take it.
- **`ping` needs an unprivileged ICMP socket, which few systems have.** macOS and Windows have one (on Windows a raw ICMP socket, which a normal user may open there). Linux allows it only to the groups in `net.ipv4.ping_group_range`: most desktop distributions allow everyone, a hardened server may not (see [troubleshooting](troubleshooting.md)). **FreeBSD, NetBSD, OpenBSD and Solaris have none, so `ping` does not work there, not even as root** - it says `socket: protocol not supported` and exits `1`. illumos and AIX are probably the same but untried.
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

- **`ping --timeout` does not limit the name lookup** ([#14](https://github.com/dmartsapp/shint/issues/14)). The ping library resolves with its own fixed 5-second limit.
- **`ping` stopped with `Ctrl+C` ends without its statistics** ([#25](https://github.com/dmartsapp/shint/issues/25)).
- **`ping` does not work on FreeBSD, NetBSD, OpenBSD and Solaris**, and probably not on illumos and AIX ([#112](https://github.com/dmartsapp/shint/issues/112)).

All three are in `ping`'s library, and are planned with its replacement ([#70](https://github.com/dmartsapp/shint/issues/70)).

## Quirks kept on purpose

A few things in the JSON output are odd, and stay that way so that scripts written against them keep working: `timeout_ms` holds seconds, and `ping`'s `from_port`/`to_port` hold `7` and mean nothing. They are listed in [Output](output.md#inputparams).
