import type { Resource, ResourceType } from '@/types/resource';
import { getDiskPercent, getMemoryPercent } from '@/types/resource';
import { formatBytes } from '@/utils/format';
import type { AvailabilityTargetKind } from '@/api/availabilityTargets';
import { normalizeAvailabilityTargetKind } from '@/utils/availabilityTargetKinds';
import {
  resolveMapApp,
  resolveMapCategory,
  type MapApp,
  type MapCategoryId,
} from './infrastructureMapCatalogue';
import { resolveMapConnections, type MapConnections } from './infrastructureMapConnections';
import {
  resolveMapNetworks,
  type MapAttachment,
  type MapSegment,
} from './infrastructureMapNetworks';

/**
 * The infrastructure map draws what Pulse already monitors, and nothing else:
 * every box is a resource in the unified registry and every nesting is a
 * parent the registry reports. There are no hand-placed devices and no links
 * inferred from names, so the map cannot drift from the estate it shows.
 */

export type MapHealth = 'ok' | 'attention' | 'critical' | 'off' | 'stale' | 'unknown';

export type MapTrafficTier = 'low' | 'medium' | 'high';

/** Minimum combined throughput (10 KB/s) before a tile or network counts as active. */
export const MAP_TRAFFIC_ACTIVE_BYTES = 10 * 1024;

/** Minimum combined disk I/O rate (256 KB/s) before a workload counts as disk-active. */
export const MAP_DISK_ACTIVE_BYTES = 256 * 1024;

/** Maximum age (2 minutes) before a real-timestamped telemetry sample expires from live indicators. */
export const MAP_LIVE_SAMPLE_MAX_AGE_MS = 2 * 60 * 1000;

const REAL_EPOCH_MS_FLOOR = 1_000_000_000_000;

/** Full-duplex peak utilization threshold (%) at which a physical link is flagged as busy. */
export const MAP_LINK_SATURATED_PERCENT = 80;

export const computeMapLinkUtilizationPercent = (
  rxBytes: number | undefined,
  txBytes: number | undefined,
  speedMbps: number | undefined,
): number | undefined => {
  const safeRx =
    typeof rxBytes === 'number' && Number.isFinite(rxBytes) && rxBytes >= 0 ? rxBytes : 0;
  const safeTx =
    typeof txBytes === 'number' && Number.isFinite(txBytes) && txBytes >= 0 ? txBytes : 0;
  const peakBytes = Math.max(safeRx, safeTx);
  if (
    typeof speedMbps !== 'number' ||
    !Number.isFinite(speedMbps) ||
    !(speedMbps > 0) ||
    !(peakBytes > 0)
  ) {
    return undefined;
  }
  const pct = ((peakBytes * 8) / (speedMbps * 1_000_000)) * 100;
  if (!Number.isFinite(pct) || pct < 1) return undefined;
  return Math.min(100, Math.round(pct));
};

export const resolveMapTrafficTier = (
  bytesPerSec: number | undefined,
): MapTrafficTier | undefined => {
  if (typeof bytesPerSec !== 'number' || !Number.isFinite(bytesPerSec)) return undefined;
  if (bytesPerSec < MAP_TRAFFIC_ACTIVE_BYTES) return undefined;
  if (bytesPerSec < 1024 * 1024) return 'low';
  if (bytesPerSec < 25 * 1024 * 1024) return 'medium';
  return 'high';
};

export const resolveMapDiskTier = (bytesPerSec: number | undefined): MapTrafficTier | undefined => {
  if (typeof bytesPerSec !== 'number' || !Number.isFinite(bytesPerSec)) return undefined;
  if (bytesPerSec < MAP_DISK_ACTIVE_BYTES) return undefined;
  if (bytesPerSec < 2 * 1024 * 1024) return 'low';
  if (bytesPerSec < 20 * 1024 * 1024) return 'medium';
  return 'high';
};

export type MapPlatform =
  | 'proxmox'
  | 'vmware'
  | 'kubernetes'
  | 'docker'
  | 'truenas'
  | 'backup'
  | 'mail'
  | 'machine'
  | 'storage'
  | 'checks';

export interface MapItem {
  id: string;
  name: string;
  typeLabel: string;
  health: MapHealth;
  status: string;
  /** Proxmox VM or LXC numeric identifier, when known. */
  vmid?: number;
  /** Docker Compose project name, when an app container belongs to a Compose stack. */
  composeProject?: string;
  cpu?: number;
  memory?: number;
  /** Live inbound network rate in bytes per second, when collected. */
  netRxBytes?: number;
  /** Live outbound network rate in bytes per second, when collected. */
  netTxBytes?: number;
  /** Live disk read rate in bytes per second, when collected. */
  diskReadBytes?: number;
  /** Live disk write rate in bytes per second, when collected. */
  diskWriteBytes?: number;
  /** Live uptime in whole seconds when the resource is running or online. */
  uptimeSeconds?: number;
  /** Count of pending package or image updates reported for the resource, when positive. */
  pendingUpdates?: number;
  /** Valid ISO timestamp of the latest completed backup, when reported. */
  lastBackupAt?: string;
  /** Availability probe latency in milliseconds, when the check is reachable. */
  latencyMs?: number;
  /** Fastest reported physical link speed in Mbps across non-host-local interfaces. */
  linkSpeedMbps?: number;
  /** Full-duplex peak link utilization percentage (1 to 100) when speed and live rate are known. */
  linkUtilizationPercent?: number;
  /** Number of non-host-local interfaces currently reporting link down. */
  linkDownCount?: number;
  /** True when a Proxmox guest has firewall enabled on any network interface. */
  firewallEnabled?: boolean;
  /** The resource an availability check is explicitly linked to, when set. */
  linkedResourceId?: string;
  /** Sorted unique user-defined Docker networks a container is attached to. */
  dockerNetworks?: string[];
  /** Sorted unique lowercase Kubernetes Ingress hostnames routing to a pod via a matching Service. */
  ingressHosts?: string[];
  /** Sorted unique hardware accelerators attached to this workload (e.g. `GPU`, `iGPU`, `TPU`). */
  accelerators?: string[];
  /** Concise factual summary of an availability check probe (e.g. `HTTPS 200`, `TCP :443`). */
  probeSummary?: string;
  /** Concise TLS certificate warning for an availability check, when unhealthy or expiring. */
  certWarning?: string;
  /** The address the item answers on, when Pulse knows one. */
  address?: string;
  /** Published ports, for a container. */
  detail?: string;
  /** The networks the item is on, as ids into the model's segments. */
  segmentIds?: string[];
  /** The well-known app this guest or container is, when the map knows it. */
  app?: MapApp;
  /** The group a guest or container is drawn in on the service diagram. */
  category?: MapCategoryId;
  /**
   * What an availability check is on, as its owner typed it. The map reads
   * the kind and never works one out: a router, switch or access point is
   * drawn as that device, and every other kind as a check.
   */
  checkKind?: AvailabilityTargetKind;
  /**
   * The filesystems a guest mounts from outside itself, by size only: a
   * share or a directory of its host, never a disk of its own.
   */
  mounts?: MapMount[];
}

/** A filesystem as the guest that mounts it reports it, in bytes. */
export interface MapMount {
  total: number;
  used: number;
}

export interface MapSection {
  key: string;
  label: string;
  items: MapItem[];
}

export interface MapPool {
  id: string;
  name: string;
  health: MapHealth;
  usedPercent?: number;
  sizeLabel?: string;
  /** ZFS or storage vdev layout for display (e.g. `mirror`, `raidz2`). */
  vdevLayout?: string;
  /** Storage driver or backend type (e.g. `zfspool`, `nfs`, `pbs`). */
  storageType?: string;
  /** Uppercase degraded or faulted ZFS/array state, omitted when healthy. */
  zfsState?: string;
  /** True when the pool or array is actively resilvering, rebuilding, reconstructing, or recovering. */
  rebuildInProgress?: boolean;
  /** Sum of positive ZFS read, write, and checksum error counts when greater than zero. */
  zfsErrorCount?: number;
  /** A place backups are written to, as opposed to one guests run from. */
  backup?: boolean;
  totalBytes?: number;
  usedBytes?: number;
  /** The hosts storage shared across a cluster is set up on, as host ids. */
  hostIds?: string[];
  /** The guests the platform reports with a disk on this pool. */
  consumerIds?: string[];
}

