#!/usr/bin/env bash
# Uploads files to a tag's GitHub release - which the Binary Build & Release
# workflow creates, while the image and vulnerability workflows run beside it -
# waiting for the release to exist first:
#
#   GH_TOKEN=... bash .github/scripts/upload-release-assets.sh v4.3.0 image-sboms/*
#
# A file already there is replaced (--clobber), so a re-run job can upload
# again. It waits UPLOAD_WAIT seconds at most (default 1800), checking every
# UPLOAD_EVERY (default 20).
set -euo pipefail

tag="${1:?usage: upload-release-assets.sh <tag> <file>...}"
shift
[ "$#" -gt 0 ] || { echo "upload-release-assets.sh: no files to upload" >&2; exit 2; }
for f in "$@"; do
  [ -f "$f" ] || { echo "upload-release-assets.sh: $f is not a file" >&2; exit 2; }
done
wait_s="${UPLOAD_WAIT:-1800}"
every="${UPLOAD_EVERY:-20}"
gh="${GH:-gh}"

waited=0
until "$gh" release view "$tag" --json tagName >/dev/null 2>&1; do
  if [ "$waited" -ge "$wait_s" ]; then
    echo "upload-release-assets.sh: no release for $tag after ${wait_s}s" >&2
    exit 1
  fi
  sleep "$every"
  waited=$((waited + every))
done
"$gh" release upload "$tag" "$@" --clobber
echo "uploaded $# file(s) to the $tag release (after waiting ${waited}s for it)"
