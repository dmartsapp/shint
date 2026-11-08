# shint v4.2.4 - work in progress

Summary: Security patch: built with Go 1.27.3, which fixes 10 Go vulnerabilities in every earlier v4 release - among them, certificates from compromised intermediates were still trusted
Includes:

> The working page for `release/v4.2.4`: what the release is meant to deliver, the bugs it fixes, what changed, and what is left to do - kept up to date as the branch moves. **`Summary:`** is the one plain-language line main's README roadmap will show for this release; **`Includes:`** names releases folded into this one (like `v4.4.0`), if any. `make release` reads both, refuses to publish while anything under **To do** is unchecked, and moves this file to `releases/v4.2.4.md` in the release commit, where it stays on `main` as the release's record. Main's roadmap row links here in the meantime.

| | |
|---|---|
| Milestone | [v4.2.4](https://github.com/dmartsapp/shint/milestone/13) - due 2026-11-10 (a security fix: 48 hours from when it was flagged, 2026-11-08) |
| Kind | Security patch, outside the sprint cadence |
| Built on | `main`, after the v4.2.3 tag |
| Everything that differs from main | [main...release/v4.2.4](https://github.com/dmartsapp/shint/compare/main...release/v4.2.4) |

## To do

- [x] every target and bug below is done, or moved to a later milestone
- [x] the changelog has an entry for every change a user would notice
- [x] `Summary:` above says what this release is, in plain language

## Targets

| Target | Issue | State |
|---|---|---|

## Bugs

| Issue | State |
|---|---|
| [#91](https://github.com/dmartsapp/shint/issues/91) Every v4 release (v4.0.0 to v4.2.3) is built with Go 1.27.1 and `golang.org/x/net` v0.59.0, and `govulncheck` finds 10 standard-library vulnerabilities that shint's code reaches - GO-2026-6809 (certificates from compromised intermediates still trusted, so `web` could call one valid), HTTP/2 and HTTP/1 `CONNECT` handling, TLS ECH, MIME headers | Done - [`dac550f`](https://github.com/dmartsapp/shint/commit/dac550f): Go 1.27.3, x/net v0.62.0, `golang:1.27.3-alpine`; `govulncheck` finds nothing. First made on `release/v4.3.0` ([`516e339`](https://github.com/dmartsapp/shint/commit/516e339)) |

## Other changes

- **golangci-lint v2.13.2 -> v2.15.0** (`Makefile`, `check.yaml`, the four release workflows), forced by the Go upgrade: Go 1.27.3 writes compiled packages in export data version 5, which v2.13.2's `golang.org/x/tools` cannot read. v2.15.0 found one unchecked error, in `listen http`'s shutdown, now checked (no change in practice). Part of [`dac550f`](https://github.com/dmartsapp/shint/commit/dac550f).
- **`toolchain_test.go`** fails when the `Dockerfile`'s builder image and `go.mod` name different Go releases, so the images and the binaries cannot drift apart.
- **Docs:** the Go version comes from `go.mod` through a `{{go}}` token instead of being typed on four pages; [Moving to a new Go release](https://dmartsapp.github.io/shint/docs/tech-ci.html#moving-to-a-new-go-release) is the procedure; the dependency table is corrected (go-ping v2.0.1; x/net is a direct dependency, for `dns`).
- **Not here:** `github.com/dmartsapp/go-ping`'s own `go.mod` still names x/net v0.59.0 and Go 1.27.1. shint builds with the newer x/net either way; go-ping itself is updated with the v4.4.0 go-ping work.
