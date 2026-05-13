import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import registry_audit
from repo_file_io import canonical_repo_id, strip_local_git_env
from registry_audit import audit_registry_payload, parse_args, tracked_workspace_files


class RegistryAuditTest(unittest.TestCase):
    def git(self, repo_root: Path, *args: str) -> subprocess.CompletedProcess:
        env = os.environ.copy()
        strip_local_git_env(env)
        return subprocess.run(
            ["git", *args],
            cwd=repo_root,
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )

    def git_stdout(self, repo_root: Path, *args: str) -> str:
        return self.git(repo_root, *args).stdout.strip()

    def hook_env_for_worktree(self, worktree_root: Path) -> dict[str, str]:
        git_dir = self.git_stdout(worktree_root, "rev-parse", "--path-format=absolute", "--git-dir")
        common_dir = self.git_stdout(worktree_root, "rev-parse", "--path-format=absolute", "--git-common-dir")
        work_tree = self.git_stdout(worktree_root, "rev-parse", "--show-toplevel")
        return {
            "GIT_DIR": git_dir,
            "GIT_WORK_TREE": work_tree,
            "GIT_INDEX_FILE": str(Path(git_dir) / "index"),
            "GIT_COMMON_DIR": common_dir,
        }

    def test_parse_args_accepts_staged_flag(self) -> None:
        args = parse_args(["--check", "--staged"])
        self.assertTrue(args.check)
        self.assertTrue(args.staged)

    def test_tracked_workspace_files_uses_linked_worktree_as_canonical_local_repo(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir) / "workspace"
            repo_root = workspace / "repos" / "pulse"
            linked_worktree = workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            repo_root.mkdir(parents=True)
            linked_worktree.parent.mkdir(parents=True)

            self.git(repo_root, "init")
            (repo_root / "internal").mkdir()
            (repo_root / "internal" / "existing.go").write_text("package internal\n", encoding="utf-8")
            self.git(repo_root, "add", "internal/existing.go")
            self.git(
                repo_root,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )
            self.git(repo_root, "worktree", "add", "--detach", str(linked_worktree), "HEAD")

            (linked_worktree / "internal" / "staged.go").write_text("package internal\n", encoding="utf-8")
            self.git(linked_worktree, "add", "internal/staged.go")
            contracts_dir = linked_worktree / "docs" / "release-control" / "v6" / "internal" / "subsystems"
            contracts_dir.mkdir(parents=True)

            with patch("registry_audit.REPO_ROOT", linked_worktree), patch(
                "registry_audit.DEFAULT_CONTROL_PLANE",
                {
                    **registry_audit.DEFAULT_CONTROL_PLANE,
                    "subsystems_dir_path": str(contracts_dir),
                },
            ):
                files = tracked_workspace_files(
                    active_repos=["pulse"],
                    local_repo=canonical_repo_id(linked_worktree),
                )

            self.assertIn("internal/staged.go", files)
            self.assertNotIn("pulse:internal/staged.go", files)

    def test_tracked_workspace_files_fixture_isolated_from_linked_worktree_hook_env(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp_root = Path(tmpdir)
            caller_workspace = tmp_root / "caller" / "workspace"
            caller_repo = caller_workspace / "repos" / "pulse"
            caller_worktree = caller_workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            caller_repo.mkdir(parents=True)
            caller_worktree.parent.mkdir(parents=True)

            self.git(caller_repo, "init")
            (caller_repo / "README.md").write_text("caller\n", encoding="utf-8")
            self.git(caller_repo, "add", "README.md")
            self.git(
                caller_repo,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )
            self.git(caller_repo, "worktree", "add", "--detach", str(caller_worktree), "HEAD")

            hook_env = self.hook_env_for_worktree(caller_worktree)
            caller_head_count_before = self.git_stdout(caller_worktree, "rev-list", "--count", "HEAD")

            fixture_workspace = tmp_root / "fixture" / "workspace"
            repo_root = fixture_workspace / "repos" / "pulse"
            linked_worktree = fixture_workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            repo_root.mkdir(parents=True)
            linked_worktree.parent.mkdir(parents=True)

            with patch.dict(os.environ, hook_env, clear=False):
                self.git(repo_root, "init")
                (repo_root / "internal").mkdir()
                (repo_root / "internal" / "existing.go").write_text("package internal\n", encoding="utf-8")
                self.git(repo_root, "add", "internal/existing.go")
                self.git(
                    repo_root,
                    "-c",
                    "user.name=Pulse Test",
                    "-c",
                    "user.email=pulse-test@example.invalid",
                    "commit",
                    "-m",
                    "initial",
                )
                self.git(repo_root, "worktree", "add", "--detach", str(linked_worktree), "HEAD")

                (linked_worktree / "internal" / "staged.go").write_text("package internal\n", encoding="utf-8")
                self.git(linked_worktree, "add", "internal/staged.go")
                contracts_dir = linked_worktree / "docs" / "release-control" / "v6" / "internal" / "subsystems"
                contracts_dir.mkdir(parents=True)

                with patch("registry_audit.REPO_ROOT", linked_worktree), patch(
                    "registry_audit.DEFAULT_CONTROL_PLANE",
                    {
                        **registry_audit.DEFAULT_CONTROL_PLANE,
                        "subsystems_dir_path": str(contracts_dir),
                    },
                ):
                    files = tracked_workspace_files(
                        active_repos=["pulse"],
                        local_repo=canonical_repo_id(linked_worktree),
                    )

            self.assertIn("internal/staged.go", files)
            self.assertEqual(caller_head_count_before, self.git_stdout(caller_worktree, "rev-list", "--count", "HEAD"))
            self.assertEqual("", self.git_stdout(caller_worktree, "status", "--porcelain=v1"))
            self.assertFalse((caller_worktree / "internal").exists())

    def test_tracked_workspace_files_scrubs_linked_worktree_index_env_without_work_tree(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp_root = Path(tmpdir)
            caller_workspace = tmp_root / "caller" / "workspace"
            caller_repo = caller_workspace / "repos" / "pulse"
            caller_worktree = caller_workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            caller_repo.mkdir(parents=True)
            caller_worktree.parent.mkdir(parents=True)

            self.git(caller_repo, "init")
            (caller_repo / "README.md").write_text("caller\n", encoding="utf-8")
            self.git(caller_repo, "add", "README.md")
            self.git(
                caller_repo,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )
            self.git(caller_repo, "worktree", "add", "--detach", str(caller_worktree), "HEAD")

            hook_env = self.hook_env_for_worktree(caller_worktree)
            hook_env.pop("GIT_WORK_TREE")

            fixture_workspace = tmp_root / "fixture" / "workspace"
            repo_root = fixture_workspace / "repos" / "pulse"
            linked_worktree = fixture_workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            repo_root.mkdir(parents=True)
            linked_worktree.parent.mkdir(parents=True)

            self.git(repo_root, "init")
            (repo_root / "internal").mkdir()
            (repo_root / "internal" / "existing.go").write_text("package internal\n", encoding="utf-8")
            self.git(repo_root, "add", "internal/existing.go")
            self.git(
                repo_root,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )
            self.git(repo_root, "worktree", "add", "--detach", str(linked_worktree), "HEAD")

            (linked_worktree / "internal" / "staged.go").write_text("package internal\n", encoding="utf-8")
            self.git(linked_worktree, "add", "internal/staged.go")
            contracts_dir = linked_worktree / "docs" / "release-control" / "v6" / "internal" / "subsystems"
            contracts_dir.mkdir(parents=True)

            with patch("registry_audit.REPO_ROOT", linked_worktree), patch(
                "registry_audit.DEFAULT_CONTROL_PLANE",
                {
                    **registry_audit.DEFAULT_CONTROL_PLANE,
                    "subsystems_dir_path": str(contracts_dir),
                },
            ), patch.dict(os.environ, hook_env, clear=False):
                files = tracked_workspace_files(
                    active_repos=["pulse"],
                    local_repo=canonical_repo_id(linked_worktree),
                )

            self.assertIn("internal/staged.go", files)
            self.assertNotIn("README.md", files)

    def test_tracked_workspace_files_scrubs_hook_env_for_sibling_repos(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir) / "workspace"
            repo_root = workspace / "repos" / "pulse"
            mobile_repo = workspace / "repos" / "pulse-mobile"
            repo_root.mkdir(parents=True)
            mobile_repo.mkdir(parents=True)

            self.git(repo_root, "init")
            (repo_root / "internal").mkdir()
            (repo_root / "internal" / "local.go").write_text("package internal\n", encoding="utf-8")
            self.git(repo_root, "add", "internal/local.go")
            self.git(
                repo_root,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )

            self.git(mobile_repo, "init")
            (mobile_repo / "src" / "relay" / "__tests__").mkdir(parents=True)
            (mobile_repo / "src" / "relay" / "client.ts").write_text("export {}\n", encoding="utf-8")
            (mobile_repo / "src" / "relay" / "__tests__" / "channel.test.ts").write_text(
                "export {}\n",
                encoding="utf-8",
            )
            self.git(mobile_repo, "add", "src/relay/client.ts", "src/relay/__tests__/channel.test.ts")
            self.git(
                mobile_repo,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )

            hook_env = {
                "GIT_DIR": str(repo_root / ".git"),
                "GIT_WORK_TREE": str(repo_root),
                "GIT_INDEX_FILE": str(repo_root / ".git" / "index"),
                "GIT_COMMON_DIR": str(repo_root / ".git"),
            }
            contracts_dir = repo_root / "docs" / "release-control" / "v6" / "internal" / "subsystems"
            contracts_dir.mkdir(parents=True)

            with patch("registry_audit.REPO_ROOT", repo_root), patch(
                "registry_audit.DEFAULT_CONTROL_PLANE",
                {
                    **registry_audit.DEFAULT_CONTROL_PLANE,
                    "subsystems_dir_path": str(contracts_dir),
                },
            ), patch.dict(os.environ, hook_env, clear=False):
                files = tracked_workspace_files(
                    active_repos=["pulse", "pulse-mobile"],
                    local_repo="pulse",
                )

            self.assertIn("internal/local.go", files)
            self.assertIn("pulse-mobile:src/relay/client.ts", files)
            self.assertIn("pulse-mobile:src/relay/__tests__/channel.test.ts", files)
            self.assertNotIn("pulse-mobile:internal/local.go", files)

    def test_audit_registry_payload_accepts_valid_minimal_registry(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["internal/monitoring/canonical_guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "monitoring-runtime",
                                "label": "monitoring runtime proof",
                                "match_prefixes": ["internal/monitoring/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/monitoring/canonical_guardrails_test.go"],
                            }
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/monitoring.md",
            "internal/monitoring/monitor.go",
            "internal/monitoring/canonical_guardrails_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        self.assertEqual(report["errors"], [])
        self.assertEqual(report["summary"]["subsystem_count"], 1)
        self.assertEqual(report["summary"]["shared_ownership_count"], 0)
        self.assertEqual(report["subsystems"][0]["default_fallback_count"], 0)

    def test_audit_registry_payload_accepts_cross_repo_owned_prefixes(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "relay-runtime",
                    "lane": "L7",
                    "contract": "docs/release-control/v6/internal/subsystems/relay-runtime.md",
                    "owned_prefixes": ["pulse-mobile:src/relay/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": [
                            "pulse-mobile:src/relay/__tests__/client.test.ts",
                        ],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "mobile-relay-runtime",
                                "label": "mobile relay runtime proof",
                                "match_prefixes": ["pulse-mobile:src/relay/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": [
                                    "pulse-mobile:src/relay/__tests__/client.test.ts",
                                ],
                            }
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/relay-runtime.md",
            "pulse-mobile:src/relay/__tests__/client.test.ts",
            "pulse-mobile:src/relay/client.ts",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L7"})

        self.assertEqual(report["errors"], [])
        self.assertEqual(report["subsystems"][0]["owned_runtime_file_count"], 1)

    def test_audit_registry_payload_flags_unknown_lane_and_missing_contract(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L99",
                    "contract": "docs/release-control/v6/internal/subsystems/alerts.md",
                    "owned_prefixes": ["internal/alerts/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": [],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [],
                    },
                }
            ],
        }

        report = audit_registry_payload(payload, tracked_files={"internal/alerts/unified_eval.go"}, status_lane_ids={"L6"})

        self.assertTrue(report["errors"])
        error_text = "\n".join(report["errors"])
        self.assertIn("unknown status lane", error_text)
        self.assertIn("missing tracked file", error_text)

    def test_audit_registry_payload_flags_explicit_coverage_gap(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "cloud-paid",
                    "lane": "L3",
                    "contract": "docs/release-control/v6/internal/subsystems/cloud-paid.md",
                    "owned_prefixes": ["pkg/licensing/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["pkg/licensing/cloud_paid_guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "runtime-entitlement-surface",
                                "label": "runtime entitlement surface proof",
                                "match_prefixes": [],
                                "match_files": ["pkg/licensing/evaluator.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["pkg/licensing/cloud_paid_guardrails_test.go"],
                            }
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/cloud-paid.md",
            "pkg/licensing/evaluator.go",
            "pkg/licensing/features.go",
            "pkg/licensing/cloud_paid_guardrails_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L3"})

        self.assertTrue(report["errors"])
        self.assertIn("falls back to default verification", "\n".join(report["errors"]))

    def test_audit_registry_payload_requires_explicit_coverage_flag_true(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "api-contracts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/api-contracts.md",
                    "owned_prefixes": ["internal/api/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": ["internal/api/contract_test.go"],
                        "require_explicit_path_policy_coverage": False,
                        "path_policies": [
                            {
                                "id": "backend-payload-contracts",
                                "label": "backend API payload proof",
                                "match_prefixes": ["internal/api/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/api/contract_test.go"],
                            }
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
            "internal/api/alerts.go",
            "internal/api/contract_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        self.assertTrue(report["errors"])
        self.assertIn(
            "require_explicit_path_policy_coverage must be true",
            "\n".join(report["errors"]),
        )

    def test_audit_registry_payload_rejects_uncanonical_ordering(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/", "frontend/monitoring/"],
                    "owned_files": ["internal/monitoring/b.go", "internal/monitoring/a.go"],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": ["internal/monitoring/tests/", "frontend/tests/"],
                        "exact_files": [
                            "internal/monitoring/z_test.go",
                            "internal/monitoring/a_test.go",
                        ],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "monitoring-runtime",
                                "label": "monitoring runtime proof",
                                "match_prefixes": ["internal/monitoring/", "frontend/monitoring/"],
                                "match_files": ["internal/monitoring/b.go", "internal/monitoring/a.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": ["internal/monitoring/tests/", "frontend/tests/"],
                                "exact_files": [
                                    "internal/monitoring/z_test.go",
                                    "internal/monitoring/a_test.go",
                                ],
                            }
                        ],
                    },
                },
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/alerts.md",
                    "owned_prefixes": ["internal/alerts/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["internal/alerts/guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "alerts-runtime",
                                "label": "alerts runtime proof",
                                "match_prefixes": ["internal/alerts/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/alerts/guardrails_test.go"],
                            }
                        ],
                    },
                },
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/alerts.md",
            "docs/release-control/v6/internal/subsystems/monitoring.md",
            "frontend/tests/example.test.ts",
            "frontend/monitoring/ui.go",
            "internal/alerts/alert.go",
            "internal/alerts/guardrails_test.go",
            "internal/monitoring/a.go",
            "internal/monitoring/a_test.go",
            "internal/monitoring/b.go",
            "internal/monitoring/z_test.go",
            "internal/monitoring/tests/example_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        joined = "\n".join(report["errors"])
        self.assertIn("registry.json subsystems must be sorted by subsystem id", joined)
        self.assertIn("subsystems[0].owned_prefixes must be sorted lexicographically", joined)
        self.assertIn("subsystems[0].owned_files must be sorted lexicographically", joined)
        self.assertIn("subsystems[0].verification.test_prefixes must be sorted lexicographically", joined)
        self.assertIn("subsystems[0].verification.exact_files must be sorted lexicographically", joined)
        self.assertIn(
            "subsystems[0].verification.path_policies[0].match_prefixes must be sorted lexicographically",
            joined,
        )
        self.assertIn(
            "subsystems[0].verification.path_policies[0].match_files must be sorted lexicographically",
            joined,
        )
        self.assertIn(
            "subsystems[0].verification.path_policies[0].test_prefixes must be sorted lexicographically",
            joined,
        )
        self.assertIn(
            "subsystems[0].verification.path_policies[0].exact_files must be sorted lexicographically",
            joined,
        )

    def test_audit_registry_payload_rejects_fully_shadowed_path_policy(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["internal/monitoring/guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "all-monitoring",
                                "label": "all monitoring proof",
                                "match_prefixes": ["internal/monitoring/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/monitoring/guardrails_test.go"],
                            },
                            {
                                "id": "specific-file",
                                "label": "specific file proof",
                                "match_prefixes": [],
                                "match_files": ["internal/monitoring/monitor.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/monitoring/guardrails_test.go"],
                            },
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/monitoring.md",
            "internal/monitoring/guardrails_test.go",
            "internal/monitoring/monitor.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        self.assertIn(
            "subsystems[0].verification.path_policies[1] is unreachable because earlier path policies already match all owned runtime files",
            "\n".join(report["errors"]),
        )

    def test_audit_registry_payload_requires_declared_shared_ownership(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "api-contracts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/api-contracts.md",
                    "owned_prefixes": ["internal/api/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": ["internal/api/contract_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "api-runtime",
                                "label": "api runtime proof",
                                "match_prefixes": ["internal/api/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/api/contract_test.go"],
                            }
                        ],
                    },
                },
                {
                    "id": "unified-resources",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/unified-resources.md",
                    "owned_prefixes": ["internal/unifiedresources/"],
                    "owned_files": ["internal/api/resources.go"],
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": ["internal/unifiedresources/code_standards_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "resource-api",
                                "label": "resource api proof",
                                "match_prefixes": [],
                                "match_files": ["internal/api/resources.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/unifiedresources/code_standards_test.go"],
                            }
                        ],
                    },
                },
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
            "internal/api/contract_test.go",
            "internal/api/resources.go",
            "internal/unifiedresources/code_standards_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        self.assertIn(
            "registry.json missing shared ownership entry for 'internal/api/resources.go' owned by ['api-contracts', 'unified-resources']",
            "\n".join(report["errors"]),
        )

    def test_audit_registry_payload_rejects_stale_or_wrong_shared_ownership(self) -> None:
        payload = {
            "version": 13,
            "shared_ownerships": [
                {
                    "path": "internal/api/resources.go",
                    "rationale": "shared api resource boundary",
                    "subsystems": ["api-contracts", "monitoring"],
                },
                {
                    "path": "internal/api/stale.go",
                    "rationale": "stale declaration",
                    "subsystems": ["api-contracts", "unified-resources"],
                },
            ],
            "subsystems": [
                {
                    "id": "api-contracts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/api-contracts.md",
                    "owned_prefixes": ["internal/api/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": ["internal/api/contract_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "api-runtime",
                                "label": "api runtime proof",
                                "match_prefixes": ["internal/api/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/api/contract_test.go"],
                            }
                        ],
                    },
                },
                {
                    "id": "unified-resources",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/internal/subsystems/unified-resources.md",
                    "owned_prefixes": ["internal/unifiedresources/"],
                    "owned_files": ["internal/api/resources.go"],
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": ["internal/unifiedresources/code_standards_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "resource-api",
                                "label": "resource api proof",
                                "match_prefixes": [],
                                "match_files": ["internal/api/resources.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/unifiedresources/code_standards_test.go"],
                            }
                        ],
                    },
                },
            ],
        }
        tracked_files = {
            "docs/release-control/v6/internal/subsystems/api-contracts.md",
            "docs/release-control/v6/internal/subsystems/unified-resources.md",
            "internal/api/contract_test.go",
            "internal/api/resources.go",
            "internal/api/stale.go",
            "internal/unifiedresources/code_standards_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        joined = "\n".join(report["errors"])
        self.assertIn(
            "shared_ownerships[0].subsystems = ['api-contracts', 'monitoring'], want ['api-contracts', 'unified-resources']",
            joined,
        )
        self.assertIn(
            "shared_ownerships[1].path = 'internal/api/stale.go' is not an actual shared-owned runtime file",
            joined,
        )
        self.assertIn(
            "registry.json shared ownership entry for 'internal/api/stale.go' is stale",
            joined,
        )


if __name__ == "__main__":
    unittest.main()
