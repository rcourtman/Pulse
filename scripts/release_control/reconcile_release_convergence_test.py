#!/usr/bin/env python3

from __future__ import annotations

import contextlib
import io
import unittest
from unittest.mock import patch
import subprocess

import reconcile_release_convergence as subject


WORKFLOW_ID = 42


def release(tag: str, published_at: str, *, prerelease: bool, immutable: bool = True):
    return {
        "id": abs(hash(tag)),
        "tag_name": tag,
        "draft": False,
        "prerelease": prerelease,
        "immutable": immutable,
        "published_at": published_at,
    }


def run(
    run_id: int,
    tag: str,
    created_at: str,
    *,
    status: str = "completed",
    conclusion: str = "failure",
):
    return {
        "id": run_id,
        "workflow_id": WORKFLOW_ID,
        "path": subject.CONVERGENCE_PATH,
        "event": "workflow_dispatch",
        "head_branch": "main",
        "display_title": f"Release convergence {tag} source 1234",
        "created_at": created_at,
        "status": status,
        "conclusion": conclusion,
    }


class LatestFailedRunsTests(unittest.TestCase):
    def select(self, releases, runs):
        return subject.latest_failed_runs(
            releases, runs, workflow_id=WORKFLOW_ID, default_branch="main"
        )

    def test_selects_failed_heads_of_stable_and_preview_channels(self):
        releases = [
            release("v6.4.1", "2026-09-01T00:00:00Z", prerelease=False),
            release("v6.5.0-rc.1", "2026-09-02T00:00:00Z", prerelease=True),
        ]
        runs = [
            run(10, "v6.4.1", "2026-09-01T00:01:00Z"),
            run(11, "v6.5.0-rc.1", "2026-09-02T00:01:00Z"),
        ]
        self.assertEqual([10, 11], self.select(releases, runs))

    def test_does_not_fall_back_behind_a_mutable_channel_head(self):
        releases = [
            release("v6.4.1", "2026-09-01T00:00:00Z", prerelease=False),
            release(
                "v6.4.2", "2026-09-02T00:00:00Z", prerelease=False, immutable=False
            ),
        ]
        self.assertEqual([], self.select(releases, [run(10, "v6.4.1", "2026-09-01T00:01:00Z")]))

    def test_ignores_newer_companion_chart_releases(self):
        releases = [
            release("v6.5.0-rc.1", "2026-09-02T00:00:00Z", prerelease=True),
            release(
                "helm-chart-6.5.0-rc.1",
                "2026-09-02T00:01:00Z",
                prerelease=True,
            ),
        ]
        self.assertEqual(
            [10],
            self.select(
                releases, [run(10, "v6.5.0-rc.1", "2026-09-02T00:02:00Z")]
            ),
        )

    def test_newer_success_or_active_run_clears_old_debt(self):
        releases = [release("v6.5.0-rc.1", "2026-09-02T00:00:00Z", prerelease=True)]
        failed = run(10, "v6.5.0-rc.1", "2026-09-02T00:01:00Z")
        success = run(
            11,
            "v6.5.0-rc.1",
            "2026-09-02T00:02:00Z",
            conclusion="success",
        )
        self.assertEqual([], self.select(releases, [failed, success]))
        active = run(
            12,
            "v6.5.0-rc.1",
            "2026-09-02T00:03:00Z",
            status="in_progress",
            conclusion="",
        )
        self.assertEqual([], self.select(releases, [failed, active]))

    def test_ignores_wrong_workflow_branch_and_title(self):
        releases = [release("v6.5.0-rc.1", "2026-09-02T00:00:00Z", prerelease=True)]
        wrong_workflow = run(10, "v6.5.0-rc.1", "2026-09-02T00:01:00Z")
        wrong_workflow["workflow_id"] = 99
        wrong_branch = run(11, "v6.5.0-rc.1", "2026-09-02T00:02:00Z")
        wrong_branch["head_branch"] = "release/v6.5"
        wrong_title = run(12, "v6.5.0-rc.1", "2026-09-02T00:03:00Z")
        wrong_title["display_title"] += " injected"
        self.assertEqual(
            [], self.select(releases, [wrong_workflow, wrong_branch, wrong_title])
        )


