import type { ResourceType, ResourceVMwareMeta } from '@/types/resource';

export type ResourceDetailDrawerVMwareRowTone = 'default' | 'accent' | 'warning';

export type ResourceDetailDrawerVMwareRow = {
  label: string;
  value: string;
  tone?: ResourceDetailDrawerVMwareRowTone;
};

export type ResourceDetailDrawerVMwareSection = {
  id: 'state' | 'placement' | 'guest' | 'signals';
  label: string;
  rows: ResourceDetailDrawerVMwareRow[];
};

const asTrimmedString = (value?: string | null): string => (value || '').trim();

const formatCount = (count: number, label: string): string =>
  `${count} ${label}${count === 1 ? '' : 's'}`;

const formatBoolLabel = (value?: boolean): string => {
  if (value === undefined) return '';
  return value ? 'Yes' : 'No';
};

const vmwareEntityLabel = (entityType?: string): string => {
  const normalized = asTrimmedString(entityType).toLowerCase();
  switch (normalized) {
    case 'host':
    case 'hostsystem':
      return 'Host';
    case 'vm':
    case 'virtualmachine':
      return 'VM';
    case 'datastore':
      return 'Datastore';
    default:
      return asTrimmedString(entityType);
  }
};

const buildSignalValue = (count: number | undefined, label: string, summary?: string): string => {
  const parts: string[] = [];
  if (typeof count === 'number') {
    parts.push(formatCount(Math.max(0, count), label));
  }
  const trimmedSummary = asTrimmedString(summary);
  if (trimmedSummary) {
    parts.push(trimmedSummary);
  }
  return parts.join(' · ');
};

const hasRows = (rows: ResourceDetailDrawerVMwareRow[]): boolean => rows.length > 0;

export const buildVMwareDetailsSummary = (
  resourceType: ResourceType,
  vmware?: ResourceVMwareMeta,
): string | null => {
  if (!vmware) return null;

  const parts: string[] = [];
  const connection = asTrimmedString(vmware.connectionName) || asTrimmedString(vmware.vcenterHost);
  if (connection) {
    parts.push(connection);
  }
  parts.push('Read-only vCenter context');

  if (resourceType === 'vm' && typeof vmware.snapshotCount === 'number') {
    parts.push(formatCount(Math.max(0, vmware.snapshotCount), 'snapshot'));
  }
  if ((vmware.activeAlarmCount ?? 0) > 0) {
    parts.push(formatCount(vmware.activeAlarmCount ?? 0, 'alarm'));
  }
  if ((vmware.recentTaskCount ?? 0) > 0) {
    parts.push(formatCount(vmware.recentTaskCount ?? 0, 'task'));
  }

  return parts.join(' · ');
};

export const buildVMwareDetailSections = (
  resourceType: ResourceType,
  vmware?: ResourceVMwareMeta,
): ResourceDetailDrawerVMwareSection[] => {
  if (!vmware) {
    return [];
  }

  const stateRows: ResourceDetailDrawerVMwareRow[] = [
    {
      label: 'Connection',
      value: asTrimmedString(vmware.connectionName),
    },
    {
      label: 'vCenter',
      value: asTrimmedString(vmware.vcenterHost),
    },
    {
      label: 'Entity',
      value: vmwareEntityLabel(vmware.entityType),
    },
    {
      label: 'Overall status',
      value: asTrimmedString(vmware.overallStatus),
      tone:
        asTrimmedString(vmware.overallStatus).toLowerCase() === 'red'
          ? 'warning'
          : asTrimmedString(vmware.overallStatus)
              ? 'accent'
              : 'default',
    },
    {
      label: 'Power',
      value: asTrimmedString(vmware.powerState),
    },
    {
      label: 'Connection state',
      value: asTrimmedString(vmware.connectionState),
    },
    {
      label: 'Datastore type',
      value: asTrimmedString(vmware.datastoreType),
    },
    {
      label: 'Accessible',
      value: formatBoolLabel(vmware.datastoreAccessible),
      tone: vmware.datastoreAccessible === false ? 'warning' : 'default',
    },
    {
      label: 'Shared access',
      value: formatBoolLabel(vmware.multipleHostAccess),
    },
    {
      label: 'Maintenance',
      value: asTrimmedString(vmware.maintenanceMode),
      tone: asTrimmedString(vmware.maintenanceMode) ? 'warning' : 'default',
    },
  ].filter((row) => row.value);

  const placementRows: ResourceDetailDrawerVMwareRow[] = [
    {
      label: 'Datacenter',
      value: asTrimmedString(vmware.datacenterName),
    },
    {
      label: 'Cluster',
      value: asTrimmedString(vmware.clusterName),
    },
    {
      label: 'Compute resource',
      value: asTrimmedString(vmware.computeResourceName),
    },
    {
      label: 'Folder',
      value: asTrimmedString(vmware.folderName),
    },
    {
      label: 'Resource pool',
      value: asTrimmedString(vmware.resourcePoolName),
    },
    {
      label: 'Runtime host',
      value: asTrimmedString(vmware.runtimeHostName),
    },
    {
      label: 'Datastores',
      value: (vmware.datastoreNames ?? []).filter(Boolean).join(', '),
    },
  ].filter((row) => row.value);

  const guestRows: ResourceDetailDrawerVMwareRow[] = [
    {
      label: 'Host UUID',
      value: asTrimmedString(vmware.hostUuid),
    },
    {
      label: 'Instance UUID',
      value: asTrimmedString(vmware.instanceUuid),
    },
    {
      label: 'BIOS UUID',
      value: asTrimmedString(vmware.biosUuid),
    },
    {
      label: 'Guest OS',
      value: asTrimmedString(vmware.guestOsFamily),
    },
    {
      label: 'Guest hostname',
      value: asTrimmedString(vmware.guestHostname),
    },
    {
      label: 'Guest IPs',
      value: (vmware.guestIpAddresses ?? []).filter(Boolean).join(', '),
    },
    {
      label: 'Datastore URL',
      value: asTrimmedString(vmware.datastoreUrl),
    },
  ].filter((row) => row.value);

  const signalRows: ResourceDetailDrawerVMwareRow[] = [
    {
      label: 'Alarms',
      value: buildSignalValue(vmware.activeAlarmCount, 'alarm', vmware.activeAlarmSummary),
      tone: (vmware.activeAlarmCount ?? 0) > 0 ? 'warning' : 'default',
    },
    {
      label: 'Tasks',
      value: buildSignalValue(vmware.recentTaskCount, 'task', vmware.recentTaskSummary),
      tone: (vmware.recentTaskCount ?? 0) > 0 ? 'accent' : 'default',
    },
    {
      label: 'Snapshots',
      value:
        resourceType === 'vm' || typeof vmware.snapshotCount === 'number'
          ? formatCount(Math.max(0, vmware.snapshotCount ?? 0), 'snapshot')
          : '',
    },
  ].filter((row) => row.value);

  const sections: ResourceDetailDrawerVMwareSection[] = [
    { id: 'state', label: 'State', rows: stateRows },
    { id: 'placement', label: 'Placement', rows: placementRows },
    { id: 'guest', label: 'Guest', rows: guestRows },
    { id: 'signals', label: 'Signals', rows: signalRows },
  ];

  return sections.filter((section) => hasRows(section.rows));
};
