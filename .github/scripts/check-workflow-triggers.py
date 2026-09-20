#!/usr/bin/env python3
"""Enforce shint's rule for what may start a GitHub Actions workflow.

    python3 .github/scripts/check-workflow-triggers.py [workflow.yaml ...]

With no arguments it checks every file in .github/workflows/. Exit status 0
means every workflow follows the rule; 1 lists what does not.

The rule (only main runs actions; a release is a vX.Y.Z tag on main):

  * The only events are `push` and `workflow_call`. No pull_request, schedule,
    workflow_dispatch, workflow_run, ... - nothing a branch or a person can
    start.
  * A `push` trigger is one of two shapes, and nothing else:
      - tags: exactly ['v[0-9]+.[0-9]+.[0-9]+']  (the release pipeline), or
      - branches: [main] together with paths-ignore: ['.github/**'], so a merge
        that only touches .github (workflow changes) runs nothing.
  * A reusable workflow (`workflow_call`) has no other trigger, so it cannot be
    started directly.

Only the `on:` block is read, and only in block style (the way every workflow
here is written). A form it cannot read is reported rather than guessed at:
the check fails closed. Standard library only, like docs/build.py.
"""
import glob
import sys

TAG_PATTERN = "v[0-9]+.[0-9]+.[0-9]+"
IGNORED_PATH = ".github/**"


def strip_comment(line):
    """Drop a trailing `# comment` that is outside quotes."""
    quote = None
    for i, ch in enumerate(line):
        if quote:
            if ch == quote:
                quote = None
        elif ch in "'\"":
            quote = ch
        elif ch == "#" and (i == 0 or line[i - 1] in " \t"):
            return line[:i]
    return line


def unquote(text):
    text = text.strip()
    if len(text) >= 2 and text[0] == text[-1] and text[0] in "'\"":
        return text[1:-1]
    return text


def scalar_or_flow_list(text):
    """'' -> None; '[a, b]' -> ['a', 'b']; otherwise the unquoted scalar."""
    text = text.strip()
    if text == "":
        return None
    if text.startswith("["):
        if not text.endswith("]"):
            raise ValueError("unsupported flow list: " + text)
        inner = text[1:-1].strip()
        return [unquote(part) for part in inner.split(",")] if inner else []
    return unquote(text)


def parse_on_block(source):
    """The `on:` block of a workflow as nested dicts and lists.

    Raises ValueError for a shape this deliberately small reader does not
    understand (an inline `on: push`, a scalar where a mapping belongs, ...).
    """
    lines = []
    for raw in source.splitlines():
        line = strip_comment(raw).rstrip()
        if line.strip():
            lines.append(line)

    start = None
    for i, line in enumerate(lines):
        if line.startswith("on:") or line.startswith('"on":') or line.startswith("'on':"):
            start = i
            break
    if start is None:
        raise ValueError("no `on:` block found")
    if lines[start].split(":", 1)[1].strip():
        raise ValueError("`on:` must be written as a block (on:, then indented events), not inline")

    block = []
    for line in lines[start + 1:]:
        if not line.startswith((" ", "\t")):
            break  # the next top-level key
        block.append(line)
    if not block:
        raise ValueError("`on:` has no events")
    if any("\t" in line[: len(line) - len(line.lstrip())] for line in block):
        raise ValueError("tabs in the `on:` block")

    def parse(pos, indent):
        """Parse the mapping or list whose lines are indented by `indent`."""
        if block[pos].lstrip().startswith("- "):
            items = []
            while pos < len(block):
                cur = len(block[pos]) - len(block[pos].lstrip())
                if cur < indent:
                    break
                if cur > indent or not block[pos].lstrip().startswith("- "):
                    raise ValueError("unexpected line in list: " + block[pos].strip())
                items.append(unquote(block[pos].lstrip()[2:]))
                pos += 1
            return items, pos
        mapping = {}
        while pos < len(block):
            cur = len(block[pos]) - len(block[pos].lstrip())
            if cur < indent:
                break
            if cur > indent:
                raise ValueError("unexpected indentation: " + block[pos].strip())
            key, sep, rest = block[pos].strip().partition(":")
            if not sep:
                raise ValueError("expected `key:` but found: " + block[pos].strip())
            pos += 1
            value = scalar_or_flow_list(rest)
            if value is None and pos < len(block):
                child = len(block[pos]) - len(block[pos].lstrip())
                if child > indent:
                    value, pos = parse(pos, child)
            mapping[key.strip()] = value
        return mapping, pos

    first = len(block[0]) - len(block[0].lstrip())
    tree, end = parse(0, first)
    if end != len(block) or not isinstance(tree, dict):
        raise ValueError("could not read the whole `on:` block")
    return tree


def problems_in(tree):
    """Everything in a parsed `on:` tree that breaks the rule."""
    found = []
    for event in tree:
        if event not in ("push", "workflow_call"):
            found.append(
                "event `%s` is not allowed: only `push` and `workflow_call` may start a workflow" % event
            )
    if "workflow_call" in tree and set(tree) != {"workflow_call"}:
        found.append("a reusable workflow (`workflow_call`) must have no other trigger")
    if "push" not in tree:
        return found

    push = tree["push"]
    if not isinstance(push, dict):
        found.append("`push:` needs either `tags: ['%s']` or `branches: [main]` - bare `push:` runs for every branch and tag" % TAG_PATTERN)
        return found

    extra = set(push) - {"tags", "branches", "paths-ignore"}
    if extra:
        found.append("`push:` has unsupported keys: " + ", ".join(sorted(extra)))
    if "tags" in push:
        if push["tags"] != [TAG_PATTERN]:
            found.append("`push: tags:` must be exactly ['%s'], found %r" % (TAG_PATTERN, push["tags"]))
        if set(push) != {"tags"}:
            found.append("a tag trigger must not be combined with branches or paths filters")
    elif "branches" in push:
        if push["branches"] != ["main"]:
            found.append("`push: branches:` must be exactly [main], found %r" % (push["branches"],))
        ignore = push.get("paths-ignore")
        if not isinstance(ignore, list) or IGNORED_PATH not in ignore:
            found.append("a push-to-main workflow must have `paths-ignore: ['%s']`, so merges that only change workflows run nothing" % IGNORED_PATH)
    else:
        found.append("`push:` needs either `tags: ['%s']` or `branches: [main]`" % TAG_PATTERN)
    return found


def check_file(path):
    try:
        with open(path, encoding="utf-8") as handle:
            tree = parse_on_block(handle.read())
    except (OSError, ValueError) as err:
        return ["cannot read the triggers (%s)" % err]
    return problems_in(tree)


def main(argv):
    paths = argv or sorted(glob.glob(".github/workflows/*.yaml") + glob.glob(".github/workflows/*.yml"))
    if not paths:
        print("no workflow files found", file=sys.stderr)
        return 1
    failed = False
    for path in paths:
        found = check_file(path)
        if found:
            failed = True
            for message in found:
                print("%s: %s" % (path, message), file=sys.stderr)
        else:
            print("ok  " + path)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
