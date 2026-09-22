#!/usr/bin/env bash
# Tests for attest.sh, verify-local-check.sh and the pre-push hook, in throw-away repositories
# with throw-away SSH keys and a stand-in for make check:
#   bash .github/scripts/test-attest.sh
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
hook="$here/../../.githooks/pre-push"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
command -v ssh-keygen >/dev/null || { echo "SKIP  ssh-keygen is not installed"; exit 0; }

ssh-keygen -q -t ed25519 -N "" -C t1 -f "$tmp/key1"
ssh-keygen -q -t ed25519 -N "" -C t2 -f "$tmp/key2"
signer() { echo "tester namespaces=\"shint-check\" $(cut -d' ' -f1,2 "$tmp/$1.pub")"; }

# a stand-in for `make check`: the stages, in the order the real one prints them
cat > "$tmp/goodcheck.sh" <<'CHK'
echo "==> gofmt"; echo "==> go vet"; echo "==> go test -race"; echo "ok  	x/shint	1.0s"
echo "==> black-box battery"; echo "battery: 445 cases: 442 passed, 3 known issues, 0 skipped, 0 FAILED"
echo "==> golangci-lint"; echo "0 issues."; echo "==> govulncheck"; echo "No vulnerabilities found"
echo "==> documentation"; echo "==> workflows"; echo "==> all checks passed"
CHK
echo 'echo "==> gofmt"; echo "not formatted"; exit 1' > "$tmp/badcheck.sh"
export SHINT_SIGNING_KEY="$tmp/key1" CHECK_CMD="bash $tmp/goodcheck.sh"

check() { if eval "$2"; then echo "PASS  $1"; else echo "FAIL  $1"; sed 's/^/      /' <<<"${out:-}"; failures=$((failures+1)); fi; }

# origin + a clone whose main has: base (allowed_signers listing key1) and then one more commit
new_repo() {
  rm -rf "$tmp/origin.git" "$tmp/work" "$tmp/ci"
  git init -q --bare -b main "$tmp/origin.git"
  git clone -q "$tmp/origin.git" "$tmp/work" 2>/dev/null; cd "$tmp/work" || exit 1
  git switch -q -c main 2>/dev/null
  mkdir -p .github/scripts; cp "$here/check-report.py" .github/scripts/
  signer key1 > .github/allowed_signers
  printf 'attest:\n\tbash %s\n' "$here/attest.sh" > Makefile
  echo one > f; git add -A; git commit -q -m base; base="$(git rev-parse HEAD)"
  echo two > f; git commit -qam change
  git push -q origin main 2>/dev/null
}
# what CI does: a fresh checkout of the pushed commit, then the verifier
ci() { # extra env assignments as arguments
  rm -rf "$tmp/ci"; git clone -q "$tmp/origin.git" "$tmp/ci"; cd "$tmp/ci" || exit 1
  : > "$tmp/gh_out"
  env GITHUB_OUTPUT="$tmp/gh_out" SHA="$(git rev-parse HEAD)" BEFORE="$base" "$@" bash "$here/verify-local-check.sh" 2>&1
}
fast() { grep -q "^fast=$1$" "$tmp/gh_out"; }

# ---- attest ----
new_repo; out="$(bash "$here/attest.sh" 2>&1)"; rc=$?
tip="$(git rev-parse HEAD)"
check "attest signs a report and attaches it to the commit" '[ $rc = 0 ] && git notes --ref=checks show "$tip" | grep -q "BEGIN SSH SIGNATURE" && grep -q "attached to ${tip:0:12}" <<<"$out"'
check "the note is pushed to origin" 'git --git-dir="$tmp/origin.git" notes --ref=checks show "$tip" >/dev/null 2>&1'
check "the report says what ran" 'git notes --ref=checks show "$tip" | grep -q "\"name\": \"black-box battery\"" && git notes --ref=checks show "$tip" | grep -q "442 passed"'
check "the key is in allowed_signers, so there is no warning" '! grep -q WARNING <<<"$out"'

