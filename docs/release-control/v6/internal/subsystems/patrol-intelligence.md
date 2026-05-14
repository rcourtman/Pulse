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
17. `frontend-modern/src/utils/approvalState.ts`
18. `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts`
19. `frontend-modern/src/utils/findingAlertIdentity.ts`
20. `frontend-modern/src/utils/remediationPresentation.ts`
21. `frontend-modern/src/utils/patrolEmptyStatePresentation.ts`
22. `frontend-modern/src/utils/patrolFormat.ts`
23. `frontend-modern/src/utils/patrolRunPresentation.ts`
24. `frontend-modern/src/utils/patrolSummaryPresentation.ts`
25. `frontend-modern/src/utils/patrolRuntimePresentation.ts`
26. `frontend-modern/src/utils/patrolRuntimeActions.ts`
27. `frontend-modern/src/utils/textPresentation.ts`
28. `tests/integration/tests/73-patrol-assistant-operator-briefing.spec.ts`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change Patrol page orchestration through `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`, keep `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts` as the canonical investigation-context derivation owner, keep `frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx` as the feature shell, keep the Patrol-owned section files under `frontend-modern/src/features/patrol/` as the heavy render owners, keep `frontend-modern/src/pages/AIIntelligence.tsx` as the route shell, keep `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts` as the canonical AI summary normalization owner, and update `frontend-modern/src/stores/aiIntelligence.ts` together
2. Add or change Patrol findings, approvals, investigation, or run-history presentation through `frontend-modern/src/components/AI/FindingsPanel.tsx` and `frontend-modern/src/components/patrol/`
3. Keep remediation execution badge copy and severity styling aligned through `frontend-modern/src/components/patrol/RemediationStatus.tsx` and `frontend-modern/src/utils/remediationPresentation.ts`
4. Add or change Patrol header, summary, status runtime-state presentation, or runtime provider action presentation through `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, `frontend-modern/src/features/patrol/PatrolIntelligenceSummary.tsx`, `frontend-modern/src/components/patrol/PatrolStatusBar.tsx`, `frontend-modern/src/utils/patrolRuntimePresentation.ts`, and `frontend-modern/src/utils/patrolRuntimeActions.ts`
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
   The Patrol route must source its Patrol findings from the direct Patrol
   findings transport through the shared intelligence store. Unified findings
   may remain available to cross-product AI surfaces, but the Patrol page,
   selected-run findings panel, badges, and active-finding summaries must not
   infer current Patrol evidence by filtering the unified threshold/AI feed.
   The same source boundary owns loading and error state: Patrol findings must
   render from `patrolFindings`, `patrolFindingsLoading`, and
   `patrolFindingsError` instead of letting a pending unified findings request
   mask direct Patrol evidence that has already loaded. Background Patrol
   refreshes must preserve already-loaded findings instead of replacing the
   workspace with a blocking loading state.
   Patrol page refresh state must also be generation-aware and timeout-bounded:
   slow or stalled supporting intelligence reads may continue in the background,
   but they must not permanently disable the operator's Refresh control once
   Patrol findings and status remain visible.
2. Keep Patrol-specific copy and badge logic inside the governed Patrol presentation helpers instead of page-local branches
   Patrol assessment copy must not present an all-clear health prediction while
   active Patrol findings or Patrol runtime issues are still present. The
   canonical summary helper owns that conflict resolution so the visible
   assessment title, description, metrics, and recommended next step all speak
   from the same current findings state. Patrol-owned runtime issues must stay
   distinct from infrastructure findings in assessment copy rather than being
   described as infrastructure warning findings about Patrol itself.
   Assessment coverage caveats must also reconcile against current run-history
   proof: a stale coverage factor or prediction must not claim recent coverage
   is incomplete when the latest completed full Patrol run successfully checked
   real resources.
   Recency coverage copy must also come from the shared presentation helper:
   use verified wording only for successful full patrols, and use neutral
   checked wording for failed full patrols or scoped activity.
3. Update this contract whenever a new Patrol-specific page, store, helper, or presentation component becomes canonical runtime surface area
4. Keep retired hosted-model and trial-like Patrol acquisition copy out of the
   normal self-hosted GA app. Patrol may parse legacy transport fields, but
   header badges, runtime banners, empty states, and settings handoffs must
   present provider setup as BYOK/local/self-managed rather than promising
   managed credits or account-backed AI access.
   Server-authored Patrol readiness from the status payload is part of the
   Patrol product surface: warnings must be visible before a run starts, and
   known not-ready states must keep recoverable provider/model settings saves
   visible and actionable while blocking manual, scheduled, and scoped Patrol
   runs instead of letting operators discover provider/model/tool
   incompatibility through a failed run.
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
   posture. The initial visible prompt for a Patrol finding must lead with that
   governed approval or proposed-fix review instruction when safe metadata is
   attached, so the operator starts from approval status, risk, dry-run posture,
   and safest-next-step review instead of generic incident discussion. Finding
   handoffs must be assembled through the Patrol-owned handoff model so the
   prompt, visible briefing, model-only finding context, resource reference,
   safe next-step action label/href, bounded action reference, and request-local
   approval-required posture stay in sync. The model-only context may include
   current finding status, recurrence, investigation record facts, evidence,
   verification, approval posture, dry-run posture, proposed-fix summary, target
   resource references, and safe route-owned next-step labels/hrefs such as
   provider settings without raw command payloads. Inline Patrol approval actions in
   `frontend-modern/src/components/patrol/ApprovalSection.tsx` that open
   Assistant must follow that same Patrol-owned handoff model rather than a
   prompt-only local shortcut: pass approval ID/status/risk/target plus safe
   summary/count metadata as review context, attach the target resource
   reference, include bounded `handoff_actions` for live approvals or structured
   proposed fixes when present, force the request-local approval-required mode,
   attach the Patrol-owned visible drawer briefing for the pending approval or
   queued-fix recovery state, and never paste the approval command or
   proposed-fix command text into the chat prompt. Remediation-plan Assistant
   handoffs follow the same boundary: step labels, plan status, risk, and command
   counts are allowed, safe suggested prompts may ask about plan risk,
   prerequisites, rollback, and verification, while command and rollback command
   text stays in the governed remediation or approval surface. Generic finding
   discussion handoffs must also force request-local approval-required mode for
   any non-empty Patrol `finding_id`, including context-only findings and
   findings that reference a live approval, proposed fix, fix outcome, or
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
   render empty-state copy above a durable Patrol record. The Patrol-owned
   investigation surface must also expose the durable record's `impact`
   and `rollback` fields: the shared
   `frontend-modern/src/components/patrol/InvestigationSection.tsx`
   renderer surfaces an explicit `Impact not assessed` placeholder whenever
   an investigation record exists with no impact text, and a parallel
   `Rollback not specified` placeholder whenever rollback is empty, so the
   operator-visible gap is conspicuous rather than hidden. The Patrol
   operator briefing card built by
   `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
   may include populated impact text in its bounded detail lines but must
   omit the placeholder copy from that compact card so it stays tight,
   leaving placeholder rendering to the full investigation surface.
   `frontend-modern/src/components/AI/FindingsPanel.tsx` is the canonical
   renderer for that operator-facing impact text on the unified findings
   surface: when an expanded finding card has populated `impact`, the panel
   renders an `Impact:` line between `Description` and `Recommendation`.
   The same panel also surfaces `investigation_record.confidence` as a
   badge in the collapsed finding row (next to the investigation outcome
   badge) so operators can scan trust without expanding every card.
   Finding rows must keep their disclosure control separate from inline manual
   controls: the row summary may be a keyboard-accessible expand/collapse
   button, but acknowledge, snooze, dismiss, and other per-finding actions must
   be sibling controls, not nested inside the disclosure button or a fake
   role-button wrapper.
   `FindingsPanel.tsx` also renders the
   `previousResolvedFixSummary` operational-memory field directly on
   the expanded finding card with emerald-accented styling, so
   operators see "what worked last time" inline without having to
   open Assistant. The summary lives on the finding shell
   (`UnifiedFinding`/`Finding`), not on the per-record investigation
   presentation; the Patrol-owned investigation-context model must not
   absorb it into impact, verification, or rollback (those represent
   the current investigation, not history).
   The Patrol page renders a small Trust strip above the Findings/Runs
   tab bar that surfaces `state.patrolStatus()?.trust` (the
   FindingsTrustSummary block on the patrol-status response) as compact
   signals: fixes verified, auto-resolved, dismissed-as-noise,
   dismissed-as-expected, currently-active, and regressed-at-least-once.
   The strip is hidden when every signal is zero so a fresh install
   does not show an empty pill. The strip must read field names that
   exist on `FindingsTrustSummary`; new strip categories must extend
   that contract first rather than inventing per-page keys.
   Findings carry contextual Assistant entry points keyed on intent: the
   default "Discuss with Assistant" button opens an open-ended chat
   handoff, and a parallel "Explain" button opens the same handoff with
   a `PatrolAssistantFindingIntent='explain'` seed that asks the LLM to
   walk through what we know, why it matters, how confident the
   analysis is, and whether the recommended action is appropriate. Both
   buttons must route through `buildPatrolAssistantFindingHandoff` so
   the structured context (investigation record, operational memory,
   pending approval, proposed fix, next-step action) is attached
   uniformly; only the leading sentence differs by intent. The
   badge palette is provided by `getInvestigationConfidenceBadgeClasses`
   in `frontend-modern/src/utils/aiFindingPresentation.ts`: high is
   reassuringly emphasized, medium is neutral, low is a soft amber.
   Findings without an investigation record (or without a recorded
   confidence) must show no confidence badge rather than defaulting to
   one, mirroring the impact rule that absent metadata is not fabricated.
   Expanded Patrol finding cards must keep action density product-grade: one
   primary Assistant intent button is visible inline (`Investigate`, `Verify
   fix`, or `Explain` based on current finding state), while secondary
   Assistant intents and operator management controls live behind compact
   in-flow `Assistant` and `Manage` menus. Those menus must render inside the
   expanded finding detail rather than as clipped floating panels, and the
   per-finding "Create rule from this" action must remain disabled unless the
   finding has both a concrete resource and category for a scoped suppression
   rule.
   The Patrol store owns the sticky expanded-history state for Resolved/All
   views and must keep that data bounded: once resolved findings are requested,
   subsequent Patrol finding loads continue to request include-resolved history
   with the 200-item history limit instead of unbounded historical payloads or
   a transient active-only refresh.
   The TS API client mirrors (`UnifiedFindingRecord.impact`,
   `Finding.impact`) and the store normalizers
   (`normalizeUnifiedFindingRecord`, `normalizePatrolFindingRecord`) must
   carry impact through alongside description and recommendation rather
   than dropping it; runtime-failure findings produced by
   `internal/ai/patrol_runtime_failure.go` are the first source to depend
   on that path reaching the Patrol page surface.
   The primary Patrol assessment summary may open Assistant as a whole-surface
   review handoff, but that handoff must also flow through
   `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`.
   The summary action may pass the current assessment title, health score,
   verification recency, latest run, secondary investigation context, bounded
   recent-change and learned-correlation evidence, active-finding summaries,
   structured resource references, structured approval/action references, and
   safe source-owned suggested prompts as model-only context. The same handoff
   must also carry the Patrol-owned recommended next step as safe bounded
   metadata, including its title, detail copy, action label, and known action
   kind when present, plus the current action-disabled reason when the visible
   Patrol-owned action is unavailable, so Assistant explains the same
   operator-facing priority and current availability shown in the summary card
   instead of inventing a separate next step. Assessment-level handoffs must
   identify the drawer target as
   `patrol-assessment` with `targetId=pulse-patrol-assessment`, matching saved
   session restore semantics rather than the retired dashboard target.
   Active-finding
   summaries may include live pending approval posture only as safe metadata:
   approval ID, pending status, risk, target, requested/expiry timestamps,
   action plan identity, approval policy, plan expiry, dry-run posture, and
   command count. They must force request-local approval-required mode, keep raw
   command and approval payloads out of prompt and drawer copy, surface visible
   drawer action posture from the same safe references, make the initial prompt
   lead with approval/action review when governed references are attached, and
   frame Assistant as explanation, prioritization, and safe next-step review
   rather than a generic reactive chat
   box. That whole-surface assessment handoff must send safe
   `handoff_metadata.kind=patrol_assessment` so saved Assistant sessions restore
   as current-assessment context instead of becoming generic scoped context or
   an accidental single-finding session because one bounded action reference
   names a finding. Saved assessment and finding sessions may expose the
   Patrol-owned recommended next step title/detail/action and whitelisted
   app-route href through the safe `handoff_summary` only after command-like
   and secret-like text is withheld; live handoffs must send those safe fields
   through structured `handoff_metadata` where available, and the browser must
   use them for restored drawer copy without receiving the private model-only
   handoff context. Assessment Assistant drawer briefings must present the safe
   recommended step title, reason, and route-owned action as separate operator
   facts instead of compressing them into an opaque context sentence, and the
   suggested prompts must follow the structured recommendation action so
   provider-setting/runtime-visibility failures lead with provider checks and
   post-restore verification rather than generic coverage questions.
   When the current Patrol assessment is coverage-incomplete with no active
   infrastructure finding, the same handoff model must frame the briefing as a
   verification gap: the prompt leads with what scoped activity did and did not
   prove, visible drawer copy names the coverage gap, suggested prompts focus
   on full-run verification and early warning signals, and execution or retry
   remains operator-controlled.
   Patrol run-history entries may also open Assistant for a selected run, but
   that handoff must flow through the same Patrol-owned investigation-context
   model rather than a row-local prompt. The browser-visible prompt and drawer
   briefing may include only classified, redacted runtime-failure summaries and
   safe run identity facts. The browser request must send safe
   `handoff_metadata` for the saved-session identity envelope: kind
   `patrol_run`, run ID, safe run type/status, and a runtime-failure boolean,
   never provider-bound runtime failure detail, analysis text, tool traces, raw
   provider payloads, or scoped resource context. Backend Assistant runtime owns
   the model-only run context: it resolves the run ID from Patrol history,
   rebuilds bounded run facts, scoped resource references, sanitized analysis,
   and classified failure detail server-side, and rehydrates the same context
   from stored metadata on follow-up turns. It must force request-local
   approval-required mode, present a source-named visible drawer briefing, and
   frame Assistant as explanation and next-step review rather than execution or
   automatic retry authority.
