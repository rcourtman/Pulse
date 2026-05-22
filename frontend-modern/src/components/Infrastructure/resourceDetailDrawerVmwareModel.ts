import type {
  ResourceType,
  ResourceVMwareMeta,
  ResourceVMwareNetworkAdapter,
  ResourceVMwareSnapshot,
  ResourceVMwareVirtualDisk,
} from '@/types/resource';

export type ResourceDetailDrawerVMwareRowTone = 'default' | 'accent' | 'warning';

export type ResourceDetailDrawerVMwareRow = {
  label: string;
  value: string;
  tone?: ResourceDetailDrawerVMwareRowTone;
};

export type ResourceDetailDrawerVMwareSection = {
  id: 'state' | 'placement' | 'guest' | 'disks' | 'network' | 'signals' | 'snapshots';
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

const formatCapacityBytes = (value?: number): string => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  const precision = unitIndex === 0 || size >= 10 ? 0 : 1;
  return `${size.toFixed(precision)} ${units[unitIndex]}`;
};

const countSnapshotTree = (snapshots?: ResourceVMwareSnapshot[]): number =>
  (snapshots ?? []).reduce(
    (total, snapshot) => total + 1 + countSnapshotTree(snapshot.children),
    0,
  );

const snapshotDisplayName = (snapshot: ResourceVMwareSnapshot): string =>
  asTrimmedString(snapshot.name) ||
  asTrimmedString(snapshot.snapshot) ||
  (typeof snapshot.id === 'number' ? `Snapshot ${snapshot.id}` : 'Snapshot');

