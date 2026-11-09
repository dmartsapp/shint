#!/usr/bin/env bash
# Installs the third-party tools the image workflows use - Syft (image SBOMs)
# and Grype (image vulnerability reports) - at the versions pinned here, for
# linux/amd64 (the runners), each archive checked against the SHA-256 pinned
# here before anything in it runs:
#
#   bash .github/scripts/install-tools.sh "$RUNNER_TEMP/tools" syft grype
#
# A new version means a new pin: its version, and the SHA-256 of its
# linux_amd64 archive from the project's own checksums file on its GitHub
# release - a release at least two weeks old (docs/src/tech-ci.md).
set -euo pipefail

dir="${1:?usage: install-tools.sh <directory> <tool>...}"
shift
[ "$#" -gt 0 ] || { echo "install-tools.sh: name a tool: syft, grype" >&2; exit 2; }
base="${INSTALL_TOOLS_BASE_URL:-https://github.com}"

pin() {
  case "$1" in
    syft)  echo "1.55.0 bb65249542db5fedeeb4d3f9b018b75e7264dbfbbb10486f044581a92f90490d" ;;
    grype) echo "0.121.0 64e07133d3486115de647a66581461dccf5c732748a0a3119077395565f15519" ;;
    *) return 1 ;;
  esac
}

if command -v sha256sum >/dev/null 2>&1; then
  hasher=(sha256sum)
else
  hasher=(shasum -a 256)
fi

mkdir -p "$dir"
for tool in "$@"; do
  read -r version sum <<<"$(pin "$tool")" || { echo "install-tools.sh: no pin for $tool" >&2; exit 2; }
  # Tests only: the SHA-256 of their own archive.
  override="INSTALL_TOOLS_SHA256_$(echo "$tool" | tr '[:lower:]' '[:upper:]')"
  sum="${!override:-$sum}"
  archive="${tool}_${version}_linux_amd64.tar.gz"
  curl -fsSL --retry 3 -o "$dir/$archive" "$base/anchore/$tool/releases/download/v$version/$archive"
  got="$("${hasher[@]}" "$dir/$archive" | cut -d' ' -f1)"
  if [ "$got" != "$sum" ]; then
    rm -f "$dir/$archive"
    echo "install-tools.sh: $archive has SHA-256 $got, not the pinned $sum - not installed" >&2
    exit 1
  fi
  tar -xzf "$dir/$archive" -C "$dir" "$tool"
  rm -f "$dir/$archive"
  echo "$tool $version: $dir/$tool (SHA-256 of $archive checked)"
done
