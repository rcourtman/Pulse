import {
  normalizeSourcePlatformKey,
  normalizeSourcePlatformQueryValue,
  resolvePlatformTypeFromSources,
} from '@/utils/sourcePlatforms';
import { normalizeStorageSourceKey } from '@/utils/storageSources';
import { normalizeRecoveryItemTypeQueryValue } from '@/utils/recoveryItemTypePresentation';
import {
  canonicalizeWorkloadFilterType,
  resolveWorkloadType,
} from '@/utils/workloads';
import type { WorkloadGuest } from '@/types/workloads';
import type { Resource } from '@/types/resource';
import {
  getActionableDockerRuntimeIdFromResource,
  getPlatformDataRecord,
  hasDockerWorkloadsScope,
  isTrueNASSystemResource,
} from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredResourceKubernetesContext,
  getPreferredWorkloadsAgentHint,
} from '@/utils/resourceIdentity';
import { requiresGovernedResourceDisplay } from '@/types/resource';

export const WORKLOADS_QUERY_PARAMS = {
  type: 'type',
  platform: 'platform',
  runtime: 'runtime',
  context: 'context',
  namespace: 'namespace',
  // Canonical v6 agent filter query param.
  agent: 'agent',
  resource: 'resource',
  summaryGroup: 'summaryGroup',
} as const;

export const WORKLOADS_PATH = '/workloads';
export const PMG_THRESHOLDS_PATH = '/alerts/thresholds/mail-gateway';
export const DASHBOARD_PATH = '/dashboard';
export const ALERTS_OVERVIEW_PATH = '/alerts/overview';
export const PATROL_PATH = '/patrol';
// Deprecated compatibility alias while older callers migrate off `/ai`.
export const AI_PATROL_PATH = PATROL_PATH;
// Canonical "Recovery" surface (was historically called Backups).
export const RECOVERY_PATH = '/recovery';

export const INFRASTRUCTURE_QUERY_PARAMS = {
  source: 'source',
  query: 'q',
  resource: 'resource',
  summaryGroup: 'summaryGroup',
} as const;

export const INFRASTRUCTURE_PATH = '/infrastructure';

export const STORAGE_QUERY_PARAMS = {
  tab: 'tab',
  group: 'group',
  source: 'source',
  status: 'status',
  node: 'node',
  query: 'q',
  resource: 'resource',
  sort: 'sort',
  order: 'order',
  summaryGroup: 'summaryGroup',
} as const;

export const RECOVERY_QUERY_PARAMS = {
  rollupId: 'rollupId',
  view: 'view',
  platform: 'platform',
  stale: 'stale',
  range: 'range',
  cluster: 'cluster',
  day: 'day',
  namespace: 'namespace',
  mode: 'mode',
  itemType: 'itemType',
  scope: 'scope',
  status: 'status',
  verification: 'verification',
  node: 'node',
  query: 'q',
} as const;

const normalizeQueryValue = (value: string | null | undefined): string => (value || '').trim();
const normalizeQueryBooleanFlag = (value: string | null | undefined): string => {
  const normalized = normalizeQueryValue(value).toLowerCase();
  return normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on'
    ? '1'
    : '';
};

const normalizeWorkloadsType = (value: string | null | undefined): string =>
  canonicalizeWorkloadFilterType(normalizeQueryValue(value));

const firstNonEmpty = (values: Array<string | undefined | null>): string | undefined => {
  for (const value of values) {
    if (typeof value !== 'string') continue;
    const trimmed = value.trim();
    if (trimmed.length > 0) return trimmed;
  }
  return undefined;
};

const dedupeResourceSurfaceLinks = (links: ResourceSurfaceLink[]): ResourceSurfaceLink[] => {
  const seen = new Set<string>();
  return links.filter((link) => {
    if (seen.has(link.href)) return false;
    seen.add(link.href);
    return true;
  });
};

type WorkloadsLinkOptions = {
  type?: string | null;
  platform?: string | null;
  runtime?: string | null;
  context?: string | null;
  namespace?: string | null;
  agent?: string | null;
  resource?: string | null;
  summaryGroup?: string | null;
};

type InfrastructureLinkOptions = {
  source?: string | null;
  query?: string | null;
  resource?: string | null;
  summaryGroup?: string | null;
};

type StorageLinkOptions = {
  tab?: string | null;
  group?: string | null;
  source?: string | null;
  status?: string | null;
  node?: string | null;
  query?: string | null;
  resource?: string | null;
  sort?: string | null;
  order?: string | null;
  summaryGroup?: string | null;
};

