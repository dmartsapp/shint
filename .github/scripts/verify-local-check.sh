#!/usr/bin/env bash
# CI side of post-check-status.sh: is there a receipt for the commit that was pushed?
#
# It writes fast=true or fast=false to $GITHUB_OUTPUT (stdout when that is unset) and never
# fails: when there is no valid receipt the answer is simply "run the whole suite".
#
# A receipt is valid when the newest commit status with context local/make-check
#   * is "success",
#   * was created by the person who pushed (ACTOR) - a status from anyone else does not count, and
#   * names the tree that was checked out ("tree <first 12 characters of HEAD^{tree}>").
#
# Environment: REPO, SHA, ACTOR, GH_TOKEN (all required).
set -uo pipefail

CONTEXT="local/make-check"
repo="${REPO:?REPO (owner/name) is required}"
sha="${SHA:?SHA (the pushed commit) is required}"
actor="${ACTOR:?ACTOR (who pushed) is required}"
out="${GITHUB_OUTPUT:-/dev/stdout}"
tree="$(git rev-parse 'HEAD^{tree}' | cut -c1-12)"

say() { # fast|slow, why
  echo "fast=$1" >> "$out"
  echo "local check receipt: $2"
  [ -z "${GITHUB_STEP_SUMMARY:-}" ] || echo "**Local check receipt:** $2" >> "$GITHUB_STEP_SUMMARY"
}

# newest first; the fields are separated by a character no description or login contains
latest="$(gh api "repos/$repo/commits/$sha/statuses?per_page=100" \
  --jq "[.[] | select(.context==\"$CONTEXT\")][0] | select(. != null) | \"\(.state)|\(.creator.login)|\(.description)\"" 2>/dev/null)"
rc=$?
if [ "$rc" != 0 ]; then say false "the commit statuses could not be read, so the whole suite runs."; exit 0; fi
if [ -z "$latest" ]; then say false "none for ${sha:0:12}, so the whole suite runs. (make attest records one after a local make check, on a pushed commit.)"; exit 0; fi

state="${latest%%|*}"; rest="${latest#*|}"; creator="${rest%%|*}"; description="${rest#*|}"
if [ "$state" != success ]; then say false "the latest one is '$state', so the whole suite runs."; exit 0; fi
if [ "$creator" != "$actor" ]; then say false "recorded by $creator, not by $actor who pushed, so the whole suite runs."; exit 0; fi
case "$description" in *"tree $tree"*) ;; *) say false "it is for another tree ($description; this one is $tree), so the whole suite runs."; exit 0;; esac

say true "make check passed locally on this tree ($description); running the short subset and the vulnerability check."
