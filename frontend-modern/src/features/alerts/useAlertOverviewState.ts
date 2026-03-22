import { createMemo, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';

import type { Alert } from '@/types/api';
import type { Override } from './types';
import { useAlertAcknowledgementState } from './useAlertAcknowledgementState';

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
    effectiveAlerts()
      .filter((alert) => props.showAcknowledged() || !alert.acknowledged)
      .sort((a, b) => {
        if (a.acknowledged !== b.acknowledged) {
          return a.acknowledged ? 1 : -1;
        }
        return new Date(b.startTime).getTime() - new Date(a.startTime).getTime();
      }),
  );

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

export type AlertOverviewState = ReturnType<typeof useAlertOverviewState>;
