---
title: CI/CD workflows
lead: The eight GitHub Actions workflows (five for releases, a notifier, a README proposal, and one that checks main) - what triggers them, what each job does, what they need, and how to watch them.
description: Documentation of shint's GitHub Actions - lint, vulnerability check, binary build and release, Docker Hub and GHCR publishing - plus the GitHub-managed automation around them.
section: Technical
order: 5
nav: CI/CD workflows
---

## Overview

**Five release workflows** - Lint, Vulnerability Check, Binary Build & Release, Docker Hub Release and GHCR Release - are triggered by **the same event: pushing a tag of the form `vX.Y.Z` (digits only) that points at a commit on `main`**. Nothing else starts them: not a push to a branch (whatever its name), not a push to `main`, not a pull request, not a manual run. That is a deliberate choice - a release is the unit of quality here - and it has consequences covered under [What CI does not do](#what-ci-does-not-do).

The "on `main`" half is enforced by a [guard](#the-release-tag-guard) that every release workflow runs first.

**One more workflow, [Check](#check-after-a-merge-to-main)**, runs `make check` after a merge to `main` (and only then), so tests do not wait for release day. It publishes nothing.

They are separate files so that each has its own status and its own failure mode: a Docker Hub credential problem shows up as *that* workflow failing, not as a vague failure of "the release".

A sixth workflow, [Notify Slack](#slack-notification), starts with the same tag but builds and publishes nothing: it waits for the five and posts how each of them ended. A seventh, [README Reconcile](#readme-reconcile), starts with the tag too, publishes nothing and takes under a minute: it proposes `main`'s README for the release.

:::html
<div class="diagram">
<svg viewBox="0 0 700 300" role="img" aria-label="Workflow graph: one tag push starts five workflows">
  <defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z"/></marker></defs>
  <rect class="box accent" x="10" y="122" width="130" height="56" rx="10"/>
  <text class="h" x="75" y="146" text-anchor="middle">git push tag</text>
  <text class="sub" x="75" y="164" text-anchor="middle">v4.2.1</text>
  <path class="arrow" d="M140 150C170 150 170 30 200 30"/>
  <path class="arrow" d="M140 150C170 150 170 90 200 90"/>
  <path class="arrow" d="M140 150H200"/>
  <path class="arrow" d="M140 150C170 150 170 210 200 210"/>
  <path class="arrow" d="M140 150C170 150 170 270 200 270"/>
  <rect class="box" x="200" y="8" width="150" height="44" rx="9"/><text class="h" x="275" y="34" text-anchor="middle">Lint</text>
  <rect class="box" x="200" y="68" width="150" height="44" rx="9"/><text class="h" x="275" y="94" text-anchor="middle">Vulnerability Check</text>
  <rect class="box" x="200" y="128" width="150" height="44" rx="9"/><text class="h" x="275" y="154" text-anchor="middle">Binary Build &amp; Release</text>
  <rect class="box" x="200" y="188" width="150" height="44" rx="9"/><text class="h" x="275" y="214" text-anchor="middle">Docker Hub Release</text>
  <rect class="box" x="200" y="248" width="150" height="44" rx="9"/><text class="h" x="275" y="274" text-anchor="middle">GHCR Release</text>
  <text class="sub" x="372" y="34">reports failures as an issue</text>
  <text class="sub" x="372" y="94">reports failures as an issue</text>
  <text class="sub" x="372" y="146">gate → 14 builds → GitHub Release</text>
  <text class="sub" x="372" y="206">gate → multi-arch image → docker.io</text>
  <text class="sub" x="372" y="266">gate → multi-arch image → ghcr.io</text>
</svg>
</div>
:::

## Common ground

- **Trigger:** `on: push: tags: ['v[0-9]+.[0-9]+.[0-9]+']`, and no `branches`, `pull_request`, `schedule` or `workflow_dispatch` trigger anywhere. In GitHub's filter syntax `+` means "one or more of the previous item" and `.` is an ordinary dot, so this is `v`, digits, dot, digits, dot, digits - and nothing after. (The pattern must be quoted in YAML.)
- **The guard:** the first job of every workflow is `verify-tag`, which calls the reusable workflow `verify-release-tag.yaml`; every other job waits for it. See [The release-tag guard](#the-release-tag-guard).
- **Go toolchain:** `actions/setup-go@v5` with `go-version-file: go.mod` and `check-latest: true`, so CI builds with the version the module declares (or the newest patch of it).
- **Checkout:** `actions/checkout@v5`.
- **Linter:** `golangci-lint` **pinned to `v2.13.2`** through `golangci/golangci-lint-action@v7`. It is pinned because `latest` once resolved to a build made with an older Go than the module targets and failed for that reason alone; and action v6 does not support golangci-lint v2.
- **Vulnerabilities:** `golang/govulncheck-action@v1`.
- **Least privilege:** every job declares `permissions` explicitly (`contents: read` by default; `contents: write` only to create the release; `issues: write` only to file failure issues; `packages: write` only to push to GHCR; `id-token: write` and `attestations: write` only in the binary build, to sign and store its attestation).

## The release-tag guard

`.github/workflows/verify-release-tag.yaml` (a reusable workflow: it has no trigger of its own, so nothing can start it directly) and `.github/scripts/verify-release-tag.sh`.

A trigger filter looks only at the tag's *name*. It cannot ask "is the tagged commit on `main`?", and a tag pushed from a release branch by mistake would otherwise build and publish an unreleased state under a release number. So each workflow's first job, `verify-tag`, checks both things, and a failure skips every other job in that workflow:

| Check | How | Refused when |
|---|---|---|
| The name is `vX.Y.Z` | A regular expression, `^v[0-9]+\.[0-9]+\.[0-9]+$`, independent of the trigger filter | `v4.0.4-rc1`, `v1.2.3.4`, `v4.0`, `vx.y.z`, `v2.2.4ae` and so on. GitHub is not asked anything. |
| The commit is on `main` | `gh api repos/<repo>/compare/main...<sha>` and its `status` | `ahead` (the commit is newer than `main`, e.g. a release branch tip) or `diverged`. `identical` (the tag is on `main`'s tip) and `behind` (an older commit of `main`) pass. |

- **It fails closed.** If GitHub cannot answer the comparison, the release is refused rather than assumed fine.
- **Order matters.** Push `main` first, then the tag - the checklist already says so. A tag pushed before `main` has the commit is refused with a message saying to merge first; pushing `main` and re-running the failed workflows (`gh run rerun <run-id> --failed`) then passes, because the comparison is made live.
- **A refused tag builds and publishes nothing** and opens no "CI Failure" issue: the failure reports in each workflow's `report-failure` job are conditioned on the *check* failing (`always() && needs.<job>.result == 'failure'`), and a skipped job is not a failed one. (Plain `failure()` would not do: it is true when *any* ancestor job failed, which includes `verify-tag`. `always()` is there because an `if:` without a status function gets an implicit `success()`, which is false exactly when the check failed.) The refusal is one red `verify-tag` job with a plain-English annotation on the run.
- **What it is not.** It protects against mistakes, not against someone who can push tags *and* edit workflow files - they could change the guard too. Who may create tags is a repository-settings matter.
- **Testing it.** Workflows only run on tags, so the script has its own offline test, with a fake `gh`: `bash .github/scripts/test-verify-release-tag.sh`. It covers accepted and refused names (a bad name must not reach GitHub at all), both compare outcomes, an API error, and missing inputs.

## The workflow trigger rule

**Only `main` runs actions, and a release is a `vX.Y.Z` tag on `main`.** Nothing a branch, a pull request, a schedule or a person can start may run a workflow. That rule covers every workflow, including ones not yet written, so it is checked rather than remembered: `python3 .github/scripts/check-workflow-triggers.py`, run by `make workflows` and so by `make check`, reads the `on:` block of every file in `.github/workflows/` and fails on anything else.

| A workflow may be started by | Only in this shape |
|---|---|
| A release tag | `push: tags: ['v[0-9]+.[0-9]+.[0-9]+']`, and no other filter |
| A merge to `main` | `push: branches: [main]` **with** `paths-ignore: ['.github/**']`, so a merge that only changes workflow files runs nothing |
| Another workflow | `workflow_call` alone (a reusable workflow, like the tag guard) |

Everything else - `pull_request`, `schedule`, `workflow_dispatch`, `workflow_run`, other branches, a bare `push:` - is refused, and so is any form the checker cannot read (it fails closed: the `on:` block must be written in block style). The check has its own tests (`python3 .github/scripts/test_check_workflow_triggers.py`), including one that runs it over the real workflow files. Today the five release workflows use the first shape, [Check](#check-after-a-merge-to-main) the second, and the tag guard the third.

The GitHub-managed automation listed [below](#automation-github-manages) is not defined by workflow files, so this check cannot see it. Its default setups (CodeQL, Pages) have no path filter, so a merge that only touches `.github/` still redeploys the site and rescans; neither builds or publishes anything.

## Check after a merge to main

`.github/workflows/check.yaml`

`on: push: branches: [main]` with `paths-ignore: ['.github/**']`: it runs when something is pushed to `main`, and not when the push only changes workflow files. It does not run for a branch, a pull request or a tag.

| Job | What it does |
|---|---|
| `check` | Checks out the code, looks for a [signed local check report](#the-signed-local-check-report), sets up Go, and then runs one of two things. **With a valid report**: installs `govulncheck` and runs **`make check-quick`**, about a minute. **Without one**: installs `golangci-lint` v2.13.2 (the version `make check` insists on), `govulncheck` and `actionlint` with `go install`, and runs the whole **`make check`**, the command you run locally ([Testing](tech-testing.md#running-the-tests)), about three and a half minutes. Neither runs the live smoke test, which needs real hosts. |
| `report-failure` | If `check` itself failed, files an issue "CI Failure: make check" (see [Failure issues](#failure-issues)). |

Because it runs *after* the merge, a failure does not stop the merge - it opens an issue rather than blocking anything (see [Failure issues](#failure-issues)). Merging to main happens on its own timeline (see the checklist below); a release is a separate, later step: a `vX.Y.Z` tag pushed once `main` already has the release commit ([Releases and tagging](tech-release.md#release-checklist)).

### Merging a branch to main: a practical checklist

A plain merge to main (no tag) is the everyday case: a branch's fixes are ready and get folded in. It is different from a release, which is a `vX.Y.Z` tag pushed *afterwards* (see [Releases and tagging](tech-release.md)) - merging to main by itself publishes nothing and builds nothing. It only triggers `Check`.

**Before you merge**, on the branch, at the commit that is about to become `main`'s tip:

1. **`make check` passes** - or better, **`make attest`**, which runs `make check` and then signs and attaches a report, so the push gets the quick path below instead of the full one (see [The signed local check report](#the-signed-local-check-report)).
2. **The branch is caught up with `origin/main`** (`git fetch origin && git merge --ff-only origin/main`), so the merge into `main` really can be a fast-forward - never a rebase of pushed history, and never a merge commit that could smuggle in something CI has not checked. Whatever state the branch is in is exactly what lands on `main`.
3. **No attribution trailers** in the commits (shint's own convention; `make release-check` enforces it on release day, but the habit starts here).

**The merge:**

```bash
git switch your-branch
git fetch origin && git merge --ff-only origin/main   # only if main moved since you started
make attest                                            # optional but recommended: signs + attaches the report
git switch main
git merge --ff-only your-branch
git push origin main
```

**What that push fires:**

| The push | Fires |
|---|---|
| Touches only `.github/**` | Nothing (see [the trigger rule](#the-workflow-trigger-rule)) |
| Touches anything else | `Check` - `make check-quick` (about a minute) if the pushed commit carries a valid signed report, otherwise the whole `make check` (about three and a half minutes) |
| ...plus, separately, a `vX.Y.Z` tag pushed on a commit that is now on `main` | The five release workflows, `Notify Slack` and `README Reconcile` - a different event; see [Releases and tagging](tech-release.md) |

A plain push to `main` never builds a binary, never publishes an image or a release, and never touches the README by itself - all of that waits for the tag.

**Watching it:**

```bash
gh run list --branch main --limit 3
gh run watch $(gh run list --branch main --limit 1 --json databaseId --jq '.[0].databaseId')
```

### The signed local check report

The whole of `make check` takes about three and a half minutes on a runner (roughly: tools to install 1 minute, `go vet` for four operating systems and a 32-bit architecture (linux/arm) 50 s, the race-detector tests 70 s, the battery 50 s). The maintainer runs it anyway, on the commit, before merging - so CI does not have to repeat all of it, provided it can **check** that the run happened and **cannot be given a forgery**.

1. `make attest` (`.github/scripts/attest.sh`) runs the whole `make check` on a clean tree, builds a **report** from what it printed (`.github/scripts/check-report.py build`: the tree and commit, the time, the machine and tool versions, every stage with its result, the battery's counts, a hash of the log), **signs the report's exact bytes with an SSH key** (`ssh-keygen -Y sign`, namespace `shint-check`), and attaches report plus signature to the commit as a **git note** in `refs/notes/checks`, which it pushes. A note is used rather than a file in the repository because a committed report would change the tree it certifies and conflict on every merge; a note is tied to the commit id and travels beside the code.
2. A fast-forward merge keeps commit ids, so the note on the branch tip is the note on the commit pushed to `main`.
3. `check` (`.github/scripts/verify-local-check.sh`) fetches the note and `check-report.py verify` accepts it only if **all** of these hold: the signing key is in `.github/allowed_signers` - **as it was on the commit before this push**, so a push cannot approve itself by adding its own key (a push that introduces the file gets the whole suite); the signature matches the report byte for byte and was made for this purpose (the namespace); the report is for **the tree that was checked out**; it lists **every stage** of `make check`, all ok, result `pass`; and it is **recent** (14 days) and not from the future. Anything else - no note, another tree, a forged, edited or stale one, a key that is not allowed, an API error - means *no report*, and the whole suite runs. A missing report costs time, never coverage.
4. With a valid report, `make check-quick` runs what is cheap, what can differ on Linux, and what goes stale: `gofmt`, `go vet` for the runner, `go test` for the CLI tests and `lib`, the black-box battery's command-line groups (A to D), the documentation checks, and **`govulncheck` fresh** (the vulnerability database changes daily, so a report is no substitute; the CI run always asks it again). Left to the report: the race detector, the `lib/handlers` tests, the rest of the battery, `golangci-lint` (the release's own Lint workflow and gate run it again), the other operating systems' `go vet`, and the workflow tests. The job summary says which path ran and why.

**Setting it up.** Your public key goes in `.github/allowed_signers` (one line, namespace `shint-check`); the key that signs is `$SHINT_SIGNING_KEY`, else git's `user.signingkey`, else `~/.ssh/id_ed25519`, used through `ssh-keygen`, so a passphrase is asked for or taken from `ssh-agent`. A key used only for this is better than a login key. `make hooks` installs a `pre-push` hook that runs `make attest` when you push `main` or a release tag without a valid report; it is a convenience (a hook lives on one machine and `--no-verify` skips it), and `SHINT_SKIP_ATTEST=1` skips it on purpose. The gate is CI, not the hook.

**What a signature proves, honestly.** That the holder of the key stands behind this report for this tree - not that the tests were run honestly: `make attest` is what runs them, and anyone holding the key could sign a report they made up. It is the maintainer's word, made checkable and unforgeable by anyone else, not a proof. And Linux is then covered only by the short subset. That matters: the first runs of `make check` on a runner found two problems a Mac never shows (a battery case whose 500 KB argument Linux refuses, and shellcheck findings). So a direct push to `main`, or any commit without a valid report, still gets the full run.

## Failure issues

Lint, Vulnerability Check and Check each file an issue when their check fails: title "CI Failure: <name>", labels `bug` and `ci-failure`, assigned to the person who pushed. The body is written by `.github/scripts/write-ci-failure-issue.sh` from values the workflow passes in, so it names the workflow, the tag or branch, the commit and - the point of doing it this way - the **real URL of the failed run**. (Earlier issues used a template file, and GitHub expands `${{ }}` expressions only inside workflow files, so they showed the raw expression text; #6 to #10 are the examples.) The script has its own test, `.github/scripts/test-write-ci-failure-issue.sh`, part of `make workflows`.

## Lint

`.github/workflows/lint.yaml`

| Job | Needs | What it does |
|---|---|---|
| `verify-tag` | - | The [guard](#the-release-tag-guard). |
| `lint` | `verify-tag` | Runs `golangci-lint` with the default linter set. |
| `report-failure` | `lint` | Runs only if `lint` itself failed (not if the tag was refused and `lint` never ran): files the "CI Failure: Lint" issue described under [Failure issues](#failure-issues), using `peter-evans/create-issue-from-file@v5`. |

## Vulnerability check

`.github/workflows/vulncheck.yaml`

| Job | Needs | What it does |
|---|---|---|
| `verify-tag` | - | The [guard](#the-release-tag-guard). |
| `govulncheck` | `verify-tag` | Runs `govulncheck`, writes its report to a file, and turns it into a step summary table (status, vulnerability count, checked modules, details). |
| `report-failure` | `govulncheck` | If the check itself failed, files an issue "CI Failure: Vulnerability Check" like the lint workflow does. |

## Binary build and release

`.github/workflows/build.yaml`

| Job | Needs | What it does |
|---|---|---|
| `verify-tag` | - | The [guard](#the-release-tag-guard). |
| `gate` | `verify-tag` | Re-runs `golangci-lint` and `govulncheck` **quietly**. It does not report (the standalone workflows do); it exists so a failing check stops this workflow from shipping binaries. |
| `build` | `gate` | A matrix of 8 operating systems x 2 architectures, minus two exclusions (Solaris has no arm64 port; Android/amd64 needs cgo), plus four additions (AIX and illumos, `ppc64`/amd64 only and not part of the 8; `linux/arm` at `GOARM=6` and `linux/ppc64le`, a third and fourth Linux architecture), giving **18 binaries**. Each builds with `CGO_ENABLED=0 -buildvcs=true -trimpath -ldflags "-s -w -X main.Version=<tag>/<sha>/<time>"`, then **attests** the binary (`actions/attest@v4`: a signed SLSA build-provenance statement, keyless through Sigstore), then writes its **`.sha256` file** (`.github/scripts/write-checksums.sh`, which verifies it straight away), and uploads both as a `binary-for-<os>-<arch>` artifact. `fail-fast` is off so one bad platform does not hide the others. |
| `create-release` | `build` | Downloads every artifact, writes the release notes, and creates the GitHub Release (`softprops/action-gh-release@v3`) with all binaries attached. It needs `contents: write`. |

### Verifying a release

Every binary has a `<binary>.sha256` next to it (a line in the format `sha256sum -c` and `shasum -a 256 -c` read) and an attestation that `gh attestation verify <file> --repo dmartsapp/shint` checks. A release therefore has **28 assets: 14 binaries and 14 checksum files**. Users' instructions are on the [Install page](install.md#verify-your-download). The attestation is created in the same job as the binary, before the checksum file is added, so it covers exactly the bytes that are uploaded; the workflow needs no key or certificate of its own.

### Release notes

The notes are assembled by the workflow and contain, in order:

1. **The commit message of the tagged commit** (`github.event.head_commit.message`). *This is where the changelog for a release shows up on GitHub*, which is why the release commit's message should carry it. See [Releases and tagging](tech-release.md).
2. A **build status table**: one row per platform, checking the artifact exists.
3. A **Verify your download** paragraph with the checksum and `gh attestation verify` commands.
4. A **compare link** from the previous tag, and a `git diff --stat` summary (limited to 60,000 characters, to stay under GitHub's release-body size limit).

:::note A note on script injection
The commit message is free text written by a person, so it is passed to the shell through an environment variable (`COMMIT_MSG`) and never interpolated into the script. Interpolating `${{ github.event.head_commit.message }}` directly would let a backtick in a commit message run as a command. This is GitHub's own documented footgun.
:::

## Docker Hub and GHCR

`.github/workflows/docker-hub.yaml` and `ghcr.yaml` are identical except for the registry.

| Step | Detail |
|---|---|
| `verify-tag` | The [guard](#the-release-tag-guard); the `gate` job waits for it. |
| `gate` | The same quiet lint + vulnerability gate. |
| QEMU and Buildx | `docker/setup-qemu-action@v3`, `docker/setup-buildx-action@v3`, so one job builds for two architectures. |
| Login | Docker Hub uses the repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`. GHCR uses the workflow's own `GITHUB_TOKEN` (needs `packages: write`). |
| Compute tags | From the tag `v4.2.1`: `v4.2.1`, `4.2.1`, `4.2`, `4`, and `latest`. |
| Build and push | `docker/build-push-action@v6`, platforms `linux/amd64,linux/arm64`, build argument `VERSION=<tag>`, and OCI labels for version, revision and source. |

Image names: `docker.io/<DOCKERHUB_USERNAME>/shint` and `ghcr.io/dmartsapp/shint`. The Dockerfile is described in the [Source reference](tech-source.md#build-and-packaging).

## README Reconcile

`README Reconcile` (`.github/workflows/readme-reconcile.yaml`) proposes `main`'s README after a release. It starts with the tag like the others and runs `.github/scripts/readme-release.sh` straight away - it needs only the tag, the changelog and `main`, so it does not wait for the release workflows, and a runner that only waits is billed time. The script branches `readme/main-vX.Y.Z` off `main`, runs `readme-reconcile.py` there, commits the result with the changes and warnings as the message, pushes the branch and opens a pull request. What the reconciliation does is in [README after a release](tech-release.md#readme-after-a-release).

| | |
|---|---|
| Permissions | `contents: write` (to push the branch, never `main`), `pull-requests: write` and `issues: write` |
| If Actions may not open pull requests | Opening one fails with "GitHub Actions is not permitted to create pull requests" while the repository setting *Allow GitHub Actions to create and approve pull requests* is off. The script then opens an **issue** that links the branch (a compare link), so the proposal is not lost. Turn the setting on (Settings, Actions, General) to get the pull request instead |
| Not part of the release | The notifier does not wait for it and it does not wait for the release: a failure here leaves the release untouched (a test keeps the notifier's list of workflows free of it on purpose). Whether the release itself passed is what the Slack message says - read it before merging the proposal |
| The trigger rule | A `vX.Y.Z` tag push and nothing else, like every release workflow; there is no manual button. The same thing by hand is `make readme-reconcile TAG=vX.Y.Z` |
| Serialised | `concurrency: readme-reconcile`, so two tags pushed together are proposed one after the other |

Everything reaches the script through environment variables, never interpolated into the shell. The script can commit a *clean* reconcile straight to `main` when asked (`AUTO_COMMIT=1`, see [README after a release](tech-release.md#readme-after-a-release)); this workflow does not ask, so it only ever proposes. Like the other workflows it can only be exercised for real by a tag, so what stands in for that is the tests: `test_readme_reconcile.py` (the reconciliation, on README fixtures) and `test-readme-release.sh` (the branch, the commit, the push, the pull request and the issue fallback, in throw-away repositories with a fake `gh`), both part of `make workflows`.

## Slack notification

`Notify Slack` (`.github/workflows/notify-slack.yaml`) is the last word on a release. It starts with the tag like the others, waits until the five release workflows have finished, and posts **one message** to Slack:

| Part | What it shows |
|---|---|
| Headline and colour | `shint v4.2.1 is released` on green when everything passed; `the release pipeline failed` on red; `still running` on amber if the wait ran out |
| One line per workflow | Binary build and release (with the number of files attached to the release), Vulnerability check, Linting, Docker Hub image, GHCR image - each with its duration and a link to its run |
| What failed | For a failed workflow, the job and step (`golangci-lint › Run golangci-lint`), up to three |
| Footer | The tag, the commit (linked) and its subject, and who pushed the tag |
| Buttons | The release notes (only when everything passed) and the tag's workflow runs |

**It cannot start any other way and cannot hurt a release.** It obeys the [trigger rule](#the-workflow-trigger-rule) - a tag push, nothing else - so it cannot use `workflow_run` to be told when the others finish; it looks at the runs of its own tag every 20 seconds (`gh run list`, needing only `actions: read`) until all five have completed, for at most 50 minutes. It publishes nothing and changes nothing: if it fails, or Slack is down, the release is exactly as it was, and the failure shows only on this workflow's own run.

**The webhook is a credential.** It is the repository secret `SLACK_WEBHOOK_URL` (Settings, Secrets and variables, Actions), read into an environment variable and used only by `.github/scripts/notify-slack.py`, which never prints it - not in the log, not in an error. Nothing in the repository contains it, and a test fails `make workflows` if a Slack webhook URL is ever committed. Without the secret (a fork, or it was removed) the job says so and succeeds. To point the messages somewhere else, create a new incoming webhook in Slack, replace the secret (`gh secret set SLACK_WEBHOOK_URL`) and revoke the old one.

**Keeping it right.** The list of workflows it waits for is in the script; a test compares it with the workflow files and fails if a workflow that a tag starts is missing from it (or one that is listed no longer exists), so adding a release workflow means adding it there too. The rest of `test_notify_slack.py` covers the message (every state: passed, failed, still running), the waiting (late-starting workflows, an API hiccup, the deadline) and the posting (against a local server: success, a refusal that is not retried, a server error that is, the URL never in the output). `make workflows` runs it.

## Automation GitHub manages

A few more things run that are **not defined by files in this repository**; they are switched on in the repository settings. None of them builds or publishes a release, and none is started by a `release/**` branch:

| Name | What it is |
|---|---|
| CodeQL | GitHub's "default setup" code scanning (it currently scans the workflow files themselves); it appears in the Actions list as a dynamic workflow on pushes to `main` and on a weekly schedule. |
| pages-build-deployment | Publishes the [documentation site](tech-docs.md) from `main` after a push. |
| Graph update | Keeps GitHub's dependency graph current after a push to `main`. |
| Dependabot | Opens pull requests to update Go module dependencies (branches named `dependabot/go_modules/...`). |

## Watching a release

Push the tag and the result arrives in Slack when the last workflow finishes (see [Slack notification](#slack-notification)). To follow it from the terminal instead, use the GitHub CLI:

```bash
gh run list --limit 8                       # the five workflows for the tag, and the notifier (and Check, for a merge)
gh run watch <run-id> --exit-status         # block until one finishes
gh run view <run-id> --log-failed           # only the failing steps' logs
gh run rerun <run-id> --failed              # retry just the failed jobs
gh release view v4.2.1                      # the release and its assets
```

Typical durations: lint and vulnerability check under a minute, the binary release about two minutes, the two image builds four to six.

## Reproducing the checks locally

```bash
make check         # gofmt, go vet, go test -race, golangci-lint, govulncheck, docs and workflow checks
make test-full     # the same, plus the live smoke test - before a release
```

The Makefile is the single place these are defined; see [Testing](tech-testing.md#running-the-tests) for what each target does and which tools it needs. The lint version is checked against the one CI pins.

## What CI does not do

- **It does not run the tests before a merge.** The release workflows lint, check vulnerabilities and build; none runs `go test`. [Check](#check-after-a-merge-to-main) runs the whole suite, but only after a push to `main`; nothing runs on a branch or a pull request. Run `make check` locally first ([Testing](tech-testing.md#running-the-tests)).
- **It does not run on branches or pull requests**, so problems surface after a merge or at tag time. (`main` does not pick up workflow runs from other branches either: a workflow file on a branch does nothing until it is merged, and even then only a `vX.Y.Z` tag on `main` can start it.)
- **It does not accept just any tag.** A tag that is not `vX.Y.Z`, or is on a commit that is not on `main`, is refused by the [guard](#the-release-tag-guard) before anything is built.
- **It cannot un-publish.** A tag push that fails halfway can leave a partial release or images; see the recovery notes in [Releases and tagging](tech-release.md#when-a-release-goes-wrong).
