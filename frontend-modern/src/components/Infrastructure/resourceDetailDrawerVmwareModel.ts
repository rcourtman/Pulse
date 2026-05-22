import type {
  Resource,
  ResourceType,
  ResourceVMwareBootDevice,
  ResourceVMwareHardware,
  ResourceVMwareMeta,
  ResourceVMwareNetworkAdapter,
  ResourceVMwareSnapshot,
  ResourceVMwareTools,
  ResourceVMwareVirtualDisk,
} from '@/types/resource';
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';

export type ResourceDetailDrawerVMwareRowTone = 'default' | 'accent' | 'warning';

export type ResourceDetailDrawerVMwareRow = {
  label: string;
  value: string;
  tone?: ResourceDetailDrawerVMwareRowTone;
};

export type ResourceDetailDrawerVMwareSection = {
  id:
    | 'state'
    | 'placement'
    | 'guest'
    | 'hardware'
    | 'tools'
    | 'disks'
    | 'network'
    | 'signals'
    | 'snapshots';
  label: string;
  rows: ResourceDetailDrawerVMwareRow[];
};

const asTrimmedString = (value?: string | null): string => (value || '').trim();

const formatCount = (count: number, label: string): string =>
  `${count} ${label}${count === 1 ? '' : 's'}`;

const summarizeList = (values: string[] | undefined): string =>
  (values ?? []).map(asTrimmedString).filter(Boolean).join(', ');

const formatBoolLabel = (value?: boolean): string => {
  if (value === undefined) return '';
  return value ? 'Yes' : 'No';
};

const VMWARE_ENUM_ACRONYMS: Record<string, string> = {
  API: 'API',
  BIOS: 'BIOS',
  CPU: 'CPU',
  IP: 'IP',
  OS: 'OS',
  VM: 'VM',
  VMDK: 'VMDK',
  VMX: 'VMX',
  EFI: 'EFI',
  IPV4: 'IPv4',
  IPV6: 'IPv6',
};

const formatEnumLabel = (value?: string | null): string => {
  const normalized = asTrimmedString(value);
  if (!normalized) return '';
  return normalized
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => {
      const upper = part.toUpperCase();
      return (
        VMWARE_ENUM_ACRONYMS[upper] ?? part.charAt(0).toUpperCase() + part.slice(1).toLowerCase()
      );
    })
    .join(' ');
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

const formatMiB = (value?: number): string => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '';
  return formatCapacityBytes(value * 1024 * 1024).replace(/\.0 ([A-Z]+)/, ' $1');
};

const formatMilliseconds = (value?: number): string => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '';
  return `${value} ms`;
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

const filterNonEmptyRows = (
  rows: ResourceDetailDrawerVMwareRow[],
): ResourceDetailDrawerVMwareRow[] => rows.filter((row) => row.value);

const getWarningTone = (hasWarning: boolean): ResourceDetailDrawerVMwareRowTone =>
  hasWarning ? 'warning' : 'default';

const hardwareSummary = (hardware?: ResourceVMwareHardware): string => {
  if (!hardware) return '';
  const upgradeStatus = asTrimmedString(hardware.upgradeStatus);
  if (upgradeStatus && !['NONE', 'OK'].includes(upgradeStatus.toUpperCase())) {
    return `Hardware ${formatEnumLabel(upgradeStatus).toLowerCase()}`;
  }
  return formatEnumLabel(hardware.version);
};

const bootDeviceLabel = (device: ResourceVMwareBootDevice): string => {
  const type = formatEnumLabel(device.type);
  const details = [
    asTrimmedString(device.nic),
    ...(device.disks ?? []).map((disk) => asTrimmedString(disk)),
  ].filter(Boolean);
  return details.length > 0 ? `${type} ${details.join(', ')}` : type;
};

const bootOrderLabel = (devices?: ResourceVMwareBootDevice[]): string =>
  (devices ?? []).map(bootDeviceLabel).filter(Boolean).join(' -> ');

const cpuTopologyLabel = (vmware: ResourceVMwareMeta): string => {
  const parts = [];
  if (
    typeof vmware.cpuCount === 'number' &&
    Number.isFinite(vmware.cpuCount) &&
    vmware.cpuCount > 0
  ) {
    parts.push(`${vmware.cpuCount} vCPU`);
  }
  if (
    typeof vmware.hardware?.cpuCoresPerSocket === 'number' &&
    Number.isFinite(vmware.hardware.cpuCoresPerSocket) &&
    vmware.hardware.cpuCoresPerSocket > 0
  ) {
    parts.push(`${vmware.hardware.cpuCoresPerSocket} cores/socket`);
  }
  return parts.join(' · ');
};

