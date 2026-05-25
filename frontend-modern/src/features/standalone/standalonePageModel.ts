import type { Resource } from '@/types/resource';
import { isPulseAgentPlatformResource } from '@/utils/agentResources';

export interface StandalonePageModel {
  machines: Resource[];
  availabilityChecks: Resource[];
  resources: Resource[];
}

export const isStandaloneMachineResource = (resource: Resource): boolean =>
  isPulseAgentPlatformResource(resource);

export const isAgentlessAvailabilityResource = (resource: Resource): boolean =>
  resource.type === 'network-endpoint' ||
  resource.platformType === 'availability' ||
  resource.sources?.includes('availability') === true;

export function buildStandalonePageModel(resources: readonly Resource[]): StandalonePageModel {
  const machines = resources.filter(isStandaloneMachineResource);
  const availabilityChecks = resources.filter(isAgentlessAvailabilityResource);
  return {
    machines,
    availabilityChecks,
    resources: machines,
  };
}
