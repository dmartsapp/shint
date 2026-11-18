package lib

import (
	"slices"
	"strings"
	"testing"
)

func TestParsePorts(t *testing.T) {
	for _, c := range []struct {
		spec string
		want []int
	}{
		{"22", []int{22}},
		{"22,80,443", []int{22, 80, 443}},
		{"443,22,80", []int{22, 80, 443}},
		{"8000-8003", []int{8000, 8001, 8002, 8003}},
		{" 22 , 80-81 ", []int{22, 80, 81}},
		{"80,80,79-81", []int{79, 80, 81}},
		{"1,65535", []int{1, 65535}},
		{"5-5", []int{5}},
		{"0022", []int{22}},
	} {
		got, err := ParsePorts("ports", c.spec)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("ParsePorts(%q) = %v, %v; want %v", c.spec, got, err, c.want)
		}
	}
	if got, err := ParsePorts("ports", "1-65535"); err != nil || len(got) != 65535 {
		t.Errorf("1-65535: %d ports, %v", len(got), err)
	}
}

func TestParsePortsRefuses(t *testing.T) {
	for _, c := range []struct{ spec, says string }{
		{"", "empty"},
		{"   ", "empty"},
		{"22,,80", "empty item"},
		{"22,", "empty item"},
		{"0", "not a port"},
		{"65536", "not a port"},
		{"99999", "not a port"},
		{"123456", "not a port number"},
		{"-5", "not a port number"},
		{"+22", "not a port number"},
		{"22-", "not a port number"},
		{"1-2-3", "not a port number"},
		{"ssh", "not a port number"},
		{"0x16", "not a port number"},
		{"22;id", "not a port number"},
		{"80-22", "runs backwards (write 22-80)"},
		{"2\u00a02", "not a port number"},
		{strings.Repeat("1,", maxPortsSpec), "characters long"},
	} {
		_, err := ParsePorts("ports", c.spec)
		if err == nil || !strings.Contains(err.Error(), c.says) || !strings.HasPrefix(err.Error(), "--ports") {
			t.Errorf("ParsePorts(%.40q): %v; want an error naming --ports that says %q", c.spec, err, c.says)
		}
	}
}

func TestPortsText(t *testing.T) {
	for _, c := range []struct {
		ports []int
		want  string
	}{
		{nil, ""},
		{[]int{22}, "22"},
		{[]int{22, 80, 443}, "22,80,443"},
		{[]int{22, 80, 443, 8000, 8001, 8002}, "22,80,443,8000-8002"},
		{[]int{1, 2}, "1-2"},
		{PortRange(1, 65535), "1-65535"},
	} {
		if got := PortsText(c.ports); got != c.want {
			t.Errorf("PortsText(%v) = %q, want %q", c.ports, got, c.want)
		}
	}
	// What ParsePorts reads, PortsText writes back the same.
	ports, _ := ParsePorts("ports", "8000-8002,22,443,80,8001")
	if got := PortsText(ports); got != "22,80,443,8000-8002" {
		t.Errorf("round trip: %q", got)
	}
}
