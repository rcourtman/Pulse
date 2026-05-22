import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';
import type { Resource, ResourceChange, ResourceIncident, ResourceType } from '@/types/resource';

export type VmwarePageTabId = 'overview' | 'storage' | 'networks' | 'health' | 'activity';
export type VmwareDatastoreStatusFilter =
  | 'all'
  | 'accessible'
  | 'attention'
  | 'inaccessible'
  | 'maintenance'
  | 'unknown';
export type VmwareVirtualMachineStatusFilter =
  | 'all'
  | 'powered-on'
  | 'attention'
  | 'powered-off'
  | 'suspended'
  | 'unknown';
export type VmwareNetworkStatusFilter = 'all' | 'healthy' | 'attention' | 'unknown';
export type VmwareIncidentSeverityFilter = 'all' | 'critical' | 'warning' | 'info';
export type VmwareActivityStatusFilter = 'all' | 'tasks' | 'events' | 'failed';
export type VmwareActivityKind = 'task' | 'event' | 'activity';
export type VmwareActivityStateBucket = 'success' | 'running' | 'failed' | 'unknown';

export type VmwareTabSpec = {
  id: VmwarePageTabId;
  label: string;
  path: string;
};

// The Overview tab mirrors the vCenter inventory shape: ESXi hosts on top,
// vSphere VMs underneath grouped by runtime host. A dedicated `vms` tab would
// repeat the same inventory slice, so VM search and power filtering live inside
// the Overview table.
export const VMWARE_TAB_SPECS: readonly VmwareTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/vmware/overview' },
  { id: 'storage', label: 'Datastores', path: '/vmware/storage' },
  { id: 'networks', label: 'Networks', path: '/vmware/networks' },
  { id: 'health', label: 'Health', path: '/vmware/health' },
  { id: 'activity', label: 'Activity', path: '/vmware/activity' },
] as const;

const VMWARE_RESOURCE_TYPES = new Set<ResourceType>(['agent', 'vm', 'storage', 'network']);

const isVmwarePlatform = (resource: Resource): boolean =>
  resolveResourcePlatformType(resource) === 'vmware-vsphere';

export type VmwarePageModel = {
  resources: Resource[];
  hosts: Resource[];
  vms: Resource[];
  datastores: Resource[];
  networks: Resource[];
  incidents: VmwareIncidentRow[];
  activity: VmwareActivityRow[];
};

export type VmwareIncidentRow = {
  id: string;
  resource: Resource;
  resourceId: string;
  resourceName: string;
  resourceType: ResourceType;
  entityType: string;
  managedObjectId: string;
  severity: string;
  severityBucket: Exclude<VmwareIncidentSeverityFilter, 'all'>;
  code: string;
  source: string;
  summary: string;
  label: string;
  category: string;
  startedAt?: string;
  action: string;
  priority: number;
};

export type VmwareActivityRow = {
  id: string;
  resource: Resource;
  change: ResourceChange;
  resourceId: string;
  resourceName: string;
  resourceType: ResourceType;
  entityType: string;
  managedObjectId: string;
  activityKind: VmwareActivityKind;
  activityType: string;
  stateBucket: VmwareActivityStateBucket;
  title: string;
  state: string;
  message: string;
  description: string;
  actor: string;
  nativeId: string;
  source: string;
  occurredAt?: string;
  observedAt: string;
  sortTime: number;
};

