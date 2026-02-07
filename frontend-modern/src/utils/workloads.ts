import type { WorkloadGuest, WorkloadType } from '@/types/workloads';
import type { MetricResourceKind } from '@/utils/metricsKeys';

export const resolveWorkloadType = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type'>,
): WorkloadType => {
  if (guest.workloadType) return guest.workloadType;
  const rawType = (guest.type || '').toLowerCase();
  if (rawType === 'qemu' || rawType === 'vm') return 'vm';
  if (rawType === 'lxc' || rawType === 'oci' || rawType === 'container') return 'lxc';
  if (rawType === 'docker' || rawType === 'docker-container' || rawType === 'docker_container') {
    return 'docker';
  }
  if (rawType === 'k8s' || rawType === 'pod' || rawType === 'kubernetes') return 'k8s';
  return 'lxc';
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
    case 'lxc':
    default:
      return 'container';
  }
};

export const getCanonicalWorkloadId = (
  guest: Pick<WorkloadGuest, 'id' | 'workloadType' | 'type' | 'instance' | 'node' | 'vmid'>,
): string => {
  const type = resolveWorkloadType(guest);
  if ((type === 'vm' || type === 'lxc') && guest.instance && guest.node && guest.vmid > 0) {
    return `${guest.instance}:${guest.node}:${guest.vmid}`;
  }
  return guest.id;
};
