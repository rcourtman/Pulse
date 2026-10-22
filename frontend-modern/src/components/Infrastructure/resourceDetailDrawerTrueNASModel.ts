import type {
  Resource,
  ResourcePhysicalDiskMeta,
  ResourceStorageMeta,
  ResourceTrueNASAppMeta,
  ResourceTrueNASAppPort,
  ResourceTrueNASMeta,
  ResourceTrueNASServiceMeta,
  ResourceTrueNASShareMeta,
  ResourceTrueNASVMMeta,
} from '@/types/resource';
import {
  compactDetailRows,
  compactDetailSections,
  formatDetailBytesValue,
  formatDetailCountValue,
  formatDetailIntegerValue,
  makeDetailRow,
  type DetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/detailSectionModel';
import {
  getPhysicalDiskTemperaturePresentation,
  isPhysicalDiskRunningHot,
} from '@/features/storageBackups/diskTemperaturePresentation';
import type { TrueNASDiskTemperatureThresholdResolver } from '@/features/truenas/truenasPageModel';
import { hasImpairedResourceSource } from '@/utils/resourceSourceHealth';
import { drawerContentCardTestId } from './resourceDetailDrawerOverviewModel';

export type ResourceDetailDrawerTrueNASRowTone = DetailValueTone;

export type ResourceDetailDrawerTrueNASRow = DetailRow;

export type ResourceDetailDrawerTrueNASSection = DetailSection;

const asString = (value?: string | null): string | null => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
};

const asPositiveNumber = (value?: number): number | null =>
  typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : null;

const normalizeDelimitedLabel = (value?: string): string | null => {
  const trimmed = asString(value);
  if (!trimmed) return null;
  return trimmed
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');
};

const formatPercent = (percent?: number): string | null => {
  if (typeof percent !== 'number' || !Number.isFinite(percent)) return null;
  return `${percent.toFixed(percent >= 10 ? 1 : 2)}%`;
};

