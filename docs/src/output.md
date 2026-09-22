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
| `telnet` | `dns resolved`, `dns authoritative`, `dns resolution failed`, `connect ok`, `connect failed`, `done` |
| `icmp` | `dns resolved`, `dns authoritative`, `received reply for request #N from ADDR (ipv4/ipv6) in Nms bytes=N`, `ping failed`, `done` |
| `web` | `dns resolved`, `dns authoritative`, `response`, `timing` (with `--timing`), `request failed`, `tls verification disabled`, `using client certificate for mutual TLS`, `using custom CA bundle...`, `done` |
| `nmap` | `dns resolved`, `dns authoritative`, `scan started`, `progress`, `port open`, `scan complete`, `scan interrupted`, `done` |
| `udp` | `dns resolved`, `dns authoritative`, `probe open`, `probe closed`, `probe open|filtered`, `probe error`, `done` |
| `ntp` | `dns resolved`, `dns authoritative`, `dns resolution failed`, `response`, `query failed`, `done` |
| `wol` | `magic packet sent`, `send failed`, `done` |
| `rdns` | `dns resolved`, `dns authoritative`, `dns resolution failed`, `reverse lookup`, `reverse lookup failed`, `done` |
| `dns` | `server resolved`, `dns resolution failed`, `query`, `query failed`, `answer`, `authority`, `no server to ask`, `done` |
| `cidr` | `subnet`, `done` |
| `ip` | `interface`, `failed`, `done` |
| `listen-tcp` | `listening`, `connection accepted`, `data received`, `connection closed`, `done` |
| `listen-udp` | `listening`, `packet received`, `done` |
| `listen-http` | `listening`, `request`, `request rejected`, `request incomplete`, `done` |

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
| `module_name` | string | `telnet`, `icmp`, `web`, `nmap`, `udp`, `ntp`, `rdns` or `dns` - and `wol`, `cidr` or `ip`, whose documents have no `dns_lookup`. |
| `dns_lookup` | object | Result of resolving the host name. **Absent for `wol`, `cidr`, `ip` and `dns`** (`dns` is itself the lookup), which never look up a name (an empty lookup would only read as a failed one). |
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
| `protocol` | `tcp`, `udp`, `icmp` or `dns` (`rdns`). |
| `timeout_ms` | The `--timeout` value. **Despite the name, it is in seconds.** |
| `count`, `delay_ms`, `throttle` | `--count`, `--delay` (milliseconds) and `--throttle`. |
| `payload_bytes` | Payload size: filler size for `ping`/`udp`, request body size for `web`. |
| `method`, `data`, `headers` | HTTP method, body and headers (`web` only). `data` is also where `wol` puts the MAC address, `cidr` its comma-separated prefixes and `ip` the interface name asked for. |
| `sequential` | Always `false`; reserved. |

### dns_lookup

