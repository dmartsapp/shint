//go:build darwin

package ip

import "golang.org/x/sys/unix"

// macOS's flags for a host route cloned from a network route, for a route
// scoped to its interface, and for a broadcast address.
const (
	rtfWasCloned = unix.RTF_WASCLONED
	rtfIfScope   = unix.RTF_IFSCOPE
	rtfBroadcast = unix.RTF_BROADCAST
)
