import unittest
from datetime import datetime, timezone
from unittest.mock import patch

from work_claim import apply_claim, build_work_claim, parse_args, reserve_claim


class WorkClaimTest(unittest.TestCase):
    def test_parse_args_accepts_write_and_replace(self) -> None:
        args = parse_args(
            [
                "--kind",
                "lane",
                "--id",
                "L14",
                "--summary",
                "Tighten trust proof routing.",
                "--agent-id",
                "codex-gpt5",
                "--replace-claim-id",
                "claim-old",
                "--write",
                "--pretty",
            ]
        )
        self.assertEqual(args.kind, "lane")
        self.assertEqual(args.id, "L14")
        self.assertEqual(args.replace_claim_id, ["claim-old"])
        self.assertTrue(args.write)
        self.assertTrue(args.pretty)

    def test_build_work_claim_uses_active_target_shape(self) -> None:
        claim = build_work_claim(
            work_kind="lane",
            work_id="L14",
            summary="Tighten trust proof routing.",
            agent_id="codex-gpt5",
            target_id="v6-rc-stabilization",
            now_utc=datetime(2026, 3, 13, 18, 0, 0, tzinfo=timezone.utc),
            duration_hours=2,
        )
        self.assertEqual(claim["target_id"], "v6-rc-stabilization")
        self.assertEqual(claim["work_item"], {"kind": "lane", "id": "L14"})
        self.assertEqual(claim["claimed_at"], "2026-03-13T18:00:00Z")
        self.assertEqual(claim["heartbeat_at"], "2026-03-13T18:00:00Z")
        self.assertEqual(claim["expires_at"], "2026-03-13T20:00:00Z")

    def test_apply_claim_replaces_and_sorts(self) -> None:
        payload = {
            "work_claims": [
                {
                    "id": "claim-b",
                    "claimed_at": "2026-03-13T10:00:00Z",
                },
                {
                    "id": "claim-a",
                    "claimed_at": "2026-03-13T09:00:00Z",
                },
            ]
        }
        claim = {
            "id": "claim-c",
            "claimed_at": "2026-03-13T08:00:00Z",
        }
        updated = apply_claim(payload, claim, replace_claim_ids=["claim-b"])
        self.assertEqual([entry["id"] for entry in updated["work_claims"]], ["claim-c", "claim-a"])

    def test_reserve_claim_reports_new_audit_errors_only(self) -> None:
        payload = {"work_claims": [{"id": "claim-existing", "claimed_at": "2026-03-13T17:00:00Z"}]}
        baseline_report = {"errors": ["unchanged baseline error"]}
        updated_report = {
            "errors": [
                "unchanged baseline error",
                "active work claims ['claim-existing', 'claim-lane-l14-20260313180000'] overlap on lane:L14",
            ]
        }
        with patch("work_claim.audit_status_payload", side_effect=[baseline_report, updated_report]):
            claim, _updated, errors = reserve_claim(
                payload=payload,
                work_kind="lane",
                work_id="L14",
                summary="Tighten trust proof routing.",
                agent_id="codex-gpt5",
                target_id="v6-rc-stabilization",
                duration_hours=2,
                claim_id="claim-lane-l14-20260313180000",
                replace_claim_ids=[],
                now_utc=datetime(2026, 3, 13, 18, 0, 0, tzinfo=timezone.utc),
            )
        self.assertEqual(claim["id"], "claim-lane-l14-20260313180000")
        self.assertEqual(
            errors,
            ["active work claims ['claim-existing', 'claim-lane-l14-20260313180000'] overlap on lane:L14"],
        )


if __name__ == "__main__":
    unittest.main()
