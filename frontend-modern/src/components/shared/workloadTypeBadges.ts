import type { WorkloadType } from '@/types/workloads';

export interface WorkloadTypeBadge {
  label: string;
  title: string;
  className: string;
}

type WorkloadTypeBadgeKey =
  | WorkloadType
  | 'container'
  | 'system-container'
  | 'app-container'
  | 'host'
  | 'pod'
  | 'oci';

const BADGE_MAP: Record<WorkloadTypeBadgeKey, WorkloadTypeBadge> = {
  vm: {
    label: 'VM',
    title: 'Virtual Machine',
    className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  },
  'system-container': {
    label: 'Container',
    title: 'System Container',
    className: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  },
  container: {
    label: 'CT',
    title: 'System Container',
    className: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  },
  'app-container': {
    label: 'Containers',
    title: 'Application Container',
    className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
  },
  docker: {
    label: 'Containers',
    title: 'Container (Docker-compatible runtime)',
    className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
  },
  k8s: {
    label: 'K8s',
    title: 'Kubernetes Pod',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
  pod: {
    label: 'Pod',
    title: 'Kubernetes Pod',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
  host: {
    label: 'Host',
    title: 'Host',
    className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  },
  oci: {
    label: 'OCI',
    title: 'OCI Container',
    className: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
  },
};

const DEFAULT_BADGE: WorkloadTypeBadge = {
  label: 'Unknown',
  title: 'Unknown workload type',
  className: 'bg-surface-alt text-base-content',
};

const toTitleCase = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const normalizeKey = (value: string | null | undefined): WorkloadTypeBadgeKey | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return null;
  if (normalized === 'qemu' || normalized === 'vm') return 'vm';
  if (
    normalized === 'lxc' ||
    normalized === 'ct' ||
    normalized === 'container' ||
    normalized === 'system-container' ||
    normalized === 'system_container'
  )
    return 'system-container';
  if (
    normalized === 'docker' ||
    normalized === 'docker-container' ||
    normalized === 'docker_container' ||
    normalized === 'app-container' ||
    normalized === 'app_container'
  ) {
    return 'app-container';
  }
  if (normalized === 'k8s' || normalized === 'kubernetes') return 'k8s';
  if (normalized === 'pod') return 'pod';
  if (normalized === 'host') return 'host';
  if (normalized === 'oci') return 'oci';
  return null;
};

export const getWorkloadTypeBadge = (
  rawType: string | WorkloadType | null | undefined,
  overrides?: Partial<Pick<WorkloadTypeBadge, 'label' | 'title'>>,
): WorkloadTypeBadge => {
  const normalized = normalizeKey(rawType);
  const fallbackLabel =
    overrides?.label || toTitleCase((rawType || '').toString()) || DEFAULT_BADGE.label;

  if (!normalized) {
    return {
      ...DEFAULT_BADGE,
      label: fallbackLabel,
      title: overrides?.title || fallbackLabel,
    };
  }

  const base = BADGE_MAP[normalized];
  return {
    ...base,
    label: overrides?.label || base.label,
    title: overrides?.title || base.title,
  };
};
