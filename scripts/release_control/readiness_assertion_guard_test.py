import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import readiness_assertion_guard


def write_status(repo_root: Path, payload: dict) -> None:
    status_path = repo_root / "docs" / "release-control" / "v6" / "status.json"
    status_path.parent.mkdir(parents=True, exist_ok=True)
    status_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def base_payload() -> dict:
    return {
        "readiness_assertions": [
            {
                "id": "RA1",
                "blocking_level": "repo-ready",
                "proof_type": "automated",
                "proof_commands": [
                    {
                        "id": "pass",
                        "run": ["python3", "-c", "print('ok')"],
                    }
                ],
            }
        ]
    }


class ReadinessAssertionGuardTest(unittest.TestCase):
    def test_parse_args_accepts_filters_and_staged(self) -> None:
        args = readiness_assertion_guard.parse_args(
            ["--staged", "--blocking-level", "repo-ready", "--proof-type", "automated", "--assertion", "RA1"]
        )
        self.assertTrue(args.staged)
        self.assertEqual(args.blocking_level, ["repo-ready"])
        self.assertEqual(args.proof_type, ["automated"])
        self.assertEqual(args.assertion, ["RA1"])

    def test_selected_proof_commands_filters_by_blocking_level_and_proof_type(self) -> None:
        payload = {
            "readiness_assertions": [
                {
                    "id": "RA1",
                    "blocking_level": "repo-ready",
                    "proof_type": "automated",
                    "proof_commands": [{"id": "a", "run": ["python3", "-c", "print('a')"]}],
                },
                {
                    "id": "RA2",
                    "blocking_level": "release-ready",
                    "proof_type": "hybrid",
                    "proof_commands": [{"id": "b", "run": ["python3", "-c", "print('b')"]}],
                },
            ]
        }

        commands, errors = readiness_assertion_guard.selected_proof_commands(
            payload,
            blocking_levels={"repo-ready"},
            proof_types={"automated"},
        )

        self.assertEqual(errors, [])
        self.assertEqual(len(commands), 1)
        self.assertEqual(commands[0]["assertion_id"], "RA1")
        self.assertEqual(commands[0]["command_id"], "a")

    def test_selected_proof_commands_blocks_automated_assertion_without_commands(self) -> None:
        commands, errors = readiness_assertion_guard.selected_proof_commands(
            {
                "readiness_assertions": [
                    {
                        "id": "RA1",
                        "blocking_level": "repo-ready",
                        "proof_type": "automated",
                    }
                ]
            },
            blocking_levels={"repo-ready"},
            proof_types={"automated"},
        )

        self.assertEqual(commands, [])
        self.assertEqual(errors, ["RA1 is automated but has no proof_commands"])

    def test_main_can_read_staged_status_payload(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            subprocess.run(["git", "init"], cwd=repo_root, check=True, capture_output=True, text=True)

            payload = base_payload()
            payload["readiness_assertions"][0]["proof_commands"][0]["run"] = [
                "python3",
                "-c",
                "from pathlib import Path; Path('proof.out').write_text('ok', encoding='utf-8')",
            ]
            write_status(repo_root, payload)
            subprocess.run(
                ["git", "add", "docs/release-control/v6/status.json"],
                cwd=repo_root,
                check=True,
                capture_output=True,
                text=True,
            )

            payload["readiness_assertions"][0]["proof_commands"][0]["run"] = ["python3", "-c", "raise SystemExit(1)"]
            write_status(repo_root, payload)

            with mock.patch.object(readiness_assertion_guard, "REPO_ROOT", repo_root), mock.patch(
                "repo_file_io.REPO_ROOT", repo_root
            ):
                exit_code = readiness_assertion_guard.main(
                    ["--staged", "--blocking-level", "repo-ready", "--proof-type", "automated"]
                )

            self.assertEqual(exit_code, 0)
            self.assertEqual((repo_root / "proof.out").read_text(encoding="utf-8"), "ok")


if __name__ == "__main__":
    unittest.main()
