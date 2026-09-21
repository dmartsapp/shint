//go:build !unix

package lib

// descriptorLimit: outside Unix (Windows has no per-process descriptor limit of
// this kind) nothing lowers the number of attempts.
func descriptorLimit() (limit uint64, ok bool) { return 0, false }
