import type { ResourceType } from '@/api/charts';
import type { WorkloadGuest } from '@/types/workloads';
import {
  buildProxmoxGuestNativeId,
  getCanonicalWorkloadId,
  resolveWorkloadType,
} from '@/utils/workloads';

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

  const workloadType = resolveWorkloadType(guest);
  // Legacy guest inputs have native coordinates but no backend metrics target.
  // The History API reads native metric keys, independently of row identity.
  const nativeGuestId =
    workloadType === 'vm' || workloadType === 'system-container'
      ? buildProxmoxGuestNativeId(guest)
      : null;
  const resourceId = (nativeGuestId || getCanonicalWorkloadId(guest)).trim();
  if (!resourceId) return null;

  switch (workloadType) {
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
