# Security policy

## Supported versions

Security fixes go into **the newest release only**. Use the [latest release](https://github.com/dmartsapp/shint/releases/latest) - the Docker tags `latest`, `4` and `4.x` follow it.

Older releases stay published - their binaries, checksums, attestations and image tags are never deleted, so anything pinned to one keeps working and every release can still be verified - but they are not patched. A release found to be affected by a vulnerability is listed in the advisory, with the release that fixes it.

## Reporting a vulnerability

Report it privately: **[Report a vulnerability](https://github.com/dmartsapp/shint/security/advisories/new)** (GitHub's private reporting, on the repository's Security tab). Please do not open a public issue for a vulnerability that is not public yet.

Include what `shint --version` prints, the command you ran, and what happened. You will get an answer in the report itself, and the fix is credited to you unless you ask otherwise.

A vulnerability that is already public - in shint or in one of its dependencies - can go in an ordinary [issue](https://github.com/dmartsapp/shint/issues).

## Dependencies

shint is built with the Go toolchain and its standard library, and a few modules: `golang.org/x/net` and `golang.org/x/sys` (the Go project), `github.com/spf13/cobra` and its own dependencies, and `github.com/dmartsapp/go-ping` (maintained by this project). How their vulnerabilities are handled:

- **Fixes are taken in the release cycle.** When a dependency publishes a fix - a Go release, a new module version - shint moves to it in its next scheduled release.
- **Out of cycle, when it is serious.** When a dependency's flaw is serious for shint as it is released - it changes what shint reports, or breaks how shint is used or our own tooling - shint gets a patch release outside the cycle, as soon as the fix is public. v4.2.4 was one.
- **Every release is checked.** `govulncheck` gates every release - it refuses to publish one whose code reaches a known vulnerability - and runs on every local `make check` too.
- **Not liable for code we did not write.** shint is provided "as is", without warranty of any kind, under its [MIT license](LICENSE). The Go toolchain, its standard library and the modules shint uses are the work and responsibility of their own authors; we follow their advisories and ship their fixes as described above, but we are not liable for flaws in code we did not write.

## Software bill of materials

Every release binary has its software bill of materials (SBOM) next to it on the release page, in **CycloneDX 1.6** (`<binary>.cdx.json`) and **SPDX 2.3** (`<binary>.spdx.json`): the Go release that built it - the standard library, where every advisory so far has come from - and every Go module compiled in, with versions and `go.sum` hashes, read from the build information Go embeds in the binary. Feed one to a vulnerability scanner to check a release you run against advisories published after it.

From v4.3.0 the release workflow makes them with each binary and attests them (`gh attestation verify <binary> --repo dmartsapp/shint --predicate-type https://cyclonedx.org/bom`, or `https://spdx.dev/Document/v2.3`). v4.0.0 to v4.2.4 got theirs afterwards, made from their published binaries, unattested. Details: the [Security](https://dmartsapp.github.io/shint/docs/security.html#software-bill-of-materials) page.

## Advisories

Each advisory - what was affected, the original sources and CVE numbers, how shint was affected and what was done - is on the documentation's [Security](https://dmartsapp.github.io/shint/docs/security.html#advisories) page.

| Date | Advisory | Affected | Fixed in |
|---|---|---|---|
| 2026-11-08 | [Go 1.27.1 standard library: 10 vulnerabilities, among them compromised intermediate certificates still trusted](https://dmartsapp.github.io/shint/docs/security.html#2026-11-08-go-1-27-1-standard-library) ([#91](https://github.com/dmartsapp/shint/issues/91)) | v4.0.0 - v4.2.3 | [v4.2.4](https://github.com/dmartsapp/shint/releases/tag/v4.2.4) |
