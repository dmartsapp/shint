#!/usr/bin/env python3
"""Reconcile main's README with what has actually been released.

    python3 .github/scripts/readme-reconcile.py [options]

main's README is written by hand, and after a release nobody remembered to update
it (it still said v4.0.4 was the newest release after v4.2.0 shipped, and a
"donate" line queued on a branch never landed). This reads the README as it is and
brings it in line with three sources of truth, changing nothing else:

  * the release TAGS in git      - a version with a tag is Released, on the tag's date
                                   (the date in the tagger's own timezone), and a version
                                   without a row gets one, in version order;
  * CHANGELOG.md                 - the text of a new row: the section's opening sentence;
  * the GitHub MILESTONES        - the sprint window of a release still to come
                                   (optional: --milestones FILE, the JSON of
                                   `gh api repos/OWNER/REPO/milestones?state=all`);
  * the binary's own --help      - a command the README's Commands table lacks gets a row
                                   (optional: --help-file FILE).

It also puts back what the README must always have (the tagline's expansion, the
badge row - donate button included - and the support line at the bottom), and
WARNS - never "fixes" - what it cannot know: a row that says Released but has no
tag, a release still planned although a later one shipped, a release that came out
ahead of its sprint window. A release that never came out on its own is written
"Shipped in v4.2.0" in the Sprint column (by a person: the script cannot know); that is
a finished row, and the script only checks that the tag it names exists.

Running it again changes nothing (it is idempotent), and it touches only the
Roadmap table, the Commands table, the tagline and the badges, and the last line.

Options:
  --readme FILE      the README to reconcile (default: readme.md)
  --changelog FILE   default: CHANGELOG.md
  --tags-file FILE   lines of "vX.Y.Z YYYY-MM-DD" instead of asking git
  --tag vX.Y.Z       reconcile only this tag (default: every tag newer than the table's oldest row)
  --milestones FILE  milestones JSON, for sprint windows and "ahead of schedule" notes
  --help-file FILE   the output of `shint --help`, for the Commands table
  --report FILE      write the changes and warnings as Markdown (a pull request body)
  --check            change nothing; exit 1 if the README is not reconciled
Exit status: 0 done (or, with --check, nothing to do); 1 --check found work to do;
2 the README or another input is not in a shape this understands (it fails closed).

Standard library only, like docs/build.py.
"""
import argparse
import datetime
import json
import re
import subprocess
import sys

# --- what the README must always have -------------------------------------------------

EXPANSION = "Simple Host INspection Toolkit"
OLD_EXPANSIONS = ["that SHIt Network Tool"]
SUPPORT_URL = "https://www.paypal.com/paypalme/farhanssiddique"
SUPPORT_LINE = "If shint saves you time, you can [support its development](%s)." % SUPPORT_URL

# (key that identifies the badge in the file, the badge's Markdown), in the order they appear
BADGES = [
    ("img.shields.io/github/v/release", "[![Latest release](https://img.shields.io/github/v/release/dmartsapp/shint?label=release)](https://github.com/dmartsapp/shint/releases/latest)"),
    ("img.shields.io/badge/license-MIT", "[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)"),
    ("workflows/lint.yaml/badge.svg", "[![Lint](https://github.com/dmartsapp/shint/actions/workflows/lint.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/lint.yaml)"),
    ("workflows/vulncheck.yaml/badge.svg", "[![Vulnerability check](https://github.com/dmartsapp/shint/actions/workflows/vulncheck.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/vulncheck.yaml)"),
    ("workflows/build.yaml/badge.svg", "[![Build](https://github.com/dmartsapp/shint/actions/workflows/build.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/build.yaml)"),
    ("workflows/docker-hub.yaml/badge.svg", "[![Docker Hub](https://github.com/dmartsapp/shint/actions/workflows/docker-hub.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/docker-hub.yaml)"),
    ("workflows/ghcr.yaml/badge.svg", "[![GHCR](https://github.com/dmartsapp/shint/actions/workflows/ghcr.yaml/badge.svg)](https://github.com/dmartsapp/shint/actions/workflows/ghcr.yaml)"),
    ("workflows/check.yaml/badge.svg", "[![Check](https://github.com/dmartsapp/shint/actions/workflows/check.yaml/badge.svg?branch=main)](https://github.com/dmartsapp/shint/actions/workflows/check.yaml)"),
    ("img.shields.io/badge/Donate", "[![Donate](https://img.shields.io/badge/Donate-PayPal-00457C?logo=paypal&logoColor=white)](%s)" % SUPPORT_URL),
]