export function buildVmwarePageModel(
  resources: Resource[],
  activityChanges: ResourceChange[] = [],
): VmwarePageModel {
  const vmwareResources = resources.filter(
    (resource) => isVmwarePlatform(resource) && VMWARE_RESOURCE_TYPES.has(resource.type),
  );
  const hosts = vmwareResources.filter((resource) => resource.type === 'agent');
  const vms = vmwareResources
    .filter((resource) => resource.type === 'vm')
    .sort(compareVmwareVirtualMachines);
  const datastores = vmwareResources
    .filter(
      (resource) =>
        resource.type === 'storage' &&
        (resource.storage?.topology === 'datastore' || resource.vmware?.entityType === 'datastore'),
    )
    .sort(compareVmwareDatastores);
  const networks = vmwareResources
    .filter((resource) => resource.type === 'network' && resource.vmware?.entityType === 'network')
    .sort(compareVmwareNetworks);
  const incidents = buildVmwareIncidentRows(vmwareResources);
  const activity = buildVmwareActivityRows(vmwareResources, activityChanges);

  return {
    resources: vmwareResources,
    hosts,
    vms,
    datastores,
    networks,
    incidents,
    activity,
  };
}

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

const trimString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const enumSearchValue = (value: unknown): string => {
  const trimmed = trimString(value);
  return trimmed ? `${trimmed} ${trimmed.replace(/[_-]/g, ' ')}` : '';
};

const normalizeToken = (value: unknown): string => normalize(value).replace(/[\s_-]/g, '');

const metadataString = (
  metadata: Record<string, unknown> | undefined,
  ...keys: string[]
): string => {
  if (!metadata) return '';
  for (const key of keys) {
    const value = trimString(metadata[key]);
    if (value) return value;
  }
  return '';
};

const addLookupKey = (keys: Set<string>, value: unknown): void => {
  const key = normalize(value);
  if (key) keys.add(key);
};

const vmwareSourceAlias = (
  connectionId: unknown,
  entityType: unknown,
  managedObjectId: unknown,
): string => {
  const parts = [
    trimString(connectionId),
    trimString(entityType),
    trimString(managedObjectId),
  ].filter(Boolean);
  return parts.join(':');
};

const vmwareDatastoreDisplayName = (resource: Resource): string =>
  resource.displayName?.trim() || resource.name?.trim() || resource.id;

const vmwareVirtualMachineDisplayName = (resource: Resource): string =>
  resource.displayName?.trim() || resource.name?.trim() || resource.id;

const vmwareResourceDisplayName = (resource: Resource): string =>
  trimString(resource.displayName) ||
  trimString(resource.name) ||
  trimString(resource.vmware?.runtimeHostName) ||
  trimString(resource.vmware?.managedObjectId) ||
  resource.id;

const vmwareDatastoreStatusRank = (resource: Resource): number => {
  switch (mapVmwareDatastoreStatus(resource)) {
    case 'inaccessible':
      return 0;
    case 'maintenance':
      return 1;
    case 'attention':
      return 2;
    case 'unknown':
      return 3;
    case 'accessible':
      return 4;
  }
};

const compareVmwareDatastores = (left: Resource, right: Resource): number => {
  const rankDelta = vmwareDatastoreStatusRank(left) - vmwareDatastoreStatusRank(right);
  if (rankDelta !== 0) return rankDelta;
  return vmwareDatastoreDisplayName(left).localeCompare(vmwareDatastoreDisplayName(right));
};

const vmwareNetworkDisplayName = (resource: Resource): string =>
  resource.displayName?.trim() || resource.name?.trim() || resource.id;

const vmwareNetworkStatusRank = (resource: Resource): number => {
  switch (mapVmwareNetworkStatus(resource)) {
    case 'attention':
      return 0;
    case 'unknown':
      return 1;
    case 'healthy':
      return 2;
  }
};

const compareVmwareNetworks = (left: Resource, right: Resource): number => {
  const rankDelta = vmwareNetworkStatusRank(left) - vmwareNetworkStatusRank(right);
  if (rankDelta !== 0) return rankDelta;
  return vmwareNetworkDisplayName(left).localeCompare(vmwareNetworkDisplayName(right));
};

const vmwareVirtualMachineHostKey = (resource: Resource): string =>
  normalize(resource.vmware?.runtimeHostName || resource.parentName || 'unknown');

