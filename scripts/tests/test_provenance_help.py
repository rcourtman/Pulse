#!/usr/bin/env python3
"""Execute copied provenance recipes with a credential-free gh recorder.

This checks shell failure handling and exact verifier arguments, not Sigstore
verification, public release availability or authenticity of synthetic assets.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
VERSION = "v6.6.0-rc.1"
ASSET = f"pulse-{VERSION}-linux-amd64.tar.gz"
SOURCE = "0123456789abcdef" * 2 + "01234567"
BUNDLE = "release-build-provenance.sigstore.json"
OFFLINE_DIRECTORY = "/path/to/private-verification-directory"

GH = r'''
import json, os, pathlib, stat, sys
args = sys.argv[1:]
phase = ('download' if args[:2] == ['release', 'download'] else
         'metadata' if args[:1] == ['api'] else
         'root' if args[:2] == ['attestation', 'trusted-root'] else
         'verify' if args[:2] == ['attestation', 'verify'] else 'unexpected')
entry = {'phase': phase, 'args': args, 'directory': os.getcwd(),
         'mode': stat.S_IMODE(os.stat('.').st_mode),
         'files': {p.name: stat.S_IMODE(p.stat().st_mode)
                   for p in pathlib.Path('.').iterdir() if p.is_file()}}
with open(os.environ['GH_RECEIPT'], 'a') as f:
    f.write(json.dumps(entry) + '\n')
failure = os.environ['FAILURE']
if os.environ.get('OFFLINE') == '1':
    assert phase == 'verify', 'Offline verification must make no acquisition call'
if phase == 'download':
    assert args == ['release', 'download', os.environ['EXPECTED_VERSION'],
                    '--repo', 'rcourtman/Pulse', '--pattern', os.environ['EXPECTED_ASSET'],
                    '--pattern', 'release-build-provenance.sigstore.json'], args
    pathlib.Path(os.environ['EXPECTED_ASSET']).write_text('synthetic public asset')
    pathlib.Path('release-build-provenance.sigstore.json').write_text('synthetic bundle')
    if failure == 'download': sys.exit(22)
elif phase == 'metadata':
    assert args == ['api', 'repos/rcourtman/Pulse/releases/tags/' + os.environ['EXPECTED_VERSION'],
                    '--jq', '.target_commitish'], args
    if failure == 'metadata': sys.exit(23)
    print(os.environ['SOURCE_VALUE'])
elif phase == 'root':
    if failure == 'root-empty': sys.exit(0)
    print('synthetic trusted root')
    if failure == 'root': sys.exit(24)
elif phase == 'verify':
    assert args == ['attestation', 'verify', os.environ['EXPECTED_ASSET'],
                    '--repo', 'rcourtman/Pulse', '--bundle', 'release-build-provenance.sigstore.json',
                    '--signer-workflow', 'github.com/rcourtman/Pulse/.github/workflows/build-release-candidate.yml',
                    '--source-digest', os.environ['SOURCE_VALUE'], '--deny-self-hosted-runners',
                    '--predicate-type', 'https://slsa.dev/provenance/v1',
                    '--custom-trusted-root', 'trusted_root.jsonl'], args
    for name in (os.environ['EXPECTED_ASSET'], 'release-build-provenance.sigstore.json',
                 'trusted_root.jsonl', 'release-source-sha.txt'):
        assert pathlib.Path(name).is_file() and pathlib.Path(name).stat().st_size, name
    print('synthetic verification observation, not a trust result')
    if failure == 'verify': sys.exit(25)
else:
    raise AssertionError(args)
'''


class ProvenanceHelpTest(unittest.TestCase):
    def setUp(self):
        self.source = (ROOT / "docs/INSTALL.md").read_text()
        self.section = self.source.split("## Verify release build provenance\n", 1)[1].split("\n## ", 1)[0]
        self.recipes = re.findall(r"```bash\n(.*?)\n```", self.section, re.S)

    def exercise(self, *, offline=False, failure="none", source=SOURCE, missing=None,
                 empty=None, previous_report=False):
        self.assertEqual(len(self.recipes), 2, "Connected and offline recipes must both be executable")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            commands, inputs = root / "bin", root / "inputs"
            commands.mkdir()
            inputs.mkdir(mode=0o700)
            gh = commands / "gh"
            gh.write_text(f"#!{sys.executable}\n" + GH)
            gh.chmod(0o700)
            sentinel = root / ASSET
            sentinel.write_text("keep the existing download")
            originals = {}
            if offline:
                for name, value in {ASSET: "synthetic public asset", BUNDLE: "synthetic bundle",
                                    "trusted_root.jsonl": "synthetic trusted root",
                                    "release-source-sha.txt": source + "\n"}.items():
                    if name == missing:
                        continue
                    path = inputs / name
                    path.write_text("" if name == empty else value)
                    path.chmod(0o600)
                    originals[name] = path.read_bytes()
                if previous_report:
                    (inputs / "provenance-verification-offline.txt").write_text("keep prior proof")
            block = self.recipes[int(offline)].replace("vX.Y.Z", VERSION)
            block = block.replace(OFFLINE_DIRECTORY, str(inputs))
            receipt = root / "gh-receipt"
            env = {**os.environ, "PATH": f"{commands}:{os.environ['PATH']}", "TMPDIR": str(root),
                   "GH_RECEIPT": str(receipt), "FAILURE": failure, "SOURCE_VALUE": source,
                   "EXPECTED_VERSION": VERSION, "EXPECTED_ASSET": ASSET,
                   "OFFLINE": "1" if offline else "0"}
            result = subprocess.run(["bash", "-c", block], cwd=root, env=env,
                                    capture_output=True, text=True, timeout=10)
            calls = [json.loads(line) for line in receipt.read_text().splitlines()] if receipt.exists() else []
            self.assertEqual(sentinel.read_text(), "keep the existing download")
            for name, value in originals.items():
                self.assertEqual((inputs / name).read_bytes(), value, name)
            if previous_report:
                self.assertEqual((inputs / "provenance-verification-offline.txt").read_text(),
                                 "keep prior proof")
            for call in calls:
                self.assertEqual(call["mode"], 0o700)
                self.assertTrue(Path(call["directory"]).is_dir(), "Preserve evidence even on failure")
                self.assertTrue(all(mode == 0o600 for mode in call["files"].values()), call)
            return result, [call["phase"] for call in calls], calls

    def test_connected_recipe_uses_new_private_files_and_exact_source(self):
        result, phases, calls = self.exercise()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(phases, ["download", "metadata", "root", "verify"])
        self.assertEqual(len({call["directory"] for call in calls}), 1)
        self.assertIn("Verified build provenance", result.stdout)

    def test_connected_failures_stop_at_the_original_phase_without_retry(self):
        for failure, expected in (("download", ["download"]),
                                  ("metadata", ["download", "metadata"]),
                                  ("root", ["download", "metadata", "root"]),
                                  ("root-empty", ["download", "metadata", "root"]),
                                  ("verify", ["download", "metadata", "root", "verify"])):
            with self.subTest(failure=failure):
                result, phases, _ = self.exercise(failure=failure)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual(phases, expected)
                self.assertNotIn("Verified build provenance", result.stdout)

    def test_metadata_must_be_a_complete_commit_not_an_empty_or_moving_ref(self):
        for value in ("", "main", "null", "a" * 39, "a" * 41, "g" * 40, SOURCE + "\nmain"):
            with self.subTest(value=value):
                result, phases, _ = self.exercise(source=value)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(phases, ["download", "metadata"])

    def test_offline_recipe_uses_only_transferred_inputs_and_same_verifier(self):
        result, phases, _ = self.exercise(offline=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(phases, ["verify"])
        self.assertIn("Verified build provenance", result.stdout)

    def test_offline_missing_or_empty_inputs_never_reach_verification(self):
        for name in (ASSET, BUNDLE, "trusted_root.jsonl", "release-source-sha.txt"):
            for condition in ("missing", "empty"):
                with self.subTest(name=name, condition=condition):
                    result, phases, _ = self.exercise(offline=True, **{condition: name})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(phases, [])

    def test_offline_invalid_source_preserves_inputs_without_verification(self):
        for value in ("", "main", "a" * 39, "g" * 40):
            with self.subTest(value=value):
                result, phases, _ = self.exercise(offline=True, source=value)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(phases, [])

    def test_offline_verifier_failure_is_returned_without_retry(self):
        result, phases, _ = self.exercise(offline=True, failure="verify")
        self.assertEqual(result.returncode, 25, result.stderr)
        self.assertEqual(phases, ["verify"])
        self.assertNotIn("Verified build provenance", result.stdout)

    def test_offline_does_not_overwrite_an_earlier_report(self):
        result, phases, _ = self.exercise(offline=True, previous_report=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(phases, [])

    def test_shipped_help_and_safety_limits(self):
        self.assertEqual((ROOT / "docs/INSTALL.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/INSTALL.md").read_bytes())
        words = " ".join(self.section.split())
        for fact in ("public Community archive", "private directory", "partial files",
                     "does not install", "does not replace", "trusted root", "revocation",
                     "not proof of a completed installation", "Do not execute"):
            with self.subTest(fact=fact):
                self.assertIn(fact, words)


if __name__ == "__main__":
    unittest.main()
