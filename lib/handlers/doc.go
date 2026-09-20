// Package handlers contains one handler per shint command - telnet, icmp, web,
// nmap, udp and the three listeners - plus the pieces only they need (TLS
// configuration and wire-level byte counting).
//
// Conventions every handler follows:
//
//   - It takes plain values, never cobra types, so tests call it directly.
//   - jsonoutput selects text or JSON output. Both are rendered from the same
//     measurements, and in JSON mode nothing but the JSON document is printed
//     to stdout.
//   - A failed check is a result, not the end of the run: it is reported as an
//     ERROR line (or a success=false entry) and the remaining checks still run.
//   - The check handlers (telnet, icmp, web, nmap, udp) return true only if
//     every check passed; main.go turns that into the exit status. The
//     listeners run until stopped.
//   - --timeout bounds one operation - an attempt, a request, a port, a probe,
//     a lookup - never the whole run.
//   - Each file declares a <name>Module constant, the [module] shown in its log
//     lines.
//
// See docs/src/tech-architecture.md and docs/src/tech-cli.md.
package handlers
