import type { Alert } from '@/types/api';

/**
 * Mirror the existing system-alert identity/metadata contract. Names and alert
 * types are not scope evidence: a monitored machine can also be called Pulse,
 * and future system conditions must not need a frontend type allowlist.
 */
export function isPulseSystemAlert(alert: Pick<Alert, 'id' | 'metadata'>): boolean {
  return alert.id.startsWith('pulse-system-') || alert.metadata?.systemAlert === true;
}
