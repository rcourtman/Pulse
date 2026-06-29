# Agent Lifecycle Contract

## Contract Metadata

```json
{
  "subsystem_id": "agent-lifecycle",
  "lane": "L16",
  "contract_file": "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": ["api-contracts"]
}
```

## Purpose

Own unified agent installation, registration, update continuity, profile
management, and fleet control surfaces. Pulse v6 has one host-installed
infrastructure agent binary, `pulse-agent`; host, Docker / Podman,
Kubernetes, Proxmox-local, and other node-local telemetry are modules inside
that binary, not separate customer-facing agent products.

## Canonical Files

1. `internal/api/agent_install_command_shared.go`
2. `internal/api/config_setup_handlers.go`
   2a. `internal/api/setup_script_render.go`
3. `internal/api/unified_agent.go`
4. `internal/agentupdate/update.go`
5. `internal/hostagent/agent.go`
6. `cmd/pulse-agent/main.go`
7. `scripts/install.sh`
8. `scripts/install.ps1`
9. `frontend-modern/src/api/agentProfiles.ts`
10. `frontend-modern/src/components/Settings/AgentProfilesPanel.tsx`
11. `frontend-modern/src/components/Settings/agentProfileSettings.ts`
12. `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`
13. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`
14. `frontend-modern/src/components/Settings/connectionsTableModel.ts`
15. `frontend-modern/src/components/Settings/useConnectionsLedger.ts`
16. `frontend-modern/src/components/Settings/useConnectionRowActions.ts`
17. `frontend-modern/src/components/Settings/ConnectionEditor/ConnectionEditor.tsx`
18. `frontend-modern/src/components/Settings/ConnectionEditor/AddressProbeStep.tsx`
19. `frontend-modern/src/components/Settings/ConnectionEditor/useConnectionEditor.ts`
20. `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`
21. `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx`
22. `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx`
    22a. `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx`
23. `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`
    23a. `frontend-modern/src/components/Settings/InfrastructureAgentUpdatesDialog.tsx`
24. `frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx`
25. `frontend-modern/src/components/Settings/InfrastructureSourcePicker.tsx`
26. `frontend-modern/src/components/Settings/InfrastructureDiscoverySettingsDialog.tsx`
27. `frontend-modern/src/components/Settings/DiscoverySettingsForm.tsx`
28. `frontend-modern/src/components/Settings/discoverySettingsModel.ts`
29. `frontend-modern/src/components/Settings/infrastructureWorkspaceModel.ts`
    29a. `frontend-modern/src/components/Settings/infrastructureAgentUpdateCommandsModel.ts`
30. `frontend-modern/src/components/Settings/proxmoxSettingsModel.ts`
31. `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`
32. `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`
33. `frontend-modern/src/components/Settings/NodeModalAuthenticationSection.tsx`
34. `frontend-modern/src/components/Settings/NodeModalBasicInfoSection.tsx`
35. `frontend-modern/src/components/Settings/nodeModalModel.ts`
36. `frontend-modern/src/components/Settings/NodeModalMonitoringSection.tsx`
37. `frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx`
38. `frontend-modern/src/components/Settings/NodeModalStatusFooter.tsx`
39. `frontend-modern/src/components/Settings/useNodeModalState.ts`
40. `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`
41. `frontend-modern/src/utils/agentProfilesPresentation.ts`
42. `frontend-modern/src/utils/agentInstallCommand.ts`
    42a. `frontend-modern/src/utils/agentVersion.ts`
43. `frontend-modern/src/utils/infrastructureOnboardingPresentation.ts`
44. `frontend-modern/src/api/nodes.ts`
45. `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
46. `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`
47. `frontend-modern/src/components/Settings/infrastructureSettingsModel.ts`
48. `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts`
49. `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts`
50. `frontend-modern/src/utils/infrastructureSettingsPresentation.ts`
51. `frontend-modern/src/utils/agentCapabilityPresentation.ts`
52. `frontend-modern/src/utils/agentProfileSuggestionPresentation.ts`
53. `frontend-modern/src/utils/unifiedAgentInventoryPresentation.ts`
54. `frontend-modern/src/utils/unifiedAgentStatusPresentation.ts`
55. `frontend-modern/src/utils/nodeModalPresentation.ts`
56. `frontend-modern/src/components/Settings/useTrueNASSettingsPanelState.ts`
57. `frontend-modern/src/components/Settings/useVMwareSettingsPanelState.ts`
58. `frontend-modern/src/components/Settings/MonitoredSystemImpactPreview.tsx`
59. `internal/hostagent/proxmox_setup.go`
60. `internal/remoteconfig/client.go`
61. `internal/agenttls/config.go`
62. `internal/api/agent_exec_token_binding.go`

## Shared Boundaries

`internal/dockeragent/` is lifecycle-adjacent for agent binary/update trust
and owns the Docker / Podman collection module used by `pulse-agent`, but
Docker runtime capability truth is monitoring-owned. Lifecycle consumers must
not reinterpret standalone `Swarm.LocalNodeState=inactive` metadata as agent
enrollment, install, command, or fleet-control authority.
Inside-guest Docker / Podman visibility is also a privacy boundary: full
Docker / Podman inventory may come from a guest-local agent or another explicit
guest reporting path. LXC Docker inventory may also come from the Proxmox host
agent, but only through the explicit, server-side opt-in inventory mode that
uses minimal Docker summary commands rather than unbounded guest scraping. A
local `--enable-docker=false` or `PULSE_ENABLE_DOCKER=false` remains a hard
Unified Agent opt-out that auto-detection and remote profile settings cannot
reverse.
Proxmox workload actions may use the parent node `pulse-agent` only when the
shared `/api/resources` payload carries a backend-authored Proxmox workload
target linked to that node agent. Suppressing an inside-guest install cue for
that case means governed actions have a node-agent path; it must not be
presented as proof that a guest-local agent is installed.
Agent-facing operations-loop status wiring in `internal/api/router.go` and
`internal/api/agent_resource_context.go` is lifecycle-adjacent only because it
shares agent route infrastructure. Other handlers in `internal/api/` such as
the AI settings handler (`ai_handlers.go`) carry AI provider configuration
(for example per-provider base URL overrides) that is ai-runtime config-surface
and is not agent enrollment, liveness, or lifecycle state. Workflow starter counts on that endpoint,
contextual Assistant/external-agent collaboration counts inside the Assistant
step, the content-free Patrol control starter split, and Patrol control
completed-loop, resolved-loop, or `patrolControlValueState` proof mirrored to
`patrolAutonomyValueState` compatibility fields are
API-contract telemetry/orientation fields, not agent enrollment, liveness,
setup completion, install, update, command, recovery, or fleet-control
lifecycle authority. Native `pulse_patrol` starts, current paid
`patrol_control` starts, legacy `patrol_autonomy` starts, and legacy
`pulse_pro_activation` starts all contribute to Patrol control starter orientation;
`proActivationOperationsLoopStarterCount` remains a legacy entry-point counter
for the old alias only, while the legacy completed/resolved/value
`proActivation*` fields are compatibility aliases for that same first-party
Patrol control proof while older commercial and telemetry consumers migrate.
Lifecycle surfaces must
not treat `verified`,
legacy-compatible `verified_needs_mcp`, or `governed_decision_recorded` as
proof that an agent install, update, registration, profile rollout, or fleet
command completed. External-agent/MCP readiness is optional collaboration
setup for agents outside the app, not a first-party Patrol control completion
gate and not a lifecycle setup stage.
Server update funnel telemetry is lifecycle-adjacent only. The updater's local
history may feed anonymous 30-day attempt, success, failure, and coarse failure
category counters, but those counters are adoption analytics, not proof that a
particular agent update, profile rollout, host command, registration, or fleet
operation succeeded. Lifecycle surfaces must keep reading update readiness and
continuity from the updater, installer, connection ledger, and agent runtime
state instead of inferring it from anonymous telemetry.

1. `frontend-modern/src/api/agentProfiles.ts` shared with `api-contracts`: the agent profiles frontend client is both an agent lifecycle control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/api/nodes.ts` shared with `api-contracts`: the shared Proxmox node client is both an agent lifecycle setup/install control surface and a canonical API payload contract boundary.
3. `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx` shared with `api-contracts`: the inline node credential slot is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
4. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx` shared with `api-contracts`: the pure infrastructure operations inventory/install model is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
5. `frontend-modern/src/components/Settings/MonitoredSystemImpactPreview.tsx` shared with `cloud-paid`: the monitored-system impact preview is both a platform-connections lifecycle surface and a canonical cloud-paid monitored-system presentation boundary.
6. `frontend-modern/src/components/Settings/NodeModalAuthenticationSection.tsx` shared with `api-contracts`: the node setup authentication section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
7. `frontend-modern/src/components/Settings/NodeModalBasicInfoSection.tsx` shared with `api-contracts`: the node setup basic-info section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
8. `frontend-modern/src/components/Settings/nodeModalModel.ts` shared with `api-contracts`: the pure node setup modal model is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
9. `frontend-modern/src/components/Settings/NodeModalMonitoringSection.tsx` shared with `api-contracts`: the node setup monitoring section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
10. `frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx` shared with `api-contracts`: the node setup guide section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
11. `frontend-modern/src/components/Settings/NodeModalStatusFooter.tsx` shared with `api-contracts`: the node setup status/footer section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
12. `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts` shared with `api-contracts`: the direct-node infrastructure settings state hook is both an agent lifecycle control surface and a shared Proxmox node API contract boundary.
13. `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts` shared with `api-contracts`: the infrastructure discovery runtime state hook is both an agent lifecycle control surface and a shared discovery/settings API contract boundary.
14. `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx` shared with `api-contracts`: the infrastructure install state hook is both an agent fleet lifecycle control surface and an API token, lookup, and install transport contract boundary.
15. `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx` shared with `api-contracts`: the shared infrastructure operations state hook is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
16. `frontend-modern/src/components/Settings/useNodeModalState.ts` shared with `api-contracts`: the node setup modal state hook is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
17. `frontend-modern/src/utils/agentInstallCommand.ts` shared with `api-contracts`: the shared frontend install-command helper is both an agent lifecycle control surface and a canonical API/install transport contract boundary.
    Operator-facing agent install commands must preflight the selected
    OS/architecture download before they mutate the target host, and generated
    commands must pass enrollment secrets through short-lived token files
    rather than long-lived service/environment arguments. Windows, macOS, and
    Linux commands must keep custom CA, insecure/plain-HTTP, and optional-auth
    behavior aligned so the Machines onboarding path does not diverge by OS.
18. `frontend-modern/src/utils/infrastructureSettingsPresentation.ts` shared with `api-contracts`: the infrastructure settings presentation helper is both an agent lifecycle control surface and an API-backed direct-node/discovery settings boundary.
19. `internal/api/agent_install_command_shared.go` shared with `api-contracts`: agent install command assembly is both an agent lifecycle control surface and a canonical API payload contract boundary.
    19a. `internal/api/cloud_agent_install_command.go` shared with `api-contracts`, `cloud-paid`: hosted tenant agent install command generation is both an agent lifecycle enrollment surface and a provider-hosted tenant boundary.
    The hosted PVE/PBS install command path must stay on the same token-file
    command transport as the normal lifecycle install helpers, while failing
    closed unless hosted mode is active, the target org exists, and the minted
    `agent:report` token remains scoped to that tenant workspace.
    Provider-hosted MSP report branding must not alter agent enrollment or
    ingest authority. Agents continue to report into one tenant Pulse runtime
    with tenant-local tokens; branding is report rendering configuration inside
    that runtime, not a control-plane token, agent profile, or cross-client
    ingest path.
20. `internal/api/config_setup_handlers.go` shared with `api-contracts`: auto-register and setup handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
21. `internal/api/setup_script_render.go` shared with `api-contracts`, `storage-recovery`: the generated Proxmox setup-script is a shared boundary across agent lifecycle (forced-command keys, install/uninstall edits), API contracts (rendered token shape and encoded rerun URL), and storage/recovery (backup visibility grants, Pulse-managed temperature SSH keys, and SMART disk-temperature collection).
    PBS setup-script auto-registration remains lifecycle-owned bootstrap
    transport: rendered scripts must post registration payloads to the canonical
    Pulse base URL plus `/api/auto-register`, not to the script download
    artifact URL, so install-time registration keeps one API root regardless of
    whether the script came from `/api/setup-script` or `/api/setup-script-url`.
22. `internal/api/unified_agent.go` shared with `api-contracts`: unified agent download and installer handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
23. `scripts/install.ps1` shared with `deployment-installability`: the Windows installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.
    The Windows installer must support a non-mutating download preflight that
    can run before Administrator-only install work, must accept token-file
    enrollment input, and must persist plain-HTTP/insecure runtime continuity
    consistently with the Unix installer. The installed Windows service must
    also expose the same local health/readiness server as foreground
    `pulse-agent` runs so installer "healthy" verification and post-install
    smoke checks prove a live agent runtime, not merely a running service
    wrapper.
24. `scripts/install.sh` shared with `deployment-installability`: the shell installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.

Server update planning is part of the same lifecycle contract. The System
Updates plan must surface a structured upgrade-readiness verdict before an
operator installs a v6 update, and the backend apply route must enforce a
`blocked` verdict before starting the updater: server update path, registered
agent continuity, v5 agent migration transport security, and agent reporting
token scope must be derived from the existing updater, connections ledger, and
API token state rather than from docs-only guidance or a parallel migration
registry. v5 or legacy agents that are still reporting must be treated as
compatible with the v6 reporting boundary, while still surfacing that their
first automatic hop uses the v5 updater before v6 signature and downloaded
binary self-test protections apply. Stale agents, missing reported versions,
missing agent reporting scope, or expired agent tokens must be called out
before the update starts. Root `install.sh` v5-to-v6 upgrades must also inspect
local token metadata before binary replacement and warn when the shell path
cannot prove agent-token continuity.

Stale-agent platform notices and the Infrastructure `agentUpdates` workspace
route are part of that same lifecycle contract. Platform pages may link to the
central Infrastructure update dialog with a scoped list of affected agent
connection IDs, but command rendering stays lifecycle-owned in Settings.
`currentAgentTargetVersion()` is the canonical agent update target and must be
projected to browser clients as `/api/version.agentUpdateTargetVersion`;
platform pages and Settings must not compare agent binaries against the app
build `version` when the server would tell agents `dev`. For Unix-like agents
that Pulse already sees, the copied stale-agent update command must use
`scripts/install.sh --update` and recover URL, token, identity, custom CA, and
insecure transport from installer-owned saved state instead of asking the
operator to mint a fresh install token or exposing agent IDs in the copied
command. Windows stale-agent update commands remain on the existing token-gated
install transport until the Windows installer owns an equivalent saved-state
update mode.
Agent Fleet Doctor diagnostics extend that same read-only lifecycle triage
surface: `GET /api/agents/diagnostics` may explain stale versions, missing
reports, profile deployment drift, expected Docker/Kubernetes telemetry gaps,
identity splits, and removed-agent blocks, and may advertise existing repair
handoffs such as copy-upgrade-command or allow-reenroll. It must not perform
the repair, create an action plan, or replace the canonical `/api/connections`
fleet projection used by Infrastructure.

Agent lifecycle and fleet-operation surfaces may consume
`POST /api/actions/plan` for resource capability planning, but the action plan
contract remains API-owned through `internal/api/actions.go` and
`internal/actionplanner/planner.go`. Agent lifecycle work must not define a
parallel approval policy, blast-radius model, stale-plan hash, or execution
contract for those resource actions. Successful action plans also belong to
the API-owned action-audit trail before any lifecycle surface consumes them:
approval-required plans must be visible as `pending_approval` with initial
lifecycle evidence, and retry/idempotency handling must not create duplicate
lifecycle events. Approval or rejection decisions for those plans must flow
through `POST /api/actions/{id}/decision`, which records API-owned audit and
lifecycle evidence only; lifecycle surfaces must not treat approval as
implicit command execution or define a parallel execution handoff. Assistant
handoffs that recover a live Patrol approval by finding ID are still AI/runtime
review context only; agent lifecycle surfaces must not treat that recovered
approval reference as an agent command grant or host-execution shortcut.
Assistant session-history reads from the shared AI endpoints are likewise
API-owned browser-safe projections: lifecycle surfaces, MCP adapters, and
agents must not depend on `reasoning_content`, raw `pulse_*` / `patrol_*`
tool-call prose, token accounting text, or assistant transcript prose as an
agent command grant, install-token disclosure, or host-execution shortcut.
Searchable Assistant session-list queries on `GET /api/ai/sessions` are the
same browser-safe history navigation projection; lifecycle surfaces may not
reinterpret search matches, handoff summaries, or message counts as agent
capability state, command authority, enrollment evidence, or install-token
availability. When a planned resource capability is actually executed from an
agent-lifecycle surface, that handoff must route through
`POST /api/actions/{id}/execute` so
the API-owned action audit records `executing` before dispatch and the
terminal execution result afterward. Dry-run-only plans remain planning evidence
only; lifecycle surfaces must not present them as executable, dispatch them
through agent-local command paths, or bypass the API fail-closed execution gate.
Docker / Podman container start, stop, and restart affordances follow that same
boundary: lifecycle UI may consume unified-resource capabilities for enabled or
disabled presentation, but execution must stay inside the API-owned action
executor wired from `internal/api/router.go` and must not shell out, SSH, or call
Docker / Podman from lifecycle-local code. That API-owned executor may resolve
the command WebSocket by Docker source ID or canonical Docker host name and may
mark its vetted container lifecycle dispatch as a trusted agent command after
the API action has entered execution; lifecycle surfaces still consume only the
resource payload, action readiness, and action-audit result rather than issuing
or approving command-agent grants themselves.
Proxmox VM and LXC lifecycle affordances follow the same adjacent boundary:
lifecycle and fleet surfaces may consume backend-advertised `start`,
`shutdown`, `reboot`, and `stop` capabilities and typed `actionReadiness`, but
the only execution path is the API-owned action executor that resolves a
connected Proxmox node command agent and records action audit plus verification.
Lifecycle surfaces must not run `qm` / `pct`, SSH to a node, call Proxmox
mutation APIs, or substitute a guest-local agent to perform VM/LXC lifecycle
control.
Disconnected command-agent state is also API-owned readiness: lifecycle
surfaces may reflect missing backend-advertised capabilities, but must not
reconnect, substitute, or directly address an agent to make a stale container
action executable. Backend resource payloads and plan-action readiness are the
only supported lifecycle signal for that state. When a resource payload carries
typed `actionReadiness`, lifecycle surfaces may display the reason but must not
treat it as reconnect authority or an alternate command grant.
Assistant session rename through `PATCH /api/ai/sessions/{id}` follows that
same browser-safe history boundary. Lifecycle surfaces, MCP adapters, and
agents may display the updated title as human navigation metadata, but they
must not treat a renamed title as command authorization, host identity,
enrollment state, capability evidence, or install-token disclosure.
The native Assistant surface-tool inventory at
`GET /api/ai/assistant/surface-tools` is also AI-runtime/API-contract metadata:
lifecycle surfaces may display which Assistant tools are available, but must
not reinterpret that list as agent enrollment state, command authority, install
token scope, or proof that a host can execute lifecycle actions.
Agent lifecycle consumers of `/api/agent/events` and
`/api/agent/resource-context/{id}` must also honor the shared API command
payload boundary: API tokens with monitoring/read scope receive
`commandRedacted:true` instead of raw approval, action, or verification command
text unless they also hold action execution scope. Lifecycle UI and agents may
use those redacted events as doorbells or status summaries, but they must fetch
governed detail through the approval/action surfaces and must not treat a
monitoring-readable event stream as command disclosure or execution authority.
Lifecycle surfaces that inspect agent event kinds must consume the shared
`internal/agentcapabilities` event vocabulary. Local lifecycle, MCP, or probe
copies of `finding.created`, `approval.pending`, `action.completed`,
`stream.connected`, or `heartbeat`, or local SSE parser semantics for those
events, are contract drift because event naming and stream framing are
API/AI-owned, not lifecycle-owned.
Lifecycle consumers of agent-surface failures must also consume the shared
`internal/agentcapabilities` error envelope; lifecycle work may branch on the
stable `error` code but must not define a local failure envelope or reinterpret
agent errors as install, enrollment, command, or update authority. Branchable
agent-surface codes exposed through shared `internal/api/` routes must be
referenced from `agentcapabilities.AgentErrCode*` constants so lifecycle UI,
Assistant handoffs, MCP adapters, and manifest declarations do not drift into
separate string vocabularies.
The resource-context endpoint's additive context sections are the same
read-only boundary: lifecycle consumers, MCP adapters, and external agents may
use their bounded facts, provenance, freshness, and explicit redaction metadata
to understand a resource, but those sections must not be expanded into
provider-local config, raw discovery command output, environment values,
secret-like metadata, install-token scope, or command authority.
Discovery context exposed by `/api/discovery` is similarly API-owned
observability context, not lifecycle command authority. Lifecycle and
infrastructure setup surfaces may display service versions, config paths,
ports, bind mounts, suggested URLs, and mock-mode examples from the Discovery
API, but they must not widen install tokens, agent profiles, or command grants
because a Discovery record exists. Mock-mode Discovery records are demo
payloads for the same API contract and must not become a lifecycle-local
fixture or an implicit permission to run commands.
Agent-facing provisioning capabilities declared by
`/api/agent/capabilities` are also API-owned projections over the canonical
node lifecycle and discovery APIs. Lifecycle and infrastructure onboarding
surfaces may use `discover_lan`, `list_nodes`, `add_node`, `update_node`,
`remove_node`, `test_node_credentials`, `test_node_connection`, and
`refresh_node_cluster_membership` to orchestrate source onboarding, but those
tools must preserve settings-scope auth, typed manifest schemas, and redacted
source reads. The manifest does not turn discovered hosts into command-agent
authority, install-token scope, fleet-command grants, or permission to bypass
human approval for multi-source imports.
Native provider resource types exposed through shared `internal/api/` resource
handlers are the same kind of read-only context for lifecycle surfaces. A
TrueNAS `network-share` resource may appear in resource pickers, connection
impact previews, or cross-organization sharing only through the API and
unified-resource contracts; lifecycle code must not treat that share as an
agent install target, a command-agent authority, or a reason to widen setup
tokens.
Source-specific backup artifact routes under shared `internal/api/`, including
`/api/backups/pve` and `/api/backups/pbs`, are also read-only monitoring and
storage/recovery evidence for lifecycle-adjacent surfaces. Lifecycle code may
display those backup facts as connection or workload context, but it must not
treat a backup artifact, datastore, namespace, owner, or verification record as
an agent enrollment candidate, install target, setup-token scope, credential
grant, or fleet-command authority.
Docker / Podman native inventory and Kubernetes native API inventory follow
that same lifecycle boundary. Images, volumes, networks, Swarm tasks,
Swarm nodes, Swarm secrets, Swarm configs, engine storage-usage buckets,
Kubernetes Services, controllers, EndpointSlices, NetworkPolicies, PV/PVC
records, StorageClasses, ConfigMaps, Secrets, ServiceAccounts, Roles,
ClusterRoles, RoleBindings, ClusterRoleBindings, ResourceQuotas,
LimitRanges, PodDisruptionBudgets, HorizontalPodAutoscalers, ingresses,
namespaces, and events may appear as API/unified-resource evidence, but
lifecycle surfaces must not treat those child inventory rows as agent
enrollment candidates, install targets, fleet command authorities, setup-token
scopes, credential grants, or command material. RBAC inventory in particular
reports summary counts only and never carries credentials, individual subject
identities, or full policy rules into the lifecycle surface. Podman libpod pod records are
not part of that lifecycle-visible inventory until a libpod-native collector
owns them explicitly; Docker-compatible Podman container labels remain runtime
metadata only.
VMware vSphere `network` resources follow that same lifecycle boundary. A
vCenter connection may project networks into platform tables, resource
pickers, Assistant context, or monitored-system previews through the shared
resource contract, but those rows are provider inventory facts only. Lifecycle
surfaces must not interpret a vSphere network as a host enrollment candidate,
Pulse Agent install target, fleet command authority, or setup-token scope. When
host-shaped records are coalesced for presentation, lifecycle consumers must
use the API/unified-resource presentation result and must not create their own
merge that bypasses registry-owned report exclusions.

The node setup modal boundary must keep guided setup and manual credential
submission separate. For new PVE/PBS setup, API Inventory and Host Telemetry
Agent setup modes are command-driven auto-registration paths; Token ID/Value
fields, Test Connection, and Add Node submission belong only to Manual Token
Setup or existing-node edit flows.
The setup guide must also present the source strategy at action time: API
Inventory is the recommended least-privilege API path, Host Telemetry Agent is
the optional full-host-telemetry root-agent path, and Manual Token Setup is an
advanced manual API-token escape hatch.
For PVE, that same setup guide must keep Docker inside Proxmox LXCs attached
to the Host Telemetry Agent path, not the default API Inventory path: API
Inventory alone cannot run host-side guest Docker inventory, while the PVE Host
Telemetry Agent command must explicitly request Pulse command execution and
explain the separate server-side `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY`
opt-in.
For existing Proxmox API sources, that same setup guide must expose the
non-destructive Audit/Repair rerun path before asking operators to rotate
credentials. Audit/Repair is the first lifecycle response to suspected setup
drift: it checks the Pulse-managed user, current token presence/expiry,
expected ACLs, and older Pulse-managed tokens without replacing the stored API
secret. Full Install/Configure remains the explicit token-rotation path when
the Proxmox-side token value is gone or no longer matches Pulse.

