---
title: Troubleshooting & FAQ
lead: The things that most often surprise people, and what they mean.
description: Answers to common shint questions - timeouts vs refusals, ping permissions, slow scans, UDP results, TLS errors, exit codes and privacy.
section: Guides
order: 2
nav: Troubleshooting & FAQ
---

## "Connection refused" or "i/o timeout"?

They mean different things:

- **`connection refused`** - the target machine answered, and said nothing is listening on that port. The host is up and reachable; the service is stopped or on a different port.
- **`i/o timeout`** - no answer at all within `--timeout`. Usually a firewall silently dropping packets, a wrong address, or a host that is down.
- **`no route to host` / `network is unreachable`** - this machine has no path to that address. For an IPv6 address, your network probably has no IPv6.

## A scan takes a long time

`nmap` waits up to `--timeout` (default 5 seconds) for *each* port that does not answer, with up to 500 ports at once. A host that drops packets makes every port wait the full timeout - about 90 seconds for 9,000 ports. Add `--timeout 1` on a fast network. The progress lines show the scan is working. See [nmap](nmap.md#how-long-will-it-take).

## A scan says a port is closed, but the service is running

TCP scanning cannot tell a stopped service from a firewall that drops packets - both simply produce no connection. Check from a machine on the same network as the service, and make sure the service listens on the address you are scanning (a service bound to `127.0.0.1` is invisible from other machines).

## ping says permission denied (Linux)

Unprivileged ICMP is controlled by `net.ipv4.ping_group_range`. Run as root, or allow everyone:

```bash
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
```

macOS, the BSDs and Windows do not need this.

## ping and telnet report an IPv6 failure but the host works

A name with both IPv4 and IPv6 addresses is tested over both, and each is reported separately. If this machine cannot use IPv6, the IPv6 attempt fails while the IPv4 one succeeds, and the exit status is `1` because one of the checks failed. Two errors look like this:

| Error | Meaning |
|---|---|
| `connect: network is unreachable` | The machine has IPv6, but no route to the IPv6 internet (common on networks that only provide IPv4). |
| `socket: address family not supported by protocol` | The machine cannot create an IPv6 socket at all: its kernel has IPv6 **disabled** (for example booted with `ipv6.disable=1`). A Docker container shares its host's kernel, so it inherits this and Docker cannot fix it. |

That is real information about the machine, not a fault in the host you are checking. When IPv4 is all you care about, ask for exactly that:

```bash
shint telnet google.com 443 -4
```

`-4` resolves and checks only IPv4 addresses (`-6` does the reverse); see [IPv4 only, or IPv6 only](usage.md#ipv4-only-or-ipv6-only). shint also recognises these two errors on an IPv6 address and adds a hint to the error line - `(this system cannot use IPv6; use -4 to check IPv4 only)` - so you do not have to decode the errno.

Before v4.2.0 there was no flag: the workaround was to pass the IPv4 address itself instead of the name.

## UDP says `open|filtered`

UDP gives no acknowledgement, so silence is ambiguous: the service may simply not reply to input it does not understand, or a firewall may be dropping the packet. Send a payload the service understands with `--data`. `closed` (the host explicitly said "port unreachable") is the only definite negative.

## "certificate is not trusted" / "x509: certificate signed by unknown authority"

`web` verifies certificates by default. For a service that uses an internal CA, pass `--cacert ca.pem`. For a quick, unverified look, pass `-k` - but the answer cannot then be trusted. See [web](web.md#https-and-certificates).

## It works with curl or my browser but not with shint (behind a proxy)

`shint web` never uses a proxy: `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` are ignored, and the connection goes straight to the address the name resolves to. On a network where outside access is only possible through a proxy, that connection is blocked, so `curl` (which honours those variables) succeeds while `shint web` reports `connect failed` or `i/o timeout`. That is the honest answer to "can this machine reach that server directly?" - which is what shint asks - so it points at the network, not at the target. To check the target through the proxy, use `curl`; to check the proxy itself, `shint telnet <proxy-host> <proxy-port>`. Go skips proxies for loopback addresses anyway, so this cannot be seen against `localhost`.

## Why does my script think a 500 error succeeded?

`web` reports the HTTP status but treats any response as a completed request (as `curl` does without `-f`), so a 404 or 500 exits `0`. Check the status yourself:

```bash
shint web --json https://example.com/ | jq -e '.stats[0].status_code < 400'
```

## What are the exit codes?

`0` every check passed, `1` at least one check failed, `2` the command was used wrongly. See [Exit status](usage.md#exit-status) for exactly what counts as a failure for each command.

## A check takes a second even when it is instant

`--delay` (default 1000 ms) is applied before each attempt, including the first. Use `--delay 0`.

## macOS says the file "cannot be opened because the developer cannot be verified"

A file downloaded through a browser is quarantined. Clear the flag once: `xattr -d com.apple.quarantine shint`. Files downloaded with `curl` are not flagged.

## The listener asks to allow incoming connections

The operating system's firewall is asking whether `shint` may accept connections from the network. Allow it if you want other machines to reach it, or start the listener with `--bind 127.0.0.1` to keep it local.

## "address already in use"

Another program owns that port. Pick another, or find the owner (`lsof -i :8080` on macOS and Linux). `shint listen` exits with status `1` in this case.

## Does shint collect any data?

No. shint has no telemetry, accounts or configuration files. It sends only what the check itself needs - DNS queries, TCP connections, UDP datagrams, ICMP echo requests and the HTTP requests you ask for - to the addresses you name.

## Where do I report a problem?

Open an issue at [github.com/dmartsapp/shint/issues](https://github.com/dmartsapp/shint/issues) with the command you ran, its output (with `--json` if possible) and the version from `shint --version`.
