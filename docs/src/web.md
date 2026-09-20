---
title: web
lead: Make an HTTP or HTTPS request and see exactly what happened - status, timing, and how many bytes travelled each way - with support for custom headers, request bodies, private certificate authorities and mutual TLS.
description: shint web is a small HTTP(S) client for diagnostics - methods, headers, bodies, TLS options, mutual TLS, wire-accurate byte counts and JSON output.
section: Commands
order: 3
nav: web
---

## What it does

`shint web` sends an HTTP request and reports the outcome: whether a response came back, the status, how long it took, the transfer speed, and the number of bytes sent and received. Add `--json` for the full request and response headers (and, with `-W`, the body).

It is a diagnostic tool, not a browser: it follows redirects (up to 10) but does not fetch a page's images, scripts or other embedded resources. If the URL has no scheme, `https://` is assumed.

## Syntax

```bash
shint web <url> [-X METHOD] [-H "Name: value"]... [-P body] [-W] [flags]
```

| Flag | Meaning |
|---|---|
| `-X`, `--method` | HTTP method: `GET` (default), `POST`, `PUT`, `DELETE`, ... |
| `-H`, `--header` | A header to send, as `"Name: value"`. Repeat for several. |
| `-P`, `--payload` | The request body. |
| `-W`, `--withbody` | Include the response body in the `--json` output. |
| `--cacert FILE` | Trust this PEM CA bundle *in addition to* the system roots. |
| `--cert FILE`, `--key FILE` | Client certificate and key for mutual TLS (give both). |
| `-k`, `--insecure` | Skip certificate verification. Diagnostics only. |
| `--timing` | Show where each request's time went: DNS, connect, TLS, wait, download. See [Timing](#timing-where-the-time-went). |

## Examples

The examples below run against `shint listen http 8080` (see [listen](listen.md)) so you can reproduce them on your own machine.

### A simple GET

```bash
shint web http://127.0.0.1:8080/
```

```text
Sun Sep 20 01:50:18 MDT 2026: [web] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=114.5µs
Sun Sep 20 01:50:19 MDT 2026: [web] OK response url=http://127.0.0.1:8080/ status=200 bytes_sent=97 bytes_received=160 speed=50.92KB/s attempt=1/1 time=3.0685ms

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 3.0685ms, average: 3.0685ms, maximum: 3.0685ms
Sun Sep 20 01:50:19 MDT 2026: [web] OK done total_time=1.005419583s
```
### A path that does not exist

```bash
shint web http://127.0.0.1:8080/missing
```

```text
Sun Sep 20 01:50:19 MDT 2026: [web] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=118.625µs
Sun Sep 20 01:50:20 MDT 2026: [web] OK response url=http://127.0.0.1:8080/missing status=404 bytes_sent=104 bytes_received=175 speed=67.90KB/s attempt=1/1 time=2.516959ms

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 2.516959ms, average: 2.516959ms, maximum: 2.516959ms
Sun Sep 20 01:50:20 MDT 2026: [web] OK done total_time=1.004834208s
```
A 404 is still a *response*, so the exit status stays `0` - only a request that gets no response at all (refused, timed out, TLS failure) exits `1`. Read the status from the output or `--json` if you need to act on it.

### POST with a body and headers

```bash
shint web -X POST -P '{"name":"shint"}' \
  -H 'Content-Type: application/json' -H 'X-Request-Id: demo-42' \
  http://127.0.0.1:8080/
```

```text
Sun Sep 20 01:50:20 MDT 2026: [web] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=108.833µs
Sun Sep 20 01:50:21 MDT 2026: [web] OK response url=http://127.0.0.1:8080/ status=200 bytes_sent=189 bytes_received=160 speed=160.16KB/s attempt=1/1 time=975.584µs

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 975.584µs, average: 975.584µs, maximum: 975.584µs
Sun Sep 20 01:50:21 MDT 2026: [web] OK done total_time=1.0029005s
```
The request was 189 bytes on the wire against 97 for the plain GET. The extra 92 bytes are exactly what was added: the 16-byte body, the `Content-Type` (32) and `X-Request-Id` (23) headers you passed, the `Content-Length` header (20) that a body requires, and one more letter in `POST`.

### Repeat the request

```bash
shint web http://127.0.0.1:8080/ --count 2 --delay 300
```

```text
Sun Sep 20 01:50:21 MDT 2026: [web] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=44.125µs
Sun Sep 20 01:50:21 MDT 2026: [web] OK response url=http://127.0.0.1:8080/ status=200 bytes_sent=97 bytes_received=160 speed=125.82KB/s attempt=1/2 time=1.241875ms
Sun Sep 20 01:50:22 MDT 2026: [web] OK response url=http://127.0.0.1:8080/ status=200 bytes_sent=97 bytes_received=160 speed=64.04KB/s attempt=2/2 time=2.44ms

========================================== web STATISTICS ==========================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 1.241875ms, average: 1.840937ms, maximum: 2.44ms
Sun Sep 20 01:50:22 MDT 2026: [web] OK done total_time=605.185875ms
```
### JSON, with the response body

