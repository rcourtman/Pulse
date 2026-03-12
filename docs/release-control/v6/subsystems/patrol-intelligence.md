# Patrol Intelligence Contract

## Contract Metadata

```json
{
  "subsystem_id": "patrol-intelligence",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/patrol-intelligence.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts"
  ]
}
```

## Purpose

Own the Patrol intelligence page, its local state orchestration, findings and
approval presentation, run-history rendering, and Patrol-specific presentation
helpers.

## Canonical Files

1. `frontend-modern/src/pages/AIIntelligence.tsx`
2. `frontend-modern/src/stores/aiIntelligence.ts`
3. `frontend-modern/src/types/aiIntelligence.ts`
4. `frontend-modern/src/components/AI/FindingsPanel.tsx`
5. `frontend-modern/src/components/Brand/PulsePatrolLogo.tsx`
6. `frontend-modern/src/components/patrol/`
7. `frontend-modern/src/utils/aiFindingPresentation.ts`
8. `frontend-modern/src/utils/approvalRiskPresentation.ts`
9. `frontend-modern/src/utils/findingAlertIdentity.ts`
10. `frontend-modern/src/utils/patrolEmptyStatePresentation.ts`
11. `frontend-modern/src/utils/patrolFormat.ts`
12. `frontend-modern/src/utils/patrolRunPresentation.ts`
13. `frontend-modern/src/utils/patrolSummaryPresentation.ts`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change Patrol page orchestration through `frontend-modern/src/pages/AIIntelligence.tsx` and `frontend-modern/src/stores/aiIntelligence.ts`
2. Add or change Patrol findings, approvals, investigation, or run-history presentation through `frontend-modern/src/components/AI/FindingsPanel.tsx` and `frontend-modern/src/components/patrol/`
3. Keep Patrol transport and payload changes aligned through `frontend-modern/src/api/ai.ts` and `frontend-modern/src/api/patrol.ts`

## Forbidden Paths

1. Reintroducing Patrol finding, investigation, approval, or run-history copy directly inside page components when canonical Patrol presentation helpers already own it
2. Duplicating Patrol finding severity, lifecycle, alert-identity, or approval-risk derivation outside the governed Patrol presentation helpers
3. Letting the Patrol page, local store, and findings UI drift into separate shadow truths for the same Patrol status or finding lifecycle state

## Completion Obligations

1. Update Patrol page, state, presentation helpers, and proof files together when Patrol UX semantics change
2. Keep Patrol-specific copy and badge logic inside the governed Patrol presentation helpers instead of page-local branches
3. Update this contract whenever a new Patrol-specific page, store, helper, or presentation component becomes canonical runtime surface area

## Current State

The Patrol page, store, findings UI, and run-history presentation had been
outside the governed subsystem map even though they are the top-level runtime
surface for Patrol intelligence. This contract now owns that orchestration and
presentation boundary while leaving transport and payload-shape ownership in
`api-contracts`.
