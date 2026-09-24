---
title: dns
lead: Ask a DNS server a question and see exactly what it answered - A, AAAA, MX, TXT, NS, SOA, SRV, CAA and reverse lookups, with the server, the flags and the TTLs. dig, in shint's format, without privileges.
description: shint dns asks a DNS server for A, AAAA, CNAME, MX, NS, TXT, SOA, SRV, PTR or CAA records over UDP with a TCP fallback, shows the response code, flags and TTLs, and can ask any server, including an authoritative one.
section: Commands
order: 9
nav: dns
---

## What it does

When a name does not behave, the question is rarely "does it resolve?" but "what does *this* server say, and what is the record actually set to?" `shint dns` asks one DNS question and shows the whole answer: which server answered and how, the response code and header flags, and every record with its TTL. It speaks the DNS protocol itself, so it works the same on Linux, macOS and Windows, needs no `dig` or `nslookup` installed, and needs **no privileges** - it is an ordinary UDP query (retried over TCP if the answer is too big).

It is the general tool; [`rdns`](rdns.md) stays the quick reverse lookup through your system's resolver.

## Syntax

```bash
shint dns <name> [type] [@server] [--tcp] [--no-recurse] [--count N] [--timeout S] [--json]
```

| Argument | Meaning |
|---|---|
| `name` | The name to look up. An **IP address** is looked up in reverse (its PTR record), like `dig -x`. `.` asks about the root. |
| `type` | `A`, `AAAA`, `CNAME`, `MX`, `NS`, `TXT`, `SOA`, `SRV`, `PTR` or `CAA` (any case). **Without a type, a name is asked for its A and AAAA records** - both families, like every shint command - or just one of them with `-4` or `-6`. |
| `@server` | Which server to ask: an address or a name, with an optional port: `@1.1.1.1`, `@dns.google`, `@dns.google:5353`, `@[2606:4700:4700::1111]:5353`, `@::1`. Without it, the servers this machine is configured with are asked. It can go anywhere after the name. |

| Flag | What it does |
|---|---|
| `--tcp` | Ask over TCP only. By default the question goes over UDP and is asked again over TCP if the answer comes back truncated. |
| `--no-recurse` | Clear the "recursion desired" bit: ask the server only for what it knows itself. This is what you use against an **authoritative** server, so it does not go off and look things up. |

