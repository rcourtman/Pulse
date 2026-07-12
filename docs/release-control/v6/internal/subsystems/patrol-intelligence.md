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
2. `frontend-modern/src/features/patrol/patrolAutonomyAvailability.ts`
3. `frontend-modern/src/features/patrol/patrolControlPresentation.ts`
4. `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`
5. `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`
6. `frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`
7. `frontend-modern/src/features/patrol/PatrolIntelligenceWorkspace.tsx`
8. `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
9. `frontend-modern/src/pages/AIIntelligence.tsx`
10. `frontend-modern/src/stores/aiIntelligence.ts`
11. `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts`
12. `frontend-modern/src/types/aiIntelligence.ts`
13. `frontend-modern/src/components/AI/FindingsPanel.tsx`
14. `frontend-modern/src/components/Brand/PulsePatrolLogo.tsx`
15. `frontend-modern/src/components/patrol/`
16. `frontend-modern/src/utils/aiFindingPresentation.ts`
17. `frontend-modern/src/utils/approvalRiskPresentation.ts`
18. `frontend-modern/src/utils/approvalState.ts`
19. `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts`
20. `frontend-modern/src/utils/findingAlertIdentity.ts`
21. `frontend-modern/src/utils/remediationPresentation.ts`
22. `frontend-modern/src/utils/patrolEmptyStatePresentation.ts`
23. `frontend-modern/src/utils/patrolFormat.ts`
24. `frontend-modern/src/utils/patrolPagePresentation.ts`
25. `frontend-modern/src/utils/patrolRunPresentation.ts`
26. `frontend-modern/src/utils/patrolSummaryPresentation.ts`
27. `frontend-modern/src/utils/patrolRuntimePresentation.ts`
28. `frontend-modern/src/utils/patrolRuntimeActions.ts`
29. `frontend-modern/src/utils/textPresentation.ts`
30. `tests/integration/tests/73-patrol-assistant-operator-briefing.spec.ts`
31. `tests/integration/tests/78-monitor-first-patrol-workbench.spec.ts`

## Shared Boundaries

1. None.

## Extension Points

Desktop Autopilot activation consumes the server-owned acknowledgement
contract through `frontend-modern/src/api/patrol.ts` and
`PatrolAutopilotAcknowledgementDialog.tsx`. The UI displays requested versus
effective mode, records the current acknowledgement version before full-mode
activation, exposes revocation, and treats expiry, revocation, version drift,
and activation races as server-authored demotion. Proof is
`PatrolAutopilotAcknowledgementDialog.test.tsx`,
`usePatrolIntelligenceState.test.ts`, and desktop journey 82. This statement
does not certify mobile or physical-device behavior.

Open work descriptions are Patrol-owned operator guidance. They may mention the
visible next step, approvals, and verification results when those words help the
operator understand what to do next, but they must not become a separate proof
strip, backend accounting summary, or generic all-mode capability sentence.
Shell-level Patrol open-work counts are also Patrol-owned read-model
projections. They may de-duplicate active Patrol findings and live Patrol
approval targets so the stable `Patrol` tab shows that work exists, but they
must not become a second queue model, a renamed `Needs Attention` destination,
or historical proof/counting for resolved-only work.

1. Add or change Patrol page orchestration through `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`, keep `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts` as the canonical investigation-context derivation owner, keep `frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx` as the feature shell, keep the Patrol-owned section files under `frontend-modern/src/features/patrol/` as the heavy render owners, keep `frontend-modern/src/pages/AIIntelligence.tsx` as the route shell, keep `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts` as the canonical AI summary normalization owner, and update `frontend-modern/src/stores/aiIntelligence.ts` together
   The default Patrol workspace is the current-issues list. It must not add a
   second proof strip that restates the same finding, approval, or setup
   problem above the list. When active Patrol findings are the next operator
   step, the finding row owns the canonical primary action, contextual
   Assistant handoffs, approvals, verification, and manual controls. A
   compact row-owned issue scaffold may translate the same finding, approval,
   and workflow fields into problem, affected resource, consequence, checked
   evidence, safe workflow step, next step, and verification state, but it must
   stay attached to the active issue row and must not become a separate proof,
   trust, or status strip. Because a
   contextual Assistant handoff from that workflow is still a first-party Patrol
   starter for the same governed journey, it must record content-free workflow
   prompt activity through the shared marker route with the `pulse_patrol`
   surface before the drawer opens; it must not include finding IDs, prompt
   text, resource context, model output, or direct-action payloads in that
   marker.
   A compact Patrol work-group row may appear above the list only when it adds
   cross-source current-work grouping that the row title alone cannot express:
   pending approvals, failed governed actions, a failed/latest Patrol check,
   recurring active issues, or overdue scheduled protection. It must be
   data-only presentation from Patrol status, run history, approval state, and
   current finding composition; it must not become a status strip, trust strip,
   health summary, generic proof counter, or replacement for the issue row's
   action, evidence, approval, verification, and Assistant handoff controls.
   A calm Patrol queue must stay plain: the Open work description and the
   current-findings empty state may say that there are no current issues and
   may point to History for past outcomes, but the workspace must not render a
   calm-day protection-posture, verification-waiting, schedule-freshness, drift,
   trust, or proof strip just to restate that no current work exists.
   A calm-looking queue is not an all-clear when its checking evidence is
   stale. `patrolControlPresentation.ts` owns freshness derivation from the
   latest completed check, the server-authored Patrol interval, and the next
   scheduled check. Alongside current work, stale coverage may appear as the
   existing compact cross-source work-group row. On a calm queue, the
   current-findings empty state alone must become warning-toned so the same
   stale condition is not repeated; neither case may add a second protection,
   schedule, or proof strip.
   Monitor-context coverage posture is a separate presentation boundary, but it
   must not render as a generic proof strip on Proxmox overview or other
   monitor-first launch pages. A future monitor-specific Patrol affordance must
   be scoped to an operator action or selected Patrol context, point active work
   back to Patrol, omit drift/trust proof counters, and must not introduce a
   Patrol-page protection-posture list as a launch-page strip or generic Home
   summary.
   Runtime/setup findings are still active Patrol work, but setup-only runtime
   failures must read as one setup task rather than an infrastructure issue
   queue. The header may suppress run and schedule/model controls while setup is
   the only active work, but it must keep the Patrol mode selector visible
   because the operator's autonomy boundary remains a primary product choice
   even before Patrol can run. The workspace must replace the generic findings
   row with a dedicated provider/runtime setup task, one current issue label,
   and the direct provider-settings action. It must not show runtime severity
   chips, loop-state chips, recurrence badges, expand chevrons, or finding
   filters in this state because those controls do not help the operator fix
   Patrol setup.
   The Patrol page must not expose a generic Details/supporting-context evidence
   panel for nearby activity, learned correlations, or policy buckets. Those
   payloads may still feed Assistant context, backend investigation, and explicit
   run or finding records, but the default first-party Patrol page stays on the
   issue, consequence, and next action.
   Initial Patrol data refresh failures are degraded data state, not a route
   failure: `usePatrolIntelligenceState.ts` must fail soft, keep the Patrol
   workspace visible, and let `PatrolIntelligenceBanners.tsx` show one short
   stale-data retry banner without leaking raw transport errors.
   Watch-only and locked-control copy must describe Patrol as checking
   infrastructure and showing current issues, while the selected Watch only
   mode must describe the capability positively, such as Patrol checking
   infrastructure and reporting issues only.
   It must not frame the free/monitor boundary as evidence recording,
   proof collection, or a disabled version of remediation. Paid mode labels
   must stay short operator choices (`Ask first`, `Safe auto-fix`,
   `Autopilot`) and must not reintroduce Pro-matrix labels such as
   `Ask before changes`, `Auto-fix safe issues`, or `Policy autopilot`.
   Plan-locked Patrol controls must keep the working surface on the positive
   Watch-only capability and must not surface paid modes, disabled paid-mode
   buttons, compact Pro badges, a paid-mode matrix, or any paid-mode disclosure.
   The free Patrol surface stays a complete, clean monitoring tool with no
   mention of paid Patrol capabilities; Pro discovery belongs in Settings, the
   website/docs, and contextual at-need prompts, not in the daily-use Patrol
   control surface. The single allowed contextual at-need prompt is the
   finding-level Pulse Pro investigation handoff: it may appear only in the
   expanded finding primary-action area, only for plan-locked installs, only on
   active critical or warning findings, only as one non-salesy capability line,
   with the upgrade action gated by the upgrade-prompt policy, and never as an
   ambient matrix, badge, or paid-mode disclosure. The selected mode must remain Watch only and the copy must
   not become a feature-matrix or absence explainer.
   Paid Patrol control handoffs that start or continue the same
   journey must reach Patrol through the route-backed
   `/patrol?patrolControlStarter=patrol_control#patrol-control` affordance.
   `usePatrolIntelligenceState.ts` owns consuming that coarse starter query
   before the initial Patrol data load, writing the same content-free workflow
   prompt activity through the `patrol_control` surface, and then replacing
   the URL with the clean Patrol control anchor. The legacy
   `operationsLoopStarter=patrol_autonomy` and
   `operationsLoopStarter=pulse_pro_activation` values must remain accepted as
   compatibility aliases that normalize to the current Patrol control marker,
   and `#operations-loop` may exist only as an inbound compatibility anchor.
   That route flag is only a first-party starter marker; it must not carry
   finding IDs, prompt arguments, resource context, model output, or
   completed-work proof.
   Direct Patrol control selection from the Patrol page is also a first-party
   starter for that paid Patrol operator journey. After a successful autonomy
   settings save, `usePatrolIntelligenceState.ts` must record the same
   content-free `patrol_control` marker when the effective autonomy level or
   full-control acknowledgement actually changes and the paid Patrol control
   feature is available, then refresh Patrol status, findings, approvals, and
   run history so the next Patrol operator step is visible without waiting for a
   later poll or page reload. Locked Community/runtime saves and repeated no-op selections must
   not record paid starter activity.
   The same journey may count a generic Patrol run or recency timestamp as
   readiness to start, but it must not advance the current operator state until
   it has issue-backed Patrol evidence: an active or resolved finding,
   investigation state, pending approval, governed action, verified fix, or
   Patrol status trust signal. A healthy no-finding run stays on the Patrol
   watch state instead of masquerading as Assistant-ready work.
   The current operator state must consume the canonical
   `/api/agent/operations-loop/status` projection through the shared
   agent-capabilities frontend client when that status is available; page-local
   Patrol counts may remain only as resilience fallback and UI affordance state
   for whether Patrol can run or which single finding can be opened. Patrol page
   components must not maintain a second progress state machine
   once the backend projection has loaded. The loaded and fallback current-work
   models must keep approved and rejected governed-action decisions distinct:
   approved decisions continue to the verified-outcome step, while a rejected
   only decision is a terminal no-execution outcome and must not strand the
   operator on a verification step that cannot run.
   The same model must treat active findings and pending approvals as current
   operator work. Older completed or resolved Patrol control proof carried by
   primary `patrolControl*` fields may tailor history copy, but it must
   not make the primary action, title, or step status say the work is done
   while an active finding or approval remains. Current
   active findings must route the primary current-work action to the canonical
   finding primary action when exactly one active finding exposes one, or back
   into the Patrol findings workflow first when no direct action exists, even
   when the backend projection still says `open_assistant`; Assistant remains a
   contextual handoff from the selected finding or approval rather than the
   primary Patrol investigation CTA. The primary current-work state and
   compact progress label must describe current work state (`ready`, `needs
attention`, `approval needed`, `outcome verified`, `no active work`) instead
   of repeating the selected mode; the Patrol mode selector/header
   owns current-mode copy, and local Patrol state must expose work evidence as
   Patrol-owned issue/work counts rather than legacy proof counts.
   When no active finding or pending approval remains, terminal verified or
   rejected outcomes must read as history behind a `no active work` current
   state; current-state copy must not ask operators to reconcile old approved
   work with the current autonomy boundary. Historical verified outcomes with
   no current operator action must not render a separate proof strip
   solely to link to history; the Patrol workspace owns the deliberate history
   review affordance.
   Resolved-only issue history and trust counters are not current operator work:
   they may support history or analytics copy, but they must not make the
   primary Patrol status say Patrol found a current issue when the active
   finding and pending-approval counts are zero. Compact Patrol summaries must
   label recurrence/trust counters as past evidence rather than placing
   bare `regressed` copy beside current healthy or active-finding state, and
   health scores must be named as a `health score` instead of rendering as
   unexplained `95/100` telemetry.
   The Patrol workspace must default to current operator work: current findings,
   active investigations, approval decisions, and selected finding snapshots.
   The route and page title must lead with `Patrol`, because operators
   understand Patrol as the recurring checking loop. The default workspace title
   underneath it may use `Open work` so the page can explain current findings
   and approvals without renaming the destination. Patrol remains the
   watch/investigate/act/verify engine. The
   exception is a
   setup-only Patrol runtime failure: that state must use the `Fix Patrol setup`
   title and setup description while rendering a dedicated setup task instead of
   the generic finding row. When the list is empty, Patrol-owned empty-state copy
   must say there are no current issues and may reference past regressions only
   as history, not as current work.
   Run history is evidence and audit trail, not a peer top-level mode; expose it
   through an explicit history review affordance, keep the ledger visually
   bounded, and return the user to the findings snapshot when a historical run is
   selected. Broad supporting context such as recent changes, correlations, and
   policy coverage may be offered only as a compact `Details` affordance
   when there is an active Patrol finding or an explicitly selected history
   snapshot to explain. The full `Details` panel must render only
   after that affordance is opened; its copy must tell operators it explains
   Patrol's recorded finding or run state and must not present the data as a
   separate raw evidence console. Degraded health or historical recurrence alone
   must not surface a page-level forensic context block.
   The default Patrol surface must stay simple: the header, control selector,
   and Open work workspace own the ordinary operator state. Patrol must not
   render a separate always-visible status/activity/health strip just to prove
   the loop exists. Actionable runtime, setup, or coverage failures may still
   surface as concise warnings, and detailed status/trust/history evidence
   belongs behind the queue, history, or explicit context affordances.
   Manual page data sync is a secondary read-model affordance, not a Patrol
   operation. The Patrol header may expose it only as explicit page data sync
   for status, queue, approvals, and run history; it must not read as another
   Patrol run, must not be labeled `Refresh Patrol`, and must not use an
   endlessly animated spinner that competes with the actual Run Patrol action.
   The Patrol page must still remain understandable when the broader
   intelligence summary is missing or slow: keep the page title, selected Patrol
   mode, and Open work label visible instead of reintroducing a generic
   status strip.
   Plan-locked Patrol mode copy must present the effective Watch only
   capability as the current product state: Patrol checks resources, detects
   issues, and records findings. The free Patrol working surface must not render
   a disabled paid-mode matrix, compact Pro badges, or any paid-mode disclosure;
   it stays clean of paid-feature surfacing entirely. Runtime-locked Pro installs may still
   show explicit blocking copy because the operator already has the entitlement
   and needs the correct runtime.
   User-facing copy names this choice `Patrol mode`; stable route markers,
   telemetry fields, and API wire names may retain `patrol_control`,
   `patrolControl*`, and older autonomy aliases for compatibility.
   Patrol mode must default to a simple user choice when paid control is
   available: show the current mode, one short sentence, and the four
   available modes. The
   control-specific policy contract, hard limits, and who-approves-what details are
   useful, but they must sit behind an explicit details/limits affordance rather
   than appearing as a default matrix on every page load.
   Paid-control availability copy must frame that decision as choosing what
   Patrol may handle automatically. It must not describe the user's choice as
   deciding how far Patrol can go or how much control Patrol has, because those
   phrases imply raw authority rather than governed scope.
   The Patrol page header is part of that same boundary: when the install is
   plan-locked to Watch only, the header description must present the current
   install capability rather than a disabled Pro matrix. When a Pro operator
   selects `Watch only`, the header must describe that selected mode as a
   no-change boundary, not as the whole Patrol product promise. Governed paid
   modes may use mode-specific watch/investigate/act/verify/record copy that
   matches the selected mode.
   Manual `Run Patrol` state must distinguish the start request from follow-up
   status refresh: `Starting` is only the short-lived request-to-start phase,
   while accepted or streaming runs move into the run-in-progress state and must
   not stay blocked behind slower dashboard/status reloads.
   Operations-loop starter counts from that backend projection are evidence
   that the Assistant, Patrol mode/autonomy compatibility entry point, legacy
   Pro activation entry point, or Pulse MCP entry point was used. A first-party
   Patrol control starter may help telemetry and entry-point routing, but it
   must not render a separate ready-to-run proof strip. The visible ready state
   belongs to the header control and `Run Patrol` action; it must not read as
   configuration proof such as
   `Patrol mode ready`. Legacy Pro activation starter aliases must stay
   compatibility evidence and must not render that ready-to-run strip by
   themselves. The
   Assistant step count from that projection is different: it is contextual
   collaboration evidence from Assistant context/tool usage or external-agent
   activity, and it should take precedence over starter counts when shown.
   Patrol must not use starter evidence to bypass the issue-evidence,
   governed-decision, or verification stages.
   The same first-party journey must not treat external-agent readiness as the
   operator's primary success condition. Pulse MCP contract availability still
   comes from the shared agent-capabilities manifest exposing the operations-loop
   workflow prompt and the required governed tools through the Pulse MCP surface
   contract, and loaded readiness still comes from the backend operations-loop
   status field after the server has checked for a non-expired API token covering
   at least one Pulse MCP-published capability scope. Patrol may use those facts
   for telemetry, settings handoff, and external-agent readiness, but the default
   Patrol page loop is governed operations: watch, investigate, act under policy,
   verify, and record. The first-party Patrol workflow state must not
   load manifest-backed MCP readiness or require MCP readiness props; Pulse
   Intelligence settings and external-agent surfaces own MCP setup/readiness,
   while API security owns only the scoped-token creation surface they link to.
   Patrol mode labels and details must match the backend execution policy:
   assisted mode may say it can run low-risk fixes explicitly allowed by policy,
   but it must not imply that warning severity alone makes a command safe;
   high-risk, critical, destructive, or unknown-risk fixes remain approval-bound
   unless the operator has selected a broader autonomy level. Compact Patrol
   mode labels must remain understandable mode decisions (`Watch only`,
   `Ask first`, `Safe auto-fix`, `Autopilot`), not shorthand such as `Ask` or
   `Safe` that makes the operator infer what Patrol will do.
   The current-work and findings-workspace descriptions must derive from the
   active Patrol mode and lock state, not from a generic all-capability
   sentence. Watch-only installs must not advertise investigation, approvals, or
   fixes in the default findings copy. The default workspace description should
   tell the operator what Patrol-found problems will appear there and what
   Patrol may do under the selected mode, including whether the next row-level
   step is evidence review, approval, automatic action review, or verification
   result review; it must not describe internal queue mechanics, activation
   proof, or verification accounting.