export interface MapHost {
  id: string;
  /** False for the holders the map composes itself (shared storage, checks). */
  isResource: boolean;
  name: string;
  platform: MapPlatform;
  typeLabel: string;
  health: MapHealth;
  status: string;
  /** Logical CPU cores reported for the host, when positive. */
  cpuCores?: number;
  /** Total physical memory in bytes reported for the host, when positive. */
  memoryTotalBytes?: number;
  /** Short truthful platform or OS version label (e.g. `PVE 8.3`, `Docker 27.3`, `Debian 12`). */
  versionLabel?: string;
  /** Host primary temperature in degrees Celsius, rounded to the nearest integer. */
  temperatureC?: number;
  /** Compact host GPU summary (e.g. `RTX 4090`, `2× GPU`), stripped to generic label under Hide names. */
  gpuSummary?: string;
  /** Peak GPU utilization percentage (0 to 100) across host GPUs, when reported. */
  gpuUtilPercent?: number;
  /** Peak GPU temperature in degrees Celsius across host GPUs, rounded to integer. */
  gpuTempC?: number;
  /** Compact physical disk inventory summary (e.g. `2× NVMe · 4× HDD`). */
  diskSummary?: string;
  /** Highest reported physical disk temperature in degrees Celsius, rounded to integer. */
  diskMaxTempC?: number;
  /** Count of child physical disks reporting unhealthy, degraded, or failing health. */
  unhealthyDiskCount?: number;
  /** Live uptime in whole seconds when the host is online. */
  uptimeSeconds?: number;
  /** Count of pending package or image updates reported for the host, when positive. */
  pendingUpdates?: number;
  cpu?: number;
  memory?: number;
  /** Live inbound network rate in bytes per second, when collected. */
  netRxBytes?: number;
  /** Live outbound network rate in bytes per second, when collected. */
  netTxBytes?: number;
  /** Live disk read rate in bytes per second, when collected. */
  diskReadBytes?: number;
  /** Live disk write rate in bytes per second, when collected. */
  diskWriteBytes?: number;
  /** Fastest reported physical link speed in Mbps across non-host-local interfaces. */
  linkSpeedMbps?: number;
  /** Full-duplex peak link utilization percentage (1 to 100) when speed and live rate are known. */
  linkUtilizationPercent?: number;
  /** Number of non-host-local interfaces currently reporting link down. */
  linkDownCount?: number;
  sections: MapSection[];
  pools: MapPool[];
  /** The address the host itself answers on. */
  address?: string;
  /** Networks the host has an address on. */
  ownSegmentIds: string[];
}

export interface MapGroup {
  id: string;
  name: string;
  typeLabel: string;
  platform: MapPlatform;
  hosts: MapHost[];
  pools: MapPool[];
}

export type MapBlock = { kind: 'group'; group: MapGroup } | { kind: 'host'; host: MapHost };

export interface MapTotal {
  key: string;
  label: string;
  singular: string;
  count: number;
}

export const formatMapTotalLabel = (total: MapTotal): string =>
  total.count === 1 ? total.singular : total.label;

export interface InfrastructureMapModel {
  blocks: MapBlock[];
  /** Every network the estate's addresses fall into, largest first. */
  segments: MapSegment[];
  /** The connections between things that a collected value states. */
  connections: MapConnections;
  totals: MapTotal[];
  attentionCount: number;
  isEmpty: boolean;
  /** Newest real Unix ms `lastSeen` timestamp across resources, when present. */
  latestSampleAtMs?: number;
  /** True when live telemetry samples have expired beyond MAP_LIVE_SAMPLE_MAX_AGE_MS. */
  liveSamplesExpired?: boolean;
}

const PLATFORM_ORDER: readonly MapPlatform[] = [
  'proxmox',
  'vmware',
  'kubernetes',
  'docker',
  'truenas',
  'backup',
  'mail',
  'machine',
  'storage',
  'checks',
];

const HOST_TYPE_LABEL: Record<MapPlatform, string> = {
  proxmox: 'Proxmox node',
  vmware: 'vSphere host',
  kubernetes: 'Kubernetes cluster',
  docker: 'Docker host',
  truenas: 'TrueNAS system',
  backup: 'Backup server',
  mail: 'Mail gateway',
  machine: 'Machine',
  storage: 'Storage',
  checks: 'Checks',
};

/**
 * What a machine is called with its name hidden: this word and a number.
 * Shorter than the type label, because a tile cuts a name that does not fit
 * it and the number is all that tells two machines apart. The tile's logo
 * and its card still say what the machine is.
 */
const HOST_STAND_IN: Record<MapPlatform, string> = {
  proxmox: 'Proxmox',
  vmware: 'vSphere',
  kubernetes: 'Cluster',
  docker: 'Docker',
  truenas: 'TrueNAS',
  backup: 'Backup',
  mail: 'Mail',
  machine: 'Machine',
  storage: 'Storage',
  checks: 'Checks',
};

const SYSTEM_CONTAINER_LABEL = 'System container';

/** The same for a guest or container: its type, or a shorter word for it. */
const itemStandIn = (typeLabel: string): string =>
  typeLabel === SYSTEM_CONTAINER_LABEL ? 'Container' : typeLabel;

/** And for a check, which stays what it is on with its name hidden. */
const CHECK_STAND_IN: Partial<Record<AvailabilityTargetKind, string>> = {
  router: 'Router',
  switch: 'Switch',
  access_point: 'AP',
};

const GROUP_TYPE_LABEL: Partial<Record<MapPlatform, string>> = {
  proxmox: 'Proxmox cluster',
  vmware: 'vSphere cluster',
  docker: 'Swarm',
};

const SYSTEM_CONTAINER_TYPES = new Set<ResourceType>(['system-container', 'oci-container', 'jail']);
const POOL_TYPES = new Set<ResourceType>(['storage', 'datastore', 'pool', 'ceph']);

const SHARED_STORAGE_HOST_ID = 'map:shared-storage';
const CHECKS_HOST_ID = 'map:checks';

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });
const byName = <T extends { name: string }>(a: T, b: T) => collator.compare(a.name, b.name);

const clean = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const finitePercent = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value)
    ? Math.min(100, Math.max(0, value))
    : undefined;

const finiteNonNegative = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : undefined;

const positiveInt = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) && Number.isInteger(value) && value > 0
    ? value
    : undefined;

const positiveFinite = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : undefined;

const uptimeSecondsOf = (resource: Resource, health: MapHealth): number | undefined => {
  if (
    health === 'off' ||
    health === 'critical' ||
    resource.status === 'stopped' ||
    resource.status === 'offline'
  ) {
    return undefined;
  }
  const raw =
    positiveFinite(resource.proxmox?.uptime) ??
    positiveFinite(resource.agent?.uptimeSeconds) ??
    positiveFinite(resource.docker?.uptimeSeconds) ??
    positiveFinite(resource.pbs?.uptimeSeconds) ??
    positiveFinite(resource.truenas?.uptimeSeconds) ??
    positiveFinite(resource.pmg?.uptimeSeconds) ??
    positiveFinite(resource.kubernetes?.uptimeSeconds) ??
    positiveFinite(resource.uptime);
  if (raw === undefined) return undefined;
  const rounded = Math.round(raw);
  return rounded > 0 ? rounded : undefined;
};

const pendingUpdatesOf = (resource: Resource): number | undefined => {
  const agentPending =
    resource.agent?.packageUpdates?.supported !== false
      ? positiveInt(resource.agent?.packageUpdates?.pendingCount)
      : undefined;
  return (
    positiveInt(resource.proxmox?.pendingUpdates) ??
    agentPending ??
    positiveInt(resource.docker?.updatesAvailableCount) ??
    (resource.docker?.updateStatus?.updateAvailable === true ? 1 : undefined)
  );
};

const lastBackupAtOf = (resource: Resource): string | undefined => {
  const value = clean(resource.proxmox?.lastBackup);
  if (!value) return undefined;
  return Number.isFinite(Date.parse(value)) ? value : undefined;
};

const compactGpuName = (raw: string): string => {
  const trimmed = raw
    .replace(
      /^(?:NVIDIA\s+GeForce|NVIDIA\s+Tesla|NVIDIA\s+Quadro|NVIDIA|AMD\s+Radeon|AMD|Intel\s+Arc|Intel)\s+/i,
      '',
    )
    .trim();
  return trimmed || raw.trim() || 'GPU';
};

