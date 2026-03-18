import type { Alert } from '@/types/api';
import type { StorageRecord } from './models';

export type StorageAlertRowState = {
  hasAlert: boolean;
  alertCount: number;
  severity: 'critical' | 'warning' | null;
  hasUnacknowledgedAlert: boolean;
  unacknowledgedCount: number;
  acknowledgedCount: number;
  hasAcknowledgedOnlyAlert: boolean;
};

export const EMPTY_STORAGE_ALERT_STATE: StorageAlertRowState = {
  hasAlert: false,
  alertCount: 0,
  severity: null,
  hasUnacknowledgedAlert: false,
  unacknowledgedCount: 0,
  acknowledgedCount: 0,
  hasAcknowledgedOnlyAlert: false,
};

export const asStorageAlertRecord = (value: unknown): Record<string, Alert> => {
  if (!value) return {};

  if (Array.isArray(value)) {
    return value.reduce<Record<string, Alert>>((acc, alert) => {
      if (alert && typeof alert === 'object' && typeof (alert as Alert).id === 'string') {
        acc[(alert as Alert).id] = alert as Alert;
      }
      return acc;
    }, {});
  }

  if (typeof value !== 'object') return {};
  return value as Record<string, Alert>;
};

const severityWeight = (value: 'critical' | 'warning' | null): number => {
  if (value === 'critical') return 2;
  if (value === 'warning') return 1;
  return 0;
};

export const mergeStorageAlertRowState = (
  current: StorageAlertRowState,
  incoming: StorageAlertRowState,
): StorageAlertRowState => {
  const mergedHasUnacknowledged = current.hasUnacknowledgedAlert || incoming.hasUnacknowledgedAlert;
  const mergedAcknowledgedOnly =
    !mergedHasUnacknowledged &&
    (current.hasAcknowledgedOnlyAlert || incoming.hasAcknowledgedOnlyAlert);

  return {
    hasAlert: current.hasAlert || incoming.hasAlert,
    alertCount: current.alertCount + incoming.alertCount,
    severity:
      severityWeight(incoming.severity) > severityWeight(current.severity)
        ? incoming.severity
        : current.severity,
    hasUnacknowledgedAlert: mergedHasUnacknowledged,
    unacknowledgedCount: current.unacknowledgedCount + incoming.unacknowledgedCount,
    acknowledgedCount: current.acknowledgedCount + incoming.acknowledgedCount,
    hasAcknowledgedOnlyAlert: mergedAcknowledgedOnly,
  };
};

export const getStorageRecordAlertResourceIds = (record: StorageRecord): string[] => {
  const refs = record.refs || {};
  const details = (record.details || {}) as Record<string, unknown>;
  const detailNode = typeof details.node === 'string' ? details.node.trim() : '';
  const detailInstance =
    typeof refs.platformEntityId === 'string' ? refs.platformEntityId.trim() : '';
  const derivedLegacyId =
    detailInstance && detailNode && record.name
      ? `${detailInstance}-${detailNode}-${record.name}`
      : '';

  return Array.from(
    new Set(
      [record.id, refs.resourceId, derivedLegacyId]
        .filter((value): value is string => typeof value === 'string')
        .map((value) => value.trim())
        .filter((value) => value.length > 0),
    ),
  );
};