2. Add or change Patrol findings, approvals, investigation, status-bar, or run-history presentation through `frontend-modern/src/components/AI/FindingsPanel.tsx` and `frontend-modern/src/components/patrol/`
   Patrol owns run-history labels, counts, status variants, and domain copy, but
   visible run-history, status-bar, and runtime-summary state badges must
   compose the shared `StatusIndicatorBadge`, and
   resource/outcome/snapshot/scoped-run/status-bar/workspace-tab metadata chips
   must compose the shared `MetadataBadge`. Patrol and AI finding loading
   indicators must compose the shared `LoadingSpinner` for border-based spinner
   shell, size, tone, and accessibility behavior; icon-specific refresh
   rotation remains local icon state. Patrol run-history empty states own
   Patrol-specific copy through the Patrol presentation helper, but compact
   empty-state spacing, icon treatment, and text hierarchy must compose the
   frontend-primitives-owned `EmptyState` `variant="panel"` instead of
   page-local centered icon/text shells. If Patrol needs a new badge shape,
   tone, loading indicator variant, or empty-state shell variant, extend the
   shared primitive and registry guard instead of adding page-local rounded pill
   spans, spinner spans, or empty-state wrappers.
   `RunHistoryPanel` is body content inside the Patrol workspace heading. It
   may show the selection hint and selected-run reset action, but it must not
   render a second `History` heading under the workspace's `Patrol history`
   heading.
   Current-findings empty states must reserve warning or error tones for active
   runtime/coverage/run problems; historical regressions without active
   findings remain informational history-review context, not an active-warning
   empty state.
   The default current-findings empty state must stay action-led and plain:
   use a verified `No current issues` state only when Patrol evidence supports
   it, use `Run Patrol to check everything` when coverage is incomplete, and do
   not reintroduce `Nothing needs attention` or all-clear phrasing for a view
   that is not fully verified. Degraded or incomplete checks should say Patrol
   could not finish a clean check and ask the operator to run Patrol to refresh
   current issues, rather than exposing verification/proof terminology.
   Visible Patrol summaries, runtime text, and run-history labels should use
   operator vocabulary such as `Patrol check`, `Targeted check`,
   `Follow-up check`, `Last check`, current issues, and outcomes. Backend
   fields may retain full-run, scoped-run, verification, and compatibility
   terminology where needed for cadence, audit, or API stability, but the
   default page must not ask ordinary operators to learn those internal proof
   distinctions before they can decide what to do next.
   A manual scoped Patrol request — such as an alert's "Have Patrol
   investigate" action — is one of those Targeted checks, not a new run kind:
   it must reuse the same scoped engine, run record, and run-history vocabulary
   as automatic alert-triggered work (governed by the `ai-runtime` manual Patrol
   route contract), so the operator still sees a `Targeted check` rather than a
   route-specific label.
   Active Patrol finding expansion must stay action-led: description, impact,
   recurrence summary, primary action, Assistant handoff, approval, and manual
   controls are acceptable default content, but raw lifecycle telemetry belongs
   to explicit all/resolved/history/run-review contexts instead of the default
   current-issues expansion. Current active Patrol issue rows must also surface
   the canonical primary next action, when one exists, without requiring the
   operator to expand details first. The collapsed issue row is not a Patrol
   process log: it may show severity, resource, recency, recurrence, and
   actionable states such as approval required, investigation running,
   verification needed, failed fix, or setup attention, but it must not expose
   generic `detected`, `review finding`, raw loop-state, investigation-status,
   investigation-outcome, or confidence badges on the default Patrol page.
   Patrol approval and remediation actions own approval, denial, reapproval,
   review, and Assistant handoff semantics, but their visible action chrome must
   compose the shared `Button` primitive for success, warning-solid, primary,
   secondary, ghost, disabled, focus, and compact action behavior instead of
   page-local button shells.
