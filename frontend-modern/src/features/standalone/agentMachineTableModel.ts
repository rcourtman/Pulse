import type { ColumnDef } from '@/hooks/useColumnVisibility';
import type { HostDiskIO, HostRAIDArray, HostRAIDDevice } from '@/types/api';
import type { Resource } from '@/types/resource';
import { getPlatformTableFiniteMetric } from '@/features/platformPage/sharedPlatformPage';
import { normalizeDiskArray } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';

export type AgentMachineColumnId =
  | 'machine'
  | 'system'
  | 'agent'
  | 'cpu'
  | 'memory'
  | 'disk'
  | 'network'
  | 'diskio'
  | 'uptime'
  | 'temp'
  | 'lastSeen'
  | 'ip'
  | 'raid'
  | 'arch'
  | 'kernel'
  | 'actions';

export type AgentMachineSortKey =
  | 'name'
  | 'system'
  | 'agent'
  | 'cpu'
  | 'memory'
  | 'disk'
  | 'network'
  | 'diskio'
  | 'uptime'
  | 'temp'
  | 'lastSeen'
  | 'ip'
  | 'raid'
  | 'arch'
  | 'kernel';

export type AgentMachineSortDirection = 'asc' | 'desc';

export type AgentMachineColumn = ColumnDef & {
  id: AgentMachineColumnId;
  sortKey?: AgentMachineSortKey;
};

export const AGENT_MACHINE_COLUMNS: AgentMachineColumn[] = [
  { id: 'machine', label: 'Machine', kind: 'name', sortKey: 'name' },
  { id: 'system', label: 'System', kind: 'text', sortKey: 'system', toggleable: true },
  { id: 'agent', label: 'Agent', kind: 'text', sortKey: 'agent', toggleable: true },
  { id: 'cpu', label: 'CPU', kind: 'metric-bar', sortKey: 'cpu' },
  { id: 'memory', label: 'Memory', kind: 'metric-bar', sortKey: 'memory' },
  { id: 'disk', label: 'Disk', kind: 'metric-bar', sortKey: 'disk' },
  { id: 'network', label: 'Net I/O', kind: 'numeric-value', sortKey: 'network', toggleable: true },
  { id: 'diskio', label: 'Disk I/O', kind: 'numeric-value', sortKey: 'diskio', toggleable: true },
  { id: 'uptime', label: 'Uptime', kind: 'numeric-value', sortKey: 'uptime', toggleable: true },
  { id: 'temp', label: 'Temp', kind: 'numeric-value', sortKey: 'temp', toggleable: true },
  {
    id: 'lastSeen',
    label: 'Last seen',
    kind: 'numeric-value',
    sortKey: 'lastSeen',
    toggleable: true,
  },
  {
    id: 'ip',
    label: 'IP',
    kind: 'text',
    sortKey: 'ip',
    toggleable: true,
    defaultHidden: true,
  },
  {
    id: 'raid',
    label: 'RAID',
    kind: 'text',
    sortKey: 'raid',
    toggleable: true,
    defaultHidden: true,
  },
  {
    id: 'arch',
    label: 'Arch',
    kind: 'text',
    sortKey: 'arch',
    toggleable: true,
    defaultHidden: true,
  },
  {
    id: 'kernel',
    label: 'Kernel',
    kind: 'text',
    sortKey: 'kernel',
    toggleable: true,
    defaultHidden: true,
  },
  {
    id: 'actions',
    label: 'Actions',
    kind: 'badge',
  },
];

const AGENT_MACHINE_SORT_DESC_DEFAULTS = new Set<AgentMachineSortKey>([
  'cpu',
  'memory',
  'disk',
  'network',
  'diskio',
  'uptime',
  'temp',
  'lastSeen',
]);

type TemperatureReading = {
  label: string;
  value: number;
};

type SmartTemperatureReading = TemperatureReading & {
  standby?: boolean;
};

export type AgentMachineTemperatureDetailRow = {
  label: string;
  value: string;
  muted?: boolean;
};

export type AgentMachineTemperatureDetailSection = {
  heading: string;
  rows: AgentMachineTemperatureDetailRow[];
};

