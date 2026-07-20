import { describe, expect, it } from 'vitest';

import {
  getSelectionCardButtonClass,
  getSelectionCardDescriptionClass,
  getSelectionCardGroupClass,
  getSelectionCardIconContainerClass,
  getSelectionCardTitleClass,
  resolveSelectionCardGroupVariant,
  resolveSelectionCardTone,
} from '@/components/shared/selectionCardGroupModel';
import type {
  SelectionCardGroupVariant,
  SelectionCardTone,
} from '@/components/shared/selectionCardGroupModel';

// Module-internal class-string fragments mirrored here so every assertion is a
// concrete, hand-copied expected value (no `?raw` source string reads, no
// snapshots, no constant-equals-itself tautologies). See
// src/components/shared/selectionCardGroupModel.ts.

const GROUP_CLASS_COMPACT = 'grid grid-cols-2 gap-2';
const GROUP_CLASS_DETAIL = 'grid grid-cols-1 gap-3';

const BUTTON_BASE_DETAIL = 'p-4 rounded-md border-2 transition-all text-left';
const BUTTON_BASE_COMPACT = 'p-3 rounded-md border-2 transition-all text-center';

const ACTIVE_SUCCESS = 'border-green-500 bg-green-50 dark:bg-green-900';
const ACTIVE_ACCENT = 'border-blue-500 bg-blue-50 dark:bg-blue-900';

const INACTIVE_COMPACT = 'border-border hover:border-blue-300';
const INACTIVE_DETAIL = 'border-border hover:border-border';

const DISABLED_CLASSES = 'disabled:opacity-50 disabled:cursor-not-allowed';

