#!/usr/bin/env python3

from __future__ import annotations

import argparse
import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import subprocess
import sys

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


class TransientReadTests(unittest.TestCase):
    def result(self, code=0, stdout="{}", stderr=""):
        return subprocess.CompletedProcess([], code, stdout=stdout, stderr=stderr)

    def test_pages_discards_partial_output_before_retry(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        results = [
            self.result(1, '[{"id": 1}]', "gh: gateway timeout (HTTP 504)"),
            self.result(0, '[{"id": 1}]\n[{"id": 2}]'),
        ]
        with patch.object(subject.subprocess, "run", side_effect=results) as command, patch("time.sleep") as sleep:
            self.assertEqual([[{"id": 1}], [{"id": 2}]], github.pages("repos/example/releases"))
        self.assertEqual(2, command.call_count)
        sleep.assert_called_once_with(1)

    def test_api_retries_transient_errors_with_bounded_backoff(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        for status in (502, 503, 504):
            with self.subTest(status=status):
                failure = self.result(1, stderr=f"gh: unavailable (HTTP {status})")
                with patch.object(subject.subprocess, "run", side_effect=[failure, failure, self.result()]) as command, patch("time.sleep") as sleep:
                    self.assertEqual({}, github.api("repos/example"))
                self.assertEqual(3, command.call_count)
                self.assertEqual([1, 2], [call.args[0] for call in sleep.call_args_list])

    def test_exhausted_reads_fail_closed(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        with patch.object(subject.subprocess, "run", return_value=self.result(1, stderr="gh: timeout (HTTP 504)")) as command, patch("time.sleep") as sleep:
            with self.assertRaisesRegex(subject.ReconciliationError, "HTTP 504"):
                github.pages("repos/example/releases")
        self.assertEqual(3, command.call_count)
        self.assertEqual(2, sleep.call_count)

    def test_nontransient_errors_are_not_retried(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        for detail in ("HTTP 401", "HTTP 403", "HTTP 404", "HTTP 429", "terminal escape sequences refused"):
            with self.subTest(detail=detail):
                with patch.object(subject.subprocess, "run", return_value=self.result(1, stderr=detail)) as command, patch("time.sleep") as sleep:
                    error = subject.GitHubNotFound if detail == "HTTP 404" else subject.ReconciliationError
                    with self.assertRaises(error):
                        github.api("repos/example")
                self.assertEqual(1, command.call_count)
                sleep.assert_not_called()

    def test_mutations_are_not_retried(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        for payload in (None, {"ref": "main"}):
            with self.subTest(payload=payload):
                with patch.object(subject.subprocess, "run", return_value=self.result(1, stderr="HTTP 504")) as command, patch("time.sleep") as sleep:
                    with self.assertRaises(subject.ReconciliationError):
                        github.post("repos/example/actions/dispatches", payload)
                self.assertEqual(1, command.call_count)
                sleep.assert_not_called()


class FailedReadIdentityTests(unittest.TestCase):
    def test_header_bearing_api_failure_names_the_resource_without_retry(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        endpoint = "repos/rcourtman/Pulse/actions/runs/37395942598"
        for status in (401, 403, 404):
            with self.subTest(status=status):
                result = subprocess.CompletedProcess([], 1, stdout="", stderr=f"HTTP {status}")
                with patch.object(subject.subprocess, "run", return_value=result) as command, patch("time.sleep") as sleep:
                    with self.assertRaises(subject.ReconciliationError) as failure:
                        github.api(endpoint)
                self.assertIn(f"GET {endpoint}", str(failure.exception))
                self.assertNotIn("Accept:", str(failure.exception))
                if status == 404:
                    self.assertIsInstance(failure.exception, subject.GitHubNotFound)
                self.assertEqual(1, command.call_count)
                sleep.assert_not_called()

    def test_paginated_failure_omits_read_filters(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        resource = "repos/rcourtman/Pulse/actions/workflows/release-convergence.yml/runs"
        result = subprocess.CompletedProcess([], 1, stdout="partial", stderr="HTTP 403")
        with patch.object(subject.subprocess, "run", return_value=result) as command, patch("time.sleep") as sleep:
            with self.assertRaises(subject.ReconciliationError) as failure:
                github.pages(resource + "?event=workflow_dispatch&private_filter=fixture-only")
        self.assertIn(f"GET {resource}", str(failure.exception))
        self.assertNotIn("private_filter", str(failure.exception))
        self.assertNotIn("fixture-only", str(failure.exception))
        self.assertEqual(1, command.call_count)
        sleep.assert_not_called()

    def test_unrecognised_resource_has_no_endpoint_disclosure(self):
        for endpoint in (
            "repos/another/private-repository/customer-fixture",
            "repos/rcourtman/Pulse/../customer-fixture",
            "repos/rcourtman/Pulse/" + "x" * 257,
            "repos/rcourtman/Pulse/customer-fixture\nforged diagnostic",
        ):
            with self.subTest(endpoint=endpoint):
                self.assertEqual("GitHub API read", subject.api_read_subject(endpoint))

    def test_discovery_stops_at_the_identified_failed_read_without_mutation(self):
        github = subject.GitHub("rcourtman/Pulse", "gh")
        results = [
            subprocess.CompletedProcess([], 0, stdout='{"default_branch": "main"}', stderr=""),
            subprocess.CompletedProcess([], 1, stdout="", stderr="HTTP 404"),
        ]
        with patch.object(subject.subprocess, "run", side_effect=results) as command, patch("time.sleep") as sleep:
            with self.assertRaisesRegex(subject.GitHubNotFound, "GET repos/rcourtman/Pulse/actions/workflows/release-convergence.yml"):
                subject.discover(github)
        self.assertEqual(2, command.call_count)
        self.assertTrue(all("POST" not in call.args[0] for call in command.call_args_list))
        sleep.assert_not_called()


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


class DecisionSummaryTests(unittest.TestCase):
    def test_local_execution_needs_no_summary_file(self):
        with patch.dict(os.environ, {}, clear=True), contextlib.redirect_stdout(io.StringIO()) as output:
            subject.report_decision("No action.")
        self.assertEqual("No action.\n", output.getvalue())

    def test_summary_appends_escaped_decisions_without_claiming_delivery(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "summary.md"
            path.write_text("Existing summary\n", encoding="utf-8")
            with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(path)}):
                subject.report_decision("held <not delivered>")
                subject.report_decision("second decision")
            summary = path.read_text(encoding="utf-8")
        self.assertTrue(summary.startswith("Existing summary\n"))
        self.assertIn("held &lt;not delivered&gt;", summary)
        self.assertIn("second decision", summary)
        self.assertIn("not evidence of successful release convergence", summary)

    def test_empty_discovery_does_not_claim_absent_debt(self):
        args = argparse.Namespace(repository="rcourtman/Pulse", run_id=None,
                                  latest=True, max_attempts=5, dry_run=True)
        with patch.object(subject, "parse_args", return_value=args), \
                patch.object(subject, "discover", return_value=[]), \
                patch.object(subject, "report_decision") as report:
            self.assertEqual(0, subject.main())
        message = report.call_args.args[0]
        self.assertIn("No failed run selected for retry", message)
        self.assertIn("missing convergence runs are not qualified", message)

    def discover_with_releases(self, releases):
        github = FakeGitHub()
        github.pages = lambda endpoint: (
            [releases] if "/releases?" in endpoint
            else [{"workflow_runs": github.runs}]
        )
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "summary.md"
            with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(path)}), \
                    contextlib.redirect_stdout(io.StringIO()) as output:
                selected = subject.discover(github)
            summary = path.read_text() if path.exists() else ""
        self.assertEqual([], github.posts)
        return selected, output.getvalue(), summary

    def test_mutable_stable_debt_visible_even_when_preview_retry_selected(self):
        selected, output, summary = self.discover_with_releases([
            release("v6.4.1", "2026-09-01T00:00:00Z",
                    prerelease=False, immutable=False),
            release("v6.5.0-rc.1", "2026-09-02T00:00:00Z", prerelease=True),
        ])
        self.assertEqual([100], selected)
        for text in (output, summary):
            self.assertIn("Current stable head v6.4.1", text)
            self.assertIn("continuity debt remains", text)
            self.assertIn("No retry or fallback", text)

    def test_unknown_immutability_reports_debt_without_older_fallback(self):
        for unknown in (None, "true", 1, False):
            with self.subTest(immutable=unknown):
                head = release("v6.5.0-rc.2", "2026-09-03T00:00:00Z",
                               prerelease=True)
                head["immutable"] = unknown
                selected, output, summary = self.discover_with_releases([
                    release("v6.5.0-rc.1", "2026-09-02T00:00:00Z", prerelease=True),
                    head,
                ])
                self.assertEqual([], selected)
                self.assertIn("Current preview head v6.5.0-rc.2", output)
                self.assertIn("no confirmed immutable activation commit", summary)

    def test_old_mutable_draft_and_companion_do_not_report_current_debt(self):
        draft = release("v6.6.0", "2026-09-04T00:00:00Z",
                        prerelease=False, immutable=False)
        draft["draft"] = True
        selected, output, summary = self.discover_with_releases([
            release("v6.4.0-rc.1", "2026-09-01T00:00:00Z",
                    prerelease=True, immutable=False),
            release("v6.5.0-rc.1", "2026-09-02T00:00:00Z", prerelease=True),
            release("helm-chart-6.5.0", "2026-09-03T00:00:00Z",
                    prerelease=False, immutable=False),
            draft,
        ])
        self.assertEqual([100], selected)
        self.assertEqual("", output)
        self.assertEqual("", summary)

    def test_credential_hold_is_visible_without_retry(self):
        github = FakeGitHub(current_controls=True)
        github.unchanged_credential_containment_block = lambda run_id: True
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "summary.md"
            with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(path)}):
                subject.reconcile(github, github.run_id, 5)
            summary = path.read_text(encoding="utf-8")
        self.assertEqual([], github.posts)
        self.assertIn("held by unchanged private credential containment", summary)
        self.assertIn("no unattended retry was dispatched", summary)


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

    def missing_owner(self, github):
        original = github.api
        def api(endpoint):
            if endpoint == f"repos/{github.repository}/actions/runs/{github.owner_run_id}":
                raise subject.GitHubNotFound("gh: Not Found (HTTP 404)")
            return original(endpoint)
        github.api = api
        return github

    def test_deleted_activation_owner_can_reach_attested_workflow_not_fake_completion(self):
        github = self.missing_owner(FakeGitHub())
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            subject.reconcile(github, github.run_id, 5)
        self.assertEqual(1, len(github.posts))
        self.assertEqual(
            (f"repos/{github.repository}/actions/workflows/release-convergence.yml/dispatches", {
                "ref": "main", "inputs": {
                    "tag": "v6.5.0-rc.1", "version": "6.5.0-rc.1", "prerelease": "true",
                    "target_commitish": "c" * 40, "release_id": "300", "r2_prefix": "packet",
                    "source_release_run_id": "200",
                },
            }), github.posts[0],
        )
        self.assertIn("unavailable Actions history (404), not confirmed completion", output.getvalue())
        self.assertIn("exact immutable attestation and the existing global lease", output.getvalue())

    def test_missing_owner_preserves_current_control_retry_budget(self):
        github = self.missing_owner(FakeGitHub(current_controls=True))
        github.runs[0]["run_attempt"] = 5
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual([], github.posts)

    def test_missing_owner_does_not_override_credential_containment_or_marker_identity(self):
        held = self.missing_owner(FakeGitHub())
        held.unchanged_credential_containment_block = lambda run_id: True
        subject.reconcile(held, held.run_id, 5)
        self.assertEqual([], held.posts)
        corrupt = self.missing_owner(FakeGitHub())
        corrupt.marker["target_commitish"] = "f" * 40
        with self.assertRaisesRegex(subject.ReconciliationError, "source commit"):
            subject.reconcile(corrupt, corrupt.run_id, 5)
        self.assertEqual([], corrupt.posts)

    def test_unavailable_or_refused_owner_read_does_not_dispatch(self):
        for detail in ("HTTP 401", "HTTP 403", "HTTP 429", "HTTP 504", "transport refused"):
            with self.subTest(detail=detail):
                github = FakeGitHub()
                original = github.api
                def api(endpoint):
                    if endpoint == f"repos/{github.repository}/actions/runs/{github.owner_run_id}":
                        raise subject.ReconciliationError(detail)
                    return original(endpoint)
                github.api = api
                with self.assertRaisesRegex(subject.ReconciliationError, detail):
                    subject.reconcile(github, github.run_id, 5)
                self.assertEqual([], github.posts)

    def test_one_channel_failure_does_not_starve_the_other(self):
        args = argparse.Namespace(repository="rcourtman/Pulse", run_id=None,
                                  latest=True, max_attempts=5, dry_run=True)
        attempted = []

        def reconcile(github, run_id, max_attempts):
            attempted.append(run_id)
            if run_id == 1:
                raise subject.ReconciliationError("stable evidence is gone")

        stderr = io.StringIO()
        with patch.object(subject, "parse_args", return_value=args), \
                patch.object(subject, "discover", return_value=[1, 2]), \
                patch.object(subject, "reconcile", side_effect=reconcile), \
                contextlib.redirect_stderr(stderr):
            self.assertEqual(1, subject.main())
        self.assertEqual([1, 2], attempted)
        self.assertIn("failed for run 1: stable evidence is gone", stderr.getvalue())

    def test_precommit_owner_is_renewed_without_using_committed_budget(self):
        github = FakeGitHub(committed=False)
        github.runs[0]["run_attempt"] = 6
        subject.reconcile(github, github.run_id, 5)
        self.assertEqual(
            [(f"repos/{github.repository}/actions/runs/{github.run_id}/rerun", None)],
            github.posts,
        )


class ActivationOwnerObservationTests(unittest.TestCase):
    def state(self, github):
        return subject.activation_owner_state(
            github, github.owner_run_id, workflow_id=42, display_title=github.title
        )

    def test_present_owner_identity_and_status_are_required(self):
        for status in ("completed", "queued", "in_progress", "waiting", "pending", "requested", None, "unknown", ["completed"]):
            with self.subTest(status=status):
                github = FakeGitHub()
                original = github.api
                def api(endpoint):
                    value = original(endpoint)
                    value["status"] = status
                    return value
                github.api = api
                if not isinstance(status, str) or status == "unknown":
                    with self.assertRaisesRegex(subject.ReconciliationError, "unknown"):
                        self.state(github)
                else:
                    self.assertEqual("completed" if status == "completed" else "active", self.state(github))
                self.assertEqual([], github.posts)

    def test_mismatched_present_identity_never_becomes_missing_history(self):
        for key, value in (
            ("id", 91), ("repository", {"full_name": "other/repo"}),
            ("workflow_id", 43), ("path", subject.CREATE_RELEASE_PATH),
            ("event", "push"), ("display_title", "Release convergence v6.5.0 source 201"),
        ):
            with self.subTest(key=key):
                github = FakeGitHub()
                original = github.api
                def api(endpoint):
                    result = original(endpoint)
                    result[key] = value
                    return result
                github.api = api
                with self.assertRaises(subject.ReconciliationError):
                    self.state(github)
                self.assertEqual([], github.posts)

    def test_missing_status_observation_is_read_only_at_real_cli(self):
        # Execute the same CLI consumed after attestation by the real workflow.
        # The closed gh fixture allows exactly two GETs, never a provider call.
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            gh = root / "gh"
            record = root / "calls.jsonl"
            reply = root / "reply.json"
            gh.write_text("""#!/usr/bin/env python3
import json, os, pathlib, sys
p = pathlib.Path(os.environ["OWNER_FIXTURE"])
x = json.loads(p.read_text())
with pathlib.Path(os.environ["OWNER_CALLS"]).open("a") as f:
    f.write(json.dumps(sys.argv[1:]) + "\\n")
assert sys.argv[1] == "api" and "--method" not in sys.argv
endpoint = sys.argv[-1]
if endpoint == "repos/rcourtman/Pulse/actions/workflows/release-convergence.yml":
    if x.get("workflow_missing"):
        print("gh: Not Found (HTTP 404)", file=sys.stderr); sys.exit(1)
    print('{"id":42}')
elif endpoint == "repos/rcourtman/Pulse/actions/runs/90":
    if "error" in x:
        print(x["error"], file=sys.stderr); sys.exit(1)
    print(x["raw"] if "raw" in x else json.dumps(x["run"]))
else:
    raise AssertionError(endpoint)
""")
            gh.chmod(0o700)
            github = FakeGitHub()
            owner = github.api(f"repos/{github.repository}/actions/runs/{github.owner_run_id}")
            command = [sys.executable, str(Path(subject.__file__)), "--repository", github.repository,
                       "--activation-owner", "90", "--tag", "v6.5.0-rc.1", "--source-run-id", "200"]
            cases = [
                ({"run": owner}, 0, "completed\n"),
                ({"run": {**owner, "status": "in_progress"}}, 0, "active\n"),
                ({"error": "gh: Not Found (HTTP 404)"}, 0, "history-missing\n"),
                ({"error": "gh: Forbidden (HTTP 403)"}, 1, ""),
                ({"error": "gh: unauthorised (HTTP 401)"}, 1, ""),
                ({"error": "gh: limit (HTTP 429)"}, 1, ""),
                ({"error": "transport refused"}, 1, ""),
                ({"error": "gh: unexpected HTTP 4040"}, 1, ""),
                ({"error": "gh: HTTP 404 then refused (HTTP 403)"}, 1, ""),
                ({"raw": "truncated{"}, 1, ""),
                ({"raw": "[]"}, 1, ""),
                ({"run": {**owner, "status": "unknown"}}, 1, ""),
                ({"run": {**owner, "id": 91}}, 1, ""),
                ({"run": {**owner, "workflow_id": 43}}, 1, ""),
                ({"workflow_missing": True}, 1, ""),
            ]
            env = {**os.environ, "GH_BIN": str(gh), "OWNER_FIXTURE": str(reply), "OWNER_CALLS": str(record)}
            for case, code, output in cases:
                with self.subTest(case=case):
                    reply.write_text(json.dumps(case))
                    record.write_text("")
                    result = subprocess.run(command, env=env, text=True, capture_output=True, timeout=10)
                    self.assertEqual(code, result.returncode, result.stderr)
                    self.assertEqual(output, result.stdout)
                    calls = [json.loads(line) for line in record.read_text().splitlines()]
                    self.assertEqual(1 if case.get("workflow_missing") else 2, len(calls))
                    self.assertTrue(all(c[0] == "api" and "--method" not in c for c in calls))
                    if code:
                        self.assertIn("observation failed", result.stderr)
            for extra in (["--activation-owner", "0"], ["--tag", "v6.5.0-rc.1;unsafe"], ["--source-run-id", "0"]):
                with self.subTest(extra=extra):
                    record.write_text("")
                    result = subprocess.run(command + extra, env=env, text=True, capture_output=True, timeout=10)
                    self.assertEqual(2, result.returncode)
                    self.assertEqual("", record.read_text())


if __name__ == "__main__":
    unittest.main()
