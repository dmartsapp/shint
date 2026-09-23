---
title: Releases and tagging
lead: The philosophy behind shint's versions and tags, how a release is cut step by step, and how the version string gets into every kind of build.
description: Semantic versioning, tag format, branch conventions, the release checklist, commit-message conventions and recovery procedures for shint releases.
section: Technical
order: 6
nav: Releases and tagging
---

## Philosophy

A release is one specific thing: a `vX.Y.Z` tag. An ordinary merge to `main` is not a release by itself - see [Merging a branch to main](tech-ci.md#merging-a-branch-to-main-a-practical-checklist) for that everyday case; everything below is about the tag.

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

**One release branch per release: `release/vX.Y.Z`.** It is created from `main` when the release's sprint starts (for example `release/v4.3.0`), pushed at once, and all of the sprint's work lands on it, ending with the release commit (version bump, changelog). Never name a branch like a tag: a branch and a tag both called `v4.0.1` make `git` warn that the name is ambiguous. The older version-named branches on the remote (`v3`, `v3.1`, `v4.0.0`, `v4.0.1`) are history and are left alone.

**Cadence: one release every two weeks, one at a time.** A sprint is two weeks, Monday to Sunday, and the next release's branch is not started until the previous release has shipped. The roadmap in `main`'s README lists the planned releases and their sprint windows; it is kept up to date on `main` (see below), so it never disagrees with what shipped.

**Two kinds of README.** A README is scoped to the branch it is on:

| Branch | What `readme.md` is |
|---|---|
| a release branch (`release/vX.Y.Z`) | A working page for **that branch alone**: the release's milestone targets, the bugs it fixes, and the changes and diffs that exist only there. It is updated as the branch moves and is **never merged into `main`**. |
| `main` | The project README: **every milestone in one place** (the roadmap), the releases, and the install and usage overview. It shows the latest release dynamically - the release badge and the download link point at "latest" - so a release does not need an edit to it. |

`main`'s README is never synced from a branch: a change to it is made on `main`, as a commit of its own (a README-only commit, made before or after a release, never inside it). After a release it is [reconciled automatically](#readme-after-a-release). The consequence for a release is mechanical: the release branch's `readme.md` is put back to `main`'s before the merge, so the fast-forward changes nothing in `main`'s README, and `make release-check` refuses to pass otherwise. If `main`'s README changed while the branch was in flight, bring `main` into the branch (a merge, which rewrites nothing, or a rebase) and, on a conflict in `readme.md`, keep the branch's own page - the release commit restores `main`'s anyway.

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
- **The release commit's body is the changelog** for that version, plus how it was verified. The GitHub release page shows the changelog's section for the tag under "What's new" (`.github/scripts/changelog-section.sh`; the tagged commit's message is only the fallback for a tag with no section), so the page is right even when the tag is on a later commit than the release commit.
- Bodies explain *why*, especially for behaviour changes, and name the tests that pin the behaviour.

## How the version string gets into the binary

`main.Version` is a string variable with a default in the source, overridden at link time. Each build path sets it differently:

| Build | Version reported by `shint --version` | Set by |
|---|---|---|
| `go build` from a clone | `4.2.1` | The default in `main.go` (bump it in the release commit) |
| `make <target>` | `<tag-or-dev>-<commit date as ddmmyyyyHHMMSS>` | `Makefile` (`git tag --contains`, `git show --format=%cd`) |
| Release binary (CI) | `v4.2.1/<full commit sha>/<UTC build time>` | `build.yaml` (`-X main.Version=${{ github.ref_name }}/${{ github.sha }}/$DT`) |
| Docker image | `v4.2.1` | `Dockerfile` (`ARG VERSION`, passed as `VERSION=<tag>`; `dev` when built locally) |

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
5. **Leave `main`'s README alone.** Whether the roadmap now shows this release as shipped, or the later windows must shift because the sprint slipped, is a README-only commit **on `main`**, not part of the branch - and it is proposed for you after the tag ([README after a release](#readme-after-a-release)). Keep the branch's own README (targets, bugs, changes) up to date as you go.
6. **Run every check locally.** CI will not run the tests for you. The Makefile is the one place they are defined:

```bash
make test-full
```

Or `make attest` instead of `make check`, on the commit that will be merged: it runs `make check`, signs a report of it and attaches it to the commit as a git note, so the Check on `main` after the merge runs a one-minute subset instead of the whole suite ([the signed local check report](tech-ci.md#the-signed-local-check-report)). `make test-full` is `make check` - `gofmt`, `go vet`, `go test -race`, the black-box battery, `golangci-lint` (v2.13.2, as CI uses), `govulncheck`, the documentation tests and check (the site is current; every link, anchor and site URL resolves), and the workflow checks - followed by `make test-live`, the smoke test against real hosts, which needs the internet. See [Testing](tech-testing.md#running-the-tests).

7. **Commit the release** as the last commit on the branch, with the message convention above; the body is the changelog. The same commit puts the README back to `main`'s (`git checkout origin/main -- readme.md`). Push the branch, then run **`make release-check`**: it confirms the branch is on top of `main`, `readme.md` matches `main`'s, the version, dated changelog and release-commit message agree, there are no attribution trailers, and the tag is free.

**On release day** (the last day of the sprint window, or earlier for an urgent fix)

8. **Rebase if `main` moved, merge, tag, push** - `main` first, then the tag, as two separate pushes, never `git push --follow-tags` (re-run `make release-check` after a rebase):

```bash
git fetch origin && git rebase origin/main        # on the release branch, only if main moved
git checkout main && git merge --ff-only release/vX.Y.Z
git push origin main                              # first push: the commit
                                                   # wait here for Check to go green (next step) -
                                                   # this is why it is two pushes, not one --follow-tags
git tag -a vX.Y.Z -m "vX.Y.Z: summary" -m "<changelog>"
git push origin refs/tags/vX.Y.Z                  # second push: the tag. This starts the release
                                                   # pipeline - binaries, images, the GitHub Release -
                                                   # so it happens only once you have seen Check pass.
```

**Why not `git push --follow-tags`?** That single command would push the commit and the tag together, before anything has checked the commit that is about to be tagged. The tag push is irreversible in effect - once the pipeline runs, binaries are downloaded and images are pulled - so the gap between the two pushes is deliberate: it is where you watch [Check](tech-ci.md#check-after-a-merge-to-main) pass on `main` before minting something permanent. `make attest`, run before the merge, makes that wait short (`check-quick`, about a minute) instead of long (the whole `make check`, about three and a half).

9. **Watch the five workflows** - the result is posted to Slack when the last one finishes ([Slack notification](tech-ci.md#slack-notification)), or follow them with the [commands here](tech-ci.md#watching-a-release) - and confirm the release has 28 assets (14 binaries and their 14 `.sha256` files). (Before tagging, let the [Check](tech-ci.md#check-after-a-merge-to-main) run for the merge to `main` finish green.)
10. **Verify**: download a binary and its `.sha256`, check it (`shasum -a 256 -c`), run `gh attestation verify <file> --repo dmartsapp/shint`, and run `--version`; pull the image.

### Command by command

Everything above, top to bottom, nothing skipped. `vX.Y.Z` is the version - replace it everywhere it appears. This assumes a release branch (`release/vX.Y.Z`); for an ordinary merge to `main` that is not a release, see [Merging a branch to main](tech-ci.md#merging-a-branch-to-main-a-practical-checklist) instead - stop after "push origin main" below and skip everything from "Release day" on.

**Start the branch**, at the start of the sprint:

```bash
git switch -c release/vX.Y.Z main
git push -u origin release/vX.Y.Z
```

**Do the work.** Commits, features, fixes - whatever the sprint calls for. Nothing below happens until it is done.

**Finish the release**, on the branch:

```bash
# version and changelog
$EDITOR main.go               # bump Version to X.Y.Z
$EDITOR CHANGELOG.md          # finish and date the ## vX.Y.Z section
python3 docs/build.py         # rebuild the site so it matches

# every check, locally - CI does not run these for you
make attest                   # runs make check, then signs and attaches a report (recommended)
# or, without a report:
# make test-full               # make check + the live smoke test against real hosts

# the release commit: message body is the changelog; the README goes back to main's
git checkout origin/main -- readme.md
git add -A
git commit                    # "<type>: <summary> (vX.Y.Z)", body = the changelog, no attribution trailers
git push origin release/vX.Y.Z

# preflight: on top of main, readme.md matches main's, version/changelog/message agree, tag is free
make release-check
```

**Release day** - merge to `main`, wait for Check, only then tag:

```bash
git fetch origin && git rebase origin/main   # only if main moved since you branched; re-run make release-check after
git checkout main
git merge --ff-only release/vX.Y.Z
git push origin main
```

Wait here. Watch it with `gh run list --branch main --limit 3` or `gh run watch <id>`, until `Check` is green. Then, and only then:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z: <one-line summary>" -m "<changelog>"
git push origin refs/tags/vX.Y.Z
```

**Watch the release:**

```bash
gh run list --branch vX.Y.Z --limit 8   # the five release workflows and the Slack notifier
gh run watch <id>                       # or wait for the Slack message - see Watching a release
```

**Verify:**

```bash
gh release view vX.Y.Z                                          # 36 assets: 18 binaries, 18 .sha256 files
shasum -a 256 -c shint.darwin.arm64.sha256                      # or sha256sum -c on Linux
gh attestation verify shint.darwin.arm64 --repo dmartsapp/shint
./shint.darwin.arm64 --version
```

**Afterwards, unattended:** `README Reconcile` runs on the tag and either commits straight to `main` (a clean reconcile) or opens a pull request - or, where Actions cannot open one, an issue that links a branch - for you to read and merge. See [README after a release](#readme-after-a-release).

## README after a release

## README after a release

`main`'s README is written by hand, and a hand-kept README drifts: after v4.2.0 it still said v4.0.4 was the newest release, and a donate line that had been queued on a branch never landed. So it is reconciled by a script, `.github/scripts/readme-reconcile.py`, which reads the README as it is and changes only what the release facts say it should:

| What | Where it comes from | What it does to the README |
|---|---|---|
| Released, and on which day | The release **tags** in git (the date in the tagger's own timezone) | A planned row becomes `Released Sep 21`; a tag with no row gets one, in version order; a wrong date is corrected |
| The text of a new row | The **changelog** section's opening sentence (else its first bullet) | Used only for a row that does not exist yet - a hand-written row keeps its wording |
| The sprint window of a coming release | The GitHub **milestones** (their due date, two weeks back) | `Nov 2 - Nov 15` follows the milestone |
| A command the Commands table lacks | The binary's own `--help` | A row with the command's short description, for a human to complete |
| What every README must have | The script | The tagline's expansion, the badge row **with the donate button**, and the support line at the bottom are put back if missing |

It **warns and never guesses** about what it cannot know, and the warnings are what a person reads: a row that says Released but has no tag, a release still planned although a later one shipped, a release that came out ahead of its sprint window, and a row whose text was written as a plan (with the changelog's summary beside it, in case something planned moved to a later release). It is idempotent - a second run changes nothing - and it fails closed: a README without the Roadmap table, or with a different shape, is refused rather than guessed at.

**By hand**, on any machine with the repository:

```bash
make readme-reconcile TAG=v4.2.1          # a branch readme/main-v4.2.1 off origin/main, one commit, nothing pushed
git diff origin/main                       # read it, and the warnings the command printed
make readme-reconcile TAG=v4.2.1 PUSH=1   # ... or in one go: push it and open the pull request
```

**Automatically**, the `README Reconcile` workflow ([CI/CD](tech-ci.md#readme-reconcile)) does this after every release tag. The workflow does not commit to `main` (the script can, for a clean reconcile only, when asked - see below - and the workflow does not ask): it pushes `readme/main-vX.Y.Z` and opens a pull request (or, where the repository does not let Actions open pull requests, an issue that links the branch). Read the diff and the warnings, fix the wording they point at, and merge. `TAG` only names the branch: every release the README does not yet show is reconciled, so if two releases come out close together the newer proposal contains the older.

**When it may commit by itself.** Only a **clean** reconcile may go straight to `main` (`AUTO_COMMIT=1`), and clean means all of: the script has **no warnings** (nothing for a person to look at), the commit changes **`readme.md` and nothing else**, the README **link check** (`docs/build.py --check`) passes, and the push is a **fast-forward** (`main` did not move meanwhile). The commit carries `[skip ci]`: a README needs no test run. If any of the four fails, the script says which and takes the branch-and-pull-request path instead. In practice a patch release that adds a row is clean, and a minor release that flips a planned row is not (the row's text was written as a plan, and the script always asks a person to compare it with the changelog). Nothing enables it yet: it belongs to the end of a release that has *passed*, not to a tag that has only just been pushed, because the README must not point at a release that is not live.

## When a release goes wrong

- **Never move or delete a published tag.** Binaries and images built from it are already out there; a moved tag makes the source disagree with what people have.
- **A bug in a published release:** fix it, and publish the next patch version.
- **A workflow failed partway** (a flaky registry login, say): re-run the failed jobs (`gh run rerun <run-id> --failed`). Only if the *content* was wrong does it need a new version.
- **The guard refused a tag** (`verify-tag` is red): nothing was built or published. If the tag was pushed before `main`, push `main` and re-run the failed workflows (`gh run rerun <run-id> --failed`) - the check is made live. If the tag has the wrong name or is on the wrong commit, that is the next case.
- **A tag pushed by mistake before it was ready:** if nothing has been downloaded, remove the release and images first, then the tag, and say so. This is the one case for deleting a tag, and it should be a deliberate, announced act.

## The module path

Since v4.2.0 `go.mod` declares `module github.com/dmartsapp/shint/v4`. Go requires the major version in the module path for v2 and above, so before that - with the path `github.com/dmartsapp/shint` and v4 tags - `go install github.com/dmartsapp/shint@v4.x` could never work. From v4.2.0, `go install github.com/dmartsapp/shint/v4@latest` (or `@v4.2.0`) does. The older tags, v4.0.0 to v4.0.6, keep the old path: a published tag is never changed, so they cannot be `go install`ed and never will be.

Changing the path was a breaking change for anyone importing the packages (`lib` and `lib/handlers`). Nobody does - shint is distributed as binaries and images - so it was judged safe (the decision is on the issue that tracked it). Every import inside the repository uses the new path. The next major version would need the next suffix, `/v5`.
