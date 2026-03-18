import type { WorkloadGuest, WorkloadType } from '@/types/workloads';
import type { MetricResourceKind } from '@/utils/metricsKeys';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

/**
 * Resolve a raw type string (from API or backend) to a semantic WorkloadType.
 * Returns null when the value cannot be mapped to any known workload type.
 */
export const resolveWorkloadTypeFromString = (value?: string | null): WorkloadType | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'vm' || normalized === 'qemu') return 'vm';
  if (
    normalized === 'system-container' ||
    normalized === 'oci-container' ||
    normalized === 'lxc' ||
    normalized === 'incus' ||
    normalized === 'jail'
  ) {
    return 'system-container';
  }
  if (normalized === 'docker' || normalized === 'app-container') {
    return 'app-container';
  }
  if (
    normalized === 'pod' ||
    normalized === 'k8s' ||
    normalized === 'kubernetes' ||
    normalized === 'k8s-pod'
  ) {
    return 'pod';
  }
  return null;
};

export const canonicalizeWorkloadFilterType = (value?: string | null): string => {
  const normalized = (value || '').trim().toLowerCase();
  return canonicalizeFrontendResourceType(normalized) || normalized;
};

export const resolveWorkloadType = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type'>,
): WorkloadType => {
  const explicitType = resolveWorkloadTypeFromString(guest.workloadType);
  if (explicitType) return explicitType;
  return resolveWorkloadTypeFromString(guest.type) ?? 'system-container';
};

export const getWorkloadMetricsKind = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type'>,
): MetricResourceKind => {
  const type = resolveWorkloadType(guest);
  switch (type) {
    case 'vm':
      return 'vm';
    case 'app-container':
      return 'dockerContainer';
    case 'pod':
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