describe('selectionCardGroupModel.branchcov0720pm', () => {
  describe('resolveSelectionCardGroupVariant', () => {
    it("defaults to 'detail' when variant is undefined (?? right arm)", () => {
      expect(resolveSelectionCardGroupVariant(undefined)).toBe('detail');
    });

    it("returns 'compact' verbatim when explicitly passed (identity arm)", () => {
      expect(resolveSelectionCardGroupVariant('compact')).toBe('compact');
    });

    it("returns 'detail' verbatim when explicitly passed (identity arm)", () => {
      expect(resolveSelectionCardGroupVariant('detail')).toBe('detail');
    });
  });

  describe('resolveSelectionCardTone', () => {
    it("defaults to 'accent' when tone is undefined (?? right arm)", () => {
      expect(resolveSelectionCardTone(undefined)).toBe('accent');
    });

    it("returns 'accent' verbatim when explicitly passed (identity arm)", () => {
      expect(resolveSelectionCardTone('accent')).toBe('accent');
    });

    it("returns 'success' verbatim when explicitly passed (identity arm)", () => {
      expect(resolveSelectionCardTone('success')).toBe('success');
    });
  });

  describe('getSelectionCardGroupClass', () => {
    it("emits the detail grid tokens when variant='detail' and no className is supplied (?? '' arm, then trim)", () => {
      // `${groupClassByVariant[variant]} ${className ?? ''}`.trim()
      // With className undefined the trailing ' ' is trimmed away.
      expect(getSelectionCardGroupClass('detail', undefined)).toBe(GROUP_CLASS_DETAIL);
    });

    it("emits the compact grid tokens when variant='compact' and no className is supplied", () => {
      expect(getSelectionCardGroupClass('compact', undefined)).toBe(GROUP_CLASS_COMPACT);
    });

    it("appends a caller-supplied className for variant='detail' (className truthy arm)", () => {
      expect(getSelectionCardGroupClass('detail', 'mt-4')).toBe(`${GROUP_CLASS_DETAIL} mt-4`);
    });

    it("appends a caller-supplied className for variant='compact' (className truthy arm)", () => {
      expect(getSelectionCardGroupClass('compact', 'sm:grid-cols-3')).toBe(
        `${GROUP_CLASS_COMPACT} sm:grid-cols-3`,
      );
    });
  });

  describe('getSelectionCardButtonClass', () => {
    it("uses the detail (text-left) base, accent active classes, and skips the disabled tail when disabled=false and active=true with tone='accent'", () => {
      // base: variant === 'detail' true arm.
      // active true + tone accent -> getSelectionCardActiveClass('accent').
      // disabled false -> '' third slot, which still contributes a single space.
      const expected = [BUTTON_BASE_DETAIL, ACTIVE_ACCENT, ''].join(' ');
      expect(getSelectionCardButtonClass('detail', 'accent', true, false)).toBe(expected);
      // Sanity-check the three observable fragments directly.
      expect(getSelectionCardButtonClass('detail', 'accent', true, false)).toContain(
        BUTTON_BASE_DETAIL,
      );
      expect(getSelectionCardButtonClass('detail', 'accent', true, false)).toContain(ACTIVE_ACCENT);
      expect(getSelectionCardButtonClass('detail', 'accent', true, false)).not.toContain(
        DISABLED_CLASSES,
      );
    });

    it("uses the success active classes for tone='success' (getSelectionCardActiveClass success arm)", () => {
      const result = getSelectionCardButtonClass('detail', 'success', true, false);
      expect(result).toContain(ACTIVE_SUCCESS);
      expect(result).not.toContain(ACTIVE_ACCENT);
    });

    it("uses the compact (text-center) base when variant='compact' (variant === 'detail' false arm)", () => {
      const result = getSelectionCardButtonClass('compact', 'accent', true, false);
      expect(result).toContain(BUTTON_BASE_COMPACT);
      expect(result).not.toContain(BUTTON_BASE_DETAIL);
    });

    it("uses the compact inactive classes when active=false and variant='compact' (getSelectionCardInactiveClass compact arm)", () => {
      const result = getSelectionCardButtonClass('compact', 'accent', false, false);
      expect(result).toContain(INACTIVE_COMPACT);
      expect(result).not.toContain(INACTIVE_DETAIL);
      expect(result).not.toContain(ACTIVE_ACCENT);
    });

    it("uses the detail inactive classes when active=false and variant='detail' (getSelectionCardInactiveClass else arm)", () => {
      const result = getSelectionCardButtonClass('detail', 'accent', false, false);
      expect(result).toContain(INACTIVE_DETAIL);
      expect(result).not.toContain(INACTIVE_COMPACT);
    });

    it("appends the disabled tail when disabled=true (disabled ? ... : '' true arm)", () => {
      const result = getSelectionCardButtonClass('detail', 'accent', false, true);
      expect(result).toContain(DISABLED_CLASSES);
      // The button is still inactive here, so the inactive-detail classes must
      // remain present alongside the disabled tail.
      expect(result).toContain(INACTIVE_DETAIL);
    });

    it('joins base + active + disabled into a single string when all three slots are populated', () => {
      const expected = [BUTTON_BASE_DETAIL, ACTIVE_SUCCESS, DISABLED_CLASSES].join(' ');
      expect(getSelectionCardButtonClass('detail', 'success', true, true)).toBe(expected);
    });
  });

  describe('getSelectionCardIconContainerClass', () => {
    it("emits the green active container when tone='success' and active=true (tone === 'success' arm)", () => {
      const result = getSelectionCardIconContainerClass('success', true);
      expect(result).toBe('p-2 rounded-md bg-green-100 dark:bg-green-800');
      expect(result).toContain('bg-green-100');
      expect(result).toContain('dark:bg-green-800');
    });

    it("emits the blue active container when tone='accent' and active=true (else arm)", () => {
      const result = getSelectionCardIconContainerClass('accent', true);
      expect(result).toBe('p-2 rounded-md bg-blue-100 dark:bg-blue-800');
      expect(result).toContain('bg-blue-100');
      expect(result).not.toContain('bg-green-100');
    });

    it('emits the surface-alt container when active=false regardless of tone (active false arm)', () => {
      expect(getSelectionCardIconContainerClass('accent', false)).toBe(
        'p-2 rounded-md bg-surface-alt',
      );
      // Same surface-alt path for the success tone proves the inactive arm is
      // independent of tone.
      expect(getSelectionCardIconContainerClass('success', false)).toBe(
        'p-2 rounded-md bg-surface-alt',
      );
    });
  });

  describe('getSelectionCardTitleClass', () => {
    it("returns the compact title class for variant='compact' regardless of tone/active (variant === 'compact' early return)", () => {
      const expected = 'text-sm font-medium text-base-content';
      // active true, success tone — but compact short-circuits before tone is read.
      expect(getSelectionCardTitleClass('compact', 'success', true)).toBe(expected);
      // active false, accent tone — same early return.
      expect(getSelectionCardTitleClass('compact', 'accent', false)).toBe(expected);
    });

    it("returns the muted title class for variant='detail' and active=false (!active early return)", () => {
      // tone is irrelevant on this arm; only the !active check matters.
      expect(getSelectionCardTitleClass('detail', 'success', false)).toBe(
        'text-sm font-semibold text-base-content',
      );
      expect(getSelectionCardTitleClass('detail', 'accent', false)).toBe(
        'text-sm font-semibold text-base-content',
      );
    });

    it("returns the green title class for variant='detail', active=true, tone='success' (tone === 'success' arm)", () => {
      expect(getSelectionCardTitleClass('detail', 'success', true)).toBe(
        'text-sm font-semibold text-green-900 dark:text-green-100',
      );
    });

    it("returns the blue title class for variant='detail', active=true, tone='accent' (else arm)", () => {
      expect(getSelectionCardTitleClass('detail', 'accent', true)).toBe(
        'text-sm font-semibold text-blue-900 dark:text-blue-100',
      );
    });
  });

  describe('getSelectionCardDescriptionClass', () => {
    it("emits the compact description class for variant='compact' (variant === 'compact' true arm)", () => {
      expect(getSelectionCardDescriptionClass('compact')).toBe('text-xs text-slate-500 mt-0.5');
    });

    it("emits the detail description class for variant='detail' (else arm)", () => {
      expect(getSelectionCardDescriptionClass('detail')).toBe('text-xs text-muted');
    });
  });

  // Cross-product sanity sweep — locks the tone x variant x active matrix that
  // callers rely on so a future refactor cannot silently swap a class fragment
  // for a different arm. Each cell asserts the tone/variant/active-specific
  // tokens that the source actually emits for that combination.
  describe('tone x variant x active matrix', () => {
    const tones: SelectionCardTone[] = ['accent', 'success'];
    const variants: SelectionCardGroupVariant[] = ['compact', 'detail'];

    it('selects the green or blue active class on the button based on tone, for every variant', () => {
      for (const variant of variants) {
        for (const tone of tones) {
          const result = getSelectionCardButtonClass(variant, tone, true, false);
          const activeToken =
            tone === 'success' ? 'border-green-500 bg-green-50' : 'border-blue-500 bg-blue-50';
          expect(result).toContain(activeToken);
        }
      }
    });

    it('selects the inactive class on the button based on variant, not tone, when active=false', () => {
      for (const variant of variants) {
        for (const tone of tones) {
          const result = getSelectionCardButtonClass(variant, tone, false, false);
          const inactiveToken =
            variant === 'compact' ? 'hover:border-blue-300' : 'hover:border-border';
          expect(result).toContain(inactiveToken);
          // No active classes leak through when inactive.
          expect(result).not.toContain('bg-green-50');
          expect(result).not.toContain('bg-blue-50');
        }
      }
    });

    it('routes the icon container class by tone when active and by surface-alt when inactive', () => {
      for (const tone of tones) {
        const activeResult = getSelectionCardIconContainerClass(tone, true);
        expect(activeResult).toContain(tone === 'success' ? 'bg-green-100' : 'bg-blue-100');

        const inactiveResult = getSelectionCardIconContainerClass(tone, false);
        expect(inactiveResult).toContain('bg-surface-alt');
      }
    });

    it('switches the detail-mode active title colour between blue and green by tone', () => {
      for (const tone of tones) {
        const result = getSelectionCardTitleClass('detail', tone, true);
        const titleToken =
          tone === 'success'
            ? 'text-green-900 dark:text-green-100'
            : 'text-blue-900 dark:text-blue-100';
        expect(result).toContain(titleToken);
      }
    });

    it('keeps the compact title class tone- and active-independent across the matrix', () => {
      for (const tone of tones) {
        for (const active of [true, false]) {
          expect(getSelectionCardTitleClass('compact', tone, active)).toBe(
            'text-sm font-medium text-base-content',
          );
        }
      }
    });
  });
});