3. Keep remediation execution badge copy and severity styling aligned through `frontend-modern/src/components/patrol/RemediationStatus.tsx` and `frontend-modern/src/utils/remediationPresentation.ts`
4. Add or change Patrol header, status runtime-state presentation, or runtime provider action presentation through `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, `frontend-modern/src/features/patrol/PatrolIntelligenceBanners.tsx`, `frontend-modern/src/components/patrol/RunHistoryEntry.tsx`, `frontend-modern/src/utils/patrolRuntimePresentation.ts`, and `frontend-modern/src/utils/patrolRuntimeActions.ts`
   The header's primary run control must not become an unexplained disabled
   `Run Patrol` button when provider/model readiness blocks manual Patrol.
   In that state it becomes a direct `Fix setup` action to Provider & Models,
   while busy, disabled, unavailable, and already-running states can remain
   button states with their current reason attached.
   Patrol may show trigger mode and recent activity as compact factual context
   only inside explicit context/detail surfaces or actionable warnings sourced
   from Patrol run history and `status.trigger_status`; it must not reintroduce
   default status cards, suggested actions, or a separate activity tab to
   explain the same facts.
   Effective runtime blocks on event-triggered Patrol must pass through the same
   `status.trigger_status` presentation helper, but the default header and
   activity strip may surface them only when they explain an actionable manual
   Patrol block; background-only trigger pauses belong in secondary schedule and
   model diagnostics when manual Patrol still works.
   Patrol mode selection belongs to the always-visible header control;
   the header drawer is the secondary Schedule & model surface and may expose
   provider model, schedule, trigger tuning, and readiness errors, but it must
   not duplicate the four Patrol mode choices or reintroduce a save button
   for already auto-saving secondary fields. The provider model setting must lead with the effective
   Patrol/default model summary; the full model catalog is power-user detail
   behind an explicit change action, not the default content of the drawer.
   When Patrol mode is available, the Patrol mode selector must keep the
   default view to the selected mode and one short sentence. It must not render a
   secondary `Limits` disclosure, hard-limit matrix, or policy explainer on the
   primary operator surface; detailed policy configuration belongs in Patrol
   settings and governed action review, not beside the mode picker. That
   selected-mode sentence belongs in `patrolControlPresentation.ts`, not inline
   header markup, so the paid operator contract stays testable and does not
   drift into another activation/proof checklist. Plan-locked watch-only installs
   must keep the working surface clean of paid-feature surfacing — no disabled
   paid-mode matrix, no Pro badges, and no paid-mode disclosure — because Pro
   discovery belongs outside the daily-use Patrol surface. Watch-only copy in
   `patrolPagePresentation.ts`, `patrolControlPresentation.ts`, and
   `patrolAutonomyAvailability.ts` must describe Watch only in positive,
   capability-first language: Patrol checks infrastructure, records current
   issues, or records issues as it finds them. They must not repeat the same
   sentence across the header and mode control, repeatedly restate
   infrastructure-unchanged caveats in the default view, or imply Patrol's main
   value is a manual review chore.
5. Add or change Patrol header schedule and runtime presentation through `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, `frontend-modern/src/utils/aiPatrolSchedulePresentation.ts`, and `frontend-modern/src/utils/patrolRuntimePresentation.ts`.
   Patrol must not surface retired hosted-model credit badges or trial-like activation prompts in the normal self-hosted GA app, even when legacy transport fields are still present.
