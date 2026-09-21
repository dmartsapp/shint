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
## Good to know

- **`Ctrl+C` shows the summary.** A run with a large `--count` stops, prints how far it got, the statistics and the `done` line, and exits `1` (cut short) - see [Stopping early](usage.md#stopping-early).
- **Redirects** are followed (up to 10); the final response is what is reported, and the bytes of every hop are counted.
- **Timeouts.** `--timeout` limits each request from connecting to the last byte of the response.
- **`--payload`** means the *size* of filler data for `ping` and `udp`, but on `web` the `-P` form means the request *body*.
- **HTTP/1.1** is used for all requests; the byte counts describe that plain byte stream.
- The default `User-Agent` is `dmarts.app-http-v0.1`; override it with `-H "User-Agent: ..."`.
