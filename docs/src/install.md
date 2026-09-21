---
title: Install
lead: shint is a single file. Download it, make it executable, and run it.
description: Install shint on Linux, macOS, Windows, the BSDs, Solaris or Android - or run it from Docker, or build it from source.
section: Get started
order: 2
nav: Install
---

## Download a release

Every release publishes one binary per platform on the [releases page](https://github.com/dmartsapp/shint/releases/latest). The file name is `shint.<os>.<arch>`, with `.exe` on Windows:

| Platform | amd64 (Intel / AMD) | arm64 (Apple silicon, ARM) |
|---|---|---|
| Linux | `shint.linux.amd64` | `shint.linux.arm64` |
| macOS | `shint.darwin.amd64` | `shint.darwin.arm64` |
| Windows | `shint.windows.amd64.exe` | `shint.windows.arm64.exe` |
| FreeBSD | `shint.freebsd.amd64` | `shint.freebsd.arm64` |
| OpenBSD | `shint.openbsd.amd64` | `shint.openbsd.arm64` |
| NetBSD | `shint.netbsd.amd64` | `shint.netbsd.arm64` |
| Solaris | `shint.solaris.amd64` | (Go has no Solaris arm64 port) |
| Android | (arm64 only) | `shint.android.arm64` |

Each binary is statically linked and about 10 MB. There is nothing else to install.

### macOS and Linux

Pick the file for your machine (`uname -sm` tells you), then:

```bash
# macOS on Apple silicon; use shint.linux.amd64 etc. for other platforms
curl -L -o shint https://github.com/dmartsapp/shint/releases/latest/download/shint.darwin.arm64
chmod +x shint
sudo mv shint /usr/local/bin/shint
shint --version
```

:::note macOS "cannot be opened because the developer cannot be verified"
Files downloaded with a browser carry a quarantine flag. Clear it once with `xattr -d com.apple.quarantine shint` (files fetched with `curl`, as above, are not flagged).
:::

### Windows

In PowerShell:

```bash
Invoke-WebRequest -Uri https://github.com/dmartsapp/shint/releases/latest/download/shint.windows.amd64.exe -OutFile shint.exe
.\shint.exe --version
```

Move `shint.exe` into a folder that is on your `PATH` to run it as `shint` from anywhere.

### Check it works

```bash
shint --version
```

A release binary prints the tag, the commit it was built from and the build time; a build from source prints just the version number:

```text
v4.0.2/1c4421137f2609ad420af60415c8e6fa1756da46/2026-09-20T05:34:40+0000
```

### Verify your download

From v4.1.0 every binary comes with two ways to check it. Earlier releases have neither.

**A checksum** catches a damaged or incomplete download. Each binary has a `.sha256` file next to it on the releases page. Download both into the same folder, **keeping their original names** (the checksum file names the file it belongs to), and check:

```bash
curl -LO https://github.com/dmartsapp/shint/releases/latest/download/shint.darwin.arm64
curl -LO https://github.com/dmartsapp/shint/releases/latest/download/shint.darwin.arm64.sha256
shasum -a 256 -c shint.darwin.arm64.sha256       # macOS; on Linux: sha256sum -c shint.linux.amd64.sha256
```

```text
shint.darwin.arm64: OK
```

On Windows, in PowerShell (`Get-FileHash` uses SHA-256 by default; `-ieq` ignores case):

```bash
$expected = (Get-Content shint.windows.amd64.exe.sha256).Split(' ')[0]
(Get-FileHash shint.windows.amd64.exe).Hash -ieq $expected
```

`True` means it matches. The checksum only proves the file arrived intact: it sits on the same page as the binary, so it cannot tell you the file is genuine.

**A build attestation** does. It is a signed statement, made by the release workflow, that this exact file was built by the `dmartsapp/shint` repository from a named commit. With the [GitHub CLI](https://cli.github.com) installed:

```bash
gh attestation verify shint.darwin.arm64 --repo dmartsapp/shint
```

It succeeds only for a file that is byte-for-byte what that workflow built, and reports what it found (the repository, the workflow and the commit). It needs no key of yours: the signature comes from a short-lived certificate from [Sigstore](https://www.sigstore.dev), recorded in a public log.

:::note This is not operating-system code signing
An attestation proves where a file came from. It does not stop macOS or Windows from warning that the publisher is unknown: shint is not signed with an Apple or Microsoft certificate.
:::

## Docker

Every release is also published as a multi-architecture image (`linux/amd64`, `linux/arm64`) to two registries. The image is tiny: the same static binary on a distroless base, running as a non-root user.

```bash
docker run --rm farhansabbir/shint:latest telnet example.com 443
docker run --rm farhansabbir/shint:latest web https://example.com --json
docker run --rm farhansabbir/shint:latest nmap example.com --from 1 --to 1024 --timeout 1
```

| Registry | Image |
|---|---|
| Docker Hub | `docker.io/farhansabbir/shint` |
| GitHub Container Registry | `ghcr.io/dmartsapp/shint` |

Tags: `latest`, the full version (`4.1.0` and `v4.1.0`), and rolling `4.1` and `4`. Pin a full version in anything you care about.

The listen commands need their port published so you can reach them from outside the container:

```bash
docker run --rm -p 9000:9000/tcp farhansabbir/shint:latest listen tcp 9000
docker run --rm -p 8080:8080/tcp farhansabbir/shint:latest listen http 8080
```

## Build from source

You need [Go](https://go.dev/dl/) (the version in [go.mod](https://github.com/dmartsapp/shint/blob/main/go.mod) or newer) and, optionally, `make`.

```bash
git clone https://github.com/dmartsapp/shint.git
cd shint
go build -o shint .          # a binary for this machine
make linux-amd64             # or any other target, into ./bin
make all-platforms           # every release target
```

From v4.1.0 you can also let Go fetch and build it for you:

```bash
go install github.com/dmartsapp/shint/v4@latest       # puts `shint` in $(go env GOPATH)/bin
```

:::note Releases before v4.1.0
The module path gained its `/v4` suffix in v4.1.0. Earlier tags (`v4.0.4` and older) were published without it, and a published tag is never changed, so `go install` cannot fetch them: for those, use the release binary or build from a clone.
:::

## Shell completion

shint can generate completion scripts for your shell:

```bash
source <(shint completion bash)                              # bash
source <(shint completion zsh)                               # zsh (after compinit)
shint completion fish | source                               # fish
shint completion powershell | Out-String | Invoke-Expression # PowerShell
```

Add the line to your shell's startup file to keep it.

## Upgrade and uninstall

**Upgrade:** download the new file over the old one - there is no state to migrate. **Uninstall:** delete the file. shint writes nothing else to your system.

## Platform notes

- **ping and privileges.** On macOS, the BSDs and Windows, ICMP echo works for a normal user. On Linux it depends on the `net.ipv4.ping_group_range` setting; most desktop distributions allow everyone already, but a hardened system may restrict it to root. If `ping` fails with a permission error, either run it as root or widen the range: `sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"`. Inside Docker the same rules apply, and no extra capabilities should be needed.
- **UDP and port-scan results are best effort.** Neither has a reliable way to tell "nothing is listening" from "a firewall silently dropped the packet", so an `open|filtered` UDP result or an unresponsive TCP port means exactly that - see [troubleshooting](troubleshooting.md).
- **IPv6 listening.** Pass `--bind ::` to the listen commands to listen on IPv6. This is verified to accept IPv4 clients as well on macOS and Linux; other platforms follow Go's networking defaults but have not been verified independently.

## Next

Head to [Using shint](usage.md) for the flags every command shares, or jump to a command: [telnet](telnet.md), [ping](ping.md), [web](web.md), [nmap](nmap.md), [udp](udp.md), [listen](listen.md).