const hardwareRows = (vmware?: ResourceVMwareMeta): ResourceDetailDrawerVMwareRow[] => {
  if (!vmware?.hardware) return [];
  const hardware = vmware.hardware;
  return filterNonEmptyRows([
    {
      label: 'Guest OS',
      value: formatEnumLabel(hardware.guestOs),
    },
    {
      label: 'Hardware version',
      value: formatEnumLabel(hardware.version),
    },
    {
      label: 'Upgrade status',
      value: formatEnumLabel(hardware.upgradeStatus),
      tone: getWarningTone(
        Boolean(asTrimmedString(hardware.upgradeStatus)) &&
          !['NONE', 'OK'].includes(asTrimmedString(hardware.upgradeStatus).toUpperCase()),
      ),
    },
    {
      label: 'Upgrade policy',
      value: formatEnumLabel(hardware.upgradePolicy),
    },
    {
      label: 'Upgrade target',
      value: formatEnumLabel(hardware.upgradeVersion),
    },
    {
      label: 'Upgrade error',
      value: asTrimmedString(hardware.upgradeErrorMessage),
      tone: 'warning',
    },
    {
      label: 'Instant clone frozen',
      value: formatBoolLabel(hardware.instantCloneFrozen),
      tone: getWarningTone(hardware.instantCloneFrozen === true),
    },
    {
      label: 'CPU topology',
      value: cpuTopologyLabel(vmware),
    },
    {
      label: 'CPU hot-add',
      value: formatBoolLabel(hardware.cpuHotAddEnabled),
    },
    {
      label: 'CPU hot-remove',
      value: formatBoolLabel(hardware.cpuHotRemoveEnabled),
    },
    {
      label: 'Memory size',
      value: formatMiB(vmware?.memorySizeMib),
    },
    {
      label: 'Memory hot-add',
      value: formatBoolLabel(hardware.memoryHotAddEnabled),
    },
    {
      label: 'Memory hot-add increment',
      value: formatMiB(hardware.memoryHotAddIncrementMib),
    },
    {
      label: 'Memory hot-add limit',
      value: formatMiB(hardware.memoryHotAddLimitMib),
    },
    {
      label: 'Boot type',
      value: formatEnumLabel(hardware.bootType),
    },
    {
      label: 'EFI legacy boot',
      value: formatBoolLabel(hardware.efiLegacyBoot),
    },
    {
      label: 'Boot network protocol',
      value: formatEnumLabel(hardware.bootNetworkProtocol),
    },
    {
      label: 'Boot delay',
      value: formatMilliseconds(hardware.bootDelayMilliseconds),
    },
    {
      label: 'Boot retry',
      value: formatBoolLabel(hardware.bootRetry),
    },
    {
      label: 'Boot retry delay',
      value: formatMilliseconds(hardware.bootRetryDelayMilliseconds),
    },
    {
      label: 'Enter setup mode',
      value: formatBoolLabel(hardware.enterSetupMode),
      tone: getWarningTone(hardware.enterSetupMode === true),
    },
    {
      label: 'Boot order',
      value: bootOrderLabel(hardware.bootDevices),
    },
  ]);
};

const toolsSummary = (tools?: ResourceVMwareTools): string => {
  if (!tools) return '';
  if (tools.guestRebootRequested) return 'Tools reboot requested';
  const versionStatus = asTrimmedString(tools.versionStatus);
  if (versionStatus && !['CURRENT', 'OK'].includes(versionStatus.toUpperCase())) {
    return `Tools ${formatEnumLabel(versionStatus).toLowerCase()}`;
  }
  const runState = asTrimmedString(tools.runState);
  if (runState) return `Tools ${formatEnumLabel(runState).toLowerCase()}`;
  return '';
};