const vmwareVirtualMachineStatusRank = (resource: Resource): number => {
  switch (mapVmwareVirtualMachineStatus(resource)) {
    case 'attention':
      return 0;
    case 'suspended':
      return 1;
    case 'powered-off':
      return 2;
    case 'unknown':
      return 3;
    case 'powered-on':
      return 4;
  }
};

const compareVmwareVirtualMachines = (left: Resource, right: Resource): number => {
  const hostDelta = vmwareVirtualMachineHostKey(left).localeCompare(
    vmwareVirtualMachineHostKey(right),
  );
  if (hostDelta !== 0) return hostDelta;
  const rankDelta = vmwareVirtualMachineStatusRank(left) - vmwareVirtualMachineStatusRank(right);
  if (rankDelta !== 0) return rankDelta;
  return vmwareVirtualMachineDisplayName(left).localeCompare(
    vmwareVirtualMachineDisplayName(right),
  );
};

const incidentSeverityRank = (severity: string): number => {
  switch (mapVmwareIncidentSeverity(severity)) {
    case 'critical':
      return 3;
    case 'warning':
      return 2;
    case 'info':
      return 1;
  }
};

export function mapVmwareIncidentSeverity(
  severity: string | undefined,
): Exclude<VmwareIncidentSeverityFilter, 'all'> {
  const normalized = normalize(severity);
  if (['critical', 'crit', 'fatal', 'error', 'failed', 'failure', 'red'].includes(normalized)) {
    return 'critical';
  }
  if (['warning', 'warn', 'alert', 'degraded', 'yellow'].includes(normalized)) return 'warning';
  return 'info';
}

export function mapVmwareVirtualMachineStatus(
  resource: Resource,
): Exclude<VmwareVirtualMachineStatusFilter, 'all'> {
  const powerState = normalizeToken(resource.vmware?.powerState);
  const status = normalize(resource.status);
  const overall = normalize(resource.vmware?.overallStatus);
  const activeAlarms = resource.vmware?.activeAlarmCount ?? 0;

  if (
    activeAlarms > 0 ||
    ['red', 'yellow', 'degraded', 'warning', 'critical', 'paused'].includes(overall) ||
    ['degraded', 'warning', 'critical', 'paused'].includes(status)
  ) {
    return 'attention';
  }
  if (['poweredoff', 'off'].includes(powerState) || status === 'offline' || status === 'stopped') {
    return 'powered-off';
  }
  if (['suspended', 'suspend'].includes(powerState) || status === 'paused') return 'suspended';
  if (
    ['poweredon', 'on', 'running'].includes(powerState) ||
    status === 'online' ||
    status === 'running'
  ) {
    return 'powered-on';
  }
  return 'unknown';
}

export function mapVmwareDatastoreStatus(
  resource: Resource,
): Exclude<VmwareDatastoreStatusFilter, 'all'> {
  const maintenanceMode = normalize(resource.vmware?.maintenanceMode);
  const status = normalize(resource.status);
  const overall = normalize(resource.vmware?.overallStatus);

  if (
    maintenanceMode &&
    maintenanceMode !== 'normal' &&
    maintenanceMode !== 'none' &&
    maintenanceMode !== 'not_in_maintenance'
  ) {
    return 'maintenance';
  }
  if (resource.vmware?.datastoreAccessible === false || status === 'offline') {
    return 'inaccessible';
  }
  if (['red', 'yellow', 'degraded', 'warning', 'critical', 'paused'].includes(overall)) {
    return 'attention';
  }
  if (['degraded', 'warning', 'critical', 'paused'].includes(status)) return 'attention';
  if (resource.vmware?.datastoreAccessible === true || ['online', 'running'].includes(status)) {
    return 'accessible';
  }
  return 'unknown';
}

