# shint v4.2.3 - work in progress

Summary: `shint --version` also shows the Go version it was built with, and for which platform
Includes:

> The working page for `release/v4.2.3`: what the release is meant to deliver, the bugs it fixes, what changed, and what is left to do - kept up to date as the branch moves. **`Summary:`** is the one plain-language line main's README roadmap will show for this release; **`Includes:`** names releases folded into this one (like `v4.4.0`), if any. `make release` reads both, refuses to publish while anything under **To do** is unchecked, and moves this file to `releases/v4.2.3.md` in the release commit, where it stays on `main` as the release's record. Main's roadmap row links here in the meantime.

| | |
|---|---|
| Milestone | [v4.2.3](https://github.com/dmartsapp/shint/milestone/11) - no due date: an ad hoc patch, and the first release made with `make release` ([#76](https://github.com/dmartsapp/shint/issues/76)) end to end |
| Built on | `main`, after the v4.2.2 tag |
| Everything that differs from main | [main...release/v4.2.3](https://github.com/dmartsapp/shint/compare/main...release/v4.2.3) |

## To do

- [x] every target and bug below is done, or moved to a later milestone
- [x] the changelog has an entry for every change a user would notice
- [x] `Summary:` above says what this release is, in plain language
- [x] a clean rehearsal: `make release DRY_RUN=1` - passed on 2026-09-26 (every check, release-check, the stamped commit and README row inspected)

## Targets

| Target | Issue | State |
|---|---|---|
| `shint --version` and `-v` also print the Go toolchain and the platform the binary was built for: `v4.2.3/<commit>/<time> (go1.27.1 linux/amd64)` - the version stays the first word | [#77](https://github.com/dmartsapp/shint/issues/77) | Done - [`c8d6016`](https://github.com/dmartsapp/shint/commit/c8d6016) |

## Bugs

| Issue | State |
|---|---|
| [#78](https://github.com/dmartsapp/shint/issues/78) The fd-pressure tests (a Go test and a battery case) failed on the machine's own connection failures - a full accept queue, no free local port - and stopped two release rehearsals | Done - [`ebceb42`](https://github.com/dmartsapp/shint/commit/ebceb42): they allow those, up to 10%, while still failing on "too many open files" |

## Other changes

- This release is the end-to-end test of the new release process ([#76](https://github.com/dmartsapp/shint/issues/76)): the branch was started with `make release-start` and is released with `make release`. Its working page moved from `releases/v4.2.3.md` to `branch_readme.md` mid-sprint, when the process gained the branch page and its To do list.
