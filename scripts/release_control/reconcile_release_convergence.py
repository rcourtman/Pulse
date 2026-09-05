#!/usr/bin/env python3
"""Reconcile a failed release convergence run without replaying stale controls."""

from __future__ import annotations

import argparse
from datetime import datetime
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from typing import Any, Iterable


CONVERGENCE_PATH = ".github/workflows/release-convergence.yml"
CREATE_RELEASE_PATH = ".github/workflows/create-release.yml"
DISPLAY_TITLE = re.compile(r"^Release convergence (v[^\s]+) source ([1-9][0-9]*)$")
PRIVATE_PROMOTION_RUN = re.compile(
    r"https://github\.com/rcourtman/pulse-pro/actions/runs/([1-9][0-9]*)"
)
PRIVATE_REPOSITORY = "rcourtman/pulse-pro"
PRIVATE_PROMOTION_PATH = ".github/workflows/promote-paid-runtime-release.yml"
PAID_RUNTIME_JOB = "Converge paid-runtime broker / promote"
CONVERGENCE_VERDICT_JOB = "Customer Promotion Convergence Verdict"
CREDENTIAL_CONTAINMENT_JOB = "Require credential containment"
CREDENTIAL_BLOCK_MARKER = "credential containment gate: BLOCKED"
CREDENTIAL_CONTAINMENT_PATHS = (
    "scripts/check_credential_containment.py",
    "docs/security-rotation.md",
)
RELEASE_TAG = re.compile(
    r"^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-(?:alpha|beta|rc)\.[1-9][0-9]*)?$"
)
DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")
EXACT_SHA = re.compile(r"^[0-9a-f]{40}$")
TERMINAL_FAILURES = {
    "action_required",
    "cancelled",
    "failure",
    "stale",
    "startup_failure",
    "timed_out",
}


class ReconciliationError(ValueError):
    """The remote evidence is incomplete or inconsistent."""


class GitHubNotFound(ReconciliationError):
    """GitHub returned a definite 404 for an object that may not exist yet."""


