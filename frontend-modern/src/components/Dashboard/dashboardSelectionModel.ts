import { parseWorkloadsLinkSearch } from '@/routing/resourceLinks';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId, normalizeWorkloadViewModeParam } from '@/utils/workloads';

export interface DashboardResourceSelection {
  resourceId: string;
  selectedNode: string | null;
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

export const dashboardHasHoveredWorkload = (
  filteredGuests: WorkloadGuest[],
  hoveredId: string,
): boolean => filteredGuests.some((guest) => getCanonicalWorkloadId(guest) === hoveredId);
