#!/bin/bash

# A script to run integration tests on the Go Telnet application.
# Exit immediately if a command exits with a non-zero status.
set -e

# --- Setup ---
# Keep track of the number of failures
failures=0
# The application binary name
BINARY="./shint"

# ANSI Color Codes
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Helper function to run a test
# Arguments:
#   $1: Test description
#   $2: Command to execute
#   $3: String to expect in the output
run_test() {
    local description="$1"
    local command="$2"
    local expect="$3"
    
    echo -n "TEST: $description ... "
    
    # Execute the command and capture its output and exit code. The `||`
    # matters: shint now exits non-zero when a check fails (1) or the command
    # is misused (2), and under `set -e` a bare failing assignment would end
    # the whole script instead of being counted as one failed test.
    exit_code=0
    output=$(eval $command 2>&1) || exit_code=$?
    
    # Check if the command was successful and the output contains the expected string
    if [[ $exit_code -eq 0 && "$output" == *"$expect"* ]]; then
        printf "${GREEN}[PASS]${NC}\n"
    else
        printf "${RED}[FAIL]${NC}\n"
        echo "  - Exit Code: $exit_code"
        echo "  - Expected to contain: '$expect'"
        echo "  - Output:"
        echo "$output"
        failures=$((failures + 1))
    fi
}

# --- Build ---
echo "Building the application..."
go build -o $BINARY main.go
echo "Build complete."
echo ""

# --- Test Cases ---

# Web Tests
run_test "Web GET" "$BINARY web https://google.com --count 1" 'status=200'
run_test "Web POST" "$BINARY web -X POST https://httpbin.org/post --count 1" 'status=200'
run_test "Web GET with JSON output" "$BINARY web https://google.com --json --count 1" '"status_code": 200'

# Nmap Test
run_test "Nmap Scan" "$BINARY nmap --from 80 --to 80 google.com" "port open"

# Telnet Test
run_test "Telnet" "$BINARY telnet google.com 443" "connect ok"

# UDP Test (result state against a real host is inherently best-effort - see
# README's UDP section - so this only checks the probe ran, not its outcome)
run_test "UDP" "$BINARY udp 8.8.8.8 53 --data test" "[udp] OK probe"

# Ping Test
run_test "Ping" "$BINARY ping google.com --count 1" "icmp STATISTICS"
run_test "Ping shows the payload size" "$BINARY ping google.com --count 1 --payload 16" "bytes=16"

# Timing, reverse DNS, clock check, Wake-on-LAN (aimed at this machine, so
# nothing is woken) and the subnet calculator (offline)
run_test "Telnet, IPv4 only" "$BINARY telnet google.com 443 -4" "addresses=1"
run_test "Web timing" "$BINARY web https://google.com --timing --count 1" "timing url="
run_test "Reverse DNS" "$BINARY rdns 8.8.8.8" "dns.google."
run_test "NTP" "$BINARY ntp time.cloudflare.com" "[ntp] OK response"
run_test "Wake-on-LAN" "$BINARY wol aa:bb:cc:dd:ee:ff --broadcast 127.0.0.1 --port 9" "magic packet sent"
run_test "CIDR" "$BINARY cidr 192.168.1.10/24" "network=192.168.1.0/24"
run_test "IP" "$BINARY ip" "[ip] OK done interfaces="


# --- Summary ---
echo ""
if [ $failures -eq 0 ]; then
    printf "${GREEN}All tests passed successfully!${NC}\n"
    rm $BINARY
    exit 0
else
    printf "${RED}%d test(s) failed.${NC}\n" "$failures"
    rm $BINARY
    exit 1
fi
