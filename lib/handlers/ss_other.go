//go:build !linux && !windows

package handlers

import (
	"fmt"
	"runtime"

	"github.com/dmartsapp/shint/v4/lib"
)

// platformSockets: ss reads the socket tables of Linux and Windows only so
// far; the BSDs are #105 and will each get a file of their own, as Linux and
// Windows have. AIX builds this file too but never reaches it: the AIX binary
// has no ss command (main_ss.go).
func platformSockets() ([]lib.SSStats, error) {
	return nil, fmt.Errorf("ss is not supported on %s yet: it reads the socket tables of Linux and Windows", runtime.GOOS)
}
