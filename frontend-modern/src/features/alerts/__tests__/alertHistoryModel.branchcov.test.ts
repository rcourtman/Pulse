import { describe, expect, it } from 'vitest';

import {
  buildAlertTrends,
  buildSelectedBucketDetails,
  MS_PER_HOUR,
  type AlertTrendSeries,
} from '../alertHistoryModel';

// ---------------------------------------------------------------------------
// buildSelectedBucketDetails — branch coverage for the null guard and the
// value-returning path (including same-day vs cross-day range labels routed
// through formatAlertBucketRange).
// ---------------------------------------------------------------------------

describe('buildSelectedBucketDetails', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0); // Sun Mar 22 2026 12:00:00 UTC

  // --- null guard ----------------------------------------------------------

  it('returns null when selectedBarIndex is null', () => {
    const trends = buildAlertTrends([], '24h', now);
    expect(buildSelectedBucketDetails(null, trends, 'en-US')).toBeNull();
  });

  it('returns null for null index even with an empty trends series', () => {
    const emptyTrends: AlertTrendSeries = {
      buckets: [],
      max: 0,
      bucketSize: 1,
      bucketTimes: [],
      rangeStart: now,
      rangeHours: 0,
    };
    expect(buildSelectedBucketDetails(null, emptyTrends, 'en-US')).toBeNull();
  });

  // --- value path: index resolution & numeric start/end -------------------

  it('resolves the first bucket (index 0) using real 24h trends', () => {
    const trends = buildAlertTrends([], '24h', now);
    const details = buildSelectedBucketDetails(0, trends, 'en-US');

    expect(details).not.toBeNull();
    const expectedStart = trends.bucketTimes[0];
    expect(details!.start).toBe(expectedStart);
    expect(details!.end).toBe(expectedStart + trends.bucketSize * MS_PER_HOUR);
    expect(details!.end - details!.start).toBe(MS_PER_HOUR);
  });

  it('resolves a middle bucket using real 24h trends', () => {
    const trends = buildAlertTrends([], '24h', now);
    const mid = Math.floor(trends.bucketTimes.length / 2);
    const details = buildSelectedBucketDetails(mid, trends, 'en-US');

    expect(details).not.toBeNull();
    expect(details!.start).toBe(trends.bucketTimes[mid]);
    expect(details!.end).toBe(trends.bucketTimes[mid] + MS_PER_HOUR);
  });

  it('resolves the last bucket using real 24h trends', () => {
    const trends = buildAlertTrends([], '24h', now);
    const last = trends.bucketTimes.length - 1;
    const details = buildSelectedBucketDetails(last, trends, 'en-US');

    expect(details).not.toBeNull();
    expect(details!.start).toBe(trends.bucketTimes[last]);
    expect(details!.end).toBe(trends.bucketTimes[last] + MS_PER_HOUR);
  });

  it('honours a larger bucketSize (24h) when computing end', () => {
    const start = Date.UTC(2026, 2, 22, 0, 0, 0);
    const trends: AlertTrendSeries = {
      buckets: [0],
      max: 1,
      bucketSize: 24,
      bucketTimes: [start],
      rangeStart: start,
      rangeHours: 24,
    };
    const details = buildSelectedBucketDetails(0, trends, 'en-US');

    expect(details).not.toBeNull();
    expect(details!.start).toBe(start);
    expect(details!.end).toBe(start + 24 * MS_PER_HOUR);
    expect(details!.end - details!.start).toBe(24 * MS_PER_HOUR);
  });

  // --- rangeLabel formatting routed through formatAlertBucketRange --------

  it('produces a same-day range label (en-dash separator) for an intra-day bucket', () => {
    // Bucket 0 of a 24h/1h trend starts at Mar 21 12:00 UTC, ends 13:00 UTC.
    const trends = buildAlertTrends([], '24h', now);
    const details = buildSelectedBucketDetails(0, trends, 'en-US');

    expect(details).not.toBeNull();
    expect(typeof details!.rangeLabel).toBe('string');
    expect(details!.rangeLabel.length).toBeGreaterThan(0);
    // Same-day arm uses an en-dash, never the cross-day arrow.
    expect(details!.rangeLabel).toContain('\u2013'); // –
    expect(details!.rangeLabel).not.toContain('\u2192'); // →
    expect(details!.rangeLabel).toContain('Mar');
    expect(details!.rangeLabel).toContain('21');
  });

  it('produces a cross-day range label (arrow separator) when the bucket spans midnight', () => {
    // 6-hour bucket starting 22:00 Mar 22, ending 04:00 Mar 23 → different days.
    const crossDayStart = Date.UTC(2026, 2, 22, 22, 0, 0);
    const trends: AlertTrendSeries = {
      buckets: [0],
      max: 1,
      bucketSize: 6,
      bucketTimes: [crossDayStart],
      rangeStart: crossDayStart,
      rangeHours: 6,
    };
    const details = buildSelectedBucketDetails(0, trends, 'en-US');

    expect(details).not.toBeNull();
    expect(details!.start).toBe(crossDayStart);
    expect(details!.end).toBe(crossDayStart + 6 * MS_PER_HOUR);
    // Cross-day arm uses the arrow separator.
    expect(details!.rangeLabel).toContain('\u2192'); // →
    expect(details!.rangeLabel).not.toContain('\u2013'); // –
    // Both calendar days should appear.
    expect(details!.rangeLabel).toContain('22');
    expect(details!.rangeLabel).toContain('23');
  });

  it('passes the locale through to the underlying formatter (en-GB)', () => {
    const trends = buildAlertTrends([], '24h', now);
    const usDetails = buildSelectedBucketDetails(0, trends, 'en-US');
    const gbDetails = buildSelectedBucketDetails(0, trends, 'en-GB');

    expect(usDetails).not.toBeNull();
    expect(gbDetails).not.toBeNull();
    // Numeric start/end are locale-independent.
    expect(gbDetails!.start).toBe(usDetails!.start);
    expect(gbDetails!.end).toBe(usDetails!.end);
    // Both share the month abbreviation but may differ in time formatting.
    expect(gbDetails!.rangeLabel).toContain('Mar');
  });
});