export function mapVmwareNetworkStatus(
  resource: Resource,
): Exclude<VmwareNetworkStatusFilter, 'all'> {
  const status = normalize(resource.status);
  const overall = normalize(resource.vmware?.overallStatus);
  const activeAlarms = resource.vmware?.activeAlarmCount ?? 0;

  if (
    activeAlarms > 0 ||
    ['red', 'yellow', 'degraded', 'warning', 'critical', 'paused'].includes(overall) ||
    ['degraded', 'warning', 'critical', 'paused', 'offline'].includes(status)
  ) {
    return 'attention';
  }
  if (['online', 'running'].includes(status) || overall === 'green') return 'healthy';
  return 'unknown';
}

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const hasIncidentSignal = (incident: ResourceIncident): boolean =>
  Boolean(trimString(incident.code) || trimString(incident.summary));

const hasIncidentRollup = (resource: Resource): boolean =>
  (resource.incidentCount ?? 0) > 0 ||
  Boolean(
    trimString(resource.incidentCode) ||
    trimString(resource.incidentSummary) ||
    trimString(resource.incidentLabel),
  );

const vmwareIncidentLabel = (resource: Resource, incident: ResourceIncident): string => {
  const label = trimString(resource.incidentLabel);
  if (label) return label;
  const code = trimString(incident.code);
  if (code === 'vmware_alarm_state') return 'vSphere Alarm';
  if (code === 'vmware_health_state') return 'vSphere Health';
  return code ? titleize(code.replace(/^vmware_/, '')) : 'vSphere Health';
};

const buildIncidentRow = (
  resource: Resource,
  incident: ResourceIncident,
  index: number,
): VmwareIncidentRow => {
  const severity = trimString(incident.severity) || trimString(resource.incidentSeverity) || 'info';
  const code = trimString(incident.code) || trimString(resource.incidentCode) || 'vmware_alert';
  const summary =
    trimString(incident.summary) ||
    trimString(resource.incidentSummary) ||
    vmwareIncidentLabel(resource, incident);
  const nativeId = trimString(incident.nativeId);
  const rowKey = nativeId || code || String(index);

  return {
    id: `${resource.id}:incident:${rowKey}:${index}`,
    resource,
    resourceId: resource.id,
    resourceName: vmwareResourceDisplayName(resource),
    resourceType: resource.type,
    entityType: trimString(resource.vmware?.entityType) || resource.type,
    managedObjectId: trimString(resource.vmware?.managedObjectId) || resource.id,
    severity,
    severityBucket: mapVmwareIncidentSeverity(severity),
    code,
    source: trimString(incident.source) || trimString(incident.provider) || 'vmware',
    summary,
    label: vmwareIncidentLabel(resource, incident),
    category: trimString(resource.incidentCategory) || 'vcenter-health',
    startedAt: incident.startedAt,
    action: trimString(resource.incidentAction) || 'Investigate in vCenter',
    priority: resource.incidentPriority ?? incidentSeverityRank(severity) * 1000,
  };
};

const buildRollupIncidentRow = (resource: Resource): VmwareIncidentRow => {
  const severity = trimString(resource.incidentSeverity) || 'info';
  const code = trimString(resource.incidentCode) || 'vmware_alert';
  const count = resource.incidentCount ?? 0;
  const summary =
    trimString(resource.incidentSummary) ||
    trimString(resource.incidentLabel) ||
    `${count || 1} active vSphere alert${count === 1 ? '' : 's'}`;
  const incident: ResourceIncident = {
    code,
    severity,
    summary,
    source: 'vmware',
  };

  return {
    ...buildIncidentRow(resource, incident, 0),
    id: `${resource.id}:incident:rollup`,
  };
};

export function buildVmwareIncidentRows(resources: Resource[]): VmwareIncidentRow[] {
  const rows: VmwareIncidentRow[] = [];
  for (const resource of resources) {
    const incidents = (resource.incidents ?? []).filter(hasIncidentSignal);
    if (incidents.length > 0) {
      incidents.forEach((incident, index) =>
        rows.push(buildIncidentRow(resource, incident, index)),
      );
      continue;
    }
    if (hasIncidentRollup(resource)) {
      rows.push(buildRollupIncidentRow(resource));
    }
  }

  return rows.sort((a, b) => {
    const severityDelta = incidentSeverityRank(b.severity) - incidentSeverityRank(a.severity);
    if (severityDelta !== 0) return severityDelta;
    const priorityDelta = b.priority - a.priority;
    if (priorityDelta !== 0) return priorityDelta;
    const entityDelta = a.entityType.localeCompare(b.entityType);
    if (entityDelta !== 0) return entityDelta;
    return a.resourceName.localeCompare(b.resourceName);
  });
}

