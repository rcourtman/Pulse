#!/usr/bin/env python3
"""Guard the RC-to-GA promotion policy across docs and release workflows."""

from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import tempfile
import unittest

import yaml
from yaml.constructor import ConstructorError

import record_rc_to_ga_blocked as blocked_record
from live_runtime_proof import evaluate_live_runtime
from release_promotion_policy_support import (
    REQUIRED_STAGED_GOVERNANCE_INPUTS,
    promotion_metadata_envelope,
    slice_requires_staged_governance_inputs,
    staged_governance_input_errors,
)
from repo_file_io import REPO_ROOT, git_env, read_repo_text

USE_STAGED_GOVERNANCE = os.environ.get("PULSE_READ_STAGED_GOVERNANCE") == "1"


def read(rel: str) -> str:
    return read_repo_text(
        rel,
        staged=USE_STAGED_GOVERNANCE,
        strict_staged=USE_STAGED_GOVERNANCE and rel in REQUIRED_STAGED_GOVERNANCE_INPUTS,
    )


def read_json(rel: str) -> dict:
    return json.loads(read(rel))


def normalize_ws(text: str) -> str:
    return " ".join(text.split())


_MATERIAL_APPROVAL_RE = re.compile(
    r"(?i)(?:"
    r"\brichard[- ]approved\b|"
    r"\bapproved by (?:richard|the (?:project|product|release) owner)\b|"
    r"\b(?:project|product|release) owner (?:then )?(?:explicitly )?approved\b"
    r")"
)
_QUANTITATIVE_RATIONALE_RE = re.compile(
    r"(?i)(?:"
    r"\d+(?:\.\d+)?\s*%|"
    r"\bconversion(?: rate)?\b|"
    r"\bweekly active\b|"
    r"\bsubscriptions?\s*(?:/|per )\s*(?:month|week|year)\b|"
    r"\bgrew from\b|"
    r"\bmeasured on\b|"
    r"\bcohort(?:s)?\b"
    r")"
)
_QUANTITATIVE_EVIDENCE_FIELDS = (
    "Source",
    "Query",
    "Snapshot",
    "Measured at",
)
_UNUSABLE_EVIDENCE_VALUE_RE = re.compile(
    r"(?i)^(?:none|n/a|na|unknown|unavailable|not (?:available|recorded)|missing)$"
)
_INLINE_EVIDENCE_REFERENCE_RE = re.compile(r"`([^`]+)`")
_RFC3339_UTC_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")


def _heading_anchor(heading: str) -> str:
    normalized = re.sub(r"[^a-z0-9 _-]", "", heading.strip().lower())
    return re.sub(r"[ _]+", "-", normalized)


def _local_evidence_reference_error(
    *, field: str, value: str, record_rel: str
) -> str | None:
    reference_match = _INLINE_EVIDENCE_REFERENCE_RE.search(value)
    if reference_match is None:
        return f"'- {field}:' must contain an inline durable reference"

    reference = reference_match.group(1).strip()
    if reference.startswith(("https://", "http://")):
        return None
    if field == "Source" and re.fullmatch(r"[A-Za-z0-9_.-]+@[0-9a-f]{7,40}:.+", reference):
        return None

    path_part, separator, anchor = reference.partition("#")
    if not path_part or path_part.startswith("/") or any(char.isspace() for char in path_part):
        return f"'- {field}:' must reference a repository artifact, source identifier, or URL"

    candidate = PurePosixPath(path_part)
    if candidate.parts and candidate.parts[0] not in {"docs", "scripts"}:
        candidate = PurePosixPath(record_rel).parent / candidate
    candidate_rel = candidate.as_posix()
    try:
        target_content = read(candidate_rel)
    except (OSError, subprocess.CalledProcessError):
        return f"'- {field}:' references missing artifact {candidate_rel!r}"

    if separator:
        anchors = {
            _heading_anchor(match.group(1))
            for match in re.finditer(r"(?m)^#{1,6}\s+(.+?)\s*$", target_content)
        }
        if anchor not in anchors:
            return f"'- {field}:' references missing section #{anchor} in {candidate_rel!r}"
    return None


def material_approval_evidence_errors(
    content: str, *, record_rel: str | None = None
) -> tuple[str, ...]:
    """Return structural evidence errors for quantitatively justified owner approvals."""
    if not _MATERIAL_APPROVAL_RE.search(content):
        return ()
    if not _QUANTITATIVE_RATIONALE_RE.search(content):
        return ()

    errors: list[str] = []
    if not re.search(r"(?m)^## Quantitative Evidence\s*$", content):
        return ("missing '## Quantitative Evidence' section",)

    values: dict[str, str] = {}
    for field in _QUANTITATIVE_EVIDENCE_FIELDS:
        match = re.search(rf"(?m)^- {re.escape(field)}:\s*(.+?)\s*$", content)
        if match is None:
            errors.append(f"missing '- {field}:' provenance field")
            continue
        value = match.group(1).strip().strip("`").strip()
        values[field] = match.group(1).strip()
        if not value or _UNUSABLE_EVIDENCE_VALUE_RE.fullmatch(value):
            errors.append(f"'- {field}:' must identify durable reproducible evidence")

    for field in ("Source", "Query", "Snapshot"):
        value = values.get(field)
        if value is None or _UNUSABLE_EVIDENCE_VALUE_RE.fullmatch(value.strip("`").strip()):
            continue
        if record_rel is None:
            if _INLINE_EVIDENCE_REFERENCE_RE.search(value) is None:
                errors.append(f"'- {field}:' must contain an inline durable reference")
            continue
        error = _local_evidence_reference_error(
            field=field,
            value=value,
            record_rel=record_rel,
        )
        if error is not None:
            errors.append(error)

    measured_at = values.get("Measured at", "").strip("`").strip()
    if measured_at and not _UNUSABLE_EVIDENCE_VALUE_RE.fullmatch(measured_at):
        if _RFC3339_UTC_RE.fullmatch(measured_at) is None:
            errors.append("'- Measured at:' must be an exact RFC3339 UTC timestamp")
    return tuple(errors)


def tracked_release_control_records() -> tuple[str, ...]:
    result = subprocess.run(
        [
            "git",
            "ls-files",
            "docs/release-control/v6/internal/records/*.md",
        ],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
        env=git_env(),
    )
    return tuple(sorted(line for line in result.stdout.splitlines() if line.strip()))


def workflow_job_block(workflow: str, job: str) -> str:
    match = re.search(
        rf"(?ms)^  {re.escape(job)}:\n.*?(?=^  [A-Za-z0-9_]+:\n|\Z)",
        workflow,
    )
    if match is None:
        raise AssertionError(f"workflow missing job {job}")
    return match.group(0)


class UniqueKeyLoader(yaml.SafeLoader):
    """YAML loader that fails instead of silently overwriting duplicate keys."""


def _construct_unique_mapping(loader: UniqueKeyLoader, node: yaml.MappingNode, deep: bool = False) -> dict:
    seen: set[object] = set()
    for key_node, _ in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in seen:
            raise ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                f"found duplicate key {key!r}",
                key_node.start_mark,
            )
        seen.add(key)
    return yaml.SafeLoader.construct_mapping(loader, node, deep=deep)


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    _construct_unique_mapping,
)


_RC_DRAFT_PACKET_NAME_RE = re.compile(r"^RELEASE_NOTES_v6_RC(\d+)_DRAFT\.md$")


def discover_rc_draft_packets() -> tuple[tuple[int, str, str, str], ...]:
    """Return (rc_number, release_notes, changelog, support_pack) for every in-repo RC draft packet, sorted by RC.

    Driven from the filesystem so adding a new RC packet automatically extends
    every test that loops over the discovered set; nobody has to edit hardcoded
    dict entries per RC.
    """
    packets: list[tuple[int, str, str, str]] = []
    releases_dir = REPO_ROOT / "docs" / "releases"
    for path in sorted(releases_dir.glob("RELEASE_NOTES_v6_RC*_DRAFT.md")):
        match = _RC_DRAFT_PACKET_NAME_RE.match(path.name)
        if not match:
            continue
        n = int(match.group(1))
        packets.append(
            (
                n,
                f"docs/releases/RELEASE_NOTES_v6_RC{n}_DRAFT.md",
                f"docs/releases/V6_CHANGELOG_RC{n}_DRAFT.md",
                f"docs/releases/V6_RC{n}_OPERATOR_SUPPORT_PACK_DRAFT.md",
            )
        )
    return tuple(sorted(packets))


def rc_packet_paths_for_version(version: str) -> tuple[str, str, str] | None:
    """Return the (release_notes, changelog, support_pack) draft paths for a 6.0.0-rc.N VERSION, or None otherwise."""
    match = re.match(r"^6\.0\.0-rc\.(\d+)$", version)
    if not match:
        return None
    n = int(match.group(1))
    return (
        f"docs/releases/RELEASE_NOTES_v6_RC{n}_DRAFT.md",
        f"docs/releases/V6_CHANGELOG_RC{n}_DRAFT.md",
        f"docs/releases/V6_RC{n}_OPERATOR_SUPPORT_PACK_DRAFT.md",
    )


def support_prerelease_packet_paths_for_version(version: str) -> tuple[str, str] | None:
    """Return release-notes and changelog paths for post-GA support RC versions."""
    if not re.match(r"^6\.\d+\.\d+-rc\.\d+$", version):
        return None
    if version.startswith("6.0.0-rc."):
        return None
    return (
        f"docs/releases/RELEASE_NOTES_v{version}.md",
        f"docs/releases/V6_CHANGELOG_v{version}.md",
    )


def stable_packet_paths_for_version(version: str) -> tuple[str, str] | None:
    """Return the stable release-notes and changelog packet paths for a v6 stable VERSION."""
    if not re.match(r"^6\.\d+\.\d+$", version):
        return None
    if version == "6.0.0":
        return ("docs/releases/RELEASE_NOTES_v6.md", "docs/releases/V6_CHANGELOG.md")
    return (
        f"docs/releases/RELEASE_NOTES_v{version}.md",
        f"docs/releases/V6_CHANGELOG_v{version}.md",
    )


