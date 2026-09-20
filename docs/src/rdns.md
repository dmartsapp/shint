---
title: rdns
lead: Reverse DNS - which host names does an IP address map back to? The PTR lookup, without dig.
description: shint rdns does a reverse DNS lookup for an IP address (or for every address of a host name), over IPv4 and IPv6, and reports the names or their absence.
section: Commands
order: 8
nav: rdns
---

## What it does

Forward DNS turns a name into addresses. **Reverse DNS** goes the other way: given `8.8.8.8`, which names is it known by? The answer lives in **PTR** records, filed under a special name made from the address written backwards (`8.8.8.8.in-addr.arpa.`), and it is what shows up in mail-server checks, log analysis and "whose address is this?" questions.

`shint rdns` does that lookup as an ordinary DNS query through your system's resolver, so it needs no privileges. Like every shint command, it takes either an **IP address** or a **host name** - a name is resolved first, and every address it resolves to is looked up, over IPv4 and IPv6.

## Syntax

```bash
shint rdns <ip-or-host> [--count N] [--timeout S] [--delay MS] [--json]
```

The [shared flags](usage.md#flags-shared-by-every-command) apply: `--timeout` is how long each lookup gets, `--count` repeats them, `--delay` pauses before each. There are no flags of its own.

## Examples

### An address with a name

```bash
shint rdns 8.8.8.8
```

```text
Sun Sep 20 17:30:33 MDT 2026: [rdns] OK dns resolved host=8.8.8.8 addresses=1 ips=[8.8.8.8] time=109.292µs
Sun Sep 20 17:30:34 MDT 2026: [rdns] OK reverse lookup address=8.8.8.8 query=8.8.8.8.in-addr.arpa. attempt=1/1 names=[dns.google.] time=5.165042ms
Sun Sep 20 17:30:34 MDT 2026: [rdns] OK done lookups=1 resolved=1 total_time=1.006806792s
```

`names=[dns.google.]` is the answer. `query=` is the name that was actually asked for - the address reversed under `in-addr.arpa.` - so the result can be checked against `dig -x 8.8.8.8` or any other tool. The trailing dot is DNS's way of writing a name in full.

### An address with no name

```bash
shint rdns 192.0.2.1
```

```text
Sun Sep 20 17:30:34 MDT 2026: [rdns] OK dns resolved host=192.0.2.1 addresses=1 ips=[192.0.2.1] time=98.541µs
Sun Sep 20 17:30:35 MDT 2026: [rdns] ERROR reverse lookup failed address=192.0.2.1 query=1.2.0.192.in-addr.arpa. attempt=1/1 time=4.320042ms error="lookup 192.0.2.1: no such host"
Sun Sep 20 17:30:35 MDT 2026: [rdns] OK done lookups=1 resolved=0 total_time=1.005892916s
```

Many addresses have no PTR record, and knowing that is often the point of asking. It is reported as a **failed check**: an `ERROR` line and exit status `1`.

### A host name: every address is looked up

```bash
shint rdns one.one.one.one
```

```text
Sun Sep 20 17:30:35 MDT 2026: [rdns] OK dns resolved host=one.one.one.one addresses=4 ips=[2606:4700:4700::1001,2606:4700:4700::1111,1.1.1.1,1.0.0.1] time=5.194167ms
Sun Sep 20 17:30:36 MDT 2026: [rdns] OK reverse lookup address=2606:4700:4700::1001 query=1.0.0.1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.7.4.0.0.7.4.6.0.6.2.ip6.arpa. attempt=1/1 names=[one.one.one.one.] time=9.695625ms
Sun Sep 20 17:30:37 MDT 2026: [rdns] OK reverse lookup address=2606:4700:4700::1111 query=1.1.1.1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.7.4.0.0.7.4.6.0.6.2.ip6.arpa. attempt=1/1 names=[one.one.one.one.] time=43.543958ms
Sun Sep 20 17:30:38 MDT 2026: [rdns] OK reverse lookup address=1.1.1.1 query=1.1.1.1.in-addr.arpa. attempt=1/1 names=[one.one.one.one.] time=39.611ms
Sun Sep 20 17:30:39 MDT 2026: [rdns] OK reverse lookup address=1.0.0.1 query=1.0.0.1.in-addr.arpa. attempt=1/1 names=[one.one.one.one.] time=2.445208ms
Sun Sep 20 17:30:39 MDT 2026: [rdns] OK done lookups=4 resolved=4 total_time=4.107195958s
```

The name resolves to four addresses (two IPv6, two IPv4), and each one is asked. The IPv6 `query=` names are long: one label per hex digit of the address, in reverse, under `ip6.arpa.` (see [RFC 3596](https://www.rfc-editor.org/rfc/rfc3596)). The lookups are a second apart because of the default `--delay`; use `--delay 0` to run them back to back.

### JSON

```bash
shint rdns 8.8.8.8 --json
```

```json
{
  "input_params": { ... },
  "module_name": "rdns",
  "dns_lookup": { ... },
  "stats": [
    {
      "address": "8.8.8.8",
      "query": "8.8.8.8.in-addr.arpa.",
      "names": [
        "dns.google."
      ],
      "success": true,
      "sent_unixtime_µs": 1789947410638062,
      "recv_unixtime_µs": 1789947410644229,
      "time_taken_µs": 6166
    }
  ],
  "end_time_unixtime_µs": ...,
  "start_time_unixtime_µs": ...,
  "total_time_taken_µs": ...,
  "error": ""
}
```

`dns_lookup` describes the first step (resolving the argument, which for an address is trivial). `names` is always a list - empty for an address with no name, never `null` - and a failed lookup has `success: false` with the reason in `error`.

## Exit status

`0` when every address had at least one name. `1` when any address had none, a lookup failed or timed out, or the argument was a name that did not resolve. `2` for a bad flag value or a missing argument.

## Good to know

- **A PTR record is set by whoever owns the address block** (your ISP, your cloud provider, your own DNS), not by the machine itself, so it may be missing, generic (`ec2-...compute.amazonaws.com`) or out of date. It says nothing about whether the name maps *forward* to the same address: that "forward-confirmed" check is not done here.
- **An address can have several names** - the PTR set - and all of them are listed.
- **IPv4-mapped IPv6 addresses** (`::ffff:192.0.2.1`) are looked up as the IPv4 address they carry.
- It uses the **system's DNS servers**, like the other commands, so a private address may have a name in your organization's DNS that public resolvers do not know.
