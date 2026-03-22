import { createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import { logger } from '@/utils/logger';

import { getCanonicalAlertId } from './identity';
import type { Override } from './types';

export interface UseAlertOverviewStateProps {
  activeAlerts: Accessor<Record<string, Alert>>;
  overrides: Accessor<Override[]>;
  showAcknowledged: Accessor<boolean>;
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
}

export function useAlertOverviewState(props: UseAlertOverviewStateProps) {
  const [processingAlerts, setProcessingAlerts] = createSignal<Set<string>>(new Set());
  const [bulkAckProcessing, setBulkAckProcessing] = createSignal(false);
  const [tick, setTick] = createSignal(Date.now());
  const tickInterval = setInterval(() => setTick(Date.now()), 60_000);
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
    clearInterval(tickInterval);
    processingReleaseTimers.forEach((timer) => clearTimeout(timer));
    processingReleaseTimers.clear();
  });

  const alertStats = createMemo(() => {
    const alerts = Object.values(props.activeAlerts());
    return {
      active: alerts.filter((alert) => !alert.acknowledged).length,
      acknowledged: alerts.filter((alert) => alert.acknowledged).length,
      total24h: alerts.filter((alert) => {
        const age = tick() - new Date(alert.startTime).getTime();
        return age >= 0 && age < 86_400_000;
      }).length,
      overrides: props.overrides().length,
    };
  });

  const filteredAlerts = createMemo(() =>
    Object.values(props.activeAlerts())
      .filter((alert) => props.showAcknowledged() || !alert.acknowledged)
      .sort((a, b) => {
        if (a.acknowledged !== b.acknowledged) {
          return a.acknowledged ? 1 : -1;
        }
        return new Date(b.startTime).getTime() - new Date(a.startTime).getTime();
      }),
  );

  const unacknowledgedAlerts = createMemo(() =>
    Object.values(props.activeAlerts()).filter((alert) => !alert.acknowledged),
  );

  const releaseAlertProcessing = (alertIdentifier: string) => {
    clearProcessingReleaseTimer(alertIdentifier);
    const timer = setTimeout(() => {
      processingReleaseTimers.delete(alertIdentifier);
      setProcessingAlerts((prev) => {
        const next = new Set(prev);
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

    setProcessingAlerts((prev) => new Set(prev).add(alertIdentifier));
    const wasAcknowledged = alert.acknowledged;

    try {
      if (wasAcknowledged) {
        await AlertsAPI.unacknowledge(alertIdentifier);
        props.updateAlert(alertIdentifier, {
          acknowledged: false,
          ackTime: undefined,
          ackUser: undefined,
        });
        notificationStore.success('Alert restored');
      } else {
        await AlertsAPI.acknowledge(alertIdentifier);
        props.updateAlert(alertIdentifier, {
          acknowledged: true,
          ackTime: new Date().toISOString(),
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

    const pending = unacknowledgedAlerts();
    if (pending.length === 0) {
      return;
    }

    setBulkAckProcessing(true);
    try {
      const result = await AlertsAPI.bulkAcknowledge(
        pending.map((alert) => getCanonicalAlertId(alert)),
      );
      const successes = result.results.filter((entry) => entry.success);
      const failures = result.results.filter((entry) => !entry.success);

      successes.forEach((entry) => {
        props.updateAlert(entry.alertIdentifier, {
          acknowledged: true,
          ackTime: new Date().toISOString(),
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
    alertStats,
    filteredAlerts,
    unacknowledgedAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
  };
}