export type AgentMachineNetworkInterfaceDetail = {
  name: string;
  mac?: string;
  addresses: string[];
  rxBytes?: number;
  txBytes?: number;
  speedMbps?: number;
};

export type AgentMachineDiskIODetail = HostDiskIO;

export type AgentMachineRaidArrayDetail = HostRAIDArray;

const positiveTemperature = (value: number | undefined): number | undefined => {
  const metric = getPlatformTableFiniteMetric(value);
  return metric !== undefined && metric > 0 ? metric : undefined;
};

const maxTemperatureReading = (readings: readonly TemperatureReading[]): number | undefined =>
  readings.length > 0 ? Math.max(...readings.map((reading) => reading.value)) : undefined;

const getDiskUsagePercent = (disk: {
  total?: number;
  used?: number;
  usage?: number;
}): number | undefined => {
  const total = getPlatformTableFiniteMetric(disk.total);
  const used = getPlatformTableFiniteMetric(disk.used);
  if (total && total > 0 && typeof used === 'number') {
    return (used / total) * 100;
  }

  const usage = getPlatformTableFiniteMetric(disk.usage);
  if (usage === undefined) return undefined;
  return usage <= 1 ? usage * 100 : usage;
};

const getMaxOperationalDiskPercent = (machine: Resource): number | undefined => {
  const disks = normalizeDiskArray(machine.agent?.disks) ?? [];
  return disks.reduce<number | undefined>((maxPercent, disk) => {
    const percent = getDiskUsagePercent(disk);
    if (percent === undefined) return maxPercent;
    return maxPercent === undefined ? percent : Math.max(maxPercent, percent);
  }, undefined);
};

const getPositiveRecordReadings = (
  values: Record<string, number> | undefined,
): TemperatureReading[] =>
  Object.entries(values ?? {}).reduce<TemperatureReading[]>((readings, [label, value]) => {
    const temperature = positiveTemperature(value);
    if (temperature !== undefined) {
      readings.push({ label, value: temperature });
    }
    return readings;
  }, []);

const getSensorTemperatureReadings = (machine: Resource): TemperatureReading[] =>
  getPositiveRecordReadings(machine.agent?.sensors?.temperatureCelsius);

const getAdditionalTemperatureReadings = (machine: Resource): TemperatureReading[] =>
  getPositiveRecordReadings(machine.agent?.sensors?.additional);

const getFanReadings = (machine: Resource): TemperatureReading[] =>
  getPositiveRecordReadings(machine.agent?.sensors?.fanRpm);

const getSmartTemperatureReadings = (machine: Resource): SmartTemperatureReading[] =>
  (machine.agent?.sensors?.smart ?? []).reduce<SmartTemperatureReading[]>((readings, disk) => {
    const temperature = positiveTemperature(disk.temperature);
    if (!disk.standby && temperature === undefined) return readings;

    const device = asTrimmedString(disk.device) ?? 'disk';
    const model = asTrimmedString(disk.model);
    readings.push({
      label: model ? `${device} ${model}` : device,
      value: temperature ?? 0,
      standby: disk.standby,
    });
    return readings;
  }, []);

const getActiveSmartTemperatureReadings = (machine: Resource): TemperatureReading[] =>
  getSmartTemperatureReadings(machine).filter(
    (reading): reading is TemperatureReading => !reading.standby && reading.value > 0,
  );

const byHighestTemperature = (left: TemperatureReading, right: TemperatureReading): number =>
  right.value - left.value || left.label.localeCompare(right.label, undefined, { numeric: true });

const byLabel = (left: TemperatureReading, right: TemperatureReading): number =>
  left.label.localeCompare(right.label, undefined, { numeric: true });

const formatTemperatureValue = (reading: TemperatureReading): string =>
  `${Math.round(reading.value)}°C`;

const section = (
  heading: string,
  rows: AgentMachineTemperatureDetailRow[],
): AgentMachineTemperatureDetailSection[] => (rows.length > 0 ? [{ heading, rows }] : []);

const TEMPERATURE_SECTION_ROW_CAP = 6;

