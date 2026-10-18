import { parseWorkloadsLinkSearch } from '@/routing/resourceLinks';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId } from '@/utils/workloads';

/** Resolves the inbound `resource` deep link to the guest whose drawer opens. */
export const resolveWorkloadResourceSelection = (search: string): string | null =>
  parseWorkloadsLinkSearch(search).resource || null;

export const workloadsHasHoveredWorkload = (
  filteredGuests: WorkloadGuest[],
  hoveredId: string,
): boolean => filteredGuests.some((guest) => getCanonicalWorkloadId(guest) === hoveredId);
