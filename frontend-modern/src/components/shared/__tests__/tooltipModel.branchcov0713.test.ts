import { describe, expect, it } from 'vitest';
import {
  resolveTooltipPosition,
  sanitizeTooltipContent,
} from '@/components/shared/tooltipModel';
import type {
  TooltipAlignment,
  TooltipDirection,
  TooltipPosition,
} from '@/components/shared/tooltipModel';

// Module-internal constants mirrored here so each assertion is a concrete,
// hand-computed expected value rather than a re-derivation of the source.
// padding (gap between anchor and tooltip) = 8
// viewportPadding (min inset from the viewport edge) = 4
const PADDING = 8;
const VIEWPORT_PADDING = 4;

describe('tooltipModel.branchcov0713', () => {
  describe('sanitizeTooltipContent', () => {
    it('returns an empty string for empty-string input (Array.from produces no chars)', () => {
      expect(sanitizeTooltipContent('')).toBe('');
    });

    it('passes plain ASCII printable text through unchanged', () => {
      const text = 'Hello, World!';
      expect(sanitizeTooltipContent(text)).toBe(text);
    });

    it("keeps '\\n' (newline) via the first char === '\\n' arm", () => {
      expect(sanitizeTooltipContent('a\nb')).toBe('a\nb');
    });

    it("keeps '\\t' (tab) via the char === '\\t' arm", () => {
      expect(sanitizeTooltipContent('a\tb')).toBe('a\tb');
    });

    it("keeps both '\\n' and '\\t' together", () => {
      expect(sanitizeTooltipContent('\ta\tb\nc\td')).toBe('\ta\tb\nc\td');
    });

    it('returns a string that is only a newline unchanged', () => {
      expect(sanitizeTooltipContent('\n')).toBe('\n');
    });

    it("strips NUL ('\\x00') because codePoint 0 < 0x20 and it is not '\\n' or '\\t'", () => {
      expect(sanitizeTooltipContent('a\x00b')).toBe('ab');
    });

    it("strips BEL ('\\x07') and other C0 controls below 0x20", () => {
      expect(sanitizeTooltipContent('a\x07b\x01c')).toBe('abc');
    });

    it("strips CR ('\\r', 0x0d): below 0x20 and explicitly not '\\n'", () => {
      // Guards the subtlety that only '\n' (not '\r') is whitelisted among
      // line-break characters.
      expect(sanitizeTooltipContent('a\rb')).toBe('ab');
    });

    it("strips DEL ('\\x7f') via the codePoint !== 0x7f arm even though 0x7f >= 0x20", () => {
      expect(sanitizeTooltipContent('a\x7fb')).toBe('ab');
    });

    it('strips a string composed entirely of control characters to the empty string', () => {
      expect(sanitizeTooltipContent('\x00\x01\x07\x7f')).toBe('');
    });

    it("keeps '~' (0x7e), the printable char immediately below DEL", () => {
      // Boundary: 0x7e passes both `>= 0x20` and `!== 0x7f`.
      expect(sanitizeTooltipContent('a~b')).toBe('a~b');
    });

    it("keeps space (0x20), the lowest printable char (>= 0x20 boundary)", () => {
      expect(sanitizeTooltipContent('a b')).toBe('a b');
    });

    it('keeps an astral-plane emoji (codePoint well above 0x20, != 0x7f)', () => {
      expect(sanitizeTooltipContent('a😀b')).toBe('a😀b');
    });

    it('keeps a surrogate-pair code point as a single retained character', () => {
      // U+1D547 MATHEMATICAL DOUBLE-STRUCK CAPITAL Q -> codePoint 120135.
      // Array.from splits by code point, so the pair is one element and is kept.
      expect(sanitizeTooltipContent('x𝕏y')).toBe('x𝕏y');
    });

    it('filters a mixed string down to exactly its whitelisted characters', () => {
      expect(sanitizeTooltipContent('\thello\n\x00world\x7fend')).toBe(
        '\thello\nworldend',
      );
    });

    it('does not mutate or reorder retained characters when nothing is stripped', () => {
      const text = '  plainly visible  ';
      expect(sanitizeTooltipContent(text)).toBe(text);
    });
  });

  describe('resolveTooltipPosition', () => {
    it("centers horizontally and opens above when align='center' and direction='up' (no clamping)", () => {
      const result = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 500,
        y: 600,
        align: 'center',
        direction: 'up',
      });
      // left = x - width/2 = 500 - 50 = 450
      // top  = y - height - padding = 600 - 50 - 8 = 542
      // Neither value exceeds [viewportPadding, maxLeft]/[viewportPadding, maxTop].
      expect(result).toEqual({ left: 450, top: 542 } satisfies TooltipPosition);
    });

    it("keeps x and opens below when align='left' and direction='down' (no clamping)", () => {
      const result = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 500,
        y: 600,
        align: 'left',
        direction: 'down',
      });
      // left stays x = 500; top = y + padding = 600 + 8 = 608.
      expect(result).toEqual({ left: 500, top: 608 } satisfies TooltipPosition);
    });

    it("defaults align to 'center' and direction to 'up' when both are omitted (?? right arms)", () => {
      const withDefaults = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 500,
        y: 600,
      });
      // Same computation as the explicit center/up case.
      expect(withDefaults).toEqual({ left: 450, top: 542 });
    });

    it('clamps an above-viewport / left-of-viewport position back down to viewportPadding on both axes', () => {
      const result = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: -100,
        y: 10,
        align: 'left',
        direction: 'up',
      });
      // align left: left = x = -100 -> max(-100, 4) = 4 -> min(4, 896) = 4.
      // dir up:    top  = 10 - 50 - 8 = -48 -> max(-48, 4) = 4 -> min(4, 746) = 4.
      expect(result).toEqual({
        left: VIEWPORT_PADDING,
        top: VIEWPORT_PADDING,
      } satisfies TooltipPosition);
    });

    it('clamps a past-the-right / past-the-bottom position back to maxLeft / maxTop', () => {
      const result = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 2000,
        y: 2000,
        align: 'left',
        direction: 'down',
      });
      // maxLeft = 1000 - 100 - 4 = 896; maxTop = 800 - 50 - 4 = 746.
      // left = 2000 -> min(max(2000, 4), 896) = 896.
      // dir down: top = 2000 + 8 = 2008 -> min(max(2008, 4), 746) = 746.
      expect(result).toEqual({
        left: 1000 - 100 - VIEWPORT_PADDING,
        top: 800 - 50 - VIEWPORT_PADDING,
      } satisfies TooltipPosition);
    });

    it('forces both axes to viewportPadding when the rect is larger than the viewport (maxLeft/maxTop < viewportPadding branch)', () => {
      // The defensive `Math.max(maxLeft, viewportPadding)` term collapses to
      // viewportPadding when the tooltip does not fit at all.
      const result = resolveTooltipPosition({
        rect: { width: 1000, height: 1000 },
        viewportWidth: 100,
        viewportHeight: 100,
        x: 500,
        y: 500,
        align: 'left',
        direction: 'up',
      });
      // maxLeft = 100 - 1000 - 4 = -904 -> max(-904, 4) = 4.
      // left = min(max(500, 4), 4) = 4. Same for top.
      expect(result).toEqual({
        left: VIEWPORT_PADDING,
        top: VIEWPORT_PADDING,
      } satisfies TooltipPosition);
    });

    it("handles a zero-size rect: align='center' subtracts 0 and direction='up' subtracts only padding", () => {
      const result = resolveTooltipPosition({
        rect: { width: 0, height: 0 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 500,
        y: 600,
        align: 'center',
        direction: 'up',
      });
      // left = 500 - 0/2 = 500; top = 600 - 0 - 8 = 592.
      // maxLeft = 996, maxTop = 796 -> no clamping.
      expect(result).toEqual({ left: 500, top: 600 - PADDING } satisfies TooltipPosition);
    });

    it("clamps each axis independently when only one overflows (left axis overflows, top is in range)", () => {
      const result = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 5000,
        y: 100,
        align: 'center',
        direction: 'down',
      });
      // align center: left = 5000 - 50 = 4950 -> min(max(4950, 4), 896) = 896.
      // dir down: top = 100 + 8 = 108 -> in range, stays 108.
      expect(result).toEqual({
        left: 1000 - 100 - VIEWPORT_PADDING,
        top: 100 + PADDING,
      } satisfies TooltipPosition);
    });

    it("treats an out-of-union align as NOT 'center' (falls to the align='left' behavior)", () => {
      // Defensive: strict equality means any foreign value skips the centering
      // branch, leaving left === x.
      const bogusAlign =
        'right' as unknown as Parameters<typeof resolveTooltipPosition>[0]['align'];
      const result = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 300,
        y: 400,
        align: bogusAlign,
        direction: 'up',
      });
      // left stays 300 (no centering); top = 400 - 50 - 8 = 342.
      expect(result).toEqual({ left: 300, top: 342 } satisfies TooltipPosition);
    });

    it("treats an out-of-union direction as NOT 'up' (falls to the direction='down' behavior)", () => {
      const bogusDirection =
        'sideways' as unknown as TooltipDirection;
      const result = resolveTooltipPosition({
        rect: { width: 100, height: 50 },
        viewportWidth: 1000,
        viewportHeight: 800,
        x: 300,
        y: 400,
        align: 'center',
        direction: bogusDirection,
      });
      // align center: left = 300 - 50 = 250.
      // bogus direction -> else arm: top = 400 + 8 = 408.
      expect(result).toEqual({ left: 250, top: 408 } satisfies TooltipPosition);
    });

    it("computes maxLeft and maxTop from the supplied viewport dims even with an unusual align/direction pairing", () => {
      // align='left', direction='up' with a tight viewport to exercise both
      // clamps at once using only declared-union values.
      const result = resolveTooltipPosition({
        rect: { width: 40, height: 40 },
        viewportWidth: 200,
        viewportHeight: 150,
        x: 190,
        y: 5,
        align: 'left',
        direction: 'up',
      });
      // align left: left = 190.
      // maxLeft = 200 - 40 - 4 = 156 -> min(max(190, 4), 156) = 156.
      // dir up: top = 5 - 40 - 8 = -43 -> min(max(-43, 4), 106) = 4.
      // maxTop = 150 - 40 - 4 = 106.
      expect(result).toEqual({
        left: 200 - 40 - VIEWPORT_PADDING,
        top: VIEWPORT_PADDING,
      } satisfies TooltipPosition);
    });

    it('returns a plain { left, top } object with no extra enumerable keys', () => {
      const result = resolveTooltipPosition({
        rect: { width: 10, height: 10 },
        viewportWidth: 100,
        viewportHeight: 100,
        x: 50,
        y: 50,
        align: 'center',
        direction: 'down',
      });
      expect(Object.keys(result).sort()).toEqual(['left', 'top']);
      expect(typeof result.left).toBe('number');
      expect(typeof result.top).toBe('number');
    });

    it('matches the documented alignment/direction union members exhaustively', () => {
      // One assertion per declared align x direction combination, computed
      // against a viewport generous enough that no clamping occurs.
      const rect = { width: 20, height: 10 };
      const viewportWidth = 1000;
      const viewportHeight = 1000;
      const x = 200;
      const y = 300;

      const aligns: TooltipAlignment[] = ['left', 'center'];
      const directions: TooltipDirection[] = ['up', 'down'];

      for (const align of aligns) {
        for (const direction of directions) {
          const result = resolveTooltipPosition({
            rect,
            viewportWidth,
            viewportHeight,
            x,
            y,
            align,
            direction,
          });
          const expectedLeft = align === 'center' ? x - rect.width / 2 : x;
          const expectedTop =
            direction === 'up' ? y - rect.height - PADDING : y + PADDING;
          expect(result).toEqual({
            left: expectedLeft,
            top: expectedTop,
          } satisfies TooltipPosition);
        }
      }
    });
  });
});
