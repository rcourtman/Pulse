import type { ResourceType as MetricsHistoryResourceType } from '@/api/charts';
import {
  GPU_METRICS_HISTORY_GROUPS,
  HOST_METRICS_HISTORY_GROUPS,
} from '@/components/shared/hostMetricsHistoryModel';
import type { GuestDrawerHistoryTarget } from '@/components/Workloads/guestDrawerModel';
import {
  GUEST_DRAWER_HISTORY_GROUPS,
  type GuestDrawerHistoryGroupConfig,
} from '@/components/Workloads/guestDrawerModel';
import type { HostGPUSensor } from '@/types/api';
import type { Resource } from '@/types/resource';
import { getDiskPercent, getMemoryPercent } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const maxFiniteGPUValue = (
  resource: Resource,
  select: (gpu: HostGPUSensor) => number | undefined,
): number | undefined => {
  let maximum: number | undefined;
  for (const gpu of resource.agent?.sensors?.gpu ?? []) {
    const value = finiteMetric(select(gpu));
    if (value === undefined) continue;
    maximum = maximum === undefined ? value : Math.max(maximum, value);
  }
  return maximum;
};

export const getResourceGPUUtilizationPercent = (resource: Resource): number | undefined =>
  maxFiniteGPUValue(resource, (gpu) => {
    const utilization = finiteMetric(gpu.utilizationPercent);
    return utilization !== undefined && utilization >= 0 && utilization <= 100
      ? utilization
      : undefined;
  });

export const getResourceGPUMemoryPercent = (resource: Resource): number | undefined =>
  maxFiniteGPUValue(resource, (gpu) => {
    const used = finiteMetric(gpu.memoryUsedBytes);
    const total = finiteMetric(gpu.memoryTotalBytes);
    if (used === undefined || total === undefined || used < 0 || total <= 0) return undefined;
    return Math.min(100, Math.max(0, (used / total) * 100));
  });

export const getResourceGPUTemperatureCelsius = (resource: Resource): number | undefined =>
  maxFiniteGPUValue(resource, (gpu) => {
    const temperature = finiteMetric(gpu.temperatureCelsius);
    return temperature !== undefined && temperature > 0 && temperature <= 150
      ? temperature
      : undefined;
  });

const NODE_METRICS_HISTORY_GROUPS = HOST_METRICS_HISTORY_GROUPS.filter(
  (group) => group.id !== 'disk-io',
);

const STORAGE_METRICS_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] = [
  {
    id: 'capacity',
    label: 'Capacity',
    unit: '%',
    series: [{ metric: 'usage', label: 'Used', unit: '%', color: '#22c55e' }],
  },
];

const DISK_METRICS_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] = [
  {
    id: 'disk-activity',
    label: 'Activity',
    unit: '%',
    series: [{ metric: 'disk', label: 'Busy', unit: '%', color: '#8b5cf6' }],
  },
  {
    id: 'disk-io',
    label: 'Disk I/O',
    unit: 'B/s',
    series: [
      { metric: 'diskread', label: 'Read', unit: 'B/s', color: '#3b82f6' },
      { metric: 'diskwrite', label: 'Write', unit: 'B/s', color: '#f59e0b' },
    ],
  },
  {
    id: 'disk-thermal',
    label: 'Temperature',
    unit: 'C',
    series: [{ metric: 'smart_temp', label: 'Disk', unit: 'C', color: '#ef4444' }],
  },
];

const getBaseMetricsHistoryGroups = (
  resourceType: MetricsHistoryResourceType | undefined,
): GuestDrawerHistoryGroupConfig[] => {
  switch (resourceType) {
    case 'agent':
    case 'docker-host':
      return HOST_METRICS_HISTORY_GROUPS;
    case 'node':
      return NODE_METRICS_HISTORY_GROUPS;
    case 'storage':
      return STORAGE_METRICS_HISTORY_GROUPS;
    case 'disk':
      return DISK_METRICS_HISTORY_GROUPS;
    default:
      return GUEST_DRAWER_HISTORY_GROUPS;
  }
};

export const getResourceMetricsHistoryGroups = (
  resource: Resource,
): GuestDrawerHistoryGroupConfig[] => {
  const target = getResourceMetricsHistoryTarget(resource);
  const baseGroups = getBaseMetricsHistoryGroups(target?.resourceType);
  const supportsGPUHistory =
    target?.resourceType === 'agent' ||
    target?.resourceType === 'docker-host' ||
    target?.resourceType === 'node';
  return supportsGPUHistory && (resource.agent?.sensors?.gpu?.length ?? 0) > 0
    ? [...baseGroups, ...GPU_METRICS_HISTORY_GROUPS]
    : baseGroups;
};

export const getResourceMetricsHistoryTarget = (
  resource: Resource,
): GuestDrawerHistoryTarget | null => {
  const metricsType = asTrimmedString(resource.metricsTarget?.resourceType);
  const metricsId = asTrimmedString(resource.metricsTarget?.resourceId);
  if (metricsType && metricsId) {
    return {
      resourceType: metricsType as MetricsHistoryResourceType,
      resourceId: metricsId,
    };
  }

  if (resource.type === 'agent') {
    const resourceId = asTrimmedString(resource.id);
    return resourceId ? { resourceType: 'agent', resourceId } : null;
  }

  return null;
};

// Any resource that resolves a metrics history target can chart history —
// the backend stores and serves series for every type resolveMetricsTarget
// hands out. Each target type selects the catalog that matches the metrics
// actually persisted for it; storage and physical disks must not inherit the
// CPU/network workload catalog merely because they use the shared renderer.
// Gating on type === 'agent' silently hid history for Docker containers
// even though the store records their CPU/memory/disk/IO samples.
export const resourceSupportsMetricsHistory = (resource: Resource): boolean =>
  getResourceMetricsHistoryTarget(resource) !== null;

export const getResourceMetricsHistoryCurrentMetrics = (
  resource: Resource,
): Record<string, number | undefined> => {
  const diskPercent = resource.disk ? finiteMetric(getDiskPercent(resource)) : undefined;
  const temperature = finiteMetric(resource.temperature);
  return {
    cpu: finiteMetric(resource.cpu?.current),
    memory: resource.memory ? finiteMetric(getMemoryPercent(resource)) : undefined,
    disk: diskPercent,
    usage: diskPercent,
    netin: finiteMetric(resource.network?.rxBytes),
    netout: finiteMetric(resource.network?.txBytes),
    diskread: finiteMetric(resource.diskIO?.readRate),
    diskwrite: finiteMetric(resource.diskIO?.writeRate),
    temperature,
    smart_temp: temperature,
    gpu: getResourceGPUUtilizationPercent(resource),
    gpu_memory: getResourceGPUMemoryPercent(resource),
    gpu_temperature: getResourceGPUTemperatureCelsius(resource),
  };
};