const hostGpuTelemetryOf = (
  resource: Resource,
): { gpuSummary?: string; gpuUtilPercent?: number; gpuTempC?: number } => {
  const gpus = resource.agent?.sensors?.gpu;
  if (!gpus?.length) return {};
  let maxUtil: number | undefined;
  let maxTemp: number | undefined;
  for (const gpu of gpus) {
    const util = finitePercent(gpu.utilizationPercent);
    if (util !== undefined && (maxUtil === undefined || util > maxUtil)) {
      maxUtil = Math.round(util);
    }
    if (
      typeof gpu.temperatureCelsius === 'number' &&
      Number.isFinite(gpu.temperatureCelsius) &&
      gpu.temperatureCelsius > 0
    ) {
      const temp = Math.round(gpu.temperatureCelsius);
      if (maxTemp === undefined || temp > maxTemp) maxTemp = temp;
    }
  }
  const gpuSummary =
    gpus.length === 1 ? compactGpuName(clean(gpus[0].name)) : `${gpus.length}× GPU`;
  return {
    gpuSummary,
    ...(maxUtil !== undefined ? { gpuUtilPercent: maxUtil } : {}),
    ...(maxTemp !== undefined ? { gpuTempC: maxTemp } : {}),
  };
};

const acceleratorsOf = (resource: Resource): string[] | undefined => {
  const set = new Set<string>();
  const add = (label: string) => {
    const trimmed = clean(label);
    if (trimmed) set.add(trimmed);
  };
  for (const label of resource.docker?.accelerators ?? []) add(label);
  for (const label of resource.proxmox?.accelerators ?? []) add(label);
  // Also recognize device bind mounts reported by older Docker agents immediately.
  for (const mount of resource.docker?.mounts ?? []) {
    for (const rawPath of [mount.source, mount.destination]) {
      const path = clean(rawPath).toLowerCase();
      if (!path) continue;
      if (path.startsWith('/dev/nvidia')) add('GPU');
      else if (path.startsWith('/dev/kfd')) add('AMD GPU');
      else if (path === '/dev/dri' || path.startsWith('/dev/dri/')) add('iGPU');
      else if (path.startsWith('/dev/apex')) add('TPU');
      else if (path.startsWith('/dev/hailo')) add('NPU');
    }
  }
  if (set.size === 0) return undefined;
  return [...set].sort(collator.compare);
};

const HEALTHY_POOL_STATES = new Set([
  'ONLINE',
  'STARTED',
  'OPTIMAL',
  'HEALTHY',
  'ACTIVE',
  'NORMAL',
  'OK',
]);

const REBUILD_SYNC_ACTION = /^(resilver|rebuild|reconstruct|recover)/i;

const poolZfsStateOf = (resource: Resource): string | undefined => {
  const raw = clean(resource.storage?.zfsPoolState) || clean(resource.storage?.arrayState);
  if (!raw) return undefined;
  const normalized = raw.toUpperCase();
  return HEALTHY_POOL_STATES.has(normalized) ? undefined : normalized;
};

const poolRebuildInProgressOf = (resource: Resource): boolean =>
  resource.storage?.rebuildInProgress === true ||
  REBUILD_SYNC_ACTION.test(clean(resource.storage?.syncAction));

const zfsErrorCountOf = (resource: Resource): number | undefined => {
  const storage = resource.storage;
  if (!storage) return undefined;
  let total = 0;
  for (const count of [storage.zfsReadErrors, storage.zfsWriteErrors, storage.zfsChecksumErrors]) {
    const valid = positiveFinite(count);
    if (valid !== undefined) total += Math.round(valid);
  }
  return total > 0 ? total : undefined;
};

const validTemperature = (value: unknown, min: number, max: number): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) && value >= min && value <= max
    ? value
    : undefined;

const hostTemperatureOf = (resource: Resource): number | undefined => {
  const direct =
    validTemperature(resource.proxmox?.temperature, 1, 150) ??
    validTemperature(resource.docker?.temperature, 1, 150) ??
    validTemperature(resource.temperature, 1, 150);
  if (direct !== undefined) return Math.round(direct);

  const sensorTemps = resource.agent?.sensors?.temperatureCelsius;
  if (!sensorTemps) return undefined;
  let maxTemp: number | undefined;
  for (const reading of Object.values(sensorTemps)) {
    const valid = validTemperature(reading, 1, 150);
    if (valid !== undefined && (maxTemp === undefined || valid > maxTemp)) {
      maxTemp = valid;
    }
  }
  return maxTemp !== undefined ? Math.round(maxTemp) : undefined;
};

type DiskBucket = 'NVMe' | 'SSD' | 'HDD' | 'Disk';

const DISK_BUCKET_ORDER: readonly DiskBucket[] = ['NVMe', 'SSD', 'HDD', 'Disk'];

const UNHEALTHY_DISK_HEALTH =
  /^(FAIL|FAILED|CRITICAL|DEGRADED|PREFAIL|WARN|WARNING|UNHEALTHY|ERROR)/;

const diskBucketOf = (diskType: string | undefined): DiskBucket => {
  switch (clean(diskType).toLowerCase()) {
    case 'nvme':
      return 'NVMe';
    case 'ssd':
      return 'SSD';
    case 'hdd':
      return 'HDD';
    default:
      return 'Disk';
  }
};

const summarizeHostDisks = (
  disks: readonly Resource[],
): { diskSummary?: string; diskMaxTempC?: number; unhealthyDiskCount?: number } => {
  if (disks.length === 0) return {};
  const counts: Record<DiskBucket, number> = { NVMe: 0, SSD: 0, HDD: 0, Disk: 0 };
  let maxTemp: number | undefined;
  let unhealthyCount = 0;

  for (const disk of disks) {
    counts[diskBucketOf(disk.physicalDisk?.diskType)] += 1;
    const temp = validTemperature(disk.physicalDisk?.temperature, 1, 120);
    if (temp !== undefined && (maxTemp === undefined || temp > maxTemp)) {
      maxTemp = temp;
    }
    const diskHealth = clean(disk.physicalDisk?.health).toUpperCase();
    if (isProblem(resolveMapHealth(disk)) || UNHEALTHY_DISK_HEALTH.test(diskHealth)) {
      unhealthyCount += 1;
    }
  }

  const diskSummary =
    counts.NVMe === 0 && counts.SSD === 0 && counts.HDD === 0
      ? `${counts.Disk} ${counts.Disk === 1 ? 'disk' : 'disks'}`
      : DISK_BUCKET_ORDER.filter((bucket) => counts[bucket] > 0)
          .map((bucket) => `${counts[bucket]}× ${bucket}`)
          .join(' · ');

  return {
    diskSummary,
    ...(maxTemp !== undefined ? { diskMaxTempC: Math.round(maxTemp) } : {}),
    ...(unhealthyCount > 0 ? { unhealthyDiskCount: unhealthyCount } : {}),
  };
};

const extractMajorMinor = (raw: string | undefined): string | undefined => {
  const value = clean(raw);
  if (!value) return undefined;
  const match = value.match(/(\d+\.\d+)/) ?? value.match(/(\d+)/);
  return match?.[1];
};

const hostCpuCoresOf = (resource: Resource): number | undefined =>
  positiveInt(resource.proxmox?.cpus) ??
  positiveInt(resource.agent?.cpuCount) ??
  positiveInt(resource.docker?.cpus) ??
  positiveInt(resource.vmware?.cpuCount);

const hostMemoryTotalBytesOf = (resource: Resource): number | undefined =>
  positiveFinite(resource.memory?.total) ??
  positiveFinite(resource.proxmox?.memory?.total) ??
  positiveFinite(resource.agent?.memory?.total) ??
  positiveFinite(resource.docker?.memory?.total);

const hostVersionLabelOf = (resource: Resource): string | undefined => {
  const pve = extractMajorMinor(resource.proxmox?.pveVersion);
  if (pve) return `PVE ${pve}`;

  const docker = extractMajorMinor(resource.docker?.dockerVersion);
  if (docker) return `Docker ${docker}`;

  const runtimeVer = extractMajorMinor(resource.docker?.runtimeVersion);
  if (runtimeVer) {
    const runtime = clean(resource.docker?.runtime).toLowerCase();
    const prefix = runtime === 'podman' ? 'Podman' : 'Docker';
    return `${prefix} ${runtimeVer}`;
  }

  const pbs = extractMajorMinor(resource.pbs?.version);
  if (pbs) return `PBS ${pbs}`;

  const pmg = extractMajorMinor(resource.pmg?.version);
  if (pmg) return `PMG ${pmg}`;

  const truenas = extractMajorMinor(resource.truenas?.version);
  if (truenas) return `TrueNAS ${truenas}`;

  const k8s = extractMajorMinor(resource.kubernetes?.version);
  if (k8s) return `K8s ${k8s}`;

  const rawOsName = clean(resource.agent?.osName)
    .replace(/\([^)]*\)/g, '')
    .replace(/\bGNU\/Linux\b/gi, '')
    .replace(/\s+/g, ' ')
    .trim();
  if (!rawOsName) return undefined;
  if (/^darwin$/i.test(rawOsName)) return 'macOS';
  const osMajor = clean(resource.agent?.osVersion).match(/(\d+)/)?.[1];
  const hasMajor = osMajor !== undefined && new RegExp(`(^|\\D)${osMajor}(\\D|$)`).test(rawOsName);
  const combined = osMajor && !hasMajor ? `${rawOsName} ${osMajor}` : rawOsName;
  return combined.length > 18 ? combined.slice(0, 18).trimEnd() : combined;
};

