import { Accessor, createMemo } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Alert } from '@/types/api';
import {
  asStorageAlertRecord,
  EMPTY_STORAGE_ALERT_STATE,
  getStorageRecordAlertResourceIds,
  mergeStorageAlertRowState,
  type StorageAlertRowState,
} from '@/features/storageBackups/storageAlertState';
import { getAlertStyles, getAlertsForResource } from '@/utils/alerts';
import {
  describeStorageAlertHeadline,
  pickStorageHeadlineAlert,
} from '@/features/storageBackups/storageRowAlertPresentation';

type UseStorageAlertStateOptions = {
  records: Accessor<StorageRecord[]>;
  activeAlerts: Accessor<unknown> | unknown;
  alertsEnabled: Accessor<boolean>;
};

const ALERT_LEVEL_RANK: Record<Alert['level'], number> = { critical: 0, warning: 1, info: 2 };

const sortAlertsBySeverity = (alerts: Alert[]): Alert[] =>
  [...alerts].sort(
    (a, b) =>
      (ALERT_LEVEL_RANK[a.level] ?? 3) - (ALERT_LEVEL_RANK[b.level] ?? 3) ||
      Date.parse(a.startTime) - Date.parse(b.startTime),
  );

export const useStorageAlertState = (options: UseStorageAlertStateOptions) => {
  // One match per row: the alerts that tint and explain a row are the alerts
  // its drawer lists under Needs attention.
  const alertsByRecordId = createMemo<Record<string, Alert[]>>(() => {
    const records = options.records();
    const activeAlerts = asStorageAlertRecord(
      typeof options.activeAlerts === 'function'
        ? (options.activeAlerts as Accessor<unknown>)()
        : options.activeAlerts,
    );
    const enabled = options.alertsEnabled();
    const byRecordId: Record<string, Alert[]> = {};
    for (const record of records) {
      byRecordId[record.id] = sortAlertsBySeverity(
        getAlertsForResource(getStorageRecordAlertResourceIds(record), activeAlerts, enabled),
      );
    }
    return byRecordId;
  });

  const alertStateByRecordId = createMemo<Record<string, StorageAlertRowState>>(() => {
    const records = options.records();
    const activeAlerts = asStorageAlertRecord(
      typeof options.activeAlerts === 'function'
        ? (options.activeAlerts as Accessor<unknown>)()
        : options.activeAlerts,
    );
    const enabled = options.alertsEnabled();
    const byRecordId: Record<string, StorageAlertRowState> = {};

    for (const record of records) {
      let merged = EMPTY_STORAGE_ALERT_STATE;
      const candidateIds = getStorageRecordAlertResourceIds(record);
      for (const resourceId of candidateIds) {
        const styles = getAlertStyles(resourceId, activeAlerts, enabled);
        merged = mergeStorageAlertRowState(merged, {
          hasAlert: styles.hasAlert,
          alertCount: styles.alertCount,
          severity: styles.severity,
          hasUnacknowledgedAlert: styles.hasUnacknowledgedAlert,
          unacknowledgedCount: styles.unacknowledgedCount,
          acknowledgedCount: styles.acknowledgedCount,
          hasAcknowledgedOnlyAlert: styles.hasAcknowledgedOnlyAlert,
        });
      }
      const headlineAlert = pickStorageHeadlineAlert(alertsByRecordId()[record.id] ?? []);
      byRecordId[record.id] = {
        ...merged,
        headline: headlineAlert ? describeStorageAlertHeadline(headlineAlert) : null,
        headlineCompact: headlineAlert
          ? describeStorageAlertHeadline(headlineAlert, { compact: true })
          : null,
      };
    }

    return byRecordId;
  });

  const getRecordAlertState = (recordId: string): StorageAlertRowState =>
    alertStateByRecordId()[recordId] || EMPTY_STORAGE_ALERT_STATE;

  const getRecordAlerts = (recordId: string): Alert[] => alertsByRecordId()[recordId] ?? [];

  return {
    alertStateByRecordId,
    getRecordAlertState,
    getRecordAlerts,
  };
};
