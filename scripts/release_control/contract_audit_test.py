import unittest

from contract_audit import audit_contract_payload


class ContractAuditTest(unittest.TestCase):
    def test_audit_contract_payload_accepts_valid_contracts(self) -> None:
        registry_payload = {
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                }
            ]
        }
        status_payload = {
            "lanes": [
                {"id": "L6"},
            ]
        }
        contract_texts = {
            "docs/release-control/v6/subsystems/alerts.md": """# Alerts Contract

## Contract Metadata

```json
{
  "subsystem_id": "alerts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/alerts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json"
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/types.go`

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`

## Forbidden Paths

1. New ad hoc evaluator logic

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical alert identity is live runtime truth.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertEqual(report["errors"], [])
        self.assertEqual(report["summary"]["contract_count"], 1)
        self.assertEqual(report["contracts"][0]["subsystem_id"], "alerts")

    def test_audit_contract_payload_rejects_metadata_mismatch(self) -> None:
        registry_payload = {
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                }
            ]
        }
        status_payload = {
            "lanes": [
                {"id": "L6"},
            ]
        }
        contract_texts = {
            "docs/release-control/v6/subsystems/alerts.md": """# Alerts Contract

## Contract Metadata

```json
{
  "subsystem_id": "monitoring",
  "lane": "L3",
  "contract_file": "docs/release-control/v6/subsystems/monitoring.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json"
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/types.go`

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`

## Forbidden Paths

1. New ad hoc evaluator logic

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical alert identity is live runtime truth.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        joined = "\n".join(report["errors"])
        self.assertIn("contract metadata subsystem_id = 'monitoring', want 'alerts'", joined)
        self.assertIn("contract metadata lane = 'L3', want 'L6'", joined)
        self.assertIn(
            "contract metadata contract_file = 'docs/release-control/v6/subsystems/monitoring.md', want 'docs/release-control/v6/subsystems/alerts.md'",
            joined,
        )

    def test_audit_contract_payload_rejects_empty_required_list_section(self) -> None:
        registry_payload = {
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                }
            ]
        }
        status_payload = {
            "lanes": [
                {"id": "L6"},
            ]
        }
        contract_texts = {
            "docs/release-control/v6/subsystems/alerts.md": """# Alerts Contract

## Contract Metadata

```json
{
  "subsystem_id": "alerts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/alerts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json"
}
```

## Purpose

Own alert truth.

## Canonical Files

No list here.

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`

## Forbidden Paths

1. New ad hoc evaluator logic

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical alert identity is live runtime truth.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertIn(
            "docs/release-control/v6/subsystems/alerts.md section '## Canonical Files' must contain a numbered list",
            "\n".join(report["errors"]),
        )

    def test_audit_contract_payload_rejects_missing_canonical_file_reference(self) -> None:
        registry_payload = {
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                }
            ]
        }
        status_payload = {"lanes": [{"id": "L6"}]}
        contract_texts = {
            "docs/release-control/v6/subsystems/alerts.md": """# Alerts Contract

## Contract Metadata

```json
{
  "subsystem_id": "alerts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/alerts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json"
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/missing.go`

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`

## Forbidden Paths

1. New ad hoc evaluator logic

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical alert identity is live runtime truth.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertIn(
            "docs/release-control/v6/subsystems/alerts.md ## Canonical Files references missing path 'internal/alerts/specs/missing.go'",
            "\n".join(report["errors"]),
        )

    def test_audit_contract_payload_rejects_missing_extension_point_reference(self) -> None:
        registry_payload = {
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                }
            ]
        }
        status_payload = {"lanes": [{"id": "L6"}]}
        contract_texts = {
            "docs/release-control/v6/subsystems/alerts.md": """# Alerts Contract

## Contract Metadata

```json
{
  "subsystem_id": "alerts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/alerts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json"
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/types.go`

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/missing/`

## Forbidden Paths

1. New ad hoc evaluator logic

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical alert identity is live runtime truth.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertIn(
            "docs/release-control/v6/subsystems/alerts.md ## Extension Points references missing path 'internal/alerts/specs/missing/'",
            "\n".join(report["errors"]),
        )


if __name__ == "__main__":
    unittest.main()
