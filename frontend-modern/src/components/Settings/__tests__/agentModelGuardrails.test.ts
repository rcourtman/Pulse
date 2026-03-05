import { describe, expect, it } from 'vitest';
import agentProfilesPanelSource from '../AgentProfilesPanel.tsx?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import unifiedAgentsSource from '../UnifiedAgents.tsx?raw';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import completeStepSource from '@/components/SetupWizard/steps/CompleteStep.tsx?raw';
import unifiedNodeSelectorSource from '@/components/shared/UnifiedNodeSelector.tsx?raw';
import aiChatSource from '@/components/AI/Chat/index.tsx?raw';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import systemSettingsStateSource from '../useSystemSettingsState.ts?raw';
import infrastructureSettingsStateSource from '../useInfrastructureSettingsState.ts?raw';
import infrastructureSummarySource from '@/components/Infrastructure/InfrastructureSummary.tsx?raw';
import unifiedNodeSelectorComponentSource from '@/components/shared/UnifiedNodeSelector.tsx?raw';
import workloadsLinkSource from '@/components/Infrastructure/workloadsLink.ts?raw';
import unifiedResourceTableSource from '@/components/Infrastructure/UnifiedResourceTable.tsx?raw';
import thresholdsTableSource from '@/components/Alerts/ThresholdsTable.tsx?raw';
import loginSource from '@/components/Login.tsx?raw';
import settingsSource from '../Settings.tsx?raw';
import aiSettingsSource from '../AISettings.tsx?raw';
import appSource from '@/App.tsx?raw';
import apiTypesSource from '@/types/api.ts?raw';
import aiTypesSource from '@/types/ai.ts?raw';
import resourceTypesSource from '@/types/resource.ts?raw';
import unifiedResourcesHookSource from '@/hooks/useUnifiedResources.ts?raw';
import discoveryTypesSource from '@/types/discovery.ts?raw';
import discoveryTargetUtilsSource from '@/utils/discoveryTarget.ts?raw';
import mentionAutocompleteSource from '@/components/AI/Chat/MentionAutocomplete.tsx?raw';
import commandPaletteSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import organizationSharingPanelSource from '../OrganizationSharingPanel.tsx?raw';
import resourceLinksSource from '@/routing/resourceLinks.ts?raw';
import alertsApiSource from '@/api/alerts.ts?raw';
import licenseApiSource from '@/api/license.ts?raw';
import monitoringApiSource from '@/api/monitoring.ts?raw';
import chartsApiSource from '@/api/charts.ts?raw';
import resourceStateAdaptersSource from '@/utils/resourceStateAdapters.ts?raw';
import resourceDetailMappersSource from '@/components/Infrastructure/resourceDetailMappers.ts?raw';
import resourceBadgesSource from '@/components/Infrastructure/resourceBadges.ts?raw';
import problemResourcesTableSource from '@/pages/DashboardPanels/ProblemResourcesTable.tsx?raw';
import containerUpdatesSource from '@/stores/containerUpdates.ts?raw';
import websocketStoreSource from '@/stores/websocket.ts?raw';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import guestDrawerSource from '@/components/Dashboard/GuestDrawer.tsx?raw';