new_repo; echo dirty >> f; out="$(bash "$here/attest.sh" 2>&1)"; rc=$?
check "a dirty working tree is refused" '[ $rc = 1 ] && grep -q "uncommitted changes" <<<"$out" && ! git notes --ref=checks list 2>/dev/null | grep -q .'
new_repo; out="$(CHECK_CMD="bash $tmp/badcheck.sh" bash "$here/attest.sh" 2>&1)"; rc=$?
check "a failing check signs nothing" '[ $rc = 1 ] && grep -q "the check failed" <<<"$out" && ! git notes --ref=checks list 2>/dev/null | grep -q .'
new_repo; out="$(SHINT_SIGNING_KEY="$tmp/nope" bash "$here/attest.sh" 2>&1)"; rc=$?
check "a missing key is refused before anything runs" '[ $rc = 1 ] && grep -q "does not exist" <<<"$out" && ! grep -q "running the whole check" <<<"$out"'
new_repo; out="$(SHINT_SIGNING_KEY="$tmp/key2" bash "$here/attest.sh" 2>&1)"; rc=$?
check "a key that is not in allowed_signers is warned about, since CI will ignore it" '[ $rc = 0 ] && grep -q "WARNING: CI will not accept this report" <<<"$out"'
new_repo; out="$(NO_PUSH=1 bash "$here/attest.sh" 2>&1)"; rc=$?
check "NO_PUSH keeps the note local" '[ $rc = 0 ] && [ -z "$(git --git-dir="$tmp/origin.git" notes --ref=checks list 2>/dev/null)" ]'

# ---- CI side ----
new_repo; bash "$here/attest.sh" >/dev/null 2>&1; tip="$(git rev-parse HEAD)"
out="$(ci)"; check "a valid signed report for the pushed commit gives fast=true" 'fast true && grep -q "signed by tester" <<<"$out"'
out="$(ci MAX_AGE_DAYS=-1)"; check "a report that is too old gives fast=false" 'fast false && grep -q "older than" <<<"$out"'
out="$(ci BEFORE=)"; check "with no earlier commit the whole suite runs" 'fast false && grep -q "no earlier commit" <<<"$out"'
out="$(ci BEFORE=0000000000000000000000000000000000000000)"; check "an all-zero BEFORE is the same" 'fast false && grep -q "no earlier commit" <<<"$out"'

new_repo; cd "$tmp/work"; echo three > f; git commit -qam "no report for this one"; git push -q origin main 2>/dev/null
out="$(ci BEFORE="$(git rev-parse HEAD~1)")"; check "a commit with no report gives fast=false" 'fast false && grep -q "none for" <<<"$out"'

new_repo; cd "$tmp/work"; git notes --ref=checks add -f -m "not a report" "$(git rev-parse HEAD)"; git push -q origin refs/notes/checks:refs/notes/checks -f 2>/dev/null
out="$(ci)"; check "a note that is not a report is refused" 'fast false'

new_repo; bash "$here/attest.sh" >/dev/null 2>&1; cd "$tmp/work"; tip="$(git rev-parse HEAD)"
git notes --ref=checks show "$tip" | sed 's/"result": "pass"/"result": "pasz"/' > "$tmp/tampered"; git notes --ref=checks add -f -F "$tmp/tampered" "$tip"; git push -q origin refs/notes/checks:refs/notes/checks -f 2>/dev/null
out="$(ci)"; check "a report edited after signing is refused" 'fast false && grep -q "signature does not match" <<<"$out"'

new_repo; bash "$here/attest.sh" >/dev/null 2>&1; cd "$tmp/work"; tip="$(git rev-parse HEAD)"
echo four > f; git commit -qam "another tree"; git notes --ref=checks copy "$tip" HEAD; git push -q origin main refs/notes/checks:refs/notes/checks 2>/dev/null
out="$(ci BEFORE="$tip")"; check "a report copied onto a commit with a different tree is refused" 'fast false && grep -q "the report is for tree" <<<"$out"'

