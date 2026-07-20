import { describe, expect, it } from 'vitest';

import {
  calculateHelpPopoverPosition,
  getHelpIconSize,
  getHelpPopoverMaxWidth,
  getHelpPopoverPreferredPosition,
  getMissingHelpContentWarning,
  helpIconSizeClasses,
  resolveHelpContent,
} from '@/components/shared/helpIconModel';
import type { HelpIconProps } from '@/components/shared/helpIconModel';

// Module-internal constants mirrored here so each geometry assertion is a
// concrete, hand-computed expected value rather than a re-derivation of source.
// VIEWPORT_PADDING (min inset from any viewport edge) = 8
// POPOVER_OFFSET   (gap between the anchor button and the popover) = 8
const VIEWPORT_PADDING = 8;
const POPOVER_OFFSET = 8;

// Minimal DOMRect builder. `calculateHelpPopoverPosition` reads
// `left`, `width`, `top`, `bottom` off buttonRect and `width`, `height` off
// popoverRect, so we derive `right`/`bottom` from the position + size to mirror
// real DOMRect semantics (callers pass only left/top/width/height).
const makeRect = (rect: {
  left?: number;
  top?: number;
  width?: number;
  height?: number;
}): DOMRect => {
  const left = rect.left ?? 0;
  const top = rect.top ?? 0;
  const width = rect.width ?? 0;
  const height = rect.height ?? 0;
  return {
    left,
    top,
    right: left + width,
    bottom: top + height,
    width,
    height,
    x: left,
    y: top,
    toJSON: () => ({}),
  } as DOMRect;
};