describe('agent model guardrails', () => {
  it('keeps AgentProfilesPanel on unified resources (not host-only slices)', () => {
    expect(agentProfilesPanelSource).toContain('const { resources } = useResources()');
    expect(agentProfilesPanelSource).not.toContain("const hosts = byType('host')");
  });

  it('keeps APITokenManager runtime/agent usage mapped from unified resources', () => {
    expect(apiTokenManagerSource).toContain('const agentCapableResources = createMemo(() =>');
    expect(apiTokenManagerSource).toContain("const dockerRuntimeResources = createMemo(() => byType('docker-host'))");
    expect(apiTokenManagerSource).toContain('const hasAgentScopeResource = (resource: Resource)');
    expect(apiTokenManagerSource).toContain("resource.type === 'node'");
    expect(apiTokenManagerSource).toContain("resource.type === 'pbs'");
    expect(apiTokenManagerSource).toContain("resource.type === 'pmg'");
    expect(apiTokenManagerSource).toContain("resource.type === 'truenas'");
    expect(apiTokenManagerSource).toContain('resource.agent != null');
    expect(apiTokenManagerSource).toContain('markDockerRuntimesTokenRevoked');
    expect(apiTokenManagerSource).not.toContain('markDockerHostsTokenRevoked');
    expect(apiTokenManagerSource).not.toContain("resource.type === 'host'");
    expect(apiTokenManagerSource).not.toContain('markHostsTokenRevoked');
    expect(apiTokenManagerSource).not.toContain(
      "const hostResources = createMemo(() => byType('host'))",
    );
  });

  it('keeps UnifiedAgents free of v5 merge-workaround patterns', () => {
    expect(unifiedAgentsSource).not.toContain('previousHostTypes');
    expect(unifiedAgentsSource).not.toContain('const allHosts = createMemo(');
    expect(unifiedAgentsSource).toContain('No agent IDs available for removal');
    expect(unifiedAgentsSource).toContain('withPrivilegeEscalation');
  });

  it('keeps websocket State contract resource-first without legacy per-type arrays', () => {
    expect(apiTypesSource).toContain('export interface State {');
    expect(apiTypesSource).toContain('resources: Resource[]');
    expect(apiTypesSource).not.toContain('nodes: Node[]');
    expect(apiTypesSource).not.toContain('vms: VM[]');
    expect(apiTypesSource).not.toContain('containers: Container[]');
    expect(apiTypesSource).not.toContain('dockerHosts: DockerHost[]');
    expect(apiTypesSource).not.toContain('hosts: Host[]');
    expect(apiTypesSource).not.toContain('storage: Storage[]');
    expect(apiTypesSource).not.toContain('pbs: PBSInstance[]');
    expect(apiTypesSource).not.toContain('pmg: PMGInstance[]');
    expect(apiTypesSource).not.toContain('replicationJobs: ReplicationJob[]');
    expect(apiTypesSource).toContain('export interface DockerRuntime');
    expect(apiTypesSource).not.toContain('export interface DockerHost');
  });

  it('keeps discovery resource types on canonical v6 names', () => {
    expect(discoveryTypesSource).not.toContain("| 'lxc'");
    expect(discoveryTypesSource).not.toContain("| 'docker_lxc'");
    expect(discoveryTargetUtilsSource).toContain('discoveryTarget.agentId');
    expect(unifiedResourcesHookSource).toContain('const discoveryAgentId = v2.discoveryTarget?.agentId;');
    expect(apiTypesSource).toContain("export type GuestType = 'vm' | 'system-container';");
    expect(apiTypesSource).not.toContain("'qemu'");
    expect(apiTypesSource).not.toContain("'lxc'");
    expect(commandPaletteSource).not.toContain("'lxc'");
    expect(organizationSharingPanelSource).toContain("'system-container'");
    expect(organizationSharingPanelSource).toContain("'app-container'");
    expect(organizationSharingPanelSource).not.toContain("'container',");
  });

  it('keeps alerts agent thresholds sourced from unified agent resources', () => {
    expect(alertsPageSource).toContain('const agentResources = createMemo(');
    expect(alertsPageSource).toContain('agents={agentResources()}');
    expect(alertsPageSource).toContain("resourceType: 'Agent Disk'");
    expect(alertsPageSource).not.toContain("resourceType: 'Host Disk'");
    expect(alertsPageSource).toContain('agentDefaults');
    expect(alertsPageSource).toContain('disableAllAgents');
    expect(alertsPageSource).not.toContain('hostDefaults');
    expect(alertsPageSource).not.toContain('disableAllHosts');
  });

  it('keeps setup and node summary fallbacks aware of v6 agent facets', () => {
    expect(completeStepSource).toContain('const hasAgentFacet = (resource: Resource)');
    expect(completeStepSource).toContain("resource.type === 'node'");
    expect(completeStepSource).toContain('command -v sudo');
    expect(completeStepSource).toContain('const agentFacetResources = resources.filter(');
    expect(completeStepSource).not.toContain('state.nodes || []');
    expect(completeStepSource).not.toContain('state.hosts || []');
    expect(completeStepSource).not.toContain(
      '(state.hosts || []).length > 0\n            ? state.hosts\n            : resources',
    );
    expect(unifiedNodeSelectorSource).toContain(
      'const hasAgentFacet = (resource: Resource): boolean =>',
    );
    expect(unifiedNodeSelectorSource).toContain("resource.type === 'truenas'");
    expect(unifiedNodeSelectorSource).not.toContain('if (hostLikeResources.length === 0');
  });

  it('keeps discovery mapping on canonical agentId only', () => {
    expect(resourceDetailMappersSource).toContain('agentId: explicitDiscoveryAgentId');
    expect(resourceDetailMappersSource).not.toContain('hostId: explicitDiscoveryAgentId');
    expect(unifiedResourcesHookSource).toContain('const discoveryAgentId = v2.discoveryTarget?.agentId;');
  });

  it('keeps AI chat mention resources aware of agent facets beyond host type', () => {
    expect(aiChatSource).toContain('const hasAgentFacet = (resource: Resource): boolean =>');
    expect(aiChatSource).toContain("resource.type === 'truenas'");
    expect(aiChatSource).not.toContain("resource.type === 'host'");
    expect(aiChatSource).toContain("type: 'agent'");
    expect(aiChatSource).not.toContain("mention.type === 'agent'");
    expect(aiChatSource).toContain('const mentionsForAPI = mentions.length > 0 ? mentions : undefined;');
    expect(mentionAutocompleteSource).toContain("| 'agent'");
    expect(mentionAutocompleteSource).toContain("| 'system-container'");
    expect(mentionAutocompleteSource).not.toContain("| 'container'");
    expect(mentionAutocompleteSource).not.toContain("| 'host'");
  });

  it('keeps infrastructure agent selectors on v6 node-family resources', () => {
    expect(infrastructureSummarySource).not.toContain("resource.type === 'host'");
    expect(unifiedNodeSelectorComponentSource).not.toContain("resource.type === 'host'");
    expect(workloadsLinkSource).not.toContain("resource.type === 'host'");
    expect(unifiedResourceTableSource).toContain("buildMetricKey('agent', resource.id)");
    expect(unifiedResourceTableSource).not.toContain("buildMetricKey('host', resource.id)");
    expect(resourceBadgesSource).not.toContain("host: 'Agent'");
    expect(problemResourcesTableSource).not.toContain("host: 'Agent'");
  });

  it('keeps diagnostics alerts contract free of removed legacy threshold flags', () => {
    expect(diagnosticsPanelSource).not.toContain('legacyThresholdsDetected');
    expect(diagnosticsPanelSource).not.toContain('legacyThresholdSources');
    expect(diagnosticsPanelSource).not.toContain('legacyScheduleSettings');
    expect(systemSettingsStateSource).toContain('agentCount: number;');
    expect(systemSettingsStateSource).toContain('agentsTotal: number;');
    expect(systemSettingsStateSource).not.toContain('legacyThresholdsDetected');
    expect(systemSettingsStateSource).not.toContain('legacyThresholdSources');
    expect(systemSettingsStateSource).not.toContain('legacyScheduleSettings');
  });

  it('keeps alert thresholds routes on v6 container path only', () => {
    expect(thresholdsTableSource).toContain(
      "if (path.includes('/thresholds/containers')) return 'docker';",
    );
    expect(thresholdsTableSource).toContain('/thresholds/agents');
    expect(thresholdsTableSource).toContain("resourceType: 'Agent Disk'");
    expect(thresholdsTableSource).toContain('title="Agent Disks"');
    expect(thresholdsTableSource).not.toContain("resourceType: 'Host Disk'");
    expect(thresholdsTableSource).toContain('props.agentDefaults');
    expect(thresholdsTableSource).not.toContain('props.hostDefaults');
    expect(thresholdsTableSource).not.toContain('timeThresholds().host');
    expect(thresholdsTableSource).not.toContain('/thresholds/docker');
  });

  it('keeps workloads routing on canonical v6 agent query param only', () => {
    expect(resourceLinksSource).toContain("agent: 'agent'");
    expect(resourceLinksSource).not.toContain('LEGACY_WORKLOADS_HOST_QUERY_PARAM');
    expect(resourceLinksSource).not.toContain('canonicalHost || legacyHost');
  });

  it('keeps alerts API adapter fully canonical v6', () => {
    expect(alertsApiSource).not.toContain('normalizeAlertConfigFromAPI');
    expect(alertsApiSource).not.toContain('serializeAlertConfigForAPI');
    expect(alertsApiSource).not.toContain('hostDefaults');
    expect(alertsApiSource).not.toContain('disableAllHosts');
    expect(alertsApiSource).not.toContain('timeThresholds.host');
    expect(alertsApiSource).toContain('body: JSON.stringify(config)');
  });

  it('keeps license API free of removed legacy exchange surface', () => {
    expect(licenseApiSource).not.toContain('exchangeLicense(');
    expect(licenseApiSource).not.toContain('/exchange');
    expect(licenseApiSource).not.toContain('existing_jwt');
  });

  it('keeps monitoring API container-runtime errors on v6 naming', () => {
    expect(monitoringApiSource).toContain('Container runtime not found');
    expect(monitoringApiSource).not.toContain('Docker host not found');
    expect(apiTypesSource).toContain('export interface DockerRuntimeCommand');
    expect(apiTypesSource).not.toContain('export interface DockerHostCommand');
    expect(monitoringApiSource).toContain('deleteDockerRuntime');
    expect(monitoringApiSource).toContain('allowDockerRuntimeReenroll');
    expect(monitoringApiSource).not.toContain('deleteDockerHost');
    expect(monitoringApiSource).not.toContain('allowDockerHostReenroll');
    expect(monitoringApiSource).toContain('body: JSON.stringify({ agentId, containerId, containerName })');
    expect(monitoringApiSource).not.toContain('body: JSON.stringify({ hostId, containerId, containerName })');
    expect(monitoringApiSource).toContain('agentId?: string;');
    expect(monitoringApiSource).not.toContain('hostId?: string;');
    expect(monitoringApiSource).toContain('/agents/docker/runtimes/');
    expect(monitoringApiSource).not.toContain('/agents/docker/hosts/');
  });

  it('keeps container update actions wired to agentId naming only', () => {
    expect(containerUpdatesSource).toContain('export function syncWithAgentCommand');
    expect(containerUpdatesSource).not.toContain('export const syncWithHostCommand');
    expect(websocketStoreSource).toContain('syncWithAgentCommand(agentId, command as any)');
    expect(websocketStoreSource).not.toContain('syncWithHostCommand(hostId, command as any)');
    expect(websocketStoreSource).toContain('markDockerRuntimesTokenRevoked');
    expect(websocketStoreSource).toContain("markTokenRevoked('dockerRuntimes', tokenId, agentIds)");
    expect(guestRowSource).toContain('agentId={getWorkloadDockerHostId(props.guest)}');
    expect(guestRowSource).not.toContain('hostId={getWorkloadDockerHostId(props.guest)}');
    expect(guestDrawerSource).toContain('agentId={discoveryAgentId()}');
    expect(guestDrawerSource).not.toContain('hostId={discoveryHostId()}');
  });

  it('keeps chart API contract on canonical agentData naming', () => {
    expect(chartsApiSource).toContain('agentData?: Record<string, ChartData>');
    expect(chartsApiSource).not.toContain('hostData?: Record<string, ChartData>');
    expect(chartsApiSource).toContain('agents?: number;');
    expect(chartsApiSource).not.toContain('hosts?: number;');
    expect(chartsApiSource).toContain("| 'system-container'");
    expect(chartsApiSource).toContain("| 'app-container'");
    expect(chartsApiSource).not.toContain("| 'container'");
  });

  it('keeps login and SSO settings on provider-based flows only', () => {
    expect(loginSource).not.toContain('Legacy OIDC fallback');
    expect(loginSource).not.toContain('startOidcLogin');
    expect(settingsSource).not.toContain('<OIDCPanel');
    expect(settingsSource).not.toContain(
      "stateHosts={(state.resources ?? []).filter((r) => r.type === 'host')}",
    );
    expect(infrastructureSettingsStateSource).not.toContain("byType('host')");
    expect(appSource).not.toContain('oidcEnabled');
    expect(appSource).not.toContain('oidcUsername');
  });

  it('keeps AI settings/types free of removed legacy autonomous and schedule fields', () => {
    expect(aiTypesSource).not.toContain('autonomous_mode');
    expect(aiTypesSource).not.toContain('patrol_schedule_preset');
    expect(aiSettingsSource).not.toContain('settings_ai_autonomous_mode');
  });

  it('keeps unified resource typing normalized to node (no legacy host type)', () => {
    expect(resourceTypesSource).not.toContain("| 'host' // Standalone host (via agent)");
    expect(unifiedResourcesHookSource).not.toContain("case 'host':");
    expect(unifiedResourcesHookSource).toContain("case 'agent':");
    expect(unifiedResourcesHookSource).toContain("return 'node';");
    expect(unifiedResourcesHookSource).not.toContain("return 'host';");
  });

  it('keeps node link IDs on canonical linkedAgentId only', () => {
    expect(apiTypesSource).toContain('linkedAgentId?: string');
    expect(apiTypesSource).not.toContain('linkedHostAgentId');
    expect(resourceStateAdaptersSource).toContain('linkedAgentId: asString(platform?.linkedAgentId)');
    expect(resourceStateAdaptersSource).not.toContain('linkedHostAgentId');
  });
});
