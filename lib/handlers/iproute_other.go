//go:build !linux && !windows && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package handlers

import (
	"fmt"
	"runtime"
)

// platformRoutes: ip route reads the routing tables of Linux, macOS, the BSDs
// and Windows so far.
func platformRoutes() ([]sysRoute, error) {
	return nil, fmt.Errorf("ip route is not supported on %s yet: it reads the routing tables of Linux, macOS, the BSDs and Windows", runtime.GOOS)
}
