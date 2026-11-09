#!/usr/bin/env bash
# Tests for upload-release-assets.sh, with a fake gh.
#
#   bash .github/scripts/test-upload-release-assets.sh
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0
pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1"; fails=$((fails + 1)); }

# The fake gh: the release appears after $APPEARS_AFTER views; every call is logged.
cat > "$work/gh" <<'EOF'
#!/usr/bin/env bash
echo "$*" >> "$FAKE_LOG"
if [ "$1 $2" = "release view" ]; then
  n=$(($(cat "$FAKE_COUNT" 2>/dev/null || echo 0) + 1)); echo "$n" > "$FAKE_COUNT"
  [ "$n" -gt "${APPEARS_AFTER:-0}" ] || exit 1
  echo '{"tagName":"v9.9.9"}'; exit 0
fi
[ "$1 $2" = "release upload" ] && exit 0
exit 3
EOF
chmod +x "$work/gh"
touch "$work/a.cdx.json" "$work/b.spdx.json"
export GH="$work/gh" UPLOAD_EVERY=0

run() { rm -f "$work/log" "$work/count"; FAKE_LOG="$work/log" FAKE_COUNT="$work/count" bash "$here/upload-release-assets.sh" "$@" 2>&1; }

out="$(APPEARS_AFTER=2 run v9.9.9 "$work/a.cdx.json" "$work/b.spdx.json")"
if [ $? -eq 0 ] && [ "$(grep -c 'release view v9.9.9' "$work/log")" -eq 3 ] \
   && grep -qx "release upload v9.9.9 $work/a.cdx.json $work/b.spdx.json --clobber" "$work/log"; then
  pass "it waits for the release, then uploads every file, replacing any already there"
else
  fail "waiting and uploading: $out / $(cat "$work/log" 2>/dev/null)"
fi
case "$out" in *"uploaded 2 file(s) to the v9.9.9 release"*) pass "and says so" ;; *) fail "the output: $out" ;; esac

out="$(APPEARS_AFTER=1000 UPLOAD_WAIT=0 run v9.9.9 "$work/a.cdx.json")"
if [ $? -ne 0 ] && ! grep -q 'release upload' "$work/log"; then pass "no release in time: it fails, and uploads nothing"; else fail "no release: $out"; fi

if ! run v9.9.9 >/dev/null; then pass "no file named is refused"; else fail "no files"; fi
if ! run v9.9.9 "$work/missing.json" >/dev/null; then pass "a file that is not there is refused"; else fail "a missing file"; fi

[ "$fails" -eq 0 ] || { echo "$fails failure(s)"; exit 1; }
