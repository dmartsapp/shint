#!/usr/bin/env python3
"""The SBOMs and vulnerability reports of a pushed multi-platform image, one set per platform:

    python3 .github/scripts/image-sboms.py --image ghcr.io/dmartsapp/shint --digest sha256:<index> \\
        --name ghcr --out image-sboms

For each platform the image index holds (linux/amd64, linux/arm64 - the build's attestation
manifests are skipped), by that platform's own digest:

    image.<name>.linux.<arch>.cdx.json    the image's SBOM, CycloneDX (Syft)
    image.<name>.linux.<arch>.spdx.json   the same, SPDX (Syft)
    image.<name>.linux.<arch>.grype.json  Grype's vulnerability report of that SBOM

An image holds more than shint's binary - the distroless base's Debian packages (CA
certificates, time zones, the nonroot user) - which is why images get Syft, while the release
binaries get .github/scripts/sbom.py. Grype reports; it does not decide anything: govulncheck
gates every release.

It writes each platform's digest to $GITHUB_OUTPUT (digest_linux_amd64=sha256:...) for the
attestation steps, and a summary to $GITHUB_STEP_SUMMARY. The tools are $DOCKER, $SYFT and
$GRYPE (default docker, syft, grype). Standard library only.
"""
import argparse
import json
import os
import subprocess
import sys

sys.dont_write_bytecode = True

PLATFORMS = [("linux", "amd64"), ("linux", "arm64")]
SEVERITIES = ["Critical", "High", "Medium", "Low", "Negligible", "Unknown"]


def tool(name):
    return os.environ.get(name.upper(), name)


def run(args, **kw):
    r = subprocess.run(args, capture_output=True, text=True, **kw)
    if r.returncode != 0:
        raise SystemExit("image-sboms.py: %s failed (%d): %s" % (" ".join(args[:3]), r.returncode, (r.stderr or r.stdout).strip()[-600:]))
    return r.stdout


def platform_digests(image, digest):
    """{(os, arch): digest} of the platforms the index at image@digest holds."""
    raw = json.loads(run([tool("docker"), "buildx", "imagetools", "inspect", "--raw", "%s@%s" % (image, digest)]))
    found = {}
    for m in raw.get("manifests") or []:
        p = m.get("platform") or {}
        ref_type = (m.get("annotations") or {}).get("vnd.docker.reference.type")
        if ref_type == "attestation-manifest":
            continue
        key = (p.get("os"), p.get("architecture"))
        if key in PLATFORMS:
            found[key] = m["digest"]
    missing = [o + "/" + a for o, a in PLATFORMS if (o, a) not in found]
    if missing:
        raise SystemExit("image-sboms.py: %s@%s has no image for %s" % (image, digest, ", ".join(missing)))
    return found


def counts(report):
    """How many matches of each severity Grype found."""
    c = {s: 0 for s in SEVERITIES}
    for m in report.get("matches") or []:
        s = (m.get("vulnerability") or {}).get("severity") or "Unknown"
        c[s if s in c else "Unknown"] += 1
    return c


def main(argv=None):
    ap = argparse.ArgumentParser(description="SBOMs and Grype reports of a pushed image, per platform.")
    ap.add_argument("--image", required=True, help="the image, without tag or digest: ghcr.io/dmartsapp/shint")
    ap.add_argument("--digest", required=True, help="the digest the push gave: the image index's")
    ap.add_argument("--name", required=True, help="the registry's name in the files: ghcr, dockerhub")
    ap.add_argument("--out", required=True)
    a = ap.parse_args(argv)
    os.makedirs(a.out, exist_ok=True)
    env = dict(os.environ, SYFT_CHECK_FOR_APP_UPDATE="false", GRYPE_CHECK_FOR_APP_UPDATE="false")
    outputs, summary = [], ["## SBOMs and vulnerability reports: %s" % a.image, "",
                            "| Platform | Digest | " + " | ".join(SEVERITIES) + " |", "|---|---|" + "---|" * len(SEVERITIES)]
    for (os_, arch), digest in sorted(platform_digests(a.image, a.digest).items()):
        base = os.path.join(a.out, "image.%s.%s.%s" % (a.name, os_, arch))
        ref = "%s@%s" % (a.image, digest)
        run([tool("syft"), "scan", "registry:" + ref, "-q",
             "-o", "cyclonedx-json=%s.cdx.json" % base, "-o", "spdx-json=%s.spdx.json" % base], env=env)
        for f in (base + ".cdx.json", base + ".spdx.json"):
            with open(f) as fh:
                json.load(fh)   # written, and JSON
        run([tool("grype"), "sbom:%s.cdx.json" % base, "-q", "-o", "json=%s.grype.json" % base], env=env)
        with open(base + ".grype.json") as fh:
            c = counts(json.load(fh))
        outputs.append("digest_%s_%s=%s" % (os_, arch, digest))
        summary.append("| %s/%s | `%s` | %s |" % (os_, arch, digest, " | ".join(str(c[s]) for s in SEVERITIES)))
        print("%s/%s %s: %s, %s, %s (%s)" % (os_, arch, digest, os.path.basename(base) + ".cdx.json", ".spdx.json", ".grype.json",
                                            ", ".join("%d %s" % (c[s], s) for s in SEVERITIES if c[s]) or "no matches"))
    summary += ["", "Grype's matches are reported, not gated on: govulncheck gates the release."]
    for name, lines in (("GITHUB_OUTPUT", outputs), ("GITHUB_STEP_SUMMARY", summary)):
        if os.environ.get(name):
            with open(os.environ[name], "a") as f:
                f.write("\n".join(lines) + "\n")


if __name__ == "__main__":
    main()
