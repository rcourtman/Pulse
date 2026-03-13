import unittest

from control_plane import release_branch_for_version
from control_plane_audit import audit_control_plane_payload, parse_args


VALID_PAYLOAD = {
    "version": "1",
    "system": "pulse-release-control",
    "execution_model": "direct-repo-sessions",
    "control_plane_doc": "docs/release-control/CONTROL_PLANE.md",
    "control_plane_schema": "docs/release-control/control_plane.schema.json",
    "active_profile_id": "v6",
    "active_target_id": "v6-rc-cut",
    "profiles": [
        {
            "id": "v6",
            "lifecycle": "active",
            "root": "docs/release-control/v6",
            "prerelease_branch": "pulse/v6",
            "stable_branch": "pulse/v6",
            "source_of_truth": "docs/release-control/v6/SOURCE_OF_TRUTH.md",
            "status": "docs/release-control/v6/status.json",
            "status_schema": "docs/release-control/v6/status.schema.json",
            "development_protocol": "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
            "high_risk_matrix": "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
            "subsystems_dir": "docs/release-control/v6/subsystems",
            "registry": "docs/release-control/v6/subsystems/registry.json",
            "registry_schema": "docs/release-control/v6/subsystems/registry.schema.json",
            "subsystem_contract_template": "docs/release-control/v6/SUBSYSTEM_CONTRACT_TEMPLATE.md",
        }
    ],
    "targets": [
        {
            "id": "v6-rc-cut",
            "profile_id": "v6",
            "kind": "release",
            "status": "active",
            "summary": "Drive Pulse v6 to a governed RC cut.",
            "completion_rule": "rc_ready",
            "proof_scope": "derived",
        },
        {
            "id": "v6-ga-promotion",
            "profile_id": "v6",
            "kind": "release",
            "status": "planned",
            "summary": "Promote Pulse v6 from validated RC to governed GA.",
            "completion_rule": "release_ready",
            "proof_scope": "derived",
        }
    ],
}

VALID_PATH_KINDS = {
    "docs/release-control/CONTROL_PLANE.md": "file",
    "docs/release-control/control_plane.schema.json": "file",
    "docs/release-control/v6": "dir",
    "docs/release-control/v6/SOURCE_OF_TRUTH.md": "file",
    "docs/release-control/v6/status.json": "file",
    "docs/release-control/v6/status.schema.json": "file",
    "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md": "file",
    "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md": "file",
    "docs/release-control/v6/subsystems": "dir",
    "docs/release-control/v6/subsystems/registry.json": "file",
    "docs/release-control/v6/subsystems/registry.schema.json": "file",
    "docs/release-control/v6/SUBSYSTEM_CONTRACT_TEMPLATE.md": "file",
}


class ControlPlaneAuditTest(unittest.TestCase):
    def test_parse_args_accepts_staged_flag(self) -> None:
        args = parse_args(["--check", "--staged"])
        self.assertTrue(args.check)
        self.assertTrue(args.staged)

    def test_audit_accepts_valid_payload(self) -> None:
        report = audit_control_plane_payload(
            VALID_PAYLOAD,
            path_kinds=VALID_PATH_KINDS,
            status_report={"errors": [], "control_plane": {"active_target": {"completion_met": False}}},
        )

        self.assertEqual(report["errors"], [])
        self.assertEqual(report["summary"]["profile_count"], 1)
        self.assertFalse(report["summary"]["active_target_completion_met"])

    def test_release_branch_for_version_uses_profile_branch_policy(self) -> None:
        self.assertEqual(release_branch_for_version("6.0.0-rc.1", control_plane=VALID_PAYLOAD), "pulse/v6")
        self.assertEqual(release_branch_for_version("6.0.0", control_plane=VALID_PAYLOAD), "pulse/v6")

    def test_audit_flags_stale_active_target(self) -> None:
        report = audit_control_plane_payload(
            VALID_PAYLOAD,
            path_kinds=VALID_PATH_KINDS,
            status_report={"errors": [], "control_plane": {"active_target": {"completion_met": True}}},
        )

        self.assertTrue(report["errors"])
        self.assertIn("already satisfies its completion rule", "\n".join(report["errors"]))

    def test_audit_flags_missing_profile_path(self) -> None:
        broken_paths = dict(VALID_PATH_KINDS)
        broken_paths["docs/release-control/v6/status.json"] = "missing"

        report = audit_control_plane_payload(
            VALID_PAYLOAD,
            path_kinds=broken_paths,
            status_report={"errors": [], "control_plane": {"active_target": {"completion_met": False}}},
        )

        self.assertTrue(report["errors"])
        self.assertIn("expects file", "\n".join(report["errors"]))


if __name__ == "__main__":
    unittest.main()
