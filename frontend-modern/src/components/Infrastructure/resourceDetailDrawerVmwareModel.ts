import type {
  ResourceType,
  ResourceVMwareHardware,
  ResourceVMwareMeta,
  ResourceVMwareNetworkAdapter,
  ResourceVMwareSnapshot,
  ResourceVMwareTools,
  ResourceVMwareVirtualDisk,
} from '@/types/resource';
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';
import {
  compactDetailRows as compactRows,
  compactDetailSections as compactSections,
  type DetailRow,
  type DetailSection,
  type DetailValueTone,
  formatDetailBytesValue,
  formatDetailCountValue,
  makeDetailRow as makeRow,
} from '@/components/shared/detailSectionModel';

const asTrimmedString = (value?: string | null): string => (value || '').trim();

// Names, addresses and paths an operator copies into vCenter or a shell wrap
// in full instead of ending in an ellipsis.
const wrapRow = (
  label: string,
  value?: string | null,
  options: Parameters<typeof makeRow>[2] = {},
) => makeRow(label, value, { ...options, wrap: true });

// Tools and vNIC state only mean something while the VM runs: a powered-off
// VM has no running Tools and no connected adapters by design. Without a
// reported power state the readings are taken at face value.
const isPoweredOff = (vmware?: ResourceVMwareMeta): boolean => {
  const state = asTrimmedString(vmware?.powerState)
    .toLowerCase()
    .replace(/[^a-z]/g, '');
  return state === 'poweredoff' || state === 'suspended' || state === 'off';
};

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

