import { describe, expect, it } from 'vitest';

import {
  PLATFORM_COLUMN_ALIGN_BY_KIND,
  getPlatformColumnAlign,
  type PlatformTableColumnKind,
} from './columnAlignment';

const ALL_KINDS: PlatformTableColumnKind[] = [
  'name',
  'text',
  'metric-bar',
  'numeric-value',
  'badge',
];

describe('columnAlignment', () => {
  describe('PLATFORM_COLUMN_ALIGN_BY_KIND', () => {
    it('maps every canonical column kind to its alignment value', () => {
      // The single source of truth for all platform tables. Each assertion
      // documents the rationale-backed alignment from the module docblock.
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND.name).toBe('left');
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND.text).toBe('left');
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND['metric-bar']).toBe('center');
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND['numeric-value']).toBe('right');
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND.badge).toBe('center');
    });

    it.each([
      ['name', 'left'],
      ['text', 'left'],
      ['metric-bar', 'center'],
      ['numeric-value', 'right'],
      ['badge', 'center'],
    ] as Array<[PlatformTableColumnKind, 'left' | 'right' | 'center']>)(
      'kind %s aligns %s',
      (kind, align) => {
        expect(PLATFORM_COLUMN_ALIGN_BY_KIND[kind]).toBe(align);
      },
    );

    it('is exhaustive over the PlatformTableColumnKind union (no missing or extra keys)', () => {
      expect(Object.keys(PLATFORM_COLUMN_ALIGN_BY_KIND).sort()).toEqual(
        [...ALL_KINDS].sort(),
      );
    });

    it('only emits valid PlatformTableCellAlign values', () => {
      for (const value of Object.values(PLATFORM_COLUMN_ALIGN_BY_KIND)) {
        expect(['left', 'right', 'center']).toContain(value);
      }
    });

    it('covers all three alignment directions across the kinds', () => {
      const directions = new Set(Object.values(PLATFORM_COLUMN_ALIGN_BY_KIND));
      expect(directions).toEqual(new Set(['left', 'right', 'center']));
    });

    it('left-aligns the readable content kinds (name and text)', () => {
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND.name).toBe('left');
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND.text).toBe('left');
    });

    it('right-aligns scannable numeric values', () => {
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND['numeric-value']).toBe('right');
    });

    it('center-aligns the visual cell kinds (metric bars and badges)', () => {
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND['metric-bar']).toBe('center');
      expect(PLATFORM_COLUMN_ALIGN_BY_KIND.badge).toBe('center');
    });
  });

  describe('getPlatformColumnAlign', () => {
    it.each(ALL_KINDS)('returns the map value for kind %s', (kind) => {
      expect(getPlatformColumnAlign(kind)).toBe(PLATFORM_COLUMN_ALIGN_BY_KIND[kind]);
    });

    it('returns left for name', () => {
      expect(getPlatformColumnAlign('name')).toBe('left');
    });

    it('returns left for text', () => {
      expect(getPlatformColumnAlign('text')).toBe('left');
    });

    it('returns center for metric-bar', () => {
      expect(getPlatformColumnAlign('metric-bar')).toBe('center');
    });

    it('returns right for numeric-value', () => {
      expect(getPlatformColumnAlign('numeric-value')).toBe('right');
    });

    it('returns center for badge', () => {
      expect(getPlatformColumnAlign('badge')).toBe('center');
    });

    it('is consistent with the map across repeated calls', () => {
      for (const kind of ALL_KINDS) {
        expect(getPlatformColumnAlign(kind)).toBe(getPlatformColumnAlign(kind));
      }
    });

    // Runtime guard: the lookup is a plain property access on the map, so a
    // value outside the declared union resolves to `undefined` rather than a
    // fallback. This documents the current (typed-safe) behaviour.
    it('returns undefined for an unknown kind at runtime', () => {
      expect(
        getPlatformColumnAlign('unknown' as unknown as PlatformTableColumnKind),
      ).toBeUndefined();
    });
  });
});
