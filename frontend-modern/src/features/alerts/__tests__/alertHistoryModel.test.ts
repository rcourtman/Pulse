import { describe, expect, it, vi } from 'vitest';

import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

import {
  applyAlertHistoryWindow,
  buildAlertHistoryItems,
  buildAlertHistoryParams,
  buildAlertTrends,
  buildSelectedBucketDetails,
  formatAlertHistoryDuration,
  getAlertBucketDurationLabel,
  getAlertHistoryRowCopy,
  type HistoryItem,
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

  it('keeps a stable id order for alerts sharing a startTime (#1218)', () => {
    const sharedStart = '2026-03-22T09:00:00.000Z';
    const items = [
      { id: 'zeta', startTime: sharedStart },
      { id: 'alpha', startTime: sharedStart },
      { id: 'newest', startTime: '2026-03-22T09:30:00.000Z' },
      { id: 'mid', startTime: sharedStart },
    ] as HistoryItem[];
    const now = Date.UTC(2026, 2, 22, 10, 0, 0);
    const trends = buildAlertTrends(items, '24h', now);

    const sorted = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: '24h',
      selectedBarIndex: null,
      trends,
      now,
    });

    expect(sorted.map((item) => item.id)).toEqual(['newest', 'alpha', 'mid', 'zeta']);
  });
});

// minipc on 2026-10-06: the node temperature alert opened at 95°C, the node
// then read 78°C under its 80°C trigger, and the History row for the open
// alert still said "Node temperature at 95.0°C".
describe('alert history rows for held threshold alerts', () => {
  const NOW = Date.parse('2026-10-06T19:00:00Z');
  const heldAlert = (status?: Partial<NonNullable<Alert['metricStatus']>>): Alert =>
    ({
      id: 'homelab-minipc::metric-threshold:temperature',
      type: 'temperature',
      level: 'warning',
      resourceId: 'homelab-minipc',
      resourceName: 'minipc',
      node: 'minipc',
      instance: 'homelab',
      message: 'Node temperature at 95.0°C',
      value: 95,
      threshold: 80,
      startTime: '2026-10-06T18:30:00Z',
      lastSeen: '2026-10-06T18:40:00Z',
      acknowledged: false,
      metricStatus: status
        ? {
            phase: 'latched',
            value: 78,
            unit: '°C',
            observedAt: new Date(NOW - 10_000).toISOString(),
            trigger: 80,
            recovery: 75,
            recoveryDelaySeconds: 300,
            ...status,
          }
        : undefined,
    }) as Alert;
  const build = (activeAlerts: Record<string, Alert>, alertHistory: Alert[] = []) =>
    buildAlertHistoryItems({
      activeAlerts,
      alertHistory,
      getResource: () => undefined,
      allResources: [],
      now: NOW,
    });

  it('leads the open row with the live reading and keeps the breach for hover', () => {
    const alert = heldAlert({ phase: 'latched', value: 78 });
    const [item] = build({ [alert.id]: alert });

    // The record itself is unchanged: search and the Assistant handoff read it.
    expect(item.description).toBe('Node temperature at 95.0°C');
    const copy = getAlertHistoryRowCopy(item, () => NOW);
    expect(copy.text).toBe('Temperature 78°C now, back under the 80°C alert level');
    expect(copy.detail).toBe(
      'Stays open until it reaches 75°C or lower and stays there for 5 minutes.',
    );
    expect(copy.lastBreach).toMatch(/^Last reading at or above 80°C: 95°C, /);
    expect(copy.title.split('\n')).toEqual([copy.text, copy.detail, copy.lastBreach]);
  });

  it('says a recovering open row is recovering, with its progress', () => {
    const alert = heldAlert({ phase: 'recovering', value: 72, recoveryElapsedSeconds: 130 });
    const copy = getAlertHistoryRowCopy(build({ [alert.id]: alert })[0], () => NOW);
    expect(copy.text).toBe('Temperature 72°C now, recovering');
    expect(copy.detail).toBe('Clears after 5 minutes at 75°C or lower, 2 minutes so far.');
  });

  it('stops calling a reading "now" once evaluations stop', () => {
    const alert = heldAlert({ phase: 'latched', value: 78 });
    const [item] = build({ [alert.id]: alert });
    const copy = getAlertHistoryRowCopy(item, () => NOW + 15 * 60 * 1000);
    expect(copy.text).toMatch(/^Last reading: Temperature 78°C, /);
    expect(copy.text).not.toContain('now');
  });

  it('keeps the recorded message on closed rows and open rows without a live status', () => {
    const open = heldAlert();
    const closed = {
      ...heldAlert({ phase: 'latched', value: 78 }),
      id: 'homelab-minipc::metric-threshold:cpu',
      message: 'Resolved: Node temperature at 74.0°C',
    };
    const clock = vi.fn(() => NOW);
    const items = build({ [open.id]: open }, [closed]);
    const openItem = items.find((item) => item.id === open.id)!;
    const closedItem = items.find((item) => item.id === closed.id)!;

    expect(closedItem.status).toBe('resolved');
    expect(closedItem.liveAlert).toBeUndefined();
    expect(getAlertHistoryRowCopy(closedItem, clock)).toEqual({
      text: 'Resolved: Node temperature at 74.0°C',
      title: 'Resolved: Node temperature at 74.0°C',
    });
    // Closed rows never read the clock, so they do not re-render on its tick.
    expect(clock).not.toHaveBeenCalled();
    expect(getAlertHistoryRowCopy(openItem, clock)).toEqual({
      text: 'Node temperature at 95.0°C',
      title: 'Node temperature at 95.0°C',
    });
  });

  it('gives Pulse system alerts no live reading', () => {
    const alert = {
      ...heldAlert({ phase: 'latched', value: 78 }),
      id: 'pulse-system::storage',
      metadata: { systemAlert: true },
    } as Alert;
    const [item] = build({ [alert.id]: alert });
    expect(item.systemAlert).toBe(true);
    expect(item.liveAlert).toBeUndefined();
    expect(getAlertHistoryRowCopy(item, () => NOW).text).toBe('Node temperature at 95.0°C');
  });
});
