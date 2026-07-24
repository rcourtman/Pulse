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

const formatTemperature = (celsius?: number): string | null => {
  if (typeof celsius !== 'number' || !Number.isFinite(celsius) || celsius <= 0) return null;
  return `${celsius.toFixed(0)}°C`;
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
    row('Hostname', asString(truenas.hostname) ?? asString(resource.name)),
    row('Version', asString(truenas.version)),
    row('Uptime', formatDurationSeconds(truenas.uptimeSeconds ?? resource.uptime)),
    row('Status', normalizeDelimitedLabel(resource.status)),
  ]);

  const healthRows = compactRows([
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
        serviceCounts[status] > 0 ? `${serviceCounts[status]}` : null,
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

const buildTrueNASStorageSections = (
  resource: Resource,
  storage: ResourceStorageMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const riskReasons = (storage.risk?.reasons ?? [])
    .map((reason) => asString(reason.summary))
    .filter((value): value is string => Boolean(value));
  const zfsDeviceEvidence = zfsDeviceEvidenceLabels(storage);
  const storageRows = compactRows([
    row('Kind', storageKindLabel(storage)),
    row('State', storageStateLabel(resource, storage), {
      tone: storageStateTone(resource, storage),
    }),
    row('Pool', asString(storage.pool) ?? asString(resource.parentName)),
    row('Path', asString(storage.path)),
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

  const healthRows = compactRows([
    row('Canonical state', asString(storage.poolHealth?.canonicalState), {
      tone:
        storage.poolHealth?.canonicalState === 'ONLINE'
          ? 'success'
          : ['DEGRADED', 'FAULTED', 'OFFLINE', 'UNAVAIL'].includes(
                storage.poolHealth?.canonicalState ?? '',
              )
            ? 'warning'
            : 'default',
    }),
    row('Native state', asString(storage.poolHealth?.nativeState)),
    row('Risk', normalizeDelimitedLabel(storage.risk?.level), {
      tone: storage.risk?.level?.toLowerCase() === 'warning' ? 'warning' : 'default',
    }),
    row('Risk summary', asString(storage.riskSummary), { tone: 'warning' }),
    row('Posture', asString(storage.postureSummary), {
      tone: storage.postureSummary ? 'warning' : 'default',
    }),
    row('Protection reduced', booleanValue(storage.protectionReduced), {
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
    row('Affected vdevs', summarizeList(zfsDeviceEvidence, 2), {
      title: zfsDeviceEvidence.join(', '),
      tone: 'warning',
    }),
    row('Reasons', summarizeList(riskReasons, 2), {
      title: riskReasons.join(', '),
      tone: 'warning',
    }),
    row('Recommended', asString(storage.poolHealth?.recommendation), {
      title: asString(storage.poolHealth?.summary) ?? undefined,
    }),
    row('Evidence', summarizeList(storage.poolHealth?.evidenceCodes ?? [], 4), {
      title: (storage.poolHealth?.evidenceCodes ?? []).join(', '),
    }),
    row('Evidence source', asString(storage.poolHealth?.source)),
  ]);

  return compactSections([
    { label: 'Storage', rows: storageRows },
    { label: 'Capacity', rows: capacityRows },
    { label: 'Health', rows: healthRows },
  ]);
};

const diskStateTone = (disk: ResourcePhysicalDiskMeta): ResourceDetailDrawerTrueNASRowTone => {
  const health = disk.health?.toLowerCase();
  if (health === 'passed' || health === 'healthy' || health === 'ok') return 'success';
  if (health === 'degraded' || health === 'failed' || health === 'warning') return 'warning';
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

const buildTrueNASDiskSections = (
  resource: Resource,
  disk: ResourcePhysicalDiskMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const riskReasons = (disk.risk?.reasons ?? [])
    .map((reason) => asString(reason.summary))
    .filter((value): value is string => Boolean(value));
  const identityRows = compactRows([
    row('Device', asString(disk.devPath) ?? asString(resource.name)),
    row('Model', asString(disk.model)),
    row('Serial', asString(disk.serial)),
    row('WWN', asString(disk.wwn)),
    row('Type', diskTypeLabel(disk.diskType)),
    row('Size', formatDetailBytesValue(disk.sizeBytes)),
  ]);

  const healthRows = compactRows([
    row('Health', normalizeDelimitedLabel(disk.health), { tone: diskStateTone(disk) }),
    row('Temperature', formatTemperature(disk.temperature), {
      tone: disk.temperature && disk.temperature >= 55 ? 'warning' : 'default',
    }),
    row(
      'Wearout',
      disk.wearout === undefined || disk.wearout < 0 ? null : formatPercent(disk.wearout),
    ),
    row('RPM', formatDetailIntegerValue(disk.rpm)),
    row('Role', normalizeDelimitedLabel(disk.storageRole)),
    row('Group', asString(disk.storageGroup) ?? asString(resource.parentName)),
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
    row('Reasons', summarizeList(riskReasons, 2), {
      title: riskReasons.join(', '),
      tone: 'warning',
    }),
  ]);

  return compactSections([
    { label: 'Disk', rows: identityRows },
    { label: 'Health', rows: healthRows },
    { label: 'SMART', rows: smartRows },
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
  const appRows = compactRows([
    row('State', normalizeDelimitedLabel(app.state), {
      tone: app.state?.toLowerCase() === 'running' ? 'success' : 'warning',
    }),
    row('Version', asString(app.humanVersion) ?? asString(app.version)),
    row('Containers', formatDetailIntegerValue(containerCount)),
    row('Custom app', yesNoValue(app.customApp)),
    row(
      'App updates',
      app.upgradeAvailable === undefined ? null : app.upgradeAvailable ? 'Available' : 'Current',
      {
        tone: app.upgradeAvailable ? 'warning' : 'success',
      },
    ),
    row(
      'Image updates',
      app.imageUpdatesAvailable === undefined
        ? null
        : app.imageUpdatesAvailable
          ? 'Available'
          : 'Current',
      { tone: app.imageUpdatesAvailable ? 'warning' : 'success' },
    ),
    row('Notes', asString(app.notes)),
  ]);

  const networkRows = compactRows([
    row('Host IPs', summarizeList(app.usedHostIps ?? []), {
      title: (app.usedHostIps ?? []).join(', '),
    }),
    row('Ports', summarizeList(portLabels), { title: portLabels.join(', ') }),
    row('Networks', summarizeList(networkLabels), { title: networkLabels.join(', ') }),
  ]);

  const storageRows = compactRows([
    row('Volumes', summarizeList(volumeLabels), { title: volumeLabels.join(', ') }),
    row('Images', summarizeList(imageLabels, 2), { title: imageLabels.join(', ') }),
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

const buildTrueNASVMSections = (
  vm: ResourceTrueNASVMMeta,
): ResourceDetailDrawerTrueNASSection[] => {
  const state = normalizeDelimitedLabel(vm.state);
  const domainState = normalizeDelimitedLabel(vm.domainState);
  const sameState = state?.toLowerCase() === domainState?.toLowerCase();
  const machine = [asString(vm.machineType), asString(vm.archType)].filter(Boolean).join(' / ');
  const computeRows = compactRows([
    row('State', state ?? domainState, {
      tone: (vm.state ?? vm.domainState)?.toLowerCase() === 'running' ? 'success' : 'warning',
    }),
    row('Domain state', sameState ? null : domainState),
    row('vCPU', formatVMCpu(vm)),
    row('Topology', formatVMTopology(vm)),
    row('Memory', formatDetailBytesValue(vm.memoryBytes)),
    row('Minimum memory', formatDetailBytesValue(vm.minMemoryBytes)),
    row('CPU mode', normalizeDelimitedLabel(vm.cpuMode)),
    row('CPU model', asString(vm.cpuModel)),
  ]);

  const runtimeRows = compactRows([
    row('Bootloader', asString(vm.bootloader)),
    row('Machine', machine),
    row('Process ID', formatDetailIntegerValue(vm.pid)),
    row('UUID', asString(vm.uuid)),
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

const shareStateTone = (share: ResourceTrueNASShareMeta): ResourceDetailDrawerTrueNASRowTone => {
  if (share.enabled === false || share.locked) return 'warning';
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
    row('Dataset', asString(share.dataset)),
    row('Path', asString(share.path)),
    row('Relative path', asString(share.relativePath)),
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
    row('Aliases', summarizeList(aliases), { title: aliases.join(', ') }),
    row('Hosts', summarizeList(hosts), { title: hosts.join(', ') }),
    row('Networks', summarizeList(networks), { title: networks.join(', ') }),
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
): ResourceDetailDrawerTrueNASSection[] => {
  if (resource.truenas?.share) return buildTrueNASShareSections(resource.truenas.share);
  if (resource.truenas?.vm) return buildTrueNASVMSections(resource.truenas.vm);
  if (resource.truenas?.app) return buildTrueNASAppSections(resource.truenas.app);
  if (isTrueNASScopedResource(resource) && resource.storage) {
    return buildTrueNASStorageSections(resource, resource.storage);
  }
  if (isTrueNASScopedResource(resource) && resource.physicalDisk) {
    return buildTrueNASDiskSections(resource, resource.physicalDisk);
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
    const summary = [
      normalizeDelimitedLabel(vm.state ?? vm.domainState),
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
      formatTemperature(disk.temperature),
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
