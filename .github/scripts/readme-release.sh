#!/usr/bin/env bash
# Propose main's README for the releases that have shipped: a branch off main with
# the README reconciled (see readme-reconcile.py), never a commit on main itself.
#
#   make readme-reconcile [TAG=v4.2.0] [PUSH=1]
#   TAG=v4.2.0 PUSH=1 bash .github/scripts/readme-release.sh
#
# What it does:
#   1. branches readme/main-<tag> off origin/main (TAG defaults to the newest vX.Y.Z tag; it
#      only names the branch - every release the README does not yet show is reconciled, so
#      two releases in quick succession give two proposals of which the newer contains the older),
#   2. runs readme-reconcile.py there: release tags and dates, the changelog's summaries,
#      the GitHub milestones (when it can ask), the binary's --help, and what the README
#      must always have (expansion, badges, donate button, support line),
#   3. commits the result with the changes and warnings as the message, if there are any,
#   4. with PUSH=1 pushes the branch and opens a pull request into main - or, when GitHub
#      Actions may not open pull requests (a repository setting), an issue that links the branch.
# Nothing is merged; a person reads the diff and the warnings and merges.
#
# Environment (all optional): TAG, PUSH=1, BASE_REF (default origin/main), NO_FETCH=1,
#   BIN (a shint binary whose --help lists the commands; default: built with go, if there is go),
#   MILESTONES_JSON (a file; default: asked of GitHub through gh, if gh is usable),
#   REPO (owner/name; default: from the origin URL).
# Exit status: 0 done or nothing to do; 1 refused (dirty tree, bad tag, no such tag ...).
set -uo pipefail

base="${BASE_REF:-origin/main}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
die() { echo "readme-release: $1" >&2; exit 1; }

[ -z "$(git status --porcelain)" ] || die "the working tree has uncommitted changes; commit or stash them first"
# The script is used from a copy: the branch below is main's tree, which may not have this
# version of it (or, on a first run, any).
cp "$here/readme-reconcile.py" "$tmp/readme-reconcile.py"
if [ "${NO_FETCH:-0}" != 1 ] && git remote get-url origin >/dev/null 2>&1; then
  git fetch -q origin --tags || die "could not fetch origin"
fi
git rev-parse -q --verify "$base" >/dev/null || die "the base $base does not exist"

tag="${TAG:-}"
if [ -z "$tag" ]; then
  tag="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)"
  [ -n "$tag" ] || die "there is no vX.Y.Z tag to reconcile the README with"
fi
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "TAG must look like v1.2.3, not '$tag'"
git rev-parse -q --verify "refs/tags/$tag" >/dev/null || die "there is no tag $tag"

branch="readme/main-$tag"
if git rev-parse -q --verify "refs/heads/$branch" >/dev/null; then die "the branch $branch already exists here; look at it, or delete it to start again"; fi
if git ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
  echo "readme-release: $branch is already on origin - the README for $tag has been proposed. Nothing to do."
  exit 0
fi

# where to go back to: the branch we are on, or (a detached checkout, as in CI) the commit
start_branch="$(git symbolic-ref -q --short HEAD || true)"
start_sha="$(git rev-parse HEAD)"
restore() { if [ -n "$start_branch" ]; then git switch -q "$start_branch"; else git switch -q --detach "$start_sha"; fi; }
git switch -q -c "$branch" "$base" || die "could not create $branch from $base"

# the binary's own list of commands, for the Commands table
help_args=()
bin="${BIN:-}"
if [ -z "$bin" ] && command -v go >/dev/null 2>&1 && [ -f go.mod ]; then
  bin="$tmp/shint"; CGO_ENABLED=0 go build -o "$bin" . 2>/dev/null || bin=""
fi
if [ -n "$bin" ] && "$bin" --help > "$tmp/help.txt" 2>/dev/null; then help_args=(--help-file "$tmp/help.txt"); fi

# the milestones, for the sprint windows
ms_args=()
if [ -n "${MILESTONES_JSON:-}" ]; then
  ms_args=(--milestones "$MILESTONES_JSON")
elif command -v gh >/dev/null 2>&1; then
  repo="${REPO:-$(git remote get-url origin 2>/dev/null | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')}"
  if [ -n "$repo" ] && gh api "repos/$repo/milestones?state=all&per_page=100" > "$tmp/milestones.json" 2>/dev/null; then
    ms_args=(--milestones "$tmp/milestones.json")
  fi
fi

# ${arr[@]+"${arr[@]}"}: an empty array is not an unbound variable, even in bash 3.2 (macOS)
python3 "$tmp/readme-reconcile.py" ${help_args[@]+"${help_args[@]}"} ${ms_args[@]+"${ms_args[@]}"} --report "$tmp/report.md" > /dev/null
rc=$?
if [ "$rc" != 0 ]; then
  restore; git branch -q -D "$branch"
  die "readme-reconcile.py could not reconcile the README (exit $rc); run it by hand to see why"
fi

if [ -z "$(git status --porcelain)" ]; then
  restore; git branch -q -D "$branch"
  echo "readme-release: the README already matches $tag. Nothing to do."
  [ -s "$tmp/report.md" ] && grep -q "Needs a human look" "$tmp/report.md" && { echo; cat "$tmp/report.md"; }
  exit 0
fi

subject="docs: README - reconciled with the releases up to $tag"
git add readme.md
{ echo "$subject"; echo; cat "$tmp/report.md"; } > "$tmp/message.txt"
git commit -q -F "$tmp/message.txt" || die "could not commit"
echo "readme-release: committed the README on $branch:"
git log -1 --format='  %h %s'
echo; cat "$tmp/report.md"

if [ "${PUSH:-0}" = 1 ]; then
  git push -q origin "$branch" || die "could not push $branch"
  title="README: reconcile with the releases up to $tag"
  body="Written by .github/scripts/readme-release.sh after $tag was released. Read the diff, look at what needs a human look, then merge.

$(cat "$tmp/report.md")"
  if gh pr create --base main --head "$branch" --title "$title" --body "$body" 2>"$tmp/pr.err"; then
    echo "readme-release: opened a pull request from $branch."
  else
    echo "readme-release: could not open a pull request ($(head -c 200 "$tmp/pr.err" | tr '\n' ' ')); opening an issue that links the branch instead."
    repo="${REPO:-$(git remote get-url origin | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')}"
    gh issue create --title "$title" --body "The README for $tag is ready on the branch \`$branch\`: https://github.com/$repo/compare/main...$branch

Merge it with a fast-forward, or open a pull request from it. (Actions cannot open pull requests unless the repository setting \"Allow GitHub Actions to create and approve pull requests\" is on.)

$(cat "$tmp/report.md")" || echo "readme-release: could not open an issue either; the branch $branch is pushed." >&2
  fi
else
  echo
  echo "Next: read the diff (git diff $base), fix the wording the warnings point at, then either"
  echo "  git push -u origin $branch      and open a pull request, or merge it fast-forward into main."
fi
