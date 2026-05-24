import type { CephCluster, CephPool, Storage } from '@/types/api';

export const sanitizeCephPoolStorageComponent = (value: string): string => {
  const trimmed = value.trim();
  if (!trimmed) return '';

  return trimmed
    .replace(/[^A-Za-z0-9_.:-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-|-$/g, '');
};

export const cephPoolStorageId = (
  instanceName: string,
  pool: Pick<CephPool, 'id' | 'name'>,
): string => {
  const instance = sanitizeCephPoolStorageComponent(instanceName) || 'ceph';
  const poolName = sanitizeCephPoolStorageComponent(pool.name || '') || `pool-${pool.id}`;
  return `${instance}-ceph-pool-${poolName}`;
};

export const buildCephPoolThresholdStorage = (cephClusters: CephCluster[] = []): Storage[] => {
  const targets: Storage[] = [];

  cephClusters.forEach((cluster) => {
    const instance = cluster.instance?.trim() || cluster.id || 'ceph';
    (cluster.pools || []).forEach((pool) => {
      const name = pool.name?.trim() || `pool-${pool.id}`;
      const used = pool.storedBytes || 0;
      const free = pool.availableBytes || 0;
      const total = used + free;
      const usage = pool.percentUsed > 0 ? pool.percentUsed : total > 0 ? (used / total) * 100 : 0;

      targets.push({
        id: cephPoolStorageId(instance, pool),
        name,
        node: 'cluster',
        instance,
        type: 'ceph-pool',
        status: 'available',
        total,
        used,
        free,
        usage,
        content: 'ceph',
        shared: true,
        enabled: true,
        active: true,
        pool: name,
      });
    });
  });

  return targets;
};

export const buildThresholdStorageResources = (
  storage: Storage[] = [],
  cephClusters: CephCluster[] = [],
): Storage[] => {
  const resources = [...storage];
  const seen = new Set(resources.map((entry) => entry.id));

  buildCephPoolThresholdStorage(cephClusters).forEach((entry) => {
    if (seen.has(entry.id)) return;
    seen.add(entry.id);
    resources.push(entry);
  });

  return resources;
};
