# Agent Lifecycle Contract

## Contract Metadata

```json
{
  "subsystem_id": "agent-lifecycle",
  "lane": "L16",
  "contract_file": "docs/release-control/v6/internal/subsystems/agent-lifecycle.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts"
  ]
}
```

## Purpose

Own unified agent installation, registration, update continuity, profile
management, and fleet control surfaces.

## Canonical Files

1. `internal/api/agent_install_command_shared.go`
2. `internal/api/config_setup_handlers.go`
3. `internal/api/unified_agent.go`
4. `internal/agentupdate/update.go`
5. `internal/hostagent/agent.go`
6. `scripts/install.sh`
7. `scripts/install.ps1`
8. `frontend-modern/src/api/agentProfiles.ts`
9. `frontend-modern/src/components/Settings/AgentProfilesPanel.tsx`
10. `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`
11. `frontend-modern/src/components/Settings/InfrastructureOperationsController.tsx`
12. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`
13. `frontend-modern/src/components/Settings/InfrastructureInstallPanel.tsx`
14. `frontend-modern/src/components/Settings/InfrastructureReportingPanel.tsx`
15. `frontend-modern/src/components/Settings/InfrastructurePlatformConnectionsSummaryCard.tsx`
16. `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`
17. `frontend-modern/src/components/Settings/infrastructureWorkspaceModel.ts`
18. `frontend-modern/src/components/Settings/ProxmoxSettingsPanel.tsx`
19. `frontend-modern/src/components/Settings/proxmoxSettingsModel.ts`
20. `frontend-modern/src/components/Settings/ProxmoxDirectWorkspace.tsx`
21. `frontend-modern/src/components/Settings/ProxmoxConfiguredNodesTable.tsx`
22. `frontend-modern/src/components/Settings/ProxmoxDirectConnectionsCard.tsx`
23. `frontend-modern/src/components/Settings/ProxmoxDiscoveryResultsCard.tsx`
24. `frontend-modern/src/components/Settings/ProxmoxDeleteNodeDialog.tsx`
25. `frontend-modern/src/components/Settings/ProxmoxNodeModalStack.tsx`
26. `frontend-modern/src/components/Settings/ConfiguredNodeTables.tsx`
27. `frontend-modern/src/components/Settings/SettingsSectionNav.tsx`
28. `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`
29. `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`
30. `frontend-modern/src/components/Settings/useProxmoxDirectWorkspaceState.ts`
31. `frontend-modern/src/components/Settings/NodeModal.tsx`
32. `frontend-modern/src/components/Settings/NodeModalAuthenticationSection.tsx`
33. `frontend-modern/src/components/Settings/NodeModalBasicInfoSection.tsx`
34. `frontend-modern/src/components/Settings/nodeModalModel.ts`
35. `frontend-modern/src/components/Settings/NodeModalMonitoringSection.tsx`
36. `frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx`
37. `frontend-modern/src/components/Settings/NodeModalStatusFooter.tsx`
38. `frontend-modern/src/components/Settings/useNodeModalState.ts`
39. `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`
40. `frontend-modern/src/components/Infrastructure/deploy/ResultsStep.tsx`
41. `frontend-modern/src/utils/agentProfilesPresentation.ts`
42. `frontend-modern/src/utils/agentInstallCommand.ts`
43. `frontend-modern/src/api/nodes.ts`
44. `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
45. `frontend-modern/src/components/Settings/InfrastructureInventorySection.tsx`
46. `frontend-modern/src/components/Settings/InfrastructureActiveRowDetails.tsx`
47. `frontend-modern/src/components/Settings/InfrastructureIgnoredRowDetails.tsx`
48. `frontend-modern/src/components/Settings/InfrastructureStopMonitoringDialog.tsx`
49. `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`
50. `frontend-modern/src/components/Settings/useInfrastructureReportingState.tsx`
51. `frontend-modern/src/components/Settings/infrastructureSettingsModel.ts`
52. `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts`
53. `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts`
54. `frontend-modern/src/utils/infrastructureSettingsPresentation.ts`
55. `frontend-modern/src/utils/agentCapabilityPresentation.ts`
56. `frontend-modern/src/utils/agentProfileSuggestionPresentation.ts`
57. `frontend-modern/src/utils/configuredNodeCapabilityPresentation.ts`
58. `frontend-modern/src/utils/configuredNodeStatusPresentation.ts`
59. `frontend-modern/src/utils/unifiedAgentInventoryPresentation.ts`
60. `frontend-modern/src/utils/unifiedAgentStatusPresentation.ts`
61. `frontend-modern/src/utils/clusterEndpointPresentation.ts`
62. `frontend-modern/src/utils/nodeModalPresentation.ts`
63. `frontend-modern/src/utils/proxmoxSettingsPresentation.ts`
64. `frontend-modern/src/components/Settings/PlatformConnectionsWorkspace.tsx`
65. `frontend-modern/src/components/Settings/platformConnectionsModel.ts`
66. `frontend-modern/src/components/Settings/TrueNASSettingsPanel.tsx`
67. `frontend-modern/src/components/Settings/useTrueNASSettingsPanelState.ts`
68. `frontend-modern/src/components/Settings/VMwareSettingsPanel.tsx`
69. `frontend-modern/src/components/Settings/useVMwareSettingsPanelState.ts`
70. `frontend-modern/src/components/Settings/MonitoredSystemAdmissionPreview.tsx`
71. `internal/hostagent/proxmox_setup.go`

## Shared Boundaries