class MarkerTests(unittest.TestCase):
    def test_marker_binds_release_source_and_all_customer_digests(self):
        current_release = {
            "id": 88,
            "target_commitish": "a" * 40,
        }
        marker = {
            "schema_version": 1,
            "tag": "v6.5.0-rc.1",
            "release_id": "88",
            "target_commitish": "a" * 40,
            "source_release_run_id": "1234",
            "convergence_run_id": "5678",
            "r2_prefix": "packet",
            "server_image_digest": "sha256:" + "b" * 64,
            "control_plane_image_digest": "sha256:" + "c" * 64,
            "helm_chart_digest": "sha256:" + "d" * 64,
        }
        self.assertEqual(
            5678,
            subject.validate_marker(
                marker,
                release=current_release,
                tag="v6.5.0-rc.1",
                source_run_id=1234,
            ),
        )
        marker["server_image_digest"] = "sha256:bad"
        with self.assertRaisesRegex(subject.ReconciliationError, "server digest"):
            subject.validate_marker(
                marker,
                release=current_release,
                tag="v6.5.0-rc.1",
                source_run_id=1234,
            )


class FakeGitHub:
    repository = "rcourtman/Pulse"

    def __init__(self, *, current_controls: bool = False, committed: bool = True):
        self.run_id = 100
        self.source_run_id = 200
        self.owner_run_id = 90
        self.title = "Release convergence v6.5.0-rc.1 source 200"
        self.old_sha = "a" * 40
        self.main_sha = self.old_sha if current_controls else "b" * 40
        self.committed = committed
        self.posts = []
        self.runs = [self.run(self.run_id, self.old_sha)]
        self.marker = {
            "schema_version": 1,
            "tag": "v6.5.0-rc.1",
            "release_id": "300",
            "target_commitish": "c" * 40,
            "source_release_run_id": "200",
            "convergence_run_id": "90",
            "r2_prefix": "packet",
            "server_image_digest": "sha256:" + "d" * 64,
            "control_plane_image_digest": "sha256:" + "e" * 64,
            "helm_chart_digest": "sha256:" + "f" * 64,
        }

    def run(self, run_id, head_sha, *, attempt=1, status="completed", conclusion="failure"):
        return {
            "id": run_id,
            "workflow_id": 42,
            "path": subject.CONVERGENCE_PATH,
            "event": "workflow_dispatch",
            "repository": {"full_name": self.repository},
            "display_title": self.title,
            "head_branch": "main",
            "head_sha": head_sha,
            "status": status,
            "conclusion": conclusion,
            "run_attempt": attempt,
            "created_at": f"2026-09-02T00:{run_id % 60:02d}:00Z",
        }

    def api(self, endpoint):
        if endpoint == f"repos/{self.repository}":
            return {"default_branch": "main"}
        suffix = endpoint.removeprefix(f"repos/{self.repository}/")
        if suffix == "actions/workflows/release-convergence.yml":
            return {"id": 42}
        if suffix == "actions/workflows/create-release.yml":
            return {"id": 43}
        if suffix == "commits/main":
            return {"sha": self.main_sha}
        if suffix == "releases/tags/v6.5.0-rc.1":
            if not self.committed:
                return {"tag_name": "v6.5.0-rc.1", "draft": True}
            return {
                "id": 300,
                "tag_name": "v6.5.0-rc.1",
                "target_commitish": "c" * 40,
                "draft": False,
                "immutable": True,
                "published_at": "2026-09-02T00:00:00Z",
                "prerelease": True,
            }
        if suffix.startswith("actions/runs/"):
            run_id = int(suffix.rsplit("/", 1)[1])
            if run_id == self.owner_run_id:
                owner = self.run(self.owner_run_id, self.old_sha, conclusion="success")
                owner["status"] = "completed"
                return owner
            if run_id == self.source_run_id:
                return {
                    "id": self.source_run_id,
                    "workflow_id": 43,
                    "path": subject.CREATE_RELEASE_PATH,
                    "event": "workflow_dispatch",
                    "repository": {"full_name": self.repository},
                    "status": "in_progress",
                }
            return next(value for value in self.runs if value["id"] == run_id)
        raise AssertionError(endpoint)

    def pages(self, endpoint):
        self.assert_endpoint = endpoint
        return [{"workflow_runs": self.runs}]

    def download_marker(self, tag, directory):
        self.download = (tag, directory)
        return self.marker

    def post(self, endpoint, payload=None):
        self.posts.append((endpoint, payload))
        return True

    def unchanged_credential_containment_block(self, run_id):
        self.containment_probe = run_id
        return False


