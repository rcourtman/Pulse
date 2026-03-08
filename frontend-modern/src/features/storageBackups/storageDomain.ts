const CEPH_TYPE_SET = new Set(['rbd', 'cephfs', 'ceph']);

const CEPH_HEALTH_OK_STYLES =
  'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 border border-green-200 dark:border-green-800';
const CEPH_HEALTH_WARNING_STYLES =
  'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-200 border border-yellow-300 dark:border-yellow-800';
const CEPH_HEALTH_CRITICAL_STYLES =
  'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200 border border-red-300 dark:border-red-800';
const CEPH_HEALTH_DEFAULT_STYLES =
  'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200 border border-blue-200 dark:border-blue-700';

export interface CephHealthPresentation {
  label: string;
  badgeClass: string;
  dotClass: string;
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
