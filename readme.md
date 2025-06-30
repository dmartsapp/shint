# Go Telnet

A simple, modern, and versatile network utility tool built with Go. This tool provides a convenient way to perform common network checks, including `telnet`, `ping`, `nmap`, and `web` requests.

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

### Ping

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

### Web

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

### Nmap

```bash
./telnet nmap --from 80 --to 100 google.com
```

**Output:**

```
Mon Jun 30 13:23:42 EDT 2025: DNS lookup successful for google.com' to 1 addresses '[142.251.41.46]' in 1.585083ms
Mon Jun 30 13:23:42 EDT 2025: 142.251.41.46 has port 80 open
Total time taken: 5.002086541s
```

## Data Collection and Privacy

This tool does not collect or store any personal information. It is a command-line utility that performs network checks and displays the results to the user. The only data that is transmitted over the network is the data required to perform the requested network check (e.g., DNS queries, TCP connections, ICMP packets, HTTP requests).