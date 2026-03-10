import type { Alert } from '@/types/api';

type AlertIdentity = Pick<Alert, 'id' | 'legacyId'>;

export const getCanonicalAlertId = (alert: Pick<Alert, 'id'>): string => alert.id;

export const getLegacyAlertId = (alert: Pick<Alert, 'legacyId'>): string | undefined =>
  typeof alert.legacyId === 'string' && alert.legacyId.trim().length > 0
    ? alert.legacyId.trim()
    : undefined;

export const getAlertIdentifiers = (alert: AlertIdentity): string[] => {
  const identifiers = [getCanonicalAlertId(alert)];
  const legacyId = getLegacyAlertId(alert);
  if (legacyId && legacyId !== identifiers[0]) {
    identifiers.push(legacyId);
  }
  return identifiers;
};

export const matchesAlertIdentifier = (alert: AlertIdentity, candidate: string): boolean => {
  const trimmedCandidate = candidate.trim();
  if (!trimmedCandidate) {
    return false;
  }
  return getAlertIdentifiers(alert).includes(trimmedCandidate);
};
