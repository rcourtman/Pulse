from __future__ import annotations

from datetime import datetime, timezone
import unittest

from work_claim import claim_work, parse_args, release_claims


def base_payload() -> dict[str, object]:
    return {
        "lanes": [
            {"id": "L1", "repo_ids": ["pulse"]},
            {"id": "L2", "repo_ids": ["pulse-mobile"]},
        ],
        "lane_followups": [
            {"id": "followup-l1", "lane_ids": ["L1"], "repo_ids": ["pulse"]},
        ],
        "coverage_gaps": [
            {"id": "gap-l1", "lane_ids": ["L1"], "repo_ids": ["pulse"]},
        ],
        "candidate_lanes": [
            {
                "id": "candidate-l1",
                "target_id": "v6-product-lane-expansion",
                "current_lane_ids": ["L1"],
                "coverage_gap_ids": ["gap-l1"],
                "repo_ids": ["pulse"],
            }
        ],
        "readiness_assertions": [
            {"id": "RA1", "lane_ids": ["L1"], "repo_ids": ["pulse"]},
        ],
        "release_gates": [
            {"id": "g1", "lane_ids": ["L1"], "repo_ids": ["pulse"]},
        ],
        "open_decisions": [
            {"id": "d1", "lane_ids": ["L1"], "repo_ids": ["pulse"]},
        ],
        "work_claims": [],
    }