describe('helpIconModel.branchcov0720pm', () => {
  describe('resolveHelpContent — branch coverage', () => {
    it('returns the inline-shaped HelpContent when props.inline is truthy (inline arm)', () => {
      // The inline arm builds a brand-new object with id: 'inline' and copies
      // title/description/examples/docUrl off props.inline verbatim.
      const props: HelpIconProps = {
        inline: {
          title: 'Inline help',
          description: 'Inline description',
          examples: ['Example A', 'Example B'],
          docUrl: 'https://example.com/docs/inline',
        },
      };
      const result = resolveHelpContent(props);
      expect(result).toEqual({
        id: 'inline',
        title: 'Inline help',
        description: 'Inline description',
        examples: ['Example A', 'Example B'],
        docUrl: 'https://example.com/docs/inline',
      });
    });

    it('omits examples/docUrl from the inline shape when the inline content omits them', () => {
      // Same inline arm; the source spreads `examples: props.inline.examples`
      // and `docUrl: props.inline.docUrl` directly, so undefined fields stay
      // undefined and do not appear on the returned object.
      const result = resolveHelpContent({
        inline: { title: 'Bare', description: 'No extras' },
      });
      expect(result).toEqual({
        id: 'inline',
        title: 'Bare',
        description: 'No extras',
        examples: undefined,
        docUrl: undefined,
      });
      expect(result).not.toHaveProperty('related');
      expect(result).not.toHaveProperty('addedInVersion');
    });

    it('prefers inline content over a contentId when both are supplied (inline arm wins)', () => {
      // The inline check comes first, so a present `inline` short-circuits the
      // contentId lookup entirely.
      const result = resolveHelpContent({
        contentId: 'alerts.thresholds.delay',
        inline: { title: 'Wins', description: 'Inline beats contentId' },
      });
      expect(result?.id).toBe('inline');
      expect(result?.title).toBe('Wins');
    });

    it('delegates to getHelpContent when only props.contentId is set (contentId arm)', () => {
      // Real registry hit: 'alerts.thresholds.delay' is a registered entry in
      // src/content/help/alerts.ts, so we get the live HelpContent back.
      const result = resolveHelpContent({ contentId: 'alerts.thresholds.delay' });
      expect(result).toBeDefined();
      expect(result?.id).toBe('alerts.thresholds.delay');
      expect(result?.title).toBe('Alert Delay (Sustained Duration)');
      expect(typeof result?.description).toBe('string');
      expect(result?.description).toContain('Alert delay');
      expect(result?.examples).toEqual(
        expect.arrayContaining([expect.stringContaining('30 seconds')]),
      );
      expect(result?.addedInVersion).toBe('v4.0.0');
    });

    it('returns undefined from the contentId arm when the id is not in the registry', () => {
      // getHelpContent returns undefined for an unknown id -> resolveHelpContent
      // returns that undefined verbatim.
      const result = resolveHelpContent({ contentId: 'no.such.content.id' });
      expect(result).toBeUndefined();
    });

    it('returns undefined when neither inline nor contentId is supplied (neither arm)', () => {
      // Both guards (props.inline, props.contentId) are falsy on an empty
      // object -> falls through to the trailing `return undefined`.
      expect(resolveHelpContent({})).toBeUndefined();
    });
  });

  describe('calculateHelpPopoverPosition — top/bottom and viewport-flip branches', () => {
    it("opens above the button when preferredPosition='top' and there is room (top-fits arm)", () => {
      // viewport 1000x800, button at top:200 height:20 -> bottom:220.
      // popover height 100 -> top = 200 - 100 - 8 = 92 (>= padding, no flip).
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 500, top: 200, width: 20, height: 20 }),
        popoverRect: makeRect({ width: 200, height: 100 }),
        preferredPosition: 'top',
        viewportWidth: 1000,
        viewportHeight: 800,
      });
      // left = 500 + 20/2 - 200/2 = 500 + 10 - 100 = 410 (in range, no clamp).
      expect(result).toEqual({ top: 200 - 100 - POPOVER_OFFSET, left: 410 });
    });

    it("flips below the button when preferredPosition='top' would overflow the top of the viewport", () => {
      // button top:50, popover height:100 -> top = 50 - 100 - 8 = -58 (< 8).
      // Flip arm: top = buttonRect.bottom + POPOVER_OFFSET = 70 + 8 = 78.
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 500, top: 50, width: 20, height: 20 }),
        popoverRect: makeRect({ width: 200, height: 100 }),
        preferredPosition: 'top',
        viewportWidth: 1000,
        viewportHeight: 800,
      });
      expect(result.top).toBe(50 + 20 + POPOVER_OFFSET);
      expect(result.left).toBe(410);
    });

    it("opens below the button when preferredPosition='bottom' and there is room (bottom-fits arm)", () => {
      // button bottom:420, popover height:100 -> top = 420 + 8 = 428.
      // 428 + 100 = 528 <= 800 - 8 = 792 -> no flip.
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 500, top: 400, width: 20, height: 20 }),
        popoverRect: makeRect({ width: 200, height: 100 }),
        preferredPosition: 'bottom',
        viewportWidth: 1000,
        viewportHeight: 800,
      });
      expect(result).toEqual({ top: 420 + POPOVER_OFFSET, left: 410 });
    });

    it("flips above the button when preferredPosition='bottom' would overflow the bottom of the viewport", () => {
      // button bottom:770, popover height:100 -> top = 770 + 8 = 778.
      // 778 + 100 = 878 > 800 - 8 = 792 -> flip arm.
      // top = buttonRect.top - popoverRect.height - POPOVER_OFFSET = 750 - 100 - 8 = 642.
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 500, top: 750, width: 20, height: 20 }),
        popoverRect: makeRect({ width: 200, height: 100 }),
        preferredPosition: 'bottom',
        viewportWidth: 1000,
        viewportHeight: 800,
      });
      expect(result.top).toBe(750 - 100 - POPOVER_OFFSET);
      expect(result.left).toBe(410);
    });

    it('clamps left to VIEWPORT_PADDING when the centered popover would overflow the left edge', () => {
      // button at left:0 width:20 -> center at x=10 -> popover left = 10 - 100 = -90.
      // max(8, min(-90, 792)) = max(8, -90) = 8.
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 0, top: 400, width: 20, height: 20 }),
        popoverRect: makeRect({ width: 200, height: 100 }),
        preferredPosition: 'top',
        viewportWidth: 1000,
        viewportHeight: 800,
      });
      expect(result.left).toBe(VIEWPORT_PADDING);
      // top axis is unaffected: 400 - 100 - 8 = 292.
      expect(result.top).toBe(292);
    });

    it('clamps left to viewportWidth - popoverRect.width - VIEWPORT_PADDING when overflowing right', () => {
      // button at left:990 width:20 -> center at x=1000 -> popover left = 1000 - 100 = 900.
      // maxRight = 1000 - 200 - 8 = 792 -> min(900, 792) = 792 -> max(8, 792) = 792.
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 990, top: 400, width: 20, height: 20 }),
        popoverRect: makeRect({ width: 200, height: 100 }),
        preferredPosition: 'top',
        viewportWidth: 1000,
        viewportHeight: 800,
      });
      expect(result.left).toBe(1000 - 200 - VIEWPORT_PADDING);
      expect(result.top).toBe(292);
    });

    it('clamps the final top to VIEWPORT_PADDING when the post-flip value is still negative', () => {
      // Edge case: top arm flips to bottom, but bottom also pushes past viewport
      // bottom, so the trailing top clamp pulls it back down to VIEWPORT_PADDING.
      // viewport 100x80, button top:0 height:4 bottom:4, popover height:100.
      // top arm: top = 0 - 100 - 8 = -108 (< 8) -> flip -> top = 4 + 8 = 12.
      // Final clamp: max(8, min(12, 80 - 100 - 8 = -28)) = max(8, -28) = 8.
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 10, top: 0, width: 4, height: 4 }),
        popoverRect: makeRect({ width: 20, height: 100 }),
        preferredPosition: 'top',
        viewportWidth: 100,
        viewportHeight: 80,
      });
      expect(result.top).toBe(VIEWPORT_PADDING);
    });

    it('clamps the final top to viewportHeight - popoverRect.height - VIEWPORT_PADDING on the bottom arm', () => {
      // viewport 100x80, button bottom near the bottom edge, popover taller than viewport.
      // bottom arm: top = buttonRect.bottom + 8.
      // Final clamp: max(8, min(top, 80 - 100 - 8 = -28)) = max(8, -28) = 8.
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 10, top: 70, width: 4, height: 4 }),
        popoverRect: makeRect({ width: 20, height: 100 }),
        preferredPosition: 'bottom',
        viewportWidth: 100,
        viewportHeight: 80,
      });
      expect(result.top).toBe(VIEWPORT_PADDING);
    });

    it('returns a plain { top, left } object with no extra enumerable keys', () => {
      const result = calculateHelpPopoverPosition({
        buttonRect: makeRect({ left: 100, top: 100, width: 10, height: 10 }),
        popoverRect: makeRect({ width: 50, height: 50 }),
        preferredPosition: 'top',
        viewportWidth: 1000,
        viewportHeight: 800,
      });
      expect(Object.keys(result).sort()).toEqual(['left', 'top']);
      expect(typeof result.left).toBe('number');
      expect(typeof result.top).toBe('number');
    });
  });

  describe('getHelpIconSize — ?? fallback branches', () => {
    it("returns 'sm' when size is omitted (right operand arm)", () => {
      expect(getHelpIconSize(undefined)).toBe('sm');
    });

    it('returns the supplied size verbatim when set (left operand arm)', () => {
      expect(getHelpIconSize('xs')).toBe('xs');
      expect(getHelpIconSize('sm')).toBe('sm');
      expect(getHelpIconSize('md')).toBe('md');
    });
  });

  describe('getHelpPopoverMaxWidth — ?? fallback branches', () => {
    it('returns 320 when maxWidth is omitted (right operand arm)', () => {
      expect(getHelpPopoverMaxWidth(undefined)).toBe(320);
    });

    it('returns the supplied maxWidth verbatim when set (left operand arm)', () => {
      expect(getHelpPopoverMaxWidth(200)).toBe(200);
      expect(getHelpPopoverMaxWidth(0)).toBe(0);
    });
  });

  describe('getHelpPopoverPreferredPosition — ?? fallback branches', () => {
    it("returns 'top' when position is omitted (right operand arm)", () => {
      expect(getHelpPopoverPreferredPosition(undefined)).toBe('top');
    });

    it('returns the supplied position verbatim when set (left operand arm)', () => {
      expect(getHelpPopoverPreferredPosition('top')).toBe('top');
      expect(getHelpPopoverPreferredPosition('bottom')).toBe('bottom');
    });
  });

  describe('getMissingHelpContentWarning — branch coverage', () => {
    it('returns undefined when contentId is falsy (early-return arm)', () => {
      expect(getMissingHelpContentWarning(undefined)).toBeUndefined();
      expect(getMissingHelpContentWarning('')).toBeUndefined();
    });

    it('formats the warning string with the supplied contentId (template arm)', () => {
      expect(getMissingHelpContentWarning('alerts.thresholds.delay')).toBe(
        '[HelpIcon] No content found for ID: alerts.thresholds.delay',
      );
      expect(getMissingHelpContentWarning('some.missing.id')).toBe(
        '[HelpIcon] No content found for ID: some.missing.id',
      );
    });
  });

  describe('helpIconSizeClasses — constant map contents', () => {
    it('exposes a tailwind class for every declared size and nothing else', () => {
      expect(Object.keys(helpIconSizeClasses).sort()).toEqual(['md', 'sm', 'xs']);
      expect(helpIconSizeClasses.xs).toBe('w-3 h-3');
      expect(helpIconSizeClasses.sm).toBe('w-3.5 h-3.5');
      expect(helpIconSizeClasses.md).toBe('w-4 h-4');
    });
  });
});
