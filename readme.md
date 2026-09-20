# that SHIt Network Tool - aka shint, (v4.x.y)

## Introduction

A simple, modern, and versatile network utility tool built with Go. It bundles a handful of small diagnostics that are normally reached for as separate programs — a `telnet`-style TCP connectivity check, a basic ICMP `ping`, a `wget`/`curl`-style HTTP(S) client, a limited TCP-only `nmap` port scanner, a `udp` probe, and local `listen` servers for testing the others without needing a real remote endpoint.

**📦 [Download the latest release](https://github.com/dmartsapp/shint/releases/latest)** — prebuilt binaries for Linux, macOS, Windows, FreeBSD, OpenBSD, NetBSD, Solaris, and Android.

Every check below runs independently on each tagged release (`.github/workflows/*.yaml`), so each row is that specific stage's own live status, not one combined pass/fail:

| Check | Status |
|---|---|
| Lint (`golangci-lint`) | [![Lint](https://github.com/dmartsapp/shint/actions/workflows/lint.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/lint.yaml) |
| Vulnerability check (`govulncheck`) | [![Vulnerability Check](https://github.com/dmartsapp/shint/actions/workflows/vulncheck.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/vulncheck.yaml) |
| Binary build (14 platforms) | [![Binary Build & Release](https://github.com/dmartsapp/shint/actions/workflows/build.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/build.yaml) |
| Release | [![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest) |
| Docker Hub | [![Docker Hub](https://github.com/dmartsapp/shint/actions/workflows/docker-hub.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/docker-hub.yaml) [![Docker Hub version](https://img.shields.io/docker/v/farhansabbir/shint?label=%3A&sort=semver&logo=docker)](https://hub.docker.com/r/farhansabbir/shint) |
| GitHub Container Registry (GHCR) | [![GHCR](https://github.com/dmartsapp/shint/actions/workflows/ghcr.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/ghcr.yaml) [![GHCR image](https://img.shields.io/badge/image-dmartsapp%2Fshint-blue?logo=github)](https://github.com/dmartsapp/shint/pkgs/container/shint) |

"Binary build" and "Docker Hub"/"GHCR" each re-run the lint/vulnerability gate internally before building anything (so none of them ship a binary or image if either would fail), rather than depending on the separate Lint/Vulnerability Check workflows above finishing first - see [Development](#development) for why.

**Note:** Version 3.0.0 added `udp`, `listen tcp`/`listen udp`, and authenticated-TLS options on `web` (`--cacert`/`--cert`/`--key`/`--insecure`), fixed several correctness/race bugs from v2, and unified the text-mode log output across every command; v3.1.0 added `listen http`; v4.0.0 added full dual-stack IPv6 support across every command; v4.0.1 added per-request timing/byte metrics to `listen tcp`/`listen http` and fixed the `web` bandwidth calculation; v4.0.2 makes `web` and `listen http` measure bytes identically (everything on the wire, headers included). See [Changelog](#changelog) for the full list.

## Features

- **Telnet:** Test TCP connectivity to a host on a specific port.
- **Ping:** Send ICMP ECHO_REQUEST packets to a host to test reachability (no root/admin required on macOS/Linux/BSD in the common case — see [Platform notes](#platform-notes)).
- **Web:** Make an HTTP/HTTPS request to a URL and display the response, including a small REST-client mode (method, headers, body) and authenticated-TLS options (custom CA, client certificate for mTLS, or skip verification entirely).
- **Nmap:** Scan for open TCP ports on a host within a given range.
- **UDP:** Send a UDP probe to a host/port and classify the result (`open`, `closed`, or `open|filtered`), the same way a basic UDP port scan works, since UDP has no handshake to confirm a listener.
- **Listen:** Start a local `tcp`, `udp`, or `http` listener so `telnet`, `udp`, `web`, and `nmap` can be exercised end-to-end when there's no real server to test against. `listen http` is a minimal JSON status endpoint that accepts any method on `/` and 404s everywhere else — handy for both plain TCP and HTTP-level checks against the same process.
- **JSON Output:** Every command supports `--json` for machine-readable output.
- **Cross-Platform:** Static binaries for Linux, macOS, Windows, FreeBSD, OpenBSD, NetBSD, Solaris, and Android — 14 OS/architecture combinations in total. See [Supported platforms](#supported-platforms).

## Installation

Download the latest release for your platform from the [releases page](https://github.com/dmartsapp/shint/releases/latest).

Alternatively, build from source (requires Go, see `go.mod` for the minimum version):

```bash
git clone https://github.com/dmartsapp/shint.git
cd shint
make            # builds linux/darwin/windows (amd64+arm64) into bin/
make all-platforms  # also builds freebsd/openbsd/netbsd/solaris/android
make run        # quick local build + run without arguments
```

Or run it straight from the container image, no Go toolchain needed:

```bash
docker run --rm farhansabbir/shint:latest telnet google.com 443
```

See [Docker image](#docker-image) for available tags and details.

## Global flags

These flags are defined once on the root command and apply to every subcommand, though their meaning is adapted for the `listen` commands (noted below):

| Flag | Default | Meaning |
|---|---|---|
| `--count` | `1` | Number of iterations to run. For `listen tcp`/`listen udp`, this is instead the max connections/packets to accept before exiting; `0` means run until Ctrl+C. |
| `--timeout` | `5` | Seconds to wait for a connection/response. For `listen tcp`/`listen udp`, this is the idle read timeout per connection/packet loop; `0` means no timeout. |
| `--delay` | `1000` | Milliseconds to wait between iterations. |
| `--payload` | `4` | Payload size in bytes for `ping` and `udp` (filler content; ignored by `udp` if `--data` is given). |
| `--throttle` | `false` | Randomize the delay between iterations (0-10000ms) instead of using a fixed `--delay`. |
| `--json` | `false` | Emit machine-readable JSON instead of the human-readable log lines described below. |

## Logging format

Every command's human-readable (non-`--json`) output follows the same shape, so the stream is easy to read or grep regardless of which command produced it:

```
<timestamp>: [<module>] OK|ERROR <message> key=value key=value ...
```

For example:

```
Mon Jun 30 13:23:26 EDT 2025: [telnet] OK connect ok host=142.251.41.46 port=443 attempt=1/1 time=7.076ms
Mon Jun 30 13:23:27 EDT 2025: [web] ERROR request failed url=https://example.invalid/ attempt=1/1 time=1.2s error="dial tcp: lookup example.invalid: no such host"
```

Each command that runs more than a single request also prints a shared summary banner at the end:

```
======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 7.076ms, average: 7.076ms, maximum: 7.076ms
```

`--json` output has its own stable schema per command (shown in each section below) and does not include these log lines.

## Usage

### Telnet

Attempts a TCP connection to a host on a specific port.

**Syntax:**

```bash
./shint telnet [host] [port]
```

**Example:**

```bash
./shint telnet google.com 443
```

**Output:**

```
Mon Jun 30 13:23:25 EDT 2025: [telnet] OK dns resolved host=google.com addresses=1 ips=[142.251.41.46] time=13.048166ms
Mon Jun 30 13:23:26 EDT 2025: [telnet] OK connect ok host=142.251.41.46 port=443 attempt=1/1 time=7.076ms

======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 7.076ms, average: 7.076ms, maximum: 7.076ms
Mon Jun 30 13:23:26 EDT 2025: [telnet] OK done total_time=1.021345708s
```

**JSON Output:**

```json
{
  "input_params": {
    "module_name": "telnet",
    "sequential": false,
    "throttle": false,
    "host": "google.com",
    "from_port": 443,
    "to_port": 443,
    "protocol": "tcp",
    "timeout_ms": 5,
    "count": 1,
    "delay_ms": 1000,
    "payload_bytes": 4
  },
  "module_name": "telnet",
  "dns_lookup": {
    "hostname": "google.com",
    "resolved_addresses": ["142.251.41.46"],
    "error": "",
    "success": true,
    "time_taken_µs": 11595
  },
  "stats": [
    {
      "address": "142.251.41.46",
      "success": true,
      "recv_unixtime_µs": 1751305205592777,
      "sent_unixtime_µs": 1751305205584764,
      "time_taken_µs": 8013
    }
  ],
  "end_time_unixtime_µs": 1751305205592783,
  "start_time_unixtime_µs": 1751305204572105,
  "total_time_taken_µs": 1020678,
  "error": ""
}
```

### Ping

Sends ICMP ECHO_REQUEST packets to a host to test reachability. Resolves both IPv4 and IPv6 addresses by default - a dual-stack hostname is pinged over **both** protocols in the same run, each shown as its own log line/stat entry (see the `(ipv4)`/`(ipv6)` tag on each line below).

**Syntax:**

```bash
./shint ping [host]
```

**Example:**

```bash
./shint ping google.com --count 2
```

**Output** (google.com is dual-stack, so both addresses get pinged every iteration):

```
Sat Sep 19 11:38:09 MDT 2026: [icmp] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.46.78] time=143.623541ms
Sat Sep 19 11:38:09 MDT 2026: [icmp] OK received reply for request #1 from 142.251.46.78 (ipv4) in 27ms
Sat Sep 19 11:38:09 MDT 2026: [icmp] OK received reply for request #1 from 2607:f8b0:400a:803::200e (ipv6) in 31ms
Sat Sep 19 11:38:10 MDT 2026: [icmp] OK received reply for request #2 from 142.251.46.78 (ipv4) in 29ms
Sat Sep 19 11:38:10 MDT 2026: [icmp] OK received reply for request #2 from 2607:f8b0:400a:803::200e (ipv6) in 29ms

========================================= icmp STATISTICS =========================================
Requests sent: 4, Response received: 4, Success: 100%
Latency: minimum: 27ms, average: 29ms, maximum: 31ms
Sat Sep 19 11:38:10 MDT 2026: [icmp] OK done packets_lost=0 stddev_ms=1.414 resolve_time=143.623541ms total_time=1.063519208s
```

**JSON Output:**

```json
{
  "input_params": {
    "module_name": "icmp",
    "host": "google.com",
    "from_port": 7,
    "to_port": 7,
    "protocol": "icmp",
    "timeout_ms": 5,
    "count": 1,
    "delay_ms": 1000,
    "payload_bytes": 4
  },
  "module_name": "icmp",
  "dns_lookup": {
    "hostname": "google.com",
    "resolved_addresses": ["142.251.41.46"],
    "success": true,
    "time_taken_µs": 1795
  },
  "stats": [
    {
      "address": "142.251.41.46",
      "success": true,
      "sequence": 1,
      "payload_size_bytes": 0,
      "recv_unixtime_ms": 1751305210636,
      "sent_unixtime_ms": 1751305210626,
      "time_taken_ms": 10
    }
  ],
  "end_time_unixtime_µs": 1751305210636123,
  "start_time_unixtime_µs": 1751305209622907,
  "total_time_taken_µs": 1013216,
  "error": ""
}
```

### Web

Makes an HTTP(S) request to a URL and displays the response. Does not follow redirects or fetch embedded resources. If the URL has no scheme, `https://` is assumed.

**Syntax:**

```bash
./shint web [url] [flags]
```

**Example (simple GET):**

```bash
./shint web https://google.com
```

**Output:**

```
Sat Sep 19 23:31:30 MDT 2026: [web] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.46.78] time=3.298084ms
Sat Sep 19 23:31:31 MDT 2026: [web] OK response url=https://google.com status=200 bytes_sent=219 bytes_received=31326 speed=87.31KB/s attempt=1/1 time=350.379125ms

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 350.379125ms, average: 350.379125ms, maximum: 350.379125ms
Sat Sep 19 23:31:31 MDT 2026: [web] OK done total_time=1.355749917s
```

#### Bytes sent and received

`bytes_sent` and `bytes_received` (the same names in the log line and in `--json`) are everything that actually crossed the connection for that request, headers included - not just the bodies: the request line, headers and body going out, and the status line, headers and body coming back (chunked-encoding framing included), summed over every hop if a redirect is followed (which is why the `https://google.com` example above sent 219 bytes: two requests). Over HTTPS they are counted on the decrypted side, i.e. the HTTP bytes, not the TLS records. Since that includes headers `web` adds on its own (`Host`, `User-Agent`, `Accept-Encoding`, `Content-Length`), the figure is larger than the headers and body visible in the JSON output, which lists them separately.

`listen http` counts its own traffic the same way, so `web`'s `bytes_sent` equals the listener's `bytes_received` and `web`'s `bytes_received` equals the listener's `bytes_sent` for the same request.

#### REST client flags

*   `-X`, `--method`: HTTP method to use (e.g. `GET`, `POST`, `PUT`, `DELETE`). Defaults to `GET`.
*   `-P`, `--payload`: Request body to send.
*   `-H`, `--header`: A header to include; repeatable (e.g. `-H "Content-Type: application/json" -H "Authorization: Bearer <token>"`).
*   `-W`, `--withbody`: Include the full response body in the JSON output.

**Example (POST with JSON body):**

```bash
./shint web -X POST -P '{"name": "test"}' -H "Content-Type: application/json" --json -W https://httpbin.org/post
```

#### TLS flags (authenticated HTTPS)

*   `--cacert <file>`: PEM CA bundle to trust in addition to the system pool, for verifying a self-signed or internally-issued server certificate.
*   `--cert <file>` / `--key <file>`: PEM client certificate/key pair presented to the server for mutual TLS (mTLS). Both must be given together.
*   `-k`, `--insecure`: Skip TLS certificate verification entirely (like `curl -k`). Only for quick diagnostics — the response cannot be trusted.

**Example (mTLS against a service behind an internal CA):**

```bash
./shint web https://internal.example.com \
  --cacert ca.pem \
  --cert client.pem --key client-key.pem
```

If `--cacert` is omitted and the server's certificate isn't already trusted by the system, the request fails with an `x509: certificate signed by unknown authority`-style error — that's expected; either supply `--cacert` or pass `--insecure` if you just need a quick, unverified check.

### Nmap

Scans for open TCP ports on a host within a given range. Concurrency is capped internally (500 simultaneous dial attempts) so scanning a wide range like `1-65535` won't exhaust local file descriptors.

**Syntax:**

```bash
./shint nmap --from [start_port] --to [end_port] [host]
```

**Example:**

```bash
./shint nmap --from 80 --to 100 google.com
```

**Output:**

```
Mon Jun 30 13:23:42 EDT 2025: [nmap] OK dns resolved host=google.com addresses=1 ips=[142.251.41.46] time=1.585083ms
Mon Jun 30 13:23:42 EDT 2025: [nmap] OK port open host=142.251.41.46 port=80
Mon Jun 30 13:23:47 EDT 2025: [nmap] OK scan complete ports_scanned=21 open=1 time=5.001s
Mon Jun 30 13:23:47 EDT 2025: [nmap] OK done total_time=5.002086541s
```

**JSON Output:**

```json
{
  "input_params": {
    "module_name": "nmap",
    "host": "google.com",
    "from_port": 80,
    "to_port": 100,
    "protocol": "tcp",
    "timeout_ms": 5,
    "count": 1
  },
  "module_name": "nmap",
  "dns_lookup": {
    "hostname": "google.com",
    "resolved_addresses": ["142.251.41.46"],
    "success": true,
    "time_taken_µs": 1826
  },
  "stats": [
    { "address": "142.251.41.46", "port": 80, "success": true },
    { "address": "142.251.41.46", "port": 100, "success": false }
  ],
  "end_time_unixtime_µs": 1751305293221854,
  "start_time_unixtime_µs": 1751305288219383,
  "total_time_taken_µs": 5002471,
  "error": ""
}
```

### UDP

Sends a UDP datagram to a host/port and classifies the outcome. Because UDP has no handshake, this works the same way a basic UDP port scan does:

- **`open`** — a reply datagram was received before the timeout.
- **`closed`** — the OS surfaced an ICMP port-unreachable for the connected socket (seen as a "connection refused" style read error).
- **`open|filtered`** — nothing came back before the timeout. This is the common outcome for UDP services that silently ignore unexpected input, so it can't be told apart from a firewall dropping the packet.

**Syntax:**

```bash
./shint udp [host] [port] [flags]
```

*   `-D`, `--data`: Explicit payload to send instead of the generated `--payload`-sized filler.

**Example:**

```bash
./shint udp 127.0.0.1 19192 --data "hello"
```

**Output:**

```
Sat Sep 19 01:20:12 MDT 2026: [udp] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=152.958µs
Sat Sep 19 01:20:13 MDT 2026: [udp] OK probe open host=127.0.0.1 port=19192 attempt=1/1 sent=5 received=5 time=4.105708ms
Sat Sep 19 01:20:13 MDT 2026: [udp] OK done probes_sent=1 open=1 total_time=1.006498875s
```

**JSON Output:**

```json
{
  "input_params": { "module_name": "udp", "host": "127.0.0.1", "from_port": 19192, "to_port": 19192, "protocol": "udp" },
  "module_name": "udp",
  "dns_lookup": { "hostname": "127.0.0.1", "resolved_addresses": ["127.0.0.1"], "success": true },
  "stats": [
    {
      "address": "127.0.0.1",
      "port": 19192,
      "state": "open",
      "success": true,
      "bytes_sent": 5,
      "bytes_received": 5,
      "response_preview": "hello",
      "time_taken_µs": 4105
    }
  ],
  "error": ""
}
```

### Listen

Starts a local listener so `telnet`, `udp`, `web`, and `nmap` can be tested without a real remote server. Unlike every other command, `listen`'s own `--count` defaults to `0` (unlimited - keep listening until Ctrl+C) rather than the root `--count`'s default of `1`, since the whole point of starting a listener is usually to leave it up for a while; pass `--count N` to stop automatically after N connections/packets/requests instead. `--timeout` still means idle read timeout here (`0` = none) - see [Global flags](#global-flags). A summary line is printed on exit either way.

**Syntax:**

```bash
./shint listen tcp [port] [--bind 0.0.0.0] [--echo]
./shint listen udp [port] [--bind 0.0.0.0] [--echo]
./shint listen http [port] [--bind 0.0.0.0]
```

*   `--bind`: Local address to bind to (default `0.0.0.0`).
*   `--echo`: Echo received data back to the sender/connection.

**Example (start a listener, then connect from another shell):**

```bash
# Terminal 1: wait for one connection then exit
./shint listen tcp 9000 --echo --count 1

# Terminal 2
./shint telnet 127.0.0.1 9000
```

**Output (Terminal 1):**

```
Sat Sep 19 01:20:01 MDT 2026: [listen-tcp] OK listening address=0.0.0.0:9000 max_connections=1 echo=true
Sat Sep 19 01:20:03 MDT 2026: [listen-tcp] OK connection accepted remote=127.0.0.1:51451 local=127.0.0.1:9000
Sat Sep 19 01:20:03 MDT 2026: [listen-tcp] OK connection closed remote=127.0.0.1:51451 bytes_received=0 bytes_sent=0
Sat Sep 19 01:20:03 MDT 2026: [listen-tcp] OK done connections=1 bytes_received=0 bytes_sent=0 total_time=2.025359459s
```

Each chunk read from a connection is treated as one "request handled" for measurement purposes: with `--echo`, a text-mode `data received` line (and its JSON-mode equivalent below) reports that chunk's `bytes_sent` (the echoed reply) and `time_taken`/`processing_time_µs` (time from the read returning to the echo write completing) alongside `bytes_received`.

With `--json`, each received chunk/packet is printed as one JSON line as it arrives (JSON Lines format), for easy piping into another tool:

```json
{"protocol":"tcp","remote_address":"127.0.0.1:51496","local_address":"127.0.0.1:9000","bytes_read":20,"bytes_sent":20,"processing_time_µs":42,"preview":"hello-json-listener","unixtime_µs":1789802518134725}
```

#### HTTP listener

`listen http` is a minimal test endpoint useful for both plain TCP checks (`telnet`/`nmap` against it) and real HTTP checks (`web`, `curl`, a load balancer health check, etc.): any HTTP method on `/` returns a small JSON status dict, and every other path returns 404 with the same shape. `--bind`'s usual meaning still applies; `--echo` has no effect here (there's no request-body echoing - the response is always the fixed status dict).

**Example:**

```bash
./shint listen http 8080
```

```bash
curl -s -X POST http://127.0.0.1:8080/        # {"status":"ok"},  200
curl -s http://127.0.0.1:8080/anything/else    # {"status":"not found"}, 404
```

**Output:**

```
Sat Sep 19 03:02:50 MDT 2026: [listen-http] OK listening address=0.0.0.0:8080 max_requests=unlimited
Sat Sep 19 03:02:51 MDT 2026: [listen-http] OK request method=GET path=/ status=200 remote=127.0.0.1:54598 bytes_received=98 bytes_sent=160 time_taken=285.5µs
Sat Sep 19 03:02:51 MDT 2026: [listen-http] OK request method=GET path=/anything/else status=404 remote=127.0.0.1:54601 bytes_received=111 bytes_sent=175 time_taken=375.125µs
```

With `--json`, each request is printed as one JSON line, once the request is complete:

```json
{"method":"GET","path":"/","status_code":200,"remote_address":"127.0.0.1:54598","bytes_received":98,"bytes_sent":160,"processing_time_µs":202,"unixtime_µs":1789808580361517}
```

`bytes_received` and `bytes_sent` are the raw bytes that crossed the connection - request line, headers and body in; status line, headers and body out - counted the same way `web` counts its own, so the two sides' figures match (see [Bytes sent and received](#bytes-sent-and-received)). `time_taken` / `processing_time_µs` covers handling that one request, from entering the handler to the response being flushed. The exit summary carries the byte totals.

The listener never parses or acts on request headers: only the request line's method and path are used (to route `/` vs. everything else, and to log). Conditional, range, and content-negotiation headers change nothing, and responses carry no `ETag` or `Last-Modified`, since a validator is only useful if `If-None-Match`/`If-Modified-Since` were honored. The request body is read and discarded unseen, so a large upload finishes before it is answered and the byte count is complete.

## Supported platforms

Release binaries are built with `CGO_ENABLED=0` and `-trimpath -s -w`, so each is a small (~7-9MB), statically-linked, dependency-free executable — copy it anywhere and run it. 14 OS/architecture combinations are built on every tagged release:

| OS | amd64 | arm64 |
|---|---|---|
| Linux | ✅ | ✅ |
| macOS (darwin) | ✅ | ✅ |
| Windows | ✅ | ✅ |
| FreeBSD | ✅ | ✅ |
| OpenBSD | ✅ | ✅ |
| NetBSD | ✅ | ✅ |
| Solaris | ✅ | — (Go has no solaris/arm64 port) |
| Android | — | ✅ |

Android's `amd64` target is skipped: it's the emulator-only architecture and requires cgo (external linking) for its libc syscall shims, which would force the build off `CGO_ENABLED=0` and defeat the point of a small static binary. `arm64` covers real devices (and Termux) and builds with the same static, dependency-free approach as everything else.

## Docker image

Every tagged release is also published as a multi-arch (`linux/amd64`, `linux/arm64`) image to both the GitHub Container Registry and Docker Hub, built from the `Dockerfile` at the repo root: a `golang:1.27.1-alpine` build stage compiling the same static (`CGO_ENABLED=0`) binary as the release binaries, copied into a `gcr.io/distroless/static-debian12:nonroot` final image (no shell, no package manager, CA certificates included so `web`'s HTTPS requests verify normally). A tag push of `v4.0.2` publishes:

```
ghcr.io/dmartsapp/shint:v4.0.2        docker.io/farhansabbir/shint:v4.0.2
ghcr.io/dmartsapp/shint:4.0.2         docker.io/farhansabbir/shint:4.0.2
ghcr.io/dmartsapp/shint:4.0           docker.io/farhansabbir/shint:4.0
ghcr.io/dmartsapp/shint:4             docker.io/farhansabbir/shint:4
ghcr.io/dmartsapp/shint:latest        docker.io/farhansabbir/shint:latest
```

```bash
docker run --rm farhansabbir/shint:latest nmap --from 1 --to 1024 example.com
docker run --rm farhansabbir/shint:latest web https://example.com --json

# listen commands need the container's port published to reach it from outside
docker run --rm -p 9000:9000/tcp farhansabbir/shint:latest listen tcp 9000 --bind 0.0.0.0
docker run --rm -p 8080:8080/tcp farhansabbir/shint:latest listen http 8080
```

`ping` inside a container follows the same unprivileged-ICMP rules as running on the host directly (see [Platform notes](#platform-notes)) - no extra `--cap-add` should be needed on a typical Docker host.

Build it locally with `docker build -t shint .` (or `docker buildx build --platform linux/amd64,linux/arm64 ...` to reproduce the multi-arch CI build).

## Platform notes

- **ICMP (`ping`) privileges:** on macOS, BSD, and Windows, ICMP echo works for a regular, non-root/non-admin user out of the box. On Linux it depends on the `net.ipv4.ping_group_range` sysctl; most desktop distributions ship it open to all users already, but a hardened or minimal distro may restrict it to root. If `ping` fails with a permission error there, either run as root or widen the range: `sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"`.
- **UDP/nmap results are best-effort:** neither protocol has a reliable way to distinguish "nothing is listening" from "a firewall silently dropped the packet." Treat `open|filtered` and unresponsive TCP ports accordingly.
- **IPv6 listening:** pass `--bind ::` to `listen tcp`/`listen udp`/`listen http` for IPv6. Verified dual-stack (an IPv4 client can also reach a `::`-bound listener) on both macOS and Linux; not independently verified on Windows/BSD/Solaris, though Go's `net` package aims for consistent behavior across platforms. The default bind (`0.0.0.0`) is unaffected either way - IPv4-only, as before.

## Development

```bash
go build ./...          # compile
go vet ./...             # static analysis
go test ./... -race      # unit tests, race detector on
make all                 # cross-compile the desktop triad
make all-platforms       # cross-compile all 14 release targets
```

CI is five independent workflow files (`.github/workflows/*.yaml`), all triggered by the same tagged push (`v*.*.*`), so each has its own status badge (see the table at the top) instead of one combined pass/fail:

- **`lint.yaml`** / **`vulncheck.yaml`** - `golangci-lint` and `govulncheck` respectively, each filing a GitHub issue on failure.
- **`build.yaml`** - re-runs the same two checks as an internal gate (not a duplicate report - just a "don't proceed if this would fail" guard), builds all 14 [platform binaries](#supported-platforms), and creates the GitHub Release with them attached.
- **`docker-hub.yaml`** / **`ghcr.yaml`** - each with the same internal gate, build and push the multi-arch [Docker image](#docker-image) to their one registry.

They're separate files specifically so a registry outage or a Docker Hub credential problem, say, shows up as *that* row failing rather than obscuring whether the binaries themselves were fine.

## Changelog

### v4.0.2

- `web` and `listen http` now measure bytes the same way: everything on the wire, headers included, counted at the connection instead of estimated from the parsed message. v4.0.1's `web` figure estimated response-header size and never measured what was sent, while `listen http` reported body bytes only, so the two never agreed. `web` now reports `bytes_sent` and `bytes_received` (`--json` and text), summed over every hop if a redirect is followed - `bytes_received` replaces `bytes_downloaded`, and `bandwidth_kbs` is derived from it - and `listen http`'s `bytes_received` / `bytes_sent` are the full request and response, so `web`'s `bytes_sent` equals the listener's `bytes_received` and vice versa. The listener also totals both in its exit summary.
- `web --json`: `input_params.payload_bytes` is now the request body size; it used to add the number of `-H` flags to it.
- `web` text output: the response line no longer repeats the status three times over (`response ok ... status="200 OK"` is now `response ... status=200`).
- `listen http` reads the request body before answering (it already discarded it in v4.0.1, but only after the response went out): Go's HTTP client abandons an upload that is still in progress once a complete `Connection: close` response comes back, so a large POST/PUT (past roughly the socket buffer - about 170KB on loopback) could be cut off partway.
- `listen http` responses carry no validators (`ETag`/`Last-Modified`) and request headers are never acted on - see [HTTP listener](#http-listener).

### v4.0.1

- `listen tcp` now measures each read (+ optional echo write) as one handled "request": text-mode `data received`/`connection closed`/`done` lines and JSON-mode `ListenEvent`s report `bytes_sent` alongside the existing `bytes_received`, plus `processing_time_µs` (`time_taken` in text mode) for how long that read/echo cycle took.
- `listen http` now reports `bytes_received` (the request body's length, now drained rather than ignored), `bytes_sent` (the response body's length), and `processing_time_µs`/`time_taken` for every request, in both text and `--json` mode.
- Fixed `web`'s bandwidth figure (`speed` in text mode, now also exposed as `bandwidth_kbs` in `--json` output): it previously added `len(header)` - the number of header *keys*, not their byte size - onto the downloaded-bytes count, understating the real transfer size and the KB/s derived from it. Both now use an actual header byte-size estimate.

### v4.0.0

- **Full dual-stack IPv6 support.** `telnet`, `nmap`, `udp`, `web`, and `listen` all resolve and operate over both IPv4 and IPv6 now (a dual-stack hostname is checked/scanned/pinged over both in the same run) - `NetworkType` changed from `"ip4"` to `"ip"`. `listen tcp`/`listen udp`/`listen http` accept `--bind ::` for IPv6 (or dual-stack, platform-dependent - see [Platform notes](#platform-notes)). `ping` already got this in the previous release via the [go-ping](https://github.com/dmartsapp/go-ping) v2.0.0 upgrade (which also fixed a data race, a payload-size clamp bug, a sequence-matching bug, and a JSON-encoding bug on the ICMP side); every other command catches up here.
- Fixed `listen tcp`/`listen udp`/`listen http` exiting after exactly one connection/packet/request by default: `listen`'s own `--count` now defaults to `0` (unlimited, until Ctrl+C) instead of inheriting the root `--count`'s default of `1`.
- Fixed `listen http` occasionally dropping its own response (`curl: (52) Empty reply from server`) right as it hit its request budget - a real race between the process exiting and net/http's internal per-connection goroutine still flushing that same response. Hardened with an explicit flush, a deterministic `Connection: close`, and a proper `Server.Shutdown` wait before returning.
- CI split from one combined workflow into five independent ones (lint, vulnerability check, binary build & release, Docker Hub, GHCR) so each has its own live status instead of one pass/fail covering everything - see the table at the top.

### v3.1.0

- Added `listen http`: a minimal JSON status endpoint (any method on `/` returns `{"status":"ok"}`, every other path 404s with `{"status":"not found"}`), useful for both plain TCP and HTTP-level reachability checks against the same process.
- Docker images now publish to Docker Hub (`farhansabbir/shint`) in addition to GHCR.

### v3.0.0

- Added `udp` (UDP probe with open/closed/open\|filtered classification).
- Added `listen tcp` / `listen udp` local listeners for testing without a real remote server.
- Added `--cacert`/`--cert`/`--key`/`--insecure` to `web` for authenticated TLS checks (custom CA trust and mutual TLS) against HTTPS services.
- Fixed data races on the JSON stats slices in `telnet` and `web` (concurrent goroutines appending to a shared slice without synchronization).
- Fixed a nil-pointer risk in `web` when given an unparseable URL.
- Fixed `web --json --withbody` serializing the already-consumed request body reader instead of the payload actually sent.
- Added input validation (port ranges, positive `--count`/`--timeout`, `--from <= --to` on `nmap`) instead of panicking or hanging on bad input.
- Bounded `nmap`'s concurrency so a wide port range can't exhaust file descriptors, and fixed it ignoring context cancellation during the scan loop so a slow/wide scan now actually stops at the caller's `--timeout` instead of running every dial out individually.
- Unified text-mode logging across every command into one greppable format (see [Logging format](#logging-format)).
- Upgraded to Go 1.26.3 and the latest available cobra/x-net/x-sys releases; removed stray dead config (`go.env`, `.gitmodules`) left over from an earlier private-submodule setup.
- Expanded the release build matrix from 6 to 14 OS/architecture targets (added FreeBSD, OpenBSD, NetBSD, Solaris, Android).
- Added a multi-arch Docker image published to GHCR on every tagged release (see [Docker image](#docker-image)).
- Added an exhaustive, hermetic test suite (60 tests) covering both packages, including a full mTLS round trip against a locally-generated CA.

## Data Collection and Privacy

This tool does not collect or store any personal information. It is a command-line utility that performs network checks and displays the results to the user. The only data that is transmitted over the network is the data required to perform the requested network check (e.g., DNS queries, TCP/UDP connections, ICMP packets, HTTP requests).
