const CEPH_TYPE_SET = new Set(['rbd', 'cephfs', 'ceph']);

const CEPH_HEALTH_OK_STYLES =
  'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 border border-green-200 dark:border-green-800';
const CEPH_HEALTH_WARNING_STYLES =
  'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-200 border border-yellow-300 dark:border-yellow-800';
const CEPH_HEALTH_CRITICAL_STYLES =
  'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200 border border-red-300 dark:border-red-800';
const CEPH_HEALTH_DEFAULT_STYLES =
  'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200 border border-blue-200 dark:border-blue-700';

export const isCephType = (type?: string | null): boolean => {
  const normalized = (type ?? '').trim().toLowerCase();
  return CEPH_TYPE_SET.has(normalized);
};

export const getCephHealthLabel = (health?: string | null): string => {
  if (!health) return 'CEPH';
  const normalized = health.toUpperCase();
  return normalized.startsWith('HEALTH_') ? normalized.replace('HEALTH_', '') : normalized;
};

export const getCephHealthStyles = (health?: string | null): string => {
  const normalized = (health ?? '').toUpperCase();
  if (normalized === 'HEALTH_OK') {
    return CEPH_HEALTH_OK_STYLES;
  }
  if (normalized === 'HEALTH_WARN' || normalized === 'HEALTH_WARNING') {
    return CEPH_HEALTH_WARNING_STYLES;
  }
  if (normalized === 'HEALTH_ERR' || normalized === 'HEALTH_ERROR' || normalized === 'HEALTH_CRIT') {
    return CEPH_HEALTH_CRITICAL_STYLES;
  }
  return CEPH_HEALTH_DEFAULT_STYLES;
};
