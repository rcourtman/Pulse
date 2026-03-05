type DiscoveryTargetLike = {
  resourceType?: string;
  resourceId?: string;
  agentId?: string;
} | null;

const asTrimmedString = (value: unknown): string | undefined => {
  if (typeof value !== 'string') return undefined;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
};

export const isAgentDiscoveryResourceType = (resourceType: unknown): boolean => {
  const normalized = asTrimmedString(resourceType)?.toLowerCase();
  return normalized === 'agent';
};

export const getAgentDiscoveryResourceId = (
  discoveryTarget: DiscoveryTargetLike,
): string | undefined => {
  if (!discoveryTarget || !isAgentDiscoveryResourceType(discoveryTarget.resourceType)) {
    return undefined;
  }
  return (
    asTrimmedString(discoveryTarget.resourceId) ||
    asTrimmedString(discoveryTarget.agentId)
  );
};
