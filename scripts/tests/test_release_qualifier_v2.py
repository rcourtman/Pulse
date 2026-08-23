#!/usr/bin/env python3
"""Adversarial executable tests for the unused release qualifier v2."""

from __future__ import annotations

import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml
from yaml.constructor import ConstructorError


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_PATH = ROOT / ".github/workflows/qualify-release-containers-v2.yml"
WORKFLOW = WORKFLOW_PATH.read_text(encoding="utf-8")
EXPECTED_REPOSITORY = "rcourtman/Pulse"
EXPECTED_WORKFLOW_PATH = ".github/workflows/qualify-release-containers-v2.yml"
CALLED_SHA = "a" * 40
CALLER_SHA = "b" * 40

sys.path.insert(0, str(ROOT / "scripts"))
from release_candidate_manifest import create_manifest, verify_local  # noqa: E402


class UniqueKeyLoader(yaml.BaseLoader):
    """Keep GitHub-style scalar strings while rejecting shadowed YAML keys."""


def construct_unique_mapping(
    loader: UniqueKeyLoader,
    node: yaml.MappingNode,
    deep: bool = False,
) -> dict[str, object]:
    mapping: dict[str, object] = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if not isinstance(key, str):
            raise ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                "workflow mapping key must be a string",
                key_node.start_mark,
            )
        if key in mapping:
            raise ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                f"duplicate workflow key: {key}",
                key_node.start_mark,
            )
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    construct_unique_mapping,
)
PARSED_WORKFLOW = yaml.load(WORKFLOW, Loader=UniqueKeyLoader)
if not isinstance(PARSED_WORKFLOW, dict):
    raise AssertionError("qualifier v2 workflow must be a YAML mapping")


def extract_step_block(step_name: str) -> str:
    marker = f"      - name: {step_name}\n"
    start = WORKFLOW.find(marker)
    if start < 0:
        raise AssertionError(f"workflow step not found: {step_name}")
    end = WORKFLOW.find("\n      - name:", start + len(marker))
    if end < 0:
        end = len(WORKFLOW)
    return WORKFLOW[start:end]


def extract_job_block(job_name: str) -> str:
    marker = f"  {job_name}:\n"
    start = WORKFLOW.find(marker)
    if start < 0:
        raise AssertionError(f"workflow job not found: {job_name}")
    match = re.search(r"^  [A-Za-z0-9_-]+:\n", WORKFLOW[start + len(marker) :], re.M)
    if match is None:
        return WORKFLOW[start:]
    return WORKFLOW[start : start + len(marker) + match.start()]


def extract_run_script(step_name: str) -> str:
    step = extract_step_block(step_name)
    marker = "        run: |\n"
    start = step.find(marker)
    if start < 0:
        raise AssertionError(f"run block not found for step: {step_name}")
    lines = step[start + len(marker) :].splitlines()
    script_lines: list[str] = []
    for line in lines:
        if line and not line.startswith("          "):
            break
        script_lines.append(line[10:] if line else "")
    return "\n".join(script_lines) + "\n"


GATE_SCRIPT = PARSED_WORKFLOW["jobs"]["validate_invocation"]["steps"][0]["run"]
if not isinstance(GATE_SCRIPT, str):
    raise AssertionError("qualifier v2 gate must have one scalar run script")


def canonical_environment() -> dict[str, str]:
    return {
        "CALLED_WORKFLOW_REPOSITORY": EXPECTED_REPOSITORY,
        "CALLED_WORKFLOW_FILE_PATH": EXPECTED_WORKFLOW_PATH,
        "CALLED_WORKFLOW_REF": (
            f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}@{CALLED_SHA}"
        ),
        "CALLED_WORKFLOW_SHA": CALLED_SHA,
        "CALLER_REPOSITORY": EXPECTED_REPOSITORY,
        "CALLER_REPOSITORY_ID": "932825524",
        "CALLER_REPOSITORY_OWNER_ID": "8825017",
        "CALLER_REF": "refs/heads/main",
        "CALLER_SHA": CALLER_SHA,
        "CALLER_WORKFLOW_REF": (
            f"{EXPECTED_REPOSITORY}/.github/workflows/create-release.yml@refs/heads/main"
        ),
        "CALLER_WORKFLOW_SHA": CALLER_SHA,
        "CALLER_EVENT_NAME": "workflow_dispatch",
        "RELEASE_VERSION": "6.3.2",
    }


