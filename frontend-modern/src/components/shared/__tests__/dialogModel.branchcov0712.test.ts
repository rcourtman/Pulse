import { describe, expect, it } from 'vitest';
import type { DialogLayout } from '@/components/shared/dialogModel';
import {
  getDialogAlignmentClass,
  getDialogPanelClass,
  getDialogViewportClass,
} from '@/components/shared/dialogModel';

// Drive every branch of getDialogViewportClass, getDialogAlignmentClass, and
// getDialogPanelClass. Each function is a single ternary keyed on
// `layout === 'drawer-right'`; getDialogPanelClass adds a `panelClass ?? ...`
// nullish-coalescing with its own inner drawer-vs-modal ternary, plus a final
// `.trim()`. Every assertion is exact-string equality.

describe('getDialogViewportClass', () => {
  it("returns the 'p-0' padding arm for the drawer-right layout", () => {
    expect(getDialogViewportClass('drawer-right')).toBe(
      'relative h-full overflow-y-auto pointer-events-none p-0',
    );
  });

  it("returns the 'p-4 sm:p-6' padding arm for the modal layout", () => {
    expect(getDialogViewportClass('modal')).toBe(
      'relative h-full overflow-y-auto pointer-events-none p-4 sm:p-6',
    );
  });

  it("falls into the modal (else) arm for any value that isn't strictly 'drawer-right'", () => {
    // Defensive: strict equality means a foreign string lands in the else arm,
    // producing the same classes as 'modal'.
    const foreign = 'drawer-left' as unknown as DialogLayout;
    expect(getDialogViewportClass(foreign)).toBe(
      'relative h-full overflow-y-auto pointer-events-none p-4 sm:p-6',
    );
  });
});

describe('getDialogAlignmentClass', () => {
  it('returns the stretch/right-justified arm for the drawer-right layout', () => {
    expect(getDialogAlignmentClass('drawer-right')).toBe(
      'flex min-h-full items-stretch justify-end',
    );
  });

  it('returns the centered arm for the modal layout', () => {
    expect(getDialogAlignmentClass('modal')).toBe(
      'flex min-h-full items-start justify-center sm:items-center',
    );
  });

  it("falls into the modal (else) arm for any value that isn't strictly 'drawer-right'", () => {
    const foreign = 'bottom-sheet' as unknown as DialogLayout;
    expect(getDialogAlignmentClass(foreign)).toBe(
      'flex min-h-full items-start justify-center sm:items-center',
    );
  });
});

describe('getDialogPanelClass', () => {
  // The shared leading segment of every result, exactly as emitted by the
  // template literal before the layout branch and panelClass slot.
  const BASE =
    'relative flex min-h-0 w-full flex-col overflow-hidden bg-surface border border-border outline-none pointer-events-auto';
  const DRAWER_BRANCH =
    'h-dvh max-w-[720px] rounded-none border-y-0 border-r-0 animate-slide-up sm:h-full sm:max-h-dvh sm:rounded-l-xl sm:border-y sm:border-r-0';
  const MODAL_BRANCH = 'max-h-[calc(100dvh-2rem)] rounded-md animate-slide-up';

  describe('layout ternary — drawer-right arm', () => {
    it('emits the drawer branch and trims the trailing empty fallback when panelClass is undefined', () => {
      // Exercises: layout === 'drawer-right' (true), panelClass ?? (true → '') arm,
      // and the .trim() of the trailing whitespace left by the empty fallback.
      expect(getDialogPanelClass('drawer-right')).toBe(`${BASE} ${DRAWER_BRANCH}`);
    });

    it('appends a provided panelClass after the drawer branch', () => {
      // Exercises: layout === 'drawer-right' (true), panelClass ?? left arm (provided).
      expect(getDialogPanelClass('drawer-right', 'w-[420px]')).toBe(
        `${BASE} ${DRAWER_BRANCH} w-[420px]`,
      );
    });

    it('treats an empty-string panelClass as a present value (?? left arm) yielding the same output as undefined', () => {
      // ?? only fires for null/undefined, so '' is used verbatim. The trailing
      // whitespace from the empty slot is then removed by .trim(), producing the
      // same string as the undefined case above.
      expect(getDialogPanelClass('drawer-right', '')).toBe(`${BASE} ${DRAWER_BRANCH}`);
    });
  });

  describe('layout ternary — modal (else) arm', () => {
    it('appends the max-w-lg fallback after the modal branch when panelClass is undefined', () => {
      // Exercises: layout === 'drawer-right' (false), panelClass ?? (false → 'max-w-lg') arm.
      expect(getDialogPanelClass('modal')).toBe(`${BASE} ${MODAL_BRANCH} max-w-lg`);
    });

    it('appends a provided panelClass after the modal branch', () => {
      // Exercises: layout === 'drawer-right' (false), panelClass ?? left arm (provided).
      expect(getDialogPanelClass('modal', 'max-w-2xl')).toBe(`${BASE} ${MODAL_BRANCH} max-w-2xl`);
    });

    it('treats an empty-string panelClass as a present value, dropping the max-w-lg fallback (?? left arm)', () => {
      // '' is non-nullish so ?? does not fire; the modal-only max-w-lg default
      // is NOT appended. The trailing whitespace is removed by .trim().
      expect(getDialogPanelClass('modal', '')).toBe(`${BASE} ${MODAL_BRANCH}`);
    });
  });

  describe('layout ternary — defensive (foreign value falls into modal arm)', () => {
    it('routes a non-DialogLayout string through the modal branch', () => {
      // Strict equality with 'drawer-right' fails for any other string, so the
      // else (modal) branch is taken for both the layout class and the panel
      // fallback. Confirms the function has no hidden third arm.
      const foreign = 'center' as unknown as DialogLayout;
      expect(getDialogPanelClass(foreign)).toBe(`${BASE} ${MODAL_BRANCH} max-w-lg`);
    });
  });
});
