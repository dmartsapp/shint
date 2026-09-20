#!/usr/bin/env bash
# Writes <binary>.sha256 next to every release binary in a directory.
#
#   bash .github/scripts/write-checksums.sh bin
#
# Each file holds one line, `<64 hex digits>  <file name>`, which is what both
# `sha256sum -c` (Linux) and `shasum -a 256 -c` (macOS) read, and what
# PowerShell's Get-FileHash reproduces. The name is the plain file name, so the
# check works from the directory the two files were downloaded to. Each
# checksum is verified straight after it is written.
set -euo pipefail

dir="${1:?usage: write-checksums.sh <directory>}"
cd "$dir"

if command -v sha256sum >/dev/null 2>&1; then
  hasher=(sha256sum)
else
  hasher=(shasum -a 256)
fi

count=0
for file in shint.*; do
  case "$file" in *.sha256) continue ;; esac
  [ -f "$file" ] || continue
  "${hasher[@]}" "$file" > "$file.sha256"
  "${hasher[@]}" -c "$file.sha256" > /dev/null
  count=$((count + 1))
done

if [ "$count" -eq 0 ]; then
  echo "no shint.* binaries found in $dir" >&2
  exit 1
fi
echo "wrote $count checksum file(s) in $dir"
