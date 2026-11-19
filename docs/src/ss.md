---
title: ss
lead: What is listening here, and which process holds it? This machine's TCP and UDP sockets, one line each, with the process that holds each one - no privileges needed.
description: shint ss lists this machine's TCP and UDP sockets, IPv4 and IPv6, one line each with the state, local and remote address, queues, user and the process holding it, on Linux and Windows, without privileges.
section: Commands
order: 13
nav: ss
---

## What it does

"Is anything listening on 5432?", "which process has port 8080?" and "where is this machine connected to?" are questions about *this* machine's sockets. The tools that answer them differ on each system (`ss -tulpn` on Linux, `netstat -ano` on Windows, `lsof -i` elsewhere), and most show the socket in one place and its process in another. `shint ss` gives one line per socket with everything known about it - protocol, state, addresses, queues, user, and the process holding it - in shint's usual format.

It reads the operating system's socket tables and **sends nothing**. It needs no privileges: run as a normal user, it lists every socket, but for a socket that belongs to another user's process it cannot say which process holds it. Such a socket shows `owner=hidden`, and a `WARN` line counts them. Run it as root to see every process.

It works on **Linux** and **Windows**. Other systems are covered under [Platforms](#platforms): most say `ss` is not supported yet, and the AIX binary does not have it at all.

## Syntax

```bash
shint ss [-t | -u] [-l] [--ports LIST] [--pid PID] [-4 | -6] [--json]
```

| Flag | Lists only |
|---|---|
| `-t`, `--tcp` | TCP sockets |
| `-u`, `--udp` | UDP sockets |
| `-l`, `--listening` | listening sockets: TCP in `LISTEN`, UDP with no peer |
| `--ports LIST` | sockets with one of these ports at either end: `22`, `22,80,443`, `8000-8100` (the same lists as [`nmap --ports`](nmap.md)) |
| `--pid PID` | sockets that process holds |
| `-4`, `-6` | IPv4 or IPv6 sockets |

The filters combine: `-l -t --ports 5432` is "a TCP socket listening on port 5432". With none, every TCP and UDP socket is listed. The other [shared flags](usage.md#flags-shared-by-every-command) (`--count`, `--timeout`, `--delay`, ...) mean nothing for a command that never waits, and are ignored.

## What it reports

One `socket` line per socket:

| Field | Meaning |
|---|---|
| `proto` | `tcp` or `udp`. |
| `state` | The TCP state: `LISTEN`, `ESTABLISHED`, `SYN-SENT`, `TIME-WAIT`, `CLOSE-WAIT`, ... For UDP, `UNCONN` (bound, no peer) or `ESTABLISHED` (connected to one peer). Windows does not report a state for UDP, so it is left out there. |
| `local` | The local address and port. `0.0.0.0` and `[::]` mean every address of this machine. |
| `remote` | The peer's address and port. Left out for a socket that has none, such as a listener. |
| `recv_q`, `send_q` | Bytes waiting to be read by the process, and sent but not yet acknowledged by the peer. For a listening socket, `recv_q` is the number of connections waiting to be accepted. Shown only when not zero, since a queue that keeps growing is the interesting case. Linux only. |
| `process`, `pid`, `fd` | The process holding the socket: its name, its process ID and the socket's file descriptor number in it. A socket shared by several processes (a server's workers) has `processes=[nginx(pid=1201,fd=6),nginx(pid=1202,fd=6)]` instead. Windows has no descriptor numbers, so there is no `fd` there. |
| `owner=hidden` | The socket belongs to a process this user may not inspect, so which process holds it is not known. |
| `user` | The user the socket belongs to (`uid` when the user has no name on this machine). Linux only. |

A socket with no process and no `owner=hidden` is one that no process holds: a TCP socket in `TIME-WAIT` stays in the table for a while after its process has closed it.

Then a `done` line counts the sockets listed (`tcp`, `udp`, `listening`, and `owners_hidden` when there are any). When some owners are hidden, a `WARN` line says so. It does not fail the run.

The sockets are sorted: TCP first, then listening sockets before the others, then by local port, so the same machine is listed the same way twice.

## Examples

### What is listening

```bash
shint ss -l
```

```text
Wed Nov 18 00:47:38 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=0.0.0.0:22 owner=hidden user=root
Wed Nov 18 00:47:38 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=[::]:22 owner=hidden user=root
Wed Nov 18 00:47:38 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=127.0.0.1:5432 owner=hidden user=postgres
Wed Nov 18 00:47:38 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=127.0.0.1:8500 process=consul pid=17929 fd=18 user=deploy
Wed Nov 18 00:47:38 CST 2026: [ss] OK socket proto=udp state=UNCONN local=0.0.0.0:5353 owner=hidden user=avahi
Wed Nov 18 00:47:38 CST 2026: [ss] OK socket proto=udp state=UNCONN local=127.0.0.1:8600 process=consul pid=17929 fd=13 user=deploy
Wed Nov 18 00:47:38 CST 2026: [ss] OK done sockets=6 tcp=4 udp=2 listening=6 owners_hidden=4 total_time=5.692163ms
Wed Nov 18 00:47:38 CST 2026: [ss] WARN owners hidden sockets=4 warning="these sockets belong to processes this user may not inspect, so which process holds them is not shown; run as root to see it"
```

Run as the user `deploy`, it shows the process for that user's Consul agent. The SSH, PostgreSQL and mDNS sockets belong to other users, so it lists them with their user but without the process. Run as root, every socket gets its process:

```bash
sudo shint ss -l -t
```

```text
Wed Nov 18 00:48:02 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=0.0.0.0:22 process=sshd pid=812 fd=3 user=root
Wed Nov 18 00:48:02 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=0.0.0.0:80 processes=[nginx(pid=1201,fd=6),nginx(pid=1202,fd=6)] user=root
Wed Nov 18 00:48:02 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=[::]:22 process=sshd pid=812 fd=4 user=root
Wed Nov 18 00:48:02 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=127.0.0.1:5432 process=postgres pid=944 fd=7 user=postgres
Wed Nov 18 00:48:02 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=127.0.0.1:8500 process=consul pid=17929 fd=18 user=deploy
Wed Nov 18 00:48:02 CST 2026: [ss] OK done sockets=5 tcp=5 udp=0 listening=5 total_time=4.120511ms
```

nginx's listener is one socket held by two worker processes, so it is one line naming both.

### Is anything listening on a port?

```bash
shint ss -l --ports 5432 > /dev/null && echo "postgres is up" || echo "nothing on 5432"
```

When nothing matches, `ss` fails (exit status `1`) and says what it looked for:

```text
Wed Nov 18 00:49:10 CST 2026: [ss] ERROR failed error="no socket matches: listening, port 5432"
```

So a script, a health check or a container entrypoint can wait for a port with nothing but shint.

### Where is this machine connected to?

```bash
shint ss -t --ports 443
```

```text
Wed Nov 18 00:50:21 CST 2026: [ss] OK socket proto=tcp state=ESTABLISHED local=192.168.1.20:49526 remote=140.82.112.3:443 process=git-remote-http pid=30211 fd=5 user=deploy
Wed Nov 18 00:50:21 CST 2026: [ss] OK socket proto=tcp state=TIME-WAIT local=192.168.1.20:49518 remote=140.82.112.3:443
Wed Nov 18 00:50:21 CST 2026: [ss] OK done sockets=2 tcp=2 udp=0 listening=0 total_time=3.011207ms
```

`--ports` matches either end, so this lists the connections *to* port 443 elsewhere as well as anything listening on 443 here. To see what one process has open, use `--pid`:

```bash
shint ss --pid 17929
```

### Windows

```powershell
shint.exe ss -l -t
```

```text
Wed Nov 18 00:52:40 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=0.0.0.0:135 process=svchost.exe pid=1092
Wed Nov 18 00:52:40 CST 2026: [ss] OK socket proto=tcp state=LISTEN local=0.0.0.0:445 process=System pid=4
Wed Nov 18 00:52:40 CST 2026: [ss] OK done sockets=2 tcp=2 udp=0 listening=2 total_time=2.402115ms
```

Windows tells any user which process owns every socket, so nothing is hidden there and no elevation is needed. `System` (PID 4) is the kernel itself, which serves file sharing (445) and `http.sys`. Windows has no file descriptor numbers, user or queue sizes in its socket table, and no state for UDP sockets.

### JSON

```bash
shint ss -l -t --ports 80 --json
```

```json
{
  "input_params": { "module_name": "ss", "protocol": "tcp", "ports": "80", "listening": true, ... },
  "module_name": "ss",
  "stats": [
    {
      "protocol": "tcp",
      "family": "ipv4",
      "state": "LISTEN",
      "local_address": "0.0.0.0",
      "local_port": 80,
      "recv_q": 0,
      "send_q": 0,
      "uid": 0,
      "user": "root",
      "processes": [
        { "pid": 1201, "name": "nginx", "fd": 6 },
        { "pid": 1202, "name": "nginx", "fd": 6 }
      ]
    }
  ],
  "end_time_unixtime_µs": ...,
  "start_time_unixtime_µs": ...,
  "total_time_taken_µs": ...,
  "error": ""
}
```

One object per socket, holding everything about it. A field a platform does not report is left out rather than written as zero: `recv_q`, `send_q`, `uid` and `user` on Windows, `fd` on Windows, `state` for UDP on Windows. So `"uid": 0` always means root, never "unknown". `remote_address` and `remote_port` are left out for a socket with no peer. `processes` is left out when no process holds the socket, and when `"owner_hidden": true` says they could not be seen. Like [`ip`](ip.md), the document has no `dns_lookup`: nothing is looked up. `input_params` records the filters: `protocol` (`tcp`, `udp` or `tcp,udp`), `ports`, `listening` and `pid`. See [Output](output.md#stats-entries) for every field.

## Platforms

| System | `ss` |
|---|---|
| Linux | Supported: it reads `/proc`. Android too, although apps built for Android 10 and later may not read the socket tables. |
| Windows | Supported. Any user may see which process holds every socket, so nothing is hidden. |
| macOS, FreeBSD, NetBSD, OpenBSD, Solaris, illumos | Not supported yet: `ss` says so and exits `1`. Planned: the BSDs ([#105](https://github.com/dmartsapp/shint/issues/105)), Solaris 11 ([#113](https://github.com/dmartsapp/shint/issues/113)) and illumos ([#114](https://github.com/dmartsapp/shint/issues/114)). |
| AIX | **Not in the AIX binary.** AIX has no documented way to list its sockets, with the process holding each, without root. Its own tools (`netstat -A` with `rmsock`, and `lsof`) read kernel memory as root. Its `/proc/<pid>/fd` cannot open a socket, nor say that an entry is one. shint never asks for root, so `ss` is left out of the AIX binary rather than shipped as a command that can never work: `shint ss` there is an unknown command (exit `2`). |

## Exit status

- `0` when at least one socket was listed.
- `1` when nothing matched the filters, the socket tables could not be read, or the system is not supported yet.
- `2` for using it wrongly: an argument, `-4` and `-6` together, a `--ports` list that is not one, or a `--pid` that is not a number of 1 or more. On AIX, `ss` itself, which the AIX binary does not have.

## Good to know

- **Containers have their own sockets.** A container with its own network namespace (the Docker default) has its own socket table, and `ss` lists the one it runs in. On the host you see Docker's published ports, often held by `docker-proxy`, but not what listens inside a container. Run `shint ss` inside the container for that.
- **A dual-stack listener is IPv6.** A server listening on `[::]` usually accepts IPv4 too, but the socket is an IPv6 one, so `-4` does not list it.
- **Process names come from the process.** On Linux the name is the kernel's short name for the process, at most 15 characters, and any process can set its own. So a name is printed escaped: a control character in it cannot act on your terminal.
- **It is a snapshot.** A socket opened or closed while the tables are read may be missing or listed in its old state. On a busy server, run it twice before drawing conclusions.
- **Only TCP and UDP.** Unix domain sockets, raw sockets (`ping`'s) and other kinds of file are not listed: this is a network tool, not `lsof`.
- **`owner=hidden` is not an error.** It is the honest answer for a normal user. On Linux, root (or a process with the right capability) sees every owner.
- **It does not change anything.** There is no way to close a socket or stop a process with shint.
