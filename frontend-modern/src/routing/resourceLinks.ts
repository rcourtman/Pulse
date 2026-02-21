export const WORKLOADS_QUERY_PARAMS = {
  type: 'type',
  runtime: 'runtime',
  context: 'context',
  namespace: 'namespace',
  host: 'host',
  resource: 'resource',
} as const;

export const WORKLOADS_PATH = '/workloads';
export const PMG_THRESHOLDS_PATH = '/alerts/thresholds/mail-gateway';
export const DASHBOARD_PATH = '/dashboard';
export const ALERTS_OVERVIEW_PATH = '/alerts/overview';
export const AI_PATROL_PATH = '/ai';
// Canonical "Recovery" surface (was historically called Backups).
export const RECOVERY_PATH = '/recovery';

export const INFRASTRUCTURE_QUERY_PARAMS = {
  source: 'source',
  query: 'q',
  legacyQuery: 'search',
  resource: 'resource',
} as const;

export const INFRASTRUCTURE_PATH = '/infrastructure';

export const STORAGE_QUERY_PARAMS = {
  tab: 'tab',
  group: 'group',
  source: 'source',
  status: 'status',
  node: 'node',
  query: 'q',
  legacyQuery: 'search',
  resource: 'resource',
  sort: 'sort',
  order: 'order',
} as const;

export const RECOVERY_QUERY_PARAMS = {
  view: 'view',
  rollupId: 'rollupId',
  provider: 'provider',
  cluster: 'cluster',
  namespace: 'namespace',
  mode: 'mode',
  scope: 'scope',
  status: 'status',
  verification: 'verification',
  node: 'node',
  query: 'q',
} as const;

const normalizeQueryValue = (value: string | null | undefined): string => (value || '').trim();

type WorkloadsLinkOptions = {
  type?: string | null;
  runtime?: string | null;
  context?: string | null;
  namespace?: string | null;
  host?: string | null;
  resource?: string | null;
};

type InfrastructureLinkOptions = {
  source?: string | null;
  query?: string | null;
  resource?: string | null;
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
};

type RecoveryLinkOptions = {
  view?: 'events' | 'protected' | null;
  rollupId?: string | null;
  provider?: string | null;
  cluster?: string | null;
  namespace?: string | null;
  mode?: string | null;
  scope?: string | null;
  status?: string | null;
  verification?: string | null;
  node?: string | null;
  query?: string | null;
};

export const parseWorkloadsLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  return {
    type: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.type)),
    runtime: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.runtime)),
    context: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.context)),
    namespace: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.namespace)),
    host: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.host)),
    resource: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.resource)),
  };
};

export const buildWorkloadsPath = (options: WorkloadsLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const type = normalizeQueryValue(options.type);
  const runtime = normalizeQueryValue(options.runtime);
  const context = normalizeQueryValue(options.context);
  const namespace = normalizeQueryValue(options.namespace);
  const host = normalizeQueryValue(options.host);
  const resource = normalizeQueryValue(options.resource);
  if (type) params.set(WORKLOADS_QUERY_PARAMS.type, type);
  if (runtime) params.set(WORKLOADS_QUERY_PARAMS.runtime, runtime);
  if (context) params.set(WORKLOADS_QUERY_PARAMS.context, context);
  if (namespace) params.set(WORKLOADS_QUERY_PARAMS.namespace, namespace);
  if (host) params.set(WORKLOADS_QUERY_PARAMS.host, host);
  if (resource) params.set(WORKLOADS_QUERY_PARAMS.resource, resource);
  const query = params.toString();
  return query ? `${WORKLOADS_PATH}?${query}` : WORKLOADS_PATH;
};

export const parseInfrastructureLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  const queryValue =
    normalizeQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.query)) ||
    normalizeQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.legacyQuery));
  return {
    source: normalizeQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.source)),
    query: queryValue,
    resource: normalizeQueryValue(params.get(INFRASTRUCTURE_QUERY_PARAMS.resource)),
  };
};

