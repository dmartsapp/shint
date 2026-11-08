package lib

import "strconv"

// httpStatusNames is the IANA HTTP Status Code Registry
// (https://www.iana.org/assignments/http-status-codes/), as of 2026-11-08: every
// permanent registration, with the registry's own description. RFC 9110 is the
// reference for most of them; the rest cite the RFC that registered them.
// Temporary registrations (104 at the time of writing) are left out - they
// expire. Not Go's http.StatusText: several of its names predate RFC 9110 (it
// says "Request Entity Too Large" for 413, "Unprocessable Entity" for 422, and
// "I'm a teapot" for the reserved 418).
var httpStatusNames = map[int]string{
	100: "Continue",
	101: "Switching Protocols",
	102: "Processing",
	103: "Early Hints",

	200: "OK",
	201: "Created",
	202: "Accepted",
	203: "Non-Authoritative Information",
	204: "No Content",
	205: "Reset Content",
	206: "Partial Content",
	207: "Multi-Status",
	208: "Already Reported",
	226: "IM Used",

	300: "Multiple Choices",
	301: "Moved Permanently",
	302: "Found",
	303: "See Other",
	304: "Not Modified",
	305: "Use Proxy",
	306: "(Unused)",
	307: "Temporary Redirect",
	308: "Permanent Redirect",

	400: "Bad Request",
	401: "Unauthorized",
	402: "Payment Required",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	406: "Not Acceptable",
	407: "Proxy Authentication Required",
	408: "Request Timeout",
	409: "Conflict",
	410: "Gone",
	411: "Length Required",
	412: "Precondition Failed",
	413: "Content Too Large",
	414: "URI Too Long",
	415: "Unsupported Media Type",
	416: "Range Not Satisfiable",
	417: "Expectation Failed",
	418: "(Unused)",
	421: "Misdirected Request",
	422: "Unprocessable Content",
	423: "Locked",
	424: "Failed Dependency",
	425: "Too Early",
	426: "Upgrade Required",
	428: "Precondition Required",
	429: "Too Many Requests",
	431: "Request Header Fields Too Large",
	451: "Unavailable For Legal Reasons",

	500: "Internal Server Error",
	501: "Not Implemented",
	502: "Bad Gateway",
	503: "Service Unavailable",
	504: "Gateway Timeout",
	505: "HTTP Version Not Supported",
	506: "Variant Also Negotiates",
	507: "Insufficient Storage",
	508: "Loop Detected",
	510: "Not Extended (OBSOLETED)",
	511: "Network Authentication Required",
}

// httpStatusClasses are the five classes of RFC 9110 section 15, by first digit.
var httpStatusClasses = map[int]string{
	1: "Informational",
	2: "Successful",
	3: "Redirection",
	4: "Client Error",
	5: "Server Error",
}

// HTTPStatusText is the standard name of an HTTP status code, the status_text
// every command shows next to a status: the registry's name for a registered
// code ("Forbidden"); for an unregistered one the class RFC 9110 says a client
// must treat it as ("Server Error (5xx)" for Cloudflare's 520); and "Invalid"
// for a code outside 100-599, which RFC 9110 calls invalid - 0 included, which a
// server can send as "000". Where shint itself uses status 0 for "no response",
// the caller leaves status_text empty instead. It is never the reason phrase the
// server sent: HTTP/2 and HTTP/3 have none, and an HTTP/1.1 server may send any
// text.
func HTTPStatusText(code int) string {
	if name, ok := httpStatusNames[code]; ok {
		return name
	}
	if code < 100 || code > 599 {
		return "Invalid"
	}
	class := code / 100
	return httpStatusClasses[class] + " (" + strconv.Itoa(class) + "xx)"
}