class WorkClaimTest(unittest.TestCase):
    def test_parse_args_accepts_claim_flags(self) -> None:
        args = parse_args(
            [
                "--kind",
                "candidate-lane",
                "--id",
                "candidate-l1",
                "--summary",
                "Advance the candidate lane.",
                "--agent-id",
                "codex",
                "--pretty",
            ]
        )
        self.assertEqual(args.kind, "candidate-lane")
        self.assertEqual(args.work_id, "candidate-l1")
        self.assertEqual(args.agent_id, "codex")
        self.assertTrue(args.pretty)

    def test_claim_work_replaces_same_agent_claim_and_prunes_expired(self) -> None:
        payload = base_payload()
        payload["work_claims"] = [
            {
                "id": "old-active",
                "agent_id": "codex",
                "summary": "Old lane claim.",
                "target_id": "v6-product-lane-expansion",
                "claimed_at": "2026-03-19T09:00:00Z",
                "heartbeat_at": "2026-03-19T09:15:00Z",
                "expires_at": "2026-03-19T11:00:00Z",
                "work_item": {"kind": "lane", "id": "L2"},
            },
            {
                "id": "expired-claim",
                "agent_id": "other-agent",
                "summary": "Expired gap claim.",
                "target_id": "v6-product-lane-expansion",
                "claimed_at": "2026-03-19T06:00:00Z",
                "heartbeat_at": "2026-03-19T06:15:00Z",
                "expires_at": "2026-03-19T07:00:00Z",
                "work_item": {"kind": "coverage-gap", "id": "gap-l1"},
            },
        ]

        result = claim_work(
            payload,
            work_kind="candidate-lane",
            work_id="candidate-l1",
            summary="Advance the candidate lane.",
            agent_id="codex",
            ttl_hours=2,
            now_utc=datetime(2026, 3, 19, 10, 0, tzinfo=timezone.utc),
        )

        self.assertEqual(result["action"], "claimed")
        self.assertEqual(result["claim"]["target_id"], "v6-product-lane-expansion")
        self.assertEqual(result["claim"]["work_item"], {"kind": "candidate-lane", "id": "candidate-l1"})
        self.assertEqual([claim["id"] for claim in result["replaced_claims"]], ["old-active"])
        self.assertEqual([claim["id"] for claim in result["pruned_expired_claims"]], ["expired-claim"])
        self.assertEqual(result["payload"]["work_claims"], [result["claim"]])

    def test_claim_work_renews_exact_same_claim(self) -> None:
        payload = base_payload()
        payload["work_claims"] = [
            {
                "id": "codex-candidate-lane-candidate-l1",
                "agent_id": "codex",
                "summary": "Old summary.",
                "target_id": "v6-product-lane-expansion",
                "claimed_at": "2026-03-19T09:00:00Z",
                "heartbeat_at": "2026-03-19T09:15:00Z",
                "expires_at": "2026-03-19T11:00:00Z",
                "work_item": {"kind": "candidate-lane", "id": "candidate-l1"},
            }
        ]

        result = claim_work(
            payload,
            work_kind="candidate-lane",
            work_id="candidate-l1",
            summary="Renewed summary.",
            agent_id="codex",
            ttl_hours=2,
            now_utc=datetime(2026, 3, 19, 10, 30, tzinfo=timezone.utc),
        )

        self.assertEqual(result["action"], "renewed")
        self.assertEqual(result["claim"]["id"], "codex-candidate-lane-candidate-l1")
        self.assertEqual(result["claim"]["claimed_at"], "2026-03-19T09:00:00Z")
        self.assertEqual(result["claim"]["heartbeat_at"], "2026-03-19T10:30:00Z")
        self.assertEqual(result["claim"]["expires_at"], "2026-03-19T12:30:00Z")
        self.assertEqual(result["claim"]["summary"], "Renewed summary.")

    def test_claim_work_rejects_existing_same_work_item_claim(self) -> None:
        payload = base_payload()
        payload["work_claims"] = [
            {
                "id": "other-agent-claim",
                "agent_id": "other-agent",
                "summary": "Already taking the candidate lane.",
                "target_id": "v6-product-lane-expansion",
                "claimed_at": "2026-03-19T09:00:00Z",
                "heartbeat_at": "2026-03-19T09:15:00Z",
                "expires_at": "2026-03-19T11:00:00Z",
                "work_item": {"kind": "candidate-lane", "id": "candidate-l1"},
            }
        ]

        with self.assertRaisesRegex(ValueError, "already reserves candidate-lane:candidate-l1"):
            claim_work(
                payload,
                work_kind="candidate-lane",
                work_id="candidate-l1",
                summary="Try to steal the candidate lane.",
                agent_id="codex",
                ttl_hours=2,
                now_utc=datetime(2026, 3, 19, 10, 0, tzinfo=timezone.utc),
            )

    def test_release_claims_removes_matching_agent_claim(self) -> None:
        payload = base_payload()
        payload["work_claims"] = [
            {
                "id": "codex-lane-L1",
                "agent_id": "codex",
                "summary": "Advance lane L1.",
                "target_id": "v6-product-lane-expansion",
                "claimed_at": "2026-03-19T09:00:00Z",
                "heartbeat_at": "2026-03-19T09:15:00Z",
                "expires_at": "2026-03-19T11:00:00Z",
                "work_item": {"kind": "lane", "id": "L1"},
            },
            {
                "id": "other-agent-lane-L2",
                "agent_id": "other-agent",
                "summary": "Advance lane L2.",
                "target_id": "v6-product-lane-expansion",
                "claimed_at": "2026-03-19T09:05:00Z",
                "heartbeat_at": "2026-03-19T09:15:00Z",
                "expires_at": "2026-03-19T11:00:00Z",
                "work_item": {"kind": "lane", "id": "L2"},
            },
        ]

        result = release_claims(
            payload,
            agent_id="codex",
            work_kind="lane",
            work_id="L1",
            now_utc=datetime(2026, 3, 19, 10, 0, tzinfo=timezone.utc),
        )

        self.assertEqual(result["action"], "released")
        self.assertEqual([claim["id"] for claim in result["released_claims"]], ["codex-lane-L1"])
        self.assertEqual(
            [claim["id"] for claim in result["payload"]["work_claims"]],
            ["other-agent-lane-L2"],
        )


if __name__ == "__main__":
    unittest.main()
