# AI Runtime Contract

## Contract Metadata

```json
{
  "subsystem_id": "ai-runtime",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/ai-runtime.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts",
    "cloud-paid",
    "frontend-primitives"
  ]
}
```

## Purpose

Own the Pulse Intelligence Core: canonical context, governed actions, safety
gates, approval state, action audit, and verification. That core backs Pulse
Patrol as the primary built-in operator that checks infrastructure, investigates
issues, follows the chosen Patrol mode before acting, verifies outcomes, and
records what happened. Pulse Assistant is the contextual
explanation, approval, and handoff access path over that same work, while the
Pulse MCP adapter is the external-agent access path over canonical API
contracts. The subsystem also owns AI orchestration, runtime cost control,
shared AI transport surfaces, and browser-visible Assistant transcript actions
that define what visible operator/model text can leave the transcript without
exposing hidden provider/tool metadata.

Interactive Assistant invocation authority is fail-closed and request-local.
The `read_only` presentation means no model-invokable durable Pulse-state or
infrastructure mutation is projected or executable. An `ai:chat` token grants
conversation, read, and session access only; it cannot write knowledge, change
finding state, approve, or execute. Infrastructure mutation additionally
requires explicit `ai:execute` authority and a control level that permits it.
Provider projection and registry execution consume the same invocation-aware
policy. Detection may write only through its explicit finding report/resolve
allowlist; investigation is domain-read-only and may emit at most one
side-effect-free typed proposal whose correlation identity is server-authored.
Unknown profiles, actions, aliases, origins, scopes, and mutation
classifications fail closed.

Model/tool orchestration is bounded by construction rather than by request
timeout. The legacy `Service` streaming and non-streaming paths share one
ten-provider-call budget owned by `internal/ai/service_tool_loop.go`; reaching
that budget returns a typed terminal failure. A provider that repeats the same
policy-denied tool name and arguments terminates on the second provider call,
before the repeated call can execute. Denial-loop termination must not create
an approval, mint command authority, dispatch an agent message, or mutate
external state. The canonical chat `AgenticLoop` retains its separately
configurable bounded-turn contract for native Assistant and Patrol flows.

Discovery `run` currently persists read-derived evidence in the discovery
cache. That cache write is not an H1 finding/knowledge/infrastructure mutation
exception and remains a separately governed residual under
`lane-followup:agent-native-continuous-discovery-findings` until discovery's
canonical persistence boundary is resolved.
App-shell Patrol chrome may expose only content-free current-work pressure, such
as an open-work count on the stable `Patrol` tab, from Patrol-owned findings and
approval read models. It must not rename the destination, create a second
findings queue, expose finding or approval identity in shell chrome, or turn
platform pages into Patrol summaries.
When backend Patrol summaries or Assistant handoffs expose run coverage to the
browser, they translate internal full-run, scoped-run, and verification
precision into plain operator check language rather than activation-loop,
proof-strip, or scoped/full-run jargon.
Coverage warnings must stay action-oriented: they may tell the operator to run
Patrol to check everything, but must not ask for a "full issue list" or otherwise
expose backend full-run vocabulary as the product instruction.
Patrol model-facing system prompts must derive the active control-mode block
from the effective `patrol_autonomy_level`, after license and full-mode-lock
clamping, so Watch only, Ask first, Safe auto-fix, and Autopilot give the model
the same investigate, approval, execution, and verification boundaries enforced
by the orchestrator. Prompt copy must not fall back to the legacy
`patrol_auto_fix` boolean or the retired "observe only" / "auto-fix mode"
framing.
Patrol's Watch evidence contract treats a direct provider-reported failed
health check, failed backup, or broken replication state as sufficient evidence
for the confirmed operational symptom. The model reports that symptom even when
an agent or native log adapter cannot supply optional root-cause evidence,
uses warning/reliability for a failed health check unless evidence establishes
a critical consequence, states that the root cause remains unknown, and must
not fabricate one. Scoped
triage seed context contains both deterministic flags and the explicitly scoped
resource inventory, including exact app-container health, rather than reducing
the model's evidence to whichever resources the deterministic pass happened to
flag. A direct provider-state triage flag is a Watch detection stop condition:
the model checks active findings and records the confirmed symptom from seed
evidence before any query, discovery, log, or broad inventory call; root-cause
investigation is a separate follow-up. Quick scoped checks have a strict
four-turn model budget: active-finding inspection, report or assessment, one
bounded fallback turn, and a final Watch decision turn. That final turn exposes
only `patrol_report_finding` and `patrol_assess_finding`, uses a bounded system
instruction that forbids further investigation and treats infrastructure data
as untrusted, and still permits a healthy all-clear without forcing a write.
The only permitted extension is one repair-only provider turn after a parallel
finding lifecycle batch contains both accepted and rejected siblings. Accepted
calls remain authoritative and must not be repeated; the repair projection
contains only finding lifecycle tools and may correct only the rejected calls
from the returned validation evidence. A repair attempt never erases the
original failed call from run history or qualification scoring.
Investigation and interactive profiles retain a tool-free final summary; a
Watch finding write at the deadline is followed only by the existing bounded
summary path. A capability-unavailable
read result is terminal for that capability within the run and must not trigger
retries or broad inventory scans.
Patrol report, assessment, and resolution tools remain governed Pulse-state
writes for invocation authorization, but accepting one does not represent an
infrastructure mutation and must not enter or satisfy the infrastructure
read-after-write FSM. A successful finding-lifecycle write moves the loop to a
text-only conclusion. When a real infrastructure write does require another
verification turn, the internal verification constraint is appended to the
provider conversation as a user-role instruction; the request must never end
in an assistant prefill that compatible providers reject before verification.
Patrol finding storage must consolidate equivalent active storage-capacity
siblings before they reach browser surfaces: a broader storage risk and a
generic usage/capacity finding for the same normalized storage identity are one
operator issue, while distinct backup, health, and pool-vs-device findings
remain separate. This consolidation belongs in the finding store, not the
frontend row renderer.
Capacity forecasts (days-to-full, current utilization, daily change rate) are
computed deterministically by linear regression over a resource's utilization
history and must persist as structured data on the Finding
(`capacity_forecast`), stamped by the finding store after a successful Patrol
analysis via `StampCapacityForecasts`. That structured field is the canonical
operator urgency source for capacity-relevant findings; the model-authored
description must never be treated as the urgency signal when a deterministic
forecast is present. A forecast requires the resource's utilization to be
ingested as a time-series in metrics history; resources without recorded
history cannot produce a forecast and must report its absence honestly rather
than fabricate one, and stable-high (>=80%) pools with no clear fill trend must
carry the deterministic "no fill trend" reading so the model's speculation
cannot override the verified trend. The forecast must query utilization
history under the resource's metrics-target ID (the same key the monitoring
layer records under, exposed as `StoragePoolView.SourceID()`), not the
canonical resource ID; the two diverge for most storage sources, so querying
the canonical ID silently leaves the feature dormant. The forecast's own
`resourceID` stays canonical so it can still be stamped onto findings keyed by
canonical resource ID.
Persisted Patrol run analysis is a display-safe summary, not an unchecked raw
model transcript. Infrastructure status and actions-taken text in run history
must be reconciled from the accepted structured Patrol result: accepted
findings, resolved finding IDs, run errors, and user-visible finding counts.
Rejected, failed, or attempted tool calls must not appear as actions taken, and
healthy/no-finding runs must not claim that an unaccepted
`patrol_report_finding` action occurred. Backup/PBS seed context must preserve
the source identity behind backup observations, including PBS instance,
datastore, and namespace when those fields are available, so model-authored
key observations can name the affected backup source rather than saying only
"PBS".

The public Pulse Intelligence overview is a projection of those runtime
contracts, not a second Assistant tool inventory. It must point readers at the
registry-owned Assistant tool governance and the manifest-owned external-agent
capabilities instead of carrying hand-maintained tool tables.
It is also the public product-language contract for paid Patrol capabilities:
Pro positioning must describe hands-on Patrol modes, issue investigation,
governed fixes, verified outcomes, and history. Public docs and generated
commercial copy must not reintroduce proof-first activation-loop framing or the
retired `Patrol Control Levels`, `Patrol control`, and `Alert investigation`
labels as the visible paid story. API fields and route markers may retain
compatibility names such as `autonomy_level` only when the surrounding copy
plainly calls the operator-facing choice `Patrol mode`.
Its Core/Patrol/Assistant/MCP relationship block must be rendered from
`PulseIntelligenceOverviewMarkdown(Manifest.SurfaceContract)` rather than
maintained as prose separate from the agent capabilities manifest.
Reusable workflow starters are manifest-owned `workflowPrompts` metadata
derived from the shared Pulse Intelligence workflow-prompt core. Prompt names,
Assistant display labels, presentation kind hints, descriptions, arguments,
availability, and rendered text belong to that shared catalogue. Pulse
Assistant-compatible starters, frontend manifest clients, MCP `prompts/list`,
and generated docs consume that catalogue instead of maintaining per-surface
prompt lists or browser-local starter copy.
The shared catalogue includes the global `pulse_operations_loop` workflow
starter for the Patrol mode operations loop: it is projected only when the
manifest has fleet context, resource context, findings, governed
plan/decide/execute action, and finding resolution capabilities. Its rendered
prompt must keep the operator in an approve-or-reject loop with execution only
after policy allows it and verification before finding resolution, while the
status schema treats Patrol mode starter evidence only as entry-point
orientation, Patrol mode completed-loop evidence as aggregate terminal
approve/reject proof, and Patrol mode resolved-loop evidence only as
aggregate approved-and-verified outcome proof, not Assistant transcript, Patrol
finding identity, approval payload, action command, or MCP setup detail.
`pulse_patrol`, the current paid `patrol_control` marker, the legacy
`patrol_autonomy` marker, and the legacy `pulse_pro_activation` alias all
contribute to first-party Patrol mode starter evidence;
`proActivationOperationsLoopStarterCount` is a compatibility field retained
for older external-agent clients, while primary clients must read
`patrolControlOperationsLoopStarterCount`. The other `proActivation*` status
fields are compatibility aliases for the same first-party Patrol mode
journey rather than a separate AI-runtime loop, and manifest descriptions must
not describe them as a Pro activation journey.
The same status schema must expose the aggregate Patrol mode value state as
a content-safe enum so Assistant-compatible and MCP-facing agents can branch
without reverse-engineering counts. `governed_decision_recorded` is a safe
terminal decision state, not proven operations value; `verified` is the only
completed first-party value state and requires the approved, verified outcome
plus recorded action history. MCP/external-agent readiness remains optional
external-agent setup context.
Native Assistant workflow starters in `frontend-modern/src/components/AI/Chat/`
are a browser projection of that same catalogue, not a second prompt registry:
they may decide which manifest-owned prompt is contextually usable, but display
labels and rendered prompt text must come from shared manifest metadata and the
shared backend renderer on
`POST /api/ai/workflow-prompts/render`.
AI provider integration is registry-owned. Provider identity, display names,
protocol family, default model, default base URL, credential fields,
configured-state fields, clear-key fields, env-var hints, docs links, and
fallback catalog models belong in the config-layer provider registry, not in
factory branches, settings-card literals, or per-provider model-list code. New
direct chat-compatible providers extend that registry and the shared
chat-compatible provider client; native transports remain explicit only where
the protocol is not chat-compatible.
Provider definitions that declare a BaseURLField (today OpenAI, Ollama, and
Z.ai) expose a user-overridable endpoint via the AI settings payload; the
registry default base URL applies when no override is stored, so one provider
can serve both standard and alternate (e.g. Z.ai coding) endpoint tiers.
Patrol-blessed quickstart models are registry-owned the same way. For a
provider whose model catalog is user-supplied (today Ollama), the suggested
Patrol model, its hardware-expectation note, and its equivalent catalog tags
live on the provider definition in `internal/config/ai_providers.go` and
project through `/api/settings/ai`; settings surfaces render that projection
and must not hardcode blessed model IDs. A blessing must be verified against
Patrol's real tool-call preflight before it ships or changes; the env-gated
`TestManualOllamaBlessedModelPreflight` harness in `internal/ai/` is the
re-blessing procedure, and connectivity or catalog presence alone is not
verification (qwen3:4b passed connectivity yet emitted a tool call in 0 of 4
preflight runs, which is why only qwen3:8b is blessed). Recommended-model
resolution may prefer blessed models only by exact normalized catalog ID
match, never by family or substring, so cloud-gateway catalog neighbours of
the same model family are not promoted by fuzzy matching.
Native provider tool-calling is a transport projection over the shared
provider-neutral `Tool`, `ToolChoice`, `ToolCall`, and `ToolResult` shapes.
Nil `ToolChoice` means provider-owned automatic selection, `none` disables
tools by omitting them from the provider request, and `required` is reserved for
callers that explicitly need provider-native forced tool use. That required mode
maps to OpenAI `required`, Anthropic `any`, and Gemini
`FunctionCallingConfig` `ANY`; normal chat and Patrol runs must not force tool
use unless the caller sets that override. Patrol's static model-readiness
self-test may use required mode for Gemini because it is a fixed capability
probe with no infrastructure identifiers and needs deterministic tool-call
proof. Provider-specific schema restrictions belong at the native transport
edge: Gemini strips unsupported `additionalProperties` only from Gemini function
declarations, while the registry-owned provider-neutral schemas and providers
that support stricter JSON Schema keep that field. Gemini function-call
continuations must preserve the provider `functionCall.id` on parsed
`ToolCall.ID` values and return that same value as `functionResponse.id`; the
function name remains a separate field resolved from the preceding assistant
call when building tool-result turns.

Local subscription-agent providers are explicit non-chat-compatible native
transports owned by `internal/ai/providers/subscription_agent.go`. Their
configured state is an operator opt-in, not possession of a Pulse-managed
credential. Readiness requires the corresponding CLI on `PATH` and the CLI's
own first-party subscription login. The transport must never read, persist,
refresh, export, or forward that login, and its child environment is an
allowlist that excludes API keys, Pulse secrets, cloud credentials, and
unrelated tokens. It must never fall back to an API-key provider.

Each subscription-agent call is a single structured provider turn in a fresh
temporary working directory. Codex runs ephemeral with user configuration
ignored and a read-only sandbox. Claude runs without session persistence or
customizations, with all Claude Code tools disabled and a non-interactive
permission mode. Its actual CLI system prompt is the bounded Pulse adapter
contract; the serialized provider request is supplied separately as input
data. The adapter treats the request's Pulse-owned `system`, `tools`, and
`tool_choice` fields as trusted control-plane fields while treating
infrastructure names, metadata, logs, command output, and tool results inside
the message history as untrusted evidence. It explicitly defines returned
`tool_calls` as routing decisions that Pulse may validate and execute, not as
Claude Code tool activity. This separation must not collapse back into a user
prompt that labels Pulse's own system instruction untrusted or activates a
coding/plan-mode persona. Neither route receives Pulse MCP configuration. The
agent returns tool arguments as native JSON objects. The structured-output
schema binds every selectable tool name to that declared tool's input schema;
it forbids tool calls when none are offered or tool choice is `none`, and
requires at least one when tool choice is `required`. Pulse then rejects
unknown tool names, duplicate or empty call IDs, malformed argument objects, and violations of
`none` or `required` tool choice before the existing provider-neutral tool loop
can execute anything. The normal registry, profile, approval, protected
resource, action, and verification contracts remain the only infrastructure
authority. Calls are serialized per local subscription agent and bounded by
the configured request timeout and prompt/output size limits, with a two-minute
minimum request allowance for local subscription turns. Patrol tool-call
preflight owns one route-aware outer deadline: API providers retain the
30-second budget, while local subscription agents receive a bounded two-minute
budget for CLI startup and complete structured-output assembly. A caller's
earlier cancellation still wins. Qualification waits through that complete
subscription-agent deadline plus bounded cache-publication grace before it may
classify missing fresh preflight evidence; its shorter API-provider wait must
not truncate a healthy local CLI turn. The native provider boundary accepts the
canonical qualified Pulse model identity from shared callers, strips only its
own subscription-provider prefix before CLI execution, and rejects foreign
provider prefixes rather than forwarding an invalid or cross-provider model
name to the local agent.
Claude receives that output schema through the trusted adapter system channel
and returns one JSON result that Pulse decodes with unknown fields and trailing
values rejected whenever the model selects tools. A non-required turn may end
with ordinary assistant prose instead; Pulse accepts that as a no-tool
completion only after rejecting provider tool-call artifacts against the
offered tool catalogue. Required-tool turns still fail when no structured call
is returned. Pulse does not use Claude Code's hidden `--json-schema`
retry loop: live qualification proved that wrapper could exhaust retries after
the model had already completed valid finding calls, incorrectly converting a
durable Patrol outcome into a provider failure. Codex may continue to use its
native output-schema file because its CLI exposes the completed turn directly.
Patrol consumes the provider streaming interface, so the adapter projects each
fully validated CLI turn into canonical buffered `content`, `tool_start`, and
`done` events. It must emit nothing before the complete CLI response passes the
schema and tool-boundary checks. This is transport compatibility rather than a
claim of token-by-token CLI streaming; Pulse still owns tool execution,
`tool_end` progress, run-history persistence, and verification.
Codex JSONL event output is also a fail-closed audit channel: reported command,
file, MCP, web, computer, or image-tool activity invalidates the turn. This
detects a violated adapter instruction but cannot retroactively protect data a
same-user read-only process already read, so deployment guidance must require a
dedicated least-privilege Pulse OS account and must not describe the local
agent route as equivalent isolation to a remote chat-completions API.

`codex-subscription:*` and `claude-subscription:*` are distinct provider routes
in settings, readiness, model catalogues, usage, and qualification reports.
Qualification may temporarily opt in the selected route when applying an
explicit model override, must restore the previous opt-in and Patrol model even
after failure, and records `inference_route=local_subscription_agent` so a
subscription run is never compared or published as a metered API route.
Qualification cost evidence must preserve that route distinction. Unknown
pricing on `metered_api` remains a fail-closed dollar-budget failure. A
`local_subscription_agent` or `local_model_server` run instead records its
monetary cost as unknown with `budget_applicable=false`; it must not invent a
zero-dollar price, and latency, provider/plan errors, tool efficiency, and any
usage counts exposed by the transport remain scored and reportable.
The configured Z.ai `/api/coding/paas/` endpoint is likewise reported as
`coding_plan_allowance`, with unknown monetary cost and no per-run metered-API
dollar budget; the standard `/api/paas/` endpoint remains `metered_api`.

The provider-neutral agentic loop runs independent tool calls in parallel, but
parallelism must not erase a same-turn Patrol lifecycle dependency. When a
provider returns `patrol_get_findings` together with
`patrol_report_finding`, `patrol_assess_finding`, or
`patrol_resolve_finding`, Pulse executes that batch sequentially in the
provider's original order. It does not reorder an invalid write-before-read
batch, and it retains parallel execution for independent reads and independent
finding writes. This makes the existing read-before-write guard deterministic
without requiring the model to understand an internal goroutine schedule.

## Canonical Files

1. `internal/ai/`
   1a. `cmd/pulse-mcp/main.go`
   1b. `cmd/pulse-mcp/README.md`
   1c. `internal/agentcapabilities/`
   1d. `scripts/generate-pulse-intelligence-docs.go`
2. `internal/config/ai.go`
   2a. `internal/config/ai_providers.go`
3. `internal/api/ai_handler.go`
4. `internal/api/ai_handlers.go`
5. `internal/api/ai_hosted_runtime.go`
6. `internal/api/ai_intelligence_handlers.go`
7. `frontend-modern/src/api/agentCapabilities.ts`
   7a. `frontend-modern/src/api/generated/agentCapabilities.ts`
8. `frontend-modern/src/api/ai.ts`
   8a. `frontend-modern/src/api/aiChat.ts`
9. `frontend-modern/src/api/patrol.ts`
10. `frontend-modern/src/components/AI/AICostDashboard.tsx`
11. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx`
12. `frontend-modern/src/components/AI/Chat/`
13. `frontend-modern/src/utils/aiChatPresentation.ts`
14. `frontend-modern/src/utils/assistantPageContext.ts`
15. `frontend-modern/src/utils/aiControlLevelPresentation.ts`
16. `frontend-modern/src/utils/aiCostPresentation.ts`
17. `frontend-modern/src/utils/aiProviderHealthPresentation.ts`
18. `frontend-modern/src/utils/aiProviderPresentation.ts`
19. `frontend-modern/src/utils/textPresentation.ts`
20. `frontend-modern/src/stores/aiRuntimeState.ts`
21. `frontend-modern/src/stores/aiChat.ts`
22. `docs/AI.md`
23. `pkg/aicontracts/investigation.go`
24. `pkg/aicontracts/orchestrator_deps.go`
25. `pkg/aicontracts/fix_execution.go`
26. `pkg/extensions/ai_autofix.go`
27. `internal/ai/qualification/`
28. `cmd/patrol-qualify/`
29. `tests/qualification/patrol/`
30. `docs/AI_PATROL_QUALIFICATION.md`

## Shared Boundaries

1. `cmd/pulse-mcp/main.go` shared with `api-contracts`: the Pulse MCP adapter runtime is both an AI runtime surface for external-agent access to Pulse Intelligence and a canonical API contract projection over the agent capabilities manifest and Pulse MCP surface tool contract.
2. `cmd/pulse-mcp/README.md` shared with `api-contracts`: the Pulse MCP adapter guide is both an AI runtime surface for external-agent access to Pulse Intelligence and a canonical API contract projection over the agent capabilities manifest.
3. `frontend-modern/src/api/agentCapabilities.ts` shared with `api-contracts`: the agent capabilities frontend client is both the Pulse Intelligence external-agent manifest consumer and a canonical API payload contract boundary.
   Its presentation sanitizer may translate legacy Pro activation manifest
   descriptions into Patrol mode outcome/status language, but it must not
   reintroduce proof-first or activation-loop copy into Assistant, Patrol, or
   external-agent onboarding surfaces.
4. `frontend-modern/src/api/ai.ts` shared with `api-contracts`: the AI frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
5. `frontend-modern/src/api/aiChat.ts` shared with `api-contracts`: the Assistant chat frontend client is both the first-party Assistant transport surface and a canonical API payload contract boundary.
6. `frontend-modern/src/api/generated/agentCapabilities.ts` shared with `api-contracts`: the generated agent capabilities frontend types are both the Pulse Intelligence manifest TypeScript projection and a canonical API payload contract boundary.
7. `frontend-modern/src/api/patrol.ts` shared with `api-contracts`: the Patrol frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
8. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx` shared with `api-contracts`, `frontend-primitives`: the External agents settings panel is the optional settings-shell projection of Pulse MCP onboarding, the AI runtime connected-agent onboarding surface, and a presentation consumer of the shared agent capabilities frontend client.
   Its default copy must frame connected clients as optional connector access
   to Pulse context and Patrol work, with Patrol as the operator that watches,
   acts within Patrol mode, asks for approval when required, verifies outcomes,
   and records history. The normal Assistant settings view must keep setup
   mechanics behind a `Show connector setup` disclosure, with direct setup
   links opening that disclosure automatically. Default copy must make clear
   that connected tools do not get separate powers. Tool-contract posture and manifest facts are
   client-builder diagnostics and must stay behind a Developer details
   disclosure rather than beside the normal external-agent setup steps; posture
   and policy context belongs under Patrol access model, while prompt, scope,
   failure-code, and tool inventories must sit one level deeper behind Live
   manifest details using user-facing labels such as Agent starting points and
   Agent capabilities.
   Direct links to
   `/settings/pulse-intelligence/assistant#external-agent-setup` must focus this
   External agents panel after the Assistant settings layout settles,
   because connected agents are optional access to Patrol work and should not
   strand users at the generic token inventory. Legacy
   `/settings/security/api#external-agent-setup` and
   `/settings/security/api#pulse-mcp-setup` links must remain accepted and
   redirect to the canonical Pulse Intelligence Assistant route.
   The `Choose Patrol mode` handoff must target the Patrol operator surface at
   `/patrol#patrol-control`, where the mode is configured, while API
   Access remains the token and external-client setup surface. The expanded
   setup checklist owns that handoff as Step 1; the panel header must not repeat
   a second visible `Choose Patrol mode` action while setup is open.
   The visible full-surface token preset for that setup is `Patrol external agent`;
   `pulse_intelligence_agent` remains only the compatibility preset id used by
   the route and token model.
9. `frontend-modern/src/stores/aiChat.ts` shared with `frontend-primitives`: the assistant drawer and session store is both an AI runtime control surface and a canonical app-shell presentation boundary.
10. `internal/agentcapabilities/action_target.go` shared with `api-contracts`: the Pulse Intelligence governed action target type and resource-to-action-target mapping vocabulary are both the Assistant approval/runtime routing contract and the canonical API/agent target contract for governed actions.
11. `internal/agentcapabilities/control_level.go` shared with `api-contracts`: the Pulse Intelligence control-level vocabulary and control-tool availability predicate are both the Assistant runtime gating contract and the canonical API/agent permission posture for governed action tools.
12. `internal/agentcapabilities/errors.go` shared with `api-contracts`: the Pulse Intelligence agent error envelope is both the canonical API failure payload contract and the AI runtime adapter error-parsing contract for Assistant and external-agent surfaces.
13. `internal/agentcapabilities/events.go` shared with `api-contracts`: the Pulse Intelligence event vocabulary is both the canonical API SSE event contract and the AI runtime adapter notification contract for Assistant and external-agent surfaces.
14. `internal/agentcapabilities/governance_prompt.go` shared with `api-contracts`: the Pulse Intelligence surface-affordance-resolved model-facing operating-instruction, tool-governance prompt, reusable provider-tool governance description, Assistant-native offered-tool filtering, and Assistant-native interactive question-tool governance projections are both the Assistant system-prompt governance section and the shared API/agent vocabulary for action mode, approval posture, MCP affordance advertisement, and non-registry interaction-tool boundaries.
15. `internal/agentcapabilities/http.go` shared with `api-contracts`: the Pulse Intelligence agent HTTP substrate is both the API capabilities invocation contract and the shared AI runtime adapter execution primitive for MCP and reference agent clients.
16. `internal/agentcapabilities/invocation.go` shared with `api-contracts`: the canonical registry-owned invocation descriptors (per-tool discriminator, enum-exact case coverage, closed workflow-kind and mutation-target vocabularies, deep-copied lookups, fail-closed classification with unknown targets denied at policy evaluation) are both the native Assistant/FSM safety-classification contract and the canonical API/agent governed-invocation policy contract consumed by provider projection and registry runtime enforcement; canonical tool names cannot carry descriptor overrides.
17. `internal/agentcapabilities/manifest.go` shared with `api-contracts`: the canonical Pulse Intelligence agent capabilities manifest declaration, including capability display titles, manifest-owned finding lifecycle schemas, manifest-owned governed action schemas and routes, manifest-owned external-adapter surface tool contracts, and manifest-owned structured output schemas, is both the API discovery payload source and the AI runtime projection contract for Pulse Assistant and MCP-facing agent tools.
18. `internal/agentcapabilities/markdown.go` shared with `api-contracts`: the Pulse Intelligence manifest Markdown projection, including manifest-owned capability titles, surface-filtered Pulse MCP tool/error inventories, and prompt labels, is both the canonical API/agent documentation projection and the AI runtime onboarding projection for Assistant-compatible external-agent surfaces.
19. `internal/agentcapabilities/mcp.go` shared with `api-contracts`: the Pulse Intelligence MCP protocol version, JSON-RPC, method dispatch, method payload, surface-tool-contract-gated initialize operating-instruction and capability advertisement payload, manifest surface-filtered tools/list and tools/call execution bridge, manifest surface-gated resources/list and resources/read bridge, manifest-owned and surface-affordance-gated workflow prompt projection, protocol wire aliases, resource and prompt handler gates, and notification projection collectively define the external-agent adapter wire contract over the shared Pulse Intelligence tool core; MCP initialize, tools/call execution, resource list/read projection, and prompt list/get projection must enter through manifest-owned surface and workflow-prompt contracts so raw capability slices cannot bypass the published external-adapter contract.
20. `internal/agentcapabilities/mcp_adapter.go` shared with `api-contracts`: the Pulse MCP adapter setup contract defaults and normalization are both the canonical API manifest setup projection and the AI runtime onboarding contract for Assistant-compatible external-agent surfaces.
21. `internal/agentcapabilities/projection.go` shared with `api-contracts`: the agent capability external-tool projection helper, normalized manifest-owned surface tool contract resolution and tools-affordance gating, manifest-owned resource-context route and argument vocabulary, operator-state capability and route vocabulary, finding workflow capability and lifecycle argument vocabulary including resolution and dismissal notes, governed action capability, route, and argument vocabulary, manifest-owned tool title and outputSchema projection, structured Pulse capability _meta, and shared tool behavior hints are both the canonical API manifest projection contract and the AI runtime adapter projection for Pulse Assistant and MCP-facing agent tools, with MCP annotation and metadata wire names confined to adapter-edge aliases.
22. `internal/agentcapabilities/provider_tool_artifacts.go` shared with `api-contracts`: the provider tool-call artifact detector and streaming tool-name prefix splitter are both the Assistant stream-sanitization boundary and the shared external-adapter leak guard for provider-native tool-call markup that escaped the structured channel.
23. `internal/agentcapabilities/schema.go` shared with `api-contracts`: the agent capability input schema contract is both the canonical API manifest schema envelope and the AI runtime structured tool-schema, governance-aware provider-projection with neutral behavior hints and Pulse governance metadata, offered-tool governance extraction for Assistant prompt policy, manifest-affordance-gated Assistant provider-surface composition, manifest raw-schema to Assistant provider-schema projection for capability tools, legacy native Assistant utility provider aliases and schemas, provider-call normalization, provider-result context projection, Assistant-native interaction provider-tool declaration, and live Assistant execution-normalization contract for Pulse Assistant and MCP-facing agent tools.
    Provider projection is closed by default (`additionalProperties:false`), provider-emitted internal approval metadata is rejected before registry execution, and the registry rejects undeclared public arguments before a handler runs. Interactive approved replays may carry only the non-serializable server-attached approval field. Public Assistant chat always installs an approval-gated request posture; caller JSON cannot promote the service to autonomous infrastructure authority, and autonomous model sessions cannot enter either the canonical or legacy raw-command executor.
24. `internal/agentcapabilities/scopes.go` shared with `api-contracts`: the manifest-derived required-scope summary is both the canonical API/agent token guidance contract and the AI runtime adapter startup/onboarding contract for Assistant-compatible external-agent surfaces.
25. `internal/agentcapabilities/sse.go` shared with `api-contracts`: the Pulse Intelligence SSE subscription transport and record parser are both the canonical API event-stream consumption contract and the AI runtime adapter push bridge contract for MCP and reference agent clients.
26. `internal/agentcapabilities/surface_contract.go` shared with `api-contracts`: the Pulse Intelligence operator-surface affordance contract, shared surface-affordance, surface-tool identity, Assistant surface tool filtering, normalized external surface tool resolver, surface lookup, affordance labels, and manifest-published external-adapter surface tool allowlist projection are both the canonical API manifest surface model and the AI runtime prompt and onboarding guardrail for Assistant and MCP-facing surfaces.
27. `internal/agentcapabilities/text_tool_invocation.go` shared with `api-contracts`: the Pulse Intelligence text tool invocation parser, internal approval argument, and current_resource handle vocabulary are both the Assistant approved-action execution projection and the shared tool-call params bridge for governed Pulse Intelligence tool calls, with MCP tools/call compatibility staying at the adapter edge.
28. `internal/agentcapabilities/tool_call.go` shared with `api-contracts`: the Pulse Intelligence shared tool-call params, normalization, validation, direct registry preparation, registry-entrypoint failure result helpers, and provider/registry tool-call safety classification are both the native Assistant execution/FSM contract and the canonical API/agent tools/call compatibility contract for governed Pulse Intelligence tool calls; the shared invocation-blocked result is the stable refusal for every profile-denied mutation (pulse-state and infrastructure alike).
29. `internal/agentcapabilities/tool_execution.go` shared with `api-contracts`: the Pulse Intelligence neutral capability tool HTTP execution helper and direct tool execution output/error mapper are both the Assistant-native direct execution contract and the canonical API/agent request/response execution contract, with MCP adapters consuming the neutral helpers only after the shared MCP manifest-surface execution bridge has applied the published surface tool contract.
30. `internal/agentcapabilities/tool_marker.go` shared with `api-contracts`: the Pulse Intelligence Assistant tool marker vocabulary and approval/policy marker parser are both the Assistant structured tool-result compatibility contract and the canonical API/agent branching contract for governed tool outcomes.
31. `internal/agentcapabilities/tool_names.go` shared with `api-contracts`: the Pulse Intelligence registry tool-name vocabulary is both the native Assistant execution/display contract and the canonical API/agent tool identity contract for MCP-facing external-agent adapters.
32. `internal/agentcapabilities/tool_response.go` shared with `api-contracts`: the shared tool response envelope, tool error-code vocabulary, and tool-result error-code and verification evidence parsers are both the Assistant structured tool-result contract and the canonical API/agent branching contract for Pulse Intelligence tool failures, recovery tracking, and write self-verification.
33. `internal/agentcapabilities/tool_result.go` shared with `api-contracts`: the Pulse Intelligence shared tool-result content/result envelope, structuredContent projection, result constructors, HTTP response-to-result mapping, text projection, and result interpretation helpers are both the Assistant registry result contract and the canonical API/agent result projection contract for governed tool outcomes.
34. `internal/agentcapabilities/types.go` shared with `api-contracts`: the agent capabilities manifest wire type, manifest-owned external-adapter surface tool contract field, capability display title and structured output schema fields, approval-policy vocabulary, capability governance normalization, and tool-governance descriptor shape are both the canonical API payload contract and the AI runtime projection contract for Pulse Assistant and MCP-facing agent tools.
35. `internal/agentcapabilities/workflow_prompt.go` shared with `api-contracts`: the Pulse Intelligence workflow prompt catalogue, manifest-owned `workflowPrompts` projection, MCP prompt title projection, presentation kind hints, shared resource-context and finding argument vocabulary, Patrol issue-handling capability gating, argument validation, and manifest-gated shared prompt rendering rules are both the AI runtime starter contract for Assistant-compatible surfaces and the canonical API/agent prompt projection contract for MCP-facing clients.
36. `internal/api/ai_handler.go` shared with `api-contracts`: Pulse Assistant handlers are both an AI runtime control surface and a canonical API payload contract boundary.
37. `internal/api/ai_handlers.go` shared with `api-contracts`: AI settings and remediation handlers are both an AI runtime control surface and a canonical API payload contract boundary.
38. `internal/api/ai_intelligence_handlers.go` shared with `api-contracts`: AI intelligence handlers are both an AI runtime control surface and a canonical API payload contract boundary.
39. `pkg/aicontracts/action_broker.go` shared with `api-contracts`: the public typed action-proposal broker contract is both an AI runtime proposal boundary (the only sanctioned Patrol route to an infrastructure mutation) and a canonical API dependency contract over the shared action lifecycle service.
40. `pkg/aicontracts/fix_execution.go` shared with `api-contracts`: the public approved-fix execution contract is both an AI runtime approved-action boundary and a canonical API dependency contract for Patrol and enterprise auto-fix binders.
41. `pkg/aicontracts/investigation.go` shared with `api-contracts`: the public Patrol investigation record and finding contract is both an AI runtime handoff boundary and a canonical API payload contract for Patrol, Assistant, unified findings, persistence, and audit surfaces.
42. `pkg/aicontracts/orchestrator_deps.go` shared with `api-contracts`: the public investigation orchestrator dependency contract is both an AI runtime handoff boundary and a canonical API payload contract for Assistant and Patrol tool-call history.
43. `pkg/extensions/ai_autofix.go` shared with `api-contracts`: the enterprise auto-fix extension dependency seam is both an AI runtime approved-action boundary and a canonical API extension contract over Assistant and Patrol execution dependencies.
44. `scripts/generate-pulse-intelligence-docs.go` shared with `api-contracts`: the Pulse Intelligence manifest docs generator is both an AI runtime docs/onboarding projection and a canonical API contract projection over the agent capabilities manifest and Pulse MCP surface tool contract.

