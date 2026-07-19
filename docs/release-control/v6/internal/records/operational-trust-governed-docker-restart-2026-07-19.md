# Operational Trust: Governed Docker Restart

Date: 2026-07-19
Specification:
`docs/release-control/v6/internal/OPERATIONAL_TRUST_IMPLEMENTATION_SPEC.md`
Phase: 5, governed actions and verification
Candidate: `protection-posture-attention-queue`

## Decision

The first Operational Trust mutation is the existing declared Docker container
`restart` capability, offered only from a canonical
`docker-container-health` operational record.

Pulse does not create a Patrol-local executor. The attention surface projects
eligibility into the existing action lifecycle, then the shared Actions review
owns approval, execution, audit, receipt, and verification. The action origin
binds the exact operational record and evidence IDs. The stable request ID is
derived from the record and capability, so repeated planning and execution
replay the same durable action rather than sending the mutation twice.

The offer fails closed unless all of the following remain true:

1. the lifecycle record is open or acknowledged
2. every contributing evidence envelope is fresh, complete, confirmed,
   sufficiently permitted, and bound unambiguously to the same canonical
   container
3. the current unified resource still declares the exact `restart` capability,
   admin approval floor, and Docker lifecycle handler
4. the live executor reports the capability ready
5. the current operator can plan, approve, and execute the action

An existing origin-bound action remains reviewable as durable history after the
operational record resolves, subject to current authorization.

## Verification Boundary

The Docker executor performs its existing typed readback after the provider
accepts the restart. That readback may confirm or contradict the action
postcondition, and its trust class remains visible. A successful command or
confirmed running-state readback does not close the operational issue.

Only a later detector-owned, fresh healthy observation may transition the
canonical operational record to resolved. The Patrol detail therefore
distinguishes:

- pending execution or verification
- confirmed restart postcondition, with the issue still open
- contradicted postcondition, with the issue still open
- inconclusive verification, with the issue still open

Provider timeout, callback loss, and server restart retain the durable dispatch
attempt and reconcile a correlated receipt without redispatch.

## User Lens

Operator job:

> This container is unhealthy. Show me the one safe action Pulse can actually
> perform, let me review it before anything is sent, and tell me whether it
> worked without pretending the original health problem is fixed.

Live deterministic exercise:

1. Open Patrol from the monitor-first shell.
2. Select the unhealthy API container.
3. Read the impact, fresh evidence posture, bounded restart postcondition, and
   explicit approval boundary in the selected detail.
4. Open the shared governed-action review.
5. Expand planning-time policy evidence.
6. Approve, perform a second deliberate Run action, and close the review.
7. Read either confirmed or contradicted verification beside the still-open
   operational issue.

Distance to the answer is one Patrol navigation, one item selection, and one
action review. The default queue carries no mutation button. The selected
detail contains one bounded action; exact policy, approval, delivery, and
verification forensics remain in the shared review.

Keep / demote / cut:

- Keep one eligible safe action beside the selected issue.
- Keep the expected postcondition and approval warning before opening review.
- Keep execution versus verification truth explicit after the action.
- Demote policy authorities, exact audit identity, delivery receipt, and raw
  verification evidence to the shared review.
- Cut actions for stale, partial, permission-limited, ambiguous, unsupported,
  or currently unavailable evidence.
- Cut provider success as a synonym for operational recovery.
- Cut duplicate execute controls and any Patrol-local action lifecycle.

Vocabulary:

- `Review and approve` states the next operator decision.
- `Review action` opens durable history after a plan exists.
- `Postcondition confirmed` says what the action proved; it does not say the
  service is healthy.
- `Issue stays open until fresh health evidence` explains why the queue item
  remains.

Verdict: `product`. The flow is bounded, deliberate, auditable, responsive,
keyboard operable, and honest about both failed verification and the separate
detector-owned recovery boundary.

## User and Comparative Evidence

