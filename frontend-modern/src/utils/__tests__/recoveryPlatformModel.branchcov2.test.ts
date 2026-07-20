import { describe, expect, it } from 'vitest';

import {
  getRecoveryPointPlatform,
  getRecoveryRollupPlatforms,
  normalizeRecoveryPoint,
  normalizeRecoveryPointsResponse,
  normalizeRecoveryRollup,
  normalizeRecoveryRollupsResponse,
} from '@/utils/recoveryPlatformModel';

/**
 * Branch-coverage companion to recoveryPlatformModel.test.ts.
 *
 * The non-exported helpers (toTrimmedString, normalizeRecoveryDisplay,
 * getRecoveryItemResourceId, getRecoveryItemRef, normalizeRecoveryMeta) are
 * exercised indirectly through the exported normalizers, since they are
 * module-private. Each test below targets specific arms of the conditionals
 * documented in recoveryPlatformModel.ts.
 */
describe('recoveryPlatformModel.branchcov2', () => {
  describe('toTrimmedString (via exported wrappers)', () => {
    it('trims surrounding whitespace from string inputs', () => {
      // Branch: typeof value === 'string' -> value.trim()
      expect(getRecoveryPointPlatform({ platform: '  truenas  ' })).toBe('truenas');
      expect(getRecoveryPointPlatform({ provider: '\tproxmox-pbs\n' })).toBe('proxmox-pbs');
    });

    it('returns empty string for non-string values', () => {
      // Branch: typeof value !== 'string' -> '' (hit via a numeric subject id).
      const malformed = {
        id: 'p',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
        subjectResourceId: 12345,
      } as unknown as Parameters<typeof normalizeRecoveryPoint>[0];
      const result = normalizeRecoveryPoint(malformed);
      expect(result).toStrictEqual({
        id: 'p',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
      });
      expect('itemResourceId' in result).toBe(false);
    });
  });

  describe('getRecoveryPointPlatform', () => {
    it('returns empty string when both platform and provider are absent', () => {
      expect(getRecoveryPointPlatform(null)).toBe('');
      expect(getRecoveryPointPlatform(undefined)).toBe('');
      expect(getRecoveryPointPlatform({})).toBe('');
    });
  });

  describe('getRecoveryRollupPlatforms', () => {
    it('falls back to providers when platforms is an empty array', () => {
      expect(getRecoveryRollupPlatforms({ platforms: [], providers: ['x'] })).toEqual(['x']);
    });

    it('falls back to providers when platforms is not an array', () => {
      const malformed = {
        platforms: 'nope',
        providers: ['kubernetes'],
      } as unknown as Parameters<typeof getRecoveryRollupPlatforms>[0];
      expect(getRecoveryRollupPlatforms(malformed)).toEqual(['kubernetes']);
    });

    it('returns an empty array when both platforms and providers are absent', () => {
      expect(getRecoveryRollupPlatforms({})).toEqual([]);
      expect(getRecoveryRollupPlatforms(null)).toEqual([]);
      expect(getRecoveryRollupPlatforms(undefined)).toEqual([]);
    });

    it('trims and filters blank entries from the resolved values', () => {
      expect(getRecoveryRollupPlatforms({ platforms: ['  a  ', '', '   ', 'b'] })).toEqual([
        'a',
        'b',
      ]);
      // All entries normalize to empty -> filter(Boolean) yields [].
      expect(getRecoveryRollupPlatforms({ platforms: ['   ', ''] })).toEqual([]);
    });
  });

  describe('normalizeRecoveryDisplay (via normalizeRecoveryPoint)', () => {
    const basePoint = {
      id: 'p1',
      kind: 'snapshot' as const,
      mode: 'snapshot' as const,
      outcome: 'success' as const,
    };

    it('passes a null display through unchanged', () => {
      // Branch: display == null -> return display (null).
      expect(normalizeRecoveryPoint({ ...basePoint, display: null })).toStrictEqual({
        ...basePoint,
        display: null,
      });
    });

    it('omits the display key entirely when display is undefined', () => {
      // Branch: display !== undefined === false -> display key not spread.
      const result = normalizeRecoveryPoint(basePoint);
      expect(result).toStrictEqual(basePoint);
      expect('display' in result).toBe(false);
    });

    it('uses canonical itemLabel/itemType when present without falling back', () => {
      expect(
        normalizeRecoveryPoint({
          ...basePoint,
          display: { itemLabel: 'IL', itemType: 'IT', isWorkload: true },
        }),
      ).toStrictEqual({
        ...basePoint,
        display: { itemLabel: 'IL', itemType: 'IT', isWorkload: true },
      });
    });

    it('falls back to subjectLabel/subjectType when itemLabel/itemType are absent', () => {
      expect(
        normalizeRecoveryPoint({
          ...basePoint,
          display: { subjectLabel: 'SL', subjectType: 'ST', detailsSummary: 'ds' },
        }),
      ).toStrictEqual({
        ...basePoint,
        display: { itemLabel: 'SL', itemType: 'ST', detailsSummary: 'ds' },
      });
    });

    it('falls back to subjectLabel when itemLabel is blank but subjectLabel is set', () => {
      expect(
        normalizeRecoveryPoint({
          ...basePoint,
          display: { itemLabel: '   ', subjectLabel: 'SL2' },
        }),
      ).toStrictEqual({
        ...basePoint,
        display: { itemLabel: 'SL2' },
      });
    });

    it('omits itemLabel and itemType keys when both canonical and subject fields are blank', () => {
      // Exercises the falsy arms of the `...(x ? {itemLabel} : {})` spreads.
      expect(
        normalizeRecoveryPoint({
          ...basePoint,
          display: { itemLabel: '  ', subjectLabel: '', itemType: '', subjectType: '  ' },
        }),
      ).toStrictEqual({
        ...basePoint,
        display: {},
      });
    });
  });

  describe('getRecoveryItemResourceId (via normalizeRecoveryPoint)', () => {
    const basePoint = {
      id: 'p2',
      kind: 'backup' as const,
      mode: 'remote' as const,
      outcome: 'success' as const,
    };

    it('prefers itemResourceId over subjectResourceId', () => {
      expect(
        normalizeRecoveryPoint({ ...basePoint, itemResourceId: 'a', subjectResourceId: 'b' }),
      ).toStrictEqual({ ...basePoint, itemResourceId: 'a' });
    });

    it('falls back to subjectResourceId when itemResourceId is absent', () => {
      expect(normalizeRecoveryPoint({ ...basePoint, subjectResourceId: 'b' })).toStrictEqual({
        ...basePoint,
        itemResourceId: 'b',
      });
    });

    it('omits itemResourceId when both ids are absent', () => {
      // Branch: toTrimmedString(undefined) || toTrimmedString(undefined) -> ''.
      const result = normalizeRecoveryPoint(basePoint);
      expect(result).toStrictEqual(basePoint);
      expect('itemResourceId' in result).toBe(false);
    });
  });

  describe('getRecoveryItemRef (via normalizeRecoveryPoint)', () => {
    const basePoint = {
      id: 'p3',
      kind: 'snapshot' as const,
      mode: 'snapshot' as const,
      outcome: 'success' as const,
    };

    it('prefers itemRef over subjectRef', () => {
      expect(
        normalizeRecoveryPoint({
          ...basePoint,
          itemRef: { type: 'item' },
          subjectRef: { type: 'subject' },
        }),
      ).toStrictEqual({ ...basePoint, itemRef: { type: 'item' } });
    });

    it('falls back to subjectRef when itemRef is absent', () => {
      expect(
        normalizeRecoveryPoint({ ...basePoint, subjectRef: { type: 'subject' } }),
      ).toStrictEqual({ ...basePoint, itemRef: { type: 'subject' } });
    });

    it('omits itemRef when both refs are absent (resolves to null)', () => {
      // Branch: value?.itemRef || value?.subjectRef || null -> null (falsy, not spread).
      const result = normalizeRecoveryPoint(basePoint);
      expect(result).toStrictEqual(basePoint);
      expect('itemRef' in result).toBe(false);
    });
  });

  describe('normalizeRecoveryPoint', () => {
    it('returns a minimal point unchanged when optional fields are absent', () => {
      const minimal = {
        id: 'min',
        kind: 'snapshot' as const,
        mode: 'local' as const,
        outcome: 'failed' as const,
      };
      expect(normalizeRecoveryPoint(minimal)).toStrictEqual(minimal);
    });
  });

  describe('normalizeRecoveryRollup', () => {
    it('returns a minimal rollup unchanged when optional fields are absent', () => {
      const minimal = { rollupId: 'r-min', lastOutcome: 'warning' as const };
      expect(normalizeRecoveryRollup(minimal)).toStrictEqual(minimal);
    });

    it('passes a null display through unchanged', () => {
      expect(
        normalizeRecoveryRollup({ rollupId: 'r1', lastOutcome: 'success', display: null }),
      ).toStrictEqual({ rollupId: 'r1', lastOutcome: 'success', display: null });
    });
  });

  describe('normalizeRecoveryMeta (via response normalizers)', () => {
    it('passes valid finite numeric meta through unchanged', () => {
      const meta = { page: 2, limit: 50, total: 7, totalPages: 1 };
      expect(normalizeRecoveryPointsResponse({ data: [], meta })).toStrictEqual({
        data: [],
        meta,
      });
    });

    it('applies defaults when meta is null or undefined', () => {
      const defaults = { page: 1, limit: 0, total: 0, totalPages: 1 };
      expect(
        normalizeRecoveryPointsResponse({
          data: [],
        } as unknown as Parameters<typeof normalizeRecoveryPointsResponse>[0]),
      ).toStrictEqual({ data: [], meta: defaults });
      expect(
        normalizeRecoveryRollupsResponse({
          data: [],
          meta: null,
        } as unknown as Parameters<typeof normalizeRecoveryRollupsResponse>[0]),
      ).toStrictEqual({ data: [], meta: defaults });
    });
  });

  describe('normalizeRecoveryPointsResponse', () => {
    it('coerces non-finite and non-number meta fields to defaults and non-array data to []', () => {
      expect(
        normalizeRecoveryPointsResponse({
          data: 'not-an-array',
          meta: { page: NaN, limit: Infinity, total: 'twenty', totalPages: undefined },
        } as unknown as Parameters<typeof normalizeRecoveryPointsResponse>[0]),
      ).toStrictEqual({
        data: [],
        meta: { page: 1, limit: 0, total: 0, totalPages: 1 },
      });
    });
  });

  describe('normalizeRecoveryRollupsResponse', () => {
    it('normalizes an array of rollups and preserves finite meta', () => {
      expect(
        normalizeRecoveryRollupsResponse({
          data: [
            {
              rollupId: 'r1',
              lastOutcome: 'success',
              providers: ['truenas'],
            },
          ],
          meta: { page: 1, limit: 10, total: 1, totalPages: 1 },
        }),
      ).toStrictEqual({
        data: [{ rollupId: 'r1', lastOutcome: 'success', platforms: ['truenas'] }],
        meta: { page: 1, limit: 10, total: 1, totalPages: 1 },
      });
    });

    it('returns empty data and default meta for a malformed payload', () => {
      expect(
        normalizeRecoveryRollupsResponse({
          data: null,
          meta: undefined,
        } as unknown as Parameters<typeof normalizeRecoveryRollupsResponse>[0]),
      ).toStrictEqual({
        data: [],
        meta: { page: 1, limit: 0, total: 0, totalPages: 1 },
      });
    });
  });
});
