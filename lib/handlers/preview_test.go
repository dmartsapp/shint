package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPreviewBytesKeepsPrintableTextAsItIs(t *testing.T) {
	for _, s := range []string{"hello shint", `{"status":"ok"}`, "GET / HTTP/1.1", "naïve café", "日本語のテキスト", "with \"quotes\" and \\ backslash", ""} {
		if got := previewBytes([]byte(s)); got != s {
			t.Errorf("previewBytes(%q) = %q, want it unchanged", s, got)
		}
	}
	if got := previewBytes([]byte("trailing newlines are trimmed\r\n\r\n")); got != "trailing newlines are trimmed" {
		t.Errorf("got %q", got)
	}
}

func TestPreviewBytesEscapesWhatWouldMisleadOrHarm(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"binary, invalid UTF-8":           {"\xff\xaa\xbb", `\xff\xaa\xbb`},
		"a terminal escape":               {"\x1b[31mred\x1b[0m", `\x1b[31mred\x1b[0m`},
		"a line break forging a log line": {"ok\n[listen-tcp] OK fake", `ok\n[listen-tcp] OK fake`},
		"tab and carriage return":         {"a\tb\rc", `a\tb\rc`},
		"NUL and DEL":                     {"a\x00b\x7fc", `a\x00b\x7fc`},
		"a C1 control (CSI, 0x9b)":        {"a\u009bb", `a\u009bb`},
		"a byte order mark":               {"\ufeffx", `\ufeffx`},
	} {
		if got := previewBytes([]byte(tc.in)); got != tc.want {
			t.Errorf("%s: previewBytes(%q) = %q, want %q", name, tc.in, got, tc.want)
		}
	}
	// the magic packet that started this: six 0xFF bytes, then a MAC sixteen times
	if got := previewBytes(MagicPacket([]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}))[:24]; got != `\xff\xff\xff\xff\xff\xff\xaa\xbb`[:24] {
		t.Errorf("magic packet preview starts %q", got)
	}
}

func TestPreviewBytesTruncatesOnACharacterBoundary(t *testing.T) {
	long := strings.Repeat("a", 118) + "日本語" // the 120-byte cut falls inside 日
	got := previewBytes([]byte(long))
	if !strings.HasSuffix(got, "...") || strings.Contains(got, `\x`) || !utf8.ValidString(got) {
		t.Errorf("preview cut mid-character or not marked truncated: %q", got)
	}
	if got != strings.Repeat("a", 118)+"..." {
		t.Errorf("got %q", got)
	}
	plain := previewBytes([]byte(strings.Repeat("b", 500)))
	if plain != strings.Repeat("b", 120)+"..." {
		t.Errorf("a long ASCII payload: %q", plain)
	}
}

// Whatever it is fed, the preview is one line of printable ASCII-safe text
// (escapes are ASCII) and valid UTF-8, so it can be logged and JSON-encoded.
func TestPreviewBytesIsAlwaysOneSafeLine(t *testing.T) {
	for i := 0; i < 256; i++ {
		in := []byte{byte(i), 'x', byte(255 - i)}
		got := previewBytes(in)
		if !utf8.ValidString(got) || strings.ContainsAny(got, "\n\r\x1b\x00") {
			t.Errorf("previewBytes(% x) = %q is not one safe line", in, got)
		}
		if _, err := json.Marshal(got); err != nil {
			t.Errorf("not JSON-encodable: %v", err)
		}
	}
}
