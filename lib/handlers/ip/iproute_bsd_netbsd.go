//go:build netbsd

package ip

// NetBSD has no cloned or scoped routes in the sense macOS has, and no
// broadcast route flag: its subnet broadcast address is not in the routing
// table (seen on NetBSD 11.1).
const (
	rtfWasCloned = 0
	rtfIfScope   = 0
	rtfBroadcast = 0
)
