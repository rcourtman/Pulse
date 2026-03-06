import type { Resource, ResourceType } from '@/types/resource';
import { getAgentDiscoveryResourceId, isAgentDiscoveryResourceType } from '@/utils/discoveryTarget';

const AGENT_FACET_INFRASTRUCTURE_TYPES = new Set<ResourceType>(['node', 'pbs', 'pmg', 'truenas']);
const AGENT_PROFILE_ASSIGNABLE_TYPES = new Set<ResourceType>([
  'docker-host',
  'node',
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
