"""Cases that currently fail because of a bug that is open on the issue tracker.

Each entry keeps its case failing on purpose: the run stays green while the bug is there and goes
red the moment the case passes, saying to delete the entry. So when a fix lands, delete its lines
here in the same commit; when a new bug is found, add its case and the issue number.

    case id: (issue, platforms the bug is known on, or None for all)
"""
KNOWN = {
    # listen http reports a stalled request as answered / ignores malformed ones
    "J.http-client-stalls-mid-body-is-not-logged-as-ok": ("#34", None),
    "J.http-malformed-request-is-logged-and-counted": ("#35", None),
    # web -k warns at ERROR level
    "F.tls-selfsigned-insecure": ("#37", None),
    "F.tls-insecure-and-cacert": ("#37", None),
    "F.tls-mismatch-server-insecure": ("#37", None),
}
