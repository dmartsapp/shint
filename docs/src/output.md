---
title: Output formats
lead: Every line shint prints, and every field in its JSON, in one place.
description: Reference for shint's text log format, the statistics block, and the JSON schema of every command.
section: Reference
order: 1
nav: Output formats
---

## Text output

### The line format

Every command's text output follows one grammar, so it is easy to read, `grep` and parse regardless of which command produced it:

```text
<timestamp>: [<module>] OK|ERROR <message> key=value key=value ...
```

| Part | Meaning |
|---|---|
| `<timestamp>` | Local time when the line was printed, in the format of Go's `time.UnixDate` (`Sun Sep 20 01:26:48 MDT 2026`). |
| `[<module>]` | The command that spoke. |
| `OK` / `ERROR` | The level of this one line. An `ERROR` line means that step failed (or, for a few warnings such as `-k`, that it deserves attention). |
| `<message>` | A short lowercase description of the event. |
| `key=value` | Details. Durations print like `1.3ms`; values containing spaces are quoted (`error="dial tcp ..."`). |

Results and their `ERROR` lines go to **stdout**. Usage errors (a bad flag or value) go to **stderr**.

### The statistics block

Commands that repeat a check (`telnet`, `ping`, `web`) finish with a summary:

```text
======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 4.606042ms, average: 4.606042ms, maximum: 4.606042ms
```
`Success` is the percentage of attempts that got an answer. When nothing answered, the latency line reads `minimum: 0, average: 0, maximum: 0`.

### Modules and their messages

| Module | Messages |
|---|---|
| `telnet` | `dns resolved`, `dns resolution failed`, `connect ok`, `connect failed`, `done` |
| `icmp` | `dns resolved`, `received reply for request #N from ADDR (ipv4/ipv6) in Nms`, `ping failed`, `done` |
| `web` | `dns resolved`, `response`, `request failed`, `tls verification disabled`, `using client certificate for mutual TLS`, `using custom CA bundle...`, `done` |
| `nmap` | `dns resolved`, `scan started`, `progress`, `port open`, `scan complete`, `scan interrupted`, `done` |
| `udp` | `dns resolved`, `probe open`, `probe closed`, `probe open|filtered`, `probe error`, `done` |
| `listen-tcp` | `listening`, `connection accepted`, `data received`, `connection closed`, `done` |
| `listen-udp` | `listening`, `packet received`, `done` |
| `listen-http` | `listening`, `request`, `done` |

## JSON output

