//go:build !linux && !windows && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package ip

import (
	"fmt"
	"runtime"
)

// platformNeighbours: ip arp reads the neighbour tables of Linux, macOS, the
// BSDs and Windows so far.
func platformNeighbours() ([]sysNeighbour, error) {
	return nil, fmt.Errorf("ip arp is not supported on %s yet: it reads the neighbour tables of Linux, macOS, the BSDs and Windows", runtime.GOOS)
}