// Hover tooltips can't scroll, so each section is capped — but truncation must
// be visible, not silent (readings are sorted worst-first, so the cap drops
// the least alarming ones).
const capTemperatureRows = (
  rows: AgentMachineTemperatureDetailRow[],
): AgentMachineTemperatureDetailRow[] =>
  rows.length > TEMPERATURE_SECTION_ROW_CAP
    ? [
        ...rows.slice(0, TEMPERATURE_SECTION_ROW_CAP),
        {
          label: `+${rows.length - TEMPERATURE_SECTION_ROW_CAP} more`,
          value: '',
          muted: true,
        },
      ]
    : rows;

const flattenTemperatureSections = (
  sections: readonly AgentMachineTemperatureDetailSection[],
): string =>
  sections
    .flatMap((entry) => [
      entry.heading,
      ...entry.rows.map((row) => (row.value ? `${row.label}: ${row.value}` : row.label)),
    ])
    .join('\n');

const getMetricPercent = (metric: Resource['cpu'] | undefined): number | undefined =>
  getPlatformTableFiniteMetric(metric?.current);

export const getAgentMachineCpuPercent = (machine: Resource): number | undefined =>
  getMetricPercent(machine.cpu);

export const getAgentMachineMemoryPercent = (machine: Resource): number | undefined => {
  const total = getPlatformTableFiniteMetric(machine.memory?.total);
  const used = getPlatformTableFiniteMetric(machine.memory?.used);
  if (total && total > 0 && typeof used === 'number') {
    return (used / total) * 100;
  }
  return (
    getPlatformTableFiniteMetric(machine.memory?.current) ??
    getPlatformTableFiniteMetric(machine.agent?.memory?.usage)
  );
};

export const getAgentMachineDiskPercent = (machine: Resource): number | undefined => {
  const maxDiskPercent = getMaxOperationalDiskPercent(machine);
  if (maxDiskPercent !== undefined) return maxDiskPercent;

  const total = getPlatformTableFiniteMetric(machine.disk?.total);
  const used = getPlatformTableFiniteMetric(machine.disk?.used);
  if (total && total > 0 && typeof used === 'number') {
    return (used / total) * 100;
  }
  return getPlatformTableFiniteMetric(machine.disk?.current);
};

export const getAgentMachineNetworkTotal = (machine: Resource): number | undefined => {
  const rx = getPlatformTableFiniteMetric(machine.network?.rxBytes);
  const tx = getPlatformTableFiniteMetric(machine.network?.txBytes);
  if (rx === undefined && tx === undefined) return undefined;
  return (rx ?? 0) + (tx ?? 0);
};

const positiveMetric = (value: number | undefined): number | undefined => {
  const metric = getPlatformTableFiniteMetric(value);
  return metric !== undefined && metric > 0 ? metric : undefined;
};

const nonNegativeMetric = (value: number | undefined): number => {
  const metric = getPlatformTableFiniteMetric(value);
  return metric !== undefined && metric > 0 ? metric : 0;
};

const nonNegativeFiniteMetric = (value: number | undefined): number | undefined => {
  const metric = getPlatformTableFiniteMetric(value);
  return metric !== undefined && metric >= 0 ? metric : undefined;
};

const uniqueTrimmedValues = (values: readonly string[] | undefined): string[] => {
  const seen = new Set<string>();
  const result: string[] = [];

  for (const candidate of values ?? []) {
    const value = asTrimmedString(candidate);
    if (!value || seen.has(value)) continue;
    seen.add(value);
    result.push(value);
  }

  return result;
};

const searchableValue = (value: unknown): string | undefined => {
  if (typeof value === 'string') return asTrimmedString(value);
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return undefined;
};

const appendSearchValue = (values: string[], value: unknown) => {
  const normalized = searchableValue(value);
  if (normalized) values.push(normalized);
};

const appendSearchValues = (values: string[], candidates: readonly unknown[]) => {
  for (const candidate of candidates) appendSearchValue(values, candidate);
};

