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


if __name__ == "__main__":
    unittest.main()
