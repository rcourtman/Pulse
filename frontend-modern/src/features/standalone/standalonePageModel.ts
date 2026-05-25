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

const availabilityTargetKindFor = (resource: Resource): string =>
  String(
    resource.availability?.targetKind ??
      (resource.platformData?.availability as { targetKind?: string } | undefined)?.targetKind ??
      '',
  )
    .trim()
    .toLowerCase();

export const isAgentlessMachineResource = (resource: Resource): boolean =>
  isAgentlessAvailabilityResource(resource) && availabilityTargetKindFor(resource) === 'machine';

export function buildStandalonePageModel(resources: readonly Resource[]): StandalonePageModel {
  const machines = resources.filter(
    (resource) => isStandaloneMachineResource(resource) || isAgentlessMachineResource(resource),
  );
  const availabilityChecks = resources.filter(isAgentlessAvailabilityResource);
  return {
    machines,
    availabilityChecks,
    resources: machines,
  };
}
