# Pulse Intelligence Proactive Operations Lane

Date: 2026-05-08
Owner: L23 Monitor-first Patrol operations
Evidence tier: local-rehearsal

## Decision

The `pulse-intelligence-proactive-operations-product-gap` coverage gap is resolved by adding L23 as a first-class v6 lane.

Pulse Intelligence is not generic Assistant chat. The governed product contract is:

- Patrol owns structured detection, readiness, investigation, and run-history records.
- Assistant explains and collaborates from safe backend-owned handoff context rehydrated from those records.
- Approval, action audit, resource policy, and resource timeline contracts remain canonical product prerequisites for any governed action.
- Browser-authored handoff payloads are hints only; backend and subsystem contracts own the durable context.

## Evidence

The current floor is carried across the existing subsystem contracts and proof surfaces:

- `docs/AI.md`
- `docs/release-control/v6/internal/subsystems/ai-runtime.md`
- `docs/release-control/v6/internal/subsystems/api-contracts.md`
- `docs/release-control/v6/internal/subsystems/patrol-intelligence.md`
- `docs/release-control/v6/internal/subsystems/security-privacy.md`
- `docs/release-control/v6/internal/subsystems/unified-resources.md`
- `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
- `frontend-modern/src/components/AI/FindingsPanel.tsx`
- `frontend-modern/src/components/AI/Chat/index.tsx`
- `internal/ai/patrol_assistant_handoff.go`
- `internal/api/ai_handler.go`
- `internal/api/contract_test.go`

## Result

`status.json` now models monitor-first Patrol operations as L23 instead of
leaving either the proactive-operations floor or the completed Operational
Trust workbench expansion in candidate queues. Future agents should pick
concrete L23 slices directly when strengthening the attention lifecycle,
Patrol-to-Assistant explanation boundary, approval, governed action, or
verification contract.
