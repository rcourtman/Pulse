import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import type { Alert, AlertDeliveryDiagnosis } from '@/types/api';
import type { Override } from './types';
import { useAlertAcknowledgementState } from './useAlertAcknowledgementState';
import { useAlertSnoozeState } from './useAlertSnoozeState';

export interface AlertGroup {
  key: string;
  primary: Alert;
  related: Alert[];
  correlated: boolean;
}

export function computeAlertGroupKey(alert: Alert): string {
  const correlation = alert.correlation;
  if (correlation?.kind === 'shared-system' && correlation.key.trim() !== '') {
    return `correlation:${correlation.kind}:${correlation.key.trim()}`;
  }
  const resourceId = alert.resourceId?.trim();
  return resourceId ? `resource:${resourceId}` : `alert:${alert.id}`;
}

export interface UseAlertOverviewStateProps {
  activeAlerts: Accessor<Record<string, Alert>>;
  overrides: Accessor<Override[]>;
  showAcknowledged: Accessor<boolean>;
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
}

export function useAlertOverviewState(props: UseAlertOverviewStateProps) {
  const [tick, setTick] = createSignal(Date.now());
  const tickInterval = setInterval(() => setTick(Date.now()), 60_000);
  const activeAlerts = createMemo(() => Object.values(props.activeAlerts()));
  const {
    effectiveAlerts: acknowledgedAlerts,
    unacknowledgedAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
    handleGroupAcknowledge,
  } = useAlertAcknowledgementState({
    alerts: activeAlerts,
    updateAlert: props.updateAlert,
    allowRestore: true,
  });
  const {
    effectiveAlerts,
    processing: snoozeProcessingAlerts,
    handleSnooze,
    handleUnsnooze,
  } = useAlertSnoozeState({ alerts: acknowledgedAlerts, updateAlert: props.updateAlert });

  onCleanup(() => {
    clearInterval(tickInterval);
  });

  // Delivery diagnoses answer "did/will this alert notify?" per card. One
  // bulk request covers every active alert; a failed refresh keeps the last
  // snapshot and the card simply shows no delivery line for unknown alerts.
  const [deliveryDiagnoses, setDeliveryDiagnoses] = createSignal<
    Record<string, AlertDeliveryDiagnosis>
  >({});
  let diagnosisStateDisposed = false;
  onCleanup(() => {
    diagnosisStateDisposed = true;
  });
  const refreshDeliveryDiagnoses = async () => {
    if (activeAlerts().length === 0) {
      setDeliveryDiagnoses({});
      return;
    }
    try {
      const list = await AlertsAPI.getDeliveryDiagnoses();
      if (diagnosisStateDisposed) return;
      const next: Record<string, AlertDeliveryDiagnosis> = {};
      for (const diagnosis of list) {
        next[diagnosis.alertIdentifier || diagnosis.alertId] = diagnosis;
      }
      setDeliveryDiagnoses(next);
    } catch {
      // Silent degrade: no diagnosis, no delivery line.
    }
  };
  const activeAlertIdsKey = createMemo(() =>
    activeAlerts()
      .map((alert) => alert.id)
      .sort()
      .join('\n'),
  );
  createEffect(() => {
    // Refresh when the active alert set changes and on the shared minute
    // tick, so time-based holds (cooldown, quiet hours) stay current.
    activeAlertIdsKey();
    tick();
    void refreshDeliveryDiagnoses();
  });

  const alertStats = createMemo(() => {
    const alerts = effectiveAlerts();
    const recent = alerts.filter((alert) => {
      const ts = new Date(alert.startTime).getTime();
      if (Number.isNaN(ts)) return true;
      const age = tick() - ts;
      return age >= 0 && age < 86_400_000;
    });
    return {
      active: alerts.filter((alert) => !alert.acknowledged).length,
      acknowledged: alerts.filter((alert) => alert.acknowledged).length,
      total24h: recent.length,
      critical24h: recent.filter((alert) => alert.level === 'critical').length,
      overrides: props.overrides().length,
    };
  });

  const filteredAlerts = createMemo(() =>
    effectiveAlerts()
      .filter((alert) => props.showAcknowledged() || !alert.acknowledged)
      .sort((a, b) => {
        if (a.acknowledged !== b.acknowledged) {
          return a.acknowledged ? 1 : -1;
        }
        const severityRank = (level: string) =>
          level === 'critical' ? 0 : level === 'warning' ? 1 : 2;
        const severityDiff = severityRank(a.level) - severityRank(b.level);
        if (severityDiff !== 0) return severityDiff;
        const timeDiff = new Date(b.startTime).getTime() - new Date(a.startTime).getTime();
        if (timeDiff !== 0) return timeDiff;
        return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
      }),
  );

  const groupedAlerts = createMemo<AlertGroup[]>(() => {
    const sorted = filteredAlerts();
    const groups = new Map<string, Alert[]>();
    for (const alert of sorted) {
      const key = computeAlertGroupKey(alert);
      const existing = groups.get(key);
      if (existing) {
        existing.push(alert);
      } else {
        groups.set(key, [alert]);
      }
    }
    const result: AlertGroup[] = [];
    for (const [key, alerts] of groups) {
      const primaryIndex = alerts.findIndex((alert) => alert.correlation?.role === 'primary');
      const primary = primaryIndex >= 0 ? alerts[primaryIndex] : alerts[0];
      result.push({
        key,
        primary,
        related: alerts.filter((_, index) => index !== (primaryIndex >= 0 ? primaryIndex : 0)),
        correlated: key.startsWith('correlation:'),
      });
    }
    return result;
  });

  return {
    alertStats,
    filteredAlerts,
    groupedAlerts,
    unacknowledgedAlerts,
    processingAlerts,
    snoozeProcessingAlerts,
    bulkAckProcessing,
    deliveryDiagnoses,
    refreshDeliveryDiagnoses,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
    handleGroupAcknowledge,
    handleSnooze,
    handleUnsnooze,
  };
}

export type AlertOverviewState = ReturnType<typeof useAlertOverviewState>;
