"""Tests for image-sboms.py, with stand-ins for docker, syft and grype:

    python3 .github/scripts/test_image_sboms.py
"""
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "image-sboms.py")

# The index docker/build-push-action pushes: two platforms, and BuildKit's attestation manifests.
INDEX = {
    "schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json",
    "manifests": [
        {"digest": "sha256:" + "a" * 64, "platform": {"os": "linux", "architecture": "amd64"}},
        {"digest": "sha256:" + "b" * 64, "platform": {"os": "linux", "architecture": "arm64"}},
        {"digest": "sha256:" + "c" * 64, "platform": {"os": "unknown", "architecture": "unknown"},
         "annotations": {"vnd.docker.reference.type": "attestation-manifest", "vnd.docker.reference.digest": "sha256:" + "a" * 64}},
    ],
}

FAKE = r'''#!/usr/bin/env python3
import json, os, sys
name = os.path.basename(sys.argv[0])
with open(os.environ["FAKE_LOG"], "a") as f:
    f.write(name + " " + " ".join(sys.argv[1:]) + "\n")
a = sys.argv[1:]
if name == "docker":
    if os.environ.get("FAKE_DOCKER_FAIL"):
        sys.stderr.write("unauthorized\n"); sys.exit(1)
    print(os.environ["FAKE_INDEX"]); sys.exit(0)
if name == "syft":
    for i, x in enumerate(a):
        if x == "-o":
            fmt, path = a[i + 1].split("=", 1)
            json.dump({"bomFormat": "CycloneDX"} if fmt == "cyclonedx-json" else {"spdxVersion": "SPDX-2.3"}, open(path, "w"))
    sys.exit(0)
if name == "grype":
    for i, x in enumerate(a):
        if x == "-o":
            fmt, path = a[i + 1].split("=", 1)
            json.dump({"matches": [{"vulnerability": {"severity": "High"}}, {"vulnerability": {"severity": "Low"}},
                                   {"vulnerability": {"severity": "Low"}}, {"vulnerability": {}}]}, open(path, "w"))
    sys.exit(0)
sys.exit(2)
'''


class ImageSBOMs(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="test-image-sboms-")
        self.addCleanup(shutil.rmtree, self.tmp)
        self.env = dict(os.environ, FAKE_LOG=os.path.join(self.tmp, "log"), FAKE_INDEX=json.dumps(INDEX),
                        GITHUB_OUTPUT=os.path.join(self.tmp, "output"), GITHUB_STEP_SUMMARY=os.path.join(self.tmp, "summary"))
        for t in ("docker", "syft", "grype"):
            p = os.path.join(self.tmp, t)
            with open(p, "w") as f:
                f.write(FAKE)
            os.chmod(p, os.stat(p).st_mode | stat.S_IEXEC)
            self.env[t.upper()] = p

    def run_it(self, **env):
        return subprocess.run([sys.executable, SCRIPT, "--image", "ghcr.io/dmartsapp/shint", "--digest", "sha256:" + "f" * 64,
                               "--name", "ghcr", "--out", os.path.join(self.tmp, "out")],
                              capture_output=True, text=True, env=dict(self.env, **env))

    def read(self, name):
        with open(os.path.join(self.tmp, name)) as f:
            return f.read()

    def test_each_platform_by_its_own_digest(self):
        r = self.run_it()
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(sorted(os.listdir(os.path.join(self.tmp, "out"))), [
            "image.ghcr.linux.amd64.cdx.json", "image.ghcr.linux.amd64.grype.json", "image.ghcr.linux.amd64.spdx.json",
            "image.ghcr.linux.arm64.cdx.json", "image.ghcr.linux.arm64.grype.json", "image.ghcr.linux.arm64.spdx.json"])
        log = self.read("log")
        self.assertIn("docker buildx imagetools inspect --raw ghcr.io/dmartsapp/shint@sha256:" + "f" * 64, log)
        self.assertIn("syft scan registry:ghcr.io/dmartsapp/shint@sha256:" + "a" * 64 + " -q -o cyclonedx-json=", log)
        self.assertIn("syft scan registry:ghcr.io/dmartsapp/shint@sha256:" + "b" * 64, log)
        self.assertNotIn("c" * 64, log.split("\n", 1)[1])   # the attestation manifest is not scanned
        self.assertIn("grype sbom:", log)
        self.assertEqual(self.read("output"), "digest_linux_amd64=sha256:%s\ndigest_linux_arm64=sha256:%s\n" % ("a" * 64, "b" * 64))
        summary = self.read("summary")
        self.assertIn("| linux/amd64 | `sha256:%s` | 0 | 1 | 0 | 2 | 0 | 1 |" % ("a" * 64), summary)
        self.assertIn("not gated on", summary)
        self.assertIn("1 High, 2 Low, 1 Unknown", r.stdout)

    def test_a_platform_missing_from_the_index_stops_it(self):
        index = dict(INDEX, manifests=INDEX["manifests"][:1] + INDEX["manifests"][2:])
        r = self.run_it(FAKE_INDEX=json.dumps(index))
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("has no image for linux/arm64", r.stderr)

    def test_a_failing_tool_stops_it_and_says_why(self):
        r = self.run_it(FAKE_DOCKER_FAIL="1")
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("unauthorized", r.stderr)
        self.assertFalse(os.path.exists(os.path.join(self.tmp, "output")))


if __name__ == "__main__":
    unittest.main()
