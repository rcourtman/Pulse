import type { AIChatContext } from '@/stores/aiChat';
import type { WorkloadGuest } from '@/types/workloads';
import { buildResourceAssistantContextForTarget } from '@/utils/resourceAssistantContextModel';
import {
  getCanonicalWorkloadId,
  resolveDiscoveryTargetForWorkload,
  resolveWorkloadType,
} from '@/utils/workloads';

const firstTrimmed = (values: Array<string | null | undefined>): string | undefined => {
  for (const value of values) {
    const trimmed = (value || '').trim();
    if (trimmed) return trimmed;
  }
  return undefined;
};

export const getGuestAssistantTechnology = (guest: WorkloadGuest): string | undefined => {
  const workloadType = resolveWorkloadType(guest);
  if (workloadType === 'app-container') {
    return firstTrimmed([guest.containerRuntime, guest.platformType, guest.type]);
  }
  if (workloadType === 'pod') {
    return firstTrimmed([guest.platformType, 'kubernetes']);
  }
  return firstTrimmed([guest.platformType, guest.type]);
};

export const buildGuestAssistantContext = (guest: WorkloadGuest): AIChatContext => {
  const canonicalId = getCanonicalWorkloadId(guest);
  const workloadType = resolveWorkloadType(guest);
  const discoveryTarget = resolveDiscoveryTargetForWorkload(guest);

  return buildResourceAssistantContextForTarget({
    id: canonicalId,
    name: guest.name || canonicalId,
    type: workloadType,
    source: 'guest-drawer',
    status: guest.status,
    technology: getGuestAssistantTechnology(guest),
    parentName: guest.node,
    primaryIdentity: guest.id,
    discoveryTarget,
    discoveryReadiness: guest.discoveryReadiness,
  });
};
