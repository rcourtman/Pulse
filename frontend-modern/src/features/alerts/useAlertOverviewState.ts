import { createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import type { Alert } from '@/types/api';
import type { Override } from './types';
import { useAlertAcknowledgementState } from './useAlertAcknowledgementState';

export interface AlertGroup {
  key: string;
  primary: Alert;
  related: Alert[];
}

export function computeAlertGroupKey(alert: Alert): string {
  const rid = alert.resourceId ?? '';
  const segments = rid.split('/');
  if (segments.length > 2) {
    return segments.slice(0, -1).join('/');
  }
  return rid;
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
    effectiveAlerts,
    unacknowledgedAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
  } = useAlertAcknowledgementState({
    alerts: activeAlerts,
    updateAlert: props.updateAlert,
    allowRestore: true,
  });

  onCleanup(() => {
    clearInterval(tickInterval);
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
        const severityRank = (level: string) => (level === 'critical' ? 0 : 1);
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
      result.push({ key, primary: alerts[0], related: alerts.slice(1) });
    }
    return result;
  });

  return {
    alertStats,
    filteredAlerts,
    groupedAlerts,
    unacknowledgedAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
  };
}

export type AlertOverviewState = ReturnType<typeof useAlertOverviewState>;