const formatSnapshotDate = (value?: string | number): string => {
  if (value === undefined || value === null || value === '') return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return asTrimmedString(String(value));
  const pad = (part: number) => String(part).padStart(2, '0');
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(
    date.getUTCDate(),
  )} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())} UTC`;
};

const snapshotValue = (snapshot: ResourceVMwareSnapshot): string => {
  const parts = [
    snapshot.current ? 'current' : '',
    asTrimmedString(snapshot.state),
    formatSnapshotDate(snapshot.createdAt),
    snapshot.quiesced === undefined ? '' : snapshot.quiesced ? 'quiesced' : 'not quiesced',
    asTrimmedString(snapshot.description),
  ].filter(Boolean);
  return parts.join(' · ');
};

const flattenSnapshotRows = (
  snapshots: ResourceVMwareSnapshot[] | undefined,
  depth = 0,
): ResourceDetailDrawerVMwareRow[] => {
  const rows: ResourceDetailDrawerVMwareRow[] = [];
  for (const snapshot of snapshots ?? []) {
    rows.push({
      label: `${depth > 0 ? `${'-'.repeat(depth)} ` : ''}${snapshotDisplayName(snapshot)}`,
      value: snapshotValue(snapshot) || asTrimmedString(snapshot.snapshot),
      tone: snapshot.current ? 'accent' : 'default',
    });
    rows.push(...flattenSnapshotRows(snapshot.children, depth + 1));
  }
  return rows;
};

const adapterDisplayName = (adapter: ResourceVMwareNetworkAdapter): string =>
  asTrimmedString(adapter.label) ||
  asTrimmedString(adapter.nic) ||
  asTrimmedString(adapter.macAddress) ||
  'Network adapter';

const adapterNetworkName = (adapter: ResourceVMwareNetworkAdapter): string =>
  asTrimmedString(adapter.networkName) ||
  asTrimmedString(adapter.networkId) ||
  asTrimmedString(adapter.opaqueNetworkId) ||
  asTrimmedString(adapter.hostDevice) ||
  asTrimmedString(adapter.backingType);

const adapterConnectionLabel = (adapter: ResourceVMwareNetworkAdapter): string => {
  const parts = [
    asTrimmedString(adapter.state),
    adapter.startConnected === undefined
      ? ''
      : adapter.startConnected
        ? 'starts connected'
        : 'does not start connected',
    adapter.allowGuestControl === undefined
      ? ''
      : adapter.allowGuestControl
        ? 'guest control'
        : 'no guest control',
  ].filter(Boolean);
  return parts.join(' · ');
};

const adapterValue = (adapter: ResourceVMwareNetworkAdapter): string => {
  const parts = [
    asTrimmedString(adapter.type),
    adapterNetworkName(adapter),
    asTrimmedString(adapter.macAddress),
    adapterConnectionLabel(adapter),
  ].filter(Boolean);
  return parts.join(' · ');
};

const adapterTone = (adapter: ResourceVMwareNetworkAdapter): ResourceDetailDrawerVMwareRowTone =>
  asTrimmedString(adapter.state).toLowerCase() === 'not_connected' ? 'warning' : 'default';

const networkAdapterRows = (
  adapters: ResourceVMwareNetworkAdapter[] | undefined,
): ResourceDetailDrawerVMwareRow[] =>
  (adapters ?? [])
    .map((adapter) => ({
      label: adapterDisplayName(adapter),
      value: adapterValue(adapter),
      tone: adapterTone(adapter),
    }))
    .filter((row) => row.value);

const virtualDiskDisplayName = (disk: ResourceVMwareVirtualDisk): string =>
  asTrimmedString(disk.label) || asTrimmedString(disk.disk) || 'Virtual disk';

const formatVirtualDiskAddress = (disk: ResourceVMwareVirtualDisk): string => {
  const type = asTrimmedString(disk.type).toUpperCase();
  if (type === 'SCSI' && disk.scsiBus !== undefined && disk.scsiUnit !== undefined) {
    return `SCSI ${disk.scsiBus}:${disk.scsiUnit}`;
  }
  if (type === 'SATA' && disk.sataBus !== undefined && disk.sataUnit !== undefined) {
    return `SATA ${disk.sataBus}:${disk.sataUnit}`;
  }
  if (type === 'NVME' && disk.nvmeBus !== undefined && disk.nvmeUnit !== undefined) {
    return `NVMe ${disk.nvmeBus}:${disk.nvmeUnit}`;
  }
  if (type === 'IDE' && disk.idePrimary !== undefined && disk.ideMaster !== undefined) {
    return `IDE ${disk.idePrimary ? 'primary' : 'secondary'} ${
      disk.ideMaster ? 'master' : 'slave'
    }`;
  }
  return type;
};

const virtualDiskValue = (disk: ResourceVMwareVirtualDisk): string => {
  const parts = [
    formatVirtualDiskAddress(disk),
    formatCapacityBytes(disk.capacityBytes),
    asTrimmedString(disk.datastoreName),
    asTrimmedString(disk.backingType),
    asTrimmedString(disk.vmdkFile),
  ].filter(Boolean);
  return parts.join(' · ');
};

const virtualDiskRows = (
  disks: ResourceVMwareVirtualDisk[] | undefined,
): ResourceDetailDrawerVMwareRow[] =>
  (disks ?? [])
    .map((disk) => ({
      label: virtualDiskDisplayName(disk),
      value: virtualDiskValue(disk),
      tone: 'default' as ResourceDetailDrawerVMwareRowTone,
    }))
    .filter((row) => row.value);

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

const filterNonEmptyRows = (
  rows: ResourceDetailDrawerVMwareRow[],
): ResourceDetailDrawerVMwareRow[] => rows.filter((row) => row.value);

const getStatusTone = (status?: string | null): ResourceDetailDrawerVMwareRowTone => {
  const normalized = asTrimmedString(status).toLowerCase();
  if (normalized === 'red') return 'warning';
  if (normalized) return 'accent';
  return 'default';
};

const getWarningTone = (hasWarning: boolean): ResourceDetailDrawerVMwareRowTone =>
  hasWarning ? 'warning' : 'default';

const getAccentTone = (hasAccent: boolean): ResourceDetailDrawerVMwareRowTone =>
  hasAccent ? 'accent' : 'default';

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

  const snapshotCount =
    typeof vmware.snapshotCount === 'number'
      ? Math.max(0, vmware.snapshotCount)
      : countSnapshotTree(vmware.snapshotTree);
  if (resourceType === 'vm' && snapshotCount > 0) {
    parts.push(formatCount(snapshotCount, 'snapshot'));
  }
  const networkAdapterCount = vmware.networkAdapters?.length ?? 0;
  if (resourceType === 'vm' && networkAdapterCount > 0) {
    parts.push(formatCount(networkAdapterCount, 'vNIC'));
  }
  const virtualDiskCount = vmware.virtualDisks?.length ?? 0;
  if (resourceType === 'vm' && virtualDiskCount > 0) {
    parts.push(formatCount(virtualDiskCount, 'disk'));
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

  const stateRows = filterNonEmptyRows([
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
      tone: getStatusTone(vmware.overallStatus),
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
      tone: getWarningTone(vmware.datastoreAccessible === false),
    },
    {
      label: 'Shared access',
      value: formatBoolLabel(vmware.multipleHostAccess),
    },
    {
      label: 'Maintenance',
      value: asTrimmedString(vmware.maintenanceMode),
      tone: getWarningTone(Boolean(asTrimmedString(vmware.maintenanceMode))),
    },
  ]);

  const placementRows = filterNonEmptyRows([
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
  ]);

  const guestRows = filterNonEmptyRows([
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
  ]);

  const networkRows = resourceType === 'vm' ? networkAdapterRows(vmware.networkAdapters) : [];
  const diskRows = resourceType === 'vm' ? virtualDiskRows(vmware.virtualDisks) : [];

  const signalRows = filterNonEmptyRows([
    {
      label: 'Alarms',
      value: buildSignalValue(vmware.activeAlarmCount, 'alarm', vmware.activeAlarmSummary),
      tone: getWarningTone((vmware.activeAlarmCount ?? 0) > 0),
    },
    {
      label: 'Tasks',
      value: buildSignalValue(vmware.recentTaskCount, 'task', vmware.recentTaskSummary),
      tone: getAccentTone((vmware.recentTaskCount ?? 0) > 0),
    },
    {
      label: 'Snapshots',
      value:
        resourceType === 'vm' || typeof vmware.snapshotCount === 'number'
          ? formatCount(
              Math.max(
                0,
                typeof vmware.snapshotCount === 'number'
                  ? vmware.snapshotCount
                  : countSnapshotTree(vmware.snapshotTree),
              ),
              'snapshot',
            )
          : '',
    },
  ]);

  const snapshotRows = resourceType === 'vm' ? flattenSnapshotRows(vmware.snapshotTree) : [];

  const sections: ResourceDetailDrawerVMwareSection[] = [
    { id: 'state', label: 'State', rows: stateRows },
    { id: 'placement', label: 'Placement', rows: placementRows },
    { id: 'guest', label: 'Guest', rows: guestRows },
    { id: 'disks', label: 'Virtual disks', rows: diskRows },
    { id: 'network', label: 'Network', rows: networkRows },
    { id: 'signals', label: 'Signals', rows: signalRows },
    { id: 'snapshots', label: 'Snapshot tree', rows: snapshotRows },
  ];

  return sections.filter((section) => hasRows(section.rows));
};
