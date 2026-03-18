import { describe, expect, it } from 'vitest';
import {
  getCephHealthPresentation,
  getCephHealthLabel,
  getCephHealthStyles,
  getCephLoadingStatePresentation,
  getCephDisconnectedStatePresentation,
  getCephNoClustersStatePresentation,
  getCephPoolsSearchEmptyStatePresentation,
  getCephServiceStatusPresentation,
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
const CEPH_SERVICE_OK_TEXT = 'text-green-600 dark:text-green-400';
const CEPH_SERVICE_WARNING_TEXT = 'text-yellow-600 dark:text-yellow-400';
const CEPH_SERVICE_CRITICAL_TEXT = 'text-red-600 dark:text-red-400';

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
        [undefined, 'UNKNOWN'],
        [null, 'UNKNOWN'],
        ['OK', 'OK'],
        ['HEALTH_OK', 'OK'],
        ['WARN', 'WARN'],
        ['HEALTH_WARN', 'WARN'],
        ['HEALTH_WARNING', 'WARN'],
        ['HEALTH_ERR', 'ERROR'],
        ['HEALTH_ERROR', 'ERROR'],
        ['HEALTH_CRIT', 'ERROR'],
        ['CRITICAL', 'ERROR'],
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
        ['OK', CEPH_HEALTH_OK_STYLES],
        ['HEALTH_OK', CEPH_HEALTH_OK_STYLES],
        ['WARN', CEPH_HEALTH_WARNING_STYLES],
        ['HEALTH_WARN', CEPH_HEALTH_WARNING_STYLES],
        ['HEALTH_WARNING', CEPH_HEALTH_WARNING_STYLES],
        ['ERROR', CEPH_HEALTH_CRITICAL_STYLES],
        ['HEALTH_ERR', CEPH_HEALTH_CRITICAL_STYLES],
        ['HEALTH_ERROR', CEPH_HEALTH_CRITICAL_STYLES],
        ['HEALTH_CRIT', CEPH_HEALTH_CRITICAL_STYLES],
        ['CRITICAL', CEPH_HEALTH_CRITICAL_STYLES],
        ['HEALTH_UNKNOWN', CEPH_HEALTH_DEFAULT_STYLES],
        [undefined, CEPH_HEALTH_DEFAULT_STYLES],
        [null, CEPH_HEALTH_DEFAULT_STYLES],
      ];

      cases.forEach(([health, expected]) => {
        expect(getCephHealthStyles(health)).toBe(expected);
      });
    });
  });

  describe('getCephHealthPresentation', () => {
    it('returns a complete canonical Ceph health presentation contract', () => {
      expect(getCephHealthPresentation('HEALTH_OK')).toEqual({
        label: 'OK',
        badgeClass: CEPH_HEALTH_OK_STYLES,
        dotClass: 'bg-green-500',
      });
      expect(getCephHealthPresentation('HEALTH_WARN')).toEqual({
        label: 'WARN',
        badgeClass: CEPH_HEALTH_WARNING_STYLES,
        dotClass: 'bg-yellow-500',
      });
      expect(getCephHealthPresentation('CRITICAL')).toEqual({
        label: 'ERROR',
        badgeClass: CEPH_HEALTH_CRITICAL_STYLES,
        dotClass: 'bg-red-500',
      });
      expect(getCephHealthPresentation(undefined)).toEqual({
        label: 'UNKNOWN',
        badgeClass: CEPH_HEALTH_DEFAULT_STYLES,
        dotClass: 'bg-blue-500',
      });
    });
  });

  describe('getCephServiceStatusPresentation', () => {
    it('returns a canonical presentation contract for ceph service counts', () => {
      expect(getCephServiceStatusPresentation({ running: 3, total: 3 })).toEqual({
        label: 'healthy',
        textClass: CEPH_SERVICE_OK_TEXT,
      });

      expect(getCephServiceStatusPresentation({ running: 1, total: 3 })).toEqual({
        label: 'degraded',
        textClass: CEPH_SERVICE_WARNING_TEXT,
      });

      expect(getCephServiceStatusPresentation({ running: 0, total: 3 })).toEqual({
        label: 'down',
        textClass: CEPH_SERVICE_CRITICAL_TEXT,
      });
    });

    it('preserves the current 0/0 healthy behavior and handles missing values', () => {
      expect(getCephServiceStatusPresentation({ running: 0, total: 0 })).toEqual({
        label: 'healthy',
        textClass: CEPH_SERVICE_OK_TEXT,
      });

      expect(getCephServiceStatusPresentation(undefined)).toEqual({
        label: 'healthy',
        textClass: CEPH_SERVICE_OK_TEXT,
      });
    });
  });

  describe('ceph page state presentation', () => {
    it('returns canonical ceph loading copy', () => {
      expect(getCephLoadingStatePresentation()).toEqual({
        title: 'Loading Ceph data...',
        description: 'Connecting to the monitoring service.',
      });
    });

    it('returns canonical ceph disconnected copy', () => {
      expect(getCephDisconnectedStatePresentation(true)).toEqual({
        title: 'Connection lost',
        description: 'Attempting to reconnect…',
      });
      expect(getCephDisconnectedStatePresentation(false)).toEqual({
        title: 'Connection lost',
        description: 'Unable to connect to the backend server',
      });
    });

    it('returns canonical no-clusters copy', () => {
      expect(getCephNoClustersStatePresentation()).toEqual({
        title: 'No Ceph Clusters Detected',
        description:
          'Ceph cluster data will appear here when detected via the Pulse agent on your Proxmox nodes. Install the agent on a node with Ceph configured.',
      });
    });

    it('returns canonical pool-search empty copy', () => {
      expect(getCephPoolsSearchEmptyStatePresentation('fast')).toEqual({
        text: 'No pools match "fast"',
      });
    });
  });
});
