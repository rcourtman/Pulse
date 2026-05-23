import type { Resource } from '@/types/resource';
import { isPulseAgentPlatformResource } from '@/utils/agentResources';

export interface AgentsPageModel {
  resources: Resource[];
}

export const isAgentsPageResource = (resource: Resource): boolean =>
  isPulseAgentPlatformResource(resource);

export function buildAgentsPageModel(resources: readonly Resource[]): AgentsPageModel {
  return {
    resources: resources.filter(isAgentsPageResource),
  };
}
