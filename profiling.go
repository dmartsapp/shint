//go:build profiling

package main

import (
	"os"
	"runtime"
	"runtime/pprof"
)

// Only in a binary built with -tags profiling, as make profile builds one
// (test/profile/profile.py): SHINT_CPUPROFILE names a file for a CPU profile of
// the whole run, and SHINT_MEMPROFILE one for the memory allocated during it,
// both written by stopProfiling as the process exits. Release binaries are built
// without the tag and contain none of this.
func init() {
	if path := os.Getenv("SHINT_CPUPROFILE"); path != "" {
		if f, err := os.Create(path); err == nil {
			if pprof.StartCPUProfile(f) == nil {
				prev := stopProfiling
				stopProfiling = func() {
					pprof.StopCPUProfile()
					_ = f.Close()
					prev()
				}
			} else {
				_ = f.Close()
			}
		}
	}
	if path := os.Getenv("SHINT_MEMPROFILE"); path != "" {
		prev := stopProfiling
		stopProfiling = func() {
			prev()
			f, err := os.Create(path)
			if err != nil {
				return
			}
			defer func() { _ = f.Close() }()
			runtime.GC()
			_ = pprof.Lookup("allocs").WriteTo(f, 0)
		}
	}
}