def staged_files() -> tuple[str, ...]:
    result = subprocess.run(
        ["git", "diff", "--cached", "--name-only"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
        env=git_env(),
    )
    return tuple(line for line in result.stdout.splitlines() if line.strip())


STAGED_FILES = staged_files() if USE_STAGED_GOVERNANCE else ()
REQUIRES_STAGED_GOVERNANCE_INPUTS = slice_requires_staged_governance_inputs(STAGED_FILES)
STAGED_GOVERNANCE_INPUT_ERRORS = (
    tuple(staged_governance_input_errors(use_staged_governance=True))
    if REQUIRES_STAGED_GOVERNANCE_INPUTS
    else ()
)


class ReleasePromotionPolicyTest(unittest.TestCase):
    def setUp(self) -> None:
        if USE_STAGED_GOVERNANCE and not REQUIRES_STAGED_GOVERNANCE_INPUTS:
            self.skipTest("staged slice does not touch the promotion-proof surface")
        if (
            STAGED_GOVERNANCE_INPUT_ERRORS
            and self._testMethodName != "test_staged_governance_inputs_are_present"
        ):
            self.skipTest("staged governance inputs missing; see test_staged_governance_inputs_are_present")

    def test_staged_governance_inputs_are_present(self) -> None:
        if STAGED_GOVERNANCE_INPUT_ERRORS:
            self.fail(
                "staged promotion proof inputs are incomplete:\n- "
                + "\n- ".join(STAGED_GOVERNANCE_INPUT_ERRORS)
            )

    def test_release_workflows_reject_duplicate_yaml_keys(self) -> None:
        for workflow_path in (
            ".github/workflows/create-release.yml",
            ".github/workflows/release-convergence.yml",
            ".github/workflows/retry-release-convergence.yml",
            ".github/workflows/promote-private-pro-runtime.yml",
            ".github/workflows/promote-floating-tags.yml",
            ".github/workflows/helm-pages.yml",
            ".github/workflows/update-demo-server.yml",
            ".github/workflows/deploy-demo-server.yml",
        ):
            with self.subTest(workflow_path=workflow_path):
                yaml.load(read(workflow_path), Loader=UniqueKeyLoader)

    def test_customer_promotion_lease_helper_is_tracked_and_executable(self) -> None:
        helper = REPO_ROOT / "scripts" / "release_control" / "customer_promotion_lease.sh"
        tracked = subprocess.run(
            ["git", "ls-files", "--error-unmatch", str(helper.relative_to(REPO_ROOT))],
            cwd=REPO_ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(tracked.returncode, 0, tracked.stderr)
        self.assertTrue(os.access(helper, os.X_OK), f"{helper} must be executable")
        convergence = read(".github/workflows/release-convergence.yml")
        self.assertIn("scripts/release_control/customer_promotion_lease.sh acquire", convergence)
        self.assertIn("scripts/release_control/customer_promotion_lease.sh release", convergence)

    def test_release_commit_dispatches_durable_convergence_before_activation(self) -> None:
        workflow = read(".github/workflows/create-release.yml")
        convergence = read(".github/workflows/release-convergence.yml")
        publication_preflight = workflow_job_block(
            workflow, "publication_trust_preflight"
        )
        readiness = workflow_job_block(workflow, "release_readiness")
        dispatch = workflow_job_block(workflow, "dispatch_release_convergence")
        activation = workflow_job_block(workflow, "activate_release")
        commit_verdict = workflow_job_block(workflow, "release_commit_verdict")
        recovery = read(".github/workflows/recover-release-activation.yml")
        recovery_activation = workflow_job_block(recovery, "recover_activation")

        self.assertIn("runs-on: ubuntu-24.04", publication_preflight)
        self.assertIn("secrets.WORKFLOW_PAT", publication_preflight)
        self.assertIn(
            "check-github-release-immutability.sh", publication_preflight
        )
        self.assertIn("github.event.inputs.draft_only != 'true'", publication_preflight)
        self.assertIn(
            "historical_asset_backfill_only != 'true'", publication_preflight
        )
        for early_job_name in (
            "build_release_candidate",
            "frontend_bundle",
            "frontend_checks",
            "windows_install_command_smoke",
            "release_note_visuals",
            "stage_private_pro_runtime",
        ):
            with self.subTest(early_job_name=early_job_name):
                early_job = workflow_job_block(workflow, early_job_name)
                self.assertIn("- publication_trust_preflight", early_job)

        for dependency in (
            "publication_trust_preflight",
            "create_release",
            "publish_docker",
            "validate_release_assets",
            "install_sh_smoke",
            "publish_helm_chart",
            "stage_private_pro_runtime",
        ):
            self.assertIn(f"- {dependency}", readiness)
        for mutable_job in (
            "publish_helm_pages",
            "promote_floating_tags",
            "promote_private_pro_runtime",
            "update_stable_demo",
        ):
            with self.subTest(mutable_job=mutable_job):
                self.assertNotIn(f"- {mutable_job}", readiness)
                self.assertNotIn(f"- {mutable_job}", activation)
                self.assertNotRegex(workflow, rf"(?m)^  {mutable_job}:$")
                mutable = workflow_job_block(convergence, mutable_job)
                self.assertIn("needs: acquire_customer_promotion_lease", mutable)

        self.assertIn("- release_readiness", activation)
        self.assertIn(
            "needs.publication_trust_preflight.result == 'success'", readiness
        )
        self.assertIn("- publication_trust_preflight", commit_verdict)
        self.assertIn(
            'require_result "publication trust preflight"', commit_verdict
        )
        self.assertIn("- dispatch_release_convergence", activation)
        self.assertNotIn("- release_readiness", dispatch)
        self.assertIn("- create_release", dispatch)
        self.assertIn("- stage_private_pro_runtime", dispatch)
        self.assertIn("github.event.inputs.draft_only != 'true'", dispatch)
        self.assertIn("needs.prepare.outputs.historical_asset_backfill_only != 'true'", dispatch)
        self.assertIn("return_run_details: true", dispatch)
        self.assertIn("release-convergence.yml/dispatches", dispatch)
        self.assertIn("Customer convergence dispatch did not return an exact workflow run", dispatch)
        self.assertIn("continue-on-error: true", activation)
        self.assertIn("returning ${TAG} to draft quarantine", activation)
        self.assertIn("Resuming quarantined activation for ${TAG}", activation)
        self.assertNotIn('[ -n "$published_at" ] ||', activation)
        self.assertIn('[ "$activation_committed" = "true" ] ||', activation)
        self.assertIn('committed=false', activation)
        self.assertIn('[ "$committed" != "true" ]', activation)
        self.assertIn("release-activation.json", activation)
        self.assertIn("r2_prefix: $r2_prefix", activation)
        self.assertIn(".r2_prefix == $r2_prefix", activation)
        self.assertIn(".r2_prefix == $r2_prefix", convergence)
        for digest_field in (
            "server_image_digest",
            "control_plane_image_digest",
            "helm_chart_digest",
        ):
            self.assertIn(digest_field, activation)
            self.assertIn(digest_field, convergence)
            self.assertIn(digest_field, recovery_activation)
            self.assertIn(digest_field, commit_verdict)
        self.assertIn("committed=true", activation)
        self.assertIn("Immutably committed, attested, and publicly verified ${TAG}", activation)
        self.assertIn("Draft activation marker digest does not match", activation)
        self.assertIn(".immutable // false", activation)
        self.assertIn("verify-github-release-integrity.sh", activation)
        self.assertIn("verify-github-release-integrity.sh", convergence)
        self.assertIn("verify-github-release-integrity.sh", commit_verdict)
        for publication_job in (activation, recovery_activation):
            with self.subTest(publication_job=publication_job[:40]):
                self.assertIn(
                    "IMMUTABILITY_ADMIN_TOKEN: ${{ secrets.WORKFLOW_PAT }}",
                    publication_job,
                )
                self.assertIn("check-github-release-immutability.sh", publication_job)
                setting_check = publication_job.index(
                    "check-github-release-immutability.sh"
                )
                self.assertLess(
                    setting_check,
                    publication_job.index('-X PATCH --input', setting_check),
                )
        marker_upload = activation.index('gh release upload "${TAG}"')
        publish_patch = activation.index(
            '-X PATCH --input "$publish_payload"', marker_upload
        )
        commit_flip = activation.index("committed=true", publish_patch)
        activation_readback = activation.index("curl -fsSL --retry 12", commit_flip)
        self.assertIn(
            '--repo "${GITHUB_REPOSITORY}"',
            activation[marker_upload:publish_patch],
        )
        self.assertLess(marker_upload, publish_patch)
        self.assertLess(publish_patch, commit_flip)
        self.assertLess(commit_flip, activation_readback)

        # Failure injection: the draft marker is still compensatable, but once
        # immutable publication succeeds, a later public read-back failure may
        # not attempt to mutate the locked release.
        state = {"activated": False, "marker_staged": True, "committed": False}
        should_remove_marker = state["marker_staged"] and not state["committed"]
        self.assertTrue(should_remove_marker)
        state["activated"] = True
        state["committed"] = True
        activation_readback_succeeded = False
        should_quarantine = state["activated"] and not state["committed"]
        self.assertFalse(activation_readback_succeeded)
        self.assertFalse(should_quarantine)
        self.assertIn("Release Activation Commit Verdict", commit_verdict)
        self.assertIn("release-activation.json", commit_verdict)
        for forbidden in (
            "FLOATING_RESULT",
            "HELM_PAGES_RESULT",
            "PRIVATE_PRO_PROMOTION_RESULT",
            "DEMO_RESULT",
        ):
            self.assertNotIn(forbidden, commit_verdict)

        demo = workflow_job_block(convergence, "update_stable_demo")
        self.assertNotIn("release_id:", demo)
        demo_workflow = read(".github/workflows/update-demo-server.yml")
        self.assertNotIn("release_id:", demo_workflow)
        self.assertNotIn("unpublished draft", demo_workflow)

    def test_activation_recovery_reuses_the_qualified_candidate_without_rebuilding(self) -> None:
        release = read(".github/workflows/create-release.yml")
        recovery = read(".github/workflows/recover-release-activation.yml")
        job = workflow_job_block(recovery, "recover_activation")

        self.assertIn("group: release-v${{ github.event.inputs.version", release)
        self.assertIn("group: release-${{ inputs.tag }}", recovery)
        self.assertIn('.path == ".github/workflows/create-release.yml"', job)
        self.assertIn("release_readiness", job)
        self.assertIn("dispatch_release_convergence", job)
        self.assertIn("release_readiness is the canonical DAG join", job)
        self.assertNotIn("docker_build", job)
        self.assertNotIn("helm_smoke", job)
        self.assertIn("failure outside the recoverable activation boundary", job)
        self.assertIn("release-candidate-manifest-${source_sha}-${version}", job)
        self.assertIn("scripts/release_candidate_manifest.py verify-release", job)
        self.assertIn('--release-body-file "${release_body}"', job)
        self.assertLess(
            job.index("--jq '.body // \"\"' > \"${release_body}\""),
            job.index("scripts/release_candidate_manifest.py verify-release"),
        )
        self.assertIn('(any(.assets[]?; .name == "release-activation.json") | not)', job)
        self.assertIn("release-convergence.yml/dispatches", job)
        self.assertIn("return_run_details: true", job)
        self.assertIn("activation_recovery_run_id", job)
        self.assertIn("for attempt in $(seq 1 12)", job)
        self.assertIn("waiting for GitHub indexing", job)
        self.assertIn('--repo "${GITHUB_REPOSITORY}"', job)
        self.assertNotIn("build-release-candidate.yml", recovery)
        self.assertNotIn("scripts/build-release.sh", recovery)
        self.assertIn("Recovered draft activation marker digest does not match", job)
        self.assertIn(".immutable // false", job)
        self.assertIn("verify-github-release-integrity.sh", job)

        convergence = read(".github/workflows/release-convergence.yml")
        helm_pages_caller = workflow_job_block(convergence, "publish_helm_pages")
        self.assertIn("actions: read", helm_pages_caller)
        self.assertIn("contents: write", helm_pages_caller)

        marker_upload = job.index('gh release upload "${TAG}"')
        publish_patch = job.index(
            '-X PATCH --input "${publish_payload}"', marker_upload
        )
        committed = job.index("committed=true", publish_patch)
        readback = job.index("curl -fsSL --retry 12", committed)
        self.assertLess(marker_upload, publish_patch)
        self.assertLess(publish_patch, committed)
        self.assertLess(committed, readback)

        helm_pages = read(".github/workflows/helm-pages.yml")
        self.assertIn("Require activated GitHub release", helm_pages)
        self.assertIn("release-activation.json", helm_pages)
        self.assertIn("source_release_run_id", helm_pages)
        self.assertIn("Helm Pages refuses to advertise inactive", helm_pages)
        for command in ("view", "upload", "create", "edit"):
            with self.subTest(command=command):
                self.assertRegex(
                    helm_pages,
                    rf'gh release {command} .*?(?:\\\n\s*)?--repo "\$\{{GITHUB_REPOSITORY\}}"',
                )

        floating_tags = read(".github/workflows/promote-floating-tags.yml")
        self.assertIn("Require activated GitHub release", floating_tags)
        self.assertIn("--json isDraft,publishedAt,tagName", floating_tags)
        self.assertIn("Floating-tag promotion refuses inactive release", floating_tags)

    def test_each_post_commit_surface_failure_is_retriable_convergence_debt(self) -> None:
        release_workflow = read(".github/workflows/create-release.yml")
        convergence = read(".github/workflows/release-convergence.yml")
        retry = read(".github/workflows/retry-release-convergence.yml")
        commit_verdict = workflow_job_block(release_workflow, "release_commit_verdict")
        convergence_verdict = workflow_job_block(convergence, "convergence_verdict")

        surfaces = {
            "promote_floating_tags": ("FLOATING_RESULT", "Docker aliases"),
            "publish_helm_pages": ("HELM_PAGES_RESULT", "Helm Pages"),
            "promote_private_pro_runtime": ("PRIVATE_PRO_RESULT", "paid-runtime broker"),
            "update_stable_demo": ("DEMO_RESULT", "stable demo"),
        }
        for failed_job, (result_variable, surface_name) in surfaces.items():
            with self.subTest(failed_job=failed_job):
                injected_results = {job: "success" for job in surfaces}
                injected_results[failed_job] = "failure"
                release_state = "committed"
                convergence_state = (
                    "debt"
                    if any(result != "success" for result in injected_results.values())
                    else "converged"
                )
                self.assertEqual(release_state, "committed")
                self.assertEqual(convergence_state, "debt")
                self.assertNotIn(result_variable, commit_verdict)
                self.assertIn(result_variable, convergence_verdict)
                self.assertIn(surface_name, convergence_verdict)
                self.assertIn("the committed release remains public", convergence_verdict)

        self.assertIn("github.event.workflow_run.conclusion != 'success'", retry)
        self.assertIn("actions/runs/${RUN_ID}/rerun", retry)
        self.assertNotIn("rerun-failed-jobs", retry)
        self.assertIn("RUN_ATTEMPT >= MAX_ATTEMPTS", retry)
        self.assertIn("release-activation.json", retry)

    def test_mutating_reusable_workflows_have_no_direct_dispatch_lock_bypass(self) -> None:
        convergence = read(".github/workflows/release-convergence.yml")
        for workflow_path, convergence_job in (
            (".github/workflows/promote-floating-tags.yml", "promote_floating_tags"),
            (".github/workflows/helm-pages.yml", "publish_helm_pages"),
            (".github/workflows/update-demo-server.yml", "update_stable_demo"),
        ):
            with self.subTest(workflow_path=workflow_path):
                workflow = read(workflow_path)
                self.assertIn("workflow_call:", workflow)
                self.assertNotIn("workflow_dispatch:", workflow)
                self.assertIn(
                    "needs: acquire_customer_promotion_lease",
                    workflow_job_block(convergence, convergence_job),
                )

        dry_run = read(".github/workflows/release-dry-run.yml")
        demo_verification = workflow_job_block(dry_run, "demo_path_preflight")
        self.assertIn("uses: ./.github/workflows/update-demo-server.yml", demo_verification)
        self.assertIn("verify_only: true", demo_verification)

        retired_bootstrap = read(".github/workflows/deploy-demo-server.yml")
        self.assertIn("Verify Current Committed Stable Demo (No Mutation)", retired_bootstrap)
        self.assertIn("tag: latest", retired_bootstrap)
        self.assertIn("verify_only: true", retired_bootstrap)
        for forbidden in ("go build", "scp ", "docker compose", "verify_only: false"):
            self.assertNotIn(forbidden, retired_bootstrap)

        # Only the lease owner may call a mutator. The two demo callers outside
        # convergence are verification-only and cannot write the stable host.
        expected_callers = {
            "promote-floating-tags.yml": {"release-convergence.yml"},
            "helm-pages.yml": {"release-convergence.yml"},
            "update-demo-server.yml": {
                "release-convergence.yml",
                "release-dry-run.yml",
                "deploy-demo-server.yml",
            },
        }
        workflow_dir = REPO_ROOT / ".github" / "workflows"
        for callee, expected in expected_callers.items():
            callers = {
                path.name
                for path in workflow_dir.glob("*.yml")
                if f"uses: ./.github/workflows/{callee}" in read(str(path.relative_to(REPO_ROOT)))
            }
            self.assertEqual(callers, expected)

    def test_live_demo_and_paid_broker_require_exact_committed_lease_owner(self) -> None:
        convergence = read(".github/workflows/release-convergence.yml")
        demo_workflow = read(".github/workflows/update-demo-server.yml")
        demo_mutation = workflow_job_block(demo_workflow, "update-demo")
        demo_caller = workflow_job_block(convergence, "update_stable_demo")
        paid_caller = workflow_job_block(convergence, "promote_private_pro_runtime")
        paid_dispatch = read(".github/workflows/promote-private-pro-runtime.yml")

        for needle in (
            "Require exact committed activation marker for mutation",
            "inputs.verify_only != true",
            "release-activation.json",
            ".convergence_run_id == $convergence_run_id",
            "Stable demo mutation refuses mutable, inactive, or prerelease tag",
            "https://raw.githubusercontent.com/${{ github.repository }}/${LEASE_SHA}/${OWNER_ASSET_NAME}",
            ".immutable // false",
            ".schema_version == 2",
        ):
            self.assertIn(needle, demo_mutation)
        self.assertIn("tag: ${{ inputs.tag }}", demo_caller)
        self.assertIn("verify_only: false", demo_caller)
        self.assertIn("activation_convergence_run_id: ${{ github.run_id }}", demo_caller)
        self.assertIn("convergence_owner_asset_name:", demo_caller)
        self.assertIn("convergence_owner_asset_sha256:", demo_caller)

        self.assertIn(
            "pulse_lease_sha: ${{ needs.acquire_customer_promotion_lease.outputs.lock_sha }}",
            paid_caller,
        )
        self.assertIn("pulse_convergence_run_id: ${{ github.run_id }}", paid_caller)
        self.assertIn("pulse_owner_asset_name:", paid_caller)
        self.assertIn("pulse_owner_asset_sha256:", paid_caller)
        for needle in (
            "pulse_lease_sha: $pulse_lease_sha",
            "pulse_convergence_run_id: $pulse_convergence_run_id",
            "pulse_owner_asset_name: $pulse_owner_asset_name",
            "pulse_owner_asset_sha256: $pulse_owner_asset_sha256",
            "return_run_details: true",
            ".immutable == true",
            "https://raw.githubusercontent.com/${GITHUB_REPOSITORY}/${PULSE_LEASE_SHA}/${PULSE_OWNER_ASSET_NAME}",
            ".schema_version == 2",
            "Paid-runtime mutation is bound to immutable",
        ):
            self.assertIn(needle, paid_dispatch)

    def test_global_promotion_lease_serializes_and_rejects_out_of_order_channels(self) -> None:
        release_workflow = read(".github/workflows/create-release.yml")
        convergence = read(".github/workflows/release-convergence.yml")
        lease = workflow_job_block(convergence, "acquire_customer_promotion_lease")
        release_lease = workflow_job_block(convergence, "release_customer_promotion_lease")
        lease_script = read("scripts/release_control/customer_promotion_lease.sh")

        self.assertIn(
            "release-v${{ github.event.inputs.version || github.ref || github.run_id }}",
            release_workflow,
        )
        self.assertNotIn("release-customer-promotion-lock", release_workflow)
        self.assertIn("customer_promotion_lease.sh acquire", lease)
        self.assertIn("refs/heads/release-customer-promotion-lock", lease_script)
        self.assertIn("git push --atomic origin", lease_script)
        self.assertIn('"${lock_commit}:${owner_ref}"', lease_script)
        self.assertIn('"repos/${GITHUB_REPOSITORY}/git/refs"', lease_script)
        self.assertIn("Bootstrapped absent customer-promotion lease ref", lease_script)
        self.assertIn("--force-with-lease=\"${LOCK_REF}:${observed_sha}\"", lease_script)
        self.assertIn("owner_status", lease_script)
        self.assertIn("sort -Vr", lease)
        self.assertIn(".isPrerelease == $prerelease", lease)
        self.assertIn("release-activation.json", lease)
        self.assertIn("superseded=true", lease)
        self.assertIn("no global customer pointer will move backward", lease)
        self.assertIn("customer_promotion_lease.sh release", release_lease)
        self.assertIn("--force-with-lease=\"${LOCK_REF}:${lock_sha}\"", lease_script)
        self.assertIn('schema_version: 2', lease_script)
        self.assertIn('git hash-object -w "${owner_record}"', lease_script)
        self.assertIn('GIT_INDEX_FILE="${lock_index}" git update-index', lease_script)
        self.assertNotIn('gh release upload "${TAG}" "${owner_record}"', lease)

        helm_pages = workflow_job_block(convergence, "publish_helm_pages")
        convergence_verdict = workflow_job_block(convergence, "convergence_verdict")
        self.assertNotIn("superseded != 'true'", helm_pages)
        self.assertLess(
            convergence_verdict.index('require_success "Helm Pages"'),
            convergence_verdict.index('if [ "${SUPERSEDED}" = "true" ]'),
        )
        for rollback_prone_job in (
            "promote_floating_tags",
            "promote_private_pro_runtime",
            "update_stable_demo",
        ):
            self.assertIn(
                "superseded != 'true'",
                workflow_job_block(convergence, rollback_prone_job),
            )
        self.assertIn("without moving rollback-prone pointers", convergence_verdict)

        def version_key(tag: str) -> tuple[int, int, int, int, int]:
            match = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)(?:-rc\.(\d+))?", tag)
            self.assertIsNotNone(match)
            assert match is not None
            major, minor, patch, rc = match.groups()
            return (
                int(major),
                int(minor),
                int(patch),
                1 if rc is None else 0,
                int(rc or 0),
            )

        def admitted(target: str, committed_same_channel: tuple[str, ...]) -> bool:
            return target == max(committed_same_channel, key=version_key)

        self.assertFalse(admitted("v6.2.0", ("v6.2.0", "v6.2.1")))
        self.assertTrue(admitted("v6.2.1", ("v6.2.0", "v6.2.1")))
        self.assertFalse(admitted("v6.3.0-rc.1", ("v6.3.0-rc.1", "v6.3.0-rc.2")))
        self.assertTrue(admitted("v6.3.0-rc.2", ("v6.3.0-rc.1", "v6.3.0-rc.2")))
        self.assertTrue(admitted("v6.2.1", ("v6.2.1",)))
        self.assertTrue(admitted("v6.3.0-rc.2", ("v6.3.0-rc.2",)))

        helm_index = {"v6.2.1"}
        pointer_target = "v6.2.1"
        older = "v6.2.0"
        helm_index.add(older)
        if admitted(older, (older, pointer_target)):
            pointer_target = older
        self.assertEqual(helm_index, {"v6.2.0", "v6.2.1"})
        self.assertEqual(pointer_target, "v6.2.1")

        rc_helm_index = {"v6.3.0-rc.2"}
        rc_pointer_target = "v6.3.0-rc.2"
        older_rc = "v6.3.0-rc.1"
        rc_helm_index.add(older_rc)
        if admitted(older_rc, (older_rc, rc_pointer_target)):
            rc_pointer_target = older_rc
        self.assertEqual(rc_helm_index, {"v6.3.0-rc.1", "v6.3.0-rc.2"})
        self.assertEqual(rc_pointer_target, "v6.3.0-rc.2")

    def test_prepare_to_commit_handoff_survives_delayed_activation(self) -> None:
        release_workflow = read(".github/workflows/create-release.yml")
        convergence = read(".github/workflows/release-convergence.yml")
        retry = read(".github/workflows/retry-release-convergence.yml")
        activation = workflow_job_block(release_workflow, "activate_release")

        self.assertIn(
            "Release convergence ${{ inputs.tag }} source ${{ inputs.source_release_run_id }}",
            convergence,
        )
        await_commit = workflow_job_block(convergence, "await_activation_commit")
        self.assertIn("timeout-minutes: 360", await_commit)
        self.assertIn('max_attempts=10500', await_commit)
        self.assertIn('sleep 2', await_commit)
        self.assertIn("source_release_run_id", await_commit)
        self.assertIn('gh run view "${EXPECTED_SOURCE_RUN_ID}"', await_commit)
        self.assertIn('source_status}" = "completed"', await_commit)
        self.assertIn("completed without the exact activation marker", await_commit)
        self.assertIn('source_conclusion}" != "success"', await_commit)
        self.assertIn('select(.name == "release-activation.json") | .state', await_commit)
        self.assertIn('marker_asset_state}" != "uploaded"', await_commit)
        self.assertIn("max_completed_propagation_attempts=20", await_commit)
        self.assertIn("uploaded but not publicly readable yet", await_commit)
        self.assertIn("does not match the expected immutable release identity", await_commit)
        self.assertIn("EXPECTED_CONVERGENCE_RUN_ID: ${{ github.run_id }}", await_commit)
        self.assertIn('original_status}" != "completed"', await_commit)
        self.assertIn("successor adoption is not yet allowed", await_commit)
        self.assertIn("activation_marker_sha256", await_commit)

        self.assertIn("source_release_run_id=\"${BASH_REMATCH[2]}\"", retry)
        self.assertIn("actions/runs/${source_release_run_id}", retry)
        self.assertIn('source_status}" != "completed"', retry)
        self.assertIn("RUN_ATTEMPT < 50", retry)
        self.assertIn("renewing the pre-commit convergence owner", retry)

        self.assertIn("require_viable_convergence_owner()", activation)
        self.assertEqual(activation.count("require_viable_convergence_owner"), 3)
        self.assertIn('gh run view "${CONVERGENCE_RUN_ID}"', activation)
        self.assertIn("for attempt in $(seq 1 12)", activation)
        self.assertIn("waiting for GitHub indexing", activation)
        self.assertIn("--json event,status,conclusion,workflowName,displayTitle,url", activation)
        self.assertIn('expected_title="Release convergence ${TAG} source ${GITHUB_RUN_ID}"', activation)
        self.assertIn('[ "${owner_status}" = "completed" ]', activation)
        self.assertIn("before staging the exact marker", activation)
        self.assertIn("validate_existing_activation_commit", activation)
        self.assertIn(
            "Recover release activation ${TAG} source ${GITHUB_RUN_ID}", activation
        )
        self.assertIn(
            '.path == ".github/workflows/recover-release-activation.yml"',
            activation,
        )
        self.assertIn(
            '.path == ".github/workflows/release-convergence.yml"', activation
        )
        verdict = workflow_job_block(release_workflow, "release_commit_verdict")
        self.assertIn("activation_recovery_run_id", verdict)
        self.assertIn(
            '.status == "completed" and .conclusion == "success"', verdict
        )
        self.assertLess(
            activation.index(
                "require_viable_convergence_owner\n"
                "          if [ -z \"${IMMUTABILITY_ADMIN_TOKEN:-}\" ]"
            ),
            activation.index("-X PATCH --input \"$publish_payload\""),
        )
        marker_upload = activation.index('gh release upload "${TAG}"')
        final_owner_check = activation.rindex("require_viable_convergence_owner")
        publish_patch = activation.index(
            '-X PATCH --input "$publish_payload"', final_owner_check
        )
        self.assertLess(marker_upload, final_owner_check)
        self.assertLess(final_owner_check, publish_patch)

    def test_fresh_fixed_code_convergence_can_adopt_completed_original_owner(self) -> None:
        convergence = read(".github/workflows/release-convergence.yml")
        await_commit = workflow_job_block(convergence, "await_activation_commit")
        lease = workflow_job_block(convergence, "acquire_customer_promotion_lease")
        lease_script = read("scripts/release_control/customer_promotion_lease.sh")

        original_owner = "100"
        successor_owner = "200"
        original_status = "completed"
        marker_lineage = {
            "tag": "v6.2.0",
            "source_release_run_id": "50",
            "convergence_run_id": original_owner,
            "r2_prefix": "v6.2.0-pro-20260808-50",
        }
        successor_may_adopt = (
            marker_lineage["convergence_run_id"] != successor_owner
            and original_status == "completed"
        )
        self.assertTrue(successor_may_adopt)
        self.assertIn("Adopting committed ${TAG} from completed convergence owner", await_commit)
        self.assertIn("ACTIVATION_OWNER_RUN_ID", lease)
        self.assertIn("ACTIVATION_MARKER_SHA256", lease)
        self.assertIn(
            "release-convergence-owner-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}.json",
            lease_script,
        )
        self.assertIn("owner_asset_sha256", lease_script)
        self.assertIn("git commit-tree", lease_script)
        self.assertNotIn("gh release upload", lease)

        stale_name = f"release-convergence-owner-{original_owner}-5.json"
        successor_name = f"release-convergence-owner-{successor_owner}-1.json"
        self.assertNotEqual(stale_name, successor_name)

    def test_customer_promotion_lease_commit_contains_exact_owner_evidence(self) -> None:
        script = REPO_ROOT / "scripts" / "release_control" / "customer_promotion_lease.sh"
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            remote = root / "remote.git"
            checkout = root / "checkout"
            output = root / "github-output"
            subprocess.run(["git", "init", "--bare", str(remote)], check=True, capture_output=True)
            subprocess.run(["git", "init", str(checkout)], check=True, capture_output=True)
            for key, value in (
                ("user.name", "Pulse Test"),
                ("user.email", "pulse-test@example.invalid"),
            ):
                subprocess.run(
                    ["git", "config", key, value], cwd=checkout, check=True
                )
            (checkout / "README").write_text("base\n", encoding="utf-8")
            subprocess.run(["git", "add", "README"], cwd=checkout, check=True)
            subprocess.run(["git", "commit", "-m", "base"], cwd=checkout, check=True, capture_output=True)
            subprocess.run(["git", "remote", "add", "origin", str(remote)], cwd=checkout, check=True)
            subprocess.run(
                ["git", "push", "origin", "HEAD:refs/heads/release-customer-promotion-lock"],
                cwd=checkout,
                check=True,
                capture_output=True,
            )

            env = os.environ.copy()
            env.update(
                {
                    "GH_TOKEN": "test-token",
                    "GITHUB_REPOSITORY": "rcourtman/Pulse",
                    "GITHUB_RUN_ID": "200",
                    "GITHUB_RUN_ATTEMPT": "3",
                    "GITHUB_ACTOR": "pulse-test",
                    "GITHUB_ACTOR_ID": "1234",
                    "GITHUB_OUTPUT": str(output),
                    "TARGET_COMMITISH": "a" * 40,
                    "RELEASE_ID": "321",
                    "SOURCE_RELEASE_RUN_ID": "100",
                    "R2_PREFIX": "v6.5.0-pro-test",
                    "ACTIVATION_OWNER_RUN_ID": "150",
                    "ACTIVATION_MARKER_SHA256": "b" * 64,
                }
            )
            result = subprocess.run(
                [str(script), "acquire", "v6.5.0"],
                cwd=checkout,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            outputs = dict(
                line.split("=", 1)
                for line in output.read_text(encoding="utf-8").splitlines()
            )
            self.assertRegex(outputs["lock_sha"], r"^[0-9a-f]{40}$")
            self.assertEqual(
                outputs["owner_asset_name"],
                "release-convergence-owner-200-3.json",
            )
            record_text = subprocess.run(
                [
                    "git",
                    "show",
                    f"{outputs['lock_sha']}:{outputs['owner_asset_name']}",
                ],
                cwd=checkout,
                text=True,
                capture_output=True,
                check=True,
            ).stdout
            record = json.loads(record_text)
            self.assertEqual(record["schema_version"], 2)
            self.assertEqual(record["convergence_run_id"], "200")
            self.assertEqual(record["activation_owner_run_id"], "150")
            self.assertNotIn("lease_sha", record)
            self.assertEqual(
                outputs["owner_asset_sha256"],
                hashlib.sha256(record_text.encode()).hexdigest(),
            )
            retained = subprocess.run(
                [
                    "git",
                    "ls-remote",
                    "origin",
                    "refs/tags/release-convergence-owner-200-3",
                ],
                cwd=checkout,
                text=True,
                capture_output=True,
                check=True,
            ).stdout.split()[0]
            self.assertEqual(retained, outputs["lock_sha"])

    def test_private_dispatches_wait_for_the_exact_created_run(self) -> None:
        release_workflow = read(".github/workflows/create-release.yml")
        private_promotion_workflow = read(
            ".github/workflows/promote-private-pro-runtime.yml"
        )
        cases = (
            (
                release_workflow,
                "stage_private_pro_runtime",
                "repos/rcourtman/pulse-enterprise/actions/workflows/build-pro-release.yml/dispatches",
                "build_run_id",
            ),
            (
                private_promotion_workflow,
                "promote",
                "repos/rcourtman/pulse-pro/actions/workflows/promote-paid-runtime-release.yml/dispatches",
                "promote_run_id",
            ),
        )

        for workflow, job_name, endpoint, run_id_variable in cases:
            with self.subTest(job_name=job_name):
                job = workflow_job_block(workflow, job_name)
                self.assertIn("return_run_details: true", job)
                self.assertIn(
                    '-H "X-GitHub-Api-Version: 2026-03-10"',
                    job,
                )
                self.assertIn(endpoint, job)
                self.assertIn(
                    f'{run_id_variable}="$(jq -r \'.workflow_run_id // empty\'',
                    job,
                )
                self.assertIn(f'"${{{run_id_variable}}}"', job)
                self.assertIn('local run_id="$2"', job)
                self.assertIn('gh run view "${run_id}"', job)
                self.assertIn("did not return an exact workflow run ID", job)
                self.assertNotIn("gh run list", job)
                self.assertNotIn("started_at", job)

    def test_release_promotion_policy_requires_live_rc_and_v5_policy(self) -> None:
        content = read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        self.assertIn("`beta.N` is the normal user-testing stage", content)
        self.assertIn("`rc.N` is reserved for a build the release owner believes", content)
        self.assertIn("Stable promotion lineage must come from a published `rc.N`", content)
        self.assertIn("live run of the release pipeline for the prerelease tag itself", content)
        self.assertIn("an accidental git tag by itself", content)
        self.assertIn("does not count as a shipped prerelease", content)
        self.assertIn("do not promote to `stable` until the active control-plane target", content)
        self.assertIn("A live release-pipeline exercise already completed for the promoted prerelease tag", content)
        self.assertIn("maintenance-only window lasts 90 calendar days", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("release notes may keep placeholder", content)
        self.assertIn("Exact v6 GA and v5 end-of-support dates locked before GA publish", content)
        self.assertIn("governed prerelease and stable release branches", content)
        self.assertIn("Customer-facing private Pulse Pro archives", content)
        self.assertIn("public alpha, beta, or RC tag", content)
        self.assertIn("license.pulserelay.pro/pulse-pro:6.0.0", content)
        self.assertIn("moving branch", content)
        self.assertIn("`implemented`", content)
        self.assertIn("`release-validated`", content)
        self.assertIn("`live-verified`", content)
        self.assertIn("scripts/release_control/live_runtime_proof.py", content)
        self.assertIn("an operator statement cannot substitute for the receipt", content)


    def test_live_runtime_claim_rejects_unknown_successful_posture(self) -> None:
        observed, failures = evaluate_live_runtime(
            expected_version="6.2.0-rc.8",
            observed_version="6.2.0-rc.8",
            postures=[
                {
                    "subjectResourceId": "vm-100",
                    "state": "unknown",
                    "lastSuccessfulPointAt": "2026-08-04T10:00:00Z",
                }
            ],
            minimum_postures=1,
            minimum_successful_postures=1,
        )
        self.assertEqual(observed["unknownWithSuccessfulPointCount"], 1)
        self.assertTrue(any("remain unknown" in failure for failure in failures))

    def test_v6_ga_owner_risk_exception_is_bounded_and_packet_aligned(self) -> None:
        policy = read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        checklist = read("docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md")
        owner_record = read(
            "docs/release-control/v6/internal/records/current-branch-ga-owner-approval-2026-07-02.md"
        )
        release_notes = read("docs/releases/RELEASE_NOTES_v6.md")
        v5_policy = read("docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md")
        status = read_json("docs/release-control/v6/internal/status.json")
        control_plane = read_json("docs/release-control/control_plane.json")

        normalized_policy = normalize_ws(policy)
        normalized_checklist = normalize_ws(checklist)
        normalized_owner_record = normalize_ws(owner_record)
        normalized_release_notes = normalize_ws(release_notes)
        normalized_v5_policy = normalize_ws(v5_policy)

        self.assertIn("v6.0.0 owner-risk exception", policy)
        self.assertIn("bounded v6.0.0 release-owner risk acceptance", normalized_policy)
        self.assertIn("not validation evidence for the post-RC7 changes", normalized_policy)
        self.assertIn("not a standing policy for later stable releases", normalized_policy)
        self.assertIn("Additional RC or soak required: no", normalized_owner_record)
        self.assertIn("Additional current-branch validation required before GA: no", normalized_owner_record)
        self.assertIn("accepting the remaining current-branch validation risk", normalized_owner_record)
        self.assertIn("not validation evidence for the post-RC7", normalized_checklist)

        self.assertIn("current `pulse/v6-release` branch", normalized_release_notes)
        self.assertIn("seven release candidates and accumulated post-RC7 fixes", normalized_release_notes)
        self.assertIn("Pulse v5 entered maintenance-only support on `2026-07-04`.", normalized_release_notes)
        self.assertIn("existing v5 users until `2026-10-02`.", normalized_release_notes)
        self.assertIn("Pulse v5 entered maintenance-only support on 2026-07-04.", normalized_v5_policy)
        self.assertIn("2026-10-02. After 2026-10-02", normalized_v5_policy)

        gate = next(gate for gate in status["release_gates"] if gate["id"] == "rc-to-ga-promotion-readiness")
        self.assertEqual(gate["status"], "passed")
        self.assertIn("owner risk acceptance", gate["summary"])
        self.assertIn(
            "docs/release-control/v6/internal/records/current-branch-ga-owner-approval-2026-07-02.md",
            {item["path"] for item in gate["evidence"]},
        )

        decisions = {decision["id"]: decision for decision in status["resolved_decisions"]}
        self.assertEqual(
            decisions["current-branch-ga-owner-risk-acceptance"]["decided_at"],
            "2026-07-02",
        )

        ga_target = next(target for target in control_plane["targets"] if target["id"] == "v6-ga-promotion")
        self.assertIn("Pulse v6 GA", ga_target["summary"])
        self.assertIn("main is the canonical latest-and-greatest branch", ga_target["summary"])

    def test_pre_release_checklist_tracks_rc_to_ga_gate_inputs(self) -> None:
        content = read("docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("release pipeline has already been exercised on a real prerelease tag", content)
        self.assertIn("not an accidental git tag", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("replace any placeholder GA notice dates", content)
        self.assertIn("rc-to-ga-rehearsal-summary", content)
        self.assertIn("rc-to-ga-promotion-readiness", content)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", content)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", content)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(content))

    def test_v5_support_policy_and_release_notes_publish_exact_notice(self) -> None:
        policy = read("docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md")
        release_notes = read("docs/releases/RELEASE_NOTES_v6.md")
        self.assertIn("maintenance-only support immediately on the v6 GA date", policy)
        self.assertIn("90 calendar days from the v6 GA", policy)
        self.assertIn("pulse/v5-maintenance", policy)
        if "Pulse v5 Support Transition" in release_notes:
            self.assertIn("publish an explicit exception", release_notes)
            self.assertRegex(
                release_notes,
                re.compile(r"Pulse v5 entered maintenance-only support on `(?:\[v6-ga-date\]|\d{4}-\d{2}-\d{2})`\.")
            )
            self.assertRegex(
                release_notes,
                re.compile(r"existing v5 users until `(?:\[v5-eos-date\]|\d{4}-\d{2}-\d{2})`\.")
            )
        else:
            self.assertRegex(release_notes, re.compile(r"(Pre-Release Notes|Release Candidate Notes)"))
            self.assertRegex(release_notes, re.compile(r"(final GA release|stable `v6\.0\.0` release)"))
            self.assertNotIn("Pulse v5 Support Transition", release_notes)

    def test_release_notes_index_points_at_current_rc_packet(self) -> None:
        release_index = read("docs/RELEASE_NOTES.md")
        # Stable + shipped-RC1 packet paths are hardcoded because they don't
        # follow the *_DRAFT.md naming pattern that distinguishes in-flight
        # prerelease packets.
        for path in (
            "docs/releases/RELEASE_NOTES_v6.md",
            "docs/releases/V6_CHANGELOG.md",
            "docs/UPGRADE_v6.md",
            "docs/releases/RELEASE_NOTES_v6_RC1.md",
            "docs/releases/V6_CHANGELOG_RC1.md",
            "docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md",
        ):
            self.assertIn(path, release_index)
        # Every discovered RC draft packet (rc.2 onward) must be linked.
        for _, release_notes, changelog, support_pack in discover_rc_draft_packets():
            with self.subTest(packet=release_notes):
                self.assertIn(release_notes, release_index)
                self.assertIn(changelog, release_index)
                self.assertIn(support_pack, release_index)

    def test_operator_support_packs_keep_free_first_paid_continuity_wording(self) -> None:
        support_pack_paths = ("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md",) + tuple(
            support_pack for _, _, _, support_pack in discover_rc_draft_packets()
        )
        for rel in support_pack_paths:
            with self.subTest(rel=rel):
                support_pack = read(rel)
                self.assertIn(
                    "keep the current recurring price, with self-hosted monitoring and",
                    support_pack,
                )
                self.assertIn("child-resource volume not metered", support_pack)
                self.assertIn("core monitoring included", support_pack)
                self.assertNotIn("uncapped monitored-system plus guest", support_pack)
                self.assertNotIn("uncapped monitored-system and guest capacity", support_pack)
                self.assertNotIn("guest-capacity continuity", support_pack)
                self.assertNotIn("core monitoring unlimited", normalize_ws(support_pack))

    def test_stable_release_packet_describes_platform_shaped_frontend_on_unified_backend(self) -> None:
        """After the rc.6 IA revert, the stable v6 release docs must describe
        the frontend as platform-shaped (Proxmox, Docker, Kubernetes, TrueNAS,
        vSphere, Standalone) on a unified resource backend. Drift back to the
        rc.1-rc.5 unified Infrastructure/Workloads/Storage/Recovery framing in
        the stable packet would mislead operators reading the canonical v6
        release notes.
        """
        stable_docs = {
            "release_notes": read("docs/releases/RELEASE_NOTES_v6.md"),
            "changelog": read("docs/releases/V6_CHANGELOG.md"),
            "operator_support_pack": read("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md"),
        }

        for name, content in stable_docs.items():
            with self.subTest(name=name):
                normalized = normalize_ws(content)
                # Anti-patterns for prior product shapes the stable packet
                # must not describe as current truth.
                self.assertNotIn("default route lands on `Dashboard`", normalized)
                self.assertNotIn("around Dashboard, Infrastructure", normalized)
                self.assertNotIn(
                    "`Dashboard`, `Infrastructure`, `Workloads`, `Storage`, and `Recovery`",
                    content,
                )
                self.assertNotIn("- `Dashboard`", content)
                # Anti-patterns for the reverted unified IA so the stable
                # packet does not silently drift back to it.
                self.assertNotIn(
                    "Authenticated users now land on `Infrastructure`",
                    normalized,
                )
                self.assertNotIn(
                    "default route lands on `Infrastructure`",
                    normalized,
                )
                self.assertNotIn(
                    "Infrastructure as the default landing surface",
                    content,
                )

        self.assertIn(
            "platform-shaped top-level navigation",
            normalize_ws(stable_docs["release_notes"]),
        )
        self.assertIn(
            "platform-shaped",
            normalize_ws(stable_docs["changelog"]),
        )
        self.assertIn(
            "platform-shaped top-level pages",
            normalize_ws(stable_docs["operator_support_pack"]),
        )

    def test_rc1_changelog_keeps_current_free_first_licensing_posture(self) -> None:
        changelog = read("docs/releases/V6_CHANGELOG_RC1.md")
        normalized = normalize_ws(changelog)
        self.assertIn("Pricing/limit note", changelog)
        self.assertIn("include core monitoring by default", normalized)
        self.assertIn("not a monitoring-volume paywall", normalized)
        self.assertNotIn(
            "monitored-system limits, commercial posture, and trial eligibility",
            changelog,
        )
        self.assertNotIn("Limits are applied to canonical top-level monitored systems", changelog)

    def test_version_file_matches_current_rc_packet(self) -> None:
        current_version = read("VERSION").strip()
        release_index = read("docs/RELEASE_NOTES.md")
        stable_packet_paths = stable_packet_paths_for_version(current_version)
        if stable_packet_paths is not None:
            release_notes_path, changelog_path = stable_packet_paths
            release_notes = read(release_notes_path)
            changelog = read(changelog_path)
            self.assertIn(release_notes_path, release_index)
            self.assertIn(changelog_path, release_index)
            self.assertIn(f"Pulse v{current_version} Release Notes", release_notes)
            self.assertIn(f"`v{current_version}`", release_notes)
            self.assertIn(f"Pulse v{current_version}", changelog)
        else:
            packet_paths = rc_packet_paths_for_version(current_version)
            support_packet_paths = support_prerelease_packet_paths_for_version(current_version)
            self.assertTrue(
                packet_paths is not None or support_packet_paths is not None,
                f"VERSION={current_version} does not match a governed v6 prerelease packet pattern",
            )
            if packet_paths is not None:
                release_notes_path, changelog_path, operator_pack_path = packet_paths

                release_notes = read(release_notes_path)
                changelog = read(changelog_path)
                operator_pack = read(operator_pack_path)

                self.assertIn(f"current in-repo v6 `rc.{current_version.rsplit('.', 1)[1]}` draft packet", release_index)
                self.assertIn(release_notes_path, release_index)
                self.assertIn(changelog_path, release_index)
                self.assertIn(operator_pack_path, release_index)
                self.assertNotIn("current stable v6 release packet", release_index)
                self.assertIn(f"Pulse v{current_version} Draft Release Notes", release_notes)
                self.assertIn(f"`v{current_version}`", release_notes)
                self.assertIn(f"Pulse v{current_version} Draft Changelog", changelog)
                self.assertIn(f"`v{current_version}`", operator_pack)
            else:
                assert support_packet_paths is not None
                release_notes_path, changelog_path = support_packet_paths

                release_notes = read(release_notes_path)
                changelog = read(changelog_path)

                self.assertIn("current v6 support release candidate packet", release_index)
                self.assertIn(release_notes_path, release_index)
                self.assertIn(changelog_path, release_index)
                self.assertIn(f"Pulse v{current_version} Release Notes", release_notes)
                self.assertIn(f"`v{current_version}`", release_notes)
                self.assertIn(f"Pulse v{current_version}", changelog)

    def test_v611_packet_records_proxmox_backup_posture_identity_fix(self) -> None:
        release_notes = normalize_ws(read("docs/releases/RELEASE_NOTES_v6.1.1.md"))
        changelog = normalize_ws(read("docs/releases/V6_CHANGELOG_v6.1.1.md"))

        self.assertIn(
            "Proxmox backup and snapshot recovery points preserve canonical workload identity",
            release_notes,
        )
        self.assertIn("obsolete protection-posture row is removed", release_notes)
        self.assertIn(
            "Proxmox backup, snapshot, and PBS recovery points preserve canonical workload identity",
            changelog,
        )
        self.assertIn("posture refresh removes obsolete rows", changelog)

    def test_upgrade_guide_points_at_current_rc_support_pack(self) -> None:
        upgrade_guide = read("docs/UPGRADE_v6.md")
        current_version = read("VERSION").strip()
        self.assertIn("sudo /bin/update --version vX.Y.Z", upgrade_guide)
        self.assertIn("follow the signed server-installer flow in [INSTALL.md](INSTALL.md)", upgrade_guide)
        self.assertIn("the historical Pulse update signer was not recovered", normalize_ws(upgrade_guide))
        self.assertIn("manual reinstall or other explicit trust migration", normalize_ws(upgrade_guide))
        self.assertIn("### License and Entitlements", upgrade_guide)
        self.assertNotIn("### License, Trial, and Entitlements", upgrade_guide)
        self.assertIn("does not expose a general in-app trial, trial-return callback, or hosted AI quickstart", normalize_ws(upgrade_guide))
        self.assertIn(
            "Self-hosted monitoring and child-resource volume are not metered under the current v6 policy",
            upgrade_guide,
        )
        self.assertIn("monitored-system, guest, or child-resource volume cap", upgrade_guide)
        self.assertNotIn("uncapped monitored-system and guest capacity", upgrade_guide)
        self.assertNotIn("uncapped capacity automatically", upgrade_guide)
        self.assertNotIn("`POST /api/license/trial/start`", upgrade_guide)
        self.assertNotIn("signed activation token to `/auth/trial-activate`", upgrade_guide)
        self.assertNotIn("25 hosted Patrol", upgrade_guide)
        stable_packet_paths = stable_packet_paths_for_version(current_version)
        if stable_packet_paths is not None:
            release_notes_path, changelog_path = stable_packet_paths
            self.assertIn(release_notes_path, upgrade_guide)
            self.assertIn(changelog_path, upgrade_guide)
            for _, _, _, support_pack in discover_rc_draft_packets():
                self.assertNotIn(support_pack, upgrade_guide)
            self.assertNotIn("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md", upgrade_guide)
        else:
            packet_paths = rc_packet_paths_for_version(current_version)
            support_packet_paths = support_prerelease_packet_paths_for_version(current_version)
            self.assertTrue(
                packet_paths is not None or support_packet_paths is not None,
                f"VERSION={current_version} does not match a governed v6 prerelease packet pattern",
            )
            if packet_paths is not None:
                current_support_pack = packet_paths[2]
                self.assertIn(current_support_pack, upgrade_guide)
                for _, _, _, support_pack in discover_rc_draft_packets():
                    if support_pack == current_support_pack:
                        continue
                    self.assertNotIn(support_pack, upgrade_guide)
                self.assertNotIn("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md", upgrade_guide)
            else:
                assert support_packet_paths is not None
                release_notes_path, changelog_path = support_packet_paths
                self.assertIn(release_notes_path, upgrade_guide)
                self.assertIn(changelog_path, upgrade_guide)
                for _, _, _, support_pack in discover_rc_draft_packets():
                    self.assertNotIn(support_pack, upgrade_guide)
                self.assertNotIn("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md", upgrade_guide)

    def test_prerelease_feedback_template_uses_generic_current_rc_wording(self) -> None:
        template = read(".github/ISSUE_TEMPLATE/v6_rc_feedback.yml")
        self.assertIn("placeholder: v6.0.0-rc.N", template)
        self.assertIn("placeholder: rcourtman/pulse:v6.0.0-rc.N or pulse-linux-amd64", template)
        self.assertIn("Upgrade to the current v6 RC build", template)
        self.assertNotIn("v6.0.0-rc.1", template)

    def test_demo_site_copy_points_at_current_release_packet_index(self) -> None:
        demo_copy = read("docs/releases/V6_RC_DEMO_SITE_COPY.md")
        self.assertIn("docs/RELEASE_NOTES.md", demo_copy)
        self.assertIn("Current RC packet: `docs/releases/`", demo_copy)
        self.assertNotIn("docs/releases/RELEASE_NOTES_v6.md", demo_copy)
        self.assertNotIn("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md", demo_copy)

    def test_signpath_test_signing_workflow_is_non_publishing_and_test_only(self) -> None:
        workflow = read(".github/workflows/signpath-test-signing.yml")

        self.assertIn("workflow_dispatch:", workflow)
        self.assertNotIn("pull_request:", workflow)
        self.assertNotIn("schedule:", workflow)
        self.assertNotIn("push:", workflow)
        self.assertIn("SignPath Test Signing Proof (Never Publish)", workflow)
        self.assertIn("if ($env:GITHUB_REF_NAME -ne 'main')", workflow)
        self.assertNotIn("'${{ inputs.version }}'", workflow)
        self.assertIn("does not match repository VERSION", workflow)
        self.assertIn("signing-policy-slug: test-signing", workflow)
        self.assertNotIn(
            "signing-policy-slug: ${{ vars.SIGNPATH_SIGNING_POLICY_SLUG }}",
            workflow,
        )
        self.assertIn(
            "The canonical SIGNPATH_SIGNING_POLICY_SLUG must remain release-signing.",
            workflow,
        )
        self.assertIn(
            "signpath/github-action-submit-signing-request@b9d91eadd323de506c0c81cf0c7fe7438f3360fd # v2",
            workflow,
        )
        self.assertIn("signedArtifactsPublished = $false", workflow)
        self.assertIn("signedArtifactsUploadedAsGitHubArtifact = $false", workflow)
        self.assertIn("nonProduction = $true", workflow)
        self.assertIn("WaitForExit(90000)", workflow)
        self.assertIn("System32\\certutil.exe", workflow)
        self.assertIn("WaitForExit(30000)", workflow)
        self.assertIn("certutil timed out after 30 seconds", workflow)
        self.assertIn("'-addstore'", workflow)
        self.assertIn("'-delstore'", workflow)
        self.assertNotIn("'-user'", workflow)
        self.assertIn(
            "ephemeralTestTrustScope = 'LocalMachine/Root and LocalMachine/TrustedPublisher'",
            workflow,
        )
        self.assertNotIn("X509Store]::new", workflow)
        self.assertNotIn("Import-Certificate", workflow)
        self.assertIn("path: signpath-test-signing-evidence.json", workflow)
        self.assertEqual(
            workflow.count(
                "uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1"
            ),
            2,
        )
        self.assertNotIn("path: signpath-test-output", workflow)
        self.assertNotIn("gh release", workflow)
        self.assertNotIn("release-candidate", workflow)
        for name in (
            "pulse-agent-windows-amd64.exe",
            "pulse-agent-windows-arm64.exe",
            "pulse-agent-windows-386.exe",
        ):
            self.assertIn(name, workflow)

    def test_update_demo_server_workflow_uses_stable_tag_example(self) -> None:
        workflow = read(".github/workflows/update-demo-server.yml")
        self.assertIn("Stable release tag to deploy", workflow)
        self.assertIn("Prerelease demo updates are retired after v6 GA", workflow)
        self.assertNotIn("v6.0.0-rc.1", workflow)

    def test_rehearsal_template_and_workflow_capture_ga_rehearsal_record(self) -> None:
        template = read("docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md")
        workflow = read(".github/workflows/release-dry-run.yml")
        release_workflow = read(".github/workflows/create-release.yml")
        dry_run_trigger = read("scripts/trigger-release-dry-run.sh")
        recorder = read("scripts/release_control/record_rc_to_ga_rehearsal.py")
        internal_recorder = read("scripts/release_control/internal/record_rc_to_ga_rehearsal.py")
        renderer = read("scripts/release_control/render_release_body.py")
        generator = read("scripts/generate-release-notes.sh")
        release_notes_template = read("docs/releases/RELEASE_NOTES_TEMPLATE.md")
        resolver = read("scripts/release_control/resolve_release_promotion.py")
        self.assertIn("GitHub Actions run URL", template)
        self.assertIn("Exact GA date to publish with GA", template)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", template)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", template)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(template))
        self.assertIn("rc-to-ga-rehearsal-summary", workflow)
        self.assertIn("build_release_candidate:", workflow)
        self.assertIn("if: ${{ inputs.version != '' }}", workflow)
        self.assertIn("require_macos_signing: true", workflow)
        self.assertIn(
            "require_windows_signing: false",
            workflow,
        )
        self.assertIn("WINDOWS_AUTHENTICODE_AVAILABLE", workflow)
        self.assertIn("unsigned_windows_exception:", workflow)
        self.assertIn("unsigned_windows_reason:", workflow)
        self.assertIn("windows_signing_backend: signpath", workflow)
        self.assertRegex(
            workflow,
            r"(?ms)^  build_release_candidate:\n.*?^    permissions:\n"
            r"      actions: write\n      attestations: write\n      contents: read\n"
            r"      id-token: write\n    uses: \.\/\.github\/workflows\/build-release-candidate\.yml$",
        )
        self.assertRegex(
            release_workflow,
            r"(?ms)^  build_release_candidate:\n.*?^    permissions:\n"
            r"      actions: write\n      attestations: write\n      contents: read\n"
            r"      id-token: write\n    uses: \.\/\.github\/workflows\/build-release-candidate\.yml$",
        )
        self.assertIn("Definitive Dry-Run Verdict", workflow)
        self.assertIn('require_result "exact-SHA release candidate" "$CANDIDATE_RESULT" success', workflow)
        self.assertIn('require_result "stable demo no-mutation verification" "$DEMO_RESULT" success', workflow)
        self.assertNotIn("if: ${{ github.event_name == 'workflow_dispatch' }}", workflow)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id ${{ github.run_id }}", workflow)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", workflow)
        self.assertIn("control_plane.py --branch-for-version", workflow)
        self.assertIn('git fetch --prune origin main "${REQUIRED_BRANCH}" --tags', workflow)
        self.assertIn("resolve_release_promotion.py", workflow)
        self.assertIn("- Rollback command:", workflow)
        self.assertIn("- Candidate stable tag:", workflow)
        self.assertIn("- Promotion channel:", workflow)
        self.assertIn("- Promoted prerelease tag:", workflow)
        self.assertIn("Prerelease soak hours at rehearsal time", workflow)
        self.assertIn("Planned GA date", workflow)
        self.assertIn("Planned v5 end-of-support date", workflow)
        self.assertIn("go test -p 1 ./...", workflow)
        self.assertIn("2-core hosted runners", workflow)
        self.assertIn("resolve_release_promotion.py", release_workflow)
        self.assertIn("render_release_body.py", release_workflow)
        self.assertIn("build_rollback_section", renderer)
        self.assertIn("# Pulse v${VERSION} Release Notes", generator)
        self.assertIn("## What's improved", generator)
        self.assertIn("## What's improved", release_notes_template)
        self.assertIn("promotion-metadata", release_notes_template)
        self.assertIn("default_output_path", internal_recorder)
        self.assertIn("output path already exists", internal_recorder)
        self.assertIn("default_output_path", recorder)
        self.assertIn("rollback_version is required for every release rehearsal and promotion", resolver)
        self.assertIn("Stable promotion requires promoted_from_tag", resolver)
        self.assertIn("Only governed stable patch releases may use the routine no-RC path.", resolver)
        self.assertIn("Stable v6.0.0 requires ga_date in YYYY-MM-DD form", resolver)
        self.assertIn("release_notes must include the exact ga_date", resolver)
        self.assertIn("check-workflow-dispatch-inputs.py", dry_run_trigger)
        self.assertIn('--branch "$CURRENT_BRANCH"', dry_run_trigger)
        self.assertIn("release-dry-run.yml", dry_run_trigger)
        self.assertIn("gh workflow run release-dry-run.yml", dry_run_trigger)
        self.assertIn("Release Dry Run executes the selected remote ref", dry_run_trigger)
        self.assertIn("Hotfix exception to bypass 72-hour prerelease soak? [y/N]", dry_run_trigger)
        self.assertIn("blank only for approved hotfix", dry_run_trigger)
        self.assertIn('if [ -z "$PROMOTED_FROM_TAG" ] && [ "$HOTFIX_EXCEPTION" != "true" ]; then', dry_run_trigger)
        self.assertNotIn("Continue anyway?", dry_run_trigger)
        self.assertIn('if [ "${REHEARSAL_CONCLUSION}" != "success" ]; then', workflow)
        self.assertIn("did not produce a valid promotion metadata envelope", workflow)
        self.assertIn("Do not use this artifact to clear", workflow)

    def test_release_workflow_enforces_rc_lineage_soak_and_v5_notice(self) -> None:
        content = read(".github/workflows/create-release.yml")
        update_demo_workflow = read(".github/workflows/update-demo-server.yml")
        deploy_demo_workflow = read(".github/workflows/deploy-demo-server.yml")
        demo_ssh_helper = read(".github/scripts/setup-demo-ssh.sh")
        demo_reachability_helper = read(".github/scripts/check-demo-reachability.sh")
        validation_workflow = read(".github/workflows/validate-release-assets.yml")
        candidate_workflow = read(".github/workflows/build-release-candidate.yml")
        compiler_workflow = read(".github/workflows/compile-release-payload.yml")
        qualifier_workflow = read(".github/workflows/qualify-release-containers.yml")
        docker_build = workflow_job_block(qualifier_workflow, "qualify")
        release_validator = read("scripts/validate-release.sh")
        helper = read("scripts/trigger-release.sh")
        stable_patch_helper = read("scripts/trigger-stable-patch.sh")
        renderer = read("scripts/release_control/render_release_body.py")
        policy = read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        source_of_truth = read("docs/release-control/v6/internal/SOURCE_OF_TRUTH.md")
        runbook = read("docs/releases/V6_PRERELEASE_RUNBOOK.md")
        resolver = read("scripts/release_control/resolve_release_promotion.py")
        dry_run_workflow = read(".github/workflows/release-dry-run.yml")
        dry_run_helper = read("scripts/trigger-release-dry-run.sh")
        contract = read("docs/release-control/v6/internal/subsystems/deployment-installability.md")
        self.assertIn("It does not automatically check out or build `pulse-enterprise`.", runbook)
        self.assertIn("public `pulse-v...` release archives are OSS runtime artifacts", runbook)
        self.assertIn("`pulse-pro-v...` archives", runbook)
        self.assertIn("`bin/pulse --version` identifies `Pulse Pro`", runbook)
        self.assertIn("Paid-user GA is part of that same release boundary", contract)
        self.assertIn(
            "the public Pulse release workflow builds OSS `pulse-v...` artifacts only",
            normalize_ws(contract),
        )
        self.assertIn("`pulse-pro-v...` archives identify `Pulse Pro`", normalize_ws(contract))
        self.assertIn("https://pulserelay.pro/download.html", contract)
        self.assertIn("PULSE_IMAGE`-aware compose image line", normalize_ws(contract))
        self.assertIn("hardcoded `image: rcourtman/pulse:...`", normalize_ws(contract))
        self.assertIn("Resolving governed promotion metadata...", helper)
        self.assertIn("--release-notes-file \"$NOTES_FILE\"", helper)
        self.assertIn("--validate-notes-file \"$NOTES_FILE\"", helper)
        self.assertIn("--rawfile release_notes \"$NOTES_FILE\"", helper)
        self.assertIn("gh workflow run create-release.yml --ref \"$CURRENT_BRANCH\" --json", helper)
        self.assertIn('--arg hotfix_exception "$HOTFIX_EXCEPTION"', helper)
        self.assertIn(
            '--arg unsigned_windows_exception "$UNSIGNED_WINDOWS_EXCEPTION"',
            helper,
        )
        self.assertIn('--arg draft_only "false"', helper)
        self.assertNotIn("--argjson", helper)
        self.assertNotIn('-f release_notes="$(cat "$NOTES_FILE")"', helper)
        self.assertIn("--validate-notes-file \"$NOTES_FILE\"", stable_patch_helper)
        self.assertIn("--rawfile release_notes \"$NOTES_FILE\"", stable_patch_helper)
        self.assertIn("gh workflow run create-release.yml --ref \"$CURRENT_BRANCH\" --json", stable_patch_helper)
        self.assertIn("blank for hotfix with no RC lineage", helper)
        self.assertIn('if [ -z "$PROMOTED_FROM_TAG" ] && [ "$HOTFIX_EXCEPTION" != "true" ]; then', helper)
        self.assertIn("control_plane.py --branch-for-version", content)
        self.assertIn('git fetch --prune origin main "${REQUIRED_BRANCH}" --tags', content)
        self.assertIn('REQUIRED_BRANCH: ${{ steps.branch_policy.outputs.required_branch }}', content)
        self.assertIn("resolve_release_promotion.py", content)
        self.assertIn("release_stage: ${{ steps.promotion.outputs.release_stage }}", content)
        self.assertIn("--enforce-prerelease-observation-window", content)
        self.assertIn("DRAFT_ONLY_INPUT", content)
        self.assertIn("--enforce-prerelease-observation-window", helper)
        self.assertNotIn("--enforce-prerelease-observation-window", dry_run_workflow)
        self.assertNotIn("--enforce-prerelease-observation-window", dry_run_helper)
        self.assertIn("render_release_body.py", content)
        self.assertIn('WORKFLOW_OUTPUT_3: ${{ needs.prepare.outputs.release_stage }}', content)
        self.assertIn('--promotion-channel "${WORKFLOW_OUTPUT_3}"', content)
        self.assertIn(
            "needs.prepare.outputs.release_stage != 'alpha' && needs.prepare.outputs.release_stage != 'beta'",
            content,
        )
        self.assertIn("build_rollback_section", renderer)
        self.assertIn("uses: ./.github/workflows/publish-docker.yml", content)
        self.assertIn("release-convergence.yml/dispatches", content)
        self.assertIn("Release Activation Commit Verdict", content)
        self.assertNotIn('gh workflow run publish-docker.yml --ref "${REQUIRED_BRANCH}"', content)
        self.assertNotIn('gh workflow run update-demo-server.yml --ref "${REQUIRED_BRANCH}"', content)
        self.assertIn("sanitize_release_notes", renderer)
        self.assertIn("validate_release_notes_shape", renderer)
        self.assertIn("validate_release_body_shape", renderer)
        self.assertIn("GitHub's stored release body does not exactly match", renderer)
        self.assertIn("Do not treat this as published", renderer)
        self.assertIn("_DRAFT.md", renderer)
        self.assertIn("rollback target and exact reinstall command recorded", policy)
        self.assertIn("rc-to-ga-rehearsal-summary", policy)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", policy)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(policy))
        self.assertIn("recorded rollback target plus exact", source_of_truth)
        self.assertIn("hours of prerelease soak", resolver)
        self.assertIn("minimum is 72 hours unless hotfix_exception is true", resolver)
        self.assertIn("MIN_PRERELEASE_OBSERVATION_HOURS = 24", resolver)
        self.assertIn("cohort checkpoint, not a delivery vehicle", normalize_ws(policy))
        self.assertIn("at least 24 hours of public observation", normalize_ws(source_of_truth))
        self.assertIn(
            "Public checkpoints at the same maturity stage on one version line are separated by at least 24 hours",
            normalize_ws(contract),
        )
        self.assertIn("build_rollback_section", renderer)
        self.assertIn("promotion metadata out of customer notes", renderer)
        self.assertIn("historical_asset_backfill_only:", content)
        self.assertIn("Repair an already-published release packet in place without rebuilding binaries", content)
        self.assertIn("draft: true", content)
        self.assertIn("activate_release:", content)
        self.assertIn("Publish the fully staged release", content)
        self.assertIn('gh api "repos/${{ github.repository }}/releases?per_page=100" --paginate', content)
        self.assertIn('git push origin "refs/tags/${TAG}" --force', content)

        self.assertIn('Retargeting existing draft tag ${TAG}', content)
        self.assertIn('Resuming quarantined draft for ${TAG}', content)
        self.assertIn('Resuming quarantined draft release for ${TAG}', content)
        self.assertIn('release_activation_committed=${RELEASE_ACTIVATION_COMMITTED}', content)
        self.assertIn('[ "$EXISTING_RELEASE_ACTIVATION_COMMITTED" != "true" ]', content)
        self.assertIn('[ "$ACTIVATION_COMMITTED" != "true" ]', content)
        self.assertNotIn('[ -z "$EXISTING_RELEASE_PUBLISHED_AT" ]', content)
        self.assertNotIn('[ -z "$PUBLISHED_AT" ]', content)
        self.assertIn('--rawfile body "$NOTES_FILE"', content)
        self.assertIn('--input "$RELEASE_PAYLOAD"', content)
        self.assertIn('--expected-body-file "$NOTES_FILE"', content)
        self.assertIn('historical_asset_backfill_only=${HISTORICAL_ASSET_BACKFILL_ONLY}', content)
        self.assertIn(
            "if: ${{ always() && needs.prepare.result == 'success' && needs.build_release_candidate.result == 'success' && needs.create_release.result == 'success' && needs.prepare.outputs.historical_asset_backfill_only != 'true' }}",
            content,
        )
        self.assertIn("candidate_manifest_artifact:", validation_workflow)
        self.assertIn("release_candidate_manifest.py verify-release", validation_workflow)
        self.assertIn('--release-body-file "$RUNNER_TEMP/release-body.md"', validation_workflow)
        self.assertIn("VALIDATION_EXIT_CODE=${PIPESTATUS[0]}", validation_workflow)
        self.assertIn("if: ${{ needs.prepare.outputs.historical_asset_backfill_only == 'true' }}", content)
        self.assertIn("issues: write", content)
        self.assertIn("statuses: write", content)
        self.assertIn("statuses: write", validation_workflow)
        self.assertIn("curl --fail-with-body --silent --show-error -X POST", validation_workflow)
        self.assertIn('"context": "Release Asset Validation"', validation_workflow)
        self.assertIn('WORKFLOW_OUTPUT_4: ${{ steps.context.outputs.tag }}', validation_workflow)
        self.assertIn(
            'WORKFLOW_OUTPUT_5: ${{ steps.context.outputs.target_commitish }}',
            validation_workflow,
        )
        self.assertIn('--arg tag "${WORKFLOW_OUTPUT_4}"', validation_workflow)
        self.assertIn('--arg target_commitish "${WORKFLOW_OUTPUT_5}"', validation_workflow)
        self.assertIn("{body: $body, tag_name: $tag, target_commitish: $target_commitish}", validation_workflow)
        self.assertIn("{draft: true, tag_name: $tag, target_commitish: $target_commitish}", validation_workflow)
        self.assertIn("Validation release body update detached release tag", validation_workflow)
        self.assertIn("Validation release body update changed target_commitish", validation_workflow)
        self.assertIn("--validate-body-file \"$RELEASE_BODY_FILE\"", validation_workflow)
        self.assertIn("--expected-body-file \"$CLEAN_BODY_FILE\"", validation_workflow)
        self.assertIn("Quarantine malformed release body", validation_workflow)
        self.assertIn(
            "Draft releases are quarantined; published releases remain immutable for explicit remediation.",
            validation_workflow,
        )
        for step in (
            "Quarantine malformed release body",
            "Update release body - Success",
            "Delete all release assets on failure",
            "Update release body - Failure",
        ):
            self.assertIn(
                f"- name: {step}\n"
                "        if: steps.context.outputs.should_run == 'true' && "
                "steps.context.outputs.draft == 'true'",
                validation_workflow,
            )
        self.assertNotIn(
            "Release was published; reverting to draft before deleting assets",
            validation_workflow,
        )
        self.assertNotIn(
            "A published release edit introduced invalid assets",
            validation_workflow,
        )
        self.assertIn(
            'ACTUAL_RELEASE_TAG=$(jq -r \'.tag_name // empty\' "$RELEASE_JSON_FILE")',
            content,
        )
        self.assertIn(
            'ACTUAL_TARGET_COMMITISH=$(jq -r \'.target_commitish // empty\' "$RELEASE_JSON_FILE")',
            content,
        )
        self.assertIn('WORKFLOW_OUTPUT_1: ${{ needs.prepare.outputs.tag }}', content)
        self.assertIn('./scripts/backfill-release-assets.sh --tag "${WORKFLOW_OUTPUT_1}" --repo "${{ github.repository }}"', content)
        self.assertIn('./scripts/validate-published-release.sh "${WORKFLOW_OUTPUT_1}" "${{ github.repository }}"', content)
        self.assertIn("PULSE_UPDATE_SIGNING_KEY: ${{ secrets.PULSE_UPDATE_SIGNING_KEY }}", content)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}", content)
        self.assertIn(
            normalize_ws(
                """
                - name: Validate published release packet
                  env:
                    PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}
                    WORKFLOW_OUTPUT_1: ${{ needs.prepare.outputs.tag }}
                  run: |
                    ./scripts/validate-published-release.sh "${WORKFLOW_OUTPUT_1}" "${{ github.repository }}"
                """
            ),
            normalize_ws(content),
        )
        self.assertNotIn("pulse_update_signing_key=${{ secrets.PULSE_UPDATE_SIGNING_KEY }}", docker_build)
        self.assertIn("Validate installer signing key pins", candidate_workflow)
        self.assertIn("timeout-minutes: 60", candidate_workflow)
        self.assertNotRegex(candidate_workflow, r"(?m)^\s+runs-on:.*self-hosted")
        self.assertIn("Compile Exact-SHA Release Payload on Ephemeral VM", compiler_workflow)
        self.assertIn("runs-on: ubuntu-24.04", compiler_workflow)
        self.assertNotRegex(compiler_workflow, r"(?m)^\s+runs-on:.*self-hosted")
        self.assertNotIn("pulse-pve-", compiler_workflow)
        self.assertIn("GITHUB_WORKFLOW_SHA", compiler_workflow)
        self.assertIn("ref: ${{ inputs.source_sha }}", compiler_workflow)
        self.assertIn("PULSE_RELEASE_BUILD_JOBS: \"2\"", compiler_workflow)
        self.assertIn("actions/workflows/compile-release-payload.yml/dispatches", candidate_workflow)
        self.assertIn("X-GitHub-Api-Version: 2026-03-10", candidate_workflow)
        self.assertIn("compiler_run_id: ${{ steps.dispatch.outputs.compiler_run_id }}", candidate_workflow)
        self.assertIn('.path == ".github/workflows/compile-release-payload.yml"', candidate_workflow)
        self.assertIn(
            "EXPECTED_ARTIFACT_ID: ${{ needs.obtain-release-payload.outputs.artifact_id }}",
            candidate_workflow,
        )
        self.assertIn(
            "EXPECTED_ARTIFACT_DIGEST: ${{ needs.obtain-release-payload.outputs.artifact_digest }}",
            candidate_workflow,
        )
        self.assertIn(
            "EXPECTED_COMPILER_RUN_ID: ${{ needs.obtain-release-payload.outputs.compiler_run_id }}",
            candidate_workflow,
        )
        self.assertIn(".workflow_run.head_sha == $source_sha", candidate_workflow)
        self.assertIn("sha256sum --check --", candidate_workflow)
        self.assertIn("compiled-payload-verification.json", candidate_workflow)
        self.assertIn("separate-ephemeral-github-hosted-compiler-workflow", candidate_workflow)
        self.assertIn("Verify Native Signing Configuration", candidate_workflow)
        self.assertEqual(candidate_workflow.count("needs: signing-configuration"), 2)
        self.assertIn("require_windows_signing: ${{ needs.prepare.outputs.require_windows_signing == 'true' }}", content)
        self.assertIn("unsigned_windows_exception:", content)
        self.assertIn("unsigned_windows_reason:", content)
        self.assertIn("WINDOWS_AUTHENTICODE_AVAILABLE = False", resolver)
        self.assertIn("WINDOWS_AUTHENTICODE_STANDING_UNSIGNED_MIN_VERSION = (6, 3, 2)", resolver)
        self.assertIn('version not in {"6.1.0", "6.1.1", "6.1.2", "6.2.0", "6.2.1", "6.3.0", "6.3.1", "6.3.2"}', resolver)
        self.assertIn("not Authenticode-signed", resolver)
        self.assertIn("windows_signing_backend: signpath", content)
        self.assertIn('if [[ "$REQUIRE_WINDOWS_SIGNING" == "true" ]]', candidate_workflow)
        self.assertIn("inputs.require_windows_signing", candidate_workflow)
        self.assertIn("signpath/github-action-submit-signing-request@b9d91eadd323de506c0c81cf0c7fe7438f3360fd # v2", candidate_workflow)
        self.assertIn("github-artifact-id: ${{ steps.upload-unsigned-windows.outputs.artifact-id }}", candidate_workflow)
        self.assertIn("windows-signing-evidence.json", candidate_workflow)
        for signpath_setting in (
            "SIGNPATH_API_TOKEN",
            "SIGNPATH_ORGANIZATION_ID",
            "SIGNPATH_PROJECT_SLUG",
            "SIGNPATH_SIGNING_POLICY_SLUG",
            "SIGNPATH_ARTIFACT_CONFIGURATION_SLUG",
            "SIGNPATH_EXPECTED_CERTIFICATE_SUBJECT",
        ):
            self.assertIn(signpath_setting, candidate_workflow)
        for signing_secret in (
            "APPLE_DEVELOPER_ID_CERTIFICATE_P12_BASE64",
            "APPLE_DEVELOPER_ID_CERTIFICATE_PASSWORD",
            "APPLE_DEVELOPER_ID_APPLICATION_IDENTITY",
            "APPLE_NOTARY_KEY_P8_BASE64",
            "APPLE_NOTARY_KEY_ID",
            "APPLE_NOTARY_ISSUER_ID",
            "WINDOWS_CODE_SIGNING_CERTIFICATE_PFX_BASE64",
            "WINDOWS_CODE_SIGNING_CERTIFICATE_PASSWORD",
        ):
            self.assertIn(signing_secret, candidate_workflow)
        self.assertIn('tar -xzf "$tarball" -C "$extract_dir" -- "$@"', release_validator)
        self.assertNotIn('tar -xOf "$tarball" "$entry"', release_validator)
        self.assertIn("go run ./scripts/release_update_key.go public-key-ssh", candidate_workflow)
        self.assertIn("does not trust the configured release signing key", candidate_workflow)
        self.assertIn("scripts/install-mcp.sh release/install-mcp.sh", candidate_workflow)
        self.assertIn("scripts/install-mcp.ps1 release/install-mcp.ps1", candidate_workflow)
        self.assertIn("$PinnedReleaseSshPublicKey = '${TRUSTED_SSH_PUBLIC_KEY}'", candidate_workflow)
        self.assertIn("TRUSTED_SSH_PUBLIC_KEY", update_demo_workflow)
        self.assertIn('sed -i "s|^PINNED_RELEASE_SSH_PUBLIC_KEY=.*|PINNED_RELEASE_SSH_PUBLIC_KEY=\\"${TRUSTED_SSH_PUBLIC_KEY}\\"|" /tmp/pulse-install.sh', update_demo_workflow)
        self.assertIn("bash .github/scripts/setup-demo-ssh.sh", update_demo_workflow)
        self.assertIn("bash .github/scripts/check-demo-reachability.sh", update_demo_workflow)
        self.assertIn("ping: ${{ secrets.DEMO_SERVER_HOST }}", update_demo_workflow)
        self.assertIn("tailscale/github-action@306e68a486fd2350f2bfc3b19fcd143891a4a2d8 # v4", update_demo_workflow)
        self.assertIn("uses: ./.github/workflows/update-demo-server.yml", deploy_demo_workflow)
        self.assertIn("verify_only: true", deploy_demo_workflow)
        self.assertIn('MAX_SSH_SETUP_ATTEMPTS="${DEMO_SSH_SETUP_ATTEMPTS:-3}"', demo_ssh_helper)
        self.assertIn("ipaddress.ip_address(sys.argv[1])", demo_ssh_helper)
        self.assertIn("host_needs_dns=false", demo_ssh_helper)
        self.assertIn('getent hosts "$DEMO_SERVER_HOST"', demo_ssh_helper)
        self.assertIn('ssh-keyscan -T 10 -H "$DEMO_SERVER_HOST"', demo_ssh_helper)
        self.assertIn("Demo network preflight passed, but ssh-keyscan did not return host keys", demo_ssh_helper)
        self.assertIn('tailscale ping --c 3 --timeout 10s "$DEMO_SERVER_HOST"', demo_reachability_helper)
        self.assertIn('nc -z -w 5 "$DEMO_SERVER_HOST" "$TCP_PORT"', demo_reachability_helper)
        self.assertIn("Demo peer is not present in the runner peer map yet.", demo_reachability_helper)
        self.assertIn("derive the OpenSSH installer trust key from `PULSE_UPDATE_SIGNING_PUBLIC_KEY`", normalize_ws(contract))
        self.assertIn('SYFT_VERSION="1.42.4"', content)
        self.assertIn('SYFT_ARCHIVE="syft_${SYFT_VERSION}_linux_amd64.tar.gz"', content)
        self.assertIn('SYFT_SHA256="590650c2743b83f327d1bf9bec64f6f83b7fec504187bb84f500c862bf8f2a0f"', content)
        self.assertIn('install -m 0755 "${TMP_DIR}/syft" /usr/local/bin/syft', content)
        self.assertIn('release_upload_with_retry "${TAG}" release/*.sig --clobber', content)
        self.assertIn('release_upload_with_retry "${TAG}" release/*.sshsig --clobber', content)
        self.assertIn('release_upload_with_retry "${TAG}" release/*.sbom.spdx.json --clobber', content)
        self.assertIn('gh release upload "$@"', content)
        self.assertIn('gh release upload failed on attempt ${attempt}/${max_attempts}; retrying in ${wait_seconds}s', content)
        self.assertIn('gh release upload failed after ${max_attempts} attempts', content)
        self.assertIn("Running current organization-sharing E2E suite...", content)
        self.assertIn(
            "tests/66-organization-sharing-approval-ui.spec.ts",
            content,
        )
        self.assertNotIn("npx playwright test tests/03-multi-tenant.spec.ts", content)
        self.assertIn('PULSE_E2E_ENTITLEMENT_PROFILE: "multi-tenant"', content)
        self.assertIn("Collect integration diagnostics", content)
        self.assertIn("release-integration-diagnostics/docker.log", content)
        self.assertIn("docker ps -a || true", content)
        self.assertIn("docker logs pulse-test-server", content)
        self.assertIn("docker logs pulse-mock-github", content)
        self.assertIn("Upload integration Playwright report", content)
        self.assertIn("Upload integration failures", content)
        self.assertIn("tests/integration/test-results/", content)
        self.assertIn("tests/integration/release-integration-diagnostics/", content)
        self.assertIn("--target runtime_prebuilt", docker_build)
        self.assertIn("--target agent_runtime_prebuilt", docker_build)
        self.assertIn("id-token: write", candidate_workflow)
        self.assertIn("attestations: write", candidate_workflow)
        self.assertIn(
            "uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4",
            candidate_workflow,
        )
        self.assertIn(
            "subject-checksums: ${{ runner.temp }}/release-candidate-subjects.sha256",
            candidate_workflow,
        )
        self.assertIn("Preserve portable build provenance", candidate_workflow)
        self.assertIn("release-build-provenance.sigstore.json", candidate_workflow)
        self.assertLess(
            candidate_workflow.index("Validate complete candidate locally"),
            candidate_workflow.index("Attest complete release candidate"),
        )
        self.assertLess(
            candidate_workflow.index("Attest complete release candidate"),
            candidate_workflow.index("Seal immutable candidate manifest"),
        )
        self.assertIn("release-build-provenance.sigstore.json", content)
        build_script = read("scripts/build-release.sh")
        release_asset_helper = read("scripts/release_asset_common.sh")
        backfill_script = read("scripts/backfill-release-assets.sh")
        backfill_workflow = read(".github/workflows/backfill-release-assets.yml")
        self.assertIn('RELEASE_PACKET_SBOM="pulse-v${VERSION}-release.sbom.spdx.json"', build_script)
        self.assertIn('SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"', build_script)
        self.assertIn('PULSE_REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"', build_script)
        self.assertIn('cd "${PULSE_REPO_ROOT}"', build_script)
        self.assertIn('source "${SCRIPT_DIR}/release_asset_common.sh"', build_script)
        self.assertIn('pulse_release_prepare_signing_state "pulse-installer" "pulse-install"', build_script)
        self.assertIn('pulse_release_generate_packet_sbom "${RELEASE_DIR}" "${RELEASE_PACKET_SBOM}"', build_script)
        self.assertIn('pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"', build_script)
        self.assertIn(': "${PULSE_SCRIPTS_DIR:=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"', release_asset_helper)
        self.assertIn(': "${PULSE_REPO_ROOT:=$(cd "${PULSE_SCRIPTS_DIR}/.." && pwd)}"', release_asset_helper)
        self.assertIn('go -C "${PULSE_REPO_ROOT}" run ./scripts/release_update_key.go "$@"', release_asset_helper)
        self.assertIn('pulse_release_go_run_update_key public-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"', release_asset_helper)
        self.assertIn(
            'pulse_release_go_run_update_key fingerprint --public-key "${PULSE_RELEASE_UPDATE_PUBLIC_KEY}"',
            release_asset_helper,
        )
        self.assertIn('pulse_release_go_run_update_key public-key-ssh --private-key "${PULSE_UPDATE_SIGNING_KEY}"', release_asset_helper)
        self.assertIn('pulse_release_go_run_update_key openssh-private-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"', release_asset_helper)
        self.assertIn('pulse_release_go_run_update_key sign --private-key "${PULSE_UPDATE_SIGNING_KEY}" --file "${absolute_file}"', release_asset_helper)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY", release_asset_helper)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY_FINGERPRINT", release_asset_helper)
        self.assertIn(
            "Verified update signing public key fingerprint: ${PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT}",
            release_asset_helper,
        )
        self.assertIn('"${resolved_tool}" "dir:${release_dir}" -o "spdx-json=${tmp_sbom}"', release_asset_helper)
        self.assertIn('find . -maxdepth 1 -type f \\( -name \'*.sig\' -o -name \'*.sshsig\' \\) -delete', release_asset_helper)
        self.assertIn('source "${SCRIPT_DIR}/release_asset_common.sh"', backfill_script)
        self.assertIn('gh release download "${TAG}" -R "${REPO}" --dir "${RELEASE_DIR}" --clobber', backfill_script)
        self.assertIn('pulse_release_generate_packet_sbom "${PAYLOAD_DIR}" "${RELEASE_PACKET_SBOM}"', backfill_script)
        self.assertIn('pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"', backfill_script)
        self.assertIn('gh release upload "${TAG}" "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}" --clobber', backfill_script)
        self.assertIn("name: Backfill Release Assets", backfill_workflow)
        self.assertIn("workflow_dispatch:", backfill_workflow)
        self.assertIn('SYFT_VERSION="1.42.4"', backfill_workflow)
        self.assertIn("TAG: ${{ inputs.tag }}", backfill_workflow)
        self.assertIn("REPOSITORY: ${{ github.repository }}", backfill_workflow)
        self.assertIn('./scripts/backfill-release-assets.sh --tag "${TAG}" --repo "${REPOSITORY}"', backfill_workflow)
        self.assertIn('./scripts/validate-published-release.sh "${TAG}" "${REPOSITORY}"', backfill_workflow)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}", backfill_workflow)
        self.assertIn(
            normalize_ws(
                """
                - name: Validate published release packet
                  env:
                    PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}
                    REPOSITORY: ${{ github.repository }}
                    TAG: ${{ inputs.tag }}
                  run: |
                    ./scripts/validate-published-release.sh "${TAG}" "${REPOSITORY}"
                """
            ),
            normalize_ws(backfill_workflow),
        )
        self.assertIn("./scripts/prepare-release-container-context.sh", candidate_workflow)
        self.assertIn('test "${actual_server}" = "${expected_server}"', docker_build)
        self.assertIn('test "${actual_agent}" = "${expected_agent}"', docker_build)
        self.assertNotIn("secrets.", docker_build)
        self.assertNotIn("provenance: false", content)
        self.assertIn("Derived rollback command:", helper)
        self.assertIn("sudo /bin/update --version", helper)
        self.assertIn("v6 GA date to publish with GA", helper)
        self.assertIn("--arg ga_date \"$GA_DATE\"", helper)
        self.assertIn("ga_date", resolver)
        self.assertIn("v5_eos_date", resolver)
        self.assertIn("Stable v6.0.0 requires v5_eos_date in YYYY-MM-DD form", resolver)
        self.assertIn("release_notes must include the Pulse v5 maintenance-only support notice", resolver)
        dry_run_workflow = read(".github/workflows/release-dry-run.yml")
        self.assertIn("Required rollback stable version to rehearse", dry_run_workflow)
        self.assertIn("rollback_version:\n        description: 'Required rollback stable version to rehearse", dry_run_workflow)
        self.assertIn("required: true", dry_run_workflow)
        # Scheduled watchdog runs carry no dispatch inputs, so the rehearsal
        # step must derive the rollback target; the derive flag stays gated on
        # the schedule event so manual dispatches keep explicit rollback.
        self.assertIn(
            'if [ "${EVENT_NAME}" = "schedule" ] && [ -z "${ROLLBACK_VERSION_INPUT:-}" ]; then',
            dry_run_workflow,
        )
        self.assertIn("--derive-rollback-latest-stable", dry_run_workflow)
        self.assertIn("--derive-rollback-latest-stable", resolver)
        self.assertIn("derive_latest_stable_rollback_tag", resolver)
        self.assertIn("Required: prior stable version to pin for rollback", content)
        self.assertIn("rollback_version:\n        description: 'Required: prior stable version to pin for rollback", content)
        self.assertIn("check-workflow-dispatch-inputs.py", helper)
        self.assertIn('--branch "$CURRENT_BRANCH"', helper)
        self.assertIn('--ref "$CURRENT_BRANCH"', helper)
        self.assertIn("Release automation executes the selected remote ref", helper)
        self.assertNotIn("Continue anyway?", helper)
        self.assertIn("Audit header composition", content)
        self.assertIn("run: npm --prefix frontend-modern run lint:headers", content)
        self.assertIn("pushed governed release-branch copy of `.github/workflows/release-dry-run.yml`", policy)
        self.assertIn("GitHub executes the selected remote ref", normalize_ws(policy))
        checklist = read("docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("pushed governed release-branch copy of `.github/workflows/release-dry-run.yml`", checklist)
        self.assertIn("workflow_dispatch", checklist)
        self.assertIn("selected remote ref", normalize_ws(checklist))
        self.assertIn("local rehearsal branch exactly matches `origin`", checklist)
        self.assertIn("derive the governed release branch from release-control metadata", checklist)
        template = read("docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md")
        self.assertIn("governed release line from `control_plane.json`", template)
        self.assertIn("pulse/v6-release", template)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", runbook)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", runbook)
        self.assertIn("Existing unpublished draft releases for the same tag are updated in place", runbook)
        self.assertIn("Do not rewrite shipped RC notes in place", runbook)
        self.assertIn("`rc.1`, `rc.2`, and later prerelease", runbook)
        self.assertIn("The current RC release packet is prepared and internally linked", runbook)
        self.assertIn("operators know the update signer changed", normalize_ws(runbook))
        self.assertIn("manual reinstall or other explicit trust-migration path", normalize_ws(runbook))
        self.assertIn("points at the current in-repo draft packet", runbook)
        # Runbook example version tracks the current VERSION file (any active 6.0.0-rc.N).
        current_version = read("VERSION").strip()
        if rc_packet_paths_for_version(current_version) is not None:
            self.assertIn(f'export RC_VERSION="{current_version}"', runbook)
        self.assertIn("printf '%s\\n' \"$RC_VERSION\" > VERSION", runbook)
        self.assertIn("canonical file-backed helper", runbook)
        self.assertIn(
            './scripts/trigger-release.sh "$RC_VERSION" "$RELEASE_NOTES_FILE"',
            runbook,
        )
        self.assertIn("Do not paste multiline release notes", runbook)
        self.assertIn("compares GitHub's stored body byte-for-byte", runbook)
        self.assertIn("Keep the current release-notes, changelog, and operator-support packet in", runbook)
        self.assertIn("Published release bodies must also stay publication-safe", contract)
        self.assertIn("Release-note transport is file-backed and fail-closed", contract)
        self.assertIn("malformed edited body is quarantined", contract)
        self.assertIn("must state the continuity impact explicitly", normalize_ws(contract))
        self.assertIn(
            "append the standardized installation and promotion metadata sections exactly once",
            normalize_ws(contract),
        )

    def test_v620_owner_soak_waiver_is_version_bound(self) -> None:
        policy = normalize_ws(
            read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        )
        owner_record = normalize_ws(
            read(
                "docs/release-control/v6/internal/records/"
                "v6.2.0-stable-cutoff-owner-approval-2026-08-09.md"
            )
        )

        self.assertIn("v6.2.0 release-cutoff exception", policy)
        self.assertIn("not soak evidence and not a standing exception", policy)
        self.assertIn("`v6.2.0`-only unsigned-Windows exception", policy)
        self.assertIn("not a standing decision for later releases", policy)
        self.assertIn("Promoted prerelease: `v6.2.0-rc.11`", owner_record)
        self.assertIn("Rollback target: `v6.1.2`", owner_record)
        self.assertIn(
            "Exact rollback reinstall command: `./scripts/install.sh --version v6.1.2`",
            owner_record,
        )
        self.assertIn("explicitly approved a `v6.2.0`-only unsigned-Windows exception", owner_record)
        self.assertIn("not Authenticode-signed", owner_record)
        self.assertIn("Unknown Publisher warning", owner_record)

    def test_v621_unsigned_windows_owner_exception_is_version_bound(self) -> None:
        policy = normalize_ws(
            read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        )
        owner_record = normalize_ws(
            read(
                "docs/release-control/v6/internal/records/"
                "v6.2.1-unsigned-windows-owner-approval-2026-08-10.md"
            )
        )

        self.assertIn("v6.2.1 unsigned-Windows exception", policy)
        self.assertIn("release run `31343128024` failed closed", policy)
        self.assertIn("explicitly authorized unsigned Windows artifacts", policy)
        self.assertIn("Stable `v6.2.2` and later restore mandatory Authenticode", policy)
        self.assertIn("Release: `v6.2.1`", owner_record)
        self.assertIn("Failed release run: `31343128024`", owner_record)
        self.assertIn("explicitly authorized unsigned Windows Unified Agent artifacts", owner_record)
        self.assertIn("not Authenticode-signed", owner_record)
        self.assertIn("Unknown Publisher warning", owner_record)

    def test_v630_owner_telemetry_cutoff_is_version_bound(self) -> None:
        policy = normalize_ws(
            read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        )
        owner_record = normalize_ws(
            read(
                "docs/release-control/v6/internal/records/"
                "v6.3.0-stable-cutoff-owner-approval-2026-08-22.md"
            )
        )

        self.assertIn("v6.3.0 release-cutoff exception", policy)
        self.assertIn("bounded v6.3.0 owner-risk acceptance", policy)
        self.assertIn("not soak evidence and not a standing exception", policy)
        self.assertIn("Promoted prerelease: `v6.3.0-rc.6`", owner_record)
        self.assertIn("Rollback target: `v6.2.1`", owner_record)
        self.assertIn(
            "Exact rollback reinstall command: `./scripts/install.sh --version v6.2.1`",
            owner_record,
        )
        self.assertIn("18 active installs: 10 binary and 8 Docker", owner_record)
        self.assertIn("56 rolling-window update attempts, 56 successes, and zero failures", owner_record)
        self.assertIn("separate, version-bound `v6.3.0` unsigned-Windows decision", owner_record)

        signing_record = normalize_ws(
            read(
                "docs/release-control/v6/internal/records/"
                "v6.3.0-unsigned-windows-owner-approval-2026-08-22.md"
            )
        )
        self.assertIn("v6.3.0 unsigned-Windows exception", policy)
        self.assertIn("Stable `v6.3.1` and later restore mandatory Authenticode", policy)
        self.assertIn("Windows Authenticode signing is not yet available", signing_record)
        self.assertIn("not Authenticode-signed", signing_record)
        self.assertIn("Unknown Publisher warning", signing_record)

        patch_signing_record = normalize_ws(
            read(
                "docs/release-control/v6/internal/records/"
                "v6.3.1-unsigned-windows-owner-approval-2026-08-23.md"
            )
        )
        self.assertIn("v6.3.1 unsigned-Windows exception", policy)
        self.assertIn("Stable `v6.3.2` and later restore mandatory Authenticode", policy)
        self.assertIn("SignPath production certificate remains `CSR PENDING`", patch_signing_record)
        self.assertIn("not Authenticode-signed", patch_signing_record)
        self.assertIn("Unknown Publisher warning", patch_signing_record)

    def test_v640_expedited_stable_cutoff_is_version_bound(self) -> None:
        policy = normalize_ws(
            read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        )
        owner_record = normalize_ws(
            read(
                "docs/release-control/v6/internal/records/"
                "v6.4.0-stable-cutoff-owner-approval-2026-08-28.md"
            )
        )

        self.assertIn("v6.4.0 expedited stable-cutoff exception", policy)
        self.assertIn("not soak evidence and not a standing exception", policy)
        self.assertIn("Runtime content cutoff: `18b22d1ebbfe542484652e419320fc7643a792f0`", owner_record)
        self.assertIn("Promoted published prerelease: `v6.4.0-rc.12`", owner_record)
        self.assertIn("Rollback target: `v6.3.2`", owner_record)
        self.assertIn(
            "Exact rollback reinstall command: `./scripts/install.sh --version v6.3.2`",
            owner_record,
        )
        self.assertIn("active customer harm", owner_record)
        self.assertIn("not 72-hour soak evidence", owner_record)
        self.assertIn("standing Windows Authenticode-unavailable policy", owner_record)
        self.assertIn("not Authenticode-signed", owner_record)
        self.assertIn("Unknown Publisher warning", owner_record)

        unavailable_signing_record = normalize_ws(
            read(
                "docs/release-control/v6/internal/records/"
                "windows-authenticode-unavailable-owner-policy-2026-08-25.md"
            )
        )
        self.assertIn("Windows Authenticode unavailable policy from v6.3.2", policy)
        self.assertIn("must not require per-version unsigned allowlist updates", policy)
        self.assertIn("Invalid request to SignPath API", unavailable_signing_record)
        self.assertIn("not Authenticode-signed", unavailable_signing_record)
        self.assertIn("Unknown Publisher warning", unavailable_signing_record)

    def test_release_artifact_workflows_refuse_stable_without_matching_rc(self) -> None:
        publish = read(".github/workflows/publish-docker.yml")
        promote = read(".github/workflows/promote-floating-tags.yml")
        demo = read(".github/workflows/update-demo-server.yml")
        demo_profile = read(".github/scripts/resolve-demo-runtime-profile.sh")
        preview_deploy = read(".github/workflows/deploy-demo-server.yml")
        release_workflow = read(".github/workflows/create-release.yml")
        candidate_workflow = read(".github/workflows/build-release-candidate.yml")
        qualifier_workflow = read(".github/workflows/qualify-release-containers.yml")
        dry_run_workflow = read(".github/workflows/release-dry-run.yml")
        helm = read(".github/workflows/publish-helm-chart.yml")
        helm_pages = read(".github/workflows/helm-pages.yml")
        convergence = read(".github/workflows/release-convergence.yml")
        artifact_validator = read("scripts/release_control/validate_artifact_release_line.py")
        chart = read("deploy/helm/pulse/Chart.yaml")
        chart_sync = read("scripts/sync_chart_release_metadata.py")
        demo_smoke = read("scripts/demo_public_browser_smoke.cjs")
        runbook = read("docs/releases/V6_PRERELEASE_RUNBOOK.md")
        self.assertIn("validate_artifact_release_line.py", publish)
        self.assertIn('--anticipated-source-sha "${SOURCE_SHA}"', publish)
        self.assertIn('test "$(git rev-parse HEAD)" = "${EXPECTED_SOURCE_SHA}"', publish)
        self.assertIn("if tag_exists_fn(normalized_tag):", artifact_validator)
        self.assertIn("not anticipated source", artifact_validator)
        self.assertIn("provenance: mode=max", publish)
        self.assertIn("sbom: true", publish)
        self.assertIn("name: Publish ${{ matrix.image }} image", publish)
        self.assertIn("fail-fast: false", publish)
        self.assertIn("if: matrix.image == 'server'", publish)
        self.assertIn("if: matrix.image == 'control-plane'", publish)
        self.assertIn("uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4", publish)
        self.assertIn("subject-name: docker.io/rcourtman/pulse", publish)
        self.assertIn("subject-name: ghcr.io/${{ github.repository_owner }}/pulse", publish)
        # pulse-agent ships as release-asset binaries, not as a Docker image
        # (see commit dropping the agent image publish steps). The agent
        # attestation subject-names intentionally do not appear here.
        self.assertNotIn("subject-name: docker.io/rcourtman/pulse-agent", publish)
        self.assertNotIn("subject-name: ghcr.io/${{ github.repository_owner }}/pulse-agent", publish)
        self.assertIn("create-storage-record: false", publish)
        self.assertIn("server_digest:", publish)
        self.assertIn("control_plane_digest:", publish)
        self.assertIn("Verify exact image identities and provenance", publish)
        self.assertIn("verify-release-container-images.sh", publish)
        self.assertIn("target: runtime_prebuilt", publish)
        self.assertIn("target: control_plane_prebuilt", publish)
        self.assertIn("Verify exact-candidate container payload", publish)
        self.assertNotIn("PULSE_LICENSE_PUBLIC_KEY", publish)
        self.assertNotIn("PULSE_UPDATE_SIGNING_KEY", publish)
        self.assertNotIn("provenance: false", publish)
        self.assertIn("validate_artifact_release_line.py", promote)
        self.assertIn("source_sha:", promote)
        self.assertIn("server_digest:", promote)
        self.assertIn("control_plane_digest:", promote)
        self.assertIn("verify-release-container-images.sh", promote)
        self.assertIn('docker_source="docker.io/rcourtman/${image}@${expected_digest}"', promote)
        self.assertIn('ghcr_source="ghcr.io/${OWNER}/${image}@${expected_digest}"', promote)
        self.assertNotIn('"rcourtman/${image}:${TAG}"', promote)
        self.assertNotIn('"ghcr.io/${OWNER}/${image}:${TAG}"', promote)
        self.assertIn("control_plane.py --branch-for-version", demo)
        self.assertIn("demo-stable", demo)
        self.assertIn("Refusing prerelease tag", demo)
        self.assertIn("Prerelease demo updates are retired after v6 GA", demo)
        self.assertIn("The latest alias is allowed only for verification-only checks.", demo)
        self.assertIn("Resolved verification-only target to latest stable release", demo)
        self.assertNotIn("github.event_name == 'release'", demo)
        self.assertNotIn("preview-v6", demo)
        self.assertNotIn("demo-preview-v6", demo)
        self.assertNotIn('SERVICE_NAME="pulse-v6-preview"', demo)
        self.assertNotIn("Preview demo updates must not target the stable pulse service.", demo)
        self.assertIn("workflow_call:", demo)
        self.assertIn("verify_only:", demo)
        self.assertIn("tag: latest", dry_run_workflow)
        self.assertIn("verify_only: true", dry_run_workflow)
        self.assertIn("Verify Current Stable Demo Path (No Mutation)", dry_run_workflow)
        self.assertIn("tailscale/github-action@306e68a486fd2350f2bfc3b19fcd143891a4a2d8 # v4", demo)
        self.assertIn("oauth-client-id: ${{ secrets.TS_OAUTH_CLIENT_ID }}", demo)
        self.assertIn("oauth-secret: ${{ secrets.TS_OAUTH_SECRET }}", demo)
        self.assertIn("ping: ${{ secrets.DEMO_SERVER_HOST }}", demo)
        # The static 90-day TS_AUTHKEY was retired for the OAuth client
        # (0a9a29d63); the runner mints an ephemeral tagged node key per run.
        self.assertNotIn("TS_AUTHKEY", demo)
        self.assertIn("DEMO_EXPECTED_HOSTNAME", demo)
        self.assertIn("Verify target host identity", demo)
        self.assertIn("Demo environment points at host $REMOTE_HOSTNAME but expected $DEMO_EXPECTED_HOSTNAME.", demo)
        self.assertIn("Restore demo runtime configuration", demo)
        self.assertIn("Resolve target-compatible demo runtime profile", demo)
        self.assertIn("mockEagerHistoryPVEGuestLimit", demo_profile)
        self.assertIn("UpdateMetricCohort", demo_profile)
        self.assertIn("mockLargeEstateStartupReady", demo_profile)
        self.assertIn('PROFILE="legacy-bounded"', demo_profile)
        self.assertIn("MOCK_NODES=8", demo_profile)
        self.assertIn("MOCK_VMS_PER_NODE=6", demo_profile)
        self.assertIn("MOCK_LXCS_PER_NODE=4", demo_profile)
        self.assertIn("MOCK_DOCKER_HOSTS=2", demo_profile)
        self.assertIn("MOCK_DOCKER_CONTAINERS=8", demo_profile)
        self.assertIn("MOCK_GENERIC_HOSTS=2", demo_profile)
        self.assertIn("MOCK_K8S_CLUSTERS=1", demo_profile)
        self.assertIn("MOCK_K8S_NODES=3", demo_profile)
        self.assertIn("MOCK_K8S_PODS=12", demo_profile)
        self.assertIn("MOCK_K8S_DEPLOYMENTS=4", demo_profile)
        self.assertIn("MOCK_SEED_DURATION=2h", demo_profile)
        self.assertIn("MOCK_SAMPLE_INTERVAL=5m", demo_profile)
        self.assertIn("MOCK_UPDATE_INTERVAL=15s", demo_profile)
        self.assertIn("resolve_config_dir", demo)
        self.assertIn("set_env_value DEMO_MODE true", demo)
        self.assertIn("set_env_value PULSE_MOCK_MODE true", demo)
        self.assertIn('set_env_value PULSE_MOCK_NODES "$MOCK_NODES"', demo)
        self.assertIn("set_env_value PULSE_MOCK_SEED_METRICS_STORE false", demo)
        self.assertIn('set_env_value PULSE_MOCK_TRENDS_SEED_DURATION "$MOCK_SEED_DURATION"', demo)
        self.assertIn('set_env_value PULSE_MOCK_TRENDS_SAMPLE_INTERVAL "$MOCK_SAMPLE_INTERVAL"', demo)
        self.assertIn('set_env_value PULSE_MOCK_UPDATE_INTERVAL "$MOCK_UPDATE_INTERVAL"', demo)
        self.assertIn("ensure_demo_fixture_entitlement", demo)
        self.assertIn('"demo_fixtures"', demo)
        self.assertIn("del(.integrity)", demo)
        self.assertIn("Demo fixture entitlement ensured in governed demo billing state.", demo)
        self.assertIn("Demo service restarted with governed demo runtime configuration.", demo)
        self.assertIn("/api/license/runtime-capabilities", demo)
        self.assertIn("Mock mode enabled", demo)
        self.assertIn("Demo server mock mode did not enable after entitlement sync", demo)
        self.assertIn(".resources | if type == \"array\" then length else 0 end", demo)
        self.assertNotIn(".nodes | length", demo)
        self.assertIn("Mock resources detected", demo)
        self.assertIn("canonical mock resources are missing", demo)
        self.assertIn("Verify frontend parity", demo)
        self.assertIn("Verify public browser smoke", demo)
        self.assertIn("./scripts/run_demo_public_browser_smoke.sh", demo)
        self.assertIn("extract_entry_asset()", demo)
        self.assertIn(r'<script\b[^>]*\bsrc=\"(/assets/index-[^\"]*\.js)\"', demo)
        self.assertIn("Public demo is serving $PUBLIC_ASSET but the target service is serving $REMOTE_ASSET.", demo)
        self.assertIn("uses: ./.github/workflows/publish-docker.yml", release_workflow)
        self.assertIn("release-convergence.yml/dispatches", release_workflow)
        self.assertIn("uses: ./.github/workflows/build-release-candidate.yml", release_workflow)
        self.assertIn("Build Immutable Release Candidate", release_workflow)
        self.assertIn("Release Activation Commit Verdict", release_workflow)
        self.assertNotIn("Require recent exact-SHA stable patch preflight", release_workflow)
        self.assertNotIn("gh workflow run update-demo-server.yml", release_workflow)
        self.assertNotIn("gh workflow run publish-docker.yml", release_workflow)
        self.assertIn("Verify Current Committed Stable Demo (No Mutation)", preview_deploy)
        self.assertIn("uses: ./.github/workflows/update-demo-server.yml", preview_deploy)
        self.assertIn("tag: latest", preview_deploy)
        self.assertIn("verify_only: true", preview_deploy)
        self.assertNotIn("go build", preview_deploy)
        self.assertNotIn("scp ", preview_deploy)
        self.assertIn("validate_artifact_release_line.py", helm)
        self.assertIn(".github/workflows/create-release.yml", helm_pages)
        self.assertIn("release-activation.json", helm_pages)
        self.assertIn('gh run download "${SOURCE_RELEASE_RUN_ID}"', helm_pages)
        self.assertIn("release_branch_for_version", artifact_validator)
        self.assertIn("matching prerelease tag", artifact_validator)
        self.assertIn("previous stable tag", artifact_validator)
        self.assertIn("stable_patch", artifact_validator)
        self.assertIn("Refusing {purpose}", artifact_validator)
        self.assertIn("Assemble exact-candidate runtime and agent images", qualifier_workflow)
        self.assertIn('kind load docker-image "${SMOKE_IMAGE_REPOSITORY}:${SMOKE_IMAGE_TAG}" --name pulse-test', qualifier_workflow)
        self.assertIn('--set image.repository="${SMOKE_IMAGE_REPOSITORY}"', qualifier_workflow)

        self.assertIn('--set image.pullPolicy=Never', qualifier_workflow)
        self.assertNotIn("needs.docker_build.result", release_workflow)
        self.assertNotIn("needs.helm_smoke.result", release_workflow)
        self.assertIn('qualified chart metadata does not match the activated release', helm_pages)
        self.assertIn("workflow_call:", helm_pages)
        self.assertNotIn("workflow_run:", helm_pages)
        self.assertIn('gh run download "${SOURCE_RELEASE_RUN_ID}"', helm_pages)
        self.assertIn('--name "pulse-chart-${VERSION}"', helm_pages)
        self.assertIn("release-activation.json", helm_pages)
        self.assertIn(".github/workflows/create-release.yml", helm_pages)
        self.assertNotIn('git pull --rebase origin "$REQUIRED_BRANCH"', helm_pages)
        self.assertNotIn('git push origin HEAD:"$REQUIRED_BRANCH"', helm_pages)
        self.assertNotIn("HELM_DOCS_VERSION", helm_pages)
        self.assertNotIn("helm package deploy/helm/pulse", helm_pages)
        self.assertNotIn("git pull --rebase origin main", helm_pages)
        self.assertNotIn("git push origin main", helm_pages)
        self.assertNotIn("kind load docker-image", helm_pages)
        self.assertIn("Publish chart release and merge Pages index", helm_pages)
        self.assertIn('gh release create "${chart_release}" "${chart_path}"', helm_pages)
        self.assertIn('helm repo index "${index_work}"', helm_pages)
        self.assertIn('git -C gh-pages push origin HEAD:gh-pages', helm_pages)
        self.assertIn('grep -q "version: ${VERSION}"', helm_pages)
        self.assertIn('helm show chart pulse-public/pulse --version "${VERSION}"', helm_pages)
        self.assertNotIn("helm status pulse || true", helm_pages)
        self.assertNotIn("kubectl describe pods", helm_pages)
        self.assertIn("release-convergence.yml/dispatches", release_workflow)
        self.assertIn("Release Activation Commit Verdict", release_workflow)
        self.assertNotIn('gh workflow run update-demo-server.yml --ref "${REQUIRED_BRANCH}"', release_workflow)
        self.assertNotIn('TARGET="preview-v6"', release_workflow)
        self.assertIn("sync_chart_release_metadata.py", helm)
        self.assertNotIn("sync_chart_release_metadata.py", helm_pages)
        self.assertIn("--chart deploy/helm/pulse/Chart.yaml", helm)
        self.assertIn('git checkout --detach "refs/tags/${RELEASE_TAG}"', helm)
        self.assertIn("Verify public GHCR chart identity and provenance", helm)
        self.assertIn("helm registry logout ghcr.io || true", helm)
        self.assertIn("actions/attest@", helm)
        self.assertIn("verify-release-helm-chart.sh", helm)
        self.assertIn("subject-digest: ${{ steps.push.outputs.chart_digest }}", helm)
        self.assertIn("value: ${{ jobs.publish.outputs.chart_digest }}", helm)
        self.assertIn("chart_digest: ${{ steps.proof.outputs.chart_digest }}", helm)
        self.assertIn("${GITHUB_SHA} does not match ${RELEASE_TAG}", helm)
        self.assertIn("verify-release-helm-chart.sh", helm_pages)
        self.assertRegex(
            helm_pages,
            r"(?s)- name: Verify immutable chart identity\n\s+env:\n\s+GH_TOKEN: \$\{\{ github\.token \}\}",
        )
        self.assertIn(
            "chart_digest: ${{ needs.acquire_customer_promotion_lease.outputs.helm_chart_digest }}",
            convergence,
        )
        self.assertNotIn("versions/latest/restore", helm)
        self.assertNotIn("-f visibility=public", helm)
        self.assertNotIn("Package visibility configuration attempted", helm)
        self.assertNotIn("blob/main/docs/KUBERNETES.md", chart)
        self.assertNotIn("raw.githubusercontent.com/rcourtman/Pulse/main/docs/images/pulse-logo.svg", chart)
        self.assertIn("blob/{tag}/docs/KUBERNETES.md", chart_sync)
        self.assertIn("raw.githubusercontent.com/{repo}/{tag}/docs/images/pulse-logo.svg", chart_sync)
        self.assertIn("both stable and prerelease releases dispatch", runbook)
        self.assertIn("Release `6.0.0` from `pulse/v6-release`", runbook)
        self.assertIn("Prerelease public demo deployment is retired after v6 GA", runbook)
        self.assertNotIn("separate v6 preview demo environment", runbook)
        self.assertNotIn("preview-v6", runbook)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(runbook))
        self.assertIn("waitUntil: 'domcontentloaded'", demo_smoke)
        self.assertIn("getByLabel('Username').waitFor({ state: 'visible', timeout: 120000 })", demo_smoke)
        self.assertIn("getByLabel('Password').waitFor({ state: 'visible', timeout: 120000 })", demo_smoke)
        self.assertIn("getByRole('button', { name: 'Sign in to Pulse' }).waitFor({ state: 'visible', timeout: 120000 })", demo_smoke)
        self.assertIn("PULSE_DEMO_AUTH_USER", demo_smoke)
        self.assertIn("PULSE_DEMO_AUTH_PASS", demo_smoke)
        self.assertIn("getByRole('button', { name: 'Sign in to Pulse' }).click()", demo_smoke)
        self.assertIn("page.locator('main').waitFor({ state: 'visible', timeout: 60000 })", demo_smoke)
        self.assertIn("getByRole('status', { name: 'Backend and live data stream are connected.' })", demo_smoke)
        self.assertNotIn("getByText('Connected', { exact: true })", demo_smoke)
        self.assertIn("Public demo remained on the authenticated loading shell", demo_smoke)
        self.assertNotIn("waitUntil: 'networkidle'", demo_smoke)

    def test_stable_demo_recovery_is_fixed_current_bits_only(self) -> None:
        workflow = read(".github/workflows/recover-demo-server.yml")
        recovery = read(".github/scripts/recover-demo-runtime.sh")

        self.assertIn("workflow_dispatch:", workflow)
        self.assertNotIn("workflow_call:", workflow)
        self.assertNotIn("inputs:", workflow)
        self.assertIn("environment: demo-stable", workflow)
        self.assertIn("contents: read", workflow)
        self.assertIn("cancel-in-progress: false", workflow)
        self.assertIn('gh api "repos/${GITHUB_REPOSITORY}/releases/latest"', workflow)
        self.assertIn("Resolve target-compatible recovery profile", workflow)
        self.assertIn("resolve-demo-runtime-profile.sh", workflow)
        self.assertIn(".github/scripts/recover-demo-runtime.sh", workflow)
        self.assertIn("Verify public health", workflow)
        self.assertIn("Verify public frontend parity", workflow)
        self.assertIn("./scripts/run_demo_public_browser_smoke.sh", workflow)
        self.assertIn("Compensate failed mutated recovery", workflow)
        self.assertIn("Capture bounded Pulse failure diagnostics", workflow)
        self.assertIn("demo-runtime-diagnostics.txt", workflow)
        self.assertIn("journalctl -u pulse --since '-30 minutes'", workflow)
        self.assertIn("journalctl -u pulse --since '-10 minutes'", workflow)
        self.assertIn("tail -400", workflow)
        self.assertIn("sudo systemctl stop pulse", workflow)
        self.assertIn("Retain bounded recovery evidence", workflow)
        self.assertNotIn("release-convergence.yml", workflow)
        self.assertNotIn("install.sh", workflow)
        self.assertNotIn("scp ", workflow)

        self.assertIn('if [ "$#" -ne 16 ]', recovery)
        self.assertIn('SERVICE_NAME="pulse"', recovery)
        self.assertIn('RELAY_SERVICE_NAME="pulse-relay"', recovery)
        self.assertIn('EXPECTED_BINARY="/opt/pulse/bin/pulse"', recovery)
        self.assertIn('EXPECTED_UNIT="/etc/systemd/system/pulse.service"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_NODES "$MOCK_NODES"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_VMS_PER_NODE "$MOCK_VMS_PER_NODE"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_LXCS_PER_NODE "$MOCK_LXCS_PER_NODE"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_DOCKER_HOSTS "$MOCK_DOCKER_HOSTS"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_DOCKER_CONTAINERS "$MOCK_DOCKER_CONTAINERS"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_GENERIC_HOSTS "$MOCK_GENERIC_HOSTS"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_K8S_CLUSTERS "$MOCK_K8S_CLUSTERS"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_K8S_NODES "$MOCK_K8S_NODES"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_K8S_PODS "$MOCK_K8S_PODS"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_K8S_DEPLOYMENTS "$MOCK_K8S_DEPLOYMENTS"', recovery)
        self.assertIn("set_env_value PULSE_MOCK_SEED_METRICS_STORE false", recovery)
        self.assertIn('grep -Fxq "PULSE_MOCK_SEED_METRICS_STORE=false"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_TRENDS_SEED_DURATION "$MOCK_SEED_DURATION"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_TRENDS_SAMPLE_INTERVAL "$MOCK_SAMPLE_INTERVAL"', recovery)
        self.assertIn('set_env_value PULSE_MOCK_UPDATE_INTERVAL "$MOCK_UPDATE_INTERVAL"', recovery)
        self.assertIn('sudo systemctl restart "$SERVICE_NAME"', recovery)
        self.assertIn('sudo systemctl stop "$SERVICE_NAME"', recovery)
        self.assertIn('sudo kill -QUIT "$failed_pid"', recovery)
        self.assertIn("clear_demo_operational_history", recovery)
        self.assertIn("restore_demo_operational_history", recovery)
        self.assertIn("alerts/events.db", recovery)
        self.assertIn("alerts/alert-history.json.imported", recovery)
        self.assertIn("alerts/alert-history.backup.json.imported", recovery)
        self.assertIn("ai_incidents.json", recovery)
        self.assertIn("demo-operational-history.tar", recovery)
        self.assertNotIn('rm -f "/etc/pulse/', recovery)
        self.assertIn('AFTER_RELAY_PID" = "$BEFORE_RELAY_PID', recovery)
        self.assertIn('AFTER_BINARY_SHA" = "$BEFORE_BINARY_SHA', recovery)
        self.assertIn('AFTER_UNIT_SHA" = "$BEFORE_UNIT_SHA', recovery)
        self.assertIn('AFTER_DROPINS_SHA" = "$BEFORE_DROPINS_SHA', recovery)
        self.assertNotIn('AFTER_CONFIG_SHA" = "$BEFORE_CONFIG_SHA', recovery)
        self.assertIn("/etc/pulse/.env /etc/pulse/billing.json", recovery)
        self.assertIn("restore_runtime_config", recovery)
        self.assertIn("sudo cp --archive", recovery)
        self.assertNotIn("eval ", recovery)
        self.assertNotIn("sudo bash", recovery)

    def test_blocked_record_tracks_current_target_and_candidate_version(self) -> None:
        blocked_record_surface = {
            "VERSION",
            "docs/release-control/control_plane.json",
            "docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
            "docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-04-04.md",
            "scripts/release_control/record_rc_to_ga_blocked.py",
        }
        if USE_STAGED_GOVERNANCE and not any(path in blocked_record_surface for path in STAGED_FILES):
            self.skipTest("staged slice does not touch the blocked-record promotion surface")
        blocked = read("docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-04-04.md")
        current_version = read("VERSION").strip()
        active_target_id = read_json("docs/release-control/control_plane.json")["active_target_id"]
        if stable_packet_paths_for_version(current_version) is not None:
            self.assertIn("VERSION=6.0.0", blocked)
            if current_version != "6.0.0":
                self.assertNotIn(f"VERSION={current_version}", blocked)
        else:
            self.assertTrue(
                rc_packet_paths_for_version(current_version) is not None
                or support_prerelease_packet_paths_for_version(current_version) is not None,
                f"VERSION={current_version} does not match a governed v6 prerelease packet pattern",
            )
            self.assertIn("VERSION=6.0.0", blocked)
            self.assertNotIn(f"VERSION={current_version}", blocked)
        self.assertIn("artifact-owned candidate stable tag", blocked)
        self.assertIn("artifact-owned promotion channel", blocked)
        self.assertIn("artifact-owned promoted prerelease tag", blocked)
        self.assertIn("artifact-owned rollback target", blocked)
        self.assertIn("Materialize the final rehearsal record from that artifact without", blocked)
        self.assertIn("hand-repairing any missing candidate tag, promoted prerelease tag, rollback", blocked)
        if active_target_id == "v6-ga-promotion":
            self.assertIn(
                f"The active control-plane target is `{active_target_id}`, so stable or GA",
                blocked,
            )
        elif active_target_id == "v6-product-lane-expansion":
            self.assertIn(
                "The active control-plane target is `v6-ga-promotion`, so stable or GA",
                blocked,
            )
        else:
            self.assertIn(f"The active control-plane target is still `{active_target_id}`, not", blocked)
        matrix = read("docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md")
        self.assertIn(promotion_metadata_envelope(), normalize_ws(matrix))
        expected = blocked_record.build_blocked_record(record_date="2026-04-04")
        if current_version != "6.0.0" or active_target_id != "v6-ga-promotion":
            return
        if blocked != expected:
            record_path = REPO_ROOT / "docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-04-04.md"
            if os.environ.get("BLESS_GOVERNANCE_FIXTURES") == "1":
                record_path.write_text(expected, encoding="utf-8")
                self.skipTest(
                    "Regenerated rc-to-ga-promotion-readiness-blocked-2026-04-04.md "
                    "under BLESS_GOVERNANCE_FIXTURES=1; stage the file and rerun without the env var."
                )
            self.fail(
                "Blocked-record fixture drifted from build_blocked_record() output. "
                "This usually means VERSION bumped or a new RC tag landed since the "
                "fixture was last regenerated. To fix, run either:\n"
                "  python3 scripts/release_control/record_rc_to_ga_blocked.py "
                "--output docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-04-04.md "
                "--record-date 2026-04-04\n"
                "  (or)\n"
                "  BLESS_GOVERNANCE_FIXTURES=1 python3 -m unittest release_promotion_policy_test"
            )

    def test_routine_stable_patch_entrypoint_is_noninteractive_and_integrated(self) -> None:
        helper = read("scripts/trigger-stable-patch.sh")
        policy = read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        contract = read("docs/release-control/v6/internal/subsystems/deployment-installability.md")

        self.assertNotIn("read -r", helper)
        self.assertNotIn("read -p", helper)
        self.assertIn("--dry-run", helper)
        self.assertIn("--derive-rollback-latest-stable", helper)
        self.assertIn("docs/releases/RELEASE_NOTES_v${VERSION}.md", helper)
        self.assertIn("Use --dry-run only", helper)
        self.assertNotIn("timedelta(hours=24)", helper)
        self.assertNotIn(".createdAt >= $cutoff", helper)
        self.assertIn("gh workflow run create-release.yml", helper)
        self.assertIn("gh workflow run \"$WORKFLOW\"", helper)
        self.assertIn("--unsigned-windows-exception-reason", helper)
        self.assertIn("--unsigned-windows-exception", helper)
        self.assertIn("unsigned_windows_exception", helper)
        self.assertIn("unsigned_windows_reason", helper)
        self.assertIn('--arg hotfix_exception "$HOTFIX_EXCEPTION"', helper)
        self.assertIn(
            '--arg unsigned_windows_exception "$UNSIGNED_WINDOWS_EXCEPTION"',
            helper,
        )
        self.assertIn('--arg draft_only "false"', helper)
        self.assertNotIn("--argjson", helper)
        self.assertIn("Single-Build Release Path", policy)
        self.assertIn("Routine Stable Patch Path", policy)
        self.assertIn("single publish workflow performs the exact-SHA preflight", normalize_ws(policy))
        self.assertIn("An asynchronous dispatch or manual SSH deployment is not release completion.", normalize_ws(contract))


