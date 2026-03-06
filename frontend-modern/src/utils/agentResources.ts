import type { Resource, ResourceType } from '@/types/resource';
import {
  getAgentDiscoveryResourceId,
  isAgentDiscoveryResourceType,
  isAppContainerDiscoveryResourceType,
} from '@/utils/discoveryTarget';

const AGENT_FACET_INFRASTRUCTURE_TYPES = new Set<ResourceType>([
  'agent',
  'pbs',
  'pmg',
  'truenas',
]);
const AGENT_PROFILE_ASSIGNABLE_TYPES = new Set<ResourceType>([
  'docker-host',
  'agent',
  'pbs',
  'pmg',
  'truenas',
  'k8s-cluster',
]);

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asTrimmedString = (value: unknown): string | undefined => {
  if (typeof value !== 'string') return undefined;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
};

export const getPlatformDataRecord = (resource: Resource): Record<string, unknown> | undefined =>
  resource.platformData ? (resource.platformData as Record<string, unknown>) : undefined;

export const getPlatformAgentRecord = (resource: Resource): Record<string, unknown> | undefined =>
  asRecord(getPlatformDataRecord(resource)?.agent);

export const getExplicitAgentIdFromResource = (resource: Resource): string | undefined => {
  const platformData = getPlatformDataRecord(resource);
  const platformAgent = getPlatformAgentRecord(resource);
  const kubernetes = asRecord(platformData?.kubernetes);

  return (
    asTrimmedString(resource.agent?.agentId) ||
    asTrimmedString(platformAgent?.agentId) ||
    asTrimmedString(platformData?.agentId) ||
    asTrimmedString(resource.kubernetes?.agentId) ||
    asTrimmedString(kubernetes?.agentId)
  );
};

export const getActionableAgentIdFromResource = (resource: Resource): string | undefined =>
  getExplicitAgentIdFromResource(resource) ||
  getAgentDiscoveryResourceId(resource.discoveryTarget) ||
  asTrimmedString(resource.discoveryTarget?.agentId);

export const getActionableDockerRuntimeIdFromResource = (
  resource: Resource,
): string | undefined => {
  const platformData = getPlatformDataRecord(resource);
  const docker = asRecord(platformData?.docker);

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
      ? asTrimmedString(resource.discoveryTarget?.agentId) || resource.id
      : undefined)
  );
};

export const getActionableKubernetesClusterIdFromResource = (
  resource: Resource,
): string | undefined => {
  const platformData = getPlatformDataRecord(resource);
  const kubernetes = asRecord(platformData?.kubernetes);

  if (resource.discoveryTarget?.resourceType === 'k8s' && resource.discoveryTarget.resourceId) {
    return resource.discoveryTarget.resourceId;
  }

  return (
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(kubernetes?.clusterId) ||
    asTrimmedString(platformData?.clusterId) ||
    (resource.type === 'k8s-cluster' ? resource.id : undefined)
  );
};

export const hasAgentFacet = (resource: Resource): boolean =>
  Boolean(
    resource.agent ||
      getPlatformAgentRecord(resource) ||
      getExplicitAgentIdFromResource(resource) ||
      (isAgentDiscoveryResourceType(resource.discoveryTarget?.resourceType) &&
        resource.discoveryTarget.agentId),
  );

export const isAgentFacetInfrastructureResource = (resource: Resource): boolean =>
  AGENT_FACET_INFRASTRUCTURE_TYPES.has(resource.type) && hasAgentFacet(resource);

export const isAgentProfileAssignableResource = (resource: Resource): boolean =>
  AGENT_PROFILE_ASSIGNABLE_TYPES.has(resource.type);
