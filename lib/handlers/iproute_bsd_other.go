//go:build dragonfly || freebsd || netbsd || openbsd

package handlers

// The BSDs have no cloned or scoped routes in the sense macOS has.
const (
	rtfWasCloned = 0
	rtfIfScope   = 0
)
