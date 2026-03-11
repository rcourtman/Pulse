import unittest
from pathlib import Path

from canonical_completion_guard import REPO_ROOT
from subsystem_lookup import lookup_paths


class SubsystemLookupTest(unittest.TestCase):
    def test_lookup_paths_reports_multiple_subsystems_for_shared_runtime_file(self) -> None:
        result = lookup_paths(["internal/api/resources.go"])
        impacted = {entry["subsystem"] for entry in result["impacted_subsystems"]}
        self.assertEqual(impacted, {"api-contracts", "unified-resources"})
        self.assertEqual(result["status_audit_errors"], [])
        self.assertEqual(result["status_summary"]["lane_count"], 12)

        file_entry = result["files"][0]
        matches = {match["subsystem"] for match in file_entry["matches"]}
        self.assertEqual(matches, {"api-contracts", "unified-resources"})
        for match in file_entry["matches"]:
            self.assertEqual(match["lane_context"]["lane_id"], "L6")
            self.assertEqual(match["lane_context"]["lane"]["id"], "L6")

    def test_lookup_paths_classifies_tests_without_runtime_matches(self) -> None:
        result = lookup_paths(["internal/api/contract_test.go"])
        self.assertEqual(result["files"][0]["classification"], "test-or-fixture")
        self.assertEqual(result["files"][0]["matches"], [])

    def test_lookup_paths_reports_unowned_runtime_files(self) -> None:
        result = lookup_paths(["README.md"])
        self.assertEqual(result["unowned_runtime_files"], ["README.md"])

    def test_lookup_paths_normalizes_absolute_repo_paths(self) -> None:
        absolute = str(Path(REPO_ROOT, "internal/api/resources.go"))
        result = lookup_paths([absolute])
        self.assertEqual(result["files"][0]["path"], "internal/api/resources.go")

    def test_lookup_paths_classifies_governance_files_as_ignored(self) -> None:
        result = lookup_paths(["docs/release-control/v6/status.json"])
        self.assertEqual(result["files"][0]["classification"], "ignored")
        self.assertEqual(result["files"][0]["matches"], [])

    def test_lookup_paths_includes_relevant_open_decisions_for_lane(self) -> None:
        result = lookup_paths(["pkg/licensing/features.go"])
        match = next(
            item
            for item in result["files"][0]["matches"]
            if item["subsystem"] == "cloud-paid"
        )
        lane_context = match["lane_context"]
        self.assertEqual(lane_context["lane_id"], "L3")
        self.assertEqual(
            {decision["id"] for decision in lane_context["open_decisions"]},
            {"cloud-msp-stripe-prices", "cloud-msp-price-id-propagation"},
        )

    def test_lookup_paths_keeps_cross_cutting_resolved_decisions_for_lane(self) -> None:
        result = lookup_paths(["internal/monitoring/monitor.go"])
        match = next(
            item
            for item in result["files"][0]["matches"]
            if item["subsystem"] == "monitoring"
        )
        lane_context = match["lane_context"]
        self.assertEqual(lane_context["lane_id"], "L6")
        self.assertEqual(
            {decision["id"] for decision in lane_context["resolved_decisions"]},
            {"host-type-migration-boundary-audit", "orchestrator-retired", "top-level-governance-split"},
        )


if __name__ == "__main__":
    unittest.main()
