import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import { logger } from '@/utils/logger';

import { getCanonicalAlertId } from './identity';

export interface UseAlertAcknowledgementStateProps {
  alerts: Accessor<Alert[]>;
  updateAlert?: (alertIdentifier: string, updates: Partial<Alert>) => void;
  allowRestore?: boolean;
}

type AlertAcknowledgementOverride = Pick<Alert, 'acknowledged' | 'ackTime' | 'ackUser'>;

export function useAlertAcknowledgementState(props: UseAlertAcknowledgementStateProps) {
  const [processingAlerts, setProcessingAlerts] = createSignal<Set<string>>(new Set());
  const [bulkAckProcessing, setBulkAckProcessing] = createSignal(false);
  const [acknowledgementOverrides, setAcknowledgementOverrides] = createSignal<
    Record<string, AlertAcknowledgementOverride>
  >({});
  const processingReleaseTimers = new Map<string, ReturnType<typeof setTimeout>>();

  const clearProcessingReleaseTimer = (alertIdentifier: string) => {
    const timer = processingReleaseTimers.get(alertIdentifier);
    if (timer === undefined) {
      return;
    }
    clearTimeout(timer);
    processingReleaseTimers.delete(alertIdentifier);
  };

  onCleanup(() => {
    processingReleaseTimers.forEach((timer) => clearTimeout(timer));
    processingReleaseTimers.clear();
  });

  createEffect(() => {
    const alertsByIdentifier = new Map(
      props.alerts().map((alert) => [getCanonicalAlertId(alert), alert] as const),
    );

    setAcknowledgementOverrides((previous) => {
      let changed = false;
      const next = { ...previous };

      for (const [alertIdentifier, override] of Object.entries(previous)) {
        const alert = alertsByIdentifier.get(alertIdentifier);
        if (!alert || alert.acknowledged === override.acknowledged) {
          delete next[alertIdentifier];
          changed = true;
        }
      }

      return changed ? next : previous;
    });
  });

  const effectiveAlerts = createMemo(() =>
    props.alerts().map((alert) => {
      const override = acknowledgementOverrides()[getCanonicalAlertId(alert)];
      return override ? { ...alert, ...override } : alert;
    }),
  );

  const unacknowledgedAlerts = createMemo(() =>
    effectiveAlerts().filter((alert) => !alert.acknowledged),
  );

  const applyAlertUpdate = (alertIdentifier: string, updates: AlertAcknowledgementOverride) => {
    setAcknowledgementOverrides((previous) => ({
      ...previous,
      [alertIdentifier]: updates,
    }));
    props.updateAlert?.(alertIdentifier, updates);
  };

  const releaseAlertProcessing = (alertIdentifier: string) => {
    clearProcessingReleaseTimer(alertIdentifier);
    const timer = setTimeout(() => {
      processingReleaseTimers.delete(alertIdentifier);
      setProcessingAlerts((previous) => {
        const next = new Set(previous);
        next.delete(alertIdentifier);
        return next;
      });
    }, 1500);
    processingReleaseTimers.set(alertIdentifier, timer);
  };

  const handleAlertAcknowledgement = async (alert: Alert) => {
    const alertIdentifier = getCanonicalAlertId(alert);
    if (processingAlerts().has(alertIdentifier)) {
      return;
    }

    const currentAlert =
      effectiveAlerts().find((entry) => getCanonicalAlertId(entry) === alertIdentifier) ?? alert;
    const wasAcknowledged = currentAlert.acknowledged;
    if (wasAcknowledged && !props.allowRestore) {
      return;
    }

    setProcessingAlerts((previous) => new Set(previous).add(alertIdentifier));

    try {
      if (wasAcknowledged) {
        await AlertsAPI.unacknowledge(alertIdentifier);
        applyAlertUpdate(alertIdentifier, {
          acknowledged: false,
          ackTime: undefined,
          ackUser: undefined,
        });
        notificationStore.success('Alert restored');
      } else {
        await AlertsAPI.acknowledge(alertIdentifier);
        applyAlertUpdate(alertIdentifier, {
          acknowledged: true,
          ackTime: new Date().toISOString(),
          ackUser: undefined,
        });
        notificationStore.success('Alert acknowledged');
      }
    } catch (error) {
      logger.error(
        `Failed to ${wasAcknowledged ? 'unacknowledge' : 'acknowledge'} alert:`,
        error,
      );
      notificationStore.error(`Failed to ${wasAcknowledged ? 'restore' : 'acknowledge'} alert`);
    } finally {
      releaseAlertProcessing(alertIdentifier);
    }
  };

  const handleBulkAcknowledge = async () => {
    if (bulkAckProcessing()) {
      return;
    }

    const pendingAlerts = unacknowledgedAlerts();
    if (pendingAlerts.length === 0) {
      return;
    }

    setBulkAckProcessing(true);
    try {
      const result = await AlertsAPI.bulkAcknowledge(
        pendingAlerts.map((alert) => getCanonicalAlertId(alert)),
      );
      const acknowledgedAt = new Date().toISOString();
      const successes = result.results.filter((entry) => entry.success);
      const failures = result.results.filter((entry) => !entry.success);

      successes.forEach((entry) => {
        applyAlertUpdate(entry.alertIdentifier, {
          acknowledged: true,
          ackTime: acknowledgedAt,
          ackUser: undefined,
        });
      });

      if (successes.length > 0) {
        notificationStore.success(
          `Acknowledged ${successes.length} ${successes.length === 1 ? 'alert' : 'alerts'}.`,
        );
      }

      if (failures.length > 0) {
        notificationStore.error(
          `Failed to acknowledge ${failures.length} ${failures.length === 1 ? 'alert' : 'alerts'}.`,
        );
      }
    } catch (error) {
      logger.error('Bulk acknowledge failed', error);
      notificationStore.error('Failed to acknowledge alerts');
    } finally {
      setBulkAckProcessing(false);
    }
  };

  return {
    effectiveAlerts,
    unacknowledgedAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
  };
}