type RecoveryLinkOptions = {
  rollupId?: string | null;
  view?: string | null;
  platform?: string | null;
  stale?: string | null;
  range?: string | null;
  cluster?: string | null;
  day?: string | null;
  namespace?: string | null;
  mode?: string | null;
  itemType?: string | null;
  scope?: string | null;
  status?: string | null;
  verification?: string | null;
  node?: string | null;
  query?: string | null;
};

export type ResourceSurfaceLink = {
  href: string;
  label: string;
  compactLabel: string;
  ariaLabel: string;
};

type ResolvedResourceSurfaceLinkOptions = {
  resourceId: string;
  displayName: string;
  resource?: Resource | null;
  allowInfrastructureFallback?: boolean;
};

const RECOVERY_LEGACY_PLATFORM_QUERY_PARAM = 'provider';

export const parseWorkloadsLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  return {
    type: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.type)),
    platform: normalizeSourcePlatformQueryValue(params.get(WORKLOADS_QUERY_PARAMS.platform)),
    runtime: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.runtime)),
    context: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.context)),
    namespace: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.namespace)),
    agent: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.agent)),
    resource: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.resource)),
    summaryGroup: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.summaryGroup)),
  };
};

export const buildWorkloadsPath = (options: WorkloadsLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const type = normalizeWorkloadsType(options.type);
  const platform = normalizeSourcePlatformQueryValue(options.platform);
  const runtime = normalizeQueryValue(options.runtime);
  const context = normalizeQueryValue(options.context);
  const namespace = normalizeQueryValue(options.namespace);
  const agent = normalizeQueryValue(options.agent);
  const resource = normalizeQueryValue(options.resource);
  const summaryGroup = normalizeQueryValue(options.summaryGroup);
  if (type) params.set(WORKLOADS_QUERY_PARAMS.type, type);
  if (platform) params.set(WORKLOADS_QUERY_PARAMS.platform, platform);
  if (runtime) params.set(WORKLOADS_QUERY_PARAMS.runtime, runtime);
  if (context) params.set(WORKLOADS_QUERY_PARAMS.context, context);
  if (namespace) params.set(WORKLOADS_QUERY_PARAMS.namespace, namespace);
  if (agent) params.set(WORKLOADS_QUERY_PARAMS.agent, agent);
  if (resource) params.set(WORKLOADS_QUERY_PARAMS.resource, resource);
  if (summaryGroup) params.set(WORKLOADS_QUERY_PARAMS.summaryGroup, summaryGroup);
  const query = params.toString();
  return query ? `${WORKLOADS_PATH}?${query}` : WORKLOADS_PATH;
};

export const parseInfrastructureLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  return {
    source: normalizeSourcePlatformQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.source)),
    query: normalizeQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.query)),
    resource: normalizeQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.resource)),
    summaryGroup: normalizeQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.summaryGroup)),
  };
};

export const buildInfrastructurePath = (options: InfrastructureLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const source = normalizeSourcePlatformQueryValue(options.source);
  const query = normalizeQueryValue(options.query);
  const resource = normalizeQueryValue(options.resource);
  const summaryGroup = normalizeQueryValue(options.summaryGroup);
  if (source) params.set(INFRASTRUCTURE_QUERY_PARAMS.source, source);
  if (query) params.set(INFRASTRUCTURE_QUERY_PARAMS.query, query);
  if (resource) params.set(INFRASTRUCTURE_QUERY_PARAMS.resource, resource);
  if (summaryGroup) params.set(INFRASTRUCTURE_QUERY_PARAMS.summaryGroup, summaryGroup);
  const serialized = params.toString();
  return serialized ? `${INFRASTRUCTURE_PATH}?${serialized}` : INFRASTRUCTURE_PATH;
};

export const buildInfrastructureResourceHref = (resourceId: string): string | null => {
  const trimmed = resourceId.trim();
  return trimmed ? buildInfrastructurePath({ resource: trimmed }) : null;
};

export const buildInfrastructureResourceLink = (
  resourceId: string,
  displayName: string,
): ResourceSurfaceLink | null => {
  const href = buildInfrastructureResourceHref(resourceId);
  if (!href) return null;
  return {
    href,
    label: 'Open in Infrastructure',
    compactLabel: 'Infrastructure',
    ariaLabel: `Open related infrastructure for ${displayName}`,
  };
};