SPRINT_DAYS = 14          # a sprint window is the two weeks that end on the milestone's due date
LATER = 10 ** 9
X = 10 ** 6               # "v4.5.x" sorts after every numbered v4.5.N

ROW_VERSION = re.compile(r"^\*\*v(\d+)\.(\d+)\.(\d+|x)\*\*$")
FOLDED = re.compile(r"^Shipped in (v\d+\.\d+\.\d+)$")   # a release that never came out on its own
TAG = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")


class Unreadable(Exception):
    """The README (or another input) is not in the shape this reads."""


# --- small helpers --------------------------------------------------------------------

def nice(day):
    """Sep 21 - the README's way to write a date (no zero padding, no year)."""
    return "%s %d" % (day.strftime("%b"), day.day)


def parse_day(text):
    return datetime.date.fromisoformat(text[:10])


def split_row(line):
    """The cells of a Markdown table row; a backslash-escaped pipe is text, not a separator."""
    body = line.strip()
    if not (body.startswith("|") and body.endswith("|")):
        raise Unreadable("not a table row: " + line[:60])
    return [c.strip() for c in re.split(r"(?<!\\)\|", body[1:-1])]


def join_row(cells):
    return ("| " + " | ".join(cells) + " |").replace("|  |", "| |")   # an empty cell stays "| |"


def cell_text(text):
    return text.replace("|", "\\|")


def version_of(cell):
    m = ROW_VERSION.match(cell.strip())
    if not m:
        return None
    patch = X if m.group(3) == "x" else int(m.group(3))
    return (int(m.group(1)), int(m.group(2)), patch)


def label(version):
    return "v%d.%d.%d" % version


def is_done(cell):
    """A row that is no longer a plan: released, or folded into a later release."""
    return cell.startswith("Released") or bool(FOLDED.match(cell))


class Table:
    """A Markdown table inside a README: the lines it occupies and its rows."""

    def __init__(self, lines, heading):
        try:
            h = next(i for i, l in enumerate(lines) if l.strip() == heading)
        except StopIteration:
            raise Unreadable("the README has no '%s' heading" % heading)
        start = next((i for i in range(h + 1, len(lines)) if lines[i].startswith("|")), None)
        if start is None or (h + 1 < len(lines) and any(l.startswith("## ") for l in lines[h + 1:start])):
            raise Unreadable("no table under '%s'" % heading)
        end = start
        while end < len(lines) and lines[end].startswith("|"):
            end += 1
        self.start, self.end = start, end        # lines[start:end]
        self.header = split_row(lines[start])
        self.rows = [split_row(l) for l in lines[start + 2:end]]

    def render(self, lines):
        out = [join_row(self.header), lines[self.start + 1]] + [join_row(r) for r in self.rows]
        return lines[:self.start] + out + lines[self.end:]


# --- inputs ---------------------------------------------------------------------------

def read_git_tags():
    """{'v4.2.0': date, ...} for every vX.Y.Z tag, with the date in the tagger's own timezone."""
    out = subprocess.run(["git", "for-each-ref", "--format=%(refname:short)|%(taggerdate:short)|%(creatordate:short)", "refs/tags"],
                         capture_output=True, text=True)
    if out.returncode != 0:
        raise Unreadable("git could not list the tags: " + out.stderr.strip()[:120])
    tags = {}
    for line in out.stdout.splitlines():
        name, tagger, creator = (line.split("|") + ["", "", ""])[:3]
        if TAG.match(name):
            tags[name] = parse_day(tagger or creator)
    return tags