That shared monitored-system impact preview boundary also owns the disabled
platform-connection lifecycle state. Once a TrueNAS or VMware setup form marks
the connection disabled, lifecycle surfaces must treat a canonical zero-delta
or removal-only preview as a valid save path instead of holding the dialog in
an add-only posture.
Agentless availability targets no longer belong to the infrastructure
source-management surface. MQTT, HTTP/S, ping, and TCP checks are monitoring
availability resources owned by the Settings Monitoring availability panel and
its model, while infrastructure remains limited to platform API connections,
agent-backed systems, discovery, and install lifecycle. Lifecycle UI may
observe availability rows only through shared API/resource facts; it must not
offer them from Add infrastructure, Manage, or the host connections ledger, and
must not ask them for SSH, setup tokens, auto-registration, agent profiles, or
install commands. Their managed actions remain availability-resource actions:
pause, test, edit, and remove.
An availability target classified as `machine` is still agentless unless a
Pulse Agent actually registers for that host. That classification exists so
servers, desktops, laptops, and comparable computers such as a Mac mini can
use machine-specific reachability copy in Availability checks; it must not
create Standalone Machines membership, lifecycle authority, install state,
agent profile membership, or host-command capability. A `targetKind=machine`
add-dialog handoff is only a preselected availability-resource creation path,
not an agent install or lifecycle onboarding state.
The lifecycle-owned onboarding presentation helper must consume the governed
platform support manifest for readiness stage, primary mode, canonical
projections, and support-floor posture.
`frontend-modern/src/utils/infrastructureOnboardingPresentation.ts` may adapt
those facts into source-strategy copy, but it must not turn an admitted
`first-lab-ready` platform such as VMware into a product-level supported
claim, invent platform-local projections, or classify assistant control beyond
the manifest's support-floor row. Admitted API platforms may remain available
inside the add-flow source picker and source-manager summaries, but their
operator-facing badge must be preview/admission language and empty-state copy
must keep them separate from the supported-source list until the governed live
proof promotes the platform.
That same lifecycle-owned helper must keep first-class platform APIs separate
from governed Pulse Agent host profiles. Host/appliance compatibility such as
Unraid is presented as an agent install/profile path sourced from the manifest
`agent_host_profiles` section, not as `PLATFORM_TYPE_KEYS` membership or a
peer API-backed platform.
The add-infrastructure picker must preserve that boundary while presenting a
plain operator-facing system finder: users choose recognizable infrastructure
systems such as Unraid, TrueNAS, Proxmox, Docker, Kubernetes, or standalone
hosts, and the helper maps those choices into API-backed or agent-backed routes
only after selection. The first picker surface must support aliases for common
operator terms and must not make API/agent taxonomy the primary choice model.
Agentless MQTT, HTTP/S, ping, and TCP probes are not infrastructure choices;
their add flow belongs to the Settings Monitoring availability owner.
Agent-backed typed routes must keep that context in the installer: Unraid,
Docker, Kubernetes, and generic host choices may share the same unified agent
installer, but their visible title, recommendation copy, preferred install
profile, and first command section must match the selected system instead of
falling back to an all-platform generic agent screen.
The generic host installer route is `?add=linux-host`: setup completion,
Proxmox "Install agent" shortcuts, detect-flow agent handoffs, and first-run
test helpers must target that typed route so the operator sees the standalone
Linux/macOS/Windows host context. The older `?add=agent` value may remain a
compatibility input, but it is not the canonical destination for new handoffs.
Agent runtime normalization must use that same governed host-profile manifest:
profile identity tokens and runtime platform fallback values such as
`unraid` -> `linux` are generated into the runtime resolver instead of being
redeclared in host-agent or settings-table branches. Host-profile detection is
an identity fact and must not depend on optional storage probes succeeding; an
Unraid host still reports the governed `unraid` profile and `linux` runtime
platform when `mdcmd` or array-topology collection is unavailable.
Unraid array collection belongs to that same runtime-normalized agent path:
`internal/hostagent/unraid.go` must treat empty `DISK_NP`/`DISK_NP_DSBL`
slots with no device, id, filesystem, or size as unassigned topology
placeholders rather than failed disks. Assigned disks may use `diskId`/`rdevId`
as the serial fallback when Unraid does not expose a separate serial field, so
monitoring receives stable disk identity without inventing host-profile or
platform state from optional storage probe success.
That Unraid runtime path must also prefer native appliance topology over
generic block-device inference. The Unified Agent should best-effort merge
`/var/local/emhttp/disks.ini` into the `mdcmd status` view and carry disk
device, model, transport, filesystem, size, used/free capacity, temperature,
spin state, read/write counters, and error counters in the report contract.
Failure to read the native file must degrade to the existing mdcmd view without
blocking host reporting, but successful native collection is the canonical
source for Unraid array/cache membership; SMART rows are supplemental hardware
telemetry, not the owner of Unraid storage topology.
First-class platform hosts that also run the Pulse Agent must keep the same
operator-facing system identity split: a Proxmox VE node may report a Debian
runtime platform underneath, but the host-agent OS identity and infrastructure
System badge must resolve and present `Proxmox VE` / `PVE` from PVE runtime
evidence instead of exposing the Debian base distro as the primary system label.
The agent-side PVE version probe must prefer fast package metadata such as the
installed `pve-manager` package before falling back to `pveversion`, and that
fallback budget must tolerate slower ARM/Pi PVE installs so platform identity
does not lose the runtime version on small lab nodes.
The lifecycle-owned infrastructure source manager also owns platform/system
grouping as source-management content, but not its table band presentation:
`frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx` must
route table-level product/system group rows through the shared grouped table row
helper instead of carrying lifecycle-local background or hover classes. Its
desktop source-management table also inherits the surrounding settings panel
frame and the shared `Table` shell from `frontend-primitives`; lifecycle work
must not restore a local `overflow-x-auto` wrapper or a page-local table card
around that table.
The same source-manager landing surface must keep onboarding primary and
compact: `Add infrastructure` remains the obvious action, the setup summary is
one short status strip, and detailed governance/fleet state stays in the
systems table rows or deeper fleet surfaces instead of expanding into duplicate
explanatory bands above the table.
Network discovery is the exception because manual scans are an operator command
whose progress must be observable in the source manager. `InfrastructureSourceManager.tsx`
must keep one explicit Network discovery band above the setup summary that
shows the current enabled/scanning/error/result state, scan scope, last scan
metadata, and review action for discovered candidates before anything is added.
Its copy must describe only the platform APIs represented by the LAN discovery
candidate model, and it must not imply TrueNAS, VMware, Docker, Kubernetes, or
agent command discovery when this source-manager path cannot surface those
candidates.
The `DiscoverySettingsForm.tsx` scan-scope selector must make automatic versus
custom subnet selection a full-row operator control, not a narrow or hidden
input target; common subnet chips and the custom subnet field must share the
same environment-override and in-flight-save lock semantics as the scan-scope
selector.
The discovery settings safety notice may use the frontend-primitives-owned
`CalloutCard` shell, but the lifecycle contract still owns the operator-facing
scan-scope semantics: environment overrides, immediate persistence, automatic
interface scan breadth, and custom-subnet guidance must remain visible before
the selector rather than becoming feature-local styling copy.
Adjacent Assistant resource-context handling in `internal/api/ai_handler.go`
may reference agent-backed resources only as selected-resource, model-only
context. It must not mutate agent lifecycle state, start agent discovery, or
grant agent command authority unless the request flows through the governed
agent/action execution contract.
Reported agent-backed host profiles must be visible at the source-manager
grouping level without changing the canonical connection type: an Unraid agent
row stays `agent` internally, but the connected-systems table groups it under
the governed `Unraid` host-profile family and routes that group's add action
back to the typed Unraid onboarding flow. Discovered source candidates remain
attached to the base source group until a configured agent reports a governed
host profile; discovery must not be visually inherited by a profiled group.
The source-management ledger must keep source-level health and per-member
agent posture separate. Cluster parent rows may summarize the primary source
and their rendered members, but attached-agent fleet warnings that are already
represented by member rows must not be duplicated onto the cluster parent.
Passive agent config or rollout handshakes whose only cause is a missing
comparable applied agent configuration fingerprint are not operator-actionable
setup attention and must not be counted as a visible infrastructure problem
unless a row or member still renders them as an actionable highlight.
The API-owned distinction is important: an unmanaged default agent config is
not a rollout at all, while a managed desired config that lacks an applied
fingerprint is rollout attention until the agent report proves convergence.
Configured-node settings tables follow the same boundary:
`frontend-modern/src/components/Settings/ConfiguredNodeTables.tsx` may own node,
credential, capability, status, and action cells, but table scroll framing must
stay on the shared `Table` primitive. If a configured-node table needs bounded
vertical height, apply it through `Table` `wrapperClass` instead of an outer
lifecycle-local scroll container.
Agent profile management tables follow that same presentation boundary:
`frontend-modern/src/components/Settings/AgentProfilesPanel.tsx` may own agent
profile and assignment columns, but embedded table framing must route through
`PulseDataGrid`'s shared frame variants instead of lifecycle-local
`overflow-x-auto` or side-border wrappers.
Agent profile prompt and description textareas follow the same shared primitive
boundary. `AgentProfilesPanel.tsx` and `SuggestProfileModal.tsx` may own agent
profile lifecycle behavior and API mutations, but multi-line form controls in
those surfaces must compose `FormTextarea` instead of local raw `<textarea>`
wrappers.
Agent profile read-only state indicators follow the same presentation split:
`AgentProfilesPanel.tsx` may own lifecycle status labels and assignment state,
but badge tone, optional dot wiring, sizing, and shell classes must compose the
frontend-primitives-owned `StatusIndicatorBadge` instead of calling status tone
helpers or recreating badge spans inline.
Agent profile commercial gates follow the same primitive split:
`AgentProfilesPanel.tsx` and `useAgentProfilesPanelState.ts` may own lifecycle
entitlement checks, upgrade destinations, and label selection, but the
button-styled upgrade action must compose frontend-primitives'
`UpgradeButtonLink` instead of exporting or applying a lifecycle-local CTA
class helper.
Agent profile loading indicators follow that same split:
`AgentProfilesPanel.tsx` may own license-checking, profile-loading, and empty
profile semantics, but spinner shell, size, tone, and accessible status
behavior must compose frontend-primitives' `LoadingSpinner` instead of
lifecycle-local `animate-spin` border classes.
Agent profile empty states follow that same split:
`AgentProfilesPanel.tsx` may own profile and assignment empty-state copy and
lifecycle conditions, but compact empty-state spacing, icon treatment, and text
hierarchy must compose frontend-primitives' `EmptyState` `variant="panel"`
instead of lifecycle-local centered icon/text shells.

## Extension Points

Manual scoped Patrol work that reaches `internal/api/ai_handlers.go` (such as an
alert-initiated targeted Patrol check via `POST /api/ai/patrol/run`) is
investigation-only over agent-reporting resources: it must not alter agent
lifecycle, registration, token binding, reporting contracts, or install-command
identity. The scoped Patrol request carries resource identity only and reuses
the existing Patrol scoped engine, so it adds no agent lifecycle control
surface and no new `internal/api/` lifecycle handler.

1. Add or change install-command generation, canonical /api/auto-register behavior, or installer download behavior through the owned `internal/api/` files above.
   Canonical `/api/auto-register` auth is split by intent: when the setup-token
   bootstrap path succeeds, lifecycle clients must complete registration with
   the one-time `authToken` only and must not also send the long-lived
   `X-API-Token` header. The header-backed `agent:report` fallback exists only
   for update-only re-registration when setup-token fetch is unavailable.
   Canonical auto-register WebSocket semantics must distinguish first-time node
   creation from idempotent existing-node refreshes: only newly created PVE/PBS
   nodes may emit `node_auto_registered`, while matched existing nodes that
   rotate or refresh credentials must emit a non-toast configuration refresh
   such as `nodes_changed`.
   Browser consumers must treat `node_auto_registered` as a real-time,
   timestamped lifecycle event rather than durable infrastructure state:
   fresh first-time events may show one success toast, but replayed, stale, or
   duplicate events must refresh configuration silently so an old registration
   cannot present as a newly connected node. The event payload must preserve
   the canonical registration `source`; script-initiated first-time
   registrations are the only source that may display the operator-facing
   success toast, while background agent registrations must update the ledger
   without claiming the operator just added that node.
   Shared `internal/api/` session and auth changes consumed by lifecycle
   routes must preserve durable principal IDs as the authorization key. Agent
   lifecycle surfaces may display contact email when supplied by the shared
   auth boundary, but they must not reinterpret SSO or Stripe email as the
   canonical user identifier for setup, install, or fleet-management actions.
   Approved-action tool invocation parsing in `internal/api/router_routes_ai_relay.go`
   is not an agent-lifecycle route grammar: lifecycle-adjacent setup or repair
   flows that execute governed Pulse tools must inherit
   `internal/agentcapabilities.ParseTextToolInvocation` through the API relay
   instead of duplicating `pulse_*` command-string parsing in lifecycle helpers.
   Approved-action replay through that same relay must also use the shared
   `internal/agentcapabilities` approved-action argument helper, not lifecycle-local
   approval argument keys or pass-through maps.
   Native Assistant workflow-prompt rendering through
   `POST /api/ai/workflow-prompts/render` is likewise AI-runtime/API-contract
   ownership even though it is wired through shared `internal/api/` files:
   lifecycle surfaces may benefit from the resulting Assistant prompt text, but
   they must not treat manifest `workflowPrompts`, rendered prompt bodies, or
   Assistant starter availability as install, enrollment, or fleet-control
   route grammar.
   AI provider registry and `/api/settings/ai` credential-shape changes in
   `internal/api/ai_handlers.go` are likewise adjacent AI-runtime/API-contract
   ownership. Agent lifecycle may depend on the shared authenticated API
   substrate staying intact, but provider ids, provider default endpoints,
   model routes, API-key fields, and provider configured state are not agent
   install, enrollment, reporting, or fleet-control lifecycle inputs.
   The adjacent workflow-starter activity markers on
   `POST /api/ai/workflow-prompts/activity` and
   `POST /api/agent/workflow-prompt-activity` follow that same split:
   lifecycle surfaces may use the aggregate activation reports, but the
   content-free prompt-name/surface/timestamp marker is not agent enrollment,
   install readiness, fleet-control liveness, command authority, or agent
   update state. The first-party `pulse_patrol` starter surface means a user
   entered the Pulse Intelligence operations-loop journey from app UX; it does
   not imply a host agent is installed, healthy, or authorized to run commands.
   The first-party `patrol_control` starter surface and legacy
   `patrol_autonomy` / `pulse_pro_activation` aliases are paid Patrol control
   journey markers only; lifecycle code must not treat any of them as setup,
   install, fleet-health, or command-agent state.
   The adjacent `GET /api/agent/patrol-control/status` projection in
   `internal/api/agent_resource_context.go` may expose operator-readable Patrol
   status copy for native and external-agent orchestration, but lifecycle
   surfaces must not reinterpret that copy, its next action, or its
   completed/resolved Patrol outcome counts as install readiness, enrollment
   health, command authorization, or agent update state.
   Hosted Pulse Account may deep-link an MSP operator into a tenant workspace's
   agent-install surface through a signed local handoff target such as
   `/settings/infrastructure?add=linux-host`, but install-token generation,
   installer command construction, and first-agent onboarding remain
   agent-lifecycle/API-owned inside the tenant runtime. Control-plane handoff
   code must not mint lifecycle tokens or synthesize install commands for the
   tenant; it may only establish the tenant session and local destination.
   Availability-target API changes are adjacent but not lifecycle-owned:
   agentless MQTT, HTTP/S, ping, and TCP targets may appear in shared
   `internal/api/` handlers and resource projections, but their settings owner
   is `frontend-modern/src/components/Settings/AvailabilitySettingsPanel.tsx`
   with `frontend-modern/src/components/Settings/availabilitySettingsModel.ts`.
   They must remain settings/API availability resources and must not create
   install commands, agent tokens, host uninstall actions, stop-monitoring
   lifecycle actions, or infrastructure add-flow states. The `machine`
   target-kind label may make an agentless server, desktop, laptop, or Mac mini
   visible in Standalone Machines, but lifecycle code must still treat it as
   reachability evidence rather than an enrolled host.
   Global resource timeline API changes are likewise adjacent when they touch
   shared `internal/api/` route wiring: `/api/resources/timeline` may expose
   monitoring-read provider activity for platform pages, but it must not create
   setup-token authority, install-command state, fleet command execution, or
   agent lifecycle enrollment semantics.
   Patrol autonomy API changes are likewise adjacent when they touch shared
   `internal/api/` route wiring: monitor-mode configuration and remediation
   entitlement payloads remain AI runtime/API-contract owned and must not create
   agent lifecycle authority, install-token scope, or fleet command semantics.
   Clearing stale Patrol full-mode unlock state during monitor-only saves is
   also an AI runtime entitlement clamp, not a lifecycle enrollment or command
   authority signal.
   Patrol readiness and settings-save payload changes on those shared handlers
   are also adjacent only: structured provider/model/tool causes may be exposed
   to Patrol and Assistant, but they do not grant agent install, enrollment,
   or fleet command authority. Assistant runtime identity strings exposed by
   those handlers must name the first-party surface as Pulse Assistant rather
   than the legacy generic `Pulse AI`, and lifecycle consumers must not treat
   that label as agent registration, install progress, or command-agent
   readiness state. Diagnostics from `internal/api/diagnostics.go` follow the
   same adjacent boundary: lifecycle consumers may read
   `assistantRuntimeConnected`, but they must not reinterpret it as MCP
   transport state, agent registration, install progress, or command-agent
   readiness.
   Profile-suggestion availability errors consumed through
   `frontend-modern/src/api/agentProfiles.ts` are shared Pulse Intelligence
   availability guidance, not lifecycle registration state; the client must
   point operators at Assistant & Patrol settings instead of reviving `Pulse
   Assistant settings` copy.
   Hosted handoff subjects consumed through the shared API auth boundary must
   already be stable, non-email principals; lifecycle-adjacent routes must not
   recover authority from a blank handoff subject by falling back to contact
   email.
   The same rule applies to hosted public magic-link sessions consumed by
   lifecycle-adjacent routes: shared auth must mint a browser session only for
   a stored organization principal, not for a contact email on a blank
   owner/member row.
   Checkout webhook magic-link delivery follows that same shared-auth
   dependency: lifecycle-adjacent consumers may observe billing activation
   after server-owned org linkage, but they must not treat Stripe contact email
   as fleet authority unless the API-owned organization resolver maps it to a
   stored owner/member principal first.
   Runtime org authorization consumed by lifecycle-adjacent setup and fleet
   routes must use strict `OwnerUserID`/member `UserID` membership checks; the
   API-owned email-aware organization helpers are migration/delivery helpers
   only and must not let contact email become setup, install, or fleet
   authority after a stable principal exists.
   API-token owner metadata follows the same rule: lifecycle-adjacent setup or
   mobile-pairing token flows may consume the shared token helper, but they
   must not pass extension metadata that authors or overwrites `owner_user_id`.
   Agent install command and deploy bootstrap/enrollment tokens must derive
   owner identity from the API-authenticated caller through the shared
   server-side token owner helper; bootstrap deploy metadata may bind cluster,
   job, target, and expected node, but not the human owner.
   Command-agent WebSocket identity follows the same lifecycle/auth split:
   install-command tokens that enable command execution may be minted before
   the final agent ID is known, but only Pulse-minted PVE/PBS install-command
   tokens may bind on first `/api/agent/ws` registration. That first-use bind
   must persist the registering agent ID and hostname and become authoritative
   for later command registration attempts; generic unbound `agent:exec`
   tokens are not lifecycle credentials and must fail closed.
   The canonical durable-principal vocabulary for those shared auth routes is
   recorded in `docs/release-control/v6/internal/IDENTITY_INVARIANTS.md`; agent
   lifecycle work may consume that identity but must not define a parallel
   email-keyed actor model for fleet operations. SSO-authenticated lifecycle
   callers must already arrive with the API-owned provider-scoped principal;
   lifecycle code must not recover authority from SSO email or display claims.
   That same lifecycle-owned setup path also owns script teardown behavior:
   rerunning the governed setup script in remove mode must call canonical
   `/api/auto-unregister` with `source:"script"` before local credentials are
   deleted, and the short recent-setup-token grace window must remain valid for
   that follow-up uninstall so operators can immediately back out a reviewed
   script-managed Proxmox setup without leaving a stale node record in Pulse.
   That same lifecycle-owned setup path also owns Proxmox `authorized_keys`
   symlink preservation for temperature-monitoring SSH keys: generated PVE
   setup scripts must resolve the real authorized-keys target before filtering
   Pulse-managed `# pulse-` lines during install or removal.
   That same shared `internal/api/` adjacency does not transfer ownership of
   AI provider setup or mobile Patrol-provider bridging:
   `internal/api/ai_handlers.go` and `internal/api/chat_service_adapter.go`
   may be invoked by lifecycle-adjacent surfaces, but AI runtime auth and
   provider selection remain `ai-runtime` plus `api-contracts` concerns rather
   than install-token or lifecycle credential state. Legacy Anthropic OAuth
   fields exposed by AI settings are cleanup-only compatibility state:
   lifecycle-adjacent setup, install, profile, and relay flows must not treat
   stored OAuth tokens or `auth_method=oauth` as an agent enrollment, provider
   bootstrap, or AI provider-readiness signal. API-facing Assistant chat tool
   calls projected through `internal/api/chat_service_adapter.go` must stay
   on the shared `agentcapabilities` provider-call shape; lifecycle consumers
   must not treat Assistant transcript tool-call IDs, inputs, output, success
   flags, or provider continuation metadata as enrollment, assignment, installer,
   or fleet-command evidence. Lifecycle flows must not
   recreate the retired Patrol quickstart bootstrap path, mint server-issued
   hosted-model tokens, or derive AI provider state from installation identity.
   Per-request `/api/ai/chat` execution-mode overrides follow that same
   boundary: lifecycle-adjacent consumers may rely on Assistant approval
   semantics, but scoped `autonomous_mode:false` chat requests must not be
   reinterpreted as agent registration, assignment, installer, or connection
   lifecycle state. Patrol finding handoffs that force approval-required mode
   from a non-empty `finding_id` are likewise AI/runtime governance, not an
   agent command grant or lifecycle authorization primitive.
   Patrol queued-fix approvals may now seed the shared action-audit store with
   planned and pending lifecycle evidence, but lifecycle surfaces must treat
   those records and their persisted requester identity as action-governance
   context only. Backend-refreshed Assistant handoffs may also carry that
   requester identity as provenance, but it still does not create install,
   enrollment, assignment, or fleet-command authority.
   Lifecycle flows must not reintroduce anonymous bootstrap identity,
   tenant-local commercial-owner surrogates, or fake activation records when
   they traverse those shared handlers. They also must not infer tenant
   creation, email issuance, or public-route availability from
   `/api/public/signup` response codes or payload fields just because that
   commercial route lives under the shared `internal/api/` tree.
   That same retired quickstart boundary is vendor-neutral at the lifecycle
   edge too: lifecycle-adjacent consumers may observe old `quickstart:*`
   values only as compatibility data being cleared by shared settings helpers.
   They must not bake vendor model IDs or provider-model fallback rules into
   install or activation flows just because those routes share the backend API
   tree.
   Lifecycle-adjacent resource reads that traverse `internal/api/resources.go`
   must also preserve the canonical unified-resource `name -> type -> id`
   order instead of inheriting map order or page-local re-sorts, so install
   and runtime hydration do not present one resource ordering at first load and
   a different ordering after the first live refresh.
   Those same resource-list reads must preserve browser-encoded comma-separated
   `/api/resources` type filters. Lifecycle-adjacent consumers that request
   agent or Docker host resource rows through standard query builders must
   receive the same canonical resource set as literal comma filters instead of
   treating `%2C` as part of a resource type token.
   Those lifecycle-adjacent reads may observe resource API
   `policyPosture` aggregation as read-only data-governance context, but they
   must not reinterpret sensitivity, routing, or redaction counts as install
   capacity, registration eligibility, or agent assignment state.
   Relay mobile credential issuance is not an agent bootstrap or lifecycle
   repair path just because it lives under shared `internal/api/` routing.
   `POST /api/security/tokens/relay-mobile` must remain API/security owned,
   must require the paid `relay` entitlement before token minting, and must not
   be reused by installer, auto-registration, assignment, or repair flows as an
   alternate setup credential.
   When lifecycle surfaces also hydrate from `/api/state`, that first-session
   snapshot must carry the same canonical resource types and display names as
   `/api/resources` instead of briefly showing legacy host aliases before the
   first websocket-backed refresh lands.
   Chart-adjacent lifecycle reads in shared `internal/api/router.go` must obey
   that same mock-aware unified snapshot boundary: demo `/api/charts` and
   `/api/charts/infrastructure` payloads may not bypass
   `GetUnifiedReadStateOrSnapshot()` and silently drop VMware-backed host rows
   that the canonical mock estate already published through `/api/state` and
   `/api/resources`.
   Persisted legacy hosted quickstart model IDs are therefore not lifecycle
   truth either: when shared settings helpers load or save historical
   quickstart values, they must clear them before adjacent install or
   activation flows read the payload. Tenant-local lifecycle routes may reuse
   shared installation activation or effective entitlement billing state for
   their own governed purposes, but they must not fork per-org activation
   caches, alternate installation-token stores, synthetic entitlement mirrors,
   or competing AI-provider identity. That same shared AI/runtime boundary owns
   Patrol execution identity: lifecycle-adjacent flows may trigger or observe
   Patrol runs through `internal/api/chat_service_adapter.go`, but they must
   preserve the stable execution identifier that describes one higher-level
   Patrol run across agentic turns.
2. Add or change update continuity and persisted-version handoff through `internal/agentupdate/`.
3. Add or change runtime-side Unified Agent startup, first-report assembly, and enroll/runtime continuity through `internal/hostagent/`.
   Proxmox host-agent setup must treat local `proxmox-registered` markers as a cache, not authority: before skipping token setup or node repair, `internal/hostagent/proxmox_setup.go` must revalidate the current type and candidate hosts against Pulse through the canonical auto-register contract.
   Runtime-side PVE token setup must also keep the same permission shape as the
   generated setup script: Pulse-managed PVE monitor tokens are
   privilege-separated, and `PVEAuditor`, optional `PulseMonitor`, plus
   `/storage` `PVEDatastoreAdmin` grants must be mirrored to both
   `pulse-monitor@pve` and the concrete `pulse-monitor@pve!pulse-*` token id.
   Runtime startup must preserve the token-optional path for already-installed
   non-enrollment agents; only enrollment mode may fail CLI configuration
   loading solely because no API token was supplied.
   That runtime-side ownership includes local disk telemetry collection in
   `internal/hostagent/smartctl.go`. Linux SMART discovery must prefer
   `smartctl --scan-open` typed targets before generic block-device fallback so
   controller-backed disks keep their canonical SMART and wearout coverage.
   FreeBSD SMART probing must retry through the canonical typed and untyped
   device modes and the SCT temperature status path before settling on standby
   or no-data results, and partial or plain-text smartctl output must still
   preserve model, serial, health, and temperature data through the same
   host-agent runtime boundary instead of leaving monitoring to guess.
   macOS thermal telemetry belongs in the same host-agent sensor contract, but
   it must report Darwin `pmset` thermal and performance pressure as
   `sensors.thermalState` instead of inventing Celsius readings from unavailable
   Apple silicon sensor values.
   Runtime RAID collection uses `/proc/mdstat` as the canonical discovery
   baseline for Linux md arrays. `mdadm --detail` may enrich level, state,
   member, UUID, and rebuild fields when available, but missing or failing
   mdadm detail probes must not hide a kernel-reported md array from the
   unified agent report.
   Server-side lifecycle admission must preserve that same continuity across
   Pulse restart and upgrade. When a standalone host comes back with the same
   durable machine/report identity and token continuity, the shared admission
   path must treat it as the existing system until live inventory rebuild
   catches up, rather than as a fresh enrolment competing for another
   monitored-system slot.
