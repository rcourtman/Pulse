import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import {
  formatVmwareClusterServices,
  getVmwareStaleNotice,
  isVmwareSourceImpaired,
  type VmwareStaleNotice,
} from '@/utils/vmwareDisplay';
import type { Resource, ResourceChange, ResourceIncident, ResourceType } from '@/types/resource';
import type { PlatformIssue } from '@/features/platformPage/PlatformIssueReason';
import { getPlatformResourceStaleNotice } from '@/features/platformPage/resourceStaleness';
import { getSimpleStatusIndicator, type StatusIndicator } from '@/utils/status';
import { workloadNodeScopeId } from '@/components/Workloads/workloadTopology';

export type VmwarePageTabId = 'overview' | 'storage' | 'networks' | 'health' | 'activity';
export type VmwareDatastoreStatusFilter =
  'all' | 'accessible' | 'attention' | 'inaccessible' | 'maintenance' | 'unknown';
export type VmwareVirtualMachineStatusFilter =
  'all' | 'powered-on' | 'attention' | 'powered-off' | 'suspended' | 'unknown';
export type VmwareNetworkStatusFilter = 'all' | 'healthy' | 'attention' | 'unknown';
export type VmwareIncidentSeverityFilter = 'all' | 'attention' | 'critical' | 'warning' | 'info';
export type VmwareIncidentSeverityBucket = Exclude<
  VmwareIncidentSeverityFilter,
  'all' | 'attention'
>;
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
  severityBucket: VmwareIncidentSeverityBucket;
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
  // vCenter's event class (HostConnectedEvent); the title reads as vCenter's
  // own message when it sent one, so the class is kept for search and detail.
  eventType: string;
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

export type VmwareTabInventoryEvidence = {
  typeCounts?: Partial<Record<ResourceType, number>>;
  incidentCount?: number;
  hasActivityInventory?: boolean;
};

const hasVmwareTabInventory = (
  model: VmwarePageModel,
  tab: VmwarePageTabId,
  evidence: VmwareTabInventoryEvidence = {},
): boolean => {
  switch (tab) {
    case 'overview':
      return true;
    case 'storage':
      return model.datastores.length > 0 || (evidence.typeCounts?.storage ?? 0) > 0;
    case 'networks':
      return model.networks.length > 0 || (evidence.typeCounts?.network ?? 0) > 0;
    case 'health':
      return model.incidents.length > 0 || (evidence.incidentCount ?? 0) > 0;
    case 'activity':
      return model.activity.length > 0 || Boolean(evidence.hasActivityInventory);
  }
};

export const getVmwarePageTabSpecs = (
  model: VmwarePageModel,
  evidence: VmwareTabInventoryEvidence = {},
): readonly VmwareTabSpec[] =>
  VMWARE_TAB_SPECS.filter((tab) => hasVmwareTabInventory(model, tab.id, evidence));

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

const vmwareConnectionFields = (resource: Resource) => ({
  id: resource.vmware?.connectionId?.trim().toLowerCase() || '',
  aliases: [resource.vmware?.connectionName, resource.vmware?.vcenterHost]
    .map((value) => value?.trim().toLowerCase() || '')
    .filter(Boolean),
});

// Whether the page's vSphere inventory comes from one vCenter connection, which
// makes a vCenter column repeat on every row. Decided from the whole inventory,
// never from filtered rows, so a search that narrows to one vCenter does not
// drop the column. A resource without a connection ID is matched to a known
// connection by its name or host, so one vCenter is never counted twice.
export const isSingleVmwareVcenter = (resources: readonly Resource[]): boolean => {
  const fields = resources.map(vmwareConnectionFields);
  const idByAlias = new Map<string, string>();
  for (const { id, aliases } of fields) {
    if (id) aliases.forEach((alias) => idByAlias.set(alias, id));
  }
  const connections = new Set<string>();
  for (const { id, aliases } of fields) {
    const key = id || aliases.map((alias) => idByAlias.get(alias)).find(Boolean) || aliases[0];
    if (key) connections.add(key);
  }
  return connections.size <= 1;
};

export const vmwareVcenterLabel = (resource: Resource): string =>
  resource.vmware?.connectionName?.trim() || resource.vmware?.vcenterHost?.trim() || '';

