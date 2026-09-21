---
title: CI/CD workflows
lead: The five GitHub Actions workflows - what triggers them, what each job does, what they need, and how to watch them.
description: Documentation of shint's GitHub Actions - lint, vulnerability check, binary build and release, Docker Hub and GHCR publishing - plus the GitHub-managed automation around them.
section: Technical
order: 5
nav: CI/CD workflows
---

## Overview

All five workflows are triggered by **the same event: pushing a tag of the form `vX.Y.Z` (digits only) that points at a commit on `main`**. Nothing else starts them: not a push to a branch (whatever its name), not a push to `main`, not a pull request, not a manual run. That is a deliberate choice - a release is the unit of quality here - and it has consequences covered under [What CI does not do](#what-ci-does-not-do).

The "on `main`" half is enforced by a [guard](#the-release-tag-guard) that every workflow runs first.

They are separate files so that each has its own status and its own failure mode: a Docker Hub credential problem shows up as *that* workflow failing, not as a vague failure of "the release".

:::html
<div class="diagram">
<svg viewBox="0 0 700 300" role="img" aria-label="Workflow graph: one tag push starts five workflows">
  <defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z"/></marker></defs>
  <rect class="box accent" x="10" y="122" width="130" height="56" rx="10"/>
  <text class="h" x="75" y="146" text-anchor="middle">git push tag</text>
  <text class="sub" x="75" y="164" text-anchor="middle">v4.0.6</text>
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
- **Least privilege:** every job declares `permissions` explicitly (`contents: read` by default; `contents: write` only to create the release; `issues: write` only to file failure issues; `packages: write` only to push to GHCR).

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

Everything else - `pull_request`, `schedule`, `workflow_dispatch`, `workflow_run`, other branches, a bare `push:` - is refused, and so is any form the checker cannot read (it fails closed: the `on:` block must be written in block style). The check has its own tests (`python3 .github/scripts/test_check_workflow_triggers.py`), including one that runs it over the real workflow files. Today the five release workflows use the first shape, the tag guard the third, and nothing uses the second; it is there for the `make check` job planned for v4.1.0.

The GitHub-managed automation listed [below](#automation-github-manages) is not defined by workflow files, so this check cannot see it. Its default setups (CodeQL, Pages) have no path filter, so a merge that only touches `.github/` still redeploys the site and rescans; neither builds or publishes anything.

## Lint

`.github/workflows/lint.yaml`

| Job | Needs | What it does |
|---|---|---|
| `verify-tag` | - | The [guard](#the-release-tag-guard). |
| `lint` | `verify-tag` | Runs `golangci-lint` with the default linter set. |
| `report-failure` | `lint` | Runs only if `lint` itself failed (not if the tag was refused and `lint` never ran): files an issue titled "CI Failure: Lint" (labels `bug`, `ci-failure`, assigned to the person who pushed the tag) from `.github/ISSUE_TEMPLATE/ci_failure.md`, using `peter-evans/create-issue-from-file@v5`. The file is static text: GitHub expands `${{ }}` expressions only inside workflow files, so the issue points at the Actions tab rather than at one specific run. |

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
| `build` | `gate` | A matrix of 8 operating systems x 2 architectures, minus two exclusions (Solaris has no arm64 port; Android/amd64 needs cgo), giving **14 binaries**. Each builds with `CGO_ENABLED=0 -buildvcs=true -trimpath -ldflags "-s -w -X main.Version=<tag>/<sha>/<time>"` and uploads a `binary-for-<os>-<arch>` artifact. `fail-fast` is off so one bad platform does not hide the others. |
| `create-release` | `build` | Downloads every artifact, writes the release notes, and creates the GitHub Release (`softprops/action-gh-release@v1`) with all binaries attached. It needs `contents: write`. |

### Release notes

The notes are assembled by the workflow and contain, in order:

1. **The commit message of the tagged commit** (`github.event.head_commit.message`). *This is where the changelog for a release shows up on GitHub*, which is why the release commit's message should carry it. See [Releases and tagging](tech-release.md).
2. A **build status table**: one row per platform, checking the artifact exists.
3. A **compare link** from the previous tag, and a `git diff --stat` summary (limited to 60,000 characters, to stay under GitHub's release-body size limit).

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
| Compute tags | From the tag `v4.0.6`: `v4.0.6`, `4.0.6`, `4.0`, `4`, and `latest`. |
| Build and push | `docker/build-push-action@v6`, platforms `linux/amd64,linux/arm64`, build argument `VERSION=<tag>`, and OCI labels for version, revision and source. |

Image names: `docker.io/<DOCKERHUB_USERNAME>/shint` and `ghcr.io/dmartsapp/shint`. The Dockerfile is described in the [Source reference](tech-source.md#build-and-packaging).

## Automation GitHub manages

A few more things run that are **not defined by files in this repository**; they are switched on in the repository settings. None of them builds or publishes a release, and none is started by a `release/**` branch:

| Name | What it is |
|---|---|
| CodeQL | GitHub's "default setup" code scanning (it currently scans the workflow files themselves); it appears in the Actions list as a dynamic workflow on pushes to `main` and on a weekly schedule. |
| pages-build-deployment | Publishes the [documentation site](tech-docs.md) from `main` after a push. |
| Graph update | Keeps GitHub's dependency graph current after a push to `main`. |
| Dependabot | Opens pull requests to update Go module dependencies (branches named `dependabot/go_modules/...`). |

## Watching a release

Push the tag, then follow it from the terminal with the GitHub CLI:

```bash
gh run list --limit 8                       # the five workflows for the tag
gh run watch <run-id> --exit-status         # block until one finishes
gh run view <run-id> --log-failed           # only the failing steps' logs
gh run rerun <run-id> --failed              # retry just the failed jobs
gh release view v4.0.6                      # the release and its assets
```

Typical durations: lint and vulnerability check under a minute, the binary release about two minutes, the two image builds four to six.

## Reproducing the checks locally

```bash
make check         # gofmt, go vet, go test -race, golangci-lint, govulncheck, docs and workflow checks
make test-full     # the same, plus the live smoke test - before a release
```

The Makefile is the single place these are defined; see [Testing](tech-testing.md#running-the-tests) for what each target does and which tools it needs. The lint version is checked against the one CI pins.

## What CI does not do

- **It does not run the test suite.** The workflows lint, check vulnerabilities and build; none runs `go test`. Tests are the release checklist's job ([Releases and tagging](tech-release.md#release-checklist)) and must pass locally before a tag is pushed.
- **It does not run on branches or pull requests**, so problems surface at tag time. Run the local checks above first. (`main` does not pick up workflow runs from other branches either: a workflow file on a branch does nothing until it is merged, and even then only a `vX.Y.Z` tag on `main` can start it.)
- **It does not accept just any tag.** A tag that is not `vX.Y.Z`, or is on a commit that is not on `main`, is refused by the [guard](#the-release-tag-guard) before anything is built.
- **It cannot un-publish.** A tag push that fails halfway can leave a partial release or images; see the recovery notes in [Releases and tagging](tech-release.md#when-a-release-goes-wrong).