const HOST_LOCAL_INTERFACE =
  /^(lo|docker\d*|podman\d*|br-[0-9a-f]{12}|veth.*|virbr\d*|cni.*|flannel.*|cali.*|kube-ipvs\d*)$/i;

const isRealEpochMs = (value: number | undefined): value is number =>
  typeof value === 'number' && Number.isFinite(value) && value >= REAL_EPOCH_MS_FLOOR;

/** Single canonical sample expiry comparison used by both model gating and UI timers. */
export const isSampleTimestampExpired = (lastSeenMs: number | undefined, nowMs: number): boolean =>
  isRealEpochMs(lastSeenMs) && nowMs - lastSeenMs >= MAP_LIVE_SAMPLE_MAX_AGE_MS;

const isLiveTelemetryFresh = (
  resource: Resource,
  health: MapHealth,
  nowMs: number,
  rateObservedAtMs?: number,
): boolean => {
  if (
    health === 'off' ||
    health === 'stale' ||
    health === 'unknown' ||
    resource.status === 'stopped' ||
    resource.status === 'offline' ||
    resource.status === 'paused'
  ) {
    return false;
  }
  const sampleAtMs = isRealEpochMs(rateObservedAtMs) ? rateObservedAtMs : resource.lastSeen;
  if (isSampleTimestampExpired(sampleAtMs, nowMs)) {
    return false;
  }
  return true;
};

const networkRatesOf = (
  resource: Resource,
  health: MapHealth,
  nowMs: number,
): { netRxBytes?: number; netTxBytes?: number } => {
  if (!resource.network) return {};
  const rxObservedAt = resource.network.rxObservedAt ?? resource.network.observedAt;
  const txObservedAt = resource.network.txObservedAt ?? resource.network.observedAt;
  const netRxBytes = isLiveTelemetryFresh(resource, health, nowMs, rxObservedAt)
    ? finiteNonNegative(resource.network.rxBytes)
    : undefined;
  const netTxBytes = isLiveTelemetryFresh(resource, health, nowMs, txObservedAt)
    ? finiteNonNegative(resource.network.txBytes)
    : undefined;
  return {
    ...(netRxBytes !== undefined ? { netRxBytes } : {}),
    ...(netTxBytes !== undefined ? { netTxBytes } : {}),
  };
};

const diskRatesOf = (
  resource: Resource,
  health: MapHealth,
  nowMs: number,
): { diskReadBytes?: number; diskWriteBytes?: number } => {
  if (!resource.diskIO) return {};
  const readObservedAt = resource.diskIO.readObservedAt ?? resource.diskIO.observedAt;
  const writeObservedAt = resource.diskIO.writeObservedAt ?? resource.diskIO.observedAt;
  const diskReadBytes = isLiveTelemetryFresh(resource, health, nowMs, readObservedAt)
    ? finiteNonNegative(resource.diskIO.readRate)
    : undefined;
  const diskWriteBytes = isLiveTelemetryFresh(resource, health, nowMs, writeObservedAt)
    ? finiteNonNegative(resource.diskIO.writeRate)
    : undefined;
  return {
    ...(diskReadBytes !== undefined ? { diskReadBytes } : {}),
    ...(diskWriteBytes !== undefined ? { diskWriteBytes } : {}),
  };
};

const latencyOf = (resource: Resource): number | undefined => {
  if (resource.type !== 'network-endpoint') return undefined;
  if (resource.availability?.available === false) return undefined;
  return finiteNonNegative(resource.availability?.latencyMillis);
};

const interfaceTelemetryOf = (
  resource: Resource,
  health: MapHealth,
): { linkSpeedMbps?: number; linkDownCount?: number } => {
  const lists = [
    resource.agent?.networkInterfaces,
    resource.docker?.networkInterfaces,
    resource.proxmox?.networkInterfaces,
  ];
  let maxSpeed = 0;
  const downNames = new Set<string>();
  let unnamedDown = 0;
  const countLinkDown =
    health !== 'off' &&
    health !== 'critical' &&
    (health === 'ok' ||
      health === 'attention' ||
      resource.status === 'online' ||
      resource.status === 'running');
  for (const list of lists) {
    for (const nic of list ?? []) {
      const name = clean(nic.name);
      if (HOST_LOCAL_INTERFACE.test(name)) continue;
      if (
        typeof nic.speedMbps === 'number' &&
        Number.isFinite(nic.speedMbps) &&
        nic.speedMbps > maxSpeed
      ) {
        maxSpeed = nic.speedMbps;
      }
      if (countLinkDown && nic.linkUp === false) {
        if (name) downNames.add(name.toLowerCase());
        else unnamedDown += 1;
      }
    }
  }
  const linkDownCount = downNames.size + unnamedDown;
  return {
    ...(maxSpeed > 0 ? { linkSpeedMbps: maxSpeed } : {}),
    ...(linkDownCount > 0 ? { linkDownCount } : {}),
  };
};

const proxmoxFirewallEnabled = (resource: Resource): boolean =>
  resource.platformType === 'proxmox-pve' &&
  (resource.proxmox?.networkInterfaces ?? []).some((nic) => nic.firewall === true);

const DEFAULT_DOCKER_NETWORKS = new Set(['bridge', 'host', 'none', 'ingress', 'docker_gwbridge']);

const dockerNetworksOf = (resource: Resource): string[] | undefined => {
  if (resource.type !== 'app-container') return undefined;
  const names = new Set<string>();
  for (const network of resource.docker?.networks ?? []) {
    const name = clean(network.name);
    if (!name || DEFAULT_DOCKER_NETWORKS.has(name.toLowerCase())) continue;
    names.add(name);
  }
  if (names.size === 0) return undefined;
  return [...names].sort(collator.compare);
};

const probeSummaryOf = (resource: Resource): string | undefined => {
  if (resource.type !== 'network-endpoint') return undefined;
  const avail = resource.availability;
  if (!avail) return undefined;
  const protocol = clean(avail.protocol).toLowerCase();
  const statusCode =
    typeof avail.applicationStatusCode === 'number' &&
    Number.isFinite(avail.applicationStatusCode) &&
    avail.applicationStatusCode > 0
      ? Math.round(avail.applicationStatusCode)
      : undefined;
  const port =
    typeof avail.port === 'number' && Number.isFinite(avail.port) && avail.port > 0
      ? Math.round(avail.port)
      : undefined;
  if (!protocol) {
    return statusCode !== undefined ? `HTTP ${statusCode}` : undefined;
  }
  if (protocol === 'http' || protocol === 'https') {
    const label = protocol.toUpperCase();
    return statusCode !== undefined ? `${label} ${statusCode}` : label;
  }
  if (protocol === 'tcp' || protocol === 'udp') {
    const label = protocol.toUpperCase();
    return port !== undefined ? `${label} :${port}` : label;
  }
  if (protocol === 'push') return 'Push';
  return protocol.toUpperCase();
};

const certWarningOf = (resource: Resource): string | undefined => {
  if (resource.type !== 'network-endpoint') return undefined;
  const cert = resource.availability?.certificate;
  if (!cert) return undefined;
  const trust = clean(cert.trustStatus).toLowerCase();
  if (trust === 'expired') return 'Cert expired';
  if (trust === 'untrusted') return 'Cert untrusted';
  if (
    trust === 'invalid' ||
    trust === 'not-yet-valid' ||
    (trust !== 'self-signed' && cert.chainValid === false) ||
    cert.hostnameValid === false
  ) {
    return 'Cert invalid';
  }
  if (trust === 'expiring_soon') return 'Cert expiring soon';
  if (resource.availability?.certificateMonitoring && cert.notAfter) {
    const expiryMs = Date.parse(cert.notAfter);
    const warnDays = resource.availability.certificateExpiryWarningDays ?? 30;
    if (Number.isFinite(expiryMs) && warnDays > 0) {
      const remainingMs = expiryMs - Date.now();
      if (remainingMs <= 0) return 'Cert expired';
      if (remainingMs <= warnDays * 86_400_000) return 'Cert expiring soon';
    }
  }
  return undefined;
};

