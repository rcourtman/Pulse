import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { StatusIndicator } from '@/utils/status';

/** Only explicit IDs, never a shared display name, hostname or node label. */
export function getPMGAlertResourceIds(resource: Resource): string[] {
  return [
    resource.id,
    resource.canonicalIdentity?.primaryId,
    ...(resource.canonicalIdentity?.supersededIds ?? []),
    ...(resource.canonicalIdentity?.aliases ?? []).filter(
      (alias) => alias.startsWith('pmg:') || alias.startsWith('agent:'),
    ),
    resource.pmg?.instanceId,
    resource.metricsTarget?.resourceId,
  ].filter((id): id is string => typeof id === 'string' && id.trim().length > 0);
}

function classifyHealth(status: string | undefined): StatusIndicator {
  const raw = (status ?? '').trim().toLowerCase();
  if (raw === 'online' || raw === 'ok' || raw === 'healthy') {
    return { variant: 'success', label: 'Healthy' };
  }
  if (raw === 'degraded' || raw === 'warning' || raw === 'partial') {
    return { variant: 'warning', label: 'Degraded' };
  }
  if (raw === 'unhealthy' || raw === 'unreachable') {
    return { variant: 'danger', label: 'Unreachable' };
  }
  if (raw === 'offline' || raw === 'error' || raw === 'critical' || raw === 'down') {
    return { variant: 'danger', label: 'Offline' };
  }
  return { variant: 'muted', label: raw || 'Unknown' };
}

const HEALTH_RANK = { success: 0, info: 1, muted: 2, warning: 3, danger: 4 };

/** A successful version/detail read cannot clear the resource's adverse state. */
export function getPMGHealth(
  statuses: Array<string | undefined>,
  alerts: Alert[] = [],
): StatusIndicator {
  const known = statuses.filter((status) => status?.trim()).map(classifyHealth);
  const base = known.reduce<StatusIndicator>(
    (worst, next) => (HEALTH_RANK[next.variant] > HEALTH_RANK[worst.variant] ? next : worst),
    known[0] ?? classifyHealth(undefined),
  );
  const open = alerts.filter((alert) => !alert.acknowledged);
  if (open.length === 0 || base.variant === 'danger') return base;
  return {
    variant: open.some((alert) => alert.level === 'critical') ? 'danger' : 'warning',
    label: 'Needs attention',
  };
}