9. Keep the normal Patrol assessment summary plain and operator-first rather
   than a hero-style or decorative card surface. The default collapsed state
   should show the Patrol assessment, score, concise current-risk copy, one
   recommended next step, and one route-owned action. Verification detail,
   supporting metrics, and whole-assessment Assistant discussion belong behind
   the details expansion unless Assistant itself is the recommended action.

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
with chat and AI settings. The Patrol configuration model selector must remain
state-driven across async settings/catalog loading: a saved direct-provider
model such as `deepseek:deepseek-v4-flash` must render as that provider model
once the shared catalog supplies it, not fall back visually to the default
model or an unrelated OpenRouter entry because the popover mounted after the
catalog request completed.
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
the Patrol-owned recommended next step, plus deduped resource references and
safe structured approval/action references as
model-only context while forcing `autonomousMode:false` and summarizing
proposed-fix command-bearing records and command-bearing change events without
raw command text. Its visible Assistant briefing must also use those safe
references to distinguish pending governed approvals or attached action
references from a generic assessment discussion, including approval-policy and
dry-run posture when available. When no approval or governed action outranks
the summary recommendation, the briefing action label and initial prompt may
lead with that recommendation, but Assistant remains explanatory and may not
start Patrol runs, settings changes, diagnostics, remediation, or approvals
from the handoff; if the recommended action is currently disabled, the prompt
and briefing must say why instead of describing it as an available action. Its
initial prompt must prioritize approvals or action
references before broader assessment discussion while command payloads stay
out of the drawer.
Run-history rows now follow that same Assistant handoff model. A selected
`frontend-modern/src/components/patrol/RunHistoryEntry.tsx` row may open
Assistant through
`frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts` with a
`[Patrol Run Context]` block, scoped resource references, runtime failure
summary/detail when present, bounded outcome and coverage facts, and a visible
`Patrol run attached` drawer briefing. The row must not create a second
prompt-only Assistant shortcut or imply that Assistant can retry Patrol,
change provider configuration, or execute remediation from run-history context.
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
That same browser proof now covers the Patrol configuration save contract.
The advanced Patrol panel must stay within the desktop viewport, scroll its
own contents to the Apply control, and surface the backend's concrete
license/validation reason when a save is rejected instead of replacing it with
a generic `Failed to save advanced settings` toast. That inline failure may
handoff to Assistant only as model-only explanation context: raw command,
script, credential, and provider-detail payloads stay redacted, Assistant opens
with `autonomousMode:false`, and the configuration panel closes so the operator
is not left behind an overlapping popover. Configuration-failure handoffs
must send safe `handoff_metadata.kind=patrol_configuration_failure` plus only
the runtime-failure boolean needed for drawer/session presentation, so the
saved session restores as a Patrol configuration issue without carrying raw
provider, credential, command, or retry payloads into the browser.
Successful provider-model saves that return
`patrol_readiness.status=not_ready` are still configuration issues, not silent
successes: the Patrol popover must keep the saved provider/model visible,
render a `Patrol configuration needs attention` inline state with the returned
readiness cause and summary, and hand off to Assistant as a model-only Patrol
configuration issue rather than as a save failure.
The readiness contract now applies before Patrol work is admitted, not only
after a page render: recoverable Patrol provider/model settings saves must
persist and echo structured readiness cause metadata, manual run requests must
return the structured readiness reason if a stale UI still submits, and
scheduled or scoped alert/anomaly runs must skip before calling the model while
preserving the blocked reason and cause in Patrol status. The Patrol
configuration state owner must also clamp stale investigation/remediation
autonomy back to findings-only `monitor` and clear stale full-mode unlock state
before submitting Apply Configuration when the safe-remediation entitlement is
not effective, so an expired or downgraded plan cannot turn a recoverable
configuration review into a Pro-only save failure. The same inline error
surface must render any Patrol readiness
provider, model, and summary carried by the failure object, and keep the
provider-settings action immediately available before the operator chooses to
open the Assistant handoff.
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
The same precedence applies to Patrol-to-Assistant assessment handoffs. An
active finding, pending approval, or governed action reference must remain the
primary Assistant prompt and briefing posture even when the attached model
context carries a secondary coverage caveat. Coverage-incomplete language such
as explaining scoped activity or a full-run gap is the primary handoff framing
only for coverage-only assessments with no active findings.
That same assessment contract must also distinguish Patrol-owned runtime
findings from infrastructure findings. When the only active Patrol findings are
synthetic Patrol service/runtime conditions such as the `ai-service`
provider-credit failure, the top assessment should read as a Patrol runtime
issue rather than implying infrastructure issues were detected across the
estate. When there is exactly one active Patrol runtime finding, that same
assessment copy should name the concrete runtime failure, such as
`Provider billing or quota issue`, instead of reducing the state to a generic count of
runtime findings.
That primary assessment must also expose a single visible recommended next
step derived from the same Patrol summary contract, not a page-local helper.
`frontend-modern/src/utils/patrolSummaryPresentation.ts` owns that decision
from pending governed approvals, Patrol runtime issues, active infrastructure
findings, verification posture, and the current assessment tone. Pending
approvals take priority over general triage, coverage-incomplete states must
ask for full verification before any all-clear claim, runtime impairment must
point the operator back to restoring Patrol visibility, and verified healthy
states should fall back to continued scheduled monitoring rather than
inviting generic Assistant chat.
When that recommendation has an immediate governed operator path, the same
helper must also declare the bounded action kind. The summary card may map
those action kinds only to existing Patrol controls: run a full Patrol,
review approval-scoped findings, review active findings, open Patrol provider
settings, or open the Patrol-owned Assistant assessment handoff. It must not
invent page-local remediation, approval, or execution authority from
recommendation text.
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
label.
Patrol run history must follow the same operator-facing runtime failure
contract. Expanded erroring runs should show the backend-provided
`error_summary`, bounded `error_detail`, and the shared direct `Open Patrol
provider settings` action near the run narrative, so a provider/model/tool
failure remains actionable even when the operator starts from the Runs tab
rather than the synthetic runtime finding.
Those run-history failure fields are customer/API-safe classified detail, not
raw provider payload storage. The backend must normalize both newly written
and already persisted Patrol run records before browser/API serialization, so
provider protocol fragments such as `tool_choice`, `reasoning_content`, raw
endpoint URLs, credential hints, or model-internal error strings do not leak
through run history while the operator still receives an actionable recovery
summary.
The Patrol surface must not keep presenting runtime-warning or runtime-issue
copy after a successful provider-backed Patrol run proves the selected
provider/model can run tool-backed analysis. Scoped success may still leave
estate coverage incomplete, but it must clear stale Patrol runtime impairment
state and suppress provider-only readiness warnings for live-proven
tool-capable models.
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
to fix Patrol provider configuration in Assistant & Patrol settings and rerun
Patrol, optionally adding context notes, rather than hiding the runtime issue.
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
Malformed or missing approval expiry timestamps must fail closed in the shared
approval-state utility: they are not live/actionable pending approvals and the
owning finding must return to the needs-attention path. If malformed approval
records still reach presentation helpers, they must sort after valid timestamps
and must not produce non-deterministic comparator results. Patrol approval
banners must apply the same fail-closed timestamp posture to visible countdown
copy instead of rendering invalid math such as `NaN`.
Patrol fix approvals also inherit the unified action-governance preflight
contract: queued fixes must keep their plan-level dry-run availability, safety
checks, verification steps, approval policy, and action id in the shared
approval/action-audit model instead of storing Patrol-only execution context.
The investigation approval adapter must seed the unified action-audit store
with planned and pending lifecycle evidence when it creates the approval, so
Assistant handoffs and resource timelines can hydrate the same canonical
action record before any operator decision or execution occurs. Those queued
fix records must carry `pulse_patrol` as the requester and lifecycle actor so
resource timelines preserve the product source of the proposal instead of
flattening proactive Patrol work into generic Assistant chat activity. Pending
approval payloads and Patrol Assistant handoff actions must carry the same
requester identity as safe metadata, without copying the approval command
payload into Assistant.
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
Patrol-owned supporting context may still apply a bounded explanation-layer
normalization before rendering recent changes or attaching them to Assistant:
same-state timeline records must describe the changed substate, such as Docker
image status or command posture, instead of surfacing no-op wording like an
`online` to `online` transition in the Patrol page or Assistant handoff.
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
Finding dismissal reasons now carry distinct operational semantics, not just
copy variants. `not_an_issue` permanently suppresses (Suppressed=true);
`expected_behavior` acknowledges the finding forever without escalation;
`will_fix_later` is a real operational commitment — `FindingsStore.Dismiss`
populates `Finding.RemindAt` (default `DefaultWillFixLaterRemindAfter`, 7
days), and the next re-detection after `RemindAt` clears the dismissal and
records a `reminded` lifecycle event so the operator sees their commitment
lapsed instead of the finding being silently swallowed forever. Severity
escalation still wakes any dismissed finding regardless of reason. The
`dismiss_finding` LLM tool response surfaces the remind-at date in plain
language so Patrol's own conversational explanations stay aligned with this
contract.
Patrol's findings panel must also surface that commitment to operators on
the canonical Patrol surface, not only inside the LLM tool response. The
inline dismiss confirmation must preview the will_fix_later remind-at
deadline before the operator confirms (and explain the
`expected_behavior` and `not_an_issue` paths so all three feel
deliberate), and dismissed-as-`will_fix_later` rows must show the pending
`Reminding <date>` badge in an amber tone so the operator can see their
own commitment without expanding the row. The store-level
`UnifiedFinding.remindAt` field is the canonical source for both
surfaces; render code reads it from the store, the store normalizer
promotes it from `remind_at` on both the unified and patrol-direct
fetch paths.
That same dismiss confirmation must also surface a non-blocking
recurrence hint when the operator is about to dismiss a finding with
`regressionCount > 1` as `not_an_issue` or `expected_behavior` (the two
"stay quiet forever" reasons). The hint must nudge the operator toward
the reminder-bearing `will_fix_later` path without blocking the dismiss,
and must NOT appear for `will_fix_later` itself or for findings with no
prior regression. This is the operator-facing half of "Pulse learns from
operator dismissal patterns" — a recurring issue should not be silently
buried just because the operator is moving fast on triage.
The collapsed finding row must also surface `regressionCount` as a small
pill next to the investigation-confidence badge whenever
`regressionCount > 0`, so a triaging operator scanning the list can spot
"this is not a one-off" without expanding each card. The pill stays
absent on fresh detections (count == 0) so the row stays clean for
ordinary findings, and the styling (amber tone) reads as a recurrence
signal rather than a generic muted note.
The Patrol surface must also expose a manual "Mark resolved" action on
active findings, calling the canonical `/api/ai/patrol/resolve` endpoint
through `aiIntelligenceStore.resolveFinding`. This closes the loop when
the operator has fixed the underlying issue out-of-band and shouldn't
have to wait for Pulse's auto-detection to clear it. The action is gated
to `status === 'active'` (the server rejects double-resolves) and is
visually distinct (emerald accent) from the destructive dismiss
controls; it must route through the shared store action so the refresh
and error UX stays uniform with acknowledge, snooze, and dismiss.
That same surface must also attribute closures honestly using the
`autoResolved` flag now plumbed through `UnifiedFinding`. Resolution
copy reads "Resolved by you <time>" when `autoResolved === false` and
the finding is not a Patrol fix outcome (`fix_verified`, `fix_executed`,
`resolved`), so the operator timeline reads as "you closed this" rather
than "auto-resolved" when the operator clicked Mark resolved
themselves. Patrol-driven fix outcomes keep their existing copy because
those describe Pulse's actual remediation, which is more specific than
mere auto-detection.
The Patrol page header copy must also name the trust loop the surface
owns end-to-end, not the runtime controls it happens to embed. The
canonical `PATROL_PAGE_DESCRIPTION` (and the matching
`PATROL_PAGE_TITLE_TOOLTIP`) state that "Pulse investigates your
infrastructure, gathers evidence for every finding, and proposes safe
fixes under your approval policy" — the proactive-intelligence framing
described in the Pulse Intelligence vision (investigation +
explanation + governed action). The tooltip on the page-header title
must read the same string from the canonical helper rather than
maintaining a parallel copy, so hover and inline never tell different
stories about what Patrol does.
The same Patrol page header must also surface a compact trust summary
("N active · M regressed · K fixes verified") read from
`state.patrolStatus()?.trust` immediately under the page title, gated
on at least one non-zero signal so a fresh install doesn't render an
empty header strip. The detailed Trust strip in
`PatrolIntelligenceWorkspace.tsx` stays as the canonical breakdown for
operators reviewing findings; the header line is the
trust-at-a-glance entry point that names what Patrol's loop has
produced before the operator scrolls into the workspace tabs.
The same page-header recency line must also surface the coverage
signal from the most recent completed run via
`PatrolRecencyPresentation.resourcesChecked`, populated by
`getPatrolRecencyPresentation` from
`PatrolRunRecord.resources_checked`. The render reads "Last full
patrol: 3m ago — verified 47 resources" so operators see both
temporal recency and coverage in one line. The field stays optional
(omitted when zero) so a degenerate run that completed without
checking any resources does not render a misleading "verified 0
resources" line.
The expanded finding card must also expose a "Copy summary" action
that produces a paste-ready Markdown summary of the finding (severity,
title, resource header, description, impact, recommendation,
confidence, regression count). The formatter is the canonical
`formatFindingForClipboard` helper in
`frontend-modern/src/utils/aiFindingPresentation.ts`; the render must
route through `copyToClipboard` from `@/utils/clipboard` so the
delegate-to-teammate workflow is uniform with the existing Discuss
and Explain entry points. Investigation evidence and rollback plans
are intentionally omitted from the clipboard shape — those are
conversation context for the Assistant flow, not "share this
finding" context for chat or tickets.
The resource detail drawer now exposes the operator-set state via the
`ResourceOperatorStateSection` component on the Overview tab. The
section sits alongside `ResourceActionHistory` so the "what overrides
has the operator set" and "what actions has Pulse taken" stories read
together, and routes through the canonical
`@/api/resourceOperatorState` TS client with no parallel fetch path.
The two boolean toggles (`IntentionallyOffline`,
`NeverAutoRemediate`) are dirty-tracked locally with explicit
Save/Discard actions; flipping `NeverAutoRemediate` true requires an
explicit confirmation prompt because it's a safety override that
locks the resource against all automated remediation, while flipping
it false (releasing the lock) is permissive. Maintenance windows have
their own scheduler (HTML5 datetime-local inputs with 1h / 4h / 24h
quick presets and a free-form reason field); the section
distinguishes a future-scheduled window from one that currently covers
`now` so the operator sees "scheduled" before the window opens,
"active" while it covers now, and clean state once it ends. Scheduler
saves preserve the toggle state and toggle saves preserve the window,
so the two facets stay decoupled — editing one does not lose work on
the other. The section uses `createNonSuspendingQuery` rather than
`createResource` so the drawer's parent Suspense boundary does not
flicker the page-level fallback while operator state is in flight.

