// Browser-only resource-hook seam for #2321. The production page and shared
// tab rail stay real; source-scoped facets/resources are synthetic and switch
// on command. Global counts deliberately contain unrelated provider rows.
import { createSignal } from 'solid-js';
import type { Resource } from '../src/types/resource';
import type { UnifiedResourceFacets } from '../src/hooks/useUnifiedResources';

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

const [counts, setCounts] = createSignal<UnifiedResourceFacets | null>(null);
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
    aggregations: () => ({ total: 5, byType: { storage: 1, vm: 1, ceph: 1, pmg: 1 } }),
    facets: options?.cacheKey === 'proxmox-tab-evidence' ? counts : () => null,
    policyPosture: () => null,
    resourceSnapshotChange: () => ({ version: 0, changedIds: null }),
    loading: () => false,
    error: () => null,
    refetch: async () => resources(),
    mutate: () => resources(),
  };
}