const formatMiB = (value?: number): string => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '';
  return (
    formatDetailBytesValue(value * 1024 * 1024, {
      allowZero: true,
      precision: 'compact',
      trimWhole: true,
    }) ?? ''
  );
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
): DetailRow[] => {
  const rows: DetailRow[] = [];
  for (const snapshot of snapshots ?? []) {
    const snapshotRow = wrapRow(
      `${depth > 0 ? `${'-'.repeat(depth)} ` : ''}${snapshotDisplayName(snapshot)}`,
      snapshotValue(snapshot) || asTrimmedString(snapshot.snapshot),
      { tone: snapshot.current ? 'accent' : 'default' },
    );
    if (snapshotRow) rows.push(snapshotRow);
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

const adapterTone = (adapter: ResourceVMwareNetworkAdapter): DetailValueTone =>
  asTrimmedString(adapter.state).toLowerCase() === 'not_connected' ? 'warning' : 'default';

const networkAdapterRows = (
  adapters: ResourceVMwareNetworkAdapter[] | undefined,
  poweredOff: boolean,
): DetailRow[] =>
  compactRows(
    (adapters ?? []).map((adapter) =>
      wrapRow(adapterDisplayName(adapter), adapterValue(adapter), {
        tone: poweredOff ? 'default' : adapterTone(adapter),
      }),
    ),
  );

const getWarningTone = (hasWarning: boolean): DetailValueTone =>
  hasWarning ? 'warning' : 'default';

const hardwareSummary = (hardware?: ResourceVMwareHardware): string => {
  if (!hardware) return '';
  const upgradeStatus = asTrimmedString(hardware.upgradeStatus);
  if (upgradeStatus && !['NONE', 'OK'].includes(upgradeStatus.toUpperCase())) {
    return `Hardware ${formatEnumLabel(upgradeStatus).toLowerCase()}`;
  }
  return formatEnumLabel(hardware.version);
};

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

const hardwareRows = (vmware?: ResourceVMwareMeta): DetailRow[] => {
  if (!vmware?.hardware) return [];
  const hardware = vmware.hardware;
  const upgradeStatus = asTrimmedString(hardware.upgradeStatus).toUpperCase();
  const upgradeAttention = Boolean(upgradeStatus) && !['NONE', 'OK'].includes(upgradeStatus);
  return compactRows([
    makeRow('Guest OS', formatEnumLabel(hardware.guestOs)),
    makeRow('Hardware version', formatEnumLabel(hardware.version)),
    makeRow('CPU topology', cpuTopologyLabel(vmware)),
    makeRow('Memory size', formatMiB(vmware?.memorySizeMib)),
    // Upgrade and clone status only surface when something is actionable;
    // when everything is in the default state these rows resolve to empty
    // strings and compactRows drops them. Capability toggles
    // (CPU/memory hot-add, boot config) live in the raw API and stay out
    // of the operator drawer because they don't change minute to minute
    // and operators don't act on them from Pulse.
    makeRow('Upgrade status', upgradeAttention ? formatEnumLabel(hardware.upgradeStatus) : '', {
      tone: getWarningTone(upgradeAttention),
    }),
    makeRow('Upgrade error', asTrimmedString(hardware.upgradeErrorMessage), { tone: 'warning' }),
    makeRow(
      'Instant clone frozen',
      hardware.instantCloneFrozen === true ? formatBoolLabel(true) : '',
      {
        tone: getWarningTone(hardware.instantCloneFrozen === true),
      },
    ),
    makeRow('Enter setup mode', hardware.enterSetupMode === true ? formatBoolLabel(true) : '', {
      tone: getWarningTone(hardware.enterSetupMode === true),
    }),
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

const toolsRows = (tools: ResourceVMwareTools | undefined, poweredOff: boolean): DetailRow[] => {
  if (!tools) return [];
  const versionStatus = asTrimmedString(tools.versionStatus).toUpperCase();
  const versionAttention = Boolean(versionStatus) && !['CURRENT', 'OK'].includes(versionStatus);
  // Drawer surfaces what an operator scans for: is Tools running, is its
  // version current, has the guest asked for a reboot, did the last install
  // error. Install metadata (install type, upgrade policy, auto-update
  // capability, attempt count, reboot components / time) stays in the raw
  // API; we only surface it when something is actionable.
  return compactRows([
    makeRow('Run state', formatEnumLabel(tools.runState), {
      tone: getWarningTone(
        !poweredOff &&
          Boolean(asTrimmedString(tools.runState)) &&
          !['RUNNING', 'STARTED'].includes(asTrimmedString(tools.runState).toUpperCase()),
      ),
    }),
    makeRow('Version status', versionAttention ? formatEnumLabel(tools.versionStatus) : '', {
      tone: getWarningTone(versionAttention),
    }),
    makeRow('Version', asTrimmedString(tools.version)),
    makeRow('Guest reboot', tools.guestRebootRequested === true ? 'Requested' : '', {
      tone: getWarningTone(tools.guestRebootRequested === true),
    }),
    makeRow('Last install error', asTrimmedString(tools.errorMessage), { tone: 'warning' }),
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
    formatDetailBytesValue(disk.capacityBytes, {
      allowZero: true,
      precision: 'compact',
      trimWhole: true,
    }),
    asTrimmedString(disk.datastoreName),
    asTrimmedString(disk.backingType),
    asTrimmedString(disk.vmdkFile),
  ].filter(Boolean);
  return parts.join(' · ');
};

const virtualDiskRows = (disks: ResourceVMwareVirtualDisk[] | undefined): DetailRow[] =>
  compactRows(
    (disks ?? []).map((disk) =>
      wrapRow(virtualDiskDisplayName(disk), virtualDiskValue(disk), { tone: 'default' }),
    ),
  );

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
    parts.push(formatDetailCountValue(Math.max(0, count), label));
  }
  const trimmedSummary = asTrimmedString(summary);
  if (trimmedSummary) {
    parts.push(trimmedSummary);
  }
  return parts.join(' · ');
};

// vCenter's overall status colours, in plain words: red is a problem, yellow
// may be one, green is fine and gray is not known.
const OVERALL_STATUS_PRESENTATION: Record<string, { label: string; tone: DetailValueTone }> = {
  red: { label: 'Problem (red)', tone: 'danger' },
  yellow: { label: 'Warning (yellow)', tone: 'warning' },
  green: { label: 'OK (green)', tone: 'success' },
  gray: { label: 'Unknown (gray)', tone: 'default' },
};

const overallStatusRow = (status?: string | null): DetailRow | null => {
  const normalized = asTrimmedString(status).toLowerCase();
  const presentation = OVERALL_STATUS_PRESENTATION[normalized];
  return presentation
    ? makeRow('Overall status', presentation.label, { tone: presentation.tone })
    : makeRow('Overall status', asTrimmedString(status));
};

// A host that vCenter cannot reach is the reason its row reads offline.
const connectionStateTone = (state?: string): DetailValueTone => {
  const normalized = asTrimmedString(state)
    .toLowerCase()
    .replace(/[^a-z]/g, '');
  return normalized === 'disconnected' || normalized === 'notresponding' ? 'warning' : 'default';
};

// Datastores report "normal" when not in maintenance; anything else
// (entering or in maintenance) means VMs cannot rely on it.
const maintenanceRow = (mode?: string): DetailRow | null => {
  const raw = asTrimmedString(mode);
  if (!raw) return null;
  if (raw.toLowerCase() === 'normal') return makeRow('Maintenance', 'No');
  const words = raw.replace(/([a-z])([A-Z])/g, '$1 $2').replace(/[_-]+/g, ' ');
  return makeRow('Maintenance', words.charAt(0).toUpperCase() + words.slice(1).toLowerCase(), {
    tone: 'warning',
  });
};

const getAccentTone = (hasAccent: boolean): DetailValueTone => (hasAccent ? 'accent' : 'default');

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
    parts.push(formatDetailCountValue(snapshotCount, 'snapshot'));
  }
  const networkAdapterCount = vmware.networkAdapters?.length ?? 0;
  if (resourceType === 'vm' && networkAdapterCount > 0) {
    parts.push(formatDetailCountValue(networkAdapterCount, 'vNIC'));
  }
  const virtualDiskCount = vmware.virtualDisks?.length ?? 0;
  if (resourceType === 'vm' && virtualDiskCount > 0) {
    parts.push(formatDetailCountValue(virtualDiskCount, 'disk'));
  }
  if (resourceType === 'network') {
    const hostCount = vmware.networkHostNames?.length ?? vmware.networkHostIds?.length ?? 0;
    const vmCount = vmware.networkVmNames?.length ?? vmware.networkVmIds?.length ?? 0;
    if (hostCount > 0) parts.push(formatDetailCountValue(hostCount, 'host'));
    if (vmCount > 0) parts.push(formatDetailCountValue(vmCount, 'VM'));
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
    parts.push(formatDetailCountValue(vmware.activeAlarmCount ?? 0, 'alarm'));
  }
  if ((vmware.recentTaskCount ?? 0) > 0) {
    parts.push(formatDetailCountValue(vmware.recentTaskCount ?? 0, 'task'));
  }

  return parts.join(' · ');
};

export const buildVMwareDetailSections = (
  resourceType: ResourceType,
  vmware?: ResourceVMwareMeta,
): DetailSection[] => {
  if (!vmware) {
    return [];
  }

  const poweredOff = isPoweredOff(vmware);
  const stateRows = compactRows([
    wrapRow('Connection', asTrimmedString(vmware.connectionName)),
    wrapRow('vCenter', asTrimmedString(vmware.vcenterHost)),
    makeRow('Entity', vmwareEntityLabel(vmware.entityType)),
    overallStatusRow(vmware.overallStatus),
    // Power state is already conveyed by the workload row's status dot and
    // the SYSTEM card. We only resurface Connection state here because it
    // matters when an ESXi host is disconnected and the row dot alone can't
    // tell you why.
    makeRow('Connection state', formatEnumLabel(vmware.connectionState), {
      tone: connectionStateTone(vmware.connectionState),
    }),
    makeRow('Datastore type', asTrimmedString(vmware.datastoreType)),
    makeRow('Accessible', formatBoolLabel(vmware.datastoreAccessible), {
      tone: getWarningTone(vmware.datastoreAccessible === false),
    }),
    makeRow('Shared access', formatBoolLabel(vmware.multipleHostAccess)),
    maintenanceRow(vmware.maintenanceMode),
    makeRow('Network type', formatEnumLabel(vmware.networkType)),
  ]);

  const placementRows = compactRows([
    wrapRow('Datacenter', asTrimmedString(vmware.datacenterName)),
    wrapRow('Cluster', asTrimmedString(vmware.clusterName)),
    makeRow('Cluster services', formatVmwareClusterServices(vmware)),
    wrapRow('Compute resource', asTrimmedString(vmware.computeResourceName)),
    wrapRow('Folder', asTrimmedString(vmware.folderName)),
    wrapRow('Resource pool', asTrimmedString(vmware.resourcePoolName)),
    wrapRow('Runtime host', asTrimmedString(vmware.runtimeHostName)),
    wrapRow('Datastores', (vmware.datastoreNames ?? []).filter(Boolean).join(', ')),
  ]);

  // UUID fields (host / instance / BIOS) and datastoreUrl are API-shaped
  // identifiers an operator never types or compares from the drawer; they
  // are still available in the raw resource payload. Keep guest identity
  // human-readable.
  const guestRows = compactRows([
    makeRow('Guest OS', asTrimmedString(vmware.guestOsFamily)),
    wrapRow('Guest hostname', asTrimmedString(vmware.guestHostname)),
    wrapRow('Guest IPs', (vmware.guestIpAddresses ?? []).filter(Boolean).join(', ')),
  ]);

  const networkRows =
    resourceType === 'vm'
      ? networkAdapterRows(vmware.networkAdapters, poweredOff)
      : compactRows([
          wrapRow('Hosts', summarizeList(vmware.networkHostNames)),
          wrapRow('VMs', summarizeList(vmware.networkVmNames)),
        ]);
  const vmwareHardwareRows = resourceType === 'vm' ? hardwareRows(vmware) : [];
  const vmwareToolsRows = resourceType === 'vm' ? toolsRows(vmware.tools, poweredOff) : [];
  const diskRows = resourceType === 'vm' ? virtualDiskRows(vmware.virtualDisks) : [];

  const signalRows = compactRows([
    makeRow(
      'Alarms',
      buildSignalValue(vmware.activeAlarmCount, 'alarm', vmware.activeAlarmSummary),
      {
        tone: getWarningTone((vmware.activeAlarmCount ?? 0) > 0),
        wrap: true,
      },
    ),
    makeRow('Tasks', buildSignalValue(vmware.recentTaskCount, 'task', vmware.recentTaskSummary), {
      tone: getAccentTone((vmware.recentTaskCount ?? 0) > 0),
    }),
    makeRow(
      'Snapshots',
      resourceType === 'vm' || typeof vmware.snapshotCount === 'number'
        ? formatDetailCountValue(
            Math.max(
              0,
              typeof vmware.snapshotCount === 'number'
                ? vmware.snapshotCount
                : countSnapshotTree(vmware.snapshotTree),
            ),
            'snapshot',
          )
        : '',
    ),
  ]);

  const snapshotRows = resourceType === 'vm' ? flattenSnapshotRows(vmware.snapshotTree) : [];

  return compactSections([
    { label: 'State', rows: stateRows },
    { label: 'Placement', rows: placementRows },
    { label: 'Guest', rows: guestRows },
    { label: 'Virtual hardware', rows: vmwareHardwareRows },
    { label: 'VMware Tools', rows: vmwareToolsRows },
    { label: 'Virtual disks', rows: diskRows },
    { label: 'Network', rows: networkRows },
    { label: 'Signals', rows: signalRows },
    { label: 'Snapshot tree', rows: snapshotRows },
  ]);
};