`--json` prints a single, indented JSON document (the listen commands print one compact JSON object per line as events happen - see [Listener events](#listener-events)). Key names are stable; changes to them are called out in the [changelog](changelog.md).

```json
{
  "input_params": {
    "module_name": "telnet",
    "sequential": false,
    "throttle": false,
    "host": "127.0.0.1",
    "from_port": 9000,
    "to_port": 9000,
    "protocol": "tcp",
    "timeout_ms": 5,
    "count": 1,
    "delay_ms": 1000,
    "payload_bytes": 4,
    "method": "",
    "data": "",
    "headers": null
  },
  "module_name": "telnet",
  "dns_lookup": {
    "hostname": "127.0.0.1",
    "resolved_addresses": [
      "127.0.0.1"
    ],
    "error": "",
    "success": true,
    "time_taken_µs": 97
  },
  "stats": [
    {
      "address": "127.0.0.1",
      "success": false,
      "recv_unixtime_µs": 0,
      "sent_unixtime_µs": 1789890611755345,
      "time_taken_µs": 709,
      "error": "dial tcp 127.0.0.1:9000: connect: connection refused"
    }
  ],
  "end_time_unixtime_µs": 1789890611756071,
  "start_time_unixtime_µs": 1789890610754105,
  "total_time_taken_µs": 1001966,
  "error": ""
}
```
### Top level

| Field | Type | Meaning |
|---|---|---|
| `input_params` | object | The parameters the command ran with. |
| `module_name` | string | `telnet`, `icmp`, `web`, `nmap` or `udp`. |
| `dns_lookup` | object | Result of resolving the host name. |
| `stats` | array | One entry per check; the shape depends on the command (below). |
| `start_time_unixtime_µs` | integer | When the run started, in Unix microseconds. |
| `end_time_unixtime_µs` | integer | When it ended. |
| `total_time_taken_µs` | integer | Duration of the whole run. |
| `error` | string | A run-level error (for example a failed lookup); empty on success. |

### input_params

| Field | Meaning |
|---|---|
| `module_name` | The command. |
| `host` | The target as given. |
| `from_port`, `to_port` | The port, or the scanned range for `nmap`. (For `ping` these hold `7`, the echo port, and carry no meaning.) |
| `protocol` | `tcp`, `udp` or `icmp`. |
| `timeout_ms` | The `--timeout` value. **Despite the name, it is in seconds.** |
| `count`, `delay_ms`, `throttle` | `--count`, `--delay` (milliseconds) and `--throttle`. |
| `payload_bytes` | Payload size: filler size for `ping`/`udp`, request body size for `web`. |
| `method`, `data`, `headers` | HTTP method, body and headers (`web` only). |
| `sequential` | Always `false`; reserved. |

### dns_lookup

| Field | Meaning |
|---|---|
| `hostname` | The name that was resolved. |
| `resolved_addresses` | Every address it resolved to, IPv4 and IPv6. |
| `success`, `error` | Whether the lookup worked, and why not. |
| `time_taken_µs` | How long it took. |

### Stats entries

**telnet**

| Field | Meaning |
|---|---|
| `address` | The address that was tried. |
| `success` | Whether the connection was established. |
| `sent_unixtime_µs`, `recv_unixtime_µs` | When the attempt started and when it connected. |
| `time_taken_µs` | Connect time. |
| `error` | Present only on failure. |

**icmp** (times are in milliseconds)

| Field | Meaning |
|---|---|
| `address`, `success`, `sequence` | Target, whether it was answered, and the request number. |
| `payload_size_bytes` | Reported payload size. |
| `sent_unixtime_ms`, `recv_unixtime_ms`, `time_taken_ms` | Timing of the echo request and its reply. |

**web**

| Field | Meaning |
|---|---|
| `url`, `success`, `status_code` | The URL, whether a response arrived, and its HTTP status (`0` if none). |
| `request` | `method`, `body` and `headers` as sent by the application (headers the HTTP client adds itself, such as `Host`, are counted in `bytes_sent` but not listed). |
| `response` | `header`, and `body` when `-W` is given (parsed as JSON when it is JSON, otherwise a string). Empty on failure. |
| `bytes_sent`, `bytes_received` | Everything that crossed the connection - see [web](web.md#bytes-sent-and-received). |
| `bandwidth_kbs` | `bytes_received` divided by the time taken, in KB/s. |
| `sent_unixtime_µs`, `recv_unixtime_µs`, `time_taken_µs` | Timing. |
| `errors` | Problems noticed, such as a malformed `-H` value or, on failure, why no response arrived. |

**nmap**

| Field | Meaning |
|---|---|
| `address`, `port` | What was probed. |
| `success` | `true` if the port accepted the connection. Every scanned port is listed, open or not. |

**udp**

| Field | Meaning |
|---|---|
| `address`, `port` | What was probed. |
| `state` | `open`, `closed`, `open|filtered` or `error`. |
| `success` | `false` only when the probe itself errored. Note this is *not* the exit-status rule: `closed` is reported here as `success: true` (the probe worked) but makes the process exit `1`. |
| `bytes_sent`, `bytes_received`, `response_preview` | Payload sizes and a short preview of any reply. |
| `sent_unixtime_µs`, `recv_unixtime_µs`, `time_taken_µs` | Timing. |
| `error` | Present only on failure. |

## Listener events

The listen commands print one JSON object per line as events happen (JSON Lines), suitable for piping into another tool.

**listen tcp / udp**

| Field | Meaning |
|---|---|
| `protocol` | `tcp` or `udp`. |
| `remote_address`, `local_address` | The two ends. |
| `bytes_read` | Bytes received in this read or packet. |
| `bytes_sent` | Bytes echoed back (`tcp` with `--echo`; omitted when zero). |
| `processing_time_µs` | Time from the read to the echo completing (`tcp`; omitted when zero). |
| `preview` | A short, single-line preview of the data. |
| `unixtime_µs` | When it happened. |
| `error` | Present only on failure. |

**listen http**

| Field | Meaning |
|---|---|
| `method`, `path`, `status_code` | The request line's method and path, and the status answered. |
| `remote_address` | The client. |
| `bytes_received`, `bytes_sent` | Raw bytes on the connection in each direction, headers and body included. |
| `processing_time_µs` | Time the handler took. |
| `unixtime_µs` | When the connection finished. |

## Units at a glance

| Where | Unit |
|---|---|
| Timestamps and durations in JSON | microseconds (`_µs`), except `icmp` which uses milliseconds (`_ms`) |
| `--timeout`, and `timeout_ms` in JSON | seconds |
| `--delay`, and `delay_ms` in JSON | milliseconds |
| `bytes_*` | bytes |
| `bandwidth_kbs`, `speed=` | kilobytes per second (1 KB = 1024 bytes) |
