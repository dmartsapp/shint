---
title: Releases and tagging
lead: The philosophy behind shint's versions and tags, how a release is made with two commands, and how the version string gets into every kind of build.
description: Semantic versioning, tag format, branch conventions, make release-start and make release, commit-message conventions and recovery procedures for shint releases.
section: Technical
order: 6
nav: Releases and tagging
---

## Philosophy

A release is one specific thing: a `vX.Y.Z` tag. An ordinary merge to `main` is not a release by itself - see [Merging a branch to main](tech-ci.md#merging-a-branch-to-main-a-practical-checklist) for that everyday case; everything below is about the tag. **To make a release, follow [Making a release](#making-a-release)**: two commands, step by step.

**A tag is a release, and a release is forever.** Pushing a tag of the form `vX.Y.Z`, on a commit that is on `main`, starts the entire release pipeline ([CI/CD workflows](tech-ci.md)): binaries are built and attached to a GitHub Release, and container images are pushed under that version. People download those files and pull those images, so a published tag is treated as **immutable**. It is never moved, deleted or re-pushed. If something is wrong, the answer is a new, higher version.

**Versions describe what users experience**, following [semantic versioning](https://semver.org):

| Bump | When | Examples from this project |
|---|---|---|
| **Major** (`v4.0.0`) | A change in how existing commands behave broadly enough that scripts and habits may notice | v4.0.0 made every command dual-stack (IPv4 *and* IPv6) - the same command now tests more addresses |
| **Minor** (`v3.1.0`) | A new capability, existing behaviour intact | v3.1.0 added `listen http` |
| **Patch** (`v4.0.2`) | Fixes, measurement corrections, documentation | v4.0.1 per-request listener metrics; v4.0.2 wire-accurate byte counts; v4.0.3 timeout fixes, exit status, progress output, this documentation; v4.0.4 `ping` timeout and payload size, the tag-only release pipeline |

When a patch changes something a script could observe (v4.0.3's exit status is the example), the changelog says so plainly and in bold, rather than the version pretending it did not happen.

**The tag message and the release commit carry the story.** A tag is annotated, with a message of the form `vX.Y.Z: one-line summary` followed by the changelog, and the commit it points at carries the changelog too. `make release` writes both from `CHANGELOG.md`; the GitHub release notes are the changelog's section for the tag.

## Making a release

A release takes **two commands**: `make release-start` when the sprint begins, and `make release` on release day. Everything that used to be typed by hand - the version, the date, the order of the steps - is done by them, and the second one can always be run again after it stops. The steps below use **v4.4.0** as the example; put your version in its place.

### Before your first release

Once per machine, check that you have what `make release` uses:

```bash
gh auth status          # logged in to GitHub as a maintainer of dmartsapp/shint
make check              # the local toolchain is complete: golangci-lint, govulncheck, actionlint
```

and that your SSH public key is in `.github/allowed_signers`: `make attest` signs the check report with it ([the signed local check report](tech-ci.md#the-signed-local-check-report)), and may ask for the key's passphrase.

### Step 1 - start the release branch (the first day of the sprint)

```bash
git switch main && git pull --ff-only
make release-start VERSION=4.4.0
```

You are now on `release/v4.4.0`, already pushed. It has two new things: a `## v4.4.0 - unreleased` heading at the top of `CHANGELOG.md`, and the working page [`releases/v4.4.0.md`](https://github.com/dmartsapp/shint/tree/main/releases), made from a template with the milestone's link and due date.

### Step 2 - do the work (during the sprint)

On `release/v4.4.0`, for every change:

1. **Commit and push it** (`git push`). A push to a release branch starts no workflow, so push as often as you like.
2. **Add its changelog entry** under `## v4.4.0 - unreleased` in `CHANGELOG.md`: one bullet, its first sentence in bold, saying what changed for users (and, in bold, if it is something a script could notice). Leave the heading as `unreleased` - the date is stamped on release day, never typed.
3. **Keep the working page current**: `releases/v4.4.0.md`'s targets, bugs and other changes, with their issues and commits.

Before release day, fill in the working page's **`Summary:`** line - one plain-language sentence about the release, which becomes its row in the roadmap of `main`'s README - and **`Includes:`** if another release is folded into this one (for example `Includes: v4.5.0`). If `main` gets commits of its own meanwhile, do nothing: `make release` merges them in.

### Step 3 - rehearse (optional, the day before)

```bash
make release DRY_RUN=1
```

It runs everything up to the question in a throw-away clone, with pushing disabled, and prints what would be published. Read it; the last lines say where the clone is, to look at or delete.

### Step 4 - release (the last day of the sprint window)

```bash
git switch release/v4.4.0 && git pull --ff-only
make release
```

What you will see, in order:

1. **`==> main`, `==> stamp`, `==> the release commit`** - `main` merged in if it moved; the version, today's date and the README row stamped; the release commit made. Seconds.
2. **`==> checks on the release commit`** - `make attest` (the whole `make check`, signed), `make test-live` and `make release-check`. A few minutes; this is where it may ask for your SSH key's passphrase.
3. **`==> ready to publish v4.4.0`** - the commit, the date, the summary, the README row, the changelog's entries, and any issues still open on the milestone. Then the question:

   ```text
   Publish v4.4.0? This pushes main and then the tag, which starts the release pipeline and cannot be taken back. [y/N]
   ```

   Answer **`y`** to go on. Anything else stops with nothing pushed.
4. **`==> main`, `==> Check on main`** - `main` pushed, then the wait for Check on that commit: about a minute with the signed report.
5. **`==> the tag`** - the tag pushed. The release pipeline starts now.
6. **`==> the release pipeline`** - the workflows watched to the end (about 10 to 15 minutes), the assets counted, this machine's binary downloaded and its checksum, attestation and `--version` checked, the milestone closed.
7. **`==> v4.4.0 is released`** - done.

### Step 5 - if it stops

It prints `release: STOPPED - ` and the reason. Fix what it says and run `make release` again: it recognises what is already done and carries on.

| It stopped at | What it means | What to do |
|---|---|---|
| `the working tree has uncommitted changes` | Something is not committed | Commit it (or `git stash`), then `make release` |
| `has no 'Summary:' line` | The working page has no summary | Write the `Summary:` line in `releases/v4.4.0.md`, commit, `make release` |
| `has no entries yet`, or `says '...', not 'unreleased'` | The changelog section is empty, or its heading was dated by hand | Add the entries / put the heading back to `## v4.4.0 - unreleased`, commit, `make release` |
| `does not merge cleanly` | `main` moved and conflicts with the branch | `git merge origin/main`, resolve, `git commit`, `make release` |
| `failed. The release commit was taken back off` | `make attest`, `make test-live` or `make release-check` failed | Nothing was pushed and the branch is as it was. Fix, commit, `make release` |
| You answered anything but `y` | - | Nothing was pushed. `make release` again when ready (it asks again), or take the release commit off with the `git reset` it printed |
| `Check on main ended 'failure'` | `main` has the release commit, but no tag exists | A flake: `gh run rerun <run-id>` (the id is in the message), then `make release`. A real problem: fix it forward on `main`, and tag the fix by hand ([When a release goes wrong](#when-a-release-goes-wrong)) |
| `is tagged, but something above failed` | The release is public; a workflow, the asset count or the binary check failed | Re-run a failed workflow (`gh run rerun <run-id> --failed`), then `make release` to check again. If the content is wrong, the answer is the next patch release |

### Step 6 - afterwards

Nothing. `main`'s README already shows the release, `releases/v4.4.0.md` is on `main` as its record, and the milestone is closed. The release branch can stay: it is history. The next sprint starts again at step 1.

### What `make release` does, stage by stage

| Stage | What happens | Reversible? |
|---|---|---|
| Preflight | A clean tree; the tag is free; the changelog section has entries and still says `unreleased`; the working page has a `Summary:`. The milestone's open issues are listed. | - |
| Main | If `origin/main` moved since the branch was cut, it is merged in. A conflict stops here. | yes |
| Stamp | The version from the branch name and **today's date from the clock** go into `main.go`, the changelog heading and the working page's title; the release's roadmap row goes into `readme.md` (`readme-reconcile.py --summary`); the site is rebuilt (`docs/build.py`). | yes |
| Release commit | Its message is generated from the changelog (see [Commit messages](#commit-messages)). | yes |
| Checks | [`make attest`](tech-ci.md#the-signed-local-check-report) - the whole `make check`, signed and attached to **this exact commit** - then `make test-live`, then `make release-check`. A failure takes the release commit back off the branch. | yes |
| **Publish v4.4.0? [y/N]** | The summary of what is about to go out. The one question. | - |
| Main | `main` is fast-forwarded to the release commit and pushed; then the wait for [Check](tech-ci.md#check-after-a-merge-to-main) on that commit. A red Check stops here: nothing is tagged. | a commit on `main` stays, but nothing is published |
| Tag | The annotated tag - `vX.Y.Z: <Summary>` and the changelog - is pushed. This starts the release pipeline. | **no** |
| Verify | The release workflows watched to the end; the assets counted against the previous release's; this machine's binary downloaded and its checksum, [attestation](install.md) and `--version` checked; the milestone closed if it has no open issues. | - |

**Main first, then the tag, with a wait in between.** Pushing the commit and the tag together (`git push --follow-tags`) would start the irreversible part before anything outside this machine has checked the commit. The wait for Check is the gap where that happens; `make attest` makes it about a minute (`check-quick`) instead of the whole suite. The script does the waiting, so the order cannot be got wrong.

**Options:** `NO_ATTEST=1` uses `make check` instead of `make attest` (Check on `main` then runs the whole suite, about three and a half minutes); `SKIP_LIVE=1` leaves out the live smoke test; `RELEASE_YES=1` answers the question for you - for automation only.

:::note Doing it by hand
`.github/scripts/release.py` is the reference for the order. By hand, the same release is: merge `origin/main` into the branch if it moved; set `Version` in `main.go`; date the changelog heading with today's date; run `python3 .github/scripts/readme-reconcile.py --tags-file <every tag and its date, plus the new one> --tag vX.Y.Z --summary "<Summary>"`; `python3 docs/build.py`; commit with the generated message; `make attest`; `make release-check`; push `main`; wait for Check; `git tag -a vX.Y.Z`; `git push origin refs/tags/vX.Y.Z`.
:::

## Tag format

- `v` + `MAJOR.MINOR.PATCH`, **digits only**: `v4.0.3`. The workflows match `v[0-9]+.[0-9]+.[0-9]+` and nothing else - no `-rc1`, no `+build`, no letter suffixes.
- **Annotated** (`git tag -a`), never lightweight, so a tag has an author, a date and a message.
- The tag points at the commit that contains the version bump and the changelog - the tip of `main` at release time.
- **The commit must be on `main`** when the tag is pushed, so `main` is pushed first. The pipeline's [tag guard](tech-ci.md#the-release-tag-guard) refuses a tag whose commit is on a branch only.

:::note History you will see in the tag list
Tags up to `v2.2.6` predate the current process, and the list contains a run of letter-suffixed tags (`v2.2.2a`, `v2.2.4ae`, ...) that appear to have been created while the CI pipeline was being developed. They are historical; the current rule is one tag per real release. Since v4.0.4 a tag like those - or `v3.0.0-dryrun-test`, which once started a build - would start nothing: the pattern no longer matches it. The tags that are real releases (`v3.0.0`, `v3.1.0`, `v4.0.0` ... `v4.0.3`) all match.
:::

## Branches and cadence

`main` always contains the latest release. A repository ruleset (`protect_mother`) forbids deleting it and forbids non-fast-forward pushes to it, so its history is linear: a release is a fast-forward of `main`, and the tag, the release branch tip and `main` all end up on the same commit. No other branch is restricted.

**One release branch per release: `release/vX.Y.Z`**, created from `main` when the release's sprint starts, pushed at once, and ending with the release commit. Never name a branch like a tag: a branch and a tag both called `v4.0.1` make `git` warn that the name is ambiguous. The older version-named branches on the remote (`v3`, `v3.1`, `v4.0.0`, `v4.0.1`) are history and are left alone.

**Cadence: one release every two weeks, one at a time.** A sprint is two weeks, Monday to Sunday. The roadmap in `main`'s README lists the planned releases and their sprint windows.

**Ready is not published.** Finishing early does not mean shipping early: finished work waits on its branch, and the release happens on the last day of the sprint window. That keeps the cadence predictable for users, and keeps `main` - which the documentation site deploys from, and whose `main.go` gives the site its version - in step with what is actually published. An urgent fix does not have to wait for its window; it is released as soon as it is ready.

**The working page is `releases/vX.Y.Z.md`.** A release branch keeps what it is for - the milestone's targets, the bugs it fixes, what changed - in [`releases/vX.Y.Z.md`](https://github.com/dmartsapp/shint/tree/main/releases), updated as the branch moves. Two lines in it are read by `make release`: **`Summary:`**, one plain-language sentence that becomes the release's row in the roadmap of `main`'s README, and **`Includes:`**, releases folded into this one, if any. The file merges into `main` with the release and stays there as its record. (Before v4.3.0 the working page was the branch's own `readme.md`; those branches still have it.)

**`main`'s README is written on `main`, with one exception.** It is the project README - every milestone (the roadmap), the releases, the install and usage overview - and it shows the latest release dynamically (the release badge and the download link point at "latest"). A release branch never changes it, except that **the release commit writes the release's own roadmap row** (from `Summary:`), so the README is right the moment `main` has the release. Anything else in it is a README-only commit on `main`.

What each kind of push starts (see [CI/CD workflows](tech-ci.md)):

| Push | Starts |
|---|---|
| a `release/**` branch | nothing |
| `main` (a merge) | the documentation site deploy, CodeQL and the [Check](tech-ci.md#check-after-a-merge-to-main) workflow (`make check`) - but no release workflow. Nothing at all if the push only changes `.github/`, apart from the site deploy and CodeQL, which GitHub manages |
| a `vX.Y.Z` tag on a commit on `main` | the whole release pipeline: lint, vulnerability check, binaries, GitHub Release, both Docker images |
| a tag of any other form, or a `vX.Y.Z` tag on a commit that is not on `main` | the workflows start, the [guard](tech-ci.md#the-release-tag-guard) refuses, and nothing is built or published |

## Commit messages

```plain
<type>: <what changed and why, in one line>

<body: what a reader needs - the why, and the tests that pin the behaviour>
```

- **Types** seen in the history: `feat:` (new capability), `fix:` (correction), `docs:` (documentation only), and `release:` for the two commits `make release-start` and `make release` write.
- **The release commit is generated**, never typed: `release: <Summary> (vX.Y.Z)`, then the summary, then `Full changelog for vX.Y.Z` and the changelog's section for the version. The GitHub release page shows that section under "What's new" (`.github/scripts/changelog-section.sh`; the tagged commit's message is only the fallback for a tag with no section).
- Bodies explain *why*, especially for behaviour changes, and name the tests that pin the behaviour.

## How the version string gets into the binary

`main.Version` is a string variable with a default in the source, overridden at link time. Each build path sets it differently:

| Build | Version reported by `shint --version` | Set by |
|---|---|---|
| `go build` from a clone | `{{version}}` | The default in `main.go` (`make release` stamps it) |
| `make <target>` | `<tag-or-dev>-<commit date as ddmmyyyyHHMMSS>` | `Makefile` (`git tag --contains`, `git show --format=%cd`) |
| Release binary (CI) | `v{{version}}/<full commit sha>/<UTC build time>` | `build.yaml` (`-X main.Version=${{ github.ref_name }}/${{ github.sha }}/$DT`) |
| Docker image | `v{{version}}` | `Dockerfile` (`ARG VERSION`, passed as `VERSION=<tag>`; `dev` when built locally) |

So a release binary is traceable to an exact commit and moment, and a source build tells you which release it descends from.

**The documentation never hard-codes the current version.** A page that shows it - an install example, a tag list - writes `\{{version}}` (`{{version}}`), `\{{minor}}` (`{{minor}}`) or `\{{major}}` (`{{major}}`), and `docs/build.py` fills them in from `main.go`, so a release edits no page. A backslash keeps one literally: `\\{{version}}`.

## README after a release

`main`'s README is written by hand, and a hand-kept README drifts: after v4.2.0 it still said v4.0.4 was the newest release, and a donate line that had been queued on a branch never landed. `.github/scripts/readme-reconcile.py` reads the README as it is and changes only what the release facts say it should:

| What | Where it comes from | What it does to the README |
|---|---|---|
| Released, and on which day | The release **tags** in git (the date in the tagger's own timezone) | A planned row becomes `Released Sep 21`; a tag with no row gets one, in version order; a wrong date is corrected |
| The text of a release's row | The release's **`Summary:`** (`--summary`, from `make release`); otherwise the **changelog** section's opening sentence (else its first bullet), for a row that does not exist yet | With a summary, the row says it; without one, a hand-written row keeps its wording |
| Releases folded into another | The working page's **`Includes:`** (`--folded`) | Their rows become `Shipped in vX.Y.Z` |
| The sprint window of a coming release | The GitHub **milestones** (their due date, two weeks back) | `Nov 2 - Nov 15` follows the milestone |
| A command the Commands table lacks | The binary's own `--help` | A row with the command's short description, for a human to complete |
| What every README must have | The script | The tagline's expansion, the badge row **with the donate button**, and the support line at the bottom are put back if missing |

It **warns and never guesses** about what it cannot know: a row that says Released but has no tag, a release still planned although a later one shipped, a release that came out ahead of its sprint window, and - without a summary - a row whose text was written as a plan. It is idempotent - a second run changes nothing - and it fails closed: a README without the Roadmap table, or with a different shape, is refused rather than guessed at.

**`make release` runs it in the release commit**, with the new tag and today's date, `--tag`, `--summary` and `--folded`, so `main`'s README has the release's row the moment `main` has the release. For the few minutes between the push of `main` and the end of the release pipeline, the row says Released while the assets are still being built.

**The `README Reconcile` workflow** ([CI/CD](tech-ci.md#readme-reconcile)) still runs after every release tag, as a safety net: after a release made with `make release` it finds the README already right and does nothing. When it does find work - a release made another way - it pushes `readme/main-vX.Y.Z` and opens a pull request (or, where the repository does not let Actions open pull requests, an issue that links the branch). **By hand**, on any machine with the repository:

```bash
make readme-reconcile TAG=v{{version}}          # a branch readme/main-v{{version}} off origin/main, one commit, nothing pushed
git diff origin/main                       # read it, and the warnings the command printed
make readme-reconcile TAG=v{{version}} PUSH=1   # ... or in one go: push it and open the pull request
```

**When it may commit by itself.** Only a **clean** reconcile may go straight to `main` (`AUTO_COMMIT=1`), and clean means all of: the script has **no warnings**, the commit changes **`readme.md` and nothing else**, the README **link check** (`docs/build.py --check`) passes, and the push is a **fast-forward**. The commit carries `[skip ci]`. If any of the four fails, the script says which and takes the branch-and-pull-request path instead. The workflow does not ask for it.

## When a release goes wrong

- **Never move or delete a published tag.** Binaries and images built from it are already out there; a moved tag makes the source disagree with what people have.
- **A bug in a published release:** fix it, and publish the next patch version.
- **`make release` stopped:** read why, fix it, and run `make release` again - it carries on from where it stopped ([Making a release](#making-a-release)). Before the question nothing has left your machine; after it, the stop message says what is already public.
- **A workflow failed partway** (a flaky registry login, say): re-run the failed jobs (`gh run rerun <run-id> --failed`). Only if the *content* was wrong does it need a new version.
- **The guard refused a tag** (`verify-tag` is red): nothing was built or published. If the tag was pushed before `main`, push `main` and re-run the failed workflows (`gh run rerun <run-id> --failed`) - the check is made live. If the tag has the wrong name or is on the wrong commit, that is the next case.
- **A tag pushed by mistake before it was ready:** if nothing has been downloaded, remove the release and images first, then the tag, and say so. This is the one case for deleting a tag, and it should be a deliberate, announced act.

## The module path

Since v4.2.0 `go.mod` declares `module github.com/dmartsapp/shint/v4`. Go requires the major version in the module path for v2 and above, so before that - with the path `github.com/dmartsapp/shint` and v4 tags - `go install github.com/dmartsapp/shint@v4.x` could never work. From v4.2.0, `go install github.com/dmartsapp/shint/v4@latest` (or `@v4.2.0`) does. The older tags, v4.0.0 to v4.0.6, keep the old path: a published tag is never changed, so they cannot be `go install`ed and never will be.

Changing the path was a breaking change for anyone importing the packages (`lib` and `lib/handlers`). Nobody does - shint is distributed as binaries and images - so it was judged safe (the decision is on the issue that tracked it). Every import inside the repository uses the new path. The next major version would need the next suffix, `/v5`.
