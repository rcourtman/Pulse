import { describe, expect, it } from 'vitest';
import {
  buildRecoveryActivitySummary,
  buildRecoveryAttentionItems,
  buildRecoveryFreshnessBuckets,
  buildRecoveryOutcomeSegments,
  buildRecoveryPostureSegments,
  buildRecoveryPostureSummary,
  RECOVERY_FRESHNESS_BUCKETS,
  RECOVERY_SUMMARY_TIME_RANGES,
  RECOVERY_SUMMARY_TIME_RANGE_LABELS,
} from '@/utils/recoverySummaryPresentation';

describe('recoverySummaryPresentation', () => {
  it('exposes canonical recovery summary time ranges and labels', () => {
    expect(RECOVERY_SUMMARY_TIME_RANGES).toEqual(['7d', '30d', '90d']);
    expect(RECOVERY_SUMMARY_TIME_RANGE_LABELS['7d']).toBe('7d');
    expect(RECOVERY_SUMMARY_TIME_RANGE_LABELS['30d']).toBe('30d');
    expect(RECOVERY_SUMMARY_TIME_RANGE_LABELS['90d']).toBe('90d');
  });

  it('exposes canonical freshness bucket presentation', () => {
    expect(RECOVERY_FRESHNESS_BUCKETS).toEqual([
      { key: 'under1h', label: '<1h', color: 'bg-emerald-500' },
      { key: 'under24h', label: '<24h', color: 'bg-emerald-400' },
      { key: 'under7d', label: '<7d', color: 'bg-amber-400' },
      { key: 'over7d', label: '>7d', color: 'bg-red-500' },
    ]);
  });

  it('builds outcome segments with counts and percentages', () => {
    expect(
      buildRecoveryOutcomeSegments({
        total: 10,
        counts: {
          success: 7,
          warning: 2,
          failed: 1,
          running: 0,
          unknown: 0,
        },
      }),
    ).toMatchObject([
      { outcome: 'success', count: 7, percent: 70 },
      { outcome: 'warning', count: 2, percent: 20 },
      { outcome: 'failed', count: 1, percent: 10 },
    ]);
  });

  it('builds freshness buckets from latest success timestamps', () => {
    const now = Date.parse('2026-03-09T12:00:00Z');
    const buckets = buildRecoveryFreshnessBuckets(
      [
        { rollupId: 'a', lastOutcome: 'success', lastSuccessAt: '2026-03-09T11:30:00Z' },
        { rollupId: 'b', lastOutcome: 'success', lastSuccessAt: '2026-03-09T06:00:00Z' },
        { rollupId: 'c', lastOutcome: 'success', lastSuccessAt: '2026-03-05T12:00:00Z' },
        { rollupId: 'd', lastOutcome: 'failed', lastSuccessAt: null },
      ],
      now,
    );

    expect(buckets.map((bucket) => bucket.count)).toEqual([1, 1, 1, 1]);
  });

  it('builds posture summary and disjoint posture segments for recovery readiness', () => {
    const now = Date.parse('2026-03-09T12:00:00Z');
    const rollups = [
      { rollupId: 'healthy', lastOutcome: 'success', lastSuccessAt: '2026-03-09T11:30:00Z' },
      { rollupId: 'stale', lastOutcome: 'success', lastSuccessAt: '2026-02-28T12:00:00Z' },
      { rollupId: 'failed', lastOutcome: 'failed', lastSuccessAt: '2026-03-08T12:00:00Z' },
      { rollupId: 'warning', lastOutcome: 'warning', lastSuccessAt: '2026-03-08T12:00:00Z' },
      { rollupId: 'running', lastOutcome: 'running', lastSuccessAt: '2026-03-09T10:30:00Z' },
      { rollupId: 'never', lastOutcome: 'failed', lastAttemptAt: '2026-03-09T10:00:00Z' },
    ];

    expect(buildRecoveryPostureSummary(rollups, now)).toMatchObject({
      healthy: 1,
      stale: 1,
      failed: 1,
      warning: 1,
      running: 1,
      neverSucceeded: 1,
      unknown: 0,
      attention: 4,
    });

    expect(buildRecoveryPostureSegments(rollups, now)).toMatchObject([
      { key: 'healthy', count: 1, percent: 17 },
      { key: 'stale', count: 1, percent: 17 },
      { key: 'failed', count: 1, percent: 17 },
      { key: 'warning', count: 1, percent: 17 },
      { key: 'running', count: 1, percent: 17 },
      { key: 'never-succeeded', count: 1, percent: 17 },
    ]);
  });

  it('summarizes activity bars and headline metrics', () => {
    const activity = buildRecoveryActivitySummary([
      { day: '2026-03-07', total: 0, snapshot: 0, local: 0, remote: 0 },
      { day: '2026-03-08', total: 4, snapshot: 1, local: 1, remote: 2 },
      { day: '2026-03-09', total: 2, snapshot: 1, local: 1, remote: 0 },
    ]);

    expect(activity).toMatchObject({
      hasData: true,
      totalEvents: 6,
      activeDays: 2,
      latestCount: 2,
      busiestCount: 4,
    });
    expect(activity.bars).toHaveLength(3);
    expect(activity.bars[1]).toMatchObject({ isPeak: true });
    expect(activity.bars[2]).toMatchObject({ isLatest: true });
  });

  it('builds attention items ordered by operational urgency', () => {
    expect(
      buildRecoveryAttentionItems({
        counts: {
          success: 5,
          warning: 2,
          failed: 1,
          running: 1,
          unknown: 0,
        },
        stale: 3,
        neverSucceeded: 2,
      }),
    ).toMatchObject([
      { key: 'failed', count: 1 },
      { key: 'never-succeeded', count: 2 },
      { key: 'warning', count: 2 },
      { key: 'stale', count: 3 },
      { key: 'running', count: 1 },
    ]);
  });
});
