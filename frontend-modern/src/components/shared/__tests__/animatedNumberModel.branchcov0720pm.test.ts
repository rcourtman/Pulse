import { describe, expect, it } from 'vitest';

import {
  easeAnimatedNumberProgress,
  formatAnimatedInteger,
  sanitizeAnimatedNumberValue,
} from '@/components/shared/animatedNumberModel';

// Every assertion below is a hand-computed expected value against the real
// runtime output of the three exported helpers in animatedNumberModel.ts
// (no `?raw` source-string reads, no snapshots, no constant-equals-itself
// tautologies). See src/components/shared/animatedNumberModel.ts.

describe('animatedNumberModel.branchcov0720pm', () => {
  describe('sanitizeAnimatedNumberValue', () => {
    it('returns 0 for NaN (!Number.isFinite(value) true arm)', () => {
      // NaN is not finite -> early-return 0.
      expect(sanitizeAnimatedNumberValue(NaN)).toBe(0);
    });

    it('returns 0 for +Infinity (!Number.isFinite(value) true arm)', () => {
      expect(sanitizeAnimatedNumberValue(Infinity)).toBe(0);
    });

    it('returns 0 for -Infinity (!Number.isFinite(value) true arm)', () => {
      expect(sanitizeAnimatedNumberValue(-Infinity)).toBe(0);
    });

    it('returns a positive finite value verbatim (happy path, identity arm)', () => {
      // Number.isFinite(42) -> true -> fall through to `return value`.
      expect(sanitizeAnimatedNumberValue(42)).toBe(42);
    });

    it('returns a negative finite value verbatim (happy path, identity arm)', () => {
      expect(sanitizeAnimatedNumberValue(-7)).toBe(-7);
    });

    it('returns 0 verbatim at the zero boundary (happy path, identity arm)', () => {
      // 0 is finite, so the guard is skipped — the value flows through unchanged.
      expect(sanitizeAnimatedNumberValue(0)).toBe(0);
    });

    it('returns a fractional finite value verbatim (happy path, identity arm)', () => {
      expect(sanitizeAnimatedNumberValue(3.14)).toBe(3.14);
    });
  });

  describe('formatAnimatedInteger', () => {
    // `String(Math.round(sanitizeAnimatedNumberValue(value)))` — composition of
    // sanitize (guard) + Math.round + String(). Each case locks a distinct arm
    // of that pipeline by asserting on the observable string output.

    it('formats a representative positive integer as its decimal string', () => {
      // sanitize(42) -> 42; Math.round(42) -> 42; String(42) -> '42'.
      expect(formatAnimatedInteger(42)).toBe('42');
    });

    it('rounds a fractional value up to the nearest integer before formatting', () => {
      // Math.round(42.7) -> 43.
      expect(formatAnimatedInteger(42.7)).toBe('43');
    });

    it('rounds a fractional value down to the nearest integer before formatting', () => {
      // Math.round(42.4) -> 42.
      expect(formatAnimatedInteger(42.4)).toBe('42');
    });

    it('formats a negative integer (preserves the leading minus)', () => {
      expect(formatAnimatedInteger(-5)).toBe('-5');
    });

    it('formats 0 as "0" (zero boundary, no rounding artefacts)', () => {
      expect(formatAnimatedInteger(0)).toBe('0');
    });

    it('formats a value just below .5 by rounding down (Math.round half-up boundary)', () => {
      // Math.round(0.4999) -> 0.
      expect(formatAnimatedInteger(0.4999)).toBe('0');
    });

    it('formats a value at exactly .5 by rounding up (Math.round half-up boundary)', () => {
      // Math.round(0.5) -> 1.
      expect(formatAnimatedInteger(0.5)).toBe('1');
    });

    it('sanitizes NaN to "0" via the sanitize guard before rounding', () => {
      // sanitize(NaN) -> 0; Math.round(0) -> 0; String(0) -> '0'.
      expect(formatAnimatedInteger(NaN)).toBe('0');
    });

    it('sanitizes +Infinity to "0" via the sanitize guard before rounding', () => {
      expect(formatAnimatedInteger(Infinity)).toBe('0');
    });

    it('sanitizes -Infinity to "0" via the sanitize guard before rounding', () => {
      expect(formatAnimatedInteger(-Infinity)).toBe('0');
    });
  });

  describe('easeAnimatedNumberProgress', () => {
    // `Math.max(0, Math.min(progress, 1))` clamps to [0, 1]; then cubic ease-out
    // `1 - (1 - bounded) ^ 3`. Each case drives a distinct clamp arm and asserts
    // a hand-computed expected numeric output.

    it('clamps progress < 0 to 0 and yields the eased value 0 (max-wins arm)', () => {
      // bounded = max(0, min(-0.5, 1)) = max(0, -0.5) = 0
      // eased = 1 - (1 - 0) ^ 3 = 1 - 1 = 0
      expect(easeAnimatedNumberProgress(-0.5)).toBe(0);
    });

    it('clamps progress > 1 to 1 and yields the eased value 1 (min-wins arm)', () => {
      // bounded = max(0, min(1.5, 1)) = max(0, 1) = 1
      // eased = 1 - (1 - 1) ^ 3 = 1 - 0 = 1
      expect(easeAnimatedNumberProgress(1.5)).toBe(1);
    });

    it('passes an in-range progress through both clamps untouched (identity arm)', () => {
      // bounded = max(0, min(0.5, 1)) = max(0, 0.5) = 0.5
      // eased = 1 - (1 - 0.5) ^ 3 = 1 - 0.125 = 0.875
      expect(easeAnimatedNumberProgress(0.5)).toBe(0.875);
    });

    it('eases the quarter-progress point with the cubic curve (identity arm, second sample)', () => {
      // bounded = 0.25 -> eased = 1 - (0.75) ^ 3 = 1 - 0.421875 = 0.578125
      expect(easeAnimatedNumberProgress(0.25)).toBe(0.578125);
    });

    it('returns 0 at the progress === 0 lower boundary (no clamp needed)', () => {
      // bounded = 0 -> eased = 1 - 1 = 0.
      expect(easeAnimatedNumberProgress(0)).toBe(0);
    });

    it('returns 1 at the progress === 1 upper boundary (no clamp needed)', () => {
      // bounded = 1 -> eased = 1 - 0 = 1.
      expect(easeAnimatedNumberProgress(1)).toBe(1);
    });

    it('clamps a deeply-negative progress to 0 (max-wins arm, far side)', () => {
      // bounded = max(0, min(-100, 1)) = 0 -> eased = 0.
      expect(easeAnimatedNumberProgress(-100)).toBe(0);
    });

    it('clamps a large overflow progress to 1 (min-wins arm, far side)', () => {
      // bounded = max(0, min(100, 1)) = 1 -> eased = 1.
      expect(easeAnimatedNumberProgress(100)).toBe(1);
    });
  });
});
