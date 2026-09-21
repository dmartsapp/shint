#!/usr/bin/env bash
# Tests for post-check-status.sh and verify-local-check.sh, in throw-away repositories with a fake gh:
#   bash .github/scripts/test-local-check.sh
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

mkdir -p "$tmp/bin"
# fake gh: `api -X POST .../statuses/<sha>` is logged; `api .../statuses` answers from $GH_STATUS (a line: state|creator|description)
cat > "$tmp/bin/gh" <<'GH'
#!/usr/bin/env bash
echo "gh $*" >> "$GH_LOG"
[ "${GH_FAIL:-0}" = 1 ] && exit 1
case "$*" in
  *"-X POST"*) exit 0;;
  *statuses*) [ -n "${GH_STATUS:-}" ] && echo "$GH_STATUS"; exit 0;;
esac
GH
chmod +x "$tmp/bin/gh"
export GH_LOG="$tmp/gh.log" PATH="$tmp/bin:$PATH" NO_FETCH=1

new_repo() {
  rm -rf "$tmp/origin.git" "$tmp/work"; : > "$GH_LOG"
  git init -q --bare -b main "$tmp/origin.git"
  git clone -q "$tmp/origin.git" "$tmp/work" 2>/dev/null; cd "$tmp/work" || exit 1
  git switch -q -c main 2>/dev/null
  echo one > f; git add -A; git commit -q -m one
  git push -q origin main 2>/dev/null; git fetch -q origin
}
check() { if eval "$2"; then echo "PASS  $1"; else echo "FAIL  $1"; sed 's/^/      /' <<<"$out"; failures=$((failures+1)); fi; }

# ---- posting
new_repo; out="$(REPO=o/r bash "$here/post-check-status.sh" 2>&1)"; rc=$?
sha="$(git rev-parse HEAD)"; tree="$(git rev-parse 'HEAD^{tree}' | cut -c1-12)"
check "posts a success status for the commit, with the tree in the description" '[ $rc = 0 ] && grep -q "^gh api -X POST repos/o/r/statuses/$sha -f state=success -f context=local/make-check -f description=make check passed locally (.*); tree $tree" "$GH_LOG"'
new_repo; echo dirty > f; out="$(REPO=o/r bash "$here/post-check-status.sh" 2>&1)"; rc=$?
check "refuses a working tree with uncommitted changes" '[ $rc = 1 ] && grep -q "uncommitted changes" <<<"$out" && [ ! -s "$GH_LOG" ]'
new_repo; echo two > f; git commit -qam two; out="$(REPO=o/r bash "$here/post-check-status.sh" 2>&1)"; rc=$?
check "refuses a commit that is not on origin" '[ $rc = 1 ] && grep -q "not on origin yet" <<<"$out" && [ ! -s "$GH_LOG" ]'
new_repo; out="$(GH_FAIL=1 REPO=o/r bash "$here/post-check-status.sh" 2>&1)"; rc=$?
check "says so when the status cannot be posted" '[ $rc = 1 ] && grep -q "could not post the status" <<<"$out"'

# ---- verifying
verify() { GITHUB_OUTPUT="$tmp/out.txt" REPO=o/r SHA="$(git rev-parse HEAD)" ACTOR="$1" bash "$here/verify-local-check.sh" 2>&1; }
new_repo; tree="$(git rev-parse 'HEAD^{tree}' | cut -c1-12)"
good="success|alice|make check passed locally (darwin/arm64, go1.25.0); tree $tree"

: > "$tmp/out.txt"; out="$(GH_STATUS="$good" verify alice)"
check "a success from the pusher for this tree gives fast=true" 'grep -q "^fast=true" "$tmp/out.txt" && grep -q "short subset" <<<"$out"'
: > "$tmp/out.txt"; out="$(GH_STATUS="" verify alice)"
check "no receipt gives fast=false and the whole suite" 'grep -q "^fast=false" "$tmp/out.txt" && grep -q "none for" <<<"$out"'
: > "$tmp/out.txt"; out="$(GH_STATUS="failure|alice|make check failed; tree $tree" verify alice)"
check "a status that is not a success gives fast=false" 'grep -q "^fast=false" "$tmp/out.txt" && grep -q "'"'"'failure'"'"'" <<<"$out"'
: > "$tmp/out.txt"; out="$(GH_STATUS="$good" verify bob)"
check "a status recorded by someone other than the pusher does not count" 'grep -q "^fast=false" "$tmp/out.txt" && grep -q "recorded by alice, not by bob" <<<"$out"'
: > "$tmp/out.txt"; out="$(GH_STATUS="success|alice|make check passed locally (darwin/arm64, go1.25.0); tree 000000000000" verify alice)"
check "a status for another tree does not count" 'grep -q "^fast=false" "$tmp/out.txt" && grep -q "another tree" <<<"$out"'
: > "$tmp/out.txt"; out="$(GH_FAIL=1 verify alice)"
check "an API error gives fast=false and never fails the job" '[ -n "$(cat "$tmp/out.txt")" ] && grep -q "^fast=false" "$tmp/out.txt" && grep -q "could not be read" <<<"$out"'
: > "$tmp/out.txt"; out="$(GH_STATUS="$good" GITHUB_STEP_SUMMARY="$tmp/summary.md" verify alice)"
check "the answer goes to the job summary too" 'grep -q "Local check receipt" "$tmp/summary.md"'

# the two scripts must agree on the context name
check "both scripts use the same status context" '[ "$(grep -h "^CONTEXT=" "$here/post-check-status.sh" "$here/verify-local-check.sh" | sort -u | wc -l | tr -d " ")" = 1 ]'

echo
if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
