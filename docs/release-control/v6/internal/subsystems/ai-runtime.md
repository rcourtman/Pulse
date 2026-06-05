# AI Runtime Contract

## Contract Metadata

```json
{
  "subsystem_id": "ai-runtime",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/ai-runtime.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": ["api-contracts", "cloud-paid", "frontend-primitives"]
}
```

## Purpose

Own Pulse Assistant and Patrol backend runtime behavior, AI orchestration,
runtime cost control, and shared AI transport surfaces.

## Canonical Files

1. `internal/ai/`
2. `internal/config/ai.go`
3. `internal/api/ai_handler.go`
4. `internal/api/ai_handlers.go`
5. `internal/api/ai_hosted_runtime.go`
6. `internal/api/ai_intelligence_handlers.go`
7. `frontend-modern/src/api/ai.ts`
8. `frontend-modern/src/api/aiChat.ts`
9. `frontend-modern/src/api/patrol.ts`
10. `frontend-modern/src/components/AI/AICostDashboard.tsx`
11. `frontend-modern/src/components/AI/Chat/`
12. `frontend-modern/src/utils/aiChatPresentation.ts`
13. `frontend-modern/src/utils/aiControlLevelPresentation.ts`
14. `frontend-modern/src/utils/aiCostPresentation.ts`
15. `frontend-modern/src/utils/aiProviderHealthPresentation.ts`
16. `frontend-modern/src/utils/aiProviderPresentation.ts`
17. `frontend-modern/src/utils/aiSessionDiffPresentation.ts`
18. `frontend-modern/src/utils/textPresentation.ts`
19. `frontend-modern/src/stores/aiRuntimeState.ts`
20. `frontend-modern/src/stores/aiChat.ts`
21. `docs/AI.md`
22. `pkg/aicontracts/investigation.go`

## Shared Boundaries

