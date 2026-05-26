import type { ResourceType as MetricsHistoryResourceType } from '@/api/charts';
import type { GuestDrawerHistoryTarget } from '@/components/Workloads/guestDrawerModel';
import type { Resource } from '@/types/resource';
import { getDiskPercent, getMemoryPercent } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

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

export const resourceSupportsMetricsHistory = (resource: Resource): boolean =>
  resource.type === 'agent' && getResourceMetricsHistoryTarget(resource) !== null;

export const getResourceMetricsHistoryFallbackMetrics = (
  resource: Resource,
): Record<string, number | undefined> => {
  if (resource.type !== 'agent') {
    return {};
  }

  return {
    cpu: finiteMetric(resource.cpu?.current),
    memory: resource.memory ? finiteMetric(getMemoryPercent(resource)) : undefined,
    disk: resource.disk ? finiteMetric(getDiskPercent(resource)) : undefined,
    netin: finiteMetric(resource.network?.rxBytes),
    netout: finiteMetric(resource.network?.txBytes),
    diskread: finiteMetric(resource.diskIO?.readRate),
    diskwrite: finiteMetric(resource.diskIO?.writeRate),
  };
};
