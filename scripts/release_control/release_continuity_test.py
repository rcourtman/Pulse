#!/usr/bin/env python3

from __future__ import annotations

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


def valid_release() -> dict[str, object]:
    return {
        "id": 12345,
        "tag_name": "v6.4.2",
        "target_commitish": SOURCE_SHA,
        "draft": False,
        "prerelease": False,
        "immutable": True,
        "published_at": "2026-08-31T17:00:00Z",
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
                "--github-output",
                str(output),
            ]
            if command == "activation":
                activation_path = root / "activation.json"
                activation_path.write_text(json.dumps(activation), encoding="utf-8")
                args.extend(["--activation-json", str(activation_path)])
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
        result, diagnostic, output = self.run_command(
            "activation", valid_release(), valid_activation()
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
            "activation", valid_release(), activation
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
        release = valid_release()
        release["immutable"] = False
        activation = valid_activation()
        del activation["server_image_digest"]
        del activation["control_plane_image_digest"]
        del activation["helm_chart_digest"]

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


if __name__ == "__main__":
    unittest.main()
