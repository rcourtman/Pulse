#!/usr/bin/env python3
"""Run the real hook with synthetic index input and recording tool doubles.

These controls establish hook admission and failure propagation, not a secret
scanner, browser, linter or governance verdict. No repository is created and
no host hook, Git configuration or enforcement is changed.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
HOOK = ROOT / ".husky/pre-commit"
BASH = shutil.which("bash")
# Well beyond a pipe's capacity, with the only match before the neutral tail.
TAIL = [f"z-fixture/path-{i:05d}.txt" for i in range(16000)]


class PrecommitAdmissionTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-hook-admission-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.calls = self.root / "calls.jsonl"
        self.names = self.root / "names.txt"
        self.patch = self.root / "patch.txt"
        self.names.write_text("README.md\n")
        self.patch.write_text("+ordinary synthetic content\n")
        self.hook = self.root / "pre-commit"
        shutil.copyfile(HOOK, self.hook)
        # A closed PATH makes scanner-present and fallback cases independent
        # of whatever happens to be installed on the developer/CI host.
        for tool in ("grep", "awk", "dirname", "sed", "sort", "xargs"):
            target = shutil.which(tool)
            self.assertIsNotNone(target, tool)
            (self.bin / tool).symlink_to(target)
        self.env = {
            **os.environ,
            "PATH": str(self.bin),
            "HOOK_FIXTURE_CALLS": str(self.calls),
            "HOOK_FIXTURE_NAMES": str(self.names),
            "HOOK_FIXTURE_PATCH": str(self.patch),
        }
        # Do not inherit local hook overrides from the caller.
        for key in ("HOOK_FIXTURE_FAIL", "HOOK_FIXTURE_GIT_FAIL", "GOLANGCI_LINT_GOTOOLCHAIN"):
            self.env.pop(key, None)
        self.tool("git", """
args = sys.argv[1:]
kind = 'names' if args == ['diff', '--cached', '--name-only'] else 'patch'
if args not in (['diff', '--cached', '--name-only'], ['diff', '--cached']):
    raise SystemExit(64)
if os.environ.get('HOOK_FIXTURE_GIT_FAIL') == kind:
    raise SystemExit(31)
path = os.environ['HOOK_FIXTURE_NAMES' if kind == 'names' else 'HOOK_FIXTURE_PATCH']
sys.stdout.buffer.write(Path(path).read_bytes())
if os.environ.get('HOOK_FIXTURE_GIT_FAIL') == 'partial-' + kind:
    raise SystemExit(31)
""")
        for name in ("gitleaks", "python3", "go", "npm"):
            self.recording_tool(name)

    def tool(self, name, body):
        target = self.bin / name
        target.write_text(f"#!{sys.executable}\nimport json, os, sys\nfrom pathlib import Path\n" + body)
        target.chmod(0o700)
        return target

    def recording_tool(self, name):
        return self.tool(name, r"""
with Path(os.environ['HOOK_FIXTURE_CALLS']).open('a') as output:
    output.write(json.dumps([Path(sys.argv[0]).name, *sys.argv[1:]]) + '\n')
