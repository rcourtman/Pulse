/**
 * Tests for format utility functions
 */
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import {
  formatBytes,
  formatSpeed,
  formatCompactSpeed,
  formatObservedSpeed,
  formatPercent,
  formatCpuPercent,
  formatNumber,
  formatUptime,
  formatAbsoluteTime,
  formatRelativeTime,
  formatTimeUntil,
  getBackupInfo,
} from '@/utils/format';

describe('formatBytes', () => {
  it('formats bytes with auto precision (small values get 2 decimals)', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(512)).toBe('512 B'); // >= 100, 0 decimals
    expect(formatBytes(1024)).toBe('1.00 KB'); // < 10, 2 decimals
    expect(formatBytes(1536)).toBe('1.50 KB'); // < 10, 2 decimals
  });

  it('formats kilobytes with auto precision', () => {
    expect(formatBytes(1024 * 1024)).toBe('1.00 MB'); // < 10, 2 decimals
    expect(formatBytes(1536 * 1024)).toBe('1.50 MB'); // < 10, 2 decimals
    expect(formatBytes(50 * 1024 * 1024)).toBe('50.0 MB'); // 10-100, 1 decimal
    expect(formatBytes(256 * 1024 * 1024)).toBe('256 MB'); // >= 100, 0 decimals
  });

  it('formats megabytes with auto precision', () => {
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1.00 GB'); // < 10, 2 decimals
    expect(formatBytes(1.5 * 1024 * 1024 * 1024)).toBe('1.50 GB'); // < 10, 2 decimals
    expect(formatBytes(45 * 1024 * 1024 * 1024)).toBe('45.0 GB'); // 10-100, 1 decimal
  });

  it('formats gigabytes with auto precision', () => {
    expect(formatBytes(1024 * 1024 * 1024 * 1024)).toBe('1.00 TB'); // < 10, 2 decimals
  });

  it('handles negative values', () => {
    expect(formatBytes(-1024)).toBe('0 B');
  });

  it('handles explicit decimal places', () => {
    expect(formatBytes(1536 * 1024, 2)).toBe('1.50 MB');
    expect(formatBytes(1536 * 1024, 0)).toBe('2 MB');
    expect(formatBytes(1536 * 1024, 1)).toBe('1.5 MB');
  });
});

describe('formatSpeed', () => {
  it('formats speed with auto precision', () => {
    expect(formatSpeed(0)).toBe('0 B/s');
    expect(formatSpeed(1024)).toBe('1.00 KB/s'); // < 10, 2 decimals
    expect(formatSpeed(1024 * 1024)).toBe('1.00 MB/s'); // < 10, 2 decimals
    expect(formatSpeed(50 * 1024 * 1024)).toBe('50.0 MB/s'); // 10-100, 1 decimal
  });

  it('handles negative values', () => {
    expect(formatSpeed(-1024)).toBe('0 B/s');
  });

  it('handles explicit decimal places', () => {
    expect(formatSpeed(1024, 0)).toBe('1 KB/s');
    expect(formatSpeed(1024 * 1024, 1)).toBe('1.0 MB/s');
  });
});

describe('formatCompactSpeed', () => {
  it.each([
    [0, '0'],
    [-1024, '0'],
    [Number.NaN, '0'],
    [512, '<1K'],
    [1024, '1.0K'],
    [3.32 * 1024 * 1024, '3.3M'],
    [9.94 * 1024, '9.9K'],
    [9.96 * 1024, '10K'],
    [512 * 1024, '512K'],
    [1023.6 * 1024, '1.0M'],
    [2.5 * 1024 ** 3, '2.5G'],
    [2 ** 64, '16E'],
    [Number.MAX_VALUE, '>1023E'],
  ])('formats %s B/s as %s', (value, expected) => {
    expect(formatCompactSpeed(value)).toBe(expected);
  });
});

describe('formatCpuPercent', () => {
  it.each([
    [0, '0%'],
    [Number.MIN_VALUE, '<0.1%'],
    [0.026, '<0.1%'],
    [0.09999, '<0.1%'],
    [0.1, '0.1%'],
    [0.17, '0.2%'],
    [0.49, '0.5%'],
    [1.04, '1%'],
    [3.04, '3%'],
    [9.4, '9.4%'],
    [10, '10%'],
    [50.5, '51%'],
    [100, '100%'],
    [NaN, '—'],
    [Infinity, '—'],
  ])('formats %s as %s without inventing a zero', (value, label) => {
    expect(formatCpuPercent(value)).toBe(label);
  });
});

