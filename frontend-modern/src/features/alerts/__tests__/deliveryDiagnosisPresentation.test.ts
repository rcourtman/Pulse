import { describe, expect, it } from 'vitest';

import type { AlertDeliveryDiagnosis } from '@/types/api';

import {
  describeAlertDeliveryStatus,
  describeAlertEventReason,
} from '../deliveryDiagnosisPresentation';

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

  it('shows dispatch evidence without claiming destination success', () => {
    const diagnosis = baseDiagnosis({ lastNotified: '2026-08-26T10:15:00Z' });
    const line = describeAlertDeliveryStatus(diagnosis, false);
    expect(line?.tone).toBe('muted');
    expect(line?.label).toMatch(/^Dispatch requested /);
  });

  it('shows dispatch without promising another send when cooldown has no next time', () => {
    const line = describeAlertDeliveryStatus(
      baseDiagnosis({
        status: 'suppressed',
        reason: 'cooldown',
        lastNotified: '2026-08-26T10:15:00Z',
      }),
      false,
    );
    expect(line?.label).toMatch(/^Dispatch requested /);
    expect(line?.label).not.toContain('next');
  });

  it.each([undefined, '', 'invalid'])(
    'does not invent dispatch for timestamp %s',
    (lastNotified) => {
      expect(describeAlertDeliveryStatus(baseDiagnosis({ lastNotified }), false)?.label).toBe(
        'Notification pending',
      );
      expect(
        describeAlertDeliveryStatus(
          baseDiagnosis({
            status: 'suppressed',
            reason: 'cooldown',
            lastNotified,
          }),
          false,
        )?.label,
      ).toBe('Waiting for cooldown');
    },
  );

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
    expect(line?.label).toMatch(/^Dispatch requested .* — next eligible /);
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

// A historic dispatch must not hide the reason a notification is held now.
describe('current holds take precedence over historic dispatch', () => {
  it.each([
    ['monitor_only', 'Monitor-only — no notifications', 'muted'],
    ['notifications_disabled', 'Notifications are turned off', 'attention'],
    ['notifications_inactive', 'Notification delivery not turned on', 'attention'],
    ['rate_limited', 'Hourly notification limit reached', 'attention'],
    ['flapping', 'Flapping — notifications paused', 'attention'],
    ['suppression_window', 'Notifications paused', 'attention'],
  ] as const)('preserves %s with a previous dispatch', (reason, label, tone) => {
    expect(
      describeAlertDeliveryStatus(
        baseDiagnosis({
          status: 'suppressed',
          reason,
          lastNotified: '2026-09-07T06:00:00Z',
        }),
        false,
      ),
    ).toEqual({ label, tone });
  });

  it.each([undefined, '', 'invalid'])(
    'keeps quiet hours visible without a valid replay time (%s)',
    (quietHoursReplayAt) => {
      expect(
        describeAlertDeliveryStatus(
          baseDiagnosis({
            status: 'deferred',
            reason: 'quiet_hours:performance',
            lastNotified: '2026-09-07T06:00:00Z',
            quietHoursReplayAt,
          }),
          false,
        ),
      ).toEqual({ label: 'Quiet hours — notifies later', tone: 'muted' });
    },
  );

  it('respects backend acknowledgement even before the card badge updates', () => {
    expect(
      describeAlertDeliveryStatus(
        baseDiagnosis({
          status: 'suppressed',
          reason: 'acknowledged',
          lastNotified: '2026-09-07T06:00:00Z',
        }),
        false,
      ),
    ).toBeNull();
  });
});

describe('describeAlertEventReason', () => {
  it.each([
    ['acknowledged', 'Acknowledged'],
    ['flapping', 'Flapping'],
    ['notifications_inactive', 'Delivery not turned on'],
    ['notifications_disabled', 'Notifications off'],
    ['monitor_only', 'Monitor-only'],
    ['quiet_hours', 'Quiet hours'],
    ['rate_limited', 'Hourly limit'],
    ['suppression_window', 'Paused'],
    ['cooldown', 'Cooldown'],
  ])('names %s without classifying it as a delivery failure', (reason, label) => {
    expect(describeAlertEventReason(reason)).toBe(label);
    expect(describeAlertEventReason(`${reason}:performance`)).toBe(label);
  });

  it('retains an unknown reason including its detail', () => {
    expect(describeAlertEventReason('future_hold:detail')).toBe('future_hold:detail');
  });

  it.each([undefined, ''])('uses a neutral fallback for missing reason %s', (reason) => {
    expect(describeAlertEventReason(reason)).toBe('Held');
  });
});