/**
 * The VM-table scope a vSphere host row narrows to: the scope the workload
 * table already gave one of the host's own VMs (`workloadScopeById`, keyed by
 * VM resource ID), so host and VM rows never disagree. A VM names its host by
 * the host's managed object ID, which only one vCenter connection gives that
 * host. A host with no listed VMs gets a scope that names it and matches
 * nothing, so the table reads "VMs on <host>" and is empty.
 */
export const getVsphereHostWorkloadScope = (
  host: Resource,
  resources: readonly Resource[],
  workloadScopeById: ReadonlyMap<string, string>,
): string => {
  const hostObjectId = host.vmware?.managedObjectId?.trim() || '';
  const connectionId = host.vmware?.connectionId?.trim() || '';
  for (const resource of resources) {
    if (
      hostObjectId &&
      resource.type === 'vm' &&
      resource.vmware?.runtimeHostId?.trim() === hostObjectId &&
      (resource.vmware?.connectionId?.trim() || '') === connectionId
    ) {
      const scope = workloadScopeById.get(resource.id);
      if (scope) return scope;
    }
  }
  return workloadNodeScopeId({
    node: host.name,
    instance:
      host.vmware?.clusterName ?? host.vmware?.datacenterName ?? host.vmware?.connectionName ?? '',
  });
};

const SEVERE_VMWARE_RESOURCE_STATUSES = new Set([
  'offline',
  'stopped',
  'error',
  'failed',
  'faulted',
  'critical',
  'unhealthy',
]);

export const getVmwareResourceDisplayStatus = (resource: Resource): string => {
  if (SEVERE_VMWARE_RESOURCE_STATUSES.has(normalize(resource.status))) return resource.status;
  return isVmwareSourceImpaired(resource) ? 'degraded' : resource.status;
};

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

const vmwareShortName = (resource: Resource): string =>
  resource.displayName?.trim() || resource.name?.trim() || resource.id;

const vmwareResourceDisplayName = (resource: Resource): string =>
  trimString(resource.displayName) ||
  trimString(resource.name) ||
  trimString(resource.vmware?.runtimeHostName) ||
  trimString(resource.vmware?.managedObjectId) ||
  resource.id;

const DATASTORE_STATUS_RANK: Record<Exclude<VmwareDatastoreStatusFilter, 'all'>, number> = {
  inaccessible: 0,
  maintenance: 1,
  attention: 2,
  unknown: 3,
  accessible: 4,
};

const compareVmwareDatastores = (left: Resource, right: Resource): number =>
  DATASTORE_STATUS_RANK[mapVmwareDatastoreStatus(left)] -
    DATASTORE_STATUS_RANK[mapVmwareDatastoreStatus(right)] ||
  vmwareShortName(left).localeCompare(vmwareShortName(right));

const NETWORK_STATUS_RANK: Record<Exclude<VmwareNetworkStatusFilter, 'all'>, number> = {
  attention: 0,
  unknown: 1,
  healthy: 2,
};

const compareVmwareNetworks = (left: Resource, right: Resource): number =>
  NETWORK_STATUS_RANK[mapVmwareNetworkStatus(left)] -
    NETWORK_STATUS_RANK[mapVmwareNetworkStatus(right)] ||
  vmwareShortName(left).localeCompare(vmwareShortName(right));

const vmwareVirtualMachineHostKey = (resource: Resource): string =>
  normalize(resource.vmware?.runtimeHostName || resource.parentName || 'unknown');

const VM_STATUS_RANK: Record<Exclude<VmwareVirtualMachineStatusFilter, 'all'>, number> = {
  attention: 0,
  suspended: 1,
  'powered-off': 2,
  unknown: 3,
  'powered-on': 4,
};

const compareVmwareVirtualMachines = (left: Resource, right: Resource): number =>
  vmwareVirtualMachineHostKey(left).localeCompare(vmwareVirtualMachineHostKey(right)) ||
  VM_STATUS_RANK[mapVmwareVirtualMachineStatus(left)] -
    VM_STATUS_RANK[mapVmwareVirtualMachineStatus(right)] ||
  vmwareShortName(left).localeCompare(vmwareShortName(right));