export const resolveMapHealth = (resource: Resource): MapHealth => {
  const verdict = resource.health?.verdict;
  if (verdict) return verdict;
  switch (resource.status) {
    case 'online':
    case 'running':
      return 'ok';
    case 'degraded':
    case 'warning':
      return 'attention';
    case 'offline':
      return 'critical';
    case 'stopped':
    case 'paused':
      return 'off';
    default:
      return 'unknown';
  }
};

const displayName = (resource: Resource): string =>
  clean(resource.displayName) || clean(resource.name) || resource.id;

const hostPlatform = (resource: Resource): MapPlatform | null => {
  switch (resource.type) {
    case 'docker-host':
      return 'docker';
    case 'k8s-cluster':
      return 'kubernetes';
    case 'pbs':
      return 'backup';
    case 'pmg':
      return 'mail';
    case 'agent':
      if (resource.platformType === 'proxmox-pve') return 'proxmox';
      if (resource.platformType === 'vmware-vsphere') return 'vmware';
      if (resource.platformType === 'truenas') return 'truenas';
      return 'machine';
    default:
      return null;
  }
};

type Attachments = ReadonlyMap<string, MapAttachment[]>;

interface PodExposure {
  detail?: string;
  ingressHosts?: string[];
}

type PodExposures = ReadonlyMap<string, PodExposure>;

const formatPortList = (ports: Iterable<number>): string | undefined => {
  const unique = new Set<number>();
  for (const port of ports) {
    if (typeof port === 'number' && Number.isFinite(port) && port > 0) {
      unique.add(Math.round(port));
    }
  }
  if (unique.size === 0) return undefined;
  const sorted = [...unique].sort((a, b) => a - b);
  const shown = sorted.slice(0, 3).join(', ');
  return sorted.length > 3 ? `:${shown} +${sorted.length - 3}` : `:${shown}`;
};

const publishedPorts = (resource: Resource): string | undefined => {
  const ports: number[] = [];
  for (const port of resource.docker?.ports ?? []) {
    if (typeof port.publicPort === 'number' && port.publicPort > 0) ports.push(port.publicPort);
  }
  for (const port of resource.docker?.endpointPorts ?? []) {
    if (typeof port.publishedPort === 'number' && port.publishedPort > 0) {
      ports.push(port.publishedPort);
    }
  }
  for (const port of resource.truenas?.app?.usedPorts ?? []) {
    for (const hostPort of port.hostPorts ?? []) {
      if (typeof hostPort.hostPort === 'number' && hostPort.hostPort > 0) {
        ports.push(hostPort.hostPort);
      }
    }
  }
  for (const container of resource.truenas?.app?.containers ?? []) {
    for (const port of container.portConfig ?? []) {
      for (const hostPort of port.hostPorts ?? []) {
        if (typeof hostPort.hostPort === 'number' && hostPort.hostPort > 0) {
          ports.push(hostPort.hostPort);
        }
      }
    }
  }
  return formatPortList(ports);
};

interface IndexedK8sService {
  selectorEntries: Array<[string, string]>;
  names: string[];
  exposedPorts: number[];
}

const resolveKubernetesPodExposures = (resources: readonly Resource[]): PodExposures => {
  const servicesByScope = new Map<string, IndexedK8sService[]>();
  const ingressHostsByService = new Map<string, Set<string>>();

  for (const resource of resources) {
    const parentId = clean(resource.parentId);
    if (!parentId) continue;
    const namespace = clean(resource.kubernetes?.namespace) || 'default';
    if (resource.type === 'k8s-service') {
      const selectorEntries = Object.entries(resource.kubernetes?.selector ?? {});
      if (selectorEntries.length === 0) continue;
      const names = [
        ...new Set(
          [
            clean(resource.name).toLowerCase(),
            clean(resource.kubernetes?.serviceName).toLowerCase(),
          ].filter(Boolean),
        ),
      ];
      const serviceType = clean(resource.kubernetes?.serviceType).toLowerCase();
      const exposedPorts: number[] = [];
      if (serviceType === 'nodeport') {
        for (const port of resource.kubernetes?.servicePorts ?? []) {
          if (
            typeof port.nodePort === 'number' &&
            Number.isFinite(port.nodePort) &&
            port.nodePort > 0
          ) {
            exposedPorts.push(port.nodePort);
          }
        }
      } else if (serviceType === 'loadbalancer') {
        for (const port of resource.kubernetes?.servicePorts ?? []) {
          if (typeof port.port === 'number' && Number.isFinite(port.port) && port.port > 0) {
            exposedPorts.push(port.port);
          }
        }
      }
      const scopeKey = `${parentId}\0${namespace}`;
      const list = servicesByScope.get(scopeKey) ?? [];
      list.push({ selectorEntries, names, exposedPorts });
      servicesByScope.set(scopeKey, list);
    } else if (resource.type === 'k8s-ingress') {
      const serviceName = clean(resource.kubernetes?.serviceName).toLowerCase();
      if (!serviceName) continue;
      const hosts = (resource.kubernetes?.hosts ?? [])
        .map((host) => clean(host).toLowerCase())
        .filter((host) => Boolean(host) && host !== '*');
      if (hosts.length === 0) continue;
      const key = `${parentId}\0${namespace}\0${serviceName}`;
      const set = ingressHostsByService.get(key) ?? new Set<string>();
      for (const host of hosts) set.add(host);
      ingressHostsByService.set(key, set);
    }
  }

  const exposures = new Map<string, PodExposure>();
  if (servicesByScope.size === 0) return exposures;

  for (const resource of resources) {
    if (resource.type !== 'pod') continue;
    const parentId = clean(resource.parentId);
    const labels = resource.kubernetes?.labels;
    if (!parentId || !labels) continue;
    const namespace = clean(resource.kubernetes?.namespace) || 'default';
    const scopeServices = servicesByScope.get(`${parentId}\0${namespace}`);
    if (!scopeServices?.length) continue;

    const ports: number[] = [];
    const hosts = new Set<string>();
    for (const service of scopeServices) {
      if (!service.selectorEntries.every(([key, value]) => labels[key] === value)) continue;
      for (const port of service.exposedPorts) ports.push(port);
      for (const serviceName of service.names) {
        const matchedHosts = ingressHostsByService.get(`${parentId}\0${namespace}\0${serviceName}`);
        if (!matchedHosts) continue;
        for (const host of matchedHosts) hosts.add(host);
      }
    }
    const detail = formatPortList(ports);
    const ingressHosts = hosts.size > 0 ? [...hosts].sort(collator.compare) : undefined;
    if (detail || ingressHosts) {
      exposures.set(resource.id, {
        ...(detail ? { detail } : {}),
        ...(ingressHosts ? { ingressHosts } : {}),
      });
    }
  }
  return exposures;
};

const toItem = (
  resource: Resource,
  typeLabel: string,
  attachments: Attachments,
  nowMs: number,
  podExposure?: PodExposure,
): MapItem => {
  const attached = attachments.get(resource.id) ?? [];
  const health = resolveMapHealth(resource);
  const rates = networkRatesOf(resource, health, nowMs);
  const nic = interfaceTelemetryOf(resource, health);
  const uptimeSeconds = uptimeSecondsOf(resource, health);
  const pendingUpdates = pendingUpdatesOf(resource);
  const lastBackupAt = lastBackupAtOf(resource);
  const linkUtilizationPercent = computeMapLinkUtilizationPercent(
    rates.netRxBytes,
    rates.netTxBytes,
    nic.linkSpeedMbps,
  );
  const latencyMs = latencyOf(resource);
  const firewallEnabled = proxmoxFirewallEnabled(resource);
  const linkedResourceId =
    resource.type === 'network-endpoint'
      ? clean(resource.availability?.linkedResourceId) || undefined
      : undefined;
  const dockerNetworks = dockerNetworksOf(resource);
  const probeSummary = probeSummaryOf(resource);
  const certWarning = certWarningOf(resource);
  const vmid = positiveInt(resource.proxmox?.vmid);
  const composeProject =
    resource.type === 'app-container'
      ? clean(resource.docker?.composeProject) ||
        clean(resource.docker?.podman?.composeProject) ||
        undefined
      : undefined;
  const detail =
    resource.type === 'app-container'
      ? publishedPorts(resource)
      : resource.type === 'pod'
        ? podExposure?.detail
        : undefined;
  const ingressHosts = resource.type === 'pod' ? podExposure?.ingressHosts : undefined;
  const accelerators = acceleratorsOf(resource);
  return {
    id: resource.id,
    name: displayName(resource),
    typeLabel,
    health,
    status: resource.status,
    ...(vmid !== undefined ? { vmid } : {}),
    ...(composeProject ? { composeProject } : {}),
    cpu: finitePercent(resource.cpu?.current),
    memory: finitePercent(getMemoryPercent(resource)),
    ...rates,
    ...diskRatesOf(resource, health, nowMs),
    ...(uptimeSeconds !== undefined ? { uptimeSeconds } : {}),
    ...(pendingUpdates !== undefined ? { pendingUpdates } : {}),
    ...(lastBackupAt ? { lastBackupAt } : {}),
    ...nic,
    ...(linkUtilizationPercent !== undefined ? { linkUtilizationPercent } : {}),
    ...(latencyMs !== undefined ? { latencyMs } : {}),
    ...(firewallEnabled ? { firewallEnabled: true } : {}),
    ...(linkedResourceId ? { linkedResourceId } : {}),
    ...(dockerNetworks ? { dockerNetworks } : {}),
    ...(ingressHosts ? { ingressHosts } : {}),
    ...(accelerators ? { accelerators } : {}),
    ...(probeSummary ? { probeSummary } : {}),
    ...(certWarning ? { certWarning } : {}),
    ...(attached[0]?.address ? { address: attached[0].address } : {}),
    ...(attached.length > 0 ? { segmentIds: attached.map((entry) => entry.segmentId) } : {}),
    ...(detail ? { detail } : {}),
  };
};

