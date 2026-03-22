import {
  createEffect,
  createMemo,
  createResource,
  createSignal,
  type Accessor,
  type Setter,
} from 'solid-js';
import type {
  Resource,
  ResourceChangeKind,
  ResourceChangeSourceAdapter,
  ResourceChangeSourceType,
} from '@/types/resource';
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
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { AIAPI } from '@/api/ai';
import { ResourceAPI } from '@/api/resources';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import {
  getResourcePolicyBadges,
  getResourcePolicyDisplayLabel,
  getResourcePolicyRedactionLabels,
  getResourceRoutingScopeLabel,
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

export interface UseResourceDetailDrawerStateOptions {
  resource: Resource;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
}

export interface UseResourceDetailDrawerStateResult {
  activeTab: Accessor<DrawerTab>;
  setActiveTab: Setter<DrawerTab>;
  debugEnabled: Accessor<boolean>;
  copied: Accessor<boolean>;
  showReportModal: Accessor<boolean>;
  setShowReportModal: Setter<boolean>;
  showInvestigationContext: Accessor<boolean>;
  setShowInvestigationContext: Setter<boolean>;
  showCorrelationContext: Accessor<boolean>;
  setShowCorrelationContext: Setter<boolean>;
  showDiscoveryContext: Accessor<boolean>;
  setShowDiscoveryContext: Setter<boolean>;
  showHostDetails: Accessor<boolean>;
  setShowHostDetails: Setter<boolean>;
  showServiceDetails: Accessor<boolean>;
  setShowServiceDetails: Setter<boolean>;
  showDockerUpdateControls: Accessor<boolean>;
  setShowDockerUpdateControls: Setter<boolean>;
  showPbsJobDetail: Accessor<boolean>;
  setShowPbsJobDetail: Setter<boolean>;
  showPmgMailFlowDetail: Accessor<boolean>;
  setShowPmgMailFlowDetail: Setter<boolean>;
  displayName: Accessor<string>;
  kubernetesClusterName: Accessor<string>;
  resolveResourceLabel: (resourceId: string) => string;
  statusIndicator: Accessor<ReturnType<typeof getAgentStatusIndicator>>;
  lastSeen: Accessor<string>;
  lastSeenAbsolute: Accessor<string>;
  platformBadge: Accessor<ReturnType<typeof getPlatformBadge>>;
  sourceBadge: Accessor<ReturnType<typeof getSourceBadge>>;
  typeBadge: Accessor<ReturnType<typeof getTypeBadge>>;
  unifiedSourceBadges: Accessor<ReturnType<typeof getUnifiedSourceBadges>>;
  hasUnifiedSources: Accessor<boolean>;
  policyBadges: Accessor<ReturnType<typeof getResourcePolicyBadges>>;
  policyRedactions: Accessor<string[]>;
  governanceSummary: Accessor<string>;
  hasGovernanceData: Accessor<boolean>;
  platformData: Accessor<PlatformData | undefined>;
  agentMeta: Accessor<AgentPlatformData | undefined>;
  kubernetesMeta: Accessor<KubernetesPlatformData | undefined>;
  kubernetesCapabilityBadges: Accessor<Array<{ label: string; classes: string; title: string }>>;
  proxmoxNode: Accessor<ReturnType<typeof toNodeFromProxmox>>;
  agentInfo: Accessor<ReturnType<typeof toAgentFromResource>>;
  temperatureRows: Accessor<ReturnType<typeof buildTemperatureRows>>;
  dockerHostData: Accessor<DockerPlatformData | undefined>;
  dockerHostSourceId: Accessor<string | null>;
  dockerUpdatesAvailable: Accessor<number>;
  dockerContainerCount: Accessor<number>;
  dockerUpdatesCheckedRelative: Accessor<string>;
  dockerHostCommand: Accessor<DockerPlatformData['command']>;
  dockerHostCommandActive: Accessor<boolean>;
  dockerUpdateActionsDisabled: Accessor<boolean>;
  dockerUpdateActionsLoading: Accessor<boolean>;
  dockerActionError: Accessor<string>;
  setDockerActionError: Setter<string>;
  dockerActionNote: Accessor<string>;
  setDockerActionNote: Setter<string>;
  confirmUpdateAll: Accessor<boolean>;
  setConfirmUpdateAll: Setter<boolean>;
  dockerActionBusy: Accessor<boolean>;
  setDockerActionBusy: Setter<boolean>;
  dockerSwarmInfo: Accessor<DockerPlatformData['swarm']>;
  dockerSwarmClusterKey: Accessor<string>;
  k8sDeploymentsPrefillNamespace: Accessor<string>;
  setK8sDeploymentsPrefillNamespace: Setter<string>;
  timelineKindFilter: Accessor<ResourceChangeKind | ''>;
  setTimelineKindFilter: Setter<ResourceChangeKind | ''>;
  timelineSourceTypeFilter: Accessor<ResourceChangeSourceType | ''>;
  setTimelineSourceTypeFilter: Setter<ResourceChangeSourceType | ''>;
  timelineSourceAdapterFilter: Accessor<ResourceChangeSourceAdapter | ''>;
  setTimelineSourceAdapterFilter: Setter<ResourceChangeSourceAdapter | ''>;
  resourceIntelligence: Accessor<ResourceIntelligence | null>;
  resourceDependencies: Accessor<ResourceIntelligence['dependencies']>;
  resourceDependents: Accessor<ResourceIntelligence['dependents']>;
  resourceCorrelations: Accessor<ResourceIntelligence['correlations']>;
  hasCorrelationContext: Accessor<boolean>;
  hasInvestigationContext: Accessor<boolean>;
  investigationContextSummary: Accessor<string>;
  pbsData: Accessor<PlatformData['pbs']>;
  pmgData: Accessor<PlatformData['pmg']>;
  pbsJobTotal: Accessor<number>;
  pmgQueueBacklog: Accessor<number>;
  pmgUpdatedRelative: Accessor<string>;
  pbsVisibleJobBreakdown: Accessor<Array<{ label: string; value: number }>>;
  pmgVisibleQueueBreakdown: Accessor<Array<{ label: string; value: number; warn?: boolean }>>;
  pmgVisibleMailBreakdown: Accessor<Array<{ label: string; value: number }>>;
  resourceTimeline: Accessor<Resource['recentChanges']>;
  historyFacetCounts: Accessor<
    Awaited<ReturnType<typeof ResourceAPI.getFacetBundle>>['counts'] | null
  >;
  historyRecentChanges: Accessor<Resource['recentChanges']>;
  hasTimelineFilters: Accessor<boolean>;
  historyLoadingLabel: Accessor<string>;
  resourceTimelineCount: Accessor<number>;
  sortedResourceTimeline: Accessor<Resource['recentChanges']>;
  facetBundleError: Accessor<string>;
  refetchHistoryFacets: () => unknown;
  mergedSources: Accessor<string[]>;
  sourceSummary: Accessor<{ label: string; className: string; title: string } | null>;
  identityAliasValues: Accessor<string[]>;
  primaryIdentityRows: Accessor<ReturnType<typeof getPrimaryResourceIdentityRows>>;
  identityCardHasRichData: Accessor<boolean>;
  aliasPreviewValues: Accessor<string[]>;
  hasAliasOverflow: Accessor<boolean>;
  hasIdentitySupportContext: Accessor<boolean>;
  hasMergedSources: Accessor<boolean>;
  discoveryConfig: Accessor<ReturnType<typeof toDiscoveryConfig>>;
  discoveryContextSummary: Accessor<string | null>;
  hasHostDetails: Accessor<boolean>;
  hostDetailSummary: Accessor<string | null>;
  hasServiceDetails: Accessor<boolean>;
  serviceDetailsSummary: Accessor<string | null>;
  headerIdentity: Accessor<string>;
  relatedLinks: Accessor<
    Array<{ href: string; label: string; compactLabel: string; ariaLabel: string }>
  >;
  hasRuntimeOperationalContext: Accessor<boolean>;
  sourceSections: Accessor<Array<{ id: string; label: string; payload: unknown }>>;
  sourceStatus: Accessor<NonNullable<PlatformData['sourceStatus']>>;
  identityMatchInfo: Accessor<unknown>;
  debugJson: Accessor<string>;
  tabs: Accessor<Array<{ id: DrawerTab; label: string }>>;
  handleCopyJson: () => Promise<void>;
}

export const useResourceDetailDrawerState = (
  options: UseResourceDetailDrawerStateOptions,
): UseResourceDetailDrawerStateResult => {
  const { resource, resolveResourceLabel: resolveResourceLabelInput } = options;
  const [activeTab, setActiveTab] = createSignal<DrawerTab>('overview');
  const [debugEnabled] = createLocalStorageBooleanSignal(STORAGE_KEYS.DEBUG_MODE, false);
  const [copied, setCopied] = createSignal(false);
  const [showReportModal, setShowReportModal] = createSignal(false);
  const [showInvestigationContext, setShowInvestigationContext] = createSignal(false);
  const [showCorrelationContext, setShowCorrelationContext] = createSignal(false);
  const [showDiscoveryContext, setShowDiscoveryContext] = createSignal(false);
  const [showHostDetails, setShowHostDetails] = createSignal(false);
  const [showServiceDetails, setShowServiceDetails] = createSignal(false);
  const [showDockerUpdateControls, setShowDockerUpdateControls] = createSignal(false);
  const [showPbsJobDetail, setShowPbsJobDetail] = createSignal(false);
  const [showPmgMailFlowDetail, setShowPmgMailFlowDetail] = createSignal(false);

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
    () => policyBadges().length > 0 || Boolean(governanceSummary()),
  );

  const agentMeta = createMemo(
    () => resource.agent ?? (platformData()?.agent as AgentPlatformData | undefined),
  );
  const kubernetesMeta = createMemo(
    () => resource.kubernetes ?? (platformData()?.kubernetes as KubernetesPlatformData | undefined),
  );
  const kubernetesCapabilityBadges = createMemo(() => {
    const capabilities = kubernetesMeta()?.metricCapabilities;
    if (!capabilities) return [];

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

  const [dockerActionError, setDockerActionError] = createSignal('');
  const [dockerActionNote, setDockerActionNote] = createSignal('');
  const [confirmUpdateAll, setConfirmUpdateAll] = createSignal(false);
  const [dockerActionBusy, setDockerActionBusy] = createSignal(false);
  const dockerSwarmInfo = createMemo(() => dockerHostData()?.swarm);
  const dockerSwarmClusterKey = createMemo(() => {
    const swarm = dockerSwarmInfo();
    return (swarm?.clusterName || swarm?.clusterId || '').trim();
  });

  const [k8sDeploymentsPrefillNamespace, setK8sDeploymentsPrefillNamespace] = createSignal('');
  const resourceFacetId = createMemo(() => resource.id.trim());
  const [timelineKindFilter, setTimelineKindFilter] = createSignal<ResourceChangeKind | ''>('');
  const [timelineSourceTypeFilter, setTimelineSourceTypeFilter] = createSignal<
    ResourceChangeSourceType | ''
  >('');
  const [timelineSourceAdapterFilter, setTimelineSourceAdapterFilter] = createSignal<
    ResourceChangeSourceAdapter | ''
  >('');
  const resourceFacetRequest = createMemo(() => {
    const id = resourceFacetId();
    return id ? { id } : null;
  });
  const [resourceFacets, { refetch: refetchResourceFacets }] = createResource(
    resourceFacetRequest,
    async (request) => {
      if (!request?.id) return null;
      return ResourceAPI.getFacetBundle(request.id, { limit: 25 });
    },
    { initialValue: null },
  );
  const [resourceIntelligence] = createResource(
    resourceFacetRequest,
    async (request) => {
      if (!request?.id) return null;
      return AIAPI.getResourceIntelligence(request.id);
    },
    { initialValue: null as ResourceIntelligence | null },
  );
  const resourceDependencies = createMemo(() => resourceIntelligence()?.dependencies ?? []);
  const resourceDependents = createMemo(() => resourceIntelligence()?.dependents ?? []);
  const resourceCorrelations = createMemo(() => resourceIntelligence()?.correlations ?? []);
  const hasCorrelationContext = createMemo(
    () =>
      resourceDependencies().length > 0 ||
      resourceDependents().length > 0 ||
      resourceCorrelations().length > 0,
  );
  const hasInvestigationContext = createMemo(
    () => Boolean(resourceIntelligence()) || hasGovernanceData(),
  );
  const investigationContextSummary = createMemo(() => {
    const intel = resourceIntelligence();
    const summary: string[] = [];

    if (intel) {
      summary.push(`AI health ${intel.health.grade} · ${Math.round(intel.health.score)}/100`);
    }
    if (resourceCorrelations().length > 0) {
      summary.push(
        `${resourceCorrelations().length} correlation${resourceCorrelations().length === 1 ? '' : 's'}`,
      );
    }
    if (resource.policy?.routing.scope) {
      summary.push(`Routing ${getResourceRoutingScopeLabel(resource.policy.routing.scope)}`);
    }

    return summary.join(' · ');
  });
  const timelineFacetRequest = createMemo(() => {
    const id = resourceFacetId();
    if (!id) return null;
    const kind = timelineKindFilter();
    const sourceType = timelineSourceTypeFilter();
    const sourceAdapter = timelineSourceAdapterFilter();
    if (!kind && !sourceType && !sourceAdapter) return null;
    return { id, kind, sourceType, sourceAdapter };
  });
  const [timelineFacets, { refetch: refetchTimelineFacets }] = createResource(
    timelineFacetRequest,
    async (request) => {
      if (!request) return null;
      return ResourceAPI.getFacetBundle(request.id, {
        limit: 25,
        kind: request.kind || undefined,
        sourceType: request.sourceType || undefined,
        sourceAdapter: request.sourceAdapter || undefined,
      });
    },
    { initialValue: null },
  );

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
  const resourceTimeline = createMemo(
    () => resourceFacets()?.recentChanges ?? resource.recentChanges ?? [],
  );
  const resourceFacetCounts = createMemo(
    () => resourceFacets()?.counts ?? resource.facetCounts ?? null,
  );
  const historyFacetBundle = createMemo(() =>
    timelineFacetRequest() ? (timelineFacets() ?? resourceFacets()) : resourceFacets(),
  );
  const historyFacetCounts = createMemo(
    () => historyFacetBundle()?.counts ?? resourceFacetCounts() ?? null,
  );
  const historyRecentChanges = createMemo(
    () => historyFacetBundle()?.recentChanges ?? resourceTimeline(),
  );
  const historyTimeline = createMemo(() => historyRecentChanges());
  const hasTimelineFilters = createMemo(() =>
    Boolean(timelineKindFilter() || timelineSourceTypeFilter() || timelineSourceAdapterFilter()),
  );
  const historyLoadingLabel = createMemo(() => {
    if (timelineFacetRequest()) {
      return timelineFacets.loading ? 'Refreshing filtered changes...' : 'Filtered changes loaded';
    }
    return resourceFacets.loading ? 'Refreshing changes...' : 'Changes loaded';
  });
  const resourceTimelineCount = createMemo(
    () => historyFacetCounts()?.recentChanges ?? historyRecentChanges().length,
  );
  const sortedResourceTimeline = createMemo(() =>
    [...historyTimeline()].sort((left, right) => {
      const leftTime = Date.parse(left.observedAt || '');
      const rightTime = Date.parse(right.observedAt || '');
      return (
        (Number.isFinite(rightTime) ? rightTime : 0) - (Number.isFinite(leftTime) ? leftTime : 0)
      );
    }),
  );
  const facetBundleError = createMemo(() => {
    const error = timelineFacetRequest() ? timelineFacets.error : resourceFacets.error;
    if (!error) return '';
    return (error as Error)?.message || 'Failed to load resource history';
  });
  const refetchHistoryFacets = () => {
    if (timelineFacetRequest()) {
      return refetchTimelineFacets();
    }
    return refetchResourceFacets();
  };
  const mergedSources = createMemo(() => platformData()?.sources ?? []);
  const sourceStatus = createMemo(() => platformData()?.sourceStatus ?? {});
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
  const primaryIdentityRows = createMemo(() => getPrimaryResourceIdentityRows(resource));
  const identityCardHasRichData = createMemo(
    () =>
      primaryIdentityRows().length > 0 ||
      (resource.identity?.ips?.length || 0) > 0 ||
      (resource.tags?.length || 0) > 0 ||
      identityAliasValues().length > 0,
  );
  const aliasPreviewValues = createMemo(() =>
    identityAliasValues().slice(0, ALIAS_COLLAPSE_THRESHOLD),
  );
  const hasAliasOverflow = createMemo(
    () => identityAliasValues().length > ALIAS_COLLAPSE_THRESHOLD,
  );
  const hasIdentitySupportContext = createMemo(
    () =>
      (resource.identity?.ips?.length ?? 0) > 0 ||
      (resource.tags?.length ?? 0) > 0 ||
      identityAliasValues().length > 0,
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
    if (!data) return [];
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

  createEffect(() => {
    if (!debugEnabled() && activeTab() === 'debug') {
      setActiveTab('overview');
    }
  });

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

  createEffect(() => {
    const current = activeTab();
    const available = new Set(tabs().map((tab) => tab.id));
    if (!available.has(current)) {
      setActiveTab('overview');
    }
  });

  const handleCopyJson = async () => {
    const payload = debugJson();
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(payload);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = payload;
        textarea.setAttribute('readonly', 'true');
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return {
    activeTab,
    setActiveTab,
    debugEnabled,
    copied,
    showReportModal,
    setShowReportModal,
    showInvestigationContext,
    setShowInvestigationContext,
    showCorrelationContext,
    setShowCorrelationContext,
    showDiscoveryContext,
    setShowDiscoveryContext,
    showHostDetails,
    setShowHostDetails,
    showServiceDetails,
    setShowServiceDetails,
    showDockerUpdateControls,
    setShowDockerUpdateControls,
    showPbsJobDetail,
    setShowPbsJobDetail,
    showPmgMailFlowDetail,
    setShowPmgMailFlowDetail,
    displayName,
    kubernetesClusterName,
    resolveResourceLabel,
    statusIndicator,
    lastSeen,
    lastSeenAbsolute,
    platformBadge,
    sourceBadge,
    typeBadge,
    unifiedSourceBadges,
    hasUnifiedSources,
    policyBadges,
    policyRedactions,
    governanceSummary,
    hasGovernanceData,
    platformData,
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
    dockerActionError,
    setDockerActionError,
    dockerActionNote,
    setDockerActionNote,
    confirmUpdateAll,
    setConfirmUpdateAll,
    dockerActionBusy,
    setDockerActionBusy,
    dockerSwarmInfo,
    dockerSwarmClusterKey,
    k8sDeploymentsPrefillNamespace,
    setK8sDeploymentsPrefillNamespace,
    timelineKindFilter,
    setTimelineKindFilter,
    timelineSourceTypeFilter,
    setTimelineSourceTypeFilter,
    timelineSourceAdapterFilter,
    setTimelineSourceAdapterFilter,
    resourceIntelligence,
    resourceDependencies,
    resourceDependents,
    resourceCorrelations,
    hasCorrelationContext,
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
    resourceTimeline,
    historyFacetCounts,
    historyRecentChanges,
    hasTimelineFilters,
    historyLoadingLabel,
    resourceTimelineCount,
    sortedResourceTimeline,
    facetBundleError,
    refetchHistoryFacets,
    mergedSources,
    sourceSummary,
    identityAliasValues,
    primaryIdentityRows,
    identityCardHasRichData,
    aliasPreviewValues,
    hasAliasOverflow,
    hasIdentitySupportContext,
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
    sourceStatus,
    identityMatchInfo,
    debugJson,
    tabs,
    handleCopyJson,
  };
};