def read_text(path):
    with open(path) as f:
        return f.read()


def write_text(path, text):
    with open(path, "w") as f:
        f.write(text)


def read_tags_file(path):
    tags = {}
    for line in read_text(path).splitlines():
        if line.strip() and not line.startswith("#"):
            name, day = line.split()[:2]
            if TAG.match(name):
                tags[name] = parse_day(day)
    return tags


def read_changelog(path):
    """{'v4.2.0': {'date': 'YYYY-MM-DD'|None, 'summary': text}} for every '## vX.Y.Z' section."""
    try:
        text = read_text(path)
    except OSError:
        return {}
    sections = {}
    parts = re.split(r"^## (v\d+\.\d+\.\d+)(?: - (\S+))?\s*$", text, flags=re.M)
    for i in range(1, len(parts) - 2, 3):
        name, date, body = parts[i], parts[i + 1], parts[i + 2]
        sections[name] = {"date": date if date and re.match(r"\d{4}-\d{2}-\d{2}$", date) else None, "summary": summarize(body)}
    return sections


def summarize(body):
    """A section's opening sentence if it has one, else the bold lead of its first bullet."""
    for para in re.split(r"\n\s*\n", body.strip()):
        para = para.strip()
        if para and not para.startswith("-"):
            return " ".join(para.split())
    m = re.search(r"^- \*\*(.+?)\*\*", body, flags=re.M)
    if m:
        return re.sub(r"\s+", " ", m.group(1)).rstrip(":.") + "."
    return ""


def read_milestones(path):
    """{'v4.3.0': due date} for milestones that have a due date."""
    try:
        data = json.loads(read_text(path))
    except (OSError, ValueError) as e:
        raise Unreadable("cannot read the milestones: %s" % e)
    return {m["title"]: parse_day(m["due_on"]) for m in data if m.get("due_on") and TAG.match(m.get("title", ""))}


def read_commands(path):
    """[(name, short description)] from cobra's 'Available Commands:' block."""
    lines = read_text(path).splitlines()
    try:
        i = next(k for k, l in enumerate(lines) if l.strip() == "Available Commands:")
    except StopIteration:
        raise Unreadable("no 'Available Commands:' in the help text")
    found = []
    for line in lines[i + 1:]:
        if not line.strip():
            break
        m = re.match(r"^\s+(\S+)\s{2,}(.+?)\s*$", line)
        if m and m.group(1) not in ("help", "completion"):
            found.append((m.group(1), m.group(2)))
    return found


# --- the reconciliation ---------------------------------------------------------------

def reconcile(text, tags, changelog, milestones=None, commands=None, only_tag=None):
    """Return (new text, changes, warnings)."""
    changes, warnings = [], []
    lines = text.split("\n")

    ensure_tagline(lines, changes, warnings)
    ensure_badges(lines, changes)
    lines = reconcile_roadmap(lines, tags, changelog, milestones or {}, only_tag, changes, warnings)
    if commands:
        lines = reconcile_commands(lines, commands, changes)
    ensure_support_line(lines, changes)
    return "\n".join(lines), changes, warnings


def ensure_tagline(lines, changes, warnings):
    head = next((i for i, l in enumerate(lines[:8]) if l.startswith("**")), None)
    if head is None:
        warnings.append("The README has no bold tagline near the top, so its expansion (%s) could not be checked." % EXPANSION)
        return
    if EXPANSION in lines[head]:
        return
    for old in OLD_EXPANSIONS:
        if old in lines[head]:
            lines[head] = lines[head].replace(old, EXPANSION)
            changes.append("The tagline says **%s**." % EXPANSION)
            return
    warnings.append("The tagline does not say '%s' and is not an old wording this knows how to replace; left alone." % EXPANSION)