const NETWORK_FILESYSTEMS = new Set([
  'nfs',
  'nfs4',
  'cifs',
  'smb3',
  'smbfs',
  'fuse.sshfs',
  '9p',
  'virtiofs',
  'ceph',
  'fuse.glusterfs',
]);

// What a guest mounts from outside itself, and nothing else: a host folder
// Proxmox binds into a container (`mp0` from `/mnt/media`), or a network
// share. Anything else a guest reports is a disk of its own, which a clone
// of the same guest has a copy of, so it is never read as shared.
const isOutsideMount = (device: string, filesystem: string): boolean =>
  (/^mp\d+$/.test(filesystem) && device.startsWith('/') && !device.startsWith('/dev/')) ||
  NETWORK_FILESYSTEMS.has(filesystem) ||
  /^[^/\\\s:]{2,}:\//.test(device) ||
  device.startsWith('//');

const mountsOf = (resource: Resource): MapMount[] => {
  const mounts: MapMount[] = [];
  for (const disk of resource.proxmox?.disks ?? []) {
    const { total, used } = disk;
    if (clean(disk.mountpoint) === '/') continue;
    if (!isOutsideMount(clean(disk.device), clean(disk.filesystem).toLowerCase())) continue;
    if (typeof total !== 'number' || typeof used !== 'number' || !(total > 0) || used < 0) continue;
    mounts.push({ total, used });
  }
  return mounts;
};

// A guest or container is what the service diagram draws as one tile, so it
// also carries the app it is and the group that puts it in.
const toService = (
  resource: Resource,
  typeLabel: string,
  attachments: Attachments,
  nowMs: number,
  podExposure?: PodExposure,
): MapItem => {
  const app = resolveMapApp({ name: displayName(resource), image: resource.docker?.image });
  const mounts = mountsOf(resource);
  return {
    ...toItem(resource, typeLabel, attachments, nowMs, podExposure),
    ...(app ? { app } : {}),
    category: resolveMapCategory(app, resource.tags),
    ...(mounts.length > 0 ? { mounts } : {}),
  };
};

const isBackupTarget = (resource: Resource): boolean =>
  resource.type === 'datastore' ||
  resource.platformType === 'proxmox-pbs' ||
  clean(resource.storage?.type).toLowerCase() === 'pbs';

const toPool = (resource: Resource): MapPool => {
  const total = resource.disk?.total;
  const used = resource.disk?.used;
  const vdevLayout = clean(resource.storage?.vdevLayout) || undefined;
  const storageType = clean(resource.storage?.type) || undefined;
  const zfsState = poolZfsStateOf(resource);
  const rebuildInProgress = poolRebuildInProgressOf(resource);
  const zfsErrorCount = zfsErrorCountOf(resource);
  const consumerIds = (resource.storage?.topConsumers ?? [])
    .map((consumer) => clean(consumer.resourceId))
    .filter(Boolean);
  return {
    id: resource.id,
    name: displayName(resource),
    health: resolveMapHealth(resource),
    usedPercent: finitePercent(getDiskPercent(resource)),
    sizeLabel: typeof total === 'number' && total > 0 ? formatBytes(total, 0) : undefined,
    ...(vdevLayout ? { vdevLayout } : {}),
    ...(storageType ? { storageType } : {}),
    ...(zfsState ? { zfsState } : {}),
    ...(rebuildInProgress ? { rebuildInProgress: true } : {}),
    ...(zfsErrorCount !== undefined ? { zfsErrorCount } : {}),
    ...(isBackupTarget(resource) ? { backup: true } : {}),
    ...(typeof total === 'number' && total > 0 && typeof used === 'number' && used >= 0
      ? { totalBytes: total, usedBytes: used }
      : {}),
    ...(consumerIds.length > 0 ? { consumerIds } : {}),
  };
};

interface HostDraft {
  host: MapHost;
  resource: Resource;
  sections: Map<string, MapSection>;
  disks: Resource[];
}

const sectionFor = (draft: HostDraft, key: string, label: string): MapSection => {
  let section = draft.sections.get(key);
  if (!section) {
    section = { key, label, items: [] };
    draft.sections.set(key, section);
  }
  return section;
};

// The order a host's sections are kept in.
const SECTION_RANK = (key: string): number => {
  if (key === 'vms') return 1;
  if (key === 'lxcs') return 2;
  if (key === 'containers') return 3;
  return 4;
};

const placeChild = (
  draft: HostDraft,
  child: Resource,
  attachments: Attachments,
  podExposures: PodExposures,
  nowMs: number,
): 'placed' | 'pool' | 'skipped' => {
  const platform = draft.host.platform;
  if (child.type === 'vm') {
    sectionFor(draft, 'vms', 'VMs').items.push(toService(child, 'VM', attachments, nowMs));
    return 'placed';
  }
  if (SYSTEM_CONTAINER_TYPES.has(child.type)) {
    const proxmox = platform === 'proxmox';
    sectionFor(draft, 'lxcs', proxmox ? 'LXCs' : 'System containers').items.push(
      toService(child, proxmox ? 'LXC' : SYSTEM_CONTAINER_LABEL, attachments, nowMs),
    );
    return 'placed';
  }
  if (child.type === 'app-container') {
    const app = platform === 'truenas';
    sectionFor(draft, 'containers', app ? 'Apps' : 'Containers').items.push(
      toService(child, app ? 'App' : 'Container', attachments, nowMs),
    );
    return 'placed';
  }
  if (child.type === 'pod') {
    const namespace = clean(child.kubernetes?.namespace) || 'default';
    sectionFor(draft, `ns:${namespace}`, namespace).items.push(
      toService(child, 'Pod', attachments, nowMs, podExposures.get(child.id)),
    );
    return 'placed';
  }
  if (POOL_TYPES.has(child.type)) {
    draft.host.pools.push(toPool(child));
    return 'pool';
  }
  return 'skipped';
};

const groupKeyFor = (resource: Resource, platform: MapPlatform): string | null => {
  const cluster = clean(resource.clusterId);
  if (!cluster) return null;
  if (platform === 'proxmox') {
    return resource.proxmox?.isClusterMember === false ? null : `proxmox:${cluster}`;
  }
  if (platform === 'vmware') return `vmware:${cluster}`;
  if (platform === 'docker') return `docker:${cluster}`;
  return null;
};

const isProblem = (health: MapHealth): boolean => health === 'attention' || health === 'critical';

