# shint v4.0.6 - released 2026-09-21

> **This page describes the `release/v4.0.6` branch only**: what the release delivered, the bugs it fixed, and what changed on the branch. It is a record of that release, added after the tag and not part of it. It is **never merged into `main`**, whose README tracks every milestone and release in one place: **[README on main](https://github.com/dmartsapp/shint/blob/main/readme.md)**.

| | |
|---|---|
| Release | [v4.0.6](https://github.com/dmartsapp/shint/releases/tag/v4.0.6), tag on [`280759e`](https://github.com/dmartsapp/shint/commit/280759e), 2026-09-21 |
| Kind | Patch on v4.0.5, cut for two `fix-immediate` bugs (wrong answers): flagged 06:52 UTC, released 07:45 UTC the same day |
| Milestone | [v4.0.6](https://github.com/dmartsapp/shint/milestone/7) - closed |
| Everything that differs from the previous release | [v4.0.5...v4.0.6](https://github.com/dmartsapp/shint/compare/v4.0.5...v4.0.6) |
| Documentation | [CHANGELOG.md](CHANGELOG.md) (the `v4.0.6` section) and `docs/src/` |

## Milestone targets

| Target | Issue | State |
|---|---|---|
| `web` must not report a response that ended early as a success | [#26](https://github.com/dmartsapp/shint/issues/26) | Done and released |
| `ping` must not report an unreachable host as reachable | [#27](https://github.com/dmartsapp/shint/issues/27) | Done and released (fixed in go-ping v2.0.1) |
| A black-box test battery in `make test`, so bugs like these are found by a test | - | Done and released |
| The result of every release posted to Slack | - | Done and released |

## Bugs

| Issue | State |
|---|---|
| [#26](https://github.com/dmartsapp/shint/issues/26) A response body that ended early (closed short of its `Content-Length`, reset, or stalled until `--timeout`) was logged as `OK response status=200` with exit 0 | Fixed in [`263aba2`](https://github.com/dmartsapp/shint/commit/263aba2): it is a failed attempt - `ERROR response incomplete` with the read error, `success: false` in `--json`, exit 1 |
| [#27](https://github.com/dmartsapp/shint/issues/27) While another ping on the machine was getting replies, a ping to an address nobody answers reported success (9 runs in 10 in the reproduction) - the ping library matched a reply on the sequence number alone | Fixed in go-ping [v2.0.1](https://github.com/dmartsapp/go-ping/releases/tag/v2.0.1) (every echo request gets its own identifier and only its reply is accepted); shint takes it in [`77d554d`](https://github.com/dmartsapp/shint/commit/77d554d). It also happened between two addresses of one host pinged in parallel. Linux is unchanged by design |

## Other changes on the branch

- **`make test` now runs a black-box battery** ([`49847ff`](https://github.com/dmartsapp/shint/commit/49847ff)): 356 cases driving the real binary against loopback servers that misbehave on purpose, judged on exit status, output, JSON validity, timing, crashes, signals and memory. It found 15 issues, [#26](https://github.com/dmartsapp/shint/issues/26) to [#40](https://github.com/dmartsapp/shint/issues/40); the two above are fixed here, and the cases for the others are listed in `test/battery/known_issues.py` and must keep failing until each is fixed. See `test/battery/README.md`.
- **The release pipeline posts its result to Slack** ([`280759e`](https://github.com/dmartsapp/shint/commit/280759e)): a sixth, tag-only workflow waits for the other five and posts one message with each workflow's outcome, duration and link. The webhook is the repository secret `SLACK_WEBHOOK_URL`.
- The version bump, the dated changelog and the docs version examples are in [`6607f21`](https://github.com/dmartsapp/shint/commit/6607f21).

## Found by the battery, planned for later releases

[#28](https://github.com/dmartsapp/shint/issues/28), [#29](https://github.com/dmartsapp/shint/issues/29), [#30](https://github.com/dmartsapp/shint/issues/30), [#31](https://github.com/dmartsapp/shint/issues/31), [#32](https://github.com/dmartsapp/shint/issues/32), [#33](https://github.com/dmartsapp/shint/issues/33), [#34](https://github.com/dmartsapp/shint/issues/34) (v4.1.0 - in progress on `release/v4.1.0`), and, from the same battery: [#35](https://github.com/dmartsapp/shint/issues/35), [#36](https://github.com/dmartsapp/shint/issues/36), [#38](https://github.com/dmartsapp/shint/issues/38), [#39](https://github.com/dmartsapp/shint/issues/39), [#40](https://github.com/dmartsapp/shint/issues/40) (v4.2.0) and [#37](https://github.com/dmartsapp/shint/issues/37) (v4.3.0).

## How README files work in this project

- A **release branch's README** (this page) covers that branch alone: its milestone targets, its bugs, its changes and diffs. It is updated as the branch moves.
- **`main`'s README** covers the whole project: all milestones, the releases, the install and usage overview. It shows the latest release dynamically (the release badge and download link point at "latest"), so a release needs no edit to it. Changes to it are made on `main`, in their own commits, never brought in from a branch.
- So before a release branch is merged, its `readme.md` is put back to `main`'s (`make release-check` verifies that), and the fast-forward changes nothing in `main`'s README. Details: [Releases and tagging](https://dmartsapp.github.io/shint/docs/tech-release.html).
