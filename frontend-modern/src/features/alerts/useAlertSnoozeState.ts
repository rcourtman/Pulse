import { createEffect, createMemo, createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import type { OperationalRecord } from '@/types/operationalTrust';
import {
  getAlertOverviewResumedNotification,
  getAlertOverviewSnoozeFailureNotification,
  getAlertOverviewSnoozedNotification,
} from '@/utils/alertOverviewPresentation';
import { logger } from '@/utils/logger';

import { getCanonicalAlertId } from './identity';

function recordForSnooze(alert: Alert, until: string, now: string): OperationalRecord {
  const existing = alert.operationalRecord;
  return {
    id: existing?.id ?? `alert:${getCanonicalAlertId(alert)}`,
    canonicalSpecId: existing?.canonicalSpecId ?? alert.type,
    subjectResourceId: existing?.subjectResourceId ?? alert.resourceId,
    state: 'suppressed',
    severity: existing?.severity ?? alert.level ?? 'unknown',
    firstObservedAt: existing?.firstObservedAt ?? alert.startTime,
    lastObservedAt: existing?.lastObservedAt ?? alert.lastSeen ?? alert.startTime,
    stateChangedAt: now,
    resolvedAt: existing?.resolvedAt,
    acknowledgement: existing?.acknowledgement,
    suppression: { at: now, by: 'current-user', reason: 'user_snooze', expiresAt: until },
    evidenceIds: existing?.evidenceIds ?? [],
    causeKey: existing?.causeKey ?? getCanonicalAlertId(alert),
    relatedResourceIds: existing?.relatedResourceIds ?? [],
    impactSummary: existing?.impactSummary,
    recommendedNextStep: existing?.recommendedNextStep,
  };
}

export function isAlertSnoozed(alert: Alert, now = Date.now()): boolean {
  const suppression = alert.operationalRecord?.suppression;
  if (alert.operationalRecord?.state !== 'suppressed' || !suppression?.expiresAt) return false;
  const expiry = Date.parse(suppression.expiresAt);
  return Number.isFinite(expiry) && expiry > now;
}

export function useAlertSnoozeState(props: {
  alerts: Accessor<Alert[]>;
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
}) {
  type SnoozeOverride = {
    record: OperationalRecord | undefined;
    baseline: OperationalRecord | undefined;
  };
  const [overrides, setOverrides] = createSignal<Record<string, SnoozeOverride>>({});
  const [processing, setProcessing] = createSignal<Set<string>>(new Set());

  const effectiveAlerts = createMemo(() =>
    props.alerts().map((alert) => {
      const id = getCanonicalAlertId(alert);
      return Object.prototype.hasOwnProperty.call(overrides(), id)
        ? { ...alert, operationalRecord: overrides()[id].record }
        : alert;
    }),
  );

  // The optimistic record only bridges the request to the shared alert-state
  // update. Once that source advances beyond the baseline, it resumes sole
  // ownership so websocket expiry and server-normalized actor data cannot be
  // masked by a stale local override.
  createEffect(() => {
    const incoming = props.alerts();
    const current = overrides();
    let next: Record<string, SnoozeOverride> | undefined;
    for (const [id, override] of Object.entries(current)) {
      const alert = incoming.find((candidate) => getCanonicalAlertId(candidate) === id);
      if (alert && alert.operationalRecord !== override.baseline) {
        next ??= { ...current };
        delete next[id];
      }
    }
    if (next) setOverrides(next);
  });

  const setProcessingAlert = (id: string, value: boolean) => {
    setProcessing((current) => {
      const next = new Set(current);
      value ? next.add(id) : next.delete(id);
      return next;
    });
  };

  const handleSnooze = async (alert: Alert, until: Date) => {
    const id = getCanonicalAlertId(alert);
    const previous = alert.operationalRecord;
    const sourceBaseline = props
      .alerts()
      .find((candidate) => getCanonicalAlertId(candidate) === id)?.operationalRecord;
    const untilISO = until.toISOString();
    const optimistic = recordForSnooze(alert, untilISO, new Date().toISOString());
    setProcessingAlert(id, true);
    setOverrides((current) => ({
      ...current,
      [id]: { record: optimistic, baseline: sourceBaseline },
    }));
    props.updateAlert(id, { operationalRecord: optimistic });
    try {
      await AlertsAPI.snooze(id, untilISO);
      notificationStore.success(getAlertOverviewSnoozedNotification(until.toLocaleString()));
    } catch (error) {
      setOverrides((current) => {
        const next = { ...current };
        delete next[id];
        return next;
      });
      props.updateAlert(id, { operationalRecord: previous });
      logger.error('Failed to snooze alert:', error);
      notificationStore.error(getAlertOverviewSnoozeFailureNotification(false));
      throw error;
    } finally {
      setProcessingAlert(id, false);
    }
  };

  const handleUnsnooze = async (alert: Alert) => {
    const id = getCanonicalAlertId(alert);
    const previous = alert.operationalRecord;
    const sourceBaseline = props
      .alerts()
      .find((candidate) => getCanonicalAlertId(candidate) === id)?.operationalRecord;
    const resumed = previous
      ? {
          ...previous,
          state: alert.acknowledged ? ('acknowledged' as const) : ('open' as const),
          stateChangedAt: new Date().toISOString(),
          suppression: undefined,
        }
      : undefined;
    setProcessingAlert(id, true);
    setOverrides((current) => ({
      ...current,
      [id]: { record: resumed, baseline: sourceBaseline },
    }));
    props.updateAlert(id, { operationalRecord: resumed });
    try {
      await AlertsAPI.unsnooze(id);
      notificationStore.success(getAlertOverviewResumedNotification());
    } catch (error) {
      setOverrides((current) => {
        const next = { ...current };
        delete next[id];
        return next;
      });
      props.updateAlert(id, { operationalRecord: previous });
      logger.error('Failed to resume alert:', error);
      notificationStore.error(getAlertOverviewSnoozeFailureNotification(true));
      throw error;
    } finally {
      setProcessingAlert(id, false);
    }
  };

  return { effectiveAlerts, processing, handleSnooze, handleUnsnooze };
}