```bash
shint web --json -W http://127.0.0.1:8080/
```

```json
{
  "input_params": { ... },
  "module_name": "web",
  "dns_lookup": { ... },
  "stats": [
    {
      "url": "http://127.0.0.1:8080/",
      "errors": [],
      "request": {
        "body": "",
        "headers": {
          "User-Agent": [
            "dmarts.app-http-v0.1"
          ]
        },
        "method": "GET"
      },
      "response": {
        "body": {
          "status": "ok"
        },
        "header": {
          "Content-Type": [
            "application/json"
          ],
          "Date": [
            "Sun, 20 Sep 2026 07:50:23 GMT"
          ]
        }
      },
      "success": true,
      "recv_unixtime_µs": 1789890623262263,
      "sent_unixtime_µs": 1789890623259517,
      "time_taken_µs": 2452,
      "bytes_sent": 97,
      "bytes_received": 160,
      "status_code": 200,
      "bandwidth_kbs": 63.70725243361704
    }
  ],
  "end_time_unixtime_µs": 1789890623262274,
  "start_time_unixtime_µs": 1789890622257625,
  "total_time_taken_µs": 1004649,
  "error": ""
}
```
## Bytes sent and received

`bytes_sent` and `bytes_received` are **everything that crossed the connection** for the request - the request line, headers and body going out; the status line, headers and body coming back - not just the bodies. That is why a `GET` with no body still reports 97 bytes sent.

Two properties make the numbers trustworthy:

- **They are measured on the connection**, not estimated from the parsed message, so they include headers the client adds itself (`Host`, `User-Agent`, `Accept-Encoding`, `Content-Length`) and chunked-encoding framing.
- **They match the other side.** [`shint listen http`](listen.md) counts its traffic the same way, so `web`'s `bytes_sent` equals the listener's `bytes_received` and vice versa. In the examples above both report 97 sent / 160 received for the GET.

Over HTTPS the count is taken on the decrypted side (the HTTP bytes, without TLS record overhead), and when a redirect is followed the bytes of every hop are added up.

## HTTPS and certificates

### A certificate that is not trusted

Verification is on by default. A certificate that was not issued by a trusted authority fails:

```bash
shint web https://localhost:9443/
```

```text
Sun Sep 20 01:54:17 MDT 2026: [web] OK dns resolved host=localhost addresses=2 ips=[::1,127.0.0.1] time=3.918125ms
Sun Sep 20 01:54:17 MDT 2026: [web] ERROR request failed url=https://localhost:9443/ attempt=1/1 time=16.314208ms error="Get \"https://localhost:9443/\": tls: failed to verify certificate: x509: “localhost” certificate is not trusted"
```
### Trust your own certificate authority

For an internal service, give `--cacert` the PEM file of the CA that signed its certificate (it is used in addition to the system roots):

```bash
shint web --cacert ca.pem https://localhost:9443/
```

```text
Sun Sep 20 01:54:18 MDT 2026: [web] OK using custom CA bundle to verify server certificate
Sun Sep 20 01:54:18 MDT 2026: [web] OK dns resolved host=localhost addresses=2 ips=[::1,127.0.0.1] time=1.848583ms
Sun Sep 20 01:54:18 MDT 2026: [web] OK response url=https://localhost:9443/ status=200 bytes_sent=97 bytes_received=175 speed=17.66KB/s attempt=1/1 time=9.677334ms
```
### Skip verification (diagnostics only)

`-k` disables certificate checks, like `curl -k`. shint prints a warning line to remind you the answer cannot be trusted:

```bash
shint web -k https://localhost:9443/
```

```text
Sun Sep 20 01:54:18 MDT 2026: [web] ERROR tls verification disabled warning="certificate checks are skipped, response may not be trustworthy"
Sun Sep 20 01:54:18 MDT 2026: [web] OK dns resolved host=localhost addresses=2 ips=[::1,127.0.0.1] time=2.149791ms
Sun Sep 20 01:54:18 MDT 2026: [web] OK response url=https://localhost:9443/ status=200 bytes_sent=97 bytes_received=175 speed=38.07KB/s attempt=1/1 time=4.48925ms
```
### Mutual TLS

Some services require the client to present a certificate too. Pass both `--cert` and `--key`. Without them the server refuses the handshake:

```bash
shint web --cacert ca.pem https://localhost:9444/
```