def positive_int(value: object, subject: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        raise ReconciliationError(f"{subject} is not a positive integer")
    return value


def timestamp(value: object, subject: str) -> datetime:
    if not isinstance(value, str):
        raise ReconciliationError(f"{subject} has no timestamp")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ReconciliationError(f"{subject} has an invalid timestamp") from exc
    if parsed.tzinfo is None:
        raise ReconciliationError(f"{subject} timestamp has no timezone")
    return parsed


def require_run_identity(
    run: dict[str, Any],
    *,
    repository: str,
    workflow_id: int,
    path: str,
    display_title: str | None = None,
) -> None:
    checks = {
        "repository": run.get("repository", {}).get("full_name") == repository,
        "workflow": run.get("workflow_id") == workflow_id,
        "workflow path": run.get("path") == path,
        "event": run.get("event") == "workflow_dispatch",
    }
    if display_title is not None:
        checks["display title"] = run.get("display_title") == display_title
    failed = [name for name, valid in checks.items() if not valid]
    if failed:
        raise ReconciliationError("run has mismatched " + ", ".join(failed))


def validate_marker(
    marker: object,
    *,
    release: dict[str, Any],
    tag: str,
    source_run_id: int,
) -> int:
    if not isinstance(marker, dict):
        raise ReconciliationError("activation marker is not an object")
    release_id = positive_int(release.get("id"), "release ID")
    source_sha = release.get("target_commitish")
    if not isinstance(source_sha, str) or EXACT_SHA.fullmatch(source_sha) is None:
        raise ReconciliationError("release target is not an exact commit")
    checks = {
        "schema": marker.get("schema_version") == 1,
        "tag": marker.get("tag") == tag,
        "release ID": marker.get("release_id") == str(release_id),
        "source commit": marker.get("target_commitish") == source_sha,
        "source release run": marker.get("source_release_run_id") == str(source_run_id),
        "R2 prefix": isinstance(marker.get("r2_prefix"), str),
        "server digest": bool(DIGEST.fullmatch(str(marker.get("server_image_digest", "")))),
        "control-plane digest": bool(
            DIGEST.fullmatch(str(marker.get("control_plane_image_digest", "")))
        ),
        "Helm digest": bool(DIGEST.fullmatch(str(marker.get("helm_chart_digest", "")))),
    }
    failed = [name for name, valid in checks.items() if not valid]
    if failed:
        raise ReconciliationError("activation marker has mismatched " + ", ".join(failed))
    owner = marker.get("convergence_run_id")
    if not isinstance(owner, str) or not owner.isdigit() or int(owner) <= 0:
        raise ReconciliationError("activation marker has an invalid convergence owner")
    return int(owner)


def latest_failed_runs(
    releases: Iterable[object],
    runs: Iterable[object],
    *,
    workflow_id: int,
    default_branch: str,
) -> list[int]:
    """Return the latest failed convergence for each current immutable channel."""
    latest_release: dict[bool, tuple[datetime, dict[str, Any]]] = {}
    for index, value in enumerate(releases):
        if not isinstance(value, dict) or value.get("draft") is not False:
            continue
        prerelease = value.get("prerelease")
        tag = value.get("tag_name")
        if (
            not isinstance(prerelease, bool)
            or not isinstance(tag, str)
            or RELEASE_TAG.fullmatch(tag) is None
        ):
            continue
        published = timestamp(value.get("published_at"), f"release {index}")
        if prerelease not in latest_release or published > latest_release[prerelease][0]:
            latest_release[prerelease] = (published, value)

    # Never fall back to an older release when the advertised channel head is
    # mutable. That is continuity debt requiring a replacement, not a target
    # whose aliases should be promoted again.
    current_tags = {
        str(release["tag_name"])
        for _, release in latest_release.values()
        if release.get("immutable") is True
    }
    newest: dict[str, tuple[datetime, dict[str, Any]]] = {}
    for index, value in enumerate(runs):
        if not isinstance(value, dict):
            continue
        title = value.get("display_title")
        match = DISPLAY_TITLE.fullmatch(title) if isinstance(title, str) else None
        if match is None or match.group(1) not in current_tags:
            continue
        if (
            value.get("workflow_id") != workflow_id
            or value.get("path") != CONVERGENCE_PATH
            or value.get("event") != "workflow_dispatch"
            or value.get("head_branch") != default_branch
        ):
            continue
        created = timestamp(value.get("created_at"), f"convergence run {index}")
        tag = match.group(1)
        if tag not in newest or created > newest[tag][0]:
            newest[tag] = (created, value)

    result: list[int] = []
    for _, run in newest.values():
        if run.get("status") == "completed" and run.get("conclusion") in TERMINAL_FAILURES:
            result.append(positive_int(run.get("id"), "convergence run ID"))
    return sorted(result)


class GitHub:
    def __init__(
        self,
        repository: str,
        gh: str,
        *,
        mutate: bool = True,
        private_token: str = "",
    ) -> None:
        self.repository = repository
        self.gh = gh
        self.mutate = mutate
        self.private_token = private_token

    def _run(
        self,
        arguments: list[str],
        *,
        output: bool = True,
        token: str = "",
    ) -> str:
        env = None
        if token:
            env = os.environ.copy()
            env["GH_TOKEN"] = token
        result = subprocess.run(
            [self.gh, *arguments],
            check=False,
            capture_output=output,
            text=True,
            env=env,
        )
        if result.returncode != 0:
            detail = result.stderr.strip().splitlines() if output else []
            suffix = f": {detail[-1]}" if detail else ""
            if "HTTP 404" in result.stderr:
                raise GitHubNotFound(
                    f"GitHub object was not found ({' '.join(arguments[:3])}){suffix}"
                )
            raise ReconciliationError(f"GitHub command failed ({' '.join(arguments[:3])}){suffix}")
        return result.stdout if output else ""

    def api(self, endpoint: str, *, token: str = "") -> dict[str, Any]:
        try:
            value = json.loads(
                self._run(
                    [
                        "api",
                        "-H",
                        "Accept: application/vnd.github+json",
                        "-H",
                        "X-GitHub-Api-Version: 2026-03-10",
                        endpoint,
                    ],
                    token=token,
                )
            )
        except json.JSONDecodeError as exc:
            raise ReconciliationError(f"GitHub returned invalid JSON for {endpoint}") from exc
        if not isinstance(value, dict):
            raise ReconciliationError(f"GitHub returned a non-object for {endpoint}")
        return value

    def pages(self, endpoint: str, *, token: str = "") -> list[object]:
        output = self._run(["api", "--paginate", endpoint], token=token)
        decoder = json.JSONDecoder()
        value: list[object] = []
        offset = 0
        while offset < len(output):
            while offset < len(output) and output[offset].isspace():
                offset += 1
            if offset == len(output):
                break
            try:
                page, offset = decoder.raw_decode(output, offset)
            except json.JSONDecodeError as exc:
                raise ReconciliationError(
                    f"GitHub returned invalid pages for {endpoint}"
                ) from exc
            value.append(page)
        if not value:
            raise ReconciliationError(f"GitHub returned no pages for {endpoint}")
        return value

    def post(self, endpoint: str, payload: dict[str, Any] | None = None) -> None:
        if not self.mutate:
            print(f"DRY RUN: POST {endpoint}")
            return
        arguments = [
            "api",
            "--method",
            "POST",
            "-H",
            "Accept: application/vnd.github+json",
            "-H",
            "X-GitHub-Api-Version: 2026-03-10",
            endpoint,
        ]
        if payload is not None:
            arguments.extend(["--input", "-"])
            result = subprocess.run(
                [self.gh, *arguments],
                input=json.dumps(payload, separators=(",", ":")),
                check=False,
                capture_output=True,
                text=True,
            )
            if result.returncode != 0:
                detail = result.stderr.strip().splitlines()
                suffix = f": {detail[-1]}" if detail else ""
                raise ReconciliationError(f"GitHub mutation failed for {endpoint}{suffix}")
            return
        self._run(arguments)

    def download_marker(self, tag: str, directory: Path) -> dict[str, Any]:
        self._run(
            [
                "release",
                "download",
                tag,
                "--repo",
                self.repository,
                "--pattern",
                "release-activation.json",
                "--dir",
                str(directory),
            ]
        )
        try:
            value = json.loads((directory / "release-activation.json").read_text())
        except (OSError, json.JSONDecodeError) as exc:
            raise ReconciliationError("downloaded activation marker is unreadable") from exc
        if not isinstance(value, dict):
            raise ReconciliationError("downloaded activation marker is not an object")
        return value

    def job_log(self, repository: str, job_id: int, *, token: str = "") -> str:
        return self._run(
            [
                "api",
                "-H",
                "Accept: application/vnd.github+json",
                "-H",
                "X-GitHub-Api-Version: 2026-03-10",
                f"repos/{repository}/actions/jobs/{job_id}/logs",
            ],
            token=token,
        )

    def unchanged_credential_containment_block(self, run_id: int) -> bool:
        """Whether the paid-runtime failure is an unchanged operator-owned block."""
        if not self.private_token:
            return False
        jobs = flatten_pages(
            self.pages(
                f"repos/{self.repository}/actions/runs/{run_id}/jobs?per_page=100"
            ),
            "jobs",
        )
        paid_jobs = [
            value
            for value in jobs
            if isinstance(value, dict)
            and value.get("name") == PAID_RUNTIME_JOB
            and value.get("conclusion") == "failure"
        ]
        if len(paid_jobs) != 1:
            return False
        # Containment may explain the paid-runtime failure and the aggregate
        # verdict it necessarily makes red. It cannot explain another failed
        # customer surface, an interrupted finalizer, or incomplete job
        # evidence; those remain ordinary convergence debt and must retain the
        # unattended retry path.
        expected_failures = sorted((PAID_RUNTIME_JOB, CONVERGENCE_VERDICT_JOB))
        if any(
            not isinstance(value, dict)
            or not isinstance(value.get("name"), str)
            or not value.get("name")
            or value.get("status") != "completed"
            or value.get("conclusion") not in {"success", "failure", "skipped"}
            for value in jobs
        ):
            return False
        failed_jobs = sorted(
            value["name"] for value in jobs if value["conclusion"] == "failure"
        )
        if failed_jobs != expected_failures:
            return False
        paid_job_id = positive_int(paid_jobs[0].get("id"), "paid-runtime job ID")
        annotations = flatten_pages(
            self.pages(
                f"repos/{self.repository}/check-runs/{paid_job_id}/annotations?per_page=100"
            )
        )
        private_run_ids = {
            int(match.group(1))
            for value in annotations
            if isinstance(value, dict) and isinstance(value.get("message"), str)
            for match in PRIVATE_PROMOTION_RUN.finditer(value["message"])
        }
        if len(private_run_ids) != 1:
            return False
        private_run_id = private_run_ids.pop()
        private_run = self.api(
            f"repos/{PRIVATE_REPOSITORY}/actions/runs/{private_run_id}",
            token=self.private_token,
        )
        if (
            private_run.get("repository", {}).get("full_name") != PRIVATE_REPOSITORY
            or private_run.get("path") != PRIVATE_PROMOTION_PATH
            or private_run.get("event") != "workflow_dispatch"
            or private_run.get("status") != "completed"
            or private_run.get("conclusion") != "failure"
        ):
            return False
        private_jobs = flatten_pages(
            self.pages(
                f"repos/{PRIVATE_REPOSITORY}/actions/runs/{private_run_id}/jobs?per_page=100",
                token=self.private_token,
            ),
            "jobs",
        )
        containment_jobs = [
            value
            for value in private_jobs
            if isinstance(value, dict)
            and value.get("name") == CREDENTIAL_CONTAINMENT_JOB
            and value.get("conclusion") == "failure"
        ]
        if len(containment_jobs) != 1:
            return False
        containment_job_id = positive_int(
            containment_jobs[0].get("id"), "credential-containment job ID"
        )
        containment_log = self.job_log(
            PRIVATE_REPOSITORY, containment_job_id, token=self.private_token
        )
        if CREDENTIAL_BLOCK_MARKER not in containment_log:
            return False
        blocked_head = private_run.get("head_sha")
        current_head = self.api(
            f"repos/{PRIVATE_REPOSITORY}/commits/main", token=self.private_token
        ).get("sha")
        if (
            not isinstance(blocked_head, str)
            or EXACT_SHA.fullmatch(blocked_head) is None
            or not isinstance(current_head, str)
            or EXACT_SHA.fullmatch(current_head) is None
        ):
            raise ReconciliationError("private repository head is not an exact commit")

        def containment_state(ref: str) -> tuple[str, ...]:
            blobs: list[str] = []
            for path in CREDENTIAL_CONTAINMENT_PATHS:
                value = self.api(
                    f"repos/{PRIVATE_REPOSITORY}/contents/{path}?ref={ref}",
                    token=self.private_token,
                )
                blob = value.get("sha")
                if value.get("type") != "file" or not isinstance(blob, str) or not blob:
                    raise ReconciliationError(
                        f"private credential-containment input is invalid: {path}"
                    )
                blobs.append(blob)
            return tuple(blobs)

        return containment_state(blocked_head) == containment_state(current_head)


def flatten_pages(pages: Iterable[object], key: str | None = None) -> list[object]:
    values: list[object] = []
    for index, page in enumerate(pages):
        if key is None:
            if not isinstance(page, list):
                raise ReconciliationError(f"GitHub page {index} is not a list")
            values.extend(page)
        else:
            if not isinstance(page, dict) or not isinstance(page.get(key), list):
                raise ReconciliationError(f"GitHub page {index} has no {key} list")
            values.extend(page[key])
    return values


def reconcile(github: GitHub, run_id: int, max_attempts: int) -> None:
    repository = github.repository
    repository_state = github.api(f"repos/{repository}")
    default_branch = repository_state.get("default_branch")
    if not isinstance(default_branch, str) or not default_branch:
        raise ReconciliationError("repository has no default branch")
    convergence_workflow = github.api(
        f"repos/{repository}/actions/workflows/release-convergence.yml"
    )
    create_workflow = github.api(f"repos/{repository}/actions/workflows/create-release.yml")
    convergence_workflow_id = positive_int(convergence_workflow.get("id"), "workflow ID")
    create_workflow_id = positive_int(create_workflow.get("id"), "create-release workflow ID")

    run = github.api(f"repos/{repository}/actions/runs/{run_id}")
    require_run_identity(
        run,
        repository=repository,
        workflow_id=convergence_workflow_id,
        path=CONVERGENCE_PATH,
    )
    if run.get("status") != "completed" or run.get("conclusion") not in TERMINAL_FAILURES:
        print(f"Convergence run {run_id} no longer has terminal convergence debt; no action.")
        return
    title = run.get("display_title")
    match = DISPLAY_TITLE.fullmatch(title) if isinstance(title, str) else None
    if match is None:
        raise ReconciliationError("convergence display title has an invalid identity")
    tag, source_text = match.groups()
    if RELEASE_TAG.fullmatch(tag) is None:
        raise ReconciliationError("convergence display title has an invalid release tag")
    source_run_id = int(source_text)

    all_runs = flatten_pages(
        github.pages(
            f"repos/{repository}/actions/workflows/release-convergence.yml/runs"
            "?event=workflow_dispatch&per_page=100"
        ),
        "workflow_runs",
    )
    matching_runs = [
        value
        for value in all_runs
        if isinstance(value, dict)
        and value.get("workflow_id") == convergence_workflow_id
        and value.get("display_title") == title
    ]
    if not matching_runs:
        raise ReconciliationError("convergence history omitted the requested run")
    newest = max(matching_runs, key=lambda item: timestamp(item.get("created_at"), "run"))
    if newest.get("id") != run_id:
        print(f"A newer convergence run already owns {title}; no action.")
        return
    try:
        release = github.api(f"repos/{repository}/releases/tags/{tag}")
    except GitHubNotFound:
        release = {}
    committed = (
        release.get("tag_name") == tag
        and release.get("draft") is False
        and release.get("immutable") is True
        and isinstance(release.get("published_at"), str)
        and bool(release.get("published_at"))
    )
    if not committed:
        source = github.api(f"repos/{repository}/actions/runs/{source_run_id}")
        require_run_identity(
            source,
            repository=repository,
            workflow_id=create_workflow_id,
            path=CREATE_RELEASE_PATH,
        )
        if source.get("status") != "completed" and positive_int(
            run.get("run_attempt"), "run attempt"
        ) < 50:
            github.post(f"repos/{repository}/actions/runs/{run_id}/rerun")
            print(f"Renewed pre-commit convergence owner {run_id} for active source {source_run_id}.")
            return
        print(f"{tag} has no immutable activation commit; no convergence retry was dispatched.")
        return

    if github.unchanged_credential_containment_block(run_id):
        print(
            f"Convergence run {run_id} is held by unchanged private credential containment; "
            "no unattended retry was dispatched."
        )
        return

    with tempfile.TemporaryDirectory(prefix="release-convergence-") as raw:
        marker = github.download_marker(tag, Path(raw))
    owner_run_id = validate_marker(
        marker, release=release, tag=tag, source_run_id=source_run_id
    )
    owner_run = github.api(f"repos/{repository}/actions/runs/{owner_run_id}")
    require_run_identity(
        owner_run,
        repository=repository,
        workflow_id=convergence_workflow_id,
        path=CONVERGENCE_PATH,
        display_title=title,
    )
    if owner_run.get("status") != "completed":
        raise ReconciliationError("activation marker owner is not terminal")

    default_commit = github.api(f"repos/{repository}/commits/{default_branch}").get("sha")
    if not isinstance(default_commit, str) or EXACT_SHA.fullmatch(default_commit) is None:
        raise ReconciliationError("default branch did not resolve to an exact commit")
    if run.get("head_sha") == default_commit:
        # Exhaustion is a terminal state of the retry policy, not a failure of
        # the reconciler itself. The failed convergence run remains the durable
        # debt signal; scheduled reconciliation must not recreate the same
        # operational failure forever.
        attempts = sum(
            positive_int(item.get("run_attempt"), "run attempt")
            for item in matching_runs
            if item.get("head_sha") == run.get("head_sha")
        )
        if attempts >= max_attempts:
            print(
                f"{title} remains failed after the {attempts}-attempt retry budget; "
                "no unchanged-control retry was dispatched."
            )
            return
        github.post(f"repos/{repository}/actions/runs/{run_id}/rerun")
        print(f"Re-ran current-control convergence {run_id} for committed {tag}.")
        return

    prerelease = release.get("prerelease")
    if not isinstance(prerelease, bool):
        raise ReconciliationError("release has no prerelease classification")
    payload = {
        "ref": default_branch,
        "inputs": {
            "tag": tag,
            "version": tag.removeprefix("v"),
            # The workflow-dispatch API models input values as strings even
            # when the receiving workflow declares a boolean input.
            "prerelease": "true" if prerelease else "false",
            "target_commitish": marker["target_commitish"],
            "release_id": marker["release_id"],
            "r2_prefix": marker["r2_prefix"],
            "source_release_run_id": marker["source_release_run_id"],
        },
    }
    github.post(
        f"repos/{repository}/actions/workflows/release-convergence.yml/dispatches",
        payload,
    )
    print(
        f"Dispatched fresh convergence controls after observing {default_commit} for committed {tag}; "
        f"the failed run used {run.get('head_sha')}."
    )


def discover(github: GitHub) -> list[int]:
    repository = github.repository
    repository_state = github.api(f"repos/{repository}")
    default_branch = repository_state.get("default_branch")
    if not isinstance(default_branch, str) or not default_branch:
        raise ReconciliationError("repository has no default branch")
    workflow = github.api(f"repos/{repository}/actions/workflows/release-convergence.yml")
    workflow_id = positive_int(workflow.get("id"), "workflow ID")
    releases = flatten_pages(
        github.pages(f"repos/{repository}/releases?per_page=100")
    )
    runs = flatten_pages(
        github.pages(
            f"repos/{repository}/actions/workflows/release-convergence.yml/runs"
            "?event=workflow_dispatch&per_page=100"
        ),
        "workflow_runs",
    )
    return latest_failed_runs(
        releases,
        runs,
        workflow_id=workflow_id,
        default_branch=default_branch,
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", required=True)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--run-id", type=int)
    group.add_argument("--latest", action="store_true")
    parser.add_argument("--max-attempts", type=int, default=5)
    parser.add_argument("--dry-run", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.run_id is not None and args.run_id <= 0:
        print("run ID must be positive", file=sys.stderr)
        return 2
    if not 1 <= args.max_attempts <= 20:
        print("max attempts must be between 1 and 20", file=sys.stderr)
        return 2
    gh = os.environ.get("GH_BIN", "gh")
    github = GitHub(
        args.repository,
        gh,
        mutate=not args.dry_run,
        private_token=os.environ.get("PRO_REPOSITORY_TOKEN", ""),
    )
    try:
        run_ids = discover(github) if args.latest else [args.run_id]
        if not run_ids:
            print("No current immutable release has unattended convergence debt.")
            return 0
        for run_id in run_ids:
            assert run_id is not None
            reconcile(github, run_id, args.max_attempts)
    except (OSError, ReconciliationError) as exc:
        print(f"release convergence reconciliation failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
