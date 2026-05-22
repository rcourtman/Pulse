import { describe, expect, it } from 'vitest';
import {
  buildVMwareDetailSections,
  buildVMwareDetailsSummary,
} from '@/components/Infrastructure/resourceDetailDrawerVmwareModel';
import type { ResourceVMwareMeta } from '@/types/resource';

describe('resourceDetailDrawerVmwareModel', () => {
  it('surfaces vSphere snapshot trees as read-only VM detail context', () => {
    const vmware: ResourceVMwareMeta = {
      connectionName: 'Lab VC',
      snapshotTree: [
        {
          snapshot: 'snapshot-201',
          name: 'pre-upgrade',
          createdAt: '2026-03-28T18:15:00Z',
          state: 'poweredOn',
          quiesced: true,
          children: [
            {
              snapshot: 'snapshot-202',
              name: 'post-migration-checkpoint',
              createdAt: '2026-03-29T18:15:00Z',
              state: 'poweredOn',
              current: true,
              quiesced: false,
            },
          ],
        },
      ],
    };

    expect(buildVMwareDetailsSummary('vm', vmware)).toBe(
      'Lab VC · Read-only vCenter context · 2 snapshots',
    );

    const snapshots = buildVMwareDetailSections('vm', vmware).find(
      (section) => section.id === 'snapshots',
    );

    expect(snapshots?.rows).toEqual([
      {
        label: 'pre-upgrade',
        value: 'poweredOn · 2026-03-28 18:15 UTC · quiesced',
        tone: 'default',
      },
      {
        label: '- post-migration-checkpoint',
        value: 'current · poweredOn · 2026-03-29 18:15 UTC · not quiesced',
        tone: 'accent',
      },
    ]);
  });
});
