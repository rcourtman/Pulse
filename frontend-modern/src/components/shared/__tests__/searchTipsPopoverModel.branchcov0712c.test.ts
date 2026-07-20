import { describe, expect, it } from 'vitest';
import {
  getSearchTipsPopoverPositionClass,
  getSearchTipsPopoverTriggerClass,
} from '@/components/shared/searchTipsPopoverModel';

// `triggerBaseClasses` is a module-private const that is not exported, so we
// mirror the literal here in order to assert the full, concrete composed string
// for every branch of getSearchTipsPopoverTriggerClass.
const TRIGGER_BASE_CLASSES =
  'text-xs font-medium focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 focus:ring-offset-white dark:focus:ring-blue-400';

describe('searchTipsPopoverModel.branchcov0712c', () => {
  describe('getSearchTipsPopoverPositionClass', () => {
    it("returns 'left-0' when align === 'left' (truthy ternary arm)", () => {
      expect(getSearchTipsPopoverPositionClass('left')).toBe('left-0');
    });

    it("returns 'right-0' when align === 'right' (falsy ternary arm)", () => {
      expect(getSearchTipsPopoverPositionClass('right')).toBe('right-0');
    });

    it("returns 'right-0' when align is omitted (undefined -> falsy ternary arm)", () => {
      expect(getSearchTipsPopoverPositionClass()).toBe('right-0');
    });

    it('returns the else-arm value for an unexpected align value (defensive)', () => {
      // The ternary only special-cases 'left'; anything else (including a
      // wrong-typed value that slips past the union at runtime) must fall to
      // the 'right-0' arm.
      const bogus = 'center' as unknown as Parameters<typeof getSearchTipsPopoverPositionClass>[0];
      expect(getSearchTipsPopoverPositionClass(bogus)).toBe('right-0');
    });
  });

  describe('getSearchTipsPopoverTriggerClass', () => {
    it("returns the button variant classes when triggerVariant === 'button' (first if arm)", () => {
      expect(getSearchTipsPopoverTriggerClass('button')).toBe(
        `rounded-md border border-border px-2.5 py-1 text-muted transition-colors hover:bg-surface-hover ${TRIGGER_BASE_CLASSES}`,
      );
    });

    it("returns the link variant classes when triggerVariant === 'link' (first if false, second if arm)", () => {
      expect(getSearchTipsPopoverTriggerClass('link')).toBe(
        `rounded px-1 py-0.5 underline decoration-dotted underline-offset-4 transition-colors hover:text-base-content ${TRIGGER_BASE_CLASSES}`,
      );
    });

    it("returns the icon variant classes when triggerVariant === 'icon' (both ifs false -> default return)", () => {
      expect(getSearchTipsPopoverTriggerClass('icon')).toBe(
        `flex h-10 w-10 items-center justify-center rounded-full transition-colors hover:text-muted sm:h-5 sm:w-5 ${TRIGGER_BASE_CLASSES}`,
      );
    });

    it('falls through both ifs to the default return for an unknown variant (defensive)', () => {
      // An out-of-union value must not match either `if` guard and so lands in
      // the trailing default (the icon) return.
      const bogus = 'ghost' as unknown as Parameters<typeof getSearchTipsPopoverTriggerClass>[0];
      expect(getSearchTipsPopoverTriggerClass(bogus)).toBe(
        `flex h-10 w-10 items-center justify-center rounded-full transition-colors hover:text-muted sm:h-5 sm:w-5 ${TRIGGER_BASE_CLASSES}`,
      );
    });

    it('appends the shared trigger base classes to every named variant', () => {
      // Guards the invariant that all three branches keep the focus-ring tail;
      // also exercises each variant once more on a single, concrete assertion.
      expect(getSearchTipsPopoverTriggerClass('button')).toContain(TRIGGER_BASE_CLASSES);
      expect(getSearchTipsPopoverTriggerClass('link')).toContain(TRIGGER_BASE_CLASSES);
      expect(getSearchTipsPopoverTriggerClass('icon')).toContain(TRIGGER_BASE_CLASSES);
    });
  });
});
