package lib

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
)

// NetworkType is the address family a run resolves and checks: "ip" (the
// default) means both IPv4 and IPv6 - a host with both records is checked over
// both, and every address is reported on its own - "ip4" and "ip6" restrict it
// to one, as the -4 and -6 flags do. It is what net.Resolver.LookupIP takes as
// its network argument, so ResolveName follows it with nothing more to do. Set
// once at startup with SetIPFamily; nothing changes it while a command runs.
var NetworkType = "ip"

// SetIPFamily applies the -4 and -6 flags. Asking for both is a mistake, not a
// way to say "either".
func SetIPFamily(ipv4Only, ipv6Only bool) error {
	switch {
	case ipv4Only && ipv6Only:
		return errors.New("-4/--ipv4 and -6/--ipv6 cannot be used together")
	case ipv4Only:
		NetworkType = "ip4"
	case ipv6Only:
		NetworkType = "ip6"
	default:
		NetworkType = "ip"
	}
	return nil
}

// FamilyName is "IPv4" or "IPv6" when the run is restricted to one, and
// "IPv4 or IPv6" when it is not.
func FamilyName() string {
	switch NetworkType {
	case "ip4":
		return "IPv4"
	case "ip6":
		return "IPv6"
	}
	return "IPv4 or IPv6"
}

// FamilyAllows reports whether ip is an address the run may use.
func FamilyAllows(ip net.IP) bool {
	switch NetworkType {
	case "ip4":
		return ip.To4() != nil
	case "ip6":
		return ip.To4() == nil
	}
	return true
}

// DialNetwork turns a plain "tcp" or "udp" into "tcp4"/"tcp6" (or "udp4"/"udp6")
// when the run is restricted to one family, so a dialer that picks its own
// address - the HTTP client's does - cannot wander into the other one.
func DialNetwork(network string) string {
	if (network == "tcp" || network == "udp") && (NetworkType == "ip4" || NetworkType == "ip6") {
		return network + NetworkType[2:]
	}
	return network
}

// HostFamilyConflict says what is wrong when host is an IP address that the
// chosen family rules out: an IPv6 address with -4, an IPv4 one with -6. A host
// name is never a conflict - it is resolved for that family - and neither is an
// address of the right family. nil means carry on.
func HostFamilyConflict(host string) error {
	if NetworkType == "ip" {
		return nil
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return nil
	}
	if !FamilyAllows(net.IP(addr.AsSlice())) {
		return fmt.Errorf("%s is an %s address, but %s was requested", host, map[string]string{"ip4": "IPv6", "ip6": "IPv4"}[NetworkType], flagName())
	}
	return nil
}

func flagName() string {
	if NetworkType == "ip4" {
		return "-4/--ipv4"
	}
	return "-6/--ipv6"
}

// FamilyHint explains, in a few words, a failure whose cause is this machine's
// lack of IPv6 rather than anything about the target: the system cannot create
// an IPv6 socket at all (its kernel has IPv6 disabled - the error is "address
// family not supported by protocol"), or it has no route to IPv6 addresses. It
// returns "" for every other error and for IPv4 addresses.
func FamilyHint(err error) string {
	var op *net.OpError
	if !errors.As(err, &op) || op.Addr == nil {
		return ""
	}
	var ip net.IP
	switch a := op.Addr.(type) {
	case *net.TCPAddr:
		ip = a.IP
	case *net.UDPAddr:
		ip = a.IP
	case *net.IPAddr:
		ip = a.IP
	default:
		return ""
	}
	if ip == nil || ip.To4() != nil {
		return ""
	}
	switch {
	case errors.Is(err, syscall.EAFNOSUPPORT):
		return "this system cannot use IPv6; use -4 to check IPv4 only"
	case errors.Is(err, syscall.ENETUNREACH):
		return "this system has no route to IPv6 addresses; use -4 to check IPv4 only"
	}
	return ""
}

// ExplainError is err's text, with FamilyHint's explanation in parentheses when
// there is one.
func ExplainError(err error) string {
	if hint := FamilyHint(err); hint != "" {
		return err.Error() + " (" + hint + ")"
	}
	return err.Error()
}
