//go:build linux

package ip

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"

	"github.com/dmartsapp/shint/v4/lib/handlers/internal/term"
)

// platformRoutes reads Linux's routing tables from /proc: IPv4 from
// /proc/net/route, IPv6 from /proc/net/ipv6_route (absent on a kernel without
// IPv6). Any user may read them (Android apps may be refused).
func platformRoutes() ([]sysRoute, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) && runtime.GOOS == "android" {
			return nil, errors.New("could not read /proc/net/route: permission denied (apps built for Android 10 and later may not read it)")
		}
		return nil, fmt.Errorf("could not read /proc/net/route: %w", term.UnwrapPath(err))
	}
	routes, err := parseProcRoute(f, binary.NativeEndian)
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("could not read /proc/net/route: %w", err)
	}
	f6, err := os.Open("/proc/net/ipv6_route")
	if errors.Is(err, fs.ErrNotExist) {
		return routes, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read /proc/net/ipv6_route: %w", term.UnwrapPath(err))
	}
	defer func() { _ = f6.Close() }()
	routes6, err := parseProcIPv6Route(f6)
	if err != nil {
		return nil, fmt.Errorf("could not read /proc/net/ipv6_route: %w", err)
	}
	return append(routes, routes6...), nil
}
