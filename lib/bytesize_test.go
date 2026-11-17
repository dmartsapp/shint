package lib

import "testing"

func TestParseByteSize(t *testing.T) {
	for in, want := range map[string]int64{
		"0": 0, "1": 1, "10485760": 10 << 20, "10M": 10 << 20, "10m": 10 << 20, "10MB": 10 << 20, "10MiB": 10 << 20,
		"512K": 512 << 10, "512kb": 512 << 10, "1G": 1 << 30, " 2M ": 2 << 20, "1024G": 1 << 40,
	} {
		got, err := ParseByteSize("max-body", in)
		if err != nil || got != want {
			t.Errorf("ParseByteSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "-1", "abc", "10X", "1.5M", "M", "1025G", "99999999999999999999"} {
		if _, err := ParseByteSize("max-body", in); err == nil {
			t.Errorf("ParseByteSize(%q) accepted", in)
		}
	}
}
