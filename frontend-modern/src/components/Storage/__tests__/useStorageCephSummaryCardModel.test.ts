import { describe, expect, it } from 'vitest';
import { createRoot } from 'solid-js';
import { useStorageCephSummaryCardModel } from '@/components/Storage/useStorageCephSummaryCardModel';

describe('useStorageCephSummaryCardModel', () => {
  it('returns canonical ceph header and card state', () => {
    createRoot((dispose) => {
      const model = useStorageCephSummaryCardModel({
        summary: () => ({
          clusters: [
            {
              id: 'cluster-1',
              instance: 'cluster-main',
              name: 'cluster-main Ceph',
              health: 'HEALTH_OK',
              healthMessage: 'Healthy',
              totalBytes: 300,
              usedBytes: 120,
              availableBytes: 180,
              usagePercent: 40,
              numMons: 3,
              numMgrs: 2,
              numOsds: 6,
              numOsdsUp: 6,
              numOsdsIn: 6,
              numPGs: 128,
              pools: undefined,
              services: undefined,
              lastUpdated: Date.now(),
            },
          ],
          totalBytes: 300,
          usedBytes: 120,
          availableBytes: 180,
          usagePercent: 40,
        }),
      });

      expect(model.header()).toMatchObject({
        heading: 'Ceph Summary',
        clusterCountLabel: '1 cluster detected',
        totalLabel: '300 B',
      });
      expect(model.clusterCards()).toHaveLength(1);
      expect(model.clusterCards()[0]).toMatchObject({
        id: 'cluster-1',
        title: 'cluster-main Ceph',
        healthLabel: 'OK',
      });
      dispose();
    });
  });
});
