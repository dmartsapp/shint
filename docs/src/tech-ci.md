---
title: CI/CD workflows
lead: The five GitHub Actions workflows - what triggers them, what each job does, what they need, and how to watch them.
description: Documentation of shint's GitHub Actions - lint, vulnerability check, binary build and release, Docker Hub and GHCR publishing - plus the GitHub-managed automation around them.
section: Technical
order: 5
nav: CI/CD workflows
---

## Overview

All five workflows are triggered by **the same event: pushing a tag that matches `v*.*.*`**. Nothing runs on a push to `main` or on a pull request. That is a deliberate choice - a release is the unit of quality here - and it has consequences covered under [What CI does not do](#what-ci-does-not-do).

They are separate files so that each has its own status and its own failure mode: a Docker Hub credential problem shows up as *that* workflow failing, not as a vague failure of "the release".

:::html
<div class="diagram">
<svg viewBox="0 0 700 300" role="img" aria-label="Workflow graph: one tag push starts five workflows">
  <defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z"/></marker></defs>
  <rect class="box accent" x="10" y="122" width="130" height="56" rx="10"/>
  <text class="h" x="75" y="146" text-anchor="middle">git push tag</text>
  <text class="sub" x="75" y="164" text-anchor="middle">v4.0.3</text>
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

- **Trigger:** `on: push: tags: ['v*.*.*']`.
- **Go toolchain:** `actions/setup-go@v5` with `go-version-file: go.mod` and `check-latest: true`, so CI builds with the version the module declares (or the newest patch of it).
- **Checkout:** `actions/checkout@v5`.
- **Linter:** `golangci-lint` **pinned to `v2.13.2`** through `golangci/golangci-lint-action@v7`. It is pinned because `latest` once resolved to a build made with an older Go than the module targets and failed for that reason alone; and action v6 does not support golangci-lint v2.
- **Vulnerabilities:** `golang/govulncheck-action@v1`.
- **Least privilege:** every job declares `permissions` explicitly (`contents: read` by default; `contents: write` only to create the release; `issues: write` only to file failure issues; `packages: write` only to push to GHCR).

## Lint

`.github/workflows/lint.yaml`

| Job | What it does |
|---|---|
| `lint` | Runs `golangci-lint` with the default linter set. |
| `report-failure` | Runs only if `lint` failed: files an issue titled "CI Failure: Lint" (labels `bug`, `ci-failure`, assigned to the person who pushed the tag) from `.github/ISSUE_TEMPLATE/ci_failure.md`, using `peter-evans/create-issue-from-file@v5`. |

## Vulnerability check

`.github/workflows/vulncheck.yaml`

| Job | What it does |
|---|---|
| `govulncheck` | Runs `govulncheck`, writes its report to a file, and turns it into a step summary table (status, vulnerability count, checked modules, details). |
| `report-failure` | On failure, files an issue "CI Failure: Vulnerability Check" like the lint workflow does. |

## Binary build and release

`.github/workflows/build.yaml`

| Job | Needs | What it does |
|---|---|---|
| `gate` | - | Re-runs `golangci-lint` and `govulncheck` **quietly**. It does not report (the standalone workflows do); it exists so a failing check stops this workflow from shipping binaries. |
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
| `gate` | The same quiet lint + vulnerability gate. |
| QEMU and Buildx | `docker/setup-qemu-action@v3`, `docker/setup-buildx-action@v3`, so one job builds for two architectures. |
| Login | Docker Hub uses the repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`. GHCR uses the workflow's own `GITHUB_TOKEN` (needs `packages: write`). |
| Compute tags | From the tag `v4.0.3`: `v4.0.3`, `4.0.3`, `4.0`, `4`, and `latest`. |
| Build and push | `docker/build-push-action@v6`, platforms `linux/amd64,linux/arm64`, build argument `VERSION=<tag>`, and OCI labels for version, revision and source. |

Image names: `docker.io/<DOCKERHUB_USERNAME>/shint` and `ghcr.io/dmartsapp/shint`. The Dockerfile is described in the [Source reference](tech-source.md#build-and-packaging).

## Automation GitHub manages

Three more things run that are **not defined by files in this repository**; they are switched on in the repository settings:

| Name | What it is |
|---|---|
| CodeQL | GitHub's "default setup" code scanning; it appears in the Actions list as a dynamic workflow on pushes to `main`. |
| pages-build-deployment | Publishes the [documentation site](tech-docs.md) from `main` after a push. |
| Dependabot | Opens pull requests to update Go module dependencies (branches named `dependabot/go_modules/...`). |

## Watching a release

Push the tag, then follow it from the terminal with the GitHub CLI:

```bash
gh run list --limit 8                       # the five workflows for the tag
gh run watch <run-id> --exit-status         # block until one finishes
gh run view <run-id> --log-failed           # only the failing steps' logs
gh run rerun <run-id> --failed              # retry just the failed jobs
gh release view v4.0.3                      # the release and its assets
```

Typical durations: lint and vulnerability check under a minute, the binary release about two minutes, the two image builds four to six.

## Reproducing the checks locally

```bash
go vet ./...
go test -race ./...
golangci-lint run ./...          # use v2.13.2, as CI does
govulncheck ./...
```

## What CI does not do

- **It does not run the test suite.** The workflows lint, check vulnerabilities and build; none runs `go test`. Tests are the release checklist's job ([Releases and tagging](tech-release.md#release-checklist)) and must pass locally before a tag is pushed.
- **It does not run on branches or pull requests**, so problems surface at tag time. Run the local checks above first.
- **It cannot un-publish.** A tag push that fails halfway can leave a partial release or images; see the recovery notes in [Releases and tagging](tech-release.md#when-a-release-goes-wrong).
