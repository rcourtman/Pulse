import { describe, expect, it } from 'vitest';
import {
  recordDiskMetric,
  getDiskMetricHistory,
  getLatestDiskMetric,
  getDiskMetricsVersion,
  getDiskMetricsStats,
} from '@/stores/diskMetricsHistory';

describe('diskMetricsHistory', () => {
  describe('recordDiskMetric', () => {
    it('records metric snapshot with correct values', () => {
      const before = getDiskMetricsVersion();
      const resourceId = `test-${Date.now()}-r1`;
      recordDiskMetric(resourceId, 1000, 500, 10, 5, 25);
      const after = getDiskMetricsVersion();

      expect(after).toBe(before + 1);

      const history = getDiskMetricHistory(resourceId, 60000);
      expect(history).toHaveLength(1);
      expect(history[0].readBps).toBe(1000);
      expect(history[0].writeBps).toBe(500);
      expect(history[0].readIops).toBe(10);
      expect(history[0].writeIops).toBe(5);
      expect(history[0].ioTime).toBe(25);
    });

    it('rounds values correctly', () => {
      const resourceId = `test-${Date.now()}-r2`;
      recordDiskMetric(resourceId, 1000.7, 500.3, 10.9, 5.1, 25.67);

      const history = getDiskMetricHistory(resourceId, 60000);
      expect(history[0].readBps).toBe(1001);
      expect(history[0].writeBps).toBe(500);
      expect(history[0].readIops).toBe(11);
      expect(history[0].writeIops).toBe(5);
      expect(history[0].ioTime).toBe(25.7);
    });

    it('stores metrics for different resources separately', () => {
      const resourceA = `test-${Date.now()}-ra`;
      const resourceB = `test-${Date.now()}-rb`;
      recordDiskMetric(resourceA, 1000, 500, 10, 5, 25);
      recordDiskMetric(resourceB, 2000, 1000, 20, 10, 50);

      const historyA = getDiskMetricHistory(resourceA, 60000);
      const historyB = getDiskMetricHistory(resourceB, 60000);

      expect(historyA).toHaveLength(1);
      expect(historyB).toHaveLength(1);
      expect(historyA[0].readBps).toBe(1000);
      expect(historyB[0].readBps).toBe(2000);
    });

    it('appends to existing history for same resource', () => {
      const resourceId = `test-${Date.now()}-r3`;
      recordDiskMetric(resourceId, 1000, 500, 10, 5, 25);
      recordDiskMetric(resourceId, 2000, 1000, 20, 10, 50);

      const history = getDiskMetricHistory(resourceId, 60000);
      expect(history).toHaveLength(2);
    });
  });

  describe('getDiskMetricHistory', () => {
    it('returns empty array for unknown resource', () => {
      const history = getDiskMetricHistory(`unknown-${Date.now()}`, 60000);
      expect(history).toEqual([]);
    });
  });

  describe('getLatestDiskMetric', () => {
    it('returns null for unknown resource', () => {
      const latest = getLatestDiskMetric(`unknown-${Date.now()}`);
      expect(latest).toBeNull();
    });

    it('returns most recent metric', () => {
      const resourceId = `test-${Date.now()}-r4`;
      recordDiskMetric(resourceId, 1000, 500, 10, 5, 25);
      recordDiskMetric(resourceId, 2000, 1000, 20, 10, 50);
      recordDiskMetric(resourceId, 3000, 1500, 30, 15, 75);

      const latest = getLatestDiskMetric(resourceId);
      expect(latest?.readBps).toBe(3000);
    });
  });

  describe('getDiskMetricsVersion', () => {
    it('increments when new metrics are recorded', () => {
      const before = getDiskMetricsVersion();
      const resourceId = `test-${Date.now()}-r5`;
      recordDiskMetric(resourceId, 1000, 500, 10, 5, 25);
      const after = getDiskMetricsVersion();

      expect(after).toBe(before + 1);
    });
  });

  describe('getDiskMetricsStats', () => {
    it('returns resource count and buffer size', () => {
      const stats = getDiskMetricsStats();

      expect(stats).toHaveProperty('resourceCount');
      expect(stats).toHaveProperty('bufferSize');
      expect(stats.bufferSize).toBe(2000);
    });
  });
});
