package listen

import (
	"fmt"
	"net"
	"strings"

	"github.com/dmartsapp/shint/v4/lib"
)

// A listener binds every interface by default - checking that a port is
// reachable from another machine is what listen is for - and it has no
// authentication. So when it is reachable from the network it says so as it
// starts (#102), and how to keep it to this machine.

// reachableFromNetwork is whether a listener bound to bind accepts
// connections from other machines: every address but loopback - an
// unspecified one (0.0.0.0, ::, or none: every interface) and any specific
// address of the machine's. A name other than localhost is taken to be one.
func reachableFromNetwork(bind string) bool {
	b := strings.Trim(strings.TrimSpace(bind), "[]")
	if b == "" {
		return true
	}
	if strings.EqualFold(b, "localhost") {
		return false
	}
	ip := net.ParseIP(b)
	return ip == nil || !ip.IsLoopback()
}

// exposureNotice prints, in text mode, the line that says a listener can be
// reached from other machines.
func exposureNotice(module, bind, addr string) {
	if !reachableFromNetwork(bind) {
		return
	}
	fmt.Println(lib.LogWithLevel(module, lib.LevelWarn, "reachable from other machines "+lib.Fields("address", addr,
		"note", "no authentication: anyone who can reach this port can use it; --bind 127.0.0.1 keeps it to this machine")))
}
