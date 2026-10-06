import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { getActionableAgentIdFromResource } from '@/utils/agentResources';
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
      rowClass: 'bg-red-50 dark:bg-red-950/25 border-l-4 border-red-500 dark:border-red-400',
      indicatorClass: 'bg-red-500',
      badgeClass: 'bg-red-100 text-red-800 dark:bg-red-900/25 dark:text-red-200',
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
        'bg-yellow-50 dark:bg-yellow-950/25 border-l-4 border-yellow-500 dark:border-yellow-400',
      indicatorClass: 'bg-yellow-500',
      badgeClass: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/25 dark:text-yellow-200',
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
      rowClass: 'bg-blue-50 dark:bg-blue-950/25 border-l-4 border-blue-500 dark:border-blue-400',
      indicatorClass: 'bg-blue-500',
      badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900/25 dark:text-blue-200',
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

const ALERT_LEVEL_RANK: Record<Alert['level'], number> = { critical: 0, warning: 1, info: 2 };

// The alert keys one unified resource answers to. Alerts raised on the unified
// resource itself (provider incidents, availability, TrueNAS, vSphere,
// Kubernetes) carry its id; poller alerts carry the canonical primary id, the
// Proxmox source id or the metrics target (PBS, PMG, storage). A machine also
// answers to its agent ("agent:<id>") and Docker runtime ("docker:<id>") keys,
// and its components' alerts nest under those: "agent:<id>/disk:<mount>",
// "agent:<id>/raid:<device>", "docker:<id>/<container>".
const getUnifiedResourceAlertKeys = (
  resource: Resource,
): { exact: Set<string>; machine: string[] } => {
  const exact = new Set<string>();
  const add = (value: string | undefined) => {
    const trimmed = value?.trim();
    if (trimmed) exact.add(trimmed);
  };
  add(resource.id);
  add(resource.canonicalIdentity?.primaryId);
  resource.canonicalIdentity?.supersededIds?.forEach(add);
  add(resource.proxmox?.sourceId);
  add(resource.metricsTarget?.resourceId);

  const machine = new Set<string>();
  if (resource.type === 'agent' || resource.type === 'docker-host') {
    const agentId = getActionableAgentIdFromResource(resource);
    if (agentId) machine.add(agentId.startsWith('agent:') ? agentId : `agent:${agentId}`);
    // The backend writes the agent key of a machine reported through a linked
    // agent (a Proxmox node, a Docker host) into the canonical aliases.
    resource.canonicalIdentity?.aliases
      ?.filter((alias) => alias.startsWith('agent:'))
      .forEach((alias) => machine.add(alias));
    // Docker hosts are "docker-host" resources or agents with a Docker runtime;
    // the backend keys their alerts on the Docker host's source id.
    const dockerHostId =
      resource.docker?.hostSourceId?.trim() ||
      (resource.metricsTarget?.resourceType === 'docker-host'
        ? resource.metricsTarget.resourceId?.trim()
        : undefined);
    if (dockerHostId) machine.add(`docker:${dockerHostId}`);
  }
  machine.forEach(add);
  return { exact, machine: [...machine] };
};

/**
 * Open alerts for one unified resource, read from the websocket's active
 * alert map, most severe first. This is the canonical source for a resource
 * drawer's "Needs attention" list: resources do not embed their alerts.
 */
export function getAlertsForUnifiedResource(
  resource: Resource,
  activeAlerts: Record<string, Alert>,
  alertsEnabled: boolean | undefined = isAlertsDetectionEnabled(),
): Alert[] {
  if (!alertsEnabled) return [];
  const { exact, machine } = getUnifiedResourceAlertKeys(resource);
  const machinePrefixes = machine.map((key) => `${key}/`);
  return Object.values(activeAlerts)
    .filter(
      (alert) =>
        exact.has(alert.resourceId) ||
        machinePrefixes.some((prefix) => alert.resourceId?.startsWith(prefix)),
    )
    .sort(
      (a, b) =>
        (ALERT_LEVEL_RANK[a.level] ?? 3) - (ALERT_LEVEL_RANK[b.level] ?? 3) ||
        Date.parse(a.startTime) - Date.parse(b.startTime),
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
