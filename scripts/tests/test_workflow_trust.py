#!/usr/bin/env python3
"""Tests for the GitHub Actions trust contract."""

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
MODULE_PATH = REPO_ROOT / "scripts" / "check_workflow_trust.py"
SPEC = importlib.util.spec_from_file_location("check_workflow_trust", MODULE_PATH)
assert SPEC and SPEC.loader
workflow_trust = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = workflow_trust
SPEC.loader.exec_module(workflow_trust)

PIN = "a" * 40
DIGEST = "b" * 64


class WorkflowTrustTest(unittest.TestCase):
    def audit(self, content: str) -> list[str]:
        with tempfile.TemporaryDirectory() as temporary_directory:
            path = Path(temporary_directory) / "test.yml"
            path.write_text(content, encoding="utf-8")
            return [finding.message for finding in workflow_trust.audit_workflow(path)]

    def test_accepts_immutable_dependencies_and_explicit_checkout_credentials(self) -> None:
        findings = self.audit(
            f"""permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@{PIN}
        with:
          persist-credentials: false
      - uses: owner/action/path@{PIN} # v1
      - uses: docker://example/image@sha256:{DIGEST}
      - uses: ./.github/workflows/local.yml
"""
        )
        self.assertEqual(findings, [])

    def test_rejects_mutable_action_runner_and_container_references(self) -> None:
        findings = self.audit(
            """permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: owner/action@v2
      - uses: docker://alpine:3.22
"""
        )
        self.assertTrue(any("mutable hosted runner" in finding for finding in findings))
        self.assertTrue(any("full commit SHA" in finding for finding in findings))
        self.assertTrue(any("sha256 digest" in finding for finding in findings))

    def test_rejects_implicit_or_unjustified_checkout_credentials(self) -> None:
        omitted = self.audit(
            f"""permissions: {{}}
steps:
  - uses: actions/checkout@{PIN}
"""
        )
        self.assertTrue(any("must set persist-credentials" in finding for finding in omitted))

        unjustified = self.audit(
            f"""permissions: {{}}
steps:
  - uses: actions/checkout@{PIN}
    with:
      persist-credentials: true
"""
        )
        self.assertTrue(any("require # required" in finding for finding in unjustified))

        mixed_case = self.audit(
            f"""permissions: {{}}
steps:
  - uses: Actions/Checkout@{PIN}
"""
        )
        self.assertTrue(
            any("must set persist-credentials" in finding for finding in mixed_case)
        )

    def test_accepts_documented_authenticated_git_write(self) -> None:
        findings = self.audit(
            f"""permissions: {{}}
steps:
  - uses: actions/checkout@{PIN}
    with:
      persist-credentials: true  # required: authenticated git writes
"""
        )
        self.assertEqual(findings, [])

    def test_requires_explicit_least_privilege_permissions(self) -> None:
        missing = self.audit("jobs: {}\n")
        self.assertTrue(any("top-level permissions" in finding for finding in missing))

        broad = self.audit("permissions: write-all\njobs: {}\n")
        self.assertTrue(any("scope mapping" in finding for finding in broad))

        dynamic = self.audit("permissions: ${{ inputs.permissions }}\njobs: {}\n")
        self.assertTrue(any("scope mapping" in finding for finding in dynamic))

    def test_rejects_shell_template_data_but_accepts_env_data(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    steps:
      - run: echo "${{ inputs.name }}"
      - run: |
          echo "${{ secrets.ACCESS_TOKEN }}"
          echo "${{ github.token }}"
      - env:
          NAME: ${{ inputs.name }}
          TOKEN: ${{ github.token }}
        run: printf '%s\\n' "$NAME" >/dev/null
"""
        )
        self.assertEqual(
            sum("must enter run scripts through env" in finding for finding in findings),
            3,
        )

    def test_repository_workflows_satisfy_contract(self) -> None:
        findings = workflow_trust.audit_directory(REPO_ROOT / ".github" / "workflows")
        self.assertEqual([finding.render() for finding in findings], [])


if __name__ == "__main__":
    unittest.main()
