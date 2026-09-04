#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "release_control" / "release_continuity.py"
SOURCE_SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64


def encoded(payload: object) -> bytes:
    return json.dumps(payload).encode("utf-8")


def activation_asset(payload: object) -> dict[str, object]:
    content = encoded(payload)
    return {
        "name": "release-activation.json",
        "state": "uploaded",
        "size": len(content),
        "digest": "sha256:" + hashlib.sha256(content).hexdigest(),
    }


def valid_release(activation: object | None = None) -> dict[str, object]:
    if activation is None:
        activation = valid_activation()
    return {
        "id": 12345,
        "tag_name": "v6.4.2",
        "target_commitish": SOURCE_SHA,
        "draft": False,
        "prerelease": False,
        "immutable": True,
        "published_at": "2026-08-31T17:00:00Z",
        "assets": [activation_asset(activation)],
    }


def valid_activation() -> dict[str, object]:
    return {
        "schema_version": 1,
        "tag": "v6.4.2",
        "release_id": "12345",
        "target_commitish": SOURCE_SHA,
        "source_release_run_id": "1001",
        "convergence_run_id": "1002",
        "r2_prefix": "releases/v6.4.2",
        "server_image_digest": DIGEST,
        "control_plane_image_digest": DIGEST,
        "helm_chart_digest": DIGEST,
    }


