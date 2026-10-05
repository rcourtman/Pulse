import { Accessor, createMemo } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
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

export const useStorageAlertState = (options: UseStorageAlertStateOptions) => {
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
      const headlineAlert = pickStorageHeadlineAlert(
        getAlertsForResource(candidateIds, activeAlerts, enabled),
      );
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

  return {
    alertStateByRecordId,
    getRecordAlertState,
  };
};