The shared agent-capabilities manifest also owns the runtime surface contract:
the manifest wire type and generated frontend projection must name Pulse
Intelligence Core as the shared core, Pulse Patrol as the primary built-in
operator, and Pulse Assistant plus Pulse MCP as contextual and external-agent
access paths over the same governed capabilities. Frontend copy, MCP docs, and
Assistant onboarding must consume that relationship from the manifest rather
than carrying parallel surface tables.
The same manifest owns Pulse MCP adapter setup facts: server name, command,
base URL flag/default, token environment variable, and supported client config
families. Settings, README generation, and adapter onboarding may present those
facts for operators, but they must not maintain separate MCP setup constants.
The in-app setup surface for those facts belongs under Pulse Intelligence
settings because Pulse MCP is an external-agent access path over governed
Patrol actions. API Access may be linked for scoped-token creation, but it must
not host the external-agent setup walkthrough.
Pulse Intelligence telemetry readiness for the external-agent surface follows
that same manifest surface boundary: a non-expired API token covering any
Pulse MCP-published capability scope is enough to mark MCP/external-agent
readiness, while route-level usage still comes only from authenticated
capability activity markers. Readiness must not be tied to the full manifest
scope set because least-privilege external-agent setups are supported.
The `pulse-mcp` adapter marks its own capability requests with the content-free
`X-Pulse-Agent-Surface: pulse_mcp` header so telemetry can distinguish adapter
use from direct external-agent API use without recording prompts, payloads,
route parameters, token identity, or resource identifiers.
Successful workflow starter rendering follows the same content-free boundary:
native Assistant render calls, first-party paid Patrol autonomy handoffs, and
Pulse MCP `prompts/get` calls may persist only the manifest prompt name, coarse
surface (`pulse_assistant`, `pulse_patrol`, `patrol_autonomy`, legacy
`pulse_pro_activation`, or `pulse_mcp`), and timestamp. The first-party
activation marker is `POST /api/ai/workflow-prompts/activity`; it is an
authenticated `ai:chat` route for app-owned surfaces such as the paid Patrol
autonomy handoff into Patrol, validates prompt names against the manifest
workflow prompt catalogue, accepts only first-party surfaces, and must remain a
starter marker rather than proof that Assistant context, governed action
approval, execution, verification, or finding resolution occurred.
`pulse_pro_activation` exists only as a legacy alias for older Pro
success/current-plan entry points; current paid handoffs use `patrol_autonomy`
and neither surface adds a new prompt, tool, model surface, or operations-loop
completion signal. Prompt arguments, rendered text, resource
IDs, finding IDs, session IDs, token identity, request bodies, and model output
must not enter workflow starter activity history or outbound telemetry.
Runtime model-facing instructions must consume that same contract as data.
`BuildPulseAssistantOperatingInstructions` identifies Pulse Assistant as the
native in-app surface over Pulse Intelligence Core, while manifest-backed MCP
initialize responses identify Pulse MCP as the external-agent adapter over the
same core and Pulse Patrol as the primary built-in operator. That copy belongs in
`internal/agentcapabilities`; Assistant prompts, MCP initialize payloads, and
future agent surfaces must not re-create local Assistant/MCP/Patrol relationship
wording outside the manifest-owned surface contract.
Surface tool summaries follow the same shared-core rule. The native Assistant
runtime exposes provider tools through `ProjectPulseAssistantProviderTools`,
which applies the manifest-owned Pulse Assistant surface affordances before
registry or native interaction tools reach model requests, and exposes
`PulseToolExecutor.AssistantSurfaceToolContract`, which projects the same
runtime-available provider tools through `ProjectPulseAssistantSurfaceToolContract`;
external-agent adapters project request/response manifest capabilities through
`ProjectManifestSurfaceToolContract` and publish the static external-adapter
inventory on `Manifest.SurfaceToolContracts`. Missing external-adapter
`surfaceToolContracts` entries fail closed rather than inferring tools from raw
manifest capabilities; static surface-tool affordance metadata may narrow an
external operator surface, but must not re-enable a disabled surface affordance
or keep tool/capability names when the effective `tools` affordance is false.
Disabled Assistant affordances likewise fail closed rather than allowing runtime
registry availability to re-enable provider tools.
Assistant prompt fallback governance follows that same contract: when no live
executor-owned offered-tool manifest is available, registry-owned fallback
governance must enter through
`CanonicalToolGovernanceForManifestSurface(ControlLevelControlled, CanonicalManifest(), SurfaceIDPulseAssistant)`,
and the chat prompt fallback must derive explicit offered tool names from
`ManifestSurfaceAffordances` so registry tools and the native `pulse_question`
interaction tool cannot be reintroduced by nil offered-tool semantics.
Assistant registry tools, Assistant-native interaction tools, and MCP manifest
capabilities must stay explicit buckets in that contract so Pulse can support
both surfaces without duplicating tool logic or
implying MCP replaces the in-app Assistant.
Frontend consumers must import the generated `SurfaceToolContract` shape from
the shared agent-capabilities client rather than maintaining a local copy.
Patrol work contract availability for browser surfaces is also a shared
agent-capabilities-client verdict: consumers must derive the Pulse MCP contract
from the normalized MCP adapter setup, the manifest-owned
`pulse_operations_loop` workflow prompt, and the Pulse MCP surface tool contract
through `frontend-modern/src/api/agentCapabilities.ts`; feature shells must not
hard-code MCP contract availability or infer it from raw manifest capabilities.
Runtime external-agent readiness is stricter:
`GET /api/agent/patrol-control/status` may mark `externalAgentReady` only when
that contract is available and a single non-expired API token covers every
scope required by the published Pulse MCP Patrol work capability set.
`GET /api/agent/operations-loop/status` remains only a compatibility alias for
older clients. That shared Patrol work tool set starts with
`get_patrol_control_status`, the count-only
`GET /api/agent/patrol-control/status` orientation read, then uses the same
fleet-context, resource-context, finding, governed action, and finding
resolution tools as the rest of the governed issue-handling flow. The
structured `list_resource_capabilities` read is the companion to
resource-context for action planning: it returns the per-resource
governed capability names and parameter schemas an agent needs to
populate `plan_action` inputs without guessing, where resource-context
renders capabilities only as count-limited prose facts.
`get_operations_loop_status` and `/api/agent/operations-loop/status` are
compatibility aliases, not primary capability names. AI runtime prompts, MCP
readiness helpers, and frontend presentation helpers must not carry separate
Patrol work inventories. The native Patrol work surface may display that same
status projection, but it must fetch it through the shared agent-capabilities
frontend client so Assistant, MCP, and native UI stay on the same Patrol work
contract. The manifest-owned status schema must describe governed counts as
decision-backed, split approved and rejected decision counts explicitly, and
describe verified counts as approved-action-backed. The same schema now exposes
content-free Patrol work starter counts for the total flow and the
Assistant, Patrol mode, older-client compatibility field, and Pulse MCP
source surfaces, an Assistant step count for contextual Assistant or
external-agent collaboration, plus a content-free Patrol mode
completed-loop count derived when Patrol mode starter evidence, Patrol
issue evidence, contextual collaboration, and either a rejected governed
decision or an approved governed decision with verified outcome proof coexist
in the same status window. It also exposes a stricter Patrol mode
resolved-loop count only when that status window has an approved governed
decision and verified outcome proof. The `proActivationOperationsLoopStarterCount`
field is retained only for older clients; primary clients must use
`patrolControlOperationsLoopStarterCount`. The legacy `patrolAutonomy*` and
`proActivation*` completed-loop, resolved-loop, and value-state fields mirror
the Patrol mode counts and value state for compatibility. AI runtime
surfaces may use those counts to show that a starter
was rendered or launched, that contextual collaboration occurred, or that the
first-party Patrol mode
loop reached terminal or resolved proof, but must not treat them as Assistant
transcript, approval payload, action command, resource identity, or
finding-resolution detail. External-agent prompts and MCP adapters may treat a
rejected-only decision as a terminal no-execution outcome, but approved
decisions must still continue through execution and verification before the
verified-outcome stage is complete.
Runtime Assistant availability remains an authenticated Assistant concern,
served by `GET /api/ai/assistant/surface-tools`, not a static fact in the
public agent-capabilities manifest. When the Pulse Assistant shell displays
live capability availability, it must load that authenticated endpoint through
`AIChatAPI.getAssistantSurfaceTools()` and format the posture through
`frontend-modern/src/api/agentCapabilities.ts`; the shell must not keep a local
registry-tool catalogue or reuse MCP manifest capability names as native
Assistant inventory. The External agents settings panel may show the
external Pulse MCP tool posture, but it must get the normalized static
`SurfaceToolContract` from `manifest.surfaceToolContracts` through
`getAgentManifestSurfaceToolContract(manifest, AGENT_SURFACE_ID_PULSE_MCP)`
and format it with `getAgentSurfaceToolPosturePresentation`;
external-adapter request/response filtering, including the `subscribe_events`
streaming exception and any capability omitted from the published Pulse MCP
surface allowlist, must be owned by the backend projection in
`ProjectManifestSurfaceToolContracts`; shared capability reads must use
`ResolveManifestSurfaceToolContract` and `ManifestSurfaceToolCapabilities` so
the normalized surface contract and the resolved `tools` affordance gate are
applied once. MCP initialize must also advertise the `tools` capability and
name "offered tools" in operating instructions only when that same published
surface tool contract resolves. MCP prompt support must similarly enter through
`MCPManifestSurfacePromptProjectionSupported`, which combines the target
surface's `prompts` affordance with the manifest-owned workflow prompt
catalogue before initialize or `prompts/list` advertises prompts, and through
`GetMCPPromptFromManifestSurface` before `prompts/get` renders any prompt. The
global `pulse_operations_loop` prompt is part of that same shared catalogue,
not a native Assistant-only starter: it must appear in Assistant and MCP only
when the manifest can support the full Patrol-to-Assistant-to-governed-action
flow, and it must guide contextual explanation, plan-first governed action,
approval or rejection only when the returned policy requires a decision,
policy-allowed execution, post-action verification, and finding resolution from
the same rendered prompt text. The rendered prompt must orient
first through `get_patrol_control_status` and re-read that status, resource
context, and findings before treating an outcome as verified. Successful render activity for
that starter is activation evidence for the Patrol issue-handling journey, not
proof of contextual collaboration, governed action execution, verification, or
finding resolution.
Native Assistant callers may request a preferred workflow prompt through safe
browser context, but the drawer must resolve that request against the
manifest-owned workflow prompt catalogue and render it through
`/api/ai/workflow-prompts/render`; it must not inline prompt templates or
silently append to an existing composer draft. Preferred workflow prompt
requests are one-shot browser seeds and must be cleared with the scoped handoff
payload after the first successful send.
The MCP `tools/call` execution
helper must
accept the manifest and surface id, apply the same affordance and surface-tool
allowlist used by `tools/list`, and only then delegate to neutral capability
HTTP execution; it must not accept a caller-supplied raw capability slice as the
MCP execution boundary. Frontend consumers may normalize a published
`surfaceToolContracts` entry for presentation, but must not infer MCP tools
from raw manifest capabilities when that entry is missing, and must not add a
Pulse-MCP-specific frontend helper alias around the generic surface resolver.
The generated Pulse MCP README follows the same rule: its surface-contract
block must be rendered by `MCPSurfaceContractMarkdown` from
`Manifest.SurfaceContract`, alongside the manifest-derived scope, tool, prompt,
and error-code inventories.
The public Pulse Intelligence overview follows the public-docs version of that
rule: the marked overview block in `docs/AI.md` must be rendered by
`PulseIntelligenceOverviewMarkdown` from the same manifest-owned
`SurfaceContract`, while surrounding product detail may remain human-authored.
The External agents settings panel follows the browser-facing version of
that same rule: its Pulse Intelligence Core, Patrol, Assistant, and MCP surface
summary must be projected from `/api/agent/capabilities.surfaceContract` through
`frontend-modern/src/api/agentCapabilities.ts`, not maintained as panel-local
copy; surface affordance badges are part of that same manifest projection. Its
capability-publication wording must describe published manifest-owned surface
contracts, not raw backend capability rows, so the panel does not imply that
adding a raw capability automatically exposes it to Pulse MCP.
Its external-agent recovery summary must also be manifest-backed: stable
failure-code chips or summaries may be rendered for setup ergonomics, but they
must derive from capability `errorCodes` through the shared frontend manifest
client rather than a panel-local recovery-code list.
External-agent token setup copy in that panel must route operators back to the
API Access token creation surface and the manifest-derived `Pulse Intelligence
agent` preset instead of carrying MCP-local token minting logic or a
component-local required-scope list.

## Extension Points

Patrol model qualification extends only through
`internal/ai/qualification/`, `cmd/patrol-qualify/`, and reviewed manifests
under `tests/qualification/patrol/`. A manifest's qualification repeat profile must be
capable of clearing the shared 95% Wilson lower-bound gate at a perfect pass
rate; manifest validation rejects statistically impossible profiles before any
live fault is provisioned. `live-suite` may sequence every manifest
for one explicit track, but it must acquire and freshly preflight one exact
provider/model route before provisioning any scenario, retain that route for
every repetition, and restore it once after the complete suite. Scenario
runners assert the leased route and must not rewrite model settings or trigger
their own provider preflights. A fresh suite preflight failure aborts setup once
as qualification-infrastructure evidence; cached failure state must never be
recounted as multiple model or scenario failures without independent provider
invocations. `live-suite` must retain the existing live-fault and separate
remediation authorization gates for every run. Voluntary community evidence
must be a local, mode-0600, allowlist-only derivation: it may carry aggregate
scores, safety booleans, cost/latency, exact model/provider, scenario and
runtime provenance, a pre-run public challenge, and source content digests. It
must not copy raw findings, resources, topology, endpoints, logs, prompts,
model output, tool identity or payloads, action identity or payloads, or error
prose, and creating it must never perform a network upload. Community evidence
may shortlist models but cannot satisfy Pulse certification or hosted-model
selection without a separately reproduced controlled-lab campaign.
Patrol finding-report authoring guidance extends through the registered
`patrol_report_finding` schema's required-argument list. Normal and bounded
final-decision prompts must render that shared list and require every parallel
report call to be independently complete; they must not maintain a second
hand-written field list. Runtime may let a model recover after a rejected call,
but formal qualification continues to reject every unsuccessful tool call,
including a recovered schema-validation lapse. When one parallel lifecycle
batch has both accepted and rejected siblings, the accepted writes must not
force the normal text-only summary before one bounded repair-only turn. That
turn may use only the lifecycle tools needed to correct rejected calls, must
preserve accepted calls without repetition, and may extend the normal Watch
turn budget by at most one provider call.

Assistant control wording must identify the effective scope as Assistant chat
only. Patrol autonomy and global Actions remain separate authority surfaces;
the chat pill may display the effective `control_level` but must not imply that
read-only chat demotes Patrol or infrastructure action policy.

The manual Patrol route is an extension boundary for scoped work. `POST
/api/ai/patrol/run` (`HandleForcePatrol`) accepts an optional scope body
(`resource_ids` and/or `resource_types`, plus optional `alert_identifier`,
`alert_type`, and `context`) to run a manual Targeted check through the same
`TriggerScopedPatrol` engine and scoped run record as automatic alert-triggered
work; with no body it keeps the fleet-wide Patrol check. The scoped path must
reuse the existing scoped engine rather than adding a parallel trigger route,
honour the same Patrol readiness gate as a full run, bypass the full-run
cadence gate (targeted checks never consume a manual full-run allowance), and
carry resource identity only — no command, prompt, or remediation payload —
while the route keeps requiring admin plus `ai:execute` scope for both shapes.

Pulse Intelligence presents Patrol as the primary first-party operations
surface. Pulse Assistant is the in-app contextual explanation, approval-card,
governed-action, verification, and handoff access path over Patrol work.
`cmd/pulse-mcp/` is the external-agent adapter for MCP-speaking clients. The
adapter must stay thin: it fetches `/api/agent/capabilities`, projects that
canonical manifest as MCP tools, and forwards calls to the declared API routes.
The shared `MCPManifestToolServer` must therefore consume the manifest as a
unit, not only a detached capability slice, so initialize instructions, tool
projection, resources, prompts, notifications, and future manifest-backed
surface metadata remain coupled to the same discovery document. It must resolve
the target surface's affordance contract before initialize advertisement and
before tool, resource, and prompt handlers run; raw manifest capabilities,
workflow prompts, or context routes are availability inputs only after the
surface contract allows that affordance.
When an adapter renders a manifest prompt locally, it may notify Pulse through
the authenticated `POST /api/agent/workflow-prompt-activity` marker endpoint
only after the prompt render succeeds. That marker route accepts manifest-owned
prompt names only, requires the token to cover `monitoring:read`, normalizes
`X-Pulse-Agent-Surface: pulse_mcp` into the Pulse MCP surface, and records no
prompt arguments, rendered text, resources, findings, token identity, or
request payload.
`scripts/generate-pulse-intelligence-docs.go` is part of that coupling: it
must update the MCP README's Pulse Intelligence surface contract and the public
`docs/AI.md` overview from the manifest instead of leaving hand-written
Core/Patrol/Assistant/MCP relationship prose beside generated inventories.
Public AI docs may describe outbound usage telemetry only as counts,
feature flags, and coarse Patrol control and governed Pulse Intelligence
operations adoption flags and counters.
They must keep prompts, chat messages, command text, action output, token
values, resource identifiers, and hostnames explicitly outside that telemetry
scope.
Governed-operation workflow starter counts may appear in that outbound telemetry
only as 30-day aggregate counts for the canonical `pulse_operations_loop`
prompt, split by total, native Assistant, first-party Patrol, and Pulse MCP surfaces. Those counts
may make `pulse_intelligence_loop_active_30d` true because the operator entered
the guided journey, but they must not satisfy contextual-collaboration,
complete-loop, approved-execution, or resolved-loop telemetry by themselves.
Assistant cost-ledger events may carry only the coarse `context_scope`
classifier for product-originated finding, resource, handoff, action, or
structured-mention turns; telemetry may aggregate that classifier into
governed-context Assistant counts, but it must not export prompts, session IDs,
resource IDs, finding IDs, command text, or action payloads.
Assistant cost-ledger events may also carry only a count-only
`tool_call_count` for accepted, model-selected governed tool calls within the
turn. Telemetry may aggregate that number into Assistant governed-tool
collaboration counts, but it must not export tool names, tool arguments, tool
results, provider call IDs, transcript content, resource IDs, finding IDs,
command text, action payloads, or session IDs.
The Pulse MCP README and the in-app Agent integrations panel must present that
adapter as one reusable runtime contract with client-native config wrappers:
OpenCode uses a top-level `mcp` object in `opencode.json` / `opencode.jsonc`,
while Claude-style clients use `mcpServers`. The shared command, base URL, and
token environment variable remain the Pulse Intelligence contract; client file
formats are wrappers around that contract, not separate Pulse Intelligence
surfaces. In the in-app Pulse Intelligence settings panel, those copied wrapper
snippets are on-demand setup detail; they must not occupy the default
external-agent read above the Patrol-autonomy and scoped-token hierarchy. The
Security API Access page may mint the scoped token, but it must not become the
external-agent setup page.
Full-surface token scope guidance must come from the canonical manifest's
manifest-owned `requiredScopes` set through shared
`agentcapabilities.ManifestRequiredScopeList` /
`ManifestRequiredScopeMarkdownList` helpers rather than Assistant-, UI-, or
adapter-local hardcoded or recomputed scope copy.
The native Assistant runtime prompt, lifecycle logs, and readiness messages
must identify that first-party surface as Pulse Assistant rather than the
legacy generic `Pulse AI` persona; the shared engine/core vocabulary remains
Pulse Intelligence.
The canonical manifest declaration, manifest wire shape, action-mode enum,
registry tool-name vocabulary, structured registry tool schema, provider projection helpers, JSON Schema
object-envelope helpers, provider tool/call/result shapes, provider-call
projection helper, provider tool-name catalog exact/prefix matching,
provider tool-call artifact detection and streaming tool-name prefix holding,
provider tool-call input normalization that clones caller
maps and nested JSON-like argument values before initializing empty input
objects, provider-emitted streamed tool-input parsing, final provider
tool-input raw fallback handling, and native Assistant question input parsing,
control-level vocabulary, control-tool availability
predicate, tool-governance defaulting, disabled-control guidance, capability
lookup and typed lookup-error helpers, external-tool
projection helpers, and shared agent HTTP substrate, including named capability
execution, live in
`internal/agentcapabilities`; MCP JSON-RPC envelopes, method constants,
request decoding, line-delimited stdio request serving, notification response
policy, stable JSON-RPC encoding, manifest-backed tool-server semantics,
tool-server method dispatch, initialize instruction/tool-call/resource/prompt payloads,
tool-server initialize result construction, notification methods, MCP
`tools/call` raw params decode, MCP resource URI projection, context-backed
`resources/list` / `resources/read` projection, and manifest-backed
`prompts/list` / `prompts/get` projection live at the MCP adapter edge.
Native Assistant registry declarations are part of that same identity contract:
every registry `Tool.Definition.Name` for Pulse Intelligence tools, including
the read-only `pulse_summarize` reporting synthesis tool and Patrol runtime
tools, must consume `internal/agentcapabilities/tool_names.go` constants rather
than owning string literals in `internal/ai/tools`.
Shared Pulse Intelligence workflow prompt definitions, manifest-owned
`workflowPrompts` catalogue selection, prompt display labels, MCP prompt title
projection, presentation kind hints, prompt argument validation, and prompt
rendering rules live in the neutral workflow-prompt core so native Assistant
starters, frontend manifest consumers, and external MCP prompts cannot drift.
When `workflowPrompts` is present on a manifest it is authoritative; capability
projection is compatibility plumbing for older manifests, not a second prompt
catalogue. The native Assistant starter row must therefore derive starter
availability from `workflowPrompts` and the attached Assistant context, then call
the AI/API-owned render route before inserting any prompt text into the composer;
it must not copy prompt labels, prompt bodies, or MCP `prompts/get` rendering
logic into frontend state. MCP prompt list/get helpers must likewise accept the
manifest-owned workflow prompt catalogue through `ManifestPulseWorkflowPrompts`,
`ProjectMCPWorkflowPrompts`, `GetMCPPromptFromManifestSurface`,
`GetMCPPromptFromManifest`, and
`BuildMCPPromptFromManifest`; they must not expose raw-capability `ProjectMCPPrompts`,
`GetMCPPrompt`, or `BuildMCPPrompt` entrypoints that can re-create a second MCP
prompt catalogue. Shared tool-call
parameter normalization and validation live in the neutral tool-call core: tool
names are trimmed and required, argument maps
are cloned deeply enough to detach nested JSON-like `body` maps/slices and
initialized, and malformed or semantically invalid tool-call params must fail
before capability lookup so native Assistant execution, the legacy text
projection, and external MCP adapters cannot drift on empty-name, nil-argument,
or caller-owned nested argument handling. Shared capability tool HTTP execution
and direct execution output/error mapping live in the neutral tool-execution
core, while result wrapping, result-text extraction, result interpretation, and
the canonical `HTTPCallResponse` to shared tool-result mapping live in the
neutral tool-result core. JSON object result bodies must also project into the
same shared structuredContent envelope while preserving the serialized text
content block for compatibility, so MCP clients and native Assistant paths see
one canonical result contract. Native Assistant direct execution paths must use that
shared text/marker interpretation and output/error projection instead of
flattening tool-result content blocks or branching on `isError`, approval, or
policy outcomes locally, so in-app execution and external MCP adapters
interpret the shared result envelope the same way. MCP `tools/call` decoding is
adapter-edge protocol work, but execution authorization must still enter the
neutral tool-execution core only after the shared manifest surface contract has
selected the allowed request/response capabilities. Patrol-only Assistant
registry tools that return JSON must build those responses through the shared
`agentcapabilities` JSON result constructors instead of manually marshaling JSON
into text-only results, so `structuredContent` remains available to both native
Assistant and external-agent result consumers. Patrol verification
tool-call records that decide success, failure,
approval-required, or policy-blocked outcomes must also branch through the
shared tool result interpretation rather than treating `isError` alone as the
success boundary. Reference clients that need a successful raw response body must also
use the shared request/response capability body helper so named capability
lookup, path/body projection, API-token headers, and stable non-2xx error
formatting stay in one place. The
agent SSE subscription transport, record parser, actionable-record filter, and
SSE-to-MCP notification bridge also live there so MCP notification bridges and
reference probes consume the same `Accept: text/event-stream`, status handling,
`event:` / `data:` framing rules, transport-event filtering, and JSON-RPC
notification projection.
The legacy
approved-action text projection for Pulse tool calls is also parsed there into
the shared `ToolCallParams` shape, including the internal approved-action
argument key/helper that carries a granted approval into replayed tool execution,
the `current_resource` handle plus governed aliases, and the recursive
tool-argument detector that blocks unresolved attached-resource placeholders
before visible execution; text invocations are then normalized and validated
through the same shared parameter contract, so
Assistant approval execution and MCP-compatible adapters cannot drift on tool
name, `default_api:` prefix, quoted argument handling, placeholder target
semantics, empty names, nil
arguments, or pre-approved action replay semantics. Structured tool
response envelopes, tool error-code vocabulary, the structured tool-result
error-code parser, and the `verification.ok` evidence parser also live there
so Assistant blocked/failed tool outcomes, provider call params, agent-facing
tool failure contracts, FSM block/recovery codes, recovery tracking, and
write-tool self-verification semantics cannot drift. The legacy-compatible Assistant tool markers
for approval-required and policy-blocked outcomes also live there, including
the stable `APPROVAL_REQUIRED:` / `POLICY_BLOCKED:` prefixes, payload `type`
values, formatter helpers, parser helpers, and the typed approval-required
payload contract for plan, context-confidence, preflight, target, risk, and
description data. Tool families may own the resource-specific payload fields
they add to those markers, but they must not redeclare the marker envelope,
rebuild anonymous approval payload readers, branch on local prefix strings, or
hide shared marker parsing behind Assistant-local parser wrappers. Legacy
service approval-marker handling may keep compatibility fallback behavior for
old marker text, but the canonical decision path must call the shared marker
parser directly at the branch site. Tool execution must not drop the returned
argument map when carrying a granted approval into replayed tool execution.
Native Assistant tool protocol aliases must expose only the shared neutral
registry shapes the Assistant actually executes (`Tool`, `ToolCallParams`,
`ToolResult`, `ToolContent`, and structured tool-response envelopes). JSON-RPC,
MCP initialize/list/resource/prompt payloads, and other MCP wire aliases remain
at the external adapter edge in `internal/agentcapabilities/mcp.go`, not in
`internal/ai/tools/protocol.go`. Chat transcript and API-facing tool-result
shapes must alias the shared provider-call contract while the richer in-app
Assistant transcript projects to that shared shape through an explicit helper.
Provider-result context projection also belongs to `internal/agentcapabilities`:
native Assistant, the legacy AI service loop, external adapters, and future
Pulse Intelligence surfaces must build full transcript results and model-context
results through the shared projection so transcript preservation, error flags,
and model-result truncation cannot drift into surface-local pairs.
API handlers, Assistant tool governance, `cmd/pulse-mcp`, and the in-repo probe
consume that shared contract so agent projections cannot drift into local
structs, local structured schema types, local provider schema projection, local
schema wrappers, local request/response filters, local capability maps, local
path placeholder parsing, local argument-to-body shaping, local manifest
fetchers, local API-token header spelling, local stable error-envelope
formatting, local named capability execution, local tool-governance defaulting,
local disabled-control guidance, local MCP manifest-backed
tool-server handlers, local MCP method dispatch, local MCP request decoding or
line-framing loops, local JSON-RPC encoders, local notification response
checks, local MCP initialize builders, local
HTTP-to-MCP result wrapping, local MCP result interpretation,
local SSE record scanners, local SSE-to-MCP notification bridges, local
tool-result maps, or local tool-description builders. Path placeholders from the manifest and top-level
tool arguments must be projected through the shared
`agentcapabilities.ProjectCapabilityCall` helper before dispatch, so path
values are percent-encoded single segments and non-placeholder write arguments
become the JSON request body when no nested `body` object is provided.
Internal tool-call metadata such as approved-action grants must stay on the
shared `agentcapabilities` argument helper path and be stripped by the shared
projection before any public manifest-backed HTTP body is built.
Canonical IDs containing `:`, `/`, spaces, or other reserved characters cannot
alter the route shape.
The Assistant and MCP surfaces must also rely on the router-owned manifest
route/scope proof: every canonical capability is projected through the shared
call helper into a concrete request, then exercised against the API router with
a token that lacks the advertised scope so drift between manifest metadata,
agent-facing placeholders, and real authorization fails before either surface
ships a mismatched tool.
Native Assistant provider seams are part of the Pulse Assistant surface, not
the external MCP adapter. Chat service aliases, API handler setter signatures,
router dependency wiring, direct in-app tool execution, and findings-store
adapters plus the broader native tool adapter family must use Assistant/Pulse
Intelligence terminology; `MCP` names are reserved for the protocol adapter,
MCP-compatible action-contract interface, wire envelopes, and result
interpretation contracts.
The Pulse Intelligence event vocabulary is part of that same shared
`internal/agentcapabilities` boundary. API SSE producers, the
`subscribe_events` manifest description, MCP notification advertising, MCP
transport-event filtering, MCP notification method projection, shared SSE
record parsing, shared SSE-to-MCP notification bridging, and reference probes
must consume the shared event kind constants/helpers instead of maintaining local copies of
`finding.created`, `approval.pending`, `action.completed`, `stream.connected`,
or `heartbeat`.
MCP `tools/list` must use the shared
`agentcapabilities.ProjectManifestSurfaceTools` projection for the Pulse MCP
surface. The manifest-owned `surfaceToolContracts` field is the allowlist for
which request/response tools Pulse MCP may advertise or execute; raw
`Manifest.Capabilities` remains the metadata and execution source, not the
surface availability decision. MCP initialize capability advertisement,
operating-instruction affordance copy, `tools/list`, `tools/call`,
`resources/list`, `resources/read`, `prompts/list`, and `prompts/get` must also
pass through the manifest-owned surface affordance contract: the surface must
allow tools, resources, prompts, and capability metadata before raw manifest
capability or prompt presence can expose them to an external client. The shared
`agentcapabilities.ProjectTools`
helper still owns the full request/response tool shape, including
manifest-owned capability metadata (category, method/path, scope, action mode,
approval policy, request/response shape, stable error codes, input schema, and
structured output schema). That metadata must be available both in model-facing
description prose and in the standard MCP `_meta["pulse.capability"]` object
for clients that need structured route/scope/governance facts, so external-agent
clients see the same governed API contract without a second MCP-specific tool
registry, projection loop, allowlist, or prose table. Action mode (`read`,
`mixed`, `write`) and approval policy (`scope_only`, `action_plan`) are
manifest-owned governance fields; MCP must not infer or override them from HTTP
method or local tool names. Manifest-backed capability lookup and `tools/list`
projection must also detach raw `inputSchema`, raw `outputSchema`, structured
metadata maps, and error-code slices through `agentcapabilities.CloneCapability`,
`CloneCapabilities`, and `CloneRawMessage`, so external-agent adapters cannot
mutate the manifest-backed contract while serializing or post-processing one
projected tool list.
The manifest-owned `get_patrol_control_status` structured output schema is
part of the same AI runtime contract. Its four-step operator rollup is a
content-free stage projection for Assistant and MCP callers: governance step
counts represent pending approvals until an approved or rejected decision
exists, then represent decision evidence, while verification step counts
represent verified outcomes or terminal rejected decisions. Optional MCP
readiness remains a separate `externalAgentReady` capability signal, not an
operator step. AI runtime surfaces must consume that manifest schema and backend
projection instead of inventing chat-local stage-count rules. The schema's
Patrol control starter count includes native Patrol, current paid Patrol
control, and legacy Pro activation handoff starts; its
completed-loop and resolved-loop counts are aggregate orientation fields for
native Assistant and MCP-facing orchestration only. Legacy
`patrolAutonomy*` and `proActivation*` completed-loop, resolved-loop, and
value-state fields are aliases for the Patrol control values; the legacy Pro
activation starter field remains an entry-point-specific count. All of
those fields must remain free of prompt names, finding IDs, action IDs,
resource names, actors, request bodies, commands, or model output.
Resolved-loop semantics remain approved-and-verified only.
`cmd/pulse-mcp` startup guidance and the generated MCP README token-scope
block must consume the manifest-owned `requiredScopes` summary through
`agentcapabilities.ManifestRequiredScopeList` /
`agentcapabilities.ManifestRequiredScopeMarkdownList`. MCP README tool and
capability-specific error-code inventories are generated through
`scripts/generate-pulse-intelligence-docs.go` from the manifest-owned Pulse MCP
surface contract via `agentcapabilities.MCPToolCapabilityInventoryMarkdown` and
`agentcapabilities.MCPErrorCodeInventoryMarkdown`. Workflow-prompt inventory is
generated through `agentcapabilities.MCPPromptInventoryMarkdown` from the same
`MCPManifestSurfacePromptProjectionSupported` gate and
`ManifestPulseWorkflowPrompts` / `ProjectMCPWorkflowPrompts` projection used by
`prompts/list`, including manifest labels as prompt titles. README tool lists,
prompt lists, scope lists, and capability-specific error-code lists must not be
hand-maintained snapshots. MCP
`tools/call` must use the same shared
`agentcapabilities.ManifestSurfaceToolCapabilities` resolver as `tools/list`;
streaming capabilities such as `subscribe_events`, and raw manifest
capabilities omitted from the Pulse MCP surface contract, remain unavailable to
request/response tool calls even if a client sends their name manually. Adding
a raw capability to `Manifest.Capabilities` is not enough to publish an MCP
tool; the Pulse MCP `surfaceToolContracts` allowlist must opt it into that
external-agent surface. Publishing the Patrol work prompt also requires the
surface contract to include `get_patrol_control_status`, so MCP clients can
read the same content-safe Patrol work stage before requesting fleet, finding,
resource, approval, action, or resolution detail. The legacy
`get_operations_loop_status` name may remain accepted only at compatibility
edges.
MCP `resources/list` and `resources/read` are also shared manifest-backed
projections: `resources/list` must call the canonical `get_fleet_context`
capability, `resources/read` must call `get_resource_context`, and the
`pulse://resource/<resource-id>` URI shape, resource content MIME type, and
resource read parameter validation live in `internal/agentcapabilities` rather
than in `cmd/pulse-mcp` or an MCP-only inventory registry. The
`get_fleet_context` capability accepts optional additive filter arguments
(hasFindings, severity, technology, resourceType) that an agent may pass
through `tools/call` to narrow the fleet to a relevant subset; the projection
layer forwards non-path GET arguments as URL query parameters so the manifest
contract, not a per-capability adapter, owns query-string transport. The
`resources/list` adapter calls fleet-context with no filters and continues to
receive the full fleet.
than in `cmd/pulse-mcp` or an MCP-only inventory registry. They must enter
through `ManifestSurfaceResourceCapabilities`,
`ListMCPManifestSurfaceResourcesHTTP`, and
`ReadMCPManifestSurfaceResourceHTTP`, not raw-capability
`ListMCPResourcesHTTP` or `ReadMCPResourceHTTP` helpers. The MCP resource
methods remain disabled when the target surface affordance contract disables
resources, even if those context capabilities exist in the manifest.
Operator-state writes, governed finding lifecycle tools, and action
plan/decision/execute tools must also carry manifest-owned `inputSchema`
definitions for their exact arguments so external-agent clients receive typed
fields and enums from the canonical manifest instead of an MCP-local free-form
body convention. When the manifest has no authored schema, MCP may derive a
permissive fallback through the shared `agentcapabilities.ToolInputSchema`
helper, but it must not hand-roll a second JSON Schema envelope.
Governed finding lifecycle tools must return the agent-stable error envelope
from `internal/agentcapabilities/errors.go` for branchable failures
(`invalid_finding_request`, `finding_not_found`,
`finding_action_not_allowed`, `patrol_unavailable`) and declare those codes in
the canonical manifest; UI callers may still use the same HTTP routes, but the
wire contract is owned by the Pulse Intelligence agent surface once the route
is advertised there. The exported `agentcapabilities.AgentErrCode*` constants
are the source of those values for both manifest declarations and
`internal/api/` handler emissions; local string literals in either layer are
contract drift.
The governed action tools also declare `mock_mode_enabled` for read-only mock
runtimes and `action_plan_identity_mismatch` when a presented reviewed hash no
longer matches the authoritative plan. Pulse MCP documentation must remain a
generated projection of those manifest codes so Assistant and external-agent
adapters branch on the same refusal vocabulary as direct HTTP clients.
`GET /api/actions` and `GET /api/actions/{id}` use that same shared vocabulary
for invalid view/limit and internal list/detail failures even though those
relay-mobile continuity reads are not separate Assistant tools; their codes
must not drift into handler-local literals or imply Task 10 terminal truth.
Manifest `scope` values are auth-owned vocabulary from
`pkg/auth` and must be declared through manifest-local aliases to those
constants, not repeated as string literals, so MCP tools and direct HTTP agents
advertise the same scopes the API authorization layer enforces. Patrol finding
capabilities (`list_findings`, `acknowledge_finding`, `snooze_finding`,
`dismiss_finding`, and `resolve_finding`) are part of the AI runtime route
surface and therefore advertise `ai:execute`; they must not be downgraded to
monitoring scopes in MCP docs or manifest projections while the router gates
those HTTP routes through AI execution authorization.
Do not add MCP-only business logic or a second tool registry here. If a new
operation should be available to external agents, add it to the canonical API
capability manifest and make Assistant consume the same governed backend
contract when the in-app UX needs it.
Assistant prompt governance follows the same rule: live turns derive tool
mode, typed approval policy (`scope_only` or `action_plan`), approval summary,
and tool summary text from the shared `agentcapabilities.ToolGovernanceDescriptor`
shape returned by `PulseToolExecutor.ListToolGovernance`; descriptor defaults
must be constructed through `agentcapabilities.NewToolGovernanceDescriptor`,
and fallback prompt
text must call the registry-owned
`tools.CanonicalToolGovernanceForSurface` helper with the explicit requesting
Pulse Intelligence surface rather than maintaining a chat-local copy of tool names, governance
descriptor fields, policy vocabulary, or governance prose.
Assistant-only structured interaction tools such as `pulse_question` must also
be projected through the shared
`agentcapabilities.BuildAssistantToolGovernancePromptSection` and
`agentcapabilities.AssistantGovernanceOfferedToolNames` helpers rather than by
hand-building prompt-policy lines or offered-tool filters in chat-local code;
those tools remain native Assistant interaction boundaries, not MCP manifest
capabilities.
Direct Assistant registry execution is also part of the shared Pulse
Intelligence tool-call contract: `ToolRegistry.Execute` must normalize and
validate tool names and arguments through `agentcapabilities.ToolCallParams`
and use the shared invalid-params, unknown-tool, and control-disabled tool
result helpers before lookup or handler dispatch, so in-app direct execution
cannot drift from external MCP `tools/call` on trimmed names, blank-name
rejection, nil-argument initialization, deep caller-argument copying, or
standard entrypoint failure envelopes.
Chat-service direct execution paths such as autonomous command execution and
approved-fix replay must route through one Assistant registry execution helper
before calling `agentcapabilities.InterpretDirectToolExecution`, so
surface-specific failure wording does not fork executor lookup, registry
dispatch, marker interpretation, or shared result handling. MCP-named
direct-execution symbols are not part of the shared adapter contract; native
Assistant code and MCP adapters must use the neutral direct tool execution
helper.
Cross-repo auto-fix and Patrol-approved action binders must resolve approved
tool execution through `AIAutoFixHandlerDeps.ResolveApprovedAssistantToolExecutor`;
that resolver returns the native `ApprovedAssistantToolExecutor` only, so
enterprise code cannot recreate MCP-named Assistant execution selection. The
current public runtime must populate `AssistantToolExecutor` only; MCP remains
an external adapter boundary and is not an approved-fix execution dependency.
Assistant telemetry that records recoverable policy-block attempts must also
use the shared `agentcapabilities` tool-response error-code constants rather
than local string literals, so metrics labels, structured tool responses,
Assistant recovery tracking, and MCP-facing agent error vocabulary stay
aligned.
Assistant provider tool schemas are also registry-owned: chat must obtain the
runtime provider tool surface from `PulseToolExecutor.AssistantProviderTools`
so availability filtering, registry-owned governance descriptors, and
Assistant-native interaction tools are composed at one executor-owned boundary
instead of being paired manually in chat paths. That executor projection must
project registered `tools.Tool.InputSchema` values through the shared
`agentcapabilities.ProjectProviderTools` provider-schema helper and the
manifest-affordance-gated
`agentcapabilities.ProjectPulseAssistantProviderTools` provider-surface helper,
and model-selected tool calls must flow through
`agentcapabilities.ProjectProviderToolCallToToolCall` and the shared
`agentcapabilities.NormalizeProviderToolCallForExecution` /
`NormalizeProviderToolCallsForExecution` execution normalizers, not local schema
maps, compatibility aliases, raw provider call fields, chat-local provider
projection wrappers, or MCP-shaped provider-projection wrappers.
MCP-shaped provider projection names remain protocol compatibility aliases in
`internal/agentcapabilities/mcp.go`; they must not be implemented in the neutral
provider schema boundary or imported by native Assistant runtime paths.
Shared projected-tool behavior hints are the same class of boundary: the
neutral projection owns `ToolBehaviorHints`, while any `MCPToolAnnotations`
name is an alias in the MCP wire edge only.
Assistant-only provider tools are still shared Pulse Intelligence declarations:
the native `pulse_question` clarification tool must be included in provider
tool lists through `agentcapabilities.ProjectPulseAssistantProviderTools` and
`agentcapabilities.AssistantNativeProviderTools`, its declaration must come
from `agentcapabilities.NewPulseQuestionProviderTool`, its JSON Schema must
remain owned by `agentcapabilities.PulseQuestionProviderInputSchema`, its
question type/defaulting vocabulary must use
`agentcapabilities.PulseQuestionToolTypeValues` and
`agentcapabilities.NormalizePulseQuestionToolType`, its emitted provider input
must parse through `agentcapabilities.ParsePulseQuestionToolInput`, and
allowlists that recognize leaked provider tool calls must consume
`agentcapabilities.NewAssistantProviderToolNameCatalog` instead of hard-coding
Assistant-native names or rebuilding provider-tool name sets locally. Provider
tool-call artifact detection and streaming partial-tool-name holding must use
`agentcapabilities.ProviderToolCallArtifactIndex` and
`agentcapabilities.SplitTrailingProviderToolNamePrefix` with that shared catalog,
so Assistant and future adapter boundaries do not fork DSML/XML/pipe/JSON/function
leak semantics. Chat may own waiting for user answers, UI event
emission, and FSM user-input behavior, but it must not carry a chat-local
provider schema, description, tool-list append rule, prompt-filter rule, parser,
tool identity, or provider artifact detector for that interaction.
That schema boundary is also a copy boundary: registry tool definitions must
normalize through `agentcapabilities.Tool.NormalizeCollections`, and
`ToolRegistry.Register` / `ToolRegistry.ListTools` must not expose caller-owned
or registry-owned `InputSchema` maps, required slices, enum slices, provider
input-schema maps, or JSON-schema default containers for later mutation by an
Assistant, provider, or external-agent surface. Direct provider schema
projection helpers must normalize through that same copy boundary before
emitting provider JSON so JSON-schema default containers cannot alias registry
definitions.
Provider tool-call normalization must also stay shared: provider responses,
Assistant transcript storage, live Assistant execution, and API adapter
projections must get independent input maps, nested JSON-like argument values,
trimmed executable tool names, initialized empty argument maps, and raw provider
thought-signature payload bytes from
`agentcapabilities.ProviderToolCall.NormalizeCollections` plus the shared
execution normalizers so a later mutation in one surface cannot rewrite another
surface's tool arguments or provider reasoning-continuity metadata.
Assistant transcript normalization is part of that copy boundary:
`chat.Message.NormalizeCollections` and `chat.ToolCall.NormalizeCollections`
must detach tool-call slices, nested tool-call input maps/slices, provider
thought signatures, success pointers, and tool-result pointers before stored
messages are reused across session storage, provider-continuation, browser/API,
or orchestrator adapter surfaces.
Provider-facing tool results follow the same shared boundary: Assistant
tool execution must project shared tool-call results through
`agentcapabilities.NewProviderToolResultFromToolResult` and build transcript or
model-context result messages through `agentcapabilities.NewProviderToolResult`
so result text flattening and `is_error` semantics stay aligned with
external-agent adapters. MCP-named provider-result projection helpers are not
part of the adapter contract; runtime paths that need the plain text from a
shared tool result must call `agentcapabilities.ToolResultText` at the execution
site instead of keeping Assistant-, Patrol-, or MCP-local result-text wrappers.
Runtime paths that convert chat transcripts, repair orphan tool calls, or run
legacy service tool loops must not assemble `providers.ToolResult` or
`chat.ToolResult` structs locally, and must not hide shared provider-result
construction behind Assistant-local pointer wrappers.
Native Assistant registry execution and external MCP method dispatch must also
normalize shared `ToolResult` values through
`agentcapabilities.ToolResult.NormalizeCollections` before returning them, so
handler-owned content slices and nil content collections cannot leak across
Assistant, provider, or MCP response boundaries. MCP may retain only actual
`MCPToolResult` / `MCPContent` protocol wire aliases; result constructors,
structured tool-response projection, JSON object and array-wrapper
structuredContent projection, text extraction, and interpretation stay neutral
shared helpers rather than MCP-named wrappers.
Assistant runtime settings and tool exposure must derive
read-only/controlled/autonomous semantics from
`agentcapabilities.ControlLevel` and
`agentcapabilities.ControlLevelAllowsControlTools`; future MCP adapters may
alias the same values, but must not carry separate control-tool availability
rules, disabled-control guidance, or tool-governance defaulting.

