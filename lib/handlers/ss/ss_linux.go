//go:build linux

package ss

import (
	"encoding/binary"
	"os"

	"github.com/dmartsapp/shint/v4/lib"
)

// platformSockets reads Linux's socket tables from /proc (Android too: Go
// builds it as Linux, though apps built for Android 10 and later may not read the tables).
func platformSockets() ([]lib.SSStats, error) {
	return readProcSockets("/proc", binary.NativeEndian, os.Geteuid())
}