describe('formatPercent', () => {
  it('formats percentages correctly', () => {
    expect(formatPercent(0)).toBe('0%');
    expect(formatPercent(50)).toBe('50%');
    expect(formatPercent(100)).toBe('100%');
  });

  it('rounds to nearest integer', () => {
    expect(formatPercent(50.4)).toBe('50%');
    expect(formatPercent(50.5)).toBe('51%');
    expect(formatPercent(50.6)).toBe('51%');
  });

  it('handles very small values', () => {
    expect(formatPercent(0.1)).toBe('0%');
    expect(formatPercent(0.49)).toBe('0%');
  });

  it('handles non-finite values', () => {
    expect(formatPercent(NaN)).toBe('0%');
    expect(formatPercent(Infinity)).toBe('0%');
    expect(formatPercent(-Infinity)).toBe('0%');
  });
});

describe('formatNumber', () => {
  it('formats numbers with locale separators', () => {
    expect(formatNumber(0)).toBe('0');
    expect(formatNumber(1000)).toMatch(/1[,.]000/); // Locale-dependent
  });

  it('handles non-finite values', () => {
    expect(formatNumber(NaN)).toBe('0');
    expect(formatNumber(Infinity)).toBe('0');
  });
});

describe('formatUptime', () => {
  it('formats seconds correctly', () => {
    expect(formatUptime(0)).toBe('0s');
    expect(formatUptime(30)).toBe('0m');
  });

  it('formats minutes correctly', () => {
    expect(formatUptime(60)).toBe('1m');
    expect(formatUptime(120)).toBe('2m');
    expect(formatUptime(3599)).toBe('59m');
  });

  it('formats hours correctly', () => {
    expect(formatUptime(3600)).toBe('1h 0m');
    expect(formatUptime(7200)).toBe('2h 0m');
    expect(formatUptime(7230)).toBe('2h 0m'); // 2 hours, 0 minutes, 30 seconds
    expect(formatUptime(7260)).toBe('2h 1m');
  });

  it('formats days correctly', () => {
    expect(formatUptime(86400)).toBe('1d 0h');
    expect(formatUptime(86400 + 3600)).toBe('1d 1h');
    expect(formatUptime(86400 * 7 + 3600 * 12)).toBe('7d 12h');
  });

  it('uses condensed format when requested', () => {
    expect(formatUptime(86400 + 3600, true)).toBe('1d');
    expect(formatUptime(3600 + 60, true)).toBe('1h');
    expect(formatUptime(60, true)).toBe('1m');
  });

  it('handles negative values', () => {
    expect(formatUptime(-100)).toBe('0s');
  });
});

describe('formatAbsoluteTime', () => {
  it('returns empty string for falsy input', () => {
    expect(formatAbsoluteTime(0)).toBe('');
  });

  it('formats timestamp correctly', () => {
    // Create a known date: March 15, 2024 at 14:30
    const date = new Date(2024, 2, 15, 14, 30); // Month is 0-indexed
    const timestamp = date.getTime();
    const result = formatAbsoluteTime(timestamp);
    expect(result).toBe('15 Mar 14:30');
  });

  it('pads hours and minutes with zeros', () => {
    const date = new Date(2024, 0, 5, 9, 5); // Jan 5 at 09:05
    const result = formatAbsoluteTime(date.getTime());
    expect(result).toBe('5 Jan 09:05');
  });
});

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-03-15T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns empty string for falsy input', () => {
    expect(formatRelativeTime(0)).toBe('');
  });

  it('formats seconds ago', () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 30 * 1000)).toBe('30s ago');
    expect(formatRelativeTime(now - 1 * 1000)).toBe('1s ago');
  });

  it('formats minutes ago', () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 60 * 1000)).toBe('1 min ago');
    expect(formatRelativeTime(now - 5 * 60 * 1000)).toBe('5 mins ago');
    expect(formatRelativeTime(now - 59 * 60 * 1000)).toBe('59 mins ago');
  });

  it('formats hours ago', () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 60 * 60 * 1000)).toBe('1 hour ago');
    expect(formatRelativeTime(now - 5 * 60 * 60 * 1000)).toBe('5 hours ago');
    expect(formatRelativeTime(now - 23 * 60 * 60 * 1000)).toBe('23 hours ago');
  });

  it('formats days ago', () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 24 * 60 * 60 * 1000)).toBe('1 day ago');
    expect(formatRelativeTime(now - 7 * 24 * 60 * 60 * 1000)).toBe('7 days ago');
  });

  it('formats months ago', () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 30 * 24 * 60 * 60 * 1000)).toBe('1 month ago');
    expect(formatRelativeTime(now - 60 * 24 * 60 * 60 * 1000)).toBe('2 months ago');
  });

  it('formats years ago', () => {
    const now = Date.now();
    expect(formatRelativeTime(now - 365 * 24 * 60 * 60 * 1000)).toBe('1 year ago');
    expect(formatRelativeTime(now - 2 * 365 * 24 * 60 * 60 * 1000)).toBe('2 years ago');
  });

  it('handles future timestamps', () => {
    const now = Date.now();
    expect(formatRelativeTime(now + 60 * 1000)).toBe('0s ago');
  });
});

