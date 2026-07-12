import { describe, expect, it } from 'vitest';
import {
  getPulseDataGridAlignClass,
  getPulseDataGridWidthAttr,
} from '@/components/shared/pulseDataGridModel';

// Branch-coverage tests for the two still-uncovered model helpers in
// pulseDataGridModel.ts. Each `it` targets a specific branch (switch arm,
// regex arm, optional-chain arm, falsy guard) and asserts the concrete string
// or undefined return value — never truthiness alone.

describe('pulseDataGridModel.branchcov2', () => {
  describe('getPulseDataGridAlignClass', () => {
    it.each([
      { align: 'center', expected: 'text-center justify-center' },
      { align: 'right', expected: 'text-right justify-end' },
      { align: 'left', expected: 'text-left justify-start' },
    ] as const)(
      'returns the dedicated class tuple for the $align switch arm',
      ({ align, expected }) => {
        // Drives each named case label. 'left' falls through into the default
        // body but still lands on its own case label first.
        expect(getPulseDataGridAlignClass(align)).toBe(expected);
      },
    );

    it('falls through to the default arm when called with no argument (align === undefined)', () => {
      // The optional parameter is undefined -> no case matches -> default body.
      expect(getPulseDataGridAlignClass()).toBe('text-left justify-start');
    });

    it('hits the default arm when align is explicitly undefined', () => {
      // Same default arm via an explicit undefined argument.
      expect(getPulseDataGridAlignClass(undefined)).toBe('text-left justify-start');
    });

    it('hits the default arm for a malformed align value cast through unknown', () => {
      // Defensive branch: a runtime-only value outside the union still reaches
      // the default arm and must yield the left-aligned class tuple.
      type AlignArg = Parameters<typeof getPulseDataGridAlignClass>[0];
      const bogus = 'justify' as unknown as AlignArg;
      expect(getPulseDataGridAlignClass(bogus)).toBe('text-left justify-start');
    });
  });

  describe('getPulseDataGridWidthAttr', () => {
    describe('optional chain (width?.trim()) and falsy guard (!trimmed)', () => {
      it('returns undefined when width is undefined (?. short-circuit)', () => {
        // width?.trim() -> undefined; !trimmed truthy -> early return undefined.
        expect(getPulseDataGridWidthAttr(undefined)).toBeUndefined();
      });

      it('returns undefined when width is null cast to string (?. short-circuit on null)', () => {
        // null is nullish, so the ?. optional chain short-circuits to undefined
        // exactly like the undefined case.
        const nullable = null as unknown as string;
        expect(getPulseDataGridWidthAttr(nullable)).toBeUndefined();
      });

      it('returns undefined for an empty string (trim yields "" -> !trimmed truthy)', () => {
        // .trim() is actually invoked; result is "" which is falsy -> early return.
        expect(getPulseDataGridWidthAttr('')).toBeUndefined();
      });

      it('returns undefined for a whitespace-only string (trim yields "" -> !trimmed truthy)', () => {
        // Proves trim() runs before the guard: "   " is truthy pre-trim but
        // collapses to "" and is rejected.
        expect(getPulseDataGridWidthAttr('   ')).toBeUndefined();
      });
    });

    describe('px branch (/^\\d+(\\.\\d+)?px$/)', () => {
      it('strips the trailing "px" for an integer pixel value', () => {
        // First regex matches -> returns trimmed.slice(0, -2) == "100".
        expect(getPulseDataGridWidthAttr('100px')).toBe('100');
      });

      it('strips the trailing "px" for a decimal pixel value (\\d+\\.\\d+ optional group)', () => {
        // Exercises the (\\.\\d+)? optional group inside the px regex.
        expect(getPulseDataGridWidthAttr('10.5px')).toBe('10.5');
      });

      it('returns "0" for "0px" (zero is not collapsed by the guard)', () => {
        // "0px" is truthy so the guard is skipped; the px arm returns "0".
        expect(getPulseDataGridWidthAttr('0px')).toBe('0');
      });

      it('trims surrounding whitespace before applying the px regex', () => {
        // width?.trim() -> "100px" -> px arm -> "100". Confirms trim precedes
        // regex evaluation.
        expect(getPulseDataGridWidthAttr('  100px  ')).toBe('100');
      });
    });

    describe('percent branch (/^\\d+(\\.\\d+)?%$)', () => {
      it('returns the trimmed percent string verbatim for an integer percent', () => {
        // Left operand of the || matches -> returns trimmed unmodified.
        expect(getPulseDataGridWidthAttr('50%')).toBe('50%');
      });

      it('returns the trimmed percent string verbatim for a decimal percent (optional group)', () => {
        // Exercises the (\\.\\d+)? optional group inside the % regex.
        expect(getPulseDataGridWidthAttr('10.5%')).toBe('10.5%');
      });

      it('returns "0%" for "0%" (zero percent passes through)', () => {
        expect(getPulseDataGridWidthAttr('0%')).toBe('0%');
      });
    });

    describe('bare-number branch (/^\\d+(\\.\\d+)?$)', () => {
      it('returns the trimmed bare integer verbatim', () => {
        // % operand of the || fails, bare-number operand matches -> returns
        // trimmed unmodified.
        expect(getPulseDataGridWidthAttr('100')).toBe('100');
      });

      it('returns the trimmed bare decimal verbatim (optional group)', () => {
        // Exercises the (\\.\\d+)? optional group inside the bare-number regex.
        expect(getPulseDataGridWidthAttr('10.5')).toBe('10.5');
      });

      it('returns "0" for bare "0" (zero passes through, not collapsed)', () => {
        expect(getPulseDataGridWidthAttr('0')).toBe('0');
      });
    });

    describe('final fallback return undefined (no regex arm matches)', () => {
      it('returns undefined for a CSS keyword like "auto"', () => {
        // All three regexes fail -> final return undefined.
        expect(getPulseDataGridWidthAttr('auto')).toBeUndefined();
      });

      it('returns undefined for a non-px unit like "10em"', () => {
        // "10em" looks numeric but the unit is not px/% -> no match.
        expect(getPulseDataGridWidthAttr('10em')).toBeUndefined();
      });

      it('returns undefined for a near-miss px value "100p" (missing x)', () => {
        // Boundary: only the "px" suffix is rejected, proving the regex is not
        // a loose "ends with p" check.
        expect(getPulseDataGridWidthAttr('100p')).toBeUndefined();
      });

      it('returns undefined for a value with internal letters "5rem"', () => {
        expect(getPulseDataGridWidthAttr('5rem')).toBeUndefined();
      });
    });
  });
});
