//go:build dragonfly || freebsd || openbsd

package ip

import "golang.org/x/sys/unix"

// The BSDs have no cloned or scoped routes in the sense macOS has. A subnet's
// broadcast address is a host route of its own on OpenBSD (netstat -rn's UHb),
// not a route anyone looks for: it is left out, as macOS's (an ARP entry with
// the broadcast MAC there) already is.
const (
	rtfWasCloned = 0
	rtfIfScope   = 0
	rtfBroadcast = unix.RTF_BROADCAST
)
