---
title: Releases and tagging
lead: The philosophy behind shint's versions and tags, how a release is cut step by step, and how the version string gets into every kind of build.
description: Semantic versioning, tag format, branch conventions, the release checklist, commit-message conventions and recovery procedures for shint releases.
section: Technical
order: 6
nav: Releases and tagging
---

## Philosophy

**A tag is a release, and a release is forever.** Pushing a tag matching `v*.*.*` starts the entire release pipeline ([CI/CD workflows](tech-ci.md)): binaries are built and attached to a GitHub Release, and container images are pushed under that version. People download those files and pull those images, so a published tag is treated as **immutable**. It is never moved, deleted or re-pushed. If something is wrong, the answer is a new, higher version.

**Versions describe what users experience**, following [semantic versioning](https://semver.org):

| Bump | When | Examples from this project |
|---|---|---|
| **Major** (`v4.0.0`) | A change in how existing commands behave broadly enough that scripts and habits may notice | v4.0.0 made every command dual-stack (IPv4 *and* IPv6) - the same command now tests more addresses |
| **Minor** (`v3.1.0`) | A new capability, existing behaviour intact | v3.1.0 added `listen http` |
| **Patch** (`v4.0.2`) | Fixes, measurement corrections, documentation | v4.0.1 per-request listener metrics; v4.0.2 wire-accurate byte counts; v4.0.3 timeout fixes, exit status, progress output, this documentation |

When a patch changes something a script could observe (v4.0.3's exit status is the example), the changelog says so plainly and in bold, rather than the version pretending it did not happen.

**The tag message and the release commit carry the story.** A tag is annotated, with a message of the form `vX.Y.Z: one-line summary` followed by the changelog. The commit the tag points at has a message that *is* the changelog, because the release workflow copies it into the GitHub release notes.

## Tag format

- `v` + `MAJOR.MINOR.PATCH`: `v4.0.3`. The workflows match `v*.*.*`.
- **Annotated** (`git tag -a`), never lightweight, so a tag has an author, a date and a message.
- The tag points at the commit that contains the version bump and the changelog - normally the tip of `main`.

:::note History you will see in the tag list
Tags up to `v2.2.6` predate the current process, and the list contains a run of letter-suffixed tags (`v2.2.2a`, `v2.2.4ae`, ...) that appear to have been created while the CI pipeline was being developed. They are historical; the current rule is one tag per real release.
:::

## Branches

`main` always contains the latest release. Release preparation has used a branch named after the version (`v3`, `v3.1`, `v4.0.0`, `v4.0.1` exist on the remote) which is then merged into `main`. History on `main` is linear: a release is merged with `--ff-only`, so the tag, the branch tip and `main` all point at the same commit.

Because CI triggers only on tags (see [What CI does not do](tech-ci.md#what-ci-does-not-do)), pushing `main` is safe and starts nothing.

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
| `go build` from a clone | `4.0.3` | The default in `main.go` (bump it in the release commit) |
| `make <target>` | `<tag-or-dev>-<commit date as ddmmyyyyHHMMSS>` | `Makefile` (`git tag --contains`, `git show --format=%cd`) |
| Release binary (CI) | `v4.0.3/<full commit sha>/<UTC build time>` | `build.yaml` (`-X main.Version=${{ github.ref_name }}/${{ github.sha }}/$DT`) |
| Docker image | `v4.0.3` | `Dockerfile` (`ARG VERSION`, passed as `VERSION=<tag>`; `dev` when built locally) |

So a release binary is traceable to an exact commit and moment, and a source build tells you which release it descends from.

## Release checklist

1. **Decide the version** using the table above.
2. **Update the version constant** in `main.go` and the Docker tag example in the docs if needed.
3. **Update `CHANGELOG.md`**: add a `## vX.Y.Z` section, newest first. Rebuild the site (`python3 docs/build.py`) so the [Changelog](changelog.md) page matches.
4. **Run every check locally.** CI will not run the tests for you:

```bash
gofmt -l .                       # should list nothing new
go vet ./...
go test -race ./...
golangci-lint run ./...          # v2.13.2, as CI uses
govulncheck ./...
bash basic_module_test.sh        # optional, needs the internet
python3 docs/build.py --check    # the site is current and its links resolve
```

5. **Commit** with the message convention above; the body is the changelog.
6. **Tag** the commit, annotated, with the changelog in the message:

```bash
git tag -a v4.0.3 -m "v4.0.3: summary" -m "<changelog>"
```

7. **Merge to `main`** (fast-forward) and **push** - `main` first, then the tag, which starts the pipeline:

```bash
git checkout main && git merge --ff-only <branch>
git push origin main
git push origin refs/tags/v4.0.3
```

8. **Watch the five workflows** ([commands here](tech-ci.md#watching-a-release)) and confirm the release has 14 assets.
9. **Verify**: download a binary and run `--version`; pull the image.

## When a release goes wrong

- **Never move or delete a published tag.** Binaries and images built from it are already out there; a moved tag makes the source disagree with what people have.
- **A bug in a published release:** fix it, and publish the next patch version.
- **A workflow failed partway** (a flaky registry login, say): re-run the failed jobs (`gh run rerun <run-id> --failed`). Only if the *content* was wrong does it need a new version.
- **A tag pushed by mistake before it was ready:** if nothing has been downloaded, remove the release and images first, then the tag, and say so. This is the one case for deleting a tag, and it should be a deliberate, announced act.

## The module path

`go.mod` declares `module github.com/dmartsapp/shint` with no `/v4` suffix, although the tags are v4. Go requires the major version in the module path for v2 and above, so `go install github.com/dmartsapp/shint@v4.x` does **not** work. The supported ways to get shint are release binaries, the Docker images, and building from a clone; the [Install page](install.md) says so. Changing the path would be a breaking change for anyone importing the packages, so it has been left as is.