1. `frontend-modern/src/api/agentProfiles.ts` shared with `api-contracts`: the agent profiles frontend client is both an agent lifecycle control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/api/nodes.ts` shared with `api-contracts`: the shared Proxmox node client is both an agent lifecycle setup/install control surface and a canonical API payload contract boundary.
3. `frontend-modern/src/components/Settings/InfrastructureOperationsController.tsx` shared with `api-contracts`: the infrastructure operations controller is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
4. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx` shared with `api-contracts`: the pure infrastructure operations inventory/install model is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
5. `frontend-modern/src/components/Settings/MonitoredSystemAdmissionPreview.tsx` shared with `cloud-paid`: the monitored-system admission preview is both a platform-connections lifecycle surface and a canonical cloud-paid monitored-system presentation boundary.
6. `frontend-modern/src/components/Settings/NodeModal.tsx` shared with `api-contracts`: the node setup modal render shell is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
7. `frontend-modern/src/components/Settings/NodeModalAuthenticationSection.tsx` shared with `api-contracts`: the node setup authentication section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
8. `frontend-modern/src/components/Settings/NodeModalBasicInfoSection.tsx` shared with `api-contracts`: the node setup basic-info section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
9. `frontend-modern/src/components/Settings/nodeModalModel.ts` shared with `api-contracts`: the pure node setup modal model is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
10. `frontend-modern/src/components/Settings/NodeModalMonitoringSection.tsx` shared with `api-contracts`: the node setup monitoring section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
11. `frontend-modern/src/components/Settings/NodeModalSetupGuideSection.tsx` shared with `api-contracts`: the node setup guide section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
12. `frontend-modern/src/components/Settings/NodeModalStatusFooter.tsx` shared with `api-contracts`: the node setup status/footer section is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
13. `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts` shared with `api-contracts`: the direct-node infrastructure settings state hook is both an agent lifecycle control surface and a shared Proxmox node API contract boundary.
14. `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts` shared with `api-contracts`: the infrastructure discovery runtime state hook is both an agent lifecycle control surface and a shared discovery/settings API contract boundary.
15. `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx` shared with `api-contracts`: the infrastructure install state hook is both an agent fleet lifecycle control surface and an API token, lookup, and install transport contract boundary.
16. `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx` shared with `api-contracts`: the shared infrastructure operations state hook is both an agent fleet lifecycle control surface and an API token, lookup, assignment, and reporting/install contract boundary.
17. `frontend-modern/src/components/Settings/useInfrastructureReportingState.tsx` shared with `api-contracts`: the infrastructure reporting state hook is both an agent fleet lifecycle control surface and an API-backed assignment, reporting, and reconnect contract boundary.
18. `frontend-modern/src/components/Settings/useNodeModalState.ts` shared with `api-contracts`: the node setup modal state hook is both an agent lifecycle control surface and a shared API-backed install/setup contract boundary.
19. `frontend-modern/src/utils/agentInstallCommand.ts` shared with `api-contracts`: the shared frontend install-command helper is both an agent lifecycle control surface and a canonical API/install transport contract boundary.
20. `frontend-modern/src/utils/infrastructureSettingsPresentation.ts` shared with `api-contracts`: the infrastructure settings presentation helper is both an agent lifecycle control surface and an API-backed direct-node/discovery settings boundary.
21. `internal/api/agent_install_command_shared.go` shared with `api-contracts`: agent install command assembly is both an agent lifecycle control surface and a canonical API payload contract boundary.
22. `internal/api/config_setup_handlers.go` shared with `api-contracts`: auto-register and setup handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
23. `internal/api/unified_agent.go` shared with `api-contracts`: unified agent download and installer handlers are both an agent lifecycle control surface and a canonical API payload contract boundary.
24. `scripts/install.ps1` shared with `deployment-installability`: the Windows installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.
25. `scripts/install.sh` shared with `deployment-installability`: the shell installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.

## Extension Points

1. Add or change install-command generation, canonical /api/auto-register behavior, or installer download behavior through the owned `internal/api/` files above.
   That same shared `internal/api/` adjacency does not transfer ownership of
   Patrol quickstart bootstrap or mobile Patrol-provider bridging:
   `internal/api/ai_handlers.go` and `internal/api/chat_service_adapter.go`
   may be invoked by lifecycle-adjacent surfaces, but server-issued
   quickstart tokens, Patrol quickstart credit snapshots, and AI runtime auth
   remain `ai-runtime` plus `api-contracts` concerns rather than install-token
   or lifecycle credential state. Server-verified runtime identity remains the
   only authority for Patrol quickstart bootstrap: self-hosted installs use the
   shared installation-scoped `activation.enc`, while entitled hosted lanes use
   the signed entitlement lease already carried in canonical billing state.
   Lifecycle flows must not reintroduce anonymous bootstrap identity,
   tenant-local commercial-owner surrogates, or fake activation records when
   they traverse those shared handlers.
   That same shared quickstart boundary is vendor-neutral at the lifecycle
   edge too: lifecycle-adjacent consumers may observe the stable
   `quickstart:pulse-hosted` alias in AI settings payloads, but they must not
   bake vendor model IDs or provider-model fallback rules into install or
   activation flows just because those routes share the backend API tree.
   Persisted legacy hosted quickstart model IDs are therefore not lifecycle
   truth either: when shared settings helpers load or save historical
   quickstart values, they must normalize them back to
   `quickstart:pulse-hosted` before adjacent install or activation flows read
   the payload.
   The machine-scoped quickstart authority must stay canonical too:
   tenant-local lifecycle routes may reuse shared installation activation or
   effective entitlement billing state, but they must not fork per-org
   activation caches, alternate installation-token stores, synthetic
   entitlement mirrors, or competing quickstart-owner identity. That same
   shared AI/runtime boundary now also owns Patrol quickstart execution
   identity: lifecycle-adjacent flows may trigger or observe Patrol runs
   through `internal/api/chat_service_adapter.go`, but they must preserve the
   stable execution identifier that lets the hosted quickstart contract bill
   once per higher-level Patrol run rather than once per internal provider
   turn.
2. Add or change update continuity and persisted-version handoff through `internal/agentupdate/`.
3. Add or change runtime-side Unified Agent startup, first-report assembly, and enroll/runtime continuity through `internal/hostagent/`.
   Proxmox host-agent setup must treat local `proxmox-registered` markers as a cache, not authority: before skipping token setup or node repair, `internal/hostagent/proxmox_setup.go` must revalidate the current type and candidate hosts against Pulse through the canonical auto-register contract.
   That runtime-side ownership includes local disk telemetry collection in
   `internal/hostagent/smartctl.go`. Linux SMART discovery must prefer
   `smartctl --scan-open` typed targets before generic block-device fallback so
   controller-backed disks keep their canonical SMART and wearout coverage.
   FreeBSD SMART probing must retry through the canonical typed and untyped
   device modes and the SCT temperature status path before settling on standby
   or no-data results, and partial or plain-text smartctl output must still
   preserve model, serial, health, and temperature data through the same
   host-agent runtime boundary instead of leaving monitoring to guess.
