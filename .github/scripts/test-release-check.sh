#!/usr/bin/env bash
# Tests for release-check.sh, in throw-away git repositories:
#   bash .github/scripts/test-release-check.sh
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/release-check.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

# A repository with main, and release/v1.2.3 on top of it in a good state.
new_repo() {
  local dir="$tmp/$1"
  rm -rf "$dir"; mkdir -p "$dir"; cd "$dir" || exit 1
  git init -q -b main
  printf 'package main\nvar (\n\tVersion string = "1.2.2"\n)\n' > main.go
  printf '# shint\nmain readme\n' > readme.md
  printf '# Changelog\n\n## v1.2.2 - 2026-01-01\n\n- old\n' > CHANGELOG.md
  git add -A && git commit -q -m "main"
  git switch -q -c release/v1.2.3
  printf '# v1.2.3 branch readme\n' > readme.md          # the branch page ...
  git add -A && git commit -q -m "branch readme"
  sed -i.bak 's/1\.2\.2"/1.2.3"/' main.go && rm -f main.go.bak
  printf '# Changelog\n\n## v1.2.3 - 2026-02-02\n\n- new\n\n## v1.2.2 - 2026-01-01\n\n- old\n' > CHANGELOG.md
  git checkout -q main -- readme.md                      # ... restored to main's for the release
  git add -A && git commit -q -m "fix: the release (v1.2.3)

Full changelog for v1.2.3
=========================

- new"
}

run() { NO_FETCH=1 BASE_REF=main bash "$script" 2>&1; }

expect() { # name, expected exit (0/1), text that must appear
  local name="$1" want="$2" needle="$3" out rc
  out="$(run)"; rc=$?
  if [ "$rc" = "$want" ] && grep -q -- "$needle" <<<"$out"; then echo "PASS  $name"; else
    echo "FAIL  $name (exit $rc, want $want; looking for: $needle)"; sed 's/^/      /' <<<"$out"; failures=$((failures+1)); fi
}

new_repo good;               expect "a good release passes"                          0 "release-check: all passed"
new_repo readme
  printf 'branch text\n' > readme.md; git commit -qam "readme"
                             expect "a README that differs from main's is refused"   1 "FAIL  readme.md differs"
new_repo notrebased
  git switch -q main; echo x > other.txt; git add -A; git commit -qm "main moved"; git switch -q release/v1.2.3
                             expect "a branch behind main is refused"                1 "FAIL  the branch is not on top"
new_repo version
  sed -i.bak 's/1\.2\.3"/1.2.4"/' main.go; rm -f main.go.bak; git commit -qam "v"
                             expect "a Version that does not match is refused"       1 "FAIL  main.go Version is '1.2.4'"
new_repo unreleased
  sed -i.bak 's/^## v1.2.3 - 2026-02-02/## v1.2.3 - unreleased/' CHANGELOG.md; rm -f CHANGELOG.md.bak; git commit -qam "c"
                             expect "an unreleased changelog is refused"             1 "FAIL  CHANGELOG's newest section"
new_repo nomsg
  git commit -q --amend -m "fix: the release, without the changelog in the message"
                             expect "a release commit without the changelog is refused" 1 "FAIL  the tip commit's message lacks"
new_repo dirty
  echo change >> CHANGELOG.md
                             expect "an uncommitted change is refused"               1 "FAIL  working tree has uncommitted"
new_repo tag
  git tag v1.2.3
                             expect "an existing tag is refused"                     1 "FAIL  the tag v1.2.3 already exists"
new_repo trailer
  git commit -q --allow-empty -m "fix: the release (v1.2.3)

Full changelog for v1.2.3

Co-Authored-By: Someone <someone@example.com>"
                             expect "an attribution trailer is refused"              1 "FAIL  a commit on the branch has"
new_repo wrongbranch
  git switch -q -c feature/x
                             expect "a branch that is not release/vX.Y.Z is refused" 1 "FAIL  the current branch is 'feature/x'"

echo
if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
