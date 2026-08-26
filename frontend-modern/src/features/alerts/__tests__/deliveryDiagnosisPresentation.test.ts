import { describe, expect, it } from 'vitest';

import type { AlertDeliveryDiagnosis } from '@/types/api';

import { describeAlertDeliveryStatus } from '../deliveryDiagnosisPresentation';

const baseDiagnosis = (overrides: Partial<AlertDeliveryDiagnosis>): AlertDeliveryDiagnosis => ({
  alertIdentifier: 'a1',
  alertId: 'a1',
  trackingKey: 'node/a1/cpu',
  status: 'would_send',
  reason: 'ready',
  message: 'Alert delivery is currently eligible for notification delivery.',
  alertType: 'cpu',
  level: 'warning',
  notificationsEnabled: true,
  activationState: 'active',
  cooldownMinutes: 5,
  maxAlertsHour: 10,
  recentAlertsInHour: 0,
  flappingActive: false,
  flappingHistoryInWindow: 0,
  flappingThreshold: 5,
  flappingWindowSeconds: 300,
  ...overrides,
});

describe('describeAlertDeliveryStatus', () => {
  it('returns null without a diagnosis', () => {
    expect(describeAlertDeliveryStatus(undefined, false)).toBeNull();
  });

  it('returns null for acknowledged alerts regardless of status', () => {
    const diagnosis = baseDiagnosis({ status: 'suppressed', reason: 'cooldown' });
    expect(describeAlertDeliveryStatus(diagnosis, true)).toBeNull();
  });

  it('shows the notified time when eligible and already notified', () => {
    const diagnosis = baseDiagnosis({ lastNotified: '2026-08-26T10:15:00Z' });
    const line = describeAlertDeliveryStatus(diagnosis, false);
    expect(line?.tone).toBe('muted');
    expect(line?.label).toMatch(/^Notified /);
  });

  it('shows pending when eligible but never notified', () => {
    const line = describeAlertDeliveryStatus(baseDiagnosis({}), false);
    expect(line).toEqual({ label: 'Notification pending', tone: 'muted' });
  });

  it('shows quiet hours with the replay time for deferred alerts', () => {
    const diagnosis = baseDiagnosis({
      status: 'deferred',
      reason: 'quiet_hours:performance',
      quietHoursReplayAt: '2026-08-27T07:00:00Z',
    });
    const line = describeAlertDeliveryStatus(diagnosis, false);
    expect(line?.tone).toBe('muted');
    expect(line?.label).toMatch(/^Quiet hours — notifies /);
  });

  it('treats cooldown as healthy and shows the next eligible time', () => {
    const diagnosis = baseDiagnosis({
      status: 'suppressed',
      reason: 'cooldown',
      lastNotified: '2026-08-26T10:15:00Z',
      nextEligibleAt: '2026-08-26T10:20:00Z',
    });
    const line = describeAlertDeliveryStatus(diagnosis, false);
    expect(line?.tone).toBe('muted');
    expect(line?.label).toMatch(/^Notified .* — next /);
  });

  it.each([
    ['rate_limited', 'Hourly notification limit reached'],
    ['flapping', 'Flapping — notifications paused'],
    ['notifications_disabled', 'Notifications are turned off'],
    ['notifications_inactive', 'Notification delivery not turned on'],
  ] as const)('marks %s with the attention tone', (reason, label) => {
    const diagnosis = baseDiagnosis({ status: 'suppressed', reason });
    expect(describeAlertDeliveryStatus(diagnosis, false)).toEqual({ label, tone: 'attention' });
  });

  it('shows the pause end time for suppression windows', () => {
    const diagnosis = baseDiagnosis({
      status: 'suppressed',
      reason: 'suppression_window',
      suppressedUntil: '2026-08-26T11:00:00Z',
    });
    const line = describeAlertDeliveryStatus(diagnosis, false);
    expect(line?.tone).toBe('attention');
    expect(line?.label).toMatch(/^Notifications paused until /);
  });

  it('stays silent on unknown reasons', () => {
    const diagnosis = baseDiagnosis({ status: 'suppressed', reason: 'future_reason' });
    expect(describeAlertDeliveryStatus(diagnosis, false)).toBeNull();
  });
});