4. Keep shared `internal/api/` helper edits isolated from agent lifecycle semantics: Patrol-specific status transport or alert-trigger wiring changes in shared handlers must not bleed into auto-register, installer, or fleet-control behavior unless this contract moves in the same slice.
   The same isolation rule applies to AI settings payload work in `internal/api/ai_handlers.go`: provider auth fields, masked-secret echoes, and provider-test model selection remain AI/runtime plus API-contract ownership and must not be reinterpreted as lifecycle setup or registration semantics just because they share backend helper layers.
   The same shared-helper rule now covers SSO outbound discovery and metadata fetches plus credential-file loads in `internal/api/sso_outbound.go`, `internal/api/saml_service.go`, and `internal/api/oidc_service.go`: lifecycle-adjacent setup or auth work may depend on that shared trust boundary, but it must not fork a second HTTP client, redirect policy, or file-read rule inside lifecycle-local flows.
4. Keep legacy Unified Agent compatibility names explicitly secondary when touching shared `internal/api/` runtime helpers: the legacy host-route family and `host-agent:*` scope names may remain as ingress or migration aliases, but they must not retake primary ownership in router state, live runtime scope checks, handler commentary, or operator-facing guidance.
5. Add or change installer flags, persisted service arguments, or upgrade-safe re-entry behavior through `scripts/install.sh` and `scripts/install.ps1`.
6. Add or change profile management, the extracted agent profiles runtime owner, the pure unified-agent inventory/install model, the API-backed platform connections workspace shell, route model, reporting summary owner, shared install/inventory/dialog section owners, the split infrastructure install/reporting state owners, the split direct-node/discovery infrastructure settings owners plus their shared model, shared frontend install-command assembly, Proxmox setup/install API transport, TrueNAS platform-connection management, VMware platform-connection management, the shared monitored-system admission preview shell for those platform connections, setup-completion install handoff transport, deploy-fallback manual install transport, and fleet-control presentation through `frontend-modern/src/api/agentProfiles.ts`, `frontend-modern/src/api/nodes.ts`, `frontend-modern/src/components/Settings/AgentProfilesPanel.tsx`, `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`, `frontend-modern/src/components/Settings/InfrastructureOperationsController.tsx`, `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`, `frontend-modern/src/components/Settings/InfrastructureInstallPanel.tsx`, `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`, `frontend-modern/src/components/Settings/InfrastructureReportingPanel.tsx`, `frontend-modern/src/components/Settings/InfrastructureInventorySection.tsx`, `frontend-modern/src/components/Settings/InfrastructureActiveRowDetails.tsx`, `frontend-modern/src/components/Settings/InfrastructureIgnoredRowDetails.tsx`, `frontend-modern/src/components/Settings/InfrastructureStopMonitoringDialog.tsx`, `frontend-modern/src/components/Settings/InfrastructurePlatformConnectionsSummaryCard.tsx`, `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`, `frontend-modern/src/components/Settings/infrastructureWorkspaceModel.ts`, `frontend-modern/src/components/Settings/MonitoredSystemAdmissionPreview.tsx`, `frontend-modern/src/components/Settings/PlatformConnectionsWorkspace.tsx`, `frontend-modern/src/components/Settings/platformConnectionsModel.ts`, `frontend-modern/src/components/Settings/TrueNASSettingsPanel.tsx`, `frontend-modern/src/components/Settings/useTrueNASSettingsPanelState.ts`, `frontend-modern/src/components/Settings/VMwareSettingsPanel.tsx`, `frontend-modern/src/components/Settings/useVMwareSettingsPanelState.ts`, `frontend-modern/src/components/Settings/ProxmoxSettingsPanel.tsx`, `frontend-modern/src/components/Settings/proxmoxSettingsModel.ts`, `frontend-modern/src/components/Settings/ProxmoxDirectWorkspace.tsx`, `frontend-modern/src/components/Settings/ProxmoxConfiguredNodesTable.tsx`, `frontend-modern/src/components/Settings/ProxmoxDirectConnectionsCard.tsx`, `frontend-modern/src/components/Settings/ProxmoxDiscoveryResultsCard.tsx`, `frontend-modern/src/components/Settings/ProxmoxDeleteNodeDialog.tsx`, `frontend-modern/src/components/Settings/ProxmoxNodeModalStack.tsx`, `frontend-modern/src/components/Settings/ConfiguredNodeTables.tsx`, `frontend-modern/src/components/Settings/SettingsSectionNav.tsx`, `frontend-modern/src/components/Settings/infrastructureSettingsModel.ts`, `frontend-modern/src/components/Settings/useInfrastructureConfiguredNodesState.ts`, `frontend-modern/src/components/Settings/useInfrastructureDiscoveryRuntimeState.ts`, `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`, `frontend-modern/src/components/Settings/useInfrastructureOperationsState.tsx`, `frontend-modern/src/components/Settings/useInfrastructureReportingState.tsx`, `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts`, `frontend-modern/src/components/Settings/useProxmoxDirectWorkspaceState.ts`, `frontend-modern/src/components/Settings/NodeModal.tsx`, `frontend-modern/src/components/Settings/nodeModalModel.ts`, `frontend-modern/src/components/Settings/useNodeModalState.ts`, `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`, and `frontend-modern/src/utils/agentInstallCommand.ts`.
   Those lifecycle-owned settings hooks may consume websocket state only through `frontend-modern/src/contexts/appRuntime.ts`; they must not import `frontend-modern/src/App.tsx` or recreate root-shell providers.
   Agent-profile suggestion affordances remain adjacent assistant UX, not a
   second AI settings reader. `frontend-modern/src/components/Settings/useAgentProfilesPanelState.ts`
   may gate the Ideas action only from the shared assistant-availability fact
   seeded by `/api/security/status.sessionCapabilities.assistantEnabled`, and
   must not probe `/api/settings/ai` just to decide whether that affordance
   should render.
