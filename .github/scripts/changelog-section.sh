#!/usr/bin/env bash
# Print the CHANGELOG.md section of one release - what the GitHub release page shows as "What's new":
#
#   bash .github/scripts/changelog-section.sh v4.2.0 [CHANGELOG.md]
#
# Everything under the "## v4.2.0 - date" heading up to the next "## v" heading, without the
# heading and without blank lines at either end. Prints nothing (and exits 0) when there is no
# such section, so the caller can fall back. The release page used to show the commit message of
# the commit the tag is on, which is the wrong text whenever that is not the release commit.
set -uo pipefail
tag="${1:?usage: changelog-section.sh vX.Y.Z [CHANGELOG.md]}"
file="${2:-CHANGELOG.md}"
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "changelog-section: '$tag' is not vX.Y.Z" >&2; exit 1; }
[ -f "$file" ] || { echo "changelog-section: no $file" >&2; exit 1; }
awk -v want="## $tag" '
  /^## v[0-9]/ { if (on) exit; split($0, a, " - "); if (a[1] == want) { on = 1 }; next }
  on { lines[++n] = $0 }
  END {
    first = 1; while (first <= n && lines[first] == "") first++
    last = n;  while (last >= first && lines[last] == "") last--
    for (i = first; i <= last; i++) print lines[i]
  }' "$file"