class MaterialApprovalEvidenceTest(unittest.TestCase):
    def test_source_of_truth_requires_quantitative_approval_provenance(self) -> None:
        content = normalize_ws(
            read("docs/release-control/v6/internal/SOURCE_OF_TRUTH.md")
        )
        self.assertIn(
            "material owner-approval record that relies on quantitative rationale",
            content,
        )
        self.assertIn(
            "durable source, exact query or canonical report command, sanitized snapshot, and measurement time",
            content,
        )
        self.assertIn("separate durable owner-confirmation artifact", content)

    def test_quantitative_owner_approval_requires_reproducible_provenance(self) -> None:
        content = """# Material decision

The release owner explicitly approved this change because weekly active installs
grew from 100 to 1,000 and conversion reached 7.9%.
"""
        self.assertEqual(
            material_approval_evidence_errors(content),
            ("missing '## Quantitative Evidence' section",),
        )

    def test_quantitative_evidence_rejects_placeholder_provenance(self) -> None:
        content = """# Material decision

Approved by the project owner after conversion reached 7.9%.

## Quantitative Evidence

- Source: unavailable
- Query: `scripts/report.py --as-of 2026-08-08`
- Snapshot: `records/report-2026-08-08.json`
- Measured at: `2026-08-08T01:47:00Z`
"""
        self.assertEqual(
            material_approval_evidence_errors(content),
            ("'- Source:' must identify durable reproducible evidence",),
        )

    def test_quantitative_evidence_rejects_unreferenced_field_values(self) -> None:
        content = """# Material decision

The product owner explicitly approved this change after a cohort reached 7.9%.

## Quantitative Evidence

- Source: production telemetry
- Query: weekly conversion report
- Snapshot: sanitized aggregate results
- Measured at: `2026-08-08T01:47:00Z`
"""
        self.assertEqual(
            material_approval_evidence_errors(content),
            (
                "'- Source:' must contain an inline durable reference",
                "'- Query:' must contain an inline durable reference",
                "'- Snapshot:' must contain an inline durable reference",
            ),
        )

    def test_quantitative_evidence_rejects_missing_local_artifact(self) -> None:
        content = """# Material decision

Approved by the project owner after conversion reached 7.9%.

## Quantitative Evidence

- Source: `missing-evidence.md`
- Query: `missing-evidence.md#query`
- Snapshot: `missing-evidence.md#snapshot`
- Measured at: `2026-08-08T01:47:00Z`
"""
        record_rel = "docs/release-control/v6/internal/records/example.md"
        self.assertEqual(
            material_approval_evidence_errors(content, record_rel=record_rel),
            (
                "'- Source:' references missing artifact 'docs/release-control/v6/internal/records/missing-evidence.md'",
                "'- Query:' references missing artifact 'docs/release-control/v6/internal/records/missing-evidence.md'",
                "'- Snapshot:' references missing artifact 'docs/release-control/v6/internal/records/missing-evidence.md'",
            ),
        )

    def test_quantitative_evidence_accepts_complete_provenance(self) -> None:
        content = """# Material decision

The product owner explicitly approved this change after a cohort reached 7.9%.

## Quantitative Evidence

- Source: `pulse-pro:license-server/telemetry.sqlite`
- Query: `scripts/report.py --as-of 2026-08-08`
- Snapshot: `records/report-2026-08-08.json`
- Measured at: `2026-08-08T01:47:00Z`
"""
        self.assertEqual(material_approval_evidence_errors(content), ())

    def test_tracked_material_approval_records_have_reproducible_provenance(self) -> None:
        failures: list[str] = []
        for rel in tracked_release_control_records():
            errors = material_approval_evidence_errors(read(rel), record_rel=rel)
            failures.extend(f"{rel}: {error}" for error in errors)
        self.assertEqual(failures, [], "\n".join(failures))


if __name__ == "__main__":
    unittest.main()
