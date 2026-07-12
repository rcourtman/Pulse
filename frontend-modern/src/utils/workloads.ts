import type {
  WorkloadContainerViewMode,
  WorkloadGuest,
  WorkloadType,
  ViewMode,
} from '@/types/workloads';
import type { ResourceType as DiscoveryResourceType } from '@/types/discovery';
import type { Resource, ResourceDiscoveryTarget } from '@/types/resource';
import type { MetricResourceKind } from '@/utils/metricsKeys';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';
import { canonicalDiscoveryResourceType } from '@/utils/discoveryTarget';
import {
  normalizeSourcePlatformQueryValue,
  normalizeSourcePlatformScopes,
} from '@/utils/sourcePlatforms';

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
  if (normalized === 'container') return 'container';
  if (normalized === 'system-container') return 'system-container';
  if (normalized === 'docker' || normalized === 'app-container') return 'app-container';
  if (normalized === 'k8s' || normalized === 'kubernetes' || normalized === 'pod') return 'pod';
  return null;
};

export const isContainerWorkloadType = (
  value: WorkloadType,
): value is 'system-container' | 'app-container' =>
  value === 'system-container' || value === 'app-container';

export const isContainerWorkloadViewMode = (value: ViewMode): value is WorkloadContainerViewMode =>
  value === 'container' || value === 'system-container' || value === 'app-container';

export const workloadMatchesViewMode = (
  workloadType: WorkloadType,
  viewMode: ViewMode,
): boolean => {
  if (viewMode === 'all') return true;
  if (viewMode === 'container') return isContainerWorkloadType(workloadType);
  return workloadType === viewMode;
};

export const getWorkloadPlatformScopes = (
  guest: Pick<WorkloadGuest, 'platformScopes' | 'platformType'>,
): string[] => normalizeSourcePlatformScopes(guest.platformScopes, guest.platformType);

export const workloadMatchesPlatformScope = (
  guest: Pick<WorkloadGuest, 'platformScopes' | 'platformType'>,
  platformScope?: string | null,
): boolean => {
  const normalizedScope = normalizeSourcePlatformQueryValue(platformScope);
  if (!normalizedScope || normalizedScope === 'all') return true;
  return getWorkloadPlatformScopes(guest).includes(normalizedScope);
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
  if (type === 'vm' || type === 'system-container') {
    const canonicalId = buildCanonicalNodeScopedWorkloadId({
      instance: guest.instance,
      node: guest.node,
      vmid: guest.vmid,
    });
    if (canonicalId) {
      return canonicalId;
    }
  }
  return guest.id;
};

export const buildAppContainerMetadataId = ({
  dockerHostId,
  name,
}: Pick<WorkloadGuest, 'dockerHostId' | 'name'>): string | null => {
  const hostId = (dockerHostId || '').trim();
  const containerName = (name || '').trim().replace(/^\/+/, '');
  if (!hostId || !containerName) return null;
  return `app-container:${hostId}:name:${containerName}`;
};

export const getWorkloadMetadataIdCandidates = (
  guest: Pick<
    WorkloadGuest,
    'id' | 'workloadType' | 'type' | 'instance' | 'node' | 'vmid' | 'dockerHostId' | 'name'
  >,
): string[] => {
  const canonicalId = getCanonicalWorkloadId(guest);
  const candidates: string[] = [];

  if (isDockerManagedAppContainer(guest)) {
    const appContainerId = buildAppContainerMetadataId(guest);
    if (appContainerId) candidates.push(appContainerId);
  }

  if (canonicalId) candidates.push(canonicalId);

  return [...new Set(candidates)];
};

export const getWorkloadMetadataId = (
  guest: Pick<
    WorkloadGuest,
    'id' | 'workloadType' | 'type' | 'instance' | 'node' | 'vmid' | 'dockerHostId' | 'name'
  >,
): string => getWorkloadMetadataIdCandidates(guest)[0] || getCanonicalWorkloadId(guest);

type NodeScopedWorkloadIdentity = {
  instance?: string | null;
  node?: string | null;
  vmid?: number | null;
};

const normalizeNodeScopedWorkloadKeyPart = (value?: string | null): string => (value || '').trim();

export const buildCanonicalNodeScopedWorkloadId = ({
  instance,
  node,
  vmid,
}: NodeScopedWorkloadIdentity): string | null => {
  const normalizedInstance = normalizeNodeScopedWorkloadKeyPart(instance);
  const normalizedNode = normalizeNodeScopedWorkloadKeyPart(node);
  const normalizedVmid = Number.isFinite(vmid) ? Number(vmid) : 0;
  if (!normalizedInstance || !normalizedNode || normalizedVmid <= 0) {
    return null;
  }
  return `${normalizedInstance}:${normalizedNode}:${normalizedVmid}`;
};

export const getCanonicalWorkloadIdForResource = (
  resource: Pick<Resource, 'id' | 'type' | 'clusterId' | 'proxmox'>,
): string => {
  const workloadType = resolveWorkloadTypeFromString(resource.type);
  if (workloadType === 'vm' || workloadType === 'system-container') {
    const canonicalId = buildCanonicalNodeScopedWorkloadId({
      instance: resource.proxmox?.instance || resource.clusterId,
      node: resource.proxmox?.node || resource.proxmox?.nodeName,
      vmid: resource.proxmox?.vmid,
    });
    if (canonicalId) {
      return canonicalId;
    }
  }
  return resource.id;
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
    | 'containerId'
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
    // Discovery shells out to `docker exec <id> ...` on the agent host, so we
    // need the Docker-native container id (or name), not the synthetic
    // canonical workload id used for UI routing.
    const resourceId = (guest.containerId || '').trim();
    return agentId && resourceId ? { resourceType: 'app-container', agentId, resourceId } : null;
  }
  if (type === 'pod') {
    const agentId = (guest.kubernetesAgentId || guest.instance || guest.node || '').trim();
    const rawId = (guest.id || '').trim();
    const match = rawId.match(/^k8s:[^:]+:pod:(.+)$/);
    const resourceId = (match?.[1] || rawId).trim();
    return agentId && resourceId ? { resourceType: 'pod', agentId, resourceId } : null;
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
    | 'containerId'
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
    | 'containerId'
  >,
): boolean => Boolean(resolveDiscoveryTargetForWorkload(guest));

export const getWebInterfaceTargetLabelForWorkload = (
  guest: Pick<WorkloadGuest, 'workloadType' | 'type'>,
): 'container' | 'workload' =>
  resolveWorkloadType(guest) === 'app-container' ? 'container' : 'workload';
