import { createMemo, type Accessor } from 'solid-js';
import type { Resource } from '@/types/resource';
import { requiresGovernedResourceDisplay } from '@/types/resource';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { getAgentStatusIndicator } from '@/utils/status';
import {
  getPlatformBadge,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import { buildWorkloadsHref } from '@/components/Infrastructure/workloadsLink';
import { buildServiceDetailLinks } from '@/components/Infrastructure/serviceDetailLinks';
import {
  getPrimaryResourceIdentity,
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
  formatInteger,
  toAgentFromResource,
  toDiscoveryConfig,
  toNodeFromProxmox,
  type AgentPlatformData,
  type DockerPlatformData,
  type KubernetesPlatformData,
  type PlatformData,
} from '@/components/Infrastructure/resourceDetailMappers';
import { formatIdentifierLabel } from '@/utils/textPresentation';

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
  const kubernetesCapabilityBadges = createMemo(() => {
    const capabilities = kubernetesMeta()?.metricCapabilities;
    if (!capabilities) return [] as Array<{ label: string; classes: string; title: string }>;

    const supportedBadge =
      'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400';
    const unsupportedBadge =
      'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted';
    const badges: { label: string; classes: string; title: string }[] = [];

    if (capabilities.nodeCpuMemory) {
      badges.push({
        label: 'K8s Node CPU/Memory',
        classes: supportedBadge,
        title: 'Node CPU and memory metrics are available.',
      });
    }
    if (capabilities.nodeTelemetry) {
      badges.push({
        label: 'Node Telemetry (Agent)',
        classes: supportedBadge,
        title: 'Linked Pulse agent provides node uptime, temperature, disk, network, and disk I/O.',
      });
    }
    if (capabilities.podCpuMemory) {
      badges.push({
        label: 'Pod CPU/Memory',
        classes: supportedBadge,
        title: 'Pod CPU and memory metrics are available.',
      });
    }
    if (capabilities.podNetwork) {
      badges.push({
        label: 'Pod Network',
        classes: supportedBadge,
        title: 'Pod network throughput is available.',
      });
    }
    if (capabilities.podEphemeralDisk) {
      badges.push({
        label: 'Pod Ephemeral Disk',
        classes: supportedBadge,
        title: 'Pod ephemeral storage usage is available.',
      });
    }
    if (!capabilities.podDiskIo) {
      badges.push({
        label: 'Pod Disk I/O Unsupported',
        classes: unsupportedBadge,
        title:
          'Pod disk read/write throughput is not collected by the Kubernetes integration path today.',
      });
    }

    return badges;
  });

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
      summary.push(`AI health ${intel.health.grade} · ${Math.round(intel.health.score)}/100`);
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
  const pbsJobTotal = createMemo(() => {
    const pbs = pbsData();
    if (!pbs) return 0;
    return (
      (pbs.backupJobCount || 0) +
      (pbs.syncJobCount || 0) +
      (pbs.verifyJobCount || 0) +
      (pbs.pruneJobCount || 0) +
      (pbs.garbageJobCount || 0)
    );
  });
  const pmgQueueBacklog = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return 0;
    return (pmg.queueDeferred || 0) + (pmg.queueHold || 0);
  });
  const pmgUpdatedRelative = createMemo(() => {
    const raw = pmgData()?.lastUpdated;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });
  const pbsJobBreakdown = createMemo(() => {
    const pbs = pbsData();
    if (!pbs) return [] as Array<{ label: string; value: number }>;
    return [
      { label: 'Backup', value: pbs.backupJobCount || 0 },
      { label: 'Sync', value: pbs.syncJobCount || 0 },
      { label: 'Verify', value: pbs.verifyJobCount || 0 },
      { label: 'Prune', value: pbs.pruneJobCount || 0 },
      { label: 'Garbage', value: pbs.garbageJobCount || 0 },
    ];
  });
  const pbsVisibleJobBreakdown = createMemo(() => {
    const all = pbsJobBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const pmgQueueBreakdown = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return [] as Array<{ label: string; value: number; warn?: boolean }>;
    return [
      { label: 'Active', value: pmg.queueActive || 0 },
      { label: 'Deferred', value: pmg.queueDeferred || 0, warn: (pmg.queueDeferred || 0) > 0 },
      { label: 'Hold', value: pmg.queueHold || 0, warn: (pmg.queueHold || 0) > 0 },
      { label: 'Incoming', value: pmg.queueIncoming || 0 },
    ];
  });
  const pmgVisibleQueueBreakdown = createMemo(() => {
    const all = pmgQueueBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });
  const pmgMailBreakdown = createMemo(() => {
    const pmg = pmgData();
    if (!pmg) return [] as Array<{ label: string; value: number }>;
    return [
      { label: 'Mail', value: pmg.mailCountTotal || 0 },
      { label: 'Spam', value: pmg.spamIn || 0 },
      { label: 'Virus', value: pmg.virusIn || 0 },
    ];
  });
  const pmgVisibleMailBreakdown = createMemo(() => {
    const all = pmgMailBreakdown();
    const nonZero = all.filter((entry) => entry.value > 0);
    return nonZero.length > 0 ? nonZero : all;
  });

  const mergedSources = createMemo(() => platformData()?.sources ?? []);
  const sourceStatus = createMemo<NonNullable<PlatformData['sourceStatus']>>(
    () => platformData()?.sourceStatus ?? {},
  );
  const sourceHealthSummary = createMemo(() => {
    const entries = Object.entries(sourceStatus());
    if (entries.length === 0) return null;

    let healthy = 0;
    let warning = 0;
    let unhealthy = 0;
    const parts: string[] = [];

    for (const [source, status] of entries) {
      const normalized = (status?.status || '').trim().toLowerCase();
      parts.push(`${source}:${normalized || 'unknown'}`);
      if (['online', 'running', 'healthy', 'connected', 'ok'].includes(normalized)) {
        healthy += 1;
      } else if (['degraded', 'warning', 'stale'].includes(normalized)) {
        warning += 1;
      } else {
        unhealthy += 1;
      }
    }

    const total = entries.length;
    if (unhealthy > 0) {
      return {
        label: `${unhealthy}/${total} unhealthy`,
        className: 'text-red-600 dark:text-red-400',
        title: parts.join(' • '),
      };
    }
    if (warning > 0) {
      return {
        label: `${warning}/${total} degraded`,
        className: 'text-amber-600 dark:text-amber-400',
        title: parts.join(' • '),
      };
    }
    return {
      label: `${healthy}/${total} healthy`,
      className: 'text-emerald-600 dark:text-emerald-400',
      title: parts.join(' • '),
    };
  });
  const sourceSummary = createMemo(() => {
    const health = sourceHealthSummary();
    if (health) return health;
    const sources = mergedSources();
    if (sources.length === 0) return null;
    return {
      label: sources.length === 1 ? sources[0].toUpperCase() : `${sources.length} sources`,
      className: 'text-base-content',
      title: sources.join(' • '),
    };
  });

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

    const discoveryMode =
      config.resourceType === 'agent'
        ? 'Host discovery'
        : `${formatIdentifierLabel(config.resourceType)} discovery`;

    return config.hostname ? `${discoveryMode} via ${config.hostname}` : discoveryMode;
  });

  const hostDetailCards = createMemo(() => {
    const cards: string[] = [];

    if (proxmoxNode()) {
      cards.push('system', 'hardware', 'storage');
    }

    const agent = agentInfo();
    if (agent) {
      cards.push('system', 'hardware');
      if ((agent.networkInterfaces?.length ?? 0) > 0) cards.push('network');
      if ((agent.disks?.length ?? 0) > 0) cards.push('disks');
      if ((agentMeta()?.raid?.length ?? 0) > 0) cards.push('raid');
      if (temperatureRows().length > 0) cards.push('temperatures');
    }

    return cards;
  });
  const hasHostDetails = createMemo(() => hostDetailCards().length > 0);
  const hostDetailSummary = createMemo(() => {
    const labels = Array.from(new Set(hostDetailCards()));
    if (labels.length === 0) return null;

    const categories =
      labels.length === 1
        ? labels[0]
        : labels.length === 2
          ? `${labels[0]} and ${labels[1]}`
          : `${labels.slice(0, -1).join(', ')}, and ${labels[labels.length - 1]}`;

    return `${hostDetailCards().length} detail card${hostDetailCards().length === 1 ? '' : 's'} covering ${categories}.`;
  });
  const hasServiceDetails = createMemo(
    () => resource.type === 'docker-host' || Boolean(pbsData()) || Boolean(pmgData()),
  );
  const serviceDetailsSummary = createMemo(() => {
    if (resource.type === 'docker-host') {
      return `${formatInteger(dockerContainerCount())} containers · ${formatInteger(dockerUpdatesAvailable())} updates`;
    }

    const pbs = pbsData();
    if (pbs) {
      return `${formatInteger(pbs.datastoreCount)} datastores · ${formatInteger(pbsJobTotal())} jobs`;
    }

    const pmg = pmgData();
    if (pmg) {
      return `${formatInteger(pmg.queueTotal)} queue total · ${formatInteger(pmgQueueBacklog())} backlog`;
    }

    return null;
  });

  const workloadsHref = createMemo(() => buildWorkloadsHref(resource));
  const headerIdentity = createMemo(() => getPrimaryResourceIdentity(resource));
  const relatedLinks = createMemo(() => {
    const links: Array<{ href: string; label: string; compactLabel: string; ariaLabel: string }> =
      [];
    const workloads = workloadsHref();
    if (workloads) {
      links.push({
        href: workloads,
        label: 'Open in Workloads',
        compactLabel: 'Workloads',
        ariaLabel: `Open related workloads for ${displayName()}`,
      });
    }
    links.push(...buildServiceDetailLinks(resource));
    const seen = new Set<string>();
    return links.filter((link) => {
      if (seen.has(link.href)) return false;
      seen.add(link.href);
      return true;
    });
  });
  const hasRuntimeOperationalContext = createMemo(
    () => kubernetesCapabilityBadges().length > 0 || relatedLinks().length > 0,
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
    headerIdentity,
    relatedLinks,
    hasRuntimeOperationalContext,
    sourceSections,
    identityMatchInfo,
    debugJson,
    tabs,
  };
};

export type ResourceDetailDrawerDerivedState = ReturnType<typeof useResourceDetailDrawerDerivedState>;
