import { normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import { normalizeStorageSourceKey } from '@/utils/storageSources';
import { normalizeRecoveryItemTypeQueryValue } from '@/utils/recoveryItemTypePresentation';
import { canonicalizeWorkloadFilterType } from '@/utils/workloads';

export const WORKLOADS_QUERY_PARAMS = {
  type: 'type',
  platform: 'platform',
  runtime: 'runtime',
  context: 'context',
  namespace: 'namespace',
  cluster: 'cluster',
  // Canonical v6 agent filter query param.
  agent: 'agent',
  resource: 'resource',
  summaryGroup: 'summaryGroup',
} as const;

export const STANDALONE_PATH = '/standalone';
export const STANDALONE_DEFAULT_TAB = 'machines';
export const PROXMOX_PATH = '/proxmox';
export const PROXMOX_DEFAULT_TAB = 'overview';
export const DOCKER_PATH = '/docker';
export const DOCKER_DEFAULT_TAB = 'overview';
export const KUBERNETES_PATH = '/kubernetes';
export const KUBERNETES_DEFAULT_TAB = 'overview';
export const TRUENAS_PATH = '/truenas';
export const TRUENAS_DEFAULT_TAB = 'overview';
export const VMWARE_PATH = '/vmware';
export const VMWARE_DEFAULT_TAB = 'overview';
export const PMG_THRESHOLDS_PATH = '/alerts/thresholds/mail-gateway';
export const PATROL_PATH = '/patrol';

export const DOCKER_QUERY_PARAMS = {
  host: 'host',
} as const;

export const STORAGE_QUERY_PARAMS = {
  tab: 'tab',
  group: 'group',
  source: 'source',
  status: 'status',
  diskRole: 'diskRole',
  diskGroup: 'diskGroup',
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
  state: 'state',
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

type WorkloadsLinkOptions = {
  type?: string | null;
  platform?: string | null;
  runtime?: string | null;
  context?: string | null;
  namespace?: string | null;
  cluster?: string | null;
  agent?: string | null;
  resource?: string | null;
  summaryGroup?: string | null;
};

type DockerLinkOptions = {
  host?: string | null;
};

type StorageLinkOptions = {
  tab?: string | null;
  group?: string | null;
  source?: string | null;
  status?: string | null;
  diskRole?: string | null;
  diskGroup?: string | null;
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
  state?: string | null;
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

const RECOVERY_LEGACY_PLATFORM_QUERY_PARAM = 'provider';

export const parseWorkloadsLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  return {
    type: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.type)),
    platform: normalizeSourcePlatformQueryValue(params.get(WORKLOADS_QUERY_PARAMS.platform)),
    runtime: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.runtime)),
    context: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.context)),
    namespace: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.namespace)),
    cluster: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.cluster)),
    agent: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.agent)),
    resource: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.resource)),
    summaryGroup: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.summaryGroup)),
  };
};

const serializedRouteSearch = (params: URLSearchParams): string => {
  const query = params.toString();
  return query ? `?${query}` : '';
};

export const buildWorkloadsRouteSearch = (options: WorkloadsLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const type = normalizeWorkloadsType(options.type);
  const platform = normalizeSourcePlatformQueryValue(options.platform);
  const runtime = normalizeQueryValue(options.runtime);
  const context = normalizeQueryValue(options.context);
  const namespace = normalizeQueryValue(options.namespace);
  const cluster = normalizeQueryValue(options.cluster);
  const agent = normalizeQueryValue(options.agent);
  const resource = normalizeQueryValue(options.resource);
  const summaryGroup = normalizeQueryValue(options.summaryGroup);
  if (type) params.set(WORKLOADS_QUERY_PARAMS.type, type);
  if (platform) params.set(WORKLOADS_QUERY_PARAMS.platform, platform);
  if (runtime) params.set(WORKLOADS_QUERY_PARAMS.runtime, runtime);
  if (context) params.set(WORKLOADS_QUERY_PARAMS.context, context);
  if (namespace) params.set(WORKLOADS_QUERY_PARAMS.namespace, namespace);
  if (cluster) params.set(WORKLOADS_QUERY_PARAMS.cluster, cluster);
  if (agent) params.set(WORKLOADS_QUERY_PARAMS.agent, agent);
  if (resource) params.set(WORKLOADS_QUERY_PARAMS.resource, resource);
  if (summaryGroup) params.set(WORKLOADS_QUERY_PARAMS.summaryGroup, summaryGroup);
  return serializedRouteSearch(params);
};

