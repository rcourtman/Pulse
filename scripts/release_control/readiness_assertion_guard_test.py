import json
import os
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
    def git(self, repo_root: Path, *args: str) -> subprocess.CompletedProcess:
        env = os.environ.copy()
        env.pop("GIT_INDEX_FILE", None)
        return subprocess.run(
            ["git", *args],
            cwd=repo_root,
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )

    def test_parse_args_accepts_filters_and_staged(self) -> None:
        args = readiness_assertion_guard.parse_args(
            [
                "--staged",
                "--active-target",
                "--blocking-level",
                "rc-ready",
                "--proof-type",
                "automated",
                "--assertion",
                "RA1",
            ]
        )
        self.assertTrue(args.staged)
        self.assertTrue(args.active_target)
        self.assertEqual(args.blocking_level, ["rc-ready"])
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
                    "blocking_level": "rc-ready",
                    "proof_type": "hybrid",
                    "proof_commands": [{"id": "b", "run": ["python3", "-c", "print('b')"]}],
                },
                {
                    "id": "RA3",
                    "blocking_level": "release-ready",
                    "proof_type": "hybrid",
                    "proof_commands": [{"id": "c", "run": ["python3", "-c", "print('c')"]}],
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

    def test_main_active_target_uses_control_plane_phase(self) -> None:
        payload = {
            "readiness_assertions": [
                {
                    "id": "RA1",
                    "blocking_level": "repo-ready",
                    "proof_type": "automated",
                    "proof_commands": [{"id": "repo", "run": ["python3", "-c", "print('repo')"]}],
                },
                {
                    "id": "RA2",
                    "blocking_level": "rc-ready",
                    "proof_type": "hybrid",
                    "proof_commands": [{"id": "rc", "run": ["python3", "-c", "print('rc')"]}],
                },
                {
                    "id": "RA8",
                    "blocking_level": "release-ready",
                    "proof_type": "hybrid",
                    "proof_commands": [{"id": "ga", "run": ["python3", "-c", "print('ga')"]}],
                },
            ]
        }

        with mock.patch.object(readiness_assertion_guard, "load_status_payload", return_value=payload), mock.patch.object(
            readiness_assertion_guard,
            "active_target_blocking_levels",
            return_value=("repo-ready", "rc-ready"),
        ), mock.patch.object(readiness_assertion_guard, "run_selected_proof_commands", return_value=0) as runner:
            exit_code = readiness_assertion_guard.main(["--active-target", "--proof-type", "hybrid"])

        self.assertEqual(exit_code, 0)
        self.assertEqual(
            runner.call_args.args[0],
            [
                {
                    "assertion_id": "RA2",
                    "command_id": "rc",
                    "cwd": ".",
                    "run": ["python3", "-c", "print('rc')"],
                }
            ],
        )

    def test_main_blocks_conflicting_active_target_and_explicit_blocking_levels(self) -> None:
        payload = {
            "readiness_assertions": [
                {
                    "id": "RA8",
                    "blocking_level": "release-ready",
                    "proof_type": "hybrid",
                    "proof_commands": [{"id": "ga", "run": ["python3", "-c", "print('ga')"]}],
                }
            ]
        }

        with mock.patch.object(readiness_assertion_guard, "load_status_payload", return_value=payload), mock.patch.object(
            readiness_assertion_guard,
            "active_target_blocking_levels",
            return_value=("repo-ready", "rc-ready"),
        ):
            exit_code = readiness_assertion_guard.main(
                ["--active-target", "--blocking-level", "release-ready", "--proof-type", "hybrid"]
            )

        self.assertEqual(exit_code, 1)

    def test_main_skips_active_target_with_no_machine_proof_scope(self) -> None:
        payload = base_payload()

        with mock.patch.object(readiness_assertion_guard, "load_status_payload", return_value=payload), mock.patch.object(
            readiness_assertion_guard,
            "active_target_blocking_levels",
            return_value=(),
        ), mock.patch.object(readiness_assertion_guard, "run_selected_proof_commands") as runner:
            exit_code = readiness_assertion_guard.main(["--active-target", "--proof-type", "automated"])

        self.assertEqual(exit_code, 0)
        runner.assert_not_called()

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

    def test_deduplicated_proof_commands_groups_shared_runs(self) -> None:
        deduped = readiness_assertion_guard.deduplicated_proof_commands(
            [
                {
                    "assertion_id": "RA3",
                    "command_id": "shared-a",
                    "cwd": ".",
                    "run": ["python3", "-c", "print('shared')"],
                },
                {
                    "assertion_id": "RA4",
                    "command_id": "shared-b",
                    "cwd": ".",
                    "run": ["python3", "-c", "print('shared')"],
                },
                {
                    "assertion_id": "RA2",
                    "command_id": "unique",
                    "cwd": ".",
                    "run": ["python3", "-c", "print('unique')"],
                },
            ]
        )

        self.assertEqual(len(deduped), 2)
        self.assertEqual(deduped[0]["assertion_ids"], ["RA3", "RA4"])
        self.assertEqual(deduped[0]["command_ids"], ["shared-a", "shared-b"])
        self.assertEqual(deduped[1]["assertion_ids"], ["RA2"])

    def test_main_can_read_staged_status_payload(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            self.git(repo_root, "init")

            payload = base_payload()
            payload["readiness_assertions"][0]["proof_commands"][0]["run"] = [
                "python3",
                "-c",
                "from pathlib import Path; Path('proof.out').write_text('ok', encoding='utf-8')",
            ]
            write_status(repo_root, payload)
            self.git(repo_root, "add", "docs/release-control/v6/status.json")

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

    def test_run_selected_proof_commands_executes_staged_repo_python_script(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            self.git(repo_root, "init")

            script_rel = "scripts/release_control/proof_script.py"
            script_path = repo_root / script_rel
            script_path.parent.mkdir(parents=True, exist_ok=True)
            script_path.write_text(
                "from helper_module import write_ok\nwrite_ok()\n",
                encoding="utf-8",
            )
            helper_path = repo_root / "scripts" / "release_control" / "helper_module.py"
            helper_path.write_text(
                "from pathlib import Path\n\n\ndef write_ok():\n    Path('proof.out').write_text('ok', encoding='utf-8')\n",
                encoding="utf-8",
            )
            self.git(repo_root, "add", script_rel, "scripts/release_control/helper_module.py")

            script_path.write_text("raise SystemExit(1)\n", encoding="utf-8")

            commands = [
                {
                    "assertion_id": "RA8",
                    "command_id": "staged-script",
                    "cwd": ".",
                    "run": ["python3", script_rel],
                }
            ]

            with mock.patch.object(readiness_assertion_guard, "REPO_ROOT", repo_root), mock.patch(
                "repo_file_io.REPO_ROOT", repo_root
            ):
                exit_code = readiness_assertion_guard.run_selected_proof_commands(commands, staged=True)

            self.assertEqual(exit_code, 0)
            self.assertEqual((repo_root / "proof.out").read_text(encoding="utf-8"), "ok")


if __name__ == "__main__":
    unittest.main()
