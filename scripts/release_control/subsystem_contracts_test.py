import unittest
from unittest.mock import patch
from pathlib import Path
import subprocess
import tempfile

from subsystem_contracts import (
    contract_reference_matches_path,
    load_contract_graph,
    parse_contract_text,
    referenced_contracts_for_path,
    tracked_contract_files,
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

## Shared Boundaries

1. `internal/shared/runtime.go` shared with `other-subsystem`: shared runtime boundary.

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
                {"heading": "## Shared Boundaries", "path": "internal/shared/runtime.go"},
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

## Shared Boundaries

1. `internal/shared/runtime.go` shared with `other-subsystem`: shared runtime boundary.

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

        shared_matches = referenced_contracts_for_path("internal/shared/runtime.go", contract_graph)
        self.assertEqual(len(shared_matches), 1)
        self.assertEqual(shared_matches[0]["subsystem_id"], "example")
        self.assertEqual(
            shared_matches[0]["matched_references"],
            [
                {"heading": "## Shared Boundaries", "path": "internal/shared/runtime.go"},
            ],
        )

    def test_tracked_contract_files_can_read_staged_contract_content(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            contracts_dir = repo_root / "docs" / "release-control" / "v6" / "subsystems"
            contracts_dir.mkdir(parents=True, exist_ok=True)
            contract_path = contracts_dir / "example.md"
            contract_rel = "docs/release-control/v6/subsystems/example.md"

            subprocess.run(["git", "init"], cwd=repo_root, check=True, capture_output=True, text=True)
            contract_path.write_text("# staged version\n", encoding="utf-8")
            subprocess.run(["git", "add", contract_rel], cwd=repo_root, check=True, capture_output=True, text=True)
            contract_path.write_text("# working tree version\n", encoding="utf-8")

            with (
                patch("subsystem_contracts.REPO_ROOT", repo_root),
                patch("subsystem_contracts.CONTRACTS_DIR", contracts_dir),
            ):
                self.assertEqual(tracked_contract_files()[contract_rel], "# working tree version\n")
                self.assertEqual(tracked_contract_files(staged=True)[contract_rel], "# staged version\n")


if __name__ == "__main__":
    unittest.main()