const SEVERITY_RANK: Record<VmwareIncidentSeverityBucket, number> = {
  critical: 3,
  warning: 2,
  info: 1,
};

const incidentSeverityRank = (severity: string): number =>
  SEVERITY_RANK[mapVmwareIncidentSeverity(severity)];

export function mapVmwareIncidentSeverity(
  severity: string | undefined,
): VmwareIncidentSeverityBucket {
  const normalized = normalize(severity);
  if (/^(crit(ical)?|fatal|error|fail(ed|ure)|red)$/.test(normalized)) return 'critical';
  if (/^(warn(ing)?|alert|degraded|yellow)$/.test(normalized)) return 'warning';
  return 'info';
}

export function normalizeVmwarePowerStateToken(value: string | undefined): string {
  return (value || '')
    .trim()
    .toLowerCase()
    .replace(/[\s_-]/g, '');
}

const POWER_STATE_INFO: Record<string, [string, 'success' | 'warning' | 'danger']> = {
  poweredon: ['On', 'success'],
  on: ['On', 'success'],
  poweredoff: ['Off', 'danger'],
  off: ['Off', 'danger'],
  suspended: ['Suspended', 'warning'],
};

export function formatVmwarePowerState(value: string | undefined): string {
  return (
    POWER_STATE_INFO[normalizeVmwarePowerStateToken(value)]?.[0] ?? (trimString(value) || 'Unknown')
  );
}

export function getVmwarePowerStateVariant(
  state: string | undefined,
): 'success' | 'warning' | 'danger' | 'muted' {
  return POWER_STATE_INFO[normalizeVmwarePowerStateToken(state)]?.[1] ?? 'muted';
}

const ATTENTION_STATUSES = new Set(['degraded', 'warning', 'critical', 'paused']);
const ATTENTION_OVERALL_STATUSES = new Set(['red', 'yellow']);
const isAttentionOverall = (overall: string): boolean =>
  ATTENTION_OVERALL_STATUSES.has(overall) || ATTENTION_STATUSES.has(overall);