const formatDurationSeconds = (seconds?: number): string | null => {
  const value = asPositiveNumber(seconds);
  if (!value) return null;
  const days = Math.floor(value / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(value / 3_600);
  if (hours > 0) return `${hours}h`;
  const minutes = Math.floor(value / 60);
  return minutes > 0 ? `${minutes}m` : '<1m';
};

const summarizeList = (
  values: string[],
  visibleCount = 3,
): ResourceDetailDrawerTrueNASRow['value'] => {
  const visible = values.map((value) => value.trim()).filter(Boolean);
  if (visible.length === 0) return '';
  const head = visible.slice(0, visibleCount);
  const suffix = visible.length > head.length ? ` +${visible.length - head.length}` : '';
  return `${head.join(', ')}${suffix}`;
};

const booleanValue = (value?: boolean): string | null => {
  if (value === undefined) return null;
  return value ? 'Enabled' : 'Disabled';
};

const yesNoValue = (value?: boolean): string | null => {
  if (value === undefined) return null;
  return value ? 'Yes' : 'No';
};

const row = makeDetailRow;
const compactRows = compactDetailRows;
const compactSections = compactDetailSections;

// Identifiers and paths an operator copies (hostnames, serials, dataset
// paths, addresses) wrap in full instead of ending in an ellipsis.
const wrapRow = (
  label: string,
  value?: string | null,
  options: Parameters<typeof makeDetailRow>[2] = {},
) => makeDetailRow(label, value, { ...options, wrap: true });

const joinList = (values: readonly string[]): string =>
  values
    .map((value) => value.trim())
    .filter(Boolean)
    .join(', ');

// The health rows often repeat one summary under several labels (a degraded
// pool's risk, posture and protection all say so). The first label keeps it.
const withoutRepeatedValues = (
  rows: Array<ResourceDetailDrawerTrueNASRow | null>,
): ResourceDetailDrawerTrueNASRow[] => {
  const seen = new Set<string>();
  return compactRows(rows).filter((detailRow) => {
    // Short values ("Yes", "2 services") legitimately repeat; sentences do not.
    if (!/\s/.test(detailRow.value) || detailRow.value.length < 16) return true;
    const key = detailRow.value.trim().toLowerCase();
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
};

const isTrueNASScopedResource = (resource: Resource): boolean =>
  resource.platformType === 'truenas' ||
  resource.platformScopes?.includes('truenas') === true ||
  resource.sources?.includes('truenas') === true ||
  resource.storage?.platform === 'truenas' ||
  resource.tags?.includes('truenas') === true;

type TrueNASServiceStatus = 'running' | 'attention' | 'stopped' | 'disabled';

const truenasServiceStatus = (service: ResourceTrueNASServiceMeta): TrueNASServiceStatus => {
  const state = asString(service.state)?.toLowerCase() ?? '';
  if (['running', 'started', 'active'].includes(state)) return 'running';
  if (['failed', 'error', 'crashed', 'degraded', 'unknown'].includes(state)) return 'attention';
  if (['stopped', 'stop', 'inactive'].includes(state)) {
    return service.enabled === false ? 'disabled' : 'stopped';
  }
  return service.enabled === false ? 'disabled' : 'attention';
};

const serviceStatusLabel = (status: TrueNASServiceStatus): string => {
  if (status === 'running') return 'Running';
  if (status === 'attention') return 'Attention';
  if (status === 'stopped') return 'Stopped';
  return 'Disabled';
};

const serviceStatusTone = (status: TrueNASServiceStatus): ResourceDetailDrawerTrueNASRowTone => {
  if (status === 'running') return 'success';
  if (status === 'attention' || status === 'stopped') return 'warning';
  return 'default';
};

const serviceNameLabel = (service: ResourceTrueNASServiceMeta): string | null => {
  const value = asString(service.service) ?? asString(service.id);
  if (!value) return null;
  const normalized = value.toLowerCase();
  if (['ftp', 'nfs', 's3', 'smb', 'snmp', 'ssh', 'ups'].includes(normalized)) {
    return normalized.toUpperCase();
  }
  if (normalized === 'smartd') return 'SMART';
  return normalizeDelimitedLabel(value);
};

const buildTrueNASSystemSections = (
  resource: Resource,
  truenas: ResourceTrueNASMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const services = truenas.services ?? [];
  const serviceCounts = services.reduce<Record<TrueNASServiceStatus, number>>(
    (counts, service) => {
      counts[truenasServiceStatus(service)] += 1;
      return counts;
    },
    { running: 0, attention: 0, stopped: 0, disabled: 0 },
  );
  const serviceLabels = services
    .map((service) => serviceNameLabel(service))
    .filter((value): value is string => Boolean(value));
  const pids = services.flatMap((service) => service.pids ?? []).filter((pid) => pid > 0);
  const systemRows = compactRows([
    wrapRow('Hostname', asString(truenas.hostname) ?? asString(resource.name)),
    row('Version', asString(truenas.version)),
    row('Uptime', formatDurationSeconds(truenas.uptimeSeconds ?? resource.uptime)),
    row('Status', normalizeDelimitedLabel(resource.status)),
  ]);

  const healthRows = withoutRepeatedValues([
    row('Storage risk', normalizeDelimitedLabel(truenas.storageRisk?.level), {
      tone: truenas.storageRisk?.level?.toLowerCase() === 'warning' ? 'warning' : 'default',
    }),
    row('Storage summary', asString(truenas.storageRiskSummary), { tone: 'warning' }),
    row('Storage posture', asString(truenas.storagePostureSummary), {
      tone: truenas.storagePostureSummary ? 'warning' : 'default',
    }),
    row('Protection reduced', yesNoValue(truenas.protectionReduced), {
      tone: truenas.protectionReduced ? 'warning' : 'success',
    }),
    row('Protection summary', asString(truenas.protectionSummary), {
      tone: truenas.protectionSummary ? 'warning' : 'default',
    }),
    row('Rebuild active', yesNoValue(truenas.rebuildInProgress), {
      tone: truenas.rebuildInProgress ? 'warning' : 'success',
    }),
    row('Rebuild', asString(truenas.rebuildSummary)),
  ]);

  const serviceRows = compactRows([
    row(
      'Services',
      services.length > 0 ? formatDetailCountValue(services.length, 'service') : null,
    ),
    ...(['running', 'attention', 'stopped', 'disabled'] as TrueNASServiceStatus[]).map((status) =>
      row(
        serviceStatusLabel(status),
        serviceCounts[status] > 0 ? formatDetailCountValue(serviceCounts[status], 'service') : null,
        {
          tone: serviceStatusTone(status),
        },
      ),
    ),
    row('PIDs', pids.length > 0 ? summarizeList(pids.map(String), 4) : null, {
      title: pids.join(', '),
    }),
    row('Names', summarizeList(serviceLabels, 6), { title: serviceLabels.join(', ') }),
  ]);

  return compactSections([
    { label: 'System', rows: systemRows },
    { label: 'Storage Health', rows: healthRows },
    { label: 'Services', rows: serviceRows },
  ]);
};

const storageKindLabel = (storage: ResourceStorageMeta): string | null => {
  const layout = asString(storage.vdevLayout);
  if (layout) return normalizeDelimitedLabel(layout);
  const topology = asString(storage.topology);
  if (topology) return normalizeDelimitedLabel(topology);
  return normalizeDelimitedLabel(storage.type);
};

const storageStateLabel = (resource: Resource, storage: ResourceStorageMeta): string | null =>
  asString(storage.zfsPoolState) ??
  normalizeDelimitedLabel(storage.arrayState) ??
  normalizeDelimitedLabel(resource.status);

const storageStateTone = (
  resource: Resource,
  storage: ResourceStorageMeta,
): ResourceDetailDrawerTrueNASRowTone => {
  const state = (storage.zfsPoolState ?? storage.arrayState ?? resource.status).toLowerCase();
  if (state === 'online' || state === 'healthy' || state === 'mounted') return 'success';
  if (state === 'degraded' || state === 'warning' || state === 'offline') return 'warning';
  if (['faulted', 'unavail', 'removed', 'suspended'].includes(state)) return 'danger';
  return 'default';
};

const storageUsageLabel = (resource: Resource): string | null => {
  const used = formatDetailBytesValue(resource.disk?.used);
  const total = formatDetailBytesValue(resource.disk?.total);
  if (used && total) return `${used} / ${total}`;
  return formatPercent(resource.disk?.current);
};

const storageProtectionLabel = (value?: string): string | null => {
  const protection = asString(value);
  if (!protection) return null;
  const normalized = protection.toLowerCase();
  if (normalized === 'zfs' || normalized.startsWith('raidz')) return normalized.toUpperCase();
  return normalizeDelimitedLabel(protection);
};

const zfsScanLabel = (storage: ResourceStorageMeta): string | null => {
  const scan = storage.zfsPool?.scanDetails;
  if (!scan) return asString(storage.zfsPool?.scan);
  const operation = normalizeDelimitedLabel(scan.function) ?? 'Scan';
  const state = normalizeDelimitedLabel(scan.state);
  const progress =
    typeof scan.percentage === 'number' && Number.isFinite(scan.percentage) && scan.percentage > 0
      ? ` (${scan.percentage.toFixed(1)}%)`
      : '';
  const errors =
    typeof scan.errors === 'number' && scan.errors > 0 ? ` · ${scan.errors} errors` : '';
  return `${operation}${state ? ` ${state}` : ''}${progress}${errors}`;
};

const zfsErrorLabel = (storage: ResourceStorageMeta): string | null => {
  const read = storage.zfsReadErrors ?? storage.zfsPool?.readErrors ?? 0;
  const write = storage.zfsWriteErrors ?? storage.zfsPool?.writeErrors ?? 0;
  const checksum = storage.zfsChecksumErrors ?? storage.zfsPool?.checksumErrors ?? 0;
  if (read <= 0 && write <= 0 && checksum <= 0) return null;
  return `Read ${read} · Write ${write} · Checksum ${checksum}`;
};

const zfsDeviceEvidenceLabels = (storage: ResourceStorageMeta): string[] =>
  (storage.zfsPool?.devices ?? [])
    .filter((device) => {
      const state = device.state?.trim().toUpperCase();
      return (
        device.missing === true ||
        (state !== '' && !['ONLINE', 'AVAIL', 'INUSE'].includes(state)) ||
        device.readErrors > 0 ||
        device.writeErrors > 0 ||
        device.checksumErrors > 0
      );
    })
    .map((device) => {
      const name =
        asString(device.disk) ?? asString(device.path) ?? asString(device.name) ?? 'Vdev';
      const role = normalizeDelimitedLabel(device.role);
      const type = normalizeDelimitedLabel(device.type);
      const state = device.missing ? 'Missing' : (asString(device.state) ?? 'Unknown');
      const context = [role, type].filter(Boolean).join(' / ');
      const errors = [device.readErrors, device.writeErrors, device.checksumErrors].some(
        (value) => value > 0,
      )
        ? ` · R ${device.readErrors} W ${device.writeErrors} C ${device.checksumErrors}`
        : '';
      return `${name}${context ? ` (${context})` : ''}: ${state}${errors}`;
    });

// A pool's health is its content, the way RAID is a Proxmox node's: it stays
// in view on the Overview. A dataset or share has no pool health of its own.
const isTrueNASPool = (storage: ResourceStorageMeta): boolean =>
  Boolean(storage.zfsPool || storage.poolHealth) ||
  asString(storage.topology)?.toLowerCase() === 'pool' ||
  asString(storage.type)?.toLowerCase().includes('pool') === true;

const canonicalPoolStateTone = (state?: string): ResourceDetailDrawerTrueNASRowTone => {
  if (state === 'ONLINE') return 'success';
  if (state === 'DEGRADED' || state === 'OFFLINE') return 'warning';
  if (state === 'FAULTED' || state === 'UNAVAIL') return 'danger';
  return 'default';
};

const buildTrueNASStorageSections = (
  resource: Resource,
  storage: ResourceStorageMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const riskReasons = (storage.risk?.reasons ?? [])
    .map((reason) => asString(reason.summary))
    .filter((value): value is string => Boolean(value));
  const zfsDeviceEvidence = zfsDeviceEvidenceLabels(storage);
  const stateLabel = storageStateLabel(resource, storage);
  const stateRow = row('State', stateLabel, { tone: storageStateTone(resource, storage) });
  const canonicalState = asString(storage.poolHealth?.canonicalState);
  const pool = isTrueNASPool(storage);
  const storageRows = compactRows([
    row('Kind', storageKindLabel(storage)),
    // A pool states its state in its health card; a dataset here.
    pool ? null : stateRow,
    wrapRow('Pool', asString(storage.pool) ?? asString(resource.parentName)),
    wrapRow('Path', asString(storage.path)),
    row('Protection', storageProtectionLabel(storage.protection)),
    row('Shared', yesNoValue(storage.shared)),
  ]);

  const capacityRows = compactRows([
    row('Usage', storageUsageLabel(resource)),
    row('Used', formatDetailBytesValue(resource.disk?.used)),
    row('Total', formatDetailBytesValue(resource.disk?.total)),
    row('Percent', formatPercent(resource.disk?.current)),
    row('Children', formatDetailIntegerValue(resource.childCount)),
    row('Consumers', formatDetailIntegerValue(storage.consumerCount)),
  ]);

  // What the operator reads first: is the pool OK, and if not, why and what
  // to do. Evidence codes and native states stay under Details.
  const healthRows = withoutRepeatedValues([
    pool ? stateRow : null,
    row(
      'Pool health',
      canonicalState && canonicalState.toLowerCase() !== stateLabel?.toLowerCase()
        ? canonicalState
        : null,
      { tone: canonicalPoolStateTone(canonicalState ?? undefined) },
    ),
    row('Risk', normalizeDelimitedLabel(storage.risk?.level), {
      tone: storage.risk?.level?.toLowerCase() === 'warning' ? 'warning' : 'default',
    }),
    row('Risk summary', asString(storage.riskSummary), { tone: 'warning' }),
    row('Posture', asString(storage.postureSummary), {
      tone: storage.postureSummary ? 'warning' : 'default',
    }),
    row('Protection reduced', yesNoValue(storage.protectionReduced), {
      tone: storage.protectionReduced ? 'warning' : 'success',
    }),
    row('Protection summary', asString(storage.protectionSummary), {
      tone: storage.protectionSummary ? 'warning' : 'default',
    }),
    row('Rebuild', asString(storage.rebuildSummary)),
    row('Scan / resilver', zfsScanLabel(storage), {
      tone: storage.zfsPool?.scanDetails?.errors ? 'warning' : 'default',
    }),
    row('ZFS errors', zfsErrorLabel(storage), { tone: 'warning' }),
    wrapRow('Affected vdevs', joinList(zfsDeviceEvidence), { tone: 'warning' }),
    wrapRow('Reasons', joinList(riskReasons), { tone: 'warning' }),
    wrapRow('Recommended', asString(storage.poolHealth?.recommendation), {
      title: asString(storage.poolHealth?.summary) ?? undefined,
    }),
  ]);

  const evidenceRows = compactRows([
    row('Native state', asString(storage.poolHealth?.nativeState)),
    wrapRow('Evidence', joinList(storage.poolHealth?.evidenceCodes ?? [])),
    row('Evidence source', asString(storage.poolHealth?.source)),
  ]);

  return compactSections([
    { label: 'Storage', rows: storageRows },
    { label: 'Capacity', rows: capacityRows },
    {
      label: 'Health',
      rows: healthRows,
      ...(pool ? { testId: drawerContentCardTestId('truenas-pool-health') } : {}),
    },
    { label: 'Health evidence', rows: evidenceRows },
  ]);
};

const diskStateTone = (disk: ResourcePhysicalDiskMeta): ResourceDetailDrawerTrueNASRowTone => {
  const health = disk.health?.toLowerCase();
  if (health === 'passed' || health === 'healthy' || health === 'ok') return 'success';
  if (health === 'failed') return 'danger';
  if (health === 'degraded' || health === 'warning') return 'warning';
  return 'default';
};

const diskTypeLabel = (value?: string): string | null => {
  const diskType = asString(value);
  if (!diskType) return null;
  const normalized = diskType.toLowerCase();
  if (normalized === 'nvme') return 'NVMe';
  if (['sata', 'sas', 'ssd', 'hdd'].includes(normalized)) return normalized.toUpperCase();
  return normalizeDelimitedLabel(diskType);
};

const formatDiskHours = (hours?: number): string | null => {
  const value = asPositiveNumber(hours);
  if (!value) return null;
  return `${formatDetailIntegerValue(value) ?? value.toFixed(0)}h`;
};

const diskTemperature = (disk: ResourcePhysicalDiskMeta) =>
  getPhysicalDiskTemperaturePresentation({
    temperature: disk.temperature ?? 0,
    collection: disk.collection,
  });

// A retained reading, such as a silent host agent's on a merged disk row, is
// not current: it takes no heat tone and its title says why. A current reading
// is hot from the trigger the disk's temperature alert fires at (its own
// override, the TrueNAS-wide value, then its type's policy), the threshold the
// TrueNAS storage table judges it by too. Without a resolver, heat is not
// judged.
const diskTemperatureRow = (
  resource: Resource,
  disk: ResourcePhysicalDiskMeta,
  resolveDiskTemperatureThresholds?: TrueNASDiskTemperatureThresholdResolver,
): ResourceDetailDrawerTrueNASRow | null => {
  const reading = diskTemperature(disk);
  if (!reading) return null;
  if (!reading.current) {
    return row('Temperature', `${reading.label} (last known)`, {
      title: reading.title,
      tone: 'muted',
    });
  }
  const thresholds = resolveDiskTemperatureThresholds?.(resource) ?? null;
  const hot = isPhysicalDiskRunningHot(
    { temperature: disk.temperature ?? 0, collection: disk.collection },
    thresholds,
  );
  return row('Temperature', reading.label, { tone: hot ? 'warning' : 'default' });
};

const currentDiskTemperatureLabel = (disk: ResourcePhysicalDiskMeta): string | null => {
  const reading = diskTemperature(disk);
  return reading?.current ? reading.label : null;
};

const buildTrueNASDiskSections = (
  resource: Resource,
  disk: ResourcePhysicalDiskMeta,
  resolveDiskTemperatureThresholds?: TrueNASDiskTemperatureThresholdResolver,
): ResourceDetailDrawerTrueNASSection[] => {
  const riskReasons = (disk.risk?.reasons ?? [])
    .map((reason) => asString(reason.summary))
    .filter((value): value is string => Boolean(value));
  const identityRows = compactRows([
    wrapRow('Device', asString(disk.devPath) ?? asString(resource.name)),
    wrapRow('Model', asString(disk.model)),
    wrapRow('Serial', asString(disk.serial)),
    wrapRow('WWN', asString(disk.wwn)),
    row('Type', diskTypeLabel(disk.diskType)),
    row('Size', formatDetailBytesValue(disk.sizeBytes)),
  ]);

  const healthRows = compactRows([
    row('Health', normalizeDelimitedLabel(disk.health), { tone: diskStateTone(disk) }),
    diskTemperatureRow(resource, disk, resolveDiskTemperatureThresholds),
    row(
      'Wearout',
      disk.wearout === undefined || disk.wearout < 0 ? null : formatPercent(disk.wearout),
    ),
    row('RPM', formatDetailIntegerValue(disk.rpm)),
    row('Role', normalizeDelimitedLabel(disk.storageRole)),
    wrapRow('Group', asString(disk.storageGroup) ?? asString(resource.parentName)),
    row('State', normalizeDelimitedLabel(disk.storageState)),
    row('Spun down', yesNoValue(disk.spunDown)),
  ]);

  const smartRows = compactRows([
    row('Power on', formatDiskHours(disk.smart?.powerOnHours)),
    row('Power cycles', formatDetailIntegerValue(disk.smart?.powerCycles)),
    row('Reallocated', formatDetailIntegerValue(disk.smart?.reallocatedSectors), {
      tone: disk.smart?.reallocatedSectors ? 'warning' : 'default',
    }),
    row('Pending sectors', formatDetailIntegerValue(disk.smart?.pendingSectors), {
      tone: disk.smart?.pendingSectors ? 'warning' : 'default',
    }),
    row('Offline uncorrectable', formatDetailIntegerValue(disk.smart?.offlineUncorrectable), {
      tone: disk.smart?.offlineUncorrectable ? 'warning' : 'default',
    }),
    row('CRC errors', formatDetailIntegerValue(disk.smart?.udmaCrcErrors), {
      tone: disk.smart?.udmaCrcErrors ? 'warning' : 'default',
    }),
    row('Media errors', formatDetailIntegerValue(disk.smart?.mediaErrors), {
      tone: disk.smart?.mediaErrors ? 'warning' : 'default',
    }),
    row('Unsafe shutdowns', formatDetailIntegerValue(disk.smart?.unsafeShutdowns)),
    row('Available spare', formatPercent(disk.smart?.availableSpare)),
    row('Percentage used', formatPercent(disk.smart?.percentageUsed)),
  ]);

  const riskRows = compactRows([
    row('Risk', normalizeDelimitedLabel(disk.risk?.level), {
      tone: disk.risk?.level?.toLowerCase() === 'warning' ? 'warning' : 'default',
    }),
    wrapRow('Reasons', joinList(riskReasons), { tone: 'warning' }),
  ]);

  // Health and SMART readings are the disk's content, the way the Proxmox node
  // drawer shows its disks: they stay in view on the Overview.
  return compactSections([
    { label: 'Disk', rows: identityRows },
    { label: 'Health', rows: healthRows, testId: drawerContentCardTestId('truenas-disk-health') },
    { label: 'SMART', rows: smartRows, testId: drawerContentCardTestId('truenas-disk-smart') },
    { label: 'Risk', rows: riskRows },
  ]);
};

const portLabel = (port: ResourceTrueNASAppPort): string | null => {
  const protocol = asString(port.protocol)?.toLowerCase();
  const container =
    asPositiveNumber(port.containerPort) !== null
      ? `${port.containerPort}${protocol ? `/${protocol}` : ''}`
      : protocol;
  const hostPorts = (port.hostPorts ?? [])
    .map((hostPort) => {
      const portNumber = asPositiveNumber(hostPort.hostPort);
      if (!portNumber) return null;
      const hostIp = asString(hostPort.hostIp);
      return hostIp ? `${hostIp}:${portNumber}` : `${portNumber}`;
    })
    .filter((value): value is string => Boolean(value));

  if (hostPorts.length > 0 && container) return `${hostPorts.join(', ')} -> ${container}`;
  if (hostPorts.length > 0) return hostPorts.join(', ');
  return container ?? null;
};

const appPortLabels = (app: ResourceTrueNASAppMeta): string[] =>
  (app.usedPorts ?? []).map(portLabel).filter((value): value is string => Boolean(value));

const appVolumeLabels = (app: ResourceTrueNASAppMeta): string[] =>
  (app.volumes ?? [])
    .map((volume) => {
      const source = asString(volume.source);
      const destination = asString(volume.destination);
      if (source && destination) return `${source} -> ${destination}`;
      return destination ?? source;
    })
    .filter((value): value is string => Boolean(value));

const appNetworkLabels = (app: ResourceTrueNASAppMeta): string[] =>
  (app.networks ?? [])
    .map((network) => asString(network.name) ?? asString(network.id))
    .filter((value): value is string => Boolean(value));

const buildTrueNASAppSections = (
  app: ResourceTrueNASAppMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const containerCount = app.containerCount ?? app.containers?.length;
  const portLabels = appPortLabels(app);
  const volumeLabels = appVolumeLabels(app);
  const imageLabels = (app.images ?? []).map((image) => image.trim()).filter(Boolean);
  const networkLabels = appNetworkLabels(app);
  const appState = asString(app.state)?.toLowerCase();
  const appRows = compactRows([
    // A stopped app is a choice the row already shows; a crashed one is not.
    row('State', normalizeDelimitedLabel(app.state), {
      tone:
        appState === 'running'
          ? 'success'
          : appState === 'crashed' || appState === 'error' || appState === 'failed'
            ? 'danger'
            : 'default',
    }),
    row('Version', asString(app.humanVersion) ?? asString(app.version)),
    row('Containers', formatDetailIntegerValue(containerCount)),
    row('Custom app', yesNoValue(app.customApp)),
    // An available update is something to act on, as for a Docker container:
    // the Overview lifts it in front.
    row(
      'App updates',
      app.upgradeAvailable === undefined ? null : app.upgradeAvailable ? 'Available' : 'Current',
      {
        tone: app.upgradeAvailable ? 'accent' : 'success',
        lift: app.upgradeAvailable === true,
      },
    ),
    row(
      'Image updates',
      app.imageUpdatesAvailable === undefined
        ? null
        : app.imageUpdatesAvailable
          ? 'Available'
          : 'Current',
      {
        tone: app.imageUpdatesAvailable ? 'accent' : 'success',
        lift: app.imageUpdatesAvailable === true,
      },
    ),
    row('Notes', asString(app.notes)),
  ]);

  const networkRows = compactRows([
    wrapRow('Host IPs', joinList(app.usedHostIps ?? [])),
    wrapRow('Ports', joinList(portLabels)),
    wrapRow('Networks', joinList(networkLabels)),
  ]);

  const storageRows = compactRows([
    wrapRow('Volumes', joinList(volumeLabels)),
    wrapRow('Images', joinList(imageLabels)),
  ]);

  return compactSections([
    { label: 'App', rows: appRows },
    { label: 'Networking', rows: networkRows },
    { label: 'Storage', rows: storageRows },
  ]);
};

const formatVMCpu = (vm: ResourceTrueNASVMMeta): string | null => {
  const vcpus = asPositiveNumber(vm.vcpus);
  if (vcpus) return formatDetailCountValue(vcpus, 'vCPU', 'vCPU');
  const cores = asPositiveNumber(vm.cores);
  const threads = asPositiveNumber(vm.threads);
  if (cores && threads) return `${cores} cores x ${threads} threads`;
  if (cores) return formatDetailCountValue(cores, 'core');
  if (threads) return formatDetailCountValue(threads, 'thread');
  return null;
};

const formatVMTopology = (vm: ResourceTrueNASVMMeta): string | null => {
  const cores = asPositiveNumber(vm.cores);
  const threads = asPositiveNumber(vm.threads);
  if (cores && threads) return `${cores} cores x ${threads} threads`;
  if (cores) return formatDetailCountValue(cores, 'core');
  if (threads) return formatDetailCountValue(threads, 'thread');
  return null;
};

// Matches the stopped states in mapTrueNASVMStatus (features/truenas). A VM set
// to autostart that is stopped is the one stopped VM nobody chose, and the
// VMs table hides its State note on phones, so the drawer says it too.
const TRUENAS_VM_STOPPED_STATES = new Set(['stopped', 'shutoff', 'shutdown', 'poweroff']);

// Stale TrueNAS data may hold an old state, so the note waits for a current one.
const vmConditionLabel = (resource: Resource, vm: ResourceTrueNASVMMeta): string | null => {
  if (hasImpairedResourceSource(resource, 'truenas')) return null;
  if (!vm.autostart) return null;
  const state = (asString(vm.state) ?? asString(vm.domainState))?.toLowerCase();
  // Without a native state, the table falls back to the resource status.
  const stopped = state
    ? TRUENAS_VM_STOPPED_STATES.has(state)
    : resource.status === 'offline' || resource.status === 'stopped';
  return stopped ? 'Should be running' : null;
};

const buildTrueNASVMSections = (
  resource: Resource,
  vm: ResourceTrueNASVMMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const state = normalizeDelimitedLabel(vm.state);
  const domainState = normalizeDelimitedLabel(vm.domainState);
  const sameState = state?.toLowerCase() === domainState?.toLowerCase();
  const machine = [asString(vm.machineType), asString(vm.archType)].filter(Boolean).join(' / ');
  const computeRows = compactRows([
    // A stopped VM is a choice the row already shows. Condition below says
    // when it should be running.
    row('State', state ?? domainState, {
      tone: (vm.state ?? vm.domainState)?.toLowerCase() === 'running' ? 'success' : 'default',
    }),
    row('Domain state', sameState ? null : domainState),
    row('Condition', vmConditionLabel(resource, vm), { tone: 'warning' }),
    row('vCPU', formatVMCpu(vm)),
    row('Topology', formatVMTopology(vm)),
    row('Memory', formatDetailBytesValue(vm.memoryBytes)),
    row('Minimum memory', formatDetailBytesValue(vm.minMemoryBytes)),
    row('CPU mode', normalizeDelimitedLabel(vm.cpuMode)),
    wrapRow('CPU model', asString(vm.cpuModel)),
  ]);

  const runtimeRows = compactRows([
    row('Bootloader', asString(vm.bootloader)),
    row('Machine', machine),
    row('Process ID', formatDetailIntegerValue(vm.pid)),
    wrapRow('UUID', asString(vm.uuid)),
  ]);

  const deviceRows = compactRows([
    row('Total', formatDetailIntegerValue(vm.deviceCount)),
    row('Disks', formatDetailIntegerValue(vm.diskCount)),
    row('NICs', formatDetailIntegerValue(vm.nicCount)),
    row('Displays', formatDetailIntegerValue(vm.displayCount)),
    row('CD-ROMs', formatDetailIntegerValue(vm.cdromCount)),
    row('USB', formatDetailIntegerValue(vm.usbCount)),
    row('PCI', formatDetailIntegerValue(vm.pciCount)),
  ]);

  const flagRows = compactRows([
    row('Autostart', booleanValue(vm.autostart)),
    row('Secure boot', booleanValue(vm.secureBoot)),
    row('TPM', booleanValue(vm.trustedPlatformModule)),
    row('Suspend on snapshot', booleanValue(vm.suspendOnSnapshot)),
    row('Display available', booleanValue(vm.displayAvailable)),
  ]);

  return compactSections([
    { label: 'Compute', rows: computeRows },
    { label: 'Runtime', rows: runtimeRows },
    { label: 'Devices', rows: deviceRows },
    { label: 'Flags', rows: flagRows },
  ]);
};

const shareProtocolLabel = (share: ResourceTrueNASShareMeta): string | null =>
  asString(share.protocol)?.toUpperCase() ?? null;

const shareStateLabel = (share: ResourceTrueNASShareMeta): string => {
  if (share.enabled === false) return 'Disabled';
  if (share.locked) return 'Locked';
  return 'Enabled';
};

// A disabled share is a choice; a locked one (its encrypted dataset is
// locked) cannot serve clients until someone unlocks it.
const shareStateTone = (share: ResourceTrueNASShareMeta): ResourceDetailDrawerTrueNASRowTone => {
  if (share.enabled === false) return 'default';
  if (share.locked) return 'warning';
  return 'success';
};

const shareModeLabel = (share: ResourceTrueNASShareMeta): string | null => {
  if (share.readOnly === true) return 'Read-only';
  if (share.readOnly === false) return 'Read/write';
  return null;
};

const shareUserGroupLabel = (user?: string, group?: string): string | null => {
  const userLabel = asString(user);
  const groupLabel = asString(group);
  if (userLabel && groupLabel) return `${userLabel}:${groupLabel}`;
  return userLabel ?? groupLabel;
};

const buildTrueNASShareSections = (
  share: ResourceTrueNASShareMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const aliases = (share.aliases ?? []).map((value) => value.trim()).filter(Boolean);
  const hosts = (share.hosts ?? []).map((value) => value.trim()).filter(Boolean);
  const networks = (share.networks ?? []).map((value) => value.trim()).filter(Boolean);
  const security = (share.security ?? []).map((value) => value.trim()).filter(Boolean);
  const shareRows = compactRows([
    row('State', shareStateLabel(share), { tone: shareStateTone(share) }),
    row('Protocol', shareProtocolLabel(share)),
    wrapRow('Dataset', asString(share.dataset)),
    wrapRow('Path', asString(share.path)),
    wrapRow('Relative path', asString(share.relativePath)),
    row('Comment', asString(share.comment)),
  ]);

  const accessRows = compactRows([
    row('Mode', shareModeLabel(share)),
    row('Browsable', yesNoValue(share.browsable)),
    row('Locked', yesNoValue(share.locked)),
    row('Access enumeration', booleanValue(share.accessBasedEnumeration)),
    row('Audit', booleanValue(share.auditEnabled)),
    row('Snapshots', booleanValue(share.exposeSnapshots)),
  ]);

  const clientRows = compactRows([
    wrapRow('Aliases', joinList(aliases)),
    wrapRow('Hosts', joinList(hosts)),
    wrapRow('Networks', joinList(networks)),
    row('Security', summarizeList(security), { title: security.join(', ') }),
    row('Map root', shareUserGroupLabel(share.mapRootUser, share.mapRootGroup)),
    row('Map all', shareUserGroupLabel(share.mapAllUser, share.mapAllGroup)),
  ]);

  return compactSections([
    { label: 'Share', rows: shareRows },
    { label: 'Access', rows: accessRows },
    { label: 'Clients', rows: clientRows },
  ]);
};

export const buildTrueNASDetailSections = (
  resource: Resource,
  resolveDiskTemperatureThresholds?: TrueNASDiskTemperatureThresholdResolver,
): ResourceDetailDrawerTrueNASSection[] => {
  if (resource.truenas?.share) return buildTrueNASShareSections(resource.truenas.share);
  if (resource.truenas?.vm) return buildTrueNASVMSections(resource, resource.truenas.vm);
  if (resource.truenas?.app) return buildTrueNASAppSections(resource.truenas.app);
  if (isTrueNASScopedResource(resource) && resource.storage) {
    return buildTrueNASStorageSections(resource, resource.storage);
  }
  if (isTrueNASScopedResource(resource) && resource.physicalDisk) {
    return buildTrueNASDiskSections(
      resource,
      resource.physicalDisk,
      resolveDiskTemperatureThresholds,
    );
  }
  if (isTrueNASScopedResource(resource) && resource.truenas) {
    return buildTrueNASSystemSections(resource, resource.truenas);
  }
  return [];
};

export const buildTrueNASDetailsSummary = (resource: Resource): string | null => {
  const share = resource.truenas?.share;
  if (share) {
    const summary = [
      shareProtocolLabel(share),
      shareStateLabel(share),
      asString(share.dataset) ?? asString(share.path),
      shareModeLabel(share),
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  const vm = resource.truenas?.vm;
  if (vm) {
    const deviceCount = asPositiveNumber(vm.deviceCount);
    const condition = vmConditionLabel(resource, vm);
    const summary = [
      normalizeDelimitedLabel(vm.state ?? vm.domainState),
      condition?.toLowerCase(),
      formatVMCpu(vm),
      formatDetailBytesValue(vm.memoryBytes),
      deviceCount ? formatDetailCountValue(deviceCount, 'device') : null,
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  const app = resource.truenas?.app;
  if (app) {
    const portCount = appPortLabels(app).length;
    const updateCount = [app.upgradeAvailable, app.imageUpdatesAvailable].filter(Boolean).length;
    const summary = [
      normalizeDelimitedLabel(app.state),
      formatDetailCountValue(app.containerCount ?? app.containers?.length ?? 0, 'container'),
      formatDetailCountValue(portCount, 'port'),
      updateCount > 0 ? formatDetailCountValue(updateCount, 'update') : null,
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  if (isTrueNASScopedResource(resource) && resource.storage) {
    const storage = resource.storage;
    const summary = [
      storageKindLabel(storage),
      storageStateLabel(resource, storage),
      storageUsageLabel(resource),
      asString(storage.riskSummary),
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  if (isTrueNASScopedResource(resource) && resource.physicalDisk) {
    const disk = resource.physicalDisk;
    const summary = [
      diskTypeLabel(disk.diskType),
      normalizeDelimitedLabel(disk.health),
      formatDetailBytesValue(disk.sizeBytes),
      // The summary has no room to say a retained reading is not current.
      currentDiskTemperatureLabel(disk),
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  if (isTrueNASScopedResource(resource) && resource.truenas) {
    const truenas = resource.truenas;
    const serviceCount = truenas.services?.length;
    const summary = [
      asString(truenas.version),
      formatDurationSeconds(truenas.uptimeSeconds ?? resource.uptime),
      serviceCount !== undefined ? formatDetailCountValue(serviceCount, 'service') : null,
      asString(truenas.storageRiskSummary) ?? asString(truenas.protectionSummary),
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  return null;
};

export const hasTrueNASDetailSections = (resource: Resource): boolean =>
  buildTrueNASDetailSections(resource).length > 0;
