// Browser-only resource-hook seam for #2321. The production page and shared
// tab rail stay real; counts/resources are synthetic and switch on command.
import { createSignal } from 'solid-js';
import type { Resource } from '../src/types/resource';
import type { UnifiedResourceAggregations } from '../src/hooks/useUnifiedResources';

export * from '../src/hooks/useUnifiedResources';

const pbs = {
  id: 'pbs-only', type: 'pbs', name: 'pbs-only', displayName: 'pbs-only',
  platformId: 'pbs-only', platformType: 'proxmox-pbs', sourceType: 'api',
  sources: ['pbs'], status: 'online', lastSeen: Date.now(),
  pbs: { instanceId: 'pbs-only', hostname: 'pbs-only', datastores: [] },
} as Resource;
const pmg = {
  id: 'pmg-one', type: 'pmg', name: 'pmg-one', displayName: 'pmg-one',
  platformId: 'pmg-one', platformType: 'proxmox-pmg', sourceType: 'api',
  sources: ['pmg'], status: 'online', lastSeen: Date.now(),
} as Resource;

const [counts, setCounts] = createSignal<UnifiedResourceAggregations | null>(null);
(window as unknown as { __proxmoxTabProof: { setCounts: typeof setCounts } }).__proxmoxTabProof = {
  setCounts,
};

export function useUnifiedResources(options?: { cacheKey?: string }) {
  const resources = () => {
    switch (options?.cacheKey) {
      case 'proxmox-backups-shell':
        return [pbs];
      case 'proxmox-mail':
        return [pmg];
      default:
        return [] as Resource[];
    }
  };
  return {
    resources,
    aggregations: counts,
    facets: () => null,
    policyPosture: () => null,
    resourceSnapshotChange: () => ({ version: 0, changedIds: null }),
    loading: () => false,
    error: () => null,
    refetch: async () => resources(),
    mutate: () => resources(),
  };
}
