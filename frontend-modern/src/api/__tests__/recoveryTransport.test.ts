import { afterEach, describe, expect, expectTypeOf, it, vi } from 'vitest';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { createRoot } from 'solid-js';

import {
  filterTrueNASProtectionPoints,
  sortTrueNASProtectionPoints,
} from '@/features/truenas/truenasPageModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import type {
  RecoveryPoint,
  RecoveryPointDisplay,
  RecoveryPointDisplayTransport,
  RecoveryPointTransport,
} from '@/types/recovery';
import { normalizeRecoveryPointsResponse } from '@/utils/recoveryPlatformModel';

const apiFetchJSONMock = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: apiFetchJSONMock }));

afterEach(() => {
  apiFetchJSONMock.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
});

const productionSources = (dir: string): string[] =>
  readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      return entry.name === '__tests__' || entry.name === '__fixtures__'
        ? []
        : productionSources(path);
    }
    return /\.(ts|tsx)$/.test(entry.name) && !/\.(test|spec)\.tsx?$/.test(entry.name) ? [path] : [];
  });

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
  });

  it('normalizes legacy display aliases onto the canonical item labels', () => {
    const [point] = normalizeRecoveryPointsResponse({
      data: [
        {
          id: 'point-2',
          platform: 'truenas',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'success',
          display: { subjectLabel: ' tank/apps ', subjectType: 'dataset' },
        },
      ],
      meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
    }).data;
    expect(point.display).toEqual({ itemLabel: 'tank/apps', itemType: 'dataset' });
  });

  it('hands the TrueNAS Protection tab normalized recovery points', async () => {
    apiFetchJSONMock.mockResolvedValue({
      data: [
        {
          id: 'point-3',
          provider: 'truenas',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'success',
          subjectRef: { type: 'truenas-dataset', name: 'tank/apps' },
          display: { subjectLabel: 'tank/apps', subjectType: 'dataset' },
        },
      ],
      meta: { page: 1, limit: 200, total: 1, totalPages: 1 },
    });
    let dispose = () => {};
    const points = createRoot((rootDispose) => {
      dispose = rootDispose;
      return useRecoveryPoints(() => ({ platform: 'truenas', page: 1, limit: 200 })).points;
    });
    try {
      await vi.waitFor(() => expect(points()).toHaveLength(1));
      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/recovery/points?page=1&limit=200&platform=truenas',
      );
      expect(points()[0]).toEqual({
        id: 'point-3',
        platform: 'truenas',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
        itemRef: { type: 'truenas-dataset', name: 'tank/apps' },
        display: { itemLabel: 'tank/apps', itemType: 'dataset' },
      });
    } finally {
      dispose();
    }
  });

  it('labels, sorts and finds TrueNAS rows by the backend subject label only through the decode', async () => {
    // The points handler sends the dataset label only as display.subjectLabel.
    // The TrueNAS Protection tab reads display.itemLabel, never the subject
    // names, so the fold in recoveryPlatformModel is what names these rows.
    // Ids sort opposite to labels, so an unlabelled row would sort by id.
    const backendPoint = (id: string, label: string) => ({
      id,
      platform: 'truenas',
      provider: 'truenas',
      kind: 'snapshot',
      mode: 'snapshot',
      outcome: 'success',
      completedAt: '2026-10-01T00:00:00Z',
      display: { subjectLabel: label, subjectType: 'dataset', itemType: 'dataset' },
    });
    apiFetchJSONMock.mockResolvedValue({
      data: [backendPoint('point-a', 'tank/zeta'), backendPoint('point-b', 'tank/alpha')],
      meta: { page: 1, limit: 200, total: 2, totalPages: 1 },
    });
    let dispose = () => {};
    const points = createRoot((rootDispose) => {
      dispose = rootDispose;
      return useRecoveryPoints(() => ({ platform: 'truenas', page: 1, limit: 200 })).points;
    });
    try {
      await vi.waitFor(() => expect(points()).toHaveLength(2));
      expect(sortTrueNASProtectionPoints(points()).map((point) => point.id)).toEqual([
        'point-b',
        'point-a',
      ]);
      expect(
        filterTrueNASProtectionPoints(points(), 'zeta', 'all').map((point) => point.id),
      ).toEqual(['point-a']);
    } finally {
      dispose();
    }
  });

  it('keeps the backend subject names on the transport types only', () => {
    // Enforced by the frontend type check, not at runtime: a consumer cannot
    // read subjectRef or display.subjectLabel off a normalized point.
    expectTypeOf<RecoveryPoint>().not.toHaveProperty('subjectRef');
    expectTypeOf<RecoveryPointDisplay>().not.toHaveProperty('subjectLabel');
    expectTypeOf<RecoveryPointDisplay>().not.toHaveProperty('subjectType');
    expectTypeOf<RecoveryPointTransport>().toHaveProperty('subjectRef');
    expectTypeOf<RecoveryPointDisplayTransport>().toHaveProperty('subjectLabel');
    expectTypeOf<RecoveryPointDisplayTransport>().toHaveProperty('subjectType');
  });

  it('keeps useRecoveryPoints the only recovery-points reader', () => {
    // The rollup, facet and series hooks went with the aggregate Recovery
    // page, and the item-type helper with the recovery query serializer that
    // was its last reader. The scan matches literal /api/recovery/ paths only.
    const recoveryReaders = productionSources('src')
      .filter((path) => /['"`]\/api\/recovery\//.test(readFileSync(path, 'utf8')))
      .sort();
    expect(recoveryReaders).toEqual([
      join('src', 'hooks', 'useProtectionPostures.ts'),
      join('src', 'hooks', 'useRecoveryPoints.ts'),
    ]);
    for (const retired of [
      'src/hooks/useRecoveryRollups.ts',
      'src/hooks/useRecoveryPointsFacets.ts',
      'src/hooks/useRecoveryPointsSeries.ts',
      'src/utils/recoveryItemTypePresentation.ts',
      'src/utils/recoveryArtifactModePresentation.ts',
    ]) {
      expect(existsSync(retired), retired).toBe(false);
    }
  });
});
