---
title: listen
lead: Start a TCP, UDP or HTTP server on your own machine, so you can test firewalls, load balancers, monitoring and the other shint commands without touching anything real.
description: shint listen runs a local TCP, UDP or minimal JSON HTTP listener that reports every connection, packet or request, with byte counts.
section: Commands
order: 10
nav: listen
---

## What it does

`shint listen` opens a port and reports everything that arrives. It is the counterpart of the other commands: point [`telnet`](telnet.md), [`nmap`](nmap.md), [`udp`](udp.md) or [`web`](web.md) at it (from the same machine or another one) and you get a known, controllable target to check your network, firewall rules, proxies and health checks against.

There are three modes:

| Command | Behaviour |
|---|---|
| `shint listen tcp <port>` | Accepts TCP connections, reports each chunk received, optionally echoes it back. |
| `shint listen udp <port>` | Receives UDP datagrams, reports each one, optionally echoes it back. |
| `shint listen http <port>` | A minimal HTTP server: any method on `/` answers `{"status":"ok"}`; any other path answers 404 `{"status":"not found"}`. |

## Syntax

```bash
shint listen tcp|udp|http <port> [--bind ADDR] [--echo] [--count N] [--timeout S] [--json]
```

| Flag | Default | Meaning |
|---|---|---|
| `--bind ADDR` | `0.0.0.0` | Address to listen on. Use `127.0.0.1` for this machine only, or `::` for IPv6. |
| `--echo` | off | Send received data back to the sender (`tcp` and `udp`). |
| `--count N` | `0` | Stop after N connections / packets / requests. `0` means keep going until `Ctrl+C`. |
| `--timeout S` | `5` | Close a connection that has been quiet this long. `0` disables it. |
| `--json` | off | Print one JSON line per event instead of log lines. |

Unlike every other command, `listen` runs until you stop it (its `--count` defaults to `0`), because leaving a server up is usually the point. A summary is printed when it exits.

## TCP

```bash
shint listen tcp 9002 --echo --count 1
```

Send it something from another terminal:

```bash
printf 'hello shint\n' | nc 127.0.0.1 9002
```

```text
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK listening address=0.0.0.0:9002 max_connections=1 echo=true
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK connection accepted remote=127.0.0.1:59324 local=127.0.0.1:9002
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK data received remote=127.0.0.1:59324 bytes_received=12 bytes_sent=12 time_taken=28.708µs preview="hello shint"
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK connection closed remote=127.0.0.1:59324 bytes_received=12 bytes_sent=12
Sun Sep 20 01:50:27 MDT 2026: [listen-tcp] OK done connections=1 bytes_received=12 bytes_sent=12 total_time=410.53875ms
```
For each read, `bytes_received` is what arrived, `bytes_sent` is what was echoed back (0 without `--echo`), and `time_taken` is how long handling it took. With `--json` each read is one line (this run listened on port 9003 and was sent `hello-json`):

```json
{"protocol":"tcp","remote_address":"127.0.0.1:59327","local_address":"127.0.0.1:9003","bytes_read":11,"bytes_sent":11,"processing_time_µs":25,"preview":"hello-json","unixtime_µs":1789890629205451}
```
A plain `shint telnet 127.0.0.1 9002` connects and closes without sending anything, so it shows up as a connection with zero bytes.

## UDP

```bash
shint listen udp 9001 --echo --count 2
```

Probe it from another terminal (`shint udp 127.0.0.1 9001 --data hello`, and again with `--payload 16`):

```text
Sun Sep 20 01:50:30 MDT 2026: [listen-udp] OK listening address=0.0.0.0:9001 max_packets=2 echo=true
Sun Sep 20 01:50:31 MDT 2026: [listen-udp] OK packet received remote=127.0.0.1:56126 bytes=5 preview=hello
Sun Sep 20 01:50:32 MDT 2026: [listen-udp] OK packet received remote=127.0.0.1:52727 bytes=16 preview=dddddddddddddddd
Sun Sep 20 01:50:32 MDT 2026: [listen-udp] OK done packets=2 bytes_received=21 total_time=2.434746875s
```
## HTTP

```bash
shint listen http 8080
```

Call it with `curl`, a browser, a load balancer's health check, or [`shint web`](web.md):

