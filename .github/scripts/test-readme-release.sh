#!/usr/bin/env bash
# Tests for readme-release.sh, in throw-away git repositories with a fake gh:
#   bash .github/scripts/test-readme-release.sh
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/readme-release.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

mkdir -p "$tmp/bin"
# a fake gh: logs what it was asked to do; GH_FAIL_PR=1 makes `pr create` fail; milestones come from a file
cat > "$tmp/bin/gh" <<'GH'
#!/usr/bin/env bash
echo "gh $*" >> "$GH_LOG"
case "$1 $2" in
  "pr create") [ "${GH_FAIL_PR:-0}" = 1 ] && { echo "GitHub Actions is not permitted to create pull requests" >&2; exit 1; }; echo "https://example.test/pull/1";;
  "issue create") echo "https://example.test/issues/9";;
  "api repos"*) cat "$GH_MILESTONES";;
  *) if [ "$1" = api ]; then cat "$GH_MILESTONES"; fi;;
esac
GH
chmod +x "$tmp/bin/gh"
# a fake shint whose --help lists commands
cat > "$tmp/bin/shint" <<'SH'
#!/usr/bin/env bash
cat <<'HELP'
Available Commands:
  ping        Send ICMP ECHO_REQUEST to a host
  wol         Send a Wake-on-LAN magic packet
  help        Help about any command

Flags:
  -h, --help   help
HELP
SH
chmod +x "$tmp/bin/shint"
export GH_LOG="$tmp/gh.log" GH_MILESTONES="$tmp/milestones.json"
echo '[{"title":"v1.3.0","due_on":"2026-12-20T00:00:00Z","state":"open"}]' > "$GH_MILESTONES"
export PATH="$tmp/bin:$PATH" BIN="$tmp/bin/shint" NO_FETCH=1

README_OLD='# shint

**that SHIt Network Tool** - checks.

