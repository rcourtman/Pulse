# Legacy Host Classification Audit

Date: 2026-03-05
Scope: `pulse` repo only, focused on v6-facing paths and release verification artifacts.

## Verdict

No additional active v6 runtime leaks were found after fixing the agent-registration journey.

The remaining `host` references in v6-adjacent code fall into three intentional buckets:

1. Compatibility boundaries that explicitly reject or normalize legacy input.
2. Internal state or wire-format shims that still bridge older model names to v6-facing output.
3. Non-resource semantics where `host` means hostname, SSH host key, backup type, or endpoint host.

## Must Remove

None found in active v6 runtime/read surfaces during this pass.

The previously-found leak is now fixed:

- `tests/integration/tests/journeys/04-agent-install-registration.spec.ts`
- `tests/integration/evals/tasks/agent-registration.md`

Those artifacts now require unified `resources[]` and `agent.type = "unified"`.

## Intentional Compatibility Boundaries

These are correct to keep because they harden v6 against old clients or old persisted inputs:

- `internal/api/resources_test.go`
  Confirms `/api/resources?type=host` is rejected.
- `internal/api/org_handlers_test.go`
  Confirms organization sharing rejects `resourceType: "host"`.
- `internal/api/ai_handler_test.go`
  Confirms legacy chat mention aliases like `host`, `container`, and `k8s` are dropped.
- `internal/api/ai_handlers_test.go`
  Confirms `target_type: "host"` is rejected for run-command normalization.
- `internal/monitoring/monitor_helpers.go`
  Keeps explicit legacy-agent detection: only `agent.type = "unified"` is canonical.

## Intentional Internal Shims

These are still legacy-shaped internally, but they are not evidence of v6 model leakage:

- `internal/monitoring/monitor.go`
  `monitorLegacyResourceType()` is the frontend wire-shape mapper for `/api/state` broadcast payloads.
  It emits canonical v6-facing types like `agent`, `node`, and `docker-host`.
- `internal/models/state_snapshot.go`
  `StateSnapshot` and `ResolveResource()` still model host-agent state for producer/wire compatibility.
  This is not the canonical read model; `ReadState` and unified resources are.

## Non-Resource Host Terminology

These are unrelated to the removed v5 resource type and should not be treated as migration debt:

- `internal/mock/generator.go`
  Backup payloads use `Type: "host"` to mean PMG host config backups, not unified resources.
- `internal/monitoring/knownhosts.go`
  SSH known-hosts management.
- Node configuration and endpoint code using `host` as a network address or URL host field.

## Ratchets Added

- `internal/unifiedresources/code_standards_test.go`
  Added `TestV6AgentRegistrationArtifactsStayCanonical` to prevent regressions in:
  - `tests/integration/tests/journeys/04-agent-install-registration.spec.ts`
  - `tests/integration/evals/tasks/agent-registration.md`

The ratchet bans:

- `state.hosts`
- `hosts array`
- `type: 'host'`
- `agent.type = "host"`

And requires:

- unified `resources[]`
- unified agent marker `type: 'unified'` / `agent.type = "unified"`
