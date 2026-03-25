# Patrol Intelligence Contract

## Contract Metadata

```json
{
  "subsystem_id": "patrol-intelligence",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/patrol-intelligence.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": ["frontend-primitives"]
}
```

## Purpose

Own the Patrol intelligence route shell, feature surface, local state
orchestration, findings and approval presentation, run-history rendering, and
Patrol-specific presentation helpers.

## Canonical Files

1. `frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx`
2. `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`
3. `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`
4. `frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`
5. `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`
6. `frontend-modern/src/features/patrol/PatrolIntelligenceWorkspace.tsx`
7. `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
8. `frontend-modern/src/pages/AIIntelligence.tsx`
9. `frontend-modern/src/stores/aiIntelligence.ts`
10. `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts`
11. `frontend-modern/src/types/aiIntelligence.ts`
12. `frontend-modern/src/components/AI/FindingsPanel.tsx`
13. `frontend-modern/src/components/Brand/PulsePatrolLogo.tsx`
14. `frontend-modern/src/components/patrol/`
15. `frontend-modern/src/utils/aiFindingPresentation.ts`
16. `frontend-modern/src/utils/approvalRiskPresentation.ts`
17. `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts`
18. `frontend-modern/src/utils/aiQuickstartPresentation.ts`
19. `frontend-modern/src/utils/findingAlertIdentity.ts`
20. `frontend-modern/src/utils/remediationPresentation.ts`
21. `frontend-modern/src/utils/patrolEmptyStatePresentation.ts`
22. `frontend-modern/src/utils/patrolFormat.ts`
23. `frontend-modern/src/utils/patrolRunPresentation.ts`
24. `frontend-modern/src/utils/patrolSummaryPresentation.ts`
25. `frontend-modern/src/utils/patrolRuntimePresentation.ts`
26. `frontend-modern/src/utils/textPresentation.ts`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change Patrol page orchestration through `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`, keep `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts` as the canonical investigation-context derivation owner, keep `frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx` as the feature shell, keep the Patrol-owned section files under `frontend-modern/src/features/patrol/` as the heavy render owners, keep `frontend-modern/src/pages/AIIntelligence.tsx` as the route shell, keep `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts` as the canonical AI summary normalization owner, and update `frontend-modern/src/stores/aiIntelligence.ts` together
2. Add or change Patrol findings, approvals, investigation, or run-history presentation through `frontend-modern/src/components/AI/FindingsPanel.tsx` and `frontend-modern/src/components/patrol/`
3. Keep remediation execution badge copy and severity styling aligned through `frontend-modern/src/components/patrol/RemediationStatus.tsx` and `frontend-modern/src/utils/remediationPresentation.ts`
4. Add or change Patrol header, summary, or status runtime-state presentation through `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`, `frontend-modern/src/components/patrol/PatrolStatusBar.tsx`, and `frontend-modern/src/utils/patrolRuntimePresentation.ts`
5. Add or change Patrol header quickstart-credit or schedule presentation through `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, `frontend-modern/src/utils/aiQuickstartPresentation.ts`, and `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts`
   Patrol must not show the exhausted quickstart-credit warning while runtime state is active on a non-quickstart provider path; that warning belongs only to active quickstart usage or a blocked quickstart-exhausted runtime.
6. Keep Patrol and chat identifier-label presentation aligned through the shared `frontend-modern/src/utils/textPresentation.ts`
7. Keep Patrol and chat stream-matching / mention dedupe aligned through the shared `frontend-modern/src/utils/chatIdentifiers.ts`
8. Keep Patrol transport and payload changes aligned through the governed AI runtime and API contract transport surfaces

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