def ensure_badges(lines, changes):
    first_section = next((i for i, l in enumerate(lines) if l.startswith("## ")), len(lines))
    top = "\n".join(lines[:first_section])
    missing = [(key, md) for key, md in BADGES if key not in top]
    if not missing:
        return
    at = max((i for i in range(first_section) if lines[i].startswith("[![")), default=None)
    if at is None:  # no badge row at all: start one under the tagline
        tag = next((i for i, l in enumerate(lines[:8]) if l.startswith("**")), 0)
        lines[tag + 1:tag + 1] = [""] + [md for _, md in missing]
    else:
        lines[at + 1:at + 1] = [md for _, md in missing]
    for _, md in missing:
        changes.append("Badge added: %s." % re.match(r"\[!\[([^\]]*)\]", md).group(1))


def ensure_support_line(lines, changes):
    if any(SUPPORT_URL in l and "support" in l.lower() for l in lines):
        return
    while lines and lines[-1] == "":
        lines.pop()
    lines.extend(["", SUPPORT_LINE, ""])
    changes.append("The support (donate) line is back at the bottom.")


def sprint_text(due):
    return "%s - %s" % (nice(due - datetime.timedelta(days=SPRINT_DAYS - 1)), nice(due))


def reconcile_roadmap(lines, tags, changelog, milestones, only_tag, changes, warnings):
    table = Table(lines, "## Roadmap")
    if len(table.header) != 3:
        raise Unreadable("the Roadmap table should have three columns (Release, Sprint, What it brings)")
    numbered = [version_of(r[0]) for r in table.rows if version_of(r[0])]
    if not numbered:
        raise Unreadable("the Roadmap table has no '**vX.Y.Z**' rows")
    oldest = min(numbered)

    def index_of(version):
        for i, r in enumerate(table.rows):
            if version_of(r[0]) == version:
                return i
        return None

    def insert(version, cells):
        key = (lambda v: LATER if v is None else v)
        pos = next((i for i, r in enumerate(table.rows) if key(version_of(r[0])) > version), len(table.rows))
        table.rows.insert(pos, cells)

    released = {}
    for name, day in tags.items():
        version = tuple(int(x) for x in TAG.match(name).groups())
        if only_tag and name != only_tag:
            continue
        if version < oldest:
            continue  # older than anything the README covers
        released[version] = day
    if only_tag and not released and TAG.match(only_tag):
        raise Unreadable("the tag %s is not among the tags given (or is older than the Roadmap's oldest row)" % only_tag)

    touched = set()   # rows this run added or moved to Released: the one-off notes are about these
    for version in sorted(released):
        day, name = released[version], label(version)
        want = "Released " + nice(day)
        i = index_of(version)
        entry = changelog.get(name)
        if entry and entry["date"] and parse_day(entry["date"]) != day:
            warnings.append("%s: the changelog is dated %s but the tag is dated %s (the README follows the tag)." % (name, entry["date"], day.isoformat()))
        if i is None:
            summary = (entry or {}).get("summary") or "See the [changelog](CHANGELOG.md)."
            insert(version, ["**%s**" % name, want, cell_text(summary)])
            touched.add(version)
            changes.append("Row added for **%s**: %s." % (name, want))
            if not entry:
                warnings.append("%s has no section in CHANGELOG.md, so its row only points there." % name)
            continue
        row = table.rows[i]
        if row[1] == want:
            continue
        was = row[1]
        row[1] = want
        touched.add(version)
        if was.startswith("Released"):
            changes.append("**%s**: the date was corrected (%s -> %s)." % (name, was, want))
        else:
            changes.append("**%s** is marked %s (it was planned for %s)." % (name, want, was))
            if entry and entry["summary"]:
                warnings.append("**%s**: the row's text was written as a plan. The changelog says: \"%s\" Edit the row if what shipped differs (something planned may have moved to a later release)." % (name, entry["summary"]))

    # planned rows follow their milestones' sprint windows
    for row in table.rows:
        version = version_of(row[0])
        if not version or version[2] == X or is_done(row[1]):
            continue
        due = milestones.get(label(version))
        if due and row[1] != sprint_text(due):
            changes.append("**%s**: the sprint window follows its milestone (%s -> %s)." % (label(version), row[1], sprint_text(due)))
            row[1] = sprint_text(due)

    # what cannot be known: warn
    tagged = set(read_all_tag_versions(tags))
    if tags:
        for row in table.rows:
            version = version_of(row[0])
            if version and version[2] != X and row[1].startswith("Released") and version not in tagged:
                warnings.append("**%s** is marked Released but there is no tag %s. Tag it, or change the row." % (label(version), label(version)))
        for row in table.rows:
            folded = FOLDED.match(row[1])
            if folded and folded.group(1) not in {label(v) for v in tagged}:
                warnings.append("**%s** says %s but there is no tag %s." % (row[0].strip("*"), row[1], folded.group(1)))
        newest = max(tagged, default=None)
        for row in table.rows:
            version = version_of(row[0])
            if version and version[2] != X and not is_done(row[1]) and newest and version < newest:
                warnings.append("**%s** is still planned although %s has been released. Was it skipped or folded into a later release?" % (label(version), label(newest)))
        for version, day in sorted(released.items()):
            if version not in touched:
                continue
            due = milestones.get(label(version))
            if due and day < due - datetime.timedelta(days=SPRINT_DAYS - 1):
                warnings.append("%s was released %d days ahead of its sprint window (%s)." % (label(version), (due - datetime.timedelta(days=SPRINT_DAYS - 1) - day).days, sprint_text(due)))
    return table.render(lines)