export const buildInfrastructurePath = (options: InfrastructureLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const source = normalizeQueryValue(options.source);
  const query = normalizeQueryValue(options.query);
  const resource = normalizeQueryValue(options.resource);
  if (source) params.set(INFRASTRUCTURE_QUERY_PARAMS.source, source);
  if (query) params.set(INFRASTRUCTURE_QUERY_PARAMS.query, query);
  if (resource) params.set(INFRASTRUCTURE_QUERY_PARAMS.resource, resource);
  const serialized = params.toString();
  return serialized ? `${INFRASTRUCTURE_PATH}?${serialized}` : INFRASTRUCTURE_PATH;
};

export const parseStorageLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  const queryValue =
    normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.query)) ||
    normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.legacyQuery));
  return {
    tab: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.tab)),
    group: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.group)),
    source: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.source)),
    status: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.status)),
    node: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.node)),
    query: queryValue,
    resource: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.resource)),
    sort: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.sort)),
    order: normalizeQueryValue(params.get(STORAGE_QUERY_PARAMS.order)),
  };
};

export const buildStoragePath = (options: StorageLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const tab = normalizeQueryValue(options.tab);
  const group = normalizeQueryValue(options.group);
  const source = normalizeQueryValue(options.source);
  const status = normalizeQueryValue(options.status);
  const node = normalizeQueryValue(options.node);
  const query = normalizeQueryValue(options.query);
  const resource = normalizeQueryValue(options.resource);
  const sort = normalizeQueryValue(options.sort);
  const order = normalizeQueryValue(options.order);

  if (tab) params.set(STORAGE_QUERY_PARAMS.tab, tab);
  if (group) params.set(STORAGE_QUERY_PARAMS.group, group);
  if (source) params.set(STORAGE_QUERY_PARAMS.source, source);
  if (status) params.set(STORAGE_QUERY_PARAMS.status, status);
  if (node) params.set(STORAGE_QUERY_PARAMS.node, node);
  if (query) params.set(STORAGE_QUERY_PARAMS.query, query);
  if (resource) params.set(STORAGE_QUERY_PARAMS.resource, resource);
  if (sort) params.set(STORAGE_QUERY_PARAMS.sort, sort);
  if (order) params.set(STORAGE_QUERY_PARAMS.order, order);

  const serialized = params.toString();
  return serialized ? `/storage?${serialized}` : '/storage';
};

export const parseRecoveryLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);

  return {
    view: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.view)),
    rollupId: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.rollupId)),
    provider: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.provider)),
    cluster: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.cluster)),
    namespace: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.namespace)),
    mode: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.mode)),
    scope: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.scope)),
    status: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.status)),
    verification: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.verification)),
    node: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.node)),
    query: normalizeQueryValue(params.get(RECOVERY_QUERY_PARAMS.query)),
  };
};

export const buildRecoveryPath = (options: RecoveryLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const view = normalizeQueryValue(options.view);
  const rollupId = normalizeQueryValue(options.rollupId);
  const provider = normalizeQueryValue(options.provider);
  const cluster = normalizeQueryValue(options.cluster);
  const namespace = normalizeQueryValue(options.namespace);
  const mode = normalizeQueryValue(options.mode);
  const scope = normalizeQueryValue(options.scope);
  const status = normalizeQueryValue(options.status);
  const verification = normalizeQueryValue(options.verification);
  const node = normalizeQueryValue(options.node);
  const query = normalizeQueryValue(options.query);

  if (view) params.set(RECOVERY_QUERY_PARAMS.view, view);
  if (rollupId) params.set(RECOVERY_QUERY_PARAMS.rollupId, rollupId);
  if (provider) params.set(RECOVERY_QUERY_PARAMS.provider, provider);
  if (cluster) params.set(RECOVERY_QUERY_PARAMS.cluster, cluster);
  if (namespace) params.set(RECOVERY_QUERY_PARAMS.namespace, namespace);
  if (mode) params.set(RECOVERY_QUERY_PARAMS.mode, mode);
  if (scope) params.set(RECOVERY_QUERY_PARAMS.scope, scope);
  if (status) params.set(RECOVERY_QUERY_PARAMS.status, status);
  if (verification) params.set(RECOVERY_QUERY_PARAMS.verification, verification);
  if (node) params.set(RECOVERY_QUERY_PARAMS.node, node);
  if (query) params.set(RECOVERY_QUERY_PARAMS.query, query);

  const serialized = params.toString();
  return serialized ? `${RECOVERY_PATH}?${serialized}` : RECOVERY_PATH;
};
