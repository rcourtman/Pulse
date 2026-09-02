#!/usr/bin/env python3

from __future__ import annotations

import unittest

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


class ReconciliationTests(unittest.TestCase):
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

    def test_attempt_budget_counts_distinct_runs_and_reruns(self):
        github = FakeGitHub()
        github.runs[0]["run_attempt"] = 3
        github.runs.append(github.run(101, github.old_sha, attempt=2))
        with self.assertRaisesRegex(subject.ReconciliationError, "after 5 attempts"):
            subject.reconcile(github, 101, 5)
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
