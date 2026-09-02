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
    def audit(self, content: str, name: str = "test.yml") -> list[str]:
        with tempfile.TemporaryDirectory() as temporary_directory:
            path = Path(temporary_directory) / name
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

    def test_oidc_write_requires_reviewed_literal_hosted_runner(self) -> None:
        trusted = self.audit(
            """permissions: {}
jobs:
  attest:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      id-token: write
    steps:
      - run: echo attest
"""
        )
        self.assertEqual(trusted, [])

        for runner in (
            "self-hosted",
            "[self-hosted, Linux, X64]",
            "${{ matrix.runner }}",
            "ubuntu-26.04",
        ):
            with self.subTest(runner=runner):
                findings = self.audit(
                    f"""permissions: {{}}
jobs:
  attest:
    runs-on: {runner}
    timeout-minutes: 10
    permissions:
      id-token: write
    steps:
      - run: echo attest
"""
                )
                self.assertTrue(
                    any("trusted delivery identity" in finding for finding in findings)
                )

        duplicate = self.audit(
            """permissions: {}
jobs:
  attest:
    runs-on: ubuntu-24.04
    runs-on: windows-2025
    timeout-minutes: 10
    permissions:
      id-token: write
    steps:
      - run: echo attest
"""
        )
        self.assertTrue(
            any("trusted delivery identity" in finding for finding in duplicate)
        )

    def test_top_level_oidc_permission_applies_to_local_jobs(self) -> None:
        findings = self.audit(
            """permissions:
  id-token: write
jobs:
  attest:
    runs-on: self-hosted
    timeout-minutes: 10
    steps:
      - run: echo attest
"""
        )
        self.assertTrue(
            any("trusted delivery identity" in finding for finding in findings)
        )

    def test_reusable_workflow_caller_owns_oidc_runner_boundary(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  attest:
    permissions:
      id-token: write
    uses: ./.github/workflows/attest.yml
"""
        )
        self.assertEqual(findings, [])

    def test_yaml_escaped_keys_cannot_bypass_trust_policy(self) -> None:
        findings = self.audit(
            r'''"o\x6e":
  "pull_request_targ\u0065t":
permissions: {}
"jo\u0062s":
  attest:
    "runs-\U0000006fn": self-hosted
    "timeout-\x6dinutes": 10
    "permi\x73sions":
      "id-\u0074oken": write
    steps:
      - "u\x73es": owner/action@main
      - "\U00000072un": echo "${{ inputs.name }}"
'''
        )
        self.assertTrue(
            any("pull_request_target is prohibited" in finding for finding in findings)
        )
        self.assertTrue(
            any("trusted delivery identity" in finding for finding in findings)
        )
        self.assertTrue(any("full commit SHA" in finding for finding in findings))
        self.assertTrue(
            any("must enter run scripts through env" in finding for finding in findings)
        )

    def test_yaml_expansion_and_flow_forms_cannot_hide_trust_fields(self) -> None:
        explicit = self.audit(
            '''permissions: {}
jobs:
  attest:
    runs-on: self-hosted
    timeout-minutes: 10
    ? "permissions"
    :
      id-token: write
    steps:
      - run: echo attest
'''
        )
        self.assertTrue(any("explicit YAML mapping keys" in item for item in explicit))

        anchored = self.audit(
            '''permissions: {}
jobs:
  attest: &shared_job
    runs-on: self-hosted
    timeout-minutes: 10
    permissions:
      id-token: write
    steps:
      - run: echo attest
  copied: *shared_job
'''
        )
        self.assertTrue(any("YAML anchors" in item for item in anchored))

        flow_alias_keys = self.audit(
            '''permissions: {}
metadata: [&permission_key permissions, &oidc_key id-token, &uses_key uses]
jobs:
  attest:
    runs-on: self-hosted
    timeout-minutes: 10
    *permission_key:
      *oidc_key: write
    steps:
      - *uses_key: owner/action@main
'''
        )
        self.assertTrue(any("YAML anchors" in item for item in flow_alias_keys))

        flow = self.audit(
            '''permissions: {}
jobs:
  test:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps: [{uses: owner/action@main}]
'''
        )
        self.assertTrue(any("block mappings and sequences" in item for item in flow))

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

    def test_only_allows_hardened_closed_pr_target_cancellation(self) -> None:
        workflow = (
            REPO_ROOT
            / ".github"
            / "workflows"
            / "reclaim-closed-pr-capacity.yml"
        ).read_text()
        findings = self.audit(
            workflow,
            workflow_trust.SAFE_PULL_REQUEST_TARGET_WORKFLOW,
        )
        self.assertEqual(findings, [])

        unsafe = workflow.replace(
            "            await cleanup.cancelClosedPullRequestRuns({ github, context, core });",
            "            require('child_process').exec('git fetch origin pull/1/head');\n"
            "            await cleanup.cancelClosedPullRequestRuns({ github, context, core });",
        )
        findings = self.audit(
            unsafe,
            workflow_trust.SAFE_PULL_REQUEST_TARGET_WORKFLOW,
        )
        self.assertTrue(
            any("pull_request_target is prohibited" in finding for finding in findings)
        )

        unsafe_checkout = workflow.replace(
            "          persist-credentials: false",
            "          persist-credentials: false\n"
            "          repository: ${{ github.event.pull_request.head.repo.full_name }}",
        )
        findings = self.audit(
            unsafe_checkout,
            workflow_trust.SAFE_PULL_REQUEST_TARGET_WORKFLOW,
        )
        self.assertTrue(
            any("pull_request_target is prohibited" in finding for finding in findings)
        )

        findings = self.audit(workflow)
        self.assertTrue(
            any("pull_request_target is prohibited" in finding for finding in findings)
        )

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

    def test_privileged_jobs_cannot_consume_unsigned_caches(self) -> None:
        findings = self.audit(
            f"""permissions:
  contents: read
jobs:
  secret_job:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/setup-go@{PIN}
        with:
          cache: true
      - uses: actions/cache/restore@{PIN}
        with:
          path: ~/.cache
          key: privileged
      - env:
          SIGNING_TOKEN: ${{{{ secrets.SIGNING_TOKEN }}}}
        run: echo signed
  publisher:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      packages: write
    steps:
      - uses: docker/build-push-action@{PIN}
        with:
          cache-from: type=registry,ref=example.invalid/cache
          cache-to: type=registry,ref=example.invalid/cache
"""
        )
        self.assertEqual(
            sum("credential- or write-capable jobs" in finding for finding in findings),
            4,
        )

    def test_read_only_jobs_can_cache_and_privileged_jobs_can_disable_cache(self) -> None:
        findings = self.audit(
            f"""permissions:
  contents: read
jobs:
  read_only:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/cache@{PIN}
        with:
          path: ~/.cache
          key: read-only
  privileged:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/setup-go@{PIN}
        with:
          cache: false
      - uses: actions/setup-node@{PIN}
        with:
          package-manager-cache: false
      - env:
          SIGNING_TOKEN: ${{{{ secrets.SIGNING_TOKEN }}}}
        run: echo signed
"""
        )
        self.assertEqual(findings, [])

    def test_privileged_setup_actions_must_disable_automatic_caching(self) -> None:
        findings = self.audit(
            f"""permissions: {{}}
jobs:
  privileged:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/setup-go@{PIN}
      - uses: actions/setup-node@{PIN}
      - env:
          SIGNING_TOKEN: ${{{{ secrets.SIGNING_TOKEN }}}}
        run: echo signed
"""
        )
        self.assertEqual(
            sum("explicitly disable setup-action caches" in finding for finding in findings),
            2,
        )

    def test_quoted_write_permissions_and_workflow_secrets_are_privileged(self) -> None:
        quoted_write = self.audit(
            f'''permissions: {{}}
jobs:
  publisher:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      "contents": "write"
    steps:
      - uses: actions/cache@{PIN}
'''
        )
        self.assertTrue(
            any(
                "must not restore or save unsigned caches" in finding
                for finding in quoted_write
            )
        )

        inherited_secret = self.audit(
            f'''permissions: {{}}
env:
  SIGNING_TOKEN: ${{{{ secrets.SIGNING_TOKEN }}}}
jobs:
  publisher:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/cache@{PIN}
'''
        )
        self.assertTrue(
            any(
                "must not restore or save unsigned caches" in finding
                for finding in inherited_secret
            )
        )

    def test_job_trust_structure_accepts_any_consistent_yaml_indent(self) -> None:
        indented_privileged_job = self.audit(
            f"""permissions:
    contents: read
jobs:
    publisher:
        runs-on: ubuntu-24.04
        permissions:
            contents: write
        steps:
            - uses: actions/cache@{PIN}
"""
        )
        self.assertTrue(
            any("explicit timeout-minutes" in finding for finding in indented_privileged_job)
        )
        self.assertTrue(
            any(
                "must not restore or save unsigned caches" in finding
                for finding in indented_privileged_job
            )
        )

        indented_top_level_grant = self.audit(
            f"""permissions:
    contents: write
jobs:
    publisher:
        runs-on: ubuntu-24.04
        timeout-minutes: 10
        steps:
            - uses: actions/cache@{PIN}
"""
        )
        self.assertTrue(
            any(
                "must not restore or save unsigned caches" in finding
                for finding in indented_top_level_grant
            )
        )

        trailing_top_level_grant = self.audit(
            f"""jobs:
    publisher:
        runs-on: ubuntu-24.04
        timeout-minutes: 10
        steps:
            - uses: actions/cache@{PIN}
permissions:
    contents: write
"""
        )
        self.assertTrue(
            any(
                "must not restore or save unsigned caches" in finding
                for finding in trailing_top_level_grant
            )
        )

        trailing_top_level_secret = self.audit(
            f"""permissions:
    contents: read
jobs:
    publisher:
        runs-on: ubuntu-24.04
        timeout-minutes: 10
        steps:
            - uses: actions/cache@{PIN}
env:
    SIGNING_TOKEN: ${{{{ secrets.SIGNING_TOKEN }}}}
"""
        )
        self.assertTrue(
            any(
                "must not restore or save unsigned caches" in finding
                for finding in trailing_top_level_secret
            )
        )

        read_only_env_value = self.audit(
            f"""permissions:
    contents: read
jobs:
    validation:
        runs-on: ubuntu-24.04
        timeout-minutes: 10
        env:
            ACCESS_MODE: write
        steps:
            - uses: actions/cache@{PIN}
"""
        )
        self.assertEqual(read_only_env_value, [])

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
      -
        run: |2- # explicit indentation remains generated shell source
          echo "${{ github.event.issue.title }}"
      - env:
          NAME: ${{ inputs.name }}
          TOKEN: ${{ github.token }}
        run: printf '%s\\n' "$NAME" >/dev/null
"""
        )
        self.assertEqual(
            sum("must enter run scripts through env" in finding for finding in findings),
            6,
        )

    def test_rejects_template_data_in_executable_action_inputs(self) -> None:
        findings = self.audit(
            f"""permissions: {{}}
jobs:
  unsafe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/github-script@{PIN}
        with:
          script: |
            const title = '${{{{ github.event.issue.title }}}}';
            const requested = '${{{{ inputs.name }}}}';
      - with:
          inlineScript: echo "${{{{ secrets.DEPLOY_TOKEN }}}}"
        uses: azure/cli@{PIN}
      - uses: azure/powershell@{PIN}
        with:
          inlineScript: |
            Write-Output '${{{{ steps.prepare.outputs.command }}}}'
      -
        with:
          script: |2- # explicit indentation remains generated source
            const body = '${{{{ github.event.comment.body }}}}';
        uses: actions/github-script@{PIN}
"""
        )
        self.assertEqual(
            sum("executable action inputs" in finding for finding in findings),
            5,
        )

        safe = self.audit(
            f"""permissions: {{}}
jobs:
  safe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/github-script@{PIN}
        env:
          TITLE: ${{{{ github.event.issue.title }}}}
        with:
          script: |
            const title = process.env.TITLE;
            const source = '${{{{ github.sha }}}}';
"""
        )
        self.assertEqual(safe, [])

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
      -
        env:
          BARE_RELEASE: ${{ inputs.bare_release }}
        run: echo "bare=$BARE_RELEASE" >> "$GITHUB_OUTPUT"
