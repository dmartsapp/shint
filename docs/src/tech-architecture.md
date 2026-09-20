---
title: Architecture
lead: How shint is put together - the layers, the life of a command, the dependencies and the concurrency model.
description: Technical overview of shint - repository layout, layers, request lifecycle, dependencies and concurrency.
section: Technical
order: 1
nav: Architecture
---

## Goals

The design follows from a few goals, and most decisions in the code can be traced back to one of them:

- **One static binary.** No runtime, no config files, no helper programs. Everything is built with `CGO_ENABLED=0`, so a release is a single file per platform.
- **One way of working.** Every command takes a target, shares the same flags, prints the same line grammar and offers the same JSON skeleton.
- **Testable without a network.** Every check can be pointed at a listener the tests start themselves (`127.0.0.1` or `::1`); the `listen` commands exist partly for this reason.
- **No privileges.** Nothing requires root: ICMP uses unprivileged sockets, and there is no raw-socket scanning.
- **Honest measurement.** Timeouts bound the operation they are named for, and byte counts are taken on the wire. See [Design notes](tech-design.md).

## Repository layout

```plain
shint/
├── main.go                 command line: cobra commands, flags, validation, exit status
├── main_test.go            end-to-end tests: real exit statuses via the test binary
├── lib/
│   ├── lib.go              shared helpers: DNS, dialing, validation, statistics
│   ├── output.go           JSON output types and the text log format
│   └── handlers/           one file per command (plus TLS and wire counting)
├── docs/                   this documentation site (Markdown sources + build script)
├── .github/workflows/      the five CI/CD workflows, plus the tag guard they all call
├── .github/scripts/        the tag guard's script and its offline test
├── Makefile                cross-compilation for every release platform
├── Dockerfile              multi-arch container image
├── basic_module_test.sh    live-internet smoke test
├── CHANGELOG.md            release history
└── readme.md               the short introduction
```

The full, file-by-file description is in the [Source reference](tech-source.md).

## Layers

:::html
<div class="diagram">
<svg viewBox="0 0 660 372" role="img" aria-label="shint architecture: command line, handlers, shared library, Go standard library">
  <defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z"/></marker></defs>
  <rect class="box accent" x="110" y="14" width="440" height="62" rx="10"/>
  <text class="h" x="330" y="41" text-anchor="middle">main.go - the command line</text>
  <text class="sub" x="330" y="61" text-anchor="middle">cobra commands · flags · validation · exit status</text>
  <path class="arrow" d="M330 76V108"/>
  <rect class="box" x="110" y="108" width="300" height="70" rx="10"/>
  <text class="h" x="260" y="135" text-anchor="middle">lib/handlers - one per command</text>
  <text class="sub" x="260" y="155" text-anchor="middle">telnet · icmp · web · nmap · udp · listen</text>
  <text class="sub" x="260" y="170" text-anchor="middle">concurrency, output (text or JSON), result</text>
  <rect class="box" x="440" y="108" width="110" height="70" rx="10"/>
  <text class="h" x="495" y="135" text-anchor="middle">go-ping/v2</text>
  <text class="sub" x="495" y="155" text-anchor="middle">ICMP echo</text>
  <path class="arrow" d="M410 143H440"/>
  <path class="arrow" d="M260 178V210"/>
  <rect class="box" x="110" y="210" width="300" height="64" rx="10"/>
  <text class="h" x="260" y="236" text-anchor="middle">lib - shared building blocks</text>
  <text class="sub" x="260" y="256" text-anchor="middle">DNS · dialing · validation · log lines · JSON types</text>
  <path class="arrow" d="M260 274V306"/>
  <rect class="box" x="110" y="306" width="440" height="52" rx="10"/>
  <text class="h" x="330" y="330" text-anchor="middle">Go standard library</text>
  <text class="sub" x="330" y="347" text-anchor="middle">net · net/http · crypto/tls · context → the operating system's network stack</text>
</svg>
</div>
:::

- **`main.go`** owns everything about *invoking* shint: it defines the commands and flags, validates arguments, and turns a handler's result into the process exit status. It contains no networking.
- **`lib/handlers`** owns everything about *running* a check: the concurrency, the timing, the choice between text and JSON output, and the decision of whether every check passed. Each handler takes plain values (not cobra types), which is what makes it directly testable.
- **`lib`** holds the pieces shared by more than one handler: name resolution, the single-port dial, argument validators, the statistics helpers, and the output types and log formatting that keep every command consistent.

