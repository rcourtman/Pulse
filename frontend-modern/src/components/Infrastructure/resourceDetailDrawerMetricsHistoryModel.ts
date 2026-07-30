import type { ResourceType as MetricsHistoryResourceType } from '@/api/charts';
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

const GPU_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] = [
  ...GUEST_DRAWER_HISTORY_GROUPS,
  {
    id: 'gpu-utilization',
    label: 'GPU Utilization',
    unit: '%',
    series: [
      { metric: 'gpu', label: 'Core', unit: '%', color: '#06b6d4' },
      { metric: 'gpu_memory', label: 'VRAM', unit: '%', color: '#6366f1' },
    ],
  },
  {
    id: 'gpu-thermal',
    label: 'GPU Thermal',
    unit: 'C',
    series: [{ metric: 'gpu_temperature', label: 'GPU', unit: 'C', color: '#ef4444' }],
  },
];

export const getResourceMetricsHistoryGroups = (
  resource: Resource,
): GuestDrawerHistoryGroupConfig[] =>
  (resource.agent?.sensors?.gpu?.length ?? 0) > 0
    ? GPU_HISTORY_GROUPS
    : GUEST_DRAWER_HISTORY_GROUPS;

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
// hands out (agent, vm, system-container, app-container, pod, disk, ceph).
// Gating on type === 'agent' silently hid history for Docker containers
// even though the store records their CPU/memory/disk/IO samples.
export const resourceSupportsMetricsHistory = (resource: Resource): boolean =>
  getResourceMetricsHistoryTarget(resource) !== null;

export const getResourceMetricsHistoryFallbackMetrics = (
  resource: Resource,
): Record<string, number | undefined> => {
  return {
    cpu: finiteMetric(resource.cpu?.current),
    memory: resource.memory ? finiteMetric(getMemoryPercent(resource)) : undefined,
    disk: resource.disk ? finiteMetric(getDiskPercent(resource)) : undefined,
    netin: finiteMetric(resource.network?.rxBytes),
    netout: finiteMetric(resource.network?.txBytes),
    diskread: finiteMetric(resource.diskIO?.readRate),
    diskwrite: finiteMetric(resource.diskIO?.writeRate),
    gpu: getResourceGPUUtilizationPercent(resource),
    gpu_memory: getResourceGPUMemoryPercent(resource),
    gpu_temperature: getResourceGPUTemperatureCelsius(resource),
  };
};