const sensorSearchValues = (machine: Resource): unknown[] => [
  ...Object.keys(machine.agent?.sensors?.temperatureCelsius ?? {}),
  ...Object.keys(machine.agent?.sensors?.fanRpm ?? {}),
  ...Object.keys(machine.agent?.sensors?.additional ?? {}),
  ...(machine.agent?.sensors?.smart ?? []).flatMap((disk) => [
    disk.device,
    disk.model,
    disk.serial,
    disk.wwn,
    disk.type,
    disk.health,
    disk.standby ? 'standby' : undefined,
  ]),
];

export const getAgentMachineNetworkInterfaceDetails = (
  machine: Resource,
): AgentMachineNetworkInterfaceDetail[] =>
  (machine.agent?.networkInterfaces ?? []).reduce<AgentMachineNetworkInterfaceDetail[]>(
    (details, iface, index) => {
      const name = asTrimmedString(iface.name);
      const mac = asTrimmedString(iface.mac);
      const addresses = uniqueTrimmedValues(iface.addresses);
      const rxBytes = getPlatformTableFiniteMetric(iface.rxBytes);
      const txBytes = getPlatformTableFiniteMetric(iface.txBytes);
      const speedMbps = positiveMetric(iface.speedMbps);

      if (
        !name &&
        !mac &&
        addresses.length === 0 &&
        rxBytes === undefined &&
        txBytes === undefined &&
        speedMbps === undefined
      ) {
        return details;
      }

      details.push({
        name: name ?? `eth${index}`,
        ...(mac ? { mac } : {}),
        addresses,
        ...(rxBytes !== undefined ? { rxBytes } : {}),
        ...(txBytes !== undefined ? { txBytes } : {}),
        ...(speedMbps !== undefined ? { speedMbps } : {}),
      });
      return details;
    },
    [],
  );

export const getAgentMachineDiskIOTotal = (machine: Resource): number | undefined => {
  const read = getPlatformTableFiniteMetric(machine.diskIO?.readRate);
  const write = getPlatformTableFiniteMetric(machine.diskIO?.writeRate);
  if (read === undefined && write === undefined) return undefined;
  return (read ?? 0) + (write ?? 0);
};

export const getAgentMachineDiskIODetails = (machine: Resource): AgentMachineDiskIODetail[] => {
  const diskIO = machine.agent?.diskIO ?? machine.agent?.diskIo ?? [];
  return diskIO.reduce<AgentMachineDiskIODetail[]>((details, disk, index) => {
    const device = asTrimmedString(disk.device);
    const readBytes = nonNegativeFiniteMetric(disk.readBytes);
    const writeBytes = nonNegativeFiniteMetric(disk.writeBytes);
    const readOps = nonNegativeFiniteMetric(disk.readOps);
    const writeOps = nonNegativeFiniteMetric(disk.writeOps);
    const readTimeMs = nonNegativeFiniteMetric(disk.readTimeMs);
    const writeTimeMs = nonNegativeFiniteMetric(disk.writeTimeMs);
    const ioTimeMs = nonNegativeFiniteMetric(disk.ioTimeMs);

    if (
      !device &&
      readBytes === undefined &&
      writeBytes === undefined &&
      readOps === undefined &&
      writeOps === undefined &&
      readTimeMs === undefined &&
      writeTimeMs === undefined &&
      ioTimeMs === undefined
    ) {
      return details;
    }

    details.push({
      device: device ?? `disk-${index + 1}`,
      ...(readBytes !== undefined ? { readBytes } : {}),
      ...(writeBytes !== undefined ? { writeBytes } : {}),
      ...(readOps !== undefined ? { readOps } : {}),
      ...(writeOps !== undefined ? { writeOps } : {}),
      ...(readTimeMs !== undefined ? { readTimeMs } : {}),
      ...(writeTimeMs !== undefined ? { writeTimeMs } : {}),
      ...(ioTimeMs !== undefined ? { ioTimeMs } : {}),
    });
    return details;
  }, []);
};

export const getAgentMachineTemperatureCelsius = (machine: Resource): number | undefined => {
  const direct = positiveTemperature(machine.temperature);
  if (direct !== undefined) return direct;

  return (
    maxTemperatureReading(getSensorTemperatureReadings(machine)) ??
    maxTemperatureReading(getActiveSmartTemperatureReadings(machine))
  );
};