[![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest)

## Commands

| Command | What it answers |
|---|---|
| `shint ping <host>` | Is it up? |

## Roadmap

| Release | Sprint | What is coming |
|---|---|---|
| **v1.0.0** | Released Jan 1 | First |
| **v1.1.0** | Feb 1 - Feb 14 | Second |
| **v1.3.0** | Dec 7 - Dec 20 | Fourth |
'

# origin (bare) and a clone with main, a CHANGELOG and two tags
new_repo() {
  rm -rf "$tmp/origin.git" "$tmp/work"; : > "$GH_LOG"
  git init -q --bare -b main "$tmp/origin.git"
  git clone -q "$tmp/origin.git" "$tmp/work" 2>/dev/null; cd "$tmp/work" || exit 1
  git switch -q -c main 2>/dev/null
  printf '%s\n' "$README_OLD" > readme.md
  printf '# Changelog\n\n## v1.1.0 - 2026-02-03\n\nThe second release.\n\n- **Two.** More.\n\n## v1.0.0 - 2026-01-01\n\n- **One.** First.\n' > CHANGELOG.md
  git add -A && git commit -q -m "main"
  GIT_COMMITTER_DATE="2026-01-01T12:00:00+0000" git tag -a v1.0.0 -m v1.0.0
  GIT_COMMITTER_DATE="2026-02-03T12:00:00+0000" git tag -a v1.1.0 -m v1.1.0
  git push -q origin main --tags 2>/dev/null
  git fetch -q origin
}

run() { BASE_REF=origin/main bash "$script" 2>&1; }

check() { # name, condition (a shell command), on failure print $out
  if eval "$2"; then echo "PASS  $1"; else echo "FAIL  $1"; sed 's/^/      /' <<<"$out"; failures=$((failures+1)); fi
}

new_repo; out="$(TAG=v1.1.0 run)"; rc=$?
check "proposes the README on readme/main-<tag>, leaving main alone" '[ $rc = 0 ] && [ "$(git branch --show-current)" = readme/main-v1.1.0 ] && [ "$(git rev-parse origin/main)" = "$(git rev-parse main)" ]'
check "the branch starts at main's tip with one commit" '[ "$(git rev-list --count origin/main..HEAD)" = 1 ]'
check "the tag is marked released on its date" 'grep -q "| \*\*v1.1.0\*\* | Released Feb 3 | Second |" readme.md'
check "the expansion, donate button and support line are back" 'grep -q "Simple Host INspection Toolkit" readme.md && grep -q "\[!\[Donate\]" readme.md && grep -q "support its development" readme.md'
check "a command the binary has and the README lacks gets a row" 'grep -q "^| `shint wol` | Send a Wake-on-LAN magic packet |" readme.md && ! grep -q "shint help" readme.md'
check "the milestone moves the planned window" 'grep -q "| \*\*v1.3.0\*\* | Dec 7 - Dec 20 | Fourth |" readme.md'
check "the commit message carries the changes" 'git log -1 --format=%B | grep -q "marked Released Feb 3"'
check "no attribution trailers" '! git log -1 --format=%B | grep -Eiq "co-authored-by|generated with"'
check "nothing was pushed or opened without PUSH=1" '! grep -Eq "pr create|issue create" "$GH_LOG" && ! git ls-remote --exit-code --heads origin readme/main-v1.1.0 >/dev/null 2>&1'

new_repo; out="$(TAG=v1.1.0 run)"; git switch -q main
out="$(TAG=v1.1.0 run)"; rc=$?
check "a branch that already exists here is refused" '[ $rc = 1 ] && grep -q "already exists here" <<<"$out"'

new_repo; out="$(TAG=v1.1.0 PUSH=1 run)"; rc=$?
check "PUSH=1 pushes the branch and opens a pull request" '[ $rc = 0 ] && git ls-remote --exit-code --heads origin readme/main-v1.1.0 >/dev/null && grep -q "^gh pr create --base main --head readme/main-v1.1.0" "$GH_LOG"'
check "the pull request body has the report" 'grep -q "marked Released Feb 3" "$GH_LOG"'
git switch -q main; git branch -q -D readme/main-v1.1.0   # a later run, on a machine that does not have the local branch
out="$(TAG=v1.1.0 PUSH=1 run)"; rc=$?
check "a branch already on origin is not proposed twice" '[ $rc = 0 ] && grep -q "already on origin" <<<"$out"'

new_repo; out="$(TAG=v1.1.0 PUSH=1 GH_FAIL_PR=1 run)"; rc=$?
check "when a pull request cannot be opened, an issue links the branch" '[ $rc = 0 ] && grep -q "^gh issue create" "$GH_LOG" && grep -q "compare/main...readme/main-v1.1.0" "$GH_LOG"'

new_repo; out="$(TAG=v1.0.0 run)"; rc=$?
check "the tag only names the branch: every release is reconciled" '[ $rc = 0 ] && [ "$(git branch --show-current)" = readme/main-v1.0.0 ] && grep -q "| \*\*v1.1.0\*\* | Released Feb 3 | Second |" readme.md'

new_repo; out="$(run)"; rc=$?
check "without TAG the newest tag is used" '[ $rc = 0 ] && [ "$(git branch --show-current)" = readme/main-v1.1.0 ]'

new_repo; git switch -q main; TAG=v1.1.0 BASE_REF=origin/main bash "$script" >/dev/null 2>&1; git switch -q main
# now make main reconciled: merge the proposal, then run for the same tag under another branch name
git merge -q --ff-only readme/main-v1.1.0 && git branch -q -D readme/main-v1.1.0
git push -q origin main 2>/dev/null; git fetch -q origin
out="$(run)"; rc=$?
check "when the README already matches, there is nothing to do and no branch is left" '[ $rc = 0 ] && grep -q "already matches" <<<"$out" && [ "$(git branch --show-current)" = main ] && ! git rev-parse -q --verify refs/heads/readme/main-v1.1.0 >/dev/null'

# the same from a detached checkout of the tag, as a workflow has it: HEAD goes back where it was
git switch -q --detach v1.1.0; before="$(git rev-parse HEAD)"; git branch -q -f main origin/main 2>/dev/null
out="$(run)"; rc=$?
check "from a detached HEAD, with nothing to do, HEAD is left where it was" '[ $rc = 0 ] && grep -q "already matches" <<<"$out" && [ "$(git rev-parse HEAD)" = "$before" ] && [ -z "$(git branch --show-current)" ]'
git switch -q main

# The scripts live on a branch that main does not have (as they did the first time this ran for real):
# switching to main's tree must not take the script away from under itself.
new_repo; git switch -q -c tooling; mkdir -p .github/scripts
cp "$here/readme-release.sh" "$here/readme-reconcile.py" .github/scripts/; git add -A; git commit -q -m "tooling"
out="$(TAG=v1.1.0 BASE_REF=origin/main bash .github/scripts/readme-release.sh 2>&1)"; rc=$?
check "the script needs nothing from the branch it switches away from" '[ $rc = 0 ] && [ ! -e .github/scripts/readme-reconcile.py ] && grep -q "Released Feb 3" readme.md'
git switch -q main

new_repo; echo x > untracked-change.txt; git add untracked-change.txt; out="$(TAG=v1.1.0 run)"; rc=$?
check "a dirty working tree is refused" '[ $rc = 1 ] && grep -q "uncommitted changes" <<<"$out"'
new_repo; out="$(TAG=1.1.0 run)"; rc=$?
check "a TAG that is not vX.Y.Z is refused" '[ $rc = 1 ] && grep -q "must look like v1.2.3" <<<"$out"'
new_repo; out="$(TAG=v9.9.9 run)"; rc=$?
check "a TAG that does not exist is refused" '[ $rc = 1 ] && grep -q "there is no tag v9.9.9" <<<"$out"'

new_repo; printf '# no roadmap\n' > readme.md; git commit -qam "bad readme"; git push -q origin main 2>/dev/null; git fetch -q origin
out="$(TAG=v1.1.0 run)"; rc=$?
check "a README it cannot read is refused, and the branch is cleaned up" '[ $rc = 1 ] && grep -q "could not reconcile" <<<"$out" && [ "$(git branch --show-current)" = main ] && ! git rev-parse -q --verify refs/heads/readme/main-v1.1.0 >/dev/null'

echo
if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