```text
Sun Sep 20 01:54:18 MDT 2026: [web] ERROR request failed url=https://localhost:9444/ attempt=1/1 time=8.781459ms error="Get \"https://localhost:9444/\": remote error: tls: handshake failure"
```
```bash
shint web --cacert ca.pem --cert client.pem --key client-key.pem https://localhost:9444/
```

```text
Sun Sep 20 01:54:18 MDT 2026: [web] OK using client certificate for mutual TLS
Sun Sep 20 01:54:18 MDT 2026: [web] OK using custom CA bundle to verify server certificate
Sun Sep 20 01:54:18 MDT 2026: [web] OK dns resolved host=localhost addresses=2 ips=[::1,127.0.0.1] time=1.388416ms
Sun Sep 20 01:54:18 MDT 2026: [web] OK response url=https://localhost:9444/ status=200 bytes_sent=97 bytes_received=175 speed=18.41KB/s attempt=1/1 time=9.285083ms
```
## Timing: where the time went

A slow request is a different problem depending on *where* it is slow. `--timing` adds the breakdown to each request: how long the name lookup, the connection, the TLS handshake, the wait for the server, and the download each took.

| Phase | What it measures |
|---|---|
| `dns` | The name lookup for this request. `0s` when the URL has an IP address instead of a name, or the connection is reused. |
| `connect` | Opening the TCP connection. `0s` on a reused connection. If the name has several addresses and the first fails, this includes the time spent on it, up to the address that worked. |
| `tls` | The TLS handshake. `0s` for `http://` and for a reused connection. |
| `wait` | From the request being fully sent to the **first byte of the response**: the server's time to respond, plus one network round trip. This is what browsers call "time to first byte" less the connection set-up. |
| `download` | From the first byte of the response to the last byte of its body. |
| `total` | The whole hop, from asking for a connection to the last byte read. |

