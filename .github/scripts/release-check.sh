#!/usr/bin/env bash
# Preflight for release day, run on the release branch after the release commit:
#
#   make release-check          (or: bash .github/scripts/release-check.sh)
#
# It checks what a fast-forward merge into main and a tag would otherwise carry
# unnoticed. Exit status 0 means every check passed; 1 means at least one failed.
#
#   * the branch is release/vX.Y.Z and main.go's Version is X.Y.Z
#   * the working tree is clean and the branch sits on top of main (so the merge
#     is a fast-forward, which the repository ruleset requires)
#   * readme.md is identical to main's: a release branch's README is a working
#     page for that branch and must not reach main (see docs/src/tech-release.md)
#   * CHANGELOG.md's newest section is X.Y.Z and dated, not "unreleased"
#   * the tip commit is the release commit (its message carries the full changelog)
#   * no commit on the branch carries a Co-Authored-By trailer or a
#     "Generated with" line, and the tag vX.Y.Z does not exist yet
#
# Environment: BASE_REF (default origin/main), NO_FETCH=1 to skip `git fetch`.
set -uo pipefail

base="${BASE_REF:-origin/main}"
failures=0
pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1"; failures=$((failures + 1)); }

if [ "${NO_FETCH:-0}" != 1 ] && git remote get-url origin >/dev/null 2>&1; then
  git fetch -q origin || fail "could not fetch origin"
fi
git rev-parse -q --verify "$base" >/dev/null || { echo "FAIL  base ref $base does not exist"; exit 1; }

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" =~ ^release/v([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
  version="${BASH_REMATCH[1]}"
  pass "on release branch $branch"
else
  fail "the current branch is '$branch', not release/vX.Y.Z"
  echo; echo "release-check: $failures failed"; exit 1
fi

code_version="$(sed -n 's/^[[:space:]]*Version string = "\([0-9.]*\)".*/\1/p' main.go | head -n1)"
if [ "$code_version" = "$version" ]; then pass "main.go Version is $version"; else fail "main.go Version is '$code_version', the branch is for $version"; fi

if [ -z "$(git status --porcelain)" ]; then pass "working tree is clean"; else fail "working tree has uncommitted changes"; fi

if git merge-base --is-ancestor "$base" HEAD; then pass "the branch is on top of $base (a fast-forward merge is possible)"; else fail "the branch is not on top of $base - rebase it: git rebase $base"; fi

if git diff --quiet "$base" -- readme.md; then pass "readme.md is identical to $base's"; else fail "readme.md differs from $base's - restore it: git checkout $base -- readme.md"; fi

heading="$(grep -m1 '^## v' CHANGELOG.md || true)"
if [[ "$heading" =~ ^##\ v${version//./\\.}\ -\ [0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
  pass "CHANGELOG's newest section is v$version, dated"
else
  fail "CHANGELOG's newest section is '${heading:-<none>}', expected '## v$version - YYYY-MM-DD'"
fi

if git log -1 --format=%B | grep -q "Full changelog for v${version}"; then pass "the tip commit carries the full changelog"; else fail "the tip commit's message lacks 'Full changelog for v$version' - it becomes the release notes"; fi

if git log "$base..HEAD" --format=%B | grep -Eiq '^co-authored-by:|generated with|noreply@anthropic\.com'; then fail "a commit on the branch has a Co-Authored-By trailer or a 'Generated with' line"; else pass "no attribution trailers in the branch's commits"; fi

if git rev-parse -q --verify "refs/tags/v$version" >/dev/null; then
  fail "the tag v$version already exists"
elif [ "${NO_FETCH:-0}" != 1 ] && [ -n "$(git ls-remote --tags origin "refs/tags/v$version" 2>/dev/null)" ]; then
  fail "the tag v$version already exists on origin"
else
  pass "the tag v$version does not exist yet"
fi

echo
if [ "$failures" = 0 ]; then echo "release-check: all passed - safe to fast-forward main and tag v$version"; else echo "release-check: $failures FAILED"; exit 1; fi
