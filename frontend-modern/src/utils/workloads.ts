import type { WorkloadGuest, WorkloadType } from '@/types/workloads';
import type { MetricResourceKind } from '@/utils/metricsKeys';

/**
 * Resolve a raw type string (from API or backend) to a semantic WorkloadType.
 * Returns null when the value cannot be mapped to any known workload type.
 */
export const resolveWorkloadTypeFromString = (value?: string | null): WorkloadType | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'vm') return 'vm';
  if (normalized === 'system-container' || normalized === 'oci-container') {
    return 'system-container';
  }
  if (
    normalized === 'docker' ||
    normalized === 'app-container'
  ) {
    return 'docker';
  }
  if (
    normalized === 'pod' ||
    normalized === 'k8s' ||
    normalized === 'kubernetes' ||
    normalized === 'k8s-pod'
  ) {
    return 'k8s';
  }
  return null;
};

export const resolveWorkloadType = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type'>,
): WorkloadType => {
  if (guest.workloadType) return guest.workloadType;
  return resolveWorkloadTypeFromString(guest.type) ?? 'system-container';
};

export const getWorkloadMetricsKind = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type'>,
): MetricResourceKind => {
  const type = resolveWorkloadType(guest);
  switch (type) {
    case 'vm':
      return 'vm';
    case 'docker':
      return 'dockerContainer';
    case 'k8s':
      return 'k8s';
    case 'system-container':
    default:
      return 'container';
  }
};

export const getCanonicalWorkloadId = (
  guest: Pick<WorkloadGuest, 'id' | 'workloadType' | 'type' | 'instance' | 'node' | 'vmid'>,
): string => {
  const type = resolveWorkloadType(guest);
  if (
    (type === 'vm' || type === 'system-container') &&
    guest.instance &&
    guest.node &&
    guest.vmid > 0
  ) {
    return `${guest.instance}:${guest.node}:${guest.vmid}`;
  }
  return guest.id;
};
