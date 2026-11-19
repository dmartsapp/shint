---
title: CI/CD workflows
lead: The two GitHub Actions workflows - Release, which a vX.Y.Z tag starts, and Check, which a merge to main starts - what triggers them, what each job does, what it hands on to the next, and how to watch them.
description: Documentation of shint's GitHub Actions - the Release workflow (lint, vulnerability check, binaries and the GitHub release, Docker Hub and GHCR images, the README proposal, the Slack and Discussions announcement) and Check - plus the GitHub-managed automation around them.
section: Technical
order: 5
nav: CI/CD workflows
---

## Overview

**[Release](#the-release-workflow)** (`.github/workflows/release.yaml`) is triggered by **one event: pushing a tag of the form `vX.Y.Z` (digits only) that points at a commit on `main`**. Nothing else starts it: not a push to a branch (whatever its name), not a push to `main`, not a pull request, not a manual run. That is a deliberate choice - a release is the unit of quality here - and it has consequences covered under [What CI does not do](#what-ci-does-not-do). The "on `main`" half is enforced by a [guard](#the-release-tag-guard), the workflow's first job.

Everything a release does is a job in that one workflow, so the run's graph on GitHub shows what waits for what: the checks run once, side by side; the binaries and the two container images build side by side once all checks passed; the GitHub release, the images' reports and the README proposal follow what they need; and the last two jobs get every job's result - one opens an issue when something failed, the other posts the outcome to Slack and announces a published release. The run has **one result**: green only when every part passed. Which part failed, where, and what it held up is in the run's graph, its summary, the Slack message and the failure issue.

**[Check](#check-after-a-merge-to-main)** (`.github/workflows/check.yaml`) runs `make check` after a merge to `main` (and only then), so tests do not wait for release day. It publishes nothing.

:::html
<div class="diagram">
<svg viewBox="0 0 760 340" role="img" aria-label="The Release workflow's jobs: verify-tag, then lint, vulncheck and hadolint side by side, then the binaries and the two images, then the GitHub release, the README proposal and the images' reports, and last the failure issue and the announcement">
  <defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z"/></marker></defs>
  <rect class="box accent" x="6" y="118" width="104" height="52" rx="10"/>
  <text class="h" x="58" y="140" text-anchor="middle">tag pushed</text>
  <text class="sub" x="58" y="158" text-anchor="middle">v{{version}}</text>
  <path class="arrow" d="M110 144H130"/>
  <rect class="box" x="130" y="118" width="110" height="52" rx="10"/>
  <text class="h" x="185" y="140" text-anchor="middle">verify-tag</text>
  <text class="sub" x="185" y="158" text-anchor="middle">vX.Y.Z on main</text>
  <path class="arrow" d="M240 144C252 144 252 52 266 52"/>
  <path class="arrow" d="M240 144H266"/>
  <path class="arrow" d="M240 144C252 144 252 236 266 236"/>
  <rect class="box" x="266" y="32" width="112" height="40" rx="9"/><text class="h" x="322" y="57" text-anchor="middle">lint</text>
  <rect class="box" x="266" y="124" width="112" height="40" rx="9"/><text class="h" x="322" y="149" text-anchor="middle">vulncheck</text>
  <rect class="box" x="266" y="216" width="112" height="40" rx="9"/><text class="h" x="322" y="241" text-anchor="middle">hadolint</text>
  <path class="arrow" d="M378 52H410"/>
  <path class="arrow" d="M378 144H410"/>
  <path class="arrow" d="M378 236H410"/>
  <path class="arrow" d="M378 52C394 52 394 144 410 144"/>
  <path class="arrow" d="M378 236C394 236 394 144 410 144"/>
  <rect class="box" x="410" y="32" width="128" height="40" rx="9"/><text class="h" x="474" y="57" text-anchor="middle">binaries (x18)</text>
  <rect class="box" x="410" y="124" width="128" height="40" rx="9"/><text class="h" x="474" y="149" text-anchor="middle">docker-hub</text>
  <rect class="box" x="410" y="216" width="128" height="40" rx="9"/><text class="h" x="474" y="241" text-anchor="middle">ghcr</text>
  <path class="arrow" d="M538 52H580"/>
  <rect class="box" x="580" y="32" width="170" height="40" rx="9"/><text class="h" x="665" y="57" text-anchor="middle">github-release</text>
  <path class="arrow" d="M665 72V98"/>
  <rect class="box" x="580" y="98" width="170" height="36" rx="9"/><text class="h" x="665" y="121" text-anchor="middle">readme</text>
  <path class="arrow" d="M538 144C560 144 560 190 580 190"/>
  <path class="arrow" d="M538 236C560 236 560 196 580 196"/>
  <rect class="box" x="580" y="172" width="170" height="40" rx="9"/><text class="h" x="665" y="197" text-anchor="middle">image-reports</text>
  <text class="sub" x="584" y="230">also needs github-release</text>
  <rect class="box" x="266" y="290" width="160" height="38" rx="9"/><text class="h" x="346" y="314" text-anchor="middle">report-failure</text>
  <path class="arrow" d="M426 309H470"/>
  <rect class="box accent" x="470" y="290" width="160" height="38" rx="9"/><text class="h" x="550" y="314" text-anchor="middle">announce</text>
  <text class="sub" x="10" y="300">last: they need every job,</text>
  <text class="sub" x="10" y="318">and get each one's result</text>
</svg>
</div>
:::

## Common ground

- **Trigger:** Release has `on: push: tags: ['v[0-9]+.[0-9]+.[0-9]+']` and nothing else; Check has `on: push: branches: [main]`. There is no `pull_request`, `schedule` or `workflow_dispatch` trigger anywhere. In GitHub's filter syntax `+` means "one or more of the previous item" and `.` is an ordinary dot, so the tag pattern is `v`, digits, dot, digits, dot, digits - and nothing after. (The pattern must be quoted in YAML.)
- **The guard:** Release's first job is `verify-tag`; every other job waits for it. See [The release-tag guard](#the-release-tag-guard).
- **Go toolchain:** `actions/setup-go@v5` with `go-version-file: go.mod` and `check-latest: true`, so CI builds with the version the module declares (or the newest patch of it).
- **Checkout:** `actions/checkout@v5`.
- **Linter:** `golangci-lint` **pinned to `v2.15.0`** through `golangci/golangci-lint-action@v7` (Check installs the same version with `go install`). It is pinned because `latest` once resolved to a build made with an older Go than the module targets and failed for that reason alone; and action v6 does not support golangci-lint v2. **A new Go can need a newer golangci-lint**: golangci-lint reads the compiled packages the module's Go writes with its own copy of `golang.org/x/tools`, so when a Go release changes that format, an older golangci-lint fails on every package it type-checks - Go 1.27.3 introduced format version 5, which golangci-lint v2.13.2 (x/tools v0.49.0) cannot read (`export data version 5 is greater than maximum supported version 4`) and v2.14.0 and later (x/tools v0.50.0 and later) can. It is the x/tools inside golangci-lint that matters, not the Go it was built with.
- **Vulnerabilities:** `golang/govulncheck-action@v1`.
- **Third-party code is pinned**: every action by commit SHA (see [Pinned actions and images](#pinned-actions-and-images)); the four tools downloaded directly - Syft, Grype, Dockle, Hadolint - by version and SHA-256 (`.github/scripts/install-tools.sh`).
- **Least privilege:** Release grants nothing at the workflow level (`permissions: {}`), and each job asks for what it needs: `contents: read` by default; `contents: write` only to create the release or add files to it (`github-release`, `image-reports`) and to push the README proposal's branch (`readme`); `packages: write` only to push to GHCR (`ghcr`); `id-token: write` and `attestations: write` only where attestations are signed (`binaries`, the image jobs); `issues: write` only to open the failure issue; `actions: read` only to read the run's jobs (`report-failure`, `announce`); `discussions: write` only to announce (`announce`). Each secret is read only by the job that uses it: Docker Hub's by `docker-hub`, the Slack webhook by `announce`.

## The release-tag guard

The `verify-tag` job of `.github/workflows/release.yaml`, running `.github/scripts/verify-release-tag.sh`.

A trigger filter looks only at the tag's *name*. It cannot ask "is the tagged commit on `main`?", and a tag pushed from a release branch by mistake would otherwise build and publish an unreleased state under a release number. So the workflow's first job, `verify-tag`, checks both things, and a failure skips every job but the announcement. It also hands on the version (`version`, `minor` and `major`: `{{version}}`, `{{minor}}`, `{{major}}`) as outputs, for the jobs that tag images:

| Check | How | Refused when |
|---|---|---|
| The name is `vX.Y.Z` | A regular expression, `^v[0-9]+\.[0-9]+\.[0-9]+$`, independent of the trigger filter | `v4.0.4-rc1`, `v1.2.3.4`, `v4.0`, `vx.y.z`, `v2.2.4ae` and so on. GitHub is not asked anything. |
| The commit is on `main` | `gh api repos/<repo>/compare/main...<sha>` and its `status` | `ahead` (the commit is newer than `main`, e.g. a release branch tip) or `diverged`. `identical` (the tag is on `main`'s tip) and `behind` (an older commit of `main`) pass. |

- **It fails closed.** If GitHub cannot answer the comparison, the release is refused rather than assumed fine.
- **Order matters.** Push `main` first, then the tag - the checklist already says so. A tag pushed before `main` has the commit is refused with a message saying to merge first; pushing `main` and re-running the run's failed jobs (`gh run rerun <run-id> --failed`) then passes, because the comparison is made live.
- **A refused tag builds and publishes nothing** and opens no "CI Failure" issue: `report-failure` runs only when `verify-tag` passed and another job failed (`always() && needs.verify-tag.result == 'success' && contains(needs.*.result, 'failure')`). (Plain `failure()` would not do: it is true when *any* ancestor job failed, which includes `verify-tag`. `always()` is there because an `if:` without a status function gets an implicit `success()`, which is false exactly when something failed.) The refusal is one red `verify-tag` job with a plain-English annotation on the run, and the Slack message says the tag was refused.
- **What it is not.** It protects against mistakes, not against someone who can push tags *and* edit workflow files - they could change the guard too. Who may create tags is a repository-settings matter.
- **Testing it.** Workflows only run on tags, so the script has its own offline test, with a fake `gh`: `bash .github/scripts/test-verify-release-tag.sh`. It covers accepted and refused names (a bad name must not reach GitHub at all), both compare outcomes, an API error, and missing inputs.

## The workflow trigger rule

**Only `main` runs actions, and a release is a `vX.Y.Z` tag on `main`.** Nothing a branch, a pull request, a schedule or a person can start may run a workflow. That rule covers every workflow, including ones not yet written, so it is checked rather than remembered: `python3 .github/scripts/check-workflow-triggers.py`, run by `make workflows` and so by `make check`, reads the `on:` block of every file in `.github/workflows/` and fails on anything else.

| A workflow may be started by | Only in this shape |
|---|---|
| A release tag | `push: tags: ['v[0-9]+.[0-9]+.[0-9]+']`, and no other filter |
| A merge to `main` | `push: branches: [main]` **with** `paths-ignore: ['.github/**']`, so a merge that only changes workflow files runs nothing |
| Another workflow | `workflow_call` alone (a reusable workflow; none is used today) |

Everything else - `pull_request`, `schedule`, `workflow_dispatch`, `workflow_run`, other branches, a bare `push:` - is refused, and so is any form the checker cannot read (it fails closed: the `on:` block must be written in block style). The check has its own tests (`python3 .github/scripts/test_check_workflow_triggers.py`), including one that runs it over the real workflow files. Today [Release](#the-release-workflow) uses the first shape and [Check](#check-after-a-merge-to-main) the second.

The GitHub-managed automation listed [below](#automation-github-manages) is not defined by workflow files, so this check cannot see it. Its default setups (CodeQL, Pages) have no path filter, so a merge that only touches `.github/` still redeploys the site and rescans; neither builds or publishes anything.

## Check after a merge to main

`.github/workflows/check.yaml`

`on: push: branches: [main]` with `paths-ignore: ['.github/**']`: it runs when something is pushed to `main`, and not when the push only changes workflow files. It does not run for a branch, a pull request or a tag.

| Job | What it does |
|---|---|
| `check` | Checks out the code, looks for a [signed local check report](#the-signed-local-check-report), sets up Go, installs Hadolint (pinned, for `make dockerfile-lint`, on both paths - a laptop seldom has it, so a report does not stand for it), and then runs one of two things. **With a valid report**: installs `govulncheck` and runs **`make check-quick`**, about a minute. **Without one**: installs `golangci-lint` v2.15.0 (the version `make check` insists on), `govulncheck` and `actionlint` with `go install`, and runs the whole **`make check`**, the command you run locally ([Testing](tech-testing.md#running-the-tests)), about three and a half minutes. Neither runs the live smoke test, which needs real hosts. |
| `report-failure` | If `check` itself failed, files an issue "CI Failure: make check" (see [Failure issues](#failure-issues)). |

Because it runs *after* the merge, a failure does not stop the merge - it opens an issue rather than blocking anything (see [Failure issues](#failure-issues)). Merging to main happens on its own timeline (see the checklist below); a release is a separate, later step: a `vX.Y.Z` tag pushed once `main` already has the release commit and Check has passed on it - `make release` does both, in that order ([Making a release](tech-release.md#making-a-release)).

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
| ...plus, separately, a `vX.Y.Z` tag pushed on a commit that is now on `main` | The Release workflow - a different event; see [Releases and tagging](tech-release.md) |

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
4. With a valid report, `make check-quick` runs what is cheap, what can differ on Linux, and what goes stale: `gofmt`, `go vet` for the runner, `go test` for the CLI tests and `lib`, the black-box battery's command-line groups (A to D), the documentation checks, and **`govulncheck` fresh** (the vulnerability database changes daily, so a report is no substitute; the CI run always asks it again). Left to the report: the race detector, the `lib/handlers` tests, the rest of the battery, `golangci-lint` (the Release workflow's `lint` job runs it again), the other operating systems' `go vet`, and the workflow tests. The job summary says which path ran and why.

**Setting it up.** Your public key goes in `.github/allowed_signers` (one line, namespace `shint-check`); the key that signs is `$SHINT_SIGNING_KEY`, else git's `user.signingkey`, else `~/.ssh/id_ed25519`, used through `ssh-keygen`, so a passphrase is asked for or taken from `ssh-agent`. A key used only for this is better than a login key. `make hooks` installs a `pre-push` hook that runs `make attest` when you push `main` or a release tag without a valid report; it is a convenience (a hook lives on one machine and `--no-verify` skips it), and `SHINT_SKIP_ATTEST=1` skips it on purpose. The gate is CI, not the hook.

**What a signature proves, honestly.** That the holder of the key stands behind this report for this tree - not that the tests were run honestly: `make attest` is what runs them, and anyone holding the key could sign a report they made up. It is the maintainer's word, made checkable and unforgeable by anyone else, not a proof. And Linux is then covered only by the short subset. That matters: the first runs of `make check` on a runner found two problems a Mac never shows (a battery case whose 500 KB argument Linux refuses, and shellcheck findings). So a direct push to `main`, or any commit without a valid report, still gets the full run.

## Failure issues

A failed check opens an issue, labels `bug` and `ci-failure`, assigned to the person who pushed:

- **Check** files "CI Failure: make check". Its body is written by `.github/scripts/write-ci-failure-issue.sh` from values the workflow passes in, so it names the branch, the commit and - the point of doing it this way - the **real URL of the failed run**. (Earlier issues used a template file, and GitHub expands `${{ }}` expressions only inside workflow files, so they showed the raw expression text; #6 to #10 are the examples.) The script has its own test, `.github/scripts/test-write-ci-failure-issue.sh`, part of `make workflows`.
- **Release** files one issue for a failed release, "CI Failure: Release vX.Y.Z", from its `report-failure` job: a table of **every part with its result**, the job and step each failure happened in (each failed binary build, by platform), why each skipped part was skipped (the part it needs did not pass), what was published anyway, and the run's URL. See [The outcome](#the-outcome-the-failure-issue-and-the-announcement).

## Moving to a new Go release

`go.mod`'s `go` line (now {{go}}) chooses the Go release for everything: every workflow sets up that Go (`go-version-file: go.mod`), the `Dockerfile`'s builder image must name the same one (`toolchain_test.go` fails otherwise), and the documentation fills it in (`\{{go}}`). shint's whole networking stack is the standard library, so **a Go patch release is often a security fix for shint**: when `govulncheck` reports a standard-library advisory, the fix is a newer Go, not a change to shint's code - v4.2.4 moved from Go 1.27.1 to 1.27.3 for exactly that, as a security patch (see the [changelog](changelog.md)).

1. Install the new Go locally and check `go version`.
2. `go mod edit -go=X.Y.Z`, and move any `golang.org/x/...` module `govulncheck` names (`go get golang.org/x/net@vA.B.C`, then `go mod tidy`). Check the module's own `go` line first: it must not ask for a newer Go than the one you are moving to.
3. `Dockerfile`: `FROM golang:X.Y.Z-alpine@sha256:<digest>`, once Docker Hub has that tag - the image is pinned to its digest, found as described in [Pinned actions and images](#pinned-actions-and-images).
4. `make lint`. If golangci-lint fails on every package with `export data version N is greater than maximum supported version M`, the new Go writes compiled packages in a format the pinned golangci-lint cannot read: move the pin to a golangci-lint whose `golang.org/x/tools` can (check its `go.mod`, and x/tools' `internal/pkgbits/version.go`) - in `Makefile` (`GOLANGCI_LINT_VERSION`), `check.yaml`, and the `lint` job of `release.yaml` - and install it: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@vX.Y.Z`. A newer golangci-lint can find new problems; fix them in the same change. Clear its cache (`golangci-lint cache clean`) before believing a clean run: cached results can hide the failure.
5. `make check` - `govulncheck` must say *No vulnerabilities found* - and a changelog entry naming the advisories.

Binaries already published keep the Go they were built with; only the next release carries the fix, so a serious advisory can be a reason for a patch release.

## Pinned actions and images

Every action a workflow uses is named by a **full commit SHA**, with its version as a comment, and both base images in the `Dockerfile` by **digest**:

```yaml
- uses: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09 # v5.1.0
```

```dockerfile
FROM golang:{{go}}-alpine@sha256:<digest> AS builder
```

A tag can be moved: whoever controls an action's repository, or an image's, could otherwise change what the release pipeline runs - and it holds the registry tokens and makes the attestations. A SHA or a digest cannot be moved. `make workflows` (in `make check`) runs `.github/scripts/check-pins.py`, which refuses a `uses:` that is not a commit SHA - a local action (`./...`) or an image by digest excepted - and a `FROM` without a digest.

Pins do not move by themselves: there is no Dependabot. **To move one** - a newer release of an action, or a rebuilt base image:

1. **An action:** find the release and its commit - `gh api repos/OWNER/REPO/releases/latest --jq .tag_name`, then `gh api repos/OWNER/REPO/git/ref/tags/vX.Y.Z --jq .object` (for an annotated tag, `object.type` is `tag`: one more `gh api repos/OWNER/REPO/git/tags/<sha> --jq .object.sha` gives the commit). Read the release's notes - a new major can change inputs.
2. **An image:** its index digest, which covers every platform - `docker buildx imagetools inspect golang:{{go}}-alpine` (the `Digest:` line), or Docker Hub's `https://hub.docker.com/v2/repositories/library/golang/tags/{{go}}-alpine` (`digest`).
3. Replace the SHA and the comment (or the digest) everywhere it appears, run `make check`, and land it on `main` like any CI change.

## The release workflow

`.github/workflows/release.yaml` - everything a release tag starts, as jobs of one workflow (the graph is at the top of this page):

| Job | Needs | What it does | Hands on |
|---|---|---|---|
| `verify-tag` | - | The [guard](#the-release-tag-guard). | `version`, `minor`, `major` |
| `lint` | `verify-tag` | `golangci-lint` with the default linter set. | - |
| `vulncheck` | `verify-tag` | `govulncheck`, its report in the run's summary (status, count, checked modules, details) - after a failure too, since the report is what says why. | `vulnerabilities`; the report, as the artifact `govulncheck-report` |
| `hadolint` | `verify-tag` | Hadolint on the `Dockerfile` (`--failure-threshold error`: an error-level finding fails, warnings are printed). | - |
| `binaries` | the checks | One job per platform, 18 in all: build, attest, checksum, SBOMs, attest those ([below](#the-binaries-and-the-github-release)). | each platform's files, as the artifact `binary-for-<os>-<arch>` |
| `github-release` | `vulncheck`, `binaries` | Writes the release notes and creates the GitHub release with every binary's files and the govulncheck report. Runs only when **every** binary built: a release is never published with some missing. | `url`, `assets` |
| `docker-hub`, `ghcr` | the checks | The multi-arch image for each registry, its SBOMs, reports and attestations ([below](#the-container-images)). | `image`, `digest`; the SBOMs and reports, as the artifacts `image-sboms-<registry>` |
| `image-reports` | `github-release`, `docker-hub`, `ghcr` | Attaches each pushed image's SBOMs and reports to the release. | - |
| `readme` | `github-release` | [README Reconcile](#readme-reconcile): proposes `main`'s README for the release. | - |
| `report-failure` | every job above | Only when one of them failed: the [failure issue](#failure-issues). | `issue` |
| `announce` | every job above, and `report-failure` | Always: the [Slack notification](#slack-notification) and, for a published release, the announcement. | - |

**The checks run once, and nothing is published unless all three passed.** The binaries and both images need `lint`, `vulncheck` and `hadolint`; the binaries and the images then build side by side, so the run takes as long as its slowest image (six to seven minutes), not the sum. Before, each of the binary and image workflows ran its own copy of the checks, and Lint and Vulnerability Check ran them again: golangci-lint and govulncheck ran four times a release.

### What a job hands on, and how a failure shows

The rule for this workflow: every part's success or failure is captured where the parts after it, and whoever reads the run, can see it.

- **Outputs, not guesses.** A job that a later one depends on publishes what it produced as job outputs - `verify-tag` the version the image jobs tag with, `github-release` the release's URL and file count, the image jobs their digests, `vulncheck` its count - and files as artifacts (the binaries, the govulncheck report, the images' SBOMs and reports). The later job reads `needs.<job>.outputs` and downloads the artifacts; nothing is recomputed or polled for.
- **Every result reaches the end.** `report-failure` and `announce` `need` every other job and run `if: always()`, so they run whatever happened, and get each job's `result` (`success`, `failure`, `cancelled`, `skipped`) and outputs as JSON (`toJSON(needs)`) - plus the run's jobs from GitHub's API, for durations and the step each failed job failed in. A skipped job is explained by the job it needs: `GitHub release: skipped - Binaries did not pass`.
- **A job that must run after a failure says what it could not do.** `image-reports` runs once the release exists, whatever the images did (`always() && needs.github-release.result == 'success'`): it attaches the reports of each image that was pushed, and fails, naming the image, when one was not - so the gap is in the run's result, not hidden.
- **One result, with the detail beside it.** The run is red when any job failed. Which job, which step, what was published anyway and what was held up is in the run's graph, the summary `announce` writes (a table of every part), the Slack message and the failure issue.

Re-running: **Re-run failed jobs** on the run (or `gh run rerun <id> --failed`) starts the failed jobs again and the ones that were skipped because of them; the jobs that passed are not run again.

### The binaries and the GitHub release

`binaries` is a matrix of 8 operating systems x 2 architectures, minus two exclusions (Solaris has no arm64 port; Android/amd64 needs cgo), plus four additions (AIX and illumos, `ppc64`/amd64 only and not part of the 8; `linux/arm` at `GOARM=6` and `linux/ppc64le`, a third and fourth Linux architecture), giving **18 binaries**. Each builds with `CGO_ENABLED=0 -buildvcs=true -trimpath -ldflags "-s -w -X main.Version=<tag>/<sha>/<time>"`, then **attests** the binary (`actions/attest@v4`: a signed SLSA build-provenance statement, keyless through Sigstore), then writes its **`.sha256` file** (`.github/scripts/write-checksums.sh`, which verifies it straight away), then its **SBOMs**, `<binary>.cdx.json` (CycloneDX 1.6) and `<binary>.spdx.json` (SPDX 2.3) (`.github/scripts/sbom.py`, which reads the build information with `go version -m -json` - no third-party SBOM tool), **attests each** (`actions/attest@v4` with `sbom-path`), and hands all four on as a `binary-for-<os>-<arch>` artifact. `fail-fast` is off so one bad platform does not hide the others - the failure issue names each one that failed.

`github-release` downloads the binaries' artifacts and the govulncheck report, writes the release notes, and creates the GitHub release (`softprops/action-gh-release@v3`) with all of them attached. It needs `contents: write`.

#### Verifying a release

Every binary has a `<binary>.sha256` next to it (a line in the format `sha256sum -c` and `shasum -a 256 -c` read), an attestation that `gh attestation verify <file> --repo dmartsapp/shint` checks, and its two SBOMs, each attested too (`--predicate-type https://cyclonedx.org/bom`, or `https://spdx.dev/Document/v2.3`). A release therefore has **89 assets: 18 binaries, each with its checksum file and its two SBOMs (72), the govulncheck report, and each registry's image SBOMs, Grype reports and Dockle checks for two platforms (16)**. v4.2.1 to v4.2.4 had 36 before their SBOMs were added afterwards (see [Software bill of materials](security.md#software-bill-of-materials)), and no image files or report (v4.2.4 has its images' SBOMs since, made afterwards: 80). `make release` checks that every binary has all three, that the images' files and the report are there, and that there are as many binaries as in the release before. Users' instructions are on the [Install page](install.md#verify-your-download). The attestation is created in the same job as the binary, before the checksum file is added, so it covers exactly the bytes that are uploaded; the workflow needs no key or certificate of its own.

#### Release notes

The notes are assembled by `github-release` and contain, in order:

1. **What's new**: the changelog's section for the tag (`.github/scripts/changelog-section.sh`); the tagged commit's message only when the changelog has no section for it. See [Releases and tagging](tech-release.md).
2. A **Verify your download** paragraph with the checksum and `gh attestation verify` commands, and the SBOMs.
3. A **compare link** from the previous tag, and a `git diff --stat` summary (limited to 60,000 characters, to stay under GitHub's release-body size limit).

(Until v4.4.0 a build status table followed What's new, a row per platform. It could only ever say "Success": the release is not created unless every platform built. The failure issue is where a failed platform shows now.)

:::note A note on script injection
The commit message is free text written by a person, so it is passed to the shell through an environment variable (`COMMIT_MSG`) and never interpolated into the script. Interpolating `${{ github.event.head_commit.message }}` directly would let a backtick in a commit message run as a command. This is GitHub's own documented footgun, and a test (`test_notify_slack.py`) checks that event text reaches a step only through `env`.
:::

### The container images

`docker-hub` and `ghcr` are the same job for two registries. They are two jobs rather than a matrix so that each holds only its own credentials: Docker Hub's secrets, or GHCR's `packages: write`.

| Step | Detail |
|---|---|
| QEMU and Buildx | `docker/setup-qemu-action@v3`, `docker/setup-buildx-action@v3`, so one job builds for two architectures. |
| Login | Docker Hub uses the repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`. GHCR uses the workflow's own `GITHUB_TOKEN` (needs `packages: write`). |
| Compute tags | From the tag and `verify-tag`'s outputs: `v{{version}}`, `{{version}}`, `{{minor}}`, `{{major}}`, and `latest`. |
| Build and push | `docker/build-push-action@v6`, platforms `linux/amd64,linux/arm64`, build argument `VERSION=<tag>`, and OCI labels for version, revision and source; `sbom: true` and `provenance: mode=max` put Docker's own attestations in the image index beside each platform's image - an SBOM made by BuildKit's scanner, and the build's full provenance - which Docker Scout (on Docker Hub) and `docker buildx imagetools inspect` read. Its digest is the job's `digest` output. |
| Syft, Grype and Dockle | `.github/scripts/install-tools.sh` downloads them at the versions pinned in it, and checks each download against the SHA-256 pinned beside the version before anything in it runs. |
| SBOMs and vulnerability reports | `.github/scripts/image-sboms.py`: for each platform in the pushed image index (the build's attestation manifests skipped), by that platform's own digest, Syft writes the image's SBOM - `image.<registry>.linux.<arch>.cdx.json` and `.spdx.json` - Grype scans it into `image.<registry>.linux.<arch>.grype.json`, and Dockle checks the image (pulled by its digest and saved as an archive, whatever the runner's own platform) against the CIS Docker Benchmark into `image.<registry>.linux.<arch>.dockle.json`; the step summary gets a table of Grype's matches by severity and Dockle's findings by level. Grype and Dockle **report and do not gate**: govulncheck gates the release. |
| Attest | Each SBOM, with `actions/attest@v4` (`subject-name` the image, `subject-digest` its platform's digest, `sbom-path`). GHCR's attestations are also pushed to the registry, as referrers of the image (`push-to-registry: true`); Docker Hub's stay on GitHub, where `gh attestation verify` finds them by digest. |
| Hand on | The eight files, as the artifact `image-sboms-<registry>`; `image-reports` attaches them to the release (`.github/scripts/upload-release-assets.sh`). |

Images get Syft rather than `sbom.py` because an image holds more than shint's binary: the distroless base's Debian packages (CA certificates, time zones, the user and group files), which Syft reads from the image's package database. The image's own shint binary is built in the Dockerfile, not the one on the release page, so its SBOM is its own.

The images build beside the binaries, not after them, so a failed binary build does not stop them: a run can push images and still not create the GitHub release. The Slack message and the failure issue say what was published.

**A new Syft, Grype, Dockle or Hadolint** is a new pin in `install-tools.sh`: a release at least two weeks old, and the SHA-256 of its linux/amd64 download from the project's own checksums file on that release. `test-install-tools.sh` checks the script against archives served from `file://`; the tools themselves first run on a release tag.

Image names: `docker.io/<DOCKERHUB_USERNAME>/shint` and `ghcr.io/dmartsapp/shint`. The Dockerfile is described in the [Source reference](tech-source.md#build-and-packaging).

### README Reconcile

The `readme` job is the safety net for `main`'s README after a release. A release made with `make release` already wrote its roadmap row in the release commit ([README after a release](tech-release.md#readme-after-a-release)), so the job usually finds nothing to do; it proposes a change only when the README is still behind the tags. It runs once the GitHub release exists (it needs `github-release`), so it never proposes a README for a release that did not go out, and runs `.github/scripts/readme-release.sh`: it branches `readme/main-vX.Y.Z` off `main`, runs `readme-reconcile.py` there, commits the result with the changes and warnings as the message, pushes the branch and opens a pull request. What the reconciliation does is in [README after a release](tech-release.md#readme-after-a-release).

| | |
|---|---|
| Permissions | `contents: write` (to push the branch, never `main`), `pull-requests: write` and `issues: write` |
| If Actions may not open pull requests | Opening one fails with "GitHub Actions is not permitted to create pull requests" while the repository setting *Allow GitHub Actions to create and approve pull requests* is off. The script then opens an **issue** that links the branch (a compare link), so the proposal is not lost. Turn the setting on (Settings, Actions, General) to get the pull request instead |
| After the release | It publishes nothing. A failure here is reported like any other - the run is red, the failure issue names it - but the release was already published, so it is announced all the same (the Slack message is amber and says the proposal did not go through) |
| By hand | `make readme-reconcile TAG=vX.Y.Z` |

Everything reaches the script through environment variables, never interpolated into the shell. The script can commit a *clean* reconcile straight to `main` when asked (`AUTO_COMMIT=1`, see [README after a release](tech-release.md#readme-after-a-release)); this job does not ask, so it only ever proposes. Like the rest of the workflow it can only be exercised for real by a tag, so what stands in for that is the tests: `test_readme_reconcile.py` (the reconciliation, on README fixtures) and `test-readme-release.sh` (the branch, the commit, the push, the pull request and the issue fallback, in throw-away repositories with a fake `gh`), both part of `make workflows`.

### The outcome: the failure issue and the announcement

`report-failure` and `announce` both run `.github/scripts/notify-slack.py`, with `NEEDS` (every job's result and outputs) and the run's id.

#### Slack notification

`announce` is the last word on a release. It posts **one message** to Slack with the result:

| Part | What it shows |
|---|---|
| Headline and colour | `shint v{{version}} is released` on green when every part passed; on amber when the release was published but the README proposal failed; `the release failed` on red, with what was published anyway (`published: Docker Hub image, GHCR image`); `the tag was refused` on red when `verify-tag` failed |
| One line per part | The tag, golangci-lint, govulncheck, Hadolint, the binaries, the GitHub release, each image, the images' reports and the README proposal - each with its result, what it handed on (the version, the vulnerability count, the number of files, the image digest), its duration, and a link to its job |
| What failed | For a failed part, each failed job and the step it failed in (`Build linux/arm64 › Build the binary`); for a skipped part, the part it needed that did not pass |
| Footer | The tag, the commit (linked) and its subject, and who pushed the tag |
| Buttons | The release notes (when published), the run, and the failure issue (when one was opened) |

It writes the same table, a row per part, to the run's summary.

**When the release was published, it is announced too**, for the people who use shint rather than the ones who build it:

- **A second Slack post**, right after the result: `shint v{{version}} is out`, the release's one-line summary (the `Summary:` line of `releases/vX.Y.Z.md`), the headline of each changelog entry for the version (its leading bold sentence; "Release process" entries are left out, and after eight the rest are counted), how to get it (the release, `docker pull`, `go install`), and buttons for the release notes, the changelog and the documentation.
- **A discussion** in the repository's *Announcements* category, titled `vX.Y.Z released`, with the same content in Markdown and every highlight. It is made once: a re-run that finds a discussion with that title leaves it alone. This is why the job has `discussions: write`.

A release that was not published is never announced. The body goes to `gh` as a raw field (`-f`), so its text is never read as a file name - the mistake that once posted a file path as eight announcements' text.

**It cannot hurt a release.** It publishes nothing and changes nothing: if it fails, or Slack is down, the release is exactly as it was. Until v4.4.0 it was a workflow of its own that polled the release workflows every 20 seconds for up to 50 minutes, because workflows cannot `need` each other; as a job it needs no waiting at all.

**The webhook is a credential.** It is the repository secret `SLACK_WEBHOOK_URL` (Settings, Secrets and variables, Actions), read into an environment variable by `announce` alone and used only by `notify-slack.py`, which never prints it - not in the log, not in an error. Nothing in the repository contains it, and a test fails `make workflows` if a Slack webhook URL is ever committed. Without the secret (a fork, or it was removed) the job says so and succeeds. To point the messages somewhere else, create a new incoming webhook in Slack, replace the secret (`gh secret set SLACK_WEBHOOK_URL`) and revoke the old one.

**Keeping it right.** The script's list of parts, and of what each part needs, is checked against `release.yaml` by a test: a job added to the workflow without being listed there, or a `needs:` that changed, fails `make workflows`. The rest of `test_notify_slack.py` covers the results (published, failed, refused; why a part was skipped; what was published anyway), the message, the failure issue, and the posting (against a local server: success, a refusal that is not retried, a server error that is, the URL never in the output).

## Automation GitHub manages

A few more things run that are **not defined by files in this repository**; they are switched on in the repository settings. None of them builds or publishes a release, and none is started by a `release/**` branch:

| Name | What it is |
|---|---|
| CodeQL | GitHub's "default setup" code scanning (it currently scans the workflow files themselves); it appears in the Actions list as a dynamic workflow on pushes to `main` and on a weekly schedule. |
| pages-build-deployment | Publishes the [documentation site](tech-docs.md) from `main` after a push. |
| Graph update | Keeps GitHub's dependency graph current after a push to `main`. |
| Dependabot | Opens pull requests to update Go module dependencies (branches named `dependabot/go_modules/...`). |

## Watching a release

Push the tag and the result arrives in Slack when the run's last job finishes (see [Slack notification](#slack-notification)); `make release` follows the run too and prints every job's result. To follow it from the terminal yourself, use the GitHub CLI:

```bash
gh run list --workflow release.yaml --limit 3    # the tag's Release run
gh run watch <run-id> --exit-status              # every job, live, until the run finishes
gh run view <run-id>                             # the jobs and their results
gh run view <run-id> --log-failed                # only the failing steps' logs
gh run rerun <run-id> --failed                   # the failed jobs again, and the ones they held up
gh release view v{{version}}                           # the release and its assets
```

Typical durations (from v4.4.0's runs): the tag check a few seconds, the three checks under a minute each, the 18 binaries about a minute side by side, the release half a minute, the two image builds six to seven minutes - about eight minutes from the tag to the announcement.

## Reproducing the checks locally

```bash
make check         # gofmt, go vet, go test -race, golangci-lint, govulncheck, docs and workflow checks
make test-full     # the same, plus the live smoke test - before a release
```

The Makefile is the single place these are defined; see [Testing](tech-testing.md#running-the-tests) for what each target does and which tools it needs. The lint version is checked against the one CI pins.

## What CI does not do

- **It does not run the tests before a merge.** The Release workflow lints, checks vulnerabilities and builds; it does not run `go test`. [Check](#check-after-a-merge-to-main) runs the whole suite, but only after a push to `main`; nothing runs on a branch or a pull request. Run `make check` locally first ([Testing](tech-testing.md#running-the-tests)).
- **It does not run on branches or pull requests**, so problems surface after a merge or at tag time. (`main` does not pick up workflow runs from other branches either: a workflow file on a branch does nothing until it is merged, and even then only a `vX.Y.Z` tag on `main` can start it.)
- **It does not accept just any tag.** A tag that is not `vX.Y.Z`, or is on a commit that is not on `main`, is refused by the [guard](#the-release-tag-guard) before anything is built.
- **It cannot un-publish.** A tag push that fails halfway can leave a partial release or images; see the recovery notes in [Releases and tagging](tech-release.md#when-a-release-goes-wrong).

## Backup mirror

The repository is mirrored to a second, currently private git host, kept in sync automatically and independently of anything in this document - no workflow here pushes to it, and nothing about the CI/CD pipeline above depends on it existing. It exists purely so the source and its full history survive an outage or loss of access to the primary repository; it is not a second place to file issues, open pull requests, or expect any of the automation described on this page.
