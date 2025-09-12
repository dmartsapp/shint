# Go Telnet v2.x.y

## Introduction

A simple, modern, and versatile network utility tool built with Go. This tool provides a convenient way to perform common network checks, including `telnet`, `ping`, `nmap`, and `web` requests.

[![Build and release telnet binary](https://github.com/dmartsapp/telnet/actions/workflows/actions.yaml/badge.svg)](https://github.com/dmartsapp/telnet/actions/workflows/actions.yaml)

**Note:** Version 2.0.0 introduces a new subcommand-based interface. See the [Usage](#usage) section for details.

## Features

- **Telnet:** Test connectivity to a host on a specific port.
- **Ping:** Send ICMP ECHO_REQUEST packets to a host to test reachability.
- **Web:** Make an HTTP GET request to a URL and display the response.
- **Nmap:** Scan for open TCP ports on a host within a given range.
- **JSON Output:** All commands support JSON output for easy parsing and integration with other tools.
- **Cross-Platform:** Binaries are available for Linux, macOS, and Windows.

## Installation

You can download the latest release for your operating system from the [releases page](https://github.com/farhansabbir/telnet/releases/latest).

Alternatively, you can build the project from source:

```bash
git clone https://github.com/farhansabbir/telnet.git
cd telnet
make
```

## Usage

### Telnet

The `telnet` command attempts to connect to a host on a specific port.

**Syntax:**

```bash
./telnet telnet [host] [port]
```

**Example:**

```bash
./telnet telnet google.com 443
```

**Output:**

```
Mon Jun 30 13:23:25 EDT 2025: DNS lookup successful for google.com' to 1 addresses '[142.251.41.46]' in 13.048166ms
Mon Jun 30 13:23:26 EDT 2025: Successfully connected to 142.251.41.46 on port 443 after 7.076ms

======================================= telnet STATISTICS =======================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 7.076ms, average: 7.076ms, maximum: 7.076ms
Total time taken: 1.021345708s
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
    "resolved_addresses": [
      "142.251.41.46"
    ],
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

The `ping` command sends ICMP ECHO_REQUEST packets to a host to test reachability.

**Syntax:**

```bash
./telnet ping [host]
```

**Example:**

```bash
./telnet ping google.com
```

**Output:**

```
Mon Jun 30 13:23:32 EDT 2025: Received response for request #1 from 142.251.41.46 with 4 bytes of data in 9ms
========================================= Ping stats ============================================
Packets sent: 1, Packets received: 1, Packets lost: 0, Ping success: 100% 
Total time: 1.011017458s, Resolve time: 1.409125ms
Min time: 9ms, Max time: 9ms, Avg time: 9.000ms, Std dev: 0.000, Total time: 1.011017458s
```

**JSON Output:**

```json
{
  "input_params": {
    "module_name": "icmp",
    "sequential": false,
    "throttle": false,
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
    "resolved_addresses": [
      "142.251.41.46"
    ],
    "error": "",
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

The `web` command makes an HTTP request to a URL and displays the response. It can be used for simple GET requests or as a more advanced REST client.

**Syntax:**

```bash
./telnet web [url] [flags]
```

**Example (Simple GET):**

```bash
./telnet web https://google.com
```

**Output:**

```
Mon Jun 30 13:23:36 EDT 2025: DNS lookup successful for google.com' to 1 addresses '[142.251.41.46]' in 1.377ms
Mon Jun 30 13:23:36 EDT 2025: Response: 200 OK, bytes downloaded: 17722, speed: 73.97030287755966KB/s, time taken: 233.967416ms

========================================== web STATISTICS ==========================================
Requests sent: 1, Response received: 1, Success: 100%
Latency: minimum: 233.967416ms, average: 233.967416ms, maximum: 233.967416ms
Total time taken: 235.525041ms
```

### REST Client (Advanced: from v2.2.0)

The `web` command includes a powerful REST client for making API requests. You can specify the HTTP method, send a request body, and add custom headers.

**Flags:**

*   `-X`, `--method`: The HTTP method to use (e.g., `GET`, `POST`, `PUT`, `DELETE`). Defaults to `GET`.
*   `-P`, `--payload`: The HTTP payload (request body) to send.
*   `-H`, `--header`: An HTTP header to include in the request. This flag can be specified multiple times for multiple headers (e.g., `-H "Content-Type: application/json" -H "Authorization: Bearer <token>"`).
*   `-W`, `--withbody`: Include the full response body in the JSON output.

**Example (POST Request with JSON):**

This example sends a POST request with a JSON payload and a custom `Content-Type` header. The output is requested in JSON format and includes the full response body.

```bash
./telnet web -X POST -P '{"name": "test"}' -H "Content-Type: application/json" --json -W https://httpbin.org/post
```

**JSON Output (POST Request):**

```json
{
  "input_params": {
    "module_name": "web",
    "sequential": false,
    "throttle": false,
    "host": "httpbin.org",
    "from_port": 443,
    "to_port": 443,
    "protocol": "tcp",
    "timeout_ms": 5,
    "count": 1,
    "delay_ms": 1000,
    "payload_bytes": 15,
    "method": "POST",
    "data": "{\"name\": \"test\"}",
    "headers": [
      "Content-Type: application/json"
    ]
  },
  "module_name": "web",
  "dns_lookup": {
    "hostname": "httpbin.org",
    "resolved_addresses": [
      "3.231.133.143",
      "54.162.127.135",
      "..."
    ],
    "error": "",
    "success": true,
    "time_taken_µs": 12345
  },
  "stats": [
    {
      "url": "https://httpbin.org/post",
      "errors": [],
      "request": {
        "body": {},
        "headers": {
          "Content-Type": [
            "application/json"
          ],
          "User-Agent": [
            "dmarts.app-http-v0.1"
          ]
        },
        "method": "POST"
      },
      "response": {
        "body": {
          "args": {},
          "data": "{\"name\": \"test\"}",
          "files": {},
          "form": {},
          "headers": {
            "Accept-Encoding": "gzip",
            "Content-Length": "15",
            "Content-Type": "application/json",
            "Host": "httpbin.org",
            "User-Agent": "dmarts.app-http-v0.1",
            "X-Amzn-Trace-Id": "Root=1-689e3a4b-..."
          },
          "json": {
            "name": "test"
          },
          "origin": "...",
          "url": "https://httpbin.org/post"
        },
        "header": {
          "Access-Control-Allow-Credentials": [
            "true"
          ],
          "Access-Control-Allow-Origin": [
            "*"
          ],
          "Content-Length": [
            "484"
          ],
          "Content-Type": [
            "application/json"
          ],
          "Date": [
            "Mon, 24 Jul 2025 18:00:00 GMT"
          ],
          "Server": [
            "gunicorn/19.9.0"
          ]
        }
      },
      "success": true,
      "recv_unixtime_µs": 1753380000123456,
      "sent_unixtime_µs": 1753380000000000,
      "time_taken_µs": 123456,
      "bytes_downloaded": 1024,
      "status_code": 200
    }
  ],
  "end_time_unixtime_µs": 1753380000123456,
  "start_time_unixtime_µs": 1753379999000000,
  "total_time_taken_µs": 1123456,
  "error": ""
}
```

### Nmap


The `nmap` command scans for open TCP ports on a host within a given range.

**Syntax:**

```bash
./telnet nmap --from [start_port] --to [end_port] [host]
```

**Example:**

```bash
./telnet nmap --from 80 --to 100 google.com
```

**Output:**

```
Mon Jun 30 13:23:42 EDT 2025: DNS lookup successful for google.com' to 1 addresses '[142.251.41.46]' in 1.585083ms
Mon Jun 30 13:23:42 EDT 2025: 142.251.41.46 has port 80 open
Total time taken: 5.002086541s
```

**JSON Output:**

```json
{
  "input_params": {
    "module_name": "nmap",
    "sequential": false,
    "throttle": false,
    "host": "google.com",
    "from_port": 80,
    "to_port": 100,
    "protocol": "tcp",
    "timeout_ms": 5,
    "count": 1,
    "delay_ms": 0,
    "payload_bytes": 0
  },
  "module_name": "nmap",
  "dns_lookup": {
    "hostname": "google.com",
    "resolved_addresses": [
      "142.251.41.46"
    ],
    "error": "",
    "success": true,
    "time_taken_µs": 1826
  },
  "stats": [
    {
      "address": "142.251.41.46",
      "port": 80,
      "success": true
    },
    {
      "address": "142.251.41.46",
      "port": 100,
      "success": false
    }
  ],
  "end_time_unixtime_µs": 1751305293221854,
  "start_time_unixtime_µs": 1751305288219383,
  "total_time_taken_µs": 5002471,
  "error": ""
}
```

## Data Collection and Privacy

This tool does not collect or store any personal information. It is a command-line utility that performs network checks and displays the results to the user. The only data that is transmitted over the network is the data required to perform the requested network check (e.g., DNS queries, TCP connections, ICMP packets, HTTP requests).