export const getAgentMachineTemperatureDetailSections = (
  machine: Resource,
): AgentMachineTemperatureDetailSection[] => {
  const sensorReadings = getSensorTemperatureReadings(machine)
    .sort(byHighestTemperature)
    .map((reading) => ({
      label: reading.label,
      value: formatTemperatureValue(reading),
    }));
  const activeSmartReadings = getActiveSmartTemperatureReadings(machine)
    .sort(byHighestTemperature)
    .map((reading) => ({
      label: `Disk ${reading.label}`,
      value: formatTemperatureValue(reading),
    }));
  const standbySmartReadings = getSmartTemperatureReadings(machine)
    .filter((reading) => reading.standby)
    .sort(byLabel)
    .map((reading) => ({
      label: `Disk ${reading.label}`,
      value: 'standby',
      muted: true,
    }));
  const fanReadings = getFanReadings(machine)
    .sort(byLabel)
    .map((reading) => ({
      label: reading.label,
      value: `${Math.round(reading.value)} RPM`,
    }));
  const additionalReadings = getAdditionalTemperatureReadings(machine)
    .sort(byHighestTemperature)
    .map((reading) => ({
      label: reading.label,
      value: formatTemperatureValue(reading),
    }));
  return [
    ...section('Temperatures', capTemperatureRows(sensorReadings)),
    ...section(
      'Disk Temperatures',
      capTemperatureRows([...activeSmartReadings, ...standbySmartReadings]),
    ),
    ...section('Fan Speeds', capTemperatureRows(fanReadings)),
    ...section('Other Sensors', capTemperatureRows(additionalReadings)),
  ];
};

export const getAgentMachineTemperatureTitle = (machine: Resource): string =>
  flattenTemperatureSections(getAgentMachineTemperatureDetailSections(machine));

export const timestampMillisFrom = (
  value: number | string | Date | undefined,
): number | undefined => {
  if (value instanceof Date) {
    const millis = value.getTime();
    return Number.isFinite(millis) ? millis : undefined;
  }
  if (typeof value === 'string') {
    const millis = Date.parse(value);
    return Number.isFinite(millis) ? millis : undefined;
  }
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return undefined;
  return value < 10_000_000_000 ? value * 1000 : value;
};

export const getAgentMachineIpValues = (machine: Resource): string[] => {
  const candidates = [
    ...(machine.identity?.ips ?? []),
    ...(machine.agent?.networkInterfaces ?? []).flatMap((iface) => iface.addresses ?? []),
  ];
  return uniqueTrimmedValues(candidates);
};

export const getAgentMachinePrimaryIp = (machine: Resource): string =>
  getAgentMachineIpValues(machine)[0] ?? '';

const getAgentMachineRaidDeviceDetails = (
  devices: readonly HostRAIDDevice[] | undefined,
): HostRAIDDevice[] =>
  (devices ?? []).reduce<HostRAIDDevice[]>((details, device, index) => {
    const deviceName = asTrimmedString(device.device);
    const state = asTrimmedString(device.state);
    const slot = getPlatformTableFiniteMetric(device.slot);

    if (!deviceName && !state && slot === undefined) return details;

    details.push({
      device: deviceName ?? `disk-${index + 1}`,
      state: state ?? 'unknown',
      slot: slot !== undefined ? Math.round(slot) : index,
    });
    return details;
  }, []);

export const getAgentMachineRaidArrayDetails = (machine: Resource): AgentMachineRaidArrayDetail[] =>
  (machine.agent?.raid ?? []).reduce<AgentMachineRaidArrayDetail[]>((details, array, index) => {
    const device = asTrimmedString(array.device);
    const name = asTrimmedString(array.name);
    const level = asTrimmedString(array.level);
    const state = asTrimmedString(array.state);
    const devices = getAgentMachineRaidDeviceDetails(array.devices);
    const counts = [
      array.totalDevices,
      array.activeDevices,
      array.workingDevices,
      array.failedDevices,
      array.spareDevices,
    ].map(nonNegativeMetric);
    const rebuildPercent = nonNegativeMetric(array.rebuildPercent);
    const rebuildSpeed = asTrimmedString(array.rebuildSpeed);

    if (
      !device &&
      !name &&
      !level &&
      !state &&
      devices.length === 0 &&
      counts.every((count) => count === 0) &&
      rebuildPercent === 0
    ) {
      return details;
    }

    details.push({
      device: device ?? name ?? `array-${index + 1}`,
      ...(name ? { name } : {}),
      level: level ?? 'unknown',
      state: state ?? 'unknown',
      totalDevices: counts[0],
      activeDevices: counts[1],
      workingDevices: counts[2],
      failedDevices: counts[3],
      spareDevices: counts[4],
      devices,
      rebuildPercent,
      ...(rebuildSpeed ? { rebuildSpeed } : {}),
    });
    return details;
  }, []);

