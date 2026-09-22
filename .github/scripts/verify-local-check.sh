#!/usr/bin/env bash
# CI side of attest.sh: is there a signed check report for the commit that was pushed?
#
# It writes fast=true or fast=false to $GITHUB_OUTPUT (stdout when that is unset) and never
# fails: when there is no valid report the answer is simply "run the whole suite".
#
# A report is valid when the git note on the pushed commit (refs/notes/checks)
#   * is signed by a key listed in .github/allowed_signers - the copy on the commit BEFORE this
#     push, so a push cannot approve itself by adding its own key in the same breath (a push
#     that introduces the file, or has no earlier commit, gets the whole suite),
#   * is for the tree that was checked out, lists every stage of make check as ok, and is recent.
# check-report.py does the checking; this only fetches what it needs.
#
# Environment: SHA (the pushed commit), BEFORE (main's tip before the push; all zeros or empty
# when there is none), optional MAX_AGE_DAYS (default 14), GITHUB_OUTPUT, GITHUB_STEP_SUMMARY.
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
sha="${SHA:?SHA (the pushed commit) is required}"
before="${BEFORE:-}"
out="${GITHUB_OUTPUT:-/dev/stdout}"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
tree="$(git rev-parse "$sha^{tree}")"

say() { # true|false, why
  echo "fast=$1" >> "$out"
  echo "local check report: $2"
  [ -z "${GITHUB_STEP_SUMMARY:-}" ] || echo "**Local check report:** $2" >> "$GITHUB_STEP_SUMMARY"
}

git fetch -q --no-tags --depth=1 origin '+refs/notes/checks:refs/notes/checks' 2>/dev/null
if ! git notes --ref=checks show "$sha" > "$tmp/note" 2>/dev/null; then
  say false "none for ${sha:0:12}, so the whole suite runs. (make attest signs and attaches one after a local make check.)"; exit 0
fi

if [ -z "$before" ] || [[ "$before" =~ ^0+$ ]]; then
  say false "there is no earlier commit to take the allowed signers from, so the whole suite runs."; exit 0
fi
if ! { git cat-file -e "$before^{commit}" 2>/dev/null || git fetch -q --no-tags --depth=1 origin "$before" 2>/dev/null; } \
   || ! git show "$before:.github/allowed_signers" > "$tmp/allowed_signers" 2>/dev/null; then
  say false ".github/allowed_signers is not on the commit before this push (${before:0:12}), so it cannot be trusted yet and the whole suite runs."; exit 0
fi

if result="$(python3 "$here/check-report.py" verify --note "$tmp/note" --allowed-signers "$tmp/allowed_signers" --tree "$tree" --max-age-days "${MAX_AGE_DAYS:-14}" 2>&1)"; then
  say true "${result#check-report: valid: }; running the short subset and the vulnerability check."
else
  say false "${result#check-report: } The whole suite runs."
fi
