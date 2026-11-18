//go:build darwin

package handlers

import "golang.org/x/sys/unix"

// macOS's flags for a host route cloned from a network route, and for a route
// scoped to its interface.
const (
	rtfWasCloned = unix.RTF_WASCLONED
	rtfIfScope   = unix.RTF_IFSCOPE
)
