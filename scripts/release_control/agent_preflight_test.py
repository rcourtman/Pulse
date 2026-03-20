import unittest
from unittest.mock import patch

import agent_preflight


class AgentPreflightTest(unittest.TestCase):
    def test_audit_preflight_passes_when_branch_and_active_claim_match(self) -> None:
        control_plane = {
            "prerelease_branch": "pulse/v6-release",
            "active_profile_id": "v6",
            "active_target_id": "v6-rc-stabilization",
        }
        status_payload = {
            "work_claims": [
                {
                    "id": "codex-lane-l13",
                    "agent_id": "codex",
                    "work_item": {"kind": "lane", "id": "L13"},
                    "claimed_at": "2026-03-20T10:00:00Z",
                    "expires_at": "2026-03-20T12:00:00Z",
                }
            ]
        }

        with patch.object(agent_preflight, "active_control_plane", return_value=control_plane), patch.object(
            agent_preflight, "load_status_payload", return_value=status_payload
        ), patch.object(agent_preflight, "current_branch", return_value="pulse/v6-release"):
            report = agent_preflight.audit_preflight(agent_id="codex", require_active_claim=True)

        self.assertEqual(report["errors"], [])
        self.assertEqual(report["active_claim_count"], 1)
        self.assertEqual(report["agent_active_claim_count"], 1)

    def test_audit_preflight_requires_active_claim_for_agent(self) -> None:
        control_plane = {
            "prerelease_branch": "pulse/v6-release",
            "active_profile_id": "v6",
            "active_target_id": "v6-rc-stabilization",
        }
        status_payload = {"work_claims": []}

        with patch.object(agent_preflight, "active_control_plane", return_value=control_plane), patch.object(
            agent_preflight, "load_status_payload", return_value=status_payload
        ), patch.object(agent_preflight, "current_branch", return_value="pulse/v6-release"):
            report = agent_preflight.audit_preflight(agent_id="codex", require_active_claim=True)

        self.assertTrue(report["errors"])
        self.assertIn("expected exactly one active work claim", report["errors"][0])

    def test_audit_preflight_flags_branch_mismatch(self) -> None:
        control_plane = {
            "prerelease_branch": "pulse/v6-release",
            "active_profile_id": "v6",
            "active_target_id": "v6-rc-stabilization",
        }
        status_payload = {"work_claims": []}

        with patch.object(agent_preflight, "active_control_plane", return_value=control_plane), patch.object(
            agent_preflight, "load_status_payload", return_value=status_payload
        ), patch.object(agent_preflight, "current_branch", return_value="main"):
            report = agent_preflight.audit_preflight(agent_id="codex", require_active_claim=False)

        self.assertTrue(report["errors"])
        self.assertIn("does not match active prerelease branch", report["errors"][0])


if __name__ == "__main__":
    unittest.main()