export const buildProxmoxPath = (tab: string = PROXMOX_DEFAULT_TAB): string => {
  const normalized = tab.trim().replace(/^\/+|\/+$/g, '');
  return normalized ? `${PROXMOX_PATH}/${normalized}` : PROXMOX_PATH;
};

export const buildStandalonePath = (tab: string = STANDALONE_DEFAULT_TAB): string => {
  const normalized = tab.trim().replace(/^\/+|\/+$/g, '');
  return normalized ? `${STANDALONE_PATH}/${normalized}` : STANDALONE_PATH;
};

export const buildDockerPath = (tab: string = DOCKER_DEFAULT_TAB): string => {
  const normalized = tab.trim().replace(/^\/+|\/+$/g, '');
  return normalized ? `${DOCKER_PATH}/${normalized}` : DOCKER_PATH;
};

export const buildDockerRouteSearch = (options: DockerLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const host = normalizeQueryValue(options.host);
  if (host) params.set(DOCKER_QUERY_PARAMS.host, host);
  return serializedRouteSearch(params);
};

export const buildKubernetesPath = (tab: string = KUBERNETES_DEFAULT_TAB): string => {
  const normalized = tab.trim().replace(/^\/+|\/+$/g, '');
  return normalized ? `${KUBERNETES_PATH}/${normalized}` : KUBERNETES_PATH;
};

export const buildTrueNASPath = (tab: string = TRUENAS_DEFAULT_TAB): string => {
  const normalized = tab.trim().replace(/^\/+|\/+$/g, '');
  return normalized ? `${TRUENAS_PATH}/${normalized}` : TRUENAS_PATH;
};

export const buildVmwarePath = (tab: string = VMWARE_DEFAULT_TAB): string => {
  const normalized = tab.trim().replace(/^\/+|\/+$/g, '');
  return normalized ? `${VMWARE_PATH}/${normalized}` : VMWARE_PATH;
};

export const parseStorageLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  return {
    tab: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.tab)),
    group: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.group)),
    source: normalizeStorageSourceKey(params.get(STORAGE_QUERY_PARAMS.source)),
    status: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.status)),
    diskRole: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.diskRole)),
    diskGroup: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.diskGroup)),
    node: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.node)),
    query: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.query)),
    resource: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.resource)),
    sort: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.sort)),
    order: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.order)),
    summaryGroup: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.summaryGroup)),
  };
};

export const buildStorageRouteSearch = (options: StorageLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const tab = normalizeQueryValue(options.tab);
  const group = normalizeQueryValue(options.group);
  const source = normalizeStorageSourceKey(options.source);
  const status = normalizeQueryValue(options.status);
  const diskRole = normalizeQueryValue(options.diskRole);
  const diskGroup = normalizeQueryValue(options.diskGroup);
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
  if (diskRole) params.set(STORAGE_QUERY_PARAMS.diskRole, diskRole);
  if (diskGroup) params.set(STORAGE_QUERY_PARAMS.diskGroup, diskGroup);
  if (node) params.set(STORAGE_QUERY_PARAMS.node, node);
  if (query) params.set(STORAGE_QUERY_PARAMS.query, query);
  if (resource) params.set(STORAGE_QUERY_PARAMS.resource, resource);
  if (sort) params.set(STORAGE_QUERY_PARAMS.sort, sort);
  if (order) params.set(STORAGE_QUERY_PARAMS.order, order);
  if (summaryGroup) params.set(STORAGE_QUERY_PARAMS.summaryGroup, summaryGroup);

  return serializedRouteSearch(params);
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
    state: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.state)),
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

export const buildRecoveryRouteSearch = (options: RecoveryLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const rollupId = normalizeQueryValue(options.rollupId);
  const view = normalizeQueryValue(options.view);
  const platform = normalizeSourcePlatformQueryValue(options.platform);
  const state = normalizeQueryValue(options.state);
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
  if (state) params.set(RECOVERY_QUERY_PARAMS.state, state);
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

  return serializedRouteSearch(params);
};
