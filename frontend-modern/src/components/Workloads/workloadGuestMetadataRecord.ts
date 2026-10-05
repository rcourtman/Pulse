import type { GuestMetadata } from '@/api/guestMetadata';
import type { WorkloadGuest } from '@/types/workloads';
import { getWorkloadMetadataIdCandidates } from '@/utils/workloads';

export type WorkloadGuestMetadataMap = Record<string, GuestMetadata>;

/**
 * Saved metadata for a workload row. Stable identities win over the legacy
 * instance:node:vmid key that v5 data volumes still carry.
 */
export const getWorkloadGuestMetadataRecord = (
  guest: WorkloadGuest,
  byId: WorkloadGuestMetadataMap,
): GuestMetadata | undefined => {
  for (const metadataId of getWorkloadMetadataIdCandidates(guest)) {
    if (metadataId && byId[metadataId]) {
      return byId[metadataId];
    }
  }
  return byId[`${guest.instance}:${guest.node}:${guest.vmid}`];
};
