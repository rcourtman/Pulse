import unittest

from canonical_completion_guard import infer_required_contracts


class CanonicalCompletionGuardTest(unittest.TestCase):
    def test_monitoring_runtime_change_requires_monitoring_contract(self):
        required = infer_required_contracts(["internal/monitoring/monitor.go"])
        self.assertEqual(
            required,
            {
                "docs/release-control/v6/subsystems/monitoring.md": [
                    "internal/monitoring/monitor.go"
                ]
            },
        )

    def test_unified_resource_api_change_requires_two_contracts(self):
        required = infer_required_contracts(["internal/api/resources.go"])
        self.assertEqual(
            required,
            {
                "docs/release-control/v6/subsystems/api-contracts.md": [
                    "internal/api/resources.go"
                ],
                "docs/release-control/v6/subsystems/unified-resources.md": [
                    "internal/api/resources.go"
                ],
            },
        )

    def test_test_only_changes_do_not_require_contract_updates(self):
        required = infer_required_contracts(
            [
                "internal/monitoring/monitor_extra_coverage_test.go",
                "frontend-modern/src/components/Alerts/__tests__/ThresholdsTable.test.tsx",
            ]
        )
        self.assertEqual(required, {})


if __name__ == "__main__":
    unittest.main()