- [#1034: Docker container start/stop/restart option](https://github.com/rcourtman/Pulse/issues/1034)
  directly requests a container restart control in Pulse. Operational Trust
  places it in selected issue context rather than adding an unaudited hover
  mutation.
- [#1564: failed one-click container update can leave a service stopped](https://github.com/rcourtman/Pulse/issues/1564)
  demonstrates why provider mutation success, rollback posture, visible
  failure, and postcondition truth must remain distinct.
- [#1586: command execution token rejected despite command-enabled setup](https://github.com/rcourtman/Pulse/issues/1586)
  demonstrates that current executor readiness and authorization must be
  checked before an action is offered.
- [Docker Engine API: restart a container](https://docs.docker.com/reference/api/engine/version/v1.46/#tag/Container/operation/ContainerRestart)
  defines restart as a narrow container-scoped provider operation and returns
  transport/provider acceptance, not proof of application health.
- [Docker Engine API](https://docs.docker.com/reference/api/engine/) is
  versioned and exposes separate inspect/event state, supporting a readback
  boundary rather than treating the restart response as verification.
- [Kubernetes probes](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
  likewise separate container lifecycle state from readiness and workload
  health. Pulse keeps the same distinction even though this first capability is
  Docker-specific.

The selected capability is intentionally smaller than update/recreate,
package-cache cleanup, or broad autonomous remediation. It directly answers a
public request while fitting the already-declared capability, approval,
delivery, receipt, and verification contracts.

## Runtime and API Result

- `internal/ai/attention_actions.go` owns the pure eligibility and offer
  projection.
- `internal/api/attention_actions.go` owns bounded enrichment and
  `POST /api/ai/patrol/attention/{id}/actions/restart/plan`.
- Planning re-evaluates lifecycle evidence, resource capability, live executor
  readiness, and operator authority immediately before calling the canonical
  action lifecycle.
- `ActionOrigin` additively carries `operationalRecordId` and sorted,
  deduplicated `evidenceIds`.
- Memory and SQLite stores expose bounded latest-action lookup by operational
  record. SQLite uses an indexed JSON-origin query; a 200-record batch is one
  store read.
- List enrichment happens after pagination. Detail enriches only the selected
  item. Summary performs no action enrichment.
- The browser client can request only the fixed zero-parameter plan. It cannot
  supply origin, target, evidence IDs, handler, actor, or authority.
- Existing action plan/decision/execute/detail routes and
  `ActionReviewDialog` remain the only approval and execution lifecycle.

## Proof

Backend contract and integration:

```text
go test ./internal/actionlifecycle ./internal/actionplanner \
  ./internal/unifiedresources ./internal/ai ./internal/api -count=1
```

Focused proofs cover every fail-closed eligibility reason, origin/evidence
binding, slash-containing record IDs, plan replay, approval, exactly-once
execution, confirmed and contradicted verification, action-without-resolution,
fresh detector recovery, timeout after send, correlated late receipt, and
SQLite restart reconciliation without resend.

Frontend:

```text
npm run type-check
npm run test -- \
  src/features/patrol/__tests__/PatrolAttentionWorkbench.test.tsx \
  src/features/actions/__tests__/ActionReviewDialog.test.tsx
npx eslint \
  src/api/patrolAttention.ts \
  src/features/patrol/PatrolAttentionWorkbench.tsx \
  src/features/patrol/__tests__/PatrolAttentionWorkbench.test.tsx
```

Browser:

```text
PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 \
  npx playwright test \
  tests/91-operational-trust-attention-workbench.spec.ts \
  --project=chromium

PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 \
  npx playwright test \
  tests/91-operational-trust-attention-workbench.spec.ts \
  --project=mobile-chrome
```

Both five-journey matrices pass. The governed-action journeys prove policy
disclosure, approval, exactly one execute request, confirmed and contradicted
verification copy, still-open lifecycle truth, focus restoration after the
detail node is refreshed, screen-reader names, 390-pixel layout, no document
overflow, and reduced motion.

## Remaining Specification Work

This record accepts Phase 5 only. It does not close the overall Operational
Trust goal or primary candidate. Phase 6 still owns the complete compatibility,
migration, concurrency, failure, retention, load, telemetry, documentation,
upgrade, and final cross-repository governance matrix required by all fourteen
completion criteria.