Guest-family AI runtime code paths (VMs and LXC system containers) share
generic helpers instead of per-family copies. Patrol guest intelligence
gathers both families through `gatherGuestIntelligenceFromViews` in
`internal/ai/patrol_intelligence.go`; the Assistant query tool resolves
single-guest gets through `canonicalGuestGetResult` / `guestViewGetResult`
and search matches through `addCanonicalGuestSearchMatches` /
`addGuestViewSearchMatches` in `internal/ai/tools/tools_query.go`; per-guest
disk summaries flow through `appendGuestDiskSummaries` in
`internal/ai/tools/tools_storage.go`. A new guest family extends those
helpers (their view-constraint interfaces name the read-state methods they
need) rather than re-rolling a parallel loop. The same shape applies to
mutating tool pipelines: file append/write share `executeFileMutation`
driven by `fileMutationSpec` (`internal/ai/tools/tools_file.go`), and
namespaced kubectl actions share `executeKubernetesResourceAction` driven by
`kubernetesResourceAction` (`internal/ai/tools/tools_kubernetes.go`) — new
file or kubectl actions add a spec, keeping approval-command text stable,
instead of duplicating the approval/audit pipeline. OpenAI-compatible message
conversion for both non-streaming and streaming turns is shared through
`convertMessagesToOpenAI` (`internal/ai/providers/openai.go`), Anthropic message
conversion for both non-streaming and streaming turns is shared by the API-key
and OAuth clients through `convertMessagesToAnthropic`
(`internal/ai/providers/anthropic.go`), Gemini message conversion for both
non-streaming and streaming turns is shared through `convertMessagesToGemini`
(`internal/ai/providers/gemini.go`), and AI memory history files load through
the generic `loadMemoryHistory`
(`internal/ai/memory/paths.go`). `Finding`/`findingJSON` and
`UnifiedFinding`/`unifiedFindingJSON` are deliberate marshal-mirror twins —
do not merge them; the mirror invariants are enforced by
`TestFindingJSONMirrorStaysInSync` and
`TestUnifiedFindingJSONMirrorStaysInSync`. The same mirror posture applies on
the transport side: `orchestratorChatAdapter.GetMessages`
(`internal/api/ai_handlers.go`) and `chatServiceAdapter.GetMessages`
(`internal/api/chat_service_adapter.go`) convert the same chat-service
messages onto deliberately separate output contracts; do not merge them, and
keep their field mapping aligned. Orchestrator-facing tool-call and
tool-result payloads must project through
`aicontracts.OrchestratorToolCallInfoFromProvider`,
`OrchestratorToolCallInfo.ProviderToolCall`,
`aicontracts.OrchestratorToolResultInfoFromProvider`, and
`OrchestratorToolResultInfo.ProviderToolResult` so the public investigation
handoff contract inherits the same `agentcapabilities` input-map,
thought-signature, and provider-result normalization as Assistant and MCP —
`TestOrchestratorAndChatAdaptersMapTheSameMessageFields` enforces it.

Assistant frontend presentation changes under
`frontend-modern/src/components/AI/Chat/` must keep live tool activity aligned
with OpenCode's mutable tool-part pattern without losing Pulse's operator audit
trail. The current OpenCode reference at fetched `origin/dev` commit
`233427f08eb8b2a0627123816e807123fbbe5b26` keeps tool input/progress/result as
a single mutable part in `packages/tui/src/context/sync-v2.tsx` lines 276-345,
updates running tool metadata from the execution context in
`packages/opencode/src/session/tools.ts` lines 53-66, and suppresses completed
successful tool detail when `showDetails` is off in
`packages/tui/src/routes/session/index.tsx` lines 1704-1779. Pulse adapts that
by keeping successful completed tool rows command/status visible, compacting
them while the same Assistant turn is still streaming and newer concrete
activity has arrived, and keeping successful raw output behind the details
disclosure by default once the turn completes. Pending tools, skipped/canceled
tools, failed tools, and completed-turn tool details must remain
visible/inspectable; failed output may render inline because it is actionable.
AI Chat icon-only actions own Assistant/session semantics, labels, and
handlers, but their visible button chrome must compose the shared
`ActionIconButton` primitive from `frontend-modern/src/components/shared/Button.tsx`.
Header utilities, session row controls, transcript fallback close/download
controls, activity-dock queued follow-up actions, composer send, footer
help/route actions, and compact dismiss controls must not recreate local
h-5/h-6/h-7/h-8/h-9 icon-button class shells. Message and tool transcript copy
controls own Assistant copy targets and copied-state timing, but their visible
copied-state icon chrome must compose `CopyValueButton` from the same shared
button primitive boundary rather than message- or tool-local copy/check shells.

Assistant live workflow status follows OpenCode's current-state timeline
model. CANONICAL (supersedes the per-row transcript-status rules elsewhere in
this contract): live workflow status is FOOTER-OWNED and must never be narrated
into the scrolling transcript. OpenCode keeps "the model is doing X" in a single
pinned footer line and puts only durable artifacts (user text, reasoning, tool
calls, the answer) in the timeline; Pulse mirrors that by rendering the latest
replacing workflow status ONLY in the activity dock above the composer
(`currentStatus` / `assistant-activity-dock` in
`frontend-modern/src/components/AI/Chat/index.tsx`) and never as a transcript
row or message-header chip — `shouldRenderWorkflowStatusEvent` and
`shouldShowHeaderWorkflowStatus` in
`frontend-modern/src/components/AI/Chat/MessageItem.tsx` are hard-`false`. The
dock must stay up for the whole turn: it is gated on the streaming assistant
message (`assistantTurnActive`), not `chat.isLoading()`, because loading flips
false at visible-turn-complete and would make the footer flash and vanish.
History may persist for audit/session state, but the dock renders the latest
replacing status immediately. Pulse must not replay stale status history through
artificial timers when the stream already has a newer workflow/tool state.
Frontend display code may refresh elapsed-time suffixes while a turn is active,
but status selection must read the current workflow status directly rather than
deriving an older display status from `workflowStatusHistory`.

1. Add or change chat runtime, Patrol orchestration, findings generation, or remediation behavior through `internal/ai/`
   Patrol deterministic disk-health evidence changes in
   `internal/ai/patrol_signals.go` and `internal/ai/patrol_ai.go` must read
   SMART health, normalized SMART counters, and canonical physical-disk identity
   from storage and metrics tool payloads without treating missing or unknown
   SMART health as `PASSED`. Non-zero pending-sector, offline-uncorrectable,
   media-error, and related SMART counters must remain model-facing reliability
   evidence even when the headline health field is `PASSED`, so Patrol does not
   summarize a disk as healthy while deterministic counters indicate disk risk.
2. Add or change canonical AI provider config, provider registry metadata, provider-scoped model selection, or runtime auth/base-URL defaults through `internal/config/ai.go` and `internal/config/ai_providers.go`.
   Direct providers that speak the shared chat-compatible API must use the
   registry-backed `AIProviderProtocolOpenAICompatible` path and
   `NewOpenAICompatibleClient` rather than adding new provider-specific factory
   branches. Per-provider auth fields, configured booleans, clear-key fields,
   default base URLs, display names, docs links, env-var hints, and fallback
   model rows must be declared once in `internal/config/ai_providers.go` and
   projected to `/api/settings/ai`; frontend settings may arrange those
   provider cards, but must not become the source of truth for provider
   capability or credential metadata. Provider model-cache keys must include
   provider identity, auth/base-URL inputs, and credential identity changes
   without exposing raw secret values.
   Ollama request keep-alive is a provider runtime option owned by this path:
   `internal/config/ai.go` stores the value, `/api/settings/ai` exposes it,
   and `internal/ai/providers/ollama.go` is the only layer that turns it into
   the Ollama `keep_alive` request field. An empty configured value means
   Pulse omits `keep_alive` so the Ollama server default applies.
   Per-surface model fallback chains are owned by the `Get*Model()` getters in
   `internal/config/ai.go` and follow the operator's intent split between
   interactive and background work. Interactive chat resolves
   `ChatModel -> Model`. Background bulk work — Patrol, auto-fix, and
   service-context discovery — resolves through the Patrol tier:
   `PatrolModel -> Model` for Patrol itself, `AutoFixModel -> PatrolModel ->
   Model` for auto-fix, and `DiscoveryModel -> PatrolModel -> Model` for
   discovery. Discovery is high-fan-out (one model call per container or
   service per refresh), so it must NOT fall back straight to the shared
   default model past a configured Patrol override: an operator who selected
   a cheap Patrol model has declared their background-work model, and
   scheduled context refreshes silently burning the shared (often premium)
   model with no chat activity is the bug this chain prevents. Settings copy
   that describes the shared default model must state this chain rather than
   claiming the shared default governs discovery unconditionally.
   Cloud context privacy is a FIXED posture, not a user setting. Pulse is a
   self-hosted homelab/SMB tool: when the operator points the Assistant at a cloud
   model they have accepted that their (non-secret) infrastructure detail reaches
   that provider, so the cloud model receives real resource context (names, IPs,
   config) and the Assistant is actually useful. There is intentionally NO
   `cloud_context_privacy` dial and NO `share_operational_context_with_cloud`
   toggle — the privacy control users understand is the choice of model (a cloud
   provider vs. a local Ollama model). Granular per-resource governance is an
   enterprise concern, not the homelab default.
   Two invariants always hold at the model boundary and are NOT configurable:
   1. Prompt-secret sanitation strips credentials (API keys, passwords, tokens) —
      credentials never reach a cloud model.
   2. The local-only floor: resources the policy engine routes
      `ResourceRoutingScopeLocalOnly` (Restricted sensitivity — tagged secret/pii,
      PMG, k8s-secrets, ...) have their identifiers redacted and never leave the
      local trust boundary. Everything else (Internal and Sensitive) flows.
      Both are enforced by the model-boundary sanitizer
      `modelboundary.RequestSanitizerForModel(model, provider)` (`internal/ai/modelboundary`),
      which returns nil for local (Ollama) models — local always receives full context.
      UNIVERSAL backstop rule: EVERY code path that sends infrastructure-derived
      content to an external model MUST install `RequestSanitizerForModel` before the
      provider request — not only the interactive agentic loop. This explicitly
      includes session compaction (`internal/ai/chat/session_compaction.go`
      `SummarizeSession`), which sends the PERSISTED transcript (original user prompts
      and tool outputs carry raw identifiers), and the shared service helper
      `(*Service).requestSanitizerForModel` (`internal/ai/service.go`) used by
      discovery analysis, the report and fleet narrators, quick analysis, and the
      ExecuteAgentic paths. A new model-bound request path that skips the sanitizer is
      a leak and is not permitted. (Static capability probes with no resource content,
      e.g. the Patrol preflight self-test, are exempt only because their payload is
      fixed and carries no identifiers.)
      Redaction-placeholder hygiene: Pulse-authored model-bound directives (the
      resource-context handoff instructions in `internal/ai/chat/service.go` and
      `internal/ai/chat/plain_text_resource_context.go`) must NOT inject the literal
      redaction placeholder (`unifiedresources.ResourcePolicyRedactedLabel`,
      "redacted by policy") into the prompt — naming it makes the model echo it back
      as if it were a resource name. Directives reference withheld labels neutrally
      ("a withheld or placeholder label") and must instruct the model not to repeat a
      withheld placeholder back to the user as the resource identity; the
      `current_resource` handle remains the authoritative target.
