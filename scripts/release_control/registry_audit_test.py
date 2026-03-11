import unittest

from registry_audit import audit_registry_payload


class RegistryAuditTest(unittest.TestCase):
    def test_audit_registry_payload_accepts_valid_minimal_registry(self) -> None:
        payload = {
            "version": 10,
            "subsystems": [
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["internal/monitoring/canonical_guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "monitoring-runtime",
                                "label": "monitoring runtime proof",
                                "match_prefixes": ["internal/monitoring/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/monitoring/canonical_guardrails_test.go"],
                            }
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/subsystems/monitoring.md",
            "internal/monitoring/monitor.go",
            "internal/monitoring/canonical_guardrails_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        self.assertEqual(report["errors"], [])
        self.assertEqual(report["summary"]["subsystem_count"], 1)
        self.assertEqual(report["subsystems"][0]["default_fallback_count"], 0)

    def test_audit_registry_payload_flags_unknown_lane_and_missing_contract(self) -> None:
        payload = {
            "version": 10,
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L99",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                    "owned_prefixes": ["internal/alerts/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": [],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [],
                    },
                }
            ],
        }

        report = audit_registry_payload(payload, tracked_files={"internal/alerts/unified_eval.go"}, status_lane_ids={"L6"})

        self.assertTrue(report["errors"])
        error_text = "\n".join(report["errors"])
        self.assertIn("unknown status lane", error_text)
        self.assertIn("missing tracked file", error_text)

    def test_audit_registry_payload_flags_explicit_coverage_gap(self) -> None:
        payload = {
            "version": 10,
            "subsystems": [
                {
                    "id": "cloud-paid",
                    "lane": "L3",
                    "contract": "docs/release-control/v6/subsystems/cloud-paid.md",
                    "owned_prefixes": ["pkg/licensing/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["pkg/licensing/cloud_paid_guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "runtime-entitlement-surface",
                                "label": "runtime entitlement surface proof",
                                "match_prefixes": [],
                                "match_files": ["pkg/licensing/evaluator.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["pkg/licensing/cloud_paid_guardrails_test.go"],
                            }
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/subsystems/cloud-paid.md",
            "pkg/licensing/evaluator.go",
            "pkg/licensing/features.go",
            "pkg/licensing/cloud_paid_guardrails_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L3"})

        self.assertTrue(report["errors"])
        self.assertIn("falls back to default verification", "\n".join(report["errors"]))

    def test_audit_registry_payload_requires_explicit_coverage_flag_true(self) -> None:
        payload = {
            "version": 10,
            "subsystems": [
                {
                    "id": "api-contracts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/api-contracts.md",
                    "owned_prefixes": ["internal/api/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": ["internal/api/contract_test.go"],
                        "require_explicit_path_policy_coverage": False,
                        "path_policies": [
                            {
                                "id": "backend-payload-contracts",
                                "label": "backend API payload proof",
                                "match_prefixes": ["internal/api/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/api/contract_test.go"],
                            }
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/subsystems/api-contracts.md",
            "internal/api/alerts.go",
            "internal/api/contract_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        self.assertTrue(report["errors"])
        self.assertIn(
            "require_explicit_path_policy_coverage must be true",
            "\n".join(report["errors"]),
        )

    def test_audit_registry_payload_rejects_uncanonical_ordering(self) -> None:
        payload = {
            "version": 10,
            "subsystems": [
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/", "frontend/monitoring/"],
                    "owned_files": ["internal/monitoring/b.go", "internal/monitoring/a.go"],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": ["internal/monitoring/tests/", "frontend/tests/"],
                        "exact_files": [
                            "internal/monitoring/z_test.go",
                            "internal/monitoring/a_test.go",
                        ],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "monitoring-runtime",
                                "label": "monitoring runtime proof",
                                "match_prefixes": ["internal/monitoring/", "frontend/monitoring/"],
                                "match_files": ["internal/monitoring/b.go", "internal/monitoring/a.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": ["internal/monitoring/tests/", "frontend/tests/"],
                                "exact_files": [
                                    "internal/monitoring/z_test.go",
                                    "internal/monitoring/a_test.go",
                                ],
                            }
                        ],
                    },
                },
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                    "owned_prefixes": ["internal/alerts/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["internal/alerts/guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "alerts-runtime",
                                "label": "alerts runtime proof",
                                "match_prefixes": ["internal/alerts/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/alerts/guardrails_test.go"],
                            }
                        ],
                    },
                },
            ],
        }
        tracked_files = {
            "docs/release-control/v6/subsystems/alerts.md",
            "docs/release-control/v6/subsystems/monitoring.md",
            "frontend/tests/example.test.ts",
            "frontend/monitoring/ui.go",
            "internal/alerts/alert.go",
            "internal/alerts/guardrails_test.go",
            "internal/monitoring/a.go",
            "internal/monitoring/a_test.go",
            "internal/monitoring/b.go",
            "internal/monitoring/z_test.go",
            "internal/monitoring/tests/example_test.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        joined = "\n".join(report["errors"])
        self.assertIn("registry.json subsystems must be sorted by subsystem id", joined)
        self.assertIn("subsystems[0].owned_prefixes must be sorted lexicographically", joined)
        self.assertIn("subsystems[0].owned_files must be sorted lexicographically", joined)
        self.assertIn("subsystems[0].verification.test_prefixes must be sorted lexicographically", joined)
        self.assertIn("subsystems[0].verification.exact_files must be sorted lexicographically", joined)
        self.assertIn(
            "subsystems[0].verification.path_policies[0].match_prefixes must be sorted lexicographically",
            joined,
        )
        self.assertIn(
            "subsystems[0].verification.path_policies[0].match_files must be sorted lexicographically",
            joined,
        )
        self.assertIn(
            "subsystems[0].verification.path_policies[0].test_prefixes must be sorted lexicographically",
            joined,
        )
        self.assertIn(
            "subsystems[0].verification.path_policies[0].exact_files must be sorted lexicographically",
            joined,
        )

    def test_audit_registry_payload_rejects_fully_shadowed_path_policy(self) -> None:
        payload = {
            "version": 10,
            "subsystems": [
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/"],
                    "owned_files": [],
                    "verification": {
                        "allow_same_subsystem_tests": True,
                        "test_prefixes": [],
                        "exact_files": ["internal/monitoring/guardrails_test.go"],
                        "require_explicit_path_policy_coverage": True,
                        "path_policies": [
                            {
                                "id": "all-monitoring",
                                "label": "all monitoring proof",
                                "match_prefixes": ["internal/monitoring/"],
                                "match_files": [],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/monitoring/guardrails_test.go"],
                            },
                            {
                                "id": "specific-file",
                                "label": "specific file proof",
                                "match_prefixes": [],
                                "match_files": ["internal/monitoring/monitor.go"],
                                "allow_same_subsystem_tests": False,
                                "test_prefixes": [],
                                "exact_files": ["internal/monitoring/guardrails_test.go"],
                            },
                        ],
                    },
                }
            ],
        }
        tracked_files = {
            "docs/release-control/v6/subsystems/monitoring.md",
            "internal/monitoring/guardrails_test.go",
            "internal/monitoring/monitor.go",
        }

        report = audit_registry_payload(payload, tracked_files=tracked_files, status_lane_ids={"L6"})

        self.assertIn(
            "subsystems[0].verification.path_policies[1] is unreachable because earlier path policies already match all owned runtime files",
            "\n".join(report["errors"]),
        )


if __name__ == "__main__":
    unittest.main()
