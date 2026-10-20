import { createMemo, type Accessor } from 'solid-js';
import type { Resource, ResourceRelationship } from '@/types/resource';
import type { HostNetworkInterface } from '@/types/api';
import { requiresGovernedResourceDisplay } from '@/types/resource';
import type { ResourceVMwareMeta } from '@/types/resource';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { getAgentStatusIndicator } from '@/utils/status';
import { mapDockerContainerStatus } from '@/features/docker/dockerPageModel';
import {
  dedupeResourceBadges,
  getInfrastructureSystemIdentityBadges,
  getPlatformBadge,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import {
  getPreferredResourceClusterName,
  getPreferredInfrastructureDisplayName,
} from '@/utils/resourceIdentity';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import { useAlertsActivation } from '@/stores/alertsActivation';
import {
  getResourcePolicyBadges,
  getResourcePolicyGovernedSummary,
  getResourcePolicyRedactionLabels,
  getResourceRoutingScopeLabel,
  hasDefaultResourcePolicyPosture,
} from '@/utils/resourcePolicyPresentation';
import { formatResourceAnalysisSummary } from '@/utils/resourceAnalysisPresentation';
import type { ResourceIntelligence } from '@/types/aiIntelligence';
import {
  buildCustomSensorRows,
  buildTemperatureRows,
  HOST_AGENT_STOPPED_REPORTING_REASON,
  toAgentFromResource,
  toNodeFromProxmox,
  type AgentPlatformData,
  type DockerPlatformData,
  type KubernetesPlatformData,
  type PlatformData,
} from '@/components/Infrastructure/resourceDetailMappers';
import {
  getResourceGuestReadPrecaution,
  toDiscoveryConfig,
} from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import {
  buildPbsActiveTasks,
  buildPbsVisibleJobBreakdown,
  buildPmgVisibleMailBreakdown,
  buildPmgVisibleQueueBreakdown,
  getPbsJobTotal,
  getPmgQueueBacklog,
  getServiceDetailsSummary,
} from './resourceDetailDrawerServiceModel';
import {
  buildAccessSummary,
  buildHostDetailCards,
  buildHostDetailSummary,
  buildKubernetesCapabilityBadges,
  buildRelatedLinks,
  buildSourceSummary,
  hasRuntimeOperationalContext as buildHasRuntimeOperationalContext,
} from './resourceDetailDrawerOperationalModel';
import {
  getResourceMetricsHistoryCurrentMetrics,
  getResourceMetricsHistoryDeferredMetrics,
  getResourceMetricsHistoryGroups,
  getResourceMetricsHistoryTarget,
  resourceSupportsMetricsHistory,
} from './resourceDetailDrawerMetricsHistoryModel';
import {
  buildIdentityMatchInfo,
  buildResourceDebugBundle,
  buildResourceIdentityView,
  buildSourceSections,
} from './resourceDetailDrawerIdentityModel';
import {
  buildVMwareDetailsSummary,
  buildVMwareDetailSections,
} from './resourceDetailDrawerVmwareModel';
import {
  buildTrueNASDetailsSummary,
  buildTrueNASDetailSections,
} from './resourceDetailDrawerTrueNASModel';
import {
  buildKubernetesDetailsSummary,
  buildKubernetesDetailSections,
} from './resourceDetailDrawerKubernetesModel';
import { getResourceHealthIssuePresentation } from './resourceHealthPresentation';

type DrawerTab = 'overview' | 'manage' | 'mail' | 'namespaces' | 'deployments' | 'swarm' | 'debug';

interface UseResourceDetailDrawerDerivedStateOptions {
  resource: Resource;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
  debugEnabled: Accessor<boolean>;
  resourceIntelligence: Accessor<ResourceIntelligence | null>;
  resourceRelationships?: Accessor<readonly ResourceRelationship[] | null | undefined>;
}

export const useResourceDetailDrawerDerivedState = (
  options: UseResourceDetailDrawerDerivedStateOptions,
) => {
  const resource = () => options.resource;
  const {
    resolveResourceLabel: resolveResourceLabelInput,
    debugEnabled,
    resourceIntelligence,
  } = options;

  const displayName = createMemo(() => getPreferredInfrastructureDisplayName(resource()));
  const kubernetesClusterName = createMemo(() => getPreferredResourceClusterName(resource()) ?? '');
  const resolveResourceLabel = (resourceId: string): string =>
    resolveResourceLabelInput?.(resourceId)?.trim() || resourceId;
  // A container's dot reads its run state and health check, the same mapper
  // its table row uses, so a running but unhealthy container is not green here
  // while red in the row above.
  const statusIndicator = createMemo(() =>
    resource().type === 'app-container'
      ? mapDockerContainerStatus(resource())
      : getAgentStatusIndicator({ status: resource().status }),
  );
  const healthIssue = createMemo(() => getResourceHealthIssuePresentation(resource()));
  // An open drawer keeps its rows while a resource's timestamps stay the same,
  // which is exactly when a silent resource's Last seen must keep aging, so
  // the drawer's ages read the shared clock.
  const now = useRelativeTimeNow();
  const lastSeen = createMemo(() => formatRelativeTime(resource().lastSeen, { now: now() }));
  const lastSeenAbsolute = createMemo(() => formatAbsoluteTime(resource().lastSeen));

  const platformBadge = createMemo(() => getPlatformBadge(resource().platformType));
  const sourceBadge = createMemo(() => getSourceBadge(resource().sourceType));
  const typeBadge = createMemo(() => getTypeBadge(resource().type));
  const platformData = createMemo(() => resource().platformData as PlatformData | undefined);
  const unifiedSourceBadges = createMemo(() => {
    const sources = resource().sources;
    return getUnifiedSourceBadges(
      Array.isArray(sources) && sources.length > 0 ? sources : (platformData()?.sources ?? []),
    );
  });
  const systemIdentityBadges = createMemo(() => getInfrastructureSystemIdentityBadges(resource()));
  const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
  const headerBadges = createMemo(() =>
    dedupeResourceBadges([
      typeBadge(),
      ...(systemIdentityBadges().length > 0
        ? systemIdentityBadges()
        : hasUnifiedSources()
          ? unifiedSourceBadges()
          : [platformBadge(), sourceBadge()]),
    ]),
  );
  const policyBadges = createMemo(() => getResourcePolicyBadges(resource().policy));
  const policyRedactions = createMemo(() => getResourcePolicyRedactionLabels(resource().policy));
  const governanceSummary = createMemo(() =>
    requiresGovernedResourceDisplay(resource().policy)
      ? getResourcePolicyGovernedSummary(resource())
      : (resource().aiSafeSummary?.trim() ?? ''),
  );
  const hasGovernanceData = createMemo(
    () =>
      !hasDefaultResourcePolicyPosture(resource().policy) &&
      (policyBadges().length > 0 || Boolean(governanceSummary())),
  );

  const agentMeta = createMemo(
    () => resource().agent ?? (platformData()?.agent as AgentPlatformData | undefined),
  );
  const kubernetesMeta = createMemo(
    () =>
      resource().kubernetes ?? (platformData()?.kubernetes as KubernetesPlatformData | undefined),
  );
  const vmwareData = createMemo(
    () => resource().vmware ?? (platformData()?.vmware as ResourceVMwareMeta | undefined),
  );
  const kubernetesCapabilityBadges = createMemo(() =>
    buildKubernetesCapabilityBadges(kubernetesMeta()?.metricCapabilities),
  );

  const proxmoxNode = createMemo(() => toNodeFromProxmox(resource()));
  const proxmoxNetworkInterfaces = createMemo(
    () =>
      resource().proxmox?.networkInterfaces ??
      (platformData()?.proxmox as { networkInterfaces?: HostNetworkInterface[] } | undefined)
        ?.networkInterfaces ??
      [],
  );
  const agentInfo = createMemo(() => toAgentFromResource(resource(), agentMeta()));
  // A silent agent's sensors and disk usage are its last report, even on a
  // row another source keeps online.
  const agentReadingsLastKnownReason = createMemo(() =>
    resource().agent?.stale ? HOST_AGENT_STOPPED_REPORTING_REASON : undefined,
  );
  const temperatureRows = createMemo(() =>
    buildTemperatureRows(agentInfo()?.sensors, {
      lastKnownReason: agentReadingsLastKnownReason(),
    }),
  );
  const customSensorRows = createMemo(() =>
    buildCustomSensorRows(agentInfo()?.sensors, {
      lastKnownReason: agentReadingsLastKnownReason(),
    }),
  );

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
    return formatRelativeTime(parsed, { now: now() });
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
  const resourceRelationships = createMemo(
    () => options.resourceRelationships?.() ?? resource().relationships ?? [],
  );
  const hasMeaningfulResourceIntelligence = createMemo(() => {
    const intel = resourceIntelligence();
    if (!intel) return false;

    return (
      (intel.health.score ?? 100) < 100 ||
      intel.health.trend !== 'stable' ||
      (intel.health.prediction?.trim() ?? '') !== '' ||
      (intel.health.factors?.length ?? 0) > 0 ||
      (intel.note_count ?? 0) > 0 ||
      (intel.recent_changes?.length ?? 0) > 0
    );
  });
  const hasCorrelationContext = createMemo(
    () =>
      resourceRelationships().length > 0 ||
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
      summary.push(formatResourceAnalysisSummary(intel.health.grade, intel.health.score));
    }
    const policy = resource().policy;
    if (policy?.routing.scope && !hasDefaultResourcePolicyPosture(policy)) {
      summary.push(`Routing ${getResourceRoutingScopeLabel(policy.routing.scope)}`);
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
    return formatRelativeTime(parsed, { now: now() });
  });
  const pbsVisibleJobBreakdown = createMemo(() => buildPbsVisibleJobBreakdown(pbsData()));
  const pbsActiveTasks = createMemo(() => buildPbsActiveTasks(pbsData()));
  const pbsActiveTaskCount = createMemo(() => pbsActiveTasks().length);
  const pmgVisibleQueueBreakdown = createMemo(() => buildPmgVisibleQueueBreakdown(pmgData()));
  const pmgVisibleMailBreakdown = createMemo(() => buildPmgVisibleMailBreakdown(pmgData()));

  const mergedSources = createMemo(() => platformData()?.sources ?? []);
  const sourceStatus = createMemo<NonNullable<PlatformData['sourceStatus']>>(
    () => platformData()?.sourceStatus ?? {},
  );
  const sourceSummary = createMemo(() => buildSourceSummary(mergedSources(), sourceStatus()));

  const identityView = createMemo(() => buildResourceIdentityView(resource()));
  const identityAliasValues = createMemo(() => identityView().identityAliasValues);
  const identityIpValues = createMemo(() => identityView().identityIpValues);
  const primaryIdentityRows = createMemo(() => identityView().primaryIdentityRows);
  const identityCardHasRichData = createMemo(() => identityView().identityCardHasRichData);
  const aliasPreviewValues = createMemo(() => identityView().aliasPreviewValues);
  const hasAliasOverflow = createMemo(() => identityView().hasAliasOverflow);
  const hasMergedSources = createMemo(() => mergedSources().length > 1);
  const discoveryConfig = createMemo(() => toDiscoveryConfig(resource()));
  const guestReadPrecaution = createMemo(() => getResourceGuestReadPrecaution(resource()));

  const hostDetailCards = createMemo(() =>
    buildHostDetailCards({
      hasProxmoxNode: Boolean(proxmoxNode()),
      hasAgentDetails: Boolean(agentInfo()),
      networkInterfaceCount: Math.max(
        agentInfo()?.networkInterfaces?.length ?? 0,
        proxmoxNetworkInterfaces().length,
      ),
      diskCount: agentInfo()?.disks?.length ?? 0,
      raidCount: agentMeta()?.raid?.length ?? 0,
      temperatureRowCount: temperatureRows().length + customSensorRows().length,
    }),
  );
  const hasHostDetails = createMemo(() => hostDetailCards().length > 0);
  const hostDetailSummary = createMemo(() => buildHostDetailSummary(hostDetailCards()));
  const hasServiceDetails = createMemo(
    () => resource().type === 'docker-host' || Boolean(pbsData()) || Boolean(pmgData()),
  );
  const serviceDetailsSummary = createMemo(() => {
    return getServiceDetailsSummary({
      resourceType: resource().type,
      docker: dockerHostData(),
      pbs: pbsData(),
      pmg: pmgData(),
    });
  });
  const vmwareDetailSections = createMemo(() =>
    buildVMwareDetailSections(resource().type, vmwareData()),
  );
  const hasVMwareDetails = createMemo(() => vmwareDetailSections().length > 0);
  const vmwareDetailsSummary = createMemo(() =>
    buildVMwareDetailsSummary(resource().type, vmwareData()),
  );
  const { getTrueNASDiskTemperatureThresholds } = useAlertsActivation();
  const trueNASDetailSections = createMemo(() =>
    buildTrueNASDetailSections(resource(), getTrueNASDiskTemperatureThresholds),
  );
  const hasTrueNASDetails = createMemo(() => trueNASDetailSections().length > 0);
  const trueNASDetailsSummary = createMemo(() => buildTrueNASDetailsSummary(resource()));
  const kubernetesDetailSections = createMemo(() => buildKubernetesDetailSections(resource()));
  const hasKubernetesDetails = createMemo(() => kubernetesDetailSections().length > 0);
  const kubernetesDetailsSummary = createMemo(() => buildKubernetesDetailsSummary(resource()));
  const metricsHistoryTarget = createMemo(() => getResourceMetricsHistoryTarget(resource()));
  const metricsHistoryCurrentMetrics = createMemo(() =>
    getResourceMetricsHistoryCurrentMetrics(resource()),
  );
  const metricsHistoryDeferredMetrics = createMemo(() =>
    getResourceMetricsHistoryDeferredMetrics(resource()),
  );
  const metricsHistoryGroups = createMemo(() => getResourceMetricsHistoryGroups(resource()));
  const hasMetricsHistory = createMemo(() => resourceSupportsMetricsHistory(resource()));
  const relatedLinks = createMemo(() => buildRelatedLinks(resource(), displayName()));
  const accessSummary = createMemo(() =>
    buildAccessSummary({
      hasWebInterface: Boolean(discoveryConfig()),
      links: relatedLinks(),
    }),
  );
  const hasAccessContext = createMemo(
    () => Boolean(discoveryConfig()) || relatedLinks().length > 0,
  );
  const hasRuntimeOperationalContext = createMemo(() =>
    buildHasRuntimeOperationalContext(kubernetesCapabilityBadges()),
  );

  const sourceSections = createMemo(() => buildSourceSections(platformData()));
  const identityMatchInfo = createMemo(() => buildIdentityMatchInfo(platformData()));
  const debugBundle = createMemo(() =>
    buildResourceDebugBundle({
      resource: resource(),
      platformData: platformData(),
      sourceStatus: sourceStatus(),
      identityMatchInfo: identityMatchInfo(),
    }),
  );
  const debugJson = createMemo(() => JSON.stringify(debugBundle(), null, 2));

  const tabs = createMemo(() => {
    const base = [
      // A resource with stored history charts it on Overview; there is no
      // separate History tab repeating those charts at another range.
      { id: 'overview' as DrawerTab, label: 'Overview' },
      { id: 'manage' as DrawerTab, label: 'Manage' },
      ...(resource().type === 'pmg' ? [{ id: 'mail' as DrawerTab, label: 'Mail' }] : []),
      ...(resource().type === 'k8s-cluster'
        ? [{ id: 'namespaces' as DrawerTab, label: 'Namespaces' }]
        : []),
      ...(resource().type === 'k8s-cluster'
        ? [{ id: 'deployments' as DrawerTab, label: 'Deployments' }]
        : []),
      ...(resource().type === 'docker-host' && dockerSwarmClusterKey()
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
    healthIssue,
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
    vmwareData,
    kubernetesCapabilityBadges,
    proxmoxNode,
    proxmoxNetworkInterfaces,
    agentInfo,
    agentReadingsLastKnownReason,
    temperatureRows,
    customSensorRows,
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
    resourceRelationships,
    hasCorrelationContext,
    hasMeaningfulResourceIntelligence,
    hasInvestigationContext,
    investigationContextSummary,
    pbsData,
    pmgData,
    pbsJobTotal,
    pbsActiveTasks,
    pbsActiveTaskCount,
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
    accessSummary,
    hasAccessContext,
    hasHostDetails,
    hostDetailSummary,
    hasServiceDetails,
    serviceDetailsSummary,
    metricsHistoryTarget,
    metricsHistoryCurrentMetrics,
    metricsHistoryDeferredMetrics,
    metricsHistoryGroups,
    hasMetricsHistory,
    guestReadPrecaution,
    hasVMwareDetails,
    vmwareDetailsSummary,
    vmwareDetailSections,
    hasTrueNASDetails,
    trueNASDetailsSummary,
    trueNASDetailSections,
    hasKubernetesDetails,
    kubernetesDetailsSummary,
    kubernetesDetailSections,
    relatedLinks,
    hasRuntimeOperationalContext,
    sourceSections,
    identityMatchInfo,
    debugJson,
    tabs,
  };
};