export const getAgentMachineRaidSummary = (machine: Resource): string => {
  const arrays = getAgentMachineRaidArrayDetails(machine);
  if (arrays.length === 0) return '';

  const failed = arrays.filter((array) => (array.failedDevices ?? 0) > 0).length;
  if (failed > 0) {
    return `${failed}/${arrays.length} degraded`;
  }

  const rebuilding = arrays.filter((array) => (array.rebuildPercent ?? 0) > 0).length;
  if (rebuilding > 0) {
    return `${rebuilding}/${arrays.length} rebuilding`;
  }

  const states = new Set(
    arrays
      .map((array) => asTrimmedString(array.state)?.toLowerCase())
      .filter((state): state is string => Boolean(state)),
  );
  if (states.size === 1) {
    return `${arrays.length} ${[...states][0]}`;
  }
  return `${arrays.length} arrays`;
};

export const matchesAgentMachineSearch = (
  machine: Resource,
  search: string,
  getSystemLabel: (machine: Resource) => string,
  getAgentLabel: (machine: Resource) => string,
): boolean => {
  const needle = search.trim().toLowerCase();
  if (!needle) return true;

  const values: string[] = [];
  appendSearchValues(values, [
    machine.name,
    machine.displayName,
    machine.id,
    machine.parentName,
    machine.platformId,
    machine.platformType,
    machine.status,
    machine.technology,
    machine.identity?.hostname,
    machine.identity?.machineId,
    machine.canonicalIdentity?.displayName,
    machine.canonicalIdentity?.hostname,
    machine.canonicalIdentity?.platformId,
    machine.canonicalIdentity?.primaryId,
    ...(machine.canonicalIdentity?.aliases ?? []),
    ...(machine.identity?.ips ?? []),
    ...(machine.tags ?? []),
    getSystemLabel(machine),
    getAgentLabel(machine),
    machine.agent?.agentId,
    machine.agent?.agentVersion,
    machine.agent?.hostname,
    machine.agent?.platform,
    machine.agent?.hostProfile,
    machine.agent?.osName,
    machine.agent?.osVersion,
    machine.agent?.kernelVersion,
    machine.agent?.architecture,
    machine.agent?.uptimeSeconds,
    machine.agent?.cpuCount,
  ]);
  appendSearchValues(values, getAgentMachineIpValues(machine));
  appendSearchValues(
    values,
    getAgentMachineNetworkInterfaceDetails(machine).flatMap((iface) => [
      iface.name,
      iface.mac,
      ...iface.addresses,
      iface.speedMbps,
    ]),
  );
  appendSearchValues(
    values,
    getAgentMachineDiskIODetails(machine).map((disk) => disk.device),
  );
  appendSearchValues(
    values,
    getAgentMachineRaidArrayDetails(machine).flatMap((array) => [
      array.device,
      array.name,
      array.level,
      array.state,
      array.rebuildSpeed,
      ...array.devices.flatMap((device) => [device.device, device.state, device.slot]),
    ]),
  );
  appendSearchValues(values, [getAgentMachineRaidSummary(machine), ...sensorSearchValues(machine)]);

  return values.join(' ').toLowerCase().includes(needle);
};

export const getNextAgentMachineSortState = (
  currentKey: AgentMachineSortKey,
  currentDirection: AgentMachineSortDirection,
  nextKey: AgentMachineSortKey,
): { key: AgentMachineSortKey; direction: AgentMachineSortDirection } => {
  if (currentKey === nextKey) {
    return { key: nextKey, direction: currentDirection === 'asc' ? 'desc' : 'asc' };
  }

  return {
    key: nextKey,
    direction: AGENT_MACHINE_SORT_DESC_DEFAULTS.has(nextKey) ? 'desc' : 'asc',
  };
};

