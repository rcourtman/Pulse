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
18. `frontend-modern/src/utils/findingAlertIdentity.ts`
19. `frontend-modern/src/utils/remediationPresentation.ts`
20. `frontend-modern/src/utils/patrolEmptyStatePresentation.ts`
21. `frontend-modern/src/utils/patrolFormat.ts`
22. `frontend-modern/src/utils/patrolRunPresentation.ts`
23. `frontend-modern/src/utils/patrolSummaryPresentation.ts`
24. `frontend-modern/src/utils/patrolRuntimePresentation.ts`
25. `frontend-modern/src/utils/textPresentation.ts`
26. `tests/integration/tests/73-patrol-assistant-operator-briefing.spec.ts`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change Patrol page orchestration through `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`, keep `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts` as the canonical investigation-context derivation owner, keep `frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx` as the feature shell, keep the Patrol-owned section files under `frontend-modern/src/features/patrol/` as the heavy render owners, keep `frontend-modern/src/pages/AIIntelligence.tsx` as the route shell, keep `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts` as the canonical AI summary normalization owner, and update `frontend-modern/src/stores/aiIntelligence.ts` together
2. Add or change Patrol findings, approvals, investigation, or run-history presentation through `frontend-modern/src/components/AI/FindingsPanel.tsx` and `frontend-modern/src/components/patrol/`
3. Keep remediation execution badge copy and severity styling aligned through `frontend-modern/src/components/patrol/RemediationStatus.tsx` and `frontend-modern/src/utils/remediationPresentation.ts`
4. Add or change Patrol header, summary, or status runtime-state presentation through `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`, `frontend-modern/src/components/patrol/PatrolStatusBar.tsx`, and `frontend-modern/src/utils/patrolRuntimePresentation.ts`
5. Add or change Patrol header schedule and runtime presentation through `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts`, and `frontend-modern/src/utils/patrolRuntimePresentation.ts`.
   Patrol must not surface retired hosted-model credit badges or trial-like activation prompts in the normal self-hosted GA app, even when legacy transport fields are still present.
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
4. Keep retired hosted-model and trial-like Patrol acquisition copy out of the
   normal self-hosted GA app. Patrol may parse legacy transport fields, but
   header badges, runtime banners, empty states, and settings handoffs must
   present provider setup as BYOK/local/self-managed rather than promising
   managed credits or account-backed AI access.
5. Keep customer-facing Patrol naming product-first: page titles, route chrome,
   summary copy, actions, and empty states should lead with `Patrol` or
   `Pulse Patrol` rather than generic `AI` branding. Reserve `AI` terminology
   for explicit provider/configuration settings, shared runtime internals, or
   other technical capability boundaries where the underlying ownership really
   is AI runtime rather than the Patrol product surface.
6. Keep Patrol brand icon accessibility contextual: `PulsePatrolLogo` remains
   label-bearing when the icon stands alone, but Patrol headings and actions
   that already include visible Patrol text must render the logo as decorative
   so accessible names do not repeat as `Pulse Patrol Patrol`.
7. Keep Patrol remediation copy operator-safe even while legacy transport
   fields retain `auto_fix` naming for compatibility: header autonomy controls,
   Pro-locked helper text, investigation outcome labels, and run-history badges
   must present the paid capability as safe remediation or remediation actions,
   not as a broad automation promise.