# a push cannot approve itself: same push adds the signer that signs its report
new_repo; cd "$tmp/work"; signer key2 >> .github/allowed_signers; git commit -qam "add a signer"
SHINT_SIGNING_KEY="$tmp/key2" bash "$here/attest.sh" >/dev/null 2>&1; git push -q origin main 2>/dev/null
out="$(ci BEFORE="$(git rev-parse HEAD~1)")"; check "a key added by the same push is not trusted for that push" 'fast false && grep -q "not in the allowed signers" <<<"$out"'

# the file has to exist on the commit before the push
new_repo; cd "$tmp/work"; git rm -q --cached .github/allowed_signers; git commit -qm "drop it"; git add .github/allowed_signers; echo x > g; git add g; git commit -qm "add it back"
bash "$here/attest.sh" >/dev/null 2>&1; git push -q origin main 2>/dev/null
out="$(ci BEFORE="$(git rev-parse HEAD~1)")"; check "allowed_signers that the previous commit did not have is not trusted" 'fast false && grep -q "is not on the commit before" <<<"$out"'

# ---- the pre-push hook ----
zeros=0000000000000000000000000000000000000000
push() { # local sha, remote ref, environment...
  local sha="$1" ref="$2"; shift 2
  (cd "$tmp/work" && echo "refs/heads/local $sha $ref $zeros" | env "$@" bash "$hook" 2>&1)
}
new_repo; bash "$here/attest.sh" >/dev/null 2>&1; cd "$tmp/work"; tip="$(git rev-parse HEAD)"
out="$(push "$tip" refs/heads/main)"; rc=$?
check "the hook lets a commit with a valid report through" '[ $rc = 0 ] && grep -q "has a valid signed check report" <<<"$out"'
new_repo; cd "$tmp/work"; tip="$(git rev-parse HEAD)"
out="$(push "$tip" refs/heads/main)"; rc=$?
check "a push of main with no report makes one" '[ $rc = 0 ] && grep -q "running make attest" <<<"$out" && git notes --ref=checks show "$tip" >/dev/null 2>&1'
new_repo; cd "$tmp/work"; tip="$(git rev-parse HEAD)"
out="$(push "$tip" refs/tags/v1.2.3)"; rc=$?
check "so does a release tag" '[ $rc = 0 ] && grep -q "running make attest" <<<"$out"'
new_repo; cd "$tmp/work"; tip="$(git rev-parse HEAD)"
out="$(push "$tip" refs/heads/feature)"; rc=$?
check "another branch is left alone" '[ $rc = 0 ] && [ -z "$out" ]'
out="$(push "$tip" refs/tags/not-a-release)"; rc=$?
check "and so is a tag that is not a release" '[ $rc = 0 ] && [ -z "$out" ]'
out="$(push "$zeros" refs/heads/main)"; rc=$?
check "a deletion is left alone" '[ $rc = 0 ] && [ -z "$out" ]'
out="$(push "$tip" refs/heads/main SHINT_SKIP_ATTEST=1)"; rc=$?
check "SHINT_SKIP_ATTEST pushes without a report, saying so" '[ $rc = 0 ] && grep -q "without a signed check report" <<<"$out" && ! git notes --ref=checks show "$tip" >/dev/null 2>&1'
new_repo; cd "$tmp/work"; tip="$(git rev-parse HEAD)"; echo dirty >> f
out="$(push "$tip" refs/heads/main)"; rc=$?
check "a dirty tree cannot be attested by the hook: the push is refused" '[ $rc = 1 ] && grep -q "cannot be checked here" <<<"$out"'
new_repo; cd "$tmp/work"; tip="$(git rev-parse HEAD)"
out="$(push "$tip" refs/heads/main CHECK_CMD="bash $tmp/badcheck.sh")"; rc=$?
check "a failing check refuses the push" '[ $rc = 1 ] && grep -q "not pushing" <<<"$out"'

echo
if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
