import {
  parseWorkloadsLinkSearch,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import type { WorkloadGuest } from '@/types/workloads';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import { getCanonicalWorkloadId, normalizeWorkloadViewModeParam } from '@/utils/workloads';

export interface DashboardResourceSelection {
  resourceId: string;
  selectedNode: string | null;
}

export interface DashboardSelectionNavigateTargetOptions {
  pathname: string;
  search: string;
  resourceId: string | null;
}

export const resolveDashboardResourceSelection = (
  search: string,
): DashboardResourceSelection | null => {
  const { resource: resourceId, type } = parseWorkloadsLinkSearch(search);
  if (!resourceId) return null;

  const normalizedViewMode = normalizeWorkloadViewModeParam(type || '');
  const [firstSegment] = resourceId.split(':');
  const structuredResourceType = normalizeWorkloadViewModeParam(firstSegment || '');
  const legacyScopedMatch =
    normalizedViewMode !== 'app-container' &&
    normalizedViewMode !== 'pod' &&
    structuredResourceType !== 'app-container' &&
    structuredResourceType !== 'pod'
      ? resourceId.match(/^([^:]+):([^:]+):(\d+)$/)
      : null;
  const selectedNode = legacyScopedMatch ? `${legacyScopedMatch[1]}-${legacyScopedMatch[2]}` : null;

  return {
    resourceId,
    selectedNode,
  };
};

export const resolveDashboardSelectionNavigateTarget = ({
  pathname,
  search,
  resourceId,
}: DashboardSelectionNavigateTargetOptions): string | null => {
  const currentParams = new URLSearchParams(search);
  const nextParams = new URLSearchParams(search);
  nextParams.delete(WORKLOADS_QUERY_PARAMS.resource);

  const normalizedResourceId = resourceId?.trim() || '';
  if (normalizedResourceId) {
    nextParams.set(WORKLOADS_QUERY_PARAMS.resource, normalizedResourceId);
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