class ReleaseContinuityTest(unittest.TestCase):
    def run_command(
        self,
        command: str,
        release: object,
        activation: object | None = None,
        stable_refs: object | None = None,
        releases: object | None = None,
        registries: list[object] | None = None,
    ) -> tuple[subprocess.CompletedProcess[str], dict[str, object], str]:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            release_path = root / "release.json"
            release_path.write_text(json.dumps(release), encoding="utf-8")
            diagnostic = root / "diagnostic.json"
            output = root / "github-output"
            output.touch()
            args = [
                sys.executable,
                str(SCRIPT),
                command,
                "--release-json",
                str(release_path),
                "--diagnostic",
                str(diagnostic),
            ]
            if command != "frontier":
                args.extend(["--github-output", str(output)])
            if command == "activation":
                activation_path = root / "activation.json"
                activation_path.write_bytes(encoded(activation))
                args.extend(["--activation-json", str(activation_path)])
            if command == "frontier":
                stable_refs_path = root / "stable-refs.json"
                releases_path = root / "releases.json"
                stable_refs_path.write_text(json.dumps(stable_refs), encoding="utf-8")
                releases_path.write_text(json.dumps(releases), encoding="utf-8")
                args.extend(
                    [
                        "--stable-refs-json",
                        str(stable_refs_path),
                        "--releases-json",
                        str(releases_path),
                    ]
                )
                for index, registry in enumerate(
                    registries or [{"name": "registry.example/pulse", "tags": []}]
                ):
                    registry_path = root / f"registry-{index}.json"
                    registry_path.write_text(json.dumps(registry), encoding="utf-8")
                    args.extend(["--registry-tags-json", str(registry_path)])
            result = subprocess.run(
                args,
                cwd=ROOT,
                text=True,
                capture_output=True,
                check=False,
            )
            return (
                result,
                json.loads(diagnostic.read_text(encoding="utf-8")),
                output.read_text(encoding="utf-8"),
            )

    def test_accepts_exact_immutable_stable_release(self) -> None:
        result, diagnostic, output = self.run_command("release", valid_release())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(diagnostic["status"], "success")
        self.assertEqual(diagnostic["violations"], [])
        self.assertIn("referenceable=true\n", output)
        self.assertIn("tag=v6.4.2\n", output)
        self.assertIn("release_id=12345\n", output)
        self.assertIn(f"source_sha={SOURCE_SHA}\n", output)

    def test_mutable_release_fails_with_one_actionable_reason(self) -> None:
        release = valid_release()
        release["immutable"] = False
        result, diagnostic, output = self.run_command("release", release)
        self.assertEqual(result.returncode, 1)
        self.assertIn("referenceable=true\n", output)
        self.assertIn("tag=v6.4.2\n", output)
        self.assertIn("release_id=12345\n", output)
        self.assertIn(f"source_sha={SOURCE_SHA}\n", output)
        self.assertEqual(diagnostic["status"], "failure")
        self.assertEqual(
            [item["code"] for item in diagnostic["violations"]],
            ["release_mutable"],
        )
        self.assertIn("immutable=false", result.stderr)
        self.assertIn("never repair the packet in place", result.stderr)

    def test_reports_every_release_identity_violation(self) -> None:
        result, diagnostic, output = self.run_command(
            "release",
            {
                "id": True,
                "tag_name": "v6.4.2-rc.1\nforged",
                "target_commitish": "main",
                "draft": True,
                "prerelease": True,
                "immutable": None,
                "published_at": "",
            },
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(output, "")
        self.assertEqual(
            {item["code"] for item in diagnostic["violations"]},
            {
                "release_id_invalid",
                "stable_tag_invalid",
                "source_identity_invalid",
                "release_is_draft",
                "release_is_prerelease",
                "release_mutable",
                "publication_time_invalid",
            },
        )
        self.assertNotIn("forged", result.stderr)

    def test_accepts_exact_activation_binding_and_emits_digests(self) -> None:
        activation = valid_activation()
        result, diagnostic, output = self.run_command(
            "activation", valid_release(activation), activation
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(diagnostic["status"], "success")
        self.assertRegex(output, r"activation_sha256=[0-9a-f]{64}\n")
        self.assertIn(f"server_image_digest={DIGEST}\n", output)
        self.assertIn(f"control_plane_image_digest={DIGEST}\n", output)
        self.assertIn(f"helm_chart_digest={DIGEST}\n", output)

    def test_activation_mismatches_are_classified_without_outputs(self) -> None:
        activation = valid_activation()
        activation.update(
            {
                "schema_version": True,
                "tag": "v6.4.1",
                "release_id": 12345,
                "target_commitish": "c" * 40,
                "source_release_run_id": "",
                "convergence_run_id": 1002,
                "r2_prefix": "",
                "server_image_digest": "latest",
                "control_plane_image_digest": None,
                "helm_chart_digest": "sha256:ABC",
            }
        )
        result, diagnostic, output = self.run_command(
            "activation", valid_release(activation), activation
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(output, "")
        self.assertEqual(
            {item["code"] for item in diagnostic["violations"]},
            {
                "activation_schema_invalid",
                "activation_tag_mismatch",
                "activation_release_mismatch",
                "activation_source_mismatch",
                "source_run_invalid",
                "convergence_run_invalid",
                "delivery_prefix_invalid",
                "server_digest_invalid",
                "control_plane_digest_invalid",
                "helm_digest_invalid",
            },
        )
        self.assertTrue(
            all(
                "publish a corrected replacement" in item["action"]
                for item in diagnostic["violations"]
            )
        )

    def test_mutable_release_does_not_hide_activation_damage(self) -> None:
        activation = valid_activation()
        del activation["server_image_digest"]
        del activation["control_plane_image_digest"]
        del activation["helm_chart_digest"]
        release = valid_release(activation)
        release["immutable"] = False

        result, diagnostic, output = self.run_command(
            "activation", release, activation
        )

        self.assertEqual(result.returncode, 1)
        self.assertEqual(output, "")
        self.assertEqual(
            [item["code"] for item in diagnostic["violations"]],
            [
                "server_digest_invalid",
                "control_plane_digest_invalid",
                "helm_digest_invalid",
            ],
        )

    def test_activation_requires_one_digest_bound_release_asset(self) -> None:
        activation = valid_activation()
        release = valid_release(activation)
        release["assets"] = []

        result, diagnostic, output = self.run_command(
            "activation", release, activation
        )

        self.assertEqual(result.returncode, 1)
        self.assertEqual(output, "")
        self.assertEqual(
            [item["code"] for item in diagnostic["violations"]],
            ["activation_asset_invalid"],
        )

    def test_activation_rejects_download_that_differs_from_release_metadata(self) -> None:
        expected = valid_activation()
        downloaded = valid_activation()
        downloaded["r2_prefix"] = "releases/v6.4.2-corrupted"

        result, diagnostic, output = self.run_command(
            "activation", valid_release(expected), downloaded
        )

        self.assertEqual(result.returncode, 1)
        self.assertEqual(output, "")
        self.assertEqual(
            [item["code"] for item in diagnostic["violations"]],
            [
                "activation_asset_size_mismatch",
                "activation_asset_digest_mismatch",
            ],
        )

    def test_frontier_accepts_tags_at_or_behind_advertised_release(self) -> None:
        release = valid_release()
        result, diagnostic, output = self.run_command(
            "frontier",
            release,
            stable_refs=[
                {"ref": "refs/tags/v6.4.1"},
                {"ref": "refs/tags/v6.4.2"},
                {"ref": "refs/tags/v6.4.3-rc.1"},
            ],
            releases=[release],
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(diagnostic["status"], "success")
        self.assertEqual(diagnostic["identity"]["newer_stable_tags"], "")
        self.assertEqual(output, "")

    def test_frontier_rejects_newer_stable_tag_without_release_packet(self) -> None:
        release = valid_release()
        result, diagnostic, _ = self.run_command(
            "frontier",
            release,
            stable_refs=[[{"ref": "refs/tags/v6.4.2"}, {"ref": "refs/tags/v6.4.3"}]],
            releases=[[release]],
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            [item["code"] for item in diagnostic["violations"]],
            ["stable_tag_without_release"],
        )
        self.assertEqual(diagnostic["identity"]["orphaned_stable_tags"], "v6.4.3")
        self.assertIn("orphaned version", result.stderr)

    def test_frontier_rejects_public_registry_version_beyond_latest(self) -> None:
        release = valid_release()
        result, diagnostic, _ = self.run_command(
            "frontier",
            release,
            stable_refs=[{"ref": "refs/tags/v6.4.2"}],
            releases=[release],
            registries=[
                {
                    "name": "docker.io/rcourtman/pulse",
                    "tags": ["latest", "6.4", "6.4.2", "v6.4.3"],
                },
                {
                    "name": "ghcr.io/rcourtman/pulse",
                    "tags": ["v6.4.3", "6.4.3-rc.1"],
                },
            ],
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            [item["code"] for item in diagnostic["violations"]],
            ["registry_stable_tag_beyond_latest"],
        )
        self.assertEqual(
            diagnostic["identity"]["registry_stable_tags_beyond_latest"],
            "v6.4.3",
        )
        self.assertIn("docker.io/rcourtman/pulse", diagnostic["violations"][0]["actual"])
        self.assertIn("ghcr.io/rcourtman/pulse", diagnostic["violations"][0]["actual"])

    def test_frontier_rejects_published_stable_release_beyond_latest(self) -> None:
        release = valid_release()
        newer = {**release, "id": 67890, "tag_name": "v6.4.3"}
        result, diagnostic, _ = self.run_command(
            "frontier",
            release,
            stable_refs=[{"ref": "refs/tags/v6.4.3"}],
            releases=[release, newer],
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            [item["code"] for item in diagnostic["violations"]],
            ["newer_stable_release_not_advertised"],
        )

    def test_frontier_allows_in_progress_draft_beyond_latest(self) -> None:
        release = valid_release()
        draft = {
            **release,
            "id": 67890,
            "tag_name": "v6.4.3",
            "draft": True,
            "published_at": None,
        }
        result, diagnostic, _ = self.run_command(
            "frontier",
            release,
            stable_refs=[{"ref": "refs/tags/v6.4.3"}],
            releases=[release, draft],
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(diagnostic["status"], "success")


if __name__ == "__main__":
    unittest.main()
