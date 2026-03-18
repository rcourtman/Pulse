import type { WorkloadType } from '@/types/workloads';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

export interface WorkloadTypePresentation {
  label: string;
  title: string;
  className: string;
}

type WorkloadTypePresentationKey =
  | WorkloadType
  | 'system-container'
  | 'app-container'
  | 'agent'
  | 'pod'
  | 'oci-container';

const PRESENTATION_MAP: Record<WorkloadTypePresentationKey, WorkloadTypePresentation> = {
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
  'app-container': {
    label: 'Containers',
    title: 'Application Container',
    className: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
  },
  pod: {
    label: 'Pod',
    title: 'Kubernetes Pod',
    className: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  },
  agent: {
    label: 'Agent',
    title: 'Agent',
    className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  },
  'oci-container': {
    label: 'OCI',
    title: 'OCI Container',
    className: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
  },
};

const DEFAULT_PRESENTATION: WorkloadTypePresentation = {
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

export const normalizeWorkloadTypePresentationKey = (
  value: string | null | undefined,
): WorkloadTypePresentationKey | null => {
  const canonical = canonicalizeFrontendResourceType(value);
  if (!canonical) return null;
  if (
    canonical === 'vm' ||
    canonical === 'system-container' ||
    canonical === 'app-container' ||
    canonical === 'pod' ||
    canonical === 'agent' ||
    canonical === 'oci-container'
  ) {
    return canonical;
  }
  return null;
};

export const getWorkloadTypePresentation = (
  rawType: string | WorkloadType | null | undefined,
  overrides?: Partial<Pick<WorkloadTypePresentation, 'label' | 'title'>>,
): WorkloadTypePresentation => {
  const normalized = normalizeWorkloadTypePresentationKey(rawType);
  const fallbackLabel =
    overrides?.label || toTitleCase((rawType || '').toString()) || DEFAULT_PRESENTATION.label;

  if (!normalized) {
    return {
      ...DEFAULT_PRESENTATION,
      label: fallbackLabel,
      title: overrides?.title || fallbackLabel,
    };
  }

  const base = PRESENTATION_MAP[normalized];
  return {
    ...base,
    label: overrides?.label || base.label,
    title: overrides?.title || base.title,
  };
};

export const getWorkloadTypeLabel = (rawType: string | WorkloadType | null | undefined): string =>
  getWorkloadTypePresentation(rawType).label;
