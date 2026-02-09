export const WORKLOADS_QUERY_PARAMS = {
  type: 'type',
  context: 'context',
  host: 'host',
  resource: 'resource',
} as const;

export const WORKLOADS_PATH = '/workloads';
export const STORAGE_V2_PATH = '/storage-v2';
export const BACKUPS_V2_PATH = '/backups-v2';
export const PMG_THRESHOLDS_PATH = '/alerts/thresholds/mail-gateway';
export const DASHBOARD_PATH = '/dashboard';
export const ALERTS_OVERVIEW_PATH = '/alerts/overview';
export const AI_PATROL_PATH = '/ai';

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

export const BACKUPS_QUERY_PARAMS = {
  guestType: 'type',
  source: 'source',
  namespace: 'namespace',
  backupType: 'backupType',
  status: 'status',
  group: 'group',
  node: 'node',
  query: 'q',
  legacyQuery: 'search',
} as const;

const normalizeQueryValue = (value: string | null | undefined): string => (value || '').trim();

type WorkloadsLinkOptions = {
  type?: string | null;
  context?: string | null;
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

type BackupsLinkOptions = {
  guestType?: string | null;
  source?: string | null;
  namespace?: string | null;
  backupType?: string | null;
  status?: string | null;
  group?: string | null;
  node?: string | null;
  query?: string | null;
};

export const parseWorkloadsLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  return {
    type: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.type)),
    context: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.context)),
    host: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.host)),
    resource: normalizeQueryValue(params.get(WORKLOADS_QUERY_PARAMS.resource)),
  };
};

export const buildWorkloadsPath = (options: WorkloadsLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const type = normalizeQueryValue(options.type);
  const context = normalizeQueryValue(options.context);
  const host = normalizeQueryValue(options.host);
  const resource = normalizeQueryValue(options.resource);
  if (type) params.set(WORKLOADS_QUERY_PARAMS.type, type);
  if (context) params.set(WORKLOADS_QUERY_PARAMS.context, context);
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

export const buildStorageV2Path = (options: StorageLinkOptions = {}): string => {
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
  return serialized ? `${STORAGE_V2_PATH}?${serialized}` : STORAGE_V2_PATH;
};

export const parseBackupsLinkSearch = (search: string) => {
  const params = new URLSearchParams(search);
  const queryValue =
    normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.query)) ||
    normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.legacyQuery));
  return {
    guestType: normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.guestType)),
    source: normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.source)),
    namespace: normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.namespace)),
    backupType: normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.backupType)),
    status: normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.status)),
    group: normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.group)),
    node: normalizeQueryValue(params.get(BACKUPS_QUERY_PARAMS.node)),
    query: queryValue,
  };
};

export const buildBackupsPath = (options: BackupsLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const guestType = normalizeQueryValue(options.guestType);
  const source = normalizeQueryValue(options.source);
  const namespace = normalizeQueryValue(options.namespace);
  const backupType = normalizeQueryValue(options.backupType);
  const status = normalizeQueryValue(options.status);
  const group = normalizeQueryValue(options.group);
  const node = normalizeQueryValue(options.node);
  const query = normalizeQueryValue(options.query);

  if (guestType) params.set(BACKUPS_QUERY_PARAMS.guestType, guestType);
  if (source) params.set(BACKUPS_QUERY_PARAMS.source, source);
  if (namespace) params.set(BACKUPS_QUERY_PARAMS.namespace, namespace);
  if (backupType) params.set(BACKUPS_QUERY_PARAMS.backupType, backupType);
  if (status) params.set(BACKUPS_QUERY_PARAMS.status, status);
  if (group) params.set(BACKUPS_QUERY_PARAMS.group, group);
  if (node) params.set(BACKUPS_QUERY_PARAMS.node, node);
  if (query) params.set(BACKUPS_QUERY_PARAMS.query, query);

  const serialized = params.toString();
  return serialized ? `/backups?${serialized}` : '/backups';
};

export const buildBackupsV2Path = (options: BackupsLinkOptions = {}): string => {
  const params = new URLSearchParams();
  const guestType = normalizeQueryValue(options.guestType);
  const source = normalizeQueryValue(options.source);
  const namespace = normalizeQueryValue(options.namespace);
  const backupType = normalizeQueryValue(options.backupType);
  const status = normalizeQueryValue(options.status);
  const group = normalizeQueryValue(options.group);
  const node = normalizeQueryValue(options.node);
  const query = normalizeQueryValue(options.query);

  if (guestType) params.set(BACKUPS_QUERY_PARAMS.guestType, guestType);
  if (source) params.set(BACKUPS_QUERY_PARAMS.source, source);
  if (namespace) params.set(BACKUPS_QUERY_PARAMS.namespace, namespace);
  if (backupType) params.set(BACKUPS_QUERY_PARAMS.backupType, backupType);
  if (status) params.set(BACKUPS_QUERY_PARAMS.status, status);
  if (group) params.set(BACKUPS_QUERY_PARAMS.group, group);
  if (node) params.set(BACKUPS_QUERY_PARAMS.node, node);
  if (query) params.set(BACKUPS_QUERY_PARAMS.query, query);

  const serialized = params.toString();
  return serialized ? `${BACKUPS_V2_PATH}?${serialized}` : BACKUPS_V2_PATH;
};
