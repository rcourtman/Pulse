import { describe, expect, it } from 'vitest';

import type { Disk } from '@/types/api';

import { getWorkloadsDiskUsageText } from '../diskListModel';

// getWorkloadsDiskUsageText is a single ternary delegating to
// hasWorkloadsDiskCapacity, which itself branches on
// `typeof disk.total === 'number' && disk.total > 0`. Each test below pins a
// concrete branch outcome (true arm formats used/total bytes; false arm returns
// the fixed 'Usage unavailable' copy) with exact string assertions.

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

describe('diskListModel.getWorkloadsDiskUsageText (branch coverage 0712c)', () => {
  describe('true arm: hasWorkloadsDiskCapacity(disk) is true (total is a positive number)', () => {
    it('formats "used/total" bytes with auto precision for both operands', () => {
      // hasWorkloadsDiskCapacity true (total 100 > 0) -> true arm.
      // formatBytes(25)  = '25.0 B'  (value 25 -> 1 decimal)
      // formatBytes(100) = '100 B'   (value 100 -> 0 decimals)
      expect(getWorkloadsDiskUsageText(makeDisk({ used: 25, total: 100 }))).toBe('25.0 B/100 B');
    });

    it('renders 0 used bytes via the formatBytes(!bytes) early return, keeping the true arm', () => {
      // total 200 > 0 -> true arm; used 0 -> formatBytes(0) early-returns '0 B'.
      // formatBytes(200) = '200 B' (value >= 100 -> 0 decimals).
      expect(getWorkloadsDiskUsageText(makeDisk({ used: 0, total: 200 }))).toBe('0 B/200 B');
    });

    it('formats fractional sub-10 values with 2 decimals on the true arm', () => {
      // total 1024 > 0 -> true arm.
      // formatBytes(5) = '5.00 B' (value < 10 -> 2 decimals).
      // formatBytes(1024): i = floor(log(1024)/log(1024)) = 1, value = 1 (<10) -> '1.00 KB'.
      expect(getWorkloadsDiskUsageText(makeDisk({ used: 5, total: 1024 }))).toBe('5.00 B/1.00 KB');
    });
  });

  describe('false arm: hasWorkloadsDiskCapacity(disk) is false -> "Usage unavailable"', () => {
    it('returns the unavailable copy when total === 0 (total not > 0)', () => {
      // typeof total === 'number' true, but total > 0 false -> hasWorkloadsDiskCapacity false.
      expect(getWorkloadsDiskUsageText(makeDisk({ total: 0, used: 0, free: 0 }))).toBe(
        'Usage unavailable',
      );
    });

    it('returns the unavailable copy when total is a negative number', () => {
      // typeof total === 'number' true, total > 0 false (-5) -> false arm.
      expect(getWorkloadsDiskUsageText(makeDisk({ total: -5, used: 0, free: 0 }))).toBe(
        'Usage unavailable',
      );
    });

    it('returns the unavailable copy when total is not a number (typeof guard fails)', () => {
      // Defensive branch: a Disk whose total is undefined at runtime. The
      // typeof check inside hasWorkloadsDiskCapacity short-circuits before the
      // total > 0 comparison, so formatBytes is never called (no NaN output).
      // Disk.total is typed `number`, so the undefined value needs a cast.
      const disk = { total: undefined, used: 25, free: 75, usage: 25 } as unknown as Disk;
      expect(getWorkloadsDiskUsageText(disk)).toBe('Usage unavailable');
    });
  });
});
