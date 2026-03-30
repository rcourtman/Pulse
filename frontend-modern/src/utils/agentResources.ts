import type { Resource, ResourceType } from '@/types/resource';
import {
  getAgentDiscoveryResourceId,
  isAgentDiscoveryResourceType,
  isAppContainerDiscoveryResourceType,
} from '@/utils/discoveryTarget';
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import { asTrimmedString } from '@/utils/stringUtils';

const AGENT_FACET_INFRASTRUCTURE_TYPES = new Set<ResourceType>(['agent', 'pbs', 'pmg']);
const AGENT_PROFILE_ASSIGNABLE_TYPES = new Set<ResourceType>([
  'docker-host',
  'agent',
  'pbs',
  'pmg',
  'k8s-cluster',
]);

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

type KubernetesContextLike = {
  clusterId?: string | null;
  name?: string | null;
  kubernetes?: {
    clusterName?: string | null;
    context?: string | null;
    clusterId?: string | null;
  } | null;
  platformData?: {
    kubernetes?: {
      clusterName?: string | null;
      context?: string | null;
      clusterId?: string | null;
    } | null;
  } | null;
};

type ResourceClusterNameLike = KubernetesContextLike & {
  identity?: {
    clusterName?: string | null;
  } | null;
  proxmox?: {
    clusterName?: string | null;
  } | null;
  platformData?: {
    kubernetes?: {
      clusterName?: string | null;
      context?: string | null;
      clusterId?: string | null;
    } | null;
    proxmox?: {
      clusterName?: string | null;
    } | null;
  } | null;
};

export const getPlatformDataRecord = (resource: Resource): Record<string, unknown> | undefined =>
  resource.platformData ? (resource.platformData as Record<string, unknown>) : undefined;

const hasPlatformSource = (resource: Resource, source: string): boolean => {
  const sources = getPlatformDataRecord(resource)?.sources;
  if (!Array.isArray(sources)) return false;
  return sources.some((value) => normalizeSourcePlatformKey(value) === source);
};

export const getPlatformAgentRecord = (resource: Resource): Record<string, unknown> | undefined =>
  asRecord(getPlatformDataRecord(resource)?.agent);

export const isTrueNASSystemResource = (resource: Resource): boolean =>
  resource.type === 'agent' &&
  (resource.platformType === 'truenas' || hasPlatformSource(resource, 'truenas'));

export const getExplicitAgentIdFromResource = (resource: Resource): string | undefined => {
  const platformData = getPlatformDataRecord(resource);
  const platformAgent = getPlatformAgentRecord(resource);
  const kubernetes = asRecord(platformData?.kubernetes);

  return (
    asTrimmedString(resource.agent?.agentId) ||
    asTrimmedString(platformAgent?.agentId) ||
    asTrimmedString(platformData?.agentId) ||
    asTrimmedString(platformData?.linkedAgentId) ||
    asTrimmedString(resource.kubernetes?.agentId) ||
    asTrimmedString(kubernetes?.agentId)
  );
};

export const getActionableAgentIdFromResource = (resource: Resource): string | undefined =>
  getExplicitAgentIdFromResource(resource) ||
  getAgentDiscoveryResourceId(resource.discoveryTarget ?? null) ||
  asTrimmedString(resource.discoveryTarget?.agentId);

export const getActionableDockerRuntimeIdFromResource = (
  resource: Resource,
): string | undefined => {
  const platformData = getPlatformDataRecord(resource);
  const docker = asRecord(platformData?.docker);
  const identity = asRecord(resource.identity);

  if (
    isAppContainerDiscoveryResourceType(resource.discoveryTarget?.resourceType) &&
    resource.discoveryTarget?.resourceId
  ) {
    return resource.discoveryTarget.resourceId;
  }

  return (
    asTrimmedString(docker?.hostSourceId) ||
    asTrimmedString(platformData?.hostSourceId) ||
    (resource.type === 'docker-host'
      ? asTrimmedString(identity?.machineId) || asTrimmedString(platformData?.machineId)
      : undefined) ||
    (resource.metricsTarget?.resourceType === 'docker-host'
      ? asTrimmedString(resource.metricsTarget.resourceId)
      : undefined) ||
    (resource.type === 'docker-host' ? asTrimmedString(resource.discoveryTarget?.agentId) : undefined)
  );
};

export const hasDockerWorkloadsScope = (resource: Resource): boolean => {
  const platformData = getPlatformDataRecord(resource);
  return resource.type === 'docker-host' || Boolean(asRecord(platformData?.docker));
};

export const getActionableKubernetesClusterIdFromResource = (
  resource: Resource,
): string | undefined => {
  if (resource.discoveryTarget?.resourceType === 'pod' && resource.discoveryTarget.resourceId) {
    return resource.discoveryTarget.resourceId;
  }

  return (
    getPreferredResourceKubernetesContext(resource) ||
    (resource.type === 'k8s-cluster' ? resource.id : undefined)
  );
};

export const getPreferredResourceKubernetesContext = (
  resource: KubernetesContextLike,
): string | undefined => {
  return (
    asTrimmedString(resource.kubernetes?.clusterName) ||
    asTrimmedString(resource.kubernetes?.context) ||
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(resource.platformData?.kubernetes?.clusterName) ||
    asTrimmedString(resource.platformData?.kubernetes?.context) ||
    asTrimmedString(resource.platformData?.kubernetes?.clusterId) ||
    asTrimmedString(resource.clusterId)
  );
};

export const getPreferredResourceClusterName = (
  resource: ResourceClusterNameLike,
): string | undefined =>
  getPreferredResourceKubernetesContext(resource) ||
  asTrimmedString(resource.identity?.clusterName) ||
  asTrimmedString(resource.proxmox?.clusterName) ||
  asTrimmedString(resource.platformData?.proxmox?.clusterName) ||
  asTrimmedString(resource.name);

export const getMetricsChartKeyCandidatesFromResource = (resource: Resource): string[] => {
  const candidates = [
    asTrimmedString(resource.metricsTarget?.resourceId),
    getActionableDockerRuntimeIdFromResource(resource),
    getActionableKubernetesClusterIdFromResource(resource),
    getActionableAgentIdFromResource(resource),
    asTrimmedString(resource.discoveryTarget?.resourceId),
    asTrimmedString(resource.discoveryTarget?.agentId),
    asTrimmedString(resource.id),
    asTrimmedString(resource.name),
    asTrimmedString(resource.platformId),
  ].filter((value): value is string => Boolean(value));

  return Array.from(new Set(candidates));
};

export const hasAgentFacet = (resource: Resource): boolean => {
  const discoveryTarget = resource.discoveryTarget;
  return Boolean(
    resource.agent ||
      getPlatformAgentRecord(resource) ||
      getExplicitAgentIdFromResource(resource) ||
      (isAgentDiscoveryResourceType(discoveryTarget?.resourceType) && discoveryTarget?.agentId),
  );
};

export const isAgentFacetInfrastructureResource = (resource: Resource): boolean =>
  AGENT_FACET_INFRASTRUCTURE_TYPES.has(resource.type) && hasAgentFacet(resource);

export const isAgentProfileAssignableResource = (resource: Resource): boolean =>
  AGENT_PROFILE_ASSIGNABLE_TYPES.has(resource.type);
