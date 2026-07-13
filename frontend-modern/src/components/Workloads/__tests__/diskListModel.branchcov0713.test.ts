import { describe, expect, it } from 'vitest';

import type { Disk } from '@/types/api';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';

import {
  buildWorkloadsDiskPresentation,
  getWorkloadsDiskLabel,
  getWorkloadsDiskLabelTitle,
  getWorkloadsDiskProgressClass,
  getWorkloadsDiskProgressWidth,
  getWorkloadsDiskTypeLabel,
  getWorkloadsDiskUsagePercent,
  getWorkloadsDiskUsagePercentLabel,
  hasWorkloadsDiskCapacity,
} from '../diskListModel';

// getWorkloadsDiskUsageText is intentionally NOT re-tested here: every one of
// its branches is already pinned by diskListModel.branchcov0712c.test.ts. It is
// still exercised indirectly below through buildWorkloadsDiskPresentation
// (which composes it), which covers both arms without duplicating unit tests.

// Tailwind background classes mirrored from metricThresholds.BG_CLASSES. The
// default disk display thresholds are warning 80 / critical 90
// (METRIC_THRESHOLDS.disk), used whenever thresholds is undefined or null.
const NORMAL_CLASS = 'bg-metric-normal-bg dark:bg-metric-normal-bg';
const WARNING_CLASS = 'bg-metric-warning-bg dark:bg-metric-warning-bg';
const CRITICAL_CLASS = 'bg-metric-critical-bg dark:bg-metric-critical-bg';

const makeDisk = (overrides: Partial<Disk> = {}): Disk => ({
  total: 100,
  used: 25,
  free: 75,
  usage: 25,
  mountpoint: '/',
  type: 'ext4',
  device: '/dev/sda1',
  ...overrides,
});

