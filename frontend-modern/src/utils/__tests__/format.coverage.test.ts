/**
 * Coverage tests for format utility functions.
 *
 * Fills boundary / edge-case gaps not already exercised by
 * `format.test.ts` (byte/duration formatters) and `formatExtra.test.ts`
 * (basic happy-path coverage for the exports below).
 */
import { describe, expect, it } from 'vitest';
import {
  ANOMALY_SEVERITY_CLASS,
  estimateTextWidth,
  formatAnomalyRatio,
  formatPowerOnHours,
  getShortImageName,
  normalizeDiskArray,
} from '@/utils/format';

describe('formatPowerOnHours - boundaries', () => {
  it('treats negative hours as a raw hour count (no clamping)', () => {
    expect(formatPowerOnHours(-5)).toBe('-5 hours');
    expect(formatPowerOnHours(-5, true)).toBe('-5h');
  });

  it('rounds days half-up at the 1.5-day boundary', () => {
    // 35h = 1.458 days -> rounds down to 1
    expect(formatPowerOnHours(35)).toBe('1 days');
    // 36h = exactly 1.5 days -> Math.round rounds half toward +Inf -> 2
    expect(formatPowerOnHours(36)).toBe('2 days');
    expect(formatPowerOnHours(36, true)).toBe('2d');
  });

  it('keeps the day bucket up to 8759 hours (just under a year)', () => {
    expect(formatPowerOnHours(8759)).toBe('365 days');
    expect(formatPowerOnHours(8759, true)).toBe('365d');
  });

  it('condenses mid-range day values', () => {
    // 100h / 24 = 4.1667 -> rounds to 4
    expect(formatPowerOnHours(100, true)).toBe('4d');
  });

  it('formats years with one decimal just above the 8760 threshold', () => {
    // 9000/8760 = 1.0274 -> toFixed(1) = "1.0"
    expect(formatPowerOnHours(9000)).toBe('1.0 years');
    expect(formatPowerOnHours(9000, true)).toBe('1.0y');
  });

  it('formats fractional years', () => {
    expect(formatPowerOnHours(13140)).toBe('1.5 years');
    expect(formatPowerOnHours(13140, true)).toBe('1.5y');
  });

  it('handles very large hour counts', () => {
    // 100000/8760 = 11.4155 -> "11.4"
    expect(formatPowerOnHours(100000)).toBe('11.4 years');
    expect(formatPowerOnHours(100000, true)).toBe('11.4y');
  });
});

describe('estimateTextWidth - edge inputs', () => {
  it('counts whitespace characters as full-width', () => {
    expect(estimateTextWidth(' ')).toBe(13.5);
    expect(estimateTextWidth('  ')).toBe(19);
  });

  it('counts control characters (tab, newline) as length 1', () => {
    expect(estimateTextWidth('\t')).toBe(13.5);
    expect(estimateTextWidth('\n')).toBe(13.5);
  });

  it('counts surrogate pairs as length 2 (UTF-16 code units)', () => {
    // "😀" is one code point but two UTF-16 code units -> length === 2
    expect('😀'.length).toBe(2);
    expect(estimateTextWidth('😀')).toBe(19);
  });

  it('scales linearly for very long strings', () => {
    expect(estimateTextWidth('a'.repeat(100))).toBe(558);
  });

  it('is strictly monotonic with respect to length', () => {
    const a = estimateTextWidth('a');
    const aa = estimateTextWidth('aa');
    const aaa = estimateTextWidth('aaa');
    expect(a).toBeLessThan(aa);
    expect(aa).toBeLessThan(aaa);
  });
});

describe('formatAnomalyRatio - boundaries', () => {
  it('returns the double-up arrow at exactly the 1.5 boundary', () => {
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 150 })).toBe('↑↑');
  });

  it('returns a single arrow just below 1.5', () => {
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 149.9 })).toBe('↑');
  });

  it('returns the double-up arrow just below 2', () => {
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 199.99 })).toBe('↑↑');
  });

  it('returns a single arrow at exactly 1x (no anomaly)', () => {
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 100 })).toBe('↑');
  });

  it('returns a single arrow for a decrease (current < baseline)', () => {
    // 50/100 = 0.5 -> below 1.5 threshold -> "↑" (no directional distinction)
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 50 })).toBe('↑');
  });

  it('returns a single arrow when current_value is 0', () => {
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 0 })).toBe('↑');
  });

  it('formats very large ratios with one decimal', () => {
    expect(formatAnomalyRatio({ baseline_mean: 1, current_value: 1000 })).toBe('1000.0x');
  });

  it('rounds ratio decimals (not truncates)', () => {
    // 7/3 = 2.3333 -> toFixed(1) = "2.3"
    expect(formatAnomalyRatio({ baseline_mean: 3, current_value: 7 })).toBe('2.3x');
  });

  it('treats negative/negative baselines as a positive ratio', () => {
    // -200 / -100 = 2 -> "2.0x"
    expect(formatAnomalyRatio({ baseline_mean: -100, current_value: -200 })).toBe('2.0x');
  });

  it('returns a single arrow for a negative ratio (opposite signs)', () => {
    // 100 / -100 = -1 -> below 1.5 -> "↑"
    expect(formatAnomalyRatio({ baseline_mean: -100, current_value: 100 })).toBe('↑');
    // -300 / 100 = -3 -> below 1.5 -> "↑" (magnitude ignored)
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: -300 })).toBe('↑');
  });

  it('does not short-circuit on a NaN baseline (falls through to "↑")', () => {
    // NaN === 0 is false, ratio is NaN, all comparisons false -> "↑"
    expect(formatAnomalyRatio({ baseline_mean: NaN, current_value: 100 })).toBe('↑');
  });
});

