import type { Alert } from '@/types/api';

// Get alert highlighting styles based on active alerts for a resource
export const getAlertStyles = (resourceId: string, activeAlerts: Record<string, Alert>) => {
  const alertsForResource = Object.values(activeAlerts).filter(
    (alert) => alert.resourceId === resourceId,
  );

  let highestSeverity: 'critical' | 'warning' | null = null;
  let hasPoweredOffAlert = false;
  let hasNonPoweredOffAlert = false;

  alertsForResource.forEach((alert) => {
    if (alert.level === 'critical' || (alert.level === 'warning' && highestSeverity !== 'critical')) {
      highestSeverity = alert.level;
    }

    if (alert.type === 'powered-off') {
      hasPoweredOffAlert = true;
    } else {
      hasNonPoweredOffAlert = true;
    }
  });

  const alertCount = alertsForResource.length;

  // Return appropriate styling based on alert severity
  if (highestSeverity === 'critical') {
    return {
      rowClass: 'bg-red-50 dark:bg-red-950/30 border-l-4 border-red-500 dark:border-red-400',
      indicatorClass: 'bg-red-500',
      badgeClass: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
      hasAlert: alertCount > 0,
      alertCount,
      severity: 'critical' as const,
      hasPoweredOffAlert,
      hasNonPoweredOffAlert,
    };
  }

  if (highestSeverity === 'warning') {
    return {
      rowClass:
        'bg-yellow-50 dark:bg-yellow-950/20 border-l-4 border-yellow-500 dark:border-yellow-400',
      indicatorClass: 'bg-yellow-500',
      badgeClass: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
      hasAlert: alertCount > 0,
      alertCount,
      severity: 'warning' as const,
      hasPoweredOffAlert,
      hasNonPoweredOffAlert,
    };
  }

  return {
    rowClass: '',
    indicatorClass: '',
    badgeClass: '',
    hasAlert: false,
    alertCount: 0,
    severity: null,
    hasPoweredOffAlert: false,
    hasNonPoweredOffAlert: false,
  };
};

// Get alert messages for a specific resource
export const getResourceAlerts = (
  resourceId: string,
  activeAlerts: Record<string, Alert>,
): Alert[] => {
  return Object.values(activeAlerts).filter((alert) => alert.resourceId === resourceId);
};
