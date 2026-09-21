//go:build unix

package lib

import "syscall"

// descriptorLimit is how many file descriptors this process may have open: its
// soft RLIMIT_NOFILE. Go raises the soft limit to the hard limit when a program
// starts, so this is the ceiling the process really has - which is low only where
// the hard limit is (a container, a CI runner, a script that ran ulimit -n).
func descriptorLimit() (limit uint64, ok bool) {
	var l syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &l); err != nil {
		return 0, false
	}
	return uint64(l.Cur), true //nolint:unconvert // Rlimit.Cur is an int64 on some platforms
}
