import { Accessor, createMemo } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  asStorageAlertRecord,
  EMPTY_STORAGE_ALERT_STATE,
  getStorageRecordAlertResourceIds,
  mergeStorageAlertRowState,
  type StorageAlertRowState,
} from '@/features/storageBackups/storageAlertState';
import { getAlertStyles } from '@/utils/alerts';

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
      byRecordId[record.id] = merged;
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
