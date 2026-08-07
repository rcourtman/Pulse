# Action approval readiness and replacement-plan proof — 2026-08-07

## Decision

Pulse now applies every non-mutating dispatch-readiness gate before persisting a human approval and repeats the same check at dispatch. An operator may still reject a pending action when readiness is false. Expired or drifted plans can be replaced, but the replacement receives a new action identity and plan hash and must be reviewed and approved independently.

## Invariants

- Approval is not durable unless expiry, dry-run policy, emergency stop, executor wiring, resource/capability freshness, resource remediation policy, and executor-owned live availability all pass.
- A failed readiness check leaves the action pending and does not append an approval or lifecycle event.
- Executor-owned refusal reason and remediation remain exact through the lifecycle, API, and review dialog.
- Refresh is bound to the reviewed plan hash and is allowed only for expired or drifted plans.
- Patrol refresh reconstructs current policy inputs and the trusted Patrol service actor while preserving finding, investigation, and evidence origin.
- Refresh never copies approval and never mutates the old action into a new plan.
- Execution success alone does not become `fix_verified`; existing independent-evidence rules remain authoritative.

## Governed surfaces

- `internal/actionlifecycle/service.go`
- `internal/api/actions.go`
- `internal/api/patrol_action_broker.go`
- `internal/api/router_routes_monitoring.go`
- `internal/mutationregistry/manifest.json`
- `frontend-modern/src/features/actions/ActionReviewDialog.tsx`
- subsystem contracts for API, unified resources, AI runtime, agent lifecycle, and storage recovery

## Proof

- Backend lifecycle tests cover readiness loss before approval, rejection while blocked, no audit mutation, expired/drifted replacement identity, origin preservation, and refresh idempotency.
- API tests cover exact readiness errors, pending-state preservation, Patrol policy/origin reconstruction, plan-hash binding, stable agent error codes, mutation inventory, and route inventory.
- Frontend tests cover API refresh binding, exact refusal presentation, rejection availability, replacement handoff, and structured API error detail.
- Browser verification at 1440×1000 confirms the exact command-agent unblock, Reject present, and Approve absent.
- Browser verification at 390×844 confirms no document overflow and reachable Close, Refresh plan, and Reject controls.
- The browser refresh interaction returns a fresh ready replacement, displays the re-review notice, removes Refresh plan, and only then displays Approve.
- `status_audit.py --check` and `registry_audit.py --check` validate the final governed state.

The command results for the accepted commit are recorded in the task handoff and `frontend-modern/browser-verification.json` records the user-visible browser proof.
