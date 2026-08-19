import type { Resource } from '@/types/resource';

// Keep the original key so operators who hid the earlier panel retain the same
// preference when the information moves into the existing filter controls.
export const PLATFORM_ESTATE_COUNTS_STORAGE_KEY = 'platformEstateOverviewVisible';

export const deserializePlatformEstateCountsVisibility = (raw: string): boolean => raw !== 'false';

export type ProxmoxEstateTopology = {
  clusters: number;
  nodes: number;
  standalone: number;
};

export function buildProxmoxEstateTopology(resources: readonly Resource[]): ProxmoxEstateTopology {
  const nodes = resources.filter((resource) => resource.type === 'agent');
  const clusters = new Set(
    nodes
      .map((resource) => resource.proxmox?.clusterName?.trim() || resource.clusterId?.trim() || '')
      .filter(Boolean),
  );
  const standalone = nodes.filter(
    (resource) => !resource.proxmox?.clusterName?.trim() && !resource.clusterId?.trim(),
  ).length;

  return { clusters: clusters.size, nodes: nodes.length, standalone };
}