The investigation runtime hands the orchestrator a finding
pre-enriched with the operator-set state and operational memory it
needs to reason from. The `aicontracts.Finding` shape carries an
optional `OperatorContext` (intentionally offline, never
auto-remediate, maintenance window) and an `OperationalMemory`
projection (regression count, previous resolved fix summary, times
raised); `MaybeInvestigateFinding` populates both from the
in-process findings store and the operator-state provider before
calling the orchestrator. This is the data path that converts
"Pulse holds privileged context" into "Pulse uses privileged
context" — the orchestrator (in pulse-pro) reads the fields when
formatting its system prompt, so investigations on locked or
under-maintenance resources reason from the operator's
commitments instead of contradicting them.

The findings store also consumes per-resource operator-set state via
the narrow `ResourceOperatorStateProvider` interface installed by
`SetResourceOperatorStateProvider`. The API layer wires an adapter
that projects `unified.ResourceOperatorState` into a
`ResourceOperatorStateProjection` so `internal/ai` does not need to
import `internal/unifiedresources`. The projection returns every
operator-set signal in one call (maintenance window + the
`IntentionallyOffline` flag), and the new-finding path branches on
both: maintenance window suppression takes priority and emits
`operator_state_cause: maintenance_window` plus the
`maintenance_end_at` metadata; the `IntentionallyOffline` branch is
the indefinite fallback that emits `operator_state_cause:
intentionally_offline` with no end-at field because the suppression
has no scheduled end. Both branches set `DismissedReason =
expected_behavior` and write a `UserNote` naming the cause so the
operator can audit why future findings stayed quiet without
expanding each row. Default deployments without a provider behave
identically to before — operator-state suppression is opt-in.