1. `frontend-modern/src/api/ai.ts` shared with `api-contracts`: the AI frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/api/patrol.ts` shared with `api-contracts`: the Patrol frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
3. `frontend-modern/src/stores/aiChat.ts` shared with `frontend-primitives`: the assistant drawer and session store is both an AI runtime control surface and a canonical app-shell presentation boundary.
4. `internal/api/ai_handler.go` shared with `api-contracts`: Pulse Assistant handlers are both an AI runtime control surface and a canonical API payload contract boundary.
5. `internal/api/ai_handlers.go` shared with `api-contracts`: AI settings and remediation handlers are both an AI runtime control surface and a canonical API payload contract boundary.
6. `internal/api/ai_intelligence_handlers.go` shared with `api-contracts`: AI intelligence handlers are both an AI runtime control surface and a canonical API payload contract boundary.

## Extension Points

1. Add or change chat runtime, Patrol orchestration, findings generation, or remediation behavior through `internal/ai/`
2. Add or change canonical AI provider config, provider-scoped model selection, or runtime auth/base-URL defaults through `internal/config/ai.go`.
   Ollama request keep-alive is a provider runtime option owned by this path:
   `internal/config/ai.go` stores the value, `/api/settings/ai` exposes it,
   and `internal/ai/providers/ollama.go` is the only layer that turns it into
   the Ollama `keep_alive` request field. An empty configured value means
   Pulse omits `keep_alive` so the Ollama server default applies.
3. Add or change Pulse Assistant request flow through `internal/api/ai_handler.go`, `frontend-modern/src/api/ai.ts`, and `frontend-modern/src/api/aiChat.ts`
4. Add or change Patrol, alert-analysis, or remediation transport through `internal/api/ai_handlers.go`, `internal/api/ai_intelligence_handlers.go`, and `frontend-modern/src/api/patrol.ts`
   Provider preflight diagnostics returned from `internal/api/ai_handlers.go`
   must reuse the Patrol runtime failure classifier in `internal/ai/` and
   expose only safe operator-facing cause, summary, recommendation, model, and
   action fields. Raw provider response bodies and transport errors may be
   logged server-side or attached as redacted internal Patrol evidence where
   governed, but they must not be returned through the browser provider-test
   contract.
   Patrol findings history transport must stay bounded when resolved findings
   are included: `/api/ai/patrol/findings?include_resolved=1` defaults to a
   200-finding limit and caps explicit limits at 500, and the frontend Patrol
   client/store must send the same bounded history request once the Resolved or
   All view has made expanded history sticky. Per-finding suppression creation
   is similarly narrow by default: the browser helper must require a concrete
   resource ID and category, while backend broad/wildcard suppression scopes
   require an explicit `allow_broad_scope` request from a dedicated rule
   management surface.
5. Add or change AI usage/cost dashboard presentation through `frontend-modern/src/components/AI/AICostDashboard.tsx` and `frontend-modern/src/utils/aiCostPresentation.ts`
6. Add or change AI provider, control-level, or chat/session presentation through `frontend-modern/src/components/AI/Chat/`, `frontend-modern/src/utils/aiProviderPresentation.ts`, `frontend-modern/src/utils/aiProviderHealthPresentation.ts`, `frontend-modern/src/utils/aiControlLevelPresentation.ts`, `frontend-modern/src/utils/aiChatPresentation.ts`, and `frontend-modern/src/utils/aiSessionDiffPresentation.ts`
   AI provider/model presentation must preserve the model transport route when
   the selected provider is a gateway. OpenRouter-routed model IDs such as
   `openrouter:deepseek/...` must render with an explicit `via OpenRouter`
   label in the shared picker, System AI settings status, and inherited default
   descriptions unless the server-supplied model name already carries that
   route. Direct provider models must not gain a gateway label.
7. Keep AI chat presentation helpers aligned through `frontend-modern/src/components/AI/Chat/` and the shared `frontend-modern/src/utils/textPresentation.ts`
8. Keep assistant drawer context, session, and org-switch reset state aligned through the shared `frontend-modern/src/stores/aiChat.ts` boundary instead of letting `frontend-modern/src/App.tsx`, `frontend-modern/src/AppLayout.tsx`, or feature callers fork their own assistant shell state
   That shared drawer ownership also covers passive resource reads while the
   shell is mounted but closed. `frontend-modern/src/components/AI/Chat/`
   may consume the live websocket snapshot or the existing unified-resource
   cache for assistant context and suggestions, but it must not reopen
   `useResources()` or trigger a second unfiltered `all-resources` REST fetch
   just because the drawer component is present in the app shell.
   The same app-shell boundary keeps Patrol/Assistant utility navigation
   accessible-name safe: labelled icon SVGs may remain meaningful when rendered
   standalone, but `frontend-modern/src/AppLayout.tsx` must treat them as
   decorative inside tabs so the announced tab name comes from product chrome
   and meaningful badge text rather than icon title duplication. Scoped
   approval handoffs sourced from Patrol, active alerts, or alert incident
   timelines must render as source-named investigation handoffs in the drawer
   instead of generic dashboard briefs. Source-owned handoff helpers may attach
   bounded model-only context, resources, action references, and metadata, but
   they must not synthesize, prefill, or auto-submit a user prompt. The drawer
   presentation must stay compact: source, status, one primary subject, and an
   optional safe route link. It must not render Patrol-authored remediation
   steps, evidence chips, command summaries, recommendations, or suggested-prompt
   chips as the answer. Patrol assessment handoffs must use the same
   `patrol-assessment` target identity for live opens and restored sessions
   rather than inheriting retired dashboard context. While such a handoff is
   attached, the Assistant empty
   message state must also remain source-named and must not fall back to generic
   cluster/system starter prompts that compete with the attached briefing.
   Structured Patrol run and Patrol configuration handoffs may render bounded,
   redacted diagnostic lines in the drawer when they are opened directly from
   Patrol runtime/configuration surfaces, but the attached headline must remain
   source-owned (`Patrol run attached`, `Patrol configuration ... attached`)
   and the drawer must still exclude raw provider payloads, commands, and
   Patrol-authored remediation steps.
   Reloaded Assistant sessions may consume the backend-owned
   `handoff_summary` only as safe presentation state and a Patrol finding
   pointer; hidden model context, command payloads, preflight data, and action
   results stay backend-owned and must not be reconstructed in the browser.
9. Add or change public AI overview wording through `docs/AI.md`; it may
   describe Assistant and Patrol capabilities, but it must not revive legacy
   commercial shorthand such as `incident memory` as a current product promise.
   The public overview must also preserve the model-owned AI boundary: Pulse
   Assistant and Patrol provide governed context, tools, safety gates,
   approval state, and audit trails, while the configured LLM owns diagnosis,
   prioritization, remediation reasoning, and tool choice. Public copy must not
   present Pulse code as the intelligence engine, a prompt-keyword router, a
   learned operational-meaning engine, or a deterministic finding author.
10. Platform/runtime top-level pages registered in
    `frontend-modern/src/App.tsx` and the primary tab list in
    `frontend-modern/src/AppLayout.tsx` must keep the AI launcher chrome
    intact: they extend `aiChatStore`-aware route shells rather than mounting
    bespoke layouts that suppress the Assistant launcher, Patrol entry, or
    the AI keyboard-shortcut handlers. New per-platform sub-routes inherit
    the shared AI-aware chrome by virtue of routing through `AppLayout`, and
    must not introduce a parallel chat surface, launcher button, or model
    picker on the platform page itself; cross-platform AI guidance stays
    routed through Assistant and Patrol.
    The primary-nav demotion of Infrastructure / Workloads / Storage /
    Recovery does not change Patrol or Assistant addressability: both
    surfaces remain reachable through the utility tab strip in
    `AppLayout.tsx` and via the canonical Patrol path, and platform pages
    must not replicate Patrol findings, Assistant prompts, or AI launcher
    affordances inside their own chrome.
    The vSphere Networks sub-route follows the same AI runtime boundary as the
    vSphere overview, datastore, health, and activity routes. Network rows may
    seed Assistant or Patrol context only as shared `network` unified-resource
    references read through `/api/resources*` and the common handoff payloads;
    the VMware page must not introduce VMware-local AI prompts, a provider
    model picker, or a vSphere-specific chat/runtime route just because
    networks are now rendered as a first-class API-native table.
    The frontend-primitives-owned Machines surface follows the same AI runtime
    boundary while retaining the internal `standalone` route/id contract: route
    registration and primary-tab chrome may expose the support-manifest `agent`
    platform plus agentless availability endpoints, but the page must not add an
    agent-specific Assistant prompt surface, AI launcher, model picker, or
    browser-owned model context. Machine rows may seed Assistant or Patrol only
    through the shared unified resource handoff contracts. Its IA, primary
    shell position, and landing eligibility belong to `frontend-primitives`;
    those changes must not alter Patrol or Assistant utility-tab ordering,
    launcher visibility, or shared keyboard handling in
    `frontend-modern/src/AppLayout.tsx`.

## Forbidden Paths

1. Leaving new `internal/ai/` runtime entry points unowned under broad architecture or generic API ownership
2. Duplicating AI orchestration, Patrol runtime, or cost-tracking logic outside `internal/ai/`
3. Treating AI transport files as payload-only boundaries when they also define live runtime control behavior

## Completion Obligations

1. Update this contract when canonical AI runtime or transport entry points move, including transport-level provider request-shape changes such as OpenAI-compatible `tool_choice` handling, runtime-failure classification splits (for example separating tool-choice request rejection, no tool-capable endpoint, and generic model-level lack of tool support into distinct causes), Patrol-specific verification surfaces such as `POST /api/ai/patrol/preflight` that exercise the full chat-completions path with a minimal tool definition rather than only listing models, Patrol-preflight cache observability where the AI Service caches the most recent preflight outcome (success, soft warning, or classified failure) and the AI settings response surfaces it as `patrol_preflight` so the UI can hydrate a "last verified" indicator without forcing operators to re-run preflight on every page load, the auto-trigger contract on `HandleUpdateAISettings` where the save handler runs `TriggerPatrolPreflightAsync` only when the change actually moved Patrol transport (model swap, provider key for that model changed, or assistant just enabled with a Patrol model) so routine settings saves do not burn provider tokens, the startup-seed contract where the AI Service handler dispatches the same async preflight on Pulse boot when assistant is enabled and a Patrol model is configured so the cache is populated for the first `/api/settings/ai` poll after a restart instead of blanking back to "never verified", the readiness-integration contract where the `tools` check in the Patrol readiness payload consults the cached preflight and surfaces the classified evidence (success, soft warning, or failure with classified summary plus "last preflight <age>") for the configured provider+model when available (falling back to the static `PatrolToolReadinessForModel` classifier only when the cache is empty or holds a result for a different model), the stateless-Patrol-input contract where `ExecutePatrolStream` must pass only the current run's user prompt into the agentic loop rather than reloading the persisted `patrol-main` session history (so a prior run that ended with orphan `tool_calls` cannot poison every subsequent run with malformed conversation structure), and the deterministic-resolve-gate contract where the `patrol_resolve_finding` tool adapter rejects LLM-driven resolves of event/persistent category findings (`backup`, `reliability`, `security`, `general`) when a deterministic verifier exists for the finding's key and that verifier either still detects the failure signal **or returns an inconclusive result** — preventing the LLM from optimistically resolving a finding its current investigation simply didn't re-surface, which was the source of the "Backup failed" flap (detected → auto-resolved → re-detected ten times in a day before this gate). The fail-closed-on-inconclusive policy treats verifier errors (timeouts, executor unavailability, transport faults) as "we don't know" rather than "go ahead": resolution of an event/persistent finding is effectively permanent (next detection registers as a regression and inflates counters), so the safe default is to refuse and require either a successful re-verification or operator action, the assessment-recovery contract where the overall-health "Recent Patrol errors" coverage factor in `summarizeRecentPatrolCoverage` suppresses the score penalty once three consecutive trailing successful full Patrol runs exist at the most-recent end of the recent-runs window — so the grade reflects current reality after a Patrol-affecting bug is fixed rather than dragging stale failures forward for the ~9 hours it takes scheduled runs to age them out of the trailing-10 ratio, the orphan-tool-call-repair contract where `convertToProviderMessages` injects synthetic is_error tool result messages for any `tool_call_id` in an assistant message that has no matching downstream tool result, so a chat session that ended mid-tool-call (network drop, ctx timeout, browser crash) cannot poison its next message with the structural-violation error the provider rejects — the synthetic content is marked is_error=true and explains the interruption so the model can retry the call or proceed without the data, and the patrol-session-bound contract where `ExecutePatrolStream` calls `SessionStore.TrimMessages` after persisting each run's messages to cap the patrol-main session at 200 messages (roughly two recent runs' worth) — without the bound the file grew unbounded at every scheduled run, reaching 16 MB and 3,593 messages within a month and making every `AddMessage` rewrite linearly more expensive; the canonical Patrol forensic log is the `PatrolRunRecord` history surfaced at `/api/ai/patrol/runs`, not the chat-session-shaped file
2. Keep AI runtime and shared API proof routing aligned in `registry.json`
3. Preserve explicit coverage for chat, Patrol, remediation, and cost-control behavior when AI runtime changes. Interactive Assistant and Patrol tool selection must remain model-owned: Pulse may provide governed context, tools, approval state, resource-resolution facts, safety policy, and neutral resource-scoped action history, but it must not add prompt-keyword routers, expected-tool retries, auto-recovery tool calls, keyword-matched prior-fix suggestions, or Pulse-authored remediation/finding fallbacks that choose the next investigative or corrective action for the model.
   Assistant FSM gates remain safety boundaries after the model chooses a tool:
   repeated model attempts must not waive post-write verification or allow a
   new state-changing tool before the model has supplied current verification
   evidence through an allowed read/resolve path.
   Assistant restored-session and recent-session context is also model-bound
   context, not an identity-policy bypass: when the referenced unified resource
   is governed or redacted, backend context builders must use the resource
   policy label or redacted value, suppress exact location/routing metadata
   such as `target_host`, and avoid exposing raw provider IDs, aliases, IPs, or
   hostnames to the model unless policy allows them.
   Resource drawer Assistant handoffs use the `resource_context` metadata kind
   and must attach product-originated resources as model-only context, not as
   saved user text. The backend must preserve that handoff kind through session
   persistence/restore, prepend an explicit selected-resource, discovery,
   tool-target-handle, data, raw-context, and action boundary before the
   resource context pack, and sanitize streamed assistant content, saved
   assistant messages, and tool results through the same unified-resource
   policy redaction path used for the context pack. The only model-facing tool
   target for an attached redacted resource is `current_resource`; read,
   query-get, and discovery tools resolve that handle server-side against the
   session-selected resource and must not treat `redacted by policy` as a raw
   infrastructure identifier. The live `resource-context` eval is the required
   regression proof for this path: the model must not ask which resource the
   user means, must not call discovery just to identify the attached resource,
   must report attached discovery readiness from context without a discovery
   tool call, must use the safe handle for scoped reads, must refuse raw
   provider/config/environment/secret-bearing context expansion, and must not
   leak configured forbidden resource details in content or tool inputs.
   Context-only resource handoff turns must be enforced at the tool-manifest
   boundary, not only by prompt wording: unless the operator explicitly asks for
   live runtime verification, a read attempt, or discovery execution, the
   Assistant loop must receive no tools and must answer from the attached
   context or state that Pulse does not currently have that fact.
   Resource-context model packs and drawer handoff briefings must carry the
   canonical discovery readiness state (`fresh`, `stale`, `missing`, `running`,
   `failed`, `unavailable`, or `unsupported`) with provenance and freshness
   metadata so Assistant can explain whether it is grounded in current
   discovery data before choosing any tool.
   Patrol deterministic triage signals are prioritized evidence seeds for the
   configured model; they must not be described as a Pulse-authored final
   diagnosis, proof that unflagged resources are healthy, or a reason to
   prohibit the model from choosing governed tools for adjacent evidence
   relevant to its own findings.
   Patrol runtime failures are part of that runtime contract: provider, model,
   tool-calling, auth, quota, rate-limit, context-window, and connectivity
   failures must be classified in `internal/ai/` before they reach operators,
   surfaced as the synthetic Patrol runtime finding, and preserved on patrol
   run records as structured error summary/detail instead of collapsing to
   generic analysis-failed copy. Demo-mode Patrol run records must carry
   explicit source provenance and must not persist as live runtime evidence;
   outside demo mode, run-history reads, run lookup, and Patrol coverage
   scoring must filter both source-marked and legacy `demo-run-*` records so
   live assessment state cannot be contaminated by public demo fixtures.
   Unavailable-provider blocked states must direct operators to Assistant &
   Patrol provider settings and tool-capable Patrol model selection, not
   legacy `Settings > Pulse Assistant` copy.
   Patrol status must also carry server-authored readiness for provider,
   model, settings-persistence, and tool-calling prerequisites so the UI can
   block known-bad manual Patrol runs before they become generic runtime
   failures. The same `internal/ai` readiness evaluation must gate Patrol
   runtime admission directly: settings saves that are needed to recover a bad
   provider/model selection must persist and return structured readiness cause
   metadata, while manual run requests, scheduled ticks, and scoped
   alert/anomaly runs must fail or skip before LLM execution when the selected
   Patrol model/provider is known not-ready.
   Monitor-only Patrol autonomy saves are part of the same runtime gate:
   when the safe-remediation extension or entitlement is unavailable, both the
   browser state owner and `internal/api/ai_handlers.go` must clear stale
   full-mode unlock state while clamping autonomy back to `monitor`, so paid
   remediation permission cannot survive through a free runtime save.
   Patrol update-safety observation is part of the same read-state runtime
   boundary: Docker host/container snapshots used to detect image-digest
   divergence must be sourced from canonical `ReadState.DockerHosts()` views,
   with model-shaped data limited to the watcher adapter input rather than
   direct `StateSnapshot.DockerHosts` reads in the Patrol run loop.
4. Keep discovery scheduling authoritative through `internal/config/ai.go`: `discovery_enabled` and `discovery_interval_hours` must govern both lightweight infrastructure discovery and deep service-discovery background loops. `internal/api/ai_handlers.go` must preserve an explicitly supplied `discovery_interval_hours: 0` as the manual-only setting and may only apply the 24-hour default when discovery is enabled without an explicit interval payload. Discovery analysis remains a Pulse tool-led model workflow: Pulse supplies agent/API/metrics evidence and cache orchestration, while the selected model provides the intelligence. Background, settings-triggered, or drawer-triggered discovery progress must describe that discovery analysis directly and must not imply a live Pulse Assistant chat transcript unless the run is actually executing inside the chat surface. Settings-triggered manual discovery is an explicit operator refresh and must not open, append to, or masquerade as a Pulse Assistant session. Assistant and Patrol access to discovery must stay behind the governed `pulse_discovery` tool, including an explicit forced refresh action for known resources, while settings-level manual runs must use the canonical `/api/discovery/run` new/changed/stale/repairable sweep rather than a frontend-only shortcut. Fresh-but-unidentified service-discovery records are not complete when canonical resource metadata, stored facts, or safe command evidence can deterministically identify a known workload and endpoint; cached reads and the canonical sweep must repair those records instead of presenting `Unknown Service` as fresh.
5. Preserve auditability for outbound model-bound context exports and keep the export record aligned with the prompt boundary that actually reaches the provider
   External provider-bound unified-resource context must enforce the same
   data-handling policy the export audit records: `local-only` resources are
   represented only as aggregate posture and omitted from detailed prompt
   sections, while sensitive alert text is scrubbed through the shared
   unified-resource redaction helper before it reaches a non-local model.
   The final provider-bound chat, Patrol, investigation, tool-result, and any
   retained legacy managed-model compatibility requests must also pass through
   that same resource-policy sanitizer immediately before transport, so later
   agentic turns cannot reintroduce local-only identifiers after the original
   context export.
6. Keep AI resource and incident context aligned with the canonical unified-resource timeline before falling back to patrol-local change detectors
7. Keep platform assistant read/control claims aligned with
   `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`. New
   platform-native reads or writes must extend the shared Assistant tool
   contracts, and read-only or augmentation-only platforms must stay explicit
   there instead of drifting into provider-local tools.
8. Keep Pulse Assistant action governance canonical in the shared tool
   registry. Tool prompts and approval surfaces must derive read, mixed, write,
   and approval-policy claims from `internal/ai/tools/registry.go` and
   `internal/ai/tools/executor.go` instead of maintaining hand-written
   prompt-only tool lists. Fallback Assistant governance text and Patrol
   system-prompt tool summaries must mirror the same registry-owned capability
   shape, including `pulse_discovery` read-or-refresh behavior, instead of
   presenting stale read-only summaries. Frontend approval cards must surface backend
   approval risk/description without hiding a pending approval when skip or
   deny fails. Action-producing tools must also persist the unified
   `ActionPlan.Preflight` dry-run boundary through
   `internal/ai/tools/action_audit.go` rather than leaving dry-run availability
   as chat-only text.
   When the shared registry blocks a control tool in read-only mode, its
   operator guidance must point to Assistant & Patrol settings and the Pulse
   Assistant Permissions Control mode, not legacy Pulse Assistant settings
   paths.
9. Keep self-hosted Patrol messaging aligned with the v6 GA product contract:
   ordinary self-hosted installs use BYOK or local providers, and the runtime
   must not surface retired managed-model credits, trial prompts, account-backed
   AI activation, or general hosted chat entitlement in the normal app.
   The shared app shell must also keep `/cloud` and `/cloud/signup` out of
   ordinary self-hosted public routes so Cloud acquisition cannot reappear as a
   proxy for retired hosted-model or AI quickstart activation.
   The public AI overview must likewise use productized context language such
   as alert history, Patrol runs, and resource timelines instead of presenting
   `incident memory` as a standalone feature. It must also describe Patrol
   baselines, trends, correlations, forecasts, and deterministic signals as
   model-bound evidence/context rather than Pulse-authored intelligence or
   fallback finding creation.
10. Keep discovery-analysis prompt bounds and response budgets aligned across
   `internal/ai/service.go` and the shared service-discovery prompt builders:
   the runtime must reserve enough output tokens for structured discovery JSON,
   and discovery prompts must cap fact/path/port fan-out explicitly instead of
   relying on providers to truncate oversized infrastructure inventories.
   That same runtime-owned command-target boundary must resolve hostnames
   through `internal/unifiedresources/hostname_equivalence.go`.
   `internal/ai/tools/internal_routing.go`,
   `internal/ai/tools/tools_control.go`, and adjacent AI command helpers may
   match a short host against its FQDN, but they must not broaden that
   fallback into a generic short-name collapse that would make two distinct
   FQDNs with the same short host look interchangeable.
11. Keep AI runtime transport compatibility separate from operator-facing
    product copy. Existing Patrol payload fields such as `fixed_count`,
    `auto_fix_model`, and `patrol_auto_fix` may remain stable wire/API names,
    but frontend comments, API denial messages, runtime logs, status labels,
    CLI help, and commercial prompts that describe the capability must use safe
    remediation or remediation wording.
12. Keep AI control-level presentation runtime-owned rather than tier-owned.
    `frontend-modern/src/utils/aiControlLevelPresentation.ts` and
    `frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx`
    may describe approval posture, but must not add Pro-badge suffixes or
    local commercial tracking around those runtime controls.
13. Keep Assistant control and Patrol paid runtime settings entitlement-effective
    at every runtime boundary. Stored config may preserve autonomous, Patrol
    auto-remediation, and alert-triggered analysis preferences so they come
    back if entitlement returns, but API responses, chat executor startup,
    restart, settings-update, request-clone paths, and Patrol execution must
    clamp those values through runtime entitlements before exposing or enforcing
    them.
14. Keep agent-backed Patrol reachability checks aligned with the agent command
    policy. `internal/ai/patrol_prober.go` may use connected agents for
    read-only guest ping probes, but it must validate each target as an IP
    address and issue only the single-target `ping -c 1 -W 1 <ip>` command
    shape covered by the agent-exec auto-approval policy. It must not compose
    shell loops, accept hostnames, interpolate unvalidated targets, or bypass
    approval requirements for compound commands.
15. Keep OpenAI-compatible streaming finalization fail-closed. `ChatStream` may
    flush buffered final SSE lines when EOF arrives with unread bytes and may
    accept providers that omit `[DONE]` only after a terminal `finish_reason`,
    but it must not emit `done` or executable tool calls from partial tool-call
    builders when the stream closes before that terminal provider state.
16. Keep Patrol investigations product-facing through the shared
    `aicontracts.InvestigationRecord` contract. Patrol may keep
    `InvestigationSession` as execution detail, but Assistant handoff,
    unified findings, persistence, and approval/remediation context must use
    the durable investigation record when they need operator-facing
    investigation context. The durable record carries top-level `impact`
    and `rollback` strings alongside the existing `verification` array, so
    Assistant `/api/ai/chat` enrichment surfaces consequence-if-ignored and
    undo intent when Patrol has populated them and remains silent when the
    fields are empty rather than fabricating placeholder analysis through
    the model. The TS API client must keep its `InvestigationRecord` and
    `InvestigationRecordTrigger` mirrors aligned with the Go struct,
    including the `trigger.cause` string, so frontend handoff context does
    not lose backend-attributed failure cause. Detection-time finding
    creation may author `Finding.Impact` directly so the
    consequence-if-ignored statement is set at finding birth rather than
    waiting for an AI investigation to derive it;
    `BuildFindingInvestigationRecord` then propagates `Finding.Impact`
    into `InvestigationRecord.Impact` without transformation, and the AI
    engine must not synthesize impact text from severity or category when
    the finding source has not authored one. The Patrol runtime-failure
    classifier (`internal/ai/patrol_runtime_failure.go`) is the canonical
    example: it stamps a constant impact string on every runtime-failure
    cause because the operational consequence of a non-running Patrol is
    invariant across causes, and only the recommendation varies. That
    detection-time impact text propagates through `FindingsStore.Add` (which
    must overwrite `existing.Impact` alongside `Description` and
    `Recommendation` so re-detected findings adopt freshly-classified
    impact rather than preserving stale empty values left by older
    binaries) and through the Finding to UnifiedFinding conversion in
    `internal/api/router.go` so the operator-facing
    `unified.UnifiedFinding` surface receives the same impact string the
    durable investigation record carries. Threshold-alert findings author
    impact through a parallel hand-curated catalog in
    `internal/ai/unified/alerts.go` (`generateImpact`), keyed on alert
    type rather than severity or category, so threshold findings carry
    consequence-if-ignored copy at detection time without depending on an
    AI investigation. The unified-store update paths
    (`UnifiedStore.AddFromAlert` and `UnifiedStore.AddFromAI`) must
    propagate Impact on re-detected findings the same way they propagate
    Description: AddFromAlert backfills empty Impact on existing findings;
    AddFromAI overwrites existing Impact when the incoming finding carries
    one. Threshold-alert conversion must not synthesize remediation
    recommendations; unknown alert types must return an empty impact rather
    than synthesizing generic copy.
    Investigation-record `Rollback` is sourced from the canonical
    `RemediationPlan` when one exists for the finding:
    `AggregatePlanRollbackSteps` in
    `internal/ai/investigation_records.go` flattens
    `RemediationStep.Rollback` strings into a deduplicated record-level
    slice, and the patrol findings build site
    (`internal/ai/patrol_findings.go`) populates `record.Rollback` from
    `remediationEngine.GetPlanForFinding` when the engine and an active
    plan are present. Rollback must remain absent rather than fabricated
    when no plan exists, mirroring the impact rule.
    LLM-generated AI patrol findings author impact through the
    `patrol_report_finding` tool schema in
    `internal/ai/tools/tools_patrol.go`: the tool exposes an optional
    `impact` parameter, `PatrolFindingInput.Impact` carries it through,
    and `patrolFindingCreatorAdapter.CreateFinding` writes it onto
    `Finding.Impact` so the LLM's authored consequence-if-ignored copy
    flows through the same propagation path used by curated catalogs.
    The patrol system prompt instructs the LLM to author concrete
    operational consequences (named workloads, jobs, recovery windows)
    and to leave `impact` empty rather than fabricate one when the
    consequence is genuinely unknown; the runtime must not synthesize a
    default in that case.
    The action broker enforces a plan-hash drift check at the execute
    boundary: when an approval ID resolves to a stored plan with a
    PlanHash, the freshly-recomputed approval-equivalent hash from the
    actual payload (using `approvalPlanHash`, the same function used at
    approval-creation time) must match. Mismatch returns
    `ErrActionPlanDrift` and refuses dispatch; the contract is "the
    operator approved exactly this (command, target, reason)
    combination" and a different one cannot run under the stale
    approval. When `approvedHash` is empty (older approval records, or
    contract paths that did not author one), validation is skipped to
    preserve existing behavior. The check is currently wired in
    `executeCommandWithAudit` for shell-command actions; the native-
    action path uses a different hash function (`actionPlanHashForParams`)
    so a coherent canonical-hash refactor must precede adding the same
    check there.
    The broker runs a class-derived read-after-write verification check
    immediately after a successful dispatch. `VerificationCommandForCommand`
    in `internal/ai/tools/tools_control.go` returns the executable check
    keyed on the same command class as the preflight authoring (e.g.
    `systemctl is-active <unit>` after a service-restart). The check
    runs through the same agent path as the dispatch and the outcome is
    persisted on `ExecutionResult.Verification` so the audit history
    shows not only what the action did but whether the read-back
    confirmed the intended state. Container-class verification is
    deferred to pulse_docker's existing tool-level `docker inspect`
    check; classes without a derivable verification command leave
    `Verification` nil rather than fabricating a verified=true entry.
    The approval preflight presented to operators authors per-command-class
    safety and verification context on top of the default broker-level
    posture. `classifyApprovalCommand` and
    `approvalCommandClassPreflightAdditions` in
    `internal/ai/tools/tools_control.go` bucket common Pulse remediation
    actions (service-restart, service-stop, service-start, service-reload,
    container-restart, container-stop, k8s-rollout-restart, plus the
    Proxmox VM lifecycle classes proxmox-vm-reboot, proxmox-vm-stop,
    proxmox-vm-start, proxmox-vm-shutdown and the matching pct-driven
    proxmox-ct-* container lifecycle classes) and return hand-authored
    operational copy: what the command actually touches, how Pulse will
    read back success. The additions append onto the default
    safety/verification arrays rather than replacing them, so the
    broker's structural posture (org scope, hash match, single-use
    approval) remains visible alongside the class-specific copy.
    Unknown command classes must return empty additions rather than
    fabricated padding — operators see only the default content, not
    invented assertions about what an unrecognized command will do.
    The Proxmox classes intentionally do not derive a broker-level
    `VerificationCommandForCommand` check because pulse_control's
    `verifyGuestAction` already runs `qm status` / `pct status` at the
    tool layer; adding a parallel broker dispatch would double-run the
    same read-after-write check.
    Drift refusal must also persist a Failed audit record with the
    Request, Plan, and Approvals snapshots intact and a Result whose
    ErrorMessage is prefixed `plan_drift:` so the audit trail shows
    every drift attempt that was caught. Operators reviewing the action
    audit history must be able to see drift refusals as first-class
    audit rows, not only in WARN-level logs; the `plan_drift:` prefix
    is a stable token for audit-UI filters and alert rules to
    distinguish drift from generic execution failures.
    `FindingsStore.GetTrustSummary` returns a snapshot of how currently
    tracked findings have resolved (tracked, currently-active, resolved,
    auto-resolved, fix-verified, fix-failed, dismissed-as-noise,
    dismissed-as-expected, dismissed-as-later, suppressed,
    regressed-at-least-once). It is the data layer for trust metrics on
    operator surfaces. `PatrolService.GetFindingsTrustSummary` exposes
    the same snapshot through the service boundary, and the
    patrol-status API response carries it under
    `PatrolStatusResponse.Trust` so the Patrol page can render a Trust
    strip without callers reaching past the service boundary. The summary is intentionally a snapshot of the
    in-memory store, not lifetime totals; once findings are cleaned up
    they no longer contribute. Downstream surfaces must frame the
    counts as current-state distribution rather than historical
    aggregates, and the AutoResolved bucket includes both the
    `Resolve(auto=true)` path and the
    `UpdateInvestigationOutcome(fix_verified)` path.
    Findings carry a `previous_resolved_fix_summary` field as
    operational memory across regressions: when a finding that had a
    resolved investigation with a proposed fix is re-detected,
    `FindingsStore.Add` captures
    `existing.InvestigationRecord.ProposedFix.Description` into the new
    field BEFORE clearing the investigation record, and the chat-context
    builder surfaces it as a "Previous Resolved Fix" line so Assistant
    sees what worked last time rather than treating each regression as a
    blank-slate diagnosis. The summary mirrors onto
    `unified.UnifiedFinding` and propagates through the Finding to
    UnifiedFinding conversion in `internal/api/router.go` and through
    `UnifiedStore.AddFromAI`'s update branch (non-empty overwrite). The
    TS API client mirrors the field on `UnifiedFindingRecord` and
    `Finding`, the aiIntelligence store normalizer copies it through as
    `previousResolvedFixSummary`, and `FindingsPanel.tsx` renders it on
    the expanded finding card so the operator sees the memory cue
    inline rather than only inside Assistant chat context. When `/api/ai/chat` receives `finding_id`, the
    runtime must enrich the provider turn from that durable record while
    preserving the user's authored prompt as the persisted conversation
    message; the model-only handoff may persist as session metadata so
    same-session follow-up turns keep the Patrol finding context without
    mutating saved user messages. Patrol run-history handoffs follow the same
    backend-owned context rule: the browser may seed only safe `patrol_run`
    metadata such as run ID/type/status/runtime-failure posture, while
    `/api/ai/chat` must rehydrate model-only run context, scoped resources, and
    safe failure detail from the current Patrol run record before model
    execution and again on same-session follow-up turns. If the Patrol run no
    longer resolves, browser-authored run context, resources, and actions must
    be dropped rather than used as fallback provider context. When the handoff
    identifies a resource, the
    runtime may also seed the session's resolved-resource scope, but only through
    canonical unified-resource tool registration so allowed actions, executors,
    and explicit-access checks stay governed. Structured handoff resource
    references may persist as session model-context metadata for follow-up turns,
    but they remain references only; each turn must rehydrate them from the
    current canonical unified-resource model before action validation can use
    them. Structured finding references from Patrol/Assistant handoffs may also
    persist as session model-context metadata so follow-up turns can refresh the
    current unified finding and investigation record before model execution;
    those references remain model-only context selectors, not saved user text or
    lifecycle authority. When the current finding identifies root-cause or
    correlated finding IDs, Assistant may resolve those related findings through
    the current unified finding store and include compact related-finding
    summaries plus their structured handoff resources as model-only explanation
    context. Those summaries must carry current recency facts and latest
    lifecycle state from the related record rather than only title/resource
    prose. Those related findings must be deduplicated, bounded, refreshed from
    current store state, and treated only as context for the same operator
    conversation; they do not grant approval, lifecycle, disclosure, or
    execution authority. If the referenced finding no longer resolves through
    the current unified finding store, Assistant must invalidate the stored
    model-only handoff and unpinned handoff-seeded resource scope instead of
    falling back to stale investigation context. The refreshed finding context
    must include unified finding lifecycle and recency facts such as active,
    resolved, snoozed, dismissed or suppressed state, detection/last-seen/
    resolved timestamps, recurrence, regression, and recent lifecycle events so
    Assistant explains the current Patrol record rather than only the original
    investigation narrative. The saved-session handoff envelope must also
    preserve first-class Patrol source identity when product callers provide
    safe metadata. Patrol assessment handoffs remain `patrol_assessment`
    whole-surface review sessions even when their bounded action references
    name individual findings; the session list must not infer a
    `patrol_finding` identity from those action references once metadata is
    present. Patrol configuration failure handoffs remain
    `patrol_configuration_failure` sessions and may expose only the safe
    runtime-failure boolean needed for browser presentation. Run-specific
    fields stay reserved for `patrol_run` handoffs, while hidden model context,
    command payloads, preflight output, and action results remain
    backend-owned. The finding briefing may surface the primary
    finding's current recency facts, bounded evidence snapshot,
    verification summary, latest lifecycle event, and governed action
    artifact metadata, but it must not generate Pulse-authored attention
    reasons, operator-decision framing, or remediation next-step guidance
    for the model. Detailed lifecycle events must stay in a bounded
    `[Finding Lifecycle Context]` block with an explicit model-only boundary.
    Assistant runtime may also hydrate canonical
    resource-policy context for those handoff resources, using the same
    unified-resource resolution and policy presentation helpers that govern
    mention prefetch and provider-bound redaction; that context remains
    model-only handling guidance, not saved user text or disclosure authority.
    Before injecting any product-originated handoff context into the model
    prompt, the runtime must also apply canonical resource-policy redaction to
    the assembled handoff text itself, including finding briefings and
    lower-level finding/action context, so local-model prompts and non-local
    provider transport share the same governed identity boundary.
    Assistant runtime may also hydrate current canonical resource-state context
    for those handoff resources, including compact status, freshness,
    source-health, metric, incident, and governed-capability summaries from the
    unified-resource model. That state snapshot remains model-only read-only
    infrastructure context, must honor the same policy/redaction boundary before
    provider transport, and must not grant approval or execution authority.
    Assistant runtime may also hydrate canonical
    relationship context for those handoff resources through
    `FormatResourceRelationshipContext(...)` and canonical parent-edge synthesis,
    but those topology facts remain read-only explanation context and do not
    grant action authority. Assistant runtime may also hydrate recent changes for
    those handoff resources from the canonical unified-resource timeline as
    model-only context on each turn; it must resolve product-originated handoff
    references through the canonical unified-resource provider before querying
    timeline changes, with raw handoff IDs used only as a compatibility fallback.
    Those timeline facts remain read-only explanation context and do not grant
    action authority. Assistant runtime must also treat the shared
    `internal/agentcontext` resource context pack as the canonical rich
    resource-grounding substrate for product-originated resource handoffs:
    runtime/discovery, topology, safety, policy, and recent-change facts are
    hydrated as model-only context from canonical unified resources and the
    resource store, carry provenance/freshness/redaction metadata, and must not
    include raw command output, provider config, environment values, mount paths,
    label maps, or secret-bearing metadata. The runtime may also persist structured pending-action and
    approval references from the same investigation record as
    model-context metadata, and the API handoff builder may recover the current
    live Patrol investigation-fix approval by finding ID when the durable record
    does not yet carry the latest approval ID. Those references are review
    context only: they must not include raw command text, must not grant
    approval or execution authority, and must route any requested action back
    through the governed approval/remediation flow. Patrol provides the
    configured LLM with observed finding context, evidence, policy posture, and
    governed action state; the LLM owns diagnosis and remediation reasoning.
    Operator-visible handoffs must not describe Patrol as having already
    authored the correct remediation. The finding briefing may carry only
    factual governed action artifact metadata from those same structured
    references after live-approval recovery, including safe current status,
    request/expiry timestamps, approval policy, action plan identity, plan
    expiry, and dry-run posture, so Assistant sees current approval/action state
    without Pulse choosing the model's next investigative or corrective step.
    When those references include an approval ID, Assistant runtime may refresh
    a current status snapshot from the canonical approval store on each turn,
    but it must enforce org scoping and still omit the approval command payload.
    When those references resolve to a governed action plan or action audit,
    Assistant runtime must hydrate the canonical action ID, lifecycle state,
    requester, capability, approval policy, plan expiry, preflight/dry-run
    summary, and terminal success/failure state from the action-audit store
    rather than treating the original approval as the current action truth.
    That action snapshot remains model-only review context and must expose
    lifecycle status rather than raw execution output or command text.
    The public chat session contract may expose only a bounded
    `handoff_summary` for this private model-context metadata so reloaded
    Assistant sessions can still be identified as scoped Patrol/product
    handoffs. That summary may include the handoff kind, finding ID, resource
    and Patrol run ID, safe run type/status/runtime-failure flags, resource and
    action counts, a primary resource label, last-known approval/action status,
    risk level, and timestamp, but it must not expose Patrol-authored
    recommended next-step titles, recommendation details, route-owned action
    labels, model-only handoff text, runtime failure detail, action
    preflight/result bodies, remediation descriptions, raw commands, or
    approval command payloads. Its
    `requires_approval` field is a current operator-decision flag only: pending
    approval states may set it, but approved, denied, rejected, executing,
    completed, failed, expired, or otherwise historical action references must
    remain action context without being relabeled as requiring approval.
    When the Assistant drawer restores any session from that `handoff_summary`,
    it must restore the scoped request-local approval boundary as well as the
    safe visible briefing: the next chat turn must carry
    `autonomous_mode:false` even when the summary is context-only and has no
    queued action, while the visible badge/action copy must still reflect the
    actual last-known action state or Patrol assessment context instead of
    inventing a pending approval or restoring a Patrol recommendation. That
    restoration is success-bound: if the underlying session message load fails,
    the drawer must leave the current context untouched instead of applying
    summary-derived Patrol or approval state for a session the operator is not
    actually viewing.
    Before `/api/ai/sessions` returns summaries with stored handoff action
    references, the chat runtime must refresh their safe approval/action status
    from the canonical approval store and action-audit store. Session listing is
    an operator decision surface, so it must not leave stale pending/approval
    labels in the drawer after the governed action moved on, and it must still
    omit raw commands, preflight bodies, and execution output.
    The Assistant drawer must also fetch that current session list before
    opening the session picker instead of presenting mount-time cached
    summaries as the operator's decision surface. For restored Patrol
    assessment or finding sessions, that picker must present only the safe
    handoff kind, source, resource/action counts, and approval/action status
    from `handoff_summary`; it must not restore Patrol recommended
    next-step title/detail/action labels, route-owned hrefs, or disabled-action
    reasons as visible or hidden context.
    Browser-originated `handoff_context`, `handoff_resources`, and
    `handoff_actions` plus safe `handoff_metadata` are one-shot request seeds
    for the first successful chat turn. Patrol next-step titles, details,
    labels, and route-owned hrefs do not belong in `handoff_metadata`, and
    model-context text parsing must not resurrect them as a legacy fallback.
    After that send succeeds, the drawer
    must clear those request payloads while preserving the safe visible
    briefing and request-local
    approval-required posture; later turns must rely on backend-owned session
    model-context hydration and current canonical stores instead of resending
    stale browser handoff payloads. Patrol approval-row Assistant entries are
    still Patrol finding handoffs, not local prompt-only shortcuts: live
    approval rows, expired action-artifact rows, and missing-detail queued-fix
    recovery rows must route through the shared Patrol finding handoff builder
    so the backend receives the same bounded model-only finding context,
    resource reference, and safe action reference posture that the main finding
    handoff uses.
    Proposed-fix command text must stay out of both the persisted chat message
    and the model-only handoff context, and command payloads remain
    approval-context data, not conversational copy.
    `/api/ai/chat` must also clamp Patrol finding handoffs to
    approval-required mode when a request carries a non-empty `finding_id` or
    resolves to model-only briefing, resource, or action context, by forcing the
    request-local autonomous-mode override to false, even when a caller supplied
    `autonomous_mode:true`. That clamp belongs to the
    backend/API execution boundary, does not mutate the user's persistent AI
    control setting, and prevents product-originated Patrol action context from
    becoming silent command authority.
    The chat runtime must apply any request-local autonomous-mode override to
    both the per-request `AgenticLoop` and the cloned `PulseToolExecutor`;
    persistent autonomous settings must not leak into scoped approval-required
    handoffs through executor state. When such an override forces approval mode
    and the saved control level is autonomous, the executor clone must clamp its
    effective control level to controlled for that request only, so even
    policy-allowed diagnostic commands require operator approval in scoped
    handoffs without mutating the user's saved setting.
    The Assistant drawer may also render an attached context briefing for that
    handoff, but the briefing is runtime context visibility only: it must not
    mutate chat control settings, execute tools, or reveal raw command payloads.
    Resource-drawer Assistant entries use that same briefing path with
    `handoff_metadata.kind=resource_context`, a structured `handoff_resources`
    reference, and `autonomous_mode:false`; they must not prefill or submit a
    browser-authored prompt, and any rich resource facts must be hydrated by the
    backend context-pack path rather than reconstructed in the browser.
    Safe route-owned briefing actions may render as app links when the handoff
    includes an `actionHref`, but those links are navigation guidance only and
    do not grant tool execution or approval authority.
    Request-local approval-required scoped handoffs must present that boundary
    through compact source-named drawer state and the effective control label,
    so Patrol approval/finding handoffs and alert-investigation handoffs are
    named by their source rather than as generic dashboard briefs.

## Current State

Primary nav moved to governed platform/runtime destinations on 2026-05-16 and
was clarified on 2026-05-25 through `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx`: the top of the app may expose canonical
platform pages (Proxmox, Kubernetes, TrueNAS, vSphere) plus the
Docker / Podman container-runtime lens (shown as Docker in the shell), aggregate
platform-owned Workloads / Storage / Recovery sub-surfaces, Alerts, Patrol, and Settings.
Provider/runtime destinations must pass the shared support-and-resource-
evidence gate before they appear in navigation, command palette entries,
keyboard shortcuts, or landing fallbacks. Aggregate workspace tabs are retired
top-level routes, not compatibility placeholders. Admitted-only or absent
platform/runtime surfaces must stay hidden from the AI-adjacent shell and must
not be kept as disabled placeholders. The legacy `/infrastructure` route shell
was retired alongside its page wrapper, and `/workloads`, `/storage`,
`/recovery`, `/ceph`, `/ai`, and `/operations/*` remain unregistered
top-level routes. Primary platform tab `settingsRoute` handoffs must also point
to the canonical `/settings/infrastructure` workspace rather than retired
settings aliases such as `/settings/workloads/docker` or nested
`/settings/infrastructure/platforms/*` paths. The AI Chat launcher, Patrol surfaces, and
`AssistantHandoffPayload` deep links must use canonical platform or runtime
routes (`/proxmox/overview`, `/proxmox/storage`, `/kubernetes/workloads`, etc.)
rather than reviving retired Infrastructure or aggregate workspace paths;
adding a platform tab through the same shell files must not fork Assistant or
Patrol shell state or smuggle in AI-owned platform reads.
Post-auth `/` and `/login` resolution follows the frontend-primitives-owned
provider-first platform landing contract, so the assistant-capable shell never
overrides Machines-surface eligibility or revives legacy Infrastructure as the
default estate surface. The user-facing Machines label is an app-shell
presentation label for the existing `standalone` route/id and must not create a
separate AI handoff or prompt namespace.

Alert-triggered scoped patrols now investigate the specific breach rather than
running a broad health check. The alert bridge (`internal/ai/unified/bridge.go`,
`internal/ai/unified/setup.go`) carries the firing alert's real payload — type,
level, value, threshold, resource identifier, and message — into
`PatrolScope.AlertContext`, and `internal/ai/patrol_ai.go` /
`internal/ai/patrol_triggers.go` frame the `alert_fired` run around that breach
instead of suppressing threshold context. Whether an alert triggers a patrol at
all is the operator's per-rule policy: `AIConfig.AlertTriggersInvestigation`
(`internal/config/ai.go`) enforces the master enable, a minimum-severity floor
(`patrol_alert_trigger_min_severity`, default critical-only), and an optional
alert-type allowlist (`patrol_alert_trigger_types`, empty = all types). The
router-side bridge wiring consults that policy and skips queuing the scoped
patrol when the alert does not qualify; an unknown alert level is treated as
critical so it is never silently dropped.

The route-backed Proxmox platform tab is app-shell navigation only. Adding the
tab through `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx` must not fork Assistant or Patrol shell
state, synthesize platform-specific handoffs, or add AI-owned platform reads.
Future Proxmox-native Assistant or Patrol read/write claims must extend the
shared AI tool, handoff, and platform-support contracts instead of hiding
behind route registration or tab chrome.

Patrol deterministic signal extraction (`internal/ai/patrol_signals.go`)
does not mirror the Alerts surface. The `pulse_alerts` tool output is
intentionally absent from the signal switch in `DetectSignals` — alerts
already have their own canonical surface, lifecycle, and operator
acknowledgement model, and the `SignalActiveAlert` mirror path has
been removed. Mirroring previously double-counted (every alert was
also a Patrol "Active alert detected" finding), dragged the health
score down for issues the operator already knew about, and produced
bogus `auto_resolved` → re-detected → regressed cycles when the LLM
explicitly resolved the mirrored finding while the underlying alert
kept firing. Patrol's job, per its own system prompt, is to surface
issues alerts cannot — trends, capacity risks, misconfigurations,
reliability gaps, cross-resource correlations. The Alerts page is
the canonical surface for currently-firing alerts. To retire the
alert-mirror findings already persisted from an earlier build,
`FindingsStore.SetPersistence` runs a one-shot pass on load that
auto-resolves any active finding matching the legacy signature
(title `"Active alert detected"`, source `ai-analysis`, category
`general`) with a clear retirement reason; the pass is idempotent
and self-cleaning. The same load pass also resets the
`RegressionCount` and clears `LastRegressionAt` on any active
finding whose lifecycle contains an `auto_resolved` event **when
the finding's category is not eligible for stale-auto-resolve**
(i.e. anything other than `performance` or `capacity`). For
event/persistent categories there is no legitimate absence-driven
resolution path: an `auto_resolved` event there came either from
one of the now-gated absence paths (legacy reason strings still
recognized) or from the LLM `patrol_resolve_finding` tool
(empty-message via `Resolve(_, true)`) which has repeatedly
misjudged findings backed by still-active conditions and reverted
through a regression on the next run. The reset appends a
`regression_counter_reset` lifecycle event so the migration is
idempotent; genuine recurrences from then on accrue cleanly.
Performance/capacity findings retain their counter because the
metric-cleared resolution model is sound there.

The overall health score (`calculateOverallHealth` in
`internal/ai/intelligence.go`) tiers the "recent Patrol errors" coverage
factor by the ratio of errored runs to relevant runs in the scoring
window. Above 50% of recent runs erroring is a `-30` impact and is
described as "Most recent Patrol runs encountered errors"; above 25%
is `-20`; otherwise the original `-10` light-tier description applies.
This prevents the score chip from showing grade A while the same
assessment surface warns the operator that coverage is incomplete or
recent runs failed, which previously happened whenever one successful
manual run sat among many failed startup runs.
Downstream Assistant handoffs must treat that coverage factor as a
secondary caveat when Patrol also carries active findings, pending
approvals, or governed action references. The coverage-gap explanation
and scoped-activity prompt are primary Assistant framing only for
coverage-only assessments; active Patrol findings keep the prompt,
briefing action label, and safety note focused on finding priority,
affected resources, evidence, and the governed next step.

Absence-based auto-resolve paths in `internal/ai/patrol_ai.go` are
all gated on the category whitelist exposed by
`CategorySupportsStaleAutoResolve` in `internal/ai/findings.go`. Two
paths use the gate: `reconcileStaleFindings` (auto-resolves findings
the LLM didn't re-mention in a successful run) and the
resource-absent branch inside the seed-prompt builder (auto-resolves
findings whose resource isn't in the current global inventory
snapshot). Only `performance` and `capacity` findings — continuous
current-state metric thresholds where the most recent successful
scan's observation is authoritative — may be auto-resolved from
absence. `reliability`, `backup`, `security`, and `general` findings
represent discrete events or persistent states; the LLM not
re-mentioning a failed backup task or a single inventory snapshot
missing a container (transient agent reconnect, container churn,
refresh gap) is not evidence that the underlying issue has cleared.
Those categories must stay active until explicitly resolved either
by the LLM calling `patrol_resolve_finding` with evidence or by
operator action through the governed findings store. Lifecycle events recorded by `findings.go` must
not introduce duplicate generic transition rows: the canonical
`syncLoopStateLocked` records `loop_transition_violation` only when a
transition is rejected, and otherwise leaves the semantic event
(`auto_resolved`, `regressed`, `dismissed`, `acknowledged`, `snoozed`,
`reminded`, `suppression_lifted`, etc.) as the single audit row for the
transition. Re-detections of an already-active finding update
`TimesRaised` and `LastSeenAt` only — they must not append a `detected`
lifecycle event, because a heartbeat is not a transition.

The findings store now consults a `ResourceOperatorStateProvider`
during the new-finding path. The interface lives in `internal/ai` to
avoid an import cycle with `internal/unifiedresources`; the API layer
wires the adapter at startup, projecting
`unified.ResourceOperatorState` into the
`ResourceOperatorStateProjection` shape the findings runtime
consumes. The projection carries every operator-set signal in one
call (active maintenance window plus the `IntentionallyOffline`
flag) so adding new signals later does not multiply round-trips per
finding.

The approval store exposes `SetOnApprovalCreated(cb)` so the API
layer can install a fire-and-forget callback that runs after every
successful `CreateApproval` (the approval is already persisted at
that point). The callback is invoked on its own goroutine against
a snapshot of the request, keeping the approval hot path off any
consumer's slowness and avoiding any chance of the consumer
reentering the store under the held write lock. This is the seam
the agent SSE stream uses to publish `approval.pending` events
without coupling the approval store to the api package.
`ApprovalRequest.CanonicalResourceID()` is the helper the bridge
uses to stamp `resourceId` on those events, derived from
`(TargetType, TargetID, TargetName)` via the same rule the store
uses internally for preflight context normalization — Plan-less
approvals (the common shape on the approval hot path) still carry
a canonical resource id agents can match against the rest of
Pulse.

`PulseToolExecutor` exposes `SetOnActionCompleted(cb)` as the
parallel seam for action-audit terminal states. The action-audit
hot path in `internal/ai/tools/action_audit.go` routes every
terminal-state record (Completed, runtime-Failed, plan-drift
refusal, operator-lock refusal, recovery-branch fail) through a
single helper `publishActionCompleted(record)` which guards on
nil callback, defensively filters non-terminal states, and fires
the callback on its own goroutine after the audit record has
been persisted. Refused-before-dispatch failures preserve the
canonical `plan_drift:` and `resource_remediation_locked:`
error-token prefixes on `record.Result.ErrorMessage` so the agent
SSE stream's `action.completed` payload carries them verbatim —
agents branch on the prefix rather than parsing human text.

The investigation runtime now hands the orchestrator a Finding
pre-enriched with operator-set state and operational memory.
`MaybeInvestigateFinding` (in `internal/ai/patrol_findings.go`)
calls `f.ToCoreFinding()` then attaches a
`FindingOperatorContext` from the in-memory operator-state
projection (intentionally offline, never auto-remediate, active
maintenance window) and a `FindingOperationalMemory` projection
(regression count, previous resolved fix summary, times raised)
populated from fields the internal `Finding` already carries. The
orchestrator (in pulse-pro) consumes these fields when reasoning
about the next move — it does not need a separate read to get
the situated picture, and it can avoid proposing fixes the
operator has locked the resource against.

`ResourceOperatorStateProjection` carries `NeverAutoRemediate`
alongside `IntentionallyOffline` and `MaintenanceWindow` so the
investigation read path and the suppression read path share a
single projection. The findings store exposes the projection via
`OperatorStateProjectionFor`; the suppression hot path keeps its
existing internal access. Both paths see the same operator-state
facts so investigation reasoning and suppression behavior cannot
drift against each other.

A cross-slice consequence worth pinning: operator-state-suppressed
findings (auto-dismissed with `DismissedReason="expected_behavior"`
and `operator_state_cause` metadata) are also ineligible for
autonomous investigation, because `Finding.ShouldInvestigate`
already gates on `f.DismissedReason != ""`. Investigation budget is
not spent on findings the operator has told Pulse to stay quiet
about, regardless of autonomy level. This is delivered by the
existing chain (Add → auto-dismiss → ShouldInvestigate-false)
rather than a separate runtime check; the contract test in
`findings_test.go` pins the relationship so a future refactor of
either branch cannot silently waste investigation budget on
operator-suppressed findings.

The operator-state suppression is also reversible. When a finding
auto-dismissed under `operator_state_cause` re-detects after the
underlying suppression has lifted (maintenance window passed,
`IntentionallyOffline` cleared), `FindingsStore.Add` wakes it with
a `suppression_lifted` lifecycle event. The wake gates on the most
recent lifecycle dismiss event carrying `operator_state_cause`
metadata via `findOperatorStateDismissCause`, so a manual operator
dismissal that supersedes an earlier auto-dismiss is not
falsely re-awakened — the helper stops at the first `dismissed`
event when scanning from newest backwards, treating that as the
authoritative state.

When the projection carries an active maintenance window, the
new-finding path auto-dismisses with reason `expected_behavior`,
attributes the suppression on the lifecycle timeline
(`operator_state_cause: maintenance_window`, with
`maintenance_end_at` metadata), and persists the finding for audit
history. The action broker (`executeCommandWithAudit` in
`internal/ai/tools/action_audit.go`) consults the same
`resource_operator_state` table on every dispatch and refuses with
`unifiedresources.ErrResourceRemediationLocked` when the operator
has set `NeverAutoRemediate=true` on the target resource. Refusal
persists a Failed audit record whose `ErrorMessage` is prefixed
`resource_remediation_locked:` so the audit timeline shows every
refused dispatch, paralleling the `plan_drift:` shape from the drift
guard. Operator state outranks per-action approval — the broker
refuses even when the approval ID resolves and the plan hash matches. The `IntentionallyOffline` branch is the indefinite
counterpart — same auto-dismiss but with
`operator_state_cause: intentionally_offline` and no
`maintenance_end_at` field because the suppression has no scheduled
end. Maintenance windows take priority over intentionally-offline
when both are active, because the time-bounded suppression is the
more honest one to surface to the operator. Deployments without a
provider keep the original new-finding behavior — suppression is
opt-in.

This subsystem now makes Pulse Assistant and Patrol backend runtime ownership
explicit inside the current architecture lane instead of leaving those
surfaces implicit inside broad architecture or generic API ownership. A later
lane split can still promote this area into its own product lane once the
governed floor is ready.
That backend/runtime ownership does not require the Patrol product surface to
inherit `AI` as its canonical browser route: the customer-facing shell may use
`/patrol` while shared AI transport, provider settings, and payload contracts
remain the governed technical boundary behind it.
Operator-configured provider base URLs remain part of that backend transport
boundary. Ollama keeps supporting remote or local instances, but
`internal/ai/providers/ollama.go` must normalize the configured base URL and
route requests through the shared restricted outbound HTTP transport so
metadata, link-local, and redirect-escape paths do not bypass the runtime's
egress guardrails.
That same operator-facing vocabulary rule applies to the runtime usage surface:
`frontend-modern/src/components/AI/AICostDashboard.tsx` must present provider
usage and spend backing Pulse Assistant and Patrol rather than generic `AI`
history, and `frontend-modern/src/utils/aiCostPresentation.ts` must own the
title, empty/loading states, budget note, and reset/export history messaging so
settings shells and runtime widgets do not fork their own usage wording.
That same runtime-facing table ownership applies to the cost dashboard shell:
`frontend-modern/src/components/AI/AICostDashboard.tsx` owns provider usage,
budget, and history semantics, but its tabular presentation must compose the
shared `frontend-modern/src/components/shared/Table.tsx` primitive instead of
carrying AI-local scroll wrappers or raw table shell markup. Any future AI
usage table styling change must extend the shared primitive or its governed
wrapper affordances first, then consume that contract from the dashboard.

`internal/ai/` is the live backend AI engine. It owns chat execution, Patrol
orchestration, findings generation, investigation support, provider selection,
remediation flow, and cost persistence.
That Patrol runtime ownership includes seed-context admission control.
`internal/ai/patrol_ai.go` must build Patrol and triage prompts from
canonical seed sections, size them against the runtime budget model, and when
a provider reports a smaller real context window than the static model map,
reassemble the same canonical sections under tighter provider-derived budgets
instead of hard-failing or truncating ad hoc prompt strings.
That same backend runtime ownership also includes bounded Patrol and
investigation read models. `internal/ai/patrol_history_persistence.go` and
`internal/ai/proxmox/events.go` must cap persisted-history loads and
caller-requested read limits at the canonical runtime maxima instead of
allocating directly from raw on-disk counts or transport-supplied limits.
Callers may request fewer records, but AI runtime storage and correlation
surfaces remain responsible for enforcing the governed ceilings that protect
memory and keep Patrol/history behavior stable under malformed or oversized
inputs.
That same backend runtime ownership includes `internal/config/ai.go`, because
provider auth, base URLs, provider-scoped model defaults, and other persisted
runtime AI selection rules must stay canonical in the shared AI config model
instead of drifting into handler-local fallbacks or frontend-only assumptions.
That same provider-model ownership now explicitly forbids Pulse from baking
vendor model IDs into BYOK default selection. `internal/config/ai.go` may
persist an explicit operator-chosen model, but when a BYOK provider is
configured without a concrete model selection,
`internal/ai/model_resolution.go` must resolve the effective model from the
provider's live catalog at runtime using the shared provider metadata policy
instead of reviving static vendor constants in config defaults, service
fallbacks, or frontend setup flows.
That same provider-model ownership also governs live-catalog failure fallback:
when runtime client construction fails, test credentials intentionally block a
provider catalog, or a provider returns no usable models, the effective BYOK
selection may fall back only to the provider-owned default declared in
`internal/config/ai.go`. Runtime startup, connection-test, and load-config
paths may not return an empty effective model or borrow another provider's
selection just because live model discovery was unavailable. DeepSeek's
provider-owned fallback must track the current V4 API contract and use
`deepseek-v4-flash` rather than retired compatibility aliases such as
`deepseek-chat` or `deepseek-reasoner`; AI runtime context-window and cost
budgeting must likewise know the V4 Flash/Pro 1M context and distinct pricing
classes before Patrol treats those models as ready.
The shared `/api/ai/models` catalog must preserve that same direct-provider
fallback posture for configured DeepSeek paths: when DeepSeek live catalog
listing fails or omits current V4 entries, the backend catalog must still
surface direct `deepseek-v4-flash` and `deepseek-v4-pro` options plus clearly
labelled legacy aliases so saved Patrol or Assistant selections do not render
as unrelated default models in the browser.
Retired quickstart ownership is now an inert compatibility boundary, not a
self-hosted GA runtime path. The old quickstart provider, bootstrap manager,
and local token-cache persistence API are removed from the Pulse runtime;
ordinary self-hosted Assistant, Patrol, and AI Settings flows must use the
operator's configured provider or local model and must not bootstrap managed
credits, hosted-model tokens, or quickstart-backed provider clients from the
frontend.
Public-facing copy that reflects old quickstart fields must normalize back to
provider or local-model setup. It must not promise managed credits, account
activation support, trial CTAs, anonymous Community bootstrap, or full hosted
chat access in ordinary self-hosted v6 GA flows.
That same runtime-backed contract now governs AI settings enablement too:
unconfigured installs open provider setup, while stale managed-credit or
activation-required states are treated as compatibility metadata rather than a
direct-enable path.
That same AI/runtime boundary now also owns the server-side assistant
availability fact used by the app shell. `internal/api/ai_handlers.go`,
`internal/api/security_status_capabilities.go`, and
`internal/api/router_routes_auth_security.go` must expose one canonical
`/api/security/status.sessionCapabilities.assistantEnabled` signal for the
closed assistant affordance, so unrelated shells do not probe
`/api/settings/ai` or `/api/ai/sessions` during ordinary route bootstrap just
to decide whether the assistant drawer may be opened.
That same frontend runtime boundary now also owns the shared AI read model for
AI-owned surfaces. `frontend-modern/src/stores/aiRuntimeState.ts` is the
canonical frontend owner for shared `/api/settings/ai` and `/api/ai/models`
reads used by chat, Patrol, and AI usage surfaces, while
`frontend-modern/src/components/Settings/useAISettingsState.ts` remains the
write-side settings owner. AI-owned surfaces must not fork their own mount-time
settings/model fetch loops once this store exists.
The assistant drawer/session shell is a separate shared boundary:
`frontend-modern/src/stores/aiChat.ts` owns open state, focused-input handoff,
context accumulation, and org-switch clearing for the assistant drawer, while
`frontend-modern/src/stores/aiRuntimeState.ts` owns the shared backend-backed
settings and model catalog reads. AI runtime consumers must not move drawer
shell state into page-local signals or teach `aiChat.ts` to bootstrap its own
`/api/settings/ai` or `/api/ai/models` reads.
That same drawer boundary owns responsive presentation too. The canonical
assistant drawer may dock and push the authenticated shell only when the
viewport is wide enough to preserve a usable primary operating surface; once
the available viewport drops below that shell threshold, the drawer must
become an overlay owned by `frontend-modern/src/components/AI/Chat/index.tsx`
instead of compressing Infrastructure, Workloads, Storage, or other primary
runtime pages into an unusable narrow column or forking page-local layout
exceptions.
The closed assistant launcher follows the same shared-shell rule. While the
mobile navigation shell is active, `frontend-modern/src/AppLayout.tsx` must
present the launcher as a bottom floating affordance that clears the mobile
nav instead of restoring the desktop right-edge rail at an earlier breakpoint.
The edge-mounted launcher is only valid at the desktop shell breakpoint where
the primary navigation and page chrome are also desktop-mode.
Non-AI shell notices may coexist in `frontend-modern/src/AppLayout.tsx`, but
they must remain presentation-only. Prerelease banners, billing callouts, or
other header-adjacent notices must not fork assistant open state, gate on AI
runtime fetches, or move assistant availability logic out of
`frontend-modern/src/stores/aiChat.ts` and `frontend-modern/src/useAppRuntimeState.ts`
just because they share the same authenticated shell. The remaining
prerelease-shell treatment is the compact `Preview` badge on rc-channel
builds; `frontend-modern/src/AppLayout.tsx` must not revive a standalone
release-candidate banner, release-notes CTA, or feedback CTA that starts
participating in assistant-shell state or modal ownership.
The retired monitored-system capacity banner follows the same shell rule:
`frontend-modern/src/App.tsx` must not reintroduce app-shell commercial
volume warnings just because settings or support surfaces still expose
monitored-system grouping data. Assistant state and shell notices stay
independent from retired infrastructure-volume commerce.
That same shared shell boundary must respect blocking modal ownership.
`frontend-modern/src/App.tsx` and `frontend-modern/src/AppLayout.tsx` may use
the shared dialog runtime to hide the closed assistant launcher and close the
drawer while a blocking shared dialog is open, but they must not leave Pulse
Assistant interactive behind a modal or fork a second assistant-open state
model to do it.
That same shared shell rule applies when presentation policy suppresses hosted
organization chrome: `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx` may hide org switchers or demo-only org
labels, but they must not couple assistant visibility, session reset, or
drawer-open behavior to that organization presentation state.
That same shell boundary also owns demo-only support-surface suppression:
Pulse no longer exposes Operations as a top-level route. Demo-only support
surfaces now hide inside the shared Settings navigation instead, and assistant
availability plus reset behavior must stay independent of that settings-nav
presentation choice.
Authenticated `/login` recovery belongs to that same route shell boundary:
once login succeeds, `frontend-modern/src/App.tsx` must resolve `/login`
through the canonical provider-first post-auth landing route instead of leaving
the assistant-capable authenticated shell stranded on a route that only exists
for logged-out presentation.
App-shell route preloading may include the Patrol route module, but it must
remain module-only. It must not prefetch AI settings, model state, findings,
chat sessions, or assistant context while the drawer is closed.
`docs/release-control/v6/internal/subsystems/registry.json` must therefore keep
`frontend-modern/src/stores/aiRuntimeState.ts` and
`frontend-modern/src/components/AI/Chat/` on the explicit AI runtime proof
route, and keep `frontend-modern/src/stores/aiChat.ts` on the shared
AI-runtime/frontend-primitives proof boundary instead of leaving the chat shell
or assistant drawer state unowned.
That same settings/runtime boundary now also governs BYOK first-run setup:
`frontend-modern/src/components/Settings/useAISettingsState.ts` may send only
provider credentials or base URLs when the operator connects a provider, and
`internal/api/ai_handlers.go` plus `internal/ai/service.go` must persist the
resolved provider model returned by the canonical runtime selection path. The
setup surface must not reintroduce vendor-default model IDs in modal payloads
just to make the backend accept the request.
Interactive Assistant chat must not put a Pulse-authored intent router, scout
model, or explore pre-pass in front of the operator's selected model. The
runtime may assemble governed context and expose the approved tool list, but
the selected model owns the decision to answer directly, ask a question, read
context, or request an action. Pulse must not use prompt heuristics to force
`tool_choice=any`, force a named tool, retry because an expected tool was not
used, or hide tools from the model based on keyword detection. Pulse
enforcement starts after that model choice: approval mode, FSM gates, strict
resource resolution, and tool policy remain the safety boundary.
Session continuity context follows the same boundary: Pulse may provide
neutral recent-resource facts and explicit resource addressing facts, but it
must not use prompt-keyword or pronoun heuristics to rewrite a user message as
targeted, inject log-routing instructions, or tell the model which context is
the answer. Pre-model context prefetch may only use structured resource
mentions explicitly selected by the operator; it must not scan plain chat text
for resource names, infer unresolved `@name` references, or inject lookup
results before the selected model chooses whether to use tools.
Legacy remediation memory follows the same boundary. Pulse may provide
resource-scoped prior action history as neutral context, but it must not
keyword-match the current problem against old fixes, label those fixes as
successful matches for the current issue, or use remediation memory to
recommend a command before the selected model has reasoned over current
evidence.

The same runtime ownership now includes the customer-facing AI usage and cost
surface. `frontend-modern/src/components/AI/AICostDashboard.tsx` is the
canonical AI usage dashboard shell, while
`frontend-modern/src/utils/aiCostPresentation.ts` owns its shared loading,
empty-state, and range-button presentation contract. Future cost-surface work
must extend those owners instead of reintroducing inline AI usage copy or
dashboard-local segmented-button styling.
The same runtime boundary also owns the shared AI semantic presentation
helpers used across chat, settings, and usage surfaces.
`frontend-modern/src/utils/aiProviderPresentation.ts`,
`frontend-modern/src/utils/aiProviderHealthPresentation.ts`,
`frontend-modern/src/utils/aiControlLevelPresentation.ts`,
`frontend-modern/src/utils/aiChatPresentation.ts`,
`frontend-modern/src/utils/aiSessionDiffPresentation.ts` are the canonical owners
for provider naming, provider health labels, control-level semantics,
chat drawer title/subtitle, launcher title/aria copy, session-menu labeling,
discovery hint framing, chat/session empty states, assistant message and
question-card labels, and session-diff badges.
Settings and chat surfaces must consume those helpers instead of keeping local
AI wording or model/provider inference branches.
Assistant chat must not render Pulse-authored explore pre-pass cards or
internal workflow-state cards as assistant output. The user-facing stream is
model text, model thinking where supported, model-selected tool calls, governed
approval requests, and model questions; internal runtime telemetry stays out of
the chat transcript.
Cold-start Assistant chat session creation is also stream-owned. Ordinary first
messages may call `/api/ai/chat` without a `session_id`; `chat.Service.ExecuteStream`
must create or resolve the durable session before provider execution and emit a
first-class `session` SSE event carrying `SessionData{ID: ...}`. The browser
chat runtime binds its active session from that stream event, with `done` and
`question` session identifiers retained only as compatible terminal/interactive
payloads, rather than issuing a separate `/api/ai/sessions` preflight before
the first user message. Explicit session-management actions may still create a
session through the session endpoint.

The AI transport files are shared with `api-contracts`, not delegated away to
it. `frontend-modern/src/api/ai.ts`,
`frontend-modern/src/api/patrol.ts`,
`internal/api/ai_handler.go`,
`internal/api/ai_handlers.go`, and
`internal/api/ai_hosted_runtime.go`, and
`internal/api/ai_intelligence_handlers.go` are runtime control surfaces for
the AI product while also remaining canonical payload contract boundaries.
That same AI transport boundary now also defines the narrow Pulse Mobile
runtime compatibility rule: mobile relay credentials are minted with the
dedicated backend-owned `relay:mobile:access` scope, and only the explicit
route inventory in `internal/api/relay_mobile_capability.go` may accept that
scope as a compatibility alias alongside legacy `ai:chat` or `ai:execute`
mobile tokens. Broader AI runtime surfaces must stay on their canonical AI
scopes instead of treating the mobile relay capability as a general-purpose
AI permission, and any new mobile-compatible AI route must land by extending
that governed backend inventory and proof set in the same slice.
That same shared AI transport boundary now also owns hosted AI bootstrap
retirement. When Pulse Cloud runs in hosted mode and no explicit `ai.enc`
exists yet, `internal/api/ai_hosted_runtime.go`, `internal/api/ai_handler.go`,
and `internal/api/ai_handlers.go` must return the same unconfigured
BYOK/local-provider default as self-hosted settings instead of deriving a
quickstart-backed managed-model config from hosted billing state. Any
explicitly written AI config remains authoritative, and hosted billing state
must not be converted into quickstart credits or a managed-model runtime.
That same hosted and self-hosted settings boundary must also retire legacy
hosted quickstart model aliases on read and write. Persisted values such as
`quickstart:minimax-2.5m` are historical implementation detail, not governed
runtime truth, so `internal/config/ai.go`,
`internal/config/persistence.go`, and `internal/api/ai_handlers.go` must clear
them before the runtime, API payloads, or structured logs consume those fields.
That same runtime boundary also owns approval-store lifecycle in
`internal/api/ai_handler.go`. Settings-driven enablement and restart must be
able to cold-start the direct AI runtime, initialize approval persistence, and
leave `/api/ai/approvals` ready for mobile and remediation flows even when AI
was disabled at process boot. The approval cleanup loop must follow owned AI
runtime lifetime rather than an HTTP request context, and approval persistence
may fail closed only when AI is actually disabled instead of because runtime
enablement happened after startup.
Pending approval reads from that store must be deterministic across web, mobile
relay, and API consumers: live pending approvals are ordered by soonest expiry,
then highest operational risk, then oldest request time, with approval ID as
the final tie-break so map iteration cannot decide which governed action looks
most urgent.
That same approval boundary also owns approved command execution. When
`internal/api/ai_handlers.go`, `internal/ai/service.go`, or
`internal/ai/tools/action_audit.go` consume a governed approval record, the
runtime must carry that approval identifier into the final
`agentexec.ExecuteCommandPayload` so the host agent can re-check the shared
command policy locally and fail closed on blocked or still-unapproved commands
instead of treating control-plane approval as an implicit bypass.
Discovery deep scans are the one runtime that does not flow through the
approval boundary. `internal/ai/discovery_adapter.go` is the only call site
allowed to mark an `agentexec.ExecuteCommandPayload` as `Trusted`. The
catalog of probes lives in `internal/servicediscovery/commands.go`, is
read-only by construction (`cat`, `ps`, `ss`, `find` under known config
roots) and is wrapped in `docker exec`, `pct exec`, or `qm guest exec`
without ever interpolating caller-supplied strings. Both the server-side
`agentexec` authorize path and the agent-side `hostagent` authorize path
must honor that `Trusted` flag by bypassing the approval requirement,
while still enforcing `PolicyBlock`. AI tool calls, Patrol fixes, and
Assistant remediation must continue to flow through the governed approval
record path and must never set `Trusted` on their payloads.
Discovery command-backed scans are additionally gated by the operator's
Discovery setting, not by the mere presence of a service-discovery store,
connected agent, or command-capable agent token. `discovery_enabled=false`
must fail closed for background sweeps, `/api/discovery/run`, forced
single-resource discovery, and `pulse_discovery` refreshes before any
`DeepScanner` command dispatch. `discovery_enabled=true` with
`discovery_interval_hours: 0` is the only manual-command-scan mode: recurring
scans stay stopped, but explicit admin-triggered refreshes may use the
hardcoded trusted catalog.
The value boundary for keeping Discovery is observed workload context:
Assistant and Patrol may consume normalized service name, version, endpoint,
port, config path, data path, log path, bind-mount, confidence, and user-note
fields through `pulse_discovery` or scoped prefetch. Raw command output remains
debug/admin material and must not become the primary Assistant context. When a
Discovery record includes a suggested web URL, the tool response and prefetch
summary may include that URL as observed context, but it must be treated as a
candidate rather than an operator-approved management URL.
The same action-audit boundary now also requires persisted action records to
carry a normalized plan and preflight: action id, request id, capability,
approval policy, dry-run availability, safety checks, verification steps, and
timestamps are normalized before persistence by the unified-resource store, so
runtime callers cannot publish an execution audit that skipped the canonical
planning contract.
Patrol investigation-fix approvals must use that same action-audit boundary:
when the orchestrator queues a fix approval, `internal/api/ai_handlers.go` must
attach a governed action plan, seed the shared action-audit store as planned
and pending with `pulse_patrol` as the requester/actor, and leave later
execution or approval decisions to the governed action/approval paths instead
of creating Patrol-only execution context or collapsing Patrol proposals into
generic Assistant-origin actions. The approval record itself must also persist
and expose that requester identity so `/api/ai/approvals` and Assistant
handoffs preserve Patrol provenance before later action-audit hydration refreshes
the current action state. Backend chat refresh of a Patrol finding handoff must
hydrate the same requester identity directly from the live approval record, so
Assistant does not depend on browser-authored metadata to distinguish
Patrol-origin proposals from generic Assistant actions.
The same ownership includes the Pulse query tool schema under
`internal/ai/tools/`: topology-query input names must stay canonical inside
the AI runtime itself, so new tool arguments such as `max_proxmox_nodes`
cannot reintroduce parallel legacy aliases once the backend query contract is
renamed.
That same AI tool ownership also governs `pulse_read action="exec"` safety.
`internal/ai/tools/tools_query.go` and `internal/ai/tools/tools_read.go` must
fail closed on unknown commands: the shared read path may execute only commands
that are known read-only by construction or proven read-only by an explicit
content inspector. The runtime must not preserve a model-trusted fallback for
unknown binaries, custom scripts, downloads, shells, or dual-use interpreters
such as `python`, `node`, `ruby`, `perl`, `bash`, or `sh`, because those
surfaces can mutate state even when invoked in non-interactive forms. Wrapper
and inspection-shaped commands must inherit the same fail-closed boundary:
`timeout` may only bound an inner command that independently classifies as
read-only, `env` with a utility is executable and therefore blocked, `find`
must reject write or exec actions, `awk` and `sed` must not regain read-only
status through direct invocation or pipes, `wget` is read-only only for spider
checks, and `curl` must reject request-body, mutation-method, config, cookie
jar, upload, and file-output forms while preserving ordinary HTTP(S) probes.
That same AI tool ownership now also includes canonical resource-native
control. `internal/ai/tools/executor.go`,
`internal/ai/tools/tools_control.go`, and `internal/api/router.go` must keep
API-backed control actions such as TrueNAS app start/stop/restart on the
shared `pulse_control` tool with `type="resource"` and native audited
execution, instead of adding provider-local control tools or bypassing the
shared approval and policy model.
That same AI tool ownership now also includes canonical resource-native
diagnostics. `internal/ai/tools/tools_read.go`,
`internal/ai/tools/executor.go`, and `internal/api/router.go` must keep
API-backed app log reads such as TrueNAS app-container logs on the shared
`pulse_read` tool with `action="logs"` and `resource_id=<canonical app>`
instead of requiring `target_host` for non-agent platforms or adding a
provider-local log-read tool.
That same AI tool ownership now also includes canonical resource-native
configuration reads. `internal/ai/tools/tools_query.go`,
`internal/ai/tools/executor.go`, and `internal/api/router.go` must keep
API-backed app configuration reads such as TrueNAS app-container runtime
shape on the shared `pulse_query` tool with `action="config"` and
`resource_id=<canonical app>` instead of forcing those resources through the
guest-config shim or adding a provider-local config tool.
That bounded tool set is the current Assistant floor for TrueNAS. Supported
now means read-side app logs/config and native app start/stop/restart on
canonical `app-container` resources through the shared `pulse_read`,
`pulse_query`, and `pulse_control` tools. Pulse does not promise a blanket
TrueNAS admin plane, host command execution on API-backed systems without the
unified agent, or provider-local AI tools outside the shared action-governed
runtime contract.
That same platform-claim boundary now also covers the admitted VMware vSphere
direction. The phase-1 Assistant floor is
read-only access to canonical VMware-backed `agent`, `vm`, and `storage`
resources through the shared read and query paths only. The AI runtime must
not add VMware-local tools or action verbs for VM power, snapshot lifecycle,
guest operations, host maintenance, or cluster administration before the
governed action surface expands.
That same VMware AI rule now also includes capability exposure. Even if
runtime code can identify VMware-backed actions through upstream APIs,
canonical resource capabilities and tool routing must stay read-only in phase
1: shared `pulse_read` and `pulse_query` may expose VMware-backed context, but
`pulse_control` must not grow VMware verbs and VMware-backed resources must not
advertise action metadata that implies a supported VMware admin plane.
That same capability boundary also governs resolved-context enforcement inside
`internal/ai/chat/context_prefetch.go`, `internal/ai/tools/tools_query.go`, and
`internal/ai/tools/tools_control.go`. Once the shared runtime has resolved a
canonical VMware-backed `agent`, `vm`, or `storage`, Assistant summaries may
not emit `pulse_control` instructions for it. Phase-1 VMware host and
datastore summaries without discovery must direct `pulse_query` or
`pulse_read` only, VMware guest summaries must stay explicitly read-only, shared
resource registrations must stay limited to read-side actions, and any
attempted `pulse_control` restart/stop/shutdown path must fail as a read-only
denial instead of falling through to legacy guest resolution or provider-local
control assumptions.
That same boundary also governs shared Assistant wording in
`internal/ai/chat/service.go` and `internal/ai/tools/tools_control.go`: the
base system prompt and `pulse_control` schema/description must not claim that a
generic `vm` or `system-container` is controllable. Shared AI text must describe
control as capability-gated and explicitly allow read-only platform variants
such as VMware phase-1 guests.
That same VMware AI rule also includes the investigation path. Alarm, health,
event, task, metrics-history, and snapshot-tree context for VMware-backed
resources must stay reachable through those same shared read/query surfaces
and canonical resource links rather than through a VMware-only AI tool or
provider-local incident adapter.
That same shared read/query rule also governs AI prompt hints and prefetch
summaries in `internal/ai/chat/service.go` and
`internal/ai/chat/context_prefetch.go`: API-backed read-only resources such as
VMware-backed `agent` / `vm` / `storage` and TrueNAS-backed host/storage
resources must not inherit synthetic `target_host` log-routing hints from
agent-routed platforms. Shared AI context should carry canonical
`resource_id` guidance for those resources, and `pulse_read action=logs` may
only be suggested when the runtime has an explicit native resource read path
such as supported TrueNAS `app-container` logs.
If a caller still targets `pulse_read action=logs` with `resource_id` for a
resource that lacks that native log path, the shared tool boundary must fail
as a structured blocked response with a governed recovery hint toward the
correct shared path, such as `pulse_query action=get` for API-backed read-only
resources or `target_host` plus `container` for agent-routed app logs.
When that recovery path is safe to execute deterministically, the blocked
response should also carry a structured recovery tool call so the shared
agentic loop can retry through the correct shared tool and arguments instead
of assuming every recovery is a `command` rewrite on the original tool.
That same VMware AI rule also now includes mention resolution. Frontend
Assistant mention payloads for VMware-backed `agent`, `vm`, `storage`, and
canonical `app-container` resources must preserve the shared unified resource
ID coming from `/api/resources`, and backend prefetch/runtime code must
resolve those mentions through canonical read-state lookups rather than
reconstructing provider-local IDs in the UI or adding VMware-only read routes
under `/api/vmware/*` for Assistant context.
That same AI tool ownership also applies to recovery-backed storage reads.
When `internal/ai/tools/adapters.go` returns recovery points with malformed
persisted metadata omitted at the shared recovery-store boundary, the storage
tool runtime in `internal/ai/tools/tools_storage.go` must still keep snapshot
and backup-task results visible by preferring canonical point fields such as
`display.clusterLabel`, `display.nodeHostLabel`, `display.entityIdLabel`,
`display.itemType`, and point outcome before falling back to raw `details`.
That availability contract also applies when recovery points are the only storage data source.
`internal/ai/tools/executor.go` must keep `pulse_storage` exposed whenever a
`RecoveryPointsProvider` is configured, so tenant and self-hosted Chat surfaces do not lose
recovery-backed snapshot and backup-task reads just because backup/read-state adapters are absent.
Tenant-scoped AI services must now also follow canonical runtime ownership:
Patrol may initialize and operate from tenant `ReadState` and unified-resource
providers without requiring a tenant snapshot-provider bridge, and
`internal/api/ai_handlers.go` must not mint tenant-local `StateSnapshot`
adapters purely to satisfy Patrol when canonical tenant read-state is already
available.
That same AI ownership also extends to persisted runtime state under
`internal/config/persistence.go`: AI findings, usage history, patrol run
history, and chat sessions must not keep legacy plaintext files on the runtime
primary path once the process can read them. Plaintext AI persistence files may
only serve as migration input and must be rewritten immediately into
encrypted-at-rest storage on load.
That same Patrol runtime ownership also governs Patrol run-summary taxonomy.
`internal/ai/` must keep API-backed TrueNAS systems distinct from unified-agent
hosts in runtime counts, triage summaries, and persisted Patrol run history
instead of collapsing both surfaces back into `hosts_checked` or generic
`agent` resource wording.
That same config-persistence boundary also owns fixed runtime file paths: the
resolved data directory must be normalized once and fixed AI/runtime filenames
must rejoin through the shared storage-path helper instead of raw
`filepath.Join(dataDir, "...")` construction.
That same persistence boundary also governs AI memory package storage roots:
fixed store files such as change history, incident memory, and remediation
history must resolve through normalized owned data directories and fixed
storage-leaf joins instead of raw `filepath.Join(dataDir, ...)` paths.
The same migration-only rule applies to guest knowledge under
`internal/ai/knowledge/`: legacy `.json` knowledge files and plaintext `.enc`
knowledge files may only serve as migration input, and the knowledge store
must rewrite canonical encrypted-at-rest storage immediately on load instead
of leaving guest knowledge plaintext on disk until a future note update.
That same knowledge-store boundary also governs directory scans: when the store
rejoins discovered knowledge files for reads, it must route those already-owned
leaves back through the shared storage-path helper instead of rebuilding raw
`filepath.Join(dataDir, entry.Name())` paths.
Chat-session and guest-knowledge persistence now also keep canonical on-disk
names opaque and machine-owned. Legacy identifier-derived filenames may be
discovered only by inspecting already-owned files for embedded record IDs, and
the next successful write must rewrite them to hashed canonical paths instead
of preserving user-controlled identifiers as filesystem path segments.
That trust boundary also applies when the store is constructed: if the
knowledge store cannot initialize encryption, construction must fail closed
instead of silently creating a plaintext-at-rest runtime store.

Unified-resource-backed AI context now also consumes the canonical
policy-aware metadata contract. The AI runtime may summarize governed resource
policy counts for context, and it must switch to `aiSafeSummary` when a
resource is marked `local-only` instead of leaking raw resource names or local
identifiers for restricted resources through ad hoc context formatting.
That governed context should also surface the canonical routing posture and
redaction hints that were derived from the shared policy model, so prompts
reflect the same sensitivity, routing, and scrub decisions that the runtime
uses for export boundaries instead of rebuilding privacy posture locally.
That governed posture block and its export-routing inputs now also flow through
the dedicated `internal/ai/resource_context_policy_model.go` owner, so
`resource_context.go` stays on AI context composition instead of duplicating
policy redaction sections or recomputing export metadata inline.
That same ownership now includes the canonical policy-posture summary object
itself: `resource_context.go` must compute the shared
`unifiedresources.SummarizePolicyPosture(...)` result exactly once per unified
context build and pass that summary into
`buildUnifiedResourcePolicyContext(...)`, instead of letting downstream AI
context helpers silently rebuild posture counts from the raw resource slice.
The same shared policy presenter also owns the routing-scope labels used in
the AI-facing policy surfaces, so the policy wording stays canonical instead
of being rendered inline by the consumer.
That same policy boundary now applies to chat structured-mention prefetch and
resource-summary formatting: mention resolution must consume canonical
unified-resource policy metadata, skip discovery fan-out when governed
redaction already blocks cloud-safe raw context, and withhold routing
coordinates, bind-mount paths, hostnames, and discovery file paths whenever
resource policy marks those identifiers as redacted.
The governed mention formatter must also render the policy line and redaction
list through the shared unified-resource policy presentation helper so the
chat prefetch path stays aligned with the same canonical sensitivity, routing,
and redaction labels used by the AI summary and resource drawer.
The decision to show that governed mention block now comes from the shared
unified-resource policy helper as well, so the local gate stays aligned with
the same routing and redaction rules as the rendered summary itself.
The governed mention preamble and footer text now also come from the shared
policy presenter, so the warning copy around the block does not drift from the
canonical policy wording.
The complete governed mention block is also assembled by the shared policy
presenter, so chat prefetch only decides when to render it and never rebuilds
the summary layout locally.
The chat prefetch path now also calls the shared governed-summary predicate
directly at each mention site, so it no longer carries a local wrapper around
the canonical policy decision or a separate mention-summary trim helper.
Structured mention resolution also uses the shared AI tools discovery
canonicalization helpers now, so chat prefetch and discovery responses agree
on resource-type and target-ID formatting instead of maintaining chat-local
copies.
The chat mention picker now also carries the canonical preferred resource
label as `label` through the structured mention payload, and the insertion
path uses that same label for prompt text and cursor placement, so mention
search, selection, and submission do not depend on a raw `displayName` field
fork.
Structured `app-container` mentions must now use canonical unified-resource
identity (`app-container:<host>:<provider_uid>`) instead of a Docker-transport
ID. Frontend mention pickers should emit that canonical ID for every
app-container, including API-backed platforms such as TrueNAS, while backend
structured-mention resolution may continue to accept legacy `docker:...`
mentions only as a compatibility path.
Compatibility-only top-level TrueNAS mention types must also collapse to the
canonical `agent` host type at that same handler boundary, so the AI runtime
does not carry a parallel raw `truenas` mention contract once transport input
has been normalized.
That same compatibility-collapse rule also applies to alert, finding, and
Patrol scope payloads. API-backed TrueNAS systems may still keep `truenas`
platform metadata and separate run-history coverage counts, but AI resource
type fields must normalize to canonical `agent` once they cross the governed
runtime boundary.
The same governed-context rule also applies to the main unified AI resource
overview: infrastructure, workload, alert-label, and top-consumer summaries
must not leak raw resource names, cluster labels, IP addresses, or unresolved
topology identifiers once canonical resource policy marks aliases, hostnames,
platform IDs, or addresses as redacted. Sensitive resources should remain
useful through `aiSafeSummary` and explicit redaction markers rather than
falling back to raw local identifiers in list or summary sections.
That same governed policy boundary now extends through AI tool payloads and
chat-memory extraction. Resource-bearing `pulse_query` results must carry the
canonical `policy` and `ai_safe_summary` fields derived from unified resources,
and deterministic knowledge extraction must prefer those governed summaries
when policy redaction covers aliases, hostnames, or platform IDs instead of
persisting raw resource labels into cached AI facts.
That same `pulse_query` boundary now also owns canonical resource coverage for
API-backed platforms such as TrueNAS. The runtime must expose canonical
`agent`, `app-container`, `storage`, and `physical-disk` resource views
through the shared unified-resource model instead of falling back to Proxmox-
or Docker-local enumerations when a platform projects onto canonical host,
storage, disk, or workload contracts. Compatibility aliases such as
`system` and `storage-pool` may still be accepted at the `pulse_query`
boundary, but the governed runtime contract is the canonical `agent` /
`storage` read path and the resolved-context registration emitted from it.
That same runtime contract applies to resource-native diagnostics. When
resolved context points at an API-backed canonical `app-container` such as a
TrueNAS app, chat/runtime prompt hints and tool execution must route log reads
through `resource_id` on `pulse_read` rather than inventing agent-host hints
for platforms that are not reached through the unified agent.
Unified AI context should follow the same rule: storage summaries may mention
canonical storage pools and physical disks that need attention, but must not
mislabel lower-topology storage resources such as TrueNAS datasets as
top-level pools.
That same requirement includes `pulse_query action=config`: guest-config
payloads must carry canonical resource policy metadata, and config-fact
extraction must not persist raw guest hostnames when governed redaction covers
hostname or platform identity fields. The same `action=config` contract now
also applies to API-backed canonical `app-container` resources such as
TrueNAS apps: runtime routing must resolve the shared resource identity first
and then read native config through the owned provider path rather than
falling back to guest semantics.
Outbound model-bound context exports now also belong to this runtime
boundary. When the AI service assembles unified-resource context for a model
request, it must record a durable export audit with the active destination
model and governed redaction decision instead of treating the prompt boundary
as a transient formatting step.
That export decision must come from the shared unified-resource privacy
helpers, so sensitivity floors and redaction-triggered routing stay aligned
with the canonical policy contract instead of being recomputed in AI-local
code.
The export audit should also record canonical human-readable redaction labels
from the shared policy presentation helper, so the audit trail and the
resource-context surfaces speak the same governed redaction language instead
of reformatting hint names locally.
The canonical AI-safe summary builder also owns the `sensitive` and
`restricted` suffix phrases, so downstream AI consumers should treat those
ending fragments as shared policy output instead of inventing their own
wording.
The same AI runtime boundary now also consumes the canonical unified-resource
timeline when it assembles rich resource or incident context. Recent-change
context should come from the shared resource store first so AI prompts reflect
the same change record that powers the resource API, with patrol-local change
detectors only serving as fallback coverage when the canonical store is not
available. When that patrol-local fallback is used, it must render through the
shared memory change presentation helper so the same heading, scope prefix, and
change-type labels are reused instead of being rebuilt ad hoc in AI-local code.
`internal/ai/memory/incidents.go` is therefore an alert-scoped investigation
projection only: it may retain notes, analysis, command executions, runbooks,
and alert lifecycle breadcrumbs for one incident, but it must not become a
parallel source of truth for durable backend history that already belongs to
`internal/unifiedresources/`.
When canonical resource history is available, the incident read path must also
project alert lifecycle and remediation entries back out of the unified-resource
timeline instead of reading those durable facts only from AI memory. AI memory
may retain annotation-only entries such as notes and analysis, but the live
incident timeline shown to handlers, prompts, and operators should read as one
projection over canonical resource history plus investigation-local annotations.
That read-side projection must also discard incident-local derived lifecycle
state when canonical history is present: acknowledgement, resolution, and
command or runbook entries in `internal/ai/memory/incidents.go` may still
exist as compatibility-era shell state for segmentation and fallback, but the
projected incident returned to runtime consumers must rebuild those fields from
canonical resource changes and preserve only annotation-local entries such as
analysis and notes.
The remaining shell should stay as narrow as possible: alert occurrence
boundaries and annotation anchors may remain private implementation state, but
public incident status, acknowledgement, and remediation entries should be
treated as read-model output rebuilt from canonical history whenever that
history exists.
That boundary should also be visible in code shape: the persisted incident
shell used by `internal/ai/memory/incidents.go` should stay a private storage
model for occurrence segmentation and annotations, while the exported
`Incident` type remains the public/projected read model returned to handlers
and operators.
The AI correlation root-cause engine also consumes the canonical unified-
resource relationship model directly, so cross-resource reasoning stays aligned
with the same relationship edges that back the resource API instead of
maintaining a parallel relationship vocabulary inside AI correlation.
The canonical relationship-summary helper also feeds resource change records,
so AI timeline prompts read the same relationship wording and edge labels that
the unified-resource contract emits instead of building another summary shape
in AI-local code.
The same shared change presenter also owns the resource state, restart,
incident, and config summary fragments used by change emission, so the AI
timeline prompt can reuse the canonical from/to wording before it formats the
markdown section itself.
The Patrol-backed correlation endpoint, resource-intelligence payload, and
seed prompt correlations now flow through the shared AI intelligence facade
first, so the detector remains an implementation detail behind one canonical
correlation access path instead of being routed directly by handlers or prompt
builders.
AI-facing policy metadata must also be cloned through the shared unified-
resource policy helper so chat and tools consumers do not maintain their own
policy copy logic. Chat mention prefetch now calls that shared helper directly
at each resolved mention site rather than going through an AI-local wrapper.
AI resource and intelligence consumers now also refresh canonical identity and
policy through the shared unified-resource metadata helper, so the AI runtime
no longer keeps its own slice-level normalization shim for the same
composition.
Chat knowledge extraction and resource-context rendering now also consume the
shared unified-resource label helpers directly, so governed labels and
redacted values stay consistent without AI-local presentation shims.
Those same paths also use the shared resource display-name helper, so the
name-or-ID fallback stays aligned across chat extraction, resource context,
and unified adapter presentation.
The unified resource context's IP summaries now also route through the shared
policy redaction helper, so the local "IPs" line follows the same governed
redaction decision and label vocabulary as the rest of the policy-aware
resource presentation layer. Cluster labels for AI resource context now also
come from the shared unified-resource presentation helper, so the same policy
rules govern cluster names and IP summaries instead of leaving the fallback
logic in the AI package.
The policy-posture aggregate itself now also comes from
`internal/unifiedresources/policy_posture.go`, so AI summaries and resource
context reuse the same canonical sensitivity, routing, and redaction counts
instead of collecting governance posture in an AI-local helper.
That shared presentation layer also owns the elapsed-time and "ago" wording
utilities, so the same "time ago" phrasing stays consistent across resource,
incident, and fallback memory summaries instead of being reformatted
independently.
The canonical resource-change kind, source type, and source adapter labels
now also come from the shared change presentation helper, so the resource
summary card and drawer history use the same badge vocabulary instead of
hardcoding their own labels.
Action-plan stale-plan protection now keys the durable audit payload on the
canonical `resourceVersion`, `policyVersion`, and `planHash` fields only,
so the audit record stays on the minimal deterministic contract instead of
carrying extra versioning for relationship topology.
Resource-only incident context should follow the same rule: if an alert
timeline is absent, the incident prompt path should fall back to the canonical
unified-resource timeline rather than depending only on patrol-local change
memory.
When both an alert identifier and a canonical resource ID are known, the prompt
path should include both surfaces in source-precedence order: alert-scoped
incident memory first, canonical resource timeline second.

The same runtime boundary now also owns durable action execution auditing.
`internal/ai/chat/service.go` initializes the unified-resource audit store on
startup. Governed API action execution must enter through
`POST /api/actions/{id}/execute`, which records `executing` before invoking the
registered executor and records the terminal `completed` or `failed` result
afterward; missing executors must fail closed without mutating the approved
audit record. Existing write-action tool paths under `internal/ai/tools/`
must keep their persisted lifecycle and result records aligned with that same
unified-resource action state machine: approval decisions must use the
canonical action decision transition, execution starts must use
`BeginActionExecution` plus `RecordActionExecutionStart`, and terminal tool
results must use `CompleteActionExecution` plus
`RecordActionExecutionResult` rather than inventing AI-local execution states.
AI incident handling must now also write durable resource-history facts
through the canonical unified-resource change store when a concrete resource
target is known. Command executions and runbook executions triggered during an
alert investigation may remain visible inside `internal/ai/memory/incidents.go`
as operator-facing incident projection entries, but the durable backend truth
for those events now belongs to canonical `ResourceChange` kinds such as
`command_executed` and `runbook_executed`, keyed by canonical resource ID and
linked back to the alert through metadata instead of being stored only in AI
memory.
The patrol-local `memory.ChangeDetector.GetChangesSummary` path now also
delegates to the shared memory recent-change presentation helper, so any
future fallback summary entry point inherits the same heading, resource
prefixing, and change-type labels without re-implementing the markdown shape.
Those unified-resource action and export audit records are now also exposed
through the enterprise audit read surface so operators can inspect the
execution trail without reaching into storage internals.
AI resource and incident context now also surfaces a canonical relationship
section from unified-resource relationships, so relationship wording and edge
provenance stay aligned with the same shared resource model instead of being
reconstructed from the drawer or prompt helpers.
That relationship section is now rendered by the shared
`internal/unifiedresources.FormatResourceRelationshipContext` helper, so the
service layer only resolves the canonical resource and does not rebuild the
section format locally.
The canonical recent-change sentence formatting also lives in
`internal/unifiedresources.FormatResourceChangeSummary`, so AI runtime prompt
sections and Patrol seed context reuse the same change wording instead of
keeping another lane-local formatter.
The confidence percentage wording used by the drawer's change timeline rows
also flows through a shared frontend formatter, so the same `50%`-style
labels stay consistent across timeline surfaces instead of being re-derived
in the component.
The remaining fallback token humanization used by those same timeline and
drawer surfaces also flows through one shared frontend helper, so the
title-casing and underscore cleanup used for change and drawer labels stay
centralized instead of being reimplemented locally.
The canonical recent-change section wrapper also lives in
`internal/unifiedresources.FormatResourceRecentChangesContext`, so the AI
summary and resource-specific context share the same heading and prefix rules
instead of rebuilding that section layout locally.
The canonical memory conversion helpers also live in
`internal/ai/memory/presentation.go`, so the Patrol fallback feed and the
AI summary path translate between unified-resource changes and memory.Change
through one shared adapter boundary instead of keeping local shims.
The related-resource correlation section now also comes from the shared
correlation formatter in `internal/ai/correlation`, so resource chat and
incident prompts reuse the same learned-edge wording instead of rebuilding a
second patrol-local bullet format.
The Patrol intelligence page now also fetches the learned correlation list
from the canonical AI correlations endpoint, so the global AI surface and the
resource drawer both expose the same learned edge evidence instead of only
showing a correlation count. The same page and drawer now render that list
through the shared `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
card, so the learned-correlation layout and edge wording stay aligned across
both surfaces. That shared card also owns the correlation ordering and
truncation rule, so callers pass raw learned edges instead of page-specific
top-N slices.
Assistant finding handoffs now also receive a model-only finding briefing
derived from the current unified finding and structured Patrol investigation
record before the lower-level finding context. That briefing must summarize the
finding, resource, priority, current recency facts, bounded evidence and
verification summaries, investigation confidence, latest lifecycle event, and
governed action artifact metadata as factual model context without generating
Pulse-authored attention, operator-decision, or remediation guidance, while
leaving detailed lifecycle history, current
resource-state, timeline, related-finding, and action-audit
hydration in the existing canonical AI runtime handoff builders. Related
root-cause and
correlated finding records may be summarized from current unified finding state,
including their recency and latest lifecycle facts, and may seed their own
handoff resources for canonical policy, state, topology, and timeline
hydration. That related context is explanation and review context only, not
approval or execution authority. Detailed lifecycle events are
likewise current Patrol review context only. The assembled briefing, lifecycle,
and related context are policy-sanitized by the chat handoff runtime before
prompt injection, so governed resource names, IDs, aliases, nodes, paths, and
addresses are redacted or represented through the canonical AI-safe summary
instead of leaking through product prose.
The same page and drawer now also render their recent-change timeline through
the shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
card, so the canonical recent-change layout and relative-time wording stay
aligned across both surfaces instead of being rebuilt as page-local feeds.
The Patrol intelligence seed context now also prefers the canonical
unified-resource timeline before falling back to the patrol-local change
detector, so deterministic patrol context and resource detail context share
the same change source of truth.
The unified intelligence summary should follow the same rule when it counts
recent activity, so the shared AI summary and the Patrol seed context stay
aligned with the canonical timeline.
The same unified intelligence summary now also surfaces a canonical policy
posture snapshot derived from unified resources, so sensitivity, routing, and
redaction counts stay aligned with the governed resource model that the
runtime uses for prompt export and context rendering.
That posture snapshot must render redaction labels through the canonical
unified-resource hint order, not alphabetically, so the AI summary, drawer,
and any future policy surfaces all present the same redaction precedence.
Its sensitivity and routing counts must also follow the canonical
unified-resource order and shared human-readable count summaries, so both the
backend summary and the frontend policy card stay aligned on the same
presentation sequence.
The unified AI resource data-governance block must also use the shared
unified-resource redaction-label helper directly, so the same canonical
policy labels back both the posture summary and the governed prompt context
without an AI-local wrapper.
The governed query-fact and resource-context paths must also use the shared
unified-resource policy helpers for the `aiSafeSummary` decision and
redaction predicates, so the same local-only and redaction rules are applied
consistently instead of being reimplemented in chat-local helpers.
The frontend unified-resource hook now trusts backend canonical `policy` and
`aiSafeSummary` values directly, so the canonical summary and policy posture
stay aligned with the same resource-policy boundary that governs
policy-aware routing and redaction without any frontend-local re-normalization.
The resource detail drawer now also resolves the visible AI-safe summary
through the same shared policy helper, so governed resources still show the
canonical redacted label if the backend summary is missing instead of
silently dropping the summary block.
The per-resource intelligence payload returned from
`/api/ai/intelligence?resource_id=...` now carries recent changes,
dependencies, dependents, correlations, and knowledge only; policy posture
stays on the system-wide intelligence summary and the Patrol governance card
instead of riding the resource-detail payload.
That same resource-intelligence payload also carries dependency and
dependent correlation context from unified-resource correlations, so the drawer
can show canonical correlation relationships without reconstructing them from the
relationship timeline alone.
The shared AI resource and infrastructure prompt contexts should also surface
the same canonical recent changes section before any patrol-local fallback so
the model sees the same timeline entries that power the resource API and
intelligence summary counts.
The `/api/ai/intelligence/changes` endpoint should also route through the
canonical unified-intelligence recent-change accessor before any
patrol-local detector fallback, so the API surface reads the same unified
timeline source that powers the summary payload.
Retired dashboard Pulse Brief context follows the same monitoring-first AI
boundary in negative space: `frontend-modern/src/features/dashboardOverview/`
and the Dashboard route must not be restored just to create an Assistant-ready
operator paragraph. Future overview or brief surfaces need a governed product
owner first, must pass fact-bound structured context from owning Infrastructure,
Workloads, Patrol, storage, recovery, and alert summaries, and must not let an
unbounded prompt become a route's source of truth.
Future route-to-Assistant handoffs must also keep their execution mode scoped
to the request. When an overview brief opens Assistant, the drawer may prefill
only governed prompt/context data, but the submitted chat request must set
`autonomous_mode:false`, preserve the operator's persistent Assistant
control-level setting, and disclose the temporary approval-required mode in
the drawer instead of showing the generic Autonomous warning.
Scoped Assistant handoffs that originate in owned product surfaces may also
send bounded `handoff_context` text, structured `handoff_resources`, and safe
structured `handoff_actions` through `frontend-modern/src/api/aiChat.ts` and
`/api/ai/chat`. That context is model-only session metadata, not saved
user-authored message text, and the backend must clamp the exchange to
approval-required mode whenever such scoped handoff context, resources, or
action references are present. Patrol finding IDs remain stricter: when
`finding_id` resolves, backend-refreshed durable Patrol context remains the
canonical authority; the handler may merge only a recognized same-finding
Patrol product handoff section as secondary model-only briefing, and it must
drop mismatched resource/action references plus raw command payload lines.
Direct alert-investigation runtime handoffs follow the same rule even when
they bypass the chat drawer. `/api/ai/investigate-alert` must set
`ai.ExecuteRequest.AutonomousMode` to false plus
`ai.ExecuteRequest.RequireCommandApproval` to true, and
`internal/ai/alert_provider.go` must frame diagnostics as approval-bound
operator actions rather than instructing the model to execute commands because
they appear safe.
Those backend AI and Patrol change summaries should derive their canonical
labels and provenance fragments from
`internal/unifiedresources/change_presentation.go`, so the resource-model
semantics are shared before any surface-specific markdown styling is applied.
The patrol-local recent-change fallback itself should derive its section layout
and change labels from `internal/ai/memory/presentation.go`, so detector-based
fallbacks stay consistent across AI runtime entry points when the canonical
resource timeline is unavailable.
The per-resource intelligence payload returned from
`/api/ai/intelligence?resource_id=...` should also include the canonical
`recent_changes` history so UI and API consumers can read the same timeline
slice that the prompt context uses.
The system-wide `/api/ai/intelligence` summary should also surface the same
canonical recent-change slice, alongside the count, so the aggregate payload
and the prompt context stay aligned on the same shared timeline source.
The frontend Patrol intelligence page now also consumes that canonical
summary payload directly through the shared AI client and store, so the
visible summary card stays aligned with the same recent-change slice that the
runtime and API contracts expose.
The Patrol runtime now also exports a canonical `runtime_state` alongside
`blocked_reason` in the Patrol status payload, so provider-availability and any
legacy managed-credit block conditions remain part of the governed runtime
contract instead of being inferred later from the last successful patrol
summary.
When missing provider configuration blocks Patrol, `blocked_reason` must point
to Assistant & Patrol provider settings and tool-capable Patrol model
selection.
That runtime-state contract must be derived from live Patrol runtime inputs,
not only from the last failed run attempt, and the backend must clear any stale
managed-credit block once a provider or local model configuration returns.
The same runtime contract now also governs when the system-wide Patrol health
summary is allowed to read as healthy. `internal/ai/intelligence.go` must not
derive `Health A` or `100/100` from "no active findings" alone when recent
Patrol evidence is limited to alert-scoped runs or includes recent Patrol run
errors; the summary must degrade and explain that overall infrastructure health
is not fully verified until a recent successful full Patrol run exists.
That coverage explanation must also stay faithful to the actual recent run
shape. When the most recent verification evidence includes a full Patrol run
that ended with errors, the health summary must say that a recent full patrol
errored rather than claiming recent activity was limited to scoped runs.
The Patrol status payload must keep that same scope distinction explicit in its
own recency fields. `last_patrol_at` is reserved for the most recent completed
full Patrol run, while scoped runs and fix-verification checks advance
`last_activity_at` without pretending a full verification sweep just happened.
That same runtime contract also owns scoped trigger source policy. Alert- and
anomaly-triggered Patrol work are independent runtime gates; the canonical AI
settings model must preserve them separately, and runtime status must expose
which scoped sources are enabled plus whether queued scoped work or busy-mode
acceleration is currently active.
That same runtime boundary also owns which Patrol work counts toward
full-patrol cadence gates. Community-tier or other full-run limits must key
off completed full sweeps only; recent scoped or verification activity may
advance `last_activity_at`, but it must not block a manual full Patrol request
as if a scheduled estate-wide sweep already happened.
The Patrol startup scheduler must preserve that coverage guarantee as well:
`internal/ai/patrol_run.go` may skip the startup full patrol only when recent
run history already includes a successful full Patrol run, not merely because
some recent scoped alert-triggered run exists.
The Patrol runtime also owns synthetic Patrol service findings canonically.
Provider-credit and provider-auth failures raised against the synthetic
`ai-service` Patrol resource are runtime conditions, not inventory resources,
so the full-run seed/reconcile path must not auto-resolve them as
`Resource no longer exists in infrastructure` just because `ai-service` is not
present in the infrastructure snapshot. Those findings stay active until
Patrol actually succeeds or resolves them for a Patrol-owned reason.
That success boundary includes provider-backed scoped Patrol runs. A successful
scoped run proves that Patrol can currently reach the selected provider/model
and complete tool-backed analysis, so it must clear the synthetic
`ai-service` runtime failure just as a successful full Patrol run does, without
loosening ordinary scoped finding reconciliation for infrastructure issues.
Because those findings represent Patrol blindness rather than operator-triaged
infrastructure noise, the Patrol runtime must also reject manual acknowledge,
snooze, dismiss, resolve, and suppress actions against synthetic `ai-service`
runtime findings. The canonical recovery path is to correct Patrol provider
configuration in Assistant & Patrol settings and let Patrol re-evaluate the
runtime condition on the next run.
The shared findings lifecycle must also treat a regressed issue as a new active
occurrence. When a resolved finding reappears, `internal/ai/findings.go` must
clear any stale acknowledgement timestamp from the prior occurrence instead of
carrying that acknowledgement forward onto the regressed active issue. The
same owner must normalize already-persisted active findings on load when a
stored acknowledgement predates the last recorded regression, then persist the
cleaned state back through the canonical findings store.
AI chat tool-name labels, pending-tool headers, and assistant status copy now
also route through the shared frontend identifier-label helper, so the chat
surfaces do not keep their own underscore-stripping behavior separate from
the rest of the governed presentation helpers.
AI chat stream matching and mention dedupe now route through the shared
frontend chat identifier helper, so tool-name prefix stripping and mention-key
normalization stay aligned across the chat runtime instead of being redefined
inline in the stream processor or container component.
That same provider-stream boundary also owns EOF-safe SSE finalization for
OpenAI-compatible chat streams. Provider reads that return payload bytes with
`io.EOF`, or close immediately after the final `data:` frame, must still
process the buffered frame set and route tool-call assembly plus final done
event emission through the same canonical finalizer used for `[DONE]` instead
of dropping the last chunk or leaving tool calls unfinalized on clean close.
That same provider-transport boundary owns OpenAI-compatible tool protocol
adaptation. Pulse must keep normal tool selection automatic/model-owned for
OpenAI-compatible providers, including direct DeepSeek paths. Text-only
turns reached through loop, budget, or verification gates should omit tools
entirely instead of sending provider-specific `tool_choice=none`; that transport
setting must not be used as an intent classifier.
Reasoning-backed provider turns that return tool calls with `reasoning_content`
must preserve that reasoning state on the following tool-result turn when the
provider requires it, so Assistant and Patrol can complete multi-turn tool use
against live BYOK providers.
Readiness classification for the same provider path must be model-aware, not
provider-only. Current official DeepSeek V4 tool-capable models may report
Patrol readiness as ready; legacy DeepSeek aliases may only warn with the
alias-retirement posture and a recommendation to select the current V4 model
IDs; unknown direct DeepSeek model IDs must be not-ready with
`model_unavailable`; and known reasoning-only families must continue to fail
closed before Patrol work is admitted.
That same browser-owned chat read model must keep target normalization helper-
driven. Assistant shells may still derive legacy VM identifiers or display
labels for read-only targeting, but they must do so through shared helpers and
store context precedence instead of passing component-local resource objects or
duplicating naming fallbacks inline.
That same runtime boundary also owns executor session isolation. Shared AI
runtime services may reuse one canonical executor configuration, but each chat
or Patrol run must clone that executor before attaching resolved-context,
approval-routing, or patrol-finding state so concurrent sessions cannot
overwrite one another's mutable runtime context.
That same Patrol runtime boundary owns Community monitor-mode autonomy saves.
The open-source/free `PUT /api/ai/patrol/autonomy` adapter may persist
findings-only `monitor` configuration and the governed investigation budget /
timeout clamps, but it must continue to reject `approval`, `assisted`, and
`full` autonomy with the canonical safe-remediation license response.
The same canonical findings store owns dismissal-reason semantics. The three
`dismissed_reason` values must remain behaviorally distinct, not copy-only
variants: `not_an_issue` flips `Suppressed=true`, `expected_behavior`
acknowledges without escalation, and `will_fix_later` is an operator
commitment that populates `Finding.RemindAt` (default
`DefaultWillFixLaterRemindAfter`, 7 days). On re-detection, the canonical
store wakes a `will_fix_later` finding once `RemindAt` has passed by
clearing the dismissal and emitting a `reminded` lifecycle event, and the
`dismiss_finding` LLM tool response must communicate the remind-at date so
Patrol's conversational explanations stay aligned with the persisted
behavior.
The unified-finding mirror in `internal/ai/unified/alerts.go` also carries
that same `RemindAt` field so the API surface preserves the will_fix_later
wake-up deadline across the canonical findings store and the read model.
The `AddFromAI` dedup-merge path must mirror `RemindAt` onto the existing
record (including clearing it when a remind-at wake or undismiss has
already cleared the dismissal in the canonical store), and the TS API
clients in `frontend-modern/src/api/patrol.ts` and
`frontend-modern/src/api/ai.ts` must round-trip the `remind_at` field
verbatim so the operator surface can preview and badge the deadline.
The same Patrol API client also exposes the operator-driven manual
resolve path. `resolveFinding(findingId)` in
`frontend-modern/src/api/patrol.ts` must POST `{finding_id}` to the
canonical `/api/ai/patrol/resolve` endpoint owned by
`HandleResolveFinding` in `internal/api/ai_handlers.go`, mirroring the
acknowledge / snooze / dismiss client surface so the same Patrol service
contract drives every operator-feedback action.
The `unified.UnifiedFinding` mirror also carries an explicit
`AutoResolved` flag alongside `ResolvedAt`, set by the canonical
`Finding.AutoResolved` field. The AddFromAI dedup-merge path must
mirror that flag (allowing flips between auto-detected closure and
operator-driven closure as the canonical store transitions), and the
Finding to UnifiedFinding conversion in `internal/api/router.go` must
copy `f.AutoResolved` on both the live wire-up callback and the
persistence-recovery resync, so the frontend can honestly attribute who
closed the loop instead of flattening every resolution into a generic
"resolved" state.

The same canonical AI runtime now also owns the report-narrative surface.
`internal/ai.Service` implements `pkg/reporting.Narrator` and
`pkg/reporting.FindingsProvider` directly through `report_narrator.go` and
`report_findings.go` so the reporting engine can request an AI-generated
executive summary without depending on AI-internal types. The narrator is a
single-turn, no-tools call that reuses the canonical provider abstraction
already powering Patrol and Assistant; the request sanitizer, model
selection (`PatrolModel` preferred, falling back to `GetChatModel()`), cost
budget enforcement (`enforceBudget("report_narrative")`), and provider
factory must be the same shared seams used by `QuickAnalysis`, not a
parallel report-only provider stack. The structured report payload sent to
the model is denormalised through `buildReportNarratorPayload` so reporting
package types do not leak into the prompt surface, and the response is
parsed through `parseReportNarratorResponse` which tolerates an optional
`json` code fence the model may emit despite the no-fences instruction.
Severity normalisation maps the model's free-form output back onto the
narrative bullet severity set the renderer understands (`ok`, `info`,
`warning`, `critical`); unknown values default to `info` rather than
silently rendering as muted. Both interfaces fail closed: a nil provider,
parse failure, empty narrative, or context cancellation returns an error
so the reporting engine falls back to the deterministic heuristic
narrator. The findings provider filters the patrol findings store by
resource ID and lifecycle overlap with the report window via
`findingOverlapsWindow`, and truncates to `reportFindingsLimit` (25)
sorted entries so retrospective summaries stay within a predictable
prompt budget. Reporting is therefore an additive consumer of AI
runtime, not a new ownership boundary, and the narrator/findings
surfaces inherit the same governance the rest of the canonical AI
runtime already enforces.

The same canonical AI runtime now also owns the fleet-level report
narrative through `report_fleet_narrator.go`. `Service` implements
`pkg/reporting.FleetNarrator` with its own use-case label
(`report_narrative_fleet`) so fleet vs single-resource spend is
distinguishable in the cost ledger, and so the budget gate
(`enforceBudget`) and dashboard taxonomy can address the two
separately. The fleet payload is denormalised through
`buildReportFleetPayload` into compact per-resource rows plus a
fleet-wide aggregate so prompt cost scales linearly with fleet
size without exploding token usage. The same fail-closed invariant
holds: nil provider, parse failure, empty narrative, or context
cancellation returns an error so the reporting engine falls back to
`HeuristicFleetNarrator`. Single-resource report narration is
deliberately not propagated through the multi-report path; a
50-resource fleet report performs exactly one AI call (the fleet
narrator) rather than 51 (one per resource plus a fleet-level
summary).

Both the single-resource and fleet narrator system prompts also
encode an explicit detection-boundary invariant: Pulse Patrol is the
canonical detection layer, and the report narrators must function as
summarizers of Patrol's classified state rather than parallel
detectors. The narrator may classify an observation at "warning" or
"critical" severity only when it is backed by a Patrol finding, an
alert, or a hard-threshold breach in the structured input (cpu max
above 90, memory avg above 85, disk avg above 85, failed or
high-wear disks, storage pools at 90 percent or more). Patterns it
notices in the metric data without that backing may be mentioned at
"info" severity but must not be promoted. Recommendations follow
the same rule: no remediation for inferred issues that lack a
finding, alert, or threshold breach. This keeps the report
narrative honestly retrospective on Patrol's work and prevents
silent shadow-classification competing with Patrol's detection
rules.

The same reporting synthesis layer is now exposed to Pulse
Assistant as a first-class chat tool, `pulse_summarize`. The tool
wraps the engine's `NarrativeFor` and `FleetNarrativeFor` entry
points (single-resource and fleet modes selected by an `action`
parameter) so an operator can ask "what's been happening with
pve1 this week" or "where should I look across my fleet" and get
a structured retrospective answer in chat rather than having to
generate, download, and read a PDF. The tool is read-only (no
approval gate, no control-level requirement) and returns a JSON
envelope carrying the narrative source, health status, observations
or outliers, recommendations, and provenance disclaimer. v1 always
returns heuristic narrative; the AI narrator wiring through the
chat session is a focused follow-up that adds `Narrator`,
`FleetNarrator`, and `FindingsProvider` plumbing to the executor
configuration so the tool inherits the same per-tenant AI service
the report PDF endpoint already uses. Reporting therefore expands
from an export-shaped feature into a first-class capability
Assistant can compose with — the underlying engine surface stays
unchanged.

That follow-up has now landed. `chat.Config` carries three optional
fields (`ReportNarrator`, `ReportFleetNarrator`,
`ReportFindingsProvider`) which are threaded through to
`tools.ExecutorConfig` and stored on `PulseToolExecutor`. The
`pulse_summarize` tool reads them when building requests so the
engine sees a populated narrator when the tenant's AI service is
configured. The router installs a `SetReportNarratorResolver`
closure on the chat handler that mirrors the reporting handler's
pattern: it asks the AISettingsHandler for the per-tenant
`ai.Service` and, when that service has `Enabled=true`, returns it
as the implementation for all three roles (Service satisfies
`reporting.Narrator`, `reporting.FleetNarrator`, and
`reporting.FindingsProvider` already). An unconfigured tenant still
sees the heuristic fallback — the tool never errors on missing AI,
matching the report PDF's graceful-degradation posture. AI-narrated
chat synthesis therefore uses the same provider, sanitizer, model
selection, cost ledger (report_narrative / report_narrative_fleet
use-cases), and budget gate the report PDF endpoint already
enforces — there is exactly one canonical synthesis path for both
surfaces.

The same canonical AI runtime now also records user-chat token
usage to the cost ledger. `chat.Service.ExecuteStream` was a
long-standing gap: the agentic loop accumulated token counts via
stream callbacks and surfaced them in the SSE done envelope, but
nothing on the server side recorded a `cost.UsageEvent`. Chat is
the bulk of AI token spend, so the operator AI usage dashboard
was understating cost dramatically. `recordChatTurnCost` now runs
after every `loop.ExecuteWithTools` return — success or error,
since the operator was billed regardless of whether the loop
produced a clean response. It emits a `cost.UsageEvent` with
`UseCase="chat"` in the same shape the rest of the runtime uses.
The store is threaded through `chat.Config.CostStore`, wired by
the router from the per-tenant `AISettingsHandler.GetAIService`
via `Service.CostStore()`. `ExecutePatrolStream` deliberately
does NOT record here — its caller (`patrol_ai.go`) records via
its own helper, so cost is never double-counted on the
patrol-via-chat path.