4. Keep shared agent-side TLS identity fail-closed across `cmd/pulse-agent/main.go`, `internal/hostagent/`, `internal/agentupdate/`, `internal/remoteconfig/client.go`, and `internal/agenttls/config.go`. Self-signed deployments may use a canonical pinned Pulse server certificate fingerprint, but lifecycle transport must route that pin through reporting, enrollment, command websocket, remote-config, and self-update clients instead of widening `PULSE_INSECURE_SKIP_VERIFY` into a blanket MITM carve-out. A configured custom CA bundle is part of that same trust boundary: if the bundle is unreadable or invalid, lifecycle transport must refuse the connection path rather than silently downgrading back to system roots.
5. Keep release-grade updater trust fail-closed across `internal/agentupdate/`, `internal/dockeragent/`, and the shared `internal/api/unified_agent.go` download helpers. When release builds embed trusted update signing keys, published agent binaries and installer assets must carry detached `.sig` plus `.sshsig` sidecars; updater/runtime paths must require `X-Signature-Ed25519` in addition to `X-Checksum-Sha256`, and installer-owned download flows must require the matching base64-encoded `X-Signature-SSHSIG`, instead of silently downgrading to checksum-only trust.
6. Keep shared `internal/api/` helper edits isolated from agent lifecycle semantics: Patrol-specific status transport or alert-trigger wiring changes in shared handlers must not bleed into auto-register, installer, or fleet-control behavior unless this contract moves in the same slice.
   The same isolation rule applies to AI settings payload work in `internal/api/ai_handlers.go`: provider auth fields, masked-secret echoes, provider-test model selection, and legacy Anthropic OAuth cleanup fields remain AI/runtime plus API-contract ownership and must not be reinterpreted as lifecycle setup, provider activation, or registration semantics just because they share backend helper layers.
   Patrol readiness labels on the same settings payload, including the
   user-facing Patrol control label for the stable `configuration` check ID,
   remain AI/runtime plus API-contract wording and must not be treated as agent
   lifecycle configuration, install readiness, registration state, or fleet
   control setup.
   Governed AI action-target normalization in
   `internal/api/ai_handlers.go` and `internal/api/ai_resource_types.go`
   follows that same split: Assistant approval routing may map resource types
   to action targets, but lifecycle setup, enrollment, installer, and command
   websocket target semantics remain owned by the lifecycle contracts above.
   The same isolation rule applies to Pulse Assistant chat SSE progress in
   `internal/api/ai_handler.go`: neutral `workflow_state` transport liveness
   such as `stream_idle`, provider startup, retry, fallback, and model-thinking
   status remains AI/runtime plus API-contract ownership and must not be
   reinterpreted as agent enrollment, install progress, command websocket
   liveness, or fleet-control freshness.
   Native Assistant workflow-prompt rendering through
   `POST /api/ai/workflow-prompts/render` follows the same split: manifest-owned
   prompt names, argument validation, and rendered starter text remain
   AI/runtime plus API-contract ownership and must not become lifecycle setup,
   enrollment, installer, or fleet-control state just because the route lives in
   `internal/api/ai_handler.go`.
   Workflow-starter activity telemetry recorded after successful Assistant
   prompt rendering, first-party Patrol activation handoff, Pro activation
   handoff, or Pulse MCP prompt rendering follows the same ownership split. The
   marker may contribute to Pulse Intelligence activation reporting, but
   lifecycle code must not treat it as proof of installed agents, reported
   hosts, command connectivity, stale-agent freshness, or fleet-operation
   capability.
   The same isolation rule applies to report branding validation and rendering
   request assembly in `internal/api/system_settings.go` and
   `internal/api/metrics_reporting_handlers.go`: lifecycle-owned install,
   enrollment, and reporting freshness flows may coexist with generated
   reports, but workspace logo material remains API/security/reporting
   ownership and must not become agent credential, install-token, or fleet
   lifecycle state.
   The same isolation rule applies to Patrol investigation-record propagation
   through shared AI intelligence handlers and `internal/api/router.go`:
   lifecycle surfaces may observe the resulting resource context, but they must
   not reinterpret `investigation_record` as agent enrollment, installer,
   command policy, or fleet-control authority. The same isolation rule applies
   to operator-facing finding fields like `impact` and `recommendation`
   propagated through that router conversion: lifecycle surfaces may render
   them in finding context but must not treat them as enrollment, installer,
   or fleet-control authority for the named resource. The same scope applies
   to the `previous_resolved_fix_summary` operational-memory field carried on
   findings across regressions: lifecycle surfaces may render it in finding
   context but must not replay the recorded fix description as an installer
   command, enrollment action, or fleet-control instruction. The same
   scope applies to the `trust` block on the patrol-status response
   (`FindingsTrustSummary`): lifecycle surfaces may read it as adjacent
   operator context but must not derive enrollment, installer, or
   fleet-control authority from any of its counters. Assistant chat enrichment from
   `finding_id` follows the same rule: lifecycle-owned command execution and
   agent auto-approval policy stay canonical in the agent/runtime owners, not
   in Patrol investigation-record prompt text. Model-only Assistant handoff
   context for a Patrol finding, including same-session metadata retained for
   follow-up turns and any resolved-resource scope hydrated from that finding,
   is also not agent lifecycle state and must not be used as enrollment
   evidence, command-websocket identity, or installer authority. If Assistant
   stores the originating finding ID to refresh the current unified finding and
   investigation record on follow-up turns, that stored reference remains an
   AI/runtime context selector and still cannot become agent enrollment,
   lifecycle, or command-websocket authority. Clearing that stored handoff when
   the finding no longer resolves is also AI/runtime invalidation, not an agent
   lifecycle transition. Unified finding lifecycle facts, latest lifecycle
   event briefing lines, and detailed lifecycle context included in Assistant
   handoff context remain Patrol/AI review metadata and must not become agent
   lifecycle state or fleet-control authority. Primary finding recency,
   evidence, verification, and governed action artifact facts in the finding
   briefing and related root-cause or correlated finding summaries resolved for
   the same Assistant handoff follow that same isolation rule, including any
   recency or latest lifecycle facts attached to those related summaries: they
   may explain Patrol context to the model and seed model-only handoff
   resources, but they are not enrollment evidence,
   command-websocket identity, agent command grants, or auto-approval policy.
   Structured Assistant handoff action references from the same Patrol finding
   remain AI/runtime review metadata only; lifecycle code must not treat
   approval IDs, action IDs, fix IDs, action lifecycle state, risk, or
   target-resource labels from that handoff as agent command grants, enrollment
   proof, or auto-approval policy. Any approval or action-audit status snapshot
   refreshed for Assistant review remains read-only AI/runtime context and must
   not become command-websocket authority or lifecycle execution state.
   Patrol run `handoff_metadata` retained for saved Assistant sessions follows
   the same isolation rule: a run ID, run type/status, runtime-failure flag, or
   scoped resource label may restore Assistant presentation, but it must not
   become agent enrollment evidence, installer state, command-websocket
   identity, or fleet-control authority.
   Assistant finding-briefing action metadata derived from those same recovered
   handoff action references stays under that review-only boundary; lifecycle
   surfaces must not reinterpret clearer approval metadata as install,
   enrollment, or command-execution authority.
   The same isolation rule applies to CSRF token-store behavior in
   `internal/api/csrf_store.go`: lifecycle-adjacent browser flows may rely on
   the shared API/security layer to keep parallel replacement-token retries
   valid for one authenticated session, but retained CSRF hashes are not
   install tokens, setup-token state, enrollment authority, or agent credential
   continuity.
   The same isolation rule applies to the Pulse Intelligence agent capability
   manifest in `internal/agentcapabilities/manifest.go`: lifecycle surfaces may consume infrastructure
   setup routes advertised there, but manifest governance fields, MCP tool
   projection, shared external-tool projection helpers, shared schema-envelope
   helpers, and external-agent typed argument schemas remain API/AI-owned
   contract state rather than agent enrollment or installer semantics.
   Native Pulse Assistant provider seams and native tool-adapter names in
   shared `internal/api/ai_handler.go`, `internal/api/ai_handlers.go`,
   `internal/api/agent_profiles_tools.go`, `internal/api/router.go`, and
   `internal/api/router_routes_ai_relay.go`
   follow that same isolation rule: `MCP` remains an external protocol,
   manifest, and wire-schema term, while the in-app Assistant tool family is
   AI/runtime plus API-contract state. Lifecycle consumers must not infer
   enrollment, installer, command-websocket, or fleet-control authority from
   those native Assistant tool names, and must not reintroduce MCP-named native
   adapters to express lifecycle ownership.
   Pulse Intelligence operations-loop external-agent readiness in
   `internal/api/agent_resource_context.go` follows that same shared-helper
   split: the Pulse MCP token/scope check is AI/runtime plus API-contract
   ownership and must not become agent enrollment, installer, fleet-control, or
   command-websocket readiness.
   Patrol control completed/resolved proof and `patrolControlValueState`
   follow the shared `internal/telemetry` count-only classifier even when they
   are surfaced by `internal/api/agent_resource_context.go`; legacy
   `patrolAutonomy*` fields mirror those values only for compatibility, and
   lifecycle surfaces may observe them as operations-orientation evidence only.
   `proActivationOperationsLoopStarterCount` is only legacy entry-point
   orientation, and the legacy completed/resolved/value `proActivation*` status
   fields mirror Patrol control values as compatibility aliases, not as a second
   lifecycle signal. MCP readiness remains a separate
   external-agent handoff signal and is not a Patrol control proof input,
   agent install, registration, update, profile, command, or fleet-control
   completion.
   Approved Assistant tool execution exposed through the same shared
   `internal/api/router_routes_ai_relay.go` extension point follows that native
   naming rule: `AssistantToolExecutor` / `ApprovedAssistantToolExecutor` is the
   only current cross-repo fix-execution dependency. MCP remains an external
   protocol adapter term and must not reappear as a lifecycle command ownership
   or approved-fix execution dependency.
   The same shared-helper rule now covers SSO outbound discovery and metadata fetches plus credential-file loads in `internal/api/sso_outbound.go`, `internal/api/saml_service.go`, and `internal/api/oidc_service.go`: lifecycle-adjacent setup or auth work may depend on that shared trust boundary, but it must not fork a second HTTP client, redirect policy, file-read rule, or SSO entitlement interpretation inside lifecycle-local flows. SAML and multi-provider SSO availability remains API/security-owned Community-tier behavior, not an agent lifecycle setup gate.
   The same shared-helper rule also covers organization membership and
   cross-organization sharing transport in `internal/api/org_handlers.go` plus
   adjacent route wiring. Lifecycle surfaces may coexist on the same settings
   shell, but they must not reinterpret pending-versus-accepted org-share
   state as install authority, monitored-system admission truth, or enroll-time
   access control; target-org approval remains organization-settings plus
   security/privacy ownership even when the implementation moves through the
   shared backend API tree.
7. Keep legacy Unified Agent compatibility names explicitly secondary when touching shared `internal/api/` runtime helpers: the legacy host-route family and `host-agent:*` scope names may remain as ingress or migration aliases, but they must not retake primary ownership in router state, live runtime scope checks, handler commentary, or operator-facing guidance.
8. Add or change the unified agent CLI entrypoint, version/help exit semantics, or startup argument/error routing through `cmd/pulse-agent/main.go`.
   The CLI entrypoint owns propagation of persistence context into runtime-owned helpers. When installer-selected state roots differ from the default, `cmd/pulse-agent/main.go` must pass that exact `StateDir` through both the host-agent runtime and updater startup paths instead of letting one path silently fall back to `/var/lib/pulse-agent`.
   The same runtime-owned boundary also owns Pulse control-plane URL validation for agent startup, remote config, updater continuity, and command transport. Non-loopback control-plane URLs remain HTTPS/WSS by default, but explicitly insecure agent/dev-runtime flows may use plain HTTP/WS for LAN development control planes; installer-persisted dev URLs must not be accepted by one runtime path and rejected by another.
   The unified agent CLI copy follows the same command-execution vocabulary as the install surface. `cmd/pulse-agent/main.go` may keep the `--enable-commands` flag name for compatibility, but the help text and inline comments must describe command execution as Pulse command execution for Patrol actions and governed Proxmox LXC Docker inventory rather than reviving AI auto-fix language.
   The unified agent CLI copy also owns operator-facing Docker / Podman runtime
   labels. `cmd/pulse-agent/main.go` may keep the historical
   `--enable-docker` and `--docker-runtime` flag names for compatibility, but
   help text and inline comments must describe the module and runtime as
   Docker / Podman rather than exposing the generic container-runtime family
   label. The `--enable-docker` help text must use the operator-facing
   "Enable Docker / Podman Agent module" wording instead of leaking the
   implementation-level collection-module name.
   The CLI entrypoint also owns the local Docker / Podman privacy opt-out:
   when `--enable-docker=false` or `PULSE_ENABLE_DOCKER=false` is set on
   the host, auto-detection and remote config must not start the Docker /
   Podman module.
9. Add or change installer flags, persisted service arguments, or upgrade-safe re-entry behavior through `scripts/install.sh` and `scripts/install.ps1`.
   On Unix systemd hosts, `scripts/install.sh` must keep the default agent unit
   hardened, but command-enabled Proxmox VE agents selected with
   `--enable-commands --enable-proxmox` and type `pve` or `all` must explicitly
   allow the minimum `pct exec` / `lxc-attach` service posture by setting
   `NoNewPrivileges=false` and `RestrictSUIDSGID=false`. That exception is
   limited to the PVE command-agent path; it must not become the default for
   Docker / Podman, host-only, PBS-only, or ordinary non-command agents.
   Persistence-sensitive NAS targets must keep one canonical continuity model here: installer-owned bootstraps may use flash-backed or immutable-root launch hooks only as thin trampolines, while the durable wrapper, state, and reboot-surviving binary copy stay in the governed persistent state directory that updater continuity also refreshes.
   Approval-gated command execution must expose stable rejection reasons for
   invalid approval grants so fleet operators can distinguish missing, expired,
   mismatched, and signature-invalid grants through agent metrics.
10. Preserve canonical token-lifecycle reads in shared `internal/api/` auth/security helpers so lifecycle-adjacent setup and install flows do not revoke a displayed relay pairing token after `lastUsedAt` proves that an already paired device is actively depending on that credential.
11. Preserve backend-owned Pulse Mobile relay runtime credential minting in those same shared `internal/api/` auth/security helpers so lifecycle-adjacent setup and install flows reuse the canonical mobile token route instead of reintroducing wildcard or browser-authored runtime token bundles.
12. Preserve the dedicated backend-owned `relay:mobile:access` capability and its governed backward-compatible route inventory plus the shared helper call sites around it, so lifecycle-adjacent setup and install flows do not widen the mobile device credential back into general AI chat/execute scope ownership.
13. Preserve shipped security-doc guidance in shared lifecycle setup helpers so `internal/api/config_setup_handlers.go` and adjacent install/setup runtime paths point operators at the running build's local security documentation route rather than GitHub `main` links.
14. Keep shared `internal/api/router.go` workload-chart downsampling presentation-only: when that router caps mixed-cadence workload history into equal-time buckets for operator-facing cards, lifecycle-adjacent setup and fleet surfaces must not reuse the shaped chart samples as heartbeat, enrollment, or last-seen authority.
    That same presentation-only boundary must preserve canonical millisecond timestamps when it serializes chart points, so lifecycle-adjacent first-host and fleet surfaces do not misread rounded chart samples as duplicate or restarted heartbeat evidence.
    The same rule now applies to storage summary interaction. Shared sticky-card or row-hover focus behavior on infrastructure, workloads, and storage may reuse the canonical chart transport, but lifecycle-adjacent install, enrollment, and fleet surfaces must not treat highlighted summary series or sticky-shell state as agent freshness or setup progress.
    The same rule now applies to infrastructure-summary metric filters. Shared
    infrastructure and other route-owned consumers may narrow the canonical
    `/api/charts/infrastructure` payload with a `metrics` query for
    presentation hot paths, but lifecycle surfaces must not reinterpret
    omitted disk or network series as missing lifecycle telemetry, missing
    agent capabilities, or reduced fleet freshness truth.
    The same rule now applies to retired compact dashboard summary payloads.
    Shared `internal/api/resources.go` routes must not restore
    `/api/resources/dashboard-summary` as a compatibility read; lifecycle
    surfaces must continue to use install inventory, enrollment proof, and
    fleet freshness truth from their owning contracts.
    The same presentation-only boundary now covers compact storage summary
    chart reads as well. Shared `/api/charts/storage-summary` transport may
    request only the canonical `used` and `avail` storage series needed for the
    dashboard capacity sparkline, and lifecycle surfaces must not reinterpret
    the omitted `usage` or `total` series as missing lifecycle telemetry or
    enrollment-state evidence.
    Dashboard storage trend consumers on that shared router boundary must now reuse the single `/api/storage-charts` summary response instead of fanning out per-pool `/api/metrics-store/history` reads, and lifecycle surfaces still must treat that batched storage summary transport as presentation context only rather than install, enrollment, or freshness truth.