const isVmwareActivityChange = (change: ResourceChange): boolean => {
  if (change.kind !== 'activity') return false;
  if (trimString(change.sourceAdapter) === 'vmware_adapter') return true;
  const activityType = metadataString(change.metadata, 'activity_type');
  if (activityType.startsWith('vmware_')) return true;
  return Boolean(
    metadataString(
      change.metadata,
      'vmwareTask',
      'vmwareTaskName',
      'vmwareTaskState',
      'vmwareEvent',
      'vmwareEventType',
      'vmwareEventMessage',
    ),
  );
};

const mapVmwareActivityKind = (
  activityType: string,
  metadata: Record<string, unknown> | undefined,
): VmwareActivityKind => {
  const normalized = normalizeToken(activityType);
  if (
    normalized.includes('task') ||
    metadataString(metadata, 'vmwareTask', 'vmwareTaskName', 'vmwareTaskState')
  ) {
    return 'task';
  }
  if (
    normalized.includes('event') ||
    metadataString(metadata, 'vmwareEvent', 'vmwareEventType', 'vmwareEventMessage')
  ) {
    return 'event';
  }
  return 'activity';
};

export function mapVmwareActivityStateBucket(state: string): VmwareActivityStateBucket {
  const normalized = normalizeToken(state);
  if (
    ['error', 'failed', 'failure', 'cancelled', 'canceled', 'timedout', 'timeout', 'red'].includes(
      normalized,
    )
  ) {
    return 'failed';
  }
  if (['running', 'queued', 'pending', 'inprogress', 'started'].includes(normalized)) {
    return 'running';
  }
  if (['success', 'succeeded', 'complete', 'completed', 'ok', 'green'].includes(normalized)) {
    return 'success';
  }
  return 'unknown';
}

const parseActivitySortTime = (change: ResourceChange): number => {
  const value = trimString(change.occurredAt) || trimString(change.observedAt);
  const parsed = value ? new Date(value).getTime() : Number.NaN;
  return Number.isFinite(parsed) ? parsed : 0;
};

const vmwareActivityResourceKeys = (resource: Resource): Set<string> => {
  const keys = new Set<string>();
  addLookupKey(keys, resource.id);
  addLookupKey(keys, resource.canonicalIdentity?.primaryId);
  for (const alias of resource.canonicalIdentity?.aliases ?? []) {
    addLookupKey(keys, alias);
  }
  const sourceAlias = vmwareSourceAlias(
    resource.vmware?.connectionId,
    resource.vmware?.entityType,
    resource.vmware?.managedObjectId,
  );
  addLookupKey(keys, sourceAlias);
  addLookupKey(keys, resource.vmware?.managedObjectId);
  if (sourceAlias && resource.type === 'storage') {
    addLookupKey(keys, `storage:${sourceAlias}`);
  }
  return keys;
};

const vmwareActivityChangeKeys = (change: ResourceChange): Set<string> => {
  const keys = new Set<string>();
  addLookupKey(keys, change.resourceId);
  const metadata = change.metadata;
  const sourceAlias = vmwareSourceAlias(
    metadataString(metadata, 'vmwareConnectionId'),
    metadataString(metadata, 'vmwareEntityType'),
    metadataString(metadata, 'vmwareManagedObjectId'),
  );
  addLookupKey(keys, sourceAlias);
  addLookupKey(keys, metadataString(metadata, 'vmwareManagedObjectId'));
  if (sourceAlias) {
    addLookupKey(keys, `storage:${sourceAlias}`);
  }
  return keys;
};