class JobLogTests(unittest.TestCase):
    def test_uses_captured_sanitised_job_reader_with_private_auth(self):
        github = subject.GitHub("rcourtman/Pulse", "gh", mutate=False)
        log = "Require credential containment\tcheck\tcredential containment gate: BLOCKED\n"
        with patch.object(subject.subprocess, "run", return_value=subprocess.CompletedProcess(
            [], 0, stdout=log, stderr=""
        )) as command, contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(log, github.job_log(
                subject.PRIVATE_REPOSITORY, 123, token="test-private-token"
            ))
        self.assertEqual("", output.getvalue())
        args, kwargs = command.call_args
        self.assertEqual(["gh", "run", "view", "--repo", subject.PRIVATE_REPOSITORY,
                          "--job", "123", "--log"], args[0])
        self.assertTrue(kwargs["capture_output"])
        self.assertEqual("test-private-token", kwargs["env"]["GH_TOKEN"])

    def test_unavailable_job_logs_fail_closed(self):
        github = subject.GitHub("rcourtman/Pulse", "gh", mutate=False)
        with patch.object(subject.subprocess, "run", return_value=subprocess.CompletedProcess(
            [], 1, stdout="", stderr="log unavailable"
        )):
            with self.assertRaisesRegex(subject.ReconciliationError, "log unavailable"):
                github.job_log(subject.PRIVATE_REPOSITORY, 123)


