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
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/types.go`

## Shared Boundaries

1. None.

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
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/types.go`

## Shared Boundaries

1. None.

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
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own alert truth.

## Canonical Files

No list here.

## Shared Boundaries

1. None.

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
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/missing.go`

## Shared Boundaries

1. None.

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
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/types.go`

## Shared Boundaries

1. None.

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

    def test_audit_contract_payload_requires_declared_cross_subsystem_dependencies(self) -> None:
        registry_payload = {
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/"],
                    "owned_files": [],
                },
                {
                    "id": "unified-resources",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/unified-resources.md",
                    "owned_prefixes": ["internal/unifiedresources/"],
                    "owned_files": [],
                },
            ]
        }
        status_payload = {"lanes": [{"id": "L6"}]}
        contract_texts = {
            "docs/release-control/v6/subsystems/monitoring.md": """# Monitoring Contract

## Contract Metadata

```json
{
  "subsystem_id": "monitoring",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/monitoring.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own monitoring truth.

## Canonical Files

1. `internal/monitoring/monitor.go`
2. `internal/unifiedresources/views.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add typed read access through `internal/unifiedresources/views.go`

## Forbidden Paths

1. New ad hoc snapshot truth

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical monitoring still depends on unified read-state truth.
""",
            "docs/release-control/v6/subsystems/unified-resources.md": """# Unified Resources Contract

## Contract Metadata

```json
{
  "subsystem_id": "unified-resources",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/unified-resources.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own unified resource truth.

## Canonical Files

1. `internal/unifiedresources/views.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add view adapters through `internal/unifiedresources/`

## Forbidden Paths

1. New parallel runtime registries

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical read state owns the live view layer.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertIn(
            "docs/release-control/v6/subsystems/monitoring.md contract metadata dependency_subsystem_ids = [], want ['unified-resources']",
            "\n".join(report["errors"]),
        )

    def test_audit_contract_payload_rejects_stale_declared_dependencies(self) -> None:
        registry_payload = {
            "shared_ownerships": [],
            "subsystems": [
                {
                    "id": "alerts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/alerts.md",
                    "owned_prefixes": ["internal/alerts/"],
                    "owned_files": [],
                },
                {
                    "id": "monitoring",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/monitoring.md",
                    "owned_prefixes": ["internal/monitoring/"],
                    "owned_files": [],
                },
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
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": ["monitoring"]
}
```

## Purpose

Own alert truth.

## Canonical Files

1. `internal/alerts/specs/types.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add new alert rule kinds in `internal/alerts/specs/`

## Forbidden Paths

1. New ad hoc evaluator logic

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical alert identity is live runtime truth.
""",
            "docs/release-control/v6/subsystems/monitoring.md": """# Monitoring Contract

## Contract Metadata

```json
{
  "subsystem_id": "monitoring",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/monitoring.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own monitoring truth.

## Canonical Files

1. `internal/monitoring/monitor.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add pollers in `internal/monitoring/`

## Forbidden Paths

1. New snapshot-only paths

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical monitoring truth is still being consolidated.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertIn(
            "docs/release-control/v6/subsystems/alerts.md contract metadata dependency_subsystem_ids = ['monitoring'], want []",
            "\n".join(report["errors"]),
        )

    def test_audit_contract_payload_requires_shared_boundary_entries_from_registry(self) -> None:
        registry_payload = {
            "shared_ownerships": [
                {
                    "path": "internal/api/resources.go",
                    "rationale": "shared api resource boundary",
                    "subsystems": ["api-contracts", "unified-resources"],
                }
            ],
            "subsystems": [
                {
                    "id": "api-contracts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/api-contracts.md",
                    "owned_prefixes": ["internal/api/"],
                    "owned_files": [],
                },
                {
                    "id": "unified-resources",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/unified-resources.md",
                    "owned_prefixes": ["internal/unifiedresources/"],
                    "owned_files": ["internal/api/resources.go"],
                },
            ],
        }
        status_payload = {"lanes": [{"id": "L6"}]}
        contract_texts = {
            "docs/release-control/v6/subsystems/api-contracts.md": """# API Contracts

## Contract Metadata

```json
{
  "subsystem_id": "api-contracts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/api-contracts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own API truth.

## Canonical Files

1. `internal/api/resources.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add resource payload fields in `internal/api/resources.go`

## Forbidden Paths

1. New handler-local payload drift

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical API resource payloads are live.
""",
            "docs/release-control/v6/subsystems/unified-resources.md": """# Unified Resources Contract

## Contract Metadata

```json
{
  "subsystem_id": "unified-resources",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/unified-resources.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own unified resource truth.

## Canonical Files

1. `internal/unifiedresources/views.go`

## Shared Boundaries

1. `internal/api/resources.go` shared with `api-contracts`: shared api resource boundary.

## Extension Points

1. Add resource adapters in `internal/unifiedresources/`

## Forbidden Paths

1. New parallel runtime registries

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical read state owns the live view layer.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertIn(
            "docs/release-control/v6/subsystems/api-contracts.md section '## Shared Boundaries' paths = [], want ['internal/api/resources.go']",
            "\n".join(report["errors"]),
        )

    def test_audit_contract_payload_rejects_shared_boundary_without_partner_marker(self) -> None:
        registry_payload = {
            "shared_ownerships": [
                {
                    "path": "internal/api/resources.go",
                    "rationale": "shared api resource boundary",
                    "subsystems": ["api-contracts", "unified-resources"],
                }
            ],
            "subsystems": [
                {
                    "id": "api-contracts",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/api-contracts.md",
                    "owned_prefixes": ["internal/api/"],
                    "owned_files": [],
                },
                {
                    "id": "unified-resources",
                    "lane": "L6",
                    "contract": "docs/release-control/v6/subsystems/unified-resources.md",
                    "owned_prefixes": ["internal/unifiedresources/"],
                    "owned_files": ["internal/api/resources.go"],
                },
            ],
        }
        status_payload = {"lanes": [{"id": "L6"}]}
        contract_texts = {
            "docs/release-control/v6/subsystems/api-contracts.md": """# API Contracts

## Contract Metadata

```json
{
  "subsystem_id": "api-contracts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/api-contracts.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own API truth.

## Canonical Files

1. `internal/api/resources.go`

## Shared Boundaries

1. `internal/api/resources.go` shared with unified-resources: shared api resource boundary.

## Extension Points

1. Add resource payload fields in `internal/api/resources.go`

## Forbidden Paths

1. New handler-local payload drift

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical API resource payloads are live.
""",
            "docs/release-control/v6/subsystems/unified-resources.md": """# Unified Resources Contract

## Contract Metadata

```json
{
  "subsystem_id": "unified-resources",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/unified-resources.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own unified resource truth.

## Canonical Files

1. `internal/unifiedresources/views.go`

## Shared Boundaries

1. `internal/api/resources.go` shared with `api-contracts`: shared api resource boundary.

## Extension Points

1. Add resource adapters in `internal/unifiedresources/`

## Forbidden Paths

1. New parallel runtime registries

## Completion Obligations

1. Update contract and tests together

## Current State

Canonical read state owns the live view layer.
""",
        }

        report = audit_contract_payload(
            registry_payload=registry_payload,
            status_payload=status_payload,
            contract_texts=contract_texts,
        )

        self.assertIn(
            "docs/release-control/v6/subsystems/api-contracts.md shared boundary 'internal/api/resources.go' must mention partner subsystem 'unified-resources' in backticks",
            "\n".join(report["errors"]),
        )


if __name__ == "__main__":
    unittest.main()