15. Keep install-script serving fallback-free, and keep the lifecycle
    agent-download fallback pinned to published release lineage. The served
    install-script endpoints (/install.sh, /install.ps1) have no GitHub fallback:
    `internal/api/unified_agent.go` serves the locally bundled AGENT installer
    (present in every release tarball and Docker image) or fails closed, so
    first-host install, repair, and fleet continuity never receive the top-level
    GitHub install.sh SERVER installer in place of the agent installer (issue
    #1470). The agent-BINARY download fallback that remains may only treat stable
    tags and explicit RC prerelease tags as release assets; working-line dev
    prereleases and build-metadata versions must fail closed so install/repair do
    not depend on unpublished or branch-local URLs.
16. Keep self-hosted purchase handoff state on the adjacent commercial/auth
    boundary. When shared `internal/api/router.go`,
    `internal/api/router_routes_cloud.go`, `internal/api/licensing_handlers.go`,
    or `internal/api/demo_mode_commercial.go` evolve public
    `/auth/license-purchase-start` or `/auth/license-purchase-activate`,
    lifecycle-adjacent setup and fleet
    surfaces may rely on that public-route wiring but must not reinterpret the
    commercial-owned `portal_handoff_id`, server-resolved checkout intent, purchase-return tokens,
    activation-bridge form state, owned billing purchase-arrival states, or
    demo-hidden commercial route policy as installer credentials,
    registration state, or fleet enrollment authority. The adjacent licensing
    boundary also owns public-vs-Pro runtime build attribution for activated
    installs; lifecycle and fleet surfaces may read the resulting
    entitlement/runtime-capability posture, but must not infer paid runtime
    status from agent install commands, public image tags, or local lifecycle
    inventory. The same adjacent
    commercial boundary also owns purchase-start unavailability recovery:
    lifecycle-adjacent surfaces may coexist with that shared browser route, but
    they must not strand install or fleet flows on a raw Pulse Account error
    tab, reinterpret `purchase=unavailable` as lifecycle repair state, or
    bypass the shared secure callback policy that limits self-hosted commercial
    return URLs to HTTPS instance origins or direct-loopback HTTP and keeps
    hosted commercial follow-up fetches on the restricted outbound client.
    bypass the owned billing retry/recovery path.
    That same adjacent commercial/auth boundary also owns the canonical
    self-hosted purchase intent label: lifecycle-adjacent setup and install
    flows may observe `self_hosted_plan`, but they must not keep emitting or
    inferring legacy `max_monitored_systems` intent/query values once the
    uncapped self-hosted model is canonical.
    The same adjacent commercial boundary treats migrated-v5 monitored-system
    grandfathering as retired compatibility metadata. Lifecycle surfaces may
    react to active license or entitlement payloads, but they must not cache
    their own pre-activation host counts, synthesize a grandfather floor,
    restart capacity reconciliation from billing reads, or reinterpret
    continuity payloads as install eligibility, fleet enrollment evidence, or
    `0 / limit` monitored-system state.
    That same adjacent commercial boundary also owns authenticated
    install-version attribution for migrated installs: lifecycle surfaces may
    observe versioned commercial status, but they must not treat
    activation/exchange/refresh version fields as installer enrollment state,
    invent a second fleet-version cache, or backfill install lineage from
    local host inventory when the shared licensing runtime already sends the
    canonical process version.
    That same adjacent commercial boundary also owns internal demo-fixture
    grants: lifecycle surfaces may observe that a governed demo runtime is
    fixture-backed, but they must not mint, echo, or infer the internal
    `demo_fixtures` capability through install/setup payloads or installer
    heuristics.
    The same lifecycle-adjacent platform-connections boundary also assumes
    direct TrueNAS and VMware connection writes fail closed when canonical
    monitored-system grouping is unavailable. Shared `internal/api/` preview
    helpers may return `monitored_system_usage_unavailable` before save, and
    VMware must not collect external vCenter inventory before that canonical
    grouping view is safe, so fleet/setup surfaces do not fork monitored-system
    identity through direct API writes.
    The same lifecycle-adjacent platform-connections boundary now also owns
    the unified connections ledger (`GET /api/connections`) and address
    probe (`POST /api/connections/probe`). Lifecycle surfaces may observe
    agent `Host.LastSeen`-backed rows on that ledger, but must not
    reinterpret derived `state` (active/paused/unauthorized/unreachable/
    stale/pending) as install authority or treat the probe response as
    enrollment state. Metadata, link-local, multicast, and unspecified
    probe destinations must fail closed before any outbound dial, and
    lifecycle surfaces must surface that canonical rejection instead of
    retrying through lane-local probe helpers. Ledger writes still flow
    through the per-type config
    endpoints that own admission checks, and the `Disabled` flag on
    PVE/PBS/PMG surfaced by that ledger must remain a pause-only signal
    rather than an installer pre-flight gate.
    The unified add surface is the governed modal flow mounted by
    `InfrastructureWorkspace.tsx`: the landing page owns the persistent
    instance list, the picker dialog owns source-type selection, and
    `frontend-modern/src/components/Settings/ConnectionEditor/ConnectionEditor.tsx`
    owns detect-driven credential handoff plus type-specific form bodies.
    That same landing now owns explicit discovery review for API-backed
    Proxmox-family systems: `InfrastructureSourceManager.tsx` may surface
    discovered VE / PBS / PMG candidates under their matching platform groups,
    and `InfrastructureWorkspace.tsx` may route a candidate's `Review` action
    into the same typed add dialog with canonical prefills. Lifecycle flows
    must not fork a second discovery-specific credential wizard or treat
    discovery results as already-enrolled systems before the operator saves
    the governed add form. That same landing-owned shell now keeps discovery
    compact: the persistent page may expose only a concise discovery status
    line plus `Run discovery` / `Discovery settings` actions, while new-source
    admission stays on the per-platform table actions instead of competing
    with discovery at the top of the page. Command-backed discovery sweeps and
    forced single-resource refreshes remain API/AI-owned admin operations:
    lifecycle surfaces may expose the controls, but route-level authority must
    require `settings:write` plus the Discovery enablement gate, not
    `monitoring:write`, enrollment state, or any fleet lifecycle shortcut.
    Forced workload discovery also remains API/service-owned for endpoint
    identity: lifecycle surfaces must not fill a Proxmox parent node or agent
    name into workload trigger hostnames when the operator did not provide one,
    because the discovery service resolves VM and system-container names from
    canonical state before suggesting URLs or classifying known services.
    The editor's probe step calls the aggregator probe endpoint and
    dispatches the detected or manually-selected type into a credential
    slot; it must not bypass the probe endpoint or fabricate probe
    candidates, and the agent credential slot must continue to reach
    `InfrastructureInstallerSection.tsx` so install handoffs remain on the
    canonical unified-agent install path. Those governed add/edit dialogs
    must also keep their form body scrollable inside the modal:
    `InfrastructureWorkspace.tsx` and `ConnectionEditor.tsx` keep the
    content shell on `min-h-0` flex columns so long lifecycle forms do not
    clip lower fields behind the dialog boundary. For PVE, PBS, and PMG, the
    credential slot is
    `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`,
    which reuses the existing `NodeModalBasicInfoSection`,
    `NodeModalAuthenticationSection`, `NodeModalMonitoringSection`, and
    `NodeModalStatusFooter` primitives inline under the editor — dropping
    the Dialog wrapper and the surrounding discovery/configured-nodes
    workspace. The inline credential slot must keep the visible setup sequence
    as `Endpoint`, `Authentication`, and `Coverage` before the PVE/PBS/PMG
    setup forms so lifecycle actions keep a stable operator model inside the
    unified editor. For TrueNAS and VMware, the credential slots are
    `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx`
    and
    `frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx`;
    they extract the inner form bodies from the per-type panels and
    render them inline under the editor while still driving the existing
    `TrueNASSettingsPanelState` and `VMwareSettingsPanelState` APIs for
    save, test, preview, and impact-preview behavior. Feature-disabled,
    delete-error, and probe guidance notices in the connection editor may
    compose frontend-primitives' `CalloutCard`; agent-lifecycle owns the
    selected-source lifecycle semantics and form handoff, not colored local
    notice shells. The add flow
    must not reintroduce the full per-type workspace (Proxmox discovery
    card, configured nodes table, node-modal stack; TrueNAS/VMware
    connection list with headers and row actions) into the credential
    slot, because that previously showed the ledger-of-other-systems in
    the middle of entering one system's credentials. The configured
    systems ledger reads exclusively from the unified aggregator: the
    `InfrastructureSourceManager` rows are produced by
    `frontend-modern/src/components/Settings/useConnectionsLedger.ts`,
    which polls `GET /api/connections` and maps the aggregator-derived
    state (active/paused/unauthorized/unreachable/stale/pending) and
    active scope keys into the source-manager display. `InfrastructureWorkspace`
    must not reconstruct per-type health, scope, or last-seen columns
    from any retired reporting-local state for configured connection
    rows; the aggregator is the only configured-connections source of
    truth. That same aggregator-authored connection/member payload also owns
    discovery reconciliation for already represented hosts: when a platform
    row, its attached agent augmentation, or one of its child members already
    carries canonical host aliases, the settings shell must suppress any
    duplicate discovery candidate for that same platform instead of asking the
    operator to review the same machine twice under a hostname row and an
    IP-only candidate row. Per-row Manage actions live directly on the
    `InfrastructureSourceManager` row and open the governed edit/detail flow;
    `frontend-modern/src/components/Settings/useConnectionRowActions.ts`
    owns the pause/remove API dispatch, two-click remove confirm, and
    per-id action error presentation inside that flow. Last-error detail is rendered
    inline on the row when `connection.lastError` is non-null, not
    hidden behind a click-through. Remove-confirm on an agent detail flow
    reveals the Linux + Windows uninstall commands so the operator can copy
    and run them before the final confirm; that expansion replaces the legacy
    `InfrastructureActiveRowDetails` surface-breakdown drawer. The
    ledger must never reintroduce a separate detail page or `Dialog`
    drawer for viewing a connection's aggregator fields — everything is
    on the row.
    The TrueNAS and VMware credential slots carry per-surface Monitor\*
    scope the same way the PVE/PBS/PMG credential slot already does:
    `TrueNASSettingsPanelState`/`VMwareSettingsPanelState` read and
    write positive `MonitorDatasets`/`MonitorPools`/`MonitorReplication`
    (TrueNAS) and `MonitorVMs`/`MonitorHosts`/`MonitorDatastores`
    (VMware) booleans, and the credential slot renders a
    "Collection scope" checkbox cluster backed by those fields. The
    unified aggregator must project those flags into the connection
    row's `scope` map and keep `capabilities.supportsScope: true` for
    TrueNAS/VMware — reintroducing the per-type Stop-this-surface
    dialog or hard-coding scope to a fixed all-true map is forbidden.

## Forbidden Paths

1. New install or update continuity behavior hidden only inside broad monitoring ownership.
2. Agent profile or fleet-control behavior implemented outside the canonical agent settings/profile surfaces.
3. Installer or update flows that depend on branch-tip, dev-only, or non-release asset behavior for supported RC/stable paths.
4. Lifecycle setup, install, or fleet surfaces that invoke retired self-hosted trial acquisition; `POST /api/license/trial/start` and the retired `/auth/trial-activate` callback must stay closed on the ordinary self-hosted router rather than reappearing as lifecycle-local CTAs or retry paths.

## Completion Obligations

1. Update this contract when agent lifecycle ownership changes. Routes added under the shared `internal/api/` extension point that are clearly outside lifecycle ownership (for example `POST /api/ai/patrol/preflight`, the `patrol_preflight` snapshot field added to `/api/settings/ai`, the auto-trigger preflight dispatch on settings save, the startup-seed dispatch in `NewAISettingsHandler`, and the cached-preflight integration into the Patrol `tools` readiness check — all owned by ai-runtime) do not extend this subsystem's contract; they live in their owning subsystem.
2. Keep shared API proof routing aligned whenever install, register, or profile payloads change.
3. Update runtime and settings tests in the same slice when lifecycle behavior changes.
4. Keep host-agent test hooks, command-client factories, and timing overrides
   instance-scoped under `internal/hostagent/agent.go`; lifecycle-owned
   registration and update paths must not depend on package-global mutable test
   seams that can leak between concurrent agent sessions or tests.
5. Preserve canonical /api/auto-register node identity continuity when canonical hosts shift between hostname and IP forms for the same node.
   That same lifecycle-adjacent shared-API boundary now covers relay and
   command-target hostname resolution too. When lifecycle flows reuse
   `internal/api/router_routes_ai_relay.go`, `internal/agentexec/server.go`,
   or other shared agent-target helpers, they may treat a short hostname as
   equivalent to the same agent's FQDN, but they must not widen that fallback
   into a short-name collapse that would make two different FQDNs appear to
   be the same lifecycle target.
6. Keep Proxmox registration continuity self-healing: stale local registration markers must be verified against Pulse before the host agent skips setup, and a missing matching node on the Pulse side must drive canonical re-registration instead of asking operators to delete marker files manually.
7. Keep first-session lifecycle handoff explicit: the live setup completion
   surface in `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`
   must route the primary CTA into `/settings/infrastructure?add=pick`, frame
   that route as source strategy selection, and present platform API inventory
   plus Pulse Agent telemetry as peer choices for Proxmox, TrueNAS, VMware,
   standalone hosts, and future provider integrations rather than leaving
   post-setup next actions implicit. A direct Pulse Agent handoff may remain as
   a secondary control for operators who already know the first source is
   agent-managed, but the primary first-run path is the unified source picker.
   Once the completion surface observes connected systems, that same handoff
   model must derive its follow-up actions from the canonical connected-system
   path classification rather than a raw connected-agent count. API-backed
   first-session states must keep `Add infrastructure` visible for both
   API-backed and agent-managed next systems instead of reviving separate
   `Platform connections` and `Infrastructure Install` branches. The
   completion panel, infrastructure installer, install state, and agent-profile
   settings surfaces must also stay free of local browser commercial or
   onboarding metrics wrappers. Lifecycle surfaces may navigate to canonical
   destinations, but Pulse Account and server-owned reporting routes own
   commercial event capture.
   API-backed versus agent-managed classification must come from the governed
   onboarding paths in
   `docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json` through
   the shared frontend manifest helper, not from a Setup Wizard-local platform
   allowlist. When preview-only browser proof needs a deterministic connected
   snapshot, `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`
   may accept a preview-provided connected-resource override, but the live
   first-session runtime path must keep `/api/state` polling as the sole
   source of connected-system truth when no override is supplied.
   Localized setup-completion copy may route through
   `frontend-modern/src/i18n/messages.ts`, but the lifecycle contract remains
   unchanged: add-infrastructure and agent-install destinations, API token
   meaning, generated credential values, install/source product identifiers,
   and reported resource names must stay stable while only surrounding
   operator-facing guidance translates.
   7a. Keep lifecycle-neutral shared `internal/api/` changes from altering agent
   setup, registration, install, or profile payloads by accident. AI runtime
   or entitlement work that touches shared router or handler wiring must keep
   lifecycle public routes, setup-token validation, and agent profile payloads
   unchanged unless the lifecycle contract and its proofs are updated in the
   same slice.
8. Keep `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
   oriented around the first monitored host. Install-token generation,
   governed command copy, and install instructions belong to the canonical
   lifecycle path; transport details, trust overrides, profile tuning, and
   adjacent alternatives must remain secondary to that first-host onboarding
   narrative, including an explicit advanced-options disclosure so first-time
   operators see token generation, command copy, and status confirmation
   before non-default connection controls.
   Setup handoff and connected-system completion actions in that installer
   surface must keep their lifecycle meaning, destination, and token/download
   behavior here, while composing the frontend-primitives `Button` variants for
   success, outlined success, and ghost success chrome instead of carrying
   lifecycle-local emerald button shells.
9. Keep `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`
   and `frontend-modern/src/components/Settings/infrastructureWorkspaceModel.ts`
   aligned with that same lifecycle path. The bare
   `/settings/infrastructure` route must render one Connections and inventory
   ledger that lists top-level infrastructure only — active or ignored
   infrastructure roots plus saved Proxmox VE, PBS, PMG, TrueNAS, VMware, and
   agent-managed entries — as sibling rows sharing one
   system/coverage/collection/status/last-activity workspace model, so
   operators can read infrastructure state in one scan instead of hopping
   between install, reporting, and provider shells. Guest-linked agent rows
   still belong to the reporting inventory and inline lifecycle detail, but
   they must not appear as peer connection rows on that top ledger. Adding a
   new system must stay a single entry point on that ledger:
   one `Add infrastructure` entry point that opens the source picker, keeps
   `Install on a host` explicit only after the operator chooses Pulse Agent,
   and opens the saved-connection create flow for API-backed platforms on the
   same page. `/settings/infrastructure/install`,
   `/settings/infrastructure/platforms`, and
   `/settings/infrastructure/operations` are retired settings aliases and must
   fail route eligibility instead of resolving to section focus or rendering
   separate page shells. The canonical add flow is
   `/settings/infrastructure?add=<step>`. Read-only sessions must redirect the
   canonical Infrastructure workspace back to `/settings/infrastructure`,
   suppress the add-system entry point, and hide configuration-only sections so
   presentation-policy restrictions still hold. That top ledger must also stay
   readable inside the governed settings
   shell at ordinary desktop widths: `InfrastructureSourceManager.tsx` must
   keep the compact row/card presentation readable without forcing horizontal
   scrolling just to reach primary controls. Dedicated collection and
   last-activity columns may return only once the workspace has enough width
   to show the full ledger without clipping headers or row actions.
10. Keep post-install lifecycle completion explicit inside
    `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
    and `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`.
11. Keep the dev first-session proof deterministic on the real wizard path:
    `tests/integration/tests/helpers.ts` and
    `tests/integration/tests/11-first-session.spec.ts` must refresh first-run
    state through `/api/security/dev/reset-first-run`, then prove the
    canonical `Add infrastructure` handoff and the explicit `Install Pulse
Agent` secondary handoff against the live setup wizard instead of relying
    on stale bootstrap tokens, dashboard fallbacks, or preview-only coverage.
    The primary handoff must land on the shared infrastructure onboarding
    contract at `/settings/infrastructure?add=pick` and normalize back to
    `/settings/infrastructure` instead of reviving a separate
    platform-management shell. The secondary agent handoff must land on
    `/settings/infrastructure?add=agent`.
    When the first host reports successfully, the install workflow must treat
    that as a completion handoff with direct navigation into
    `/settings/infrastructure` and the first visible platform/runtime page
    instead of leaving operators on a generic lookup result or reviving the
    removed `/infrastructure` route. When the workspace starts from zero
    active connected
    infrastructure and install commands are available, the same lifecycle path
    must auto-watch the canonical `/api/state` projection for the first
    reporting host rather than requiring a brand-new operator to know and type
    a hostname or agent ID just to see the first success handoff. When that
    workspace is entered through first-run setup handoff, the same lifecycle
    path must also auto-create the scoped first-host install token so the
    operator lands on ready-to-copy commands instead of being asked to perform a
    second manual token-generation step immediately after securing the server.
    Any first-run credentials download generated from that same handoff must
    describe the prepared first-host token path consistently instead of telling
    the operator to generate another install token manually.
12. Keep `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`
    ordered around the actual first-run operator sequence: credentials that must
    be saved now should be visible before the operator leaves the screen, and
    the completion surface should present one canonical primary next-step path
    into Add infrastructure instead of repeating competing install or dashboard
    CTAs across multiple sections. Once the first monitored system is
    already connected, that same surface must pivot its primary CTA and headline
    to `/settings/infrastructure` so the operator returns to the shared
    infrastructure workspace rather than being told to connect the first source
    again or being sent through a generic home/dashboard completion. While the
    first source is still pending,
    that same completion narrative must describe Add infrastructure as the
    place where the operator chooses platform API inventory, Pulse Agent
    telemetry, or both. If the operator selects the direct agent path from that
    completion surface, the agent install body may prepare the first-host
    scoped install token from setup handoff, and when it names the shared
    settings workspace for follow-up lifecycle control it must use the
    canonical `Infrastructure` label instead of reviving the retired
    `Infrastructure Operations` wording.
    not as a second manual token-generation task the operator still needs to
    figure out.
13. Keep API-backed platform onboarding explicit across
    `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`,
    `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`,
    `frontend-modern/src/utils/agentInstallCommand.ts`,
    `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`,
    `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`, and
    `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`.
    Machines/host onboarding means a Pulse Agent install with full host
    telemetry, while agentless machine reachability remains Availability-owned.
    Unix-family copied installer commands must run the shared installer
    preflight before privilege escalation, verify the exact agent binary
    artifact, and pass token-bearing installs through ephemeral `--token-file`
    transport instead of raw service arguments.
    TrueNAS must be presented as an API-backed source flow through Add
    infrastructure first, not as a dedicated Unified Agent install profile. The
    agent install path may remain available for optional later agent
    augmentation on TrueNAS, but first-run copy, alternative CTAs, and
    install-profile lists must not imply that an agent install is the required
    bootstrap for TrueNAS support in Pulse.
14. Keep first-session and lifecycle-adjacent frontend resource handling on the
    canonical unified-resource boundary. Top-level TrueNAS appliances may reach
    setup-completion or infrastructure lifecycle surfaces only as canonical
    `agent` resources with `platformType: 'truenas'`; any legacy raw
    `resource.type === 'truenas'` compatibility collapse belongs in the shared
    frontend resource adapters, not in setup or lifecycle-local UI branching.
15. Keep lifecycle-adjacent AI transport compatibility on the shared
    `internal/api/` boundary. If chat mention parsing, alert investigation
    targets, or adjacent Assistant resource transport still accept a legacy
    top-level `truenas` type, that value must collapse immediately to the
    canonical `agent` host type before lifecycle surfaces, setup handoffs, or
    operator-visible route state consume it.
16. Keep onboarding ownership aligned with
    `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`: agent-backed
    first-class platforms belong to the install/reporting lifecycle path,
    API-backed first-class platforms belong to the Add infrastructure API
    source flow, and any later unified-agent augmentation on an API-backed
    platform must remain an optional secondary path instead of silently
    becoming the required bootstrap.
17. Preserve the on-agent command-policy approval gate in
    `internal/hostagent/commands.go` and `internal/agentexec/server.go`.
    Both ends may honor a `Trusted` flag on `ExecuteCommandPayload` to
    bypass the `PolicyRequireApproval` branch, but only when the payload
    is constructed by a vetted Pulse-internal call site that ships its
    own hardcoded command catalog (today only the servicediscovery deep
    scanner via `internal/ai/discovery_adapter.go`). `PolicyBlock` must
    still apply to trusted payloads, and `Trusted` must never be set
    from a deserialised HTTP body, a user-supplied command string, an
    AI tool call, or a governed approval consumer — those callers must
    continue to carry an `ApprovalID` and approval grant.

## Current State

Denied Patrol investigation-fix approvals passing through shared
`internal/api/` handlers are adjacent AI-runtime/action-governance state only.
The `fix_rejected` finding outcome records that an operator declined a proposed
Patrol fix before execution; it must not be reinterpreted as agent enrollment,
install progress, fleet liveness, or an agent-command failure.

Patrol finding lifecycle payloads exposed through shared AI handlers, including
operator resolution-note fields, remain AI-runtime/API-contract vocabulary:
adjacent fleet and install surfaces may render the lifecycle state when another
surface supplies it, but they must not reinterpret it as install progress,
agent approval state, topology truth, or lifecycle-owned remediation policy.

Default-org token scoping and notification-settings fan-out on shared
`internal/api/` handlers are likewise adjacent only: org-bound token denial
for the default org and instance-wide webhook allowlist propagation are
api-contract/security owned and change no install, enrollment, setup-token,
or fleet command authority. Agent tokens keep routing agents into their bound
org exactly as before.

Notification webhook management changes on shared `internal/api/` handlers are
likewise adjacent only: the webhook `signingSecret` payload field, its masked
list representation, and masked-echo preservation on update are
notifications/API-contract owned and grant no agent install, enrollment,
setup-token, or fleet command authority.

Alert delivery diagnosis on shared `internal/api/alerts.go` is likewise
adjacent only: `/api/alerts/delivery-diagnosis` is alerts/API-contract owned
read-only notification-policy evidence and must not be interpreted as agent
enrollment, agent liveness, install progress, setup-token authority, or fleet
repair eligibility.

The router projection-builder (`internal/api/router.go`) that wires
the operator-state provider into the findings runtime now also
populates `NeverAutoRemediate` on the projection. The investigation
runtime reads the same projection to attach `OperatorContext` to
findings handed to the orchestrator, so investigation reasoning sees
the same lock-against-remediation flag that the action broker
enforces downstream — no possible drift between "what Patrol
proposes" and "what the broker accepts."

The same router wiring owns the alert-bridge patrol-trigger callback. It now
receives the full alert payload as a struct and consults the operator's
per-rule trigger policy before queuing a scoped patrol: an `alert_fired` event
that fails `AIConfig.AlertTriggersInvestigation` (below the minimum-severity
floor or outside the alert-type allowlist) is logged and dropped without
entering the trigger manager, so alert-driven investigation lifecycle stays
bounded to the alerts the operator opted into.

`/api/agent/events` is the SSE stream agents subscribe to for
real-time notifications: `finding.created` when a new finding is
raised, `approval.pending` when a remediation request enters
StatusPending and waits on operator decision, `action.completed`
when an action audit reaches a terminal state (Completed,
runtime-Failed, or refused-before-dispatch with stable
`plan_drift:` / `resource_remediation_locked:` error-token
prefixes preserved verbatim; successful dispatches carry a
`verification` block — the agent-stable projection of the
broker's read-after-write probe, with verification command/note
details redacted to stable markers at the action-audit readback
boundary — so agents close the "did it actually work?" loop without
polling /api/actions/{id}),
and a 15-second heartbeat keepalive.
The broadcaster drops real published events for slow subscribers
rather than blocking publishers, so the patrol-finding runtime, the
approval store's post-create callback, and the executor's
post-completion callback can publish without ever stalling on
consumer slowness. API-owned action execution uses the same shared
terminal publisher, including stale-plan `action_plan_drift`
refusals that fail the audit with a `plan_drift:` result before any
executor is called. Heartbeats are stream-local keepalives written
to each connected response; one subscriber's heartbeat ticker must
not publish heartbeat events to every other subscriber.
The agent runtime keeps the broadcaster wired across restarts, and
the capabilities manifest declares the stream under
`subscribe_events` so the surface stays self-describing.

Shared `internal/api/router.go` metrics-history reads may now expose
host-agent CPU temperature as an `agent` chart metric for Proxmox node
drawers, but lifecycle surfaces must continue treating that data as
presentation telemetry only. Temperature history is not heartbeat authority,
agent enrollment state, or freshness proof; those remain owned by the
agent-lifecycle event, token, and last-seen contracts.

`/api/agent/capabilities` is the discovery document any external
agent reads to learn what Pulse exposes. The manifest is
unauthenticated and cacheable; the underlying capabilities keep
their own auth scopes. Adding a capability is a deliberate "this
is part of the agent surface" commitment so the agent contract
stays curated, not auto-derived from every internal endpoint.
The manifest's action mode and approval policy metadata are API/AI-owned
governance posture; lifecycle setup, enrollment, and fleet-control flows may
surface that metadata to agents but must not reinterpret it as install,
registration, heartbeat, update, or agent authority.
The same adjacency applies to manifest `inputSchema` metadata: typed tool
arguments help external agents call API-owned actions, but do not create a
lifecycle-owned setup command path, operator-state write contract, or
agent-control argument registry.

`/api/agent/resource-context/{id}` is the agent-paradigm substrate
endpoint: any agent (in-process Patrol/Assistant or external) reads
the full situated picture of a resource — identity, operator state,
active findings, pending approvals scoped to the resource, recent
actions including refused dispatches with their stable token
prefixes preserved — in one call. The endpoint is read-only under
`monitoring:read`. The active-findings section flows through an
`AgentFindingsProvider` adapter wired in `router.go` from the
patrol service so the api package stays free of an `internal/ai`
import; the parallel `AgentApprovalsProvider` adapter resolves the
canonical approval store at request time, filters full per-resource
summaries by canonical resource id and org, and exposes a separate
resource-keyed count projection for fleet reads. The agent runtime
keeps both providers wired across restarts.

`/api/agent/fleet-context` is the companion triage view: one read
returns a thin per-resource rollup across every resource visible
to the org — identity, operator flags
(`intentionallyOffline`, `neverAutoRemediate`,
`maintenanceWindowActive`), per-severity finding counts, and
pending-approval count. Same auth scope (`monitoring:read`) and
same provider wiring as the per-resource bundle; the fleet sweep
walks the registry once and reuses the in-memory findings index,
one bounded approval-store scan grouped by canonical resource id,
and a per-resource operator-state SQLite point lookup. Agents pick
"where do I focus?" from the fleet view and then drill into the
per-resource bundle for depth.

`/api/agent/capabilities` is registered in the router's
`publicPaths` list so the global auth middleware does not gate
the discovery manifest. The manifest is unauthenticated by
design: an agent that does not yet have a token must still be
able to introspect Pulse to learn how to ask for one. The
underlying capabilities the manifest declares keep their own auth
scopes — the public posture applies only to discovery itself.

The action governance loop (`/api/actions/plan`,
`/api/actions/{id}/decision`, `/api/actions/{id}/execute`) joined
the agent surface via the manifest under a new `action` category,
gated by the `ai:execute` scope (distinct from `monitoring:write`
because action governance is the governed-execution dimension).
The handlers were migrated from the platform-wide `APIError`
envelope to the agent-stable `{"error", "message", "details"?}`
shape so the substrate keeps a single envelope contract across
read, write, and action capabilities.
The governed Patrol finding lifecycle tools advertised in the same
manifest (`acknowledge_finding`, `snooze_finding`, `dismiss_finding`,
`resolve_finding`) also return that agent-stable envelope for branchable
failures and declare their closed error-code set in the manifest. That is
an API/AI-runtime contract only: it does not create agent lifecycle state,
registration state, install state, or any new durable recovery artifact.
Docker / Podman lifecycle execution extends that same action category only:
the manifest advertises the `execute_action` substrate, while per-resource
container capabilities, policy checks, action audit records, and terminal
verification remain API-owned facts rather than agent lifecycle state.

The findings runtime now consumes operator-set per-resource state
through a provider adapter wired in `internal/api/router.go` at
startup. The adapter returns a `ResourceOperatorStateProjection`
covering every signal in one call (active maintenance window plus
the indefinite `IntentionallyOffline` flag), so adding new signals
later does not multiply round-trips per finding. When a resource is
in an operator-set maintenance window OR is marked intentionally
offline, new findings against it are auto-acknowledged at creation
time so the operator's commitment is honored without flooding
notifications. The agent runtime keeps the operator commitment honored across
restarts because the underlying state lives in the durable
`resource_operator_state` SQLite table.

The `/api/resources/{id}/operator-state` GET / PUT / DELETE handlers in
`internal/api/resources_operator_state.go` are the canonical operator
surface for setting per-resource intent (intentionally offline, never
auto-remediate, maintenance window, criticality hint). The route lives
on the same monitoring router (`router_routes_monitoring.go`) as the
rest of `/api/resources/{id}/...`; method-keyed scope dispatch means GET
runs under `monitoring:read` while PUT and DELETE require
`monitoring:write` because they modulate Patrol's behavior on findings
against the resource. The agent runtime must surface the same
operator-set state across restarts — persistence is in the
`resource_operator_state` SQLite table managed by the unified-resources
store from slice 29 — so a maintenance window or never-auto-remediate
flag set before a process restart is honored after the agent reloads.

Patrol-finding to unified-finding mirroring in `internal/api/router.go`
also keeps the will_fix_later wake-up deadline (`Finding.RemindAt`)
intact across restarts. Both the live wire-up callback and the
persistence-recovery resync must copy `f.RemindAt` onto the unified
finding so the operator's commitment survives a reboot or process
restart instead of silently lapsing into the canonical findings store
without being mirrored on the API surface.
The same router boundary now also threads `Finding.AutoResolved`
through to `UnifiedFinding.AutoResolved` so an operator who manually
clicked Mark resolved before a restart keeps that attribution after
the agent reloads findings from disk. Without that mirror the
recovery sync would flatten every prior closure into Pulse's
auto-detection bucket and erase the operator-vs-Pulse distinction
across process restarts.

Linux agent privilege hardening is now part of the installer/runtime contract.
The supported full-telemetry systemd agent may still run as `root`, but
`cmd/pulse-agent/main.go` must bind health/metrics to loopback by default,
`scripts/install.sh` must preserve explicit health-address disable/open choices
in the rendered service, and generated systemd units must keep conservative
sandboxing in place unless a future telemetry requirement records a narrower
exception.
New Proxmox VE and Proxmox Backup Server setup must default to the API
Inventory path: the UI may recommend a root Pulse Agent only as the Host
Telemetry Agent path for temperatures, SMART, local storage detail,
agent-driven operations, or other node-local telemetry the Proxmox API cannot
provide.
Generated API Inventory scripts remain a one-time privileged setup action, but
their steady-state credential must be a narrowly scoped Proxmox API token: PVE
setup must use a privilege-separated token and mirror generated ACLs onto both
the service user and token, while PBS setup must keep `Audit` grants on both
the service user and token.

Generated TrueNAS CORE rc.d service scripts must give `/usr/sbin/daemon -r` a
supervisor pidfile with `-P`, keep the child pid in a separate diagnostic
pidfile, and stop legacy child-pidfile installs by resolving the child back to
its daemon supervisor before replacing or restarting the agent binary.

Deploy selection and retry no longer carry monitored-system capacity feedback.
Lifecycle-owned deploy surfaces must not revive license-slot, workspace-slot,
plan-upgrade, or monitored-system capacity wording in user-facing
confirmation, preflight, retry, and status labels.

The infrastructure workspace collapsed to a single `/settings/infrastructure`
route. `buildInfrastructureWorkspacePath()` always returns the base path;
add flow state is carried only by the `add` query parameter on that base path.
`SetupCompletionPanel.tsx` uses the shared infrastructure path builders for all
install and platform CTAs.
`frontend-modern/src/utils/infrastructureSettingsPresentation.ts` owns the
customer-facing Settings Infrastructure target label and onboarding source
strategy copy. Lifecycle and setup guidance must point operators to
`Settings → Infrastructure` and must not revive removed subpaths such as
`Settings → Infrastructure → Proxmox`.
The shared monitored-system impact preview now formats save-impact
summaries through `frontend-modern/src/utils/monitoredSystemPresentation.ts`
so infrastructure setup screens describe count impact and grouping changes
without raw slash-quota rendering.

This subsystem now sits under the dedicated agent lifecycle and fleet
operations lane so install, registration, update continuity, profile
management, and fleet safety stop hiding inside architecture, migration, or
monitoring work.
Lifecycle-owned connected-infrastructure and reporting browsers now also keep
governed platform rows on canonical local operator identity while tolerating
optional optimistic hostnames. Shared row models may fall back to the row name
when staging a removal state, but they must not resurrect legacy
`policy.display` shims or require platform-managed surfaces to synthesize a
second hostname contract.
That same adjacent `internal/api/` router boundary now also keeps usage-data
controls out of lifecycle truth. Agent install, reporting, and setup surfaces
must not depend on `/api/upgrade-metrics/*`, telemetry preview routes, or
local-only upgrade-event state under shared licensing/auth routing. The normal
customer product router must keep those retired commercial analytics routes
absent, and lifecycle code must not reinterpret telemetry preview payloads or
published-release classification fields as enrollment evidence, agent
freshness, or setup progress truth.
That same adjacent `internal/api/` boundary now also keeps public demos from
leaking commercial state through lifecycle-adjacent surfaces. Agent install,
reporting, and setup flows may share backend helpers with billing or license
transport, but `DEMO_MODE` must continue to 404 commercial read surfaces
instead of teaching lifecycle or mock-mode paths to bypass licensing. Public
demo readiness therefore comes from hiding commercial presentation on the
shared API boundary, not from introducing a second fake-entitlement path into
lifecycle-owned install or reporting flows. Browser-facing lifecycle surfaces
must also treat `/api/security/status` as the canonical public-demo bootstrap
contract. The backend source-of-truth fact remains
`sessionCapabilities.demoMode`, but lifecycle surfaces must consume the shared
resolved `presentationPolicy` instead of inferring demo posture from headers,
`/api/health`, or hostname heuristics.
That same shared API boundary now owns the hidden-versus-runtime-only split as
well: lifecycle-adjacent flows may inherit non-commercial
`/api/license/runtime-capabilities` reads when demo-visible product behavior
needs them, but `/api/license/commercial-posture`,
`/api/license/entitlements`, and `/auth/license-purchase-start` stay hidden in
public demo mode and those lifecycle flows must not depend on licensed
identity, plan labels, upgrade reasons, checkout handoff state, or observed
usage counts surviving the public-demo contract.
That same demo-safe runtime contract keeps monitored-system capacity posture
out of public-preview runtime capabilities. Lifecycle-adjacent install or
reporting surfaces may still depend on demo-safe capability flags, but they
must not expect `monitored_system_capacity`, admission-freeze copy, or
observed plan overage posture to exist.
That shared runtime-capabilities exception may also carry paid-runtime block
records for private Pro runtime hooks. Lifecycle-adjacent flows must treat
those records as executable-runtime identity, not as agent enrollment,
installer success, or lifecycle capability evidence, and must leave private
runtime download guidance to the licensing/settings surfaces that own it.
The same presentation-policy split now governs paid lifecycle extensions in
ordinary self-hosted v6 installs. Agent profile management may remain an
entitled lifecycle surface, but default Infrastructure navigation must not
advertise agent-profile upgrades, trial prompts, or paid helper links while
`presentationPolicy.hideUpgrade` is true; it should stay on the free source
manager unless an explicit entitlement or recovery context makes the paid
lifecycle surface relevant.
The normal Infrastructure installer also follows that contract. Agent-command
execution controls may describe the runtime trust and command-execution effect,
but their default labels and tooltips must not mention Pro requirements or paid
upgrade posture while they are part of the ordinary host-install workflow.
That same demo-hidden API boundary also keeps runtime-admin operations out of
public lifecycle flows: `/api/diagnostics`,
`/api/diagnostics/docker/prepare-token`, and `/api/logs/*` must return `404`
in demo mode instead of exposing runtime bundles, log streams, or diagnostics
payloads through a nominally read-only preview account. `GET` and `HEAD` reads
for `/api/admin/users` and manual discovery at `/api/discover` are part of
that same hidden boundary; lifecycle-adjacent UI must not rely on those routes
remaining discoverable in public demo mode.
Lifecycle-owned browser shells must also defer any commercial helper reads
until that presentation policy resolves so demo suppression stays fail-closed
during first render instead of racing hidden commercial endpoints from shared
setup or install surfaces.
The governed exception is
`frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`: because
that first-run completion surface renders before the authenticated shell has
mounted `frontend-modern/src/useAppRuntimeState.ts`, it may issue the local
commercial posture bootstrap needed for trial and upgrade posture, and it may
force-refresh that posture after a successful trial start. Other
lifecycle-adjacent authenticated-shell surfaces such as
`frontend-modern/src/components/Settings/useNodeModalState.ts` and
`frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts` must
consume the shared posture owner instead of reintroducing their own mount-time
commercial reads.
Even on that governed first-run exception, render-time commercial gating must
not revive trial-status selectors or raw commercial-posture reads inside
`SetupCompletionPanel.tsx`; first-run can link to explicit plan or support
handoff only where presentation policy allows.
That same shared boundary now also owns the one-time checkout-return lookup:
lifecycle-adjacent surfaces may initiate billing or account handoff through
shared public routes, but they must never persist, derive, or replay the
server-owned portal checkout state or owned billing purchase-arrival state as
lifecycle state.
Lifecycle-adjacent storage and fleet surfaces now also depend on one governed
physical-disk history transport. When agent-backed disk telemetry is rendered
through shared drawers or lifecycle-adjacent resource context, those reads
must flow through the canonical `/api/metrics-store/history` boundary and the
disk `MetricsTarget.ResourceID` that monitoring projects for the resource,
rather than reviving a browser-local collector or a lifecycle-only
agent/device identity.
That shared metrics-history boundary may enforce commercial history windows
such as Relay 14-day and Pro 90-day retention for operator charts, but lifecycle
surfaces must treat those windows as presentation entitlements only. Agent
registration, heartbeat, installer status, and fleet freshness must not infer
lifecycle truth from whether a longer chart range is enabled or denied.
That same adjacent API boundary now also owns internal demo-fixture runtime
gating. Lifecycle-adjacent install, reporting, and demo-facing flows may
share mock-mode handlers in dev and test, but release builds must authorize
runtime mock rewiring only through the internal `demo_fixtures` entitlement,
and browser-facing lifecycle surfaces must not infer or persist that internal
grant from public runtime-capabilities or presentation-policy payloads.
Shared workload-chart reads that lifecycle surfaces reuse must stay
presentation-only on that same boundary: `internal/api/router.go` may batch
those reads in parallel, but it must request only the canonical rendered
metric set for workload cards instead of widening the hot path back to
fetch-all metrics on behalf of install or reporting callers.
The same presentation-only rule applies when shared infrastructure-summary or
workloads-summary chart routes serve a short cached response for repeated
org/range/scope requests: lifecycle-adjacent surfaces may render those charts
as operator context, but agent registration, heartbeat, installer status,
profile assignment, reporting freshness, and fleet-control readiness must not
derive authority from the cached chart payload or its timestamp.
That shared `internal/api/` dependency now also assumes hosted tenant AI and
relay bootstrap reads use one effective hosted billing lease before
lifecycle-adjacent flows inspect runtime readiness, so install and setup
surfaces do not observe a tenant-org Pulse Assistant state that disagrees
with the machine-owned hosted entitlement already backing the same instance.
That same shared `internal/api/` dependency now also assumes AI settings stay
vendor-neutral on that boundary. Lifecycle-adjacent setup and infrastructure
surfaces may depend on the shared AI settings transport being available, but
they must not revive host-install or first-run branches that guess provider
model defaults once the backend owns BYOK model resolution from live provider
catalogs.
That same shared dependency now also assumes settings-driven AI enablement can
cold-start the direct Assistant runtime and approval persistence without a
prior chat session. Lifecycle-adjacent mobile pairing and setup flows depend
on `/api/ai/approvals` becoming ready from the first governed settings save,
not only after some earlier process-start or chat-start side effect has
already initialized the approval store.
That same shared dependency now also assumes mock-mode Assistant runtime
startup is AI/runtime-only. When mock mode is active, `internal/api/ai_handler.go`
may enable the Assistant runtime in memory so `/api/ai/chat` can exercise the
typed mock stream without configured providers, but that effective config must
not become agent enrollment, install readiness, command approval authority, or
persisted lifecycle setup state.
That same shared `internal/api/ai_handler.go` dependency now also assumes
Assistant finding handoffs may include a model-only factual finding briefing
from Patrol's unified finding and structured investigation record. Lifecycle-adjacent
setup, install, and fleet surfaces may treat that briefing as adjacent
AI/runtime guidance only; it must not become agent readiness, install status,
or lifecycle execution authority. When that briefing includes recovered
approval lifecycle metadata, action plan identity, approval policy, or dry-run
posture, those fields remain Assistant review context and must not be treated
as an agent command grant, enrollment state, update readiness, or fleet-control
decision.
Patrol run handoffs through that same shared handler and `internal/api/router.go`
remain metadata-driven Assistant review context only. The router may expose a
run-history lookup so AI runtime can rebuild model-only run context, but the
resulting scoped resources, runtime failure summaries, and follow-up session
metadata must not become agent command authorization, reachability proof,
install state, profile assignment, or fleet lifecycle evidence.
Assistant session undo/redo through `POST /api/ai/sessions/{id}/undo` and
`POST /api/ai/sessions/{id}/redo` follows the same adjacent AI/runtime boundary.
The restored prompt, redo availability flag, and restored message count are
conversation repair metadata only; lifecycle-adjacent surfaces must not treat
them as agent enrollment state, command authorization, update readiness, or
fleet-control evidence.
Legacy OpenCode-style Assistant file-change routes under
`/api/ai/sessions/{id}/diff`, `/revert`, and `/unrevert` are not
agent-lifecycle operations. If those routes are called directly, the API must
fail them as unsupported rather than presenting file diffs or reverts as
agent command rollback, enrollment repair, update rollback, or fleet-control
authority.
That same shared dependency now also assumes hosted cloud handoff authorizes
tenant org access before browser lifecycle continues. Lifecycle-adjacent opens
into hosted workspaces may depend on `internal/api/cloud_handoff_handlers.go`,
but the canonical contract is that a successful handoff exchange may continue
only when the handed-off account already has server-owned tenant membership.
The exchange path must derive the effective role from the existing owner/member
record, reject any handoff claim that would upgrade that stored role, and fail
closed when the tenant org still has a blank `OwnerUserID` instead of letting
the first owner-shaped token claim the tenant during browser session minting.
Lifecycle-owned paywalls now also follow the shared commercial navigation
contract. `frontend-modern/src/components/Settings/AgentProfilesPanel.tsx` and
`frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts` may
request the canonical commercial destination from the shared license boundary,
but they must leave internal-versus-external navigation semantics to
`frontend-primitives` instead of hardcoding pricing URLs or tab-open behavior
inside lifecycle-owned settings surfaces.
That same lifecycle-owned settings surface must also keep assistant
availability as an app-shell fact instead of an AI-runtime fetch. Agent
Profiles may read the shared browser bootstrap availability state to decide
whether assistant affordances render, but they must not mount
`frontend-modern/src/stores/aiRuntimeState.ts` or call `/api/settings/ai`
just to decide whether to show assistant-adjacent UI.
That same platform-connections ownership now also includes mock-runtime
continuity for API-backed platforms. When `/api/system/mock-mode` flips a
running server between real and mock data, the canonical TrueNAS and VMware
settings routes must keep surfacing through the same Add infrastructure source
picker and handoff URLs instead of depending on process-start-only wiring or a
mock-only alternate shell.
That same lifecycle-owned mock path now also requires one shared fixture owner
for API-backed platform onboarding. TrueNAS and VMware connection-list payloads
shown in Add infrastructure must be assembled from the canonical
`internal/mock/` platform fixture layer, so settings handoff metadata cannot
drift from the runtime mock inventory and shared storage/recovery context.
That same lifecycle-adjacent mock path must stay graph-first at the shared
`internal/api/` boundary. When lifecycle-adjacent handlers depend on mock
platform inventory or recovery context, they must consume
`internal/mock/fixture_graph.go` and its graph-owned projections instead of
reintroducing snapshot-only or platform-only helper exports.
Lifecycle-adjacent summary chart consumers may still depend on shared
`internal/api/router.go` transport, but any synthetic mock series on that path
must resolve through canonical `resourceType` and `resourceID` identities
rather than lifecycle-local seed prefixes, so platform handoff surfaces do not
see a different recent tail than the runtime mock inventory they describe.
When those lifecycle-adjacent surfaces call `/api/charts/infrastructure`, the
shared `metrics` filter contract must stay authoritative through the backend
batch loader as well, so quickstart or install readouts that only render CPU
and memory do not silently pay for disk/network guest fan-out.
That same hosted continuity contract also applies to the older direct tenant
magic-link path. Lifecycle-adjacent control-plane redirects through
`/auth/cloud-handoff` must preserve canonical account/user/role identity in the
handoff token long enough for the tenant runtime to validate the existing org
membership and derive the bounded effective role before it lands in protected
hosted routes. Direct opens must fail closed on missing membership, blank-owner
orgs, or owner/admin role escalation attempts instead of diverging from the
newer portal exchange path by repairing org metadata on arrival.
Lifecycle-adjacent entry surfaces must also treat public hosted signup as a
server-side identity bootstrap: the signup response cannot expose or define the
owner principal, and follow-on lifecycle access must rely on the stored
organization membership reached through magic-link verification.
That same shared `internal/api/` organization boundary also now assumes
self-hosted org membership is consent-backed rather than manager-written for a
target user ID. Lifecycle-adjacent setup, install, or hosted-entry surfaces
may call `/api/orgs/{id}/members`, but a new user must land in a pending
invitation record and become a real member only after the invited account
accepts through the canonical invitation routes. Owner transfer remains an
existing-member operation on that same boundary; lifecycle-adjacent flows may
not treat an unaccepted invitation or arbitrary `userId` string as a
member-shaped owner target, and they may not complete owner transfer through a
stale ambient browser cookie. The acting owner must re-enter through a fresh
browser session on the shared auth boundary before lifecycle-adjacent surfaces
can permanently reassign org ownership.
That same shared `internal/api/` dependency also assumes telemetry
transparency remains explicitly system-settings-owned. When lifecycle-adjacent
setup or router work touches shared `internal/api/` files, telemetry preview
and install-ID reset routes must keep reusing the canonical system-settings
trust boundary and server-owned telemetry runtime instead of borrowing agent
lifecycle proof or state ownership just because the same router surface moved.
Content-free Pulse Intelligence telemetry rollups under shared `internal/api/`
also remain system-settings, API-contract, and security/privacy ownership:
lifecycle-adjacent surfaces may observe that action-plan, approval,
approved-action-decision, rejected-action-decision, first-party workflow
starter, Pro activation starter, and external-agent usage is summarized, but
they must not reinterpret those anonymous counters as agent enrollment, install
success, recovery scope, or lifecycle state.
External-agent activity may be counted when a narrow API token satisfies the
specific manifest capability scope being called, including read-only
`monitoring:read` context usage. That signal remains API-owned collaboration
telemetry, not evidence that a command agent installed, checked in, or applied
fleet state.
External-agent/MCP readiness may also be true for a non-expired token that
covers any Pulse MCP-published capability scope, but that readiness remains a
collaboration-surface setup signal. Agent lifecycle code must not reinterpret
it as host-agent enrollment, command-agent reachability, or applied fleet
configuration. The manifest shape alone may prove that Pulse can publish the
external-agent contract, but it must not satisfy `externalAgentReady` on
`/api/agent/patrol-control/status` without a current token that covers at
least one Pulse MCP surface capability. The legacy
`/api/agent/operations-loop/status` URL is only a compatibility alias. Missing
external-agent readiness must not downgrade or block first-party Patrol control
completed/resolved proof; the legacy `verified_needs_mcp` state is retained
only as compatibility input and must not become new lifecycle or activation
output.
The Patrol-control status projection at
`GET /api/agent/patrol-control/status` follows the same ownership split. Its
stage labels, next action, Patrol issue evidence count, pending approval count,
contextual collaboration count, governed action count, verified outcome count,
Patrol control starter/completed/resolved proof exposed first through
`patrolControl*` fields and mirrored through `patrolAutonomy*` compatibility
fields,
`proActivationOperationsLoopStarterCount` as legacy entry-point orientation,
legacy completed/resolved/value `proActivation*` aliases, and optional
external-agent readiness
are content-free Pulse Intelligence collaboration signals; lifecycle code must
not reinterpret them as installed-agent proof, command-agent heartbeat,
installer progress, profile convergence, or fleet-control authority.
When that projection lets an aggregate active Patrol finding or pending approval
outrank older completed/resolved loop proof, the result is still operator
orientation only; it is not host-agent liveness, install health, or command
authorization evidence.
The same route may use action lifecycle events to notice recent loop activity,
but its governance and verification counts remain action-governance signals:
`governedActionCount` requires approved or rejected governed-action evidence,
`approvedDecisionCount` and `rejectedDecisionCount` expose that split without
identifiers, and `verifiedOutcomeCount` requires an approved governed action
with verified post-action evidence. Agent lifecycle surfaces must not satisfy
those stages from generic `executing`, `completed`, or `failed` command-agent
state, and must not reinterpret a rejected-only no-execution terminal decision
as command-agent success or failure.
The same verified-outcome predicate backs outbound Pulse Intelligence
approved-success telemetry: a completed action result is not lifecycle proof
unless the approved action also has `VerificationOutcome.Status=verified` or a
canonical verification result that ran and succeeded. Agent lifecycle surfaces
must not count host-agent command completion, installer success, or generic
execution success as Patrol control resolved-loop proof.
The route's four-step operator rollup follows the same boundary: governance
step counts may represent pending approvals before a decision or
approved/rejected decision evidence after one exists, while verification step
counts may represent verified outcomes or terminal rejected decisions. Optional
MCP readiness stays in `externalAgentReady`, not in the operator step list.
Agent lifecycle surfaces must not read those step counts as host-agent
heartbeat, command-agent install state, or fleet-control authority.
Approved action decision telemetry may use action lifecycle events or approved
approval records as its API-owned proof source, but the exported value remains
an anonymous approve/reject journey counter. Agent-lifecycle surfaces must not
read that rollup as command dispatch, command-agent reachability, profile
convergence, install success, or fleet lifecycle state.
Approved execution attempt telemetry may use action lifecycle events as its
API-owned proof source, but the exported value remains an anonymous operations
loop counter. Agent-lifecycle surfaces must not treat that rollup as command
agent heartbeat, installation proof, profile convergence, or fleet lifecycle
state.
Approved action success telemetry may also derive from the same governed action
audit stream, but only as a content-free count of approved actions that reached
completed state with a successful result. The existing approved execution
counter remains attempt-based for compatibility; the success counter must not
export resource identifiers, actor identifiers, command text, command output,
verification details, or host-agent state.
The external-agent recent-use bit is backed by content-free authenticated
agent/MCP route activity for manifest-capable API tokens; it is not an agent
lifecycle heartbeat, install proof, or generic token-last-used proxy.
The MCP adapter recent-use bit is narrower adapter-origin telemetry for
`pulse-mcp` requests on that same shared agent surface. It may distinguish MCP
adapter collaboration from direct external-agent API collaboration, but agent
lifecycle surfaces must not treat it as evidence that `pulse-agent` installed,
checked in, accepted remote config, or executed a command.
That same shared `internal/api/ai_handlers.go` dependency also now assumes
Patrol-specific settings and status expansions stay Patrol-owned. When shared
AI handlers add split scoped-trigger fields, recency labels, or trigger-state
transport for Patrol, lifecycle-adjacent setup and fleet surfaces must treat
those payloads as Patrol-only runtime context and must not reinterpret them as
agent install readiness, enrollment health, or fleet-control state.
Provider preflight diagnostics on the same handler remain AI runtime readiness
context as well: lifecycle-adjacent setup and fleet surfaces may not treat
`provider_auth`, `provider_connection`, model-selection, or provider-settings
recommendations as agent registration, updater trust, or fleet-control health.
That same shared AI handler dependency also assumes direct alert-investigation
execution mode is AI/API-owned. Request-scoped `AutonomousMode:false` and
`RequireCommandApproval:true` on `/api/ai/investigate-alert` are Assistant
action-governance facts, not agent install readiness, command reachability, or
fleet-control capability signals.
Visible `stream_idle` workflow progress on that same legacy Assistant SSE
route, and on `/api/ai/execute/stream`, is likewise Assistant/API transport
liveness only. It must not be reused as agent heartbeat, enrollment progress,
installer status, command websocket liveness, or fleet freshness evidence.
The model-boundary sanitizer that governs how much Assistant infrastructure
context reaches cloud models (credentials and local-only resources always
withheld) is an AI-runtime privacy concern; it is not agent enrollment config,
installer readiness, command reachability, or any fleet-control capability signal.
Patrol finding chat handoffs follow the same ownership split: when
`/api/ai/chat` resolves a `finding_id` into model-only Patrol briefing,
resource, or action context, the backend-enforced `autonomous_mode:false`
clamp is Assistant action-governance, not agent readiness, fleet command
reachability, or enrollment health.
If the same request also carries recognized Patrol product handoff context,
resources, or action references, the API handler may merge only same-finding
secondary model context after backend-refreshed finding context; those merged
references still must not become agent enrollment, install, readiness, or
fleet-command authority.
Scoped Assistant handoffs from alert or incident surfaces may also send
bounded `handoff_context`, `handoff_resources`, and Patrol assessment
`handoff_actions` through `/api/ai/chat` without a `finding_id`;
lifecycle-adjacent surfaces may treat those references as AI/runtime
explanation context only, and must not reinterpret them as agent readiness,
install status, command reachability, enrollment health, fleet execution
authority, or agent command grants.
Those handoffs must remain context-only for the configured model. Lifecycle
code must not convert alert, incident, or Patrol Assistant handoffs into
pre-filled prompts, suggested prompt chips, required tool routes, or agent
lifecycle next-step instructions.
That same shared `internal/api/` dependency also now assumes SSO test and
metadata-preview routes fail closed on validated outbound URL handling.
Lifecycle-adjacent setup and hosted bootstrap surfaces may depend on those
shared helpers, but they must not reintroduce raw URL concatenation,
userinfo-bearing fetch targets, or origin-root OIDC discovery drift when
operators validate identity configuration.
That same lifecycle-adjacent identity validation path now also assumes the
manual SAML test payload preserves the optional `idpSloUrl` alongside
`idpSsoUrl` on the shared API contract, so operators validating hosted
identity before first-user or first-host handoff do not silently lose logout
endpoint validation when they choose manual SAML entry instead of metadata
import.
That same shared `internal/api/` dependency also now assumes post-auth browser
handoff stays on one canonical local redirect builder. Lifecycle-adjacent setup
and hosted bootstrap surfaces may depend on shared OIDC/SAML callbacks, but
they must not reintroduce per-handler `returnTo` shaping that can bypass the
governed local-path validation before success or error query markers are added.
That same shared auth dependency also assumes release builds do not revive the
development-only admin bypass path. Lifecycle-adjacent setup or first-session
flows may still run in non-release developer mode with `ALLOW_ADMIN_BYPASS`,
but release binaries must compile that env override out instead of carrying a
runtime branch that can be reopened after deployment.
That same shared auth dependency now also assumes direct auth probes fail
explicitly without overwriting narrower route-owned failures. Lifecycle-adjacent
setup-token, recovery, and install helpers may depend on shared auth wrappers,
but missing setup-token or API-token-only failures must preserve their specific
response instead of being flattened into a second generic auth body.

Agent lifecycle owns the install/register/update continuity surfaces, but it
does not own unified-resource history or control-plane timeline persistence.
Those runtime changes now travel through the shared API and unified-resource
contracts, which keeps fleet bootstrap and identity continuity separate from
resource-change recording and historical inspection.
The shared API runtime now also exposes unified-resource action, lifecycle,
and export audit reads alongside the enterprise audit surface. That read path
belongs to the API and unified-resource contracts, not to lifecycle ownership,
so the agent-install and registration lane stays focused on fleet continuity
instead of adopting execution-history persistence as a side effect.
That shared audit-read path also now requires the dedicated `audit:read`
token scope instead of inheriting broader `settings:read` access, so
lifecycle-adjacent install and registration surfaces cannot regain enterprise
audit history just by holding general settings visibility.
The connected-infrastructure reporting workspace also now treats API-backed
platform surfaces as platform-connection-managed capabilities, not host-managed
agent extensions. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`,
`frontend-modern/src/components/Settings/useConnectionsLedger.ts`,
`frontend-modern/src/components/Settings/useConnectionRowActions.ts`, and
`frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx` must keep
Proxmox, PBS, PMG, and TrueNAS on the shared Infrastructure API-backed path,
while only machine-installed agent, Docker, and Kubernetes surfaces
participate in host stop-monitoring scope, uninstall commands, and upgrade
actions. That same lifecycle-owned reporting contract now also owns guest-link
truth for agent rows: when a host agent is actually attached to a VM or system
container, the shared connected-infrastructure payload must preserve that
linked guest identity so the top Connections and inventory ledger can stay
scoped to top-level infrastructure instead of rendering guest-backed agents as
peer infrastructure roots.
Those unified audit list endpoints also clamp oversized `limit` requests to
the governed maximum, so audit history stays bounded even when callers ask
for arbitrarily large pages.
The adjacent enterprise audit-log read path now also preserves structured
store-failure codes (`audit_store_busy`, `audit_store_unavailable`) instead of
generic 500s; lifecycle surfaces may share that API layer, but they do not own
or reinterpret audit-store health.
That same shared `internal/api/` dependency also now assumes hosted runtime
websocket upgrades trust the cloud proxy only through explicit tenant
`PULSE_TRUSTED_PROXY_CIDRS` wiring, so first-session handoff and agent-facing
live activity surfaces do not degrade into reconnect loops when a hosted
workspace is opened through the control plane. That proxy-trust boundary must
also reject wildcard trust ranges such as `0.0.0.0/0` or `::/0` at startup,
and agent-adjacent forwarded-header reads must fail closed if invalid wildcard
proxy trust configuration is present.
That same lifecycle-owned command websocket now derives an explicit
same-origin HTTP `Origin` header for `/api/agent/ws` from the canonical Pulse
base URL through `internal/securityutil/websocket_origin.go`, and the agent
receiver must reject missing or cross-host origins before registration.
Runtime command sockets therefore stay on the same fail-closed host/proxy
continuity contract as the browser websocket path instead of accepting
originless upgrades. That same receiver-owned admission path must also cap
concurrent websocket connections per client IP before upgrade so one source
cannot hold unbounded agent command sockets open.
That same shared helper layer also now assumes the Pulse Mobile relay runtime
credential reaches only the explicit backend-owned route inventory, so
lifecycle-adjacent setup and install flows cannot accidentally widen the
paired-device credential just by touching neighboring `internal/api/` routes.
The same shared API runtime now also exposes dedicated unified-resource
timeline reads through `internal/api/resources.go` plus the bundled facet
history read used by the drawer, but those query surfaces remain owned by the
API and unified-resource contracts rather than by lifecycle continuity.
Those timeline reads also accept governed filters for change kind, source
type, and source adapter, and the underlying store owns the filtered counts so
agent lifecycle routing still stays on canonical fleet-continuity ownership
instead of re-deriving resource history locally.
Those dedicated resource timeline and facet reads are also relationship-aware
at the API boundary: lifecycle-adjacent fleet views may consume the direct plus
`relatedResources` history returned by `internal/api/resources.go`, but they
must not rebuild cross-resource timeline joins inside lifecycle-owned routes or
change the direct-only store default used by other callers.
The bundled facet read may also expose the selected resource's canonical
capabilities and relationships for shared drawers, but lifecycle-adjacent
surfaces must treat those fields as API/unified-resource facts rather than
agent-lifecycle-owned install, approval, or topology state.
Agent-host, Kubernetes, and runtime parentage exposed through `ParentID` must
therefore enter shared drawers as facet relationships from
`internal/api/resources.go`; lifecycle surfaces must not rederive those edges
from agent install state, cluster names, or local fleet table grouping.
That same shared `internal/api/` boundary now also exposes a dedicated VM
inventory export route for reporting. Fleet and install surfaces may coexist
with that export, but `internal/api/reporting_inventory_handlers.go` and
`internal/api/router_routes_licensing.go` remain API-owned reporting transport,
not lifecycle-owned inventory or install behavior.
That adjacent reporting transport now also includes a reporting catalog route
whose nested VM inventory definition owns panel copy, performance report
options, export title, column schema, and filename prefixes. Lifecycle-
adjacent install and fleet surfaces may read those facts, but they must not
redefine reporting or inventory schema locally.
That catalog route is intentionally metadata-readable without the
`advanced_reporting` feature gate so locked admin reporting shells can stay on
the same API-owned definition before upsell; lifecycle-adjacent surfaces must
not treat that metadata visibility as permission to execute paid report/export
routes.
That same API-owned performance-report definition also governs transport-side
validation and attachment naming. Lifecycle-adjacent fleet surfaces may depend
on those downloads, but they must treat allowed formats, multi-resource caps,
optional metric/title support, default fallback range windows, attachment
filename stems, and invalid-format validation copy as API-owned reporting
contract rather than mirroring local constants.
That adjacent export contract now also carries canonical Proxmox pool
membership for each VM row. Lifecycle-adjacent install and fleet surfaces may
reuse those current-state facts, but they must still treat the pool column as
API-owned reporting data rather than introducing lifecycle-local guest
inventory assembly.
The same API serializer now also refreshes canonical identity and policy
metadata through the shared unified-resource helper before it returns
resource payloads, so lifecycle-adjacent links keep the same canonical
metadata pass as the rest of the resource API instead of composing local
attach wrappers.
That same shared `internal/api/` dependency now also keeps Patrol runtime
availability explicit as API-owned state. Lifecycle-adjacent setup and install
flows may touch the shared AI handler layer, but they must not collapse a
blocked Patrol runtime back into generic healthy status just because the last
successful summary snapshot was green.
Invalid `sourceAdapter` values are rejected at the API boundary, so the fleet
lane continues to consume only the canonical adapter set rather than
introducing a broader compatibility escape hatch.
That same API boundary now routes the `kind`, `sourceType`, and
`sourceAdapter` query values through the shared unified-resource change
filter parser, so the lifecycle lane keeps the transport contract aligned
with the canonical resource-history model instead of rebuilding filter
normalization locally.
That same shared `internal/api/` boundary now also keeps recovery payload
platform vocabulary canonical at the transport edge. Lifecycle-adjacent
surfaces that deep-link into recovery may still depend on those handlers, but
they must treat response `platform` / `platforms` as API-owned fields and use
legacy `provider` aliases only as compatibility fallback rather than reviving
provider-shaped transport assumptions in fleet flows.
That same lifecycle boundary also relies on canonical Kubernetes pod metrics
targets. Pod-facing drawers may expose `MetricsTarget.ResourceID` only as the
history lookup coordinate, but they must keep the prefixed
`k8s:<cluster>:pod:<uid>` contract and let metrics-history handlers
canonicalize any legacy bare pod ID back onto that key, otherwise pod detail
history and workload summary cards split onto different timelines.
The router now wires the tenant resource state provider during initial setup
when a multi-tenant monitor is present, so tenant-scoped fleet pages do not
trip a missing-provider 500 before the monitor has finished initializing.
The dedicated profile client now also routes list, schema, and validation
parsing through shared response helpers in `frontend-modern/src/api/agentProfiles.ts`,
so profile transport stays aligned with the governed API contract instead of
reintroducing local array or JSON parsing rules.
That same lifecycle-owned install/profile surface now also keeps trial-start
CTA orchestration out of ordinary authenticated self-hosted feature gates.
Agent profile paywalls may describe the entitled lifecycle capability and link
to neutral plan review, but they must not present direct trial-start copy inside
the normal Settings flow or open-code `startProTrial()` branches in each
lifecycle surface. Any paid handoff belongs to explicit plan, hosted,
activation, recovery, or support contexts governed by presentation policy.

The owned backend API surfaces must preserve the exact-release installer
fallback, canonical /api/auto-register behavior, and hosted org install-command
contracts instead of leaving those guarantees implied by generic API ownership.
Those shared auth/security helpers now also own the dedicated
`relay:mobile:access` capability that backs Pulse Mobile pairing. Lifecycle-
adjacent setup and install flows may depend on that helper layer, but they may
only consume the server-owned minting route and the governed compatibility
gates for the mobile runtime endpoints. They must not recreate broader
AI-scoped mobile credentials or invent route-local scope exceptions.
That same lifecycle-adjacent setup path now also depends on the hosted relay
runtime helper inside `internal/api/`. Hosted Pulse Cloud tenants must not
require an operator to visit Settings and manually `PUT /api/settings/relay`
before Pulse Mobile pairing becomes possible. When hosted entitlements grant
relay, the shared backend helper must auto-bootstrap the canonical relay
runtime state that onboarding and relay-status reads consume, while still
preserving explicit operator-owned disablement when a real relay config was
already written.
That same hosted setup boundary also depends on tenant browser sessions staying
canonical after cloud handoff. Lifecycle-adjacent mobile pairing and hosted
admin setup routes may run without local credentials configured, but shared
`internal/api/auth.go` helpers must still honor a valid hosted `pulse_session`
before any API-only token fallback or optional-auth anonymous fallback so
operators can mint relay-mobile credentials and continue onboarding from the
hosted runtime itself even after that tenant has already minted managed API
tokens.
That same lifecycle-adjacent hosted setup path now depends on AI bootstrap
staying explicitly BYOK/local before the first settings write. Hosted
operators may land in Chat, Patrol-backed setup hints, or AI-dependent
remediation surfaces before anyone has visited AI Settings, but the shared
`internal/api/` hosted-AI helper must return the same unconfigured
provider-setup state as self-hosted unless an explicit AI config exists. It
must not fabricate installation activation, quickstart credits, or a
quickstart-backed `ai.enc` from billing state. Historical quickstart grant
fields may stay parseable in billing state for old files, but hosted setup and
pairing must not treat them as active AI inventory while entitlement refresh
rewrites lease-backed plan and capability data.
That same shared entitlement refresh path must also keep hosted effective-org
ownership canonical for lifecycle-adjacent routes: when pairing or relay-mobile
bootstrap arrives scoped to a tenant org with no org-local lease, the refresh
must target the instance-level `default` billing lease and evaluator instead
of persisting a second empty tenant copy. Otherwise hosted pairing falls back
to free-tier behavior even though the machine already carries the paid hosted
lease.
The same setup boundary also depends on canonical org-management privilege
surviving the next step: once the request is scoped to a hosted tenant org,
shared `internal/api/security_setup_fix.go` helpers must allow that org's
owner/admin membership to exercise settings-bound pairing routes instead of
requiring a separate configured local admin username that does not exist on
hosted tenants.
The same setup boundary also owns the dedicated relay-mobile bootstrap read:
once the backend mints the server-owned Pulse Mobile credential, the QR,
deep-link, and validation reads in `internal/api/router_routes_ai_relay.go`
must accept that `relay:mobile:access` scope directly instead of demanding the
broader settings-read privilege that the pairing token was never meant to
carry.
That same adjacent `internal/api/` reporting surface also keeps lifecycle-
adjacent automation on the canonical time-window transport contract. Any setup,
handoff, or scheduled lifecycle flow that triggers performance reports must
treat reporting `start`/`end` values as optional RFC3339 fields owned by the
API contract, with malformed or inverted ranges rejected as
`400 invalid_time_range` rather than silently drifting to a fallback window.
Those same lifecycle-triggered reporting calls must also stay inside the
API-owned `metricType`/`title` limits and the strict multi-report JSON body
rules instead of assuming the backend will coerce malformed payloads into a
best-effort report.
When those lifecycle-adjacent calls fail validation, adjacent automation should
rely on the API-owned error codes rather than message-text heuristics, because
the backend contract owns the reporting validation classification.
The API-backed platform onboarding surface now lives inside the shared
Infrastructure workspace. `InfrastructureSourceManager.tsx`,
`connectionsTableModel.ts`, `InfrastructureWorkspace.tsx`,
`infrastructureWorkspaceModel.ts`, `InfrastructureInstallerSection.tsx`,
`useInfrastructureSettingsState.ts`, `useTrueNASSettingsPanelState.ts`,
`useVMwareSettingsPanelState.ts`,
`proxmoxSettingsModel.ts`, `useInfrastructureConfiguredNodesState.ts`, and
`useInfrastructureDiscoveryRuntimeState.ts` own the fallback
install/direct/reporting operator flow, with `InfrastructureSourceManager.tsx`
plus `connectionsTableModel.ts` as the canonical top-level infrastructure
ledger and the governed add/edit modals as the API-backed add/edit surface.
Operator-facing setup copy should use `Add infrastructure` and source-strategy
language for the shared Infrastructure onboarding path
(`/settings/infrastructure?add=pick`) rather than reviving the standalone
`PlatformConnectionsWorkspace.tsx` shell or the old `Platform connections`
label.
That infrastructure destination now has one canonical mental model:
configured infrastructure sources stay visible on the landing page as the
primary objects the operator manages. The landing table is instance-first, not
type-first: existing connections or agent-backed hosts render inside one
platform-banded systems ledger, each platform section owns its own `Add`
action, and the page does not fork back into a second monitored-systems ledger
below.
Adding infrastructure therefore happens in two governed steps. The
`?add=pick` modal owns grouped source-type selection and may offer
`Detect API platform` as a secondary utility. The `?add=detect` modal owns
probe-driven handoff into `ConnectionEditor.tsx`; its outer shell title and
description must also use API-platform / management-API-endpoint language so
operators do not mistake the probe for Unraid, Docker, or generic Pulse Agent
host detection. Typed add routes jump
straight into the matching credential or install body. The shared Settings
sidebar still owns only the top-level `Infrastructure` destination; movement
between landing, picker, detect flow, add form, and edit form belongs to
explicit actions inside `InfrastructureWorkspace.tsx`, not extra sidebar
entries or body-replacing workspace subtabs.
That same landing/table contract now also owns collection-method phrasing.
`connectionsTableModel.ts`, `useConnectionsLedger.ts`, and
`InfrastructureSourceManager.tsx` must present the same plain-language subtitle (`via platform API`, `via Pulse Agent`, or `via
platform API and Pulse Agent`) from the shared ledger contract instead of
shipping badge-only heuristics that operators have to decode visually.
That same lifecycle-owned platform onboarding boundary must keep API-backed
provider state operationally useful, not CRUD-only.
`useTrueNASSettingsPanelState.ts` and `useVMwareSettingsPanelState.ts` must
surface shared runtime health, poll cadence, discovered contribution summary,
and canonical infrastructure / workloads / storage / recovery handoffs coming
from the saved-connection APIs instead of falling back to provider-local
inference or agent-first setup guidance. Saved-connection retests must use the
server-owned test routes, must allow masked-secret continuity on edit, and
must refresh the shared connection-summary state after a save or retest
completes. `frontend-modern/src/utils/clusterEndpointPresentation.ts` and
`frontend-modern/src/utils/proxmoxSettingsPresentation.ts` remain part of that
same governed lifecycle surface, so endpoint reachability state,
discovery-prefill defaults, and variant copy do not drift into card-local
strings or prefill assembly.
That same platform-onboarding boundary also defines the agent-optional rule
for API-backed platforms. TrueNAS and VMware may surface Assistant control,
diagnostics, configuration reads, and runtime insight through the backend-owned
connection and polling path, but adjacent lifecycle flows must not start
treating a unified-agent install as the required bootstrap for provider-backed
operations.
That same boundary also defines the only acceptable VMware phase-1 path:
`vCenter` under the shared Infrastructure onboarding flow. Lifecycle-adjacent
flows must not invent a VMware-only setup shell, direct-ESXi branch, or
agent-first bootstrap story just because the runtime now has a live VMware
connection panel and poller.
That same platform-onboarding boundary also owns demo/mock continuity for API-
backed settings surfaces. When `/api/system/mock-mode` is enabled, provider
fixtures and their downstream infrastructure/workloads/storage/recovery
handoffs must still read from `internal/mock/fixture_graph.go`, so
operator-facing demos stay coherent across those adjacent product surfaces
without a restart.
That same lifecycle-owned platform-onboarding boundary also owns configured
Proxmox, PBS, and PMG replacement continuity. Node update handlers must pass
the current platform surface into monitored-system admission through the shared
structured replacement selector so host or name edits preserve the intended
slot without reintroducing lifecycle-local matcher closures or empty-estate
fallbacks.
That same shared router boundary must treat infrastructure summary chart
normalization as summary-only presentation transport: long-range chart bucket
shaping may improve operator-facing summary readability, but it must not be
reused as lifecycle freshness, heartbeat, or enrollment-state authority.
That same shared chart boundary may resolve provider-backed workload history
through unified metrics targets, but emitted workload IDs must stay on the
canonical `/workloads` row contract so lifecycle settings, reporting, and
handoff surfaces never depend on provider-native metric keys.
That same lifecycle-owned settings slice now also owns the shared VMware
handoff framing. `InfrastructureWorkspace.tsx`,
`useInfrastructureSettingsState.ts`, and
`useSettingsInfrastructurePanelProps.ts` must surface VMware availability and
connection counts from the same shared infrastructure settings state that owns
the inline VMware credential flow itself, rather than letting adjacent setup
surfaces grow a second VMware availability fetch or a VMware-only handoff
path.
That same infrastructure workspace boundary now also owns the first-run
handoff copy for new operators. `InfrastructureWorkspace.tsx` must keep
platform API inventory and Pulse Agent telemetry explicit in the shared
workspace instead of leaving first-session guidance implicit in generic
settings-shell prose or retreating to one provider's name or one onboarding
mode as the primary story.
That same first-run infrastructure handoff now also owns the instance-first
add flow. `InfrastructureSourcePicker.tsx` must present grouped source types
only after the operator deliberately clicks `Add infrastructure`, and
`ConnectionEditor.tsx` must stay focused on detect-driven handoff and the
selected type's form body instead of reviving a second top-level catalog.
Product grouping belongs to the governed platform-support presentation helper,
not lane-local card lists. Detect utility copy must stay provider-neutral and
operationally plain, and returning from a chosen credential slot to detect
must reset probe input and result state rather than reopening the editor with
stale no-match or detected-product state already rendered. Render-order proof
for that landing belongs to DOM-backed settings tests, not raw source-string
position checks, so lifecycle ownership continues to guard the operator-visible
order after reasonable component extraction or copy refactors.
When that infrastructure workspace needs to redirect operators to the plan-
owned self-hosted commercial surface for billing, license status, or paid
feature activation, it must
consume the settings-owned referral copy from
`frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
instead of carrying workspace-local commercial guidance or reaching back into
generic commercial presentation helpers from the hosted infrastructure route.
Shared licensing routes under `internal/api/` must not retain normal-product
`upgrade-metrics` route names for compatibility. Lifecycle-adjacent settings
and install flows must treat that route family as retired local commercial
analytics, not as a reason to reintroduce default self-hosted upgrade prompts
or local handoff event capture.
That canonical /api/auto-register behavior now also includes hostname/IP continuity:
reruns that arrive through a different canonical host form must reuse the same
Pulse-managed node record and token instead of forking duplicate fleet entries.
That same lifecycle contract also governs the runtime-side Proxmox setup host
selection in `internal/hostagent/proxmox_setup.go`: when the system hostname
resolves to a non-loopback, non-link-local address, the generated Proxmox
registration host must stay on that canonical hostname instead of downgrading
to a route-inferred interface IP. Route-aware IP detection remains the fallback
only when hostname resolution is unusable, so multi-NIC and internal-CA
deployments preserve canonical hostname continuity without losing an IP escape
hatch for non-DNS installs.
That same Proxmox registration boundary must now also let Pulse choose from the
agent's ordered candidate host list instead of blindly persisting the agent's
first preference. Unified Agent setup must send canonical `candidateHosts`
alongside the preferred `host`, and `/api/auto-register` must store the first
candidate that Pulse can actually reach for fingerprint capture from its own
network view so mixed-DNS and split-network installs do not register a host the
server itself cannot use afterward. That same selection path must only persist
`VerifySSL=true` when Pulse actually captured a certificate fingerprint for the
selected host; if every candidate fingerprint probe fails, registration must
fall back to the preferred normalized host with strict TLS disabled instead of
pretending public-CA verification is now safe for a self-signed Proxmox node.
Cluster-member Proxmox agents that match a non-primary endpoint of an existing
cluster source are already registered for lifecycle purposes, even when the
cluster source's primary connection is currently disconnected. Only the primary
configured endpoint may drive disconnected-source repair and token rotation; a
covered member endpoint must not rotate the cluster token just because it can
reach `/api/auto-register`.
That same canonical behavior also includes one auth transport for Proxmox
completion: runtime-side Unified Agent and script callers must send `/api/auto-register`
authentication through a one-time setup token in the request-body
`authToken` field instead of keeping either a header-auth compatibility path
or a long-lived admin-token completion path alive.
That same first-session lifecycle boundary also owns bootstrap-token
recovery: the supported operator path is `pulse bootstrap-token`, and the
runtime may not keep `.bootstrap_token` as an unstructured plaintext secret
file after startup. Canonical persistence must encrypt the bootstrap token at
rest and rewrite any legacy plaintext bootstrap-token file immediately into
the encrypted canonical format on load. Automatic startup logs may surface the
token file path for local recovery, but they must never print the bootstrap
token value itself into stdout, systemd journal, Docker logs, or Kubernetes
pod logs. The validation endpoint for that same bootstrap token must also
rate-limit per client and return an explicit `Retry-After` backoff instead of
offering an unbounded brute-force surface during first-run setup.
That same deploy/install runtime boundary also owns peer-node SSH trust.
`internal/hostagent/commands_deploy.go` must resolve and persist peer host
keys through the managed `ssh_known_hosts` store before any automated deploy
fan-out writes a bootstrap token or runs the installer on a remote node, keep
`StrictHostKeyChecking=yes`, and fail closed on key mismatch or missing-host-
key state instead of downgrading to unauthenticated SSH during install.
That same boundary also owns least-privilege peer deploy execution: when
operators configure a non-root SSH user for deploy fan-out, privileged token
write and install steps must escalate through non-interactive `sudo` on the
remote node instead of hard-coding `root@` for every SSH hop or silently
falling back to a second unaudited privilege path.
That same transport boundary also keeps plaintext Pulse URLs loopback-only.
`internal/securityutil/httpurl.go` owns the canonical Pulse transport
normalization used by `internal/hostagent/agent.go`,
`internal/hostagent/commands.go`, `internal/agentupdate/update.go`,
`internal/dockeragent/agent.go`, `internal/kubernetesagent/agent.go`, and
`internal/remoteconfig/client.go`. Those runtime clients may keep local-
development `http://` or `ws://` only for loopback hosts, but private-network
and remote Pulse URLs must still use HTTPS/WSS. `InsecureSkipVerify` may
relax certificate verification on TLS transport; it must not reopen plaintext
HTTP for private-network updater, websocket, reporting, or remote-config
paths.
That same first-run lifecycle boundary also keeps unauthenticated setup local.
Lifecycle-adjacent quick setup or recovery entrypoints may exist before an
operator has configured auth, but they must stay direct-loopback only and any
recovery token/session path must stay bound to the generating localhost client
instead of reopening auth for all loopback callers.
That same shared `internal/api/` lifecycle boundary also assumes tenant-scoped
resource helpers stay on canonical unified-resource seeds: adjacent fleet and
install surfaces may not revive raw tenant `StateSnapshot` fallback through
shared API resource wiring once `UnifiedResourceSnapshotForTenant` exists.
That same shared `internal/api/` dependency now also includes the monitored-system
ledger support read: lifecycle-adjacent inventory and billing surfaces may
show the counted monitored systems coming from agent-backed infrastructure, but
the shared API helper must expose the canonical unified-resource grouping
explanation instead of rebuilding count reasons from install or registration
state.
That shared ledger read must also preserve canonical grouped system status,
including `warning`, so lifecycle-adjacent operator surfaces do not mislabel
live agent-backed infrastructure as `Unknown` when the unified-resource layer
already resolved a governed degraded state.
That same ledger read now also carries backend-owned status explanation copy,
and lifecycle-adjacent details must render it beside the counting rationale so
operators can interpret warning, offline, and unknown states without inventing
local status semantics.
Those status details are now structured as well: lifecycle-adjacent consumers
must preserve the canonical reason list from the ledger read so operators can
see which grouped source or surface degraded and its canonical `reported_at`
timestamp,
instead of only seeing a generic warning/offline paragraph.
That same ledger read also treats the canonical `latest_included_signal`
object as the freshest included grouped observation. Lifecycle-adjacent
consumers must not label it with generic single-source health wording, and
should use the canonical object when they need attribution for which grouped
surface reported most recently. Retired flat alias fields must not reappear as
parallel lifecycle signal inputs or contract language.
Lifecycle-adjacent workspace copy must also keep the same commercial framing:
infrastructure operations may point operators to Plans for billing, but it
must describe that boundary in license-status and unlocked-capability terms
rather than reviving monitored-system plan limits, legacy agent-allocation
language, or treating the entire destination as the `Pulse Pro` tier page.
That same direct-workspace boundary now also owns the shared customer-facing
error copy for discovery and configured-node actions through
`frontend-modern/src/utils/infrastructureSettingsPresentation.ts`, so direct
Proxmox settings mutations do not drift back to inline toast text inside the
runtime hooks.
That same fleet lifecycle boundary now also owns the shared capability,
status, and inventory presentation helpers that those settings surfaces reuse.
`frontend-modern/src/utils/agentCapabilityPresentation.ts`,
`frontend-modern/src/utils/agentProfileSuggestionPresentation.ts`,
`frontend-modern/src/utils/configuredNodeCapabilityPresentation.ts`,
`frontend-modern/src/utils/configuredNodeStatusPresentation.ts`,
`frontend-modern/src/utils/unifiedAgentInventoryPresentation.ts`, and
`frontend-modern/src/utils/unifiedAgentStatusPresentation.ts` are the
canonical owners for agent capability badges, profile suggestion formatting,
configured-node capability/status badges, monitoring-stopped inventory copy,
and unified-agent status labels. Lifecycle-adjacent settings and inventory
surfaces should extend those helpers instead of reintroducing inline fleet
semantics in panels, workspace models, or reporting hooks.
That same boundary now also assumes canonical resource payloads preserve
shared facet totals through `facetCounts`, so the resource list and detail
surfaces can keep row summaries aligned without re-inferring totals from
consumer-local slices.
That same shared facet bundle now also carries grouped `recentChangeKinds`
counts by canonical change kind, so the lifecycle-adjacent detail surfaces can
report restart, anomaly, and other timeline distribution without rebuilding
timeline math in the browser.
That same shared facet bundle now also carries grouped
`recentChangeSourceTypes` counts by canonical source type, so the
lifecycle-adjacent detail surfaces can distinguish platform events, pulse
diffs, heuristics, user actions, and agent actions without re-inferencing the
provenance mix in the browser.
That same shared facet bundle now also carries grouped
`recentChangeSourceAdapters` counts by canonical source adapter, so the
lifecycle-adjacent detail surfaces can distinguish Docker, Proxmox, TrueNAS,
and ops-helper provenance without re-inferencing the integration mix in the
browser.
Timeline entries surfaced through that same boundary also preserve
`relatedResources` correlation context for non-relationship changes, so adjacent
fleet and install surfaces can link the affected neighbors without trying to
reconstruct correlation context from the raw resource payload alone.
That same shared `internal/api/` boundary now also assumes tenant AI services
stay on canonical Patrol runtime wiring: adjacent fleet and install surfaces
must not revive tenant snapshot-provider bridges through shared AI handler
setup once Patrol can initialize from tenant `ReadState` and unified-resource
providers directly.
That same boundary now also assumes the Patrol-backed recent-changes API
surface reads through the canonical intelligence facade first, so adjacent
fleet and install surfaces do not bypass the shared unified timeline through
the old detector-only handler path.
The Patrol-backed correlation API surface must follow the same canonical
intelligence-facade path, so fleet and install surfaces do not need to know
about the detector directly when they render learned relationship context.
That same canonical /api/auto-register response must stay on one completion
truth: caller-supplied Proxmox credentials complete registration with a
direct-use action, and the runtime no longer preserves a dead pending-secret
placeholder state. That same response
must also stay truthful
about lifecycle state: it may not claim the node is already registered
successfully while local token creation is still outstanding.
That same first-hop lifecycle boundary must validate that response shape
instead of trusting HTTP success alone: runtime-side Unified Agent and installer callers must
require the canonical `status="success"` plus `action="use_token"` response
contract before treating registration as complete.
That same canonical response contract must also carry the runtime-owned
identity truth back to those callers: `type`, `source`, normalized `host`, and
matching `nodeId`/`nodeName` must describe the resolved stored node record, and
installer/runtime-side Unified Agent success reporting must use that returned canonical node
identity instead of the caller's pre-registration `serverName`.
The canonical /api/auto-register response must preserve canonical
node identity: `nodeId` must carry the resolved stored node name rather than
the raw host URL or requested `serverName`, so every live registration caller
stays aligned with saved fleet state.
That same /api/auto-register boundary must also preserve canonical
live-event identity: the `node_auto_registered` WebSocket payload must emit the
normalized stored host plus the resolved stored node name in `name`, `nodeId`,
and `nodeName`, rather than broadcasting raw request fields that can drift
from the saved node record.
That same runtime-side Unified Agent boundary also owns one canonical ingest
name through `internal/api/agent_ingest.go` and `internal/api/router*.go`:
the primary runtime surface is the Unified Agent report/config boundary, while
the `/api/agents/host/*` routes remain compatibility aliases only and may not
re-emerge as the primary lifecycle concept in router state, handlers, or
proofs.
The remote-config side of that same Unified Agent boundary now also carries a
backend-owned desired-config fingerprint. `Monitor.GetHostAgentConfig` must
compute the metadata after profile settings and command enablement decisions
have been merged. `/api/agents/agent/{id}/config` must keep the existing
config `signature` backward-compatible with installed agents by signing the
legacy canonical payload shape only; newer clients validate `desiredConfig` by
recomputing it from the signed command decision and signed settings payload,
restricted to the agent-applied settings key schema. Broader applied-state
reporting remains the next contract gap for managed config: until the runtime
report carries a comparable applied config fingerprint for an assigned managed
desired config, `/api/connections` must surface that desired config metadata as
pending or unknown and must not claim rollout convergence from host report
fields such as `commandsEnabled` or `diskExclude`. The empty default desired
config is different: it may be signed and served for compatibility, but without
a managed command decision or agent-applied setting it is not a rollout and
must project as `configDrift: not-applicable` with a current applied rollout.
Command policy follows the same lifecycle truth boundary: the server desired
command setting is not applied runtime truth until a current agent report
confirms it. `/api/connections` must expose desired command policy, applied
agent truth, effective enforcement, and bounded reason separately so lifecycle
surfaces can show desired-disabled/applied-enabled and
desired-enabled/applied-disabled as drift or attention, and no-report cases as
pending or unknown rather than in-sync.
That same canonical /api/auto-register path must also complete the live
post-registration contract after persistence: it must trigger discovery refresh
and emit the canonical `node_auto_registered` WebSocket payload instead of
stopping at a backend-only save/response path.
That same post-registration discovery update must keep structured error
ownership in discovery runtime state: lifecycle handlers may broadcast the
deprecated string `errors` list only as a compatibility field derived from
canonical `structured_errors`, not as a second live discovery owner path.
That same canonical /api/auto-register path must also accept caller-supplied Proxmox token
completion for confirmed runtime-side Unified Agent or script flows, so live registration
surfaces stay on one governed completion contract instead of inventing a
second explicit-token endpoint outside /api/auto-register.
On the PVE side, only tokens that previously came back through a completed
`source="agent"` or `source="script"` auto-register flow count as reusable
confirmed credentials, so interrupted runs cannot harden a false `use_token`
state from any non-canonical token placeholder.
The canonical setup-script path must stamp that same `source="script"` marker
on /api/auto-register payloads, and canonical registration callers must send
that source explicitly, so confirmed script-created tokens stay distinguishable
from agent-created tokens across later canonical reruns.
That same canonical request contract must also reject any non-canonical source
marker: `/api/auto-register` accepts only `source="agent"` and
`source="script"` so v6 does not preserve arbitrary caller labels as a hidden
compatibility surface.
That same canonical request contract must also reject any non-canonical node
type: `/api/auto-register` accepts only `type="pve"` and `type="pbs"` so
unsupported runtime labels cannot slip through as fake successful fleet
registrations.
That same canonical request contract must also reject non-canonical token
identities: `/api/auto-register` accepts only Pulse-managed
`pulse-monitor@{pve|pbs}!pulse-...` token ids, so v6 does not preserve
arbitrary, cross-type, or non-Pulse-managed token labels as successful
registration state.
That same canonical token identity must also stay deterministic across live
callers: `install.sh`, generated setup scripts, and runtime-side Unified Agent-driven Proxmox
registration must all create the same Pulse-managed `pulse-<canonical-scope-slug>`
token name for a given Pulse endpoint instead of letting one caller drift into
timestamp-suffixed or rerun-local token identities.
The corresponding node setup modal owner is now an explicit shell-plus-sections
surface:
`ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx` composes
`NodeModalBasicInfoSection.tsx`, `NodeModalAuthenticationSection.tsx`,
`NodeModalMonitoringSection.tsx`, `NodeModalStatusFooter.tsx`,
`nodeModalModel.ts`, and `useNodeModalState.ts`.
That same node setup owner also includes
`frontend-modern/src/utils/nodeModalPresentation.ts`, which now owns the
canonical node-type defaults, endpoint/auth placeholders, monitoring coverage
copy, and test-result styling for PVE, PBS, and PMG setup.
That presentation layer remains presentation-only for those API-managed
Proxmox, PBS, and PMG connections. Lifecycle guidance in that settings surface
may explain monitored-system grouping, but monitored-system volume caps are
retired and must not reappear as a modal-local rule or exemption path.
That same deterministic token-identity contract also applies to backend-owned
turnkey Proxmox token creation: generated setup scripts and the password-based
PBS add-node path must derive Pulse-managed token names from the canonical
Pulse endpoint itself rather than request-local `Host` fallbacks, so loopback
or proxy-facing admin requests cannot fork monitor-token identity for the same
Pulse instance.
That same generated setup-script path must now complete registration through
the canonical /api/auto-register contract itself: locally created Proxmox tokens must
be submitted directly on the canonical contract instead of diverging into a
second registration shape.
That same setup bootstrap surface must also keep canonical request handling
aligned across `/api/setup-script-url` and `/api/setup-script`: unsupported
node types may not drift into implicit PBS script generation, and the direct
setup-script route must normalize the supplied host before emitting script text
or rerun URLs so the bootstrap artifact and downloaded script stay on the same
node identity.
That same setup bootstrap surface must also stay owned by one backend bootstrap
artifact builder: `/api/setup-script-url` response fields, setup-token hinting,
download URLs, script filenames, and the generated script's rerun command must
all derive from the same canonical bootstrap contract instead of being rebuilt
as separate handler-local shell snippets.
That same canonical request contract must also keep one-time setup-token auth
on a single field: `/api/auto-register` accepts `authToken` as the governed
request payload key and may not preserve a parallel `setupCode` alias.
That same governed runtime path must also keep its active auth terminology on
setup tokens instead of setup-code residue: `config_setup_handlers.go`,
`config_handlers.go`, and their direct proofs must model the one-time
credential as a setup token in runtime names, logs, and auth failure text.
That same auth failure contract must also fail specifically on the canonical
setup-token requirement: missing `authToken` input on `/api/auto-register` may
not collapse back to a generic authentication message once the route is
governed as setup-token-only.
Shared router auth bypass for that setup-token route remains a handler-ownership
mechanism only: lifecycle-adjacent routes may bypass global auth so they can
return their route-specific setup-token failure, not so unauthenticated callers
can fall through to a successful lifecycle mutation.
That same canonical request contract must also keep field-validation failures
specific: mismatched `tokenId`/`tokenValue` input may not collapse into
generic missing-field output, and other missing canonical fields must return
explicit `Missing required canonical auto-register fields: ...` guidance.
That same owned setup and auto-register boundary participates in the canonical
monitored-system grouping model without commercial volume admission. A new
`/api/auto-register` completion may project whether it dedupes onto an
existing top-level monitored system or creates a new one, but the lifecycle
surface must not block API-backed monitoring on a self-hosted or hosted
monitored-system cap.
That grouping projection must come from the same canonical prospective
monitored-system projection the runtime uses for final grouped counting.
Auto-register may preview its own candidate, but it must not keep a
lifecycle-local counter, drift on source priority, or treat missing grouping
usage as a commercial admission state.
The retired private monitored-system admission policy hook must not return as
a lifecycle-local branch or exemption rule.
When lifecycle-adjacent setup or support surfaces need to explain why a
candidate would count or dedupe, they must consume the shared monitored-system
ledger preview contract rather than rebuilding a second preview model from
setup-local transport fields. `frontend-modern/src/components/Settings/MonitoredSystemImpactPreview.tsx`
is the shared shell for that explanation inside platform-connections settings,
so provider-specific panels must not fork their own monitored-system preview
copy or inline projected-usage rendering.
That shared shell must use neutral count-impact language for ordinary platform
connection previews. Previews must not describe "capacity", finite policy
failures, or raw `current / limit` quota math as the operator-facing mental
model for monitoring.
That same grouping readiness boundary assumes settled canonical usage, not the
first non-nil monitor view. Lifecycle-owned setup or first-host surfaces may
not display counted-system totals as final against a provider-owned
supplemental platform such as TrueNAS or VMware until the monitor has both
seen an initial baseline for every active connection and rebuilt the canonical
store at or after that provider watermark.
That same lifecycle-owned preview surface must keep provider save actions
gated on a successful monitored-system grouping preview. TrueNAS and VMware
settings may not create or update a connection while the preview is missing,
loading, unavailable, or errored, and save-time backend races must reuse the
same canonical unavailable presentation state instead of falling back to
provider-local billing messages.
That same lifecycle-adjacent request contract now also assumes canonical
enablement defaults. New platform-connection preview/test/add payloads must
inherit the provider default `enabled=true` when the field is omitted, while
saved-connection preview/test/update payloads must preserve stored enablement
unless the caller explicitly changes it, so setup surfaces do not accidentally
preview an unchanged active connection as inactive just because JSON omitted a
bool field.
That same validation contract must stay coherent across the public
`/api/auto-register` route and the direct canonical handler path used by the
same runtime surface, so Unified Agent/setup entry points do not inherit divergent
messages for the same missing-field or token-pair failures.
That same canonical caller contract must also require explicit node identity
input from live callers: `/api/auto-register` may not synthesize `serverName`
from `host` once installer, setup-script, and runtime-side Unified Agent callers all send the
canonical field directly.
That same canonical runtime path must also keep overlap and rerun continuity
wording on the canonical `/api/auto-register` contract itself: active runtime
messages and helpers may not preserve the deleted "secure auto-register" split
when describing host-identity, DHCP-continuity, or in-place token-update
matches.
That same canonical runtime path must keep token-completion validation wording
on the canonical contract too: incomplete `tokenId`/`tokenValue` payloads may
not preserve deleted "secure token completion" wording in live handler
messages.
That same migration rule also applies to `scripts/install.sh`: installer-owned
Proxmox auto-registration must keep local token creation in the installer, but
submit the resulting token completion through the canonical /api/auto-register
contract directly as the one supported completion path.
That same shared `scripts/install.sh` boundary must also keep one canonical
runtime-argument builder for the service and wrapper launch flags it persists.
Token-bearing installs, token-file systemd installs, and wrapper-script
launches may not each rebuild their own shell fragment for `--url`, `--token`,
feature toggles, identity flags, or disk-exclude transport; they must all
derive from the same installer-owned argument item list so lifecycle state does
not drift by install path.
That same install/setup boundary must also keep setup bootstrap metadata on one
backend-owned artifact model. Proxmox setup-script downloads, rerun guidance,
and `/api/setup-script-url` responses may not each carry mirrored local struct
definitions for the same bootstrap fields.
That same lifecycle shell transport must also keep one shared render owner for
generated PVE and PBS setup scripts: the handler may validate inputs and choose
the artifact, but the shell body itself must come from shared backend render
helpers rather than an inline handler-local template engine.
Those install and setup-command paths now also preserve the configured
canonical `PublicURL` end to end when the admin session originates from the
local frontend loopback, including the configured HTTPS scheme and path, so
generated commands do not silently downgrade agent reachability to `http://`.
That same backend install-command boundary must also normalize trailing slashes
on canonical base URLs before composing installer asset paths or response
snippets, so `/api/agent-install-command` and the container-runtime migration
token path cannot drift onto `//install.sh` or slash-suffixed `PULSE_URL`
values when `PublicURL` or `AgentConnectURL` is configured with a trailing `/`.
That shared frontend install-command helper must also stay under explicit proof
routing on both sides instead of relying only on downstream consumer coverage:
changes in `frontend-modern/src/utils/agentInstallCommand.ts` must continue to
carry the direct `frontend-install-command-helper` lifecycle proof together
with the API-contract helper proof.
That same shared diagnostics dependency must also preserve canonical
fallback-reason continuity at the API boundary: when
`internal/api/diagnostics.go` serializes monitoring memory-source breakdowns
for lifecycle-adjacent diagnostics surfaces, legacy aliases and empty
fallback-reason fields must still normalize onto the governed canonical reason
contract instead of depending on monitor-owned snapshot accessors to have run
first.
That same shared diagnostics dependency now also assumes local commercial and
onboarding analytics stay out of user diagnostics entirely: lifecycle-adjacent
admin surfaces may consume operational diagnostics, but they must not restore
self-hosted upgrade-metric summaries or infrastructure-onboarding analytics to
`internal/api/diagnostics.go` or the settings diagnostics panel. The retired
local commercial metrics reporting routes must stay absent from the normal
product API and must not become lifecycle setup, install, or fleet-progress
signals.
Lifecycle-adjacent Docker and Podman module diagnostics are part of that same
shared backend dependency. When `internal/api/diagnostics.go` emits module
health notes for Docker and Podman, the copy must keep Infrastructure as the
operator recovery surface and must not send users back to retired
agent-management routes.
Lifecycle-adjacent Docker / Podman management responses are part of that same
shared backend dependency. When `internal/api/docker_agents.go`,
`internal/api/docker_metadata.go`, or `frontend-modern/src/api/monitoring.ts`
surface host removal, hide/unhide, pending uninstall, display-name, or metadata
errors, the operator-facing copy must describe Docker / Podman modules or hosts
rather than reviving generic container-runtime labels or a separate Docker
product identity.
That same shared `internal/api/` dependency now also assumes auth persistence
compatibility is handled as an explicit migration/import boundary: legacy
raw-token `sessions.json` and `csrf_tokens.json` files may load for upgrade
continuity, but `session_store.go` and `csrf_store.go` must immediately
rewrite hashed canonical persistence during load instead of leaving raw-token
files on the primary runtime path until a later save side effect happens to
run.
That same shared `internal/api/` dependency also assumes ordinary self-hosted
commercial-trial acquisition is retired: lifecycle-adjacent setup, install, and
fleet surfaces must not expose direct trial CTAs or depend on
`POST /api/license/trial/start`, and the normal router must fail that path as
`404` without mutating entitlements. The retired `/auth/trial-activate`
self-hosted callback must also stay absent from lifecycle retry and backoff
behavior. Lifecycle-adjacent setup and install surfaces must also treat
`trial_eligible` and `trial_eligibility_reason` as retired compatibility
fields, not as prompt state or setup transport state.
Legacy-named hosted entitlement verifier plumbing under shared `internal/api/`
is boundary-only commercial compatibility, not lifecycle setup state:
agent-lifecycle surfaces may consume the resolved entitlement outcome, but
must not treat `TrialActivation*` names or the retained
`PULSE_TRIAL_ACTIVATION_PUBLIC_KEY` literal as permission to recreate trial
acquisition, setup retry, or install-progress prompts.
That same shared `internal/api/` dependency also assumes session-carried OIDC
refresh tokens stay fail-closed at rest: `session_store.go` may only persist
or recover those tokens through encrypted-at-rest session payloads, and any
missing-crypto or invalid-ciphertext path must drop the refresh token instead
of leaving plaintext-at-rest session state on the lifecycle runtime path.
That same shared `internal/api/` dependency also assumes notification test
handlers stay decode-and-delegate only: `internal/api/notifications.go` may
surface adjacent operator test actions, but service-template selection and
generic webhook-test payload fallback must remain notifications-owned instead
of becoming a second API-layer owner under the shared helper surface.
That same shared API boundary also assumes legacy service-specific webhook
aliases are rewritten at ingress only: `internal/api/notifications.go` may
accept compatibility keys like Pushover `app_token` / `user_token`, but it
must return and forward only canonical `token` / `user` fields so agent-
adjacent shared `internal/api/` surfaces do not inherit a second live alias
contract.
That same shared `internal/api/` dependency now also assumes recovery-token
persistence follows the same rule: raw recovery secrets may be minted for
immediate operator use, but `recovery_tokens.go` must persist only token hashes
and treat any legacy plaintext-token file as a one-time migration input that
is rewritten immediately into hashed canonical persistence on load.
That same shared `internal/api/` dependency now also assumes those auth stores
stay owned by the configured router data path: session, CSRF, and
recovery-token runtime state may not silently bind themselves to hidden
`/etc/pulse` fallback initialization or retain old-path state after a
reconfiguration.
That same shared `internal/api/` dependency also assumes those auth stores
tear down synchronously when lifecycle-adjacent routers or hosted runtimes are
reconfigured: session and CSRF workers may not rely on best-effort background
signals that can wedge teardown, block temp-path cleanup, or leave first-
session and hosted handoff validation hanging behind a stale auth worker, and
each router must retain the exact session, CSRF, and recovery-token workers it
initialized so later global rebinds cannot orphan a live test or hosted-runtime
data path.
That same path-ownership rule also applies to bootstrap-token recovery and
adjacent hosted billing side effects that share the `internal/api/` boundary:
CLI/bootstrap retrieval, webhook dedupe state, and customer-index persistence
must all route through the shared runtime data-dir helper instead of carrying
private `/etc/pulse` fallbacks in neighboring entry points.
That same shared `internal/api/` boundary also assumes manual auth env writes
and first-session status reads resolve the `.env` path through the shared
auth-path helper, so lifecycle-adjacent setup and password flows do not each
reconstruct their own `/etc/pulse/.env` fallback logic.
That same shared `internal/api/` dependency also assumes config import reloads
fail closed without panicking when optional runtime managers are absent.
Lifecycle-adjacent setup, install, and restore flows may invoke the shared
config-import path before every notification or monitoring manager is wired,
but `internal/api/config_export_import_handlers.go` must still rebind the
imported configuration without turning missing optional managers into a fatal
reload path.
The same proof boundary also owns deterministic first-run re-entry for the
managed local backend: integration helpers may use the seeded runtime-state
primary API token to call the dev-only `/api/security/dev/reset-first-run`
route, but they may not recreate auth teardown by deleting files or rebuilding
bootstrap state outside the canonical backend path.
That same shared `internal/api/` boundary also assumes generated developer
warnings do not mis-teach the local runtime split: the embedded frontend notice
under `internal/api/DO_NOT_EDIT_FRONTEND_HERE.md` may point operators to the
shared backend on `:7655` when explaining the proxy relationship, but it must
keep the hot-reload browser entrypoint on `http://127.0.0.1:5173` so lifecycle-
adjacent setup and install guidance does not regress to the backend port.
Those same lifecycle-adjacent setup and password flows must now also route
`.env` writes through the shared writable auth-env helper instead of
re-implementing config-path writes plus data-path fallback ordering inline.
The same agent-lifecycle boundary now also fails closed on profile assignment:
assigning an agent to a non-existent profile must return a not-found contract
instead of persisting an orphan profile reference through the API.
That same missing-profile assignment contract must survive the shared frontend
control surface: `frontend-modern/src/api/agentProfiles.ts` must preserve the
canonical missing-profile message for assignment 404s, and
`AgentProfilesPanel.tsx` and `InfrastructureInstallerSection.tsx` must resync profile state after
that rejection instead of flattening it into a generic assignment failure while
leaving stale profile options visible.
That same shared profile-management boundary must also fail closed on malformed
list payloads: `frontend-modern/src/api/agentProfiles.ts` may not silently
reinterpret non-array profile or assignment responses as an empty state, and
`useAgentProfilesPanelState.ts` / `InfrastructureInstallerSection.tsx` must surface that load
failure instead of pretending no profiles exist.
That same shared profile-management boundary must also fail closed on malformed
profile-object, suggestion, schema, and validation payloads: the shared
`agentProfiles` client may not trust partial profile objects, malformed schema
definitions, or malformed validation/suggestion bodies, and the profile editor
plus suggestion modal must surface those canonical contract failures instead of
flattening them into generic save/delete/schema/validation fallback copy.
That same frontend profile-management boundary now keeps its render shell and
runtime owner separate: `AgentProfilesPanel.tsx` is the surface shell, while
`useAgentProfilesPanelState.ts` owns license gating, AI availability, profile
load/save mutations, assignment resync, and modal form lifecycle so the panel
does not carry a second inline controller.
That same connected profile-assignment surface must also preserve canonical
local operator identity for monitored systems. When governed resources such as
PBS or PMG appear in the assignment list, the panel must keep the local
instance label for ordering and row display instead of substituting governed
summary text, so profile assignment remains instance-specific.
Canonical Proxmox auto-register must also preserve the legacy DHCP continuity
contract: when a node reruns registration from a new IP but presents the
same canonical node name and deterministic Pulse-managed token identity, Pulse
must update the existing node in place instead of duplicating it as a second
inventory record.
That same profile-management UI boundary must also stay on the direct
`agent-profiles-surface` proof path, rather than relying only on the shared
API client coverage to catch lifecycle drift in `AgentProfilesPanel.tsx`.
That same profile-management presentation helper must also stay on that direct
`agent-profiles-surface` proof path, rather than relying only on panel-level
tests to catch lifecycle drift in
`frontend-modern/src/utils/agentProfilesPresentation.ts`.
Shared `internal/api/` recovery transport helpers now also preserve normalized
filter coherence across rollup, point-history, series, and facet views so
agent-adjacent protected-resource drill-downs do not fork between protected
items and history slices under the same active recovery filter set.
That same shared `internal/api/` recovery boundary must also preserve the
canonical provider-neutral `itemType` filter and display contract. When
agent-adjacent recovery data originates from Proxmox, Kubernetes, TrueNAS, or
other platform-native subjects, the shared transport layer must normalize
those source-specific labels onto the governed recovery item vocabulary before
the UI route/filter state sees them, so lifecycle-adjacent drill-downs remain
coherent across platforms instead of reintroducing Proxmox-native subject
types as the de facto recovery model.
That same shared recovery boundary now also treats `platform` as the canonical
operator-facing filter query for lifecycle-adjacent drill-down links. Any
legacy `provider` alias support must remain compatibility-only input behind
the shared API/router layer rather than becoming the route shape lifecycle
surfaces copy back out to operators.
That same lifecycle-adjacent recovery drill-down boundary must also stay on
canonical `itemResourceId` filter and payload vocabulary. When lifecycle
surfaces deep-link into shared recovery handlers or consume recovery payloads,
they should treat legacy `subjectResourceId` only as an API-layer compatibility
alias rather than reviving it as the route or runtime model they expose.
That same lifecycle-adjacent recovery drill-down boundary must also stay on
canonical `itemRef` payload vocabulary. When lifecycle surfaces consume shared
recovery point or rollup payloads, they should treat legacy `subjectRef` only
as an API-layer compatibility alias rather than reviving it as the runtime
item-reference model they expose back out to operators.

The updater/runtime surfaces must preserve the one-shot `updated_from`
continuity handoff and the non-TLS continuity path for supported self-hosted
installs, so upgrade-safe agent behavior does not drift between install,
restart, and reconnect paths.
The v5-to-v6 automatic migration boundary must name its first-hop trust model
explicitly: an already-installed v5 `pulse-agent` may use its v5 updater to
reach v6, but release readiness and operator docs must warn that this hop
depends on HTTPS or trusted local-network transport until the v6 signature and
downloaded-binary self-test protections are installed. High-assurance
environments must be pointed at signed v6 installer reinstall instead of a
plain-HTTP first hop.
That same shared agent transport boundary must not force operators to choose
between public-CA trust and blanket TLS disablement. `cmd/pulse-agent/main.go`,
`internal/hostagent/`, `internal/agentupdate/`, and adjacent remote-config
transport must accept a canonical pinned Pulse server certificate fingerprint
for self-signed deployments, and that pin must flow through reporting,
enrollment, command websocket, remote-config, and self-update transport
instead of widening `PULSE_INSECURE_SKIP_VERIFY` into an all-path MITM
carve-out.
Release-grade updater continuity must also stay fail-closed on signed assets.
When release builds embed trusted update signing keys through
`internal/updatesignature`, `internal/agentupdate/` and
`internal/dockeragent/` must require both `X-Checksum-Sha256` and
`X-Signature-Ed25519`, while installer-owned download flows must also require
the matching base64-encoded `X-Signature-SSHSIG`, and
`internal/api/unified_agent.go` must only serve published release installers
and agent binaries from local or proxied assets that carry the matching
detached signature sidecars.
That same self-update pre-flight must keep the live agent token out of process
argv. `internal/agentupdate/update.go` and legacy
`internal/dockeragent/self_update.go` may pass a short-lived `0600` token file
into `cmd/pulse-agent/main.go --self-test --token-file`, but they must not
revive `--token <secret>` argument passing that exposes the runtime
credential through `/proc/*/cmdline`.
That same unified-agent runtime boundary also owns vendor-aware host identity.
When gopsutil reports generic Linux platform fields on NAS appliances,
`internal/hostagent/` must prefer canonical platform files such as Synology DSM
or QNAP QTS/QuTS version manifests before the first report is built, so
downstream monitoring and alerting do not depend on hostname or display-name
heuristics to infer the real vendor OS.
Proxmox VE host identity follows the same vendor-aware rule: when `/etc/pve`,
`pveversion`, or Proxmox package metadata proves the host is PVE,
`internal/hostagent/` must report `Proxmox VE` plus the PVE product version
when available, while preserving the underlying Linux runtime platform for
runtime/platform-normalization decisions.
That same runtime continuity must stay on direct lifecycle proof routes too:
changes under `internal/hostagent/` must continue to carry the explicit
`unified-agent-runtime` proof, and changes under `internal/agentupdate/` must
continue to carry the explicit `agent-update-runtime` proof, instead of
relying on broad owned-prefix coverage to catch lifecycle regressions in the
Unified Agent runtime and updater boundaries.

The settings/profile surfaces must keep unified v6 agent identity and profile
assignment behavior canonical, rather than falling back to host-era or
module-local assumptions. That includes copied shell install and upgrade
commands in the unified settings surface: privilege-escalation wrappers must
preserve the full installer argument list exactly, so selecting target profile,
token, and command-execution flags cannot be dropped at the last clipboard hop.
That same target-profile continuity must hold for PowerShell transport as well:
when the selected profile enables Proxmox mode, copied Windows install commands
must preserve both `PULSE_ENABLE_PROXMOX` and `PULSE_PROXMOX_TYPE`, and
`scripts/install.ps1` must persist those flags into the managed service
arguments instead of silently collapsing back to generic host monitoring.
The same lifecycle ownership now also covers manual node setup command
presentation in the extracted node setup surface
(`ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`,
`NodeModalSetupGuideSection.tsx`, `nodeModalModel.ts`, and
`useNodeModalState.ts`): the copied PVE permission snippet must stay
aligned with the canonical backend setup script, including comma-joined
privilege transport and non-destructive `PulseMonitor` role updates, instead
of shipping a stale local fork.
That same node setup modal owner must also route Proxmox agent-install command
generation through the canonical `NodesAPI.getAgentInstallCommand` client for
both PVE and PBS, instead of mixing client-mediated and ad hoc raw POST
transport for the same backend lifecycle command surface. That same settings
surface must consume the shared validated response uniformly for both node
types, surfacing canonical install-command errors inline instead of collapsing
one pane back to generic notification-only failure.
That same node setup modal owner must also route Proxmox quick-setup command
generation and manual setup-script download through canonical `NodesAPI`
helpers for both PVE and PBS, preserving the shared setup-token and expiry
contract instead of letting one node type drift onto a raw fetch-only path.
That same node setup modal owner must also stay on the direct
`node-setup-settings-surface` proof path across
`ConnectionEditor/CredentialSlots/NodeCredentialSlot.tsx`,
`NodeModalAuthenticationSection.tsx`, `NodeModalBasicInfoSection.tsx`,
`NodeModalMonitoringSection.tsx`, `NodeModalSetupGuideSection.tsx`,
`NodeModalStatusFooter.tsx`, `nodeModalModel.ts`, and `useNodeModalState.ts`,
rather than relying only on broad lane ownership or downstream command tests
to catch lifecycle drift in the settings surface.
That same Proxmox lifecycle transport now explicitly includes the shared
`frontend-modern/src/api/nodes.ts` client boundary itself: changes to setup
command or install-command request transport must carry both lifecycle proof
and the shared API contract instead of staying implicit behind downstream
consumer tests alone.
That same lifecycle ownership also covers the setup completion preview's copied Unix
install handoff in `SetupCompletionPanel`: the first-session install snippet must use the
same shell-safe URL/token quoting, `curl -fsSL` failure behavior, and
root-or-sudo privilege wrapper contract as the governed unified install
surface instead of carrying a stale inline transport variant.
That same setup-completion install transport must also preserve the canonical
plain-HTTP continuity rule: when the configured Pulse URL is `http://`, the
copied Unix install command must carry `--insecure` through the shared host
install command builder instead of bypassing the lifecycle transport contract
with local inline shell assembly.
That same Unix install-command contract also governs backend-generated Proxmox
install transport in `internal/api/agent_install_command_shared.go`: the
canonical `/api/agent-install-command` and hosted Proxmox install-command
surfaces must emit the same root-or-sudo privilege wrapper already required by
the shared frontend Unix builder, instead of returning a raw `| bash -s --`
pipeline that drifts from the lane's governed install shape.
The same lifecycle shell-transport contract also applies to the diagnostics
container-runtime migration install command in `internal/api/router.go`: that
response must emit the canonical `--enable-host=false` flag and the governed
root-or-sudo wrapper, rather than falling back to the stale `--disable-host`
alias or a raw `curl | sudo bash` pipe that drifts from the managed install
surface.
That same diagnostics migration command must stay on the shared backend
install-command helper path in `internal/api/agent_install_command_shared.go`,
rather than rebuilding a local shell formatter in `router.go`, so optional
token omission, plain-HTTP `--insecure`, trailing-slash normalization, and the
governed privilege wrapper stay aligned with the rest of the lifecycle install
surface.
That same lifecycle shell transport also governs the quick setup command
returned by `/api/setup-script-url`: `config_setup_handlers.go` must emit a
shell-quoted `curl -fsSL` fetch for the generated setup script, and the
token-bearing and tokenless variants must come through a shared helper instead
of open-coding a stale `curl -sSL` pipeline in the handler.
That same bootstrap route must also stay on one canonical request shape:
`/api/setup-script-url` accepts a single JSON object with only the supported
request fields, and the handler must fail closed on unknown fields or trailing
JSON instead of tolerating typo-compatible or concatenated payloads.
That same request contract also keeps backup-permission semantics explicit:
`backup_perms` / `backupPerms` is a PVE-only bootstrap option, and both
`/api/setup-script` and `/api/setup-script-url` must reject it for PBS instead
of quietly carrying a no-op flag through the canonical setup surface.
That same bootstrap request boundary must stay canonical on host identity too:
`/api/setup-script` no longer generates placeholder-host scripts for later
repair, and both setup routes must reject missing `host` input instead of
minting artifacts that can only fail closed after download.
That same request boundary must stay canonical on Pulse identity too:
`/api/setup-script` no longer reconstructs `pulse_url` from the request-local
origin, and both setup routes must require the explicit canonical Pulse URL
that the rest of the bootstrap envelope already carries through `url`,
`command*`, and downstream auto-register state.
That same bootstrap boundary must now also stay canonical on identity: the
request must carry a supported `type` and non-empty `host`, the backend must
normalize that host before minting the one-time setup token, and both
installer-owned and runtime-side Unified Agent callers must validate the returned
bootstrap `type`, normalized `host`, and live `expires` metadata before they
trust the returned `setupToken`. That consumer-side validation must fail closed
on already-expired bootstrap responses rather than treating any non-empty
`expires` field as usable. That same `/api/setup-script-url` request boundary
must also stay truthful about auth: setup tokens only bootstrap the later
`/api/setup-script` and `/api/auto-register` flows, while the setup-script-url
request itself remains a normal authenticated request once Pulse auth exists.
Those same installer-owned and runtime-side Unified Agent callers must also require the
full canonical bootstrap artifact, including token-bearing `downloadURL` and
masked `tokenHint`, so they do not keep accepting an older reduced setup-token
response shape after the runtime and shared settings client have moved to the
full envelope.
The shared settings/frontend consumer in `frontend-modern/src/api/nodes.ts`
must stay on that same canonical bootstrap contract too, normalizing and
validating the returned setup-script-url identity fields instead of exposing a
raw JSON passthrough to `NodeModal` and related quick-setup surfaces. That
shared frontend consumer must also reject already-expired setup-script-url
responses instead of treating any positive `expires` value as sufficient, and
it must validate the returned `setupToken` without retaining that raw secret
beyond the shared client boundary.
The extracted node setup modal owner must then consume that canonicalized
response directly,
including copying the token-bearing `commandWithEnv` field while rendering the
non-secret `commandWithoutEnv` preview instead of re-interpreting the
bootstrap payload through local nullable fallbacks.
Operator-facing quick-setup display must also stay on the runtime-owned token
boundary: the shared frontend client must require masked `tokenHint`, and the
extracted node setup modal owner must render that hint rather than the full returned
`setupToken` once the bootstrap artifact itself already carries the live
secret. That non-secret preview contract applies to both the PVE and PBS
quick-setup panes; the settings surface may not let one path keep rendering
the token-bearing command after the other has switched to the governed
`commandWithoutEnv` preview. Operator guidance on those panes must stay
truthful too: once the visible UI only shows a masked hint, copy-success text
may not instruct the operator to paste a token "shown below" and must instead
state that the copied command already embeds the one-time setup token. The same settings quick-setup surface must also trim and validate the Endpoint URL
before manual setup-script download, so download and copy paths stay on the
same canonical host-input contract. That same manual download path must also
stay on one shell-script artifact contract: `/api/setup-script` responses must
ship with canonical `text/x-shellscript` attachment headers and deterministic
`pulse-setup-*.sh` filenames, while `frontend-modern/src/api/nodes.ts` and the
extracted node setup modal owner must validate and use the returned content type and filename
instead of inventing local text/plain download metadata.
Manual download must also stay non-interactive without re-exposing raw setup
tokens in UI state: `/api/setup-script-url` must return a dedicated
token-bearing `downloadURL`, and the shared frontend client plus the extracted node setup modal owner
must use that runtime-owned download artifact instead of fetching the plain
script `url` and then relying on a separately displayed token value.
That same settings quick-setup surface must also treat `/api/setup-script-url`
as one canonical bootstrap artifact per active host/type/mode: copy and manual
download must reuse the returned `url`, `downloadURL`, `scriptFileName`,
`commandWithEnv`, `tokenHint`, and `expires` until the artifact expires or the
operator changes the endpoint, instead of re-fetching and rebuilding a second
local download path or caching the raw setup token past the shared frontend
client.
That same public/operator guidance must also describe that canonical bootstrap
artifact truthfully: API docs and Proxmox/PBS setup guides may not fall back to
stale raw `curl -sSL ... | bash` examples or omit the returned bootstrap
artifact fields once the runtime and settings surfaces are contractually using
`url`, `scriptFileName`, `command*`, `setupToken`, and `expires`.
That same bootstrap response boundary must also own the setup-script filename
before download happens: `/api/setup-script-url` must return the canonical
`scriptFileName`, and the settings quick-setup surface must use that runtime
metadata for operator guidance instead of hardcoded PVE/PBS script names that
can drift from the downloaded artifact.
That same setup-token bootstrap response must also stay coherent for the
non-frontend consumers: the runtime-side Unified Agent and installer Proxmox registration must
reject missing or mismatched canonical `url`, `scriptFileName`, `command`,
`commandWithEnv`, and `commandWithoutEnv` fields instead of consuming
`/api/setup-script-url` as a token-only response.
That same quick-setup transport must also preserve the governed root-or-sudo
continuity used by the install surface: `/api/setup-script-url` commands must
execute `bash` directly when already root and fall back to `sudo` otherwise,
including preserving `PULSE_SETUP_TOKEN` through the sudo path instead of
assuming operators are already in a root shell.
That same transport rule also applies to the generated PVE and PBS setup
scripts themselves: operator-facing retry and off-host rerun guidance printed
by `HandleSetupScript` must advertise the same fail-fast `curl -fsSL` fetch
shape instead of drifting back to stale `curl -sSL` examples inside the script
body.
That embedded guidance must preserve the same root-or-sudo continuity too, so
the script body does not hand operators a direct-root-only retry command after
the API response itself already supports both execution paths.
That same retry guidance must also preserve `PULSE_SETUP_TOKEN` continuity
through both the direct-root and sudo paths, so reruns from the generated PVE
and PBS setup scripts stay on the same non-interactive setup-token contract
instead of silently falling back to an interactive prompt.
That same rerun-token contract must also hydrate `PULSE_SETUP_TOKEN` from any
embedded setup token before the script prints rerun guidance, so generated
PVE/PBS scripts issued with canonical `setup_token` transport do not drop back
to prompt mode on the next hop.
That same setup-script bootstrap boundary must keep one token name end to end:
`/api/setup-script` accepts only the canonical `setup_token` query when a token
is embedded into the script payload, and the rendered PVE/PBS script body uses
only `PULSE_SETUP_TOKEN` instead of lane-local alias variables.
The same generated PVE setup-script boundary must also preserve cleanup
continuity for discovered legacy tokens: when the script offers to remove old
Pulse tokens from the same server scope, it must iterate the actual discovered
`pve` and `pam` token lists instead of falling through an undefined placeholder
loop variable that turns cleanup into a no-op. That discovery path must also
reuse the canonical Pulse-managed token prefix for the active Pulse URL, while
still matching legacy timestamp-suffixed variants, instead of rebuilding a
lane-local IP-pattern guess that drifts from `buildPulseMonitorTokenName`.
The generated PBS setup-script boundary must preserve that same cleanup
discovery contract instead of keeping a separate IP-pattern matcher for old
token cleanup.
The same generated PVE setup-script boundary must also preserve
Proxmox-managed `/root/.ssh/authorized_keys` symlinks when lifecycle setup or
removal touches Pulse-managed temperature-monitoring SSH keys: scripts must
resolve the real authorized-keys target before filtering `# pulse-` entries and
must use that resolved path for both install and uninstall edits.
That same generated PVE setup-script boundary also owns the forced-command
shape for temperature-monitoring SSH keys: new keys must execute the
Pulse-owned `/usr/local/sbin/pulse-sensors` wrapper, not raw `sensors -j`, so
lifecycle setup can collect CPU sensor data and SMART disk temperatures through
one bounded JSON payload while preserving old forced-key compatibility in the
runtime collector.
That same generated setup-script boundary must also use exact token-name
matching when it decides whether to rotate an existing Pulse-managed token, so
reruns do not treat partial-name collisions as the canonical managed token.
The generated PBS setup-script branch must also keep token-copy guidance
truthful: it may only print the one-time token-copy banner after token creation
has actually succeeded, not ahead of a failure path that produced no token.
That same generated PBS setup-script branch must also keep auto-register
attempt guidance truthful: it may only print the attempt banner on the real
request path, after token-availability and setup-token gating are resolved,
rather than before a skip branch that never sends a registration request.
That same rerun path must also preserve the backend-owned encoded setup-script
request URL: embedded `SETUP_SCRIPT_URL` values in generated setup scripts must
keep the canonical `host`, `pulse_url`, and `backup_perms` query contract
instead of rebuilding a lossy raw query string inside the shell.
That same off-host fallback path must not invent a second manual token-creation
workflow either: when the script is run outside a Proxmox host, it must direct
the operator back to rerun on the host through the canonical generated command
instead of teaching a separate `pveum` + Pulse Settings flow that can drift
from the backend-owned lifecycle contract.
That same runtime boundary must also preserve canonical privilege guidance when
the script is launched directly: generated setup scripts may not fall back to
the stale "Please run this script as root" wording, and must instead use the
same root requirement language already carried by the governed retry wrapper.
That same manual-follow-up surface must also preserve one canonical token
placeholder contract across its adjacent branches: generated PVE and PBS setup
scripts may not drift between "[See above]", "Check the output above...", and
other local variants when the token value is only available in prior output.
That same completion boundary must also preserve one canonical success message
across generated PVE and PBS setup scripts, so identical successful
auto-register outcomes do not surface different node-type-specific wording for
the same finished lifecycle state.
That same auto-register boundary must also fail closed when token extraction
fails after token creation: generated PVE and PBS setup scripts may not
continue into prompt or request assembly with an empty token value, and must
instead stop on the canonical "token value unavailable" branch before any
registration attempt is formed.
That same PBS auto-register path must also report skipped states truthfully:
when setup-token input is absent or token extraction never produced a usable
secret, the script may not relabel that skip as a failed request before
success confirmation.
The generated PVE and PBS setup scripts must also fail closed on
auto-register success detection: their runtime branch may only treat a
response as successful when it contains an explicit `success:true` signal,
rather than any broad `success` substring match that could misclassify
`success:false` payloads as a completed registration.
That same auto-register path must also fail closed on HTTP and transport
errors: the generated scripts must use fail-fast `curl -fsS` transport and
gate success parsing on a successful curl exit code instead of interpreting
arbitrary error output as a registrable response body.
That same generated setup-script boundary must also preserve setup-token
messaging continuity: when auto-register authentication fails, operator
guidance must point back to the one-time Pulse setup token flow rather than
telling the user to provide or validate an API token that this script path no
longer uses.
That same auth-failure guidance must also stay truthful once the generated
setup script has already sent a registration request: it may not branch back
into a missing-token explanation after the request path proves a setup token
was present, and must instead direct the operator to mint a fresh setup token
from Pulse Settings → Nodes and rerun. That same auth-failure state must also
block the later manual-details footer, so the script does not immediately
contradict itself by offering manual completion with the current token details.
That same completion boundary must also preserve outcome truth: generated PVE
and PBS setup scripts may only claim successful Pulse registration when
auto-register actually succeeded, and must otherwise present the result as
token setup plus manual registration follow-up instead of announcing a false
successful onboarding state.
That same manual-follow-up path must also stay on the canonical node-add
contract: generated setup scripts may not redirect operators onto a stale
secondary registration-token rerun flow, and must instead point them to finish
registration with the emitted token details in Pulse Settings → Nodes.
That same manual-follow-up path must also keep its failure summary on that
canonical node-add contract: generated setup scripts may not fall back to
vague "manual configuration may be needed" copy when the emitted token details
already define the exact Pulse Settings → Nodes completion path.
That same manual-follow-up path must also preserve the canonical node host
identity already in scope for the script, rather than falling back to a stale
placeholder host string that forces the operator to reconstruct the node
address by hand.
That same host continuity rule applies to PBS as well: generated setup scripts
may not replace the requested canonical PBS host with runtime-local interface
discovery in the manual-add footer, because DHCP or multi-NIC nodes can make
that fallback diverge from the host the operator actually intended to register.
That same PBS host continuity must survive auth-skip and token-skip fallback
branches too: the generated script must bind the canonical PBS host before any
auto-register gating that can short-circuit into manual completion, so the
manual footer never emits a blank or lost host URL when setup-token input is
missing.
That same fail-closed host rule must also apply when the script never received
any canonical host at all: generated PVE and PBS scripts may not fall back to
placeholder host values in manual completion and must instead direct the
operator to regenerate the script with a valid host URL.
That same PBS host binding must exist before token-creation failure fallback as
well, so the final manual footer still preserves the canonical requested host
even when the script cannot mint a usable PBS token and never reaches the
auto-register branch at all.
Generated PVE and PBS setup scripts must also fail closed on token-creation
failure truth: if Proxmox token minting fails, the script may not continue into
fake manual token details or claim token setup completed. It must skip
auto-register, surface the token-creation failure explicitly, and direct the
operator to rerun after fixing the node-local token error.
That same failure-truth contract also applies to token extraction errors after
creation output is returned: generated setup scripts may not tell operators
that manual registration might still work from that broken output. They must
keep the flow on rerun-after-fix guidance until a usable token value actually
exists, and the final completion footer/manual-details branch must key off that
usable-token state rather than raw token-create success.
That same manual-follow-up path must also preserve canonical Settings-surface
language across both PVE and PBS setup scripts, so the operator is always
directed back to Pulse Settings → Nodes with the emitted token details instead
of drifting onto lane-local wording for one node type.
That same canonical path must also hold inside the immediate auto-register
failure branch itself, so generated scripts do not fall back to a shorter
"Pulse Settings" variant before the final manual-completion footer repeats the
correct Settings → Nodes destination or diverge into a separate numbered
manual-setup detour instead of reusing the same "use the token details below"
completion contract. That includes transport/request failures before the
backend ever returns a response body, not just explicit error payloads.
That same `SetupCompletionPanel` transport must also preserve the governed self-signed
and private-CA continuity controls used by the shared lifecycle command
surface: the first-session setup-completion install handoff must pass explicit
`--insecure` and `--cacert` choices through the shared Unix install builder so
the very first installer fetch and the installer runtime stay aligned with the
same transport contract as `InfrastructureInstallerSection.tsx` and
`useInfrastructureOperationsState.tsx`. In explicit insecure mode, that
means the outer `curl` fetch must widen to `-kfsSL` instead of preserving
strict TLS until `install.sh` starts.
That same first-session install surface must also preserve canonical
agent-to-Pulse addressing, not just browser-local origin: `SetupCompletionPanel` must
default to the governed security status `agentUrl` when available and allow an
operator override for agent connectivity, so setup-completion commands do not
silently hand out loopback or wrong-origin install transport.
That same first-session surface must also preserve Windows install parity:
`SetupCompletionPanel` may not stop at Unix-only shell transport while claiming Windows
coverage. Its PowerShell install command must route through the shared
transport helper so URL, token, insecure-TLS, and custom-CA behavior stay
aligned with `InfrastructureInstallerSection.tsx` and
`useInfrastructureOperationsState.tsx`.
That same first-session setup-completion surface also owns the operator's v6
mental model for Unified Agent onboarding: `SetupCompletionPanel` must teach that one
Unified Agent install creates one canonical Pulse system resource first, then
layers workload discovery and API-linked platform context onto that same
inventory. It may not present Docker, Kubernetes, Proxmox, or TrueNAS as
competing primary onboarding paths, nor fall back to logo-led feature
brochure copy that obscures the unified-resource contract the wizard is
supposed to introduce.
That same onboarding/install guidance must also preserve the simple fleet
mental model for clustered and API-backed systems: platform connections own
cluster or appliance inventory, while Pulse Agent remains the low-overhead
per-machine install path for full node-local telemetry. Settings and first-run
install copy may recommend installing the agent on every machine that needs
temperatures, SMART disk data, services, Docker, or Kubernetes telemetry, but
they may not imply that API-backed cluster visibility or best-effort peer
augmentation is equivalent to a local agent install on that machine.
Docker onboarding copy must also distinguish the standalone Docker / Podman
install path from Docker inside Proxmox LXCs: standalone runtimes use the
Docker runtime profile on that host, while Proxmox LXC Docker inventory uses a
Proxmox VE node agent with command execution plus explicit server-side
`PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true`, and the copy may not imply
that installing an agent inside every LXC is required. The Docker empty state,
infrastructure installer, and Proxmox node setup guide must all preserve that
same distinction.
That same connected-systems summary must preserve canonical local operator
identity for newly connected infrastructure. When governed resources such as
PBS or PMG appear in the setup-completion poll, the surface must show their
local instance labels instead of replacing those identities with governed
summary text, so the operator can tell which system actually connected.
That same first-session setup-completion surface must also honor the lane's
optional-auth install contract: when Pulse does not require API tokens, the
wizard may switch to tokenless install commands only after an explicit operator
confirmation, but it must preserve the generated token by default and keep that
explicit token path available instead of collapsing onboarding into tokenless-only
transport. Once the operator has explicitly chosen tokenless mode, repeated
wizard copies must preserve that tokenless choice instead of silently rotating
back onto token-auth transport after the first clipboard action. That same
tokenless choice must also survive the wizard's background "agent connected"
token-rotation path: new agent arrivals may not regenerate a token or flip the
surface back to token-auth mode while explicit tokenless onboarding remains the
active contract.
That same wizard boundary must also keep its credentials drawer and exported
credentials file aligned with the current rotated install token, rather than
continuing to display or download the stale bootstrap token after the install
command surface has already moved on. It must do that without erasing the
stable bootstrap admin API credential: the wizard needs to preserve both the
admin token and the current rotated install token as separate operator-visible
surfaces instead of collapsing them into one mutable credential slot. The saved
credentials handoff must also preserve the current agent-install URL and the
matching install command shape for both Unix and Windows onboarding, so
exported first-session material cannot drift back to browser-local login
context or Unix-only transport while the live wizard command surface is using
a governed or operator-overridden agent endpoint. When the operator explicitly
confirms tokenless optional-auth mode, those same credential surfaces must stop
claiming a current install token and instead present tokenless install mode as
the active onboarding contract. The primary install guidance text in the wizard
must switch with that mode as well: tokenless onboarding may not keep
advertising automatic token rotation after each copy once the active transport
is explicitly tokenless.
The same first-session contract now also owns the landing handoff after secure
setup: RC-proof and helpers must treat direct navigation into
`/settings/infrastructure?add=pick` as the canonical completion path, rather
than assuming an agent-only install landing or the legacy dashboard-only
landing still defines successful wizard completion.
For already-connected completion states, the `Open Infrastructure` action must
return through the shared infrastructure workspace route
`/settings/infrastructure`, not the root app shell or a dashboard alias. The
source-picker and agent-install actions remain the explicit first-source
handoffs at `/settings/infrastructure?add=pick` and
`/settings/infrastructure?add=agent`, so setup-completion tests must assert the
route helpers rather than allowing a generic home-route completion.
That same `SetupCompletionPanel` boundary must also stay on the direct
`setup-completion-source-picker-surface` proof path, rather than relying only
on shared helper coverage or downstream install tests to catch lifecycle drift
in the setup completion surface.
That same first-session browser proof must also exercise the explicit
`Install Pulse Agent` secondary action through the real setup wizard flow,
rather than relying only on the preview route or prose-level assertions to
represent the agent-managed alternative.
The same ownership also covers manual install fallback in the infrastructure
settings surface: active and ignored Connected infrastructure rows must now
come from the backend-owned `connectedInfrastructure` projection instead of a
frontend-local merge of raw unified-resource facets and removed runtime arrays,
and v6 clients no longer treat those removed runtime arrays as a parallel
settings contract, so lifecycle scope and reconnect behavior stay canonical
across host, Docker, and Kubernetes reporting.
deploy results surface: `ResultsStep` must request the canonical backend
install command from `/api/agent-install-command` for failed deploy targets
instead of rebuilding a local shell snippet that can drift from the governed
installer contract. That fallback surface must consume the shared validated
`NodesAPI.getAgentInstallCommand` response, so malformed backend payloads fail
closed and the raw backend install token stays inside the shared client
boundary rather than leaking into deploy UI state.
Deploy wizard target tables are lifecycle-owned presentation surfaces:
`CandidatesStep`, `ConfirmStep`, `PreflightStep`, `DeployingStep`, and
`ResultsStep` must use the shared frontend `Table` primitive for scroll and
table semantics instead of raw table markup or step-local scroll frames.
Deploy selection and retry UI must not consume retired monitored-system
capacity boundaries. Lifecycle UI must avoid workspace-capacity, legacy
license-slot, and plan-upgrade language in deploy confirmation, preflight, or
retry surfaces.
That same deploy wizard boundary must also stay on the direct
`deploy-fallback-install-surface` proof path, rather than relying only on the
shared install helper or downstream deploy tests to catch lifecycle drift in
the infrastructure fallback surface.
The same Windows install, upgrade, and uninstall copies must also preserve
operator-selected transport and capability toggles: if the settings surface
enables insecure TLS mode or Pulse command execution, the PowerShell path must
carry `PULSE_INSECURE_SKIP_VERIFY` and `PULSE_ENABLE_COMMANDS` through to the
installer where those settings apply, so Windows agents do not diverge from the
governed shell transport.
That same command-enabled lifecycle path must still enforce the shared command
policy on the agent itself. `internal/hostagent/commands.go` may accept the
installed command-execution capability, but it must re-evaluate
`internal/agentexec/policy.go` immediately before `sh -c`, reject
`PolicyBlock` commands regardless of caller, and require a consumed approval
identifier before executing any `PolicyRequireApproval` command so a missed
control-plane gate cannot silently turn into host-level RCE. The single
exception is commands marked `Trusted` in the wire payload: those originate
from a vetted Pulse-internal subsystem whose command catalog is hardcoded
in Pulse source (today only the servicediscovery deep scanner, which wraps
read-only inspections in `docker exec`). `PolicyBlock` still applies to
trusted payloads. The `Trusted` field must never be set by a code path that
deserialises user input or accepts a caller-supplied command string.
That same copied install transport must also normalize canonical base URLs
before composing installer asset paths: when operators enter a trailing-slash
Pulse URL, shell and PowerShell install commands must trim it before appending
`/install.sh` or `/install.ps1` so lifecycle transport does not drift onto
double-slash asset paths.
That same shared install-command transport must also fail closed on blank local
overrides: whitespace-only custom Pulse endpoint input in `InfrastructureInstallerSection.tsx`
or `SetupCompletionPanel.tsx` may not override the canonical backend-governed
endpoint, and shared command builders must reject blank endpoint URLs instead
of composing `/install.sh` or `/install.ps1` from an empty base.
That same copied upgrade boundary must preserve canonical runtime identity when
inventory already knows it: shell upgrade payloads must carry `--agent-id` and
`--hostname`, and PowerShell upgrade payloads must carry `PULSE_AGENT_ID` and
`PULSE_HOSTNAME`, so rerunning an upgrade does not silently collapse back to
local-machine identity.
Copied per-agent uninstall commands must also preserve the selected agent's
canonical identity instead of relying on local fallback discovery alone: when
inventory already knows the agent ID for the chosen row, the shell and
PowerShell uninstall payloads must carry that ID through to the installer so
managed removal deregisters the intended agent record even if local state or
hostname lookup is stale.
That same uninstall continuity must preserve canonical hostname fallback too:
copied shell uninstall payloads must carry `--hostname`, copied PowerShell
uninstall payloads must carry `PULSE_HOSTNAME`, and both installer runtimes
must prefer that explicit hostname during lookup fallback before querying local
machine identity. That fallback must also fail closed when hostname matches are
ambiguous: hostname matches may resolve only when they identify one and only
one agent, and display-name or short-hostname fallbacks must return not found
otherwise.
That governed hostname lookup fallback must also normalize query transport:
both installer runtimes must percent-encode the resolved hostname before
calling `/api/agents/agent/lookup`, so canonical identity recovery does not
drift on hostnames that contain spaces or other query-significant characters.
That same copied uninstall transport must also fail closed under required auth:
when Pulse requires API tokens, shell and PowerShell uninstall commands must
carry the same resolved token contract as install and upgrade instead of
silently degrading to tokenless deregistration transport.
That same copied Unix lifecycle transport must also preserve shell-safe
canonical identity: shell uninstall and upgrade commands must quote the
selected URL, token, agent ID, and hostname as command arguments instead of
interpolating raw inventory values into the shell line.
That same copied Windows lifecycle transport must also preserve
PowerShell-safe canonical identity: uninstall and upgrade commands must escape
selected URL, token, agent ID, and hostname values before placing them into
PowerShell env assignments or command text.
The same transport rule applies to copied install commands: shell install
payloads must quote canonical URL/token transport, and PowerShell install
payloads must escape URL/token values before they enter env assignments or
`irm` command text. The same Windows upgrade boundary must quote the resolved
PowerShell script URL as well, so canonical URLs with spaces or other
PowerShell-significant characters do not break copied upgrade transport after
the env assignments have already been escaped.
That same copied lifecycle transport must also preserve explicit custom CA
trust whenever the operator provides it: shell install, upgrade, and uninstall
commands must pass `--cacert` to both the outer installer download and the
installer runtime, while Windows install, upgrade, and uninstall commands must
emit `PULSE_CACERT` and fetch `install.ps1` through a transport-aware
PowerShell bootstrap that honors insecure-TLS or custom-CA settings on the
first script download instead of only after the installer has already started.
That bootstrap parity must match the installer's accepted trust formats too:
Windows copied commands must treat `PULSE_CACERT` as the same PEM/CRT/CER
certificate input that `scripts/install.ps1` accepts, rather than narrowing
the first-hop bootstrap to constructor-only certificate formats.
That same unified settings shell install and upgrade transport must also
preserve plain-HTTP continuity automatically: when the selected Pulse URL uses
`http://`, copied Unix commands must append `--insecure` even without the
manual TLS-skip toggle, while only the explicit TLS-skip toggle may widen curl
itself to `-k`.
That same unified settings installer surface must not drift between preview and
clipboard transport: the rendered Linux/macOS/BSD and Windows install snippets
must already include the active token choice, custom-CA trust, insecure/plain-
HTTP handling, install-profile flags, and command-execution mode instead of
displaying one command and mutating it only during copy.
For Unix-family host installs, that same seamless installer contract requires
the copied command to fetch the shared installer into an ephemeral directory,
run `install.sh --preflight-only` before privilege escalation, and fail before
`sudo` if the selected Pulse URL or exact `/download/pulse-agent?arch=...`
artifact is unavailable or missing checksum metadata. Token-bearing copied
commands must pass the credential to the installer through an ephemeral
`--token-file` and clean it up with the downloaded installer script, so the
installed service never receives a raw `--token` argument.
That same Windows install boundary must preserve the canonical server URL even
for the interactive PowerShell snippet: copied commands that still prompt for a
token must export `PULSE_URL` before invoking `install.ps1`, so the selected
agent-to-Pulse address cannot drift back to a default prompt target.
When the operator has already generated or selected a token, that same
interactive Windows install snippet must preserve the selected token in copied
transport as well, rather than silently dropping back to a second manual token
prompt while every other lifecycle command stays bound to the chosen credential.
The inverse must also hold: token-required instances without a selected token
must keep that interactive Windows snippet prompt-driven instead of exporting a
placeholder `PULSE_TOKEN` value into copied transport.
That same rule applies to copied Windows uninstall transport: after `PULSE_URL`
is escaped into env assignments, the uninstall path must still quote the
resolved `install.ps1` URL so canonical URLs with spaces remain valid
PowerShell transport during deregistration and removal.
For the shell installer, that continuity must also survive beyond the original
clipboard command: when install or upgrade runs with explicit `--agent-id` or
`--hostname`, `scripts/install.sh` must persist those values into its saved
connection state and recover them during later offline uninstall instead of
dropping back to ambient local discovery.
That same lifecycle-owned `connection.env` contract must also stay on one
installer-owned helper path: `scripts/install.sh` may not write the state file
one way and then recover it through a separate field-by-field inline parser,
because lifecycle ownership requires one canonical reader/writer for persisted
install identity and trust metadata.
That same lifecycle ownership must cover service control too: the installer may
still choose different platform adapters, but stop/restart semantics for the
managed agent must route through shared installer helpers instead of being
re-authored in each upgrade, systemd, OpenRC, SysV, or FreeBSD branch.
That same rule applies to teardown: uninstall and reinstall cleanup may not
rebuild disable/remove flows inline per platform. Shared installer helpers
must own service stop/disable/remove semantics for systemd, OpenRC, SysV, and
service-command runtimes so lifecycle cleanup stays canonical.
The same lifecycle rule applies to TrueNAS bootstrap too: boot-time recovery
for SCALE and CORE may only vary at the service-manager adapter, while binary
sync, service-link recreation, and startup sequencing stay on one
installer-owned renderer instead of drifting across separate embedded scripts.
That same lifecycle ownership must also cover service definition rendering:
systemd and FreeBSD rc.d files may not preserve parallel heredoc definitions
for the same agent runtime contract. Shared installer renderers must own the
common service shape, with platform branches only choosing the correct runtime
path, dependency targets, and logging adapter.
That same lifecycle rule also applies to installer completion: success,
unhealthy, and upgrade result handling may not drift by platform branch.
Shared installer helpers must own the save-state handoff, health verification,
canonical completion `json_event` output, and uninstall guidance instead of
letting each service-manager branch narrate those outcomes separately.
The same lifecycle rule applies to FreeBSD enablement too: direct rc.d install
and TrueNAS CORE boot recovery may not mutate `pulse_agent_enable` through
separate inline snippets. A shared installer-owned rc.conf enablement helper
must own that contract so lifecycle recovery and direct installs do not drift,
and that helper must execute the shared snippet in-process instead of defining
it in a discarded subshell.
The same rule applies to SysV registration: direct install may not keep its own
inline `update-rc.d` / `chkconfig` / manual symlink block while teardown owns a
separate canonical removal path. Shared installer helpers must own SysV
enablement and disablement semantics as one lifecycle contract.
The same durability rule applies to `scripts/install.ps1`: when Windows install
or upgrade runs with explicit agent or hostname identity, the installer must
persist that connection state under ProgramData and recover it during later
uninstall before falling back to machine-local discovery.
That Windows installer-owned continuity state is only valid for the currently
installed agent. After a successful uninstall, `scripts/install.ps1` must clear
its ProgramData state so later reruns cannot inherit stale identity or transport
context from a removed node.
That persisted installer-owned state must also retain self-signed transport
intent: when install or upgrade ran in insecure TLS mode, later offline
uninstall must recover that mode from saved state instead of silently
reverting to strict certificate validation.
For the shell installer, the same offline transport continuity also applies to
custom CA trust: when install or upgrade ran with `--cacert`, later offline
uninstall must recover that saved CA bundle path before reaching for governed
lookup or deregistration transport.
The Windows installer must now preserve the same installer-owned custom CA
transport continuity for its own network calls: when install or upgrade ran
with `PULSE_CACERT`, later offline uninstall must recover that saved CA
certificate path before governed lookup or deregistration falls back to
strict default trust.
That same Windows custom-CA continuity must also reach the long-lived unified
agent runtime: `scripts/install.ps1` must persist `--cacert` into the managed
service arguments, and `pulse-agent` must apply that bundle to updater,
remote-config, Unified Agent report, and command-channel HTTPS transport instead
of limiting `PULSE_CACERT` to installer-owned download and uninstall traffic.
That saved shell uninstall recovery must not depend only on a missing URL or
token. When the operator reruns uninstall with only partial CLI context, the
installer must still reload any missing persisted agent, hostname, insecure-TLS,
or custom-CA continuity before governed lookup or deregistration falls back to
ambient local state.
That same insecure-TLS continuity must hold during the Windows installer's own
network traffic, not only in the persisted service args: when the operator
selects insecure mode, `scripts/install.ps1` must also relax certificate
validation for its binary download and uninstall deregistration requests so
PowerShell transport can reach self-signed Pulse instances end to end.
The same copied install and upgrade commands must also fail closed on
token-optional Pulse instances: when the server does not require API tokens,
the command builder must omit token arguments entirely instead of serializing a
fake sentinel token value into shell or PowerShell install transport.
That token-optional settings path must still preserve explicit governed token
selection when the operator generates one anyway: optional auth widens the
contract to allow tokenless transport, but it must not erase or suppress a real
selected token and force copied install commands back to tokenless-only mode.
The installer scripts themselves must honor that same optional-auth contract:
`scripts/install.sh` and `scripts/install.ps1` must accept a missing token and
persist service arguments without `--token` on token-optional Pulse instances,
instead of advertising a no-token flow in settings while the installer still
fails validation at runtime.
That same optional-auth install contract also applies to backend-generated
Proxmox install commands in `internal/api/config_setup_handlers.go` and
`internal/api/agent_install_command_shared.go`: when Pulse auth is not
configured, the canonical agent-install-command API must return tokenless
install transport and must not persist a new API token record just because an
operator opened a backend-driven install surface.
That same backend-owned setup/install boundary also owns shipped security-doc
guidance in runtime responses and logs: `internal/api/config_setup_handlers.go`
and adjacent lifecycle setup helpers must not point operators at GitHub
`main` for security instructions that the running build already serves
locally, and should use the shipped `/docs/SECURITY.md` path instead.
The same optional-auth continuity must hold after install as well: Unified Agent
runtime startup may not reject a blank token unless enrollment is explicitly
enabled, agent report transport must omit auth headers when no token is
configured, and Proxmox auto-register flows must still complete without
serializing an empty token header on token-optional Pulse instances.
That same runtime-side reporting boundary must keep its product terminology
canonical in active comments and operator-facing logs: `internal/hostagent/`
may remain a package-location fact, but successful and failed report transport
must describe the runtime as the Unified Agent rather than reintroducing
"host agent" wording into v6 operator guidance.
That same post-install optional-auth contract must also hold during managed
removal: uninstall and deregistration flows must still notify Pulse with the
canonical agent-uninstall payload when URL and agent identity are known, and
must only attach API-token headers when a real token exists instead of
silently skipping deregistration on token-optional installs.
The same settings/profile boundary must also preserve assigned-profile
continuity when a referenced profile is no longer present in the fetched
profile list: assignment controls must keep the missing profile visible as the
current state instead of collapsing the agent back to a false default-looking
selection.
That same uninstall-command boundary must also preserve platform-canonical
transport in copied utility actions: Windows agents must receive the
PowerShell uninstall flow, and copied uninstall payloads must never substitute
an API token record ID where the runtime expects the real token secret for
server-side deregistration.
The same rule applies to Unix shell uninstall commands in the shared fleet
settings surface: copied uninstall payloads may include only a real token
secret when one is available, and must never fall back to token record IDs or
other settings-only identifiers that the installer runtime cannot authenticate.
Token-optional Windows uninstall commands must also preserve the canonical
server URL in `PULSE_URL`; otherwise the PowerShell installer can remove the
service locally while losing the deregistration path back to Pulse.

Shared `internal/api/` resource helpers now also expose governed
policy-aware resource metadata. Agent lifecycle and fleet-control surfaces may
consume canonical `policy` and `aiSafeSummary` fields from unified resource
payloads when they need resource context, but they must not fork their own
sensitivity-classification or local-vs-cloud routing heuristics on the same
runtime boundary. The same shared resource boundary now also owns the bundled
facet history read path for timeline data, so fleet lifecycle surfaces that
open resource drawers must continue to consume the backend bundle instead of
reassembling a local multi-call summary.
That same shared `internal/api/` extension-point boundary now also assumes
canonical security-token lifecycle reads. Lifecycle-adjacent setup and install
flows may inspect token metadata through the shared auth/security routes, but
they must not assume a displayed relay pairing token is disposable once
`lastUsedAt` is set. Shared helper changes that refresh, hide, or replace a
pairing credential must preserve used-token continuity instead of deleting a
credential that an already paired device still depends on.
That same shared `internal/api/` boundary now also owns agent-derived
physical-disk history transport. Lifecycle-adjacent storage drawers and fleet
resource surfaces may show host SMART-backed disk telemetry through the shared
`/api/metrics-store/history` route, but they must read the canonical disk
metrics target that monitoring projects for the resource instead of reviving a
browser-local disk collector, agent/device concatenation scheme, or other
surface-local history identity.
The browser-side runtime boundary is now explicit too. Lifecycle-owned
settings hooks such as
`frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts` and
`frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`
may read websocket state only through
`frontend-modern/src/contexts/appRuntime.ts`. They must not import `@/App` or
recreate app-shell providers, because `frontend-modern/src/App.tsx` owns
provider placement while lifecycle hooks must stay lazy-load safe and
shell-independent.
That same adjacent `internal/api/` boundary now also keeps public demos from
leaking commercial state through lifecycle-adjacent surfaces. Agent install,
reporting, and setup flows may share backend helpers with billing or license
transport, but `DEMO_MODE` must continue to 404 commercial read surfaces
instead of teaching lifecycle or mock-mode paths to bypass licensing. Public
demo readiness therefore comes from hiding commercial presentation on the
shared API boundary, not from introducing a second fake-entitlement path into
lifecycle-owned install or reporting flows. Browser-facing lifecycle surfaces
must also treat `/api/security/status.sessionCapabilities.demoMode` as the
canonical public-demo bootstrap signal instead of inferring demo posture from
headers, `/api/health`, or hostname heuristics.
Lifecycle-owned connected-infrastructure and reporting surfaces must also keep
governed platform rows on canonical local operator identity while tolerating
optional optimistic hostnames. Shared row models may fall back to the row name
when staging a removal state, but they must not resurrect legacy
`policy.display` shims or require platform-managed surfaces to synthesize a
second hostname contract.
That same lifecycle boundary also owns host-agent runtime test seams. Shared
agent lifecycle code such as `internal/hostagent/agent.go` must keep mutable
test hooks, command-client factories, and timing overrides on the per-config or
per-agent instance instead of package-global variables, so concurrent
lifecycle-owned update and registration paths cannot leak one test or runtime
override into another agent session.
The infrastructure workspace now uses a single flat route
(`/settings/infrastructure`) instead of the former three-sub-route layout
(`/install`, `/platforms`, `/connections`). Panel state — which add/edit
modal is open and which add step is active — is managed through the governed
`InfrastructurePanelStep` query contract rather than URL segments.
`frontend-modern/src/components/Settings/infrastructureWorkspaceModel.ts`
exposes `buildInfrastructureWorkspacePath()` and
`buildInfrastructureOnboardingPath()` as the sole path-building contract for
lifecycle-adjacent install and setup surfaces. Callers that formerly passed
`'platforms'` or `'install'` to `buildInfrastructureWorkspacePath` must use the
no-argument form or the query-backed onboarding builder; removed subpaths must
not remain as compatibility hrefs.
`frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx` now uses
shared infrastructure path-builder constants for workspace, add-infrastructure,
and agent-install CTAs, replacing the former pair of
`INFRASTRUCTURE_INSTALL_PATH` and `INFRASTRUCTURE_PLATFORMS_PATH` constants.
Installer-owned runtime continuity on persistence-sensitive NAS platforms is
also explicit again: `scripts/install.sh` now owns the QNAP bootstrap contract
that waits for the persistent data volume before launching the stored wrapper,
and `internal/agentupdate/update.go` keeps the persisted QNAP binary copy in
sync on self-update so reboot does not roll the runtime back to an older
binary.