"""
        )
        self.assertEqual(
            sum("validated or encoded" in finding for finding in findings),
            5,
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

    def test_temporary_bash_environment_does_not_clear_taint(self) -> None:
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
          RELEASE_NAME="safe value" true
          RELEASE_NAME=safe{ true }
          printf 'release=%s\\n' "$RELEASE_NAME" >> "$GITHUB_OUTPUT"
"""
        )
        self.assertEqual(
            sum("validated or encoded" in finding for finding in findings),
            1,
        )

    def test_conditional_bash_reassignment_does_not_clear_possible_taint(self) -> None:
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
          if [ "$RELEASE_NAME" = latest ]; then
            RELEASE_NAME=v1.2.3
          fi
          printf 'release=%s\\n' "$RELEASE_NAME" >> "$GITHUB_OUTPUT"
"""
        )
        self.assertEqual(
            sum("validated or encoded" in finding for finding in findings),
            1,
        )

    def test_unconditional_bash_reassignment_clears_possible_taint(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  safe:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - env:
          RELEASE_NAME: ${{ inputs.release_name }}
        run: |
          RELEASE_NAME=v1.2.3
          printf 'release=%s\\n' "$RELEASE_NAME" >> "$GITHUB_OUTPUT"
"""
        )
        self.assertEqual(findings, [])

    def test_powershell_trusted_reassignment_is_case_insensitive(self) -> None:
        findings = self.audit(
            """permissions: {}
jobs:
  safe:
    runs-on: windows-2025
    timeout-minutes: 10
    steps:
      - shell: pwsh
        env:
          RELEASE_NAME: ${{ inputs.release_name }}
        run: |
          $release_name = 'safe'
          "release=$RELEASE_NAME" >> $env:GITHUB_OUTPUT
"""
        )
        self.assertEqual(findings, [])

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
