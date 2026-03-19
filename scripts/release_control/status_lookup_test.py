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
    "coverage_gaps": [
        {
            "id": "resource-change-and-timeline",
            "summary": "Resource change and timeline are still under-modeled.",
            "status": "planned",
            "proposed_resolution": "lane-split",
            "coverage_impact": 15,
            "lane_ids": ["L6", "L13"],
            "repo_ids": ["pulse", "pulse-enterprise"],
            "subsystem_ids": ["alerts", "api-contracts", "monitoring", "unified-resources"],
        }
    ],
    "candidate_lanes": [
        {
            "id": "resource-change-intelligence",
            "summary": "Promote canonical resource relationships and cross-resource timelines into an explicit lane.",
            "status": "planned",
            "target_id": "v6-product-lane-expansion",
            "repo_ids": ["pulse", "pulse-enterprise"],
            "current_lane_ids": ["L6", "L13"],
            "subsystem_ids": ["alerts", "api-contracts", "monitoring", "unified-resources"],
            "coverage_gap_ids": ["resource-change-and-timeline"],
        }
    ],
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
    "candidate_lane_queue": [
        {
            "candidate_lane_id": "resource-change-intelligence",
            "rank": 1,
            "available": True,
            "available_rank": 1,
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

    def test_parse_args_accepts_candidate_lane_selector(self) -> None:
        args = parse_args(["--candidate-lane", "resource-change-intelligence", "--pretty"])
        self.assertEqual(args.candidate_lane, "resource-change-intelligence")
        self.assertTrue(args.pretty)

    def test_lookup_status_entry_returns_lane(self) -> None:
        result = lookup_status_entry(REPORT, kind="lane", entry_id="L16")
        self.assertEqual(result["entry"]["summary"], "Agent lifecycle and fleet operations.")
        self.assertEqual(result["control_plane"]["active_profile_id"], "v6")

    def test_lookup_status_entry_returns_candidate_lane(self) -> None:
        result = lookup_status_entry(REPORT, kind="candidate_lane", entry_id="resource-change-intelligence")
        self.assertEqual(result["entry"]["target_id"], "v6-product-lane-expansion")
        self.assertEqual(result["queue_item"]["rank"], 1)

    def test_lookup_status_entry_returns_coverage_gap(self) -> None:
        result = lookup_status_entry(REPORT, kind="coverage_gap", entry_id="resource-change-and-timeline")
        self.assertEqual(result["entry"]["coverage_impact"], 15)
        self.assertEqual(result["linked_candidate_lane_ids"], ["resource-change-intelligence"])

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

    def test_render_pretty_includes_candidate_lane_queue_details(self) -> None:
        result = lookup_status_entry(REPORT, kind="candidate_lane", entry_id="resource-change-intelligence")
        rendered = render_pretty(result)
        self.assertIn("candidate_lane resource-change-intelligence", rendered)
        self.assertIn("target=v6-product-lane-expansion", rendered)
        self.assertIn("queue: rank=1 available=True available_rank=1", rendered)

    def test_render_pretty_includes_coverage_gap_links(self) -> None:
        result = lookup_status_entry(REPORT, kind="coverage_gap", entry_id="resource-change-and-timeline")
        rendered = render_pretty(result)
        self.assertIn("resolution=lane-split impact=15", rendered)
        self.assertIn("linked_candidate_lanes=resource-change-intelligence", rendered)


if __name__ == "__main__":
    unittest.main()
