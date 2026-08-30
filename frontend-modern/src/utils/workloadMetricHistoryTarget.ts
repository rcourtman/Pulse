import type { ResourceType } from '@/api/charts';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId, resolveWorkloadType } from '@/utils/workloads';

export interface WorkloadMetricHistoryTarget {
  resourceType: ResourceType;
  resourceId: string;
}

export const getWorkloadMetricHistoryTarget = (
  guest: WorkloadGuest,
): WorkloadMetricHistoryTarget | null => {
  const explicitResourceType = guest.metricsTarget?.resourceType;
  const explicitResourceId = guest.metricsTarget?.resourceId?.trim();
  if (explicitResourceType && explicitResourceId) {
    return { resourceType: explicitResourceType, resourceId: explicitResourceId };
  }

  const resourceId = getCanonicalWorkloadId(guest).trim();
  if (!resourceId) return null;

  switch (resolveWorkloadType(guest)) {
    case 'vm':
      return { resourceType: 'vm', resourceId };
    case 'system-container':
      return { resourceType: 'system-container', resourceId };
    case 'app-container':
      return { resourceType: 'app-container', resourceId };
    case 'pod':
      return { resourceType: 'pod', resourceId };
    default:
      return null;
  }
};