The dependency direction is strictly downward: `main` → `handlers` → `lib`.

## Life of a command

Following `shint telnet example.com 443 --count 2`:

1. `main()` adds the commands to the root command and calls `Execute`. cobra parses the arguments into the package-level flag variables and calls `telnetCmd.Run`.
2. `Run` validates the port and the numeric flags. A bad value calls `usage(...)`, which prints to stderr and records exit status `2`; nothing else runs.
3. `Run` calls `handlers.TelnetHandler`, which resolves the name with `lib.ResolveName` (both A and AAAA records), bounded by `--timeout`.
4. For each iteration and each address it waits `--delay`, then starts a goroutine that calls `lib.IsPortUp` - a `net.Dialer` with the per-attempt timeout - and records the result under a mutex.
5. Once every goroutine is done, the handler prints the statistics block (text) or marshals the JSON document, and returns `true` only if every attempt succeeded.
6. `Run` passes that to `finish(ok)`, which records exit status `1` if it was `false`; `main` then exits with the recorded status.

The other commands follow the same six steps; only step 4 differs.

## Dependencies

| Module | Version | Used for |
|---|---|---|
| Go | 1.27.1 (`go.mod`) | The toolchain, and the whole networking stack (`net`, `net/http`, `crypto/tls`) |
| `github.com/spf13/cobra` | v1.10.2 | Commands, flags, help, shell completion |
| `github.com/dmartsapp/go-ping/v2` | v2.0.0 | ICMP echo over IPv4 and IPv6 (used only by `ping`) |
| `github.com/spf13/pflag`, `mousetrap` | indirect | cobra's flag parsing and Windows support |
| `golang.org/x/net`, `golang.org/x/sys` | indirect | Pulled in by go-ping |

Everything else - DNS, TCP, UDP, HTTP, TLS, JSON, signal handling - is the standard library. There is no HTTP framework, no logging library and no configuration library.

## Concurrency model

| Command | How the work is spread |
|---|---|
| `telnet`, `web`, `udp` | Attempts are launched one at a time, `--delay` apart, each in its own goroutine. Results are collected under a mutex and a `WaitGroup` waits for all of them. |
| `ping` | go-ping runs the echo requests in parallel; shint streams its log lines from a channel. |
| `nmap` | A launcher loop starts one goroutine per port, but a channel used as a semaphore keeps at most **500** in flight. A separate goroutine prints progress from a ticker. |
| `listen tcp` | An accept loop starts one goroutine per connection. |
| `listen udp` | A single read loop. |
| `listen http` | Go's `net/http` server; each connection is wrapped so its bytes can be counted and reported when it closes. |

Cancellation is by `context`. `nmap` and the listeners cancel on `Ctrl+C` (`SIGINT`/`SIGTERM`); no command imposes a run-wide deadline, because `--timeout` bounds single operations only.

## Cross-cutting concerns

| Concern | Where it lives |
|---|---|
| Text vs JSON output | Each handler checks its `jsonoutput` flag; both modes are produced from the same measurements. |
| Log grammar | `lib.LogWithTimestamp`, `lib.Fields`, `lib.LogStats` in `lib/output.go`. |
| JSON schema | The structs in `lib/output.go`; field tags are the contract. |
| Dual-stack | `lib.NetworkType = "ip"` makes every lookup return A and AAAA records; handlers test each address separately. |
| Byte counting | `lib/handlers/wireconn.go`, shared by `web` and `listen http`. |
| Exit status | `exitCode`, `usage()` and `finish()` in `main.go`; handlers only report success or failure. |

## Where to change what

| To change... | Look in |
|---|---|
| A flag, a default, a help text | `main.go` |
| What a command does | its file in `lib/handlers/` |
| A log line's wording | the handler that prints it (and its test) |
| A JSON field | `lib/output.go` (and update [Output formats](output.md)) |
| How addresses are resolved or dialed | `lib/lib.go` |
| The release build | `.github/workflows/build.yaml`, `Makefile`, `Dockerfile` |
