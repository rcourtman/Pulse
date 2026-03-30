import type { WorkloadGuest, WorkloadType, ViewMode } from '@/types/workloads';
import type { ResourceType as DiscoveryResourceType } from '@/types/discovery';
import type { ResourceDiscoveryTarget } from '@/types/resource';
import type { MetricResourceKind } from '@/utils/metricsKeys';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';
import { canonicalDiscoveryResourceType } from '@/utils/discoveryTarget';

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

export const normalizeWorkloadViewModeParam = (value: string): ViewMode | null => {
  const normalized = value.trim().toLowerCase();
  if (normalized === 'all') return 'all';
  if (normalized === 'vm') return 'vm';
  if (normalized === 'system-container') return 'system-container';
  if (normalized === 'docker' || normalized === 'app-container') return 'app-container';
  if (normalized === 'k8s' || normalized === 'kubernetes' || normalized === 'pod') return 'pod';
  return null;
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

export const isDockerManagedAppContainer = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type' | 'platformType' | 'containerRuntime'>,
): boolean => {
  if (resolveWorkloadType(guest) !== 'app-container') return false;

  const platform = (guest.platformType || '').trim().toLowerCase();
  if (platform === 'truenas') return false;
  if (platform === 'docker') return true;

  const rawType = (guest.type || '').trim().toLowerCase();
  if (rawType === 'docker') return true;

  const runtime = (guest.containerRuntime || '').trim().toLowerCase();
  return runtime === 'docker' || runtime === 'podman';
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

export const resolveDiscoveryTargetForWorkload = (
  guest: Pick<
    WorkloadGuest,
    | 'discoveryTarget'
    | 'workloadType'
    | 'type'
    | 'platformType'
    | 'containerRuntime'
    | 'dockerHostId'
    | 'kubernetesAgentId'
    | 'instance'
    | 'node'
    | 'vmid'
    | 'id'
  >,
): ResourceDiscoveryTarget | null => {
  const explicit = guest.discoveryTarget;
  if (explicit) {
    const resourceType = canonicalDiscoveryResourceType(explicit.resourceType);
    const agentId = (explicit.agentId || '').trim();
    const resourceId = (explicit.resourceId || '').trim();
    if (resourceType && agentId && resourceId) {
      return {
        resourceType: resourceType as ResourceDiscoveryTarget['resourceType'],
        agentId,
        resourceId,
        hostname: explicit.hostname,
      };
    }
  }

  const type = resolveWorkloadType(guest);
  if (type === 'app-container') {
    if (!isDockerManagedAppContainer(guest)) return null;
    const agentId = (guest.dockerHostId || '').trim();
    const resourceId = (guest.id || '').trim();
    return agentId && resourceId
      ? { resourceType: 'app-container', agentId, resourceId }
      : null;
  }
  if (type === 'pod') {
    const agentId = (guest.kubernetesAgentId || guest.instance || guest.node || '').trim();
    const rawId = (guest.id || '').trim();
    const match = rawId.match(/^k8s:[^:]+:pod:(.+)$/);
    const resourceId = (match?.[1] || rawId).trim();
    return agentId && resourceId ? { resourceType: 'pod', agentId, resourceId } : null;
  }
  if ((type === 'vm' || type === 'system-container') && Number.isFinite(guest.vmid) && guest.vmid > 0) {
    const agentId = (guest.node || '').trim();
    const resourceId = String(guest.vmid);
    return agentId ? { resourceType: type, agentId, resourceId } : null;
  }
  return null;
};

export const getDiscoveryResourceTypeForWorkload = (
  guest: Pick<
    WorkloadGuest,
    | 'discoveryTarget'
    | 'workloadType'
    | 'type'
    | 'platformType'
    | 'containerRuntime'
    | 'dockerHostId'
    | 'kubernetesAgentId'
    | 'instance'
    | 'node'
    | 'vmid'
    | 'id'
  >,
): DiscoveryResourceType | null =>
  (resolveDiscoveryTargetForWorkload(guest)?.resourceType as DiscoveryResourceType | undefined) ??
  null;

export const hasDiscoverySupportForWorkload = (
  guest: Pick<
    WorkloadGuest,
    | 'discoveryTarget'
    | 'workloadType'
    | 'type'
    | 'platformType'
    | 'containerRuntime'
    | 'dockerHostId'
    | 'kubernetesAgentId'
    | 'instance'
    | 'node'
    | 'vmid'
    | 'id'
  >,
): boolean => Boolean(resolveDiscoveryTargetForWorkload(guest));

export const getWebInterfaceTargetLabelForWorkload = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type'>,
): 'container' | 'workload' =>
  resolveWorkloadType(guest) === 'app-container' ? 'container' : 'workload';
