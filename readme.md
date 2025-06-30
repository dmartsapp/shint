# Go Telnet v2.0.0

A simple, modern, and versatile network utility tool built with Go. This tool provides a convenient way to perform common network checks, including `telnet`, `ping`, `nmap`, and `web` requests.

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

The `web` command makes an HTTP GET request to a URL and displays the response.

**Syntax:**

```bash
./telnet web [url]
```

**Example:**

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

**JSON Output:**

```json
{
  "input_params": {
    "module_name": "web",
    "sequential": false,
    "throttle": false,
    "host": "google.com",
    "from_port": 0,
    "to_port": 0,
    "protocol": "tcp",
    "timeout_ms": 5,
    "count": 1,
    "delay_ms": 1000,
    "payload_bytes": 0
  },
  "module_name": "web",
  "dns_lookup": {
    "hostname": "google.com",
    "resolved_addresses": null,
    "error": "",
    "success": false,
    "time_taken_µs": 0
  },
  "stats": [
    {
      "url": "https://google.com",
      "success": true,
      "recv_unixtime_µs": 1751305214850478,
      "sent_unixtime_µs": 1751305214633820,
      "time_taken_µs": 216658,
      "bytes_downloaded": 17779,
      "status_code": 200
    }
  ],
  "end_time_unixtime_µs": 1751305214850486,
  "start_time_unixtime_µs": 1751305214633753,
  "total_time_taken_µs": 216733,
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