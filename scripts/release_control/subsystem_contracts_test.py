import unittest

from subsystem_contracts import (
    contract_reference_matches_path,
    load_contract_graph,
    parse_contract_text,
    referenced_contracts_for_path,
)


class SubsystemContractsTest(unittest.TestCase):
    def test_parse_contract_text_extracts_metadata_and_path_references(self) -> None:
        parsed, errors = parse_contract_text(
            "docs/release-control/v6/subsystems/example.md",
            """# Example Contract

## Contract Metadata

```json
{
  "subsystem_id": "example"
}
```

## Purpose

Own example truth.

## Canonical Files

1. `internal/example/runtime.go`

## Extension Points

1. Add new adapters in `internal/example/`

## Forbidden Paths

1. None

## Completion Obligations

1. Update contract

## Current State

Stable.
""",
        )

        self.assertEqual(errors, [])
        self.assertEqual(parsed["metadata"]["subsystem_id"], "example")
        self.assertEqual(
            parsed["path_references"],
            [
                {"heading": "## Canonical Files", "path": "internal/example/runtime.go"},
                {"heading": "## Extension Points", "path": "internal/example/"},
            ],
        )

    def test_contract_reference_matches_path_supports_prefixes(self) -> None:
        self.assertTrue(contract_reference_matches_path("internal/example/", "internal/example/runtime.go"))
        self.assertFalse(contract_reference_matches_path("internal/example/", "internal/other/runtime.go"))
        self.assertTrue(contract_reference_matches_path("internal/example/runtime.go", "internal/example/runtime.go"))
        self.assertFalse(contract_reference_matches_path("internal/example/runtime.go", "internal/example/other.go"))

    def test_referenced_contracts_for_path_returns_matching_subsystem(self) -> None:
        contract_graph = load_contract_graph(
            {
                "docs/release-control/v6/subsystems/example.md": """# Example Contract

## Contract Metadata

```json
{
  "subsystem_id": "example"
}
```

## Purpose

Own example truth.

## Canonical Files

1. `internal/example/runtime.go`

## Extension Points

1. Add new adapters in `internal/example/`

## Forbidden Paths

1. None

## Completion Obligations

1. Update contract

## Current State

Stable.
"""
            }
        )

        matches = referenced_contracts_for_path("internal/example/runtime.go", contract_graph)
        self.assertEqual(len(matches), 1)
        self.assertEqual(matches[0]["subsystem_id"], "example")
        self.assertEqual(
            matches[0]["matched_references"],
            [
                {"heading": "## Canonical Files", "path": "internal/example/runtime.go"},
                {"heading": "## Extension Points", "path": "internal/example/"},
            ],
        )


if __name__ == "__main__":
    unittest.main()
