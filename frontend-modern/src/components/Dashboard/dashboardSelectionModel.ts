import {
  parseWorkloadsLinkSearch,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import type { WorkloadGuest } from '@/types/workloads';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import { getCanonicalWorkloadId } from '@/utils/workloads';

export interface DashboardResourceSelection {
  resourceId: string | null;
  summaryGroupId: string | null;
}

export interface DashboardSelectionNavigateTargetOptions {
  pathname: string;
  search: string;
  resourceId: string | null;
  summaryGroupId: string | null;
}

export const resolveDashboardResourceSelection = (
  search: string,
): DashboardResourceSelection | null => {
  const { resource: resourceId, summaryGroup: summaryGroupId } = parseWorkloadsLinkSearch(search);
  if (!resourceId && !summaryGroupId) return null;

  return {
    resourceId: resourceId || null,
    summaryGroupId: summaryGroupId || null,
  };
};

export const resolveDashboardSelectionNavigateTarget = ({
  pathname,
  search,
  resourceId,
  summaryGroupId,
}: DashboardSelectionNavigateTargetOptions): string | null => {
  const currentParams = new URLSearchParams(search);
  const nextParams = new URLSearchParams(search);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.resource);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.summaryGroup);

  const normalizedResourceId = resourceId?.trim() || '';
  const normalizedSummaryGroupId = summaryGroupId?.trim() || '';
  if (normalizedResourceId) {
    nextParams.set(WORKLOADS_QUERY_PARAMS.resource, normalizedResourceId);
  }
  if (normalizedSummaryGroupId) {
    nextParams.set(WORKLOADS_QUERY_PARAMS.summaryGroup, normalizedSummaryGroupId);
  }

  if (areSearchParamsEquivalent(currentParams, nextParams)) {
    return null;
  }

  const nextSearch = nextParams.toString();
  const nextPathname = pathname.trim() || WORKLOADS_PATH;
  return nextSearch ? `${nextPathname}?${nextSearch}` : nextPathname;
};

export const dashboardHasHoveredWorkload = (
  filteredGuests: WorkloadGuest[],
  hoveredId: string,
): boolean => filteredGuests.some((guest) => getCanonicalWorkloadId(guest) === hoveredId);

export const dashboardHasVisibleWorkloadGroupScope = (
  filteredGuests: WorkloadGuest[],
  groupScope: SummarySeriesGroupScope,
): boolean => {
  const filteredGuestIds = new Set(
    filteredGuests.map((guest) => getCanonicalWorkloadId(guest)).filter(Boolean),
  );
  return groupScope.seriesIds.some((seriesId) => filteredGuestIds.has(seriesId));
};
