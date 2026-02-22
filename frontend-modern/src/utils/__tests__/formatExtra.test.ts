import { describe, expect, it } from 'vitest';
import {
  formatPowerOnHours,
  estimateTextWidth,
  formatAnomalyRatio,
  getShortImageName,
  normalizeDiskArray,
  ANOMALY_SEVERITY_CLASS,
} from '@/utils/format';

describe('formatPowerOnHours', () => {
  it('returns hours for less than 24 hours', () => {
    expect(formatPowerOnHours(0)).toBe('0 hours');
    expect(formatPowerOnHours(1)).toBe('1 hours');
    expect(formatPowerOnHours(23)).toBe('23 hours');
  });

  it('returns days for 24+ hours', () => {
    expect(formatPowerOnHours(24)).toBe('1 days');
    expect(formatPowerOnHours(48)).toBe('2 days');
    expect(formatPowerOnHours(168)).toBe('7 days');
  });

  it('returns years for 8760+ hours', () => {
    expect(formatPowerOnHours(8760)).toBe('1.0 years');
    expect(formatPowerOnHours(17520)).toBe('2.0 years');
    expect(formatPowerOnHours(43800)).toBe('5.0 years');
  });

  it('returns condensed format when condensed is true', () => {
    expect(formatPowerOnHours(0, true)).toBe('0h');
    expect(formatPowerOnHours(24, true)).toBe('1d');
    expect(formatPowerOnHours(8760, true)).toBe('1.0y');
  });
});

describe('estimateTextWidth', () => {
  it('estimates width based on character count', () => {
    expect(estimateTextWidth('')).toBe(8);
    expect(estimateTextWidth('a')).toBe(13.5);
    expect(estimateTextWidth('abc')).toBe(24.5);
    expect(estimateTextWidth('hello')).toBe(35.5); // 5 chars = 27.5 + 8
    expect(estimateTextWidth('hello world')).toBe(68.5); // 11 chars = 60.5 + 8
  });
});

describe('formatAnomalyRatio', () => {
  it('returns null for null/undefined input', () => {
    expect(formatAnomalyRatio(null)).toBeNull();
    expect(formatAnomalyRatio(undefined)).toBeNull();
  });

  it('returns null when baseline_mean is 0', () => {
    expect(formatAnomalyRatio({ baseline_mean: 0, current_value: 100 })).toBeNull();
  });

  it('returns 2x+ ratio as formatted number', () => {
    expect(formatAnomalyRatio({ baseline_mean: 50, current_value: 100 })).toBe('2.0x');
    expect(formatAnomalyRatio({ baseline_mean: 25, current_value: 100 })).toBe('4.0x');
  });

  it('returns ↑↑ for 1.5x-2x ratio', () => {
    expect(formatAnomalyRatio({ baseline_mean: 50, current_value: 90 })).toBe('↑↑');
    expect(formatAnomalyRatio({ baseline_mean: 40, current_value: 70 })).toBe('↑↑');
  });

  it('returns ↑ for less than 1.5x ratio', () => {
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 140 })).toBe('↑');
    expect(formatAnomalyRatio({ baseline_mean: 100, current_value: 120 })).toBe('↑');
  });
});

describe('ANOMALY_SEVERITY_CLASS', () => {
  it('has critical class', () => {
    expect(ANOMALY_SEVERITY_CLASS.critical).toBe('text-red-400');
  });

  it('has high class', () => {
    expect(ANOMALY_SEVERITY_CLASS.high).toBe('text-orange-400');
  });

  it('has medium class', () => {
    expect(ANOMALY_SEVERITY_CLASS.medium).toBe('text-yellow-400');
  });

  it('has low class', () => {
    expect(ANOMALY_SEVERITY_CLASS.low).toBe('text-blue-400');
  });
});

describe('getShortImageName', () => {
  it('returns dash for undefined', () => {
    expect(getShortImageName(undefined)).toBe('—');
  });

  it('returns dash for empty string', () => {
    expect(getShortImageName('')).toBe('—');
  });

  it('returns full name for simple image', () => {
    expect(getShortImageName('nginx:latest')).toBe('nginx:latest');
  });

  it('returns last two components for registry URLs', () => {
    expect(getShortImageName('ghcr.io/owner/image:tag')).toBe('owner/image:tag');
    expect(getShortImageName('docker.io/library/nginx:latest')).toBe('library/nginx:latest');
  });

  it('strips sha256 digest', () => {
    expect(getShortImageName('nginx:latest@sha256:abc123')).toBe('nginx:latest');
  });

  it('handles complex registry paths', () => {
    expect(getShortImageName('registry.example.com/foo/bar/myapp:v1.0')).toBe('bar/myapp:v1.0');
  });
});

describe('normalizeDiskArray', () => {
  it('returns undefined for null/undefined input', () => {
    expect(normalizeDiskArray(undefined)).toBeUndefined();
    expect(normalizeDiskArray(null)).toBeUndefined();
  });

  it('returns undefined for empty array', () => {
    expect(normalizeDiskArray([])).toBeUndefined();
  });

  it('normalizes disk with used and total', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda', total: 1000, used: 500 }]);
    expect(result).toHaveLength(1);
    expect(result?.[0].total).toBe(1000);
    expect(result?.[0].used).toBe(500);
    expect(result?.[0].free).toBe(500);
    expect(result?.[0].usage).toBe(50);
  });

  it('calculates free from total and used when free is missing', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda', total: 1000, used: 300 }]);
    expect(result?.[0].free).toBe(700);
  });

  it('handles missing free and calculates correctly', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda', total: 1000 }]);
    expect(result?.[0].free).toBe(1000);
    expect(result?.[0].usage).toBe(0);
  });

  it('handles zero total gracefully', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda', total: 0, used: 0 }]);
    expect(result?.[0].free).toBe(0);
    expect(result?.[0].usage).toBe(0);
  });

  it('uses filesystem type when available', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda', filesystem: 'ext4' }]);
    expect(result?.[0].type).toBe('ext4');
  });

  it('uses type as fallback when filesystem missing', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda', type: 'xfs' }]);
    expect(result?.[0].type).toBe('xfs');
  });

  it('preserves mountpoint', () => {
    const result = normalizeDiskArray([{ device: '/dev/sda', mountpoint: '/home' }]);
    expect(result?.[0].mountpoint).toBe('/home');
  });
});