export function buildInfrastructureMapModel(
  resources: readonly Resource[],
  options?: { nowMs?: number },
): InfrastructureMapModel {
  const nowMs = options?.nowMs ?? Date.now();
  let latestSampleAtMs: number | undefined;
  const byId = new Map<string, Resource>();
  for (const resource of resources) {
    byId.set(resource.id, resource);
    if (
      isRealEpochMs(resource.lastSeen) &&
      (latestSampleAtMs === undefined || resource.lastSeen > latestSampleAtMs)
    ) {
      latestSampleAtMs = resource.lastSeen;
    }
  }
  const liveSamplesExpired = isSampleTimestampExpired(latestSampleAtMs, nowMs);
  const networks = resolveMapNetworks(resources);
  const attachments = networks.attachments;
  const podExposures = resolveKubernetesPodExposures(resources);

  const drafts = new Map<string, HostDraft>();
  for (const resource of resources) {
    const platform = hostPlatform(resource);
    if (!platform) continue;
    // A Kubernetes node is a machine too, but on the map it belongs inside
    // the cluster the registry parents it to.
    const parent = resource.parentId ? byId.get(resource.parentId) : undefined;
    if (resource.type === 'agent' && parent?.type === 'k8s-cluster') continue;
    const own = attachments.get(resource.id) ?? [];
    const health = resolveMapHealth(resource);
    const rates = networkRatesOf(resource, health, nowMs);
    const nic = interfaceTelemetryOf(resource, health);
    const linkUtilizationPercent = computeMapLinkUtilizationPercent(
      rates.netRxBytes,
      rates.netTxBytes,
      nic.linkSpeedMbps,
    );
    const cpuCores = hostCpuCoresOf(resource);
    const memoryTotalBytes = hostMemoryTotalBytesOf(resource);
    const versionLabel = hostVersionLabelOf(resource);
    const temperatureC = hostTemperatureOf(resource);
    const gpu = hostGpuTelemetryOf(resource);
    const uptimeSeconds = uptimeSecondsOf(resource, health);
    const pendingUpdates = pendingUpdatesOf(resource);
    drafts.set(resource.id, {
      resource,
      sections: new Map(),
      disks: [],
      host: {
        id: resource.id,
        isResource: true,
        name: displayName(resource),
        platform,
        typeLabel: HOST_TYPE_LABEL[platform],
        health,
        status: resource.status,
        ...(cpuCores !== undefined ? { cpuCores } : {}),
        ...(memoryTotalBytes !== undefined ? { memoryTotalBytes } : {}),
        ...(versionLabel ? { versionLabel } : {}),
        ...(temperatureC !== undefined ? { temperatureC } : {}),
        ...gpu,
        ...(uptimeSeconds !== undefined ? { uptimeSeconds } : {}),
        ...(pendingUpdates !== undefined ? { pendingUpdates } : {}),
        cpu: finitePercent(resource.cpu?.current),
        memory: finitePercent(getMemoryPercent(resource)),
        ...rates,
        ...diskRatesOf(resource, health, nowMs),
        ...nic,
        ...(linkUtilizationPercent !== undefined ? { linkUtilizationPercent } : {}),
        sections: [],
        pools: [],
        ...(own[0]?.address ? { address: own[0].address } : {}),
        ownSegmentIds: own.map((entry) => entry.segmentId),
      },
    });
  }

  // The host a resource sits on is the parent the registry gives it.
  const owningDraft = (resource: Resource): HostDraft | undefined =>
    resource.parentId ? drafts.get(resource.parentId) : undefined;
  const owningDiskDraft = (resource: Resource): HostDraft | undefined => {
    const direct = owningDraft(resource);
    if (direct) return direct;
    const parent = resource.parentId ? byId.get(resource.parentId) : undefined;
    return parent && POOL_TYPES.has(parent.type) ? owningDraft(parent) : undefined;
  };

  const sharedPools: Resource[] = [];
  const checks: MapItem[] = [];
  for (const resource of resources) {
    if (drafts.has(resource.id)) continue;
    if (resource.type === 'network-endpoint') {
      const checkKind = normalizeAvailabilityTargetKind(resource.availability?.targetKind);
      checks.push({
        ...toItem(resource, 'Check', attachments, nowMs),
        ...(checkKind ? { checkKind } : {}),
      });
      continue;
    }
    if (resource.type === 'physical_disk') {
      const diskHost = owningDiskDraft(resource);
      if (diskHost) diskHost.disks.push(resource);
      continue;
    }
    const draft = owningDraft(resource);
    if (draft) {
      placeChild(draft, resource, attachments, podExposures, nowMs);
      continue;
    }
    if (!resource.parentId && POOL_TYPES.has(resource.type)) sharedPools.push(resource);
  }

  const groups = new Map<string, MapGroup>();
  const looseHosts: MapHost[] = [];
  const hostGroup = new Map<string, MapGroup>();
  for (const draft of drafts.values()) {
    const host = draft.host;
    const diskMeta = summarizeHostDisks(draft.disks);
    if (diskMeta.diskSummary) host.diskSummary = diskMeta.diskSummary;
    if (diskMeta.diskMaxTempC !== undefined) host.diskMaxTempC = diskMeta.diskMaxTempC;
    if (diskMeta.unhealthyDiskCount !== undefined) {
      host.unhealthyDiskCount = diskMeta.unhealthyDiskCount;
    }
    host.sections = [...draft.sections.values()]
      .map((section) => ({
        ...section,
        items: [...section.items].sort(byName),
      }))
      .sort(
        (a, b) => SECTION_RANK(a.key) - SECTION_RANK(b.key) || collator.compare(a.label, b.label),
      );
    host.pools.sort(byName);
    const key = groupKeyFor(draft.resource, host.platform);
    if (!key) {
      looseHosts.push(host);
      continue;
    }
    let group = groups.get(key);
    if (!group) {
      group = {
        id: `map:group:${key}`,
        name: clean(draft.resource.clusterId),
        typeLabel: GROUP_TYPE_LABEL[host.platform] ?? 'Cluster',
        platform: host.platform,
        hosts: [],
        pools: [],
      };
      groups.set(key, group);
    }
    group.hosts.push(host);
    hostGroup.set(host.id, group);
  }

  // Shared storage belongs to the cluster that reports it. Proxmox names the
  // connection, vSphere names the hosts that mount it; anything else stays in
  // its own card rather than being guessed into a cluster.
  const unplacedPools: MapPool[] = [];
  const vmwareHostsByName = new Map<string, MapHost | null>();
  for (const draft of drafts.values()) {
    if (draft.host.platform !== 'vmware') continue;
    const name = clean(draft.resource.name).toLowerCase();
    vmwareHostsByName.set(name, vmwareHostsByName.has(name) ? null : draft.host);
  }
  for (const resource of sharedPools) {
    const pool = toPool(resource);
    let target: MapGroup | undefined;
    if (resource.platformType === 'proxmox-pve') {
      target = groups.get(`proxmox:${clean(resource.proxmox?.instance)}`);
      // Proxmox names the nodes the storage is set up on, by node name. A
      // name two of the cluster's hosts answer to points at neither.
      const byName = new Map<string, MapHost | null>();
      for (const host of target?.hosts ?? []) {
        const own = drafts.get(host.id)?.resource;
        const names = new Set(
          [host.name, own?.name, own?.proxmox?.nodeName]
            .map((name) => clean(name).toLowerCase())
            .filter(Boolean),
        );
        for (const name of names) {
          const known = byName.get(name);
          byName.set(name, known === undefined || known === host ? host : null);
        }
      }
      const hostIds = [
        ...new Set(
          (resource.storage?.nodes ?? [])
            .map((node) => byName.get(clean(node).toLowerCase())?.id)
            .filter((id): id is string => Boolean(id)),
        ),
      ];
      if (hostIds.length > 0) pool.hostIds = hostIds;
    } else if (resource.platformType === 'vmware-vsphere') {
      const mounted = (resource.storage?.nodes ?? []).map((node) =>
        vmwareHostsByName.get(clean(node).toLowerCase()),
      );
      const owners = new Set(mounted.map((host) => (host ? hostGroup.get(host.id) : undefined)));
      if (mounted.length > 0 && owners.size === 1) target = [...owners][0];
      const hostIds = [
        ...new Set(mounted.map((host) => host?.id).filter((id): id is string => Boolean(id))),
      ];
      if (target && hostIds.length > 0) pool.hostIds = hostIds;
    }
    if (target) target.pools.push(pool);
    else unplacedPools.push(pool);
  }

  const blocks: MapBlock[] = [];
  for (const group of groups.values()) {
    group.hosts.sort(byName);
    group.pools.sort(byName);
    blocks.push({ kind: 'group', group });
  }
  for (const host of looseHosts) blocks.push({ kind: 'host', host });

  const blockPlatform = (block: MapBlock) =>
    block.kind === 'group' ? block.group.platform : block.host.platform;
  const blockName = (block: MapBlock) =>
    block.kind === 'group' ? block.group.name : block.host.name;
  blocks.sort(
    (a, b) =>
      PLATFORM_ORDER.indexOf(blockPlatform(a)) - PLATFORM_ORDER.indexOf(blockPlatform(b)) ||
      (a.kind === b.kind ? 0 : a.kind === 'group' ? -1 : 1) ||
      collator.compare(blockName(a), blockName(b)),
  );

  if (unplacedPools.length > 0) {
    const pools = unplacedPools.sort(byName);
    blocks.push({
      kind: 'host',
      host: {
        id: SHARED_STORAGE_HOST_ID,
        isResource: false,
        name: 'Shared storage',
        platform: 'storage',
        typeLabel: HOST_TYPE_LABEL.storage,
        health: worstHealth(pools.map((pool) => pool.health)),
        status: '',
        sections: [],
        pools,
        ownSegmentIds: [],
      },
    });
  }
  if (checks.length > 0) {
    checks.sort(byName);
    blocks.push({
      kind: 'host',
      host: {
        id: CHECKS_HOST_ID,
        isResource: false,
        name: 'Availability checks',
        platform: 'checks',
        typeLabel: HOST_TYPE_LABEL.checks,
        health: worstHealth(checks.map((check) => check.health)),
        status: '',
        sections: [{ key: 'checks', label: 'Endpoints', items: checks }],
        pools: [],
        ownSegmentIds: [],
      },
    });
  }

  return summarize(
    blocks,
    networks.segments,
    ownedConnections(resources, attachments),
    latestSampleAtMs,
    liveSamplesExpired,
  );
}

