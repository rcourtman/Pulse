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
8. `frontend-modern/src/api/patrol.ts`
9. `frontend-modern/src/components/AI/AICostDashboard.tsx`
10. `frontend-modern/src/components/AI/Chat/`
11. `frontend-modern/src/utils/aiChatPresentation.ts`
12. `frontend-modern/src/utils/aiControlLevelPresentation.ts`
13. `frontend-modern/src/utils/aiCostPresentation.ts`
14. `frontend-modern/src/utils/aiExplorePresentation.ts`
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
2. Add or change canonical AI provider config, provider-scoped model selection, or runtime auth/base-URL defaults through `internal/config/ai.go`
3. Add or change Pulse Assistant request flow through `internal/api/ai_handler.go` and `frontend-modern/src/api/ai.ts`
4. Add or change Patrol, alert-analysis, or remediation transport through `internal/api/ai_handlers.go`, `internal/api/ai_intelligence_handlers.go`, and `frontend-modern/src/api/patrol.ts`
5. Add or change AI usage/cost dashboard presentation through `frontend-modern/src/components/AI/AICostDashboard.tsx` and `frontend-modern/src/utils/aiCostPresentation.ts`
6. Add or change AI provider, control-level, chat/session, or explore-state presentation through `frontend-modern/src/components/AI/Chat/`, `frontend-modern/src/utils/aiProviderPresentation.ts`, `frontend-modern/src/utils/aiProviderHealthPresentation.ts`, `frontend-modern/src/utils/aiControlLevelPresentation.ts`, `frontend-modern/src/utils/aiChatPresentation.ts`, `frontend-modern/src/utils/aiSessionDiffPresentation.ts`, and `frontend-modern/src/utils/aiExplorePresentation.ts`
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
   and meaningful badge text.
9. Add or change public AI overview wording through `docs/AI.md`; it may
   describe Assistant and Patrol capabilities, but it must not revive legacy
   commercial shorthand such as `incident memory` as a current product promise.

## Forbidden Paths

1. Leaving new `internal/ai/` runtime entry points unowned under broad architecture or generic API ownership
2. Duplicating AI orchestration, Patrol runtime, or cost-tracking logic outside `internal/ai/`
3. Treating AI transport files as payload-only boundaries when they also define live runtime control behavior

## Completion Obligations

1. Update this contract when canonical AI runtime or transport entry points move
2. Keep AI runtime and shared API proof routing aligned in `registry.json`
3. Preserve explicit coverage for chat, Patrol, remediation, and cost-control behavior when AI runtime changes
4. Keep discovery scheduling authoritative through `internal/config/ai.go`: `discovery_enabled` and `discovery_interval_hours` must govern both lightweight infrastructure discovery and deep service-discovery background loops
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
   prompt-only tool lists, and frontend approval cards must surface backend
   approval risk/description without hiding a pending approval when skip or
   deny fails. Action-producing tools must also persist the unified
   `ActionPlan.Preflight` dry-run boundary through
   `internal/ai/tools/action_audit.go` rather than leaving dry-run availability
   as chat-only text.
