# API Contracts

## Contract Metadata

```json
{
  "subsystem_id": "api-contracts",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/api-contracts.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "agent-lifecycle",
    "ai-runtime",
    "cloud-paid",
    "patrol-intelligence"
  ]
}
```

## Purpose

Own canonical runtime payload shapes between backend and frontend, including
the trust boundary that keeps customer-safe support diagnostics and normal
product API routes free of maintainer commercial analytics.

## Canonical Files

1. `internal/api/contract_test.go`
   1a. `internal/api/platform_connection_shared.go`
   1b. `internal/api/metadata_handlers_shared.go`
2. `internal/api/resources.go`
3. `internal/api/discovery_handlers.go`
4. `internal/api/alerts.go`
5. `internal/api/activity_audit_handlers.go`
6. `internal/api/actions.go`
   5a. `internal/api/action_executor.go`
   5b. `internal/api/docker_container_action_executor.go`
   5c. `internal/api/proxmox_guest_action_executor.go`
7. `internal/actionplanner/planner.go`
8. `pkg/pulsecli/api_client.go`
9. `pkg/pulsecli/actions.go`
10. `pkg/pulsecli/fleet.go`
11. `pkg/pulsecli/root.go`
12. `frontend-modern/src/types/api.ts`
13. `frontend-modern/src/types/actionAudit.ts`
14. `frontend-modern/src/api/actionAudit.ts`
    7a. `frontend-modern/src/api/resourceActions.ts`
    7b. `frontend-modern/src/api/agentCapabilities.ts`
    7c. `frontend-modern/src/api/generated/agentCapabilities.ts`
15. `frontend-modern/src/api/responseUtils.ts`
16. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx`
17. `frontend-modern/src/components/Settings/APITokenManager.tsx`
18. `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`
19. `frontend-modern/src/constants/apiScopes.ts`
20. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`
21. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`
22. `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`
23. `frontend-modern/src/components/Settings/NodeModalAuthenticationSection.tsx`
24. `frontend-modern/src/components/Settings/NodeModalBasicInfoSection.tsx`
25. `frontend-modern/src/components/Settings/nodeModalModel.ts`
26. `frontend-modern/src/components/Settings/NodeModalMonitoringSection.tsx`
27. `frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx`
28. `frontend-modern/src/components/Settings/NodeModalStatusFooter.tsx`
29. `frontend-modern/src/components/Settings/useNodeModalState.ts`
30. `frontend-modern/src/utils/agentInstallCommand.ts`
31. `frontend-modern/src/api/nodes.ts`
32. `frontend-modern/src/api/license.ts`
    23a. `frontend-modern/src/api/settings.ts`
33. `frontend-modern/src/api/monitoredSystemLedger.ts`
34. `frontend-modern/src/api/resources.ts`
35. `frontend-modern/src/api/monitoring.ts`
36. `internal/api/monitored_system_ledger.go`
37. `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`
38. `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts`
39. `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts`
40. `frontend-modern/src/utils/apiTokenPresentation.ts`
41. `frontend-modern/src/utils/infrastructureSettingsPresentation.ts`
42. `internal/api/router_routes_auth_security.go`
43. `internal/api/relay_hosted_runtime.go`
44. `internal/api/ai_hosted_runtime.go`
45. `internal/api/router_routes_licensing.go`
46. `internal/api/reporting_inventory_handlers.go`
47. `internal/cloudcp/portal/bootstrap.go`
48. `internal/cloudcp/portal/handlers.go`
49. `internal/cloudcp/portal/page.go`
50. `internal/cloudcp/portal/page_templates.go`
51. `internal/cloudcp/portal/frontend/src/index.ts`
52. `internal/cloudcp/portal/frontend/src/shell.ts`
53. `internal/cloudcp/portal/frontend/src/billing.ts`
54. `internal/cloudcp/portal/frontend/src/runtime.ts`
55. `internal/cloudcp/portal/frontend/src/types.ts`
56. `internal/cloudcp/portal/frontend/src/styles.css`
57. `internal/cloudcp/portal/frontend/tsconfig.json`
58. `internal/cloudcp/portal/frontend_sync_test.go`
59. `internal/api/recovery_handlers.go`
    51a. `internal/api/pbs_backups.go`
60. `internal/api/config_setup_handlers.go`
    52a. `internal/api/setup_script_render.go`
    52b. `internal/api/cloud_agent_install_command.go`
61. `internal/api/demo_mode_commercial.go`
62. `internal/api/demo_mode_operations.go`
63. `internal/api/security_status_capabilities.go`
64. `internal/api/demo_middleware.go`
65. `frontend-modern/src/stores/aiRuntimeState.ts`
66. `internal/api/connections_types.go`
67. `internal/api/connections_aggregator.go`
68. `internal/api/connections_handlers.go`
69. `internal/api/connections_probe.go`
70. `frontend-modern/src/api/connections.ts`
71. `frontend-modern/src/utils/connectionErrorPresentation.ts`
72. `internal/api/availability_handlers.go`
73. `frontend-modern/src/api/availabilityTargets.ts`
74. `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx`
    65a. `frontend-modern/src/components/Settings/AvailabilitySettingsPanel.tsx`
    65b. `frontend-modern/src/components/Settings/availabilitySettingsModel.ts`
75. `pkg/aicontracts/investigation.go`
    66a. `pkg/aicontracts/orchestrator_deps.go`
    66b. `pkg/aicontracts/fix_execution.go`
76. `internal/api/ai_intelligence_handlers.go`
77. `frontend-modern/src/api/ai.ts`
78. `frontend-modern/src/api/aiChat.ts`
    69a. `frontend-modern/src/api/aiChatDevStreamFixture.ts`
79. `frontend-modern/src/api/patrol.ts`
80. `frontend-modern/src/api/generated/aiChatEvents.ts`
81. `internal/api/agent_exec_token_binding.go`
    72a. `cmd/pulse-mcp/main.go`
    72b. `cmd/pulse-mcp/README.md`
    72c. `cmd/agent-probe/main.go`
    72d. `scripts/generate-pulse-intelligence-docs.go`
82. `internal/agentcontext/`
    73a. `internal/agentcapabilities/`
    73b. `pkg/extensions/ai_autofix.go`
83. `scripts/generate-types.go`

## Shared Boundaries

Assistant chat `workflow_state` events are a shared AI/API payload boundary.
The backend source of truth is `internal/ai/chat.WorkflowStateData`, generated
to `frontend-modern/src/api/generated/aiChatEvents.ts` by
`scripts/generate-types.go` and pinned by `internal/api/contract_test.go`.
Agent capabilities manifest payloads follow the same rule:
`internal/agentcapabilities.Manifest`, `Capability`, `CapabilityCategory`,
`SurfaceContract`, `SurfaceContractComponent`, `OperatorSurfaceContract`,
`SurfaceAffordanceContract`, `MCPAdapterContract`, and
`MCPAdapterConfigFamily` generate
`frontend-modern/src/api/generated/agentCapabilities.ts`; the shared frontend
client must alias those generated types instead of redeclaring a browser-local
manifest model. The manifest-owned `surfaceContract` is the
canonical API statement that Pulse Intelligence Core is shared, Pulse Patrol is
the primary built-in operator, and Pulse Assistant plus Pulse MCP are the
supported access paths over the same governed capabilities, including the
affordances each path exposes. The Patrol surface description is also
manifest-owned product copy; API clients, docs, and settings projections must
render the concise policy-bound operator summary from the manifest instead of
carrying local control-level prose.
The manifest-owned `mcpAdapter` is the canonical API statement for Pulse MCP
client setup facts: server name, command, base URL flag/default, token
environment variable, and supported client config families. UI and docs may
compose copy around those facts, but they must not carry component-local or
README-local setup constants.
Manifest-backed MCP initialize responses must render that relationship from
`Manifest.SurfaceContract` rather than hard-coded adapter prose. Native
Assistant operating instructions must use the same shared renderer, with
surface-specific affordance flags controlling only whether tools, resources,
prompts, capability metadata, and Assistant-native interactive questions are
advertised. MCP initialize responses and JSON-RPC handlers must resolve those
surface affordances before advertising or serving tools, resources, prompts, or
capability metadata; raw manifest capability and prompt presence can satisfy an
allowed affordance, but cannot enable a disabled surface affordance. Prompt
advertising and serving must use `MCPManifestSurfacePromptProjectionSupported`
and `GetMCPPromptFromManifestSurface` so the target surface's `prompts`
affordance and the manifest-owned workflow prompt catalogue are resolved
together before initialize, `prompts/list`, or `prompts/get` exposes prompts.
The Pulse Intelligence manifest docs generator must also render the MCP README
surface-contract block from `Manifest.SurfaceContract` and the MCP client
config block from `Manifest.MCPAdapter`; scope, tool, prompt, error-code, and
Assistant/MCP/Core/Patrol relationship docs are all API-manifest projections,
not parallel onboarding snapshots.
Native Assistant workflow-prompt rendering is a canonical API projection of
that same manifest. `POST /api/ai/workflow-prompts/render` is an authenticated
`ai:chat` route owned by `internal/api/ai_handler.go` and
`internal/api/router_routes_ai_relay.go`; it accepts the manifest prompt name
plus argument map, validates them against the manifest-owned workflow prompt
catalogue through `internal/agentcapabilities`, and returns only the rendered
Assistant prompt text plus description. Browser
clients must read prompt names, display labels, presentation kind hints,
descriptions, and arguments through the generated `workflowPrompts` manifest
type and call `AIChatAPI.renderWorkflowPrompt` rather than rendering prompt
templates locally. Browser context may carry a preferred workflow prompt name
only as a request to render one manifest-owned starter through that API route;
it is not a transport for prompt text or a reason to bypass argument resolution.
MCP `prompts/list` projects those same labels as protocol prompt titles and MCP
`prompts/get` remains the protocol-edge projection over the same shared
workflow-prompt core. Route inventory and AI chat scope tests must pin that
route so it does not drift into public manifest discovery, relay-mobile access,
or `ai:execute` approval replay.
Successful workflow prompt rendering is also the API boundary for content-free
guided-starter activity: native Assistant rendering records only the manifest
prompt name, coarse Assistant surface, and timestamp; first-party Pulse app
surfaces that start the same governed journey without rendering prompt text may
record through authenticated `POST /api/ai/workflow-prompts/activity`; and MCP
adapters that render prompts locally may report the same event through
`POST /api/agent/workflow-prompt-activity`. The first-party marker route is an
authenticated `ai:chat` route owned by `internal/api/ai_handler.go` and
`internal/api/router_routes_ai_relay.go`; it accepts one JSON object with a
manifest-declared prompt name plus an optional first-party surface, allows only
`pulse_assistant`, `pulse_patrol`, `patrol_control`, `patrol_autonomy`, and
the legacy `pulse_pro_activation` alias, defaults omitted surface to `pulse_assistant`,
rejects unknown prompts or non-first-party surfaces, and returns no payload.
`pulse_patrol` is the ordinary first-party Patrol entry marker, while
`patrol_control` is the current paid Patrol control handoff marker for the
same manifest-owned Patrol work prompt. `patrol_autonomy` and
`pulse_pro_activation` remain legacy Pro activation entry-point aliases for
compatibility only; none of these surfaces is a separate prompt, route,
Assistant surface, or completion proof.
The browser-only paid Patrol control handoff may carry the coarse
`patrolControlStarter=patrol_control` route query so Patrol can record that
same marker after navigation. The old `operationsLoopStarter=patrol_control`,
`operationsLoopStarter=patrol_autonomy`, and
`operationsLoopStarter=pulse_pro_activation` values must remain accepted as
legacy aliases that normalize to the current Patrol control starter, but those
queries are not API payload fields and may not contain prompt arguments,
resource IDs, finding IDs, account identity, token identity, or terminal proof.
Consuming that route handoff must still enter the authenticated first-party
marker route described here before any status projection treats it as Patrol
control starter evidence. Operations status projections count `pulse_patrol`,
`patrol_control`, legacy `patrol_autonomy`, and legacy `pulse_pro_activation`
as Patrol control starter evidence, while `proActivationOperationsLoopStarterCount`
preserves only the legacy Pro activation entry-point count. The
agent marker route is an authenticated `monitoring:read` API-token route,
accepts only one JSON object with a manifest-declared prompt name, normalizes
the `X-Pulse-Agent-Surface` value `pulse_mcp` to the Pulse MCP surface, and
must reject unknown prompts without recording activity. Neither route may
accept or persist prompt arguments, rendered text, resource IDs, finding IDs,
session IDs, token identity, request bodies, or model output.
Telemetry preview and outbound ping payloads project that same content-free
activity into 30-day Patrol work starter counters: one total starter count
plus source-specific Assistant, Pulse Patrol, primary Patrol control, legacy
Pro activation entry-point, and Pulse MCP counts. Those payload fields are
API-contract fields for adoption/value measurement only; they must not carry
prompt text, prompt arguments, resource/finding identity, session identity,
checkout/account identity, or request details.
`GET /api/agent/patrol-control/status` owns the shared content-safe Patrol
control value state exposed to native Pulse and MCP-facing agents. The legacy
`GET /api/agent/operations-loop/status` URL remains a compatibility alias, not
the primary route:
`not_started`, `in_progress`, `governed_decision_recorded`,
`verified_needs_mcp`, and `verified`. `verified_needs_mcp` remains a tolerated
legacy value for older payloads, but MCP readiness is no longer a Patrol
control value gate. Counts remain the aggregate evidence source, while
`patrolControlValueState` is the canonical branch signal for distinguishing a
safe terminal rejection from a verified operations-value outcome.
`patrolAutonomyValueState` and
`proActivationValueProofState` mirror that value as compatibility aliases for
older commercial and telemetry consumers. Only `verified` means the first-party
Patrol control loop has approved-governed action, verified outcome proof, and
recorded action history; adjacent UI or agent clients must not infer that from
`nextAction=complete` alone.
Current-work precedence is part of that same contract: `activeFindingCount` must
include aggregate active issue-level findings from the canonical Patrol findings
store, including runtime/service findings that are not attached to a unified
registry resource. Pending approvals and active findings are current operator
work and must outrank older completed/resolved loop proof when the route chooses
`nextAction` and step status. Historical Patrol control proof remains recorded
as count-only value evidence, but it must render as history until the current
finding or approval is handled.
The authenticated `GET /api/agent/patrol-control/status` projection exposes the same content-free starter evidence as count-only fields
(`operationsLoopStarterCount`, `assistantOperationsLoopStarterCount`,
`patrolOperationsLoopStarterCount`,
`patrolControlOperationsLoopStarterCount`,
`patrolControlCompletedOperationsLoopCount`,
`patrolControlResolvedOperationsLoopCount`, `patrolControlValueState`,
`patrolAutonomyOperationsLoopStarterCount`,
`patrolAutonomyCompletedOperationsLoopCount`,
`patrolAutonomyResolvedOperationsLoopCount`, `patrolAutonomyValueState`,
`proActivationOperationsLoopStarterCount`,
`proActivationCompletedOperationsLoopCount`,
`proActivationResolvedOperationsLoopCount`, and `mcpOperationsLoopStarterCount`)
inside the status evidence window. These fields are orientation/adoption
evidence for Assistant, Patrol control, older compatibility clients, and MCP
loop entry points. `patrolControlOperationsLoopStarterCount` is the primary
starter count for native Pulse Patrol and Patrol control starts, while
`patrolAutonomyOperationsLoopStarterCount` mirrors it for compatibility and
`proActivationOperationsLoopStarterCount` is retained only for older
external-agent clients; primary clients must use
`patrolControlOperationsLoopStarterCount`. Only the completed-loop and
resolved-loop counts may
summarize that first-party starter evidence and terminal loop evidence
coexisted in the current evidence window. `patrolControlCompletedOperationsLoopCount`
requires Patrol issue evidence, contextual Assistant or external-agent
collaboration, and either a rejected governed decision or an approved governed
decision with verified outcome proof. `patrolControlResolvedOperationsLoopCount`
is stricter: it requires Patrol issue evidence, contextual Assistant or
external-agent collaboration, an approved governed decision, and verified
outcome proof. The completed-loop, resolved-loop, and value-state
`patrolAutonomy*` and `proActivation*` fields are compatibility aliases for the
same Patrol control proof model; human-facing manifest descriptions and
external-agent docs must describe all of these fields as Patrol control
compatibility metadata rather than a Pro activation journey. The
`internal/telemetry.ClassifyPulseIntelligencePatrolControlProof` helper is the
canonical count-only classifier for those completed/resolved counts and the
`patrolControlValueState` string; API status projections and outbound
telemetry/adoption projections must call that helper or its legacy alias
wrapper instead of restating the branch rules locally, and must not pass
`externalAgentReady` into that classifier because MCP readiness is not a Patrol
control proof input. The
`externalAgentReady` field is not manifest-shape readiness by itself: the route
must first prove the Pulse MCP Patrol work contract exists in the canonical
manifest and then require a single non-expired API token covering every scope
required by the published Pulse MCP Patrol work capability set. It is
optional external-agent readiness for settings handoff and MCP clients, not a
completed/resolved Patrol control proof requirement. They must
not expose
prompt names, surfaces, identities, payloads, checkout/account
identity, route parameters, action IDs, command text, resource names, or
finding IDs, and they do not replace the detail-owning Assistant, Patrol,
governed-action, verification, or finding-resolution routes.
The public `docs/AI.md` Pulse Intelligence overview must use the same API
projection: its marked Core/Patrol/Assistant/MCP block is rendered by
`PulseIntelligenceOverviewMarkdown(Manifest.SurfaceContract)`, while
surrounding product detail may stay human-authored.
The External agents settings panel is part of the same API projection: its
surface summary must consume the generated frontend manifest types and
`/api/agent/capabilities.surfaceContract` through
`frontend-modern/src/api/agentCapabilities.ts` instead of repeating
Assistant/MCP/Core/Patrol relationship copy or affordance labels in the
component.
Its MCP client examples and copied config snippets must also consume
`/api/agent/capabilities.mcpAdapter` through that same shared frontend client;
the component must not own local `pulse-mcp`, `--base-url`, token-env, or
client-family constants, and copied config snippets should be available only
inside an on-demand setup disclosure rather than becoming the default API Access
page content.
The public investigation orchestrator dependency contract in
`pkg/aicontracts/orchestrator_deps.go` is also part of this shared boundary:
orchestrator tool-call and tool-result payloads must bridge to the shared
`agentcapabilities.ProviderToolCall` / `ProviderToolResult` shapes through the
`aicontracts` projection helpers, preserving Assistant/MCP input-map,
thought-signature, and provider-result normalization instead of duplicating
local payload copies in API handlers.
The API contract proof for native Assistant provider tools follows the same
shared-boundary rule: chat-facing API tests must pin
`PulseToolExecutor.AssistantProviderTools` as the single native Assistant
provider-tool projection entrypoint, while the executor owns the manifest
surface-affordance gate and the pairing of runtime-available registry tools,
registry governance descriptors, and Assistant-native interaction tools before
provider requests are assembled.
Surface tool inventories are a shared Pulse Intelligence API contract rather
than a UI or adapter inference. `internal/agentcapabilities.SurfaceToolContract`
and `ProjectPulseIntelligenceSurfaceToolContracts` must describe Pulse
Assistant tools as `assistant_registry` sourced provider tools, splitting
registry-backed tools from Assistant-native interaction tools such as
`pulse_question`, while manifest-declared external-adapter tools are
`capability_manifest` sourced request/response capabilities projected from
`Manifest.Capabilities` by `ProjectManifestSurfaceToolContracts` and published
on `Manifest.SurfaceToolContracts`. External surfaces, including Pulse MCP,
must resolve through the surface-generic manifest contract path and must fail
closed when their published `surfaceToolContracts` entry is missing. Native
Assistant provider-tool projection must enter through
`ProjectPulseAssistantProviderTools`, so disabled Assistant affordances also
fail closed instead of letting runtime registry availability re-enable provider
tools. The two surfaces therefore share governance vocabulary and core
execution contracts without pretending their tool-name lists are
interchangeable.
The frontend-facing contract type must be generated from that same Go shape
(`frontend-modern/src/api/generated/agentCapabilities.ts`) and re-exported by
the shared agent-capabilities client. Do not hand-write a parallel frontend
copy. Runtime surface-tool normalization and count/presentation helpers also
belong in `frontend-modern/src/api/agentCapabilities.ts`, so native Assistant
UI, MCP onboarding surfaces, and future agent-compatible surfaces do not infer
tool availability from local tool-name lists. Do not expose live Assistant
registry availability through the public, cacheable `/api/agent/capabilities`
manifest; that runtime inventory belongs behind the authenticated
`GET /api/ai/assistant/surface-tools` Assistant surface endpoint when the UI
needs it. The Pulse Assistant shell may render a runtime tool-posture chip, but
it must call `AIChatAPI.getAssistantSurfaceTools()` and
`getAgentSurfaceToolPosturePresentation` instead of hard-coding registry tool
names or treating MCP capability names as native Assistant tools. The Agent
integrations panel may render MCP tool posture from the static public manifest,
but the request/response capability filtering and `SurfaceToolContract`
construction must be backend-owned in `Manifest.SurfaceToolContracts`; the
generic `getAgentManifestSurfaceToolContract(manifest,
AGENT_SURFACE_ID_PULSE_MCP)` frontend client path may normalize only that
published manifest field. Missing `surfaceToolContracts` entries must not cause
the browser to infer MCP tools from raw capabilities. The panel must not know the
streaming-only `subscribe_events` exception, call a per-surface projection
alias, or maintain a local MCP tool count. The shared frontend client must not
export a Pulse-MCP-specific surface tool helper; MCP onboarding uses the generic
surface resolver with `AGENT_SURFACE_ID_PULSE_MCP` so future external surfaces do
not require browser-side helper duplication.
Frontend surfaces that need to know whether Pulse MCP exposes the full Patrol
work loop must derive that readiness through the shared agent-capabilities
client from the manifest-owned operations-loop workflow prompt, MCP adapter
contract, and Pulse MCP surface tool contract rather than hard-coding readiness
in the consuming component. The Patrol work capability set includes the
content-safe `get_patrol_control_status` capability as the first orientation
read, followed by fleet context, resource context, findings, governed action
planning, action decision, action execution, and finding resolution; readiness
helpers, workflow-prompt gates, and MCP surface projections must consume that
one shared set instead of maintaining local loop-tool lists. Native Patrol
surfaces that need the current loop evidence must call the authenticated
`GET /api/agent/patrol-control/status` route through
`frontend-modern/src/api/agentCapabilities.ts`, not through a
Patrol-local fetcher or a browser-side reconstruction of agent capability
progress. First-party Patrol presentation may tolerate legacy `open_mcp` and
`verified_needs_mcp` payloads as already verified operator-loop history, and it
may show `externalAgentReady` only as optional external-agent context. Patrol
current-work state/model must not fetch the manifest to derive MCP readiness,
accept MCP readiness props, or introduce a page-local MCP readiness contract.
First-party Patrol UI may present that projection as Patrol current work, but
the authenticated route, query compatibility aliases, and generated wire fields
remain the `operations-loop` contract until a deliberate API migration changes
them.
Backend consumers that need an external surface's request/response capability
set must resolve it through `ResolveManifestSurfaceToolContract` and
`ManifestSurfaceToolCapabilities`, not by reading raw `surfaceToolContracts`
entries directly; that shared path normalizes `CapabilityNames`/`ToolNames`,
filters non-request/response capabilities, and enforces the resolved surface
`tools` affordance before docs, adapters, or UI projections see any tools.
Provider retry progress must use typed fields (`attempt`, `max_attempts`,
`retry_after_ms`) on that same event instead of frontend-only string parsing or
provider-specific ad hoc events; the Assistant UI may format those fields, but
must not invent retry progress that the stream contract did not carry. Assistant
`provider_start` status text must remain route-neutral (`Waiting for
assistant.`) while selected route identity travels through the typed `provider`
and `model` fields. `provider_retry` status text must describe a selected-route
retry, not a provider fallback or hidden route switch. Assistant chat stream
payloads must not expose automatic provider fallback metadata:
`workflow_state` may carry the selected `provider` and `model` plus same-route
retry fields, but `provider_fallback`, `failed_provider`, `failed_model`,
`next_provider`, and `next_model` are retired from the generated stream event
contract. Cross-route recovery belongs to explicit failed-turn actions, not
hidden `/api/ai/chat` stream mutation.
The frontend chat stream client may expose a local stream-open lifecycle hook
for UI state, but that hook is not a backend `workflow_state` payload. It must
fire only after `/api/ai/chat` returns an OK event-stream response and before
the first parsed SSE event is delivered to the reducer, allowing the Assistant
drawer to promote local `request_send` status to local `request_wait` without
inventing provider activity.
Assistant local stream fixtures are part of the same frontend API contract:
`frontend-modern/src/api/aiChatDevStreamFixture.ts` may short-circuit only
explicit `/fixture ...` prompts in development or test mode, must emit the same
typed stream event sequence as live chat, and must never open a provider request.
That helper also owns the exported fixture-name and alias catalog consumed by
Assistant command discovery; frontend command surfaces may search and insert
`/fixture` from those exports, but they must not duplicate the fixture registry
or advertise fixture commands in production.
Fixtures used for visible stream proof must pace status/tool/content events
enough for the browser to paint the intermediate state, including keeping the
`/fixture tool-burst` running tool row visible before the matching `tool_end`;
unit tests may explicitly disable that pace, but the live dev fixture must not
collapse to the terminal row.
The `/fixture tool-chain` stream must pace consecutive
`tool_start`/`tool_progress`/`tool_end` transitions across at least two tools,
so browser proof can stop on a completed compact row plus the next live running
row without a provider request.
Fixtures that emit consecutive context/read/query tool events must keep those
events as ordinary typed `tool_start` / `tool_end` activity and must not encode
obsolete grouped-context wording in fixture answer content. The fixture payload
contract proves the stream reducer and transcript renderer against the same
chronological event order a live provider would produce; UI grouping or footer
summaries are not part of the fixture contract.
The provider-retry fixture must exercise selected-route retry by emitting
`provider_retry` for the selected route and completing with that same model; it
must not simulate an automatic switch to a configured gateway or alternate
provider route.
Queue verification fixtures must cover both the active hold turn and the queued
drain turn so UX proof can exercise queued follow-up ordering and tool rows
without consuming external model quota.
Assistant session summarization is a typed compaction API contract, not a
free-form helper response. `POST /api/ai/sessions/{id}/summarize` returns a
browser-safe `ChatSessionCompactionResult` with stable fields for `success`,
`status`, `message`, `session_id`, `summary_message_id`,
`original_message_count`, `compacted_message_count`, `compacted_messages`,
`kept_recent_messages`, and `summary_chars`. Short sessions may return
`success=false` with `status=not_needed`; transport failures, missing sessions,
provider errors, or active-running sessions remain normal API errors. The
payload must not include the raw transcript, raw provider response, raw tool
input/output, hidden reasoning, secrets, or token values. Frontend consumers
under `frontend-modern/src/api/aiChat.ts` must preserve that typed response and
let the Assistant drawer reload canonical session state instead of rebuilding a
compacted transcript from local browser state.

Infrastructure settings copied agent commands are a shared API/payload boundary
even when the final command string is assembled in the browser. Selected-row
upgrade commands may carry fresh token and identity fields when the operator is
rerunning a known install command for a specific connection, but stale-agent
update commands opened from platform notices must not serialize API tokens,
agent IDs, or hostnames into Unix shell payloads. Those commands route through
the `agentUpdates` settings query for scoped row selection and then call the
installer-owned `scripts/install.sh --update` saved-state path on the target
host. Stale-agent UI must consume `/api/version`'s
`agentUpdateTargetVersion`, not the app build `version`, so development/source
builds that intentionally report `dev` to agents do not surface update nags
when no real agent update target exists. Windows remains on the existing
token-gated PowerShell payload until its installer owns the same saved-state
update contract.

Summary-chart response caching is a shared API boundary:
`internal/api/router.go` may serve a short cached JSON payload for repeated
infrastructure-summary and workloads-summary requests with the same
organization, range, metric set, and workload scope, but that cache is
transport-only. It may amortize polling and remount cost, but it must not
change normalized response shape, bypass monitor or read-state availability
checks, merge tenants, or become the source of truth for telemetry freshness.

The `/api/resources` type filter is the REST contract boundary for platform
page native inventory. It must accept canonical Docker / Podman runtime tokens
(`docker-image`, `docker-volume`, `docker-network`, `docker-task`,
`docker-swarm-node`, `docker-secret`, `docker-config`) and canonical
Kubernetes API object tokens
(`k8s-namespace`, `k8s-service`, `k8s-replicaset`, `k8s-statefulset`,
`k8s-daemonset`, `k8s-job`, `k8s-cronjob`, `k8s-ingress`,
`k8s-endpoint-slice`, `k8s-network-policy`, `k8s-persistent-volume`,
`k8s-persistent-volume-claim`, `k8s-storage-class`, `k8s-configmap`,
`k8s-secret`, `k8s-serviceaccount`, `k8s-role`, `k8s-cluster-role`,
`k8s-role-binding`, `k8s-cluster-role-binding`, `k8s-resource-quota`,
`k8s-limit-range`, `k8s-pod-disruption-budget`,
`k8s-horizontal-pod-autoscaler`, `k8s-event`) whenever unified resources can
publish those records. RBAC tokens (`k8s-role`, `k8s-cluster-role`,
`k8s-role-binding`, `k8s-cluster-role-binding`) accept singular and plural
aliases identically to the rest of the K8s type allow-list, and the resource
payload carries summary RBAC fields only (rule count, role kind / role name,
subject count, subject Kinds, ClusterRole aggregation labels) — full
PolicyRule contents and individual subject names are not exposed through any
API surface. Docker node and Swarm node aliases may normalize to the
canonical `docker-swarm-node` token, and Swarm secret/config aliases may
normalize to `docker-secret` and `docker-config`, but unsupported legacy aliases
should continue to fail closed instead of silently widening platform queries.
For Proxmox-authored VM and system-container resources, `/api/resources`
also owns the workload action/discovery target coordinates. The payload may
emit `discoveryTarget` for those workloads only when unified resources has
linked the Proxmox node parent to a stable Pulse agent ID; `agentId` is that
linked agent ID, not the Proxmox node display name. Clients must not infer a
Proxmox workload action target from `node` plus `vmid` when the backend omits
that target.

Discovery read endpoints are a canonical API payload boundary even when Pulse
is running in mock mode. `/api/discovery`, typed discovery detail/progress
routes, type filters, agent filters, and status must expose mock-authored
Discovery records through the same `servicediscovery.ResourceDiscovery` and
summary response shapes used by the live discovery service. Live service data
remains primary when a service is configured; mock fixtures may supplement or
stand in for that data only in mock mode, and they must not expose raw command
output or bypass the normal non-admin redaction path.
Forced discovery trigger requests must preserve workload identity at the API
boundary. `POST /api/discovery/{type}/{target}/{id}` may use the route target
as a hostname fallback only for host-agent discovery; VM, system-container,
Docker, and Kubernetes workload triggers must leave hostname empty unless the
caller supplied one so the discovery service resolves the workload name and
endpoint from canonical resource state instead of mistaking the parent node for
the workload endpoint.

Source-specific backup artifact routes are canonical API payload boundaries.
`/api/backups/pve` owns Proxmox VE task, storage-archive, and guest-snapshot
collections. `/api/backups/pbs` owns Proxmox Backup Server artifacts and must
carry the PBS-authored `models.PBSBackup` fields, including datastore,
namespace, backup type, VMID, backup time, size, protection, verification,
files, owner, and comment. Recovery history may index those artifacts, but it
must not be the only API path for PBS protection or verification state when a
platform page needs source-native backup columns.

Hosted workspace handoff exchange is a canonical API/session payload boundary.
`/api/cloud/handoff/exchange` may redirect to a tenant-local target path only
when that path is carried in the signed control-plane JWT as `target_path`.
The exchange must ignore unsigned request-form or query redirect targets,
sanitize the signed target as a same-origin local path, fall back to `/` when
the claim is absent or unsafe, and include `target_path` in JSON responses only
after the same sanitization. This lets Pulse Account deep-link an MSP operator
to tenant-owned destinations such as agent installation or reporting without
turning the tenant exchange into an open redirect or moving API ownership for
agent tokens, alerts, or reports into the control plane.

Hosted tenant agent install commands are a canonical tenant-runtime API payload
boundary. The hosted route and its reusable command helper must produce the
same PVE/PBS install command shape while preserving tenant-local token
persistence, org binding, token metadata, and already-initialized tenant
monitor refresh behavior. Provider-hosted MSP control-plane proofs may exercise
that helper, but they must not create a second control-plane-only token path.

Performance report branding is a canonical reporting API payload boundary.
Reporting handlers may attach `ReportBranding` to the existing single-resource
and multi-resource report request contracts by combining provider-default
runtime configuration with an optional tenant-local `system.json` workspace
override. The API must pass the active `white_label` entitlement state into the
reporting layer, and the reporting layer remains the final gate: an unentitled
tenant runtime must render the normal Pulse report identity even if branding
configuration is present. Provider-hosted MSP proofs may exercise branding
through tenant runtimes, but the control plane must not gain MSP-specific report
generation plumbing or cross-client report content. Tenant-local workspace
branding overrides may carry display names and bounded inline logo data only;
local filesystem `logoPath` input is reserved for provider-default runtime
configuration and must not be accepted from persisted workspace settings.

Pulse Account workspace summaries carry setup state as a backend-owned payload
contract. Browser bootstrap and `/api/portal/dashboard` workspace entries may
include `setup_status` with only `ready`, `setup_path`, `install_agents`,
`configure_outputs`, or `review`, plus optional setup evidence counts such as
`agent_count`, `agent_token_count`, `unused_agent_token_count`,
`last_agent_seen_at`, `alert_route_count`, `disabled_alert_route_count`,
`report_schedule_count`, and `disabled_report_schedule_count` when the control
plane has a canonical source for them. MSP account bootstrap may also include
account-level `setup_templates` for provider guidance, but those templates are
copy and sequence guidance only; workspace readiness remains owned by the
workspace setup facts. The canonical source is a read-only tenant-local setup
facts reader, not
browser inference or portal-local state. Health review still wins first:
active workspaces with a failed latest health check report `review`. Healthy
or unchecked workspaces with no used non-expired `agent:report` token report
`install_agents`; workspaces with an agent but no enabled alert route or no
enabled report schedule report `configure_outputs`; only workspaces with at
least one reporting agent, enabled alert route, and enabled report schedule may
report `ready`. `setup_path` remains the fallback when no canonical setup facts
source is available. The browser may present that state as a setup checklist
and deep-link into tenant install/reporting surfaces, but it must not infer
cross-client readiness from health alone or mint tenant-owned agent, alert, or
report configuration from Pulse Account.
MSP account UI may present those same workspace entries as clients, but that
language is portal presentation only. Bootstrap and dashboard payloads continue
to expose account `kind`, `workspaces`, tenant IDs, role IDs, setup facts, and
member roles with the stable workspace field names. Mixed-account views must
scope MSP client wording to MSP account surfaces and keep Cloud or self-hosted
workspace wording scoped to their own account surfaces. Provider setup
templates are collapsible guidance over the account payload; they must not
replace tenant-local setup facts, become a readiness source, or require a
payload shape change when the portal presents compact client rows.

1. `cmd/pulse-mcp/main.go` shared with `ai-runtime`: the Pulse MCP adapter runtime is both an AI runtime surface for external-agent access to Pulse Intelligence and a canonical API contract projection over the agent capabilities manifest and Pulse MCP surface tool contract.
2. `cmd/pulse-mcp/README.md` shared with `ai-runtime`: the Pulse MCP adapter guide is both an AI runtime surface for external-agent access to Pulse Intelligence and a canonical API contract projection over the agent capabilities manifest.
   Manifest path placeholders projected by `cmd/pulse-mcp` are canonical API
   path parameters, not string concatenation hints: adapter calls must use the
   shared `internal/agentcapabilities` call projection helper to
   percent-encode placeholder values as single path segments before dispatching
   to Pulse so resource IDs and node IDs cannot change the declared route. That
   same helper owns the top-level-argument fallback for JSON request bodies:
   non-placeholder write arguments become the body when the caller does not
   provide a nested `body` object, and internal tool-call metadata such as
   approved-action grants is stripped before any public API body is built.
   MCP tool descriptions and fallback tool input schemas must project the same
   manifest-owned method/path, scope, request/response shape, category,
   action-mode, approval-policy, and stable error-code metadata through that
   shared helper rather than maintaining a second adapter-local capability
   table. The manifest-owned `surfaceToolContracts` field is the Pulse MCP
   tool allowlist for `tools/list` and `tools/call`, and the same shared
   `ManifestSurfaceToolCapabilities` resolver must gate execution, so
   streaming-only capabilities such as `subscribe_events`, or raw manifest
   capabilities omitted from the Pulse MCP surface contract, cannot be invoked
   through an MCP request/response tool even if a client sends their name
   manually. Adding a raw capability to `Manifest.Capabilities` is not enough
   to publish an MCP tool; the Pulse MCP `surfaceToolContracts` allowlist must
   opt it into that external-agent surface. A `surfaceToolContracts` entry may
   narrow the operator surface's affordance posture, but it must not re-enable a
   disabled surface affordance; normalized external-adapter surface summaries
   must clear tool and capability names when the effective `tools` affordance is
   false. MCP initialize capability
   advertisement and the shared manifest-backed tool, resource, and prompt
   handlers must also pass through the manifest-owned surface affordance
   contract, so tools, resources, prompts, and capability metadata are exposed
   only when the target surface allows that affordance and the manifest has the
   backing data to satisfy it. Prompt support is proven through
   `MCPManifestSurfacePromptProjectionSupported`, not a raw workflow-prompt
   presence check at individual call sites. Generated MCP prompt inventory must
   also pass through the same surface prompt projection gate rather than listing
   raw workflow prompts directly.
   MCP notification advertising, transport-event filtering, notification
   method projection, and SSE-to-MCP notification bridging are also shared API
   contract projections: the event kinds listed in
   `capabilities.experimental.pulseNotifications.kinds`, the
   `subscribe_events` manifest description, and the API SSE producer must use
   the same `internal/agentcapabilities` event vocabulary rather than
   adapter-local string lists.
   The agent SSE subscription transport, record parser, actionable-record
   filter, and MCP notification projector are part of that same shared boundary:
   `cmd/pulse-mcp` and reference probes may own connection lifetime and
   reconnect policy, but they must not carry their own event-stream request
   headers, subscribe status handling, `event:` / `data:` scanner loops,
   transport-event parser semantics, or SSE-to-MCP notification bridge.
   MCP JSON-RPC method constants, JSON-RPC envelopes, request decoding,
   notification response policy, shared manifest-backed and
   surface-affordance-gated tool-server semantics,
   shared tool-server method dispatch, initialize/tool-call/resource payloads,
   notification method projection, MCP resource URI projection,
   context-backed `resources/list` / `resources/read` projection, and prompt
   catalog/rendering payloads belong to the MCP adapter edge. MCP must not
   re-export neutral provider-call, provider-result, registry-preparation,
   result-construction, result-text, result-interpretation, or direct-execution
   helpers under MCP names; those helpers live under the neutral shared tool
   core and are consumed directly by Assistant and adapter surfaces. The raw
   `tools/call` params decode may stay in `mcp.go`, but parameter
   normalization and validation belong to the neutral shared tool-call core:
   tool names must be trimmed and required, argument maps must be cloned deeply
   enough to detach nested JSON-like `body` maps/slices and then initialized,
   and invalid params must fail before capability lookup so empty-name calls are
   not reported as unknown manifest tools.
   Projected-tool behavior hints are neutral API manifest projection state:
   `ToolBehaviorHints` belongs in the shared projection core, while
   `MCPToolAnnotations` may exist only as a protocol-facing alias at the MCP
   adapter edge.
   Native Assistant direct registry execution must consume the neutral
   `ToolCallParams` normalization/validation contract and the shared
   invalid-params, unknown-tool, and control-disabled tool result helpers from
   `internal/agentcapabilities/tool_call.go` before tool lookup or handler
   dispatch, rather than carrying a second name/argument path or local
   entrypoint failure envelope for in-app execution.
   The approved-action text projection for Pulse tool calls is part of that
   same boundary: API relay execution may receive the legacy command string,
   but parsing tool names, `default_api:` prefixes, and quoted arguments must
   happen in `internal/agentcapabilities`, return the shared
   `ToolCallParams` shape, and pass through the same normalization and
   validation contract as decoded MCP `tools/call` payloads. The same helper
   owns the `current_resource` handle plus governed aliases and recursive
   tool-argument detector so native Assistant, API relay execution, and
   MCP-compatible adapters cannot drift on unresolved attached-resource
   placeholder handling.
   Structured tool response envelopes, the tool-result error-code parser,
   the `verification.ok` evidence parser, provider tool/call/result shape projection,
   provider-result context projection for full transcript plus model-context results,
   neutral provider-tool behavior hints and Pulse governance metadata,
   provider-emitted streamed tool-input parsing, final provider tool-input raw
   fallback handling, native Assistant provider-tool name catalog and exact/prefix matching,
   provider tool-call artifact detection and streaming tool-name prefix holding, question-type/defaulting
   vocabulary, native Assistant question input parsing, and tool error-code/FSM recovery vocabulary are part of that same boundary; native Assistant tool
   protocol wrappers may preserve only neutral registry tool aliases and must
   not expose MCP/JSON-RPC wire aliases or carry local copies of provider
   tool JSON, provider call params, chat tool-call JSON, chat tool-result JSON,
   provider-native tool-call artifact regexes, the blocked/failed/retryable shape, common error-code strings, recovery error-code parsing, write self-verification evidence parsing, or the
   approval-required / policy-blocked marker prefixes, payload `type` values,
   formatter helpers, parser helpers, or typed approval-required payload
   contract used by governed Assistant tool outcomes. Chat and API consumers
   may project that shared marker payload into their own event shapes, but they
   must not define anonymous local approval-marker readers or discard the shared
   approved-action argument map returned for replayed tool execution. MCP-shaped
   provider projection aliases and structured tool-response result aliases must
   not exist as helper wrappers; MCP may keep protocol wire aliases only for the
   actual `tools/call` params/result envelopes while using the neutral provider,
   tool-response, and direct-execution helpers by their shared names.
   Assistant registry tool schemas must also behave as API-contract values:
   `agentcapabilities.Tool.NormalizeCollections` owns independent copies of
   structured `InputSchema` maps, required slices, enum slices, provider
   input-schema maps, and JSON-schema default containers; direct provider
   schema projection helpers must normalize through that same path before
   emitting provider JSON, while `ToolRegistry.Register` and
   `ToolRegistry.ListTools` must normalize at their respective boundaries
   instead of exposing mutable caller-owned or registry-owned schema
   collections.
   Manifest-backed external-agent tools follow the same rule:
   `agentcapabilities.CloneCapability`, `CloneCapabilities`, and
   `CloneRawMessage` must detach raw manifest `inputSchema` and `outputSchema`
   bytes, structured `_meta["pulse.capability"]` maps, and error-code slices
   before lookup or projection results leave the shared boundary, so MCP
   `tools/list` serialization and adapter post-processing cannot mutate the
   fetched or canonical manifest.
   Provider tool-call input normalization is owned there too: provider
   responses, stored Assistant transcript calls, and API-facing chat-message
   projections must clone tool input maps, nested JSON-like argument values, and
   raw provider thought-signature payload bytes
   through the shared `ProviderToolCall.NormalizeCollections` path rather than
   reusing mutable caller maps across surfaces.
   Provider result context projection is owned there as well: native Assistant,
   external adapters, and future Pulse Intelligence surfaces must use the shared
   projection for full transcript results and model-context results instead of
   locally pairing provider tool-result structs or reimplementing the truncation
   notice.
   Shared tool result normalization is also owned by the neutral tool-result
   core:
   `agentcapabilities.ToolResult.NormalizeCollections` must initialize empty
   content collections, detach content slices, and clone structuredContent maps
   before native Assistant registry execution or external MCP method dispatch
   returns a result envelope.
   Result-text extraction, result interpretation, JSON object and array-wrapper
   structuredContent projection, and `HTTPCallResponse` to shared tool-result
   wrapping must live in `internal/agentcapabilities/tool_result.go`; MCP may
   alias only the actual `MCPToolResult`/`MCPContent` wire envelopes and must
   not keep MCP-named constructor, text, or interpretation wrappers.
   Direct native execution over those shared result envelopes must use the
   surface-neutral `agentcapabilities.InterpretDirectToolExecution` helper;
   MCP-named direct-execution symbols are not part of the adapter contract.
   Manifest fetches, agent request
   construction, API-token header placement, JSON content-type posture,
   capability HTTP invocation, neutral `ExecuteCapabilityToolHTTP`
   request/response execution, request/response capability body-return helpers,
   and stable non-2xx error-envelope formatting are
   shared-core behavior. `cmd/pulse-mcp` may own stdio and retry/backoff policy,
   but it must not re-declare the MCP wire/result structs or local agent HTTP
   client rules that Assistant tool execution and reference clients also
   consume.
3. `frontend-modern/src/api/agentCapabilities.ts` shared with `ai-runtime`: the agent capabilities frontend client is both the Pulse Intelligence external-agent manifest consumer and a canonical API payload contract boundary.
4. `frontend-modern/src/api/agentProfiles.ts` shared with `agent-lifecycle`: the agent profiles frontend client is both an agent lifecycle control surface and a canonical API payload contract boundary.
5. `frontend-modern/src/api/ai.ts` shared with `ai-runtime`: the AI frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
   `/api/settings/ai` is the provider-registry projection for Assistant and
   Patrol runtime configuration. Its `providers` collection exposes
   non-secret provider metadata from the config-layer registry, including
   provider id, display name, protocol family, default route, default base URL,
   credential field names, configured-state field names, clear-key field names,
   env-var hints, docs URL, gateway flag, and current configured state. The
   legacy top-level configured booleans may remain for browser compatibility,
   but new provider additions must update the registry-backed projection and
   contract tests rather than adding an untyped browser-only provider list.
   API responses must never echo provider secret values; settings updates may
   accept credential and clear-key fields, persist trimmed values, and return
   only configured state.
6. `frontend-modern/src/api/aiChat.ts` shared with `ai-runtime`: the Assistant chat frontend client is both the first-party Assistant transport surface and a canonical API payload contract boundary.
7. `frontend-modern/src/api/generated/agentCapabilities.ts` shared with `ai-runtime`: the generated agent capabilities frontend types are both the Pulse Intelligence manifest TypeScript projection and a canonical API payload contract boundary.
8. `frontend-modern/src/api/nodes.ts` shared with `agent-lifecycle`: the shared Proxmox node client is both an agent lifecycle setup/install control surface and a canonical API payload contract boundary.
9. `frontend-modern/src/api/notifications.ts` shared with `notifications`: the notifications frontend client is both a notification delivery control surface and a canonical API payload contract boundary.
10. `frontend-modern/src/api/orgs.ts` shared with `organization-settings`: the organization frontend client is both an organization settings control surface and a canonical API payload contract boundary.
11. `frontend-modern/src/api/patrol.ts` shared with `ai-runtime`: the Patrol frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
    The Patrol status payload owns Patrol readiness as structured API state:
    provider/model/settings/tool prerequisites must travel as bounded readiness
    checks instead of frontend-only heuristics or generic analysis-failed text.
    `/api/settings/ai/update` must persist recoverable provider/model changes
    and return the same structured readiness cause in its settings payload, while
    `/api/ai/patrol/run` must use the `patrol_readiness_not_ready` error taxonomy
    when it rejects a known-bad Patrol runtime configuration. Bounded `status`,
    `cause`, `provider`, and `model` details are the canonical transport shape.
    Patrol may demote the shared model catalog behind an explicit advanced UI
    action, but it must still source selectable model routes from the shared AI
    runtime catalog/settings payloads rather than introducing a Patrol-local
    model list or inferred provider catalog.
    `/api/ai/patrol/findings` is the canonical Patrol-page findings source, and
    `/api/ai/patrol/runs` records may expose bounded `source` provenance so demo
    evidence can be separated from live runtime assessment state without
    changing the stable run payload shape.
    The Patrol status payload's `trigger_status` object is the canonical
    transport for alert/anomaly trigger queue and mode facts. Patrol summary UI
    may present that state as factual trigger-mode context, but it must not infer
    trigger enablement or queue state from settings payloads, route state, or
    run-history labels.
12. `frontend-modern/src/api/rbac.ts` shared with `organization-settings`: the RBAC frontend client is both an organization settings control surface and a canonical API payload contract boundary.
13. `frontend-modern/src/api/security.ts` shared with `security-privacy`: the security frontend client is both a security/privacy control surface and a canonical API payload contract boundary.
14. `frontend-modern/src/api/updates.ts` shared with `deployment-installability`: the updates frontend client is both a deployment-installability control surface and a canonical API payload contract boundary.
    It must preserve `/api/updates/plan.readiness` payloads as backend-owned
    API state so settings UI can render `ready`, `attention`, and `blocked`
    update checks without rebuilding upgrade state locally. The frontend may
    disable blocked installs, but the backend apply route remains authoritative
    and must reject a recomputed `blocked` readiness verdict.
15. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx` shared with `ai-runtime`, `frontend-primitives`: the External agents settings panel is the optional settings-shell projection of Pulse MCP onboarding, the AI runtime connected-agent onboarding surface, and a presentation consumer of the shared agent capabilities frontend client.
    The panel may own settings placement, copy composition, local fallback
    rendering, and the user-facing hierarchy that frames Patrol as the primary
    governed operator with MCP clients as secondary access to the same approval
    policy and action history. Capability category order, labels,
    descriptions, required scopes, the Pulse Intelligence surface contract,
    surface affordance badges, and capability rows must still come through
    `frontend-modern/src/api/agentCapabilities.ts` instead of a browser-local
    category table or component-local manifest fetcher.
    Manifest-owned prompt, scope, failure-code, tool-contract posture, and raw capability
    inventories should be available for client builders but must stay demoted
    behind a builder-oriented Developer details disclosure rather than occupying
    the default Assistant settings view. The external-agent setup checklist must
    stay behind a user-triggered `Show connector setup` disclosure, while direct setup
    links open that disclosure automatically. Posture and policy summaries
    belong under Patrol access model, while prompt, scope, failure-code, and
    tool inventories must be nested behind Live manifest details so opening the
    developer disclosure does not immediately dump the full manifest. Visible
    nested labels should use user-facing terms such as Agent starting points and
    Agent capabilities rather than raw workflow/MCP inventory labels. External
    adapter posture must read as `External agents expose ... through Patrol
    mode`; `Pulse MCP` and `pulse_operations_loop` may remain only as manifest
    or wire names inside builder/debugging detail, not as the visible product
    model a user must learn.
    The External agents Patrol handoff must use `Choose Patrol mode` and
    frame the first step as choosing what Patrol may handle automatically before
    a connected agent can request work; connected-agent setup must not revive
    activation-loop or raw MCP readiness language as the primary call to action.
    When the setup checklist is expanded, Step 1 owns the visible Patrol-mode
    handoff and the panel header must not repeat the same call to action.
    The same wording applies when API-owned projections feed Patrol availability
    or plan-comparison presentation: customer-facing helpers must describe
    governed scope as what Patrol may handle automatically, not as how far Patrol
    can go or how much control Patrol has.
    Client config follows the same product hierarchy: it may expose
    manifest-derived OpenCode and Claude-style wrappers, but the default panel
    must lead with Patrol control and scoped-token setup, not raw JSON blocks.
    Direct `/settings/pulse-intelligence/assistant#external-agent-setup` links
    must scroll and briefly focus the External agents panel after the
    Assistant settings layout settles, so the external-agent setup route lands
    on the optional client setup boundary rather than generic token inventory.
    Legacy `/settings/security/api#external-agent-setup` and
    `/settings/security/api#pulse-mcp-setup` links must remain accepted and
    redirect to the canonical Pulse Intelligence Assistant route. Normal API
    Access visits keep token management first; token-preset links may still
    land on API Access because credential minting remains the API token
    contract, not a separate credential or execution surface.
    The Patrol control handoff must target the Patrol operator surface where the
    inline control level is configured, while API Access remains the token
    and external-agent setup surface.
    Failure-code
    summaries in the panel must derive from capability `errorCodes` through the
    same frontend manifest client instead of a browser-local recovery-code
    registry. Explanatory capability-publication copy in that panel must point
    at the manifest-owned surface contract, not raw backend capability rows, so
    the API contract remains clear that Pulse MCP publication requires the
    `surfaceToolContracts` boundary. Token setup handoffs in
    this panel may link to the API token creation form, but the named
    full-surface preset must remain the manifest-derived `Patrol external agent`
    preset from `frontend-modern/src/components/Settings/APITokenManager.tsx`
    and `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`; the
    Agent integrations panel must not hardcode or recompute the required token
    scope set. MCP adapter setup facts in the panel are also manifest-owned:
    server name, command, base URL flag/default, token environment variable,
    supported config families, and copied OpenCode/`mcpServers` snippets must
    flow through `frontend-modern/src/api/agentCapabilities.ts` from
    `/api/agent/capabilities.mcpAdapter`, not component-local constants.
    The default setup copy must frame connected agents as optional access to
    Pulse context and Patrol work, keep Patrol as the built-in operator that
    watches, acts within Patrol mode, asks when approval is required, verifies
    outcomes, and records history, and say external agents use that same
    boundary. It must not use "Use Pulse MCP only" or "outside client" copy, and
    it must not describe external-agent setup as a separate operator journey.
    The Patrol control handoff belongs to the Patrol control route; API Access
    may mint the token and show MCP client setup, but it must not send operators
    to Assistant settings to choose Patrol's control level.
16. `frontend-modern/src/components/Settings/APITokenManager.tsx` shared with `security-privacy`: the API token settings surface is both a security/privacy control surface and a canonical API payload contract boundary.
    The API token inventory table may own credential and usage cells, but it
    must inherit embedded table framing from `frontend-primitives`
    `PulseDataGrid` rather than carrying API-token-local scroll or border
    wrappers around the grid.
    Docker and Podman token usage copy must come from
    `frontend-modern/src/utils/apiTokenPresentation.ts` and its shared source
    platform label, so API-token tables, revoke warnings, and token presets do
    not reintroduce generic `container runtime` operator-facing labels.
    API-token scope-reference documentation links may compose
    frontend-primitives' `ExternalTextLink`; API contracts own the token scope,
    usage, preset, and revoke semantics, not new-tab anchor safety or link
    chrome.
    Token refresh/loading state remains API contract data only while the
    visible spinner shell routes through frontend-primitives-owned
    `LoadingSpinner` instead of an API-token-local animate-spin SVG.
    The create-token section may expose stable in-page anchors for sibling
    API Access onboarding surfaces, but preset scope contents must continue to
    flow from the API manifest client and token manager model rather than from
    anchor callers such as Agent integrations.
    Empty scope selection is not a wildcard request in the token manager UI:
    generation must require an explicit scoped preset, custom scope, or
    deliberate Full access selection before the frontend sends a create-token
    payload.
    Token scope selector semantics stay API-contract owned, but the visible
    pressed/unpressed selector pill shell routes through frontend-primitives
    `SelectablePillButton` instead of API-token-local rounded-full selector
    classes.
17. `frontend-modern/src/components/Settings/apiTokenManagerModel.ts` shared with `security-privacy`: the pure API token settings model is both a security/privacy control surface and a canonical API payload contract boundary.
18. `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx` shared with `agent-lifecycle`: the inline node credential slot is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
19. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx` shared with `agent-lifecycle`: the pure infrastructure operations inventory/install model is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
    Docker and Podman install-profile labels and descriptions in that shared
    model must derive their customer-facing source name from
    `frontend-modern/src/utils/sourcePlatforms.ts` rather than page-local
    wording, because those install choices set API-token and install-command
    expectations for the operator.
    That same install-profile model must keep the standalone Docker / Podman
    runtime profile distinct from Proxmox LXC Docker inventory. The Docker
    profile may force local runtime monitoring, while Docker inside LXCs must
    point operators to the Proxmox VE node profile plus command execution and
    explicit server-side guest Docker inventory opt-in.
    That same browser/API boundary must not retain customer-side commercial or
    onboarding telemetry wrappers around infrastructure operations. Pulse
    Account and the server-owned commercial reporting routes own commercial
    event ingestion; the infrastructure operations model and install state may
    navigate to canonical destinations, but must not import or call local
    `upgradeMetrics`, `conversionEvents`, or infrastructure onboarding metrics
    wrappers.
    Agentless availability targets and structured `/api/connections` fleet posture share this same settings/API boundary as
    monitoring-managed endpoint configuration, not host-install lifecycle or prose-derived row labels.
    `internal/api/availability_handlers.go` owns CRUD and test payloads for
    `/api/availability-targets`, while
    `frontend-modern/src/api/availabilityTargets.ts` and
    `frontend-modern/src/components/Settings/AvailabilitySettingsPanel.tsx`,
    `frontend-modern/src/components/Settings/availabilitySettingsModel.ts`,
    and `AvailabilityTargetSlot.tsx` own the browser transport shape.
    Availability target probe-result and error notices may compose
    frontend-primitives' `CalloutCard` for shared settings callout chrome;
    API contracts own the target CRUD/test payload semantics and endpoint
    routing, not local colored alert shells.
    Connections ledger rows with type
    `availability` must route management handoffs, pause, remove, and test
    actions to those availability-target endpoints and must not reuse node,
    SSH, platform API, or Pulse Agent setup payloads.
    Mock mode must expose authored availability targets through those same
    list, saved-test, and connections-ledger payloads so demo endpoints exercise
    the canonical API contract rather than a frontend-only fixture.
20. `frontend-modern/src/components/Settings/NodeModalAuthenticationSection.tsx` shared with `agent-lifecycle`: the node setup authentication section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
21. `frontend-modern/src/components/Settings/NodeModalBasicInfoSection.tsx` shared with `agent-lifecycle`: the node setup basic-info section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
22. `frontend-modern/src/components/Settings/nodeModalModel.ts` shared with `agent-lifecycle`: the pure node setup modal model is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
23. `frontend-modern/src/components/Settings/NodeModalMonitoringSection.tsx` shared with `agent-lifecycle`: the node setup monitoring section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
24. `frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx` shared with `agent-lifecycle`: the node setup guide section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
25. `frontend-modern/src/components/Settings/NodeModalStatusFooter.tsx` shared with `agent-lifecycle`: the node setup status/footer section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
26. `frontend-modern/src/components/Settings/useAPITokenManagerState.ts` shared with `security-privacy`: the API token settings state hook is both a security/privacy control surface and a canonical API payload contract boundary.
27. `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts` shared with `agent-lifecycle`: the direct-node infrastructure settings state hook is both an agent lifecycle control surface and a shared Proxmox node API contract boundary.
28. `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts` shared with `agent-lifecycle`: the infrastructure discovery runtime state hook is both an agent lifecycle control surface and a shared discovery/settings API contract boundary.
    That same shared boundary also owns settings-route polling scope for discovery payloads: the `/api/discover` refresh loop and websocket-backed discovery status hydration may run only while the operator is on the canonical Infrastructure settings workspace at `/settings/infrastructure`, not on retired infrastructure sub-routes.
    Discovery refreshes and scan results are route-scoped state updates. If the
    settings route unmounts while a request is in flight, the hook must drop the
    result and avoid surfacing stale background errors into the next settings
    panel.
    It also owns the explicit manual-scan contract for `/api/discover`: when
    the operator runs discovery from the infrastructure manager, the hook must
    consume the immediate POST response body as the next source of truth for
    discovered candidates and scan errors rather than waiting for a later poll
    or websocket update. Cached GET payloads and manual POST payloads must both
    normalize their `updated` / `timestamp` values into one millisecond-backed
    `lastResultAt` state so discovery review rows do not depend on transport-
    specific timestamp shapes.
29. `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx` shared with `agent-lifecycle`: the infrastructure install state hook is both an agent fleet lifecycle control surface and an API token, lookup, and install transport contract boundary.
    Setup-completion handoff token creation is valid only while the active
    infrastructure add step is an installer route (`agent`, `linux-host`,
    `unraid`, `docker`, or `kubernetes`). Mounting the infrastructure workspace
    for unrelated settings panels must not auto-create install tokens from a
    stale setup handoff.
30. `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx` shared with `agent-lifecycle`: the shared infrastructure operations state hook is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
31. `frontend-modern/src/components/Settings/useNodeModalState.ts` shared with `agent-lifecycle`: the node setup modal state hook is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
32. `frontend-modern/src/constants/apiScopes.ts` shared with `security-privacy`: the API token scope catalog is both a security/privacy token-management trust surface and a canonical API token payload boundary.
    Docker and Podman scope labels must use the shared source platform label
    rather than generic `container` copy, because those labels surface directly
    in token presets, custom scopes, and inventory badges.
    The `ai:execute` scope label is customer-facing API contract copy: it must
    describe Pulse Intelligence actions as governed Patrol actions for plans,
    approvals, policy-allowed fixes, verification, and history rather than as
    generic operations workflows or a separate agent product.
33. `frontend-modern/src/utils/agentInstallCommand.ts` shared with `agent-lifecycle`: the shared frontend install-command helper is both an agent lifecycle control surface and a canonical API/install transport contract boundary.
    Generated install commands are part of the API contract because they bind
    the UI-selected Pulse URL, token source, custom CA, insecure/plain-HTTP
    behavior, and `/download/pulse-agent?arch=...` availability proof into the
    installer invocation. Windows, macOS, and Linux commands must therefore
    preflight the exact platform artifact and avoid raw token process
    arguments. Windows generated commands must pass boolean installer intent
    through environment variables and invoke `install.ps1` through child
    PowerShell `-File` calls so Windows PowerShell argument binding cannot
    reinterpret copied `$true` values as strings or let preflight-only returns
    abort the parent wrapper before install.
34. `frontend-modern/src/utils/apiTokenPresentation.ts` shared with `security-privacy`: the API token presentation helper is both a security/privacy control surface and a canonical API token management boundary.
    It owns the operator-facing Docker / Podman token vocabulary used by API
    Access, token presets, usage summaries, and revoke warnings.
35. `frontend-modern/src/utils/infrastructureSettingsPresentation.ts` shared with `agent-lifecycle`: the infrastructure settings presentation helper is both an agent lifecycle control surface and an API-backed direct-node/discovery settings boundary.
36. `internal/agentcapabilities/action_target.go` shared with `ai-runtime`: the Pulse Intelligence governed action target type and resource-to-action-target mapping vocabulary are both the Assistant approval/runtime routing contract and the canonical API/agent target contract for governed actions.
37. `internal/agentcapabilities/control_level.go` shared with `ai-runtime`: the Pulse Intelligence control-level vocabulary and control-tool availability predicate are both the Assistant runtime gating contract and the canonical API/agent permission posture for governed action tools.
38. `internal/agentcapabilities/errors.go` shared with `ai-runtime`: the Pulse Intelligence agent error envelope is both the canonical API failure payload contract and the AI runtime adapter error-parsing contract for Assistant and external-agent surfaces.
39. `internal/agentcapabilities/events.go` shared with `ai-runtime`: the Pulse Intelligence event vocabulary is both the canonical API SSE event contract and the AI runtime adapter notification contract for Assistant and external-agent surfaces.
40. `internal/agentcapabilities/governance_prompt.go` shared with `ai-runtime`: the Pulse Intelligence surface-affordance-resolved model-facing operating-instruction, tool-governance prompt, reusable provider-tool governance description, Assistant-native offered-tool filtering, and Assistant-native interactive question-tool governance projections are both the Assistant system-prompt governance section and the shared API/agent vocabulary for action mode, approval posture, MCP affordance advertisement, and non-registry interaction-tool boundaries.
41. `internal/agentcapabilities/http.go` shared with `ai-runtime`: the Pulse Intelligence agent HTTP substrate is both the API capabilities invocation contract and the shared AI runtime adapter execution primitive for MCP and reference agent clients.
42. `internal/agentcapabilities/manifest.go` shared with `ai-runtime`: the canonical Pulse Intelligence agent capabilities manifest declaration, including capability display titles, manifest-owned finding lifecycle schemas, manifest-owned governed action schemas and routes, manifest-owned external-adapter surface tool contracts, and manifest-owned structured output schemas, is both the API discovery payload source and the AI runtime projection contract for Pulse Assistant and MCP-facing agent tools.
43. `internal/agentcapabilities/markdown.go` shared with `ai-runtime`: the Pulse Intelligence manifest Markdown projection, including manifest-owned capability titles, surface-filtered Pulse MCP tool/error inventories, and prompt labels, is both the canonical API/agent documentation projection and the AI runtime onboarding projection for Assistant-compatible external-agent surfaces.
44. `internal/agentcapabilities/mcp.go` shared with `ai-runtime`: the Pulse Intelligence MCP protocol version, JSON-RPC, method dispatch, method payload, surface-tool-contract-gated initialize operating-instruction and capability advertisement payload, manifest surface-filtered tools/list and tools/call execution bridge, manifest surface-gated resources/list and resources/read bridge, manifest-owned and surface-affordance-gated workflow prompt projection, protocol wire aliases, resource and prompt handler gates, and notification projection collectively define the external-agent adapter wire contract over the shared Pulse Intelligence tool core; MCP initialize, tools/call execution, resource list/read projection, and prompt list/get projection must enter through manifest-owned surface and workflow-prompt contracts so raw capability slices cannot bypass the published external-adapter contract.
45. `internal/agentcapabilities/mcp_adapter.go` shared with `ai-runtime`: the Pulse MCP adapter setup contract defaults and normalization are both the canonical API manifest setup projection and the AI runtime onboarding contract for Assistant-compatible external-agent surfaces.
46. `internal/agentcapabilities/projection.go` shared with `ai-runtime`: the agent capability external-tool projection helper, normalized manifest-owned surface tool contract resolution and tools-affordance gating, manifest-owned resource-context route and argument vocabulary, operator-state capability and route vocabulary, finding workflow capability and lifecycle argument vocabulary including resolution and dismissal notes, governed action capability, route, and argument vocabulary, manifest-owned tool title and outputSchema projection, structured Pulse capability _meta, and shared tool behavior hints are both the canonical API manifest projection contract and the AI runtime adapter projection for Pulse Assistant and MCP-facing agent tools, with MCP annotation and metadata wire names confined to adapter-edge aliases.
47. `internal/agentcapabilities/provider_tool_artifacts.go` shared with `ai-runtime`: the provider tool-call artifact detector and streaming tool-name prefix splitter are both the Assistant stream-sanitization boundary and the shared external-adapter leak guard for provider-native tool-call markup that escaped the structured channel.
48. `internal/agentcapabilities/schema.go` shared with `ai-runtime`: the agent capability input schema contract is both the canonical API manifest schema envelope and the AI runtime structured tool-schema, governance-aware provider-projection with neutral behavior hints and Pulse governance metadata, offered-tool governance extraction for Assistant prompt policy, manifest-affordance-gated Assistant provider-surface composition, manifest raw-schema to Assistant provider-schema projection for capability tools, legacy native Assistant utility provider aliases and schemas, provider-call normalization, provider-result context projection, Assistant-native interaction provider-tool declaration, and live Assistant execution-normalization contract for Pulse Assistant and MCP-facing agent tools.
49. `internal/agentcapabilities/scopes.go` shared with `ai-runtime`: the manifest-derived required-scope summary is both the canonical API/agent token guidance contract and the AI runtime adapter startup/onboarding contract for Assistant-compatible external-agent surfaces.
50. `internal/agentcapabilities/sse.go` shared with `ai-runtime`: the Pulse Intelligence SSE subscription transport and record parser are both the canonical API event-stream consumption contract and the AI runtime adapter push bridge contract for MCP and reference agent clients.
51. `internal/agentcapabilities/surface_contract.go` shared with `ai-runtime`: the Pulse Intelligence operator-surface affordance contract, shared surface-affordance, surface-tool identity, Assistant surface tool filtering, normalized external surface tool resolver, surface lookup, affordance labels, and manifest-published external-adapter surface tool allowlist projection are both the canonical API manifest surface model and the AI runtime prompt and onboarding guardrail for Assistant and MCP-facing surfaces.
52. `internal/agentcapabilities/text_tool_invocation.go` shared with `ai-runtime`: the Pulse Intelligence text tool invocation parser, internal approval argument, and current_resource handle vocabulary are both the Assistant approved-action execution projection and the shared tool-call params bridge for governed Pulse Intelligence tool calls, with MCP tools/call compatibility staying at the adapter edge.
53. `internal/agentcapabilities/tool_call.go` shared with `ai-runtime`: the Pulse Intelligence shared tool-call params, normalization, validation, direct registry preparation, registry-entrypoint failure result helpers, and provider/registry tool-call safety classification are both the native Assistant execution/FSM contract and the canonical API/agent tools/call compatibility contract for governed Pulse Intelligence tool calls.
54. `internal/agentcapabilities/tool_execution.go` shared with `ai-runtime`: the Pulse Intelligence neutral capability tool HTTP execution helper and direct tool execution output/error mapper are both the Assistant-native direct execution contract and the canonical API/agent request/response execution contract, with MCP adapters consuming the neutral helpers only after the shared MCP manifest-surface execution bridge has applied the published surface tool contract.
55. `internal/agentcapabilities/tool_marker.go` shared with `ai-runtime`: the Pulse Intelligence Assistant tool marker vocabulary and approval/policy marker parser are both the Assistant structured tool-result compatibility contract and the canonical API/agent branching contract for governed tool outcomes.
56. `internal/agentcapabilities/tool_names.go` shared with `ai-runtime`: the Pulse Intelligence registry tool-name vocabulary is both the native Assistant execution/display contract and the canonical API/agent tool identity contract for MCP-facing external-agent adapters.
57. `internal/agentcapabilities/tool_response.go` shared with `ai-runtime`: the shared tool response envelope, tool error-code vocabulary, and tool-result error-code and verification evidence parsers are both the Assistant structured tool-result contract and the canonical API/agent branching contract for Pulse Intelligence tool failures, recovery tracking, and write self-verification.
58. `internal/agentcapabilities/tool_result.go` shared with `ai-runtime`: the Pulse Intelligence shared tool-result content/result envelope, structuredContent projection, result constructors, HTTP response-to-result mapping, text projection, and result interpretation helpers are both the Assistant registry result contract and the canonical API/agent result projection contract for governed tool outcomes.
59. `internal/agentcapabilities/types.go` shared with `ai-runtime`: the agent capabilities manifest wire type, manifest-owned external-adapter surface tool contract field, capability display title and structured output schema fields, approval-policy vocabulary, capability governance normalization, and tool-governance descriptor shape are both the canonical API payload contract and the AI runtime projection contract for Pulse Assistant and MCP-facing agent tools.
60. `internal/agentcapabilities/workflow_prompt.go` shared with `ai-runtime`: the Pulse Intelligence workflow prompt catalogue, manifest-owned `workflowPrompts` projection, MCP prompt title projection, presentation kind hints, shared resource-context and finding argument vocabulary, Patrol issue-handling capability gating, argument validation, and manifest-gated shared prompt rendering rules are both the AI runtime starter contract for Assistant-compatible surfaces and the canonical API/agent prompt projection contract for MCP-facing clients.
61. `internal/api/access_control_handlers.go` shared with `organization-settings`: RBAC role and user-assignment handlers are both an organization settings control surface and a canonical API payload contract boundary.
    The shared node setup boundary above owns the guided/manual setup split
    for PVE/PBS consumers: API Inventory and Host Telemetry Agent setup modes
    are auto-registration paths, while Token ID/Value fields, Test Connection,
    and Add Node are manual-token or existing-node edit controls only.
    That same client contract must expose the setup strategy before a token
    path is chosen: API Inventory is the recommended least-privilege API path,
    Host Telemetry Agent is the optional full-host-telemetry root-agent path,
    and Manual Token Setup is a manual API-token escape hatch.
    For PVE, the Host Telemetry Agent setup path is also the governed
    host-side Docker-in-LXC inventory setup path: the frontend request to
    `/api/agent-install-command` must explicitly ask for command execution, the
    generated install command must render `--enable-commands`, and the default
    API Inventory path must not imply it can run the opted-in host-side Docker
    inventory collector.
    The inline node credential slot must keep the visible submit sequence as
    `Endpoint`, `Authentication`, and `Coverage` before the API-backed setup
    controls. That sequence is presentation guidance for the existing setup
    payload phases; it does not create a second node setup API model or allow
    page-local payload ownership.
62. `internal/api/agent_install_command_shared.go` shared with `agent-lifecycle`: agent install command assembly is both an agent lifecycle control surface and a canonical API payload contract boundary.
    Frontend and backend Unix install command builders must stay on the same
    token-file and preflight transport contract: tokens are passed to the
    installer as ephemeral files, and host install snippets must verify the
    target Pulse URL plus exact agent binary artifact before root escalation.
    The shared Proxmox agent install command contract may expose
    `--enable-commands` only through an explicit request field. The PVE Host
    Telemetry Agent setup surface is the approved caller for that opt-in when
    the operator wants Patrol actions or server-opted-in Docker-in-LXC
    inventory from the Proxmox node; generic Proxmox API Inventory setup must
    remain least-privilege and command-execution-free.
    32a. `internal/api/cloud_agent_install_command.go` shared with `agent-lifecycle`, `cloud-paid`: hosted tenant agent install commands are agent lifecycle enrollment transport, hosted/provider MSP tenant boundary, and canonical API payload contract.
    The route and reusable helper must both mint PVE/PBS install tokens only in
    hosted mode, only for an existing tenant/org, and only into that tenant
    runtime's token store with the org boundary, command shape, token metadata,
    and already-loaded tenant-monitor refresh behavior preserved.
63. `internal/api/ai_handler.go` shared with `ai-runtime`: Pulse Assistant handlers are both an AI runtime control surface and a canonical API payload contract boundary.
    Assistant session list payloads may expose only the safe
    `handoff_summary` projection needed by the browser to mark and restore a
    scoped handoff. The payload must not expose provider-bound model context,
    raw commands, preflight output, action results, or remediation command
    descriptions. That safe projection owns Patrol handoff identity as data,
    not browser inference: `kind` may identify `patrol_assessment`,
    `patrol_configuration_failure`, `patrol_run`, `patrol_finding`, or generic
    `scoped_context`; assessment and configuration-failure sessions must not be
    collapsed into one finding because a safe action reference contains a
    finding ID. `patrol_configuration_failure` may carry only the
    runtime-failure boolean needed for drawer/session presentation, and
    run-specific fields remain reserved for `patrol_run`. Chat requests that
    carry `handoff_metadata.kind=patrol_run` are identity envelopes, not
    browser-authored model context: the API handler must resolve the run ID
    through the backend Patrol service, rebuild the model-only run context and
    resources server-side, and ignore request-side run context, resources, or
    actions when the run cannot be resolved. Stored session metadata must be
    readable by the handler so follow-up turns can rehydrate the same backend
    context without asking the browser to resend provider-bound payloads.
    `GET /api/ai/sessions` also owns the Assistant history lookup contract:
    optional `search` text must filter the browser-safe session projection
    before optional `limit` truncation, and matching may use only safe session
    identity, title, timestamps, and handoff-summary fields. Search must not
    expose provider-bound model context, stored prompts, raw tool output, or
    remediation details, and frontend clients must route searchable picker
    requests through the shared `AIChatAPI.listSessions({ search, limit })`
    helper rather than inventing a parallel local history endpoint.
    `PATCH /api/ai/sessions/{id}` owns the Assistant session-title mutation
    contract. The request body is limited to a user-visible `title`; the
    handler must reject empty titles, the chat store must normalize whitespace
    and bound persisted title length, and the response must be the updated
    browser-safe `ChatSession` projection. Rename must not expose or mutate
    stored prompts, messages, provider reasoning, model handoff context,
    approvals, action state, or tool evidence. Browser clients must call the
    shared `AIChatAPI.renameSession(sessionId, title)` helper so path encoding
    and JSON body shape stay canonical.
    `POST /api/ai/sessions/{id}/undo` and
    `POST /api/ai/sessions/{id}/redo` own the Assistant turn-repair API
    contract. Undo removes the latest user-authored turn and all later
    assistant/tool messages as one durable unit, returns a browser-safe
    `restored_prompt`, `removed_messages`, and `can_redo`, and must not expose
    provider reasoning, raw tool output, model-only handoff text, approval
    payload internals, or remediation command data. Redo restores the latest
    undone turn and returns `restored_messages` plus the remaining `can_redo`
    state. `GET /api/ai/sessions` and other `ChatSession` projections may
    expose only the boolean `can_redo` hint alongside safe title/timestamp,
    count, and handoff-summary fields so the drawer can re-enable redo after a
    reload without reading the redo stack itself. Browser clients must use the
    shared `AIChatAPI.undoLastTurn(sessionId)` and
    `AIChatAPI.redoLastTurn(sessionId)` helpers so path encoding and response
    shape stay canonical.
    OpenCode-style file diff/revert session routes are deliberately not part
    of Pulse's supported Assistant session contract: Pulse sessions do not own
    local code-file edits, and infrastructure mutations must be reviewed
    through governed approval/action history. Browser clients must not expose
    or call `GET /api/ai/sessions/{id}/diff`,
    `POST /api/ai/sessions/{id}/revert`, or
    `POST /api/ai/sessions/{id}/unrevert`; legacy direct calls to those routes
    return `501 Not Implemented` with an explicit unsupported message rather
    than a placeholder success payload.
    Resource-context follow-up turns are different from Patrol-run rehydration:
    browser-safe `handoff_metadata.kind=resource_context` must not replace a
    stored rich handoff envelope with a partial metadata-only envelope, and the
    handler must not ask the browser to resend resource context that the chat
    runtime can rehydrate from the stored selected-resource envelope.
    Assistant message-history payloads are part of the same browser-safe
    projection boundary: `GET /api/ai/sessions/{id}/messages` must encode only
    messages passed through `chat.Message.ClientSafe()`. Collection fields stay
    stable for clients, but hidden provider `reasoning_content`, raw
    `pulse_*` / `patrol_*` tool-call prose, DSML/XML/function-call envelopes,
    JSON tool-call text, token accounting strings, and provider thinking text
    must not serialize into API history. Runtime history may retain provider
    reasoning for model continuity, but restored drawer state must expose only
    visible assistant prose plus governed structured tool evidence. Persisted
    tool-result transport messages must be folded into the assistant
    `tool_calls` entries that own them, exposing completed `output` and
    `success` state while keeping internal `tool_result` records out of the
    browser transcript. Assistant message history must also preserve the
    effective `model` route that produced the answer, including local runtime
    routes such as `pulse:local-inventory`. Frontend API types must accept
    restored `tool_calls[].input` from backend history as a structured object
    as well as legacy/display strings, then normalize it before rendering.
    Chat stream events are generated from `internal/ai/chat` payload structs
    into `frontend-modern/src/api/generated/aiChatEvents.ts`; agent capability
    manifest types are generated from `internal/agentcapabilities` payload
    structs into `frontend-modern/src/api/generated/agentCapabilities.ts`.
    Those generated projections must stay derived through
    `scripts/generate-types.go`, and the chat event union must not include the
    retired `explore_status` pre-pass event. Runtime
    workflow telemetry may remain a transport event, but browser Assistant
    streams must use `chat.StreamEvent.ClientSafe()` so raw provider
    `thinking` payloads and serialized tool-call prose cannot cross the API
    boundary. Browser Assistant output is owned by the AI runtime presentation
    contract rather than by API-client event typing.
    Tool execution progress is part of this SSE contract: `tool_start`,
    `tool_progress`, `tool_cancel`, and `tool_end` must carry typed payloads
    generated from `internal/ai/chat` structs so the frontend can mutate one
    visible tool row from pending to running/waiting to completed/error, or
    cancel a runtime-policy-hidden pending row, without inventing a
    browser-only event shape. The shared frontend SSE consumer in
    `frontend-modern/src/api/streaming.ts` may insert an
    animation-frame-backed paint checkpoint with a bounded timer fallback after
    opted-in Assistant progress/tool/status events, including events that arrive
    through separate queued stream reads rather than the same decoded HTTP
    chunk. OpenCode parity is source-backed by fetched `origin/dev` commit
    `9ed17da55ab1f7360cc0e01075f763e27fa899e9`,
    `packages/opencode/src/cli/cmd/tui/context/sync-v2.tsx`, where
    `apply(event)` mutates typed message parts event-by-event (`apply(event)`,
    line 120; `session.next.tool.progress`, line 317). Pulse chat text and
    reasoning deltas must therefore get an immediate first-delta paint checkpoint
    and bounded periodic checkpoints for long coalesced `content` / `thinking`
    runs, without yielding after every token. That pacing is an API-client
    consumption rule only: it must not change event payload shape, event order,
    timeout handling, reader cleanup, parse-error handling, or ordinary
    content-token throughput.
    Cold Assistant streams must include a typed `session` event backed by
    `internal/ai/chat.SessionData` as soon as the HTTP SSE writer is ready and
    an immediate neutral `workflow_state` preparation event before backend
    handoff recovery, model resolution, selected-route retry planning, context
    prefetch, inventory reads, or user-visible provider output so the frontend
    can bind the backend-created session and show live progress without a
    separate create-session request. That cold-stream model resolution must use
    explicit configured chat routes or stable provider defaults without waiting
    on provider model catalogs; `/api/ai/models` and settings responses own
    catalog-backed recommendation, not the first-response chat stream.
    The same SSE transport must emit neutral `workflow_state` events with
    phase `stream_idle` during governed silent intervals while request-bound
    Assistant execution is still in flight. Hidden SSE comment heartbeats are
    not sufficient user-visible progress. The handler must serialize comment
    heartbeats and JSON event writes through one writer lock, and the
    `stream_idle` event must stop before terminal `done`/`error` rather than
    becoming assistant-authored transcript content.
    The generated frontend union, stream parser
    tests, and backend JSON snapshot proof must stay in lockstep with that payload;
    `done.session_id` and `question.session_id` remain compatibility payloads,
    not the primary cold-session creation contract. Assistant `done` events
    must also carry the effective `model` route that completed the stream so
    provider fallback, transcript labels, retries, and model-route recovery stay
    tied to the actual responding provider rather than the failed request route.
    Mock-mode Assistant chat is part of that same API contract: when mock mode
    is active, `internal/api/ai_handler.go` must start, restart, sync, and
    tenant-initialize the chat runtime from an in-memory effective config that
    enables Assistant without mutating the persisted `AIConfig`. `/api/ai/chat`
    must therefore accept cold Assistant requests in mock mode even when no real
    provider or model is configured, and it must return the normal typed SSE
    sequence rather than the service-not-running recovery error. The mock
    fixture must pace its browser-facing status/tool/content events enough to
    prove the visible stream contract, while tests may disable that pace to keep
    backend proof fast.
    The frontend API client may also own an explicit local dev/test Assistant
    stream fixture at `frontend-modern/src/api/aiChatDevStreamFixture.ts` for
    fast UX iteration against the real stream reducer. That fixture may
    short-circuit `frontend-modern/src/api/aiChat.ts` only in Vite dev or test
    mode, only for reserved `/fixture ...` prompts, and only by calling the
    normal `AIChatAPI.chat` event callback with the generated
    `AIChatStreamEvent` union (`session`, `workflow_state`, `thinking`,
    `tool_start`, `tool_progress`, `tool_cancel`, `tool_end`, `content`,
    `done`). It must not open a backend session, mutate persisted chat history,
    add browser-only stream event shapes, or become a production fallback for
    provider or VPN failures. At least one fixture-backed Assistant tool
    sequence must exercise the OpenCode-parity raw tool-input path by starting
    with incomplete provider-style `raw_input`, then mutating the same tool row
    with completed arguments through the normal `tool_progress` event.
    At least one fixture-backed Assistant tool sequence must exercise the
    skipped-tool lifecycle by resolving a visible `tool_start` row through
    `tool_cancel` instead of hiding the activity. At least one fixture-backed
    Assistant tool sequence must exercise live pending/running tool evidence by
    emitting placeholder structured input plus raw provider-style input on
    `tool_start`, mutating that same row through `tool_progress`, and pacing the
    running state long enough for browser proof to inspect and copy both the
    completed input and current progress before `tool_end`. At least one
    fixture-backed Assistant tool sequence must exercise successful long
    plain-text tool output by emitting enough output lines to prove the browser
    drawer renders a bounded collapsed preview, avoids an opaque output badge,
    and preserves the full output in expandable details. At least one
    fixture-backed Assistant tool sequence must exercise compacted provider artifact
    suppression by emitting compacted pre-tool content before a governed tool row
    and final normal answer content. At least one fixture-backed Assistant tool
    sequence must exercise a buffered `tool_start` -> `tool_end` burst with no
    provider delay between the two events, followed by a paced content/terminal
    step, so browser proof can verify that fast command activity remains
    perceptible without a real provider request. At least one fixture-backed
    Assistant tool sequence must exercise consecutive multi-tool activity with a
    paced completed first row followed by a live second row, so browser proof can
    verify replacement/compaction motion without provider timing. At least one
    fixture-backed Assistant sequence must exercise `provider_retry` with `attempt`,
    `max_attempts`, and `retry_after_ms` metadata plus a paced retry window, so
    browser proof can verify visible retry countdown behavior without a real
    provider request, VPN dependency, or API spend. At least one fixture-backed
    Assistant sequence must exercise `stream_idle` after selected-provider
    startup, so browser proof can verify visible idle liveness without forcing a
    real provider to pause on demand. At least one fixture-backed Assistant
    sequence must exercise the local prompt-send state by emitting `session`,
    then pacing the first backend `workflow_state` long enough for browser proof
    to verify immediate visible activity without opening a provider request.
64. `internal/api/ai_handlers.go` shared with `ai-runtime`: AI settings and remediation handlers are both an AI runtime control surface and a canonical API payload contract boundary.
    The AI settings payload on `/api/settings/ai` carries no cloud-context-privacy
    field: cloud context behavior is a fixed posture (real context to cloud, with
    credentials and local-only resources always withheld), not a settings-payload
    knob, so neither a `cloud_context_privacy` dial nor a
    `share_operational_context_with_cloud` boolean is part of the request or
    response contract.
    Legacy Assistant SSE routes in this handler that still use the older
    execute envelope, including `/api/ai/execute/stream` and
    `/api/ai/investigate-alert`, must preserve their existing top-level
    `complete` payload shape while sharing the governed Assistant transport
    liveness contract: visible neutral `workflow_state` events with phase
    `stream_idle` during silent intervals, hidden SSE comments only as
    keepalives, serialized heartbeat/event writes, and no idle progress after
    terminal `complete`/`error`/`done`.
    Provider test responses from `/api/ai/test` and provider-specific
    `/api/ai/test/{provider}` preflight responses must return one safe
    structured diagnostic envelope: `success`, `message`, optional `model`,
    `cause`, `summary`, `recommendation`, and `action`, plus `provider` on the
    provider-specific endpoint.
    The provider-specific endpoint may accept an optional JSON request body
    with `model` so Assistant can test the exact selected chat model; when that
    field carries an explicit provider prefix it must match the route provider
    instead of silently testing a different provider's model.
    Failure payloads must use the AI runtime's Patrol failure-cause vocabulary
    and safe remediation text instead of returning raw upstream provider errors.
    General `/api/ai/test*` payloads use neutral provider diagnostic copy for
    Assistant and settings readiness; Patrol-specific wording is reserved for
    Patrol preflight, Patrol findings, and Patrol run records. Raw details stay
    available only to server logs or redacted governed internal Patrol
    evidence. The frontend API client, settings shell, and Assistant drawer
    must treat this payload as the canonical provider health contract rather
    than parsing free-form provider error strings.
65. `internal/api/ai_intelligence_handlers.go` shared with `ai-runtime`: AI intelligence handlers are both an AI runtime control surface and a canonical API payload contract boundary.
66. `internal/api/config_setup_handlers.go` shared with `agent-lifecycle`: auto-register and setup handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
    That same shared boundary also owns reachable-host selection truth for canonical Proxmox registration: runtime callers may propose ordered `candidateHosts`, but the API contract must persist and echo the first candidate Pulse can actually reach instead of freezing the caller's rejected first preference into the stored node endpoint.
    That same canonical payload contract also owns strict-TLS truth for that selected host: `/api/auto-register` may only persist `VerifySSL=true` when Pulse actually captured a certificate fingerprint for the selected candidate, and it must not pretend public-CA verification is safe after every candidate fingerprint probe failed.
    For PVE cluster sources, that same contract must distinguish primary
    configured endpoints from covered cluster-member endpoints. A caller whose
    candidate hosts match a non-primary `clusterEndpoints` entry is already
    registered and must not rotate the shared Proxmox token, even if the
    primary connection is currently disconnected. A caller whose candidates
    match the primary configured endpoint may still drive disconnected-source
    repair.
    That same contract now owns stale-marker verification as well: setup-token-authenticated `checkRegistration` requests may omit token completion fields and must answer `{registered:boolean}` from canonical candidate-host matching so runtime repair can distinguish real registrations from stale local marker files without rotating tokens first.
    That same contract owns auto-register WebSocket event intent: a successful
    completion that creates a new PVE/PBS node may broadcast
    `node_auto_registered`, but successful idempotent matches or credential
    refreshes for an already configured node must use a non-toast config-change
    event such as `nodes_changed` instead of re-announcing first registration.
    The event payload must carry the canonical `source` field so browser
    consumers can distinguish script-initiated operator handoffs from
    background agent lifecycle churn.
    That same shared setup contract also owns teardown symmetry for script-managed Proxmox nodes: `/api/auto-unregister` must accept the canonical `type`, normalized `host`, explicit `serverName`, optional canonical `tokenId`, request-body `authToken`, and `source:"script"` payload, and it must answer the same canonical success envelope on both real removals and idempotent no-op reruns so browser/runtime callers do not invent a second uninstall vocabulary.
    That same setup-script payload contract owns the Proxmox permission shape
    for generated scripts, runtime-side host-agent setup, and installer
    auto-registration. PVE setup paths must create
    `pulse-monitor@pve!pulse-*` tokens with privilege separation enabled, then
    apply `PVEAuditor`, optional `PulseMonitor`, and optional `/storage`
    `PVEDatastoreAdmin` ACLs to both `pulse-monitor@pve` and the concrete token
    id. PBS scripts must grant `Audit` to both `pulse-monitor@pbs` and the
    concrete token id. Browser/runtime setup callers must not fork this into
    token-shared, token-unprivileged, or user-only ACL variants.
67. `internal/api/enterprise_extension_rbac_admin.go` shared with `organization-settings`: RBAC admin extension endpoints are both an organization settings control surface and a canonical API payload contract boundary.
68. `internal/api/licensing_bridge.go` shared with `cloud-paid`: commercial licensing bridge handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
69. `internal/api/licensing_handlers.go` shared with `cloud-paid`: commercial licensing handlers carry both API payload contract and cloud-paid entitlement boundary ownership.
    That same shared licensing boundary also owns authenticated
    install-version and runtime-build attribution: `internal/api/router.go`
    must hand the canonical process `serverVersion` and normalized runtime
    identity into `internal/api/licensing_handlers.go`, and the shared
    licensing runtime must carry those exact values through `/v1/activate`,
    `/v1/licenses/exchange`, and `/v1/grants/refresh` so migrated installs can
    be attributed to exact builds and public-vs-Pro runtime status without
    inventing a second version source or trusting browser-supplied hints.
    That same shared licensing boundary also owns self-hosted purchase-return
    framing. `/auth/license-purchase-start` and
    `/auth/license-purchase-activate` may return operators only to the
    canonical self-hosted settings route at
    `/settings/system/billing/plan`, and the bridge pages must describe that
    surface as a plan-owned destination (`Plans`, `Plan activated`,
    `Finalizing plan upgrade`) rather than as a tier-owned `Pulse Pro billing`
    page. Frontend callers may still render the unlocked tier name inside that
    destination, but the browser/API contract must not reintroduce
    Pulse-Pro-as-page-name copy in callback titles, actions, or retry
    guidance.
70. `internal/api/licensing_legacy_retry.go` shared with `cloud-paid`: the background legacy-exchange retry loop carries both API payload contract and cloud-paid entitlement boundary ownership.
71. `internal/api/notifications.go` shared with `notifications`: notification handlers are both a notification delivery control surface and a canonical API payload contract boundary.
72. `internal/api/org_handlers.go` shared with `organization-settings`: organization management handlers are both an organization settings control surface and a canonical API payload contract boundary.
73. `internal/api/org_lifecycle_handlers.go` shared with `organization-settings`: organization lifecycle handlers are both an organization settings control surface and a canonical API payload contract boundary.
74. `internal/api/payments_webhook_handlers.go` shared with `cloud-paid`: commercial payment webhook handlers carry both API payload contract and cloud-paid billing boundary ownership.
75. `internal/api/public_signup_handlers.go` shared with `cloud-paid`: hosted signup handlers carry both API payload contract and cloud-paid hosted provisioning boundary ownership.
    That same shared boundary also owns public hosted-signup response privacy:
    syntactically valid `/api/public/signup` requests must return one generic
    `202 Accepted` Pulse Account message whether provisioning/email side effects
    ran or were suppressed by the owner-email limiter, while invalid bodies and
    true server failures remain explicit.
76. `internal/api/relay_mobile_capability.go` shared with `relay-runtime`: the backend-owned Pulse Mobile relay capability inventory is both a relay runtime boundary and a canonical API payload contract surface.
77. `internal/api/resources.go` shared with `unified-resources`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.
78. `internal/api/security.go` shared with `security-privacy`: the security handlers are both a security/privacy control surface and a canonical API payload contract boundary.
    That same shared security/API boundary owns CSRF replacement-token
    concurrency. When parallel browser mutations arrive with stale or missing
    CSRF tokens for the same session, `internal/api/csrf_store.go` may retain
    a bounded set of recent unexpired token hashes so each server-issued
    replacement can validate its retry. Logout, password-change, and explicit
    session revocation must still delete the full session token set rather than
    leaving any retained replacement token valid.
79. `internal/api/security_tokens.go` shared with `security-privacy`: the security token handlers are both a security/privacy control surface and a canonical API payload contract boundary.
    Token owner identity is reserved for the server-authenticated principal:
    shared token-minting helpers must derive `owner_user_id` from the current
    session or caller token and reject extension metadata that tries to
    overwrite that field. API-owned token minting paths that bypass
    `/api/security/tokens`, including agent install, deploy bootstrap/enroll,
    container runtime migration, quick security setup, and API-token
    regeneration must still call the shared owner setter and must not encode
    owner identity inside their extension metadata maps.
    The dedicated Pulse Mobile relay token route is part of that same API
    contract even though its runtime capability is Relay-owned:
    `POST /api/security/tokens/relay-mobile` must pass normal admin and
    `settings:write` authorization, then require the paid `relay` feature
    before minting a `relay:mobile:access` credential. Community installs may
    receive the standard license-required response, but direct API calls must
    not bypass Relay entitlement by creating mobile runtime tokens.
80. `internal/api/setup_script_render.go` shared with `agent-lifecycle`, `storage-recovery`: the generated Proxmox setup-script is a shared boundary across agent lifecycle (forced-command keys, install/uninstall edits), API contracts (rendered token shape and encoded rerun URL), and storage/recovery (backup visibility grants, Pulse-managed temperature SSH keys, and SMART disk-temperature collection).
81. `internal/api/slo.go` shared with `performance-and-scalability`: the SLO endpoint is both an API contract surface and a protected performance hot-path boundary.
82. `internal/api/system_settings.go` shared with `security-privacy`: the system settings telemetry and auth controls are both a security/privacy control surface and a canonical API payload contract boundary.
83. `internal/api/unified_agent.go` shared with `agent-lifecycle`: unified agent download and installer handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
    Development-mode missing-binary responses must report the build command
    for the requested normalized OS/architecture, not a hard-coded Linux
    target, so installer preflight failures point operators at the artifact
    they actually need.
84. `internal/api/updates.go` shared with `deployment-installability`: update handlers are both a deployment-installability control surface and a canonical API payload contract boundary.
85. `pkg/aicontracts/fix_execution.go` shared with `ai-runtime`: the public approved-fix execution contract is both an AI runtime approved-action boundary and a canonical API dependency contract for Patrol and enterprise auto-fix binders.
86. `pkg/aicontracts/investigation.go` shared with `ai-runtime`: the public Patrol investigation record and finding contract is both an AI runtime handoff boundary and a canonical API payload contract for Patrol, Assistant, unified findings, persistence, and audit surfaces.
87. `pkg/aicontracts/orchestrator_deps.go` shared with `ai-runtime`: the public investigation orchestrator dependency contract is both an AI runtime handoff boundary and a canonical API payload contract for Assistant and Patrol tool-call history.
88. `pkg/extensions/ai_autofix.go` shared with `ai-runtime`: the enterprise auto-fix extension dependency seam is both an AI runtime approved-action boundary and a canonical API extension contract over Assistant and Patrol execution dependencies.
89. `scripts/generate-pulse-intelligence-docs.go` shared with `ai-runtime`: the Pulse Intelligence manifest docs generator is both an AI runtime docs/onboarding projection and a canonical API contract projection over the agent capabilities manifest and Pulse MCP surface tool contract.
    Update-plan responses own the structured readiness verdict for server
    updater capability, rollback support, agent continuity, v5 agent migration
    transport security, and agent reporting token scope. That verdict is part
    of the update-plan API contract, not a settings-only migration registry.
    When v5 or legacy agents are present, readiness must preserve the
    `agent-migration-security` warning that automatic first-hop migration
    depends on HTTPS or trusted local-network transport, with signed-installer
    reinstall as the high-assurance path. `POST /api/updates/apply` must derive
    the requested target version through the shared update-target validation
    path, recompute readiness from live backend state, and reject `blocked`
    verdicts before update execution starts.
    The platform-connections API contract also owns inactive monitored-system
    candidate semantics end to end. `enabled=false` on TrueNAS or VMware preview,
    test, add, and update payloads must serialize through the shared ledger client
    as `active:false`, and preview responses may legitimately return `no_change`,
    `removes_existing`, or `removes_multiple` with empty projected-system lists
    when the disabled candidate no longer contributes to a monitored-system group.
    That same monitored-system grouping contract now also owns restart-safe host
    report continuity at the API boundary. The removed monitored-system limit
    enforcement path must not return; the API should treat a returning standalone
    host report as existing grouping context when monitoring can match it to recent
    persisted host continuity, so a server restart or v6 upgrade does not change
    the explanatory grouping model before the live
    inventory rebuild catches up. Genuinely new host identities must still return
    the canonical monitored-system blocked payload.

## Extension Points

0. Parallel platform/resource handler families share their flow logic instead
   of per-platform copies. `internal/api/platform_connection_shared.go` owns
   the platform connection update flow (locate by trimmed ID, decode with
   stored-record fallback via the generic `decodeOptionalInstanceRequest`,
   normalize, preserve masked secrets, validate, persist, respond redacted —
   `updatePlatformConnection` + `platformConnectionUpdateSpec`) and the
   admin+settings:write item-route mux builder
   (`platformConnectionItemRoute`); a new platform connection surface wires a
   spec instead of re-rolling the flow, so masked-secret preservation cannot
   be forgotten. `internal/api/metadata_handlers_shared.go` owns the
   guest/docker metadata GET/PUT payload semantics (empty object instead of
   null, zero record instead of 404 — pinned by
   `TestContract_MetadataGetPayloadsUseZeroRecordsInsteadOf404`). Deploy
   preflights and jobs share `handleDeployJobStatus` /
   `handleDeployJobEvents` in `internal/api/deploy_handlers.go`; recovery
   points series/facets share `parseRecoveryListPointsOptions`; docker and
   kubernetes agent lifecycle PUTs share per-handler
   `handleHostLifecycleAction` / `handleClusterLifecycleAction`; PBS/PMG node
   connection probes share `testProxmoxPlatformConnection`; secrets-backed
   sqlite stores (handoff JTI, purchase-return redemptions) open through
   `openHardenedSecretsDB` so the private-permission hardening policy stays
   single-sourced; privileged settings endpoints gate through
   `serveSetupTokenOrSettingsWrite` in `internal/api/router.go`; patrol
   findings convert to the unified store through one `unifiedFindingFromAI`.
1. Add or change payload fields through handler + contract tests together
2. Update frontend API types in lockstep with backend contract changes.
   Websocket-backed API consumers such as `frontend-modern/src/components/Settings/useAPITokenManagerState.ts` and `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx` may read runtime context only through `frontend-modern/src/contexts/appRuntime.ts`; they must not import `frontend-modern/src/App.tsx`, because payload ownership remains in the API contract rather than the root shell.
   2a. Route settings infrastructure connected-system ledgers through
   `/api/connections` and `frontend-modern/src/components/Settings/useConnectionsLedger.ts`
   together. The frontend ledger may retain the last fulfilled connection
   snapshot while polling or manual reload is in flight, but that retention is
   only a fetch lifecycle rule; it must not synthesize rows, downgrade backend
   fleet state, or replace the shared connection projection with page-local
   placeholders.
   2b. Route agentless availability target kind changes through
   `internal/api/availability_handlers.go`,
   `internal/api/platform_mock_connections.go`,
   `frontend-modern/src/api/availabilityTargets.ts`, and the unified-resource
   availability payload together. The bounded `targetKind` vocabulary is
   `machine`, `service`, and `device`; missing legacy values default to
   `service`. Browser add-dialog routes may carry a `targetKind` query value
   to preselect that bounded kind, but the persisted contract remains the
   `AvailabilityTarget.targetKind` payload. Browser consumers must not place
   availability targets into Standalone Machines from that classification;
   Machines membership requires Pulse Agent resource evidence. API clients
   must not infer that classification from protocol, hostname, port, or row
   label.
3. Add dedicated contract tests for new stable payloads
   Unified resource type-filter and organization-share resource type additions
   must route through `internal/api/resources.go`, `internal/api/org_handlers.go`,
   frontend resource typing, and `internal/api/contract_test.go` together.
   Native provider projections such as TrueNAS `network-share` may be accepted
   by `/api/resources` filters and cross-organization share normalization only
   after the unified-resource contract defines the canonical resource type and
   payload facet.
   3a. Route diagnostics payload fields and user-facing diagnostics copy through
   `internal/api/diagnostics.go`,
   `internal/api/diagnostics_contract_test.go`,
   `internal/api/diagnostics_additional_test.go`, and
   `internal/api/diagnostics_memory_test.go` together. Docker and Podman
   health notes emitted by diagnostics must lead with Docker / Podman module
   language and route operator recovery to the Infrastructure and
   Security settings surfaces rather than generic runtime family wording or
   retired agent-management destinations. Pulse Assistant diagnostics must
   expose native runtime availability as `assistantRuntimeConnected`; the
   diagnostics payload must not revive legacy MCP transport fields such as
   `mcpConnected` or `mcpToolCount` for the first-party Assistant surface.
   3b. Route Docker / Podman management API response copy through
   `internal/api/docker_agents.go`, `internal/api/docker_metadata.go`,
   `frontend-modern/src/api/monitoring.ts`, and their route/client tests
   together. Operator-facing responses for Docker / Podman host removal,
   hide/unhide, pending uninstall, display-name, and host metadata paths must
   use Docker / Podman module or host wording instead of generic container
   runtime labels.
   3c. Route Assistant finding handoff context changes through
   `internal/api/ai_handler.go`, `internal/api/ai_handler_test.go`, and
   `internal/api/contract_test.go` together. Patrol-originated handoffs must
   keep `[Finding Briefing]`, `[Finding Context]`, `[Finding Lifecycle
    Context]`, structured handoff resources, related root-cause/correlation
   finding context, and structured handoff actions model-only, with the
   briefing summarizing latest lifecycle event and factual governed action
   artifact metadata without raw command text or Pulse-authored next-step
   guidance. Structured handoff action
   references may use the current live Patrol investigation-fix approval for
   the finding when that approval is newer than the approval ID on the durable
   record, but the payload may carry only IDs, status/risk/target metadata,
   request/expiry timestamps, action plan identity and expiry, safe generated
   approval summaries, command counts, and fix/action references, never the
   approval command payload. Patrol
   remediation-plan handoffs must use the same boundary for model-only
   context: plan status, risk, step labels, and command counts are allowed,
   while raw command and rollback command payloads remain in governed action
   surfaces. Frontend Patrol finding-discussion handoffs must force a
   request-local approval-required Assistant mode instead of inheriting the
   user's persistent autonomous control setting; live approval, action artifact,
   fix-outcome, and remediation-plan references only add structured action
   metadata, they are not the trigger for the boundary. Frontend-visible Patrol
   briefing payloads must stay compact and must not include suggested prompt
   chips, Patrol-authored next-step recommendations, or route-owned
   recommendation metadata. Frontend queued-fix recovery handoffs
   where the live approval or action artifact payload is unavailable must still
   carry that Patrol-owned finding briefing, current `fix_queued` posture,
   request-local approval-required mode, and model-only evidence context; they must
   not degrade into generic Assistant investigation chat or imply that
   execution can proceed from missing command payloads. Expired-approval
   recovery handoffs may use a still-available structured action artifact payload
   only as safe metadata: description, target, risk, rationale, destructive
   posture, and command count may enter the briefing, while raw command text
   remains owned by governed remediation or approval surfaces. If the unified
   finding list lacks a full investigation record, frontend finding-discussion
   handoffs may hydrate the latest investigation session for the same safe
   action artifact metadata, but they must not paste raw action command text
   into the authored prompt or visible briefing. Direct
   alert-investigation API handoffs through `internal/api/ai_handlers.go` must
   enforce that same request-scoped boundary by setting
   `ai.ExecuteRequest.AutonomousMode` to
   false and `ai.ExecuteRequest.RequireCommandApproval` to true; API proof must
   keep this guarded in both `internal/api/ai_handlers_test.go` and
   `internal/api/contract_test.go`. Governed action artifact lines in the
   briefing must derive from those
   same structured action references after recovery so the briefing cannot
   contradict the handoff action payload. Related finding
   context must resolve from the current unified finding store, stay bounded
   and deduplicated, include current recency and latest lifecycle facts, and
   seed only structured handoff resources for canonical policy, state,
   topology, and timeline hydration. Chat execution owns
   resource-policy sanitization of the assembled model-only handoff before
   prompt injection, so API payload builders may pass structured product
   context without turning raw resource identity into user-authored text or
   disclosure authority.
   3d. Route command-agent WebSocket registration token semantics through
   `internal/api/agent_exec_token_binding.go`, `internal/api/router.go`, and
   `internal/api/contract_test.go` together. `agent:exec` tokens must already
   be bound to the registering agent ID or hostname before `/api/agent/ws`
   accepts command registration. The only first-use exception is a
   Pulse-minted PVE/PBS install-command token carrying the governed install
   metadata; that token may bind once to the first command agent ID and
   hostname that registers, and a later different agent or hostname must be
   rejected. Generic unbound `agent:exec` tokens remain fail-closed.
4. Route unified resource sensitivity, routing, and `aiSafeSummary` payload changes through `internal/api/resources.go`, `internal/api/contract_test.go`, and the canonical frontend resource consumer proofs together; resource governance metadata must not ship as an API-only or frontend-only heuristic
   That same resource payload contract owns `aggregations.policyPosture` on
   `/api/resources` and `/api/resources/stats`. The aggregation must be derived
   from canonical unified-resource policy metadata, normalized as camelCase
   resource API JSON, and exercised with backend contract tests plus the
   canonical `useUnifiedResources` frontend hook proof whenever it changes.
5. Route unified-resource action, lifecycle, and export audit reads through `internal/api/activity_audit_handlers.go`, `internal/api/router_routes_licensing.go`, and `internal/api/contract_test.go` together so the control-plane execution trail stays on a governed API contract instead of a store-only shape
   Enterprise audit-log reads are part of that same API boundary. Audit list
   and verification handlers must preserve structured storage failure
   semantics from `pkg/audit`: transient store pressure returns `503`
   `audit_store_busy` with `Retry-After`, unavailable or corrupt audit storage
   returns `503` `audit_store_unavailable`, and unrelated query failures remain
   `query_failed`. List pagination must stay bounded so audit history reads
   cannot become unbounded table scans through API parameters.
   Plan-only unified action planning is part of that same API-first action
   contract: `POST /api/actions/plan` must route through
   `internal/api/actions.go`, `internal/actionplanner/planner.go`, and
   `internal/api/contract_test.go` together, returning deterministic
   `ActionPlan` identity, approval policy, blast radius, resource/policy
   versions, plan hash, and preflight checks without approving or executing the
   capability. A successful plan must also be durably recorded in the unified
   action-audit store with initial lifecycle evidence before the API returns:
   approval-required plans land as `pending_approval`, retries with the same
   deterministic action identity may upsert the audit record, and retries must
   not duplicate the initial lifecycle events. MCP, CLI, and UI consumers may
   adapt this payload, but they must not become the source of truth for action
   planning semantics.
   Executor-owned live readiness is part of planning, not a UI precheck:
   after planner validation and before audit persistence, `actions.go` must ask
   the registered executor whether the resource/capability is currently
   executable. A failed readiness check returns `409`
   `action_execution_unavailable`, does not create or mutate an action audit
   record, and must not dispatch commands. Docker / Podman lifecycle readiness
   uses that hook for command-agent connectivity and runtime posture; browser
   controls may consume advertised capabilities, but must not replace this
   check with direct shell, SSH, provider, or agent-command calls.
   Docker / Podman lifecycle execution resolves the command WebSocket by the
   Docker reporting agent ID first, then by canonical Docker host name when the
   runtime source ID differs from the command-agent registration ID. Once
   `/api/actions/{id}/execute` has entered the API-owned execution contract,
   the executor may dispatch the vetted container lifecycle and verification
   commands as trusted agent commands; the host-agent hard-block policy still
   applies, and the action audit remains the approval and lifecycle source of
   truth.
   Proxmox QEMU VM and LXC lifecycle execution is part of that same API-owned
   action contract. Proxmox guest resources may expose only conservative
   lifecycle operations (`start`, `shutdown`, `reboot`, `stop`) and execution
   must route through the registered action executor, not through platform
   tables, Assistant tool shortcuts, direct Proxmox API mutation, SSH, or
   guest-local agents. The executor must resolve a connected Proxmox node
   command agent from the unified resource's linked node agent first, then the
   canonical Proxmox node hostname, dispatch only vetted `qm` / `pct`
   lifecycle commands as trusted action-owned agent commands after the API
   action enters execution, and verify the resulting Proxmox state with
   `qm status` / `pct status` before recording success. Templates, locked
   guests, stale Proxmox inventory, incomplete VMID/node metadata, unsupported
   handlers, and disconnected node command agents must fail closed through the
   same readiness and audit model.
   Resource payloads may expose the same executor-owned unavailable state as
   `actionReadiness[]` entries with stable `name`, `available`, `reasonCode`,
   and `reason` fields so browser and agent clients can explain disabled
   actions without treating unavailable capabilities as executable.
   Action approval decisions are API-owned as a separate non-execution
   contract: `POST /api/actions/{id}/decision` may only record an
   `approved` or `rejected` decision against a persisted `pending_approval`
   action, update the canonical action audit, and append lifecycle evidence
   atomically. It must not invoke the capability driver, create an
   `executing`/`completed` event, or attach an execution result; execution
   requires a later explicit execution contract.
   Patrol investigation-fix approvals produced through
   `internal/api/ai_handlers.go` must enter that same API-owned model at queue
   time: the approval adapter must attach a governed `ActionPlan`, persist the
   initial action audit as `planned` then `pending_approval`, and keep the
   approval ID, action ID, requester identity, dry-run/preflight posture, and
   lifecycle trail available for Assistant handoff and resource action-history
   hydration. Queueing a Patrol investigation fix must stamp the action request
   and initial lifecycle events as `pulse_patrol`; it must not execute the fix,
   create a Patrol-local audit record that bypasses `/api/actions`, or present
   Patrol-origin proposals as generic Assistant-origin actions. `/api/ai/approvals`
   must expose the persisted `requestedBy` identity so frontend Assistant
   handoffs can carry Patrol provenance before the chat runtime refreshes the
   current action state from action audit. Backend `/api/ai/chat` refresh of
   a finding handoff must also recover that requester identity from the live
   approval record when action audit has not yet hydrated the current action
   state. Denying a Patrol investigation-fix approval through
   `/api/ai/approvals/{id}/deny` is part of the same API contract: it must
   persist the approval-store denial, record a `rejected` action-audit decision
   when the approval carries a governed action plan, and update the owning
   finding's investigation outcome to `fix_rejected` instead of leaving
   `fix_queued` with no live approval.
   Action execution is API-owned as the next explicit contract:
   `POST /api/actions/{id}/execute` may only start execution for an approved
   action or an approval-free executable plan, must atomically persist the
   `executing` lifecycle state before invoking a registered executor, and must
   atomically persist the terminal `completed` or `failed` result afterward.
   Before the endpoint enters `executing`, it must rebuild the current
   resource/capability plan from the unified registry and compare the stored
   action id, resource version, policy version, and plan hash. A mismatch is a
   refused-before-dispatch `action_plan_drift` failure: the executor is not
   called, the action audit is moved to `failed` with a `plan_drift:` result,
   lifecycle evidence is appended, and the shared `action.completed` SSE bridge
   publishes the terminal failure so agents do not poll a stale plan into
   execution.
   Dry-run-only plans are not executable plans and must fail closed before any
   `executing` mutation. If no API executor is registered, the endpoint must
   fail closed without mutating the approved audit record or appending execution
   lifecycle events.
   Approval must never imply execution, and local UI, CLI, MCP, agent, or
   storage/recovery adapters must not bypass this endpoint with a parallel
   execution transport.
   The supported CLI adapters for this contract are `pulse actions plan`,
   `pulse actions decide`, and `pulse actions execute`, owned by
   `pkg/pulsecli/actions.go` and registered from `pkg/pulsecli/root.go`. They
   must remain thin authenticated clients for `POST /api/actions/plan`,
   `POST /api/actions/{id}/decision`, and
   `POST /api/actions/{id}/execute` rather than importing planner internals,
   creating action IDs, or executing capabilities locally.
   `pulse actions capabilities` may read
   the canonical `GET /api/resources/{id}/facets` payload to discover resource
   capability names and parameter schemas before planning, but it must not
   invent a parallel capability inventory or expose internal handler names.
   `pulse actions audit` and `pulse actions events` may read
   `GET /api/audit/actions` and `GET /api/audit/actions/{id}/events` as
   verification adapters, but they must remain read-only views of the canonical
   action audit and lifecycle trail rather than a second audit store.
6. Route dedicated unified-resource timeline and facet-bundle reads through `frontend-modern/src/api/resources.ts`, `internal/api/resources.go`, and `internal/api/contract_test.go` together so the backend facet contract and the frontend client stay aligned on one timeline-first surface, while capability and relationship detail stays backend-owned for AI correlation and change detection.
   `/api/resources/{id}/timeline` and `/api/resources/{id}/facets` must keep
   resource timelines relationship-aware by opting into the canonical
   `ResourceChangeFilters.IncludeRelated` store path, so a resource timeline
   includes direct changes and changes that name the resource in
   `relatedResources` instead of hiding child or dependency activity from the
   owning resource.
   The same facet bundle must return the selected resource's backend-authored
   `capabilities` and canonical `relationships` alongside recent changes and
   grouped counts, so frontend detail surfaces consume one governed API payload
   instead of rebuilding capability or topology context from the list response.
   Provider-wide timeline reads are part of the same API contract:
   `GET /api/resources/timeline` returns the canonical resource timeline
   response shape with an empty `resourceId`, uses the shared `since`, `limit`,
   `kind`, `sourceType`, and `sourceAdapter` parser, and remains protected by
   the monitoring-read scope. Platform pages may use it for API-authored
   provider activity such as vSphere tasks and events, but they must not create
   page-local activity stores or a second query vocabulary.
7. Route unified-resource list ordering through `internal/api/resources.go`, `internal/api/contract_test.go`, and the owned unified-resource registry helpers together; list payloads must stay deterministic for equal-name resources by carrying one canonical `name -> type -> id` tie-break across cold seed, REST pagination, and websocket-backed refreshes instead of inheriting map order or page-local re-sorts
   That same shared API contract also owns the external resource `type`, canonical display name, and cluster identity published through `/api/resources` and `/api/state`; the websocket/state hydrate path must not emit legacy aliases or raw store labels once the unified resource contract has normalized them.
   Realtime `/api/state` and websocket `resources` snapshots must also collapse
   transient split host identities before broadcast: a Proxmox-node row and a
   host-agent row for the same machine must leave the API boundary as one
   hybrid `agent` resource with merged source facets, not as duplicate rows
   that disappear only after REST reconciliation.
8. Route unified-agent installer and binary download headers through `internal/api/unified_agent.go` and `internal/api/contract_test.go` together. Unified-agent BINARY downloads must keep the canonical `X-Checksum-Sha256` plus `X-Signature-Ed25519` contract for updater clients whether the binary is served locally or proxied from the matching GitHub release, instead of leaving callers to infer trust from source location alone. The served install-script endpoints (GET /install.sh and /install.ps1) are governed differently and have NO GitHub fallback at all: they serve the locally bundled AGENT installer or fail closed with 503. The agent installer is a per-build artifact bundled into every release tarball and Docker image, not a release asset, so the endpoint must never fetch the top-level GitHub install.sh release asset (the SERVER installer, which rejects the agent wizard's --url / --token-file, issue #1470). It attaches the base64-encoded `X-Signature-SSHSIG` header when the local detached signatures are present and omits it when they are not; a present-but-unsigned local agent installer is still served, because the agent install path (curl piped into bash) does not verify these headers, so correctness of the served script outranks signature presence.
9. Route canonical AI intelligence summary and resource-intelligence reads through `frontend-modern/src/api/ai.ts`, `frontend-modern/src/stores/aiIntelligence.ts`, `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts`, `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`, `frontend-modern/src/features/patrol/PatrolIntelligenceSurface.tsx`, the Patrol-owned section files under `frontend-modern/src/features/patrol/`, `frontend-modern/src/pages/AIIntelligence.tsx`, `internal/api/ai_handlers.go`, and `internal/api/contract_test.go` together so the store normalization owner, runtime hook, feature shell, Current work workspace, section owners, route shell, and backend payload stay aligned on one governed surface, including the canonical recent-changes slice
   while keeping the learning counters backend-only coverage, so Patrol keeps health and findings primary and renders timeline, correlation, and policy-posture data as selected-item investigation context rather than as a separate headline product metric
   and that secondary investigation context remains explanatory API evidence,
   not a default workspace mode: the Patrol page may expand recent-change,
   correlation, and policy-posture details only for an active Patrol finding or
   an explicitly selected run record, and must not surface those backend
   context payloads as a page-level forensic block from degraded summary health
   alone
   and the Patrol status presentation boundary, so trigger/scheduling status
   from Patrol status APIs stays in the header/control surface instead of being
   repeated as default status chrome, watch-only presentation says Patrol watches
   infrastructure and shows current issues while the selected Watch only mode
   describes the capability positively, such as Patrol watching infrastructure
   and reporting issues only, avoiding repeated header/control
   sentences, `Will not` policy-list wording, secondary Limits disclosures, or repeated
   infrastructure-unchanged caveats, and the browser-visible label stays
   operator-readable even when internal Assistant handoff metadata still uses
   assessment terminology
   and the Patrol findings empty-state behavior, so `0 active findings` only renders as a healthy frontend conclusion when the same governed AI summary contract still reports healthy overall health; degraded or not-fully-verified health predictions must flow through to the Patrol findings surface instead of being replaced by page-local "looks healthy" copy
   and the Patrol control presentation boundary, so the always-visible Patrol
   control selector owns the selected autonomy level, plan-locked installs keep
   watch-only as the current capability while disabled paid-level buttons may
   stay visible. Compact Pro badges and a `Plans & Billing` action may appear
   only when upgrade prompts are allowed; that billing action stays a compact
   handoff beside the selector rather than a separate upsell panel, and the
   selector must still avoid Limits controls, hard-limit matrices, or
   absent-feature explainers.
   Compact control labels remain understandable decisions such as `Ask first`,
   `Safe auto-fix`, and
   `Autopilot` rather than transport shorthand or Pro-matrix labels, the Patrol
   header description derives from the same
   effective control state, and the advanced settings
   drawer stays limited to model, schedule, trigger, and user-level model checks
   sourced from the existing settings APIs rather than duplicating a second
   control-level chooser. Runtime or provider setup
   readiness may suppress run, schedule, model, trigger, and provider-repair
   controls until Patrol can check infrastructure reliably, but it must not hide
   the Patrol mode selector or replace it with setup-first proof/status chrome.
   When provider/model readiness blocks manual Patrol while the header still
   shows current infrastructure work, the primary run control projects that API
   state as a direct Provider & Models setup action instead of an inert disabled
   `Run Patrol` button
   and the first-party Patrol mode starter boundary, so successful direct
   autonomy saves may record the coarse content-free `patrol_control` workflow
   prompt marker only when the effective control level or full-control
   acknowledgement changes and the paid Patrol control feature is available,
   while the autonomy settings API remains the source of truth for persisted
   control state rather than carrying prompt content, finding context, or
   completed-work proof; after recording that marker, Patrol state must reload
   Patrol status, findings, approvals, and run history so first-party control
   changes become visible as the next current-work step without waiting for polling
   and the Patrol trust-history evidence carried by Patrol status, so historical
   regressions keep the current findings empty state out of green all-clear copy
   even when the active finding count is zero and the latest summary score is
   otherwise healthy
   and the Patrol Current work assessment behavior, so the same governed AI summary contract decides whether the workspace leads with verified health, issues detected, coverage incomplete, or another attention state instead of letting count-only page fragments emit a stale `No issues found` conclusion
   and the Patrol Current work copy boundary, so the API-owned health,
   finding, run, and control facts remain semantic inputs while browser-visible
   empty and descriptive text stays operator-facing: what will appear there,
   what Patrol may do under the selected control level, and what to run or
   review next, not activation-loop proof, queue internals, or verification
   accounting
   and the Patrol control dialog/cross-surface Assistant handoff copy, so API
   compatibility identifiers such as `patrol_configuration_failure` may remain
   stable while user-facing and model-facing labels describe setting, saving,
   and reviewing Patrol control rather than a generic configuration/apply flow
   and the Patrol main-surface treatment itself, so the same governed summary contract lands inside the existing workspace and Current work surfaces while severity travels through compact header accents and icon badges instead of a page-local full-width verdict strip
   and the Patrol workspace badge treatment, so the API-owned finding,
   runtime, and run-history counts remain semantic input only while visible
   state and count badges route through frontend-primitives-owned
   `StatusIndicatorBadge` and `MetadataBadge` instead of page-local class
   strings in the Patrol section owners
   and the Patrol control-level copy, so frontend section owners treat
   `PatrolAutonomyLevel` plus lock state as the API input for current-work and
   findings-workspace descriptions instead of presenting generic investigation
   or fix capabilities when the payload currently resolves to watch-only.
   The same API input owns the visible Patrol control boundary summary: what
   Patrol may do, what must ask for approval or Pro/runtime availability, and
   any explicit hard limits must derive from `PatrolAutonomyLevel` plus lock
   state, not from page-local safety thresholds or operations-loop completion
   proof. The default API-backed presentation must stay on the selected control
   level and its plain summary; it must not render a separate Limits disclosure,
   hard-limit matrix, or always-on explanatory matrix beside the mode picker.
   and the Patrol run-record copy, so `finding_ids` remains the API-owned
   fail-closed scoping input while the frontend presents selected history as a
   Patrol run record instead of a generic findings filter or snapshot workflow
	   and Patrol loading indicator state remains API data only while the visible
	   header and finding loading spinners route through frontend-primitives-owned
	   `LoadingSpinner` instead of encoding API state in page-local spinner shells
	   and Patrol first-load refresh failures remain degraded data state rather
	   than route failure, so frontend Patrol load orchestration keeps the
	   workspace mounted, preserves any last-known Patrol evidence, exposes a
	   retry affordance, and sends transport details only to debug/API diagnostics
	   instead of raw page copy
	   and the Patrol issue-row presentation boundary, so active collapsed Patrol
	   rows use API-owned severity, subject, recency, recurrence, and actionable
	   workflow state as semantic input while suppressing raw `loopState`,
	   investigation-status, investigation-outcome, confidence, and generic
	   review/detected process badges from the default row; those process details
	   remain available in expanded context, selected run records, Assistant
	   handoff context, or API diagnostics when they are actually needed
	   and the Patrol verification summary derived from run history, so the page also states whether recent Patrol evidence came from a successful full patrol or only from scoped/erroring runs instead of leaving verification scope implicit
   and the same-day activity-mix explanation derived from that governed run history, so when a recent full patrol is followed by alert-triggered or anomaly-triggered scoped work the verification surface can explain the mix directly instead of reconstructing it from page-local timing heuristics
   and the Patrol status recency split, so `last_patrol_at` remains reserved for completed full Patrol sweeps while scoped runs and verification checks advance `last_activity_at` without claiming a fresh full-estate verification pass
   and the Patrol Assistant handoff model, so frontend handoff prompts pass
   current finding context, safe action posture, and resource references as
   bounded request metadata while leaving tool selection and remediation
   reasoning to the configured LLM instead of serializing a frontend-authored
   tool route or fix plan into the API request
   and the Patrol control presentation boundary, so frontend copy may summarize
   assisted mode as low or medium-risk automatic fixes allowed by policy, but
   the risk threshold itself remains backend-owned and must not be inferred from
   warning severity, UI labels, or page-local finding state
   and the Patrol control route target, so the frontend-owned
   `/patrol#patrol-control` anchor remains the canonical navigation affordance,
   while `/patrol#operations-loop` remains inbound compatibility only, rather
   than becoming an API payload field, Assistant prompt body, or backend
   completion state machine. The canonical anchor must resolve to the visible
   Patrol mode selector, not to the assessment workspace; that anchor may
   route a new Pro user to Patrol mode from a generic Patrol run state, but issue-backed
   progress through Assistant, governed decision, verification, and MCP parity
   must still derive from real Patrol finding, investigation, approval, action,
   or trust evidence owned by the Patrol state model. The native Patrol
   workspace must not load the authenticated operations-loop status projection
   to decide current work; that projection remains an API/MCP/telemetry and
   adjacent commercial compatibility surface, while local Patrol state exposes
   Patrol work from status, findings, approvals, and run history. The route
   anchor itself must stay out of API payloads and must not become completion
   proof.
   When that projection still returns `nextAction: open_assistant` for a
   current Patrol finding, the first-party Patrol page must treat it as
   compatibility guidance and route the primary action into the Patrol findings
   workflow; Assistant handoffs remain contextual selected-finding actions, not
   the canonical API completion step for Patrol current work.
   and the canonical alert-triggered Patrol enqueue path in `internal/api/router.go`, so alert-fired Patrol work flows through the unified alert bridge and trigger manager instead of being duplicated by monitor callback wiring
   and the shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx` card, so canonical recent-change timelines stay rendered through one governed frontend card instead of separate page-local list loops
   and the shared `frontend-modern/src/utils/resourceChangePresentation.ts` formatter used by the summary page and resource drawer, so canonical change wording does not drift across surfaces
   and the Patrol-owned `Details` context selector, so same-state recent-change
   records from the canonical AI payload are normalized into changed-substate
   wording before Patrol renders them or attaches them to Assistant, avoiding
   no-op operator copy such as `online` to `online` while preserving the
   backend-owned timeline event, and so those context payloads stay behind a
   compact operator-opened affordance rather than creating a default forensic
   block on the Patrol page
   and the `/api/ai/intelligence/changes` route plus `internal/api/contract_test.go`, so the canonical recent-changes endpoint stays on the same intelligence facade and contract snapshot instead of bypassing the shared timeline source
   and the canonical policy-posture snapshot derived from unified resources, so sensitivity, routing, and redaction counts stay owned by the same AI summary contract instead of being reconstructed as a page-local governance rollup
   and the resource-intelligence payload carried by the drawer AI card, so the resource-detail surface stays on one canonical intelligence contract instead of introducing a separate detail endpoint
   and the learned-correlation payload loaded into the shared AI intelligence store, so the Patrol intelligence page and the AI summary page consume the same governed correlation slice instead of each page fetching its own copy
   and the shared dashboard-load bundle inside `frontend-modern/src/stores/aiIntelligence.ts`, so the page orchestration stays on the store-owned bundle instead of enumerating the AI fetches inline
   and the Patrol page refresh lifecycle in `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`, so slow or stalled secondary reads from that shared dashboard-load bundle may continue resolving in the background while the operator-facing Patrol refresh control remains generation-aware, timeout-bounded, and reusable once Patrol findings and status are already visible
   and the Patrol header support drawer in `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx`, so API-owned Patrol status and trigger facts can feed a secondary Schedule & model surface without turning provider model, schedule, trigger tuning, or background-only runtime-policy pauses into the primary Patrol control decision
   and the shared `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx` card, so the AI summary page renders the governed policy-posture counts while the resource drawer stays on per-resource policy lines instead of carrying duplicate posture UI loops
   and the dedicated `frontend-modern/src/features/patrol/patrolInvestigationContextModel.ts` owner, so recent-change, learned-correlation, and policy-coverage summary text stays derived from the canonical AI payload in one place instead of as hook-local count and pluralization logic
   and that same Patrol investigation-context owner, so the current Patrol
   assessment summary may open Assistant with bounded model-only assessment,
   verification, latest-run, supporting-context, active-finding, and
   resource reference context while leaving prioritization and next-step
   reasoning to the configured LLM; active
   finding entries may carry live pending Patrol approval posture only as safe
   structured handoff actions with approval ID/status/risk/target/request/expiry
   metadata, action plan identity/policy/expiry, dry-run posture, and command
   counts instead of pasting page-local UI text or raw command payloads into chat,
   the drawer target must stay `patrol-assessment` rather than a retired
   dashboard target,
   and may derive model-only approval/action posture from that same safe
   metadata, while visible drawer copy stays compact and does not expose
   Patrol-authored recommendations, prompt chips, or action labels as the
   answer,
   and that same Patrol investigation-context owner, so coverage signals from
   the canonical AI summary remain secondary caveats when the same assessment
   carries active findings, pending approvals, or governed action references;
   Assistant context, briefing action labels, and safety notes must
   carry finding priority, affected resources, evidence, and governed approval
   posture as context instead of recasting the whole handoff as a coverage gap
   and that same Patrol investigation-context owner, so coverage-incomplete
   assessments with no active infrastructure findings are serialized as a
   verification-gap handoff: model-only context and visible briefing copy must explain what
   scoped activity did and did not prove, keep latest-run and supporting-context
   facts model-only, and leave next-step selection to the configured LLM
   handoff-only metadata, and avoid introducing backend fields beyond the
   existing Patrol status plus run-history contracts
   and that same Patrol investigation-context owner, so visible Assistant
   drawer handoffs may include live pending-approval metadata only as safe
   operator context: approval ID, status, risk, requested/expiry timestamps,
   target label, generated approval summary, and command count are allowed, while
   approval command payloads stay inside governed approval/remediation surfaces,
   and finding-level handoff context may be derived from the same
   bounded metadata only so Assistant receives approval status, risk,
   dry-run posture, and existing action artifact metadata without Pulse choosing the next
   step or receiving raw command/execution payloads; finding-level handoffs may also send one
   bounded model-only `handoff_context`, one `handoff_resources` target
   reference, and one `handoff_actions` entry for that governed approval or
   action artifact so the Assistant runtime can refresh finding and action posture
   from IDs and safe summaries instead of relying on pasted chat text
   and the dedicated `frontend-modern/src/stores/aiIntelligenceSummaryModel.ts` owner, so recent-change counts and governed policy-posture fallbacks normalize once at the shared store boundary instead of as Patrol-hook-local payload repair
   and the shared `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx` card, so learned correlations and correlation context stay rendered through one governed frontend card instead of separate page-local list loops
   and the same shared correlation card's ordering and truncation rule, so callers pass raw correlations instead of encoding their own top-N sort behavior
   and the shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx` and `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx` cards' infrastructure resource-link default, so the Patrol page, resource drawer, and problem-resource dashboard panels inherit the canonical resource-filter path construction instead of rebuilding infrastructure URLs inline
   and the Patrol runtime-remediation destination shared with the AI settings endpoint, so summary actions, run-history runtime-failure actions, and runtime-finding actions may reuse the governed provider-settings route while still presenting that destination in Patrol as provider configuration instead of generic `AI Settings` copy
   and the Patrol route-shell destination itself, so the thin page shell at `frontend-modern/src/pages/AIIntelligence.tsx` may continue to bridge the shared AI-runtime payload boundary while exposing `/patrol` as the canonical product route and keeping retired `/ai` browser entry points unregistered
   and the Patrol route-shell accessibility boundary, so brand icons in `frontend-modern/src/features/patrol/PatrolIntelligenceHeader.tsx` stay decorative when the same heading already exposes visible Patrol text, preventing duplicate accessible names such as `Pulse Patrol Patrol`
   and the Patrol mode selector boundary, so the default header and the
   Patrol mode dialog compose the shared
   `frontend-modern/src/components/shared/FilterButtonGroup.tsx` primitive for
   the visible `Watch only` / `Ask before changes` / `Auto-fix safe issues` /
   `Policy autopilot` presentation while the API contract remains the sole owner of
   accepted autonomy values, `full_mode_unlocked` persistence,
   license-required rejection shape, and the monitor-only clamp; when the
   API/license state clamps Patrol to monitor for plan reasons, frontend
   consumers must present watch-only as the effective capability and badge paid
   levels rather than rendering a default Pro-absence explainer or a full-loop
   header description, while runtime-locked Pro installs may explain the missing
   runtime; provider model, schedule, trigger tuning, and readiness validation
   must remain advanced settings plumbing alongside that inline Patrol mode
   boundary. API-adjacent frontend and docs projections may keep legacy wire
   values such as `monitor`, `assisted`, and `full`, but customer-facing copy
   must name the visible choices as `Watch only`, `Auto-fix safe issues`, and
   `Policy autopilot` rather than leaking compatibility terminology.
   policy rather than changing the compatibility API boundary; setup-only
   readiness may hide those run/configuration affordances, but the visible
   Patrol mode selector remains the primary policy boundary during setup
   and the Patrol mode presentation boundary, so `frontend-modern/src/features/patrol/PatrolIntelligenceWorkspace.tsx` routes active Patrol findings into the Patrol-owned findings workflow, renders the first-party loop as watch/investigate/act-under-policy/verify/record, uses `Patrol mode` as the human-facing name for the governed autonomy selector while preserving `patrol_control`, `patrolControl*`, and `patrol_autonomy` only for compatibility route and wire identifiers, and demotes Assistant and external-agent readiness out of the primary operator loop without introducing a new API request shape, frontend-authored tool route, serialized remediation plan, or page-local MCP setup contract; direct single-finding CTAs must derive from existing finding-presentation helpers and canonical routes such as the Patrol provider-settings route, selected findings, approvals, and history rows may still open contextual Assistant handoffs through their governed owners, setup-only Patrol runtime failures must use existing finding/runtime fields to render the Patrol-owned `Fix Patrol setup` framing, a dedicated setup task, and one direct `Open Provider & Models` action while suppressing the readiness banner, generic issue-row chips, filter chrome, and run-history action chrome that would compete with provider setup, but recent changes, correlations, and policy-coverage payloads must not render as a generic first-party Details/evidence console on the Patrol page, and raw finding lifecycle telemetry must be reserved for explicit all/resolved/history or selected-run review states instead of default active current-issue expansion
   The Patrol page may consume server-authored readiness and preflight-backed
   status, but it must translate that transport state into `Patrol setup issue`
   or `Patrol setup warning`, provider/model context, and the canonical
   `Open Provider & Models` action. Raw preflight, tool-call observation, and
   readiness-internal wording remain API/settings diagnostics rather than
   first-party Patrol operator banner copy.
   and the external-agent Patrol-control status route, so `GET /api/agent/patrol-control/status` may expose only aggregate Patrol issue evidence, aggregate active issue-level Patrol finding counts, pending approvals, contextual collaboration counts inside the Assistant step, governed action counts split into approved and rejected decision counts, verified outcome counts, Patrol control starter/completed-loop/resolved-loop proof exposed through primary `patrolControl*` fields, `patrolAutonomy*` compatibility fields, `proActivationOperationsLoopStarterCount` as an older-client compatibility field, completed/resolved/value `proActivation*` compatibility aliases, optional token-backed MCP readiness, generated timestamps, and the next coarse loop action for native and MCP-facing orchestration; it must not expose finding IDs, action IDs, resource names, commands, prompt text, model output, actors, request bodies, token identity, token names, or token counts, and it must not replace the canonical fleet-context, resource-context, finding, approval, or action routes that own the underlying detail. Its `progressLabel` is operator-facing Patrol status copy: it must describe the current action or outcome in plain Patrol terms such as checking, investigating, approval, rejection, verification, and recorded history rather than activation-loop, value-proof, autonomy, or MCP-completion language. Human-facing manifest capability descriptions, generated MCP README text, and frontend projections for this route must say Patrol mode; wire identifiers such as `patrol_control`, `patrolControlCompletedOperationsLoopCount`, `patrol_autonomy`, `patrolAutonomyCompletedOperationsLoopCount`, and `patrolAutonomyValueState` may retain control/autonomy terminology for compatibility, and `proActivation*` wire identifiers may remain only as compatibility fields, not as user-visible Pro activation journey copy. The route may count generic execution lifecycle as recent loop activity, but the governance stage is decision-backed: `governedActionCount` must require approved or rejected governed-action evidence, `approvedDecisionCount` and `rejectedDecisionCount` must preserve that split without identifiers, and `verifiedOutcomeCount` must require an approved governed action with verified post-action evidence. Current active findings and pending approvals are live operator work: they must keep the next action and four-step rollup pointed at the current finding or approval before older Patrol control completed/resolved proof is presented as history. Aggregate issue evidence and resolved trust history are not by themselves current operator work, so native Patrol consumers must not turn resolved-only history into current finding copy or actions. `patrolControlCompletedOperationsLoopCount` and the mirrored `patrolAutonomyCompletedOperationsLoopCount` may only be derived when content-free Patrol control starter evidence, Patrol issue evidence, contextual Assistant or external-agent collaboration evidence, and either rejected governed-decision evidence or approved governed-decision evidence with verified outcome evidence coexist in the same status window. `patrolControlResolvedOperationsLoopCount` and the mirrored `patrolAutonomyResolvedOperationsLoopCount` may only be derived when content-free Patrol control starter evidence, Patrol issue evidence, contextual Assistant or external-agent collaboration evidence, approved governed-decision evidence, and verified outcome evidence coexist in the same status window; no Patrol control or compatibility alias field may carry prompt, account, action, resource, or finding identity. The four-step `steps` rollup must keep counts aligned with the operator evidence that satisfies each stage: Patrol counts issue evidence, Assistant counts contextual collaboration without prompt or response content, governance counts pending approvals until an approved or rejected decision exists, then counts those decisions, and verification counts verified outcomes or terminal rejected decisions when no write ran. External agents remain optional readiness context through `externalAgentReady` rather than a first-party activation gate or operator step. A rejected-only decision is terminal for the execution-verification branch because no write ran; an approved decision still requires verified outcome proof before the verification step completes.
   The approved-success/verified-outcome predicate is shared inside
   `internal/api/`: outbound Pulse Intelligence action telemetry and
   `GET /api/agent/patrol-control/status` must both route through the same
   approved-action verification helper. A completed action with
   `ExecutionResult.Success=true` is execution evidence only; it must not set
   `verifiedOutcomeCount`, approved-action-success telemetry, Patrol control
   resolved-loop proof, or paid resolved-loop proof unless the action also has
   `VerificationOutcome.Status=verified` or a canonical verification result that
   ran and succeeded.
10. Route frontend API-client parsed error propagation, API-error-status fallback handling, allowed-status handling, custom status-specific error handling, command-trigger success envelope handling, shared response parsing pipelines, missing-resource lookup handling, metadata CRUD routing, stream event consumption, response status, collection normalization, scalar payload coercion, and structured error normalization through canonical shared helpers under `frontend-modern/src/api/`
    Telemetry preview payloads are part of this same frontend API-client
    boundary. `frontend-modern/src/api/settings.ts` must mirror the
    content-free `internal/telemetry.Ping` payload shape exposed by the system
    settings preview route, including Pulse Intelligence loop booleans, Assistant
    and Patrol counters, operations-loop workflow starter counts, governed-action
    counters including approved/rejected action decisions, and external-agent/MCP aggregate, adapter-origin, and
    capability-class counters, so browser preview, privacy disclosure, and
    outbound heartbeat JSON do not drift into separate contracts.
    Operations-loop starter fields may count only the canonical
    `pulse_operations_loop` prompt over the rotating telemetry window, with
    total, native Assistant, first-party Patrol, primary Patrol control,
    legacy Pro activation entry-point, and Pulse MCP counters. They are
    active-journey evidence,
    not proof that the user collaborated with context/tools, approved execution,
    verified an outcome, or resolved a finding.
    The external-agent/MCP readiness boolean in that same payload is a
    manifest-surface capability signal: a non-expired API token covering any
    Pulse MCP-published capability scope may configure the surface, while recent
    use still requires route activity markers and generic `ai:chat` tokens do
    not count as external-agent readiness.
    Assistant chat stream workflow-state payloads are part of this same
    frontend API-client boundary. `workflow_state` events must keep `phase`,
    `message`, `state`, and `tool` stable, selected-route starts may carry
    `provider` and `model`, and selected-route retry may carry `attempt`,
    `max_attempts`, and `retry_after_ms`; automatic provider fallback metadata
    (`provider_fallback`, `failed_provider`, `failed_model`, `next_provider`,
    `next_model`) is retired from `/api/ai/chat` stream payloads. The
    selected-route `provider_start` message is operator
    progress copy and must be active/in-progress wording such as
    `<Provider> is starting the response.` while the typed `provider` and `model`
    fields carry the exact route identity. The generated
    `frontend-modern/src/api/generated/aiChatEvents.ts` type must stay derived
    from `internal/ai/chat/types.go` through `scripts/generate-types.go`;
    `frontend-modern/src/api/generated/agentCapabilities.ts` must stay
    derived from `internal/agentcapabilities`, including manifest surface
    contract fields; and frontend API tests must pin
    any new generated SSE fields, including live tool-start and tool-progress
    payload fields such as `phase` and `message` and pending tool
    cancellation payloads such as `reason`. Control-level
    values exposed through AI settings or chat payloads must alias the shared
    `agentcapabilities.ControlLevel` vocabulary rather than redefining
    read-only/controlled/autonomous strings in API-local code.
    That same shared org-management client boundary now owns target-consent
    sharing semantics across `frontend-modern/src/api/orgs.ts`,
    `internal/api/org_handlers.go`, and the shared org route wiring. Cross-org
    share creation must remain a pending request until the target organization
    accepts it, the payload must preserve `status`, `acceptedAt`, and
    `acceptedBy`, and widening an accepted share's requested role must reset the
    share back to `pending`. Downstream settings surfaces must not infer live
    access from share creation alone or recreate manager-only pending-share
    visibility rules locally.
11. Add or change API token scope, assignment, and revocation presentation through `frontend-modern/src/components/Settings/APITokenManager.tsx`, `frontend-modern/src/components/Settings/apiTokenManagerModel.ts`, and `frontend-modern/src/components/Settings/useAPITokenManagerState.ts`
    That same shared token contract also owns audit scope separation: audit event, verification, summary, export, and unified action/export audit reads must require the dedicated `audit:read` scope instead of reusing broader monitoring or settings-read token grants.
12. Add or change infrastructure operations token generation, lookup, assignment, the pure unified-agent inventory/install model, the split infrastructure install state owner, the split direct-node/discovery infrastructure settings owners, the shared infrastructure-operations state provider/context shell, and install presentation through `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`, `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts`, `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts`, `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`, and `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`. Phase 9 retired the InfrastructureOperationsController shell and the useInfrastructureReportingState reporting path; they must not be reintroduced, and aggregator-backed reporting reads are owned by `frontend-modern/src/components/Settings/useConnectionsLedger.ts` under the frontend-primitives contract.
    That same aggregator-backed reporting read may hide passive config or
    rollout handshakes from primary source-manager attention when the API
    reason says only that an agent has not yet reported a comparable applied
    configuration fingerprint. That is a presentation choice over the
    canonical `Connection.fleet` payload, not a second API state: the raw fleet
    fields must remain available for deeper fleet diagnostics and must not be
    rewritten into source credential, liveness, or setup failure facts.
    That same governed infrastructure-operations API boundary also owns discovery polling activation: the shared discovery runtime may only poll `/api/discover` while the settings shell has the `infrastructure-connections` route active, so route-level IA changes cannot silently keep discovery traffic alive on unrelated systems or install screens.
    That same governed setup/install boundary also owns uninstall convergence: when a script-managed Proxmox node removes its local Pulse credentials, the canonical `/api/auto-unregister` API must remove the matching stored node immediately and emit the same discovery/node-deleted refresh semantics as manual deletion, so the infrastructure sources table does not keep a stale active row until the next failed poll.
    That same governed install-presentation boundary must preserve the
    Docker-versus-Proxmox LXC distinction from the shared model: generated
    install surfaces may not imply that Docker inside Proxmox LXCs requires a
    guest-local agent when the intended path is a Proxmox node agent with
    command execution and server-side guest Docker inventory opt-in.
    That same governed setup/install boundary also owns Proxmox
    `authorized_keys` symlink preservation in the rendered PVE setup script:
    temperature-key install and uninstall edits must resolve the real
    authorized-keys target before filtering Pulse-managed `# pulse-` lines,
    and `internal/api/contract_test.go` must pin the generated shell shape.
13. Keep `internal/api/session_store.go` on a fail-closed auth-persistence boundary: persisted OIDC refresh tokens may only round-trip through encrypted-at-rest session payloads, and any missing-crypto or invalid-ciphertext path must drop the token instead of preserving plaintext-at-rest session state.
14. Keep tenant AI handler wiring on canonical provider ownership: `internal/api/ai_handlers.go` may wire tenant `ReadState` and tenant-scoped unified-resource providers into AI services, but it must not revive tenant snapshot-provider bridges once Patrol can initialize and verify from those canonical providers directly.
15. Keep Patrol status transport semantics explicit in that same AI handler layer: the Patrol status endpoint must carry machine-readable runtime availability such as blocked, running, disabled, active, or unavailable rather than asking frontend consumers to infer operator state from stale summaries or run history.
16. Keep legacy Patrol quickstart transport semantics retired from the public v6 GA contract: ordinary AI settings and Patrol status payloads must not expose quickstart credit/status fields, and any stale hosted-model blocked copy that survives from compatibility state must normalize back to provider/local-model setup rather than presenting credit badges or acquisition prompts.
17. Keep Patrol intelligence summary transport semantics single-voiced: the canonical overall-health payload and Patrol run-history payload together must support one primary assessment plus one explicit verification explanation, and frontend consumers must not need to derive a second compact assessment or verification verdict row from the same payloads beneath the primary assessment strip.
    That same transport split now supports the visible Patrol assessment and
    action metadata without adding another API field: the frontend summary
    contract derives compact state from existing overall-health, run-history,
    active finding, runtime, and pending-approval facts, so the API remains the
    source of facts while the configured LLM owns next-step reasoning. Those
    action references map back to existing API-backed Patrol controls and
    approval/finding filters; summary transport must not become a new execution
    or approval API.
    The API contract should stay presentation-neutral here: it supplies the
    score, current risk facts, verification facts, and action state needed for a
    compact operator summary, but it must not imply a hero, card, duplicate
    verdict layout, or expanded assessment panel. Normal Patrol page consumers
    should keep explanatory assessment, verification, activity, and supporting
    context in the owning Findings, Runs, and `Details` surfaces rather
    than re-expanding the primary assessment strip.
18. Keep Pulse Mobile relay credential minting and permission ownership on backend ownership: `internal/api/router_routes_auth_security.go`, `internal/api/security_tokens.go`, `internal/api/auth.go`, `internal/api/relay_mobile_capability.go`, `internal/api/router_routes_ai_relay.go`, and `frontend-modern/src/api/security.ts` may expose the canonical mobile runtime token creator and governed route gates, but browser callers must only consume that route and must not define the mobile runtime scope, compatibility gate list, route inventory, or token-purpose metadata locally.
19. Keep hosted tenant browser-session precedence on the shared auth boundary: `internal/api/auth.go`, `internal/api/contract_test.go`, and hosted tenant callers must treat a valid `pulse_session` as authoritative before any API-only token fallback or no-local-auth anonymous fallback, so cloud handoff can continue into protected hosted routes without flattening the operator back to `anonymous` or forcing a browser session through bearer-token-only mode after the tenant has minted API tokens.
    That same shared auth boundary also owns hosted handoff authorization. `internal/api/cloud_handoff.go`,
    `internal/api/cloud_handoff_handlers.go`, and hosted tenant callers must derive the effective tenant role from
    pre-existing server-side org membership only, rather than trusting the handoff JWT to append missing members,
    repair org metadata, or upgrade roles on arrival. Handoff may mint a browser session only when the tenant org
    already contains the account as the owner or a member with a valid stored role, and tenant orgs with a blank
    `OwnerUserID` must fail closed instead of being claimed by the first owner-shaped handoff token.
    Hosted handoff session identity must bind to the signed stable user subject
    (`sub`/`UserID`) rather than the contact email. Email may participate only
    as legacy membership lookup and delivery metadata, and any canonicalization
    of email-keyed tenant membership must preserve the stored role instead of
    creating or elevating membership from the token. Blank or email-shaped
    handoff subjects are invalid even when the tenant still contains legacy
    email-keyed owner/member rows.
    Hosted magic-link verification follows the same API/session identity rule:
    the token may carry contact email for delivery, but `/api/public/magic-link/verify`
    must resolve that email against current server-side organization metadata
    and create the browser session for the stored owner/member principal.
    Magic-link request and verify paths must fail closed when contact email
    matches a blank owner/member principal instead of sending or accepting a
    token that would turn email into the session identity.
    Checkout webhook post-payment magic-link delivery follows the same
    resolver-owned rule: Stripe `customer_email` can drive only best-effort
    delivery after server-owned org linkage is validated, and it must not send
    when the linked org row has matching contact metadata but no stored
    owner/member principal.
    Live org authorization is stricter than those delivery and migration
    helpers: `internal/api/authorization.go`, `internal/api/org_handlers.go`,
    `internal/api/cloud_org_admin_auth.go`, and settings-scope checks must use
    strict `OwnerUserID`/member `UserID` membership helpers for request access,
    not the email-aware compatibility accessors.
    Public hosted signup must therefore keep the generated owner user ID
    server-side for org metadata and RBAC assignment while using returned
    contact email only for `GenerateToken`/`SendMagicLink`; the accepted signup
    response remains uniform and must not expose the owner principal.
20. Keep tenant settings-scope authorization aligned with org management: `internal/api/security_setup_fix.go`, `internal/api/contract_test.go`, and settings-bound hosted callers must allow the current non-default org owner/admin membership to exercise privileged tenant routes, rather than requiring a separate configured local admin identity after hosted handoff.
    Hosted handoff must not be treated as an org-management side effect for that same privilege boundary. Only
    canonical invitation, membership-management, or explicit owner-transfer flows may create tenant membership or
    change the stored owner/admin role. Shared auth routes and downstream settings consumers must treat handoff role
    claims as bounded by the server-owned membership record, never as authority to elevate tenant privileges.
    That same org-management transport now owns explicit acceptance for new self-hosted membership as well.
    `internal/api/org_handlers.go`, `frontend-modern/src/api/orgs.ts`, and `internal/api/contract_test.go` must
    keep new-user adds on the canonical pending-invitation payload (`kind:"invitation"`) plus current-user
    accept/decline routes, rather than binding an arbitrary username directly into `org.Members`. Immediate role
    mutation remains valid only for already-accepted members, and owner transfer must fail closed unless the
    target user is already a stored member. The same permanent-control boundary also requires a fresh browser
    session for owner transfer: `internal/api/auth.go`, `internal/api/session_store.go`, and
    `internal/api/org_handlers.go` must reject transfer attempts unless the request carries the bound
    `pulse_session` cookie for the acting owner and that session was minted recently enough to represent an
    explicit re-auth, rather than letting any long-lived hijacked session permanently reassign org ownership.
    That same shared auth boundary also owns pre-auth local setup and recovery containment. When no authentication is
    configured, anonymous fallback and bootstrap quick setup may run only on direct loopback, recovery tokens must bind
    to the generating client IP, and recovery may mint only a browser-bound localhost session rather than a shared
    filesystem toggle that disables auth for every loopback client.
    Direct `CheckAuth` callers must also fail explicitly: a non-loopback or missing-credential request may not fall
    through as a silent `200`, and middleware wrappers must use shared response capture so route-specific auth errors
    remain single-written rather than being replaced by a second generic auth body.
    The same shared auth boundary also owns release-build admin bypass gating.
    `internal/api/auth.go` may keep `ALLOW_ADMIN_BYPASS` for non-release
    development workflows, but release builds must compile that env override
    out entirely instead of reading it and deciding at runtime whether to
    honor or ignore it.
21. Keep mobile onboarding payload reads aligned with the server-owned relay-mobile credential: `internal/api/router_routes_ai_relay.go`, `internal/api/onboarding_handlers.go`, and `internal/api/contract_test.go` must allow the dedicated `relay:mobile:access` scope to reach the governed QR, deep-link, and connection-validation payloads without reintroducing a broader `settings:read` requirement for token-authenticated pairing clients.
    That same shared relay/runtime boundary also owns hostname target
    equivalence for agent command routing. `internal/api/router_routes_ai_relay.go`
    and `internal/api/contract_test.go` may match a short host against the
    canonical connected-agent FQDN, but they must do so through
    `internal/unifiedresources/hostname_equivalence.go` and must not collapse
    distinct FQDNs that merely share the same short hostname into one API
    target. When an explicit `targetHost` misses that canonical match, the
    shared relay adapter must keep the result empty instead of silently
    falling back to the lone connected agent.
    That same shared runtime-token boundary also owns agent-exec binding.
    `internal/api/deploy_handlers.go`, `internal/api/router.go`, and
    `internal/api/contract_test.go` must mint agent-exec-capable runtime
    tokens with a server-owned `bound_agent_id` and reject websocket
    registration when the token is missing binding metadata or names a
    different agent. That same websocket admission path must also cap
    concurrent connections per client IP before upgrade so one source cannot
    hold unbounded agent-exec sockets open. Legacy `bound_hostname` metadata may be normalized only
    as compatibility input into that same canonical `agent-<hostname>`
    binding, and unbound agent-exec tokens must fail closed instead of being
    treated as global command authority.
    Proxmox-side LXC Docker detection and inventory wiring in
    `internal/api/router.go` shares that agent-exec transport boundary:
    router startup may configure the monitoring checker or collector only when
    explicit server env opt-in is present, must route execution through the
    authenticated Proxmox node agent, and must keep inventory collection behind
    the monitoring-owned minimal Docker summary contract rather than exposing
    a new public API payload shape.
22. Keep hosted billing-state quickstart grants retired from new shared API flows: `internal/api/hosted_entitlement_refresh.go`, hosted signup, and trial-state construction must not auto-grant or refresh quickstart inventory for new workspaces, while low-level billing-state readers may still preserve historical fields that already exist on disk.
23. Keep hosted AI settings bootstrap on the shared API contract as a retired path: `internal/api/ai_hosted_runtime.go`, `internal/api/ai_handlers.go`, `internal/api/ai_handler.go`, and `internal/api/contract_test.go` must treat a missing `ai.enc` in hosted mode as an unconfigured BYOK/local-provider state, not as a machine-owned `quickstart:pulse-hosted` bootstrap condition. Hosted tenant reads may inherit billing state for commercial authorization, but they must not create quickstart-backed AI config or call the quickstart bootstrap upstream route.
24. Keep post-boot AI enablement contract-backed on the shared AI/mobile approval surface: `internal/api/ai_handler.go`, `internal/api/ai_handlers.go`, `internal/api/router_routes_ai_relay.go`, and `internal/api/contract_test.go` must turn the governed approvals-list API into the canonical empty-list payload as soon as settings-driven AI enablement succeeds, rather than leaving that surface on `503 Approval store not initialized` until some separate startup-only side effect happens.
25. Keep infrastructure summary chart transport contract-backed on the shared API surface: `internal/api/router.go`, `internal/api/contract_test.go`, and frontend infrastructure summary consumers must normalize long-range mixed-cadence history into equal-time summary buckets before shipping the infrastructure charts API payload, so 7-day and 30-day summary cards do not expose compressed right-edge tails just because recent samples arrive at a finer storage resolution.
26. Keep long-range workload chart transport time-proportional on the shared API surface: `internal/api/router.go`, `internal/api/contract_test.go`, and workload chart consumers must cap mixed-cadence workload history by equal-time buckets rather than raw point index for the per-workload and aggregate workload chart APIs, so 7-day and 30-day workload cards do not bunch recent samples at the right edge just because recent telemetry is stored more densely.
27. Keep chart timestamp precision canonical on that same shared API surface: when `internal/api/router.go` serializes monitoring history into infrastructure or workload chart payloads, it must preserve canonical millisecond timestamps from the shared monitoring timeline instead of rounding through whole-second conversion, so seeded mock history and live appends collapse onto one operator-visible timeline instead of appearing as duplicated tail samples.
28. Keep Patrol remediation payload naming backward-compatible without leaking
    legacy automation-first wording into product copy. `frontend-modern/src/api/patrol.ts`,
    `internal/api/ai_handlers.go`, and `internal/api/router_routes_ai_relay.go`
    may continue to expose stable transport fields such as `fixed_count`,
    `auto_fix`, `autofix`, and the `ai_autofix` capability name, but comments,
    API license-required messages, and presentation labels layered on that API
    contract must describe the operator-visible capability as remediation or
    safe remediation workflows.
29. Keep storage chart identity canonical on that same shared API surface: the shared storage charts endpoint must key pool and physical-disk series by the resolved unified-resource `MetricsTarget.ResourceID`, not by canonical resource IDs or page-local aliases, so storage rows, focused summary cards, sticky summary shells, and detail charts all address the same history series in live and mock mode.
30. Keep synthetic summary-chart fallback identity canonical on that same shared API surface: when `internal/api/router.go` has to synthesize mock summary history for infrastructure, workloads, or storage cards, it must derive the fallback from canonical `resourceType`, `resourceID`, and `metricType` ownership instead of raw min/max seed-prefix helpers, so range changes and runtime mock updates stay on one governed timeline.
    The same compact chart boundary also owns aggregate-only storage summary
    transport. `/api/charts/storage-summary` may batch only the canonical
    `used` and `avail` storage series required for the aggregate capacity
    sparkline, and it must not regress into the full per-pool storage payload
    or a fetch-all-metrics backend path just because the storage page carries a
    broader chart surface.
    When mock mode is active, that same endpoint must come from the
    monitor-owned aggregate summary cache rather than rehydrating each pool
    chart on request.
31. Keep workload-chart response identity canonical on that same shared API surface: `internal/api/router.go`, `internal/api/contract_test.go`, and workload summary consumers must emit provider-backed VM and system-container series under the same canonical workload IDs that workloads page rows use, while resolving history through the unified `MetricsTarget.ResourceID`, so hover and focus selection do not fall off for provider-backed rows.
    Kubernetes pod workload rows follow that same contract through their
    metrics target. `/api/resources` may expose pod history only through the
    unified `MetricsTarget.ResourceID`, but that target must be the canonical
    prefixed runtime key `k8s:<cluster>:pod:<uid>` and not the bare source pod
    ID, so pod workload rows and pod chart payloads stay on one history series.
32. Keep the hosted account portal bootstrap intelligible without duplicate
    chrome. `internal/cloudcp/portal/page.go`, the maintained portal frontend
    bundle, and the shared portal styles may refine layout density, but the
    account/billing shell must remain understandable from the primary header,
    section title, and factual body content alone instead of depending on a
    second context-chip strip to restate the same scope.
33. Keep storage wire metadata lossless across shared API payload types.
    `frontend-modern/src/types/api.ts` must continue to expose provider-backed
    storage metadata such as Proxmox `pool` and `zfsPool` fields when the
    backend emits them, instead of silently dropping that detail from the
    shared runtime contract.
34. Keep hosted entitlement refresh ownership on the same governed API contract
    as hosted status and entitlements reads. `internal/api/licensing_handlers.go`,
    `internal/api/hosted_entitlement_refresh.go`, and
    `internal/api/contract_test.go` must resolve the effective hosted billing
    target before refresh, persistence, and evaluator rewiring, so tenant-
    scoped hosted routes cannot refresh against an empty non-default org while
    the machine's real hosted lease still lives on `default`.
    The hosted verifier bridge may keep the legacy
    `PULSE_TRIAL_ACTIVATION_PUBLIC_KEY` environment literal for deployed tenant
    compatibility, but API call sites that validate hosted entitlement leases
    must route through the `HostedEntitlement*` licensing aliases rather than
    treating the retired trial-activation callback as the active acquisition
    model.
35. Keep public demo bootstrap posture on the shared security-status contract.
    `internal/api/router_routes_auth_security.go`,
    `internal/api/security_status_capabilities.go`, frontend security-status
    consumers, and shared demo-mode stores must treat
    `/api/security/status.sessionCapabilities.demoMode` as the canonical
    browser bootstrap signal for public demo posture instead of asking
    frontend callers to infer demo state from response headers, `/api/health`
    probes, or hostname heuristics. Shared browser stores that consume
    Patrol approvals must also fail closed from that resolved demo policy at
    the store boundary, so public demo shells do not probe `/api/ai/approvals`
    or `/api/ai/remediation/plans` after the read-only demo posture is already
    known.
36. Keep public demo commercial posture middleware-owned on that same shared
    API contract. `internal/api/demo_middleware.go`,
    `internal/api/demo_mode_commercial.go`,
    `internal/api/subscription_entitlements.go`, and
    `internal/api/contract_test.go` must classify commercial routes centrally
    as either hidden (`404`) or runtime-safe. Public demo browsers may read the
    non-commercial `/api/license/runtime-capabilities` contract for feature
    truth, while `/api/license/commercial-posture`,
    `/api/license/entitlements`, and `/auth/license-purchase-start` stay
    hidden. Upgrade prompts, trial nudges, monitored-system migration guidance,
    usage counts, billing identity, and plan metadata must therefore not depend
    on hidden commercial routes surviving the public demo boundary.
37. Keep the storage summary route in `internal/api/router.go` as the
    canonical storage summary contract across dashboard and storage consumers.
    `internal/api/router.go`,
    `internal/api/contract_test.go`, and shared frontend consumers must expose
    pooled storage history through one response keyed by canonical
    metrics-target IDs, preserve millisecond chart timestamps, and avoid
    reconstructing storage summary behavior from per-pool
    `/api/metrics-store/history` fan-out.
38. Keep infrastructure summary metric filtering canonical on that same shared
    API surface. `frontend-modern/src/api/charts.ts`,
    `internal/api/router_routes_monitoring.go`, `internal/api/router.go`,
    `internal/api/types.go`, and `internal/api/contract_test.go` must route
    optional infrastructure-summary `metrics` filters through one governed
    transport contract, so route-owned consumers can request only the series
    they render without inventing a second summary endpoint or silently
    widening back to disk/network payloads. The same contract must carry those
    requested metric filters through the shared guest-chart batch loader in
    `internal/monitoring/monitor_metrics.go` instead of fetching the full guest
    metric set and trimming after the API payload is already assembled.
39. Keep the retired compact dashboard overview route absent from that same
    shared API surface. `internal/api/resources.go`,
    `internal/api/router_routes_monitoring.go`, and
    `frontend-modern/src/api/resources.ts` must not restore
    `/api/resources/dashboard-summary`, `useDashboardOverview`, or frontend
    dashboard consumers as compatibility paths for KPI cards, problem-resource
    rows, governed resource labels, top-infrastructure identity, or metrics-
    target join keys. New summary payloads must be owned by their product
    route and pinned in the API contract there.
40. Keep mock and demo chart reads on the same canonical unified snapshot as
    the rest of the API surface. `internal/api/router.go`,
    `internal/api/contract_test.go`, and chart consumers must route
    `/api/charts`, `/api/charts/infrastructure`, and `/api/storage-charts`
    through `GetUnifiedReadStateOrSnapshot()` whenever mock or demo
    presentation is active, so VMware, storage, and infrastructure series stay
    aligned with `/api/resources` and `/api/state` instead of drifting onto the
    live store-backed graph.
41. Route the unified connections ledger and address probe through
    `internal/api/connections_types.go`,
    `internal/api/connections_aggregator.go`,
    `internal/api/connections_handlers.go`,
    `internal/api/connections_probe.go`, and
    `frontend-modern/src/api/connections.ts` together so `GET /api/connections`
    and `POST /api/connections/probe` stay on one canonical payload shape
    instead of re-deriving state from per-type config stores in the frontend.
    State must remain a derived field sourced from in-memory scheduler health
    (`monitoring.Monitor.SchedulerHealth()`) plus agent `Host.LastSeen`; the
    endpoint must not introduce new persisted per-connection state. The probe
    endpoint must remain admin-gated (`RequireAdmin` + `ScopeSettingsWrite`)
    to block unauthenticated SSRF against internal hosts. That same probe path
    must also validate user-supplied addresses before probing, reject metadata,
    link-local, multicast, and unspecified destinations, and pin each outbound
    dial to the first permitted resolved IP so DNS rebinding cannot swap the
    target between validation and connect time. That same `/api/connections`
    payload now also owns the additive `systems[]` grouping contract for the
    infrastructure settings source manager. Those grouped rows must stay
    source-oriented and backend-authored: one primary source row may carry
    attached collection methods such as a linked Pulse Agent, but attached
    methods must not be emitted as duplicate peer rows when backend ownership
    can prove they augment the same source. When the owning source is a
    Proxmox cluster, that same backend-authored system payload must also
    carry the canonical cluster identity so the frontend can label the row by
    cluster moniker instead of by one endpoint node's hostname. That grouped
    payload must also carry the backend-authored cluster member collection
    with node identity, endpoint, node-local status, and any linked agent
    connection id so the frontend can render child node composition without
    reverse-engineering it from standalone agent rows. Those member records,
    plus any primary or attached connection row that represents the same host,
    must also carry canonical host aliases when the backend knows them, so
    discovery and settings surfaces can reconcile hostname-only and IP-only
    views of the same enrolled machine instead of showing a second
    "discovered" candidate row for an already represented source member or
    API-plus-agent source row. Exact host-agent matches for a configured
    Proxmox primary source must also attach through the backend systems
    payload when workload node inventory is absent because credentials or
    reachability are blocked, so the infrastructure ledger does not split the
    same physical host into an API row plus a standalone host-agent row.
    Agent-backed
    connections also own canonical version/update facts on that same payload:
    when a source or attachment is backed by Pulse Agent, `/api/connections`
    carries the
    installed agent version, the current server-side target agent version when
    it is meaningful, and whether an update is available, so settings surfaces
    do not invent frontend-local version comparison rules. That same shared
    contract also carries compact `agentIdentity` facts on agent-backed
    connections, including the reported hostname, report IP, platform/OS,
    kernel, architecture, and command capability, so settings surfaces can
    render recognizable standalone-host identity without a second inventory
    fetch or frontend-local host reconciliation rules.
    Agent config drift on that same payload must source desired fingerprints
    from `Monitor.GetHostAgentConfig(...).DesiredConfig` or the same
    `remoteconfig.BuildDesiredConfigMetadata` path. Desired config metadata is
    actionable only when the server has assigned a managed desired value, such
    as an explicit command-execution policy or an agent-applied settings key.
    The empty default host-agent config may still carry signed metadata for
    validation, but `/api/connections` must not turn that passive default into
    an operator-visible pending rollout. Agent rows without a managed config
    override expose `configDrift.status: not-applicable` and a current applied
    rollout with a bounded no-rollout reason. When a managed desired
    fingerprint exists, the aggregator must not manufacture convergence by
    assigning desired and applied to the same local report-field fingerprint;
    if host state lacks a trustworthy applied config fingerprint,
    `configDrift` stays pending or unknown and rollout stays non-current.
    Appliance-specific Pulse Agent compatibility is an additive host-profile
    fact on that same identity payload. For Unraid and similar host profiles,
    `agentIdentity.platform` remains the canonical runtime platform such as
    `linux`, while `agentIdentity.hostProfile` carries the governed profile id
    such as `unraid`; frontend clients must not re-promote those profile ids
    into first-class platform types. Once a governed host profile is resolved,
    the runtime platform is the profile's manifest runtime platform even when
    the host's base distro token is something implementation-specific such as
    Slackware on Unraid.
    Unified-resource agent payloads use the same split: `agent.platform` is
    the normalized runtime platform and `agent.hostProfile` is the optional
    governed profile id for host/appliance presentation.
    The API aggregator must resolve host-profile identity tokens and runtime
    platform fallback values through the generated platform-support backend
    projection in `internal/platformsupport/manifest_generated.go`, so payload
    normalization cannot drift into API-local Unraid or appliance branches.
    `pulse fleet connections` may read that same `GET /api/connections`
    payload as a deterministic CLI adapter for agent-ready operations, but it
    must remain a read-only view over the canonical connections ledger rather
    than re-deriving fleet governance state in CLI-local code.
    The legacy `Storage` payload shape exposed by the frontend types in
    `frontend-modern/src/types/api.ts` may also carry an optional
    canonical `platform` field (e.g. `vmware-vsphere`, `truenas`,
    `proxmox-pve`) when the source adapter knows which platform owns
    the storage record. Frontend source-filter resolvers must prefer
    that canonical platform key over the on-disk `type` (`vsan`,
    `vmfs`, `nfs41`, `zfs-pool`) so platform-page source filters,
    chip option counts, and grouping all collapse to the canonical
    platform vocabulary instead of leaking storage technology labels
    into operator chrome.

## Forbidden Paths

1. Handler-local payload shape drift without a contract test
2. Untracked compatibility aliases becoming permanent runtime contracts
3. Frontend-only payload assumptions that are not owned in backend contracts
4. Frontend API clients inferring canonical HTTP status from `Error.message` text
5. Frontend API clients branching on raw `response.status` checks for governed status handling instead of the shared response-status helpers
6. Frontend API clients parsing governed success or stream payloads with raw `response.json()`, ad hoc `response.text()` + `JSON.parse(...)`, or per-module `JSON.parse(...)` stream decoding instead of the shared response parsing helpers
7. Frontend API clients normalizing nullable or legacy collection payloads with module-local `|| []`, `?? []`, or ad hoc `Array.isArray(...)` fallbacks instead of shared collection helpers
8. Frontend API clients swallowing non-not-found API failures behind broad `catch { return null; }` fallbacks instead of routing only canonical `404` cases through explicit status checks
9. Frontend API clients coercing governed backend payload fields through module-local scalar helper stacks instead of shared scalar coercion helpers
10. Frontend API clients normalizing governed structured error payloads through module-local helper functions instead of shared error normalization helpers
11. Frontend API clients open-coding parsed non-OK response throwing with `throw new Error(await readAPIErrorMessage(...))` instead of the shared response assertion helper
12. Frontend API clients open-coding governed `assertAPIResponseOK(...); parseRequiredJSON(...)` or `parseOptionalJSON(...)` tandems instead of shared response pipeline helpers
13. Frontend API clients open-coding governed `404 => null` response branches for resource lookups instead of shared missing-resource response helpers
14. Agent and guest metadata clients duplicating the same CRUD transport logic instead of using one shared metadata client
15. AI stream clients duplicating SSE reader, timeout, chunk-splitting, and JSON event parsing loops instead of using one shared stream consumer
16. Monitoring delete and idempotent mutate clients open-coding `404`/`204` allowed-status branches instead of using canonical shared allowed-status helpers
17. Governed frontend API clients open-coding `if (!response.ok) { if (isAPIResponseStatus(...)) throw new Error(...) }` status-to-user-message branches instead of using canonical shared custom-status error helpers
18. Monitoring command-trigger clients open-coding `parseOptionalAPIResponse(response, { success: true }, ...)` success-envelope fallbacks instead of using a canonical shared success-envelope helper
19. Governed frontend API clients open-coding `try/catch` wrappers around `apiFetchJSON(...)` just to map `402` or `404` into `[]`, `{ plans: [] }`, or `null` instead of using canonical shared API-error-status fallback helpers
20. Backend config/settings handlers pointing operator guidance at GitHub `main` docs when the running build already ships that guidance locally under `/docs/`
21. Telemetry preview or reset endpoints drifting from the exact server-owned telemetry runtime contract instead of reusing the same source-of-truth snapshot and install-ID state the background sender uses
22. Shared SSO test or metadata-preview handlers open-coding outbound metadata/discovery URLs, allowing userinfo-bearing HTTP(S) inputs, or rebuilding `/.well-known/openid-configuration` with origin-root string concatenation instead of the shared validated URL helpers before any outbound request
23. AI settings handlers echoing raw provider secrets or testing the wrong provider model: `/api/settings/ai` may expose masked provider-auth presence such as `ollama_password_set`, but backend payloads must never echo stored secrets back to clients, and provider-specific test routes must stay bound to the selected provider's own configured model instead of whichever other provider currently owns the default `model` field
24. `/api/diagnostics` exposing maintainer/admin analytics such as commercial
    funnel, sales funnel, pricing/checkout conversion, or infrastructure
    onboarding telemetry. Customer diagnostics may expose runtime health,
    supportability, and sanitized troubleshooting state; admin analytics must
    stay behind admin-owned metrics routes.

## Completion Obligations

1. Update contract tests when payloads change, including admin verification endpoints such as `POST /api/ai/patrol/preflight` whose response shape (`tool_call_observed`, `duration_ms`, classified `cause`/`summary`/`recommendation`, plus `recorded_at`/`recorded_at_unix` for the cached snapshot) is part of the canonical Patrol diagnostic surface, the `patrol_preflight` snapshot field on `/api/settings/ai` that hydrates the Check Patrol model panel on page load, the auto-trigger contract on `POST/PUT /api/settings/ai` whose handler dispatches preflight in the background only when the change actually moved Patrol transport so routine saves do not write a new `patrol_preflight` snapshot, the startup-seed contract where `NewAISettingsHandler` dispatches the same async preflight after `LoadConfig()` succeeds so the first `/api/settings/ai` poll after a Pulse restart already carries a populated `patrol_preflight` snapshot, and the GET-symmetry contract where `HandleGetAISettings` includes `patrol_readiness` (with the cached-preflight-augmented `tools` check) on the same response that already carries `patrol_preflight`, so the Patrol page picks up classified preflight evidence on first load instead of only after a save; readiness checks may keep stable machine IDs such as `configuration`, but user-facing labels in this payload must say Patrol mode rather than Patrol configuration, and the settings UI must summarize successful diagnostic snapshots as model readiness instead of rendering raw preflight/tool-call wording
   The same diagnostic payload may inform Patrol page setup banners, but the
   Patrol page must present it as setup/model-check state and must not render
   raw preflight or tool-call-observation copy in the header banner.
2. Update frontend API types in the same slice
3. Route runtime changes through the explicit API-contract proof policies in `registry.json`; default fallback proof routing is not allowed
4. Update this contract when canonical payload ownership changes
5. Keep `/api/resources` policy metadata aligned across backend payload tests and canonical frontend resource consumers whenever sensitivity or routing fields change
6. Keep Patrol status payloads explicit enough that the frontend can present blocked runtime state without treating a previously healthy summary snapshot as current runtime truth, and keep Patrol recency semantics explicit in transport by reserving `last_patrol_at` for completed full patrols while exposing any Patrol activity separately through `last_activity_at`
   and the scoped-trigger status payload on that same Patrol status surface, so queued scoped work, busy-mode state, and per-source enablement (`alert` versus `anomaly`) stay transport-backed instead of being inferred by page-local heuristics
   and the event-trigger runtime-block payload on that same scoped-trigger
   status surface, so `event_triggers_blocked`,
   `event_triggers_blocked_reason`, and
   `event_triggers_blocked_message` explain effective runtime policy without
   making the transport look like the operator turned alert or anomaly triggers
   off; default Patrol UI surfaces may use that payload to explain an
   actionable manual-run block, but must not promote a background-only policy
   pause as ordinary operator guidance when manual Patrol still works
   and the split Patrol trigger settings contract, so `patrol_alert_triggers_enabled` and `patrol_anomaly_triggers_enabled` are the canonical AI settings fields while legacy `patrol_event_triggers_enabled` remains a compatibility aggregate rather than the primary control surface
   and the retired quickstart compatibility contract, so `/api/settings/ai` and
   `/api/ai/patrol/status` no longer carry `quickstart_credits_remaining`,
   `quickstart_credits_total`, `quickstart_credits_available`,
   `quickstart_blocked_reason`, or `using_quickstart` as ordinary v6 GA fields,
   and shared handlers must not invent client-authored commercial identity,
   synthetic credits, or quickstart-backed config now that the hosted
   quickstart bootstrap path is retired
   and the provider-first availability rule, so missing managed-model activation
   identity must not block ordinary BYOK/local setup and must not silently
   attempt anonymous bootstrap
   and the hosted model alias retirement rule, so persisted legacy hosted
   quickstart model IDs such as `quickstart:minimax-2.5m` are cleared before
   `/api/settings/ai` responds, instead of leaking stale vendor identifiers or a
   managed-model promise back into governed payloads
   and the AI settings blocked-reason contract, so stale
   `quickstart_blocked_reason` values are cleared or interpreted as
   compatibility metadata when a provider-backed path is active or the
   self-hosted app is routing the operator to provider/local-model setup
   and the public interpretation rule, so compatibility state must not become
   generic hosted AI quota, anonymous Community entitlement, trial CTA,
   account-backed activation support, or full-chat entitlement in normal
   self-hosted v6 GA UI
   and the Patrol autonomy save contract, so Community/free runtime payloads
   may persist only `monitor` autonomy settings through
   `/api/ai/patrol/autonomy`, while `approval`, `assisted`, and `full` return
   the canonical license-required response instead of a generic save failure,
   and Patrol frontend state owners must clamp stale paid autonomy to `monitor`
   and send `full_mode_unlocked:false` before submitting that endpoint when the
   safe-remediation entitlement is not effective. The monitor-only backend path
   must likewise clear stale full-mode unlock state in its response and
   persistence layer instead of preserving paid remediation state through a free
   configuration save
   and the Patrol settings-save readiness contract, so
   `/api/settings/ai/update` may save a selected Patrol provider/model even
   when that model is not ready for tool-backed Patrol execution, but it must
   echo `patrol_readiness` with stable `cause` metadata and execution routes
   must continue to fail closed before model calls. Frontend Patrol settings
   consumers must surface that saved-but-not-ready response as a saved
   configuration issue with the echoed provider, model, cause, and summary
   instead of reporting the successful save as a failed save or hiding the
   readiness blocker behind a generic notification; and the structured
   investigation-record contract, so unified findings may
   expose `investigation_record` only through the shared
   `aicontracts.InvestigationRecord` payload shape, with frontend API types
   and backend contract tests updated in the same slice as any field change.
   That shared shape carries top-level `impact` and `rollback` fields
   alongside the existing `verification` array so paid Patrol responses
   can name the consequence of operator inaction and the undo intent for a
   proposed fix at the record root rather than buried inside per-step fix
   payload, and `trigger.cause` mirrors the Go failure-cause string in the
   TS API client so frontend consumers see the same patrol-failure
   attribution the backend persists. Both `impact` and `rollback` are
   omitempty/normalize-empty and must remain absent rather than fabricated
   when Patrol has not yet populated them. Top-level `impact` also lives on
   `unified.UnifiedFinding` directly, not only on the nested investigation
   record, so detection-time consequence-if-ignored copy authored on
   `Finding.Impact` (for example by the Patrol runtime-failure classifier)
   reaches API consumers even when no investigation record is attached. The
   Finding to UnifiedFinding conversion in `internal/api/router.go` and the
   dedup-merge path in `FindingsStore.Add` must both propagate `impact`
   alongside `description` and `recommendation` rather than dropping it;
   persisted findings created by older binaries must adopt the
   freshly-classified impact text on next re-detection rather than
   preserving the empty value.
   The action audit `result` field carries an optional `verification`
   block (TS `ActionVerificationResult` mirroring the Go type) with
   `ran`, `command`, `output`, `success`, `ranAt`, and `note`. API
   consumers (specifically the Resource Action History on the
   infrastructure detail drawer) must round-trip the verification
   block as the action-audit readback API returns it and render it as a
   distinct outcome row alongside the dispatch `result`. The `command`,
   `output`, and `note` fields are redacted to stable markers at
   persistence/readback boundaries, including migrated legacy rows, so
   consumers must not expect or re-expose raw historical verification
   details. Operators see whether Pulse ran the read-after-write probe
   and whether it confirmed the intended state, not raw command output.
   When `ran=false` (no derivable check, or feature disabled for the
   action class) nothing must be rendered, matching the no-fabrication
   rule.
   The action audit `verificationOutcome` field carries the bounded
   operator-facing verification status and evidence summary. Frontend
   consumers must map `verified`, `unverified`, `failed`, and `unknown`
   through shared action-audit presentation helpers, preserving
   `evidenceSummary` as audit evidence while avoiding route-local
   wording or raw status-token-first copy.
   The patrol-status response (`PatrolStatusResponse`) carries an
   optional `trust` block of type `ai.FindingsTrustSummary` that
   surfaces the trust-metrics snapshot for the Patrol page. The block
   is populated when a patrol service is available and omitted
   otherwise; the JSON keys are `tracked`, `currently_active`,
   `resolved`, `auto_resolved`, `fix_verified`, `fix_failed`,
   `dismissed_as_noise`, `dismissed_as_expected`, `dismissed_as_later`,
   `suppressed`, and `regressed_at_least_once`. The TS API client
   carries a parallel `FindingsTrustSummary` interface in
   `frontend-modern/src/api/patrol.ts` and a `trust?` field on
   `PatrolStatus`. Snapshot semantics, not lifetime totals.
   The Patrol Assistant handoff carries an optional
   `intent` field of type `PatrolAssistantFindingIntent`
   (`'discuss' | 'explain'`) so contextual entry points on the finding
   surface can route through the same handoff builder while seeding
   different leading sentences. The structured handoff context
   (investigation record, operational memory, pending approval,
   proposed fix, next-step action) must be attached identically across
   intents; only the seed prompt's framing varies.
   The TS API client mirrors the
   `previous_resolved_fix_summary` field on both `UnifiedFindingRecord`
   (in `frontend-modern/src/api/ai.ts`) and `Finding`
   (in `frontend-modern/src/api/patrol.ts`), and the store normalizer
   promotes it to the camelCase `previousResolvedFixSummary` on the
   store-level `UnifiedFinding` so finding-card render code can read
   it. Without this mirror, the field stays backend-only and operators
   cannot see "what worked last time" without opening Assistant.
   `unified.UnifiedFinding` also carries a
   `previous_resolved_fix_summary` operational-memory field captured at
   regression time from
   `existing.InvestigationRecord.ProposedFix.Description`. The
   AddFromAI update branch must propagate non-empty
   `previous_resolved_fix_summary` into the existing finding using the
   same overwrite pattern as `description`, `impact`, and
   `recommendation`, and the Finding to UnifiedFinding conversion in
   `internal/api/router.go` must copy `f.PreviousResolvedFixSummary`
   alongside the other operator-facing strings.
   The same shape also carries an optional `remind_at` timestamp (ISO 8601) on both `UnifiedFindingRecord` and Patrol `Finding` shapes. It
   is populated only when `dismissed_reason === 'will_fix_later'` and
   represents the wake-up deadline at which the next re-detection clears
   the dismissal — the operator-facing half of the canonical
   `Finding.RemindAt` contract. The store normalizer promotes it to
   camelCase `remindAt` on `UnifiedFinding`, the Finding to
   UnifiedFinding conversion in `internal/api/router.go` must copy
   `f.RemindAt`, and the AddFromAI update branch must mirror it
   (including clearing on remind-at wake or undismiss) so the dedup
   surface stays consistent with the canonical findings store. The
   Findings panel must visibly preview the deadline at dismiss-confirm
   time and badge dismissed-as-will_fix_later rows with the pending
   remind-at, otherwise the new behavior is invisible until the
   reminder fires.
   The Patrol manual-feedback endpoint surface also includes
   `POST /api/ai/patrol/resolve` (handled by `HandleResolveFinding` in
   `internal/api/ai_handlers.go`) for operator-driven manual resolution
   when the operator has fixed an issue out-of-band. The TS client
   `resolveFinding(findingId)` in `frontend-modern/src/api/patrol.ts`
   must post `{finding_id}` to that endpoint and surface the boolean
   result through `aiIntelligenceStore.resolveFinding` so the operator
   surface gets uniform refresh and error UX with acknowledge / snooze
   / dismiss.
   The same `UnifiedFindingRecord` shape also carries `auto_resolved`,
   the operator-vs-Pulse attribution flag set by the canonical findings
   store. The store normalizer must promote it to camelCase
   `autoResolved` on `UnifiedFinding`, the Finding to UnifiedFinding
   conversion in `internal/api/router.go` must copy `f.AutoResolved`,
   and the AddFromAI dedup-merge path must mirror it onto the existing
   record. The frontend resolution-reason helper must use that flag to
   attribute operator-driven Mark resolved closures as "Resolved by
   you" instead of generic auto-detection copy, while keeping Patrol's
   own fix outcomes (`fix_verified`, `fix_executed`, `resolved`)
   priority because those describe Pulse's actual remediation rather
   than mere auto-detection.
   The canonical Patrol page header copy is owned by the
   patrol-intelligence subsystem and must consume from
   `frontend-modern/src/utils/patrolPagePresentation.ts`. The
   `PatrolIntelligenceHeader` shell on the Patrol API surface reads
   the title, description, and title tooltip from
   `getPatrolPageHeaderMeta()` rather than carrying inline copy, so the
   operator-facing framing of what Patrol owns (watching infrastructure,
   detecting issues, recording findings, and escalating into governed
   investigation/action only when the selected Patrol mode
   allows it) stays tied to the contract instead of drifting between
   hover and inline.
   The Patrol page must not introduce header or workspace trust strips over
   this payload. High-signal trust facts from
   `state.patrolStatus()?.trust` (the `FindingsTrustSummary` block already
   plumbed through the patrol status payload) may feed the primary
   assessment readout, but they remain render-only consumers of the existing
   canonical signal and must not introduce a new fetch path or duplicate data
   plumbing.
   The same page-header recency line also reads
   `PatrolRecencyPresentation.resourcesCheckedLabel` — derived by
   `getPatrolRecencyPresentation` from
   `PatrolRunRecord.resources_checked` and the latest completed run outcome —
   so the operator sees coverage alongside recency without needing a
   new API surface. The label stays optional when coverage is zero, says
   verified only for successful full patrols, and uses neutral checked wording
   for errored full patrols or scoped activity.
   The Patrol Current work empty-state copy uses that same run-history-backed
   coverage truth when reconciling AI summary coverage factors: a successful
   full run with non-zero `resources_checked` suppresses stale
   coverage-incomplete wording, while scoped, missing, zero-coverage, or errored
   runs still allow the coverage caveat to surface.
   and the Assistant finding-context request contract, so `/api/ai/chat`
   payloads carrying `finding_id` may hydrate a structured investigation
   summary from the unified finding, but raw action commands must stay
   out of the persisted prompt and inside governed approval/remediation
   context; the backend may pass that summary as model-only handoff context
   for the current turn and retain it as same-session model context for
   follow-up turns. The backend may also carry structured handoff resource
   references from the same finding into chat execution, but those references
   must hydrate through canonical unified-resource registration before they
   affect session-scoped action validation. If stored for follow-up turns, they
   remain model-context references rather than saved user text or action
   authority, and the runtime must re-resolve them against current canonical
   resources before use. The API/chat boundary may also store the originating
   finding ID as model-context metadata so later turns in the same Assistant
   session can refresh the current unified finding and investigation record
   before model execution; that reference is not lifecycle authority and must not
   be persisted as user text. If that reference no longer resolves through the
   current unified finding store, the API/chat boundary must clear the stored
   handoff context and unpinned handoff-seeded resource scope rather than
   replaying stale investigation state. The chat handoff payload must carry the
   unified finding's lifecycle and recency state, including active/resolved/
   snoozed/dismissed/suppressed status, detection/last-seen/resolution
   timestamps, recurrence/regression facts, and recent lifecycle events, so API
   consumers do not reduce Assistant context to an outdated investigation
   summary. The frontend store boundary must preserve those recurrence facts
   from the shared payload, including `times_raised`, so Patrol presentation and
   Assistant handoff helpers do not infer repeated findings from page-local
   state. The briefing must carry the primary finding's current recency facts,
   bounded evidence snapshot, verification summary, and factual governed action
   artifact metadata before investigation guidance and may carry the latest
   lifecycle event as the current handoff state, while the
   detailed lifecycle list must stay bounded and model-only. Chat execution may
   also resolve root-cause and correlated finding IDs from that current unified
   finding into compact related-finding summaries and structured handoff
   resources. Those related summaries may include the
   related record's current recency and latest lifecycle facts, but those
   related records remain model-only explanation context and must not become
   saved user text, disclosure authority, lifecycle authority, approval
   authority, or execution authority. Chat execution may also hydrate canonical
   resource-policy context for those resources through unified-resource
   resolution and shared policy presentation helpers, but the resulting handling
   guidance is read-only, model-only context and must not become saved user text,
   disclosure authority, or action authority. Chat execution must apply that
   same resource-policy boundary to the assembled product-originated handoff
   text before model prompt injection, including finding briefings and
   lower-level finding/action context, so governed resource identities are
   redacted even when the selected model is local and no provider-bound
   sanitizer will run. Chat execution may also hydrate
   current canonical resource-state context for those resources, including
   compact status, freshness, source-health, metric, incident, and
   governed-capability summaries from the unified-resource model, but that
   snapshot is model-only read-only infrastructure context and must not become
   saved user text, disclosure authority, or action authority. Chat execution
   may also hydrate canonical relationship context for those resources through shared
   unified-resource relationship presentation and parent-edge synthesis, but
   topology context is read-only and must not become saved user text or action
   authority. Chat execution may also hydrate recent changes for those handoff
   resources from the canonical unified-resource timeline, but it must resolve
   product-originated resource references through the canonical unified-resource
   provider before querying timeline changes, with raw handoff IDs used only as a
   compatibility fallback. The resulting context is read-only, model-only
   explanation data and must not become saved user text or action authority. The
   backend may also carry structured
   pending-action and approval references from the investigation record into chat
   execution, and may recover the current live Patrol investigation-fix approval
   for the finding when the durable record has no current approval ID, but those
   references must omit raw action commands, remain model-only review
   context, and leave approval/execution authority with the governed approval
   and remediation APIs. Frontend-visible pending-approval drawer briefings must
   be a presentation of that same safe handoff context: approval ID, status,
   risk, target, requested/expiry timestamps, action label, and approval-flow
   safety posture may be shown, while raw commands stay outside chat prompt and
   context payloads. Chat execution may refresh approval status snapshots for
   those references from the canonical approval store, but that snapshot is
   read-only, org-scoped, and must not expose or infer the raw command. When the
   API handoff builder recovers a live approval, the first model-only finding
   briefing and structured handoff action must use that recovered approval's
   safe lifecycle metadata, request/expiry timestamps, action plan identity,
   approval policy, plan expiry, and dry-run posture as factual action artifact
   metadata instead of falling back to stale investigation-record approval
   posture. When the reference resolves to a governed action plan or
   action audit, chat execution must hydrate the canonical action ID, lifecycle
   state, requester, capability, approval policy, plan expiry, preflight/dry-run
   summary, and terminal success/failure state from the action-audit store so
   API consumers do not mistake the original approval snapshot for current
   action truth. That action-audit snapshot remains model-only review context
   and must not expose raw command text or raw execution output. Frontend
   handoff briefings must derive from the same shared investigation payload
   rather than inventing a second finding-context transport shape.
7. Keep Patrol summary payload consumers aligned on one assessment hierarchy: transport-driven Patrol summary data may show supporting counts and outcomes, but the canonical assessment and verification states must remain singular and not be repeated as a second compact verdict strip. The collapsed readout should describe current operator state and score directly rather than combining reassuring grade labels with active-issue copy; recurrence and trust counters are historical evidence unless the active finding or approval payload marks current operator work. Patrol-owned status, finding, and run-history payloads are sufficient fallback evidence for Current issues when the broader intelligence summary payload is missing, slow, or unavailable. Frontend consumers of the operations-loop projection must present the compact progress label as current work state, not as a duplicate of the selected Patrol autonomy mode, and terminal verified/rejected outcomes with no active finding or pending approval must stay history detail behind `No active work`; native Patrol consumers must not render a separate proof strip solely to expose that history action. A first-party Patrol control starter may render a ready-to-run Patrol step only after subtracting the legacy `proActivationOperationsLoopStarterCount` alias from aggregate Patrol-control starter evidence, so a legacy activation-entry count alone cannot revive the old proof UI.
8. Keep Patrol verification and activity facts unified on one transport-backed secondary status area: when frontend consumers combine Patrol status payloads (`runtime_state`, `last_patrol_at`, `last_activity_at`, `trigger_status`) with run-history transport, the latest run result, activity mix, scoped-trigger state, and circuit-breaker context must read as one supporting explanation beneath Current issues or selected history instead of being re-expanded into a separate full-width status strip plus duplicate summary layers
   and the Patrol runtime-failure run-history contract, so backend payloads,
   persistence adapters, and `frontend-modern/src/api/patrol.ts` preserve
   `error_summary` and `error_detail` whenever an erroring run has structured
   provider, model, tool, context-window, quota, auth, rate-limit, or
   connectivity failure context
   and the Patrol run-history Assistant handoff contract, so frontend
   run-history consumers may pass a bounded `[Patrol Run Context]` block,
   scoped resource references, run outcome/coverage facts, sanitized analysis,
   and structured runtime failure summary/detail as model-only chat context
   while forcing request-local approval-required mode and leaving retries,
   configuration changes, and remediation authority outside the chat payload
   and the main Patrol page composition boundary, so once that governed
   secondary area exists inside the Current issues and history workspace the same payloads must not
   also drive a second page-level status strip elsewhere on the route
   and the Patrol `Details` disclosure rule, so recent changes,
   learned correlations, and policy coverage stay backend and Assistant context
   instead of advertising a parallel Patrol workflow on otherwise healthy fully
   verified states or degraded summary health alone. First-party Patrol page
   consumers must not turn those payloads into a generic Details/supporting
   context panel; selected finding and run records remain the source of truth for
   what the operator can inspect or do next.
9. Keep AI settings setup transport vendor-neutral: `/api/settings/ai/update`
   must accept provider credentials or base URLs without a baked vendor model
   ID, resolve the effective BYOK `model` through the canonical runtime
   provider-catalog policy, and return that resolved model on the same shared
   `/api/settings/ai` payload instead of depending on frontend-supplied model
   defaults.
10. Keep AI settings paid-control fields entitlement-effective at the API
    payload boundary. `/api/settings/ai` and `/api/settings/ai/update` may
    preserve stored autonomous, Patrol auto-remediation, and alert-triggered
    analysis preferences in config, but response payloads must expose only the
    control level and paid Patrol settings currently allowed by runtime
    entitlements.
11. Treat Patrol summary supporting metrics as readouts, not reinterpretations: when frontend consumers derive cards such as active findings, criticals, warnings, or fixes from the canonical payloads, those cards must stay numeric and must not synthesize new assessment labels like `Issues detected` or verification labels like `Partial verification` beneath the primary summary contract
12. Treat active Patrol runtime transport as compatible with factual activity surfaces: when the runtime is currently running, frontend consumers may surface in-progress activity context, but they must not replace the activity strip with a second assessment verdict derived from runtime state alone
13. Treat Patrol recency as a singular transport-driven fact: once header metadata, verification copy, or the findings footer already present the governed Patrol timing context, frontend summary consumers must not derive an extra timing pill from the same payloads inside the primary summary card
14. Treat Patrol findings counts as a singular supporting surface as well: when the summary shell already exposes count cards for active findings, warnings, criticals, and fixes, the primary assessment card must not repeat those same payload-derived counts as secondary badges
15. Treat Patrol schedule and recency as header-owned metadata on the main Patrol page: findings empty-state consumers should not receive or restate `next_patrol_at`, `last_patrol_at`, `last_activity_at`, or interval timing once those transport fields are already presented by the primary header and verification shell
16. Keep recovery payload filters canonical across `/api/recovery/rollups`, `/api/recovery/points`, `/api/recovery/series`, and `/api/recovery/facets`: when `internal/api/recovery_handlers.go` adds a governed recovery filter or display field such as provider-neutral `itemType`, the same normalized transport must land across all four endpoints and the contract tests must pin both outbound payload shape and accepted query aliases in the same slice
17. Keep recovery platform-query vocabulary canonical across that same `/api/recovery/*` surface: operator-facing transport must emit `platform` as the canonical query field, accepted legacy `provider` aliases must remain compatibility-only input, and `internal/api/contract_test.go` must pin that fallback behavior in the same slice as any handler change
18. Keep recovery payload platform vocabulary canonical across that same `/api/recovery/*` surface: point payloads must expose `platform`, rollup payloads must expose `platforms`, and any compatibility `provider` / `providers` aliases must remain secondary fallback fields rather than replacing the shared response model
19. Keep recovery linked-resource vocabulary canonical across that same `/api/recovery/*` surface: points and rollups must expose `itemResourceId` as the canonical linked-resource field, accepted legacy `subjectResourceId` aliases must remain compatibility-only input or secondary payload fields, and the shared proof surface must pin that normalization in the same slice as any handler change
20. Keep recovery external item-reference vocabulary canonical across that same `/api/recovery/*` surface: point and rollup payloads must expose `itemRef` as the canonical external item-reference field, accepted legacy `subjectRef` aliases must remain compatibility-only secondary payload fields, and the shared proof surface must pin that normalization in the same slice as any handler change
21. Keep first-host lookup completion explicit on the shared install-state API
    boundary: when
    `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`
    receives a successful connected-agent lookup result, the canonical install
    flow must expose direct navigation into `/settings/infrastructure` and the
    first visible platform/runtime page rather than leaving the operator on a
    transport-only status readout or reviving the removed `/infrastructure`
    route.
22. Keep the shared first-host detection contract explicit on `/api/state` as
    used by
    `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`:
    the canonical `connectedInfrastructure` projection must stay suitable for
    detecting the first active reporting system during install so brand-new
    operators can receive the first success handoff without typing a hostname
    or agent ID.
23. Keep the shared first-run install-token transport explicit on
    `/api/security/tokens` as used by
    `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`:
    once quick setup has produced the setup handoff credentials, the canonical
    token-creation contract must remain usable immediately from the install
    workspace so the first-host flow can auto-create the scoped install token
    without forcing the operator through a second manual token-generation step.
    Any downloaded first-run handoff instructions emitted by that same shared
    install-state surface must describe that prepared token path consistently
    with the live runtime behavior rather than directing the operator to create
    another install token manually.
24. Keep connected-infrastructure surface vocabulary canonical across the
    shared `/api/state` and reporting/install consumers:
    `frontend-modern/src/types/api.ts` must treat `truenas` as a first-class
    connected-infrastructure surface kind, and connected-infrastructure
    consumers such as
    `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`
    together with
    `frontend-modern/src/components/Settings/useConnectionsLedger.ts` and
    `frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx`
    must preserve the transport distinction between machine-managed surfaces
    (`agent`, `docker`, `kubernetes`) and platform-connections-managed
    surfaces (`proxmox`, `pbs`, `pmg`, `truenas`) instead of collapsing them
    into one uninstall/stop-monitoring model. That same shared payload
    contract must also preserve guest-linked host identity on connected
    infrastructure and removed-host records through `linkedVmId` and
    `linkedContainerId`, so settings consumers can keep the top connections
    ledger scoped to top-level infrastructure without re-deriving guest status
    from names or local heuristics.
25. Keep AI settings payload continuity explicit on the shared `/api/settings/ai`
    surface: `internal/api/ai_handlers.go` and `internal/api/contract_test.go`
    must expose masked provider-auth state such as `ollama_username` and
    `ollama_password_set` without echoing raw stored secrets, and the same
    backend contract must keep provider test routes bound to the selected
    provider's configured model instead of whichever other provider currently
    owns the default `model` field.
    Anthropic provider readiness on this shared settings payload is API-key
    backed only: legacy OAuth fields may be echoed as cleanup state, but they
    must not count toward `configured`, `anthropic_configured`,
    `configured_providers`, model listing, provider tests, or runtime provider
    construction. `/api/ai/oauth/start` and `/api/ai/oauth/exchange` fail closed
    with `unsupported_anthropic_oauth`, `/api/ai/oauth/callback` must not save
    tokens, and `/api/ai/oauth/disconnect` may clear stored legacy tokens.
    The Ollama provider payload also owns `ollama_keep_alive` as the canonical
    request keep-alive field: GET and update responses must expose the
    normalized configured value, update requests must reject malformed values,
    an empty string means omit Ollama `keep_alive`, and stored provider secrets
    remain masked independently of that runtime option.
    Discovery scheduling is part of that same AI settings payload contract:
    settings saves from `frontend-modern/src/components/Settings/useAISettingsState.ts`
    must send `discovery_enabled` and `discovery_interval_hours` together as
    explicit form truth, and the backend must persist a provided
    `discovery_interval_hours: 0` as manual-only rather than replacing it with
    the automatic-scan default. Manual command-backed discovery is available
    only while `discovery_enabled` is true and must be admin-gated at the
    backend. When Discovery is disabled, direct known-resource refreshes,
    `/api/discovery/run`, and governed `pulse_discovery` refreshes must fail
    closed before any agent command dispatch. The API progress payload must
    describe discovery evidence analysis rather than presenting background work
    as a visible Pulse Assistant chat session.
26. Keep shared AI runtime reads centralized on that same governed contract:
    `frontend-modern/src/stores/aiRuntimeState.ts` is the canonical frontend
    read owner for `/api/settings/ai` and `/api/ai/models`. AI-owned consumers
    such as `frontend-modern/src/features/patrol/usePatrolIntelligenceState.ts`,
    `frontend-modern/src/components/AI/Chat/index.tsx`, and
    `frontend-modern/src/components/AI/AICostDashboard.tsx` must reuse that
    shared store for read-side runtime truth, while
    `frontend-modern/src/components/Settings/useAISettingsState.ts` remains
    the write-side settings owner. Non-AI settings surfaces such as
    `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`
    must not probe `/api/settings/ai` just to gate assistant affordances.
    AI-owned refresh actions may still force a shared reload or sync that
    store after an owned settings mutation, but they must not reintroduce
    page-local mount loops that fetch `/api/settings/ai` or `/api/ai/models`
    separately for chat, Patrol, and cost/budget views.
    The `/api/ai/models` catalog must also include backend-owned
    direct-provider fallback entries for configured DeepSeek V4 paths when
    live provider catalog reads fail, so Patrol and Assistant model selectors
    keep saved `deepseek:` selections visible without inventing page-local
    catalog entries or falling back to another provider's model.
27. Keep API-backed first-target onboarding canonical on that same shared
    infrastructure-settings boundary:
    `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`,
    `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`,
    `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`,
    `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`, and
    `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx` must
    present TrueNAS and other API-backed platforms as Platform connections-first
    onboarding rather than as dedicated unified-agent install profiles. The
    shared host-install contract may guide operators through the first
    agent-managed host, but alternate CTAs and setup-completion guidance must
    route API-backed first systems through the canonical infrastructure
    onboarding contract at `/settings/infrastructure?add=pick`, while
    agent-managed first hosts use `/settings/infrastructure?add=agent`. The
    infrastructure workspace may consume those onboarding query params and
    normalize the browser back to `/settings/infrastructure`, but first-run
    callers must not fall back to the retired `/settings/infrastructure/install`
    or `/settings/infrastructure/platforms` deep links.
28. Keep the shared agent-download fallback transport pinned to published
    release lineage. The served install-script endpoints (/install.sh,
    /install.ps1) have no GitHub fallback at all (see item 8); they serve the
    locally bundled agent installer or fail closed. The remaining GitHub fallback
    is the agent-BINARY download proxy: `internal/api/unified_agent.go` and
    `internal/api/contract_test.go` must only map stable tags or explicit RC
    prerelease tags without build metadata to GitHub release assets; dev
    prereleases such as `v6.0.0-dev`, git-described `+git...` builds, and other
    unpublished prerelease identifiers must fail closed on that API boundary
    instead of generating fake release URLs from a local runtime version string.
29. Keep local trial-start transport retired from self-hosted v6 GA runtime
    paths. `POST /api/license/trial/start` must not be registered as an ordinary
    in-app acquisition endpoint; browser API clients, route inventory, demo
    mode, and feature gates must all treat it as absent. The retired
    `/auth/trial-activate` self-hosted return path must also stay absent from
    the ordinary router and settings UI; signed hosted entitlement leases may
    refresh cached hosted/cloud entitlement state, but they must not create a
    local Pro trial acquisition callback. Entitlement payloads may retain
    `trial_eligible` and `trial_eligibility_reason` as compatibility fields,
    but ordinary self-hosted responses leave eligibility false and the reason
    empty; active trial state is represented by `subscription_state`,
    `trial_expires_at`, and `trial_days_remaining` only.
30. Keep `/api/security/dev/reset-first-run` transport-backed and genuinely
    unauthenticated: when the dev reset route clears first-run auth it must
    also clear any env-backed auth state that feeds `/api/security/status`, so
    the status payload flips `hasAuthentication` to `false`, preserves
    `bootstrapTokenPath`, and allows browser-owned first-session proof to
    re-enter the real setup wizard instead of silently falling back to an
    already-authenticated app state. That recovery transport may expose the
    bootstrap token file path, but it must not emit the token value into
    automatic runtime logs.
31. Keep shared SSO test and metadata-preview transport fail-closed: SAML
    metadata URLs and OIDC issuer URLs must reject non-HTTP or userinfo-bearing
    inputs before any outbound request is attempted, and OIDC discovery must
    append `/.well-known/openid-configuration` beneath the configured issuer
    base path instead of resetting to the origin root.
32. Keep config-archive import reloads fail-closed on the shared API/runtime
    boundary. `internal/api/config_export_import_handlers.go`,
    `internal/api/contract_test.go`, and adjacent config/runtime helpers must
    tolerate absent notification managers and other optional runtime managers
    after a successful import-triggered reload request, returning a controlled
    API outcome instead of panicking or leaving browser-visible state half
    rewired.

## Current State

Manifest-backed Patrol finding lifecycle schemas are the API source of truth
for Assistant provider-tool optionality as well as MCP/API discovery. Legacy
Assistant runtime paths may project those schemas into provider-tool JSON, but
they must not re-require optional lifecycle fields such as `resolution_note` or
dismissal `note` after `/api/agent/capabilities` and the Patrol lifecycle API
declare them optional.
Legacy native Assistant utility provider aliases for `run_command`, `fetch_url`,
and `set_resource_url`, plus their provider JSON schemas, are part of the same
shared API/runtime contract through
`agentcapabilities.LegacyAssistantUtilityProviderTools`. The older native
Assistant service may keep exposing those compatibility aliases, but it must not
own separate inline provider schema maps or local tool-input argument strings
while the registry-backed execution path continues to converge.
Native Assistant registry tools that expose Patrol finding lifecycle actions
must use the same `agentcapabilities` argument constants for `finding_id`,
`resolution_note`, `reason`, and `note`, so provider tool schemas, execution
maps, `/api/agent/capabilities`, and MCP projections stay on one API vocabulary.

That same shared API boundary now also owns the default-org token scoping
rule and instance-wide notification settings propagation. A token explicitly
bound to one or more organizations is scoped to those organizations only and
must not implicitly reach the default org's data; authenticated users and
legacy unbound tokens retain default-org access, and binding `default`
explicitly still grants it (pinned by
`TestContract_OrgBoundTokenIsScopedAwayFromDefaultOrg`). System settings that
shape notification transport (webhook private-target allowlist, public URL)
are stored instance-wide and must be applied to every live tenant monitor's
notification manager on update and reload, and inherited by tenant monitors at
creation, instead of reaching only the request's org or the default monitor.

That same notifications boundary also carries the webhook signing-secret
payload contract: webhook management payloads may include an optional
`signingSecret` that enables HMAC-signed deliveries, the list API must mask a
configured secret with the shared redaction placeholder instead of returning
it, and an update that echoes the masked placeholder must preserve the stored
secret rather than overwriting it, matching the header and custom-field
secret-handling rules above.

Proxmox setup bootstrap now includes a non-destructive Audit/Repair path in the
generated PVE script and existing-source setup guide. That path audits token
presence, token expiry, Pulse-managed token drift, and expected ACLs, then
reapplies safe permissions without replacing the stored API token; full
Install/Configure remains the explicit rotation and re-registration path.

VMware vSphere phase-1 inventory now reaches the product through shared API
contracts rather than provider-local read routes. `/api/vmware/connections`
owns saved vCenter connection health and observed counts, while canonical
`agent`, `vm`, `storage`, and `network` resources flow through
`/api/resources`, `/api/resources/stats`, `/api/state`, and shared Assistant
mention payloads. Host-shaped presentation also shares the
`ResourceRegistry.ListForPresentation` / `CoalescePresentationHostResources`
boundary so state and resource list responses agree without bypassing
registry-owned report exclusions.

TrueNAS platform-connections responses treat native VMs and network shares as
first-class observed contribution facets alongside systems, pools, datasets,
apps, disks, and recovery artifacts. The frontend TrueNAS API client must
normalize and preserve those fields for settings and platform handoffs instead
of collapsing them into generic app, storage, or runtime counts.

The frontend `Node` API projection now carries optional `networkIn`,
`networkOut`, `diskRead`, and `diskWrite` fields so table consumers can align
Proxmox host rows with workload I/O columns. These fields are a read-model
extension over existing resource telemetry; they must remain optional until the
backend transport contract explicitly guarantees them for every node source,
and consumers must tolerate absence without inventing a second API shape.
The AI settings contract (`internal/api/ai_handlers.go`) now carries the
per-rule alert-trigger policy for scoped patrols. The response always projects
`patrol_alert_trigger_min_severity` (`warning` | `critical`, normalized from the
critical-only default) and a non-nil `patrol_alert_trigger_types` allowlist
(empty = all types). The update request accepts both as optional pointer fields;
the handler rejects any `patrol_alert_trigger_min_severity` other than `warning`
or `critical` with `400`, and lowercases, trims, drops blanks, and de-duplicates
the types allowlist before persistence so the stored shape is canonical.
Renaming the operator-facing "Alert-Triggered Analysis" toggle to "Container
Update Risk" in `PatrolIntelligenceHeader.tsx` is presentation-only and carries
no wire delta: the persisted settings keys stay `alert_triggered_analysis`,
`patrol_alert_triggers_enabled`, and `patrol_alert_trigger_min_severity`. The
operator label and the API field name are deliberately decoupled, so new copy
must not rename the transport fields to chase the display string.
The shared metrics-history API also treats `metric=temperature` as a canonical
agent/node chart metric for Proxmox node drawers. `resourceType=agent` may serve
the current host-agent CPU package temperature as a live fallback when persisted
history is still cold, while `resourceType=node` may serve the Proxmox node
temperature fallback from the same `/api/metrics-store/history` response shape.
The transport remains the existing metrics-history payload; it must not add a
Proxmox-only chart endpoint or drawer-local temperature response contract.
Host sensor summaries also carry optional `thermalState` for platforms such as
macOS that expose thermal pressure but not stable Celsius sensor readings. API
payloads must keep that state separate from `temperatureCelsius`; clients may
display pressure and throttling limits, but must not synthesize a temperature
metric or table value from pressure-only payloads.

`aicontracts.Finding` (the shape Patrol hands the investigation
orchestrator) carries optional `OperatorContext` and
`OperationalMemory` projections. The router-side wire-up populates
the operator-state projection with `NeverAutoRemediate` alongside
`IntentionallyOffline` and the maintenance-window block, and the
investigation runtime in `internal/ai/patrol_findings.go` attaches
the projection to the Finding before calling the orchestrator. This
is the one in-process write path that decides what the orchestrator
sees about operator commitments; all consumers (in-process Patrol,
external Claude Code, and MCP-speaking clients) read the same enriched shape.

`/api/agent/events` is the agent SSE stream — the substrate piece
that closes the agent-paradigm triangle (discovery + bundled reads +
push notifications). Agents subscribe once and receive real-time
notifications: `finding.created` when a new finding is raised
(suppressed when the finding was auto-dismissed by operator-state),
`approval.pending` when a remediation request enters
`StatusPending` and is waiting on operator decision (carries
`approvalId`, `resourceId`, target tuple, `riskLevel`,
`requestedBy`, `requestedAt`, `expiresAt`, plus `command` only for
session callers or API tokens that also carry `ai:execute`; plain
`monitoring:read` tokens receive `commandRedacted:true` — full
detail stays behind `/api/approvals/{id}`, the event is a
doorbell),
`action.completed` when an action audit reaches a terminal state —
Completed, runtime-Failed, or refused-before-dispatch (refusals
carry stable error-token prefixes such as `plan_drift:`,
`action_plan_expired:`, `action_dry_run_only:`, or
`resource_remediation_locked:` verbatim on `errorMessage` so agents
branch on the prefix rather than parsing human text; successful
dispatches carry a `verification` block — the agent-stable
projection of the broker's read-after-write probe — with `ran`,
`success`, `command`, `note`, `ranAt` so agents close the
"did it actually work?" loop without a follow-up audit fetch;
action command disclosure follows the governed `ai:execute` rule, while
verification command/note details in the action-audit projection remain
stable redaction markers; stable error-token prefixes remain verbatim
for agent branching;
refused dispatches omit `verification` because the probe never
runs) — and `heartbeat` every 15 seconds so an idle connection
can confirm the stream is alive. Each event carries a monotonic ID
so agents can dedupe and reason about ordering. Heartbeats are
stream-local keepalives written directly to the connected response,
not global broadcaster publishes, so concurrent subscribers do not
multiply heartbeat fan-out for one another. The broadcaster drops
real published events for slow subscribers rather than blocking the
publish path — publishers (the patrol findings runtime, the
approval store's post-create callback, the executor's
post-completion callback, and the API action-execution terminal
publisher) cannot stall on consumer slowness. The
stream sits behind `monitoring:read` and runs through the same auth
path as the rest of the external-agent surface; the capabilities manifest declares the stream
under `subscribe_events` so external agents discover it without
out-of-band documentation.
The stable event kind strings and the stream-local transport markers are owned
by `internal/agentcapabilities/events.go`; API constants may expose typed
aliases, but API handlers, manifest descriptions, MCP adapters, and probes must
not re-declare the vocabulary independently.

The approval store exposes `SetOnApprovalCreated(cb)` so the API
layer can install a fire-and-forget callback that runs after every
successful `CreateApproval`. The callback fires on its own
goroutine against a snapshot of the request, keeping the approval
hot path off any consumer's slowness and avoiding any chance of the
consumer reentering the store under the held write lock. The router
wires the callback through `AIHandler.SetApprovalCreatedCallback`,
which both installs the callback on the active store and re-installs
it whenever `ensureApprovalStore` builds a new store for a different
data dir. `ApprovalRequest.CanonicalResourceID()` returns the
canonical `type:id` form the agent SSE bridge stamps on
`approval.pending` events, derived from `(TargetType, TargetID,
TargetName)` via the same rule the store uses internally — agents
match the result against canonical resource ids elsewhere in Pulse
without depending on a `Plan` being populated.

`PulseToolExecutor` exposes `SetOnActionCompleted(cb)` as the
parallel seam for action-audit terminal states. Every dispatch
that reaches `ActionStateCompleted` or `ActionStateFailed` —
including refused-before-dispatch refusals routed through the
plan-drift and operator-lock guards in `executeCommandWithAudit`,
the per-execution result lane shared by `executeCommandWithAudit`
and `executeNativeActionWithAudit`, and the recovery branch when
state-machine normalization fails — calls
`publishActionCompleted(record)`, which dispatches the callback on
its own goroutine after the audit record has already been
persisted. The callback is installed once per chat-service per
org through `wireAIChatDependenciesForService` against
`chatService.GetExecutor()`, so multi-tenant chat-service rebuilds
re-wire the bridge without coupling the tools package to the api
package. API-owned execution installs `ResourceHandlers` on the
same `PublishActionCompletedRecord` projector so direct
`/api/actions/{id}/execute` completions and stale-plan refusals
emit the identical payload shape. `action.completed` payloads preserve the canonical
refusal-token prefixes (`plan_drift:`, `action_plan_expired:`,
`action_dry_run_only:`, `resource_remediation_locked:`) on
`errorMessage` verbatim so agents branch on the stable code
without parsing human messages.

`/api/agent/capabilities` is the discovery document for Pulse
Intelligence's external-agent surface. The manifest declares each
agent-consumable
capability with stable name (snake_case agent identifier),
description, category (`context` / `provisioning` /
`operator-state` / `finding` / `action`), HTTP method + path,
required auth scope, action mode (`read`, `mixed`, `write`),
approval policy (`scope_only`, `action_plan`), response shape name,
optional JSON Schema `inputSchema`, and the closed set of stable
error codes the response may carry. Agents fetch this once at startup
to learn what's available; the `cmd/pulse-mcp` adapter reads the
manifest's Pulse MCP `surfaceToolContracts` entry to register MCP tools, and
any future adapter (HTTP-API SDK, Claude-Code custom toolkit, etc.) reads the
same manifest surface contract the same way:
manifest-driven projection is the substrate's deliberate single source
of truth for "what can an agent do here?". The canonical manifest declaration,
manifest wire type, registry tool-name vocabulary, external tool projection
helpers, typed capability lookup error, request/response capability resolver, call route/body
projection helpers, path-parameter substitution helpers, JSON Schema
object-envelope helpers, manifest HTTP fetch, authenticated agent request
builder, capability HTTP executor, named capability HTTP executor,
named request/response capability HTTP executor, shared MCP
request decoder, line-delimited request serving, notification response policy,
stable JSON-RPC encoding, manifest-backed tool server,
surface-tool contract lookup, surface-filtered tool projection and execution,
tool-server method dispatcher,
initialize-result builder, MCP resource URI projection, context-backed
resources/list and resources/read projection, stable error-envelope formatter,
MCP tool-result text/marker interpreter, provider tool-result constructor and
context projection,
approved-action argument handling for replayed tool execution, internal
tool-argument filtering before public body projection, SSE record parser,
SSE-to-MCP notification bridge, shared tool-governance defaulting, and
disabled-control guidance live in
`internal/agentcapabilities` and are consumed by Assistant tool governance, the
API, `cmd/pulse-mcp`, and the in-repo `cmd/agent-probe` reference client; new
Pulse-native or external agent surfaces must reuse that contract instead of
defining local manifest structs, local tool-description builders, local
request/response filters, local argument-to-body shapers, local path
placeholder parsers, local `/api/agent/capabilities` fetchers, local API-token
header spelling, local named capability execution, local MCP initialize-result
builders, local MCP manifest-backed tool handlers, local MCP request decoding,
local MCP line-framing loops, local JSON-RPC encoders, local MCP notification
response checks, local MCP method dispatch, local MCP
result interpretation, local provider tool-result struct assembly, local
request/response capability body readers, local non-2xx error formatting,
local MCP resource URI builders, local resources/list or resources/read
projectors, local SSE scanners, local SSE-to-MCP notification bridges, local
tool-governance defaulting, local approved-action argument keys, local
disabled-control guidance, or local schema wrappers. Native Assistant registry
declarations must also reuse that API identity contract: every registry
`Tool.Definition.Name`, including `pulse_summarize` and Patrol runtime tools,
must come from `internal/agentcapabilities/tool_names.go` so MCP and future
external-agent adapters project the same tool identities that Assistant
executes. The manifest-backed MCP
tool server must receive the whole manifest and derive its surface-specific
tool projection, execution allowlist, and initialize surface-contract
instructions from that single value; adapters must not pass only a capability
slice and then maintain a parallel surface relationship beside it. The manifest itself is unauthenticated
and cacheable (`Cache-Control: public,
max-age=300`) — declared in the router's `publicPaths` list so
the global auth middleware does not gate it; the underlying
capabilities keep their own auth scopes. The manifest's
`unauthenticated` posture is the chicken-and-egg fix: an agent
that does not yet have a token must still be able to introspect
Pulse to learn how to ask for one. Adding a capability is a
deliberate "this is part of the external-agent surface" commitment — the
manifest is hand-authored rather than auto-generated so contract
decisions (which capabilities are agent-stable, what the stable
error codes are, what category and governance posture each belongs to) cannot
drift behind code changes. Capability `scope` values are not part of that
hand-authored vocabulary: they must come from `pkg/auth` via manifest-local
aliases, matching the constants API authorization uses, rather than literal
strings that can drift from the enforced route scopes.
Every manifest capability's method, path, and advertised scope must also be
proved against the live router by projecting a concrete request through
`agentcapabilities.ProjectCapabilityCall` and requiring a wrong-scope API token
to fail with the manifest's advertised missing scope. The proof is intentionally
router-owned rather than table-owned: manifest placeholder names may stay
agent-facing, but the concrete projected route must reach the API boundary that
enforces the same scope.
API-owned Pulse Assistant dependency seams must also keep MCP as an adapter
boundary, not an internal name for the native Assistant runtime. `AIService`
provider setter types, AI handler pass-throughs, router chat dependency wiring,
AI settings control-refresh callbacks, direct approved-tool replay into chat, and the native tool adapter family must
use Assistant/Pulse Intelligence terminology, while `MCP` remains valid only
for the external adapter, shared MCP wire/result contracts, and explicitly
deprecated API compatibility aliases. The cross-repo auto-fix dependency shape must
publish `ApprovedAssistantToolExecutor` / `AssistantToolExecutor` as the native
approved-tool execution contract. Callers must use
`AIAutoFixHandlerDeps.ResolveApprovedAssistantToolExecutor`, and that resolver
returns only the native Assistant executor; MCP-named executor compatibility is
not part of this boundary. Current public API relay wiring must populate only
`AssistantToolExecutor`.
API-visible Assistant runtime readiness and lifecycle messages must name the
first-party surface as Pulse Assistant rather than the legacy generic `Pulse AI`
runtime.
API-visible shared Intelligence messages such as profile-suggestion
unavailability, AI settings persistence failures, Patrol preflight copy, and
remediation-impact logs must not narrow the shared Assistant/Patrol substrate
back to `Pulse Assistant settings` or generic `Pulse AI`; they must use Pulse
Intelligence for shared runtime availability and Provider & Models for the
shared provider/model settings surface.
Patrol finding capability scopes are API-authorization-owned, not
adapter-owned. `list_findings`, `acknowledge_finding`, `snooze_finding`, and
`dismiss_finding` must match the relay/mobile runtime route inventory, and
`resolve_finding` must match the direct `POST /api/ai/patrol/resolve` route.
Those Patrol finding review and lifecycle routes are gated by `ai:execute`, so
the manifest and MCP projection must not advertise monitoring scopes for them
while the router enforces AI execution authorization.
Capabilities that replace operator-state, mutate Patrol finding lifecycle, or
participate in the action plan/decision/execute loop must publish typed
`inputSchema` definitions for their exact top-level tool arguments, including
path parameters such as `resourceId` / `actionId`, full-replacement flags such
as `intentionallyOffline` / `neverAutoRemediate`, and stable enums such as
criticality, dismissal reasons, or approval outcomes. External adapters may
project those schemas directly, but must not replace them with adapter-local
body wrappers or prose-only argument hints. Shared manifest schemas use the
shared strict object envelope; adapter fallback schemas use the same helper with
explicitly permissive additional properties when the endpoint contract is only
partially known.

Infrastructure onboarding is part of that same manifest contract,
not an MCP-only shim. The provisioning category projects the
canonical `/api/config/nodes` node lifecycle and `/api/discover`
LAN-discovery routes as `list_nodes`, `add_node`, `update_node`,
`remove_node`, `test_node_credentials`, `test_node_connection`,
`refresh_node_cluster_membership`, and `discover_lan`. Those
capabilities must carry typed schemas for path/body arguments and
must preserve the underlying settings scopes. Listing configured
sources returns redacted credential state only; token or password
secret values may be accepted on write/test requests but must not
come back through `list_nodes` or any manifest metadata.

The external-agent surface uses one error-envelope shape across every
endpoint — `{"error": "<stable_code>", "message": "<human>",
"details"?: {"<field>": "<reason>"}}` — written via
`writeJSONError` (no details) or `writeJSONErrorWithDetails`
(with field-level reasons). Agents branch on the `error` field
(snake_case stable codes); the `message` field carries
human-readable text agents can surface to operators without
losing the code; the optional `details` map carries field-level
failure reasons on validation errors so agents do not parse
human text to identify which field went wrong.
The envelope type, constructor, and parser live in
`internal/agentcapabilities/errors.go`; API handlers emit that shared type and
external-agent clients/probes parse it there instead of carrying local envelope
structs.
The same file owns the `AgentErrCode*` constants for every
capability-specific agent error code. The canonical manifest and the
agent-surface handlers must reference those constants rather than duplicate
string literals, so Assistant, MCP clients, probes, and API handlers share one
branching vocabulary.

There are two layers of stable codes. First, **capability-specific
codes** are declared per capability in the manifest's `errorCodes`
list and represent the closed set of failure modes that
capability's handler emits. Today these are:
`resource_not_found` (`get_resource_context`),
`operator_state_not_set` (`get_operator_state`),
`operator_state_invalid` (`set_operator_state`);
`invalid_finding_request`, `finding_not_found`,
`finding_action_not_allowed`, and `patrol_unavailable`
(`acknowledge_finding`, `snooze_finding`, `dismiss_finding`,
`resolve_finding`); and the action-governance codes declared on
`plan_action`, `decide_action`, and `execute_action`.
Adding or renaming one requires both the handler emission and the
manifest declaration to move together — drift between them is a
contract regression. Second, **cross-cutting codes** apply to
every authenticated endpoint and are emitted by the
multi-tenant / auth middleware rather than any specific
capability handler: `invalid_org` (400) when the org id resolves
to garbage, `org_suspended` (403) when the resolved org has been
suspended, and `access_denied` (403) when the org RBAC denies the
request. Agents must accept these alongside any capability's
declared codes; the manifest deliberately does not duplicate them
on every entry.

The external-agent substrate is end-to-end exercised by two paired tests in
`internal/api/agent_substrate_e2e_test.go`. The first test boots
the full router stack and walks discovery → triage → depth: fetch
the manifest unauthenticated, call `/api/agent/patrol-control/status`
authenticated, call `/api/agent/fleet-context`
authenticated, drill into `/api/agent/resource-context/{id}`, and
confirm the SSE stream path is gated rather than absent. The
second test walks the operator-state intent loop end-to-end:
GET-not-set → PUT-valid → GET-round-trip → PUT-invalid →
DELETE → GET-not-set-again → DELETE-idempotent, asserting the
manifest's declared error codes (`operator_state_not_set`,
`operator_state_invalid`) reach the wire under the canonical
`error` key and that the URL canonical id wins over any
body-supplied id (preventing scope-confusion writes). Together
the two tests are the substantive proof for the external-agent surface —
read, write, push — as one substrate.

The companion worked example lives at `cmd/agent-probe/main.go` —
a small standalone Go program that walks the same discovery →
triage → depth → push flow against a running Pulse instance.
The probe avoids API handler and AI-runtime imports; it reuses only the tiny
`internal/agentcapabilities` wire/projection/HTTP contract so the in-repo
reference client cannot drift from the manifest shape, named capability HTTP
execution rule, request/response body-return helper, path projection
semantics, API-token header convention, or stable error envelope behavior; it
also consumes the shared SSE subscription
transport, record parser, and actionable-record filter so the push walkthrough
uses the same event-stream request, status, framing, and transport-event
filtering rules as the MCP bridge. External agents can define the same JSON shape from the
manifest or a generated client. Anyone building MCP servers, Claude Code
integrations, or custom agents on top of Pulse can read it top-to-bottom to see
how the substrate fits together. The probe resolves capabilities and paths from
the manifest rather than hardcoding them, so discovery moves automatically
follow.

A second adapter lives at `cmd/pulse-mcp/main.go` — a minimal
MCP (Model Context Protocol) server that exposes Pulse Intelligence's
external-agent substrate as MCP tools so Claude Desktop, Claude Code,
OpenCode, and other MCP-speaking clients can drive Pulse
natively. The adapter is the test for whether the substrate's contracts
were really cheap to project: each MCP tool is a manifest capability allowed
by the manifest-owned Pulse MCP surface tool contract. `cmd/pulse-mcp` must use the shared
`internal/agentcapabilities` projection helpers for the complete
surface-filtered request/response tool-list projection, tools/call params
decode, surface-filtered capability lookup translation, initialize-result
construction through the target surface affordance contract, manifest fetch, neutral
capability tool HTTP execution (including path placeholder substitution,
argument-to-route/body projection, agent request construction, and capability
HTTP dispatch), MCP JSON-RPC request decoding, notification response policy,
manifest-backed MCP tool-server handlers, MCP JSON-RPC method dispatch, MCP
result wrapping, MCP resource URI projection, context-backed `resources/list`
and `resources/read` projection through `get_fleet_context` and
`get_resource_context`, shared result text projection at execution sites,
shared result text/marker interpretation, and the shared SSE-to-MCP notification
bridge. When the manifest supplies an `inputSchema`, the shared helper forwards
it verbatim; otherwise it derives a minimal fallback from path placeholders and
method (path `{name}` segments become required string properties;
non-GET/DELETE tools accept a `body` object). Adding a request/response
capability to the manifest-owned Pulse MCP surface tool contract automatically
extends the MCP tool surface without changes in the adapter. Adding a raw
manifest capability without adding it to that surface contract must not alter
MCP `tools/list` or `tools/call`. Pulse Assistant remains the first-party
in-app surface over the same
governed contracts; MCP is the external-agent adapter and must not grow a
parallel action pipeline.
MCP resource browsing follows that same rule: the adapter may advertise
resources when the target surface affordance allows resources and the manifest
carries the canonical context capabilities, but `resources/list` is only a
fleet-context projection and `resources/read` is only a resource-context
projection. Resource projection must enter through
`ManifestSurfaceResourceCapabilities`,
`ListMCPManifestSurfaceResourcesHTTP`, and
`ReadMCPManifestSurfaceResourceHTTP`; raw-capability resource helpers must not
define a second MCP resource surface. Prompt advertising and `prompts/get`
follow the same surface affordance gate before the shared workflow-prompt
catalogue is projected; initialize and `prompts/list` must use
`MCPManifestSurfacePromptProjectionSupported`, and `prompts/get` must use
`GetMCPPromptFromManifestSurface`, so disabled prompt affordances cannot be
re-enabled by raw workflow prompt presence.
The global `pulse_operations_loop` prompt is a Patrol work API manifest contract as much
as an Assistant starter: it is advertised only when the manifest includes fleet
operations-loop status, fleet context, resource context, finding list, action
planning, action decision, action execution, and finding resolution
capabilities. Its rendered instructions
must keep external agents on the same governed Patrol work flow as Pulse
Assistant by explaining Patrol evidence in context, planning first, asking for
approval or rejection only when the returned policy requires a decision,
executing when policy allows, re-reading Patrol status, context, and findings
for verification, and resolving the finding only after the outcome has been checked.
Resource URIs use the shared `pulse://resource/<resource-id>` shape and must
not become an adapter-local resource registry.
`subscribe_events` is
intentionally excluded — SSE streaming doesn't fit the
request/response tool shape; agents that need real-time push
consume the SSE stream directly. The adapter speaks JSON-RPC 2.0
over stdio with line-delimited framing, and preserves the
substrate's stable error envelope (`{"error": "code", "message":
"..."}`) verbatim through the shared `HTTPCallResponse` to MCP
content-and-isError result helper so agents on the MCP side branch on the same
stable codes without the adapter reinterpreting response bodies or statuses.
Optionally, with `--emit-notifications`, the adapter also
uses the shared initialize-result builder to advertise
`experimental.pulseNotifications.kinds`, then uses the shared event bridge to
subscribe to `/api/agent/events` and write each non-transport SSE event as a
JSON-RPC notification on stdout
(`notifications/finding.created`, `notifications/approval.pending`,
`notifications/action.completed`); the notification's `params` is
the SSE `data` payload verbatim so MCP-bound autonomous agents
react to pushes without holding a separate HTTP connection. The
flag is off by default because not every MCP client surfaces
server-initiated notifications; transport plumbing
(`stream.connected`, `heartbeat`) is filtered through the shared
agent-capabilities event vocabulary.

The integration guide for `cmd/pulse-mcp` lives at
`cmd/pulse-mcp/README.md`. It carries the canonical reusable MCP runtime facts,
OpenCode's native top-level `mcp` config shape for `opencode.json` /
`opencode.jsonc`, the common `mcpServers` block for Claude-style clients, the
env-var contract for the API token, manifest-derived generated scope,
surface-filtered request/response tool, workflow prompt, and stable error-code inventories, the
context-backed MCP resource browsing behavior, and the documented limitations
(`subscribe_events` is not a callable tool; manifest fetched once at startup).
`scripts/generate-pulse-intelligence-docs.go` owns the generated public
overview and README scope/tool/prompt/error-code blocks and must use
`internal/agentcapabilities` projections rather than a second catalogue. README
tool and capability-specific error-code blocks must use the same manifest-owned
Pulse MCP surface contract as runtime `tools/list` and `tools/call`.
Manifest-owned capability titles, workflow prompt definitions,
manifest-owned `workflowPrompts` catalogue selection, MCP tool/prompt title
projection, presentation kind hints, Patrol work availability gating, and
prompt argument validation are owned by the neutral
`ManifestPulseWorkflowPrompts` / `ProjectPulseWorkflowPrompts` /
`BuildPulseWorkflowPromptFromManifest` contract; MCP keeps only protocol wire
projection and surface-specific resource URI hints.
documentation-local capability table. External maintainers wiring
Pulse into their MCP-speaking client read that document, not the
package source.
`docs/AGENT_SUBSTRATE.md` is the current high-level operator and integrator
summary for the same substrate. It must reflect the in-app
`Settings -> API Access -> Agent integrations` surface, both the OpenCode
native `opencode.json` / `mcp` shape and the common `mcpServers` block, and the
published `install-mcp.sh` / `install-mcp.ps1` release path; it must not
preserve old gap copy claiming there is no Pulse settings surface, no config
snippet, or no `pulse-mcp` distribution path.
`docs/releases/AGENT_PARADIGM.md` is the release-facing projection of the same
contract. It must frame `cmd/pulse-mcp` as a generic MCP adapter for
MCP-speaking clients, include OpenCode-native `mcp` setup alongside the common
`mcpServers` shape, and point full surface token guidance at the manifest-owned
`requiredScopes` list rather than hand-maintained partial scope examples.
The public `/api/agent/capabilities` manifest must also expose
`requiredScopes`, the canonical deduplicated scope list derived from its
declared capabilities through `internal/agentcapabilities.RequiredCapabilityScopes`.
Browser settings surfaces, `cmd/pulse-mcp` startup errors, MCP documentation,
probes, and future clients must consume that manifest-owned list for
full-surface token guidance instead of rebuilding a scope list from local
hardcoded assumptions.

`/api/agent/resource-context/{id}` is the agent-consumable bundled
context endpoint. One read returns the full situated picture of a
resource — identity, operator-set state (with server-computed
`maintenanceWindowActive` flag), active findings as the
`AgentResourceFindingSnapshot` projection (lightweight subset of
the seven-question schema fields agents need), pending approvals
scoped to this resource as the `AgentResourceApprovalSummary`
projection (id, command, riskLevel, requestedBy, requestedAt,
expiresAt — same vocabulary as `approval.pending` SSE events so
"what's pending right now" and "what just became pending" agree
on shape, with `commandRedacted:true` replacing raw command text
for plain `monitoring:read` API tokens), and recent action audits
including refused dispatches
with their stable token prefixes (`resource_remediation_locked:`,
`plan_drift:`) preserved verbatim and the same agent-stable
`verification` block the SSE `action.completed` payload carries
(shared `AgentResourceActionVerification` projection, shared
`projectAgentResourceVerification` helper) so the bundle's depth
view and the doorbell speak the same vocabulary on probe outcomes;
action and verification command/note text follows the same stable-marker
redaction rule as action-audit readback, while stable status/error
prefixes remain verbatim.
The same response also carries additive `contextSections`, built by the
shared `internal/agentcontext` package, with stable fact/redaction objects,
source, trust-tier, observed-at, and generated-at metadata. Those sections are
the canonical rich resource context pack for Assistant, MCP/tool clients, and
copy/export consumers: they may expose bounded Pulse-authored or
Pulse-observed runtime/discovery, topology, safety, policy, and recent-change
facts, but must not expose raw command output, provider config, environment
values, unbounded metadata maps, bind-mount paths, label values, or secret-like
capability parameters.
The shape is intentionally narrower than the full internal types
so agents see a stable agent-paradigm contract, decoupled from
internal type evolution.
Active findings come through the `AgentFindingsProvider` adapter
wired at startup so the api package stays free of an
`internal/ai` import; the patrol service holds the canonical
findings store and projects each `Finding` into the snapshot
shape. Pending approvals come through the parallel
`AgentApprovalsProvider` adapter — the router wires a request-time
approval-store provider that resolves `approval.GetStore()` on
each read (so multi-tenant store rebuilds stay honored), filters
via `approval.BelongsToOrg` for tenant scope, and matches
`req.CanonicalResourceID()` against the requested resource so
cross-resource approvals don't leak into the bundle. Active
findings, pending approvals, and recent actions are always arrays
(never null) so agents can iterate without nil-checking; absent
operator state surfaces as a missing `operatorState` field
(omitempty), distinguishing "no operator overrides" from
"all-zero overrides recorded."

`/api/agent/fleet-context` is the agent-consumable triage view
across every resource visible to the org. One read returns a thin
per-resource rollup: identity (canonical id, type, name,
technology), operator-intent flags (`intentionallyOffline`,
`neverAutoRemediate`, `maintenanceWindowActive` — server-computed
once at request time so agents don't re-evaluate timestamps
client-side), per-severity finding counts as the
`AgentFleetFindingCounts` struct (`total`, `critical`, `warning`,
`info`), and `pendingApprovalCount`. The `resources` slice is
always an array (never null) so agents iterate without
nil-checking; an empty registry surfaces as `resources: []`. The
endpoint walks the registry once and reuses the same per-resource
adapters as the per-resource bundle (operator-state via
`unified.ResourceStore.GetResourceOperatorState`, findings via
`AgentFindingsProvider`, pending approvals via the
`AgentApprovalsProvider` bulk count projection). Fleet-context
approval counts must come from one org-scoped, resource-keyed scan
of the bounded pending-approval list, not N calls to the
per-resource approval summary helper. Agents pick a focus from the
fleet view, then drill into the per-resource bundle for depth.

The TS client `frontend-modern/src/api/resourceOperatorState.ts`
mirrors the canonical Go shape from
`internal/unifiedresources/resource_operator_state.go` and exposes
`getResourceOperatorState`, `setResourceOperatorState`, and
`clearResourceOperatorState` against the
`/api/resources/{id}/operator-state` endpoint. The GET path
normalizes the server's `404 operator_state_not_set` response into
`null` so callers see "no state recorded" as a clean default rather
than a thrown error; non-404 errors propagate. The PUT path
percent-encodes the canonical resource id segment so colon-bearing
ids round-trip safely through URL routing.

The router wires the operator-state adapter into the findings runtime
at startup: `internal/api/router.go` calls
`patrol.GetFindings().SetResourceOperatorStateProvider(...)` with a
`ResourceOperatorStateProviderFunc` closure that reads the unified
store and returns a `ResourceOperatorStateProjection` carrying every
operator-set signal in one call (maintenance window via
`state.IsInMaintenanceAt` plus the `IntentionallyOffline` flag).
This keeps `internal/ai` free of an `internal/unifiedresources`
import while giving the findings runtime full per-resource
suppression without each org needing a separate adapter declaration
and without growing per-finding lookups as new signals land.

`/api/resources/{id}/operator-state` is the canonical surface for
operator-set per-resource intent (intentionally offline, never
auto-remediate, maintenance window, criticality hint). GET requires
`monitoring:read` and returns `404` with `{ "error":
"operator_state_not_set" }` when no entry exists; PUT and DELETE
require `monitoring:write` because they modulate Patrol's behavior on
findings against the resource. PUT replaces the entire record (no
per-field merge); the `canonicalId` taken from the URL path
authoritatively wins over any value in the request body so a request
addressed at `/vm:101` cannot retarget the write at a different
resource through body manipulation. Server populates `setAt` and
`setBy` from the request time and authenticated identity, ignoring any
client values to keep the audit attribution honest. Validation
violations surface as `400` with `{ "error":
"operator_state_invalid", "message": "..." }` so the frontend can
branch on the stable error code rather than string-matching the
message. DELETE is idempotent (`204` whether or not an entry was
present). The handler dispatches off `r.Method` rather than mounting
three sibling routes so the URL surface stays a single resource path
matching the rest of `/api/resources/{id}/...`.

The action governance loop at `/api/actions/plan`,
`/api/actions/{id}/decision`, and `/api/actions/{id}/execute` is
the canonical write surface for capability invocations against a
resource. All three require the `ai:execute` scope, which is
distinct from `monitoring:write` because action governance is the
governed-execution dimension of the substrate. The handlers emit
the agent-stable error envelope (`{"error": "<code>", "message":
"...", "details": {...}}`), matching the rest of the agent
surface, so an agent that branches on the `error` field across
read, write, and action capabilities sees the same envelope
shape everywhere. The `details` map is optional and carries
field-level reasons on validation failures (e.g.
`{"resourceId": "resource id is required"}`) so agents do not
need to parse human text to know which field went wrong. The
manifest's per-capability `errorCodes` lists declare the closed
set: `plan_action` carries `invalid_action_request`,
`resource_not_found`, `capability_not_found`,
`action_execution_unavailable`; `decide_action`
carries `missing_id`, `invalid_id`, `invalid_action_decision`,
`action_not_found`, `action_not_pending`, `action_plan_expired`;
`execute_action` carries `missing_id`, `invalid_id`,
`invalid_action_execution`, `action_not_found`,
`action_not_approved`, `action_already_executing`,
`action_execution_final`, `action_dry_run_only`,
`action_plan_expired`, `action_plan_drift`,
`resource_remediation_locked`, `action_executor_unavailable`. 5xx internal-failure codes
(audit-store outages, encode failures) are not declared per
capability; agents branch on 5xx generically.

The action endpoints do not introduce new routes. The same paths
that the frontend has historically called (zero current frontend
consumers, verified) now back the manifest entries; the
substrate's "manifest projection is cheap" promise has a
documented footnote that bringing an existing endpoint into the
external-agent surface may require migrating its error envelope from the
platform-wide `APIError` shape to the agent-stable shape. That
migration now covers the action governance handlers and the Patrol
finding lifecycle handlers (`acknowledge_finding`, `snooze_finding`,
`dismiss_finding`, `resolve_finding`). Future external-agent additions
on existing endpoints will pay the same cost. The trade-off is
intentional: the external-agent surface keeps a single envelope
contract rather than carrying a wrapper layer to translate per
capability.

Deploy-job monitored-system volume denials are retired. API routes may still
accept historical payloads that mention old license-slot terminology for
migration or diagnostics, but runtime deploy responses must not emit
`skipped_license`, `license_limit`, workspace-slot reservations, or
plan-upgrade retry labels for monitored infrastructure volume.

`useInfrastructureDiscoveryRuntimeState.ts` no longer gates `/api/discover`
polling on a settings tab name; polling is mount-scoped. The tab guard was
removed when the infrastructure nav collapsed to one `infrastructure-systems`
entry.
That same settings boundary now also keeps the customer-facing Settings
Infrastructure target label in
`frontend-modern/src/utils/infrastructureSettingsPresentation.ts`. API-backed
setup, discovery, and guidance surfaces must point operators to
`Settings → Infrastructure` rather than hard-coding retired nested settings
paths such as `Settings → Infrastructure → Proxmox`.

The API layer already uses contract tests in many places, but every major live
contract should continue moving toward canonical-only runtime shapes.
The shared API-token presentation helper also owns API token management-location
copy for Settings surfaces. Token reveal and rotation guidance must point
operators to `Settings → API Access` and must not revive legacy
`Security → API tokens` wording.
That same shared `internal/api/` boundary now also keeps ephemeral auth flow
state and request correlation fail-closed. OIDC authorization state storage
must cap abandoned entries and evict the earliest-expiring state before
unbounded growth, bootstrap token validation must enforce a per-client retry
limit with an explicit `Retry-After` contract, and incoming `X-Request-ID`
headers may only round-trip when they fit the bounded safe character set used
for logs and response headers.
That same shared settings/licensing contract now also owns the split usage-data
payload model. `frontend-modern/src/api/settings.ts`,
`internal/api/router_routes_licensing.go`, and adjacent settings callers must
keep anonymous outbound telemetry as the only browser-visible usage-data scope;
local commercial reporting controls stay internal/admin-owned and must not
surface as ordinary Settings controls. The telemetry preview payload must ship
normalized version identity fields (`version`, `version_raw`, `version_channel`,
`version_build`, `version_is_development`, and
`version_is_published_release`) instead of leaving browser callers to infer
published-release truth from raw build strings.
That same preview contract now includes the complete anonymous telemetry
payload shape, including aggregate self-hosted adoption counters for monitored
platforms, workloads, storage, and availability targets plus coarse feature
booleans and content-free Patrol, Assistant, and external-agent usage counters.
The frontend `TelemetryPingPreview` type in
`frontend-modern/src/api/settings.ts` must mirror every browser-visible JSON
field from `internal/telemetry.Ping`, including the resolved operations-loop,
approved-execution, first-party Assistant, external-agent, and Pulse MCP adapter
variants used by Pro value reporting, plus source-specific operations-loop
starter request counts including the Pro activation entry point.
Those Pulse Intelligence fields may describe only configured/active/governed-action
adoption state plus Assistant, Patrol detection, Patrol investigation, Patrol
resolution, aggregate external-agent use, adapter-origin MCP use, approval,
and governed-action counts inside the
rotating telemetry window. Browser callers
may display or copy the exact payload, but they must not derive hostnames,
infrastructure identifiers, prompt/chat content, command text, action output,
license tiers, token values, token counts, resource IDs, finding IDs, or
approval identities from it.
Approved action decision telemetry is part of that same anonymous API
contract: `pulse_intelligence_approved_action_decisions_30d` must count
distinct action IDs with approved-decision lifecycle evidence in the telemetry
window, or approval records whose approved decision timestamp is inside the
window for older rows that lack lifecycle events. It may count toward completed
operations-loop proof, but it must not be conflated with approved execution
attempts or approved action successes, and it must never expose action IDs,
actors, reasons, resource IDs, command text, action output, or verification
detail.
Approved execution attempt telemetry is part of that same anonymous API
contract: `pulse_intelligence_approved_action_attempts_30d` must be counted as
distinct action IDs with execution-attempt lifecycle evidence (`executing`,
`completed`, or `failed`) resolved back to an approved action audit, with final
audit-state fallback only for older rows that lack lifecycle events. It must
not be derived from generic action-plan, approval-request, token-use, Assistant
chat, or MCP call counts, and it must never expose action IDs, actors, reasons,
resource IDs, command text, or action output.
Approved action success telemetry is the stricter completion proof in that same
contract: `pulse_intelligence_approved_action_successes_30d` must count only
distinct approved action audits that reached completed state successfully, with
lifecycle-backed attribution where lifecycle evidence exists and final
audit-state fallback only for older rows. Failed or refused approved actions may
count as attempts but must not count as successes, and the success counter must
not expose action IDs, actors, reasons, resource IDs, command text, verification
detail, or action output.
Rejected action decision telemetry is the declined-governance proof in that same
contract: `pulse_intelligence_rejected_action_decisions_30d` must count distinct
action IDs with rejection-decision lifecycle evidence in the telemetry window,
with final rejected audit-state fallback only for older rows that lack lifecycle
events. It may count toward governed-action activity, but it must not count as
an approved execution attempt, approved success, or resolved operations loop
proof, and it must never expose action IDs, actors, reasons, resource IDs,
command text, or action output.
Completed operations-loop telemetry is the coarse approve/reject journey proof
in that same contract. `pulse_intelligence_complete_operations_loop_30d` and
the source-specific operations-loop booleans may be true only when the same
rotating telemetry window contains Patrol issue evidence, contextual
Assistant/MCP/external-agent collaboration, and either a rejected action
decision, an approved action decision, or approved execution-attempt evidence.
Generic Patrol runs, Patrol AI calls, action plans, and approval requests may
activate or reach the loop, but they must not complete it without issue-backed
Patrol evidence and a real decision/outcome signal.
Patrol resolution telemetry is the content-free outcome proof in that same
contract. `pulse_intelligence_patrol_resolved_findings_30d` may count only
findings whose own lifecycle reached resolved state in the telemetry window, or
findings whose investigation outcome reached `resolved` or `fix_verified` with
investigation completion evidence in that window. It must not count generic
Patrol runs, new findings, model calls, action approvals, or successful actions
without a resolved/fix-verified Patrol finding outcome, and it must never expose
finding IDs, resource IDs, finding text, remediation details, verification
detail, actors, command text, or action output.
Resolved operations loop telemetry is a derived content-free boolean:
`pulse_intelligence_resolved_operations_loop_30d` may be true only when the
same telemetry window contains Patrol resolution plus Assistant governed-context
or governed-tool collaboration or MCP/external-agent collaboration, and at
least one approved action success. In this contract, approved action success is
the shared verified-outcome predicate used by the operations-loop status route:
the approved action must have `VerificationOutcome.Status=verified` or a
canonical verification result that ran and succeeded. A bare successful
execution result is not sufficient. It is co-occurrence proof for value
reporting, not causality proof, and it must not carry any identifiers, prompts,
tool names, command payloads, action outputs, or finding text.
Patrol control completed-loop telemetry is the primary derived content-free
paid-value boolean:
`pulse_intelligence_patrol_control_completed_operations_loop_30d` may be true
only when the same telemetry window contains Patrol control starter evidence
from `pulse_patrol`, `patrol_control`, legacy `patrol_autonomy`, or legacy
`pulse_pro_activation`, Patrol issue evidence, contextual Assistant or
MCP/external-agent collaboration, and either a rejected governed decision or an
approved governed decision with verified outcome proof. It is terminal-loop
Patrol control proof, not causality proof, and it must not carry
checkout/account identity, token identity, prompt content, resource IDs,
finding IDs, action IDs, command payloads, action outputs, or verification
detail.
Patrol control resolved-loop telemetry is stricter:
`pulse_intelligence_patrol_control_resolved_operations_loop_30d` may be true
only when the same content-free window also contains an approved governed
decision and verified outcome proof. It must not treat rejected decisions as
resolved-loop proof. These telemetry booleans must use the same
`internal/telemetry.ClassifyPulseIntelligencePatrolControlProof` count-only
classifier as `GET /api/agent/patrol-control/status`, so native status, MCP
status consumers, telemetry preview, and outbound telemetry cannot diverge on
what completed, resolved, or governed-decision proof means. Legacy
`pulse_intelligence_pro_activation_completed_operations_loop_30d` and
`pulse_intelligence_pro_activation_resolved_operations_loop_30d` are
compatibility mirrors of those primary Patrol control booleans.
Paid Patrol control telemetry is a derived content-free cohort:
`pulse_intelligence_patrol_control_paid_completed_operations_loop_30d` may be
true only when the same payload also reports `paid_license=true` and
`pulse_intelligence_patrol_control_completed_operations_loop_30d=true`.
`pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d` may be
true only when the same payload also reports `paid_license=true` and
`pulse_intelligence_patrol_control_resolved_operations_loop_30d=true`. Legacy
`pulse_intelligence_pro_activation_paid_completed_operations_loop_30d` and
`pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d` mirror
those primary paid Patrol-control fields for cohort continuity. These fields
are paid-cohort proof for aggregate activation, retention, and free-to-paid
comparison, not causality proof; they must not carry exact plan tier,
checkout/account identity, license IDs, token identity, prompts, resources,
findings, action IDs, command payloads, action outputs, or verification detail.
External-agent recent-use telemetry is a content-free capability-activity marker
for authenticated calls into the agent/MCP contract. Every request/response
capability published through the Pulse MCP surface must map to one coarse
activity class before the route handler runs, and the marker must require the
called capability's manifest-declared scope rather than the entire manifest
scope set. Read-only external agents using `monitoring:read` must therefore
count for context/event-stream activity without being treated as action,
settings, or approval-capable callers. Generic API token last-use timestamps
are not canonical proof that the external-agent surface was used.
The `pulse-mcp` adapter may stamp the content-free
`X-Pulse-Agent-Surface: pulse_mcp` header on those same capability requests so
the telemetry snapshot can expose adapter-specific recent use while keeping the
aggregate external-agent recent-use bit valid for both direct HTTP agents and
the adapter. Route handlers must normalize unknown or absent surface markers
back to direct `agent_api` use and must never persist client identity, prompts,
request bodies, route parameters, or local resource identifiers as part of the
surface marker.
Pulse MCP `prompts/get` starter activity uses the same surface marker but a
separate route-specific payload: `POST /api/agent/workflow-prompt-activity`
records only the manifest prompt name after successful local prompt rendering.
It does not count as a capability-class request and must not infer
external-agent contextual collaboration or action execution from prompt access
alone.
That same browser-transport contract now tolerates sparse preview
payloads without changing the runtime truth. Patrol transport may omit
`finding_ids`, and infrastructure removal previews may stage optimistic rows
only after canonical IDs have been resolved or a safe row-name fallback has
been chosen. API-adjacent browser callers must not reinterpret missing IDs or
preview arrays as authoritative empty success.
Monitored-system commercial admission is retired from that owned live contract.
Community, Relay, Pro, hosted, MSP, and private-policy monitoring routes must
not gate persistence on monitored-system volume. Preview routes may still
project prospective candidates or previewed source records through the
canonical monitored-system resolver for inventory explanation, but they must
not expose cap verdicts, `current_available`, or save-blocking entitlement
state.
Supplemental-provider startup readiness now belongs to the monitored-system
ledger and impact-preview availability states, not to active entitlement limit
fields. API contracts must not serialize a live monitored-system count from
the first store-backed read-state when provider-owned inventories such as
TrueNAS or VMware have not yet completed an initial baseline and been rebuilt
into the canonical monitor store.
That same workload-chart boundary now also owns the rendered-metric budget on
the shared monitoring routes. `/api/charts/workloads` and
`/api/charts/workloads-summary` may batch provider-backed reads in parallel,
but they must request only the canonical workload metrics they actually
serialize (`cpu`, `memory`, `disk`, `netin`, `netout`), with Kubernetes pods
staying on that same five-metric set, instead of widening back to disk
read/write or fetch-all backend batches that the browser never renders.
The shared metrics-history contract now also owns physical-disk live I/O
windows. `/api/metrics-store/history` must accept `resourceType=disk`, keep
`30m` as a valid compact live range, and resolve `disk`, `diskread`,
`diskwrite`, and `smart_temp` against the canonical disk
`MetricsTarget.ResourceID` that unified resources already expose, instead of
leaving storage drawers or other callers to fork a disk-local history route or
invent an alternate disk identity.
That same metrics-history contract also owns Kubernetes pod identity
normalization. `/api/metrics-store/history` must accept legacy bare pod IDs
such as `cluster-1:pod:pod-1`, canonicalize them onto the unified pod metrics
target `k8s:cluster-1:pod:pod-1`, and keep the response `resourceId` on that
canonical key. When store-backed history is absent, the handler must fall back
to the same in-memory guest metrics cache that workload charts use for pods,
so demo and mock Kubernetes charts do not go blank while aggregate workload
charts still render.
That same metrics-history contract also owns canonical Kubernetes type
coverage across the shared chart clients. `/api/resources`,
`frontend-modern/src/api/charts.ts`, and `/api/metrics-store/history` must
preserve the shared metrics-target family for clusters, nodes, pods, and
deployments rather than treating prefixed pod IDs as a special case and
dropping `k8s-deployment` onto an untyped fallback. Cluster history stays on
the canonical cluster source key, node history on
`<cluster>:node:<uid-or-name>`, pod history on
`k8s:<cluster>:pod:<uid-or-namespace/name>`, and deployment history on
`<cluster>:deployment:<uid-or-namespace/name>`, so demo and live workload
detail charts all resolve through one governed identity contract.
That same metrics-history contract also owns commercial history-range
enforcement. `frontend-modern/src/api/charts.ts` may expose `14d` as a
first-class `HistoryTimeRange`, and `/api/metrics-store/history` must parse
positive day ranges before querying the store so entitlement checks cannot be
bypassed with duration syntax. Community instances must remain capped at seven
days, Relay must allow 14 days and reject longer history, and Pro-tier
entitlements must continue to allow 90-day history.
The Pulse Account commercial shell now also owns a dedicated bootstrap
contract in `internal/cloudcp/portal/page.go`, `internal/cloudcp/portal/handlers.go`,
and `internal/cloudcp/portal/handlers_test.go`. `/api/portal/bootstrap` and
the in-page `pulse-account-bootstrap` payload must stay shape-identical for
account identity context, signed-out versus signed-in shell state, workspace
summaries, and renderer-owned public, commercial, and control-plane route
configuration, including the canonical bootstrap route path, magic-link request
path, signup path, and stable workspace summary fields such as `created_at`.
That workspace summary contract must expose explicit health semantics: `healthy`
for passing health checks, `checking` only when no completed health check
exists yet, and `unhealthy` for a failed latest health check.
That same portal API contract treats `deleting` and `deleted` tenants as
control-plane history, not visible workspace payloads: the browser bootstrap,
`/api/portal/dashboard`, and `/api/portal/workspaces/{tenant_id}` must filter
soft-deleted rows consistently, with detail reads returning `404` for hidden
workspaces rather than leaking retired runtime records into the account shell.
That same shared `internal/api/` plus `internal/websocket/hub.go` boundary also
owns browser websocket origin continuity for reverse-proxied runtimes. Same-host
browser origins must continue to connect when a reverse proxy preserves the
external host but terminates TLS upstream, so live updates do not fail merely
because the backend hop is plain HTTP. Forwarded host/proto headers may extend
that same-origin boundary only after explicit trusted proxy CIDRs are injected,
so hosted tenants and proxies that rewrite hostnames still fail closed onto the
trusted forwarded-origin contract instead of weakening cross-site websocket
checks. Browser-facing websocket upgrades must also require an explicit
`Origin` header even when `allowedOrigins` is wildcarded, so missing-origin
requests cannot silently bypass the cross-site websocket boundary.
`PULSE_TRUSTED_PROXY_CIDRS` must also reject wildcard trust ranges such as
`0.0.0.0/0` or `::/0` at startup, while runtime forwarded-header parsing
fails closed if an invalid wildcard proxy trust range somehow reaches the
process.
That same shared boundary now also owns outbound SSO metadata and discovery
URL handling. SAML test/preview metadata fetches and OIDC issuer discovery
must normalize absolute HTTP(S) inputs through shared helpers, reject
userinfo-bearing URLs before any outbound request, and append the OIDC
well-known path relative to the issuer base instead of resetting discovery to
the origin root. Runtime SAML metadata refresh, runtime OIDC discovery, and
admin-side SSO test/preview fetches must all use that same restricted outbound
transport policy, including same-origin redirect validation and checked
regular-file loads for any configured SSO credential or CA-bundle path.
That same restricted outbound transport is also the canonical cross-product
egress boundary. It is exported via `pkg/securityutil`, with
`internal/securityutil` retaining Pulse-local wrappers, so adjacent products
such as `pulse-enterprise` can reuse the same DNS-rebinding-safe dial and
redirect policy for operator-configured audit webhooks instead of reintroducing
raw `http.Client` egress paths.
That same SSO boundary also owns manual SAML endpoint validation payloads.
`internal/api/identity_sso_handlers.go`, `internal/api/saml_service.go`, and
`internal/api/contract_test.go` must preserve both `idpSsoUrl` and optional
`idpSloUrl` on the shared SAML test request, and both fields must fail closed
through the same validated absolute HTTP(S) helpers instead of letting the
manual logout URL drift out of the request model or bypass the governed URL
normalization path.
That same runtime SSO contract also owns the Pulse-side public URL that feeds
SAML service-provider metadata and auth requests. `internal/api/saml_handlers.go`,
`internal/api/saml_service.go`, and the SAML regression tests must rebind
previously initialized SAML providers to the current configured `PublicURL`
before metadata or browser login flows emit SP entity, ACS, or metadata URLs,
so a stale startup-time blank/relative base URL cannot leak back into runtime
metadata or auth request generation once the canonical external URL is known.
That same SSO API boundary also owns final browser redirect construction after
local auth handoff. OIDC and SAML success/error handlers must build their
local `returnTo` targets through one canonical local-path helper that rejects
absolute or host-bearing targets before query params are appended, so shared
identity flows cannot drift back to per-handler open-redirect shaping.
That same runtime SSO contract also owns browser-session principal identity:
`internal/api/oidc_handlers.go`, `internal/api/saml_handlers.go`,
`internal/api/auth_principal_identity.go`, and `internal/api/contract_test.go` must derive
OIDC/SAML session users from provider-scoped stable subjects (`sub`/`NameID`
plus provider ID), not mutable username, email, or display-name claims. Legacy
username/email RBAC assignments may be copied only as compatibility migration
inputs when no authoritative group mapping is present.
The SSO entitlement contract is part of that same API boundary. Provider
administration, SAML metadata preview/test, and runtime SAML routes may require
the core `sso` capability, but they must not gate SAML or multi-provider
provider management on the compatibility `advanced_sso` key. `advanced_sso`
remains only an included entitlement payload key for older clients, while OIDC,
SAML, and multi-provider SSO are Community-tier runtime capabilities.
Commercial self-service actions in that shell must stay same-origin as well:
the frontend may only call the portal-owned `/api/portal/commercial/*` routes,
and `internal/cloudcp/portal/commercial_proxy.go` plus `internal/cloudcp/routes.go`
own the server-side proxy boundary to the shared license/commercial APIs so
the browser runtime does not widen control-plane CSP with direct cross-origin
commercial fetches.
That same shared commercial boundary now also applies to Patrol feature
handoffs. API-backed Patrol surfaces may consume canonical commercial hrefs
from the shared license/commercial contract, but they must not re-decide
internal-versus-external navigation behavior inside API-adjacent page or hook
owners once the contract can resolve to both in-app and public destinations.
That same shared `internal/api/` boundary now also owns browser presentation
policy for public-demo and commercial suppression. `/api/security/status` must
continue to expose the raw session capability fact
`sessionCapabilities.demoMode`, but browser shells and shared frontend stores
now consume the explicit `presentationPolicy` payload from that same response
as the canonical runtime contract for `demoMode`, `readOnly`,
`hideCommercial`, and `hideUpgrade`. Commercial posture and billing stores
must therefore defer their first read until that policy has resolved, so
public demos fail closed without probing hidden commercial routes during
bootstrap.
For ordinary self-hosted v6 installs, that same security-status contract owns
the free-first commercial posture: `hideUpgrade` defaults to true outside
hosted mode, and API consumers must treat it as a prompt-suppression contract
for upgrade links, trial CTAs, plan upsells, and paid-only navigation rather
than as a billing entitlement change.
That same contract split also makes the licensing boundary explicit:
`/api/license/runtime-capabilities` is the public runtime feature contract,
`/api/license/commercial-posture` is the non-billing upgrade posture
contract for real customer workspaces, and `/api/license/entitlements`
remains billing-only. New callers must extend one of those owned shapes
instead of reviving a combined entitlement payload for mixed runtime,
commercial, and billing concerns.
Paid self-hosted runtime identity is part of that owned shape: entitlement
and runtime-capability payloads must carry a normalized `runtime` identity,
and `runtime-capabilities` must include `blocked_capabilities` entries when a
valid license grants a feature that the current community runtime cannot
execute. Billing entitlements may still report the licensed feature set, but
browser feature gates must use the runtime-capabilities contract for executable
truth and show the paid-runtime-required action instead of treating the license
as inactive.
The same runtime identity must also flow to the license server during
activation, legacy exchange, and grant refresh so support/audit tooling can
differentiate a paid customer running the public community binary from one
running the private Pulse Pro runtime even when both artifacts share a version
tag.
That same shared licensing contract also owns internal runtime-only
capabilities. Release demo runtimes may use the internal `demo_fixtures`
entitlement to authorize mock fixture data and `/api/system/mock-mode`
transitions, but browser-facing entitlement and runtime payloads must filter
that capability back out so public callers never learn or depend on internal
demo-fixture grants.
That same shared licensing boundary now also owns release-build enforcement of
that internal demo-fixture capability. Dev and test builds may keep local
fixture proof tolerant so mock-backed demos can be exercised without a paid
grant, but release builds must gate runtime mock rewiring through the
build-tagged `shouldEnforceReleaseDemoFixtureRuntime()` contract before
`syncReleaseDemoFixtureRuntime()` can enable fixtures on a live server.
Browser payloads and public-demo callers must still never see or depend on
that internal grant.
That same shared API contract now also owns browser-proofed read separation.
Non-billing browser journeys such as
`tests/integration/tests/11-first-session.spec.ts`,
`tests/integration/tests/journeys/01-smoke-bootstrap-login-infrastructure.spec.ts`,
and `tests/integration/tests/journeys/03-relay-pairing.spec.ts` may call
`/api/license/runtime-capabilities` for feature truth, but they must assert
zero browser requests to `/api/license/entitlements`. Billing activation,
upgrade, and owned billing panels remain the only browser surfaces allowed to
read the billing-only entitlements contract.
`/portal` is now one bootstrap-driven shell for both anonymous and
authenticated users, so new account frontend work must extend that shared
contract rather than inventing a second local payload shape, reviving separate
login/portal templates, or hardcoding production URLs, route prefixes, or
DOM-scraped account facts in static assets. That canonical renderer now lives
under `internal/cloudcp/portal/frontend/`, is embedded from
`internal/cloudcp/portal/dist/`, and is guarded by
`internal/cloudcp/portal/frontend_sync_test.go`, so the maintained frontend
sources and the committed embedded bundle cannot drift silently. The maintained
portal source tree now also owns explicit runtime/bootstrap type definitions
and one task-first shell model across desktop and phone widths: narrow-screen
navigation must collapse the same bootstrap-driven task shell into a compact
task strip, not a second mobile-only route or DOM contract, and the runtime
must keep the active task visibly in-frame when that strip scrolls. That same
shared bootstrap shell must also compress account identity into a compact
mobile summary strip rather than introducing a second narrow-screen
account-context payload or task-specific DOM contract. When that shared shell
opens a lower workspace job surface such as lifecycle review or the
create-workspace form, the runtime must reveal the opened surface instead of
leaving the user at the top of the list. The same shared runtime contract must
also keep the workspace detail rail absent until a lifecycle or
create-workspace job is active, rather than rendering a default idle
lifecycle explainer before the user has picked a task. The same task-first
runtime rule now also applies to `Access`: the hosted roster is the default
surface, and invite, role-change, or remove controls only appear when the
matching access job is active. When `can_manage` is false, that same roster
must stay a review surface rather than rendering a third action column full of
fake disabled row state. That same typed bootstrap/runtime contract must also
ship the current hosted roster snapshot in the portal bootstrap payload so the
first `Access` render is a real review surface rather than a fetch-first or
error-first placeholder; later member API reads remain refresh and mutation
follow-through. That same shared access contract must keep stable access
subject identity across bootstrap and mutation responses: hosted roster rows
carry `subject_id`, `state`, and optional `user_id`, unknown-email invites
return `202 Accepted` with `state=pending` instead of auto-binding a guessed
future user record, and portal magic-link verification may materialize that
pending subject into a membership only after the invited email authenticates
through the portal-owned session path. `Billing` follows the same shared runtime
contract: hosted billing remains the default primary path, self-hosted billing
jobs open one panel at a time, and the runtime must reveal the active billing
panel on phone-width layouts instead of leaving it offscreen. The same
bootstrap/runtime contract must also carry explicit truth for whether
self-hosted commercial history is relevant to the signed-in account, so
hosted-only accounts do not render self-hosted license, refund, privacy, or
self-hosted escalation paths by default, and self-hosted-only accounts do not
front-load an empty hosted-billing block before the real self-hosted jobs.
That same runtime handoff contract now also covers product-originated
self-hosted upgrade arrivals: `/portal?portal_handoff_id=...`
may open a portal-owned upgrade job inside `Billing`, but it must not
fabricate broader self-hosted commercial history or reveal
retrieve/refund/privacy panels for a hosted-only account that only arrived
through an upgrade CTA.
That same commercial contract now also includes the self-hosted purchase
return path. Product-originated upgrade handoffs must include a canonical
commercial-owned `portal_handoff_id` that resolves server-side to the bound
checkout intent. Pulse still binds checkout completion to a signed
`purchase_return_token`, but that token must stay inside the Pulse-owned
activation callback path rather than leaking into the portal arrival URL. The
portal runtime must resolve the verified portal handoff through the shared
commercial API and use only that owned handoff-derived checkout state when it
starts checkout instead of trusting browser referrer state, raw
`checkout_intent_id`, or loose `feature` / `return_url` parameters. The
browser-facing `GET /v1/checkout/portal-handoff` response must not expose the
bound `checkout_intent_id`, and `POST /v1/checkout/session` must accept only
`portal_handoff_id` for product-originated upgrade arrivals so the license
server resolves the private checkout intent internally before Stripe session
creation. That handoff response is now intentionally narrowly stateful: first
resolution must stamp `resolved_at`, the portal-facing lifecycle must stay
derived from the owned handoff plus the private checkout intent
(`created`, `resolved`, `checkout_started`, `completed`), and completed
handoffs must refuse browser checkout replay instead of silently reopening
commercial state. The owned handoff row is also the canonical binding record
for product-originated self-hosted checkout: it must persist the signed
`purchase_return_jti`, the bound Stripe `session_id`, and the timestamps that
prove resolve, checkout-start, and completion. Stripe success must return
that same `portal_handoff_id` into Pulse's activation callback, and Pulse
must compare both `portal_handoff_id` and `purchase_return_jti` against the
commercial checkout-session result before redeeming the activation key, so
browser form/query state and Stripe metadata alone never become the source of
truth for a completed self-hosted upgrade. That same owned callback path must
resolve only to HTTPS instance origins or a direct loopback HTTP origin, and
the hosted entitlement follow-up fetches behind that path must stay on the
restricted outbound client instead of raw commercial HTTP calls. Once that
commercial binding
verifies, Pulse's owned callback must persist a dedicated local
purchase-return redemption record keyed by `portal_handoff_id` plus
`purchase_return_jti`, use explicit local redemption state
(`started`, `activated`, `failed`) instead of a generic replay tombstone, and
allow retry only from owned failed state rather than by deleting the local
binding outright. That same owned contract also
retires the old compatibility
bootstrap surfaces: Pulse must not expose a separate public
`GET /auth/license-purchase-handoff` resolver, and the commercial server must
not expose a direct browser bootstrap through `GET /v1/checkout/intent` once
`portal_handoff_id` is canonical. Pulse's public
`GET /auth/license-purchase-activate`
callback then serves an auto-submitting bridge into the owned POST activation
path, which redeems the completed checkout through the shared
license/commercial API before returning the browser to the owned billing plan
route. Stripe cancel must return directly to owned billing with
`purchase=cancelled`; activation success, expiry, and failure must return to
owned billing with explicit arrival states so the billing runtime can surface
those results in-product. If Pulse cannot create the initial Pulse Account
portal handoff, `GET /auth/license-purchase-start` must still return the
browser to owned billing with `purchase=unavailable` so the runtime can
surface the failure in-product instead of leaving the operator on a raw
service error. When the upgrade flow was opened in a secondary tab,
the callback may refresh the originating billing tab and close itself; when no
owned billing tab is present, the same contract still owns intent
normalization. Product-originated self-hosted purchase handoff must emit
`feature=self_hosted_plan` and `intent=self_hosted_plan` as the canonical
browser/runtime value. The older `max_monitored_systems` label may be accepted
only as a backward-compatible alias during request or callback normalization,
but Pulse and the license server must not emit it as the primary self-hosted
purchase intent once the uncapped self-hosted model is canonical.
Compatibility-only Pro capabilities and legacy monitored-system aliases must
not become marketed upgrade reasons in the free-tier entitlement payload; the
payload should describe paid surfaces that are actually being sold rather than
recreating the retired monitored-infrastructure paywall.
opener is available, the callback must still return the current tab to the
owned billing route automatically instead of leaving the operator on a dead
success page.
That same typed bootstrap/runtime contract must also derive the default signed-
in shell section from account shape: hosted accounts open on `Workspaces`,
self-hosted-only accounts open on `Billing`, and the signed-in shell keeps
precise workspace counts inline on `Workspaces` instead of exposing a separate
`Summary` tab as a primary or default destination.
That same typed shell-section contract now excludes `overview` entirely:
`internal/cloudcp/portal/frontend/src/types.ts`,
`internal/cloudcp/portal/frontend/src/shell_section.ts`, and
`internal/cloudcp/portal/frontend/src/shell.ts` may route only the governed
`workspaces`, `access`, `billing`, and `support` destinations, with hosted
account arrivals defaulting to `workspaces` and self-hosted-only arrivals
defaulting to `billing`.
The same account-shape runtime contract must also keep the shell navigation
honest: the task row is `Workspaces`, `Access`, `Billing`, and `Support`.
Self-hosted-only accounts must drop hosted-only `Workspaces` and `Access`
surfaces rather than implying live hosted work, and any shared fallback
surface that still resolves there must render an explicit unavailable state. `Support`
follows the same
account-shape runtime contract: self-hosted-only accounts expose only the
billing escalation path and billing-specific handoff packet, and hosted
workspace/access escalation controls must not render when no hosted account
exists.
The same typed bootstrap/runtime contract must also keep permission copy
honest for hosted view-only roles: when `can_manage` is false, `Workspaces`,
`Access`, and hosted `Billing` must stop advertising create, roster-mutation,
or hosted-billing actions and must instead state that an owner or admin is
required.
That same typed shell contract must also keep account context quiet and
literal: the signed-in shell should render one account-context header with the
current account title, kind, role, and short orienting copy, not a second
summary deck competing with the active task surface.
The same permission contract must also drive hosted `Support`: when
`can_manage` is false, the support shell may route the user back to
`Workspaces`, `Access`, or `Billing` only as review and owner/admin handoff
paths, not as live hosted mutation paths the current role can execute.
The same typed bootstrap/runtime contract must also keep inline workspace
counts and shell copy honest to account shape: hosted-only accounts may not
mention self-hosted billing utilities by default, and hosted view-only roles
must say when hosted billing still needs owner/admin authority.
The same permission contract must also drive the compact account-context
summary: the strip may not describe full hosted access-control or billing
ownership when the current role can only review workspaces or roster state.
That same typed runtime contract must also normalize account-role labels before
render: customer-facing copy may say `Owner`, `Admin`, `Tech`, or `Read-only`,
but it must not surface raw runtime identifiers such as `read_only` or legacy
aliases such as `member`.
That same runtime contract must also keep the first available action
permission-honest for hosted view-only accounts: when no ready workspace
exists, the primary route must stay on reviewable `Workspaces` or `Access`
surfaces before any blocked hosted billing or owner/admin-only mutation path.
That same shared request/runtime boundary must also preserve task-specific
failure copy on transport errors: portal job surfaces may not leak raw strings
such as `Network error.`, and must instead surface the owned fallback for the
exact action that failed.
That same typed summary contract must also keep `Ready` honest when no hosted
workspace exists yet: hosted accounts with zero workspaces may not route the
user into current workspace review, and must instead render that nothing is
ready until the first hosted workspace exists.
That same typed summary contract must also keep `Needs attention` honest when
only suspended workspaces remain: hosted workspace history alone may not make
the shell imply that active work is ready.
That same typed summary contract must also stay fact-first: summary copy may
not synthesize urgency or health verdicts such as `Nothing urgent` or
`Healthy now`, and must instead render concrete counts, explicit workspace
state, and next-action routing from the owned runtime payload.
That same typed portal runtime contract must also keep task and status copy
literal across the account surface: customer-facing wording may not use
commentary such as `obvious`, `actual work`, `trustworthy`, or `settled` when
the runtime already knows the concrete state, action, or failure being
rendered. The same typed contract applies to shell badges, section labels,
context chips, route labels, and error headings: they must render the exact
action or state (`Manage access`, `Hosted billing attached`, `Email support`,
`Failed to load roster`) instead of shorthand such as `Manage`, `Hosted`, or
generic alert labels. Support copy is part of the same typed contract:
escalation surfaces must render short literal path/account/action wording
instead of longer procedural prose.
That same typed `Access` contract must also keep the idle managed roster
structurally honest: when no remove job is active, the roster remains a
two-column review surface for operator and role. The third action column
appears only for the live remove-access job instead of repeating fake idle row
state.
That same typed portal page contract also owns favicon cache-busting: the
rendered `<link rel="icon">` must point at the shared `/favicon.svg` asset
through a versioned href so new portal icon revisions bypass browser cache on
deploy instead of waiting for asset expiry.
That same typed portal page contract must also preserve a calm, flat
account-tool visual posture across all portal scenarios: no gradients, heavy
shadows, or decorative dashboard chrome. The shell uses a compact identity bar
(account name, role, kind) and a horizontal tab bar for Workspaces, Access,
Billing, and Support. Content panels render directly below the tab bar without
reintroducing a second shell-level hero, overview panel, summary deck, or
metric grid ahead of the active task. The `Workspaces` panel may own one
section header, one quiet inline facts line, and one inline next-action row
above the workspace list when those elements are part of the same task
surface; they must not drift into a separate overview destination or duplicate
context strip. Action buttons (Create workspace, Invite people, Change roles,
Remove access) are integrated into toolbar rows within their respective
bordered data cards rather than existing as free-floating elements above
content. Hierarchy is driven by spacing, typography, and 1px borders rather
than cards, pills, stacked metrics, or ornamental side rails competing with
the active task.
That same typed page contract also applies before auth: the signed-out portal
surface must keep one obvious sign-in action plus precise account-scope
presentation, instead of falling back to a separate marketing-like hero and
generic login card that drifts away from the owned account shell model.
plus a package-local `tsc --noEmit` gate, so future account-shell work should
extend the typed source boundary instead of reviving opaque global runtime
objects, document-wide render events, or untyped embedded asset edits.
Hosted Pulse Cloud tenant-org AI reads now also follow that same canonical
rule: `internal/api/ai_hosted_runtime.go`, `internal/api/ai_handlers.go`,
`internal/api/ai_handler.go`, and `internal/api/hosted_billing_state.go`
must derive bootstrap and runtime readiness from the effective hosted billing
lease, falling back to the machine-owned `default` lease when a tenant org
has no org-local billing state, so `/api/settings/ai`, `/api/ai/status`, and
`/api/ai/sessions` cannot drift across separate entitlement interpretations.
The shared API-token management surface now also preserves canonical local
operator identity when explaining where a token is currently in use. Runtime
and infrastructure usage labels in the revoke flow keep the local instance
name for Docker hosts, agents, PBS, PMG, and similar monitored systems
instead of replacing those identities with governed summary text, so
revocation decisions remain instance-specific and auditable.
The unified resource API payload now carries the richer domain facets directly
through the owned backend response: resource objects can expose canonical
`capabilities`, `relationships`, `recentChanges`, and derived `facetCounts`
in addition to policy and identity metadata, so the backend payload contract
stays aligned with the timeline and control-plane model instead of flattening
those fields away. The frontend consumer, however, only preserves the
timeline-first `recentChanges` slice and its counts on the bundle contract.
Relationship facets include explicit `resource.relationships` plus the
canonical parent edge derived from `ParentID` by the unified-resource model
when no equivalent edge already exists, so topology hydration stays
backend-owned and one-hop relationship maps do not vary by page.
The same resource contract now also exposes a dedicated
`/api/resources/{id}/timeline` history endpoint and bundled facet reads under
`/api/resources/{id}/facets`, so operators can inspect change history without
depending on a monolithic resource payload.
The recovery API boundary now also keeps canonical platform vocabulary
consistent on both sides of the transport. `/api/recovery/*` queries use
`platform` as the operator-facing filter key, and the point/rollup payloads
now expose `platform` / `platforms` as the primary response fields while
legacy `provider` aliases remain compatibility-only for older decoders.
The reporting API contract now also treats current-state fleet inventory as a
first-class surface separate from historical metrics reports.
`internal/api/reporting_inventory_handlers.go`,
`internal/api/router_routes_licensing.go`, and the settings reporting shell now
own `/api/admin/reports/catalog` as the canonical operator-facing reporting
catalog plus `/api/admin/reports/inventory/vms/export` as the stable VM
inventory sub-contract. The catalog endpoint owns the reporting panel title,
description, locked-shell teaser copy, enabled-shell guidance copy,
historical performance report options, and nested VM inventory definition,
while the export endpoint remains the
spreadsheet-shaped CSV transport. That
export is intentionally not comment-prefixed like the legacy metrics CSV, and
it now carries Proxmox pool membership from the canonical unified VM runtime
model instead of inferring or reconstructing that field locally inside the
frontend or handler.
That same catalog payload also owns the optional performance-report capability
surface: `supportsMetricFilter` and `supportsCustomTitle` are contract flags,
not UI hints, so frontend consumers and request builders must not render or
emit unsupported metric-filter or custom-title fields from local assumptions.
The same reporting catalog and inventory export definitions also own backend
transport validation and download semantics. `internal/api/metrics_reporting_handlers.go`
and `internal/api/reporting_inventory_handlers.go` must derive allowed formats,
default format selection, multi-resource limits, optional metric/title field
emission, canonical default-title fallback, default fallback range window,
attachment filename stems, single-report filename subject, filename date-stamp
style, and invalid-format validation copy from the canonical reporting
definitions instead of hardcoding a second local contract.
Frontend consumers may still keep a local fallback filename for defensive
download behavior, but when the server returns `Content-Disposition` they must
prefer that attachment filename as the canonical transport output.
That same catalog contract is also authoritative for frontend request builders:
consumers may validate or reject malformed payloads, but they must not invent
replacement report endpoints, filename prefixes, export routes, or default
range windows from frontend-local fallback constants once the catalog has been
accepted.
Reporting time windows follow the same rule: `start` and `end` stay optional,
but when present they must parse as RFC3339 and `end` must not be earlier than
`start`; invalid values are a `400 invalid_time_range` transport failure, not a
silent fallback to the default reporting window.
The same transport contract also owns reporting-field and body validation:
`metricType` must stay within the governed character set/length, `title` must
stay within the governed length cap, and the multi-report JSON body must remain
strictly parsed with the canonical size ceiling, unknown-field rejection, and
no trailing payload tolerance instead of accepting malformed operator input and
drifting onward.
Those validation failures also keep stable API error codes owned by the backend
contract itself; handlers must not infer `invalid_metric_type`,
`invalid_title`, or similar response codes by parsing their own human-readable
error text.
The catalog route itself is intentionally metadata-readable without the
`advanced_reporting` feature gate so locked admin surfaces can present the same
canonical reporting definition before upsell, while report generation and
inventory export remain feature-gated execution routes.
That metadata route is still a version boundary as well. Current Pulse servers
must expose `/api/admin/reports/catalog`, but frontend consumers may treat a
`404` from that route as an old-backend compatibility signal and fall back to
the legacy report-generation transport only; they must not synthesize or guess
the newer catalog-owned inventory export contract when the backend does not
provide it.
The licensing API must also stay internally coherent in local dev mode. When
backend feature gates are widened by `PULSE_DEV=true` or demo/mock mode,
`/api/license/runtime-capabilities` must advertise the same capability set in
`capabilities`; it must not leave frontend shells on stale free-tier gating
while backend `HasFeature()` already treats those features as available.
That widening still has to respect runtime feature flags. A capability like
`multi_tenant` must stay absent from dev/demo entitlement payloads until the
process also has `PULSE_MULTI_TENANT_ENABLED=true`; otherwise admin shells
drift into impossible routes that the same backend still rejects as disabled.
The same rule applies to placeholder or plan-marker capabilities as well:
dev/demo entitlement payloads must not advertise non-operable entries like
`white_label`, `multi_user`, or `unlimited` just because they exist in tier
metadata, when the current runtime does not expose a corresponding usable
feature surface.
The `/api/resources` serializer now also refreshes canonical identity and
policy metadata through the shared unified-resource helper before it writes
the payload, so backend and frontend contract tests stay aligned on one
canonical metadata pass instead of consumer-local attach wrappers.
Those history reads now also accept governed `kind`, `sourceType`, and
`sourceAdapter` query filters, and the backend store owns the corresponding
filtered counts, so the timeline contract can narrow by change class and
adapter provenance without inventing a frontend-only relationship slice.
The same facet bundle contract now also returns grouped `recentChangeKinds`
counts by canonical `ChangeKind`, so the shared drawer and summary chips can
show the distribution of restarts, anomalies, state transitions, and other
timeline classes without guessing from the loaded slice.
The same facet bundle contract now also returns grouped
`recentChangeSourceTypes` counts by canonical source type, so the shared
drawer and summary chips can distinguish platform events, pulse diffs,
heuristics, user actions, and agent actions without inventing frontend-local
provenance heuristics.
The same facet bundle contract now also returns grouped
`recentChangeSourceAdapters` counts by canonical source adapter, so the
shared drawer and summary chips can distinguish Docker, Proxmox, TrueNAS, and
ops-helper provenance without inventing frontend-local integration heuristics.
Client consumers of the node setup transport must keep setup/install requests
on the NodesAPI client and must not reintroduce local trial-start helpers or
client-side trial status-code maps inside node setup state. Any commercial
handoff from setup must use explicit plan, activation, recovery, support, or
hosted routes governed by presentation policy.
That same frontend/API split now also requires node setup state to keep
commercial posture out of the NodesAPI transport contract. `useNodeModalState.ts`
may route explicit plan or support handoff through governed commercial helpers
where presentation policy allows, but it must not show trial CTAs or repurpose
raw commercial-posture fields as if they were part of setup transport.
Canonical timeline entries now also preserve correlation context in
`relatedResources`, so the history surface can explain which neighboring
resources moved with restart, anomaly, config, state transition, and
relationship changes instead of only exposing correlation endpoints when the edge
itself changed.
Restart timeline entries are also a first-class contract now: `restart` change
kinds can serialize Docker and Kubernetes restart metadata instead of being
folded into generic state transitions.
Incident-driven anomaly entries are also a first-class contract now:
`metric_anomaly` change kinds can serialize canonical incident rollup changes
instead of being flattened into generic status churn.
For relationship changes, the `from` and `to` fields now summarize the actual
edge(s) rather than only the parent pointer, so the API contract keeps the
relationship transition legible even before the frontend expands the
related-resource chips.
The same relationship and change presenters now also own the state, restart,
incident, and config summary fragments that feed those timeline values, so the
API surface preserves the canonical wording before the frontend renders it.
Invalid `sourceAdapter` values are rejected at the API boundary, so the filter
contract stays aligned with the canonical adapter set rather than silently
falling back to an empty slice.
The same resource-timeline contract now also owns canonical parsing for
`kind`, `sourceType`, and `sourceAdapter` query values, so the HTTP handler
stays thin and the change model remains the source of truth for timeline
filter validation.
The same API contract now also exposes the unified-resource control-plane
history through dedicated enterprise audit reads. The action, lifecycle, and
export history endpoints live in `internal/api/activity_audit_handlers.go` and
`internal/api/router_routes_licensing.go`, and the contract tests now pin their
response shapes so the execution trail remains queryable through the governed
API surface rather than only through the underlying store.
The infrastructure platform-connections contract now also owns TrueNAS
connection CRUD under `internal/api/truenas_handlers.go` and
`internal/api/router_routes_registration.go`. `/api/truenas/connections`
must stay the canonical API-backed platform boundary for listing, creating,
updating, deleting, and testing TrueNAS integrations, and `PUT` updates must
preserve masked secrets (`********`) instead of clearing stored API keys or
passwords when operators edit non-secret fields from the settings surface.
Draft validation must stay on `POST /api/truenas/connections/test`, while
re-testing one saved connection must route through
`POST /api/truenas/connections/{id}/test` so the server reuses stored secret
material instead of forcing the frontend to round-trip redaction placeholders
back through the draft-test API. That saved-connection test route must also
accept the edit-form payload for an existing connection and merge unchanged
masked secrets server-side, so editing operators can test changed host / port /
TLS fields before saving without re-entering retained credentials. For
row-level saved-connection tests with no edit overlay payload, that same route
must update the canonical TrueNAS poll summary owner so subsequent
`/api/truenas/connections` reads reflect refreshed last-success or last-error
state instead of leaving settings health disconnected from manual operator
tests.
That same route family now also owns pre-save monitored-system grouping
preview. `POST /api/truenas/connections/preview` and
`POST /api/truenas/connections/{id}/preview` must return the shared
monitored-system ledger preview contract sourced from canonical
unified-resource projection, including current/projected grouped systems rather
than page-local settings estimates, cap verdicts, or provider-local
counters.
That same `/api/truenas/connections` list boundary now also owns the
operator-facing runtime summary for those configured connections. The list
response must carry the canonical redacted config together with poll health
(`intervalSeconds`, last success/failure, consecutive failures) and discovered
platform contribution summary (host/resource identity plus systems, pools,
datasets, apps, VMs, shares, disks, and recovery artifacts) so the platform-connections
workspace can render real API-backed status and handoff context without
inventing a settings-local shadow fetch path. `frontend-modern/src/api/truenas.ts`
owns the browser normalizer for that observed summary and must preserve those
native inventory facets instead of collapsing them into a generic app or
storage count. Zero-value legacy
`pollIntervalSeconds` config must normalize back to the canonical 60-second
default at this same boundary instead of leaking ambiguous `0` values to the
frontend.
That same `/api/truenas/connections` boundary also owns explicit disabled-path
semantics: the `truenas_disabled` response exists only when the server has
explicitly opted out of the default-on TrueNAS integration, not as the normal
bootstrap state for a supported platform.
That same platform-connections boundary therefore defines the current TrueNAS
onboarding floor for Pulse. Supported now means operators can bootstrap
TrueNAS through the shared Infrastructure onboarding flow
(`Platform connections` may remain the operator-facing setup-wizard label,
but it lands on `/settings/infrastructure?add=pick` and normalizes into the
shared workspace) and `/api/truenas/connections` without the unified agent,
preserve masked secrets on ordinary edits, retest saved connections through
the stored-secret path, and see last-sync plus discovered contribution
summaries on the same settings surface. Pulse does not promise a separate
TrueNAS-only onboarding wizard, agent-required bootstrap, or public
provider-local app/log/config APIs at this floor.
That same infrastructure platform-connections contract is also the only
acceptable public backend boundary for the admitted VMware vSphere phase-1
direction. `/api/vmware/connections` must be the canonical
admin-only route family for listing, creating, updating, deleting, and testing
stored `vCenter` integrations under one saved-connection model. A green draft
or saved-connection test must mean the declared phase-1 floor is reachable
through the backend runtime, not merely that one of VMware's API families
answered. Pulse may keep separate vSphere Automation API and VI JSON clients
under that one saved connection, but the public API contract must hide that
multi-client runtime detail behind one canonical health and contribution
summary surface. Phase 1 must also keep the negative space explicit: no public
`/api/vmware/hosts`, `/api/vmware/vms`, `/api/vmware/datastores`,
`/api/vmware/events`, `/api/vmware/tasks`, or VMware control routes should be
introduced while inventory, alerts, history, and Assistant reads still route
through the shared canonical Pulse surfaces.
That same `/api/vmware/connections` family now also owns the current phase-1
implementation contract under `internal/api/vmware_handlers.go`,
`internal/api/router.go`, `internal/api/router_routes_registration.go`, and
`frontend-modern/src/api/vmware.ts`. The list response must carry one redacted
stored connection shape plus canonical `poll` health and `observed`
contribution summary (`hosts`, `vms`, `datastores`, `networks`, `viRelease`) so
the shared settings workspace can render VMware status without another
provider-local inventory route. When base inventory succeeds but optional signal or topology
reads degrade, that same `observed` payload must carry the canonical
partial-success shape (`degraded`, `issueCount`, summarized `issues`) instead
of collapsing the whole connection to `poll.lastError` or pretending the
refresh was fully healthy. That `poll` payload is the canonical runtime contract:
backend handlers must source it from the poller-owned per-connection summary,
saved row-level retests with no payload must refresh that same summary owner,
and edit-form overlay tests must preserve the stored summary until a real save
succeeds. Compatibility acceptance of a historical `test` field may exist only
inside shared frontend normalization; the backend route family itself must stay
on `poll` for the operator-facing response model. `POST /api/vmware/connections/test`
must stay the draft test surface, while `POST /api/vmware/connections/{id}/test`
remains the saved connection retest surface. The explicit disabled path also
stays on this boundary: `404 vmware_disabled` means the operator or runtime has
opted out of the default-on VMware candidate, not that the platform requires a
different onboarding contract.
That same route family now also owns source-native monitored-system grouping
preview. `POST /api/vmware/connections/preview` and
`POST /api/vmware/connections/{id}/preview` must project the discovered
provider-backed record set through the shared monitored-system ledger preview
contract before persistence, including current/projected grouped systems rather
than cap verdicts or a handler-local vCenter candidate estimate.
That same TrueNAS and VMware platform-connections contract now also owns
runtime mock continuity. When `/api/system/mock-mode` flips on a running
server, `/api/truenas/connections` and `/api/vmware/connections` must
immediately return the canonical mock connection payloads without restart, and
the shared `/api/resources` surface must expose the corresponding platform
inventory through `source=truenas` and `source=vmware-vsphere`. Shared query
parsing may accept `vmware-vsphere` as the operator-facing VMware alias, but
the emitted canonical resource source remains the shared `vmware` source
family rather than a second backend source key.
That same VMware test contract now also owns structured setup-failure
classification. When `POST /api/vmware/connections/test` or
`POST /api/vmware/connections/{id}/test` fails, the backend payload must
preserve the canonical top-level `code` plus string-valued `details.error` and
`details.category`, and shared browser normalization in
`frontend-modern/src/utils/apiClient.ts` plus
`frontend-modern/src/api/responseUtils.ts` must carry that metadata through the
shared error object without inventing a VMware-only fetch or parsing path.
That same TrueNAS and VMware platform-connections contract now also owns
per-surface scope as a first-class field on the connection shape. The
TrueNAS connection payload must carry positive `monitorDatasets`,
`monitorPools`, and `monitorReplication` booleans; the VMware connection
payload must carry positive `monitorVms`, `monitorHosts`, and
`monitorDatastores` booleans. `internal/config/truenas.go` and
`internal/config/vmware.go` must default those fields to true on new
instances and migrate legacy all-false records to all-true inside
`ApplyDefaults`, so existing `truenas.json` and `vmware.json` on disk
continue monitoring every surface after upgrade. The unified
`/api/connections` aggregator in `internal/api/connections_aggregator.go`
must project those booleans into the connection row's `scope` map and
declare `capabilities.supportsScope: true` for TrueNAS and VMware rather
than hard-coding an all-true scope; `frontend-modern/src/api/truenas.ts`
and `frontend-modern/src/api/vmware.ts` must round-trip the booleans
through their `normalize*Connection` and `serialize*ConnectionInput`
helpers without dropping them on edit-save.
That same VMware API boundary now also owns the phase-1 runtime negative
space around inventory projection. `internal/api/router.go` may wire VMware's
supplemental ingest into the shared `/api/resources` surface so canonical
`agent`, `vm`, `storage`, and `network` records can appear elsewhere in Pulse,
but the public backend contract must still stop at `/api/vmware/connections*` for
provider-local routes. Phase 1 must not add public `/api/vmware/resources`,
`/api/vmware/history`, `/api/vmware/alerts`, or VMware-specific recovery
transport just because the internal poller now projects VMware-backed
resources into the shared canonical inventory.
That same shared API contract now also owns Assistant mention transport for
those canonical resources. `frontend-modern/src/api/aiChat.ts`,
`internal/api/ai_handler.go`, and `internal/api/ai_handlers.go` must preserve
structured mention payloads for canonical `agent`, `vm`, `storage`, and
`network` resources as shared unified-resource IDs plus shared mention types,
so VMware-backed reads stay on `/api/ai/*` and `/api/resources*` instead of
introducing VMware-only mention payloads or provider-local inventory reads
under `/api/vmware/*`. Runtime-specific container/app mentions remain shared
unified-resource mentions as well; VMware network inventory does not create a
provider-local mention family.
That same `/api/ai/chat` payload boundary owns per-request execution-mode
overrides. Dashboard Pulse Brief and other scoped handoffs may include
`autonomous_mode:false` on the chat request to force approval-required command
execution for that exchange, but the transport must treat the field as a
request override only and must not mutate the user's persistent AI control
setting.
That same chat transport boundary owns new-session anchoring. When the request
omits `session_id`, the handler may generate and stream a session ID
immediately so the browser can anchor the visible turn, but that generated ID is
not evidence of a persisted conversation. Stored finding or handoff recovery may
run only for a client-supplied existing `session_id`; generated IDs must enter
the chat service directly and let the service create the session without first
probing persisted handoff metadata or legacy session files.
The same chat payload boundary now carries scoped product handoff context:
`handoff_context` is bounded model-only text, `handoff_resources` are structured
resource references used to seed canonical resource-policy, state, relationship,
and timeline hydration, and `handoff_actions` are structured approval/action
references used to seed canonical approval and action-audit refresh.
`handoff_metadata` is the browser-safe identity envelope for restoring saved
product handoffs, currently including Patrol run kind, run ID, safe run
type/status, a runtime-failure boolean rather than runtime failure detail, and
bounded resource/action counts when available. Resource drawer handoffs use
`handoff_metadata.kind=resource_context` with a structured `handoff_resources`
reference and no browser-authored prompt or model text; the backend chat
runtime must hydrate the shared resource context pack from canonical resources
instead of trusting the browser to serialize rich context. Context-only
resource handoff questions must remain context-first at the API/runtime
boundary: unless the operator explicitly asks for discovery execution, live
verification, or a read attempt, the runtime withholds tools and returns the
attached context or an explicit missing-fact answer rather than treating the
question as permission to call discovery/read tools. Frontend-visible Patrol
assessment briefings must not render recommendation fields as separate title,
reason, route-action facts, or prompt chips; the configured model owns those
decisions from the structured handoff metadata and bounded chat context.
Frontend handoff builders may send these fields for owned alert, incident,
Patrol assessment, Patrol finding, or Patrol run-history context, but the
backend must not persist them as user-authored message text, and the frontend
must not prefill or auto-submit a product-authored user prompt from them. They
are context only and must be treated as explanation/review context only. When a
Patrol `finding_id` resolves,
backend-refreshed durable finding context remains canonical; the handler may
merge only recognized same-finding Patrol product handoff text and same-finding
resource/action references as secondary model-only review context, while
dropping mismatched references and raw command payload lines so the model cannot
continue from stale or spoofed Patrol authority.
The `/api/ai/sessions` response may expose `handoff_summary` for sessions that
carry private Assistant model-context metadata, but that payload is a safe
reload marker only. It may carry the handoff kind, finding ID, Patrol run ID,
safe run type/status/runtime-failure flags, counts, primary-resource label,
last-known approval/action status, risk level, and summary timestamp; it must
not serialize the model-only `handoff_context`, runtime failure detail, action
preflight/result bodies, remediation descriptions, raw commands, or approval
command payloads, and it must not preserve Patrol-authored next-step
recommendation fields from legacy handoffs.
Patrol finding handoffs are stricter than ordinary chat requests: when a request
carries a non-empty `finding_id` or resolves to model-only Patrol briefing,
resource, or action context, `internal/api/ai_handler.go` must clamp the
request-local autonomous mode to false even if the caller supplied
`autonomous_mode:true`. That server-side clamp is part of the public API
contract because the frontend handoff setting is only advisory unless the
backend preserves the approval-required boundary.
That same backend API boundary now also owns the negative space around
assistant control. Wiring native TrueNAS app actions into
`internal/api/router.go`, `internal/api/ai_handler.go`, or adjacent backend
helpers must not introduce a parallel public `/api/truenas/apps/...` control
surface; provider-backed app control for Pulse Assistant stays behind the
shared AI runtime tool contract unless this API contract changes in the same
slice.
That same negative-space rule also applies to assistant diagnostics. Wiring
native TrueNAS app log reads into `internal/api/router.go`,
`internal/api/ai_handler.go`, or adjacent backend helpers must not introduce a
parallel public `/api/truenas/apps/.../logs` surface; provider-backed app log
reads for Pulse Assistant stay behind the shared `pulse_read` runtime tool
contract unless this API contract changes in the same slice.
That same negative-space rule also applies to assistant configuration reads.
Wiring native TrueNAS app config into `internal/api/router.go`,
`internal/api/ai_handler.go`, or adjacent backend helpers must not introduce a
parallel public `/api/truenas/apps/.../config` surface; provider-backed app
config for Pulse Assistant stays behind the shared `pulse_query
action="config"` runtime tool contract unless this API contract changes in the
same slice.
The monitored-system ledger contract now also carries a canonical grouping
explanation payload. `/api/license/monitored-system-ledger` must expose the
shared monitored-system explanation summary, sanitized grouping reasons, and
included top-level surfaces exactly as the unified-resource resolver computed
them, while the frontend client stays in lockstep with that nested payload
shape.
That same ledger contract must also preserve the canonical monitored-system
status enum end to end. Backend normalization may fail closed for unsupported
values, but it must not flatten governed `warning` state to `unknown`, because
the billing and inventory surfaces need the real top-level runtime status the
unified-resource resolver computed.
That same contract now also owns the backend-authored status explanation paired
with that enum, and the monitored-system ledger details surface must render it
alongside the counting explanation instead of inventing page-local wording for
what online, warning, offline, or unknown means.
That nested status explanation is now a structured contract, not summary-only
copy: `/api/license/monitored-system-ledger` must preserve the canonical
summary plus the ordered reason list from unified resources, including the
degraded source or surface, its status, and its canonical `reported_at`
timestamp, so mixed fresh/stale grouped systems remain explainable through one
governed API shape.
That canonical summary must also carry the mixed-source freshness explanation
when the freshest grouped observation came from a different source than the
degraded one, so API consumers can show a fresh `Last Seen` value without
making warning or offline state look contradictory.
That freshest grouped observation is now canonically exposed as the structured
`latest_included_signal` object. Its `at`, `source`, `name`, and `type` fields
identify exactly which included top-level surface reported most recently.
The backend payload contract now emits only that structured object, and the
frontend monitored-system client should parse that canonical wire contract
directly rather than keeping flat alias fallback for
`latest_included_signal_at`, `latest_included_signal_source`, or `last_seen`.
The canonical nested status-reason timestamp is `reported_at`, and the
normalized client contract must expose only that field.
That same monitored-system ledger contract now also owns prospective
explanation. `POST /api/license/monitored-system-ledger/preview` must accept
one canonical candidate plus an optional structured replacement selector and
return the canonical current/projected count delta, effect label, and
current/projected ledger entries produced by the shared monitored-system
projection layer instead of by handler-local heuristics. It must not return an
enforced-limit verdict or make persistence depend on a commercial volume
decision.
Configured Proxmox, PBS, and PMG update handlers in
internal/api/config*node_handlers.go may use that same structured
replacement-selector contract when they explain monitored-system grouping:
source-owned names, host URLs, hostnames, and resource identifiers may cross
the API boundary, but handler-local matcher closures must not become the
source of truth for replacement identity or save-time admission.
Provider-backed preview routes such as `/api/truenas/connections/preview`,
`/api/truenas/connections/{id}/preview`, `/api/vmware/connections/preview`,
and `/api/vmware/connections/{id}/preview` must serialize that same canonical
preview shape directly; they may not down-scope the response to local counts or
hide current/projected grouped systems from the governed contract.
That same platform-connections preview contract now also owns candidate-state
defaulting. New connection preview and test payloads must inherit the
canonical provider default `enabled=true` when the field is omitted, while
saved-connection preview, test, and update payloads must preserve the stored
`enabled` state unless the request explicitly changes it. Shared handlers may
not let zero-value JSON decode silently turn an unchanged connection into an
inactive monitored-system candidate.
Inactive TrueNAS and VMware candidates must stay on that same canonical API
contract as zero-delta or removal-only previews. Those routes may not fail
validation just because no projected monitored-system rows remain once the
candidate is treated as non-counting.
That client contract must also fail closed when older or partial payloads omit
the nested explanation object: the frontend may normalize missing explanation
fields to empty reasons/surfaces plus a safe default summary, but it must not
crash or invent non-canonical grouping details.
That same frontend monitored-system client must not keep its own parallel
fallback copy for those summaries. When the payload omits frontend-authored
status or explanation text during mixed-version rollouts, the client should
source its safe default wording from the governed monitored-system
presentation helper instead of duplicating local strings inside the API
normalizer.
Action-plan stale-plan protection on those audit records now uses the canonical
`resourceVersion`, `policyVersion`, and `planHash` fields only, so the
response contract stays deterministic without extra version baggage.
The same API contract now also owns the dedicated frontend resource facet
client in `frontend-modern/src/api/resources.ts`, which fetches the governed
capability, relationship, and timeline surfaces from `internal/api/resources.go`
instead of teaching the drawer or list views to reconstruct them inline.
Those facet reads now explicitly include the selected resource's canonical
`capabilities` and `relationships`, so action affordances and relationship-map
surfaces remain hydrated from the same backend-owned resource contract as
recent changes and facet counts.
The relationship facet must call the shared unified-resource parent-edge
deriver rather than serializing only the raw resource relationship slice, so
resources parented through the registry still expose the same canonical
relationship-map contract as resources with adapter-supplied edges.
The same AI resource-intelligence payload now also carries dependency and
dependent correlation arrays plus correlation evidence, so the drawer can render
canonical correlation context from the shared AI contract instead of inferring it
from the relationship facet payload alone.
The same AI frontend client now also loads `/api/ai/intelligence/correlations`
through the shared `frontend-modern/src/stores/aiIntelligence.ts` store for
the Patrol intelligence page and the AI summary page, so the
learned-correlation list is governed by the same API contract that backs the
resource drawer's correlation evidence instead of being fetched as page-local state.
That correlations route now reads through the canonical AI intelligence
facade first, so the handler and its payload keep the detector behind one
shared access layer instead of routing directly to Patrol-local correlation
state.
That store now also owns the Patrol page load bundle, so the page refresh path
stays aligned on one store-owned orchestration layer instead of re-encoding
the AI bundle inline. Frontend Patrol load orchestration must treat first-load
transport or settings failures as stale-data state rather than throwing through
the route: the page stays mounted, preserves any last-known Patrol evidence,
and exposes a retry affordance while backend/API failures remain available to
debug logging and API-level diagnostics.
The AI summary page now also renders the canonical
`frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
card for policy posture, so sensitivity, routing, and redaction counts are
presented through one governed frontend component while the resource drawer
keeps only the per-resource policy lines.
The unified action, lifecycle, and export audit reads now also clamp oversized
`limit` requests to the governed maximum of `1000`, so the control-plane audit
surface stays bounded even when callers ask for arbitrarily large history
pages.
Unified action audit payloads must also expose the normalized action plan
preflight through `plan.preflight`: API consumers should see whether a dry-run
was available, what safety checks were recorded, and what verification steps
remain, instead of inferring action safety from free-form result text.
The frontend action-audit client in `frontend-modern/src/api/actionAudit.ts`
and its typed payload mirror in `frontend-modern/src/types/actionAudit.ts` are
the canonical browser-side reader for `GET /api/audit/actions`; resource
workflow surfaces must consume that client instead of rebuilding audit query
URLs or silently ignoring the endpoint's gated-unavailable state.
The frontend resource-action client in `frontend-modern/src/api/resourceActions.ts`
is the canonical browser-side writer for `/api/actions/plan`,
`/api/actions/{id}/decision`, and `/api/actions/{id}/execute`; Docker / Podman
container lifecycle controls and future resource workflow buttons must consume
that client rather than posting directly from feature-local fetch helpers.
Those relationship and timeline payloads now also carry `lastSeenAt` freshness
and optional metadata through the same owned contract, so the drawer can
preserve provenance without inventing a separate relationship-detail schema.
Relationship-change timeline entries now also use the canonical relationship
summary helper for their compact `from` and `to` wording, so the API keeps the
human-readable edge label aligned with the unified-resource relationship
presenter instead of reconstructing a local type-token summary.
The same `/api/resources/{id}/timeline` filter contract now also routes its
kinds, source types, and source adapters through the shared unified-resource
change-filter parser, so API validation stays owned by the change model rather
than being re-parsed separately in the HTTP handler.
The tenant-scoped unified resource API now also stays on canonical
unified-resource seeds end to end: `internal/api/resources.go`,
`internal/api/router_helpers.go`, and `internal/api/state_provider.go` no
longer treat raw tenant `StateSnapshot` data as a live registry-seeding owner
once `UnifiedResourceSnapshotForTenant` is available.
The router now wires the tenant resource state provider during initial setup
when a multi-tenant monitor is present, so non-default org resource list and
facet reads do not fall back to a missing-provider 500 during normal tenant
requests.
The unified infrastructure settings surface now also follows an explicit
shared boundary with agent-lifecycle. Changes to
`frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`,
`frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`,
`frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`,
`frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`,
and `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`
must carry this contract together with the shared agent-lifecycle contract and
the dedicated API proof files for token generation, agent lookup, profile
assignment, install/uninstall copy transport, and Proxmox setup/install flows,
rather than remaining unowned consumers of those contract surfaces.
That shared infrastructure-settings boundary must also stay under explicit
proof routing on both sides instead of relying only on generic owned-file
coverage on the API-contract side: token generation, agent lookup, profile
assignment, install/uninstall copy transport, and inline Proxmox credential
flows must continue to carry the direct proof paths together with the
lifecycle-side surface proof.
The same shared-boundary rule now applies to `frontend-modern/src/api/agentProfiles.ts`,
`frontend-modern/src/api/nodes.ts`,
`frontend-modern/src/utils/agentInstallCommand.ts`,
`internal/api/agent_install_command_shared.go`,
`internal/api/config_setup_handlers.go`, and `internal/api/unified_agent.go`:
agent install/register/profile control changes must preserve canonical API
payload behavior instead of drifting into subsystem-local transport rules.
That same shared boundary now assumes `InfrastructureWorkspace.tsx` owns the
top-level ledger shell, `frontend-modern/src/components/Settings/useConnectionsLedger.ts`
consumes the canonical `/api/connections` projection for configured rows, and
`InfrastructureInstallerSection.tsx` plus
`ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx` consume the shared
API-backed lifecycle state. The retired
`InfrastructureOperationsController.tsx` shell and
`useInfrastructureReportingState.tsx` reporting path must not be reintroduced
as parallel transport owners.
That same `/api/connections` projection also owns collection-method truth for
the infrastructure ledger: `useConnectionsLedger.ts` must derive one canonical
subtitle (`via platform API`, `via Pulse Agent`, or `via platform API and
Pulse Agent`) from the shared system/component payload instead of letting page-
local tables invent their own API-versus-agent badge heuristics.
That same `/api/connections` row contract now also owns the fleet-governance
projection consumed by the infrastructure workspace. `Connection.fleet` is the
canonical machine-readable source for enrollment state, liveness, version
drift, adapter health, config rollout, credential status, update posture, and
remote-control posture. Its nested `configDrift`, `rollout`,
`credentialHealth`, and `commandPolicy` objects carry the explicit
desired-versus-applied config fingerprints, staged rollout state, richer
credential validity/rotation facts, and command-policy enforcement state used
by Settings and CLI consumers. Frontend settings surfaces may format those
facts, but must not infer a second fleet state from row labels, error-message
text, or provider-local table heuristics.
Unmanaged host-agent defaults are not a fleet rollout. When the backend
resolved only the default signed config with no managed command policy or
agent-applied setting, `fleet.configDrift` must remain `not-applicable` and
`fleet.rollout` must remain `current`/`applied`; Settings may show deeper
diagnostics, but must not count that passive state as source setup attention.
Only a real managed desired config fingerprint can create pending rollout
attention when an applied fingerprint is absent or mismatched.
The `fleet.commandPolicy` object is the canonical desired/applied convergence
contract for remote command enablement. It must carry the desired server
policy, the applied agent-reported truth when available, the effective
enforcement state, and a bounded reason explaining convergence, drift, missing
report data, denial, or unsupported capability. Desired disabled with applied
enabled, and desired enabled with applied disabled, are both drift or
enforcement-attention states, not in-sync states. When no current agent report
exists, the applied side remains pending or unknown with an explicit no-report
reason, and the row must not be considered converged. Top-level
`remoteControl` may stay as compact presentation compatibility, but it must
not overstate desired server policy as applied agent runtime truth or collapse
desired/applied disagreement into one enabled/disabled fact.
That same shared infrastructure-settings boundary also owns install-profile
semantics surfaced by
`frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`:
the recommended auto profile may describe Proxmox auto-detect only as the
canonical unpinned runtime mode that lets the agent register every detected
local PVE or PBS service. Frontend copy on that shared model must not imply a
hidden single-type selection or invent a profile flag that the installer and
auto-register contract do not actually persist.
That shared `frontend-modern/src/api/agentProfiles.ts` boundary must also stay
under explicit proof routing on both sides instead of remaining a generic
frontend-client match on the API-contract side: assignment, delete, unassign,
and suggestion transport changes must carry the direct profile-client proof
together with the lifecycle-side profile proof.
That shared `frontend-modern/src/api/nodes.ts` boundary must also stay under
explicit proof routing on both sides instead of remaining a generic
frontend-client match on the API-contract side: Proxmox setup-script and
agent-install command transport changes must carry the direct lifecycle/client
proof together with a direct API-contract client proof.
That same rule also applies to the shared update transport surface:
`frontend-modern/src/api/updates.ts` and `internal/api/updates.go` must carry a
direct API-contract proof path instead of relying only on the generic frontend
client or backend payload fallback coverage.
That same rule also applies to the shared security transport surface:
`frontend-modern/src/api/security.ts`, `internal/api/security.go`,
`internal/api/security_tokens.go`, and `internal/api/system_settings.go` must
carry a direct API-contract proof path instead of relying only on the generic
frontend client or backend payload fallback coverage.
That same rule now applies to the shared backend lifecycle install/register
surface as well: `internal/api/agent_install_command_shared.go`,
`internal/api/config_setup_handlers.go`, and `internal/api/unified_agent.go`
must carry a direct API-contract proof path instead of relying only on the
generic `internal/api/` backend payload prefix.
That same backend-owned `internal/api/` boundary also includes the generated
embedded-frontend warning surface used during local development.
`internal/api/DO_NOT_EDIT_FRONTEND_HERE.md` must direct developers to edit
`frontend-modern/src`, identify `http://127.0.0.1:5173` as the hot-reload
frontend dev shell, and describe `http://127.0.0.1:7655` as the proxied
backend dependency instead of teaching `7655` as the browser-facing dev
entrypoint.
That shared frontend install-command helper must also stay under explicit proof
routing instead of remaining an orphan utility: changes in
`frontend-modern/src/utils/agentInstallCommand.ts` must carry the direct
helper proof path, not rely only on downstream consumer tests to catch
transport drift.
That same backend install-command contract must also normalize trailing slashes
on canonical base URLs before composing installer asset paths or response
payloads, so `/api/agent-install-command` and the governed container-runtime
token response cannot emit `//install.sh` or slash-suffixed `pulseURL`
transport when `PublicURL` or `AgentConnectURL` already ends with `/`.
That same governed container-runtime migration response must also preserve the
canonical lifecycle shell payload shape: `installCommand` in the diagnostics
docker prepare-token response may not emit the stale `--disable-host` alias or
an ad hoc `curl | sudo bash` pipeline, and must instead match the canonical
root-or-sudo wrapped install transport with `--enable-host=false`.
That diagnostics install-command payload must also be assembled through the
shared backend install-command helper in `internal/api/agent_install_command_shared.go`
instead of a handler-local shell formatter, so token omission, plain-HTTP
`--insecure`, and trailing-slash normalization stay under one canonical API
contract surface.
That same Docker / Podman diagnostics and admin-route API copy must preserve
the single installed-agent product identity: Docker / Podman telemetry is a
module reported by `pulse-agent`, not a separate customer-facing
Docker-specific agent product. Existing `/api/agents/docker/*` route names and
stable error codes may remain for compatibility, but response messages,
diagnostics notes, docs, and proof labels must use Docker / Podman module
language.
That same diagnostics boundary must also consume the canonical monitoring
memory-source catalog instead of maintaining a second local trust/fallback
classifier. Node, VM, and LXC memory-source aliases must normalize to the same
governed labels and fallback-reason contract before diagnostics memory-source
breakdowns are serialized.
That same diagnostics boundary must also backfill canonical fallback reasons
when a raw snapshot reaches the API layer without one, so
`buildMemorySourceDiagnostics` stays self-consistent even if a caller bypasses
`GetDiagnosticSnapshots()` and hands diagnostics a legacy alias directly.
That same diagnostics boundary now explicitly excludes maintainer analytics.
`internal/api/diagnostics.go` must not serialize commercial funnel, sales
funnel, pricing/checkout conversion, or infrastructure onboarding telemetry in
`/api/diagnostics`; local upgrade/onboarding metrics must not reappear as
normal product API routes, system settings fields, startup DB artifacts, or
customer frontend state. `frontend-modern/scripts/settings-diagnostics-boundary-audit.mjs`
now checks `internal/api/diagnostics.go` alongside the Settings diagnostics
frontend boundary through the canonical frontend audit runner, so this
admin-analytics payload class cannot return as a customer-visible diagnostics
contract. The same diagnostics boundary also keeps the first-party Assistant
surface out of MCP transport vocabulary: `AIChatDiagnostic` may report native
Assistant runtime availability through `assistantRuntimeConnected`, but
`mcpConnected` and `mcpToolCount` must stay out of `/api/diagnostics`.
That same admin-analytics boundary now retires the local commercial metrics
routes from the normal customer product API: `/api/upgrade-metrics/events`,
`/api/upgrade-metrics/stats`, `/api/upgrade-metrics/health`,
`/api/upgrade-metrics/config`, and `/api/admin/upgrade-metrics-funnel` must
return `404` in normal router contracts. Customer frontend surfaces must not
call local pricing, checkout, paywall, commercial funnel, or
infrastructure-onboarding signals, and must not keep customer-side
commercial/onboarding metrics wrapper modules as a compatibility layer.
That same public-demo API boundary must also hide runtime-admin operations
surfaces instead of treating them as harmless reads. Demo sessions must receive
`404` for `/api/diagnostics`, `/api/diagnostics/docker/prepare-token`, and the
shared `/api/logs/*` endpoints, so the preview shell cannot expose runtime
diagnostics, log streams, or downloadable log bundles behind a supposedly
read-only demo account. That same hidden read-side boundary includes `GET` and
`HEAD` reads for `/api/admin/users` and manual discovery at `/api/discover`;
public demo mode may block writes generically, but it must not reveal that
admin-user inventory or manual-discovery read routes exist.
That shared infrastructure install boundary now also preserves copied shell
command payload continuity: any privilege-escalation wrapper applied at
`frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
through `useInfrastructureOperationsState.tsx` must keep the full canonical
installer argument list intact instead of dropping token, profile, or
command-execution flags between display and clipboard transport.
That same shared infrastructure-settings boundary now also consumes the canonical
`connectedInfrastructure` projection from the backend state contract instead of
reconstructing reporting rows by merging raw unified-resource facets and
removed-* arrays in the browser. v6 clients no longer receive those removed-\_
arrays at all for this surface; Connected infrastructure row
identity, reporting-surface labels, and ignore/reconnect scope must be owned
by the backend payload contract, with frontend rendering limited to
presentation and operator actions.
That same install-command payload continuity now also applies when auth is
optional: copied install and upgrade commands must omit token arguments
entirely on token-optional Pulse instances rather than serializing a fake
sentinel token into the governed shell or PowerShell payload.
That same shared installer boundary must also stay on one runtime-argument
contract after the command is copied: `scripts/install.sh` may not rebuild
separate service-flag strings for token-bearing and token-file install paths,
and must instead derive persisted `--url`, optional `--token`, feature
toggles, identity flags, and disk-exclude transport from one canonical
installer-owned argument item list.
That same optional-auth contract now extends through the first governed
runtime transport boundary: post-install Unified Agent report requests and
Proxmox auto-register requests must use the canonical `authToken` request
field for one-time setup-token auth instead of any API-token auth header path,
so the canonical API surface does not preserve parallel auth transports or a
second auth meaning for the same field.
The self-hosted commercial entitlement payload now has no monitored-system
counted-unit contract. `max_monitored_systems`, `max_agents`, and `max_nodes`
may be decoded only at explicit legacy import, purchase-return, or scrubbing
boundaries, and must not be re-emitted as active runtime limits. Add-node,
auto-register, deploy, TrueNAS, VMware, Kubernetes, Docker, and other platform
registration routes must accept net-new monitored systems without commercial
volume admission, while the monitored-system ledger remains an explanatory
inventory/debug surface only.
That same retired-cap contract owns prospective grouping and replacement
projection without admission enforcement. Config-backed PVE/PBS/PMG, TrueNAS,
VMware, and other API-backed registration or update routes may preview
current/projected monitored-system grouping through the canonical resolver, but
the preview cannot carry `limit`, `would_exceed_limit`, commercial-denial
state, or any save-time cap override. Save-time errors should be ordinary
platform, auth, validation, or readiness failures; the shared frontend API
error path must not preserve a monitored-system cap preview from old 402
payloads.
Legacy v5/v6 continuity metadata is now scrub-only. `/api/license/status`,
`/api/license/runtime-capabilities`, `/api/license/commercial-posture`, and
`/api/license/entitlements` must not expose monitored-system capacity posture,
grandfathered floors, current/unavailable limit fields, or admission-freeze
copy. Historical `max_monitored_systems` values can remain in tests and
migration fixtures only to prove they are deleted before runtime
entitlements, claims, billing-state responses, or frontend presentation see
them.
That same configured-path contract now also has an explicit shared owner for
manual auth env files: `internal/api/auth_env_path.go` must remain the only
place that derives `.env` from configured runtime paths, and neighboring
handlers like `router.go`, `router_routes_auth_security.go`, and
`security_setup_fix.go` may not reconstruct their own `/etc/pulse/.env`
fallbacks once runtime path authority has been centralized.
That same monitored-system ledger boundary now also governs frontend client
normalization. `frontend-modern/src/api/monitoredSystemLedger.ts` must decode
mixed-version payloads into one normalized response shape before render
surfaces consume it: `status_explanation`, `explanation`, and
`latest_included_signal` are the client contract exposed to the UI, while
missing mixed-version fields may be repaired only inside that API client layer
rather than in panel-local fallback helpers.
That same shared API boundary rule now also applies to notification test
handlers: `internal/api/notifications.go` may decode webhook-test requests and
return the governed response envelope, but notifications-owned service-template
selection, safe header copying, and generic webhook-test payload fallback must
stay in `internal/notifications/` rather than becoming a second API-layer owner
for the same transport contract.
The notifications API boundary also carries the canonical webhook template
shape used by the frontend service chooser: `frontend-modern/src/api/notifications.ts`
must expose the registry's service label, description, and mention-copy
metadata, and it may not invent a second frontend-only service taxonomy for
the chooser.
That same notifications boundary must also canonicalize legacy service-specific
input aliases at ingress instead of leaving them as a live runtime contract:
Pushover `app_token` / `user_token` may be accepted only at config/API/UI input
boundaries, and API responses plus live notification runtime state must carry
only canonical `token` / `user` fields.
That same shared owner now also governs writable auth env target order:
setup, password-change, and auth-status flows must route `.env` writes through
the shared helper instead of open-coding config-path writes plus ad hoc
data-path fallback branches in each handler.
Those shared profile-assignment settings surfaces must also preserve canonical
assignment visibility when an assignment references a profile ID that no longer
resolves in the fetched profile collection: the current payload state must stay
visible to the operator instead of collapsing into an empty/default select
value that misstates the backend assignment.
That same shared install-command boundary must preserve selected Proxmox target
profiles across PowerShell transport:
`frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
and `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`
must emit `PULSE_ENABLE_PROXMOX` and `PULSE_PROXMOX_TYPE` when the operator
copies a Windows install command for a Proxmox-targeted flow, and
`scripts/install.ps1` must convert those env vars back into canonical
`pulse-agent` service args so the copied payload does not drift from the
governed shell command contract.
That same shared PowerShell install transport must also preserve
operator-selected insecure TLS and command-execution settings: copied Windows
install and upgrade payloads must emit `PULSE_INSECURE_SKIP_VERIFY` and
`PULSE_ENABLE_COMMANDS` when enabled, and copied Windows uninstall payloads
must still emit `PULSE_INSECURE_SKIP_VERIFY` when enabled, so
`scripts/install.ps1` does not silently drop self-signed transport intent on
the Windows path.
That same shared lifecycle transport must also preserve explicit custom CA
selection end to end: copied shell install, upgrade, and uninstall payloads
must pass `--cacert` to both the outer installer download and the governed
installer runtime, while copied Windows install, upgrade, and uninstall
payloads must emit `PULSE_CACERT` and use a PowerShell bootstrap that applies
custom-CA or insecure-TLS certificate handling before `install.ps1` is fetched,
not only after the installer starts executing. That bootstrap must accept the
same PEM/CRT/CER trust input that `scripts/install.ps1` itself accepts, so the
shared command contract does not narrow custom-CA behavior on the first fetch.
That same shell transport contract also applies to the governed setup-completion
install handoff in `SetupCompletionPanel`: when the operator supplies a custom CA path
or opts into insecure/self-signed transport, the shared Unix install builder
must carry those choices through both the outer `curl` fetch and the installer
runtime instead of leaving the first-session onboarding path behind the shared
lifecycle/API contract. For explicit insecure/self-signed mode, that first-hop
fetch must widen to `curl -kfsSL`; preserving `--insecure` only on the later
installer runtime is not sufficient.
That same shared lifecycle/API boundary must also keep setup-script bootstrap
transport under one owned backend shape: `/api/setup-script-url` response
payloads and `/api/setup-script` rerun guidance must derive URL, download URL,
file name, token hint, and env/non-env command variants from one canonical
bootstrap artifact builder instead of duplicating those fields in separate
handler-local payload assembly paths.
That same owned setup-script contract now also covers the rendered shell body:
PVE and PBS script text must come from shared backend render helpers instead of
remaining duplicated inside the setup handler, so the API boundary owns one
artifact contract plus one render path rather than a route-local script engine.
That owned backend shape must itself stay singular: the shared setup artifact
model is the API contract, and handler-local response structs may not mirror
or remap the same `url`, `downloadURL`, `scriptFileName`, command, expiry, and
token metadata in parallel.
That same setup-completion contract must also preserve the canonical agent-connect
URL boundary: first-session install commands must prefer the backend-governed
security status `agentUrl` and only fall back to browser origin when no
canonical agent endpoint exists, while still allowing a local override for
bootstrap cases where the operator needs a different agent-to-Pulse address.
That same shared first-session install contract also applies to Windows
transport: `SetupCompletionPanel` must expose a governed PowerShell install command and
route it through the shared lifecycle helper, so `PULSE_URL`, optional
`PULSE_TOKEN`, insecure/self-signed TLS handling, and `PULSE_CACERT` stay
identical to the Windows install payload contract already enforced in
`InfrastructureInstallerSection.tsx` and `useInfrastructureOperationsState.tsx`.
That same first-session install boundary must also preserve the shared
optional-auth command contract: the Unix install builder must support omitted
`--token` transport, and `SetupCompletionPanel` may only omit that argument after an
explicit "without token" confirmation when auth is optional, while preserving
the generated token path by default so onboarding does not drift from the
governed settings behavior. After that explicit tokenless confirmation,
repeated wizard copy actions must keep emitting tokenless payloads instead of
silently rotating back to `PULSE_TOKEN` or `--token` transport on the next
rendered command. The same rule applies to wizard-owned background token
rotation: agent-connection polling may not regenerate a token or restore
token-auth payloads while explicit tokenless onboarding is still the active
contract.
That same first-session token contract must also stay coherent across the
setup-completion credential surfaces: once `SetupCompletionPanel` rotates the active install
token, the displayed credential token and downloaded credentials payload must
emit that same current token instead of exporting the stale bootstrap token
while the copied install command already uses a different one. At the same
time, the stable bootstrap admin API token must remain separately visible and
copyable; the setup wizard may not replace the admin credential with the
rotating install token and call that payload contract complete. That same
exported credentials payload must also carry the current agent-install URL and
matching install command contract for both Unix and Windows transport,
including any operator override, instead of serializing only browser-local
login context or Unix-only onboarding while the live setup-completion install
surface has already switched to a different governed endpoint. When explicit
tokenless optional-auth mode is active, the same payload and drawer contract
must report tokenless install mode instead of serializing a misleading current
install token that is no longer part of the active command transport, and the
operator guidance text on the install surface must stop claiming automatic
token rotation after each copy while tokenless transport is active.
That same insecure-TLS contract also applies to installer-owned HTTP traffic:
when `PULSE_INSECURE_SKIP_VERIFY` is set, `scripts/install.ps1` must use the
same relaxed certificate policy for the governed binary download and uninstall
API callback requests instead of preserving `--insecure` only for the later
agent runtime.
That same shared infrastructure install boundary must also preserve
platform-canonical uninstall command payloads: copied utility actions for
Windows agents must emit the PowerShell uninstall transport, and uninstall
payloads must only carry real API token secrets rather than token record IDs
when server-side deregistration is requested.
That same uninstall payload rule now also applies to copied Unix shell flows:
`frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`
must never serialize a token record ID into the governed `--token` argument
when building uninstall transport, because the backend runtime only accepts
the raw token secret or no token at all.
The same shared uninstall transport must preserve `PULSE_URL` for token-optional
Windows flows, because `install.ps1` reads its canonical server endpoint from
that environment variable when composing the governed uninstall request.
That same copied uninstall boundary must also preserve the selected agent's
canonical identity when inventory already has it: shell uninstall payloads must
carry `--agent-id`, and PowerShell uninstall payloads must carry
`PULSE_AGENT_ID`, so deregistration targets the intended governed agent record
instead of depending on local fallback files or hostname lookup.
The same identity-preservation contract applies to copied upgrade transport:
shell upgrade payloads must carry `--agent-id` and `--hostname`, and
PowerShell upgrade payloads must carry `PULSE_AGENT_ID` and `PULSE_HOSTNAME`,
so upgrade reruns stay bound to the selected governed inventory record.
That selected-agent upgrade rerun is distinct from the stale-agent update
commands opened from platform notices. The `agentUpdates` Infrastructure route
may carry scoped agent connection IDs in the URL so Settings can select the
affected inventory rows, but Unix-like copied stale-agent update commands must
not serialize API tokens, agent IDs, or hostnames into the shell payload. They
must call `scripts/install.sh --update` and let the installer recover the
saved connection state on the target host. Windows stale-agent update commands
continue to use the existing token-gated PowerShell install command shape
until `install.ps1` has its own saved-state update mode.
That same Unix transport boundary must also preserve shell-safe argument
encoding: copied shell uninstall and upgrade payloads must quote canonical URL,
token, agent ID, and hostname arguments so governed lifecycle commands do not
break or reinterpret inventory values with shell-significant characters.
The same Windows transport boundary must also preserve PowerShell-safe argument
encoding: copied PowerShell uninstall and upgrade payloads must escape
canonical URL, token, agent ID, and hostname values before they enter env
assignments or `irm` command text, and the copied Windows upgrade payload must
quote the resolved script URL so canonical URLs containing spaces remain a
valid PowerShell transport. The same Windows uninstall payload must quote its
resolved script URL too; escaping `PULSE_URL` into env assignments is not
sufficient if the later `install.ps1` invocation can still be split by
PowerShell parsing.
That same install-command boundary must use the identical escaping rules:
copied shell install payloads must quote canonical URL/token arguments, and
copied PowerShell install payloads must escape canonical URL/token values
before they enter env assignments or `irm` transport. The same interactive
Windows install snippet must also export `PULSE_URL` explicitly when copying a
selected canonical agent address, not just the fully qualified `install.ps1`
download URL.
That same shared install payload contract must also normalize trailing slashes
on canonical Pulse URLs before composing installer asset paths, so copied shell
and PowerShell install transport cannot drift onto `//install.sh` or
`//install.ps1` when operators paste a base URL that already ends with `/`.
When a governed token is already selected, that same interactive Windows
install payload must carry `PULSE_TOKEN` too; the copied command may not discard
the chosen credential and regress to a second manual prompt while other
install/uninstall/upgrade payloads stay token-bound.
When no real token has been selected yet, that same interactive Windows payload
must not serialize a placeholder token into `PULSE_TOKEN`; the contract remains
prompt-driven until a governed credential actually exists.
That optional-auth install contract must also remain bidirectional: when Pulse
allows tokenless transport, the settings surface may omit `PULSE_TOKEN` after a
real "without token" confirmation, but it must still preserve a real generated
token if the operator chooses one instead of collapsing optional auth into a
tokenless-only command builder.
That same optional-auth payload rule now also covers backend-generated Proxmox
install responses: when auth is not configured, the canonical
agent-install-command API must omit `token` and `--token` from its payload
instead of implicitly persisting a new API token record and mutating the
server's auth-configured state just to render a backend-driven install
command.
The same uninstall contract applies to hostname fallback identity: shell
payloads must carry `--hostname`, PowerShell payloads must carry
`PULSE_HOSTNAME`, and the uninstall scripts must prefer that explicit hostname
when performing governed `/api/agents/agent/lookup` fallback. That lookup must
fail closed on ambiguous hostname matches: installer-driven recovery may only
resolve a hostname when the match is unique, and display-name or short-hostname
fallbacks must return not found rather than picking an arbitrary agent.
That lookup fallback transport must be canonicalized on both installer paths:
shell and PowerShell uninstall flows must percent-encode the selected hostname
before issuing `/api/agents/agent/lookup`, so API-owned identity recovery does
not depend on raw query interpolation.
The same shell uninstall contract also applies to persisted connection state:
when `scripts/install.sh` receives explicit `--agent-id` or `--hostname`, it
must store those values alongside URL/token in `connection.env` and recover
them before invoking governed uninstall fallback.
The same persisted-identity contract applies to `scripts/install.ps1`: Windows
install and upgrade must store URL, token, agent ID, and hostname continuity in
installer-owned state and reload those values during governed uninstall before
using local fallback files or hostname discovery.
That ProgramData continuity state is scoped to the live installation only:
after governed uninstall succeeds, `scripts/install.ps1` must remove the saved
state so stale agent identity or transport metadata cannot leak into later
removal or reinstall flows.
The same persisted-state contract applies to self-signed transport continuity:
canonical installer-owned uninstall state must retain insecure TLS intent and
reload it during governed offline uninstall, so self-signed Pulse instances do
not lose deregistration reachability after the original clipboard command.
That same persisted shell uninstall state must retain `--cacert` continuity:
`scripts/install.sh` must store and recover the custom CA bundle path from
`connection.env` so governed lookup and uninstall calls continue to trust the
intended Pulse certificate chain offline.
That shell `connection.env` recovery contract is keyed to partial uninstall
context, not only an entirely missing URL/token pair: if any governed uninstall
identity or transport field is absent on the command line, the script must
reload the missing persisted continuity before using API-owned lookup fallback.
Those register/install control surfaces now also carry a canonical host
identity continuity contract: `/api/auto-register` and token reuse must treat
hostname-form and IP-form URLs for the same node as one API-owned identity so
reruns do not fork duplicate runtime records or shadow token payloads.
That canonical `/api/auto-register` payload must also preserve token-action
truth: canonical completion now requires caller-supplied `tokenId` and
`tokenValue`, and the response must stay on the direct-use
`action="use_token"` contract as the only supported completion path.
That same contract must be enforced by first-hop callers too: install and
runtime-side Unified Agent registration clients may not treat a bare 2xx response or a loose
`status` field as success; they must validate the canonical `status`,
`action`, and token/identity response shape.
That same canonical `/api/auto-register` contract must also accept caller-supplied
Proxmox token completion directly on that contract: when a runtime-side Unified Agent or
generated flow already created the canonical token locally, the request may
carry `tokenId` and `tokenValue`, and the response must stay on the direct-use
`action="use_token"` contract as the only supported completion path.
That same runtime transport contract also governs the agent-ingest boundary in
`internal/api/agent_ingest.go` and `internal/api/router*.go`: the primary
request/response surface is the Pulse Unified Agent route family, while
`/api/agents/host/*` stays a compatibility alias and must not leak back into
handler naming, router-owned state, or proof labels as if it were a second
product-facing API surface.
That confirmation marker must survive the legacy setup-script transport too:
script-generated `/api/auto-register` payloads must send `source="script"`,
and canonical callers must send that source explicitly, so later canonical
reruns can distinguish real confirmed credentials from agent-created tokens.
That same `/api/auto-register` request contract must also reject non-canonical
source values outright: only `source="agent"` and `source="script"` are valid,
so the backend does not preserve arbitrary caller labels as accidental API
surface.
That same `/api/auto-register` request contract must also reject non-canonical
node types outright: only `type="pve"` and `type="pbs"` are valid, so the
backend does not complete unsupported runtime labels as fake successful
registrations.
That same `/api/auto-register` request contract must also reject non-canonical
token identities outright: `tokenId` must be a Pulse-managed canonical
identifier in the form `pulse-monitor@{pve|pbs}!pulse-<canonical-scope-slug>`
matching the requested node type, so the backend does not preserve arbitrary,
cross-type, or non-Pulse-managed token IDs as accidental API surface.
That same caller-supplied token contract must also stay deterministic across
the live registration clients: installer, setup-script, and runtime-side Unified Agent Proxmox
flows must converge on the same Pulse-managed `pulse-<canonical-scope-slug>`
token name for the same Pulse endpoint instead of serializing caller-local
timestamp variants into the canonical `/api/auto-register` payload.
That same deterministic token-name contract also governs backend turnkey
credential setup: the password-based PBS add-node flow and generated
setup-script payloads must derive Pulse-managed token names from the canonical
Pulse endpoint itself rather than request-local `Host` fallbacks, so loopback
or proxy-facing admin requests cannot fork the token scope for the same Pulse
instance.
That same generated setup-script payload must now also opt into the canonical
registration contract explicitly: locally created Proxmox token completions
must send `tokenId` and `tokenValue` as the canonical request shape.
That same request contract must also accept one-time setup-token auth through
`authToken` only, so `/api/auto-register` does not keep a duplicate
`setupCode` payload alias alongside the canonical field.
That same shared discovery transport surface must also keep structured error
ownership in the runtime model: `pkg/discovery` and `internal/discovery` own
`structured_errors`, while `internal/api/config_discovery_handlers.go`,
`internal/api/config_setup_handlers.go`, and `internal/api/config_node_handlers.go`
may derive the deprecated `errors` string list only as a compatibility field
at the API and WebSocket boundary.
That same WebSocket state boundary must also stay tenant-aware by construction:
`internal/websocket` may not keep a separate default-org state getter beside the
tenant-aware state path, and default-org snapshots must flow through the same
`org_id="default"` contract used for non-default organizations.
That same canonical auth contract must also keep its runtime and user-facing
terminology on setup tokens: active `/api/auto-register` auth failures and the
owning handler/proof names may not drift back to setup-code wording after the
payload contract has been canonicalized.
That same first-session security boundary also governs bootstrap-token
persistence and retrieval: the one-time setup secret may remain recoverable
through the supported `pulse bootstrap-token` command, but `.bootstrap_token`
may not remain a raw plaintext secret file on disk. Canonical runtime
persistence must encrypt that token at rest and rewrite any legacy plaintext
bootstrap-token file immediately into the encrypted canonical format on load.
That same first-session contract also owns the dev/test reset response used by
managed-backend proof: `/api/security/dev/reset-first-run` may exist only for
development verification, must require authenticated `settings:write`, must
clear persisted auth state through the shared auth-env and token-persistence
helpers, and must return the regenerated `bootstrapToken` together with the
canonical `bootstrapTokenPath` needed to re-enter first-run deterministically.
That same setup-token-only contract must also keep missing-token failures
specific: `/api/auto-register` may not answer a missing `authToken` request
with a generic authentication error after the route has been narrowed to the
setup-token flow.
That same canonical request contract must also keep field-validation failures
specific: mismatched `tokenId`/`tokenValue` input may not collapse into
generic missing-field output, and other missing canonical fields must return
explicit `Missing required canonical auto-register fields: ...` guidance.
That same request/validation contract must stay coherent across both entry
points on the canonical runtime surface: the public `/api/auto-register`
handler and the direct canonical handler path may not drift onto different
messages for the same missing-field or token-pair failures.
That same canonical request contract must also require an explicit
`serverName` field from live callers rather than synthesizing node identity
from `host` inside the backend.
That same canonical backend contract must also keep overlap-continuity runtime
messages on canonical `/api/auto-register` wording: the helper/log surface for
resolved-host matches, DHCP continuity matches, and in-place token updates may
not preserve the deleted "secure auto-register" split.
That same canonical runtime path must keep token-completion validation wording
on the canonical contract too: incomplete `tokenId`/`tokenValue` payloads may
not preserve deleted "secure token completion" wording in live handler
messages.
That same canonical request contract also governs runtime-side Unified Agent-initiated
Proxmox completion: callers must fetch and use a one-time setup token in
`authToken` instead of carrying long-lived admin authentication directly on
`/api/auto-register`.
That same canonical caller-supplied completion request shape also governs `scripts/install.sh`:
installer-owned Proxmox auto-registration must submit local token creation
results with `tokenId` and `tokenValue` on the canonical `/api/auto-register` contract instead
of emitting any alternate payload shape.
The unified-agent uninstall command contract must also fail closed on
token-required Pulse instances: copied shell and PowerShell uninstall payloads
must use the same resolved token source as install and upgrade, so required
auth cannot silently collapse into tokenless deregistration transport.
Agent profile assignment payloads now also fail closed on missing profiles:
`POST /api/admin/profiles/assignments` must reject unknown `profile_id`
references with the canonical not-found response instead of writing orphan
assignment rows that no governed UI can represent.
That same not-found assignment contract must propagate through the shared
frontend client path: `frontend-modern/src/api/agentProfiles.ts` must surface
the canonical missing-profile message for 404 assignment responses, and the
settings profile surfaces in `AgentProfilesPanel.tsx` and `InfrastructureInstallerSection.tsx`
must treat that message as a resync trigger so stale profile options do not
survive after the backend has already rejected them.
That same shared response contract must also fail closed on malformed list
payloads: the profile-management client may not treat non-array profile or
assignment responses as empty collections, and `AgentProfilesPanel.tsx` /
`InfrastructureInstallerSection.tsx` must surface the resulting load failure instead of
flattening it into a fake zero-profile state.
That same shared response contract must also fail closed on malformed
profile-object, suggestion, schema, and validation payloads: the
profile-management client may not accept partial profile objects, malformed
schema definitions, or malformed validation/suggestion bodies as successful
contract responses, and the profile editor plus suggestion modal must surface
those canonical response failures instead of collapsing them into generic
save/delete/schema/validation fallback messaging.
The canonical Proxmox auto-register contract must also preserve legacy DHCP
continuity semantics: when `/api/auto-register` receives the same
canonical node name together with the deterministic Pulse-managed token ID for
that node, it must update the existing PVE or PBS entry in place even if the
host IP has changed, rather than duplicating the node under a second endpoint.
That same `/api/auto-register` payload contract must now also accept ordered
`candidateHosts` from runtime-side Proxmox callers and treat `host` as the
preferred candidate, not an untouchable answer. The backend must normalize the
candidate list, ignore invalid alternates, and persist the first candidate it
can actually reach for TLS fingerprint capture from Pulse's own network view so
registration payloads do not lock in an endpoint the server cannot later poll.
That same response contract must echo the stored reachable candidate back in
the canonical `host` field, not the caller's rejected first preference, so
runtime-side Unified Agent confirmation and later setup/install surfaces stay
aligned on the actual persisted polling endpoint.
The unified-agent install endpoints now also carry an exact-release fallback
contract: when `/install.sh` or `/install.ps1` cannot be served locally, the
backend must proxy the install script asset from the exact GitHub release that
matches `serverVersion` and must fail closed for dev or unreleased builds
rather than serving branch-tip installer logic.
That same response contract now also owns signed release-asset headers:
published agent-binary and installer downloads served through
`internal/api/unified_agent.go` must surface `X-Checksum-Sha256`,
`X-Signature-Ed25519`, and the base64-encoded detached `X-Signature-SSHSIG`,
and release-tagged local assets must not bypass that header contract just
because the binary or script is present on disk.
That same transport rule is now explicit about prerelease classes too: only
stable tags and explicit RC prerelease tags without build metadata qualify as
published install-script release assets. Working-line dev prereleases such as
`v6.0.0-dev`, git-described builds with `+git...` metadata, and other
non-published prerelease identifiers must fail closed on that shared
`internal/api/unified_agent.go` boundary instead of generating fake GitHub
release URLs from a local runtime version string.
The `/api/updates/plan` contract must also fail closed without becoming a
transport error on supported non-auto-update deployments: `manual`,
`development`, and `source` runtimes must return an explicit manual update
plan payload instead of `404 No updater for deployment type`, so first-session
and settings surfaces do not treat valid deployment modes as broken update
transport.
The same update-plan contract now carries an optional `readiness` verdict.
Backend handlers own the `ready` / `attention` / `blocked` status vocabulary
and per-check payload shape, while frontend clients must preserve that payload
unchanged so settings surfaces can disable automatic install on blocked checks
without inventing a parallel migration state model. The same payload must also
carry the v5-to-v6 first-hop transport warning when legacy agents are present,
because the first automatic hop runs through the already-installed v5 updater
before v6 signature and downloaded-binary self-test protections apply. The UI
disablement is only presentation: backend apply handlers must still enforce
`blocked` readiness server-side.
Those same install-command payloads now also carry a non-TLS continuity
contract: when Pulse returns a plain `http://` base URL for a generated agent
install command, the command must include `--insecure` so the installed agent
keeps its update path alive on lab or self-hosted targets instead of silently
skipping updater checks after the first install.
The same plain-HTTP continuity rule applies to governed frontend-generated
host install transport too: shared Unix install command builders must append
`--insecure` for `http://` Pulse URLs so setup-completion copies cannot drift from
the lifecycle contract already enforced in the unified settings surface.
That same frontend install-command contract must also fail closed on blank
local overrides: whitespace-only custom Pulse endpoint input in
`InfrastructureInstallerSection.tsx` or `SetupCompletionPanel.tsx` may not override the canonical
backend-governed endpoint, and the shared install-command helper must reject
blank base URLs instead of composing installer script paths from an empty
transport root.
That same install-command payload contract also covers backend-generated
Proxmox install responses in `internal/api/agent_install_command_shared.go`:
the `/api/agent-install-command` payload and hosted tenant Proxmox install
payload must emit the same root-or-sudo Unix wrapper contract as the governed
frontend builder, rather than exposing a stale raw `| bash -s --` transport
shape through the API surface.
That same rule applies to the unified settings shell lifecycle copies:
frontend-generated Unix install and upgrade commands must append `--insecure`
for `http://` Pulse URLs automatically, while only the explicit insecure-TLS
toggle may widen curl transport itself to `-k`.
That same unified settings install boundary must also preserve preview/copy
parity: the rendered Linux/macOS/BSD and Windows install snippets in
`InfrastructureInstallerSection.tsx` must already reflect the active token contract, custom-CA
transport, insecure/plain-HTTP behavior, install-profile env/flags, and
command-execution mode, rather than showing a stale base command that is only
rewritten at copy time.
For Unix-family frontend-generated install snippets, that same contract also
owns the preflight and credential transport shape: the rendered command must
download the shared installer into an ephemeral directory, run
`install.sh --preflight-only` before root/sudo escalation, verify that
`/download/pulse-agent?arch=...` is reachable with checksum metadata, and
pass selected tokens to the installer through an ephemeral `--token-file`
instead of a raw `--token` service argument.
The loopback-originated install and setup payloads now also preserve the full
configured `PublicURL` when that URL is the canonical external route, instead
of rewriting only the host and inheriting an `http://` request-local scheme
that would drift the generated command away from the governed public endpoint.
The canonical frontend client contract for Proxmox setup transport now also
applies to `/api/setup-script-url` and `/api/setup-script`: governed settings
surfaces must request quick-setup commands and manual setup-script downloads
through shared `frontend-modern/src/api/nodes.ts` helpers for both `type:"pve"`
and `type:"pbs"`, preserving the runtime-owned bootstrap artifact metadata
instead of open-coding one node type onto raw fetch branches.
That same `/api/setup-script-url` response contract must now also preserve the
canonical bootstrap identity explicitly through returned `type` and normalized
`host`, and the handler must reject missing or unsupported `type`/`host`
input instead of minting open-ended setup tokens with caller-local host
formatting.
That same setup-script-url boundary must keep a strict request shape too: the
handler accepts one canonical JSON object only, and unknown fields or trailing
JSON must fail closed as invalid request shape instead of being ignored as
forward-compatible extras.
That same bootstrap request boundary must also keep `backupPerms` truthful:
the flag is part of the canonical PVE setup contract only, so `/api/setup-script`
and `/api/setup-script-url` must reject it for `type:"pbs"` instead of
silently accepting a transport-level no-op.
That same setup bootstrap contract also keeps host identity explicit across
both routes: `/api/setup-script` and `/api/setup-script-url` must reject
missing `host` input instead of issuing placeholder-host artifacts that only
fail later during execution.
That same request boundary must also keep canonical type and host handling
aligned across both setup routes: `/api/setup-script` may not treat unknown
`type` values as implicit PBS requests, and it must normalize the supplied
host before rendering script text so returned artifacts and rerun URLs preserve
the same canonical node identity as `/api/setup-script-url`.
That same setup bootstrap contract also keeps Pulse identity explicit across
both routes: `/api/setup-script` may not derive `pulse_url` from the request
origin once `/api/setup-script-url` is already returning canonical Pulse URL
metadata, and missing `pulse_url` input must fail closed instead of silently
forking the bootstrap surface onto request-local origin state.
That same canonical bootstrap response shape must also stay enforced by the
shared frontend setup client in `frontend-modern/src/api/nodes.ts`, so
settings-owned quick-setup flows fail closed on malformed `type`, `host`,
`url`, `downloadURL`, `command`, `setupToken`, `tokenHint`, or `expires`
fields instead of passing raw backend JSON deeper into lane-local UI state. That shared client
must validate the returned `setupToken` but may not expose or retain it once
the operator-facing surface only needs the runtime-owned bootstrap artifact
plus masked `tokenHint`.
That frontend bootstrap consumer must also treat `expires` as a live-expiry
field, not merely a positive number, so expired setup-script-url responses are
rejected before quick-setup UI state or copy actions trust the returned setup
token.
That same settings quick-setup surface must consume the canonicalized response
directly:
`frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx`
inside
`frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`
must copy the governed token-bearing `commandWithEnv` field but render
`commandWithoutEnv` as the visible preview, using the guaranteed `expires`
value without reintroducing module-local nullable fallbacks. The same shared
surface must
also treat `setupToken` as bootstrap transport data and `tokenHint` as the
operator-facing display field, so the UI does not re-expose the full one-time
token once the copied/downloaded artifact already carries it. That preview
secrecy rule must stay symmetric across both supported Proxmox types, so the
PBS quick-setup branch may not preserve the token-bearing preview after the
PVE branch has moved to the governed `commandWithoutEnv` display contract.
That same quick-setup guidance must also stay truthful after the preview is
masked: copy-success messaging may not tell the operator to paste a token
"shown below" once only `tokenHint` remains visible, and stale raw-token
cleanup paths may not survive in one Proxmox branch after the shared UI state
has moved to hint-only handling.
That same shared settings consumer must keep command-driven setup and manual
credential submission distinct. When a new PVE/PBS setup is in API Inventory or
Host Telemetry Agent mode, the settings UI must not render Token ID/Value
fields, Test Connection, or Add Node controls; those controls are only valid for
Manual Token Setup or existing-node edit flows.
That same shared frontend setup surface must also trim and validate the
canonical `host` input before invoking `/api/setup-script` downloads, and the
shared `frontend-modern/src/api/nodes.ts` helper must reject empty `host` or
`pulseUrl` inputs instead of serializing whitespace-corrupted query params.
That same `/api/setup-script` payload contract must also stay explicit at the
artifact boundary: successful responses are shell-script downloads with
canonical `text/x-shellscript` content type plus an attachment filename, and
the shared `frontend-modern/src/api/nodes.ts` client must reject malformed
download headers instead of flattening script delivery into an untyped text
blob.
That same setup bootstrap contract must also keep manual download
non-interactive without depending on a separately rendered secret: the
setup-script-url payload must return a token-bearing `downloadURL`, and the
shared frontend client must fetch setup scripts through that field instead of
reusing the plain script `url` that omits the setup token.
That same shared frontend setup surface must also treat
`/api/setup-script-url` as the canonical bootstrap artifact source for the
current host/type/mode: quick-setup copy and manual script download must reuse
the returned `url`, `downloadURL`, `scriptFileName`, `commandWithEnv`,
`tokenHint`, and `expires` until that artifact expires or the operator changes
the endpoint, instead of rebuilding a second download request from lane-local
form state or retaining the raw setup token inside frontend cache state.
For existing Proxmox API sources, that same setup surface must present the
rendered setup script's Audit/Repair action as the non-destructive repair path.
The frontend may tell operators to rerun the same canonical command and choose
Audit/Repair, but it must not imply the current API token is rotated during
repair. Install/Configure remains the explicit rotation and re-registration
path when the token value itself needs to be replaced.
That same setup surface must also keep the Docker-in-LXC guidance with the PVE
Host Telemetry Agent mode. API Inventory copy may point operators to Host
Telemetry Agent when Docker runs inside LXCs, but it must not claim that API
setup alone can run the host-side Docker collector.
That same bootstrap artifact contract must also stay coherent in public-facing
guidance: `docs/API.md` and operator setup guides may not describe
`/api/setup-script-url` as if it only returned a token plus bare URL, and they
may not publish stale `curl -sSL ... | bash` setup examples after the runtime
and settings surfaces have standardized on the returned canonical `command*`
fields.
That same setup-script-url payload contract must also return the canonical
setup-script filename as `scriptFileName`, and the shared settings/bootstrap
consumer may not hardcode separate script names for PVE or PBS once the
runtime-owned filename is available.
That same setup-script-url payload must remain a coherent bootstrap artifact
envelope for all live consumers, not only the frontend: `url`,
`downloadURL`, `scriptFileName`, `command`, `commandWithEnv`,
`commandWithoutEnv`, and masked `tokenHint` are part of the canonical response
shape, and runtime-side Unified Agent/installer consumers must fail closed when those fields
are missing or mismatched instead of silently treating the response as
setup-token-only.
That same consumer contract must also treat `expires` as a live-expiry field,
not merely a populated one: installer and runtime-side Unified Agent callers must reject
bootstrap responses whose returned expiry timestamp is already in the past.
That same setup-script-url auth boundary must stay explicit too: returned
`setupToken` values bootstrap `/api/setup-script` and `/api/auto-register`,
but they do not authenticate the `/api/setup-script-url` request itself once
Pulse auth is configured.
That same setup-script-url payload contract now also fixes the shell transport
it returns: the `command`, `commandWithEnv`, and `commandWithoutEnv` fields
must use shell-quoted `curl -fsSL` fetches assembled through a shared backend
helper rather than a handler-local `curl -sSL` pipeline.
Those returned setup-script command fields must also preserve the governed
root-or-sudo execution contract, including carrying `PULSE_SETUP_TOKEN`
through the sudo path when present instead of assuming direct-root execution.
That same setup-script contract now also covers the generated script text:
operator guidance embedded in `/api/setup-script` responses must keep the same
fail-fast `curl -fsSL` fetch wording for retry and missing-host examples
instead of returning stale `curl -sSL` transport in the script payload.
That embedded guidance must also advertise the same root-or-sudo execution
shape as the API-returned quick-setup command instead of drifting onto a
direct-root-only `| bash` retry path inside the script payload.
That same script-payload guidance must preserve `PULSE_SETUP_TOKEN` across
those retry examples too, so the generated script text does not drop the
non-interactive setup-token contract even when it preserves the shell wrapper.
That same generated-script payload must also hydrate `PULSE_SETUP_TOKEN` from
an embedded setup token before those rerun examples are shown, so canonical
`setup_token`-issued scripts keep the same non-interactive contract on the
next hop instead of silently reverting to a prompt.
That same `/api/setup-script` boundary must keep one token name too: embedded
bootstrap uses only the `setup_token` query, and the rendered setup script body
uses only `PULSE_SETUP_TOKEN` rather than keeping `AUTH_TOKEN` or
`SETUP_AUTH_TOKEN` compatibility aliases alive.
That same generated-script payload must also remove discovered legacy tokens
from the concrete `pve` and `pam` token lists it already enumerated, rather
than iterating an undefined shell variable and silently turning operator-chosen
cleanup into a no-op.
That same generated PVE setup-script payload must also preserve Proxmox-managed
`/root/.ssh/authorized_keys` symlinks when it installs or removes Pulse
temperature-monitoring SSH keys: the rendered shell must resolve the symlink
target before filtering Pulse-managed `# pulse-` entries, use the resolved path
for both install and uninstall edits, and keep this behavior pinned in
`internal/api/contract_test.go`.
That same rendered PVE setup-script payload must also bind the temperature SSH
key to the Pulse-owned `/usr/local/sbin/pulse-sensors` wrapper rather than to
raw `sensors -j`. The wrapper is the setup-script API contract for legacy SSH
temperature collection: it must emit a bounded JSON object with `sensors` and
`smart` members, install or verify `smartmontools` for SATA/SAS/HDD disk
temperatures, and keep `sensors -j` only as a compatibility fallback inside
the wrapper/runtime collector path.
That same generated-script payload must also preserve the canonical encoded
rerun URL contract: embedded `SETUP_SCRIPT_URL` values must carry the exact
selected `host`, `pulse_url`, and `backup_perms` query state instead of
reconstructing a raw query string inside the shell.
That same off-host branch may not advertise a second manual `pveum` token
creation contract either; when the runtime lacks Proxmox host tooling, the
payload must direct operators back to rerun on the host through the canonical
generated command instead of inventing a separate Pulse Settings token-entry
workflow.
That same script payload must also preserve canonical privilege-error wording
for direct execution: the generated runtime may not regress to the stale
"Please run this script as root" string and must instead use the same root
requirement language as the governed retry examples.
That same manual-add payload must also preserve one canonical token placeholder
string when the script cannot echo the secret again from process state, rather
than drifting across neighboring branches with lane-local variants like
"[See above]" or "Check the output above...".
That same payload must also preserve one canonical success-message contract
across generated PVE and PBS scripts, rather than returning node-type-specific
phrasing for the same successful auto-register result.
That same setup-script payload must also discover legacy cleanup candidates
through the canonical Pulse-managed token prefix for the active Pulse URL,
while still matching legacy timestamp-suffixed variants, instead of rebuilding
an IP-derived regex that can drift from `buildPulseMonitorTokenName`.
That same cleanup-discovery contract applies to both generated PVE and PBS
setup-script payloads; node type may not fork onto different legacy token-name
matching rules for the same Pulse-managed token surface.
That same payload must also use exact token-name matching for rerun rotation
detection, rather than broad substring checks over token-list output, so the
canonical managed token contract does not collide with unrelated partial-name
matches.
That same PVE setup-script payload must provide an Audit/Repair menu action and
non-interactive `PULSE_SETUP_ACTION` selector that audit and reapply
Pulse-managed setup without rotating the API token. The repair path may create
the missing `pulse-monitor@pve` user, report the current token's Expire value,
inventory matching Pulse-managed tokens under the `pve` and legacy `pam`
realms, and reapply safe user/token ACLs, including the optional storage grant
when requested. If the current token is absent or Pulse still receives 401
after repair, the generated script must direct the operator to
Install/Configure for explicit token rotation and re-registration instead of
silently minting a replacement token from repair mode.
That same payload must also keep PBS token-copy guidance truthful: the
one-time token banner may only be emitted from the successful token-create
branch, not before the creation result is known.
That same payload must also keep PBS auto-register attempt guidance truthful:
the generated script may only print its attempt banner on the branch that is
actually about to send the registration request, not before token-unavailable
or missing-auth skip handling.
That same payload must also fail closed when token creation output does not
yield a usable token value: the generated script may not continue into prompt
or request assembly with an empty token secret, and must instead stop on the
canonical token-value-unavailable branch before any registration POST is built.
That same setup-script payload must also fail closed on auto-register success
parsing: the generated script may not treat any bare `success` substring as a
successful response, and must instead require an explicit `success:true`
signal before claiming registration succeeded.
That same payload contract must also fail closed on auto-register transport:
the generated script must use fail-fast `curl -fsS` request transport and only
evaluate the response payload after a successful curl exit status, rather than
parsing ambiguous stderr or HTTP-failure output as a valid registration body.
For PBS setup scripts, the generated auto-register POST URL must be the
canonical Pulse base URL plus `/api/auto-register`. The script may not append
that path to the setup-script download artifact URL, because the artifact URL is
only for fetching the script and is not the API root.
That same setup-script payload must also preserve the canonical auth guidance:
authentication failures in the generated script text must reference the active
Pulse setup-token flow, not stale API-token setup instructions, because the
payload now authenticates auto-register through one-time setup tokens.
That same auth-failure payload must also stay truthful after a request attempt:
once the generated script has already entered the registration-request path,
it may not fall back to a missing-token explanation and must instead report
that the provided setup token was invalid or expired, directing the operator
to fetch a fresh setup token from Pulse Settings → Nodes and rerun. The final
completion/footer path must honor that same auth-failure state instead of
reopening manual completion with the emitted token details.
That same payload must also preserve truthful completion messaging: generated
setup-script text may only announce successful Pulse registration when the
payload's auto-register branch succeeded, and must otherwise describe the
result as manual follow-up using the emitted token details.
That same manual-follow-up payload may not advertise a stale `PULSE_REG_TOKEN`
rerun contract: when auto-register falls back to manual completion, the script
text must direct the operator to Pulse Settings → Nodes with the emitted token
details rather than inventing a second registration-token flow.
That same manual-follow-up payload must also keep its failure-summary text on
that same canonical completion path: the generated script may not fall back to
vague "manual configuration may be needed" wording when it already knows the
operator should finish registration through Pulse Settings → Nodes with the
emitted token details.
That same immediate failure path may not fork into a separate numbered manual
setup list either; it must point directly at the same token-details-below
Settings → Nodes completion contract used by the final manual footer, including
the branch where the registration POST itself fails before a response payload
can be parsed.
That same manual-follow-up payload must also preserve the canonical host value
already carried by the script payload, instead of reverting to a placeholder
host string in the rendered manual-add instructions.
That same host-continuity contract also applies to generated PBS scripts: the
manual-add footer must preserve the canonical `host` payload value instead of
replacing it with a runtime-discovered local IP that may not match the API
contract the caller requested.
That same PBS payload contract must also bind the canonical `host` before any
setup-token gating that can skip auto-registration, so manual fallback output
cannot lose the host URL when the operator does not provide a setup token.
That same host binding must also precede token-creation failure fallback, so
the rendered manual footer still carries the canonical `host` payload even
when the script fails before any auto-register request can be assembled.
If the caller never supplied a canonical `host` at all, the rendered script
must fail closed instead of surfacing placeholder host values as manual
registration targets; it must direct the caller to regenerate the setup script
with a valid host URL.
That same payload must also preserve token-creation failure truth: when
Proxmox token minting fails, the rendered script may not emit placeholder token
details or report token setup completed. It must keep the host binding, skip
auto-register assembly, and tell the caller to rerun after the token-creation
error is fixed.
That same payload must also preserve token-extraction failure truth: if the
returned token output does not yield a usable token secret, the script may not
advertise manual registration as a fallback path from that broken payload and
must instead direct the caller to rerun after the token output issue is fixed.
Rendered completion and manual-detail payload branches must treat only an
extractable token secret as ready; token-create success alone is not enough.
That same rendered PBS payload must also distinguish skipped auto-register
states from attempted request failures, so missing setup-token input or missing
usable token secret cannot surface the generic request-failed-before-success
banner.
That same payload must also preserve canonical manual-completion phrasing
across generated PVE and PBS scripts: both must use the Settings → Nodes
manual-add language instead of diverging onto node-type-specific fallback
headings that imply different completion paths.
That same generated payload may not shorten the earlier auto-register failure
branch back to plain "Pulse Settings" wording either; both the immediate
failure guidance and the final manual footer must preserve the same Settings →
Nodes completion destination.
`/api/charts/workloads-summary` now also has a canonical hot-path invariant:
aggregate workload charts must preserve stable guest counts while batching
store-backed metric reads across workload types, with no payload shape change.
That endpoint now also carries an explicit API p95 budget under the same
store-backed mixed-workload fixture used to verify the batched hot path.
That same summary-chart contract now also owns synthetic mock fallback
identity. When `internal/api/router.go` needs to synthesize summary history
for workloads, infrastructure, or storage cards, it must key those series by
canonical `resourceType`, `resourceID`, and `metricType` instead of ad hoc
seed-prefix bounds, so all time ranges and runtime mock samples stay on one
governed timeline.
Frontend AI API clients now also normalize `402 Payment Required` responses for
optional paywalled collections into explicit empty states, so Pulse Pro gating
does not become a transport error path during page bootstrap.
That frontend status handling must now route through the shared
`frontend-modern/src/api/responseUtils.ts` status helpers rather than through
message-text heuristics in individual API modules.
Optional not-found response handling in frontend API clients must now also use
those shared response-status helpers rather than open-coded `response.status`
branches in each module.
The same rule now applies to no-content and service-unavailable handling in
governed frontend API clients.
Governed frontend API clients must now also route required and safe success
payload parsing through the shared response parsing helpers rather than through
open-coded `response.json()` calls in each module.
The same rule now applies to optional success payload parsing, including lookup
responses that may legitimately return an empty body but must not use ad hoc
`response.text()` plus `JSON.parse(...)` branches in individual modules.
Investigation and AI chat SSE event payload parsing must now also route through
the shared text-to-JSON helper in `frontend-modern/src/api/responseUtils.ts`
rather than through per-module `JSON.parse(...)` stream decoding.
Nullable or legacy collection payloads in governed frontend API clients must
now also route through shared collection-normalization helpers in
`frontend-modern/src/api/responseUtils.ts` rather than through module-local
`|| []`, `?? []`, or `Array.isArray(...)` fallback branches.
That rule now also covers patrol run history responses so malformed or legacy
run collections collapse through the shared helper instead of per-module
fallback lists.
The `/api/ai/patrol/runs` frontend history clients must now also route their
shared fetch plus run-normalization pipeline through one canonical local helper
in `frontend-modern/src/api/patrol.ts` rather than duplicating the same
endpoint-specific stack across each history variant.
That patrol run-history contract now also treats non-positive or malformed
`limit` query values as defaulted input and caps oversized requests to the
backend maximum, rather than letting invalid caller input widen the history
payload unexpectedly.
The frontend Patrol history clients in `frontend-modern/src/api/patrol.ts`
must mirror that normalization before sending the request: invalid and
non-positive caller input collapses back to the client default of `30`, and
oversized requests clamp to the backend maximum of `100`.
Patrol run detail access for selected-history UX must now resolve a canonical
single-run contract at `/api/ai/patrol/runs/{id}` instead of probing bounded
history pages and hoping the target run is still inside a recent window; the
tool-call trace UI must fetch the selected run by ID, with
`?include=tool_calls` carrying the full trace only when explicitly requested.
Frontend investigation rendering for unified Patrol findings must also key off
finding-level investigation metadata, not only `investigation_session_id`:
the investigation detail endpoint is addressed by finding ID, so findings with
canonical `investigation_status`, `investigation_outcome`, or non-zero
`investigation_attempts` must still surface investigation UI even when the
session ID field is absent or blank.
That same Patrol findings UI contract must keep `fix_queued` approval recovery
actions visible even when no live pending approval remains and
`/api/ai/findings/{id}/investigation` resolves to `null` or omits
`proposed_fix`: queued remediation state cannot collapse into a dead badge with
no user action path.
The `/api/ai/chat` finding handoff contract must also include a model-only
`[Finding Briefing]` generated from the unified finding and structured Patrol
investigation record before detailed `[Finding Context]`. The briefing is the
canonical factual handoff frame for Assistant: it carries the finding summary,
resource, priority, current recency facts, bounded evidence and verification
summaries, investigation confidence, latest lifecycle facts, and governed
action artifact metadata without raw command text. It must not include a
Pulse-authored attention reason, operator decision, or recommended next step. Any recorded
action note from a durable investigation record is context, not remediation
guidance. That governed action artifact metadata must come from the same
structured action reference sent to chat execution, including any recovered live
Patrol approval, so visible and model-only handoffs stay consistent. Patrol's
frontend Assistant drawer briefing must use that same factual frame for
visible handoffs from findings, while the downstream chat
service hydrates live resource state, timeline, and action audit context around
that same handoff. Backend chat handling must treat matching frontend Patrol
handoff context as secondary to durable finding context, not a replacement for
server-refreshed approval, resource, action, or requester authority. Frontend Patrol
handoff helpers may consume current pending
approval list payloads only as safe metadata for that visible briefing and any
structured `handoff_actions`: approval ID, status, risk, request/expiry
timestamps, target label, requester identity, action ID, approval policy,
plan expiry, and dry-run summary are allowed. Assessment-level visible
briefings may reuse that same safe metadata for factual action labels and safety notes,
while approval command text remains inside the governed approval/remediation
surface.
Patrol approval-row Assistant handoffs must use the same safe metadata boundary
and set `autonomousMode:false` for the request-local chat handoff; they must not
paste raw approval or action command text into a chat prompt.
Patrol remediation-plan or action-artifact Assistant handoffs must pass only safe
status, risk, description, and command-count posture as non-authoritative context
for the configured LLM to critique; raw plan command and rollback command
payloads remain owned by the governed remediation/action APIs and panels. The
frontend Assistant drawer must not turn those artifacts into visible Patrol step
lists, suggested-prompt chips, or a "plan attached" answer. Evidence,
confidence, approval risk, prerequisites, recurrence, rollback, and verification
belong in model-only context or governed approval surfaces unless the operator
explicitly asks the LLM for them.
Patrol run-history serialization and persistence must also preserve full field
parity across API responses and restart boundaries, including
`pmg_checked`, `rejected_findings`, `triage_flags`, `triage_skipped_llm`, and
explicit empty `finding_ids` or `effective_scope_resource_ids` arrays when a
run represents an empty snapshot or an intentionally empty effective scope.
The same patrol run-history contract now also treats
`effective_scope_resource_ids` as the canonical analyzed-resource scope when
present, including when it is an explicit empty array, and frontend snapshot
selection must treat an explicit empty `finding_ids` array as an empty snapshot
rather than falling back to unrelated current findings; a missing
`finding_ids` field must retain its "no snapshot filter available" meaning
rather than being collapsed into an empty snapshot.
That same frontend run-history path must also preserve and expose
`triage_flags` and `triage_skipped_llm` from canonical patrol run records so
deterministic triage-only runs do not collapse into generic "no analysis"
history entries.
Patrol run-history payloads must also preserve structured runtime failure
context. When a Patrol run records `error_count > 0`, the backend may include
`error_summary` and `error_detail`; persistence, API responses, and
`frontend-modern/src/api/patrol.ts` must preserve those fields so the Patrol UI
can explain provider/model/tool runtime failures without scraping finding
copy or inferring meaning from a generic error status.
Frontend run-history presentation must derive the collapsed operator record
from the canonical run payload rather than from page-local telemetry strings:
runtime errors may read as `Patrol needs attention`, finding counts may read as
found/fixed/open/resolved work, and technical fields such as trigger reason,
tool-call count, tokens, and raw traces remain secondary forensic context.
Patrol status payloads no longer carry quickstart credit state as an ordinary
v6 GA API contract. `quickstart_credits_remaining`,
`quickstart_credits_total`, and `using_quickstart` are retired public fields;
pricing, README, Patrol header copy, and AI settings copy must present
provider/local-model setup and paid operational extras instead of managed
credits, anonymous bootstrap, trial CTAs, account-backed AI activation, or
hosted-chat access. Historical billing-state fields may remain parseable for
old data, but hosted signup, hosted entitlement refresh, and trial-state
construction must not mint new quickstart inventory.
That same Patrol status contract now also carries a canonical `runtime_state`
field, so the frontend can distinguish blocked, running, disabled, active,
and unavailable Patrol runtime states without deriving operator status from
stale health summaries, last-run history, or local blocked-reason heuristics.
The backend status payload must derive that blocked runtime state from current
provider and runtime availability, and it must clear stale managed-credit block
metadata once provider or local-model configuration returns, so the Patrol
status endpoint cannot leave Patrol looking healthy or paused based on an
out-of-date last-run artifact.
`PatrolStatusResponse.trigger_status` also separates configured scoped trigger
source preferences from effective runtime policy. `alert_triggers_enabled` and
`anomaly_triggers_enabled` describe the operator-controlled source gates, while
`event_triggers_blocked`, `event_triggers_blocked_reason`, and
`event_triggers_blocked_message` describe a runtime policy pause such as the
local development background-automation guard. Runtime policy must not make the
transport look like the operator turned alert or anomaly triggers off, and the
default Patrol UI must not treat a background-only policy pause as actionable
operator guidance unless it also explains why manual Patrol is blocked.
Patrol mutate endpoints that depend on the background service must also fail
closed with `503 Service Unavailable` when AI service initialization is absent
rather than dereferencing a nil service and crashing before a contract response
is written.
The `/api/recovery/rollups` transport now also carries the same normalized
filter contract as `/api/recovery/points`, `/api/recovery/series`, and
`/api/recovery/facets`: cluster, node, namespace, workload scope,
verification, and free-text query filters must remain coherent across all four
recovery endpoints so the recovery UI cannot render mismatched protected-item
and history views for the same active filter set.
That same recovery API contract now also includes canonical provider-neutral
`itemType` transport. `internal/api/recovery_handlers.go` must normalize
provider-native aliases such as `proxmox-vm` onto the shared recovery item
type vocabulary before filters reach rollups, points, series, or facets, and
those same handlers must preserve that normalized shape back out through
`display.itemType` and facet option payloads instead of forcing frontend
surfaces to re-derive cross-platform recovery categories from raw
`subjectType`.
That same recovery API boundary now treats `platform` as the canonical
operator-facing query field across `/api/recovery/rollups`, `/api/recovery/points`,
`/api/recovery/series`, and `/api/recovery/facets`. The handlers may continue
mapping that boundary onto internal provider fields, but accepted legacy
`provider` aliases must be compatibility-only input and must not replace the
canonical transport query shape.
That same recovery API boundary must also treat `itemResourceId` as the
canonical linked-resource filter and payload field across those same
`/api/recovery/*` endpoints. Accepted legacy `subjectResourceId` aliases may
remain as compatibility-only input or secondary payload fields during the v6
transition, but the shared transport contract and frontend decode path must
normalize them back onto canonical `itemResourceId`.
That same recovery API boundary must also treat `itemRef` as the canonical
external item-reference field across point and rollup payloads. Accepted
legacy `subjectRef` aliases may remain as compatibility-only secondary fields
during the v6 transition, but the shared transport contract and frontend
decode path must normalize them back onto canonical `itemRef`.
That same outbound recovery transport now also treats `platform` and
`platforms` as the canonical response fields for point and rollup payloads.
Compatibility `provider` and `providers` fields may remain during the v6
transition, but the shared API contract and frontend decode path must treat
them as fallback aliases rather than the primary response vocabulary.
`internal/api/contract_test.go` must pin that alias behavior directly, so the
canonical `platform` query and the legacy `provider` fallback cannot drift
between recovery endpoints without tripping the shared API proof surface.
`internal/api/contract_test.go` is the canonical proof owner for that
boundary, so response payload shape plus route and query compatibility like
`itemType`, `type`, and legacy `provider` aliases must be pinned there
whenever the shared recovery transport shape changes.
The same rule now also covers optional nested node cluster endpoint collections
so `frontend-modern/src/api/nodes.ts` does not own its own
`Array.isArray(node.clusterEndpoints)` response-shape branch.
Canonical alert incident and bulk-acknowledge result payloads must now also
flow through frontend API clients without no-op per-module wrapper
normalization when the backend shape is already canonical.
Legacy `alert_identifier` compatibility promotion in unified finding and patrol
run payloads must now also route through one shared helper in
`frontend-modern/src/api/responseUtils.ts` rather than duplicated per-module
record wrappers.
AI frontend clients must now also call canonical status helpers and direct
URL-segment encoding behavior without module-local alias wrappers when those
wrappers add no contract value.
The discovery frontend client must now also centralize typed and agent route
construction through dedicated path builders rather than repeating route
templates or trivial collection-path aliases across each endpoint.
Notifications email config parsing and node cluster endpoint normalization must
now also route through shared scalar coercion helpers in
`frontend-modern/src/api/responseUtils.ts` rather than through per-module
string/boolean/number helper stacks.
The same shared scalar coercion rule now also applies to monitoring agent
lookup timestamps so `lastSeen` normalization does not live as a module-local
`typeof`/`Date.parse(...)` branch in `frontend-modern/src/api/monitoring.ts`.
The same scalar-coercion contract now also covers optional Proxmox
`clusterEndpoints` collections in `frontend-modern/src/api/nodes.ts`:
frontend consumers may normalize endpoint fields, but they must not fork the
canonical collection-shape guard or reintroduce legacy `alert_identifier`
field access once camelCase `alertIdentifier` has been promoted by the shared
response helpers.
The same frontend API contract now also governs Proxmox agent-install command
transport in `frontend-modern/src/api/nodes.ts`: the canonical client request
shape for `/api/agent-install-command` must support both `type:"pve"` and
`type:"pbs"` with the same explicit `enableProxmox` flag, so install-command
surfaces do not fork into ad hoc raw POST payloads for different Proxmox node
types. PVE Host Telemetry Agent consumers that need Patrol actions or
server-opted-in Docker-in-LXC inventory must preserve the explicit
`enableCommands` request field instead of appending command-execution flags in
the browser or implying that the server will infer guest Docker authority from
`enableProxmox`. That same shared client boundary must also validate a non-empty
`command` response and keep the raw backend `token` field inside
`frontend-modern/src/api/nodes.ts` rather than leaking it into downstream UI
state. Downstream Proxmox install-command consumers like the extracted node
setup surface
(`ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`,
`NodeModalAuthenticationSection.tsx`, `NodeModalSetupGuideSection.tsx`,
`nodeModalModel.ts`, and `useNodeModalState.ts`) must then surface those
canonical validation errors
directly rather than collapsing one node-type pane back to generic
copy-generation failure.
Hosted organization-route gating now also falls under this API payload
boundary: when hosted tenants hit organization membership or billing surfaces
through `internal/api/org_handlers.go` and `internal/api/router.go`, inactive
subscriptions must fail with the canonical hosted `402 subscription_required`
payload instead of reusing the self-hosted `multi_tenant_disabled` contract or
falling through to an untyped transport error.
Hosted signup and magic-link error payload normalization must now also route
through shared structured error normalization helpers in
`frontend-modern/src/api/responseUtils.ts` rather than through module-local
error-shape parsing functions.
Governed frontend API clients must now also route canonical non-OK response
throwing through the shared response assertion helper in
`frontend-modern/src/api/responseUtils.ts` rather than open-coding
`throw new Error(await readAPIErrorMessage(...))` in each module.
The same governed modules must now also route assert-then-parse response
pipelines through shared required/optional response helpers in
`frontend-modern/src/api/responseUtils.ts` rather than repeating
`assertAPIResponseOK(...); parseRequiredJSON(...)` or `parseOptionalJSON(...)`
sequences in each client.
Hosted cloud-handoff and billing-admin payloads are canonical API contracts as
well. The handoff exchange must normalize the verified operator email before
it is written into the browser session and before it is returned in the JSON
success payload so session identity, org membership, and handoff payloads
cannot drift on email casing. Hosted billing-admin reads for non-default orgs
must also project the effective default-org hosted lease when the tenant-local
billing file has not been materialized yet, so admin billing-state payloads
stay coherent with the tenant's active entitlement payload instead of briefly
regressing to local trial/default state.
Canonical missing-resource lookups in governed frontend API clients must now
also route `404 => null` response handling through shared response helpers in
`frontend-modern/src/api/responseUtils.ts` rather than open-coding local
status branches in discovery and monitoring clients.
Agent and guest metadata CRUD clients must now also route through one shared
metadata client in `frontend-modern/src/api/metadataClient.ts` rather than
duplicating the same `get/update/delete/list` transport logic in two files.
AI investigation and chat stream clients must now also route through one shared
SSE JSON event consumer in `frontend-modern/src/api/streaming.ts` rather than
duplicating reader lifecycle, timeout, chunk parsing, and event decoding logic
in each module.
Monitoring delete and idempotent mutate clients must now also route `404`/`204`
success cases through shared allowed-status helpers in
`frontend-modern/src/api/responseUtils.ts` instead of open-coding local
status-branch stacks in each method.
The docker-runtime and kubernetes-cluster resource clients in
`frontend-modern/src/api/monitoring.ts` must now also route shared delete,
allowed-missing mutation, and display-name transport mechanics through
canonical resource-oriented helpers in that file rather than duplicating the
same fetch-and-assert stacks across runtime and cluster variants.
The same monitoring resource clients must now also route shared no-body
`POST` actions and success-envelope command triggers through canonical
resource-oriented helpers in `frontend-modern/src/api/monitoring.ts` rather
than duplicating identical `POST` transport logic across reenroll and runtime
command endpoints.
Those helpers must stay named and structured in resource terms rather than
reintroducing managed-resource terminology, so the monitoring transport layer
matches the canonical resource model exposed elsewhere in v6.
Those monitoring command helpers must also preserve the canonical frontend
fetch-options contract: governed callers pass string-keyed headers only, and
empty-body success responses normalize through the shared success-envelope
parsing path rather than local `response.ok` branches.
Legacy persisted Unified Agent scope aliases from v5 and early v6 installs
must also canonicalize to the current `agent:*` scope identifiers at the
backend contract boundary, so existing installed agents continue to satisfy
`agent:report`, `agent:config:read`, `agent:manage`, and `agent:enroll`
requirements without manual token replacement after upgrade. That
canonicalization may live only at request-ingress and persistence/migration
boundaries; live token records, runtime scope checks, and API payloads may not
preserve or re-emit `host-agent:*` aliases.
`GET /api/agents/agent/{id}/config` now also owns desired-config metadata in
the backend API contract. The response `config.desiredConfig` carries a
non-secret versioned hash computed after profile settings and command
enablement decisions have been merged. The existing `signature` field stays
backward-compatible with installed agents and covers the legacy canonical
payload shape: `agentId`, `commandsEnabled`, `settings`, `issuedAt`, and
`expiresAt`. New clients validate `desiredConfig` by recomputing it from the
signed command decision and the signed settings payload, restricted to the
agent-applied settings key schema, rather than treating `desiredConfig` itself
as a direct member of that signature payload.
Agent profile delete and unassign clients must now also route canonical `204`
success handling through shared allowed-status helpers in
`frontend-modern/src/api/responseUtils.ts` instead of open-coding local
`if (!isAPIResponseStatus(response, 204))` branches.
Agent profile suggestion and monitoring display-name mutations must now also
route custom `503` and `404` user-facing error promotion through shared
custom-status error helpers in `frontend-modern/src/api/responseUtils.ts`
instead of open-coding local `if (!response.ok) { if (isAPIResponseStatus(...))
throw new Error(...) }` stacks.
Monitoring command-trigger clients must now also route empty-body
`{ success: true }` fallback behavior through a shared success-envelope helper
in `frontend-modern/src/api/responseUtils.ts` instead of open-coding
`parseOptionalAPIResponse(response, { success: true }, ...)` in each method.
AI chat SSE now also treats interactive `question` events as a canonical API
contract surface: backend and frontend must preserve `session_id`,
`question_id`, and the structured `questions` array without handler-local
rewrites or alternate payload aliases.
That same chat SSE contract must remain request-bound. If the HTTP request
context is canceled or the client disconnects, backend assistant execution
must cancel with the request rather than continuing on a detached background
context until an unrelated timeout expires.
Config-registration API contracts at `/api/auto-register` and
`/api/config/nodes` now also require deterministic automated proof: backend
verification must stub TLS fingerprint capture and Proxmox cluster-detection
probes rather than depending on live network reachability, so canonical
request/response verification reflects contract behavior instead of ambient
lab state.
That same canonical `/api/auto-register` response contract must preserve
node identity on success: `nodeId` must carry the resolved stored node name,
not the raw host URL or requested `serverName`, so registration payloads stay
aligned with fleet-control payload consumers.
That same response contract must also return the rest of the backend-owned
completion identity coherently: `type`, `source`, normalized `host`, and
matching `nodeName` must align with the saved node record so installer and
runtime-side Unified Agent callers do not keep separate local success identities after Pulse has
already canonicalized the node.
That same `/api/auto-register` contract also governs the
`node_auto_registered` WebSocket payload: it must emit the normalized stored
host plus the resolved stored node identity in `name`, `nodeId`, and
`nodeName`, rather than leaking raw request fields that can diverge from the
saved node record, together with the effective token id that was reused or
issued.
AI and agent-profile collection/detail clients must now also route `apiFetchJSON`
`402`/`404` fallback behavior through shared API-error-status fallback helpers in
`frontend-modern/src/api/responseUtils.ts` instead of open-coding local
`try/catch` wrappers that map those statuses to `[]`, `{ plans: [] }`, or
`null`.
Paywalled Patrol remediation-intelligence responses must also scrub derived
metadata together with the collection itself: when remediation history is
license-locked, `remediations`, `count`, and `stats` must all collapse to an
explicit empty state rather than leaking paid history totals through a partial
payload.
Hosted billing-state payloads now also treat Stripe webhook-backed commercial
state as canonical API contract data: when checkout and subscription webhooks
persist paid state, `plan_version`, `stripe_price_id`, and active paid-feature
limits must stay aligned while retired monitored-system volume keys are
scrubbed instead of being restored from stale checkout or canceled-state
carryover.
That same hosted billing API boundary also owns runtime base-path resolution:
`internal/api/payments_webhook_handlers.go` must derive webhook dedupe and
customer-index storage from the shared runtime data-dir helper in
`internal/config/config.go` instead of carrying its own `/etc/pulse` fallback,
so hosted billing API side effects stay aligned with the same configured data
directory used by the rest of the product.
Not-found detail lookups in governed frontend API clients must now also route
through explicit status-based `404` handling rather than through broad
catch-all `null` fallbacks that hide real backend failures.
Session and CSRF persistence compatibility under `internal/api/session_store.go`
and `internal/api/csrf_store.go` now also has an explicit governed migration
proof route: legacy raw-token `sessions.json` and `csrf_tokens.json` files must
load through explicit migration helpers, rewrite immediately into hashed
canonical persistence, and stay covered by
`internal/api/session_store_test.go`, `internal/api/csrf_store_test.go`, plus
`tests/migration/v5_session_db_test.go`, rather than borrowing the generic
backend payload contract proof path.
That same governed auth persistence boundary must also stay owned by the
configured runtime data path instead of hidden package-singleton fallbacks:
session, CSRF, and recovery-token stores may not silently self-initialize on
`/etc/pulse` from first access or lock onto the first caller forever through
`sync.Once`; the configured router data path must remain the canonical owner of
those persistence stores, and reinitializing that data path must replace the
old runtime store rather than leaking prior-path state forward.
That same configured-path rule also applies to runtime auth/config reloads:
`internal/config/watcher.go` may use `PULSE_AUTH_CONFIG_DIR` only as an
explicit override, but otherwise it must watch the resolved runtime
`ConfigPath` / `DataPath` owner. The watcher may not probe `/etc/pulse` or
`/data` and silently override the configured path authority for `.env` and
`api_tokens.json` reloads.
That same configured-path rule also applies to manual auth env writes and
status reads under `internal/api/router.go`,
`internal/api/router_routes_auth_security.go`, and
`internal/api/security_setup_fix.go`: those handlers must resolve `.env`
through the shared auth-path helper instead of rebuilding `/etc/pulse/.env`
fallback logic inline.
That same governed auth persistence rule now also covers recovery-token state
under `internal/api/recovery_tokens.go`: raw recovery secrets may be minted for
one-time operator use, but `recovery_tokens.json` must persist only token
hashes and treat any legacy plaintext-token file as an explicit migration input
that is rewritten immediately into hashed canonical persistence on load instead
of leaving raw recovery secrets on the primary runtime disk path.
That same governed persistence rule also covers `internal/config/persistence.go`
API token metadata handling: `api_tokens.json` may hold only hashed token
records, but a legacy plaintext metadata file may only be migration input.
Canonical runtime persistence must rewrite plaintext API token metadata
immediately into encrypted-at-rest storage on load instead of treating the
unencrypted file as a normal primary path.
That same fail-closed persistence rule also applies to persisted OIDC refresh
tokens in `internal/api/session_store.go`: refresh tokens may only be loaded
from or saved to encrypted-at-rest session payloads, and the runtime must drop
them whenever session-store crypto is unavailable or the stored ciphertext is
not canonically decryptable instead of preserving plaintext-at-rest session
state.
OIDC access-token issued-at metadata is part of the same session persistence
contract. Short-lived access tokens must persist `oidc_access_token_issued_at`
beside `oidc_access_token_exp` so refresh scheduling can use a relative
lifetime window after restart instead of treating every five-minute token as
immediately refreshable.
Hosted signup handler payload flow now also follows an explicit shared
boundary: `internal/api/public_signup_handlers.go` owns request/response and
magic-link payload semantics, while `internal/hosted/provisioner.go` owns the
shared org bootstrap and rollback mechanics that the hosted signup handler
invokes.
That shared public-signup response contract is now intentionally uniform for
syntactically valid requests: the route returns `202 Accepted` with one generic
Pulse Account message whether provisioning/email side effects ran or were
suppressed by the owner-email rate limiter, while invalid request bodies and
true server failures remain explicit.
The API token settings surface now also follows the same explicit ownership
rule. Changes to `frontend-modern/src/components/Settings/APITokenManager.tsx`,
`frontend-modern/src/components/Settings/apiTokenManagerModel.ts`, and
`frontend-modern/src/components/Settings/useAPITokenManagerState.ts` must
carry this contract and the dedicated API-token management proof file instead
of remaining an unowned consumer of token scope labels, token assignment
visibility, and revoke-state presentation.
That shared API-token boundary must also stay under explicit proof routing on
both sides instead of relying only on broad settings-surface coverage on the
security side: token settings changes must continue to carry the direct
`api-token-management-surface` API-contract proof together with the
security-side surface proof.
That same shared commercial API boundary now treats local trial-start transport
as retired for normal self-hosted v6 GA. `POST /api/license/trial/start` must stay
out of the router inventory and must not return the old hosted-signup or
trial-rate-limit acquisition payloads from an ordinary self-hosted runtime;
`frontend-modern/src/api/license.ts`, demo mode, and feature gates must not
expose a start-trial client method or in-app CTA in the same slice as any
handler change. Commercial migration state must travel through
`commercial_migration`, not through trial-denial reason strings.
That migration transport must cover every degraded path, not just exchange
rejections: when a persisted v5 license exists but cannot be read or
decrypted, the runtime must publish a terminal `commercial_migration` state
(`persisted_license_unreadable`) instead of degrading to Community behind a
log line. Startup legacy-exchange failures classified as pending must
self-retry in the background with backoff for the life of the process —
a transient license-server or DNS failure at first boot must not require a
manual restart or panel retry to complete a paid migration.
That same shared commercial API boundary also owns hosted self-serve failure
transport semantics. Hosted trial request and verification failures may render
owned HTML pages, but they must preserve the originating Pulse instance and
customer form context instead of collapsing into generic control-plane failures
or dead-end text with no route back to the originating runtime.
That same boundary must also keep token scope presets lazily derived from the
canonical scope constants: `apiTokenManagerModel.ts` may expose
`getAPITokenScopePresets()`, but it must not publish an eagerly evaluated
top-level preset array that can reintroduce settings-chunk initialization-order
failures in production bundles.
That same boundary now also includes
`frontend-modern/src/utils/apiTokenPresentation.ts`, so token load/create/
revoke errors keep one governed customer-facing message source instead of
reappearing as hook-local strings.
That same token surface, together with `frontend-modern/src/api/security.ts`,
`internal/api/security.go`, `internal/api/security_tokens.go`, and
`internal/api/system_settings.go`, now also follows an explicit shared
boundary with `security-privacy` so auth posture, token authority, and
telemetry/privacy control semantics stop borrowing their governance only from
the broader API lane.
The `/api/security/tokens` payload contract now also carries explicit owner
binding: token create/list responses must preserve the originating
`ownerUserId` together with org scope so long-lived automation credentials
cannot appear detached from their intended human identity, and shared
token-minting helpers must reject caller-supplied metadata that attempts to
overwrite the reserved `owner_user_id` field. That owner binding now extends
to token constructors outside the generic token manager, including agent
install, deploy bootstrap/runtime enrollment, container runtime migration,
first-run setup, and token-regeneration paths.
The shared direct-node/discovery settings boundary now also includes
`frontend-modern/src/utils/infrastructureSettingsPresentation.ts`, so the
customer-facing mutation and validation copy used by the governed runtime
hooks stays explicit under the same API-backed settings proof instead of
living as an unowned utility.
That same backend-owned config/settings boundary also owns shipped security-doc
references in operator guidance. `internal/api/config_system_handlers.go` and
shared setup helpers must not point API responses or runtime guidance at
GitHub `main` for security instructions that the running build already serves
locally; those references belong on the shipped `/docs/SECURITY.md` path.
That same governed token contract must fail closed on mutation. Limited-scope
API tokens may only create, rotate, or delete tokens whose effective scopes
are a subset of the caller's own scopes; token-management routes must not let a
settings-capable but narrower token revoke or replace a broader credential.
Those owner-bound credentials now also define the effective authenticated
principal on governed API routes: when token metadata carries `ownerUserId`,
RBAC and audit-facing auth resolution must use that bound user identity rather
than a detached synthetic `token:<id>` subject, while still preserving token
scope and org enforcement.
The onboarding QR payload flow now also carries explicit token-bound auth
semantics: when the frontend requests `/api/onboarding/qr` with a pairing
token, the API client must send that token explicitly so the returned payload
and deep link represent the exact minted pairing credential rather than the
ambient browser session, and the mobile-facing `relay.url`/`relay_url` fields
must normalize the stored relay instance endpoint to the app endpoint
(`/ws/app`) so mobile pairing never receives the instance-only `/ws/instance`
route.
Incoming organization-share payloads now also preserve requested access-role
semantics at the API boundary: `/api/orgs/{id}/shares/incoming` must hide
shares whose `accessRole` exceeds the caller's effective role in the target
organization instead of leaking share metadata that the caller cannot
legitimately accept or use.
That same inbound-sharing contract now also carries explicit target-org
consent semantics. `POST /api/orgs/{id}/shares` must create pending share
requests rather than granting live access immediately, target-org owners or
admins must accept or decline those requests through
`POST /api/orgs/{id}/shares/incoming/{shareId}/accept` and
`DELETE /api/orgs/{id}/shares/incoming/{shareId}`, and
`/api/orgs/{id}/shares/incoming` must expose pending requests only to those
target-org managers. Once accepted, the payload must preserve `status`,
`acceptedAt`, and `acceptedBy`, and accepted shares may remain visible only to
members whose effective role satisfies the share's `accessRole`.
Updating an already accepted share must also preserve that consent boundary:
changing the requested `accessRole` resets the share to `pending` and clears
the acceptance metadata so a source org cannot silently widen an approved
grant without a new target-side approval.
Organization membership and authorization payloads now also follow an explicit
live-role contract: `/api/orgs` must list only organizations the caller
currently belongs to, and org-management endpoints must reflect member
promotion or demotion immediately rather than continuing to authorize from
stale owner/admin assumptions after the role change has already been
persisted.
System settings API payloads now also carry an explicit v6 channel contract:
`updateChannel` resolves to `stable` or `rc` with `stable` as the default, and
`autoUpdateEnabled` must serialize as `false` whenever the effective channel is
`rc`, even if stale persisted state or omitted request fields would otherwise
leave unattended updates enabled.
Update API channel selection now also follows that same contract: `/api/updates`
surfaces accept only `stable` or `rc`, reject unsupported channel values at the
HTTP boundary, and must not allow a `stable` installation path to apply a
prerelease tarball even when a caller posts a direct GitHub release URL.
The `/api/resources` and `/api/resources/stats` handlers now also carry a
single-snapshot aggregation invariant: canonical `aggregations.byType` must be
derived from the same registry list snapshot used for that request's response
path, so the contract stays deterministic without paying for duplicate
registry-clone work on the hot path. That same governed resource contract now
also includes backend-derived `policy` and `aiSafeSummary` fields, and list,
detail, and child payloads must source those values from canonical unified
resource metadata rather than from frontend- or AI-local heuristics.
`/api/resources`, `/api/resources/stats`, and `/api/state` also share the same
presentation coalescing boundary for host-shaped resources. When multiple
authoritative reports describe the same host identity, resource handlers and
state serialization must consume `ResourceRegistry.ListForPresentation` or the
shared `CoalescePresentationHostResources` helper rather than reimplementing a
route-local merge. Report-merge exclusions created from canonical ingestion
remain authoritative at that boundary, so presentation coalescing may remove
duplicate host fragments but must not rejoin resources the registry has already
recorded as intentionally separate.
That same resource-handler seed contract must also stay on canonical unified
resource ownership for tenant-scoped requests: once a tenant state provider
implements `UnifiedResourceSnapshotForTenant`, `/api/resources` may not fall
back to raw tenant `StateSnapshot` seeding when that unified seed is empty.
That same mock/runtime contract now also governs chart payloads under
`internal/api/router.go`: when demo or mock presentation is enabled,
`/api/charts`, `/api/charts/infrastructure`, and `/api/storage-charts` must
read through `GetUnifiedReadStateOrSnapshot()` so chart payloads use the same
canonical mock unified-resource snapshot as `/api/resources` and `/api/state`
instead of drifting onto the live store-backed graph.
Tenant AI service wiring now follows that same canonical ownership rule:
`internal/api/ai_handlers.go` may provide tenant `ReadState` and
tenant-scoped unified-resource providers, but it must not mint tenant snapshot
provider bridges purely to satisfy Patrol once the Patrol runtime can operate
from those canonical tenant providers directly.
Hosted licensing handlers now also carry a tenant-scoped fallback contract:
when hosted auth handoff preserves a non-default tenant org like `t-...`,
`/api/license/status`, `/api/license/commercial-posture`,
`/api/license/entitlements`, and `/api/license/runtime-capabilities` must
still evaluate the instance-level hosted billing lease from `default` if that
tenant org has no org-local billing state of its own, rather than failing
closed into
`subscription_required` on first entry.
That same hosted entitlement contract also owns lease refresh targeting:
when a hosted tenant request arrives on a non-default org with no org-local
lease, `internal/api/hosted_entitlement_refresh.go` must resolve the effective
billing target through the same default-org fallback before it refreshes,
persists, or rewires the evaluator. Runtime routes such as
`/api/ai/approvals` must not refresh against the empty tenant org and silently
fall back to `license_required` while the real hosted entitlement lease still
exists on `default`.
That same hosted browser-session contract must also remain authoritative once
the handoff lands on the tenant runtime: when a valid `pulse_session` cookie
is present, shared `internal/api/auth.go` helpers must authenticate that
session before any API-only token fallback or no-local-auth anonymous fallback
is considered, so hosted protected routes such as relay-mobile token minting,
onboarding reads, and billing-admin/API surfaces stay reachable after cloud
handoff instead of flattening the operator back to `anonymous` or demanding a
bearer token from the browser as soon as the tenant has minted one.
That same shared auth contract also governs unauthenticated local recovery and
bootstrap ingress: before auth exists, anonymous fallback and `/api/security/quick-setup`
must remain direct-loopback only, and recovery tokens may authorize only the
same loopback client IP that minted them when establishing a browser recovery
session.
That same shared settings-scope contract must then preserve canonical
org-management privilege on the tenant side: when a hosted or multi-tenant
request is scoped to a non-default org, `internal/api/security_setup_fix.go`
must honor the org's owner/admin membership model for settings-bound routes
such as relay-mobile token minting, instead of requiring a separate configured
local admin username that hosted tenants do not carry.
The same onboarding boundary in `internal/api/router_routes_ai_relay.go` and
`internal/api/relay_mobile_capability.go` must
also accept the dedicated `relay:mobile:access` scope for
`/api/onboarding/qr`, `/api/onboarding/validate`, and
`/api/onboarding/deep-link`, because those payloads are the canonical
bootstrap surface for the server-minted mobile credential.
The shared security token contract now also includes single-record metadata
reads. `internal/api/security_tokens.go`,
`internal/api/router_routes_auth_security.go`,
`frontend-modern/src/api/security.ts`, and
`frontend-modern/src/types/api.ts` own the canonical `record.lastUsedAt` and
`record.expiresAt` lookup shape for one token, and relay pairing surfaces must
consume that same contract when deciding whether a displayed QR token can be
revoked or must be preserved as an already-used device credential. That same
contract now also owns backend-minted Pulse Mobile relay access tokens: the
server route, not the browser, defines the canonical dedicated
`relay:mobile:access` runtime scope, the explicit route inventory in
`internal/api/relay_mobile_capability.go`, its backward-compatible
server-side route gates alongside legacy `ai:chat` and `ai:execute` mobile
tokens, and the token-purpose metadata. Route expansion for Pulse Mobile must
land by editing that backend-owned inventory plus its proofs, rather than by
sprinkling ad hoc compatibility checks across handlers. The pairing UI only
consumes that server-owned credential when requesting the onboarding payload.
That same shared backend API contract now also owns hosted relay bootstrap
reads. `internal/api/router.go`, `internal/api/onboarding_handlers.go`, and
`internal/api/relay_hosted_runtime.go` must derive `/api/settings/relay` and
the mobile onboarding payload from the same runtime helper. In hosted mode,
when no explicit relay config exists but the default hosted billing lease
grants `relay` and carries an entitlement JWT plus canonical `instance_host`,
those read surfaces must auto-bootstrap the persisted relay runtime with the
default relay server URL, a machine-owned hosted instance secret, and
generated relay identity metadata instead of requiring a prior manual
`PUT /api/settings/relay`. The API response contract must continue to expose
only public relay fields while omitting the hosted instance secret and
private key.
That same shared backend API contract now also owns hosted AI bootstrap
retirement. `internal/api/ai_hosted_runtime.go`, `internal/api/ai_handler.go`,
`internal/api/ai_handlers.go`, and `internal/api/contract_test.go` must derive
`/api/settings/ai` and the initial hosted AI runtime from the same runtime
helper. In hosted mode, when no explicit `ai.enc` exists, those read surfaces
must return the same unconfigured BYOK/local-provider default used by
self-hosted settings instead of persisting a quickstart-backed AI config with
the old `quickstart:pulse-hosted` alias. Hosted tenant-org reads may still
inherit the default hosted billing lease for commercial authorization, but they
must not turn that lease into AI quickstart credits or a managed-model runtime.
Once a real AI config exists, that explicit operator-owned state remains
authoritative.
The same hosted contract now also requires tenant Pulse Assistant runtime
startup to consume that hosted-aware config path and to refuse caching a
failed tenant chat service, so tenant-org `/api/ai/status` and
`/api/ai/sessions` cannot stay wedged behind a stale pre-bootstrap service
after the lease-backed AI config has been persisted.
That same shared AI/mobile API contract now also owns approval-list readiness
for settings-driven enablement. `internal/api/ai_handler.go`,
`internal/api/ai_handlers.go`, `internal/api/router_routes_ai_relay.go`, and
`internal/api/contract_test.go` must keep the governed approvals-list surface
on its empty-list payload once AI is enabled, even when the first enablement
happens after process startup. A post-boot settings save may not leave that
surface on `503 Approval store not initialized` just because the direct AI
runtime had not previously started.
That same shared AI settings contract also owns provider-auth continuity and
provider-scoped test selection. `internal/api/ai_handlers.go` and
`internal/api/contract_test.go` must expose masked Ollama auth state through
`ollama_username` and `ollama_password_set`, accept provider-auth updates
without echoing raw secrets back into the payload, and keep provider test
routes bound to the provider's own configured model instead of whichever
other provider currently owns the default `model` selection. Legacy Anthropic
OAuth fields are cleanup-only compatibility state on that same contract and
must never revive an OAuth-backed Anthropic provider path.
That same shared `/api/settings/ai` contract now also owns vendor-neutral BYOK
setup. Frontend callers may submit provider credentials or base URLs without a
concrete vendor model ID, and `internal/api/ai_handlers.go` must resolve and
persist the effective `model` through the canonical runtime provider-catalog
selection path before returning the updated payload. `/api/settings/ai` reads
must then echo that resolved model back as the canonical default selection, so
UI setup flows and provider test routes do not drift into frontend-baked model
defaults or handler-local vendor fallbacks.
That same shared config/runtime contract also owns import-triggered reload
safety. When `internal/api/config_export_import_handlers.go` imports a config
archive and rebinds shared runtime state, the reload path must tolerate absent
notification or monitoring managers and degrade gracefully instead of
panicking on optional side effects. `/api/config/import` may be exercised from
proof or setup contexts that do not yet have every long-lived runtime manager
wired, but the contract must still leave the imported configuration readable
through the canonical API surface.
`/api/config/export` and `/api/config/import` are therefore router auth-bypass
entries only so their handlers can own stricter route-local auth and
public-network checks; global middleware must not preempt those handlers or
mask their public-network `403` outcomes with a generic auth failure.
That same shared infrastructure-settings API contract now also owns the
connected-infrastructure distinction between machine-managed and
platform-connections-managed reporting. `frontend-modern/src/types/api.ts`,
`frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`,
`frontend-modern/src/components/Settings/useConnectionsLedger.ts`, and
`frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx`
must treat `truenas` as a canonical connected-infrastructure surface kind
alongside `proxmox`, `pbs`, and `pmg`, and the settings reporting/install
surfaces must keep those platform-managed rows navigable back to platform
connections instead of presenting host uninstall or stop-monitoring actions
that only apply to `agent`, `docker`, and `kubernetes`.
Agentless availability targets extend the managed-source distinction without
living in the platform-source settings home. The
API contract for `availability` rows is an address/protocol probe target plus
runtime status, projected through the connections ledger and unified resources
as a `network-endpoint`. Browser callers may test unsaved or saved targets, but
the persisted target list remains owned by `/api/availability-targets` and
must be managed from `/settings/monitoring/availability`, not reconstructed
from resource snapshots or monitored-system counts.
Mock availability fixtures must still behave like saved targets: `/api/connections`
reports them as availability rows, `/api/availability-targets` lists them with
probe status, and saved-test calls return the synthetic probe result instead of
attempting live network I/O against demo-only addresses.
That same shared metrics-history contract now also owns physical-disk live I/O
windows. `internal/api/router.go` must accept `resourceType=disk` on
`/api/metrics-store/history`, keep `30m` as a valid compact live range, and
resolve `disk`, `diskread`, `diskwrite`, and `smart_temp` against the
canonical disk `MetricsTarget.ResourceID` the unified resource already
exposes. Storage drawers and other consumers must not fork a disk-local live
history route, alternate query identity, or feature-specific fallback payload
when the governed chart API already owns that transport.
The shared browser contract now also includes a neutral app-runtime context
boundary for websocket-backed API consumers. API-contract-owned hooks such as
`frontend-modern/src/components/Settings/useAPITokenManagerState.ts` and
`frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`
may read websocket state through `frontend-modern/src/contexts/appRuntime.ts`,
but payload truth, bootstrap rules, and commercial identity still belong to
the governed API handlers and contract tests. Those hooks must not import
`@/App` or treat root-shell ownership as transport authority.
That same shared commercial API contract now also owns the public demo
read-side boundary. `internal/api/demo_mode_commercial.go`,
`internal/api/licensing_handlers.go`,
`internal/api/monitored_system_ledger.go`, and
`internal/api/subscription_state_handlers.go` must fail closed with a generic
`404` for public-demo billing, license-status, and monitored-system-ledger
reads or preview probes whenever `DEMO_MODE` is enabled. Demo runtimes may
still use real server-side entitlement evaluation internally, but the
governed browser/API contract must not expose commercial identity, usage, or
upgrade-state payloads back to public viewers through those read surfaces.
The adjacent public-demo admin-operations policy also hides `GET`/`HEAD`
probes for `/api/admin/users` and `/api/discover` with the same generic `404`
posture, while non-read attempts still fall through to the demo read-only
mutation block so write probes retain the canonical `403`.
That same monitored-system inventory contract now also owns direct write-path
semantics for platform connections. `internal/api/truenas_handlers.go`,
`internal/api/vmware_handlers.go`, and `internal/api/contract_test.go` must
allow TrueNAS and VMware connection creates/updates without monitored-system
volume admission, even when the explanatory monitored-system view is unsettled
or rebuilding. VMware writes should report provider validation failures from
the provider path itself rather than masking them behind capacity accounting.
That same browser-transport contract now tolerates sparse preview
payloads without changing the runtime truth. Patrol transport may omit
`finding_ids`, and infrastructure removal previews may stage optimistic rows
only after canonical IDs have been resolved or a safe row-name fallback has
been chosen. API-adjacent browser callers must not reinterpret missing IDs or
preview arrays as authoritative empty success.
That same shared browser transport contract now also owns the discovery polling
mount scope.
`frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts`
no longer gates `/api/discover` polling on the settings tab name; polling
starts whenever the hook is mounted and stops on cleanup. Callers must not
re-introduce a per-tab gate on this boundary. The discovery subnet settings
write path through `SettingsAPI.updateSystemSettings` remains governed by the
shared `internal/api/` settings boundary and is unaffected by the polling
scope change.
The shared Patrol autonomy API contract now separates findings-only monitor
configuration from paid remediation autonomy. `GET /api/ai/patrol/autonomy`
continues to clamp effective Community autonomy to `monitor`, and
`PUT /api/ai/patrol/autonomy` in the open-source/free adapter must accept and
persist only `monitor` settings while returning the canonical license-required
payload for investigation or remediation autonomy levels. Frontend Patrol state
owners must not rely on that 402 as normal control flow for stale local state:
when the current entitlement locks safe remediation, they submit `monitor` even
if older persisted settings or a previous entitlement left `approval`,
`assisted`, or `full` in memory.
The Patrol header and configuration-dialog presentation for that API boundary
must compose the shared
`frontend-modern/src/components/shared/FilterButtonGroup.tsx` selector: the
endpoint contract still owns the accepted autonomy values and license-required
response shape, while the frontend owns only the option mapping,
entitlement-derived disabled state, and layout choice needed to keep labels
readable in the current container. Local Patrol selector styling must not become
a second source of truth for the API's monitor-only clamp.
The reporting transport contract now also carries an optional narrative
interpretation layer alongside the deterministic data surface. The Go-side
`pkg/reporting.MetricReportRequest` gains optional `Narrator` and
`FindingsProvider` fields that handlers populate from the per-tenant AI
service when configured; both fields stay handler-internal and never
appear on the wire. The corresponding `ReportData` struct gains
`Narrative *Narrative`, `PriorPeriod *PriorPeriodInput`, and
`Findings []FindingSummary` fields that the engine populates before
rendering. When the request omits a narrator, the engine still attaches
a heuristic narrative through `HeuristicNarrator` so renderers always
have one source of truth, and the rendered PDF output is
byte-equivalent to the prior heuristic-only contract. When the
narrator is supplied, the engine queries the comparable prior window
automatically (same length, ending at `req.Start`) and threads its
aggregate stats through `NarrativeInput.PriorPeriod` so deltas can be
expressed; `internal/api/metrics_reporting_handlers.go` does not have
to plumb a second request for this.
The PDF output owns three additional rendered sections gated on
`ReportData.Narrative`: an executive prose paragraph between the
deterministic health card and the deterministic Quick Stats table, a
`Period-over-period changes` section after Recommended Actions, and a
muted provenance footer (`Pulse Assistant narrative.
Verify against the data tables in this report.`) when
`Narrative.Source` is `ai`. Charts, stats tables, alert lists, storage
and disk sections must remain deterministic and rendered from the
same underlying `ReportData` aggregates so every AI claim is
verifiable against the data immediately adjacent to it; the AI
narrator owns the prose layer only, not the verifiable surface. The
narrator must fail closed: nil provider, parse failure, timeout, or
empty response causes the engine to fall back to the heuristic
narrative without surfacing the AI failure to the caller, so reporting
is never blocked by AI availability.
Multi-resource fleet reports (`engine.GenerateMulti`) now also carry an
optional fleet-level narrative through a distinct
`pkg/reporting.FleetNarrator` interface, kept separate from the
single-resource `Narrator` because the input shape is different (one
cross-resource view rather than one resource's stats with a prior
window). `MultiReportRequest` gains optional `FleetNarrator`,
`Narrator`, and `FindingsProvider` fields; `MultiReportData` gains
`FleetNarrative *FleetNarrative` populated by the engine before
rendering. Per-resource narratives are intentionally not produced on
the multi path: a 50-resource fleet report would otherwise trigger 50
AI calls. Synthesis lives in the single fleet-level call instead.
The fleet PDF renders the FleetNarrative in the fleet summary cover
when present, replacing the legacy "Highest CPU / Most alerts"
heuristic bullets. The narrative section owns: an executive prose
paragraph, a `Resources to investigate` outlier list (named
resources, severity-coloured), a `Cross-cutting patterns` section,
recommendations, an optional period-comparison paragraph, and a
provenance footer when `Source` is `ai`. The deterministic resource
summary table (resource, type, status, avg CPU/memory/disk, alert
count) remains rendered from the same per-resource aggregates
above the narrative section, so every named outlier is verifiable
against the table immediately below it.
Both narrators (single-resource and fleet) carry a detection-
boundary invariant in their system prompts: a "warning" or
"critical" classification on any observation, outlier, or pattern
must be backed by a Patrol finding, an alert, or a hard-threshold
breach visible in the structured input (cpu max above 90, memory
avg above 85, disk avg above 85, failed or high-wear disks, or
storage pools at 90 percent or more). Trends the narrator infers
from metric data without that backing are constrained to "info"
severity. The same constraint applies to recommendations.
Detection and severity classification are owned by Patrol; the
report narrators summarize Patrol's work and must not function as
parallel classifiers, so the report PDF cannot become a back-door
detection surface that diverges from Patrol's findings store.
The reporting engine also exposes two non-rendering entry points
on its `Engine` interface — `NarrativeFor(req MetricReportRequest)
(*Narrative, error)` and `FleetNarrativeFor(req MultiReportRequest)
(*FleetNarrative, error)` — that return the structured narrative
without invoking the PDF or CSV output stage. These are the seams
Pulse Assistant tools and other programmatic consumers use to reach
the same retrospective synthesis the report carries, in a form they
can present in chat or another non-export context. Both run the
same query path, the same narrator resolution, and the same
fail-closed-to-heuristic fallback as their rendering counterparts;
they differ only in skipping the fpdf/csv stage. Test stubs
implementing the Engine interface must implement these methods so
the contract is honoured across the entire interface surface, not
just the export-shaped subset.