// A gateway or resolver address that exactly one thing on the map answers on
// is that thing. Held by none, or by several, it stays a bare address.
function ownedConnections(
  resources: readonly Resource[],
  attachments: Attachments,
): MapConnections {
  const connections = resolveMapConnections(resources);
  const owners = new Map<string, Set<string>>();
  for (const [id, attached] of attachments) {
    for (const { address } of attached) {
      if (!address) continue;
      const ids = owners.get(address) ?? new Set<string>();
      ids.add(id);
      owners.set(address, ids);
    }
  }
  const own = <T extends { address: string }>(entry: T): T & { resourceId?: string } => {
    const ids = owners.get(entry.address);
    return ids?.size === 1 ? { ...entry, resourceId: [...ids][0] } : entry;
  };
  return {
    links: connections.links,
    gateways: connections.gateways.map(own),
    // A router is often the resolver too. The same address is the same box.
    resolvers: connections.resolvers.map(own).map((resolver) => {
      const gateway = connections.gateways.findIndex((entry) => entry.address === resolver.address);
      return gateway >= 0 ? { ...resolver, gateway } : resolver;
    }),
  };
}

const HEALTH_RANK: Record<MapHealth, number> = {
  critical: 5,
  attention: 4,
  stale: 3,
  unknown: 2,
  off: 1,
  ok: 0,
};

function worstHealth(values: readonly MapHealth[]): MapHealth {
  let worst: MapHealth = 'ok';
  for (const value of values) if (HEALTH_RANK[value] > HEALTH_RANK[worst]) worst = value;
  return worst;
}

const blockHosts = (block: MapBlock): MapHost[] =>
  block.kind === 'group' ? block.group.hosts : [block.host];

function summarize(
  blocks: MapBlock[],
  segments: MapSegment[],
  connections: MapConnections,
  latestSampleAtMs?: number,
  liveSamplesExpired?: boolean,
): InfrastructureMapModel {
  const counts = new Map<string, number>();
  const add = (key: string, by: number) => counts.set(key, (counts.get(key) ?? 0) + by);
  let attentionCount = 0;

  for (const block of blocks) {
    if (block.kind === 'group') {
      add('pools', block.group.pools.length);
      attentionCount += block.group.pools.filter((pool) => isProblem(pool.health)).length;
    }
    for (const host of blockHosts(block)) {
      if (host.isResource) {
        add('hosts', 1);
        if (isProblem(host.health)) attentionCount += 1;
      }
      add('pools', host.pools.length);
      attentionCount += host.pools.filter((pool) => isProblem(pool.health)).length;
      for (const section of host.sections) {
        const key = section.key.startsWith('ns:') ? 'pods' : section.key;
        add(key, section.items.length);
        attentionCount += section.items.filter((item) => isProblem(item.health)).length;
      }
    }
  }

  const TOTAL_LABELS: Array<[string, string, string]> = [
    ['hosts', 'Hosts', 'Host'],
    ['vms', 'VMs', 'VM'],
    ['lxcs', 'LXCs', 'LXC'],
    ['containers', 'Containers', 'Container'],
    ['pods', 'Pods', 'Pod'],
    ['pools', 'Storage', 'Storage'],
    ['checks', 'Checks', 'Check'],
  ];
  const totals = TOTAL_LABELS.map(([key, label, singular]) => ({
    key,
    label,
    singular,
    count: counts.get(key) ?? 0,
  })).filter((total) => total.count > 0);

  return {
    blocks,
    segments,
    connections,
    totals,
    attentionCount,
    isEmpty: blocks.length === 0,
    ...(latestSampleAtMs !== undefined ? { latestSampleAtMs } : {}),
    ...(liveSamplesExpired ? { liveSamplesExpired: true } : {}),
  };
}

/**
 * The same map with every name replaced by what the thing is, for an image
 * that can be posted without publishing hostnames. Versions, sizes, counts
 * and health stay: they are what makes the picture worth sharing.
 */
export function anonymizeInfrastructureMapModel(
  model: InfrastructureMapModel,
): InfrastructureMapModel {
  const counters = new Map<string, number>();
  const next = (label: string): string => {
    const count = (counters.get(label) ?? 0) + 1;
    counters.set(label, count);
    return `${label} ${count}`;
  };
  // One resource keeps one stand-in name wherever it is drawn.
  const names = new Map<string, string>();
  const nameFor = (id: string, label: string): string => {
    let name = names.get(id);
    if (!name) {
      name = next(label);
      names.set(id, name);
    }
    return name;
  };
  // Pools that share a name share a stand-in. The map draws a cluster's
  // `local` on every node as one tile, and it has to stay one tile with the
  // names hidden, or the image shows a different estate from the page.
  const anonymizePools = (pools: MapPool[]): MapPool[] =>
    pools.map((pool) => ({ ...pool, name: nameFor(`pool-name:${pool.name}`, 'Pool') }));
  const anonymizeItems = (items: MapItem[]): MapItem[] =>
    items.map(
      ({
        address: _address,
        dockerNetworks: _dockerNetworks,
        ingressHosts: _ingressHosts,
        composeProject: _composeProject,
        vmid: _vmid,
        ...item
      }) => ({
        ...item,
        // A well-known app keeps its own name: it says what runs here and
        // gives nothing away about the estate.
        name: item.app
          ? item.app.label
          : nameFor(
              item.id,
              (item.checkKind && CHECK_STAND_IN[item.checkKind]) || itemStandIn(item.typeLabel),
            ),
      }),
    );
  const anonymizeHost = ({ address: _address, ...host }: MapHost): MapHost => ({
    ...host,
    ...(host.gpuSummary
      ? { gpuSummary: /^\d+×\s+GPU$/i.test(host.gpuSummary) ? host.gpuSummary : 'GPU' }
      : {}),
    name: host.isResource ? nameFor(host.id, HOST_STAND_IN[host.platform]) : host.name,
    pools: anonymizePools(host.pools),
    sections: host.sections.map((section) => ({
      ...section,
      label: section.key.startsWith('ns:') ? next('Namespace') : section.label,
      items: anonymizeItems(section.items),
    })),
  });

  return {
    ...model,
    // A route's hostnames, estate-specific volume names, and a gateway's
    // address identify the estate. The connections and the counts stay: they
    // are the shape of it, not its names.
    connections: {
      links: model.connections.links.map(({ sensitiveDetail, detail, ...link }) => ({
        ...link,
        hosts: [],
        ...(!sensitiveDetail && detail ? { detail } : {}),
      })),
      gateways: model.connections.gateways.map((gateway) => ({ ...gateway, address: '' })),
      resolvers: model.connections.resolvers.map((resolver) => ({ ...resolver, address: '' })),
    },
    segments: model.segments.map((segment) => ({
      ...segment,
      label: segment.kind === 'other' ? segment.label : next('Network'),
      // A subnet, a cluster name or a bridge name says as much as the label.
      detail: undefined,
      bridge: undefined,
    })),
    blocks: model.blocks.map((block) =>
      block.kind === 'host'
        ? { kind: 'host', host: anonymizeHost(block.host) }
        : {
            kind: 'group',
            group: {
              ...block.group,
              name: next(block.group.typeLabel),
              hosts: block.group.hosts.map(anonymizeHost),
              pools: anonymizePools(block.group.pools),
            },
          },
    ),
  };
}
