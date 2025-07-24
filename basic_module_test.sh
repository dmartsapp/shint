#!/bin/bash

# A script to run integration tests on the Go Telnet application.
# Exit immediately if a command exits with a non-zero status.
set -e

# --- Setup ---
# Keep track of the number of failures
failures=0
# The application binary name
BINARY="./telnet"

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
    
    # Execute the command and capture its output and exit code
    output=$(eval $command 2>&1)
    exit_code=$?
    
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
run_test "Web GET" "$BINARY web https://google.com --count 1" "Response: 200 OK"
run_test "Web POST" "$BINARY web -X POST https://httpbin.org/post --count 1" "Response: 200 OK"
run_test "Web GET with JSON output" "$BINARY web https://google.com --json --count 1" '"status_code": 200'

# Nmap Test
run_test "Nmap Scan" "$BINARY nmap --from 80 --to 80 google.com" "has port 80 open"

# Telnet Test
run_test "Telnet" "$BINARY telnet google.com 443" "Successfully connected"

# Ping Test
run_test "Ping" "$BINARY ping google.com --count 1" "Packets sent: 1, Packets received: 1"


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
