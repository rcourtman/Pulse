import type { WorkloadGuest } from '@/types/workloads';
import { buildInfrastructurePath } from '@/routing/resourceLinks';
import { resolveWorkloadType } from '@/utils/workloads';

const firstNonEmpty = (values: Array<string | undefined | null>): string | undefined => {
  for (const value of values) {
    if (typeof value !== 'string') continue;
    const trimmed = value.trim();
    if (trimmed.length > 0) return trimmed;
  }
  return undefined;
};

export const buildInfrastructureHrefForWorkload = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);

  if (type === 'vm' || type === 'lxc') {
    const query = firstNonEmpty([guest.node, guest.instance, guest.name]);
    return buildInfrastructurePath({ source: 'proxmox', query });
  }

  if (type === 'docker') {
    const query = firstNonEmpty([guest.contextLabel, guest.node, guest.instance, guest.name]);
    return buildInfrastructurePath({ source: 'docker', query });
  }

  if (type === 'k8s') {
    const query = firstNonEmpty([
      guest.contextLabel,
      guest.instance,
      guest.namespace,
      guest.node,
      guest.name,
    ]);
    return buildInfrastructurePath({ source: 'kubernetes', query });
  }

  return buildInfrastructurePath();
};
