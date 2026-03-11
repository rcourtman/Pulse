# Subsystem Contract Template

Use this template for any new major subsystem that needs a canonical contract.

## Contract Metadata

```json
{
  "subsystem_id": "example-subsystem",
  "lane": "L0",
  "contract_file": "docs/release-control/v6/subsystems/example-subsystem.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

State what this subsystem owns and what it explicitly does not own.

## Canonical Files

List the files that contain the subsystem truth.

## Extension Points

List the only approved places to extend the subsystem.

## Forbidden Paths

List the patterns and files that future work must not use.

## Completion Obligations

List what must be updated when the subsystem changes.

## Current State

Record the current migration/end-state summary in a few lines.
