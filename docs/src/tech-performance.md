---
title: Performance
lead: How fast and how lean every shint release is, measured the same way each time, on the machine that measured it.
description: shint's performance history - wall time, CPU time and peak memory of every command scenario, the binary's size and the tests' times, release by release and machine by machine.
section: Technical
order: 8
nav: Performance
---

Every release is [profiled](tech-testing.md#profiling) before it is published, and the run is committed with it in `.profiling/`. This page draws that history. Pick a **machine** - timings only compare between runs on the same one - a **metric**, and two **builds** to compare; every scenario is charted below over all the builds that machine has measured.

:::performance
:::

## Reading it

- **Medians, with the spread.** Each scenario runs several times after an unmeasured warm-up; the line is the median, the band the fastest to the slowest run. A change smaller than the runs' own spread is marked `~` in the comparison: it is noise, not a result.
- **What is measured** is shint's own process, from the operating system's accounting of it: from start to exit (wall time), the processor time it used (user plus system), and its peak memory. The operating system's own counters measure other things, at other points, and are not used.
- **Backfill** points (hollow) are releases published before profiling existed, measured later from their published binaries: the scenarios and the binary's size only.
- **Machines** are identified by a hash of their hardware's identifier, never by name. A machine's description - model, processor, cores, memory, system, Go - is what it reported in its latest run.

## The same from the command line

```bash
make profile-history                        # this machine: every scenario across its builds, with a trend
make profile-history METRIC=rss LAST=6      # peak memory, the last six builds
make profile-machines                       # every machine that has measured something
make profile-compare A=v4.2.2 B=v4.2.3      # two builds, on the same machine when both have run on it
make profile-ab A=v4.2.3 B=HEAD             # two builds head to head, now, interleaved - the one to trust
```

Runs from different days carry the machine's drift between them - see [interleaved, because a machine drifts](tech-testing.md#profiling). To judge a change, measure both builds together with `make profile-ab`.

A new run shows up here once it is committed and the site is rebuilt (`python3 docs/build.py`) - which `make release` does for every release.
