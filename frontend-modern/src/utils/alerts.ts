import type { Alert } from '@/types/api';
import { isAlertsActivationEnabled } from '@/utils/alertsActivation';

const noAlertStyles = {
  rowClass: '',
  indicatorClass: '',
  badgeClass: '',
  hasAlert: false,
  alertCount: 0,
  severity: null as 'critical' | 'warning' | null,
  hasPoweredOffAlert: false,
  hasNonPoweredOffAlert: false,
  hasUnacknowledgedAlert: false,
  unacknowledgedCount: 0,
  acknowledgedCount: 0,
  hasAcknowledgedOnlyAlert: false,
};

// Get alert highlighting styles based on active alerts for a resource
export const getAlertStyles = (
  resourceId: string,
  activeAlerts: Record<string, Alert>,
  alertsEnabled: boolean | undefined = isAlertsActivationEnabled(),
) => {
  if (!alertsEnabled) {
    return noAlertStyles;
  }

  const alertsForResource = Object.values(activeAlerts).filter(
    (alert) => alert.resourceId === resourceId,
  );

  const unacknowledgedAlerts = alertsForResource.filter((alert) => !alert.acknowledged);
  const acknowledgedAlerts = alertsForResource.filter((alert) => alert.acknowledged);

  let highestSeverity: 'critical' | 'warning' | null = null;
  let hasPoweredOffAlert = false;
  let hasNonPoweredOffAlert = false;

  unacknowledgedAlerts.forEach((alert) => {
    if (
      alert.level === 'critical' ||
      (alert.level === 'warning' && highestSeverity !== 'critical')
    ) {
      highestSeverity = alert.level;
    }

    if (alert.type === 'powered-off') {
      hasPoweredOffAlert = true;
    } else {
      hasNonPoweredOffAlert = true;
    }
  });

  const alertCount = alertsForResource.length;
  const unacknowledgedCount = unacknowledgedAlerts.length;
  const acknowledgedCount = acknowledgedAlerts.length;
  const hasUnacknowledgedAlert = unacknowledgedCount > 0;
  const hasAlert = alertCount > 0;

  if (highestSeverity === 'critical') {
    return {
      rowClass: 'bg-red-50 dark:bg-red-950 border-l-4 border-red-500 dark:border-red-400',
      indicatorClass: 'bg-red-500',
      badgeClass: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
      hasAlert,
      alertCount,
      severity: 'critical' as const,
      hasPoweredOffAlert,
      hasNonPoweredOffAlert,
      hasUnacknowledgedAlert,
      unacknowledgedCount,
      acknowledgedCount,
      hasAcknowledgedOnlyAlert: !hasUnacknowledgedAlert && acknowledgedCount > 0,
    };
  }

  if (highestSeverity === 'warning') {
    return {
      rowClass:
        'bg-yellow-50 dark:bg-yellow-950 border-l-4 border-yellow-500 dark:border-yellow-400',
      indicatorClass: 'bg-yellow-500',
      badgeClass: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
      hasAlert,
      alertCount,
      severity: 'warning' as const,
      hasPoweredOffAlert,
      hasNonPoweredOffAlert,
      hasUnacknowledgedAlert,
      unacknowledgedCount,
      acknowledgedCount,
      hasAcknowledgedOnlyAlert: !hasUnacknowledgedAlert && acknowledgedCount > 0,
    };
  }

  return {
    rowClass: '',
    indicatorClass: '',
    badgeClass: '',
    hasAlert,
    alertCount,
    severity: null,
    hasPoweredOffAlert,
    hasNonPoweredOffAlert,
    hasUnacknowledgedAlert,
    unacknowledgedCount,
    acknowledgedCount,
    hasAcknowledgedOnlyAlert: !hasUnacknowledgedAlert && acknowledgedCount > 0,
  };
};
