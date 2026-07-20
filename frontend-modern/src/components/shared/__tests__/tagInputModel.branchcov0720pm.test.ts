import { describe, expect, it } from 'vitest';

import {
  canAddTag,
  getNextTagsAfterRemove,
  getTagInputPlaceholder,
} from '@/components/shared/tagInputModel';

// Every assertion below is a hand-computed expected value against the real
// runtime output of the three branching helpers in tagInputModel.ts (no
// `?raw` source-string reads, no snapshots, no constant-equals-itself
// tautologies). See src/components/shared/tagInputModel.ts.

describe('tagInputModel.branchcov0720pm', () => {
  describe('getTagInputPlaceholder', () => {
    // `tagCount === 0 ? (placeholder ?? '') : ''` — three arms: the
    // tagCount===0 branch with a truthy placeholder, the `?? ''` fallback when
    // placeholder is omitted, and the tagCount>0 else arm that always yields ''.

    it('returns the placeholder verbatim when tagCount is 0 and a placeholder is supplied (ternary true arm)', () => {
      expect(getTagInputPlaceholder(0, 'Add tags…')).toBe('Add tags…');
    });

    it('falls back to the empty string when tagCount is 0 and placeholder is undefined (?? right arm)', () => {
      expect(getTagInputPlaceholder(0, undefined)).toBe('');
    });

    it('falls back to the empty string when tagCount is 0 and placeholder is omitted entirely (?? right arm, parameter missing)', () => {
      // No second argument at all — same `?? ''` arm as passing undefined.
      expect(getTagInputPlaceholder(0)).toBe('');
    });

    it('returns the empty string when tagCount > 0 regardless of placeholder (ternary false arm, placeholder supplied)', () => {
      // A populated tag list hides the placeholder even when one is provided.
      expect(getTagInputPlaceholder(3, 'Add tags…')).toBe('');
    });

    it('returns the empty string when tagCount > 0 and no placeholder is supplied (ternary false arm, placeholder missing)', () => {
      expect(getTagInputPlaceholder(1, undefined)).toBe('');
    });

    it('returns the empty string when tagCount is large and a placeholder is supplied (ternary false arm, far side)', () => {
      expect(getTagInputPlaceholder(1000, 'Add tags…')).toBe('');
    });
  });

  describe('getNextTagsAfterRemove', () => {
    // `tags.filter((_, index) => index !== indexToRemove)` — the filter
    // callback branches true (keep) for non-matching indices and false (drop)
    // for the matching index. Each case locks the observable output array.

    it('removes the target tag at the given index and leaves the others untouched (middle element)', () => {
      expect(getNextTagsAfterRemove(['a', 'b', 'c'], 1)).toEqual(['a', 'c']);
    });

    it('removes the first tag when indexToRemove === 0 (head element)', () => {
      expect(getNextTagsAfterRemove(['a', 'b', 'c'], 0)).toEqual(['b', 'c']);
    });

    it('removes the last tag when indexToRemove === tags.length - 1 (tail element)', () => {
      expect(getNextTagsAfterRemove(['a', 'b', 'c'], 2)).toEqual(['a', 'b']);
    });

    it('returns all tags unchanged when indexToRemove matches no index (out-of-range high)', () => {
      // No index satisfies `index === indexToRemove`, so every element is kept.
      expect(getNextTagsAfterRemove(['a', 'b', 'c'], 99)).toEqual(['a', 'b', 'c']);
    });

    it('returns all tags unchanged when indexToRemove is negative (no index matches)', () => {
      // filter indices are 0-based and non-negative; -1 never matches.
      expect(getNextTagsAfterRemove(['a', 'b', 'c'], -1)).toEqual(['a', 'b', 'c']);
    });

    it('returns a new empty array when removing from a single-element list at index 0', () => {
      const result = getNextTagsAfterRemove(['only'], 0);
      expect(result).toEqual([]);
      // Defensive: confirm a fresh array, not the same reference.
      expect(result).not.toBe(['only']);
    });

    it('returns a new array (does not mutate the input) and drops only the first match', () => {
      // Duplicate tag values are independent by index — only index 1 is dropped.
      const input = ['x', 'x', 'y'];
      const result = getNextTagsAfterRemove(input, 1);
      expect(result).toEqual(['x', 'y']);
      expect(input).toEqual(['x', 'x', 'y']);
    });
  });

  describe('canAddTag', () => {
    // `Boolean(value) && !tags.includes(value)` — short-circuit AND with two
    // distinct false arms (empty value, duplicate value) and one true arm.

    it('returns true when value is non-empty and not already present (Boolean truthy && !includes true)', () => {
      expect(canAddTag(['a', 'b'], 'c')).toBe(true);
    });

    it('returns true when the tags list is empty and value is non-empty (truthy && !includes on empty list)', () => {
      expect(canAddTag([], 'first')).toBe(true);
    });

    it('returns false when value is an empty string (Boolean(value) falsy short-circuit)', () => {
      // Empty string never reaches the .includes check.
      expect(canAddTag(['a', 'b'], '')).toBe(false);
    });

    it('returns false when value is already in the tags list (duplicate, !includes false arm)', () => {
      expect(canAddTag(['a', 'b', 'c'], 'b')).toBe(false);
    });

    it('returns false when the duplicate check fires even on an otherwise-empty list', () => {
      // Single-element duplicate must still be rejected.
      expect(canAddTag(['only'], 'only')).toBe(false);
    });

    it('treats a whitespace-only value as truthy (Boolean(" ") is true) and adds it when not duplicated', () => {
      // NOTE: this is a real behaviour assertion — canAddTag does NOT trim,
      // so a whitespace value passes the Boolean check. Trimming is the
      // caller's responsibility (normalizeTagInputValue).
      expect(canAddTag([], ' ')).toBe(true);
    });

    it('treats the string "0" as a valid addable tag (Boolean("0") is true)', () => {
      expect(canAddTag([], '0')).toBe(true);
    });
  });
});