selector = ' '.join([Path(sys.argv[0]).name, *sys.argv[1:]])
raise SystemExit(23 if os.environ.get('HOOK_FIXTURE_FAIL') == selector else 0)
""")

    def staged(self, match=None, *, position=0):
        paths = list(TAIL)
        if match is not None:
            paths.insert(position, match)
        self.names.write_text("\n".join(paths) + "\n")

    def observed(self):
        return [json.loads(line) for line in self.calls.read_text().splitlines()] if self.calls.exists() else []

    def run_hook(self, *, fail=None, git_fail=None):
        env = {**self.env}
        if fail:
            env['HOOK_FIXTURE_FAIL'] = fail
        if git_fail:
            env['HOOK_FIXTURE_GIT_FAIL'] = git_fail
        return subprocess.run([BASH, str(self.hook)], cwd=self.root, env=env,
                              capture_output=True, text=True, timeout=20)

    def test_every_restricted_file_blocks_before_scanner_or_other_checks(self):
        for name in ('active_subs.json', 'charges.json', 'customers.json', 'subscriptions.json'):
            with self.subTest(name=name):
                self.staged(name)
                result = self.run_hook()
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn('BLOCKED: restricted file pattern matched:', result.stdout)
                self.assertEqual(self.observed(), [])

    def test_sensitive_extension_at_either_end_blocks_before_scanner(self):
        for position in (0, len(TAIL)):
            with self.subTest(position=position):
                self.staged('fixture.pem', position=position)
                result = self.run_hook()
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn('BLOCKED: sensitive file type', result.stdout)
                self.assertEqual(self.observed(), [])

    def test_docs_match_runs_mirror_and_preserves_its_failure(self):
        self.staged('docs/fixture.md')
        result = self.run_hook(fail='python3 scripts/check_docs_mirror.py --staged')
        self.assertEqual(result.returncode, 23, result.stdout + result.stderr)
        self.assertIn(['python3', 'scripts/check_docs_mirror.py', '--staged'], self.observed())
        self.assertNotIn('Pre-commit checks passed!', result.stdout)

    def test_governance_match_runs_guard_and_preserves_its_failure(self):
        status = self.root / 'docs/release-control/v6/internal/status.json'
        status.parent.mkdir(parents=True)
        status.write_text('{}')
        self.staged('internal/api/fixture.go')
        result = self.run_hook(fail='python3 scripts/release_control/governance_stage_guard.py')
        self.assertEqual(result.returncode, 23, result.stdout + result.stderr)
        self.assertIn(['python3', 'scripts/release_control/governance_stage_guard.py'], self.observed())
        self.assertNotIn('Pre-commit checks passed!', result.stdout)

    def test_frontend_match_runs_lint_and_preserves_its_failure(self):
        frontend = self.root / 'frontend-modern'
        frontend.mkdir()
        (frontend / 'package.json').write_text('{}')
        self.staged('frontend-modern/src/fixture.ts')
        result = self.run_hook(fail='npm run lint')
        self.assertEqual(result.returncode, 23, result.stdout + result.stderr)
        self.assertIn(['npm', 'run', 'lint'], self.observed())
        self.assertNotIn('Pre-commit checks passed!', result.stdout)

    def test_go_match_runs_lint_and_preserves_its_failure(self):
        self.recording_tool('golangci-lint')
        source = self.root / 'internal/api/fixture.go'
        source.parent.mkdir(parents=True)
        source.write_text('package api\n')
        (self.root / 'go.mod').write_text('module fixture\n\ngo 1.26.10\n')
        self.staged('internal/api/fixture.go')
        self.env['GOLANGCI_LINT_GOTOOLCHAIN'] = 'go1.26.10'
        result = self.run_hook(fail='golangci-lint run --concurrency 2 --timeout 20m --new-from-rev=HEAD ./internal/api')
        self.assertEqual(result.returncode, 23, result.stdout + result.stderr)
        self.assertTrue(any(call[0] == 'golangci-lint' for call in self.observed()))
        self.assertNotIn('Pre-commit checks passed!', result.stdout)

    def test_large_nonmatching_list_does_not_invent_checks(self):
        (self.root / 'frontend-modern').mkdir()
        (self.root / 'frontend-modern/package.json').write_text('{}')
        self.recording_tool('golangci-lint')
        self.staged()
        result = self.run_hook()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(any(call[0] in ('npm', 'golangci-lint', 'go') for call in self.observed()))
        self.assertIn(['python3', 'scripts/release_control/browser_verification_guard.py'], self.observed())
        self.assertIn('Pre-commit checks passed!', result.stdout)

    def test_index_read_failure_is_not_a_clean_or_nonmatching_index(self):
        self.staged('active_subs.json')
        for mode in ('names', 'partial-names'):
            with self.subTest(mode=mode):
                result = self.run_hook(git_fail=mode)
                self.assertEqual(result.returncode, 31, result.stdout + result.stderr)
                self.assertEqual(self.observed(), [])
                self.assertNotIn('Pre-commit checks passed!', result.stdout)

    def test_scanner_rejection_stays_gating_before_other_checks(self):
        result = self.run_hook(fail='gitleaks protect --staged --config .gitleaks.toml --no-banner')
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn('BLOCKED: gitleaks detected secrets', result.stdout)
        self.assertEqual(len(self.observed()), 1)
        self.assertNotIn('Pre-commit checks passed!', result.stdout)

    def fallback(self, text):
        (self.bin / 'gitleaks').unlink()
        self.patch.write_text(text)
        return self.run_hook()

    def test_large_fallback_stripe_match_cannot_disappear_on_pipe_close(self):
        result = self.fallback('+cus_synthetic_fixture\n' + '+ordinary fixture line\n' * 50000)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn('WARNING: Potential Stripe identifiers', result.stdout)
        self.assertEqual(self.observed(), [])

    def test_large_fallback_key_match_cannot_disappear_on_pipe_close(self):
        marker = '-----BEGIN ' + 'OPENSSH PRIVATE ' + 'KEY-----'
        result = self.fallback('+' + marker + '\n' + '+ordinary fixture line\n' * 50000)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn('BLOCKED: Likely secret detected', result.stdout)
        self.assertEqual(self.observed(), [])

    def test_fallback_empty_filtered_patch_is_a_valid_no_match(self):
        for patch in ('', '-deleted content\n', '+fixture_test.go\n', '+.husky/pre-commit\n'):
            with self.subTest(patch=patch):
                if not (self.bin / 'gitleaks').exists():
                    self.recording_tool('gitleaks')
                result = self.fallback(patch)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('Pre-commit checks passed!', result.stdout)

    def test_fallback_patch_read_failure_stays_gating(self):
        (self.bin / 'gitleaks').unlink()
        for mode in ('patch', 'partial-patch'):
            with self.subTest(mode=mode):
                result = self.run_hook(git_fail=mode)
                self.assertEqual(result.returncode, 31, result.stdout + result.stderr)
                self.assertEqual(self.observed(), [])
                self.assertNotIn('Pre-commit checks passed!', result.stdout)


if __name__ == '__main__':
    unittest.main()
