package lib

import (
	"context"
	"net"
	"time"
)

const (
	DATETIMEFORMAT string = "Mon Jan 2 15:04:05 MST 2006"
	NetworkType    string = "ip4" // other networks are ip which includes both v4 and v6, and ip6 which is only v6
	Protocol       string = "tcp"
)

func ResolveName(ctx context.Context, name string) ([]string, error) {
	var resolver net.Resolver
	ipaddresses, err := resolver.LookupIP(ctx, NetworkType, name)
	var addresses = make([]string, 0)
	for _, address := range ipaddresses {
		addresses = append(addresses, address.String())
	}
	return addresses, err
}

func LogWithTimestamp(log string, iserror bool) string {
	if !iserror {
		return time.Now().Format(DATETIMEFORMAT) + ": " + log
	}
	return time.Now().Format(DATETIMEFORMAT) + ": Error! " + log

}
