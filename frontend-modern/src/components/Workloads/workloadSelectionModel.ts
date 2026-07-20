import { parseWorkloadsLinkSearch } from '@/routing/resourceLinks';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId } from '@/utils/workloads';

export interface WorkloadsResourceSelection {
  resourceId: string | null;
  summaryGroupId: string | null;
}

export const resolveWorkloadResourceSelection = (
  search: string,
): WorkloadsResourceSelection | null => {
  const { resource: resourceId, summaryGroup: summaryGroupId } = parseWorkloadsLinkSearch(search);
  if (!resourceId && !summaryGroupId) return null;

  return {
    resourceId: resourceId || null,
    summaryGroupId: summaryGroupId || null,
  };
};

export const workloadsHasHoveredWorkload = (
  filteredGuests: WorkloadGuest[],
  hoveredId: string,
): boolean => filteredGuests.some((guest) => getCanonicalWorkloadId(guest) === hoveredId);

export const workloadsHasVisibleWorkloadGroupScope = (
  filteredGuests: WorkloadGuest[],
  groupScope: SummarySeriesGroupScope,
): boolean => {
  const filteredGuestIds = new Set(
    filteredGuests.map((guest) => getCanonicalWorkloadId(guest)).filter(Boolean),
  );
  return groupScope.seriesIds.some((seriesId) => filteredGuestIds.has(seriesId));
};
