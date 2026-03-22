import { parseWorkloadsLinkSearch } from '@/routing/resourceLinks';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId } from '@/utils/workloads';

export interface DashboardResourceSelection {
  resourceId: string;
  selectedNode: string | null;
}

export const resolveDashboardResourceSelection = (
  search: string,
): DashboardResourceSelection | null => {
  const { resource: resourceId } = parseWorkloadsLinkSearch(search);
  if (!resourceId) return null;

  const [instance, node, vmid] = resourceId.split(':');
  const selectedNode = instance && node && vmid ? `${instance}-${node}` : null;

  return {
    resourceId,
    selectedNode,
  };
};

export const dashboardHasHoveredWorkload = (
  filteredGuests: WorkloadGuest[],
  hoveredId: string,
): boolean => filteredGuests.some((guest) => getCanonicalWorkloadId(guest) === hoveredId);
