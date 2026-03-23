import { createMemo, type Accessor } from 'solid-js';
import type { Resource } from '@/types/resource';
import { requiresGovernedResourceDisplay } from '@/types/resource';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { getAgentStatusIndicator } from '@/utils/status';
import {
  dedupeResourceBadges,
  getPlatformBadge,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import {
  getPrimaryResourceIdentityRows,
  getResourceIdentityAliases,
  getPreferredResourceClusterName,
  getPreferredResourceDisplayName,
} from '@/utils/resourceIdentity';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import {
  getResourcePolicyBadges,
  getResourcePolicyDisplayLabel,
  getResourcePolicyRedactionLabels,
  getResourceRoutingScopeLabel,
  hasDefaultResourcePolicyPosture,
} from '@/utils/resourcePolicyPresentation';
import type { ResourceIntelligence } from '@/types/aiIntelligence';
import {
  ALIAS_COLLAPSE_THRESHOLD,
  buildTemperatureRows,
  toAgentFromResource,
  toNodeFromProxmox,
  type AgentPlatformData,
  type DockerPlatformData,
  type KubernetesPlatformData,
  type PlatformData,
} from '@/components/Infrastructure/resourceDetailMappers';
import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import {
  buildPbsVisibleJobBreakdown,
  buildPmgVisibleMailBreakdown,
  buildPmgVisibleQueueBreakdown,
  getPbsJobTotal,
  getPmgQueueBacklog,
  getServiceDetailsSummary,
} from './resourceDetailDrawerServiceModel';
import {
  buildHostDetailCards,
  buildHostDetailSummary,
  buildKubernetesCapabilityBadges,
  buildRelatedLinks,
  buildSourceSummary,
  hasRuntimeOperationalContext as buildHasRuntimeOperationalContext,
} from './resourceDetailDrawerOperationalModel';

type DrawerTab = 'overview' | 'mail' | 'namespaces' | 'deployments' | 'swarm' | 'debug';

interface UseResourceDetailDrawerDerivedStateOptions {
  resource: Resource;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
  debugEnabled: Accessor<boolean>;
  resourceIntelligence: Accessor<ResourceIntelligence | null>;
}

export const useResourceDetailDrawerDerivedState = (
  options: UseResourceDetailDrawerDerivedStateOptions,
) => {
  const { resource, resolveResourceLabel: resolveResourceLabelInput, debugEnabled, resourceIntelligence } =
    options;

  const displayName = createMemo(() => getPreferredResourceDisplayName(resource));
  const kubernetesClusterName = createMemo(() => getPreferredResourceClusterName(resource) ?? '');
  const resolveResourceLabel = (resourceId: string): string =>
    resolveResourceLabelInput?.(resourceId)?.trim() || resourceId;
  const statusIndicator = createMemo(() => getAgentStatusIndicator({ status: resource.status }));
  const lastSeen = createMemo(() => formatRelativeTime(resource.lastSeen));
  const lastSeenAbsolute = createMemo(() => formatAbsoluteTime(resource.lastSeen));

  const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
  const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
  const typeBadge = createMemo(() => getTypeBadge(resource.type));
  const platformData = createMemo(() => resource.platformData as PlatformData | undefined);
  const unifiedSourceBadges = createMemo(() =>
    getUnifiedSourceBadges(platformData()?.sources ?? []),
  );
  const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
  const headerBadges = createMemo(() =>
    dedupeResourceBadges([
      typeBadge(),
      ...(hasUnifiedSources() ? unifiedSourceBadges() : [platformBadge(), sourceBadge()]),
    ]),
  );
  const policyBadges = createMemo(() => getResourcePolicyBadges(resource.policy));
  const policyRedactions = createMemo(() => getResourcePolicyRedactionLabels(resource.policy));
  const governanceSummary = createMemo(() =>
    requiresGovernedResourceDisplay(resource.policy)
      ? getResourcePolicyDisplayLabel(resource)
      : (resource.aiSafeSummary?.trim() ?? ''),
  );
  const hasGovernanceData = createMemo(
    () =>
      !hasDefaultResourcePolicyPosture(resource.policy) &&
      (policyBadges().length > 0 || Boolean(governanceSummary())),
  );

  const agentMeta = createMemo(
    () => resource.agent ?? (platformData()?.agent as AgentPlatformData | undefined),
  );
  const kubernetesMeta = createMemo(
    () => resource.kubernetes ?? (platformData()?.kubernetes as KubernetesPlatformData | undefined),
  );
  const kubernetesCapabilityBadges = createMemo(() =>
    buildKubernetesCapabilityBadges(kubernetesMeta()?.metricCapabilities),
  );

  const proxmoxNode = createMemo(() => toNodeFromProxmox(resource));
  const agentInfo = createMemo(() => toAgentFromResource(resource, agentMeta()));
  const temperatureRows = createMemo(() => buildTemperatureRows(agentInfo()?.sensors));

  const dockerHostData = createMemo(() => platformData()?.docker as DockerPlatformData | undefined);
  const dockerHostSourceId = createMemo(
    () => (dockerHostData()?.hostSourceId || '').trim() || null,
  );
  const dockerUpdatesAvailable = createMemo(() => dockerHostData()?.updatesAvailableCount ?? 0);
  const dockerContainerCount = createMemo(() => dockerHostData()?.containerCount ?? 0);
  const dockerUpdatesCheckedRelative = createMemo(() => {
    const raw = dockerHostData()?.updatesLastCheckedAt;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });
  const dockerHostCommand = createMemo(() => dockerHostData()?.command);
  const dockerHostCommandActive = createMemo(() => {
    const status = (dockerHostCommand()?.status || '').trim().toLowerCase();
    return ['queued', 'dispatched', 'acknowledged', 'in_progress'].includes(status);
  });
  const dockerUpdateActionsDisabled = createMemo(() => shouldHideDockerUpdateActions());
  const dockerUpdateActionsLoading = createMemo(() => !areSystemSettingsLoaded());
  const dockerSwarmInfo = createMemo(() => dockerHostData()?.swarm);
  const dockerSwarmClusterKey = createMemo(() => {
    const swarm = dockerSwarmInfo();
    return (swarm?.clusterName || swarm?.clusterId || '').trim();
  });

  const resourceDependencies = createMemo(() => resourceIntelligence()?.dependencies ?? []);
  const resourceDependents = createMemo(() => resourceIntelligence()?.dependents ?? []);
  const resourceCorrelations = createMemo(() => resourceIntelligence()?.correlations ?? []);
  const hasMeaningfulResourceIntelligence = createMemo(() => {
    const intel = resourceIntelligence();
    if (!intel) return false;

    return (
      (intel.health.score ?? 100) < 100 ||
      intel.health.trend !== 'stable' ||
      (intel.health.prediction?.trim() ?? '') !== '' ||
      (intel.health.factors?.length ?? 0) > 0 ||
      (intel.note_count ?? 0) > 0 ||
      (intel.recent_changes?.length ?? 0) > 0 ||
      resourceDependencies().length > 0 ||
      resourceDependents().length > 0 ||
      resourceCorrelations().length > 0
    );
  });
  const hasCorrelationContext = createMemo(
    () =>
      resourceDependencies().length > 0 ||
      resourceDependents().length > 0 ||
      resourceCorrelations().length > 0,
  );
  const hasInvestigationContext = createMemo(
    () => hasMeaningfulResourceIntelligence() || hasGovernanceData(),
  );
  const investigationContextSummary = createMemo(() => {
    const intel = resourceIntelligence();
    const summary: string[] = [];

    if (intel && hasMeaningfulResourceIntelligence()) {
      summary.push(`AI ${intel.health.grade} · ${Math.round(intel.health.score)}/100`);
    }
    if (resourceCorrelations().length > 0) {
      summary.push(
        `${resourceCorrelations().length} correlation${resourceCorrelations().length === 1 ? '' : 's'}`,
      );
    }
    if (resource.policy?.routing.scope && !hasDefaultResourcePolicyPosture(resource.policy)) {
      summary.push(`Routing ${getResourceRoutingScopeLabel(resource.policy.routing.scope)}`);
    }

    return summary.join(' · ');
  });

  const pbsData = createMemo(() => platformData()?.pbs);
  const pmgData = createMemo(() => platformData()?.pmg);
  const pbsJobTotal = createMemo(() => getPbsJobTotal(pbsData()));
  const pmgQueueBacklog = createMemo(() => getPmgQueueBacklog(pmgData()));
  const pmgUpdatedRelative = createMemo(() => {
    const raw = pmgData()?.lastUpdated;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });
  const pbsVisibleJobBreakdown = createMemo(() => buildPbsVisibleJobBreakdown(pbsData()));
  const pmgVisibleQueueBreakdown = createMemo(() => buildPmgVisibleQueueBreakdown(pmgData()));
  const pmgVisibleMailBreakdown = createMemo(() => buildPmgVisibleMailBreakdown(pmgData()));

  const mergedSources = createMemo(() => platformData()?.sources ?? []);
  const sourceStatus = createMemo<NonNullable<PlatformData['sourceStatus']>>(
    () => platformData()?.sourceStatus ?? {},
  );
  const sourceSummary = createMemo(() => buildSourceSummary(mergedSources(), sourceStatus()));

  const identityAliasValues = createMemo(() => getResourceIdentityAliases(resource));
  const identityIpValues = createMemo(() => resource.identity?.ips ?? []);
  const primaryIdentityRows = createMemo(() => getPrimaryResourceIdentityRows(resource));
  const identityCardHasRichData = createMemo(
    () =>
      primaryIdentityRows().length > 0 ||
      identityIpValues().length > 0 ||
      (resource.tags?.length || 0) > 0 ||
      identityAliasValues().length > 0,
  );
  const aliasPreviewValues = createMemo(() =>
    identityAliasValues().slice(0, ALIAS_COLLAPSE_THRESHOLD),
  );
  const hasAliasOverflow = createMemo(
    () => identityAliasValues().length > ALIAS_COLLAPSE_THRESHOLD,
  );
  const hasMergedSources = createMemo(() => mergedSources().length > 1);
  const discoveryConfig = createMemo(() => toDiscoveryConfig(resource));
  const discoveryContextSummary = createMemo(() => {
    const config = discoveryConfig();
    if (!config) return null;

    if (config.resourceType === 'agent') {
      return null;
    }

    const discoveryMode =
      config.resourceType === 'agent'
        ? 'Host analysis'
        : `${formatIdentifierLabel(config.resourceType)} analysis`;

    return config.hostname ? `${discoveryMode} via ${config.hostname}` : discoveryMode;
  });

  const hostDetailCards = createMemo(() =>
    buildHostDetailCards({
      hasProxmoxNode: Boolean(proxmoxNode()),
      hasAgentDetails: Boolean(agentInfo()),
      networkInterfaceCount: agentInfo()?.networkInterfaces?.length ?? 0,
      diskCount: agentInfo()?.disks?.length ?? 0,
      raidCount: agentMeta()?.raid?.length ?? 0,
      temperatureRowCount: temperatureRows().length,
    }),
  );
  const hasHostDetails = createMemo(() => hostDetailCards().length > 0);
  const hostDetailSummary = createMemo(() => buildHostDetailSummary(hostDetailCards()));
  const hasServiceDetails = createMemo(
    () => resource.type === 'docker-host' || Boolean(pbsData()) || Boolean(pmgData()),
  );
  const serviceDetailsSummary = createMemo(() => {
    return getServiceDetailsSummary({
      resourceType: resource.type,
      docker: dockerHostData(),
      pbs: pbsData(),
      pmg: pmgData(),
    });
  });

  const relatedLinks = createMemo(() => buildRelatedLinks(resource, displayName()));
  const hasRuntimeOperationalContext = createMemo(
    () => buildHasRuntimeOperationalContext(kubernetesCapabilityBadges(), relatedLinks()),
  );

  const sourceSections = createMemo(() => {
    const data = platformData();
    if (!data) {
      return [] as Array<{ id: string; label: string; payload: unknown }>;
    }
    const sections = [
      { id: 'proxmox', label: 'Proxmox', payload: data.proxmox },
      { id: 'agent', label: 'Agent', payload: data.agent },
      { id: 'docker', label: 'Containers', payload: data.docker },
      { id: 'pbs', label: 'PBS', payload: data.pbs },
      { id: 'pmg', label: 'PMG', payload: data.pmg },
      { id: 'kubernetes', label: 'Kubernetes', payload: data.kubernetes },
      { id: 'metrics', label: 'Metrics', payload: data.metrics },
    ];
    return sections.filter((section) => section.payload !== undefined);
  });
  const identityMatchInfo = createMemo(() => {
    const data = platformData();
    return (
      data?.identityMatch ??
      data?.matchResults ??
      data?.matchCandidates ??
      data?.matches ??
      undefined
    );
  });
  const debugBundle = createMemo(() => ({
    resource,
    identity: {
      resourceIdentity: resource.identity,
      matchInfo: identityMatchInfo(),
    },
    sources: {
      sourceStatus: sourceStatus(),
      proxmox: platformData()?.proxmox,
      agent: platformData()?.agent,
      docker: platformData()?.docker,
      pbs: platformData()?.pbs,
      pmg: platformData()?.pmg,
      kubernetes: platformData()?.kubernetes,
      metrics: platformData()?.metrics,
    },
  }));
  const debugJson = createMemo(() => JSON.stringify(debugBundle(), null, 2));

  const tabs = createMemo(() => {
    const base = [
      { id: 'overview' as DrawerTab, label: 'Overview' },
      ...(resource.type === 'pmg' ? [{ id: 'mail' as DrawerTab, label: 'Mail' }] : []),
      ...(resource.type === 'k8s-cluster'
        ? [{ id: 'namespaces' as DrawerTab, label: 'Namespaces' }]
        : []),
      ...(resource.type === 'k8s-cluster'
        ? [{ id: 'deployments' as DrawerTab, label: 'Deployments' }]
        : []),
      ...(resource.type === 'docker-host' && dockerSwarmClusterKey()
        ? [{ id: 'swarm' as DrawerTab, label: 'Swarm' }]
        : []),
    ];
    if (debugEnabled()) {
      base.push({ id: 'debug' as DrawerTab, label: 'Debug' });
    }
    return base;
  });

  return {
    displayName,
    kubernetesClusterName,
    resolveResourceLabel,
    statusIndicator,
    lastSeen,
    lastSeenAbsolute,
    platformBadge,
    sourceBadge,
    typeBadge,
    platformData,
    unifiedSourceBadges,
    hasUnifiedSources,
    headerBadges,
    policyBadges,
    policyRedactions,
    governanceSummary,
    hasGovernanceData,
    agentMeta,
    kubernetesMeta,
    kubernetesCapabilityBadges,
    proxmoxNode,
    agentInfo,
    temperatureRows,
    dockerHostData,
    dockerHostSourceId,
    dockerUpdatesAvailable,
    dockerContainerCount,
    dockerUpdatesCheckedRelative,
    dockerHostCommand,
    dockerHostCommandActive,
    dockerUpdateActionsDisabled,
    dockerUpdateActionsLoading,
    dockerSwarmInfo,
    dockerSwarmClusterKey,
    resourceDependencies,
    resourceDependents,
    resourceCorrelations,
    hasCorrelationContext,
    hasMeaningfulResourceIntelligence,
    hasInvestigationContext,
    investigationContextSummary,
    pbsData,
    pmgData,
    pbsJobTotal,
    pmgQueueBacklog,
    pmgUpdatedRelative,
    pbsVisibleJobBreakdown,
    pmgVisibleQueueBreakdown,
    pmgVisibleMailBreakdown,
    mergedSources,
    sourceStatus,
    sourceSummary,
    identityIpValues,
    identityAliasValues,
    primaryIdentityRows,
    identityCardHasRichData,
    aliasPreviewValues,
    hasAliasOverflow,
    hasMergedSources,
    discoveryConfig,
    discoveryContextSummary,
    hasHostDetails,
    hostDetailSummary,
    hasServiceDetails,
    serviceDetailsSummary,
    relatedLinks,
    hasRuntimeOperationalContext,
    sourceSections,
    identityMatchInfo,
    debugJson,
    tabs,
  };
};

export type ResourceDetailDrawerDerivedState = ReturnType<typeof useResourceDetailDrawerDerivedState>;