6. Keep Patrol and chat identifier-label presentation aligned through the shared `frontend-modern/src/utils/textPresentation.ts`
7. Keep Patrol and chat stream-matching / mention dedupe aligned through the shared `frontend-modern/src/utils/chatIdentifiers.ts`
8. Keep Patrol transport and payload changes aligned through the governed AI runtime and API contract transport surfaces

## Forbidden Paths

1. Reintroducing Patrol finding, investigation, approval, or run-history copy directly inside page components when canonical Patrol presentation helpers already own it
2. Duplicating Patrol finding severity, lifecycle, alert-identity, or approval-risk derivation outside the governed Patrol presentation helpers
3. Letting the Patrol page, local store, and findings UI drift into separate shadow truths for the same Patrol status or finding lifecycle state
4. Recreating Patrol run-history, status-bar, runtime-summary, or workspace-tab status/metadata badge shells locally instead of composing `StatusIndicatorBadge` and `MetadataBadge`
5. Recreating border-based Patrol or AI finding loading spinner shells locally instead of composing `LoadingSpinner`

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
   Selected run-history snapshots must render their recorded empty or
   unavailable finding state immediately; a global Patrol findings refresh may
   only block the selected run view while the run references finding ids that
   still need the direct Patrol findings payload.
   Patrol page refresh state must also separate background data loads from the
   operator-clicked Update status action: slow or stalled supporting
   intelligence reads may continue in the background, but they must not make the
   shared header action spin or stay disabled once Patrol findings and status
   remain visible. The header action is a status/history sync affordance; it
   must not read like another Patrol run or imply that it changes
   infrastructure.
   Patrol trigger-status copy in the default header and activity strip must stay
   actionable. Runtime policy pauses for alert/anomaly-triggered background
   checks, such as the local development safety guard, may explain a blocked
   manual run or advanced diagnostics, but must not be presented as ordinary
   operator guidance when manual Patrol still works. The Patrol page header
   labels this line as `Automation`, not `Trigger status`, and when a run is
   already in progress the summary must say new automatic and manual runs are
   paused until the current run finishes. The Patrol enabled toggle label must
   remain distinct from run progress; a running Patrol still labels the toggle
   state as `Patrol enabled`, while the run button and current-issues list own
   `Running`/`Patrol running` copy.
   `PatrolIntelligenceHeader.tsx` owns the `patrol-control` route target and
   the `operations-loop` compatibility anchor used by Patrol mode entry-point
   handoffs; the page must keep those anchors on the actual always-visible
   Patrol mode selector rather than on a separate onboarding banner or
   generic Patrol container. The Patrol surface must preserve the
   issue-evidence rule: Patrol mode can start from a generic Patrol run
   state, but investigation, approval, verification, and external-agent parity
   require a real finding, investigation, or governed outcome signal. The native
   Patrol workspace must derive current work from Patrol status, findings,
   approvals, and run history rather than loading the authenticated
   operations-loop status projection. That projection may remain available to
   API, MCP, telemetry, and adjacent commercial surfaces, but it must not create
   a separate default proof strip or bypass the current-issues workflow.
   Legacy `patrolAutonomy*` and `proActivation*` fields may be consumed only at
   compatibility edges, not as first-party Patrol workspace state.
   Current active finding detail must use the current active-finding count for
   operator copy instead of the aggregate historical issue-evidence count, so a
   page with one live finding and older verified history reads as current work,
   not as a pile of old issues.
   The journey must also consume `patrolControlValueState` from that
   projection when present, falling back to `patrolAutonomyValueState` and then
   `proActivationValueProofState` only for compatibility. A
   `governed_decision_recorded` state is a safe
   terminal rejection/decision to review, not a verified value outcome, and its
   primary action must return the operator to decision context rather than the
   resolved-outcome list. A `verified` state may be presented as verified Patrol
   operations value; legacy `verified_needs_mcp` payloads must be interpreted as
   already verified first-party Patrol value with MCP readiness left as optional
   external-agent context.
   Run history is secondary review context: the default history panel must lead
   with recent snapshots and keep older runs behind an explicit expansion, while
   preserving full forensic access for operators who deliberately open it.
   Individual run entries must read as an operator action record first (`All
clear`, `Found N new issues`, `Fixed N issues`, `N issues still open`, or
   `Patrol needs attention`) with checked-resource coverage as supporting
   detail. Trigger reason, duration, tool-call counts, tokens, triage flags,
   and raw tool traces are forensic context and must stay secondary to what
   Patrol did.
2. Keep Patrol-specific copy and badge logic inside the governed Patrol presentation helpers instead of page-local branches
   Patrol assessment copy must not present an all-clear health prediction while
   active Patrol findings or Patrol runtime issues are still present. The
   canonical summary helper owns that conflict resolution so the visible
   assessment title, description, and compact metrics all speak from the same
   current findings state. Patrol-owned runtime issues must stay
   distinct from infrastructure findings in assessment copy rather than being
   described as infrastructure warning findings about Patrol itself.
   Historical Patrol trust regressions must also suppress a green all-clear in
   the current findings empty state: `0 active findings` means no current Patrol
   work, while prior regressions remain Patrol history review context.
   Stale Patrol coverage must suppress the same green all-clear even when the
   last completed check found no issues. Freshness uses at least a 24-hour
   tolerance and otherwise allows two configured Patrol intervals before
   warning, so deliberately slower schedules are not mislabeled while an old
   default-schedule result cannot remain green indefinitely.
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
   On the Patrol page, that state must render as `Patrol setup issue` or
   `Patrol setup warning`, with provider/model context and the direct
   `Open Provider & Models` action. Raw diagnostic terms such as preflight,
   tool-call observation, or readiness internals belong in Provider & Models
   and backend/test contracts, not in the first-party Patrol operator banner.
5. Keep customer-facing Patrol naming product-first while preserving the
   operator-workbench job words inside the page: the daily work destination
   leads with `Patrol`, while summary copy, actions, empty states, and mode
   controls may describe findings, approvals, and work that needs attention.
   Reserve `AI` terminology for
   explicit provider/configuration settings, shared runtime internals, or other
   technical capability boundaries where the underlying ownership really is AI
   runtime rather than the Patrol product surface.
6. Keep Patrol brand icon accessibility contextual: `PulsePatrolLogo` remains
   label-bearing when the icon stands alone, but Patrol headings and actions
   that already include visible Patrol text must render the logo as decorative
   so accessible names do not repeat as `Pulse Patrol Patrol` or similar
   duplicated names.
7. Keep Patrol remediation copy operator-safe even while legacy transport
   fields retain `auto_fix` naming for compatibility: header autonomy controls,
   Pro-locked helper text, investigation outcome labels, and run-history badges
   must present the paid capability as safe remediation or remediation actions,
   not as a broad automation promise.
