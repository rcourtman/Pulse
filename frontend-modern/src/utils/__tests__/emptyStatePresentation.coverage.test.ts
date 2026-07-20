import { describe, expect, it } from 'vitest';
import { getEmptyStatePresentation, type EmptyStateTone } from '@/utils/emptyStatePresentation';

// NOTE: The existing `emptyStatePresentation.test.ts` only covers the `danger`
// and `default` tones. This file adds coverage for the remaining tones, the
// cross-tone distinctness invariant, and the actual behavior for invalid input.
//
// IMPORTANT divergence from the task brief: the source has NO `|| DEFAULT`
// fallback branch. `getEmptyStatePresentation` performs a raw record lookup
// (`EMPTY_STATE_PRESENTATION[tone]`), so any tone absent from the record
// resolves to `undefined` rather than falling back to default classes. These
// tests assert that real current behavior.

describe('getEmptyStatePresentation — full tone coverage', () => {
  it('every supported tone yields a distinct value for each class field', () => {
    const tones: EmptyStateTone[] = ['default', 'info', 'success', 'warning', 'danger'];

    const presentations = tones.map((tone) => getEmptyStatePresentation(tone));

    const iconClasses = new Set(presentations.map((p) => p.iconClass));
    const titleClasses = new Set(presentations.map((p) => p.titleClass));
    const descriptionClasses = new Set(presentations.map((p) => p.descriptionClass));

    // Distinctness across all five tones proves the tones are not aliases of
    // one another; a size < 5 would indicate a duplicated (copy-paste) class.
    expect(iconClasses.size).toBe(tones.length);
    expect(titleClasses.size).toBe(tones.length);
    expect(descriptionClasses.size).toBe(tones.length);
  });

  it('default and danger tones are differentiated (existing-test pair, asserted indirectly)', () => {
    // The existing test asserts their exact literals independently; here we
    // additionally prove they are not the same presentation object.
    const def = getEmptyStatePresentation('default');
    const danger = getEmptyStatePresentation('danger');
    expect(def.iconClass).not.toBe(danger.iconClass);
    expect(def.titleClass).not.toBe(danger.titleClass);
    expect(def.descriptionClass).not.toBe(danger.descriptionClass);
  });

  describe('invalid or missing tone input', () => {
    // Source does a raw record lookup with no fallback, so every unrecognized
    // key resolves to `undefined` (no `|| DEFAULT` branch exists).
    it.each([
      ['unknown string key', 'nonexistent'],
      ['empty string', ''],
      ['null', null],
      ['undefined', undefined],
    ])('returns undefined for %s', (_label, invalid) => {
      const result = getEmptyStatePresentation(invalid as unknown as EmptyStateTone);
      expect(result).toBeUndefined();
    });

    it('does not fall back to default classes for an unknown tone', () => {
      const fallback = getEmptyStatePresentation('nonexistent' as unknown as EmptyStateTone);
      const def = getEmptyStatePresentation('default');
      // Explicitly documents the absence of a default-fallback branch.
      expect(fallback).not.toEqual(def);
      expect(fallback).toBeUndefined();
    });
  });
});