9. Keep self-hosted Patrol messaging aligned with the v6 GA product contract:
   ordinary self-hosted installs use BYOK or local providers, and the runtime
   must not surface retired managed-model credits, trial prompts, account-backed
   AI activation, or general hosted chat entitlement in the normal app.
   The shared app shell must also keep `/cloud` and `/cloud/signup` out of
   ordinary self-hosted public routes so Cloud acquisition cannot reappear as a
   proxy for retired hosted-model or AI quickstart activation.
   The public AI overview must likewise use productized context language such
   as alert history, Patrol runs, and resource timelines instead of presenting
   `incident memory` as a standalone feature.
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
    investigation context. When `/api/ai/chat` receives `finding_id`, the
    runtime must enrich the provider turn from that durable record while
    preserving the user's authored prompt as the persisted conversation
    message; the model-only handoff may persist as session metadata so
    same-session follow-up turns keep the Patrol finding context without
    mutating saved user messages. When the handoff identifies a resource, the
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
    lifecycle authority. If the referenced finding no longer resolves through
    the current unified finding store, Assistant must invalidate the stored
    model-only handoff and unpinned handoff-seeded resource scope instead of
    falling back to stale investigation context. The refreshed finding context
    must include unified finding lifecycle and recency facts such as active,
    resolved, snoozed, dismissed or suppressed state, detection/last-seen/
    resolved timestamps, recurrence, regression, and recent lifecycle events so
    Assistant explains the current Patrol record rather than only the original
    investigation narrative. Assistant runtime may also hydrate canonical
    resource-policy context for those handoff resources, using the same
    unified-resource resolution and policy presentation helpers that govern
    mention prefetch and provider-bound redaction; that context remains
    model-only handling guidance, not saved user text or disclosure authority.
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
    action authority. The runtime may also persist structured pending-action and
    approval references from the same investigation record as
    model-context metadata, but those references are review context only: they
    must not include raw command text, must not grant approval or execution
    authority, and must route any operator decision back through the governed
    approval/remediation flow. When
    those references include an approval ID, Assistant runtime may refresh a
    current status snapshot from the canonical approval store on each turn, but
    it must enforce org scoping and still omit the approval command payload.
    When those references resolve to a governed action plan or action audit,
    Assistant runtime must hydrate the canonical action ID, lifecycle state,
    requester, capability, approval policy, plan expiry, preflight/dry-run
    summary, and terminal success/failure state from the action-audit store
    rather than treating the original approval as the current action truth.
    That action snapshot remains model-only review context and must expose
    lifecycle status rather than raw execution output or command text.
    Proposed-fix command text must stay out of both the persisted chat message
    and the model-only handoff context, and command payloads remain
    approval-context data, not conversational copy.
    The Assistant drawer may also render an attached context briefing for that
    handoff, but the briefing is runtime context visibility only: it must not
    mutate chat control settings, execute tools, or reveal raw command payloads.

## Current State

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
selection just because live model discovery was unavailable.
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
through the canonical post-auth landing route instead of leaving the
assistant-capable authenticated shell stranded on a route that only exists for
logged-out presentation.
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
That same provider-model contract applies to the chat explore pre-pass in
`internal/ai/chat/service_explore.go`: any runtime model that is valid for the
main chat execution path must resolve through the dedicated explore provider
path as well. Retired quickstart model strings such as
`quickstart:pulse-hosted` must fail closed and route the operator back to
BYOK/local-provider setup instead of being accepted as managed-model runtime.

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
`frontend-modern/src/utils/aiSessionDiffPresentation.ts`, and
`frontend-modern/src/utils/aiExplorePresentation.ts` are the canonical owners
for provider naming, provider health labels, control-level semantics,
chat drawer title/subtitle, launcher title/aria copy, session-menu labeling,
discovery hint framing, chat/session empty states, assistant message and
question-card labels, session-diff badges, and explore-status labels.
Settings and chat surfaces must consume those helpers instead of keeping local
AI wording or model/provider inference branches.

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
That same approval boundary also owns approved command execution. When
`internal/api/ai_handlers.go`, `internal/ai/service.go`, or
`internal/ai/tools/action_audit.go` consume a governed approval record, the
runtime must carry that approval identifier into the final
`agentexec.ExecuteCommandPayload` so the host agent can re-check the shared
command policy locally and fail closed on blocked or still-unapproved commands
instead of treating control-plane approval as an implicit bypass.
The same action-audit boundary now also requires persisted action records to
carry a normalized plan and preflight: action id, request id, capability,
approval policy, dry-run availability, safety checks, verification steps, and
timestamps are normalized before persistence by the unified-resource store, so
runtime callers cannot publish an execution audit that skipped the canonical
planning contract.
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
surfaces can mutate state even when invoked in non-interactive forms.
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
Assistant finding handoffs now also receive a model-only operator briefing
derived from the current unified finding and structured Patrol investigation
record before the lower-level finding context. That briefing must summarize the
finding, resource, priority, investigation confidence, recommended next step,
and governed action posture as operator guidance, while leaving current
resource-state, timeline, and action-audit hydration in the existing canonical
AI runtime handoff builders. It is explanation and review context only, not
approval or execution authority.
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
Because those findings represent Patrol blindness rather than operator-triaged
infrastructure noise, the Patrol runtime must also reject manual acknowledge,
snooze, dismiss, resolve, and suppress actions against synthetic `ai-service`
runtime findings. The canonical recovery path is to correct AI/provider
configuration and let Patrol re-evaluate the runtime condition on the next run.
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
