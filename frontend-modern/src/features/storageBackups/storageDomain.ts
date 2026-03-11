const CEPH_TYPE_SET = new Set(['rbd', 'cephfs', 'ceph']);

const CEPH_HEALTH_OK_STYLES =
  'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 border border-green-200 dark:border-green-800';
const CEPH_HEALTH_WARNING_STYLES =
  'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-200 border border-yellow-300 dark:border-yellow-800';
const CEPH_HEALTH_CRITICAL_STYLES =
  'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200 border border-red-300 dark:border-red-800';
const CEPH_HEALTH_DEFAULT_STYLES =
  'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200 border border-blue-200 dark:border-blue-700';
const CEPH_SERVICE_OK_TEXT = 'text-green-600 dark:text-green-400';
const CEPH_SERVICE_WARNING_TEXT = 'text-yellow-600 dark:text-yellow-400';
const CEPH_SERVICE_CRITICAL_TEXT = 'text-red-600 dark:text-red-400';

export interface CephHealthPresentation {
  label: string;
  badgeClass: string;
  dotClass: string;
}

export interface CephServiceStatusPresentation {
  label: 'healthy' | 'degraded' | 'down';
  textClass: string;
}

export interface CephPageStatePresentation {
  title: string;
  description: string;
}

export interface CephServiceLike {
  running?: number | null;
  total?: number | null;
}

const normalizeCephHealth = (health?: string | null): 'ok' | 'warning' | 'critical' | 'unknown' => {
  const normalized = (health ?? '').trim().toUpperCase();
  if (normalized === 'OK' || normalized === 'HEALTH_OK') return 'ok';
  if (
    normalized === 'WARN' ||
    normalized === 'WARNING' ||
    normalized === 'HEALTH_WARN' ||
    normalized === 'HEALTH_WARNING'
  ) {
    return 'warning';
  }
  if (
    normalized === 'ERR' ||
    normalized === 'ERROR' ||
    normalized === 'CRITICAL' ||
    normalized === 'HEALTH_ERR' ||
    normalized === 'HEALTH_ERROR' ||
    normalized === 'HEALTH_CRIT'
  ) {
    return 'critical';
  }
  return 'unknown';
};

export const isCephType = (type?: string | null): boolean => {
  const normalized = (type ?? '').trim().toLowerCase();
  return CEPH_TYPE_SET.has(normalized);
};

export const getCephHealthPresentation = (health?: string | null): CephHealthPresentation => {
  switch (normalizeCephHealth(health)) {
    case 'ok':
      return {
        label: 'OK',
        badgeClass: CEPH_HEALTH_OK_STYLES,
        dotClass: 'bg-green-500',
      };
    case 'warning':
      return {
        label: 'WARN',
        badgeClass: CEPH_HEALTH_WARNING_STYLES,
        dotClass: 'bg-yellow-500',
      };
    case 'critical':
      return {
        label: 'ERROR',
        badgeClass: CEPH_HEALTH_CRITICAL_STYLES,
        dotClass: 'bg-red-500',
      };
    default:
      return {
        label: 'UNKNOWN',
        badgeClass: CEPH_HEALTH_DEFAULT_STYLES,
        dotClass: 'bg-blue-500',
      };
  }
};

export const getCephHealthLabel = (health?: string | null): string =>
  getCephHealthPresentation(health).label;

export const getCephHealthStyles = (health?: string | null): string =>
  getCephHealthPresentation(health).badgeClass;

export const getCephServiceStatusPresentation = (
  service?: CephServiceLike | null,
): CephServiceStatusPresentation => {
  const running = service?.running ?? 0;
  const total = service?.total ?? 0;

  if (running === total) {
    return {
      label: 'healthy',
      textClass: CEPH_SERVICE_OK_TEXT,
    };
  }

  if (running > 0) {
    return {
      label: 'degraded',
      textClass: CEPH_SERVICE_WARNING_TEXT,
    };
  }

  return {
    label: 'down',
    textClass: CEPH_SERVICE_CRITICAL_TEXT,
  };
};

export const getCephLoadingStatePresentation = (): CephPageStatePresentation => ({
  title: 'Loading Ceph data...',
  description: 'Connecting to the monitoring service.',
});

export const getCephDisconnectedStatePresentation = (
  reconnecting: boolean,
): CephPageStatePresentation => ({
  title: 'Connection lost',
  description: reconnecting
    ? 'Attempting to reconnect…'
    : 'Unable to connect to the backend server',
});

export const getCephNoClustersStatePresentation = (): CephPageStatePresentation => ({
  title: 'No Ceph Clusters Detected',
  description:
    'Ceph cluster data will appear here when detected via the Pulse agent on your Proxmox nodes. Install the agent on a node with Ceph configured.',
});

export const getCephPoolsSearchEmptyStatePresentation = (search: string) => ({
  text: `No pools match "${search}"`,
});
