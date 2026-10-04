import type { Alert } from '@/types/api';

// Synthetic payloads shaped like Manager.RaiseSystemAlert. No native services.
export const SYSTEM_ALERT_TYPES = [
  'backup-evaluation',
  'notification-delivery',
  'deadman-delivery',
  'deadman-monitoring-stalled',
  'deadman-interruption',
  'deadman-state',
] as const;

export function makeSystemAlert(
  type: string = 'backup-evaluation',
  overrides: Partial<Alert> = {},
): Alert {
  return {
    id: `pulse-system-${type}`,
    type,
    resourceId: '',
    resourceName: 'Pulse',
    node: '',
    instance: '',
    level: 'warning',
    message:
      "Backup-age alerts were not evaluated because recovery data could not be read. Existing backup alerts have been kept; check Pulse's logs.",
    value: 0,
    threshold: 0,
    startTime: '2026-10-04T15:00:00Z',
    lastSeen: '2026-10-04T15:01:00Z',
    acknowledged: false,
    metadata: { systemAlert: true, systemAlertType: type },
    ...overrides,
  };
}
