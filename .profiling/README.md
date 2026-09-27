# Profiling history

How fast and how lean each shint release is, measured the same way every time, so the history shows what improved and where to look next. Written by `make profile` (`test/profile/profile.py`); how it works: [Profiling](https://dmartsapp.github.io/shint/docs/tech-testing.html#profiling).

```
.profiling/
  vX.Y.Z/<UTC time>/        one run: make release profiles every release and commits the run with it
    summary.json            the machine, the scenarios, binary sizes, test and battery times
    test-cpu-*.pprof        CPU and memory profiles of the lib and lib/handlers tests,
    test-mem-*.pprof        each with a .top.txt: the functions that took the most
    cmd-cpu-*.pprof         the binary's own profile in each scenario (from the `profiling`
    cmd-mem-*.pprof         build tag's hook, in releases that have it), also with .top.txt
  dev/                      your own runs (make profile without TAG); git ignores them
```

- **`kind`** in `summary.json`: `release` (made by `make release` on the release's own tree), or `backfill` (a release published before profiling existed, measured later from its published binary: scenarios and sizes only).
- **The machine** comes first in every run. Timings compare only between runs with the same `machine.id` (a hash of the hardware's identifier, not its name), and best on the same power source.
- **What is measured** is shint's own processes, from the operating system's accounting of each one: wall time, CPU time and peak memory. The operating system's own disk and network counters measure other things, and are not used.

```bash
make profile-compare A=v4.2.3 B=v4.3.0     # a tag's latest run, or a run's folder
go tool pprof -http=: .profiling/v4.3.0/<time>/cmd-cpu-web.pprof
```
