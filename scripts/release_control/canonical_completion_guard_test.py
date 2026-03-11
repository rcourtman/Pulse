import unittest

from canonical_completion_guard import (
    SUBSYSTEM_REGISTRY,
    build_verification_requirements,
    infer_impacted_subsystems,
    load_subsystem_rules,
    staged_verification_files_for_requirement,
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
            self.assertIn("path_policies", rule["verification"])

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
                        "path_policies": [
                            {
                                "id": "metrics-hot-path",
                                "label": "monitoring metrics hot-path proof",
                                "match_prefixes": [],
                                "match_files": ["internal/monitoring/monitor_metrics.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": [
                                    "internal/monitoring/monitor_metrics_chart_batch_bench_test.go",
                                    "internal/monitoring/monitor_metrics_slo_test.go",
                                ],
                            }
                        ],
                    },
                    "verification_requirements": [
                        {
                            "id": "default",
                            "label": "default subsystem verification",
                            "touched_runtime_files": ["internal/monitoring/monitor.go"],
                            "allow_same_subsystem_tests": True,
                            "test_prefixes": [],
                            "exact_files": ["internal/unifiedresources/code_standards_test.go"],
                        }
                    ],
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
                        "path_policies": [
                            {
                                "id": "backend-payload-contracts",
                                "label": "backend API payload proof",
                                "match_prefixes": ["internal/api/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                                "exact_files": [
                                    "internal/api/contract_test.go",
                                    "frontend-modern/src/types/api.ts",
                                ],
                            },
                            {
                                "id": "frontend-api-clients",
                                "label": "frontend API client proof",
                                "match_prefixes": ["frontend-modern/src/api/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                                "exact_files": ["frontend-modern/src/types/api.ts"],
                            },
                            {
                                "id": "frontend-api-types",
                                "label": "frontend API type sync proof",
                                "match_prefixes": [],
                                "match_files": ["frontend-modern/src/types/api.ts"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                                "exact_files": ["frontend-modern/src/types/api.ts"],
                            },
                        ],
                    },
                    "verification_requirements": [
                        {
                            "id": "backend-payload-contracts",
                            "label": "backend API payload proof",
                            "touched_runtime_files": ["internal/api/resources.go"],
                            "allow_same_subsystem_tests": False,
                            "test_prefixes": ["frontend-modern/src/api/__tests__/"],
                            "exact_files": [
                                "internal/api/contract_test.go",
                                "frontend-modern/src/types/api.ts",
                            ],
                        }
                    ],
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
                        "path_policies": [
                            {
                                "id": "identity-canonicalization",
                                "label": "canonical identity proof",
                                "match_prefixes": [],
                                "match_files": [
                                    "internal/unifiedresources/canonical_identity.go",
                                    "internal/unifiedresources/identity.go",
                                    "internal/unifiedresources/ids.go",
                                    "internal/unifiedresources/legacy_aliases.go",
                                    "internal/unifiedresources/physical_disk_ids.go",
                                    "internal/unifiedresources/types.go",
                                ],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": [
                                    "internal/unifiedresources/canonical_ids_types_test.go",
                                    "internal/unifiedresources/canonical_identity_test.go",
                                    "internal/unifiedresources/code_standards_test.go",
                                    "internal/unifiedresources/identity_test.go",
                                    "internal/unifiedresources/ids_test.go",
                                ],
                            },
                            {
                                "id": "read-state-views",
                                "label": "read-state and adapter proof",
                                "match_prefixes": [],
                                "match_files": [
                                    "internal/unifiedresources/adapters.go",
                                    "internal/unifiedresources/monitor_adapter.go",
                                    "internal/unifiedresources/read_state.go",
                                    "internal/unifiedresources/views.go",
                                ],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": [
                                    "frontend-modern/src/stores/__tests__/websocket-unified.test.ts",
                                    "internal/unifiedresources/adapter_coverage_test.go",
                                    "internal/unifiedresources/adapters_test.go",
                                    "internal/unifiedresources/monitor_adapter_read_state_test.go",
                                    "internal/unifiedresources/views_test.go",
                                ],
                            },
                        ],
                    },
                    "verification_requirements": [
                        {
                            "id": "default",
                            "label": "default subsystem verification",
                            "touched_runtime_files": ["internal/api/resources.go"],
                            "allow_same_subsystem_tests": True,
                            "test_prefixes": [],
                            "exact_files": [
                                "internal/unifiedresources/code_standards_test.go",
                                "frontend-modern/src/stores/__tests__/websocket-unified.test.ts",
                            ],
                        }
                    ],
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
        requirement = build_verification_requirements(
            monitoring_rule,
            ["internal/monitoring/monitor.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            monitoring_rule,
            requirement,
            ["internal/unifiedresources/code_standards_test.go"],
        )
        self.assertEqual(matches, ["internal/unifiedresources/code_standards_test.go"])

    def test_same_subsystem_test_counts_when_registry_allows_it(self):
        monitoring_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "monitoring")
        requirement = build_verification_requirements(
            monitoring_rule,
            ["internal/monitoring/monitor.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            monitoring_rule,
            requirement,
            ["internal/monitoring/monitor_extra_coverage_test.go"],
        )
        self.assertEqual(matches, ["internal/monitoring/monitor_extra_coverage_test.go"])

    def test_api_contracts_reject_arbitrary_same_subsystem_test(self):
        api_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        requirement = build_verification_requirements(
            api_rule,
            ["internal/api/resources.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            api_rule,
            requirement,
            ["internal/api/stripe_webhook_handlers_test.go"],
        )
        self.assertEqual(matches, [])

    def test_api_contracts_accept_allowed_test_prefix(self):
        api_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        requirement = build_verification_requirements(
            api_rule,
            ["internal/api/resources.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            api_rule,
            requirement,
            ["frontend-modern/src/api/__tests__/alerts.test.ts"],
        )
        self.assertEqual(matches, ["frontend-modern/src/api/__tests__/alerts.test.ts"])

    def test_performance_rejects_non_performance_test(self):
        perf_rule = next(
            rule for rule in load_subsystem_rules() if rule["id"] == "performance-and-scalability"
        )
        requirement = build_verification_requirements(
            perf_rule,
            ["pkg/metrics/store.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            perf_rule,
            requirement,
            ["internal/api/router_test.go"],
        )
        self.assertEqual(matches, [])

    def test_monitoring_metrics_hot_path_uses_specific_proof_policy(self):
        monitoring_rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "monitoring")
        requirements = build_verification_requirements(
            monitoring_rule,
            ["internal/monitoring/monitor_metrics.go"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "metrics-hot-path",
                    "label": "monitoring metrics hot-path proof",
                    "touched_runtime_files": ["internal/monitoring/monitor_metrics.go"],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "internal/monitoring/monitor_metrics_chart_batch_bench_test.go",
                        "internal/monitoring/monitor_metrics_slo_test.go",
                    ],
                }
            ],
        )

    def test_frontend_primitive_page_controls_uses_specific_guardrails(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "frontend-primitives")
        requirements = build_verification_requirements(
            rule,
            ["frontend-modern/src/components/shared/PageControls.tsx"],
        )
        self.assertEqual(
            requirements,
            [
                {
                    "id": "page-controls-and-filters",
                    "label": "page controls and filter proof",
                    "touched_runtime_files": [
                        "frontend-modern/src/components/shared/PageControls.tsx"
                    ],
                    "allow_same_subsystem_tests": False,
                    "test_prefixes": [],
                    "exact_files": [
                        "frontend-modern/src/components/shared/FilterToolbar.test.tsx",
                        "frontend-modern/src/components/shared/PageControls.guardrails.test.ts",
                        "frontend-modern/src/components/shared/PageControls.test.tsx",
                    ],
                }
            ],
        )

    def test_api_backend_runtime_can_use_types_file_as_proof(self):
        rule = next(rule for rule in load_subsystem_rules() if rule["id"] == "api-contracts")
        requirement = build_verification_requirements(
            rule,
            ["internal/api/security.go"],
        )[0]
        matches = staged_verification_files_for_requirement(
            rule,
            requirement,
            ["frontend-modern/src/types/api.ts"],
        )
        self.assertEqual(matches, ["frontend-modern/src/types/api.ts"])


if __name__ == "__main__":
    unittest.main()