The route file `frontend-modern/src/pages/AIIntelligence.tsx` is now also a
thin shell that delegates to the feature-owned
`frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx`, so Patrol
runtime state and presentation no longer accumulate directly in the route
component itself.
The feature surface now also keeps the same shell/runtime split internally:
`frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx` owns feature
composition, the Patrol-owned section files under
`frontend-modern/src/features/patrol/` own the header, banner, summary, and
workspace render surfaces, and
`frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts` owns Patrol
state, transport, polling, and effect lifecycle. The shell and section surfaces
must not re-accumulate Patrol API calls, timer orchestration, or store refresh
semantics, and `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts` now
owns the canonical summary normalization so Patrol consumers inherit one
governed recent-change and policy-posture snapshot instead of reintroducing
hook-local fallback logic.
The Patrol page now also treats Patrol runtime availability as a first-class
render contract: the header chip, primary summary card, and status bar must
all route through the shared `frontend-modern/src/utils/patrolRuntimePresentation.ts`
helper plus the backend `runtime_state` payload instead of inferring operator
state from the last healthy summary snapshot or run history alone.
That active-runtime label must stay operational rather than verdict-like: the
header chip should communicate that Patrol is enabled or available, not imply
that infrastructure health is currently good merely because the runtime is on.
That render rule now also has browser-level proof in
`tests/integration/tests/18-patrol-runtime-state.spec.ts`: when the backend
reports `runtime_state=blocked`, the real `/ai` route must show Patrol as
paused, keep the blocked reason visible, disable manual Patrol runs, and
suppress stale healthy summary headlines such as `Health A · 100/100` even if
the last summary payload still looks healthy.
That same Patrol-owned presentation rule also applies to the findings empty
state: `frontend-modern/src/components/AI/FindingsPanel.tsx` must not treat
`0 active findings` as equivalent to "your infrastructure looks healthy" when
the Patrol runtime is blocked, disabled, or unavailable, or when the canonical
overall-health summary is degraded or not fully verified. The green healthy
empty state belongs only to an actually healthy Patrol summary, while degraded
coverage or paused-runtime states must surface the governing warning/error copy
through `frontend-modern/src/utils/patrolEmptyStatePresentation.ts`.
That degraded empty-state copy must also interpret the finding state rather
than simply replaying the primary assessment sentence verbatim: when coverage
is incomplete, the findings panel should tell the operator that Patrol has not
surfaced active findings but that this is not a full all-clear, so the page
does not duplicate the summary prediction as if it were a second independent
status surface.
The Patrol summary surface must follow that same hierarchy. The primary summary
headline in `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`
should state Patrol's current assessment first, such as verified healthy,
issues detected, or coverage incomplete, with the health grade only as
supporting evidence. Secondary metric strips must not render `No issues found`
when the same governed overall-health summary says coverage is incomplete or
health still requires attention; that compact summary state now belongs to the
shared `frontend-modern/src/utils/patrolSummaryPresentation.ts` helper.
That same helper also owns the primary assessment explanation. The summary card
must not pair an `Issues detected` headline with a raw coverage-only
`overall_health.prediction` sentence from a separate source; when active
findings and incomplete verification are both true, the Patrol summary should
describe both in one canonical assessment message.
The summary recency chip must follow the same governed scope distinction. When
the latest completed activity was only a scoped run, the summary should label
that timestamp as `Last activity` instead of `Last patrol`; `Last full patrol`
belongs only to the most recent completed full Patrol run.
That same recency contract also applies to the header metadata row. The top
header must not revert to a generic `Last:` timestamp when the rest of Patrol
is explicitly distinguishing activity from full verification recency.
That summary surface must also avoid reintroducing a second compact assessment
or verification layer beneath the primary card. Supporting metric strips
belong to counts and outcomes such as active findings, critical findings,
warnings, and fixes; they must not repeat Patrol assessment labels or
verification labels in a second row that competes with the primary governed
assessment and verification copy above.
The same supporting-chip rule applies to timing: the primary summary card may
show health and active-finding support, but it should not add another recency
pill once header metadata, verification, and findings footer already carry the
governed activity/verification timestamps.
That same summary surface must also explain what Patrol actually verified.
Recent run history should drive a visible verification summary that tells the
operator whether Patrol recently completed a successful full patrol, only ran
scoped alert-triggered checks, or ended its most recent full patrol with
errors, so the page does not leave trust and coverage as implicit background
knowledge.
The Patrol status bar should stay factual and operational within that same
hierarchy. `frontend-modern/src/components/patrol/PatrolStatusBar.tsx` is a
recent-activity strip, not a second health verdict: when Patrol is active it
should report recent run activity, run kind, latest run result, and run-count
context instead of emitting another green/amber all-clear headline that can
drift from the governed assessment summary above it.
That recent-activity copy also has to remain intelligible in compact or
plain-text renders: the latest-run segment must keep an explicit textual
separator between run kind and result, so degraded entries read as
`Scoped run · error` rather than collapsing into concatenated strings like
`Scoped runerror`.
That same Patrol-owned timing contract also applies to the findings empty
state footer. `frontend-modern/src/components/AI/FindingsPanel.tsx` must use
the canonical Patrol countdown semantics for `next_patrol_at` instead of
formatting future schedule timestamps through generic relative-time helpers;
otherwise the findings footer can contradict the header by rendering the same
next scheduled patrol as `just now` while the main Patrol shell correctly
shows a multi-hour countdown.
That footer must also use the canonical Patrol recency label rather than a
generic `Last:` prefix, so scoped-only recent activity is rendered as
`Last activity` and does not silently revert to patrol/full-verification
language in the findings surface.
When Patrol is currently running, that strip should still stay factual rather
than switching to another verdict label: the runtime may add an explicit
in-progress indicator, but the primary activity label remains recent activity
instead of a competing Patrol-state verdict.
That latest run result must come from the effective run outcome, not the raw
status field alone: shared Patrol run presentation helpers must treat any run
with execution errors as an error result even when the stored status text is
still `healthy`, so run history and header activity surfaces do not present a
false green outcome for an incomplete Patrol execution.
hook-local fallback logic.
The Patrol header now also has explicit helper ownership for its quickstart and
schedule presentation. `frontend-modern/src/utils/aiQuickstartPresentation.ts`
and `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts` are the
canonical owners for quickstart-credit messaging and Patrol interval option
labeling used by `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`;
future Patrol header work should extend those helpers instead of rebuilding
credit badges or schedule labels inline in the header shell. That same header
surface must now gate exhausted-credit messaging on actual runtime usage:
Patrol may only show the quickstart exhausted warning when `using_quickstart`
is true or when the Patrol runtime is explicitly blocked by quickstart
exhaustion, not merely because the stored credit counter reached zero while a
configured provider path keeps Patrol active.
`frontend-modern/src/utils/remediationPresentation.ts` is now also the
canonical owner for remediation result badge copy and success/failure styling
used by `frontend-modern/src/components/patrol/RemediationStatus.tsx`, so
Patrol execution feedback does not drift into inline message branches inside
the component.

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
The Patrol-backed `/api/ai/intelligence/changes` endpoint should also read
through the canonical intelligence facade first and only fall back to the
local detector when the unified timeline is unavailable, so the API payload
stays aligned with the same governed recent-change source.
Patrol-owned resource and global intelligence prompt contexts should also
render the canonical recent changes section before any patrol-local change
detector fallback so the prompt surface stays aligned with the shared
unified-resource timeline.
When that detector fallback is used, the Patrol runtime must render recent
changes through the shared memory presentation helper so the same heading,
resource prefixing, and change labels are reused across the Patrol and AI
fallback paths.
`memory.ChangeDetector.GetChangesSummary` now also delegates to that shared
helper, so the detector-owned summary API and the Patrol fallback prompt path
stay aligned on the same markdown shape.
Those same Patrol-owned prompt contexts now also surface a canonical
relationship section from unified-resource relationships, so edge labels,
directionality, and provenance stay aligned with the shared relationship model
instead of being reconstructed locally.
That relationship section is now rendered by the shared
`internal/unifiedresources.FormatResourceRelationshipContext` helper, so the
Patrol runtime only resolves the canonical relationship context rather than
formatting the relationship section itself.
Patrol-owned correlation context now also comes through the shared AI
intelligence facade before reaching the detector, so the learned correlation
surface is routed through the same canonical AI ownership boundary as recent
changes and relationship data instead of being pulled from the detector
directly in each caller.
The Patrol seed context and AI runtime prompt path now also share the same
correlation summary formatter from `internal/ai/correlation`, so learned-edge
wording and confidence/count annotations stay canonical across the prompt
surface instead of being rebuilt in each caller.
The Patrol page also now renders the canonical intelligence summary card
through the governed AI client and store, so the visible page summary and the
resource/timeline sections stay aligned on the same shared backend slice.
That same summary card now keeps recent changes and learned correlations
primary while leaving the broader learning counters as backend coverage, so
the page does not present telemetry-style counts as a headline intelligence
story.
That Patrol summary card now also includes the canonical data-governance
posture snapshot from the shared AI summary payload, so the visible page can
show the same sensitivity, routing, and redaction distribution that the
runtime derives from unified resources.
The resource drawer now carries canonical dependency and dependent
correlation context plus canonical correlation evidence through the
resource-intelligence payload, so the resource-level AI card can surface
relationship reachability and learned edge patterns directly from the AI
contract instead of inventing a second correlation summary.
The Patrol intelligence page now also consumes the learned correlation list
from the canonical AI correlations endpoint through the shared
`frontend-modern/src/stores/aiIntelligence.ts` store, so the global summary
and the resource drawer both reflect the same learned edge evidence instead
of each page fetching its own copy. Both surfaces now render that evidence
through the shared `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
card, so the correlation layout stays governed by one component instead of
two page-local card implementations. That shared card also owns the
correlation ordering and truncation rule, so the page and drawer hand it raw
correlation lists instead of slicing or re-sorting them locally.
The same page and drawer now also share the canonical
`frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
card for recent changes, so the timeline layout and relative-time wording
stay governed by one frontend feed instead of separate page-local loops.
Patrol trial-entry surfaces now also share the canonical
`frontend-modern/src/utils/trialStartAction.ts` owner for hosted handoff and
denial handling. `ApprovalSection.tsx` and
`usePatrolIntelligenceState.ts` may still choose Patrol-specific success copy,
but they must not reintroduce local `startProTrial()` status-code branches
that diverge from the commercial backend contract.
That same store now owns the Patrol dashboard load bundle as well, so the
page refresh path stays aligned on a single orchestrated AI bundle instead of
repeating the individual summary, findings, approval, and correlation fetches
inline.
The shared
`frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx` and
`frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
cards now own the canonical Infrastructure resource-link default, so the
Patrol page and resource drawer inherit resource-filter href construction
through the shared summary cards instead of rebuilding local wrappers in each
surface.
The Patrol intelligence page now also renders the canonical
`frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
card, so the data-governance posture counts stay rendered from one governed
frontend component on the page instead of being duplicated in the resource
drawer.
That same Patrol summary surface now keeps health and findings primary while
rendering recent changes, learned correlations, and policy posture as
secondary investigation context behind an explicit disclosure, so expansion
lane concepts stay available for deeper investigation without reading as the
headline Patrol product story.
That secondary investigation-context summary now also routes through the
dedicated `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
owner, so the Patrol hook composes one canonical payload-to-summary derivation
instead of rebuilding recent-change, correlation, and governed-resource count
copy inline.
The Patrol page's run-history tab label is now also tightened to `Runs`, while
the underlying run-history panel remains canonical for snapshot filtering and
tool-call inspection. That copy change is intentional: run history is support
context for Patrol findings, not a peer primary workflow beside findings.
The Patrol page and resource drawer now also share the canonical
`frontend-modern/src/utils/resourceChangePresentation.ts` formatter so
recent-change kind and headline wording stays aligned wherever the canonical
timeline is surfaced.
The Patrol page and resource drawer now keep canonical relationship semantics
in the backend correlation path, while the visible frontend stays
timeline-first and does not surface a separate relationship presentation
helper.
The backend Patrol and AI runtime summaries now also share
`internal/unifiedresources/change_presentation.go` for the canonical
change-kind and provenance mapping, so the same resource-model semantics
drive both the backend summaries and the frontend presentation helpers.
That same helper now also owns the one-line recent-change summary text used
by the AI runtime prompt sections and Patrol seed context, so the change
wording itself stays canonical before the surrounding section headers are
applied.
The same helper now also owns the canonical recent-change section wrapper,
so the Patrol page and AI runtime can share the same heading and resource
prefix rules instead of rebuilding that section locally.
The canonical shared AI resource context now also surfaces policy routing and
redaction hints from unified resources, so the Patrol page and resource drawer
see the same governance posture that the runtime uses for export boundaries.
Patrol finding dismissal reasons and Patrol status labels now also route
through the shared frontend identifier-label helper, so the Patrol surfaces
do not keep their own underscore-stripping behavior separate from the rest
of the governed presentation helpers.
