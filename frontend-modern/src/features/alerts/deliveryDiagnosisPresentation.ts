import type { AlertDeliveryDiagnosis } from '@/types/api';

export interface AlertDeliveryStatusLine {
  label: string;
  // 'attention' marks held notifications the user may not expect; 'muted'
  // marks healthy or user-chosen states.
  tone: 'muted' | 'attention';
}

const formatShortTime = (value?: string): string | null => {
  if (!value) return null;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

// describeAlertDeliveryStatus turns a delivery diagnosis into the one-line
// notification status shown on the alert card. Returns null when the card
// should show no line (acknowledged alerts already carry a badge, and an
// unknown reason is better silent than wrong).
export const describeAlertDeliveryStatus = (
  diagnosis: AlertDeliveryDiagnosis | undefined,
  acknowledged: boolean,
): AlertDeliveryStatusLine | null => {
  if (!diagnosis || acknowledged) return null;

  const reason = (diagnosis.reason || '').split(':')[0];

  if (diagnosis.status === 'would_send') {
    const notifiedAt = formatShortTime(diagnosis.lastNotified);
    if (notifiedAt) return { label: `Notified ${notifiedAt}`, tone: 'muted' };
    return { label: 'Notification pending', tone: 'muted' };
  }

  if (diagnosis.status === 'deferred') {
    const replayAt = formatShortTime(diagnosis.quietHoursReplayAt);
    return {
      label: replayAt ? `Quiet hours — notifies ${replayAt}` : 'Quiet hours — notifies later',
      tone: 'muted',
    };
  }

  switch (reason) {
    case 'acknowledged':
      return null;
    case 'cooldown': {
      const notifiedAt = formatShortTime(diagnosis.lastNotified);
      const nextAt = formatShortTime(diagnosis.nextEligibleAt);
      if (notifiedAt && nextAt) {
        return { label: `Notified ${notifiedAt} — next ${nextAt}`, tone: 'muted' };
      }
      if (notifiedAt) return { label: `Notified ${notifiedAt}`, tone: 'muted' };
      return { label: 'Waiting for cooldown', tone: 'muted' };
    }
    case 'rate_limited':
      return { label: 'Hourly notification limit reached', tone: 'attention' };
    case 'flapping':
      return { label: 'Flapping — notifications paused', tone: 'attention' };
    case 'suppression_window': {
      const until = formatShortTime(diagnosis.suppressedUntil);
      return {
        label: until ? `Notifications paused until ${until}` : 'Notifications paused',
        tone: 'attention',
      };
    }
    case 'notifications_disabled':
      return { label: 'Notifications are turned off', tone: 'attention' };
    case 'notifications_inactive':
      return { label: 'Notification delivery not turned on', tone: 'attention' };
    case 'monitor_only':
      return { label: 'Monitor-only — no notifications', tone: 'muted' };
    default:
      return null;
  }
};
