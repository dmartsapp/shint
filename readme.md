# shint v4.0.5 - released 2026-09-20

> **This page describes the `release/v4.0.5` branch only**: what the release delivered, the bugs it fixed, and what changed on the branch. It is a record of that release, added after the tag and not part of it. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Release | [v4.0.5](https://github.com/dmartsapp/shint/releases/tag/v4.0.5), tag on [`7659cb7`](https://github.com/dmartsapp/shint/commit/7659cb7), 2026-09-20 |
| Kind | Patch on v4.0.4, one fix - reported the same day from the v4.0.4 build |
| Milestone | [v4.0.5](https://github.com/dmartsapp/shint/milestone/6) - closed |
| Everything that differs from the previous release | [v4.0.4...v4.0.5](https://github.com/dmartsapp/shint/compare/v4.0.4...v4.0.5) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.0.5` section) and `docs/src/` |

## Milestone targets

| Target | Issue | State |
|---|---|---|
| `Ctrl+C` on a repeating `telnet`, `web` or `udp` run shows the summary and how far the run got | [#24](https://github.com/dmartsapp/shint/issues/24) | Done and released |

## Bugs

| Issue | State |
|---|---|
| [#24](https://github.com/dmartsapp/shint/issues/24) `shint web http://host/ --count 100` interrupted with `Ctrl+C` ended with a bare `^C` - no statistics, no `done` line | Fixed in [`1efd88a`](https://github.com/dmartsapp/shint/commit/1efd88a): the run stops, prints `ERROR interrupted attempts_completed=5 attempts_planned=100`, the statistics for the attempts that completed and the `done` line (the one complete document with `--json`). **Exit status 1** (the run was cut short, as for an interrupted `nmap`); a second `Ctrl+C` ends the process at once |

Still open, on purpose: [#25](https://github.com/dmartsapp/shint/issues/25) - `ping` cannot show a summary on `Ctrl+C` because the ping library cannot be cancelled part-way (planned with the go-ping upgrade in v4.4.0).

## Other changes on the branch

- A long `--delay` no longer makes `Ctrl+C` wait, and an attempt still in flight is dropped rather than counted as a failure.
- The version bump, the dated changelog and the docs version examples are in the release commit [`7659cb7`](https://github.com/dmartsapp/shint/commit/7659cb7).

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
