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
CHECKOUT_PIN = next(iter(workflow_trust.PROTECTED_CHECKOUT_PINS))


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
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@{CHECKOUT_PIN}
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

    def test_requires_bounded_literal_runner_job_timeout(self) -> None:
        missing = self.audit(
            """permissions: {}
jobs:
  test:
    runs-on: ubuntu-24.04
    steps:
      - run: echo test
"""
        )
        self.assertTrue(
            any("explicit timeout-minutes" in finding for finding in missing)
        )

        duplicate = self.audit(
            """permissions: {}
jobs:
  test:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    timeout-minutes: 20
    steps:
      - run: echo test
"""
        )
        self.assertTrue(
            any("explicit timeout-minutes" in finding for finding in duplicate)
        )

        dynamic = self.audit(
            """permissions: {}
jobs:
  test:
    runs-on: ubuntu-24.04
    timeout-minutes: ${{ inputs.timeout }}
    steps:
      - run: echo test
"""
        )
        self.assertTrue(
            any("integer from 1 through 360" in finding for finding in dynamic)
        )

        for invalid_value in ("0", "361"):
            invalid = self.audit(
                f"""permissions: {{}}
jobs:
  test:
    runs-on: ubuntu-24.04
    timeout-minutes: {invalid_value}
    steps:
      - run: echo test
"""
            )
            self.assertTrue(
                any("integer from 1 through 360" in finding for finding in invalid)
            )

    def test_reusable_workflow_jobs_do_not_require_caller_timeout(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  delegated:
    uses: ./.github/workflows/reusable.yml
"""
        )
        self.assertEqual(findings, [])

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

    def test_requires_reviewed_protected_checkout_pin(self) -> None:
        findings = self.audit(
            f"""permissions: {{}}
steps:
  - uses: actions/checkout@{PIN}
    with:
      persist-credentials: false
"""
        )
        self.assertTrue(
            any(
                "privileged-event protection baseline" in finding
                for finding in findings
            )
        )

    def test_rejects_privileged_pr_trigger_and_checkout_opt_out(self) -> None:
        findings = self.audit(
            f"""on:
  pull_request_target:
permissions: {{}}
steps:
  - uses: actions/checkout@{CHECKOUT_PIN}
    with:
      persist-credentials: false
      allow-unsafe-pr-checkout: true
"""
        )
        self.assertTrue(
            any("pull_request_target is prohibited" in finding for finding in findings)
        )
        self.assertTrue(any("must not opt out" in finding for finding in findings))

    def test_workflow_run_requires_canonical_upstream_code(self) -> None:
        missing_branch = self.audit(
            """on:
  workflow_run:
    workflows: [Build]
    types: [completed]
permissions: {}
"""
        )
        self.assertTrue(
            any("literal canonical branch list" in finding for finding in missing_branch)
        )
        quoted_missing_branch = self.audit(
            '''"on":
  "workflow_run":
    workflows: [Build]
permissions: {}
'''
        )
        self.assertTrue(
            any(
                "literal canonical branch list" in finding
                for finding in quoted_missing_branch
            )
        )

        for branches in ("[release]", "[main, release]", "${{ fromJSON(vars.BRANCHES) }}"):
            with self.subTest(branches=branches):
                findings = self.audit(
                    f"""on:
  workflow_run:
    workflows: [Build]
    types: [completed]
    branches: {branches}
permissions: {{}}
"""
                )
                self.assertTrue(
                    any("literal canonical branch list" in finding for finding in findings)
                )

        trusted = self.audit(
            f'''"on":
  "workflow_run":
    workflows: [Build]
    "branches":
      - "main"
permissions: {{}}
steps:
  - "uses": actions/checkout@{CHECKOUT_PIN}
    with:
      "persist-credentials": false
'''
        )
        self.assertEqual(trusted, [])

        untrusted_checkout = self.audit(
            f"""on:
  workflow_run:
    workflows: [Build]
    branches: [main]
permissions: {{}}
steps:
  - uses: actions/checkout@{CHECKOUT_PIN}
    with:
      persist-credentials: false
      ref: ${{{{ github.event.workflow_run.head_sha }}}}
"""
        )
        self.assertTrue(
            any("must not select code" in finding for finding in untrusted_checkout)
        )

    def test_workflow_run_rejects_artifact_and_code_ingress(self) -> None:
        findings = self.audit(
            f"""on:
  workflow_run:
    workflows: [Build]
    branches: [main]
permissions: {{}}
steps:
  - uses: actions/download-artifact@{PIN}
  - run: gh run download "$RUN_ID"
  - run: gh pr checkout 17
  - run: git fetch origin refs/pull/17/head
  - run: curl -fsS https://api.github.com/repos/o/r/actions/artifacts/17/zip
"""
        )
        self.assertEqual(
            sum(
                "workflow artifacts or repository code" in finding
                for finding in findings
            ),
            4,
        )
        self.assertTrue(
            any(
                "must not download upstream workflow artifacts" in finding
                for finding in findings
            )
        )

        release_data_only = self.audit(
            """on:
  workflow_run:
    workflows: [Release]
    branches: [main]
permissions: {}
steps:
  - run: gh release download v1.2.3 --pattern release-activation.json
  - run: gh api --method POST repos/o/r/actions/runs/17/rerun
"""
        )
        self.assertEqual(release_data_only, [])

    def test_accepts_documented_authenticated_git_write(self) -> None:
        findings = self.audit(
            f"""permissions: {{}}
steps:
  - uses: actions/checkout@{CHECKOUT_PIN}
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

    def test_rejects_broad_or_dynamic_job_permission_overrides(self) -> None:
        broad = self.audit(
            """permissions:
  contents: read
jobs:
  unsafe:
    permissions: write-all
"""
        )
        self.assertTrue(any("job permissions" in finding for finding in broad))

        dynamic = self.audit(
            """permissions: {}
jobs:
  unsafe:
    permissions: ${{ inputs.permissions }}
"""
        )
        self.assertTrue(any("job permissions" in finding for finding in dynamic))

    def test_accepts_explicit_job_permission_mappings(self) -> None:
        findings = self.audit(
            """permissions:
  contents: read
jobs:
  read_only:
    permissions: {}
  publisher:
    permissions:
      contents: write
      id-token: write
"""
        )
        self.assertEqual(findings, [])

    def test_rejects_shell_template_data_but_accepts_env_data(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    steps:
      - run: echo "${{ inputs.name }}"
      - run: echo "${{ inputs['name'] }}"
      - run: echo "${{ toJSON(secrets) }}"
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
            5,
        )

    def test_rejects_step_and_job_outputs_in_generated_shell(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - run: echo "${{ steps.parse.outputs.version }}"
      - run: echo "${{ needs.prepare.outputs.release_tag }}"
      - run: echo "${{ steps['parse']['outputs']['version'] }}"
      - env:
          VERSION: ${{ steps.parse.outputs.version }}
          RELEASE_TAG: ${{ needs.prepare.outputs.release_tag }}
        run: printf '%s %s\\n' "$VERSION" "$RELEASE_TAG"
"""
        )
        self.assertEqual(
            sum("must enter run scripts through env" in finding for finding in findings),
            3,
        )

    def test_rejects_raw_workflow_data_in_github_command_files(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    env:
      JOB_RELEASE: ${{ needs.prepare.outputs.release }}
    steps:
      - env:
          RELEASE_NAME: ${{ inputs.release_name }}
          DERIVED_DIGEST: ${{ steps.build.outputs.digest }}
        run: |
          echo "release=$RELEASE_NAME" >> "$GITHUB_OUTPUT"
          printf 'digest=%s\\n' "$DERIVED_DIGEST" >> "$GITHUB_ENV"
          echo "job_release=$JOB_RELEASE" >> "$GITHUB_STATE"
      - run: |
          EVENT_TAG=$(jq -r '.release.tag_name' "$GITHUB_EVENT_PATH")
          echo "tag=$EVENT_TAG" >> "$GITHUB_OUTPUT"
"""
        )
        self.assertEqual(
            sum("validated or encoded" in finding for finding in findings),
            4,
        )

    def test_rejects_aliased_workflow_data_in_github_command_files(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - env:
          RELEASE_NAME: ${{ inputs.release_name }}
        run: |
          alias="$RELEASE_NAME"
          copied="prefix-${alias}"
          printf 'release=%s\\n' "$copied" >> "$GITHUB_OUTPUT"
          trimmed="${RELEASE_NAME#v}"
          printf 'trimmed=%s\\n' "${trimmed}" >> "$GITHUB_OUTPUT"
      - shell: pwsh
        env:
          RELEASE_NAME: ${{ inputs.release_name }}
        run: |
          $alias = $env:RELEASE_NAME
          "release=$alias" >> $env:GITHUB_OUTPUT
          [string] $braced = "${env:RELEASE_NAME}"
          "braced=$braced" >> $env:GITHUB_OUTPUT
"""
        )
        self.assertEqual(
            sum("validated or encoded" in finding for finding in findings),
            4,
        )

    def test_accepts_safe_command_file_writer_for_validated_values(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  safe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - env:
          RELEASE_INPUT: ${{ inputs.release }}
        run: |
          [[ "$RELEASE_INPUT" =~ ^v[0-9]+\\.[0-9]+\\.[0-9]+$ ]]
          validated_release="$RELEASE_INPUT"
          python3 scripts/write_github_output.py release "$validated_release"
"""
        )
        self.assertEqual(findings, [])

    def test_rejects_dispatch_data_and_whole_github_contexts_in_shell(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - run: echo "${{ github.event.inputs.release_name }}"
      - run: echo "${{ github.event.client_payload.command }}"
      - run: echo "${{ github['event']['inputs']['release_name'] }}"
      - run: echo "${{ github.event['client_payload']['command'] }}"
      - run: echo "${{ toJSON(github.event) }}"
      - run: echo "${{ toJSON(github) }}"
      - run: echo "${{ github['token'] }}"
      - env:
          RELEASE_NAME: ${{ github.event.inputs.release_name }}
          COMMAND: ${{ github.event.client_payload.command }}
          EVENT_JSON: ${{ toJSON(github.event) }}
        run: printf '%s %s %s\n' "$RELEASE_NAME" "$COMMAND" "$EVENT_JSON"
"""
        )
        self.assertEqual(
            sum("must enter run scripts through env" in finding for finding in findings),
            7,
        )

    def test_rejects_untrusted_github_metadata_in_generated_shell(self) -> None:
        findings = self.audit(
            """on: [pull_request]
permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    steps:
      - run: echo "${{ github.event.pull_request.title }}"
      - run: |
          echo "${{ github.head_ref }}"
          echo "${{ github.event.workflow_run.head_branch }}"
      - env:
          TITLE: ${{ github.event.pull_request.title }}
        run: printf '%s\\n' "$TITLE" >/dev/null
      - run: echo "${{ github.event.pull_request.base.sha }}"
"""
        )
        self.assertEqual(
            sum("untrusted GitHub metadata" in finding for finding in findings),
            3,
        )

    def test_pull_request_workflows_cannot_receive_confidential_secrets(self) -> None:
        findings = self.audit(
            """on:
  push:
  pull_request:
permissions: {}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    steps:
      - env:
          GH_TOKEN: ${{ secrets.WORKFLOW_PAT }}
          GH_TOKEN_BRACKET: ${{ secrets['WORKFLOW_PAT'] }}
          DYNAMIC_SECRET: ${{ secrets[vars.SECRET_NAME] }}
          WHOLE_SECRET_CONTEXT: ${{ toJSON(secrets) }}
          PUBLIC_KEY: ${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}
          PUBLIC_KEY_BRACKET: ${{ secrets['PULSE_LICENSE_PUBLIC_KEY'] }}
        run: echo checked
"""
        )
        self.assertEqual(
            sum("must not reference confidential repository secrets" in finding for finding in findings),
            4,
        )

        trusted_push = self.audit(
            """on: push
permissions: {}
jobs:
  trusted:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - env:
          GH_TOKEN: ${{ secrets.WORKFLOW_PAT }}
        run: echo checked
"""
        )
        self.assertEqual(trusted_push, [])

    def test_repository_workflows_satisfy_contract(self) -> None:
        findings = workflow_trust.audit_directory(REPO_ROOT / ".github" / "workflows")
        self.assertEqual([finding.render() for finding in findings], [])


if __name__ == "__main__":
    unittest.main()
