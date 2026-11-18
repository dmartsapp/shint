package handlers

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

// This file reads Linux's routing tables from /proc. It has no build tag, so
// its tests run everywhere; iproute_linux.go points it at the real files.

// The route flags /proc prints (include/uapi/linux/route.h, ipv6_route.h).
const (
	rtfUp      = 0x0001
	rtfGateway = 0x0002
	rtfReject  = 0x0200
	rtfCache   = 0x01000000 // IPv6: a cached clone, not a route
	rtfAnycast = 0x00100000 // IPv6: a local anycast address
	rtfLocal   = 0x80000000 // IPv6: a local address (the local table)
)

// parseProcRoute reads /proc/net/route: a header line, then per IPv4 route
// "Iface Destination Gateway Flags RefCnt Use Metric Mask ..." with the
// addresses as 32-bit hex words in the kernel's byte order. A route that is
// not up is left out; a line that does not fit is an error.
func parseProcRoute(r io.Reader, order binary.ByteOrder) ([]sysRoute, error) {
	sc := bufio.NewScanner(r)
	var out []sysRoute
	for line := 0; sc.Scan(); line++ {
		f := strings.Fields(sc.Text())
		if line == 0 {
			if len(f) == 0 || f[0] != "Iface" {
				return nil, errors.New("the first line is not the table's header")
			}
			continue
		}
		if len(f) == 0 {
			continue
		}
		if len(f) < 8 {
			return nil, fmt.Errorf("line %d: %d fields, want at least 8", line+1, len(f))
		}
		dst, err1 := procIPv4(f[1], order)
		gw, err2 := procIPv4(f[2], order)
		mask, err3 := procIPv4(f[7], order)
		flags, err4 := strconv.ParseUint(f[3], 16, 32)
		metric, err5 := strconv.Atoi(f[6])
		if err := errors.Join(err1, err2, err3, err4, err5); err != nil {
			return nil, fmt.Errorf("line %d: %w", line+1, err)
		}
		if flags&rtfUp == 0 {
			continue
		}
		bits, ok := prefixBits(mask.AsSlice())
		if !ok {
			return nil, fmt.Errorf("line %d: mask %s is not a prefix", line+1, mask)
		}
		route := sysRoute{Prefix: netip.PrefixFrom(dst, bits).Masked(), Iface: f[0], Metric: metric, HasMetric: true, Reject: flags&rtfReject != 0}
		if flags&rtfGateway != 0 {
			route.Gateway = gw
		}
		if route.Iface == "*" {
			route.Iface = ""
		}
		out = append(out, route)
	}
	return out, sc.Err()
}

// parseProcIPv6Route reads /proc/net/ipv6_route: no header, and per route
// "dest dest_len src src_len next_hop metric refcnt use flags iface", all hex,
// addresses in network order. It holds every table, so local addresses,
// anycast addresses, cached clones, multicast and the kernel's own "null"
// route (a reject route with the highest metric) are left out: what remains is
// what ip -6 route shows.
func parseProcIPv6Route(r io.Reader) ([]sysRoute, error) {
	sc := bufio.NewScanner(r)
	var out []sysRoute
	for line := 1; sc.Scan(); line++ {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		if len(f) != 10 {
			return nil, fmt.Errorf("line %d: %d fields, want 10", line, len(f))
		}
		dst, err1 := procIPv6(f[0])
		bits, err2 := strconv.ParseUint(f[1], 16, 8)
		gw, err3 := procIPv6(f[4])
		metric, err4 := strconv.ParseUint(f[5], 16, 32)
		flags, err5 := strconv.ParseUint(f[8], 16, 32)
		if err := errors.Join(err1, err2, err3, err4, err5); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if bits > 128 {
			return nil, fmt.Errorf("line %d: prefix length %d", line, bits)
		}
		reject := flags&rtfReject != 0
		if flags&rtfUp == 0 || flags&(rtfCache|rtfAnycast|rtfLocal) != 0 || dst.IsMulticast() || (reject && metric == 0xffffffff) {
			continue
		}
		route := sysRoute{Prefix: netip.PrefixFrom(dst, int(bits)).Masked(), Iface: f[9], Metric: int(metric), HasMetric: true, Reject: reject}
		if flags&rtfGateway != 0 {
			route.Gateway = gw
			if gw.IsLinkLocalUnicast() {
				route.Gateway = gw.WithZone(f[9])
			}
		}
		if reject && route.Iface == "lo" {
			route.Iface = "" // a reject route is not on any interface
		}
		out = append(out, route)
	}
	return out, sc.Err()
}

// procIPv4 decodes an IPv4 address written as one 32-bit hex word in the
// kernel's byte order.
func procIPv4(s string, order binary.ByteOrder) (netip.Addr, error) {
	if len(s) != 8 {
		return netip.Addr{}, fmt.Errorf("address %q is not 8 hex digits", s)
	}
	w, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("address %q is not hex", s)
	}
	var b [4]byte
	order.PutUint32(b[:], uint32(w))
	return netip.AddrFrom4(b), nil
}

// procIPv6 decodes an IPv6 address written as 32 hex digits in network order.
func procIPv6(s string) (netip.Addr, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 {
		return netip.Addr{}, fmt.Errorf("address %q is not 32 hex digits", s)
	}
	return netip.AddrFrom16([16]byte(b)), nil
}

// prefixBits is the length of a netmask, if it is one (contiguous ones).
func prefixBits(mask []byte) (int, bool) {
	bits, seenZero := 0, false
	for _, b := range mask {
		for i := 7; i >= 0; i-- {
			if b&(1<<i) != 0 {
				if seenZero {
					return 0, false
				}
				bits++
			} else {
				seenZero = true
			}
		}
	}
	return bits, true
}
