import { createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import {
  getAlertOverviewAcknowledgedNotification,
  getAlertOverviewAcknowledgementFailureNotification,
  getAlertOverviewBulkAcknowledgedNotification,
  getAlertOverviewBulkAcknowledgeFailureNotification,
  getAlertOverviewBulkAcknowledgeGenericFailureNotification,
  getAlertOverviewRestoredNotification,
} from '@/utils/alertOverviewPresentation';
import { logger } from '@/utils/logger';

import { getCanonicalAlertId } from './identity';

export interface UseAlertAcknowledgementStateProps {
  alerts: Accessor<Alert[]>;
  // The shared alert store owns the optimistic acknowledgement: it holds back
  // stale payloads until the server confirms, then lets later server changes
  // (an unacknowledge from another session) through. Keep no second copy
  // here; one would outlive that confirmation and hide those changes.
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
  allowRestore?: boolean;
}

export function useAlertAcknowledgementState(props: UseAlertAcknowledgementStateProps) {
  const [processingAlerts, setProcessingAlerts] = createSignal<Set<string>>(new Set());
  const [bulkAckProcessing, setBulkAckProcessing] = createSignal(false);
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

  const unacknowledgedAlerts = createMemo(() =>
    props.alerts().filter((alert) => !alert.acknowledged),
  );

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
      props.alerts().find((entry) => getCanonicalAlertId(entry) === alertIdentifier) ?? alert;
    const wasAcknowledged = currentAlert.acknowledged;
    if (wasAcknowledged && !props.allowRestore) {
      return;
    }

    setProcessingAlerts((previous) => new Set(previous).add(alertIdentifier));

    try {
      if (wasAcknowledged) {
        await AlertsAPI.unacknowledge(alertIdentifier);
        props.updateAlert(alertIdentifier, {
          acknowledged: false,
          ackTime: undefined,
          ackUser: undefined,
        });
        notificationStore.success(getAlertOverviewRestoredNotification());
      } else {
        await AlertsAPI.acknowledge(alertIdentifier);
        props.updateAlert(alertIdentifier, {
          acknowledged: true,
          ackTime: new Date().toISOString(),
          ackUser: undefined,
        });
        notificationStore.success(getAlertOverviewAcknowledgedNotification());
      }
    } catch (error) {
      logger.error(`Failed to ${wasAcknowledged ? 'unacknowledge' : 'acknowledge'} alert:`, error);
      notificationStore.error(getAlertOverviewAcknowledgementFailureNotification(wasAcknowledged));
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
        props.updateAlert(entry.alertIdentifier, {
          acknowledged: true,
          ackTime: acknowledgedAt,
          ackUser: undefined,
        });
      });

      if (successes.length > 0) {
        notificationStore.success(getAlertOverviewBulkAcknowledgedNotification(successes.length));
      }

      if (failures.length > 0) {
        notificationStore.error(
          getAlertOverviewBulkAcknowledgeFailureNotification(failures.length),
        );
      }
    } catch (error) {
      logger.error('Bulk acknowledge failed', error);
      notificationStore.error(getAlertOverviewBulkAcknowledgeGenericFailureNotification());
    } finally {
      setBulkAckProcessing(false);
    }
  };

  const handleGroupAcknowledge = async (alerts: Alert[]) => {
    const pending = alerts.filter((alert) => {
      const id = getCanonicalAlertId(alert);
      return !props.alerts().find((e) => getCanonicalAlertId(e) === id)?.acknowledged;
    });
    if (pending.length === 0) return;

    const identifiers = pending.map((alert) => getCanonicalAlertId(alert));
    const groupProcessing = new Set(identifiers);
    setProcessingAlerts((previous) => new Set([...previous, ...groupProcessing]));

    try {
      const result = await AlertsAPI.bulkAcknowledge(identifiers);
      const acknowledgedAt = new Date().toISOString();
      const successes = result.results.filter((entry) => entry.success);

      successes.forEach((entry) => {
        props.updateAlert(entry.alertIdentifier, {
          acknowledged: true,
          ackTime: acknowledgedAt,
          ackUser: undefined,
        });
      });

      if (successes.length > 0) {
        notificationStore.success(getAlertOverviewBulkAcknowledgedNotification(successes.length));
      }
    } catch (error) {
      logger.error('Group acknowledge failed', error);
      notificationStore.error(getAlertOverviewBulkAcknowledgeGenericFailureNotification());
    } finally {
      groupProcessing.forEach((id) => releaseAlertProcessing(id));
    }
  };

  return {
    unacknowledgedAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
    handleGroupAcknowledge,
  };
}
