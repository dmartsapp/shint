#!/usr/bin/env bash
# Tests for install-tools.sh, against archives served from file:// - nothing
# is downloaded from the network, and nothing installed is run.
#
#   bash .github/scripts/test-install-tools.sh
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0
pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1"; fails=$((fails + 1)); }

if command -v sha256sum >/dev/null 2>&1; then hasher=(sha256sum); else hasher=(shasum -a 256); fi

# A fake release of each tool, laid out as GitHub serves them: three archives, and Hadolint's bare binary.
for x in "syft 1.55.0 anchore/syft syft_1.55.0_linux_amd64.tar.gz" "grype 0.121.0 anchore/grype grype_0.121.0_linux_amd64.tar.gz" \
         "dockle 0.4.15 goodwithtech/dockle dockle_0.4.15_Linux-64bit.tar.gz"; do
  read -r tool version repo file <<<"$x"
  mkdir -p "$work/src/$tool" "$work/site/$repo/releases/download/v$version"
  printf '#!/bin/sh\necho fake %s\n' "$tool" > "$work/src/$tool/$tool"
  tar -czf "$work/site/$repo/releases/download/v$version/$file" -C "$work/src/$tool" "$tool"
done
mkdir -p "$work/site/hadolint/hadolint/releases/download/v2.15.1"
printf '#!/bin/sh\necho fake hadolint\n' > "$work/site/hadolint/hadolint/releases/download/v2.15.1/hadolint-linux-x86_64"
export INSTALL_TOOLS_BASE_URL="file://$work/site"
sum() { "${hasher[@]}" "$work/site/$1" | cut -d' ' -f1; }

out="$(INSTALL_TOOLS_SHA256_SYFT="$(sum anchore/syft/releases/download/v1.55.0/syft_1.55.0_linux_amd64.tar.gz)" \
  INSTALL_TOOLS_SHA256_GRYPE="$(sum anchore/grype/releases/download/v0.121.0/grype_0.121.0_linux_amd64.tar.gz)" \
  INSTALL_TOOLS_SHA256_DOCKLE="$(sum goodwithtech/dockle/releases/download/v0.4.15/dockle_0.4.15_Linux-64bit.tar.gz)" \
  INSTALL_TOOLS_SHA256_HADOLINT="$(sum hadolint/hadolint/releases/download/v2.15.1/hadolint-linux-x86_64)" \
  bash "$here/install-tools.sh" "$work/ok" syft grype dockle hadolint 2>&1)"
if [ $? -eq 0 ] && [ -x "$work/ok/syft" ] && [ -x "$work/ok/grype" ] && [ -x "$work/ok/dockle" ] && [ -x "$work/ok/hadolint" ] \
   && ! ls "$work/ok"/*.tar.gz "$work/ok"/hadolint-* >/dev/null 2>&1; then
  pass "downloads whose SHA-256 matches are installed - archives unpacked, a bare binary renamed - and made executable"
else
  fail "matching downloads: $out"
fi
case "$out" in *"syft 1.55.0:"*"SHA-256 of syft_1.55.0_linux_amd64.tar.gz checked"*) pass "it says which version, and that it checked" ;; *) fail "the output: $out" ;; esac

# The real pins: these fake archives do not match them.
out="$(bash "$here/install-tools.sh" "$work/bad" syft 2>&1)"
if [ $? -ne 0 ] && [ ! -e "$work/bad/syft" ] && [ -z "$(ls -A "$work/bad")" ]; then
  pass "an archive that does not match the pin is refused, and nothing is left"
else
  fail "a mismatched archive: $out"
fi
case "$out" in *"not the pinned bb65249542db5fedeeb4d3f9b018b75e7264dbfbbb10486f044581a92f90490d"*) pass "and the message names the pin" ;; *) fail "the message: $out" ;; esac

if ! bash "$here/install-tools.sh" "$work/x" cosign >/dev/null 2>&1; then pass "a tool with no pin is refused"; else fail "an unpinned tool was installed"; fi
if ! bash "$here/install-tools.sh" "$work/x" >/dev/null 2>&1; then pass "no tool named is refused"; else fail "no tool named"; fi
if ! INSTALL_TOOLS_BASE_URL="file://$work/nowhere" INSTALL_TOOLS_SHA256_SYFT=x bash "$here/install-tools.sh" "$work/x" syft >/dev/null 2>&1; then
  pass "a download that fails stops it"
else
  fail "a failed download"
fi

[ "$fails" -eq 0 ] || { echo "$fails failure(s)"; exit 1; }
