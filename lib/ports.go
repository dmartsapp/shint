package lib

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// maxPortsSpec bounds what --ports may be: every port from 1 to 65535 written
// out one by one, with commas, is under 400,000 characters, so anything longer
// is not a port list.
const maxPortsSpec = 400000

// ParsePorts reads a port list such as "22,80,443,8000-8100": port numbers and
// ranges, separated by commas. Spaces around an item are allowed; an empty
// item, a sign, or a port outside 1-65535 is not. The ports come back sorted,
// each once, however often they were given.
func ParsePorts(name, spec string) ([]int, error) {
	if len(spec) > maxPortsSpec {
		return nil, fmt.Errorf("--%s is %d characters long, more than any port list can be", name, len(spec))
	}
	if strings.TrimSpace(spec) == "" {
		return nil, fmt.Errorf("--%s is empty: give ports like 22,80,443 or a range like 8000-8100", name)
	}
	seen := make(map[int]bool)
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("--%s %q has an empty item: give ports like 22,80,443 or a range like 8000-8100", name, spec)
		}
		low, high, isRange := strings.Cut(item, "-")
		from, err := portNumber(name, low, item)
		if err != nil {
			return nil, err
		}
		to := from
		if isRange {
			if to, err = portNumber(name, high, item); err != nil {
				return nil, err
			}
			if from > to {
				return nil, fmt.Errorf("--%s: the range %q runs backwards (write %d-%d)", name, item, to, from)
			}
		}
		for p := from; p <= to; p++ {
			seen[p] = true
		}
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	slices.Sort(ports)
	return ports, nil
}

// portNumber reads one end of an item: decimal digits only, 1-65535.
func portNumber(name, text, item string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 5 || strings.Trim(text, "0123456789") != "" {
		return 0, fmt.Errorf("--%s: %q is not a port number or a range like 8000-8100", name, item)
	}
	n, _ := strconv.Atoi(text) // at most five digits: cannot fail or overflow
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("--%s: %d is not a port (ports are 1-65535)", name, n)
	}
	return n, nil
}

// PortRange is every port from first to last.
func PortRange(first, last int) []int {
	ports := make([]int, 0, last-first+1)
	for p := first; p <= last; p++ {
		ports = append(ports, p)
	}
	return ports
}

// PortsText writes sorted ports back as a short list, runs as ranges:
// [22 80 443 8000 8001 8002] is "22,80,443,8000-8002".
func PortsText(ports []int) string {
	var b strings.Builder
	for i := 0; i < len(ports); {
		j := i
		for j+1 < len(ports) && ports[j+1] == ports[j]+1 {
			j++
		}
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(ports[i]))
		if j > i {
			b.WriteByte('-')
			b.WriteString(strconv.Itoa(ports[j]))
		}
		i = j + 1
	}
	return b.String()
}