3. Add or change Pulse Assistant request flow through `internal/api/ai_handler.go`, `frontend-modern/src/api/ai.ts`, and `frontend-modern/src/api/aiChat.ts`
   Assistant session compaction is a runtime-backed session workflow, not a
   local waiting message, transcript-only UI action, or stubbed summarize
   endpoint. The referenced OpenCode source at fetched `origin/dev` commit
   `e82542b8023a8374f29c23b70ec019c8f256354e` registers `/compact` with
   `/summarize` as an alias in
   `packages/tui/src/routes/session/index.tsx` lines 548-570 and calls the
   runtime session summary action with the current provider/model route. Pulse
   adapts that behavior through `POST /api/ai/sessions/{id}/summarize`,
   `chat.Service.SummarizeSession`, and
   `SessionStore.CompactWithSummary`: the backend must reject active-running
   sessions, build a bounded and redacted transcript for the active chat model,
   record provider token usage as `assistant_session_compaction`, persist one
   visible assistant summary row plus the latest bounded turns, clear redo
   state, and never stream or persist raw provider tool-call artifacts, hidden
   reasoning, secrets, or raw provider responses as the compacted transcript.
   The drawer header action and `/compact` slash command must call the same
   endpoint, surface compacting progress, reload the compacted session and
   session list, keep focus in the composer after completion, and treat
   `not_needed` results as quiet operator feedback instead of an error.
   OpenRouter-routed Assistant chat requests must set a bounded default
   completion budget when the runtime request does not specify `MaxTokens`.
   OpenRouter preflights affordability against the requested maximum completion
   budget, so leaving the field unset can make tiny chat turns reserve a
   model-scale output default and fail against per-key total limits despite the
   account having credit. That bounded default must still be high enough for
   normal detailed Assistant answers such as inventory breakdowns; provider
   request shaping must not make an ordinary answer finish mid-sentence. The
   provider boundary owns this request-shape fix;
   UI code must not work around it with route-specific retry copy. If the
   provider still rejects the request because the requested completion budget
   exceeds the key limit, the Assistant error message must name that key-limit
   condition without exposing raw provider JSON, URLs, or key identifiers.
   Assistant stream provider failures emitted through chat must be classified
   in `internal/ai/chat` before callback events reach the frontend. Endpoint
   configuration/URL errors, provider reachability/transport failures, auth,
   quota/billing, rate-limit, timeout, and cancellation states must render as
   operator-facing Settings or retry guidance. Raw Go transport strings,
   provider JSON bodies, request URLs, request methods, dashboard links, and
   key-management links must not be streamed or persisted as chat-visible
   assistant output.
   Assistant provider readiness is part of that same request flow: when the
   drawer opens or the selected chat model changes, the frontend must start a
   background verification of the selected provider/model through
   `/api/ai/test/{provider}`. That check must use neutral provider diagnostic
   copy owned by `internal/ai/`, not Patrol runtime-finding wording, and the
   drawer may surface the result as actionable retry/settings status plus
   same-model configured-provider alternatives without converting it into
   assistant-authored output. Provider checking is a background diagnostic and
   must not delay the user's first useful chat turn; the drawer should keep
   routine selected-route health visible in compact composer chrome while
   reserving the larger provider-status banner for requested checks and
   actionable failures. A confirmed selected-route provider error must keep
   typed text and focus without blocking normal chat dispatch, but the send path
   must still use the currently selected route until the operator explicitly
   chooses a different route. Same-model configured-provider alternatives remain
   visible one-click route changes and failed-turn retry choices, not automatic
   send-time recovery or hidden provider fallback. The OpenCode reference
   behavior for this slice is retries on the selected provider/model, not
   silent fallback to another provider or route; Pulse must adapt that as
   explicit route choice plus visible route search/recovery. Frontend stream
   reducers must ignore retired `provider_fallback` workflow phases entirely
   instead of rendering compatibility "fallback rejected" rows, because the
   selected-route retry/error contract is the only live recovery path until an
   operator explicitly chooses another route. The Assistant
   `/model` command therefore has two owned paths: an exact `provider:model-id`
   argument selects that route directly, and an OpenCode-style
   `provider/model-id` argument is accepted only when the provider prefix is a
   known Pulse provider before being normalized back to canonical
   `provider:model-id` storage. Partial or unknown slash text opens the shared
   model picker with the search text prefilled instead of becoming a hidden
   route guess.
   Assistant model-selection defaults are settings-owned: the drawer may persist
   explicit model selections only for concrete session IDs, while blank-session
   chat defaults must flow from `/api/settings/ai` `chat_model` or `model`.
   Browser storage must not keep a hidden `__default__` model route that
   overrides the configured chat model after the operator changes provider
   settings; stale blank-session defaults must be ignored and cleaned on mount
   so routes such as OpenRouter become visible and effective immediately.
   Assistant session model continuity is transcript-owned, not ambient UI state.
   `internal/ai/chat` must persist the effective selected model route on the
   user turn as well as final assistant turns, and the browser chat runtime must
   restore the latest explicit `provider:model` route from loaded session
   messages before publishing the loaded session ID to drawer/session storage.
   During an active Assistant turn, the drawer must keep the effective model
   route visible next to the live turn status using the route recorded on the
   streaming assistant message first and the selected route only as a fallback;
   live progress text remains the activity description, while route identity is
   separate compact chrome so provider/model state stays inspectable without
   polluting status copy. Provider-start copy must stay route-neutral
   ("Waiting for assistant.") because selected-route identity belongs in the
   route chrome and stream metadata, not in transient activity text; retry copy
   must say Pulse is retrying the selected route, not imply automatic provider
   or model fallback.
   Legacy bare model names, provider response IDs without a Pulse route prefix,
   URLs, and malformed route strings must be ignored so loading older sessions
   cannot corrupt the active selector. The referenced OpenCode source at fetched
   `origin/dev` commit `2006259a02a87edf9e37f253cbddf3188309026b` restores
   local session model state from the latest user message in
   `packages/app/src/pages/session.tsx` lines 361-368 through
   `packages/app/src/pages/session/session-model-helpers.ts`; Pulse adapts that
   pattern by using its persisted explicit route contract rather than terminal
   provider/model objects.
   Follow-up sends during an active Assistant response are chat-runtime queue
   state that steers the running response by default. The drawer must accept
   and echo the user's follow-up as a queued user turn without aborting or
   replacing the active model stream, must show an itemized composer-adjacent
   queue with per-follow-up edit/remove controls plus clear-all, and must
   offer each follow-up to the running loop through
   `POST /api/ai/sessions/{id}/steer` (mid-turn steering). A steering-accepted
   follow-up joins the loop at its next turn boundary (the abort-check site in
   `agentic.go executeWithTools`) as a plain user message, never mid-provider-
   stream and never mid-tool-batch; the loop announces the injection with a
   `steer_applied` stream event carrying the client row id so the drawer
   settles the pending row into a delivered user turn, splices the message
   into `resultMessages` at its true position, and persists it through the
   end-of-run save (whose skip-user-messages rule exempts `Steered` messages).
   Steering carries prompt text only: it cannot change the model route,
   control level, or autonomous mode of the running turn, does not extend the
   turn budget or reset wrap-up brakes, and is rejected for system sessions.
   Once accepted for steering, a follow-up row loses its edit/remove
   affordances (the text is in the loop's hands). Delivery is not guaranteed:
   a run that finishes before a boundary discards unconsumed steers without
   persisting them, and the drawer, which keeps the row queued until
   `steer_applied` arrives, drains it in order after the active stream
   becomes idle exactly as before — the pre-steering queue semantics remain
   the fallback path, and paused queues do not steer. Queued follow-ups must
   snapshot the effective
   model route at enqueue time so a later model/provider switch cannot silently
   reroute an already-queued user turn, and both the transcript queued-user row
   and composer-adjacent queue row must surface that snapshotted route label when
   it exists; explicit failed-turn recovery actions labelled from a configured
   provider route must pass that selected route as an override instead of
   relying on ambient selector state. The referenced
   OpenCode source at fetched `origin/dev` commit
   `1399323b78a04229d9bfe00c7436d7f41770fda8` keeps current, recent, and
   selected models as structured `{ providerID, modelID }` values in
   `packages/opencode/src/cli/cmd/tui/component/dialog-model.tsx` and
   `packages/opencode/src/provider/provider.ts`, so Pulse must preserve the
   equivalent route identity for queued and retried turns. Queue-drain UX
   verification must have a deterministic local fixture path so agents can
   test queued follow-up echo, hold, drain, and tool-row replacement without
   opening a real provider stream. The referenced OpenCode source at fetched
   `origin/dev` commit `31c099be435d59bd6749ace7a9f2bb2245e6d3fa` emits
   local demo tool events through
   `packages/opencode/src/cli/cmd/run/demo.ts` (`startTool` and `doneTool`,
   lines 451-525; demo command routing, lines 1039-1076); Pulse adapts that by
   keeping `/fixture queue-hold` and `/fixture queued-follow-up` fully local
   browser fixtures for queue interaction proof. The referenced OpenCode source
   at fetched `origin/dev` commit
   `e82542b8023a8374f29c23b70ec019c8f256354e` exposes queued prompts through
   `RunQueuedPromptSelectBody` in
   `packages/opencode/src/cli/cmd/run/footer.command.tsx` lines 588-650:
   queued rows are searchable and keyboard-operable, with Enter/Ctrl+E editing
   the selected prompt and Delete/Ctrl+D removing it. Pulse adapts that pattern
   to the browser drawer by making each composer-adjacent queued follow-up row
   focusable and directly operable with Enter to edit and Delete/Backspace to
   remove, while retaining the visible icon buttons and avoiding terminal-only
   shortcuts that collide with browser chrome. OpenCode's `DialogModel` feeds
   current, recent, favorite, and provider model rows into
   `DialogSelect`, while `DialogSelect` maintains a selected row and handles
   up/down/page/home/end/return navigation. The fetched OpenCode
   `origin/dev` commit `31c099be435d59bd6749ace7a9f2bb2245e6d3fa` keeps those
   bindings in `packages/tui/src/ui/dialog-select.tsx` lines 288-378. Pulse's
   browser model picker must expose the same route identity as a labelled
   search/listbox surface, focus search on open, move keyboard focus from
   search into the current or filtered model row, and keep every actionable
   list row, including catalog-disclosure rows, navigable without requiring
   mouse clicks. The 2026-06-07 model-provider slice rechecked OpenCode
   `origin/dev` commit `914a643`: `packages/app/src/components/dialog-select-model.tsx`
   exposes provider connection and model management actions directly inside
   the model picker, while `packages/tui/src/component/dialog-model.tsx`
   exposes a provider action in the same dialog. Pulse adapts that by wiring
   the Assistant model chooser's provider action to the governed Assistant &
   Patrol provider settings route instead of hiding provider repair behind a
   separate slash command or silently switching model routes. The same
   2026-06-07 activity-visibility pass rechecked OpenCode
   `packages/app/src/pages/session/message-timeline.tsx` and
   `message-timeline.data.ts`: active turns are represented as session timeline
   rows, with busy/retry state kept visible while assistant parts continue to
   arrive. Pulse adapts that by keeping fast command/tool completions in a
   transient running state long enough for the compact transcript row to paint,
   while still collapsing large command output behind the existing tool-details
   affordance. Stop is the explicit
   interruption path: it must abort the active stream, clear queued follow-ups
   and pending tool/approval/question affordances, preserve any partial model
   text, return focus to the composer, and render a neutral transcript marker
   rather than persisting synthetic assistant answer text or surfacing the
   interruption as a retryable provider failure.
   The referenced OpenCode source at commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9` routes prompt interruption
   through the prompt command in
   `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx`
   (`session.interrupt`, lines 379-399) and handles stream interruption in
   `packages/opencode/src/session/processor.ts` (`Effect.onInterrupt`, lines
   977-983; aborted tool state finalization, lines 888-915). Pulse adapts that
   terminal active-part behavior by clearing unresolved pending tool,
   approval, and question transcript rows on explicit Stop while preserving
   already-streamed answer text and completed tool evidence before showing the
   neutral stopped marker.
   The same terminal cleanup applies to non-interrupt terminal paths. When a
   stream reaches `done`, emits a structured `error`, or rejects before an SSE
   terminal event, the drawer must not leave pending tool, approval, or
   question controls live in the transcript. The referenced OpenCode source at
   commit `9ed17da55ab1f7360cc0e01075f763e27fa899e9` mutates active tool
   parts into terminal `success` or `failed` rows in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`
   (`session.next.tool.success`, lines 328-348;
   `session.next.tool.failed`, lines 350-371), and the processor marks
   interrupted active tools terminal before completing the assistant message in
   `packages/opencode/src/session/processor.ts` (lines 888-917). Pulse adapts
   that by retaining completed tool rows and assistant text while removing
   unresolved interactive rows from terminal browser transcript state.
   Completed tool rows must preserve timing once a pending row resolves. The
   referenced OpenCode source at fetched `origin/dev` commit
   `effd27b23900720a53e965396ff1a105c1f7e9c8` carries tool identity and
   state through `toolCommit` / `startTool` / `doneTool` in
   `packages/opencode/src/cli/cmd/run/session-data.ts` (lines 622-733) and
   formats elapsed tool state with `span` in
   `packages/opencode/src/cli/cmd/run/tool.ts` (lines 191-200). Pulse adapts
   that by carrying the stream event start/end timestamps into the completed
   browser tool row, so the visible activity timeline does not collapse when a
   running command is replaced by its completed result. When a live browser
   stream receives a `tool_start` and successful `tool_end` in the same
   buffered burst, the completed row may briefly present as running before it
   settles to completed, matching OpenCode's perceptible pending-tool motion
   at fetched `origin/dev` commit
   `1025540fcc2a69609a0131a7168300205656d728` in
   `packages/ui/src/components/basic-tool.tsx` (`pending`, lines 90 and
   195-205) and `packages/ui/src/components/tool-status-title.tsx` while
   keeping Pulse's typed completed tool row, persisted status, answer stream,
   and failure state authoritative. That settle interval must be bounded,
   live-only, success-only, and must not delay assistant content, terminal
   cleanup, or failed-tool visibility. Completed browser tool
   rows must also preserve output trust when a raw preview is intentionally
   suppressed: the referenced OpenCode source at fetched `origin/dev` commit
   `1025540fcc2a69609a0131a7168300205656d728` keeps output visibility as an
   explicit tool-view policy in `packages/opencode/src/cli/cmd/run/tool.ts`
   (`ToolView.output`, lines 40-44; tool display hooks, lines 117-123), and
   commits allowed tool output into scrollback in
   `packages/opencode/src/cli/cmd/run/session-data.ts` (lines 960-964). Pulse
   completed rows must expose safe bounded plain-text output previews when they
   can be rendered without turning the transcript into a raw terminal dump,
   while keeping the full raw output behind the existing details disclosure.
   Structured, binary, unavailable, or otherwise noisy output may collapse to
   compact output metadata. Raw input/output details must remain copyable from
   the disclosure without expanding the default transcript.
   Completed Assistant context summaries must use the same operator-facing
   tool-input presentation as the visible tool rows, not raw internal tool
   identifiers. A completed `pulse_read` device inspection should therefore
   summarize as the inspected resource/action, while raw tool names such as
   `pulse_read` or bare fallback labels such as `read` remain fallback-only
   debug/detail vocabulary.
   Live workflow activity is also active turn state, not disposable waiting
   copy. The referenced OpenCode source at fetched `origin/dev` commit
   `1025540fcc2a69609a0131a7168300205656d728` defines model, shell, step,
   and tool lifecycle events in `packages/core/src/session/event.ts` (model
   switched lines 62-70; shell started/ended lines 151-174; tool lifecycle
   lines 340-392), projects them into durable message/part rows in
   `packages/core/src/session/message-updater.ts` (step started lines
   189-210; tool called/progress/success/failed lines 269-335), and renders
   model-switch plus tool rows in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`
   (model switch lines 120-122 and 262-274; tool rows lines 455-500).
   Pulse adapts that by keeping the latest browser `workflow_status` activity
   visible in the live stream event sequence across content, tool, approval,
   and question boundaries instead of dropping it the instant a richer row
   arrives. Burst workflow statuses may still replace one another until a
   durable stream boundary appears, and terminal cleanup on done/error/Stop
   may remove transient workflow-status rows while preserving typed
   model-switch and tool evidence. When the selected model route is already
   known at turn creation, Pulse must seed a selected-model stream event before
   backend/provider activity arrives so the transcript and active-turn footer
   show concrete route motion instead of a generic request-start wait.
   If backend/provider startup immediately confirms a different selected route
   before any durable assistant content, thinking, tool, approval, or question
   evidence arrives, that confirmed route replaces the provisional local
   selected-model row rather than stacking multiple "Using ..." rows. Once a
   durable boundary exists, later route events are preserved chronologically.
   Rendered workflow-status rows, active-turn footer text, and transcript
   export status lines must normalize internal tool identifiers such as
   `pulse_query`, `pulse_read`, and `pulse_exec` into operator-facing activity
   labels, while preserving the raw typed event payload for Details/debug paths.
   Provider retry states are a distinct live activity kind: a `provider_retry`
   workflow status must render as retrying in the active-turn footer and live
   workflow row so the operator sees selected-route recovery in progress rather
   than another generic waiting/thinking state.
   Assistant-authored visible answer prose is part of the same boundary: model
   text may describe the action in operator vocabulary such as "read command"
   or "inventory lookup", but raw internal identifiers such as `pulse_read`
   must not appear in the normal transcript, copied answer text, or transcript
   export unless the operator explicitly opens raw details/debug content.
   Selected-model route rows are typed route evidence, not live activity. They
   may render immediately while the backend/provider stream is still quiet, but
   they must not hide or outrank workflow progress once a `request_start`,
   provider, tool, thinking, approval, question, or content activity is
   available. This follows the referenced OpenCode separation between session
   status and model/tool transcript events at fetched `origin/dev` commit
   `31c099be435d59bd6749ace7a9f2bb2245e6d3fa`: Pulse keeps the selected route
   visible as durable evidence while the active-turn footer and live workflow
   rows answer what the assistant is doing now. The generated frontend SSE
   contract must include workflow `provider` and `model` fields so
   selected-route activity is typed end to end.
   The transcript viewport is part of the same live activity contract. The
   referenced OpenCode source at fetched `origin/dev` commit
   `1025540fcc2a69609a0131a7168300205656d728` forwards reducer commits into
   terminal scrollback in
   `packages/opencode/src/cli/cmd/run/stream.ts` (lines 140-145) and renders
   bottom-sticky activity in
   `packages/opencode/src/cli/cmd/run/footer.subagent.tsx` (lines 148-153)
   with `stickyScroll={true}` and `stickyStart="bottom"`. Pulse adapts that
   by tracking whether the browser transcript is pinned to the live bottom
   before streamed content or tool rows grow, continuing to follow active
   output while pinned, and preserving the operator's scroll position after
   they intentionally scroll away from live activity. When the transcript is
   unpinned while messages exist, the drawer must expose an accessible
   jump-to-latest command that returns the operator to the live bottom without
   requiring a manual scroll gesture.
   Composer prompt history is also drawer-local chat-runtime state: the drawer
   may persist a bounded local history of submitted prompt text and structured
   mentions for ArrowUp/ArrowDown recall, but that history must not persist or
   replay one-shot finding handoff, approval, autonomous-mode, or other scoped
   send options. The referenced OpenCode source at fetched `origin/dev` commit
   `ba57718b0516c7a8670d1e820b1a24146a8b8262` binds prompt history navigation
   in `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx` to the
   input cursor boundary: previous history is available at the start of the
   current draft, and next history restores the saved draft at the end. Pulse's
   Assistant drawer follows that draft-safe recall model rather than limiting
   history recall to an empty composer. Once a history entry is loaded, the
   same boundary ownership remains direction-specific: ArrowUp may move to an
   older prompt only from the start edge, and ArrowDown may move newer or
   restore the saved draft only from the end edge, so normal textarea movement
   is not stolen at the opposite boundary. Scoped context replay remains owned
   by explicit session handoff metadata or queued follow-up edit state.
   Unsent composer drafts are recoverable remount state, not prompt history:
   closing or remounting the Assistant drawer must restore the current prompt,
   structured mentions, cursor position, and any queued-follow-up edit metadata
   without writing the draft into submitted prompt history. The referenced
   OpenCode source at commit `9ed17da55ab1f7360cc0e01075f763e27fa899e9`
   keeps a module-local `stashed` prompt in
   `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx`, restores it
   in `Prompt` `onMount` (lines 595-604), and captures it in `onCleanup`
   (lines 607-610). Pulse adapts that source behavior with a drawer-local
   composer stash so normal close/reopen does not drop an unsent user thought
   or downgrade a scoped queued follow-up into a plain prompt.
   Composer submission must read the live input buffer at submit time and
   suppress only overlapping dispatches from the same browser interaction. The
   referenced OpenCode source at commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9`
   `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx` guards
   `Prompt.submit()` with local `submitting` state (lines 910-924) and syncs
   the live input `plainText` into prompt state before downstream reads
   (lines 930-936) so double-dispatch and IME/composition races do not send a
   stale or phantom prompt. Pulse adapts that by reading the live textarea
   value before trimming and by using a one-dispatch composer guard that still
   releases immediately for intentional queued follow-ups during an active
   turn.
   Assistant drawer controls are part of that same prompt command surface. The
   referenced OpenCode prompt source exposes command-named actions such as
   prompt submit, session interrupt, prompt history movement, and prompt stash
   operations through explicit command titles before those actions reach the
   terminal UI. Pulse's browser drawer must therefore expose command-specific
   accessible names and selected/expanded state for always-visible chat actions
   such as New session, session history, collapse/close, autonomous-warning
   recovery, and the control-mode selector instead of depending on title-only
   icon controls or ambiguous short labels.
   The slash command autocomplete is a command-discovery surface, not just a
   text shortcut hint. The referenced OpenCode source at fetched `origin/dev`
   commit `914a643` advertises session
   commands such as `/models`, `/new`, `/sessions`, `/compact`, `/export`, and
   `/connect` in `packages/tui/src/feature-plugins/home/tips-view.tsx` lines
   179-192, while each command registration carries a command title, category,
   and slash metadata in `packages/tui/src/routes/session/index.tsx` (for
   `/compact`, lines 548-570). Pulse adapts that by showing the full local
   Assistant command set in the scrollable slash list instead of silently
   capping discovery to the first few commands, and every listed command row
   must carry a matching icon so `/compact`, `/undo`, `/redo`, and other
   lower-frequency actions do not appear as unfinished blank rows. OpenCode's
   current app prompt popover keeps unmatched slash searches inside the prompt
   surface and renders an empty-command state in
   `packages/app/src/components/prompt-input/slash-popover.tsx` (fetched
   `origin/dev` commit `914a643`, lines 96-100); Pulse adapts that by keeping
   genuinely unmatched `/...` drafts open with a clear empty state while still
   hiding disabled-only command matches so manual Enter can explain why the local
   command is unavailable. The slash list must avoid persistent visible keyboard
   shortcut instruction footers; owned key handling is proven by behavior and
   accessible command state instead of extra instructional chrome.
   Slash autocomplete close semantics are owned by that same prompt command
   surface. The referenced OpenCode source at fetched `origin/dev` commit
   `4519a1da329c1a4fc384054e7203ba7d06928205` clears a transient slash
   command query when `hide()` closes prompt autocomplete in
   `packages/opencode/src/cli/cmd/tui/component/prompt/autocomplete.tsx`
   (lines 668-678), so Pulse's browser slash command popup must close Escape
   and outside-click locally, clear only slash-only command drafts, return focus
   to the composer, and leave ordinary prompts or already-submitted local
   command actions untouched. Closing the popup must not leave `/mo`, `/new`, or
   another executable command token behind for the next Enter press.
   Prompt-owned popups must consume their local keyboard commands before global
   drawer handlers can reinterpret them. The same OpenCode autocomplete source
   wraps previous/next selection at list boundaries in
   `packages/tui/src/component/prompt/autocomplete.tsx` lines 552-559 at fetched
   `origin/dev` commit `31c099be435d59bd6749ace7a9f2bb2245e6d3fa`, and
   registers prompt-scoped previous/next/hide/select/complete commands in
   `packages/opencode/src/cli/cmd/tui/component/prompt/autocomplete.tsx`
   (lines 600-658), and OpenCode's help dialog binds Escape as a dialog-local
   close command in
   `packages/opencode/src/cli/cmd/tui/ui/dialog-help.tsx` (lines 11-16).
   Pulse adapts that by having slash autocomplete, resource mention
   autocomplete, and Assistant command help prevent default handling and stop
   later document-level keyboard handlers for their owned navigation,
   selection, and close keys. Slash and resource mention autocomplete must wrap
   up/down movement across the visible options, and resource mentions must expose
   the same selected-row state as a labelled listbox rather than only a visual
   hover fill. Escape inside those local surfaces must close the local
   popup/dialog only; it must not also close the Assistant drawer.
   Failed-turn retry is part of that same local chat-runtime boundary: a
   retryable in-memory assistant error may replay the original user turn's
   structured mentions, finding id, approval override, handoff resources,
   handoff actions, and handoff metadata, but must not reconstruct scoped
   context from prompt history or saved transcript prose. Provider/model route
   recovery is explicit user-visible recovery, not hidden chat execution. A
   failed turn may expose route-and-retry actions through the existing drawer
   model selector so operators can move from a blocked direct provider route to a
   configured gateway or alternate model without losing the draft or creating a
   parallel picker, but `/api/ai/chat` must not automatically switch provider or
   model routes inside a single Assistant turn after the selected route fails.
   That prohibition includes same-model gateway equivalents and configured
   provider defaults: those routes may be offered as explicit recovery actions,
   but the selected turn owns its selected route until it completes or fails.
   The referenced OpenCode source at fetched `dev` commit
   `e82542b8023a8374f29c23b70ec019c8f256354e` models this boundary by keeping
   same-route retry visible in `packages/opencode/src/session/retry.ts`,
   publishing retry status from `packages/opencode/src/session/processor.ts`,
   rendering retry state in `packages/ui/src/components/session-retry.tsx`, and
   treating provider/model changes as explicit session events in
   `packages/core/src/session/event.ts` and
   `packages/opencode/src/session/prompt.ts`. Pulse adapts that behavior for the
   drawer instead of implementing automatic cross-provider route switching.
   Primary interactive chat model resolution must use the explicit configured
   chat route or, when no explicit chat route exists, a stable provider default
   without calling provider model catalogs before the selected stream starts. If
   an explicit chat route is unconfigured, retired, or recognized as a
   specialized non-chat endpoint, `/api/ai/chat` must fail that selected route
   visibly instead of substituting a same-provider default. Catalog-backed
   recommendation belongs to settings/model-list and explicit route-recovery
   flows, not `/api/ai/chat` first-response startup. Once the selected provider
   route starts, transient pre-output transport failures may retry the same route and
   must surface a `provider_retry` workflow state with `attempt`,
   `max_attempts`, and `retry_after_ms` before sleeping. If same-route retry is
   exhausted before visible output, Pulse must emit a normal provider error and
   leave recovery to the failed-turn actions; it must not emit
   `provider_fallback`, `failed_provider`, `failed_model`, `next_provider`, or
   `next_model` workflow metadata. Once visible output has streamed, Pulse must
   also keep the failure on that visible attempt rather than silently changing
   route mid-turn. Frontend reducers must discard stale or nonconforming
   `provider_fallback` workflow metadata without rendering a rejected-route
   status, switching routes, or inferring a model switch from `next_model`.
   Provider retry
   workflow states that include `retry_after_ms`
   must render as live countdowns derived from the workflow `started_at`
   timestamp while the turn is active, matching OpenCode's visible retry wait
   behavior instead of freezing the first retry label.
   Assistant completion events must carry the effective model route that actually
   completed the selected turn. The drawer must render the initial
   `provider_start` workflow state as a selected-model row (`Using ...`) before
   visible content or tool activity when the event arrives. Later model-route
   changes may still be rendered as typed `model_switch` transcript rows when
   they come from explicit route recovery or restored historical data, but the UI
   must label them neutrally as selected or switched routes rather than as
   automatic provider fallback, and completing a streamed route-switch row must
   not promote that route into the active model selection without an explicit
   user action.
   Interactive Assistant streams must establish the session ID and emit the
   `session` event once as soon as the HTTP SSE writer is ready, before finding
   handoff recovery, model resolution, selected-provider startup,
   handoff/context prefetch, recent-session injection, inventory summary reads,
   tool scoping, or provider startup. The chat service then persists/ensures
   that same session ID while suppressing duplicate session events. This keeps
   the drawer's visible turn anchored while backend preparation continues and
   prevents simple prompts from appearing stuck in an opaque thinking state.
   Stored finding or handoff recovery is eligible only for a session ID supplied
   by the browser as an existing conversation. A session ID generated at the
   HTTP stream boundary for a new turn must not trigger persisted handoff
   lookups or legacy session discovery before the service creates the session,
   because that turns a new prompt into a large-history storage scan before
   the runtime can even report progress.
   After the stream is anchored, pre-model work must keep emitting neutral
   `workflow_state` progress for the long phases that can otherwise look dead:
   context preparation, bounded canonical inventory reads for count/overview
   turns, and selected-provider startup. These events are runtime progress, not
   Assistant-authored analysis, and they must not become keyword routers,
   explore pre-passes, or instructions that choose the model's next action.
   Backend-emitted `workflow_state` rows that explain real provider, context, or
   tool progress must remain visible while the turn is active, but they are
   transient runtime state rather than completed Assistant evidence. Terminal
   `done`, terminal `error`, and explicit Stop must clear workflow-status rows
   together with local prompt placeholders and unresolved interactive rows such
   as approvals, questions, and pending tools, while preserving durable
   model-route, content, reasoning, completed-tool, canceled-tool, and error
   evidence. The referenced OpenCode
   source at fetched `origin/dev` commit
   `914a643ab26053df463ef78a1deadcdb223e8783` writes step, tool, and text parts
   as soon as they arrive in `packages/opencode/src/session/processor.ts` lines
   151-162, 322-339, 443-466, and 787-806, then renders completed text parts
   from stored part state in `packages/ui/src/components/message-part.tsx` lines
   1545-1551; Pulse adapts that part-style visibility by keeping live workflow
   activity visible without storing stale provider-start or waiting labels as
   completed transcript evidence.
   Selected-provider startup copy must describe the provider route as actively
   starting the response (for example, `<Provider> is starting the response.`)
   rather than as a passive wait label, while preserving the same
   `provider_start` phase and concrete model-route payload.
   The HTTP chat stream handler also owns visible idle progress for silent
   intervals after the stream has opened: if no client-visible Assistant event
   reaches the browser for the governed idle interval while execution is still
   request-bound, the handler must emit a neutral `workflow_state` with phase
   `stream_idle` rather than relying only on hidden SSE comment heartbeats.
   Legacy Assistant SSE routes that still use the older execute event envelope,
   including `/api/ai/execute/stream` and `/api/ai/investigate-alert`, share
   the same transport-liveness obligation while preserving their existing
   response payload shapes.
   Comment heartbeats and visible events must share a serialized writer so a
   progress tick cannot interleave bytes with a provider/tool event. This is a
   transport liveness signal only; it must replace the active workflow status,
   stop before terminal `done`/`error`, and must not become persisted assistant
   prose or raw provider reasoning.
   The drawer transcript owns detailed in-flight progress for the active turn:
   a new assistant row must start with a neutral local preparation status until
   stream progress arrives, must show the current effective model route while
   the turn is still streaming, and must keep late workflow progress visible
   through the composer footer while transcript rows continue to render typed
   Assistant evidence. Workflow status is live
   progress, not answer content or a delayed walkthrough; each new
   `workflow_state` replaces the canonical active status immediately. The
   browser may retain a bounded in-flight history of those replaced labels for
   state continuity. When several statuses arrive in one backend/network burst,
   the live transcript row and active-turn composer footer must use the same
   paced presentation helper so the operator sees the row move through
   preparation, context, provider, and tool states instead of jumping to the
   final burst label. Single fresh status updates still render immediately, and
   separated transcript workflow rows stay tied to their own streamed event
   when a durable content/tool/approval/question boundary exists. Older neutral
   statuses may disappear when replaced by newer status, tool, reasoning, or
   answer evidence; that replacement is the intended motion signal.
   That history must stay live-only: once visible assistant text, tool progress,
   approvals, questions, terminal `done`, terminal `error`, or explicit Stop
   take over, stale workflow text and the presentation history must clear so
   the row does not keep saying it is waiting on a phase that has already been
   superseded. Live workflow and pending-tool activity must retain a per-state
   start timestamp so the drawer can show elapsed wait/run time for long
   provider starts and tool calls instead of repeating a timeless waiting label.
   Completed plain-text tool output must keep a bounded transcript preview
   visible by default, with full input/output available through expansion, so
   the operator sees evidence of what completed without receiving a raw output
   dump. Structured, binary, unavailable, or otherwise noisy output may stay
   collapsed behind details, but a successful text command must not degrade to
   an opaque `output available` badge when a safe preview can be rendered.
   If the UI enters a loading turn before the assistant shell or first
   `workflow_state` exists, the active-turn footer must show the active startup
   status `Sending prompt.` and derive elapsed time from the submitted, non-queued
   user turn or assistant shell timestamp. Queued follow-up messages must not
   reset that active-turn startup clock. The same active-turn footer owns the
   Stop affordance while a response is running, so interruption remains attached
   to the live status row instead of hiding only in the composer action cluster.
   The referenced OpenCode source at fetched `origin/dev` commit
   `0875203a6c726d7a37b5ffbb770cc433c98e7cd6` mutates typed message parts as
   events arrive in `packages/opencode/src/cli/cmd/tui/context/sync.tsx`
   (`message.part.updated` and `message.part.delta`) and represents session
   activity as separately updated status in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`
   (`session.next.step.started`, `session.next.text.delta`, and tool state
   events). Pulse adapts that event-by-event activity model with browser-safe
   `workflow_state` and typed tool rows, including replacing `stream_idle`
   status while no provider event has arrived, rather than exposing terminal
   UI internals or chain-of-thought.
   The referenced OpenCode source at fetched `origin/dev` commit
   `1399323b78a04229d9bfe00c7436d7f41770fda8` applies each typed event to the
   active assistant message in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`; Pulse adapts that
   precedence by letting typed content, model-switch, tool, approval, and
   question evidence own the visible row once it exists, while later neutral
   workflow states such as provider reasoning still replace the live footer
   heartbeat instead of being hidden behind generic "generating" copy.
   The referenced OpenCode source at commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9` renders active assistant work
   as session-owned parts in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx`
   (`Session`, line 185; `ToolPart` mapping, lines 1788-1796;
   `InlineToolRow`, line 1926) and mutates the matching active tool/text state
   in `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`
   (`apply(event)`, line 120; `session.next.tool.*`, lines 276-372);
   Pulse's browser drawer adapts that model by keeping the transcript rows
   typed and stable while the active-turn footer remains a replacing live
   status slot for provider and workflow progress between visible parts.
   The same OpenCode commit applies each streamed session event as its own
   state mutation in `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`
   (`apply(event)`, line 120; `session.next.tool.input.started`, line 276;
   `session.next.tool.progress`, line 317; `session.next.tool.success`, line
   328; `session.next.tool.failed`, line 350), so Pulse's shared browser SSE
   consumer must treat opted-in Assistant progress events as paint checkpoints.
   Pulse's active-turn status ranking must also treat terminal `tool_end`
   events as fresh visible activity, adapting OpenCode's in-place
   completed/error tool-part mutation by replacing stale provider-start footer
   copy with completed or failed tool activity while still preferring any
   separate unresolved pending tool over a terminal sibling tool.
   Read-only shell-backed Assistant tools must expose the redacted `$ ...`
   command preview in the active-turn status while the tool is running and when
   its terminal row replaces stale workflow copy; friendly intent labels may
   remain inside tool cards, but the live dock must not hide a known read-only
   command behind generic "running command" text. Write/control commands remain
   governed by approval and action surfaces rather than gaining a free-form live
   shell shortcut. The referenced OpenCode source at fetched `origin/dev` commit
   `e82542b8023a8374f29c23b70ec019c8f256354e` records shell commands as visible
   commits in `packages/opencode/src/cli/cmd/run/session-data.ts`
   (`startShell`, lines 690-702; `session.next.shell.started`, lines 783-795;
   `session.next.shell.ended`, lines 803-821) and keeps non-shell tool status
   visible through `toolStatus` on running tool parts (lines 932-942). Pulse
   adapts that shell visibility by reusing its redacted read-only command
   preview in the browser-safe active-turn status instead of exposing raw
   outputs, secrets, or unrestricted control commands.
   `frontend-modern/src/api/streaming.ts` must yield through an
   animation-frame-backed browser paint checkpoint, with a bounded timer
   fallback for inactive tabs, before draining the next opted-in event; a plain
   synchronous loop or microtask-only pause is insufficient because it can still
   render workflow/tool steps only after a batch has already finished. Chat text
   and reasoning deltas must paint immediately for the first delta in a run and
   then periodically so coalesced provider chunks do not arrive as one burst,
   while avoiding a frame wait for every token.
   Pulse's reducer must also keep the same keyed-part behavior: repeated
   `tool_start` or `tool_progress` events for one backend tool ID, or the same
   normalized tool name when an older server omits IDs, upsert one pending tool
   row and collapse stale duplicate pending rows instead of replaying several
   near-identical steps in the transcript.
   `frontend-modern/src/api/aiChat.ts` owns the Assistant predicate: token
   content and hidden reasoning may continue to opt out of those checkpoints so
   answer streaming remains fast, while session, workflow, model-switch, tool,
   approval, and question events remain user-visible progress checkpoints.
   That same frontend stream boundary owns the deterministic dev/test fixture
   used for fast Assistant UX iteration: `frontend-modern/src/api/aiChat.ts`
   may short-circuit an explicit local fixture prompt only when
   `import.meta.env.DEV` or test mode is active, and the fixture must emit the
   same typed `StreamEvent` objects (`session`, `workflow_state`, `thinking`,
   `tool_start`, `tool_progress`, `tool_cancel`, `tool_end`, `content`,
   `done`) through the normal `useChat` reducer rather than rendering a
   separate mock transcript. The local `provider-retry` fixture must exercise
   the real `provider_retry` workflow state, including attempt metadata and
   `retry_after_ms`, so browser proof of retry countdown behavior does not
   depend on external provider availability or API spend.
   The local `tool-chain` fixture must exercise consecutive tool activity with
   paced `tool_start` -> `tool_progress` -> `tool_end` transitions for one
   tool followed by a second live tool, so browser proof can verify completed
   rows compact while the next tool row replaces the active motion.
   Dev/test fixture prompts are also part of the Assistant command-discovery
   surface: slash autocomplete and Assistant command help must expose an
   insertable `/fixture` command in development/test builds, search it by
   canonical fixture names such as `provider-retry`, and submit completed
   `/fixture <name>` prompts through the normal chat send path so
   `maybeRunAIChatDevStreamFixture` remains the single local execution boundary.
   Production command surfaces must omit that dev command instead of advertising
   a fixture mode that the runtime will not intercept.
   The local `stream-idle` fixture must exercise the real `stream_idle`
   workflow state after selected-provider startup so browser proof of visible
   idle liveness does not depend on making a real provider pause on demand.
   The local `send-hold` fixture must exercise the local prompt-send boundary
   by emitting the session event and then holding before the first backend
   `workflow_state`, so browser proof can capture `Sending prompt.` without
   depending on real provider latency.
   This fixture is a local development primitive, not a product mode: it must
   not bypass backend governance in production, persist as a server session, or
   introduce any UI-only event shape that real providers cannot emit.
   Live answer text must be paced in the presentation layer, not throttled or
   mutated in the canonical stream state: appended content is revealed in short
   readable slices while the turn is streaming, and completed/restored messages
   render immediately from the full stored content. This adapts OpenCode's
   `createPacedValue` live markdown pattern in
   `packages/ui/src/components/message-part.tsx` so Pulse feels active without
   delaying copy/export, tool boundaries, approvals, or audit evidence.
   Live workflow labels follow that same presentation-layer constraint: the
   canonical SSE/store state remains the latest typed `workflow_state`, and the
   browser must render single fresh status updates immediately while using
   bounded live-only history only to pace backend/network burst replacements.
   This follows the current OpenCode TUI event mutation model at fetched
   `origin/dev` commit
   `06d7840d1d42c9815d2d2e45e7fa4090ca4e3577`, especially
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`, where
   `session.next.step.started`, text deltas, and tool progress mutate the
   current assistant state event-by-event. Pulse uses the bounded status
   history only to preserve reducer continuity, not to delay audit evidence,
   approvals, tool boundaries, transcript export, or completed/restored
   messages.
   Completed Assistant tool rows follow the same source-anchored display policy:
   the referenced OpenCode commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9` keeps ordinary tool activity terse
   through `GenericTool`/`InlineToolRow` in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx`, hides generic
   output unless `showGenericToolOutput` is enabled, and bounds output-heavy
   blocks with `packages/opencode/src/cli/cmd/tui/util/collapse-tool-output.ts`.
   Pulse adapts that by keeping the completed tool action visible in the
   transcript, rendering only short safe plain-text output previews inline, and
   keeping structured JSON, unavailable output, noisy output, and the full raw
   payload behind Details. This preserves evidence for inspection without
   letting command output compete with the Assistant answer flow.
   The row itself owns the details trigger, not a tiny adjacent text action:
   completed tool rows with inspectable input or output must expose
   `aria-expanded`, keyboard activation, and a chevron affordance on the row
   header so opening evidence feels like expanding the tool part. That adapts
   OpenCode's `BasicTool` collapsible trigger in
   `packages/ui/src/components/basic-tool.tsx`, where the trigger row and
   arrow own expansion while pending/running tools remain locked.
   Tool-row summaries are part of that same source-anchored contract, not
   assistant prose cleanup: when provider or backend transport supplies a
   function-style call such as `pulse_read(...)` or a friendly string backed by
   structured raw input, Pulse must derive the visible row from the parsed tool
   arguments and keep the raw transport form behind Details. This adapts
   OpenCode's `ToolPart`/`InlineToolRow` rendering in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx` and the
   `session.next.tool.input.*` mutations in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`: typed tool state
   owns the action row, while raw invocation syntax is inspection detail.
   The live active-turn footer is part of the same contract: pending tools must
   reuse the tool-row action/command summary instead of falling back to generic
   tool names such as `read` or `query`. This keeps the always-visible progress
   surface aligned with OpenCode's visible `ToolPart` state rather than hiding
   the concrete command until the transcript row catches up.
   Streaming thinking rows follow the same source-anchored reasoning-display
   contract: OpenCode commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9` renders reasoning through
   `ReasoningPart`/`ReasoningHeader` in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx` and extracts only
   provider summary metadata with `reasoningSummary` in
   `packages/opencode/src/cli/cmd/tui/context/thinking.ts`. Pulse adapts that by
   showing a live `Thinking:`/completed `Thought:` row with duration and optional
   provider summary title while keeping the raw reasoning body out of the
   transcript.
   Provider reasoning must not become fallback assistant answer text when an
   OpenAI-compatible non-stream response contains `reasoning` or
   `reasoning_content` without visible `content`. The referenced OpenCode
   source at fetched `origin/dev` commit
   `4519a1da329c1a4fc384054e7203ba7d06928205` maps AI SDK
   `reasoning-start` / `reasoning-delta` / `reasoning-end` events to
   dedicated reasoning events in
   `packages/opencode/src/session/llm/ai-sdk.ts` (lines 158-188), preserves
   assistant reasoning as a `type: "reasoning"` message part in
   `packages/opencode/src/session/message-v2.ts` (lines 374-387), and keeps
   text and reasoning renderability distinct in
   `packages/ui/src/components/message-part.tsx` (lines 607-615). Pulse adapts
   that boundary by keeping `ChatResponse.Content` limited to provider
   `message.content` and storing OpenAI-compatible reasoning only in
   `ReasoningContent`, so hidden provider reasoning cannot be rendered as the
   final answer by any Pulse surface that consumes non-stream chat responses.
   The active-turn status strip follows the same source-backed part freshness
   rule: OpenCode commit `9ed17da55ab1f7360cc0e01075f763e27fa899e9`
   updates live assistant parts in place through
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`
   (`latestTool`, `latestText`, `latestReasoning`, and `apply(event)`),
   renders reasoning headers with `ReasoningPart`/`ReasoningHeader` in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx`, and keeps the
   prompt footer as the replacing live status surface in
   `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx`. Pulse adapts
   that by ranking workflow, content, hidden reasoning, and pending-tool footer
   copy by the freshest activity timestamp: a later answer token can replace an
   older tool status, and a later in-place tool progress patch can replace the
   answer status again without moving the transcript's chronological row.
   The active OpenCode-parity Assistant goal requires future parity slices to
   reference OpenCode's actual source implementation before changing Pulse
   behavior. Each slice must record the inspected OpenCode commit SHA plus
   source file, symbol/function, and line anchor for the behavior being adapted,
   including message parts, tool-state mutation, progress rendering,
   model/session selection, and prompt/footer interaction. Parity means adapting
   the proven interaction model, not guessing from screenshots or observed
   behavior alone.
   The referenced OpenCode source at fetched `origin/dev` commit
   `54f4974546104bb72f7a0be2b75b92f9defc009b` builds the model dialog from
   provider metadata, favorites, recent models, and provider sections in
   `packages/opencode/src/cli/cmd/tui/component/dialog-model.tsx`, keeps
   structured `{ providerID, modelID }` current/recent/favorite state with
   provider-catalog validation in
   `packages/opencode/src/cli/cmd/tui/context/local.tsx`, and formats the
   effective model label from provider/model metadata in
   `packages/opencode/src/cli/cmd/run/variant.shared.ts`. Pulse's Assistant
   drawer adapts that selector workflow by keeping explicit recent model routes
   above the provider catalog, routing chat-specific default/override options
   through the shared model picker, preserving selected older models in the
   visible list, and accepting custom model entries only when they are explicit
   `provider:model` routes that the backend chat stream can execute. Unknown
   recent or custom Pulse routes may survive catalog hydration only when they
   still have a valid provider/model shape; malformed route strings must not
   become selectable chat routes. The 2026-06-07 fast-model-route slice
   rechecked OpenCode `origin/dev` commit
   `31c099be435d59bd6749ace7a9f2bb2245e6d3fa`: the TUI command registry
   exposes `model.list` as `/models` plus `/mo` and hidden
   `model.cycle_recent` / `model.cycle_recent_reverse` commands in
   `packages/tui/src/app.tsx` lines 725-751, the app command registry exposes
   `model.choose` as `/model` with keybind `mod+'` in
   `packages/app/src/pages/session/use-session-commands.tsx` lines 514-522,
   and `DialogModel` promotes favorites/recents before provider catalog rows
   in `packages/tui/src/component/dialog-model.tsx` lines 23-54. Pulse adapts
   that command model by letting the composer consume `/model <provider:model>`
   as a local route switch, `/model default` as an inherited-default reset, and
   `/model next` / `/model previous` as recent-route cycling without sending
   those command strings to the provider. Malformed `/model ...` arguments must
   remain editable in the composer with a local correction message rather than
   being silently cleared or treated as assistant prompts. The referenced
   OpenCode source at commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9`
   `packages/opencode/src/cli/cmd/tui/component/dialog-model.tsx` passes the
   current structured model route into `DialogSelect` at lines 146-171, and
   `packages/opencode/src/cli/cmd/run/footer.command.tsx` marks the current
   direct-mode model row with a `current` footer at lines 796-804. Pulse adapts
   that by making the shared model picker mark the selected catalog, recent,
   override, custom, or inherited-default route as a visible `Current`
   `role="option"` row with `aria-selected`, instead of relying on background
   color alone.
   The referenced OpenCode source at fetched `origin/dev` commit
   `147169e9b78bdd8430800f883af6b6485e5156e4` runs ordinary follow-up
   prompts through a serial local queue in
   `packages/opencode/src/cli/cmd/run/runtime.queue.ts`: prompts submitted
   behind an active ordinary turn remain visible as queued prompts, expose
   removal through `FooterApi.onQueuedRemove`, and are removed from the visible
   queue before their own turn begins. Pulse's Assistant drawer adapts that
   behavior by keeping queued follow-ups in the transcript, showing queue
   position, snapshotted model-route identity, and edit/remove controls on each
   queued user row, and draining those rows through the existing chat-runtime
   queue without aborting the active model stream.
   The referenced OpenCode source at fetched `origin/dev` commit
   `09d9cf01f93798939c1284fbe974b6e1f4d2759d` registers the
   `session.interrupt` command while a turn is non-idle in
   `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx`, and its
   direct-run footer implements the same two-press interrupt guard in
   `packages/opencode/src/cli/cmd/run/footer.ts` while rendering the armed
   state in `packages/opencode/src/cli/cmd/run/footer.view.tsx`. Pulse's
   Assistant drawer adapts that ergonomics model by letting Escape from the
   focused composer arm the visible Stop control first and letting the next
   Escape confirm the same governed `chat.stop()` path as the Stop button,
   including aborting the active stream, clearing queued follow-ups, preserving
   partial text, and returning focus to the composer. The App-level Assistant
   drawer Escape guard must not close the drawer when Escape originates inside
   the shared model picker; model-picker Escape is a local search/listbox close
   path that returns focus to the picker trigger while the drawer remains open.
   The referenced OpenCode source at fetched `origin/dev` commit
   `fa2b63f850fc0a23bec2bdff9e660450d3fe7913` keeps prompt/footer status visible
   only while the session is non-idle in
   `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx`, and maps
   `turn.send`, `turn.wait`, and tool status patches into a live footer in
   `packages/opencode/src/cli/cmd/run/footer.ts` plus
   `packages/opencode/src/cli/cmd/run/session-data.ts`. Pulse's drawer adapts
   that as an active-turn status strip: no idle diagnostics, but while a turn is
   loading it must show waiting, current tool/workflow progress, or generating
   status even when the transcript already contains an in-flight assistant row.
   The referenced OpenCode source at fetched `origin/dev` commit
   `09d9cf01f93798939c1284fbe974b6e1f4d2759d` resolves the direct-run wait when
   `packages/opencode/src/cli/cmd/run/stream.transport.ts` verifies the session
   is idle, maps `turn.idle` to an idle footer phase in
   `packages/opencode/src/cli/cmd/run/footer.ts`, and then flushes scrollback
   separately from later state sync. Pulse's Assistant drawer adapts that
   completion boundary by clearing the visible active turn as soon as the chat
   stream has processed its terminal `done` or `error` event; async
   conversation/session-list refresh may continue afterward, but it must not
   keep the composer footer or transcript row saying the assistant is still
   generating.
   When multiple governed tools are pending, completed tools must not blank the
   status while another tool is still running, and the status heartbeat should
   follow the latest progressed pending tool without reordering the transcript's
   chronological tool rows. Pulse adapts this with a mutable footer status and
   inline pending rows that show the current activity plus elapsed time while
   keeping large command output collapsed.
   The referenced OpenCode source at fetched `origin/dev` commit
   `1399323b78a04229d9bfe00c7436d7f41770fda8` updates the existing typed tool
   part through `updateToolCall` in
   `packages/opencode/src/session/processor.ts` and renders ordered assistant
   content parts through `AssistantMessage`/`AssistantTool` in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`;
   Pulse pending tool progress must therefore mutate the matching row in place
   and carry separate activity freshness metadata for the footer instead of
   moving an earlier active tool row below later tools when progress arrives.
   Assistant experience parity claims against OpenCode must cite the inspected
   OpenCode commit, source file, source symbol/function, and line anchor that
   defines the behavior being adapted; observed UI behavior alone is not
   sufficient governance evidence for future parity slices.
   The referenced OpenCode source at commit
   `f750deaa3e95098fdde5fb00305b273e43c5b2cd` mutates a single tool part from
   `pending` input through `running` progress to completed/error in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`, with tool metadata
   updates flowing through `packages/opencode/src/session/tools.ts`. Pulse's
   stream contract adapts that model with `tool_start`, `tool_progress`, and
   `tool_end` events that update the same visible pending tool row in place.
   The referenced OpenCode source at fetched `origin/dev` commit
   `09d9cf01f93798939c1284fbe974b6e1f4d2759d` creates and mutates tool parts as
   input arrives in `packages/opencode/src/session/processor.ts`
   (`ensureToolCall`, `tool-input-start`, `tool-input-delta`,
   `tool-input-end`), applies those input/running/completion updates in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`
   (`session.next.tool.input.started`, `.delta`, `.ended`, `.called`,
   `.progress`, `.success`, `.failed`), and renders the same live tool part in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx`. Pulse's
   Assistant stream must mirror that timing: model-selected tools become
   visible when the tool name is known, streamed function-argument deltas,
   including Anthropic `input_json_delta` chunks, surface as `tool_progress`,
   later argument/progress updates mutate the pending row, and completion
   replaces that row rather than appending a delayed batch of steps. Waiting
   until `[DONE]`, `message_stop`, or execution completion recreates the
   delayed batch feeling this contract is meant to prevent. Policy-hidden
   placeholder attempts remain governed by the runtime and may resolve the
   pending row into a durable skipped `tool_cancel` row instead of either
   persisting a failed tool card or hiding the activity. If a terminal
   `tool_end` or `tool_cancel` reaches the browser without a matching pending
   row, the frontend must still append terminal tool evidence instead of
   dropping the only visible activity. The 2026-06-07 Pulse slice rechecked
   OpenCode `origin/dev` commit
   `1025540fcc2a69609a0131a7168300205656d728`, where
   `packages/core/src/session/event.ts` and
   `packages/core/src/session/message-updater.ts` keep tool lifecycle events as
   typed message parts through terminal success/failed states; Pulse adapts
   that lifecycle shape by making skipped policy/runtime tool calls visible as
   terminal transcript rows.
   The 2026-06-07 live-tool-evidence slice rechecked OpenCode `origin/dev`
   commit `31c099be435d59bd6749ace7a9f2bb2245e6d3fa`:
   `packages/core/src/session/event.ts` lines 307-366 define live tool input
   and bounded progress events, `packages/core/src/session/message-updater.ts`
   lines 247-293 mutates the same assistant tool part from pending input to
   running progress, and `packages/ui/src/components/basic-tool.tsx` lines
   90 and 246-253 keep pending/running tools as active compact rows rather than
   expanding details by default. Pulse adapts that source model for
   infrastructure operations by keeping the compact live row primary while
   allowing an operator to expand pending/running tool rows for copyable raw
   input and current progress evidence; raw provider envelopes stay behind
   Details and are used there when structured arguments are still placeholder
   `{}` / `[]`. The local dev `/fixture pending-tool` stream is the canonical
   no-provider browser proof fixture for this live pending-tool Details window
   and must keep a paced running state long enough to exercise the row before
   it resolves.
   The 2026-06-07 skipped-evidence slice rechecked OpenCode `origin/dev`
   commit `31c099be435d59bd6749ace7a9f2bb2245e6d3fa`:
   `packages/core/src/session/event.ts` lines 307-336 define replayable tool
   input start/delta/end events and lines 386-399 define terminal failed tool
   events, `packages/core/src/session/message-updater.ts` lines 247-267 and
   318-339 preserve and update the same assistant tool part through input and
   failure, and `packages/opencode/src/session/tools.ts` lines 48-65 and
   180-200 keep terminal tool input/output evidence. Pulse skipped/canceled
   tool rows must therefore keep raw tool input and skip reason behind the same
   expandable, copyable Details panel used for completed terminal tool rows,
   while the compact row remains the operator-readable action summary.
   While streamed arguments are still invalid, incomplete JSON, or incomplete
   provider-style function-call input, the frontend must use the `raw_input`
   fragment to show a safe partial command/path/query summary instead of a
   blank `{}` request row, then replace it with the structured summary once
   parsing succeeds. The 2026-06-07 Pulse slice rechecked OpenCode
   `origin/dev` commit `effd27b23900720a53e965396ff1a105c1f7e9c8`:
   `packages/opencode/src/session/processor.ts` lines 431-451 append
   `tool-input-delta` text to the same call, and
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx` lines 276-294
   render and mutate that pending tool part as input arrives.
   The referenced OpenCode source at fetched `origin/dev` commit
   `1399323b78a04229d9bfe00c7436d7f41770fda8` keeps tool invocations as durable
   message parts in `packages/opencode/src/session/processor.ts`
   (`ensureToolCall`, `updateToolCall`, `completeToolCall`) and renders the
   typed part immediately in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`
   (`AssistantTool`, `pendingInput`, `toolComplete`). Pulse's Assistant chat
   stream must also preserve that event-by-event feel when HTTP or browser
   buffering delivers several SSE messages in one network chunk: the frontend
   stream consumer must let the browser paint between buffered Assistant
   progress/tool/status events for chat streams instead of synchronously
   draining a coalesced `tool_start`/`tool_progress`/`tool_end` batch in one
   JavaScript task. Ordinary content-token streaming must also get an immediate
   first-delta paint checkpoint and periodic checkpoints for long coalesced
   chunks, but must not be frame-throttled on every token by that progress-batch
   safeguard.
   Mock-mode Assistant streaming is a canonical fixture for that same runtime
   contract, not a frontend-only demo. When `mockmode.IsEnabled()` is true,
   `chat.Service.createProvider` must return the internal mock Assistant
   provider before invoking injected provider factories or configured external
   providers, and `chat.Service.ExecuteStream` must short-circuit the user turn
   only after the durable session is ensured and the user message is persisted.
   The fixture must emit the same browser-facing event family used by real
   turns: anchored `session`/prepare events from the normal entry path followed
   by mock `workflow_state`, `tool_start`, `tool_progress`, `tool_end`,
   response `workflow_state`, content chunks, and terminal `done`. It must also
   persist a real assistant message with model route `pulse:mock-assistant`,
   a successful `pulse_query` tool call, and a successful `pulse_read` tool call
   whose progress event includes provider-style raw input such as
   `pulse_read(...)`. That fixture keeps session restore, transcript tooling,
   stream-contract tests, and live browser checks exercising the same
   backend-to-browser shape as a real provider/tool turn, including the raw
   function-call syntax Pulse must summarize instead of rendering. Pulse-owned
   local Assistant routes such as `pulse:local-inventory` and
   `pulse:mock-assistant` must render through named labels (`Pulse inventory`,
   `Pulse mock Assistant`) in shared model picker and transcript chrome; the
   raw route IDs are implementation details, unlike external provider route IDs
   that may remain visible for disambiguation. Mock mode is
   also an effective runtime-config
   overlay: `internal/api/ai_handler.go` startup, restart, config sync, and
   tenant service initialization must treat Assistant as enabled in memory when
   mock mode is active, without saving or mutating the persisted `AIConfig`, so
   `/api/ai/chat` and Assistant sessions are available with zero configured
   external providers or models. The live fixture must pace its status, tool,
   and content events enough for the browser to paint each state in sequence;
   unit tests may disable that pace, but the dev fixture must not collapse into
   a single final completed row. This fixture must not emit `provider_fallback`
   in non-mock mode and must not become Pulse-authored remediation or routing
   behavior. The referenced OpenCode source at fetched `origin/dev` commit
   `4519a1da329c1a4fc384054e7203ba7d06928205` publishes tool-call and
   tool-result events incrementally in
   `packages/core/src/session/runner/publish-llm-event.ts`, mutates assistant
   tool parts through pending/running/completed states in
   `packages/core/src/session/message-updater.ts`, and renders the current tool
   part state in `packages/ui/src/components/message-part.tsx` through
   `packages/ui/src/components/basic-tool.tsx`. Pulse adapts that source model
   by making the mock fixture prove the same visible state sequence without
   copying OpenCode's editor-specific tool catalog.
   The referenced OpenCode source at fetched `origin/dev` commit
   `09d9cf01f93798939c1284fbe974b6e1f4d2759d` renders tool-specific inline
   labels such as `Read <path>`, `Grep "<pattern>"`, `WebFetch <url>`, and bash
   commands in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`.
   Pulse's compact tool rows must follow that operator-language model: the row
   should summarize the actual governed action (`search "prowlarr"`, `list
active alerts`, `topology summary`, `Inspect devices on current resource`,
   or exact governed write commands where the command itself is the operator
   decision) instead of exposing only internal action names such as `QUERY
search`, `exec`, provider function calls, or raw JSON; raw input and full
   output stay available behind Details. The referenced OpenCode source at
   fetched `origin/dev` commit
   `06d7840d1d42c9815d2d2e45e7fa4090ca4e3577` renders Bash tool state through
   `Bash` in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`
   (lines 701-735), where the pending row is replaced by the concrete command
   and completed block output keeps `$ <command>` visible above bounded output.
   Pulse adapts that for read-style infrastructure tools by keeping the
   operator action label as the primary row and adding a sanitized `$ ...`
   command preview below it when the tool input carries a shell command; raw
   provider envelopes such as `pulse_read(...)`, raw JSON, and full command
   output remain Details-only. Any visible command-copy affordance must copy
   that same sanitized command preview rather than the raw provider/tool input.
   The referenced OpenCode source at
   commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9` also renders pending tool rows
   through `InlineTool` pending text in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`
   (`InlineTool`, lines 563-651; `Bash`, lines 701-735; `Read`, lines
   758-770; `Grep`, lines 788-790). Pulse adapts that by using
   action-specific pending copy such as `Writing command...`, `Preparing
query...`, and `Reading storage...` before streamed tool arguments are
   parseable, then replacing that copy with the governed command/query/resource
   summary as soon as input arrives.
   The referenced OpenCode source at fetched `origin/dev` commit
   `4519a1da329c1a4fc384054e7203ba7d06928205` renders completed bash and
   generic tool output inside the tool block and uses `collapseToolOutput` in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`
   (`GenericTool`, lines 526-559; `Bash`, lines 701-731) to keep long output bounded.
   Pulse's completed tool rows must adapt that by previewing short actionable
   plain-text output for successful and failed tools while keeping long output
   bounded behind Details. Structured JSON, unavailable output, truncated
   successful output, and the full raw payload stay behind Details. The browser
   implementation owner is
   `frontend-modern/src/components/AI/Chat/ToolExecutionBlock.tsx`; completed
   `tool_end` events preserve streamed `raw_input` for the same readable command
   summary used while the tool is pending.
   The 2026-06-07 completed-output badge slice rechecked OpenCode `origin/dev`
   commit `e82542b8023a8374f29c23b70ec019c8f256354e` in the same
   `session-v2.tsx` `GenericTool` / `Bash` renderers (lines 527-559 and
   702-735): overflow is communicated through the expandable tool block, not
   through transport-sized copy in the compact row. Pulse adapts that by keeping
   exact output type/size in the accessible label and Details panel while the
   visible compact badge says only `output available`.
   The 2026-06-07 transcript command-evidence slice rechecked OpenCode
   `origin/dev` commit `e82542b8023a8374f29c23b70ec019c8f256354e` in
   `packages/opencode/src/cli/cmd/run/session-data.ts`: OpenCode keeps tool
   activity readable in session history while stripping shell echo noise from
   assistant text. Pulse adapts that by including the same sanitized `$ command`
   preview shown in live read-tool rows in copied/exported Assistant transcripts.
   Transcripts must still exclude raw provider call syntax such as
   `pulse_read(...)`, raw JSON tool payloads, obvious command secrets, and
   unbounded raw tool output by default.
   The 2026-06-07 active-dock pacing slice rechecked current OpenCode
   `origin/dev` commit `e82542b8023a8374f29c23b70ec019c8f256354e` in
   `packages/tui/src/feature-plugins/system/session-v2.tsx`: `InlineTool`
   (lines 564-651) keeps one visible activity row that moves from pending copy
   to completed copy, while `Bash` (lines 702-740) keeps shell activity visible
   through the same inline/block transition. Pulse adapts that for the
   Assistant dock by pacing burst-arriving workflow status history locally so
   `Preparing context`, `Reading inventory`, and provider wait states appear in
   sequence; statuses that arrive one at a time still replace immediately.
   The referenced OpenCode source at fetched `origin/dev` commit
   `fa2b63f850fc0a23bec2bdff9e660450d3fe7913` also keeps assistant text,
   reasoning, and tool invocation as typed message parts in
   `packages/opencode/src/session/message.ts`, while
   `packages/opencode/src/session/processor.ts` updates text and reasoning
   parts through `*-delta` events instead of rendering raw provider tool-call
   syntax as assistant prose. The referenced OpenCode source at fetched
   `origin/dev` commit `1399323b78a04229d9bfe00c7436d7f41770fda8` renders
   reasoning with `AssistantReasoning` and `ReasoningHeader` in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`,
   separate from `AssistantText` and `AssistantTool`. Pulse's frontend stream
   reducer must preserve the same user-facing invariant: visible transcript
   content is typed assistant text, a neutral typed thinking-progress row, or a
   typed Pulse tool/approval/question row. The thinking-progress row may expose
   live activity state such as `Thinking...` / `Thinking complete`, but it must
   not render raw provider reasoning text. Suspicious compacted provider
   prelude text that looks like tool-call narration must be buffered until it is
   proven to be normal answer text or stripped when a raw tool-call marker
   arrives; it must not flash as run-on prose such as
   `I'llcheckthedevicenodes...` while the actual governed tool row is still
   being assembled, and it must not be promoted to final assistant answer text
   when the stream reaches `done` without proving a normal answer. The
   referenced OpenCode source at fetched `origin/dev` commit
   `1025540fcc2a69609a0131a7168300205656d728` keeps rendered answer text as
   typed text parts in `packages/ui/src/components/message-part.tsx` and uses
   `createPacedValue` only as a presentation-layer reveal; Pulse adapts that by
   preserving typed content exactly while suppressing malformed compacted
   tool-prelude residue rather than inventing spaces or storing it as prose.
   A pre-tool assistant preamble such as "I'll check that" does not satisfy the
   final-answer contract after Pulse executes tools. The final-response guard
   must require non-empty assistant text after the latest user/tool-result
   anchor; otherwise it must make the bounded no-tools summary call or emit the
   deterministic fallback summary. This adapts the referenced OpenCode source
   at fetched `origin/dev` commit
   `06d7840d1d42c9815d2d2e45e7fa4090ca4e3577`: `packages/opencode/src/session/llm/ai-sdk.ts`
   maps tool input/call/result stream events into typed tool events (lines
   190-246), `packages/opencode/src/session/message-v2.ts` serializes completed
   tool results and reasoning as ordered provider message parts (lines
   340-390), and `packages/ui/src/components/message-part.tsx` only renders
   non-empty typed text/reasoning/tool parts through `renderable` and
   `AssistantParts` (lines 607-728). Pulse adapts that ordered-part invariant
   inside `internal/ai/chat/agentic_final.go` because Pulse's transcript is
   persisted as messages plus tool-result anchors rather than OpenCode's
   per-part store.
   Streamed provider startup and mid-stream progress must be bounded by the
   configured Assistant request timeout, the OpenAI-compatible SSE
   response-header guard, and the per-chunk SSE body-read guard adapted from
   OpenCode's provider `wrapSSE` source. The referenced OpenCode source at
   fetched `origin/dev` commit `ba57718b0516c7a8670d1e820b1a24146a8b8262`
   wraps `text/event-stream` responses in
   `packages/opencode/src/provider/provider.ts` so each stream read either
   yields a chunk or aborts within the configured `chunkTimeout`, distinct from
   the response-header timeout. Pulse's OpenAI-compatible Assistant transport
   follows that split: transient startup failures may retry once before
   surfacing failed-turn recovery, but a stalled route must not leave the user
   in an opaque first-token or mid-answer wait for the full provider timeout.
   Assistant turn tool exposure is backend-owned runtime behavior, not a
   provider whim or frontend polish concern, and it is model-owned at the
   manifest boundary: every interactive turn that reaches the selected model
   carries the full governed tool manifest from `toolsForExecutionMode`, and
   the model decides whether to answer directly, ask a clarification, or call
   tools inside Pulse policy. Pulse must not classify prompt wording into
   text-only or query-only tool scopes — greetings, exact-reply diagnostics,
   "no tools"/"before using tools" phrasing, inventory/count/list/breakdown
   wording, alert/finding wording, and prompt length are all model-visible
   text, not Pulse routing signals. The former `assistantToolScopeForPrompt`
   keyword router, its query-only manifest filter, its conversational/
   direct-text text-only scopes, and its query-only topology prefetch (inject
   compact inventory into the user message, then withhold tools) are removed
   and must not be reintroduced; that family withheld tools from real lookups
   ("hows esphome") and zeroed the manifest on false-positive phrases
   ("before using any tools, tell me your plan"). The only manifest-boundary
   exception is the contract-sanctioned context-only resource handoff turn
   (see Completion Obligations). `TestService_ExecuteStream_ToolManifestIsModelOwned`
   (`internal/ai/chat/service_tooling_test.go`) and
   `TestService_ExecuteStream_InventoryBreakdownIsModelOwned`
   (`internal/ai/chat/service_execute_additional_test.go`) are the regression
   proofs that previously-scoped prompt families all reach the model with the
   same governed manifest and an unmodified user message.
   The system prompt's missing-target policy is resolve-before-asking, not
   ask-first: when a command or diagnostic request names no target, the
   Assistant must first use read-only query/topology tools to identify
   plausible targets, proceed against a sole plausible match for read-only
   diagnostics while naming the target in the answer, and ask the operator
   only when several plausible targets remain or the action changes state.
   The earlier ask-first framing ("Missing target information is not a safe
   default... ask for the missing target") made the Assistant deflect every
   "run X" request back to the operator — including on single-host
   deployments where no real ambiguity exists — instead of investigating
   like a competent operator (the OpenCode-parity gap this supersedes).
   Placeholder targets remain forbidden in all modes: the model must never
   guess an unresolved target or substitute `current_resource` outside an
   attached-resource turn, and write actions still require an explicit or
   operator-confirmed target.
   `TestBuildSystemPrompt_CurrentResourceRequiresResourceHandoff`
   (`internal/ai/chat/service_tooling_test.go`) pins this boundary.
   Deterministic count-only inventory prompts remain the single Pulse-owned
   local answer shortcut, and it is an answer path, not tool selection: when
   canonical topology state already carries the complete aggregate counts,
   `internal/ai/chat` should answer locally by streaming normal typed
   assistant content and `done` before provider attempt creation.
   This adapts the referenced OpenCode `message.ts` and `processor.ts` typed
   part/delta model at commit `fa2b63f850fc0a23bec2bdff9e660450d3fe7913`:
   locally owned text is still transcript text, but it must not wait on a
   remote provider or pretend that shell/tool inspection produced the answer.
   The completion metadata for that path should identify the effective route
   as `pulse:local-inventory` so the transcript label reflects Pulse-owned
   local runtime output instead of the operator's selected remote model.
   Explicit or qualified operator intent is the escape hatch from this
   shortcut, not a Pulse-authored tool choice: prompts that ask to use tools,
   request a read-only attempt, provide a shell/read command or path such as
   `ls /dev | wc -l`, or qualify the count beyond pure inventory ("how many
   vms have errors") must reach the selected model/provider with the full
   governed Assistant tool manifest instead of answering from local
   inventory. Prompt classification
   (`assistantPromptQualifiesForLocalInventoryCount` in
   `internal/ai/chat/service.go`) may only suppress the shortcut and send the
   turn to the model — its false positives fail safe, to the model — and the
   selected model still decides whether to call `pulse_read`, answer
   directly, or ask a clarification. This adapts the referenced OpenCode
   source at fetched
   `origin/dev` commit `4519a1da329c1a4fc384054e7203ba7d06928205`, where
   `packages/core/src/session/message-updater.ts` mutates tool input, called,
   progress, success, and failed states as typed session events (lines
   247-318), by preserving the operator's explicit tool intent until Pulse can
   stream a visible `tool_start` / `tool_progress` / `tool_end` path instead of
   collapsing the turn into unrelated inventory prose.
   The Assistant transcript should show compact
   activity as work happens: local context reads, governed tool names, provider
   routing, fallback, and first-token waiting must appear as the latest
   replacing live status/header status rather than a dead-looking wait state, a
   dumped list of transient phases, or a delayed replay of stale status history.
   This adapts the referenced OpenCode source at fetched `origin/dev` commit
   `914a643`, where the app session timeline derives visible activity from the
   current session status and streamed message/tool parts instead of pacing old
   status labels back through the UI. Hide or collapse unsafe details and bulky
   outputs, but do not hide the fact that Pulse is running a local read,
   invoking a governed tool, or waiting on a provider. Stream-idle heartbeats
   must inherit the selected provider/model route when Pulse has that route, so
   the visible row reads as route-specific liveness instead of falling back to
   generic Assistant waiting copy. Pulse must not rewrite the inputs of a
   model-chosen tool call (the former summary-only `pulse_query` topology
   input default is removed with the query-only scope); `pulse_query`
   defaults are owned by the tool definition the model sees, not by
   post-hoc input mutation. Assistant answer style should stay operational
   and must not use emoji, warning icons, or decorative symbols unless the user
   explicitly asks for that tone.
   The system prompt for a turn must describe only the tools actually offered to the
   provider for that turn; generic tool-governance prose must not name tools
   hidden from the current manifest. Deterministic provider-request capture is
   the primary proof harness for this fast path; live OpenRouter/browser checks
   are final smoke tests, not the iteration loop.
   Restored Assistant sessions must hydrate saved assistant content and
   persisted tool calls into the same transcript event shape used by live
   streams so switching sessions does not hide prior tool evidence or collapse
   the resumed conversation into a text-only transcript. Saved Assistant
   message history is part of that same output contract: service reads and
   `GET /api/ai/sessions/{id}/messages` must return a client-safe projection
   that normalizes collection fields, strips `reasoning_content`, and sanitizes
   stored assistant prose before the browser restores a transcript. Provider
   reasoning may remain in runtime-only history for model continuity, but it
   must not serialize through public session history or resumed drawer state.
   The referenced OpenCode source at fetched `origin/dev` commit
   `09d9cf01f93798939c1284fbe974b6e1f4d2759d` stores message parts as typed
   text, reasoning, and `tool-invocation` entries whose tool invocation state
   is `call`, `partial-call`, or `result` in
   `packages/opencode/src/session/message.ts`. Pulse adapts that restored
   transcript contract by folding persisted internal tool-result transport
   messages back into the browser-safe assistant `tool_calls` projection as
   completed `output` plus `success` state, preserving the assistant message's
   effective model route, and omitting the internal tool-result messages from
   the restored browser transcript.
   Pending tool progress is part of the same transcript contract: pending tool
   stream events must render inline as compact rows in the assistant turn,
   transition to `running` or `waiting` through `tool_progress`, and resolve in
   place on `tool_end` instead of disappearing until completion or relying only
   on the drawer-level status bar. The referenced OpenCode source at fetched
   `origin/dev` commit `ba57718b0516c7a8670d1e820b1a24146a8b8262` emits an ACP
   `tool_call` when a tool part is known and then sends `tool_call_update`
   patches for pending, running, completed, and failed states through
   `packages/opencode/src/acp/event.ts` and
   `packages/opencode/src/acp/tool.ts`; Pulse adapts that by keeping the live
   pending row's current progress text visible inside the transcript at drawer
   width, not only in the footer/status strip.
   The drawer transcript must treat those in-place progress patches as fresh
   visible activity even when the message count and stream-event count do not
   change. The referenced OpenCode source at commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9` applies
   `session.next.tool.progress` by mutating the latest matching tool part in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`
   (`apply(event)`, line 120; `session.next.tool.progress`, lines 317-326)
   and renders the same session-owned tool part through
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx`
   (`ToolPart` mapping, lines 1788-1796; `InlineToolRow`, line 1926).
   Pulse adapts that by deriving transcript scroll/freshness from the latest
   message's activity fingerprint, including pending-tool progress and
   `updatedAt`, instead of using appended-event count as the only streamed
   activity signal.
   Assistant session listing is a history-browsing path, not a send-path lock.
   The session store must not hold its shared mutex while scanning and parsing
   every persisted session file for `/api/ai/sessions`; writes must remain
   atomically readable through temp-file plus rename so a large recent-session
   list cannot block `EnsureSession`, `AddMessage`, or stored handoff reads on
   a new prompt.
   Session listing cost must not scale with stored transcript bytes.
   `SessionStore.List` serves summaries from a per-file cache validated by
   each file's (modTime, size), re-reading only files that changed since the
   last list, and a cache-miss parse must not deep-decode message or redo
   payloads (only their counts feed the summary). The cache persists as
   `.sessions_index.json` beside the session files so the first list after a
   restart stays index-speed; the index is best-effort and self-healing
   (missing, corrupt, or stale entries only cost a re-parse, never a wrong
   summary, and dotfiles are never listed as sessions). `writeSession` and
   `Delete` keep the cache and index coherent on every mutation.
   Pulse-owned background sessions are not resumable chats. The session store
   owns the well-known background session IDs (`patrol-main`, `patrol-eval`,
   `investigation-*`) through `chat.IsSystemSessionID` and marks their
   summaries `system: true` in every `Session` projection. The Assistant
   empty-state quick-resume list (`selectQuickResumeSessions` in
   `frontend-modern/src/components/AI/Chat/recentSessionsModel.ts`) must
   exclude system sessions — a Patrol detection log titled with the raw triage
   seed is forensic material, not a conversation to resume — while the
   drawer-owned session picker and the Settings sessions panel keep listing
   them for inspection. The canonical Patrol forensic surface remains the
   `PatrolRunRecord` history at `/api/ai/patrol/runs`.
   Assistant session navigation must provide a searchable history path in the
   drawer-owned picker, using the canonical `/api/ai/sessions` contract rather
   than a separate recent-chat store. Search is applied before result limiting
   so the picker can narrow older sessions without hiding matches behind a
   recency cap, the picker must open immediately with cached sessions while any
   refresh/search is still running, and loading/error states must stay inside
   the picker instead of making the main send path look busy. The referenced
   OpenCode source at fetched `origin/dev` commit
   `4519a1da329c1a4fc384054e7203ba7d06928205` implements
   `SessionSwitcherDialog` in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/session/dialog.tsx` with
   `createDebouncedSignal("", 150)`,
   `sdk.client.session.list({ search: input.query, limit: 30, ...input.filter })`,
   recency ordering through
   `orderByRecency`, and pinned/current session options before navigation.
   Pulse adapts that source workflow with a debounced Assistant session search
   field, bounded recent-session refreshes with
   `GET /api/ai/sessions?limit=30`, searched refreshes with
   `GET /api/ai/sessions?search=...&limit=30`, and searched result rows that
   resume through the same session-loading path as the normal recent list.
   Assistant session forking is a runtime-backed session workflow, not a
   header-only UI affordance. The referenced OpenCode source in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx` registers
   `session.fork` as a first-class session command and opens
   `DialogForkFromTimeline` from the current session route. Pulse adapts that
   source behavior through `POST /api/ai/sessions/{id}/fork`: the local
   `chat.Service` must route to `SessionStore.Fork`, and the store must create
   a fresh durable session ID with fresh create/update timestamps while cloning
   the saved transcript and browser-safe model handoff summary. Copied
   messages keep their per-session message/tool IDs so assistant tool-call and
   tool-result relationships remain intact inside the fork. Forking must not
   mutate the source session, must continue through normal session list/load
   contracts, and must not return `not_implemented` from the local runtime.
   OpenCode-style session file diff/revert workflows do not apply to Pulse
   Assistant sessions: Pulse does not own local code-file edits, and
   infrastructure mutations are governed through approval/action history. The
   Settings maintenance surface must not advertise session file changes or
   session revert actions, and legacy direct calls to
   `/api/ai/sessions/{id}/diff`, `/revert`, and `/unrevert` must fail
   explicitly with `501 Not Implemented` rather than returning a success-shaped
   placeholder.
   The picker trigger and result rows are part of that source-backed workflow:
   the trigger must expose its accessible name and expanded state, opening the
   picker must focus search, and each result must be a keyboard-addressable
   option with up/down, page, home/end, and Escape navigation from both the
   focused search field and focused result rows. Session-picker Escape is a
   local close path that returns focus to the session trigger and must consume
   the keypress so the App-level Assistant drawer Escape guard does not also
   close the whole drawer. Delete remains a named row action instead of a
   mouse-only affordance.
   Message-level copy is part of the same OpenCode-aligned message action
   surface, but Pulse must keep it browser-safe and role-neutral rather than
   implying unsupported message-level fork/revert semantics. The referenced
   OpenCode source in `packages/tui/src/routes/session/dialog-message.tsx`
   exposes `message.copy` by collecting non-synthetic text parts for the
   selected message. Pulse adapts that by allowing visible user and completed
   assistant transcript rows to copy their own visible text through the shared
   clipboard fallback helper; hidden reasoning, raw tool input/output, provider
   envelopes, and scoped handoff metadata remain excluded unless the operator
   explicitly opens a raw Details surface.
   Assistant session rename is part of that same source-backed session
   workflow. The referenced OpenCode source in
   `packages/opencode/src/cli/cmd/tui/component/dialog-session-list.tsx`
   imports `DialogSessionRename` and invokes the runtime session rename action
   from the session list, so Pulse must not leave rename as local picker state.
   Pulse owns the equivalent through `PATCH /api/ai/sessions/{id}`,
   `chat.Service.RenameSession`, and `SessionStore.Rename`; the drawer picker
   may render the edit inline, but saving must persist the normalized title,
   return the updated `ChatSession`, update the visible row without closing the
   picker, and leave messages, handoff context, approvals, and tool evidence
   unchanged.
   Session titles follow OpenCode's generated-title model: after a session's
   first exchange completes, the chat runtime upgrades the auto-truncated
   first-prompt placeholder to a short model-generated title
   (`chat.Service.generateSessionTitle`, `internal/ai/chat/session_title.go`).
   The upgrade runs after the done event reaches the client (no turn
   latency), only on the first exchange, and never overwrites a title the
   user set: any title that differs from the placeholder is left alone, with
   a re-check before persisting to lose races against a concurrent rename.
   Title requests pass the same model-boundary sanitizer as normal turns,
   record usage as `assistant_session_title`, and a failed or timed-out
   title call leaves the placeholder in place rather than failing anything
   user-visible.
   Assistant turn undo/redo is also a source-backed session repair workflow,
   not a transcript-local delete button. The referenced OpenCode source in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx` registers
   `session.undo` and `session.redo` as first-class session commands and uses
   runtime revert/unrevert responses to restore the user's prompt into the
   composer. Pulse adapts that behavior through
   `POST /api/ai/sessions/{id}/undo` and
   `POST /api/ai/sessions/{id}/redo`: the backend session store must remove the
   latest user-authored turn plus following assistant/tool messages as one
   durable turn, persist a bounded redo stack, expose `can_redo` on the
   browser-safe session summary, clear redo state when a new message or trim
   changes the transcript, and keep forked sessions independent from the source
   redo stack. The drawer must restore the removed prompt, structured mentions,
   finding id, handoff metadata, approval/autonomous override, and selected
   model route when available so the operator can edit and resend without
   reconstructing hidden context from transcript prose.
   Retry and regenerate are turn re-runs over the same undo boundary, not a
   parallel history mutation path. `POST /api/ai/sessions/{id}/undo` accepts an
   optional JSON body (`chat.SessionTurnUndoOptions`) whose `expected_prompt`
   field, when non-empty, makes the removal conditional on the latest
   user-authored prompt matching the prompt being re-run, so a retry whose
   failed send never reached the session cannot remove a different turn.
   Frontend retry ("Try again" on a failed turn, model-route switch retry) and
   regenerate must remove the replaced turn through this guarded undo before
   re-sending so session history never records the prompt twice; a failed
   guard or transport error degrades to a plain re-send rather than blocking
   the retry. The removed turn stays redoable until the re-sent message lands,
   at which point the normal new-message rule clears redo state. The
   regenerate affordance is offered only on the latest settled assistant
   answer (nothing streaming, no error block, no pending approval or
   question, and a user prompt available to re-send) because the undo
   boundary only operates on the last durable turn.
   The empty Assistant drawer may surface recent non-empty sessions as direct
   resume actions using the backend session list already owned by the drawer;
   it must not create a parallel recent-chat store or product-authored prompt
   shortcut path.
   Assistant output hygiene is part of the same boundary: provider reasoning
   and raw serialized tool-call artifacts must never render as assistant
   transcript prose or serialize into client-facing chat streams. Browser-bound
   Assistant stream events must pass through `chat.StreamEvent.ClientSafe()`;
   provider `thinking` chunks are runtime-only and may be retained internally
   for model continuity, but they are dropped before the browser/API boundary.
   The referenced OpenCode source at fetched `origin/dev` commit
   `1399323b78a04229d9bfe00c7436d7f41770fda8` stores assistant output as typed
   text, reasoning, and `tool-invocation` parts in
   `packages/opencode/src/session/message.ts` and mutates those typed parts
   through explicit stream events in
   `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`; Pulse adapts that
   invariant by stripping operational decorative status glyphs, warning icons,
   and check/cross badges from browser-safe assistant prose while preserving
   ordinary Unicode answer text such as units.
   The agentic stream may translate the first private provider reasoning delta
   before visible output into a neutral `model_thinking` workflow status so the
   drawer shows live activity without exposing chain-of-thought. Neutral
   progress comes from the active stream state, workflow status, or governed
   tool/question/approval events, not from a reasoning body. Pulse tool
   invocations must surface only through governed `tool_start` / `tool_end`,
   approval, or question blocks; if a provider emits `pulse_*` / `patrol_*`
   calls, DSML, XML/function-call envelopes, or JSON tool-call shapes as text
   content, the chat runtime must strip them before streaming, persistence, and
   frontend rendering. Compacted no-whitespace internal prelude text attached
   to a leaked tool invocation is part of that same artifact and must be
   suppressed or retracted from the current stream segment instead of rendered
   as assistant prose; if that compacted prelude is still pending when the
   stream closes, terminal flush must drop it rather than turning unreadable
   provider residue into the final answer. Drawer-level loading/progress status
   must stay scoped to the active turn and must collapse back to quiet when the
   turn is idle; it may mirror the latest active tool/workflow state as a
   compact heartbeat, but raw outputs, restored evidence, errors, and completed
   details stay in the transcript row that owns the turn. Completed tool rows in
   the drawer may show
   compact tool name, action summary, status, and an
   explicit details affordance, but raw tool input/output JSON must not render
   in the default transcript. Token accounting and other provider metadata
   remain runtime/accounting data, not normal transcript prose.
   Background provider-readiness checks are diagnostics and must stay quiet
   while idle unless they produce an actionable issue; checking status may
   surface during an active send, but an open empty drawer must not look busy
   just because Pulse is verifying the selected route.
4. Add or change Patrol, alert-analysis, or remediation transport through `internal/api/ai_handlers.go`, `internal/api/ai_intelligence_handlers.go`, and `frontend-modern/src/api/patrol.ts`
   Provider preflight diagnostics returned from `internal/api/ai_handlers.go`
   must reuse the Patrol runtime failure classifier in `internal/ai/` and
   expose only safe operator-facing cause, summary, recommendation, model, and
   action fields. Raw provider response bodies and transport errors may be
   logged server-side or attached as redacted internal Patrol evidence where
   governed, but they must not be returned through the browser provider-test
   contract.
   Anthropic runtime provider execution must be API-key backed. Legacy
   Anthropic OAuth tokens may remain in encrypted settings only as cleanup
   state; they must not make Anthropic configured, instantiate an OAuth-backed
   provider, refresh tokens, or send model requests. OAuth start/exchange and
   callback handling must fail closed, while disconnect may clear stored legacy
   tokens.
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
6. Add or change AI provider, control-level, or chat/session presentation through `frontend-modern/src/components/AI/Chat/`, `frontend-modern/src/utils/aiProviderPresentation.ts`, `frontend-modern/src/utils/aiProviderHealthPresentation.ts`, `frontend-modern/src/utils/aiControlLevelPresentation.ts`, `frontend-modern/src/utils/assistantPageContext.ts`, and `frontend-modern/src/utils/aiChatPresentation.ts`
   AI provider/model presentation must preserve the model transport route when
   the selected provider is a gateway. OpenRouter-routed model IDs such as
   `openrouter:deepseek/...` must render with an explicit `via OpenRouter`
   label in the shared picker, System AI settings status, and inherited default
   descriptions unless the server-supplied model name already carries that
   route. Assistant transcript rows must carry and render the effective model
   route that produced each live or restored response so route recovery and
   mixed-provider sessions remain auditable. Direct provider models must not
   gain a gateway label, but explicit direct route strings such as
   `deepseek:deepseek-v4-pro` must still render with provider identity rather
   than collapsing to a bare-looking model payload. Gateway route strings must
   still render as readable provider/model labels before catalog hydration; this
   mirrors OpenCode's source pattern of retaining structured
   `providerID`/`modelID` metadata and presenting catalog names with ID
   fallbacks rather than leaking raw route storage IDs into primary chat chrome.
   Assistant model selector actions must
   remain route-distinct: if the configured chat override resolves to the same
   route as the effective default or the already selected session model, the
   drawer must not render a duplicate override action.
   Assistant provider-readiness actions must send operators to the canonical
   Pulse Intelligence > Provider & Models route
   `/settings/pulse-intelligence/provider`; `/settings/system-ai` remains a
   compatibility alias for old deep links rather than the href emitted by new
   Assistant repair actions.
   Assistant discovery-context hints must use the same product language as
   Pulse Intelligence settings: the visible hint is `Discovery is off`, while
   the body explains that enabling it gives Assistant real service, version,
   and command context instead of generic guidance. The hint must not
   reintroduce the old `Workload Discovery` label as a separate product concept.
   Authenticated shell Assistant launches must use
   `frontend-modern/src/utils/assistantPageContext.ts` to attach factual
   current-route context before the drawer opens. That helper may identify the
   current platform, Patrol, Alerts, Settings, or unknown Pulse view and provide
   neutral briefing metadata, but it must not synthesize a prompt, require a
   tool choice, or turn Assistant back into the primary operations front door.
   Assistant route and control chrome belongs with the prompt surface, not the
   drawer title row. The referenced OpenCode source at commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9`
   `packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx` derives the
   current provider label in `Prompt` lines 196-197 and renders agent, model,
   provider, and variant metadata in the prompt footer at lines 1452-1479; the
   model metadata row uses an explicit `·` separator before the model at line
   1462 and before variants at line 1471, so adjacent metadata must not collapse
   into fused labels. Pulse's shared model picker adapts that by separating
   selected model labels from badges such as `default` in both visible text and
   the button accessible name, so an OpenRouter default route renders as
   `Qwen: Qwen3.7 Plus via OpenRouter · default` rather than
   `OpenRouterdefault`.
   The session route injects that prompt through
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx`
   `session_prompt` lines 1313-1333. Pulse adapts that by keeping the active
   model route selector, control-mode selector, and last-turn summary in
   composer chrome while the drawer header stays limited to drawer/session
   commands.
   Empty Assistant sessions are prompt-first transcript surfaces, not product
   marketing or instruction panels. The referenced OpenCode source at commit
   `9ed17da55ab1f7360cc0e01075f763e27fa899e9`
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx` derives
   session messages in `Session` lines 185-203, renders only the message
   stream inside the scrollbox at lines 1188-1295, and renders `Prompt` in the
   `session_prompt` slot at lines 1313-1333 without inserting a centered
   no-message title or subtitle. Pulse adapts that by leaving a blank
   transcript blank, keeping scoped handoff context in the drawer context
   strip, and limiting no-message transcript affordances to real
   recent-session resume actions.
   The referenced OpenCode source at fetched `origin/dev` commit
   `1399323b78a04229d9bfe00c7436d7f41770fda8` renders the completed assistant
   footer in
   `packages/opencode/src/cli/cmd/tui/feature-plugins/system/session-v2.tsx`
   (`AssistantMessage`) with agent, provider/model, and turn duration rather
   than token counts. Pulse Assistant rows adapt that by keeping visible token
   accounting out of the transcript while showing a compact completed-turn
   duration beside the effective model label once a turn reaches `done`,
   `error`, or user interruption. Runtime token usage may surface in
   low-priority composer chrome instead, but the completed-turn surface is a
   route summary rather than a token counter: the referenced OpenCode source at
   fetched `dev` commit `3867fa2bad0e644166e360e2e99cfe426fe71105` imports
   `turnSummaryCommit` in
   `packages/opencode/src/cli/cmd/run/scrollback.surface.ts` lines 19-20, and
   `packages/opencode/src/cli/cmd/run/turn-summary.ts` lines 5-46 renders a
   final system summary from agent, model, and duration. Pulse adapts that by
   deriving the latest non-streaming assistant `ChatMessage`, rendering the
   effective provider/model route, completed-turn duration, and backend-proven
   total token count in the composer footer. The input/output split remains in
   the footer title/accessibility text, not the visible transcript or composer
   copy, so shorthand such as `4358 in · 943 out` cannot read like assistant
   output. If provider usage is missing, the route and duration remain visible;
   active streaming turns never become the completed-turn summary. Cost and
   context-limit percentages stay absent until the runtime exposes those values
   through a governed contract.
   Assistant tool activity is visible transcript activity, not a hidden context
   footer. The referenced OpenCode source at fetched `origin/dev` commit
   `e82542b8023a8374f29c23b70ec019c8f256354e`
   `packages/opencode/src/cli/cmd/run/types.ts` defines append-only
   `StreamCommit` rows for assistant, reasoning, tool, and system sources at
   lines 284-312, while
   `packages/opencode/src/cli/cmd/run/footer.ts` queues commits and only
   coalesces consecutive progress chunks for the same part/tool at lines
   512-545. Pulse adapts that contract by rendering each Assistant
   `pending_tool`, completed `tool`, and `tool_cancel` event as its own
   chronological transcript row. Tool inputs, command previews, progress text,
   and outputs may remain collapsed inside the row, but context/read/query
   tools must not be replaced by a generic grouped footer that makes several
   operations appear all at once.
   Queued follow-up turns are active session pressure, not hidden backlog.
   The referenced OpenCode source at fetched `origin/dev` commit
   `e82542b8023a8374f29c23b70ec019c8f256354e`
   `packages/opencode/src/tool/todowrite.txt` describes status tracking as
   surfacing progress to the user at line 1 and requires status updates in real
   time rather than batched completion at line 25, while
   `packages/opencode/src/session/tools.ts` updates running tool metadata with
   title, status, input, and start time at lines 54-63. Pulse adapts that
   session-activity pattern by keeping queued follow-ups visible in the
   Assistant activity dock and appending the queued count to the active-turn
   headline while the current response is streaming. The queued row may retain
   edit, remove, promote, resume, and clear controls, but the primary headline
   must not imply the user's just-submitted follow-up disappeared behind a
   generic `Generating response` state.
   Restored Patrol mode save-failure handoffs may preserve compatible
   summary kinds such as `patrol_configuration_failure`, but Assistant drawer
   titles, subjects, action labels, and safety notes must describe Patrol mode
   rather than reviving generic configuration wording. Stable route markers,
   telemetry fields, and API wire names may retain `patrol_control`;
   user-facing Assistant, external-agent, generated MCP, and frontend copy must
   call the operator's choice `Patrol mode`.
7. Keep AI chat presentation helpers aligned through `frontend-modern/src/components/AI/Chat/` and the shared `frontend-modern/src/utils/textPresentation.ts`
8. Keep assistant drawer context, session, and org-switch reset state aligned through the shared `frontend-modern/src/stores/aiChat.ts` boundary instead of letting `frontend-modern/src/App.tsx`, `frontend-modern/src/AppLayout.tsx`, or feature callers fork their own assistant shell state
   The shared app shell may also mount the deployment-installability-owned
   post-update release highlights card. That card must consume published
   release notes through the update surface and must not read, reset, or
   otherwise couple itself to Assistant sessions, Patrol state, model routing,
   or AI-provider readiness.
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
   Structured Patrol run and Patrol mode save-failure handoffs may render bounded,
   redacted diagnostic lines in the drawer when they are opened directly from
   Patrol runtime/control surfaces, but the attached headline must remain
   source-owned (`Patrol run attached`, `Patrol mode ... attached`)
   and the drawer must still exclude raw provider payloads, commands, and
   Patrol-authored remediation steps.
   Reloaded Assistant sessions may consume the backend-owned
   `handoff_summary` only as safe presentation state and a Patrol finding
   pointer; hidden model context, command payloads, preflight data, and action
   results stay backend-owned and must not be reconstructed in the browser.
9. Add or change public Pulse Intelligence overview wording through `docs/AI.md`; it may
   describe Assistant and Patrol capabilities, but it must not revive legacy
   commercial shorthand such as `incident memory` as a current product promise.
   The public overview must preserve the product architecture: Pulse
   Intelligence Core owns canonical context, governed actions, safety gates,
   approval state, action audit, and verification; Pulse Patrol is the primary
   built-in operator on that core; its public summary must stay concise as
   watching, investigating, acting within the chosen Patrol mode, verifying outcomes,
   and recording what happened; and Pulse Assistant plus Pulse MCP are
   contextual and external-agent access paths over the same governed
   capabilities.
   Public overview, safety, and onboarding copy must use the same visible
   Patrol mode labels as the product (`Watch only`, `Ask first`,
   `Safe auto-fix`, `Autopilot`) and must not reintroduce the retired
   `Only watch`, `Ask before changes`, `Fix safe issues`,
   `Auto-fix safe issues`, `Full control`, or `Policy autopilot` labels as
   customer-facing names.
   The public overview must also preserve the model-owned AI boundary: Pulse
   Assistant and Patrol provide governed context, tools, safety gates,
   approval state, and audit trails, while the configured LLM owns diagnosis,
   prioritization, fix reasoning, and tool choice. Public copy must not
   present Pulse code as the intelligence engine, a prompt-keyword router, a
   deterministic auto-remediation oracle, a provider replacement, a learned
   operational-meaning engine, or a deterministic finding author.
   The public overview must not carry a hand-maintained Assistant tool table.
   Assistant tool inventory, action modes, approval policies, provider
   declarations, and Patrol-only filtering are registry-owned in
   `internal/ai/tools/` and projected through `internal/agentcapabilities/`;
   external-agent inventory belongs to `/api/agent/capabilities` and
   `pulse-mcp` `tools/list`.
   External-agent setup metadata follows the same rule: Pulse MCP server name,
   command, base URL flag/default, token environment variable, and supported
   client config families are manifest-owned in
   `/api/agent/capabilities.mcpAdapter`. Assistant, Pulse Intelligence
   settings, README, and MCP surfaces may present that setup, but must not
   maintain separate MCP setup constants. Security API settings may only host
   the scoped-token creation link used by that setup.
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

Qualification floor: Patrol model launch and product-claim qualification must use
   `cmd/patrol-qualify` against reviewed scenario-owned ground truth. Expected
   faults and independent postconditions must be declared before the run and
   must not derive from Patrol-selected tools, deterministic signal extraction,
   or model output. Watch, investigation, and remediation are separate tracks;
   remediation decisions require the runner's second authorization gate, exact
   action/finding/investigation/resource/capability/plan-hash binding, and an
   out-of-band postcondition. Scorer or tool-transcript replay is regression
   evidence only and cannot replace live normal-collection, real-model proof.
   A live Docker remediation lab must opt in separately to a command-enabled
   disposable agent with a short-lived `agent:exec` token; Watch and
   investigation labs remain report-only. The scenario must expect the
   state-dependent capability advertised by the canonical resource contract
   (`start` for an exited container), and teardown must revoke the temporary
   token as well as restore and remove the exact-run-labelled resources.
   Shared or remote labs must use exact-run-labelled disposable resources,
   two-pass cleanup, full Docker inventory comparison including images, and
   must never manufacture faults in production infrastructure.
   Read-only investigation tools that accept canonical resource IDs must
   resolve provider coordinates internally before calling provider-owned
   discovery or inspection paths. A model must not need to infer a Docker
   container ID or substitute a display name after `pulse_query` returns a
   canonical `app-container` ID; cross-tool identity translation is an AI
   runtime responsibility.
   Investigation action-catalog and proposal tools inherit that identity
   boundary in both directions: an exact, uniquely resolved Docker provider
   coordinate may be translated back to its canonical `app-container` target,
   and the catalog response plus captured proposal must carry that canonical
   target. Unknown or ambiguous references still fail closed. The model is not
   responsible for Pulse's provider-to-canonical identity plumbing.
   Agentic context compaction is also evidence-preserving rather than merely
   size-reducing. When a read tool returns structured safety-relevant state,
   its deterministic knowledge projection must retain the bounded fields needed
   for later diagnosis and action choice while excluding labels, environment
   values, and other unrelated raw inspection content. Docker inspection
   compaction therefore retains container state, running/OOM/dead flags, exit
   and health results, restart count/policy, and image identity; it must not
   collapse a valid inspection to punctuation or the first formatting lines.

Patrol autonomous-loop floor: Watch must preserve model-owned investigation
while requiring an accepted structured outcome for every active finding the
run presents or returns. New issues use `patrol_report_finding`; existing
   issues use `patrol_assess_finding` with exactly one `present`, `resolved`, or
   `uncertain` verdict. Missing verdicts are run errors, not absence-based
resolution or all-clear evidence. `present` refreshes durable evidence and
run ownership, `resolved` remains behind deterministic fail-closed
verification, and `uncertain` keeps the finding active. Scoped Patrol must
use its trigger as the initial clue rather than an artificial reasoning
boundary: within read-only authority the model may iteratively inspect related
canonical resources, relationships, metrics, timelines, changes, and other
governed evidence needed to test hypotheses. Governance constrains mutation
authority, data policy, and resource access; it must not add approval-shaped
friction to each safe investigative step. Scoped Patrol must still
resolve canonical/source IDs and unique aliases before collection, reject
   ambiguous or unmatched API identities synchronously, persist an error run
   for runtime zero-match races, record requested and effective identities,
   and summarize only findings within the effective scope. Watch may mutate
   only finding lifecycle state; Pro investigation remains structurally
   read-only. Its finding resource is an observation anchor, never a search
   fence: the Pro prompt must explicitly require evidence-led expansion,
   competing-hypothesis testing, and a distinction between affected and causal
   resources. When evidence selects a different remediation target, the
   investigation-only `patrol_action_capabilities` lookup exposes that exact
   resource's current typed catalogue without authority or proposal side
   effects; only then may `patrol_propose_action` leave one side-effect-free
   typed proposal for the canonical approval, execution, and independent-
   verification lifecycle.

1. Update this contract when canonical AI runtime or transport entry points move, including transport-level provider request-shape changes such as OpenAI-compatible `tool_choice` handling, runtime-failure classification splits (for example separating tool-choice request rejection, no tool-capable endpoint, and generic model-level lack of tool support into distinct causes), Patrol-specific verification surfaces such as `POST /api/ai/patrol/preflight` that exercise the full chat-completions path with a minimal tool definition rather than only listing models, Patrol-preflight cache observability where the AI Service caches the most recent preflight outcome (success, soft warning, or classified failure) and the AI settings response surfaces it as `patrol_preflight` so the UI can hydrate a "last verified" indicator without forcing operators to re-run preflight on every page load, the auto-trigger contract on `HandleUpdateAISettings` where the save handler runs `TriggerPatrolPreflightAsync` only when the change actually moved Patrol transport (model swap, provider key for that model changed, or assistant just enabled with a Patrol model) so routine settings saves do not burn provider tokens, the startup-seed contract where the AI Service handler dispatches the same async preflight on Pulse boot when assistant is enabled and a Patrol model is configured so the cache is populated for the first `/api/settings/ai` poll after a restart instead of blanking back to "never verified", the readiness-integration contract where the `tools` check in the Patrol readiness payload consults the cached preflight and surfaces the classified evidence (success, soft warning, or failure with classified summary plus "last preflight <age>") for the configured provider+model when available (falling back to the static `PatrolToolReadinessForModel` classifier only when the cache is empty or holds a result for a different model), the preflight-runtime-recovery contract where a successful Patrol preflight with an observed tool call resolves the synthetic Patrol runtime failure finding while failed or no-tool-call preflights leave it active, the stateless-Patrol-input contract where `ExecutePatrolStream` must pass only the current run's user prompt into the agentic loop rather than reloading the persisted `patrol-main` session history (so a prior run that ended with orphan `tool_calls` cannot poison every subsequent run with malformed conversation structure), and the deterministic-resolve-gate contract where the `patrol_resolve_finding` tool adapter rejects LLM-driven resolves of event/persistent category findings (`backup`, `reliability`, `security`, `general`) when a deterministic verifier exists for the finding's key and that verifier either still detects the failure signal **or returns an inconclusive result** — preventing the LLM from optimistically resolving a finding its current investigation simply didn't re-surface, which was the source of the "Backup failed" flap (detected → auto-resolved → re-detected ten times in a day before this gate). The fail-closed-on-inconclusive policy treats verifier errors (timeouts, executor unavailability, transport faults) as "we don't know" rather than "go ahead": resolution of an event/persistent finding is effectively permanent (next detection registers as a regression and inflates counters), so the safe default is to refuse and require either a successful re-verification or operator action. The gate has a symmetric counterpart, the verified stale-resolve contract: `reconcileStaleFindings` (`internal/ai/patrol_ai.go`) may auto-resolve an event/persistent finding that was seeded but neither re-reported nor resolved ONLY when the finding's key has a deterministic verifier and that verifier affirmatively confirms the failure signal is gone — absence of a re-report remains insufficient evidence for these categories, "still present" or inconclusive verification leaves the finding active (the same fail-closed default), and verifications are capped per reconcile pass (`maxVerifiedStaleResolvesPerRun`) with deferred candidates logged and retried on the next successful full patrol. Without this counterpart the lifecycle was asymmetric: a genuinely fixed backup or recovered service stayed an active finding indefinitely unless the LLM happened to call `patrol_resolve_finding`. `hasDeterministicVerifierForKey` (`internal/ai/patrol_findings.go`) is the single source of truth for which keys have verifiers, consulted by both the gate and the reconcile pass, and must stay aligned with the dispatch switch in `verifyFixDeterministically` (it previously listed two of the seven dispatch keys, silently skipping verification that existed). Finding keys normalize onto the canonical verifier vocabulary in `normalizeFindingKey` via an alias map of unambiguous directional synonyms (`high-cpu` → `cpu-high`, `high-memory` → `memory-high`, `high-disk` → `disk-high`) so deduplication and deterministic verification meet on one key, and the `patrol_report_finding` key guidance (`internal/ai/tools/tools_patrol.go`) teaches the canonical vocabulary; semantically distinct keys (`pbs-job-failed`, `node-offline`) must not be aliased onto verifier keys whose resource model they do not match, the assessment-recovery contract where the overall-health "Recent Patrol errors" coverage factor in `summarizeRecentPatrolCoverage` suppresses the score penalty once three consecutive trailing successful full Patrol runs exist at the most-recent end of the recent-runs window — so the grade reflects current reality after a Patrol-affecting bug is fixed rather than dragging stale failures forward for the ~9 hours it takes scheduled runs to age them out of the trailing-10 ratio, the orphan-tool-call-repair contract where `convertToProviderMessages` injects synthetic is_error tool result messages for any `tool_call_id` in an assistant message that has no matching downstream tool result, so a chat session that ended mid-tool-call (network drop, ctx timeout, browser crash) cannot poison its next message with the structural-violation error the provider rejects — the synthetic content is marked is_error=true and explains the interruption so the model can retry the call or proceed without the data, and the patrol-session-bound contract where `ExecutePatrolStream` calls `SessionStore.TrimMessages` after persisting each run's messages to cap the patrol-main session at 200 messages (roughly two recent runs' worth) — without the bound the file grew unbounded at every scheduled run, reaching 16 MB and 3,593 messages within a month and making every `AddMessage` rewrite linearly more expensive; the canonical Patrol forensic log is the `PatrolRunRecord` history surfaced at `/api/ai/patrol/runs`, not the chat-session-shaped file
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
   tool call, must use the safe handle for scoped reads, may use the attached
   PII-free operational context (service access pattern, config/data/log paths,
   ports) to answer and guide the operator, must still refuse to expand
   environment variables, credentials, or other secret-bearing content, must not
   reveal raw hostnames, IPs, or aliases, and must not leak configured forbidden
   resource details in content or tool inputs.
   Plain-text resource references in live read, log, verification, or
   command-intent Assistant prompts may use the same selected-resource handle
   only after backend-owned canonical inventory resolution proves exactly one
   resource match, including explicit target-kind wording such as host, node,
   VM, container, or storage when inherited node labels would otherwise collide.
   That path must register the resource in the session resolved context, mark it
   as explicit current-turn access, and prepend a safe resource-context directive
   that exposes `current_resource` but not raw aliases, hostnames, platform IDs,
   paths, or other policy-redacted labels to external providers. Because that
   provider-safe rewrite replaces the user message, plain-text resolution runs
   only as a fallback when the prefetcher did not already resolve a structured
   mention or handoff resource; otherwise it would discard the injected
   cloud-safe operational context. Ambiguous or
   non-live prompts must fail closed to normal model clarification/query
   behavior; this is not a prompt-keyword router and must not choose, retry, or
   execute the model's next investigative action.
   Context-only resource handoff turns must be enforced at the tool-manifest
   boundary, not only by prompt wording: unless the operator explicitly asks for
   live runtime verification, a read attempt, or discovery execution, the
   Assistant loop must receive no tools and must answer from the attached
   context or state that Pulse does not currently have that fact.
   Resource-context model packs and drawer handoff briefings must carry the
   canonical discovery readiness state (`fresh`, `stale`, `missing`, `running`,
   `failed`, `unavailable`, or `unsupported`) with provenance and freshness
   metadata so Assistant can explain whether it is grounded in current
   discovery data before choosing any tool. Drawer handoffs route their attached
   resources through the same context-prefetch path as explicit `@`-mentions, so
   on a cloud turn the resource's cloud-safe operational context (access pattern,
   config/data/log paths, ports, and discovery freshness/age) reaches the model
   exactly as it does for an `@`-mention; identifying fields (hostname, IP, alias)
   stay redacted at the model boundary. The cloud-safe context must carry the
   discovery's age (a non-identifying timestamp) when known so the model can
   caveat staleness rather than presenting cached operational detail as current.
   The handoff path must not withhold operational context that the `@`-mention
   path delivers.
   The base Assistant system prompt must instruct the model to use this
   provenance: briefly attribute facts drawn from Pulse context to their source
   (a discovered fact's source, a metric/event timestamp), caveat stale or cached
   context instead of presenting it as current, and keep attribution concise and
   inline rather than citing every sentence.
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
   generic analysis-failed copy. If the provider fails after completing tool
   calls or accepting finding lifecycle operations, `runAIAnalysisState` must
   return that partial result alongside the error and the full/scoped run must
   preserve its completed tool records, token usage, accepted finding IDs,
   assessments, and display-safe partial response in `PatrolRunRecord`. This is
   diagnostic evidence only: the run remains `status=error`, must not trigger
   success-only runtime-finding recovery or absence-based reconciliation, and
   `internal/ai/qualification` must retain its diagnostic score while applying
   a hard failure for non-zero run errors or error status. Demo-mode Patrol run
   records must carry
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
   Readiness checks may retain stable machine IDs such as `configuration`, but
   human-facing labels in the settings payload must frame the operator boundary
   as Patrol mode rather than Patrol configuration.
   Monitor-only Patrol autonomy saves are part of the same runtime gate:
   when the safe-remediation extension or entitlement is unavailable, both the
   browser state owner and `internal/api/ai_handlers.go` must clear stale
   full-mode unlock state while clamping autonomy back to `monitor`, so paid
   remediation permission cannot survive through a free runtime save.
   Event-triggered Patrol runtime policy is also part of this gate: when local
   development background automation or another runtime policy pauses automatic
   alert/anomaly-triggered checks, `internal/ai` must expose an explicit
   event-trigger block reason/message on trigger status while preserving the
   operator's configured alert/anomaly trigger preferences.
   Patrol update-safety observation is part of the same read-state runtime
   boundary: Docker host/container snapshots used to detect image-digest
   divergence must be sourced from canonical `ReadState.DockerHosts()` views,
   with model-shaped data limited to the watcher adapter input rather than
   direct `StateSnapshot.DockerHosts` reads in the Patrol run loop.
4. Keep discovery scheduling authoritative through `internal/config/ai.go`: `discovery_enabled` and `discovery_interval_hours` must govern both lightweight infrastructure discovery and deep service-discovery background loops. `internal/api/ai_handlers.go` must preserve an explicitly supplied `discovery_interval_hours: 0` as the manual-only setting and may only apply the 24-hour default when discovery is enabled without an explicit interval payload. Discovery analysis remains a Pulse tool-led model workflow: Pulse supplies agent/API/metrics evidence and cache orchestration, while the selected model provides the intelligence. Background, settings-triggered, or drawer-triggered discovery progress must describe that discovery analysis directly and must not imply a live Pulse Assistant chat transcript unless the run is actually executing inside the chat surface. Settings-triggered manual discovery is an explicit operator refresh and must not open, append to, or masquerade as a Pulse Assistant session. Assistant and Patrol access to discovery must stay behind the governed `pulse_discovery` tool, including an explicit forced refresh action for known resources, while settings-level manual runs must use the canonical `/api/discovery/run` new/changed/stale/repairable sweep rather than a frontend-only shortcut. Discovery analysis must use the selected discovery route only: provider-owned default resolution may choose a model before the request when no explicit route exists, but `AnalyzeForDiscovery` must not fall back to a stale default provider or another configured provider after selected-route provider construction or execution fails. Fresh-but-unidentified service-discovery records are not complete when canonical resource metadata, stored facts, or safe command evidence can deterministically identify a known workload and endpoint; cached reads and the canonical sweep must repair those records instead of presenting `Unknown Service` as fresh.
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
   The `pulse_discovery` tool response must carry each discovered fact's
   provenance — the `source` that produced it and its `confidence` — alongside
   the fact's category/key/value, so the model can attribute and weight what it
   reports instead of stating untraceable facts. Both are omitted when empty.
   When the shared registry blocks a control tool in read-only mode, its
   operator guidance must point to Pulse Intelligence > Provider & Models
   settings and the Pulse Assistant Permissions Control mode, not legacy Pulse
   Assistant settings paths.
   Runtime status, preflight, settings-persistence, profile-suggestion, and
   remediation-impact messages must preserve the same product boundary:
   use Pulse Assistant only for the first-party in-app Assistant runtime itself,
   Provider & Models for the shared provider/model settings surface, and Pulse
   Intelligence for the shared Assistant/Patrol/agent substrate rather than
   reviving the legacy generic `Pulse AI` label or `Pulse Assistant settings`
   copy. Pulse Intelligence external-agent setup copy may point to Patrol mode,
   but it must frame the first step as setting how autonomous Patrol should be
   before connected agents request work; it must not present connected-agent
   setup as a separate operations-loop, MCP-readiness, or feature-unlock model.
9. Keep self-hosted Patrol messaging aligned with the v6 GA product contract:
   ordinary self-hosted installs use BYOK or local providers, and the runtime
   must not surface retired managed-model credits, trial prompts, account-backed
   AI activation, or general hosted chat entitlement in the normal app.
   The shared app shell must also keep `/cloud` and `/cloud/signup` out of
   ordinary self-hosted public routes so Cloud acquisition cannot reappear as a
   proxy for retired hosted-model or AI quickstart activation.
   The public Pulse Intelligence overview must likewise use productized context language such
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
    deferred to pulse_docker's existing tool-level runtime inspect
    check (`docker inspect` or `podman inspect`, according to the
    resolved container runtime); classes without a derivable verification command leave
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
    proxmox-ct-\* container lifecycle classes) and return hand-authored
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
    present. Patrol mode save-failure handoffs remain
    `patrol_configuration_failure` sessions for compatibility and may expose only the safe
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
    through the governed approval and fix flow. Patrol provides the
    configured LLM with observed finding context, evidence, policy posture, and
    governed action state; the LLM owns diagnosis and fix reasoning.
    Operator-visible handoffs must not describe Patrol as having already
    authored the correct fix. The finding briefing may carry only
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
    omit raw commands, preflight bodies, and execution output. That refresh is
    bounded, not exhaustive: the list is newest-first and only the
    `maxSessionHandoffRefreshPerList` (20) most recent action-carrying
    sessions are re-checked per list call, which covers every summary the
    recent-sessions surface and picker top present; deeper history self-heals
    because the send path refreshes the active session's handoff actions on
    load before the turn executes.
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

Assistant compatibility audit writers no longer use whole-record action-audit
replacement. Fresh approval and direct-action records use atomic creation with
their initial events, while decisions, refusals, execution starts, and results
use the same typed CAS transitions as the canonical action lifecycle. This
keeps legacy boundary producers from reopening the state-rewind path while
their mutation-plane consolidation remains separately governed.

AI provider model-cache identity must never expose reusable credential material
or deterministic unkeyed credential hashes. `internal/ai/service.go` derives
cache-only credential identities with a process-local random HMAC key, so an
unchanged configuration remains cacheable during the process lifetime while
credential rotation invalidates the cache without creating an offline
credential verifier.

The canonical findings store must present one active Patrol issue per real
problem. When the model reports an equivalent active sibling under a different
finding ID, key, or severity for the same resource, `internal/ai/findings.go`
must merge it into the existing active finding before it reaches
`/api/ai/patrol/findings`, keep the highest observed severity for the merged
issue, increment recurrence, and record a bounded duplicate-merge lifecycle
event instead of creating a second operator-facing row. Distinct symptoms on
the same resource, such as CPU pressure and memory pressure, must remain
separate findings. Verification is
`go test ./internal/ai -run 'TestFindingsStore'` plus
`go test ./internal/ai -run 'TestPatrolService_GetAllFindings|TestPatrolService_GetFindingsForResource|TestPatrolService_GetAllFindingsIncludingResolved'`.

Public Pulse Intelligence overview, safety, and onboarding copy now use the
same visible Patrol mode labels as the product: `Watch only`,
`Ask first`, `Safe auto-fix`, and `Autopilot`. The legacy `Only watch`,
`Ask before changes`, `Fix safe issues`, `Auto-fix safe issues`,
`Full control`, and `Policy autopilot` names may remain only as historical
context or compatibility implementation details, not as customer-facing Patrol
mode labels.

Assistant provider-readiness repair actions now use the canonical Pulse
Intelligence > Provider & Models route
`/settings/pulse-intelligence/provider`. The legacy `/settings/system-ai`
route remains a compatibility alias for old deep links, not a href emitted by
new Assistant provider-repair actions.

Primary platform route memory in `frontend-modern/src/AppLayout.tsx` may
preserve platform-local query state such as workload filters, but that memory
remains navigation chrome only. It must not fork Assistant drawer state, Patrol
utility route state, prompt context, resource reads, commercial posture, or
cross-platform query parameters.

Rejected Patrol investigation-fix approvals are terminal governed-action
decisions in the AI runtime. `/api/ai/approvals/{id}/deny` must persist the
approval-store denial, record a rejected unified action-audit decision when the
approval carries a governed action plan, and move the owning finding to
`fix_rejected` so Assistant handoffs, Patrol summaries, and external agent
adapters see the declined fix as explicit loop state rather than a disappeared
pending approval.

Legacy Assistant provider-tool declarations for manifest-backed Patrol finding
lifecycle tools now consume the manifest-owned raw input schemas through
`agentcapabilities.ProviderInputSchemaFromRaw`. The native Assistant runtime
may keep first-party model-facing descriptions, but it must not re-declare
required arguments such as `resolution_note` or dismissal `note` when the
canonical Pulse Intelligence manifest and API contract mark those fields
optional.
Legacy native Assistant utility provider aliases for `fetch_url` and
`set_resource_url`, plus their provider JSON schemas, are owned by
`agentcapabilities.LegacyAssistantUtilityProviderTools`. The older native
Assistant service may continue to expose those compatibility aliases while the
execution migration proceeds, but it must consume the shared provider projection
and shared argument constants rather than carrying inline schema maps or local
tool-input string keys. The legacy `run_command` alias is retired: the Assistant
must not offer it, and a fabricated invocation must fail closed before any
command dispatch. Raw command execution remains available only through the
governed capability and action-lifecycle path.
Native Assistant registry tools that operate on Patrol finding lifecycle state
must also consume the same `agentcapabilities` argument vocabulary for
`finding_id`, `resolution_note`, `reason`, and `note` instead of repeating
local field-name strings in their provider schema or execution maps.

Anthropic runtime provider execution is API-key backed only. Legacy Anthropic
OAuth tokens may remain in encrypted settings solely for disconnect cleanup;
they do not make Anthropic configured, do not instantiate a provider, do not
refresh tokens, and cannot send model requests.

The done event carries `context_limit_tokens` (the active model's context
window from `providers.ContextWindowTokens`) alongside token usage, and the
drawer's last-turn summary renders input tokens as a percentage of that
window ("8,500 tokens (6% of context)") so operators can see compaction
coming; when the limit is absent the summary renders exactly as before.
The done event also carries `session_cost_usd`, the estimated cumulative
spend for the session summed from the operator cost ledger
(`cost.Store.SessionCostUSD`) across the session's recorded events: chat
turns, compaction, and title calls, all of which stamp `session_id` on
their `cost.UsageEvent`. The figure is omitted whenever any of the
session's events prices against an unknown model, and free local models
(Ollama) price known-at-zero, so the drawer's last-turn summary appends
"$0.12 session" only when the whole session is honestly priceable and
non-zero. The summary never invents a client-side pricing table; the
backend ledger is the only cost authority.

Post-tool model turns mark the status handoff: every agentic turn after the
first emits a `model_processing` workflow state ("Working on the response
with the gathered results.") before the provider call, so a provider that
reasons server-side without streaming events can never leave the previous
tool's status stale on screen; `model_thinking` still upgrades the status
when reasoning deltas actually arrive. Pinned by the interaction-quality
corpus scenario "post-tool model turn marks the status handoff".

Session titles are model-generated after the first exchange
(`internal/ai/chat/session_title.go`): the chat runtime upgrades the
auto-truncated first-prompt placeholder once the done event has reached the
client, never overwrites a user-set title (placeholder comparison plus a
pre-persist re-check), passes the model-boundary sanitizer, records usage as
`assistant_session_title`, and leaves the placeholder on failure or timeout.
The long-form contract lives with the session rename clause in Extension
Points.

Finding identity is the LLM-assigned key (resource+category+key hash → ID),
and key collisions are surfaced, not forked: when a same-ID re-detection's
title shares essentially no keywords with the existing finding's title
(`keywordOverlap` at or below `findingIdentityShiftMaxTitleOverlap`,
`internal/ai/findings.go`), the merge proceeds — the latest report still owns
the text, because key forking would split LLM rephrasings of one real issue
into duplicate findings — but `FindingsStore.Add` appends a
`content_replaced` lifecycle event preserving the previous and new titles in
metadata and logs the collision. This keeps the operator timeline honest
when a distinct issue reuses an existing finding's identity (including the
resolved case, where the reactivation otherwise reads as a regression of the
old issue: both `content_replaced` and `regressed` events appear).
Rephrasings and identical re-detections stay event-free per the
heartbeat-not-transition rule. The shared lifecycle presentation
(`frontend-modern/src/utils/aiFindingPresentation.ts`,
patrol-intelligence subsystem) labels the event "Re-detected with different
details" — see that contract's Current State entry for the UI-side rule.
`TestFindingsStore_KeyCollisionRecordsContentReplacedEvent` and its
companion tests (`internal/ai/findings_lifecycle_test.go`) pin the behavior.

The interaction-quality scenario corpus
(`internal/ai/chat/interaction_scenario_corpus_test.go`) is the canonical
regression home for chat-feel promises, mirroring the Discovery corpus
(`internal/servicediscovery/scenario_corpus_test.go`). Each scenario drives a
full `ExecuteStream` turn against a scripted provider and pins the
browser-facing event stream — ordered event types, forbidden event types,
answer-text teeth, and payload teeth — for the user-visible promises shipped
in the OpenCode-restraint arc: clean plain/greeting turns with no tool noise,
compact tool events with real tool names, the clean no-narrative fallback
sentence (no raw JSON or provider call ids), invisible pre-event provider
retries, and exactly one clear error event on terminal provider failure.
Interaction-quality fixes must land with a corpus scenario stating the
promise that would fail without the fix, so loop refactors (including
Patrol-phase work on the shared agentic loop) cannot silently regress the
chat feel.

The Assistant system prompt's missing-target policy is resolve-before-asking
(`buildSystemPromptWithToolGovernance`, `internal/ai/chat/service.go`): a
command or diagnostic request that names no target sends the model to
read-only query/topology tools first; a sole plausible match is used directly
for read-only diagnostics and named in the answer; the operator is asked only
when several plausible targets remain or the action changes state. This
supersedes the ask-first framing ("Missing target information is not a safe
default") that deflected every untargeted "run X" request back to the
operator even on single-host deployments. Placeholder targets
(`current_resource` outside an attached-resource turn) remain forbidden in
all modes, and write actions still require an explicit operator-confirmed
target. `TestBuildSystemPrompt_CurrentResourceRequiresResourceHandoff` pins
the boundary strings; the full Extension-Points entry sits beside the
model-owned tool-manifest rule.

The per-turn Assistant system prompt carries the current wall-clock time (the
Pulse server clock) so the Assistant answers "what time/date is it" directly
instead of deflecting ("I don't have access to a real-time clock") or demanding
a `target_host` just to run `date`. The timestamp is appended in
`AgenticLoop.getSystemPrompt` (`internal/ai/chat/agentic_prompt.go`) rather than
baked into `baseSystemPrompt`: the base prompt is frozen when the loop is
constructed at service start, so anything that must stay fresh per turn
(execution mode, current time) is appended on each turn. The current time
carries no PII and is therefore safe on cloud-routed turns regardless of the
operational-context sharing opt-in.

The streaming transcript list reconciles messages by `id` rather than rendering
the raw message array by reference. `useChat` rebuilds its message array
immutably on every stream event (each content chunk, workflow-status change, and
tool update spreads a brand-new message object), so a reference-keyed `<For>`
would tear down and recreate the whole `MessageItem` on every event — the
visible flashing, rows popping in and out, and transcript jumping up and down
during a turn. `ChatMessages` (`frontend-modern/src/components/AI/Chat/ChatMessages.tsx`)
reconciles the incoming array into a keyed `solid-js/store` mirror so each
message keeps a stable identity across updates; `MessageItem` already reads every
field through accessors, so only the genuinely changed text/rows update in place.
This keeps the live transcript stable the way OpenCode's timeline is.

The same stability rule applies one level down, to the per-message
stream-event/tool-row list inside `MessageItem`. `groupStreamEventsForDisplay`
remaps blocks to fresh objects on every tick and `StreamDisplayEvent` carries no
id, but the grouped list is strictly append-ordered (the grouper only pushes new
blocks or mutates the open content/thinking block in place — it never inserts
mid-list or reorders). It therefore renders through `<Index>` (positional
keying), not a reference-keyed `<For>`: each row keeps its DOM node across
event-object rebuilds at a stable position, so the streaming answer block and
completed tool rows update in place instead of re-mounting (and re-parsing
markdown) on every content delta. Positional keying is correct here precisely
because the list is append-only; a list that could reorder or key by identity
must not use `<Index>`.

The streaming answer block itself renders by morphing the DOM, not by replacing
`innerHTML` on every tick. `renderMarkdown` re-parses the full (DOMPurify-
sanitized) answer on each paced reveal; assigning that to `innerHTML` rebuilds
the entire prose subtree, so a multi-paragraph or list/table answer flickers and
reflows every earlier line as it streams. `AssistantMarkdownBlock` instead feeds
the sanitized HTML to `morphMarkdownInto`
(`frontend-modern/src/components/AI/Chat/markdownMorph.ts`), which reconciles old
and new trees in place: structurally-identical nodes are left untouched, same-tag
nodes are morphed (recursing into children so a growing `<ol>`/`<table>` keeps
its earlier `<li>`/`<tr>` and only the last one updates), and the growing tail
block updates its text node rather than being rebuilt. The morph is a rendering
optimization only — `renderMarkdown` remains the sole sanitization gate and the
morph must never be fed unsanitized HTML.

Live workflow status is footer-owned, not a transcript artifact (the OpenCode
restraint model — see the canonical statement in Extension Points, which
supersedes the older per-row transcript-status rules in this contract). Earlier
Pulse rendered "Preparing context / Reading inventory / Counting / Waiting for
assistant / retrying" both as transcript rows AND in the activity dock, so the
chat narrated every internal step into the timeline. Now `MessageItem` never
renders workflow status — neither the per-event `role="status"` row
(`shouldRenderWorkflowStatusEvent` is `false`) nor the early-phase header chip
(`shouldShowHeaderWorkflowStatus` is `false`). The transcript carries only
durable artifacts (reasoning, tool calls, model-route rows, the answer). The
single live "assistant is working" indicator is the activity dock above the
composer, which shows a spinner + the current status + the model route + Stop.
The dock is gated on `assistantTurnActive` (loading OR a streaming assistant
message) rather than `chat.isLoading()` so it persists for the whole turn:
`isLoading` flips false at visible-turn-complete, which previously made the dock
flash its status for a frame and vanish.

Prompt wording must not gate tool availability at all. The greeting/meta
text-only classifier (`assistantPromptLooksConversational`) is removed along
with the rest of the `assistantToolScopeForPrompt` keyword router: even "hi"
or "thanks" turns carry the full governed manifest, because tools offered is
not tools used — the model answers a greeting without calling anything, while
short prompts that look conversational to a keyword list are usually resource
lookups ("hows esphome", "check frigate", "grafana cpu") that need the
query/read/metrics tools to answer from Pulse data. Withholding tools by word
count or greeting keywords made the Assistant tell the user it had "no
infrastructure context or diagnostic tools" and ask them to run `docker ps`
themselves, for a resource Pulse already inventories. Tool selection stays
model-owned: offer the tools and let the model decide whether to use them,
rather than pre-deciding from prompt keywords/length that no tools are
needed. `TestService_ExecuteStream_ToolManifestIsModelOwned` is the
regression proof.

The `pulse_query` `get` action (`executeGetResource` in
`internal/ai/tools/tools_query.go`) must accept a canonical resource handle
(`<type>-<hash>`, e.g. `system-container-599a2e3…`) as `resource_id` without a
separate `resource_type` and infer the type from the handle, because that is how
a model naturally references a resource it read from context. Failing such a call
with "resource_type is required" burns visibly-failed tool calls in the
transcript before the model recovers with the numeric id. The type is recovered
from the trailing hex hash segment (no type word is all-hex); a bare numeric
VMID still requires an explicit `resource_type`.

For a resource resolved through the unified-resource provider, every
`pulse_query` `get`, `list`, and `search` response must keep `id` equal to the
canonical unified resource ID. Provider-native identifiers such as a Docker
container ID remain accepted lookup aliases and internal execution targets,
but they must not replace the canonical ID in model-visible resource results.
Otherwise a model can faithfully copy Pulse-provided evidence into a Patrol
finding and still mis-link that finding to a source record instead of the
canonical resource selected by the operator or scorer.

When the model runs tools but returns no final narrative, the deterministic
fallback summary (`buildAutomaticFallbackSummary` in
`internal/ai/chat/agentic_final.go`) must read as a clean operator message, not a
dump of internals. It names the tools it ran by their real tool name resolved
from the assistant tool calls (`pulse_` prefix stripped) — never the opaque
provider call id (`call_…`/`toolu_…`/`fc_…`), which must not leak into
chat-visible text — and it must not append raw tool output / result snippets.
The earlier form ("I completed N successful check(s) using call_27f0f389…, …
automatic summary. Latest successful result snippet: {…raw JSON…}") is forbidden:
provider call ids and raw tool JSON are not operator-facing answer content.

Assistant slash-command availability is part of the command runtime contract,
not only visual polish. The OpenCode reference at fetched `origin/dev` commit
`c495635` filters prompt slash commands through the registered command catalog
and omits disabled builtin commands from the prompt popover in
`packages/app/src/components/prompt-input.tsx` (slash command list creation,
lines 677-699), while command metadata owns `disabled` and `slash` in
`packages/app/src/context/command.tsx`. Pulse adapts that by deriving command
availability from the same predicates as the drawer toolbar: prompt slash
autocomplete hides unavailable local commands, command help may show them
disabled with a reason, and manual slash submissions respect the same
availability before running local session actions.

Assistant provider-route recovery must read as explicit operator action, not
automatic fallback. The OpenCode reference at fetched `origin/dev` commit
`914a643` retries the selected provider/model route through
`packages/opencode/src/session/retry.ts` and
`packages/opencode/src/session/processor.ts` by passing
`input.model.providerID` into `SessionRetry.policy`, renders retry state in
`packages/ui/src/components/session-retry.tsx`, and records provider/model
changes as explicit `ModelSwitched` session events in
`packages/core/src/session/event.ts` plus `packages/opencode/src/session/prompt.ts`.
Pulse adapts that by keeping same-route retry visible, ignoring obsolete
provider route-switch metadata without changing the selected route, and labeling
failed-turn or readiness recovery buttons as explicit route/model-route choices
instead of implying automatic route adoption.

Assistant model-catalog failure is selector-local state, not a drawer
initialization failure. The OpenCode reference at fetched `dev` commit
`3867fa2bad0e644166e360e2e99cfe426fe71105`
`packages/opencode/src/cli/error.ts` lines 58-69 formats missing model/catalog
state as an operator-facing model-selection problem with a list-models hint,
while `packages/opencode/src/session/llm.ts` lines 96-104 resolves the
selected provider/model route independently for the stream. Pulse adapts that
by letting Assistant sessions, settings, route health, and the composer finish
opening when `/api/ai/models` fails; the catalog error stays attached to the
model selector and may be refreshed explicitly, but startup must not log or
render a broad Assistant initialization failure, and it must not replace the
selected model route.

Assistant completed-turn chrome is route-owned summary, not raw usage output.
The OpenCode reference at fetched `dev` commit
`3867fa2bad0e644166e360e2e99cfe426fe71105` imports `turnSummaryCommit` in
`packages/opencode/src/cli/cmd/run/scrollback.surface.ts` lines 19-20, while
`packages/opencode/src/cli/cmd/run/turn-summary.ts` lines 5-46 emits a final
system summary with agent, model, and duration. Pulse adapts that by deriving
the latest completed, non-streaming assistant turn and surfacing its effective
provider/model route, duration, and backend-proven token total in low-priority
composer chrome. Input/output usage remains title/accessibility detail only,
missing provider usage does not hide route and duration, and active streaming
turns never appear as the completed-turn summary.

Assistant tool activity now follows an OpenCode-referenced chronological row
model where appropriate for Pulse. Consecutive context/read/query tools render
as visible transcript rows in arrival order instead of being replaced by a
grouped context footer, while command previews, inputs, progress, and large
outputs remain contained inside each tool row. This keeps the user-facing stream
feeling active without dumping large command output into the default answer.
Fast tool completions must also stay visibly live long enough to be perceived:
the frontend stream reducer stamps sub-420ms successful tool completions with a
transient settle deadline, and the row renders that deadline as a running state
even if the turn's `done` event has already arrived. The referenced OpenCode
source at fetched `dev` commit `e82542b8023a8374f29c23b70ec019c8f256354e`
implements the same user-visible principle in
`packages/opencode/src/cli/cmd/run/session-data.ts` by emitting a `start` commit
for running tools and a later completed/error commit instead of only surfacing a
batched terminal transcript. Pulse adapts that as an in-memory UI settle window
because Pulse transcripts persist completed tool facts, not OpenCode scrollback
commit phases.
The 2026-06-08 live-tool-start slice rechecked OpenCode source at fetched `dev`
commit `3867fa2bad0e644166e360e2e99cfe426fe71105`:
`packages/opencode/src/session/processor.ts` creates the visible tool part in
`ensureToolCall` (lines 299-349) and moves that same part to `running` on
`tool-call` (lines 507-521). Pulse adapts that by making `ToolStartData`
self-describing live activity: `tool_start` must carry a running phase when
execution is visible, and the frontend must render a new start row as running
immediately even if `tool_progress` arrives in a later paint.

Assistant provider retries are a first-class visible workflow state, not a
hidden server log. The referenced OpenCode source at fetched `dev` commit
`7ae856a9e97130f664f6f11fa5871a2795de9902` defines retry session status in
`packages/opencode/src/session/status.ts` (`SessionStatus.Info` /
`SessionStatus.set`), wires retry updates from
`packages/opencode/src/session/processor.ts` through
`SessionRetry.policy({ set })`, and renders retry attempt/backoff state from
`status().type === "retry"` in
`packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx`. Pulse adapts
that contract at the owning stream boundary: `WorkflowStateData` carries
`attempt`, `max_attempts`, and `retry_after_ms`, and `AgenticLoop` emits
`provider_retry` before sleeping between transient pre-output provider
attempts. The frontend active-turn footer and live transcript workflow row
render that same typed workflow state immediately as compact attempt/backoff
progress, counting down `retry_after_ms` from `started_at` while the turn
remains active, so the user sees Pulse moving through a retry instead of
staring at an obsolete provider-wait message. Retry workflow states are not
held behind workflow-history pacing.

Assistant workflow progress is also a live typed activity row while a turn is
in flight, not only hidden footer state. The referenced OpenCode source at
fetched `dev` commit `7ae856a9e97130f664f6f11fa5871a2795de9902` stores
session status separately from message parts in
`packages/opencode/src/cli/cmd/tui/context/sync.tsx` (`session_status`,
`session.status` event handling) and renders a session from live messages,
permissions, questions, and running tool parts in
`packages/opencode/src/cli/cmd/tui/routes/session/index.tsx` (`Session`,
`messages`, `permissions`, `questions`, foreground `ToolPart` selection, and
`session.status` handling). Pulse adapts that source pattern with a transient
frontend `workflow_status` display event: each incoming backend
`workflow_state` replaces the prior workflow row, the active footer reads the
same typed status, and visible assistant content, reasoning, tool,
approval/question, terminal `done`, and terminal `error` events clear that row.
The frontend now also keeps a bounded live-only `workflowStatusHistory` for the
active assistant message so the reducer preserves state continuity while
backend preparation, context, and provider-start labels are being replaced. The
shared workflow-status presentation helper owns both the transcript row and the
active-turn composer footer so the two live surfaces show the latest canonical
workflow state immediately. Retry/backoff and stream-idle liveness states cut
through pacing because they are current route health, not neutral setup motion.
The transcript and footer therefore show current motion while the provider is
starting, retrying, or reasoning, but completed answers do not retain stale
internal-progress prose.
Stream-idle heartbeats are part of that same visible workflow state. When the
frontend knows the selected provider/model route from the backend workflow event
or the streaming assistant message, the idle heartbeat must render as
route-specific liveness such as `OpenRouter is still working; waiting for more
response data.` instead of reverting to generic Assistant waiting copy.
Prompt dispatch itself is also a visible live state: before any backend
workflow event returns, the frontend seeds the active assistant turn with a
local `request_send` / `Sending prompt.` workflow row after any selected-model
row. That row is explicitly transient: it is the current live state but does
not enter the paced backend workflow history, the first backend workflow state
may replace it, and content, thinking, tool, approval, question, model-route,
done, or error evidence removes it from the durable transcript. This adapts the
referenced OpenCode source at fetched `dev` commit
`e82542b8023a8374f29c23b70ec019c8f256354e`, where
`packages/opencode/src/cli/cmd/run/runtime.queue.ts` emits `turn.send` with
`sending prompt` at lines 174-184, `packages/opencode/src/cli/cmd/run/footer.ts`
maps `turn.send` and `turn.wait` to live footer status at lines 133-145, and
`packages/opencode/src/cli/cmd/run/stream.transport.ts` promotes quiet
post-send periods to `waiting for assistant` at lines 1350-1357. Pulse keeps
that send/wait distinction in the live chat turn by promoting an unreplaced
local `request_send` status to local `request_wait` / `Waiting for assistant`
when the browser API client receives an open SSE response but before any parsed
backend-visible SSE activity arrives, while preserving the Pulse transcript
rule that only backend workflow, route, content, and tool evidence remain once
real activity exists.

Assistant drawer shell status follows the same OpenCode-referenced separation
between prompt-adjacent operational status and transcript content. The
referenced OpenCode source at fetched `dev` commit
`e82542b8023a8374f29c23b70ec019c8f256354e` renders agent/model metadata beneath
the prompt and active/retry/interrupt state in the prompt footer in
`packages/tui/src/component/prompt/index.tsx`, with broader system status kept
in `packages/tui/src/routes/session/footer.tsx`. Pulse adapts that pattern for
the web drawer by keeping model route, recent-route cycling, control mode,
last-turn usage, active workflow progress, queued follow-ups, route-recovery
notices, and the autonomous control warning in the input-adjacent
composer/status rail.
Those items stay visible and actionable, but they do not compete with the
transcript as separate top-of-drawer banners unless they are provider readiness
or scoped handoff context surfaces with their own governed content.
Queued follow-up pressure is part of that active-turn state: the frontend
derives the count from queued user transcript turns, appends it to the primary
active Assistant headline while a turn is running, and keeps the separate
queued-follow-up row actionable for reorder/cancel operations.
That suffix must compose with live workflow pacing rather than disabling it:
when backend workflow states arrive in a burst, the active headline still walks
through the paced status sequence while retaining the queued follow-up count.
This preserves the OpenCode-referenced live-part feel from
`packages/tui/src/routes/session/index.tsx` lines 1492-1503 and the running
tool metadata updates in `packages/opencode/src/session/tools.ts` lines 54-63,
while adapting the interaction to Pulse's single active-turn plus queued
follow-up safety model.

Assistant chat actions follow the same OpenCode-referenced footer principle:
actions are reachable from prompt-adjacent chrome, not only from a hidden slash
draft. The referenced OpenCode source at fetched `dev` commit
`e82542b8023a8374f29c23b70ec019c8f256354e` builds the direct command surface in
`packages/opencode/src/cli/cmd/run/footer.command.tsx` (`RunCommandMenuBody`
with suggested actions, slash commands, and search) and keeps the footer command
entry discoverable from `packages/opencode/src/cli/cmd/run/footer.view.tsx`.
Pulse adapts that pattern for the web drawer by exposing the existing Assistant
commands dialog from the composer chrome and making that dialog searchable and
keyboard-selectable. The button, slash-triggered help, dialog search, and
keyboard selection must use the same `ASSISTANT_SLASH_COMMANDS` registry,
`filterAssistantSlashCommands`, `AssistantCommandHelpDialog`, and
`executeSlashCommand` path as `/help` and `/commands`; they must not introduce
a second command registry or a separate provider-bound prompt path. The command
dialog search box must compose the frontend-primitives `SearchField` so the
shared search-control registry owns clear-button behavior, input chrome,
keyboard forwarding, and native-search-input drift prevention while AI runtime
keeps ownership of command filtering and execution semantics.

Primary nav moved to governed platform/runtime destinations on 2026-05-16 and
was clarified on 2026-05-25 and 2026-06-25 through `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx`: the top of the app may expose canonical
platform pages (Proxmox, Kubernetes, TrueNAS, vSphere) plus the
Docker / Podman container-runtime lens (shown as Docker in the shell), aggregate
platform-owned Workloads / Storage / Recovery sub-surfaces, Alerts, Patrol, and
Settings. `Patrol` is the visible destination for Patrol-owned operator work; it
must route through the canonical `/patrol` surface and must not create a second
Assistant or Intelligence route namespace.
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
call (active maintenance window, `IntentionallyOffline`,
`NeverAutoRemediate`, and `Criticality`) so adding new signals later does
not multiply round-trips per finding. `FindingsStore.Add` stamps
`Finding.ResourceCriticality` from that projection on new and re-detected
findings. The field persists as `resource_criticality` and may affect
same-severity Patrol attention ordering only; it must not mutate
`Finding.Severity` or bypass severity escalation/resolution rules.

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
and `Criticality` alongside `IntentionallyOffline` and `MaintenanceWindow` so the
investigation read path and the suppression read path share a
single projection. The findings store exposes the projection via
`OperatorStateProjectionFor`; the suppression hot path keeps its
existing internal access. Both paths see the same operator-state
facts so investigation reasoning, suppression behavior, and Patrol
priority stamping cannot drift against each other.

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
history. The action broker consults the same `resource_operator_state` table
on every dispatch — both the agent-command path
(`executeCommandWithAudit`) and the native provider path
(`executeNativeActionWithAudit`) in
`internal/ai/tools/action_audit.go` run the shared
`checkRemediationLockForDispatch` gate — and refuses with
`unifiedresources.ErrResourceRemediationLocked` when the operator
has set `NeverAutoRemediate=true` on the target resource. Refusal
persists a Failed audit record whose `ErrorMessage` is prefixed
`resource_remediation_locked:` so the audit timeline shows every
refused dispatch, paralleling the `plan_drift:` shape from the drift
guard. Operator state outranks per-action approval — the broker
refuses even when the approval ID resolves and the plan hash matches.
When the lock state cannot be determined (no audit store wired, or
the operator-state lookup errors), dispatches that do not carry an
approved human decision fail CLOSED with
`ErrRemediationLockStateUnknown` and a Failed audit record prefixed
`remediation_lock_state_unknown:` — the broker must not assume an
unreadable lock is unset while Patrol or Assistant run autonomously.
Dispatches backed by an approved human decision keep the historical
fail-open posture on unknown lock state (the operator explicitly
signed off on the exact plan), with the degraded lookup logged. The `IntentionallyOffline` branch is the indefinite
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
Usage grouping rows must present product concepts such as Assistant sessions,
Patrol runs, Discovery runs, or concrete resource labels. Raw target storage
keys, opaque session identifiers, UUIDs, and values such as
`assistant_session_title:<id>` are accounting implementation detail and must
stay out of the operator-facing table.
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
Explicit saved model routes fail closed when their provider is not configured
or cannot be initialized. Pulse may not replace an operator-selected route such
as `deepseek:*`, `openrouter:*`, or `openai:*` with another configured provider
default during settings load, Assistant chat startup, or service initialization.
Provider-owned defaults are allowed only when no explicit route exists, or when
the same configured provider needs its governed chat-suitable/default model
because live catalog lookup failed or the explicit model is unsuitable for chat.
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
The global update progress watcher in `frontend-modern/src/App.tsx` is
likewise server-updater shell chrome, not assistant surface: its in-progress
stage vocabulary mirrors the backend updater pipeline (including the
`restoring` stage emitted by update rollback), and assistant state, drawer
ownership, and AI runtime surfaces must not key off those update stages.
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
`frontend-modern/src/utils/aiChatPresentation.ts` are the canonical owners
for provider naming, provider health labels, control-level semantics,
chat drawer title/subtitle, launcher title/aria copy, session-menu labeling,
discovery hint framing, chat/session empty states, assistant message and
question-card labels.
Discovery hint framing must follow the Pulse Intelligence settings IA: use the
simple `Discovery` label, and explain the concrete service context it unlocks,
without promoting workload discovery as another first-class product surface.
Settings and chat surfaces must consume those helpers instead of keeping local
AI wording or model/provider inference branches.
Assistant chat must not render Pulse-authored explore pre-pass cards or
internal workflow-state cards as assistant output. The user-facing stream is
model text, model thinking where supported, model-selected tool calls, governed
approval requests, and model questions; internal runtime telemetry stays out of
the chat transcript. The browser runtime may keep the latest `workflow_state`
message on the in-flight assistant turn only as drawer status text while waiting
for model content, so provider/session progress is visible without turning
runtime telemetry into transcript content.
Cold-start Assistant chat session creation is also stream-owned. Ordinary first
messages may call `/api/ai/chat` without a `session_id`; `chat.Service.ExecuteStream`
must create or resolve the durable session before provider execution and emit a
first-class `session` SSE event carrying `SessionData{ID: ...}`. The browser
chat runtime binds its active session from that stream event, with `done` and
`question` session identifiers retained only as compatible terminal/interactive
payloads, rather than issuing a separate `/api/ai/sessions` preflight before
the first user message. Explicit session-management actions may still create a
session through the session endpoint.
The Assistant drawer's `New` action is a local blank-conversation reset, not a
backend session creation. It must clear the active transcript, scoped handoff
context, and browser session ID immediately, leave the model selection intact,
and avoid adding empty conversations to the session list; the next submitted
message materializes the durable session through the stream-owned cold-start
contract above.
The drawer must also stay composer-first: when Assistant opens, starts a blank
conversation, or loads an existing session, the textarea is registered with the
shared `aiChatStore` focus owner and focused without requiring an extra click.
Global shell shortcuts may use that store focus boundary, but drawer-local code
must not fork a second input-focus registry.

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
Patrol-origin proposals from generic Assistant actions. Rejected Patrol
investigation-fix approvals must also enter that shared decision lifecycle:
`/api/ai/approvals/{id}/deny` records a rejected unified action-audit decision
when the approval has a governed action plan and moves the owning Patrol
finding to `fix_rejected`, so a declined fix remains a visible governed loop
outcome rather than disappearing when the pending approval leaves the queue.
The typed action-proposal seam in `pkg/aicontracts/action_broker.go` is the
successor to that command-shaped approval flow and the ONLY sanctioned route
from an enterprise Patrol investigation to a Pulse infrastructure mutation:
`OrchestratorActionBroker` exposes exactly `Capabilities` (read-only catalog
of a resource's advertised capabilities and parameter schemas, including
sensitivity and core-owned auto-authorization eligibility) and `Submit`
(typed proposal). The contract
deliberately omits org ID, requestedBy, autonomy, risk, approval-policy,
destructive, command, and target-host fields, and has no decide or execute
methods, so enterprise code can neither claim authorization nor dispatch;
authorization always derives inside core from the tenant Patrol mode,
capability eligibility, persisted per-resource allowlist/window, remediation
lock, and the canonical lifecycle. Enterprise receives the resulting pending
or terminal disposition, including honest verification status, but never an
authorization primitive. `OrchestratorDeps.ActionBroker` carries the seam and is
Policy-authorized submission enters `actionlifecycle.ExecuteUnderPolicy`; the
broker may evaluate eligibility to decide whether to attempt automatic
admission, but it cannot persist approval or call an executor itself. The
lifecycle re-evaluates the full policy under the shared admission coordinator
and atomically persists the typed authorization lease, policy approval, and
executing transition. Revocation before that transition produces zero dispatch
and no policy approval. Human approvals remain semantically separate.
`OrchestratorDeps.ActionBroker` carries the seam and is
now REQUIRED: the enterprise factory disables the orchestrator when it is
absent rather than falling back. The command-execution side doors are
gone. `OrchestratorChatService` exposes only investigation-specific
execution and listing (`ExecuteInvestigationStream` returning a
structured `OrchestratorInvestigationResult`, and
`ListInvestigationTools`); it has no generic `ExecuteStream`, no
`SetAutonomousMode`, no `ListAvailableTools`, and the request type carries
no autonomy field, so enterprise code can neither select a profile nor
grant execution authority. `OrchestratorCommandExecutor`,
`OrchestratorApprovalStore`, and the autonomy/fix-verifier/license
dependency interfaces are removed along with their pulse-side adapters
(`orchestratorChatAdapter.ExecuteStream`/`ExecuteCommand`/
`SetAutonomousMode`, `orchestratorApprovalAdapter`,
`autonomyLevelProviderAdapter`, `patrolFixVerifierAdapter`,
`licenseCheckerForOrchestrator`). The enterprise investigation
orchestrator no longer parses `PROPOSED_FIX`, runs guardrails, executes
commands, or queues command-shaped approvals: it consumes the structured
investigation result and, only for a non-nil proposal from a completely
successful run, submits through the broker and persists the resulting
`ActionReference`. Command text in investigation prose is inert
historical text with no execution, approval, or remediation-plan pathway
(`classifyInvestigationOutcome` reads only conclusion markers). The core
`PatrolService.generateRemediationPlanFromInvestigation` path that copied
`Fix.Commands` into an executable remediation plan is deleted, so no
second command-backed remediation artifact remains. The legacy
`aiautofix` command-approval endpoints fail closed
(`command_fix_retired`), never approving, re-arming, or executing a
persisted command-shaped fix. Investigations reference their canonical
action through the additive `Action *ActionReference` fields on
`InvestigationSession` and `InvestigationRecord`; the command-shaped
`ProposedFix`/`ApprovalID` fields remain readable for old persisted
records only and are never populated by new investigations nor re-armed
into executable approvals. Proposal parameters live in the canonical
action audit, not duplicated into investigation stores. Proofs:
`TestOrchestratorActionBrokerIsProposeOnly`,
`TestOrchestratorChatServiceIsInvestigationOnly`,
`TestOrchestratorDepsHasNoCommandOrAutonomyDeps`,
`TestActionProposalWireShapeIsTypedAndCommandFree`, and
`TestActionReferenceIsAdditiveOnInvestigationShapes` in
`pkg/aicontracts/contracts_test.go`, plus the enterprise
`TestProposedFixProseIsInertHistoricalText` negative regression.
Tool-call safety classification is registry-owned through the canonical
invocation descriptors in `internal/agentcapabilities/invocation.go`:
every registered Pulse tool has a static or discriminator-based
descriptor whose classification carries both a workflow kind
(read/resolve/write) and a mutation target (`none`, `pulse_state`, or
`infrastructure`). A mixed tool's descriptor must exactly cover its
schema enum for the declared discriminator (Kubernetes discriminates on
`type`, not `action`); `ToolRegistry.Register` panics on a missing
descriptor or on missing/extra cases, so an unclassifiable tool is not
registerable. Missing, malformed, or unknown discriminator values
classify fail-closed as write/infrastructure. Provider projection
(`ToolRegistry.ListTools`/`ListToolGovernance`) and runtime enforcement
(`ToolRegistry.Execute`, before the handler runs) consume the same
descriptor under one `InvocationPolicy` (control level plus the
request-local `deny_infrastructure_mutations` restriction on the
executor, which clones per session and never serializes): projection
removes forbidden enum values, drops tools with no permitted
invocation, and recomputes the offered governance action mode, so the
offered schema and the enforcement boundary can never disagree.
Control-level blocks keep returning the operator guidance message;
policy blocks return the shared invocation-blocked result.
Registration authority on the Assistant tool registry is split:
`registerBuiltin` is the construction-time path for canonical Pulse
tools (shared descriptor mandatory, override rejected), while
`RegisterExtension` - the only path exposed through
`PulseToolExecutor.RegisterTool` - rejects every canonical tool name
outright and requires the extension to declare its own descriptor.
Registry entries are append-only: a name registers exactly once, so a
later registration (with or without a descriptor override) can never
replace an already-governed handler such as pulse_read's
execution-intent-enforcing exec path.
The classification vocabulary is closed: descriptor validation rejects
any class outside the known kinds and mutation targets, and
`InvocationPolicy.Allows` independently denies unknown mutation targets
outright, so an unvalidated class still cannot execute. Descriptor
lookups and registration store deep copies, so callers can never mutate
the canonical table through shared case maps or static class pointers,
and registration rejects descriptor overrides for canonical tool names
(overrides exist only for genuinely non-canonical extension/test
names). The projected governance action mode derives from mutation
targets, not workflow kinds, and is recomputed even when no enum value
was removed; a projection whose remaining invocations mutate nothing
downgrades to scope-only approval metadata. Docker `check_updates`
classifies read/none: it queues a read-only scan and must not drive
verification workflow or make a read-only Docker projection look mixed.
Execution posture is profile-owned through the core-only, never-
serialized `tools.ExecutionProfile` (interactive Assistant, Patrol
detection, Patrol investigation). Both Patrol profiles are
non-interactive, deny infrastructure mutations, and clear any inherited
autonomous mode; detection additionally restricts pulse-state mutations
to an explicit allowlist of the finding lifecycle tools
(patrol_report_finding / patrol_resolve_finding - a blanket pulse-state
allowance would also permit alert dismissal and knowledge writes),
while investigation denies all pulse-state mutations. Chat turns build
ONE effective request executor (control level, autonomy, profile,
resolved context) BEFORE provider projection and clone that executor
for every provider attempt, so the offered schema and the runtime
boundary always agree; scheduled Patrol (`ExecutePatrolStream`) applies
the detection profile the same way, and `ListAvailableTools` projects
through the identical path. Non-interactive profiles independently hide
pulse_question from the manifest AND runtime-block it in the agentic
loop before the interactive-call-set special case - a fabricated
question call returns a non-interactive error without emitting a
waiting event and sibling tool calls from the same provider turn keep
processing; approval waits never block (they queue), and the
tool-only-turn wrap-up guardrail is interactive-profile-owned rather
than keyed on autonomy. The system prompt describes detection and
investigation modes directly instead of claiming controlled or
autonomous execution. Non-interactive operation grants no mutation
authority. The profile vocabulary is closed: unknown profile values are
rejected at apply time and classify as non-interactive, never as the
permissive interactive default.
The typed proposal channel rides the investigation profile.
`patrol_action_capabilities` and `patrol_propose_action` are side-effect-free,
mutation-none, read-kind tools projected and executable ONLY under the
investigation profile: the registry policy rejects a fabricated call under any
other posture before the handler runs. The capability lookup accepts one
canonical resource ID and reads the same tenant-bound catalogue used by
proposal validation without consuming proposal cardinality. This allows the
model to inspect the typed actions of a causal resource discovered during the
run instead of forcing the original finding resource to remain the action
target. The proposal tool's model-authored schema carries only
resource_id, capability_name, params, and reason; proposal, finding,
investigation, and evidence identity are injected from trusted
orchestration context through the request-local proposal capture sink
(`tools.ProposalCapture`), which executor clones share so one run has
exactly one capture. Tool calls carry an explicit invocation envelope
(tool-use ID, name, arguments) through `ExecuteInvocation`, and the
sink keys on call identity plus payload fingerprint: an idempotent
replay (same ID, same payload) re-succeeds; the same ID with a
different payload latches a terminal integrity error; a second distinct
valid proposal latches terminal ambiguity - and both terminal states
INVALIDATE the captured proposal, because concurrent per-turn execution
makes "first" nondeterministic. Proposals count only after catalog
validation (advertised capability, declared/required/enum parameters,
sensitive parameters rejected before success with no value echo in any
output) and a successful tool result. `ExecuteInvestigationStream`
returns proposal cardinality as a structured result - consumers never
reconstruct proposals from session messages - with typed errors for
ambiguity, integrity violations, and the failed-attempts-only case
(zero successful proposals with failed attempts is an error outcome,
never the valid zero-proposal conclusion); `ListInvestigationTools`
projects through the identical profile path. Proposal parameter values
exist only transiently for provider continuation and validation: the
canonical exposure projector
(`RedactToolCallArgumentsForExposure`) redacts them from the durable
chat transcript and every tool_start/tool_progress/tool_end stream
event, while the action audit remains their canonical durable home.
Proposal validation reuses the planner's canonical exported rules
(`actionplanner.FindCapability` exact-name matching and
`actionplanner.ValidateParams` for declared/required/typed/enum/pattern
parameters and malformed schemas), so proposal acceptance and planning
can never drift; the sensitive-parameter rejection stays a
proposal-specific ratchet on top. Captured proposals are immutable:
params and evidence identity are deep-cloned on capture and again on
outcome, and fingerprint serialization failures are errors, never a
shared sentinel value. Provider-streamed RawInput overrides on progress
events are discarded for exposure-restricted tools - the override is
unredacted model output and would reintroduce exactly what the
projector removed. An investigation run refuses to start without
finding and investigation identity, and any run error nils the
proposal while preserving simultaneous proposal errors via errors.Join:
a non-nil proposal exists only from a completely successful run.
Proofs: `internal/ai/tools/proposal_capture_test.go` (concurrent
ambiguity with nil proposal, replay/integrity semantics by tool-use ID,
failed-attempts typed error, sensitive rejection without echo,
profile-only projection and execution, caller-mutation immunity,
exact-name matching, canonical planner validation) and the end-to-end
loop redaction and progress-override proofs in
`internal/ai/chat/service_tooling_test.go`. A blocked question call is persisted to
the durable transcript paired with its refusal result (the assistant
message records the call, so the transcript must never retain an
unanswered call). Profile proofs live in
`internal/ai/tools/invocation_policy_test.go` (detection allowlist,
investigation structural read-only, profile clone isolation) and
`internal/ai/chat/service_tooling_test.go` (profile prompt modes,
question-tool hiding, effective-executor projection).
Handler-level checks remain defense in depth, and pulse_read's
structural execution-intent classifier
(`ClassifyExecutionIntent` rejecting write-or-unknown command text
before dispatch) is mandatory second-stage enforcement for exec, not
merely defense in depth: the static read/none descriptor alone cannot
prove arbitrary command text safe. The
deny restriction is deliberately separate from autonomous mode:
suppressing interactive questions grants no mutation authority.
`pulse_file_edit` is write-only (append/write); file inspection routes
through `pulse_read` action="file". The retired hard-coded classifier
misread Kubernetes's discriminator and classified `type:"scale"` as
read, and mixed tools such as Docker bypassed tool-level control
gating entirely (an `action:"update"` reached direct execution at
read-only). Proofs: `internal/agentcapabilities/invocation_test.go`
and the invocation-policy regression suite in
`internal/ai/tools/invocation_policy_test.go`.
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
to Pulse Intelligence > Provider & Models settings and tool-capable Patrol model
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
The backend may keep precise full-run, scoped-run, and verification terms in
wire fields and internal decisions, but browser-facing summary and handoff copy
emitted from `internal/ai` should translate that precision into check language:
recent broad checks, targeted checks, follow-up checks, current issues, and
verified outcomes. Runtime explanations must not surface activation-loop,
proof-strip, or scoped/full-run jargon as the ordinary operator vocabulary.
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
acceleration is currently active. Runtime policy blocks, including the local
development background-automation guard, must be represented as an explicit
event-trigger block on the Patrol trigger status rather than by rewriting the
operator's alert/anomaly trigger preferences to disabled.
That same runtime boundary also owns which Patrol work counts toward
full-patrol cadence gates. Community-tier or other full-run limits must key
off completed full sweeps only; recent scoped or verification activity may
advance `last_activity_at`, but it must not block a manual full Patrol request
as if a scheduled estate-wide sweep already happened.
The manual Patrol route carries that same scoped/full distinction. `POST
/api/ai/patrol/run` (`internal/api/ai_handlers.go` `HandleForcePatrol`) accepts
an optional scope body — `resource_ids` and/or `resource_types`, optionally with
`alert_identifier`, `alert_type`, and `context`. With a scope it runs a manual
Targeted check through the same scoped engine (`TriggerScopedPatrol`) and run
record (`Type: "scoped"`) as automatic alert-triggered work, not a new
investigation path; without a body it keeps the legacy fleet-wide Patrol check
behaviour. A manual scoped run must reuse the existing scoped engine rather
than adding a parallel trigger route, must honour the same Patrol readiness gate
as a full run, and must bypass the full-run cadence gate because targeted
checks never consume an operator's manual full-run allowance. The request must
carry resource identity only — no command, prompt, or remediation payload — and
the route must keep requiring admin plus `ai:execute` scope for both shapes.
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
That success boundary includes provider-backed scoped Patrol runs and successful
Patrol tool-call preflights. A successful scoped run proves that Patrol can
currently reach the selected provider/model and complete tool-backed analysis;
a successful preflight with an observed tool call proves the configured
provider/model currently accepts Patrol's tool-call path. Either must clear the
synthetic `ai-service` runtime failure just as a successful full Patrol run
does, without loosening ordinary scoped finding reconciliation for
infrastructure issues. A soft-warning preflight where the provider responds but
the model does not emit a tool call is not sufficient recovery evidence.
Because those findings represent Patrol blindness rather than operator-triaged
infrastructure noise, the Patrol runtime must also reject manual acknowledge,
snooze, dismiss, resolve, and suppress actions against synthetic `ai-service`
runtime findings. The canonical recovery path is to correct Patrol provider
configuration in Pulse Intelligence > Provider & Models settings and let Patrol
re-evaluate the runtime condition on the next run.
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
Patrol detection and investigation streams under that boundary must also carry
an explicit 60-second inter-chunk stall allowance, capped by the operator's
configured provider request timeout. Interactive Assistant streams retain the
short default stall bound. A compatible provider pausing between streamed
reasoning and its next tool-call chunk must not invalidate an otherwise live
Patrol run, while a genuinely stalled stream must still terminate within the
Patrol run budget.
Qualification scoring must keep synthetic Patrol runtime findings on the
`ai-service` resource separate from model-authored infrastructure findings.
Provider/runtime failure remains an unconditional qualification hard failure,
but its synthetic service finding must not be reported as an infrastructure
false positive or matched to scenario ground truth.
Qualification cost gates must resolve the configured provider route rather
than silently borrowing a direct-provider family price. Reviewed OpenRouter
routes use exact model IDs and model-specific review dates from the public
OpenRouter Models API; aliases, fast variants, routers, and other unreviewed
routes remain unknown and therefore fail any non-zero qualification cost
budget until their actual route price is explicitly recorded. Live scoring
uses the provider resolved by Patrol readiness, even when the configured
OpenRouter model is an unprefixed slash route such as
`anthropic/claude-sonnet-5`; scorer replay persists and reuses that resolved
provider so the same captured report prices deterministically.
After a non-interactive Patrol finding lifecycle write succeeds, the structured
tool result is authoritative and the remaining text-only provider turn is only
for operator-facing summary prose. That turn must use a bounded summary system
instruction rather than resending the full detection and investigation prompt;
it may not invent findings, evidence, actions, verification, or remediation.
Docker dependency qualification fixtures must use only executables present in
the catalogue's pinned image and must reach a healthy dependency/client
baseline before fault injection. The Watch correlation and Pro investigation
fixtures use Alpine's `nc` applet for the disposable HTTP dependency; a missing
optional daemon must fail baseline convergence and never be scored as a model
miss.
Live qualification collection convergence must require both exact scenario
resource names and the scenario-owned Docker oracle state projected by Pulse's
canonical resources API. A direct lab oracle may prove the fault exists, but
Patrol must not run until the normal agent/API path independently exposes the
relevant health, running-state, or restart-count condition; a stale healthy
projection is a collection failure, never a model miss. These expectations are
derived from manifest oracles before execution and never from Patrol tool
selection, preserving non-circular scoring.
Healthy negative controls use the same convergence rule in the opposite
direction: their manifest-owned baseline predicates must be visible through
the canonical resource projection before Patrol starts. A transient `starting`
projection cannot force the model to spend a confirmation turn after the lab
oracle has already established `healthy`.
Watch qualification must score the confirmed provider-visible symptom rather
than require an unproved causal attribution. In the correlated Docker scenario,
the stopped dependency remains the independently injected fault and the
downstream unhealthy client is the required Watch finding; one finding proves
symptom deduplication without teaching Watch that every stopped container is
unexpected. Pro investigation separately owns the causal diagnosis and its
evidence requirements. Ground-truth v2 therefore records the injected target
and the independently declared expected-finding resource as separate resolved
identities; an expected resource may differ only when the manifest declared it
as related before execution. V1 replay artifacts retain target-as-expected
semantics.
Pro cross-resource qualification must keep the initial trigger scope separate
from the lab's collected/oracle resource set. A reviewed `scope_resources`
alias list narrows only the Patrol trigger; it never removes related resources
from collection or ground truth. Scenario-owned `root_cause_resources` and
`affected_resources` aliases are resolved before execution and scored against
the exact `Root Cause` and `Affected Resources` response sections using the
collected canonical name or ID. The expected diagnosis therefore remains
independent of the read-only tools Patrol chose, and merely repeating the
downstream symptom cannot pass causal grounding.
An existing-finding reconfirmation scenario may seed its scored run only from
a successful prerequisite Patrol run that created a run-owned finding on every
scenario-owned expected resource. An incidental provider, readiness, or other
unrelated finding cannot satisfy setup, and the failed prerequisite remains in
the report as diagnostic evidence rather than being converted into a model
reconfirmation miss.
Scenario-owned `required_summary_term_groups` may declare reviewed semantic
alternatives for one independently known fact, such as Docker's injected
`stopped` action and its normally collected `exited` state. Every group still
requires one match, but scoring must not fail an otherwise exact causal
diagnosis merely because the model used the provider-visible equivalent rather
than the injector's verb. These alternatives are fixed in the manifest before
execution and never inferred from the model response or its tool path.
The canonical `pulse_query` schema and executor must admit the same resource
types. In particular, `action=get` accepts `docker-host`, returns a governed
read-only host response when the identity resolves, and returns a successful
typed `not_found` result when it does not; a model following the advertised
schema must not incur a failed tool call because the executor used a narrower
legacy type switch.
Scoped Patrol prompt context must describe canonical/source IDs and aliases as
identity aliases, not count each value as a distinct infrastructure resource.
The exact scoped inventory is the authoritative resource set. Patrol must call
`patrol_get_findings` once near the beginning of a run and reuse that snapshot
for all lifecycle decisions, avoiding duplicate reads and their extra provider
turns on both healthy and faulted runs.
On a quiet deterministic triage with a current exact scoped inventory showing
the scoped resources running and healthy, no container restart evidence, and no
active alerts or findings, the model uses the supplied snapshot as sufficient
calm-day evidence and does not spend platform or inventory tool calls merely
reconfirming the same state. Exact scoped app-container rows carry the canonical
restart count from normal collection. A non-zero count is a concrete signal
that prevents the calm-day shortcut and may justify one targeted current
resource read. A provider-observed count of at least three deterministically
flags repeated exits as model context. The model may report that grounded
reliability warning even when the sampled lifecycle state is `running`, but it
must not call the container an active restart loop from that snapshot alone. A
current `restarting` state or a count that increased from the scoped snapshot
confirms the active-loop symptom. The model does not spend logs, discovery,
Docker-service, or other root-cause tool calls after the repeated-restart
symptom is established; causal analysis belongs to the separate Pro
investigation track. Quiet triage is not a deterministic replacement for
model-owned assessment.
Model-authored structured findings remain concise for continuous operation:
description uses at most three short sentences, impact one sentence, evidence
three concrete facts, and recommendation two short sentences, without
repeating the same state or caveat across fields. Concision must preserve the
exact evidence needed to verify the finding rather than replacing it with a
generic summary. The provider-facing `patrol_report_finding` schema and its
runtime handler both require non-empty evidence and a safe, actionable
recommendation for every model-authored finding. A bounded investigation or
verification step is a valid recommendation when remediation is not yet
justified; impact remains optional so the contract never pressures the model
to fabricate a consequence. Providers that omit either grounding field must
receive a tool error and no partial finding may be persisted.
The provider schema is also the source of truth for the required-argument
checklist rendered into Patrol's normal and bounded final-decision prompts.
Every report call must be independently complete, including parallel calls for
several findings; model-facing guidance must not maintain a second hand-written
field list that can drift from the registered tool schema. Runtime recovery
after a rejected call remains allowed so Patrol can safely finish the user
outcome, but formal qualification continues to reject every unsuccessful tool
call, including a recovered schema-validation lapse. Recovery evidence is a
useful product diagnostic, not permission to retroactively relabel a published
run or weaken protocol reliability as a launch gate.
Infrastructure values remain untrusted across both the full Patrol prompt and
the bounded post-finding summary turn. Model-authored analysis, findings, and
operator summaries must not quote, reproduce, or closely paraphrase embedded
instructions, prompt-injection payloads, canary markers, or secrets from
names, labels, annotations, logs, command output, discovered metadata, or tool
results. When relevant, the model may state only that untrusted metadata was
ignored, without repeating its content.
Provider-facing `pulse_docker` calls are host-scoped: every advertised action
requires a Docker host name or ID in the structured schema, matching the
executor contract and preventing invalid hostless Swarm-state calls.
Persisted Patrol tool inputs must retain complete structured finding calls up
to the bounded 16 KiB record limit so normal evidence-rich findings remain
deterministically replayable. If any captured input is nevertheless incomplete
or malformed, report generation must preserve the report, replay diagnostic,
Markdown, and checksums, mark the run failed, and refuse to present that capture
as executable deterministic replay.
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
Every successful autonomy mutation must publish the persisted investigation
budget and timeout to the already-wired tenant investigation orchestrator
before a newly triggered finding can be admitted. Requiring a server restart,
or leaving the orchestrator on its construction-time defaults while the API
reports the new values, is a contract violation: it silently turns the
operator's model-led investigation allowance into a smaller framework-owned
ceiling. A running investigation snapshots its limits when execution starts,
so later settings changes affect subsequent runs without racing or rewriting
an in-flight run.
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
When a chat turn carries a product-originated finding, resource, handoff,
action, or structured-mention context, the event may also include the coarse
`context_scope` classifier so privacy-safe telemetry can count governed-context
Assistant collaboration without exporting prompts, session IDs, resource IDs,
finding IDs, command text, or action payloads.
When the same turn includes accepted model-selected governed tool calls, the
agentic loop records only the total count on the resulting cost event. That
count supports Pulse Intelligence adoption telemetry for Assistant
collaboration while keeping tool names, arguments, results, provider call IDs,
transcript content, resource IDs, finding IDs, command text, action payloads,
and session IDs out of persisted/exported telemetry.
The store is threaded through `chat.Config.CostStore`, wired by
the router from the per-tenant `AISettingsHandler.GetAIService`
via `Service.CostStore()`. `ExecutePatrolStream` deliberately
does NOT record here — its caller (`patrol_ai.go`) records via
its own helper, so cost is never double-counted on the
patrol-via-chat path.

Patrol action continuity is audit-authoritative after proposal submission.
Investigation execution exposes run failure and proposal failure as independent
typed channels; a proposal-only failure may become a needs-attention outcome,
but a simultaneous provider/runtime failure must remain a failed investigation.
Before the model runs, the enterprise orchestrator receives the exact broker
capability catalog, including approval floor, parameter types, enums, patterns,
and sensitivity, and must explicitly forbid proposal guessing when catalog
lookup fails or returns no capabilities. Once `ActionBroker.Submit` succeeds,
action-transition callbacks are wakeups only: `internal/api/patrol_action_reconciliation.go`
re-reads the authoritative action audit, projects its current `ActionReference`
onto both the investigation and finding, and maps terminal verification onto
`fix_verified`, `fix_verification_failed`, or `fix_verification_unknown`.
Investigation reads perform the same hydration by action id or trusted origin,
so a missed callback cannot strand Patrol on stale approval state. Finding
lifecycle publication is idempotent and emits unified lifecycle updates plus
honest terminal push outcomes without turning unverified execution into an
all-clear.
Policy-authorized submission is still audit-authoritative. Missing policy
state, provider errors, closed/out-of-window resource policy, unsupported
eligibility, or a Patrol mode below the capability's class leave the action
pending rather than converting a valid proposal into an investigation failure.
An idempotent resubmission returns the existing action disposition and must not
execute a terminal action twice.

### Canonical mutation registry boundary

Assistant infrastructure mutations are classified by the generated
`internal/mutationregistry` registry before handler execution. The only active
model-originated mutation is `pulse_control type=resource`, which submits a
typed capability request to the shared action-lifecycle planner and never
contacts infrastructure directly. Retired tool names and compatibility aliases
cannot be registered or shadowed by extensions. Docker update/control,
Kubernetes scale/restart/delete, arbitrary exec, raw command, and file writes
are omitted from offered schemas and denied if fabricated.

The registry audits enumerate actual registered tool discriminator values and
bind mechanically discovered API, job, and transport candidates to one registry
disposition. Transport lifecycle entries must name non-transport lifecycle
authority and committed authority before delivery. Task 07 owns durable
delivery/reconnect and Task 10 owns terminal truth/compensation; incomplete
Kubernetes/native-provider, delivery, and rollback paths remain
`retired_denied`, not parallel executors. Docker container image updates left
the incomplete set: `resource.docker.container-update` and
`transport.agent.docker-container-update` are lifecycle-dispositioned over the
typed `docker_container_update` operation with durable receipts and declared
backup/rollback compensation, while the model-originated
`assistant.docker.update` route and the legacy queued update transports stay
`retired_denied`.

Enterprise command-remediation records are readable historical imports only.
Production code contains no command or rollback execution algorithm; exported
approve/execute/rollback interfaces and HTTP endpoints are permanently inert
even when a command executor is injected.

The agent transport catalogue now classifies wire roles independently from
mutation identity. `docker_container_lifecycle` is the typed mutation request
for the existing `resource.docker.container-lifecycle` capability and may be
sent only after committed action-lifecycle authority.
`agent_operation_query` is query-only reconciliation;
`agent_operation_query_result` and `docker_container_lifecycle_result` are
non-admitting receipt/result messages. Query, result, receipt, and general
protocol roles are forbidden from carrying a mutation registry id or durable
authority reference, so response-shaped lookalikes cannot become dispatch
entry points. Task 10 remains the sole owner of `ActionResultV2` execution,
verification, evidence, and compensation truth.

Action planning and approval attribution are now server-owned across Assistant,
Patrol, and MCP projections. Trusted brokers use explicit service/policy actor
contexts; public `requestedBy` content cannot become audit authority. The agent
manifest advertises the granular `actions:plan`, `actions:approve`, and
`actions:execute` scopes while the server alone owns the bounded legacy-scope
compatibility window. AI-facing method labels, model assertions, and local
biometric claims never satisfy an MFA floor. Until the core step-up verifier
accepts action-bound cryptographic evidence, the honest runtime outcome is
step-up unavailable, not MFA approved.

The legacy Patrol approval bridge must project stored approvals into that same
server-owned model before a lifecycle decision is appended. It binds a scoped
service requester, the deciding human `ActorBinding`, and method/session
`ApprovalEvidence` to the exact action, plan hash, outcome, organization, and
decision time. A zero-version legacy approval requirement is upgraded to the
current canonical floor before evaluation; missing or inconsistent authority
fails closed. A dry-run plan is never executable and must persist refusal
without minting decision approval authority.

Patrol action proposals persist the exact bounded policy authorities consulted
at planning time through the canonical unified-resource `policyDecision`
object. Capability, tenant Patrol, and resource operator factors retain typed
source revisions and reason codes; unavailable and absent sources remain
explicit rather than becoming human-authored labels. Planning and dispatch
call one shared pure Patrol evaluator over separately fetched current inputs.
The plan snapshot is descriptive context for Task 11 and is never passed back
as automatic authority; Task 04 dispatch admission still re-fetches policy and
issues the only executable lease.

AI and Patrol projections consume `unifiedresources.ActionResultV2`; they do
not own action-result enums. Finding reconciliation maps confirmed,
contradicted, and inconclusive verification independently from execution, and
therefore cannot treat terminal audit state as proof of the postcondition.
Legacy investigation outcome strings remain projections for compatibility and
must be derived from canonical truth rather than reconstructed locally. The
typed disposition retains the serialized canonical two-axis result so the
legacy single outcome is never the sole durable/read-model truth.

### Server-owned Patrol Autopilot activation

Patrol full mode is now an effective server decision, not the stored
`patrol_full_mode_unlocked` compatibility boolean. The runtime evaluates a
versioned acknowledgement and activation bound to one human actor credential
and organization; missing, stale, expired, revoked, malformed, cross-tenant,
or API-token evidence falls back to approval mode. Acknowledgement creation is
separate from activation, lower modes clear activation without rewriting
history, and version rotation or revocation takes effect before new Patrol
admission through the existing Task 04 policy-mutation coordinator. The API
handler and every tenant AI service share the same injected server clock and
current-version provider, so a newly supported contract cannot activate at the
API boundary while remaining stale in the runtime that enforces Patrol mode.

The accepted limits promise only policy allowlisting, emergency-stop and
approval-floor enforcement, reconciliation when supported, disclosed evidence
class, and honest inconclusive outcomes. They do not promise independent
verification, and execution success remains separate from outcome truth.
Task 11 still owns the explicit acknowledgement UX and browser/device proof;
Task 12 still owns final certification.

### Task 09 deterministic APT workflow routing (non-lab floor)

`internal/ai/findings_apt_workflows.go` deterministically produces agent-managed
APT update and pressure-gated package-cache findings. Admission requires fresh
dual-timestamp telemetry and the exact current canonical capability/handler.
Stale, skewed, replayed, errored, or capability-less state is unknown and must
not resolve an active finding; only fresh authoritative clear evidence or
explicit resource removal reconciles it. The same pure detector entry point is
exercised through an exact empty-parameter proposal, shared planning/policy and
human approval, durable typed dispatch, canonical terminal audit, and finding
resolution for both workflows. Finding evidence stays bounded and contains no
command, path, package selector, raw APT output, stderr, or reboot authority.
The APT verifier claims only canonical APT finding keys. It must return
unhandled for CPU, memory, disk, and every other non-APT key so the owning
deterministic verifier can evaluate that finding; APT resource lookup failure
must never intercept unrelated verification.

Fake-only callback-loss tests reopen the server-side action store and consume
Task 07 terminal receipts without resending either typed mutation, then
reconcile the audit and originating finding. This closes the non-lab detector-
to-reconciliation code/test floor only. Claims 16/17 and both APT scorecards
remain below operational completion because browser workflow evidence,
disposable Debian/Ubuntu real-lab evidence, and Task 12 final-SHA certification
remain explicit residuals; no tier-5 or tier-6 evidence is claimed here.

`fix_verified` is reserved for a canonical action result whose successful
execution has a confirmed independent postcondition. Agent-attested
confirmation remains useful evidence but projects to
`fix_verification_unknown`; investigation and finding records remain unresolved
and terminal push copy stays explicitly inconclusive. This keeps legacy
single-outcome consumers conservative while `ActionDisposition.ActionResultV2`
retains both truth axes and the evidence source.

### Proxmox guest lifecycle Patrol detector floor

`internal/ai/findings_proxmox_lifecycle.go` owns the deterministic production
detector for Proxmox VM and LXC stopped transitions. The first observation is a
baseline only: Patrol may emit a finding only when a fresh Proxmox source
observation moves the same canonical guest from running to stopped at a newer
source timestamp. Admission also requires the exact current resource-owned
`start` capability and matching `proxmox.vm.lifecycle` or
`proxmox.ct.lifecycle` handler. Templates, locked guests, stale or missing
source authority, repeated timestamps, unknown states, and capability loss
fail closed. Existing stopped guests therefore do not become findings merely
because Patrol or Pulse restarted.

Production admission also reconciles the transition against the canonical,
organization-scoped action-audit store before emitting. A completed `shutdown`
or `stop` may suppress the finding only for the same canonical guest when its
canonical `ActionResultV2` execution succeeded and a recorded approval falls
between the prior fresh running observation and the newer stopped observation.
Legacy-only results, failed or nonterminal actions, other capabilities or
resources, out-of-window decisions, cross-organization stores, and audit-read
errors do not suppress the warning. Suppression advances the observation
baseline so the intentional stop cannot surface one cycle later.

The watcher runs before the model-provider gate whenever guest analysis is
enabled, so detection is not conditional on an available LLM. Fresh running
evidence reconciles an active finding; stale or incomplete state does not, and
explicit resource removal uses its own typed resolution reason. Finding
evidence is bounded to guest kind, stopped state, Proxmox instance/node/VMID,
and observation time and carries no command, credential, token, or provider
mutation authority. Operator maintenance and intentionally-offline intent
continue through the shared finding-store suppression boundary.

The non-lab integration proof takes the emitted finding through an exact empty-
parameter `start` proposal, shared planning, separate human approval, durable
node-agent dispatch, canonical action audit, and server-side Proxmox API
observation. Only confirmed independent evidence projects the originating
finding to `fix_verified`; command success or same-agent status alone cannot
produce the green terminal state.
