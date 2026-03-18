import unittest

from status_lookup import lookup_status_entry, parse_args, render_pretty


REPORT = {
    "control_plane": {
        "active_profile_id": "v6",
        "active_target": {
            "id": "v6-rc-stabilization",
            "kind": "stabilization",
        },
    },
    "summary": {
        "lane_count": 16,
    },
    "lanes": [
        {
            "id": "L16",
            "summary": "Agent lifecycle and fleet operations.",
            "gap": 2,
            "status": "partial",
            "completion_state": "open",
            "derived_status": "behind-target",
            "repo_ids": ["pulse"],
            "subsystem_ids": ["agent-lifecycle"],
            "evidence_count": 10,
            "completion_summary": "Still needs install-command and auto-register proof tightening.",
            "completion_tracking": [],
            "blockers": [],
        }
    ],
    "readiness_assertions": [
        {
            "id": "RA8",
            "summary": "Promotion rehearsal remains blocked.",
            "derived_status": "gates-pending",
            "blocking_level": "release-ready",
            "proof_type": "hybrid",
            "lane_ids": ["L1", "L9"],
            "subsystem_ids": [],
            "release_gate_ids": ["rc-to-ga-promotion-readiness"],
            "evidence_count": 7,
            "proof_command_count": 1,
            "highest_evidence_tier": "local-rehearsal",
            "minimum_evidence_tier": "real-external-e2e",
        }
    ],
    "release_gates": [
        {
            "id": "rc-to-ga-promotion-readiness",
            "summary": "RC-to-GA promotion still blocked.",
            "status": "blocked",
            "effective_status": "blocked",
            "blocking_level": "release-ready",
            "lane_ids": ["L1", "L9"],
            "repo_ids": ["pulse", "pulse-pro"],
            "subsystem_ids": [],
            "highest_evidence_tier": "local-rehearsal",
            "minimum_evidence_tier": "real-external-e2e",
        }
    ],
    "lane_followups": [
        {
            "id": "mobile-post-rc-hardening",
            "summary": "Mobile follow-up.",
            "status": "planned",
            "lane_ids": ["L5"],
            "repo_ids": ["pulse", "pulse-mobile"],
            "subsystem_ids": [],
        }
    ],
    "open_decisions": [],
    "resolved_decisions": [],
    "work_claims": [
        {
            "id": "codex-l16",
            "summary": "Tighten setup-script continuity.",
            "agent_id": "codex",
            "target_id": "v6-rc-stabilization",
            "claimed_at": "2026-03-14T12:00:00Z",
            "expires_at": "2026-03-14T14:00:00Z",
            "work_item": {"kind": "lane", "id": "L16"},
        }
    ],
}


class StatusLookupTest(unittest.TestCase):
    def test_parse_args_accepts_lane_selector(self) -> None:
        args = parse_args(["--lane", "L16", "--pretty"])
        self.assertEqual(args.lane, "L16")
        self.assertTrue(args.pretty)

    def test_lookup_status_entry_returns_lane(self) -> None:
        result = lookup_status_entry(REPORT, kind="lane", entry_id="L16")
        self.assertEqual(result["entry"]["summary"], "Agent lifecycle and fleet operations.")
        self.assertEqual(result["control_plane"]["active_profile_id"], "v6")

    def test_lookup_status_entry_returns_release_gate(self) -> None:
        result = lookup_status_entry(
            REPORT,
            kind="release_gate",
            entry_id="rc-to-ga-promotion-readiness",
        )
        self.assertEqual(result["entry"]["effective_status"], "blocked")

    def test_lookup_status_entry_raises_for_missing_entry(self) -> None:
        with self.assertRaises(KeyError):
            lookup_status_entry(REPORT, kind="lane", entry_id="L99")

    def test_render_pretty_includes_lane_summary(self) -> None:
        result = lookup_status_entry(REPORT, kind="lane", entry_id="L16")
        rendered = render_pretty(result)
        self.assertIn("lane L16", rendered)
        self.assertIn("gap=2 status=partial", rendered)
        self.assertIn("completion_summary: Still needs install-command", rendered)

    def test_render_pretty_includes_work_claim_work_item(self) -> None:
        result = lookup_status_entry(REPORT, kind="work_claim", entry_id="codex-l16")
        rendered = render_pretty(result)
        self.assertIn("work=lane:L16", rendered)
        self.assertIn("agent=codex", rendered)


if __name__ == "__main__":
    unittest.main()
