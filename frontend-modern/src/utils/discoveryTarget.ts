import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

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

const normalizeDiscoveryResourceType = (resourceType: unknown): string | undefined => {
  return asTrimmedString(resourceType)?.toLowerCase();
};

export const canonicalDiscoveryResourceType = (resourceType: unknown): string | undefined => {
  const normalized = normalizeDiscoveryResourceType(resourceType);
  if (!normalized) return undefined;
  return canonicalizeFrontendResourceType(normalized) || normalized;
};

export const toDiscoveryAPIResourceType = (resourceType: unknown): string | undefined => {
  const canonical = canonicalDiscoveryResourceType(resourceType);
  if (!canonical) return undefined;
  if (canonical === 'pod') return 'k8s';
  return canonical;
};

export const isAgentDiscoveryResourceType = (resourceType: unknown): boolean => {
  const normalized = normalizeDiscoveryResourceType(resourceType);
  return normalized === 'agent';
};

export const isAppContainerDiscoveryResourceType = (resourceType: unknown): boolean => {
  const normalized = normalizeDiscoveryResourceType(resourceType);
  return normalized === 'app-container';
};

export const getAgentDiscoveryResourceId = (
  discoveryTarget: DiscoveryTargetLike,
): string | undefined => {
  if (!discoveryTarget || !isAgentDiscoveryResourceType(discoveryTarget.resourceType)) {
    return undefined;
  }
  return asTrimmedString(discoveryTarget.resourceId) || asTrimmedString(discoveryTarget.agentId);
};