The phases add up to the total (to within the trace's own bookkeeping, microseconds).

### One request

```bash
shint web https://example.com --timing
```

```text
Sun Sep 20 17:36:50 MDT 2026: [web] OK dns resolved host=example.com addresses=4 ips=[2606:4700:10::6814:179a,2606:4700:10::ac42:93f3,172.66.147.243,104.20.23.154] time=5.305375ms
Sun Sep 20 17:36:51 MDT 2026: [web] OK response url=https://example.com status=200 bytes_sent=94 bytes_received=705 speed=4.85KB/s attempt=1/1 time=141.889375ms
Sun Sep 20 17:36:51 MDT 2026: [web] OK timing url=https://example.com hop=1/1 status=200 connection=new dns=3.845ms connect=34.038ms tls=55.517ms wait=45.806ms download=1.907ms total=141.678ms attempt=1/1

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 141.889375ms, average: 141.889375ms, maximum: 141.889375ms
Sun Sep 20 17:36:51 MDT 2026: [web] OK done total_time=1.14980475s
```

Read the `timing` line: of the 141 ms, 55 ms was the TLS handshake, 46 ms the server taking to answer, 34 ms opening the connection and 4 ms the lookup. The download of the small page was 2 ms. (`time=` on the `response` line is the same request measured end to end; the `dns resolved` line at the top is a separate lookup made before the request, so its time is not one of these.)

### A redirect: one line per hop

When a redirect is followed, each hop gets its own line, numbered `hop=1/2`, with the status that hop returned:

```bash
shint web http://google.com --timing
```

```text
Sun Sep 20 17:36:51 MDT 2026: [web] OK dns resolved host=google.com addresses=2 ips=[2607:f8b0:400a:803::200e,142.251.46.78] time=5.836708ms
Sun Sep 20 17:36:53 MDT 2026: [web] OK response url=http://google.com status=200 bytes_sent=218 bytes_received=30872 speed=69.91KB/s attempt=1/1 time=431.219708ms
Sun Sep 20 17:36:53 MDT 2026: [web] OK timing url=http://google.com hop=1/2 status=301 connection=new dns=3.677ms connect=199.103ms tls=0s wait=46.649ms download=625µs total=250.689ms attempt=1/1
Sun Sep 20 17:36:53 MDT 2026: [web] OK timing url=http://www.google.com/ hop=2/2 status=200 connection=new dns=4.932ms connect=42.979ms tls=0s wait=74.752ms download=56.807ms total=180.304ms attempt=1/1

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 431.219708ms, average: 431.219708ms, maximum: 431.219708ms
Sun Sep 20 17:36:53 MDT 2026: [web] OK done total_time=1.439891417s
```

The first hop is the `301` from `http://google.com` (which itself needed a connection: 199 ms here), the second the `200` from `http://www.google.com/`, on a new connection to a different host. The two hops account for the request's whole `time=`.

### A connection that is reused

With `--count`, later requests can use the connection an earlier one left open, and `--timing` shows it:

```bash
shint web https://example.com --timing --count 2 --delay 500
```

```text
Sun Sep 20 17:36:53 MDT 2026: [web] OK dns resolved host=example.com addresses=4 ips=[2606:4700:10::ac42:93f3,2606:4700:10::6814:179a,172.66.147.243,104.20.23.154] time=6.259083ms
Sun Sep 20 17:36:53 MDT 2026: [web] OK response url=https://example.com status=200 bytes_sent=94 bytes_received=705 speed=5.86KB/s attempt=1/2 time=117.558167ms
Sun Sep 20 17:36:53 MDT 2026: [web] OK timing url=https://example.com hop=1/1 status=200 connection=new dns=5.217ms connect=28.472ms tls=47.975ms wait=35.328ms download=87µs total=117.461ms attempt=1/2
Sun Sep 20 17:36:54 MDT 2026: [web] OK response url=https://example.com status=200 bytes_sent=94 bytes_received=705 speed=18.04KB/s attempt=2/2 time=38.163792ms
Sun Sep 20 17:36:54 MDT 2026: [web] OK timing url=https://example.com hop=1/1 status=200 connection=reused dns=0s connect=0s tls=0s wait=37.862ms download=166µs total=38.092ms attempt=2/2

========================================== web STATISTICS ==========================================
Requests sent: 2, Response received: 2, Success: 100%
Latency: minimum: 38.163792ms, average: 77.860979ms, maximum: 117.558167ms
Sun Sep 20 17:36:54 MDT 2026: [web] OK done total_time=1.047312667s
```

The second request says `connection=reused` and has no `dns`, `connect` or `tls` at all - which is why it took 38 ms instead of 117 ms.

### A request that fails

A failed request still reports the time it spent before it failed:

```bash
shint web http://127.0.0.1:1 --timing --timeout 2
```

```text
Sun Sep 20 17:36:54 MDT 2026: [web] OK dns resolved host=127.0.0.1 addresses=1 ips=[127.0.0.1] time=58.209µs
Sun Sep 20 17:36:55 MDT 2026: [web] ERROR request failed url=http://127.0.0.1:1 attempt=1/1 time=893.5µs error="Get \"http://127.0.0.1:1\": dial tcp 127.0.0.1:1: connect: connection refused"
Sun Sep 20 17:36:55 MDT 2026: [web] OK timing url=http://127.0.0.1:1 hop=1/1 connection=new dns=0s connect=602µs tls=0s wait=0s download=0s total=788µs attempt=1/1

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 0
Latency: minimum: 0, average: 0, maximum: 0
Sun Sep 20 17:36:55 MDT 2026: [web] OK done total_time=1.002765167s
```

There is no `status` on the `timing` line - no response arrived - and the 602 µs of `connect` is how long the refusal took.

### JSON

With `--json`, each stat gains a `timing` object (and only with `--timing`: without the flag the field is absent, so existing consumers see no change). `hops` has one entry per hop, times in microseconds:

```json
{
  ...
  "stats": [
    {
      "url": "http://google.com",
      "success": true,
      "status_code": 200,
      "time_taken_µs": 434854,
      "timing": {
        "hops": [
          {
            "url": "http://google.com",
            "status_code": 301,
            "reused_connection": false,
            "dns_µs": 3891,
            "connect_µs": 208242,
            "tls_µs": 0,
            "wait_µs": 43617,
            "download_µs": 469,
            "total_µs": 256947
          },
          {
            "url": "http://www.google.com/",
            "status_code": 200,
            "reused_connection": false,
            "dns_µs": 4370,
            "connect_µs": 45324,
            "tls_µs": 0,
            "wait_µs": 75112,
            "download_µs": 52380,
            "total_µs": 177746
          }
        ]
      },
      ...
    }
  ]
}
```

`status_code` on a hop is `0` when that hop got no response. `reused_connection` is `true` when the connection was one an earlier request had left open.

## Good to know

- **`Ctrl+C` shows the summary.** A run with a large `--count` stops, prints how far it got, the statistics and the `done` line, and exits `1` (cut short) - see [Stopping early](usage.md#stopping-early).
- **Redirects** are followed (up to 10); the final response is what is reported, and the bytes of every hop are counted.
- **Timeouts.** `--timeout` limits each request from connecting to the last byte of the response.
- **Timing.** `--timing` changes nothing about how the request is made: the redirect limit, the byte counts and the exit status are the same with and without it.
- **`--payload`** means the *size* of filler data for `ping` and `udp`, but on `web` the `-P` form means the request *body*.
- **HTTP/1.1** is used for all requests; the byte counts describe that plain byte stream.
- The default `User-Agent` is `dmarts.app-http-v0.1`; override it with `-H "User-Agent: ..."`.