def parse_outputs(content: str) -> dict[str, str]:
    outputs: dict[str, str] = {}
    for line in content.splitlines():
        name, separator, value = line.partition("=")
        if not separator or not name or name in outputs:
            raise AssertionError(f"malformed gate output: {line!r}")
        outputs[name] = value
    return outputs


class GateResult:
    def __init__(
        self,
        process: subprocess.CompletedProcess[str],
        output: str,
    ) -> None:
        self.process = process
        self.output = output


class ReleaseQualifierV2GateTest(unittest.TestCase):
    def run_gate(self, overrides: dict[str, str] | None = None) -> GateResult:
        with tempfile.TemporaryDirectory() as temp_dir:
            output_path = Path(temp_dir) / "github-output"
            output_path.touch()
            environment = os.environ.copy()
            environment.update(canonical_environment())
            environment.update(overrides or {})
            environment["GITHUB_OUTPUT"] = str(output_path)
            process = subprocess.run(
                ["bash", "-c", GATE_SCRIPT],
                cwd=ROOT,
                env=environment,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )
            return GateResult(process, output_path.read_text(encoding="utf-8"))

    def assert_rejected(
        self,
        overrides: dict[str, str],
        expected_error: str,
    ) -> None:
        result = self.run_gate(overrides)
        self.assertNotEqual(result.process.returncode, 0, result.process.stdout)
        self.assertIn(expected_error, result.process.stderr)
        self.assertEqual(result.output, "", "rejected input must emit no outputs")

    def test_accepts_only_supported_stable_and_prerelease_versions(self) -> None:
        for version, runner in (
            ("6.3.2", '"ubuntu-24.04"'),
            (
                "6.3.2-rc.12",
                '["self-hosted","Linux","X64","pulse-pve-build"]',
            ),
            (
                "6.3.2-alpha.2",
                '["self-hosted","Linux","X64","pulse-pve-build"]',
            ),
            (
                "6.3.2-beta.3",
                '["self-hosted","Linux","X64","pulse-pve-build"]',
            ),
        ):
            with self.subTest(version=version):
                result = self.run_gate({"RELEASE_VERSION": version})
                self.assertEqual(result.process.returncode, 0, result.process.stderr)
                self.assertEqual(
                    parse_outputs(result.output),
                    {
                        "release_version": version,
                        "source_sha": CALLER_SHA,
                        "container_artifact_name": (
                            f"release-container-payload-{CALLER_SHA}-{version}"
                        ),
                        "runner": runner,
                    },
                )

    def test_rejects_untrusted_called_workflow_identity(self) -> None:
        alternate_sha = "c" * 40
        cases = (
            (
                "external repository",
                {"CALLED_WORKFLOW_REPOSITORY": "attacker/Pulse"},
                "called workflow repository is not trusted",
            ),
            (
                "alternate path",
                {"CALLED_WORKFLOW_FILE_PATH": ".github/workflows/other.yml"},
                "called workflow path is not trusted",
            ),
            (
                "branch reference",
                {
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}@main"
                    )
                },
                "called workflow reference is not the matching immutable SHA",
            ),
            (
                "full branch reference",
                {
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}"
                        "@refs/heads/main"
                    )
                },
                "called workflow reference is not the matching immutable SHA",
            ),
            (
                "pull request reference",
                {
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}"
                        "@refs/pull/42/merge"
                    )
                },
                "called workflow reference is not the matching immutable SHA",
            ),
            (
                "tag reference",
                {
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}"
                        "@refs/tags/v6.3.2"
                    )
                },
                "called workflow reference is not the matching immutable SHA",
            ),
            (
                "short SHA",
                {
                    "CALLED_WORKFLOW_SHA": "a" * 12,
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}@{'a' * 12}"
                    ),
                },
                "called workflow SHA is not a full lowercase commit SHA",
            ),
            (
                "uppercase SHA",
                {
                    "CALLED_WORKFLOW_SHA": "A" * 40,
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}@{'A' * 40}"
                    ),
                },
                "called workflow SHA is not a full lowercase commit SHA",
            ),
            (
                "malformed SHA",
                {
                    "CALLED_WORKFLOW_SHA": "g" * 40,
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}@{'g' * 40}"
                    ),
                },
                "called workflow SHA is not a full lowercase commit SHA",
            ),
            (
                "inconsistent full SHA",
                {
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/{EXPECTED_WORKFLOW_PATH}@{alternate_sha}"
                    )
                },
                "called workflow reference is not the matching immutable SHA",
            ),
            (
                "reference repository mismatch",
                {
                    "CALLED_WORKFLOW_REF": (
                        f"attacker/Pulse/{EXPECTED_WORKFLOW_PATH}@{CALLED_SHA}"
                    )
                },
                "called workflow reference is not the matching immutable SHA",
            ),
            (
                "reference path mismatch",
                {
                    "CALLED_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/.github/workflows/other.yml@{CALLED_SHA}"
                    )
                },
                "called workflow reference is not the matching immutable SHA",
            ),
        )
        for name, overrides, error in cases:
            with self.subTest(name=name):
                self.assert_rejected(overrides, error)

    def test_rejects_untrusted_caller_identity_and_context(self) -> None:
        cases = (
            (
                "external repository",
                {"CALLER_REPOSITORY": "attacker/Pulse"},
                "caller repository is not trusted",
            ),
            (
                "recycled repository name",
                {"CALLER_REPOSITORY_ID": "1"},
                "caller repository ID is not trusted",
            ),
            (
                "wrong owner identity",
                {"CALLER_REPOSITORY_OWNER_ID": "1"},
                "caller repository owner ID is not trusted",
            ),
            (
                "push event",
                {"CALLER_EVENT_NAME": "push"},
                "caller event is not an authorized release dispatch",
            ),
            (
                "pull request target on main",
                {"CALLER_EVENT_NAME": "pull_request_target"},
                "caller event is not an authorized release dispatch",
            ),
            (
                "feature branch",
                {"CALLER_REF": "refs/heads/feature"},
                "caller ref is not main",
            ),
            (
                "pull request ref",
                {"CALLER_REF": "refs/pull/42/merge"},
                "caller ref is not main",
            ),
            (
                "tag ref",
                {"CALLER_REF": "refs/tags/v6.3.2"},
                "caller ref is not main",
            ),
            (
                "unqualified branch",
                {"CALLER_REF": "main"},
                "caller ref is not main",
            ),
            (
                "short SHA",
                {
                    "CALLER_SHA": "b" * 12,
                    "CALLER_WORKFLOW_SHA": "b" * 12,
                },
                "caller SHA is not a full lowercase commit SHA",
            ),
            (
                "uppercase SHA",
                {
                    "CALLER_SHA": "B" * 40,
                    "CALLER_WORKFLOW_SHA": "B" * 40,
                },
                "caller SHA is not a full lowercase commit SHA",
            ),
            (
                "malformed SHA",
                {
                    "CALLER_SHA": "z" * 40,
                    "CALLER_WORKFLOW_SHA": "z" * 40,
                },
                "caller SHA is not a full lowercase commit SHA",
            ),
            (
                "caller workflow on tag",
                {
                    "CALLER_WORKFLOW_REF": (
                        f"{EXPECTED_REPOSITORY}/.github/workflows/create-release.yml"
                        "@refs/tags/v6.3.2"
                    )
                },
                "caller workflow reference is not a workflow on main",
            ),
            (
                "caller workflow outside canonical repository",
                {
                    "CALLER_WORKFLOW_REF": (
                        "attacker/Pulse/.github/workflows/create-release.yml"
                        "@refs/heads/main"
                    )
                },
                "caller workflow reference is not a workflow on main",
            ),
            (
                "caller workflow SHA mismatch",
                {"CALLER_WORKFLOW_SHA": "c" * 40},
                "caller workflow SHA does not match the caller commit",
            ),
            (
                "caller workflow short SHA",
                {"CALLER_WORKFLOW_SHA": "b" * 12},
                "caller workflow SHA is not a full lowercase commit SHA",
            ),
        )
        for name, overrides, error in cases:
            with self.subTest(name=name):
                self.assert_rejected(overrides, error)

    def test_rejects_unsupported_or_shell_active_versions_without_execution(self) -> None:
        unsupported = (
            "",
            "v6.3.2",
            "5.1.0",
            "7.0.0",
            "6.3",
            "6.3.2.1",
            "6.3.2-rc",
            "6.3.2-rc.x",
            "6.3.2-preview.1",
            "6.3.2-nightly.1",
            "6.3.2+build.1",
            "6.3.2-rc.1+build.1",
            "6.3.2; echo unsafe",
            "6.3.2`id`",
            "6.3.2'quoted'",
            '6.3.2"quoted"',
            " 6.3.2",
            "6.3.2 ",
            "6.3.2\nmalicious",
        )
        for version in unsupported:
            with self.subTest(version=version):
                self.assert_rejected(
                    {"RELEASE_VERSION": version},
                    "release version is not a supported Pulse v6 version",
                )

        with tempfile.TemporaryDirectory() as temp_dir:
            marker = Path(temp_dir) / "must-not-exist"
            self.assert_rejected(
                {"RELEASE_VERSION": f"6.3.2$(touch {marker})"},
                "release version is not a supported Pulse v6 version",
            )
            self.assertFalse(marker.exists(), "version input reached shell evaluation")

    def test_manifest_verification_rejects_identity_and_payload_mismatches(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            release_dir = Path(temp_dir) / "payload"
            release_dir.mkdir()
            asset = release_dir / "pulse"
            asset.write_bytes(b"immutable")
            manifest = create_manifest(release_dir, "6.3.2", CALLER_SHA)

            with self.assertRaisesRegex(ValueError, "version .* does not match"):
                verify_local(release_dir, manifest, "6.3.3", CALLER_SHA)
            with self.assertRaisesRegex(ValueError, "source SHA .* does not match"):
                verify_local(release_dir, manifest, "6.3.2", "c" * 40)
            asset.write_bytes(b"Immutable")
            with self.assertRaisesRegex(ValueError, "digest mismatch"):
                verify_local(release_dir, manifest, "6.3.2", CALLER_SHA)


class ReleaseQualifierV2ShapeTest(unittest.TestCase):
    def test_yaml_loader_rejects_shadowed_security_keys(self) -> None:
        for duplicate in ("runs-on", "permissions", "run"):
            with self.subTest(duplicate=duplicate), self.assertRaisesRegex(
                ConstructorError,
                f"duplicate workflow key: {duplicate}",
            ):
                yaml.load(
                    f"job:\n  {duplicate}: safe\n  {duplicate}: unsafe\n",
                    Loader=UniqueKeyLoader,
                )

    def test_gate_is_the_only_unconditional_hosted_root(self) -> None:
        jobs = PARSED_WORKFLOW["jobs"]
        self.assertIsInstance(jobs, dict)
        self.assertEqual(list(jobs), ["validate_invocation", "qualify"])
        self.assertEqual(PARSED_WORKFLOW["permissions"], {})

        parsed_gate = jobs["validate_invocation"]
        self.assertEqual(parsed_gate["runs-on"], "ubuntu-24.04")
        self.assertEqual(parsed_gate["permissions"], {})
        self.assertEqual(len(parsed_gate["steps"]), 1)
        self.assertNotIn("uses", parsed_gate["steps"][0])
        self.assertEqual(
            extract_run_script("Reject untrusted caller or release identity").rstrip(),
            GATE_SCRIPT.rstrip(),
        )

        gate = extract_job_block("validate_invocation")
        self.assertIn("runs-on: ubuntu-24.04", gate)
        self.assertIn("permissions: {}", gate)
        self.assertEqual(gate.count("      - name:"), 1)
        for forbidden in (
            "uses:",
            "actions/checkout",
            "download-artifact",
            "github.token",
            "secrets.",
            "curl ",
            "wget ",
            "docker ",
        ):
            self.assertNotIn(forbidden, gate)

        qualify = extract_job_block("qualify")
        parsed_qualify = jobs["qualify"]
        self.assertEqual(parsed_qualify["needs"], "validate_invocation")
        self.assertEqual(
            parsed_qualify["if"],
            "needs.validate_invocation.result == 'success'",
        )
        self.assertEqual(
            parsed_qualify["runs-on"],
            "${{ fromJSON(needs.validate_invocation.outputs.runner) }}",
        )
        self.assertIn("needs: validate_invocation", qualify)
        self.assertIn("if: needs.validate_invocation.result == 'success'", qualify)
        self.assertNotIn("always()", qualify)
        self.assertNotIn("inputs.version", qualify)
        self.assertNotIn("github.sha", qualify)

    def test_called_and_caller_contexts_are_mapped_only_through_environment(self) -> None:
        gate = extract_job_block("validate_invocation")
        for mapping in (
            "CALLED_WORKFLOW_REPOSITORY: ${{ job.workflow_repository }}",
            "CALLED_WORKFLOW_FILE_PATH: ${{ job.workflow_file_path }}",
            "CALLED_WORKFLOW_REF: ${{ job.workflow_ref }}",
            "CALLED_WORKFLOW_SHA: ${{ job.workflow_sha }}",
            "CALLER_REPOSITORY: ${{ github.repository }}",
            "CALLER_REPOSITORY_ID: ${{ github.repository_id }}",
            "CALLER_REPOSITORY_OWNER_ID: ${{ github.repository_owner_id }}",
            "CALLER_REF: ${{ github.ref }}",
            "CALLER_SHA: ${{ github.sha }}",
            "CALLER_WORKFLOW_REF: ${{ github.workflow_ref }}",
            "CALLER_WORKFLOW_SHA: ${{ github.workflow_sha }}",
            "CALLER_EVENT_NAME: ${{ github.event_name }}",
            "RELEASE_VERSION: ${{ inputs.version }}",
        ):
            self.assertIn(mapping, gate)

        scripts = [
            step["run"]
            for job in PARSED_WORKFLOW["jobs"].values()
            for step in job["steps"]
            if "run" in step
        ]
        self.assertGreaterEqual(len(scripts), 4)
        for script in scripts:
            self.assertNotIn(
                "${{",
                script,
                "GitHub expressions must enter shell only through env",
            )

    def test_workflow_has_no_selectable_source_or_artifact_identity(self) -> None:
        parsed_inputs = PARSED_WORKFLOW["on"]["workflow_call"]["inputs"]
        self.assertEqual(list(parsed_inputs), ["version"])
        inputs_start = WORKFLOW.index("    inputs:\n")
        inputs_end = WORKFLOW.index("\n# The hosted validation job", inputs_start)
        input_block = WORKFLOW[inputs_start:inputs_end]
        input_names = re.findall(r"^      ([A-Za-z0-9_-]+):$", input_block, re.M)
        self.assertEqual(input_names, ["version"])
        self.assertNotIn("source_sha:", input_block)
        self.assertNotIn("container_artifact:", input_block)
        self.assertIn(
            'artifact_name = f"release-container-payload-{caller_sha}-{version}"',
            GATE_SCRIPT,
        )

    def test_checkout_and_artifact_consumption_are_gated_and_immutable(self) -> None:
        checkout = extract_step_block("Checkout validated caller source")
        self.assertIn("repository: rcourtman/Pulse", checkout)
        self.assertIn("ref: ${{ needs.validate_invocation.outputs.source_sha }}", checkout)
        self.assertIn("persist-credentials: false", checkout)

        download = extract_step_block(
            "Download current-run exact-candidate container payload"
        )
        self.assertIn(
            "name: ${{ needs.validate_invocation.outputs.container_artifact_name }}",
            download,
        )
        for cross_run_input in (
            "run-id:",
            "repository:",
            "github-token:",
            "pattern:",
            "merge-multiple:",
        ):
            self.assertNotIn(cross_run_input, download)

        self.assertLess(WORKFLOW.index("verify-local"), WORKFLOW.index("docker buildx"))

    def test_runner_manifest_and_binary_identity_contracts_are_preserved(self) -> None:
        qualify = extract_job_block("qualify")
        for runner_contract in (
            "runs-on: ${{ fromJSON(needs.validate_invocation.outputs.runner) }}",
        ):
            self.assertIn(runner_contract, qualify)
        for validated_runner in (
            "'\"ubuntu-24.04\"'",
            "'[\"self-hosted\",\"Linux\",\"X64\",\"pulse-pve-build\"]'",
        ):
            self.assertIn(validated_runner, GATE_SCRIPT)

        verify_step = extract_step_block("Verify exact-candidate container payload")
        for expected in (
            "scripts/release_candidate_manifest.py verify-local",
            '--version "$VALIDATED_RELEASE_VERSION"',
            '--source-sha "$VALIDATED_SOURCE_SHA"',
            "release-container-payload.json",
        ):
            self.assertIn(expected, verify_step)

        identity = extract_step_block(
            "Verify container binaries match immutable candidate"
        )
        for expected in (
            "payload/release/amd64/bin/pulse",
            "sha256sum /app/pulse",
            "pulse-agent-linux-amd64",
            "sha256sum /usr/local/bin/pulse-agent",
            "pulse-control-plane-linux-amd64",
            "sha256sum /usr/local/bin/pulse-control-plane",
            'test "${actual_server}" = "${expected_server}"',
            'test "${actual_agent}" = "${expected_agent}"',
            'test "${actual_control_plane}" = "${expected_control_plane}"',
        ):
            self.assertIn(expected, identity)

    def test_v2_is_unused_and_existing_callers_still_target_v1(self) -> None:
        references: list[str] = []
        for workflow_path in sorted((ROOT / ".github/workflows").glob("*.y*ml")):
            if workflow_path == WORKFLOW_PATH:
                continue
            if WORKFLOW_PATH.name in workflow_path.read_text(encoding="utf-8"):
                references.append(workflow_path.name)
        self.assertEqual(references, [])

        build_candidate = (
            ROOT / ".github/workflows/build-release-candidate.yml"
        ).read_text(encoding="utf-8")
        create_release = (ROOT / ".github/workflows/create-release.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn(
            "uses: ./.github/workflows/qualify-release-containers.yml",
            build_candidate,
        )
        self.assertIn(
            "uses: ./.github/workflows/qualify-release-containers.yml",
            create_release,
        )

    def test_permissions_and_actions_are_minimal_and_immutable(self) -> None:
        self.assertRegex(WORKFLOW, r"(?m)^permissions: \{\}$")
        gate = extract_job_block("validate_invocation")
        qualify = extract_job_block("qualify")
        parsed_jobs = PARSED_WORKFLOW["jobs"]
        self.assertEqual(PARSED_WORKFLOW["permissions"], {})
        self.assertEqual(parsed_jobs["validate_invocation"]["permissions"], {})
        self.assertEqual(
            parsed_jobs["qualify"]["permissions"],
            {"contents": "read"},
        )
        self.assertIn("permissions: {}", gate)
        self.assertIn("permissions:\n      contents: read", qualify)
        for action in re.findall(r"(?m)^\s+uses: ([^\s#]+)", WORKFLOW):
            self.assertRegex(
                action,
                r"^[^@]+@[0-9a-f]{40}$",
                f"action is not pinned to a full commit SHA: {action}",
            )

    def test_gate_test_executes_the_exact_workflow_script(self) -> None:
        self.assertIn("python3 - <<'PY'", GATE_SCRIPT)
        self.assertIn('expected_repository = "rcourtman/Pulse"', GATE_SCRIPT)
        self.assertNotIn("${{", GATE_SCRIPT)


if __name__ == "__main__":
    unittest.main()