class CredentialContainmentTests(unittest.TestCase):
    def github(
        self,
        *,
        current_head=None,
        containment="failure",
        containment_log=subject.CREDENTIAL_BLOCK_MARKER,
        containment_state_changed=False,
        extra_public_job=None,
        verdict="failure",
    ):
        private_run_id = 700
        paid_job_id = 800
        private_head = "d" * 40
        current_head = current_head or private_head
        github = subject.GitHub(
            "rcourtman/Pulse", "gh", mutate=False, private_token="private-token"
        )

        def pages(endpoint, *, token=""):
            if endpoint == (
                "repos/rcourtman/Pulse/actions/runs/100/jobs?per_page=100"
            ):
                self.assertEqual("", token)
                return [
                    {
                        "jobs": [
                            {
                                "id": paid_job_id,
                                "name": subject.PAID_RUNTIME_JOB,
                                "status": "completed",
                                "conclusion": "failure",
                            },
                            {
                                "id": paid_job_id + 1,
                                "name": subject.CONVERGENCE_VERDICT_JOB,
                                "status": "completed",
                                "conclusion": verdict,
                            },
                            *([extra_public_job] if extra_public_job else []),
                        ]
                    }
                ]
            if endpoint == (
                f"repos/rcourtman/Pulse/check-runs/{paid_job_id}/annotations?per_page=100"
            ):
                self.assertEqual("", token)
                return [
                    [
                        {
                            "message": "private Pro live promotion failed: "
                            f"https://github.com/rcourtman/pulse-pro/actions/runs/{private_run_id}"
                        }
                    ]
                ]
            if endpoint == (
                f"repos/{subject.PRIVATE_REPOSITORY}/actions/runs/{private_run_id}/jobs?per_page=100"
            ):
                self.assertEqual("private-token", token)
                return [
                    {
                        "jobs": [
                            {
                                "id": 900,
                                "name": subject.CREDENTIAL_CONTAINMENT_JOB,
                                "conclusion": containment,
                            }
                        ]
                    }
                ]
            raise AssertionError(endpoint)

        def api(endpoint, *, token=""):
            self.assertEqual("private-token", token)
            if endpoint == f"repos/{subject.PRIVATE_REPOSITORY}/actions/runs/{private_run_id}":
                return {
                    "repository": {"full_name": subject.PRIVATE_REPOSITORY},
                    "path": subject.PRIVATE_PROMOTION_PATH,
                    "event": "workflow_dispatch",
                    "status": "completed",
                    "conclusion": "failure",
                    "head_sha": private_head,
                }
            if endpoint == f"repos/{subject.PRIVATE_REPOSITORY}/commits/main":
                return {"sha": current_head}
            content_prefix = f"repos/{subject.PRIVATE_REPOSITORY}/contents/"
            if endpoint.startswith(content_prefix):
                path, ref = endpoint.removeprefix(content_prefix).split("?ref=", 1)
                blob = "1" * 40 if path == subject.CREDENTIAL_CONTAINMENT_PATHS[0] else "2" * 40
                if containment_state_changed and ref == current_head:
                    blob = "3" * 40
                return {"type": "file", "sha": blob}
            raise AssertionError(endpoint)

        github.pages = pages
        github.api = api
        github.job_log = lambda repository, job_id, token="": (
            containment_log
            if (repository, job_id, token)
            == (subject.PRIVATE_REPOSITORY, 900, "private-token")
            else self.fail((repository, job_id, token))
        )
        return github

    def test_recognises_unchanged_private_credential_block(self):
        self.assertTrue(self.github().unchanged_credential_containment_block(100))

    def test_private_change_rearms_convergence(self):
        github = self.github(
            current_head="e" * 40, containment_state_changed=True
        )
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_unrelated_private_change_does_not_rearm_convergence(self):
        github = self.github(current_head="e" * 40)
        self.assertTrue(github.unchanged_credential_containment_block(100))

    def test_other_private_failure_remains_retriable(self):
        github = self.github(containment="success")
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_other_public_surface_failure_remains_retriable(self):
        github = self.github(
            extra_public_job={
                "id": 802,
                "name": "Converge Helm Pages / release",
                "status": "completed",
                "conclusion": "failure",
            }
        )
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_interrupted_public_job_remains_retriable(self):
        github = self.github(
            extra_public_job={
                "id": 802,
                "name": "Release global customer-promotion lease",
                "status": "completed",
                "conclusion": "cancelled",
            }
        )
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_missing_failed_aggregate_verdict_remains_retriable(self):
        github = self.github(verdict="success")
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_duplicate_failed_aggregate_verdict_remains_retriable(self):
        github = self.github(
            extra_public_job={
                "id": 802,
                "name": subject.CONVERGENCE_VERDICT_JOB,
                "status": "completed",
                "conclusion": "failure",
            }
        )
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_malformed_public_job_remains_retriable(self):
        github = self.github(
            extra_public_job={
                "id": 802,
                "status": "completed",
                "conclusion": "failure",
            }
        )
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_containment_job_error_without_block_marker_remains_retriable(self):
        github = self.github(containment_log="checkout failed")
        self.assertFalse(github.unchanged_credential_containment_block(100))

    def test_missing_private_token_cannot_weaken_retry(self):
        github = subject.GitHub("rcourtman/Pulse", "gh", mutate=False)
        github.pages = lambda endpoint: self.fail(endpoint)
        self.assertFalse(github.unchanged_credential_containment_block(100))


