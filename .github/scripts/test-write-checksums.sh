#!/usr/bin/env bash
# Tests for write-checksums.sh:  bash .github/scripts/test-write-checksums.sh
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/write-checksums.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0
check() { if [ "$2" = 0 ]; then echo "PASS  $1"; else echo "FAIL  $1"; failures=$((failures+1)); fi; }

if command -v sha256sum >/dev/null 2>&1; then verify=(sha256sum -c); else verify=(shasum -a 256 -c); fi

mkdir "$tmp/bin"
head -c 4096 /dev/urandom > "$tmp/bin/shint.linux.amd64"
head -c 5000 /dev/urandom > "$tmp/bin/shint.windows.amd64.exe"
echo "notes" > "$tmp/bin/README.txt"          # not a binary: must be ignored

bash "$script" "$tmp/bin" > "$tmp/out.txt" 2>&1
check "runs"                                   "$?"
check "one .sha256 per binary"                 "$([ -f "$tmp/bin/shint.linux.amd64.sha256" ] && [ -f "$tmp/bin/shint.windows.amd64.exe.sha256" ]; echo $?)"
check "nothing for other files"                "$([ ! -e "$tmp/bin/README.txt.sha256" ]; echo $?)"
check "reports the count"                      "$(grep -q 'wrote 2 checksum' "$tmp/out.txt"; echo $?)"

line="$(cat "$tmp/bin/shint.linux.amd64.sha256")"
check "format is '<64 hex>  <plain name>'"     "$([[ "$line" =~ ^[0-9a-f]{64}\ \ shint\.linux\.amd64$ ]]; echo $?)"

want="$(python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "$tmp/bin/shint.linux.amd64")"
check "hash matches an independent SHA-256"    "$([ "${line%% *}" = "$want" ]; echo $?)"

( cd "$tmp/bin" && "${verify[@]}" shint.linux.amd64.sha256 shint.windows.amd64.exe.sha256 >/dev/null 2>&1 )
check "the platform's own tool verifies them"  "$?"

bash "$script" "$tmp/bin" >/dev/null 2>&1
check "running again adds no .sha256.sha256"   "$([ ! -e "$tmp/bin/shint.linux.amd64.sha256.sha256" ]; echo $?)"

echo tampered >> "$tmp/bin/shint.linux.amd64"
( cd "$tmp/bin" && "${verify[@]}" shint.linux.amd64.sha256 >/dev/null 2>&1 ); rc=$?
check "a modified binary fails verification"   "$([ $rc -ne 0 ]; echo $?)"

mkdir "$tmp/empty"
bash "$script" "$tmp/empty" >/dev/null 2>&1; rc=$?
check "a directory without binaries is an error" "$([ $rc -ne 0 ]; echo $?)"
bash "$script" >/dev/null 2>&1; rc=$?
check "a missing directory argument is an error" "$([ $rc -ne 0 ]; echo $?)"

echo
if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
