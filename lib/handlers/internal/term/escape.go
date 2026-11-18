package term

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Escape renders bytes that came from somewhere else - a packet, a DNS
// record - as one line that is safe to print: printable text as it is, and
// everything else escaped: \n \r \t for line breaks and tabs, \xNN for other
// control characters and for bytes that are not valid UTF-8, \uNNNN for
// non-printable characters above 0x7F. Written raw, such bytes garble the
// terminal, let escape sequences act on it and let a line break forge a log
// line; escaped they can do none of those. It neither trims nor truncates: see
// Preview for that.
func Escape(b []byte) string {
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

// Preview renders a short, single-line, human-safe preview of received
// bytes for logging: trailing newlines trimmed, long payloads truncated (at a
// character boundary), printable text shown as it is and everything else
// escaped - \n \r \t for line breaks and tabs, \xNN for other control
// characters and for bytes that are not valid UTF-8, \uNNNN for non-printable
// characters above 0x7F.
//
// A payload is whatever a stranger sent. Written raw it garbles the terminal,
// lets terminal escape sequences (ESC [ ...) act on it, and lets a line break
// forge a log line of its own; escaped it can do none of those.
func Preview(b []byte) string {
	const maxPreview = 120
	trimmed := []byte(strings.TrimRight(string(b), "\r\n"))
	truncated := false
	if len(trimmed) > maxPreview {
		cut := maxPreview
		for cut > 0 && !utf8.RuneStart(trimmed[cut]) { // do not cut a character in half
			cut--
		}
		trimmed, truncated = trimmed[:cut], true
	}
	sb := Escape(trimmed)
	if truncated {
		sb += "..."
	}
	return sb
}

// reason is the cause at the end of a net/netip error, without the
// netip.ParsePrefix("...") calls it wraps around it: "prefix length out of range".
func Reason(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, "): "); i >= 0 {
		return msg[i+3:]
	}
	return msg
}

// shortErr is the cause of a network error without the "read udp
// 192.168.1.5:54966->192.0.2.55:53:" wrapper Go puts in front of it: the line it
// appears on already names the server. "i/o timeout", "connection refused".
func ShortError(err error) string {
	var op *net.OpError
	if errors.As(err, &op) && op.Err != nil {
		msg := op.Err.Error()
		for _, prefix := range []string{"read: ", "write: ", "connect: ", "dial: "} {
			msg = strings.TrimPrefix(msg, prefix)
		}
		return msg
	}
	return err.Error()
}

// unwrapPath drops the "open <path>:" a *fs.PathError adds, when the message
// already names the file.
func UnwrapPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