The auto-dismiss is reversible. When a finding previously
auto-dismissed under `operator_state_cause` re-detects after the
suppression has lifted (maintenance window passed,
`IntentionallyOffline` cleared, or provider unwired), the
new-finding path wakes it with a `suppression_lifted` lifecycle
event carrying the previous cause.

The operator-facing FindingsPanel must also distinguish
auto-suppressed dismissals from manual operator dismissals on the
row. Both serialize as `DismissedReason="expected_behavior"`, but
the auto-dismiss carries `operator_state_cause` lifecycle metadata.
A parallel "auto: maintenance" / "auto: intentionally offline"
badge sits next to the existing dismissed-reason badge so the
operator sees who closed the loop — Pulse on their behalf vs their
own decision. The badge routes through the canonical
`getOperatorStateDismissCause` and
`formatOperatorStateDismissCauseLabel` helpers; the lifecycle scan
mirrors the Go-side helper's newest-first contract so a manual
dismissal that supersedes an earlier auto-dismiss is reported as
manual. Time-bounded operator
commitments do not silently turn into permanent dismissals; manual
operator dismissals (no `operator_state_cause` metadata) are
unaffected by this wake path because the helper that detects the
cause stops at the first `dismissed` event when scanning newest
first, treating that as the authoritative state.