const compareNullableNumber = (
  left: number | undefined,
  right: number | undefined,
  direction: AgentMachineSortDirection,
): number => {
  const leftMissing = left === undefined;
  const rightMissing = right === undefined;
  if (leftMissing && rightMissing) return 0;
  if (leftMissing) return 1;
  if (rightMissing) return -1;
  return direction === 'asc' ? left - right : right - left;
};

const compareText = (
  left: string | undefined,
  right: string | undefined,
  direction: AgentMachineSortDirection,
): number => {
  const leftValue = (left ?? '').trim().toLowerCase();
  const rightValue = (right ?? '').trim().toLowerCase();
  if (!leftValue && !rightValue) return 0;
  if (!leftValue) return 1;
  if (!rightValue) return -1;
  const result = leftValue.localeCompare(rightValue, undefined, { numeric: true });
  return direction === 'asc' ? result : -result;
};

export const sortAgentMachines = (
  machines: readonly Resource[],
  sortKey: AgentMachineSortKey,
  direction: AgentMachineSortDirection,
  getSystemLabel: (machine: Resource) => string,
  getAgentLabel: (machine: Resource) => string,
): Resource[] =>
  [...machines].sort((left, right) => {
    let result = 0;
    switch (sortKey) {
      case 'cpu':
        result = compareNullableNumber(
          getAgentMachineCpuPercent(left),
          getAgentMachineCpuPercent(right),
          direction,
        );
        break;
      case 'memory':
        result = compareNullableNumber(
          getAgentMachineMemoryPercent(left),
          getAgentMachineMemoryPercent(right),
          direction,
        );
        break;
      case 'disk':
        result = compareNullableNumber(
          getAgentMachineDiskPercent(left),
          getAgentMachineDiskPercent(right),
          direction,
        );
        break;
      case 'network':
        result = compareNullableNumber(
          getAgentMachineNetworkTotal(left),
          getAgentMachineNetworkTotal(right),
          direction,
        );
        break;
      case 'diskio':
        result = compareNullableNumber(
          getAgentMachineDiskIOTotal(left),
          getAgentMachineDiskIOTotal(right),
          direction,
        );
        break;
      case 'uptime':
        result = compareNullableNumber(
          left.uptime ?? left.agent?.uptimeSeconds,
          right.uptime ?? right.agent?.uptimeSeconds,
          direction,
        );
        break;
      case 'temp':
        result = compareNullableNumber(
          getAgentMachineTemperatureCelsius(left),
          getAgentMachineTemperatureCelsius(right),
          direction,
        );
        break;
      case 'lastSeen':
        result = compareNullableNumber(
          timestampMillisFrom(left.lastSeen),
          timestampMillisFrom(right.lastSeen),
          direction,
        );
        break;
      case 'system':
        result = compareText(getSystemLabel(left), getSystemLabel(right), direction);
        break;
      case 'agent':
        result = compareText(getAgentLabel(left), getAgentLabel(right), direction);
        break;
      case 'ip':
        result = compareText(
          getAgentMachinePrimaryIp(left),
          getAgentMachinePrimaryIp(right),
          direction,
        );
        break;
      case 'raid':
        result = compareText(
          getAgentMachineRaidSummary(left),
          getAgentMachineRaidSummary(right),
          direction,
        );
        break;
      case 'arch':
        result = compareText(left.agent?.architecture, right.agent?.architecture, direction);
        break;
      case 'kernel':
        result = compareText(left.agent?.kernelVersion, right.agent?.kernelVersion, direction);
        break;
      case 'name':
      default:
        result = compareText(
          left.displayName || left.name || left.id,
          right.displayName || right.name || right.id,
          direction,
        );
        break;
    }

    if (result !== 0) return result;
    return compareText(
      left.displayName || left.name || left.id,
      right.displayName || right.name || right.id,
      'asc',
    );
  });