8. Keep the Patrol store aligned with the shared structured investigation
   record when transport carries one. `frontend-modern/src/stores/aiIntelligence.ts`
   may retain `investigationRecord` as data for Assistant handoff and Patrol
   presentation, but visible Patrol copy and Assistant handoff context
   must flow through the governed Patrol investigation-context helpers. Those
   helpers also own the Assistant drawer briefing content for Patrol records,
   including the rule that action artifact commands are summarized by count only
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
   Assistant's governed action artifact metadata must derive from the same
   structured handoff action after that recovery, so the briefing cannot
   contradict the action reference passed to chat execution while still leaving
   next-step reasoning to the configured LLM.
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
   Assistant handoffs from Patrol findings must also include a concise factual
   finding briefing derived from the unified finding and structured
   investigation record before the detailed finding context, so Assistant
   receives current risk, recency, evidence snapshot, verification summary,
   conclusion, latest lifecycle event, and governed approval/action artifact
   metadata instead of behaving like a generic chat over a pasted incident dump.
   Patrol is the scheduled
   probe, context assembler, and execution-governance owner; the configured LLM
   is the diagnostic and remediation-reasoning owner. Patrol handoffs may
   provide system context, resource posture, action posture, and governed tools,
   but must not synthesize, pre-fill, or auto-submit chat prompts, force active
   tool use, name a required tool path, show suggested prompt chips, or present
   a Patrol-authored remediation answer for the LLM to execute. Patrol
   runs must still call the configured model when deterministic triage is quiet,
   and unmatched deterministic signals may be returned as context for another
   model pass but must not be converted into Pulse-authored findings after the
   model declines to report them. Legacy unified AI-finding integration must
   likewise store model-reported findings as evidence and must not auto-generate
   remediation plans from finding metadata.
   Visible Patrol
   Assistant handoffs must not make the operator think Pulse has already
   produced the correct fix. The visible Assistant drawer briefing opened from a Patrol
   finding must be compact and source-named: current status/risk, one primary
   subject, and any approval-required boundary, with richer evidence, action
   artifacts, and command counts staying in model-only or governed action
   context rather than drawer chrome; prompt suggestions must not be generated
   for Patrol handoffs. When a
   structured investigation record is not available yet, the same Patrol-owned
   helper must still brief the operator from current finding facts such as
   active status, severity, recurrence, and loop state instead of opening a
   generic empty Assistant drawer. When a live pending Patrol approval exists
   for that finding, the visible Assistant briefing may include only safe
   approval metadata such as approval ID, pending status, risk, requested time,
   expiry, target label, generated approval summary, and command count; it must
   not copy the approval command payload into Assistant drawer prose. The
   model-only runtime briefing must apply that same
   recovered approval reference when adding factual governed action artifact
   metadata. The initial prompt for a Patrol finding may include governed action
   context when safe metadata is attached, but it must frame that context as
   input for the LLM to evaluate rather than as a Patrol-authored remediation
   answer. Finding
   handoffs must be assembled through the Patrol-owned handoff model so the
   prompt, visible briefing, model-only finding context, resource reference,
   bounded action reference, and request-local approval-required posture stay in
   sync. The model-only context may include
   current finding status, recurrence, investigation record facts, evidence,
   verification, approval state, dry-run posture, existing action artifact
   summary, target resource references, and governed action references without
   raw command payloads. Inline Patrol approval actions in
   `frontend-modern/src/components/patrol/ApprovalSection.tsx` that open
   Assistant must follow that same Patrol-owned handoff model rather than a
   prompt-only local shortcut: pass approval ID/status/risk/target plus safe
   summary/count metadata as review context, attach the target resource
   reference, include bounded `handoff_actions` for live approvals or structured
   action artifacts when present, force the request-local approval-required mode,
   attach the Patrol-owned visible drawer briefing for the pending approval or
   queued-fix recovery state, and never paste the approval command or
   action command text into the chat prompt. Existing remediation-plan or
   action-plan artifacts follow the same boundary: plan status, risk, and
   command counts are allowed as non-authoritative governed action posture for
   the LLM to critique, but visible handoffs must not render Patrol step lists
   or suggested prompt chips, and command or rollback command text stays in the
   governed remediation or approval surface. Generic finding
   discussion handoffs must also force request-local approval-required mode for
   any non-empty Patrol `finding_id`, including context-only findings and
   findings that reference a live approval, action artifact, fix outcome, or
   remediation plan, so default autonomous Assistant settings cannot bypass the
   Patrol action-governance boundary. The assembled handoff must still pass
   through the Assistant runtime's resource-policy sanitizer before prompt
   injection, so Patrol-owned prose
   cannot leak governed resource names, IDs, aliases, nodes, paths, or
   addresses outside the canonical policy boundary.
   If a `fix_queued` finding no longer has a live approval payload or detailed
   action artifact payload available, the recovery action must still open Assistant
   with the same Patrol-owned visible briefing and approval-required posture
   from current finding facts; it must not fall back to generic investigation
   chat or invite execution from missing command details.
   Patrol run-history Assistant handoffs are a separate run-context surface:
   the visible drawer must keep the `Patrol run attached` headline, may show the
   scoped run subject, safe runtime-failure/action label, and classified
   redacted failure summary, and must continue to omit raw provider payloads,
   command text, and remediation instructions.
   If the live approval is gone but the structured action artifact payload is still
   available, the recovery Assistant briefing must carry only safe action artifact
   metadata such as description, target, risk, rationale, destructive posture,
   and command count. Raw command text remains in the governed remediation or
   approval panel, while Assistant gets enough context to explain approval
   recovery and risk without becoming an execution surface.
   Generic finding-level Assistant handoffs must use that same safe metadata
   boundary when the list response lacks a full investigation record: they may
   hydrate the latest investigation session to recover action artifact summary,
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
   finding briefing card built by
   `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
   may include populated impact text in its bounded detail lines but must
   omit the placeholder copy from that compact card so it stays tight,
   leaving placeholder rendering to the full investigation surface.
   `frontend-modern/src/components/AI/FindingsPanel.tsx` is the canonical
   renderer for that operator-facing impact text on the unified findings
   surface: when an expanded finding card has populated `impact`, the panel
   renders an `Impact:` line between `Description` and `Recommendation`.
   On non-Patrol findings views, the same panel may surface
   `investigation_record.confidence` as a collapsed-row badge beside the
   investigation outcome. The default Patrol page keeps that process evidence in
   the expanded issue, run history, or Assistant context so current issue rows
   stay readable.
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
    `FindingsPanel.tsx` also renders the deterministic `capacityForecast`
    urgency line on the expanded finding card, projected through
    `presentCapacityForecast` (`patrolCapacityForecastPresentation`). That line
    is the operator's primary capacity-urgency signal: it states direction
    (Filling up / Stable / Declining), days-to-full, daily change rate, and
    current utilization from the backend-computed forecast, and it must take
    precedence over the model-authored description whenever a forecast is
    present. The panel must not invent, smooth, or client-side infer a forecast;
    a missing forecast simply means no urgency line renders.
   The Patrol page must not render a standalone Trust strip above the
   Findings/Runs tab bar or a parallel header trust line. The primary
   Patrol assessment readout is the default owner for current operator
   state, and may include high-signal trust facts such as
   `state.patrolStatus()?.trust.regressed_at_least_once` (the
   FindingsTrustSummary block on the patrol-status response) only as
   compact status text. New trust categories must extend the
   `FindingsTrustSummary` contract first rather than inventing
   per-page keys.
   Findings carry contextual Assistant entry points keyed on intent: the
   default "Discuss with Assistant" button opens an open-ended chat
   handoff, and a parallel "Explain" button opens the same handoff with
   a `PatrolAssistantFindingIntent='explain'` seed that asks the LLM to
   walk through what we know, why it matters, how confident the
   analysis is, what remains uncertain, and what the model would do next. Both
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
   Resource operator priority is a Patrol attention-queue tie-breaker, not a
   second queue model and not a severity rewrite. `resource_criticality` from
   the Patrol findings API normalizes to `UnifiedFinding.resourceCriticality`;
   `sortFindingsForAttentionQueue` may order same-severity findings by
   high/medium/default/low before runtime and recency, but severity,
   investigation outcome, approval state, and the direct Patrol findings source
   remain the canonical current-work model. The operator edits that priority
   and the associated note from `ResourceOperatorStateSection` on the resource
   detail drawer, alongside maintenance/offline/remediation-lock state, so the
   drawer stays the single resource-level Patrol intent surface.
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
   model-only context. Assessment-level handoffs must
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
   frame Assistant as explanation, prioritization, and model-owned next-step reasoning
   rather than a generic reactive chat
   box. That whole-surface assessment handoff must send safe
   `handoff_metadata.kind=patrol_assessment` so saved Assistant sessions restore
   as current-assessment context instead of becoming generic scoped context or
   an accidental single-finding session because one bounded action reference
   names a finding. Saved assessment and finding sessions must not expose or
   restore Patrol-authored next-step titles, recommendation detail, action
   labels, or app-route hrefs through `handoff_summary`; legacy stored
   recommendation fields are ignored rather than converted into hidden context.
   Assessment Assistant drawer briefings must stay compact and source-named
   instead of presenting a recommended step title, reason, route action, or
   suggested prompt chips as operator-facing answers, so
   provider-setting/runtime-visibility failures remain plain context for the
   configured model rather than Pulse-authored recovery guidance.
   When the current Patrol assessment is coverage-incomplete with no active
   infrastructure finding, the same handoff model may describe incomplete
   coverage as evidence, but it must let the configured model decide whether
   more verification, operator action, or governed tool use is needed. Execution
   or retry remains operator-controlled.
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
9. Keep the normal Patrol status summary plain and operator-first rather
   than a hero-style or decorative card surface. The default collapsed state is
   a compact readout, not a headline block: show the Patrol status label,
   current operator state, concise high-signal trust posture such as
   regressions when present, and score. Do not combine reassuring grade labels such as
   `Health A` with issue-state copy such as `Issues detected` in the collapsed
   line. Do not add a normal-path assessment details expansion: assessment
   explanation, verification detail, activity mix, and supporting metrics belong in the
   owning Findings, Runs, or `Details` surfaces
   instead of reopening the compact strip into a sparse status panel.

## Current State

Patrol provider-repair actions now use the canonical Pulse Intelligence >
Provider & Models route `/settings/pulse-intelligence/provider`. The legacy
`/settings/system-ai` route remains a compatibility alias for old deep links,
not a href emitted by new Patrol setup, finding, run-history, or summary
repair actions.

The finding lifecycle timeline renders backend lifecycle event types through
the shared label map in
`frontend-modern/src/utils/aiFindingPresentation.ts`
(`formatFindingLifecycleType`), with unknown types falling back to the
identifier formatter. The `content_replaced` event — emitted by the
ai-runtime findings store when a same-key re-detection's text is
substantially different from the existing finding (a key collision; see the
ai-runtime contract's Current State entry) — is labeled "Re-detected with
different details" so the operator timeline explains the shift in plain
language; the event's metadata carries the previous and new titles.
`FindingsPanel.test.ts` (`lifecycleLabels`) pins the label.

The Patrol findings panel
(`frontend-modern/src/components/AI/FindingsPanel.tsx`) dropped its "Open
related infrastructure / workloads / storage / recovery" cross-jump chip
strip on 2026-05-16 alongside the platform-first migration. The
`useResources` lookup that fed those chips and the supporting
`buildResolvedResourceSurfaceLinks` helper from
`frontend-modern/src/routing/resourceLinks.ts` were retired in the same
pass; the expanded finding body now keeps investigation in-place through
`getFindingPrimaryActionPresentation` (the canonical action href) plus the
shared manual-controls helper. New finding-detail affordances must not
reanimate the legacy chip strip or build URLs against the retired top-level
routes.

The Patrol page, store, findings UI, and run-history presentation had been
outside the governed subsystem map even though they are the top-level runtime
surface for Patrol intelligence. This contract now owns that orchestration and
presentation boundary while leaving shared transport and payload-shape
ownership in the governed AI runtime and API contract surfaces.

The Patrol control panel (`PatrolIntelligenceHeader.tsx`,
`usePatrolIntelligenceState.ts`) exposes the per-rule alert-trigger policy
directly under the Alert-Triggered Patrols toggle. A minimum-severity selector
("Investigate alerts at or above": Critical only / Warning and critical) renders
only while alert triggers are enabled and persists through
`AIAPI.updateSettings({ patrol_alert_trigger_min_severity })` with optimistic
state and revert-on-error, mirroring the existing trigger-toggle handlers. The
selector reads `patrol_alert_trigger_min_severity` from the settings response,
defaulting to critical-only, and must keep using the shared AI settings shape
rather than forking a patrol-local form.

The Patrol page now keeps the Patrol mode policy inline on the main surface,
not inside the secondary Schedule & model drawer: the first configurable decision remains
what Patrol may handle automatically (`Watch only`, `Ask first`,
`Safe auto-fix`, `Autopilot`), and there must be only one visible chooser for
that decision.
Per-resource automatic-action scope is configured in the canonical resource
drawer, not duplicated on the Patrol page. The drawer may expose only
capabilities whose backend contract declares auto-authorization eligibility,
requires an explicit capability allowlist, and may collect an optional daily
time/timezone window. Patrol mode remains the tenant-wide upper bound: a
resource opt-in cannot widen Watch only or Ask first, admit elevated work in
Safe auto-fix, bypass the full-mode unlock, or override Never auto-remediate.
Pending Patrol actions snapshot the bounded capability, tenant Patrol, and
resource operator authorities actually consulted at planning into the
server-authored action `policyDecision`. Its typed reason codes and revisions
are explanatory evidence for Task 11, including unavailable, missing,
emergency-stop, mode, allowlist, Never, and window posture. Planning and
dispatch share one pure evaluator, but dispatch re-fetches current inputs and
persists a separate authorization lease; the snapshot cannot authorize an
automatic action or suppress current-policy revocation.
Commercial, runtime, and documentation copy that describes this same decision
must also use those visible labels and the umbrella name `Patrol mode`, not
the retired `Only watch`, `Fix safe issues`, `Full control`, or generic
`Patrol control level` vocabulary.
Provider model, run schedule, trigger tuning, readiness checks, and saved
readiness issues belong to the secondary Schedule & model drawer. Within that
secondary drawer, the panel disambiguates the two alert-driven AI toggles by
scope rather than by near-identical names. The genuinely general path is
Alert-Triggered Patrols, which runs a focused Patrol investigation of the
alert's own issue. The Pro-gated `AlertTriggeredAnalysis` toggle presents as
"Container Update Risk" with copy scoped to container-update alerts, because the
enterprise `AlertTriggeredAnalyzer` only assesses `docker-container-update`
alerts and returns nil for every other alert type. New copy must keep the
container-update toggle scoped to its real Docker-update-risk capability instead
of drifting back to a generically named "Alert-Triggered Analysis" control that
collides with Patrol's own alert investigation.

The route file `frontend-modern/src/pages/AIIntelligence.tsx` is now also a
thin shell that delegates to the feature-owned
`frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx`, so Patrol
runtime state and presentation no longer accumulate directly in the route
component itself. The governed customer-facing route for that shell is now
`/patrol`, while retired `/ai` browser entry points stay unregistered instead
of remaining as compatibility redirects or a second product path.
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
provider settings boundary, but the underlying AI runtime catalog must stay shared
with chat and AI settings. The advanced Patrol model selector must remain
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
The primary summary strip is descriptive only: it surfaces the current Patrol
assessment label and compact counts, while action choices remain in the
Findings/Runs workspace, header controls, and the LLM-driven Assistant chat.
`frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts` may
package the current Patrol assessment, verification posture, latest run,
secondary investigation context, bounded recent-change and learned-correlation
evidence, bounded active-finding summaries, deduped resource references, and
safe structured approval/action references as model-only context, but visible
drawer copy must not promote Pulse-authored next-step metadata, action chips, or
suggested prompts as the answer. Command payloads stay out of drawer chrome.
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
That same browser proof now covers the Patrol control and advanced-settings
split. The advanced Patrol settings drawer must stay within the desktop
viewport, avoid duplicating the inline Patrol control policy, expose
provider/model, schedule, trigger, and user-level model checks directly, and surface
the backend's concrete license/validation reason when a settings change is
rejected instead of replacing it with a generic `Failed to save advanced
settings` toast. That inline failure may
handoff to Assistant only as model-only explanation context: raw command,
script, credential, and provider-detail payloads stay redacted, Assistant opens
with `autonomousMode:false`, and the Patrol control panel closes so the operator
is not left behind an overlapping popover. Patrol-control save-failure handoffs
must keep the compatible `handoff_metadata.kind=patrol_configuration_failure` plus only
the runtime-failure boolean needed for drawer/session presentation, so the
saved session restores as a Patrol control issue without carrying raw
provider, credential, command, or retry payloads into the browser.
Successful provider-model saves that return
`patrol_readiness.status=not_ready` are still Patrol control issues, not silent
successes: the Patrol popover must keep the saved provider/model visible,
render a `Patrol control needs attention` inline state with the returned
readiness cause and summary, and hand off to Assistant as a model-only Patrol
control issue rather than as a save failure.
A separate browser proof in
`tests/integration/tests/78-monitor-first-patrol-workbench.spec.ts` keeps the
authenticated launch workbench monitor-first. When the runtime has monitored
Proxmox resources, legacy infrastructure entry points must resolve to the
Proxmox monitoring lens before Patrol or Assistant chrome, while the stable
Patrol tab may still expose the open-work count. Calm-day copy such as `No
current issues` and checked-resource coverage belongs inside the Patrol route
after the operator chooses Patrol; the monitor lens must not become a hidden
Patrol all-clear page, and Assistant launchers must stay scoped to the current
surface.
The readiness contract now applies before Patrol work is admitted, not only
after a page render: recoverable Patrol provider/model settings saves must
persist and echo structured readiness cause metadata, manual run requests must
return the structured readiness reason if a stale UI still submits, and
scheduled or scoped alert/anomaly runs must skip before calling the model while
preserving the blocked reason and cause in Patrol status. The Patrol
control state owner must also clamp stale investigation/remediation
autonomy back to findings-only `monitor` and clear stale full-mode unlock state
before persisting Patrol control when the safe-remediation entitlement is not
effective, so an expired or downgraded plan cannot turn a recoverable control
review into a Pro-only save failure. The same inline error
surface must render any Patrol readiness
provider, model, and summary carried by the failure object, and keep the
provider-settings action immediately available before the operator chooses to
open the Assistant handoff.
The Patrol mode selector in that header and configuration dialog must
compose the shared `frontend-modern/src/components/shared/FilterButtonGroup.tsx`
instead of rebuilding a local active-button group. The wide default Patrol
header uses the segmented layout; the constrained configuration dialog may use
the shared prominent layout so all four mode labels remain readable without
inventing a Patrol-local selector. Patrol owns the default visible four-level
policy presentation (`Watch only`, `Ask first`, `Safe auto-fix`,
`Autopilot`), entitlement locks, and the rule that choosing the highest
Autopilot level sends `full_mode_unlocked:true` while choosing any lower level clears that
acknowledgement. The shared primitive owns pressed-state semantics,
disabled-option behavior, and active/inactive selector styling.
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
must not show Pro trial CTAs, upgrade links, paid helper copy, or a plan-lock
upsell banner in the default Patrol workflow unless hosted mode, an explicit
commercial handoff, or an active entitlement makes those actions relevant.
That degraded empty-state copy must also interpret the finding state rather
than simply replaying the primary assessment sentence verbatim: when coverage
is incomplete, the findings panel should tell the operator to run Patrol to
complete coverage before trusting the all-clear. The page must not duplicate
the assessment prediction as a second independent status surface above current
issues.
Secondary metric strips must not render `No issues found` when the same
governed overall-health summary says coverage is incomplete or health still
requires attention; compact assessment derivation belongs to the shared
`frontend-modern/src/utils/patrolSummaryPresentation.ts` helper for Assistant,
history, and explicit context consumers rather than the default Patrol page.
Any explicit context assessment shell must also stay inside the shared Pulse
card language. It should use the same neutral `bg-surface` page-card base as
adjacent Pulse surfaces, carrying severity through compact header accents, icon
treatments, and badges instead of tinting an entire full-width assessment panel
as a one-off warning banner.
That supporting score chip must also avoid overstating infrastructure truth.
When the current state is dominated by incomplete coverage or Patrol-owned
runtime failures, the chip should read as an `Assessment` grade rather than an
`Health` grade; `Health` belongs to verified healthy infrastructure states.
That same helper also owns explicit assessment explanations. Assessment
consumers must not pair an `Issues detected` headline with a raw coverage-only
`overall_health.prediction` sentence from a separate source; when active
findings and incomplete verification are both true, the assessment should
describe both in one canonical message.
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
Runtime setup copy from that helper must stay neutral across watch-only and
paid control modes: it should explain that Patrol needs runtime or provider
setup before it can check infrastructure reliably, not promise investigation,
approval-backed fixes, or autonomous action from a locked install.
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
lead with the same direct `Open Provider & Models` action that the top
assessment uses, and that action must use the canonical Pulse Intelligence
route `/settings/pulse-intelligence/provider` rather than the legacy
`/settings/system-ai` compatibility alias. Runtime/setup findings in the
default current-issues flow must stay primary-action-only: they should not add
a visible `Open in Assistant` secondary button or a generic `Manage` menu
beside the direct setup action.
Assistant can still receive runtime context from the broader Patrol/Assistant
handoff paths, but a setup impairment should not read as a menu of workflows.
When setup is the only active Patrol work, the Patrol page should collapse that
state into one `Fix Patrol setup` workspace task with the direct
`Open Provider & Models` action; the readiness banner must not duplicate that
same action above it.
The current-issues list must consume that same canonical primary action for
exactly one active runtime finding, so the visible next step opens Patrol
provider settings directly instead of making the operator first click through a
generic findings review CTA.
The setup-only Patrol workspace must keep that same single-path focus: when
Patrol cannot check infrastructure because provider setup is blocked, the
workspace may show the current setup issue and the provider-settings action, but
must not expose run history as a competing action until Patrol can actually
check infrastructure or the operator has opened a specific run record.
That same contract must fail closed on manual lifecycle controls too. Patrol
runtime findings are Patrol-owned impairment signals, not ordinary estate
findings, so the findings list must not offer generic acknowledge, snooze,
dismiss, resolve, or suppress controls for them. The correct operator path is
to fix Patrol provider configuration in Pulse Intelligence > Provider & Models
settings and rerun Patrol, optionally adding context notes, rather than hiding
the runtime issue.
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
That same summary surface must also explain what Patrol actually checked
without turning coverage mechanics into the default product language. Recent
run history should drive a visible check summary that tells the operator
whether Patrol recently completed a clean broad check, only ran targeted alert
or anomaly-triggered checks, or ended its most recent broad check with errors,
so the page does not leave trust and coverage as implicit background knowledge.
When same-day run history shows both a recent broad check and a burst of
targeted or follow-up activity, that same surface should expose the recent
activity mix in check language instead of asking operators to reconcile a
`Recently verified` headline with a busy Patrol strip elsewhere on the page.
Fix-verification checks belong to that same explanation layer as follow-up
checks, not as evidence of a fresh full-estate sweep.
The same hierarchy applies to the `Details` supporting context.
Correlations, recent changes, and policy posture are secondary explanation for
deeper investigation, so the default workspace may expose only the compact
context affordance near the findings/history controls. When that disclosure is
expanded, the page must render the panel as the immediate next workspace content
and explicitly tell operators that the selected finding or run record remains
the source of truth, while nearby activity, related patterns, and inspection
boundaries are context Patrol considered and do not change the finding or count
as a fresh Patrol run.
When Patrol is healthy and fully verified, that supporting-context disclosure
should stay out of the main page flow instead of advertising a second parallel
Patrol workflow with nothing active to explain.
That same operational context belongs behind the same secondary disclosure as
verification, not as a default full-width strip that competes with the findings
workspace. The workspace and shared
`frontend-modern/src/utils/patrolRunPresentation.ts` helpers may carry latest
run kind/result, scoped-trigger state, and circuit-breaker warnings as factual
support when the user opens the relevant context or when the page must surface
an actionable warning. They must not make activity-mix labels, raw run-count
breakdowns, health scores, or loop-proof counters default-page content.
That same secondary area should simplify by consolidation rather than
deletion. Patrol should keep meaningful run and trigger facts available, but it
should stop repeating the same runtime story as a summary card, a metric row,
and a separate status strip all at once.
Do not reintroduce a separate Patrol status strip for this context. Compact
activity facts belong in run history, selected finding details, or explicit
secondary context, not as a second Patrol verdict label.
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
Those runtime facts must stay aligned with Patrol mode copy. The
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
That same empty-state contract must become run-record-aware when the user is
reviewing an explicit Patrol run. A selected run with an explicit empty
`finding_ids` record should explain that no findings were recorded for that run
and, when applicable, carry the canonical run coverage summary and issue caveat
instead of falling back to the generic `No Patrol findings to display` copy.
That same internal `finding_ids` scoping must apply to the findings control bar
too. When the user is looking at an explicit run record, filter pills and their
attention or approval counts must derive from that run-scoped finding set
rather than borrowing global Patrol finding counts from outside the selected
run.
That same run-scoped model must also apply to the `Findings` tab badge itself.
When the selected run carries an explicit empty `finding_ids` record, or when
the run lacks `finding_ids` entirely, the tab must fail closed instead of
borrowing global active-finding counts and tones from outside the selected run.
That same scoped count model must drive conditional findings filters too. The
`Needs Attention` and `Approvals` buckets, their counts, and the auto-reset
logic that returns the operator to `Active` when a bucket disappears must all
read from the same run-scoped count source rather than mixing run-record pills
with global queue truth.
That same fail-closed `finding_ids` rule applies to inline run-history findings
as well. Expanded run cards should route through the same run-aware findings
surface as the primary workspace, so legacy runs without `finding_ids` still
show an explicit finding-record-unavailable state instead of disappearing or
being coerced into an empty findings record.
That same rule applies to the primary findings workspace when a run is
selected. A selected run without `finding_ids` must not borrow global Patrol
findings, filter buckets, or queue counts; the findings surface should enter an
explicit finding-record-unavailable state instead.
That same unknown-record state should be visible in the selected-run shell too.
When the operator is reviewing a legacy run without `finding_ids`, the
selected-run banner should explicitly say that the finding record is unavailable
instead of implying a fully verifiable run-scoped findings view.
That same trust rule applies inside expanded run-history narratives. A legacy
run without `finding_ids` must not render an `All clear` conclusion
just because its aggregate counters are zero; the narrative should explicitly
state that run-specific findings could not be fully verified.
That same truthfulness rule applies to the expanded run outcomes strip. Legacy
runs without `finding_ids` must not render a green `All clear` outcome badge
from zero aggregate counters; they should show an explicit finding-record
unavailable state instead.
That same caveat belongs in compact latest-run summaries too. When the most
recent Patrol run lacks `finding_ids`, the status bar's latest-run segment
should say that the finding record is unavailable instead of
flattening the run into a plain healthy-looking summary.
That same caveat belongs in collapsed run-history rows. Legacy runs without
`finding_ids` must carry an explicit finding-record-unavailable marker in the
top row instead of looking like a clean zero-findings run until expanded.
That same rule applies to run-status badges. A legacy run without findings
records must not keep a green `healthy` badge when the surrounding UI is
saying findings verification is unavailable; the canonical run-status
presentation should downgrade that state to a neutral `completed` badge.
That same truthfulness rule applies to the run-history shell copy. The `Recent
patrol runs` helper text must not present run review as a findings filter; it
should frame history as Patrol run records, and when visible runs include
legacy entries without `finding_ids`, or when the selected run itself lacks a
finding record, the shell should say so explicitly.
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
That same finding row should avoid redundant state stacking. Active Patrol issue
rows must not add baseline `detected` or raw loop-state badges; non-Patrol
finding rows may still use loop-state badges only when they communicate a more
specific state than acknowledged/current observation metadata.
Collapsed finding-row badges now follow the shared metadata badge boundary:
`frontend-modern/src/utils/aiFindingPresentation.ts` owns Patrol-specific
status/source/severity/investigation/confidence/workflow state-to-tone mapping,
while `frontend-modern/src/components/AI/FindingsPanel.tsx` renders those chips
through `MetadataBadge` with the shared outlined compact appearance. Patrol
workflow chips may appear only for specific actionable or recorded states such
as `Review approval`, `Verify outcome`, and `Outcome recorded`. Plain active or
detected findings must not add passive `Detected`, `Review finding`, or other
generic workflow badges beside the severity badge. Grouped current findings
must use issue/resource language, not internal signal-count language. New
finding-row badges must extend the Patrol presentation helper and
`MetadataBadge` tone vocabulary rather than restoring local bordered xs spans.
The visible finding summary is also the default disclosure target for the
issue: clicking it, pressing Enter, pressing Space, or using the explicit
View details/Hide details control must open the same detail panel. Inline
approval, setup, and manual action controls stay separate siblings so those
actions do not accidentally toggle the issue details.
The inline investigation surface follows the same boundary:
`frontend-modern/src/components/patrol/InvestigationSection.tsx` must compose
`MetadataBadge` for live investigation status/outcome chips, durable Patrol
record status/outcome/confidence chips, and tool metadata chips. Patrol may
derive labels in `patrolInvestigationContextModel.ts` and tones in
`aiFindingPresentation.ts`, but the visible chip shell stays primitive-owned.
Approval and run tool-call detail chips follow the same rule:
`frontend-modern/src/utils/approvalRiskPresentation.ts` and
`frontend-modern/src/utils/patrolRunPresentation.ts` own risk/result labels and
semantic badge tones, while `ApprovalBanner`, `ApprovalSection`, and
`RunToolCallTrace` render those chips through `MetadataBadge`.
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
flattening proactive Patrol work into generic Assistant chat activity. Rejected
Patrol fix approvals must remain first-class Patrol outcomes: the backend
denial path records the shared action-audit decision, marks the owning finding
`fix_rejected`, and the Patrol control loop counts that state as a governed
action rejection while keeping approved-action verification separate. Pending
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
making recent changes, learned correlations, and policy posture available only
through the on-demand `Details` context inspector, so expansion
lane concepts stay available for deeper investigation without reading as the
headline Patrol product story.
That secondary investigation-context summary now also routes through the
dedicated `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts`
owner, so the Patrol hook composes one canonical payload-to-summary derivation
instead of rebuilding recent-change, correlation, and governed-resource count
copy inline.
The Patrol page's run-history tab label is now also tightened to `Runs`, while
the underlying run-history panel remains canonical for run-record review,
internal `finding_ids` scoping, and tool-call inspection. That copy change is
intentional: run history is support context for Patrol findings, not a peer
primary workflow beside findings, and visible copy should frame selected runs
as Patrol records rather than snapshot filters.
Within that same run-history surface, coverage copy must stay canonical across
the header chips, narrative summary, and selected-run record. Scoped runs
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
The Patrol page header copy must also name the surface boundary, not
the runtime controls it happens to embed. The canonical
`PATROL_PAGE_DESCRIPTION` (and the matching `PATROL_PAGE_TITLE_TOOLTIP`)
must frame Patrol as scheduled infrastructure probing and context
assembly for the configured model, with fixes kept behind the approval
policy. The tooltip on the page-header title must read the same string
from the canonical helper rather than maintaining a parallel copy, so
hover and inline never tell different stories about what Patrol does.
The Patrol page header must not add a second trust-at-a-glance line under
the page title. Current active/regressed/finding posture belongs to the
primary assessment readout and the finding rows, so the header remains
focused on product title, recency, and route controls instead of repeating
the same trust counters above the workspace tabs.
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
Patrol control may receive a starter count from the canonical operations-loop
status projection through `patrolAutonomy*` compatibility fields, with the
legacy entry-point starter count treated as the same entry-point context
when the primary field is absent. That count is only
journey copy; Patrol must still require active finding, pending approval,
governed action, or verified outcome evidence before it marks the Patrol issue
step complete or opens the Assistant handoff as a completed loop stage. Current
active findings and pending approvals must keep the journey on current work even
when older completed/resolved proof is present. The
projection may also report Patrol control completed-loop and resolved-loop
counts through compatibility fields, with legacy Pro activation counts as fallback aliases. Patrol may use
the completed-loop count only as content-free terminal decision detail after
the loop already has Patrol issue evidence, contextual collaboration, and
either a rejected governed decision or an approved governed decision with
verified outcome evidence. Patrol may use the resolved-loop count only as
stricter approved-and-verified detail after the loop also has an approved
governed decision and verified outcome evidence.

The typed `ActionReference` is the primary Patrol workflow model in both the
finding row and expanded action review. Pending actions say approve or reject;
planned or approved actions say run; executing actions say running; terminal
actions present verified, failed, or honestly inconclusive verification. The
expanded `ApprovalSection` renders the canonical plan, approval floor,
preflight, safety checks, verification steps, and rollback availability, then
uses only `/api/actions` decision/execute calls. Legacy `ProposedFix` and
`ApprovalID` data may explain historical records but must never reveal a raw
command, present an approve/run control, or reconstruct an executable action;
when the typed reference is absent the UI says action details are unavailable
and offers an Assistant handoff. Collapsed-row attention state must consult the
same investigation action reference, so it cannot claim there is no approval
while the expanded panel has one. Browser proof must exercise pending,
terminal-verified, and legacy-history states rather than judging only source.

Backend Patrol finding reconciliation now reads `ActionResultV2`: execution
failed or known-not-run maps to fix failure; confirmed verification maps to
verified; contradicted verification maps to verification failure; and
not-attempted or inconclusive verification remains verification unknown. A
successful execution never overwrites contradictory verification. Task 11
still owns browser wording and proof for the distinct terminal states.

Patrol now consumes the server-derived effective Autopilot mode. Requested
`full` is admitted only with a current persisted human acknowledgement and
exact activation for the same actor credential and organization; legacy
booleans, revocation, expiry, version rotation, and malformed evidence fall
back to approval mode before policy-authorized action submission. The accepted
limits explicitly allow inconclusive verification and never turn execution
success into outcome truth. Task 11 still owns acknowledgement presentation,
cancel/no-submit browser proof, and device coverage; M7 remains open until that
work and Task 12 certification are complete.
