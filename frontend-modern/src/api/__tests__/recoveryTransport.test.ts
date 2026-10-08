import { afterEach, describe, expect, expectTypeOf, it, vi } from 'vitest';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { createRoot, type Accessor } from 'solid-js';

import {
  filterTrueNASProtectionPoints,
  sortTrueNASProtectionPoints,
} from '@/features/truenas/truenasPageModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useRecoveryPoints, type RecoveryPointsQuery } from '@/hooks/useRecoveryPoints';
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

  it('asks /api/recovery/points only for the page, limit and platform it was given', async () => {
    // The points endpoint accepts more filters, but the only reader sends these
    // three. A caller that casts extra filters in must not get them on the wire.
    apiFetchJSONMock.mockResolvedValue({
      data: [],
      meta: { page: 2, limit: 200, total: 0, totalPages: 1 },
    });
    const smuggled = {
      platform: ' truenas ',
      page: 2.7,
      limit: Number.NaN,
      rollupId: 'rollup-1',
      kind: 'snapshot',
      mode: 'snapshot',
      outcome: 'failed',
      itemType: 'dataset',
      itemResourceId: 'res-1',
      subjectResourceId: 'res-2',
      q: 'tank',
      cluster: 'cluster-1',
      node: 'node-1',
      namespace: 'namespace-1',
      scope: 'workload',
      verification: 'verified',
      from: '2026-10-01T00:00:00Z',
      to: '2026-10-02T00:00:00Z',
    } as RecoveryPointsQuery;
    let dispose = () => {};
    createRoot((rootDispose) => {
      dispose = rootDispose;
      useRecoveryPoints(() => smuggled);
    });
    try {
      await vi.waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalledTimes(1));
      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/recovery/points?page=2&limit=200&platform=truenas',
      );
    } finally {
      dispose();
    }
  });

  it('leaves the platform off a blank request and makes no request without a query', async () => {
    apiFetchJSONMock.mockResolvedValue({
      data: [],
      meta: { page: 1, limit: 200, total: 0, totalPages: 1 },
    });
    let dispose = () => {};
    createRoot((rootDispose) => {
      dispose = rootDispose;
      useRecoveryPoints(() => null);
      useRecoveryPoints(() => ({ platform: '   ' }));
    });
    try {
      await vi.waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalledTimes(1));
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/recovery/points?page=1&limit=200');
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

  it('keeps the recovery points query and hook result to what the Protection tab reads', () => {
    // Enforced by the frontend type check, not at runtime. The tab sends
    // platform, page and limit and reads the points, the loading and error
    // state and a refetch; the aggregate page's other filters and its meta and
    // resolvedOnce members went with it.
    expectTypeOf<keyof RecoveryPointsQuery>().toEqualTypeOf<'page' | 'limit' | 'platform'>();
    type Result = ReturnType<typeof useRecoveryPoints>;
    expectTypeOf<keyof Result>().toEqualTypeOf<'response' | 'points' | 'refetch'>();
    expectTypeOf<Result>().not.toHaveProperty('meta');
    expectTypeOf<Result>().not.toHaveProperty('resolvedOnce');
    expectTypeOf<Parameters<typeof useRecoveryPoints>[0]>().toEqualTypeOf<
      Accessor<RecoveryPointsQuery | null | undefined>
    >();
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
