import type {
  Resource,
  ResourceTrueNASAppMeta,
  ResourceTrueNASAppPort,
  ResourceTrueNASShareMeta,
  ResourceTrueNASVMMeta,
} from '@/types/resource';

export type ResourceDetailDrawerTrueNASRowTone = 'default' | 'accent' | 'warning' | 'success';

export type ResourceDetailDrawerTrueNASRow = {
  label: string;
  value: string;
  title?: string;
  tone?: ResourceDetailDrawerTrueNASRowTone;
};

export type ResourceDetailDrawerTrueNASSection = {
  label: string;
  rows: ResourceDetailDrawerTrueNASRow[];
};

const asString = (value?: string | null): string | null => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
};

const asPositiveNumber = (value?: number): number | null =>
  typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : null;

const formatInteger = (value?: number): string | null => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null;
  return new Intl.NumberFormat().format(Math.round(value));
};

const normalizeDelimitedLabel = (value?: string): string | null => {
  const trimmed = asString(value);
  if (!trimmed) return null;
  return trimmed
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');
};

const formatBytes = (bytes?: number): string | null => {
  const value = asPositiveNumber(bytes);
  if (!value) return null;
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let scaled = value;
  let unitIndex = 0;
  while (scaled >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024;
    unitIndex += 1;
  }
  return `${scaled.toFixed(scaled >= 100 ? 0 : scaled >= 10 ? 1 : 2)} ${units[unitIndex]}`;
};

const formatCount = (value: number, singular: string, plural = `${singular}s`): string =>
  `${new Intl.NumberFormat().format(value)} ${value === 1 ? singular : plural}`;

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

const row = (
  label: string,
  value: string | null | undefined,
  options: Pick<ResourceDetailDrawerTrueNASRow, 'title' | 'tone'> = {},
): ResourceDetailDrawerTrueNASRow | null => {
  const trimmed = value?.trim();
  if (!trimmed) return null;
  return { label, value: trimmed, ...options };
};

const compactRows = (
  rows: Array<ResourceDetailDrawerTrueNASRow | null>,
): ResourceDetailDrawerTrueNASRow[] =>
  rows.filter((entry): entry is ResourceDetailDrawerTrueNASRow => Boolean(entry));

const compactSections = (
  sections: Array<ResourceDetailDrawerTrueNASSection | null>,
): ResourceDetailDrawerTrueNASSection[] =>
  sections.filter((section): section is ResourceDetailDrawerTrueNASSection =>
    Boolean(section && section.rows.length > 0),
  );

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
    row('Containers', formatInteger(containerCount)),
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
  if (vcpus) return formatCount(vcpus, 'vCPU', 'vCPU');
  const cores = asPositiveNumber(vm.cores);
  const threads = asPositiveNumber(vm.threads);
  if (cores && threads) return `${cores} cores x ${threads} threads`;
  if (cores) return formatCount(cores, 'core');
  if (threads) return formatCount(threads, 'thread');
  return null;
};

const formatVMTopology = (vm: ResourceTrueNASVMMeta): string | null => {
  const cores = asPositiveNumber(vm.cores);
  const threads = asPositiveNumber(vm.threads);
  if (cores && threads) return `${cores} cores x ${threads} threads`;
  if (cores) return formatCount(cores, 'core');
  if (threads) return formatCount(threads, 'thread');
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
    row('Memory', formatBytes(vm.memoryBytes)),
    row('Minimum memory', formatBytes(vm.minMemoryBytes)),
    row('CPU mode', normalizeDelimitedLabel(vm.cpuMode)),
    row('CPU model', asString(vm.cpuModel)),
  ]);

  const runtimeRows = compactRows([
    row('Bootloader', asString(vm.bootloader)),
    row('Machine', machine),
    row('Process ID', formatInteger(vm.pid)),
    row('UUID', asString(vm.uuid)),
  ]);

  const deviceRows = compactRows([
    row('Total', formatInteger(vm.deviceCount)),
    row('Disks', formatInteger(vm.diskCount)),
    row('NICs', formatInteger(vm.nicCount)),
    row('Displays', formatInteger(vm.displayCount)),
    row('CD-ROMs', formatInteger(vm.cdromCount)),
    row('USB', formatInteger(vm.usbCount)),
    row('PCI', formatInteger(vm.pciCount)),
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
      formatBytes(vm.memoryBytes),
      deviceCount ? formatCount(deviceCount, 'device') : null,
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  const app = resource.truenas?.app;
  if (app) {
    const portCount = appPortLabels(app).length;
    const updateCount = [app.upgradeAvailable, app.imageUpdatesAvailable].filter(Boolean).length;
    const summary = [
      normalizeDelimitedLabel(app.state),
      formatCount(app.containerCount ?? app.containers?.length ?? 0, 'container'),
      formatCount(portCount, 'port'),
      updateCount > 0 ? formatCount(updateCount, 'update') : null,
    ].filter((value): value is string => Boolean(value));
    return summary.length > 0 ? summary.join(', ') : null;
  }

  return null;
};
