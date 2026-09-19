# that SHIt Network Tool - aka shint, (v3.x.y)

## Introduction

A simple, modern, and versatile network utility tool built with Go. It bundles a handful of small diagnostics that are normally reached for as separate programs — a `telnet`-style TCP connectivity check, a basic ICMP `ping`, a `wget`/`curl`-style HTTP(S) client, a limited TCP-only `nmap` port scanner, a `udp` probe, and local `listen` servers for testing the others without needing a real remote endpoint.

[![Build and release telnet binary](https://github.com/dmartsapp/shint/actions/workflows/actions.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/actions.yaml)

**Note:** Version 3.0.0 added `udp`, `listen tcp`/`listen udp`, and authenticated-TLS options on `web` (`--cacert`/`--cert`/`--key`/`--insecure`), fixed several correctness/race bugs from v2, and unified the text-mode log output across every command; v3.1.0 adds `listen http`, a minimal JSON status endpoint for testing plain TCP and HTTP reachability against the same process. See [Changelog](#changelog) for the full list.

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

Sends ICMP ECHO_REQUEST packets to a host to test reachability.

**Syntax:**

```bash
./shint ping [host]
```

**Example:**

```bash
./shint ping google.com --count 2
```

**Output:**

```
Mon Jun 30 13:23:32 EDT 2025: [icmp] OK dns resolved host=google.com addresses=1 ips=[142.251.41.46] time=1.409125ms
Mon Jun 30 13:23:32 EDT 2025: [icmp] OK Received response for request #1 from 142.251.41.46 with 4 bytes of data in 9ms
Mon Jun 30 13:23:33 EDT 2025: [icmp] OK Received response for request #2 from 142.251.41.46 with 4 bytes of data in 10ms

========================================= icmp STATISTICS =========================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 9ms, average: 9.5ms, maximum: 10ms
Mon Jun 30 13:23:33 EDT 2025: [icmp] OK done packets_lost=0 stddev_ms=0.500 resolve_time=1.409125ms total_time=2.011017458s
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
Mon Jun 30 13:23:36 EDT 2025: [web] OK dns resolved host=google.com addresses=1 ips=[142.251.41.46] time=1.377ms
Mon Jun 30 13:23:36 EDT 2025: [web] OK response ok url=https://google.com status="200 OK" bytes=17722 speed=73.97KB/s attempt=1/1 time=233.967416ms

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 233.967416ms, average: 233.967416ms, maximum: 233.967416ms
Mon Jun 30 13:23:36 EDT 2025: [web] OK done total_time=235.525041ms
```

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
Sat Sep 19 01:20:03 MDT 2026: [listen-tcp] OK connection closed remote=127.0.0.1:51451 bytes_total=0
Sat Sep 19 01:20:03 MDT 2026: [listen-tcp] OK done connections=1 bytes_received=0 total_time=2.025359459s
```

With `--json`, each received chunk/packet is printed as one JSON line as it arrives (JSON Lines format), for easy piping into another tool:

```json
{"protocol":"tcp","remote_address":"127.0.0.1:51496","local_address":"127.0.0.1:9000","bytes_read":20,"preview":"hello-json-listener","unixtime_µs":1789802518134725}
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
Sat Sep 19 03:02:51 MDT 2026: [listen-http] OK request method=GET path=/ status=200 remote=127.0.0.1:54598
Sat Sep 19 03:02:51 MDT 2026: [listen-http] OK request method=GET path=/anything/else status=404 remote=127.0.0.1:54601
```

With `--json`, each request is printed as one JSON line as it arrives:

```json
{"method":"GET","path":"/","status_code":200,"remote_address":"127.0.0.1:54598","unixtime_µs":1789808580361517}
```

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

Every tagged release is also published as a multi-arch (`linux/amd64`, `linux/arm64`) image to both the GitHub Container Registry and Docker Hub, built from the `Dockerfile` at the repo root: a `golang:1.27.1-alpine` build stage compiling the same static (`CGO_ENABLED=0`) binary as the release binaries, copied into a `gcr.io/distroless/static-debian12:nonroot` final image (no shell, no package manager, CA certificates included so `web`'s HTTPS requests verify normally). A tag push of `v3.0.0` publishes:

```
ghcr.io/dmartsapp/shint:v3.0.0        docker.io/farhansabbir/shint:v3.0.0
ghcr.io/dmartsapp/shint:3.0.0         docker.io/farhansabbir/shint:3.0.0
ghcr.io/dmartsapp/shint:3.0           docker.io/farhansabbir/shint:3.0
ghcr.io/dmartsapp/shint:3             docker.io/farhansabbir/shint:3
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

## Development

```bash
go build ./...          # compile
go vet ./...             # static analysis
go test ./... -race      # unit tests, race detector on
make all                 # cross-compile the desktop triad
make all-platforms       # cross-compile all 14 release targets
```

CI (`.github/workflows/actions.yaml`) runs `golangci-lint` and `govulncheck` on every tagged push (`v*.*.*`), then builds and releases all 14 platform binaries and pushes the multi-arch Docker image to both GHCR and Docker Hub (see [Docker image](#docker-image)).

## Changelog

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