The [shared flags](usage.md#flags-shared-by-every-command) apply: `--timeout` is how long **each server** gets, `--count` repeats the question, `--delay` pauses before each, `-4`/`-6` restrict the record types asked (without a type) and the servers used, and `--json` prints one document.

## Which server answers

- **With `@server`**, that server (a name is resolved first; if it has several addresses they are tried in order).
- **Without it**, before sending queries over the network, **forward lookups check the local hosts file** (and the built-in fallback for `localhost` to `127.0.0.1` and `::1`). If a matching record exists, it is answered immediately with `nameserver=hosts` and `transport=file` (flags `[qr,aa]`, TTL 0) without network traffic, allowing local and offline resolution.
- If no hosts record matches (or for reverse PTR lookups), the servers your system is configured with are asked **in order**: the first that answers is used, and a server that does not answer (no reply in time, connection refused) is skipped, with the reason written on the line (`skipped=...`). That is what a resolver does, and what `dig` does. Any answer counts - `NXDOMAIN` from the first server is an answer, not a reason to try the second.

The system's servers are found by asking Go's own resolver which ones it would use, so the list is right on every platform, including Windows where there is no `/etc/resolv.conf`.

## Reading the output

Each question prints one `query` line, then one line per record:

| Field | Meaning |
|---|---|
| `name`, `type` | What was asked (the name fully qualified, with its trailing dot). |
| `nameserver`, `transport` | Who answered and how: `udp` or `tcp`. `retried=tcp` means the UDP answer was truncated and the TCP one is shown. |
| `rcode` | The response code: `noerror`, `nxdomain` (the name does not exist), `servfail`, `refused`, ... **In lower case on purpose**: `NOERROR` would contain the text `ERROR`, and in shint an `ERROR` line means a failed check. The JSON has the usual capitals. |
| `flags` | The header flags that were set: `qr` (a response), `aa` (an **authoritative** answer), `tc` (truncated), `rd` (recursion was asked for), `ra` (the server offers recursion), `ad` (the server says it validated DNSSEC), `cd`. |
| `answers`, `authority`, `additional` | How many records are in each section. Answers and authority are listed; additional (glue, the EDNS option) is only counted. |
| `time` | How long the exchange took. |
| `skipped` | The servers tried first that did not answer, and why. |
| `answer` lines | `name`, `type`, `ttl` (seconds the record may be cached) and `data`, written the way `dig` writes it: an MX as `"10 mail.example.com."`, a CAA as `"0 issue \"letsencrypt.org\""`, an SRV as `"priority weight port target"`. |
| `authority` lines | The records of the authority section - for a negative answer, the **SOA** of the zone, whose last number is how long the "no" may be cached. |

A TXT record's character strings are joined into one value (as SPF and DKIM readers do), and anything that is not plain text - control characters, escape sequences, bytes that are not valid UTF-8 - is shown escaped (`\x1b`, `\n`), never raw: a hostile record cannot garble your terminal or forge a log line.

## Examples

### A name: both families

```bash
shint dns example.com @1.1.1.1
```

```text
Mon Sep 21 12:00:10 MDT 2026: [dns] OK query name=example.com. type=A nameserver=1.1.1.1:53 transport=udp attempt=1/1 rcode=noerror flags=[qr,rd,ra] answers=2 authority=0 additional=0 time=16.978ms
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=example.com. type=A ttl=122 data=104.20.23.154
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=example.com. type=A ttl=122 data=172.66.147.243
Mon Sep 21 12:00:10 MDT 2026: [dns] OK query name=example.com. type=AAAA nameserver=1.1.1.1:53 transport=udp attempt=1/1 rcode=noerror flags=[qr,rd,ra] answers=2 authority=0 additional=0 time=16.022ms
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=example.com. type=AAAA ttl=126 data=2606:4700:10::6814:179a
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=example.com. type=AAAA ttl=126 data=2606:4700:10::ac42:93f3
Mon Sep 21 12:00:10 MDT 2026: [dns] OK done queries=2 answered=2 total_time=33.114792ms
```

Two questions (A and AAAA), each with its own exchange line. The flags `qr,rd,ra` say: a response, to a query that asked for recursion, from a server that offers it. The TTLs (122 and 126 seconds) are how long resolvers may keep the records - they count down between runs.

### Mail servers

```bash
shint dns gmail.com MX @8.8.8.8
```

```text
Mon Sep 21 12:00:10 MDT 2026: [dns] OK query name=gmail.com. type=MX nameserver=8.8.8.8:53 transport=udp attempt=1/1 rcode=noerror flags=[qr,rd,ra] answers=5 authority=0 additional=0 time=27.939ms
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=gmail.com. type=MX ttl=1468 data="5 gmail-smtp-in.l.google.com."
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=gmail.com. type=MX ttl=1468 data="10 alt1.gmail-smtp-in.l.google.com."
...
```

The number is the preference: lower is tried first. The answers come in whatever order the server sends them.

### An address: the reverse lookup

```bash
shint dns 8.8.8.8 @1.1.1.1
```

```text
Mon Sep 21 12:00:10 MDT 2026: [dns] OK query name=8.8.8.8.in-addr.arpa. type=PTR nameserver=1.1.1.1:53 transport=udp attempt=1/1 rcode=noerror flags=[qr,rd,ra] answers=1 authority=0 additional=0 time=17.894ms
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=8.8.8.8.in-addr.arpa. type=PTR ttl=77661 data=dns.google.
Mon Sep 21 12:00:10 MDT 2026: [dns] OK done queries=1 answered=1 total_time=18.01275ms
```

### Asking an authoritative server directly

```bash
shint dns example.com SOA @a.iana-servers.net --no-recurse
```

```text
Mon Sep 21 12:00:10 MDT 2026: [dns] OK server resolved host=a.iana-servers.net addresses=2 ips=[2001:500:8f::53,199.43.135.53] time=5.047333ms
Mon Sep 21 12:00:10 MDT 2026: [dns] OK query name=example.com. type=SOA nameserver=[2001:500:8f::53]:53 transport=udp attempt=1/1 rcode=noerror flags=[qr,aa] answers=1 authority=0 additional=0 time=67.352ms
Mon Sep 21 12:00:10 MDT 2026: [dns] OK answer name=example.com. type=SOA ttl=3600 data="ns.icann.org. noc.dns.icann.org. 2026091701 7200 3600 1209600 3600"
Mon Sep 21 12:00:10 MDT 2026: [dns] OK done queries=1 answered=1 total_time=72.573833ms
```

The flags are `qr,aa`: an **authoritative** answer, with no `rd`/`ra` because recursion was not asked for. This is the record as the zone's own server has it, not as some cache remembers it - the way to tell whether a change has been published (compare the SOA serial `2026091701`) or is only waiting for a cache to expire. The server name was resolved first, hence the `server resolved` line; to find a zone's servers, ask `shint dns example.com NS`.

### A name that does not exist

```bash
shint dns nope.example.invalid A @1.1.1.1
```

```text
Mon Sep 21 12:00:10 MDT 2026: [dns] ERROR query failed name=nope.example.invalid. type=A nameserver=1.1.1.1:53 transport=udp attempt=1/1 rcode=nxdomain flags=[qr,rd,ra] answers=0 authority=1 additional=0 time=18.951ms error="no such domain (NXDOMAIN): nope.example.invalid. does not exist"
Mon Sep 21 12:00:10 MDT 2026: [dns] OK authority name=. type=SOA ttl=86400 data="a.root-servers.net. nstld.verisign-grs.com. 2026092101 1800 900 604800 86400"
Mon Sep 21 12:00:10 MDT 2026: [dns] OK done queries=1 answered=0 total_time=19.116917ms
```

A failed check: an `ERROR` line with the reason and exit status `1`. The authority section names the zone that said "no" and, in the last number of its SOA, how long that "no" may be cached (here 86400 seconds).

### A server that does not answer

```bash
shint dns example.com A @192.0.2.55 --timeout 1
```

```text
Mon Sep 21 12:00:11 MDT 2026: [dns] ERROR query failed name=example.com. type=A attempt=1/1 time=1.002883s error="no DNS server answered: 192.0.2.55:53 (i/o timeout)"
Mon Sep 21 12:00:11 MDT 2026: [dns] OK done queries=1 answered=0 total_time=1.003585459s
```

With no `@server`, every configured server is listed in that message, each with its own reason - which is how you find out that the first one is down and the second is doing all the work.

### JSON

```bash
shint dns example.com A @1.1.1.1 --json
```

```json
{
  "input_params": { ... },
  "module_name": "dns",
  "stats": [
    {
      "name": "example.com.",
      "type": "A",
      "nameserver": "1.1.1.1:53",
      "transport": "udp",
      "rcode": "NOERROR",
      "flags": ["qr", "rd", "ra"],
      "success": true,
      "answers": [
        { "name": "example.com.", "type": "A", "ttl": 120, "data": "104.20.23.154" },
        { "name": "example.com.", "type": "A", "ttl": 120, "data": "172.66.147.243" }
      ],
      "authority": [],
      "additional_count": 0,
      "sent_unixtime_µs": 1790013612552117,
      "recv_unixtime_µs": 1790013612571858,
      "time_taken_µs": 19715
    }
  ],
  "end_time_unixtime_µs": ...,
  "start_time_unixtime_µs": ...,
  "total_time_taken_µs": ...,
  "error": ""
}
```

Like [`cidr`](cidr.md), the document has no `dns_lookup`: `dns` *is* the lookup. `input_params.host` is the name asked and `data` the types. Each question is one entry in `stats`; `answers` and `authority` are always lists (empty, never `null`), `rcode` keeps the usual capitals, and a TXT record also carries `strings` with its character strings kept apart. A failed question has `success: false` and the reason in `error`; when no server answered, `server` is absent and `skipped_servers` lists each one with why.

## Exit status

`0` when every question was answered with at least one record of the type asked. `1` when any question was not: the name does not exist (`NXDOMAIN`), it exists but has no record of that type (the message says so: an alias whose target has none is called out), the server failed (`SERVFAIL`) or refused (`REFUSED`), the reply was malformed, no server answered in time, an `@server` name did not resolve, or the run was cut short by `Ctrl+C`. `2` for using it wrongly: an unknown type, an invalid name (a label over 63 characters, a name over 253, a space), an invalid `@server`, more than one `@server`, a type given with an IP address, `-4` with `-6`.

## Good to know

- **A reply is believed only if it answers this question.** The query carries a random ID; a reply with another ID, that is not a response, or that repeats a different question is ignored and the wait goes on, so a stray or forged packet cannot end - or answer - your query. A timeout then says how many replies were ignored. (Over UDP the socket is also connected to the server, so the operating system drops datagrams from anywhere else.)
- **UDP first, TCP when needed.** The query announces EDNS with a 1232-byte buffer, the size that avoids IP fragmentation. If the answer still does not fit, the server sets the `tc` flag and shint asks again over TCP (`retried=tcp`). A server that rejects EDNS (some old ones answer `FORMERR`) is asked again without it. `--tcp` skips UDP altogether.
- **No DNSSEC validation.** The `ad` flag is shown when the server says it validated the answer, but shint does not check signatures itself.
- **Internationalized names** must be written in their `xn--` (punycode) form (`xn--bcher-kva.de`); a name with non-ASCII characters is refused rather than sent as bytes no server would match.
- **What it cannot ask for**: `ANY`, zone transfers (`AXFR`) and record types outside the list above.
- **`--count` repeats the whole question**, and `Ctrl+C` stops the run and shows how far it got, like the other commands.