export function mapVmwareVirtualMachineStatus(
  resource: Resource,
): Exclude<VmwareVirtualMachineStatusFilter, 'all'> {
  if (isVmwareSourceImpaired(resource)) return 'attention';
  const powerState = normalizeToken(resource.vmware?.powerState);
  const status = normalize(getVmwareResourceDisplayStatus(resource));
  const overall = normalize(resource.vmware?.overallStatus);
  const activeAlarms = resource.vmware?.activeAlarmCount ?? 0;

  if (activeAlarms > 0 || isAttentionOverall(overall) || ATTENTION_STATUSES.has(status)) {
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

const datastoreInaccessible = (resource: Resource): boolean =>
  resource.vmware?.datastoreAccessible === false || normalize(resource.status) === 'offline';

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
  if (datastoreInaccessible(resource)) return 'inaccessible';
  if (isVmwareSourceImpaired(resource)) return 'attention';
  if (isAttentionOverall(overall) || ATTENTION_STATUSES.has(status)) return 'attention';
  if (resource.vmware?.datastoreAccessible === true || ['online', 'running'].includes(status)) {
    return 'accessible';
  }
  return 'unknown';
}

export function mapVmwareNetworkStatus(
  resource: Resource,
): Exclude<VmwareNetworkStatusFilter, 'all'> {
  const status = normalize(getVmwareResourceDisplayStatus(resource));
  const overall = normalize(resource.vmware?.overallStatus);
  const activeAlarms = resource.vmware?.activeAlarmCount ?? 0;

  if (
    activeAlarms > 0 ||
    isAttentionOverall(overall) ||
    ATTENTION_STATUSES.has(status) ||
    status === 'offline'
  ) {
    return 'attention';
  }
  if (['online', 'running'].includes(status) || overall === 'green') return 'healthy';
  return 'unknown';
}

// Stale rows show the non-colour Stale data cue beside the name. Keep alarms
// and outages ahead of the generic staleness line while surfacing collection
// errors or filling in when staleness is the only attention reason.
const vmwareAttentionReasons = (
  resource: Resource,
  leading: string[] = [],
  staleNotice: VmwareStaleNotice | null = null,
): string[] => {
  const reasons = [...leading];
  const push = (value: unknown) => {
    const text = trimString(value);
    if (text && !reasons.some((existing) => existing.toLowerCase() === text.toLowerCase())) {
      reasons.push(text);
    }
  };
  const incidents = (resource.incidents ?? [])
    .filter(hasIncidentSignal)
    .sort(
      (a, b) => incidentSeverityRank(b.severity ?? '') - incidentSeverityRank(a.severity ?? ''),
    );
  const beforeIncidents = reasons.length;
  incidents.forEach((incident) =>
    push(incident.summary || vmwareIncidentLabel(resource, incident)),
  );
  if (reasons.length === beforeIncidents) {
    const overall = normalize(resource.vmware?.overallStatus);
    const alarms = resource.vmware?.activeAlarmCount ?? 0;
    if (ATTENTION_OVERALL_STATUSES.has(overall)) push(`vCenter health is ${overall}`);
    else if (alarms > 0) push(`${alarms} active vCenter alarm${alarms === 1 ? '' : 's'}`);
  }
  if (staleNotice && (!reasons.length || staleNotice.error)) push(staleNotice.detail);
  return reasons;
};

const hasCriticalVmwareSignal = (resource: Resource): boolean =>
  normalize(resource.vmware?.overallStatus) === 'red' ||
  (resource.incidents ?? []).some(
    (incident) => mapVmwareIncidentSeverity(incident.severity) === 'critical',
  );

// A row's status dot and its Health reason come from one classification, so a
// datastore in maintenance or a network with an active alarm never shows a
// green dot beside an amber reason.
export const getVmwareRowIndicator = (
  resource: Resource,
  issue: PlatformIssue | null,
  status = getVmwareResourceDisplayStatus(resource),
): StatusIndicator =>
  issue
    ? { variant: issue.tone === 'danger' ? 'danger' : 'warning', label: issue.label }
    : getSimpleStatusIndicator(status);

// Health sorts the worst rows first: red, then amber, then healthy.
export const vmwareIssueSortRank = (issue: PlatformIssue | null): number =>
  issue ? (issue.tone === 'danger' ? 0 : 1) : 2;

const DATASTORE_ISSUE_LABELS = {
  inaccessible: 'Inaccessible',
  maintenance: 'Maintenance',
  attention: 'Attention',
} as const;

export type VmwareRowIssue = PlatformIssue & {
  stale: boolean;
};

export function getVmwareDatastoreIssue(resource: Resource): VmwareRowIssue | null {
  const bucket = mapVmwareDatastoreStatus(resource);
  if (bucket === 'accessible' || bucket === 'unknown') return null;
  // The bucket picks one filter, but a datastore in maintenance can also be
  // unreachable or carry a critical alarm: each condition gives its reason,
  // and the worst sets the colour.
  const staleNotice = getVmwareStaleNotice(resource);
  const stale = staleNotice !== null;
  const inaccessible = datastoreInaccessible(resource);
  const leading: string[] = [];
  if (inaccessible)
    leading.push(stale ? 'Last reported inaccessible' : 'vCenter reports it inaccessible');
  if (bucket === 'maintenance') {
    leading.push(
      normalize(resource.vmware?.maintenanceMode).includes('entering')
        ? 'Entering maintenance mode'
        : 'In maintenance mode',
    );
  }
  const reasons = vmwareAttentionReasons(resource, leading, staleNotice);
  return {
    tone: inaccessible || hasCriticalVmwareSignal(resource) ? 'danger' : 'warning',
    label: DATASTORE_ISSUE_LABELS[bucket],
    reasons: reasons.length ? reasons : ['vCenter reports a problem'],
    stale,
  };
}

export function getVmwareNetworkIssue(resource: Resource): VmwareRowIssue | null {
  if (mapVmwareNetworkStatus(resource) !== 'attention') return null;
  const staleNotice = getVmwareStaleNotice(resource);
  const stale = staleNotice !== null;
  const offline = normalize(resource.status) === 'offline';
  const leading = offline ? [stale ? 'Last reported offline' : 'vCenter reports it offline'] : [];
  const reasons = vmwareAttentionReasons(resource, leading, staleNotice);
  return {
    tone: offline || hasCriticalVmwareSignal(resource) ? 'danger' : 'warning',
    label: 'Attention',
    reasons: reasons.length > 0 ? reasons : ['vCenter reports a problem'],
    stale,
  };
}

export const formatVmwareIdentifierLabel = (value: string): string =>
  value
    .trim()
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
  return code ? formatVmwareIdentifierLabel(code.replace(/^vmware_/, '')) : 'vSphere Health';
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

  return rows.sort(
    (a, b) =>
      incidentSeverityRank(b.severity) - incidentSeverityRank(a.severity) ||
      b.priority - a.priority ||
      a.entityType.localeCompare(b.entityType) ||
      a.resourceName.localeCompare(b.resourceName),
  );
}

