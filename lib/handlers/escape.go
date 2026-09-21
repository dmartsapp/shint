package handlers

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// escapeBytes renders bytes that came from somewhere else - a packet, a DNS
// record - as one line that is safe to print: printable text as it is, and
// everything else escaped: \n \r \t for line breaks and tabs, \xNN for other
// control characters and for bytes that are not valid UTF-8, \uNNNN for
// non-printable characters above 0x7F. Written raw, such bytes garble the
// terminal, let escape sequences act on it and let a line break forge a log
// line; escaped they can do none of those. It neither trims nor truncates: see
// previewBytes for that.
func escapeBytes(b []byte) string {
	var sb strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&sb, "\\x%02x", b[0])
		case r == '\n':
			sb.WriteString(`\n`)
		case r == '\r':
			sb.WriteString(`\r`)
		case r == '\t':
			sb.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&sb, "\\x%02x", r)
		case !unicode.IsPrint(r):
			fmt.Fprintf(&sb, "\\u%04x", r)
		default:
			sb.WriteRune(r)
		}
		b = b[size:]
	}
	return sb.String()
}
