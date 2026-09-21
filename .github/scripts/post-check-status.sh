#!/usr/bin/env bash
# Record that `make check` passed on this machine, as a commit status on the pushed commit:
#
#   make attest            (runs make check, then this)
#   bash .github/scripts/post-check-status.sh
#
# The Check workflow, on a push to main, looks for exactly this status (context
# local/make-check, from the person who pushed, for the tree that was pushed). If it
# finds it, CI runs only a short subset instead of the whole suite; if not, it runs
# the whole suite. So a missing or stale status costs time, never coverage.
#
# The status is attached to a commit GitHub has, so the commit must be pushed (a branch
# is enough) - and because a fast-forward merge keeps commit ids, a status on the branch
# tip is the status on main's tip after the merge.
#
# Environment (optional): REPO (owner/name; default: from the origin URL), NO_FETCH=1.
set -uo pipefail

CONTEXT="local/make-check"
die() { echo "post-check-status: $1" >&2; exit 1; }

[ -z "$(git status --porcelain)" ] || die "the working tree has uncommitted changes: the check would not be of the commit"
sha="$(git rev-parse HEAD)"
tree="$(git rev-parse 'HEAD^{tree}' | cut -c1-12)"
if [ "${NO_FETCH:-0}" != 1 ] && git remote get-url origin >/dev/null 2>&1; then git fetch -q origin || die "could not fetch origin"; fi
[ -n "$(git branch -r --contains "$sha" 2>/dev/null)" ] || die "this commit is not on origin yet: push the branch first (a status attaches to a commit GitHub has)"

repo="${REPO:-$(git remote get-url origin | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')}"
where="$(go env GOOS 2>/dev/null)/$(go env GOARCH 2>/dev/null)"
goversion="$(go version 2>/dev/null | awk '{print $3}')"
description="make check passed locally ($where, $goversion); tree $tree"
description="${description:0:140}"

gh api -X POST "repos/$repo/statuses/$sha" -f state=success -f context="$CONTEXT" -f description="$description" >/dev/null \
  || die "could not post the status (is gh signed in, with access to $repo?)"
echo "post-check-status: recorded on ${sha:0:12}: $CONTEXT = success ($description)"
