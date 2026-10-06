import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import {
  describeStorageAlertHeadline,
  getStorageRowAlertPresentation,
  pickStorageHeadlineAlert,
} from '@/features/storageBackups/storageRowAlertPresentation';

const makeAlert = (overrides: Partial<Alert> = {}): Alert =>
  ({
    id: 'alert-1',
    type: 'usage',
    level: 'warning',
    resourceId: 'storage-1',
    resourceName: 'tank',
    node: 'pve1',
    instance: 'pve1',
    message: 'Storage at 88.9%',
    value: 88.9,
    threshold: 85,
    startTime: '2026-10-04T12:00:00Z',
    acknowledged: false,
    ...overrides,
  }) as Alert;

describe('storageRowAlertPresentation', () => {
  it('returns unacknowledged critical row styling canonically', () => {
    const result = getStorageRowAlertPresentation({
      alertState: {
        hasAlert: true,
        alertCount: 1,
        severity: 'critical',
        hasUnacknowledgedAlert: true,
        unacknowledgedCount: 1,
        acknowledgedCount: 0,
        hasAcknowledgedOnlyAlert: false,
      },
      parentNodeOnline: true,
      isExpanded: false,
      isResourceHighlighted: false,
    });

    expect(result.rowClass).toContain('bg-red-50');
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_#ef4444]');
    expect(result.dataAlertState).toBe('unacknowledged');
  });

  it('returns acknowledged-only row styling canonically', () => {
    const result = getStorageRowAlertPresentation({
      alertState: {
        hasAlert: true,
        alertCount: 1,
        severity: 'warning',
        hasUnacknowledgedAlert: false,
        unacknowledgedCount: 0,
        acknowledgedCount: 1,
        hasAcknowledgedOnlyAlert: true,
      },
      parentNodeOnline: true,
      isExpanded: false,
      isResourceHighlighted: false,
    });

    expect(result.rowClass).toContain('bg-surface-alt');
    expect(result.rowClass).toContain('shadow-[inset_4px_0_0_0_rgba(156,163,175,0.8)]');
    expect(result.dataAlertState).toBe('acknowledged');
  });

  it('does not claim usage is over the limit while the alert holds below it', () => {
    const holding = (phase: 'latched' | 'recovering') =>
      makeAlert({
        metricStatus: {
          phase,
          value: phase === 'latched' ? 83 : 79,
          unit: '%',
          observedAt: '2026-10-04T12:30:00Z',
          trigger: 85,
          recovery: 80,
          recoveryDelaySeconds: 300,
        },
      });
    expect(describeStorageAlertHeadline(holding('latched'))).toBe(
      'Under 85% limit, clears at 80% or lower',
    );
    expect(describeStorageAlertHeadline(holding('latched'), { compact: true })).toBe('Clears ≤80%');
    expect(describeStorageAlertHeadline(holding('recovering'))).toBe(
      'Recovering, clears at 80% or lower',
    );
    expect(describeStorageAlertHeadline(holding('recovering'), { compact: true })).toBe(
      'Recovering',
    );
  });

  it('explains a highlighted row with a fill forecast, a crossed limit or the alert text', () => {
    const forecast = (days: number) =>
      makeAlert({ threshold: 100, metadata: { forecastDaysToFull: days } });
    expect(describeStorageAlertHeadline(forecast(3.4))).toBe('Full in ~3 days');
    expect(describeStorageAlertHeadline(forecast(3.4), { compact: true })).toBe('Full in ~3d');
    expect(describeStorageAlertHeadline(forecast(1.2))).toBe('Full in ~1 day');
    expect(describeStorageAlertHeadline(forecast(0.4))).toBe('Full within a day');
    expect(describeStorageAlertHeadline(forecast(0.4), { compact: true })).toBe('Full <1d');
    expect(describeStorageAlertHeadline(makeAlert())).toBe('Over 85% usage limit');
    expect(describeStorageAlertHeadline(makeAlert(), { compact: true })).toBe('Over 85%');
    expect(
      describeStorageAlertHeadline(
        makeAlert({ type: 'zfs-pool-state', threshold: 0, message: 'Pool archive is DEGRADED' }),
      ),
    ).toBe('Pool archive is DEGRADED');
  });

  it('explains the row with its most severe, most recent open alert', () => {
    const olderWarning = makeAlert({ id: 'w1', startTime: '2026-10-04T10:00:00Z' });
    const newerWarning = makeAlert({ id: 'w2', startTime: '2026-10-04T11:00:00Z' });
    const ackedCritical = makeAlert({ id: 'c1', level: 'critical', acknowledged: true });
    expect(pickStorageHeadlineAlert([olderWarning, ackedCritical, newerWarning])?.id).toBe('w2');
    expect(
      pickStorageHeadlineAlert([olderWarning, makeAlert({ id: 'c2', level: 'critical' })])?.id,
    ).toBe('c2');
    expect(pickStorageHeadlineAlert([ackedCritical])).toBeNull();
  });
});
