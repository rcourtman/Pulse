import type { Alert } from '@/types/api';
import { isAlertsDetectionEnabled } from '@/utils/alertsActivation';

const noAlertStyles = {
  rowClass: '',
  indicatorClass: '',
  badgeClass: '',
  hasAlert: false,
  alertCount: 0,
  severity: null as 'critical' | 'warning' | 'info' | null,
  hasPoweredOffAlert: false,
  hasNonPoweredOffAlert: false,
  hasUnacknowledgedAlert: false,
  unacknowledgedCount: 0,
  acknowledgedCount: 0,
  hasAcknowledgedOnlyAlert: false,
};

// Get alert highlighting styles based on active alerts for a resource.
// When nodeMatch is provided, also includes alerts whose `node` field matches
// (covers storage/topology/disk alerts that belong to a node but have a
// different resourceId than the node itself).
export const getAlertStyles = (
  resourceId: string | string[],
  activeAlerts: Record<string, Alert>,
  alertsEnabled: boolean | undefined = isAlertsDetectionEnabled(),
  nodeMatch?: string,
) => {
  if (!alertsEnabled) {
    return noAlertStyles;
  }

  const alertsForResource = getAlertsForResource(
    Array.isArray(resourceId) ? resourceId : [resourceId],
    activeAlerts,
    alertsEnabled,
    nodeMatch,
  );

  const unacknowledgedAlerts = alertsForResource.filter((alert) => !alert.acknowledged);
  const acknowledgedAlerts = alertsForResource.filter((alert) => alert.acknowledged);

  let highestSeverity: 'critical' | 'warning' | 'info' | null = null;
  let hasPoweredOffAlert = false;
  let hasNonPoweredOffAlert = false;

  unacknowledgedAlerts.forEach((alert) => {
    if (
      alert.level === 'critical' ||
      (alert.level === 'warning' && highestSeverity !== 'critical') ||
      (alert.level === 'info' && highestSeverity === null)
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

  if (highestSeverity === 'info') {
    return {
      rowClass: 'bg-blue-50 dark:bg-blue-950 border-l-4 border-blue-500 dark:border-blue-400',
      indicatorClass: 'bg-blue-500',
      badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
      hasAlert,
      alertCount,
      severity: 'info' as const,
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

export function getAlertsForResource(
  resourceIds: string[],
  activeAlerts: Record<string, Alert>,
  alertsEnabled: boolean | undefined = isAlertsDetectionEnabled(),
  nodeMatch?: string,
): Alert[] {
  if (!alertsEnabled) return [];
  const ids = new Set(resourceIds.filter(Boolean));
  return Object.values(activeAlerts).filter(
    (alert) => ids.has(alert.resourceId) || (nodeMatch !== undefined && alert.node === nodeMatch),
  );
}

// Alert types representing binary or enumerated state conditions rather
// than a metric crossing a threshold. For these, "current value vs
// threshold" is meaningless (both come through as 0 from the backend) and
// surfacing those fields in operator-facing copy is misleading. The
// Assistant briefing and prompt builders omit the value/threshold lines
// when the alert type is one of these.
const STATE_ALERT_TYPES: ReadonlySet<string> = new Set([
  'powered-off',
  'unreachable',
  'offline',
  'host-offline',
  'connectivity',
  'docker-host-offline',
  'docker-container-state',
  'docker-container-health',
]);

export function isStateAlertType(alertType: string | undefined): boolean {
  if (!alertType) return false;
  return STATE_ALERT_TYPES.has(alertType);
}

export function isMetricAlertType(alertType: string | undefined): boolean {
  return !isStateAlertType(alertType);
}
