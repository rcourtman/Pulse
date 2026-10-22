#!/usr/bin/env python3
"""Exercise fail-fast portal drift detection without npm, Go or source writes."""

import hashlib
import importlib.util
import json
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/check_portal_frontend_sync.py"
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
SPEC = importlib.util.spec_from_file_location("portal_frontend_sync", SCRIPT)
SYNC = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SYNC)


class PortalFrontendSyncTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.portal = self.root / SYNC.PORTAL
        self.frontend = self.portal / "frontend"
        self.manifest = self.portal / "dist/build_manifest.json"
        self.inputs = ["package.json", "build_config.mjs", "src/styles.css", "src/app.ts"]
        for name in self.inputs:
            path = self.frontend / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes((name + "\r\n").encode())
        self.manifest.parent.mkdir(parents=True)
        self.refresh()
        (self.portal / "dist/portal_app.js").write_text("shipped bundle sentinel\n")

    def refresh(self):
        # Independent producer-shaped fixture, not a call to the checker.
        payload = b"".join(name.encode() + b"\n" + (self.frontend / name).read_bytes() + b"\n"
                           for name in self.inputs)
        self.manifest.write_text(json.dumps({
            "source_hash": hashlib.sha256(payload).hexdigest(), "build_inputs": self.inputs,
        }))

    def snapshot(self):
        return {str(p.relative_to(self.root)): p.read_bytes()
                for p in self.root.rglob("*") if p.is_file()}

    def test_matching_manifest_and_real_checked_in_tree(self):
        self.assertRegex(SYNC.check(self.root), r"^[0-9a-f]{64}$")
        self.assertRegex(SYNC.check(ROOT), r"^[0-9a-f]{64}$")

    def test_dependency_manifest_source_config_and_style_drift_are_read_only(self):
        for name in self.inputs:
            with self.subTest(input=name):
                path = self.frontend / name
                original = path.read_bytes()
                path.write_bytes(original + b"changed\n")
                before = self.snapshot()
                with self.assertRaisesRegex(ValueError, "source hash differs"):
                    SYNC.check(self.root)
                self.assertEqual(self.snapshot(), before)
                path.write_bytes(original)

    def test_the_order_and_exact_newline_bytes_match_the_producer(self):
        old = SYNC.check(self.root)
        path = self.frontend / "src/app.ts"
        path.write_bytes(path.read_bytes().replace(b"\r\n", b"\n"))
        with self.assertRaisesRegex(ValueError, "source hash differs"):
            SYNC.check(self.root)
        self.refresh()
        self.assertNotEqual(SYNC.check(self.root), old)
        manifest = json.loads(self.manifest.read_text())
        manifest["build_inputs"].reverse()
        self.manifest.write_text(json.dumps(manifest))
        with self.assertRaisesRegex(ValueError, "source hash differs"):
            SYNC.check(self.root)

    def test_missing_or_malformed_manifest_and_missing_inputs_stop(self):
        for body in ("{", "[]", "{}", '{"source_hash":1}',
                     json.dumps({"source_hash": "0" * 64, "build_inputs": []})):
            with self.subTest(body=body):
                self.manifest.write_text(body)
                with self.assertRaises(ValueError):
                    SYNC.check(self.root)
        self.manifest.unlink()
        with self.assertRaises(OSError):
            SYNC.check(self.root)
        self.refresh()
        (self.frontend / "src/app.ts").unlink()
        with self.assertRaises(OSError):
            SYNC.check(self.root)

    def test_invalid_duplicate_and_escaping_inputs_stop(self):
        (self.frontend / "escape.ts").symlink_to(self.manifest)
        for name in (None, 3, "", "../outside", "/outside", "src/../outside", "./build_config.mjs",
                     "src\\styles.css", "src/\x00bad.ts", "build_config.mjs", "escape.ts"):
            with self.subTest(input=name):
                manifest = {"source_hash": "0" * 64, "build_inputs": [*self.inputs, name]}
                self.manifest.write_text(json.dumps(manifest))
                with self.assertRaises(ValueError):
                    SYNC.check(self.root)

    def test_missing_canonical_inputs_cannot_be_hidden_by_a_matching_hash(self):
        for name in ("build_config.mjs", "src/styles.css"):
            with self.subTest(input=name):
                self.inputs.remove(name)
                self.refresh()
                with self.assertRaisesRegex(ValueError, "missing canonical"):
                    SYNC.check(self.root)
                self.inputs.append(name)

    def test_cli_resolves_checkout_from_its_script_not_current_directory(self):
        result = subprocess.run(["python3", str(SCRIPT)], cwd=self.root,
                                capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Go tests remain required", result.stdout)

    def test_main_failure_is_actionable_and_never_repairs_or_executes_source(self):
        (self.frontend / "package.json").write_text('{"changed":true}\n')
        before = self.snapshot()
        with patch.object(SYNC, "ROOT", self.root), patch("sys.stderr") as stderr:
            self.assertEqual(SYNC.main(), 1)
        message = "".join(call.args[0] for call in stderr.write.call_args_list)
        self.assertIn(SYNC.REBUILD, message)
        self.assertIn("commit its generated bundle/manifest", message)
        self.assertEqual(self.snapshot(), before)

    def test_actual_workflow_shell_stops_before_classification_on_drift(self):
        changes = re.search(r"^  changes:\n(.*?)(?=^  [\w-]+:)", WORKFLOW.read_text(), re.M | re.S)[1]
        block = changes.split("      - name: Check shipped portal inputs before dependency work\n", 1)[1]
        block = block.split("      - name:", 1)[0]
        shell = "\n".join(line[10:] for line in block.split("        run: |\n", 1)[1].splitlines())
        shell += '\nprintf "classification reached\\n" > classified\n'
        scripts = self.root / "scripts"
        (scripts / "tests").mkdir(parents=True)
        shutil.copyfile(SCRIPT, scripts / SCRIPT.name)
        # An inert preceding self-test isolates the real checker's exit and
        # the actual workflow shell's fail-fast behaviour; no npm/Go peer runs.
        (scripts / "tests/test_portal_frontend_sync.py").write_text("pass\n")
        for drift in (False, True):
            with self.subTest(drift=drift):
                if drift:
                    (self.root / "classified").unlink()
                    (self.frontend / "package.json").write_text("changed dependency manifest\n")
                before = self.snapshot()
                result = subprocess.run(["bash", "-c", shell], cwd=self.root,
                                        capture_output=True, text=True, timeout=5)
                self.assertEqual(result.returncode, 1 if drift else 0, result.stdout + result.stderr)
                self.assertEqual((self.root / "classified").exists(), not drift)
                if drift:
                    self.assertIn(SYNC.REBUILD, result.stderr)
                    self.assertEqual(self.snapshot(), before)


class PortalFrontendSyncWorkflowTest(unittest.TestCase):
    def test_unconditional_preflight_precedes_classification_and_dependency_fanout(self):
        source = WORKFLOW.read_text()
        changes = re.search(r"^  changes:\n(.*?)(?=^  [\w-]+:)", source, re.M | re.S)[1]
        step = "Check shipped portal inputs before dependency work"
        block = changes.split("      - name: " + step + "\n", 1)[1].split("      - name:", 1)[0]
        self.assertNotIn("if:", block)
        self.assertNotIn("continue-on-error:", block)
        self.assertIn("python3 scripts/tests/test_portal_frontend_sync.py", block)
        self.assertIn("python3 scripts/check_portal_frontend_sync.py", block)
        self.assertIn("set -euo pipefail", block)
        self.assertLess(changes.index(step), changes.index("Classify changed files"))
        self.assertNotIn("setup-go", changes)
        self.assertNotIn("npm ci", changes)
        # The cheap diagnostic does not remove the original independent check.
        go_test = (ROOT / "internal/cloudcp/portal/frontend_sync_test.go").read_text()
        self.assertIn("func TestPulseAccountFrontendBundleStaysInSync", go_test)
        self.assertIn("go test -race -timeout 50m $pkgs", source)


if __name__ == "__main__":
    unittest.main()