export type VmwareHealthPosture = {
  critical: number;
  warning: number;
  info: number;
  attention: number;
  affectedResources: number;
};

export function buildVmwareHealthPosture(
  incidents: readonly VmwareIncidentRow[],
): VmwareHealthPosture {
  const posture: VmwareHealthPosture = {
    critical: 0,
    warning: 0,
    info: 0,
    attention: 0,
    affectedResources: 0,
  };
  const affectedResources = new Set<string>();
  for (const incident of incidents) {
    posture[incident.severityBucket] += 1;
    if (incident.severityBucket === 'critical' || incident.severityBucket === 'warning') {
      posture.attention += 1;
      affectedResources.add(incident.resourceId);
    }
  }
  posture.affectedResources = affectedResources.size;
  return posture;
}

const VMWARE_TASK_KEYS = ['vmwareTask', 'vmwareTaskName', 'vmwareTaskState'] as const;
const VMWARE_EVENT_KEYS = ['vmwareEvent', 'vmwareEventType', 'vmwareEventMessage'] as const;

const isVmwareActivityChange = (change: ResourceChange): boolean =>
  change.kind === 'activity' &&
  (trimString(change.sourceAdapter) === 'vmware_adapter' ||
    metadataString(change.metadata, 'activity_type').startsWith('vmware_') ||
    Boolean(metadataString(change.metadata, ...VMWARE_TASK_KEYS, ...VMWARE_EVENT_KEYS)));

const mapVmwareActivityKind = (
  activityType: string,
  metadata: Record<string, unknown> | undefined,
): VmwareActivityKind => {
  const normalized = normalizeToken(activityType);
  if (normalized.includes('task') || metadataString(metadata, ...VMWARE_TASK_KEYS)) return 'task';
  if (normalized.includes('event') || metadataString(metadata, ...VMWARE_EVENT_KEYS)) {
    return 'event';
  }
  return 'activity';
};

export function mapVmwareActivityStateBucket(state: string): VmwareActivityStateBucket {
  const normalized = normalizeToken(state);
  if (/^(error|fail(ed|ure)|cancell?ed|timed?out|red)$/.test(normalized)) return 'failed';
  if (/^(running|queued|pending|inprogress|started)$/.test(normalized)) return 'running';
  if (/^(succe(ss|eded)|completed?|ok|green)$/.test(normalized)) return 'success';
  return 'unknown';
}

const parseActivitySortTime = (change: ResourceChange): number => {
  const value = trimString(change.occurredAt) || trimString(change.observedAt);
  const parsed = value ? new Date(value).getTime() : Number.NaN;
  return Number.isFinite(parsed) ? parsed : 0;
};

const appendSourceAliasKeys = (
  keys: Set<string>,
  connectionId: unknown,
  entityType: unknown,
  managedObjectId: unknown,
  includeStorageAlias: boolean,
): void => {
  const sourceAlias = vmwareSourceAlias(connectionId, entityType, managedObjectId);
  addLookupKey(keys, sourceAlias);
  addLookupKey(keys, managedObjectId);
  if (includeStorageAlias && sourceAlias) addLookupKey(keys, `storage:${sourceAlias}`);
};

const vmwareActivityResourceKeys = (resource: Resource): Set<string> => {
  const keys = new Set<string>();
  addLookupKey(keys, resource.id);
  addLookupKey(keys, resource.canonicalIdentity?.primaryId);
  for (const alias of resource.canonicalIdentity?.aliases ?? []) addLookupKey(keys, alias);
  appendSourceAliasKeys(
    keys,
    resource.vmware?.connectionId,
    resource.vmware?.entityType,
    resource.vmware?.managedObjectId,
    resource.type === 'storage',
  );
  return keys;
};