const buildVmwareActivityResourceIndex = (resources: Resource[]): Map<string, Resource> => {
  const index = new Map<string, Resource>();
  for (const resource of resources) {
    for (const key of vmwareActivityResourceKeys(resource)) {
      if (!index.has(key)) {
        index.set(key, resource);
      }
    }
  }
  return index;
};

const resolveVmwareActivityResource = (
  change: ResourceChange,
  resourceIndex: Map<string, Resource>,
): Resource | undefined => {
  for (const key of vmwareActivityChangeKeys(change)) {
    const resource = resourceIndex.get(key);
    if (resource) return resource;
  }
  return undefined;
};

const activityChangeDedupeKey = (resource: Resource, change: ResourceChange): string =>
  [
    resource.id,
    trimString(change.id),
    trimString(change.resourceId),
    trimString(change.observedAt),
    trimString(change.reason),
  ].join('|');

const buildActivityRow = (
  resource: Resource,
  change: ResourceChange,
  index: number,
): VmwareActivityRow => {
  const metadata = change.metadata;
  const activityType = metadataString(metadata, 'activity_type');
  const activityKind = mapVmwareActivityKind(activityType, metadata);
  const title =
    metadataString(metadata, 'activity_title', 'vmwareTaskName', 'vmwareEventType') ||
    trimString(change.reason) ||
    'vSphere activity';
  const state =
    metadataString(metadata, 'activity_state', 'vmwareTaskState') || trimString(change.to);
  const message =
    metadataString(metadata, 'activity_message', 'vmwareTaskError', 'vmwareEventMessage') ||
    trimString(change.reason);
  const description = metadataString(metadata, 'vmwareTaskDescription', 'vmwareEventMessage');
  const actor = trimString(change.actor) || metadataString(metadata, 'vmwareEventUser');
  const nativeId = metadataString(metadata, 'activity_native_id', 'vmwareTask', 'vmwareEvent');
  const observedAt = trimString(change.observedAt);
  const occurredAt = trimString(change.occurredAt) || undefined;

  return {
    id: `${resource.id}:activity:${change.id || index}`,
    resource,
    change,
    resourceId: resource.id,
    resourceName: vmwareResourceDisplayName(resource),
    resourceType: resource.type,
    entityType:
      metadataString(metadata, 'vmwareEntityType') ||
      trimString(resource.vmware?.entityType) ||
      resource.type,
    managedObjectId:
      metadataString(metadata, 'vmwareManagedObjectId') ||
      trimString(resource.vmware?.managedObjectId) ||
      resource.id,
    activityKind,
    activityType,
    stateBucket: mapVmwareActivityStateBucket(state),
    title,
    state,
    message,
    description,
    actor,
    nativeId: nativeId || change.id,
    source: trimString(change.sourceAdapter) || trimString(change.sourceType) || 'vmware',
    occurredAt,
    observedAt,
    sortTime: parseActivitySortTime(change),
  };
};

export function buildVmwareActivityRows(
  resources: Resource[],
  activityChanges: ResourceChange[] = [],
): VmwareActivityRow[] {
  const resourceIndex = buildVmwareActivityResourceIndex(resources);
  const seen = new Set<string>();
  const rows: VmwareActivityRow[] = [];

  const appendRow = (resource: Resource, change: ResourceChange) => {
    if (!isVmwareActivityChange(change)) return;
    const key = activityChangeDedupeKey(resource, change);
    if (seen.has(key)) return;
    seen.add(key);
    rows.push(buildActivityRow(resource, change, rows.length));
  };

  for (const resource of resources) {
    for (const change of resource.recentChanges ?? []) {
      appendRow(resource, change);
    }
  }

  for (const change of activityChanges) {
    const resource = resolveVmwareActivityResource(change, resourceIndex);
    if (resource) {
      appendRow(resource, change);
    }
  }

  return rows.sort((left, right) => {
    const timeDelta = right.sortTime - left.sortTime;
    if (timeDelta !== 0) return timeDelta;
    const resourceDelta = left.resourceName.localeCompare(right.resourceName);
    if (resourceDelta !== 0) return resourceDelta;
    return left.id.localeCompare(right.id);
  });
}

