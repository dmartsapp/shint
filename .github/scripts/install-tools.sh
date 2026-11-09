#!/usr/bin/env bash
# Installs the third-party tools the workflows use - Syft (image SBOMs), Grype
# (image vulnerability reports), Dockle (image checks against the CIS Docker
# Benchmark) and Hadolint (the Dockerfile) - at the versions pinned here, for
# linux/amd64 (the runners), each download checked against the SHA-256 pinned
# here before anything in it runs:
#
#   bash .github/scripts/install-tools.sh "$RUNNER_TEMP/tools" syft grype dockle
#
# A new version means a new pin: its version, and the SHA-256 of its
# linux/amd64 download from the project's own checksums file on its GitHub
# release - a release at least two weeks old (docs/src/tech-ci.md).
set -euo pipefail

dir="${1:?usage: install-tools.sh <directory> <tool>...}"
shift
[ "$#" -gt 0 ] || { echo "install-tools.sh: name a tool: syft, grype, dockle, hadolint" >&2; exit 2; }
base="${INSTALL_TOOLS_BASE_URL:-https://github.com}"

# <version> <sha256> <repository> <file> <tar: an archive holding the tool | bin: the tool itself>
pin() {
  case "$1" in
    syft)     echo "1.55.0 bb65249542db5fedeeb4d3f9b018b75e7264dbfbbb10486f044581a92f90490d anchore/syft syft_1.55.0_linux_amd64.tar.gz tar" ;;
    grype)    echo "0.121.0 64e07133d3486115de647a66581461dccf5c732748a0a3119077395565f15519 anchore/grype grype_0.121.0_linux_amd64.tar.gz tar" ;;
    dockle)   echo "0.4.15 8bfc9183d1d3d67800edb45bea4bb63ddf1d28bec5f89944075a07007d79caad goodwithtech/dockle dockle_0.4.15_Linux-64bit.tar.gz tar" ;;
    hadolint) echo "2.15.1 c7187db94eeeeca956519a6af171adc31453941a1e777961f6e680f697c8c507 hadolint/hadolint hadolint-linux-x86_64 bin" ;;
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
  read -r version sum repo archive kind <<<"$(pin "$tool")" || { echo "install-tools.sh: no pin for $tool" >&2; exit 2; }
  # Tests only: the SHA-256 of their own download.
  override="INSTALL_TOOLS_SHA256_$(echo "$tool" | tr '[:lower:]' '[:upper:]')"
  sum="${!override:-$sum}"
  curl -fsSL --retry 3 -o "$dir/$archive" "$base/$repo/releases/download/v$version/$archive"
  got="$("${hasher[@]}" "$dir/$archive" | cut -d' ' -f1)"
  if [ "$got" != "$sum" ]; then
    rm -f "$dir/$archive"
    echo "install-tools.sh: $archive has SHA-256 $got, not the pinned $sum - not installed" >&2
    exit 1
  fi
  if [ "$kind" = tar ]; then
    tar -xzf "$dir/$archive" -C "$dir" "$tool"
    rm -f "$dir/$archive"
  else
    mv "$dir/$archive" "$dir/$tool"
  fi
  chmod +x "$dir/$tool"
  echo "$tool $version: $dir/$tool (SHA-256 of $archive checked)"
done
