#!/usr/bin/env python3
"""Adversarial tests for the schema-v1 rootless qualification validator."""

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
import secure_runtime_rootless_attestation_v1 as attester


def sha(value: str | bytes) -> str:
    return hashlib.sha256(value if isinstance(value, bytes) else value.encode()).hexdigest()


class RootlessAttestationV1Test(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.commit = "a" * 40
        self.artifacts = {}
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
        self.manifest.write_text(json.dumps({
            "schema_version": 1, "manifest_id": attester.SOURCE_MANIFEST_ID,
            "target_os": "linux", "description": "test boundary",
            "exact_paths": sorted(self.sources), "recursive_roots": [],
            "include_suffixes": [".go"], "exclude_suffixes": ["_test.go"],
        }), encoding="utf-8")
        self.receipt = self.make_receipt()

    def tearDown(self) -> None:
        self.temp.cleanup()

    @staticmethod
    def ts(index: int) -> str:
        value = datetime(2026, 9, 1, 10, tzinfo=timezone.utc) + timedelta(minutes=index)
        return value.isoformat().replace("+00:00", "Z")

    @staticmethod
    def socket(runtime: str, uid: int, gid: int, mode: str) -> dict:
        return {"runtime": runtime, "path": attester.expected_socket_path(runtime, uid), "uid": uid,
                "gid": gid, "mode": mode, "type": "unix", "symlink": False}

    @staticmethod
    def socket_permissions(runtime: str) -> tuple[int, str]:
        return (4100, "0660") if runtime == "docker" else (4200, "0600")

    def direct(self, runtime: str, pid: int, daemon: str, uid: int, gid: int, mode: str) -> dict:
        return {
            "collector_pid": pid, "service_pid": pid, "collection_path": "collector-owned-rootless-socket",
            "inventory_complete": True, "inventory_count": 2,
            "semantic_sha256": sha(runtime + "-semantic"),
            "full_fields_present": True, "stats_present": True,
            "secondary_structure_sha256": sha(runtime + "-secondary"),
            "daemon_id": daemon, "daemon_rootless": True,
            "socket_path": attester.expected_socket_path(runtime, uid), "socket_uid": uid,
            "socket_gid": gid, "socket_mode": mode, "socket_type": "unix", "socket_symlink": False,
        }

    def make_run(self, runtime: str, index: int) -> dict:
        uid = 1200 + index * 100
        gid, mode = self.socket_permissions(runtime)
        daemon = runtime + "-durable-daemon"
        fresh = self.direct(runtime, 4100 + index * 100, daemon, uid, gid, mode)
        migration = {**self.direct(runtime, fresh["collector_pid"] + 1, daemon, uid, gid, mode),
                     "legacy_profile": "root-command-capable", "target_profile": "typed-helper-monitoring-only",
                     "authority_reduced": True, "legacy_collector_pid": fresh["collector_pid"] + 50}
        restart = {**self.direct(runtime, fresh["collector_pid"] + 2, daemon, uid, gid, mode),
                   "previous_collector_pid": migration["collector_pid"],
                   "previous_report_stream_id": runtime + "-migration-stream"}
        daemon_restart = {**self.direct(runtime, restart["collector_pid"], daemon, uid, gid, mode),
                          "previous_daemon_pid": 5100 + index * 100, "daemon_pid": 5101 + index * 100,
                          "previous_daemon_invocation_id": runtime + "-invocation-before",
                          "daemon_invocation_id": runtime + "-invocation-after"}
        rootful_semantic = sha(runtime + "-rootful-semantic")
        fallback = {
            "collector_pid": restart["collector_pid"], "collection_mode": "typed-helper-summary",
            "direct_runtime_available": False, "helper_fallback": True, "inventory_complete": True,
            "inventory_count": 3, "rootful_baseline_inventory_count": 3,
            "semantic_sha256": rootful_semantic, "rootful_baseline_semantic_sha256": rootful_semantic,
            "full_fields_present": False, "stats_present": False, "secondary_structure_sha256": "",
            "container_actions_enabled": False, "container_updates_enabled": False, "collector_restart_count": 0,
        }
        recovery = self.direct(runtime, restart["collector_pid"], daemon, uid, gid, mode)
        ambiguity = {
            "protected_collector_pid": restart["collector_pid"], "probe_kind": "separate-unpinned-collector",
            "live_sockets": [
                self.socket(name, uid, *self.socket_permissions(name)) for name in attester.REQUIRED_RUNTIMES
            ],
            "admission_refused": True, "fail_closed": True, "daemon_probe_count": 0,
            "container_actions_enabled": False, "collector_restart_count": 0,
        }
        pin = {**self.direct(runtime, fresh["collector_pid"] + 3, daemon, uid, gid, mode),
               "previous_collector_pid": restart["collector_pid"],
               "previous_report_stream_id": runtime + "-restart-stream",
               "pin_source": "root-owned-systemd-unit", "pinned_socket_path": attester.expected_socket_path(runtime, uid),
               "socket_absent_observed": True, "fallback_report_sequence": 10, "recovery_report_sequence": 19,
               "recovered_socket_path": attester.expected_socket_path(runtime, uid),
               "selected_socket_path": attester.expected_socket_path(runtime, uid),
               "recovered_socket_uid": uid, "recovered_socket_gid": gid, "recovered_socket_mode": mode,
               "recovered_socket_type": "unix", "recovered_socket_symlink": False,
               "candidate_count": 1, "daemon_probe_count": 1, "collector_restart_count": 1}
        parity = {
            "collector_pid": pin["collector_pid"], "baseline_kind": "root-client-same-rootless-daemon",
            "baseline_inventory_count": 2, "collector_inventory_count": 2,
            "baseline_semantic_sha256": fresh["semantic_sha256"],
            "collector_semantic_sha256": fresh["semantic_sha256"],
            "collector_full_fields_present": True, "collector_stats_present": True,
            "collector_secondary_inventory_present": True,
        }
        authority = {
            "collector_pid": pin["collector_pid"], "collector_uid": uid, "effective_uid": uid,
            "effective_root": False, "safe_profile_enabled": True, "commands_enabled": False,
            "privileged_helper_enabled": True, "reduction_request_observed": True,
            "collector_command_transport_present": False, "collector_command_session_present": False,
            "container_actions_enabled": False, "container_updates_enabled": False,
            "rootful_socket_access": False, "helper_network_access": False,
        }
        evidence = [fresh, migration, restart, daemon_restart, fallback, recovery, ambiguity, pin, parity, authority,
                    {"runtime_stopped": True, "socket_absent": True, "fixtures_removed": True, "user_state_clean": True}]
        reports = {
            "fresh_install": (runtime + "-fresh-stream", 1),
            "legacy_migration": (runtime + "-migration-stream", 1),
            "collector_restart": (runtime + "-restart-stream", 1),
            "daemon_restart": (runtime + "-restart-stream", 2),
            "socket_loss_helper_fallback": (runtime + "-restart-stream", 3),
            "direct_recovery": (runtime + "-restart-stream", 4),
            "exact_pin_recovery": (runtime + "-update-stream", 20),
            "telemetry_parity": (runtime + "-update-stream", 21),
        }
        scenarios = []
        for scenario_index, name in enumerate(attester.REQUIRED_SCENARIOS):
            stream, sequence = reports.get(name, (None, None))
            scenarios.append({"name": name, "result": "passed",
                              "started_at": self.ts(index * 30 + scenario_index * 2 + 1),
                              "completed_at": self.ts(index * 30 + scenario_index * 2 + 2),
                              "report_stream_id": stream, "report_sequence": sequence,
                              "evidence": evidence[scenario_index]})
        return {
            "host": {"machine_id": f"{index + 1:032x}", "architecture": "amd64",
                     "kernel": "6.12.0-test", "systemd_version": "systemd-257"},
            "runtime": {"runtime": runtime, "runtime_version": "27.3.1" if runtime == "docker" else "5.2.2",
                        "daemon_id": daemon, "collector_uid": uid,
                        "socket_path": attester.expected_socket_path(runtime, uid), "socket_uid": uid,
                        "socket_gid": gid, "socket_mode": mode, "socket_type": "unix",
                        "socket_symlink": False, "daemon_rootless": True},
            "scenarios": scenarios,
        }

    def make_receipt(self) -> dict:
        packages = {
            "qualification_test": "github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test",
            "collector": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent",
            "helper": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper",
        }
        artifacts = {"installer": {"path_basename": "install.sh", "sha256": sha(self.artifacts["installer"].read_bytes())}}
        for name, package in packages.items():
            artifacts[name] = {"path_basename": self.artifacts[name].name,
                               "sha256": sha(self.artifacts[name].read_bytes()), "package": package,
                               "go_version": "go1.25.1", "vcs_revision": self.commit, "vcs_modified": False}
        return {
            "schema_version": 1, "kind": attester.RECEIPT_KIND, "result": "passed",
            "source_commit": self.commit, "started_at": self.ts(0), "completed_at": self.ts(60),
            "source_hashes": {name: sha(data) for name, data in self.sources.items()},
            "artifacts": artifacts, "runs": [self.make_run("docker", 0), self.make_run("podman", 1)],
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
        with mock.patch.object(attester, "inspect_go_build_identity", side_effect=lambda path: self.valid_build(packages[{v.name: k for k, v in self.artifacts.items()}[path.name]])):
            return attester.create_attestation(
                self.write_receipt(receipt), self.artifacts["qualification_test"], self.artifacts["collector"],
                self.artifacts["helper"], self.artifacts["installer"], checkout=self.root,
                manifest_path=self.manifest, verify_git_commit=False)

    def test_valid_two_disposable_host_receipt_and_artifacts(self) -> None:
        attester.validate_receipt(self.receipt)
        result = self.attest()
        self.assertEqual(result["validated_runtimes"], ["docker", "podman"])
        self.assertEqual(set(result["artifact_bindings"]), set(self.artifacts))
        self.assertIn("not-default-profile-authorization", result["limitations"])

    def test_topology_runtime_and_source_fail_closed(self) -> None:
        mutations = [
            lambda r: r["runs"].pop(),
            lambda r: r["runs"][1]["host"].update(machine_id=r["runs"][0]["host"]["machine_id"]),
            lambda r: r["runs"].reverse(),
            lambda r: r["runs"][0]["runtime"].update(daemon_rootless=False),
            lambda r: r["runs"][0]["runtime"].update(socket_path="/var/run/docker.sock"),
            lambda r: r["runs"][0]["runtime"].update(socket_mode="0666"),
            lambda r: r["source_hashes"].update({"scripts/install.sh": sha("wrong")}),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation): self.invalid(mutation)

    def test_migration_restart_stream_and_daemon_causality(self) -> None:
        mutations = [
            lambda r: r["runs"][0]["scenarios"].reverse(),
            lambda r: self.scenario(r, 0, "legacy_migration")["evidence"].update(authority_reduced=False),
            lambda r: self.scenario(r, 0, "legacy_migration").update(report_stream_id="docker-fresh-stream"),
            lambda r: self.scenario(r, 0, "collector_restart")["evidence"].update(previous_collector_pid=9999),
            lambda r: self.scenario(r, 0, "daemon_restart")["evidence"].update(daemon_id="recreated"),
            lambda r: self.scenario(r, 0, "daemon_restart")["evidence"].update(previous_daemon_pid=5101),
            lambda r: self.scenario(r, 0, "daemon_restart")["evidence"].update(previous_daemon_invocation_id="docker-invocation-after"),
            lambda r: self.scenario(r, 0, "direct_recovery").update(report_sequence=2),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation): self.invalid(mutation)

    def test_complete_fallback_reduction_ambiguity_and_structural_parity(self) -> None:
        fallback = self.scenario(self.receipt, 0, "socket_loss_helper_fallback")["evidence"]
        self.assertTrue(fallback["inventory_complete"])
        self.assertFalse(fallback["stats_present"])
        self.assertNotIn("stats_sha256", self.scenario(self.receipt, 0, "fresh_install")["evidence"])
        mutations = [
            lambda r: self.scenario(r, 0, "socket_loss_helper_fallback")["evidence"].update(inventory_complete=False),
            lambda r: self.scenario(r, 0, "socket_loss_helper_fallback")["evidence"].update(stats_present=True),
            lambda r: self.scenario(r, 0, "socket_loss_helper_fallback")["evidence"].update(container_actions_enabled=True),
            lambda r: self.scenario(r, 0, "dual_socket_ambiguity_refusal")["evidence"].update(daemon_probe_count=1),
            lambda r: self.scenario(r, 0, "dual_socket_ambiguity_refusal")["evidence"]["live_sockets"][0].update(path="/tmp/socket"),
            lambda r: self.scenario(r, 0, "dual_socket_ambiguity_refusal")["evidence"]["live_sockets"][1].update(gid=9999),
            lambda r: self.scenario(r, 0, "daemon_restart")["evidence"].update(full_fields_present=False),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation): self.invalid(mutation)

    def test_exact_pin_update_authority_and_cleanup_are_observable(self) -> None:
        mutations = [
            lambda r: self.scenario(r, 0, "exact_pin_recovery")["evidence"].update(collector_pid=4102, service_pid=4102),
            lambda r: self.scenario(r, 0, "exact_pin_recovery").update(report_stream_id="docker-restart-stream"),
            lambda r: self.scenario(r, 0, "exact_pin_recovery")["evidence"].update(fallback_report_sequence=20),
            lambda r: self.scenario(r, 0, "telemetry_parity")["evidence"].update(collector_semantic_sha256=sha("wrong")),
            lambda r: self.scenario(r, 0, "authority_isolation")["evidence"].update(api_scopes=["agent:report"]),
            lambda r: self.scenario(r, 0, "authority_isolation")["evidence"].update(reduction_request_observed=False),
            lambda r: self.scenario(r, 0, "authority_isolation")["evidence"].update(collector_command_session_present=True),
            lambda r: self.scenario(r, 0, "authority_isolation")["evidence"].update(helper_network_access=True),
            lambda r: self.scenario(r, 0, "cleanup")["evidence"].update(user_state_clean=False),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation): self.invalid(mutation)

    def test_token_named_governed_source_is_allowed_but_secret_evidence_is_not(self) -> None:
        attester.validate_receipt(self.receipt)
        self.invalid(lambda r: r["runs"][0]["host"].update(kernel="Bearer abcdefghijklmnop"))
        with self.assertRaises(attester.ValidationError): attester.reject_sensitive_evidence({"api_token": "redacted"})

    def test_artifact_bytes_symlinks_metadata_and_package_substitution_fail(self) -> None:
        self.artifacts["collector"].write_bytes(b"different")
        self.artifacts["collector"].chmod(0o700)
        with self.assertRaises(attester.ValidationError): self.attest()
        self.artifacts["collector"].write_bytes(b"collector-bytes")
        self.artifacts["collector"].chmod(0o700)
        helper_path = self.artifacts["helper"]
        real = self.root / "helper-real"
        helper_path.rename(real)
        helper_path.symlink_to(real)
        with self.assertRaises(attester.ValidationError): self.attest()
        helper_path.unlink()
        real.rename(helper_path)
        wrong = self.valid_build("github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent")
        with mock.patch.object(attester, "inspect_go_build_identity", return_value=wrong):
            with self.assertRaises(attester.ValidationError):
                attester.create_attestation(self.write_receipt(), *self.artifacts.values(), checkout=self.root,
                                            manifest_path=self.manifest, verify_git_commit=False)
        packages = [self.receipt["artifacts"][name]["package"] for name in ("qualification_test", "collector", "helper")]
        identities = [self.valid_build(package) for package in packages]
        identities[1]["vcs_revision"] = "b" * 40
        with mock.patch.object(attester, "inspect_go_build_identity", side_effect=identities):
            with self.assertRaises(attester.ValidationError):
                attester.create_attestation(self.write_receipt(), *self.artifacts.values(), checkout=self.root,
                                            manifest_path=self.manifest, verify_git_commit=False)

    @unittest.skipUnless(hasattr(os, "mkfifo"), "FIFO validation requires Unix mkfifo")
    def test_fifo_receipt_and_artifact_paths_are_rejected_without_blocking(self) -> None:
        module_dir = Path(attester.__file__).resolve().parent
        receipt_fifo = self.root / "receipt.fifo"
        os.mkfifo(receipt_fifo, 0o600)
        receipt_probe = (
            "import sys; from pathlib import Path; "
            "sys.path.insert(0, sys.argv[1]); "
            "import secure_runtime_rootless_attestation_v1 as a; "
            "a.read_immutable_receipt(Path(sys.argv[2]))"
        )
        result = subprocess.run(
            [sys.executable, "-I", "-c", receipt_probe, str(module_dir), str(receipt_fifo)],
            capture_output=True,
            timeout=2,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"receipt must be a regular file", result.stderr)

        artifact_dir = self.root / "fifo-artifact"
        artifact_dir.mkdir()
        artifact_fifo = artifact_dir / "pulse-agent"
        os.mkfifo(artifact_fifo, 0o700)
        artifact_probe = (
            "import sys; from pathlib import Path; "
            "sys.path.insert(0, sys.argv[1]); "
            "import secure_runtime_rootless_attestation_v1 as a; "
            "ctx=a.immutable_artifact_snapshot(Path(sys.argv[2]), 'pulse-agent', executable=True); "
            "ctx.__enter__()"
        )
        result = subprocess.run(
            [sys.executable, "-I", "-c", artifact_probe, str(module_dir), str(artifact_fifo)],
            capture_output=True,
            timeout=2,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"artifact pulse-agent must be a regular file", result.stderr)

    def test_go_metadata_parser_rejects_modified_and_mismatched_vcs(self) -> None:
        package = "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent"
        output = (f"/tmp/pulse-agent: go1.25.1\n\tpath\t{package}\n\tbuild\tvcs=git\n"
                  f"\tbuild\tvcs.revision={self.commit}\n\tbuild\tvcs.modified=false\n").encode()
        with mock.patch.object(attester.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, output, b"")):
            self.assertEqual(attester.inspect_go_build_identity(self.artifacts["collector"])["package"], package)
        for bad in (output.replace(b"vcs.modified=false", b"vcs.modified=true"),
                    output.replace(b"vcs=git", b"vcs=hg")):
            with mock.patch.object(attester.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, bad, b"")):
                with self.assertRaises(attester.ValidationError): attester.inspect_go_build_identity(self.artifacts["collector"])

    def test_duplicate_json_source_manifest_and_cli_contract(self) -> None:
        with self.assertRaises(attester.ValidationError):
            attester.parse_receipt_bytes(b'{"schema_version":1,"schema_version":1}')
        bad = copy.deepcopy(self.receipt)
        bad["source_hashes"]["runtime.go"] = sha("wrong")
        with self.assertRaises(attester.ValidationError): self.attest(bad)
        args = attester.parse_args(["receipt.json", "--qualification-test", "dockeragent.test",
                                    "--collector", "pulse-agent", "--helper", "pulse-agent-helper",
                                    "--installer", "install.sh"])
        self.assertEqual(args.qualification_test.name, "dockeragent.test")

    def test_canonical_manifest_expands_the_executed_boundary(self) -> None:
        checkout = Path(__file__).resolve().parents[2]
        _, hashes = attester.load_source_manifest(
            checkout, checkout / attester.SOURCE_MANIFEST_PATH
        )
        required = {
            "scripts/install.sh",
            "scripts/release_ldflags.sh",
            "scripts/installtests/secure_runtime_rootless_qualification_test.go",
            "scripts/installtests/secure_runtime_systemd_lab_test.go",
            "scripts/release_control/secure_runtime_rootless_attestation_v1.py",
            "scripts/release_control/secure_runtime_rootless_source_manifest_v1.json",
            "scripts/run-secure-runtime-rootless-qualification.sh",
            "internal/api/api_token_identity.go",
            "pkg/auth/token.go",
        }
        self.assertTrue(required.issubset(hashes), sorted(required - set(hashes)))


if __name__ == "__main__":
    unittest.main()
