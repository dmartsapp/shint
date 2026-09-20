---
title: Command-line design
lead: The conventions behind shint's commands and flags - and the reasoning, so new commands feel like they were always there.
description: The philosophy and conventions of the shint command line - grammar, shared flags, defaults, validation, output contract, exit status - and how to add a command.
section: Technical
order: 3
nav: Command-line design
---

## Principles

**1. One grammar, learned once.** `shint <command> <target> [flags]`. The target is positional because it is what you are *about*; flags are how the check behaves. Commands are named after the tools whose muscle memory they inherit - `telnet`, `ping`, `nmap` - rather than for what they technically do, and `web` and `listen` are short verbs for the rest.

**2. Shared flags are defined once.** `--count`, `--timeout`, `--delay`, `--throttle`, `--payload` and `--json` live on the root command as *persistent* flags, so they have identical names, defaults and help text everywhere. A command simply ignores the ones that do not apply. Two shadowing cases exist, both deliberate and both documented in the flag help:

| Shadow | Why |
|---|---|
| `listen --count` defaults to `0` (run until `Ctrl+C`) instead of `1` | A listener's point is usually to stay up; every other command's default is "one check and done". |
| `web -P/--payload` is the request *body* | On `ping` and `udp`, `--payload` is a *size* of filler. HTTP needs actual content, and `-P` mirrors `curl`'s single-letter style. |

**3. Safe, quick defaults.** `--count 1`, `--timeout 5`, `--delay 1000`. A bare command does one polite check. Nothing scans, floods or listens forever unless asked.

**4. Validate first, then act.** Ports must be 1-65535, `--count` and `--timeout` at least 1, `--from` no greater than `--to`, TLS options must be consistent. A bad value prints one clear line to **stderr**, exits `2`, and does nothing else - no half-run.

**5. Errors are data, not the end of the run.** A failed connection is a *result*: it is printed as an `ERROR` line (or a `success: false` entry in JSON) and the remaining checks still run. The program decides at the end whether everything passed.

**6. Streams have jobs.** Results - including `ERROR` lines about failed checks - go to stdout; usage errors go to stderr. That keeps `shint ... --json | jq` clean in every situation.

**7. No hidden state.** No configuration file, no environment variables, no saved history, no telemetry. The same command line always means the same thing, on any machine.

## The output contract

Text output follows one grammar (`<time>: [<module>] OK|ERROR <message> key=value ...`), implemented once in `lib/output.go`. Anyone adding a message uses `lib.LogWithTimestamp` and `lib.Fields` rather than formatting by hand, which is what keeps every command grep-able the same way.

JSON output is one document per run (one line per event for listeners). The rules:

- **Keys are stable.** They are `lower_snake_case`, with the unit in the name where there is one (`_µs`, `_ms`, `_bytes`). A rename or removal is called out in the changelog.
- **Failure is still JSON.** A failed check appears in `stats` with `success: false` and the reason; the run-level `error` field carries lookup failures. No text line is ever printed in front of the document.
- **One skeleton, per-command `stats`.** `input_params`, `dns_lookup`, `start/end/total time` and `error` are identical across commands; only the entries in `stats` differ.
- **Text and JSON come from the same measurements.** A handler measures once and renders twice, so the two modes cannot disagree.

Known wrinkles in the contract, kept for compatibility and listed on [Output formats](output.md): `timeout_ms` carries seconds, and `ping` reports milliseconds where everything else reports microseconds.

## Exit status

| Status | Meaning |
|---|---|
| `0` | Every check passed. |
| `1` | At least one check failed. |
| `2` | The command was misused; nothing ran. |

The scheme follows `fping` because shint has the same shape of problem: one invocation makes several checks (one per resolved address per iteration), so "the exit status is the verdict on all of them" is the only rule that never lies. The judgement calls, each covered by a test in `main_test.go`:

- **An HTTP status is data.** A `404` or `500` is a response and exits `0`, as with `curl` without `-f`. Whether that status is acceptable is the caller's policy, and it is one `jq` expression away.
- **A completed scan is a success**, whatever it found. Finding nothing open is a result. A lookup failure or a scan cut short is a failure.
- **UDP `open|filtered` is inconclusive**, not a failure: many services never answer input they do not understand. Only `closed` (an explicit ICMP refusal) and errors fail.
- **Handlers report, `main` decides.** Handlers return a `bool` ("every check passed"); only `main.go` knows about process exit. That keeps handlers testable as ordinary functions.

## Adding a command

1. **Handler.** Create `lib/handlers/<name>.go` with a `<name>Module` constant (it becomes the `[module]` in log lines) and a function that takes plain values plus `jsonoutput *bool` and returns `bool`. Print with `lib.LogWithTimestamp`/`lib.Fields`, and render JSON through `lib.JSONOutput`.
2. **Output types.** Add a `<Name>Stats` struct to `lib/output.go` with tagged fields, and document it on [Output formats](output.md).
3. **Command.** In `main.go`, add a `cobra.Command`: `Args:` for the positional shape, `Run` that validates (each failure `usage(...)`; `return`), then calls `finish(handlers.YourHandler(...))`. Register any flag in `init()` and the command in `main()`.
4. **Bound every operation** with `--timeout`, never the whole run.
5. **Tests.** A handler test with a locally started target; an entry in `TestExitStatus` (main_test.go) for each of the 0, 1 and 2 paths; a `--json` failure case in `TestFailuresUnderJSONAreStillJSON`.
6. **Docs.** A command page under `docs/src/`, a line in [Using shint](usage.md) if it changes the shared tables, and a changelog entry.
