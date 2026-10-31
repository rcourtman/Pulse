import { createMemo } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import type {
  AlertAcknowledgementRequest,
  AlertAcknowledgementRequests,
} from '@/stores/alertAcknowledgementRequests';
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

type BulkAcknowledgeResult = Awaited<ReturnType<typeof AlertsAPI.bulkAcknowledge>>;

export interface UseAlertAcknowledgementStateProps {
  alerts: Accessor<Alert[]>;
  // The shared alert store owns the optimistic acknowledgement: it holds back
  // stale payloads until the server confirms, then lets later server changes
  // (an unacknowledge from another session) through. Keep no second copy
  // here; one would outlive that confirmation and hide those changes.
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
  // The alert store's record of which request carries each alert. It outlives
  // this view, so a remounted overview neither resends an alert in flight nor
  // shows it idle, and the request that carries an alert reports its outcome.
  acknowledgementRequests: AlertAcknowledgementRequests;
  allowRestore?: boolean;
}

export function useAlertAcknowledgementState(props: UseAlertAcknowledgementStateProps) {
  const requests = props.acknowledgementRequests;
  // Busy state follows the requests themselves, so an older request's hold
  // cannot mark an alert idle while a newer request still carries it.
  const processingAlerts = createMemo(() => requests.processingAlerts());
  const bulkAckProcessing = createMemo(() => requests.bulkInFlight());

  const unacknowledgedAlerts = createMemo(() =>
    props.alerts().filter((alert) => !alert.acknowledged),
  );

  // Global and group actions share the bulk endpoint, so they read its
  // per-alert results one way: a partial result reports both counts. A
  // response from an ended alert scope (an organization switch or shutdown)
  // neither writes nor reports: the store and the notifications now belong
  // to another scope, whose alerts can share these identifiers.
  const applyBulkAcknowledgeResult = (
    request: AlertAcknowledgementRequest,
    result: BulkAcknowledgeResult,
  ) => {
    if (!requests.isCurrent(request)) {
      return;
    }

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
      notificationStore.error(getAlertOverviewBulkAcknowledgeFailureNotification(failures.length));
    }
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

    // Another request that carries this alert reports its outcome.
    const request = requests.claim('alert', [alertIdentifier]);
    if (!request) {
      return;
    }

    try {
      if (wasAcknowledged) {
        await AlertsAPI.unacknowledge(alertIdentifier);
        // A response from an ended alert scope neither writes nor reports.
        if (requests.isCurrent(request)) {
          props.updateAlert(alertIdentifier, {
            acknowledged: false,
            ackTime: undefined,
            ackUser: undefined,
          });
          notificationStore.success(getAlertOverviewRestoredNotification());
        }
      } else {
        await AlertsAPI.acknowledge(alertIdentifier);
        if (requests.isCurrent(request)) {
          props.updateAlert(alertIdentifier, {
            acknowledged: true,
            ackTime: new Date().toISOString(),
            ackUser: undefined,
          });
          notificationStore.success(getAlertOverviewAcknowledgedNotification());
        }
      }
    } catch (error) {
      logger.error(`Failed to ${wasAcknowledged ? 'unacknowledge' : 'acknowledge'} alert:`, error);
      if (requests.isCurrent(request)) {
        notificationStore.error(
          getAlertOverviewAcknowledgementFailureNotification(wasAcknowledged),
        );
      }
    } finally {
      requests.release(request);
    }
  };

  const handleBulkAcknowledge = async () => {
    if (bulkAckProcessing()) {
      return;
    }

    // Alerts a card or group request carries stay with it.
    const request = requests.claim(
      'bulk',
      unacknowledgedAlerts().map((alert) => getCanonicalAlertId(alert)),
    );
    if (!request) {
      return;
    }

    try {
      const result = await AlertsAPI.bulkAcknowledge([...request.alertIdentifiers]);
      applyBulkAcknowledgeResult(request, result);
    } catch (error) {
      logger.error('Bulk acknowledge failed', error);
      if (requests.isCurrent(request)) {
        notificationStore.error(getAlertOverviewBulkAcknowledgeGenericFailureNotification());
      }
    } finally {
      requests.release(request);
    }
  };

  const handleGroupAcknowledge = async (alerts: Alert[]) => {
    const pending = alerts.filter((alert) => {
      const id = getCanonicalAlertId(alert);
      return !props.alerts().find((e) => getCanonicalAlertId(e) === id)?.acknowledged;
    });
    // Alerts another request carries stay with it.
    const request = requests.claim(
      'group',
      pending.map((alert) => getCanonicalAlertId(alert)),
    );
    if (!request) return;

    try {
      const result = await AlertsAPI.bulkAcknowledge([...request.alertIdentifiers]);
      applyBulkAcknowledgeResult(request, result);
    } catch (error) {
      logger.error('Group acknowledge failed', error);
      if (requests.isCurrent(request)) {
        notificationStore.error(getAlertOverviewBulkAcknowledgeGenericFailureNotification());
      }
    } finally {
      requests.release(request);
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