7. Preserve canonical token-lifecycle reads in shared `internal/api/` auth/security helpers so lifecycle-adjacent setup and install flows do not revoke a displayed relay pairing token after `lastUsedAt` proves that an already paired device is actively depending on that credential.
8. Preserve backend-owned Pulse Mobile relay runtime credential minting in those same shared `internal/api/` auth/security helpers so lifecycle-adjacent setup and install flows reuse the canonical mobile token route instead of reintroducing wildcard or browser-authored runtime token bundles.
9. Preserve the dedicated backend-owned `relay:mobile:access` capability and its governed backward-compatible route inventory plus the shared helper call sites around it, so lifecycle-adjacent setup and install flows do not widen the mobile device credential back into general AI chat/execute scope ownership.
10. Preserve shipped security-doc guidance in shared lifecycle setup helpers so `internal/api/config_setup_handlers.go` and adjacent install/setup runtime paths point operators at the running build's local security documentation route rather than GitHub `main` links.
11. Keep shared `internal/api/router.go` workload-chart downsampling presentation-only: when that router caps mixed-cadence workload history into equal-time buckets for operator-facing cards, lifecycle-adjacent setup and fleet surfaces must not reuse the shaped chart samples as heartbeat, enrollment, or last-seen authority.
    That same presentation-only boundary must preserve canonical millisecond timestamps when it serializes chart points, so lifecycle-adjacent first-host and fleet surfaces do not misread rounded chart samples as duplicate or restarted heartbeat evidence.
    The same rule now applies to storage summary interaction. Shared sticky-card or row-hover focus behavior on infrastructure, workloads, and storage may reuse the canonical chart transport, but lifecycle-adjacent install, enrollment, and fleet surfaces must not treat highlighted summary series or sticky-shell state as agent freshness or setup progress.
12. Keep lifecycle installer fallback pinned to published release lineage only.
    When `internal/api/unified_agent.go` has to proxy `/install.sh` or
    `/install.ps1` from GitHub, the shared lifecycle path may only treat stable
    tags and explicit RC prerelease tags as release assets. Working-line dev
    prereleases and build-metadata versions must fail closed so first-host
    install, repair, and fleet continuity do not depend on unpublished or
    branch-local installer URLs.
13. Keep self-hosted purchase handoff state on the adjacent commercial/auth
    boundary. When shared `internal/api/router.go`,
    `internal/api/router_routes_cloud.go`, `internal/api/licensing_handlers.go`,
    or `internal/api/demo_mode_commercial.go` evolve public
    `/auth/license-purchase-start` or `/auth/license-purchase-activate`,
    lifecycle-adjacent setup and fleet
    surfaces may rely on that public-route wiring but must not reinterpret the
    commercial-owned `portal_handoff_id`, server-resolved checkout intent, purchase-return tokens,
    activation-bridge form state, owned billing purchase-arrival states, or
    demo-hidden commercial route policy as installer credentials,
    registration state, or fleet enrollment authority. The same adjacent
    commercial boundary now also owns migrated-v5 monitored-system
    grandfathering: lifecycle surfaces may react to the resulting license or
    entitlements payloads, but they must not cache their own pre-activation
    host counts, synthesize a second grandfather floor, or treat install-time
    fleet inventory as the authority for commercial continuity. They also must
    not depend on a status or entitlements read to seal pending grandfather
    continuity, use those billing reads to restart pending continuity
    reconciliation, or reinterpret continuity-verification payloads as a real
    `0 / limit` monitored-system state.
    When the commercial reconciler captures the floor, lifecycle-adjacent
    code must treat the resulting activation-state callback as commercial
    ownership cleanup only, not as install inventory proof or fleet enrollment
    state.

## Forbidden Paths

1. New install or update continuity behavior hidden only inside broad monitoring ownership.
2. Agent profile or fleet-control behavior implemented outside the canonical agent settings/profile surfaces.
3. Installer or update flows that depend on branch-tip, dev-only, or non-release asset behavior for supported RC/stable paths.

## Completion Obligations

1. Update this contract when agent lifecycle ownership changes.
2. Keep shared API proof routing aligned whenever install, register, or profile payloads change.
3. Update runtime and settings tests in the same slice when lifecycle behavior changes.
4. Preserve canonical /api/auto-register node identity continuity when canonical hosts shift between hostname and IP forms for the same node.
5. Keep Proxmox registration continuity self-healing: stale local registration markers must be verified against Pulse before the host agent skips setup, and a missing matching node on the Pulse side must drive canonical re-registration instead of asking operators to delete marker files manually.
5. Keep first-session lifecycle handoff explicit: the live setup completion
   surface in `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`
   must route the primary CTA into `/settings/infrastructure/install`, frame
   that route as the first-host install step, and present `Platform
   connections` as the named API-backed alternative for Proxmox, TrueNAS, and
   future provider integrations rather than leaving post-setup next actions
   implicit. That API-backed alternative must be a real first-run handoff
   control, not prose-only guidance.
   Once the completion surface observes connected systems, that same handoff
   model must derive its follow-up actions from the canonical connected-system
   path classification rather than a raw connected-agent count. API-backed
   first-session states must keep `Platform connections` visible without
   hiding `Infrastructure Install` when the next system should run the unified
   agent, and install-managed first-session states must not suppress the
   explicit API-backed alternative when the runtime has already connected
   platform-owned systems.
