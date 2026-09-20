---
title: Releases and tagging
lead: The philosophy behind shint's versions and tags, how a release is cut step by step, and how the version string gets into every kind of build.
description: Semantic versioning, tag format, branch conventions, the release checklist, commit-message conventions and recovery procedures for shint releases.
section: Technical
order: 6
nav: Releases and tagging
---

## Philosophy

**A tag is a release, and a release is forever.** Pushing a tag of the form `vX.Y.Z`, on a commit that is on `main`, starts the entire release pipeline ([CI/CD workflows](tech-ci.md)): binaries are built and attached to a GitHub Release, and container images are pushed under that version. People download those files and pull those images, so a published tag is treated as **immutable**. It is never moved, deleted or re-pushed. If something is wrong, the answer is a new, higher version.

**Versions describe what users experience**, following [semantic versioning](https://semver.org):

| Bump | When | Examples from this project |
|---|---|---|
| **Major** (`v4.0.0`) | A change in how existing commands behave broadly enough that scripts and habits may notice | v4.0.0 made every command dual-stack (IPv4 *and* IPv6) - the same command now tests more addresses |
| **Minor** (`v3.1.0`) | A new capability, existing behaviour intact | v3.1.0 added `listen http` |
| **Patch** (`v4.0.2`) | Fixes, measurement corrections, documentation | v4.0.1 per-request listener metrics; v4.0.2 wire-accurate byte counts; v4.0.3 timeout fixes, exit status, progress output, this documentation; v4.0.4 `ping` timeout and payload size, the tag-only release pipeline |

When a patch changes something a script could observe (v4.0.3's exit status is the example), the changelog says so plainly and in bold, rather than the version pretending it did not happen.

**The tag message and the release commit carry the story.** A tag is annotated, with a message of the form `vX.Y.Z: one-line summary` followed by the changelog. The commit the tag points at has a message that *is* the changelog, because the release workflow copies it into the GitHub release notes.

## Tag format

- `v` + `MAJOR.MINOR.PATCH`, **digits only**: `v4.0.3`. The workflows match `v[0-9]+.[0-9]+.[0-9]+` and nothing else - no `-rc1`, no `+build`, no letter suffixes.
- **Annotated** (`git tag -a`), never lightweight, so a tag has an author, a date and a message.
- The tag points at the commit that contains the version bump and the changelog - the tip of `main` at release time.
- **The commit must be on `main`** when the tag is pushed, so `main` is pushed first. The pipeline's [tag guard](tech-ci.md#the-release-tag-guard) refuses a tag whose commit is on a branch only.

:::note History you will see in the tag list
Tags up to `v2.2.6` predate the current process, and the list contains a run of letter-suffixed tags (`v2.2.2a`, `v2.2.4ae`, ...) that appear to have been created while the CI pipeline was being developed. They are historical; the current rule is one tag per real release. Since v4.0.4 a tag like those - or `v3.0.0-dryrun-test`, which once started a build - would start nothing: the pattern no longer matches it. The tags that are real releases (`v3.0.0`, `v3.1.0`, `v4.0.0` ... `v4.0.3`) all match.
:::

## Branches and cadence

`main` always contains the latest release. A repository ruleset (`protect_mother`) forbids deleting it and forbids non-fast-forward pushes to it, so its history is linear: a release is merged with `--ff-only`, and the tag, the release branch tip and `main` all end up on the same commit. No other branch is restricted.

**One release branch per release: `release/vX.Y.Z`.** It is created from `main` when the release's sprint starts (for example `release/v4.1.0`), pushed at once, and all of the sprint's work lands on it, ending with the release commit (version bump, changelog, README roadmap). Never name a branch like a tag: a branch and a tag both called `v4.0.1` make `git` warn that the name is ambiguous. The older version-named branches on the remote (`v3`, `v3.1`, `v4.0.0`, `v4.0.1`) are history and are left alone.

**Cadence: one release every two weeks, one at a time.** A sprint is two weeks, Monday to Sunday, and the next release's branch is not started until the previous release has shipped. The README's roadmap lists the planned releases and their sprint windows, and is updated as part of each release so it never disagrees with what shipped.

**Ready is not published.** Finishing early does not mean shipping early: finished work waits on its branch, and the release happens on the last day of the sprint window. That keeps the cadence predictable for users, and keeps `main` - which the documentation site deploys from, and whose `main.go` gives the site its version - in step with what is actually published. An urgent fix does not have to wait for its window; it is released as soon as it is ready.

**On release day:** merge the branch into `main` with `--ff-only`, push `main`, create the annotated tag, push the tag (the [checklist](#release-checklist) has the commands). If `main` has moved since the branch was cut, a fast-forward is impossible and the ruleset would refuse a non-fast-forward push - so rebase the *release branch* on `main` first. Force-pushing a release branch is fine; `main` is never rewritten.

What each kind of push starts (see [CI/CD workflows](tech-ci.md)):

| Push | Starts |
|---|---|
| a `release/**` branch | nothing |
| `main` (a merge) | the documentation site deploy, CodeQL and the [Check](tech-ci.md#check-after-a-merge-to-main) workflow (`make check`) - but no release workflow. Nothing at all if the push only changes `.github/`, apart from the site deploy and CodeQL, which GitHub manages |
| a `vX.Y.Z` tag on a commit on `main` | the whole release pipeline: lint, vulnerability check, binaries, GitHub Release, both Docker images |
| a tag of any other form, or a `vX.Y.Z` tag on a commit that is not on `main` | the workflows start, the [guard](tech-ci.md#the-release-tag-guard) refuses, and nothing is built or published |

## Commit messages

```plain
<type>: <what changed and why, in one line> (vX.Y.Z)

<body: the changelog for this release, and anything a reader needs>
```

- **Types** seen in the history: `feat:` (new capability), `fix:` (correction), `docs:` (documentation only). Release commits also name the version in the subject.
- **The release commit's body is the changelog** for that version, plus how it was verified. It is what appears on the GitHub release page under "Commit Message".
- Bodies explain *why*, especially for behaviour changes, and name the tests that pin the behaviour.

## How the version string gets into the binary

`main.Version` is a string variable with a default in the source, overridden at link time. Each build path sets it differently:

| Build | Version reported by `shint --version` | Set by |
|---|---|---|
| `go build` from a clone | `4.0.6` | The default in `main.go` (bump it in the release commit) |
| `make <target>` | `<tag-or-dev>-<commit date as ddmmyyyyHHMMSS>` | `Makefile` (`git tag --contains`, `git show --format=%cd`) |
| Release binary (CI) | `v4.0.6/<full commit sha>/<UTC build time>` | `build.yaml` (`-X main.Version=${{ github.ref_name }}/${{ github.sha }}/$DT`) |
| Docker image | `v4.0.6` | `Dockerfile` (`ARG VERSION`, passed as `VERSION=<tag>`; `dev` when built locally) |

So a release binary is traceable to an exact commit and moment, and a source build tells you which release it descends from.

## Release checklist

**At the start of the sprint**

1. **Create the release branch** from `main` and push it:

```bash
git switch -c release/vX.Y.Z main
git push -u origin release/vX.Y.Z
```

**When the work is done** (on the release branch)

2. **Decide the version** using the table above.
3. **Update the version constant** in `main.go` and the Docker tag example in the docs if needed.
4. **Update `CHANGELOG.md`**: add or finish the `## vX.Y.Z` section, newest first. Rebuild the site (`python3 docs/build.py`) so the [Changelog](changelog.md) page matches.
5. **Update the README roadmap**: mark the release as shipped, and shift the later windows if the sprint slipped.
6. **Run every check locally.** CI will not run the tests for you. The Makefile is the one place they are defined:

```bash
make test-full
```

That is `make check` - `gofmt`, `go vet`, `go test -race`, the black-box battery, `golangci-lint` (v2.13.2, as CI uses), `govulncheck`, the documentation tests and check (the site is current; every link, anchor and site URL resolves), and the workflow checks - followed by `make test-live`, the smoke test against real hosts, which needs the internet. See [Testing](tech-testing.md#running-the-tests).

7. **Commit the release** as the last commit on the branch, with the message convention above; the body is the changelog. Push the branch.

**On release day** (the last day of the sprint window, or earlier for an urgent fix)

8. **Rebase if `main` moved, merge, tag, push** - `main` first, then the tag, which starts the pipeline:

```bash
git fetch origin && git rebase origin/main        # on the release branch, only if main moved
git checkout main && git merge --ff-only release/vX.Y.Z
git push origin main
git tag -a vX.Y.Z -m "vX.Y.Z: summary" -m "<changelog>"
git push origin refs/tags/vX.Y.Z
```

9. **Watch the five workflows** - the result is posted to Slack when the last one finishes ([Slack notification](tech-ci.md#slack-notification)), or follow them with the [commands here](tech-ci.md#watching-a-release) - and confirm the release has 28 assets (14 binaries and their 14 `.sha256` files). (Before tagging, let the [Check](tech-ci.md#check-after-a-merge-to-main) run for the merge to `main` finish green.)
10. **Verify**: download a binary and its `.sha256`, check it (`shasum -a 256 -c`), run `gh attestation verify <file> --repo dmartsapp/shint`, and run `--version`; pull the image.

## When a release goes wrong

- **Never move or delete a published tag.** Binaries and images built from it are already out there; a moved tag makes the source disagree with what people have.
- **A bug in a published release:** fix it, and publish the next patch version.
- **A workflow failed partway** (a flaky registry login, say): re-run the failed jobs (`gh run rerun <run-id> --failed`). Only if the *content* was wrong does it need a new version.
- **The guard refused a tag** (`verify-tag` is red): nothing was built or published. If the tag was pushed before `main`, push `main` and re-run the failed workflows (`gh run rerun <run-id> --failed`) - the check is made live. If the tag has the wrong name or is on the wrong commit, that is the next case.
- **A tag pushed by mistake before it was ready:** if nothing has been downloaded, remove the release and images first, then the tag, and say so. This is the one case for deleting a tag, and it should be a deliberate, announced act.

## The module path

`go.mod` declares `module github.com/dmartsapp/shint` with no `/v4` suffix, although the tags are v4. Go requires the major version in the module path for v2 and above, so `go install github.com/dmartsapp/shint@v4.x` does **not** work. The supported ways to get shint are release binaries, the Docker images, and building from a clone; the [Install page](install.md) says so. Changing the path would be a breaking change for anyone importing the packages, so it has been left as is.