describe('diskListModel (branch coverage 0713)', () => {
  describe('hasWorkloadsDiskCapacity', () => {
    it('returns true when total is a positive number (both typeof and > 0 true)', () => {
      expect(hasWorkloadsDiskCapacity(makeDisk({ total: 100 }))).toBe(true);
    });

    it('returns false when total === 0 (typeof number true, total > 0 false)', () => {
      expect(hasWorkloadsDiskCapacity(makeDisk({ total: 0 }))).toBe(false);
    });

    it('returns false when total is a negative number (typeof number true, total > 0 false)', () => {
      expect(hasWorkloadsDiskCapacity(makeDisk({ total: -10 }))).toBe(false);
    });

    it('returns false when total is not a number (typeof guard short-circuits before the > 0 compare)', () => {
      const disk = { total: undefined, used: 0 } as unknown as Disk;
      expect(hasWorkloadsDiskCapacity(disk)).toBe(false);
    });
  });

  describe('getWorkloadsDiskUsagePercent', () => {
    it('returns 0 when the disk has no capacity (false arm of the guard)', () => {
      expect(getWorkloadsDiskUsagePercent(makeDisk({ total: 0 }))).toBe(0);
    });

    it('returns (used / total) * 100 for a normal ratio (true arm of the guard)', () => {
      expect(getWorkloadsDiskUsagePercent(makeDisk({ used: 25, total: 100 }))).toBe(25);
    });

    it('does not clamp the upper bound, so used > total yields a value over 100', () => {
      expect(getWorkloadsDiskUsagePercent(makeDisk({ used: 150, total: 100 }))).toBe(150);
    });
  });

  describe('getWorkloadsDiskLabel', () => {
    it('returns the mountpoint when it is truthy (first || arm)', () => {
      expect(getWorkloadsDiskLabel(makeDisk({ mountpoint: '/data', device: '/dev/sda1' }))).toBe(
        '/data',
      );
    });

    it('falls back to the device when the mountpoint is an empty string (second || arm)', () => {
      expect(getWorkloadsDiskLabel(makeDisk({ mountpoint: '', device: '/dev/sda1' }))).toBe(
        '/dev/sda1',
      );
    });

    it('falls back to the device when the mountpoint is undefined (second || arm)', () => {
      expect(getWorkloadsDiskLabel(makeDisk({ mountpoint: undefined, device: '/dev/nvme0' }))).toBe(
        '/dev/nvme0',
      );
    });

    it('returns "Unknown" when both mountpoint and device are falsy (final || arm)', () => {
      expect(getWorkloadsDiskLabel(makeDisk({ mountpoint: undefined, device: undefined }))).toBe(
        'Unknown',
      );
    });
  });

  describe('getWorkloadsDiskLabelTitle', () => {
    it('returns the label unchanged when it is not "Unknown" (true arm of the ternary)', () => {
      expect(getWorkloadsDiskLabelTitle('/data')).toBe('/data');
    });

    it('returns undefined when the label is "Unknown" (false arm of the ternary)', () => {
      expect(getWorkloadsDiskLabelTitle('Unknown')).toBeUndefined();
    });
  });

  describe('getWorkloadsDiskUsagePercentLabel', () => {
    it('formats a whole-number percent with toFixed(0) when the disk has capacity (true arm)', () => {
      expect(getWorkloadsDiskUsagePercentLabel(makeDisk({ used: 25, total: 100 }))).toBe('25%');
    });

    it('rounds a fractional percent to the nearest integer via toFixed(0)', () => {
      // (2 / 3) * 100 = 66.666...; toFixed(0) rounds to "67".
      expect(getWorkloadsDiskUsagePercentLabel(makeDisk({ used: 2, total: 3 }))).toBe('67%');
    });

    it('returns the em dash glyph when the disk has no capacity (false arm)', () => {
      expect(getWorkloadsDiskUsagePercentLabel(makeDisk({ total: 0 }))).toBe('—');
    });
  });

  describe('getWorkloadsDiskProgressClass', () => {
    it('maps a sub-warning percent to the normal class with default thresholds', () => {
      // Default disk thresholds: warning 80 / critical 90. 60 < 80 -> normal.
      expect(getWorkloadsDiskProgressClass(makeDisk({ used: 60, total: 100 }))).toBe(NORMAL_CLASS);
    });

    it('maps a percent in the warning band to the warning class with default thresholds', () => {
      // 85 >= 80 and < 90 -> warning.
      expect(getWorkloadsDiskProgressClass(makeDisk({ used: 85, total: 100 }))).toBe(WARNING_CLASS);
    });

    it('maps a percent at or above critical to the critical class with default thresholds', () => {
      // 90 >= 90 -> critical.
      expect(getWorkloadsDiskProgressClass(makeDisk({ used: 90, total: 100 }))).toBe(CRITICAL_CLASS);
    });

    it('maps a no-capacity disk (percent 0) to the normal class', () => {
      expect(getWorkloadsDiskProgressClass(makeDisk({ total: 0 }))).toBe(NORMAL_CLASS);
    });

    it('falls back to the default disk thresholds when thresholds is null (nullish arm)', () => {
      // thresholds null -> getMetricSeverity uses getFallbackSeverityThresholds('disk')
      // -> METRIC_THRESHOLDS.disk {80, 90}. 90 >= 90 -> critical.
      expect(getWorkloadsDiskProgressClass(makeDisk({ used: 90, total: 100 }), null)).toBe(
        CRITICAL_CLASS,
      );
    });

    it('honors custom thresholds that move the band boundaries', () => {
      // { warning: 50, critical: 75 }: 60 >= 50 and < 75 -> warning (would be
      // normal under the default 80/90 thresholds).
      const thresholds: MetricDisplayThresholds = { warning: 50, critical: 75 };
      expect(getWorkloadsDiskProgressClass(makeDisk({ used: 60, total: 100 }), thresholds)).toBe(
        WARNING_CLASS,
      );
      // 80 >= 75 -> critical under custom thresholds.
      expect(getWorkloadsDiskProgressClass(makeDisk({ used: 80, total: 100 }), thresholds)).toBe(
        CRITICAL_CLASS,
      );
      // 40 < 50 -> normal under custom thresholds.
      expect(getWorkloadsDiskProgressClass(makeDisk({ used: 40, total: 100 }), thresholds)).toBe(
        NORMAL_CLASS,
      );
    });
  });

  describe('getWorkloadsDiskProgressWidth', () => {
    it('renders the raw percent when it is <= 100', () => {
      expect(getWorkloadsDiskProgressWidth(makeDisk({ used: 25, total: 100 }))).toBe('25%');
    });

    it('clamps a percent over 100 down to 100 via Math.min', () => {
      // getWorkloadsDiskUsagePercent yields 150; Math.min(150, 100) -> 100.
      expect(getWorkloadsDiskProgressWidth(makeDisk({ used: 150, total: 100 }))).toBe('100%');
    });

    it('renders 0% for a no-capacity disk (percent 0)', () => {
      expect(getWorkloadsDiskProgressWidth(makeDisk({ total: 0 }))).toBe('0%');
    });
  });

  describe('getWorkloadsDiskTypeLabel', () => {
    it('uppercases a lowercase type string', () => {
      expect(getWorkloadsDiskTypeLabel(makeDisk({ type: 'ext4' }))).toBe('EXT4');
    });

    it('returns an empty string when type is undefined (?. short-circuits, ?? arm)', () => {
      expect(getWorkloadsDiskTypeLabel(makeDisk({ type: undefined }))).toBe('');
    });

    it('returns an empty string for an empty-string type without triggering the ?? arm', () => {
      // ''.toUpperCase() === '' which is not nullish, so ?? '' does not run, but
      // the observable result is still ''.
      expect(getWorkloadsDiskTypeLabel(makeDisk({ type: '' }))).toBe('');
    });
  });

  describe('buildWorkloadsDiskPresentation', () => {
    it('composes a full presentation for a populated disk with capacity', () => {
      // Every helper's true/happy arm: capacity present, mountpoint truthy,
      // type present. formatBytes(25) = '25.0 B', formatBytes(100) = '100 B'.
      const disk = makeDisk({
        mountpoint: '/data',
        device: '/dev/sda1',
        type: 'ext4',
        total: 100,
        used: 25,
        free: 75,
      });
      expect(buildWorkloadsDiskPresentation(disk, 2)).toStrictEqual({
        key: '/data:/dev/sda1:2',
        label: '/data',
        labelTitle: '/data',
        progressClass: NORMAL_CLASS,
        progressWidth: '25%',
        typeLabel: 'EXT4',
        usageText: '25.0 B/100 B',
        usagePercentLabel: '25%',
      });
    });

    it('uses the ?? "" arms in the key and forces the Unknown label when mountpoint and device are undefined', () => {
      // mountpoint ?? '' and device ?? '' both take their right arms -> "::0".
      // label = 'Unknown' (both falsy) -> labelTitle undefined.
      const disk = makeDisk({
        mountpoint: undefined,
        device: undefined,
        type: 'xfs',
        total: 200,
        used: 50,
        free: 150,
      });
      expect(buildWorkloadsDiskPresentation(disk, 0)).toStrictEqual({
        key: '::0',
        label: 'Unknown',
        labelTitle: undefined,
        progressClass: NORMAL_CLASS,
        progressWidth: '25%',
        typeLabel: 'XFS',
        // formatBytes(50) = '50.0 B', formatBytes(200) = '200 B'.
        usageText: '50.0 B/200 B',
        usagePercentLabel: '25%',
      });
    });

    it('renders the unavailable/zero/em-dash fields for a no-capacity disk (getWorkloadsDiskUsageText false arm)', () => {
      const disk = makeDisk({
        mountpoint: '/foo',
        device: '/dev/sdb',
        type: 'ext4',
        total: 0,
        used: 0,
        free: 0,
      });
      expect(buildWorkloadsDiskPresentation(disk, 5)).toStrictEqual({
        key: '/foo:/dev/sdb:5',
        label: '/foo',
        labelTitle: '/foo',
        progressClass: NORMAL_CLASS,
        progressWidth: '0%',
        typeLabel: 'EXT4',
        usageText: 'Usage unavailable',
        usagePercentLabel: '—',
      });
    });

    it('threads the thresholds argument through to the progress class', () => {
      // 60% is normal under default thresholds but warning under {50, 75}.
      const thresholds: MetricDisplayThresholds = { warning: 50, critical: 75 };
      const presentation = buildWorkloadsDiskPresentation(
        makeDisk({ used: 60, total: 100 }),
        0,
        thresholds,
      );
      expect(presentation.progressClass).toBe(WARNING_CLASS);
    });

    it('reflects the index argument in the key segment', () => {
      const presentation = buildWorkloadsDiskPresentation(makeDisk(), 9);
      expect(presentation.key).toBe('/:/dev/sda1:9');
    });
  });
});
