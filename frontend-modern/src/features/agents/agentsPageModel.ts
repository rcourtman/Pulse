import type { Resource } from '@/types/resource';
import { isPulseAgentPlatformResource } from '@/utils/agentResources';

export interface AgentsPageModel {
  machines: Resource[];
  availabilityChecks: Resource[];
  resources: Resource[];
}

export const isAgentsPageResource = (resource: Resource): boolean =>
  isPulseAgentPlatformResource(resource);

export const isAgentlessAvailabilityResource = (resource: Resource): boolean =>
  resource.type === 'network-endpoint' ||
  resource.platformType === 'availability' ||
  resource.sources?.includes('availability') === true;

export function buildAgentsPageModel(resources: readonly Resource[]): AgentsPageModel {
  const machines = resources.filter(isAgentsPageResource);
  const availabilityChecks = resources.filter(isAgentlessAvailabilityResource);
  return {
    machines,
    availabilityChecks,
    resources: machines,
  };
}