const vmwareActivityChangeKeys = (change: ResourceChange): Set<string> => {
  const keys = new Set<string>();
  addLookupKey(keys, change.resourceId);
  const metadata = change.metadata;
  appendSourceAliasKeys(
    keys,
    metadataString(metadata, 'vmwareConnectionId'),
    metadataString(metadata, 'vmwareEntityType'),
    metadataString(metadata, 'vmwareManagedObjectId'),
    true,
  );
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
  const m = (...keys: string[]) => metadataString(change.metadata, ...keys);
  const activityType = m('activity_type');
  const activityKind = mapVmwareActivityKind(activityType, change.metadata);
  // An event reads as vCenter's own message. The provider titles new events
  // with it; records stored before that carry the event class as their title.
  const title =
    (activityKind === 'event' ? m('vmwareEventMessage') : '') ||
    m('activity_title', 'vmwareTaskName', 'vmwareEventType') ||
    trimString(change.reason) ||
    'vSphere activity';
  const state = m('activity_state', 'vmwareTaskState') || trimString(change.to);
  return {
    id: `${resource.id}:activity:${change.id || index}`,
    resource,
    change,
    resourceId: resource.id,
    resourceName: vmwareResourceDisplayName(resource),
    resourceType: resource.type,
    entityType: m('vmwareEntityType') || trimString(resource.vmware?.entityType) || resource.type,
    managedObjectId:
      m('vmwareManagedObjectId') || trimString(resource.vmware?.managedObjectId) || resource.id,
    activityKind,
    activityType,
    stateBucket: mapVmwareActivityStateBucket(state),
    title,
    eventType: m('vmwareEventType'),
    state,
    message:
      m('activity_message', 'vmwareTaskError', 'vmwareEventMessage') || trimString(change.reason),
    description: m('vmwareTaskDescription', 'vmwareEventMessage'),
    actor: trimString(change.actor) || m('vmwareEventUser'),
    nativeId: m('activity_native_id', 'vmwareTask', 'vmwareEvent') || change.id,
    source: trimString(change.sourceAdapter) || trimString(change.sourceType) || 'vmware',
    occurredAt: trimString(change.occurredAt) || undefined,
    observedAt: trimString(change.observedAt),
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

  return rows.sort(
    (left, right) =>
      right.sortTime - left.sortTime ||
      left.resourceName.localeCompare(right.resourceName) ||
      left.id.localeCompare(right.id),
  );
}

const buildSearchHaystack = (parts: readonly unknown[]): string =>
  parts
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

const commonResourceSearchFields = (resource: Resource): readonly unknown[] => [
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
  resource.vmware?.overallStatus,
  ...(resource.tags ?? []),
];

const filterBySearch = <T>(
  items: T[],
  search: string,
  matchesStatus: (item: T) => boolean,
  haystack: (item: T) => string,
): T[] => {
  const needle = normalize(search);
  return items.filter(
    (item) => matchesStatus(item) && (!needle || haystack(item).includes(needle)),
  );
};

const vmwareDatastoreSearchHaystack = (resource: Resource): string =>
  buildSearchHaystack([
    ...commonResourceSearchFields(resource),
    resource.vmware?.datastoreType,
    resource.vmware?.datastoreUrl,
    resource.vmware?.maintenanceMode,
    resource.storage?.type,
    resource.storage?.topology,
    resource.storage?.platform,
    resource.storage?.nodes?.join(' '),
    resource.storage?.consumerTypes?.join(' '),
    resource.storage?.topConsumers?.map((consumer) => consumer.name).join(' '),
  ]);

export function filterVmwareDatastores(
  datastores: Resource[],
  search: string,
  status: VmwareDatastoreStatusFilter,
): Resource[] {
  return filterBySearch(
    datastores,
    search,
    (datastore) => status === 'all' || mapVmwareDatastoreStatus(datastore) === status,
    vmwareDatastoreSearchHaystack,
  );
}

const vmwareNetworkSearchHaystack = (resource: Resource): string =>
  buildSearchHaystack([
    ...commonResourceSearchFields(resource),
    resource.vmware?.networkType,
    resource.vmware?.networkHostNames?.join(' '),
    resource.vmware?.networkVmNames?.join(' '),
  ]);

export function filterVmwareNetworks(
  networks: Resource[],
  search: string,
  status: VmwareNetworkStatusFilter,
): Resource[] {
  return filterBySearch(
    networks,
    search,
    (network) => status === 'all' || mapVmwareNetworkStatus(network) === status,
    vmwareNetworkSearchHaystack,
  );
}

const vmwareVirtualMachineSearchHaystack = (resource: Resource): string =>
  buildSearchHaystack([
    ...commonResourceSearchFields(resource),
    resource.vmware?.clusterName,
    formatVmwareClusterServices(resource.vmware),
    resource.vmware?.computeResourceName,
    resource.vmware?.resourcePoolName,
    resource.vmware?.runtimeHostName,
    resource.vmware?.powerState,
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
    resource.vmware?.tools?.runState,
    resource.vmware?.tools?.versionStatus,
    resource.vmware?.tools?.version,
    resource.vmware?.tools?.installType,
    resource.vmware?.tools?.upgradePolicy,
    resource.vmware?.tools?.errorMessage,
    resource.vmware?.tools?.guestRebootComponents?.join(' '),
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
    resource.vmware?.activeAlarmSummary,
    resource.vmware?.recentTaskSummary,
  ]);

export function filterVmwareVirtualMachines(
  vms: Resource[],
  search: string,
  status: VmwareVirtualMachineStatusFilter,
): Resource[] {
  return filterBySearch(
    vms,
    search,
    (vm) => status === 'all' || mapVmwareVirtualMachineStatus(vm) === status,
    vmwareVirtualMachineSearchHaystack,
  );
}

const commonRowSearchFields = (row: VmwareIncidentRow | VmwareActivityRow): readonly unknown[] => {
  const staleNotice = getPlatformResourceStaleNotice('vmware', row.resource);
  return [
    row.resourceId,
    row.resourceName,
    row.resourceType,
    row.entityType,
    row.managedObjectId,
    row.source,
    row.resource.vmware?.connectionName,
    row.resource.vmware?.vcenterHost,
    row.resource.vmware?.datacenterName,
    row.resource.vmware?.clusterName,
    row.resource.vmware?.runtimeHostName,
    staleNotice?.label,
    staleNotice?.detail,
    ...(row.resource.tags ?? []),
  ];
};

const incidentSearchHaystack = (row: VmwareIncidentRow): string =>
  buildSearchHaystack([
    ...commonRowSearchFields(row),
    row.severity,
    row.code,
    row.summary,
    row.label,
    row.category,
    row.action,
    row.resource.vmware?.activeAlarmSummary,
  ]);

export function filterVmwareIncidents(
  incidents: VmwareIncidentRow[],
  search: string,
  severity: VmwareIncidentSeverityFilter,
): VmwareIncidentRow[] {
  return filterBySearch(
    incidents,
    search,
    (incident) =>
      severity === 'all' ||
      (severity === 'attention'
        ? incident.severityBucket === 'critical' || incident.severityBucket === 'warning'
        : incident.severityBucket === severity),
    incidentSearchHaystack,
  );
}

const activitySearchHaystack = (row: VmwareActivityRow): string =>
  buildSearchHaystack([
    row.id,
    ...commonRowSearchFields(row),
    row.activityKind,
    row.activityType,
    row.title,
    row.eventType,
    row.state,
    row.message,
    row.description,
    row.actor,
    row.nativeId,
    ...(row.change.relatedResources ?? []),
  ]);

export function filterVmwareActivity(
  activity: VmwareActivityRow[],
  search: string,
  status: VmwareActivityStatusFilter,
): VmwareActivityRow[] {
  return filterBySearch(
    activity,
    search,
    (row) =>
      status === 'all' ||
      (status === 'tasks' && row.activityKind === 'task') ||
      (status === 'events' && row.activityKind === 'event') ||
      (status === 'failed' && row.stateBucket === 'failed'),
    activitySearchHaystack,
  );
}
