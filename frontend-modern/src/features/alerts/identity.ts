import type { Alert } from '@/types/api';

type AlertIdentity = Pick<Alert, 'id'>;

export const getCanonicalAlertId = (alert: Pick<Alert, 'id'>): string => alert.id;

export const getAlertIdentifiers = (alert: AlertIdentity): string[] => [getCanonicalAlertId(alert)];

export const matchesAlertIdentifier = (alert: AlertIdentity, candidate: string): boolean => {
  const trimmedCandidate = candidate.trim();
  if (!trimmedCandidate) {
    return false;
  }
  return getCanonicalAlertId(alert) === trimmedCandidate;
};
