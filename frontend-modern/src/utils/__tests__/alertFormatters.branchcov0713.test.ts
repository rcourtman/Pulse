import { describe, expect, it } from 'vitest';
import { getAlertUnit, formatAlertValue, formatAlertThreshold } from '@/utils/alertFormatters';

// Branch-coverage companion to alertFormatters.test.ts. The sibling suite
// already exercises the temperature/temp arms, the undefined/''/cpu/memory/disk
// default-% arms, and the NaN/Infinity -> N/A guard. This file targets arms and
// boundary values it leaves open.
//
// Two SUSPECTED SOURCE BUGS are exercised here by asserting their CURRENT
// behaviour (NOT fixed; new test file only — see GLM_REPORT.md):
//
// BUG #1 (critical): the throughput branch in getAlertUnit is DEAD CODE.
//   alertFormatters.ts lowercases the metricType before lookup:
//       const typeLower = metricType.toLowerCase();
//   but THROUGHPUT_METRIC_TYPES holds camelCase keys
//   ('diskRead','diskWrite','networkIn','networkOut'). After lowercasing,
//   'diskread' / 'networkin' etc. are NOT in the set, so the
//   `return ' MB/s'` line is unreachable from any input and every throughput
//   metric falls through to the default '%'. (TEMPERATURE works only because
//   its set keys are already lowercase.)
//
// BUG #2 (minor): formatAlertThreshold only rejects NaN, not all non-finite
//   values. Unlike formatAlertValue (which guards with !Number.isFinite),
//   threshold guards with Number.isNaN, so +Infinity passes both guards and is
//   interpolated verbatim as 'Infinity%'.

describe('alertFormatters — branch coverage (batch 0713)', () => {
  describe('getAlertUnit — null falsy arm, dead throughput branch, default arm', () => {
    it('treats null metricType as falsy and returns % via the !metricType guard', () => {
      // Declared type is `string | undefined`; null violates it, so cast through
      // unknown to keep strict TS clean. `!null` is true -> early '%'.
      const mt = null as unknown as Parameters<typeof getAlertUnit>[0];
      expect(getAlertUnit(mt)).toBe('%');
    });

    it('exact throughput set keys fall through to % — BUG #1 (dead branch)', () => {
      // Each value IS a member of THROUGHPUT_METRIC_TYPES, but because the source
      // lowercases the input and the set keys are camelCase, the lookup misses
      // and execution reaches the final `return '%'` instead of `return ' MB/s'`.
      expect(getAlertUnit('diskRead')).toBe('%');
      expect(getAlertUnit('diskWrite')).toBe('%');
      expect(getAlertUnit('networkIn')).toBe('%');
      expect(getAlertUnit('networkOut')).toBe('%');
    });

    it('upper / mixed / lower variants of throughput types also miss the set', () => {
      // Exercises the THROUGHPUT_METRIC_TYPES.has(typeLower) check returning
      // false for every case variant (the case-folding cannot reconstruct the
      // camelCase keys), then the final default '%'.
      expect(getAlertUnit('DISKREAD')).toBe('%');
      expect(getAlertUnit('NetworkIn')).toBe('%');
      expect(getAlertUnit('networkout')).toBe('%');
    });

    it('falls through to % for unknown and whitespace-only metric types', () => {
      // 'latency' is unknown to both sets; '  ' is a truthy non-empty string so
      // it skips the !metricType guard, lowercases to itself, misses both sets,
      // and hits the final `return '%'`.
      expect(getAlertUnit('latency')).toBe('%');
      expect(getAlertUnit('  ')).toBe('%');
    });
  });

  describe('formatAlertValue — finite boundaries, negatives, throughput propagation', () => {
    it('formats a finite 0 value (not N/A) — boundary for the isFinite guard', () => {
      // 0 is finite, so it must NOT take the 'N/A' arm; it formats normally.
      expect(formatAlertValue(0)).toBe('0.0%');
      expect(formatAlertValue(0, 'temperature')).toMatch(/^0\.0°[CF]$/);
    });

    it('formats a negative finite value rather than returning N/A', () => {
      // Only non-finite (NaN/+/-Infinity) hit N/A; a real negative formats.
      expect(formatAlertValue(-3.25)).toBe('-3.3%');
    });

    it('propagates the dead throughput branch to % through formatAlertValue — BUG #1', () => {
      // getAlertUnit('diskRead') returns '%' (see BUG #1), so the formatted
      // value carries '%' rather than ' MB/s'.
      expect(formatAlertValue(100, 'diskRead')).toBe('100.0%');
      expect(formatAlertValue(42.5, 'networkOut')).toBe('42.5%');
    });

    it('still returns N/A when the value is non-finite even with a throughput unit', () => {
      // The isFinite guard short-circuits before getAlertUnit is consulted, so
      // the (dead) throughput branch is never reached for non-finite values.
      expect(formatAlertValue(NaN, 'diskRead')).toBe('N/A');
      expect(formatAlertValue(Infinity, 'networkIn')).toBe('N/A');
      expect(formatAlertValue(-Infinity, 'diskWrite')).toBe('N/A');
    });
  });

  describe('formatAlertThreshold — non-finite arms, boundary, throughput propagation', () => {
    it('classifies -Infinity as Disabled via the value <= 0 arm', () => {
      // -Infinity passes the undefined/NaN guard (it is not NaN), then satisfies
      // `value <= 0`, so it is 'Disabled'. This contrasts with formatAlertValue,
      // which would return 'N/A' via !Number.isFinite — see BUG #2.
      expect(formatAlertThreshold(-Infinity, 'diskWrite')).toBe('Disabled');
    });

    it('formats a tiny positive threshold (boundary just above the <= 0 arm)', () => {
      // 0.001 > 0, so it escapes both guards and reaches the raw interpolation.
      expect(formatAlertThreshold(0.001)).toBe('0.001%');
    });

    it('propagates the dead throughput branch to % through formatAlertThreshold — BUG #1', () => {
      // formatAlertThreshold does not call toFixed; it interpolates the raw
      // number, so an integer renders without a trailing decimal. The unit is
      // '%' (not ' MB/s') because of BUG #1.
      expect(formatAlertThreshold(100, 'diskRead')).toBe('100%');
      expect(formatAlertThreshold(250.5, 'networkOut')).toBe('250.5%');
    });

    it('renders a positive Infinity threshold literally — BUG #2 (isFinite NOT guarded)', () => {
      // Unlike formatAlertValue, threshold only rejects NaN, so +Infinity is
      // not NaN, is not <= 0, and is interpolated verbatim. Asserting CURRENT
      // (buggy) behaviour so the suite stays green; GLM_REPORT.md documents it.
      expect(formatAlertThreshold(Infinity, 'diskRead')).toBe('Infinity%');
      expect(formatAlertThreshold(Infinity)).toBe('Infinity%');
    });
  });
});
