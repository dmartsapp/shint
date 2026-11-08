---
title: Security
lead: Which releases get security fixes, how to report a vulnerability, how shint handles its dependencies' vulnerabilities - and every advisory so far, with its original sources.
description: shint's security and support policy, and its security advisories with CVE numbers, sources, impact and resolution.
section: Reference
order: 3
nav: Security
---

## Supported versions

Security fixes go into **the newest release only**: use the [latest release](https://github.com/dmartsapp/shint/releases/latest) (`shint --version` says which one you have; the Docker tags `latest`, `{{major}}` and `{{minor}}` follow it). Upgrading is the fix - see [Install](install.md).

**Older releases stay published, and are not patched.** Their binaries, checksums, attestations and image tags are never deleted or replaced: anything pinned to one keeps working, every release can still be verified, and what a user ran can always be examined later. A release found to be affected by a vulnerability is listed in that vulnerability's [advisory](#advisories), with the release that fixes it. Taking a release down is kept for a release that is itself compromised - a tampered build, say - not for one that turns out to contain a known flaw.

## Reporting a vulnerability

Report it privately: **[Report a vulnerability](https://github.com/dmartsapp/shint/security/advisories/new)**, GitHub's private reporting on the repository's Security tab. Please do not open a public issue for a vulnerability that is not public yet.

Include what `shint --version` prints, the command you ran and what happened. The answer comes in the report itself, and the fix is credited to you unless you ask otherwise.

A vulnerability that is already public - in shint or in one of its dependencies - can go in an ordinary [issue](https://github.com/dmartsapp/shint/issues).

## Dependencies

shint is built with the [Go](https://go.dev) toolchain and its standard library - which carries nearly all of its networking: DNS, TCP, UDP, HTTP and TLS - and a few modules: `golang.org/x/net` and `golang.org/x/sys` (from the Go project), `github.com/spf13/cobra` with its own dependencies, and `github.com/dmartsapp/go-ping` (maintained by this project). The [dependency table](tech-architecture.md#dependencies) lists their versions.

**How their vulnerabilities are handled:**

- **In the release cycle.** When a dependency publishes a fix - a Go release, a new version of a module - shint moves to it in its next scheduled release (one every two weeks; see [Branches and cadence](tech-release.md#branches-and-cadence)).
- **Out of cycle, when it is serious.** When a dependency's flaw is serious for shint as it is released - it changes what shint reports, or breaks how shint is used or the project's own tooling - shint gets a patch release outside the cycle, as soon as the fix is public. [v4.2.4](#2026-11-08-go-1-27-1-standard-library) was one.
- **Not liable for code we did not write.** shint is provided "as is", without warranty of any kind, under its [MIT license](https://github.com/dmartsapp/shint/blob/main/LICENSE). The Go toolchain, its standard library and the modules shint uses are the work and the responsibility of their own authors. We follow their advisories and ship their fixes as described here, but we are not liable for flaws in code we did not write.

## How releases are checked

- **[`govulncheck`](https://go.dev/doc/security/vuln/)**, the Go project's vulnerability scanner, gates every release: the release workflows refuse to publish a release whose code reaches a known vulnerability in the [Go vulnerability database](https://pkg.go.dev/vuln/). It also runs in every local `make check` ([Testing](tech-testing.md)).
- **It reports what shint's code can reach**, not only what it contains: a vulnerability counts when there is a call path from shint to the vulnerable function. A path is not proof that the flaw can be triggered - it often needs conditions shint never creates - so each advisory below says how shint was affected in practice. shint takes the fix either way.
- **Between releases nothing scans on its own.** By design, no workflow runs except for a release tag ([the workflow trigger rule](tech-ci.md#the-workflow-trigger-rule)). An advisory published between two releases is found by the next `make check` or release - which is why the advisory below was found a month after its first fixes were published.
- **Release integrity** - checksums and signed build attestations for every binary - is described in [Install](install.md).

## Advisories

### 2026-11-08: Go 1.27.1 standard library

| | |
|---|---|
| Affected | **shint v4.0.0 to v4.2.3**: every binary and container image, all built with Go 1.27.1 and `golang.org/x/net` v0.59.0. Older lines (v3 and before) were built with older Go releases and are not supported. |
| Fixed in | **[v4.2.4](https://github.com/dmartsapp/shint/releases/tag/v4.2.4)**, released 2026-11-08: built with Go 1.27.3 and `golang.org/x/net` v0.62.0 |
| What to do | Upgrade to v4.2.4 or later ([Install](install.md)). A Docker image pulled by `latest`, `4` or `4.2` gets it on the next pull. Building an older version from source with a current Go also gets the fixed standard library (its x/net stays at v0.59.0, whose advisories shint does not reach). |
| Issue | [#91](https://github.com/dmartsapp/shint/issues/91) |

**What happened.** The Go project published fixes for ten vulnerabilities in its standard library that shint's code reaches - nine in Go 1.27.2 (October 8, 2026) and one in Go 1.27.3 (October 21) - after shint v4.2.3 (September 26) was built with Go 1.27.1. `govulncheck` found them on November 8, in a local `make check` during the v4.3.0 work, and v4.2.4 shipped the same day as an out-of-cycle security patch.

**The vulnerabilities**, from the [Go vulnerability database](https://pkg.go.dev/vuln/) (dates are the database's publication dates; the issue, fix and announcement links are the Go project's own):

| Advisory | CVE | What | Fixed in | Original sources |
|---|---|---|---|---|
| [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) (2026-10-21) | [CVE-2026-111943](https://www.cve.org/CVERecord?id=CVE-2026-111943) | Distrust compromised intermediate certificates in crypto/x509 | Go 1.26.10, Go 1.27.3 | [issue](https://go.dev/issue/82345), [fix](https://go.dev/cl/851047), [announcement](https://groups.google.com/g/golang-announce/c/66pojueaQek) |
| [GO-2026-6605](https://pkg.go.dev/vuln/GO-2026-6605) (2026-10-08) | [CVE-2026-56866](https://www.cve.org/CVERecord?id=CVE-2026-56866) | HTTP/1 client connection desynchronization after CONNECT rejection in net/http | Go 1.26.9, Go 1.27.2 | [issue](https://go.dev/issue/81740), [fix](https://go.dev/cl/847306), [announcement](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI) |
| [GO-2026-6613](https://pkg.go.dev/vuln/GO-2026-6613) (2026-10-08) | [CVE-2026-94439](https://www.cve.org/CVERecord?id=CVE-2026-94439) | HTTP/1 server connection desynchronization after 2xx CONNECT response in net/http | Go 1.26.9, Go 1.27.2 | [issue](https://go.dev/issue/81744), [fix](https://go.dev/cl/847311), [announcement](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI) |
| [GO-2026-6610](https://pkg.go.dev/vuln/GO-2026-6610) (2026-10-08) | [CVE-2026-78660](https://www.cve.org/CVERecord?id=CVE-2026-78660) | HTTP/2 transport accepts malformed framing-related headers in net/http | Go 1.26.9, Go 1.27.2, x/net v0.60.0 | [issue](https://go.dev/issue/81115), [fix 1](https://go.dev/cl/835145), [fix 2](https://go.dev/cl/836385), [announcement](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI) |
| [GO-2026-6611](https://pkg.go.dev/vuln/GO-2026-6611) (2026-10-08) | [CVE-2026-78669](https://www.cve.org/CVERecord?id=CVE-2026-78669) | Excessive CPU consumption from repeated initial window changes in net/http | Go 1.26.9, Go 1.27.2, x/net v0.60.0 | [issue](https://go.dev/issue/81742), [fix 1](https://go.dev/cl/847186), [fix 2](https://go.dev/cl/847308), [announcement 1](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI), [announcement 2](https://groups.google.com/g/golang-announce/c/ZPwCyRUuGBs) |
| [GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617) (2026-10-08) | [CVE-2026-97032](https://www.cve.org/CVERecord?id=CVE-2026-97032) | HTTP/2 server crash due to HPACK encoder race in net/http | Go 1.26.9, Go 1.27.2, x/net v0.60.0 | [issue](https://go.dev/issue/81867), [fix 1](https://go.dev/cl/847188), [fix 2](https://go.dev/cl/847313), [announcement 1](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI), [announcement 2](https://groups.google.com/g/golang-announce/c/ZPwCyRUuGBs) |
| [GO-2026-6612](https://pkg.go.dev/vuln/GO-2026-6612) (2026-10-08) | [CVE-2026-78663](https://www.cve.org/CVERecord?id=CVE-2026-78663) | Double flow control refund on HTTP/2 server streams in net/http | Go 1.26.9, Go 1.27.2, x/net v0.60.0 | [issue](https://go.dev/issue/81743), [fix 1](https://go.dev/cl/847187), [fix 2](https://go.dev/cl/847310), [announcement 1](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI), [announcement 2](https://groups.google.com/g/golang-announce/c/ZPwCyRUuGBs) |
| [GO-2026-6603](https://pkg.go.dev/vuln/GO-2026-6603) (2026-10-08) | [CVE-2026-78659](https://www.cve.org/CVERecord?id=CVE-2026-78659) | HTTP/2 server memory exhaustion due to Trailer headers in net/http | Go 1.26.9, Go 1.27.2, x/net v0.60.0 | [issue](https://go.dev/issue/81857), [fix 1](https://go.dev/cl/847185), [fix 2](https://go.dev/cl/847314), [announcement 1](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI), [announcement 2](https://groups.google.com/g/golang-announce/c/ZPwCyRUuGBs) |
| [GO-2026-6607](https://pkg.go.dev/vuln/GO-2026-6607) (2026-10-08) | [CVE-2026-97031](https://www.cve.org/CVERecord?id=CVE-2026-97031) | Reject malformed ECH outer extension references in crypto/tls | Go 1.26.9, Go 1.27.2 | [issue](https://go.dev/issue/81855), [fix](https://go.dev/cl/847312), [announcement](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI) |
| [GO-2026-6608](https://pkg.go.dev/vuln/GO-2026-6608) (2026-10-08) | [CVE-2026-94440](https://www.cve.org/CVERecord?id=CVE-2026-94440) | Memory limit bypass when parsing MIME headers in net/textproto, mime/multipart | Go 1.26.9, Go 1.27.2 | [issue](https://go.dev/issue/81741), [fix](https://go.dev/cl/847307), [announcement](https://groups.google.com/g/golang-announce/c/U2fTuyDJznI) |
| [GO-2026-6866](https://pkg.go.dev/vuln/GO-2026-6866) (2026-10-27) | [CVE-2026-97045](https://www.cve.org/CVERecord?id=CVE-2026-97045) | Check all ancestor namespace transitions when rendering raw text in html in golang.org/x/net | x/net v0.62.0 | [issue](https://go.dev/issue/82622), [fix](https://go.dev/cl/855121), [announcement](https://groups.google.com/g/golang-announce/c/sqrs2vTx_Xs) |
| [GO-2026-6860](https://pkg.go.dev/vuln/GO-2026-6860) (2026-10-27) | [CVE-2026-78661](https://www.cve.org/CVERecord?id=CVE-2026-78661) | Avoid quadratic complexity when coalescing text nodes in html in golang.org/x/net | x/net v0.62.0 | [issue](https://go.dev/issue/82224), [fix](https://go.dev/cl/849365), [announcement](https://groups.google.com/g/golang-announce/c/sqrs2vTx_Xs) |

The last two are in `golang.org/x/net/html`, which shint does not import - `govulncheck` reported them as present but not reached - and are listed because v4.2.4 moved past them too.

**How shint was affected.** One of them changed what shint could report, and one needs a request only a user can ask for; the rest were reachable in the code, but need conditions shint does not create:

| Advisory | How shint was affected |
|---|---|
| [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) | **Affected.** `web` verifies every HTTPS certificate with `crypto/x509`. Where the trust store had not been updated to drop these intermediates - a container image's CA bundle is fixed when the image is built - `web` could accept a certificate chained through one, and report a response instead of a certificate error. |
| [GO-2026-6605](https://pkg.go.dev/vuln/GO-2026-6605) | **Only with `web -X CONNECT` and a body.** `web` uses no proxy, so it never sends a `CONNECT` of its own; asked for one with `--data`, and refused with a keep-alive answer, a later attempt of the same run (`--count`) could read the wrong response. |
| [GO-2026-6613](https://pkg.go.dev/vuln/GO-2026-6613) | **Not in practice** - tested with v4.2.3 and v4.2.4. A real `CONNECT host:port` is answered 404 by `listen http`, never a 2xx; even an unusual `CONNECT /`, answered 200, has its connection closed after the answer, and a request sent behind it is never served. |
| [GO-2026-6610](https://pkg.go.dev/vuln/GO-2026-6610) | **Not in practice.** The flaw is in the HTTP/2 transport acting as a reverse proxy; `web` speaks HTTP/1.1 only, and forwards nothing. |
| [GO-2026-6611](https://pkg.go.dev/vuln/GO-2026-6611) | **Not in practice.** It needs an HTTP/2 connection; `web` speaks HTTP/1.1 only, and `listen http` serves HTTP/1.1 only. |
| [GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617) | **Not in practice.** HTTP/2 servers only; `listen http` serves plain HTTP/1.1 and never enables HTTP/2. |
| [GO-2026-6612](https://pkg.go.dev/vuln/GO-2026-6612) | **Not in practice.** HTTP/2 servers only, as above. |
| [GO-2026-6603](https://pkg.go.dev/vuln/GO-2026-6603) | **Not in practice.** HTTP/2 servers only, as above. |
| [GO-2026-6607](https://pkg.go.dev/vuln/GO-2026-6607) | **Not in practice.** A flaw of TLS servers; shint runs none (`web` is a TLS client, `listen http` has no TLS). |
| [GO-2026-6608](https://pkg.go.dev/vuln/GO-2026-6608) | **Not in practice.** Multipart form parsing; shint parses no multipart forms (`listen http` reads request bodies without parsing them). |
| [GO-2026-6866](https://pkg.go.dev/vuln/GO-2026-6866) | **Not affected.** In `golang.org/x/net/html`, which shint does not import. Cleared anyway by moving to x/net v0.62.0. |
| [GO-2026-6860](https://pkg.go.dev/vuln/GO-2026-6860) | **Not affected.** As above. |

**What was done, in v4.2.4** ([`dac550f`](https://github.com/dmartsapp/shint/commit/dac550f)):

- `go.mod` names Go 1.27.3 - the version every workflow builds the binaries with - and `golang.org/x/net` v0.62.0 (v0.60.0 would have been enough for the reachable ones; v0.62.0 also clears the two in `x/net/html`). `govulncheck` finds no vulnerabilities at all.
- The container images are built with `golang:1.27.3-alpine`, and a test now fails if the `Dockerfile` and `go.mod` ever name different Go releases, so the images and the binaries cannot drift apart.
- golangci-lint moved from v2.13.2 to v2.15.0: Go 1.27.3 writes compiled packages in a newer format that v2.13.2 cannot read.
- How to move to a new Go release, for the next one, is written down: [Moving to a new Go release](tech-ci.md#moving-to-a-new-go-release).