```bash
shint web http://127.0.0.1:8080/
shint web http://127.0.0.1:8080/missing
shint web -X POST -P '{"name":"shint"}' -H 'Content-Type: application/json' http://127.0.0.1:8080/
```

Each request is reported once it is complete:

```text
Sun Sep 20 01:50:18 MDT 2026: [listen-http] OK listening address=0.0.0.0:8080 max_requests=6
Sun Sep 20 01:50:19 MDT 2026: [listen-http] OK request method=GET path=/ status=200 remote=127.0.0.1:59313 bytes_received=97 bytes_sent=160 time_taken=504.375µs
Sun Sep 20 01:50:20 MDT 2026: [listen-http] OK request method=GET path=/missing status=404 remote=127.0.0.1:59314 bytes_received=104 bytes_sent=175 time_taken=340.042µs
Sun Sep 20 01:50:21 MDT 2026: [listen-http] OK request method=POST path=/ status=200 remote=127.0.0.1:59315 bytes_received=189 bytes_sent=160 time_taken=85.583µs
Sun Sep 20 01:50:21 MDT 2026: [listen-http] OK request method=GET path=/ status=200 remote=127.0.0.1:59316 bytes_received=97 bytes_sent=160 time_taken=73.291µs
Sun Sep 20 01:50:22 MDT 2026: [listen-http] OK request method=GET path=/ status=200 remote=127.0.0.1:59317 bytes_received=97 bytes_sent=160 time_taken=150.042µs
Sun Sep 20 01:50:23 MDT 2026: [listen-http] OK request method=GET path=/ status=200 remote=127.0.0.1:59318 bytes_received=97 bytes_sent=160 time_taken=139.875µs
Sun Sep 20 01:50:23 MDT 2026: [listen-http] OK done requests=6 bytes_received=681 bytes_sent=975 total_time=5.459357042s
```
`bytes_received` and `bytes_sent` are the raw bytes on the connection - request line, headers and body in; status line, headers and body out - counted the same way [`web`](web.md#bytes-sent-and-received) counts its own. Compare the first request above (97 in, 160 out) with the client's output for the same request: they match exactly. `time_taken` is how long the server spent handling the request.

With `--json`, one line per request:

```json
{"method":"GET","path":"/","status_code":200,"remote_address":"127.0.0.1:59321","bytes_received":97,"bytes_sent":160,"processing_time_µs":189,"unixtime_µs":1789890626321434}
```
### What the HTTP listener does not do

It is deliberately dumb, so it behaves the same every time:

- It uses only the request line (method and path) to decide between `200` and `404`. **It never parses or acts on headers or bodies** - conditional headers such as `If-None-Match`, `Range` or `Accept-Encoding` change nothing, and the response carries no `ETag` or `Last-Modified`. The request body is read and discarded unseen, so a large upload finishes before the answer and the byte count is complete.
- Every response closes the connection (`Connection: close`), so each connection carries exactly one request.
- There is no request echo: the answer is always the small fixed status document.

## Good to know

- **The `preview` is always one safe line.** Plain text is shown as it is; anything else is shown escaped - `\n`, `\r`, `\t`, `\xNN` for other control characters and for bytes that are not valid text, `\uNNNN` for other non-printable characters - and long payloads are cut at 120 bytes with `...`. A binary packet (see [`wol`](wol.md) for a real one) cannot garble your terminal, and a payload cannot pass off a line break or an escape sequence as your terminal's or the log's own.
- **Ports below 1024** normally need administrator rights (or a capability) on Linux; other systems vary. Use a high port such as 8080 or 9000 to avoid the question.
- **Firewalls.** macOS and Windows may ask whether to allow incoming connections the first time; choose the network scope you intend. Bind to `127.0.0.1` when you only want to test locally.
- **IPv6.** `--bind ::` listens on IPv6 (and, on macOS and Linux, IPv4 as well).
- **Docker.** Publish the port so it is reachable from outside: `docker run --rm -p 8080:8080 farhansabbir/shint listen http 8080`.
- **Exit status** is `0` when the listener finishes normally (its `--count` reached, or `Ctrl+C`) and `1` if the port cannot be bound, for example because it is already in use.