const vmwareDatastoreSearchHaystack = (resource: Resource): string =>
  [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.status,
    resource.vmware?.connectionName,
    resource.vmware?.vcenterHost,
    resource.vmware?.managedObjectId,
    resource.vmware?.datacenterName,
    resource.vmware?.folderName,
    resource.vmware?.datastoreType,
    resource.vmware?.datastoreUrl,
    resource.vmware?.maintenanceMode,
    resource.vmware?.overallStatus,
    resource.storage?.type,
    resource.storage?.topology,
    resource.storage?.platform,
    resource.storage?.nodes?.join(' '),
    resource.storage?.consumerTypes?.join(' '),
    resource.storage?.topConsumers?.map((consumer) => consumer.name).join(' '),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterVmwareDatastores(
  datastores: Resource[],
  search: string,
  status: VmwareDatastoreStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return datastores.filter((datastore) => {
    if (status !== 'all' && mapVmwareDatastoreStatus(datastore) !== status) return false;
    if (!needle) return true;
    return vmwareDatastoreSearchHaystack(datastore).includes(needle);
  });
}

const vmwareNetworkSearchHaystack = (resource: Resource): string =>
  [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.status,
    resource.vmware?.connectionName,
    resource.vmware?.vcenterHost,
    resource.vmware?.managedObjectId,
    resource.vmware?.datacenterName,
    resource.vmware?.folderName,
    resource.vmware?.networkType,
    resource.vmware?.overallStatus,
    resource.vmware?.networkHostNames?.join(' '),
    resource.vmware?.networkVmNames?.join(' '),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterVmwareNetworks(
  networks: Resource[],
  search: string,
  status: VmwareNetworkStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return networks.filter((network) => {
    if (status !== 'all' && mapVmwareNetworkStatus(network) !== status) return false;
    if (!needle) return true;
    return vmwareNetworkSearchHaystack(network).includes(needle);
  });
}

const vmwareVirtualMachineSearchHaystack = (resource: Resource): string =>
  [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.status,
    resource.vmware?.connectionName,
    resource.vmware?.vcenterHost,
    resource.vmware?.managedObjectId,
    resource.vmware?.datacenterName,
    resource.vmware?.clusterName,
    formatVmwareClusterServices(resource.vmware),
    resource.vmware?.computeResourceName,
    resource.vmware?.folderName,
    resource.vmware?.resourcePoolName,
    resource.vmware?.runtimeHostName,
    resource.vmware?.powerState,
    resource.vmware?.overallStatus,
    resource.vmware?.instanceUuid,
    resource.vmware?.biosUuid,
    resource.vmware?.guestOsFamily,
    resource.vmware?.guestHostname,
    resource.vmware?.guestIpAddresses?.join(' '),
    resource.vmware?.datastoreNames?.join(' '),
    resource.vmware?.networkAdapters
      ?.map((adapter) =>
        [
          adapter.label,
          adapter.type,
          adapter.macAddress,
          adapter.backingType,
          adapter.networkId,
          adapter.networkName,
          adapter.opaqueNetworkId,
          adapter.hostDevice,
          adapter.state,
        ]
          .filter(Boolean)
          .join(' '),
      )
      .join(' '),
    resource.vmware?.virtualDisks
      ?.map((disk) =>
        [disk.disk, disk.label, disk.type, disk.backingType, disk.vmdkFile, disk.datastoreName]
          .filter(Boolean)
          .join(' '),
      )
      .join(' '),
    [
      resource.vmware?.tools?.runState,
      resource.vmware?.tools?.versionStatus,
      resource.vmware?.tools?.version,
      resource.vmware?.tools?.installType,
      resource.vmware?.tools?.upgradePolicy,
      resource.vmware?.tools?.errorMessage,
      resource.vmware?.tools?.guestRebootComponents?.join(' '),
    ]
      .filter(Boolean)
      .join(' '),
    [
      enumSearchValue(resource.vmware?.hardware?.guestOs),
      enumSearchValue(resource.vmware?.hardware?.version),
      enumSearchValue(resource.vmware?.hardware?.upgradePolicy),
      enumSearchValue(resource.vmware?.hardware?.upgradeVersion),
      enumSearchValue(resource.vmware?.hardware?.upgradeStatus),
      resource.vmware?.hardware?.upgradeErrorMessage,
      enumSearchValue(resource.vmware?.hardware?.bootType),
      enumSearchValue(resource.vmware?.hardware?.bootNetworkProtocol),
      resource.vmware?.hardware?.bootDevices
        ?.map((device) =>
          [device.type, device.nic, device.disks?.join(' ')].filter(Boolean).join(' '),
        )
        .join(' '),
    ]
      .filter(Boolean)
      .join(' '),
    resource.vmware?.activeAlarmSummary,
    resource.vmware?.recentTaskSummary,
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterVmwareVirtualMachines(
  vms: Resource[],
  search: string,
  status: VmwareVirtualMachineStatusFilter,
): Resource[] {
  const needle = normalize(search);
  return vms.filter((vm) => {
    if (status !== 'all' && mapVmwareVirtualMachineStatus(vm) !== status) return false;
    if (!needle) return true;
    return vmwareVirtualMachineSearchHaystack(vm).includes(needle);
  });
}

const incidentSearchHaystack = (row: VmwareIncidentRow): string =>
  [
    row.resourceId,
    row.resourceName,
    row.resourceType,
    row.entityType,
    row.managedObjectId,
    row.severity,
    row.code,
    row.source,
    row.summary,
    row.label,
    row.category,
    row.action,
    row.resource.vmware?.connectionName,
    row.resource.vmware?.vcenterHost,
    row.resource.vmware?.datacenterName,
    row.resource.vmware?.clusterName,
    row.resource.vmware?.runtimeHostName,
    row.resource.vmware?.activeAlarmSummary,
    ...(row.resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterVmwareIncidents(
  incidents: VmwareIncidentRow[],
  search: string,
  severity: VmwareIncidentSeverityFilter,
): VmwareIncidentRow[] {
  const needle = normalize(search);
  return incidents.filter((incident) => {
    if (severity !== 'all' && incident.severityBucket !== severity) return false;
    if (!needle) return true;
    return incidentSearchHaystack(incident).includes(needle);
  });
}

const activitySearchHaystack = (row: VmwareActivityRow): string =>
  [
    row.id,
    row.resourceId,
    row.resourceName,
    row.resourceType,
    row.entityType,
    row.managedObjectId,
    row.activityKind,
    row.activityType,
    row.title,
    row.state,
    row.message,
    row.description,
    row.actor,
    row.nativeId,
    row.source,
    row.resource.vmware?.connectionName,
    row.resource.vmware?.vcenterHost,
    row.resource.vmware?.datacenterName,
    row.resource.vmware?.clusterName,
    row.resource.vmware?.runtimeHostName,
    ...(row.change.relatedResources ?? []),
    ...(row.resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterVmwareActivity(
  activity: VmwareActivityRow[],
  search: string,
  status: VmwareActivityStatusFilter,
): VmwareActivityRow[] {
  const needle = normalize(search);
  return activity.filter((row) => {
    if (status === 'tasks' && row.activityKind !== 'task') return false;
    if (status === 'events' && row.activityKind !== 'event') return false;
    if (status === 'failed' && row.stateBucket !== 'failed') return false;
    if (!needle) return true;
    return activitySearchHaystack(row).includes(needle);
  });
}
