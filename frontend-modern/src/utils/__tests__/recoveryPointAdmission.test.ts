import { describe, expect, it } from 'vitest';
import { normalizeRecoveryPointsResponse } from '@/utils/recoveryPlatformModel';

const point = {
  id: 'snapshot-a',
  provider: 'truenas',
  kind: 'snapshot',
  mode: 'snapshot',
  outcome: 'success',
};

describe('recovery point HTTP admission', () => {
  it.each([undefined, null, false, [], {}, { data: null }, { data: {} }, { data: 'unreadable' }])(
    'rejects an unreadable envelope (%j), rather than returning empty evidence',
    (response) => {
      expect(() => normalizeRecoveryPointsResponse(response)).toThrow(
        'Recovery point response is unreadable.',
      );
    },
  );

  it.each([null, false, 1, 'row', [], {}])('rejects a malformed row (%j) atomically', (row) => {
    expect(() => normalizeRecoveryPointsResponse({ data: [point, row] })).toThrow(
      'Recovery point response is unreadable.',
    );
  });

  it.each(['id', 'kind', 'mode', 'outcome'])('requires a usable %s on every row', (key) => {
    for (const invalid of [undefined, null, '', '  ', 3, false]) {
      expect(() =>
        normalizeRecoveryPointsResponse({ data: [{ ...point, [key]: invalid }] }),
      ).toThrow('Recovery point response is unreadable.');
    }
  });

  it.each(['display', 'details', 'itemRef', 'subjectRef', 'repositoryRef'])(
    'rejects an unreadable %s container without accepting part of the page',
    (key) => {
      for (const invalid of [false, 1, 'unreadable', []]) {
        expect(() =>
          normalizeRecoveryPointsResponse({ data: [point, { ...point, id: 'b', [key]: invalid }] }),
        ).toThrow('Recovery point response is unreadable.');
      }
    },
  );

  it('rejects conflicting duplicate identities', () => {
    expect(() =>
      normalizeRecoveryPointsResponse({ data: [point, { ...point, outcome: 'failed' }] }),
    ).toThrow('Recovery point response is unreadable.');
  });

  it('accepts explicit empty arrays without reviving retired pagination metadata', () => {
    expect(normalizeRecoveryPointsResponse({ data: [], meta: { total: 0 } })).toStrictEqual({
      data: [],
    });
  });

  it('preserves optional nulls, alias decoding and future open-string outcomes', () => {
    expect(
      normalizeRecoveryPointsResponse({
        data: [{ ...point, outcome: 'future-state', display: null, details: null }],
      }),
    ).toStrictEqual({
      data: [
        {
          id: 'snapshot-a',
          platform: 'truenas',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'future-state',
          display: null,
          details: null,
        },
      ],
    });
  });
});