export const buildInfrastructureHrefForWorkload = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);

  if (type === 'vm' || type === 'system-container') {
    const query = firstNonEmpty([guest.node, guest.instance, guest.name]);
    return buildInfrastructurePath({ source: 'proxmox-pve', query });
  }

  if (type === 'app-container') {
    const query = firstNonEmpty([guest.contextLabel, guest.node, guest.instance, guest.name]);
    return buildInfrastructurePath({
      source: guest.platformType === 'truenas' ? 'truenas' : 'docker',
      query,
    });
  }

  if (type === 'pod') {
    const query = firstNonEmpty([
      guest.contextLabel,
      guest.instance,
      guest.namespace,
      guest.node,
      guest.name,
    ]);
    return buildInfrastructurePath({ source: 'kubernetes', query });
  }

  return buildInfrastructurePath();
};

const resolveKubernetesContextForResource = (resource: Resource): string | undefined => {
  const kubernetesContext = getPreferredResourceKubernetesContext(resource);
  if (resource.type === 'k8s-cluster') {
    const displayLabel = requiresGovernedResourceDisplay(resource.policy)
      ? getPreferredInfrastructureDisplayName(resource)
      : resource.displayName?.trim() || resource.name?.trim() || undefined;
    return firstNonEmpty([kubernetesContext, displayLabel]);
  }
  if (resource.type === 'k8s-node') {
    return kubernetesContext;
  }
  if (resource.type === 'pod') {
    return kubernetesContext;
  }
  return undefined;
};

const resolveHostHintForResource = (resource: Resource): string | undefined =>
  getPreferredWorkloadsAgentHint(resource);

const resolveDockerWorkloadsHintForResource = (resource: Resource): string | undefined =>
  getActionableDockerRuntimeIdFromResource(resource) || resolveHostHintForResource(resource);

const hasMergedSource = (resource: Resource, source: string): boolean => {
  const platformData = getPlatformDataRecord(resource);
  const mergedSources = Array.isArray(platformData?.sources) ? platformData.sources : [];
  return mergedSources.some((value) => normalizeSourcePlatformKey(value) === source);
};

export const buildWorkloadsHrefForResource = (resource: Resource): string | null => {
  if (resource.type === 'k8s-cluster' || resource.type === 'k8s-node') {
    const context = resolveKubernetesContextForResource(resource);
    return buildWorkloadsPath({ type: 'pod', platform: 'kubernetes', context });
  }

  if (resource.type === 'docker-host') {
    const agent = resolveDockerWorkloadsHintForResource(resource);
    return buildWorkloadsPath({ type: 'app-container', platform: 'docker', agent });
  }

  if (resource.type === 'agent') {
    const agent = resolveHostHintForResource(resource);
    if (resource.platformType === 'truenas' || hasMergedSource(resource, 'truenas')) {
      return buildWorkloadsPath({ type: 'app-container', platform: 'truenas', agent });
    }
    if (hasDockerWorkloadsScope(resource)) {
      return buildWorkloadsPath({
        type: 'app-container',
        platform: 'docker',
        agent: resolveDockerWorkloadsHintForResource(resource),
      });
    }
    return buildWorkloadsPath({ agent });
  }

  if (
    resource.type === 'vm' ||
    resource.type === 'system-container' ||
    resource.type === 'oci-container' ||
    resource.type === 'jail'
  ) {
    return buildWorkloadsPath({
      type: resource.type,
      platform: resource.platformType,
      agent: firstNonEmpty([resource.parentId, resource.platformId]),
      resource: resource.id,
    });
  }

  if (resource.type === 'app-container') {
    const isTrueNAS = resource.platformType === 'truenas' || hasMergedSource(resource, 'truenas');
    const isDocker = resource.platformType === 'docker' || hasMergedSource(resource, 'docker');
    return buildWorkloadsPath({
      type: 'app-container',
      platform: isTrueNAS ? 'truenas' : isDocker ? 'docker' : resource.platformType,
      agent: isDocker
        ? firstNonEmpty([
            getActionableDockerRuntimeIdFromResource(resource),
            resource.parentId,
            resource.platformId,
          ])
        : firstNonEmpty([resource.parentId, resource.platformId]),
      resource: resource.id,
    });
  }

  if (resource.type === 'pod') {
    return buildWorkloadsPath({
      type: 'pod',
      platform: 'kubernetes',
      context: resolveKubernetesContextForResource(resource),
      namespace: firstNonEmpty([resource.kubernetes?.namespace, resource.labels?.namespace]),
      resource: resource.id,
    });
  }

  return null;
};