6. Keep `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
   oriented around the first monitored host. Install-token generation,
   governed command copy, and install instructions belong to the canonical
   lifecycle path; transport details, trust overrides, profile tuning, and
   adjacent alternatives must remain secondary to that first-host onboarding
   narrative, including an explicit advanced-options disclosure so first-time
   operators see token generation, command copy, and status confirmation
   before non-default connection controls.
7. Keep `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`
   and `frontend-modern/src/components/Settings/infrastructureWorkspaceModel.ts`
   aligned with that same lifecycle path. Bare infrastructure settings routes
   must default to the install workspace, and the workspace shell must make
   the first-host sequence explicit before operators drift into reporting and
   control surfaces.
8. Keep post-install lifecycle completion explicit inside
   `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`
   and `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`.
9. Keep the dev first-session proof deterministic on the real wizard path:
   `tests/integration/tests/helpers.ts` and
   `tests/integration/tests/11-first-session.spec.ts` must refresh first-run
   state through `/api/security/dev/reset-first-run`, then prove both the
   canonical `Open Infrastructure Install` handoff and the explicit
   `Open Platform connections` handoff against the live setup wizard instead
   of relying on stale bootstrap tokens, dashboard fallbacks, or preview-only
   coverage.
   When the first host reports successfully, the install workflow must treat
   that as a completion handoff with direct navigation into `/dashboard` and
   `/settings/infrastructure/operations` instead of leaving operators on a
   generic lookup result. When the workspace starts from zero active connected
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
9. Keep `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`
   ordered around the actual first-run operator sequence: credentials that must
   be saved now should be visible before the operator leaves the screen, and
   the completion surface should present one canonical primary next-step path
   into Infrastructure Install instead of repeating competing install or
   dashboard CTAs across multiple sections. Once the first monitored host is
   already connected, that same surface must pivot its primary CTA and headline
   to `/` so the operator is sent to the dashboard rather than being told to
   install the first host again. While the first host is still pending, that
   same completion narrative must describe Infrastructure Install as the place
   where the first-host scoped install token is prepared from setup handoff,
   not as a second manual token-generation task the operator still needs to
   figure out.
10. Keep API-backed platform onboarding explicit across
    `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`,
    `frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx`,
    `frontend-modern/src/components/Settings/useInfrastructureInstallState.tsx`,
    `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`, and
    `frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx`.
    TrueNAS must be presented as a Platform connections workflow first, not as
    a dedicated Unified Agent install profile. The install workspace may remain
    available for optional later agent augmentation on TrueNAS, but first-run
    copy, alternative CTAs, and install-profile lists must not imply that an
    agent install is the required bootstrap for TrueNAS support in Pulse.
11. Keep first-session and lifecycle-adjacent frontend resource handling on the
    canonical unified-resource boundary. Top-level TrueNAS appliances may reach
    setup-completion or infrastructure lifecycle surfaces only as canonical
    `agent` resources with `platformType: 'truenas'`; any legacy raw
    `resource.type === 'truenas'` compatibility collapse belongs in the shared
    frontend resource adapters, not in setup or lifecycle-local UI branching.
12. Keep lifecycle-adjacent AI transport compatibility on the shared
    `internal/api/` boundary. If chat mention parsing, alert investigation
    targets, or adjacent Assistant resource transport still accept a legacy
    top-level `truenas` type, that value must collapse immediately to the
    canonical `agent` host type before lifecycle surfaces, setup handoffs, or
    operator-visible route state consume it.
13. Keep onboarding ownership aligned with
    `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`: agent-backed
    first-class platforms belong to the install/reporting lifecycle path,
    API-backed first-class platforms belong to Platform connections, and any
    later unified-agent augmentation on an API-backed platform must remain an
    optional secondary path instead of silently becoming the required bootstrap.

## Current State

This subsystem now sits under the dedicated agent lifecycle and fleet
operations lane so install, registration, update continuity, profile
management, and fleet safety stop hiding inside architecture, migration, or
monitoring work.
That same adjacent `internal/api/` boundary now also keeps public demos from
leaking commercial state through lifecycle-adjacent surfaces. Agent install,
reporting, and setup flows may share backend helpers with billing or license
transport, but `DEMO_MODE` must continue to 404 commercial read surfaces
instead of teaching lifecycle or mock-mode paths to bypass licensing. That
same boundary also hides monitored-system explanation and provider preview
routes used by lifecycle-adjacent platform connections. Public
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
Even on that governed first-run exception, render-time trial gating should stay
on shared selectors such as `isCommercialTrialActive()` instead of reading raw
commercial-posture fields inside `SetupCompletionPanel.tsx`.
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
That same shared dependency now also assumes hosted cloud handoff makes tenant
org access real before browser lifecycle continues. Lifecycle-adjacent opens
into hosted workspaces may depend on `internal/api/cloud_handoff_handlers.go`,
but the canonical contract is that a successful handoff exchange must reconcile
tenant organization membership for the handed-off account member before the
browser follows the new session into protected routes, rather than landing on a
fresh `access_denied` immediately after session minting.
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
settings routes must keep surfacing through the same Platform connections
workspace and handoff URLs instead of depending on process-start-only wiring
or a mock-only alternate shell.
That same lifecycle-owned mock path now also requires one shared fixture owner
for API-backed platform onboarding. TrueNAS and VMware connection-list payloads
shown in Platform connections must be assembled from the canonical
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
That same hosted continuity contract also applies to the older direct tenant
magic-link path. Lifecycle-adjacent control-plane redirects through
`/auth/cloud-handoff` must preserve canonical account/user/role identity in the
handoff token long enough for the tenant runtime to repair org membership
before it lands in protected hosted routes, rather than letting direct opens
diverge from the newer portal exchange path.
That same shared `internal/api/` dependency also assumes telemetry
transparency remains explicitly system-settings-owned. When lifecycle-adjacent
setup or router work touches shared `internal/api/` files, telemetry preview
and install-ID reset routes must keep reusing the canonical system-settings
trust boundary and server-owned telemetry runtime instead of borrowing agent
lifecycle proof or state ownership just because the same router surface moved.
That same shared `internal/api/ai_handlers.go` dependency also now assumes
Patrol-specific settings and status expansions stay Patrol-owned. When shared
AI handlers add split scoped-trigger fields, recency labels, or trigger-state
transport for Patrol, lifecycle-adjacent setup and fleet surfaces must treat
those payloads as Patrol-only runtime context and must not reinterpret them as
agent install readiness, enrollment health, or fleet-control state.
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
The connected-infrastructure reporting workspace also now treats API-backed
platform surfaces as platform-connection-managed capabilities, not host-managed
agent extensions. `frontend-modern/src/components/Settings/infrastructureOperationsModel.tsx`,
`InfrastructureActiveRowDetails.tsx`, and `useInfrastructureReportingState.tsx`
must keep Proxmox, PBS, PMG, and TrueNAS on the shared Platform connections
path, while only machine-installed agent, Docker, and Kubernetes surfaces
participate in host stop-monitoring scope, uninstall commands, and upgrade
actions.
Those unified audit list endpoints also clamp oversized `limit` requests to
the governed maximum, so audit history stays bounded even when callers ask
for arbitrarily large pages.
That same shared `internal/api/` dependency also now assumes hosted runtime
websocket upgrades trust the cloud proxy only through explicit tenant
`PULSE_TRUSTED_PROXY_CIDRS` wiring, so first-session handoff and agent-facing
live activity surfaces do not degrade into reconnect loops when a hosted
workspace is opened through the control plane.
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
The router now wires the tenant resource state provider during initial setup
when a multi-tenant monitor is present, so tenant-scoped fleet pages do not
trip a missing-provider 500 before the monitor has finished initializing.
The dedicated profile client now also routes list, schema, and validation
parsing through shared response helpers in `frontend-modern/src/api/agentProfiles.ts`,
so profile transport stays aligned with the governed API contract instead of
reintroducing local array or JSON parsing rules.
That same lifecycle-owned install/profile surface now also routes trial-start
CTA orchestration through `frontend-modern/src/utils/trialStartAction.ts`.
Agent profile paywalls, NodeModal upgrade prompts, and setup-completion trial
actions may choose their own success copy, but they must use the shared helper
for hosted trial redirects and canonical denial handling instead of open-coding
`startProTrial()` branches in each lifecycle surface.

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
That same lifecycle-adjacent hosted setup path also depends on AI bootstrap
staying canonical before the first settings write. Hosted operators may land
in Chat, Patrol-backed setup hints, or AI-dependent remediation surfaces
before anyone has visited AI Settings, so the shared `internal/api/`
hosted-AI bootstrap helper must persist the machine-owned quickstart-backed
`ai.enc` for entitled tenants by reusing the signed entitlement lease already
present in billing state; it must not fabricate installation activation just
to satisfy quickstart bootstrap. Otherwise lifecycle-adjacent hosted flows end
up in a fake `AI disabled` state that disappears only after a manual save.
That same lifecycle-adjacent hosted entitlement path must also preserve the
trial quickstart grant fields already seeded into billing state. Hosted setup
and pairing may depend on the shared `internal/api/` entitlement refresh, but
that refresh must not erase quickstart bootstrap inventory while it rewrites
lease-backed plan and capability data.
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
The API-backed platform connections workspace now also lives explicitly inside
this lifecycle boundary: `InfrastructureWorkspace.tsx`,
`infrastructureWorkspaceModel.ts`,
`InfrastructureInstallPanel.tsx`, `InfrastructureInstallerSection.tsx`,
`InfrastructureReportingPanel.tsx`, `InfrastructureInventorySection.tsx`,
`InfrastructureActiveRowDetails.tsx`,
`InfrastructureIgnoredRowDetails.tsx`,
`InfrastructureStopMonitoringDialog.tsx`,
`InfrastructurePlatformConnectionsSummaryCard.tsx`,
`PlatformConnectionsWorkspace.tsx`, `platformConnectionsModel.ts`,
`TrueNASSettingsPanel.tsx`, `useTrueNASSettingsPanelState.ts`,
`ProxmoxSettingsPanel.tsx`, `proxmoxSettingsModel.ts`,
`ProxmoxDirectWorkspace.tsx`, `ProxmoxConfiguredNodesTable.tsx`,
`ProxmoxDirectConnectionsCard.tsx`, `ProxmoxDiscoveryResultsCard.tsx`,
`ProxmoxDeleteNodeDialog.tsx`, `ProxmoxNodeModalStack.tsx`,
`ConfiguredNodeTables.tsx`, `SettingsSectionNav.tsx`,
`useInfrastructureOperationsState.ts`, `infrastructureSettingsModel.ts`,
`useInfrastructureConfiguredNodesState.ts`,
`useInfrastructureDiscoveryRuntimeState.ts`, `useInfrastructureSettingsState.ts`,
and `useProxmoxDirectWorkspaceState.ts` own the fallback install/direct/reporting
operator flow, with `PlatformConnectionsWorkspace.tsx` as the canonical
API-backed platform shell, `ProxmoxSettingsPanel.tsx` and
`TrueNASSettingsPanel.tsx` as provider-specific workspaces,
`useInfrastructureSettingsState.ts` as the shared platform-connections
composition boundary, and
the direct-node/discovery runtime hooks plus `useTrueNASSettingsPanelState.ts`
as the canonical provider state owners, instead of leaving those panels
ungoverned beside the canonical unified-agent install path.
That same lifecycle-owned platform-connections workspace must keep API-backed
provider state operationally useful, not CRUD-only. `TrueNASSettingsPanel.tsx`
and `useTrueNASSettingsPanelState.ts` must surface the shared runtime health,
poll cadence, discovered contribution summary, and canonical infrastructure /
workloads / storage / recovery handoffs coming from `/api/truenas/connections`
instead of falling back to panel-local inference or agent-first setup guidance.
Saved connection retests from that workspace must use the server-owned
`POST /api/truenas/connections/{id}/test` path so operators can verify stored
credentials without leaking masked-secret placeholders back into the draft
connection form contract. When the operator is editing a saved connection, that
same path must also accept the in-flight form payload and merge unchanged
masked secrets on the server, so edit-dialog tests do not force credential
re-entry just to validate changed host or TLS fields.
When an operator runs a row-level saved-connection test from that workspace,
`useTrueNASSettingsPanelState.ts` must reload the shared connection summary
after the request completes so the card reflects refreshed last-success or
last-error state instead of leaving stale health beside a success or failure
toast.
When that same platform workspace reports TrueNAS as unavailable, the disabled
state must mean the server has explicitly opted out of the default-on TrueNAS
integration, not that operators still need to enable a hidden feature gate for
normal product use.
That same API-backed platform workspace owner now also includes the shared
presentation helpers `frontend-modern/src/utils/clusterEndpointPresentation.ts`
and `frontend-modern/src/utils/proxmoxSettingsPresentation.ts`, so endpoint
reachability state, discovery-prefill defaults, and variant copy stay on the
same governed lifecycle surface instead of drifting into card-local strings or
prefill assembly.
That same platform-connections boundary also defines the agent-optional rule
for API-backed platforms. TrueNAS may surface Assistant control and runtime
insight through the backend-owned platform connection and polling path, but
adjacent lifecycle flows must not start treating a unified-agent install as
the required bootstrap for provider-backed TrueNAS operations.
That same agent-optional rule also covers Assistant diagnostics. Provider-
backed TrueNAS app log reads may route through shared AI/runtime wiring on the
platform connection and poller path, but lifecycle-adjacent setup/install
flows must not reframe those diagnostics as requiring unified-agent host
install before TrueNAS becomes operational in Pulse.
That same agent-optional rule also covers Assistant configuration reads.
Provider-backed TrueNAS app config may route through shared AI/runtime wiring
on the platform connection and poller path, but lifecycle-adjacent
setup/install flows must not reframe those config reads as requiring
unified-agent host install before TrueNAS becomes operational in Pulse.
That same platform-connections boundary now also defines the only acceptable
phase-1 VMware onboarding path. If `vmware-vsphere` implementation starts,
`PlatformConnectionsWorkspace.tsx` must add `vCenter` under the shared
API-backed workspace, preserve the saved-connection test and health model, and
keep direct `ESXi` out of the phase-1 route and install model. Lifecycle-
adjacent flows must not invent a VMware-only setup shell or reframe unified-
agent host install as the bootstrap requirement for VMware support.
That same platform-connections boundary also owns demo/mock continuity for
those settings surfaces. When `/api/system/mock-mode` is enabled,
provider-backed settings panels and their downstream infrastructure,
workloads, storage, and recovery handoffs must read the canonical connection
fixtures from `internal/mock/fixture_graph.go` instead of handler-local demo
lists, so operator-facing demos stay coherent across those adjacent product
surfaces without a restart.
That same lifecycle-owned platform-connections boundary also owns configured
Proxmox, PBS, and PMG replacement continuity. Node update handlers must pass
the current platform surface into monitored-system admission through the shared
structured replacement selector so host or name edits preserve the intended
slot without reintroducing lifecycle-local matcher closures or empty-estate
fallbacks.
That same lifecycle-owned VMware workspace must also keep the backend-runtime
shape hidden behind one operator-facing connection model. The settings surface
may show one VMware connection's poll health, last error classification, and
observed contribution summary, but it must not force the operator to manage
separate Automation API versus VI JSON sessions or understand multi-client
runtime wiring just to use the shared platform-connections path.
That same shared connection model now also includes one runtime-health owner:
manual saved-connection retests with no edit overlay must refresh the same
poller-owned `poll` summary that ordinary list reads surface, while draft tests
and edit-form overlay tests stay non-persistent until a save succeeds.
Lifecycle-owned settings flows must not bring back a second `test` status model
or a VMware-only health fetch just to describe row health.
That lifecycle-owned VMware slice now also includes the first live runtime
handoff rule. `frontend-modern/src/components/Settings/VMwareSettingsPanel.tsx`,
`frontend-modern/src/components/Settings/useVMwareSettingsPanelState.ts`,
`frontend-modern/src/components/Settings/PlatformConnectionsWorkspace.tsx`,
`frontend-modern/src/components/Settings/InfrastructurePlatformConnectionsSummaryCard.tsx`,
and `internal/api/router.go` must keep VMware on the shared platform-
connections workflow with one `vCenter` connection family and one summary card
count. Lifecycle-adjacent install or discovery surfaces must not fork that
into a VMware-only install wizard, direct-ESXi setup branch, or agent-first
bootstrap story just because the runtime now has a live VMware connection
panel and poller.
That same shared router boundary must treat infrastructure summary chart
normalization as summary-only presentation transport: long-range chart bucket
shaping may improve operator-facing summary readability, but it must not be
reused as lifecycle freshness, heartbeat, or enrollment-state authority.
That same shared chart boundary may resolve provider-backed workload history
through unified metrics targets, but emitted workload IDs must stay on the
canonical `/workloads` row contract so lifecycle settings, reporting, and
handoff surfaces never depend on provider-native metric keys.
That same lifecycle-owned settings slice now also owns the shared VMware
summary and handoff framing. `InfrastructurePlatformConnectionsSummaryCard.tsx`,
`InfrastructureReportingPanel.tsx`, `useInfrastructureSettingsState.ts`, and
`useSettingsInfrastructurePanelProps.ts` must surface VMware availability and
connection counts from the same shared infrastructure settings state that owns
the VMware panel itself, rather than letting reporting cards or adjacent setup
surfaces grow a second VMware availability fetch or a VMware-only handoff path.
That same infrastructure workspace boundary now also owns the first-run
handoff copy for new operators. `InfrastructureWorkspace.tsx` must tell a new
Pulse user to start with `Install on a host` to add the first monitored
system, while still presenting `Platform connections` as the explicit
API-backed alternative path instead of leaving first-session install guidance
implicit in generic settings-shell prose or retreating to one provider's name
as the primary alternative.
When that infrastructure workspace needs to redirect operators to the Pulse Pro
surface for billing, monitored-system limits, or license status, it must
consume the settings-owned referral copy from
`frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts`
instead of carrying workspace-local commercial guidance or reaching back into
generic commercial presentation helpers from the hosted infrastructure route.
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
the encrypted canonical format on load.
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
infrastructure operations may point operators to Pulse Pro for billing, but it
must describe that boundary in monitored-system, plan-limit, and license-status
terms rather than reviving legacy agent-allocation language.
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
surface: `NodeModal.tsx` composes `NodeModalBasicInfoSection.tsx`,
`NodeModalAuthenticationSection.tsx`, `NodeModalMonitoringSection.tsx`,
`NodeModalStatusFooter.tsx`, `nodeModalModel.ts`, and `useNodeModalState.ts`.
That same node setup owner also includes
`frontend-modern/src/utils/nodeModalPresentation.ts`, which now owns the
canonical node-type defaults, endpoint/auth placeholders, monitoring coverage
copy, and test-result styling for PVE, PBS, and PMG setup.
That presentation layer remains presentation-only for those API-managed
Proxmox, PBS, and PMG connections. Lifecycle guidance in that settings surface
may explain monitored-system caps, but commercial enforcement still belongs to
the canonical add-node and `/api/auto-register` boundaries instead of
becoming a second modal-local exemption rule.
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
That same canonical request contract must also keep field-validation failures
specific: mismatched `tokenId`/`tokenValue` input may not collapse into
generic missing-field output, and other missing canonical fields must return
explicit `Missing required canonical auto-register fields: ...` guidance.
That same owned setup and auto-register boundary now also participates in the
canonical monitored-system commercial cap. A new `/api/auto-register`
completion may proceed only when it either dedupes onto an already-counted
top-level monitored system or fits within the deduped monitored-system limit;
the lifecycle surface may not preserve a special exemption that keeps API-
backed monitored systems outside the self-hosted commercial cap.
That admission decision must come from the same canonical prospective
monitored-system projection the runtime uses for final grouped counting.
Auto-register may preview its own candidate, but it must not keep a
lifecycle-local counter, drift on source priority, or treat missing monitored-
system usage as zero; when an active cap is present and usage cannot be
resolved, the route must fail closed with retryable unavailable guidance
instead of silently admitting a net-new monitored system.
When lifecycle-adjacent setup or support surfaces need to explain why a
candidate would count or dedupe, they must consume the shared monitored-system
ledger preview contract rather than rebuilding a second preview model from
setup-local transport fields. `frontend-modern/src/components/Settings/MonitoredSystemAdmissionPreview.tsx`
is the shared shell for that explanation inside platform-connections settings,
so provider-specific panels must not fork their own monitored-system preview
copy or inline projected-usage rendering.
That same commercial readiness boundary now also assumes settled canonical
usage, not the first non-nil monitor view. Lifecycle-owned setup or first-host
surfaces may not seal migrated-v5 continuity, display counted-system totals as
final, or retry admission against a provider-owned supplemental platform such
as TrueNAS or VMware until the monitor has both seen an initial baseline for
every active connection and rebuilt the canonical store at or after that
provider watermark.
That same lifecycle-owned admission surface now also requires canonical
unavailable-reason guidance. When `/api/license/monitored-system-ledger/preview`,
`/api/truenas/connections/preview`, `/api/truenas/connections/{id}/preview`,
`/api/vmware/connections/preview`, or
`/api/vmware/connections/{id}/preview` fail with
`monitored_system_usage_unavailable`, the backend must preserve
`details.reason` from the monitor usage contract and the shared admission
preview shell plus provider panels must render helper-owned retry copy and
disable save until preview can resolve again.
That same lifecycle-owned admission surface must keep provider save actions
gated on a successful monitored-system preview. TrueNAS and VMware settings
may not create or update a connection while the admission preview is missing,
loading, unavailable, errored, or over-limit, and save-time backend races must
reuse the same canonical preview/unavailable presentation state instead of
falling back to provider-local billing messages.
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
That same shared `internal/api/` dependency now also assumes auth persistence
compatibility is handled as an explicit migration/import boundary: legacy
raw-token `sessions.json` and `csrf_tokens.json` files may load for upgrade
continuity, but `session_store.go` and `csrf_store.go` must immediately
rewrite hashed canonical persistence during load instead of leaving raw-token
files on the primary runtime path until a later save side effect happens to
run.
That same shared `internal/api/` dependency also assumes local commercial-trial
handoff remains human-usable: lifecycle-adjacent trial CTAs may allow a short
burst of retries, but the backend contract must return the real remaining
backoff through `Retry-After` plus `details.retry_after_seconds` so setup and
install-adjacent surfaces do not drift into generic “try again later” behavior.
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
`AgentProfilesPanel.tsx` and `InfrastructureOperationsController.tsx` must resync profile state after
that rejection instead of flattening it into a generic assignment failure while
leaving stale profile options visible.
That same shared profile-management boundary must also fail closed on malformed
list payloads: `frontend-modern/src/api/agentProfiles.ts` may not silently
reinterpret non-array profile or assignment responses as an empty state, and
`useAgentProfilesPanelState.ts` / `InfrastructureOperationsController.tsx` must surface that load
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
presentation in the extracted node setup modal owner (`NodeModal.tsx`,
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
`node-setup-settings-surface` proof path across `NodeModal.tsx`,
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
same transport contract as `InfrastructureOperationsController.tsx`. In explicit insecure mode, that
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
aligned with `InfrastructureOperationsController.tsx`.
That same first-session setup-completion surface also owns the operator's v6
mental model for Unified Agent onboarding: `SetupCompletionPanel` must teach that one
Unified Agent install creates one canonical Pulse system resource first, then
layers workload discovery and API-linked platform context onto that same
inventory. It may not present Docker, Kubernetes, Proxmox, or TrueNAS as
competing primary onboarding paths, nor fall back to logo-led feature
brochure copy that obscures the unified-resource contract the wizard is
supposed to introduce.
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
`/settings/infrastructure/install` as the canonical completion path, rather
than assuming the legacy dashboard-only landing still defines successful
wizard completion.
That same `SetupCompletionPanel` boundary must also stay on the direct
`setup-completion-install-surface` proof path, rather than relying only on shared
helper coverage or downstream install tests to catch lifecycle drift in the
setup completion surface.
That same first-session browser proof must also exercise the explicit
`Platform connections` completion action through the real setup wizard flow
for API-backed starts like TrueNAS, rather than relying only on the preview
route or prose-level assertions to represent the API-backed alternative.
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
That same `ResultsStep` boundary must also stay on the direct
`deploy-fallback-install-surface` proof path, rather than relying only on the
shared install helper or downstream deploy tests to catch lifecycle drift in
the infrastructure fallback surface.
The same Windows install, upgrade, and uninstall copies must also preserve
operator-selected transport and capability toggles: if the settings surface
enables insecure TLS mode or Pulse command execution, the PowerShell path must
carry `PULSE_INSECURE_SKIP_VERIFY` and `PULSE_ENABLE_COMMANDS` through to the
installer where those settings apply, so Windows agents do not diverge from the
governed shell transport.
That same copied install transport must also normalize canonical base URLs
before composing installer asset paths: when operators enter a trailing-slash
Pulse URL, shell and PowerShell install commands must trim it before appending
`/install.sh` or `/install.ps1` so lifecycle transport does not drift onto
double-slash asset paths.
That same shared install-command transport must also fail closed on blank local
overrides: whitespace-only custom Pulse endpoint input in `InfrastructureOperationsController.tsx`
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
`frontend-modern/src/components/Settings/useInfrastructureReportingState.tsx`
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
