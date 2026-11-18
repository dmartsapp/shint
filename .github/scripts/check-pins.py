#!/usr/bin/env python3
"""Refuses an action or a base image that is not pinned (issue #103).

A tag can be moved: whoever controls an action's repository, or an image's,
could otherwise change what the release pipeline runs - and it holds the
registry tokens and makes the attestations. So:

- every `uses:` in .github/workflows names an action by a full commit SHA
  (`owner/repo@<40 hex> # vX.Y.Z`, the version as a comment), a local action
  or reusable workflow (`./...`), or a Docker image by digest
  (`docker://image@sha256:<64 hex>`);
- every `FROM` in the Dockerfile names its image by digest
  (`image:tag@sha256:<64 hex>`), or a stage of the same file.

Moving a pin is described in docs/src/tech-ci.md ("Pinned actions and images").

Usage: check-pins.py [workflow files and Dockerfiles]  (default: the repository's)
"""
import glob
import os
import re
import sys

SHA = re.compile(r"^[\w.-]+/[\w.-]+(/[\w./-]+)?@[0-9a-f]{40}$")
DIGEST = re.compile(r"@sha256:[0-9a-f]{64}$")
USES = re.compile(r"^\s*(?:-\s*)?uses:\s*(\S+)")
FROM = re.compile(r"^\s*FROM\s+(?:--platform=\S+\s+)?(\S+)(?:\s+AS\s+(\S+))?", re.IGNORECASE)


def check_workflow(path, text):
    problems = []
    for n, line in enumerate(text.splitlines(), 1):
        m = USES.match(line)
        if not m:
            continue
        ref = m.group(1).strip("'\"")
        if ref.startswith("./") or SHA.match(ref) or (ref.startswith("docker://") and DIGEST.search(ref)):
            continue
        problems.append("%s:%d: uses: %s is not pinned to a commit SHA (or an image digest)" % (path, n, ref))
    return problems


def check_dockerfile(path, text):
    problems, stages = [], set()
    for n, line in enumerate(text.splitlines(), 1):
        m = FROM.match(line)
        if not m:
            continue
        image, stage = m.group(1), m.group(2)
        if image.lower() not in stages and image != "scratch" and not DIGEST.search(image):
            problems.append("%s:%d: FROM %s is not pinned to a digest (image:tag@sha256:...)" % (path, n, image))
        if stage:
            stages.add(stage.lower())
    return problems


def main(paths):
    if not paths:
        paths = sorted(glob.glob(".github/workflows/*.yml") + glob.glob(".github/workflows/*.yaml")) + ["Dockerfile"]
    problems = []
    for p in paths:
        with open(p, encoding="utf-8") as f:
            text = f.read()
        if os.path.basename(p).startswith("Dockerfile"):
            problems += check_dockerfile(p, text)
        else:
            problems += check_workflow(p, text)
    for p in problems:
        print(p)
    if problems:
        print("pin them - see docs/src/tech-ci.md, Pinned actions and images")
        return 1
    print("every action is pinned to a commit and every base image to a digest (%d files)" % len(paths))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
