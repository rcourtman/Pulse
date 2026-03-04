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
import mentionAutocompleteSource from '@/components/AI/Chat/MentionAutocomplete.tsx?raw';

describe('agent model guardrails', () => {
  it('keeps AgentProfilesPanel on unified resources (not host-only slices)', () => {
    expect(agentProfilesPanelSource).toContain('const { resources } = useResources()');
    expect(agentProfilesPanelSource).not.toContain("const hosts = byType('host')");
  });

  it('keeps APITokenManager host usage mapped from unified agent-capable resources', () => {
    expect(apiTokenManagerSource).toContain('const hostAgentResources = createMemo(() =>');
    expect(apiTokenManagerSource).toContain('const hasAgentScopeResource = (resource: Resource)');
    expect(apiTokenManagerSource).toContain("resource.type === 'node'");
    expect(apiTokenManagerSource).toContain("resource.type === 'pbs'");
    expect(apiTokenManagerSource).toContain("resource.type === 'pmg'");
    expect(apiTokenManagerSource).toContain("resource.type === 'truenas'");
    expect(apiTokenManagerSource).toContain('resource.agent != null');
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
  });

  it('keeps discovery resource types on canonical v6 names', () => {
    expect(discoveryTypesSource).not.toContain("| 'lxc'");
    expect(discoveryTypesSource).not.toContain("| 'docker_lxc'");
  });

  it('keeps alerts host-agent thresholds sourced from unified host-agent resources', () => {
    expect(alertsPageSource).toContain('const hostAgentResources = createMemo(');
    expect(alertsPageSource).toContain('hosts={hostAgentResources()}');
  });

  it('keeps setup and node summary host fallbacks aware of v6 host-agent facets', () => {
    expect(completeStepSource).toContain('const hasHostAgentFacet = (resource: Resource)');
    expect(completeStepSource).toContain("resource.type === 'node'");
    expect(completeStepSource).toContain('command -v sudo');
    expect(completeStepSource).toContain('const hostLikeResources = resources.filter(');
    expect(completeStepSource).not.toContain('state.nodes || []');
    expect(completeStepSource).not.toContain('state.hosts || []');
    expect(completeStepSource).not.toContain(
      '(state.hosts || []).length > 0\n            ? state.hosts\n            : resources',
    );
    expect(unifiedNodeSelectorSource).toContain(
      'const hasHostAgentFacet = (resource: Resource): boolean =>',
    );
    expect(unifiedNodeSelectorSource).toContain("resource.type === 'truenas'");
    expect(unifiedNodeSelectorSource).not.toContain('if (hostLikeResources.length === 0');
  });

  it('keeps AI chat mention resources aware of host-agent facets beyond host type', () => {
    expect(aiChatSource).toContain('const hasHostAgentFacet = (resource: Resource): boolean =>');
    expect(aiChatSource).toContain("resource.type === 'truenas'");
    expect(aiChatSource).not.toContain("resource.type === 'host'");
    expect(aiChatSource).toContain("type: 'agent'");
    expect(aiChatSource).toContain("mention.type === 'agent' ? { ...mention, type: 'host' } : mention");
    expect(mentionAutocompleteSource).toContain("| 'agent'");
    expect(mentionAutocompleteSource).not.toContain("| 'host'");
  });

  it('keeps infrastructure host-agent selectors on v6 node-family resources', () => {
    expect(infrastructureSummarySource).not.toContain("resource.type === 'host'");
    expect(unifiedNodeSelectorComponentSource).not.toContain("resource.type === 'host'");
    expect(workloadsLinkSource).not.toContain("resource.type === 'host'");
    expect(unifiedResourceTableSource).toContain("buildMetricKey('agent', resource.id)");
    expect(unifiedResourceTableSource).not.toContain("buildMetricKey('host', resource.id)");
  });

  it('keeps diagnostics alerts contract free of removed legacy threshold flags', () => {
    expect(diagnosticsPanelSource).not.toContain('legacyThresholdsDetected');
    expect(diagnosticsPanelSource).not.toContain('legacyThresholdSources');
    expect(diagnosticsPanelSource).not.toContain('legacyScheduleSettings');
    expect(systemSettingsStateSource).not.toContain('legacyThresholdsDetected');
    expect(systemSettingsStateSource).not.toContain('legacyThresholdSources');
    expect(systemSettingsStateSource).not.toContain('legacyScheduleSettings');
  });

  it('keeps alert thresholds routes on v6 container path only', () => {
    expect(thresholdsTableSource).toContain(
      "if (path.includes('/thresholds/containers')) return 'docker';",
    );
    expect(thresholdsTableSource).not.toContain('/thresholds/docker');
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
    expect(resourceTypesSource).not.toContain("| 'host' // Standalone host (via host-agent)");
    expect(unifiedResourcesHookSource).toContain("case 'host':");
    expect(unifiedResourcesHookSource).toContain("return 'node';");
    expect(unifiedResourcesHookSource).not.toContain("return 'host';");
  });
});
