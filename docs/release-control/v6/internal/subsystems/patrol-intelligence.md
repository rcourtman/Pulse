# Patrol Intelligence Contract

## Contract Metadata

```json
{
  "subsystem_id": "patrol-intelligence",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/patrol-intelligence.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
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
3. Keep Patrol transport and payload changes aligned through the governed AI runtime and API contract transport surfaces

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
presentation boundary while leaving shared transport and payload-shape
ownership in the governed AI runtime and API contract surfaces.

Patrol finding state must now also consume the canonical camelCase
`alertIdentifier` field and pending-approval expiry metadata end to end.
Frontend Patrol helpers may not keep shadow `alert_identifier` fallbacks or
drop `expiresAt` when deciding whether queued investigation fixes still need
operator attention.
Patrol intelligence seed context should also prefer the canonical
unified-resource timeline before falling back to the patrol-local change
detector so recent-change context stays aligned with the resource timeline
that powers the shared resource API.
Patrol-owned intelligence summaries should keep their recent-change counts
backed by the same canonical timeline when available instead of only counting
Patrol-local detector history.
Patrol-owned resource and global intelligence prompt contexts should also
render the canonical recent changes section before any patrol-local change
detector fallback so the prompt surface stays aligned with the shared
unified-resource timeline.
Those same Patrol-owned prompt contexts now also surface a canonical
resource-graph section from unified-resource relationships, so edge labels,
directionality, and provenance stay aligned with the shared graph model
instead of being reconstructed locally.
The Patrol page also now renders the canonical intelligence summary card
through the governed AI client and store, so the visible page summary and the
resource/timeline sections stay aligned on the same shared backend slice.
That Patrol summary card now also includes the canonical data-governance
posture snapshot from the shared AI summary payload, so the visible page can
show the same sensitivity, routing, and redaction distribution that the
runtime derives from unified resources.
The resource drawer now carries the same canonical posture snapshot through
the resource-intelligence payload, so the resource-level AI card can show the
org posture context without introducing a separate posture endpoint.
That same resource-intelligence payload also carries canonical dependency and
dependent graph context plus canonical correlation evidence, so the resource
drawer can surface relationship reachability and learned edge patterns
directly from the AI contract instead of inventing a second graph summary.
The Patrol intelligence page now also consumes the learned correlation list
from the canonical AI correlations endpoint through the shared
`frontend-modern/src/stores/aiIntelligence.ts` store, so the global summary
and the resource drawer both reflect the same learned edge evidence instead
of each page fetching its own copy.
That same store now owns the Patrol dashboard load bundle as well, so the
page refresh path stays aligned on a single orchestrated AI bundle instead of
repeating the individual summary, findings, approval, and correlation fetches
inline.
The Patrol intelligence page and resource drawer now also share the canonical
`frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
card, so the data-governance posture counts stay rendered from one governed
frontend component across both surfaces.
The Patrol page and resource drawer now also share the canonical
`frontend-modern/src/utils/resourceChangePresentation.ts` formatter so
recent-change kind and headline wording stays aligned wherever the canonical
timeline is surfaced.
The Patrol page and resource drawer now also share a frontend relationship
presentation helper for graph labels and provenance wording, so the same
canonical relationship semantics render consistently across the resource and
intelligence surfaces.
The backend Patrol and AI runtime summaries now also share
`internal/unifiedresources/change_presentation.go` for the canonical
change-kind and provenance mapping, so the same resource-model semantics
drive both the backend summaries and the frontend presentation helpers.
The canonical shared AI resource context now also surfaces policy routing and
redaction hints from unified resources, so the Patrol page and resource drawer
see the same governance posture that the runtime uses for export boundaries.
