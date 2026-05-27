import type { ColumnDef } from '@/hooks/useColumnVisibility';
import type { HostDiskIO, HostRAIDArray, HostRAIDDevice } from '@/types/api';
import type { Resource } from '@/types/resource';
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
  | 'kernel';

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

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

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
  const metric = finiteMetric(value);
  return metric !== undefined && metric > 0 ? metric : undefined;
};

const maxTemperatureReading = (readings: readonly TemperatureReading[]): number | undefined =>
  readings.length > 0 ? Math.max(...readings.map((reading) => reading.value)) : undefined;

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

const flattenTemperatureSections = (
  sections: readonly AgentMachineTemperatureDetailSection[],
): string =>
  sections
    .flatMap((entry) => [entry.heading, ...entry.rows.map((row) => `${row.label}: ${row.value}`)])
    .join('\n');

const getMetricPercent = (metric: Resource['cpu'] | undefined): number | undefined =>
  finiteMetric(metric?.current);

export const getAgentMachineCpuPercent = (machine: Resource): number | undefined =>
  getMetricPercent(machine.cpu);

export const getAgentMachineMemoryPercent = (machine: Resource): number | undefined => {
  const total = finiteMetric(machine.memory?.total);
  const used = finiteMetric(machine.memory?.used);
  if (total && total > 0 && typeof used === 'number') {
    return (used / total) * 100;
  }
  return finiteMetric(machine.memory?.current) ?? finiteMetric(machine.agent?.memory?.usage);
};

export const getAgentMachineDiskPercent = (machine: Resource): number | undefined => {
  const total = finiteMetric(machine.disk?.total);
  const used = finiteMetric(machine.disk?.used);
  if (total && total > 0 && typeof used === 'number') {
    return (used / total) * 100;
  }
  return finiteMetric(machine.disk?.current);
};

export const getAgentMachineNetworkTotal = (machine: Resource): number | undefined => {
  const rx = finiteMetric(machine.network?.rxBytes);
  const tx = finiteMetric(machine.network?.txBytes);
  if (rx === undefined && tx === undefined) return undefined;
  return (rx ?? 0) + (tx ?? 0);
};

const positiveMetric = (value: number | undefined): number | undefined => {
  const metric = finiteMetric(value);
  return metric !== undefined && metric > 0 ? metric : undefined;
};

const nonNegativeMetric = (value: number | undefined): number => {
  const metric = finiteMetric(value);
  return metric !== undefined && metric > 0 ? metric : 0;
};

const nonNegativeFiniteMetric = (value: number | undefined): number | undefined => {
  const metric = finiteMetric(value);
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

export const getAgentMachineNetworkInterfaceDetails = (
  machine: Resource,
): AgentMachineNetworkInterfaceDetail[] =>
  (machine.agent?.networkInterfaces ?? []).reduce<AgentMachineNetworkInterfaceDetail[]>(
    (details, iface, index) => {
      const name = asTrimmedString(iface.name);
      const mac = asTrimmedString(iface.mac);
      const addresses = uniqueTrimmedValues(iface.addresses);
      const rxBytes = finiteMetric(iface.rxBytes);
      const txBytes = finiteMetric(iface.txBytes);
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
  const read = finiteMetric(machine.diskIO?.readRate);
  const write = finiteMetric(machine.diskIO?.writeRate);
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
    .slice(0, 6)
    .map((reading) => ({
      label: reading.label,
      value: formatTemperatureValue(reading),
    }));
  const activeSmartReadings = getActiveSmartTemperatureReadings(machine)
    .sort(byHighestTemperature)
    .slice(0, 6)
    .map((reading) => ({
      label: `Disk ${reading.label}`,
      value: formatTemperatureValue(reading),
    }));
  const standbySmartReadings = getSmartTemperatureReadings(machine)
    .filter((reading) => reading.standby)
    .sort(byLabel)
    .slice(0, 6)
    .map((reading) => ({
      label: `Disk ${reading.label}`,
      value: 'standby',
      muted: true,
    }));
  const fanReadings = getFanReadings(machine)
    .sort(byLabel)
    .slice(0, 6)
    .map((reading) => ({
      label: reading.label,
      value: `${Math.round(reading.value)} RPM`,
    }));
  const additionalReadings = getAdditionalTemperatureReadings(machine)
    .sort(byHighestTemperature)
    .slice(0, 6)
    .map((reading) => ({
      label: reading.label,
      value: formatTemperatureValue(reading),
    }));
  return [
    ...section('Temperatures', sensorReadings),
    ...section('Disk Temperatures', [...activeSmartReadings, ...standbySmartReadings]),
    ...section('Fan Speeds', fanReadings),
    ...section('Other Sensors', additionalReadings),
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
    const slot = finiteMetric(device.slot);

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
