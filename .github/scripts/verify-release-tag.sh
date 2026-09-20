#!/usr/bin/env bash
# Gate for the release pipeline: a pushed tag may start it only if the tag is
# strictly vMAJOR.MINOR.PATCH and the commit it points at is on main.
#
# Inputs (environment): TAG  the pushed tag name       (github.ref_name)
#                       SHA  the commit the tag is on  (github.sha)
#                       REPO owner/name                (github.repository)
#                       GH_TOKEN for `gh`; BASE_BRANCH defaults to main.
#
# The trigger filter in each workflow already limits which tags start a run;
# this repeats the format check independently and adds what a filter cannot
# express - "the tagged commit is on main". It guards against mistakes (a tag
# put on a release branch by accident), not against someone who can push both
# a tag and workflow changes; who may create tags is a repository-settings
# question. It fails closed: if it cannot tell, it refuses.
set -euo pipefail

tag="${TAG:?TAG (the pushed tag name) is required}"
sha="${SHA:?SHA (the commit the tag points at) is required}"
repo="${REPO:?REPO (owner/name) is required}"
base="${BASE_BRANCH:-main}"

fail() {
  echo "::error title=Release tag rejected::$1"
  exit 1
}

if ! [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  fail "tag '$tag' is not of the form vX.Y.Z (digits only). Nothing was built or published."
fi

# `compare main...<sha>`: "identical" - the tag is on main's tip; "behind" - the
# commit is an older commit of main (an ancestor). "ahead" or "diverged" - it is
# not on main.
if ! status="$(gh api "repos/${repo}/compare/${base}...${sha}" --jq .status)"; then
  fail "could not compare ${sha} with ${base}; refusing to release without knowing the commit is on ${base}."
fi

case "$status" in
  identical | behind)
    echo "Tag ${tag} (${sha}) is on ${base} (${status}): the release may proceed."
    ;;
  *)
    fail "commit ${sha} (tag ${tag}) is not on ${base} (compare says: ${status}). Merge the release branch into ${base} first, then tag."
    ;;
esac
