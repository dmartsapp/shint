#!/usr/bin/env bash
# Run the whole check on this machine, sign a report of it, and attach the report to the
# commit as a git note (refs/notes/checks), so CI on main can run a short subset instead of
# repeating everything. See check-report.py for what the report is and what a signature proves.
#
#   make attest
#   bash .github/scripts/attest.sh
#
# Steps: the working tree must be clean; run `make check` (its output is shown and kept);
# build the report from that output; sign it with your SSH key; add it as a note on HEAD;
# check that the key is one .github/allowed_signers lists (CI ignores a report from any other);
# push the notes ref. If any step fails nothing is attached.
#
# The key: $SHINT_SIGNING_KEY, else git's user.signingkey, else ~/.ssh/id_ed25519. It is used
# by ssh-keygen, so a passphrase is asked for on the terminal or taken from ssh-agent. Using
# a key of its own for this (not your login key) is a good idea.
#
# Environment (all optional): SHINT_SIGNING_KEY, CHECK_CMD (default "make check"),
# NO_PUSH=1 (keep the note local), REMOTE (default origin).
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
remote="${REMOTE:-origin}"
check_cmd="${CHECK_CMD:-make check}"
die() { echo "attest: $1" >&2; exit 1; }

[ -z "$(git status --porcelain)" ] || die "the working tree has uncommitted changes: the report would not be of the commit"
commit="$(git rev-parse HEAD)"
key="${SHINT_SIGNING_KEY:-$(git config --get user.signingkey || true)}"
key="${key:-$HOME/.ssh/id_ed25519}"
case "$key" in "~/"*) key="$HOME/${key#\~/}";; esac
[ -f "$key" ] || die "the signing key $key does not exist (set SHINT_SIGNING_KEY, or git config user.signingkey to a key file)"

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
echo "attest: running the whole check ($check_cmd) on ${commit:0:12} ..."
bash -c "$check_cmd" 2>&1 | tee "$tmp/check.log"
[ "${PIPESTATUS[0]}" = 0 ] || die "the check failed, so there is nothing to sign"

python3 "$here/check-report.py" build --log "$tmp/check.log" --out "$tmp/report.json" || die "could not build the report"
python3 "$here/check-report.py" sign --report "$tmp/report.json" --key "$key" --out "$tmp/note.txt" || die "could not sign the report"

if [ -f .github/allowed_signers ]; then
  python3 "$here/check-report.py" verify --note "$tmp/note.txt" --allowed-signers .github/allowed_signers --tree "$(git rev-parse 'HEAD^{tree}')" >/dev/null 2>"$tmp/verify.err" \
    || echo "attest: WARNING: CI will not accept this report ($(head -n1 "$tmp/verify.err" | sed 's/^check-report: //')). Add your public key to .github/allowed_signers." >&2
else
  echo "attest: WARNING: there is no .github/allowed_signers, so CI cannot accept any report yet." >&2
fi

git notes --ref=checks add -f -F "$tmp/note.txt" "$commit" || die "could not add the note"
echo "attest: the signed report is attached to ${commit:0:12} (git notes --ref=checks show ${commit:0:12})"

if [ "${NO_PUSH:-0}" = 1 ]; then echo "attest: NO_PUSH=1: the note stays local"; exit 0; fi
git remote get-url "$remote" >/dev/null 2>&1 || { echo "attest: no remote '$remote': the note stays local"; exit 0; }
git push -q "$remote" refs/notes/checks:refs/notes/checks 2>"$tmp/push.err" \
  || die "could not push the notes ref ($(head -c 200 "$tmp/push.err" | tr '\n' ' ')); fetch it first: git fetch $remote refs/notes/checks:refs/notes/checks"
echo "attest: pushed refs/notes/checks to $remote"