class ReconciliationTests(unittest.TestCase):
    def test_recovery_receipts_require_successful_submission(self):
        cases = [
            ({}, "Dispatched fresh convergence controls"),
            ({"current_controls": True}, "Re-ran current-control convergence"),
            ({"committed": False}, "Renewed pre-commit convergence owner"),
        ]
        for options, receipt in cases:
            for code in (0, 1):
                with self.subTest(options=options, code=code):
                    github = FakeGitHub(**options)
                    transport = subject.GitHub(github.repository, "gh")
                    github.post = transport.post
                    result = subprocess.CompletedProcess(
                        [], code, stdout="", stderr="rejected" if code else ""
                    )
                    output = io.StringIO()
                    with patch.object(subject.subprocess, "run", return_value=result) as command, contextlib.redirect_stdout(output):
                        if code:
                            with self.assertRaises(subject.ReconciliationError):
                                subject.reconcile(github, github.run_id, 5)
                        else:
                            subject.reconcile(github, github.run_id, 5)
                    command.assert_called_once()
                    if code:
                        self.assertNotIn(receipt, output.getvalue())
                    else:
                        self.assertIn(receipt, output.getvalue())
                    self.assertNotIn("DRY RUN", output.getvalue())

    def test_post_reports_submission_only_after_success(self):
        for payload in (None, {"ref": "main"}):
            for code in (0, 1):
                with self.subTest(payload=payload, code=code):
                    github = subject.GitHub("rcourtman/Pulse", "gh")
                    result = subprocess.CompletedProcess([], code, stdout="", stderr="failed" if code else "")
                    with patch.object(subject.subprocess, "run", return_value=result) as command:
                        if code:
                            with self.assertRaises(subject.ReconciliationError):
                                github.post("repos/rcourtman/Pulse/test", payload)
                        else:
                            self.assertIs(True, github.post("repos/rcourtman/Pulse/test", payload))
                    command.assert_called_once()

    def test_dry_run_receipts_never_claim_a_mutation(self):
        cases = [
            ({}, "Would dispatch", "Dispatched"),
            ({"current_controls": True}, "Would re-run", "Re-ran"),
            ({"committed": False}, "Would renew", "Renewed"),
        ]
        for options, expected, forbidden in cases:
            with self.subTest(options=options):
                github = FakeGitHub(**options)
                transport = subject.GitHub(github.repository, "gh", mutate=False)
                github.post = transport.post
                output = io.StringIO()
                with patch.object(subject.subprocess, "run") as command, contextlib.redirect_stdout(output):
                    subject.reconcile(github, github.run_id, 5)
                command.assert_not_called()
                self.assertIn("DRY RUN: " + expected, output.getvalue())
                self.assertNotIn(forbidden, output.getvalue())
                self.assertEqual([], github.posts)

    def test_stale_failed_run_dispatches_current_controls_with_bound_inputs(self):
        github = FakeGitHub()
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual(1, len(github.posts))
        endpoint, payload = github.posts[0]
        self.assertTrue(endpoint.endswith("release-convergence.yml/dispatches"))
        self.assertEqual("main", payload["ref"])
        self.assertEqual("v6.5.0-rc.1", payload["inputs"]["tag"])
        self.assertEqual("200", payload["inputs"]["source_release_run_id"])
        self.assertEqual("true", payload["inputs"]["prerelease"])

    def test_current_failed_run_uses_exact_rerun(self):
        github = FakeGitHub(current_controls=True)
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual(
            [(f"repos/{github.repository}/actions/runs/{github.run_id}/rerun", None)],
            github.posts,
        )

    def test_newer_sibling_prevents_duplicate_mutation(self):
        github = FakeGitHub()
        github.runs.append(github.run(101, github.main_sha, status="in_progress", conclusion=""))
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual([], github.posts)

    def test_exhausted_current_controls_are_a_stable_noop(self):
        github = FakeGitHub(current_controls=True)
        github.runs[0]["run_attempt"] = 3
        github.runs.append(github.run(101, github.old_sha, attempt=2))
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            subject.reconcile(github, 101, 5)
        self.assertEqual([], github.posts)
        self.assertIn(
            "remains failed after the 5-attempt retry budget", output.getvalue()
        )

    def test_repaired_controls_rearm_exhausted_stale_debt(self):
        github = FakeGitHub()
        github.runs[0]["run_attempt"] = 5
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual(1, len(github.posts))
        endpoint, payload = github.posts[0]
        self.assertTrue(endpoint.endswith("release-convergence.yml/dispatches"))
        self.assertEqual("main", payload["ref"])

    def test_attempt_budget_resets_for_repaired_controls(self):
        github = FakeGitHub()
        github.runs[0]["run_attempt"] = 5
        github.runs.append(github.run(101, github.main_sha))
        subject.reconcile(github, 101, 5)
        self.assertEqual(
            [(f"repos/{github.repository}/actions/runs/101/rerun", None)],
            github.posts,
        )

    def test_unchanged_credential_block_is_not_retried(self):
        github = FakeGitHub(current_controls=True)
        github.unchanged_credential_containment_block = lambda run_id: True
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual([], github.posts)

    def test_precommit_owner_is_renewed_without_using_committed_budget(self):
        github = FakeGitHub(committed=False)
        github.runs[0]["run_attempt"] = 6
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual(
            [(f"repos/{github.repository}/actions/runs/{github.run_id}/rerun", None)],
            github.posts,
        )


if __name__ == "__main__":
    unittest.main()