const toolsRows = (tools?: ResourceVMwareTools): ResourceDetailDrawerVMwareRow[] => {
  if (!tools) return [];
  return filterNonEmptyRows([
    {
      label: 'Run state',
      value: formatEnumLabel(tools.runState),
      tone: getWarningTone(
        Boolean(asTrimmedString(tools.runState)) &&
          !['RUNNING', 'STARTED'].includes(asTrimmedString(tools.runState).toUpperCase()),
      ),
    },
    {
      label: 'Version status',
      value: formatEnumLabel(tools.versionStatus),
      tone: getWarningTone(
        Boolean(asTrimmedString(tools.versionStatus)) &&
          !['CURRENT', 'OK'].includes(asTrimmedString(tools.versionStatus).toUpperCase()),
      ),
    },
    {
      label: 'Version',
      value: asTrimmedString(tools.version),
    },
    {
      label: 'Install type',
      value: formatEnumLabel(tools.installType),
    },
    {
      label: 'Upgrade policy',
      value: formatEnumLabel(tools.upgradePolicy),
    },
    {
      label: 'Auto update supported',
      value: formatBoolLabel(tools.autoUpdateSupported),
    },
    {
      label: 'Install attempts',
      value:
        typeof tools.installAttemptCount === 'number' && Number.isFinite(tools.installAttemptCount)
          ? String(tools.installAttemptCount)
          : '',
    },
    {
      label: 'Guest reboot',
      value:
        tools.guestRebootRequested === undefined
          ? ''
          : tools.guestRebootRequested
            ? 'Requested'
            : 'Not requested',
      tone: getWarningTone(tools.guestRebootRequested === true),
    },
    {
      label: 'Reboot components',
      value: (tools.guestRebootComponents ?? []).filter(Boolean).join(', '),
      tone: getWarningTone((tools.guestRebootComponents ?? []).length > 0),
    },
    {
      label: 'Reboot requested at',
      value: formatSnapshotDate(tools.guestRebootRequestTime),
      tone: getWarningTone(Boolean(tools.guestRebootRequestTime)),
    },
    {
      label: 'Last install error',
      value: asTrimmedString(tools.errorMessage),
      tone: 'warning',
    },
  ]);
};

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
    case 'network':
      return 'Network';
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

const getStatusTone = (status?: string | null): ResourceDetailDrawerVMwareRowTone => {
  const normalized = asTrimmedString(status).toLowerCase();
  if (normalized === 'red') return 'warning';
  if (normalized) return 'accent';
  return 'default';
};

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
  if (resourceType === 'network') {
    const hostCount = vmware.networkHostNames?.length ?? vmware.networkHostIds?.length ?? 0;
    const vmCount = vmware.networkVmNames?.length ?? vmware.networkVmIds?.length ?? 0;
    if (hostCount > 0) parts.push(formatCount(hostCount, 'host'));
    if (vmCount > 0) parts.push(formatCount(vmCount, 'VM'));
  }
  const hardware = resourceType === 'vm' ? hardwareSummary(vmware.hardware) : '';
  if (hardware) {
    parts.push(hardware);
  }
  const tools = resourceType === 'vm' ? toolsSummary(vmware.tools) : '';
  if (tools) {
    parts.push(tools);
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
    {
      label: 'Network type',
      value: formatEnumLabel(vmware.networkType),
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
      label: 'Cluster services',
      value: formatVmwareClusterServices(vmware),
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

  const networkRows =
    resourceType === 'vm'
      ? networkAdapterRows(vmware.networkAdapters)
      : filterNonEmptyRows([
          {
            label: 'Hosts',
            value: summarizeList(vmware.networkHostNames),
          },
          {
            label: 'VMs',
            value: summarizeList(vmware.networkVmNames),
          },
        ]);
  const vmwareHardwareRows = resourceType === 'vm' ? hardwareRows(vmware) : [];
  const vmwareToolsRows = resourceType === 'vm' ? toolsRows(vmware.tools) : [];
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
    { id: 'hardware', label: 'Virtual hardware', rows: vmwareHardwareRows },
    { id: 'tools', label: 'VMware Tools', rows: vmwareToolsRows },
    { id: 'disks', label: 'Virtual disks', rows: diskRows },
    { id: 'network', label: 'Network', rows: networkRows },
    { id: 'signals', label: 'Signals', rows: signalRows },
    { id: 'snapshots', label: 'Snapshot tree', rows: snapshotRows },
  ];

  return sections.filter((section) => hasRows(section.rows));
};

export const hasVMwareDetailSections = (resource: Resource): boolean =>
  buildVMwareDetailSections(resource.type, resource.vmware).length > 0;