def read_all_tag_versions(tags):
    return [tuple(int(x) for x in TAG.match(n).groups()) for n in tags]


def reconcile_commands(lines, commands, changes):
    try:
        table = Table(lines, "## Commands")
    except Unreadable:
        return lines
    have = set()
    for row in table.rows:
        m = re.match(r"^`shint ([A-Za-z0-9_-]+)", row[0])
        if m:
            have.add(m.group(1))
    for name, short in commands:
        if name in have:
            continue
        cells = ["`shint %s`" % name, cell_text(short)]
        pos = next((i for i, r in enumerate(table.rows) if r[0].startswith("`shint listen")), len(table.rows))
        table.rows.insert(pos, cells)
        changes.append("Command row added for `shint %s` (its --help text; give it its arguments and a question)." % name)
    return table.render(lines)


# --- command line ---------------------------------------------------------------------

def report_markdown(changes, warnings):
    out = []
    if changes:
        out += ["## Changes", ""] + ["- " + c for c in changes] + [""]
    if warnings:
        out += ["## Needs a human look", ""] + ["- " + w for w in warnings] + [""]
    if not changes and not warnings:
        out += ["The README is already reconciled.", ""]
    return "\n".join(out)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--readme", default="readme.md")
    ap.add_argument("--changelog", default="CHANGELOG.md")
    ap.add_argument("--tags-file")
    ap.add_argument("--tag")
    ap.add_argument("--milestones")
    ap.add_argument("--help-file")
    ap.add_argument("--report")
    ap.add_argument("--check", action="store_true")
    a = ap.parse_args(argv)
    try:
        if a.tag and not TAG.match(a.tag):
            raise Unreadable("--tag must look like v1.2.3, got %r" % a.tag)
        text = read_text(a.readme)
        tags = read_tags_file(a.tags_file) if a.tags_file else read_git_tags()
        new, changes, warnings = reconcile(
            text, tags, read_changelog(a.changelog),
            read_milestones(a.milestones) if a.milestones else None,
            read_commands(a.help_file) if a.help_file else None,
            a.tag)
    except (Unreadable, OSError) as e:
        print("readme-reconcile: %s" % e, file=sys.stderr)
        return 2
    md = report_markdown(changes, warnings)
    if a.report:
        write_text(a.report, md)
    print(md, end="")
    if a.check:
        return 1 if new != text else 0
    if new != text:
        write_text(a.readme, new)
    return 0


if __name__ == "__main__":
    sys.exit(main())
