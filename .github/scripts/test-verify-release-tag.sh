#!/usr/bin/env bash
# Tests for verify-release-tag.sh, with a fake `gh` so nothing touches the network.
#   bash .github/scripts/test-verify-release-tag.sh
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/verify-release-tag.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Fake gh: prints $FAKE_GH_STATUS, or fails when FAKE_GH_FAIL=1; records each call.
cat > "$tmp/gh" <<'FAKE'
#!/usr/bin/env bash
echo "$*" >> "$FAKE_GH_LOG"
[ "${FAKE_GH_FAIL:-0}" = "1" ] && { echo "gh: HTTP 500" >&2; exit 1; }
echo "${FAKE_GH_STATUS:-identical}"
FAKE
chmod +x "$tmp/gh"

failures=0
run() { # name expected_exit tag status [fail] [expect_gh_calls]
  local name="$1" want="$2" tag="$3" status="$4" ghfail="${5:-0}" want_calls="${6:-}"
  : > "$tmp/calls"
  out="$(PATH="$tmp:$PATH" FAKE_GH_LOG="$tmp/calls" FAKE_GH_STATUS="$status" FAKE_GH_FAIL="$ghfail" \
        TAG="$tag" SHA="0123456789abcdef" REPO="owner/repo" bash "$script" 2>&1)"
  got=$?
  calls="$(wc -l < "$tmp/calls" | tr -d ' ')"
  ok=1
  [ "$got" = "$want" ] || ok=0
  [ -z "$want_calls" ] || [ "$calls" = "$want_calls" ] || ok=0
  if [ "$ok" = 1 ]; then echo "PASS  $name"; else
    echo "FAIL  $name (exit $got, want $want; gh calls $calls, want ${want_calls:-any})"; echo "$out" | sed 's/^/      /'; failures=$((failures+1)); fi
}

# accepted: strict vX.Y.Z on main (tip, or an older main commit)
run "vX.Y.Z on main's tip"            0 v4.0.4    identical 0 1
run "vX.Y.Z on an older main commit"  0 v4.0.4    behind    0 1
run "multi-digit version"             0 v10.20.30 identical 0 1

# refused: not on main - the accident this exists for
run "tag on a release branch (ahead)" 1 v4.0.4    ahead     0 1
run "tag on a diverged commit"        1 v4.0.4    diverged  0 1

# refused: wrong format - must be rejected WITHOUT asking GitHub anything
for bad in v4.0.4-rc1 v1.2.3.4 vx.y.z v4.0 4.0.4 v4.0.4+build "release/v4.0.4" "" v; do
  case "$bad" in "") continue;; esac
  run "bad tag '$bad'" 1 "$bad" identical 0 0
done

# fails closed when GitHub cannot answer
run "API error refuses the release"   1 v4.0.4    identical 1 1

# missing inputs are an error, not a pass
if ( unset TAG; PATH="$tmp:$PATH" SHA=x REPO=y bash "$script" >/dev/null 2>&1 ); then
  echo "FAIL  missing TAG accepted"; failures=$((failures+1)); else echo "PASS  missing TAG is an error"; fi

echo
if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
