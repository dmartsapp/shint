#!/usr/bin/env bash
# Tests for write-ci-failure-issue.sh:  bash .github/scripts/test-write-ci-failure-issue.sh
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/write-ci-failure-issue.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0

check() { # name, condition-exit-status
  if [ "$2" = 0 ]; then echo "PASS  $1"; else echo "FAIL  $1"; failures=$((failures+1)); fi
}

WORKFLOW="Lint" RUN_URL="https://github.com/o/r/actions/runs/42" REF="v4.1.0" SHA="abc123" \
  NOTE="Nothing was published." bash "$script" "$tmp/body.md"
check "writes the file"                       "$([ -s "$tmp/body.md" ]; echo $?)"
check "has the real run URL"                  "$(grep -q 'https://github.com/o/r/actions/runs/42' "$tmp/body.md"; echo $?)"
check "names the workflow, ref and commit"    "$(grep -q 'Lint.*v4.1.0.*abc123' "$tmp/body.md"; echo $?)"
check "carries the note"                      "$(grep -q 'Nothing was published.' "$tmp/body.md"; echo $?)"
check "contains no unexpanded expression"     "$(grep -q '\${{' "$tmp/body.md"; [ $? -ne 0 ]; echo $?)"

WORKFLOW="Check" RUN_URL="u" REF="main" SHA="s" bash "$script" "$tmp/plain.md"
check "the note is optional"                  "$(grep -q 'CI failure: Check' "$tmp/plain.md"; echo $?)"

# values with shell metacharacters stay inert data
WORKFLOW='Lint' RUN_URL='u' REF='$(touch '"$tmp"'/pwned)' SHA='`touch '"$tmp"'/pwned2`' bash "$script" "$tmp/inert.md"
check "metacharacters are not executed"       "$([ ! -e "$tmp/pwned" ] && [ ! -e "$tmp/pwned2" ]; echo $?)"

( unset RUN_URL; WORKFLOW=x REF=y SHA=z bash "$script" "$tmp/x.md" >/dev/null 2>&1 ); rc=$?
check "a missing value is an error"           "$([ $rc -ne 0 ]; echo $?)"
( WORKFLOW=x RUN_URL=u REF=y SHA=z bash "$script" >/dev/null 2>&1 ); rc=$?
check "a missing output path is an error"     "$([ $rc -ne 0 ]; echo $?)"

echo
if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
