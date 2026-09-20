#!/usr/bin/env bash
# Writes the body of the issue a failed CI job files, with the real run URL.
#
#   WORKFLOW=Lint RUN_URL=... REF=v4.1.0 SHA=... [NOTE=...] \
#     bash .github/scripts/write-ci-failure-issue.sh <output-file>
#
# The values arrive as environment variables (set from ${{ }} expressions in
# the workflow, which GitHub expands there) so nothing is interpolated into a
# shell script. An issue *template* file cannot do this: GitHub expands
# expressions only inside workflow files, which is why the issues filed before
# v4.0.4 showed the raw text `${{ github.server_url }}/...`.
set -euo pipefail

out="${1:?usage: write-ci-failure-issue.sh <output-file>}"
workflow="${WORKFLOW:?WORKFLOW (the workflow name) is required}"
run_url="${RUN_URL:?RUN_URL is required}"
ref="${REF:?REF (branch or tag name) is required}"
sha="${SHA:?SHA is required}"
note="${NOTE:-}"

{
  echo "### CI failure: ${workflow}"
  echo
  echo "The **${workflow}** workflow failed on \`${ref}\` (commit \`${sha}\`)."
  echo
  echo "**Run:** ${run_url}"
  echo
  echo "Open the run, find the step that failed and read its log."
  if [ -n "$note" ]; then
    echo
    echo "$note"
  fi
} > "$out"