8. Keep the Patrol store aligned with the shared structured investigation
   record when transport carries one. `frontend-modern/src/stores/aiIntelligence.ts`
   may retain `investigationRecord` as data for Assistant handoff and Patrol
   presentation, but visible Patrol copy and Assistant handoff prompt framing
   must flow through the governed Patrol investigation-context helpers. Those
   helpers also own the Assistant drawer briefing content for Patrol records,
   including the rule that proposed-fix commands are summarized by count only
   and never rendered as raw command text in the handoff surface. When Patrol
   hands a pending fix or approval into Assistant, the shared handoff may retain
   only structured action references such as approval ID, status, request/expiry
   timestamps, action plan identity, fix ID, risk, target, and resource
   identity; approval and execution authority stays with the governed
   approval/remediation surfaces. Assistant may refresh the referenced
   approval's current status for review, and the backend handoff builder must
   recover a live pending Patrol investigation-fix approval by finding ID when
   the durable record lacks the current approval reference, but Patrol
   presentation must still keep command payloads inside governed
   approval/remediation context rather than rendering them as handoff copy.
   Assistant's operator decision and approval/proposed-fix posture must derive
   from the same structured handoff action after that recovery, so the
   briefing cannot contradict the action reference passed to chat execution.
   Assistant may also hydrate the referenced action plan or action audit so the
   handoff explains current action lifecycle state, requester, capability,
   approval policy, plan expiry, preflight/dry-run posture, and terminal
   success/failure without treating approval as execution authority or exposing
   raw command/execution payloads. Assistant may also
   enrich that same handoff with refreshed unified finding and
   investigation-record state, canonical resource-policy guidance, current
   canonical resource-state and capability context, canonical
   resource-relationship context, and recent canonical resource-timeline changes
   for explanation; Assistant resolves those state, topology, and timeline
   lookups through the current canonical unified-resource model before falling
   back to handoff IDs where compatibility requires it.
   When a Patrol finding declares root-cause or correlated finding IDs, the
   Assistant handoff may also resolve those related findings through the current
   unified finding store and summarize them as model-only context. Those
   related finding summaries must include current recency and latest lifecycle
   facts when present, and may seed their own structured resources for the same
   canonical policy, state, topology, and timeline hydration, but Patrol
   presentation must treat them as explanation for the current finding rather
   than approval, lifecycle, disclosure, or execution authority.
   Assistant handoffs from Patrol findings must also include a concise operator
   briefing derived from the unified finding and structured investigation record
   before the detailed finding context, so Assistant leads with the current risk,
   attention reason, recency, evidence snapshot, verification summary,
   conclusion, latest lifecycle event, recommended next step, explicit operator
   decision framing, and governed approval/proposed-fix posture instead of
   behaving like a generic chat over a pasted incident dump. The visible
   Assistant drawer briefing opened from a Patrol finding must mirror that same
   Patrol-owned operator frame, including current severity/status, recurrence or
   regression, loop state, approval/proposed-fix posture, and the explicit
   operator decision being requested. That visible briefing remains a summary
   surface only: proposed-fix commands stay summarized by count, safe suggested
   prompts must steer the operator toward evidence, approval risk, recurrence,
   and next-step review without carrying command text, and destructive action
   copy must point back to governed approval/remediation context. When a
   structured investigation record is not available yet, the same Patrol-owned
   helper must still brief the operator from current finding facts such as
   active status, severity, recurrence, and loop state instead of opening a
   generic empty Assistant drawer. When a live pending Patrol approval exists
   for that finding, the visible Assistant briefing may include only safe
   approval metadata such as approval ID, pending status, risk, requested time,
   expiry, target label, generated approval summary, and command count; it must
   not copy the approval command payload into Assistant drawer prose. The
   model-only runtime briefing must apply that same
   recovered approval reference when framing the operator decision and action
   posture. Inline Patrol approval actions that open Assistant must follow the
   same rule: pass approval ID/status/risk/target plus safe summary/count
   metadata as review context, force the request-local approval-required mode,
   attach the Patrol-owned visible drawer briefing for the pending approval, and
   never paste the approval command or proposed-fix command text into the chat
   prompt. Remediation-plan Assistant
   handoffs follow the same boundary: step labels, plan status, risk, and command
   counts are allowed, safe suggested prompts may ask about plan risk,
   prerequisites, rollback, and verification, while command and rollback command
   text stays in the governed remediation or approval surface. Generic finding
   discussion handoffs must also force request-local approval-required mode when
   the finding already references a live approval, proposed fix, fix outcome, or
   remediation plan, so default autonomous Assistant settings cannot bypass the
   Patrol action-governance boundary. The assembled handoff must still pass
   through the Assistant runtime's resource-policy sanitizer before prompt
   injection, so Patrol-owned prose
   cannot leak governed resource names, IDs, aliases, nodes, paths, or
   addresses outside the canonical policy boundary.
   If a `fix_queued` finding no longer has a live approval payload or detailed
   proposed-fix payload available, the recovery action must still open Assistant
   with the same Patrol-owned visible briefing and approval-required posture
   from current finding facts; it must not fall back to generic investigation
   chat or invite execution from missing command details.
   If the live approval is gone but the structured proposed-fix payload is still
   available, the recovery Assistant briefing must carry only safe proposed-fix
   metadata such as description, target, risk, rationale, destructive posture,
   and command count. Raw command text remains in the governed remediation or
   approval panel, while Assistant gets enough context to explain approval
   recovery and risk without becoming an execution surface.
   Generic finding-level Assistant handoffs must use that same safe metadata
   boundary when the list response lacks a full investigation record: they may
   hydrate the latest investigation session to recover proposed-fix summary,
   risk, target, rationale, destructive posture, and command count, but they must
   still keep command text out of both the user-authored prompt and visible
   Assistant briefing.
   If the referenced finding is no longer current, Assistant must drop the
   stored handoff instead of continuing from stale Patrol context. Assistant
   handoff context must also carry the unified
   finding's current lifecycle and recency facts, including active/resolved/
   snoozed/dismissed/suppressed state, detection/last-seen/resolution
   timestamps, recurrence/regression, and recent lifecycle events, rather than
   treating the investigation conclusion as the whole current record. Detailed
   lifecycle events in Assistant handoff context must stay bounded, model-only,
   and explicitly separated from approval/execution authority. Patrol must keep
   the visible finding and drawer briefing tied to the shared investigation
   payload rather than forking a Patrol-local lifecycle, policy, topology, or
   timeline summary. The inline Patrol investigation surface must also treat a
   structured `investigationRecord` as investigation data, even when the legacy
   investigation-detail endpoint has no separate session payload, so it must not
   render empty-state copy above a durable Patrol record.
   The primary Patrol assessment summary may open Assistant as a whole-surface
   review handoff, but that handoff must also flow through
   `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`.
   The summary action may pass the current assessment title, health score,
   verification recency, latest run, secondary investigation context, bounded
   recent-change and learned-correlation evidence, active-finding summaries,
   structured resource references, structured approval/action references, and
   safe source-owned suggested prompts as model-only context. Active-finding
   summaries may include live pending approval posture only as safe metadata:
   approval ID, pending status, risk, target, requested/expiry timestamps,
   action plan identity, approval policy, plan expiry, dry-run posture, and
   command count. They must force request-local approval-required mode, keep raw
   command and approval payloads out of prompt and drawer copy, surface visible
   drawer action posture from the same safe references, make the initial prompt
   lead with approval/action review when governed references are attached, and
   frame Assistant as explanation, prioritization, and safe next-step review
   rather than a generic reactive chat
   box.

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
component itself. The governed customer-facing route for that shell is now
`/patrol`, while legacy `/ai` entry points remain compatibility redirects
instead of the canonical product path.
That route-shell ownership does not make `AI` the customer-facing Patrol
product name. Internal file, store, and transport names may still carry the
shared AI-runtime boundary where that is the real technical ownership, but the
operator-facing Patrol experience should present `Patrol` as the product and
avoid drifting back to generic `AI Intelligence` branding outside explicit
provider/settings affordances.
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
That same Patrol hook boundary now consumes shared AI settings/model truth
through `frontend-modern/src/stores/aiRuntimeState.ts` instead of mounting its
own `/api/settings/ai` or `/api/ai/models` reads. Patrol-specific state still
owns local toggle optimism, run-status orchestration, and Patrol-only copy,
including runtime-availability messaging that stays Patrol-first in
operator-facing shells and uses provider/API-key wording only for the actual
configuration boundary, but the underlying AI runtime catalog must stay shared
with chat and AI settings.
The Patrol page now also treats Patrol runtime availability as a first-class
render contract: the header chip, primary summary card, and status bar must
all route through the shared `frontend-modern/src/utils/patrolRuntimePresentation.ts`
helper plus the backend `runtime_state` payload instead of inferring operator
state from the last healthy summary snapshot or run history alone.
The primary summary card now also has a Patrol-owned Assistant assessment
handoff. `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`
opens Assistant through
`frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`, which
packages the current Patrol assessment, verification posture, latest run,
secondary investigation context, bounded recent-change and learned-correlation
evidence, bounded active-finding summaries, source-owned suggested prompts, and
deduped resource references plus safe structured approval/action references as
model-only context while forcing `autonomousMode:false` and summarizing
proposed-fix command-bearing records and command-bearing change events without
raw command text. Its visible Assistant briefing must also use those safe
references to distinguish pending governed approvals or attached action
references from a generic assessment discussion, including approval-policy and
dry-run posture when available, and its initial prompt must prioritize those
approvals or action references before broader assessment discussion while
command payloads stay out of the drawer.
That active-runtime label must stay operational rather than verdict-like: the
header chip should communicate that Patrol is enabled or available, not imply
that infrastructure health is currently good merely because the runtime is on.
That render rule now also has browser-level proof in
`tests/integration/tests/18-patrol-runtime-state.spec.ts`: when the backend
reports `runtime_state=blocked`, the real `/patrol` route must show Patrol as
paused, keep the blocked reason visible, disable manual Patrol runs, and
suppress stale healthy summary headlines such as `Health A · 100/100` even if
the last summary payload still looks healthy. Legacy `/ai` entry points must
redirect into that same Patrol-owned shell rather than preserving a second
canonical route.
That same Patrol-owned presentation rule also applies to the findings empty
state: `frontend-modern/src/components/AI/FindingsPanel.tsx` must not treat
`0 active findings` as equivalent to "your infrastructure looks healthy" when
the Patrol runtime is blocked, disabled, or unavailable, or when the canonical
overall-health summary is degraded or not fully verified. The green healthy
empty state belongs only to an actually healthy Patrol summary, while degraded
coverage or paused-runtime states must surface the governing warning/error copy
through `frontend-modern/src/utils/patrolEmptyStatePresentation.ts`.
Patrol paywall actions now follow the same shared commercial navigation split.
`frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`,
`frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`, and
`frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts` may resolve
commercial destinations from the shared license boundary, but they must leave
internal-versus-external navigation semantics to `frontend-primitives` once a
Patrol feature can resolve to either product-owned routes or public pricing.
That same Patrol-owned commercial boundary must also fail closed in public
demo and ordinary self-hosted runtimes. Patrol header, banner, approval, and
history-adjacent lock surfaces must remain informational unless the operator
is already in an entitled, hosted, or explicit commercial handoff context. The
browser-owned trigger for self-hosted suppression is the shared resolved
`presentationPolicy` from
`/api/security/status`, seeded by the backend capability fact
`sessionCapabilities.demoMode` plus the self-hosted upgrade policy. Patrol
surfaces must therefore suppress upgrade CTAs, trial nudges, checkout links,
and Pro-only helper copy from that shared policy instead of reviving local demo
heuristics or issuing early commercial reads before the policy resolves.
That same shared policy now also owns Patrol approval polling posture.
`frontend-modern/src/stores/aiIntelligence.ts` must fail
`loadPendingApprovals()` closed in public demo mode and return the canonical
empty queue from the shared store boundary itself, so dashboard and Patrol
shells do not probe `/api/ai/approvals` after the read-only demo policy has
resolved.
That shared store may retain all pending approvals for dashboard and Assistant
surfaces, but Patrol-owned presentation must consume only Patrol-scoped
approval selectors. `frontend-modern/src/components/AI/FindingsPanel.tsx` and
`frontend-modern/src/components/patrol/` must not count or render generic
Assistant command approvals as Patrol finding approvals, and dashboard
action-required affordances must use generic approval actions when the pending
request is not tied to a Patrol finding.
That same store-owned demo boundary also covers remediation artifacts.
`frontend-modern/src/stores/aiIntelligence.ts` must fail
`loadRemediationPlans()` closed in public demo mode and
`frontend-modern/src/components/AI/FindingsPanel.tsx` must consume
`aiIntelligenceStore.remediationPlans` instead of issuing its own
`AIAPI.getRemediationPlans()` read, so the public Patrol page does not trigger
`/api/ai/remediation/plans` paywall probes after demo posture has resolved.
That same posture split now also centralizes Patrol commercial bootstrap.
`frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts` and
`frontend-modern/src/components/patrol/ApprovalSection.tsx` must not mount
commercial trial or upgrade state merely because an ordinary self-hosted
operator opens Patrol. Authenticated Patrol shells inherit any allowed
commercial bootstrap from `frontend-modern/src/useAppRuntimeState.ts`, so
Patrol-specific hooks do not quietly retake ownership of commercial fetch
timing or reintroduce local trial-start actions.
Under ordinary self-hosted v6, Patrol commercial affordances must also honor
the shared `presentationPolicy.hideUpgrade` contract. A free self-hosted
install may show Patrol runtime availability and configuration gaps, but it
must not show Pro trial CTAs, upgrade links, or paid helper copy unless hosted
mode, an explicit commercial handoff, or an active entitlement makes those
actions relevant.
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
That primary assessment shell must also stay inside the shared Pulse card
language. The top summary should use the same neutral `bg-surface` page-card
base as adjacent Pulse surfaces, carrying severity through compact header
accents, icon treatments, and badges instead of tinting the entire full-width
assessment panel as a one-off warning banner.
That supporting score chip must also avoid overstating infrastructure truth.
When the current state is dominated by incomplete coverage or Patrol-owned
runtime failures, the chip should read as an `Assessment` grade rather than an
`Health` grade; `Health` belongs to verified healthy infrastructure states.
That same helper also owns the primary assessment explanation. The summary card
must not pair an `Issues detected` headline with a raw coverage-only
`overall_health.prediction` sentence from a separate source; when active
findings and incomplete verification are both true, the Patrol summary should
describe both in one canonical assessment message.
That same assessment contract must also distinguish Patrol-owned runtime
findings from infrastructure findings. When the only active Patrol findings are
synthetic Patrol service/runtime conditions such as the `ai-service`
provider-credit failure, the top assessment should read as a Patrol runtime
issue rather than implying infrastructure issues were detected across the
estate. When there is exactly one active Patrol runtime finding, that same
assessment copy should name the concrete runtime failure, such as
`Provider billing or quota issue`, instead of reducing the state to a generic count of
runtime findings.
That same runtime-owned assessment must expose the fix path directly. When the
primary Patrol issue is a Patrol runtime/provider problem rather than an
infrastructure finding, the summary card should offer a direct `Open Patrol
provider settings` action instead of making the operator dig through the
findings list to discover where to correct provider configuration.
That same runtime-versus-infrastructure distinction should route through the
shared finding-presentation helper instead of being re-inferred separately by
the summary card and the findings list. The active finding row should surface
the same Patrol runtime classification with a runtime-qualified severity badge
such as `Runtime issue` or `Runtime critical`, rather than pairing a generic
infrastructure severity chip like `warning` with a second Patrol-runtime
classification badge.
That same shared finding-presentation helper should also own Patrol finding
subject labels, so Patrol-owned synthetic service findings render as
`Patrol runtime` rather than leaking backend resource internals like
`Pulse Patrol Service (service)` into the primary findings row or assistant
handoff prompts.
That same title presentation contract should normalize Patrol-owned finding
titles too. The primary findings row, assistant handoff copy, and inline
approval surfaces should present runtime findings as `Provider billing or quota issue`
rather than repeating the product prefix or legacy `Insufficient API credits`
wording once the surrounding UI already makes the Patrol context explicit.
That same finding presentation contract should own the primary remediation path
for Patrol-owned runtime findings as well. Expanded runtime-finding rows should
offer the same direct `Open Patrol provider settings` action that the top
assessment uses, instead of falling back to only generic acknowledge, snooze,
or dismiss controls.
That same contract must fail closed on manual lifecycle controls too. Patrol
runtime findings are Patrol-owned impairment signals, not ordinary estate
findings, so the findings list must not offer generic acknowledge, snooze,
dismiss, resolve, or suppress controls for them. The correct operator path is
to fix Patrol provider configuration and rerun Patrol, optionally adding context
notes, rather than hiding the runtime issue.
That same runtime-versus-infrastructure split must carry through the summary
metrics strip as well. When Patrol-owned runtime issues are active, the
supporting metrics must stop counting them under generic infrastructure
`Warnings` or `Active findings`; the strip should break out `Runtime issues`
separately and reserve infrastructure finding counts for actual estate issues.
The findings list must respect that same trust priority. When Patrol-owned
runtime issues share a severity tier with ordinary infrastructure findings, the
runtime issue should sort first within that tier so Patrol blindness is not
buried under same-severity estate warnings.
The summary recency chip must follow the same governed scope distinction. When
the latest completed activity was only a scoped run, the summary should label
that timestamp as `Last activity` instead of `Last patrol`; `Last full patrol`
belongs only to the most recent completed full Patrol run.
That same distinction is transport-backed. `last_patrol_at` names the last
completed full Patrol sweep, while `last_activity_at` may advance on scoped
work or fix-verification checks without claiming a new full verification pass.
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
That same Patrol-owned run-history surface must keep platform-backed system
counts canonical too. When the backend distinguishes API-backed TrueNAS
systems from unified-agent hosts in Patrol run history, the run-history chips
and selected-run breakdown must render `TrueNAS` separately instead of
collapsing those systems back into the generic `agent` count.
That run-history distinction must not reopen a parallel raw `truenas`
resource-type contract in Patrol findings, scoped-run filters, or alert-backed
resource state. Those payloads stay canonical `agent` plus platform context,
while the separate `TrueNAS` count remains a run-history coverage detail.
That summary-card/metrics-strip split also applies to findings counts. The
primary assessment card may keep health as supporting context, but active
findings, warning counts, and critical counts belong to the supporting metric
strip rather than being repeated as duplicate badges inside the primary card.
That same summary surface must also explain what Patrol actually verified.
Recent run history should drive a visible verification summary that tells the
operator whether Patrol recently completed a successful full patrol, only ran
scoped alert-triggered checks, or ended its most recent full patrol with
errors, so the page does not leave trust and coverage as implicit background
knowledge.
When same-day run history shows both a recent full patrol and a burst of
scoped follow-up activity, that same verification surface should expose the
recent activity mix explicitly instead of leaving operators to reconcile a
`Recently verified` headline with a busy Patrol strip elsewhere on the page.
Fix-verification checks belong to that same explanation layer as targeted
activity, not as evidence of a fresh full-estate sweep.
The same hierarchy applies to supporting context. Correlations, recent
changes, and policy posture are secondary evidence for deeper investigation, so
the supporting-context disclosure belongs beneath the primary findings/history
workspace rather than inside the assessment card itself.
When that disclosure is expanded, the page must explicitly tell operators that
findings and run history are Patrol verification evidence, while recent
changes, learned correlations, and policy posture are explanatory context and
do not count as a fresh Patrol run.
When Patrol is healthy and fully verified, that supporting-context disclosure
should stay out of the main page flow instead of advertising a second parallel
Patrol workflow with nothing active to explain.
That same operational context belongs inside the same secondary status area as
verification, not as a separate full-width strip that competes with the
findings workspace. `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`
plus the shared `frontend-modern/src/utils/patrolRunPresentation.ts` helpers
should carry the latest run kind/result, recent activity mix, scoped-trigger
state, and any circuit-breaker warning as factual support beneath the primary
assessment instead of letting Patrol explain itself through multiple parallel
status bands.
That same secondary area should simplify by consolidation rather than
deletion. Patrol should keep meaningful run and trigger facts visible, but it
should stop repeating the same runtime story as a summary card, a metric row,
and a separate status strip all at once.
If `frontend-modern/src/components/patrol/PatrolStatusBar.tsx` is reused in a
compact context elsewhere, it must stay factual and operational within that
same hierarchy rather than introducing a second Patrol verdict label.
That same activity explanation should handle noisy Patrol behavior concretely.
When same-day history shows a mix of full sweeps, verification checks, and
scoped alert- or anomaly-triggered patrols, the verification/activity surface
should expose a compact breakdown of those run categories instead of leaving
operators to infer why Patrol looked busy from an undifferentiated run count
alone.
The same operational layer should also surface scoped-trigger state directly.
If Patrol has queued scoped work, is in busy mode, or has one scoped trigger
source disabled, the governed summary should state that as factual runtime
context instead of forcing operators to cross-reference hidden settings or
infer it from missing runs.
That recent-activity copy also has to remain intelligible in compact or
plain-text renders: the latest-run segment must keep an explicit textual
separator between run kind and result, so degraded entries read as
`Scoped run · error` rather than collapsing into concatenated strings like
`Scoped runerror`.
Those runtime facts must stay aligned with Patrol configuration copy. The
header settings surface should expose alert-triggered and anomaly-triggered
scoped patrols as separate controls, with the legacy aggregate event-trigger
toggle treated as compatibility-only transport rather than the primary product
model.
The findings empty state must also stay subordinate to the Patrol header and
assessment shell rather than mirroring their timing metadata. In the primary
Patrol page, `frontend-modern/src/components/AI/FindingsPanel.tsx` should
explain the absence of active findings without repeating `Last activity`,
`Next run`, or interval schedule details that already belong to the header and
verification hierarchy above.
That same empty-state contract must become run-snapshot-aware when the user is
filtering findings to an explicit Patrol run. A selected run with an explicit
empty `finding_ids` snapshot should explain that no findings were recorded for
that run and, when applicable, carry the canonical run coverage summary and
issue caveat instead of falling back to the generic `No Patrol findings to
display` copy.
That same snapshot scoping must apply to the findings control bar too. When the
user is looking at an explicit run snapshot, filter pills and their attention
or approval counts must derive from that snapshot-scoped finding set rather
than borrowing global Patrol finding counts from outside the selected run.
That same snapshot scoping must also apply to the `Findings` tab badge itself.
When the selected run carries an explicit empty `finding_ids` snapshot, or when
the run lacks snapshot ids entirely, the tab must fail closed instead of
borrowing global active-finding counts and tones from outside the selected run.
That same scoped count model must drive conditional findings filters too. The
`Needs Attention` and `Approvals` buckets, their counts, and the auto-reset
logic that returns the operator to `Active` when a bucket disappears must all
read from the same snapshot-aware count source rather than mixing
snapshot-scoped pills with global queue truth.
That same fail-closed snapshot rule applies to inline run-history findings as
well. Expanded run cards should route through the same snapshot-aware findings
surface as the primary workspace, so legacy runs without `finding_ids` still
show an explicit snapshot-unavailable state instead of disappearing or being
coerced into an empty findings snapshot.
That same rule applies to the primary findings workspace when a run is
selected. A selected run without `finding_ids` must not borrow global Patrol
findings, filter buckets, or queue counts; the findings surface should enter an
explicit snapshot-unavailable state instead.
That same unknown-snapshot state should be visible in the selected-run shell
too. When the operator is filtered to a legacy run without findings snapshot
ids, the selected-run banner should explicitly say that findings snapshot data
is unavailable instead of implying a fully verifiable run-scoped findings view.
That same trust rule applies inside expanded run-history narratives. A legacy
run without findings snapshot ids must not render an `All clear` conclusion
just because its aggregate counters are zero; the narrative should explicitly
state that run-specific findings could not be fully verified.
That same truthfulness rule applies to the expanded run outcomes strip. Legacy
runs without findings snapshot ids must not render a green `All clear` outcome
badge from zero aggregate counters; they should show an explicit snapshot
unavailable state instead.
That same caveat belongs in compact latest-run summaries too. When the most
recent Patrol run predates findings snapshots, the status bar's latest-run
segment should say that findings snapshot data is unavailable instead of
flattening the run into a plain healthy-looking summary.
That same caveat belongs in collapsed run-history rows. Legacy runs without
findings snapshot ids must carry an explicit snapshot-unavailable marker in the
top row instead of looking like a clean zero-findings run until expanded.
That same rule applies to run-status badges. A legacy run without findings
snapshot ids must not keep a green `healthy` badge when the surrounding UI is
saying findings verification is unavailable; the canonical run-status
presentation should downgrade that state to a neutral `completed` badge.
That same truthfulness rule applies to the run-history shell copy. The `Recent
patrol runs` helper text must not promise that every visible run can filter
findings to a concrete snapshot; when visible runs include legacy entries
without `finding_ids`, or when the selected run itself predates findings
snapshots, the shell should say so explicitly.
That same findings surface should keep its section chrome functional rather
than promotional. Inside the Patrol findings tab, the selected tab already
names the surface, so the findings card should not add another in-card product
header or marketing subtitle that simply repeats the tab-level context.
That same control bar should also stay proportional to the current findings
set. Sort controls belong only when there are multiple Patrol findings to
order; the empty state or single-finding state should not spend header space
on a no-op sort selector.
The same proportionality rule applies to the findings filter bar itself. When
there are no Patrol findings and no special approval or attention queues to
navigate, the findings surface should not render filter pills that lead only
to the same empty state.
Tab badges and finding metadata in that same surface also need explicit textual
separators rather than CSS-only spacing. Badge counts should preserve readable
plain-text output such as `Findings 1` and `Runs 30`, and metadata trails
should use textual separators like `· acknowledged 22d ago` so copied or
extracted Patrol output does not collapse words together.
That same findings contract must preserve and use `last_seen_at` for active
Patrol findings. Recurring active issues should read as `last seen ...` and
sort by current observation recency rather than presenting only the original
`detected_at`, which makes still-active Patrol service issues look stale when
they were re-observed on recent runs.
That same finding row should avoid redundant state stacking. When an active
finding is already explicitly marked `Acknowledged`, the UI should not also add
the baseline `detected` loop-state badge beside it; loop-state badges only add
value there when they communicate a more specific Patrol state.
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
The Patrol header now also has explicit helper ownership for schedule and
runtime presentation. `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts`
and `frontend-modern/src/utils/patrolRuntimePresentation.ts` are the canonical
owners for Patrol interval option labeling and runtime-state wording used by
`frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`; future
Patrol header work should extend those helpers instead of rebuilding schedule
labels, runtime copy, or retired managed-credit badges inline in the header
shell. That same header surface must normalize older hosted-model block reasons
back to provider or local-model setup guidance.
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
Patrol trial-entry surfaces are retired for normal self-hosted v6 GA.
`ApprovalSection.tsx` and `usePatrolIntelligenceState.ts` may link to explicit
plan, activation, recovery, support, or hosted handoff surfaces where
presentation policy allows, but they must not reintroduce local
`startProTrial()` status-code branches or trial-specific denial copy.
Pending Patrol fix approvals now also require a canonical urgency order across
the store and Patrol approval surfaces. `frontend-modern/src/stores/aiIntelligence.ts`,
`frontend-modern/src/components/patrol/ApprovalBanner.tsx`, and dashboard
approval consumers must treat the approval queue as `soonest expiry first`,
then higher risk, then older request time, rather than inheriting raw API
order. Approval-linked findings must follow that same ordering so multi-approval
`Review` actions jump to the most urgent finding instead of an arbitrary one.
Patrol fix approvals also inherit the unified action-governance preflight
contract: queued fixes must keep their plan-level dry-run availability, safety
checks, verification steps, approval policy, and action id in the shared
approval/action-audit model instead of storing Patrol-only execution context.
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
rendering recent changes, learned correlations, and policy posture only as
supporting context behind an explicit disclosure, so expansion lane concepts
stay available for deeper investigation without reading as the headline Patrol
product story.
That secondary investigation-context summary now also routes through the
dedicated `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
owner, so the Patrol hook composes one canonical payload-to-summary derivation
instead of rebuilding recent-change, correlation, and governed-resource count
copy inline.
The Patrol page's run-history tab label is now also tightened to `Runs`, while
the underlying run-history panel remains canonical for snapshot filtering and
tool-call inspection. That copy change is intentional: run history is support
context for Patrol findings, not a peer primary workflow beside findings.
Within that same run-history surface, coverage copy must stay canonical across
the header chips, narrative summary, and selected-run snapshot. Scoped runs
must not present scope size and checked-resource count as contradictory
independent facts; the shared run presenter should collapse them into one
coverage statement such as `Checked 1 of 2 scoped resources`, and zero-coverage
scoped runs should fail closed as `Checked 0 of N scoped resources` rather
than drifting back to a scope-only headline.
That same normalization applies to supporting effort strips inside the expanded
run card. Once the run presenter is already carrying canonical coverage copy,
secondary chips must not reintroduce a raw `Scoped to N resources` variant that
re-opens the same ambiguity.
That same zero-coverage rule also applies to the expanded narrative sentence.
When Patrol checked none of a known scoped set, the run summary must still say
`Checked 0 of N scoped resources` rather than reverting to a generic `Patrol
completed` sentence that hides the failed coverage.
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
That same Patrol findings surface must also stop being a route dead end.
Expanded finding rows should resolve the backing unified resource and surface
canonical `Infrastructure`, `Workloads`, `Storage`, and `Recovery` handoffs
through `frontend-modern/src/routing/resourceLinks.ts` where those surfaces
exist, rather than forcing operators to pivot manually through search or
assistant prompts to continue investigating API-backed platforms such as
TrueNAS.
Patrol upgrade posture now follows the same runtime-versus-commercial split as
the rest of the app. Patrol runtime availability must stay on the
non-commercial capability store, while any paid handoff uses explicit plan,
activation, recovery, support, or hosted surfaces governed by presentation
policy. Patrol surfaces must not recombine those contracts into one entitlement
payload or revive trial-start selectors in leaf components.
Patrol findings presentation now also keeps runtime identity and action routing
on shared helpers. Findings shells may link or format from feature-owned
presentation helpers, but Patrol runtime severity, title cleanup, and primary
settings actions must stay keyed to the canonical Patrol service identity
instead of reimplementing those branches in links-only or leaf badge surfaces.
