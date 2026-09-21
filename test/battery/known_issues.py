"""Cases that currently fail because of a bug that is open on the issue tracker.

Each entry keeps its case failing on purpose: the run stays green while the bug is there and goes
red the moment the case passes, saying to delete the entry. So when a fix lands, delete its lines
here in the same commit; when a new bug is found, add its case and the issue number.

    case id: (issue, platforms the bug is known on, or None for all)
"""
KNOWN = {
    # web: a response body cut short is reported as a success
    "F.srv-trunc-cleanly": ("#26", None),
    "F.srv-trunc-cleanly-json": ("#26", None),
    "F.srv-reset-mid-body": ("#26", None),
    "F.srv-reset-mid-body-json": ("#26", None),
    "F.srv-slow-body": ("#26", None),
    "F.srv-slow-body-json": ("#26", None),
    # ping: another ping's reply is taken for the target's
    "I.no-false-success-from-another-ping": ("#27", ("darwin",)),
    # web keeps the whole body in memory
    "L.web-300MB-body-memory": ("#28", None),
    # udp panics on an invalid --payload
    "H.payload-neg": ("#29", None),
    "H.payload-int64max": ("#29", None),
    # --timeout overflows
    "B.timeout-int64max": ("#30", None),
    "B.timeout-10^10": ("#30", None),
    # --count with --delay 0 runs out of file descriptors
    "E.fd-pressure-count-3000": ("#31", None),
    # failures logged at OK level
    "H.closed-port": ("#32", None),
    "I.unspecified": ("#32", None),
    # unknown subcommands and negative counts are not usage errors
    "A.completion-bogus": ("#33", None),
    "A.listen-bogus": ("#33", None),
    "B.listen-count-negative": ("#33", None),
    # listen http reports a stalled request as answered / ignores malformed ones
    "J.http-client-stalls-mid-body-is-not-logged-as-ok": ("#34", None),
    "J.http-malformed-request-is-logged-and-counted": ("#35", None),
    # web -k warns at ERROR level
    "F.tls-selfsigned-insecure": ("#37", None),
    "F.tls-insecure-and-cacert": ("#37", None),
    "F.tls-mismatch-server-insecure": ("#37", None),
    # web: no help for a URL without a scheme
    "F.url-no-scheme": ("#38", None),
}
