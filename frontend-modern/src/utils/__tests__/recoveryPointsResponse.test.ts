import { describe, expect, it } from 'vitest';

import { normalizeRecoveryPointsResponse } from '@/utils/recoveryPlatformModel';

const point = {
  id: 'snapshot-1',
  provider: 'truenas',
  kind: 'snapshot',
  mode: 'local',
  outcome: 'success',
  display: { subjectLabel: 'tank/apps', subjectType: 'dataset' },
};
const formatError = 'The recovery point response format is invalid.';

describe('recovery point response admission', () => {
  it.each([
    undefined,
    null,
    [],
    'private-proxy-response',
    1,
    {},
    { data: null },
    { data: {} },
    { data: 'private-proxy-response' },
  ])('rejects an unread inventory (%j), not a successful empty page', (payload) => {
    expect(() => normalizeRecoveryPointsResponse(payload)).toThrow(formatError);
  });

  it('accepts only an explicit empty array as an empty page and ignores unused pagination', () => {
    expect(normalizeRecoveryPointsResponse({ data: [], meta: { total: 0 } })).toEqual({ data: [] });
  });

  it.each([
    null,
    [],
    'private-provider-row',
    {},
    { ...point, id: '' },
    { ...point, id: '  ' },
    { ...point, id: 10 },
    { ...point, kind: null },
    { ...point, mode: {} },
    { ...point, outcome: true },
    { ...point, provider: 42 },
    { ...point, platform: [] },
    { ...point, completedAt: {} },
    { ...point, startedAt: 42 },
    { ...point, sizeBytes: '123' },
    { ...point, sizeBytes: -1 },
    { ...point, sizeBytes: Infinity },
    { ...point, verified: 'false' },
    { ...point, encrypted: 1 },
    { ...point, immutable: [] },
    { ...point, display: 'private-provider-label' },
    { ...point, display: { detailsSummary: {} } },
    { ...point, display: { isWorkload: 'false' } },
    { ...point, itemRef: 'private-provider-identity' },
    { ...point, subjectRef: { name: {} } },
    { ...point, repositoryRef: [] },
    { ...point, details: [] },
  ])('rejects the entire page when any consumed row is malformed (%j)', (invalid) => {
    expect(() =>
      normalizeRecoveryPointsResponse({ data: [point, invalid], private: 'do-not-render' }),
    ).toThrow(formatError);
  });

  it('rejects duplicate identities rather than letting one event replace another', () => {
    expect(() =>
      normalizeRecoveryPointsResponse({ data: [point, { ...point, outcome: 'failed' }] }),
    ).toThrow(formatError);
  });

  it('preserves compatibility, unknown strings, optional nulls and unparseable date evidence', () => {
    const payload = {
      data: [
        {
          ...point,
          kind: 'future-kind',
          mode: 'future-mode',
          outcome: 'future-outcome',
          completedAt: '2026-02-30T10:00:00Z',
          startedAt: 'not-an-instant',
          sizeBytes: 0,
          verified: false,
          immutable: null,
          subjectRef: { type: 'truenas-dataset', name: 'tank/apps' },
          repositoryRef: null,
          details: { futureField: ['unknown', 1] },
          futureField: 'preserved',
        },
      ],
    };
    const before = structuredClone(payload);
    const [accepted] = normalizeRecoveryPointsResponse(payload).data;
    expect(accepted).toMatchObject({
      platform: 'truenas',
      kind: 'future-kind',
      mode: 'future-mode',
      outcome: 'future-outcome',
      completedAt: '2026-02-30T10:00:00Z',
      startedAt: 'not-an-instant',
      sizeBytes: 0,
      verified: false,
      immutable: null,
      itemRef: { type: 'truenas-dataset', name: 'tank/apps' },
      display: { itemLabel: 'tank/apps', itemType: 'dataset' },
      details: { futureField: ['unknown', 1] },
      futureField: 'preserved',
    });
    expect(payload).toEqual(before);
  });
});
