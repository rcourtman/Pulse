#!/usr/bin/env python3
"""Execute the chart publication shell with a fake GitHub CLI; no network/writes."""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
CHART = b"qualified chart fixture"
DIGEST = "sha256:" + hashlib.sha256(CHART).hexdigest()


class HelmPagesIdentityTests(unittest.TestCase):
    """Run the workflow identity step and its real packet verifier offline."""

    def run_identity(self, *, release_updates=None, marker_updates=None,
                     tampered_marker=False, verification="pass",
                     portable=False, source_available=False, workflow=None):
        workflow = workflow or (ROOT / ".github/workflows/helm-pages.yml").read_text()
        step = workflow.split("        id: identity\n", 1)[1]
        script = textwrap.dedent(step.split("        run: |\n", 1)[1].split(
            "      - name:", 1)[0]).replace("${{ github.repository }}", "rcourtman/Pulse")
        script += '\nprintf "IDENTITY_BOUNDARY\\n"\n'
        marker = {"schema_version": 1, "tag": "v6.5.0", "release_id": "123",
                  "target_commitish": "a" * 40, "source_release_run_id": "200",
                  "helm_chart_digest": DIGEST}
        marker.update(marker_updates or {})
        signed_marker = json.dumps(marker).encode()
        downloaded_marker = signed_marker + (b"\n" if tampered_marker else b"")
        release = {"id": 123, "tag_name": "v6.5.0", "target_commitish": "a" * 40,
                   "draft": False, "published_at": "2026-10-06T00:54:36Z",
                   "immutable": True, "assets": [{"name": "release-activation.json",
                       "state": "uploaded", "size": len(signed_marker),
                       "digest": "sha256:" + hashlib.sha256(signed_marker).hexdigest()}]}
        if portable:
            release["assets"].append({"name": "release-build-provenance.sigstore.json",
                                     "state": "uploaded", "size": 30,
                                     "digest": "sha256:" + "b" * 64})
        release.update(release_updates or {})
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scripts").mkdir()
            for name in ("verify-github-release-integrity.sh", "require-safe-gh-attestation.sh",
                         "write_github_output.py"):
                shutil.copy2(ROOT / "scripts" / name, root / "scripts" / name)
            (root / "release.json").write_text(json.dumps(release))
            (root / "downloaded-marker.json").write_bytes(downloaded_marker)
            gh = root / "gh"
            gh.write_text(textwrap.dedent('''\
                #!/usr/bin/env python3
                import hashlib, json, os, pathlib, subprocess, sys
                args = sys.argv[1:]
                with open("calls.jsonl", "a") as log:
                    log.write(json.dumps(args) + "\\n")
                if args == ["version"]:
                    print("gh version 2.97.0 (test)")
                elif args[0] == "api":
                    endpoint = next(a for a in args if a.startswith("repos/"))
                    if os.environ["VERIFICATION"] == "api":
                        print("gh: Not Found (HTTP 404)", file=sys.stderr)
                        sys.exit(1)
                    if "/actions/runs/" in endpoint:
                        if os.environ["SOURCE_AVAILABLE"] != "true":
                            print("gh: Not Found (HTTP 404)", file=sys.stderr)
                            sys.exit(1)
                        payload = {"path": ".github/workflows/create-release.yml",
                                   "event": "workflow_dispatch", "head_sha": "a" * 40}
                    else:
                        assert endpoint == "repos/rcourtman/Pulse/releases/tags/v6.5.0", args
                        payload = json.loads(pathlib.Path("release.json").read_text())
                    if "--jq" in args:
                        result = subprocess.run(["jq", "-r", args[args.index("--jq") + 1]],
                                                input=json.dumps(payload), text=True)
                        sys.exit(result.returncode)
                    print(json.dumps(payload))
                elif args[:2] == ["release", "verify"]:
                    if os.environ["VERIFICATION"] == "release": sys.exit(1)
                    print('{"verified": true}')
                elif args[:2] == ["release", "download"]:
                    dest = pathlib.Path(args[args.index("--dir") + 1])
                    assert "release-activation.json" not in args, "must verify consumed marker"
                    (dest / "checksums.txt").write_text("fixture checksum manifest\\n")
                    if "release-build-provenance.sigstore.json" in args:
                        (dest / "release-build-provenance.sigstore.json").write_text("fixture bundle")
                elif args[:2] == ["release", "verify-asset"]:
                    asset = pathlib.Path(args[3])
                    if asset.name == "release-activation.json":
                        digest = hashlib.sha256(asset.read_bytes()).hexdigest()
                        pathlib.Path("consumed-marker.sha256").write_text(digest)
                        if digest != os.environ["SIGNED_MARKER_SHA256"]: sys.exit(1)
                    if os.environ["VERIFICATION"] == "asset": sys.exit(1)
                    print('{"verified": true}')
                elif args[:2] == ["attestation", "verify"]:
                    assert "--deny-self-hosted-runners" in args, args
                    assert args[args.index("--source-digest") + 1] == "a" * 40, args
                    portable = os.environ["PORTABLE"] == "true"
                    signer = "build-release-candidate.yml" if portable else "create-release.yml"
                    assert args[args.index("--signer-workflow") + 1] == (
                        "github.com/rcourtman/Pulse/.github/workflows/" + signer), args
                    assert ("--bundle" in args) == portable, args
                    assert args[args.index("--predicate-type") + 1] == "https://slsa.dev/provenance/v1", args
                    if os.environ["VERIFICATION"] == "provenance": sys.exit(1)
                else:
                    raise AssertionError(args)
                '''))
            curl = root / "curl"
            curl.write_text(textwrap.dedent('''\
                #!/usr/bin/env python3
                import pathlib, sys
                args = sys.argv[1:]
                assert args[-1] == "https://github.com/rcourtman/Pulse/releases/download/v6.5.0/release-activation.json", args
                pathlib.Path(args[args.index("-o") + 1]).write_bytes(
                    pathlib.Path("downloaded-marker.json").read_bytes())
                '''))
            gh.chmod(0o755)
            curl.chmod(0o755)
            output = root / "output"
            env = {"PATH": f"{root}:{os.environ['PATH']}", "VERSION": "6.5.0",
                   "GITHUB_REPOSITORY": "rcourtman/Pulse", "GITHUB_OUTPUT": str(output),
                   "SOURCE_RELEASE_RUN_ID": "200", "TARGET_COMMITISH": "a" * 40,
                   "CHART_DIGEST": DIGEST, "PULSE_RELEASE_ATTESTATION_ATTEMPTS": "1",
                   "PULSE_RELEASE_ATTESTATION_RETRY_DELAY": "0", "VERIFICATION": verification,
                   "SOURCE_AVAILABLE": str(source_available).lower(), "PORTABLE": str(portable).lower(),
                   "SIGNED_MARKER_SHA256": hashlib.sha256(signed_marker).hexdigest()}
            result = subprocess.run(["bash", "-c", script], cwd=root, env=env,
                                    capture_output=True, text=True)
            calls_file = root / "calls.jsonl"
            calls = [json.loads(line) for line in calls_file.read_text().splitlines()] if calls_file.exists() else []
            return result, calls, output.read_text() if output.exists() else ""

    def test_authenticated_packet_does_not_require_live_source_run(self):
        for portable in (False, True):
            with self.subTest(portable=portable):
                result, calls, output = self.run_identity(portable=portable)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("IDENTITY_BOUNDARY", result.stdout)
                self.assertIn("v6.5.0", output)
                self.assertFalse(any("/actions/runs/" in a for c in calls for a in c))
                self.assertTrue(any(c[:2] == ["release", "verify-asset"] for c in calls))
                self.assertTrue(any(c[:2] == ["attestation", "verify"] for c in calls))

    def test_tampered_consumed_marker_cannot_reach_outputs(self):
        result, _, output = self.run_identity(tampered_marker=True, source_available=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("activation asset verification failed", result.stderr)
        self.assertNotIn("IDENTITY_BOUNDARY", result.stdout)
        self.assertEqual(output, "")

    def test_packet_or_provenance_refusal_cannot_reach_outputs(self):
        for verification in ("api", "release", "asset", "provenance"):
            with self.subTest(verification=verification):
                result, calls, output = self.run_identity(verification=verification)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("IDENTITY_BOUNDARY", result.stdout)
                self.assertEqual(output, "")
                if verification == "api":
                    self.assertEqual(len(calls), 1, "an unavailable release must not seek a fallback")

    def test_wrong_release_identity_cannot_reach_outputs(self):
        for updates in ({"draft": True}, {"immutable": False}, {"published_at": ""},
                        {"tag_name": "v6.4.5"}, {"target_commitish": "b" * 40},
                        {"id": None}, {"id": 0}):
            with self.subTest(updates=updates):
                result, _, output = self.run_identity(release_updates=updates)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("IDENTITY_BOUNDARY", result.stdout)
                self.assertEqual(output, "")

    def test_authenticated_but_wrong_marker_identity_cannot_reach_outputs(self):
        for updates in ({"schema_version": 2}, {"tag": "v6.4.5"}, {"release_id": "124"},
                        {"target_commitish": "b" * 40}, {"source_release_run_id": "201"},
                        {"helm_chart_digest": "sha256:" + "c" * 64}):
            with self.subTest(updates=updates):
                result, _, output = self.run_identity(marker_updates=updates)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("IDENTITY_BOUNDARY", result.stdout)
                self.assertEqual(output, "")


class HelmPagesRetryTests(unittest.TestCase):
    def run_publication(self, *, draft=False, digest=DIGEST, prerelease=False,
                        version="6.4.3", exists=True):
        workflow = (ROOT / ".github/workflows/helm-pages.yml").read_text()
        step = workflow.split("      - name: Publish chart release and merge Pages index\n", 1)[1]
        script = textwrap.dedent(step.split("        run: |\n", 1)[1].split(
            "          git -C gh-pages config", 1)[0])
        script = script.replace("${{ github.repository }}", "test/pulse")
        script += '\nprintf "PAGES_BOUNDARY\\n"\n'
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "dist").mkdir()
            (root / "dist" / f"pulse-{version}.tgz").write_bytes(CHART)
            fake = root / "gh"
            fake.write_text(textwrap.dedent('''\
                #!/usr/bin/env python3
                import json, os, subprocess, sys
                args = sys.argv[1:]
                with open("calls.jsonl", "a") as log:
                    log.write(json.dumps(args) + "\\n")
                if args[:2] == ["release", "view"]:
                    if os.environ["EXISTS"] == "false":
                        sys.exit(1)
                    payload = {"isDraft": json.loads(os.environ["DRAFT"]),
                               "isPrerelease": json.loads(os.environ["PRERELEASE"])}
                elif args[0] == "api":
                    payload = {"assets": [{"name": "pulse-" + os.environ["VERSION"] + ".tgz",
                                           "digest": os.environ["DIGEST"]}]}
                else:
                    sys.exit(0)
                result = subprocess.run(["jq", "-r", args[args.index("--jq") + 1]],
                                        input=json.dumps(payload), text=True)
                sys.exit(result.returncode)
                '''))
            fake.chmod(0o755)
            env = {"PATH": f"{root}:{os.environ['PATH']}", "VERSION": version,
                   "GITHUB_REPOSITORY": "test/pulse", "TARGET_COMMITISH": "a" * 40,
                   "DRAFT": json.dumps(draft), "PRERELEASE": json.dumps(prerelease),
                   "DIGEST": digest, "EXISTS": json.dumps(exists)}
            result = subprocess.run(["bash", "-c", script], cwd=root, env=env,
                                    capture_output=True, text=True)
            calls = [json.loads(line) for line in (root / "calls.jsonl").read_text().splitlines()]
            return result, calls

    def test_draft_with_matching_asset_cannot_reach_pages(self):
        result, calls = self.run_publication(draft=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("PAGES_BOUNDARY", result.stdout)
        self.assertFalse(any(c[:2] in (["release", "edit"], ["release", "upload"],
                                     ["release", "create"]) for c in calls))

    def test_draft_without_asset_is_not_modified(self):
        result, calls = self.run_publication(draft=True, digest="")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls), 1)

    def test_unknown_draft_state_fails_closed(self):
        result, _ = self.run_publication(draft=None)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("PAGES_BOUNDARY", result.stdout)

    def test_published_matching_chart_retry_is_read_only(self):
        result, calls = self.run_publication()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("PAGES_BOUNDARY", result.stdout)
        self.assertEqual(len(calls), 2)

    def test_published_mismatched_chart_is_not_replaced(self):
        result, calls = self.run_publication(digest="sha256:" + "c" * 64)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls), 2)
        self.assertNotIn("PAGES_BOUNDARY", result.stdout)

    def test_published_stable_classification_is_corrected(self):
        result, calls = self.run_publication(prerelease=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls[-1][:2], ["release", "edit"])
        self.assertIn("--prerelease=false", calls[-1])
        self.assertIn("--latest=false", calls[-1])

    def test_new_stable_and_preview_classification(self):
        for version, expected in (("6.4.3", "false"), ("6.4.3-rc.2", "true")):
            with self.subTest(version=version):
                result, calls = self.run_publication(exists=False, version=version)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls[-1][:2], ["release", "create"])
                self.assertIn(f"--prerelease={expected}", calls[-1])
                self.assertIn("--latest=false", calls[-1])


