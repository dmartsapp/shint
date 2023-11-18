package lib

import (
	"net"
)

func ResolveName(name string) ([]net.IP, error) {
	ipaddresses, err := net.LookupIP(name)
	return ipaddresses, err
}