export const buildResourceSurfaceLinksForResource = (
  resource: Resource,
  displayName: string,
): ResourceSurfaceLink[] => {
  const links: ResourceSurfaceLink[] = [];
  const workloads = buildWorkloadsHrefForResource(resource);
  const workloadSearch = workloads ? new URLSearchParams(workloads.split('?')[1] ?? '') : null;
  const scopedWorkloadType = workloadSearch?.get('type')?.trim() ?? '';

  if (workloads && scopedWorkloadType) {
    links.push({
      href: workloads,
      label: 'Open in Workloads',
      compactLabel: 'Workloads',
      ariaLabel: `Open related workloads for ${displayName}`,
    });
  }

  const storage = buildStorageHrefForResource(resource);
  if (storage) {
    links.push({
      href: storage,
      label: 'Open in Storage',
      compactLabel: 'Storage',
      ariaLabel: `Open related storage for ${displayName}`,
    });
  }

  const recovery = buildRecoveryHrefForResource(resource);
  if (recovery) {
    links.push({
      href: recovery,
      label: 'Open in Recovery',
      compactLabel: 'Recovery',
      ariaLabel: `Open related recovery for ${displayName}`,
    });
  }

  return dedupeResourceSurfaceLinks(links);
};

const resolveStorageRouteSource = (resource: Resource): string => {
  const mergedPlatform = Array.isArray(resource.platformData?.sources)
    ? resolvePlatformTypeFromSources(resource.platformData.sources)
    : undefined;

  return normalizeStorageSourceKey(
    resource.storage?.platform || mergedPlatform || resource.platformType || resource.type,
  );
};

export const buildStorageHrefForResource = (resource: Resource): string | null => {
  const source = resolveStorageRouteSource(resource);
  if (!source || source === 'all') return null;

  if (
    resource.type === 'storage' ||
    resource.type === 'datastore' ||
    resource.type === 'pool' ||
    resource.type === 'dataset' ||
    resource.type === 'ceph'
  ) {
    return buildStoragePath({
      source,
      resource: resource.id,
    });
  }

  if (resource.type === 'physical_disk') {
    return buildStoragePath({
      tab: 'disks',
      source,
      node: firstNonEmpty([resource.parentId, resource.platformId]),
      resource: resource.id,
    });
  }

  if (resource.type === 'pbs') {
    return buildStoragePath({
      source,
      node: resource.id,
    });
  }

  if (isTrueNASSystemResource(resource)) {
    return buildStoragePath({
      source,
      node: resource.id,
    });
  }

  return null;
};

export const buildResolvedResourceSurfaceLinks = ({
  resourceId,
  displayName,
  resource,
  allowInfrastructureFallback = false,
}: ResolvedResourceSurfaceLinkOptions): ResourceSurfaceLink[] => {
  const links: ResourceSurfaceLink[] = [];

  if (resource) {
    const infrastructure = buildInfrastructureResourceLink(resource.id, displayName);
    if (infrastructure) {
      links.push(infrastructure);
    }
    links.push(...buildResourceSurfaceLinksForResource(resource, displayName));
    return dedupeResourceSurfaceLinks(links);
  }

  if (allowInfrastructureFallback) {
    const infrastructure = buildInfrastructureResourceLink(resourceId, displayName);
    if (infrastructure) {
      links.push(infrastructure);
    }
  }

  return dedupeResourceSurfaceLinks(links);
};

export const buildRecoveryHrefForResource = (resource: Resource): string | null => {
  const mergedPlatform = Array.isArray(resource.platformData?.sources)
    ? resolvePlatformTypeFromSources(resource.platformData.sources)
    : undefined;
  const platform = normalizeSourcePlatformQueryValue(
    resource.storage?.platform || mergedPlatform || resource.platformType || resource.type,
  );

  if (platform === 'truenas' && isTrueNASSystemResource(resource)) {
    return buildRecoveryPath({
      platform: 'truenas',
      node: resource.id,
    });
  }

  return null;
};

export const parseStorageLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  return {
    tab: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.tab)),
    group: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.group)),
    source: normalizeStorageSourceKey(params.get(STORAGE_QUERY_PARAMS.source)),
    status: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.status)),
    node: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.node)),
    query: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.query)),
    resource: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.resource)),
    sort: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.sort)),
    order: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.order)),
    summaryGroup: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.summaryGroup)),
  };
};

