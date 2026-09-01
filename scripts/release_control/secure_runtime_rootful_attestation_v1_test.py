#!/usr/bin/env python3
"""Adversarial tests for the schema-v1 rootful qualification validator."""

from __future__ import annotations

import copy
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import secure_runtime_rootful_attestation_v1 as attester


def sha(value: str | bytes) -> str:
    return hashlib.sha256(value if isinstance(value, bytes) else value.encode()).hexdigest()


class RootfulAttestationV1Test(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.commit = "a" * 40
        self.artifacts: dict[str, Path] = {}
        for key, basename in {
            "qualification_test": "dockeragent.test",
            "collector": "pulse-agent",
            "helper": "pulse-agent-helper",
            "installer": "install.sh",
        }.items():
            path = self.root / basename
            path.write_bytes(f"{key}-bytes".encode())
            path.chmod(0o700 if key != "installer" else 0o600)
            self.artifacts[key] = path
        self.sources = {
            "runtime.go": b"package runtime\n",
            "internal/api/api_tokens.go": b"package api\n",
            "scripts/install.sh": self.artifacts["installer"].read_bytes(),
        }
        for relative, data in self.sources.items():
            target = self.root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
        self.manifest = self.root / "manifest.json"
        self.manifest.write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "manifest_id": attester.SOURCE_MANIFEST_ID,
                    "target_os": "linux",
                    "description": "test boundary",
                    "exact_paths": sorted(self.sources),
                    "recursive_roots": [],
                    "include_suffixes": [".go"],
                    "exclude_suffixes": ["_test.go"],
                }
            ),
            encoding="utf-8",
        )
        self.receipt = self.make_receipt()

    def tearDown(self) -> None:
        self.temp.cleanup()

    @staticmethod
    def ts(index: int) -> str:
        value = datetime(2026, 9, 1, 10, tzinfo=timezone.utc) + timedelta(minutes=index)
        return value.isoformat().replace("+00:00", "Z")

    def summary(self, runtime: str, collector_pid: int, helper_pid: int, daemon_id: str) -> dict:
        return {
            "collector_pid": collector_pid,
            "helper_pid": helper_pid,
            "collection_mode": "typed-helper-summary",
            "inventory_complete": True,
            "inventory_count": 2,
            "semantic_sha256": sha(runtime + "-summary"),
            "full_fields_present": False,
            "stats_present": False,
            "secondary_structure_sha256": "",
            "container_updates_enabled": False,
            "container_actions_enabled": False,
            "direct_socket_access": False,
            "daemon_id": daemon_id,
            "daemon_rootless": False,
        }

    def make_run(self, runtime: str, index: int) -> dict:
        daemon = runtime + "-rootful-daemon"
        base_collector = 4100 + index * 100
        base_helper = 5100 + index * 100
        fresh = self.summary(runtime, base_collector, base_helper, daemon)
        migration = {
            **self.summary(runtime, base_collector + 1, base_helper + 1, daemon),
            "legacy_profile": "root-command-capable",
            "target_profile": "typed-helper-monitoring-only",
            "authority_reduced": True,
            "legacy_collector_pid": base_collector + 50,
        }
        collector_restart = {
            **self.summary(runtime, base_collector + 2, base_helper + 1, daemon),
            "previous_collector_pid": migration["collector_pid"],
            "previous_report_stream_id": runtime + "-migration-stream",
        }
        helper_restart = {
            **self.summary(runtime, collector_restart["collector_pid"], base_helper + 2, daemon),
            "previous_helper_pid": collector_restart["helper_pid"],
            "previous_helper_invocation_id": runtime + "-helper-before",
            "helper_invocation_id": runtime + "-helper-after",
        }
        loss = {
            "collector_pid": helper_restart["collector_pid"],
            "previous_helper_pid": helper_restart["helper_pid"],
            "collection_mode": "typed-helper-unavailable-status-only",
            "helper_available": False,
            "status_only": True,
            "inventory_complete": False,
            "inventory_present": False,
            "authoritative_inventory_replacement": False,
            "previous_authoritative_inventory_count": fresh["inventory_count"],
            "previous_authoritative_semantic_sha256": fresh["semantic_sha256"],
            "operation_status": "degraded",
            "operation": "container.inventory",
            "container_updates_enabled": False,
            "container_actions_enabled": False,
            "direct_socket_access": False,
        }
        recovery = {
            **self.summary(runtime, helper_restart["collector_pid"], base_helper + 3, daemon),
            "previous_helper_pid": helper_restart["helper_pid"],
            "previous_status_report_sequence": 3,
        }
        bounds = {
            "collector_pid": recovery["collector_pid"],
            "helper_pid": recovery["helper_pid"],
            "operation": "container.inventory",
            "failure_class": "bounded-timeout",
            "deadline_ms": 2000,
            "elapsed_ms": 2050,
            "bounded_failure_observed": True,
            "status_only_report_sequence": 5,
            "recovery_report_sequence": 6,
            "collection_mode": "typed-helper-summary",
            "inventory_complete": True,
            "previous_authoritative_inventory_count": fresh["inventory_count"],
            "previous_authoritative_semantic_sha256": fresh["semantic_sha256"],
            "recovery_inventory_count": fresh["inventory_count"],
            "recovery_semantic_sha256": fresh["semantic_sha256"],
            "full_fields_present": False,
            "stats_present": False,
            "secondary_structure_sha256": "",
            "authoritative_empty_replacement": False,
            "collector_alive": True,
            "helper_alive": True,
            "container_updates_enabled": False,
            "container_actions_enabled": False,
            "direct_socket_access": False,
        }
        update = {
            **self.summary(runtime, base_collector + 3, recovery["helper_pid"], daemon),
            "previous_collector_pid": recovery["collector_pid"],
            "previous_helper_pid": recovery["helper_pid"],
            "previous_report_stream_id": runtime + "-restart-stream",
            "update_applied": True,
            "collector_binary_sha256": sha(self.artifacts["collector"].read_bytes()),
            "helper_binary_sha256": sha(self.artifacts["helper"].read_bytes()),
        }
        authority = {
            "collector_pid": update["collector_pid"],
            "collector_uid": 1200 + index,
            "effective_uid": 1200 + index,
            "effective_root": False,
            "safe_profile_enabled": True,
            "commands_enabled": False,
            "privileged_helper_enabled": True,
            "reduction_request_observed": True,
            "collector_command_transport_present": False,
            "collector_command_session_present": False,
            "container_actions_enabled": False,
            "container_updates_enabled": False,
            "rootful_socket_access": False,
            "direct_socket_access": False,
            "helper_network_access": False,
        }
        cleanup = {
            "collector_stopped": True,
            "helper_stopped": True,
            "runtime_stopped": True,
            "socket_absent": True,
            "fixtures_removed": True,
            "state_clean": True,
        }
        evidences = [fresh, migration, collector_restart, helper_restart, loss, recovery, bounds, update, authority, cleanup]
        reports = {
            "fresh_install": (runtime + "-fresh-stream", 1),
            "legacy_migration": (runtime + "-migration-stream", 1),
            "collector_restart": (runtime + "-restart-stream", 1),
            "helper_restart": (runtime + "-restart-stream", 2),
            "helper_loss": (runtime + "-restart-stream", 3),
            "helper_recovery": (runtime + "-restart-stream", 4),
            "operation_bounds": (runtime + "-restart-stream", 6),
            "update_preservation": (runtime + "-update-stream", 1),
        }
        scenarios = []
        for scenario_index, name in enumerate(attester.REQUIRED_SCENARIOS):
            stream, sequence = reports.get(name, (None, None))
            scenarios.append(
                {
                    "name": name,
                    "result": "passed",
                    "started_at": self.ts(index * 25 + scenario_index * 2 + 1),
                    "completed_at": self.ts(index * 25 + scenario_index * 2 + 2),
                    "report_stream_id": stream,
                    "report_sequence": sequence,
                    "evidence": evidences[scenario_index],
                }
            )
        return {
            "host": {
                "machine_id": f"{index + 1:032x}",
                "architecture": "amd64",
                "kernel": "6.12.0-test",
                "systemd_version": "systemd-257",
            },
            "runtime": {
                "runtime": runtime,
                "runtime_version": "27.3.1" if runtime == "docker" else "5.2.2",
                "daemon_id": daemon,
                "daemon_rootless": False,
                "socket_path": attester.expected_socket_path(runtime),
                "socket_uid": 0,
                "socket_gid": 0,
                "socket_mode": "0660",
                "socket_type": "unix",
                "socket_symlink": False,
            },
            "scenarios": scenarios,
        }

    def make_receipt(self) -> dict:
        packages = {
            "qualification_test": "github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test",
            "collector": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent",
            "helper": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper",
        }
        artifacts = {
            "installer": {
                "path_basename": "install.sh",
                "sha256": sha(self.artifacts["installer"].read_bytes()),
            }
        }
        for name, package in packages.items():
            artifacts[name] = {
                "path_basename": self.artifacts[name].name,
                "sha256": sha(self.artifacts[name].read_bytes()),
                "package": package,
                "go_version": "go1.25.1",
                "vcs_revision": self.commit,
                "vcs_modified": False,
            }
        return {
            "schema_version": 1,
            "kind": attester.RECEIPT_KIND,
            "result": "passed",
            "source_commit": self.commit,
            "started_at": self.ts(0),
            "completed_at": self.ts(50),
            "source_hashes": {name: sha(data) for name, data in self.sources.items()},
            "artifacts": artifacts,
            "runs": [self.make_run("docker", 0), self.make_run("podman", 1)],
        }

    def scenario(self, receipt: dict, run: int, name: str) -> dict:
        return receipt["runs"][run]["scenarios"][attester.REQUIRED_SCENARIOS.index(name)]

    def invalid(self, mutation) -> None:
        receipt = copy.deepcopy(self.receipt)
        mutation(receipt)
        with self.assertRaises(attester.ValidationError):
            attester.validate_receipt(receipt)

    def write_receipt(self, receipt: dict | None = None) -> Path:
        path = self.root / "receipt.json"
        path.write_text(json.dumps(receipt or self.receipt, sort_keys=True), encoding="utf-8")
        return path

    def valid_build(self, package: str) -> dict:
        return {"package": package, "go_version": "go1.25.1", "vcs_revision": self.commit, "vcs_modified": False}

    def attest(self, receipt: dict | None = None) -> dict:
        packages = {name: self.receipt["artifacts"][name]["package"] for name in ("qualification_test", "collector", "helper")}
        by_basename = {path.name: key for key, path in self.artifacts.items()}
        with mock.patch.object(attester, "inspect_go_build_identity", side_effect=lambda path: self.valid_build(packages[by_basename[path.name]])):
            return attester.create_attestation(
                self.write_receipt(receipt),
                self.artifacts["qualification_test"],
                self.artifacts["collector"],
                self.artifacts["helper"],
                self.artifacts["installer"],
                checkout=self.root,
                manifest_path=self.manifest,
                verify_git_commit=False,
            )

    def test_valid_receipt_and_local_attestation(self) -> None:
        attester.validate_receipt(self.receipt)
        result = self.attest()
        self.assertEqual(result["validated_runtimes"], ["docker", "podman"])
        self.assertEqual(result["classification"], attester.CLASSIFICATION)
        self.assertEqual(set(result["artifact_bindings"]), set(self.artifacts))
        self.assertEqual(
            result["limitations"],
            [
                "not-published-release-provenance",
                "not-default-profile-authorization",
                "not-independent-security-review",
                "production-exact-scope-proof-is-external-prior",
            ],
        )

    def test_exact_runtime_topology_and_order_fail_closed(self) -> None:
        mutations = [
            lambda r: r.update(schema_version=True),
            lambda r: r["runs"].pop(),
            lambda r: r["runs"].reverse(),
            lambda r: r["runs"][1]["host"].update(machine_id=r["runs"][0]["host"]["machine_id"]),
            lambda r: r["runs"][1]["runtime"].update(daemon_id=r["runs"][0]["runtime"]["daemon_id"]),
            lambda r: r["runs"][0]["runtime"].update(daemon_rootless=True),
            lambda r: r["runs"][0]["runtime"].update(socket_path="/run/docker.sock"),
            lambda r: r["runs"][0]["runtime"].update(socket_uid=1000),
            lambda r: r["runs"][0]["runtime"].update(socket_mode="0666"),
            lambda r: r["runs"][0]["runtime"].update(socket_symlink=True),
            lambda r: r["runs"][0]["scenarios"].reverse(),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.invalid(mutation)

    def test_summary_is_exactly_helper_only_and_continuous(self) -> None:
        mutations = [
            lambda r: self.scenario(r, 0, "fresh_install")["evidence"].update(collection_mode="direct-rootful-socket"),
            lambda r: self.scenario(r, 0, "fresh_install")["evidence"].update(full_fields_present=True),
            lambda r: self.scenario(r, 0, "fresh_install")["evidence"].update(stats_present=True),
            lambda r: self.scenario(r, 0, "fresh_install")["evidence"].update(secondary_structure_sha256=sha("secondary")),
            lambda r: self.scenario(r, 0, "fresh_install")["evidence"].update(container_updates_enabled=True),
            lambda r: self.scenario(r, 0, "fresh_install")["evidence"].update(container_actions_enabled=True),
            lambda r: self.scenario(r, 0, "fresh_install")["evidence"].update(direct_socket_access=True),
            lambda r: self.scenario(r, 0, "helper_recovery")["evidence"].update(inventory_count=3),
            lambda r: self.scenario(r, 0, "update_preservation")["evidence"].update(semantic_sha256=sha("wrong")),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.invalid(mutation)

    def test_pid_stream_loss_recovery_and_bounds_are_causal(self) -> None:
        mutations = [
            lambda r: self.scenario(r, 0, "legacy_migration")["evidence"].update(authority_reduced=False),
            lambda r: self.scenario(r, 0, "collector_restart")["evidence"].update(previous_collector_pid=9999),
            lambda r: self.scenario(r, 0, "helper_restart")["evidence"].update(previous_helper_pid=9999),
            lambda r: self.scenario(r, 0, "helper_restart")["evidence"].update(helper_invocation_id="docker-helper-before"),
            lambda r: self.scenario(r, 0, "helper_loss")["evidence"].update(status_only=False),
            lambda r: self.scenario(r, 0, "helper_loss")["evidence"].update(inventory_present=True),
            lambda r: self.scenario(r, 0, "helper_loss")["evidence"].update(authoritative_inventory_replacement=True),
            lambda r: self.scenario(r, 0, "helper_recovery")["evidence"].update(previous_status_report_sequence=2),
            lambda r: self.scenario(r, 0, "operation_bounds")["evidence"].update(elapsed_ms=5000),
            lambda r: self.scenario(r, 0, "operation_bounds")["evidence"].update(deadline_ms=60000, elapsed_ms=100),
            lambda r: self.scenario(r, 0, "operation_bounds")["evidence"].update(authoritative_empty_replacement=True),
            lambda r: self.scenario(r, 0, "operation_bounds")["evidence"].update(stats_present=True),
            lambda r: self.scenario(r, 0, "operation_bounds")["evidence"].update(recovery_report_sequence=5),
            lambda r: self.scenario(r, 0, "update_preservation")["evidence"].update(previous_collector_pid=9999),
            lambda r: self.scenario(r, 0, "update_preservation")["evidence"].update(previous_helper_pid=9999),
            lambda r: self.scenario(r, 0, "update_preservation")["evidence"].update(helper_pid=9999),
            lambda r: self.scenario(r, 0, "update_preservation")["evidence"].update(collector_binary_sha256=sha("wrong")),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.invalid(mutation)

    def test_authority_cleanup_secret_and_extra_fields_fail_closed(self) -> None:
        mutations = [
            lambda r: self.scenario(r, 0, "authority_isolation")["evidence"].update(commands_enabled=True),
            lambda r: self.scenario(r, 0, "authority_isolation")["evidence"].update(rootful_socket_access=True),
            lambda r: self.scenario(r, 0, "authority_isolation")["evidence"].update(helper_network_access=True),
            lambda r: self.scenario(r, 0, "cleanup")["evidence"].update(socket_absent=False),
            lambda r: self.scenario(r, 0, "cleanup")["evidence"].update(user_state_clean=True),
            lambda r: r["runs"][0]["host"].update(kernel="Bearer abcdefghijklmnop"),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.invalid(mutation)

    def test_artifact_source_and_duplicate_json_binding(self) -> None:
        self.artifacts["collector"].write_bytes(b"different")
        self.artifacts["collector"].chmod(0o700)
        with self.assertRaises(attester.ValidationError):
            self.attest()
        self.artifacts["collector"].write_bytes(b"collector-bytes")
        self.artifacts["collector"].chmod(0o700)
        helper = self.artifacts["helper"]
        real_helper = self.root / "helper-real"
        helper.rename(real_helper)
        helper.symlink_to(real_helper)
        with self.assertRaises(attester.ValidationError):
            self.attest()
        helper.unlink()
        real_helper.rename(helper)
        bad = copy.deepcopy(self.receipt)
        bad["source_hashes"]["runtime.go"] = sha("wrong")
        with self.assertRaises(attester.ValidationError):
            self.attest(bad)
        with self.assertRaises(attester.ValidationError):
            attester.parse_receipt_bytes(b'{"schema_version":1,"schema_version":1}')
        with mock.patch.object(attester, "inspect_go_build_identity", return_value=self.valid_build("github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent")):
            with self.assertRaises(attester.ValidationError):
                attester.create_attestation(
                    self.write_receipt(), self.artifacts["qualification_test"], self.artifacts["collector"],
                    self.artifacts["helper"], self.artifacts["installer"], checkout=self.root,
                    manifest_path=self.manifest, verify_git_commit=False,
                )
        args = attester.parse_args([
            "receipt.json", "--qualification-test", "dockeragent.test", "--collector", "pulse-agent",
            "--helper", "pulse-agent-helper", "--installer", "install.sh",
        ])
        self.assertEqual(args.qualification_test.name, "dockeragent.test")

    @unittest.skipUnless(hasattr(os, "mkfifo"), "FIFO validation requires Unix mkfifo")
    def test_fifo_receipt_and_artifact_paths_fail_without_blocking(self) -> None:
        module_dir = Path(attester.__file__).resolve().parent
        receipt_fifo = self.root / "receipt.fifo"
        os.mkfifo(receipt_fifo, 0o600)
        receipt_probe = (
            "import sys; from pathlib import Path; sys.path.insert(0, sys.argv[1]); "
            "import secure_runtime_rootful_attestation_v1 as a; a.read_immutable_receipt(Path(sys.argv[2]))"
        )
        result = subprocess.run([sys.executable, "-I", "-c", receipt_probe, str(module_dir), str(receipt_fifo)], capture_output=True, timeout=2)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"receipt must be a regular file", result.stderr)

        artifact_fifo = self.root / "pulse-agent"
        artifact_fifo.unlink()
        os.mkfifo(artifact_fifo, 0o700)
        artifact_probe = (
            "import sys; from pathlib import Path; sys.path.insert(0, sys.argv[1]); "
            "import secure_runtime_rootful_attestation_v1 as a; "
            "a.immutable_artifact_snapshot(Path(sys.argv[2]), 'pulse-agent', executable=True).__enter__()"
        )
        result = subprocess.run([sys.executable, "-I", "-c", artifact_probe, str(module_dir), str(artifact_fifo)], capture_output=True, timeout=2)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"artifact pulse-agent must be a regular file", result.stderr)

    def test_manifest_contract_binds_transitive_harness_and_production_boundary(self) -> None:
        manifest = json.loads((Path(__file__).with_name("secure_runtime_rootful_source_manifest_v1.json")).read_text(encoding="utf-8"))
        required = {
            "scripts/installtests/secure_runtime_rootful_qualification_test.go",
            "scripts/installtests/secure_runtime_rootless_qualification_test.go",
            "scripts/installtests/secure_runtime_systemd_lab_test.go",
            "scripts/release_control/secure_runtime_rootful_attestation_v1.py",
            "scripts/release_control/secure_runtime_rootful_source_manifest_v1.json",
            "scripts/release_control/secure_runtime_rootless_attestation_v1.py",
            "scripts/run-secure-runtime-rootful-qualification.sh",
        }
        self.assertTrue(required.issubset(manifest["exact_paths"]))
        self.assertIn("internal/agenthelper", manifest["recursive_roots"])
        self.assertIn("pkg/auth", manifest["recursive_roots"])


if __name__ == "__main__":
    unittest.main()