describe('formatTimeUntil', () => {
  const now = Date.parse('2024-03-15T12:00:00Z');
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;

  it('returns empty text for falsy or unparseable input', () => {
    expect(formatTimeUntil(undefined, { now })).toBe('');
    expect(formatTimeUntil(0, { now, emptyText: '-' })).toBe('-');
    expect(formatTimeUntil('not a date', { now, emptyText: '-' })).toBe('-');
  });

  it('counts down to a future time instead of reading it as just now', () => {
    expect(formatTimeUntil(now + 30 * 1000, { now })).toBe('in under a minute');
    expect(formatTimeUntil(now + 5 * minute, { now, compact: true })).toBe('in 5m');
    expect(formatTimeUntil(now + 3 * hour, { now, compact: true })).toBe('in 3h');
    expect(formatTimeUntil(now + 6 * day, { now, compact: true })).toBe('in 6d');
    expect(formatTimeUntil(now + 45 * day, { now, compact: true })).toBe('in 45d');
  });

  it('spells units out in the long form', () => {
    expect(formatTimeUntil(now + minute, { now })).toBe('in 1 min');
    expect(formatTimeUntil(now + 5 * minute, { now })).toBe('in 5 mins');
    expect(formatTimeUntil(now + hour, { now })).toBe('in 1 hour');
    expect(formatTimeUntil(now + 5 * hour, { now })).toBe('in 5 hours');
    expect(formatTimeUntil(now + day, { now })).toBe('in 1 day');
    expect(formatTimeUntil(now + 7 * day, { now })).toBe('in 7 days');
    expect(formatTimeUntil(now + 60 * day, { now })).toBe('in 2 months');
    expect(formatTimeUntil(now + 365 * day, { now })).toBe('in 1 year');
  });

  it('rounds to the nearest unit, so a duration just set reads as chosen', () => {
    // A 24-hour, 1-hour and 7-day suppression set a few seconds ago.
    expect(formatTimeUntil(now + day - 5000, { now, compact: true })).toBe('in 1d');
    expect(formatTimeUntil(now + hour - 5000, { now, compact: true })).toBe('in 1h');
    expect(formatTimeUntil(now + 7 * day - 5000, { now, compact: true })).toBe('in 7d');
    expect(formatTimeUntil(now + 2 * hour + 10 * minute, { now, compact: true })).toBe('in 2h');
    expect(formatTimeUntil(now + 2 * hour + 50 * minute, { now, compact: true })).toBe('in 3h');
    // Months round from the remaining time itself, not from rounded days.
    expect(formatTimeUntil(now + 44 * day + 12 * hour, { now })).toBe('in 1 month');
    expect(formatTimeUntil(now + 45 * day + hour, { now })).toBe('in 2 months');
  });

  it('reads as due once the time has arrived or passed', () => {
    expect(formatTimeUntil(now, { now })).toBe('now');
    expect(formatTimeUntil(now - hour, { now, compact: true })).toBe('now');
    expect(formatTimeUntil(now - hour, { now, dueText: 'overdue' })).toBe('overdue');
  });

  it('accepts ISO strings and Date objects and defaults now to the current time', () => {
    vi.useFakeTimers();
    vi.setSystemTime(now);
    try {
      expect(formatTimeUntil('2024-03-15T15:00:00Z', { compact: true })).toBe('in 3h');
      expect(formatTimeUntil(new Date(now + 10 * minute), { compact: true })).toBe('in 10m');
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('getBackupInfo', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-03-15T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns never status for null/undefined input', () => {
    expect(getBackupInfo(null)).toEqual({
      status: 'never',
      ageMs: null,
      ageFormatted: 'Never',
    });
    expect(getBackupInfo(undefined)).toEqual({
      status: 'never',
      ageMs: null,
      ageFormatted: 'Never',
    });
  });

  it('keeps invalid timestamps distinct from an absent backup', () => {
    expect(getBackupInfo('invalid')).toEqual({
      status: 'unknown',
      ageMs: null,
      ageFormatted: 'Backup time unavailable: invalid timestamp.',
    });
    expect(getBackupInfo(0)).toEqual({
      status: 'never',
      ageMs: null,
      ageFormatted: 'Never',
    });
  });

  it('returns fresh status for backup within 24 hours', () => {
    const now = Date.now();
    const oneHourAgo = now - 60 * 60 * 1000;
    const result = getBackupInfo(oneHourAgo);
    expect(result.status).toBe('fresh');
    expect(result.ageMs).toBe(60 * 60 * 1000);
    expect(result.ageFormatted).toBe('1 hour ago');
  });

  it('returns stale status for backup between 24-72 hours', () => {
    const now = Date.now();
    const twoDaysAgo = now - 48 * 60 * 60 * 1000;
    const result = getBackupInfo(twoDaysAgo);
    expect(result.status).toBe('stale');
    expect(result.ageFormatted).toBe('2 days ago');
  });

  it('returns overdue status for backup older than 72 hours', () => {
    const now = Date.now();
    const fourDaysAgo = now - 4 * 24 * 60 * 60 * 1000;
    const result = getBackupInfo(fourDaysAgo);
    expect(result.status).toBe('overdue');
    expect(result.ageFormatted).toBe('4 days ago');
  });

  it('handles ISO date string input', () => {
    const now = Date.now();
    const oneHourAgo = new Date(now - 60 * 60 * 1000).toISOString();
    const result = getBackupInfo(oneHourAgo);
    expect(result.status).toBe('fresh');
  });

  it('handles numeric timestamp input', () => {
    const now = Date.now();
    const oneHourAgo = now - 60 * 60 * 1000;
    const result = getBackupInfo(oneHourAgo);
    expect(result.status).toBe('fresh');
  });

  describe('threshold boundaries', () => {
    it('fresh at exactly 24 hours', () => {
      const now = Date.now();
      const exactly24Hours = now - 24 * 60 * 60 * 1000;
      const result = getBackupInfo(exactly24Hours);
      expect(result.status).toBe('fresh');
    });

    it('stale just after 24 hours', () => {
      const now = Date.now();
      const justOver24Hours = now - (24 * 60 * 60 * 1000 + 1000);
      const result = getBackupInfo(justOver24Hours);
      expect(result.status).toBe('stale');
    });

    it('stale at exactly 72 hours', () => {
      const now = Date.now();
      const exactly72Hours = now - 72 * 60 * 60 * 1000;
      const result = getBackupInfo(exactly72Hours);
      expect(result.status).toBe('stale');
    });

    it('overdue just after 72 hours', () => {
      const now = Date.now();
      const justOver72Hours = now - (72 * 60 * 60 * 1000 + 1000);
      const result = getBackupInfo(justOver72Hours);
      expect(result.status).toBe('overdue');
    });
  });

  describe('custom thresholds', () => {
    it('uses custom freshHours threshold', () => {
      const now = Date.now();
      // 6 hours ago - would be fresh with default 24h threshold
      const sixHoursAgo = now - 6 * 60 * 60 * 1000;

      // With default thresholds (24h fresh, 72h stale)
      const resultDefault = getBackupInfo(sixHoursAgo);
      expect(resultDefault.status).toBe('fresh');

      // With custom threshold of 4 hours for fresh
      const resultCustom = getBackupInfo(sixHoursAgo, { freshHours: 4, staleHours: 12 });
      expect(resultCustom.status).toBe('stale'); // 6 hours > 4 hours fresh threshold
    });

    it('uses custom staleHours threshold', () => {
      const now = Date.now();
      // 2 days ago - would be stale with default 72h threshold
      const twoDaysAgo = now - 48 * 60 * 60 * 1000;

      // With default thresholds
      const resultDefault = getBackupInfo(twoDaysAgo);
      expect(resultDefault.status).toBe('stale');

      // With custom threshold of 24h stale (same as fresh)
      const resultCustom = getBackupInfo(twoDaysAgo, { freshHours: 12, staleHours: 24 });
      expect(resultCustom.status).toBe('overdue'); // 48 hours > 24 hours stale threshold
    });

    it('handles partial custom thresholds', () => {
      const now = Date.now();
      const thirtyHoursAgo = now - 30 * 60 * 60 * 1000;

      // Only provide freshHours, staleHours uses default (72h)
      const result = getBackupInfo(thirtyHoursAgo, { freshHours: 12 });
      expect(result.status).toBe('stale'); // 30h > 12h fresh, but < 72h default stale
    });
  });
});

describe('formatObservedSpeed', () => {
  it('preserves zero and leaves missing or invalid observations unavailable', () => {
    expect(formatObservedSpeed(0)).toBe('0 B/s');
    expect(formatObservedSpeed(1024)).toBe('1.00 KB/s');
    for (const value of [undefined, null, -1, NaN, Infinity]) {
      expect(formatObservedSpeed(value)).toBe('-');
    }
  });
});