export const buildStoragePath = (options: StorageLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const tab = normalizeQueryValue(options.tab);
  const group = normalizeQueryValue(options.group);
  const source = normalizeStorageSourceKey(options.source);
  const status = normalizeQueryValue(options.status);
  const node = normalizeQueryValue(options.node);
  const query = normalizeQueryValue(options.query);
  const resource = normalizeQueryValue(options.resource);
  const sort = normalizeQueryValue(options.sort);
  const order = normalizeQueryValue(options.order);
  const summaryGroup = normalizeQueryValue(options.summaryGroup);

  if (tab) params.set(STORAGE_QUERY_PARAMS.tab, tab);
  if (group) params.set(STORAGE_QUERY_PARAMS.group, group);
  if (source) params.set(STORAGE_QUERY_PARAMS.source, source);
  if (status) params.set(STORAGE_QUERY_PARAMS.status, status);
  if (node) params.set(STORAGE_QUERY_PARAMS.node, node);
  if (query) params.set(STORAGE_QUERY_PARAMS.query, query);
  if (resource) params.set(STORAGE_QUERY_PARAMS.resource, resource);
  if (sort) params.set(STORAGE_QUERY_PARAMS.sort, sort);
  if (order) params.set(STORAGE_QUERY_PARAMS.order, order);
  if (summaryGroup) params.set(STORAGE_QUERY_PARAMS.summaryGroup, summaryGroup);

  const serialized = params.toString();
  return serialized ? `/storage?${serialized}` : '/storage';
};

export const parseRecoveryLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);

  return {
    rollupId: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.rollupId)),
    view: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.view)),
    platform: normalizeSourcePlatformQueryValue(
      firstNonEmpty([
        params.get(RECOVERY_QUERY_PARAMS.platform),
        params.get(RECOVERY_LEGACY_PLATFORM_QUERY_PARAM),
      ]),
    ),
    stale: normalizeQueryBooleanFlag(params.get(RECOVERY_QUERY_PARAMS.stale)),
    range: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.range)),
    cluster: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.cluster)),
    day: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.day)),
    namespace: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.namespace)),
    mode: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.mode)),
    itemType: normalizeRecoveryItemTypeQueryValue(params.get(RECOVERY_QUERY_PARAMS.itemType)),
    scope: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.scope)),
    status: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.status)),
    verification: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.verification)),
    node: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.node)),
    query: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.query)),
  };
};

export const buildRecoveryPath = (options: RecoveryLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const rollupId = normalizeQueryValue(options.rollupId);
  const view = normalizeQueryValue(options.view);
  const platform = normalizeSourcePlatformQueryValue(options.platform);
  const stale = normalizeQueryBooleanFlag(options.stale);
  const range = normalizeQueryValue(options.range);
  const cluster = normalizeQueryValue(options.cluster);
  const day = normalizeQueryValue(options.day);
  const namespace = normalizeQueryValue(options.namespace);
  const mode = normalizeQueryValue(options.mode);
  const itemType = normalizeRecoveryItemTypeQueryValue(options.itemType);
  const scope = normalizeQueryValue(options.scope);
  const status = normalizeQueryValue(options.status);
  const verification = normalizeQueryValue(options.verification);
  const node = normalizeQueryValue(options.node);
  const query = normalizeQueryValue(options.query);

  if (rollupId) params.set(RECOVERY_QUERY_PARAMS.rollupId, rollupId);
  if (view) params.set(RECOVERY_QUERY_PARAMS.view, view);
  if (platform) params.set(RECOVERY_QUERY_PARAMS.platform, platform);
  if (stale) params.set(RECOVERY_QUERY_PARAMS.stale, stale);
  if (range) params.set(RECOVERY_QUERY_PARAMS.range, range);
  if (cluster) params.set(RECOVERY_QUERY_PARAMS.cluster, cluster);
  if (day) params.set(RECOVERY_QUERY_PARAMS.day, day);
  if (namespace) params.set(RECOVERY_QUERY_PARAMS.namespace, namespace);
  if (mode) params.set(RECOVERY_QUERY_PARAMS.mode, mode);
  if (itemType) params.set(RECOVERY_QUERY_PARAMS.itemType, itemType);
  if (scope) params.set(RECOVERY_QUERY_PARAMS.scope, scope);
  if (status) params.set(RECOVERY_QUERY_PARAMS.status, status);
  if (verification) params.set(RECOVERY_QUERY_PARAMS.verification, verification);
  if (node) params.set(RECOVERY_QUERY_PARAMS.node, node);
  if (query) params.set(RECOVERY_QUERY_PARAMS.query, query);

  const serialized = params.toString();
  return serialized ? `${RECOVERY_PATH}?${serialized}` : RECOVERY_PATH;
};
