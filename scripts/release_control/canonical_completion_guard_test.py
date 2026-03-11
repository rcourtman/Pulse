import unittest

from canonical_completion_guard import (
    SUBSYSTEM_REGISTRY,
    infer_impacted_subsystems,
    load_subsystem_rules,
    staged_verification_files_for_subsystem,
)


class CanonicalCompletionGuardTest(unittest.TestCase):
    def test_registry_exists_and_contains_required_subsystems(self):
        self.assertTrue(SUBSYSTEM_REGISTRY.exists())
        rules = load_subsystem_rules()
        ids = {rule["id"] for rule in rules}
        self.assertEqual(
            ids,
            {
                "alerts",
                "monitoring",
                "unified-resources",
                "cloud-paid",
                "api-contracts",
                "frontend-primitives",
                "performance-and-scalability",
            },
        )
        for rule in rules:
            self.assertIn("verification", rule)
            self.assertIn("allow_same_subsystem_tests", rule["verification"])
            self.assertIn("test_prefixes", rule["verification"])
            self.assertIn("exact_files", rule["verification"])

    def test_monitoring_runtime_change_requires_monitoring_contract(self):
        required = infer_impacted_subsystems(["internal/monitoring/monitor.go"])
        self.assertEqual(
            required,
            {
                "monitoring": {
                    "id": "monitoring",
                    "contract": "docs/release-control/v6/subsystems/monitoring.md",
                    "touched_runtime_files": ["internal/monitoring/monitor.go"],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["internal/unifiedresources/code_standards_test.go"],
                    },
                }
            },
        )

    def test_unified_resource_api_change_requires_two_contracts(self):
        required = infer_impacted_subsystems(["internal/api/resources.go"])
        self.assertEqual(
            required,
            {
                "api-contracts": {
                    "id": "api-contracts",
                    "contract": "docs/release-control/v6/subsystems/api-contracts.md",
                    "touched_runtime_files": ["internal/api/resources.go"],
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                        "exact_files": ["internal/api/contract_test.go"],
                    },
                },
                "unified-resources": {
                    "id": "unified-resources",
                    "contract": "docs/release-control/v6/subsystems/unified-resources.md",
                    "touched_runtime_files": ["internal/api/resources.go"],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": [
                            "internal/unifiedresources/code_standards_test.go",
                            "frontend-modern/src/stores/__tests__/websocket-unified.test.ts",
                        ],
                    },
                },
            },
        )

    def test_test_only_changes_do_not_require_contract_updates(self):
        required = infer_impacted_subsystems(
            [
                "internal/monitoring/monitor_extra_coverage_test.go",
                "frontend-modern/src/components/Alerts/__tests__/ThresholdsTable.test.tsx",
            ]
        )
        self.assertEqual(required, {})

    def test_exact_verification_file_counts_as_verification_artifact(self):
        monitoring_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "monitoring")
        matches = staged_verification_files_for_subsystem(
            monitoring_rule,
            ["internal/unifiedresources/code_standards_test.go"],
        )
        self.assertEqual(matches, ["internal/unifiedresources/code_standards_test.go"])

    def test_same_subsystem_test_counts_when_registry_allows_it(self):
        monitoring_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "monitoring")
        matches = staged_verification_files_for_subsystem(
            monitoring_rule,
            ["internal/monitoring/monitor_extra_coverage_test.go"],
        )
        self.assertEqual(matches, ["internal/monitoring/monitor_extra_coverage_test.go"])

    def test_api_contracts_reject_arbitrary_same_subsystem_test(self):
        api_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        matches = staged_verification_files_for_subsystem(
            api_rule,
            ["internal/api/stripe_webhook_handlers_test.go"],
        )
        self.assertEqual(matches, [])

    def test_api_contracts_accept_allowed_test_prefix(self):
        api_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        matches = staged_verification_files_for_subsystem(
            api_rule,
            ["frontend-modern/src/api/__tests__/alerts.test.ts"],
        )
        self.assertEqual(matches, ["frontend-modern/src/api/__tests__/alerts.test.ts"])

    def test_performance_rejects_non_performance_test(self):
        perf_rule = next(
            rule for rule in load_subsystem_rules() if rule["id"] == "performance-and-scalability"
        )
        matches = staged_verification_files_for_subsystem(
            perf_rule,
            ["internal/api/router_test.go"],
        )
        self.assertEqual(matches, [])


if __name__ == "__main__":
    unittest.main()
