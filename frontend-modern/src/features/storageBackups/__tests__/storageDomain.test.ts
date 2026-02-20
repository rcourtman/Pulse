import { describe, expect, it } from 'vitest';
import {
  getCephHealthLabel,
  getCephHealthStyles,
  isCephType,
} from '@/features/storageBackups/storageDomain';

const CEPH_HEALTH_OK_STYLES =
  'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 border border-green-200 dark:border-green-800';
const CEPH_HEALTH_WARNING_STYLES =
  'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-200 border border-yellow-300 dark:border-yellow-800';
const CEPH_HEALTH_CRITICAL_STYLES =
  'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200 border border-red-300 dark:border-red-800';
const CEPH_HEALTH_DEFAULT_STYLES =
  'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200 border border-blue-200 dark:border-blue-700';

describe('storageDomain', () => {
  describe('isCephType', () => {
    it('returns true for ceph storage types', () => {
      expect(isCephType('rbd')).toBe(true);
      expect(isCephType('cephfs')).toBe(true);
      expect(isCephType('ceph')).toBe(true);
      expect(isCephType('RBD')).toBe(true);
      expect(isCephType(' CePhFs ')).toBe(true);
    });

    it('returns false for non-ceph storage types', () => {
      expect(isCephType('dir')).toBe(false);
      expect(isCephType('lvm')).toBe(false);
      expect(isCephType('zfspool')).toBe(false);
      expect(isCephType('pbs')).toBe(false);
      expect(isCephType('')).toBe(false);
    });

    it('handles undefined and null', () => {
      expect(isCephType(undefined)).toBe(false);
      expect(isCephType(null)).toBe(false);
    });
  });

  describe('getCephHealthLabel', () => {
    it('maps health values to labels', () => {
      const cases: Array<[string | null | undefined, string]> = [
        [undefined, 'CEPH'],
        [null, 'CEPH'],
        ['HEALTH_OK', 'OK'],
        ['HEALTH_WARN', 'WARN'],
        ['HEALTH_WARNING', 'WARNING'],
        ['HEALTH_ERR', 'ERR'],
        ['HEALTH_ERROR', 'ERROR'],
        ['HEALTH_CRIT', 'CRIT'],
        ['HEALTH_UNKNOWN', 'UNKNOWN'],
      ];

      cases.forEach(([health, expected]) => {
        expect(getCephHealthLabel(health)).toBe(expected);
      });
    });
  });

  describe('getCephHealthStyles', () => {
    it('maps health values to style classes', () => {
      const cases: Array<[string | null | undefined, string]> = [
        ['HEALTH_OK', CEPH_HEALTH_OK_STYLES],
        ['HEALTH_WARN', CEPH_HEALTH_WARNING_STYLES],
        ['HEALTH_WARNING', CEPH_HEALTH_WARNING_STYLES],
        ['HEALTH_ERR', CEPH_HEALTH_CRITICAL_STYLES],
        ['HEALTH_ERROR', CEPH_HEALTH_CRITICAL_STYLES],
        ['HEALTH_CRIT', CEPH_HEALTH_CRITICAL_STYLES],
        ['HEALTH_UNKNOWN', CEPH_HEALTH_DEFAULT_STYLES],
        [undefined, CEPH_HEALTH_DEFAULT_STYLES],
        [null, CEPH_HEALTH_DEFAULT_STYLES],
      ];

      cases.forEach(([health, expected]) => {
        expect(getCephHealthStyles(health)).toBe(expected);
      });
    });
  });
});