class PublicReceiptTests(unittest.TestCase):
    def receipt(self, mode):
        workflow = (ROOT / '.github/workflows/helm-pages.yml').read_text()
        step = workflow.split('      - name: Verify public Pages chart\n', 1)[1]
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'dist').mkdir()
            (root / 'dist/pulse-6.4.3.tgz').write_bytes(b'qualified')
            programs = {
                'curl': '''#!/usr/bin/env python3
import pathlib, sys
pathlib.Path(sys.argv[sys.argv.index('-o')+1]).write_text('version: 6.4.3\\n')
''',
                'helm': '''#!/usr/bin/env python3
import os, pathlib, sys
args = sys.argv[1:]
if args[0] == 'pull':
    mode = os.environ['MODE']
    if mode == 'unavailable': sys.exit(1)
    if mode != 'missing':
        dest = pathlib.Path(args[args.index('--destination')+1])
        (dest / 'pulse-6.4.3.tgz').write_bytes(b'qualified' if mode == 'match' else b'wrong')
''',
                'sleep': '#!/bin/sh\nexit 0\n',
            }
            for name, body in programs.items():
                path = root / name
                path.write_text(body)
                path.chmod(0o755)
            return subprocess.run(['bash', '-c', script], cwd=root,
                                  env={'PATH': f"{root}:{os.environ['PATH']}",
                                       'VERSION': '6.4.3', 'MODE': mode},
                                  capture_output=True, text=True)

    def test_matching_public_package_passes(self):
        result = self.receipt('match')
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_same_version_wrong_bytes_fail(self):
        result = self.receipt('wrong')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('[OK]', result.stdout)

    def test_missing_download_fails(self):
        self.assertNotEqual(self.receipt('missing').returncode, 0)

    def test_unavailable_download_fails(self):
        self.assertNotEqual(self.receipt('unavailable').returncode, 0)


if __name__ == "__main__":
    unittest.main()
