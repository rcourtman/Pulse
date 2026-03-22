import { describe, expect, it } from 'vitest';

import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

import {
  buildAlertHistoryItems,
  buildAlertHistoryParams,
  buildAlertTrends,
  buildSelectedBucketDetails,
  formatAlertHistoryDuration,
  getAlertBucketDurationLabel,
} from '../alertHistoryModel';

describe('alertHistoryModel', () => {
  it('builds canonical history params for each range', () => {
    const now = Date.UTC(2026, 2, 22, 12, 0, 0);

    expect(buildAlertHistoryParams('24h', now)).toEqual({
      limit: 2000,
      startTime: new Date(now - 24 * 60 * 60 * 1000).toISOString(),
    });
    expect(buildAlertHistoryParams('7d', now)).toEqual({
      limit: 10000,
      startTime: new Date(now - 7 * 24 * 60 * 60 * 1000).toISOString(),
    });
    expect(buildAlertHistoryParams('30d', now)).toEqual({
      limit: 10000,
      startTime: new Date(now - 30 * 24 * 60 * 60 * 1000).toISOString(),
    });
    expect(buildAlertHistoryParams('all', now)).toEqual({ limit: 0 });
  });

  it('formats durations across minute, hour, and day boundaries', () => {
    const start = '2026-03-22T10:00:00.000Z';
    expect(formatAlertHistoryDuration(start, '2026-03-22T10:45:00.000Z')).toBe('45m');
    expect(formatAlertHistoryDuration(start, '2026-03-22T12:15:00.000Z')).toBe('2h 15m');
    expect(formatAlertHistoryDuration(start, '2026-03-24T12:00:00.000Z')).toBe('2d 2h');
  });

  it('builds history items using canonical resource type resolution', () => {
    const resource = {
      id: 'resource-1',
      name: 'vm-101',
      displayName: 'vm-101',
      type: 'vm',
    } as unknown as Resource;
    const activeAlerts: Record<string, Alert> = {
      'alert-1': {
        id: 'alert-1',
        type: 'cpu',
        level: 'critical',
        resourceId: 'resource-1',
        resourceName: 'vm-101',
        node: 'px1',
        message: 'CPU high',
        startTime: '2026-03-22T09:00:00.000Z',
        lastSeen: '2026-03-22T09:15:00.000Z',
        value: 90,
        threshold: 80,
        acknowledged: false,
      } as Alert,
    };

    const items = buildAlertHistoryItems({
      activeAlerts,
      alertHistory: [],
      getResource: (resourceId) => (resourceId === 'resource-1' ? resource : undefined),
      allResources: [resource],
      now: Date.UTC(2026, 2, 22, 10, 0, 0),
    });

    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      id: 'alert-1',
      resourceType: 'VM',
      status: 'active',
      title: 'CPU',
    });
  });

  it('builds trends and selected bucket details from filtered alerts', () => {
    const alerts = [
      { id: 'a', startTime: '2026-03-22T08:00:00.000Z' },
      { id: 'b', startTime: '2026-03-22T09:00:00.000Z' },
      { id: 'c', startTime: '2026-03-22T09:30:00.000Z' },
    ] as Array<{ id: string; startTime: string }>;
    const trends = buildAlertTrends(alerts as any, '24h', Date.UTC(2026, 2, 22, 10, 0, 0));

    expect(trends.bucketSize).toBe(1);
    expect(trends.buckets.reduce((sum, value) => sum + value, 0)).toBe(3);
    expect(getAlertBucketDurationLabel(trends.bucketSize)).toBe('1 hour');

    const details = buildSelectedBucketDetails(1, trends, 'en-GB');
    expect(details).not.toBeNull();
    expect(details?.rangeLabel).toContain('Mar');
  });
});
