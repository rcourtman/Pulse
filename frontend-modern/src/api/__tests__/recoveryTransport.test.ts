import { describe, expect, it } from 'vitest';

import {
  normalizeRecoveryPointsResponse,
  normalizeRecoveryRollupsResponse,
} from '@/utils/recoveryPlatformModel';

describe('recovery transport', () => {
  it('normalizes legacy subject recovery fields onto canonical item fields', () => {
    expect(
      normalizeRecoveryPointsResponse({
        data: [
          {
            id: 'point-1',
            provider: 'truenas',
            kind: 'snapshot',
            mode: 'snapshot',
            outcome: 'success',
            subjectResourceId: 'res-1',
            subjectRef: { type: 'truenas-dataset', name: 'tank/apps' },
          },
        ],
        meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
      }),
    ).toEqual({
      data: [
        {
          id: 'point-1',
          platform: 'truenas',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'success',
          itemResourceId: 'res-1',
          itemRef: { type: 'truenas-dataset', name: 'tank/apps' },
        },
      ],
      meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
    });

    expect(
      normalizeRecoveryRollupsResponse({
        data: [
          {
            rollupId: 'rollup-1',
            lastOutcome: 'success',
            providers: ['truenas'],
            subjectResourceId: 'res-1',
            subjectRef: { type: 'truenas-dataset', name: 'tank/apps' },
          },
        ],
        meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
      }),
    ).toEqual({
      data: [
        {
          rollupId: 'rollup-1',
          lastOutcome: 'success',
          platforms: ['truenas'],
          itemResourceId: 'res-1',
          itemRef: { type: 'truenas-dataset', name: 'tank/apps' },
        },
      ],
      meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
    });
  });
});
