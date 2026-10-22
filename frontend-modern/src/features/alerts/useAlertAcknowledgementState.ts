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
  const processingReleaseTimers = new Set<ReturnType<typeof setTimeout>>();

  onCleanup(() => {
    processingReleaseTimers.forEach((timer) => clearTimeout(timer));
    processingReleaseTimers.clear();
  });

  const unacknowledgedAlerts = createMemo(() =>
    props.alerts().filter((alert) => !alert.acknowledged),
  );

  // One timer and one Set copy per completed request, not per alert. Acknowledge
  // all is also used during floods; releasing N locks must not do N full copies.
  const releaseAlertProcessing = (identifiers: string[]) => {
    const timer = setTimeout(() => {
      processingReleaseTimers.delete(timer);
      setProcessingAlerts((previous) => {
        const next = new Set(previous);
        identifiers.forEach((id) => next.delete(id));
        return next;
      });
    }, 1500);
    processingReleaseTimers.add(timer);
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
      releaseAlertProcessing([alertIdentifier]);
    }
  };

  // Count only confirmations for this exact request. Missing results are not
  // success, duplicate results cannot inflate counts, and another alert's
  // result must never acknowledge a same-name neighbour.
  const applyBulkAcknowledgements = (
    identifiers: string[],
    result: Awaited<ReturnType<typeof AlertsAPI.bulkAcknowledge>>,
  ) => {
    const requested = new Set(identifiers);
    const confirmations = new Map<string, boolean>();
    for (const entry of result.results) {
      if (!requested.has(entry.alertIdentifier)) continue;
      confirmations.set(
        entry.alertIdentifier,
        (confirmations.get(entry.alertIdentifier) ?? true) && entry.success === true,
      );
    }
    const successes = identifiers.filter((id) => confirmations.get(id) === true);
    const failures = identifiers.length - successes.length;
    const acknowledgedAt = new Date().toISOString();
    successes.forEach((id) => {
      props.updateAlert(id, {
        acknowledged: true,
        ackTime: acknowledgedAt,
        ackUser: undefined,
      });
    });
    if (successes.length > 0) {
      notificationStore.success(getAlertOverviewBulkAcknowledgedNotification(successes.length));
    }
    if (failures > 0) {
      notificationStore.error(getAlertOverviewBulkAcknowledgeFailureNotification(failures));
    }
  };

  const handleBulkAcknowledge = async () => {
    if (bulkAckProcessing() || processingAlerts().size > 0) {
      return;
    }

    const identifiers = [...new Set(unacknowledgedAlerts().map(getCanonicalAlertId))];
    if (identifiers.length === 0) {
      return;
    }

    setBulkAckProcessing(true);
    setProcessingAlerts((previous) => new Set([...previous, ...identifiers]));
    try {
      const result = await AlertsAPI.bulkAcknowledge(identifiers);
      applyBulkAcknowledgements(identifiers, result);
    } catch (error) {
      logger.error('Bulk acknowledge failed', error);
      notificationStore.error(getAlertOverviewBulkAcknowledgeGenericFailureNotification());
    } finally {
      setBulkAckProcessing(false);
      releaseAlertProcessing(identifiers);
    }
  };

  const handleGroupAcknowledge = async (alerts: Alert[]) => {
    const groupIdentifiers = [...new Set(alerts.map(getCanonicalAlertId))];
    // A group, individual card and Acknowledge all share the same per-alert
    // lock. Do not send a second write for an overlapping request.
    if (bulkAckProcessing() || groupIdentifiers.some((id) => processingAlerts().has(id))) {
      return;
    }
    const current = new Map(props.alerts().map((alert) => [getCanonicalAlertId(alert), alert]));
    const identifiers = groupIdentifiers.filter((id) => {
      const alert = current.get(id);
      return alert && !alert.acknowledged;
    });
    if (identifiers.length === 0) return;

    setProcessingAlerts((previous) => new Set([...previous, ...identifiers]));
    try {
      const result = await AlertsAPI.bulkAcknowledge(identifiers);
      applyBulkAcknowledgements(identifiers, result);
    } catch (error) {
      logger.error('Group acknowledge failed', error);
      notificationStore.error(getAlertOverviewBulkAcknowledgeGenericFailureNotification());
    } finally {
      releaseAlertProcessing(identifiers);
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