| Field | Meaning |
|---|---|
| `hostname` | The name that was resolved. |
| `resolved_addresses` | Every address it resolved to, IPv4 and IPv6. |
| `success`, `error` | Whether the lookup worked, and why not. |
| `time_taken_µs` | How long it took. |
| `authoritative` | Present when the host is a name that has a zone: `{zone, nameservers, time_taken_µs}` - the zone it belongs to and that zone's name servers (see [Who runs the DNS for this name](usage.md#who-runs-the-dns-for-this-name)). When none could be found it has `error` and no `zone`. Absent for an IP address or a single-label name. |

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
| `payload_size_bytes` | The echo payload size in bytes (`--payload`). |
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
| `timing` | Only with `--timing`: `hops`, one entry per hop (the request and each redirect it followed), each with `url`, `status_code` (`0` if that hop got no response), `reused_connection`, and the microsecond times `dns_µs`, `connect_µs`, `tls_µs`, `wait_µs`, `download_µs`, `total_µs`. See [web](web.md#timing-where-the-time-went). |

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

**ntp**

| Field | Meaning |
|---|---|
| `address`, `success` | The server address asked, and whether a trustworthy answer came. |
| `stratum`, `version`, `leap_indicator`, `reference_id` | What the server said about itself (`leap_indicator` is `none`, `insert`, `delete` or `unsynchronized`). |
| `offset_µs` | The server's time minus this machine's, **signed**: positive means this clock is behind. |
| `round_trip_µs` | The network delay of the exchange, without the server's processing time. |
| `server_unixtime_µs` | The time the server reported. |
| `sent_unixtime_µs`, `recv_unixtime_µs`, `time_taken_µs` | Timing of the exchange. |
| `error` | Present only on failure. |

**wol**

| Field | Meaning |
|---|---|
| `mac`, `address`, `port` | The target MAC address, and where the packet was sent. |
| `success` | Whether the operating system accepted the packet for sending. It says nothing about whether the machine woke: there is no reply. |
| `bytes_sent` | `102` for a magic packet. |
| `sent_unixtime_µs`, `time_taken_µs` | When it was sent and how long the send took. |
| `error` | Present only on failure. |

**rdns**

| Field | Meaning |
|---|---|
| `address` | The address that was looked up. |
| `query` | The `in-addr.arpa.` / `ip6.arpa.` name that was asked for. |
| `names` | The host names it maps back to. A list - empty, never `null`, when there are none. |
| `success` | Whether at least one name came back. |
| `sent_unixtime_µs`, `recv_unixtime_µs`, `time_taken_µs` | Timing. |
| `error` | Present only on failure; "no such host" means the address has no PTR record. |

**cidr** - one entry per prefix, no timing (see [cidr](cidr.md#what-it-reports) for what each field means)

| Field | Meaning |
|---|---|
| `input`, `network`, `family`, `prefix_length`, `netmask`, `wildcard`, `first_address`, `last_address`, `broadcast`, `first_host`, `last_host`, `kind` | The subnet, as described on the page. `wildcard` and `broadcast` appear only where they exist. |
| `addresses`, `usable_hosts` | **Strings**, not numbers: an IPv6 prefix can hold more addresses than a 64-bit integer, and JSON tools would silently round them. |

**dns** - one entry per question (see [dns](dns.md#reading-the-output) for what each field means)

| Field | Meaning |
|---|---|
| `name`, `type` | What was asked. |
| `nameserver`, `transport` | Who answered, and `udp` or `tcp`; `nameserver` is absent when no server answered. |
| `rcode`, `flags` | The response code (`NOERROR`, `NXDOMAIN`, `SERVFAIL`, ... - capitals here, lower case in the text log) and the header flags set (`qr`, `aa`, `tc`, `rd`, `ra`, `ad`, `cd`). |
| `success` | Whether the server answered with at least one record of the type asked. |
| `answers`, `authority` | Lists of `{name, type, ttl, data}` (a TXT record also has `strings`). Empty, never `null`. |
| `additional_count` | How many records the additional section held (not listed). |
| `retried_over_tcp`, `skipped_servers` | Present when the UDP answer was truncated, or when servers tried first did not answer. |
| `sent_unixtime_µs`, `recv_unixtime_µs`, `time_taken_µs` | Timing. |
| `error` | Present only on failure. |

**ip** - one entry per interface, no timing (see [ip](ip.md#what-it-reports) for what each field means)

| Field | Meaning |
|---|---|
| `name`, `state`, `mtu`, `mac`, `flags` | The interface. `state` is `up` or `down`; `flags` a list of words; `mac` is left out when the interface has none. |
| `ipv4`, `ipv6` | Its addresses of that family as plain strings with their prefix length (`"192.168.1.20/24"`): lists that are empty, never `null`, when it has none (or after `-4`/`-6`). |
| `error` | Present only when the addresses of this interface could not be read. |

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
| `method`, `path`, `status_code` | The request line's method and path, and the status answered. A request that was not answered has `status_code` 0 (`method` and `path` only if the request line arrived); one answered `400` before it could be read has that status and no method or path. |
| `remote_address` | The client. |
| `bytes_received`, `bytes_sent` | Raw bytes on the connection in each direction, headers and body included. |
| `processing_time_µs` | Time the handler took. |
| `unixtime_µs` | When the connection finished. |
| `error` | Present only for a request that was not served: what went wrong (see [A request that was not served](listen.md#a-request-that-was-not-served)). |

## Units at a glance

| Where | Unit |
|---|---|
| Timestamps and durations in JSON | microseconds (`_µs`), except `icmp` which uses milliseconds (`_ms`). `offset_µs` (`ntp`) is signed |
| `--timeout`, and `timeout_ms` in JSON | seconds |
| `--delay`, and `delay_ms` in JSON | milliseconds |
| `bytes_*` | bytes |
| `bandwidth_kbs`, `speed=` | kilobytes per second (1 KB = 1024 bytes) |