describe('ANOMALY_SEVERITY_CLASS - structural invariants', () => {
  it('exposes exactly the four severity levels', () => {
    expect(Object.keys(ANOMALY_SEVERITY_CLASS).sort()).toEqual([
      'critical',
      'high',
      'low',
      'medium',
    ]);
  });

  it('returns undefined for an unknown severity (no fallback class)', () => {
    expect(ANOMALY_SEVERITY_CLASS.nonexistent).toBeUndefined();
    expect(ANOMALY_SEVERITY_CLASS['made-up']).toBeUndefined();
  });

  it('uses valid Tailwind text-color classes for every severity', () => {
    for (const cls of Object.values(ANOMALY_SEVERITY_CLASS)) {
      expect(cls).toMatch(/^text-[a-z]+-400$/);
    }
  });
});

describe('getShortImageName - edge cases', () => {
  it('strips a registry that includes a port number', () => {
    expect(getShortImageName('localhost:5000/myapp:v1')).toBe('myapp:v1');
  });

  it('strips a registry with port and nested path', () => {
    expect(getShortImageName('registry.io:5000/org/app:tag')).toBe('app:tag');
  });

  it('returns a tagless image name unchanged', () => {
    expect(getShortImageName('nginx')).toBe('nginx');
  });

  it('preserves a bare digest-style token that has no slash or @', () => {
    // "sha256:abc123" has no '@' and no '/', so it is returned as-is.
    expect(getShortImageName('sha256:abc123')).toBe('sha256:abc123');
  });

  it('returns the leaf before the digest when an @ is present', () => {
    expect(getShortImageName('nginx@sha256:abc123')).toBe('nginx');
  });

  it('does not strip a trailing slash (falls back to the full string)', () => {
    // split('/') yields [..., '']; last element is '' (falsy) so the
    // `|| cleanImage` branch returns the untrimmed "foo/bar/".
    expect(getShortImageName('foo/bar/')).toBe('foo/bar/');
  });

  it('returns the em-dash when only a digest with empty image prefix is given', () => {
    // "@sha256:abc" -> cleanImage "" -> last part "" -> cleanImage "" -> "—"
    expect(getShortImageName('@sha256:abc')).toBe('—');
  });

  it('does not trim surrounding whitespace', () => {
    expect(getShortImageName(' nginx ')).toBe(' nginx ');
    expect(getShortImageName('   ')).toBe('   ');
  });
});

describe('normalizeDiskArray - edge cases', () => {
  it('uses an explicitly-provided free value over total-used', () => {
    const result = normalizeDiskArray([{ total: 1000, used: 500, free: 250 }]);
    expect(result?.[0].free).toBe(250);
    expect(result?.[0].usage).toBe(50);
  });

  it('clamps computed free to 0 when used exceeds total', () => {
    const result = normalizeDiskArray([{ total: 100, used: 200 }]);
    expect(result?.[0].free).toBe(0);
    expect(result?.[0].usage).toBe(200);
  });

  it('reports exactly 100% usage when used equals total', () => {
    const result = normalizeDiskArray([{ total: 100, used: 100 }]);
    expect(result?.[0].usage).toBe(100);
    expect(result?.[0].free).toBe(0);
  });

  it('does not clamp an explicitly-provided free that exceeds total', () => {
    const result = normalizeDiskArray([{ total: 1000, used: 500, free: 5000 }]);
    expect(result?.[0].free).toBe(5000);
  });

  it('gives filesystem precedence over type when both are present', () => {
    const result = normalizeDiskArray([{ filesystem: 'ext4', type: 'xfs' }]);
    expect(result?.[0].type).toBe('ext4');
  });

  it('leaves type undefined when neither filesystem nor type is provided', () => {
    const result = normalizeDiskArray([{ total: 100, used: 50 }]);
    expect(result?.[0].type).toBeUndefined();
  });

  it('preserves the device identifier in the output', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda1', total: 100 }]);
    expect(result?.[0].device).toBe('/dev/sda1');
  });

  it('trims mountpoint whitespace before matching the non-operational set', () => {
    // "  /boot/efi  " trims to "/boot/efi" which is filtered -> all filtered -> undefined
    expect(
      normalizeDiskArray([{ mountpoint: '  /boot/efi  ', total: 100, used: 50 }]),
    ).toBeUndefined();
  });

  it('filters mountpoints containing "System Reserved" as a substring', () => {
    expect(
      normalizeDiskArray([{ mountpoint: 'System Reserved Drive', total: 100 }]),
    ).toBeUndefined();
  });

  it('filters any mountpoint under the /System/Volumes/ prefix', () => {
    expect(
      normalizeDiskArray([{ mountpoint: '/System/Volumes/VM', total: 100 }]),
    ).toBeUndefined();
  });

  it('keeps disks whose mountpoint is undefined or empty', () => {
    const result = normalizeDiskArray([
      { total: 100, used: 50 }, // mountpoint undefined
      { mountpoint: '', total: 200, used: 100 },
    ]);
    expect(result).toHaveLength(2);
  });

  it('does not trim the mountpoint in the returned object (only the filter trims)', () => {
    // "/mnt " trims to "/mnt" (operational) so it is kept, but the output
    // preserves the original trailing space.
    const result = normalizeDiskArray([{ mountpoint: '/mnt ', total: 100 }]);
    expect(result?.[0].mountpoint).toBe('/mnt ');
  });

  it('keeps only operational disks when mixed with filtered ones', () => {
    const result = normalizeDiskArray([
      { mountpoint: '/etc/pve', total: 100, used: 10 },
      { mountpoint: '/mnt/data', total: 200, used: 50 },
      { mountpoint: '/boot/firmware', total: 100, used: 10 },
    ]);
    expect(result?.map((d) => d.mountpoint)).toEqual(['/mnt/data']);
  });
});
