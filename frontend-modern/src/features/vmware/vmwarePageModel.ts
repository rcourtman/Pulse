import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import type { Resource, ResourceIncident, ResourceType } from '@/types/resource';

export type VmwarePageTabId = 'overview' | 'storage';
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
export type VmwareIncidentSeverityFilter = 'all' | 'critical' | 'warning' | 'info';

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
] as const;

const VMWARE_RESOURCE_TYPES = new Set<ResourceType>(['agent', 'vm', 'storage']);

const isVmwarePlatform = (resource: Resource): boolean =>
  resolveResourcePlatformType(resource) === 'vmware-vsphere';

export type VmwarePageModel = {
  resources: Resource[];
  hosts: Resource[];
  vms: Resource[];
  datastores: Resource[];
  incidents: VmwareIncidentRow[];
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

export function buildVmwarePageModel(resources: Resource[]): VmwarePageModel {
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
  const incidents = buildVmwareIncidentRows(vmwareResources);

  return {
    resources: vmwareResources,
    hosts,
    vms,
    datastores,
    incidents,
  };
}

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

const trimString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const normalizeToken = (value: unknown): string => normalize(value).replace(/[\s_-]/g, '');

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
